package runtime

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/worker"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	"sync"
)

type RunWorkerInput struct {
	WorkerID              string `json:"workerId"`
	MaximumConcurrentJobs int    `json:"maximumConcurrentJobs"`
}
type RunWorkerResult = platform.WorkerRunResult
type WorkerRuntime struct {
	Jobs      platform.WorkerJobsClient
	Executors platform.JobExecutorRegistry
	Runtime   func(context.Context) (platform.RuntimeView, error)
}
type workerRuntimeKey struct{}

// NewWorkerRuntime 只装配真实生成客户端和严格生产schema，不模拟jobs接口。
func NewWorkerRuntime(raw *gen.JobsClient, executors platform.JobExecutorRegistry, runtime func(context.Context) (platform.RuntimeView, error)) (WorkerRuntime, error) {
	validators := worker.ResponseValidators{}
	for _, name := range []string{"ClaimJobResult", "StartJobResult", "HeartbeatJobResult", "CheckLeaseResult", "CompleteJobResult", "FailJobResult"} {
		validate, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "Jobs"+name)
		if err != nil {
			return WorkerRuntime{}, err
		}
		validators[name] = validate
	}
	client, err := worker.NewJobsClient(raw, validators)
	if err != nil {
		return WorkerRuntime{}, err
	}
	return WorkerRuntime{Jobs: client, Executors: executors, Runtime: runtime}, nil
}
func WithWorkerRuntime(ctx context.Context, runtime WorkerRuntime) context.Context {
	return context.WithValue(ctx, workerRuntimeKey{}, runtime)
}

// RunWorker 原workerId仅兼容调用标签，真正租约worker由平台生成并以jobs回执验证。
func RunWorker(ctx context.Context, op OperationContext, input RunWorkerInput) (RunWorkerResult, error) {
	if op.CallerService != "knowledge" || op.Actor.Kind != "service" {
		return RunWorkerResult{}, platform.NewError(platform.CodeForbidden, false, "仅本域后台能运行知识任务")
	}
	if input.MaximumConcurrentJobs != 4 {
		return RunWorkerResult{}, platform.NewError(platform.CodeValidation, false, "知识worker固定一个构建槽和三个独立提取槽")
	}
	runtime, ok := ctx.Value(workerRuntimeKey{}).(WorkerRuntime)
	if !ok || runtime.Jobs == nil || runtime.Runtime == nil {
		return RunWorkerResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "真实任务worker尚未装配")
	}
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	type outcome struct {
		value platform.WorkerRunResult
		err   error
	}
	results := make(chan outcome, 2)
	var waiting sync.WaitGroup
	// 两个真实平台循环经类型筛选领取；孩子保留三个独立槽位，不会被等待中的build占满。
	for _, pool := range []struct {
		kind        string
		concurrency int
	}{{"knowledge.build", 1}, {"knowledge.extract", 3}} {
		waiting.Add(1)
		go func(kind string, concurrency int) {
			defer waiting.Done()
			value, err := worker.RunWorkerLoopWithResult(work, runtime.Jobs, platform.WorkerConfig{Owner: "knowledge", JobTypes: []string{kind}, MaxConcurrency: concurrency, Runtime: runtime.Runtime, OperationContext: indexer.ResolveClaimedJobOperation, OnLeaseRenewed: indexer.MirrorExecutionLease, OnCancellationObserved: indexer.RecordExecutionCancellation}, runtime.Executors)
			if err != nil {
				cancel()
			}
			results <- outcome{value, err}
		}(pool.kind, pool.concurrency)
	}
	waiting.Wait()
	close(results)
	total := RunWorkerResult{}
	var first error
	for result := range results {
		total.Claimed += result.value.Claimed
		total.Completed += result.value.Completed
		total.Failed += result.value.Failed
		total.Cancelled += result.value.Cancelled
		if first == nil {
			first = result.err
		}
	}
	return total, first
}
