// Package app 负责 traffic-guard 的基础启动装配，不承载准入、挑战、
// 频率、并发或封禁规则。
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
	trafficguard "nice-wiki/modules/traffic-guard/internal"
	"nice-wiki/modules/traffic-guard/internal/gen"
	"nice-wiki/modules/traffic-guard/internal/transport"
)

// ModuleSlug 是本域固定的服务身份，配置不能指向其他服务。
const ModuleSlug = "traffic-guard"

// BuildApplication 校验基础配置、建立本域目录、恢复运行代次并准备
// 生命周期容器。业务存储、OpenAPI handler 和后台函数尚未实现时，按合同
// 返回 DEPENDENCY_UNAVAILABLE，不注册空处理器伪装成可用服务。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
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
	view, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		return nil, err
	}
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖后续屏障与暂存函数")
	}
	store, err := trafficguard.OpenPolicyStore(config.DataRoot, view.DataEpoch)
	if err != nil {
		return nil, err
	}
	if err := store.RunMigrations(); err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := store.RecoverJournal(); err != nil {
		_ = store.Close()
		return nil, err
	}
	defer store.Close()
	// 已实现登录操作交公共完整入口工厂核验；其余handler缺失时由工厂
	// 真实拒绝启动，不能把这两项映射裸挂成半成品服务。
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	_, err = apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		OpenedDataEpoch: view.DataEpoch,
		AllowedCallers: map[string][]string{
			"beginLoginAttempt":  {"identity"},
			"finishLoginAttempt": {"identity"},
		},
	}, trafficguard.NewLoginHandlers(store))
	if err != nil {
		return nil, err
	}
	return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域完整handler、授权客户端与后台循环尚未实现")
}

// NewRuntimeClient 为 RunService 构造 site-config 运行权威客户端；它只
// 通过版本化 HTTP 客户端读取运行状态，不读取外域文件或数据库。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
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
	if config.Listen == "" {
		return platform.NewError(platform.CodeValidation, false, "监听地址不能为空")
	}
	if !filepath.IsAbs(config.DataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	if config.TrustAnchorRef == "" || config.SigningKeyRef == "" {
		return platform.NewError(platform.CodeValidation, false, "缺少公钥表或私钥引用")
	}
	return nil
}

func ensureDataLayout(dataRoot string) error {
	for _, name := range [...]string{"active", "control", "restore"} {
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
