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
	Tx         *sql.Tx
	Asset      gen.Asset
	EventID    Identifier
	OccurredAt platform.Timestamp
}

// PublishAssetEvent 在ready事务写入最小资源事件，转发由后续RelayOutbox负责。
func PublishAssetEvent(ctx context.Context, op OperationContext, input PublishAssetEventInput) error {
	payload, err := platform.CanonicalJSON(struct {
		SiteID              Identifier          `json:"siteId"`
		Asset               gen.AssetVersionRef `json:"asset"`
		State               string              `json:"state"`
		ReferenceGeneration gen.Revision        `json:"referenceGeneration"`
		OccurredAt          platform.Timestamp  `json:"occurredAt"`
	}{input.Asset.SiteId, input.Asset.Ref, string(input.Asset.State), 0, input.OccurredAt})
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "资源事件无法编码")
	}
	return outbox.EnqueueEvent(input.Tx, platform.EventEnvelope{EventID: input.EventID, EventType: "media.asset.changed", EventVersion: 1, Producer: "media", SubjectID: input.Asset.Ref.AssetId, Revision: platform.Revision(input.Asset.Revision), OccurredAt: input.OccurredAt, DataEpoch: op.DataEpoch, Payload: payload})
}
