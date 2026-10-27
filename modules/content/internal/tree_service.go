package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"sort"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/content/internal/gen"
)

type ReadDocumentTreeInput struct{}
type ReadDocumentTreeResult = gen.ReadDocumentTreeResult
type ListDocumentsInput struct {
	Page  ReferencePageRequest `json:"page"`
	State string               `json:"state"`
}
type ListDocumentsResult = gen.ListDocumentsResult
type MoveDocumentInput = gen.MoveDocumentInput
type MoveDocumentResult = gen.MoveDocumentResult
type ReorderChildrenInput = gen.ReorderChildrenInput
type ReorderChildrenResult = gen.ReorderChildrenResult

// TreeRuntime只接受启动装配的真实存储、鉴权/游标签名器和当前写窗口。
// Media沿用草稿提交的版本化客户端；空引用发布不调用它。
type TreeRuntime struct {
	Reference      ReferenceRuntime
	WriteWindow    platform.WriteWindowConfig
	validateResult map[string]func(json.RawMessage) error
	Media          *gen.MediaClient
	Models         *ModelRuntime
}
type treeRuntimeKey struct{}

func NewTreeRuntime(reference ReferenceRuntime, window platform.WriteWindowConfig) (TreeRuntime, error) {
	spec, err := gen.GetSwagger()
	if err != nil {
		return TreeRuntime{}, platform.NewError(platform.CodeIntegrityError, false, "文档树规格不可读")
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return TreeRuntime{}, platform.NewError(platform.CodeIntegrityError, false, "文档树规格无法编码")
	}
	validators := make(map[string]func(json.RawMessage) error)
	for action, schema := range map[string]string{"move": "MoveDocumentResult", "reorder": "ReorderChildrenResult", "metadata": "UpdateDocumentMetadataResult", "publish": "PublishDocumentResult", "withdraw": "WithdrawDocumentResult"} {
		validators[action], err = apivalidate.NewPayloadValidator(raw, schema)
		if err != nil {
			return TreeRuntime{}, err
		}
	}
	return TreeRuntime{Reference: reference, WriteWindow: window, validateResult: validators}, nil
}
func WithTreeRuntime(ctx context.Context, runtime TreeRuntime) context.Context {
	return context.WithValue(ctx, treeRuntimeKey{}, runtime)
}
func treeRuntimeFromContext(ctx context.Context) (TreeRuntime, error) {
	runtime, ok := ctx.Value(treeRuntimeKey{}).(TreeRuntime)
	if !ok || runtime.Reference.Store == nil || runtime.Reference.Identity == nil || runtime.Reference.validateMetadata == nil || runtime.validateResult["move"] == nil || runtime.validateResult["reorder"] == nil || runtime.validateResult["metadata"] == nil || runtime.validateResult["publish"] == nil || runtime.validateResult["withdraw"] == nil {
		return runtime, platform.NewError(platform.CodeDependencyUnavailable, true, "文档树运行依赖未装配")
	}
	return runtime, nil
}

func authorizeTree(ctx context.Context, op OperationContext, runtime TreeRuntime, permission, action string) error {
	if op.CallerService != "portal" || op.Actor.Kind != "admin" {
		return platform.NewError(platform.CodeForbidden, false, "文档树仅允许当前管理账户")
	}
	return apivalidate.NewIdentityAuthorizer(runtime.Reference.Identity)(platform.WithOperationContext(ctx, op), op, permission, "content:"+action)
}

func ReadDocumentTree(ctx context.Context, op OperationContext, input ReadDocumentTreeInput) (ReadDocumentTreeResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return ReadDocumentTreeResult{}, err
	}
	if err = authorizeTree(ctx, op, runtime, "content.read", "readDocumentTree"); err != nil {
		return ReadDocumentTreeResult{}, err
	}
	tx, err := beginTreeRead(ctx, op, runtime)
	if err != nil {
		return ReadDocumentTreeResult{}, err
	}
	defer tx.Rollback()
	version, documents, err := readTreeSnapshot(ctx, tx, op, runtime)
	if err != nil {
		return ReadDocumentTreeResult{}, err
	}
	return ReadDocumentTreeResult{Documents: treePreorder(documents, false), TreeRevision: int(version)}, nil
}

type treeListCursor struct {
	TreeRevision   int64 `json:"treeRevision"`
	SourceRevision int64 `json:"sourceRevision"`
	Offset         int   `json:"offset"`
}

func ListDocuments(ctx context.Context, op OperationContext, input ListDocumentsInput) (ListDocumentsResult, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return ListDocumentsResult{}, err
	}
	permission := "content.read"
	if input.State == "trash" {
		permission = "content.publish"
	}
	if err = authorizeTree(ctx, op, runtime, permission, "listDocuments"); err != nil {
		return ListDocumentsResult{}, err
	}
	if input.Page.Limit == 0 {
		input.Page.Limit = 20
	}
	binding, _ := platform.CanonicalJSON(struct {
		Operation            platform.AuthContext
		Caller, Epoch, State string
		Limit                int
	}{op.Actor, op.CallerService, op.DataEpoch, input.State, input.Page.Limit})
	scope := "content.documents.v1:" + platform.HashBytes(binding)
	position := treeListCursor{}
	if input.Page.Cursor != "" {
		raw, err := serviceauth.VerifyCursor(runtime.Reference.SigningKey, scope, input.Page.Cursor)
		if err != nil {
			return ListDocumentsResult{}, err
		}
		if decodeStorageJSON(raw, &position) != nil || position.TreeRevision < 0 || position.SourceRevision < 1 || position.Offset < 1 || position.Offset > 10000 {
			return ListDocumentsResult{}, platform.NewError(platform.CodeValidation, false, "文档列表游标载荷不可用")
		}
	}
	tx, err := beginTreeRead(ctx, op, runtime)
	if err != nil {
		return ListDocumentsResult{}, err
	}
	defer tx.Rollback()
	version, documents, err := readTreeSnapshot(ctx, tx, op, runtime)
	if err != nil {
		return ListDocumentsResult{}, err
	}
	var sourceRevision int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM reference_scan_state WHERE singleton=1`).Scan(&sourceRevision); err != nil {
		return ListDocumentsResult{}, contentStorageError()
	}
	if input.Page.Cursor != "" && (position.TreeRevision != version || position.SourceRevision != sourceRevision) {
		return ListDocumentsResult{}, platform.NewError(platform.CodeVersionMismatch, true, "文档树或元数据来源水位已变化，请从第一页重读")
	}
	_ = tx.Rollback()
	ordered := treePreorder(documents, true)
	selected := make([]gen.DocumentMetadata, 0)
	for _, document := range ordered {
		if string(document.State) == input.State {
			selected = append(selected, document)
		}
	}
	if position.Offset > len(selected) {
		return ListDocumentsResult{}, platform.NewError(platform.CodeValidation, false, "文档列表游标位置不可用")
	}
	end := min(position.Offset+input.Page.Limit, len(selected))
	result := ListDocumentsResult{Documents: selected[position.Offset:end], PageInfo: gen.PageInfo{HasMore: end < len(selected)}}
	if result.PageInfo.HasMore {
		raw, _ := json.Marshal(treeListCursor{TreeRevision: version, SourceRevision: sourceRevision, Offset: end})
		cursor, err := serviceauth.SignCursor(runtime.Reference.SigningKey, scope, raw)
		if err != nil {
			return ListDocumentsResult{}, err
		}
		result.PageInfo.NextCursor = &cursor
	}
	return result, nil
}

func beginTreeRead(ctx context.Context, op OperationContext, runtime TreeRuntime) (*sql.Tx, error) {
	if err := runtime.Reference.Store.checkRevisionScope(ctx, op); err != nil {
		return nil, err
	}
	tx, err := runtime.Reference.Store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, contentStorageError()
	}
	return tx, nil
}

// readTreeSnapshot读取同一SQL快照；当前正文与元数据一致，损坏不输出部分树。
func readTreeSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, runtime TreeRuntime) (int64, []gen.DocumentMetadata, error) {
	var epoch string
	var site sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch,site_id FROM store_identity WHERE singleton=1`).Scan(&epoch, &site); err != nil {
		return 0, nil, contentStorageError()
	}
	if epoch != op.DataEpoch {
		return 0, nil, platform.NewError(platform.CodeVersionMismatch, false, "文档树持久代次已变化")
	}
	if site.Valid && site.String != op.Actor.SiteID {
		return 0, nil, platform.NewError(platform.CodeForbidden, false, "文档树持久站点不匹配")
	}
	version, documents, err := readCreationTree(ctx, tx, op.Actor.SiteID, runtime.Reference.validateMetadata)
	if err != nil {
		return 0, nil, err
	}
	if len(documents) > 10000 {
		return 0, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "完整文档树超过登记预算")
	}
	if _, err = EnsureTreeAcyclic(EnsureTreeAcyclicInput{Documents: documents}); err != nil {
		return 0, nil, platform.NewError(platform.CodeIntegrityError, false, "持久文档树结构损坏")
	}
	var inconsistent int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents d LEFT JOIN draft_heads h ON h.site_id=d.site_id AND h.document_id=d.document_id LEFT JOIN document_metadata m ON m.site_id=d.site_id AND m.document_id=d.document_id WHERE d.site_id=? AND d.state<>'purged' AND (h.document_id IS NULL OR json_extract(m.metadata_json,'$.draftRevisionId') IS NOT h.revision_id OR NOT EXISTS(SELECT 1 FROM committed_revisions c JOIN draft_commit_receipts r ON r.commit_id=c.commit_id WHERE c.site_id=h.site_id AND c.document_id=h.document_id AND c.revision_id=h.revision_id AND c.data_epoch=?))`, op.Actor.SiteID, op.DataEpoch).Scan(&inconsistent)
	if err != nil {
		return 0, nil, contentStorageError()
	}
	if inconsistent != 0 {
		return 0, nil, platform.NewError(platform.CodeIntegrityError, false, "文档树共享head或已提交资格缺失")
	}
	if err = validateTreeSiblings(documents); err != nil {
		return 0, nil, platform.NewError(platform.CodeIntegrityError, false, "持久同父节点顺序或路径冲突")
	}
	return version, documents, nil
}

// readDocuments 只读指定的几篇文档，逐篇校验与整树快照相同（代次、站点、元数据 schema、隔离状态）。
// 读已发布正文、核对发布引用只关心这几篇，不必每次读出并校验整棵树；不存在的文档不在结果里。
func readDocuments(ctx context.Context, tx *sql.Tx, op OperationContext, runtime TreeRuntime, ids []Identifier) (map[Identifier]gen.DocumentMetadata, error) {
	var epoch string
	var site sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch,site_id FROM store_identity WHERE singleton=1`).Scan(&epoch, &site); err != nil {
		return nil, contentStorageError()
	}
	if epoch != op.DataEpoch {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "文档树持久代次已变化")
	}
	if site.Valid && site.String != op.Actor.SiteID {
		return nil, platform.NewError(platform.CodeForbidden, false, "文档树持久站点不匹配")
	}
	result := make(map[Identifier]gen.DocumentMetadata, len(ids))
	for _, id := range ids {
		if _, done := result[id]; done {
			continue
		}
		var state string
		var revision int
		var raw []byte
		var quarantined bool
		err := tx.QueryRowContext(ctx, `SELECT d.state,d.revision,CASE WHEN length(CAST(m.metadata_json AS BLOB))<=16777216 THEN m.metadata_json ELSE NULL END,EXISTS(SELECT 1 FROM document_quarantines q WHERE q.site_id=d.site_id AND q.document_id=d.document_id) FROM documents d LEFT JOIN document_metadata m ON m.site_id=d.site_id AND m.document_id=d.document_id WHERE d.site_id=? AND d.document_id=? AND d.state<>'purged'`, op.Actor.SiteID, id).Scan(&state, &revision, &raw, &quarantined)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, contentStorageError()
		}
		if runtime.Reference.validateMetadata(raw) != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "文档持久元数据不符合唯一schema")
		}
		var metadata gen.DocumentMetadata
		if decodeStorageJSON(raw, &metadata) != nil || metadata.DocumentId != id || metadata.SiteId != op.Actor.SiteID || string(metadata.State) != state || metadata.Revision != revision || quarantined {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "文档元数据或隔离状态损坏")
		}
		result[id] = metadata
	}
	return result, nil
}

func validateTreeSiblings(documents []gen.DocumentMetadata) error {
	index := make(map[Identifier]gen.DocumentMetadata, len(documents))
	for _, document := range documents {
		index[document.DocumentId] = document
	}
	orders, paths := map[string]bool{}, map[string]bool{}
	for _, document := range documents {
		if document.State != "draft" && document.State != "published" {
			continue
		}
		parent := ""
		if document.ParentId != nil {
			parent = *document.ParentId
			p := index[parent]
			if p.State != "draft" && p.State != "published" {
				return platform.NewError(platform.CodeIntegrityError, false, "活动文档位于非活动父节点下")
			}
		}
		order := parent + "\x00" + document.OrderKey
		if orders[order] {
			return platform.NewError(platform.CodeIntegrityError, false, "同父活动文档顺序key重复")
		}
		orders[order] = true
		if document.PathSegment != nil {
			path := parent + "\x00" + *document.PathSegment
			if paths[path] {
				return platform.NewError(platform.CodeConflict, false, "同父活动文档路径冲突")
			}
			paths[path] = true
		}
	}
	return nil
}
func treePreorder(documents []gen.DocumentMetadata, includeInactive bool) []gen.DocumentMetadata {
	children := make(map[string][]gen.DocumentMetadata)
	for _, document := range documents {
		parent := ""
		if document.ParentId != nil {
			parent = *document.ParentId
		}
		children[parent] = append(children[parent], document)
	}
	for _, siblings := range children {
		sort.Slice(siblings, func(i, j int) bool {
			if siblings[i].OrderKey != siblings[j].OrderKey {
				return siblings[i].OrderKey < siblings[j].OrderKey
			}
			return siblings[i].DocumentId < siblings[j].DocumentId
		})
	}
	result := make([]gen.DocumentMetadata, 0, len(documents))
	var visit func(string)
	visit = func(parent string) {
		for _, document := range children[parent] {
			if includeInactive || document.State == "draft" || document.State == "published" {
				result = append(result, document)
			}
			visit(document.DocumentId)
		}
	}
	visit("")
	return result
}

func MoveDocument(ctx context.Context, op OperationContext, input MoveDocumentInput) (MoveDocumentResult, error) {
	raw, err := executeTreeMutation(ctx, op, "move", input, input.ExpectedTreeRevision)
	if err != nil {
		return MoveDocumentResult{}, err
	}
	var result MoveDocumentResult
	if decodeStorageJSON(raw, &result) != nil {
		return result, platform.NewError(platform.CodeIntegrityError, false, "移动文档原回执损坏")
	}
	return result, nil
}
func ReorderChildren(ctx context.Context, op OperationContext, input ReorderChildrenInput) (ReorderChildrenResult, error) {
	raw, err := executeTreeMutation(ctx, op, "reorder", input, input.ExpectedTreeRevision)
	if err != nil {
		return ReorderChildrenResult{}, err
	}
	var result ReorderChildrenResult
	if decodeStorageJSON(raw, &result) != nil {
		return result, platform.NewError(platform.CodeIntegrityError, false, "重排文档原回执损坏")
	}
	return result, nil
}

func readTreeReceipt(ctx context.Context, q creationQuery, op OperationContext, action string, digest Hash, runtime TreeRuntime) (json.RawMessage, bool, error) {
	var caller, actor, storedAction string
	var storedHash Hash
	var raw json.RawMessage
	err := q.QueryRowContext(ctx, `SELECT caller_service,actor_id,action,request_hash,result_json FROM tree_operation_receipts WHERE site_id=? AND data_epoch=? AND operation_id=?`, op.Actor.SiteID, op.DataEpoch, op.OperationID).Scan(&caller, &actor, &storedAction, &storedHash, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, contentStorageError()
	}
	if caller != op.CallerService || actor != op.Actor.SubjectID || storedAction != action || storedHash != digest {
		return nil, false, platform.NewError(platform.CodeConflict, false, "树操作身份、动作或摘要与原回执冲突")
	}
	if runtime.validateResult[action](raw) != nil {
		return nil, false, platform.NewError(platform.CodeIntegrityError, false, "树操作完整原回执不符合唯一schema")
	}
	return raw, true, nil
}

func executeTreeMutation(ctx context.Context, op OperationContext, action string, input any, expected int) (json.RawMessage, error) {
	runtime, err := treeRuntimeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	operation := "moveDocument"
	if action == "reorder" {
		operation = "reorderChildren"
	}
	if err = authorizeTree(ctx, op, runtime, "content.edit", operation); err != nil {
		return nil, err
	}
	if err = runtime.Reference.Store.checkRevisionScope(ctx, op); err != nil {
		return nil, err
	}
	digest, err := draftRequestDigest("tree."+action, input)
	if err != nil {
		return nil, err
	}
	if raw, found, err := readTreeReceipt(ctx, runtime.Reference.Store.db, op, action, digest, runtime); err != nil || found {
		return raw, err
	}
	if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "文档树当前写窗口尚未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return nil, err
	}
	defer cancel()
	if view.DataEpoch != op.DataEpoch {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "树操作当前写窗口代次已变化")
	}
	runtime.Reference.Store.mu.Lock()
	defer runtime.Reference.Store.mu.Unlock()
	tx, err := runtime.Reference.Store.db.BeginTx(work, nil)
	if err != nil {
		return nil, contentStorageError()
	}
	defer tx.Rollback()
	if raw, found, err := readTreeReceipt(work, tx, op, action, digest, runtime); err != nil || found {
		return raw, err
	}
	version, documents, err := readTreeSnapshot(work, tx, op, runtime)
	if err != nil {
		return nil, err
	}
	if version != int64(expected) || version >= maxEditFence {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "文档树版本已变化或耗尽")
	}
	var changed []gen.DocumentMetadata
	var redirects []gen.RouteTombstone
	var result any
	if action == "move" {
		changed, redirects, err = planTreeMove(documents, input.(MoveDocumentInput), version+1)
		if redirects == nil {
			redirects = []gen.RouteTombstone{}
		}
		result = MoveDocumentResult{TreeRevision: int(version + 1), Redirects: redirects}
	} else {
		changed, err = planTreeReorder(documents, input.(ReorderChildrenInput))
		result = ReorderChildrenResult{TreeRevision: int(version + 1)}
	}
	if err != nil {
		return nil, err
	}
	binding, err := tx.ExecContext(work, `UPDATE store_identity SET site_id=? WHERE singleton=1 AND data_epoch=? AND (site_id IS NULL OR site_id=?)`, op.Actor.SiteID, op.DataEpoch, op.Actor.SiteID)
	if err != nil {
		return nil, contentStorageError()
	}
	bound, err := binding.RowsAffected()
	if err != nil || bound != 1 {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "树操作站点或代次已变化")
	}
	updated, err := tx.ExecContext(work, `INSERT INTO content_tree_state(site_id,revision) VALUES(?,1) ON CONFLICT(site_id) DO UPDATE SET revision=revision+1 WHERE revision=?`, op.Actor.SiteID, version)
	if err != nil {
		return nil, contentStorageError()
	}
	count, err := updated.RowsAffected()
	if err != nil || count != 1 {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "文档树CAS失败")
	}
	ids := make([]Identifier, 0, len(changed))
	for _, document := range changed {
		if int64(document.Revision) >= maxEditFence {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "文档元数据修订耗尽")
		}
		document.Revision++
		raw, _ := json.Marshal(document)
		updated, err = tx.ExecContext(work, `UPDATE documents SET revision=? WHERE site_id=? AND document_id=? AND revision=?`, document.Revision, op.Actor.SiteID, document.DocumentId, document.Revision-1)
		if err != nil {
			return nil, contentStorageError()
		}
		count, err = updated.RowsAffected()
		if err != nil || count != 1 {
			return nil, platform.NewError(platform.CodeVersionMismatch, false, "文档元数据CAS失败")
		}
		if _, err = tx.ExecContext(work, `UPDATE document_metadata SET metadata_json=? WHERE site_id=? AND document_id=?`, string(raw), op.Actor.SiteID, document.DocumentId); err != nil {
			return nil, contentStorageError()
		}
		ids = append(ids, document.DocumentId)
	}
	if action == "move" && len(redirects) > 0 {
		next := slices.Clone(documents)
		replaced := map[Identifier]gen.DocumentMetadata{}
		for _, document := range changed {
			replaced[document.DocumentId] = document
		}
		for i := range next {
			if updated, ok := replaced[next[i].DocumentId]; ok {
				next[i] = updated
			}
		}
		stored, manifestErr := commitRedirectManifest(work, tx, op, redirects, next)
		if manifestErr != nil {
			return nil, manifestErr
		}
		if stored == nil {
			stored = []gen.RouteTombstone{}
		}
		result = MoveDocumentResult{TreeRevision: int(version + 1), Redirects: stored}
	}
	raw, _ := json.Marshal(result)
	if _, err = tx.ExecContext(work, `INSERT INTO tree_operation_receipts(site_id,data_epoch,operation_id,caller_service,actor_id,action,request_hash,result_json) VALUES(?,?,?,?,?,?,?,?)`, op.Actor.SiteID, op.DataEpoch, op.OperationID, op.CallerService, op.Actor.SubjectID, action, digest, string(raw)); err != nil {
		return nil, contentStorageError()
	}
	payload, _ := json.Marshal(struct {
		SiteID             Identifier   `json:"siteId"`
		TreeRevision       int64        `json:"treeRevision"`
		ChangedDocumentIDs []Identifier `json:"changedDocumentIds"`
		DataEpoch          Identifier   `json:"dataEpoch"`
	}{op.Actor.SiteID, version + 1, ids, op.DataEpoch})
	if err = queue.EnqueueEvent(tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: "content.tree.changed", EventVersion: 1, Producer: "content", SubjectID: op.Actor.SiteID, Revision: version + 1, OccurredAt: platform.Now(), DataEpoch: op.DataEpoch, Payload: payload}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, contentStorageError()
	}
	return raw, nil
}

func planTreeMove(documents []gen.DocumentMetadata, input MoveDocumentInput, revision int64) ([]gen.DocumentMetadata, []gen.RouteTombstone, error) {
	var moving *gen.DocumentMetadata
	redirects := []gen.RouteTombstone{}
	for i := range documents {
		if documents[i].DocumentId == input.DocumentId {
			copy := documents[i]
			moving = &copy
		}
	}
	if moving == nil {
		return nil, nil, platform.NewError(platform.CodeNotFound, false, "拟移动文档不存在")
	}
	if moving.State != "draft" && moving.State != "published" {
		return nil, nil, platform.NewError(platform.CodeLocked, false, "只能移动活动文档")
	}
	if _, err := EnsureTreeAcyclic(EnsureTreeAcyclicInput{Documents: documents, MovingID: &input.DocumentId, ParentID: input.NewParentId}); err != nil {
		return nil, nil, err
	}
	if input.NewParentId != nil {
		for _, document := range documents {
			if document.DocumentId == *input.NewParentId && document.State != "draft" && document.State != "published" {
				return nil, nil, platform.NewError(platform.CodeLocked, false, "拟父文档不是活动文档")
			}
		}
	}
	if !sameIdentifierPointer(moving.ParentId, input.NewParentId) {
		planned, err := PlanOldPathRedirects(PlanOldPathRedirectsInput{DocumentId: input.DocumentId, Documents: documents, TreeRevision: int(revision)})
		if err != nil {
			return nil, nil, err
		}
		redirects = planned.Redirects
	}
	siblings := make([]gen.DocumentMetadata, 0)
	for _, document := range documents {
		if document.DocumentId != input.DocumentId && sameIdentifierPointer(document.ParentId, input.NewParentId) && (document.State == "draft" || document.State == "published") {
			siblings = append(siblings, document)
		}
	}
	sort.Slice(siblings, func(i, j int) bool { return siblings[i].OrderKey < siblings[j].OrderKey })
	position := len(siblings)
	if input.BeforeDocumentId != nil {
		position = -1
		for i, document := range siblings {
			if document.DocumentId == *input.BeforeDocumentId {
				position = i
				break
			}
		}
		if position < 0 {
			return nil, nil, platform.NewError(platform.CodeValidation, false, "相邻文档必须属于新的同父活动集合")
		}
	}
	lower, upper := "", ""
	if position > 0 {
		lower = siblings[position-1].OrderKey
	}
	if position < len(siblings) {
		upper = siblings[position].OrderKey
	}
	key, err := fractionalDocumentOrder(lower, upper)
	if err != nil {
		return nil, nil, err
	}
	moving.ParentId = input.NewParentId
	moving.OrderKey = key
	candidate := slices.Clone(documents)
	for i := range candidate {
		if candidate[i].DocumentId == input.DocumentId {
			candidate[i] = *moving
		}
	}
	if err = validateTreeSiblings(candidate); err != nil {
		return nil, nil, err
	}
	if moving.PathSegment != nil {
		if _, err = validatePathSnapshot(ValidatePathSegmentInput{Segment: *moving.PathSegment, ParentId: moving.ParentId, DocumentId: moving.DocumentId, Documents: candidate}); err != nil {
			return nil, nil, err
		}
	}
	if err = ensurePublishedAncestorPaths(candidate, moving.DocumentId); err != nil {
		return nil, nil, err
	}
	return []gen.DocumentMetadata{*moving}, redirects, nil
}

func planTreeReorder(documents []gen.DocumentMetadata, input ReorderChildrenInput) ([]gen.DocumentMetadata, error) {
	if input.ParentId != nil {
		found := false
		for _, document := range documents {
			if document.DocumentId == *input.ParentId {
				found = true
				if document.State != "draft" && document.State != "published" {
					return nil, platform.NewError(platform.CodeLocked, false, "重排父文档不是活动文档")
				}
			}
		}
		if !found {
			return nil, platform.NewError(platform.CodeNotFound, false, "重排父文档不存在")
		}
	}
	children := map[Identifier]gen.DocumentMetadata{}
	for _, document := range documents {
		if sameIdentifierPointer(document.ParentId, input.ParentId) && (document.State == "draft" || document.State == "published") {
			children[document.DocumentId] = document
		}
	}
	if len(children) != len(input.OrderedDocumentIds) {
		return nil, platform.NewError(platform.CodeValidation, false, "重排必须包含完整同父活动节点集合")
	}
	changed := make([]gen.DocumentMetadata, 0, len(children))
	for i, id := range input.OrderedDocumentIds {
		document, found := children[id]
		if !found {
			return nil, platform.NewError(platform.CodeValidation, false, "重排包含重复、缺失或跨层节点")
		}
		delete(children, id)
		document.OrderKey = reorderedDocumentKey(i)
		changed = append(changed, document)
	}
	return changed, nil
}

// reorderedDocumentKey在现有base62字典序协议中均匀分配6位key，给后续插入保留间隔。
func reorderedDocumentKey(index int) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	value := (index + 1) * 4096
	key := []byte("000000")
	for i := len(key) - 1; i >= 0; i-- {
		key[i] = alphabet[value%len(alphabet)]
		value /= len(alphabet)
	}
	return string(key)
}
