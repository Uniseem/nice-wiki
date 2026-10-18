package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/jobs/internal/gen"
)

const (
	leaseDuration   = 30 * time.Second
	claimBatchLimit = 10
	ownerClaimLimit = 8
)

// LeaseRuntimeView 由装配层从运行权威及control屏障读取；不读取active区旧屏障表。
// 请求安全点时必须同时提供屏障截止，避免维护期间无限续租。
type LeaseRuntimeView struct {
	Runtime            platform.RuntimeView
	SafePointRequested bool
	BarrierDeadline    time.Time
}

type LeaseRuntimeProvider = func(context.Context) (LeaseRuntimeView, error)

type ClaimJobInput struct {
	JobTypes  []string `json:"jobTypes,omitempty"`
	Owner     string
	WorkerID  string
	Limit     int
	DataEpoch platform.Identifier
}

type ClaimJobResult = gen.ClaimJobResult

type StartJobInput struct {
	JobID        platform.Identifier
	WorkerID     string
	FencingToken int64
	DataEpoch    platform.Identifier
}

type StartJobResult = gen.StartJobResult

type HeartbeatJobInput struct {
	JobID        platform.Identifier `json:"jobId"`
	WorkerID     string              `json:"workerId"`
	FencingToken int64               `json:"fencingToken"`
	DataEpoch    platform.Identifier `json:"dataEpoch"`
	Progress     *ProgressCheckpoint `json:"progress,omitempty"`
}
type HeartbeatJobResult = gen.HeartbeatJobResult
type CheckLeaseInput = StartJobInput
type CheckLeaseResult = gen.CheckLeaseResult

// ClaimJob 原子领取本owner的到期任务；相同操作复用原批次，不延长原租约。
func ClaimJob(ctx context.Context, op OperationContext, in ClaimJobInput) (ClaimJobResult, error) {
	if op.CallerService != in.Owner {
		return ClaimJobResult{}, platform.NewError(platform.CodeForbidden, false, "只能领取本服务的任务")
	}
	for _, jobType := range in.JobTypes {
		if _, allowed := registeredJobTypes[in.Owner][jobType]; !allowed {
			return ClaimJobResult{}, platform.NewError(platform.CodeForbidden, false, "不能领取本owner未登记的任务类型")
		}
	}
	if in.Limit == 0 {
		in.Limit = claimBatchLimit
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return ClaimJobResult{}, err
	}
	defer work.close()
	if work.runtime.Runtime.Maintenance || work.runtime.SafePointRequested {
		return ClaimJobResult{}, platform.NewError(platform.CodeLocked, false, "维护或屏障期间停止领取任务")
	}
	request, _ := json.Marshal(in)
	prior, err := work.receipt(ctx, op, "claim", request)
	if err != nil {
		return ClaimJobResult{}, err
	}
	if prior != nil {
		var result ClaimJobResult
		if err := json.Unmarshal(prior, &result); err != nil {
			return ClaimJobResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务领取回执损坏")
		}
		return result, nil
	}
	active, err := work.countActiveLeases(ctx, in)
	if err != nil {
		return ClaimJobResult{}, err
	}
	limit := min(in.Limit, claimBatchLimit, max(0, ownerClaimLimit-active))
	result := ClaimJobResult{Jobs: []gen.ClaimedJob{}, FencingTokens: []int{}}
	if limit > 0 {
		jobs, err := work.claimCandidates(ctx, in)
		if err != nil {
			return ClaimJobResult{}, err
		}
		for _, job := range jobs[:min(limit, len(jobs))] {
			updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state='leased', attempt=attempt+1,
				fencing_counter=fencing_counter+1, revision=revision+1, updated_at=?
				WHERE job_id=? AND owner=? AND data_epoch=? AND state='queued' AND revision=?`,
				work.now.Format(time.RFC3339Nano), job.claimed.Job.JobId, in.Owner, in.DataEpoch, job.revision)
			if err != nil {
				return ClaimJobResult{}, mapQueueDBError(err)
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return ClaimJobResult{}, platform.NewError(platform.CodeConflict, false, "任务已被其他领取操作修改")
			}
			job.claimed.FencingToken++
			job.claimed.Attempt++
			job.claimed.Job.State = "leased"
			job.claimed.ExpiresAt = work.now.Add(leaseDuration)
			if _, err := work.tx.ExecContext(ctx, `INSERT INTO leases(job_id,worker_id,fencing_token,expires_at,last_heartbeat)
				VALUES(?,?,?,?,?) ON CONFLICT(job_id) DO UPDATE SET worker_id=excluded.worker_id,
				fencing_token=excluded.fencing_token,expires_at=excluded.expires_at,last_heartbeat=excluded.last_heartbeat`,
				job.claimed.Job.JobId, in.WorkerID, job.claimed.FencingToken, job.claimed.ExpiresAt.Format(time.RFC3339Nano), work.now.Format(time.RFC3339Nano)); err != nil {
				return ClaimJobResult{}, mapQueueDBError(err)
			}
			if err := work.enqueueState(job.claimed.Job.JobId, in.Owner, in.DataEpoch, "leased", job.revision+1); err != nil {
				return ClaimJobResult{}, err
			}
			result.Jobs = append(result.Jobs, job.claimed)
			result.FencingTokens = append(result.FencingTokens, job.claimed.FencingToken)
		}
	}
	if err := work.saveReceipt(ctx, op, "claim", request, result); err != nil {
		return ClaimJobResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return ClaimJobResult{}, mapQueueDBError(err)
	}
	return result, nil
}

// StartJob 只接受当前owner/worker/fence的有效租约，同fence重复开始不再写事件。
func StartJob(ctx context.Context, op OperationContext, in StartJobInput) (StartJobResult, error) {
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return StartJobResult{}, err
	}
	defer work.close()
	if work.runtime.Runtime.Maintenance || work.runtime.SafePointRequested {
		return StartJobResult{}, platform.NewError(platform.CodeLocked, false, "维护或屏障期间不开始新任务")
	}
	lease, err := work.readLease(ctx, op, in)
	if err != nil {
		return StartJobResult{}, err
	}
	if lease.cancelRequested {
		return StartJobResult{}, platform.NewError(platform.CodeLocked, false, "任务已请求取消，不能开始执行")
	}
	if lease.state == "running" {
		return StartJobResult{State: "running"}, nil
	}
	if lease.state != "leased" {
		return StartJobResult{}, leaseLost()
	}
	updated, err := work.tx.ExecContext(ctx, `UPDATE jobs SET state='running',revision=revision+1,updated_at=?
		WHERE job_id=? AND data_epoch=? AND state='leased' AND revision=?`, work.now.Format(time.RFC3339Nano), in.JobID, in.DataEpoch, lease.revision)
	if err != nil {
		return StartJobResult{}, mapQueueDBError(err)
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return StartJobResult{}, leaseLost()
	}
	if err := work.enqueueState(in.JobID, op.CallerService, in.DataEpoch, "running", lease.revision+1); err != nil {
		return StartJobResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return StartJobResult{}, mapQueueDBError(err)
	}
	return StartJobResult{State: "running"}, nil
}

// HeartbeatJob 续租不超过30秒或control屏障截止；重复操作保留原expiry。
func HeartbeatJob(ctx context.Context, op OperationContext, in HeartbeatJobInput) (HeartbeatJobResult, error) {
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return HeartbeatJobResult{}, err
	}
	defer work.close()
	lease, err := work.readLease(ctx, op, StartJobInput{JobID: in.JobID, WorkerID: in.WorkerID, FencingToken: in.FencingToken, DataEpoch: in.DataEpoch})
	if err != nil {
		return HeartbeatJobResult{}, err
	}
	request, _ := json.Marshal(in)
	prior, err := work.receipt(ctx, op, "heartbeat", request)
	if err != nil {
		return HeartbeatJobResult{}, err
	}
	result := HeartbeatJobResult{ExpiresAt: work.now.Add(leaseDuration)}
	if prior != nil {
		if err := json.Unmarshal(prior, &result); err != nil {
			return HeartbeatJobResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务心跳回执损坏")
		}
	}
	if work.runtime.SafePointRequested && work.runtime.BarrierDeadline.Before(result.ExpiresAt) {
		result.ExpiresAt = work.runtime.BarrierDeadline
	}
	if prior == nil {
		updated, err := work.tx.ExecContext(ctx, `UPDATE leases SET expires_at=?,last_heartbeat=?
			WHERE job_id=? AND worker_id=? AND fencing_token=? AND expires_at=?`,
			result.ExpiresAt.Format(time.RFC3339Nano), work.now.Format(time.RFC3339Nano), in.JobID, in.WorkerID, in.FencingToken, lease.expiresAt)
		if err != nil {
			return HeartbeatJobResult{}, mapQueueDBError(err)
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return HeartbeatJobResult{}, leaseLost()
		}
		if in.Progress != nil {
			if _, err := AppendProgress(ctx, op, work.tx, AppendProgressInput{JobID: in.JobID, ProgressCheckpoint: *in.Progress}); err != nil {
				return HeartbeatJobResult{}, err
			}
		}
		if err := work.saveReceipt(ctx, op, "heartbeat", request, result); err != nil {
			return HeartbeatJobResult{}, err
		}
	}
	// 标志按当前状态返回，避免旧回执掩盖后来发生的取消或屏障。
	result.CancelRequested = lease.cancelRequested
	result.SafePointRequested = work.runtime.SafePointRequested
	if err := work.tx.Commit(); err != nil {
		return HeartbeatJobResult{}, mapQueueDBError(err)
	}
	return result, nil
}

// CheckLease 仅返回执行权是否仍有效；owner写回仍须在自己的CAS中核对fence。
func CheckLease(ctx context.Context, op OperationContext, in CheckLeaseInput) (CheckLeaseResult, error) {
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return CheckLeaseResult{}, err
	}
	defer work.close()
	if _, err := work.readLease(ctx, op, in); err != nil {
		return CheckLeaseResult{}, err
	}
	return CheckLeaseResult{Valid: true}, nil
}

type leaseOperation struct {
	store   *Store
	tx      *sql.Tx
	runtime LeaseRuntimeView
	now     time.Time
}

func beginLeaseOperation(ctx context.Context, op OperationContext, epoch platform.Identifier) (*leaseOperation, error) {
	if epoch != op.DataEpoch {
		return nil, epochMismatch()
	}
	store := currentStore()
	if store == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	store.mu.RLock()
	if store.closed || store.leaseRuntime == nil {
		store.mu.RUnlock()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务租约运行权威尚未装配")
	}
	view, err := store.leaseRuntime(ctx)
	if err != nil {
		store.mu.RUnlock()
		return nil, err
	}
	if view.Runtime.DataEpoch != epoch {
		store.mu.RUnlock()
		return nil, epochMismatch()
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		store.mu.RUnlock()
		return nil, mapQueueDBError(err)
	}
	return &leaseOperation{store: store, tx: tx, runtime: view, now: time.Now().UTC()}, nil
}

func (work *leaseOperation) close() {
	_ = work.tx.Rollback()
	work.store.mu.RUnlock()
}

type leaseRecord struct {
	state           string
	revision        int64
	expiresAt       string
	cancelRequested bool
}

func (work *leaseOperation) readLease(ctx context.Context, op OperationContext, in StartJobInput) (leaseRecord, error) {
	if (work.runtime.Runtime.Maintenance && !work.runtime.SafePointRequested) || (work.runtime.SafePointRequested && work.runtime.BarrierDeadline.IsZero()) {
		return leaseRecord{}, platform.NewError(platform.CodeDependencyUnavailable, true, "维护安全点或屏障截止尚不可用")
	}
	var lease leaseRecord
	var owner, epoch string
	var currentFence int64
	var worker sql.NullString
	var token sql.NullInt64
	var expires sql.NullString
	err := work.tx.QueryRowContext(ctx, `SELECT j.owner,j.data_epoch,j.state,j.revision,j.fencing_counter,j.cancel_requested,l.worker_id,l.fencing_token,l.expires_at
		FROM jobs j LEFT JOIN leases l ON l.job_id=j.job_id WHERE j.job_id=?`, in.JobID).
		Scan(&owner, &epoch, &lease.state, &lease.revision, &currentFence, &lease.cancelRequested, &worker, &token, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return leaseRecord{}, leaseLost()
	}
	if err != nil {
		return leaseRecord{}, mapQueueDBError(err)
	}
	if owner != op.CallerService {
		return leaseRecord{}, platform.NewError(platform.CodeForbidden, false, "只能访问本服务的任务租约")
	}
	if epoch != in.DataEpoch {
		return leaseRecord{}, epochMismatch()
	}
	if worker.String != in.WorkerID || token.Int64 != in.FencingToken || currentFence != in.FencingToken || !expires.Valid ||
		(lease.state != "leased" && lease.state != "running" && lease.state != "cancel-requested") {
		return leaseRecord{}, leaseLost()
	}
	expiry, err := time.Parse(time.RFC3339Nano, expires.String)
	if err != nil {
		return leaseRecord{}, platform.NewError(platform.CodeIntegrityError, false, "任务租约截止损坏")
	}
	if !expiry.After(work.now) || (work.runtime.SafePointRequested && !work.runtime.BarrierDeadline.After(work.now)) {
		return leaseRecord{}, leaseLost()
	}
	lease.expiresAt = expires.String
	return lease, nil
}

type claimCandidate struct {
	claimed   gen.ClaimedJob
	revision  int64
	notBefore time.Time
	createdAt time.Time
}

func (work *leaseOperation) claimCandidates(ctx context.Context, in ClaimJobInput) ([]claimCandidate, error) {
	query := `SELECT job_id,operation_id,root_operation_id,type,immutable_input_ref,attempt,fencing_counter,
 revision,not_before,created_at FROM jobs WHERE owner=? AND data_epoch=? AND state='queued' AND root_operation_id<>'' AND cancel_requested=0
 AND julianday(not_before)<=julianday(?)`
	args := []any{in.Owner, in.DataEpoch, work.now.Format(time.RFC3339Nano)}
	if len(in.JobTypes) > 0 {
		query += " AND type IN (" + strings.TrimSuffix(strings.Repeat("?,", len(in.JobTypes)), ",") + ")"
		for _, jobType := range in.JobTypes {
			args = append(args, jobType)
		}
	}
	rows, err := work.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	defer rows.Close()
	jobs := make([]claimCandidate, 0, claimBatchLimit)
	for rows.Next() {
		var job claimCandidate
		var input []byte
		var notBefore, createdAt string
		if err := rows.Scan(&job.claimed.Job.JobId, &job.claimed.Job.OperationId, &job.claimed.RootOperationId, &job.claimed.Type, &input, &job.claimed.Attempt,
			&job.claimed.FencingToken, &job.revision, &notBefore, &createdAt); err != nil {
			return nil, mapQueueDBError(err)
		}
		job.notBefore, err = time.Parse(time.RFC3339Nano, notBefore)
		if err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "任务调度时间损坏")
		}
		if job.notBefore.After(work.now) {
			continue
		}
		job.createdAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "任务创建时间损坏")
		}
		// SQLite按毫秒预筛为到期候选超集；Go精确比较时间，并只保留最早10项。
		position := sort.Search(len(jobs), func(i int) bool {
			if !jobs[i].notBefore.Equal(job.notBefore) {
				return jobs[i].notBefore.After(job.notBefore)
			}
			if !jobs[i].createdAt.Equal(job.createdAt) {
				return jobs[i].createdAt.After(job.createdAt)
			}
			return jobs[i].claimed.Job.JobId > job.claimed.Job.JobId
		})
		if position >= claimBatchLimit {
			continue
		}
		if err := json.Unmarshal(input, &job.claimed.InputRef); err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "任务输入引用损坏")
		}
		job.claimed.Owner = in.Owner
		job.claimed.DataEpoch = in.DataEpoch
		if len(jobs) < claimBatchLimit {
			jobs = append(jobs, job)
		}
		copy(jobs[position+1:], jobs[position:len(jobs)-1])
		jobs[position] = job
	}
	if err := rows.Err(); err != nil {
		return nil, mapQueueDBError(err)
	}
	return jobs, nil
}

func (work *leaseOperation) receipt(ctx context.Context, op OperationContext, kind string, request []byte) ([]byte, error) {
	var hash string
	var result []byte
	err := work.tx.QueryRowContext(ctx, `SELECT request_hash,result_json FROM lease_operations
		WHERE owner=? AND data_epoch=? AND operation_kind=? AND operation_id=?`, op.CallerService, op.DataEpoch, kind, op.OperationID).Scan(&hash, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapQueueDBError(err)
	}
	if hash != platform.HashBytes(request) {
		return nil, platform.NewError(platform.CodeConflict, false, "租约操作键已绑定不同输入")
	}
	return result, nil
}

func (work *leaseOperation) saveReceipt(ctx context.Context, op OperationContext, kind string, request []byte, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "租约回执编码失败")
	}
	_, err = work.tx.ExecContext(ctx, `INSERT INTO lease_operations(owner,data_epoch,operation_kind,operation_id,request_hash,result_json)
		VALUES(?,?,?,?,?,?)`, op.CallerService, op.DataEpoch, kind, op.OperationID, platform.HashBytes(request), encoded)
	return mapQueueDBError(err)
}

func (work *leaseOperation) enqueueState(jobID, owner, epoch, state string, revision int64) error {
	payload, err := json.Marshal(jobStateChangedPayload{JobID: jobID, State: state, JobRevision: revision, Owner: owner})
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "任务状态事件编码失败")
	}
	return outbox.EnqueueEvent(work.tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: jobEventType,
		EventVersion: jobEventVersion, Producer: "jobs", SubjectID: jobID, Revision: revision, OccurredAt: work.now.Format(time.RFC3339Nano), DataEpoch: epoch, Payload: payload})
}

func leaseLost() error {
	return platform.NewError(platform.CodeStaleResult, false, "任务租约已失效")
}

// countActiveLeases 直接比较RFC3339时间，避免SQLite毫秒截断放宽owner并发上限。
func (work *leaseOperation) countActiveLeases(ctx context.Context, in ClaimJobInput) (int, error) {
	rows, err := work.tx.QueryContext(ctx, `SELECT l.expires_at FROM jobs j JOIN leases l ON l.job_id=j.job_id
		WHERE j.owner=? AND j.data_epoch=? AND j.state IN ('leased','running','cancel-requested')`, in.Owner, in.DataEpoch)
	if err != nil {
		return 0, mapQueueDBError(err)
	}
	defer rows.Close()
	active := 0
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return 0, mapQueueDBError(err)
		}
		expires, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return 0, platform.NewError(platform.CodeIntegrityError, false, "任务租约截止损坏")
		}
		if expires.After(work.now) {
			active++
		}
	}
	return active, mapQueueDBError(rows.Err())
}

// NewLeaseHandlers 只做统一鉴权与严格结构校验之后的字段装配。
func NewLeaseHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"claimJob": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input ClaimJobInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "领取请求绑定失败"), 0)
				return
			}
			input.DataEpoch = op.DataEpoch
			result, err := ClaimJob(r.Context(), op, input)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"startJob": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input StartJobInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "启动请求绑定失败"), 0)
				return
			}
			input.JobID = r.PathValue("jobId")
			input.DataEpoch = op.DataEpoch
			result, err := StartJob(r.Context(), op, input)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"heartbeatJob": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input HeartbeatJobInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "续租请求绑定失败"), 0)
				return
			}
			input.JobID = r.PathValue("jobId")
			input.DataEpoch = op.DataEpoch
			result, err := HeartbeatJob(r.Context(), op, input)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"checkLease": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			fence, _ := strconv.ParseInt(r.URL.Query().Get("fencingToken"), 10, 64)
			result, err := CheckLease(r.Context(), op, CheckLeaseInput{JobID: r.PathValue("jobId"), WorkerID: r.URL.Query().Get("workerId"), FencingToken: fence, DataEpoch: op.DataEpoch})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"completeJob": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input CompleteJobInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "完成请求绑定失败"), 0)
				return
			}
			input.JobID = r.PathValue("jobId")
			input.DataEpoch = op.DataEpoch
			result, err := CompleteJob(r.Context(), op, input)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"failJob": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input FailJobInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "失败请求绑定失败"), 0)
				return
			}
			input.JobID = r.PathValue("jobId")
			input.DataEpoch = op.DataEpoch
			result, err := FailJob(r.Context(), op, input)
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
	}
}
