// Package internal 提供 jobs 域的任务队列持久化规则。
// 任务只保存不可变对象引用，不读取或复制所属服务的对象正文。
package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite" // 注册无CGO的 sqlite 驱动。

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/jobs/internal/gen"
	jobsmigrations "nice-wiki/modules/jobs/migrations"
)

const (
	databaseName    = "jobs.db"
	maxDedupeRunes  = 8192
	jobStateQueued  = "queued"
	jobEventType    = "jobs.job-state-changed.v1"
	jobEventVersion = 1
)

// CreateJobInput 是内部任务提交合同；HTTP层的 dataEpoch 来自可信操作上下文，
// 不从浏览器请求正文读取。
type CreateJobInput struct {
	Owner     string
	Type      string
	InputRef  ObjectRef
	DedupeKey string
	DataEpoch platform.Identifier
	NotBefore *time.Time
}

// CreateJobResult 复用生成的成功响应类型，确保 AcceptedJob 与 OpenAPI 一致。
type CreateJobResult = gen.CreateJobResult

// Store 是 jobs active 区数据库句柄。单连接保证短写事务串行执行；
// outbox 与任务状态使用同一个 SQLite 事务。
type Store struct {
	db              *sql.DB
	mu              sync.RWMutex
	siteID          string
	closed          bool
	leaseRuntime    LeaseRuntimeProvider
	identity        *gen.IdentityClient
	recovery        OwnerRecoveryChecker
	progressMu      sync.Mutex
	progressStreams map[string]int
	eventMu         sync.Mutex
	eventStreams    map[string]int
}

var activeQueue struct {
	sync.RWMutex
	store *Store
}

// InstallStore 将启动阶段打开的本域存储安装为当前任务入口使用的存储。
// 装配完整 handler 前不会调用 CreateJob，因此不会用空存储伪造服务就绪。
func InstallStore(store *Store, runtime LeaseRuntimeProvider, identity *gen.IdentityClient, recovery OwnerRecoveryChecker, siteID string) {
	store.siteID = siteID
	store.recovery = recovery
	store.identity = identity
	store.leaseRuntime = runtime
	activeQueue.Lock()
	activeQueue.store = store
	activeQueue.Unlock()
}

func currentStore() *Store {
	activeQueue.RLock()
	defer activeQueue.RUnlock()
	return activeQueue.store
}

// OpenDatabase 打开 active/jobs.db，执行本域迁移并建立 platform outbox 表。
// dataRoot 只能是绝对路径；控制区不在本函数内替换或恢复。
func OpenDatabase(dataRoot string) (*Store, error) {
	if dataRoot == "" || !filepath.IsAbs(dataRoot) {
		return nil, platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	activeRoot := filepath.Join(dataRoot, "active")
	if err := os.MkdirAll(activeRoot, 0o700); err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "建立任务数据目录失败")
	}

	dsn := "file:" + filepath.Join(activeRoot, databaseName) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开任务数据库失败")
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "任务数据库不可用")
	}
	if err := store.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close 关闭数据库并清除当前全局安装，重复关闭安全。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	activeQueue.Lock()
	if activeQueue.store == s {
		activeQueue.store = nil
	}
	activeQueue.Unlock()
	if err := s.db.Close(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "关闭任务数据库失败")
	}
	return nil
}

// CreateJob 按 owner、代次和去重键原子提交任务及初始状态事件。
// 同输入哈希的重复提交复用原任务；同去重键但输入不同返回 CONFLICT。
func CreateJob(ctx context.Context, op OperationContext, in CreateJobInput) (CreateJobResult, error) {
	if err := contextError(ctx); err != nil {
		return CreateJobResult{}, err
	}
	if err := validateCreateJobInput(ctx, op, in); err != nil {
		return CreateJobResult{}, err
	}

	store := currentStore()
	if store == nil {
		return CreateJobResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	result, err := store.createJob(ctx, op, in)
	if err != nil {
		return CreateJobResult{}, err
	}
	return result, nil
}

func validateCreateJobInput(ctx context.Context, op OperationContext, in CreateJobInput) error {
	if !platform.ValidIdentifier(op.OperationID) {
		return invalidJob()
	}
	if !utf8.ValidString(in.DedupeKey) || utf8.RuneCountInString(in.DedupeKey) < 1 || utf8.RuneCountInString(in.DedupeKey) > maxDedupeRunes {
		return invalidJob()
	}
	_, err := ValidateJobSpec(ctx, op, ValidateJobSpecInput{
		Owner:     in.Owner,
		Type:      in.Type,
		InputRef:  in.InputRef,
		DataEpoch: in.DataEpoch,
	})
	return err
}

func (s *Store) createJob(ctx context.Context, op OperationContext, in CreateJobInput) (CreateJobResult, error) {
	inputHash, inputRefJSON, err := hashCreateJobInput(in)
	if err != nil {
		return CreateJobResult{}, err
	}
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return CreateJobResult{}, err
	}
	defer work.close()
	if work.runtime.Runtime.Maintenance || work.runtime.SafePointRequested {
		return CreateJobResult{}, platform.NewError(platform.CodeLocked, false, "维护或屏障期间停止创建任务")
	}
	tx := work.tx
	rollback := func(cause error) (CreateJobResult, error) {
		_ = tx.Rollback()
		return CreateJobResult{}, cause
	}

	existing, found, err := findByDedupe(ctx, tx, in.Owner, in.DataEpoch, in.DedupeKey)
	if err != nil {
		return rollback(mapQueueDBError(err))
	}
	if found {
		if existing.InputHash != inputHash {
			return rollback(platform.NewError(platform.CodeConflict, false, "任务去重键已绑定不同输入"))
		}
		if err := tx.Commit(); err != nil {
			return CreateJobResult{}, mapQueueDBError(err)
		}
		return acceptedJob(existing.JobID, existing.OperationID), nil
	}

	jobID := platform.NewIdentifier()
	jobRevision := int64(1)
	operationID := op.OperationID
	now := platform.Now()
	notBefore := now
	if in.NotBefore != nil {
		notBefore = in.NotBefore.UTC().Format(time.RFC3339Nano)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO jobs (
			job_id, operation_id, root_operation_id, owner, type, immutable_input_ref, input_hash,
			dedupe_key, state, attempt, not_before, data_epoch, revision,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		jobID, operationID, op.RootOperationID, in.Owner, in.Type, inputRefJSON, inputHash,
		in.DedupeKey, jobStateQueued, notBefore, in.DataEpoch, jobRevision, now, now); err != nil {
		if isUniqueConstraint(err) {
			existing, found, lookupErr := findByDedupe(ctx, tx, in.Owner, in.DataEpoch, in.DedupeKey)
			if lookupErr != nil {
				return rollback(mapQueueDBError(lookupErr))
			}
			if found && existing.InputHash == inputHash {
				if commitErr := tx.Commit(); commitErr != nil {
					return CreateJobResult{}, mapQueueDBError(commitErr)
				}
				return acceptedJob(existing.JobID, existing.OperationID), nil
			}
			return rollback(platform.NewError(platform.CodeConflict, false, "任务去重键已绑定不同输入"))
		}
		return rollback(mapQueueDBError(err))
	}

	payload, err := json.Marshal(jobStateChangedPayload{
		JobID:       jobID,
		State:       jobStateQueued,
		JobRevision: jobRevision,
		Owner:       in.Owner,
	})
	if err != nil {
		return rollback(platform.NewError(platform.CodeInternal, false, "任务状态事件编码失败"))
	}
	event := platform.EventEnvelope{
		EventID:      platform.NewIdentifier(),
		EventType:    jobEventType,
		EventVersion: jobEventVersion,
		Producer:     "jobs",
		SubjectID:    jobID,
		Revision:     jobRevision,
		OccurredAt:   now,
		DataEpoch:    in.DataEpoch,
		Payload:      payload,
	}
	if err := outbox.EnqueueEvent(tx, event); err != nil {
		return rollback(err)
	}
	if err := tx.Commit(); err != nil {
		return CreateJobResult{}, mapQueueDBError(err)
	}
	return acceptedJob(jobID, operationID), nil
}

type storedJob struct {
	JobID       platform.Identifier
	OperationID platform.Identifier
	InputHash   platform.Hash
}

func findByDedupe(ctx context.Context, tx *sql.Tx, owner, epoch, dedupeKey string) (storedJob, bool, error) {
	var job storedJob
	err := tx.QueryRowContext(ctx, `
		SELECT job_id, operation_id, input_hash
		FROM jobs
		WHERE owner = ? AND data_epoch = ? AND dedupe_key = ?`,
		owner, epoch, dedupeKey).Scan(&job.JobID, &job.OperationID, &job.InputHash)
	if errors.Is(err, sql.ErrNoRows) {
		return storedJob{}, false, nil
	}
	if err != nil {
		return storedJob{}, false, err
	}
	return job, true, nil
}

func hashCreateJobInput(in CreateJobInput) (platform.Hash, []byte, error) {
	inputRefJSON, err := json.Marshal(in.InputRef)
	if err != nil {
		return "", nil, platform.NewError(platform.CodeValidation, false, "任务引用不可编码")
	}
	var notBefore *string
	if in.NotBefore != nil {
		value := in.NotBefore.UTC().Format(time.RFC3339Nano)
		notBefore = &value
	}
	canonical, err := json.Marshal(struct {
		Owner     string    `json:"owner"`
		Type      string    `json:"type"`
		InputRef  ObjectRef `json:"inputRef"`
		DedupeKey string    `json:"dedupeKey"`
		DataEpoch string    `json:"dataEpoch"`
		NotBefore *string   `json:"notBefore,omitempty"`
	}{in.Owner, in.Type, in.InputRef, in.DedupeKey, in.DataEpoch, notBefore})
	if err != nil {
		return "", nil, platform.NewError(platform.CodeValidation, false, "任务输入不可编码")
	}
	return platform.HashBytes(canonical), inputRefJSON, nil
}

// NewQueueHandlers只做已通过统一边界的请求绑定，不以本组映射冒充完整服务。
func NewQueueHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"readJob": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := ReadJob(r.Context(), op, ReadJobInput{JobID: r.PathValue("jobId"), Actor: op.Actor, DataEpoch: op.DataEpoch})
			writeEventResponse(w, op.RequestID, result, err, http.StatusOK)
		}),
		"createJob": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input gen.CreateJobRequestBody
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeEventResponse(w, op.RequestID, nil, platform.NewError(platform.CodeValidation, false, "任务提交请求绑定失败"), 0)
				return
			}
			result, err := CreateJob(r.Context(), op, CreateJobInput{
				Owner: input.Owner, Type: input.Type, InputRef: input.InputRef,
				DedupeKey: input.DedupeKey, DataEpoch: op.DataEpoch, NotBefore: input.NotBefore,
			})
			writeEventResponse(w, op.RequestID, result, err, http.StatusAccepted)
		}),
	}
}

type jobStateChangedPayload struct {
	JobID       platform.Identifier `json:"jobId"`
	State       string              `json:"state"`
	JobRevision platform.Revision   `json:"jobRevision"`
	Owner       string              `json:"owner"`
}

func acceptedJob(jobID, operationID platform.Identifier) CreateJobResult {
	return CreateJobResult{Job: gen.AcceptedJob{
		JobId:       jobID,
		OperationId: operationID,
		State:       jobStateQueued,
	}}
}

// runMigrations 按编号升序执行 migrations/ 内嵌的只追加迁移。首个迁移自建
// schema_migrations；数据库出现迁移集之外或名称不符的版本时拒绝启动。
func (s *Store) runMigrations() error {
	all, err := jobsmigrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "任务迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := s.db.Query(`SELECT version, name FROM schema_migrations`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return platform.NewError(platform.CodeIntegrityError, false, "读取任务迁移记录失败")
		}
	} else {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				_ = rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "任务迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "读取任务迁移记录失败")
		}
		_ = rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeIntegrityError, false, "任务迁移版本未知")
		}
	}
	if err := outbox.EnsureSchema(context.Background(), s.db); err != nil {
		return err
	}
	for _, migration := range all {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return platform.NewError(platform.CodeInternal, true, "开始任务迁移事务失败")
		}
		if _, err := tx.Exec(migration.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "执行任务迁移失败")
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			migration.Version, migration.Name, platform.Now()); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeInternal, false, "登记任务迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeInternal, false, "提交任务迁移失败")
		}
	}
	return nil
}

func mapQueueDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, true, "任务提交已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "任务提交已超过截止时间")
	}
	if isBusyError(err) {
		return platform.NewError(platform.CodeLocked, true, "任务数据库暂时被占用")
	}
	return platform.NewError(platform.CodeInternal, true, "任务持久化失败")
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE CONSTRAINT")
}

func isBusyError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "busy")
}

// FailJob 在owner已经落到持久安全点后确认失败；不把超时等同没有费用。
type FailJobInput struct {
	JobID          platform.Identifier `json:"jobId"`
	WorkerID       string              `json:"workerId"`
	FencingToken   int64               `json:"fencingToken"`
	ErrorCode      string              `json:"errorCode"`
	PaidResultRefs []string            `json:"paidResultRefs,omitempty"`
	DataEpoch      platform.Identifier `json:"dataEpoch"`
}

type FailJobResult struct {
	State   string     `json:"state"`
	RetryAt *time.Time `json:"retryAt,omitempty"`
}

func FailJob(ctx context.Context, op OperationContext, in FailJobInput) (FailJobResult, error) {
	work, err := beginLeaseOperation(ctx, op, in.DataEpoch)
	if err != nil {
		return FailJobResult{}, err
	}
	defer work.close()
	job, err := work.readJobRecord(ctx, in.JobID, in.DataEpoch)
	if err != nil {
		return FailJobResult{}, err
	}
	leaseInput := StartJobInput{JobID: in.JobID, WorkerID: in.WorkerID, FencingToken: in.FencingToken, DataEpoch: in.DataEpoch}
	request, _ := json.Marshal(in)
	var result FailJobResult
	found, err := work.terminalReplay(ctx, op, job, leaseInput, "fail", request, &result)
	if err != nil {
		return FailJobResult{}, err
	}
	if found {
		return result, nil
	}
	lease, err := work.readLease(ctx, op, leaseInput)
	if err != nil {
		return FailJobResult{}, err
	}
	if lease.state != "running" && lease.state != "cancel-requested" {
		return FailJobResult{}, leaseLost()
	}
	failure, err := ClassifyFailure(ctx, op, ClassifyFailureInput{ErrorCode: in.ErrorCode, Attempt: job.attempt, CancelRequested: job.cancelRequested})
	if err != nil {
		return FailJobResult{}, err
	}
	result.State = failure.TerminalState
	if failure.TerminalState == "cancelled" {
		_, err := ConfirmCancelled(ctx, op, work, ConfirmCancelledInput{JobID: in.JobID, WorkerID: in.WorkerID, FencingToken: in.FencingToken, ResultRefs: in.PaidResultRefs, DataEpoch: in.DataEpoch})
		if err != nil {
			return FailJobResult{}, err
		}
	} else {
		if failure.Retryable {
			retry, err := ComputeRetryAt(ctx, op, ComputeRetryAtInput{JobID: in.JobID, Attempt: job.attempt, Now: work.now})
			if err != nil {
				return FailJobResult{}, err
			}
			result.RetryAt = &retry.RetryAt
		}
		if err := work.mergePaidResults(ctx, in.JobID, in.PaidResultRefs, nil); err != nil {
			return FailJobResult{}, err
		}
		if err := work.finishJob(ctx, job, result.State, result.RetryAt, in.ErrorCode); err != nil {
			return FailJobResult{}, err
		}
		if result.State == "orphan" {
			if _, err := work.tx.ExecContext(ctx, `INSERT INTO job_orphans(job_id,expired_fence,worker_id,next_check_at) VALUES(?,?,?,?)`,
				job.id, in.FencingToken, in.WorkerID, work.now.Format(time.RFC3339Nano)); err != nil {
				return FailJobResult{}, mapQueueDBError(err)
			}
		}
	}
	if err := work.saveTerminal(ctx, op, leaseInput, "fail", request, result); err != nil {
		return FailJobResult{}, err
	}
	if err := work.tx.Commit(); err != nil {
		return FailJobResult{}, mapQueueDBError(err)
	}
	return result, nil
}
