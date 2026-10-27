// Package runtime 负责 model-gateway 本域私有数据库的启动装配。
// 这里不调用供应方，也不把主密钥或其内容写入数据库、日志或结果。
package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"nice-wiki/libs/platform/lifecycle"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "nice-wiki/libs/platform/pgdb"

	"nice-wiki/libs/platform"
	gatewaymigrations "nice-wiki/modules/model-gateway/migrations"
)

const databaseName = "model-gateway.db"

// OperationContext 是服务间可信操作上下文的本域别名。
type OperationContext = platform.OperationContext

// LoadRuntimeInput 是启动时由部署装配注入的私有目录和主密钥引用。
type LoadRuntimeInput struct {
	ServiceID     string `json:"serviceId"`
	MasterKeyRef  string `json:"masterKeyRef,omitempty"`
	DataDirectory string `json:"dataDirectory"`
}

// LoadRuntimeResult 是启动持久化检查的脱敏结果。
type LoadRuntimeResult struct {
	Ready          bool  `json:"ready"`
	CatalogVersion int64 `json:"catalogVersion"`
}

// Store 是本域 active 数据库句柄。所有业务域写入都应在这个数据库内
// 完成，不能把它作为HTTP DTO或跨域共享。
type Store struct {
	db             *sql.DB
	dataDirectory  string
	masterKeyRef   string
	catalogVersion int64
	mu             sync.RWMutex
	closed         bool
	execution      ExecutionOptions
}

var activeStore struct {
	sync.RWMutex
	store *Store
}

// LoadRuntime 只建立本域私有持久化和迁移；不探测模型、不发起供应方请求。
// 可选主密钥只装配可信引用，secret仅在凭据写入时读取。
func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error) {
	if err := contextError(ctx); err != nil {
		return LoadRuntimeResult{}, err
	}
	if input.ServiceID != "model-gateway" || input.DataDirectory == "" {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "model-gateway运行参数不完整")
	}
	if (input.MasterKeyRef != "" && !filepath.IsAbs(input.MasterKeyRef)) || !filepath.IsAbs(input.DataDirectory) {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "model-gateway路径必须为绝对路径")
	}
	if !platform.ValidIdentifier(op.DataEpoch) && op.DataEpoch != "" {
		return LoadRuntimeResult{}, platform.NewError(platform.CodeValidation, false, "运行代次不合法")
	}
	store, err := OpenDatabase(input.DataDirectory)
	if err != nil {
		return LoadRuntimeResult{}, err
	}
	InstallStore(store, input.MasterKeyRef)
	return LoadRuntimeResult{Ready: true, CatalogVersion: store.catalogVersion}, nil
}

// OpenDatabase 打开本域 active/model-gateway.db，启用WAL、外键和短忙等候，
// 并在同一连接上执行严格的初始迁移。control目录不随业务快照替换。
func OpenDatabase(dataDirectory string) (*Store, error) {
	if dataDirectory == "" || !filepath.IsAbs(dataDirectory) {
		return nil, platform.NewError(platform.CodeValidation, false, "模型网关数据目录必须为绝对路径")
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(dataDirectory))
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "模型网关数据目录不合法")
	}
	for _, child := range []string{"active", "control"} {
		if err := os.MkdirAll(filepath.Join(cleanRoot, child), 0o700); err != nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据目录不可用")
		}
	}

	entries, e := os.ReadDir(filepath.Join(cleanRoot, "active"))
	if e != nil {
		return nil, mapDatabaseError(e)
	}
	exists := false
	for _, entry := range entries {
		if entry.Name() == databaseName {
			exists = true
		}
		if (entry.Name() != databaseName && entry.Name() != databaseName+"-wal" && entry.Name() != databaseName+"-shm") || entry.IsDir() || !entry.Type().IsRegular() {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "模型active存在未知文件，拒绝创建新权威覆盖历史")
		}
	}
	if !exists && len(entries) > 0 {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "缺主库但存在旧WAL材料，不能宣称新建空历史")
	}
	birthPath := filepath.Join(cleanRoot, "control", "database-origin.json")
	if !exists {
		if _, e = os.Lstat(birthPath); e == nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "原控制区已记录库创建但active库丢失，禁止重建空历史")
		} else if !errors.Is(e, os.ErrNotExist) {
			return nil, mapDatabaseError(e)
		}
	}
	databasePath := filepath.Join(cleanRoot, "active", databaseName)
	if err = lifecycle.ValidatePrivatePath(databasePath, "regular-file", true); err != nil {
		return nil, err
	}
	created, createErr := os.OpenFile(databasePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	fresh := createErr == nil
	if createErr != nil && !errors.Is(createErr, os.ErrExist) {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "建立模型网关私有库失败")
	}
	if fresh {
		if err = created.Sync(); err != nil {
			_ = created.Close()
			return nil, mapDatabaseError(err)
		}
		_ = created.Close()
		directory, e := os.Open(filepath.Dir(databasePath))
		if e != nil {
			return nil, mapDatabaseError(e)
		}
		e = directory.Sync()
		_ = directory.Close()
		if e != nil {
			return nil, mapDatabaseError(e)
		}
	}
	dsn := "file:" + filepath.Join(cleanRoot, "active", databaseName) +
		"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开模型网关数据库失败")
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, dataDirectory: cleanRoot}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "模型网关数据库不可用")
	}
	if err := store.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if fresh {
		proof, _ := platform.CanonicalJSON(struct{ Format, Owner, InstanceID, CreatedAt string }{"nice-wiki.gateway-birth.v1", "model-gateway", platform.NewIdentifier(), time.Now().UTC().Format(time.RFC3339Nano)})
		certificate, e := os.OpenFile(birthPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			_ = db.Close()
			return nil, platform.NewError(platform.CodeIntegrityError, false, "库创建证书冲突或缺可写控制区，不能证明新历史")
		}
		_, e = certificate.Write(proof)
		if e == nil {
			e = certificate.Sync()
		}
		_ = certificate.Close()
		if e == nil {
			directory, openErr := os.Open(filepath.Dir(birthPath))
			if openErr != nil {
				e = openErr
			} else {
				e = directory.Sync()
				_ = directory.Close()
			}
		}
		if e != nil {
			_ = db.Close()
			return nil, mapDatabaseError(e)
		}
		if _, err = store.db.Exec(`INSERT INTO database_origin(singleton,proof_json,proof_hash) VALUES(1,?,?)`, proof, platform.HashBytes(proof)); err != nil {
			_ = db.Close()
			return nil, mapDatabaseError(err)
		}
	}

	return store, nil
}

// CurrentStore 返回当前进程安装的本域数据库；调用方不得持有其内部句柄
// 越过本域事务边界。
func CurrentStore() *Store {
	activeStore.RLock()
	defer activeStore.RUnlock()
	return activeStore.store
}

// InstallStore 仅供可信启动装配安装已打开的本域库；不读取主密钥或写代次。
func InstallStore(store *Store, masterKeyRef string) {
	store.masterKeyRef = masterKeyRef
	activeStore.Lock()
	activeStore.store = store
	activeStore.Unlock()
}

// BeginTx 在数据库关闭或上下文取消时拒绝启动新的本域事务。
func (s *Store) BeginTx(ctx context.Context) (*sql.Tx, error) {
	if s == nil || ctx == nil {
		return nil, platform.NewError(platform.CodeValidation, false, "模型网关事务参数不完整")
	}
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "模型网关数据库已关闭")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, mapDatabaseError(err)
	}
	return tx, nil
}

// DB 只供本域子包执行只读查询或事务装配，不得传给其他服务。
func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// SetDataEpoch 在应用装配读取site-config权威后写入本域水位；写业务前由
// 子包比较该水位，避免旧代次继续修改模型目录。
func (s *Store) SetDataEpoch(ctx context.Context, dataEpoch platform.Identifier) error {
	if s == nil || !platform.ValidIdentifier(dataEpoch) {
		return platform.NewError(platform.CodeValidation, false, "模型网关数据代次不合法")
	}
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	if err = tx.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton=1`).Scan(&current); err != nil {
		return mapDatabaseError(err)
	}
	if current != "" && current != dataEpoch {
		return platform.NewError(platform.CodeStaleResult, false, "启动不能重基既有业务代际，必须经过验证的恢复迁移")
	}
	if current == "" {
		var raw []byte
		var hash string
		if err = tx.QueryRowContext(ctx, `SELECT proof_json,proof_hash FROM database_origin WHERE singleton=1`).Scan(&raw, &hash); err == sql.ErrNoRows {
			return platform.NewError(platform.CodeDependencyUnavailable, false, "旧库缺O_EXCL真实创建来源，不得由空表推首次接受证明")
		} else if err != nil {
			return mapDatabaseError(err)
		}
		var proof struct{ Format, Owner, InstanceID, CreatedAt string }
		if platform.HashBytes(raw) != hash || json.Unmarshal(raw, &proof) != nil || proof.Format != "nice-wiki.gateway-birth.v1" || proof.Owner != "model-gateway" || !platform.ValidIdentifier(proof.InstanceID) {
			return platform.NewError(platform.CodeIntegrityError, false, "真实库创建来源损坏")
		}
		binding, _ := platform.CanonicalJSON(struct{ OriginHash, DataEpoch string }{hash, dataEpoch})
		if _, err = tx.ExecContext(ctx, `INSERT INTO invocation_lookup_tracking(singleton,format_version,data_epoch,proof_kind,proof_hash,created_at) VALUES(1,1,?,'fresh_activation',?,?)`, dataEpoch, platform.HashBytes(binding), platform.Now()); err != nil {
			return mapDatabaseError(err)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE credential_tracking SET enabled=1 WHERE singleton=1`); err != nil {
			return mapDatabaseError(err)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE runtime_meta SET data_epoch=?,updated_at=? WHERE singleton=1`, dataEpoch, platform.Now()); err != nil {
			return mapDatabaseError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return mapDatabaseError(err)
	}
	return nil
}

// CheckDataEpoch 拒绝已知旧代次；初次启动尚未从site-config写入水位时，
// 由应用装配在首次业务写入前完成SetDataEpoch。
func (s *Store) CheckDataEpoch(ctx context.Context, dataEpoch platform.Identifier) error {
	if s == nil || !platform.ValidIdentifier(dataEpoch) {
		return platform.NewError(platform.CodeVersionMismatch, false, "模型网关数据代次不合法")
	}
	var current string
	if err := s.db.QueryRowContext(ctx, `SELECT data_epoch FROM runtime_meta WHERE singleton = 1`).Scan(&current); err != nil {
		return mapDatabaseError(err)
	}
	if current != "" && current != string(dataEpoch) {
		return platform.NewError(platform.CodeVersionMismatch, false, "模型网关数据代次已变化")
	}
	return nil
}

// Close 关闭数据库并撤销进程内安装；重复关闭安全。
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
	activeStore.Lock()
	if activeStore.store == s {
		activeStore.store = nil
	}
	activeStore.Unlock()
	if err := s.db.Close(); err != nil {
		return platform.NewError(platform.CodeInternal, false, "关闭模型网关数据库失败")
	}
	return nil
}

// runMigrations 按编号执行 migrations/ 内嵌的只追加迁移（首个迁移自建
// schema_migrations），并初始化运行水位单例；跨服务事件表由平台建立。
func (s *Store) runMigrations() error {
	all, err := gatewaymigrations.All()
	if err != nil {
		return (platform.NewError(platform.CodeIntegrityError, false, "模型网关迁移集不合法"))
	}
	applied := map[int64]string{}
	rows, err := s.db.Query(`SELECT version, name FROM schema_migrations`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return (platform.NewError(platform.CodeIntegrityError, false, "读取模型网关迁移记录失败"))
		}
	} else {
		for rows.Next() {
			var version int64
			var name string
			if err := rows.Scan(&version, &name); err != nil {
				_ = rows.Close()
				return (platform.NewError(platform.CodeIntegrityError, false, "模型网关迁移记录损坏"))
			}
			applied[version] = name
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return (platform.NewError(platform.CodeIntegrityError, false, "读取模型网关迁移记录失败"))
		}
		_ = rows.Close()
	}
	for version, name := range applied {
		if version < 1 || version > int64(len(all)) || all[version-1].Name != name {
			return (platform.NewError(platform.CodeIntegrityError, false, "模型网关迁移版本未知"))
		}
	}
	for _, migration := range all {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return (platform.NewError(platform.CodeInternal, true, "开始模型网关迁移事务失败"))
		}
		if _, err := tx.Exec(migration.SQL); err != nil {
			_ = tx.Rollback()
			return (platform.NewError(platform.CodeIntegrityError, false, "执行模型网关迁移失败"))
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			migration.Version, migration.Name, string(platform.Now())); err != nil {
			_ = tx.Rollback()
			return (platform.NewError(platform.CodeInternal, false, "登记模型网关迁移失败"))
		}
		if err := tx.Commit(); err != nil {
			return (platform.NewError(platform.CodeInternal, false, "提交模型网关迁移失败"))
		}
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO runtime_meta (singleton, data_epoch, updated_at) VALUES (1, '', ?)`, string(platform.Now())); err != nil {
		return platform.NewError(platform.CodeInternal, false, "初始化模型网关运行水位失败")
	}
	// 跨服务事件只写平台 nw_outbox，由平台转发循环经 jobs 投递（ADR0005）。
	s.catalogVersion = int64(len(all))
	return nil
}

// MasterKeyRef只返回可信启动保存的引用，不读取secret。
func (s *Store) MasterKeyRef() string { return s.masterKeyRef }

func contextError(ctx context.Context) error {
	if ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "模型网关上下文为空")
	}
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return platform.NewError(platform.CodeDeadlineExceeded, true, "模型网关启动已超过期限")
		}
		return platform.NewError(platform.CodeCancelled, true, "模型网关启动已取消")
	default:
		return nil
	}
}

func mapDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return platform.NewError(platform.CodeCancelled, true, "模型网关数据库操作已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "模型网关数据库操作已超过期限")
	}
	return platform.NewError(platform.CodeInternal, true, "模型网关数据库操作失败")
}
