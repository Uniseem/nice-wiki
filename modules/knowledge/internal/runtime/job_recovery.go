package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/extract"
	indexer "nice-wiki/modules/knowledge/internal/index"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
	"time"
)

type ReadJobRecoveryInput = platform.OwnerRecoveryRequest
type ReadJobRecoveryResult = platform.OwnerRecoveryResult

// ReadJobRecovery 只证明原任务事实；未能证明安全的中间状态保持pending，不发新付费许可。
func ReadJobRecovery(ctx context.Context, op OperationContext, input ReadJobRecoveryInput) (ReadJobRecoveryResult, error) {
	result := ReadJobRecoveryResult{Status: "unknown", PaidResultRefs: []string{}}
	if op.CallerService != "jobs" || op.Actor.Kind != "service" || input.Owner != "knowledge" {
		return result, platform.NewError(platform.CodeForbidden, false, "恢复证明只允许jobs读取本域原任务")
	}
	if input.DataEpoch != op.DataEpoch {
		return result, platform.NewError(platform.CodeStaleResult, false, "恢复请求不是当前数据代次")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return result, err
	}
	// 执行记录与失效租约是否对得上。fence 每次领取递增：相等时逐项核 worker/attempt/取消（人工重试死信时 jobs 的租约已删除、不给 worker）；
	// 记录 fence 更小说明失效的那次领取没来得及接管、什么也没执行，按记录里更早那次执行的事实判断。
	sameClaim := func(fence int64, worker string, attempt int, cancelled bool) bool {
		if fence != input.ExpiredFence {
			return fence < input.ExpiredFence
		}
		return (input.WorkerID == "" || worker == input.WorkerID) && int64(attempt) == input.Attempt && cancelled == input.CancelRequested
	}
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原任务恢复快照不可读")
	}
	defer tx.Rollback()
	// GC不删除最终拥有者证明；即使退役索引和原派发行已回收，仍能恢复真实已完成结果。
	completed, lookup := indexer.LoadExecutionRecord(ctx, tx, input.JobID)
	if lookup == nil && completed.State == "completed" && completed.ResultRef != nil {
		if completed.JobType != input.Type || completed.ObjectID != input.InputRef.ObjectID || completed.InputRef != input.InputRef || completed.Operation.RootOperationID != input.RootOperationID || completed.Operation.Actor.SiteID != op.Actor.SiteID || completed.Operation.DataEpoch != input.DataEpoch || !sameClaim(completed.Fence, completed.WorkerID, completed.Attempt, completed.CancelRequested) {
			return result, platform.NewError(platform.CodeIntegrityError, false, "原完成证明与恢复来源或失效租约不一致")
		}
		result.Status = "completed"
		result.ResultRef = completed.ResultRef
		result.PaidResultRefs = completed.PaidRefs
		return result, nil
	}
	if lookup != nil {
		var failure *platform.Error
		if !errors.As(lookup, &failure) || failure.Code != platform.CodeNotFound {
			return result, lookup
		}
	}
	original, state, jobsOperation, site, cancelled, err := indexer.ApprovedJobInput(ctx, tx, input.Type, input.InputRef.ObjectID)
	if err != nil {
		var failure *platform.Error
		if errors.As(err, &failure) && failure.Code == platform.CodeNotFound {
			return result, nil
		}
		return result, err
	}
	// 任务先进了死信、之后本域才取消（文档撤回、删除或有了新版本）时，jobs 不再给死信记取消；这不是数据损坏，只是不必重试。
	if cancelled && !input.CancelRequested {
		return result, platform.NewError(platform.CodeConflict, false, "这个任务已被取消（例如所属文档已撤回或有了新版本），不需要重试")
	}
	if site != op.Actor.SiteID || original.Operation.DataEpoch != input.DataEpoch || original.InputRef != input.InputRef || original.Operation.RootOperationID != input.RootOperationID || original.JobID != input.JobID || jobsOperation != original.Operation.OperationID || cancelled != input.CancelRequested {
		return result, platform.NewError(platform.CodeIntegrityError, false, "恢复请求与本站原批准输入或取消事实不一致")
	}
	result.Status = "pending"
	record, err := indexer.LoadExecutionRecord(ctx, tx, input.JobID)
	if err == nil {
		if record.JobType != input.Type || record.ObjectID != input.InputRef.ObjectID || record.InputRef != input.InputRef || record.Operation.RootOperationID != input.RootOperationID || record.Operation.OperationID != jobsOperation || record.Operation.Actor.SiteID != site || record.Operation.DataEpoch != input.DataEpoch || !sameClaim(record.Fence, record.WorkerID, record.Attempt, record.CancelRequested) {
			return result, platform.NewError(platform.CodeIntegrityError, false, "恢复请求与原worker/fence/attempt不一致")
		}
		result.PaidResultRefs, err = indexer.ReadPaidResultRefs(ctx, tx, original.Operation, record.ObjectID)
		if err != nil {
			return result, err
		}
		if record.State == "completed" && record.ResultRef != nil {
			result.Status = "completed"
			result.ResultRef = record.ResultRef
			return result, nil
		}
		if record.LeaseUntil.After(time.Now()) {
			return result, nil
		}
		// 只读MG必须锁外，完成后重开SQL快照并重新核fence/cancel；不只等待本地旧unknown。
		_ = tx.Rollback()
		if err = extract.ValidateRecoveredExtractions(ctx, op, record.ObjectID); err != nil {
			return result, err
		}
		if err = modelruntime.ObserveOwnerModelSteps(ctx, op, record.ObjectID); err != nil {
			return result, err
		}
		tx, err = runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "模型观察后恢复快照不可读")
		}
		defer tx.Rollback()
		record, err = indexer.LoadExecutionRecord(ctx, tx, input.JobID)
		if err != nil {
			return result, err
		}
		if !sameClaim(record.Fence, record.WorkerID, record.Attempt, record.CancelRequested) || record.InputRef != input.InputRef {
			return result, platform.NewError(platform.CodeStaleResult, false, "模型观察期间原恢复租约或取消事实变化")
		}
		result.PaidResultRefs, err = indexer.ReadPaidResultRefs(ctx, tx, original.Operation, record.ObjectID)
		if err != nil {
			return result, err
		}
		if record.State == "completed" && record.ResultRef != nil {
			result.Status = "completed"
			result.ResultRef = record.ResultRef
			return result, nil
		}
		if record.LeaseUntil.After(time.Now()) {
			return result, nil
		}
		var unsettled int
		err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND (owner_id=? OR owner_id IN (SELECT o.extraction_id FROM extraction_origins o WHERE o.build_id=? AND NOT EXISTS(SELECT 1 FROM execution_records f WHERE f.object_id=o.extraction_id AND f.job_type='knowledge.extract' AND f.state='completed' AND f.result_ref_json IS NOT NULL) AND NOT EXISTS(SELECT 1 FROM extraction_recovered_results r WHERE r.extraction_id=o.extraction_id))) AND dispatch_started=1 AND (state NOT IN ('completed','failed','cancelled','not_accepted') OR (state='not_accepted' AND (not_accepted_json IS NULL OR not_accepted_hash='')) OR (state<>'not_accepted' AND result_json IS NULL)))+(SELECT count(*) FROM extraction_dispatches d WHERE d.extraction_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?) AND d.dispatch_state NOT IN ('completed','cancelled') AND NOT EXISTS(SELECT 1 FROM extraction_recovered_results r WHERE r.extraction_id=d.extraction_id))`, site, input.DataEpoch, record.ObjectID, record.ObjectID, record.ObjectID).Scan(&unsettled)
		if err != nil {
			return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原费用及子任务恢复事实不可读")
		}
		if unsettled == 0 {
			// 最终paid结果hash逐项回验；不以终态字符串代替原结果完整性。
			rows, err := tx.QueryContext(ctx, `SELECT state,result_json,result_hash,not_accepted_json,not_accepted_hash,owner_id,step_key,operation_id,input_hash,COALESCE((SELECT operation_json FROM build_dispatches WHERE build_id=model_steps.owner_id),(SELECT operation_json FROM extraction_dispatches WHERE extraction_id=model_steps.owner_id)) FROM model_steps WHERE site_id=? AND data_epoch=? AND (owner_id=? OR owner_id IN (SELECT o.extraction_id FROM extraction_origins o WHERE o.build_id=? AND NOT EXISTS(SELECT 1 FROM execution_records f WHERE f.object_id=o.extraction_id AND f.job_type='knowledge.extract' AND f.state='completed' AND f.result_ref_json IS NOT NULL) AND NOT EXISTS(SELECT 1 FROM extraction_recovered_results r WHERE r.extraction_id=o.extraction_id))) AND dispatch_started=1`, site, input.DataEpoch, record.ObjectID, record.ObjectID)
			if err != nil {
				return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型结果证明不可读")
			}
			for rows.Next() {
				var state, hash, negativeHash, owner, key, operation, inputHash string
				var body, negative, metadata []byte
				if err = rows.Scan(&state, &body, &hash, &negative, &negativeHash, &owner, &key, &operation, &inputHash, &metadata); err != nil {
					rows.Close()
					return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型结果证明不可读")
				}
				if state == "not_accepted" {
					var originalModel OperationContext
					if json.Unmarshal(metadata, &originalModel) != nil || originalModel.OperationID != operation || originalModel.Actor.SiteID != site || originalModel.DataEpoch != input.DataEpoch || originalModel.CostClass != platform.CostClassBackground {
						rows.Close()
						return result, platform.NewError(platform.CodeIntegrityError, false, "原键未接受观察缺真实原root来源")
					}
					if err = modelruntime.ValidateModelNonAcceptance(negative, negativeHash, site, input.DataEpoch, owner, key, operation, originalModel.RootOperationID, inputHash); err != nil {
						rows.Close()
						return result, err
					}
				} else if hash != platform.HashBytes(body) || !json.Valid(body) {
					rows.Close()
					return result, platform.NewError(platform.CodeIntegrityError, false, "原模型终态结果hash损坏")
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原模型结果证明不可读")
			}
			var workflowVersion int
			query := `SELECT workflow_version FROM builds WHERE build_id=? AND site_id=? AND data_epoch=?`
			if input.Type == "knowledge.extract" {
				query = `SELECT workflow_version FROM extractions WHERE extraction_id=? AND site_id=? AND data_epoch=?`
			}
			if err = tx.QueryRowContext(ctx, query, record.ObjectID, site, input.DataEpoch).Scan(&workflowVersion); err != nil {
				return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原任务恢复版本不可读")
			}
			_ = tx.Rollback()
			finalFees, err := modelruntime.OwnerFeesFinal(ctx, op, record.ObjectID, original.Operation.RootOperationID)
			if err != nil {
				return result, err
			}
			if !finalFees {
				return result, nil
			}
			tx, err = runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "费用证明后原任务不可读")
			}
			defer tx.Rollback()
			current, err := indexer.LoadExecutionRecord(ctx, tx, input.JobID)
			if err != nil {
				return result, err
			}
			if !sameClaim(current.Fence, current.WorkerID, current.Attempt, current.CancelRequested) {
				return result, platform.NewError(platform.CodeStaleResult, false, "费用证明期间任务归属或取消变化")
			}
			if current.State == "completed" && current.ResultRef != nil {
				result.Status = "completed"
				result.ResultRef = current.ResultRef
				result.PaidResultRefs = current.PaidRefs
				return result, nil
			}
			if current.LeaseUntil.After(time.Now()) {
				return result, nil
			}
			if workflowVersion == 1 {
				result.Status = "safeToRetry"
				return result, nil
			} // 过期仅供jobs按DEADLINE_EXCEEDED分类，不延长原批准；取消仍按原事实终止。
		}
		// 已执行阶段可能含原MG未知费用或已写中间结果；只保留事实，不猜测其可重派。
		return result, nil
	}
	var failure *platform.Error
	if !errors.As(err, &failure) || failure.Code != platform.CodeNotFound {
		return result, err
	}
	var workflowVersion int
	var objectState string
	var qualified int
	switch input.Type {
	case "knowledge.build":
		err = tx.QueryRowContext(ctx, `SELECT workflow_version,state,CASE WHEN lease_id='' AND lease_until='' THEN 1 ELSE 0 END FROM builds WHERE build_id=? AND site_id=? AND data_epoch=?`, input.InputRef.ObjectID, site, input.DataEpoch).Scan(&workflowVersion, &objectState, &qualified)
	case "knowledge.extract":
		err = tx.QueryRowContext(ctx, `SELECT workflow_version,state,CASE WHEN lease_id='' AND lease_until='' AND result_json IS NULL THEN 1 ELSE 0 END FROM extractions WHERE extraction_id=? AND site_id=? AND data_epoch=?`, input.InputRef.ObjectID, site, input.DataEpoch).Scan(&workflowVersion, &objectState, &qualified)
	}
	if err != nil {
		return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原任务执行标记不可读")
	}
	if workflowVersion != 1 || qualified != 1 || (objectState != "queued" && objectState != "cancelled" && objectState != "failed" && objectState != "superseded") || (state != "dispatched" && state != "cancelled") {
		return result, nil
	}
	var facts int
	err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=?)+(SELECT count(*) FROM extraction_origins WHERE build_id=?)+(SELECT count(*) FROM chunks WHERE build_id=?)`, site, input.DataEpoch, input.InputRef.ObjectID, input.InputRef.ObjectID, input.InputRef.ObjectID).Scan(&facts)
	if err != nil {
		return result, platform.NormalizeIOError(err, platform.CodeInternal, false, "原任务执行证据不可读")
	}
	if facts != 0 {
		return result, nil
	}
	// 新版入口在任何业务执行前先原子存execution_records；缺行且无阶段事实才证明从未执行。
	// 领取后未及接管（开始前已请求取消、首次续租即见取消、或 worker 中途退出）同样缺行：失效时 jobs 已换新 fence，旧 worker 不可能再开始。
	result.Status = "safeToRetry"
	return result, nil
}
