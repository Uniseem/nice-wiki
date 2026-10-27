// 装配问答域：私有 SQLite、生产方客户端与已实现的 HTTP 处理函数。
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	_ "nice-wiki/libs/platform/pgdb"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/qa/internal/events"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"nice-wiki/modules/qa/internal/transport"
	"nice-wiki/modules/qa/internal/turn"
	"nice-wiki/modules/qa/migrations"
)

// ModuleSlug 是本域固定的服务身份。
const ModuleSlug = "qa"

// identityEnsured 记录已经写过存储身份的站点；每个站点第一次带真实站点的请求时写一次。
var identityEnsured sync.Map

// BuildApplication 返回可运行的 Application。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*Application, error) {
	if config.Module != ModuleSlug || !filepath.IsAbs(config.DataRoot) || config.Listen == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "本域配置不合法")
	}
	for _, name := range []string{"active", "control"} {
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
		Guard:               gen.NewTrafficGuardClient(dep("traffic-guard"), ModuleSlug, key),
		Content:             gen.NewContentClient(dep("content"), ModuleSlug, key),
		PublicSite:          gen.NewPublicSiteClient(dep("public-site"), ModuleSlug, key),
		Knowledge:           gen.NewKnowledgeClient(dep("knowledge"), ModuleSlug, key),
		Memory:              gen.NewMemoryClient(dep("memory"), ModuleSlug, key),
		Localization:        gen.NewLocalizationClient(dep("localization"), ModuleSlug, key),
		Agents:              gen.NewAgentClient(dep("agent"), ModuleSlug, key),
		Models:              gen.NewModelGatewayClient(dep("model-gateway"), ModuleSlug, key),
		Jobs:                gen.NewJobsClient(dep("jobs"), ModuleSlug, key),
		Identity:            gen.NewIdentityClient(dep("identity"), ModuleSlug, key),
		AnswerContextTokens: 4096, PreflightContextTokens: 2048,
	})
	if err != nil {
		return nil, err
	}
	// 站点基准语言以 site-config 为准：站点初始化前读不到就留空，用到它的问答接口报依赖暂不可用；读到后不再变化。
	siteConfig := gen.NewSiteConfigClient(dep("site-config"), ModuleSlug, key)
	var baseLanguage sync.Map // 站点 -> 基准语言
	withBaseLanguage := func(c context.Context, op platform.OperationContext) qaruntime.Runtime {
		current := runtime
		if cached, ok := baseLanguage.Load(string(op.Actor.SiteID)); ok {
			current.BaseLanguage = cached.(string)
			return current
		}
		raw, readErr := siteConfig.ReadSiteConfig(qaruntime.ServiceReadContext(c, op))
		var site struct {
			Config struct {
				BaseLanguage string `json:"baseLanguage"`
			} `json:"config"`
		}
		if readErr == nil {
			readErr = qaruntime.DecodeResponse(runtime, "SiteConfigReadSiteConfigResult", raw, &site)
		}
		if readErr == nil && site.Config.BaseLanguage != "" {
			baseLanguage.Store(string(op.Actor.SiteID), site.Config.BaseLanguage)
			current.BaseLanguage = site.Config.BaseLanguage
		} else if os.Getenv("NW_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "站点基准语言暂不可读：%v\n", readErr)
		}
		return current
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
		// 本域私有存储的站点身份取自该站点第一个带真实站点的请求，之后不再变化。
		op, ok := platform.OperationContextFrom(c)
		if ok && op.Actor.SiteID != "" && op.Actor.SiteID != "system" {
			if _, done := identityEnsured.Load(string(op.Actor.SiteID)); !done {
				if ensureIdentity(c, db, op.Actor.SiteID, view.DataEpoch) == nil {
					identityEnsured.Store(string(op.Actor.SiteID), true)
				}
			}
		}
		if !ok {
			return qaruntime.WithRuntime(c, runtime)
		}
		return qaruntime.WithRuntime(c, withBaseLanguage(c, op))
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
	// serviceOp 后台任务以本服务身份操作本站数据；还没有任何请求建立站点身份时返回 false。
	serviceOp := func(tick context.Context) (context.Context, platform.OperationContext, bool) {
		var site string
		if err := db.QueryRowContext(tick, `SELECT site_id FROM qa_identity WHERE singleton=1`).Scan(&site); err != nil {
			return nil, platform.OperationContext{}, false
		}
		deadline := time.Now().UTC().Add(30 * time.Second)
		id := platform.NewIdentifier()
		op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline.Format(time.RFC3339Nano), DataEpoch: view.DataEpoch,
			Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: site, ExpiresAt: deadline.Format(time.RFC3339Nano), Grants: []string{}}}
		ctx := platform.WithOperationContext(tick, op)
		return qaruntime.WithRuntime(ctx, withBaseLanguage(ctx, op)), op, true
	}
	started := time.Now().UnixNano()
	interruptedHandled := false
	application.SetBackgroundTasks([]platform.BackgroundTask{{Name: "qa.outbox", Run: func(work context.Context) error {
		return lifecycle.RunRecoveryLoop(work, platform.RecoveryLoopConfig{PollMillis: 1000, Runtime: window.Runtime, WriteWindow: window.WriteWindow, Recover: func(tick context.Context) error {
			ctx, op, ok := serviceOp(tick)
			if !ok {
				return nil
			}
			_, err := events.PublishOutbox(ctx, op, events.PublishOutboxInput{Limit: 100})
			return err
		}})
	}}, {Name: "qa.housekeeping", Run: func(work context.Context) error {
		// 启动后先收尾上次进程中断的轮次，之后每分钟按保留期清理过期会话、诊断记录和已终止轮次的事件。
		return lifecycle.RunRecoveryLoop(work, platform.RecoveryLoopConfig{PollMillis: 60000, Runtime: window.Runtime, WriteWindow: window.WriteWindow, Recover: func(tick context.Context) error {
			ctx, op, ok := serviceOp(tick)
			if !ok {
				return nil
			}
			if !interruptedHandled {
				if err := turn.FailInterruptedTurns(ctx, op, started); err != nil {
					return err
				}
				interruptedHandled = true
			}
			_, err := qaruntime.PruneExpiredRecords(ctx, op, qaruntime.PruneExpiredRecordsInput{Now: time.Now(), Limit: 200})
			return err
		}})
	}}})
	retain = true
	application.Mount(handler, paths, func() { _ = db.Close() })
	return application, nil
}

// callers 为规格里每个业务接口指定调用方：访客与后台操作都经 portal，其余服务接口固定到对应服务。
func callers(spec *openapi3.T) map[string][]string {
	special := map[string][]string{
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
		"maximumOutputTokens": 1500, "maximumHistoryTokens": 2048, "maximumContextTokens": 16000, "maximumQuestionCharacters": 500, "maximumRetrievalRounds": 12, "maximumToolCalls": 48, "recordRetentionDays": 15,
		"compactionEnabled": true, "retainedRecentTokens": 15000, "contextReservePercent": 10})
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
