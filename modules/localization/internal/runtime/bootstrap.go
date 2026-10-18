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
	"nice-wiki/modules/localization/internal/gen"
	localizationmigrations "nice-wiki/modules/localization/migrations"
)

const (
	moduleSlug   = "localization"
	databaseName = "localization.db"
	journalName  = "localization-journal.jsonl"
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
	gateway              platform.GatewayClient
	publisher            platform.EventPublisher
	content              *gen.ContentClient
	sourceValidators     map[string]func(json.RawMessage) error
	cursorKey            platform.SigningKey
	root                 string
	db                   *sql.DB
	journal              *os.File
	mu                   sync.Mutex
	closeOnce            sync.Once
	closeErr             error
	journalRecovered     bool
	appliedMigrations    int64
	glossaryGeneration   int64
	leaseRecoveryPending int64
	objectGenerations    map[string]int
}

type journalEntry struct {
	ID      string             `json:"id"`
	Kind    string             `json:"kind"`
	State   string             `json:"state"`
	Payload json.RawMessage    `json:"payload,omitempty"`
	At      platform.Timestamp `json:"at"`
}

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

var (
	languageGatewayMu      sync.Mutex
	languageGatewayStore   *Store
	languageGatewayRuntime platform.SiteConfigClient
)

// InstallLanguageGateway 把已打开的业务库和运行权威交给语言配置函数。
// 调用方仍负责最终关闭存储。
func InstallLanguageGateway(store *Store, runtimeClient platform.SiteConfigClient) {
	languageGatewayMu.Lock()
	defer languageGatewayMu.Unlock()
	languageGatewayStore = store
	languageGatewayRuntime = runtimeClient
}

// ClearLanguageGateway 在启动失败关闭前解除语言配置引用。
func ClearLanguageGateway() {
	languageGatewayMu.Lock()
	defer languageGatewayMu.Unlock()
	languageGatewayStore = nil
	languageGatewayRuntime = nil
}

// LanguageGateway 返回当前语言配置使用的存储和运行权威。
func LanguageGateway() (*Store, platform.SiteConfigClient) {
	languageGatewayMu.Lock()
	defer languageGatewayMu.Unlock()
	return languageGatewayStore, languageGatewayRuntime
}

// SetDataEpoch 在迁移完成后写入本域代次水位。已有不同水位时拒绝覆盖。
func (s *Store) SetDataEpoch(ctx context.Context, dataEpoch platform.Identifier) error {
	if s == nil || s.db == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "本域代次水位存储未装配")
	}
	if !platform.ValidIdentifier(dataEpoch) {
		return platform.NewError(platform.CodeValidation, false, "当前数据代次不合法")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeInternal, true, "开始代次水位事务失败")
	}
	defer func() { _ = tx.Rollback() }()
	var current string
	if err := tx.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton = 1`).Scan(&current); err != nil {
		return platform.NormalizeIOError(err, platform.CodeIntegrityError, false, "本域代次水位不可读")
	}
	if current == string(dataEpoch) {
		return nil
	}
	if current != "" {
		return platform.NewError(platform.CodeVersionMismatch, false, "本域代次水位与运行权威不一致")
	}
	result, err := tx.ExecContext(ctx, `UPDATE runtime_meta SET data_epoch = ?, updated_at = ? WHERE singleton = 1 AND data_epoch = ''`, string(dataEpoch), string(platform.Now()))
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "写入本域代次水位失败")
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "写入本域代次水位失败")
	}
	if changed != 1 {
		return platform.NewError(platform.CodeVersionMismatch, false, "本域代次水位已变化")
	}
	if err := tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "提交代次水位失败")
	}
	return nil
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

// runMigrations 按编号执行 migrations/ 内嵌的只追加迁移；首个迁移自建
// schema_migrations，已应用版本超出迁移集或名称不符时拒绝启动。
func (s *Store) runMigrations(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	all, err := localizationmigrations.All()
	if err != nil {
		return (platform.NewError(platform.CodeIntegrityError, false, "本域迁移集不合法"))
	}
	applied := map[int64]string{}
	rows, err := s.db.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return (platform.NewError(platform.CodeIntegrityError, false, "读取本域迁移记录失败"))
		}
	} else {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				_ = rows.Close()
				return (platform.NewError(platform.CodeIntegrityError, false, "本域迁移记录损坏"))
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return (platform.NewError(platform.CodeIntegrityError, false, "读取本域迁移记录失败"))
		}
		_ = rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return (platform.NewError(platform.CodeIntegrityError, false, "本域迁移版本未知"))
		}
	}
	for _, migration := range all {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return (platform.NewError(platform.CodeInternal, true, "开始本域迁移事务失败"))
		}
		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			_ = tx.Rollback()
			return (platform.NewError(platform.CodeIntegrityError, false, "执行本域迁移失败"))
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			migration.Version, migration.Name, string(platform.Now())); err != nil {
			_ = tx.Rollback()
			return (platform.NewError(platform.CodeInternal, false, "登记本域迁移失败"))
		}
		if err := tx.Commit(); err != nil {
			return (platform.NewError(platform.CodeInternal, false, "提交本域迁移失败"))
		}
	}
	s.appliedMigrations = int64(len(all))
	return nil
}

func validateSchema(ctx context.Context, db *sql.DB) error {
	required := map[string][]string{
		"languages":                 {"language_tag", "default_language", "automatic_translation"},
		"translation_jobs":          {"job_id", "state", "lease_id"},
		"translation_units":         {"unit_id", "protected", "human_generation"},
		"translation_manifests":     {"translation_id", "glossary_generation", "state"},
		"glossary":                  {"term_id", "generation", "source"},
		"interface_catalog":         {"language", "key", "placeholders"},
		"language_preferences":      {"visitor_subject", "expires_at"},
		"approved_proposals":        {"proposal_id", "proposal_hash", "state"},
		"cache_answer_translations": {"translation_id", "answer_revision", "state"},
		"schema_migrations":         {"version", "name", "applied_at"},
		"operation_receipts":        {"site_id", "actor_id", "function_id", "input_hash"},
		"runtime_meta":              {"singleton", "data_epoch", "updated_at"},
		"local_barrier":             {"singleton", "state", "data_epoch"},
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
	s.objectGenerations = map[string]int{}
	for _, entry := range entries {
		if entry.Kind == "object.generation" {
			var reserved struct {
				Key        string `json:"key"`
				Generation int    `json:"generation"`
			}
			if entry.State != "done" || json.Unmarshal(entry.Payload, &reserved) != nil || reserved.Key == "" || reserved.Generation < 1 {
				return platform.NewError(platform.CodeIntegrityError, false, "译文对象代数日志损坏")
			}
			if reserved.Generation > s.objectGenerations[reserved.Key] {
				s.objectGenerations[reserved.Key] = reserved.Generation
			}
		}
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

// Root 仅供本域权威对象存储定位active目录，不能跨服务传递或序列化。
func (s *Store) Root() string { return s.root }

// ReserveObjectGeneration 在control日志只追加预留代数，不激活任何正文；失败事务的文件永远不会被复用覆盖。
func (s *Store) ReserveObjectGeneration(key string, minimum int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objectGenerations == nil {
		s.objectGenerations = map[string]int{}
	}
	generation := minimum
	if generation <= s.objectGenerations[key] {
		generation = s.objectGenerations[key] + 1
	}
	if generation > 9007199254740991 {
		return 0, platform.NewError(platform.CodeIntegrityError, false, "译文对象代数超过安全整数上限")
	}
	payload, err := platform.CanonicalJSON(map[string]any{"key": key, "generation": generation})
	if err != nil {
		return 0, platform.NewError(platform.CodeInternal, false, "译文代数预留编码失败")
	}
	raw, err := json.Marshal(journalEntry{ID: platform.NewIdentifier(), Kind: "object.generation", State: "done", Payload: payload, At: platform.Now()})
	if err != nil {
		return 0, platform.NewError(platform.CodeInternal, false, "译文代数日志编码失败")
	}
	if _, err = s.journal.Write(append(raw, '\n')); err != nil {
		return 0, platform.NewError(platform.CodeInternal, false, "译文代数日志写入失败")
	}
	if err = s.journal.Sync(); err != nil {
		return 0, platform.NewError(platform.CodeInternal, false, "译文代数日志同步失败")
	}
	s.objectGenerations[key] = generation
	return generation, nil
}
