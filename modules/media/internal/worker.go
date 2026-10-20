package media

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/worker"
	"nice-wiki/modules/media/internal/gen"
)

// WorkerLoopInput 的Client只接受受信装配，不从HTTP或任务对象反序列化。
type WorkerLoopInput struct {
	MaxConcurrency int                       `json:"maxConcurrency"`
	Client         platform.WorkerJobsClient `json:"-"`
}
type WorkerLoopResult struct {
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// cleanupWorkerOperation 以真实批准恢复站点/root，领取时新建期限，不沿用批准请求deadline。
func (s *localBlobStore) cleanupWorkerOperation(ctx context.Context, claimed platform.ClaimedJob) (OperationContext, error) {
	approval, found, err := s.loadCleanupApproval(ctx, s.db, claimed.InputRef.ObjectID)
	if err != nil {
		return OperationContext{}, err
	}
	if !found {
		return OperationContext{}, platform.NewError(platform.CodeNotFound, false, "任务人工批准对象不存在")
	}
	a := approval.Approval
	deadline := platform.Timestamp(time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano))
	op := OperationContext{RequestID: platform.NewIdentifier(), OperationID: claimed.OperationID, RootOperationID: claimed.RootOperationID, CallerService: "media", Actor: platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: a.SiteId, ExpiresAt: deadline}, Deadline: deadline, DataEpoch: claimed.DataEpoch, CostClass: platform.CostClassBackground}
	input := platform.JobExecutionInput{JobID: claimed.JobID, JobType: claimed.JobType, InputRef: claimed.InputRef, FencingToken: claimed.FencingToken, Attempt: claimed.Attempt, DataEpoch: claimed.DataEpoch}
	if err := checkCleanupJobBinding(op, approval, input); err != nil {
		return OperationContext{}, err
	}
	if op.DataEpoch != s.openedEpoch {
		return OperationContext{}, platform.NewError(platform.CodeVersionMismatch, false, "任务不属于本域已打开代次")
	}
	return op, nil
}

// RunWorker 只登记media.cleanup；领取、续租、取消及fence检查全部使用平台循环。
func RunWorker(ctx context.Context, op OperationContext, input WorkerLoopInput) (WorkerLoopResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return WorkerLoopResult{}, platform.NewError(platform.CodeForbidden, false, "领取循环仅供本域受信启动")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return WorkerLoopResult{}, err
	}
	if input.Client == nil || s.window.Runtime == nil {
		return WorkerLoopResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务租约客户端或运行权威未装配")
	}
	var completed, failed atomic.Int64
	config := platform.WorkerConfig{Owner: "media", JobTypes: []string{"media.cleanup"}, MaxConcurrency: input.MaxConcurrency, Runtime: s.window.Runtime, OperationContext: s.cleanupWorkerOperation}
	executors := platform.JobExecutorRegistry{"media.cleanup": func(taskCtx context.Context, job platform.JobExecutionInput) (platform.JobExecutionResult, error) {
		taskOp, ok := platform.OperationContextFrom(taskCtx)
		if !ok {
			failed.Add(1)
			return platform.JobExecutionResult{}, platform.NewError(platform.CodeForbidden, false, "任务缺平台操作上下文")
		}
		approval, found, err := s.loadCleanupApproval(taskCtx, s.db, job.InputRef.ObjectID)
		if err == nil && !found {
			err = platform.NewError(platform.CodeNotFound, false, "任务批准对象缺失")
		}
		if err == nil {
			_, err = ExecuteManualCleanup(taskCtx, taskOp, ExecuteManualCleanupInput{PlanId: job.InputRef.ObjectID, ExpectedGeneration: approval.Approval.Plan.ReferenceGeneration, Job: job})
		}
		if err != nil {
			result, failure := s.cleanupWorkerFailure(taskCtx, taskOp, job, err)
			if result.State == platform.JobSucceeded {
				completed.Add(1)
			} else {
				failed.Add(1)
			}
			return result, failure
		}
		completed.Add(1)
		return platform.JobExecutionResult{State: platform.JobSucceeded}, nil
	}}
	err = worker.RunWorkerLoop(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op), input.Client, config, executors)
	return WorkerLoopResult{completed.Load(), failed.Load()}, err
}

// cleanupWorkerFailure 取消后只做独立有界只读证明，不把未知或已决定删除说成安全取消。
func (s *localBlobStore) cleanupWorkerFailure(ctx context.Context, op OperationContext, job platform.JobExecutionInput, executeErr error) (platform.JobExecutionResult, error) {
	cancelled := errors.Is(ctx.Err(), context.Canceled)
	var cause *platform.Error
	if !cancelled && errors.As(context.Cause(ctx), &cause) && cause.Code == platform.CodeCancelled {
		cancelled = true
	}
	if !cancelled {
		return platform.JobExecutionResult{}, executeErr
	}
	// 原任务ctx已取消；此窗口仅证明现有持久事实，不触发RPC或任何写入。
	proofCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(proofCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return platform.JobExecutionResult{}, mapMediaDBError(err, "开始安全取消只读证明失败")
	}
	defer tx.Rollback()
	approval, found, err := s.loadCleanupApproval(proofCtx, tx, job.InputRef.ObjectID)
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	if !found {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeIntegrityError, false, "安全取消证明缺原批准")
	}
	if err := checkCleanupJobBinding(op, approval, job); err != nil {
		return platform.JobExecutionResult{}, err
	}
	if _, err := s.readCleanupPlan(proofCtx, tx, op, job.InputRef.ObjectID); err != nil {
		return platform.JobExecutionResult{}, err
	}
	record, decided, err := s.readCleanupExecution(proofCtx, tx, approval)
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	if err := tx.Rollback(); err != nil {
		return platform.JobExecutionResult{}, mapMediaDBError(err, "结束安全取消只读证明失败")
	}
	if decided && record.Result != nil {
		return platform.JobExecutionResult{State: platform.JobSucceeded}, nil
	}
	if decided {
		return platform.JobExecutionResult{}, executeErr
	}
	return platform.JobExecutionResult{State: platform.JobCancelled, ErrorCode: platform.CodeCancelled}, nil
}

// NewCleanupWorkerClient只装配生产方schema，HTTP字段转换和租约机制统一由平台负责。
func NewCleanupWorkerClient(client *gen.JobsClient) (platform.WorkerJobsClient, error) {
	if client == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "资源任务真实jobs客户端未装配")
	}
	document := []byte(gen.PayloadValidationSpec)
	validators := make(worker.ResponseValidators, 6)
	for _, name := range []string{"ClaimJobResult", "StartJobResult", "HeartbeatJobResult", "CheckLeaseResult", "CompleteJobResult", "FailJobResult"} {
		validate, err := apivalidate.NewPayloadValidator(document, "Jobs"+name)
		if err != nil {
			return nil, err
		}
		validators[name] = validate
	}
	return worker.NewJobsClient(client, validators)
}

// NewMediaBackgroundTasks仅构造闭包，完整应用门槛失败时不会启动任何业务后台。
func NewMediaBackgroundTasks(op OperationContext, jobs *gen.JobsClient) ([]platform.BackgroundTask, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return nil, platform.NewError(platform.CodeForbidden, false, "资源后台装配仅接受本域可信启动")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return nil, err
	}
	if jobs == nil || s.cleanupJobs == nil || s.window.Runtime == nil || s.window.WriteWindow == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "资源后台真实客户端或窗口未装配")
	}
	workerJobs, err := NewCleanupWorkerClient(jobs)
	if err != nil {
		return nil, err
	}
	eventClient, err := newMediaEventConsumer(jobs)
	if err != nil {
		return nil, err
	}
	eventAdmission := newMediaEventAdmission(s.window)
	return []platform.BackgroundTask{
		{Name: "media.ConsumeEvents", Run: func(ctx context.Context) error {
			work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
			_, err := ConsumeEvents(work, op, ConsumeEventsInput{
				Client:        eventClient,
				Runtime:       s.window.Runtime,
				AdmitBatch:    eventAdmission,
				AdmitDelivery: eventAdmission,
				ConsumerID:    "media",
				MaxInFlight:   4,
			})
			return err
		}},
		{Name: "media.RunWorker", Run: func(ctx context.Context) error {
			_, err := RunWorker(ctx, op, WorkerLoopInput{MaxConcurrency: 2, Client: workerJobs})
			return err
		}},
		{Name: "media.RelayOutbox", Run: func(ctx context.Context) error {
			work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
			return lifecycle.RunRecoveryLoop(work, platform.RecoveryLoopConfig{PollMillis: 1000, Runtime: s.window.Runtime, WriteWindow: s.window.WriteWindow, Recover: func(batchCtx context.Context) error {
				batchOp, _ := platform.OperationContextFrom(batchCtx)
				_, err := RelayOutbox(batchCtx, batchOp, RelayOutboxInput{BatchLimit: 100})
				return err
			}})
		}},
	}, nil
}
