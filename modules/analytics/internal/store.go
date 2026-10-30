package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	maxminddb "github.com/oschwald/maxminddb-golang"
	"net/url"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/inbox"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/pgdb"
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

	// 多站点：基础运行态（平台站点）之外，每个站点有一份共用存储、各自信标队列与日盐的运行态。
	base          *Runtime
	sitesMu       sync.Mutex
	perSite       map[string]*Runtime
	reportingZone string
}
type analyticsRuntimeKey struct{}
type beaconRow struct {
	VisitorID, IP, UserAgent                                                       string
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
	for _, name := range []string{"active", "control", "active/exports"} {
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
	if version > 8 {
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
	if version < 7 {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return nil, storageError(e)
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, migrations.Visitors); e != nil {
			return nil, storageError(e)
		}
		if e = tx.Commit(); e != nil {
			return nil, storageError(e)
		}
	}
	if version < 8 {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return nil, storageError(e)
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, migrations.SiteKeys); e != nil {
			return nil, storageError(e)
		}
		if _, e = tx.ExecContext(ctx, `PRAGMA user_version=8`); e != nil {
			return nil, storageError(e)
		}
		if e = tx.Commit(); e != nil {
			return nil, storageError(e)
		}
	}
	// 启动只认模板行（站点行由租户隔离层按需复制，读到哪一行不确定）。
	metaCtx := pgdb.WithTemplateView(ctx)
	if _, err = db.ExecContext(metaCtx, `INSERT OR IGNORE INTO analytics_meta(singleton,schema_version,site_id,data_epoch) VALUES(1,7,?,?)`, siteID, epoch); err != nil {
		return nil, storageError(err)
	}
	var storedSite, storedEpoch string
	if err = db.QueryRowContext(metaCtx, `SELECT schema_version,site_id,data_epoch FROM analytics_meta WHERE singleton=1`).Scan(&version, &storedSite, &storedEpoch); err != nil {
		return nil, storageError(err)
	}
	if version != 7 {
		return nil, platform.NewError(platform.CodeUnsupportedSchema, false, "统计schema标记未知")
	}
	if storedSite != siteID || storedEpoch != epoch {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "统计站点或代次与运行权威不符")
	}
	if err = inbox.EnsureSchema(ctx, db); err != nil {
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
	if rt.Epoch != op.DataEpoch {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "统计站点或代次不一致")
	}
	if rt.SiteID == op.Actor.SiteID {
		return rt, nil
	}
	return rt.ForSite(ctx, op.Actor.SiteID)
}

// ForSite 返回某个站点的运行态：和基础运行态共用存储与后台配置，但信标队列、限速计数、日盐各自独立；
// 第一次用到时绑定该站点的报表时区（站点库里的统计记录由租户隔离层按站点分开）。
func (rt *Runtime) ForSite(ctx context.Context, site string) (*Runtime, error) {
	base := rt
	if rt.base != nil {
		base = rt.base
	}
	if site == base.SiteID {
		return base, nil
	}
	if !platform.ValidIdentifier(site) {
		return nil, platform.NewError(platform.CodeVersionMismatch, false, "统计站点或代次不一致")
	}
	base.sitesMu.Lock()
	existing := base.perSite[site]
	base.sitesMu.Unlock()
	if existing != nil {
		return existing, nil
	}
	clone := &Runtime{base: base, Consumer: base.Consumer, Jobs: base.Jobs, Submitter: base.Submitter, ValidateCreateJob: base.ValidateCreateJob,
		GeoIP: base.GeoIP, DB: base.DB, Root: base.Root, Epoch: base.Epoch, SiteID: site, Key: base.Key, Window: base.Window,
		Authorize: base.Authorize, ValidateEvent: base.ValidateEvent, CountingLocation: base.CountingLocation,
		reportingZone: base.reportingZone, salts: map[string][]byte{}, queue: make([]beaconRow, 0, 256)}
	// 费用统计只对平台站点同步（模型网关的账本按平台站点登记）；站点主人的 AI 用量走 identity 的 readAiUsage。
	if base.reportingZone != "" && base.Window.Runtime != nil {
		view, err := base.Window.Runtime(ctx)
		if err != nil {
			return nil, err
		}
		if err = BindReportingTimeZone(pgdb.WithSite(ctx, site), clone, base.reportingZone, view); err != nil {
			return nil, err
		}
	}
	base.sitesMu.Lock()
	defer base.sitesMu.Unlock()
	if existing = base.perSite[site]; existing != nil {
		return existing, nil
	}
	if base.perSite == nil {
		base.perSite = map[string]*Runtime{}
	}
	base.perSite[site] = clone
	return clone, nil
}

// openSites 只列内存里已有的站点运行态（不碰数据库），停机排空队列用。
func (rt *Runtime) openSites() []*Runtime {
	base := rt
	if rt.base != nil {
		base = rt.base
	}
	result := []*Runtime{base}
	base.sitesMu.Lock()
	defer base.sitesMu.Unlock()
	for _, site := range base.perSite {
		result = append(result, site)
	}
	return result
}

// Sites 列出要处理的站点运行态：基础站点、已经出现过请求的站点，以及库里有统计记录的站点。
func (rt *Runtime) Sites(ctx context.Context) []*Runtime {
	base := rt
	if rt.base != nil {
		base = rt.base
	}
	inactive, _ := pgdb.InactiveSites(ctx, base.DB)
	var result []*Runtime
	if !inactive[base.SiteID] {
		result = append(result, base)
	}
	seen := map[string]bool{base.SiteID: true}
	known, _ := pgdb.Sites(ctx, base.DB)
	base.sitesMu.Lock()
	for site := range base.perSite {
		known = append(known, site)
	}
	base.sitesMu.Unlock()
	for _, site := range known {
		if seen[site] || inactive[site] || !platform.ValidIdentifier(site) {
			continue
		}
		seen[site] = true
		if clone, err := base.ForSite(ctx, site); err == nil {
			result = append(result, clone)
		}
	}
	return result
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
