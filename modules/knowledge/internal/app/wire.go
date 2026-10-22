// Package app 只装配知识域的私有存储与版本化客户端；公共机制全部由platform拥有。
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/knowledge/internal/admin"
	"nice-wiki/modules/knowledge/internal/build"
	"nice-wiki/modules/knowledge/internal/events"
	"nice-wiki/modules/knowledge/internal/extract"
	"nice-wiki/modules/knowledge/internal/gen"
	indexer "nice-wiki/modules/knowledge/internal/index"
	modelruntime "nice-wiki/modules/knowledge/internal/model"
	"nice-wiki/modules/knowledge/internal/overview"
	domainruntime "nice-wiki/modules/knowledge/internal/runtime"
	"nice-wiki/modules/knowledge/internal/search"
	"nice-wiki/modules/knowledge/internal/transport"
	"os"
	"path/filepath"
	"time"
)

const ModuleSlug = "knowledge"

// NewRuntimeClient 只绑定本域身份和统一运行权威客户端，不复制断言或缓存机制。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	_, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

// BuildApplication 已真实接存储/身份/源权威与现有用例；缺全部handler或worker时严格启动失败。
// 恢复控制回执、提取/重建/worker未齐，完整应用保持partial，不返回假可运行容器。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与知识域不一致")
	}
	anchors, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"site-config", "content", "media", "jobs", "model-gateway", "memory", "identity"} {
		if config.Dependencies[name] == "" {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "本域必需权威地址未配置")
		}
	}
	for _, name := range []string{"active", "control", "restore"} {
		path := filepath.Join(config.DataRoot, name)
		if err = lifecycle.ValidatePrivatePath(path, "directory", true); err != nil {
			return nil, err
		}
		if err = os.MkdirAll(path, 0700); err != nil {
			return nil, platform.NormalizeIOError(err, platform.CodeInternal, false, "私有目录建立失败")
		}
	}
	authority, err := lifecycle.NewSiteConfigClient(config, key)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, authority)
	if err != nil {
		return nil, err
	}
	swapped, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	if swapped.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeLocked, false, "本域换目录后须先完成真实恢复激活回执")
	}
	operation := serviceOperation(ctx, "system", view.DataEpoch)
	siteClient := gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key)
	validateSite, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "SiteConfigReadSiteConfigResult")
	if err != nil {
		return nil, err
	}
	raw, err := siteClient.ReadSiteConfig(platform.WithOperationContext(platform.WithPortalProof(ctx, ""), operation))
	if err != nil {
		return nil, err
	}
	if err = validateSite(raw); err != nil {
		return nil, err
	}
	var site struct {
		Config struct {
			SiteID       string `json:"siteId"`
			BaseLanguage string `json:"baseLanguage"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &site) != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "站点配置权威不可读")
	}
	operation.Actor.SiteID = site.Config.SiteID
	db, err := indexer.OpenRepository(ctx, config.DataRoot, site.Config.SiteID, view.DataEpoch)
	if err != nil {
		return nil, err
	}
	retain := false
	defer func() {
		if !retain {
			_ = db.Close()
		}
	}()
	window := platform.WriteWindowConfig{Runtime: func(work context.Context) (platform.RuntimeView, error) {
		return lifecycle.ReadRuntime(work, authority)
	}, WriteWindow: func(work context.Context) (time.Time, error) {
		current, err := lifecycle.ReadRuntime(work, authority)
		if err != nil {
			return time.Time{}, err
		}
		if current.Maintenance {
			return time.Time{}, platform.NewError(platform.CodeLocked, false, "本域维护写窗口关闭")
		}
		return time.Time{}, nil
	}}
	gateway := gen.NewModelGatewayClient(config.Dependencies["model-gateway"], ModuleSlug, key)
	model, err := modelruntime.NewModelRuntime(gateway, db, window)
	if err != nil {
		return nil, err
	}
	indexes, err := indexer.NewIndexRuntime(indexer.Runtime{DB: db, DataRoot: config.DataRoot, Gateway: gateway, InferenceClient: model.Gateway, Memory: gen.NewMemoryClient(config.Dependencies["memory"], ModuleSlug, key), Jobs: gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key), Media: gen.NewMediaClient(config.Dependencies["media"], ModuleSlug, key), Content: gen.NewContentClient(config.Dependencies["content"], ModuleSlug, key), BaseLanguage: site.Config.BaseLanguage, WriteWindow: window})
	if err != nil {
		return nil, err
	}
	executors := platform.JobExecutorRegistry{
		"knowledge.build": func(work context.Context, input platform.JobExecutionInput) (platform.JobExecutionResult, error) {
			op, ok := platform.OperationContextFrom(work)
			if !ok {
				return platform.JobExecutionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "构建执行缺原可信操作")
			}
			return build.ExecuteBuild(work, op, input)
		},
		"knowledge.extract": func(work context.Context, input platform.JobExecutionInput) (platform.JobExecutionResult, error) {
			op, ok := platform.OperationContextFrom(work)
			if !ok {
				return platform.JobExecutionResult{}, platform.NewError(platform.CodeUnauthenticated, false, "提取执行缺原可信操作")
			}
			return extract.ExecuteExtraction(work, op, input)
		},
	}
	workers, err := domainruntime.NewWorkerRuntime(indexes.Jobs, executors, window.Runtime)
	if err != nil {
		return nil, err
	}
	bind := func(work context.Context) context.Context {
		work = indexer.WithIndexRuntime(work, indexes)
		work = modelruntime.WithModelRuntime(work, model)
		work = admin.WithAdminSigningKey(work, key)
		work = domainruntime.WithWorkerRuntime(work, workers)
		return bindSearchRuntime(work, db, indexes)
	}
	startup := bind(platform.WithOperationContext(ctx, operation))
	if _, err = domainruntime.LoadRuntime(startup, operation, domainruntime.LoadRuntimeInput{DataDirectory: config.DataRoot, ServiceID: ModuleSlug}); err != nil {
		return nil, err
	}
	tasks := assembleBackgroundTasks(bind, site.Config.SiteID, view.DataEpoch, window)
	application, err := transport.NewApplication(config, anchors, authority, view.DataEpoch, db, bind, apivalidate.NewIdentityAuthorizer(gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key)), tasks)
	if err != nil {
		return nil, err
	}
	retain = true
	return application, nil
}

func serviceOperation(ctx context.Context, siteID, epoch string) platform.OperationContext {
	deadline := time.Now().UTC().Add(30 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	id := platform.NewIdentifier()
	expires := deadline.UTC().Format(time.RFC3339Nano)
	return platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: expires, DataEpoch: epoch, CostClass: platform.CostClassBackground, Actor: platform.AuthContext{SubjectID: ModuleSlug, Kind: "service", SiteID: siteID, ExpiresAt: expires, Grants: []string{}}}
}

// bindSearchRuntime 每请求只读最新私有配置，无配置时不装配假空间或模型。
func bindSearchRuntime(ctx context.Context, db *sql.DB, indexes indexer.Runtime) context.Context {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok {
		return ctx
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ctx
	}
	config, err := indexer.ReadConfig(ctx, tx, op)
	_ = tx.Rollback()
	if err != nil || !config.Enabled {
		return ctx
	}
	runtime := search.Runtime{Store: search.SQLSnapshotReader{DB: db}, Content: indexes.Content, Space: config.Space, Language: config.Language, EmbeddingProfileID: config.EmbeddingProfileId, RerankEnabled: true}
	if config.RerankProfileId != nil {
		runtime.RerankProfileID = *config.RerankProfileId
	}
	initialized, err := search.NewSearchRuntime(runtime)
	if err != nil {
		return ctx
	}
	return search.WithSearchRuntime(ctx, initialized)
}

func assembleBackgroundTasks(bind func(context.Context) context.Context, siteID, epoch string, window platform.WriteWindowConfig) []platform.BackgroundTask {
	batch := func(name string, pollMillis int, run func(context.Context, platform.OperationContext) error) platform.BackgroundTask {
		return platform.BackgroundTask{Name: name, Run: func(ctx context.Context) error {
			return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{PollMillis: pollMillis, Runtime: window.Runtime, WriteWindow: window.WriteWindow, Recover: func(work context.Context) error {
				op := serviceOperation(work, siteID, epoch)
				return run(bind(platform.WithOperationContext(work, op)), op)
			}})
		}}
	}
	nextGC := time.Now().Add(10 * time.Minute)
	return []platform.BackgroundTask{
		{Name: "knowledge.worker", Run: func(ctx context.Context) error {
			op := serviceOperation(ctx, siteID, epoch)
			_, err := domainruntime.RunWorker(bind(platform.WithOperationContext(ctx, op)), op, domainruntime.RunWorkerInput{WorkerID: ModuleSlug, MaximumConcurrentJobs: 4})
			return err
		}},
		batch("knowledge.outbox", 1000, func(ctx context.Context, op platform.OperationContext) error {
			_, err := events.PublishOutbox(ctx, op, events.PublishOutboxInput{Limit: 100, WorkerID: ModuleSlug})
			return err
		}),
		// 层级摘要与全站概括：每分钟检查一次，满 24 小时（或从没生成过）才在后台重算。
		batch("knowledge.overview", 60000, func(ctx context.Context, op platform.OperationContext) error {
			return overview.RunDaily(ctx, op)
		}),
		batch("knowledge.recovery", 60000, func(ctx context.Context, op platform.OperationContext) error {
			if err := domainruntime.RecoverExtractionTasks(ctx, op, 100); err != nil {
				return err
			}
			if _, err := domainruntime.RecoverBuildLeases(ctx, op, domainruntime.RecoverBuildLeasesInput{Now: platform.Now(), Limit: 100}); err != nil {
				return err
			}
			return build.RetryFailedBuilds(ctx, op)
		}),
		batch("knowledge.gc", 60000, func(ctx context.Context, op platform.OperationContext) error {
			if time.Now().Before(nextGC) {
				return nil
			}
			_, err := domainruntime.CollectRetiredIndexes(ctx, op, domainruntime.CollectRetiredIndexesInput{OlderThan: time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano), Limit: 100})
			if err == nil {
				nextGC = time.Now().Add(10 * time.Minute)
			}
			return err
		}),
		{Name: "knowledge.events", Run: func(ctx context.Context) error {
			op := serviceOperation(ctx, siteID, epoch)
			_, err := events.ConsumeEvents(bind(platform.WithOperationContext(ctx, op)), op, events.ConsumeEventsInput{ConsumerID: ModuleSlug, MaxInFlight: 1})
			return err
		}},
	}
}
