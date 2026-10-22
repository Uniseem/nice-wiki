package media

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

type ProposeManualCleanupInput = gen.ProposeManualCleanupInput
type ProposeManualCleanupResult = gen.ProposeManualCleanupResult
type ConfirmManualCleanupInput struct {
	PlanId                 Identifier   `json:"planId"`
	ConfirmationHash       Hash         `json:"confirmationHash"`
	AcknowledgeHistoryLoss bool         `json:"acknowledgeHistoryLoss"`
	ExpectedRevision       gen.Revision `json:"expectedRevision"`
}
type ConfirmManualCleanupResult = gen.ConfirmManualCleanupResult

type CancelManualCleanupInput struct {
	PlanId Identifier `json:"planId"`
}
type CancelManualCleanupResult = gen.CancelManualCleanupResult

func cleanupStore(op OperationContext, requireAdmin bool) (*localBlobStore, error) {
	admin := op.Actor.Kind == "admin" && op.Actor.SessionID != "" && (hasGrant(op.Actor.Grants, "media.manage") || hasGrant(op.Actor.Grants, "superadmin"))
	worker := !requireAdmin && op.CallerService == "media" && op.Actor.Kind == "service" && op.Actor.SubjectID == "media"
	if !admin && !worker {
		return nil, platform.NewError(platform.CodeForbidden, false, "没有管理媒体资源的权限")
	}
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体清理存储未装配")
	}
	if op.DataEpoch != s.openedEpoch {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "清理不属于已打开数据代次")
	}
	return s, nil
}

func (s *localBlobStore) readCleanupAsset(ctx context.Context, q sqlExecer, siteID, assetID Identifier) (gen.Asset, int, int, error) {
	var raw sql.NullString
	var site, sha, mediaType, state string
	var size int
	var fence, generation sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT a.site_id,a.sha256,a.byte_size,a.media_type,a.state,CASE WHEN length(CAST(a.asset_json AS BLOB))<=16777216 THEN a.asset_json ELSE NULL END,r.deletion_fence,r.reference_generation FROM assets a LEFT JOIN reference_objects r ON r.asset_id=a.asset_id WHERE a.asset_id=?`, assetID).Scan(&site, &sha, &size, &mediaType, &state, &raw, &fence, &generation)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.Asset{}, 0, 0, platform.NewError(platform.CodeNotFound, false, "清理资源不存在")
	}
	if err != nil {
		return gen.Asset{}, 0, 0, mapMediaDBError(err, "读取清理资源失败")
	}
	if site != siteID {
		return gen.Asset{}, 0, 0, platform.NewError(platform.CodeForbidden, false, "清理资源不属于当前站点")
	}
	var asset gen.Asset
	if !raw.Valid || s.validateAsset(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), &asset) != nil || asset.Ref.AssetId != assetID || asset.SiteId != site || asset.Ref.Sha256 != sha || asset.Ref.ByteSize != size || asset.Ref.MediaType != mediaType || string(asset.State) != state || !fence.Valid || fence.Int64 < 0 || !generation.Valid || generation.Int64 < 0 {
		return asset, 0, 0, platform.NewError(platform.CodeIntegrityError, false, "清理资源元数据或对象水位损坏")
	}
	if state != "ready" && state != "delete_pending" {
		return asset, 0, 0, platform.NewError(platform.CodeLocked, false, "资源已非可清理或可撤销阶段")
	}
	return asset, int(fence.Int64), int(generation.Int64), nil
}

func cleanupRequestHash(op OperationContext, kind string, input any) (Hash, error) {
	raw, err := platform.CanonicalJSON(struct {
		Kind   string     `json:"kind"`
		Caller string     `json:"caller"`
		Actor  string     `json:"actor"`
		Site   Identifier `json:"siteId"`
		Epoch  Identifier `json:"dataEpoch"`
		Input  any        `json:"input"`
	}{kind, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, input})
	if err != nil {
		return "", err
	}
	return platform.HashBytes(raw), nil
}

func (s *localBlobStore) readCleanupReceipt(ctx context.Context, q sqlExecer, op OperationContext, requestHash Hash, kind string, target any) (bool, error) {
	var storedHash, storedKind, rawHash string
	var raw sql.NullString
	err := q.QueryRowContext(ctx, `SELECT request_hash,kind,CASE WHEN length(CAST(result_json AS BLOB))<=16777216 THEN result_json ELSE NULL END,result_hash FROM manual_cleanup_receipts WHERE operation_id=?`, op.OperationID).Scan(&storedHash, &storedKind, &raw, &rawHash)
	if errors.Is(err, sql.ErrNoRows) {
		var count int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM manual_cleanup_approvals WHERE operation_id=?`, op.OperationID).Scan(&count); err != nil {
			return false, mapMediaDBError(err, "核对清理操作归属失败")
		}
		if count != 0 {
			return true, platform.NewError(platform.CodeConflict, false, "操作已绑定人工清理批准")
		}
		return false, nil
	}
	if err != nil {
		return false, mapMediaDBError(err, "读取人工清理原回执失败")
	}
	if storedHash != requestHash || storedKind != kind {
		return true, platform.NewError(platform.CodeConflict, false, "清理操作已绑定其他材料")
	}
	if !raw.Valid || platform.HashBytes([]byte(raw.String)) != rawHash || json.Unmarshal([]byte(raw.String), target) != nil {
		return true, platform.NewError(platform.CodeIntegrityError, false, "清理原回执损坏")
	}
	canonical, err := platform.CanonicalJSON(target)
	if err != nil || string(canonical) != raw.String {
		return true, platform.NewError(platform.CodeIntegrityError, false, "清理原回执不是原规范结果")
	}
	if kind == "propose" {
		result := target.(*ProposeManualCleanupResult)
		plan, err := platform.CanonicalJSON(result.Plan)
		if err != nil || s.validateCleanupPlan(plan) != nil || string(result.Plan.State) != "proposed" || result.Plan.Revision != 0 {
			return true, platform.NewError(platform.CodeIntegrityError, false, "原提案计划不符合唯一schema")
		}
	} else if !target.(*CancelManualCleanupResult).Cancelled {
		return true, platform.NewError(platform.CodeIntegrityError, false, "原取消回执不是成功结果")
	}
	return true, nil
}

func writeCleanupReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, requestHash Hash, kind string, planID Identifier, value any) error {
	raw, err := platform.CanonicalJSON(value)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO manual_cleanup_receipts(operation_id,request_hash,kind,plan_id,result_json,result_hash,created_at) VALUES(?,?,?,?,?,?,?)`, op.OperationID, requestHash, kind, planID, string(raw), platform.HashBytes(raw), platform.Now())
	if err != nil {
		return mapMediaDBError(err, "写入人工清理原回执失败")
	}
	return nil
}

type cleanupScan struct {
	Asset    gen.AssetVersionRef   `json:"asset"`
	Fence    int                   `json:"deletionFence"`
	Snapshot gen.ReferenceSnapshot `json:"snapshot"`
}

// ProposeManualCleanup 先完成无锁权威扫描，再用本地水位CAS持久精确风险材料。
func ProposeManualCleanup(ctx context.Context, op OperationContext, input ProposeManualCleanupInput) (ProposeManualCleanupResult, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return ProposeManualCleanupResult{}, err
	}
	requestHash, err := cleanupRequestHash(op, "propose", input)
	if err != nil {
		return ProposeManualCleanupResult{}, err
	}
	var original ProposeManualCleanupResult
	if found, err := s.readCleanupReceipt(ctx, s.db, op, requestHash, "propose", &original); found || err != nil {
		return original, err
	}
	assets := append([]gen.AssetVersionRef{}, input.Assets...)
	slices.SortFunc(assets, func(a, b gen.AssetVersionRef) int { return strings.Compare(a.AssetId, b.AssetId) })
	scans := make([]cleanupScan, 0, len(assets))
	history := make(map[string]bool)
	maxGeneration := 0
	totalScanBytes := 0
	for i, expected := range assets {
		if i > 0 && assets[i-1].AssetId == expected.AssetId {
			return ProposeManualCleanupResult{}, platform.NewError(platform.CodeValidation, false, "人工选择的资源不得重复")
		}
		asset, fence, generation, err := s.readCleanupAsset(ctx, s.db, op.Actor.SiteID, expected.AssetId)
		if err != nil {
			return ProposeManualCleanupResult{}, err
		}
		if asset.Ref != expected || asset.State != gen.AssetStateReady {
			return ProposeManualCleanupResult{}, platform.NewError(platform.CodeConflict, false, "选择资源已变化或非ready")
		}
		read, err := ValidateCleanupSources(ctx, op, ValidateCleanupSourcesInput{AssetId: expected.AssetId, ExpectedGeneration: &generation})
		if err != nil {
			return ProposeManualCleanupResult{}, err
		}
		if !read.SafeToDelete {
			return ProposeManualCleanupResult{}, platform.NewError(platform.CodeLocked, false, "资源仍有当前引用或待提交保护")
		}
		for _, ref := range read.Snapshot.HistoryReferences {
			if ref.SourceService == gen.Content {
				history[ref.SourceObjectId] = true
			}
		}
		encoded, err := platform.CanonicalJSON(read.Snapshot)
		if err != nil {
			return ProposeManualCleanupResult{}, err
		}
		totalScanBytes += len(encoded)
		if totalScanBytes > 16<<20 {
			return ProposeManualCleanupResult{}, platform.NewError(platform.CodeValidation, false, "人工计划完整扫描材料超过16MiB，请分批选择")
		}
		maxGeneration = max(maxGeneration, generation)
		scans = append(scans, cleanupScan{expected, fence, read.Snapshot})
	}
	historyIDs := make([]Identifier, 0, len(history))
	for id := range history {
		historyIDs = append(historyIDs, id)
	}
	slices.Sort(historyIDs)
	material, err := platform.CanonicalJSON(struct {
		Site    Identifier    `json:"siteId"`
		Epoch   Identifier    `json:"dataEpoch"`
		Scans   []cleanupScan `json:"scans"`
		History []Identifier  `json:"historyAffectedDocumentIds"`
	}{op.Actor.SiteID, op.DataEpoch, scans, historyIDs})
	if err != nil {
		return ProposeManualCleanupResult{}, err
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return ProposeManualCleanupResult{}, err
	}
	defer cancel()
	releaseAssets := s.lockReferenceAssets(assets)
	defer releaseAssets()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return ProposeManualCleanupResult{}, mapMediaDBError(err, "开始人工计划事务失败")
	}
	defer tx.Rollback()
	if found, err := s.readCleanupReceipt(work, tx, op, requestHash, "propose", &original); found || err != nil {
		return original, err
	}
	for _, scan := range scans {
		asset, fence, generation, err := s.readCleanupAsset(work, tx, op.Actor.SiteID, scan.Asset.AssetId)
		if err != nil {
			return ProposeManualCleanupResult{}, err
		}
		if asset.Ref != scan.Asset || asset.State != gen.AssetStateReady || fence != scan.Fence || generation != scan.Snapshot.ReferenceGeneration {
			return ProposeManualCleanupResult{}, platform.NewError(platform.CodeVersionMismatch, false, "人工计划扫描后资源水位变化")
		}
		var protections, marks int
		if err := tx.QueryRowContext(work, `SELECT (SELECT COUNT(*) FROM asset_current_references WHERE asset_id=?)+(SELECT COUNT(*) FROM reference_pin_assets p JOIN reference_sources s ON s.pin_id=p.pin_id WHERE p.asset_id=? AND s.state='pending')`, scan.Asset.AssetId, scan.Asset.AssetId).Scan(&protections); err != nil {
			return ProposeManualCleanupResult{}, mapMediaDBError(err, "核对计划引用保护失败")
		}
		if err := tx.QueryRowContext(work, `SELECT COUNT(*) FROM manual_cleanup_marks WHERE asset_id=?`, scan.Asset.AssetId).Scan(&marks); err != nil {
			return ProposeManualCleanupResult{}, mapMediaDBError(err, "核对计划删除标记失败")
		}
		if protections != 0 || marks != 0 {
			return ProposeManualCleanupResult{}, platform.NewError(platform.CodeLocked, false, "人工计划存在新引用或删除标记")
		}
	}
	now := time.Now().UTC()
	plan := gen.CleanupPlan{PlanId: platform.NewIdentifier(), Assets: assets, HistoryAffectedDocumentIds: historyIDs, ReferenceGeneration: maxGeneration, ConfirmationHash: platform.HashBytes(material), NotBefore: now.Add(24 * time.Hour), State: gen.CleanupPlanState("proposed"), Revision: 0}
	rawPlan, err := platform.CanonicalJSON(plan)
	if err != nil {
		return ProposeManualCleanupResult{}, err
	}
	if s.validateCleanupPlan(rawPlan) != nil {
		return ProposeManualCleanupResult{}, platform.NewError(platform.CodeValidation, false, "精确人工计划超出登记预算")
	}
	if _, err := tx.ExecContext(work, `INSERT INTO manual_cleanup_plans(plan_id,data_epoch,site_id,created_by,revision,state,confirmation_hash,plan_json,plan_hash,created_at,updated_at) VALUES(?,?,?,?,?,'proposed',?,?,?,?,?)`, plan.PlanId, op.DataEpoch, op.Actor.SiteID, op.Actor.SubjectID, plan.Revision, plan.ConfirmationHash, string(rawPlan), platform.HashBytes(rawPlan), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return ProposeManualCleanupResult{}, mapMediaDBError(err, "保存人工清理计划失败")
	}
	for _, scan := range scans {
		raw, err := platform.CanonicalJSON(scan.Snapshot)
		if err != nil {
			return ProposeManualCleanupResult{}, err
		}
		if _, err := tx.ExecContext(work, `INSERT INTO manual_cleanup_assets(plan_id,asset_id,deletion_fence,reference_generation,snapshot_json,snapshot_hash) VALUES(?,?,?,?,?,?)`, plan.PlanId, scan.Asset.AssetId, scan.Fence, scan.Snapshot.ReferenceGeneration, string(raw), platform.HashBytes(raw)); err != nil {
			return ProposeManualCleanupResult{}, mapMediaDBError(err, "保存计划完整扫描材料失败")
		}
	}
	result := ProposeManualCleanupResult{Plan: plan}
	if err := writeCleanupReceipt(work, tx, op, requestHash, "propose", plan.PlanId, result); err != nil {
		return ProposeManualCleanupResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProposeManualCleanupResult{}, mapMediaDBError(err, "提交人工清理计划失败")
	}
	return result, nil
}

func (s *localBlobStore) readCleanupPlan(ctx context.Context, q sqlExecer, op OperationContext, planID Identifier) (gen.CleanupPlan, error) {
	var raw sql.NullString
	var epoch, site, state, confirmationHash, digest string
	var revision int
	err := q.QueryRowContext(ctx, `SELECT data_epoch,site_id,revision,state,confirmation_hash,CASE WHEN length(CAST(plan_json AS BLOB))<=16777216 THEN plan_json ELSE NULL END,plan_hash FROM manual_cleanup_plans WHERE plan_id=?`, planID).Scan(&epoch, &site, &revision, &state, &confirmationHash, &raw, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.CleanupPlan{}, platform.NewError(platform.CodeNotFound, false, "人工清理计划不存在")
	}
	if err != nil {
		return gen.CleanupPlan{}, mapMediaDBError(err, "读取人工计划失败")
	}
	if site != op.Actor.SiteID {
		return gen.CleanupPlan{}, platform.NewError(platform.CodeForbidden, false, "人工计划不属于当前站点")
	}
	if epoch != op.DataEpoch {
		return gen.CleanupPlan{}, platform.NewError(platform.CodeVersionMismatch, false, "人工计划不属于活动代次")
	}
	var plan gen.CleanupPlan
	if !raw.Valid || platform.HashBytes([]byte(raw.String)) != digest || s.validateCleanupPlan(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), &plan) != nil || plan.PlanId != planID || plan.Revision != revision || string(plan.State) != state || plan.ConfirmationHash != confirmationHash {
		return plan, platform.NewError(platform.CodeIntegrityError, false, "人工计划原材料或索引损坏")
	}
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM manual_cleanup_assets WHERE plan_id=?`, planID).Scan(&count); err != nil {
		return plan, mapMediaDBError(err, "读取计划完整资产数量失败")
	}
	if count != len(plan.Assets) {
		return plan, platform.NewError(platform.CodeIntegrityError, false, "计划资产集合缺失")
	}
	scans := make([]cleanupScan, 0, len(plan.Assets))
	history := make(map[string]bool)
	maxGeneration := 0
	totalScanBytes := 0
	for i, asset := range plan.Assets {
		if i > 0 && plan.Assets[i-1].AssetId >= asset.AssetId {
			return plan, platform.NewError(platform.CodeIntegrityError, false, "计划资产非唯一稳定顺序")
		}
		var snapshotRaw sql.NullString
		var hash string
		var fence, generation int
		err := q.QueryRowContext(ctx, `SELECT deletion_fence,reference_generation,CASE WHEN length(CAST(snapshot_json AS BLOB))<=16777216 THEN snapshot_json ELSE NULL END,snapshot_hash FROM manual_cleanup_assets WHERE plan_id=? AND asset_id=?`, planID, asset.AssetId).Scan(&fence, &generation, &snapshotRaw, &hash)
		if err != nil {
			return plan, mapMediaDBError(err, "读取计划逐资产扫描失败")
		}
		var snapshot gen.ReferenceSnapshot
		if !snapshotRaw.Valid || platform.HashBytes([]byte(snapshotRaw.String)) != hash || s.validateSnapshot(json.RawMessage(snapshotRaw.String)) != nil || json.Unmarshal([]byte(snapshotRaw.String), &snapshot) != nil || snapshot.AssetId != asset.AssetId || snapshot.ReferenceGeneration != generation || fence < 0 || len(snapshot.CurrentReferences) != 0 || len(snapshot.SourceWatermarks) != 2 || snapshot.SourceWatermarks[0].SourceService != "content" || snapshot.SourceWatermarks[1].SourceService != "site-config" || !snapshot.SourceWatermarks[0].Complete || !snapshot.SourceWatermarks[1].Complete {
			return plan, platform.NewError(platform.CodeIntegrityError, false, "计划完整扫描材料损坏")
		}
		totalScanBytes += len(snapshotRaw.String)
		if totalScanBytes > 16<<20 {
			return plan, platform.NewError(platform.CodeIntegrityError, false, "计划完整扫描材料超过持久预算")
		}
		for _, ref := range snapshot.HistoryReferences {
			if ref.Asset != asset || !ref.Confirmed || ref.ExpiresAt != nil || ref.Kind != gen.History {
				return plan, platform.NewError(platform.CodeIntegrityError, false, "计划历史引用不属于精确资产")
			}
			if ref.SourceService == gen.Content {
				history[ref.SourceObjectId] = true
			}
		}
		maxGeneration = max(maxGeneration, generation)
		scans = append(scans, cleanupScan{asset, fence, snapshot})
	}
	historyIDs := make([]Identifier, 0, len(history))
	for id := range history {
		historyIDs = append(historyIDs, id)
	}
	slices.Sort(historyIDs)
	material, err := platform.CanonicalJSON(struct {
		Site    Identifier    `json:"siteId"`
		Epoch   Identifier    `json:"dataEpoch"`
		Scans   []cleanupScan `json:"scans"`
		History []Identifier  `json:"historyAffectedDocumentIds"`
	}{op.Actor.SiteID, op.DataEpoch, scans, historyIDs})
	if err != nil {
		return plan, err
	}
	if platform.HashBytes(material) != plan.ConfirmationHash || maxGeneration != plan.ReferenceGeneration || !slices.Equal(historyIDs, plan.HistoryAffectedDocumentIds) {
		return plan, platform.NewError(platform.CodeIntegrityError, false, "人工计划精确扫描摘要或历史风险集合不符")
	}
	return plan, nil
}

func CancelManualCleanup(ctx context.Context, op OperationContext, input CancelManualCleanupInput) (CancelManualCleanupResult, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return CancelManualCleanupResult{}, err
	}
	hash, err := cleanupRequestHash(op, "cancel", input)
	if err != nil {
		return CancelManualCleanupResult{}, err
	}
	var original CancelManualCleanupResult
	if found, err := s.readCleanupReceipt(ctx, s.db, op, hash, "cancel", &original); found || err != nil {
		return original, err
	}
	plan, err := s.readCleanupPlan(ctx, s.db, op, input.PlanId)
	if err != nil {
		return CancelManualCleanupResult{}, err
	}
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return CancelManualCleanupResult{}, err
	}
	defer cancel()
	release := s.lockReferenceAssets(plan.Assets)
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return CancelManualCleanupResult{}, mapMediaDBError(err, "开始取消计划事务失败")
	}
	defer tx.Rollback()
	if found, err := s.readCleanupReceipt(work, tx, op, hash, "cancel", &original); found || err != nil {
		return original, err
	}
	current, err := s.readCleanupPlan(work, tx, op, input.PlanId)
	if err != nil {
		return CancelManualCleanupResult{}, err
	}
	if !slices.Equal(current.Assets, plan.Assets) {
		return CancelManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "锁定后计划资产集合改变")
	}
	if err := s.cancelCleanupPlan(work, tx, op, current); err != nil {
		return CancelManualCleanupResult{}, err
	}
	result := CancelManualCleanupResult{Cancelled: true}
	if err := writeCleanupReceipt(work, tx, op, hash, "cancel", input.PlanId, result); err != nil {
		return CancelManualCleanupResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CancelManualCleanupResult{}, mapMediaDBError(err, "提交取消计划失败")
	}
	return result, nil
}

// cancelCleanupPlan 的调用方已持有计划所有资产锁与store互斥；此处绝不回源或补锁。
func (s *localBlobStore) cancelCleanupPlan(ctx context.Context, tx *sql.Tx, op OperationContext, plan gen.CleanupPlan) error {
	if string(plan.State) == "cancelled" {
		var marks int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM manual_cleanup_marks WHERE plan_id=?`, plan.PlanId).Scan(&marks); err != nil {
			return mapMediaDBError(err, "核对已取消计划标记失败")
		}
		if marks != 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "已取消计划残留删除标记")
		}
		return nil
	}
	if string(plan.State) == "deleted" {
		return platform.NewError(platform.CodeConflict, false, "计划已经完成删除，不能取消")
	}
	if plan.Revision == 9007199254740991 {
		return platform.NewError(platform.CodeLocked, false, "计划修订已耗尽")
	}
	pending := string(plan.State) == "confirmed" || string(plan.State) == "waiting"
	for _, expected := range plan.Assets {
		var decisions int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM blob_journal WHERE asset_id=? AND event='DELETING'`, expected.AssetId).Scan(&decisions); err != nil {
			return mapMediaDBError(err, "读取人工删除决定失败")
		}
		if decisions != 0 {
			return platform.NewError(platform.CodeConflict, false, "资源已有持久删除决定，不能取消")
		}
		var markPlan string
		var markFence, markGeneration int
		err := tx.QueryRowContext(ctx, `SELECT plan_id,deletion_fence,reference_generation FROM manual_cleanup_marks WHERE asset_id=?`, expected.AssetId).Scan(&markPlan, &markFence, &markGeneration)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return mapMediaDBError(err, "读取计划删除标记失败")
		}
		ownMark := err == nil && markPlan == plan.PlanId
		if pending && !ownMark {
			return platform.NewError(platform.CodeIntegrityError, false, "待删除计划缺精确标记")
		}
		if string(plan.State) == "proposed" && ownMark {
			return platform.NewError(platform.CodeIntegrityError, false, "未确认计划出现删除标记")
		}
		if !ownMark {
			asset, fence, generation, err := s.readCleanupAsset(ctx, tx, op.Actor.SiteID, expected.AssetId)
			if err != nil {
				return err
			}
			if asset.Ref != expected {
				return platform.NewError(platform.CodeIntegrityError, false, "未标记计划项的精确资产改变")
			}
			if markPlan == "" {
				if asset.State != gen.AssetStateReady {
					return platform.NewError(platform.CodeIntegrityError, false, "未标记计划项出现未知待删除状态")
				}
			} else {
				other, err := s.readCleanupPlan(ctx, tx, op, markPlan)
				if err != nil {
					return err
				}
				if (string(other.State) != "confirmed" && string(other.State) != "waiting" && string(other.State) != "blocked") || !slices.Contains(other.Assets, expected) || string(asset.State) != "delete_pending" || fence != markFence || generation != markGeneration {
					return platform.NewError(platform.CodeIntegrityError, false, "其他计划标记缺精确活动保护")
				}
			}
			continue
		}
		asset, fence, generation, err := s.readCleanupAsset(ctx, tx, op.Actor.SiteID, expected.AssetId)
		if err != nil {
			return err
		}
		if asset.Ref != expected || string(asset.State) != "delete_pending" || fence != markFence || generation != markGeneration {
			return platform.NewError(platform.CodeIntegrityError, false, "计划标记与待删除资源或对象水位不符")
		}
		if err := advanceReferenceGeneration(ctx, tx, expected.AssetId, true); err != nil {
			return err
		}
		asset.State = gen.AssetStateReady
		raw, err := platform.CanonicalJSON(asset)
		if err != nil {
			return err
		}
		changed, err := tx.ExecContext(ctx, `UPDATE assets SET state='ready',asset_json=?,updated_at=? WHERE asset_id=? AND site_id=? AND state='delete_pending'`, string(raw), platform.Now(), expected.AssetId, op.Actor.SiteID)
		if err != nil {
			return mapMediaDBError(err, "解除计划资源删除状态失败")
		}
		count, err := changed.RowsAffected()
		if err != nil || count != 1 {
			return platform.NewError(platform.CodeConflict, false, "解除删除状态发生竞争")
		}
		changed, err = tx.ExecContext(ctx, `DELETE FROM manual_cleanup_marks WHERE asset_id=? AND plan_id=? AND deletion_fence=? AND reference_generation=?`, expected.AssetId, plan.PlanId, markFence, markGeneration)
		if err != nil {
			return mapMediaDBError(err, "解除人工删除标记失败")
		}
		count, err = changed.RowsAffected()
		if err != nil || count != 1 {
			return platform.NewError(platform.CodeConflict, false, "解除人工删除标记发生竞争")
		}
	}
	previousRevision := plan.Revision
	previousState := plan.State
	plan.State = gen.CleanupPlanState("cancelled")
	plan.Revision++
	raw, err := platform.CanonicalJSON(plan)
	if err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE manual_cleanup_plans SET state='cancelled',revision=?,plan_json=?,plan_hash=?,updated_at=? WHERE plan_id=? AND data_epoch=? AND site_id=? AND revision=? AND state=?`, plan.Revision, string(raw), platform.HashBytes(raw), platform.Now(), plan.PlanId, op.DataEpoch, op.Actor.SiteID, previousRevision, previousState)
	if err != nil {
		return mapMediaDBError(err, "取消人工计划失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeConflict, false, "取消人工计划修订发生竞争")
	}
	return nil
}

// referenceCleanupAssets 在任何asset锁之前扩出相交计划的完整不可变集合。
func (s *localBlobStore) referenceCleanupAssets(ctx context.Context, op OperationContext, siteID Identifier, requested, base []gen.AssetVersionRef) ([]gen.AssetVersionRef, []Identifier, error) {
	byID := make(map[string]gen.AssetVersionRef)
	for _, asset := range base {
		byID[asset.AssetId] = asset
	}
	planSet := make(map[string]bool)
	for _, asset := range requested {
		ids, err := s.assetCleanupPlans(ctx, s.db, asset.AssetId)
		if err != nil {
			return nil, nil, err
		}
		for _, id := range ids {
			planSet[id] = true
		}
	}
	planIDs := make([]Identifier, 0, len(planSet))
	for id := range planSet {
		planIDs = append(planIDs, id)
	}
	slices.Sort(planIDs)
	planOp := op
	planOp.Actor.SiteID = siteID
	for _, id := range planIDs {
		plan, err := s.readCleanupPlan(ctx, s.db, planOp, id)
		if err != nil {
			return nil, nil, err
		}
		for _, asset := range plan.Assets {
			if existing, found := byID[asset.AssetId]; found && existing != asset {
				return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "相交计划不可变资产材料冲突")
			}
			byID[asset.AssetId] = asset
		}
	}
	assets := make([]gen.AssetVersionRef, 0, len(byID))
	for _, asset := range byID {
		assets = append(assets, asset)
	}
	slices.SortFunc(assets, func(a, b gen.AssetVersionRef) int { return strings.Compare(a.AssetId, b.AssetId) })
	return assets, planIDs, nil
}

func (s *localBlobStore) assetCleanupPlans(ctx context.Context, q sqlExecer, assetID Identifier) ([]Identifier, error) {
	rows, err := q.QueryContext(ctx, `SELECT p.plan_id FROM manual_cleanup_plans p JOIN manual_cleanup_assets a ON a.plan_id=p.plan_id WHERE a.asset_id=? AND p.state IN ('proposed','confirmed','waiting','blocked') ORDER BY p.plan_id`, assetID)
	if err != nil {
		return nil, mapMediaDBError(err, "读取相交人工计划失败")
	}
	defer rows.Close()
	ids := []Identifier{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapMediaDBError(err, "读取人工计划身份失败")
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, mapMediaDBError(err, "遍历相交人工计划失败")
	}
	return ids, nil
}

// cancelReferenceCleanupPlans 在SQL内重新核计划代次和完整锁集合，然后与引用写入同事务取消。
func (s *localBlobStore) cancelReferenceCleanupPlans(ctx context.Context, tx *sql.Tx, op OperationContext, siteID Identifier, requested, locked []gen.AssetVersionRef, planIDs []Identifier) error {
	allowed := make(map[string]bool)
	for _, id := range planIDs {
		allowed[id] = true
	}
	lockedByID := make(map[string]gen.AssetVersionRef)
	for _, asset := range locked {
		lockedByID[asset.AssetId] = asset
	}
	for _, asset := range requested {
		ids, err := s.assetCleanupPlans(ctx, tx, asset.AssetId)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if !allowed[id] {
				return platform.NewError(platform.CodeLocked, false, "锁定后出现新的相交人工计划")
			}
		}
	}
	planOp := op
	planOp.Actor.SiteID = siteID
	for _, id := range planIDs {
		plan, err := s.readCleanupPlan(ctx, tx, planOp, id)
		if err != nil {
			return err
		}
		for _, asset := range plan.Assets {
			if lockedByID[asset.AssetId] != asset {
				return platform.NewError(platform.CodeLocked, false, "人工计划完整资产不在已持有锁集合")
			}
		}
		if err := s.cancelCleanupPlan(ctx, tx, planOp, plan); err != nil {
			return err
		}
	}
	return nil
}

// CleanupApproval 是一次人工批准的不可变对象，计划取消后仍保留证据。
type CleanupApproval struct {
	SchemaVersion   int                       `json:"schemaVersion"`
	OperationId     Identifier                `json:"operationId"`
	RootOperationId Identifier                `json:"rootOperationId"`
	CallerService   string                    `json:"callerService"`
	SiteId          Identifier                `json:"siteId"`
	DataEpoch       Identifier                `json:"dataEpoch"`
	ApprovedBy      Identifier                `json:"approvedBy"`
	ConfirmedAt     time.Time                 `json:"confirmedAt"`
	RequestHash     Hash                      `json:"requestHash"`
	Input           ConfirmManualCleanupInput `json:"input"`
	Plan            gen.CleanupPlan           `json:"plan"`
	Marks           []cleanupApprovalMark     `json:"marks"`
}

type cleanupApprovalMark struct {
	AssetId             Identifier `json:"assetId"`
	DeletionFence       int        `json:"deletionFence"`
	ReferenceGeneration int        `json:"referenceGeneration"`
}

type cleanupJobInput struct {
	Owner     string        `json:"owner"`
	Type      string        `json:"type"`
	InputRef  gen.ObjectRef `json:"inputRef"`
	DedupeKey string        `json:"dedupeKey"`
	NotBefore time.Time     `json:"notBefore"`
}

type cleanupJobReceipt struct {
	Job struct {
		JobId       Identifier `json:"jobId"`
		OperationId Identifier `json:"operationId"`
		State       string     `json:"state"`
	} `json:"job"`
}

type cleanupApprovalRecord struct {
	Approval CleanupApproval
	Raw      string
	Hash     Hash
	JobInput cleanupJobInput
	Receipt  *cleanupJobReceipt
	Result   *ConfirmManualCleanupResult
}

type ReadCleanupApprovalObjectInput struct {
	PlanId       Identifier   `json:"planId"`
	Version      gen.Revision `json:"version"`
	ExpectedHash Hash         `json:"expectedHash"`
}

type ReadCleanupApprovalObjectResult struct {
	Object gen.ObjectRef `json:"object"`
	UTF8   string        `json:"utf8"`
}

func InstallCleanupJobsClient(jobs *gen.JobsClient) error {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok || jobs == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "人工清理任务客户端未装配")
	}
	s.cleanupJobs = jobs
	return nil
}

func cleanupJobRequest(approval CleanupApproval, raw string) cleanupJobInput {
	hash := platform.HashBytes([]byte(raw))
	return cleanupJobInput{Owner: "media", Type: "media.cleanup", InputRef: gen.ObjectRef{Owner: "media", ObjectId: approval.Plan.PlanId, Version: approval.Plan.Revision, Sha256: hash, ByteSize: len(raw), MediaType: "application/json"}, DedupeKey: "media.cleanup:" + approval.DataEpoch + ":" + approval.Plan.PlanId + ":" + hash, NotBefore: approval.Plan.NotBefore}
}

func (s *localBlobStore) loadCleanupApproval(ctx context.Context, q sqlExecer, planID Identifier) (cleanupApprovalRecord, bool, error) {
	var approvalRaw, jobRaw, receiptRaw, resultRaw sql.NullString
	var hash, requestHash, jobHash, operationID string
	var receiptHash, resultHash sql.NullString
	err := q.QueryRowContext(ctx, `SELECT operation_id,request_hash,CASE WHEN length(CAST(approval_json AS BLOB))<=16777216 THEN approval_json ELSE NULL END,approval_hash,CASE WHEN length(CAST(job_input_json AS BLOB))<=16777216 THEN job_input_json ELSE NULL END,job_input_hash,CASE WHEN length(CAST(job_receipt_json AS BLOB))<=16777216 THEN job_receipt_json ELSE NULL END,job_receipt_hash,CASE WHEN length(CAST(result_json AS BLOB))<=16777216 THEN result_json ELSE NULL END,result_hash FROM manual_cleanup_approvals WHERE plan_id=?`, planID).Scan(&operationID, &requestHash, &approvalRaw, &hash, &jobRaw, &jobHash, &receiptRaw, &receiptHash, &resultRaw, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return cleanupApprovalRecord{}, false, nil
	}
	if err != nil {
		return cleanupApprovalRecord{}, false, mapMediaDBError(err, "读取人工批准桥接失败")
	}
	record := cleanupApprovalRecord{Raw: approvalRaw.String, Hash: hash}
	if !approvalRaw.Valid || platform.HashBytes([]byte(approvalRaw.String)) != hash || s.validateCleanupApproval(json.RawMessage(approvalRaw.String)) != nil || json.Unmarshal([]byte(approvalRaw.String), &record.Approval) != nil {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工批准对象损坏")
	}
	a := record.Approval
	canonical, err := platform.CanonicalJSON(a)
	binding, bindErr := cleanupRequestHash(OperationContext{CallerService: a.CallerService, Actor: platform.AuthContext{SubjectID: a.ApprovedBy, SiteID: a.SiteId}, DataEpoch: a.DataEpoch}, "confirm", a.Input)
	if err != nil || bindErr != nil || string(canonical) != approvalRaw.String || a.OperationId != operationID || a.Plan.PlanId != planID || a.Input.PlanId != planID || string(a.Plan.State) != "confirmed" || a.Input.ExpectedRevision > 9007199254740989 || a.Plan.Revision != a.Input.ExpectedRevision+1 || a.Input.ConfirmationHash != a.Plan.ConfirmationHash || a.RequestHash != requestHash || binding != requestHash || !a.Plan.NotBefore.Equal(a.ConfirmedAt.Add(24*time.Hour)) || (len(a.Plan.HistoryAffectedDocumentIds) > 0 && !a.Input.AcknowledgeHistoryLoss) || len(a.Marks) != len(a.Plan.Assets) {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工批准原输入、时间或修订绑定损坏")
	}
	for i, asset := range a.Plan.Assets {
		if (i > 0 && a.Plan.Assets[i-1].AssetId >= asset.AssetId) || a.Marks[i].AssetId != asset.AssetId {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工批准完整标记集合损坏")
		}
	}
	record.JobInput = cleanupJobRequest(a, approvalRaw.String)
	jobCanonical, err := platform.CanonicalJSON(record.JobInput)
	if err != nil || !jobRaw.Valid || platform.HashBytes([]byte(jobRaw.String)) != jobHash || string(jobCanonical) != jobRaw.String {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工批准固定任务输入损坏")
	}
	if receiptRaw.Valid != receiptHash.Valid || resultRaw.Valid != resultHash.Valid || (resultRaw.Valid && !receiptRaw.Valid) {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工批准回执状态损坏")
	}
	if receiptRaw.Valid {
		record.Receipt = &cleanupJobReceipt{}
		if platform.HashBytes([]byte(receiptRaw.String)) != receiptHash.String || s.validateCleanupJobReceipt(json.RawMessage(receiptRaw.String)) != nil || json.Unmarshal([]byte(receiptRaw.String), record.Receipt) != nil || record.Receipt.Job.OperationId != a.OperationId {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工清理任务原回执损坏")
		}
		encoded, err := platform.CanonicalJSON(record.Receipt)
		if err != nil || string(encoded) != receiptRaw.String {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工清理任务原回执非规范")
		}
	}
	if resultRaw.Valid {
		expected := a.Plan
		expected.State = gen.CleanupPlanState("waiting")
		expected.Revision++
		raw, err := platform.CanonicalJSON(ConfirmManualCleanupResult{Plan: expected})
		if err != nil || platform.HashBytes([]byte(resultRaw.String)) != resultHash.String || string(raw) != resultRaw.String {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工确认原成功结果损坏")
		}
		record.Result = &ConfirmManualCleanupResult{Plan: expected}
	}
	return record, true, nil
}

func (s *localBlobStore) readCleanupApproval(ctx context.Context, q sqlExecer, op OperationContext, requestHash Hash, input ConfirmManualCleanupInput) (cleanupApprovalRecord, bool, error) {
	var used int
	if err := q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM manual_cleanup_receipts WHERE operation_id=?)+(SELECT COUNT(*) FROM manual_cleanup_approvals WHERE operation_id=? AND plan_id<>?)`, op.OperationID, op.OperationID, input.PlanId).Scan(&used); err != nil {
		return cleanupApprovalRecord{}, false, mapMediaDBError(err, "核对人工批准操作唯一归属失败")
	}
	if used != 0 {
		return cleanupApprovalRecord{}, true, platform.NewError(platform.CodeConflict, false, "操作已绑定其他清理材料")
	}
	record, found, err := s.loadCleanupApproval(ctx, q, input.PlanId)
	if err != nil || !found {
		return record, found, err
	}
	a := record.Approval
	if a.OperationId != op.OperationID || a.RootOperationId != op.RootOperationID || a.RequestHash != requestHash || a.SiteId != op.Actor.SiteID || a.DataEpoch != op.DataEpoch || a.ApprovedBy != op.Actor.SubjectID || a.CallerService != op.CallerService || a.Input != input {
		return record, true, platform.NewError(platform.CodeConflict, false, "计划已经绑定一次不同的人工批准")
	}
	return record, true, nil
}

// ReadCleanupApprovalObject 只读真实批准对象，不以批准对象代替当前计划执行资格。
func ReadCleanupApprovalObject(ctx context.Context, op OperationContext, input ReadCleanupApprovalObjectInput) (ReadCleanupApprovalObjectResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return ReadCleanupApprovalObjectResult{}, platform.NewError(platform.CodeForbidden, false, "仅本域任务可读取批准对象")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return ReadCleanupApprovalObjectResult{}, err
	}
	record, found, err := s.loadCleanupApproval(ctx, s.db, input.PlanId)
	if err != nil {
		return ReadCleanupApprovalObjectResult{}, err
	}
	if !found {
		return ReadCleanupApprovalObjectResult{}, platform.NewError(platform.CodeNotFound, false, "人工批准对象不存在")
	}
	if record.Approval.SiteId != op.Actor.SiteID || record.Approval.DataEpoch != op.DataEpoch {
		return ReadCleanupApprovalObjectResult{}, platform.NewError(platform.CodeForbidden, false, "人工批准对象不属于当前站点或代次")
	}
	if input.Version != record.JobInput.InputRef.Version || input.ExpectedHash != record.Hash {
		return ReadCleanupApprovalObjectResult{}, platform.NewError(platform.CodeVersionMismatch, false, "人工批准对象版本或摘要不符")
	}
	return ReadCleanupApprovalObjectResult{Object: record.JobInput.InputRef, UTF8: record.Raw}, nil
}

func ConfirmManualCleanup(ctx context.Context, op OperationContext, input ConfirmManualCleanupInput) (ConfirmManualCleanupResult, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	hash, err := cleanupRequestHash(op, "confirm", input)
	if err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	record, found, err := s.readCleanupApproval(ctx, s.db, op, hash, input)
	if err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	if found && record.Result != nil {
		return *record.Result, nil
	}
	if !found {
		plan, err := s.readCleanupPlan(ctx, s.db, op, input.PlanId)
		if err != nil {
			return ConfirmManualCleanupResult{}, err
		}
		if string(plan.State) != "proposed" {
			return ConfirmManualCleanupResult{}, platform.NewError(platform.CodeConflict, false, "只有原提案可以接受人工确认")
		}
		if plan.Revision != input.ExpectedRevision || plan.ConfirmationHash != input.ConfirmationHash {
			return ConfirmManualCleanupResult{}, platform.NewError(platform.CodeVersionMismatch, false, "原提案修订或精确摘要不符")
		}
		if len(plan.HistoryAffectedDocumentIds) > 0 && !input.AcknowledgeHistoryLoss {
			return ConfirmManualCleanupResult{}, platform.NewError(platform.CodeForbidden, false, "必须明确接受历史资源缺失风险")
		}
		scans, err := s.scanCleanupConfirmation(ctx, op, plan)
		if err != nil {
			return ConfirmManualCleanupResult{}, err
		}
		record, err = s.approveCleanupPlan(ctx, op, hash, input, plan, scans)
		if err != nil {
			return ConfirmManualCleanupResult{}, err
		}
	}
	if record.Result != nil {
		return *record.Result, nil
	}
	return s.submitCleanupApproval(ctx, op, hash, input, record)
}

func (s *localBlobStore) scanCleanupConfirmation(ctx context.Context, op OperationContext, plan gen.CleanupPlan) ([]cleanupScan, error) {
	scans := make([]cleanupScan, 0, len(plan.Assets))
	for _, expected := range plan.Assets {
		var originalRaw sql.NullString
		var hash string
		var fence, generation int
		if err := s.db.QueryRowContext(ctx, `SELECT deletion_fence,reference_generation,CASE WHEN length(CAST(snapshot_json AS BLOB))<=16777216 THEN snapshot_json ELSE NULL END,snapshot_hash FROM manual_cleanup_assets WHERE plan_id=? AND asset_id=?`, plan.PlanId, expected.AssetId).Scan(&fence, &generation, &originalRaw, &hash); err != nil {
			return nil, mapMediaDBError(err, "读取人工确认原扫描失败")
		}
		read, err := ValidateCleanupSources(ctx, op, ValidateCleanupSourcesInput{AssetId: expected.AssetId})
		if err != nil {
			return nil, err
		}
		if !read.SafeToDelete {
			if err := s.blockCleanupProposal(ctx, op, plan); err != nil {
				return nil, err
			}
			return nil, platform.NewError(platform.CodeLocked, false, "出现当前引用或待提交保护，原提案已阻断")
		}
		raw, err := platform.CanonicalJSON(read.Snapshot)
		if err != nil {
			return nil, err
		}
		if !originalRaw.Valid || platform.HashBytes([]byte(originalRaw.String)) != hash || string(raw) != originalRaw.String || read.Snapshot.ReferenceGeneration != generation {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "完整权威引用快照已变化，请重新生成清理提案")
		}
		scans = append(scans, cleanupScan{expected, fence, read.Snapshot})
	}
	return scans, nil
}

func (s *localBlobStore) blockCleanupProposal(ctx context.Context, op OperationContext, plan gen.CleanupPlan) error {
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	release := s.lockReferenceAssets(plan.Assets)
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return mapMediaDBError(err, "开始阻断清理提案失败")
	}
	defer tx.Rollback()
	current, err := s.readCleanupPlan(work, tx, op, plan.PlanId)
	if err != nil {
		return err
	}
	if current.Revision != plan.Revision || string(current.State) != "proposed" {
		return platform.NewError(platform.CodeConflict, false, "原清理提案状态已变化")
	}
	if plan.Revision == 9007199254740991 {
		return platform.NewError(platform.CodeLocked, false, "计划修订已耗尽")
	}
	if err := s.updateCleanupState(work, tx, op, current, "blocked"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return mapMediaDBError(err, "提交阻断提案失败")
	}
	return nil
}

func (s *localBlobStore) updateCleanupState(ctx context.Context, tx *sql.Tx, op OperationContext, plan gen.CleanupPlan, state string) error {
	previousRevision, previousState := plan.Revision, plan.State
	plan.Revision++
	plan.State = gen.CleanupPlanState(state)
	raw, err := platform.CanonicalJSON(plan)
	if err != nil {
		return err
	}
	changed, err := tx.ExecContext(ctx, `UPDATE manual_cleanup_plans SET state=?,revision=?,plan_json=?,plan_hash=?,updated_at=? WHERE plan_id=? AND data_epoch=? AND site_id=? AND revision=? AND state=?`, state, plan.Revision, string(raw), platform.HashBytes(raw), platform.Now(), plan.PlanId, op.DataEpoch, op.Actor.SiteID, previousRevision, previousState)
	if err != nil {
		return mapMediaDBError(err, "保存人工清理计划状态失败")
	}
	count, err := changed.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeConflict, false, "人工清理计划修订竞争")
	}
	return nil
}

func (s *localBlobStore) approveCleanupPlan(ctx context.Context, op OperationContext, hash Hash, input ConfirmManualCleanupInput, plan gen.CleanupPlan, scans []cleanupScan) (cleanupApprovalRecord, error) {
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return cleanupApprovalRecord{}, err
	}
	defer cancel()
	release := s.lockReferenceAssets(plan.Assets)
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return cleanupApprovalRecord{}, mapMediaDBError(err, "开始人工批准事务失败")
	}
	defer tx.Rollback()
	if original, found, err := s.readCleanupApproval(work, tx, op, hash, input); found || err != nil {
		return original, err
	}
	current, err := s.readCleanupPlan(work, tx, op, plan.PlanId)
	if err != nil {
		return cleanupApprovalRecord{}, err
	}
	if current.Revision != plan.Revision || string(current.State) != "proposed" || current.ConfirmationHash != plan.ConfirmationHash || !slices.Equal(current.Assets, plan.Assets) {
		return cleanupApprovalRecord{}, platform.NewError(platform.CodeVersionMismatch, false, "人工确认期间原计划已变化")
	}
	if current.Revision > 9007199254740989 {
		return cleanupApprovalRecord{}, platform.NewError(platform.CodeLocked, false, "计划修订不足以完成批准与等待")
	}
	now := time.Now().UTC()
	current.NotBefore = now.Add(24 * time.Hour)
	approval := CleanupApproval{SchemaVersion: 1, OperationId: op.OperationID, RootOperationId: op.RootOperationID, CallerService: op.CallerService, SiteId: op.Actor.SiteID, DataEpoch: op.DataEpoch, ApprovedBy: op.Actor.SubjectID, ConfirmedAt: now, RequestHash: hash, Input: input, Plan: current, Marks: make([]cleanupApprovalMark, 0, len(scans))}
	for _, scan := range scans {
		asset, fence, generation, err := s.readCleanupAsset(work, tx, op.Actor.SiteID, scan.Asset.AssetId)
		if err != nil {
			return cleanupApprovalRecord{}, err
		}
		if asset.Ref != scan.Asset || asset.State != gen.AssetStateReady || fence != scan.Fence || generation != scan.Snapshot.ReferenceGeneration {
			return cleanupApprovalRecord{}, platform.NewError(platform.CodeVersionMismatch, false, "批准前精确资产或引用水位已变化")
		}
		var protections int
		if err := tx.QueryRowContext(work, `SELECT (SELECT COUNT(*) FROM asset_current_references WHERE asset_id=?)+(SELECT COUNT(*) FROM reference_pin_assets p JOIN reference_sources s ON s.pin_id=p.pin_id WHERE p.asset_id=? AND s.state='pending')+(SELECT COUNT(*) FROM manual_cleanup_marks WHERE asset_id=?)+(SELECT COUNT(*) FROM blob_journal WHERE asset_id=? AND event='DELETING')`, scan.Asset.AssetId, scan.Asset.AssetId, scan.Asset.AssetId, scan.Asset.AssetId).Scan(&protections); err != nil {
			return cleanupApprovalRecord{}, mapMediaDBError(err, "核对批准前引用及删除保护失败")
		}
		if protections != 0 {
			return cleanupApprovalRecord{}, platform.NewError(platform.CodeLocked, false, "批准前出现引用或删除保护")
		}
		if err := advanceReferenceGeneration(work, tx, scan.Asset.AssetId, true); err != nil {
			return cleanupApprovalRecord{}, err
		}
		asset.State = gen.AssetState("delete_pending")
		raw, err := platform.CanonicalJSON(asset)
		if err != nil {
			return cleanupApprovalRecord{}, err
		}
		changed, err := tx.ExecContext(work, `UPDATE assets SET state='delete_pending',asset_json=?,updated_at=? WHERE asset_id=? AND site_id=? AND state='ready'`, string(raw), platform.Now(), scan.Asset.AssetId, op.Actor.SiteID)
		if err != nil {
			return cleanupApprovalRecord{}, mapMediaDBError(err, "标记人工待删除资源失败")
		}
		count, err := changed.RowsAffected()
		if err != nil || count != 1 {
			return cleanupApprovalRecord{}, platform.NewError(platform.CodeConflict, false, "标记人工待删除资源发生竞争")
		}
		if _, err := tx.ExecContext(work, `INSERT INTO manual_cleanup_marks(asset_id,plan_id,deletion_fence,reference_generation) VALUES(?,?,?,?)`, scan.Asset.AssetId, plan.PlanId, fence+1, generation+1); err != nil {
			return cleanupApprovalRecord{}, mapMediaDBError(err, "写入人工批准精确标记失败")
		}
		approval.Marks = append(approval.Marks, cleanupApprovalMark{scan.Asset.AssetId, fence + 1, generation + 1})
	}
	if err := s.updateCleanupState(work, tx, op, current, "confirmed"); err != nil {
		return cleanupApprovalRecord{}, err
	}
	approval.Plan.State = gen.CleanupPlanState("confirmed")
	approval.Plan.Revision++
	raw, err := platform.CanonicalJSON(approval)
	if err != nil {
		return cleanupApprovalRecord{}, err
	}
	if len(raw) > 16<<20 || s.validateCleanupApproval(raw) != nil {
		return cleanupApprovalRecord{}, platform.NewError(platform.CodeIntegrityError, false, "人工批准对象超出持久预算或不符合唯一schema")
	}
	record := cleanupApprovalRecord{Approval: approval, Raw: string(raw), Hash: platform.HashBytes(raw), JobInput: cleanupJobRequest(approval, string(raw))}
	jobRaw, err := platform.CanonicalJSON(record.JobInput)
	if err != nil {
		return cleanupApprovalRecord{}, err
	}
	if _, err := tx.ExecContext(work, `INSERT INTO manual_cleanup_approvals(plan_id,operation_id,request_hash,approval_json,approval_hash,job_input_json,job_input_hash,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, plan.PlanId, op.OperationID, hash, string(raw), record.Hash, string(jobRaw), platform.HashBytes(jobRaw), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return cleanupApprovalRecord{}, mapMediaDBError(err, "保存人工批准与固定任务输入失败")
	}
	if err := tx.Commit(); err != nil {
		return cleanupApprovalRecord{}, mapMediaDBError(err, "提交人工批准失败")
	}
	return record, nil
}

// checkCleanupApprovalPending 只核完整活动计划和精确标记，不回源、不授予最终删除许可。
func (s *localBlobStore) checkCleanupApprovalPending(ctx context.Context, q sqlExecer, op OperationContext, record cleanupApprovalRecord) error {
	plan, err := s.readCleanupPlan(ctx, q, op, record.Approval.Plan.PlanId)
	if err != nil {
		return err
	}
	expected := record.Approval.Plan
	actualRaw, _ := platform.CanonicalJSON(plan)
	expectedRaw, _ := platform.CanonicalJSON(expected)
	if string(actualRaw) != string(expectedRaw) {
		return platform.NewError(platform.CodeConflict, false, "人工批准计划已取消、阻断或改变")
	}
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM manual_cleanup_marks WHERE plan_id=?`, plan.PlanId).Scan(&count); err != nil {
		return mapMediaDBError(err, "读取人工批准标记数量失败")
	}
	if count != len(record.Approval.Marks) {
		return platform.NewError(platform.CodeIntegrityError, false, "人工批准缺完整标记")
	}
	for i, mark := range record.Approval.Marks {
		asset, fence, generation, err := s.readCleanupAsset(ctx, q, op.Actor.SiteID, mark.AssetId)
		if err != nil {
			return err
		}
		var ownMarks, decisions, protections int
		if err := q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM manual_cleanup_marks WHERE asset_id=? AND plan_id=? AND deletion_fence=? AND reference_generation=?),(SELECT COUNT(*) FROM blob_journal WHERE asset_id=? AND event='DELETING'),(SELECT COUNT(*) FROM asset_current_references WHERE asset_id=?)+(SELECT COUNT(*) FROM reference_pin_assets p JOIN reference_sources s ON s.pin_id=p.pin_id WHERE p.asset_id=? AND s.state='pending')`, mark.AssetId, plan.PlanId, mark.DeletionFence, mark.ReferenceGeneration, mark.AssetId, mark.AssetId, mark.AssetId).Scan(&ownMarks, &decisions, &protections); err != nil {
			return mapMediaDBError(err, "核对人工批准精确标记失败")
		}
		if asset.Ref != plan.Assets[i] || string(asset.State) != "delete_pending" || fence != mark.DeletionFence || generation != mark.ReferenceGeneration || ownMarks != 1 || decisions != 0 || protections != 0 {
			return platform.NewError(platform.CodeLocked, false, "人工批准标记、水位或引用保护已变化")
		}
	}
	return nil
}

func (s *localBlobStore) submitCleanupApproval(ctx context.Context, op OperationContext, hash Hash, input ConfirmManualCleanupInput, record cleanupApprovalRecord) (ConfirmManualCleanupResult, error) {
	if s.cleanupJobs == nil {
		return ConfirmManualCleanupResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "人工清理任务客户端未装配")
	}
	// 只读事务固定整份计划快照，必须结束后才RPC；取消仍可随时撤销原计划。
	precheck, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ConfirmManualCleanupResult{}, mapMediaDBError(err, "开始任务提交前只读快照失败")
	}
	pendingErr := s.checkCleanupApprovalPending(ctx, precheck, op, record)
	releaseErr := precheck.Rollback()
	if pendingErr != nil {
		return ConfirmManualCleanupResult{}, pendingErr
	}
	if releaseErr != nil {
		return ConfirmManualCleanupResult{}, mapMediaDBError(releaseErr, "结束任务提交前只读快照失败")
	}
	receipt := record.Receipt
	if receipt == nil {
		submitOp := op
		submitOp.CallerService = "media"
		submitOp.OperationID = record.Approval.OperationId
		submitOp.RootOperationID = record.Approval.RootOperationId
		submitOp.DataEpoch = record.Approval.DataEpoch
		submitOp.Actor = platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline}
		submitCtx := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), submitOp)
		raw, err := s.cleanupJobs.CreateJob(submitCtx, record.JobInput, record.Approval.OperationId)
		if err != nil {
			return ConfirmManualCleanupResult{}, err
		}
		receipt = &cleanupJobReceipt{}
		if s.validateCleanupJobReceipt(raw) != nil || json.Unmarshal(raw, receipt) != nil || receipt.Job.OperationId != record.Approval.OperationId {
			return ConfirmManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "人工清理任务回执与原批准操作不符")
		}
	}
	return s.completeCleanupApproval(ctx, op, hash, input, record, *receipt)
}

func (s *localBlobStore) completeCleanupApproval(ctx context.Context, op OperationContext, hash Hash, input ConfirmManualCleanupInput, record cleanupApprovalRecord, receipt cleanupJobReceipt) (ConfirmManualCleanupResult, error) {
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	defer cancel()
	release := s.lockReferenceAssets(record.Approval.Plan.Assets)
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return ConfirmManualCleanupResult{}, mapMediaDBError(err, "开始人工确认任务回执事务失败")
	}
	defer tx.Rollback()
	current, found, err := s.readCleanupApproval(work, tx, op, hash, input)
	if err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	if !found || current.Hash != record.Hash {
		return ConfirmManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "人工批准原桥接缺失或改变")
	}
	raw, err := platform.CanonicalJSON(receipt)
	if err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	if current.Receipt != nil {
		original, _ := platform.CanonicalJSON(current.Receipt)
		if string(original) != string(raw) {
			return ConfirmManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "同人工批准出现不同任务回执")
		}
	}
	if current.Result != nil {
		return *current.Result, nil
	}
	if _, err := tx.ExecContext(work, `UPDATE manual_cleanup_approvals SET job_receipt_json=?,job_receipt_hash=?,updated_at=? WHERE plan_id=? AND operation_id=? AND approval_hash=?`, string(raw), platform.HashBytes(raw), platform.Now(), input.PlanId, op.OperationID, record.Hash); err != nil {
		return ConfirmManualCleanupResult{}, mapMediaDBError(err, "保存人工清理任务原回执失败")
	}
	pendingErr := s.checkCleanupApprovalPending(work, tx, op, current)
	if pendingErr != nil {
		// 迟到任务成功也不能恢复已取消计划；仅保留真实任务回执。
		if err := tx.Commit(); err != nil {
			return ConfirmManualCleanupResult{}, mapMediaDBError(err, "提交迟到任务原回执失败")
		}
		return ConfirmManualCleanupResult{}, pendingErr
	}
	if err := s.updateCleanupState(work, tx, op, current.Approval.Plan, "waiting"); err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	plan := current.Approval.Plan
	plan.State = gen.CleanupPlanState("waiting")
	plan.Revision++
	result := ConfirmManualCleanupResult{Plan: plan}
	resultRaw, err := platform.CanonicalJSON(result)
	if err != nil {
		return ConfirmManualCleanupResult{}, err
	}
	if _, err := tx.ExecContext(work, `UPDATE manual_cleanup_approvals SET result_json=?,result_hash=?,updated_at=? WHERE plan_id=? AND operation_id=? AND approval_hash=?`, string(resultRaw), platform.HashBytes(resultRaw), platform.Now(), input.PlanId, op.OperationID, record.Hash); err != nil {
		return ConfirmManualCleanupResult{}, mapMediaDBError(err, "保存人工确认原成功结果失败")
	}
	if err := tx.Commit(); err != nil {
		return ConfirmManualCleanupResult{}, mapMediaDBError(err, "提交人工确认原成功结果失败")
	}
	return result, nil
}

type ListCleanupPlansInput struct{}
type ListCleanupPlansResult = gen.ListCleanupPlansResult

// ListCleanupPlans列出还没结束的人工计划，让24小时保护窗口内的删除可见、可撤销。
// 只核对计划原文摘要，不按当前引用复核：引用变化由确认和执行阶段处理。
func ListCleanupPlans(ctx context.Context, op OperationContext, _ ListCleanupPlansInput) (ListCleanupPlansResult, error) {
	s, err := cleanupStore(op, true)
	if err != nil {
		return ListCleanupPlansResult{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT CASE WHEN length(CAST(plan_json AS BLOB))<=16777216 THEN plan_json ELSE NULL END,plan_hash FROM manual_cleanup_plans WHERE site_id=? AND data_epoch=? AND state IN ('proposed','confirmed','waiting') ORDER BY created_at DESC LIMIT 100`, op.Actor.SiteID, op.DataEpoch)
	if err != nil {
		return ListCleanupPlansResult{}, mapMediaDBError(err, "读取人工计划列表失败")
	}
	defer rows.Close()
	result := ListCleanupPlansResult{Plans: []gen.CleanupPlan{}}
	for rows.Next() {
		var raw sql.NullString
		var digest string
		if err := rows.Scan(&raw, &digest); err != nil {
			return ListCleanupPlansResult{}, mapMediaDBError(err, "读取人工计划列表失败")
		}
		var plan gen.CleanupPlan
		if !raw.Valid || platform.HashBytes([]byte(raw.String)) != digest || s.validateCleanupPlan(json.RawMessage(raw.String)) != nil || json.Unmarshal([]byte(raw.String), &plan) != nil {
			return ListCleanupPlansResult{}, platform.NewError(platform.CodeIntegrityError, false, "人工计划原材料损坏")
		}
		result.Plans = append(result.Plans, plan)
	}
	if err := rows.Err(); err != nil {
		return ListCleanupPlansResult{}, mapMediaDBError(err, "读取人工计划列表失败")
	}
	return result, nil
}

// ListUnusedAssetsInput保留既有HTTP kind筛选；省略为全部类别。
type ListUnusedAssetsInput = ListAssetsInput
type ListUnusedAssetsResult = gen.ListUnusedAssetsResult

// recheckAssetListingPage在全部锁外回源完成后，固定本地权威水位与精确候选再次核对。
func (s *localBlobStore) recheckAssetListingPage(ctx context.Context, op OperationContext, page assetListingPage) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return mapMediaDBError(err, "开始未引用列表最终快照失败")
	}
	defer tx.Rollback()
	revision, err := readAssetListingRevision(ctx, tx)
	if err != nil {
		return err
	}
	if revision != page.Revision {
		return platform.NewError(platform.CodeVersionMismatch, true, "回源期间资源列表水位改变，请重新分页")
	}
	for _, listed := range page.Assets {
		asset, fence, generation, err := s.readCleanupAsset(ctx, tx, op.Actor.SiteID, listed.Asset.Ref.AssetId)
		if err != nil {
			return err
		}
		if asset.Ref != listed.Asset.Ref || asset.State != gen.AssetStateReady || asset.Revision != listed.Asset.Revision || fence != listed.Fence || generation != listed.Generation {
			return platform.NewError(platform.CodeVersionMismatch, true, "回源期间资源或对象水位变化")
		}
	}
	if err := tx.Rollback(); err != nil {
		return mapMediaDBError(err, "结束未引用列表最终快照失败")
	}
	return nil
}

// ListUnusedAssets按候选前进分页；空匹配页仍可有下一页，绝不把未知引用来源当空。
func ListUnusedAssets(ctx context.Context, op OperationContext, input ListUnusedAssetsInput) (ListUnusedAssetsResult, error) {
	s, err := assetListingStore(op)
	if err != nil {
		return ListUnusedAssetsResult{}, err
	}
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	position, scope, err := s.readAssetListingCursor(op, input, true)
	if err != nil {
		return ListUnusedAssetsResult{}, err
	}
	page, err := s.readAssetListingPage(ctx, op, input, position, true)
	if err != nil {
		return ListUnusedAssetsResult{}, err
	}
	result := ListUnusedAssetsResult{Assets: make([]gen.Asset, 0, len(page.Assets)), HistoryAffectedDocumentIds: []Identifier{}}
	history := make(map[Identifier]bool)
	for _, listed := range page.Assets {
		generation := gen.Revision(listed.Generation)
		scan, err := ValidateCleanupSources(ctx, op, ValidateCleanupSourcesInput{AssetId: listed.Asset.Ref.AssetId, ExpectedGeneration: &generation})
		if err != nil {
			return ListUnusedAssetsResult{}, err
		}
		sources := assetListingSources{Content: scan.Snapshot.SourceWatermarks[0].Watermark, Config: scan.Snapshot.SourceWatermarks[1].Watermark}
		if position.Sources == nil {
			position.Sources = &sources
		} else if *position.Sources != sources {
			return ListUnusedAssetsResult{}, platform.NewError(platform.CodeVersionMismatch, true, "权威引用源水位改变，请从第一页重新读取")
		}
		if !scan.SafeToDelete {
			continue
		}
		result.Assets = append(result.Assets, listed.Asset)
		for _, ref := range scan.Snapshot.HistoryReferences {
			if ref.SourceService == gen.Content {
				history[ref.SourceObjectId] = true
			}
		}
		if len(history) > 10000 {
			return ListUnusedAssetsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本页完整历史风险超出响应预算，请减少limit")
		}
	}
	if err := s.recheckAssetListingPage(ctx, op, page); err != nil {
		return ListUnusedAssetsResult{}, err
	}
	for id := range history {
		result.HistoryAffectedDocumentIds = append(result.HistoryAffectedDocumentIds, id)
	}
	slices.Sort(result.HistoryAffectedDocumentIds)
	result.PageInfo, err = s.assetListingPageInfo(scope, position, page)
	if err != nil {
		return ListUnusedAssetsResult{}, err
	}
	return result, nil
}
