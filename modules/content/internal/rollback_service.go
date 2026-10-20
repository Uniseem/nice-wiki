package content

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type RollbackPublishedRevisionInput struct {
	DocumentId               Identifier `json:"documentId"`
	HistoryId                Identifier `json:"historyId"`
	ExpectedRevision         int        `json:"expectedRevision"`
	AcknowledgeMissingAssets bool       `json:"acknowledgeMissingAssets"`
}
type RollbackPublishedRevisionResult = gen.RollbackPublishedRevisionResult

// RollbackPublishedRevision 把历史上某次发布的正文写成新的共享草稿，再按新发布走完整发布流程；
// 历史本身不被改写。需要先能取得该文档的编辑锁（别人正在编辑时返回 LOCKED）。
func RollbackPublishedRevision(ctx context.Context, op OperationContext, input RollbackPublishedRevisionInput) (RollbackPublishedRevisionResult, error) {
	tree, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	drafts, err := draftRuntimeFromContext(ctx)
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	accountName, err := authorizePublication(ctx, op, tree, "rollbackPublishedRevision", input.DocumentId)
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	var publishedID string
	err = drafts.Store.db.QueryRowContext(ctx, `SELECT published_revision_id FROM publication_history WHERE site_id=? AND document_id=? AND history_id=?`, op.Actor.SiteID, input.DocumentId, input.HistoryId).Scan(&publishedID)
	if errors.Is(err, sql.ErrNoRows) {
		return RollbackPublishedRevisionResult{}, platform.NewError(platform.CodeNotFound, false, "指定发布历史不存在")
	}
	if err != nil {
		return RollbackPublishedRevisionResult{}, contentStorageError()
	}
	body, err := readHistoricalBody(ctx, op, tree, input.DocumentId, publishedID)
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	metadata, draft, err := drafts.Store.readDraftSnapshot(ctx, op, input.DocumentId)
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	if int(metadata.Revision) != input.ExpectedRevision {
		return RollbackPublishedRevisionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "文档修订已变化")
	}
	snapshot, err := drafts.Manager.AcquireLease(ctx, op, AcquireLeaseInput{DocumentID: input.DocumentId, TabID: platform.NewIdentifier(), AccountName: accountName})
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	if !snapshot.Writable || snapshot.Lease == nil {
		return RollbackPublishedRevisionResult{}, platform.NewError(platform.CodeLocked, false, "其他人正在编辑该文档，无法回滚")
	}
	release := func() {
		_, _ = drafts.Manager.ReleaseLease(ctx, op, ReleaseEditLockInput{DocumentId: input.DocumentId, LeaseId: snapshot.Lease.LeaseId, ServerBootId: snapshot.ServerBootID})
	}
	saved, err := SaveDraft(ctx, op, SaveDraftInput{DocumentId: input.DocumentId, Body: body, ExpectedDraftVersion: draft.DraftVersion, ExpectedDraftRevisionId: draft.RevisionId,
		LeaseId: snapshot.Lease.LeaseId, FencingToken: int(snapshot.FencingToken), ServerBootId: snapshot.ServerBootID})
	release()
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	fresh, _, err := drafts.Store.readDraftSnapshot(ctx, op, input.DocumentId)
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	published, err := PublishDocument(ctx, op, PublishDocumentInput{DocumentId: input.DocumentId, DraftRevisionId: saved.Draft.RevisionId, ExpectedDraftVersion: saved.Draft.DraftVersion, ExpectedRevision: fresh.Revision})
	if err != nil {
		return RollbackPublishedRevisionResult{}, err
	}
	return RollbackPublishedRevisionResult{Reference: published.Reference, History: published.History, MissingAssets: []Identifier{}}, nil
}
