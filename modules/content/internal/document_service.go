package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/content/internal/gen"
)

type CreateDocumentInput = gen.CreateDocumentInput
type CreateDocumentResult = gen.CreateDocumentResult

func CreateDocument(ctx context.Context, op OperationContext, input CreateDocumentInput) (CreateDocumentResult, error) {
	runtime, err := draftRuntimeFromContext(ctx)
	if err != nil {
		return CreateDocumentResult{}, err
	}
	if runtime.Identity == nil || runtime.SiteConfig == nil {
		return CreateDocumentResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "创建文档授权或站点配置未装配")
	}
	if _, err = authorizeEditLock(ctx, op, runtime.Identity, "content.edit", "createDocument", op.Actor.SiteID); err != nil {
		return CreateDocumentResult{}, err
	}
	digest, err := draftRequestDigest("create", input)
	if err != nil {
		return CreateDocumentResult{}, err
	}
	record, found, err := runtime.Store.readDraftOperation(ctx, op, "create", digest)
	if err != nil {
		return CreateDocumentResult{}, err
	}
	if found && record.State == "committed" {
		var result CreateDocumentResult
		if decodeStorageJSON(record.Result, &result) != nil {
			return result, platform.NewError(platform.CodeIntegrityError, false, "创建文档原回执损坏")
		}
		return result, nil
	}
	body := DocumentBody{Type: "doc", SchemaVersion: 1, Content: []gen.Block{}}
	if !found {
		language, _, err := readDraftSiteConfig(ctx, op, runtime)
		if err != nil {
			return CreateDocumentResult{}, err
		}
		plan, err := runtime.Store.planDocumentCreation(ctx, op, input, language, body)
		if err != nil {
			return CreateDocumentResult{}, err
		}
		work, cancel, err := draftWriteContext(ctx, op, runtime)
		if err != nil {
			return CreateDocumentResult{}, err
		}
		record, err = runtime.Store.prepareDraftOperation(work, op, draftOperation{Action: "create", RequestHash: digest, Plan: plan})
		cancel()
		if err != nil {
			return CreateDocumentResult{}, err
		}
	}
	if record.State == "committed" {
		var result CreateDocumentResult
		if decodeStorageJSON(record.Result, &result) != nil {
			return result, platform.NewError(platform.CodeIntegrityError, false, "创建文档原回执损坏")
		}
		return result, nil
	}
	if record.State != "pending" {
		return CreateDocumentResult{}, platform.NewError(platform.CodeConflict, false, "创建文档意图已终止")
	}
	// 重试先复用持久IDs与CAS快照，不重新生成文档或初始语言。
	_, limit, err := readDraftSiteConfig(ctx, op, runtime)
	if err != nil {
		return CreateDocumentResult{}, err
	}
	draft, err := executeDraftCommit(ctx, op, runtime, record, body, limit)
	if err != nil {
		return CreateDocumentResult{}, err
	}
	return CreateDocumentResult{Metadata: *record.Plan.Metadata, Draft: draft}, nil
}

func draftWriteContext(ctx context.Context, op OperationContext, runtime DraftRuntime) (context.Context, context.CancelFunc, error) {
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return nil, nil, err
	}
	if view.DataEpoch != op.DataEpoch || runtime.Store.epoch != op.DataEpoch {
		cancel()
		return nil, nil, platform.NewError(platform.CodeVersionMismatch, false, "草稿安全窗口代次已变化")
	}
	return work, cancel, nil
}
func contentServiceContext(ctx context.Context, op OperationContext) context.Context {
	op.CallerService = "content"
	op.Actor = platform.AuthContext{Kind: "service", SubjectID: "content", SiteID: op.Actor.SiteID, Grants: []string{}, ExpiresAt: op.Deadline}
	return platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
}
func readDraftSiteConfig(ctx context.Context, op OperationContext, runtime DraftRuntime) (string, int64, error) {
	raw, err := runtime.SiteConfig.ReadSiteConfig(contentServiceContext(ctx, op))
	if err != nil {
		return "", 0, err
	}
	var response struct {
		Config struct {
			SiteID         Identifier `json:"siteId"`
			BaseLanguage   string     `json:"baseLanguage"`
			Initialized    bool       `json:"initialized"`
			ResourceLimits struct {
				MaxDocumentBytes int64 `json:"maxDocumentBytes"`
			} `json:"resourceLimits"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Config.SiteID != op.Actor.SiteID || !response.Config.Initialized || response.Config.BaseLanguage == "" || response.Config.ResourceLimits.MaxDocumentBytes < 1024 || response.Config.ResourceLimits.MaxDocumentBytes > maxBodyBytes {
		return "", 0, platform.NewError(platform.CodeDependencyUnavailable, true, "草稿站点配置权威不可用")
	}
	return response.Config.BaseLanguage, response.Config.ResourceLimits.MaxDocumentBytes, nil
}

func (s *Store) planDocumentCreation(ctx context.Context, op OperationContext, input CreateDocumentInput, language string, body DocumentBody) (draftOperationPlan, error) {
	if err := s.checkRevisionScope(ctx, op); err != nil {
		return draftOperationPlan{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return draftOperationPlan{}, contentStorageError()
	}
	defer tx.Rollback()
	treeRevision, documents, err := readCreationTree(ctx, tx, op.Actor.SiteID, nil)
	if err != nil {
		return draftOperationPlan{}, err
	}
	id, revision := platform.NewIdentifier(), platform.NewIdentifier()
	metadata := gen.DocumentMetadata{DocumentId: id, SiteId: op.Actor.SiteID, ParentId: input.ParentId, Title: input.Title, SourceLanguage: language, SeoDescriptionSource: "pending", State: "draft", Revision: 1, VisibilityRevision: 0, DraftRevisionId: revision}
	siblings := make([]gen.DocumentMetadata, 0)
	for _, document := range documents {
		if sameIdentifierPointer(document.ParentId, input.ParentId) && document.State != "purged" {
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
			return draftOperationPlan{}, platform.NewError(platform.CodeValidation, false, "相邻文档不是当前父节点的子节点")
		}
	}
	lower, upper := "", ""
	if position > 0 {
		lower = siblings[position-1].OrderKey
	}
	if position < len(siblings) {
		upper = siblings[position].OrderKey
	}
	metadata.OrderKey, err = fractionalDocumentOrder(lower, upper)
	if err != nil {
		return draftOperationPlan{}, err
	}
	if input.ParentId != nil {
		parentFound := false
		for _, document := range documents {
			if document.DocumentId == *input.ParentId {
				parentFound = true
				if document.State != "draft" && document.State != "published" {
					return draftOperationPlan{}, platform.NewError(platform.CodeLocked, false, "父文档已在回收站或已清空")
				}
			}
		}
		if !parentFound {
			return draftOperationPlan{}, platform.NewError(platform.CodeNotFound, false, "父文档不存在")
		}
	}
	documents = append(documents, metadata)
	if _, err = EnsureTreeAcyclic(EnsureTreeAcyclicInput{Documents: documents}); err != nil {
		return draftOperationPlan{}, err
	}
	canonical, err := CanonicalizeBody(CanonicalizeBodyInput{Body: body})
	if err != nil {
		return draftOperationPlan{}, err
	}
	return draftOperationPlan{DocumentID: id, RevisionID: revision, BodyHash: canonical.ContentHash, Metadata: &metadata, ExpectedTreeRevision: treeRevision, ExpectedReferenceVersion: 0, SourceVersion: 1, BlockIDs: []Identifier{}, References: []gen.ResourceReference{}, MediaReferences: []draftMediaReference{}}, nil
}

type creationQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readCreationTree(ctx context.Context, q creationQuery, siteID Identifier, validateMetadata func(json.RawMessage) error) (int64, []gen.DocumentMetadata, error) {
	var version int64
	err := q.QueryRowContext(ctx, `SELECT revision FROM content_tree_state WHERE site_id=?`, siteID).Scan(&version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, nil, contentStorageError()
	}
	limit, activeScan := -1, 0
	if validateMetadata != nil {
		limit, activeScan = 10001, 1
	}
	rows, err := q.QueryContext(ctx, `SELECT d.document_id,d.state,d.revision,CASE WHEN length(CAST(m.metadata_json AS BLOB))<=16777216 THEN m.metadata_json ELSE NULL END,EXISTS(SELECT 1 FROM document_quarantines q WHERE q.site_id=d.site_id AND q.document_id=d.document_id) FROM documents d LEFT JOIN document_metadata m ON m.site_id=d.site_id AND m.document_id=d.document_id WHERE d.site_id=? AND (?=0 OR d.state<>'purged') LIMIT ?`, siteID, activeScan, limit)
	if err != nil {
		return 0, nil, contentStorageError()
	}
	defer rows.Close()
	documents := make([]gen.DocumentMetadata, 0)
	for rows.Next() {
		var id Identifier
		var state string
		var revision int
		var raw []byte
		var quarantined bool
		if rows.Scan(&id, &state, &revision, &raw, &quarantined) != nil {
			return 0, nil, contentStorageError()
		}
		if validateMetadata != nil && validateMetadata(raw) != nil {
			return 0, nil, platform.NewError(platform.CodeIntegrityError, false, "文档树持久元数据不符合唯一schema")
		}
		var metadata gen.DocumentMetadata
		if decodeStorageJSON(raw, &metadata) != nil || metadata.DocumentId != id || metadata.SiteId != siteID || string(metadata.State) != state || metadata.Revision != revision || quarantined {
			return 0, nil, platform.NewError(platform.CodeIntegrityError, false, "文档树元数据或隔离状态不允许创建")
		}
		documents = append(documents, metadata)
		if validateMetadata != nil && len(documents) > 10000 {
			return 0, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "完整文档树超过登记扫描预算")
		}
	}
	if rows.Err() != nil {
		return 0, nil, contentStorageError()
	}
	if len(documents) > 0 && version == 0 {
		return 0, nil, platform.NewError(platform.CodeIntegrityError, false, "文档树缺少站点版本权威")
	}
	return version, documents, nil
}

// fractionalDocumentOrder使用有界字典序中点；饱和时明确冲突，后续排序用例另行重排。
func fractionalDocumentOrder(lower, upper string) (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	originalUpper := upper
	prefix := ""
	for i := 0; i < 128; i++ {
		lo, hi := 0, len(alphabet)-1
		if i < len(lower) {
			lo = strings.IndexByte(alphabet, lower[i])
			if lo < 0 {
				return "", platform.NewError(platform.CodeIntegrityError, false, "文档顺序key格式损坏")
			}
		}
		if i < len(upper) {
			hi = strings.IndexByte(alphabet, upper[i])
			if hi < 0 {
				return "", platform.NewError(platform.CodeIntegrityError, false, "文档顺序key格式损坏")
			}
		}
		if hi-lo > 1 {
			value := prefix + string(alphabet[(lo+hi)/2])
			if (lower == "" || value > lower) && (originalUpper == "" || value < originalUpper) {
				return value, nil
			}
			break
		}
		prefix += string(alphabet[lo])
		if lo < hi {
			upper = ""
		}
	}
	return "", platform.NewError(platform.CodeConflict, false, "文档顺序key已饱和，需要显式重排")
}
func (t *draftCommitTransaction) validateCreateTransaction(ctx context.Context) error {
	plan := t.store.record.Plan
	site := t.request.Operation.Actor.SiteID
	version, documents, err := readCreationTree(ctx, t.tx, site, nil)
	if err != nil {
		return err
	}
	if version != plan.ExpectedTreeRevision || version >= maxEditFence {
		return platform.NewError(platform.CodeVersionMismatch, false, "创建文档时站点树版本已变化")
	}
	for _, document := range documents {
		if document.DocumentId == plan.DocumentID {
			return platform.NewError(platform.CodeConflict, false, "创建文档ID已存在")
		}
		if plan.Metadata.ParentId != nil && document.DocumentId == *plan.Metadata.ParentId && document.State != "draft" && document.State != "published" {
			return platform.NewError(platform.CodeLocked, false, "创建文档父节点已删除")
		}
	}
	documents = append(documents, *plan.Metadata)
	_, err = EnsureTreeAcyclic(EnsureTreeAcyclicInput{Documents: documents})
	return err
}
