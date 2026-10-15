package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// RecoverBlobJournalInput 是启动恢复blob journal的输入；恢复参数由受信启动
// 装配注入，不接受请求字段。
type RecoverBlobJournalInput struct{}

// RecoverBlobJournalResult 是启动恢复blob journal的结果。
type RecoverBlobJournalResult struct {
	ConfirmedCount int `json:"confirmedCount"`
	IsolatedCount  int `json:"isolatedCount"`
}

type blobJournalRecoverer interface {
	RecoverBlobJournal(ctx context.Context) (RecoverBlobJournalResult, error)
}

// RecoverBlobJournal 在接受业务请求前独占检查本域 blob journal。
// PREPARED 且没有有效 COMMITTED 元数据的文件只进入 isolated；本函数不
// 扫描 ready 文件，也不凭文件年龄产生删除授权。
func RecoverBlobJournal(ctx context.Context, op OperationContext, input RecoverBlobJournalInput) (RecoverBlobJournalResult, error) {
	if ctx == nil {
		return RecoverBlobJournalResult{}, platform.NewError(platform.CodeValidation, false, "操作上下文缺失")
	}
	if err := ctx.Err(); err != nil {
		return RecoverBlobJournalResult{}, contextError(err)
	}
	if op.CallerService != "media" {
		return RecoverBlobJournalResult{}, platform.NewError(platform.CodeForbidden, false, "恢复操作调用方不合法")
	}
	store := currentBlobStore()
	if store == nil {
		return RecoverBlobJournalResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储尚未装配")
	}
	recoverer, ok := store.(blobJournalRecoverer)
	if !ok {
		return RecoverBlobJournalResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体blob恢复能力尚未装配")
	}
	return recoverer.RecoverBlobJournal(ctx)
}

type blobJournalEvent struct {
	Event    string
	SiteID   Identifier
	UploadID Identifier
	SHA256   Hash
	AssetID  Identifier
}

type journalAggregate struct {
	prepared       bool
	committed      bool
	event          blobJournalEvent
	committedEvent blobJournalEvent
}

func (s *localBlobStore) RecoverBlobJournal(ctx context.Context) (RecoverBlobJournalResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT event, site_id, upload_id, sha256, asset_id FROM blob_journal ORDER BY seq`)
	if err != nil {
		return RecoverBlobJournalResult{}, mapMediaDBError(err, "读取媒体journal失败")
	}
	aggregates := make(map[string]journalAggregate)
	order := make([]string, 0)
	for rows.Next() {
		var event blobJournalEvent
		var assetID sql.NullString
		if err := rows.Scan(&event.Event, &event.SiteID, &event.UploadID, &event.SHA256, &assetID); err != nil {
			_ = rows.Close()
			return RecoverBlobJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "媒体journal内容不合法")
		}
		event.AssetID = Identifier(assetID.String)
		if !validJournalEvent(event) {
			_ = rows.Close()
			return RecoverBlobJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "媒体journal内容不合法")
		}
		key := string(event.SiteID) + "\x00" + string(event.UploadID) + "\x00" + string(event.SHA256)
		aggregate, seen := aggregates[key]
		if !seen {
			order = append(order, key)
		}
		switch event.Event {
		case blobJournalPrepared:
			aggregate.prepared = true
			aggregate.event = event
		case blobJournalCommitted:
			if !validIdentifier(event.AssetID) {
				_ = rows.Close()
				return RecoverBlobJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "媒体COMMITTED journal缺少资源标识")
			}
			aggregate.committed = true
			aggregate.committedEvent = event
		case blobJournalDeleting:
			// DELETING 只记录人工清理已经进入的删除决定；恢复阶段不
			// 擅自继续删除，后续清理worker会按其自己的合同处理。
		}
		aggregates[key] = aggregate
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return RecoverBlobJournalResult{}, mapMediaDBError(err, "读取媒体journal失败")
	}
	_ = rows.Close()

	var result RecoverBlobJournalResult
	for _, key := range order {
		if err := ctx.Err(); err != nil {
			return RecoverBlobJournalResult{}, contextError(err)
		}
		aggregate := aggregates[key]
		if aggregate.committed {
			if err := s.confirmCommittedLocked(ctx, aggregate.committedEvent); err != nil {
				return RecoverBlobJournalResult{}, err
			}
			result.ConfirmedCount++
			continue
		}
		if !aggregate.prepared {
			continue
		}
		isolated, err := s.isolatePreparedLocked(ctx, aggregate.event)
		if err != nil {
			return RecoverBlobJournalResult{}, err
		}
		if isolated {
			result.IsolatedCount++
		}
	}
	return result, nil
}

func validJournalEvent(event blobJournalEvent) bool {
	return (event.Event == blobJournalPrepared || event.Event == blobJournalCommitted || event.Event == blobJournalDeleting) &&
		validIdentifier(event.SiteID) && validIdentifier(event.UploadID) && validHash(event.SHA256) &&
		(event.AssetID == "" || validIdentifier(event.AssetID))
}

// confirmCommittedLocked 核对 COMMITTED 记录对应的资源元数据与不可变文件。
// COMMITTED 与资源元数据同事务提交，二者不一致即为完整性错误。
func (s *localBlobStore) confirmCommittedLocked(ctx context.Context, event blobJournalEvent) error {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT asset_json FROM assets WHERE asset_id = ?`, string(event.AssetID)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NewError(platform.CodeIntegrityError, false, "COMMITTED资源元数据缺失或不一致")
	}
	if err != nil {
		return mapMediaDBError(err, "读取资源元数据失败")
	}
	var asset gen.Asset
	if err := json.Unmarshal([]byte(raw), &asset); err != nil ||
		asset.SiteId != event.SiteID || asset.Ref.Sha256 != event.SHA256 || asset.State != gen.AssetStateReady {
		return platform.NewError(platform.CodeIntegrityError, false, "COMMITTED资源元数据缺失或不一致")
	}
	actualHash, _, err := hashFile(s.blobPath(event.SiteID, event.SHA256))
	if err != nil || actualHash != event.SHA256 {
		return platform.NewError(platform.CodeIntegrityError, false, "COMMITTED blob完整性校验失败")
	}
	return nil
}

// isolatePreparedLocked 把没有 COMMITTED 的 PREPARED 产物移入 isolated。同站点
// 同 sha 已有 ready 资源时文件仍被引用，保持不动。
func (s *localBlobStore) isolatePreparedLocked(ctx context.Context, event blobJournalEvent) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM assets WHERE site_id = ? AND sha256 = ? AND state = ?`,
		string(event.SiteID), string(event.SHA256), string(gen.AssetStateReady)).Scan(&count); err != nil {
		return false, mapMediaDBError(err, "读取资源元数据失败")
	}
	if count > 0 {
		return false, nil
	}
	for _, path := range []string{
		s.stagingPath(event.UploadID, event.SHA256),
		s.blobPath(event.SiteID, event.SHA256),
	} {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return false, platform.NewError(platform.CodeDependencyUnavailable, true, "检查孤儿blob失败")
		}
		destination := s.isolatedPath("prepared", event.UploadID, event.SHA256)
		if err := os.Rename(path, destination); err != nil {
			return false, platform.NewError(platform.CodeDependencyUnavailable, true, "隔离孤儿blob失败")
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return false, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化孤儿blob隔离失败")
		}
		if err := syncDirectory(filepath.Dir(destination)); err != nil {
			return false, platform.NewError(platform.CodeDependencyUnavailable, true, "持久化孤儿blob目录失败")
		}
		return true, nil
	}
	return false, nil
}
