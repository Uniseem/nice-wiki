// Package app 装配 model-gateway 域的启动依赖、恢复入口与生命周期容器。
// 供应方适配、模型调用、费用账本和额度业务由后续工作包接入。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/model-gateway/internal/admin"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/gen"
	"nice-wiki/modules/model-gateway/internal/invocation"
	"nice-wiki/modules/model-gateway/internal/routing"
	gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
	"nice-wiki/modules/model-gateway/internal/security"
	"nice-wiki/modules/model-gateway/internal/tokens"
	"nice-wiki/modules/model-gateway/internal/transport"
)

// ModuleSlug 是本域固定的服务身份；配置不能装配成其他服务。
const ModuleSlug = "model-gateway"

// BuildApplication 按服务启动合同校验配置、建立本域目录、加载服务身份
// 信任锚与签名密钥、读取运行权威并执行恢复换目录前置流程。
//
// 本包装配私有库和任务配置映射，不读取主密钥、不启动模型或费用任务。
// 其余handler、恢复回执和后台未齐时，公共完整工厂仍拒绝启动。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
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
	runtimeView, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		return nil, err
	}
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, runtimeView)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖本域后续工作包")
	}

	store, err := gatewayruntime.OpenDatabase(config.DataRoot)
	if err != nil {
		return nil, err
	}
	gatewayruntime.InstallStore(store, config.MasterKeyRef)
	if err = store.SetDataEpoch(ctx, runtimeView.DataEpoch); err != nil {
		_ = store.Close()
		return nil, err
	}
	retain := false
	defer func() {
		if !retain {
			_ = store.Close()
		}
	}()
	taskGroup, err := lifecycle.NewTaskGroup(ctx, 256)
	if err != nil {
		return nil, err
	}
	validators := make(map[string]func(json.RawMessage) error)
	for _, name := range []string{"MediaReadAssetResult", "LanguageSuggestionApprovalResult", "SiteInitializationStatusResult", "GatewaySiteConfigResult"} {
		validator, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if err != nil {
			return nil, err
		}
		validators[name] = validator
	}
	siteClient := gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key)
	siteID, err := ReadGatewaySiteID(ctx, siteClient, runtimeView.DataEpoch, validators["GatewaySiteConfigResult"])
	if err != nil {
		return nil, err
	}
	mediaClient := gen.NewMediaClient(config.Dependencies["media"], ModuleSlug, key)
	options := gatewayruntime.ExecutionOptions{SiteID: siteID, Anchors: anchors, Tasks: taskGroup, Media: mediaClient, MediaBytes: mediaClient, Jobs: gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key), Validators: validators, WriteWindow: platform.WriteWindowConfig{Runtime: runtimeAuthority.ReadRuntimeState, WriteWindow: func(work context.Context) (time.Time, error) {
		view, err := runtimeAuthority.ReadRuntimeState(work)
		if err != nil {
			return time.Time{}, err
		}
		if view.DataEpoch != runtimeView.DataEpoch {
			return time.Time{}, platform.NewError(platform.CodeStaleResult, false, "模型写窗口代际已变化")
		}
		if view.Phase != platform.PhaseRunning && view.Phase != platform.PhaseBackupBarrier && view.Phase != platform.PhaseRestoreStaging {
			return time.Time{}, platform.NewError(platform.CodeLocked, true, "模型写窗口已关闭")
		}
		return time.Time{}, nil
	}}}
	if base := config.Dependencies["localization"]; base != "" {
		options.LanguageApprovals = gen.NewLocalizationClient(base, ModuleSlug, key)
	}
	gatewayruntime.InstallExecutionOptions(options)
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	identityClient := gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key)
	authorize := apivalidate.NewIdentityAuthorizer(identityClient)
	assignmentCallers := []string{"portal"}
	seen := map[string]bool{"portal": true}
	for _, task := range gen.AITaskCatalog {
		if !seen[task.Owner] {
			assignmentCallers = append(assignmentCallers, task.Owner)
			seen[task.Owner] = true
		}
	}
	usageCallers := append(append([]string{}, assignmentCallers...), "analytics")
	usageServices := append(append([]string{}, assignmentCallers[1:]...), "analytics")
	handlers := catalog.NewTaskAvailabilityHandlers(authorize)
	for operationID, handler := range routing.NewRouteHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range invocation.NewExecutionHandlers() {
		handlers[operationID] = handler
	}
	ledgerHandlers, err := admin.NewLedgerHandlers(key)
	if err != nil {
		return nil, err
	}
	for operationID, handler := range ledgerHandlers {
		handlers[operationID] = handler
	}
	for operationID, handler := range billing.NewPriceHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range tokens.NewTokenizerHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range tokens.NewTokenHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range catalog.NewCatalogRefreshHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range catalog.NewCatalogHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range admin.NewAdminHandlers(authorize) {
		handlers[operationID] = handler
	}
	for operationID, handler := range catalog.NewProfileHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range security.NewCredentialHandlers() {
		handlers[operationID] = handler
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		OpenedDataEpoch: runtimeView.DataEpoch,
		Authorize:       authorize,
		ServiceActorCallers: map[string][]string{"resolveTaskRoute": assignmentCallers[1:], "tokenizeText": {"knowledge"}, "lookupInference": assignmentCallers[1:],
			"createInference": assignmentCallers[1:], "readUsageStatisticsPage": {"analytics"}, "quoteCost": assignmentCallers[1:], "cancelInference": assignmentCallers[1:], "queryUsage": usageServices, "readModels": assignmentCallers[1:], "readProfiles": assignmentCallers[1:], "readInference": assignmentCallers[1:], "resumeInference": assignmentCallers[1:]},
		AllowedCallers: map[string][]string{
			"resolveTaskRoute":        assignmentCallers[1:],
			"readUsageStatisticsPage": {"analytics"},
			"lookupInference":         assignmentCallers[1:],
			"getLedger":               {"portal"},
			"readInference":           assignmentCallers,
			"resumeInference":         assignmentCallers,
			"reconcileCharge":         {"portal"},
			"createInference":         assignmentCallers[1:],
			"cancelInference":         assignmentCallers[1:],
			"queryUsage":              usageCallers,
			"readTaskAvailability":    {"qa"},
			"getPriceBook":            {"portal"},
			"quoteCost":               assignmentCallers,
			"tokenizeText":            {"knowledge"},
			"estimateTokens":          {"localization"},
			"putTaskAssignment":       {"portal"},
			"putProfile":              {"portal"},
			"putProvider":             {"portal"},
			"putModel":                {"portal"},
			"refreshModelCatalog":     {"portal"},
			"putCredential":           {"portal"},
			"rotateCredential":        {"portal"},
			"readTaskAssignments":     assignmentCallers,
			"readModels":              assignmentCallers,
			"readProfiles":            assignmentCallers,
			"readAdminCatalog":        {"portal"},
			"createConnectionProbe":   {"portal"},
			"readFeeBudgetPolicy":     {"portal"},
			"putFeeBudgetPolicy":      {"portal"},
		},
	}, handlers)
	if err != nil {
		return nil, err
	}
	application := transport.NewApplication(config, anchors, runtimeAuthority, runtimeView.DataEpoch, true, true, true)
	retain = true
	initialized := func(work context.Context) (bool, error) {
		return ReadInitializationState(work, siteClient, validators["SiteInitializationStatusResult"])
	}
	application.ConfigureBusiness(handler, nil, func(context.Context) error { return store.Close() }, func(context.Context) (bool, error) { return true, nil }, initialized)
	return application, nil
}

// NewRuntimeClient 为进程模板构造 site-config 运行权威客户端；只通过
// 版本化HTTP读取运行状态，不读取其他域的文件或数据库。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	_, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	return newRuntimeClient(config, key)
}

func newRuntimeClient(config platform.ServiceConfig, key platform.SigningKey) (platform.SiteConfigClient, error) {
	baseURL := config.Dependencies["site-config"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

func validateConfig(config platform.ServiceConfig) error {
	if config.Module != ModuleSlug {
		return platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	if config.Listen == "" || !filepath.IsAbs(config.DataRoot) {
		return platform.NewError(platform.CodeValidation, false, "本域监听或数据目录配置不合法")
	}
	if config.TrustAnchorRef == "" || config.SigningKeyRef == "" {
		return platform.NewError(platform.CodeValidation, false, "本域服务身份引用不完整")
	}
	for _, dependency := range []string{"identity", "site-config", "media", "jobs"} {
		if config.Dependencies == nil || config.Dependencies[dependency] == "" {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "缺少本域启动依赖地址")
		}
	}
	return nil
}

func ensureDataLayout(dataRoot string) error {
	for _, name := range []string{"active", "control", "restore"} {
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}
	return nil
}

// ReadInitializationState只读site-config实际初始化权威，不以epoch推测。
func ReadInitializationState(ctx context.Context, client *gen.SiteConfigClient, validate func(json.RawMessage) error) (bool, error) {
	work, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	deadline, _ := work.Deadline()
	id := platform.NewIdentifier()
	op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline.UTC().Format(time.RFC3339Nano), Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", Grants: []string{}, ExpiresAt: deadline.UTC().Format(time.RFC3339Nano)}}
	raw, err := client.ReadInitializationStatus(platform.WithOperationContext(platform.WithPortalProof(work, ""), op))
	if err != nil {
		return false, err
	}
	var status struct {
		Initialized bool `json:"initialized"`
	}
	if validate(raw) != nil || json.Unmarshal(raw, &status) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "站点初始化权威响应不符合合同")
	}
	return status.Initialized, nil
}

// ReadGatewaySiteID只取生产方严格响应的真实siteId，不把system或旧费用行当当前站点。
func ReadGatewaySiteID(ctx context.Context, client *gen.SiteConfigClient, epoch string, validate func(json.RawMessage) error) (string, error) {
	work, cancel := context.WithTimeout(platform.WithPortalProof(ctx, ""), 2500*time.Millisecond)
	defer cancel()
	deadline, _ := work.Deadline()
	id := platform.NewIdentifier()
	op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug}, DataEpoch: epoch, Deadline: deadline.UTC().Format(time.RFC3339Nano)}
	raw, err := client.ReadSiteConfig(platform.WithOperationContext(work, op))
	if err != nil {
		return "", err
	}
	if validate(raw) != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "站点配置响应不符合生产合同")
	}
	var result struct {
		Config struct {
			SiteID string `json:"siteId"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Config.SiteID == "" {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "网关当前站点尚未初始化")
	}
	return result.Config.SiteID, nil
}
