package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"unicode"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/content/internal/gen"
)

type PublishDocumentInput struct {
	DocumentId           Identifier `json:"documentId"`
	DraftRevisionId      Identifier `json:"draftRevisionId"`
	ExpectedDraftVersion int        `json:"expectedDraftVersion"`
	ExpectedRevision     int        `json:"expectedRevision"`
	ManualPathSegment    *string    `json:"manualPathSegment"`
	ManualSEO            *string    `json:"manualSEO"`
}

type PublishDocumentResult = gen.PublishDocumentResult

type WithdrawDocumentInput struct {
	DocumentId       Identifier `json:"documentId"`
	ExpectedRevision int        `json:"expectedRevision"`
}

type WithdrawDocumentResult = gen.WithdrawDocumentResult

// PublishDocument只公开当前共享草稿。原回执、元数据、快照、引用和outbox同一事务。
func PublishDocument(ctx context.Context, op OperationContext, input PublishDocumentInput) (PublishDocumentResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	account, err := authorizePublication(ctx, op, runtime, "publishDocument", input.DocumentId)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	digest, err := draftRequestDigest("publication.publish", input)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	if raw, found, err := readPublicationReceipt(ctx, runtime.Reference.Store.db, op, "publish", digest, runtime); err != nil || found {
		return decodePublishReceipt(raw, found, err)
	}
	if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return PublishDocumentResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "发布当前写窗口尚未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	defer cancel()
	if view.DataEpoch != op.DataEpoch {
		return PublishDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "发布写窗口代次已变化")
	}
	files := runtime.Reference.Store.ConfigureRevisionStorage(RevisionStorageConfig{})
	body, err := ReadRevisionBody(WithRevisionFileStore(work, files), op, ReadRevisionBodyInput{DocumentID: input.DocumentId, RevisionID: input.DraftRevisionId})
	if err != nil {
		return PublishDocumentResult{}, err
	}
	schema, err := publishedBodySchema(body.Body)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	referenceDigest, referencesPinned, err := preparePublicationPin(work, op, runtime, input.DocumentId, input.DraftRevisionId, input.ExpectedRevision, input.ManualPathSegment, input.ManualSEO)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	runtime.Reference.Store.mu.Lock()
	defer runtime.Reference.Store.mu.Unlock()
	tx, err := runtime.Reference.Store.db.BeginTx(work, nil)
	if err != nil {
		return PublishDocumentResult{}, contentStorageError()
	}
	defer tx.Rollback()
	if raw, found, err := readPublicationReceipt(work, tx, op, "publish", digest, runtime); err != nil || found {
		return decodePublishReceipt(raw, found, err)
	}
	treeRevision, documents, err := readTreeSnapshot(work, tx, op, runtime)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	current, err := publicationDocument(documents, input.DocumentId)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	if current.Revision != input.ExpectedRevision || int64(current.Revision) >= maxEditFence || treeRevision >= maxEditFence {
		return PublishDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "发布修订已变化或水位耗尽")
	}
	prepared, err := PreparePublicationMetadata(work, op, PreparePublicationMetadataInput{Metadata: *current, Body: body.Body, ManualPathSegment: input.ManualPathSegment, ManualSEO: input.ManualSEO})
	if err != nil {
		return PublishDocumentResult{}, err
	}
	if prepared.Metadata.PathSegment == nil {
		return PublishDocumentResult{}, platform.NewError(platform.CodeValidation, false, "发布缺少英文路径，请人工填写")
	}
	bound := bindPublicationTx(withPublicationFacts(work, publicationFacts{accountName: account, schemaVersion: schema, referenceDigest: referenceDigest, referencesPinned: referencesPinned}), tx)
	committed, err := CommitPublication(bound, op, CommitPublicationInput{Metadata: prepared.Metadata, Draft: DraftRevision{DocumentId: input.DocumentId, RevisionId: input.DraftRevisionId, DraftVersion: input.ExpectedDraftVersion, BodyHash: body.Object.Sha256}, ExpectedRevision: input.ExpectedRevision, Operation: string(gen.Publish)})
	if err != nil {
		return PublishDocumentResult{}, err
	}
	result := PublishDocumentResult{Reference: committed.Reference, History: committed.History, Warnings: prepared.Warnings}
	if err = insertPublicationReceipt(work, tx, op, "publish", digest, result); err != nil {
		return PublishDocumentResult{}, err
	}
	if err = tx.Commit(); err != nil {
		if raw, found, readErr := readPublicationReceipt(ctx, runtime.Reference.Store.db, op, "publish", digest, runtime); readErr != nil || found {
			return decodePublishReceipt(raw, found, readErr)
		}
		return PublishDocumentResult{}, contentStorageError()
	}
	return result, nil
}

// WithdrawDocument立即取消公开引用并提高失效水位，保留共享草稿和历史。
func WithdrawDocument(ctx context.Context, op OperationContext, input WithdrawDocumentInput) (WithdrawDocumentResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return WithdrawDocumentResult{}, err
	}
	if _, err = authorizePublication(ctx, op, runtime, "withdrawDocument", input.DocumentId); err != nil {
		return WithdrawDocumentResult{}, err
	}
	digest, err := draftRequestDigest("publication.withdraw", input)
	if err != nil {
		return WithdrawDocumentResult{}, err
	}
	if raw, found, err := readPublicationReceipt(ctx, runtime.Reference.Store.db, op, "withdraw", digest, runtime); err != nil || found {
		return decodeWithdrawReceipt(raw, found, err)
	}
	if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "发布当前写窗口尚未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return WithdrawDocumentResult{}, err
	}
	defer cancel()
	if view.DataEpoch != op.DataEpoch {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "撤回写窗口代次已变化")
	}
	runtime.Reference.Store.mu.Lock()
	defer runtime.Reference.Store.mu.Unlock()
	tx, err := runtime.Reference.Store.db.BeginTx(work, nil)
	if err != nil {
		return WithdrawDocumentResult{}, contentStorageError()
	}
	defer tx.Rollback()
	if raw, found, err := readPublicationReceipt(work, tx, op, "withdraw", digest, runtime); err != nil || found {
		return decodeWithdrawReceipt(raw, found, err)
	}
	treeRevision, documents, err := readTreeSnapshot(work, tx, op, runtime)
	if err != nil {
		return WithdrawDocumentResult{}, err
	}
	var current *gen.DocumentMetadata
	for i := range documents {
		if documents[i].DocumentId == input.DocumentId {
			current = &documents[i]
			break
		}
	}
	if current == nil {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeNotFound, false, "撤回文档不存在")
	}
	if current.State == gen.DocumentMetadataStateTrash || current.State == gen.DocumentMetadataStatePurged {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeLocked, false, "回收站或已清除文档不能撤回")
	}
	if current.State != gen.DocumentMetadataStatePublished || current.PublishedReference == nil {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeValidation, false, "只能撤回当前公开文档")
	}
	if current.PublishedReference.DocumentId != current.DocumentId {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeIntegrityError, false, "当前发布引用文档不匹配")
	}
	if current.Revision != input.ExpectedRevision || int64(current.Revision) >= maxEditFence || int64(current.VisibilityRevision) >= maxEditFence || treeRevision >= maxEditFence {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "撤回修订已变化或水位耗尽")
	}
	metadata := *current
	metadata.PublishedReference = nil
	metadata.State = gen.DocumentMetadataStateDraft
	metadata.Revision++
	metadata.VisibilityRevision++
	if err = casPublicationIdentity(work, tx, op, treeRevision, metadata, current.Revision, string(current.State)); err != nil {
		return WithdrawDocumentResult{}, err
	}
	if err = archivePublishedReferences(work, tx, op, metadata.DocumentId); err != nil {
		return WithdrawDocumentResult{}, err
	}
	refs, sourceVersion, err := listPublicationReferences(work, tx, op, metadata.DocumentId)
	if err != nil {
		return WithdrawDocumentResult{}, err
	}
	bound := bindPublicationTx(work, tx)
	occurred := platform.Now()
	withdrawn := publicationWithdrawnPayload{SiteID: op.Actor.SiteID, DocumentID: metadata.DocumentId, Revision: metadata.Revision, VisibilityRevision: metadata.VisibilityRevision, DataEpoch: op.DataEpoch, Reason: "withdrawn", OccurredAt: occurred}
	if err = enqueuePublication(bound, op, metadata.VisibilityRevision, nil, &withdrawn, metadata.DocumentId, sourceVersion, refs); err != nil {
		return WithdrawDocumentResult{}, err
	}
	if err = enqueueTreeChanged(bound, tx, op, treeRevision+1, []Identifier{metadata.DocumentId}); err != nil {
		return WithdrawDocumentResult{}, err
	}
	result := WithdrawDocumentResult{Metadata: metadata, VisibilityRevision: metadata.VisibilityRevision}
	if err = insertPublicationReceipt(work, tx, op, "withdraw", digest, result); err != nil {
		return WithdrawDocumentResult{}, err
	}
	if err = tx.Commit(); err != nil {
		if raw, found, readErr := readPublicationReceipt(ctx, runtime.Reference.Store.db, op, "withdraw", digest, runtime); readErr != nil || found {
			return decodeWithdrawReceipt(raw, found, readErr)
		}
		return WithdrawDocumentResult{}, contentStorageError()
	}
	return result, nil
}

func authorizePublication(ctx context.Context, op OperationContext, runtime TreeRuntime, action string, documentID Identifier) (string, error) {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" {
		return "", platform.NewError(platform.CodeForbidden, false, "发布仅允许当前管理账户")
	}
	if runtime.Reference.Identity == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "发布授权依赖未装配")
	}
	raw, err := runtime.Reference.Identity.CheckAuthorization(platform.WithOperationContext(ctx, op), map[string]any{
		"subjectId": op.Actor.SubjectID, "sessionId": op.Actor.SessionID, "siteId": op.Actor.SiteID,
		"permission": "content.publish", "resourceId": nil, "expectedPermissionRevision": op.Actor.PermissionRevision,
		"action": "content:" + action,
	})
	if err != nil {
		return "", err
	}
	var decision struct {
		Authorized  *bool                `json:"authorized"`
		AccountName string               `json:"accountName"`
		Auth        platform.AuthContext `json:"auth"`
	}
	if json.Unmarshal(raw, &decision) != nil || decision.Authorized == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "发布授权响应不可用")
	}
	if !*decision.Authorized {
		return "", platform.NewError(platform.CodeForbidden, false, "没有发布操作权限")
	}
	if decision.Auth.SubjectID != op.Actor.SubjectID || decision.Auth.SiteID != op.Actor.SiteID || decision.Auth.SessionID != op.Actor.SessionID || decision.Auth.SessionRevision != op.Actor.SessionRevision || decision.Auth.PermissionRevision != op.Actor.PermissionRevision {
		return "", platform.NewError(platform.CodeUnauthenticated, false, "发布授权主体或版本已变化")
	}
	if !utf8.ValidString(decision.AccountName) || utf8.RuneCountInString(decision.AccountName) < 1 || utf8.RuneCountInString(decision.AccountName) > 64 {
		return "", platform.NewError(platform.CodeIntegrityError, false, "发布账户显示名不合法")
	}
	for _, r := range decision.AccountName {
		if unicode.IsControl(r) {
			return "", platform.NewError(platform.CodeIntegrityError, false, "发布账户显示名含控制字符")
		}
	}
	return decision.AccountName, nil
}

func readPublicationReceipt(ctx context.Context, q creationQuery, op OperationContext, action string, digest Hash, runtime TreeRuntime) (json.RawMessage, bool, error) {
	var caller, actor, storedAction string
	var stored, resultHash Hash
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT caller_service,actor_id,action,request_hash,result_json,result_hash FROM publication_operation_receipts WHERE site_id=? AND data_epoch=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&caller, &actor, &storedAction, &stored, &raw, &resultHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, contentStorageError()
	}
	if caller != op.CallerService || actor != op.Actor.SubjectID || storedAction != action || stored != digest {
		return nil, false, platform.NewError(platform.CodeConflict, false, "发布操作身份、动作或摘要与原回执冲突")
	}
	if platform.HashBytes(raw) != resultHash {
		return nil, false, platform.NewError(platform.CodeIntegrityError, false, "发布原回执哈希损坏")
	}
	if action == "commit" {
		if decodeStorageJSON(raw, &CommitPublicationResult{}) != nil {
			return nil, false, platform.NewError(platform.CodeIntegrityError, false, "发布提交原回执损坏")
		}
		return raw, true, nil
	}
	if runtime.validateResult[action] == nil || runtime.validateResult[action](raw) != nil {
		return nil, false, platform.NewError(platform.CodeIntegrityError, false, "发布原回执不符合唯一schema")
	}
	return raw, true, nil
}

func insertPublicationReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, action string, digest Hash, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "发布原回执无法编码")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO publication_operation_receipts(site_id,data_epoch,operation_id,caller_service,actor_id,action,request_hash,result_json,result_hash) VALUES(?,?,?,?,?,?,?,?,?)`, op.Actor.SiteID, op.DataEpoch, op.OperationID, op.CallerService, op.Actor.SubjectID, action, digest, string(raw), platform.HashBytes(raw))
	if err != nil {
		return contentStorageError()
	}
	return nil
}

func decodePublishReceipt(raw json.RawMessage, found bool, err error) (PublishDocumentResult, error) {
	if err != nil || !found {
		return PublishDocumentResult{}, err
	}
	var result PublishDocumentResult
	if decodeStorageJSON(raw, &result) != nil {
		return PublishDocumentResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布原回执损坏")
	}
	return result, nil
}

func decodeWithdrawReceipt(raw json.RawMessage, found bool, err error) (WithdrawDocumentResult, error) {
	if err != nil || !found {
		return WithdrawDocumentResult{}, err
	}
	var result WithdrawDocumentResult
	if decodeStorageJSON(raw, &result) != nil {
		return WithdrawDocumentResult{}, platform.NewError(platform.CodeIntegrityError, false, "撤回原回执损坏")
	}
	return result, nil
}
