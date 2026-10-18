package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/knowledge/internal/build"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

type OperationContext = platform.OperationContext
type HandlePublicationInput struct {
	EventID  string                   `json:"eventId"`
	Document gen.PublishedRevisionRef `json:"document"`
	Event    platform.EventEnvelope   `json:"event"`
}
type HandlePublicationResult struct {
	Accepted bool    `json:"accepted"`
	BuildID  *string `json:"buildId,omitempty"`
}

// HandlePublication 不反向同步回源；当前性在读取源和模型派发前再次由content证明。
func HandlePublication(ctx context.Context, op OperationContext, input HandlePublicationInput) (HandlePublicationResult, error) {
	var payload struct {
		SiteID             string                   `json:"siteId"`
		Reference          gen.PublishedRevisionRef `json:"reference"`
		VisibilityRevision int                      `json:"visibilityRevision"`
	}
	if json.Unmarshal(input.Event.Payload, &payload) != nil || input.EventID != input.Event.EventID || input.Event.EventType != "content.publication.changed" || payload.SiteID != op.Actor.SiteID || payload.Reference != input.Document || input.Document.DocumentId != input.Event.SubjectID || input.Document.DataEpoch != op.DataEpoch || int64(payload.VisibilityRevision) != input.Event.Revision {
		return HandlePublicationResult{}, platform.NewError(platform.CodeValidation, false, "发布事件引用或水位不一致")
	}
	work, tx, cancel, err := indexer.BeginIndexWrite(ctx, op)
	if err != nil {
		return HandlePublicationResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	inbox, err := indexer.ConsumeInbox(work, op, indexer.ConsumeInboxInput{Event: input.Event}, tx)
	if err != nil {
		return HandlePublicationResult{}, err
	}
	result := HandlePublicationResult{Accepted: true}
	if inbox.Duplicate {
		return result, nil
	}
	var known int
	err = tx.QueryRowContext(work, `SELECT visibility_revision FROM document_watermarks WHERE site_id=? AND data_epoch=? AND document_id=?`, op.Actor.SiteID, op.DataEpoch, input.Document.DocumentId).Scan(&known)
	if err != nil && err != sql.ErrNoRows {
		return HandlePublicationResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "发布消费水位读取失败")
	}
	if known >= payload.VisibilityRevision {
		if err = tx.Commit(); err != nil {
			return HandlePublicationResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "旧发布事件消费提交失败")
		}
		return result, nil
	}
	_, err = tx.ExecContext(work, `INSERT INTO document_watermarks(site_id,data_epoch,document_id,visibility_revision,withdrawn) VALUES(?,?,?,?,0) ON CONFLICT(site_id,data_epoch,document_id) DO UPDATE SET visibility_revision=excluded.visibility_revision,withdrawn=0`, op.Actor.SiteID, op.DataEpoch, input.Document.DocumentId, payload.VisibilityRevision)
	if err != nil {
		return HandlePublicationResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "发布消费水位保存失败")
	}
	config, err := indexer.ReadConfig(work, tx, op)
	var failure *platform.Error
	missing := errors.As(err, &failure) && failure.Code == platform.CodeNotFound
	if err != nil && !missing {
		return HandlePublicationResult{}, err
	}
	if !missing && config.Enabled {
		queued, err := build.QueueBuild(work, tx, op, config, input.Document, false)
		if err != nil {
			return HandlePublicationResult{}, err
		}
		result.BuildID = &queued.BuildId
	}
	if err = tx.Commit(); err != nil {
		return HandlePublicationResult{}, platform.NormalizeIOError(err, platform.CodeInternal, false, "发布消费与构建接受提交失败")
	}
	if result.BuildID != nil {
		_ = build.DispatchBuild(ctx, op, *result.BuildID)
	}
	return result, nil
}
