package invocation

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/model-gateway/internal/gen"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)
type OperationContext=platform.OperationContext
type GetInvocationInput struct{InvocationID string `json:"invocationId"`}
type GetInvocationResult struct{Invocation gen.Invocation `json:"invocation"`;Output *platform.ModelOutput `json:"output,omitempty"`}
func GetInvocation(ctx context.Context,op OperationContext,input GetInvocationInput)(GetInvocationResult,error){tx,err:=gatewayruntime.BeginExecution(ctx,op,false);if err!=nil{return GetInvocationResult{},err};defer tx.Close();record,err:=gatewayruntime.ReadInvocationRecord(tx,op,input.InvocationID);if err!=nil{return GetInvocationResult{},err};result:=GetInvocationResult{Invocation:record.Invocation};if record.Output!=nil&&record.Invocation.ResultAttemptId!=nil{evidence,err:=ReadAttemptEvidence(tx,*record.Invocation.ResultAttemptId);if err!=nil{if ErrorCode(err)==platform.CodeNotFound{return GetInvocationResult{},platform.NewError(platform.CodeIntegrityError,false,"原结果winning证据缺失")};return GetInvocationResult{},err};if !evidence.Completed||evidence.Code!=""||evidence.Output==nil{return GetInvocationResult{},platform.NewError(platform.CodeIntegrityError,false,"原结构输出没有完整有效证据")};stored,_:=platform.CanonicalJSON(record.Output);original,_:=platform.CanonicalJSON(evidence.Output);if platform.HashBytes(stored)!=platform.HashBytes(original){return GetInvocationResult{},platform.NewError(platform.CodeIntegrityError,false,"原结构输出与winning证据不一致")};result.Output=record.Output};return result,nil}
