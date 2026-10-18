package runtime

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelclient"
	"strings"
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
	return modelclient.InvokeModel(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op), runtime.Gateway, op, platform.InvokeModelRequest{Request: input.Request, IDempotencyKey: key, Stream: input.Stream, OnEvent: runtime.OnModelEvent})
}

// ModelText 读取成功持久调用的文本增量，不把费用未知或失败当作空答案。
func ModelText(outcome InvokeModelResult) (string, error) {
	if outcome.State != "completed" {
		return "", platform.NewError(platform.CodeDependencyUnavailable, false, "模型未成功完成，保留原调用待查询")
	}
	var text strings.Builder
	for _, event := range outcome.Events {
		if event.Event == platform.StreamDelta {
			if text.Len()+len(event.Delta) > 65536 {
				return "", platform.NewError(platform.CodeIntegrityError, false, "模型文本输出超过上限")
			}
			text.WriteString(event.Delta)
		}
	}
	return text.String(), nil
}
