package runtime

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/modelclient"
	"nice-wiki/libs/platform/outbox"
	"time"
)

type InvokeModelInput = platform.InvokeModelRequest
type InvokeModelResult = platform.InvokeModelOutcome

// ConfigureExecutionAdapters 仅注入平台适配器，构造期间不探测或调用模型。
func ConfigureExecutionAdapters(gateway platform.GatewayClient, publisher platform.EventPublisher) error {
	store, _ := LanguageGateway()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本地化存储未装配")
	}
	store.gateway = gateway
	store.publisher = publisher
	return nil
}
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error) {
	store, _ := LanguageGateway()
	if store == nil || store.gateway == nil {
		return InvokeModelResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化模型适配器未装配")
	}
	return modelclient.InvokeModel(ctx, store.gateway, op, input)
}

// PublishBatch 使用平台单批outbox；租约、旧代丢弃和失败退避只由平台实现。
func PublishBatch(ctx context.Context, limit int) (platform.RelayBatchResult, error) {
	store, runtimeClient := LanguageGateway()
	if store == nil || runtimeClient == nil || store.publisher == nil {
		return platform.RelayBatchResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本地化事件发布依赖未装配")
	}
	return outbox.RunRelayBatch(ctx, outbox.NewStore(store.db), store.publisher, platform.RelayConfig{BatchLimit: limit, PollMillis: 1000, Runtime: runtimeClient.ReadRuntimeState, WriteWindow: func(work context.Context) (time.Time, error) {
		var state string
		if err := store.db.QueryRowContext(work, `SELECT state FROM local_barrier WHERE singleton=1`).Scan(&state); err != nil {
			return time.Time{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "本地化屏障不可读取")
		}
		if state != "released" {
			return time.Time{}, platform.NewError(platform.CodeLocked, true, "本地化屏障暂停事件转发")
		}
		return time.Time{}, nil
	}})
}
