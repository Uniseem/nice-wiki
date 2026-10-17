package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/content/internal/gen"
)

type UpdateDocumentMetadataInput struct {
	DocumentId       Identifier           `json:"documentId"`
	Title            string               `json:"title"`
	PathSegment      *string              `json:"pathSegment"`
	SeoDescription   *string              `json:"seoDescription"`
	ShareImage       *gen.AssetVersionRef `json:"shareImage"`
	ExpectedRevision int                  `json:"expectedRevision"`
}
type UpdateDocumentMetadataResult = gen.UpdateDocumentMetadataResult

// UpdateDocumentMetadata只提交已明确的编辑分支；manifest/资源引用变更未知时失败关闭。
func UpdateDocumentMetadata(ctx context.Context, op OperationContext, input UpdateDocumentMetadataInput) (UpdateDocumentMetadataResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return UpdateDocumentMetadataResult{}, err
	}
	if err = authorizeTree(ctx, op, runtime, "content.edit", "updateDocumentMetadata"); err != nil {
		return UpdateDocumentMetadataResult{}, err
	}
	if err = runtime.Reference.Store.checkRevisionScope(ctx, op); err != nil {
		return UpdateDocumentMetadataResult{}, err
	}
	digest, err := draftRequestDigest("metadata.update", input)
	if err != nil {
		return UpdateDocumentMetadataResult{}, err
	}
	if result, found, err := readMetadataReceipt(ctx, runtime.Reference.Store.db, op, digest, runtime); err != nil || found {
		return result, err
	}
	if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "元数据当前写窗口尚未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return UpdateDocumentMetadataResult{}, err
	}
	defer cancel()
	if view.DataEpoch != op.DataEpoch {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeVersionMismatch, false, "元数据写窗口代次已变化")
	}
	runtime.Reference.Store.mu.Lock()
	defer runtime.Reference.Store.mu.Unlock()
	tx, err := runtime.Reference.Store.db.BeginTx(work, nil)
	if err != nil {
		return UpdateDocumentMetadataResult{}, contentStorageError()
	}
	defer tx.Rollback()
	if result, found, err := readMetadataReceipt(work, tx, op, digest, runtime); err != nil || found {
		return result, err
	}
	treeRevision, documents, err := readTreeSnapshot(work, tx, op, runtime)
	if err != nil {
		return UpdateDocumentMetadataResult{}, err
	}
	var current *gen.DocumentMetadata
	for i := range documents {
		if documents[i].DocumentId == input.DocumentId {
			current = &documents[i]
			break
		}
	}
	if current == nil {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeNotFound, false, "元数据文档不存在")
	}
	if current.State != "draft" && current.State != "published" {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeLocked, false, "只能编辑活动文档元数据")
	}
	if current.Revision != input.ExpectedRevision || int64(current.Revision) >= maxEditFence || treeRevision >= maxEditFence {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeVersionMismatch, false, "元数据修订已变化或水位耗尽")
	}
	if !sameMetadataAsset(current.ShareImage, input.ShareImage) {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "分享图变更依赖精确metadata资源引用与pin生产链，当前未装配")
	}
	if !sameIdentifierPointer(current.PathSegment, input.PathSegment) {
		planned, err := PlanOldPathRedirects(PlanOldPathRedirectsInput{DocumentId: input.DocumentId, Documents: documents, TreeRevision: int(treeRevision + 1)})
		if err != nil {
			return UpdateDocumentMetadataResult{}, err
		}
		if len(planned.Redirects) != 0 {
			return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "曾发布路径更改依赖真实redirect manifest，当前未装配")
		}
	}
	if input.PathSegment != nil {
		if _, err = ValidatePathSegment(work, op, ValidatePathSegmentInput{Segment: *input.PathSegment, ParentId: current.ParentId, DocumentId: input.DocumentId, Documents: documents}); err != nil {
			return UpdateDocumentMetadataResult{}, err
		}
	}
	metadata := *current
	metadata.Title = input.Title
	metadata.PathSegment = input.PathSegment
	metadata.SeoDescription = input.SeoDescription
	metadata.SeoDescriptionSource = "pending"
	if input.SeoDescription != nil {
		metadata.SeoDescriptionSource = "manual"
	}
	metadata.Revision++
	updated, err := tx.ExecContext(work, `UPDATE content_tree_state SET revision=revision+1 WHERE site_id=? AND revision=?`, op.Actor.SiteID, treeRevision)
	if err != nil {
		return UpdateDocumentMetadataResult{}, contentStorageError()
	}
	count, err := updated.RowsAffected()
	if err != nil || count != 1 {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeVersionMismatch, false, "元数据树CAS失败")
	}
	updated, err = tx.ExecContext(work, `UPDATE documents SET revision=? WHERE site_id=? AND document_id=? AND revision=?`, metadata.Revision, op.Actor.SiteID, input.DocumentId, input.ExpectedRevision)
	if err != nil {
		return UpdateDocumentMetadataResult{}, contentStorageError()
	}
	count, err = updated.RowsAffected()
	if err != nil || count != 1 {
		return UpdateDocumentMetadataResult{}, platform.NewError(platform.CodeVersionMismatch, false, "元数据CAS失败")
	}
	raw, _ := json.Marshal(metadata)
	if _, err = tx.ExecContext(work, `UPDATE document_metadata SET metadata_json=? WHERE site_id=? AND document_id=?`, string(raw), op.Actor.SiteID, input.DocumentId); err != nil {
		return UpdateDocumentMetadataResult{}, contentStorageError()
	}
	result := UpdateDocumentMetadataResult{Metadata: metadata, Redirects: []gen.RouteTombstone{}}
	resultRaw, _ := json.Marshal(result)
	if _, err = tx.ExecContext(work, `INSERT INTO metadata_operation_receipts(site_id,data_epoch,operation_id,caller_service,actor_id,request_hash,result_json,result_hash) VALUES(?,?,?,?,?,?,?,?)`, op.Actor.SiteID, op.DataEpoch, op.OperationID, op.CallerService, op.Actor.SubjectID, digest, string(resultRaw), platform.HashBytes(resultRaw)); err != nil {
		return UpdateDocumentMetadataResult{}, contentStorageError()
	}
	payload, _ := json.Marshal(struct {
		SiteID             Identifier   `json:"siteId"`
		TreeRevision       int64        `json:"treeRevision"`
		ChangedDocumentIDs []Identifier `json:"changedDocumentIds"`
		DataEpoch          Identifier   `json:"dataEpoch"`
	}{op.Actor.SiteID, treeRevision + 1, []Identifier{input.DocumentId}, op.DataEpoch})
	if err = outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "content.tree.changed", EventVersion: 1, Producer: "content", SubjectID: op.Actor.SiteID, Revision: treeRevision + 1, OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
		return UpdateDocumentMetadataResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return UpdateDocumentMetadataResult{}, contentStorageError()
	}
	return result, nil
}

func readMetadataReceipt(ctx context.Context, q creationQuery, op OperationContext, digest Hash, runtime TreeRuntime) (UpdateDocumentMetadataResult, bool, error) {
	var caller, actor string
	var stored, resultHash Hash
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT caller_service,actor_id,request_hash,result_json,result_hash FROM metadata_operation_receipts WHERE site_id=? AND data_epoch=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&caller, &actor, &stored, &raw, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return UpdateDocumentMetadataResult{}, false, nil
	}
	if err != nil {
		return UpdateDocumentMetadataResult{}, false, contentStorageError()
	}
	if caller != op.CallerService || actor != op.Actor.SubjectID || stored != digest {
		return UpdateDocumentMetadataResult{}, false, platform.NewError(platform.CodeConflict, false, "元数据操作身份或摘要与原回执冲突")
	}
	var result UpdateDocumentMetadataResult
	if platform.HashBytes(raw) != resultHash || runtime.validateResult["metadata"](raw) != nil || decodeStorageJSON(raw, &result) != nil {
		return result, false, platform.NewError(platform.CodeIntegrityError, false, "元数据完整原回执损坏")
	}
	return result, true, nil
}
func sameMetadataAsset(a, b *gen.AssetVersionRef) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
