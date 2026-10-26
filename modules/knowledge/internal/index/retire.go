package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"time"
)

type RetireDocumentIndexInput struct {
	DocumentID         string                 `json:"documentId"`
	VisibilityRevision int                    `json:"visibilityRevision"`
	EventID            string                 `json:"eventId"`
	Event              platform.EventEnvelope `json:"event"`
}
type RetireDocumentIndexResult struct {
	Retired int `json:"retired"`
}

// RetireDocumentIndex 先关闭本域活动指针与旧租约，取消意图随同事件原子保存。
func RetireDocumentIndex(ctx context.Context, op OperationContext, input RetireDocumentIndexInput) (RetireDocumentIndexResult, error) {
	var payload struct {
		SiteID             string `json:"siteId"`
		DocumentID         string `json:"documentId"`
		VisibilityRevision int    `json:"visibilityRevision"`
		DataEpoch          string `json:"dataEpoch"`
	}
	if json.Unmarshal(input.Event.Payload, &payload) != nil || input.Event.EventType != "content.publication.withdrawn" || input.EventID != input.Event.EventID || payload.SiteID != op.Actor.SiteID || payload.DocumentID != input.DocumentID || input.DocumentID != input.Event.SubjectID || payload.VisibilityRevision != input.VisibilityRevision || int64(input.VisibilityRevision) != input.Event.Revision || payload.DataEpoch != op.DataEpoch {
		return RetireDocumentIndexResult{}, platform.NewError(platform.CodeValidation, false, "撤回事件引用或水位不一致")
	}
	work, tx, cancel, err := BeginIndexWrite(ctx, op)
	if err != nil {
		return RetireDocumentIndexResult{}, err
	}
	defer cancel()
	defer tx.Rollback()
	consumed, err := ConsumeInbox(work, op, ConsumeInboxInput{Event: input.Event}, tx)
	if err != nil {
		return RetireDocumentIndexResult{}, err
	}
	if consumed.Duplicate {
		_ = tx.Rollback()
		cancel()
		if err = CancelDocumentSideEffects(ctx, op, input.DocumentID); err != nil {
			return RetireDocumentIndexResult{}, err
		}
		return RetireDocumentIndexResult{}, nil
	}
	var known int
	err = tx.QueryRowContext(work, `SELECT visibility_revision FROM document_watermarks WHERE site_id=? AND data_epoch=? AND document_id=?`, op.Actor.SiteID, op.DataEpoch, input.DocumentID).Scan(&known)
	if err != nil && err != sql.ErrNoRows {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	if known >= input.VisibilityRevision {
		if err = tx.Commit(); err != nil {
			return RetireDocumentIndexResult{}, storageError(err)
		}
		cancel()
		if err = CancelDocumentSideEffects(ctx, op, input.DocumentID); err != nil {
			return RetireDocumentIndexResult{}, err
		}
		return RetireDocumentIndexResult{}, nil
	}
	if _, err = tx.ExecContext(work, `INSERT INTO document_watermarks(site_id,data_epoch,document_id,visibility_revision,withdrawn) VALUES(?,?,?,?,1) ON CONFLICT(site_id,data_epoch,document_id) DO UPDATE SET visibility_revision=excluded.visibility_revision,withdrawn=1`, op.Actor.SiteID, op.DataEpoch, input.DocumentID, input.VisibilityRevision); err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	rows, err := tx.QueryContext(work, `SELECT build_id FROM active_indexes WHERE site_id=? AND data_epoch=? AND document_id=? ORDER BY space_id`, op.Actor.SiteID, op.DataEpoch, input.DocumentID)
	if err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return RetireDocumentIndexResult{}, storageError(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	active := []BuildRecord{}
	for _, id := range ids {
		row, err := ReadBuildRecord(work, tx, op, id)
		if err != nil {
			return RetireDocumentIndexResult{}, err
		}
		active = append(active, row)
	}
	count := len(active)
	now := time.Now().UTC()
	if _, err = tx.ExecContext(work, `UPDATE builds SET generation_state='retired',retired_at_ns=?,cancelled=CASE WHEN state='active' THEN cancelled ELSE 1 END,lease_id='',lease_until='',state=CASE WHEN state='active' THEN state ELSE 'superseded' END WHERE site_id=? AND data_epoch=? AND document_id=? AND state NOT IN ('failed','cancelled','superseded')`, now.UnixNano(), op.Actor.SiteID, op.DataEpoch, input.DocumentID); err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	if _, err = tx.ExecContext(work, `UPDATE build_dispatches SET cancel_requested=1,updated_at=? WHERE build_id IN (SELECT build_id FROM builds WHERE site_id=? AND data_epoch=? AND document_id=?)`, now.Format(time.RFC3339Nano), op.Actor.SiteID, op.DataEpoch, input.DocumentID); err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	if _, err = tx.ExecContext(work, `UPDATE execution_records SET cancel_requested=1 WHERE site_id=? AND data_epoch=? AND job_type='knowledge.build' AND object_id IN (SELECT build_id FROM builds WHERE site_id=? AND data_epoch=? AND document_id=?)`, op.Actor.SiteID, op.DataEpoch, op.Actor.SiteID, op.DataEpoch, input.DocumentID); err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	cancelledRows, err := tx.QueryContext(work, `SELECT build_id FROM builds WHERE site_id=? AND data_epoch=? AND document_id=? AND cancelled=1`, op.Actor.SiteID, op.DataEpoch, input.DocumentID)
	if err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	cancelledIDs := []string{}
	for cancelledRows.Next() {
		var id string
		if err = cancelledRows.Scan(&id); err != nil {
			cancelledRows.Close()
			return RetireDocumentIndexResult{}, storageError(err)
		}
		cancelledIDs = append(cancelledIDs, id)
	}
	err = cancelledRows.Err()
	cancelledRows.Close()
	if err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	for _, id := range cancelledIDs {
		if err = CancelUnusedExtractions(work, tx, op, id); err != nil {
			return RetireDocumentIndexResult{}, err
		}
	}
	if _, err = tx.ExecContext(work, `DELETE FROM active_indexes WHERE site_id=? AND data_epoch=? AND document_id=?`, op.Actor.SiteID, op.DataEpoch, input.DocumentID); err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	for _, row := range active {
		payloadRaw, _ := json.Marshal(map[string]any{"documentId": input.DocumentID, "publishedRevisionId": row.Build.Document.PublishedRevisionId, "visibilityRevision": input.VisibilityRevision, "spaceId": row.Build.Space.SpaceId, "dataEpoch": op.DataEpoch})
		eventID := "index.retired." + platform.HashBytes([]byte(input.EventID+":"+row.Build.BuildId))
		if err = outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: eventID, EventType: "index.retired", EventVersion: 1, Producer: "knowledge", SubjectID: input.DocumentID, Revision: int64(input.VisibilityRevision), OccurredAt: now.Format(time.RFC3339Nano), DataEpoch: op.DataEpoch, Payload: payloadRaw}); err != nil {
			return RetireDocumentIndexResult{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return RetireDocumentIndexResult{}, storageError(err)
	}
	cancel()
	if runtime, runtimeErr := RuntimeFromContext(ctx); runtimeErr == nil && runtime.Search != nil {
		_ = runtime.Search.DeleteDocument(ctx, op.Actor.SiteID, input.DocumentID) // 失败由对账循环补删
	}
	if err = CancelDocumentSideEffects(ctx, op, input.DocumentID); err != nil {
		return RetireDocumentIndexResult{}, err
	}
	return RetireDocumentIndexResult{Retired: count}, nil
}
