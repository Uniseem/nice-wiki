package index

import (
	"context"
	"database/sql"
	"encoding/json"
	_ "modernc.org/sqlite"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/knowledge/internal/gen"
	"nice-wiki/modules/knowledge/migrations"
	"os"
	"path/filepath"
	"sort"
)

// OpenRepository 只打开本域active；代次不匹配不重标旧库。由启动方完成恢复换目录。
func OpenRepository(ctx context.Context, dataRoot, site, epoch string) (*sql.DB, error) {
	if !filepath.IsAbs(dataRoot) || site == "" || epoch == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "索引目录或可信身份未建立")
	}
	active := filepath.Join(dataRoot, "active")
	if err := os.MkdirAll(active, 0700); err != nil {
		return nil, storageError(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(active, "knowledge.db")+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)")
	if err != nil {
		return nil, storageError(err)
	}
	db.SetMaxOpenConns(1)
	closeFailure := func(err error) (*sql.DB, error) { _ = db.Close(); return nil, err }
	if err = db.PingContext(ctx); err != nil {
		return closeFailure(storageError(err))
	}
	if _, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(name TEXT PRIMARY KEY)`); err != nil {
		return closeFailure(storageError(err))
	}
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return closeFailure(storageError(err))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		var present int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE name=?`, entry.Name()).Scan(&present); err != nil {
			return closeFailure(storageError(err))
		}
		if present != 0 {
			continue
		}
		raw, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return closeFailure(storageError(err))
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return closeFailure(storageError(err))
		}
		if _, err = tx.ExecContext(ctx, string(raw)); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(name) VALUES(?)`, entry.Name())
		}
		if err != nil {
			_ = tx.Rollback()
			return closeFailure(storageError(err))
		}
		if err = tx.Commit(); err != nil {
			return closeFailure(storageError(err))
		}
	}
	if err = outbox.EnsureSchema(ctx, db); err != nil {
		return closeFailure(err)
	}
	if err = inbox.EnsureSchema(ctx, db); err != nil {
		return closeFailure(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO index_identity(singleton,site_id,data_epoch) VALUES(1,?,?) ON CONFLICT(singleton) DO NOTHING`, site, epoch); err != nil {
		return closeFailure(storageError(err))
	}
	var storedSite, storedEpoch string
	if err = db.QueryRowContext(ctx, `SELECT site_id,data_epoch FROM index_identity WHERE singleton=1`).Scan(&storedSite, &storedEpoch); err != nil {
		return closeFailure(storageError(err))
	}
	if storedSite != site || storedEpoch != epoch {
		return closeFailure(platform.NewError(platform.CodeVersionMismatch, false, "索引存储代次或站点不匹配，需完成恢复装配"))
	}
	return db, nil
}

// BeginIndexWrite 在SQL事务前取得平台当前安全窗口；私有身份不可从请求覆盖。
func BeginIndexWrite(ctx context.Context, op OperationContext) (context.Context, *sql.Tx, context.CancelFunc, error) {
	runtime, err := RuntimeFromContext(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), runtime.WriteWindow)
	if err != nil {
		return nil, nil, nil, err
	}
	if view.DataEpoch != op.DataEpoch {
		cancel()
		return nil, nil, nil, platform.NewError(platform.CodeStaleResult, false, "索引写入代次已变化")
	}
	tx, err := runtime.DB.BeginTx(work, nil)
	if err != nil {
		cancel()
		return nil, nil, nil, storageError(err)
	}
	var site, epoch string
	if err = tx.QueryRowContext(work, `SELECT site_id,data_epoch FROM index_identity WHERE singleton=1`).Scan(&site, &epoch); err != nil {
		_ = tx.Rollback()
		cancel()
		return nil, nil, nil, storageError(err)
	}
	if site != op.Actor.SiteID || epoch != op.DataEpoch {
		_ = tx.Rollback()
		cancel()
		return nil, nil, nil, platform.NewError(platform.CodeStaleResult, false, "索引存储身份已变化")
	}
	return work, tx, cancel, nil
}

// ReadConfig 只读私有配置，不捏造未配置模型或空间。
func ReadConfig(ctx context.Context, tx *sql.Tx, op OperationContext) (gen.IndexConfig, error) {
	var raw []byte
	var epoch string
	err := tx.QueryRowContext(ctx, `SELECT config_json,data_epoch FROM index_config WHERE site_id=?`, op.Actor.SiteID).Scan(&raw, &epoch)
	if err == sql.ErrNoRows {
		return gen.IndexConfig{}, platform.NewError(platform.CodeNotFound, false, "尚未配置入库模型与空间")
	}
	if err != nil {
		return gen.IndexConfig{}, storageError(err)
	}
	if epoch != op.DataEpoch {
		return gen.IndexConfig{}, platform.NewError(platform.CodeStaleResult, false, "索引配置代次已变化")
	}
	var config gen.IndexConfig
	if json.Unmarshal(raw, &config) != nil {
		return gen.IndexConfig{}, platform.NewError(platform.CodeIntegrityError, false, "索引配置损坏")
	}
	return config, nil
}

func ReadOperationReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, function, key string, input any, result any) (bool, error) {
	raw, err := platform.CanonicalJSON(input)
	if err != nil {
		return false, err
	}
	var hash string
	var stored []byte
	err = tx.QueryRowContext(ctx, `SELECT input_hash,result_json FROM operation_receipts WHERE site_id=? AND data_epoch=? AND actor_id=? AND function_id=? AND idempotency_key=?`, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, function, key).Scan(&hash, &stored)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, storageError(err)
	}
	if hash != platform.HashBytes(raw) {
		return false, platform.NewError(platform.CodeConflict, false, "相同幂等键已绑定其他输入")
	}
	if json.Unmarshal(stored, result) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "索引原操作回执损坏")
	}
	return true, nil
}
func SaveOperationReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, function, key string, input, result any) error {
	raw, err := platform.CanonicalJSON(input)
	if err != nil {
		return err
	}
	stored, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_receipts(site_id,data_epoch,actor_id,function_id,idempotency_key,input_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?,?)`, op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, function, key, platform.HashBytes(raw), stored, platform.Now())
	if err != nil {
		return storageError(err)
	}
	return nil
}

// EnsureBuildDiskBudget 实施本域已确认10%低水位，只阻止新构建，不影响原结果复用。
func EnsureBuildDiskBudget(dataRoot string) error {
	if os.Getenv("NW_DEV_NO_DISK_WATERMARK") != "" { // 只给磁盘本来就紧的开发机用
		return nil
	}
	if !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "索引私有目录尚未可信装配")
	}
	available, total, err := lifecycle.ReadFilesystemSpace(dataRoot)
	if err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "索引磁盘水位暂不可读")
	}
	if total == 0 || available <= total/10 {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "索引磁盘余量低于构建安全水位，保留现有活动索引")
	}
	return nil
}
