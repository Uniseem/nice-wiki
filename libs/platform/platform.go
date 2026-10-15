// Package platform 提供各服务共用的契约类型与基础设施接口。
//
// 本文件只承载类型与接口定义，不含业务规则；行为机制分布在
// serviceauth、opctx、problem、outbox、inbox、worker、modelclient、
// lifecycle 各包，由各服务装配后调用。
package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	PermissionRevision Revision   `json:"permissionRevision"`
	Grants             []string   `json:"grants"`
	ExpiresAt          Timestamp  `json:"expiresAt"`
}

// OperationContext 服务间可信操作元数据。
type OperationContext struct {
	RequestID       Identifier  `json:"requestId"`
	OperationID     Identifier  `json:"operationId"`
	RootOperationID Identifier  `json:"rootOperationId"`
	CallerService   string      `json:"callerService"`
	Actor           AuthContext `json:"actor"`
	Deadline        Timestamp   `json:"deadline"`
	DataEpoch       Identifier  `json:"dataEpoch"`
	CostClass       string      `json:"costClass,omitempty"`
	CancellationID  Identifier  `json:"cancellationId,omitempty"`
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

// AssertionClaims 服务断言声明；expiresAt与issuedAt相差≤60秒。
type AssertionClaims struct {
	Issuer    string     `json:"iss"`
	Audience  string     `json:"aud"`
	Method    string     `json:"method"`
	Path      string     `json:"path"`
	BodyHash  Hash       `json:"bodyHash"`
	IssuedAt  Timestamp  `json:"issuedAt"`
	ExpiresAt Timestamp  `json:"expiresAt"`
	JTI       Identifier `json:"jti"`
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
	Route    string `json:"route"`
	BodyHash Hash   `json:"bodyHash"`
}

// RuntimeView site-config权威运行状态摘要。
type RuntimeView struct {
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
	// Runtime 读取当前运行状态，用于屏障与换代判断；可为nil。
	Runtime func(ctx context.Context) (RuntimeView, error) `json:"-"`
}

// ConsumerConfig 事件订阅循环参数；事件为“名称@版本”。
type ConsumerConfig struct {
	ConsumerID  string   `json:"consumerId"`
	Events      []string `json:"events"`
	MaxInFlight int      `json:"maxInFlight"`
	// Runtime 读取当前运行状态；可为nil，nil时按running处理。
	Runtime func(ctx context.Context) (RuntimeView, error) `json:"-"`
	// ValidatePayload 按生产方payloadSchema校验载荷；nil时只做信封校验。
	ValidatePayload func(eventType string, version int, payload json.RawMessage) error `json:"-"`
}

// WorkerConfig 任务领取循环参数；owner为本域slug。
type WorkerConfig struct {
	Owner          string   `json:"owner"`
	JobTypes       []string `json:"jobTypes"`
	MaxConcurrency int      `json:"maxConcurrency"`
	// Runtime 读取当前运行状态；维护期间暂停领取。可为nil。
	Runtime func(ctx context.Context) (RuntimeView, error) `json:"-"`
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
type StreamEvent struct {
	Sequence   int64       `json:"sequence"`
	Event      string      `json:"event"`
	Delta      string      `json:"delta,omitempty"`
	Invocation *Invocation `json:"invocation,omitempty"`
	Code       string      `json:"code,omitempty"`
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
	Module         string            `json:"module"`
	Listen         string            `json:"listen"`
	DataRoot       string            `json:"dataRoot"`
	TrustAnchorRef string            `json:"trustAnchorRef"`
	SigningKeyRef  string            `json:"signingKeyRef"`
	Dependencies   map[string]string `json:"dependencies"`
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
	JobID        Identifier `json:"jobId"`
	OperationID  Identifier `json:"operationId"`
	JobType      string     `json:"type"`
	InputRef     ObjectRef  `json:"inputRef"`
	FencingToken int64      `json:"fencingToken"`
	Attempt      int        `json:"attempt"`
	DataEpoch    Identifier `json:"dataEpoch"`
	ExpiresAt    Timestamp  `json:"expiresAt"`
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
	JobType      string     `json:"jobType"`
	InputRef     Identifier `json:"inputRef"`
	FencingToken int64      `json:"fencingToken"`
	Attempt      int        `json:"attempt"`
	DataEpoch    Identifier `json:"dataEpoch"`
}

// JobExecutionResult 任务执行结果；retry表示可重试失败。
type JobExecutionResult struct {
	State     string `json:"state"`
	ResultRef string `json:"resultRef,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
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

// ModelEventStream 网关调用事件流的增量读取；事件为StreamEvent。
type ModelEventStream interface {
	Next(ctx context.Context) (StreamEvent, error)
	LastID() string
	Close() error
}

// JobsClient jobs服务的调用接口；实现自动附服务断言。
type JobsClient interface {
	PublishEvent(ctx context.Context, envelope EventEnvelope) error
	SubscribeEvents(ctx context.Context, consumerID, lastEventID string) (EventStream, error)
	AcknowledgeEvent(ctx context.Context, consumerID, eventID Identifier) error
	ClaimJob(ctx context.Context, owner, workerID string, limit int) ([]ClaimedJob, error)
	StartJob(ctx context.Context, jobID, workerID string, fencingToken int64) error
	HeartbeatJob(ctx context.Context, jobID, workerID string, fencingToken int64) (HeartbeatInfo, error)
	CheckLease(ctx context.Context, jobID, workerID string, fencingToken int64) (bool, error)
	CompleteJob(ctx context.Context, jobID string, fencingToken int64, result ObjectRef, paidResultRefs []Identifier) error
	FailJob(ctx context.Context, jobID string, fencingToken int64, errorCode string) error
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
