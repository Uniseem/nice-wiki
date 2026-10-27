package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
)

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
