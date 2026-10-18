package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
)

// OwnerRecoveryChecker 必须由实际owner版本化HTTP操作装配；不能以进程存活代替持久确认。
// 该边界不执行任务或模型。未装配、不可达、pending和unknown都保留orphan。
type OwnerRecoveryChecker func(context.Context, OperationContext, OwnerRecoveryRequest) (OwnerRecoveryResult, error)

type OwnerRecoveryRequest = platform.OwnerRecoveryRequest

type OwnerRecoveryResult = platform.OwnerRecoveryResult

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
	rows, err := work.tx.QueryContext(ctx, `SELECT j.job_id,j.owner,j.state,j.revision,j.attempt,j.fencing_counter,COALESCE(l.worker_id,''),l.expires_at,COALESCE(l.fencing_token,j.fencing_counter),j.root_operation_id,j.cancel_requested
		FROM jobs j LEFT JOIN leases l ON l.job_id=j.job_id WHERE j.data_epoch=?
		AND ((j.root_operation_id='' AND j.state IN ('queued','leased','running','retry-wait','cancel-requested'))
		OR (j.cancel_requested=1 AND j.state IN ('queued','retry-wait'))
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
		if err := rows.Scan(&item.job.id, &item.job.owner, &item.job.state, &item.job.revision, &item.job.attempt, &item.job.fence, &item.worker, &expires, &token, &rootOperationID, &item.job.cancelRequested); err != nil {
			_ = rows.Close()
			return nil, 0, mapQueueDBError(err)
		}
		due := rootOperationID == "" || (item.job.cancelRequested && (item.job.state == "queued" || item.job.state == "retry-wait"))
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
	rows, err := work.tx.QueryContext(ctx, `SELECT job_id,owner,revision,not_before FROM jobs WHERE data_epoch=? AND state='retry-wait' AND cancel_requested=0
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
		updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state='queued',revision=revision+1,updated_at=? WHERE job_id=? AND data_epoch=? AND state='retry-wait' AND cancel_requested=0 AND revision=?`, work.now.Format(time.RFC3339Nano), job.id, job.epoch, job.revision)
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

// TickSchedulesInput 是一次定时扫描的时刻和当前代次。
type TickSchedulesInput struct {
	Now       time.Time
	DataEpoch platform.Identifier
}

// TickSchedulesResult 只统计本次新插入的任务，去重复用不再计数。
type TickSchedulesResult struct {
	CreatedCount int64 `json:"createdCount"`
}

type dueSchedule struct {
	id       string
	owner    string
	jobType  string
	inputRef string
	cron     string
	timezone string
	revision int64
	nextAt   time.Time
}

// TickSchedules 在维护或安全点期间不生成新tick。到期项只合并最后一次，
// 去重键为scheduleId/tick/epoch，并在独立事务里推进nextAt。
// 已有租约回收和重排逻辑不在这里改动。
func TickSchedules(ctx context.Context, op OperationContext, in TickSchedulesInput) (TickSchedulesResult, error) {
	if err := contextError(ctx); err != nil {
		return TickSchedulesResult{}, err
	}
	if op.CallerService != "jobs" {
		return TickSchedulesResult{}, platform.NewError(platform.CodeForbidden, false, "仅jobs后台可以触发定时")
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return TickSchedulesResult{}, err
	}
	if work.runtime.Runtime.Maintenance || work.runtime.SafePointRequested {
		work.close()
		return TickSchedulesResult{}, nil
	}
	due, readErr := readDueSchedules(ctx, work, in)
	work.close()
	if readErr != nil {
		return TickSchedulesResult{}, readErr
	}
	var created int64
	var failed error
	for _, row := range due {
		if err := contextError(ctx); err != nil {
			return TickSchedulesResult{}, err
		}
		count, err := fireSchedule(ctx, op, in.DataEpoch, in.Now, row)
		if isScheduleLocked(err) {
			if failed != nil {
				return TickSchedulesResult{}, failed
			}
			return TickSchedulesResult{CreatedCount: created}, nil
		}
		if err != nil {
			if failed == nil {
				failed = err
			}
			continue
		}
		created += count
	}
	if failed != nil {
		return TickSchedulesResult{}, failed
	}
	return TickSchedulesResult{CreatedCount: created}, nil
}

// lastDueTick 计算不晚于now的最后一次匹配。nextAt尚未到期时返回false。
func lastDueTick(expression, timezone string, nextAt, now time.Time) (time.Time, bool, error) {
	if nextAt.After(now) {
		return time.Time{}, false, nil
	}
	tick, ok, err := latestCronMatch(expression, timezone, nextAt, now)
	if err != nil {
		return time.Time{}, false, err
	}
	if !ok {
		return time.Time{}, false, platform.NewError(platform.CodeIntegrityError, false, "到期定时没有可证明的触发时刻")
	}
	return tick, true, nil
}

func readDueSchedules(ctx context.Context, work *leaseOperation, in TickSchedulesInput) ([]dueSchedule, error) {
	rows, err := work.tx.QueryContext(ctx, `SELECT schedule_id, owner, job_type, input_ref, cron, timezone, revision, next_fire_at
		FROM schedules WHERE data_epoch=? AND enabled=1 ORDER BY schedule_id`, in.DataEpoch)
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	defer rows.Close()
	due := make([]dueSchedule, 0)
	for rows.Next() {
		var row dueSchedule
		var nextText string
		if err := rows.Scan(&row.id, &row.owner, &row.jobType, &row.inputRef, &row.cron, &row.timezone, &row.revision, &nextText); err != nil {
			return nil, mapQueueDBError(err)
		}
		nextAt, err := time.Parse(time.RFC3339Nano, nextText)
		if err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "定时触发时刻损坏")
		}
		if !nextAt.After(in.Now) {
			row.nextAt = nextAt
			due = append(due, row)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, mapQueueDBError(err)
	}
	return due, nil
}

func fireSchedule(ctx context.Context, op OperationContext, epoch platform.Identifier, now time.Time, row dueSchedule) (int64, error) {
	tick, ok, err := lastDueTick(row.cron, row.timezone, row.nextAt, now)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	next, err := NextFireAt(ctx, op, NextFireAtInput{Expression: row.cron, Timezone: row.timezone, After: tick})
	if err != nil {
		return 0, err
	}
	var ref ObjectRef
	if row.inputRef == "" || json.Unmarshal([]byte(row.inputRef), &ref) != nil || ref.Owner != row.owner {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "定时输入引用损坏")
	}
	// 调度器在进程内以任务owner调用已有CreateJob，满足owner校验；这不是接受客户端声明的调用方。
	// Claim拒绝空根操作，因此本次生成的操作号同时作为根。去重键仍阻止重复任务。
	createOp := op
	createOp.CallerService = row.owner
	createOp.OperationID = platform.NewIdentifier()
	createOp.RootOperationID = createOp.OperationID
	createOp.DataEpoch = epoch
	tickUTC := tick.UTC()
	dedupeKey := row.id + "/" + tickUTC.Format(time.RFC3339Nano) + "/" + string(epoch)
	result, err := CreateJob(ctx, createOp, CreateJobInput{
		Owner: row.owner, Type: row.jobType, InputRef: ref, DedupeKey: dedupeKey, DataEpoch: epoch, NotBefore: &tickUTC,
	})
	if err != nil {
		return 0, err
	}
	created := int64(0)
	if result.Job.OperationId == createOp.OperationID {
		created = 1
	}
	if err := advanceSchedule(ctx, op, epoch, row, next.NextAt, tickUTC); err != nil {
		return 0, err
	}
	return created, nil
}

func advanceSchedule(ctx context.Context, op OperationContext, epoch platform.Identifier, row dueSchedule, nextAt, tick time.Time) error {
	work, err := beginLeaseOperation(ctx, op, epoch)
	if err != nil {
		return err
	}
	defer work.close()
	updated, err := work.tx.ExecContext(ctx, `UPDATE schedules SET next_fire_at=?, last_tick=?, revision=revision+1
		WHERE schedule_id=? AND data_epoch=? AND revision=? AND enabled=1`,
		nextAt.UTC().Format(time.RFC3339Nano), tick.UTC().Format(time.RFC3339Nano), row.id, epoch, row.revision)
	if err != nil {
		return mapQueueDBError(err)
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return mapQueueDBError(err)
	}
	if count != 1 {
		return nil
	}
	if err := work.tx.Commit(); err != nil {
		return mapQueueDBError(err)
	}
	return nil
}

func isScheduleLocked(err error) bool {
	var problem *platform.Error
	return errors.As(err, &problem) && problem.Code == platform.CodeLocked
}
