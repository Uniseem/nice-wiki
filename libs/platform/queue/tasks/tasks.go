// Package tasks 是任务队列的执行端：各服务的 worker 直接在共用的任务表上领取、开始、续租、核对、完成和失败，
// 不再经 jobs 的 HTTP 接口中转。任务的提交、取消、列表、重放和定时调度仍归 jobs（ops 进程）管理。
//
// 任务表在 jobs schema（由 jobs 组件建表和迁移），部署时授权给共享角色 nw_shared，各组件角色作为成员读写。
// 每个组件按自己的数据目录打开“<数据目录>/active/jobs.db”，pgdb 把它映射到同一个 jobs schema；
// 写事务取该 schema 的建议锁，跨进程串行，语义与原来 jobs 单写者一致。
package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	_ "nice-wiki/libs/platform/pgdb"
	"nice-wiki/libs/platform/queue"
)

const (
	leaseDuration   = 30 * time.Second
	claimBatchLimit = 10
	ownerClaimLimit = 8
	jobEventType    = "jobs.job-state-changed.v1"
)

// Store 是某个组件打开的任务表句柄；runtime 读当前数据代次，代次不符时拒绝写。
type Store struct {
	db      *sql.DB
	runtime func(context.Context) (platform.RuntimeView, error)
}

// Open 打开共用任务表（不建表，jobs 组件负责迁移）。dataRoot 是本组件的数据目录。
func Open(dataRoot string, runtime func(context.Context) (platform.RuntimeView, error)) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dataRoot, "active", "jobs.db"))
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "打开任务表失败")
	}
	db.SetMaxOpenConns(2)
	return &Store{db: db, runtime: runtime}, nil
}

// Close 关闭连接池。
func (s *Store) Close() error { return s.db.Close() }

type objectRef = platform.ObjectRef

type claimedJob struct {
	Job struct {
		JobID       string `json:"jobId"`
		OperationID string `json:"operationId"`
		State       string `json:"state"`
	} `json:"job"`
	Owner           string    `json:"owner"`
	Type            string    `json:"type"`
	InputRef        objectRef `json:"inputRef"`
	FencingToken    int64     `json:"fencingToken"`
	Attempt         int       `json:"attempt"`
	DataEpoch       string    `json:"dataEpoch"`
	ExpiresAt       time.Time `json:"expiresAt"`
	RootOperationID string    `json:"rootOperationId"`
}

type claimResult struct {
	Jobs          []claimedJob `json:"jobs"`
	FencingTokens []int64      `json:"fencingTokens"`
}

type heartbeatResult struct {
	ExpiresAt       time.Time `json:"expiresAt"`
	CancelRequested bool      `json:"cancelRequested"`
}

type leaseInput struct {
	JobID        string `json:"jobId"`
	WorkerID     string `json:"workerId"`
	FencingToken int64  `json:"fencingToken"`
	DataEpoch    string `json:"dataEpoch"`
}

func leaseLost() error {
	return platform.NewError(platform.CodeStaleResult, false, "任务租约已失效")
}

func epochMismatch() error {
	return platform.NewError(platform.CodeVersionMismatch, false, "任务数据代次与当前运行代次不一致")
}

func dbError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, true, "任务操作已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "任务操作已超过截止时间")
	}
	return platform.NewError(platform.CodeInternal, true, "任务持久化失败")
}

type work struct {
	tx  *sql.Tx
	now time.Time
}

func (s *Store) begin(ctx context.Context, op platform.OperationContext) (*work, error) {
	view, err := s.runtime(ctx)
	if err != nil {
		return nil, err
	}
	if view.DataEpoch != op.DataEpoch {
		return nil, epochMismatch()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, dbError(err)
	}
	return &work{tx: tx, now: time.Now().UTC()}, nil
}

func (w *work) close() { _ = w.tx.Rollback() }

type leaseRecord struct {
	state           string
	revision        int64
	expiresAt       string
	cancelRequested bool
}

func (w *work) readLease(ctx context.Context, op platform.OperationContext, in leaseInput) (leaseRecord, error) {
	var lease leaseRecord
	var owner, epoch string
	var currentFence int64
	var worker sql.NullString
	var token sql.NullInt64
	var expires sql.NullString
	err := w.tx.QueryRowContext(ctx, `SELECT j.owner,j.data_epoch,j.state,j.revision,j.fencing_counter,j.cancel_requested,l.worker_id,l.fencing_token,l.expires_at
		FROM jobs j LEFT JOIN leases l ON l.job_id=j.job_id WHERE j.job_id=?`, in.JobID).
		Scan(&owner, &epoch, &lease.state, &lease.revision, &currentFence, &lease.cancelRequested, &worker, &token, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return leaseRecord{}, leaseLost()
	}
	if err != nil {
		return leaseRecord{}, dbError(err)
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
	if !expiry.After(w.now) {
		return leaseRecord{}, leaseLost()
	}
	lease.expiresAt = expires.String
	return lease, nil
}

// receipt 读同一操作键的回执（重复调用返回原结果，不重复生效）。
func (w *work) receipt(ctx context.Context, op platform.OperationContext, kind string, request []byte) ([]byte, error) {
	var hash string
	var result []byte
	err := w.tx.QueryRowContext(ctx, `SELECT request_hash,result_json FROM lease_operations
		WHERE owner=? AND data_epoch=? AND operation_kind=? AND operation_id=?`, op.CallerService, op.DataEpoch, kind, op.OperationID).Scan(&hash, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbError(err)
	}
	if hash != platform.HashBytes(request) {
		return nil, platform.NewError(platform.CodeConflict, false, "租约操作键已绑定不同输入")
	}
	return result, nil
}

func (w *work) saveReceipt(ctx context.Context, op platform.OperationContext, kind string, request []byte, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "租约回执编码失败")
	}
	_, err = w.tx.ExecContext(ctx, `INSERT INTO lease_operations(owner,data_epoch,operation_kind,operation_id,request_hash,result_json)
		VALUES(?,?,?,?,?,?)`, op.CallerService, op.DataEpoch, kind, op.OperationID, platform.HashBytes(request), encoded)
	return dbError(err)
}

func (w *work) enqueueState(jobID, owner, epoch, state string, revision int64) error {
	payload, err := json.Marshal(struct {
		JobID       string `json:"jobId"`
		State       string `json:"state"`
		JobRevision int64  `json:"jobRevision"`
		Owner       string `json:"owner"`
	}{jobID, state, revision, owner})
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "任务状态事件编码失败")
	}
	return queue.EnqueueEvent(w.tx, platform.EventEnvelope{EventID: platform.NewIdentifier(), EventType: jobEventType,
		EventVersion: 1, Producer: "jobs", SubjectID: jobID, Revision: revision, OccurredAt: w.now.Format(time.RFC3339Nano), DataEpoch: epoch, Payload: payload})
}

// claim 原子领取本 owner 的到期任务；同一操作键复用原批次，不延长原租约。
func (s *Store) claim(ctx context.Context, op platform.OperationContext, workerID string, limit int, jobTypes []string) (claimResult, error) {
	if limit == 0 {
		limit = claimBatchLimit
	}
	w, err := s.begin(ctx, op)
	if err != nil {
		return claimResult{}, err
	}
	defer w.close()
	request, _ := json.Marshal(struct {
		JobTypes []string `json:"jobTypes,omitempty"`
		Owner    string
		WorkerID string
		Limit    int
		Epoch    string
	}{jobTypes, op.CallerService, workerID, limit, op.DataEpoch})
	prior, err := w.receipt(ctx, op, "claim", request)
	if err != nil {
		return claimResult{}, err
	}
	if prior != nil {
		var result claimResult
		if err := json.Unmarshal(prior, &result); err != nil {
			return claimResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务领取回执损坏")
		}
		return result, nil
	}
	active, err := w.countActiveLeases(ctx, op)
	if err != nil {
		return claimResult{}, err
	}
	limit = min(limit, claimBatchLimit, max(0, ownerClaimLimit-active))
	result := claimResult{Jobs: []claimedJob{}, FencingTokens: []int64{}}
	if limit > 0 {
		jobs, err := w.claimCandidates(ctx, op, jobTypes)
		if err != nil {
			return claimResult{}, err
		}
		for _, job := range jobs[:min(limit, len(jobs))] {
			updated, err := w.tx.ExecContext(ctx, `UPDATE jobs SET state='leased', attempt=attempt+1,
				fencing_counter=fencing_counter+1, revision=revision+1, updated_at=?
				WHERE job_id=? AND owner=? AND data_epoch=? AND state='queued' AND revision=?`,
				w.now.Format(time.RFC3339Nano), job.claimed.Job.JobID, op.CallerService, op.DataEpoch, job.revision)
			if err != nil {
				return claimResult{}, dbError(err)
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return claimResult{}, platform.NewError(platform.CodeConflict, false, "任务已被其他领取操作修改")
			}
			job.claimed.FencingToken++
			job.claimed.Attempt++
			job.claimed.Job.State = "leased"
			job.claimed.ExpiresAt = w.now.Add(leaseDuration)
			if _, err := w.tx.ExecContext(ctx, `INSERT INTO leases(job_id,worker_id,fencing_token,expires_at,last_heartbeat)
				VALUES(?,?,?,?,?) ON CONFLICT(job_id) DO UPDATE SET worker_id=excluded.worker_id,
				fencing_token=excluded.fencing_token,expires_at=excluded.expires_at,last_heartbeat=excluded.last_heartbeat`,
				job.claimed.Job.JobID, workerID, job.claimed.FencingToken, job.claimed.ExpiresAt.Format(time.RFC3339Nano), w.now.Format(time.RFC3339Nano)); err != nil {
				return claimResult{}, dbError(err)
			}
			if err := w.enqueueState(job.claimed.Job.JobID, op.CallerService, op.DataEpoch, "leased", job.revision+1); err != nil {
				return claimResult{}, err
			}
			result.Jobs = append(result.Jobs, job.claimed)
			result.FencingTokens = append(result.FencingTokens, job.claimed.FencingToken)
		}
	}
	if err := w.saveReceipt(ctx, op, "claim", request, result); err != nil {
		return claimResult{}, err
	}
	if err := w.tx.Commit(); err != nil {
		return claimResult{}, dbError(err)
	}
	return result, nil
}

type claimCandidate struct {
	claimed   claimedJob
	revision  int64
	notBefore time.Time
	createdAt time.Time
}

func (w *work) claimCandidates(ctx context.Context, op platform.OperationContext, jobTypes []string) ([]claimCandidate, error) {
	query := `SELECT job_id,operation_id,root_operation_id,type,immutable_input_ref,attempt,fencing_counter,
 revision,not_before,created_at FROM jobs WHERE owner=? AND data_epoch=? AND state='queued' AND root_operation_id<>'' AND cancel_requested=0
 AND julianday(not_before)<=julianday(?)`
	args := []any{op.CallerService, op.DataEpoch, w.now.Format(time.RFC3339Nano)}
	if len(jobTypes) > 0 {
		query += " AND type IN (" + strings.TrimSuffix(strings.Repeat("?,", len(jobTypes)), ",") + ")"
		for _, jobType := range jobTypes {
			args = append(args, jobType)
		}
	}
	rows, err := w.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	jobs := make([]claimCandidate, 0, claimBatchLimit)
	for rows.Next() {
		var job claimCandidate
		var input []byte
		var notBefore, createdAt string
		if err := rows.Scan(&job.claimed.Job.JobID, &job.claimed.Job.OperationID, &job.claimed.RootOperationID, &job.claimed.Type, &input, &job.claimed.Attempt,
			&job.claimed.FencingToken, &job.revision, &notBefore, &createdAt); err != nil {
			return nil, dbError(err)
		}
		job.notBefore, err = time.Parse(time.RFC3339Nano, notBefore)
		if err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "任务调度时间损坏")
		}
		if job.notBefore.After(w.now) {
			continue
		}
		job.createdAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "任务创建时间损坏")
		}
		// 先按毫秒粗筛出候选，再精确比较时间，只保留最早的 10 项。
		position := sort.Search(len(jobs), func(i int) bool {
			if !jobs[i].notBefore.Equal(job.notBefore) {
				return jobs[i].notBefore.After(job.notBefore)
			}
			if !jobs[i].createdAt.Equal(job.createdAt) {
				return jobs[i].createdAt.After(job.createdAt)
			}
			return jobs[i].claimed.Job.JobID > job.claimed.Job.JobID
		})
		if position >= claimBatchLimit {
			continue
		}
		if err := json.Unmarshal(input, &job.claimed.InputRef); err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "任务输入引用损坏")
		}
		job.claimed.Owner = op.CallerService
		job.claimed.DataEpoch = op.DataEpoch
		if len(jobs) < claimBatchLimit {
			jobs = append(jobs, job)
		}
		copy(jobs[position+1:], jobs[position:len(jobs)-1])
		jobs[position] = job
	}
	if err := rows.Err(); err != nil {
		return nil, dbError(err)
	}
	return jobs, nil
}

// countActiveLeases 精确比较租约截止时间，统计本 owner 还有效的租约数。
func (w *work) countActiveLeases(ctx context.Context, op platform.OperationContext) (int, error) {
	rows, err := w.tx.QueryContext(ctx, `SELECT l.expires_at FROM jobs j JOIN leases l ON l.job_id=j.job_id
		WHERE j.owner=? AND j.data_epoch=? AND j.state IN ('leased','running','cancel-requested')`, op.CallerService, op.DataEpoch)
	if err != nil {
		return 0, dbError(err)
	}
	defer rows.Close()
	active := 0
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return 0, dbError(err)
		}
		expires, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return 0, platform.NewError(platform.CodeIntegrityError, false, "任务租约截止损坏")
		}
		if expires.After(w.now) {
			active++
		}
	}
	return active, dbError(rows.Err())
}

// start 只接受当前 owner/worker/fence 的有效租约；同一 fence 重复开始不再写事件。
func (s *Store) start(ctx context.Context, op platform.OperationContext, in leaseInput) error {
	w, err := s.begin(ctx, op)
	if err != nil {
		return err
	}
	defer w.close()
	lease, err := w.readLease(ctx, op, in)
	if err != nil {
		return err
	}
	if lease.cancelRequested {
		return platform.NewError(platform.CodeLocked, false, "任务已请求取消，不能开始执行")
	}
	if lease.state == "running" {
		return nil
	}
	if lease.state != "leased" {
		return leaseLost()
	}
	updated, err := w.tx.ExecContext(ctx, `UPDATE jobs SET state='running',revision=revision+1,updated_at=?
		WHERE job_id=? AND data_epoch=? AND state='leased' AND revision=?`, w.now.Format(time.RFC3339Nano), in.JobID, in.DataEpoch, lease.revision)
	if err != nil {
		return dbError(err)
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return leaseLost()
	}
	if err := w.enqueueState(in.JobID, op.CallerService, in.DataEpoch, "running", lease.revision+1); err != nil {
		return err
	}
	return dbError(w.tx.Commit())
}

// heartbeat 续租 30 秒；同一操作键重复续租保留原截止时间。取消标志按当前状态返回。
func (s *Store) heartbeat(ctx context.Context, op platform.OperationContext, in leaseInput) (heartbeatResult, error) {
	w, err := s.begin(ctx, op)
	if err != nil {
		return heartbeatResult{}, err
	}
	defer w.close()
	lease, err := w.readLease(ctx, op, in)
	if err != nil {
		return heartbeatResult{}, err
	}
	request, _ := json.Marshal(in)
	prior, err := w.receipt(ctx, op, "heartbeat", request)
	if err != nil {
		return heartbeatResult{}, err
	}
	result := heartbeatResult{ExpiresAt: w.now.Add(leaseDuration)}
	if prior != nil {
		if err := json.Unmarshal(prior, &result); err != nil {
			return heartbeatResult{}, platform.NewError(platform.CodeIntegrityError, false, "任务心跳回执损坏")
		}
	} else {
		updated, err := w.tx.ExecContext(ctx, `UPDATE leases SET expires_at=?,last_heartbeat=?
			WHERE job_id=? AND worker_id=? AND fencing_token=? AND expires_at=?`,
			result.ExpiresAt.Format(time.RFC3339Nano), w.now.Format(time.RFC3339Nano), in.JobID, in.WorkerID, in.FencingToken, lease.expiresAt)
		if err != nil {
			return heartbeatResult{}, dbError(err)
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return heartbeatResult{}, leaseLost()
		}
		if err := w.saveReceipt(ctx, op, "heartbeat", request, result); err != nil {
			return heartbeatResult{}, err
		}
	}
	result.CancelRequested = lease.cancelRequested
	return result, dbError(w.tx.Commit())
}

// checkLease 只回答执行权是否仍有效；owner 写回时仍要在自己的事务里核对 fence。
func (s *Store) checkLease(ctx context.Context, op platform.OperationContext, in leaseInput) error {
	w, err := s.begin(ctx, op)
	if err != nil {
		return err
	}
	defer w.close()
	_, err = w.readLease(ctx, op, in)
	return err
}
