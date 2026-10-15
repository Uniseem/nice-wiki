package siteconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/site-config/internal/gen"
)

type WriteTransaction = *sql.Tx

type ConfigChanged struct {
	SiteId          gen.Identifier  `json:"siteId"`
	ConfigRevision  gen.Revision    `json:"configRevision"`
	BaseLanguage    gen.LanguageTag `json:"baseLanguage"`
	HomeRevision    gen.Revision    `json:"homeRevision"`
	ChangedSections []string        `json:"changedSections"`
	OccurredAt      time.Time       `json:"occurredAt"`
}

type PublishConfigEventInput struct {
	Event   ConfigChanged  `json:"event"`
	EventId gen.Identifier `json:"eventId"`
}

type PublishConfigEventResult struct {
	Queued bool `json:"queued"`
}

// PublishConfigEvent 仅在配置提交方持有的业务事务内登记平台outbox。
func PublishConfigEvent(ctx context.Context, op platform.OperationContext, tx WriteTransaction, input PublishConfigEventInput) (PublishConfigEventResult, error) {
	raw, err := json.Marshal(input.Event)
	if err != nil {
		return PublishConfigEventResult{}, platform.NewError(platform.CodeIntegrityError, false, "配置事件无法序列化")
	}
	envelope := platform.EventEnvelope{
		EventID: input.EventId, EventType: "site.config.changed", EventVersion: 1,
		Producer: "site-config", SubjectID: input.Event.SiteId,
		Revision: platform.Revision(input.Event.ConfigRevision), DataEpoch: op.DataEpoch,
		OccurredAt: platform.Timestamp(input.Event.OccurredAt.UTC().Format(time.RFC3339Nano)), Payload: raw,
	}
	if err := outbox.EnqueueEvent(tx, envelope); err != nil {
		return PublishConfigEventResult{}, err
	}
	return PublishConfigEventResult{Queued: true}, nil
}
