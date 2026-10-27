package tasks

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"nice-wiki/libs/platform"
)

type jobRecord struct {
	id              string
	owner           string
	epoch           string
	state           string
	revision        int64
	attempt         int64
	fence           int64
	cancelRequested bool
}

type finishInput struct {
	JobID          string     `json:"jobId"`
	WorkerID       string     `json:"workerId"`
	FencingToken   int64      `json:"fencingToken"`
	ResultRef      *objectRef `json:"resultRef,omitempty"`
	ErrorCode      string     `json:"errorCode,omitempty"`
	PaidResultRefs []string   `json:"paidResultRefs,omitempty"`
	DataEpoch      string     `json:"dataEpoch"`
}

type finishResult struct {
	State    string     `json:"state"`
	Revision int        `json:"revision,omitempty"`
	RetryAt  *time.Time `json:"retryAt,omitempty"`
}

func (in finishInput) lease() leaseInput {
	return leaseInput{JobID: in.JobID, WorkerID: in.WorkerID, FencingToken: in.FencingToken, DataEpoch: in.DataEpoch}
}

// complete 只记下 owner 已经持久的不可变结果引用，不运行或重算领域任务。
func (s *Store) complete(ctx context.Context, op platform.OperationContext, in finishInput) (finishResult, error) {
	w, err := s.begin(ctx, op)
	if err != nil {
		return finishResult{}, err
	}
	defer w.close()
	job, err := w.readJobRecord(ctx, in.JobID, in.DataEpoch)
	if err != nil {
		return finishResult{}, err
	}
	request, _ := json.Marshal(in)
	var result finishResult
	found, err := w.terminalReplay(ctx, op, job, in.lease(), "complete", request, &result)
	if err != nil || found {
		return result, err
	}
	lease, err := w.readLease(ctx, op, in.lease())
	if err != nil {
		return finishResult{}, err
	}
	if lease.state != "running" && lease.state != "cancel-requested" {
		return finishResult{}, leaseLost()
	}
	if in.ResultRef != nil && in.ResultRef.Owner != op.CallerService {
		return finishResult{}, platform.NewError(platform.CodeValidation, false, "结果引用必须属于任务owner")
	}
	if err := w.mergePaidResults(ctx, in.JobID, in.PaidResultRefs, in.ResultRef); err != nil {
		return finishResult{}, err
	}
	if err := w.finishJob(ctx, job, "succeeded", nil, ""); err != nil {
		return finishResult{}, err
	}
	result = finishResult{State: "succeeded", Revision: int(job.revision + 1)}
	if err := w.saveTerminal(ctx, op, in.lease(), "complete", request, result); err != nil {
		return finishResult{}, err
	}
	return result, dbError(w.tx.Commit())
}

// fail 在 owner 已落到持久安全点后确认失败：可重试的错误按退避表重排，取消请求下确认取消，其余进死信。
func (s *Store) fail(ctx context.Context, op platform.OperationContext, in finishInput) (finishResult, error) {
	w, err := s.begin(ctx, op)
	if err != nil {
		return finishResult{}, err
	}
	defer w.close()
	job, err := w.readJobRecord(ctx, in.JobID, in.DataEpoch)
	if err != nil {
		return finishResult{}, err
	}
	request, _ := json.Marshal(in)
	var result finishResult
	found, err := w.terminalReplay(ctx, op, job, in.lease(), "fail", request, &result)
	if err != nil || found {
		return result, err
	}
	lease, err := w.readLease(ctx, op, in.lease())
	if err != nil {
		return finishResult{}, err
	}
	if lease.state != "running" && lease.state != "cancel-requested" {
		return finishResult{}, leaseLost()
	}
	state, retryable, err := classifyFailure(in.ErrorCode, job.attempt, job.cancelRequested)
	if err != nil {
		return finishResult{}, err
	}
	result.State = state
	if state == "cancelled" {
		if err := w.mergePaidResults(ctx, in.JobID, in.PaidResultRefs, nil); err != nil {
			return finishResult{}, err
		}
		if err := w.finishJob(ctx, job, "cancelled", nil, platform.CodeCancelled); err != nil {
			return finishResult{}, err
		}
	} else {
		if retryable {
			retry := retryAt(in.JobID, job.attempt, w.now)
			result.RetryAt = &retry
		}
		if err := w.mergePaidResults(ctx, in.JobID, in.PaidResultRefs, nil); err != nil {
			return finishResult{}, err
		}
		if err := w.finishJob(ctx, job, state, result.RetryAt, in.ErrorCode); err != nil {
			return finishResult{}, err
		}
		if state == "orphan" {
			if _, err := w.tx.ExecContext(ctx, `INSERT INTO job_orphans(job_id,expired_fence,worker_id,next_check_at) VALUES(?,?,?,?)`,
				job.id, in.FencingToken, in.WorkerID, w.now.Format(time.RFC3339Nano)); err != nil {
				return finishResult{}, dbError(err)
			}
		}
	}
	if err := w.saveTerminal(ctx, op, in.lease(), "fail", request, result); err != nil {
		return finishResult{}, err
	}
	return result, dbError(w.tx.Commit())
}

// classifyFailure 用固定的错误码策略：依赖不可用、超时在第 5 次前重试；请求过取消的确认取消或成孤儿；其余进死信。
func classifyFailure(code string, attempt int64, cancelRequested bool) (string, bool, error) {
	if _, known := platform.ProblemHTTPStatus[code]; !known {
		return "", false, platform.NewError(platform.CodeValidation, false, "任务失败必须使用公共错误码")
	}
	if code == platform.CodeCancelled {
		if !cancelRequested {
			return "", false, platform.NewError(platform.CodeValidation, false, "任务尚未请求安全取消")
		}
		return "cancelled", false, nil
	}
	if cancelRequested {
		return "orphan", false, nil
	}
	if (code == platform.CodeDependencyUnavailable || code == platform.CodeDeadlineExceeded) && attempt < 5 {
		return "retry-wait", true, nil
	}
	return "dead-letter", false, nil
}

// retryAt 按固定退避表（5 秒、30 秒、2 分钟、10 分钟）加确定的抖动算下一次调度。
func retryAt(jobID string, attempt int64, now time.Time) time.Time {
	delays := [...]time.Duration{5 * time.Second, 30 * time.Second, 120 * time.Second, 600 * time.Second}
	hash := sha256.Sum256([]byte(jobID + ":" + strconv.FormatInt(attempt, 10)))
	jitter := time.Duration(binary.BigEndian.Uint64(hash[:8])%1000) * time.Millisecond
	return now.Add(delays[min(max(attempt, 1), 4)-1] + jitter)
}

func (w *work) readJobRecord(ctx context.Context, jobID, epoch string) (jobRecord, error) {
	job := jobRecord{id: jobID}
	err := w.tx.QueryRowContext(ctx, `SELECT owner,data_epoch,state,revision,attempt,fencing_counter,cancel_requested FROM jobs WHERE job_id=?`, jobID).
		Scan(&job.owner, &job.epoch, &job.state, &job.revision, &job.attempt, &job.fence, &job.cancelRequested)
	if errors.Is(err, sql.ErrNoRows) {
		return jobRecord{}, platform.NewError(platform.CodeNotFound, false, "任务不存在")
	}
	if err != nil {
		return jobRecord{}, dbError(err)
	}
	if job.epoch != epoch {
		return jobRecord{}, epochMismatch()
	}
	return job, nil
}

// terminalReplay 失去租约后只允许重放已经提交的同一结果，不能恢复旧执行权。
func (w *work) terminalReplay(ctx context.Context, op platform.OperationContext, job jobRecord, in leaseInput, kind string, request []byte, result any) (bool, error) {
	if job.owner != op.CallerService {
		return false, platform.NewError(platform.CodeForbidden, false, "只能写回本服务的任务")
	}
	if job.fence != in.FencingToken {
		return false, leaseLost()
	}
	prior, err := w.receipt(ctx, op, kind, request)
	if err != nil {
		return false, err
	}
	var storedKind, workerID, hash string
	var outcome []byte
	err = w.tx.QueryRowContext(ctx, `SELECT operation_kind,worker_id,request_hash,result_json FROM job_outcomes WHERE job_id=? AND fencing_token=?`, in.JobID, in.FencingToken).
		Scan(&storedKind, &workerID, &hash, &outcome)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, dbError(err)
	}
	if err == nil {
		if workerID != in.WorkerID {
			return false, leaseLost()
		}
		if storedKind != kind || hash != platform.HashBytes(request) {
			return false, platform.NewError(platform.CodeConflict, false, "该租约已提交不同终局结果")
		}
		if prior == nil {
			if err := w.saveReceipt(ctx, op, kind, request, json.RawMessage(outcome)); err != nil {
				return false, err
			}
			if err := w.tx.Commit(); err != nil {
				return false, dbError(err)
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

func (w *work) saveTerminal(ctx context.Context, op platform.OperationContext, in leaseInput, kind string, request []byte, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "任务终局回执编码失败")
	}
	if _, err := w.tx.ExecContext(ctx, `INSERT INTO job_outcomes(job_id,fencing_token,operation_kind,worker_id,request_hash,result_json) VALUES(?,?,?,?,?,?)`,
		in.JobID, in.FencingToken, kind, in.WorkerID, platform.HashBytes(request), encoded); err != nil {
		return dbError(err)
	}
	return w.saveReceipt(ctx, op, kind, request, result)
}

func (w *work) mergePaidResults(ctx context.Context, jobID string, refs []string, result *objectRef) error {
	var resultRef, resultHash string
	var storedRefs []byte
	err := w.tx.QueryRowContext(ctx, `SELECT result_ref,paid_result_refs_json,result_hash FROM job_results WHERE job_id=?`, jobID).
		Scan(&resultRef, &storedRefs, &resultHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dbError(err)
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
		resultRef, resultHash = string(encoded), string(result.SHA256)
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "任务付费结果引用编码失败")
	}
	_, err = w.tx.ExecContext(ctx, `INSERT INTO job_results(job_id,result_ref,paid_result_refs_json,result_hash) VALUES(?,?,?,?)
		ON CONFLICT(job_id) DO UPDATE SET result_ref=excluded.result_ref,paid_result_refs_json=excluded.paid_result_refs_json,result_hash=excluded.result_hash`, jobID, resultRef, encoded, resultHash)
	return dbError(err)
}

func (w *work) finishJob(ctx context.Context, job jobRecord, state string, retry *time.Time, errorCode string) error {
	notBefore := w.now
	if retry != nil {
		notBefore = *retry
	}
	updated, err := w.tx.ExecContext(ctx, `UPDATE jobs SET state=?,revision=revision+1,not_before=?,updated_at=?,last_error_code=?
		WHERE job_id=? AND data_epoch=? AND state=? AND revision=? AND fencing_counter=?`, state, notBefore.Format(time.RFC3339Nano),
		w.now.Format(time.RFC3339Nano), errorCode, job.id, job.epoch, job.state, job.revision, job.fence)
	if err != nil {
		return dbError(err)
	}
	if n, err := updated.RowsAffected(); err != nil || n != 1 {
		return leaseLost()
	}
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM leases WHERE job_id=? AND fencing_token=?`, job.id, job.fence); err != nil {
		return dbError(err)
	}
	return w.enqueueState(job.id, job.owner, job.epoch, state, job.revision+1)
}
