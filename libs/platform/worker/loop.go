// Package worker 实现任务领取循环的通用机制。
//
// 各服务只装配本域jobTypes与执行函数；租约、续租、fence校验与
// 写回全部由本循环经jobs接口完成。
package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
)

// 领取循环参数边界与心跳节奏。
const (
	maxJobTypes       = 32
	maxConcurrencyCap = 8
	heartbeatEvery    = 10 * time.Second
	idlePause         = 2 * time.Second
	maintenancePause  = 2 * time.Second
)

// RunWorkerLoop 是任务领取循环的通用实现：
// 只领取本域登记类型；维护期间暂停；jobs:startJob后调用执行函数，
// 每10秒续租、上报进度并读取取消；写回前核对租约，成功completeJob，
// 失败failJob；租约过期立即停止写入，不重放已派发模型调用。
func RunWorkerLoop(ctx context.Context, client platform.WorkerJobsClient, config platform.WorkerConfig, executors platform.JobExecutorRegistry) error {
	_, err := RunWorkerLoopWithResult(ctx, client, config, executors)
	return err
}

// RunWorkerLoopWithResult 仅累计实际jobs回执；不把未确认的在途/取消算成终局。
func RunWorkerLoopWithResult(ctx context.Context, client platform.WorkerJobsClient, config platform.WorkerConfig, executors platform.JobExecutorRegistry) (platform.WorkerRunResult, error) {
	if config.AdmitBusiness == nil {
		config.AdmitBusiness = func(ctx context.Context) (context.Context, func(), error) { return ctx, func() {}, nil }
	}
	if config.BindExecutionScope == nil {
		config.BindExecutionScope = func(ctx context.Context, _ platform.OperationContext, _ platform.ClaimedJob) (context.Context, error) {
			return ctx, nil
		}
	}
	if client == nil || len(config.JobTypes) == 0 || config.Owner == "" || config.MaxConcurrency < 1 || config.MaxConcurrency > maxConcurrencyCap || len(config.JobTypes) > maxJobTypes || config.OperationContext == nil || config.Runtime == nil {
		return platform.WorkerRunResult{}, platform.NewError(platform.CodeValidation, false, "worker配置不合法")
	}
	typed, ok := client.(platform.TypedWorkerJobsClient)
	if !ok {
		return platform.WorkerRunResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "任务客户端不支持领取前类型筛选")
	}
	registered := map[string]bool{}
	for _, jobType := range config.JobTypes {
		if jobType == "" {
			return platform.WorkerRunResult{}, platform.NewError(platform.CodeValidation, false, "jobType不能为空")
		}
		if executors.Executor(jobType) == nil {
			return platform.WorkerRunResult{}, platform.NewError(platform.CodeValidation, false, "jobType缺少登记的执行函数")
		}
		registered[jobType] = true
	}
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	loop := &workerLoop{cancel: cancel, client: client, typed: typed, config: config, registered: registered, executors: executors}
	var wg sync.WaitGroup
	for i := 0; i < config.MaxConcurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			loop.run(work, index)
		}(i)
	}
	wg.Wait()
	return platform.WorkerRunResult{Claimed: loop.claimed.Load(), Completed: loop.completed.Load(), Failed: loop.failed.Load(), Cancelled: loop.cancelled.Load()}, loop.err
}

type workerLoop struct {
	claimed, completed, failed, cancelled atomic.Int64
	cancel                                context.CancelFunc
	errMu                                 sync.Mutex
	err                                   error
	client                                platform.WorkerJobsClient
	typed                                 platform.TypedWorkerJobsClient
	config                                platform.WorkerConfig
	registered                            map[string]bool
	executors                             platform.JobExecutorRegistry
}

func (l *workerLoop) run(ctx context.Context, index int) {
	workerID := platform.NewIdentifier() + "-" + itoa(index)
	for {
		if ctx.Err() != nil {
			return
		}
		view, ok := readRuntime(ctx, l.config.Runtime)
		if !ok || view.Maintenance || view.DataEpoch == "" {
			if !sleep(ctx, maintenancePause) {
				return
			}
			continue
		}
		scope, releaseScope, scopeErr := l.config.AdmitBusiness(ctx)
		if scopeErr != nil {
			if !sleep(ctx, maintenancePause) {
				return
			}
			continue
		}
		// 领取属于owner队列控制；具体本站授权在领取后的域factory建立。
		claimCtx, cancelClaim := context.WithTimeout(platform.WithPortalProof(scope, ""), 30*time.Second)
		deadline, _ := claimCtx.Deadline()
		id := platform.NewIdentifier()
		op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id,
			CallerService: l.config.Owner, DataEpoch: view.DataEpoch,
			Deadline: deadline.UTC().Format(time.RFC3339Nano), CostClass: platform.CostClassBackground,
			Actor: platform.AuthContext{Kind: "service", SubjectID: l.config.Owner, SiteID: "system",
				ExpiresAt: deadline.UTC().Format(time.RFC3339Nano), Grants: []string{}}}
		jobs, err := l.typed.ClaimJobTypes(platform.WithOperationContext(claimCtx, op), l.config.Owner, workerID, 1, l.config.JobTypes)
		cancelClaim()
		if err != nil {
			releaseScope()
			if ctx.Err() != nil {
				return
			}
			if isConfigRejected(err) {
				l.errMu.Lock()
				if l.err == nil {
					l.err = err
				}
				l.errMu.Unlock()
				l.cancel()
				return
			}
			if !sleep(ctx, idlePause) {
				return
			}
			continue
		}
		if len(jobs) == 0 {
			releaseScope()
			if !sleep(ctx, idlePause) {
				return
			}
			continue
		}
		l.claimed.Add(int64(len(jobs)))
		l.execute(scope, workerID, jobs[0])
		releaseScope()
	}
}

// execute 执行单个任务：start→带心跳运行→校验租约→写回。
func (l *workerLoop) execute(ctx context.Context, workerID string, job platform.ClaimedJob) {
	job.WorkerID = workerID
	op, err := l.config.OperationContext(ctx, job)
	if err != nil || op.OperationID != job.OperationID || op.RootOperationID != job.RootOperationID || op.DataEpoch != job.DataEpoch || op.CallerService != l.config.Owner {
		debugSkip(job, "解析任务操作身份", err)
		return
	}
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		debugSkip(job, "解析任务期限", err)
		return
	}
	ctx, err = l.config.BindExecutionScope(ctx, op, job)
	if err != nil {
		debugSkip(job, "绑定执行范围", err)
		return
	}
	ctx, cancelTask := context.WithDeadline(ctx, deadline)
	defer cancelTask()
	ctx = platform.WithOperationContext(ctx, op)
	jobCtx, cancelJob := context.WithCancelCause(ctx)
	defer cancelJob(nil)
	lease, err := newExecutionLease(jobCtx, op, l.client, workerID, job, cancelJob)
	if err != nil {
		debugSkip(job, "建立租约", err)
		return
	}
	defer lease.timer.Stop()
	lease.onRenewed = l.config.OnLeaseRenewed
	lease.onCancellationObserved = l.config.OnCancellationObserved
	startCtx, cancelStart := executionLeaseContext(lease, jobCtx)
	err = l.client.StartJob(workerControlContext(startCtx, op), job.JobID, workerID, job.FencingToken)
	cancelStart()
	if err != nil {
		debugSkip(job, "开始任务", err)
		return
	}
	// 首次执行也取真实续租回执，不能拿本地时钟伪造域内镜像。
	if err := renewExecutionLease(jobCtx, lease, false); err != nil {
		debugSkip(job, "首次续租", err)
		return
	}
	heartbeatDone := make(chan struct{})
	go func() { defer close(heartbeatDone); l.heartbeat(jobCtx, lease) }()
	defer func() { cancelJob(nil); <-heartbeatDone }()
	executor := l.executors.Executor(job.JobType)
	if !l.registered[job.JobType] {
		executor = nil
	}
	var result platform.JobExecutionResult
	if executor == nil {
		result = platform.JobExecutionResult{State: platform.JobFailed, ErrorCode: platform.CodeValidation}
	} else {
		var executeErr error
		result, executeErr = executor(jobCtx, platform.JobExecutionInput{
			JobID:        job.JobID,
			WorkerID:     workerID,
			JobType:      job.JobType,
			InputRef:     job.InputRef,
			FencingToken: job.FencingToken,
			Attempt:      job.Attempt,
			DataEpoch:    job.DataEpoch,
			CheckLease: func(checkCtx context.Context) error {
				return renewExecutionLease(checkCtx, lease, true)
			},
		})
		if executeErr != nil {
			debugSkip(job, "执行函数返回错误", executeErr)
			result.State = platform.JobFailed
			result.ErrorCode = platform.CodeInternal
			var failure *platform.Error
			if errors.As(executeErr, &failure) {
				result.ErrorCode = failure.Code
			}
		}
	}
	// 占用控制通道等待在途心跳完成；最终写回也受本地已知截止限制。
	lease.rpcMu.Lock()
	defer lease.rpcMu.Unlock()
	if ctx.Err() != nil || context.Cause(jobCtx) == errLeaseLost {
		return
	}
	lease.mu.Lock()
	valid := lease.expires.After(time.Now())
	lease.mu.Unlock()
	if !valid {
		return
	}
	// 取消信号只要求owner停止；只有owner明确返回安全取消才能确认。
	// 已持久成功与取消竞争仍保留成功及付费结果。
	if result.State == platform.JobCancelled {
		result.ErrorCode = platform.CodeCancelled
	} else if result.ErrorCode == platform.CodeCancelled {
		result.ErrorCode = platform.CodeInternal
	}
	finalCtx, cancelFinal := executionLeaseContext(lease, ctx)
	defer cancelFinal()
	valid, err = l.client.CheckLease(workerControlContext(finalCtx, op), job.JobID, workerID, job.FencingToken)
	if err != nil || !valid || finalCtx.Err() != nil {
		return
	}
	switch result.State {
	case platform.JobSucceeded:
		if l.client.CompleteJob(workerControlContext(finalCtx, op), job.JobID, workerID, job.FencingToken, result.ResultRef, result.PaidResultRefs) == nil {
			l.completed.Add(1)
		}
	default:
		errorCode := result.ErrorCode
		if errorCode == "" {
			errorCode = platform.CodeInternal
		}
		if l.client.FailJob(workerControlContext(finalCtx, op), job.JobID, workerID, job.FencingToken, errorCode, result.PaidResultRefs) == nil {
			if errorCode == platform.CodeCancelled {
				l.cancelled.Add(1)
			} else {
				l.failed.Add(1)
			}
		}
	}
}

// errLeaseLost 表示续租失败，任务不得再写回。
var errLeaseLost = errors.New("job lease lost")

var errJobCancelled = errors.New("job cancellation requested")

// executionLease以独立定时器限制本地执行权；续租RPC串行，避免旧回执覆盖新截止。
type executionLease struct {
	onCancellationObserved func(context.Context, platform.OperationContext, platform.ClaimedJob) error
	onRenewed              func(context.Context, platform.OperationContext, platform.ClaimedJob, platform.HeartbeatInfo) error
	ctx                    context.Context
	op                     platform.OperationContext
	client                 platform.WorkerJobsClient
	workerID               string
	job                    platform.ClaimedJob
	cancel                 context.CancelCauseFunc
	rpcMu                  sync.Mutex
	mu                     sync.Mutex
	expires                time.Time
	timer                  *time.Timer
}

func newExecutionLease(ctx context.Context, op platform.OperationContext, client platform.WorkerJobsClient, workerID string, job platform.ClaimedJob, cancel context.CancelCauseFunc) (*executionLease, error) {
	expires, err := platform.ParseTimestamp(job.ExpiresAt)
	if err != nil || !expires.After(time.Now()) {
		return nil, platform.NewError(platform.CodeLocked, false, "任务领取租约已失效")
	}
	lease := &executionLease{ctx: ctx, op: op, client: client, workerID: workerID, job: job, cancel: cancel, expires: expires}
	lease.timer = time.AfterFunc(time.Until(expires), func() { expireExecutionLease(lease) })
	return lease, nil
}

func expireExecutionLease(lease *executionLease) {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if !lease.expires.After(time.Now()) {
		lease.cancel(errLeaseLost)
	}
}

func executionLeaseContext(lease *executionLease, ctx context.Context) (context.Context, context.CancelFunc) {
	lease.mu.Lock()
	expires := lease.expires
	lease.mu.Unlock()
	return context.WithDeadline(ctx, expires)
}

// renew在租约截止内读取权威取消并续租；不可逆决定额外核当前worker/fence。
func renewExecutionLease(ctx context.Context, lease *executionLease, verify bool) error {
	lease.rpcMu.Lock()
	defer lease.rpcMu.Unlock()
	if lease.ctx.Err() != nil {
		return platform.NewError(platform.CodeCancelled, false, "任务执行已停止")
	}
	work, cancel := executionLeaseContext(lease, ctx)
	info, err := lease.client.HeartbeatJob(workerControlContext(work, lease.op), lease.job.JobID, lease.workerID, lease.job.FencingToken)
	cancel()
	if err != nil {
		lease.cancel(errLeaseLost)
		return err
	}
	if info.CancelRequested {
		if lease.onCancellationObserved != nil {
			work, cancel = executionLeaseContext(lease, ctx)
			callbackOp := lease.op
			deadline, _ := work.Deadline()
			callbackOp.Deadline = deadline.UTC().Format(time.RFC3339Nano)
			err = lease.onCancellationObserved(platform.WithOperationContext(platform.WithPortalProof(work, ""), callbackOp), callbackOp, lease.job)
			cancel()
			if err != nil {
				lease.cancel(errJobCancelled)
				return err
			}
		}
		lease.cancel(errJobCancelled)
		return platform.NewError(platform.CodeCancelled, false, "任务已请求取消")
	}
	if info.SafePointRequested {
		lease.cancel(errLeaseLost)
		return platform.NewError(platform.CodeLocked, false, "任务已请求维护安全点")
	}
	expires, err := platform.ParseTimestamp(info.ExpiresAt)
	lease.mu.Lock()
	now := time.Now()
	valid := err == nil && lease.ctx.Err() == nil && lease.expires.After(now) && expires.After(now)
	if valid {
		lease.expires = expires
		lease.timer.Reset(time.Until(expires))
	} else {
		lease.cancel(errLeaseLost)
	}
	lease.mu.Unlock()
	if !valid {
		return platform.NewError(platform.CodeLocked, false, "任务续租回执已过期或不可用")
	}
	if verify {
		work, cancel = executionLeaseContext(lease, ctx)
		valid, err = lease.client.CheckLease(workerControlContext(work, lease.op), lease.job.JobID, lease.workerID, lease.job.FencingToken)
		cancel()
		if err != nil || !valid {
			lease.cancel(errLeaseLost)
			if err != nil {
				return err
			}
			return platform.NewError(platform.CodeLocked, false, "任务执行租约已失效")
		}
	}
	if lease.ctx.Err() != nil {
		return platform.NewError(platform.CodeCancelled, false, "任务执行已停止")
	}
	if lease.onRenewed != nil {
		work, cancel = executionLeaseContext(lease, ctx)
		callbackOp := lease.op
		deadline, _ := work.Deadline()
		callbackOp.Deadline = deadline.UTC().Format(time.RFC3339Nano)
		err = lease.onRenewed(platform.WithOperationContext(platform.WithPortalProof(work, ""), callbackOp), callbackOp, lease.job, info)
		cancel()
		if err != nil {
			lease.cancel(errLeaseLost)
			return err
		}
	}
	return nil
}

// heartbeat每10秒读取取消并续租；RPC与本地执行均受已知租约截止限制。
func (l *workerLoop) heartbeat(jobCtx context.Context, lease *executionLease) {
	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-jobCtx.Done():
			return
		case <-ticker.C:
			if renewExecutionLease(jobCtx, lease, false) != nil {
				return
			}
		}
	}
}

func readRuntime(ctx context.Context, provider func(context.Context) (platform.RuntimeView, error)) (platform.RuntimeView, bool) {
	if provider == nil {
		return platform.RuntimeView{}, false
	}
	view, err := provider(ctx)
	if err != nil {
		return platform.RuntimeView{}, false
	}
	return view, true
}

// isConfigRejected 配置类永久拒绝直接退出循环，由装配侧修正。
func isConfigRejected(err error) bool {
	var libErr *platform.Error
	if errors.As(err, &libErr) {
		return libErr.Code == platform.CodeValidation || libErr.Code == platform.CodeForbidden || libErr.Code == platform.CodeNotFound
	}
	return false
}

// sleep 等待d或ctx取消；返回false表示已取消。
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

// workerControlContext分配独立控制操作并撤去用户证明，原任务站点与费用根保持。
func workerControlContext(ctx context.Context, op platform.OperationContext) context.Context {
	op.OperationID = platform.NewIdentifier()
	return platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
}

// debugSkip 在调试模式下说明任务为什么没有继续执行（默认静默，由租约到期后重新领取）。
func debugSkip(job platform.ClaimedJob, stage string, err error) {
	if os.Getenv("NW_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "任务停止 type=%s job=%s 阶段=%s err=%v\n", job.JobType, job.JobID, stage, err)
	}
}
