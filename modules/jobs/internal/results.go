package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/jobs/internal/gen"
)

type CompleteJobInput struct {
	JobID          platform.Identifier `json:"jobId"`
	WorkerID       string              `json:"workerId"`
	FencingToken   int64               `json:"fencingToken"`
	ResultRef      *ObjectRef          `json:"resultRef,omitempty"`
	PaidResultRefs []string            `json:"paidResultRefs,omitempty"`
	DataEpoch      platform.Identifier `json:"dataEpoch"`
}

type CompleteJobResult = gen.CompleteJobResult

type jobRecord struct {
	id              string
	site            string
	owner           string
	jobType         string
	epoch           string
	state           string
	revision        int64
	attempt         int64
	fence           int64
	cancelRequested bool
}

// CompleteJob 只提交owner已经持久的不可变引用，不运行或重新计算领域任务。
func CompleteJob(ctx context.Context, op OperationContext, in CompleteJobInput) (CompleteJobResult, error) {
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return CompleteJobResult{}, err
	}
	defer work.close()
	job, err := work.readJobRecord(ctx, in.JobID, in.DataEpoch)
	if err != nil {
		return CompleteJobResult{}, err
	}
	leaseInput := StartJobInput{JobID: in.JobID, WorkerID: in.WorkerID, FencingToken: in.FencingToken, DataEpoch: in.DataEpoch}
	request, _ := json.Marshal(in)
	var result CompleteJobResult
	found, err := work.terminalReplay(ctx, op, job, leaseInput, "complete", request, &result)
	if err != nil {
		return CompleteJobResult{}, err
	}
	if found {
		return result, nil
	}
	lease, err := work.readLease(ctx, op, leaseInput)
	if err != nil {
		return CompleteJobResult{}, err
	}
	if lease.state != "running" && lease.state != "cancel-requested" {
		return CompleteJobResult{}, leaseLost()
	}
	if in.ResultRef != nil && in.ResultRef.Owner != op.CallerService {
		return CompleteJobResult{}, platform.NewError(platform.CodeValidation, false, "结果引用必须属于任务owner")
	}
	if err := work.mergePaidResults(ctx, in.JobID, in.PaidResultRefs, in.ResultRef); err != nil {
		return CompleteJobResult{}, err
	}
	if err := work.finishJob(ctx, job, "succeeded", nil, ""); err != nil {
		return CompleteJobResult{}, err
	}
	result = CompleteJobResult{State: "succeeded", Revision: int(job.revision + 1)}
	if err := work.saveTerminal(ctx, op, leaseInput, "complete", request, result); err != nil {
		return CompleteJobResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return CompleteJobResult{}, mapQueueDBError(err)
	}
	return result, nil
}

func (work *leaseOperation) readJobRecord(ctx context.Context, jobID string, epoch platform.Identifier) (jobRecord, error) {
	job := jobRecord{id: jobID}
	err := work.tx.QueryRowContext(ctx, `SELECT owner,type,data_epoch,state,revision,attempt,fencing_counter,cancel_requested FROM jobs WHERE job_id=?`, jobID).
		Scan(&job.owner, &job.jobType, &job.epoch, &job.state, &job.revision, &job.attempt, &job.fence, &job.cancelRequested)
	if errors.Is(err, sql.ErrNoRows) {
		return jobRecord{}, platform.NewError(platform.CodeNotFound, false, "任务不存在")
	}
	if err != nil {
		return jobRecord{}, mapQueueDBError(err)
	}
	if job.epoch != epoch {
		return jobRecord{}, epochMismatch()
	}
	return job, nil
}

// terminalReplay 在失去租约后只允许重放已经提交的同一结果，不能恢复旧执行权。
func (work *leaseOperation) terminalReplay(ctx context.Context, op OperationContext, job jobRecord, in StartJobInput, kind string, request []byte, result any) (bool, error) {
	if job.owner != op.CallerService {
		return false, platform.NewError(platform.CodeForbidden, false, "只能写回本服务的任务")
	}
	if job.fence != in.FencingToken {
		return false, leaseLost()
	}
	prior, err := work.receipt(ctx, op, kind, request)
	if err != nil {
		return false, err
	}
	var storedKind, workerID, hash string
	var outcome []byte
	err = work.tx.QueryRowContext(ctx, `SELECT operation_kind,worker_id,request_hash,result_json FROM job_outcomes WHERE job_id=? AND fencing_token=?`, in.JobID, in.FencingToken).
		Scan(&storedKind, &workerID, &hash, &outcome)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, mapQueueDBError(err)
	}
	if err == nil {
		if workerID != in.WorkerID {
			return false, leaseLost()
		}
		if storedKind != kind || hash != platform.HashBytes(request) {
			return false, platform.NewError(platform.CodeConflict, false, "该租约已提交不同终局结果")
		}
		if prior == nil {
			if err := json.Unmarshal(outcome, result); err != nil {
				return false, platform.NewError(platform.CodeIntegrityError, false, "任务终局回执损坏")
			}
			if err := work.saveReceipt(ctx, op, kind, request, json.RawMessage(outcome)); err != nil {
				return false, err
			}
			if err := work.tx.Commit(); err != nil {
				return false, mapQueueDBError(err)
			}
		}
		prior = outcome
	}
	if prior == nil {
		return false, nil
	}
	if err := json.Unmarshal(prior, result); err != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "任务终局回执损坏")
	}
	return true, nil
}

func (work *leaseOperation) saveTerminal(ctx context.Context, op OperationContext, in StartJobInput, kind string, request []byte, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "任务终局回执编码失败")
	}
	if _, err := work.tx.ExecContext(ctx, `INSERT INTO job_outcomes(job_id,fencing_token,operation_kind,worker_id,request_hash,result_json) VALUES(?,?,?,?,?,?)`,
		in.JobID, in.FencingToken, kind, in.WorkerID, platform.HashBytes(request), encoded); err != nil {
		return mapQueueDBError(err)
	}
	return work.saveReceipt(ctx, op, kind, request, result)
}

func (work *leaseOperation) mergePaidResults(ctx context.Context, jobID string, refs []string, result *ObjectRef) error {
	var resultRef, resultHash string
	var storedRefs []byte
	err := work.tx.QueryRowContext(ctx, `SELECT result_ref,paid_result_refs_json,result_hash FROM job_results WHERE job_id=?`, jobID).
		Scan(&resultRef, &storedRefs, &resultHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return mapQueueDBError(err)
	}
	merged := []string{}
	if err == nil {
		if err := json.Unmarshal(storedRefs, &merged); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "任务付费结果引用损坏")
		}
	}
	seen := make(map[string]bool, len(merged)+len(refs))
	for _, ref := range merged {
		seen[ref] = true
	}
	for _, ref := range refs {
		if !seen[ref] {
			merged = append(merged, ref)
			seen[ref] = true
		}
	}
	if result != nil {
		encoded, err := json.Marshal(result)
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "任务结果引用编码失败")
		}
		resultRef, resultHash = string(encoded), result.Sha256
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "任务付费结果引用编码失败")
	}
	_, err = work.tx.ExecContext(ctx, `INSERT INTO job_results(job_id,result_ref,paid_result_refs_json,result_hash) VALUES(?,?,?,?)
		ON CONFLICT(job_id) DO UPDATE SET result_ref=excluded.result_ref,paid_result_refs_json=excluded.paid_result_refs_json,result_hash=excluded.result_hash`, jobID, resultRef, encoded, resultHash)
	return mapQueueDBError(err)
}

func (work *leaseOperation) finishJob(ctx context.Context, job jobRecord, state string, retryAt *time.Time, errorCode string) error {
	notBefore := work.now
	if retryAt != nil {
		notBefore = *retryAt
	}
	updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state=?,revision=revision+1,not_before=?,updated_at=?,last_error_code=?
		WHERE job_id=? AND data_epoch=? AND state=? AND revision=? AND fencing_counter=?`, state, notBefore.Format(time.RFC3339Nano),
		work.now.Format(time.RFC3339Nano), errorCode, job.id, job.epoch, job.state, job.revision, job.fence)
	if err != nil {
		return mapQueueDBError(err)
	}
	if n, err := updated.RowsAffected(); err != nil || n != 1 {
		return leaseLost()
	}
	if _, err := work.tx.ExecContext(ctx, `DELETE FROM leases WHERE job_id=? AND fencing_token=?`, job.id, job.fence); err != nil {
		return mapQueueDBError(err)
	}
	return work.enqueueState(job.id, job.owner, job.epoch, state, job.revision+1)
}
