package media

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// ExecuteManualCleanupInput 只由可信worker注册器构造，完整保留jobs原对象及租约guard。
type ExecuteManualCleanupInput struct {
	PlanId             Identifier                 `json:"planId"`
	ExpectedGeneration gen.Revision               `json:"expectedGeneration"`
	Job                platform.JobExecutionInput `json:"job"`
}
type ExecuteManualCleanupResult struct {
	DeletedAssetIds []Identifier `json:"deletedAssetIds"`
	BlockedAssetIds []Identifier `json:"blockedAssetIds"`
}

type cleanupExecution struct {
	SchemaVersion   int          `json:"schemaVersion"`
	PlanId          Identifier   `json:"planId"`
	ApprovalHash    Hash         `json:"approvalHash"`
	JobId           Identifier   `json:"jobId"`
	OperationId     Identifier   `json:"operationId"`
	RootOperationId Identifier   `json:"rootOperationId"`
	DataEpoch       Identifier   `json:"dataEpoch"`
	SiteId          Identifier   `json:"siteId"`
	WaitingRevision gen.Revision `json:"waitingRevision"`
	DecidedAt       time.Time    `json:"decidedAt"`
}
type cleanupDeletion struct {
	SchemaVersion int                 `json:"schemaVersion"`
	PlanId        Identifier          `json:"planId"`
	ApprovalHash  Hash                `json:"approvalHash"`
	Asset         gen.Asset           `json:"asset"`
	Mark          cleanupApprovalMark `json:"mark"`
	DeleteBytes   bool                `json:"deleteBytes"`
}
type cleanupExecutionRecord struct {
	Material  cleanupExecution
	Decisions []cleanupDeletion
	Result    *ExecuteManualCleanupResult
}

// checkCleanupJobBinding 只核本域原批准与jobs已落本域回执，不从任务自报值造授权。
func checkCleanupJobBinding(op OperationContext, record cleanupApprovalRecord, job platform.JobExecutionInput) error {
	ref := record.JobInput.InputRef
	actual := job.InputRef
	a := record.Approval
	if job.JobType != "media.cleanup" || actual.Owner != "media" || actual.ObjectID != ref.ObjectId || actual.Version != int64(ref.Version) || actual.SHA256 != ref.Sha256 || actual.MediaType != ref.MediaType || actual.ByteSize != int64(ref.ByteSize) || job.DataEpoch != a.DataEpoch || op.DataEpoch != a.DataEpoch || op.OperationID != a.OperationId || op.RootOperationID != a.RootOperationId || op.Actor.SiteID != a.SiteId || record.Receipt == nil || record.Result == nil || record.Receipt.Job.JobId != job.JobID {
		return platform.NewError(platform.CodeConflict, false, "任务、原批准对象或原waiting回执未精确绑定")
	}
	return nil
}

func cleanupExecutionMaterial(record cleanupApprovalRecord) cleanupExecution {
	a := record.Approval
	return cleanupExecution{1, a.Plan.PlanId, record.Hash, record.Receipt.Job.JobId, a.OperationId, a.RootOperationId, a.DataEpoch, a.SiteId, record.Result.Plan.Revision, time.Now().UTC()}
}

// readCleanupExecution 校验原规范执行决定、精确全部资产与DELETING日志；不重造原结果。
func (s *localBlobStore) readCleanupExecution(ctx context.Context, q sqlExecer, approval cleanupApprovalRecord) (cleanupExecutionRecord, bool, error) {
	var materialRaw, hash, approvalHash, jobID string
	var resultRaw, resultHash sql.NullString
	var record cleanupExecutionRecord
	err := q.QueryRowContext(ctx, `SELECT approval_hash,job_id,execution_json,execution_hash,result_json,result_hash FROM manual_cleanup_executions WHERE plan_id=?`, approval.Approval.Plan.PlanId).Scan(&approvalHash, &jobID, &materialRaw, &hash, &resultRaw, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return record, false, nil
	}
	if err != nil {
		return record, false, mapMediaDBError(err, "读取人工删除执行决定失败")
	}
	if approval.Receipt == nil || approval.Result == nil {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "删除决定缺原任务与waiting回执")
	}
	if json.Unmarshal([]byte(materialRaw), &record.Material) != nil {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工删除执行材料无法解码")
	}
	expectedMaterial := cleanupExecutionMaterial(approval)
	expectedMaterial.DecidedAt = record.Material.DecidedAt
	expected, err := platform.CanonicalJSON(expectedMaterial)
	if err != nil || len(materialRaw) > 16<<20 || approvalHash != approval.Hash || jobID != record.Material.JobId || materialRaw != string(expected) || record.Material.DecidedAt.Before(approval.Approval.Plan.NotBefore) || platform.HashBytes([]byte(materialRaw)) != hash || resultRaw.Valid != resultHash.Valid {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工删除原执行材料损坏")
	}
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM manual_cleanup_deletions WHERE plan_id=?`, record.Material.PlanId).Scan(&count); err != nil {
		return record, true, mapMediaDBError(err, "读取删除决定完整数量失败")
	}
	if count != len(approval.Approval.Plan.Assets) {
		return record, true, platform.NewError(platform.CodeIntegrityError, false, "删除决定资产集合不完整")
	}
	record.Decisions = make([]cleanupDeletion, 0, count)
	byteOwners := map[Hash]bool{}
	for i, ref := range approval.Approval.Plan.Assets {
		var raw, digest string
		if err := q.QueryRowContext(ctx, `SELECT decision_json,decision_hash FROM manual_cleanup_deletions WHERE plan_id=? AND asset_id=?`, record.Material.PlanId, ref.AssetId).Scan(&raw, &digest); err != nil {
			return record, true, mapMediaDBError(err, "读取逐资产删除决定失败")
		}
		var decision cleanupDeletion
		if len(raw) > 16<<20 || platform.HashBytes([]byte(raw)) != digest || json.Unmarshal([]byte(raw), &decision) != nil {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "逐资产删除原决定损坏")
		}
		encoded, encodeErr := platform.CanonicalJSON(decision)
		assetRaw, assetErr := platform.CanonicalJSON(decision.Asset)
		if encodeErr != nil || assetErr != nil || string(encoded) != raw || s.validateAsset(assetRaw) != nil || decision.SchemaVersion != 1 || decision.PlanId != record.Material.PlanId || decision.ApprovalHash != approval.Hash || decision.Asset.Ref != ref || decision.Asset.SiteId != record.Material.SiteId || string(decision.Asset.State) != "delete_pending" || decision.Mark != approval.Approval.Marks[i] || (decision.DeleteBytes && byteOwners[ref.Sha256]) {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "删除决定与人工批准精确材料不符")
		}
		if decision.DeleteBytes {
			byteOwners[ref.Sha256] = true
		}
		var journals, matching int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN site_id=? AND upload_id=? AND sha256=? THEN 1 ELSE 0 END),0) FROM blob_journal WHERE event='DELETING' AND asset_id=?`, record.Material.SiteId, record.Material.PlanId, ref.Sha256, ref.AssetId).Scan(&journals, &matching); err != nil {
			return record, true, mapMediaDBError(err, "核对人工删除journal失败")
		}
		if journals != 1 || matching != 1 {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工删除决定缺唯一精确DELETING日志")
		}
		record.Decisions = append(record.Decisions, decision)
	}
	if resultRaw.Valid {
		result := ExecuteManualCleanupResult{DeletedAssetIds: make([]Identifier, 0, count), BlockedAssetIds: []Identifier{}}
		for _, ref := range approval.Approval.Plan.Assets {
			result.DeletedAssetIds = append(result.DeletedAssetIds, ref.AssetId)
		}
		canonical, err := platform.CanonicalJSON(result)
		if err != nil || resultRaw.String != string(canonical) || platform.HashBytes([]byte(resultRaw.String)) != resultHash.String {
			return record, true, platform.NewError(platform.CodeIntegrityError, false, "人工删除原成功结果损坏")
		}
		record.Result = &result
	}
	return record, true, nil
}

// ExecuteManualCleanup 先完整回源并持久整份决定，再按同一不可变决定删除与恢复。
func ExecuteManualCleanup(ctx context.Context, op OperationContext, input ExecuteManualCleanupInput) (ExecuteManualCleanupResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" || input.Job.CheckLease == nil {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeForbidden, false, "人工删除仅供持有平台租约guard的本域worker")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	approval, found, err := s.loadCleanupApproval(ctx, s.db, input.PlanId)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if !found {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeNotFound, false, "人工批准对象不存在")
	}
	object, err := ReadCleanupApprovalObject(ctx, op, ReadCleanupApprovalObjectInput{input.PlanId, gen.Revision(input.Job.InputRef.Version), input.Job.InputRef.SHA256})
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if object.UTF8 != approval.Raw {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "worker读取的原批准字节改变")
	}
	if err := checkCleanupJobBinding(op, approval, input.Job); err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if input.ExpectedGeneration != approval.Approval.Plan.ReferenceGeneration {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeVersionMismatch, false, "执行原计划引用水位不符")
	}
	record, decided, err := s.readCleanupExecution(ctx, s.db, approval)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if decided && record.Result != nil {
		return *record.Result, nil
	}
	if !decided {
		plan, err := s.readCleanupPlan(ctx, s.db, op, input.PlanId)
		if err != nil {
			return ExecuteManualCleanupResult{}, err
		}
		if !cleanupWaitingEqual(plan, approval) || time.Now().UTC().Before(plan.NotBefore) {
			return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeLocked, false, "计划未在原waiting状态或批准保护期尚未结束")
		}
		snapshots := make([]gen.ReferenceSnapshot, 0, len(plan.Assets))
		for _, asset := range plan.Assets {
			scan, err := ValidateCleanupSources(ctx, op, ValidateCleanupSourcesInput{AssetId: asset.AssetId})
			if err != nil {
				return ExecuteManualCleanupResult{}, err
			}
			snapshots = append(snapshots, scan.Snapshot)
			var originalRaw string
			if err := s.db.QueryRowContext(ctx, `SELECT snapshot_json FROM manual_cleanup_assets WHERE plan_id=? AND asset_id=?`, plan.PlanId, asset.AssetId).Scan(&originalRaw); err != nil {
				return ExecuteManualCleanupResult{}, mapMediaDBError(err, "读取人工批准原历史风险失败")
			}
			var original gen.ReferenceSnapshot
			if len(originalRaw) > 16<<20 || json.Unmarshal([]byte(originalRaw), &original) != nil {
				return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "人工批准原历史风险损坏")
			}
			a, _ := platform.CanonicalJSON(original.HistoryReferences)
			b, _ := platform.CanonicalJSON(scan.Snapshot.HistoryReferences)
			if !scan.SafeToDelete || string(a) != string(b) {
				return s.blockCleanupExecution(ctx, op, approval)
			}
		}
		// 公共租约RPC与运行窗口RPC必须先于任何asset/store/SQL锁。
		if err := input.Job.CheckLease(ctx); err != nil {
			return ExecuteManualCleanupResult{}, err
		}
		record, err = s.decideCleanupExecution(ctx, op, approval, snapshots)
		if err != nil {
			return ExecuteManualCleanupResult{}, err
		}
	}
	// 已持久DELETING只按原决定收敛，不再创造授权或重跑来源扫描。
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	defer cancel()
	release := s.lockReferenceAssets(approval.Approval.Plan.Assets)
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finishCleanupExecutionLocked(work, op, approval, record)
}

func cleanupWaitingEqual(plan gen.CleanupPlan, approval cleanupApprovalRecord) bool {
	if approval.Result == nil {
		return false
	}
	a, err := platform.CanonicalJSON(plan)
	b, other := platform.CanonicalJSON(approval.Result.Plan)
	return err == nil && other == nil && string(a) == string(b) && string(plan.State) == "waiting"
}

// blockCleanupExecution 不撤销精确标记；阻断计划可由管理员另行取消。
func (s *localBlobStore) blockCleanupExecution(ctx context.Context, op OperationContext, approval cleanupApprovalRecord) (ExecuteManualCleanupResult, error) {
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	defer cancel()
	release := s.lockReferenceAssets(approval.Approval.Plan.Assets)
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return ExecuteManualCleanupResult{}, mapMediaDBError(err, "开始阻断清理执行失败")
	}
	defer tx.Rollback()
	if _, found, err := s.readCleanupExecution(work, tx, approval); err != nil {
		return ExecuteManualCleanupResult{}, err
	} else if found {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeConflict, false, "清理已存在持久删除决定")
	}
	plan, err := s.readCleanupPlan(work, tx, op, approval.Approval.Plan.PlanId)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if !cleanupWaitingEqual(plan, approval) {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeConflict, false, "阻断时原waiting计划已变化")
	}
	if plan.Revision == 9007199254740991 {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeLocked, false, "计划修订已耗尽")
	}
	if err := s.updateCleanupState(work, tx, op, plan, "blocked"); err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ExecuteManualCleanupResult{}, mapMediaDBError(err, "提交阻断清理执行失败")
	}
	return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeLocked, false, "最终来源发现新引用，清理已阻断")
}

// decideCleanupExecution 全计划在同一短事务提交DELETING，之后取消均失败关闭。
func (s *localBlobStore) decideCleanupExecution(ctx context.Context, op OperationContext, approval cleanupApprovalRecord, snapshots []gen.ReferenceSnapshot) (cleanupExecutionRecord, error) {
	work, cancel, err := s.mediaWriteContext(ctx, op)
	if err != nil {
		return cleanupExecutionRecord{}, err
	}
	defer cancel()
	release := s.lockReferenceAssets(approval.Approval.Plan.Assets)
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(work, nil)
	if err != nil {
		return cleanupExecutionRecord{}, mapMediaDBError(err, "开始人工删除决定失败")
	}
	defer tx.Rollback()
	if record, found, err := s.readCleanupExecution(work, tx, approval); found || err != nil {
		return record, err
	}
	plan, err := s.readCleanupPlan(work, tx, op, approval.Approval.Plan.PlanId)
	if err != nil {
		return cleanupExecutionRecord{}, err
	}
	if !cleanupWaitingEqual(plan, approval) || time.Now().UTC().Before(plan.NotBefore) {
		return cleanupExecutionRecord{}, platform.NewError(platform.CodeConflict, false, "最终决定前waiting计划或保护期限已变化")
	}
	if plan.Revision == 9007199254740991 {
		return cleanupExecutionRecord{}, platform.NewError(platform.CodeLocked, false, "计划修订不足以完成删除")
	}
	var markCount int
	if err := tx.QueryRowContext(work, `SELECT COUNT(*) FROM manual_cleanup_marks WHERE plan_id=?`, plan.PlanId).Scan(&markCount); err != nil {
		return cleanupExecutionRecord{}, mapMediaDBError(err, "核对最终完整marks失败")
	}
	if markCount != len(plan.Assets) {
		return cleanupExecutionRecord{}, platform.NewError(platform.CodeIntegrityError, false, "最终删除缺完整人工marks")
	}
	record := cleanupExecutionRecord{Material: cleanupExecutionMaterial(approval), Decisions: make([]cleanupDeletion, 0, len(plan.Assets))}
	ownIDs := map[Identifier]bool{}
	for _, ref := range plan.Assets {
		ownIDs[ref.AssetId] = true
	}
	byteOwners := map[Hash]bool{}
	for i, ref := range plan.Assets {
		asset, fence, generation, err := s.readCleanupAsset(work, tx, op.Actor.SiteID, ref.AssetId)
		if err != nil {
			return record, err
		}
		mark := approval.Approval.Marks[i]
		var marks, protections, decisions, readers int
		if err := tx.QueryRowContext(work, `SELECT (SELECT COUNT(*) FROM manual_cleanup_marks WHERE asset_id=? AND plan_id=? AND deletion_fence=? AND reference_generation=?),(SELECT COUNT(*) FROM asset_current_references WHERE asset_id=?)+(SELECT COUNT(*) FROM reference_pin_assets p JOIN reference_sources s ON s.pin_id=p.pin_id WHERE p.asset_id=? AND s.state='pending'),(SELECT COUNT(*) FROM blob_journal WHERE asset_id=? AND event='DELETING'),(SELECT COUNT(*) FROM asset_reader_leases WHERE asset_id=? AND expires_at_ns>?)`, ref.AssetId, plan.PlanId, mark.DeletionFence, mark.ReferenceGeneration, ref.AssetId, ref.AssetId, ref.AssetId, ref.AssetId, time.Now().UnixNano()).Scan(&marks, &protections, &decisions, &readers); err != nil {
			return record, mapMediaDBError(err, "核对最终引用、读者和删除fence失败")
		}
		if asset.Ref != ref || string(asset.State) != "delete_pending" || fence != mark.DeletionFence || generation != mark.ReferenceGeneration || generation != snapshots[i].ReferenceGeneration || marks != 1 || protections != 0 || decisions != 0 || readers != 0 {
			return record, platform.NewError(platform.CodeLocked, false, "最终引用、水位、读者或删除标记改变")
		}
		if asset.Revision == 9007199254740991 {
			return record, platform.NewError(platform.CodeLocked, false, "资源修订已耗尽")
		}
		// 同站点同SHA所有未删除资产都要计入；计划外ready/pending均保留共享bytes。
		rows, err := tx.QueryContext(work, `SELECT asset_id FROM assets WHERE site_id=? AND sha256=? AND state<>'deleted' ORDER BY asset_id`, op.Actor.SiteID, ref.Sha256)
		if err != nil {
			return record, mapMediaDBError(err, "读取共享blob资产失败")
		}
		shared := false
		for rows.Next() {
			var id Identifier
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return record, mapMediaDBError(err, "读取共享blob资产失败")
			}
			if !ownIDs[id] {
				shared = true
			}
		}
		rowsErr := rows.Err()
		rows.Close()
		if rowsErr != nil {
			return record, mapMediaDBError(rowsErr, "读取共享blob集合失败")
		}
		deleteBytes := !shared && !byteOwners[ref.Sha256]
		if deleteBytes {
			byteOwners[ref.Sha256] = true
		}
		record.Decisions = append(record.Decisions, cleanupDeletion{1, plan.PlanId, approval.Hash, asset, mark, deleteBytes})
	}
	raw, err := platform.CanonicalJSON(record.Material)
	if err != nil {
		return record, err
	}
	if _, err := tx.ExecContext(work, `INSERT INTO manual_cleanup_executions(plan_id,approval_hash,job_id,execution_json,execution_hash,created_at) VALUES(?,?,?,?,?,?)`, plan.PlanId, approval.Hash, record.Material.JobId, string(raw), platform.HashBytes(raw), platform.Now()); err != nil {
		return record, mapMediaDBError(err, "持久人工删除执行材料失败")
	}
	for _, decision := range record.Decisions {
		raw, err := platform.CanonicalJSON(decision)
		if err != nil {
			return record, err
		}
		if _, err := tx.ExecContext(work, `INSERT INTO manual_cleanup_deletions(asset_id,plan_id,decision_json,decision_hash) VALUES(?,?,?,?)`, decision.Asset.Ref.AssetId, plan.PlanId, string(raw), platform.HashBytes(raw)); err != nil {
			return record, mapMediaDBError(err, "持久逐资产删除决定失败")
		}
		if err := appendJournal(work, tx, blobJournalDeleting, op.Actor.SiteID, plan.PlanId, decision.Asset.Ref.Sha256, decision.Asset.Ref.AssetId); err != nil {
			return record, err
		}
	}
	if err := tx.Commit(); err != nil {
		return record, mapMediaDBError(err, "提交人工删除决定未知，保留原材料")
	}
	return record, nil
}

// finishCleanupExecutionLocked 只收敛已持久决定；调用方持完整asset锁与store互斥。
func (s *localBlobStore) finishCleanupExecutionLocked(ctx context.Context, op OperationContext, approval cleanupApprovalRecord, expected cleanupExecutionRecord) (ExecuteManualCleanupResult, error) {
	record, found, err := s.readCleanupExecution(ctx, s.db, approval)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if !found || record.Material != expected.Material {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "待恢复人工删除决定缺失或改变")
	}
	if record.Result != nil {
		return *record.Result, nil
	}
	plan, err := s.readCleanupPlan(ctx, s.db, op, record.Material.PlanId)
	if err != nil {
		return ExecuteManualCleanupResult{}, err
	}
	if !cleanupWaitingEqual(plan, approval) {
		return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "已有删除决定的原计划被改变")
	}
	for _, decision := range record.Decisions {
		asset, fence, generation, err := s.readCleanupAsset(ctx, s.db, op.Actor.SiteID, decision.Asset.Ref.AssetId)
		if err != nil {
			return ExecuteManualCleanupResult{}, err
		}
		current, _ := platform.CanonicalJSON(asset)
		original, _ := platform.CanonicalJSON(decision.Asset)
		if string(current) != string(original) || fence != decision.Mark.DeletionFence || generation != decision.Mark.ReferenceGeneration {
			return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "已决定删除的资源或水位被改变")
		}
		var marks, protections, readers int
		if err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM manual_cleanup_marks WHERE asset_id=? AND plan_id=? AND deletion_fence=? AND reference_generation=?),(SELECT COUNT(*) FROM asset_current_references WHERE asset_id=?)+(SELECT COUNT(*) FROM reference_pin_assets p JOIN reference_sources s ON s.pin_id=p.pin_id WHERE p.asset_id=? AND s.state='pending'),(SELECT COUNT(*) FROM asset_reader_leases WHERE asset_id=? AND expires_at_ns>?)`, asset.Ref.AssetId, plan.PlanId, fence, generation, asset.Ref.AssetId, asset.Ref.AssetId, asset.Ref.AssetId, time.Now().UnixNano()).Scan(&marks, &protections, &readers); err != nil {
			return ExecuteManualCleanupResult{}, mapMediaDBError(err, "核对已决定资源的完整保护失败")
		}
		if marks != 1 || protections != 0 || readers != 0 {
			return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "已决定资源的标记、引用或读者保护改变")
		}
		if decision.DeleteBytes {
			var shared int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM assets WHERE site_id=? AND sha256=? AND state<>'deleted' AND asset_id NOT IN (SELECT asset_id FROM manual_cleanup_deletions WHERE plan_id=?)`, op.Actor.SiteID, asset.Ref.Sha256, plan.PlanId).Scan(&shared); err != nil {
				return ExecuteManualCleanupResult{}, mapMediaDBError(err, "核对删除前共享blob保护失败")
			}
			if shared != 0 {
				return ExecuteManualCleanupResult{}, platform.NewError(platform.CodeIntegrityError, false, "原bytes删除决定出现计划外共享资产")
			}
		}
	}
	for _, decision := range record.Decisions {
		if decision.DeleteBytes {
			if err := s.removeDecidedBlob(ctx, decision); err != nil {
				return ExecuteManualCleanupResult{}, err
			}
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ExecuteManualCleanupResult{}, mapMediaDBError(err, "开始完成删除事务失败")
	}
	defer tx.Rollback()
	result := ExecuteManualCleanupResult{DeletedAssetIds: make([]Identifier, 0, len(record.Decisions)), BlockedAssetIds: []Identifier{}}
	for _, decision := range record.Decisions {
		asset := decision.Asset
		asset.State = gen.AssetState("deleted")
		asset.Revision++
		raw, err := platform.CanonicalJSON(asset)
		if err != nil {
			return result, err
		}
		changed, err := tx.ExecContext(ctx, `UPDATE assets SET state='deleted',asset_json=?,updated_at=? WHERE asset_id=? AND site_id=? AND state='delete_pending'`, string(raw), platform.Now(), asset.Ref.AssetId, op.Actor.SiteID)
		if err != nil {
			return result, mapMediaDBError(err, "提交资源已删除状态失败")
		}
		count, err := changed.RowsAffected()
		if err != nil || count != 1 {
			return result, platform.NewError(platform.CodeIntegrityError, false, "已决定删除的资产状态竞争")
		}
		changed, err = tx.ExecContext(ctx, `DELETE FROM manual_cleanup_marks WHERE asset_id=? AND plan_id=? AND deletion_fence=? AND reference_generation=?`, asset.Ref.AssetId, plan.PlanId, decision.Mark.DeletionFence, decision.Mark.ReferenceGeneration)
		if err != nil {
			return result, mapMediaDBError(err, "完成删除精确标记失败")
		}
		count, err = changed.RowsAffected()
		if err != nil || count != 1 {
			return result, platform.NewError(platform.CodeIntegrityError, false, "删除决定精确标记缺失")
		}
		if err := PublishAssetEvent(ctx, op, PublishAssetEventInput{Tx: tx, Asset: asset, EventID: platform.NewIdentifier(), OccurredAt: platform.Now(), ReferenceGeneration: decision.Mark.ReferenceGeneration}); err != nil {
			return result, err
		}
		result.DeletedAssetIds = append(result.DeletedAssetIds, asset.Ref.AssetId)
	}
	if err := s.updateCleanupState(ctx, tx, op, plan, "deleted"); err != nil {
		return result, err
	}
	raw, err := platform.CanonicalJSON(result)
	if err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE manual_cleanup_executions SET result_json=?,result_hash=? WHERE plan_id=? AND approval_hash=? AND result_json IS NULL`, string(raw), platform.HashBytes(raw), plan.PlanId, approval.Hash); err != nil {
		return result, mapMediaDBError(err, "保存人工删除原成功结果失败")
	}
	if err := tx.Commit(); err != nil {
		return result, mapMediaDBError(err, "提交人工删除结果未知，保留决定")
	}
	return result, nil
}

// removeDecidedBlob 只访问服务生成路径；未知失败保留DELETING，缺失仅在既有决定下可恢复。
func (s *localBlobStore) removeDecidedBlob(ctx context.Context, decision cleanupDeletion) error {
	ref := decision.Asset.Ref
	root, err := s.openMediaRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	name := "blobs/" + decision.Asset.SiteId + "/" + ref.Sha256[:2] + "/" + ref.Sha256
	before, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		if err := syncDirectory(filepath.Dir(s.blobPath(decision.Asset.SiteId, ref.Sha256))); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "确认已决定blob缺失的目录失败")
		}
		return nil
	}
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "读取已决定blob失败")
	}
	file, err := openMediaRegular(root, s.root, name, os.O_RDONLY)
	if err != nil {
		return err
	}
	digest := sha256.New()
	size, readErr := io.CopyBuffer(digest, chunkContextReader{ctx, file}, make([]byte, hashBufferSize))
	after, statErr := file.Stat()
	file.Close()
	if readErr != nil {
		return mapMediaDBError(readErr, "核对已决定删除blob失败")
	}
	if statErr != nil || !os.SameFile(before, after) || size != int64(ref.ByteSize) || hex.EncodeToString(digest.Sum(nil)) != ref.Sha256 {
		return platform.NewError(platform.CodeIntegrityError, false, "已决定删除blob的身份、SHA或长度不符")
	}
	actual, err := root.Lstat(name)
	if err != nil || !os.SameFile(after, actual) {
		return platform.NewError(platform.CodeIntegrityError, false, "删除前blob路径身份已改变")
	}
	if err := ctx.Err(); err != nil {
		return mapMediaDBError(err, "删除blob前操作已取消")
	}
	if err := root.Remove(name); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "删除已决定blob失败")
	}
	if err := syncDirectory(filepath.Dir(s.blobPath(decision.Asset.SiteId, ref.Sha256))); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "持久化blob删除失败")
	}
	return nil
}

// recoverCleanupExecutionsLocked 只读明确持久执行记录，启动exclusive调用，不扫描ready产生授权。
func (s *localBlobStore) recoverCleanupExecutionsLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT plan_id FROM manual_cleanup_executions ORDER BY plan_id`)
	if err != nil {
		return mapMediaDBError(err, "读取待恢复人工删除决定失败")
	}
	ids := []Identifier{}
	for rows.Next() {
		var id Identifier
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return mapMediaDBError(err, "读取人工删除恢复索引失败")
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return mapMediaDBError(err, "读取人工删除恢复集合失败")
	}
	for _, id := range ids {
		approval, found, err := s.loadCleanupApproval(ctx, s.db, id)
		if err != nil {
			return err
		}
		if !found || approval.Approval.DataEpoch != s.openedEpoch {
			return platform.NewError(platform.CodeIntegrityError, false, "删除恢复缺原批准或不属于已打开代次")
		}
		record, found, err := s.readCleanupExecution(ctx, s.db, approval)
		if err != nil {
			return err
		}
		if !found {
			return platform.NewError(platform.CodeIntegrityError, false, "人工删除恢复记录缺失")
		}
		if record.Result != nil {
			continue
		}
		a := approval.Approval
		op := OperationContext{CallerService: "media", OperationID: a.OperationId, RootOperationID: a.RootOperationId, DataEpoch: a.DataEpoch, Actor: platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: a.SiteId}}
		if _, err := s.finishCleanupExecutionLocked(ctx, op, approval, record); err != nil {
			return err
		}
	}
	return nil
}

// confirmDeletedAssetLocked 使历史COMMITTED只接受本资产完整终局删除证明。
func (s *localBlobStore) confirmDeletedAssetLocked(ctx context.Context, asset gen.Asset) error {
	var planID Identifier
	if err := s.db.QueryRowContext(ctx, `SELECT plan_id FROM manual_cleanup_deletions WHERE asset_id=?`, asset.Ref.AssetId).Scan(&planID); err != nil {
		return mapMediaDBError(err, "读取已删除资源的人工决定失败")
	}
	approval, found, err := s.loadCleanupApproval(ctx, s.db, planID)
	if err != nil {
		return err
	}
	if !found {
		return platform.NewError(platform.CodeIntegrityError, false, "已删除资源缺人工批准")
	}
	record, found, err := s.readCleanupExecution(ctx, s.db, approval)
	if err != nil {
		return err
	}
	if !found || record.Result == nil || !slices.Contains(record.Result.DeletedAssetIds, asset.Ref.AssetId) {
		return platform.NewError(platform.CodeIntegrityError, false, "已删除资源缺完整终局结果")
	}
	for _, decision := range record.Decisions {
		if decision.Asset.Ref.AssetId == asset.Ref.AssetId {
			expected := decision.Asset
			expected.State = gen.AssetState("deleted")
			expected.Revision++
			a, _ := platform.CanonicalJSON(asset)
			b, _ := platform.CanonicalJSON(expected)
			if string(a) == string(b) {
				return nil
			}
		}
	}
	return platform.NewError(platform.CodeIntegrityError, false, "已删除资源与原决定不符")
}
