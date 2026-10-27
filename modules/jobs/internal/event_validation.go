package internal

import (
	"nice-wiki/libs/platform"
	"nice-wiki/modules/jobs/internal/gen"
)

type EventDefinition = gen.EventTypeEntry

func findEventDefinition(name string, version int) (EventDefinition, error) {
	for _, definition := range gen.EventTypes {
		if definition.Name == name && definition.Version == version {
			return definition, nil
		}
	}
	return EventDefinition{}, platform.NewError(platform.CodeValidation, false, "事件类型或版本未登记")
}
