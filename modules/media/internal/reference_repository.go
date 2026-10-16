package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
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

// referenceRecord 只承载本域持久来源，不从远端payload推测pin存在性。
type referenceRecord struct {
	Material    AcquireResourceReferencesInput
	Input       *AcquireResourceReferencesInput
	BindingHash Hash
	State       string
	PinID       Identifier
	Reason      string
	Assets      []gen.AssetVersionRef
}

func (s *localBlobStore) readReferenceRecord(ctx context.Context, q sqlExecer, op OperationContext, sourceOperationID Identifier) (referenceRecord, bool, error) {
	record := referenceRecord{Material: AcquireResourceReferencesInput{SourceOperationId: sourceOperationID}, Assets: []gen.AssetVersionRef{}}
	var raw, pin, reason sql.NullString
	err := q.QueryRowContext(ctx, `SELECT site_id,source_object_id,source_revision_id,source_version,reference_digest,binding_hash,state,pin_id,sealed_reason,CASE WHEN length(CAST(input_json AS BLOB))<=16777216 THEN input_json ELSE NULL END FROM reference_sources WHERE source_service=? AND data_epoch=? AND source_operation_id=?`, op.CallerService, op.DataEpoch, sourceOperationID).Scan(&record.Material.SiteId, &record.Material.SourceObjectId, &record.Material.SourceRevisionId, &record.Material.SourceVersion, &record.Material.ReferenceDigest, &record.BindingHash, &record.State, &pin, &reason, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return record, false, nil
	}
	if err != nil {
		return record, false, mapMediaDBError(err, "读取持久引用来源失败")
	}
	expected, err := referenceBindingHash(op, record.Material)
	if err != nil {
		return record, true, err
	}
	if expected != record.BindingHash || (record.State != "pending" && record.State != "confirmed" && record.State != "sealed") {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "持久引用来源绑定损坏")
	}
	if pin.Valid {
		record.PinID = pin.String
	}
	if reason.Valid {
		record.Reason = reason.String
	}
	if record.State == "sealed" {
		if record.Reason != "source_aborted" && record.Reason != "source_rejected" {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "来源封存缺明确终止原因")
		}
	} else if reason.Valid || !pin.Valid || !raw.Valid {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "引用来源状态与pin材料不符")
	}
	if !pin.Valid {
		if raw.Valid || record.State != "sealed" {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "无pin来源未被完整封存")
		}
		return record, true, nil
	}
	var input AcquireResourceReferencesInput
	if !raw.Valid || s.validateReferenceInput == nil || s.validateReferenceInput(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), &input) != nil {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "持久引用完整输入损坏")
	}
	canonical, binding, assets, err := canonicalReferenceInput(op, input)
	if err != nil {
		return record, true, err
	}
	encoded, err := platform.CanonicalJSON(canonical)
	if err != nil {
		return record, true, err
	}
	if binding != record.BindingHash || string(encoded) != raw.String {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "持久引用输入与来源绑定不符")
	}
	record.Input = &canonical
	record.Assets = assets
	if record.State != "sealed" {
		original, found, err := s.readReferenceSource(ctx, q, op, canonical, binding)
		if err != nil {
			return record, true, err
		}
		if !found || original.Pin.PinId != record.PinID {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "持久来源与原pin回执不符")
		}
	}
	return record, true, nil
}

func readReferenceResolution(ctx context.Context, q sqlExecer, op OperationContext, record referenceRecord, kind string) error {
	var raw, digest, resolved sql.NullString
	if err := q.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(resolution_json AS BLOB))<=1024 THEN resolution_json ELSE NULL END,resolution_hash,resolved_at FROM reference_sources WHERE source_service=? AND data_epoch=? AND source_operation_id=?`, op.CallerService, op.DataEpoch, record.Material.SourceOperationId).Scan(&raw, &digest, &resolved); err != nil {
		return mapMediaDBError(err, "读取引用终局原回执失败")
	}
	expected := `{"confirmed":true}`
	if kind == "release" {
		expected = `{"released":true}`
	}
	if !raw.Valid || raw.String != expected || !digest.Valid || platform.HashBytes([]byte(raw.String)) != digest.String || !resolved.Valid || (kind == "confirm" && record.State != "confirmed") || (kind == "release" && record.State != "sealed") {
		return platform.NewError(platform.CodeIntegrityError, false, "引用终局原回执缺失或损坏")
	}
	if _, err := time.Parse(time.RFC3339Nano, resolved.String); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "引用终局原回执时刻损坏")
	}
	return nil
}

func writeReferenceResolution(ctx context.Context, tx *sql.Tx, op OperationContext, record referenceRecord, kind string) error {
	var value any = ConfirmResourceReferencesResult{Confirmed: true}
	state := "confirmed"
	if kind == "release" {
		value = ReleaseResourceReferencesResult{Released: true}
		state = "sealed"
	}
	raw, err := platform.CanonicalJSON(value)
	if err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE reference_sources SET resolution_json=?,resolution_hash=?,resolved_at=? WHERE source_service=? AND data_epoch=? AND source_operation_id=? AND state=? AND resolution_json IS NULL AND resolution_hash IS NULL AND resolved_at IS NULL`, string(raw), platform.HashBytes(raw), platform.Now(), op.CallerService, op.DataEpoch, record.Material.SourceOperationId, state)
	if err != nil {
		return mapMediaDBError(err, "写入引用终局原回执失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "引用终局原回执发生竞争或材料不完整")
	}
	return nil
}

func (s *localBlobStore) referenceObjectAssets(ctx context.Context, op OperationContext, input AcquireResourceReferencesInput, assets []gen.AssetVersionRef) ([]gen.AssetVersionRef, error) {
	byID := make(map[string]gen.AssetVersionRef)
	for _, asset := range assets {
		byID[asset.AssetId] = asset
	}
	var operationID, revisionID, digest string
	var version int
	err := s.db.QueryRowContext(ctx, `SELECT source_operation_id,source_revision_id,source_version,reference_digest FROM reference_heads WHERE source_service=? AND data_epoch=? AND site_id=? AND source_object_id=?`, op.CallerService, op.DataEpoch, input.SiteId, input.SourceObjectId).Scan(&operationID, &revisionID, &version, &digest)
	hasHead := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, mapMediaDBError(err, "读取旧当前引用水位失败")
	}
	var currentCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_entries e JOIN reference_sources s ON s.pin_id=e.pin_id WHERE s.source_service=? AND s.data_epoch=? AND s.site_id=? AND s.source_object_id=? AND e.kind='current'`, op.CallerService, op.DataEpoch, input.SiteId, input.SourceObjectId).Scan(&currentCount); err != nil {
		return nil, mapMediaDBError(err, "读取旧当前引用完整性失败")
	}
	if hasHead {
		head, found, err := s.readReferenceRecord(ctx, s.db, op, operationID)
		if err != nil {
			return nil, err
		}
		if !found || head.State != "confirmed" || head.Input == nil || head.Material.SiteId != input.SiteId || head.Material.SourceObjectId != input.SourceObjectId || head.Material.SourceRevisionId != revisionID || head.Material.SourceVersion != version || head.Material.ReferenceDigest != digest || currentCount != len(head.Input.References) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "旧当前引用水位或完整集合损坏")
		}
		if err := readReferenceResolution(ctx, s.db, op, head, "confirm"); err != nil {
			return nil, err
		}
		for _, ref := range head.Input.References {
			var kind string
			err := s.db.QueryRowContext(ctx, `SELECT kind FROM reference_entries WHERE pin_id=? AND reference_id=? AND asset_id=?`, head.PinID, ref.ReferenceId, ref.Asset.AssetId).Scan(&kind)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && kind != "current") {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "旧当前引用缺少完整原集合")
			}
			if err != nil {
				return nil, mapMediaDBError(err, "读取旧当前引用项失败")
			}
		}
		for _, asset := range head.Assets {
			if _, found := byID[asset.AssetId]; !found {
				byID[asset.AssetId] = asset
			}
		}
	} else if currentCount != 0 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "没有对象水位却存在当前引用")
	}
	result := make([]gen.AssetVersionRef, 0, len(byID))
	for _, asset := range byID {
		result = append(result, asset)
	}
	slices.SortFunc(result, func(a, b gen.AssetVersionRef) int { return strings.Compare(a.AssetId, b.AssetId) })
	return result, nil
}

func advanceReferenceGeneration(ctx context.Context, tx *sql.Tx, assetID Identifier, bumpFence bool) error {
	var fence, generation int64
	err := tx.QueryRowContext(ctx, `SELECT deletion_fence,reference_generation FROM reference_objects WHERE asset_id=?`, assetID).Scan(&fence, &generation)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (fence < 0 || generation < 0)) {
		return platform.NewError(platform.CodeIntegrityError, false, "引用对象保护水位缺失或损坏")
	}
	if err != nil {
		return mapMediaDBError(err, "读取引用对象水位失败")
	}
	if generation == math.MaxInt64 || (bumpFence && fence == math.MaxInt64) {
		return platform.NewError(platform.CodeLocked, false, "引用对象保护水位已耗尽")
	}
	increment := 0
	if bumpFence {
		increment = 1
	}
	changed, err := tx.ExecContext(ctx, `UPDATE reference_objects SET deletion_fence=deletion_fence+?,reference_generation=reference_generation+1 WHERE asset_id=? AND deletion_fence=? AND reference_generation=?`, increment, assetID, fence, generation)
	if err != nil {
		return mapMediaDBError(err, "更新引用对象保护水位失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeLocked, false, "引用对象保护CAS冲突")
	}
	return nil
}

func (s *localBlobStore) confirmReferencePin(ctx context.Context, op OperationContext, record referenceRecord) (ConfirmResourceReferencesResult, error) {
	material := record.Material
	releaseSource := s.lockReferenceKey("source\x00" + op.CallerService + "\x00" + op.DataEpoch + "\x00" + material.SourceOperationId)
	defer releaseSource()
	releaseObject := s.lockReferenceKey("object\x00" + op.CallerService + "\x00" + op.DataEpoch + "\x00" + material.SiteId + "\x00" + material.SourceObjectId)
	defer releaseObject()
	affected, err := s.referenceObjectAssets(ctx, op, material, record.Assets)
	if err != nil {
		return ConfirmResourceReferencesResult{}, err
	}
	releaseAssets := s.lockReferenceAssets(affected)
	defer releaseAssets()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "开始引用确认事务失败")
	}
	defer tx.Rollback()
	current, found, err := s.readReferenceRecord(ctx, tx, op, material.SourceOperationId)
	if err != nil {
		return ConfirmResourceReferencesResult{}, err
	}
	if !found || current.BindingHash != record.BindingHash || current.PinID != record.PinID {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "确认期间来源绑定改变")
	}
	if current.State == "sealed" {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "封存来源与已核COMMITTED决定矛盾")
	}
	if current.State == "confirmed" {
		if err := readReferenceResolution(ctx, tx, op, current, "confirm"); err != nil {
			return ConfirmResourceReferencesResult{}, err
		}
		return ConfirmResourceReferencesResult{Confirmed: true}, nil
	}
	if current.Input == nil {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "待确认来源缺完整引用集合")
	}
	for _, asset := range current.Assets {
		if _, err := s.readReferenceAsset(ctx, tx, material.SiteId, asset); err != nil {
			return ConfirmResourceReferencesResult{}, err
		}
	}
	var headOperation, headRevision, headDigest, sourceState, sourceSite, sourceObject, sourceRevision, sourceDigest string
	var headVersion, sourceVersion int
	err = tx.QueryRowContext(ctx, `SELECT h.source_operation_id,h.source_revision_id,h.source_version,h.reference_digest,s.state,s.site_id,s.source_object_id,s.source_version,s.source_revision_id,s.reference_digest FROM reference_heads h JOIN reference_sources s ON s.source_service=h.source_service AND s.data_epoch=h.data_epoch AND s.source_operation_id=h.source_operation_id WHERE h.source_service=? AND h.data_epoch=? AND h.site_id=? AND h.source_object_id=?`, op.CallerService, op.DataEpoch, material.SiteId, material.SourceObjectId).Scan(&headOperation, &headRevision, &headVersion, &headDigest, &sourceState, &sourceSite, &sourceObject, &sourceVersion, &sourceRevision, &sourceDigest)
	hasHead := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "读取对象当前引用水位失败")
	}
	if hasHead && (sourceState != "confirmed" || sourceSite != material.SiteId || sourceObject != material.SourceObjectId || sourceVersion != headVersion || sourceRevision != headRevision || sourceDigest != headDigest || headVersion < 0) {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "对象当前引用水位与持久来源矛盾")
	}
	if hasHead && headVersion == material.SourceVersion {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "同一对象水位绑定了不同确认来源")
	}
	kind := "history"
	advancing := !hasHead || material.SourceVersion > headVersion
	if advancing {
		kind = "current"
		if _, err := tx.ExecContext(ctx, `UPDATE reference_entries SET kind='history' WHERE kind='current' AND pin_id IN (SELECT pin_id FROM reference_sources WHERE source_service=? AND data_epoch=? AND site_id=? AND source_object_id=?)`, op.CallerService, op.DataEpoch, material.SiteId, material.SourceObjectId); err != nil {
			return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "保留旧当前引用为历史失败")
		}
	}
	var existingEntries int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_entries WHERE pin_id=?`, current.PinID).Scan(&existingEntries); err != nil {
		return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "读取待确认引用完整性失败")
	}
	if existingEntries != 0 {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "待确认来源已经存在确认引用")
	}
	for _, ref := range current.Input.References {
		if _, err := tx.ExecContext(ctx, `INSERT INTO reference_entries(pin_id,reference_id,asset_id,kind) VALUES(?,?,?,?)`, current.PinID, ref.ReferenceId, ref.Asset.AssetId, kind); err != nil {
			return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "保存完整确认引用集合失败")
		}
	}
	changedAssets := current.Assets
	if advancing {
		changedAssets = affected
	}
	for _, asset := range changedAssets {
		if err := advanceReferenceGeneration(ctx, tx, asset.AssetId, true); err != nil {
			return ConfirmResourceReferencesResult{}, err
		}
	}
	changed, err := tx.ExecContext(ctx, `UPDATE reference_sources SET state='confirmed' WHERE source_service=? AND data_epoch=? AND source_operation_id=? AND binding_hash=? AND state='pending'`, op.CallerService, op.DataEpoch, material.SourceOperationId, current.BindingHash)
	if err != nil {
		return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "确认来源CAS失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count != 1 {
		return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "来源确认状态已改变")
	}
	if advancing {
		if !hasHead {
			_, err = tx.ExecContext(ctx, `INSERT INTO reference_heads(source_service,data_epoch,site_id,source_object_id,source_operation_id,source_revision_id,source_version,reference_digest) VALUES(?,?,?,?,?,?,?,?)`, op.CallerService, op.DataEpoch, material.SiteId, material.SourceObjectId, material.SourceOperationId, material.SourceRevisionId, material.SourceVersion, material.ReferenceDigest)
		} else {
			var updated sql.Result
			updated, err = tx.ExecContext(ctx, `UPDATE reference_heads SET source_operation_id=?,source_revision_id=?,source_version=?,reference_digest=? WHERE source_service=? AND data_epoch=? AND site_id=? AND source_object_id=? AND source_version=? AND source_operation_id=?`, material.SourceOperationId, material.SourceRevisionId, material.SourceVersion, material.ReferenceDigest, op.CallerService, op.DataEpoch, material.SiteId, material.SourceObjectId, headVersion, headOperation)
			if err == nil {
				matched, scanErr := updated.RowsAffected()
				if scanErr != nil || matched != 1 {
					return ConfirmResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "对象引用水位CAS冲突")
				}
			}
		}
		if err != nil {
			return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "推进对象完整引用水位失败")
		}
	}
	if err := writeReferenceResolution(ctx, tx, op, current, "confirm"); err != nil {
		return ConfirmResourceReferencesResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ConfirmResourceReferencesResult{}, mapMediaDBError(err, "提交引用确认失败")
	}
	return ConfirmResourceReferencesResult{Confirmed: true}, nil
}

func (s *localBlobStore) releaseReferencePin(ctx context.Context, op OperationContext, input ReleaseResourceReferencesInput, material AcquireResourceReferencesInput, bindingHash Hash) (ReleaseResourceReferencesResult, error) {
	releaseSource := s.lockReferenceKey("source\x00" + op.CallerService + "\x00" + op.DataEpoch + "\x00" + material.SourceOperationId)
	defer releaseSource()
	releaseObject := s.lockReferenceKey("object\x00" + op.CallerService + "\x00" + op.DataEpoch + "\x00" + material.SiteId + "\x00" + material.SourceObjectId)
	defer releaseObject()
	record, found, err := s.readReferenceRecord(ctx, s.db, op, material.SourceOperationId)
	if err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	if found && record.BindingHash != bindingHash {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "释放来源完整材料冲突")
	}
	if input.PinId != nil && (!found || record.PinID != *input.PinId) {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "释放已知pin与真实来源不符")
	}
	releaseAssets := s.lockReferenceAssets(record.Assets)
	defer releaseAssets()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReleaseResourceReferencesResult{}, mapMediaDBError(err, "开始来源封存事务失败")
	}
	defer tx.Rollback()
	current, present, err := s.readReferenceRecord(ctx, tx, op, material.SourceOperationId)
	if err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	if present != found || (present && (current.BindingHash != bindingHash || current.PinID != record.PinID)) {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "来源封存期间绑定改变")
	}
	if present && current.State == "confirmed" {
		return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "已确认的当前或历史引用不得释放")
	}
	if present && current.State == "sealed" {
		if current.Reason != string(input.Reason) {
			return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "来源终止原因冲突")
		}
		if err := readReferenceResolution(ctx, tx, op, current, "release"); err != nil {
			return ReleaseResourceReferencesResult{}, err
		}
		return ReleaseResourceReferencesResult{Released: true}, nil
	}
	if present {
		var entries int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_entries WHERE pin_id=?`, current.PinID).Scan(&entries); err != nil {
			return ReleaseResourceReferencesResult{}, mapMediaDBError(err, "读取来源确认引用失败")
		}
		if entries != 0 {
			return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeIntegrityError, false, "待释放来源已存在确认引用，保留保护")
		}
		for _, asset := range current.Assets {
			if err := advanceReferenceGeneration(ctx, tx, asset.AssetId, false); err != nil {
				return ReleaseResourceReferencesResult{}, err
			}
		}
		changed, err := tx.ExecContext(ctx, `UPDATE reference_sources SET state='sealed',sealed_reason=? WHERE source_service=? AND data_epoch=? AND source_operation_id=? AND binding_hash=? AND state='pending'`, input.Reason, op.CallerService, op.DataEpoch, material.SourceOperationId, bindingHash)
		if err != nil {
			return ReleaseResourceReferencesResult{}, mapMediaDBError(err, "封存来源CAS失败")
		}
		count, err := changed.RowsAffected()
		if err != nil || count != 1 {
			return ReleaseResourceReferencesResult{}, platform.NewError(platform.CodeConflict, false, "来源封存状态竞争")
		}
	} else {
		current = referenceRecord{Material: material, BindingHash: bindingHash, State: "sealed", Reason: string(input.Reason)}
		if _, err := tx.ExecContext(ctx, `INSERT INTO reference_sources(source_service,data_epoch,source_operation_id,site_id,source_object_id,source_revision_id,source_version,reference_digest,binding_hash,state,sealed_reason,created_at) VALUES(?,?,?,?,?,?,?,?,?,'sealed',?,?)`, op.CallerService, op.DataEpoch, material.SourceOperationId, material.SiteId, material.SourceObjectId, material.SourceRevisionId, material.SourceVersion, material.ReferenceDigest, bindingHash, input.Reason, platform.Now()); err != nil {
			return ReleaseResourceReferencesResult{}, mapMediaDBError(err, "持久封存无pin来源失败")
		}
	}
	if err := writeReferenceResolution(ctx, tx, op, current, "release"); err != nil {
		return ReleaseResourceReferencesResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReleaseResourceReferencesResult{}, mapMediaDBError(err, "提交来源封存失败")
	}
	return ReleaseResourceReferencesResult{Released: true}, nil
}
