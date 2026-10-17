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
	runtime, err := referenceRuntimeFromContext(ctx)
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
	source := referenceSourceRecord{operation: input.SourceOperationId, action: "save", state: state, planRaw: planRaw}
	_, state, err = readCommittedReferenceSource(ctx, tx, op, source, plan)
	if err != nil {
		return ReadReferenceDecisionResult{}, err
	}
	// 保持JSON可见字段与公共ReferenceSourceDecision完全相同。
	result := ReadReferenceDecisionResult{State: gen.ReferenceSourceDecisionState(state), SiteId: op.Actor.SiteID, SourceOperationId: input.SourceOperationId, SourceObjectId: plan.DocumentID, SourceRevisionId: plan.RevisionID, SourceVersion: plan.SourceVersion, ReferenceDigest: plan.ReferenceDigest, DataEpoch: op.DataEpoch}
	return result, nil
}
