// Package internal 提供 jobs 域的任务队列持久化规则。
// 任务只保存不可变对象引用，不读取或复制所属服务的对象正文。
package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	_ "modernc.org/sqlite" // 注册无CGO的 sqlite 驱动。

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/jobs/internal/gen"
)

const (
	databaseName   = "jobs.db"
	maxDedupeRunes = 8192
	jobStateQueued = "queued"
	jobEventType   = "jobs.job-state-changed.v1"
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
}

// CreateJobResult 复用生成的成功响应类型，确保 AcceptedJob 与 OpenAPI 一致。
type CreateJobResult = gen.CreateJobResult

// Store 是 jobs active 区数据库句柄。单连接保证短写事务串行执行；
// outbox 与任务状态使用同一个 SQLite 事务。
type Store struct {
	db     *sql.DB
	mu     sync.RWMutex
	closed bool
}

var activeQueue struct {
	sync.RWMutex
	store *Store
}

// InstallStore 将启动阶段打开的本域存储安装为当前任务入口使用的存储。
// 装配完整 handler 前不会调用 CreateJob，因此不会用空存储伪造服务就绪。
func InstallStore(store *Store) {
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
	if err := outbox.EnsureSchema(context.Background(), db); err != nil {
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
	if !utf8.ValidString(in.DedupeKey) || utf8.RuneCountInString(in.DedupeKey) > maxDedupeRunes {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return CreateJobResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储已关闭")
	}
	if err := contextError(ctx); err != nil {
		return CreateJobResult{}, err
	}

	inputHash, inputRefJSON, err := hashCreateJobInput(in)
	if err != nil {
		return CreateJobResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CreateJobResult{}, mapQueueDBError(err)
	}
	rollback := func(cause error) (CreateJobResult, error) {
		_ = tx.Rollback()
		return CreateJobResult{}, cause
	}

	var draining int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM barriers WHERE state = 'draining' AND data_epoch = ?`,
		in.DataEpoch).Scan(&draining); err != nil {
		return rollback(mapQueueDBError(err))
	}
	if draining > 0 {
		return rollback(platform.NewError(platform.CodeLocked, false, "任务屏障正在排空，暂不接受新任务"))
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
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO jobs (
			job_id, operation_id, owner, type, immutable_input_ref, input_hash,
			dedupe_key, state, attempt, not_before, data_epoch, revision,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		jobID, operationID, in.Owner, in.Type, inputRefJSON, inputHash,
		in.DedupeKey, jobStateQueued, now, in.DataEpoch, jobRevision, now, now); err != nil {
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
	canonical, err := json.Marshal(struct {
		Owner     string     `json:"owner"`
		Type      string     `json:"type"`
		InputRef  ObjectRef  `json:"inputRef"`
		DedupeKey string     `json:"dedupeKey"`
		DataEpoch string     `json:"dataEpoch"`
	}{in.Owner, in.Type, in.InputRef, in.DedupeKey, in.DataEpoch})
	if err != nil {
		return "", nil, platform.NewError(platform.CodeValidation, false, "任务输入不可编码")
	}
	return platform.HashBytes(canonical), inputRefJSON, nil
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

func (s *Store) runMigrations() error {
	tx, err := s.db.Begin()
	if err != nil {
		return platform.NewError(platform.CodeInternal, true, "开始任务迁移事务失败")
	}
	rollback := func(cause error) error {
		_ = tx.Rollback()
		return cause
	}
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return rollback(platform.NewError(platform.CodeIntegrityError, false, "建立任务迁移记录失败"))
	}
	var version int64
	var name string
	err = tx.QueryRow(`SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &name)
	if errors.Is(err, sql.ErrNoRows) {
		version = 0
	} else if err != nil {
		return rollback(platform.NewError(platform.CodeIntegrityError, false, "读取任务迁移记录失败"))
	}
	if version > 1 || (version == 1 && name != "0001_domain.sql") {
		return rollback(platform.NewError(platform.CodeIntegrityError, false, "任务迁移版本未知"))
	}
	if version == 0 {
		if _, err := tx.Exec(domainSchemaSQL); err != nil {
			return rollback(platform.NewError(platform.CodeIntegrityError, false, "执行任务初始迁移失败"))
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (1, '0001_domain.sql', ?)`, platform.Now()); err != nil {
			return rollback(platform.NewError(platform.CodeInternal, false, "登记任务迁移失败"))
		}
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "提交任务迁移失败")
	}
	return nil
}

// domainSchemaSQL 与 modules/jobs/migrations/0001_domain.sql 保持一致。
// 运行时将迁移文本内联，避免从安装后的工作目录读取源码文件。
const domainSchemaSQL = `
CREATE TABLE IF NOT EXISTS jobs (
    job_id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL,
    owner TEXT NOT NULL,
    type TEXT NOT NULL,
    immutable_input_ref BLOB NOT NULL,
    input_hash TEXT NOT NULL,
    dedupe_key TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'leased', 'running', 'retry-wait', 'dead-letter', 'cancel-requested', 'cancelled', 'succeeded')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    not_before TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_owner_epoch_dedupe ON jobs (owner, data_epoch, dedupe_key);
CREATE INDEX IF NOT EXISTS idx_jobs_claimable ON jobs (state, not_before, created_at, job_id);

CREATE TABLE IF NOT EXISTS leases (
    job_id TEXT PRIMARY KEY REFERENCES jobs(job_id),
    worker_id TEXT NOT NULL,
    fencing_token INTEGER NOT NULL CHECK (fencing_token > 0),
    expires_at TEXT NOT NULL,
    last_heartbeat TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS progress (
    job_id TEXT NOT NULL REFERENCES jobs(job_id),
    seq INTEGER NOT NULL CHECK (seq >= 0),
    phase TEXT NOT NULL,
    completed_units INTEGER NOT NULL CHECK (completed_units >= 0),
    total_units INTEGER NOT NULL CHECK (total_units >= 0),
    safe_message_code TEXT NOT NULL,
    PRIMARY KEY (job_id, seq)
);

CREATE TABLE IF NOT EXISTS job_results (
    job_id TEXT PRIMARY KEY REFERENCES jobs(job_id),
    result_ref TEXT NOT NULL,
    paid_result_refs_json BLOB NOT NULL,
    result_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
    event_id TEXT PRIMARY KEY,
    event_seq INTEGER NOT NULL UNIQUE,
    envelope_json BLOB NOT NULL,
    source_revision INTEGER NOT NULL CHECK (source_revision >= 0),
    data_epoch TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS deliveries (
    event_id TEXT NOT NULL REFERENCES events(event_id),
    consumer_id TEXT NOT NULL,
    state TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    retry_at TEXT NOT NULL,
    PRIMARY KEY (event_id, consumer_id)
);

CREATE TABLE IF NOT EXISTS schedules (
    schedule_id TEXT PRIMARY KEY,
    owner TEXT NOT NULL,
    cron TEXT NOT NULL,
    timezone TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 0),
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    next_fire_at TEXT NOT NULL,
    last_tick TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS barriers (
    barrier_id TEXT PRIMARY KEY,
    state TEXT NOT NULL,
    domain_status_json BLOB NOT NULL,
    data_epoch TEXT NOT NULL,
    deadline TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL
);
`

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
