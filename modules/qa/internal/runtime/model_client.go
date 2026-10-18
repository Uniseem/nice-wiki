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
	input.Request.IDempotencyKey = key
	return modelclient.InvokeModel(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op), runtime.Gateway, op, platform.InvokeModelRequest{Request: input.Request, IDempotencyKey: key, Stream: input.Stream, OnEvent: runtime.OnModelEvent})
}

// ModelText 只读取网关已持久化的完整winning文本；费用状态另行保留。
func ModelText(outcome InvokeModelResult) (string, error) {
	if outcome.State != "completed" || outcome.CurrentSnapshot == nil || outcome.CurrentSnapshot.Output == nil || outcome.CurrentSnapshot.Output.Text == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, false, "模型完整文本尚未持久化，保留原调用待查询")
	}
	return *outcome.CurrentSnapshot.Output.Text, nil
}
