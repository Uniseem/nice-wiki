package media

import (
	"context"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// AcquireReaderLeaseInput 只由已完成下载授权的本域流程传入，不接受HTTP直接调用。
type AcquireReaderLeaseInput struct {
	Asset    gen.AssetVersionRef `json:"asset"`
	Deadline time.Time           `json:"deadline"`
}
type AcquireReaderLeaseResult struct {
	ReaderId  Identifier                  `json:"readerId"`
	ExpiresAt time.Time                   `json:"expiresAt"`
	Release   func(context.Context) error `json:"-"`
}

// AcquireReaderLease 将有限流期限与最终删除的同一asset锁及持久决定互斥。
// 读取流程须把返回期限用作流context期限，关闭后调用releaseReaderLease；不续期。
func AcquireReaderLease(ctx context.Context, op OperationContext, input AcquireReaderLeaseInput) (AcquireReaderLeaseResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return AcquireReaderLeaseResult{}, platform.NewError(platform.CodeForbidden, false, "读者租约仅供已授权本域下载流程")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return AcquireReaderLeaseResult{}, err
	}
	now := time.Now().UTC()
	if !input.Deadline.After(now) || input.Deadline.After(now.Add(10*time.Minute)) {
		return AcquireReaderLeaseResult{}, platform.NewError(platform.CodeValidation, false, "下载期限必须在十分钟内")
	}
	deadline, bounded := ctx.Deadline()
	if !bounded || input.Deadline.After(deadline) {
		return AcquireReaderLeaseResult{}, platform.NewError(platform.CodeValidation, false, "下载期限超过获准操作窗口")
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return AcquireReaderLeaseResult{}, err
	}
	defer cancel()
	release := s.lockReferenceAssets([]gen.AssetVersionRef{input.Asset})
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return AcquireReaderLeaseResult{}, mapMediaDBError(err, "开始读者租约失败")
	}
	defer tx.Rollback()
	asset, _, _, err := s.readCleanupAsset(work, tx, op.Actor.SiteID, input.Asset.AssetId)
	if err != nil {
		return AcquireReaderLeaseResult{}, err
	}
	var decisions int
	if err := tx.QueryRowContext(work, `SELECT COUNT(*) FROM manual_cleanup_deletions WHERE asset_id=?`, input.Asset.AssetId).Scan(&decisions); err != nil {
		return AcquireReaderLeaseResult{}, mapMediaDBError(err, "核对下载删除决定失败")
	}
	if asset.Ref != input.Asset || asset.State != gen.AssetStateReady || decisions != 0 {
		return AcquireReaderLeaseResult{}, platform.NewError(platform.CodeLocked, false, "资源版本或删除决定阻止下载")
	}
	result := AcquireReaderLeaseResult{ReaderId: platform.NewIdentifier(), ExpiresAt: input.Deadline.UTC()}
	result.Release = func(closeCtx context.Context) error { return s.releaseReaderLease(closeCtx, op, result.ReaderId) }
	if _, err := tx.ExecContext(work, `INSERT INTO asset_reader_leases(reader_id,asset_id,data_epoch,expires_at_ns,created_at) VALUES(?,?,?,?,?)`, result.ReaderId, input.Asset.AssetId, op.DataEpoch, result.ExpiresAt.UnixNano(), platform.Now()); err != nil {
		return AcquireReaderLeaseResult{}, mapMediaDBError(err, "写入读者租约失败")
	}
	if err := tx.Commit(); err != nil {
		return AcquireReaderLeaseResult{}, mapMediaDBError(err, "提交读者租约失败")
	}
	return result, nil
}

// releaseReaderLease 只释放本流已关闭的精确租约；未知释放不影响到期保护。
func (s *localBlobStore) releaseReaderLease(ctx context.Context, op OperationContext, readerID Identifier) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM asset_reader_leases WHERE reader_id=? AND data_epoch=?`, readerID, op.DataEpoch)
	if err != nil {
		return mapMediaDBError(err, "释放读者租约失败")
	}
	return nil
}
