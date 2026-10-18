// Package runtime 装配索引域的公共运行机制，不直接访问模型供应方。
package runtime

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/modelclient"
	"nice-wiki/modules/knowledge/internal/gen"
)

type OperationContext = platform.OperationContext
type InvokeModelInput struct {
	Request platform.CreateInvocationInput `json:"request"`
	Stream  bool                           `json:"stream"`
}
type InvokeModelResult = platform.InvokeModelOutcome

type ModelRuntime struct {
	Gateway         platform.GatewayClient
	AdmissionPermit *platform.AdmissionPermit
	OnEvent         func(platform.StreamEvent) error
}
type modelRuntimeKey struct{}

// NewModelRuntime 仅从生产方 schema 构造严格公共网关适配。
func NewModelRuntime(raw *gen.ModelGatewayClient) (ModelRuntime, error) {
	validators := modelclient.GatewayResponseValidators{}
	for _, name := range []string{"CreateInvocationResult", "CancelInvocationResult", "StreamEvent"} {
		validator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if err != nil {
			return ModelRuntime{}, err
		}
		validators[name] = validator
	}
	gateway, err := modelclient.NewGatewayClient(raw, validators)
	if err != nil {
		return ModelRuntime{}, err
	}
	return ModelRuntime{Gateway: gateway}, nil
}
func WithModelRuntime(ctx context.Context, runtime ModelRuntime) context.Context {
	return context.WithValue(ctx, modelRuntimeKey{}, runtime)
}

// InvokeModel 将幂等、续读、取消交给平台；本域只绑定步骤及已消费的许可。
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error) {
	runtime, ok := ctx.Value(modelRuntimeKey{}).(ModelRuntime)
	if !ok || runtime.Gateway == nil {
		return InvokeModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型网关执行链未装配")
	}
	if op.CostClass == platform.CostClassQA {
		input.Request.AdmissionPermit = runtime.AdmissionPermit
	}
	key := "knowledge." + platform.HashBytes([]byte(op.OperationID+":"+input.Request.IDempotencyKey))
	return modelclient.InvokeModel(platform.WithOperationContext(ctx, op), runtime.Gateway, op, platform.InvokeModelRequest{Request: input.Request, IDempotencyKey: key, Stream: input.Stream, OnEvent: runtime.OnEvent})
}

// CompletedOutput 只读取终态结构化结果，收费未知、失败和缺输出均不重算。
func CompletedOutput(outcome InvokeModelResult, capability string) (*platform.ModelOutput, error) {
	if outcome.State != "completed" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "模型结果未成功完成，保留原调用待查")
	}
	var output *platform.ModelOutput
	for _, event := range outcome.Events {
		if event.Output != nil {
			if output != nil || event.Event != platform.StreamCompleted || event.Output.Capability != capability {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "模型结构结果不唯一或能力不匹配")
			}
			output = event.Output
		}
	}
	if output == nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "模型缺少结构化完成结果")
	}
	return output, nil
}

// BindAdmissionPermit 复制请求独立的运行上下文；网关仍负责权威验签。
func BindAdmissionPermit(ctx context.Context, op OperationContext, permit *gen.AdmissionPermit) (context.Context, error) {
	if op.CostClass != platform.CostClassQA {
		return ctx, nil
	}
	runtime, ok := ctx.Value(modelRuntimeKey{}).(ModelRuntime)
	if !ok {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "模型执行链未装配")
	}
	runtime.AdmissionPermit = nil
	if op.CostClass == platform.CostClassQA {
		if permit == nil || permit.Token == "" || permit.RequestId != op.RootOperationID || permit.DataEpoch != op.DataEpoch || (op.Actor.Kind == "visitor" && permit.SubjectId != op.Actor.SubjectID) || (op.Actor.Kind == "service" && op.CallerService != "qa") || (permit.Action != "qa.ask" && permit.Action != "qa.followup") || string(permit.Audience) != "model-gateway" {
			return nil, platform.NewError(platform.CodeForbidden, false, "知识模型调用缺少本轮已消费准入许可")
		}
		runtime.AdmissionPermit = &platform.AdmissionPermit{AdmissionID: permit.AdmissionId, RequestID: permit.RequestId, SubjectID: permit.SubjectId, Action: permit.Action, Token: permit.Token, ExpiresAt: platform.Timestamp(permit.ExpiresAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")), SessionBudgetScope: permit.SessionBudgetScope, NetworkBudgetScope: permit.NetworkBudgetScope, DataEpoch: permit.DataEpoch, Audience: string(permit.Audience)}
	}
	return WithModelRuntime(ctx, runtime), nil
}
