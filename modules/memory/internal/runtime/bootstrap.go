// Package runtime 负责 memory 域私有存储的启动恢复；不访问其他域私有数据。
package runtime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"

	"nice-wiki/libs/platform"
)

const (
	serviceID         = "memory"
	databaseName      = "memory.db"
	controlDatabase   = "control.db"
	journalName       = "memory-journal.jsonl"
	initialSchemaName = "001_initial.sql"
)

// OperationContext 使用平台的可信操作上下文，避免在本域复制公共协议。
type OperationContext = platform.OperationContext

// LoadRuntimeInput 是启动恢复所需的封闭输入。
type LoadRuntimeInput struct {
	ServiceID    string
	DataDirectory string
}

// LoadRuntimeResult 是启动恢复结果；不暴露路径、正文或控制日志内容。
type LoadRuntimeResult struct {
	Ready           bool
	MemoryGeneration int64
}

// Store 是本域 active 数据库、control 数据库和正文对象根目录的句柄。
// control 区不随数据快照替换，journal 用追加并 fsync 的方式记录安全点。
type Store struct {
	db          *sql.DB
	controlDB   *sql.DB
	objectsRoot string
	dataEpoch   platform.Identifier
	generation  int64

	journal   *os.File
	journalMu sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

var installed struct {
	sync.RWMutex
	store *Store
}

// CurrentStore 返回已经完成启动校验的本域存储；调用方不得持有它跨越关闭。
func CurrentStore() *Store {
	installed.RLock()
	defer installed.RUnlock()
	return installed.store
}

// LoadRuntime 打开并恢复本域私有存储。未完成的业务 handler 不在此处注册；
// 只有所有索引、正文引用、映射、预算和批准命令校验完成后才报告 ready。
func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error) {
	if ctx == nil {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "启动上下文不能为空")
	}
	deadlineCtx, cancel, err := functionContext(ctx, op)
	if err != nil {
		return LoadRuntimeResult{}, err
	}
	defer cancel()

	if err := validateOperation(op); err != nil {
		return LoadRuntimeResult{}, err
	}
	if input.ServiceID != serviceID || !validText(input.DataDirectory) || !filepath.IsAbs(input.DataDirectory) {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "启动输入不合法")
	}

	store, err := OpenStore(deadlineCtx, input.DataDirectory, op.DataEpoch)
	if err != nil {
		return LoadRuntimeResult{}, err
	}
	journalID, err := store.openJournal("runtime.load", op.DataEpoch)
	if err != nil {
		_ = store.Close()
		return LoadRuntimeResult{}, err
	}

	checks := []func(context.Context, *Store, platform.Identifier) error{
		validateEntries,
		validateMappings,
		validateBudgets,
		validateApprovedCommands,
	}
	for _, check := range checks {
		if err := contextError(deadlineCtx); err != nil {
			_ = store.Close()
			return LoadRuntimeResult{}, err
		}
		if err := check(deadlineCtx, store, op.Actor.SiteID); err != nil {
			_ = store.Close()
			return LoadRuntimeResult{}, err
		}
	}

	generation, err := store.loadRuntimeState(deadlineCtx, op.DataEpoch, op.Actor.SiteID)
	if err != nil {
		_ = store.Close()
		return LoadRuntimeResult{}, err
	}
	if err := store.completeJournal(journalID); err != nil {
		_ = store.Close()
		return LoadRuntimeResult{}, err
	}
	store.generation = generation
	install(store)
	return LoadRuntimeResult{Ready: true, MemoryGeneration: generation}, nil
}

// OpenStore 打开 active/control 三区并执行本域迁移；仅供本域后续业务函数复用。
func OpenStore(ctx context.Context, dataDirectory string, dataEpoch platform.Identifier) (*Store, error) {
	if ctx == nil || !filepath.IsAbs(dataDirectory) || !platform.ValidIdentifier(dataEpoch) {
		return nil, platform.NewError(platform.CodeValidation, false, "私有存储参数不合法")
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	for _, directory := range []string{"active", "control", "restore", "objects"} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, directory), 0o700); err != nil {
			return nil, platform.NewError(platform.CodeInternal, false, "建立私有数据目录失败")
		}
	}

	store := &Store{objectsRoot: filepath.Join(dataDirectory, "objects"), dataEpoch: dataEpoch}
	closeOnError := func(err error) (*Store, error) {
		_ = store.Close()
		return nil, err
	}
	var err error
	store.db, err = openSQLite(filepath.Join(dataDirectory, "active", databaseName), ctx)
	if err != nil {
		return closeOnError(err)
	}
	store.controlDB, err = openSQLite(filepath.Join(dataDirectory, "control", controlDatabase), ctx)
	if err != nil {
		return closeOnError(err)
	}
	if err := runInitialMigration(ctx, store.db, initialSchemaSQL); err != nil {
		return closeOnError(err)
	}
	if err := runInitialMigration(ctx, store.controlDB, controlSchemaSQL); err != nil {
		return closeOnError(err)
	}
	store.journal, err = os.OpenFile(filepath.Join(dataDirectory, "control", journalName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return closeOnError(platform.NewError(platform.CodeInternal, false, "打开本域恢复日志失败"))
	}
	if err := store.recoverJournal(); err != nil {
		return closeOnError(err)
	}
	return store, nil
}

func openSQLite(path string, ctx context.Context) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开本域数据库失败")
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, mapDatabaseError(err, "本域数据库不可用")
	}
	return db, nil
}

func runInitialMigration(ctx context.Context, db *sql.DB, schema string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return mapDatabaseError(err, "开始本域迁移失败")
	}
	fail := func(code string, retryable bool, message string) error {
		_ = tx.Rollback()
		return platform.NewError(code, retryable, message)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fail(platform.CodeIntegrityError, false, "建立迁移记录失败")
	}
	var version int64
	var name string
	err = tx.QueryRowContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &name)
	if errors.Is(err, sql.ErrNoRows) {
		version = 0
	} else if err != nil {
		return fail(platform.CodeIntegrityError, false, "读取迁移记录失败")
	}
	if version > 1 || (version == 1 && name != initialSchemaName) {
		return fail(platform.CodeIntegrityError, false, "本域迁移版本未知")
	}
	if version == 0 {
		if _, err := tx.ExecContext(ctx, schema); err != nil {
			return fail(platform.CodeIntegrityError, false, "执行本域初始迁移失败")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (1, ?, ?)`, initialSchemaName, platform.Now()); err != nil {
			return fail(platform.CodeInternal, false, "登记本域迁移失败")
		}
	}
	if err := tx.Commit(); err != nil {
		return mapDatabaseError(err, "提交本域迁移失败")
	}
	return nil
}

func (s *Store) loadRuntimeState(ctx context.Context, dataEpoch, siteID platform.Identifier) (int64, error) {
	if !platform.ValidIdentifier(siteID) {
		return 0, platform.NewError(platform.CodeValidation, false, "操作上下文缺少站点身份")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, mapDatabaseError(err, "读取运行代数失败")
	}
	var generation int64
	var storedEpoch string
	err = tx.QueryRowContext(ctx, `SELECT memory_generation, data_epoch FROM runtime_state WHERE id = 1`).Scan(&generation, &storedEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		generation = 1
		if _, err := tx.ExecContext(ctx, `INSERT INTO runtime_state (id, schema_version, memory_generation, data_epoch, mapping_revision, updated_at) VALUES (1, 1, ?, ?, 0, ?)`, generation, dataEpoch, platform.Now()); err != nil {
			_ = tx.Rollback()
			return 0, mapDatabaseError(err, "初始化运行代数失败")
		}
	} else if err != nil {
		_ = tx.Rollback()
		return 0, platform.NewError(platform.CodeIntegrityError, false, "运行代数记录不可读")
	} else if storedEpoch != dataEpoch {
		_ = tx.Rollback()
		return 0, platform.NewError(platform.CodeIntegrityError, false, "本域数据代次不匹配")
	}
	if err := tx.Commit(); err != nil {
		return 0, mapDatabaseError(err, "提交运行代数失败")
	}
	return generation, nil
}

func (s *Store) openJournal(kind string, dataEpoch platform.Identifier) (string, error) {
	if !platform.ValidIdentifier(dataEpoch) {
		return "", platform.NewError(platform.CodeValidation, false, "journal代次不合法")
	}
	id := platform.NewIdentifier()
	entry := journalEntry{ID: id, Kind: kind, State: "open", DataEpoch: dataEpoch, At: platform.Now()}
	return id, s.appendJournal(entry)
}

func (s *Store) completeJournal(id string) error {
	if !platform.ValidIdentifier(id) {
		return platform.NewError(platform.CodeValidation, false, "journal标识不合法")
	}
	return s.appendJournal(journalEntry{ID: id, State: "done", At: platform.Now()})
}

func (s *Store) appendJournal(entry journalEntry) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "恢复日志不可编码")
	}
	s.journalMu.Lock()
	defer s.journalMu.Unlock()
	if s.journal == nil {
		return platform.NewError(platform.CodeInternal, false, "恢复日志未打开")
	}
	if _, err := s.journal.Write(append(raw, '\n')); err != nil {
		return platform.NewError(platform.CodeInternal, false, "恢复日志写入失败")
	}
	if err := s.journal.Sync(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "恢复日志fsync失败")
	}
	return nil
}

func (s *Store) recoverJournal() error {
	if s.journal == nil {
		return platform.NewError(platform.CodeInternal, false, "恢复日志未打开")
	}
	raw, err := os.ReadFile(s.journal.Name())
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "恢复日志不可读")
	}
	states := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || !platform.ValidIdentifier(entry.ID) {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志损坏")
		}
		if entry.State != "open" && entry.State != "done" {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志状态未知")
		}
		if entry.State == "open" && !platform.ValidIdentifier(entry.DataEpoch) {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志代次缺失")
		}
		states[entry.ID] = entry.State
	}
	for _, state := range states {
		if state == "open" {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志存在未完成决定")
		}
	}
	return nil
}

func validateEntries(ctx context.Context, s *Store, siteID platform.Identifier) error {
	rows, err := s.db.QueryContext(ctx, `SELECT entry_id, version, file_ref, body_hash FROM memory_entries WHERE site_id = ? ORDER BY entry_id, version`, siteID)
	if err != nil {
		return mapDatabaseError(err, "读取记忆条目索引失败")
	}
	defer rows.Close()
	for rows.Next() {
		var entryID, fileRefJSON, bodyHash string
		var version int64
		if err := rows.Scan(&entryID, &version, &fileRefJSON, &bodyHash); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "记忆条目索引损坏")
		}
		if err := contextError(ctx); err != nil {
			return err
		}
		if !platform.ValidIdentifier(entryID) || version < 0 || !platform.ValidHash(bodyHash) {
			return platform.NewError(platform.CodeIntegrityError, false, "记忆条目索引字段不合法")
		}
		var ref platform.ObjectRef
		if err := json.Unmarshal([]byte(fileRefJSON), &ref); err != nil || ref.Owner != serviceID || !platform.ValidIdentifier(ref.ObjectID) || !platform.ValidHash(ref.SHA256) || ref.Version != version || ref.ByteSize < 1 {
			return platform.NewError(platform.CodeIntegrityError, false, "记忆正文对象引用不合法")
		}
		if err := verifyObjectFile(s.objectsRoot, entryID, version, ref); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆条目索引读取失败")
	}
	return nil
}

func verifyObjectFile(objectsRoot, entryID string, version int64, ref platform.ObjectRef) error {
	path := filepath.Join(objectsRoot, entryID, strconv.FormatInt(version, 10)+".md")
	info, err := os.Stat(path)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆正文对象不可读")
	}
	if !info.Mode().IsRegular() || info.Size() != int64(ref.ByteSize) {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆正文对象大小不一致")
	}
	file, err := os.Open(path)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆正文对象不可打开")
	}
	defer file.Close()
	hash := sha256.New()
	reader := io.TeeReader(file, hash)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆正文对象读取失败")
	}
	prefix, err := readObjectPrefix(path, info.Size())
	if err != nil || !utf8.Valid(prefix) {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆正文对象不是合法UTF8")
	}
	if hex.EncodeToString(hash.Sum(nil)) != ref.SHA256 {
		return platform.NewError(platform.CodeIntegrityError, false, "记忆正文对象摘要不一致")
	}
	return nil
}

// readObjectPrefix 只用于 UTF-8 快速检查；正文语义和 frontmatter 由后续函数解析。
func readObjectPrefix(path string, size int64) ([]byte, error) {
	const prefixLimit = 1 << 20
	limit := size
	if limit > prefixLimit {
		limit = prefixLimit
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func validateMappings(ctx context.Context, s *Store, siteID platform.Identifier) error {
	rows, err := s.db.QueryContext(ctx, `SELECT mapping_id, task_id, entry_id, order_value, enabled, language, revision,
		EXISTS (SELECT 1 FROM entry_current WHERE site_id = task_mappings.site_id AND entry_id = task_mappings.entry_id) AS entry_exists
		FROM task_mappings WHERE site_id = ?`, siteID)
	if err != nil {
		return mapDatabaseError(err, "读取任务映射失败")
	}
	defer rows.Close()
	for rows.Next() {
		var mappingID, taskID, entryID, language string
		var orderValue, revision int64
		var enabled, entryExists int
		if err := rows.Scan(&mappingID, &taskID, &entryID, &orderValue, &enabled, &language, &revision, &entryExists); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "任务映射记录损坏")
		}
		if err := contextError(ctx); err != nil {
			return err
		}
		if !platform.ValidIdentifier(mappingID) || !allowedTask(taskID) || !platform.ValidIdentifier(entryID) || language == "" || orderValue < 0 || orderValue > 10000 || (enabled != 0 && enabled != 1) || revision < 0 || entryExists != 1 {
			return platform.NewError(platform.CodeIntegrityError, false, "任务映射字段不合法")
		}
	}
	if err := rows.Err(); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "任务映射读取失败")
	}
	return nil
}

func validateBudgets(ctx context.Context, s *Store, siteID platform.Identifier) error {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, maximum_bytes, maximum_entries, revision FROM task_budgets WHERE site_id = ?`, siteID)
	if err != nil {
		return mapDatabaseError(err, "读取任务预算失败")
	}
	defer rows.Close()
	for rows.Next() {
		var taskID string
		var maximumBytes, maximumEntries, revision int64
		if err := rows.Scan(&taskID, &maximumBytes, &maximumEntries, &revision); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "任务预算记录损坏")
		}
		if !allowedTask(taskID) || maximumBytes < 1024 || maximumBytes > 65536 || maximumEntries < 0 || maximumEntries > 64 || revision < 0 {
			return platform.NewError(platform.CodeIntegrityError, false, "任务预算字段不合法")
		}
		if err := contextError(ctx); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "任务预算读取失败")
	}
	return nil
}

func validateApprovedCommands(ctx context.Context, s *Store, siteID platform.Identifier) error {
	rows, err := s.db.QueryContext(ctx, `SELECT command_id, proposal_id, payload_hash, state FROM proposal_commands WHERE site_id = ?`, siteID)
	if err != nil {
		return mapDatabaseError(err, "读取批准命令失败")
	}
	defer rows.Close()
	for rows.Next() {
		var commandID, proposalID, payloadHash, state string
		if err := rows.Scan(&commandID, &proposalID, &payloadHash, &state); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "批准命令记录损坏")
		}
		if !platform.ValidIdentifier(commandID) || !platform.ValidIdentifier(proposalID) || !platform.ValidHash(payloadHash) {
			return platform.NewError(platform.CodeIntegrityError, false, "批准命令字段不合法")
		}
		switch state {
		case "pending", "leased", "applying", "applied", "rejected", "failed":
		default:
			return platform.NewError(platform.CodeIntegrityError, false, "批准命令状态未知")
		}
		if err := contextError(ctx); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "批准命令读取失败")
	}
	return nil
}

func validateOperation(op OperationContext) error {
	if op.CallerService != serviceID || !platform.ValidIdentifier(op.RequestID) || !platform.ValidIdentifier(op.OperationID) || !platform.ValidIdentifier(op.RootOperationID) || !platform.ValidIdentifier(op.DataEpoch) {
		return platform.NewError(platform.CodeForbidden, false, "启动操作上下文不可信")
	}
	if _, err := platform.ParseTimestamp(op.Deadline); err != nil {
		return platform.NewError(platform.CodeValidation, false, "启动截止时间不合法")
	}
	return nil
}

func functionContext(ctx context.Context, op OperationContext) (context.Context, context.CancelFunc, error) {
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		return nil, nil, platform.NewError(platform.CodeValidation, false, "启动截止时间不合法")
	}
	limit := deadline
	if candidate := time.Now().UTC().Add(30 * time.Second); candidate.Before(limit) {
		limit = candidate
	}
	child, cancel := context.WithDeadline(ctx, limit)
	return child, cancel, nil
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, false, "启动已取消")
	} else if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, false, "启动已超过截止时间")
	}
	return nil
}

func mapDatabaseError(err error, message string) error {
	if errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, false, message)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, false, message)
	}
	return platform.NewError(platform.CodeInternal, true, message)
}

func validText(value string) bool {
	return value != "" && utf8.ValidString(value) && len(value) <= 65536
}

func allowedTask(taskID string) bool {
	switch taskID {
	case "qa.preflight", "qa.inspect", "qa.answer", "qa.cache-match", "qa.cache-translate", "qa.compact",
		"knowledge.context", "knowledge.describe", "knowledge.describe-visual", "knowledge.extract-attachment", "knowledge.embed", "knowledge.rerank",
		"localization.document", "localization.ui", "localization.terms", "localization.review", "localization.suggest",
		"memory.overview", "memory.assistant", "content.slug", "content.seo":
		return true
	default:
		return false
	}
}

func install(store *Store) {
	installed.Lock()
	previous := installed.store
	installed.store = store
	installed.Unlock()
	if previous != nil && previous != store {
		_ = previous.Close()
	}
}

// Close 先 flush journal，再关闭数据库；未完成 journal 不会被静默标记完成。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		if s.journal != nil {
			if err := s.journal.Sync(); err != nil {
				s.closeErr = platform.NewError(platform.CodeInternal, false, "恢复日志flush失败")
			}
			if err := s.journal.Close(); err != nil && s.closeErr == nil {
				s.closeErr = platform.NewError(platform.CodeInternal, false, "恢复日志关闭失败")
			}
		}
		if s.controlDB != nil {
			if err := s.controlDB.Close(); err != nil && s.closeErr == nil {
				s.closeErr = platform.NewError(platform.CodeInternal, false, "控制数据库关闭失败")
			}
		}
		if s.db != nil {
			if err := s.db.Close(); err != nil && s.closeErr == nil {
				s.closeErr = platform.NewError(platform.CodeInternal, false, "业务数据库关闭失败")
			}
		}
	})
	installed.Lock()
	if installed.store == s {
		installed.store = nil
	}
	installed.Unlock()
	return s.closeErr
}

type journalEntry struct {
	ID        string `json:"id"`
	Kind      string `json:"kind,omitempty"`
	State     string `json:"state"`
	DataEpoch string `json:"dataEpoch,omitempty"`
	At        string `json:"at"`
}

const controlSchemaSQL = `
CREATE TABLE IF NOT EXISTS control_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    barrier_state TEXT NOT NULL CHECK (barrier_state IN ('released', 'draining', 'safe', 'failed')),
    data_epoch TEXT NOT NULL,
    safe_point INTEGER NOT NULL CHECK (safe_point IN (0, 1)),
    updated_at TEXT NOT NULL
);
INSERT OR IGNORE INTO control_state (id, barrier_state, data_epoch, safe_point, updated_at)
VALUES (1, 'released', '', 1, '');
CREATE TABLE IF NOT EXISTS restore_receipts (
    restore_id TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL,
    checksum TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL
);`

const initialSchemaSQL = `
CREATE TABLE memory_entries (
    site_id TEXT NOT NULL, entry_id TEXT NOT NULL, version INTEGER NOT NULL CHECK (version >= 0),
    title TEXT NOT NULL, file_ref TEXT NOT NULL, language TEXT NOT NULL, tags_json TEXT NOT NULL,
    priority INTEGER NOT NULL CHECK (priority BETWEEN 0 AND 1000), state TEXT NOT NULL CHECK (state IN ('active', 'archived')),
    updated_at TEXT NOT NULL, body_hash TEXT NOT NULL, description TEXT NOT NULL,
    maximum_bytes INTEGER NOT NULL CHECK (maximum_bytes BETWEEN 0 AND 9007199254740991),
    readonly INTEGER NOT NULL CHECK (readonly IN (0, 1)), source TEXT NOT NULL CHECK (source IN ('manual', 'assistant', 'auto')),
    auto_update_enabled INTEGER NOT NULL CHECK (auto_update_enabled IN (0, 1)), PRIMARY KEY (site_id, entry_id, version));
CREATE INDEX idx_memory_entries_current ON memory_entries (site_id, entry_id, state, version DESC);
CREATE INDEX idx_memory_entries_epoch ON memory_entries (site_id, updated_at, entry_id, version);
CREATE TABLE entry_current (site_id TEXT NOT NULL, entry_id TEXT NOT NULL, version INTEGER NOT NULL CHECK (version >= 0), PRIMARY KEY (site_id, entry_id), FOREIGN KEY (site_id, entry_id, version) REFERENCES memory_entries (site_id, entry_id, version));
CREATE TABLE entry_revisions (site_id TEXT NOT NULL, entry_id TEXT NOT NULL, version INTEGER NOT NULL CHECK (version >= 0), body_hash TEXT NOT NULL, file_ref TEXT NOT NULL, actor_id TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY (site_id, entry_id, version));
CREATE TABLE task_mappings (site_id TEXT NOT NULL, mapping_id TEXT NOT NULL, task_id TEXT NOT NULL, entry_id TEXT NOT NULL, order_value INTEGER NOT NULL CHECK (order_value BETWEEN 0 AND 10000), enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)), language TEXT NOT NULL, revision INTEGER NOT NULL CHECK (revision >= 0), PRIMARY KEY (site_id, mapping_id));
CREATE UNIQUE INDEX idx_task_mappings_identity ON task_mappings (site_id, task_id, entry_id, language);
CREATE INDEX idx_task_mappings_order ON task_mappings (site_id, task_id, language, enabled, order_value, mapping_id);
CREATE TABLE task_budgets (site_id TEXT NOT NULL, task_id TEXT NOT NULL, maximum_bytes INTEGER NOT NULL CHECK (maximum_bytes BETWEEN 1024 AND 65536), maximum_entries INTEGER NOT NULL CHECK (maximum_entries BETWEEN 0 AND 64), revision INTEGER NOT NULL CHECK (revision >= 0), PRIMARY KEY (site_id, task_id));
CREATE TABLE context_snapshots (site_id TEXT NOT NULL, snapshot_id TEXT NOT NULL, task_id TEXT NOT NULL, language TEXT NOT NULL, mapping_revision INTEGER NOT NULL CHECK (mapping_revision >= 0), manifest_hash TEXT NOT NULL, body_hash TEXT NOT NULL, byte_size INTEGER NOT NULL CHECK (byte_size >= 0), created_at TEXT NOT NULL, PRIMARY KEY (site_id, snapshot_id));
CREATE TABLE assistant_runs (site_id TEXT NOT NULL, run_id TEXT NOT NULL, source_generation INTEGER NOT NULL CHECK (source_generation >= 0), state TEXT NOT NULL CHECK (state IN ('queued', 'running', 'review', 'completed', 'cancelled', 'failed', 'superseded')), invocation_id TEXT, actor_id TEXT NOT NULL, proposal_ids_json TEXT NOT NULL, PRIMARY KEY (site_id, run_id));
CREATE TABLE proposals (site_id TEXT NOT NULL, proposal_id TEXT NOT NULL, run_id TEXT NOT NULL, kind TEXT NOT NULL CHECK (kind IN ('entry_create', 'entry_update', 'entry_archive', 'glossary')), state TEXT NOT NULL CHECK (state IN ('pending', 'approved', 'applying', 'applied', 'rejected', 'conflicted')), payload_hash TEXT NOT NULL, expected_entry_version INTEGER, approved_by TEXT, approval_revision INTEGER NOT NULL CHECK (approval_revision >= 0), PRIMARY KEY (site_id, proposal_id));
CREATE TABLE proposal_commands (site_id TEXT NOT NULL, command_id TEXT NOT NULL, proposal_id TEXT NOT NULL, payload_hash TEXT NOT NULL, state TEXT NOT NULL CHECK (state IN ('pending', 'leased', 'applying', 'applied', 'rejected', 'failed')), localization_receipt TEXT, PRIMARY KEY (site_id, command_id), UNIQUE (site_id, proposal_id));
CREATE TABLE inbox (event_id TEXT PRIMARY KEY, payload_hash TEXT NOT NULL, state TEXT NOT NULL, received_at TEXT NOT NULL);
CREATE TABLE outbox (event_id TEXT PRIMARY KEY, payload_hash TEXT NOT NULL, state TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE assistant_conversations (site_id TEXT NOT NULL, conversation_id TEXT NOT NULL, actor_id TEXT NOT NULL, version INTEGER NOT NULL CHECK (version >= 0), expires_at TEXT NOT NULL, PRIMARY KEY (site_id, conversation_id));
CREATE TABLE assistant_messages (site_id TEXT NOT NULL, message_id TEXT NOT NULL, conversation_id TEXT NOT NULL, ordinal INTEGER NOT NULL CHECK (ordinal >= 0), role TEXT NOT NULL CHECK (role IN ('user', 'assistant')), body TEXT NOT NULL, run_id TEXT, created_at TEXT NOT NULL, PRIMARY KEY (site_id, message_id), UNIQUE (site_id, conversation_id, ordinal));
CREATE TABLE runtime_state (id INTEGER PRIMARY KEY CHECK (id = 1), schema_version INTEGER NOT NULL CHECK (schema_version = 1), memory_generation INTEGER NOT NULL CHECK (memory_generation >= 0), data_epoch TEXT NOT NULL, mapping_revision INTEGER NOT NULL CHECK (mapping_revision >= 0), updated_at TEXT NOT NULL);`
