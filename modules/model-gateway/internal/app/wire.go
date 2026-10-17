// Package app 装配 model-gateway 域的启动依赖、恢复入口与生命周期容器。
// 供应方适配、模型调用、费用账本和额度业务由后续工作包接入。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/model-gateway/internal/admin"
	"nice-wiki/modules/model-gateway/internal/billing"
	"nice-wiki/modules/model-gateway/internal/catalog"
	"nice-wiki/modules/model-gateway/internal/gen"
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
	anchors, err := loadTrustAnchors(config.TrustAnchorRef)
	if err != nil {
		return nil, err
	}
	key, err := loadSigningKey(config.SigningKeyRef)
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
	defer store.Close()
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
	handlers := catalog.NewTaskAvailabilityHandlers(authorize)
	for operationID, handler := range billing.NewPriceHandlers() {
		handlers[operationID] = handler
	}
	for operationID, handler := range tokens.NewTokenHandlers() {
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
	_, err = apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		OpenedDataEpoch: runtimeView.DataEpoch,
		Authorize:       authorize,
		AllowedCallers: map[string][]string{
			"readTaskAvailability": {"qa"},
			"getPriceBook":         {"portal"},
			"quoteCost":            assignmentCallers[1:],
			"estimateTokens":       {"localization"},
			"putTaskAssignment":    {"portal"},
			"putProfile":           {"portal"},
			"putProvider":          {"portal"},
			"putModel":             {"portal"},
			"putCredential":        {"portal"},
			"rotateCredential":     {"portal"},
			"readTaskAssignments":  assignmentCallers,
			"readModels":           assignmentCallers,
			"readProfiles":         assignmentCallers,
			"readAdminCatalog":     {"portal"},
			"readFeeBudgetPolicy":  {"portal"},
			"putFeeBudgetPolicy":   {"portal"},
		},
	}, handlers)
	if err != nil {
		return nil, err
	}
	return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域完整handler、恢复回执及后台循环尚未实现")
}

// NewRuntimeClient 为进程模板构造 site-config 运行权威客户端；只通过
// 版本化HTTP读取运行状态，不读取其他域的文件或数据库。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	key, err := loadSigningKey(config.SigningKeyRef)
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

func loadTrustAnchors(ref string) (platform.TrustAnchors, error) {
	raw, err := os.ReadFile(ref)
	if err != nil {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚不可读")
	}
	var anchors platform.TrustAnchors
	if err := json.Unmarshal(raw, &anchors); err != nil || len(anchors.Services) == 0 {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚不合法")
	}
	return anchors, nil
}

func loadSigningKey(ref string) (platform.SigningKey, error) {
	raw, err := os.ReadFile(ref)
	if err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不可读")
	}
	var key platform.SigningKey
	if err := json.Unmarshal(raw, &key); err != nil || key.KeyID == "" || !filepath.IsAbs(key.PrivateKeyRef) {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不合法")
	}
	if _, err := serviceauth.LoadSigningKey(key); err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名私钥不可用")
	}
	return key, nil
}
