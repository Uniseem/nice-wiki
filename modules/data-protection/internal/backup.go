package protection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/data-protection/internal/gen"
)

type CreateBackupInput struct {
	IncludeSecrets bool                 `json:"includeSecrets"`
	Actor          platform.AuthContext `json:"-"`
	DataEpoch      string               `json:"dataEpoch"`
}
type CreateBackupResult = gen.CreateBackupResult

// CreateBackup 先持久原批准/不可变输入/状态事件，再单次提交jobs；网络失败保留真实意图供同操作重试。
func CreateBackup(ctx context.Context, op OperationContext, in CreateBackupInput) (CreateBackupResult, error) {
	if err := requireAdministrator(op); err != nil {
		return CreateBackupResult{}, err
	}
	store, err := Catalog(op)
	if err != nil {
		return CreateBackupResult{}, err
	}
	raw, err := platform.CanonicalJSON(in)
	if err != nil {
		return CreateBackupResult{}, err
	}
	inputHash := platform.HashBytes(raw)
	var id, priorHash string
	err = store.Control.QueryRowContext(ctx, `SELECT backup_id,input_hash FROM backup_jobs WHERE subject_id=? AND operation_id=?`, op.Actor.SubjectID, op.OperationID).Scan(&id, &priorHash)
	if err == nil {
		if priorHash != inputHash {
			return CreateBackupResult{}, platform.NewError(platform.CodeConflict, false, "同备份操作原输入不同")
		}
		return submitBackupJob(ctx, op, store, id)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CreateBackupResult{}, storageError(err)
	}
	if _, err = ValidateBackupRequest(ctx, op, ValidateBackupRequestInput{IncludeSecrets: in.IncludeSecrets, Actor: op.Actor, DataEpoch: in.DataEpoch}); err != nil {
		return CreateBackupResult{}, err
	}
	id = platform.NewIdentifier()
	inputID := platform.NewIdentifier()
	inputJSON, _ := platform.CanonicalJSON(map[string]string{"backupId": id})
	inputRef := platform.ObjectRef{Owner: "data-protection", ObjectID: inputID, Version: 1, SHA256: platform.HashBytes(inputJSON), MediaType: "application/json", ByteSize: int64(len(inputJSON))}
	refJSON, _ := platform.CanonicalJSON(inputRef)
	keyID := ""
	for key := range store.Keyring {
		keyID = key
	}
	now := platform.Now()
	tx, err := store.Control.BeginTx(ctx, nil)
	if err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO backup_runs(backup_id,source_epoch,status,key_id,include_secrets,approved_by,approval_permission_revision,approval_kind,site_id,root_operation_id,input_hash,approved_at,created_at) VALUES(?,?,'requested',?,?,?,?,'manual',?,?,?,?,?)`, id, op.DataEpoch, keyID, in.IncludeSecrets, op.Actor.SubjectID, op.Actor.PermissionRevision, op.Actor.SiteID, op.RootOperationID, inputHash, now, now)
	if err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO backup_jobs(backup_id,subject_id,operation_id,root_operation_id,site_id,source_epoch,input_hash,input_object_id,input_ref_json,input_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, op.Actor.SubjectID, op.OperationID, op.RootOperationID, op.Actor.SiteID, op.DataEpoch, inputHash, inputID, string(refJSON), string(inputJSON), now)
	if err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	if err = enqueueBackupState(ctx, tx, id, op.DataEpoch, "requested", ""); err != nil {
		return CreateBackupResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	return submitBackupJob(ctx, op, store, id)
}

func enqueueBackupState(ctx context.Context, tx *sql.Tx, backupID, epoch, state, manifestHash string) error {
	if state == "ready" && manifestHash == "" {
		return platform.NewError(platform.CodeIntegrityError, false, "完整备份不能缺少真实清单hash")
	}
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM vault_meta WHERE singleton=1`).Scan(&revision); err != nil {
		return storageError(err)
	}
	if revision == math.MaxInt64 {
		return platform.NewError(platform.CodeBudgetExceeded, false, "备份目录修订号已达上限")
	}
	revision++
	if _, err := tx.ExecContext(ctx, `UPDATE vault_meta SET revision=? WHERE singleton=1`, revision); err != nil {
		return storageError(err)
	}
	payload := map[string]any{"backupId": backupID, "state": state, "sourceEpoch": epoch}
	if manifestHash != "" {
		payload["manifestHash"] = manifestHash
	}
	raw, _ := platform.CanonicalJSON(payload)
	return outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "data-protection.backup-state-changed.v1", EventVersion: 1, Producer: "data-protection", SubjectID: backupID, Revision: revision, OccurredAt: platform.Now(), DataEpoch: epoch, Payload: raw})
}

// submitBackupJob 不拥有任务领取/重试队列；只提交已持久批准的原输入，并保留jobs真实原回执。
func submitBackupJob(ctx context.Context, op OperationContext, store *Store, id string) (CreateBackupResult, error) {
	var refJSON, inputJSON, recorded, operationID, rootID, siteID, epoch string
	if err := store.Control.QueryRowContext(ctx, `SELECT input_ref_json,input_json,job_result_json,operation_id,root_operation_id,site_id,source_epoch FROM backup_jobs WHERE backup_id=?`, id).Scan(&refJSON, &inputJSON, &recorded, &operationID, &rootID, &siteID, &epoch); err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	if epoch != op.DataEpoch {
		return CreateBackupResult{}, platform.NewError(platform.CodeVersionMismatch, false, "原备份意图属于旧代次")
	}
	if recorded != "" {
		var result CreateBackupResult
		if json.Unmarshal([]byte(recorded), &result) != nil || result.BackupId != id {
			return CreateBackupResult{}, platform.NewError(platform.CodeIntegrityError, false, "备份原任务回执损坏")
		}
		return result, nil
	}
	var ref platform.ObjectRef
	var input struct {
		BackupID string `json:"backupId"`
	}
	if json.Unmarshal([]byte(refJSON), &ref) != nil || json.Unmarshal([]byte(inputJSON), &input) != nil || input.BackupID != id || ref.Owner != "data-protection" || ref.Version != 1 || ref.MediaType != "application/json" || ref.SHA256 != platform.HashBytes([]byte(inputJSON)) || ref.ByteSize != int64(len(inputJSON)) {
		return CreateBackupResult{}, platform.NewError(platform.CodeIntegrityError, false, "备份任务原输入字节与引用不一致")
	}
	if store.Jobs == nil || store.PayloadValidators["JobsCreateJobResult"] == nil {
		return CreateBackupResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "备份任务提交端口未装配")
	}
	serviceOp := op
	serviceOp.OperationID = operationID
	serviceOp.RootOperationID = rootID
	serviceOp.CallerService = "data-protection"
	serviceOp.Actor = platform.AuthContext{Kind: "service", SubjectID: "data-protection", SiteID: siteID}
	serviceOp.CostClass = ""
	callCtx := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), serviceOp)
	raw, err := store.Jobs.CreateJob(callCtx, map[string]any{"owner": "data-protection", "type": "data-protection.backup", "inputRef": ref, "dedupeKey": "backup:" + id}, "backup-"+id)
	if err != nil {
		return CreateBackupResult{}, err
	}
	if err = store.PayloadValidators["JobsCreateJobResult"](raw); err != nil {
		return CreateBackupResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务提交响应不符合合同")
	}
	var response struct {
		Job gen.AcceptedJob `json:"job"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return CreateBackupResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务提交结果不可读")
	}
	result := CreateBackupResult{BackupId: id, Job: response.Job}
	resultJSON, _ := platform.CanonicalJSON(result)
	tx, err := store.Control.BeginTx(ctx, nil)
	if err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	defer tx.Rollback()
	var previous string
	if err = tx.QueryRowContext(ctx, `SELECT job_result_json FROM backup_jobs WHERE backup_id=?`, id).Scan(&previous); err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	if previous != "" {
		var prior CreateBackupResult
		if json.Unmarshal([]byte(previous), &prior) != nil || prior.BackupId != id || prior.Job.JobId != result.Job.JobId {
			return CreateBackupResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务重复提交回执不一致")
		}
		return prior, nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE backup_jobs SET job_id=?,job_result_json=? WHERE backup_id=?`, response.Job.JobId, string(resultJSON), id); err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	if err = tx.Commit(); err != nil {
		return CreateBackupResult{}, storageError(err)
	}
	return result, nil
}
