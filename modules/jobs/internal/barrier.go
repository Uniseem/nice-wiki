package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/jobs/internal/gen"
)

// ReadLeaseRuntime复用platform公开运行权威；维护/屏障期间从同一
// SnapshotManager读取内存中的屏障状态和截止时间，不读取control私有文件。
func (s *Store) ReadLeaseRuntime(runtime platform.RuntimeView) (LeaseRuntimeView, error) {
	if runtime.Phase == platform.PhaseRunning && !runtime.Maintenance && runtime.BarrierID == "" {
		return LeaseRuntimeView{Runtime: runtime}, nil
	}
	manager, err := s.currentSnapshotManager()
	if err != nil {
		return LeaseRuntimeView{}, err
	}
	status, deadline := manager.ReadBarrierStatus()
	if status == nil || deadline.IsZero() || !deadline.After(time.Now().UTC()) {
		return LeaseRuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域屏障状态或截止时间尚不可用")
	}
	if status.BarrierID != runtime.BarrierID || status.DataEpoch != runtime.DataEpoch ||
		(status.State != "draining" && status.State != "safe") {
		return LeaseRuntimeView{}, platform.NewError(platform.CodeVersionMismatch, true, "本域屏障状态与运行权威不一致")
	}
	return LeaseRuntimeView{Runtime: runtime, SafePointRequested: true, BarrierDeadline: deadline}, nil
}

// ReadReadinessEvidence正常running只依赖本域数据库journal；维护态读取真实
// SnapshotManager状态，safePoint只表示本域已安全封口，不把正常运行冒充安全点。
func (s *Store) ReadReadinessEvidence(runtime platform.RuntimeView, initialized bool) (bool, bool, bool, error) {
	if s == nil {
		return false, false, false, platform.NewError(platform.CodeDependencyUnavailable, true, "本域journal尚未完成恢复")
	}
	s.mu.RLock()
	closed, recovered := s.closed, s.journalRecovered
	s.mu.RUnlock()
	if closed || !recovered {
		return false, false, false, platform.NewError(platform.CodeDependencyUnavailable, true, "本域journal尚未完成恢复")
	}
	if runtime.Phase != platform.PhaseRunning || runtime.Maintenance || runtime.BarrierID != "" {
		manager, err := s.currentSnapshotManager()
		if err != nil {
			return false, false, false, err
		}
		status, deadline := manager.ReadBarrierStatus()
		if status == nil || deadline.IsZero() || !deadline.After(time.Now().UTC()) {
			return false, false, false, platform.NewError(platform.CodeDependencyUnavailable, true, "本域屏障状态或截止时间尚不可用")
		}
		if status.BarrierID != runtime.BarrierID || status.DataEpoch != runtime.DataEpoch ||
			(status.State != "draining" && status.State != "safe") {
			return false, false, false, platform.NewError(platform.CodeVersionMismatch, true, "本域屏障状态与运行权威不一致")
		}
		return true, status.State == "safe", false, nil
	}
	return true, false, initialized, nil
}

// ConfigureSnapshotManager安装同一platform manager，供本域屏障、快照和HTTP映射复用。
func (s *Store) ConfigureSnapshotManager(manager *lifecycle.SnapshotManager) error {
	if s == nil || manager == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本域SnapshotManager尚未装配")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储已关闭")
	}
	s.snapshotManager = manager
	return nil
}

func (s *Store) ConfigureDataProtectionClient(client *gen.DataProtectionClient) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.dataProtection = client
	s.mu.Unlock()
}

func (s *Store) snapshotClients() (*lifecycle.SnapshotManager, *gen.DataProtectionClient, error) {
	manager, err := s.currentSnapshotManager()
	if err != nil {
		return nil, nil, err
	}
	s.mu.RLock()
	client := s.dataProtection
	s.mu.RUnlock()
	return manager, client, nil
}

type BarrierRequest = gen.BarrierRequest
type BarrierStatus = gen.BarrierStatus
type CoordinateBarrierInput struct{ Request BarrierRequest }
type ReleaseBarrierInput struct {
	BarrierID string
	DataEpoch platform.Identifier
}
type ReleaseBarrierResult = gen.ReleaseBarrierResult

func barrierInput(op OperationContext, request BarrierRequest) (lifecycle.SnapshotBarrierInput, string, error) {
	if request.OperationId != op.OperationID || !platform.ValidIdentifier(request.BarrierId) {
		return lifecycle.SnapshotBarrierInput{}, "", platform.NewError(platform.CodeConflict, false, "屏障操作未绑定当前可信操作")
	}
	mode, ok := request.Mode.(string)
	if !ok || (mode != "backup" && mode != "restore") {
		return lifecycle.SnapshotBarrierInput{}, "", platform.NewError(platform.CodeValidation, false, "屏障模式不合法")
	}
	deadline := request.Deadline.UTC()
	if deadline.IsZero() {
		return lifecycle.SnapshotBarrierInput{}, "", platform.NewError(platform.CodeValidation, false, "屏障截止不合法")
	}
	step := "prepare"
	if request.Step != nil {
		step = string(*request.Step)
	}
	return lifecycle.SnapshotBarrierInput{BarrierID: request.BarrierId, OperationID: request.OperationId, Epoch: op.DataEpoch, Mode: mode, Deadline: deadline}, step, nil
}

func toBarrierStatus(status lifecycle.SnapshotBarrierStatus) gen.BarrierStatus {
	return gen.BarrierStatus{BarrierId: status.BarrierID, Owner: status.Owner, State: status.State, DataEpoch: status.DataEpoch, RevisionWatermark: gen.Revision(status.RevisionWatermark), SealProofHash: optionalHash(status.SealProofHash)}
}

func optionalHash(value string) *gen.Hash {
	if value == "" {
		return nil
	}
	result := gen.Hash(value)
	return &result
}

func toObservation(observation lifecycle.SnapshotBarrierObservation) gen.SnapshotBarrierObservation {
	return gen.SnapshotBarrierObservation{Owner: observation.Owner, ActivityRevision: int(observation.ActivityRevision), RevisionWatermark: int(observation.RevisionWatermark), Business: int(observation.Business), Cleanup: int(observation.Cleanup), FileOps: int(observation.FileOps), DrainCallbacks: int(observation.DrainCallbacks), Work: gen.SnapshotWorkState{OutboxRevision: int(observation.Work.OutboxRevision), WorkRevision: int(observation.Work.WorkRevision), PendingOutbox: int(observation.Work.PendingOutbox), PendingWork: int(observation.Work.PendingWork)}}
}

func runBarrierStep(ctx context.Context, op OperationContext, request BarrierRequest) (gen.BarrierStatus, error) {
	store := currentStore()
	if store == nil || op.CallerService != "data-protection" {
		return gen.BarrierStatus{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可以管理jobs屏障")
	}
	manager, _, err := store.snapshotClients()
	if err != nil {
		return gen.BarrierStatus{}, err
	}
	input, step, err := barrierInput(op, request)
	if err != nil {
		return gen.BarrierStatus{}, err
	}
	switch step {
	case "prepare":
		status, err := manager.PrepareBarrier(ctx, input)
		return toBarrierStatus(status), err
	case "drain":
		status, err := manager.DrainBarrier(ctx, input.BarrierID)
		return toBarrierStatus(status), err
	case "observe":
		observation, err := manager.ObserveBarrier(ctx, input.BarrierID)
		if err != nil {
			return gen.BarrierStatus{}, err
		}
		return gen.BarrierStatus{BarrierId: input.BarrierID, Owner: observation.Owner, State: "draining", DataEpoch: input.Epoch, RevisionWatermark: gen.Revision(observation.RevisionWatermark), Observation: func() *gen.SnapshotBarrierObservation { value := toObservation(observation); return &value }()}, nil
	case "seal":
		if request.SealProof == nil {
			return gen.BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "封口缺少完整proof")
		}
		proof := lifecycle.SnapshotSealProof{BarrierID: request.SealProof.BarrierId, DataEpoch: request.SealProof.DataEpoch, Observations: make([]lifecycle.SnapshotBarrierObservation, 0, len(request.SealProof.Observations)), BrokerBefore: lifecycle.SnapshotBrokerObservation{Revision: int64(request.SealProof.BrokerBefore.Revision), PendingDeliveries: int64(request.SealProof.BrokerBefore.PendingDeliveries), ActiveLeases: int64(request.SealProof.BrokerBefore.ActiveLeases)}, BrokerAfter: lifecycle.SnapshotBrokerObservation{Revision: int64(request.SealProof.BrokerAfter.Revision), PendingDeliveries: int64(request.SealProof.BrokerAfter.PendingDeliveries), ActiveLeases: int64(request.SealProof.BrokerAfter.ActiveLeases)}}
		if request.SealProof.NonJobsProofHash != nil {
			proof.NonJobsProofHash = string(*request.SealProof.NonJobsProofHash)
		}
		if request.SealProof.NonJobsSeals != nil {
			proof.NonJobsSeals = make([]lifecycle.SnapshotSealReceipt, 0, len(*request.SealProof.NonJobsSeals))
			for _, item := range *request.SealProof.NonJobsSeals {
				proof.NonJobsSeals = append(proof.NonJobsSeals, lifecycle.SnapshotSealReceipt{Owner: item.Owner, SealProofHash: string(item.SealProofHash)})
			}
		}
		for _, item := range request.SealProof.Observations {
			proof.Observations = append(proof.Observations, lifecycle.SnapshotBarrierObservation{Owner: item.Owner, ActivityRevision: int64(item.ActivityRevision), RevisionWatermark: int64(item.RevisionWatermark), Business: int64(item.Business), Cleanup: int64(item.Cleanup), FileOps: int64(item.FileOps), DrainCallbacks: int64(item.DrainCallbacks), Work: lifecycle.SnapshotWorkState{OutboxRevision: int64(item.Work.OutboxRevision), WorkRevision: int64(item.Work.WorkRevision), PendingOutbox: int64(item.Work.PendingOutbox), PendingWork: int64(item.Work.PendingWork)}})
		}
		status, err := manager.SealBarrier(ctx, proof)
		return toBarrierStatus(status), err
	case "resume-drain":
		if request.ExpectedSealHash == nil {
			return gen.BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "恢复清理缺少原封口摘要")
		}
		status, err := manager.ResumeDrain(ctx, input.BarrierID, string(*request.ExpectedSealHash))
		return toBarrierStatus(status), err
	default:
		return gen.BarrierStatus{}, platform.NewError(platform.CodeValidation, false, "屏障步骤不合法")
	}
}

func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error) {
	return runBarrierStep(ctx, op, request)
}

func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error) {
	store := currentStore()
	if store == nil || op.CallerService != "data-protection" {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可以解除jobs屏障")
	}
	if in.DataEpoch != op.DataEpoch {
		return ReleaseBarrierResult{}, platform.NewError(platform.CodeVersionMismatch, false, "解除屏障代次不符")
	}
	manager, _, err := store.snapshotClients()
	if err != nil {
		return ReleaseBarrierResult{}, err
	}
	status, err := manager.ReleaseBarrier(ctx, in.BarrierID)
	return ReleaseBarrierResult{Status: toBarrierStatus(status)}, err
}

func NewBarrierHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"coordinateBarrier": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.CoordinateBarrierRequestBody
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "屏障协调请求不可读"), 0)
				return
			}
			result, err := CoordinateBarrier(r.Context(), op, CoordinateBarrierInput{Request: body.Request})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"acquireBarrier": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.BarrierRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "屏障请求不可读"), 0)
				return
			}
			result, err := EnterBarrier(r.Context(), op, body)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"releaseBarrier": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := ReleaseBarrier(r.Context(), op, ReleaseBarrierInput{BarrierID: r.PathValue("barrierId"), DataEpoch: op.DataEpoch})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
	}
}

func (s *Store) currentSnapshotManager() (*lifecycle.SnapshotManager, error) {
	if s == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.snapshotManager == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域SnapshotManager尚未装配")
	}
	return s.snapshotManager, nil
}

func (s *Store) SnapshotDatabase() (*sql.DB, error) {
	if s == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.db == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务数据库尚未装配")
	}
	return s.db, nil
}

// SnapshotObserveWork在同一SQLite事务中读取平台持久revision和本域待决执行事实。
func (s *Store) SnapshotObserveWork(ctx context.Context) (lifecycle.SnapshotWorkState, error) {
	db, err := s.SnapshotDatabase()
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, mapQueueDBError(err)
	}
	defer tx.Rollback()
	work, err := observeSnapshotWorkTx(ctx, outbox.NewStore(db), tx)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	if err := tx.Commit(); err != nil {
		return lifecycle.SnapshotWorkState{}, mapQueueDBError(err)
	}
	return work, nil
}

func (s *Store) SnapshotWatermark(ctx context.Context) (int64, error) {
	db, err := s.SnapshotDatabase()
	if err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, mapQueueDBError(err)
	}
	defer tx.Rollback()
	work, err := observeSnapshotWorkTx(ctx, outbox.NewStore(db), tx)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, mapQueueDBError(err)
	}
	return work.WorkRevision, nil
}

// SnapshotSealWork在同一事务中重读业务待决事实，并以观察到的统一revision
// CAS封口outbox及所有已登记jobs业务表。
func (s *Store) SnapshotSealWork(ctx context.Context, work lifecycle.SnapshotWorkState, proofHash string) error {
	db, err := s.SnapshotDatabase()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return mapQueueDBError(err)
	}
	defer tx.Rollback()
	observed, err := observeSnapshotWorkTx(ctx, outbox.NewStore(db), tx)
	if err != nil {
		return err
	}
	if observed != work || work.WorkRevision != work.OutboxRevision || work.WorkRevision < 1 || work.PendingOutbox != 0 || work.PendingWork != 0 {
		return platform.NewError(platform.CodeConflict, true, "jobs工作水位或待决事实已改变")
	}
	if err := outbox.NewStore(db).SealTx(ctx, tx, work.WorkRevision, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return mapQueueDBError(err)
	}
	return nil
}

func (s *Store) SnapshotResumeWork(ctx context.Context, proofHash string) error {
	db, err := s.SnapshotDatabase()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return mapQueueDBError(err)
	}
	defer tx.Rollback()
	if err := outbox.NewStore(db).ResumeTx(ctx, tx, proofHash); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return mapQueueDBError(err)
	}
	return nil
}

// observeSnapshotWorkTx 复用同一SQLite事务读取统一outbox revision与本域
// 旧执行资格；revision直接来自平台control，不从业务表MAX推导。
func observeSnapshotWorkTx(ctx context.Context, outboxStore *outbox.Store, tx *sql.Tx) (lifecycle.SnapshotWorkState, error) {
	observation, err := outboxStore.ObserveTx(ctx, tx)
	if err != nil {
		return lifecycle.SnapshotWorkState{}, err
	}
	var pendingWork int64
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM leases)+(SELECT COUNT(*) FROM job_orphans)`).Scan(&pendingWork); err != nil {
		return lifecycle.SnapshotWorkState{}, mapQueueDBError(err)
	}
	if observation.Revision < 1 || observation.Pending < 0 || observation.Leased < 0 || pendingWork < 0 {
		return lifecycle.SnapshotWorkState{}, platform.NewError(platform.CodeIntegrityError, false, "jobs统一工作观察不完整")
	}
	return lifecycle.SnapshotWorkState{
		OutboxRevision: observation.Revision,
		WorkRevision:   observation.Revision,
		PendingOutbox:  observation.Pending,
		PendingWork:    pendingWork,
	}, nil
}
