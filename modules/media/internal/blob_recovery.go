package media

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// RecoverBlobJournalInput 是启动恢复blob journal的输入。
type RecoverBlobJournalInput = gen.RecoverBlobJournalInput

// RecoverBlobJournalResult 是启动恢复blob journal的结果。
type RecoverBlobJournalResult = gen.RecoverBlobJournalResult

type blobJournalRecoverer interface {
	RecoverBlobJournal(ctx context.Context) (gen.RecoverBlobJournalResult, error)
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

type journalAggregate struct {
	prepared        bool
	committed       bool
	event           blobJournalEvent
	committedEvent  blobJournalEvent
}

func (s *localBlobStore) RecoverBlobJournal(ctx context.Context) (gen.RecoverBlobJournalResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.journalPath)
	if errors.Is(err, os.ErrNotExist) {
		return gen.RecoverBlobJournalResult{}, nil
	}
	if err != nil {
		return gen.RecoverBlobJournalResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "打开媒体journal失败")
	}
	defer file.Close()

	aggregates := make(map[string]journalAggregate)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return gen.RecoverBlobJournalResult{}, contextError(ctx.Err())
		default:
		}
		var event blobJournalEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || !validJournalEvent(event) {
			return gen.RecoverBlobJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "媒体journal内容不合法")
		}
		key := string(event.SiteID) + "\x00" + string(event.UploadID) + "\x00" + string(event.SHA256)
		aggregate := aggregates[key]
		switch event.Event {
		case blobJournalPrepared:
			aggregate.prepared = true
			aggregate.event = event
		case blobJournalCommitted:
			if !validIdentifier(event.AssetID) {
				return gen.RecoverBlobJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "媒体COMMITTED journal缺少资源标识")
			}
			aggregate.committed = true
			aggregate.committedEvent = event
		default:
			// DELETING 只记录人工清理已经进入的删除决定；恢复阶段不
			// 擅自继续删除，后续清理worker会按其自己的合同处理。
			if event.Event != blobJournalDeleting {
				return gen.RecoverBlobJournalResult{}, platform.NewError(platform.CodeIntegrityError, false, "媒体journal事件未知")
			}
		}
		aggregates[key] = aggregate
	}
	if err := scanner.Err(); err != nil {
		return gen.RecoverBlobJournalResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取媒体journal失败")
	}

	var result gen.RecoverBlobJournalResult
	for _, aggregate := range aggregates {
		if aggregate.committed {
			if err := s.confirmCommittedLocked(aggregate.committedEvent); err != nil {
				return gen.RecoverBlobJournalResult{}, err
			}
			result.ConfirmedCount++
			continue
		}
		if !aggregate.prepared {
			continue
		}
		isolated, err := s.isolatePreparedLocked(aggregate.event)
		if err != nil {
			return gen.RecoverBlobJournalResult{}, err
		}
		if isolated {
			result.IsolatedCount++
		}
	}
	return result, nil
}

func validJournalEvent(event blobJournalEvent) bool {
	return event.SchemaVersion == localBlobStoreSchemaVersion &&
		(event.Event == blobJournalPrepared || event.Event == blobJournalCommitted || event.Event == blobJournalDeleting) &&
		validIdentifier(event.SiteID) && validIdentifier(event.UploadID) && validHash(event.SHA256) &&
		!event.OccurredAt.IsZero()
}

func (s *localBlobStore) confirmCommittedLocked(event blobJournalEvent) error {
	asset, ok := s.state.Assets[string(event.AssetID)]
	if !ok || asset.SiteId != event.SiteID || asset.Ref.Sha256 != event.SHA256 || asset.State != gen.AssetStateReady {
		return platform.NewError(platform.CodeIntegrityError, false, "COMMITTED资源元数据缺失或不一致")
	}
	path := s.blobPath(event.SiteID, event.SHA256)
	actualHash, _, err := hashFile(path)
	if err != nil || actualHash != event.SHA256 {
		return platform.NewError(platform.CodeIntegrityError, false, "COMMITTED blob完整性校验失败")
	}
	return nil
}

func (s *localBlobStore) isolatePreparedLocked(event blobJournalEvent) (bool, error) {
	if s.hasAssetForHashLocked(event.SiteID, event.SHA256) {
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

func (s *localBlobStore) hasAssetForHashLocked(siteID Identifier, sha256Hash Hash) bool {
	for _, asset := range s.state.Assets {
		if asset.SiteId == siteID && asset.Ref.Sha256 == sha256Hash && asset.State == gen.AssetStateReady {
			return true
		}
	}
	return false
}
