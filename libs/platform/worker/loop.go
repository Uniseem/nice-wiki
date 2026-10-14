// Package worker 实现任务领取循环的通用机制。
//
// 各服务只装配本域jobTypes与执行函数；租约、续租、fence校验与
// 写回全部由本循环经jobs接口完成。
package worker

import (
	"context"
	"errors"
	"strconv"
	"sync"
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
func RunWorkerLoop(ctx context.Context, client platform.JobsClient, config platform.WorkerConfig, executors platform.JobExecutorRegistry) error {
	if config.Owner == "" || config.MaxConcurrency < 1 || config.MaxConcurrency > maxConcurrencyCap || len(config.JobTypes) > maxJobTypes {
		return platform.NewError(platform.CodeValidation, false, "worker配置不合法")
	}
	registered := map[string]bool{}
	for _, jobType := range config.JobTypes {
		if jobType == "" {
			return platform.NewError(platform.CodeValidation, false, "jobType不能为空")
		}
		if executors.Executor(jobType) == nil {
			return platform.NewError(platform.CodeValidation, false, "jobType缺少登记的执行函数")
		}
		registered[jobType] = true
	}
	loop := &workerLoop{client: client, config: config, registered: registered, executors: executors}
	var wg sync.WaitGroup
	for i := 0; i < config.MaxConcurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			loop.run(ctx, index)
		}(i)
	}
	wg.Wait()
	return nil
}

type workerLoop struct {
	client     platform.JobsClient
	config     platform.WorkerConfig
	registered map[string]bool
	executors  platform.JobExecutorRegistry
}

func (l *workerLoop) run(ctx context.Context, index int) {
	workerID := platform.NewIdentifier() + "-" + itoa(index)
	for {
		if ctx.Err() != nil {
			return
		}
		if view, ok := readRuntime(ctx, l.config.Runtime); ok && view.Maintenance {
			if !sleep(ctx, maintenancePause) {
				return
			}
			continue
		}
		jobs, err := l.client.ClaimJob(ctx, l.config.Owner, workerID, 1)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if isConfigRejected(err) {
				return
			}
			if !sleep(ctx, idlePause) {
				return
			}
			continue
		}
		if len(jobs) == 0 {
			if !sleep(ctx, idlePause) {
				return
			}
			continue
		}
		l.execute(ctx, workerID, jobs[0])
	}
}

// execute 执行单个任务：start→带心跳运行→校验租约→写回。
func (l *workerLoop) execute(ctx context.Context, workerID string, job platform.ClaimedJob) {
	if err := l.client.StartJob(ctx, job.JobID, workerID, job.FencingToken); err != nil {
		// 租约可能已被回收；等待jobs重投。
		return
	}
	jobCtx, cancelJob := context.WithCancelCause(ctx)
	defer cancelJob(nil)
	go l.heartbeat(jobCtx, workerID, job, cancelJob)
	executor := l.executors.Executor(job.JobType)
	var result platform.JobExecutionResult
	if executor == nil {
		result = platform.JobExecutionResult{State: platform.JobFailed, ErrorCode: platform.CodeValidation}
	} else {
		result, _ = executor(jobCtx, platform.JobExecutionInput{
			JobID:        job.JobID,
			JobType:      job.JobType,
			InputRef:     job.InputRef.ObjectID,
			FencingToken: job.FencingToken,
			Attempt:      job.Attempt,
			DataEpoch:    job.DataEpoch,
		})
	}
	cancelJob(nil)
	if ctx.Err() != nil {
		// 进程正在退出；未写回的任务由jobs按租约到期重投。
		return
	}
	if cause := context.Cause(jobCtx); cause == errLeaseLost {
		// 租约已过期：立即停止写入，不重放已派发的模型调用。
		return
	}
	valid, err := l.client.CheckLease(ctx, job.JobID, job.FencingToken)
	if err != nil || !valid {
		return
	}
	switch result.State {
	case platform.JobSucceeded:
		_ = l.client.CompleteJob(ctx, job.JobID, job.FencingToken, platform.ObjectRef{}, nil)
	default:
		errorCode := result.ErrorCode
		if errorCode == "" {
			errorCode = platform.CodeInternal
		}
		_ = l.client.FailJob(ctx, job.JobID, job.FencingToken, errorCode)
	}
}

// errLeaseLost 表示续租失败或jobs要求取消，任务不得再写回。
var errLeaseLost = errors.New("job lease lost or cancel requested")

// heartbeat 每10秒续租；jobs要求取消或租约失效时终止任务。
func (l *workerLoop) heartbeat(jobCtx context.Context, workerID string, job platform.ClaimedJob, cancelJob context.CancelCauseFunc) {
	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-jobCtx.Done():
			return
		case <-ticker.C:
			info, err := l.client.HeartbeatJob(jobCtx, job.JobID, workerID, job.FencingToken)
			if err != nil || info.CancelRequested {
				cancelJob(errLeaseLost)
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
