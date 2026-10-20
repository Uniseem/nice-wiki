package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/content/internal/gen"
)

// DraftRuntime由服务入口装配真实依赖；不生成文档或跳过有效租约。
type DraftRuntime struct {
	Store       *Store
	Manager     *EditLockManager
	Identity    *gen.IdentityClient
	SiteConfig  *gen.SiteConfigClient
	Media       *gen.MediaClient
	WriteWindow platform.WriteWindowConfig
}
type draftRuntimeKey struct{}

func WithDraftRuntime(ctx context.Context, runtime DraftRuntime) context.Context {
	return context.WithValue(ctx, draftRuntimeKey{}, runtime)
}
func draftRuntimeFromContext(ctx context.Context) (DraftRuntime, error) {
	runtime, ok := ctx.Value(draftRuntimeKey{}).(DraftRuntime)
	if !ok || runtime.Store == nil || runtime.Manager == nil || runtime.Manager.store != runtime.Store {
		return runtime, platform.NewError(platform.CodeDependencyUnavailable, true, "共享草稿依赖未装配")
	}
	return runtime, nil
}

// draftOperationPlan仅存固定IDs/CAS/引用/元数据；正文文件保持唯一内容权威。
type draftOperationPlan struct {
	DocumentID               Identifier              `json:"documentId"`
	RevisionID               Identifier              `json:"revisionId"`
	BodyHash                 Hash                    `json:"bodyHash"`
	ExpectedHead             *Identifier             `json:"expectedHead"`
	ExpectedDraftVersion     gen.Revision            `json:"expectedDraftVersion"`
	ExpectedTreeRevision     int64                   `json:"expectedTreeRevision"`
	ExpectedReferenceVersion int64                   `json:"expectedReferenceVersion"`
	SourceVersion            int                     `json:"sourceVersion"`
	Metadata                 *gen.DocumentMetadata   `json:"metadata"`
	Fence                    DraftFenceInput         `json:"fence"`
	BlockIDs                 []Identifier            `json:"blockIds"`
	References               []gen.ResourceReference `json:"references"`
	MediaReferences          []draftMediaReference   `json:"mediaReferences"`
	ReferenceDigest          Hash                    `json:"referenceDigest"`
}
type draftOperation struct {
	Action      string
	RequestHash Hash
	Plan        draftOperationPlan
	State       string
	PinID       *Identifier
	Result      json.RawMessage
}

func (s *Store) readDraftOperation(ctx context.Context, op OperationContext, action string, digest Hash) (draftOperation, bool, error) {
	if err := s.checkRevisionScope(ctx, op); err != nil {
		return draftOperation{}, false, err
	}
	return readDraftOperationFrom(ctx, s.db, op, action, digest)
}

type draftOperationQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readDraftOperationFrom(ctx context.Context, query draftOperationQuery, op OperationContext, action string, digest Hash) (draftOperation, bool, error) {
	var record draftOperation
	var raw []byte
	var caller, actor string
	err := query.QueryRowContext(ctx, `SELECT caller_service,actor_id,action,request_hash,plan_json,state,pin_id,result_json FROM draft_operation_intents WHERE site_id=? AND data_epoch=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&caller, &actor, &record.Action, &record.RequestHash, &raw, &record.State, &record.PinID, &record.Result)
	if errors.Is(err, sql.ErrNoRows) {
		return record, false, nil
	}
	if err != nil {
		return record, false, contentStorageError()
	}
	if caller != op.CallerService || actor != op.Actor.SubjectID || record.Action != action || record.RequestHash != digest {
		return record, false, platform.NewError(platform.CodeConflict, false, "草稿操作身份、动作或摘要不一致")
	}
	if decodeStorageJSON(raw, &record.Plan) != nil {
		return record, false, platform.NewError(platform.CodeIntegrityError, false, "草稿原始意图损坏")
	}
	if err = validateDraftOperationPlan(record.Action, record.Plan); err != nil {
		return record, false, err
	}
	if record.State == "committed" {
		var committedRaw []byte
		err = query.QueryRowContext(ctx, `SELECT r.result_json FROM draft_commit_receipts r JOIN committed_revisions c ON c.commit_id=r.commit_id AND c.site_id=r.site_id AND c.data_epoch=r.data_epoch WHERE r.caller_service=? AND r.actor_id=? AND r.site_id=? AND r.data_epoch=? AND r.operation_id=? AND c.document_id=? AND c.revision_id=?`, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID, record.Plan.DocumentID, record.Plan.RevisionID).Scan(&committedRaw)
		var committed CommitDraftJournalResult
		var full struct {
			Draft    DraftRevision         `json:"draft"`
			Metadata *gen.DocumentMetadata `json:"metadata,omitempty"`
		}
		if err != nil || decodeStorageJSON(committedRaw, &committed) != nil || decodeStorageJSON(record.Result, &full) != nil || !sameStoredDraft(full.Draft, committed.Draft) || committed.Draft.RevisionId != record.Plan.RevisionID || committed.Draft.BodyHash != record.Plan.BodyHash {
			return record, false, platform.NewError(platform.CodeIntegrityError, false, "草稿已提交意图缺少一致的完整原回执或资格")
		}
		if action == "create" {
			expected, _ := json.Marshal(record.Plan.Metadata)
			actual, _ := json.Marshal(full.Metadata)
			if string(expected) != string(actual) {
				return record, false, platform.NewError(platform.CodeIntegrityError, false, "创建文档原始元数据回执矛盾")
			}
		}
	}
	return record, true, nil
}

func (s *Store) prepareDraftOperation(ctx context.Context, op OperationContext, record draftOperation) (draftOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, found, err := s.readDraftOperation(ctx, op, record.Action, record.RequestHash); err != nil {
		return record, err
	} else if found {
		return previous, nil
	}
	raw, err := json.Marshal(record.Plan)
	if err != nil {
		return record, contentStorageError()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return record, contentStorageError()
	}
	defer tx.Rollback()
	updated, err := tx.ExecContext(ctx, `UPDATE store_identity SET site_id=? WHERE singleton=1 AND data_epoch=? AND (site_id IS NULL OR site_id=?)`, op.Actor.SiteID, op.DataEpoch, op.Actor.SiteID)
	if err != nil {
		return record, contentStorageError()
	}
	count, err := updated.RowsAffected()
	if err != nil || count != 1 {
		return record, platform.NewError(platform.CodeVersionMismatch, false, "草稿意图站点或代次已变化")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO draft_operation_intents(site_id,data_epoch,operation_id,caller_service,actor_id,action,request_hash,plan_json,state) VALUES(?,?,?,?,?,?,?,?,'pending')`, op.Actor.SiteID, op.DataEpoch, op.OperationID, op.CallerService, op.Actor.SubjectID, record.Action, record.RequestHash, string(raw))
	if err != nil {
		return record, contentStorageError()
	}
	if err = tx.Commit(); err != nil {
		return record, contentStorageError()
	}
	record.State = "pending"
	return record, nil
}

func draftRequestDigest(action string, input any) (Hash, error) {
	raw, err := platform.CanonicalJSON(struct {
		Action string `json:"action"`
		Input  any    `json:"input"`
	}{action, input})
	if err != nil {
		return "", err
	}
	return platform.HashBytes(raw), nil
}

// draftCommitStore延迟打开SQL；所有文件与当前写窗口RPC都在doc guard之前。
type draftCommitStore struct {
	RevisionFileStore
	runtime   DraftRuntime
	operation OperationContext
	record    draftOperation
}

func (s *draftCommitStore) RevisionSiteID() Identifier { return s.operation.Actor.SiteID }
func (s *draftCommitStore) BeginDraftCommit(ctx context.Context, request DraftCommitRequest) (DraftCommitTransaction, error) {
	return &draftCommitTransaction{store: s, request: request}, nil
}

type draftCommitTransaction struct {
	store       *draftCommitStore
	request     DraftCommitRequest
	tx          *sql.Tx
	guard       *SaveFenceGuard
	cancel      context.CancelFunc
	work        context.Context
	finished    bool
	replayed    *CommitDraftJournalResult
	managerLife bool
}

func (t *draftCommitTransaction) FindReceipt(ctx context.Context, operationID Identifier, digest Hash) (CommitDraftJournalResult, bool, error) {
	op := t.request.Operation
	var stored Hash
	var raw, preparedRaw []byte
	err := t.store.runtime.Store.db.QueryRowContext(ctx, `SELECT r.request_hash,r.result_json,p.result_json FROM draft_commit_receipts r JOIN committed_revisions c ON c.commit_id=r.commit_id AND c.site_id=r.site_id AND c.data_epoch=r.data_epoch JOIN revision_preparations p ON p.commit_id=c.commit_id AND p.site_id=c.site_id AND p.document_id=c.document_id AND p.revision_id=c.revision_id AND p.data_epoch=c.data_epoch WHERE r.caller_service=? AND r.actor_id=? AND r.site_id=? AND r.data_epoch=? AND r.operation_id=?`, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, operationID).Scan(&stored, &raw, &preparedRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return CommitDraftJournalResult{}, false, nil
	}
	if err != nil {
		return CommitDraftJournalResult{}, false, contentStorageError()
	}
	if stored != digest {
		return CommitDraftJournalResult{}, false, platform.NewError(platform.CodeConflict, false, "草稿提交同操作摘要不同")
	}
	var result CommitDraftJournalResult
	var prepared WriteRevisionFileResult
	if decodeStorageJSON(raw, &result) != nil || decodeStorageJSON(preparedRaw, &prepared) != nil || !sameStoredDraft(result.Draft, prepared.Draft) {
		return result, false, platform.NewError(platform.CodeIntegrityError, false, "草稿提交原回执损坏")
	}
	return result, true, nil
}
func (t *draftCommitTransaction) ValidateFence(ctx context.Context, input CommitDraftJournalInput) error {
	op := t.request.Operation
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), t.store.runtime.WriteWindow)
	if err != nil {
		return err
	}
	t.cancel = cancel
	t.work = work
	if view.DataEpoch != op.DataEpoch || t.store.runtime.Store.epoch != op.DataEpoch {
		return platform.NewError(platform.CodeVersionMismatch, false, "草稿提交安全窗口代次已变化")
	}
	if t.store.record.Action == "save" {
		t.guard, err = t.store.runtime.Manager.AcquireSaveFence(work, op, t.store.record.Plan.Fence)
		if err != nil {
			return err
		}
	} else if t.store.record.Action != "create" || t.store.record.Plan.Metadata == nil || input.ExpectedHead != nil || input.ExpectedDraftVersion != 0 {
		return platform.NewError(platform.CodeForbidden, false, "首次草稿必须来自受信创建意图")
	} else {
		t.store.runtime.Manager.life.RLock()
		t.managerLife = true
		if t.store.runtime.Manager.closed {
			return platform.NewError(platform.CodeLocked, false, "创建文档leader已关闭")
		}
	}
	t.tx, err = t.store.runtime.Store.db.BeginTx(work, nil)
	if err != nil {
		return contentStorageError()
	}
	return t.validateDraftTransaction(work, input)
}

func (t *draftCommitTransaction) validateDraftTransaction(ctx context.Context, input CommitDraftJournalInput) error {
	op := t.request.Operation
	record := t.store.record
	var action, state, caller, actor string
	var digest Hash
	var planRaw []byte
	var pin *Identifier
	err := t.tx.QueryRowContext(ctx, `SELECT action,state,caller_service,actor_id,request_hash,plan_json,pin_id FROM draft_operation_intents WHERE site_id=? AND data_epoch=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&action, &state, &caller, &actor, &digest, &planRaw, &pin)
	if err != nil {
		return contentStorageError()
	}
	raw, _ := json.Marshal(record.Plan)
	if action != record.Action || caller != op.CallerService || actor != op.Actor.SubjectID || digest != record.RequestHash || string(raw) != string(planRaw) {
		return platform.NewError(platform.CodeIntegrityError, false, "草稿提交与首次持久意图不一致")
	}
	if state == "committed" {
		var receiptRaw []byte
		var receiptHash Hash
		if err = t.tx.QueryRowContext(ctx, `SELECT request_hash,result_json FROM draft_commit_receipts WHERE `+preparationScopeSQL, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&receiptHash, &receiptRaw); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "并发草稿提交缺少原回执")
		}
		var receipt CommitDraftJournalResult
		if receiptHash != t.request.CanonicalDigest || decodeStorageJSON(receiptRaw, &receipt) != nil || !sameStoredDraft(receipt.Draft, input.Draft) {
			return platform.NewError(platform.CodeIntegrityError, false, "并发草稿提交原回执矛盾")
		}
		t.replayed = &receipt
		return nil
	}
	if state != "pending" {
		return platform.NewError(platform.CodeConflict, false, "草稿意图已终局，不能迟到提交")
	}
	if record.Action == "save" && pin == nil {
		return platform.NewError(platform.CodeLocked, false, "草稿资源pin尚未持久")
	}
	var preparedRaw []byte
	var prepState string
	err = t.tx.QueryRowContext(ctx, `SELECT result_json,state FROM revision_preparations WHERE commit_id=? AND `+preparationScopeSQL, input.Journal.CommitID, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&preparedRaw, &prepState)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "草稿提交缺少同操作准备")
	}
	var prepared WriteRevisionFileResult
	if decodeStorageJSON(preparedRaw, &prepared) != nil || prepState != "prepared" || !sameStoredDraft(prepared.Draft, input.Draft) || !sameJournalIdentity(prepared.Journal, input.Journal) || record.Plan.DocumentID != input.Draft.DocumentId || record.Plan.RevisionID != input.Draft.RevisionId || record.Plan.BodyHash != input.Draft.BodyHash || !sameIdentifierPointer(record.Plan.ExpectedHead, input.ExpectedHead) || record.Plan.ExpectedDraftVersion != input.ExpectedDraftVersion {
		return platform.NewError(platform.CodeIntegrityError, false, "草稿准备或固定材料不一致")
	}
	var epoch, site string
	if err = t.tx.QueryRowContext(ctx, `SELECT data_epoch,site_id FROM store_identity WHERE singleton=1`).Scan(&epoch, &site); err != nil {
		return contentStorageError()
	}
	if epoch != op.DataEpoch || site != op.Actor.SiteID {
		return platform.NewError(platform.CodeVersionMismatch, false, "草稿持久站点或代次已变化")
	}
	if record.Action == "create" {
		return t.validateCreateTransaction(ctx)
	}
	var stateDoc string
	var fence, referenceVersion int64
	var quarantined bool
	err = t.tx.QueryRowContext(ctx, `SELECT state,delete_fence,reference_version,EXISTS(SELECT 1 FROM document_quarantines q WHERE q.site_id=d.site_id AND q.document_id=d.document_id) FROM documents d WHERE site_id=? AND document_id=?`, site, input.Draft.DocumentId).Scan(&stateDoc, &fence, &referenceVersion, &quarantined)
	if errors.Is(err, sql.ErrNoRows) {
		return platform.NewError(platform.CodeNotFound, false, "保存文档不存在")
	}
	if err != nil {
		return contentStorageError()
	}
	if err = requireEditableDocument(DocumentEditState{State: stateDoc, Quarantined: quarantined}); err != nil {
		return err
	}
	if fence != record.Plan.Fence.ExpectedDeleteFence {
		return platform.NewError(platform.CodeVersionMismatch, false, "保存文档删除fence已变化")
	}
	if referenceVersion != record.Plan.ExpectedReferenceVersion {
		return platform.NewError(platform.CodeVersionMismatch, false, "文档引用水位已变化")
	}
	// guard一直持有同doc mutex；纯内存再核只覆盖短SQL内自然到期。
	if err = t.store.runtime.Manager.matchEditLease(t.guard.cell, op, record.Plan.Fence, time.Now().UTC()); err != nil {
		return err
	}
	var head Identifier
	var version int
	err = t.tx.QueryRowContext(ctx, `SELECT revision_id,draft_version FROM draft_heads WHERE site_id=? AND document_id=?`, site, input.Draft.DocumentId).Scan(&head, &version)
	if err != nil {
		return contentStorageError()
	}
	if input.ExpectedHead == nil || head != *input.ExpectedHead || version != input.ExpectedDraftVersion {
		return platform.NewError(platform.CodeVersionMismatch, false, "共享草稿head或版本已变化")
	}
	return nil
}

func (t *draftCommitTransaction) Commit(ctx context.Context, request DraftCommitRequest) (CommitDraftJournalResult, error) {
	if t.replayed != nil {
		result := *t.replayed
		_ = t.Rollback(ctx)
		return result, nil
	}
	work := t.work
	op := request.Operation
	plan := t.store.record.Plan
	draft := request.Input.Draft
	result := CommitDraftJournalResult{Draft: draft}
	if t.store.record.Action == "create" {
		metadata := *plan.Metadata
		metadataRaw, _ := json.Marshal(metadata)
		if _, err := t.tx.ExecContext(work, `INSERT INTO documents(site_id,document_id,state,revision,delete_fence,reference_version) VALUES(?,?,'draft',1,1,1)`, op.Actor.SiteID, plan.DocumentID); err != nil {
			return result, contentStorageError()
		}
		if _, err := t.tx.ExecContext(work, `INSERT INTO document_metadata(site_id,document_id,metadata_json) VALUES(?,?,?)`, op.Actor.SiteID, plan.DocumentID, string(metadataRaw)); err != nil {
			return result, contentStorageError()
		}
		if _, err := t.tx.ExecContext(work, `INSERT INTO draft_heads(site_id,document_id,revision_id,draft_version) VALUES(?,?,?,1)`, op.Actor.SiteID, plan.DocumentID, draft.RevisionId); err != nil {
			return result, contentStorageError()
		}
		changed, err := t.tx.ExecContext(work, `INSERT INTO content_tree_state(site_id,revision) VALUES(?,1) ON CONFLICT(site_id) DO UPDATE SET revision=revision+1 WHERE revision=?`, op.Actor.SiteID, plan.ExpectedTreeRevision)
		if err != nil {
			return result, contentStorageError()
		}
		count, err := changed.RowsAffected()
		if err != nil || count != 1 {
			return result, platform.NewError(platform.CodeVersionMismatch, false, "创建文档站点树CAS失败")
		}
	} else {
		referenceCAS, err := t.tx.ExecContext(work, `UPDATE documents SET reference_version=? WHERE site_id=? AND document_id=? AND reference_version=?`, plan.SourceVersion, op.Actor.SiteID, plan.DocumentID, plan.ExpectedReferenceVersion)
		if err != nil {
			return result, contentStorageError()
		}
		count, err := referenceCAS.RowsAffected()
		if err != nil || count != 1 {
			return result, platform.NewError(platform.CodeVersionMismatch, false, "文档引用水位CAS失败")
		}
		updated, err := t.tx.ExecContext(work, `UPDATE draft_heads SET revision_id=?,draft_version=? WHERE site_id=? AND document_id=? AND revision_id=? AND draft_version=?`, draft.RevisionId, draft.DraftVersion, op.Actor.SiteID, plan.DocumentID, *plan.ExpectedHead, plan.ExpectedDraftVersion)
		if err != nil {
			return result, contentStorageError()
		}
		count, err = updated.RowsAffected()
		if err != nil || count != 1 {
			return result, platform.NewError(platform.CodeVersionMismatch, false, "共享草稿CAS失败")
		}
		updated, err = t.tx.ExecContext(work, `UPDATE document_metadata SET metadata_json=json_set(metadata_json,'$.draftRevisionId',?) WHERE site_id=? AND document_id=?`, draft.RevisionId, op.Actor.SiteID, plan.DocumentID)
		if err != nil {
			return result, contentStorageError()
		}
		count, err = updated.RowsAffected()
		if err != nil || count != 1 {
			return result, platform.NewError(platform.CodeIntegrityError, false, "共享草稿缺少元数据权威")
		}
	}
	if err := t.commitStableIDs(work, op, plan); err != nil {
		return result, err
	}
	if _, err := t.tx.ExecContext(work, `UPDATE draft_resource_references SET kind='history' WHERE site_id=? AND document_id=? AND kind='current_draft'`, op.Actor.SiteID, plan.DocumentID); err != nil {
		return result, contentStorageError()
	}
	refsRaw, _ := json.Marshal(plan.References)
	if _, err := t.tx.ExecContext(work, `INSERT INTO draft_resource_references(site_id,document_id,revision_id,source_version,kind,references_json) VALUES(?,?,?,?,'current_draft',?)`, op.Actor.SiteID, plan.DocumentID, plan.RevisionID, plan.SourceVersion, string(refsRaw)); err != nil {
		return result, contentStorageError()
	}
	if _, err := t.tx.ExecContext(work, `INSERT INTO committed_revisions(commit_id,site_id,document_id,revision_id,data_epoch,state) VALUES(?,?,?,?,?,'committed')`, request.Input.Journal.CommitID, op.Actor.SiteID, plan.DocumentID, plan.RevisionID, op.DataEpoch); err != nil {
		return result, contentStorageError()
	}
	if _, err := t.tx.ExecContext(work, `UPDATE revision_preparations SET state='committed' WHERE commit_id=? AND state='prepared'`, request.Input.Journal.CommitID); err != nil {
		return result, contentStorageError()
	}
	raw, _ := json.Marshal(result)
	if _, err := t.tx.ExecContext(work, `INSERT INTO draft_commit_receipts(commit_id,caller_service,actor_id,site_id,data_epoch,operation_id,request_hash,result_json) VALUES(?,?,?,?,?,?,?,?)`, request.Input.Journal.CommitID, op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, op.OperationID, request.CanonicalDigest, string(raw)); err != nil {
		return result, contentStorageError()
	}
	var full any = gen.SaveDraftResult{Draft: draft}
	if t.store.record.Action == "create" {
		full = gen.CreateDocumentResult{Metadata: *plan.Metadata, Draft: draft}
	}
	fullRaw, _ := json.Marshal(full)
	if _, err := t.tx.ExecContext(work, `UPDATE draft_operation_intents SET state='committed',result_json=? WHERE site_id=? AND data_epoch=? AND operation_id=? AND state='pending'`, string(fullRaw), op.Actor.SiteID, op.DataEpoch, op.OperationID); err != nil {
		return result, contentStorageError()
	}
	if t.store.record.Action == "save" {
		// 原意图中的规范材料已与事务内plan_json核对；空集合仍编码为[]。
		payload, err := json.Marshal(struct {
			SourceService     string                `json:"sourceService"`
			SourceOperationID Identifier            `json:"sourceOperationId"`
			SiteID            Identifier            `json:"siteId"`
			SourceObjectID    Identifier            `json:"sourceObjectId"`
			SourceRevisionID  Identifier            `json:"sourceRevisionId"`
			SourceVersion     int                   `json:"sourceVersion"`
			ReferenceDigest   Hash                  `json:"referenceDigest"`
			References        []draftMediaReference `json:"references"`
		}{"content", op.OperationID, op.Actor.SiteID, plan.DocumentID, plan.RevisionID, plan.SourceVersion, plan.ReferenceDigest, plan.MediaReferences})
		if err != nil {
			return result, platform.NewError(platform.CodeIntegrityError, false, "草稿资源事件载荷无法编码")
		}
		if err := outbox.EnqueueEvent(t.tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "content.resources.changed", EventVersion: 2, Producer: "content", SubjectID: plan.DocumentID, Revision: int64(plan.SourceVersion), OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
			return result, err
		}
	}
	err := t.tx.Commit()
	// database/sql的Commit已终结事务；任何错误都保留原意图与pin，由真实回执重试解决。
	t.finished = true
	t.releaseDraftTransaction()
	if err != nil {
		return CommitDraftJournalResult{}, contentStorageError()
	}
	return result, nil
}
func (t *draftCommitTransaction) commitStableIDs(ctx context.Context, op OperationContext, plan draftOperationPlan) error {
	rows, err := t.tx.QueryContext(ctx, `SELECT stable_id,state FROM document_id_ledger WHERE site_id=? AND document_id=?`, op.Actor.SiteID, plan.DocumentID)
	if err != nil {
		return contentStorageError()
	}
	previous := map[Identifier]string{}
	for rows.Next() {
		var id Identifier
		var state string
		if err = rows.Scan(&id, &state); err != nil {
			rows.Close()
			return contentStorageError()
		}
		previous[id] = state
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return contentStorageError()
	}
	current := make(map[Identifier]bool, len(plan.BlockIDs))
	for _, id := range plan.BlockIDs {
		current[id] = true
		if previous[id] == "retired" {
			return platform.NewError(platform.CodeIntegrityError, false, "正文复用了历史退役稳定ID")
		}
		if _, found := previous[id]; !found {
			if _, err = t.tx.ExecContext(ctx, `INSERT INTO document_id_ledger(site_id,document_id,stable_id,state,introduced_revision_id) VALUES(?,?,?,'active',?)`, op.Actor.SiteID, plan.DocumentID, id, plan.RevisionID); err != nil {
				return contentStorageError()
			}
		}
	}
	for id, state := range previous {
		if state == "active" && !current[id] {
			if _, err = t.tx.ExecContext(ctx, `UPDATE document_id_ledger SET state='retired',retired_revision_id=? WHERE site_id=? AND document_id=? AND stable_id=? AND state='active'`, plan.RevisionID, op.Actor.SiteID, plan.DocumentID, id); err != nil {
				return contentStorageError()
			}
		}
	}
	return nil
}
func (t *draftCommitTransaction) releaseDraftTransaction() {
	if t.guard != nil {
		t.guard.ReleaseFence()
		t.guard = nil
	}
	if t.managerLife {
		t.store.runtime.Manager.life.RUnlock()
		t.managerLife = false
	}
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
}
func (t *draftCommitTransaction) Rollback(ctx context.Context) error {
	if t.tx != nil && !t.finished {
		_ = t.tx.Rollback()
		t.finished = true
	}
	t.releaseDraftTransaction()
	return nil
}

func validateDraftOperationPlan(action string, plan draftOperationPlan) error {
	if !platform.ValidIdentifier(plan.DocumentID) || !platform.ValidIdentifier(plan.RevisionID) || !platform.ValidHash(plan.BodyHash) || plan.ExpectedDraftVersion < 0 || plan.ExpectedReferenceVersion < 0 || plan.ExpectedReferenceVersion >= maxEditFence || int64(plan.SourceVersion) != plan.ExpectedReferenceVersion+1 {
		return platform.NewError(platform.CodeIntegrityError, false, "草稿固定身份或正文摘要损坏")
	}
	if action == "create" {
		if plan.Metadata == nil || plan.ExpectedHead != nil || plan.ExpectedDraftVersion != 0 || plan.ExpectedReferenceVersion != 0 || plan.SourceVersion != 1 || plan.Metadata.DocumentId != plan.DocumentID || plan.Metadata.DraftRevisionId != plan.RevisionID {
			return platform.NewError(platform.CodeIntegrityError, false, "创建意图元数据或首次CAS损坏")
		}
		return nil
	}
	if action != "save" || plan.Metadata != nil || plan.ExpectedHead == nil || plan.Fence.DocumentID != plan.DocumentID || plan.ExpectedDraftVersion < 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "保存意图CAS或fence材料损坏")
	}
	raw, err := platform.CanonicalJSON(plan.MediaReferences)
	if err != nil || platform.HashBytes(raw) != plan.ReferenceDigest {
		return platform.NewError(platform.CodeIntegrityError, false, "保存意图资源摘要损坏")
	}
	for _, ref := range plan.MediaReferences {
		if ref.SourceService != "content" || ref.SourceObjectID != plan.DocumentID || ref.SourceRevisionID != plan.RevisionID || ref.SourceVersion != plan.SourceVersion || ref.Kind != "pending_pin" || ref.Confirmed || ref.ExpiresAt != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "保存意图资源来源材料损坏")
		}
	}
	return nil
}
