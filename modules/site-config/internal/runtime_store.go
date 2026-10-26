// Package siteconfig 实现 site-config 的私有运行状态控制库。
//
// control.db 是本域恢复协议的权威存储，位于 control/ 而非 active/，
// 因此不会被业务快照换目录覆盖。本文件同时承载 active 业务索引的
// 数据库句柄与只追加迁移；正文和公告决定仍由文件/journal恢复函数负责。
package siteconfig

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform/pgdb"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/site-config/internal/gen"
	"nice-wiki/modules/site-config/internal/migrations"
)

const runtimeDatabaseName = "control.db"

// requiredRuntimeModules 是需要登记恢复切换/换回回执的有状态域。
// portal 是无状态入口，不在 ActivationReceipt.module 枚举中。
var requiredRuntimeModules = []string{
	"identity",
	"site-config",
	"content",
	"media",
	"public-site",
	"model-gateway",
	"knowledge",
	"qa",
	"localization",
	"memory",
	"traffic-guard",
	"jobs",
	"analytics",
	"data-protection",
}

// RuntimeStore 是本域控制库句柄；同一进程仅允许一个活动实例。
type RuntimeStore struct {
	db                      *sql.DB
	activationMu            sync.RWMutex
	activationAnchors       platform.TrustAnchors
	activationVerifierReady bool
	closeOnce               sync.Once
	closeErr                error
}

var activeStore struct {
	sync.RWMutex
	store *RuntimeStore
}

// activeBusinessStore 是 active 区的独占句柄。它与 control.db 分离，
// 恢复换目录只替换 active，不会覆盖运行状态权威。
var activeBusinessStore struct {
	sync.RWMutex
	store *ActiveStore
}

// ActiveStore 是 site-config active 区私有数据库与不可变正文目录的句柄。
// 数据库保存索引、水位及独立持久初始化决定，正文文件仍是权威数据。
type ActiveStore struct {
	db               *sql.DB
	activeRoot       string
	barrierMu        sync.RWMutex
	barrierReader    snapshotBarrierStatusReader
	configMu         sync.Mutex
	settingsMu       sync.Mutex
	initializationMu sync.Mutex
	publicBaseURL    string
	platformSite     atomic.Value // string：平台自己的站点，第一次查到后缓存
	allowLocalHTTP   bool
	homeRecovered    bool
	homeRuntime      HomeRuntime
	configRecovered  bool
	cursorKey        platform.SigningKey
	cursorReady      bool

	stateMu          sync.RWMutex
	journalRecovered bool
	journalHash      platform.Hash
	closeOnce        sync.Once
	closeErr         error
}

type activeMigration struct {
	Version int64
	Name    string
	SQL     string
}

var activeMigrationNamePattern = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

// activeMigrationFiles 只嵌入 active 迁移；control.db 的 migrations 包不读取
// 此目录，避免把业务表错误地建进不可替换的控制库。
//
//go:embed migrations/active/*.sql
var activeMigrationFiles embed.FS

// OpenRuntimeStore 打开 control/control.db，设置 ADR0005 要求的 SQLite
// 参数，执行只追加迁移，并在首次建库时写入 running 初始状态。
func OpenRuntimeStore(ctx context.Context, dataRoot string) (*RuntimeStore, error) {
	if dataRoot == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "数据根缺失")
	}
	path := filepath.Join(dataRoot, "control", runtimeDatabaseName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库目录不可用")
	}
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开运行控制库失败")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &RuntimeStore{db: db}
	closeOnError := func(openErr error) (*RuntimeStore, error) {
		_ = db.Close()
		return nil, openErr
	}
	if err := db.PingContext(ctx); err != nil {
		return closeOnError(platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用"))
	}
	if err := store.runMigrations(ctx); err != nil {
		return closeOnError(err)
	}
	if err := store.ensureInitialState(ctx); err != nil {
		return closeOnError(err)
	}
	activeStore.Lock()
	activeStore.store = store
	activeStore.Unlock()
	return store, nil
}

// ConfigureActivationVerifier 只由BuildApplication装配已加载的信任锚；HTTP输入不携带锚。
func ConfigureActivationVerifier(anchors platform.TrustAnchors) error {
	store := activeRuntimeStore()
	if store == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	store.ConfigureActivationVerifier(anchors)
	return nil
}

// ConfigureActivationVerifier 只由BuildApplication装配已加载的信任锚；HTTP输入不携带锚。
func (s *RuntimeStore) ConfigureActivationVerifier(anchors platform.TrustAnchors) {
	s.activationMu.Lock()
	s.activationAnchors = anchors
	s.activationVerifierReady = true
	s.activationMu.Unlock()
}

func (s *RuntimeStore) activationVerifierAnchors() (platform.TrustAnchors, error) {
	s.activationMu.RLock()
	defer s.activationMu.RUnlock()
	if !s.activationVerifierReady || len(s.activationAnchors.Services) == 0 {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "激活回执验签锚未装配")
	}
	return s.activationAnchors, nil
}

// OpenActiveStore 打开 active/site-config.db，执行只追加迁移并校验活动
// dataEpoch。新库只建立未初始化状态；不会生成虚假的站点配置或公告。
func OpenActiveStore(ctx context.Context, dataRoot string, dataEpoch platform.Identifier) (*ActiveStore, error) {
	if dataRoot == "" || !platform.ValidIdentifier(dataEpoch) {
		return nil, platform.NewError(platform.CodeValidation, false, "active存储参数不合法")
	}
	activeRoot := filepath.Join(dataRoot, "active")
	if err := os.MkdirAll(filepath.Join(activeRoot, "journal"), 0o700); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "active存储目录不可用")
	}
	if err := os.MkdirAll(filepath.Join(activeRoot, "quarantine", "announcement"), 0o700); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "active隔离目录不可用")
	}
	path := filepath.Join(activeRoot, "site-config.db")
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(3000)" +
		"&_pragma=synchronous(FULL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开active业务库失败")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &ActiveStore{db: db, activeRoot: activeRoot}
	closeOnError := func(openErr error) (*ActiveStore, error) {
		_ = db.Close()
		return nil, openErr
	}
	if err := db.PingContext(ctx); err != nil {
		return closeOnError(platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用"))
	}
	if err := store.runActiveMigrations(ctx); err != nil {
		return closeOnError(err)
	}
	if err := outbox.EnsureSchema(ctx, db); err != nil {
		return closeOnError(err)
	}
	if err := registerSiteConfigWorkTables(ctx, db); err != nil {
		return closeOnError(err)
	}
	if err := store.ensureActiveState(ctx, dataEpoch); err != nil {
		return closeOnError(err)
	}
	activeBusinessStore.Lock()
	if activeBusinessStore.store != nil {
		activeBusinessStore.Unlock()
		return closeOnError(platform.NewError(platform.CodeConflict, false, "active业务库已被本进程打开"))
	}
	activeBusinessStore.store = store
	activeBusinessStore.Unlock()
	return store, nil
}

// Close 关闭 active 数据库；重复关闭安全，并撤销本进程的活动句柄。
func (s *ActiveStore) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		activeBusinessStore.Lock()
		if activeBusinessStore.store == s {
			activeBusinessStore.store = nil
		}
		activeBusinessStore.Unlock()
		s.closeErr = s.db.Close()
	})
	if s.closeErr != nil {
		return platform.NewError(platform.CodeInternal, false, "关闭active业务库失败")
	}
	return nil
}

func currentActiveStore() *ActiveStore {
	activeBusinessStore.RLock()
	defer activeBusinessStore.RUnlock()
	return activeBusinessStore.store
}

// OutboxStore 返回与active业务库同库的标准outbox端口；不复制平台领取/封口机制。
func (s *ActiveStore) OutboxStore() *outbox.Store {
	if s == nil || s.db == nil {
		return nil
	}
	return outbox.NewStore(s.db)
}

var siteConfigWorkTables = [...]string{
	"site_config_state",
	"config_revision_index",
	"announcement_heads",
	"initialization_decisions",
	"initialization_state",
	"config_operations",
	"config_operation_materials",
	"home_reference_decisions",
}

func registerSiteConfigWorkTables(ctx context.Context, db *sql.DB) error {
	for _, table := range siteConfigWorkTables {
		if err := outbox.RegisterWorkTable(ctx, db, table); err != nil {
			return err
		}
	}
	return nil
}

func (s *ActiveStore) configureSnapshotBarrier(reader snapshotBarrierStatusReader) {
	s.barrierMu.Lock()
	s.barrierReader = reader
	s.barrierMu.Unlock()
}

func (s *ActiveStore) readSnapshotBarrierStatus() (*lifecycle.SnapshotBarrierStatus, time.Time, bool) {
	s.barrierMu.RLock()
	reader := s.barrierReader
	s.barrierMu.RUnlock()
	if reader == nil {
		return nil, time.Time{}, false
	}
	status, changedAt := reader.ReadBarrierStatus()
	return status, changedAt, true
}

func (s *ActiveStore) runActiveMigrations(ctx context.Context) error {
	all, err := loadActiveMigrations()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "active迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := s.db.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
	if err != nil && !isMissingTable(err) {
		return platform.NewError(platform.CodeIntegrityError, false, "读取active迁移记录失败")
	}
	if err == nil {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "active迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return platform.NewError(platform.CodeDependencyUnavailable, true, "读取active迁移记录失败")
		}
		rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeIntegrityError, false, "active迁移版本或名称不匹配")
		}
	}
	for _, item := range all {
		if _, ok := applied[item.Version]; ok {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "开始active迁移事务失败")
		}
		if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "active迁移执行失败")
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			item.Version, item.Name, string(platform.Now())); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "登记active迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "提交active迁移失败")
		}
	}
	return nil
}

func loadActiveMigrations() ([]activeMigration, error) {
	entries, err := activeMigrationFiles.ReadDir("migrations/active")
	if err != nil {
		return nil, err
	}
	all := make([]activeMigration, 0, len(entries))
	for _, entry := range entries {
		match := activeMigrationNamePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, errors.New("invalid active migration name")
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, err
		}
		raw, err := activeMigrationFiles.ReadFile("migrations/active/" + entry.Name())
		if err != nil {
			return nil, err
		}
		all = append(all, activeMigration{Version: version, Name: entry.Name(), SQL: string(raw)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Version < all[j].Version })
	for i, item := range all {
		if item.Version != int64(i+1) {
			return nil, errors.New("active migration versions are not contiguous")
		}
	}
	return all, nil
}

func (s *ActiveStore) runActiveMigrationsTx(ctx context.Context, tx *sql.Tx) error {
	all, err := loadActiveMigrations()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "active迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := tx.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
	if err != nil && !isMissingTable(err) {
		return platform.NewError(platform.CodeIntegrityError, false, "读取active迁移记录失败")
	}
	if err == nil {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "active迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return platform.NewError(platform.CodeDependencyUnavailable, true, "读取active迁移记录失败")
		}
		rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return platform.NewError(platform.CodeIntegrityError, false, "active迁移版本或名称不匹配")
		}
	}
	for _, item := range all {
		if _, ok := applied[item.Version]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "active迁移执行失败")
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			item.Version, item.Name, string(platform.Now())); err != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "登记active迁移失败")
		}
	}
	return nil
}

func (s *ActiveStore) ensureActiveState(ctx context.Context, dataEpoch platform.Identifier) error {
	ctx = pgdb.WithTemplateView(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "打开active状态事务失败")
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM site_config_state`).Scan(&count); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "读取active状态失败")
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO site_config_state
                (singleton, data_epoch, initialized, site_id, config_revision,
                 config_revision_id, updated_at)
            VALUES (1, ?, 0, NULL, 0, NULL, ?)`,
			string(dataEpoch), string(platform.Now())); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "初始化active状态失败")
		}
	} else if count != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "active状态单例数量不正确")
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "提交active状态失败")
	}
	return s.validateActiveState(ctx, dataEpoch)
}

// ValidateActiveReady 校验 active schema、活动 epoch、journal 恢复指纹和初始化
// 状态。任何未知或矛盾状态都返回错误，调用方不得把它转换成ready成功。
func (s *ActiveStore) ValidateActiveReady(ctx context.Context, dataEpoch platform.Identifier) (bool, error) {
	if s == nil || s.db == nil {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库不可用")
	}
	ctx = pgdb.WithTemplateView(ctx)
	if err := s.validateActiveSchema(ctx); err != nil {
		return false, err
	}
	initialized, err := s.validateActiveStateWithResult(ctx, dataEpoch)
	if err != nil {
		return false, err
	}
	s.stateMu.RLock()
	recovered := s.journalRecovered
	expectedHash := s.journalHash
	configRecovered := s.configRecovered
	homeRecovered := s.homeRecovered
	s.stateMu.RUnlock()
	if !homeRecovered {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "首页引用本地决定尚未恢复")
	}
	if !configRecovered {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "配置journal尚未恢复")
	}
	if err := s.verifyConfigHead(ctx); err != nil {
		return false, err
	}
	if !recovered {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "公告journal尚未恢复")
	}
	currentHash, err := readAnnouncementJournalHash(s.activeRoot)
	if err != nil {
		return false, err
	}
	if currentHash != expectedHash {
		return false, platform.NewError(platform.CodeIntegrityError, false, "公告journal在恢复后发生变化")
	}
	return initialized, nil
}

func (s *ActiveStore) markJournalRecovered(hash platform.Hash) {
	s.stateMu.Lock()
	s.journalRecovered = true
	s.journalHash = hash
	s.stateMu.Unlock()
}

func (s *ActiveStore) validateActiveSchema(ctx context.Context) error {
	return s.validateActiveSchemaOn(ctx, s.db)
}

func (s *ActiveStore) validateActiveSchemaTx(ctx context.Context, tx *sql.Tx) error {
	return s.validateActiveSchemaOn(ctx, tx)
}

func (s *ActiveStore) validateActiveSchemaOn(ctx context.Context, q queryer) error {
	all, err := loadActiveMigrations()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "active迁移集不合法")
	}
	rows, err := q.QueryContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "active schema_migrations不可读")
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var version int64
		var name string
		if err := rows.Scan(&version, &name); err != nil || index >= len(all) || version != all[index].Version || name != all[index].Name {
			return platform.NewError(platform.CodeIntegrityError, false, "active迁移记录与代码不一致")
		}
		index++
	}
	if err := rows.Err(); err != nil || index != len(all) {
		return platform.NewError(platform.CodeIntegrityError, false, "active schema版本不完整")
	}
	return nil
}

func (s *ActiveStore) validateActiveState(ctx context.Context, dataEpoch platform.Identifier) error {
	_, err := s.validateActiveStateWithResult(ctx, dataEpoch)
	return err
}

func (s *ActiveStore) validateActiveStateWithResult(ctx context.Context, dataEpoch platform.Identifier) (bool, error) {
	return s.validateActiveStateWithResultOn(ctx, s.db, dataEpoch)
}

func (s *ActiveStore) validateActiveStateWithResultTx(ctx context.Context, tx *sql.Tx, dataEpoch platform.Identifier) (bool, error) {
	return s.validateActiveStateWithResultOn(ctx, tx, dataEpoch)
}

func (s *ActiveStore) validateActiveStateWithResultOn(ctx context.Context, q queryer, dataEpoch platform.Identifier) (bool, error) {
	if !platform.ValidIdentifier(dataEpoch) {
		return false, platform.NewError(platform.CodeValidation, false, "active dataEpoch不合法")
	}
	var storedEpoch, updatedAt string
	var initialized, revision int64
	var nullableSiteID, nullableRevisionID sql.NullString
	err := q.QueryRowContext(ctx, `
        SELECT data_epoch, initialized, site_id, config_revision,
               config_revision_id, updated_at
          FROM site_config_state WHERE singleton = 1`).Scan(
		&storedEpoch, &initialized, &nullableSiteID, &revision,
		&nullableRevisionID, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, platform.NewError(platform.CodeIntegrityError, false, "active状态缺失")
	}
	if err != nil {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取active状态失败")
	}
	if storedEpoch != string(dataEpoch) || (initialized != 0 && initialized != 1) || revision < 0 {
		return false, platform.NewError(platform.CodeIntegrityError, false, "active状态epoch或枚举不合法")
	}
	if _, err := time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "active状态时间不合法")
	}
	if initialized == 0 && revision == 0 {
		if nullableSiteID.Valid || nullableRevisionID.Valid {
			return false, platform.NewError(platform.CodeIntegrityError, false, "未初始化active状态包含配置引用")
		}
		return false, nil
	}
	if !nullableSiteID.Valid || !platform.ValidIdentifier(nullableSiteID.String) ||
		!nullableRevisionID.Valid || !platform.ValidIdentifier(nullableRevisionID.String) || revision < 1 {
		return false, platform.NewError(platform.CodeIntegrityError, false, "已初始化active状态缺少配置引用")
	}
	if initialized == 0 {
		state, _, readErr := s.ReadInitializationRecord(ctx, q)
		if readErr != nil {
			return false, readErr
		}
		if state.Decision != gen.Commit || state.ConfigRevision != gen.Revision(revision) || (state.Phase != gen.InitializationStatePhaseConfigCommitted && state.Phase != gen.InitializationStatePhaseBlocked) {
			return false, platform.NewError(platform.CodeIntegrityError, false, "未完成初始化的配置水位缺少commit协调状态")
		}
	}
	// 避免数据库仅有状态水位而没有对应权威文件索引。
	var indexed int
	if err := q.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM config_revision_index
         WHERE site_id = ? AND revision_id = ? AND revision = ?`,
		nullableSiteID.String, nullableRevisionID.String, revision).Scan(&indexed); err != nil {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取active配置索引失败")
	}
	if indexed != 1 {
		return false, platform.NewError(platform.CodeIntegrityError, false, "active配置状态没有对应索引")
	}
	return initialized == 1, nil
}

// Close 关闭控制库；重复关闭安全，并在成功关闭时撤销本进程的活动句柄。
func (s *RuntimeStore) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		activeStore.Lock()
		if activeStore.store == s {
			activeStore.store = nil
		}
		activeStore.Unlock()
		s.closeErr = s.db.Close()
	})
	if s.closeErr != nil {
		return platform.NewError(platform.CodeInternal, false, "关闭运行控制库失败")
	}
	return nil
}

// ReadRuntimeView 读取供 platform lifecycle 使用的脱敏运行摘要。
func (s *RuntimeStore) ReadRuntimeView(ctx context.Context) (platform.RuntimeView, error) {
	state, err := s.ReadRuntimeState(ctx)
	if err != nil {
		return platform.RuntimeView{}, err
	}
	return platform.RuntimeView{
		Revision:        platform.Revision(state.Revision),
		BarrierID:       runtimeIdentifier(state.BarrierId),
		DataEpoch:       platform.Identifier(state.DataEpoch),
		Phase:           string(state.Phase),
		RestoreID:       runtimeIdentifier(state.RestoreId),
		TargetDataEpoch: runtimeIdentifier(state.TargetDataEpoch),
		Maintenance:     state.Maintenance,
	}, nil
}

// ReadRuntimeState 读取完整的内部 RuntimeState，供本域就绪检查和未来
// x-handler 适配使用；调用方不得把控制库内部句柄或密钥写入响应。
func (s *RuntimeStore) ReadRuntimeState(ctx context.Context) (gen.RuntimeState, error) {
	return s.readState(ctx)
}

func activeRuntimeStore() *RuntimeStore {
	activeStore.RLock()
	defer activeStore.RUnlock()
	return activeStore.store
}

func (s *RuntimeStore) runMigrations(ctx context.Context) error {
	all, err := migrations.All()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移集不合法")
	}
	applied := map[int64]string{}
	rows, err := s.db.QueryContext(ctx, `SELECT version, name FROM schema_migrations`)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		if !isMissingTable(err) {
			return platform.NewError(platform.CodeIntegrityError, false, "读取运行控制库迁移记录失败")
		}
	} else if err == nil {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				rows.Close()
				return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移记录损坏")
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return platform.NewError(platform.CodeIntegrityError, false, "读取运行控制库迁移记录失败")
		}
		rows.Close()
	}
	for version := range applied {
		if version < 1 || version > int64(len(all)) {
			return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移版本超出迁移集")
		}
	}
	for _, item := range all {
		if _, ok := applied[item.Version]; ok {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "开始运行控制库迁移事务失败")
		}
		if _, err := tx.ExecContext(ctx, item.SQL); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeIntegrityError, false, "运行控制库迁移执行失败")
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			item.Version, item.Name, string(platform.Now())); err != nil {
			_ = tx.Rollback()
			return platform.NewError(platform.CodeInternal, false, "登记运行控制库迁移失败")
		}
		if err := tx.Commit(); err != nil {
			return platform.NewError(platform.CodeInternal, false, "提交运行控制库迁移失败")
		}
	}
	return nil
}

func (s *RuntimeStore) ensureInitialState(ctx context.Context) error {
	modules, err := json.Marshal(requiredRuntimeModules)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "编码运行域集合失败")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "打开运行状态事务失败")
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM runtime_state`).Scan(&count); err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "读取运行状态单例失败")
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO runtime_state
                (singleton, data_epoch, target_data_epoch, phase, maintenance,
                 restore_id, barrier_id, revision, required_modules, updated_at)
            VALUES (1, ?, NULL, 'running', 0, NULL, NULL, 1, ?, ?)`,
			string(platform.NewIdentifier()), string(modules), string(platform.Now())); err != nil {
			return platform.NewError(platform.CodeInternal, false, "初始化运行状态失败")
		}
	} else if count != 1 {
		return platform.NewError(platform.CodeIntegrityError, false, "运行状态单例数量不正确")
	}
	if err := tx.Commit(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "提交初始运行状态失败")
	}
	_, err = s.readState(ctx)
	return err
}

func (s *RuntimeStore) readState(ctx context.Context) (gen.RuntimeState, error) {
	if s == nil || s.db == nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行控制库不可用")
	}
	return s.readStateFrom(ctx, s.db)
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *RuntimeStore) readStateFrom(ctx context.Context, q queryer) (gen.RuntimeState, error) {
	var (
		dataEpoch, phase, requiredJSON, updatedAt string
		targetEpoch, restoreID, barrierID         sql.NullString
		maintenance, revision                     int64
	)
	err := q.QueryRowContext(ctx, `
        SELECT data_epoch, target_data_epoch, phase, maintenance, restore_id,
               barrier_id, revision, required_modules, updated_at
          FROM runtime_state WHERE singleton = 1`).Scan(
		&dataEpoch, &targetEpoch, &phase, &maintenance, &restoreID,
		&barrierID, &revision, &requiredJSON, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态缺失")
	}
	if err != nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeDependencyUnavailable, true, "读取运行状态失败")
	}
	modules := []string{}
	if err := json.Unmarshal([]byte(requiredJSON), &modules); err != nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行域集合损坏")
	}
	if err := validateRequiredModules(modules); err != nil {
		return gen.RuntimeState{}, err
	}
	if !platform.ValidIdentifier(platform.Identifier(dataEpoch)) || revision < 1 {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态基础字段不合法")
	}
	if maintenance != 0 && maintenance != 1 {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行维护标志不合法")
	}
	parsedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return gen.RuntimeState{}, platform.NewError(platform.CodeIntegrityError, false, "运行状态时间不合法")
	}
	state := gen.RuntimeState{
		DataEpoch:       gen.Identifier(dataEpoch),
		TargetDataEpoch: nullableIdentifier(targetEpoch),
		Phase:           gen.RuntimePhase(phase),
		Maintenance:     maintenance == 1,
		RestoreId:       nullableIdentifier(restoreID),
		BarrierId:       nullableIdentifier(barrierID),
		Revision:        gen.Revision(revision),
		RequiredModules: append([]string(nil), modules...),
		UpdatedAt:       parsedAt.UTC(),
	}
	if err := validateRuntimeStateShape(state); err != nil {
		return gen.RuntimeState{}, err
	}
	receipts, err := s.readReceipts(ctx, q, state.RestoreId)
	if err != nil {
		return gen.RuntimeState{}, err
	}
	state.ActivationReceipts = receipts
	return state, nil
}

func (s *RuntimeStore) readReceipts(ctx context.Context, q queryer, restoreID *gen.Identifier) ([]gen.ActivationReceipt, error) {
	if restoreID == nil {
		return []gen.ActivationReceipt{}, nil
	}
	rows, err := q.QueryContext(ctx, `
        SELECT module, stage_id, target_data_epoch, state, manifest_hash,
               recorded_at, signature
          FROM activation_receipts WHERE restore_id = ? ORDER BY module`, *restoreID)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取激活回执失败")
	}
	defer rows.Close()
	receipts := make([]gen.ActivationReceipt, 0)
	for rows.Next() {
		var module, stageID, targetEpoch, state, manifestHash, recordedAt, signature string
		if err := rows.Scan(&module, &stageID, &targetEpoch, &state, &manifestHash, &recordedAt, &signature); err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执损坏")
		}
		parsedAt, err := time.Parse(time.RFC3339Nano, recordedAt)
		if err != nil || !platform.ValidIdentifier(platform.Identifier(stageID)) || !platform.ValidIdentifier(platform.Identifier(targetEpoch)) || !platform.ValidHash(platform.Hash(manifestHash)) || signature == "" {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执字段不合法")
		}
		if !isRequiredModule(module) || (state != string(gen.Switched) && state != string(gen.RolledBack)) {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执枚举不合法")
		}
		receipts = append(receipts, gen.ActivationReceipt{
			Module:          gen.ActivationReceiptModule(module),
			StageId:         gen.Identifier(stageID),
			TargetDataEpoch: gen.Identifier(targetEpoch),
			State:           gen.ActivationReceiptState(state),
			ManifestHash:    gen.Hash(manifestHash),
			RecordedAt:      parsedAt.UTC(),
			Signature:       signature,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "读取激活回执失败")
	}
	if len(receipts) > 30 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "激活回执数量超限")
	}
	return receipts, nil
}

func (s *RuntimeStore) readOperation(ctx context.Context, q queryer, operationID, kind, inputHash string) (gen.RuntimeState, bool, error) {
	if operationID == "" {
		return gen.RuntimeState{}, false, nil
	}
	var storedKind, storedHash, resultJSON string
	err := q.QueryRowContext(ctx, `
        SELECT kind, input_hash, result_json
          FROM runtime_operation_receipts WHERE operation_id = ?`, string(operationID)).Scan(
		&storedKind, &storedHash, &resultJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return gen.RuntimeState{}, false, nil
	}
	if err != nil {
		return gen.RuntimeState{}, false, platform.NewError(platform.CodeDependencyUnavailable, true, "读取运行操作回执失败")
	}
	if storedKind != kind || storedHash != inputHash {
		return gen.RuntimeState{}, false, platform.NewError(platform.CodeConflict, false, "相同operationId对应不同请求")
	}
	var state gen.RuntimeState
	if err := json.Unmarshal([]byte(resultJSON), &state); err != nil {
		return gen.RuntimeState{}, false, platform.NewError(platform.CodeIntegrityError, false, "运行操作回执损坏")
	}
	if err := validateRuntimeStateShape(state); err != nil {
		return gen.RuntimeState{}, false, err
	}
	return state, true, nil
}

func (s *RuntimeStore) writeOperation(ctx context.Context, tx *sql.Tx, operationID, kind, inputHash string, state gen.RuntimeState) error {
	if operationID == "" {
		return nil
	}
	resultJSON, err := json.Marshal(state)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "编码运行操作回执失败")
	}
	_, err = tx.ExecContext(ctx, `
        INSERT INTO runtime_operation_receipts
            (operation_id, kind, input_hash, result_json, created_at)
		VALUES (?, ?, ?, ?, ?)`, string(operationID), kind, inputHash, string(resultJSON), string(platform.Now()))
	if err != nil {
		return platform.NewError(platform.CodeConflict, false, "保存运行操作回执失败")
	}
	return nil
}

func validateRequiredModules(modules []string) error {
	if len(modules) != len(requiredRuntimeModules) {
		return platform.NewError(platform.CodeIntegrityError, false, "requiredModules数量不符合14个有状态域")
	}
	for i, module := range modules {
		if module != requiredRuntimeModules[i] {
			return platform.NewError(platform.CodeIntegrityError, false, "requiredModules顺序或成员不合法")
		}
	}
	return nil
}

func isRequiredModule(module string) bool {
	for _, required := range requiredRuntimeModules {
		if required == module {
			return true
		}
	}
	return false
}

func validateRuntimeStateShape(state gen.RuntimeState) error {
	if !platform.ValidIdentifier(platform.Identifier(state.DataEpoch)) || state.Revision < 1 || len(state.RequiredModules) != len(requiredRuntimeModules) {
		return platform.NewError(platform.CodeIntegrityError, false, "运行状态不完整")
	}
	if err := validateRequiredModules(state.RequiredModules); err != nil {
		return err
	}
	switch state.Phase {
	case gen.Running, gen.BackupBarrier, gen.RestoreStaging, gen.RestoreSwitching, gen.RestoreRollingBack, gen.RestoreBlocked:
	default:
		return platform.NewError(platform.CodeIntegrityError, false, "运行phase不合法")
	}
	if state.Maintenance != (state.Phase != gen.Running) {
		return platform.NewError(platform.CodeIntegrityError, false, "运行phase与maintenance不一致")
	}
	if state.Phase == gen.Running {
		if state.TargetDataEpoch != nil || state.RestoreId != nil || state.BarrierId != nil {
			return platform.NewError(platform.CodeIntegrityError, false, "running状态包含恢复字段")
		}
		return nil
	}
	if state.BarrierId == nil || !platform.ValidIdentifier(*state.BarrierId) {
		return platform.NewError(platform.CodeIntegrityError, false, "维护状态缺少barrierId")
	}
	if state.Phase != gen.BackupBarrier {
		if state.RestoreId == nil || state.TargetDataEpoch == nil || !platform.ValidIdentifier(*state.RestoreId) || !platform.ValidIdentifier(*state.TargetDataEpoch) {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复状态缺少restoreId或targetDataEpoch")
		}
	}
	for _, receipt := range state.ActivationReceipts {
		if state.TargetDataEpoch == nil || receipt.TargetDataEpoch != *state.TargetDataEpoch {
			return platform.NewError(platform.CodeIntegrityError, false, "激活回执代次与运行状态不一致")
		}
	}
	return nil
}

func nullableIdentifier(value sql.NullString) *gen.Identifier {
	if !value.Valid {
		return nil
	}
	id := gen.Identifier(value.String)
	return &id
}

// runtimeIdentifier 仅适配platform内部RuntimeView的可选字符串；HTTP DTO保留null。
func runtimeIdentifier(value *gen.Identifier) platform.Identifier {
	if value == nil {
		return ""
	}
	return *value
}

func isMissingTable(err error) bool {
	return err != nil && (errors.Is(err, sql.ErrNoRows) ||
		containsSQLiteMessage(err, "no such table"))
}

func containsSQLiteMessage(err error, fragment string) bool {
	return err != nil && len(err.Error()) >= len(fragment) &&
		stringContains(err.Error(), fragment)
}

func stringContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
