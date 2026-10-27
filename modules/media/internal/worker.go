package media

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/libs/platform/worker"
	"nice-wiki/modules/media/internal/gen"
)

// WorkerLoopInput 的Client只接受受信装配，不从HTTP或任务对象反序列化。
type WorkerLoopInput struct {
	MaxConcurrency int                       `json:"maxConcurrency"`
	Client         platform.WorkerJobsClient `json:"-"`
}
type WorkerLoopResult struct {
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// cleanupWorkerOperation 以真实批准恢复站点/root，领取时新建期限，不沿用批准请求deadline。
func (s *localBlobStore) cleanupWorkerOperation(ctx context.Context, claimed platform.ClaimedJob) (OperationContext, error) {
	approval, found, err := s.loadCleanupApproval(ctx, s.db, claimed.InputRef.ObjectID)
	if err != nil {
		return OperationContext{}, err
	}
	if !found {
		return OperationContext{}, platform.NewError(platform.CodeNotFound, false, "任务人工批准对象不存在")
	}
	a := approval.Approval
	deadline := platform.Timestamp(time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano))
	op := OperationContext{RequestID: platform.NewIdentifier(), OperationID: claimed.OperationID, RootOperationID: claimed.RootOperationID, CallerService: "media", Actor: platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: a.SiteId, ExpiresAt: deadline}, Deadline: deadline, DataEpoch: claimed.DataEpoch, CostClass: platform.CostClassBackground}
	input := platform.JobExecutionInput{JobID: claimed.JobID, JobType: claimed.JobType, InputRef: claimed.InputRef, FencingToken: claimed.FencingToken, Attempt: claimed.Attempt, DataEpoch: claimed.DataEpoch}
	if err := checkCleanupJobBinding(op, approval, input); err != nil {
		return OperationContext{}, err
	}
	if op.DataEpoch != s.openedEpoch {
		return OperationContext{}, platform.NewError(platform.CodeVersionMismatch, false, "任务不属于本域已打开代次")
	}
	return op, nil
}

// RunWorker 只登记media.cleanup；领取、续租、取消及fence检查全部使用平台循环。
func RunWorker(ctx context.Context, op OperationContext, input WorkerLoopInput) (WorkerLoopResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return WorkerLoopResult{}, platform.NewError(platform.CodeForbidden, false, "领取循环仅供本域受信启动")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return WorkerLoopResult{}, err
	}
	if input.Client == nil || s.window.Runtime == nil {
		return WorkerLoopResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务租约客户端或运行权威未装配")
	}
	var completed, failed atomic.Int64
	config := platform.WorkerConfig{Owner: "media", JobTypes: []string{"media.cleanup"}, MaxConcurrency: input.MaxConcurrency, Runtime: s.window.Runtime, OperationContext: s.cleanupWorkerOperation,
		AdmitBusiness: lifecycle.SimpleAdmission(s.window.Runtime, ""), BindExecutionScope: func(scoped context.Context, _ platform.OperationContext, _ platform.ClaimedJob) (context.Context, error) {
			return scoped, nil
		}}
	executors := platform.JobExecutorRegistry{"media.cleanup": func(taskCtx context.Context, job platform.JobExecutionInput) (platform.JobExecutionResult, error) {
		taskOp, ok := platform.OperationContextFrom(taskCtx)
		if !ok {
			failed.Add(1)
			return platform.JobExecutionResult{}, platform.NewError(platform.CodeForbidden, false, "任务缺平台操作上下文")
		}
		approval, found, err := s.loadCleanupApproval(taskCtx, s.db, job.InputRef.ObjectID)
		if err == nil && !found {
			err = platform.NewError(platform.CodeNotFound, false, "任务批准对象缺失")
		}
		if err == nil {
			_, err = ExecuteManualCleanup(taskCtx, taskOp, ExecuteManualCleanupInput{PlanId: job.InputRef.ObjectID, ExpectedGeneration: approval.Approval.Plan.ReferenceGeneration, Job: job})
		}
		if err != nil {
			result, failure := s.cleanupWorkerFailure(taskCtx, taskOp, job, err)
			if result.State == platform.JobSucceeded {
				completed.Add(1)
			} else {
				failed.Add(1)
			}
			return result, failure
		}
		completed.Add(1)
		return platform.JobExecutionResult{State: platform.JobSucceeded}, nil
	}}
	err = worker.RunWorkerLoop(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op), input.Client, config, executors)
	return WorkerLoopResult{completed.Load(), failed.Load()}, err
}

// cleanupWorkerFailure 取消后只做独立有界只读证明，不把未知或已决定删除说成安全取消。
func (s *localBlobStore) cleanupWorkerFailure(ctx context.Context, op OperationContext, job platform.JobExecutionInput, executeErr error) (platform.JobExecutionResult, error) {
	cancelled := errors.Is(ctx.Err(), context.Canceled)
	var cause *platform.Error
	if !cancelled && errors.As(context.Cause(ctx), &cause) && cause.Code == platform.CodeCancelled {
		cancelled = true
	}
	if !cancelled {
		return platform.JobExecutionResult{}, executeErr
	}
	// 原任务ctx已取消；此窗口仅证明现有持久事实，不触发RPC或任何写入。
	proofCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(proofCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return platform.JobExecutionResult{}, mapMediaDBError(err, "开始安全取消只读证明失败")
	}
	defer tx.Rollback()
	approval, found, err := s.loadCleanupApproval(proofCtx, tx, job.InputRef.ObjectID)
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	if !found {
		return platform.JobExecutionResult{}, platform.NewError(platform.CodeIntegrityError, false, "安全取消证明缺原批准")
	}
	if err := checkCleanupJobBinding(op, approval, job); err != nil {
		return platform.JobExecutionResult{}, err
	}
	if _, err := s.readCleanupPlan(proofCtx, tx, op, job.InputRef.ObjectID); err != nil {
		return platform.JobExecutionResult{}, err
	}
	record, decided, err := s.readCleanupExecution(proofCtx, tx, approval)
	if err != nil {
		return platform.JobExecutionResult{}, err
	}
	if err := tx.Rollback(); err != nil {
		return platform.JobExecutionResult{}, mapMediaDBError(err, "结束安全取消只读证明失败")
	}
	if decided && record.Result != nil {
		return platform.JobExecutionResult{State: platform.JobSucceeded}, nil
	}
	if decided {
		return platform.JobExecutionResult{}, executeErr
	}
	return platform.JobExecutionResult{State: platform.JobCancelled, ErrorCode: platform.CodeCancelled}, nil
}

// NewCleanupWorkerClient只装配生产方schema，HTTP字段转换和租约机制统一由平台负责。
func NewCleanupWorkerClient(client *gen.JobsClient) (platform.WorkerJobsClient, error) {
	if client == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "资源任务真实jobs客户端未装配")
	}
	document := []byte(gen.PayloadValidationSpec)
	validators := make(worker.ResponseValidators, 6)
	for _, name := range []string{"ClaimJobResult", "StartJobResult", "HeartbeatJobResult", "CheckLeaseResult", "CompleteJobResult", "FailJobResult"} {
		validate, err := apivalidate.NewPayloadValidator(document, "Jobs"+name)
		if err != nil {
			return nil, err
		}
		validators[name] = validate
	}
	return worker.NewJobsClient(client, validators)
}

// NewMediaBackgroundTasks仅构造闭包，完整应用门槛失败时不会启动任何业务后台。
func NewMediaBackgroundTasks(op OperationContext, jobs *gen.JobsClient, events *queue.Queue) ([]platform.BackgroundTask, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return nil, platform.NewError(platform.CodeForbidden, false, "资源后台装配仅接受本域可信启动")
	}
	s, err := cleanupStore(op, false)
	if err != nil {
		return nil, err
	}
	if jobs == nil || events == nil || s.cleanupJobs == nil || s.window.Runtime == nil || s.window.WriteWindow == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "资源后台真实客户端或窗口未装配")
	}
	workerJobs, err := NewCleanupWorkerClient(jobs)
	if err != nil {
		return nil, err
	}
	eventClient := events.Consumer()
	eventAdmission := newMediaEventAdmission(s.window)
	return []platform.BackgroundTask{
		{Name: "media.ObjectOffload", Run: RunObjectOffload},
		{Name: "media.ConsumeEvents", Run: func(ctx context.Context) error {
			work := platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
			_, err := ConsumeEvents(work, op, ConsumeEventsInput{
				Client:        eventClient,
				Runtime:       s.window.Runtime,
				AdmitBatch:    eventAdmission,
				AdmitDelivery: eventAdmission,
				ConsumerID:    "media",
				MaxInFlight:   4,
			})
			return err
		}},
		{Name: "media.RunWorker", Run: func(ctx context.Context) error {
			_, err := RunWorker(ctx, op, WorkerLoopInput{MaxConcurrency: 2, Client: workerJobs})
			return err
		}},
	}, nil
}

const referenceChangedEvent = "content.resources.changed"

// resourceReferencesChangedPayloadV2 是content v2事件的完整pin材料。
// eventId只存在jobs信封，不能从payload补造或接受。
type resourceReferencesChangedPayloadV2 struct {
	References        []gen.AssetReference `json:"references"`
	SourceService     string               `json:"sourceService"`
	SourceOperationId Identifier           `json:"sourceOperationId"`
	SiteId            Identifier           `json:"siteId"`
	SourceObjectId    Identifier           `json:"sourceObjectId"`
	SourceRevisionId  Identifier           `json:"sourceRevisionId"`
	SourceVersion     gen.Revision         `json:"sourceVersion"`
	ReferenceDigest   Hash                 `json:"referenceDigest"`
}

// ConsumeEventsInput 由受信启动链装配，不接受HTTP或用户输入伪造客户端与运行权威。
type ConsumeEventsInput struct {
	Client        platform.EventConsumerClient                        `json:"-"`
	Runtime       func(context.Context) (platform.RuntimeView, error) `json:"-"`
	AdmitBatch    platform.WorkAdmission                              `json:"-"`
	AdmitDelivery platform.WorkAdmission                              `json:"-"`
	ConsumerID    string                                              `json:"consumerId"`
	MaxInFlight   int                                                 `json:"maxInFlight"`
}

type ConsumeEventsResult = platform.ConsumerRunResult

// ConsumeEvents 使用公共订阅循环处理content资源引用v2事件。
func ConsumeEvents(ctx context.Context, op OperationContext, input ConsumeEventsInput) (ConsumeEventsResult, error) {
	if op.CallerService != "media" || op.Actor.Kind != "service" || op.Actor.SubjectID != "media" {
		return ConsumeEventsResult{}, platform.NewError(platform.CodeForbidden, false, "引用事件订阅仅供本域受信启动")
	}
	if input.ConsumerID != "media" || input.Client == nil || input.Runtime == nil || input.AdmitBatch == nil || input.AdmitDelivery == nil {
		return ConsumeEventsResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "引用事件订阅依赖未装配")
	}
	validatePayload, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "ContentResourcesChangedPayloadV2")
	if err != nil {
		return ConsumeEventsResult{}, err
	}
	validateEvent := func(eventType string, version int, payload json.RawMessage) error {
		if eventType != referenceChangedEvent || version != 2 {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "引用事件版本未登记")
		}
		return validatePayload(payload)
	}
	config := platform.ConsumerConfig{
		ConsumerID:      input.ConsumerID,
		SiteID:          op.Actor.SiteID,
		Events:          []string{referenceChangedEvent + "@2"},
		MaxInFlight:     input.MaxInFlight,
		Runtime:         input.Runtime,
		AdmitBatch:      input.AdmitBatch,
		AdmitDelivery:   input.AdmitDelivery,
		ValidatePayload: validateEvent,
	}
	handlers := platform.EventHandlerRegistry{referenceChangedEvent: handleReferenceChangedEvent}
	return inbox.RunConsumerWithResult(platform.WithOperationContext(ctx, op), input.Client, config, handlers)
}

// handleReferenceChangedEvent 将jobs信封和v2 payload映射为本域精确事件输入。
func handleReferenceChangedEvent(ctx context.Context, delivery platform.EventDelivery) error {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "引用事件缺少可信操作上下文")
	}
	input, err := decodeReferenceChangedEvent(delivery.Envelope)
	if err != nil {
		return err
	}
	_, err = ApplyReferenceEvent(ctx, op, input)
	return err
}

func decodeReferenceChangedEvent(envelope platform.EventEnvelope) (ApplyReferenceEventInput, error) {
	if envelope.EventType != referenceChangedEvent || envelope.EventVersion != 2 {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeUnsupportedSchema, false, "引用事件版本未登记")
	}
	if envelope.Producer != "content" {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeIntegrityError, false, "引用事件生产方与事件类型不符")
	}
	var payload resourceReferencesChangedPayloadV2
	decoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeValidation, false, "引用事件v2载荷无法解码")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeValidation, false, "引用事件v2载荷包含多余数据")
	}
	if payload.SourceService != "content" {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeIntegrityError, false, "引用事件来源服务不符")
	}
	if envelope.SubjectID != payload.SourceObjectId || envelope.Revision != int64(payload.SourceVersion) {
		return ApplyReferenceEventInput{}, platform.NewError(platform.CodeIntegrityError, false, "引用事件信封水位与来源材料不符")
	}
	return ApplyReferenceEventInput{
		References:        payload.References,
		SourceService:     payload.SourceService,
		SourceOperationId: payload.SourceOperationId,
		SiteId:            payload.SiteId,
		SourceObjectId:    payload.SourceObjectId,
		SourceRevisionId:  payload.SourceRevisionId,
		SourceVersion:     payload.SourceVersion,
		ReferenceDigest:   payload.ReferenceDigest,
		EventId:           envelope.EventID,
	}, nil
}

func newMediaEventAdmission(window platform.WriteWindowConfig) platform.WorkAdmission {
	return func(ctx context.Context) (context.Context, func(), error) {
		work, _, cancel, err := lifecycle.NewWriteContext(ctx, window)
		if err != nil {
			return nil, nil, err
		}
		return work, cancel, nil
	}
}
