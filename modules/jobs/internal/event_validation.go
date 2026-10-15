package internal

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/modules/jobs/internal/gen"
)

type EventDefinition = gen.EventTypeEntry

type ValidateEnvelopeInput struct{ Event platform.EventEnvelope }
type ValidateEnvelopeResult struct{ Valid bool }

type payloadSchema struct {
	once     sync.Once
	validate func(json.RawMessage) error
	err      error
}

var eventPayloadSchemas sync.Map

// ValidateEnvelope 只接受目录固定的生产者与版本，载荷校验机制统一在platform。
func ValidateEnvelope(ctx context.Context, op OperationContext, in ValidateEnvelopeInput) (ValidateEnvelopeResult, error) {
	definition, err := findEventDefinition(in.Event.EventType, in.Event.EventVersion)
	if err != nil {
		return ValidateEnvelopeResult{}, err
	}
	if definition.Producer != in.Event.Producer || op.CallerService != definition.Producer {
		return ValidateEnvelopeResult{}, platform.NewError(platform.CodeForbidden, false, "事件生产方与服务身份不符")
	}
	if in.Event.DataEpoch != op.DataEpoch {
		return ValidateEnvelopeResult{}, epochMismatch()
	}
	if err := validateEventPayload(definition, in.Event.Payload); err != nil {
		return ValidateEnvelopeResult{}, err
	}
	return ValidateEnvelopeResult{Valid: true}, nil
}

func findEventDefinition(name string, version int) (EventDefinition, error) {
	for _, definition := range gen.EventTypes {
		if definition.Name == name && definition.Version == version {
			return definition, nil
		}
	}
	return EventDefinition{}, platform.NewError(platform.CodeValidation, false, "事件类型或版本未登记")
}

func validateEventPayload(definition EventDefinition, payload []byte) error {
	key := definition.Name + "@" + strconv.Itoa(definition.Version)
	loaded, _ := eventPayloadSchemas.LoadOrStore(key, &payloadSchema{})
	schema := loaded.(*payloadSchema)
	schema.once.Do(func() {
		schema.validate, schema.err = apivalidate.NewPayloadValidator(definition.Document, definition.SchemaName)
	})
	if schema.err != nil {
		return schema.err
	}
	return schema.validate(payload)
}

func consumerHasEvent(consumer string, name string, version int) bool {
	for _, definition := range gen.EventTypes {
		if definition.Name != name || definition.Version != version {
			continue
		}
		for _, registered := range definition.Consumers {
			if registered == consumer {
				return true
			}
		}
	}
	return false
}
