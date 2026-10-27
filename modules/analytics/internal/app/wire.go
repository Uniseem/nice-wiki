// Package app 装配 analytics 域的启动依赖与 HTTP 容器；不承载统计、审计、
// 导出或匿名访问业务规则。
package app

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/queue"
	"nice-wiki/libs/platform/worker"
	analytics "nice-wiki/modules/analytics/internal"
	"nice-wiki/modules/analytics/internal/gen"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/analytics/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "analytics"

// BuildApplication 验证启动配置，建立本域控制区与业务区，加载服务身份，
// 读取运行权威并执行启动时恢复换目录，然后绑定完整 OpenAPI 路由。
//
// 装配真实私有存储、统计任务与只读/审计入口；未完成的费用与恢复能力
// 继续失败关闭，整体就绪仍等待完整快照和journal恢复。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	if err := ensureDataLayout(config.DataRoot); err != nil {
		return nil, err
	}
	anchors, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}

	runtimeAuthority, err := newRuntimeClient(config, key)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		return nil, err
	}
	if config.Dependencies["identity"] == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "identity授权地址未配置")
	}
	identity := gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key)
	site := gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key)
	validator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "AnalyticsSiteConfigResult")
	if err != nil {
		return nil, err
	}
	siteID, err := readSiteIdentity(ctx, site, view.DataEpoch, validator)
	if err != nil {
		return nil, err
	}
	store, err := analytics.OpenStore(ctx, config.DataRoot, siteID, view.DataEpoch, key)
	if err != nil {
		return nil, err
	}
	if err = analytics.BindReportingTimeZone(ctx, store, config.ReportingTimeZone, view); err != nil {
		_ = store.Close()
		return nil, err
	}
	store.Window = platform.WriteWindowConfig{Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
		return lifecycle.ReadRuntime(ctx, runtimeAuthority)
	}, WriteWindow: func(context.Context) (time.Time, error) { return time.Time{}, nil }}
	store.Authorize = apivalidate.NewIdentityAuthorizer(identity)
	if endpoint := config.Dependencies["model-gateway"]; endpoint != "" {
		validateUsagePage, e := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "AnalyticsUsageStatisticsPageResult")
		if e != nil {
			_ = store.Close()
			return nil, e
		}
		store.UsageGateway = &analytics.UsageGateway{
			Reader:       gen.NewModelGatewayClient(endpoint, ModuleSlug, key),
			ValidatePage: validateUsagePage,
			SiteID:       siteID,
			DataEpoch:    view.DataEpoch,
		}
	}
	recovery, cancelRecovery, recoveryOp := analytics.BackgroundOperation(ctx, store, 30*time.Second)
	if err = analytics.RecoverExports(recovery, recoveryOp); err != nil {
		cancelRecovery()
		_ = store.Close()
		return nil, err
	}
	cancelRecovery()
	if config.Dependencies["jobs"] == "" {
		_ = store.Close()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "jobs地址未配置")
	}
	jobs := gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key)
	validators := worker.ResponseValidators{}
	for _, name := range []string{"ClaimJobResult", "StartJobResult", "HeartbeatJobResult", "CheckLeaseResult", "CompleteJobResult", "FailJobResult"} {
		validator, e := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "AnalyticsJobs"+name)
		if e != nil {
			_ = store.Close()
			return nil, e
		}
		validators[name] = validator
	}
	store.Jobs, err = worker.NewJobsClient(jobs, validators)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	store.Submitter = jobs
	store.ValidateCreateJob, err = apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "AnalyticsJobsCreateJobResult")
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	events, err := queue.ForConfig(ctx, config)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	store.Consumer = events.Consumer()
	eventSchemas := map[string]string{"qa.turn.completed": "AnalyticsQaCompletedPayload", "qa.turn.failed": "AnalyticsQaFailedPayload", "invocation.settled": "AnalyticsSettledPayload", "invocation.reconciled": "AnalyticsReconciledPayload"}
	eventValidators := map[string]func(json.RawMessage) error{}
	for event, schema := range eventSchemas {
		validate, e := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), schema)
		if e != nil {
			_ = store.Close()
			return nil, e
		}
		eventValidators[event] = validate
	}
	store.ValidateEvent = func(name string, version int, payload json.RawMessage) error {
		validate := eventValidators[name]
		if version != 1 || validate == nil {
			return platform.NewError(platform.CodeUnsupportedSchema, false, "统计事件schema未登记")
		}
		return validate(payload)
	}
	application, err := transport.NewApplication(config, anchors, runtimeAuthority, store, identity)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return application, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供进程模板和Ready使用。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	_, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	return newRuntimeClient(config, key)
}

// newRuntimeClient 只通过 site-config 的版本化HTTP接口读取运行权威。
func newRuntimeClient(config platform.ServiceConfig, key platform.SigningKey) (platform.SiteConfigClient, error) {
	baseURL := config.Dependencies["site-config"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

// ensureDataLayout 建立 active/control/restore 三区；control区不随业务恢复替换。
func ensureDataLayout(dataRoot string) error {
	if !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	for _, name := range [...]string{"active", "control"} {
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}
	return nil
}

// readSiteIdentity只消费生产方严格完整响应中的真实siteId，不自行生成站点身份。
func readSiteIdentity(ctx context.Context, client *gen.SiteConfigClient, epoch string, validate func(json.RawMessage) error) (string, error) {
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	deadline, _ := work.Deadline()
	id := platform.NewIdentifier()
	op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug}, DataEpoch: epoch, Deadline: deadline.UTC().Format(time.RFC3339Nano)}
	raw, err := client.ReadSiteConfig(platform.WithOperationContext(work, op))
	if err != nil {
		return "", err
	}
	if err = validate(raw); err != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "站点配置响应不符合生产合同")
	}
	var result struct {
		Config struct {
			SiteID string `json:"siteId"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Config.SiteID == "" {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "站点身份尚未建立")
	}
	return result.Config.SiteID, nil
}
