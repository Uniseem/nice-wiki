package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type referenceLock struct {
	mu    sync.Mutex
	users int
}

func (s *localBlobStore) lockReferenceKey(key string) func() {
	s.referenceLocksMu.Lock()
	if s.referenceLocks == nil {
		s.referenceLocks = make(map[string]*referenceLock)
	}
	entry := s.referenceLocks[key]
	if entry == nil {
		entry = &referenceLock{}
		s.referenceLocks[key] = entry
	}
	entry.users++
	s.referenceLocksMu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		s.referenceLocksMu.Lock()
		entry.users--
		if entry.users == 0 {
			delete(s.referenceLocks, key)
		}
		s.referenceLocksMu.Unlock()
	}
}

func (s *localBlobStore) lockReferenceAssets(assets []gen.AssetVersionRef) func() {
	releases := make([]func(), 0, len(assets))
	for _, asset := range assets {
		releases = append(releases, s.lockReferenceKey("asset\x00"+asset.AssetId))
	}
	return func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
}

func (s *localBlobStore) readReferenceSource(ctx context.Context, q sqlExecer, op OperationContext, input AcquireResourceReferencesInput, bindingHash Hash) (AcquireResourceReferencesResult, bool, error) {
	var site, object, revision, digest, storedBinding, state string
	var version int
	var rawInput, rawResult, pinID, expires, rawHash sql.NullString
	var createdAt string
	err := q.QueryRowContext(ctx, `SELECT site_id,source_object_id,source_revision_id,source_version,reference_digest,binding_hash,state,CASE WHEN length(CAST(input_json AS BLOB))<=16777216 THEN input_json ELSE NULL END,pin_id,expires_at,CASE WHEN length(CAST(result_json AS BLOB))<=16777216 THEN result_json ELSE NULL END,result_hash,created_at FROM reference_sources WHERE source_service=? AND data_epoch=? AND source_operation_id=?`, op.CallerService, op.DataEpoch, input.SourceOperationId).Scan(&site, &object, &revision, &version, &digest, &storedBinding, &state, &rawInput, &pinID, &expires, &rawResult, &rawHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AcquireResourceReferencesResult{}, false, nil
	}
	if err != nil {
		return AcquireResourceReferencesResult{}, false, mapMediaDBError(err, "读取引用来源绑定失败")
	}
	if site != input.SiteId || object != input.SourceObjectId || revision != input.SourceRevisionId || version != input.SourceVersion || digest != input.ReferenceDigest || storedBinding != bindingHash {
		return AcquireResourceReferencesResult{}, true, platform.NewError(platform.CodeConflict, false, "来源操作已经绑定其他完整材料")
	}
	if state == "sealed" {
		return AcquireResourceReferencesResult{}, true, platform.NewError(platform.CodeConflict, false, "来源已明确终止封存")
	}
	expectedInput, err := platform.CanonicalJSON(input)
	if err != nil {
		return AcquireResourceReferencesResult{}, true, err
	}
	var result AcquireResourceReferencesResult
	if (state != "pending" && state != "confirmed") || !rawInput.Valid || rawInput.String != string(expectedInput) || !pinID.Valid || !expires.Valid || !rawResult.Valid || !rawHash.Valid || platform.HashBytes([]byte(rawResult.String)) != rawHash.String || s.validatePin == nil || s.validatePin(json.RawMessage(rawResult.String)) != nil || json.Unmarshal([]byte(rawResult.String), &result) != nil || result.Pin.PinId != pinID.String || result.Pin.ExpiresAt.UTC().Format(time.RFC3339Nano) != expires.String {
		return result, true, platform.NewError(platform.CodeIntegrityError, false, "引用原回执或来源材料损坏")
	}
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil || result.Pin.ExpiresAt.Before(created.Add(24*time.Hour)) {
		return result, true, platform.NewError(platform.CodeIntegrityError, false, "原pin期限不足保护窗口")
	}
	_, _, assets, err := canonicalReferenceInput(op, input)
	if err != nil {
		return result, true, err
	}
	if !slices.Equal(result.Pin.Assets, assets) {
		return result, true, platform.NewError(platform.CodeIntegrityError, false, "引用原pin资产集合与来源不符")
	}
	var count, maximumFence int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(MAX(deletion_fence),0) FROM reference_pin_assets WHERE pin_id=?`, result.Pin.PinId).Scan(&count, &maximumFence); err != nil {
		return result, true, mapMediaDBError(err, "读取原pin对象保护失败")
	}
	if count != len(assets) || maximumFence != result.Pin.DeletionFence {
		return result, true, platform.NewError(platform.CodeIntegrityError, false, "原pin对象保护不完整")
	}
	for _, asset := range assets {
		var fence int
		err := q.QueryRowContext(ctx, `SELECT deletion_fence FROM reference_pin_assets WHERE pin_id=? AND asset_id=?`, result.Pin.PinId, asset.AssetId).Scan(&fence)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && fence <= 0) {
			return result, true, platform.NewError(platform.CodeIntegrityError, false, "原pin缺逐资产保护")
		}
		if err != nil {
			return result, true, mapMediaDBError(err, "读取原pin逐资产保护失败")
		}
	}
	return result, true, nil
}

func (s *localBlobStore) readReferenceAsset(ctx context.Context, q sqlExecer, siteID Identifier, expected gen.AssetVersionRef) (int, error) {
	var site, sha, mediaType, state string
	var size int
	var raw sql.NullString
	var fence, generation sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT a.site_id,a.sha256,a.byte_size,a.media_type,a.state,CASE WHEN length(CAST(a.asset_json AS BLOB))<=16777216 THEN a.asset_json ELSE NULL END,r.deletion_fence,r.reference_generation FROM assets a LEFT JOIN reference_objects r ON r.asset_id=a.asset_id WHERE a.asset_id=?`, expected.AssetId).Scan(&site, &sha, &size, &mediaType, &state, &raw, &fence, &generation)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, platform.NewError(platform.CodeNotFound, false, "被引用资源不存在")
	}
	if err != nil {
		return 0, mapMediaDBError(err, "读取被引用资源失败")
	}
	if site != siteID {
		return 0, platform.NewError(platform.CodeForbidden, false, "引用资源不属于来源站点")
	}
	var asset gen.Asset
	if !raw.Valid || s.validateAsset(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), &asset) != nil || asset.Ref.AssetId != expected.AssetId || asset.SiteId != site || asset.Ref.Sha256 != sha || asset.Ref.ByteSize != size || asset.Ref.MediaType != mediaType || string(asset.State) != state || !fence.Valid || fence.Int64 < 0 || !generation.Valid || generation.Int64 < 0 {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "引用资源元数据或删除fence损坏")
	}
	if fence.Int64 == math.MaxInt64 || generation.Int64 == math.MaxInt64 {
		return 0, platform.NewError(platform.CodeLocked, false, "资源删除fence或引用generation已耗尽")
	}
	if asset.State != gen.AssetStateReady {
		return 0, platform.NewError(platform.CodeLocked, false, "资源尚非ready或已经进入删除保护")
	}
	if asset.Ref != expected {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "引用资源版本、哈希或安全元数据不符")
	}
	return int(fence.Int64), nil
}

func (s *localBlobStore) acquireReferencePin(ctx context.Context, op OperationContext, input AcquireResourceReferencesInput, bindingHash Hash, assets []gen.AssetVersionRef) (AcquireResourceReferencesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	original, found, err := s.readReferenceSource(ctx, s.db, op, input, bindingHash)
	if err != nil || found {
		return original, err
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return AcquireResourceReferencesResult{}, err
	}
	defer cancel()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return AcquireResourceReferencesResult{}, mapMediaDBError(err, "开始引用预留事务失败")
	}
	defer tx.Rollback()
	original, found, err = s.readReferenceSource(work, tx, op, input, bindingHash)
	if err != nil || found {
		return original, err
	}
	fences := make([]int, len(assets))
	maximumFence := 0
	for i, asset := range assets {
		fence, err := s.readReferenceAsset(work, tx, input.SiteId, asset)
		if err != nil {
			return AcquireResourceReferencesResult{}, err
		}
		changed, err := tx.ExecContext(work, `UPDATE reference_objects SET deletion_fence=deletion_fence+1,reference_generation=reference_generation+1 WHERE asset_id=? AND deletion_fence=? AND reference_generation<9223372036854775807`, asset.AssetId, fence)
		if err != nil {
			return AcquireResourceReferencesResult{}, mapMediaDBError(err, "更新引用保护fence失败")
		}
		count, err := changed.RowsAffected()
		if err != nil || count != 1 {
			return AcquireResourceReferencesResult{}, platform.NewError(platform.CodeLocked, false, "引用保护fence发生竞争或generation耗尽")
		}
		fences[i] = fence + 1
		maximumFence = max(maximumFence, fences[i])
	}
	now := time.Now().UTC()
	result := AcquireResourceReferencesResult{Pin: gen.ReferencePin{PinId: platform.NewIdentifier(), Assets: assets, ExpiresAt: now.Add(24 * time.Hour), DeletionFence: maximumFence}}
	inputRaw, err := platform.CanonicalJSON(input)
	if err != nil {
		return AcquireResourceReferencesResult{}, err
	}
	resultRaw, err := platform.CanonicalJSON(result)
	if err != nil {
		return AcquireResourceReferencesResult{}, err
	}
	_, err = tx.ExecContext(work, `INSERT INTO reference_sources(source_service,data_epoch,source_operation_id,site_id,source_object_id,source_revision_id,source_version,reference_digest,binding_hash,input_json,state,pin_id,expires_at,result_json,result_hash,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,'pending',?,?,?,?,?)`, op.CallerService, op.DataEpoch, input.SourceOperationId, input.SiteId, input.SourceObjectId, input.SourceRevisionId, input.SourceVersion, input.ReferenceDigest, bindingHash, string(inputRaw), result.Pin.PinId, result.Pin.ExpiresAt.Format(time.RFC3339Nano), string(resultRaw), platform.HashBytes(resultRaw), now.Format(time.RFC3339Nano))
	if err != nil {
		return AcquireResourceReferencesResult{}, mapMediaDBError(err, "保存完整引用来源及原pin失败")
	}
	for i, asset := range assets {
		if _, err := tx.ExecContext(work, `INSERT INTO reference_pin_assets(pin_id,asset_id,deletion_fence) VALUES(?,?,?)`, result.Pin.PinId, asset.AssetId, fences[i]); err != nil {
			return AcquireResourceReferencesResult{}, mapMediaDBError(err, "保存逐资源pin保护失败")
		}
	}
	if err := tx.Commit(); err != nil {
		return AcquireResourceReferencesResult{}, mapMediaDBError(err, "提交引用预留失败")
	}
	return result, nil
}
