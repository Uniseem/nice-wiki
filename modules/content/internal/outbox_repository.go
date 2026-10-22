package content

import (
	"context"
	"database/sql"
	"encoding/json"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/content/internal/gen"
)

type RelayOutboxInput struct {
	BatchLimit int `json:"batchLimit"`
}
type RelayOutboxResult = platform.RelayBatchResult

// RelayRuntime仅由本域后台入口装配；不接受HTTP请求触发。
type RelayRuntime struct {
	Store       *Store
	Jobs        *gen.JobsClient
	WriteWindow platform.WriteWindowConfig
}
type relayRuntimeKey struct{}

func WithRelayRuntime(ctx context.Context, runtime RelayRuntime) context.Context {
	return context.WithValue(ctx, relayRuntimeKey{}, runtime)
}

// RelayOutbox只装配真实本域数据库和版本化jobs客户端，机制统一由平台执行。
func RelayOutbox(ctx context.Context, op OperationContext, input RelayOutboxInput) (RelayOutboxResult, error) {
	if op.CallerService != "content" || op.Actor.Kind != "service" || op.Actor.SubjectID != "content" {
		return RelayOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "事件转发仅允许本域后台服务")
	}
	runtime, ok := ctx.Value(relayRuntimeKey{}).(RelayRuntime)
	if !ok || runtime.Store == nil || runtime.Jobs == nil {
		return RelayOutboxResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "文档事件转发依赖未装配")
	}
	if err := runtime.Store.checkRevisionScope(ctx, op); err != nil {
		return RelayOutboxResult{}, err
	}
	if err := releasePurgedBacklog(ctx, op, runtime); err != nil {
		return RelayOutboxResult{}, err
	}
	return outbox.RunRelayBatch(platform.WithOperationContext(ctx, op), outbox.NewStore(runtime.Store.db), outbox.NewEventPublisher(runtime.Jobs), platform.RelayConfig{BatchLimit: input.BatchLimit, PollMillis: 1000, Runtime: runtime.WriteWindow.Runtime, WriteWindow: runtime.WriteWindow.WriteWindow})
}

type publicationChangedPayload struct {
	SiteID             Identifier               `json:"siteId"`
	Reference          gen.PublishedRevisionRef `json:"reference"`
	VisibilityRevision int                      `json:"visibilityRevision"`
	AuthorAccountID    Identifier               `json:"authorAccountId"`
	OccurredAt         platform.Timestamp       `json:"occurredAt"`
}

type publicationWithdrawnPayload struct {
	SiteID             Identifier         `json:"siteId"`
	DocumentID         Identifier         `json:"documentId"`
	Revision           int                `json:"revision"`
	VisibilityRevision int                `json:"visibilityRevision"`
	DataEpoch          Identifier         `json:"dataEpoch"`
	Reason             string             `json:"reason"`
	OccurredAt         platform.Timestamp `json:"occurredAt"`
}

type resourceReferencesChangedPayload struct {
	SiteID              Identifier              `json:"siteId"`
	DocumentID          Identifier              `json:"documentId"`
	SourceRevision      int                     `json:"sourceRevision"`
	References          []gen.ResourceReference `json:"references"`
	RemovedReferenceIDs []Identifier            `json:"removedReferenceIds"`
	DataEpoch           Identifier              `json:"dataEpoch"`
}

type PublishContentEventInput struct {
	Changed    *publicationChangedPayload        `json:"changed"`
	Withdrawn  *publicationWithdrawnPayload      `json:"withdrawn"`
	References *resourceReferencesChangedPayload `json:"references"`
	EventID    Identifier                        `json:"eventId"`
	// Revision 是outbox排序水位。发布与撤回使用本次visibilityRevision。
	Revision int `json:"-"`
}

type PublishContentEventResult struct {
	Queued bool `json:"queued"`
}

type publicationTxKey struct{}

func bindPublicationTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, publicationTxKey{}, tx)
}

func publicationTx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(publicationTxKey{}).(*sql.Tx)
	return tx, ok && tx != nil
}

// PublishContentEvent只把一种发布载荷写入当前业务事务的outbox，不调用jobs。
func PublishContentEvent(ctx context.Context, op OperationContext, input PublishContentEventInput) (PublishContentEventResult, error) {
	if input.References != nil {
		return PublishContentEventResult{}, platform.NewError(platform.CodeValidation, false, "发布事件不能生产旧版资源引用载荷")
	}
	tx, ok := publicationTx(ctx)
	if !ok {
		return PublishContentEventResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "发布事件只能写入当前业务事务")
	}
	var payload any
	var eventType string
	var subject Identifier
	var revision int64
	occurred := platform.Now()
	switch {
	case input.Changed != nil && input.Withdrawn == nil && input.References == nil:
		payload, eventType, subject, revision, occurred = input.Changed, "content.publication.changed", input.Changed.Reference.DocumentId, int64(input.Changed.VisibilityRevision), input.Changed.OccurredAt
	case input.Withdrawn != nil && input.Changed == nil && input.References == nil:
		payload, eventType, subject, revision, occurred = input.Withdrawn, "content.publication.withdrawn", input.Withdrawn.DocumentID, int64(input.Withdrawn.VisibilityRevision), input.Withdrawn.OccurredAt
	default:
		return PublishContentEventResult{}, platform.NewError(platform.CodeValidation, false, "发布事件只能有一种发布或撤回载荷，不能生产资源引用事件")
	}
	if input.Revision > 0 {
		revision = int64(input.Revision)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return PublishContentEventResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布事件载荷无法编码")
	}
	if err = outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: input.EventID, EventType: eventType, EventVersion: 1, Producer: "content", SubjectID: subject, Revision: revision, OccurredAt: occurred, DataEpoch: op.DataEpoch, Payload: raw}); err != nil {
		return PublishContentEventResult{}, err
	}
	return PublishContentEventResult{Queued: true}, nil
}
