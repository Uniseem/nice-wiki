package analytics

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
)

// PublishAggregateEvents只装配本域批次，通用领取/重试/隔离由平台处理。
func PublishAggregateEvents(ctx context.Context, op OperationContext, in PublishAggregateEventsInput) (PublishAggregateEventsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return PublishAggregateEventsResult{}, err
	}
	if rt.Publisher == nil {
		return PublishAggregateEventsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "事件转发端口未装配")
	}
	limit := int(in.BatchSize)
	if limit < 1 || limit > 100 {
		limit = 100
	}
	result, err := outbox.RunRelayBatch(platform.WithOperationContext(ctx, op), outbox.NewStore(rt.DB), rt.Publisher, platform.RelayConfig{BatchLimit: limit, PollMillis: 1000, Runtime: rt.Window.Runtime, WriteWindow: rt.Window.WriteWindow})
	return PublishAggregateEventsResult{Sent: result.Relayed}, err
}
