// 装配问答域：私有 SQLite、生产方客户端与已实现的 HTTP 处理函数。
package app

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

	"github.com/getkin/kin-openapi/openapi3"
	_ "modernc.org/sqlite"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/transport"
	"nice-wiki/modules/qa/migrations"
)

// ModuleSlug 是本域固定的服务身份。
const ModuleSlug = "qa"

var identityOnce sync.Once

// BuildApplication 返回可运行的 Application。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*Application, error) {
	if config.Module != ModuleSlug || !filepath.IsAbs(config.DataRoot) || config.Listen == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "本域配置不合法")
	}
	for _, name := range []string{"active", "control", "restore"} {
		if err := os.MkdirAll(filepath.Join(config.DataRoot, name), 0o700); err != nil {
			return nil, platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}
	anchors, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	runtimeAuthority, err := lifecycle.NewSiteConfigClient(config, key)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		return nil, err
	}
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执尚未装配")
	}
	db, err := openDatabase(config.DataRoot)
	if err != nil {
		return nil, err
	}
	retain := false
	defer func() {
		if !retain {
			_ = db.Close()
		}
	}()
	baseLanguage := "zh-Hans"
	window := platform.WriteWindowConfig{
		Runtime: func(work context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(work, runtimeAuthority)
		},
		WriteWindow: func(work context.Context) (time.Time, error) {
			current, readErr := lifecycle.ReadRuntime(work, runtimeAuthority)
			if readErr != nil {
				return time.Time{}, readErr
			}
			if current.Maintenance {
				return time.Time{}, platform.NewError(platform.CodeLocked, false, "本域维护写窗口关闭")
			}
			return time.Time{}, nil
		},
	}
	dep := func(name string) string { return config.Dependencies[name] }
	runtime, err := qaruntime.NewRuntime(qaruntime.Runtime{
		DB: db, WriteWindow: window, SigningKey: key,
		Guard:        gen.NewTrafficGuardClient(dep("traffic-guard"), ModuleSlug, key),
		Content:      gen.NewContentClient(dep("content"), ModuleSlug, key),
		PublicSite:   gen.NewPublicSiteClient(dep("public-site"), ModuleSlug, key),
		Knowledge:    gen.NewKnowledgeClient(dep("knowledge"), ModuleSlug, key),
		Memory:       gen.NewMemoryClient(dep("memory"), ModuleSlug, key),
		Localization: gen.NewLocalizationClient(dep("localization"), ModuleSlug, key),
		Models:       gen.NewModelGatewayClient(dep("model-gateway"), ModuleSlug, key),
		Jobs:         gen.NewJobsClient(dep("jobs"), ModuleSlug, key),
		BaseLanguage: baseLanguage, AnswerContextTokens: 4096, PreflightContextTokens: 2048,
	})
	if err != nil {
		return nil, err
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	identity := gen.NewIdentityClient(dep("identity"), ModuleSlug, key)
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self: ModuleSlug, Anchors: anchors, Runtime: window.Runtime, OpenedDataEpoch: view.DataEpoch,
		Authorize:      apivalidate.NewIdentityAuthorizer(identity),
		AllowedCallers: callers(spec),
		ServiceActorCallers: map[string][]string{
			"readPublicCapability": {"portal", "public-site", "site-config"}, "readCacheTranslationSource": {"localization"},
		},
		MaintenanceOperations: map[string]bool{"getConversation": true, "readPublicCapability": true},
	}, transport.NewOperationHandlers(func(c context.Context) context.Context {
		// 本域私有存储的站点身份取自首个带真实站点的请求，之后不再变化。
		if op, ok := platform.OperationContextFrom(c); ok && op.Actor.SiteID != "" && op.Actor.SiteID != "system" {
			identityOnce.Do(func() { _ = ensureIdentity(c, db, op.Actor.SiteID, view.DataEpoch) })
		}
		return qaruntime.WithRuntime(c, runtime)
	}))
	if err != nil {
		return nil, err
	}
	paths := map[string][]string{}
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			paths[path] = append(paths[path], method)
		}
	}
	application := NewApplication(config, anchors, runtimeAuthority, view.DataEpoch)
	retain = true
	application.Mount(handler, paths, func() { _ = db.Close() })
	return application, nil
}

// callers 为规格里每个业务接口指定调用方：访客与后台操作都经 portal，其余服务接口固定到对应服务。
func callers(spec *openapi3.T) map[string][]string {
	special := map[string][]string{
		"acquireBarrier": {"data-protection"}, "releaseBarrier": {"data-protection"}, "exportSnapshot": {"data-protection"},
		"stageSnapshot": {"data-protection"}, "discardSnapshot": {"data-protection"}, "readSnapshotArchive": {"data-protection"},
		"readPublicCapability": {"portal", "public-site", "site-config"}, "readCacheTranslationSource": {"localization"},
	}
	result := map[string][]string{}
	for _, item := range spec.Paths.Map() {
		for _, operation := range item.Operations() {
			id := strings.ToLower(operation.OperationID[:1]) + operation.OperationID[1:]
			if id == "health" || id == "ready" {
				continue
			}
			if who, ok := special[id]; ok {
				result[id] = who
			} else {
				result[id] = []string{"portal"}
			}
		}
	}
	return result
}

// NewRuntimeClient 构造 site-config 运行权威客户端。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	_, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

func openDatabase(dataRoot string) (*sql.DB, error) {
	dsn := "file:" + filepath.Join(dataRoot, "active", "qa.db") + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "打开问答数据库失败")
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "问答数据库不可用")
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeInternal, false, "建立迁移记录失败")
	}
	all, err := migrations.All()
	if err != nil {
		_ = db.Close()
		return nil, platform.NewError(platform.CodeIntegrityError, false, "问答迁移集不合法")
	}
	for _, migration := range all {
		var name string
		switch err = db.QueryRow(`SELECT name FROM schema_migrations WHERE version=?`, migration.Version).Scan(&name); {
		case err == nil:
			continue
		case !errors.Is(err, sql.ErrNoRows):
			_ = db.Close()
			return nil, platform.NewError(platform.CodeInternal, false, "读取迁移记录失败")
		}
		tx, beginErr := db.Begin()
		if beginErr == nil {
			_, beginErr = tx.Exec(migration.SQL)
		}
		if beginErr == nil {
			_, beginErr = tx.Exec(`INSERT INTO schema_migrations(version,name) VALUES(?,?)`, migration.Version, migration.Name)
		}
		if beginErr != nil {
			if tx != nil {
				_ = tx.Rollback()
			}
			_ = db.Close()
			return nil, platform.NewError(platform.CodeInternal, false, "执行问答迁移失败")
		}
		if err = tx.Commit(); err != nil {
			_ = db.Close()
			return nil, platform.NewError(platform.CodeInternal, false, "提交问答迁移失败")
		}
	}
	if err := outbox.EnsureSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ensureIdentity 写入本域私有存储的站点/代次身份与默认策略（默认关闭公开问答）。
func ensureIdentity(ctx context.Context, db *sql.DB, siteID, epoch platform.Identifier) error {
	policy, _ := json.Marshal(map[string]any{
		"version": 0, "enabled": false, "profileId": "unconfigured",
		"retrievalSpace":      map[string]any{"spaceId": platform.HashBytes([]byte("unconfigured")), "provider": "unconfigured", "modelVersion": "unconfigured", "dimensions": 1, "preprocessingVersion": "1"},
		"maximumOutputTokens": 1500, "maximumHistoryTokens": 2048, "maximumContextTokens": 4096, "semanticThreshold": 0.94,
		"maximumQuestionCharacters": 500, "preflightOutputTokens": 500, "inspectionOutputTokens": 800, "maximumSubqueries": 4,
		"maximumRetrievalRounds": 3, "maximumToolCalls": 5, "semanticMode": "ai_equivalence", "cacheTtlDays": 7, "recordRetentionDays": 30,
		"compactionEnabled": true, "retainedRecentTokens": 15000, "contextReservePercent": 10, "cacheEnabled": true,
	})
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "问答存储不可写")
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO qa_identity(singleton,site_id,data_epoch) VALUES(1,?,?) ON CONFLICT(singleton) DO UPDATE SET data_epoch=excluded.data_epoch WHERE site_id=excluded.site_id`, siteID, epoch); err != nil {
		return platform.NewError(platform.CodeInternal, false, "写入问答存储身份失败")
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO qa_policy(singleton,version,policy_json) VALUES(1,0,?)`, policy); err != nil {
		return platform.NewError(platform.CodeInternal, false, "写入问答默认策略失败")
	}
	return tx.Commit()
}
