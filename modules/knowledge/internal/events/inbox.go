package events

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type ConsumeEventsInput struct {
	ConsumerID  string `json:"consumerId"`
	MaxInFlight int    `json:"maxInFlight"`
}
type ConsumeEventsResult = platform.ConsumerRunResult

// ConsumeEvents 只装配本域登记的原信封处理函数；订阅、代次、确认和统计由平台拥有。
func ConsumeEvents(ctx context.Context, op OperationContext, input ConsumeEventsInput) (ConsumeEventsResult, error) {
	if op.CallerService != "knowledge" || op.Actor.Kind != "service" || input.ConsumerID != "knowledge" {
		return ConsumeEventsResult{}, platform.NewError(platform.CodeForbidden, false, "仅本域后台可订阅索引事件")
	}
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return ConsumeEventsResult{}, err
	}
	if runtime.Jobs == nil || runtime.WriteWindow.Runtime == nil || runtime.Validators["JobsAcknowledgeEventResult"] == nil || runtime.Validators["PublicationChangedPayload"] == nil || runtime.Validators["PublicationWithdrawnPayload"] == nil {
		return ConsumeEventsResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "索引事件订阅依赖未装配")
	}
	client, err := inbox.NewEventConsumer(runtime.Jobs, runtime.Validators["JobsAcknowledgeEventResult"])
	if err != nil {
		return ConsumeEventsResult{}, err
	}
	validate := func(eventType string, version int, payload json.RawMessage) error {
		if version != 1 {
			return platform.NewError(platform.CodeValidation, false, "索引事件版本未登记")
		}
		switch eventType {
		case "content.publication.changed":
			return runtime.Validators["PublicationChangedPayload"](payload)
		case "content.publication.withdrawn":
			return runtime.Validators["PublicationWithdrawnPayload"](payload)
		}
		return platform.NewError(platform.CodeValidation, false, "索引事件类型未登记")
	}
	handlers := platform.EventHandlerRegistry{
		"content.publication.changed": func(work context.Context, delivery platform.EventDelivery) error {
			current, _ := platform.OperationContextFrom(work)
			var payload struct {
				Reference gen.PublishedRevisionRef `json:"reference"`
			}
			if json.Unmarshal(delivery.Envelope.Payload, &payload) != nil {
				return platform.NewError(platform.CodeValidation, false, "发布事件绑定失败")
			}
			result, err := HandlePublication(work, current, HandlePublicationInput{EventID: delivery.Envelope.EventID, Document: payload.Reference, Event: delivery.Envelope})
			if err != nil {
				return err
			}
			if result.BuildID == nil {
				return inbox.ErrEventSkipped
			}
			return nil
		},
		"content.publication.withdrawn": func(work context.Context, delivery platform.EventDelivery) error {
			current, _ := platform.OperationContextFrom(work)
			var payload struct {
				DocumentID         string `json:"documentId"`
				VisibilityRevision int    `json:"visibilityRevision"`
			}
			if json.Unmarshal(delivery.Envelope.Payload, &payload) != nil {
				return platform.NewError(platform.CodeValidation, false, "撤回事件绑定失败")
			}
			result, err := indexer.RetireDocumentIndex(work, current, indexer.RetireDocumentIndexInput{DocumentID: payload.DocumentID, VisibilityRevision: payload.VisibilityRevision, EventID: delivery.Envelope.EventID, Event: delivery.Envelope})
			if err != nil {
				return err
			}
			if result.Retired == 0 {
				return inbox.ErrEventSkipped
			}
			return nil
		},
	}
	config := platform.ConsumerConfig{ConsumerID: "knowledge", SiteID: op.Actor.SiteID, Events: []string{"content.publication.changed@1", "content.publication.withdrawn@1"}, MaxInFlight: input.MaxInFlight, ValidatePayload: validate, Runtime: runtime.WriteWindow.Runtime}
	return inbox.RunConsumerWithResult(ctx, client, config, handlers)
}
