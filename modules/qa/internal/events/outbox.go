package events

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
)

type OperationContext = platform.OperationContext
type PublishOutboxInput struct {
	Limit    int    `json:"limit"`
	WorkerID string `json:"workerId"`
}
type PublishOutboxResult struct {
	Published int `json:"published"`
	Pending   int `json:"pending"`
}

// PublishOutbox 只装配本域已提交事件，转发、租约和有限重试完全交平台。
func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error) {
	runtime, err := qaruntime.FromContext(ctx)
	if err != nil {
		return PublishOutboxResult{}, err
	}
	if runtime.Jobs == nil {
		return PublishOutboxResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "任务事件接收端未装配")
	}
	result, err := outbox.RunRelayBatch(qaruntime.ServiceReadContext(ctx, op), outbox.NewStore(runtime.DB), outbox.NewEventPublisher(runtime.Jobs), platform.RelayConfig{BatchLimit: input.Limit, PollMillis: 1000, Runtime: runtime.WriteWindow.Runtime, WriteWindow: runtime.WriteWindow.WriteWindow})
	if err != nil {
		return PublishOutboxResult{}, err
	}
	return PublishOutboxResult{Published: int(result.Relayed), Pending: int(result.Pending)}, nil
}
