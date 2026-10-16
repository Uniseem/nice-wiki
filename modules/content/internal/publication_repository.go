package content

import (
	"context"
	"database/sql"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type ReadReferenceDecisionInput struct {
	SourceOperationId Identifier `json:"sourceOperationId"`
}
type ReadReferenceDecisionResult = gen.ReadReferenceDecisionResult

// ReadReferenceDecision只返回源最小持久决定；未知、隔离与缺失都不能作为释放证据。
func ReadReferenceDecision(ctx context.Context, op OperationContext, input ReadReferenceDecisionInput) (ReadReferenceDecisionResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return ReadReferenceDecisionResult{}, platform.NewError(platform.CodeForbidden, false, "资源来源决定仅允许media纯服务读取")
	}
	runtime, err := draftRuntimeFromContext(ctx)
	if err != nil {
		return ReadReferenceDecisionResult{}, err
	}
	if err = runtime.Store.checkRevisionScope(ctx, op); err != nil {
		return ReadReferenceDecisionResult{}, err
	}
	tx, err := runtime.Store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ReadReferenceDecisionResult{}, contentStorageError()
	}
	defer tx.Rollback()
	var planRaw []byte
	var state string
	err = tx.QueryRowContext(ctx, `SELECT plan_json,state FROM draft_operation_intents WHERE site_id=? AND data_epoch=? AND operation_id=? AND action='save'`, op.Actor.SiteID, op.DataEpoch, input.SourceOperationId).Scan(&planRaw, &state)
	if errors.Is(err, sql.ErrNoRows) {
		var other int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM draft_operation_intents WHERE site_id=? AND operation_id=? AND action='save' AND data_epoch<>?`, op.Actor.SiteID, input.SourceOperationId, op.DataEpoch).Scan(&other); err != nil {
			return ReadReferenceDecisionResult{}, contentStorageError()
		}
		if other > 0 {
			return ReadReferenceDecisionResult{}, platform.NewError(platform.CodeVersionMismatch, false, "资源来源决定属于其他数据代次")
		}
		return ReadReferenceDecisionResult{}, platform.NewError(platform.CodeNotFound, false, "资源来源意图不存在")
	}
	if err != nil {
		return ReadReferenceDecisionResult{}, contentStorageError()
	}
	var plan draftOperationPlan
	if decodeStorageJSON(planRaw, &plan) != nil {
		return ReadReferenceDecisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源来源固定材料损坏")
	}
	if err = validateDraftOperationPlan("save", plan); err != nil {
		return ReadReferenceDecisionResult{}, err
	}
	var committedCount int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM committed_revisions WHERE site_id=? AND data_epoch=? AND document_id=? AND revision_id=?`, op.Actor.SiteID, op.DataEpoch, plan.DocumentID, plan.RevisionID).Scan(&committedCount); err != nil {
		return ReadReferenceDecisionResult{}, contentStorageError()
	}
	var committedRaw, receiptRaw []byte
	// 资格行一旦存在，即使正文后来隔离仍是COMMITTED来源，不允许释放历史资源。
	err = tx.QueryRowContext(ctx, `SELECT p.result_json,r.result_json FROM revision_preparations p JOIN committed_revisions c ON c.commit_id=p.commit_id AND c.site_id=p.site_id AND c.document_id=p.document_id AND c.revision_id=p.revision_id AND c.data_epoch=p.data_epoch JOIN draft_commit_receipts r ON r.commit_id=c.commit_id JOIN draft_resource_references f ON f.site_id=c.site_id AND f.document_id=c.document_id AND f.revision_id=c.revision_id AND f.source_version=? WHERE p.site_id=? AND p.data_epoch=? AND p.operation_id=? AND p.document_id=? AND p.revision_id=?`, plan.SourceVersion, op.Actor.SiteID, op.DataEpoch, input.SourceOperationId, plan.DocumentID, plan.RevisionID).Scan(&committedRaw, &receiptRaw)
	if err == nil {
		var prepared WriteRevisionFileResult
		var receipt CommitDraftJournalResult
		if decodeStorageJSON(committedRaw, &prepared) != nil || decodeStorageJSON(receiptRaw, &receipt) != nil || !sameStoredDraft(prepared.Draft, receipt.Draft) || receipt.Draft.BodyHash != plan.BodyHash || receipt.Draft.DraftVersion != plan.ExpectedDraftVersion+1 {
			return ReadReferenceDecisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源来源COMMITTED资格或回执矛盾")
		}
		state = "committed"
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ReadReferenceDecisionResult{}, contentStorageError()
	} else if state == "committed" || committedCount != 0 {
		return ReadReferenceDecisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源来源已提交决定缺少正文资格")
	}
	if state != "pending" && state != "committed" && state != "aborted" && state != "rejected" {
		return ReadReferenceDecisionResult{}, platform.NewError(platform.CodeIntegrityError, false, "资源来源决定未知")
	}
	// 保持JSON可见字段与公共ReferenceSourceDecision完全相同。
	result := ReadReferenceDecisionResult{State: gen.ReferenceSourceDecisionState(state), SiteId: op.Actor.SiteID, SourceOperationId: input.SourceOperationId, SourceObjectId: plan.DocumentID, SourceRevisionId: plan.RevisionID, SourceVersion: plan.SourceVersion, ReferenceDigest: plan.ReferenceDigest, DataEpoch: op.DataEpoch}
	return result, nil
}
