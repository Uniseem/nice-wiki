package worker

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strconv"

	"nice-wiki/libs/platform"
)

// RawJobsClient与生成的原始HTTP客户端同形；签名由其传输层统一建立。
type RawJobsClient interface {
	ClaimJob(context.Context, any, string) (json.RawMessage, error)
	StartJob(context.Context, string, any, string) (json.RawMessage, error)
	HeartbeatJob(context.Context, string, any, string) (json.RawMessage, error)
	CheckLease(context.Context, string, ...url.Values) (json.RawMessage, error)
	CompleteJob(context.Context, string, any, string) (json.RawMessage, error)
	FailJob(context.Context, string, any, string) (json.RawMessage, error)
}

type ResponseValidators = map[string]func(json.RawMessage) error
type jobsClient struct {
	raw      RawJobsClient
	validate ResponseValidators
}

// NewJobsClient只装配真实生产方schema；不执行RPC或后台任务。
func NewJobsClient(client RawJobsClient, validators ResponseValidators) (platform.WorkerJobsClient, error) {
	if client == nil {
		return nil, platform.NewError(platform.CodeValidation, false, "任务HTTP客户端缺失")
	}
	copied := ResponseValidators{}
	for _, name := range []string{"ClaimJobResult", "StartJobResult", "HeartbeatJobResult", "CheckLeaseResult", "CompleteJobResult", "FailJobResult"} {
		if validators[name] == nil {
			return nil, platform.NewError(platform.CodeValidation, false, "任务生产方响应校验器缺失")
		}
		copied[name] = validators[name]
	}
	return &jobsClient{raw: client, validate: copied}, nil
}

func workerOperation(ctx context.Context) (platform.OperationContext, error) {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok || op.OperationID == "" || op.DataEpoch == "" {
		return op, platform.NewError(platform.CodeDependencyUnavailable, false, "任务HTTP控制缺少可信操作或代次")
	}
	return op, nil
}

func (c *jobsClient) decodeWorkerResponse(schema string, raw json.RawMessage, result any) error {
	if c.validate[schema](raw) != nil || json.Unmarshal(raw, result) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "任务生产方成功响应不符合唯一合同")
	}
	return nil
}

func (c *jobsClient) ClaimJob(ctx context.Context, owner, workerID string, limit int) ([]platform.ClaimedJob, error) {
	return c.ClaimJobTypes(ctx, owner, workerID, limit, nil)
}

func (c *jobsClient) ClaimJobTypes(ctx context.Context, owner, workerID string, limit int, jobTypes []string) ([]platform.ClaimedJob, error) {
	op, err := workerOperation(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := c.raw.ClaimJob(ctx, struct {
		Owner    string   `json:"owner"`
		WorkerID string   `json:"workerId"`
		Limit    int      `json:"limit"`
		JobTypes []string `json:"jobTypes,omitempty"`
	}{owner, workerID, limit, jobTypes}, op.OperationID)
	if err != nil {
		return nil, err
	}
	var result struct {
		Jobs []struct {
			Job struct {
				JobID       string `json:"jobId"`
				OperationID string `json:"operationId"`
				State       string `json:"state"`
			} `json:"job"`
			Owner     string             `json:"owner"`
			Type      string             `json:"type"`
			InputRef  platform.ObjectRef `json:"inputRef"`
			Fence     int64              `json:"fencingToken"`
			Attempt   int                `json:"attempt"`
			Epoch     string             `json:"dataEpoch"`
			ExpiresAt platform.Timestamp `json:"expiresAt"`
			Root      string             `json:"rootOperationId"`
		} `json:"jobs"`
		Fences []int64 `json:"fencingTokens"`
	}
	if err = c.decodeWorkerResponse("ClaimJobResult", raw, &result); err != nil {
		return nil, err
	}
	if len(result.Jobs) > limit || len(result.Fences) != len(result.Jobs) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "领取数量或并行fence不一致")
	}
	jobs := make([]platform.ClaimedJob, 0, len(result.Jobs))
	seen := map[string]bool{}
	for i, job := range result.Jobs {
		if job.Owner != owner || job.Epoch != op.DataEpoch || job.Fence != result.Fences[i] || seen[job.Job.JobID] || (len(jobTypes) > 0 && !slices.Contains(jobTypes, job.Type)) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "领取owner、代次或任务绑定矛盾")
		}
		seen[job.Job.JobID] = true
		jobs = append(jobs, platform.ClaimedJob{JobID: job.Job.JobID, OperationID: job.Job.OperationID,
			RootOperationID: job.Root, JobType: job.Type, InputRef: job.InputRef, FencingToken: job.Fence,
			Attempt: job.Attempt, DataEpoch: job.Epoch, ExpiresAt: job.ExpiresAt})
	}
	return jobs, nil
}

type leaseRequest struct {
	WorkerID string `json:"workerId"`
	Fence    int64  `json:"fencingToken"`
}

func (c *jobsClient) StartJob(ctx context.Context, jobID, workerID string, fencingToken int64) error {
	op, err := workerOperation(ctx)
	if err != nil {
		return err
	}
	raw, err := c.raw.StartJob(ctx, jobID, leaseRequest{workerID, fencingToken}, op.OperationID)
	if err != nil {
		return err
	}
	var result struct {
		State string `json:"state"`
	}
	if err = c.decodeWorkerResponse("StartJobResult", raw, &result); err != nil {
		return err
	}
	if result.State != "running" {
		return platform.NewError(platform.CodeIntegrityError, false, "任务开始回执不是running")
	}
	return nil
}

func (c *jobsClient) HeartbeatJob(ctx context.Context, jobID, workerID string, fencingToken int64) (platform.HeartbeatInfo, error) {
	op, err := workerOperation(ctx)
	if err != nil {
		return platform.HeartbeatInfo{}, err
	}
	raw, err := c.raw.HeartbeatJob(ctx, jobID, leaseRequest{workerID, fencingToken}, op.OperationID)
	if err != nil {
		return platform.HeartbeatInfo{}, err
	}
	var info platform.HeartbeatInfo
	err = c.decodeWorkerResponse("HeartbeatJobResult", raw, &info)
	return info, err
}

func (c *jobsClient) CheckLease(ctx context.Context, jobID, workerID string, fencingToken int64) (bool, error) {
	if _, err := workerOperation(ctx); err != nil {
		return false, err
	}
	raw, err := c.raw.CheckLease(ctx, jobID, url.Values{"workerId": {workerID}, "fencingToken": {strconv.FormatInt(fencingToken, 10)}})
	if err != nil {
		return false, err
	}
	var result struct {
		Valid bool `json:"valid"`
	}
	if err = c.decodeWorkerResponse("CheckLeaseResult", raw, &result); err != nil {
		return false, err
	}
	return result.Valid, nil
}

func (c *jobsClient) CompleteJob(ctx context.Context, jobID, workerID string, fencingToken int64, result *platform.ObjectRef, paidResultRefs []platform.Identifier) error {
	op, err := workerOperation(ctx)
	if err != nil {
		return err
	}
	if paidResultRefs == nil {
		paidResultRefs = []platform.Identifier{}
	}
	body := struct {
		leaseRequest
		Result *platform.ObjectRef   `json:"resultRef,omitempty"`
		Paid   []platform.Identifier `json:"paidResultRefs"`
	}{leaseRequest{workerID, fencingToken}, result, paidResultRefs}
	raw, err := c.raw.CompleteJob(ctx, jobID, body, op.OperationID)
	if err != nil {
		return err
	}
	var receipt struct {
		State string `json:"state"`
	}
	if err = c.decodeWorkerResponse("CompleteJobResult", raw, &receipt); err != nil {
		return err
	}
	if receipt.State != "succeeded" {
		return platform.NewError(platform.CodeIntegrityError, false, "任务完成回执不是succeeded")
	}
	return nil
}

func (c *jobsClient) FailJob(ctx context.Context, jobID, workerID string, fencingToken int64, errorCode string, paidResultRefs []platform.Identifier) error {
	op, err := workerOperation(ctx)
	if err != nil {
		return err
	}
	if paidResultRefs == nil {
		paidResultRefs = []platform.Identifier{}
	}
	body := struct {
		leaseRequest
		ErrorCode string                `json:"errorCode"`
		Paid      []platform.Identifier `json:"paidResultRefs"`
	}{leaseRequest{workerID, fencingToken}, errorCode, paidResultRefs}
	raw, err := c.raw.FailJob(ctx, jobID, body, op.OperationID)
	if err != nil {
		return err
	}
	var result struct {
		State string `json:"state"`
	}
	if err = c.decodeWorkerResponse("FailJobResult", raw, &result); err != nil {
		return err
	}
	switch result.State {
	case "cancelled", "orphan", "retry-wait", "dead-letter":
		return nil
	default:
		return platform.NewError(platform.CodeIntegrityError, false, "任务失败回执状态不符合生产方迁移")
	}
}
