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

	_ "nice-wiki/libs/platform/pgdb"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/memory/internal/gen"
	memorymigrations "nice-wiki/modules/memory/migrations"
)

const (
	serviceID       = "memory"
	databaseName    = "memory.db"
	controlDatabase = "control.db"
	journalName     = "memory-journal.jsonl"

	// journalKindRuntimeLoad 是启动只读校验的journal条目；中断时没有业务副作用，
	// 恢复时补记aborted即可，不应把一次失败的启动变成永久完整性错误。
	journalKindRuntimeLoad = "runtime.load"
)

// OperationContext 使用平台的可信操作上下文，避免在本域复制公共协议。
type OperationContext = platform.OperationContext

// LoadRuntimeInput 是启动恢复所需的封闭输入。
type LoadRuntimeInput struct {
	ServiceID     string
	DataDirectory string
}

// LoadRuntimeResult 是启动恢复结果；不暴露路径、正文或控制日志内容。
type LoadRuntimeResult struct {
	Ready            bool
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

	journal    *os.File
	journalMu  sync.Mutex
	writeMu    sync.Mutex
	window     platform.WriteWindowConfig
	gateway    platform.GatewayClient
	publisher  platform.EventPublisher
	jobs       *gen.JobsClient
	gatewayRaw *gen.ModelGatewayClient
	validators map[string]func(json.RawMessage) error
	closeOnce  sync.Once
	closeErr   error
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
	journalID, err := store.openJournal(journalKindRuntimeLoad, op.DataEpoch)
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
	abort := func(cause error) (LoadRuntimeResult, error) {
		_ = store.abortJournal(journalID)
		_ = store.Close()
		return LoadRuntimeResult{}, cause
	}
	for _, check := range checks {
		if err := contextError(deadlineCtx); err != nil {
			return abort(err)
		}
		if err := check(deadlineCtx, store, op.Actor.SiteID); err != nil {
			return abort(err)
		}
	}

	generation, err := store.loadRuntimeState(deadlineCtx, op.DataEpoch, op.Actor.SiteID)
	if err != nil {
		return abort(err)
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
	for _, directory := range []string{"active", "control", "restore", "active/objects"} {
		if err := os.MkdirAll(filepath.Join(dataDirectory, directory), 0o700); err != nil {
			return nil, platform.NewError(platform.CodeInternal, false, "建立私有数据目录失败")
		}
	}

	legacy, legacyErr := os.ReadDir(filepath.Join(dataDirectory, "objects"))
	if legacyErr == nil && len(legacy) > 0 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "旧根对象目录非空，需显式迁移后启动")
	}
	if legacyErr != nil && !errors.Is(legacyErr, os.ErrNotExist) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "旧根对象目录不可核对")
	}
	store := &Store{objectsRoot: filepath.Join(dataDirectory, "active", "objects"), dataEpoch: dataEpoch}
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
	activeMigrations, err := memorymigrations.All()
	if err != nil {
		return closeOnError(platform.NewError(platform.CodeIntegrityError, false, "本域业务库迁移集不合法"))
	}
	controlMigrations, err := memorymigrations.Control()
	if err != nil {
		return closeOnError(platform.NewError(platform.CodeIntegrityError, false, "本域控制库迁移集不合法"))
	}
	if err := runMigrations(ctx, store.db, activeMigrations); err != nil {
		return closeOnError(err)
	}
	if err := runMigrations(ctx, store.controlDB, controlMigrations); err != nil {
		return closeOnError(err)
	}
	// 跨服务事件只经平台 nw_outbox/nw_inbox（ADR0005），与业务写入同库同事务。
	if err := outbox.EnsureSchema(ctx, store.db); err != nil {
		return closeOnError(err)
	}
	if err := inbox.EnsureSchema(ctx, store.db); err != nil {
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

// runMigrations 按编号升序执行内嵌只追加迁移。首个迁移自建 schema_migrations；
// 已应用版本超出迁移集或名称不符时拒绝启动，每个迁移与登记在同一事务提交。
func runMigrations(ctx context.Context, db *sql.DB, all []memorymigrations.Migration) error {
	applied := map[int64]string{}
	rows, err := db.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return mapDatabaseError(err, "读取迁移记录失败")
		}
	} else {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				_ = rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return mapDatabaseError(err, "读取迁移记录失败")
		}
		_ = rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeIntegrityError, false, "本域迁移版本未知")
		}
	}
	for _, migration := range all {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return mapDatabaseError(err, "开始本域迁移失败")
		}
		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "执行本域迁移失败")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, migration.Version, migration.Name, platform.Now()); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeInternal, false, "登记本域迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return mapDatabaseError(err, "提交本域迁移失败")
		}
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

// abortJournal 记录未完成且无副作用的决定已放弃，避免下次启动误判为未决。
func (s *Store) abortJournal(id string) error {
	if !platform.ValidIdentifier(id) {
		return platform.NewError(platform.CodeValidation, false, "journal标识不合法")
	}
	return s.appendJournal(journalEntry{ID: id, State: "aborted", At: platform.Now()})
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
	// 按条目ID聚合：open后跟done/aborted即已结束；最后状态仍为open的才是未决决定。
	type pending struct {
		kind  string
		state string
	}
	states := map[string]pending{}
	order := make([]string, 0)
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || !platform.ValidIdentifier(entry.ID) {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志损坏")
		}
		if entry.State != "open" && entry.State != "done" && entry.State != "aborted" {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志状态未知")
		}
		if entry.State == "open" && (!platform.ValidIdentifier(entry.DataEpoch) || entry.Kind == "") {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志代次缺失")
		}
		current, seen := states[entry.ID]
		if !seen {
			order = append(order, entry.ID)
		}
		if entry.State == "open" {
			current.kind = entry.Kind
		}
		current.state = entry.State
		states[entry.ID] = current
	}
	for _, id := range order {
		entry := states[id]
		if entry.state != "open" {
			continue
		}
		if entry.kind != journalKindRuntimeLoad {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复日志存在未完成决定")
		}
		// 启动只读校验被中断：没有业务副作用，补记aborted后继续本次启动。
		if err := s.abortJournal(id); err != nil {
			return err
		}
	}
	return nil
}

func validateEntries(ctx context.Context, s *Store, siteID platform.Identifier) error {
	rows, err := s.db.QueryContext(ctx, `SELECT site_id,entry_id,version FROM memory_entries ORDER BY site_id,entry_id,version`)
	if err != nil {
		return mapDatabaseError(err, "读取记忆条目索引失败")
	}
	type revision struct {
		site, id string
		version  int
	}
	revisions := []revision{}
	for rows.Next() {
		var r revision
		if err = rows.Scan(&r.site, &r.id, &r.version); err != nil {
			rows.Close()
			return mapDatabaseError(err, "读取记忆条目索引失败")
		}
		revisions = append(revisions, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return mapDatabaseError(err, "读取记忆条目索引失败")
	}
	for _, r := range revisions {
		if _, err = s.ReadEntryObject(ctx, s.db, r.site, r.id, &r.version); err != nil {
			return err
		}
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
		FROM task_mappings`)
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
	rows, err := s.db.QueryContext(ctx, `SELECT task_id, maximum_bytes, maximum_entries, revision FROM task_budgets`)
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
	rows, err := s.db.QueryContext(ctx, `SELECT command_id, proposal_id, payload_hash, state FROM proposal_commands`)
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
		return platform.NewError(platform.CodeCancelled, false, "%s", message)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, false, "%s", message)
	}
	return platform.NewError(platform.CodeInternal, true, "%s", message)
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
