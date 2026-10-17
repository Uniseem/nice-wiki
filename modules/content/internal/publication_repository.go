package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
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

type CommitPublicationInput struct {
	Metadata         gen.DocumentMetadata `json:"metadata"`
	Draft            DraftRevision        `json:"draft"`
	ExpectedRevision gen.Revision         `json:"expectedRevision"`
	Operation        string               `json:"operation"`
}

type CommitPublicationResult struct {
	Reference gen.PublishedRevisionRef `json:"reference"`
	History   gen.HistoryEntry         `json:"history"`
}

type publicationFacts struct {
	accountName      string
	schemaVersion    int
	referenceDigest  Hash
	referencesPinned bool
}

type publicationFactsKey struct{}

func withPublicationFacts(ctx context.Context, facts publicationFacts) context.Context {
	return context.WithValue(ctx, publicationFactsKey{}, facts)
}

func publicationFactsFrom(ctx context.Context) (publicationFacts, bool) {
	facts, ok := ctx.Value(publicationFactsKey{}).(publicationFacts)
	return facts, ok && facts.accountName != "" && facts.schemaVersion == 1
}

// CommitPublication把当前共享草稿提交为不可变发布快照。
// 已绑定业务事务时只写入该事务；独立调用自己完成回执、CAS和提交。
func CommitPublication(ctx context.Context, op OperationContext, input CommitPublicationInput) (CommitPublicationResult, error) {
	if input.Operation == string(gen.Rollback) {
		return CommitPublicationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "历史回滚尚未装配")
	}
	if tx, ok := publicationTx(ctx); ok {
		published, err := applyPublication(ctx, tx, op, input)
		if err != nil {
			return CommitPublicationResult{}, err
		}
		return CommitPublicationResult{Reference: published.Reference, History: published.History}, nil
	}
	return commitPublicationAlone(ctx, op, input)
}

func commitPublicationAlone(ctx context.Context, op OperationContext, input CommitPublicationInput) (CommitPublicationResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return CommitPublicationResult{}, err
	}
	account, err := authorizePublication(ctx, op, runtime, "publishDocument", input.Draft.DocumentId)
	if err != nil {
		return CommitPublicationResult{}, err
	}
	digest, err := draftRequestDigest("publication.commit", input)
	if err != nil {
		return CommitPublicationResult{}, err
	}
	if raw, found, err := readPublicationReceipt(ctx, runtime.Reference.Store.db, op, "commit", digest, runtime); err != nil || found {
		return decodeCommitReceipt(raw, found, err)
	}
	if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return CommitPublicationResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "发布当前写窗口尚未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return CommitPublicationResult{}, err
	}
	defer cancel()
	if view.DataEpoch != op.DataEpoch {
		return CommitPublicationResult{}, platform.NewError(platform.CodeVersionMismatch, false, "发布写窗口代次已变化")
	}
	schema, err := readPublishedBodySchema(work, op, input.Draft)
	if err != nil {
		return CommitPublicationResult{}, err
	}
	referenceDigest, referencesPinned, err := preparePublicationPin(work, op, runtime, input.Draft.DocumentId, input.Draft.RevisionId, input.ExpectedRevision, input.Metadata.PathSegment, input.Metadata.SeoDescription)
	if err != nil {
		return CommitPublicationResult{}, err
	}
	runtime.Reference.Store.mu.Lock()
	defer runtime.Reference.Store.mu.Unlock()
	tx, err := runtime.Reference.Store.db.BeginTx(work, nil)
	if err != nil {
		return CommitPublicationResult{}, contentStorageError()
	}
	defer tx.Rollback()
	if raw, found, err := readPublicationReceipt(work, tx, op, "commit", digest, runtime); err != nil || found {
		return decodeCommitReceipt(raw, found, err)
	}
	bound := bindPublicationTx(withPublicationFacts(work, publicationFacts{accountName: account, schemaVersion: schema, referenceDigest: referenceDigest, referencesPinned: referencesPinned}), tx)
	published, err := applyPublication(bound, tx, op, input)
	if err != nil {
		return CommitPublicationResult{}, err
	}
	result := CommitPublicationResult{Reference: published.Reference, History: published.History}
	if err = insertPublicationReceipt(work, tx, op, "commit", digest, result); err != nil {
		return CommitPublicationResult{}, err
	}
	if err = tx.Commit(); err != nil {
		if raw, found, readErr := readPublicationReceipt(ctx, runtime.Reference.Store.db, op, "commit", digest, runtime); readErr != nil || found {
			return decodeCommitReceipt(raw, found, readErr)
		}
		return CommitPublicationResult{}, contentStorageError()
	}
	return result, nil
}

func readPublishedBodySchema(ctx context.Context, op OperationContext, draft DraftRevision) (int, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return 0, err
	}
	files := runtime.Reference.Store.ConfigureRevisionStorage(RevisionStorageConfig{})
	body, err := ReadRevisionBody(WithRevisionFileStore(ctx, files), op, ReadRevisionBodyInput{DocumentID: draft.DocumentId, RevisionID: draft.RevisionId, ExpectedHash: &draft.BodyHash})
	if err != nil {
		return 0, err
	}
	if body.Object.Sha256 != draft.BodyHash {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "发布正文哈希与共享草稿不一致")
	}
	return publishedBodySchema(body.Body)
}

func decodeCommitReceipt(raw json.RawMessage, found bool, err error) (CommitPublicationResult, error) {
	if err != nil || !found {
		return CommitPublicationResult{}, err
	}
	var result CommitPublicationResult
	if decodeStorageJSON(raw, &result) != nil {
		return CommitPublicationResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布提交原回执损坏")
	}
	return result, nil
}

func applyPublication(ctx context.Context, tx *sql.Tx, op OperationContext, input CommitPublicationInput) (PublishDocumentResult, error) {
	facts, ok := publicationFactsFrom(ctx)
	if !ok {
		return PublishDocumentResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布提交缺少已核对的账户或正文版本")
	}
	if input.Draft.DocumentId != input.Metadata.DocumentId || !platform.ValidHash(input.Draft.BodyHash) {
		return PublishDocumentResult{}, platform.NewError(platform.CodeIntegrityError, false, "发布草稿与元数据文档不一致")
	}
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	treeRevision, documents, err := readTreeSnapshot(ctx, tx, op, runtime)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	current, err := publicationDocument(documents, input.Metadata.DocumentId)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	if current.Revision != input.ExpectedRevision || int64(current.Revision) >= maxEditFence || int64(current.VisibilityRevision) >= maxEditFence || treeRevision >= maxEditFence {
		return PublishDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "发布修订已变化或水位耗尽")
	}
	if current.ShareImage != nil {
		return PublishDocumentResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "分享图发布依赖精确metadata资源引用与pin生产链，当前未装配")
	}
	draft, referenceVersion, err := loadSharedDraft(ctx, tx, op, current.DocumentId)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	if draft.RevisionId != input.Draft.RevisionId || draft.DraftVersion != input.Draft.DraftVersion || draft.BodyHash != input.Draft.BodyHash || current.DraftRevisionId != input.Draft.RevisionId {
		return PublishDocumentResult{}, platform.NewError(platform.CodeVersionMismatch, false, "只能发布当前共享草稿")
	}
	metadata := *current
	metadata.PathSegment = input.Metadata.PathSegment
	metadata.SeoDescription = input.Metadata.SeoDescription
	metadata.SeoDescriptionSource = input.Metadata.SeoDescriptionSource
	if metadata.PathSegment == nil {
		return PublishDocumentResult{}, platform.NewError(platform.CodeValidation, false, "发布缺少英文路径，请人工填写")
	}
	if _, err = ValidatePathSegment(ctx, op, ValidatePathSegmentInput{Segment: *metadata.PathSegment, ParentId: metadata.ParentId, DocumentId: metadata.DocumentId, Documents: documents}); err != nil {
		return PublishDocumentResult{}, err
	}
	var redirects []gen.RouteTombstone
	if !sameIdentifierPointer(current.PathSegment, metadata.PathSegment) && publicationEverPublic(*current) {
		planned, planErr := PlanOldPathRedirects(PlanOldPathRedirectsInput{DocumentId: metadata.DocumentId, Documents: documents, TreeRevision: int(treeRevision + 1)})
		if planErr != nil {
			return PublishDocumentResult{}, planErr
		}
		redirects = planned.Redirects
	}
	publishedAt := time.Now().UTC()
	metadata.Revision++
	metadata.VisibilityRevision++
	metadata.State = gen.DocumentMetadataStatePublished
	metadata.LastPublishedAt = &publishedAt
	reference := gen.PublishedRevisionRef{DocumentId: metadata.DocumentId, PublishedRevisionId: platform.NewIdentifier(), Revision: metadata.Revision, ContentHash: draft.BodyHash, SchemaVersion: facts.schemaVersion, DataEpoch: op.DataEpoch}
	metadata.PublishedReference = &reference
	if err = casPublicationIdentity(ctx, tx, op, treeRevision, metadata, current.Revision, string(current.State)); err != nil {
		return PublishDocumentResult{}, err
	}
	if err = insertPublicationSnapshot(ctx, tx, op, reference, draft.RevisionId, metadata.VisibilityRevision); err != nil {
		return PublishDocumentResult{}, err
	}
	history := gen.HistoryEntry{HistoryId: platform.NewIdentifier(), AuthorAccountId: op.Actor.SubjectID, AuthorAccountName: facts.accountName, CreatedAt: publishedAt, CommitMirrorState: gen.HistoryEntryCommitMirrorStatePending, Operation: gen.Publish, Reference: reference}
	if err = insertPublicationHistory(ctx, tx, op, metadata.DocumentId, history); err != nil {
		return PublishDocumentResult{}, err
	}
	_, sourceVersion, err := copyPublishedReferences(ctx, tx, op, metadata.DocumentId, draft.RevisionId, reference.PublishedRevisionId, referenceVersion)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	next := slices.Clone(documents)
	for i := range next {
		if next[i].DocumentId == metadata.DocumentId {
			next[i] = metadata
		}
	}
	if len(redirects) > 0 {
		if _, err = commitRedirectManifest(ctx, tx, op, redirects, next); err != nil {
			return PublishDocumentResult{}, err
		}
	}
	listed, _, err := listPublicationReferences(ctx, tx, op, metadata.DocumentId)
	if err != nil {
		return PublishDocumentResult{}, err
	}
	occurred := platform.Timestamp(publishedAt.Format(time.RFC3339Nano))
	changed := publicationChangedPayload{SiteID: op.Actor.SiteID, Reference: reference, VisibilityRevision: metadata.VisibilityRevision, AuthorAccountID: op.Actor.SubjectID, OccurredAt: occurred}
	if err = enqueuePublication(ctx, op, metadata.VisibilityRevision, &changed, nil, metadata.DocumentId, sourceVersion, listed); err != nil {
		return PublishDocumentResult{}, err
	}
	if err = enqueueTreeChanged(ctx, tx, op, treeRevision+1, []Identifier{metadata.DocumentId}); err != nil {
		return PublishDocumentResult{}, err
	}
	return PublishDocumentResult{Reference: reference, History: history, Warnings: []string{}}, nil
}

func publicationDocument(documents []gen.DocumentMetadata, documentID Identifier) (*gen.DocumentMetadata, error) {
	for i := range documents {
		if documents[i].DocumentId != documentID {
			continue
		}
		if documents[i].State == gen.DocumentMetadataStateTrash || documents[i].State == gen.DocumentMetadataStatePurged {
			return nil, platform.NewError(platform.CodeLocked, false, "回收站或已清除文档不能发布")
		}
		if documents[i].State != gen.DocumentMetadataStateDraft && documents[i].State != gen.DocumentMetadataStatePublished {
			return nil, platform.NewError(platform.CodeLocked, false, "只能发布活动文档")
		}
		return &documents[i], nil
	}
	return nil, platform.NewError(platform.CodeNotFound, false, "发布文档不存在")
}

func publicationEverPublic(document gen.DocumentMetadata) bool {
	return document.PublishedReference != nil || document.LastPublishedAt != nil || document.State == gen.DocumentMetadataStatePublished || document.VisibilityRevision > 0
}

func loadSharedDraft(ctx context.Context, tx *sql.Tx, op OperationContext, documentID Identifier) (DraftRevision, int64, error) {
	var raw []byte
	var referenceVersion int64
	err := tx.QueryRowContext(ctx, `SELECT r.result_json,d.reference_version FROM draft_heads h JOIN documents d ON d.site_id=h.site_id AND d.document_id=h.document_id JOIN committed_revisions c ON c.site_id=h.site_id AND c.document_id=h.document_id AND c.revision_id=h.revision_id AND c.data_epoch=? AND c.state='committed' JOIN draft_commit_receipts r ON r.commit_id=c.commit_id WHERE h.site_id=? AND h.document_id=?`, op.DataEpoch, op.Actor.SiteID, documentID).Scan(&raw, &referenceVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return DraftRevision{}, 0, platform.NewError(platform.CodeIntegrityError, false, "当前共享草稿缺少已提交正文")
	}
	if err != nil {
		return DraftRevision{}, 0, contentStorageError()
	}
	var stored CommitDraftJournalResult
	if decodeStorageJSON(raw, &stored) != nil || stored.Draft.DocumentId != documentID || stored.Draft.RevisionId == "" || !platform.ValidHash(stored.Draft.BodyHash) {
		return DraftRevision{}, 0, platform.NewError(platform.CodeIntegrityError, false, "当前共享草稿回执损坏")
	}
	return stored.Draft, referenceVersion, nil
}

func casPublicationIdentity(ctx context.Context, tx *sql.Tx, op OperationContext, treeRevision int64, metadata gen.DocumentMetadata, expectedRevision int, previousState string) error {
	binding, err := tx.ExecContext(ctx, `UPDATE store_identity SET site_id=? WHERE singleton=1 AND data_epoch=? AND (site_id IS NULL OR site_id=?)`, op.Actor.SiteID, op.DataEpoch, op.Actor.SiteID)
	if err != nil {
		return contentStorageError()
	}
	if err = onePublicationRow(binding, "发布站点或代次已变化"); err != nil {
		return err
	}
	updated, err := tx.ExecContext(ctx, `INSERT INTO content_tree_state(site_id,revision) VALUES(?,1) ON CONFLICT(site_id) DO UPDATE SET revision=revision+1 WHERE revision=?`, op.Actor.SiteID, treeRevision)
	if err != nil {
		return contentStorageError()
	}
	if err = onePublicationRow(updated, "发布树CAS失败"); err != nil {
		return err
	}
	updated, err = tx.ExecContext(ctx, `UPDATE documents SET state=?,revision=? WHERE site_id=? AND document_id=? AND revision=? AND state=?`, string(metadata.State), metadata.Revision, op.Actor.SiteID, metadata.DocumentId, expectedRevision, previousState)
	if err != nil {
		return contentStorageError()
	}
	if err = onePublicationRow(updated, "发布文档CAS失败"); err != nil {
		return err
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "发布元数据无法编码")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE document_metadata SET metadata_json=? WHERE site_id=? AND document_id=?`, string(raw), op.Actor.SiteID, metadata.DocumentId); err != nil {
		return contentStorageError()
	}
	return nil
}

func onePublicationRow(result sql.Result, mismatch string) error {
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return platform.NewError(platform.CodeVersionMismatch, false, mismatch)
	}
	return nil
}

func insertPublicationSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, reference gen.PublishedRevisionRef, draftRevisionID Identifier, visibility int) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO publication_snapshots(site_id,published_revision_id,document_id,draft_revision_id,content_hash,schema_version,revision,visibility_revision,data_epoch) VALUES(?,?,?,?,?,?,?,?,?)`, op.Actor.SiteID, reference.PublishedRevisionId, reference.DocumentId, draftRevisionID, reference.ContentHash, reference.SchemaVersion, reference.Revision, visibility, reference.DataEpoch)
	if err != nil {
		return contentStorageError()
	}
	return nil
}

func insertPublicationHistory(ctx context.Context, tx *sql.Tx, op OperationContext, documentID Identifier, history gen.HistoryEntry) error {
	raw, err := json.Marshal(history.Reference)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "发布历史引用无法编码")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO publication_history(site_id,history_id,document_id,published_revision_id,author_account_id,author_account_name,created_at,commit_mirror_state,git_commit_id,operation,reference_json) VALUES(?,?,?,?,?,?,?,?,NULL,?,?)`, op.Actor.SiteID, history.HistoryId, documentID, history.Reference.PublishedRevisionId, history.AuthorAccountId, history.AuthorAccountName, history.CreatedAt.UTC().Format(time.RFC3339Nano), string(history.CommitMirrorState), string(history.Operation), string(raw))
	if err != nil {
		return contentStorageError()
	}
	return nil
}

// preparePublicationPin在本地SQL锁之外确认非空引用。空集合不调用媒体，也不造空pin。
func preparePublicationPin(ctx context.Context, op OperationContext, runtime TreeRuntime, documentID, revisionID Identifier, expectedRevision int, manualPath, manualSEO *string) (Hash, bool, error) {
	metadata, refs, sourceVersion, err := readPublicationPinMaterial(ctx, runtime.Reference.Store.db, op, documentID, revisionID)
	if err != nil {
		return "", false, err
	}
	if _, err = publicationDocument([]gen.DocumentMetadata{metadata}, documentID); err != nil {
		return "", false, err
	}
	if metadata.Revision != expectedRevision || int64(metadata.Revision) >= maxEditFence || int64(metadata.VisibilityRevision) >= maxEditFence {
		return "", false, platform.NewError(platform.CodeVersionMismatch, false, "发布修订已变化或水位耗尽")
	}
	if metadata.ShareImage != nil {
		return "", false, platform.NewError(platform.CodeDependencyUnavailable, true, "分享图发布依赖精确metadata资源引用与pin生产链，当前未装配")
	}
	prepared, err := PreparePublicationMetadata(ctx, op, PreparePublicationMetadataInput{Metadata: metadata, ManualPathSegment: manualPath, ManualSEO: manualSEO})
	if err != nil {
		return "", false, err
	}
	if prepared.Metadata.PathSegment == nil {
		return "", false, platform.NewError(platform.CodeValidation, false, "发布缺少英文路径，请人工填写")
	}
	return pinPublicationReferences(ctx, op, runtime, documentID, revisionID, sourceVersion, refs)
}

func readPublicationPinMaterial(ctx context.Context, db *sql.DB, op OperationContext, documentID, revisionID Identifier) (gen.DocumentMetadata, []gen.ResourceReference, int, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gen.DocumentMetadata{}, nil, 0, contentStorageError()
	}
	defer tx.Rollback()
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(metadata_json AS BLOB))<=16777216 THEN metadata_json ELSE NULL END FROM document_metadata WHERE site_id=? AND document_id=?`, op.Actor.SiteID, documentID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || raw == nil {
		return gen.DocumentMetadata{}, nil, 0, platform.NewError(platform.CodeNotFound, false, "发布文档不存在")
	}
	if err != nil {
		return gen.DocumentMetadata{}, nil, 0, contentStorageError()
	}
	var metadata gen.DocumentMetadata
	if decodeStorageJSON(raw, &metadata) != nil || metadata.SiteId != op.Actor.SiteID || metadata.DocumentId != documentID {
		return gen.DocumentMetadata{}, nil, 0, platform.NewError(platform.CodeIntegrityError, false, "发布元数据与文档身份不一致")
	}
	refs, sourceVersion, err := loadCurrentDraftReferences(ctx, tx, op, documentID, revisionID)
	if err != nil {
		return gen.DocumentMetadata{}, nil, 0, err
	}
	return metadata, refs, sourceVersion, nil
}

func loadCurrentDraftReferences(ctx context.Context, tx *sql.Tx, op OperationContext, documentID, revisionID Identifier) ([]gen.ResourceReference, int, error) {
	var raw []byte
	var sourceVersion int
	err := tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(references_json AS BLOB))<=16777216 THEN references_json ELSE NULL END,source_version FROM draft_resource_references WHERE site_id=? AND document_id=? AND revision_id=? AND kind='current_draft'`, op.Actor.SiteID, documentID, revisionID).Scan(&raw, &sourceVersion)
	if errors.Is(err, sql.ErrNoRows) || raw == nil {
		return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "当前共享草稿缺少就绪资源引用")
	}
	if err != nil {
		return nil, 0, contentStorageError()
	}
	var refs []gen.ResourceReference
	if decodeStorageJSON(raw, &refs) != nil {
		return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "草稿资源引用损坏")
	}
	if refs == nil {
		refs = []gen.ResourceReference{}
	}
	return refs, sourceVersion, nil
}

// loadCommittedDraftSaveOperation只读取已经committed的草稿保存来源，不另造决定。
func loadCommittedDraftSaveOperation(ctx context.Context, db *sql.DB, op OperationContext, documentID, revisionID Identifier, sourceVersion int, digest Hash) (Identifier, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", contentStorageError()
	}
	defer tx.Rollback()
	var operationID Identifier
	var planRaw []byte
	err = tx.QueryRowContext(ctx, `SELECT i.operation_id,CASE WHEN length(CAST(i.plan_json AS BLOB))<=16777216 THEN i.plan_json ELSE NULL END FROM revision_preparations p JOIN committed_revisions c ON c.commit_id=p.commit_id AND c.site_id=p.site_id AND c.document_id=p.document_id AND c.revision_id=p.revision_id AND c.data_epoch=p.data_epoch AND c.state='committed' JOIN draft_commit_receipts r ON r.commit_id=c.commit_id AND r.site_id=c.site_id AND r.data_epoch=c.data_epoch AND r.operation_id=p.operation_id JOIN draft_operation_intents i ON i.site_id=p.site_id AND i.data_epoch=p.data_epoch AND i.operation_id=p.operation_id AND i.action='save' AND i.state='committed' WHERE p.site_id=? AND p.data_epoch=? AND p.document_id=? AND p.revision_id=? AND p.state='committed'`, op.Actor.SiteID, op.DataEpoch, documentID, revisionID).Scan(&operationID, &planRaw)
	if errors.Is(err, sql.ErrNoRows) {
		var other int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM revision_preparations p JOIN draft_operation_intents i ON i.site_id=p.site_id AND i.data_epoch=p.data_epoch AND i.operation_id=p.operation_id AND i.action='save' WHERE p.site_id=? AND p.document_id=? AND p.revision_id=? AND p.data_epoch<>?`, op.Actor.SiteID, documentID, revisionID, op.DataEpoch).Scan(&other); err != nil {
			return "", contentStorageError()
		}
		if other > 0 {
			return "", platform.NewError(platform.CodeVersionMismatch, false, "已提交草稿决定属于其他数据代次")
		}
		return "", platform.NewError(platform.CodeIntegrityError, false, "当前共享草稿缺少已提交的资源决定")
	}
	if err != nil {
		return "", contentStorageError()
	}
	if planRaw == nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "已提交草稿决定损坏")
	}
	var plan draftOperationPlan
	if decodeStorageJSON(planRaw, &plan) != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "已提交草稿决定损坏")
	}
	if err = validateDraftOperationPlan("save", plan); err != nil {
		return "", err
	}
	if plan.DocumentID != documentID || plan.RevisionID != revisionID || plan.SourceVersion != sourceVersion || plan.ReferenceDigest != digest || !platform.ValidIdentifier(operationID) {
		return "", platform.NewError(platform.CodeIntegrityError, false, "发布资源引用与已提交草稿决定不一致")
	}
	return operationID, nil
}

// pinPublicationReferences只对非空就绪引用调用现有媒体客户端。失败则发布尚未开始。
func pinPublicationReferences(ctx context.Context, op OperationContext, runtime TreeRuntime, documentID, revisionID Identifier, sourceVersion int, refs []gen.ResourceReference) (Hash, bool, error) {
	if len(refs) == 0 {
		return "", false, nil
	}
	for _, ref := range refs {
		if ref.DocumentId != documentID || ref.SourceRevisionId != revisionID || ref.SourceVersion != sourceVersion || ref.Kind != gen.CurrentDraft || !ref.Retained {
			return "", false, platform.NewError(platform.CodeIntegrityError, false, "发布引用来源身份不一致")
		}
	}
	mediaRefs, digest, err := draftMediaReferences(refs)
	if err != nil {
		return "", false, err
	}
	sourceOperationID, err := loadCommittedDraftSaveOperation(ctx, runtime.Reference.Store.db, op, documentID, revisionID, sourceVersion, digest)
	if err != nil {
		return "", false, err
	}
	if runtime.Media == nil {
		return "", false, platform.NewError(platform.CodeDependencyUnavailable, true, "发布资源pin依赖未装配")
	}
	request := struct {
		draftPinMaterial
		References []draftMediaReference `json:"references"`
	}{draftPinMaterial{sourceOperationID, op.Actor.SiteID, documentID, revisionID, sourceVersion, digest}, mediaRefs}
	raw, err := runtime.Media.AcquireResourceReferences(contentServiceContext(ctx, op), request)
	if err != nil {
		return "", false, err
	}
	var response struct {
		Pin struct {
			PinID  Identifier            `json:"pinId"`
			Assets []gen.AssetVersionRef `json:"assets"`
		} `json:"pin"`
	}
	if json.Unmarshal(raw, &response) != nil || !platform.ValidIdentifier(response.Pin.PinID) {
		return "", false, platform.NewError(platform.CodeIntegrityError, false, "资源pin响应不完整")
	}
	wanted := map[Identifier]gen.AssetVersionRef{}
	for _, ref := range mediaRefs {
		wanted[ref.Asset.AssetId] = ref.Asset
	}
	if len(wanted) != len(response.Pin.Assets) {
		return "", false, platform.NewError(platform.CodeIntegrityError, false, "资源pin资产集合不一致")
	}
	for _, asset := range response.Pin.Assets {
		if expected, found := wanted[asset.AssetId]; !found || expected != asset {
			return "", false, platform.NewError(platform.CodeIntegrityError, false, "资源pin资产材料不一致")
		}
		delete(wanted, asset.AssetId)
	}
	confirmed, err := runtime.Media.ConfirmResourceReferences(contentServiceContext(ctx, op), struct {
		PinID            Identifier `json:"pinId"`
		SourceRevisionID Identifier `json:"sourceRevisionId"`
		SourceVersion    int        `json:"sourceVersion"`
	}{response.Pin.PinID, revisionID, sourceVersion})
	if err != nil {
		return "", false, err
	}
	var result struct {
		Confirmed bool `json:"confirmed"`
	}
	if json.Unmarshal(confirmed, &result) != nil || !result.Confirmed {
		return "", false, platform.NewError(platform.CodeDependencyUnavailable, true, "资源引用确认尚未完成")
	}
	return digest, true, nil
}

func copyPublishedReferences(ctx context.Context, tx *sql.Tx, op OperationContext, documentID, draftRevisionID, publishedRevisionID Identifier, referenceVersion int64) ([]gen.ResourceReference, int, error) {
	refs, sourceVersion, err := loadCurrentDraftReferences(ctx, tx, op, documentID, draftRevisionID)
	if err != nil {
		return nil, 0, err
	}
	if int64(sourceVersion) != referenceVersion {
		return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "草稿资源引用水位与文档不一致")
	}
	facts, ok := publicationFactsFrom(ctx)
	if !ok {
		return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "发布提交缺少已核对的账户或正文版本")
	}
	if len(refs) == 0 {
		if facts.referencesPinned {
			return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "空发布引用不应带有资源pin")
		}
	} else {
		_, digest, digestErr := draftMediaReferences(refs)
		if digestErr != nil || !facts.referencesPinned || digest != facts.referenceDigest {
			return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "发布资源引用与已确认pin不一致")
		}
	}
	for i := range refs {
		if refs[i].DocumentId != documentID || refs[i].SourceRevisionId != draftRevisionID || refs[i].SourceVersion != sourceVersion || refs[i].Kind != gen.CurrentDraft || !refs[i].Retained {
			return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "草稿资源引用身份矛盾")
		}
		refs[i].Kind = gen.CurrentPublished
		refs[i].SourceService = "content"
	}
	encoded, err := json.Marshal(refs)
	if err != nil {
		return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "发布资源引用无法编码")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE publication_resource_references SET kind='history' WHERE site_id=? AND document_id=? AND kind='current_published'`, op.Actor.SiteID, documentID); err != nil {
		return nil, 0, contentStorageError()
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO publication_resource_references(site_id,document_id,published_revision_id,revision_id,source_version,kind,references_json) VALUES(?,?,?,?,?,'current_published',?)`, op.Actor.SiteID, documentID, publishedRevisionID, draftRevisionID, sourceVersion, string(encoded)); err != nil {
		return nil, 0, contentStorageError()
	}
	return refs, sourceVersion, nil
}

func enqueuePublication(ctx context.Context, op OperationContext, visibility int, changed *publicationChangedPayload, withdrawn *publicationWithdrawnPayload, documentID Identifier, sourceVersion int, refs []gen.ResourceReference) error {
	if refs == nil {
		refs = []gen.ResourceReference{}
	}
	if len(refs) > 10000 {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "发布资源引用超过事件登记上限")
	}
	if changed != nil {
		if _, err := PublishContentEvent(ctx, op, PublishContentEventInput{Changed: changed, EventID: platform.NewIdentifier(), Revision: visibility}); err != nil {
			return err
		}
	}
	if withdrawn != nil {
		if _, err := PublishContentEvent(ctx, op, PublishContentEventInput{Withdrawn: withdrawn, EventID: platform.NewIdentifier(), Revision: visibility}); err != nil {
			return err
		}
	}
	_, err := PublishContentEvent(ctx, op, PublishContentEventInput{References: &resourceReferencesChangedPayload{SiteID: op.Actor.SiteID, DocumentID: documentID, SourceRevision: sourceVersion, References: refs, RemovedReferenceIDs: []Identifier{}, DataEpoch: op.DataEpoch}, EventID: platform.NewIdentifier(), Revision: visibility})
	return err
}

func enqueueTreeChanged(ctx context.Context, tx *sql.Tx, op OperationContext, treeRevision int64, changed []Identifier) error {
	payload, err := json.Marshal(struct {
		SiteID             Identifier   `json:"siteId"`
		TreeRevision       int64        `json:"treeRevision"`
		ChangedDocumentIDs []Identifier `json:"changedDocumentIds"`
		DataEpoch          Identifier   `json:"dataEpoch"`
	}{op.Actor.SiteID, treeRevision, changed, op.DataEpoch})
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "文档树事件无法编码")
	}
	return outbox.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "content.tree.changed", EventVersion: 1, Producer: "content", SubjectID: op.Actor.SiteID, Revision: treeRevision, OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload})
}

func listPublicationReferences(ctx context.Context, tx *sql.Tx, op OperationContext, documentID Identifier) ([]gen.ResourceReference, int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT revision_id,source_version,kind,CASE WHEN length(CAST(references_json AS BLOB))<=16777216 THEN references_json ELSE NULL END FROM publication_resource_references WHERE site_id=? AND document_id=? ORDER BY published_revision_id`, op.Actor.SiteID, documentID)
	if err != nil {
		return nil, 0, contentStorageError()
	}
	defer rows.Close()
	result := []gen.ResourceReference{}
	source := 0
	for rows.Next() {
		var revisionID Identifier
		var version int
		var kind string
		var raw []byte
		if err = rows.Scan(&revisionID, &version, &kind, &raw); err != nil {
			return nil, 0, contentStorageError()
		}
		var refs []gen.ResourceReference
		if raw == nil || decodeStorageJSON(raw, &refs) != nil {
			return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "发布资源引用损坏")
		}
		for i := range refs {
			if refs[i].DocumentId != documentID || refs[i].SourceRevisionId != revisionID || refs[i].SourceVersion != version || !refs[i].Retained {
				return nil, 0, platform.NewError(platform.CodeIntegrityError, false, "发布资源引用身份矛盾")
			}
			refs[i].Kind = gen.ResourceReferenceKind(kind)
			refs[i].SourceService = "content"
			result = append(result, refs[i])
		}
		if version > source {
			source = version
		}
		if len(result) > 10000 {
			return nil, 0, platform.NewError(platform.CodeDependencyUnavailable, true, "发布资源引用超过事件登记上限")
		}
	}
	if err = rows.Err(); err != nil {
		return nil, 0, contentStorageError()
	}
	return result, source, nil
}

func archivePublishedReferences(ctx context.Context, tx *sql.Tx, op OperationContext, documentID Identifier) error {
	updated, err := tx.ExecContext(ctx, `UPDATE publication_resource_references SET kind='history' WHERE site_id=? AND document_id=? AND kind='current_published'`, op.Actor.SiteID, documentID)
	if err != nil {
		return contentStorageError()
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return contentStorageError()
	}
	if count != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "撤回时当前发布引用不存在")
	}
	return nil
}
