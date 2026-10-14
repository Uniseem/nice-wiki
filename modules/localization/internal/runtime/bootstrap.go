// Package runtime 实现 localization 域启动时的私有存储与恢复边界。
// 这里只负责schema/journal的完整性，不承载翻译、术语或模型业务规则。
package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/outbox"
)

const (
	moduleSlug    = "localization"
	databaseName  = "localization.db"
	journalName   = "localization-journal.jsonl"
	migrationName = "001_initial.sql"
)

// OperationContext 是平台可信执行上下文的本域别名。
type OperationContext = platform.OperationContext

// LoadRuntimeInput 是启动加载的封闭输入；store 仅供BuildApplication复用，
// 不属于HTTP或JSON业务字段。
type LoadRuntimeInput struct {
	ServiceID     string
	DataDirectory string
	store         *Store
}

// NewLoadRuntimeInput 构造启动输入；传入store时由调用方负责最终关闭。
func NewLoadRuntimeInput(serviceID, dataDirectory string, store *Store) LoadRuntimeInput {
	return LoadRuntimeInput{ServiceID: serviceID, DataDirectory: dataDirectory, store: store}
}

// LoadRuntimeResult 是启动加载结果；字段与本域types.json保持一致。
type LoadRuntimeResult struct {
	Ready              bool
	GlossaryGeneration int64
}

// Store 是active区业务库与control区决定日志的句柄。
// control区不随业务恢复换目录替换。
type Store struct {
	root              string
	db                *sql.DB
	journal           *os.File
	mu                sync.Mutex
	closeOnce         sync.Once
	closeErr          error
	journalRecovered   bool
	appliedMigrations  int64
	glossaryGeneration int64
	leaseRecoveryPending int64
}

type journalEntry struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	State   string          `json:"state"`
	Payload json.RawMessage `json:"payload,omitempty"`
	At      platform.Timestamp `json:"at"`
}

// initialMigrationSQL 与 migrations/001_initial.sql保持一致；独立二进制不
// 读取源码目录，避免生产镜像依赖工作区路径。
const initialMigrationSQL = `CREATE TABLE languages (
    language_tag                         TEXT PRIMARY KEY,
    version                              INTEGER NOT NULL CHECK (version >= 0),
    enabled                              INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    default_language                     INTEGER NOT NULL CHECK (default_language IN (0, 1)),
    translation_profile                  TEXT,
    automatic_translation                INTEGER NOT NULL CHECK (automatic_translation IN (0, 1)),
    direction                            TEXT NOT NULL CHECK (direction IN ('ltr', 'rtl')),
    reflection_enabled                   INTEGER NOT NULL CHECK (reflection_enabled IN (0, 1)),
    batch_output_tokens                  INTEGER NOT NULL CHECK (batch_output_tokens BETWEEN 100 AND 8192),
    translation_memory_minimum_characters INTEGER NOT NULL CHECK (translation_memory_minimum_characters BETWEEN 1 AND 1000),
    data_epoch                           TEXT NOT NULL,
    created_at                           TEXT NOT NULL,
    updated_at                           TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_languages_single_default
    ON languages (default_language) WHERE default_language = 1;
CREATE TABLE translation_jobs (
    job_id TEXT PRIMARY KEY,
    document_ref TEXT NOT NULL,
    target_language TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'running', 'review', 'ready', 'superseded', 'failed', 'cancelled', 'stale')),
    source_generation INTEGER NOT NULL CHECK (source_generation >= 0),
    glossary_generation INTEGER NOT NULL CHECK (glossary_generation >= 0),
    human_generation INTEGER NOT NULL CHECK (human_generation >= 0),
    memory_version_hash TEXT NOT NULL,
    lease_id TEXT,
    lease_expires_at TEXT,
    data_epoch TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_translation_jobs_state ON translation_jobs (state, target_language);
CREATE TABLE translation_units (
    unit_id TEXT PRIMARY KEY,
    translation_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    source_hash TEXT NOT NULL,
    source_text TEXT NOT NULL,
    target_language TEXT NOT NULL,
    target_file_ref TEXT,
    target_text TEXT NOT NULL,
    target_hash TEXT NOT NULL,
    origin TEXT NOT NULL CHECK (origin IN ('machine', 'human')),
    protected INTEGER NOT NULL CHECK (protected IN (0, 1)),
    version INTEGER NOT NULL CHECK (version >= 0),
    human_generation INTEGER NOT NULL CHECK (human_generation >= 0),
    status TEXT NOT NULL CHECK (status IN ('normal', 'review', 'fallback')),
    previous_target_text TEXT,
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_translation_units_translation ON translation_units (translation_id, unit_id);
CREATE TABLE translation_manifests (
    translation_id TEXT PRIMARY KEY,
    publication_ref TEXT NOT NULL,
    target_language TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK (generation >= 0),
    memory_hash TEXT NOT NULL,
    glossary_generation INTEGER NOT NULL CHECK (glossary_generation >= 0),
    state TEXT NOT NULL CHECK (state IN ('queued', 'running', 'review', 'ready', 'superseded', 'failed', 'cancelled', 'stale')),
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE glossary (
    term_id TEXT PRIMARY KEY,
    source_language TEXT NOT NULL,
    target_language TEXT NOT NULL,
    source_term TEXT NOT NULL,
    target_term TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('global', 'document')),
    document_id TEXT,
    version INTEGER NOT NULL CHECK (version >= 0),
    generation INTEGER NOT NULL CHECK (generation >= 0),
    source TEXT NOT NULL CHECK (source IN ('manual', 'auto')),
    extraction_batch_id TEXT,
    data_epoch TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_glossary_lookup ON glossary (source_language, target_language, source_term, generation);
CREATE TABLE interface_catalog (
    language TEXT NOT NULL,
    key TEXT NOT NULL,
    source_hash TEXT NOT NULL,
    value_file_ref TEXT,
    value_text TEXT NOT NULL,
    placeholders TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK (generation >= 0),
    origin TEXT NOT NULL CHECK (origin IN ('builtin', 'machine', 'human')),
    protected INTEGER NOT NULL CHECK (protected IN (0, 1)),
    version INTEGER NOT NULL CHECK (version >= 0),
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (language, key)
);
CREATE TABLE language_preferences (
    visitor_subject TEXT PRIMARY KEY,
    language TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE TABLE approved_proposals (
    proposal_id TEXT PRIMARY KEY,
    proposal_hash TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('accepted', 'applied', 'rejected')),
    term_id TEXT,
    approved_by TEXT NOT NULL,
    approval_revision INTEGER NOT NULL CHECK (approval_revision >= 0),
    expected_glossary_generation INTEGER NOT NULL CHECK (expected_glossary_generation >= 0),
    term_json TEXT NOT NULL,
    data_epoch TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE cache_answer_translations (
    translation_id TEXT PRIMARY KEY,
    cache_id TEXT NOT NULL,
    answer_revision INTEGER NOT NULL CHECK (answer_revision >= 0),
    source_hash TEXT NOT NULL,
    target_language TEXT NOT NULL,
    translated_question TEXT NOT NULL,
    translated_answer TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'running', 'ready', 'failed', 'cancelled', 'superseded')),
    data_epoch TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_cache_answer_translations_source ON cache_answer_translations (cache_id, answer_revision, target_language);
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL
);`

// OpenStore 打开active区SQLite、control区journal并初始化平台inbox/outbox
// 表；任一资源失败都关闭已打开句柄。
func OpenStore(dataRoot string) (*Store, error) {
	if dataRoot == "" || !filepath.IsAbs(dataRoot) {
		return nil, platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	root := filepath.Clean(dataRoot)
	for _, name := range []string{"active", "control"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			return nil, platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}

	dsn := "file:" + filepath.Join(root, "active", databaseName) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开本域业务库失败")
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域业务库不可用")
	}
	if err := inbox.EnsureSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := outbox.EnsureSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}

	journal, err := os.OpenFile(filepath.Join(root, "control", journalName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeInternal, false, "打开本域决定日志失败")
	}
	return &Store{root: root, db: db, journal: journal}, nil
}

// DB 返回本域业务库句柄，供同模块后续事务函数使用；调用方不得跨域传递。
func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// LoadRuntime 执行启动时schema、默认语言、术语代数和journal完整性检查。
// 未传入store时函数自持有并关闭临时句柄；BuildApplication传入store时，
// 成功后由应用生命周期负责关闭。
func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error) {
	if ctx == nil {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "启动上下文不可为空")
	}
	if input.ServiceID != moduleSlug {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "启动服务身份不属于本域")
	}
	if input.DataDirectory == "" || !filepath.IsAbs(input.DataDirectory) {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "私有数据目录必须为绝对路径")
	}
	if op.CallerService != "" && op.CallerService != input.ServiceID {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeForbidden, false, "启动操作服务身份不匹配")
	}
	workCtx, cancel, err := runtimeContext(ctx, op)
	if err != nil {
		return LoadRuntimeResult{}, err
	}
	defer cancel()
	if err := checkContext(workCtx); err != nil {
		return LoadRuntimeResult{}, err
	}

	store := input.store
	owned := false
	if store == nil {
		store, err = OpenStore(input.DataDirectory)
		if err != nil {
			return LoadRuntimeResult{}, err
		}
		owned = true
	}
	if filepath.Clean(input.DataDirectory) != store.root {
		if owned {
			_ = store.Close()
		}
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "启动数据目录与已打开存储不一致")
	}
	if owned {
		defer func() { _ = store.Close() }()
	}

	if err := store.load(workCtx); err != nil {
		return LoadRuntimeResult{}, err
	}
	return LoadRuntimeResult{Ready: true, GlossaryGeneration: store.glossaryGeneration}, nil
}

func (s *Store) load(ctx context.Context) error {
	if s == nil || s.db == nil || s.journal == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本域启动存储未装配")
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := s.runMigrations(ctx); err != nil {
		return err
	}
	if err := validateSchema(ctx, s.db); err != nil {
		return err
	}
	if err := s.recoverJournal(); err != nil {
		return err
	}
	var defaults int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM languages WHERE default_language = 1`).Scan(&defaults); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "默认语言记录不可读")
	}
	if defaults > 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "默认语言不唯一")
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(generation), 0) FROM glossary`).Scan(&s.glossaryGeneration); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "术语代数不可读")
	}
	if s.glossaryGeneration < 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "术语代数不合法")
	}
	if err := s.registerLeaseRecovery(ctx); err != nil {
		return err
	}
	return checkContext(ctx)
}

// registerLeaseRecovery 只登记启动时发现的旧租约数量；具体重排由后续
// RecoverTranslationLeases 接管，不在启动阶段重放任何模型或任务副作用。
func (s *Store) registerLeaseRecovery(ctx context.Context) error {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM translation_jobs WHERE state = 'running' AND lease_id IS NOT NULL`).Scan(&s.leaseRecoveryPending); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "旧翻译租约不可读取")
	}
	return nil
}

func (s *Store) runMigrations(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return platform.NewError(platform.CodeIntegrityError, false, "读取本域迁移记录失败")
		}
		rows = nil
	}
	applied := map[int64]string{}
	if rows != nil {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "迁移记录损坏")
			}
			if version != 1 || name != migrationName || applied[version] != "" {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "已应用迁移不属于当前迁移集")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "迁移记录读取失败")
		}
		rows.Close()
	}
	if applied[1] == migrationName {
		s.appliedMigrations = 1
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "开始迁移事务失败")
	}
	if _, err := tx.ExecContext(ctx, initialMigrationSQL); err != nil {
		_ = tx.Rollback()
		return platform.NewError(platform.CodeIntegrityError, false, "执行本域迁移失败")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, 1, migrationName, platform.Now()); err != nil {
		_ = tx.Rollback()
		return platform.NewError(platform.CodeInternal, false, "登记本域迁移失败")
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "提交本域迁移失败")
	}
	s.appliedMigrations = 1
	return nil
}

func validateSchema(ctx context.Context, db *sql.DB) error {
	required := map[string][]string{
		"languages": {"language_tag", "default_language", "automatic_translation"},
		"translation_jobs": {"job_id", "state", "lease_id"},
		"translation_units": {"unit_id", "protected", "human_generation"},
		"translation_manifests": {"translation_id", "glossary_generation", "state"},
		"glossary": {"term_id", "generation", "source"},
		"interface_catalog": {"language", "key", "placeholders"},
		"language_preferences": {"visitor_subject", "expires_at"},
		"approved_proposals": {"proposal_id", "proposal_hash", "state"},
		"cache_answer_translations": {"translation_id", "answer_revision", "state"},
		"schema_migrations": {"version", "name", "applied_at"},
	}
	for table, columns := range required {
		if err := checkContext(ctx); err != nil {
			return err
		}
		rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
		if err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "读取本域表结构失败")
		}
		found := map[string]bool{}
		for rows.Next() {
			var cid int
			var name, columnType string
			var notNull, primaryKey int
			var defaultValue any
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "本域表结构记录损坏")
			}
			found[name] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "读取本域表结构失败")
		}
		rows.Close()
		for _, column := range columns {
			if !found[column] {
				return platform.NewError(platform.CodeIntegrityError, false, "本域表结构缺少登记字段")
			}
		}
	}
	return nil
}

func (s *Store) recoverJournal() error {
	if s == nil || s.journal == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本域决定日志未装配")
	}
	entries, err := readJournal(s.journal.Name())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.State == "open" {
			return platform.NewError(platform.CodeIntegrityError, false, "本域决定日志存在未完成条目")
		}
	}
	s.journalRecovered = true
	return nil
}

// JournalRecovered 供Ready与后续恢复协调读取，不暴露journal正文。
func (s *Store) JournalRecovered() bool {
	return s != nil && s.journalRecovered
}

// Close flush决定日志并关闭全部句柄；重复关闭返回同一结果。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.journal != nil {
			if err := s.journal.Sync(); err != nil {
				s.closeErr = platform.NewError(platform.CodeInternal, false, "flush本域决定日志失败")
			} else if err := s.journal.Close(); err != nil {
				s.closeErr = platform.NewError(platform.CodeInternal, false, "关闭本域决定日志失败")
			}
			s.journal = nil
		}
		if s.db != nil {
			if err := s.db.Close(); err != nil && s.closeErr == nil {
				s.closeErr = platform.NewError(platform.CodeInternal, false, "关闭本域业务库失败")
			}
			s.db = nil
		}
	})
	return s.closeErr
}

func readJournal(name string) ([]journalEntry, error) {
	raw, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "读取本域决定日志失败")
	}
	states := map[string]journalEntry{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || !platform.ValidIdentifier(entry.ID) || (entry.State == "open" && entry.Kind == "") {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "本域决定日志损坏")
		}
		if entry.State != "open" && entry.State != "done" && entry.State != "aborted" {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "本域决定日志状态不合法")
		}
		if _, err := platform.ParseTimestamp(entry.At); err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "本域决定日志时间不合法")
		}
		states[entry.ID] = entry
	}
	entries := make([]journalEntry, 0, len(states))
	for _, entry := range states {
		entries = append(entries, entry)
	}
	return entries, nil
}

func runtimeContext(ctx context.Context, op OperationContext) (context.Context, context.CancelFunc, error) {
	deadline := time.Now().UTC().Add(30 * time.Second)
	if op.Deadline != "" {
		operationDeadline, err := platform.ParseTimestamp(op.Deadline)
		if err != nil {
			return nil, nil, platform.NewError(platform.CodeValidation, false, "启动操作截止时间不合法")
		}
		if operationDeadline.Before(deadline) {
			deadline = operationDeadline
		}
	}
	if parentDeadline, ok := ctx.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	workCtx, cancel := context.WithDeadline(ctx, deadline)
	return workCtx, cancel, nil
}

func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, false, "启动操作已超过截止时间")
		}
		return platform.NewError(platform.CodeCancelled, false, "启动操作已取消")
	default:
		return nil
	}
}
