package runtime

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelclient"
)

type InvokeModelInput struct {
	Request platform.CreateInvocationInput `json:"request"`
	Stream  bool                           `json:"stream"`
}
type InvokeModelResult = platform.InvokeModelOutcome

// InvokeModel 只装配本域步骤与原准入许可，付费/续读/取消交平台。
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error) {
	runtime, err := FromContext(ctx)
	if err != nil {
		return InvokeModelResult{}, err
	}
	if runtime.Gateway == nil {
		return InvokeModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "问答模型网关尚未装配")
	}
	if op.CostClass == platform.CostClassQA {
		if runtime.AdmissionPermit == nil {
			return InvokeModelResult{}, platform.NewError(platform.CodeForbidden, false, "问答模型调用缺少已消费许可")
		}
		input.Request.AdmissionPermit = runtime.AdmissionPermit
	}
	key := "qa." + platform.HashBytes([]byte(op.OperationID+":"+input.Request.IDempotencyKey))
	return modelclient.InvokeModel(platform.WithOperationContext(ctx, op), runtime.Gateway, op, platform.InvokeModelRequest{Request: input.Request, IDempotencyKey: key, Stream: input.Stream, OnEvent: runtime.OnModelEvent})
}
