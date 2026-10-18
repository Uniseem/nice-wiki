package analytics

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
)

type ConsumeEventsInput struct {
	ConsumerID  string `json:"consumerId"`
	MaxInFlight int    `json:"maxInFlight"`
}
type ConsumeEventsResult struct {
	Acknowledged int64 `json:"acknowledged"`
	Skipped      int64 `json:"skipped"`
	Failed       int64 `json:"failed"`
}

// ConsumeEvents将费用事件交给权威页投影入口，提交成功后由平台确认。
func ConsumeEvents(ctx context.Context, op OperationContext, input ConsumeEventsInput) (ConsumeEventsResult, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return ConsumeEventsResult{}, err
	}
	if input.ConsumerID != "analytics" || rt.Consumer == nil || rt.ValidateEvent == nil {
		return ConsumeEventsResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "事件consumer身份或真实依赖未装配")
	}
	qa := func(ctx context.Context, event EventDelivery) error {
		call, ok := platform.OperationContextFrom(ctx)
		if !ok {
			return platform.NewError(platform.CodeUnauthenticated, false, "订阅上下文缺失")
		}
		result, err := ProjectQaTurn(WithRuntime(ctx, rt), call, event)
		if err == nil && !result.Applied {
			return inbox.ErrEventSkipped
		}
		return err
	}
	usage := func(ctx context.Context, event EventDelivery) error {
		call, ok := platform.OperationContextFrom(ctx)
		if !ok {
			return platform.NewError(platform.CodeUnauthenticated, false, "订阅上下文缺失")
		}
		result, err := ConsumeAnalyticsEvent(WithUsageGateway(WithRuntime(ctx, rt), rt.UsageGateway), call, ConsumeAnalyticsEventInput{Event: event.Envelope})
		if err == nil && !result.Accepted {
			return inbox.ErrEventSkipped
		}
		return err
	}
	counts, err := inbox.RunConsumerWithResult(ctx, rt.Consumer, platform.ConsumerConfig{SiteID: rt.SiteID, ConsumerID: "analytics", Events: []string{"qa.turn.completed@1", "qa.turn.failed@1", "invocation.settled@1", "invocation.reconciled@1"}, MaxInFlight: input.MaxInFlight, Runtime: rt.Window.Runtime, ValidatePayload: rt.ValidateEvent}, platform.EventHandlerRegistry{"qa.turn.completed": qa, "qa.turn.failed": qa, "invocation.settled": usage, "invocation.reconciled": usage})
	return ConsumeEventsResult{Acknowledged: counts.Acknowledged, Skipped: counts.Skipped, Failed: counts.Failed}, err
}
