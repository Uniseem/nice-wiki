package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
)

// OwnerRecoveryChecker 必须由实际owner版本化HTTP操作装配；不能以进程存活代替持久确认。
// 该边界不执行任务或模型。未装配、不可达、pending和unknown都保留orphan。
type OwnerRecoveryChecker func(context.Context, OperationContext, OwnerRecoveryRequest) (OwnerRecoveryResult, error)

type OwnerRecoveryRequest struct {
	JobID           string
	Owner           string
	Type            string
	InputRef        ObjectRef
	RootOperationID string
	DataEpoch       string
	ExpiredFence    int64
	WorkerID        string
	Attempt         int64
	CancelRequested bool
}

type OwnerRecoveryResult struct {
	// completed：已持久完成；safeToRetry：已确认无未决费用，复用同根操作及已有阶段安全重试。
	// pending/unknown：未到可证明的安全状态，不能重跑。
	Status         string
	ResultRef      *ObjectRef
	PaidResultRefs []string
}

type orphanCandidate struct {
	job     jobRecord
	request OwnerRecoveryRequest
}

type ReapExpiredLeasesInput struct {
	Now       time.Time
	DataEpoch platform.Identifier
}

type ReapExpiredLeasesResult struct{ Count int64 }

type RequeueDueInput struct {
	Now       time.Time
	BatchSize int
	DataEpoch platform.Identifier
}

type RequeueDueResult struct{ Count int64 }

// ReapExpiredLeases 先隔离失效执行权，再在事务外向owner确认持久阶段。
func ReapExpiredLeases(ctx context.Context, op OperationContext, in ReapExpiredLeasesInput) (ReapExpiredLeasesResult, error) {
	if op.CallerService != "jobs" {
		return ReapExpiredLeasesResult{}, platform.NewError(platform.CodeForbidden, false, "仅jobs后台可以回收租约")
	}
	candidates, count, err := collectOrphans(ctx, op, in)
	if err != nil {
		return ReapExpiredLeasesResult{}, err
	}
	result := ReapExpiredLeasesResult{Count: int64(count)}
	store := currentStore()
	if store == nil {
		return result, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	checker := store.recovery
	store.mu.RUnlock()
	if checker == nil {
		return result, nil
	}
	for _, candidate := range candidates {
		recovery, err := checker(ctx, op, candidate.request)
		if err != nil {
			continue
		}
		changed, err := reconcileOrphan(ctx, op, candidate, recovery)
		if err != nil {
			return result, err
		}
		if changed {
			result.Count++
		}
	}
	return result, nil
}

func collectOrphans(ctx context.Context, op OperationContext, in ReapExpiredLeasesInput) ([]orphanCandidate, int, error) {
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return nil, 0, err
	}
	defer work.close()
	if work.runtime.Runtime.Maintenance || work.runtime.SafePointRequested {
		return nil, 0, nil
	}
	rows, err := work.tx.QueryContext(ctx, `SELECT j.job_id,j.owner,j.state,j.revision,j.attempt,j.fencing_counter,COALESCE(l.worker_id,''),l.expires_at,COALESCE(l.fencing_token,j.fencing_counter),j.root_operation_id
		FROM jobs j LEFT JOIN leases l ON l.job_id=j.job_id WHERE j.data_epoch=?
		AND ((j.root_operation_id='' AND j.state IN ('queued','leased','running','retry-wait','cancel-requested'))
		OR (j.state IN ('leased','running','cancel-requested') AND julianday(l.expires_at)<=julianday(?)))
		ORDER BY COALESCE(julianday(l.expires_at),0),j.job_id LIMIT 100`, in.DataEpoch, in.Now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, 0, mapQueueDBError(err)
	}
	type expiredLease struct {
		job    jobRecord
		worker string
	}
	expired := []expiredLease{}
	for rows.Next() {
		item := expiredLease{}
		item.job.epoch = in.DataEpoch
		var expires sql.NullString
		var rootOperationID string
		var token int64
		if err := rows.Scan(&item.job.id, &item.job.owner, &item.job.state, &item.job.revision, &item.job.attempt, &item.job.fence, &item.worker, &expires, &token, &rootOperationID); err != nil {
			_ = rows.Close()
			return nil, 0, mapQueueDBError(err)
		}
		due := rootOperationID == ""
		if expires.Valid {
			expiry, err := time.Parse(time.RFC3339Nano, expires.String)
			if err != nil {
				_ = rows.Close()
				return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "任务租约截止损坏")
			}
			due = due || !expiry.After(in.Now)
		}
		if token != item.job.fence {
			_ = rows.Close()
			return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "任务租约与持久fence不一致")
		}
		if due {
			expired = append(expired, item)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, mapQueueDBError(err)
	}
	_ = rows.Close()
	for _, item := range expired {
		job := item.job
		updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state='orphan',fencing_counter=fencing_counter+1,revision=revision+1,
			cancel_requested=CASE WHEN state='cancel-requested' THEN 1 ELSE cancel_requested END,last_error_code=?,updated_at=?
			WHERE job_id=? AND data_epoch=? AND state=? AND revision=? AND fencing_counter=?`, platform.CodeDeadlineExceeded, work.now.Format(time.RFC3339Nano), job.id, job.epoch, job.state, job.revision, job.fence)
		if err != nil {
			return nil, 0, mapQueueDBError(err)
		}
		if n, err := updated.RowsAffected(); err != nil || n != 1 {
			return nil, 0, leaseLost()
		}
		if _, err := work.tx.ExecContext(ctx, `DELETE FROM leases WHERE job_id=? AND fencing_token=?`, job.id, job.fence); err != nil {
			return nil, 0, mapQueueDBError(err)
		}
		if _, err := work.tx.ExecContext(ctx, `INSERT INTO job_orphans(job_id,expired_fence,worker_id,next_check_at) VALUES(?,?,?,?)`, job.id, job.fence, item.worker, work.now.Format(time.RFC3339Nano)); err != nil {
			return nil, 0, mapQueueDBError(err)
		}
		if err := work.enqueueState(job.id, job.owner, job.epoch, "orphan", job.revision+1); err != nil {
			return nil, 0, err
		}
	}
	rows, err = work.tx.QueryContext(ctx, `SELECT j.job_id,j.owner,j.type,j.immutable_input_ref,j.root_operation_id,j.revision,j.attempt,j.fencing_counter,j.cancel_requested,o.expired_fence,o.worker_id
		FROM jobs j JOIN job_orphans o ON o.job_id=j.job_id WHERE j.data_epoch=? AND j.state='orphan'
		AND julianday(o.next_check_at)<=julianday(?) ORDER BY julianday(o.next_check_at),j.job_id LIMIT 100`, in.DataEpoch, work.now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, 0, mapQueueDBError(err)
	}
	candidates := []orphanCandidate{}
	for rows.Next() {
		candidate := orphanCandidate{}
		var raw []byte
		if err := rows.Scan(&candidate.job.id, &candidate.job.owner, &candidate.request.Type, &raw, &candidate.request.RootOperationID,
			&candidate.job.revision, &candidate.job.attempt, &candidate.job.fence, &candidate.job.cancelRequested, &candidate.request.ExpiredFence, &candidate.request.WorkerID); err != nil {
			_ = rows.Close()
			return nil, 0, mapQueueDBError(err)
		}
		if err := json.Unmarshal(raw, &candidate.request.InputRef); err != nil {
			_ = rows.Close()
			return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "任务输入引用损坏")
		}
		candidate.job.epoch, candidate.job.state = in.DataEpoch, "orphan"
		candidate.request.JobID, candidate.request.Owner, candidate.request.DataEpoch = candidate.job.id, candidate.job.owner, in.DataEpoch
		candidate.request.Attempt, candidate.request.CancelRequested = candidate.job.attempt, candidate.job.cancelRequested
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, mapQueueDBError(err)
	}
	_ = rows.Close()
	for _, candidate := range candidates {
		if _, err := work.tx.ExecContext(ctx, `UPDATE job_orphans SET next_check_at=? WHERE job_id=?`, work.now.Add(5*time.Second).Format(time.RFC3339Nano), candidate.job.id); err != nil {
			return nil, 0, mapQueueDBError(err)
		}
	}
	if err := work.tx.Commit(); err != nil {
		return nil, 0, mapQueueDBError(err)
	}
	return candidates, len(expired), nil
}

func reconcileOrphan(ctx context.Context, op OperationContext, candidate orphanCandidate, recovery OwnerRecoveryResult) (bool, error) {
	if recovery.Status != "completed" && recovery.Status != "safeToRetry" {
		return false, nil
	}
	work, err := beginLeaseOperation(ctx, op, candidate.job.epoch)
	if err != nil {
		return false, err
	}
	defer work.close()
	if work.runtime.Runtime.Maintenance || work.runtime.SafePointRequested {
		return false, nil
	}
	job, err := work.readJobRecord(ctx, candidate.job.id, candidate.job.epoch)
	if err != nil {
		return false, err
	}
	if job.state != "orphan" || job.fence != candidate.job.fence || job.revision != candidate.job.revision {
		return false, nil
	}
	state := "succeeded"
	var retryAt *time.Time
	errorCode := ""
	if recovery.Status == "completed" {
		if recovery.ResultRef != nil && recovery.ResultRef.Owner != job.owner {
			return false, platform.NewError(platform.CodeValidation, false, "恢复结果引用不属于任务owner")
		}
	} else {
		if candidate.request.RootOperationID == "" && !job.cancelRequested {
			return false, nil
		}
		code := platform.CodeDeadlineExceeded
		if job.cancelRequested {
			code = platform.CodeCancelled
		}
		failure, err := ClassifyFailure(ctx, op, ClassifyFailureInput{ErrorCode: code, Attempt: job.attempt, CancelRequested: job.cancelRequested})
		if err != nil {
			return false, err
		}
		state, errorCode = failure.TerminalState, code
		if failure.Retryable {
			retry, err := ComputeRetryAt(ctx, op, ComputeRetryAtInput{Attempt: job.attempt, Now: work.now, JobID: job.id})
			if err != nil {
				return false, err
			}
			retryAt = &retry.RetryAt
		}
	}
	var primary *ObjectRef
	if recovery.Status == "completed" {
		primary = recovery.ResultRef
	}
	if err := work.mergePaidResults(ctx, job.id, recovery.PaidResultRefs, primary); err != nil {
		return false, err
	}
	if err := work.finishJob(ctx, job, state, retryAt, errorCode); err != nil {
		return false, err
	}
	if _, err := work.tx.ExecContext(ctx, `DELETE FROM job_orphans WHERE job_id=?`, job.id); err != nil {
		return false, mapQueueDBError(err)
	}
	if err := work.tx.Commit(); err != nil {
		return false, mapQueueDBError(err)
	}
	return true, nil
}

// RequeueDue 只改变调度状态；attempt在下一次实际领取时才增加。
func RequeueDue(ctx context.Context, op OperationContext, in RequeueDueInput) (RequeueDueResult, error) {
	if op.CallerService != "jobs" {
		return RequeueDueResult{}, platform.NewError(platform.CodeForbidden, false, "仅jobs后台可以重排任务")
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return RequeueDueResult{}, err
	}
	defer work.close()
	if work.runtime.Runtime.Maintenance || work.runtime.SafePointRequested {
		return RequeueDueResult{}, nil
	}
	rows, err := work.tx.QueryContext(ctx, `SELECT job_id,owner,revision,not_before FROM jobs WHERE data_epoch=? AND state='retry-wait'
		AND julianday(not_before)<=julianday(?) ORDER BY julianday(not_before),job_id LIMIT ?`, in.DataEpoch, in.Now.Format(time.RFC3339Nano), min(in.BatchSize, 100))
	if err != nil {
		return RequeueDueResult{}, mapQueueDBError(err)
	}
	jobs := []jobRecord{}
	for rows.Next() {
		job := jobRecord{epoch: in.DataEpoch}
		var notBefore string
		if err := rows.Scan(&job.id, &job.owner, &job.revision, &notBefore); err != nil {
			_ = rows.Close()
			return RequeueDueResult{}, mapQueueDBError(err)
		}
		due, err := time.Parse(time.RFC3339Nano, notBefore)
		if err != nil {
			_ = rows.Close()
			return RequeueDueResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务重试截止损坏")
		}
		if !due.After(in.Now) {
			jobs = append(jobs, job)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return RequeueDueResult{}, mapQueueDBError(err)
	}
	_ = rows.Close()
	for _, job := range jobs {
		updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state='queued',revision=revision+1,updated_at=? WHERE job_id=? AND data_epoch=? AND state='retry-wait' AND revision=?`, work.now.Format(time.RFC3339Nano), job.id, job.epoch, job.revision)
		if err != nil {
			return RequeueDueResult{}, mapQueueDBError(err)
		}
		if n, err := updated.RowsAffected(); err != nil || n != 1 {
			return RequeueDueResult{}, platform.NewError(platform.CodeConflict, false, "任务调度状态已变化")
		}
		if err := work.enqueueState(job.id, job.owner, job.epoch, "queued", job.revision+1); err != nil {
			return RequeueDueResult{}, err
		}
	}
	if err := work.tx.Commit(); err != nil {
		return RequeueDueResult{}, mapQueueDBError(err)
	}
	return RequeueDueResult{Count: int64(len(jobs))}, nil
}
