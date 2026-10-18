package analytics

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/worker"
)

type WorkerLoopInput struct{MaxConcurrency int `json:"maxConcurrency"`}
type WorkerLoopResult struct{Completed int64 `json:"completed"`;Failed int64 `json:"failed"`}

// RunWorker仅装配本域执行器与真实租约镜像，领取/续租/完成仍由公共平台处理。
func RunWorker(ctx context.Context,op OperationContext,input WorkerLoopInput)(WorkerLoopResult,error){
 rt,err:=runtimeFrom(ctx,op);if err!=nil{return WorkerLoopResult{},err};if rt.Jobs==nil{return WorkerLoopResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"任务控制客户端未装配")}
 execute:=func(ctx context.Context,in platform.JobExecutionInput)(platform.JobExecutionResult,error){return executePageAggregate(ctx,rt,in)}
 unavailable:=func(context.Context,platform.JobExecutionInput)(platform.JobExecutionResult,error){return platform.JobExecutionResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"费用聚合与对账权威协议尚未完整装配")}
 counts,err:=worker.RunWorkerLoopWithResult(ctx,rt.Jobs,platform.WorkerConfig{Owner:"analytics",JobTypes:[]string{"analytics.aggregate-hourly","analytics.aggregate-daily","analytics.aggregate-usage","analytics.reconcile-usage"},MaxConcurrency:input.MaxConcurrency,Runtime:rt.Window.Runtime,OperationContext:func(ctx context.Context,job platform.ClaimedJob)(OperationContext,error){return aggregateJobOperation(ctx,rt,job)},OnLeaseRenewed:recordAggregateLease,OnCancellationObserved:cancelAggregateLease},platform.JobExecutorRegistry{"analytics.aggregate-hourly":execute,"analytics.aggregate-daily":execute,"analytics.aggregate-usage":unavailable,"analytics.reconcile-usage":unavailable})
 return WorkerLoopResult{Completed:counts.Completed,Failed:counts.Failed+counts.Cancelled},err
}
