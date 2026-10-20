package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/media/internal/gen"
)

const referenceChangedEvent = "content.resources.changed"

// resourceReferencesChangedPayloadV2 是content v2事件的完整pin材料。
// eventId只存在jobs信封，不能从payload补造或接受。
type resourceReferencesChangedPayloadV2 struct {
	References        []gen.AssetReference `json:"references"`
	SourceService     string               `json:"sourceService"`
	SourceOperationId Identifier            `json:"sourceOperationId"`
	SiteId            Identifier            `json:"siteId"`
	SourceObjectId    Identifier            `json:"sourceObjectId"`
	SourceRevisionId  Identifier            `json:"sourceRevisionId"`
	SourceVersion     gen.Revision         `json:"sourceVersion"`
	ReferenceDigest   Hash                 `json:"referenceDigest"`
}

// ConsumeEventsInput 由受信启动链装配，不接受HTTP或用户输入伪造客户端与运行权威。
type ConsumeEventsInput struct {
	Client         platform.EventConsumerClient `json:"-"`
	Runtime        func(context.Context) (platform.RuntimeView, error) `json:"-"`
	AdmitBatch     platform.WorkAdmission `json:"-"`
	AdmitDelivery  platform.WorkAdmission `json:"-"`
	ConsumerID     string `json:"consumerId"`
	MaxInFlight    int `json:"maxInFlight"`
}

type ConsumeEventsResult = platform.ConsumerRunResult

// ConsumeEvents 使用公共订阅循环处理content资源引用v2事件。
func ConsumeEvents(ctx context.Context, op OperationContext, input ConsumeEventsInput) (ConsumeEventsResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return ConsumeEventsResult{}, platform.NewError(platform.CodeForbidden, false, "引用事件订阅仅供本域受信启动")
	}
	if input.ConsumerID != "media" || input.Client == nil || input.Runtime == nil || input.AdmitBatch == nil || input.AdmitDelivery == nil {
		return ConsumeEventsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "引用事件订阅依赖未装配")
	}
	validatePayload, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "ContentResourcesChangedPayloadV2")
	if err != nil {
		return ConsumeEventsResult{}, err
	}
	validateEvent := func(eventType string, version int, payload json.RawMessage) error {
		if eventType != referenceChangedEvent || version != 2 {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "引用事件版本未登记")
		}
		return validatePayload(payload)
	}
	config := platform.ConsumerConfig{
		ConsumerID:  input.ConsumerID,
		SiteID:     op.Actor.SiteID,
		Events:     []string{referenceChangedEvent + "@2"},
		MaxInFlight: input.MaxInFlight,
		Runtime:    input.Runtime,
		AdmitBatch: input.AdmitBatch,
		AdmitDelivery: input.AdmitDelivery,
		ValidatePayload: validateEvent,
	}
	handlers := platform.EventHandlerRegistry{referenceChangedEvent: handleReferenceChangedEvent}
	return inbox.RunConsumerWithResult(platform.WithOperationContext(ctx, op), input.Client, config, handlers)
}

// handleReferenceChangedEvent 将jobs信封和v2 payload映射为本域精确事件输入。
func handleReferenceChangedEvent(ctx context.Context, delivery platform.EventDelivery) error {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "引用事件缺少可信操作上下文")
	}
	input, err := decodeReferenceChangedEvent(delivery.Envelope)
	if err != nil {
		return err
	}
	_, err = ApplyReferenceEvent(ctx, op, input)
	return err
}

func decodeReferenceChangedEvent(envelope platform.EventEnvelope) (ApplyReferenceEventInput, error) {
	if envelope.EventType != referenceChangedEvent || envelope.EventVersion != 2 {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeUnsupportedSchema, false, "引用事件版本未登记")
	}
	if envelope.Producer != "content" {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeIntegrityError, false, "引用事件生产方与事件类型不符")
	}
	var payload resourceReferencesChangedPayloadV2
	decoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeValidation, false, "引用事件v2载荷无法解码")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeValidation, false, "引用事件v2载荷包含多余数据")
	}
	if payload.SourceService != "content" {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeIntegrityError, false, "引用事件来源服务不符")
	}
	if envelope.SubjectID != payload.SourceObjectId || envelope.Revision != int64(payload.SourceVersion) {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeIntegrityError, false, "引用事件信封水位与来源材料不符")
	}
	return ApplyReferenceEventInput{
		References:        payload.References,
		SourceService:     payload.SourceService,
		SourceOperationId: payload.SourceOperationId,
		SiteId:            payload.SiteId,
		SourceObjectId:    payload.SourceObjectId,
		SourceRevisionId:  payload.SourceRevisionId,
		SourceVersion:     payload.SourceVersion,
		ReferenceDigest:   payload.ReferenceDigest,
		EventId:           envelope.EventID,
	}, nil
}

func newMediaEventConsumer(client *gen.JobsClient) (platform.EventConsumerClient, error) {
	if client == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "引用事件jobs客户端未装配")
	}
	validateAck, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "JobsAcknowledgeEventResult")
	if err != nil {
		return nil, err
	}
	return inbox.NewEventConsumer(client, validateAck)
}

func newMediaEventAdmission(window platform.WriteWindowConfig) platform.WorkAdmission {
	return func(ctx context.Context) (context.Context, func(), error) {
		work, _, cancel, err := lifecycle.NewWriteContext(ctx, window)
		if err != nil {
			return nil, nil, err
		}
		return work, cancel, nil
	}
}
