package media

import (
	"context"
	"database/sql"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/media/internal/gen"
)

// PublishAssetEventInput 仅由资源提交链提供同一本域事务，不通过HTTP序列化。
type PublishAssetEventInput struct {
	Tx                  *sql.Tx
	Asset               gen.Asset
	EventID             Identifier
	OccurredAt          platform.Timestamp
	ReferenceGeneration int
}

// PublishAssetEvent 在ready或人工deleted事务写入最小事件，转发由RelayOutbox负责。
func PublishAssetEvent(ctx context.Context, op OperationContext, input PublishAssetEventInput) error {
	payload, err := platform.CanonicalJSON(struct {
		SiteID              Identifier          `json:"siteId"`
		Asset               gen.AssetVersionRef `json:"asset"`
		State               string              `json:"state"`
		ReferenceGeneration int                 `json:"referenceGeneration"`
		OccurredAt          platform.Timestamp  `json:"occurredAt"`
	}{input.Asset.SiteId, input.Asset.Ref, string(input.Asset.State), input.ReferenceGeneration, input.OccurredAt})
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "资源事件无法编码")
	}
	return outbox.EnqueueEvent(input.Tx, platform.EventEnvelope{EventID: input.EventID, EventType: "media.asset.changed", EventVersion: 1, Producer: "media", SubjectID: input.Asset.Ref.AssetId, Revision: platform.Revision(input.Asset.Revision), OccurredAt: input.OccurredAt, DataEpoch: op.DataEpoch, Payload: payload})
}

// RelayOutboxInput按公共单批合同装配，不接受HTTP任意触发。
type RelayOutboxInput struct {
	BatchLimit int `json:"batchLimit"`
}
type RelayOutboxResult = platform.RelayBatchResult

// RelayOutbox只把本域私有outbox与真实生成客户端交给平台单批机制。
func RelayOutbox(ctx context.Context, op OperationContext, input RelayOutboxInput) (RelayOutboxResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return RelayOutboxResult{}, platform.NewError(platform.CodeForbidden, false, "资源事件转发仅允许本域可信后台")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return RelayOutboxResult{}, err
	}
	if s.cleanupJobs == nil || s.window.Runtime == nil || s.window.WriteWindow == nil {
		return RelayOutboxResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "资源事件转发依赖未装配")
	}
	work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
	return outbox.RunRelayBatch(work, outbox.NewStore(s.db), outbox.NewEventPublisher(s.cleanupJobs), platform.RelayConfig{BatchLimit: input.BatchLimit, PollMillis: 1000, Runtime: s.window.Runtime, WriteWindow: s.window.WriteWindow})
}
