package events

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type PublishOutboxInput struct {
	Limit    int    `json:"limit"`
	WorkerID string `json:"workerId"`
}
type PublishOutboxResult struct {
	Published int64 `json:"published"`
	Pending   int64 `json:"pending"`
}

// PublishOutbox 复用平台单批relay，pending只代表本批未确认数，不假报全域积压。
func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error) {
	if op.CallerService != "knowledge" || op.Actor.Kind != "service" {
		return PublishOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "仅本域后台可转发索引事件")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return PublishOutboxResult{}, err
	}
	if runtime.Jobs == nil || runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return PublishOutboxResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "索引事件转发依赖未装配")
	}
	config := platform.RelayConfig{BatchLimit: input.Limit, PollMillis: 1000, WriteWindow: runtime.WriteWindow.WriteWindow, Runtime: func(work context.Context) (platform.RuntimeView, error) {
		view, err := runtime.WriteWindow.Runtime(work)
		if err != nil {
			return platform.RuntimeView{}, err
		}
		if view.DataEpoch != op.DataEpoch {
			return platform.RuntimeView{}, platform.NewError(platform.CodeStaleResult, false, "索引事件打开代次已改变")
		}
		return view, nil
	}}
	result, err := outbox.RunRelayBatch(platform.WithOperationContext(ctx, op), outbox.NewStore(runtime.DB), outbox.NewEventPublisher(runtime.Jobs), config)
	if err != nil {
		return PublishOutboxResult{}, err
	}
	return PublishOutboxResult{Published: result.Relayed, Pending: result.Pending}, nil
}
