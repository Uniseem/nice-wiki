package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	maxminddb "github.com/oschwald/maxminddb-golang"
	_ "modernc.org/sqlite"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/analytics/migrations"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Runtime只由可信启动装配；公开请求不能构造句柄、站点或密钥。
type Runtime struct {
	Consumer            platform.EventConsumerClient
	UsageGateway        *UsageGateway
	Jobs                platform.WorkerJobsClient
	Submitter           JobSubmitter
	ValidateCreateJob   func(json.RawMessage) error
	GeoIP               *maxminddb.Reader
	DB                  *sql.DB
	Root, Epoch, SiteID string
	Key                 platform.SigningKey
	Window              platform.WriteWindowConfig
	Authorize           func(context.Context, OperationContext, string, string) error
	ValidateEvent       func(string, int, json.RawMessage) error
	Publisher           platform.EventPublisher
	mu                  sync.Mutex
	flushMu             sync.Mutex
	exportMu            sync.Mutex
	queue               []beaconRow
	dropped             int64
	rateMinute          int64
	rateCount           int
	salts               map[string][]byte
	CountingLocation    *time.Location
	closeOnce           sync.Once
	closeErr            error
}
type analyticsRuntimeKey struct{}
type beaconRow struct {
	ID, DocumentID, Locale, EventType, Bucket, DayHash, Source, Country, At, Epoch string
	Status                                                                         int64
	QueuedAt                                                                       time.Time
}

const analyticsTimeLayout = "2006-01-02T15:04:05.000000000Z"

func WithRuntime(ctx context.Context, runtime *Runtime) context.Context {
	return context.WithValue(ctx, analyticsRuntimeKey{}, runtime)
}

// OpenStore连续事务迁移本域私有库；控制屏障不放active，盐不进业务快照。
func OpenStore(ctx context.Context, root, siteID, epoch string, key platform.SigningKey) (*Runtime, error) {
	for _, name := range []string{"active", "control", "restore", "active/exports"} {
		path := filepath.Join(root, name)
		if err := lifecycle.ValidatePrivatePath(path, "directory", true); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(path, 0700); err != nil {
			return nil, storageError(err)
		}
		if err := syncDirectory(path, root); err != nil {
			return nil, err
		}
	}
	path := filepath.Join(root, "active", "analytics.db")
	if err := lifecycle.ValidatePrivatePath(path, "regular-file", true); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: path}).String()+"?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)")
	if err != nil {
		return nil, storageError(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()
	if err = db.PingContext(ctx); err != nil {
		return nil, storageError(err)
	}
	var version int
	if err = db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return nil, storageError(err)
	}
	if version > 6 {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "统计schema高于当前版本")
	}
	if version == 0 {
		var tables int
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&tables); err != nil {
			return nil, storageError(err)
		}
		if tables != 0 {
			return nil, platform.NewError(platform.CodeLocked, false, "未知未版本化统计库须先明确验证迁移")
		}
		tx, txErr := db.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, storageError(txErr)
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, migrations.Schema); err != nil {
			return nil, storageError(err)
		}
		if _, err = tx.ExecContext(ctx, `PRAGMA user_version=1`); err != nil {
			return nil, storageError(err)
		}
		if err = tx.Commit(); err != nil {
			return nil, storageError(err)
		}
	}
	if version < 2 {
		tx, txErr := db.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, storageError(txErr)
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, migrations.OriginsAndPeriods); err != nil {
			return nil, storageError(err)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE analytics_meta SET schema_version=2 WHERE singleton=1`); err != nil {
			return nil, storageError(err)
		}
		if _, err = tx.ExecContext(ctx, `PRAGMA user_version=2`); err != nil {
			return nil, storageError(err)
		}
		if err = tx.Commit(); err != nil {
			return nil, storageError(err)
		}
	}
	if version < 3 {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return nil, storageError(e)
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, migrations.AggregateJobs); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `UPDATE analytics_meta SET schema_version=3 WHERE singleton=1`); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `PRAGMA user_version=3`); e != nil {
			return nil, storageError(e)
		}
		if e = tx.Commit(); e != nil {
			return nil, storageError(e)
		}
	}
	if version < 4 {
		var old int64
		if e := db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM qa_turn_counts WHERE count<>0)+(SELECT count(*) FROM unanswered_records)`).Scan(&old); e != nil {
			return nil, storageError(e)
		}
		if old != 0 {
			return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "旧问答汇总缺可证明的来源修订，须明确迁移")
		}
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return nil, storageError(e)
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, migrations.QaProjection); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `UPDATE analytics_meta SET schema_version=4 WHERE singleton=1`); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `PRAGMA user_version=4`); e != nil {
			return nil, storageError(e)
		}
		if e = tx.Commit(); e != nil {
			return nil, storageError(e)
		}
	}
	if version < 5 {
		var old int64
		if e := db.QueryRowContext(ctx, `SELECT count(*) FROM export_runs`).Scan(&old); e != nil {
			return nil, storageError(e)
		}
		if old != 0 {
			return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "旧导出缺类型主体代次与journal，须明确迁移")
		}
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return nil, storageError(e)
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, migrations.ExportJournal); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `UPDATE analytics_meta SET schema_version=5 WHERE singleton=1`); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `PRAGMA user_version=5`); e != nil {
			return nil, storageError(e)
		}
		if e = tx.Commit(); e != nil {
			return nil, storageError(e)
		}
	}
	if version < 6 {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return nil, storageError(e)
		}
		defer tx.Rollback()
		var old int64
		if e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM usage_projection)+(SELECT count(*) FROM daily_usage)+(SELECT count(*) FROM aggregation_watermarks WHERE source LIKE 'usage%')`).Scan(&old); e != nil {
			return nil, storageError(e)
		}
		if old != 0 {
			return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "旧费用投影缺完整V2权威来源，须先明确隔离或迁移，不静默升级")
		}
		if _, e = tx.ExecContext(ctx, migrations.UsageStatisticsV2); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `UPDATE analytics_meta SET schema_version=6 WHERE singleton=1`); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `PRAGMA user_version=6`); e != nil {
			return nil, storageError(e)
		}
		if e = tx.Commit(); e != nil {
			return nil, storageError(e)
		}
	}
	if _, err = db.ExecContext(ctx, `INSERT OR IGNORE INTO analytics_meta(singleton,schema_version,site_id,data_epoch) VALUES(1,6,?,?)`, siteID, epoch); err != nil {
		return nil, storageError(err)
	}
	var storedSite, storedEpoch string
	if err = db.QueryRowContext(ctx, `SELECT schema_version,site_id,data_epoch FROM analytics_meta WHERE singleton=1`).Scan(&version, &storedSite, &storedEpoch); err != nil {
		return nil, storageError(err)
	}
	if version != 6 {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "统计schema标记未知")
	}
	if storedSite != siteID || storedEpoch != epoch {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "统计站点或代次与运行权威不符")
	}
	if err = inbox.EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	if err = outbox.EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	geoPath := filepath.Join(root, "control", "geoip.mmdb")
	var geo *maxminddb.Reader
	if _, e := os.Lstat(geoPath); e == nil {
		if err = lifecycle.ValidatePrivatePath(geoPath, "regular-file", false); err != nil {
			return nil, err
		}
		geo, err = maxminddb.Open(geoPath)
		if err != nil {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "离线国家数据库损坏")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, storageError(e)
	}
	success = true
	return &Runtime{GeoIP: geo, DB: db, Root: root, Epoch: epoch, SiteID: siteID, Key: key, salts: map[string][]byte{}, queue: make([]beaconRow, 0, 4096)}, nil
}
func (rt *Runtime) Close() error {
	rt.closeOnce.Do(func() {
		rt.closeErr = rt.DB.Close()
		if rt.GeoIP != nil {
			if e := rt.GeoIP.Close(); rt.closeErr == nil {
				rt.closeErr = e
			}
		}
		rt.mu.Lock()
		for _, salt := range rt.salts {
			clear(salt)
		}
		rt.salts = nil
		rt.mu.Unlock()
	})
	return storageError(rt.closeErr)
}
func runtimeFrom(ctx context.Context, op OperationContext) (*Runtime, error) {
	rt, ok := ctx.Value(analyticsRuntimeKey{}).(*Runtime)
	if !ok || rt == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "统计私有存储未装配")
	}
	if rt.Epoch != op.DataEpoch || rt.SiteID != op.Actor.SiteID {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "统计站点或代次不一致")
	}
	return rt, nil
}
func storageError(err error) error {
	if err == nil {
		return nil
	}
	return platform.NormalizeIOError(err, platform.CodeInternal, false, "统计私有存储失败")
}
func syncDirectory(path, root string) error {
	for {
		f, err := os.Open(path)
		if err != nil {
			return storageError(err)
		}
		err = f.Sync()
		_ = f.Close()
		if err != nil {
			return storageError(err)
		}
		if path == root {
			return nil
		}
		next := filepath.Dir(path)
		if next == path {
			return platform.NewError(platform.CodeIntegrityError, false, "统计同步路径超出私有根")
		}
		path = next
	}
}

// beginWrite复用平台当前窗口，缺安全机制不假装可写；维护draining窗口由公共Manager替换装配。
func beginWrite(ctx context.Context, op OperationContext) (context.Context, *Runtime, *sql.Tx, context.CancelFunc, error) {
	rt, err := runtimeFrom(ctx, op)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if rt.Window.Runtime == nil || rt.Window.WriteWindow == nil {
		return nil, nil, nil, nil, platform.NewError(platform.CodeDependencyUnavailable, true, "统计安全写窗口未装配")
	}
	work, view, cancel, err := lifecycle.NewWriteContext(platform.WithOperationContext(ctx, op), rt.Window)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if view.DataEpoch != rt.Epoch {
		cancel()
		return nil, nil, nil, nil, platform.NewError(platform.CodeVersionMismatch, false, "统计写入代次已变化")
	}
	tx, err := rt.DB.BeginTx(work, nil)
	if err != nil {
		cancel()
		return nil, nil, nil, nil, storageError(err)
	}
	return work, rt, tx, cancel, nil
}
func advanceWatermark(ctx context.Context, tx *sql.Tx, source string) (int64, error) {
	var revision int64
	if err := tx.QueryRowContext(ctx, `UPDATE analytics_meta SET revision=revision+1 WHERE singleton=1 AND revision<9223372036854775807 RETURNING revision`).Scan(&revision); err != nil {
		return 0, storageError(err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO aggregation_watermarks(source,revision,updated_at) VALUES(?,?,?) ON CONFLICT(source) DO UPDATE SET revision=excluded.revision,updated_at=excluded.updated_at`, source, revision, time.Now().UTC().Format(analyticsTimeLayout))
	return revision, storageError(err)
}
