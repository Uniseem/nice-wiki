// Package platform 提供各服务共用的契约类型与基础设施接口。
//
// 本文件承载契约类型、接口及公共纯工具，不含业务规则；行为机制分布在
// serviceauth、opctx、problem、outbox、inbox、worker、modelclient、
// lifecycle 各包，由各服务装配后调用。
package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Identifier、Hash、Timestamp、Revision 与公共schema的标量定义一致。
type (
	// Identifier 系统生成的不透明ID，^[A-Za-z0-9._:-]+$，1..128字节。
	Identifier = string
	// Hash SHA-256小写十六进制。
	Hash = string
	// Timestamp RFC3339 UTC时间。
	Timestamp = string
	// Revision 单调非负修订号。
	Revision = int64
)

// MemoryContext 与公共schema一致，字节数只计算完整注入文本。
type MemoryContext struct {
	TaskID          string   `json:"taskId"`
	Language        string   `json:"language"`
	Text            string   `json:"text"`
	VersionHash     Hash     `json:"versionHash"`
	MappingRevision Revision `json:"mappingRevision"`
	ByteSize        int64    `json:"byteSize"`
}

// CostClass 可信费用归属类别；公开请求不能指定。
const (
	CostClassQA         = "qa"
	CostClassBackground = "background"
	CostClassAdminProbe = "admin-probe"
)

// ErrorCode 与 contracts/errors.json 登记的错误码一致。
const (
	CodeValidation            = "VALIDATION"
	CodeUnauthenticated       = "UNAUTHENTICATED"
	CodeForbidden             = "FORBIDDEN"
	CodeNotFound              = "NOT_FOUND"
	CodeGone                  = "GONE"
	CodeChallengeRequired     = "CHALLENGE_REQUIRED"
	CodeConflict              = "CONFLICT"
	CodeVersionMismatch       = "VERSION_MISMATCH"
	CodeLocked                = "LOCKED"
	CodeUnsupportedSchema     = "UNSUPPORTED_SCHEMA"
	CodeUnsupportedCapability = "UNSUPPORTED_CAPABILITY"
	CodeRateLimited           = "RATE_LIMITED"
	CodeBudgetExceeded        = "BUDGET_EXCEEDED"
	CodeCancelled             = "CANCELLED"
	CodeDeadlineExceeded      = "DEADLINE_EXCEEDED"
	CodeDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
	CodeStaleResult           = "STALE_RESULT"
	CodeIntegrityError        = "INTEGRITY_ERROR"
	CodeInternal              = "INTERNAL"
)

// ProblemHTTPStatus 按errors.json登记的错误码到HTTP状态映射；
// 与 contracts/errors.json 的 httpStatus 表保持一致。
var ProblemHTTPStatus = map[string]int{
	CodeValidation:            400,
	CodeUnauthenticated:       401,
	CodeForbidden:             403,
	CodeNotFound:              404,
	CodeGone:                  410,
	CodeChallengeRequired:     429,
	CodeConflict:              409,
	CodeVersionMismatch:       409,
	CodeLocked:                409,
	CodeUnsupportedSchema:     422,
	CodeUnsupportedCapability: 422,
	CodeRateLimited:           429,
	CodeBudgetExceeded:        429,
	CodeCancelled:             409,
	CodeDeadlineExceeded:      504,
	CodeDependencyUnavailable: 503,
	CodeStaleResult:           409,
	CodeIntegrityError:        422,
	CodeInternal:              500,
}

// Error 携带登记错误码的库错误；调用方映射为公共Problem。
// 消息不含密钥、请求正文或IP。
type Error struct {
	Code      string
	Retryable bool
	Message   string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// NewError 构造带登记错误码的库错误；code必须已登记。
func NewError(code string, retryable bool, format string, args ...any) *Error {
	return &Error{Code: code, Retryable: retryable, Message: fmt.Sprintf(format, args...)}
}

// NormalizeIOError统一保留平台错误和取消/截止语义；未知错误只使用可信安全说明。
// 不把原始存储错误、路径或请求内容带入公开错误。
func NormalizeIOError(err error, fallbackCode string, retryable bool, safeMessage string) error {
	if err == nil {
		return nil
	}
	var public *Error
	if errors.As(err, &public) {
		return public
	}
	if errors.Is(err, context.Canceled) {
		return NewError(CodeCancelled, false, "操作已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return NewError(CodeDeadlineExceeded, false, "操作已到截止时间")
	}
	return NewError(fallbackCode, retryable, "%s", safeMessage)
}

// FieldIssue 逐字段输入错误，不回显字段值。
type FieldIssue struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Problem RFC9457公共失败响应；不含秘密、问题正文、原始IP或内部堆栈。
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      string       `json:"code"`
	Detail    string       `json:"detail"`
	RequestID Identifier   `json:"requestId"`
	Retryable bool         `json:"retryable"`
	Issues    []FieldIssue `json:"issues,omitempty"`
}

// AuthContext 经identity验证的单站授权上下文。
type AuthContext struct {
	SubjectID          Identifier `json:"subjectId"`
	Kind               string     `json:"kind"`
	SiteID             Identifier `json:"siteId"`
	SessionID          Identifier `json:"sessionId,omitempty"`
	SessionRevision    Revision   `json:"sessionRevision,omitempty"`
	PermissionRevision Revision   `json:"permissionRevision"`
	Grants             []string   `json:"grants"`
	ExpiresAt          Timestamp  `json:"expiresAt"`
}

// OperationContext 服务间可信操作元数据。
type OperationContext struct {
	RequestID           Identifier  `json:"requestId"`
	OperationID         Identifier  `json:"operationId"`
	RootOperationID     Identifier  `json:"rootOperationId"`
	CallerService       string      `json:"callerService"`
	Actor               AuthContext `json:"actor"`
	Deadline            Timestamp   `json:"deadline"`
	DataEpoch           Identifier  `json:"dataEpoch"`
	CostClass           string      `json:"costClass,omitempty"`
	CancellationID      Identifier  `json:"cancellationId,omitempty"`
	LoginNetworkScopes  []Hash      `json:"-"`
	NetworkBudgetScopes []Hash      `json:"-"`
}

// EventEnvelope 持久至少一次事件信封；载荷须符合生产方events.json。
type EventEnvelope struct {
	EventID      Identifier      `json:"eventId"`
	EventType    string          `json:"eventType"`
	EventVersion int             `json:"eventVersion"`
	Producer     string          `json:"producer"`
	SubjectID    Identifier      `json:"subjectId"`
	Revision     Revision        `json:"revision"`
	OccurredAt   Timestamp       `json:"occurredAt"`
	DataEpoch    Identifier      `json:"dataEpoch"`
	Payload      json.RawMessage `json:"payload"`
}

// SigningKey 私钥引用；私钥字节只在内存，不进日志。
type SigningKey struct {
	KeyID         string `json:"keyId"`
	PrivateKeyRef string `json:"privateKeyRef"`
}

// SessionAccessClaims 是identity发行、portal使用的访问令牌声明。
// 验签不替代identity对会话、账户与撤销版本的权威检查。
type SessionAccessClaims struct {
	Issuer             string     `json:"iss"`
	Audience           string     `json:"aud"`
	SessionID          Identifier `json:"sid"`
	AccountID          Identifier `json:"sub"`
	SiteID             Identifier `json:"siteId"`
	CredentialRevision Revision   `json:"credentialRevision"`
	PermissionRevision Revision   `json:"permissionRevision"`
	SessionRevision    Revision   `json:"sessionRevision"`
	AuthEpoch          Identifier `json:"authEpoch"`
	DataEpoch          Identifier `json:"dataEpoch"`
	IssuedAt           int64      `json:"iat"`
	ExpiresAt          int64      `json:"exp"`
	TokenID            Identifier `json:"jti"`
}

// SessionSecrets 仅用于内存中的cookie适配，持久化只保留摘要。
type SessionSecrets struct {
	AccessNonce  string `json:"accessNonce"`
	RefreshToken string `json:"refreshToken"`
	CSRFToken    string `json:"csrfToken"`
}

// AssertionClaims 服务断言声明；expiresAt与issuedAt相差≤60秒。
type AssertionClaims struct {
	Issuer              string     `json:"iss"`
	Audience            string     `json:"aud"`
	Method              string     `json:"method"`
	Path                string     `json:"path"`
	BodyHash            Hash       `json:"bodyHash"`
	IssuedAt            Timestamp  `json:"issuedAt"`
	ExpiresAt           Timestamp  `json:"expiresAt"`
	JTI                 Identifier `json:"jti"`
	LoginNetworkScopes  []Hash     `json:"loginNetworkScopes,omitempty"`
	NetworkBudgetScopes []Hash     `json:"networkBudgetScopes,omitempty"`
	SiteID              Identifier `json:"siteId,omitempty"`
}

// AssertionExpectation 验证断言时的期望值。
type AssertionExpectation struct {
	Audience string `json:"audience"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	BodyHash Hash   `json:"bodyHash"`
}

// ServicePublicKey 一个服务的Ed25519公钥登记项。
type ServicePublicKey struct {
	Slug      string `json:"slug"`
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
}

// DelegationPublicKey portal用户委托公钥登记项。
type DelegationPublicKey struct {
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
}

// TrustAnchors 服务公钥表；由nw-keys生成、trustAnchorRef加载。
type TrustAnchors struct {
	Services             []ServicePublicKey    `json:"services"`
	PortalDelegationKeys []DelegationPublicKey `json:"portalDelegationKeys"`
}

// ServiceKeyID 返回slug+keyId对应的公钥base64url；未登记返回空。
func (a TrustAnchors) ServiceKeyID(slug, keyID string) string {
	for _, s := range a.Services {
		if s.Slug == slug && s.KeyID == keyID {
			return s.PublicKey
		}
	}
	return ""
}

// DelegationKeyID 返回keyId对应的委托公钥base64url；未登记返回空。
func (a TrustAnchors) DelegationKeyID(keyID string) string {
	for _, k := range a.PortalDelegationKeys {
		if k.KeyID == keyID {
			return k.PublicKey
		}
	}
	return ""
}

// DelegationClaims portal签发的用户委托声明；有效期≤10秒。
type DelegationClaims struct {
	Actor          AuthContext `json:"actor"`
	SessionVersion int         `json:"sessionVersion"`
	Route          string      `json:"route"`
	BodyHash       Hash        `json:"bodyHash"`
	ExpiresAt      Timestamp   `json:"expiresAt"`
}

// DelegationExpectation 验证委托时的期望值；下游原样转发时route留空。
type DelegationExpectation struct {
	CallerService string `json:"callerService"`
	Route         string `json:"route"`
	BodyHash      Hash   `json:"bodyHash"`
}

// RuntimeView site-config权威运行状态摘要。
type RuntimeView struct {
	Revision Revision `json:"revision"`
	BarrierID Identifier `json:"barrierId,omitempty"`
	DataEpoch       Identifier `json:"dataEpoch"`
	Phase           string     `json:"phase"`
	RestoreID       Identifier `json:"restoreId,omitempty"`
	TargetDataEpoch Identifier `json:"targetDataEpoch,omitempty"`
	Maintenance     bool       `json:"maintenance"`
}

// 运行阶段，与site-config RuntimePhase枚举一致。
const (
	PhaseRunning            = "running"
	PhaseBackupBarrier      = "backup_barrier"
	PhaseRestoreStaging     = "restore_staging"
	PhaseRestoreSwitching   = "restore_switching"
	PhaseRestoreRollingBack = "restore_rolling_back"
	PhaseRestoreBlocked     = "restore_blocked"
)

// RelayConfig outbox转发循环参数；batchLimit≤100。
type RelayConfig struct {
	BatchLimit int `json:"batchLimit"`
	PollMillis int `json:"pollMillis"`
	// Runtime 与 WriteWindow均必需；未知权威或关闭安全窗口时停止active写入。
	Runtime     func(ctx context.Context) (RuntimeView, error) `json:"-"`
	WriteWindow func(context.Context) (time.Time, error)       `json:"-"`
}

// WriteWindowConfig 由受信装配提供当前运行权威及本域control安全截止。
type WriteWindowConfig struct {
	Runtime     func(context.Context) (RuntimeView, error)
	WriteWindow func(context.Context) (time.Time, error)
}

// RecoveryLoopConfig 的Recover只执行本域一个有界恢复批次，不执行付费重算。
type RecoveryLoopConfig struct {
	PollMillis  int
	Runtime     func(context.Context) (RuntimeView, error)
	WriteWindow func(context.Context) (time.Time, error)
	Recover     func(context.Context) error
}

// ConsumerConfig 事件订阅循环参数；事件为“名称@版本”。
type ConsumerConfig struct {
	// SiteID由本域受信装配提供；空值仅用于system控制命名空间。
	SiteID      string   `json:"-"`
	ConsumerID  string   `json:"consumerId"`
	Events      []string `json:"events"`
	MaxInFlight int      `json:"maxInFlight"`
	// Runtime必需；读取失败或屏障期间暂停，不按running放行。
	Runtime func(ctx context.Context) (RuntimeView, error) `json:"-"`
	// ValidatePayload必需；生产方schema不符时不确认，等待修复或死信。
	ValidatePayload func(eventType string, version int, payload json.RawMessage) error `json:"-"`
}

// ConsumerRunResult为本次循环的实际处理/跳过确认与失败尝试计数。
type ConsumerRunResult struct {
	Acknowledged int64 `json:"acknowledged"`
	Skipped      int64 `json:"skipped"`
	Failed       int64 `json:"failed"`
}

// WorkerConfig 任务领取循环参数；owner为本域slug。
type WorkerConfig struct {
	OnLeaseRenewed func(context.Context, OperationContext, ClaimedJob, HeartbeatInfo) error `json:"-"`
	Owner          string   `json:"owner"`
	JobTypes       []string `json:"jobTypes"`
	MaxConcurrency int      `json:"maxConcurrency"`
	// Runtime 读取当前运行状态；维护期间暂停领取。可为nil。
	Runtime func(ctx context.Context) (RuntimeView, error) `json:"-"`
	// OperationContext从本域持久任务读取获准主体与预算，不凭任务ID猜测授权。
	OperationContext func(context.Context, ClaimedJob) (OperationContext, error) `json:"-"`
}

// ServiceBoundaryConfig 只由服务启动时从合同装配，不接受请求自报策略。
type ServiceBoundaryConfig struct {
	Self                  string
	Anchors               TrustAnchors
	Runtime               func(context.Context) (RuntimeView, error)
	OpenedDataEpoch       Identifier
	AllowedCallers        map[string][]string
	ServiceActorCallers   map[string][]string
	MaintenanceOperations map[string]bool
	MaxBodyBytes          int64
	OperationBodyLimits   map[string]int64
	Authorize             func(context.Context, OperationContext, string, string) error
}

type AuthorizationClient interface {
	CheckAuthorization(context.Context, any) (json.RawMessage, error)
}

// AssetRef media资源引用；经media受控读取。
type AssetRef struct {
	AssetID   Identifier `json:"assetId"`
	SHA256    Hash       `json:"sha256"`
	MediaType string     `json:"mediaType"`
	ByteSize  int64      `json:"byteSize"`
}

// ObjectRef 所属服务提供的不可变受控对象。
type ObjectRef struct {
	Owner     string     `json:"owner"`
	ObjectID  Identifier `json:"objectId"`
	Version   Revision   `json:"version"`
	SHA256    Hash       `json:"sha256"`
	MediaType string     `json:"mediaType"`
	ByteSize  int64      `json:"byteSize"`
}

// AdmissionPermit guard短期签名准入；model-gateway本地验签。
type AdmissionPermit struct {
	AdmissionID        Identifier `json:"admissionId"`
	RequestID          Identifier `json:"requestId"`
	SubjectID          Identifier `json:"subjectId"`
	Action             string     `json:"action"`
	Token              string     `json:"token"`
	ExpiresAt          Timestamp  `json:"expiresAt"`
	SessionBudgetScope Identifier `json:"sessionBudgetScope"`
	NetworkBudgetScope Identifier `json:"networkBudgetScope"`
	DataEpoch          Identifier `json:"dataEpoch"`
	Audience           string     `json:"audience"`
}

// GuardSessionClaims是风控挑战通过证明，不授予模型调用或账户权限。
type GuardSessionClaims struct {
	SubjectID Identifier `json:"subjectId"`
	SiteID    Identifier `json:"siteId"`
	SessionID Identifier `json:"sessionId"`
	DataEpoch Identifier `json:"dataEpoch"`
	ExpiresAt Timestamp  `json:"expiresAt"`
	Audience  string     `json:"audience"`
}

// 网关能力枚举。
const (
	CapabilityChat        = "chat"
	CapabilityVision      = "vision"
	CapabilityEmbedding   = "embedding"
	CapabilityRerank      = "rerank"
	CapabilityTranslation = "translation"
)

// GatewayPart 视觉输入片段；kind=text须带text，kind=image须带image。
type GatewayPart struct {
	Kind  string    `json:"kind"`
	Text  string    `json:"text,omitempty"`
	Image *AssetRef `json:"image,omitempty"`
}

// GatewayMessage chat/vision消息。
type GatewayMessage struct {
	Role    string        `json:"role"`
	Content string        `json:"content"`
	Parts   []GatewayPart `json:"parts,omitempty"`
}

// CreateInvocationInput model-gateway调用请求；字段与生产方types.json一致。
type CreateInvocationInput struct {
	Query               string           `json:"query,omitempty"`
	TaskID              string           `json:"taskId"`
	ProfileID           Identifier       `json:"profileId"`
	Capability          string           `json:"capability"`
	Messages            []GatewayMessage `json:"messages"`
	Texts               []string         `json:"texts"`
	CandidateIDs        []Identifier     `json:"candidateIds"`
	MaximumOutputTokens int64            `json:"maximumOutputTokens"`
	IDempotencyKey      string           `json:"idempotencyKey"`
	AdmissionPermit     *AdmissionPermit `json:"admissionPermit,omitempty"`
}

// ModelAttempt 一次独立计费的供应方尝试。
type ModelAttempt struct {
	AttemptID         Identifier `json:"attemptId"`
	InvocationID      Identifier `json:"invocationId"`
	Ordinal           int64      `json:"ordinal"`
	ModelID           Identifier `json:"modelId"`
	State             string     `json:"state"`
	ReservationID     Identifier `json:"reservationId"`
	ProviderRequestID string     `json:"providerRequestId,omitempty"`
	RetryReason       string     `json:"retryReason"`
}

// Invocation 网关调用状态快照。
type Invocation struct {
	InvocationID      Identifier     `json:"invocationId"`
	RootOperationID   Identifier     `json:"rootOperationId"`
	CostClass         string         `json:"costClass"`
	State             string         `json:"state"`
	ReservationID     Identifier     `json:"reservationId,omitempty"`
	ProviderRequestID string         `json:"providerRequestId,omitempty"`
	Deadline          Timestamp      `json:"deadline"`
	PriceSnapshotID   Identifier     `json:"priceSnapshotId"`
	LastSequence      int64          `json:"lastSequence"`
	Attempts          []ModelAttempt `json:"attempts"`
}

// 调用终态与StreamEvent事件名。
const (
	InvocationSucceeded     = "succeeded"
	InvocationFailed        = "failed"
	InvocationCancelled     = "cancelled"
	InvocationChargeUnknown = "charge_unknown"

	StreamAccepted  = "accepted"
	StreamDelta     = "delta"
	StreamUsage     = "usage"
	StreamCompleted = "completed"
	StreamError     = "error"
	StreamCancelled = "cancelled"
)

// StreamEvent 网关SSE事件；sequence严格递增。
// EmbeddingSpace完整标识向量空间；不得只以模型展示名比较。
type EmbeddingSpace struct {
	SpaceID              Hash   `json:"spaceId"`
	Provider             string `json:"provider"`
	ModelVersion         string `json:"modelVersion"`
	Dimensions           int64  `json:"dimensions"`
	PreprocessingVersion string `json:"preprocessingVersion"`
}

type ModelOutput struct {
	Capability     string          `json:"capability"`
	EmbeddingSpace *EmbeddingSpace `json:"embeddingSpace,omitempty"`
	Vectors        [][]float64     `json:"vectors,omitempty"`
	CandidateIDs   []Identifier    `json:"candidateIds,omitempty"`
}

type StreamEvent struct {
	Output     *ModelOutput `json:"output,omitempty"`
	Sequence   int64        `json:"sequence"`
	Event      string       `json:"event"`
	Delta      string       `json:"delta,omitempty"`
	Invocation *Invocation  `json:"invocation,omitempty"`
	Code       string       `json:"code,omitempty"`
}

// InvokeModelRequest 模型调用请求；idempotencyKey=操作ID+步骤名。
type InvokeModelRequest struct {
	Request        CreateInvocationInput `json:"request"`
	IDempotencyKey Identifier            `json:"idempotencyKey"`
	Stream         bool                  `json:"stream"`
	// OnEvent 非nil时每收到一条事件回调一次；返回错误即中止。不序列化。
	OnEvent func(StreamEvent) error `json:"-"`
}

// InvokeModelOutcome 模型调用终态；无error时有效。
type InvokeModelOutcome struct {
	InvocationID Identifier    `json:"invocationId"`
	State        string        `json:"state"`
	Events       []StreamEvent `json:"events"`
}

// ServiceConfig 通用服务配置；secret只保存引用路径。
type ServiceConfig struct {
	AllowedGitHosts []string `json:"allowedGitHosts,omitempty"`
	Module            string            `json:"module"`
	Listen            string            `json:"listen"`
	DataRoot          string            `json:"dataRoot"`
	TrustAnchorRef    string            `json:"trustAnchorRef"`
	SigningKeyRef     string            `json:"signingKeyRef"`
	DelegationKeyRef  string            `json:"delegationKeyRef,omitempty"`
	MasterKeyRef      string            `json:"masterKeyRef,omitempty"`
	Dependencies      map[string]string `json:"dependencies"`
	PublicBaseURL     string            `json:"publicBaseURL,omitempty"`
	AllowLocalHTTP    bool              `json:"allowLocalHTTP,omitempty"`
	TrustedProxyCIDRs []string          `json:"trustedProxyCIDRs,omitempty"`
}

// SwapResult 启动时换目录结果。
type SwapResult struct {
	Action    string     `json:"action"`
	RestoreID Identifier `json:"restoreId,omitempty"`
}

// 换目录动作。
const (
	SwapNone       = "none"
	SwapSwitched   = "switched"
	SwapRolledBack = "rolled-back"
)

// ClaimedJob jobs领取结果中的单个任务。
type ClaimedJob struct {
	// WorkerID为本轮平台领取者，不接受HTTP载荷自报。
	WorkerID Identifier `json:"-"`
	JobID           Identifier `json:"jobId"`
	OperationID     Identifier `json:"operationId"`
	RootOperationID Identifier `json:"rootOperationId"`
	JobType         string     `json:"type"`
	InputRef        ObjectRef  `json:"inputRef"`
	FencingToken    int64      `json:"fencingToken"`
	Attempt         int        `json:"attempt"`
	DataEpoch       Identifier `json:"dataEpoch"`
	ExpiresAt       Timestamp  `json:"expiresAt"`
}

// HeartbeatInfo 续租响应；cancelRequested表示jobs要求停止。
type HeartbeatInfo struct {
	ExpiresAt          Timestamp `json:"expiresAt"`
	CancelRequested    bool      `json:"cancelRequested"`
	SafePointRequested bool      `json:"safePointRequested"`
}

// JobExecutionInput 任务执行函数的统一输入。
type JobExecutionInput struct {
	JobID        Identifier `json:"jobId"`
	WorkerID     Identifier `json:"workerId"`
	JobType      string     `json:"jobType"`
	InputRef     ObjectRef  `json:"inputRef"`
	FencingToken int64      `json:"fencingToken"`
	Attempt      int        `json:"attempt"`
	DataEpoch    Identifier `json:"dataEpoch"`
	// CheckLease在不可逆决定前由执行器锁外调用；租约未知或取消均拒绝。
	CheckLease func(context.Context) error `json:"-"`
}

// RelayBatchResult只描述本批已领取行；不表示全域已送达。
type RelayBatchResult struct {
	Relayed     int64 `json:"relayed"`
	Pending     int64 `json:"pending"`
	Quarantined int64 `json:"quarantined"`
}

// JobExecutionResult 任务执行结果；retry表示可重试失败。
type JobExecutionResult struct {
	State          string       `json:"state"`
	ResultRef      *ObjectRef   `json:"resultRef,omitempty"`
	ErrorCode      string       `json:"errorCode,omitempty"`
	PaidResultRefs []Identifier `json:"paidResultRefs,omitempty"`
}

// 任务执行结果状态。
const (
	JobSucceeded = "succeeded"
	JobRetry     = "retry"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

// EventDelivery 事件处理函数的统一输入。
type EventDelivery struct {
	Envelope EventEnvelope `json:"envelope"`
}

// EventHandled 事件处理结果；applied=false表示重复或低修订无操作。
type EventHandled struct {
	Applied bool `json:"applied"`
}

// EventHandler 事件处理函数；在本地事务内完成inbox去重与业务写入，
// 返回nil表示已提交、可以确认。
type EventHandler func(ctx context.Context, delivery EventDelivery) error

// EventHandlerRegistry 事件名→处理函数，由本域events.json装配。
type EventHandlerRegistry map[string]EventHandler

// Handler 返回事件名的处理函数；未登记返回nil。
func (r EventHandlerRegistry) Handler(eventType string) EventHandler {
	return r[eventType]
}

// JobExecutor 任务执行函数；由领取循环传入fencing上下文。
type JobExecutor func(ctx context.Context, input JobExecutionInput) (JobExecutionResult, error)

// JobExecutorRegistry jobType→执行函数，由本域jobTypes装配。
type JobExecutorRegistry map[string]JobExecutor

// Executor 返回任务类型的执行函数；未登记返回nil。
func (r JobExecutorRegistry) Executor(jobType string) JobExecutor {
	return r[jobType]
}

// OutboxRow outbox表一行的机制视图；payload为信封载荷原文。
type OutboxRow struct {
	EventID       Identifier
	EventType     string
	EventVersion  int
	Producer      string
	SubjectID     Identifier
	Revision      Revision
	OccurredAt    Timestamp
	DataEpoch     Identifier
	Payload       json.RawMessage
	PayloadHash   Hash
	State         string
	Attempts      int
	NextAttemptAt Timestamp
}

// outbox行状态。
const (
	OutboxPending     = "pending"
	OutboxRelayed     = "relayed"
	OutboxQuarantined = "quarantined"
	OutboxDiscarded   = "discarded"
)

// OutboxStore 本域outbox表访问接口；实现须与业务库同库同事务语义。
type OutboxStore interface {
	// ClaimBatch 按提交顺序领取到期pending行并写入行租约。
	ClaimBatch(ctx context.Context, limit int, lease time.Duration) ([]OutboxRow, error)
	// MarkRelayed 确认已提交给jobs。
	MarkRelayed(ctx context.Context, eventIDs []Identifier) error
	// ReleaseForRetry 释放租约并按退避安排下次尝试。
	ReleaseForRetry(ctx context.Context, eventID Identifier, attempts int, next time.Duration) error
	// Quarantine 隔离同ID异hash或信封非法的行。
	Quarantine(ctx context.Context, eventID Identifier, reason string) error
	// Discard 丢弃单个不属于当前代的pending行。
	Discard(ctx context.Context, eventID, epoch Identifier) error
	// DiscardOtherEpochs 丢弃不属于当前代的pending行，返回丢弃数。
	DiscardOtherEpochs(ctx context.Context, epoch Identifier) (int64, error)
}

// Tx 本域数据库事务句柄；*sql.Tx天然满足。
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// EventStream 事件订阅的增量读取；Next在流关闭或结束时返回io.EOF，
// LastID返回已读到的投递游标，供Last-Event-ID续传。
type EventStream interface {
	Next(ctx context.Context) (EventEnvelope, error)
	LastID() string
	Close() error
}

// SSEFrame 保留业务JSON本身，ID与事件名只用于SSE帧，不添加领域信封。
type SSEFrame struct {
	ID    string
	Event string
	Data  json.RawMessage
}

// JSONEventStream 的Next负责本域权限与持久续传，传输生命周期由公共pump管理。
type JSONEventStream interface {
	Next(context.Context) (SSEFrame, error)
	Close() error
}

// ModelEventStream 网关调用事件流的增量读取；事件为StreamEvent。
type ModelEventStream interface {
	Next(ctx context.Context) (StreamEvent, error)
	LastID() string
	Close() error
}

// EventPublisher 只暴露outbox转发所需的持久事件接收端口。
type EventPublisher interface {
	PublishEvent(ctx context.Context, envelope EventEnvelope) error
}

// WorkerJobsClient只暴露任务租约控制，不要求无关事件端口。
type WorkerJobsClient interface {
	ClaimJob(ctx context.Context, owner, workerID string, limit int) ([]ClaimedJob, error)
	StartJob(ctx context.Context, jobID, workerID string, fencingToken int64) error
	HeartbeatJob(ctx context.Context, jobID, workerID string, fencingToken int64) (HeartbeatInfo, error)
	CheckLease(ctx context.Context, jobID, workerID string, fencingToken int64) (bool, error)
	CompleteJob(ctx context.Context, jobID, workerID string, fencingToken int64, result *ObjectRef, paidResultRefs []Identifier) error
	FailJob(ctx context.Context, jobID, workerID string, fencingToken int64, errorCode string, paidResultRefs []Identifier) error
}

// JobsClient保留既有事件与任务组合端口。
type EventConsumerClient interface {
	SubscribeEvents(context.Context, string, string) (EventStream, error)
	AcknowledgeEvent(context.Context, string, Identifier) error
}

type JobsClient interface {
	WorkerJobsClient
	EventConsumerClient
	PublishEvent(ctx context.Context, envelope EventEnvelope) error
}

// GatewayClient model-gateway服务的调用接口；实现自动附服务断言。
type GatewayClient interface {
	CreateInference(ctx context.Context, idempotencyKey string, input CreateInvocationInput) (Invocation, error)
	// ResumeInference 打开自afterSequence之后的事件SSE流。
	ResumeInference(ctx context.Context, invocationID Identifier, afterSequence int64) (ModelEventStream, error)
	CancelInference(ctx context.Context, idempotencyKey, invocationID Identifier, reason string) error
}

// SiteConfigClient site-config服务的调用接口；实现自动附服务断言。
type SiteConfigClient interface {
	ReadRuntimeState(ctx context.Context) (RuntimeView, error)
}

// Application 服务装配完成的运行容器；Serve阻塞至停止。
type Application interface {
	Serve(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// BackgroundTask 是本域已登记的后台入口；由RunService统一启动和等待退出。
type BackgroundTask struct {
	Name string
	Run  func(context.Context) error
}

// BackgroundApplication 的Drain只停止新HTTP并等待请求，不关闭私有库。
// RunService先取消任务、Drain、等待任务退出，最后才Shutdown关闭资源。
type BackgroundApplication interface {
	Application
	BackgroundTasks() []BackgroundTask
	Drain(context.Context) error
}

// ServiceSpec 各服务main传入的装配说明。
type ServiceSpec struct {
	Module string
	// BuildApplication 装配本域应用；其中先ApplyRestoreSwap再打开入口。
	BuildApplication func(ctx context.Context, cfg ServiceConfig) (Application, error)
	// RuntimeClient 构造site-config运行状态客户端；无依赖域可为nil。
	RuntimeClient func(cfg ServiceConfig) (SiteConfigClient, error)
}

// 服务间请求的公共头名称。
const (
	HeaderRequestID       = "X-Request-Id"
	HeaderOperationID     = "X-Operation-Id"
	HeaderRootOperationID = "X-Root-Operation-Id"
	HeaderCostClass       = "X-Cost-Class"
	HeaderDeadline        = "X-Deadline"
	HeaderCancellationID  = "X-Cancellation-Id"
	HeaderDataEpoch       = "X-Data-Epoch"
	HeaderPortalProof     = "X-Portal-Proof"
	HeaderAuthorization   = "Authorization"
)

// AssertionScheme 服务断言的Authorization方案前缀。
const AssertionScheme = "Bearer "

type contextKey int

const (
	opContextKey contextKey = iota
	portalProofKey
)

// WithOperationContext 把可信操作上下文写入调用链，供出站传输层继承。
func WithOperationContext(ctx context.Context, op OperationContext) context.Context {
	return context.WithValue(ctx, opContextKey, op)
}

// OperationContextFrom 取出调用链上的操作上下文；不存在返回false。
func OperationContextFrom(ctx context.Context) (OperationContext, bool) {
	op, ok := ctx.Value(opContextKey).(OperationContext)
	return op, ok
}

// WithPortalProof 记录随调用链向下转发的用户委托证明原文。
func WithPortalProof(ctx context.Context, proof string) context.Context {
	return context.WithValue(ctx, portalProofKey, proof)
}

// PortalProofFrom 取出应向下游原样转发的委托证明；不存在返回空。
func PortalProofFrom(ctx context.Context) string {
	proof, _ := ctx.Value(portalProofKey).(string)
	return proof
}

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)
	hashPattern       = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ValidIdentifier 报告v是否符合公共Identifier约束。
func ValidIdentifier(v string) bool {
	return len(v) >= 1 && len(v) <= 128 && identifierPattern.MatchString(v)
}

// ValidHash 报告v是否为SHA-256小写十六进制。
func ValidHash(v Hash) bool { return hashPattern.MatchString(v) }

// HashBytes 计算字节内容的SHA-256小写十六进制。
func HashBytes(b []byte) Hash {
	sum := sha256.Sum256(b)
	return Hash(hex.EncodeToString(sum[:]))
}

// CanonicalJSON 统一内部JSON表示：保留整数精度并按对象键排序。
// 使用Go JSON编码约定，不声称实现RFC8785；调用方先固定数组顺序与空集合。
func CanonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, NewError(CodeIntegrityError, false, "值无法规范序列化")
	}
	var parsed any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	// raw由Marshal产生；此处仅转换对象表示，不重复做结构校验。
	_ = decoder.Decode(&parsed)
	return json.Marshal(parsed)
}

// Now 返回当前UTC时刻的RFC3339字符串。
func Now() Timestamp { return Timestamp(time.Now().UTC().Format(time.RFC3339Nano)) }

// ParseTimestamp 解析RFC3339 UTC时间；失败返回零值与错误。
func ParseTimestamp(v Timestamp) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, string(v))
	if err != nil {
		return time.Time{}, NewError(CodeValidation, false, "时间格式不合法")
	}
	return t.UTC(), nil
}

// NewIdentifier 生成26字节随机十六进制ID，用于jti、requestId等。
func NewIdentifier() Identifier {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand失败属于系统级不可恢复错误。
		panic("platform: 随机数不可用: " + err.Error())
	}
	return Identifier(hex.EncodeToString(b))
}
