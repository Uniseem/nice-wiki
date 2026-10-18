package invocation

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/model-gateway/internal/gen"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)
type CancelInvocationInput=gen.CancelInvocationInput
type CancelInvocationResult=gen.CancelInvocationResult
func CancelInvocation(ctx context.Context,op OperationContext,input CancelInvocationInput)(CancelInvocationResult,error){
 tx,err:=gatewayruntime.BeginFinalization(ctx,op);if err!=nil{return CancelInvocationResult{},err};var result CancelInvocationResult
 replay,err:=gatewayruntime.ReplayExecutionReceipt(tx,op,"model-gateway.CancelInvocation",input.IdempotencyKey,input,&result);if err!=nil{tx.Close();return result,err};record,err:=gatewayruntime.ReadInvocationRecord(tx,op,input.InvocationId);if err!=nil{tx.Close();return result,err}
 if !replay{if record.Invocation.State!="succeeded"&&record.Invocation.State!="cancelled"&&record.Invocation.State!="failed"&&record.Invocation.State!="reconciled"{record.CancelRequested=true;record.Invocation.State=gen.InvocationState("cancel_requested");if err:=gatewayruntime.SaveInvocationRecord(tx,record);err!=nil{tx.Close();return result,err}};result.Invocation=record.Invocation;if err:=gatewayruntime.SaveExecutionReceipt(tx,op,"model-gateway.CancelInvocation",input.IdempotencyKey,input,result);err!=nil{tx.Close();return result,err}}
 if err:=tx.Commit();err!=nil{tx.Close();return result,err};tx.Close();options,err:=gatewayruntime.ExecutionDependencies();if err!=nil{return result,err};if options.Tasks!=nil{options.Tasks.Cancel(input.InvocationId)}
 if record.CancelRequested{if _,err:=FinishCancelledInvocation(ctx,op,record);err!=nil{return result,err}}
 current,err:=ReadExecutionRecord(ctx,op,input.InvocationId);if err!=nil{return result,err};return CancelInvocationResult{Invocation:current.Invocation},nil
}
