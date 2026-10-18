package runtime

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelclient"
)

type InvokeModelInput = platform.InvokeModelRequest
type InvokeModelResult = platform.InvokeModelOutcome

// InvokeModel 只装配一次网关调用，取消、续读与费用语义由平台统一实现。
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error) {
	s := CurrentStore()
	if s == nil || s.gateway == nil {
		return InvokeModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "记忆模型客户端未装配")
	}
	return modelclient.InvokeModel(ctx, s.gateway, op, input)
}

// ConfigureGateway 注入平台真实模型客户端，不保存供应方凭据。
func ConfigureGateway(client platform.GatewayClient) error {
	s := CurrentStore()
	if s == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "记忆存储未打开")
	}
	s.gateway = client
	return nil
}
