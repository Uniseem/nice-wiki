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

type RelayOutboxInput struct {
	BatchLimit int `json:"batchLimit"`
}

type RelayOutboxResult = platform.RelayBatchResult

// RelayRuntime 由完整Application从同一SnapshotManager和生成jobs客户端装配。
// 单批函数本身不接受请求或自行建立第二套租约。
type RelayRuntime struct {
	Store          *outbox.Store
	Publisher      platform.EventPublisher
	Runtime        func(context.Context) (platform.RuntimeView, error)
	WriteWindow    func(context.Context) (time.Time, error)
	AdmitBatch     platform.WorkAdmission
	BindEventScope func(context.Context, platform.OutboxRow) (context.Context, func(), error)
}

type relayRuntimeKey struct{}

func WithRelayRuntime(ctx context.Context, runtime RelayRuntime) context.Context {
	return context.WithValue(ctx, relayRuntimeKey{}, runtime)
}

// RelayOutbox 只把已装配的真实Store、publisher和scope交给平台单批转发。
func RelayOutbox(ctx context.Context, op platform.OperationContext, input RelayOutboxInput) (RelayOutboxResult, error) {
	if op.CallerService != "site-config" || op.Actor.Kind != "service" || op.Actor.SubjectID != "site-config" {
		return RelayOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "配置事件转发仅允许本域可信后台")
	}
	runtime, ok := ctx.Value(relayRuntimeKey{}).(RelayRuntime)
	if !ok || runtime.Store == nil || runtime.Publisher == nil || runtime.Runtime == nil || runtime.WriteWindow == nil || runtime.AdmitBatch == nil || runtime.BindEventScope == nil {
		return RelayOutboxResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "配置事件转发依赖未装配")
	}
	work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
	return outbox.RunRelayBatch(work, runtime.Store, runtime.Publisher, platform.RelayConfig{
		AdmitBatch:     runtime.AdmitBatch,
		BindEventScope: runtime.BindEventScope,
		BatchLimit:     input.BatchLimit,
		PollMillis:     1000,
		Runtime:        runtime.Runtime,
		WriteWindow:    runtime.WriteWindow,
	})
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
