package invocation

import (
 "context"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/model-gateway/internal/gen"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)
type OperationContext=platform.OperationContext
type GetInvocationInput struct{InvocationID string `json:"invocationId"`}
type GetInvocationResult struct{Invocation gen.Invocation `json:"invocation"`}
func GetInvocation(ctx context.Context,op OperationContext,input GetInvocationInput)(GetInvocationResult,error){tx,err:=gatewayruntime.BeginExecution(ctx,op,false);if err!=nil{return GetInvocationResult{},err};defer tx.Close();record,err:=gatewayruntime.ReadInvocationRecord(tx,op,input.InvocationID);if err!=nil{return GetInvocationResult{},err};return GetInvocationResult{Invocation:record.Invocation},nil}
