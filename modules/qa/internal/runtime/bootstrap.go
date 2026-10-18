// Package runtime 只装配本域私有存储与平台机制，不保存费用权威。
package runtime

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"sort"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/modelclient"
	"nice-wiki/modules/qa/internal/gen"
)

type OperationContext = platform.OperationContext

type Runtime struct {
	DB                     *sql.DB
	WriteWindow            platform.WriteWindowConfig
	SigningKey             platform.SigningKey
	Guard                  *gen.TrafficGuardClient
	Content                *gen.ContentClient
	PublicSite             *gen.PublicSiteClient
	Knowledge              *gen.KnowledgeClient
	Memory                 *gen.MemoryClient
	Localization           *gen.LocalizationClient
	Models                 *gen.ModelGatewayClient
	Jobs                   *gen.JobsClient
	Gateway                platform.GatewayClient
	Validators             map[string]func(json.RawMessage) error
	FastProfileID          string
	EmbeddingProfileID     string
	BaseLanguage           string
	AnswerContextTokens    int
	PreflightContextTokens int
	AnswerMemoryHash       string
	AdmissionPermit        *platform.AdmissionPermit
	OnModelEvent           func(platform.StreamEvent) error
}
type runtimeKey struct{}

// NewRuntime 固定所有使用的生产方响应校验器；未知响应不当作成功。
func NewRuntime(runtime Runtime) (Runtime, error) {
	runtime.Validators = make(map[string]func(json.RawMessage) error)
	for _, name := range []string{"QaEquivalenceDecision", "ContentPublicationChanged", "ContentPublicationWithdrawn", "KnowledgeIndexRetired", "MemoryContextChanged", "JobsRequestCancelResult", "GuardQueueSessionAdmissionResult", "QaDebugTurn", "QaCompaction", "QaQuestionPlan", "QaPlannerDecision", "GuardReportQuestionSignalsResult", "PublicResolvePublishedLinkResult", "QaPolicy", "QaCacheEntry", "QaTurn", "QaCitations", "GuardValidatePermitResult", "GuardConsumeAdmissionResult", "GuardReleasePermitResult", "ContentValidatePublicationsResult", "ContentReadPublishedRevisionResult", "ContentReadPublicTreeResult", "KnowledgeCreateQueryEmbeddingResult", "KnowledgeSearchResult", "KnowledgeReadSectionsResult", "MemoryResolveContextResult", "ModelTaskAvailabilityResult", "CreateInvocationResult", "CancelInvocationResult", "StreamEvent", "GetInvocationResult", "LookupInvocationResult", "LocalizationCachedAnswerTranslationResult"} {
		validator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if err != nil {
			return Runtime{}, err
		}
		runtime.Validators[name] = validator
	}
	if runtime.Models != nil {
		gateway, err := modelclient.NewGatewayClient(runtime.Models, modelclient.GatewayResponseValidators{"CreateInvocationResult": runtime.Validators["CreateInvocationResult"], "CancelInvocationResult": runtime.Validators["CancelInvocationResult"], "StreamEvent": runtime.Validators["StreamEvent"], "GetInvocationResult": runtime.Validators["GetInvocationResult"], "LookupInvocationResult": runtime.Validators["LookupInvocationResult"]})
		if err != nil {
			return Runtime{}, err
		}
		runtime.Gateway = gateway
	}
	return runtime, nil
}
func WithRuntime(ctx context.Context, runtime Runtime) context.Context {
	return context.WithValue(ctx, runtimeKey{}, runtime)
}
func FromContext(ctx context.Context) (Runtime, error) {
	runtime, ok := ctx.Value(runtimeKey{}).(Runtime)
	if !ok || runtime.DB == nil {
		return Runtime{}, platform.NewError(platform.CodeDependencyUnavailable, false, "问答存储尚未装配")
	}
	return runtime, nil
}
func DecodeResponse(runtime Runtime, name string, raw json.RawMessage, target any) error {
	if runtime.Validators[name] == nil || runtime.Validators[name](raw) != nil || json.Unmarshal(raw, target) != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "上游响应不符合登记合同")
	}
	return nil
}
func StorageError(err error) error {
	return platform.NormalizeIOError(err, platform.CodeInternal, false, "问答持久化失败")
}
func RequireAdmin(op OperationContext, permission string) error {
	if op.Actor.Kind != "admin" || op.CallerService != "portal" || !slices.Contains(op.Actor.Grants, permission) {
		return platform.NewError(platform.CodeForbidden, false, "当前账户没有此问答管理权限")
	}
	return nil
}

type Transaction struct {
	Context context.Context
	Tx      *sql.Tx
	Runtime Runtime
	cancel  context.CancelFunc
}

func (t *Transaction) Close()        { _ = t.Tx.Rollback(); t.cancel() }
func (t *Transaction) Commit() error { return StorageError(t.Tx.Commit()) }
func BeginRead(ctx context.Context, op OperationContext) (*Transaction, error) {
	return beginTransaction(ctx, op, false)
}
func BeginWrite(ctx context.Context, op OperationContext) (*Transaction, error) {
	return beginTransaction(ctx, op, true)
}
func beginTransaction(ctx context.Context, op OperationContext, write bool) (*Transaction, error) {
	runtime, err := FromContext(ctx)
	if err != nil {
		return nil, err
	}
	work := ctx
	cancel := func() {}
	if write {
		if runtime.WriteWindow.Runtime == nil || runtime.WriteWindow.WriteWindow == nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "问答写窗口未装配")
		}
		var view platform.RuntimeView
		work, view, cancel, err = lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
		if err != nil {
			return nil, err
		}
		if view.DataEpoch != op.DataEpoch {
			cancel()
			return nil, platform.NewError(platform.CodeStaleResult, false, "问答数据代次已改变")
		}
	}
	tx, err := runtime.DB.BeginTx(work, &sql.TxOptions{ReadOnly: !write})
	if err != nil {
		cancel()
		return nil, StorageError(err)
	}
	var site, epoch string
	if err := tx.QueryRowContext(work, `SELECT site_id,data_epoch FROM qa_identity WHERE singleton=1`).Scan(&site, &epoch); err != nil {
		tx.Rollback()
		cancel()
		return nil, StorageError(err)
	}
	if site != op.Actor.SiteID || epoch != op.DataEpoch {
		tx.Rollback()
		cancel()
		return nil, platform.NewError(platform.CodeStaleResult, false, "问答私有存储身份不一致")
	}
	return &Transaction{Context: work, Tx: tx, Runtime: runtime, cancel: cancel}, nil
}

// ReplayReceipt 固定同主体同输入原结果，7天内同键异输入失败关闭。
func ReplayReceipt(t *Transaction, op OperationContext, function, key string, input, result any) (bool, error) {
	raw, err := platform.CanonicalJSON(input)
	if err != nil {
		return false, platform.NewError(platform.CodeValidation, false, "操作输入无法编码")
	}
	var hash, resultHash string
	var stored []byte
	var created int64
	err = t.Tx.QueryRowContext(t.Context, `SELECT input_hash,result_json,result_hash,created_ns FROM qa_receipts WHERE site_id=? AND data_epoch=? AND actor_id=? AND function_id=? AND idempotency_key=?`, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, function, key).Scan(&hash, &stored, &resultHash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, StorageError(err)
	}
	if time.Since(time.Unix(0, created)) > 7*24*time.Hour {
		_, err := t.Tx.ExecContext(t.Context, `DELETE FROM qa_receipts WHERE site_id=? AND data_epoch=? AND actor_id=? AND function_id=? AND idempotency_key=?`, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, function, key)
		return false, StorageError(err)
	}
	if hash != platform.HashBytes(raw) {
		return false, platform.NewError(platform.CodeConflict, false, "同幂等键已用于其他输入")
	}
	if resultHash != platform.HashBytes(stored) || json.Unmarshal(stored, result) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "操作原回执损坏")
	}
	return true, nil
}
func SaveReceipt(t *Transaction, op OperationContext, function, key string, input, result any) error {
	raw, err := platform.CanonicalJSON(input)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "操作输入无法编码")
	}
	output, err := platform.CanonicalJSON(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "操作回执无法编码")
	}
	_, err = t.Tx.ExecContext(t.Context, `INSERT INTO qa_receipts(site_id,data_epoch,actor_id,function_id,idempotency_key,input_hash,result_json,result_hash,created_ns) VALUES(?,?,?,?,?,?,?,?,?)`, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, function, key, platform.HashBytes(raw), output, platform.HashBytes(output), time.Now().UnixNano())
	return StorageError(err)
}

func ReadPolicy(t *Transaction) (gen.TypesQaPolicy, error) {
	var raw []byte
	var policy gen.TypesQaPolicy
	var version int
	if err := t.Tx.QueryRowContext(t.Context, `SELECT version,policy_json FROM qa_policy WHERE singleton=1`).Scan(&version, &raw); err != nil {
		return policy, StorageError(err)
	}
	if DecodeResponse(t.Runtime, "QaPolicy", raw, &policy) != nil || policy.Version != version {
		return policy, platform.NewError(platform.CodeIntegrityError, false, "问答策略损坏")
	}
	return policy, nil
}
func ReadConversation(t *Transaction, op OperationContext, id string, owner bool) (gen.Conversation, error) {
	var result gen.Conversation
	var state, expiresRaw string
	err := t.Tx.QueryRowContext(t.Context, `SELECT conversation_id,visitor_subject,language,state,version,history_generation,expires_at FROM conversations WHERE conversation_id=? AND site_id=? AND data_epoch=?`, id, op.Actor.SiteID, op.DataEpoch).Scan(&result.ConversationId, &result.VisitorSubject, &result.Language, &state, &result.Version, &result.HistoryGeneration, &expiresRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return result, platform.NewError(platform.CodeNotFound, false, "对话不存在")
	}
	if err != nil {
		return result, StorageError(err)
	}
	if owner && result.VisitorSubject != op.Actor.SubjectID {
		return gen.Conversation{}, platform.NewError(platform.CodeForbidden, false, "只能访问自己的对话")
	}
	result.State = gen.ConversationState(state)
	expires, err := platform.ParseTimestamp(expiresRaw)
	if err != nil {
		return gen.Conversation{}, platform.NewError(platform.CodeIntegrityError, false, "对话过期信息损坏")
	}
	result.ExpiresAt = expires
	if state != "active" || !expires.After(time.Now()) {
		return gen.Conversation{}, platform.NewError(platform.CodeGone, false, "对话已过期或删除")
	}
	return result, nil
}

// TurnRecord 包含仅本域执行需要的原问题、许可与费用引用，管理投影按需筛选。
type TurnRecord struct {
	Turn              gen.Turn
	SubjectID         string
	Language          string
	Question          string
	CanonicalQuestion string
	MemoryHash        string
	Answer            string
	Citations         []gen.Citation
	Permit            platform.AdmissionPermit
	Operation         platform.OperationContext
	State             string
	LeaseID           string
	LeaseUntil        string
	CancelRequested   bool
	Redacted          bool
	CreatedAt         string
	CreatedNS         int64
	TerminalNS        int64
	AnswerKind        string
	CompletionHash    string
	JobID             string
	InvocationIDs     []string
	Plan              *QuestionPlan
}

func ReadTurn(t *Transaction, op OperationContext, id string, owner bool) (TurnRecord, error) {
	var record TurnRecord
	var turnRaw, citationsRaw, permitRaw, operationRaw, invocationsRaw, planRaw []byte
	var cancelled, redacted int
	err := t.Tx.QueryRowContext(t.Context, `SELECT turn_json,visitor_subject,language,question,canonical_question,memory_hash,answer,citations_json,permit_json,operation_json,state,lease_id,lease_until,cancel_requested,redacted,created_at,created_ns,terminal_ns,answer_kind,completion_hash,job_id,invocation_ids_json,plan_json FROM turns WHERE turn_id=? AND site_id=? AND data_epoch=?`, id, op.Actor.SiteID, op.DataEpoch).Scan(&turnRaw, &record.SubjectID, &record.Language, &record.Question, &record.CanonicalQuestion, &record.MemoryHash, &record.Answer, &citationsRaw, &permitRaw, &operationRaw, &record.State, &record.LeaseID, &record.LeaseUntil, &cancelled, &redacted, &record.CreatedAt, &record.CreatedNS, &record.TerminalNS, &record.AnswerKind, &record.CompletionHash, &record.JobID, &invocationsRaw, &planRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return record, platform.NewError(platform.CodeNotFound, false, "问答轮次不存在")
	}
	if err != nil {
		return record, StorageError(err)
	}
	if owner && record.SubjectID != op.Actor.SubjectID {
		return TurnRecord{}, platform.NewError(platform.CodeForbidden, false, "只能访问自己的问答轮次")
	}
	if redacted != 0 {
		return TurnRecord{}, platform.NewError(platform.CodeGone, false, "问答正文已按会话保留规则清除")
	}
	if DecodeResponse(t.Runtime, "QaTurn", turnRaw, &record.Turn) != nil || DecodeResponse(t.Runtime, "QaCitations", citationsRaw, &record.Citations) != nil || json.Unmarshal(permitRaw, &record.Permit) != nil || json.Unmarshal(operationRaw, &record.Operation) != nil || json.Unmarshal(invocationsRaw, &record.InvocationIDs) != nil {
		return TurnRecord{}, platform.NewError(platform.CodeIntegrityError, false, "问答轮次持久数据损坏")
	}
	if len(planRaw) > 0 {
		if DecodeResponse(t.Runtime, "QaQuestionPlan", planRaw, &record.Plan) != nil {
			return TurnRecord{}, platform.NewError(platform.CodeIntegrityError, false, "问答前置计划损坏")
		}
	}
	record.CancelRequested = cancelled != 0
	if record.Turn.TurnId != id || string(record.Turn.State) != record.State || record.Operation.DataEpoch != op.DataEpoch {
		return TurnRecord{}, platform.NewError(platform.CodeIntegrityError, false, "问答轮次身份或状态不一致")
	}
	return record, nil
}
func IsTerminal(state string) bool {
	return state == "completed" || state == "cancelled" || state == "failed" || state == "invalidated"
}
func CheckTurnLease(record TurnRecord, lease string) error {
	until, err := platform.ParseTimestamp(record.LeaseUntil)
	if err != nil || record.LeaseID != lease || !until.After(time.Now()) || record.CancelRequested || IsTerminal(record.State) {
		return platform.NewError(platform.CodeStaleResult, false, "问答执行租约或状态已失效")
	}
	return nil
}
func SaveTurn(t *Transaction, record TurnRecord) error {
	raw, err := platform.CanonicalJSON(record.Turn)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "问答状态无法编码")
	}
	citations, _ := json.Marshal(record.Citations)
	invocations, _ := json.Marshal(record.InvocationIDs)
	_, err = t.Tx.ExecContext(t.Context, `UPDATE turns SET turn_json=?,state=?,answer=?,canonical_question=?,memory_hash=?,citations_json=?,invocation_ids_json=?,cancel_requested=?,terminal_ns=?,answer_kind=?,completion_hash=? WHERE turn_id=?`, raw, record.Turn.State, record.Answer, record.CanonicalQuestion, record.MemoryHash, citations, invocations, record.CancelRequested, record.TerminalNS, record.AnswerKind, record.CompletionHash, record.Turn.TurnId)
	return StorageError(err)
}

// ServiceReadContext 只为本域公开权威读取装配纯服务身份；付费调用保留原可信主体。
func ServiceReadContext(ctx context.Context, op OperationContext) context.Context {
	op.CallerService = "qa"
	op.Actor = platform.AuthContext{Kind: "service", SubjectID: "qa", SiteID: op.Actor.SiteID, Grants: []string{}, ExpiresAt: op.Deadline}
	return platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
}

type PublishedDocument struct {
	Reference gen.PublishedRevisionRef `json:"reference"`
	Metadata  struct {
		Title          string `json:"title"`
		SourceLanguage string `json:"sourceLanguage"`
	} `json:"metadata"`
	Body               json.RawMessage   `json:"body"`
	VisibilityRevision int               `json:"visibilityRevision"`
	Blocks             map[string]string `json:"-"`
}

func ReadPublishedDocument(ctx context.Context, op OperationContext, ref gen.PublishedRevisionRef) (PublishedDocument, error) {
	runtime, err := FromContext(ctx)
	if err != nil {
		return PublishedDocument{}, err
	}
	if runtime.Content == nil {
		return PublishedDocument{}, platform.NewError(platform.CodeDependencyUnavailable, false, "正文权威未装配")
	}
	raw, err := runtime.Content.ReadPublishedRevision(ServiceReadContext(ctx, op), ref.DocumentId, url.Values{"publishedRevisionId": {ref.PublishedRevisionId}})
	if err != nil {
		return PublishedDocument{}, err
	}
	var document PublishedDocument
	if err := DecodeResponse(runtime, "ContentReadPublishedRevisionResult", raw, &document); err != nil {
		return PublishedDocument{}, err
	}
	if document.Reference != ref || ref.DataEpoch != op.DataEpoch {
		return PublishedDocument{}, platform.NewError(platform.CodeStaleResult, false, "正文来源已变化")
	}
	var body any
	decoder := json.NewDecoder(bytes.NewReader(document.Body))
	decoder.UseNumber()
	if decoder.Decode(&body) != nil {
		return PublishedDocument{}, platform.NewError(platform.CodeIntegrityError, false, "公开正文不可读")
	}
	canonical, err := platform.CanonicalJSON(body)
	if err != nil || platform.HashBytes(canonical) != ref.ContentHash {
		return PublishedDocument{}, platform.NewError(platform.CodeIntegrityError, false, "公开正文摘要不一致")
	}
	document.Blocks = make(map[string]string)
	var visit func(any) error
	visit = func(value any) error {
		node, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["blockId"].(string); ok {
				if _, exists := document.Blocks[id]; exists {
					return platform.NewError(platform.CodeIntegrityError, false, "公开正文块标识重复")
				}
				raw, err := platform.CanonicalJSON(node)
				if err != nil {
					return err
				}
				document.Blocks[id] = platform.HashBytes(raw)
			}
		}
		if content, ok := node["content"].([]any); ok {
			for _, child := range content {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(body); err != nil {
		return PublishedDocument{}, err
	}
	return document, nil
}

// ValidateCitations 检查精确发布引用和规范块hash，不依赖事件是否已经抵达。
func ValidateCitations(ctx context.Context, op OperationContext, citations []gen.Citation, language string) ([]gen.Citation, error) {
	documents := make(map[string]PublishedDocument)
	result := make([]gen.Citation, 0, len(citations))
	seen := make(map[string]bool)
	for _, citation := range citations {
		if seen[citation.CitationId] || citation.Block.Document != citation.Document {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "引用标识或来源不一致")
		}
		seen[citation.CitationId] = true
		encoded, _ := platform.CanonicalJSON(citation.Document)
		key := string(encoded)
		document, exists := documents[key]
		if !exists {
			var err error
			document, err = ReadPublishedDocument(ctx, op, citation.Document)
			if err != nil {
				return nil, err
			}
			documents[key] = document
		}
		if hash, exists := document.Blocks[citation.Block.BlockId]; !exists || hash != citation.Block.BlockHash {
			return nil, platform.NewError(platform.CodeStaleResult, false, "引用块已变化")
		}
		runtime, err := FromContext(ctx)
		if err != nil {
			return nil, err
		}
		if runtime.PublicSite == nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "公开链接权威未装配")
		}
		raw, err := runtime.PublicSite.ResolvePublishedLink(ServiceReadContext(ctx, op), struct {
			DocumentID string  `json:"documentId"`
			Language   string  `json:"language"`
			BlockID    *string `json:"blockId"`
		}{citation.Document.DocumentId, language, &citation.Block.BlockId})
		if err != nil {
			return nil, err
		}
		var resolved struct {
			Visible          bool                      `json:"visible"`
			Href             *string                   `json:"href"`
			Publication      *gen.PublishedRevisionRef `json:"publication"`
			ResolvedLanguage *string                   `json:"resolvedLanguage"`
		}
		if err := DecodeResponse(runtime, "PublicResolvePublishedLinkResult", raw, &resolved); err != nil {
			return nil, err
		}
		if !resolved.Visible || resolved.Href == nil || resolved.Publication == nil || resolved.ResolvedLanguage == nil || *resolved.Publication != citation.Document || *resolved.ResolvedLanguage != language {
			return nil, platform.NewError(platform.CodeStaleResult, false, "引用的公开语言路径或发布修订已变化")
		}
		citation.Href = *resolved.Href
		citation.VisibilityRevision = document.VisibilityRevision
		result = append(result, citation)
	}
	return result, nil
}

func PublicationFingerprint(ctx context.Context, op OperationContext) (string, error) {
	runtime, err := FromContext(ctx)
	if err != nil {
		return "", err
	}
	if runtime.Content == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, false, "发布集合权威未装配")
	}
	raw, err := runtime.Content.ReadPublicTree(ServiceReadContext(ctx, op))
	if err != nil {
		return "", err
	}
	var result struct {
		Nodes []struct {
			DocumentID string                   `json:"documentId"`
			Reference  gen.PublishedRevisionRef `json:"reference"`
		} `json:"nodes"`
	}
	if err := DecodeResponse(runtime, "ContentReadPublicTreeResult", raw, &result); err != nil {
		return "", err
	}
	refs := make([]gen.PublishedRevisionRef, 0, len(result.Nodes))
	seen := make(map[string]bool)
	for _, node := range result.Nodes {
		if seen[node.DocumentID] || node.Reference.DocumentId != node.DocumentID || node.Reference.DataEpoch != op.DataEpoch {
			return "", platform.NewError(platform.CodeIntegrityError, false, "发布集合身份不一致")
		}
		seen[node.DocumentID] = true
		refs = append(refs, node.Reference)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].DocumentId < refs[j].DocumentId })
	encoded, _ := platform.CanonicalJSON(refs)
	return platform.HashBytes(encoded), nil
}

// MemoryContext 是单一task/语言已渲染注入文本；字节数不包含JSON元数据。
type MemoryContext struct {
	TaskID          string `json:"taskId"`
	Language        string `json:"language"`
	Text            string `json:"text"`
	VersionHash     string `json:"versionHash"`
	MappingRevision int    `json:"mappingRevision"`
	ByteSize        int    `json:"byteSize"`
}

func ResolveTaskMemory(ctx context.Context, op OperationContext, task, language string) (MemoryContext, error) {
	runtime, err := FromContext(ctx)
	if err != nil {
		return MemoryContext{}, err
	}
	if runtime.Memory == nil {
		return MemoryContext{}, platform.NewError(platform.CodeDependencyUnavailable, false, "任务记忆权威尚未装配")
	}
	raw, err := runtime.Memory.ResolveContext(ServiceReadContext(ctx, op), struct {
		TaskID   string `json:"taskId"`
		Language string `json:"language"`
	}{task, language})
	if err != nil {
		return MemoryContext{}, err
	}
	var result MemoryContext
	if err := DecodeResponse(runtime, "MemoryResolveContextResult", raw, &result); err != nil {
		return MemoryContext{}, err
	}
	if result.TaskID != task || result.Language != language || result.ByteSize != len(result.Text) || result.ByteSize > 5120 {
		return MemoryContext{}, platform.NewError(platform.CodeIntegrityError, false, "任务记忆身份或UTF8预算不一致")
	}
	return result, nil
}

// QueryEmbedding 只保存knowledge返回的完整空间和原MG调用引用。
type QueryEmbedding struct {
	QueryHash    string             `json:"queryHash"`
	Space        gen.EmbeddingSpace `json:"space"`
	Vector       []float64          `json:"vector"`
	VectorHash   string             `json:"vectorHash"`
	InvocationID string             `json:"invocationId"`
	DataEpoch    string             `json:"dataEpoch"`
}

type QuestionPlan struct {
	OriginalQuestion   string        `json:"originalQuestion"`
	CanonicalQuestion  string        `json:"canonicalQuestion"`
	VisitorLanguage    string        `json:"visitorLanguage"`
	Complete           bool          `json:"complete"`
	CacheEligible      bool          `json:"cacheEligible"`
	Subqueries         []string      `json:"subqueries"`
	Classification     string        `json:"classification"`
	Risk               string        `json:"risk"`
	ContextFingerprint platform.Hash `json:"contextFingerprint"`
	ResolvedReferences bool          `json:"resolvedReferences"`
}

// CursorActor 固定可撤销的授权身份，排除每次委托变化的expiresAt。
func CursorActor(op OperationContext) any {
	return struct {
		Subject, Site, Kind, Session        string
		SessionRevision, PermissionRevision int64
	}{op.Actor.SubjectID, op.Actor.SiteID, op.Actor.Kind, op.Actor.SessionID, op.Actor.SessionRevision, op.Actor.PermissionRevision}
}
