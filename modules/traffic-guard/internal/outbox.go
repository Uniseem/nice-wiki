package trafficguard

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
)

type PublishGuardEventsInput struct {
	BatchSize int
	NowMs     int64
}
type PublishGuardEventsResult struct{ PublishedCount int64 }

// PublishGuardEvents 只装配本域参数；领取、重试、确认、代次隔离由公共relay拥有。
func PublishGuardEvents(ctx context.Context, op OperationContext, in PublishGuardEventsInput) (PublishGuardEventsResult, error) {
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok || runtime.Store == nil || runtime.Jobs == nil || runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return PublishGuardEventsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "风控事件转发依赖未装配")
	}
	if op.CallerService != "traffic-guard" {
		return PublishGuardEventsResult{}, platform.NewError(platform.CodeForbidden, false, "仅本域后台可转发风控事件")
	}
	store := runtime.Store
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.db == nil || !store.JournalRecovered {
		return PublishGuardEventsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "风控事件库尚未恢复")
	}
	if store.openedDataEpoch != op.DataEpoch {
		return PublishGuardEventsResult{}, platform.NewError(platform.CodeVersionMismatch, false, "风控事件打开代次已变化")
	}
	config := platform.RelayConfig{BatchLimit: in.BatchSize, PollMillis: 1000, WriteWindow: runtime.WriteWindow.WriteWindow, Runtime: func(work context.Context) (platform.RuntimeView, error) {
		view, err := runtime.WriteWindow.Runtime(work)
		if err != nil {
			return platform.RuntimeView{}, err
		}
		if view.DataEpoch != op.DataEpoch {
			return platform.RuntimeView{}, platform.NewError(platform.CodeVersionMismatch, false, "风控事件运行代次已变化")
		}
		return view, nil
	}}
	result, err := outbox.RunRelayBatch(platform.WithOperationContext(ctx, op), outbox.NewStore(store.db), outbox.NewEventPublisher(runtime.Jobs), config)
	if err != nil {
		return PublishGuardEventsResult{}, err
	}
	return PublishGuardEventsResult{PublishedCount: result.Relayed}, nil
}
