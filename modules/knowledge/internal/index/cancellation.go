package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
)

// CancelBuildSideEffects 只处理已持久cancel_requested的本域构建；不退款、不删未知付费引用。
func CancelBuildSideEffects(ctx context.Context, op OperationContext, buildID string) error {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return err
	}
	tx, err := runtime.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return storageError(err)
	}
	record, err := ReadBuildRecord(ctx, tx, op, buildID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	var job sql.NullString
	var state string
	var requested int
	var operationRaw []byte
	err = tx.QueryRowContext(ctx, `SELECT job_id,dispatch_state,cancel_requested,operation_json FROM build_dispatches WHERE build_id=?`, buildID).Scan(&job, &state, &requested, &operationRaw)
	if err == sql.ErrNoRows && string(record.Build.State) == "active" {
		_ = tx.Rollback()
		return nil
	}
	if err != nil {
		_ = tx.Rollback()
		return storageError(err)
	}
	if requested == 0 || state == "cancelled" || state == "completed" {
		_ = tx.Rollback()
		return nil
	}
	ids := []string{}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT invocation_id FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND invocation_id IS NOT NULL AND state NOT IN ('completed','failed','cancelled')`, op.Actor.SiteID, op.DataEpoch, buildID)
	if err != nil {
		_ = tx.Rollback()
		return storageError(err)
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			_ = tx.Rollback()
			return storageError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	_ = tx.Rollback()
	if err != nil {
		return storageError(err)
	}
	if state == "dispatching" && !job.Valid {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "原任务接受结果尚未知，保留取消意图等待原回执或worker认领")
	}
	children, err := runtime.DB.QueryContext(ctx, `SELECT d.extraction_id FROM extraction_dispatches d WHERE d.cancel_requested=1 AND d.dispatch_state NOT IN ('cancelled','completed') AND d.extraction_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?) ORDER BY d.extraction_id`, buildID)
	if err != nil {
		return storageError(err)
	}
	childIDs := []string{}
	for children.Next() {
		var id string
		if err = children.Scan(&id); err != nil {
			children.Close()
			return storageError(err)
		}
		childIDs = append(childIDs, id)
	}
	err = children.Err()
	children.Close()
	if err != nil {
		return storageError(err)
	}
	for _, id := range childIDs {
		if err = CancelExtractionSideEffects(ctx, op, id); err != nil {
			return err
		}
	}
	var original OperationContext
	if json.Unmarshal(operationRaw, &original) != nil || original.DataEpoch != op.DataEpoch || original.Actor.SiteID != op.Actor.SiteID {
		return platform.NewError(platform.CodeIntegrityError, false, "取消的原任务归属损坏")
	}
	service := op
	service.RootOperationID = original.RootOperationID
	service.CallerService = "knowledge"
	service.Actor = platform.AuthContext{SubjectID: "knowledge", Kind: "service", SiteID: op.Actor.SiteID, ExpiresAt: op.Deadline, Grants: []string{}}
	work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), service)
	if job.Valid {
		if runtime.Jobs == nil || runtime.Validators["JobsCancelJobResult"] == nil {
			return platform.NewError(platform.CodeDependencyUnavailable, false, "任务取消生产方未装配")
		}
		key := "knowledge.cancel.job." + platform.HashBytes([]byte(buildID))
		raw, err := runtime.Jobs.CancelJob(work, job.String, struct{}{}, key)
		if err != nil {
			return err
		}
		if err = runtime.Validators["JobsCancelJobResult"](raw); err != nil {
			return err
		}
		var result struct {
			State string `json:"state"`
		}
		if json.Unmarshal(raw, &result) != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "任务取消回执不可读")
		}
		switch result.State {
		case "cancelled", "cancel-requested", "succeeded", "failed", "dead-letter", "orphan":
		default:
			return platform.NewError(platform.CodeIntegrityError, false, "任务取消未确认或状态未知")
		}
	}
	for _, id := range ids {
		if runtime.InferenceClient == nil {
			return platform.NewError(platform.CodeDependencyUnavailable, false, "公共模型取消适配未装配")
		}
		key := "knowledge.cancel.model." + platform.HashBytes([]byte(buildID+":"+id))
		if err = runtime.InferenceClient.CancelInference(work, key, id, "knowledge.cancelled"); err != nil {
			return err
		}
	}
	write, update, cancel, err := BeginIndexWrite(ctx, op)
	if err != nil {
		return err
	}
	defer cancel()
	defer update.Rollback()
	// pending尚未RPC的意图与已请求取消的任务均停止新派发；未知模型结果仍原样保留。
	if _, err = update.ExecContext(write, `UPDATE build_dispatches SET dispatch_state='cancelled',updated_at=? WHERE build_id=? AND cancel_requested=1 AND dispatch_state<>'dispatching'`, platform.Now(), buildID); err != nil {
		return storageError(err)
	}
	if err = update.Commit(); err != nil {
		return storageError(err)
	}
	return nil
}

func CancelDocumentSideEffects(ctx context.Context, op OperationContext, documentID string) error {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return err
	}
	rows, err := runtime.DB.QueryContext(ctx, `SELECT d.build_id FROM build_dispatches d JOIN builds b ON b.build_id=d.build_id WHERE b.site_id=? AND b.data_epoch=? AND b.document_id=? AND d.cancel_requested=1 AND d.dispatch_state NOT IN ('cancelled','completed') ORDER BY d.build_id`, op.Actor.SiteID, op.DataEpoch, documentID)
	if err != nil {
		return storageError(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return storageError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return storageError(err)
	}
	for _, id := range ids {
		if err = CancelBuildSideEffects(ctx, op, id); err != nil {
			return err
		}
	}
	return nil
}
