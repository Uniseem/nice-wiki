package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type ReadDraftInput struct {
	DocumentId Identifier `json:"documentId"`
}
type ReadDraftResult = gen.ReadDraftResult
type SaveDraftInput struct {
	DocumentId              Identifier   `json:"documentId"`
	Body                    DocumentBody `json:"body"`
	ExpectedDraftVersion    gen.Revision `json:"expectedDraftVersion"`
	ExpectedDraftRevisionId Identifier   `json:"expectedDraftRevisionId"`
	LeaseId                 Identifier   `json:"leaseId"`
	FencingToken            int          `json:"fencingToken"`
	ServerBootId            Identifier   `json:"serverBootId"`
}
type SaveDraftResult = gen.SaveDraftResult

func ReadDraft(ctx context.Context, op OperationContext, input ReadDraftInput) (ReadDraftResult, error) {
	runtime, err := draftRuntimeFromContext(ctx)
	if err != nil {
		return ReadDraftResult{}, err
	}
	if runtime.Identity == nil {
		return ReadDraftResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "草稿读取授权未装配")
	}
	if _, err = authorizeEditLock(ctx, op, runtime.Identity, "content.read", "readDraft", input.DocumentId); err != nil {
		return ReadDraftResult{}, err
	}
	metadata, draft, err := runtime.Store.readDraftSnapshot(ctx, op, input.DocumentId)
	if err != nil {
		return ReadDraftResult{}, err
	}
	fileStore := runtime.Store.ConfigureRevisionStorage(RevisionStorageConfig{})
	body, err := ReadRevisionBody(WithRevisionFileStore(ctx, fileStore), op, ReadRevisionBodyInput{DocumentID: input.DocumentId, RevisionID: draft.RevisionId, ExpectedHash: &draft.BodyHash})
	if err != nil {
		return ReadDraftResult{}, err
	}
	// IO结束后才短暂取同doc guard，返回当前租约且不据旧回执复活锁。
	guard, err := runtime.Manager.AcquireDocumentFence(ctx, op, input.DocumentId)
	if err != nil {
		return ReadDraftResult{}, err
	}
	defer guard.ReleaseFence()
	if guard.state.Quarantined {
		return ReadDraftResult{}, platform.NewError(platform.CodeIntegrityError, false, "草稿文档已隔离")
	}
	if guard.state.State == "purged" {
		return ReadDraftResult{}, platform.NewError(platform.CodeNotFound, false, "文档已永久清空")
	}
	lease := copyEditLease(guard.cell.lease)
	if lease != nil && editLeaseExpired(lease, time.Now().UTC()) {
		lease = nil
	}
	return ReadDraftResult{Metadata: metadata, Draft: draft, Body: body.Body, Lease: lease}, nil
}

func (s *Store) readDraftSnapshot(ctx context.Context, op OperationContext, documentID Identifier) (gen.DocumentMetadata, DraftRevision, error) {
	if err := s.checkRevisionScope(ctx, op); err != nil {
		return gen.DocumentMetadata{}, DraftRevision{}, err
	}
	var metadataRaw, preparedRaw, receiptRaw []byte
	var state string
	var head Identifier
	var version, revision int
	var referenceVersion int64
	var quarantined bool
	err := s.db.QueryRowContext(ctx, `SELECT m.metadata_json,p.result_json,r.result_json,d.state,d.revision,d.reference_version,h.revision_id,h.draft_version,EXISTS(SELECT 1 FROM document_quarantines q WHERE q.site_id=d.site_id AND q.document_id=d.document_id) FROM documents d LEFT JOIN document_metadata m ON m.site_id=d.site_id AND m.document_id=d.document_id LEFT JOIN draft_heads h ON h.site_id=d.site_id AND h.document_id=d.document_id LEFT JOIN revision_preparations p ON p.site_id=d.site_id AND p.document_id=d.document_id AND p.revision_id=h.revision_id AND p.data_epoch=? LEFT JOIN committed_revisions c ON c.commit_id=p.commit_id AND c.state='committed' LEFT JOIN draft_commit_receipts r ON r.commit_id=c.commit_id WHERE d.site_id=? AND d.document_id=? AND d.state<>'purged'`, op.DataEpoch, op.Actor.SiteID, documentID).Scan(&metadataRaw, &preparedRaw, &receiptRaw, &state, &revision, &referenceVersion, &head, &version, &quarantined)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.DocumentMetadata{}, DraftRevision{}, platform.NewError(platform.CodeNotFound, false, "文档不存在或已永久清空")
	}
	if err != nil {
		return gen.DocumentMetadata{}, DraftRevision{}, platform.NewError(platform.CodeIntegrityError, false, "共享草稿head或提交资格缺失")
	}
	var metadata gen.DocumentMetadata
	var prepared WriteRevisionFileResult
	var receipt CommitDraftJournalResult
	if referenceVersion < 1 || quarantined || decodeStorageJSON(metadataRaw, &metadata) != nil || decodeStorageJSON(preparedRaw, &prepared) != nil || decodeStorageJSON(receiptRaw, &receipt) != nil || metadata.SiteId != op.Actor.SiteID || metadata.DocumentId != documentID || metadata.DraftRevisionId != head || string(metadata.State) != state || metadata.Revision != revision || prepared.Draft.DocumentId != documentID || prepared.Draft.RevisionId != head || prepared.Draft.DraftVersion != version || !sameStoredDraft(prepared.Draft, receipt.Draft) {
		return metadata, DraftRevision{}, platform.NewError(platform.CodeIntegrityError, false, "共享草稿元数据、head或原回执不一致")
	}
	return metadata, prepared.Draft, nil
}

func SaveDraft(ctx context.Context, op OperationContext, input SaveDraftInput) (SaveDraftResult, error) {
	runtime, err := draftRuntimeFromContext(ctx)
	if err != nil {
		return SaveDraftResult{}, err
	}
	if runtime.Identity == nil || runtime.SiteConfig == nil || runtime.Media == nil {
		return SaveDraftResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "草稿保存授权、站点或媒体依赖未装配")
	}
	if _, err = authorizeEditLock(ctx, op, runtime.Identity, "content.edit", "saveDraft", input.DocumentId); err != nil {
		return SaveDraftResult{}, err
	}
	canonical, err := CanonicalizeBody(CanonicalizeBodyInput{Body: input.Body})
	if err != nil {
		return SaveDraftResult{}, err
	}
	// 持久请求摘要只含正文hash，原正文只写不可变文件。
	digest, err := draftRequestDigest("save", struct {
		DocumentID      Identifier
		BodyHash        Hash
		ExpectedHead    Identifier
		ExpectedVersion int
		LeaseID         Identifier
		Fence           int
		Boot            Identifier
	}{input.DocumentId, canonical.ContentHash, input.ExpectedDraftRevisionId, input.ExpectedDraftVersion, input.LeaseId, input.FencingToken, input.ServerBootId})
	if err != nil {
		return SaveDraftResult{}, err
	}
	record, found, err := runtime.Store.readDraftOperation(ctx, op, "save", digest)
	if err != nil {
		return SaveDraftResult{}, err
	}
	var limit int64
	if !found {
		_, limit, err = readDraftSiteConfig(ctx, op, runtime)
		if err != nil {
			return SaveDraftResult{}, err
		}
		if int64(len(canonical.UTF8)) > limit {
			return SaveDraftResult{}, platform.NewError(platform.CodeValidation, false, "正文超过当前站点大小上限")
		}
		plan, err := planDraftSave(ctx, op, runtime, input, canonical.ContentHash)
		if err != nil {
			return SaveDraftResult{}, err
		}
		work, cancel, err := draftWriteContext(ctx, op, runtime)
		if err != nil {
			return SaveDraftResult{}, err
		}
		record, err = runtime.Store.prepareDraftOperation(work, op, draftOperation{Action: "save", RequestHash: digest, Plan: plan})
		cancel()
		if err != nil {
			return SaveDraftResult{}, err
		}
	}
	// 初读或并发prepare的真实返回态统一分派；只有pending可开始后续准备。
	switch record.State {
	case "committed":
		return replaySavedDraft(ctx, op, runtime, record)
	case "aborted", "rejected":
		if err = isolateAbortedDraft(ctx, op, runtime, record); err != nil {
			return SaveDraftResult{}, err
		}
		if err = releaseDraftPin(ctx, op, runtime, record); err != nil {
			return SaveDraftResult{}, err
		}
		return SaveDraftResult{}, platform.NewError(platform.CodeConflict, false, "草稿保存意图已终止，请重新读取后用新操作保存")
	case "pending":
	default:
		return SaveDraftResult{}, platform.NewError(platform.CodeIntegrityError, false, "草稿保存意图状态未知")
	}
	if found {
		if err = checkPendingDraftSave(ctx, op, runtime, record); err != nil {
			return finishDraftSaveFailure(ctx, op, runtime, record, err)
		}
		_, limit, err = readDraftSiteConfig(ctx, op, runtime)
		if err != nil {
			return SaveDraftResult{}, err
		}
	}
	// 来源材料已在Acquire之前持久；即使网络结果未知，media可同步读取pending决定。
	record, err = acquireDraftPin(ctx, op, runtime, record)
	if err != nil {
		return SaveDraftResult{}, err
	}
	if record.State == "committed" {
		return replaySavedDraft(ctx, op, runtime, record)
	}
	draft, err := executeDraftCommit(ctx, op, runtime, record, input.Body, limit)
	if err != nil {
		return finishDraftSaveFailure(ctx, op, runtime, record, err)
	}
	if err = confirmDraftPin(ctx, op, runtime, record); err != nil {
		return SaveDraftResult{}, err
	}
	return SaveDraftResult{Draft: draft}, nil
}

// pending重试只在短guard内核当前lease和原CAS；释放后才能开始RPC或正文IO。
func checkPendingDraftSave(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation) error {
	guard, err := runtime.Manager.AcquireSaveFence(ctx, op, record.Plan.Fence)
	if err != nil {
		return err
	}
	defer guard.ReleaseFence()
	_, draft, err := runtime.Store.readDraftSnapshot(ctx, op, record.Plan.DocumentID)
	if err != nil {
		return err
	}
	if draft.RevisionId != *record.Plan.ExpectedHead || draft.DraftVersion != record.Plan.ExpectedDraftVersion || guard.state.ReferenceVersion != record.Plan.ExpectedReferenceVersion {
		return platform.NewError(platform.CodeVersionMismatch, false, "待保存操作的共享草稿或引用水位已变化")
	}
	return nil
}

func finishDraftSaveFailure(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation, cause error) (SaveDraftResult, error) {
	// Commit/IO未知或完整性损坏保留保护；明确配额/语义拒绝或CAS/锁拒绝才可封存。
	var failure *platform.Error
	if !errors.As(cause, &failure) || (failure.Code != platform.CodeValidation && failure.Code != platform.CodeVersionMismatch && failure.Code != platform.CodeLocked && failure.Code != platform.CodeNotFound) {
		return SaveDraftResult{}, cause
	}
	work, cancel, err := draftWriteContext(ctx, op, runtime)
	if err != nil {
		return SaveDraftResult{}, err
	}
	terminal, err := runtime.Store.abortDraftOperation(work, op, record)
	cancel()
	if err != nil {
		return SaveDraftResult{}, err
	}
	if terminal.State == "committed" {
		return replaySavedDraft(ctx, op, runtime, terminal)
	}
	if err = isolateAbortedDraft(ctx, op, runtime, terminal); err != nil {
		return SaveDraftResult{}, err
	}
	if err = releaseDraftPin(ctx, op, runtime, terminal); err != nil {
		return SaveDraftResult{}, err
	}
	return SaveDraftResult{}, cause
}

func planDraftSave(ctx context.Context, op OperationContext, runtime DraftRuntime, input SaveDraftInput, bodyHash Hash) (draftOperationPlan, error) {
	state, err := runtime.Store.ReadDocumentEditState(ctx, op, input.DocumentId)
	if err != nil {
		return draftOperationPlan{}, err
	}
	fence := DraftFenceInput{DocumentID: input.DocumentId, LeaseID: input.LeaseId, FencingToken: int64(input.FencingToken), ServerBootID: input.ServerBootId, ExpectedDeleteFence: state.DeleteFence}
	guard, err := runtime.Manager.AcquireSaveFence(ctx, op, fence)
	if err != nil {
		return draftOperationPlan{}, err
	}
	// 初始检查只跨SQL快照，不跨正文/网络；最终提交还会重新获取真实guard。
	_, draft, err := runtime.Store.readDraftSnapshot(ctx, op, input.DocumentId)
	referenceVersion := guard.state.ReferenceVersion
	guard.ReleaseFence()
	if err != nil {
		return draftOperationPlan{}, err
	}
	if referenceVersion < 1 {
		return draftOperationPlan{}, platform.NewError(platform.CodeIntegrityError, false, "共享草稿缺少真实文档引用水位")
	}
	if draft.RevisionId != input.ExpectedDraftRevisionId || draft.DraftVersion != input.ExpectedDraftVersion || input.ExpectedDraftVersion >= int(maxEditFence) || referenceVersion >= maxEditFence {
		return draftOperationPlan{}, platform.NewError(platform.CodeVersionMismatch, false, "共享草稿版本或head已变化")
	}
	ids, err := ValidateBlockTree(ctx, op, ValidateBlockTreeInput{DocumentID: input.DocumentId, Body: input.Body})
	if err != nil {
		return draftOperationPlan{}, err
	}
	revision := platform.NewIdentifier()
	refs, err := CollectResourceReferences(CollectResourceReferencesInput{DocumentID: input.DocumentId, RevisionID: revision, SourceVersion: int(referenceVersion + 1), Body: input.Body, Kind: "current_draft"})
	if err != nil {
		return draftOperationPlan{}, err
	}
	mediaRefs, digest, err := draftMediaReferences(refs.References)
	if err != nil {
		return draftOperationPlan{}, err
	}
	head := input.ExpectedDraftRevisionId
	return draftOperationPlan{DocumentID: input.DocumentId, RevisionID: revision, BodyHash: bodyHash, ExpectedHead: &head, ExpectedDraftVersion: input.ExpectedDraftVersion, ExpectedReferenceVersion: referenceVersion, SourceVersion: int(referenceVersion + 1), Fence: fence, BlockIDs: ids.BlockIDs, References: refs.References, MediaReferences: mediaRefs, ReferenceDigest: digest}, nil
}
func executeDraftCommit(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation, body DocumentBody, limit int64) (DraftRevision, error) {
	plan := record.Plan
	files := runtime.Store.ConfigureRevisionStorage(RevisionStorageConfig{MaxDocumentBytes: limit, WriteWindow: runtime.WriteWindow})
	prepared, err := WriteRevisionFile(WithRevisionFileStore(ctx, files), op, WriteRevisionFileInput{DocumentID: plan.DocumentID, RevisionID: plan.RevisionID, Body: body, ExpectedHead: plan.ExpectedHead, ExpectedDraftVersion: plan.ExpectedDraftVersion})
	if err != nil {
		return DraftRevision{}, err
	}
	if prepared.Draft.BodyHash != plan.BodyHash {
		return DraftRevision{}, platform.NewError(platform.CodeIntegrityError, false, "草稿正文与持久意图不一致")
	}
	store := &draftCommitStore{RevisionFileStore: files, runtime: runtime, operation: op, record: record}
	committed, err := CommitDraftJournal(WithDraftCommitStore(ctx, store), op, CommitDraftJournalInput{Draft: prepared.Draft, Journal: prepared.Journal, ExpectedHead: plan.ExpectedHead, ExpectedDraftVersion: plan.ExpectedDraftVersion})
	return committed.Draft, err
}

// draftMediaReference是media API的最小DTO投影，不导入media内部实现。
type draftMediaReference struct {
	SourceService    string              `json:"sourceService"`
	SourceObjectID   Identifier          `json:"sourceObjectId"`
	SourceRevisionID Identifier          `json:"sourceRevisionId"`
	SourceVersion    int                 `json:"sourceVersion"`
	ReferenceID      Identifier          `json:"referenceId"`
	Asset            gen.AssetVersionRef `json:"asset"`
	Kind             string              `json:"kind"`
	Confirmed        bool                `json:"confirmed"`
	ExpiresAt        *time.Time          `json:"expiresAt"`
}

func draftMediaReferences(refs []gen.ResourceReference) ([]draftMediaReference, Hash, error) {
	result := make([]draftMediaReference, 0, len(refs))
	for _, ref := range refs {
		key, _ := platform.CanonicalJSON(struct {
			Document, Revision, Block, Asset Identifier
			Version                          int
		}{ref.DocumentId, ref.SourceRevisionId, ref.BlockId, ref.Asset.AssetId, ref.Asset.Version})
		// Identifier允许sha256，跨块/资产不会混淆同一引用身份。
		result = append(result, draftMediaReference{SourceService: "content", SourceObjectID: ref.DocumentId, SourceRevisionID: ref.SourceRevisionId, SourceVersion: ref.SourceVersion, ReferenceID: platform.HashBytes(key), Asset: ref.Asset, Kind: "pending_pin"})
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.ReferenceID != b.ReferenceID {
			return a.ReferenceID < b.ReferenceID
		}
		if a.Asset.AssetId != b.Asset.AssetId {
			return a.Asset.AssetId < b.Asset.AssetId
		}
		if a.Asset.Version != b.Asset.Version {
			return a.Asset.Version < b.Asset.Version
		}
		return a.Asset.Sha256 < b.Asset.Sha256
	})
	raw, err := platform.CanonicalJSON(result)
	if err != nil {
		return nil, "", err
	}
	return result, platform.HashBytes(raw), nil
}

type draftPinMaterial struct {
	SourceOperationID Identifier `json:"sourceOperationId"`
	SiteID            Identifier `json:"siteId"`
	SourceObjectID    Identifier `json:"sourceObjectId"`
	SourceRevisionID  Identifier `json:"sourceRevisionId"`
	SourceVersion     int        `json:"sourceVersion"`
	ReferenceDigest   Hash       `json:"referenceDigest"`
}

func draftPinSource(op OperationContext, record draftOperation) draftPinMaterial {
	p := record.Plan
	return draftPinMaterial{op.OperationID, op.Actor.SiteID, p.DocumentID, p.RevisionID, p.SourceVersion, p.ReferenceDigest}
}
func acquireDraftPin(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation) (draftOperation, error) {
	if record.PinID != nil {
		return record, nil
	}
	request := struct {
		draftPinMaterial
		References []draftMediaReference `json:"references"`
	}{draftPinSource(op, record), record.Plan.MediaReferences}
	raw, err := runtime.Media.AcquireResourceReferences(contentServiceContext(ctx, op), request)
	if err != nil {
		return record, err
	}
	var response struct {
		Pin struct {
			PinID         Identifier            `json:"pinId"`
			Assets        []gen.AssetVersionRef `json:"assets"`
			ExpiresAt     time.Time             `json:"expiresAt"`
			DeletionFence int64                 `json:"deletionFence"`
		} `json:"pin"`
	}
	if json.Unmarshal(raw, &response) != nil || !platform.ValidIdentifier(response.Pin.PinID) {
		return record, platform.NewError(platform.CodeIntegrityError, false, "资源pin响应不完整")
	}
	// 对照完整资产材料，不能把错误pin用于COMMITTED提交。
	wanted := map[Identifier]gen.AssetVersionRef{}
	for _, ref := range record.Plan.MediaReferences {
		wanted[ref.Asset.AssetId] = ref.Asset
	}
	if len(wanted) != len(response.Pin.Assets) {
		return record, platform.NewError(platform.CodeIntegrityError, false, "资源pin资产集合不一致")
	}
	for _, asset := range response.Pin.Assets {
		if expected, found := wanted[asset.AssetId]; !found || expected != asset {
			return record, platform.NewError(platform.CodeIntegrityError, false, "资源pin资产材料不一致")
		}
		delete(wanted, asset.AssetId)
	}
	work, cancel, err := draftWriteContext(ctx, op, runtime)
	if err != nil {
		return record, err
	}
	defer cancel()
	updated, err := runtime.Store.db.ExecContext(work, `UPDATE draft_operation_intents SET pin_id=? WHERE site_id=? AND data_epoch=? AND operation_id=? AND action='save' AND state='pending' AND request_hash=? AND (pin_id IS NULL OR pin_id=?)`, response.Pin.PinID, op.Actor.SiteID, op.DataEpoch, op.OperationID, record.RequestHash, response.Pin.PinID)
	if err != nil {
		return record, contentStorageError()
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return record, contentStorageError()
	}
	if count != 1 {
		current, found, readErr := runtime.Store.readDraftOperation(ctx, op, record.Action, record.RequestHash)
		if readErr != nil {
			return record, readErr
		}
		if found && current.State == "committed" {
			return current, nil
		}
		return record, platform.NewError(platform.CodeConflict, false, "资源pin意图已终止或改变")
	}
	pin := response.Pin.PinID
	record.PinID = &pin
	return record, nil
}
func confirmDraftPin(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation) error {
	if record.PinID == nil {
		return platform.NewError(platform.CodeIntegrityError, false, "已提交草稿缺少原资源pin")
	}
	raw, err := runtime.Media.ConfirmResourceReferences(contentServiceContext(ctx, op), struct {
		PinID            Identifier `json:"pinId"`
		SourceRevisionID Identifier `json:"sourceRevisionId"`
		SourceVersion    int        `json:"sourceVersion"`
	}{*record.PinID, record.Plan.RevisionID, record.Plan.SourceVersion})
	if err != nil {
		return err
	}
	var result struct {
		Confirmed bool `json:"confirmed"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.Confirmed {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "资源引用确认尚未完成")
	}
	return nil
}
func replaySavedDraft(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation) (SaveDraftResult, error) {
	var result SaveDraftResult
	if decodeStorageJSON(record.Result, &result) != nil || result.Draft.DocumentId != record.Plan.DocumentID || result.Draft.RevisionId != record.Plan.RevisionID || result.Draft.BodyHash != record.Plan.BodyHash {
		return result, platform.NewError(platform.CodeIntegrityError, false, "草稿保存原回执损坏")
	}
	if err := confirmDraftPin(ctx, op, runtime, record); err != nil {
		return SaveDraftResult{}, err
	}
	return result, nil
}
func (s *Store) abortDraftOperation(ctx context.Context, op OperationContext, record draftOperation) (draftOperation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return record, contentStorageError()
	}
	defer tx.Rollback()
	current, found, err := readDraftOperationFrom(ctx, tx, op, record.Action, record.RequestHash)
	if err != nil {
		return record, err
	}
	if !found {
		return record, platform.NewError(platform.CodeIntegrityError, false, "待封存草稿原意图缺失")
	}
	// 从当前同事务记录刷新完整计划、pin和原结果，不能携较早pending快照的nil pin回放。
	record = current
	if record.State == "committed" {
		_ = tx.Rollback()
		return record, nil
	}
	state := record.State
	var committed int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM committed_revisions WHERE site_id=? AND document_id=? AND revision_id=? AND data_epoch=?`, op.Actor.SiteID, record.Plan.DocumentID, record.Plan.RevisionID, op.DataEpoch).Scan(&committed); err != nil {
		return record, contentStorageError()
	}
	if committed != 0 {
		return record, platform.NewError(platform.CodeIntegrityError, false, "正文已有COMMITTED资格，禁止终止或释放资源")
	}
	if state == "pending" {
		if _, err = tx.ExecContext(ctx, `UPDATE draft_operation_intents SET state='aborted' WHERE site_id=? AND data_epoch=? AND operation_id=? AND state='pending'`, op.Actor.SiteID, op.DataEpoch, op.OperationID); err != nil {
			return record, contentStorageError()
		}
		if _, err = tx.ExecContext(ctx, `UPDATE revision_preparations SET state='aborted' WHERE site_id=? AND data_epoch=? AND operation_id=? AND state IN ('preparing','prepared')`, op.Actor.SiteID, op.DataEpoch, op.OperationID); err != nil {
			return record, contentStorageError()
		}
		if err = tx.Commit(); err != nil {
			return record, contentStorageError()
		}
		record.State = "aborted"
		return record, nil
	}
	record.State = state
	return record, nil
}
func releaseDraftPin(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation) error {
	if record.State != "aborted" && record.State != "rejected" {
		return platform.NewError(platform.CodeConflict, false, "未知或COMMITTED源不能释放资源")
	}
	reason := "source_aborted"
	if record.State == "rejected" {
		reason = "source_rejected"
	}
	raw, err := runtime.Media.ReleaseResourceReferences(contentServiceContext(ctx, op), struct {
		draftPinMaterial
		PinID  *Identifier `json:"pinId"`
		Reason string      `json:"reason"`
	}{draftPinSource(op, record), record.PinID, reason})
	if err != nil {
		return err
	}
	var result struct {
		Released bool `json:"released"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.Released {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "资源来源尚未封存")
	}
	return nil
}

// 源已同事务终止且禁止迟到COMMITTED后，才把该次未提交正文移入隔离区。
func isolateAbortedDraft(ctx context.Context, op OperationContext, runtime DraftRuntime, record draftOperation) error {
	var raw []byte
	var state string
	err := runtime.Store.db.QueryRowContext(ctx, `SELECT p.result_json,p.state FROM revision_preparations p WHERE p.site_id=? AND p.data_epoch=? AND p.operation_id=? AND p.document_id=? AND p.revision_id=? AND NOT EXISTS(SELECT 1 FROM committed_revisions c WHERE c.commit_id=p.commit_id)`, op.Actor.SiteID, op.DataEpoch, op.OperationID, record.Plan.DocumentID, record.Plan.RevisionID).Scan(&raw, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return contentStorageError()
	}
	if state != "aborted" && state != "quarantined" {
		return platform.NewError(platform.CodeIntegrityError, false, "未终止准备不能移入终止隔离")
	}
	var prepared WriteRevisionFileResult
	if decodeStorageJSON(raw, &prepared) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "终止正文原始准备损坏")
	}
	root := runtime.Store.root
	if err = moveBodyToQuarantine(ctx, root, op.Actor.SiteID, prepared.Journal, revisionBodyPath(root, op.Actor.SiteID, record.Plan.DocumentID, record.Plan.RevisionID)); err != nil {
		return err
	}
	// 原8字段日志不改格式；重复quarantined是允许的同材料终态。
	return appendJournalState(ctx, filepath.Join(root, journalRelativePath), prepared.Journal, "quarantined")
}
