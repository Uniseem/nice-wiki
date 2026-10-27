package internal

import (
	"context"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/jobs/internal/gen"
)

type RequestCancelInput struct {
	JobID     platform.Identifier  `json:"jobId"`
	Actor     platform.AuthContext `json:"-"`
	DataEpoch platform.Identifier  `json:"dataEpoch"`
}

type RequestCancelResult = gen.RequestCancelResult

// RequestCancel 只记录取消意图；owner负责停止模型、结算及确认持久安全点。
func RequestCancel(ctx context.Context, op OperationContext, in RequestCancelInput) (RequestCancelResult, error) {
	authorized, err := authorizeJobManagement(ctx, op, in.JobID)
	if err != nil {
		return RequestCancelResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return RequestCancelResult{}, err
	}
	defer work.close()
	job, err := work.readJobRecord(ctx, in.JobID, in.DataEpoch)
	if err != nil {
		return RequestCancelResult{}, err
	}
	if job.owner != authorized.owner || job.jobType != authorized.jobType || job.epoch != authorized.epoch {
		return RequestCancelResult{}, platform.NewError(platform.CodeConflict, false, "任务授权作用域已变化")
	}
	request, _ := json.Marshal(in)
	prior, err := work.receipt(ctx, op, "cancel", request)
	if err != nil {
		return RequestCancelResult{}, err
	}
	if prior != nil {
		var result RequestCancelResult
		if err := json.Unmarshal(prior, &result); err != nil {
			return RequestCancelResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务取消回执损坏")
		}
		return result, nil
	}
	state := job.state
	switch job.state {
	case "queued", "retry-wait":
		state = "cancelled"
	case "leased", "running":
		state = "cancel-requested"
	}
	if state != job.state {
		updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state=?,revision=revision+1,cancel_requested=1,updated_at=? WHERE job_id=? AND data_epoch=? AND revision=? AND state=?`,
			state, work.now.Format(time.RFC3339Nano), job.id, job.epoch, job.revision, job.state)
		if err != nil {
			return RequestCancelResult{}, mapQueueDBError(err)
		}
		if n, err := updated.RowsAffected(); err != nil || n != 1 {
			return RequestCancelResult{}, platform.NewError(platform.CodeConflict, false, "任务状态已变化")
		}
		if err := work.enqueueState(job.id, job.owner, job.epoch, state, job.revision+1); err != nil {
			return RequestCancelResult{}, err
		}
	} else if state == "orphan" && !job.cancelRequested {
		updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET revision=revision+1,cancel_requested=1,updated_at=? WHERE job_id=? AND data_epoch=? AND revision=? AND state='orphan'`, work.now.Format(time.RFC3339Nano), job.id, job.epoch, job.revision)
		if err != nil {
			return RequestCancelResult{}, mapQueueDBError(err)
		}
		if n, err := updated.RowsAffected(); err != nil || n != 1 {
			return RequestCancelResult{}, platform.NewError(platform.CodeConflict, false, "任务状态已变化")
		}
		if err := work.enqueueState(job.id, job.owner, job.epoch, state, job.revision+1); err != nil {
			return RequestCancelResult{}, err
		}
	}
	result := RequestCancelResult{State: state}
	if err := work.saveReceipt(ctx, op, "cancel", request, result); err != nil {
		return RequestCancelResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return RequestCancelResult{}, mapQueueDBError(err)
	}
	return result, nil
}
