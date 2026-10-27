package tasks

import (
	"context"

	"nice-wiki/libs/platform"
)

// Client 让 worker 循环直接用共用任务表（实现 platform.TypedWorkerJobsClient）；
// 调用方的上下文里要有可信操作（本服务身份、操作键、数据代次）。
type Client struct{ store *Store }

// NewClient 返回执行端客户端。
func NewClient(store *Store) *Client { return &Client{store: store} }

func operation(ctx context.Context) (platform.OperationContext, error) {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok || op.OperationID == "" || op.DataEpoch == "" || op.CallerService == "" {
		return op, platform.NewError(platform.CodeDependencyUnavailable, false, "任务操作缺少可信操作或代次")
	}
	return op, nil
}

func (c *Client) ClaimJob(ctx context.Context, owner, workerID string, limit int) ([]platform.ClaimedJob, error) {
	return c.ClaimJobTypes(ctx, owner, workerID, limit, nil)
}

func (c *Client) ClaimJobTypes(ctx context.Context, owner, workerID string, limit int, jobTypes []string) ([]platform.ClaimedJob, error) {
	op, err := operation(ctx)
	if err != nil {
		return nil, err
	}
	if owner != op.CallerService {
		return nil, platform.NewError(platform.CodeForbidden, false, "只能领取本服务的任务")
	}
	result, err := c.store.claim(ctx, op, workerID, limit, jobTypes)
	if err != nil {
		return nil, err
	}
	jobs := make([]platform.ClaimedJob, 0, len(result.Jobs))
	for _, job := range result.Jobs {
		jobs = append(jobs, platform.ClaimedJob{JobID: job.Job.JobID, OperationID: job.Job.OperationID, RootOperationID: job.RootOperationID,
			JobType: job.Type, InputRef: job.InputRef, FencingToken: job.FencingToken, Attempt: job.Attempt, DataEpoch: job.DataEpoch,
			ExpiresAt: platform.Timestamp(job.ExpiresAt.Format("2006-01-02T15:04:05.999999999Z07:00"))})
	}
	return jobs, nil
}

func (c *Client) StartJob(ctx context.Context, jobID, workerID string, fencingToken int64) error {
	op, err := operation(ctx)
	if err != nil {
		return err
	}
	return c.store.start(ctx, op, leaseInput{JobID: jobID, WorkerID: workerID, FencingToken: fencingToken, DataEpoch: op.DataEpoch})
}

func (c *Client) HeartbeatJob(ctx context.Context, jobID, workerID string, fencingToken int64) (platform.HeartbeatInfo, error) {
	op, err := operation(ctx)
	if err != nil {
		return platform.HeartbeatInfo{}, err
	}
	result, err := c.store.heartbeat(ctx, op, leaseInput{JobID: jobID, WorkerID: workerID, FencingToken: fencingToken, DataEpoch: op.DataEpoch})
	if err != nil {
		return platform.HeartbeatInfo{}, err
	}
	return platform.HeartbeatInfo{ExpiresAt: platform.Timestamp(result.ExpiresAt.Format("2006-01-02T15:04:05.999999999Z07:00")), CancelRequested: result.CancelRequested}, nil
}

func (c *Client) CheckLease(ctx context.Context, jobID, workerID string, fencingToken int64) (bool, error) {
	op, err := operation(ctx)
	if err != nil {
		return false, err
	}
	if err := c.store.checkLease(ctx, op, leaseInput{JobID: jobID, WorkerID: workerID, FencingToken: fencingToken, DataEpoch: op.DataEpoch}); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) CompleteJob(ctx context.Context, jobID, workerID string, fencingToken int64, result *platform.ObjectRef, paidResultRefs []platform.Identifier) error {
	op, err := operation(ctx)
	if err != nil {
		return err
	}
	_, err = c.store.complete(ctx, op, finishInput{JobID: jobID, WorkerID: workerID, FencingToken: fencingToken, ResultRef: result, PaidResultRefs: paidRefs(paidResultRefs), DataEpoch: op.DataEpoch})
	return err
}

func (c *Client) FailJob(ctx context.Context, jobID, workerID string, fencingToken int64, errorCode string, paidResultRefs []platform.Identifier) error {
	op, err := operation(ctx)
	if err != nil {
		return err
	}
	_, err = c.store.fail(ctx, op, finishInput{JobID: jobID, WorkerID: workerID, FencingToken: fencingToken, ErrorCode: errorCode, PaidResultRefs: paidRefs(paidResultRefs), DataEpoch: op.DataEpoch})
	return err
}

func paidRefs(refs []platform.Identifier) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, string(ref))
	}
	return out
}
