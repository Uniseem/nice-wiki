// Package app 装配 data-protection 的数据目录、服务身份与 HTTP 容器。
// 不执行备份、恢复、镜像或远程 Git。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	protection "nice-wiki/modules/data-protection/internal"
	"nice-wiki/modules/data-protection/internal/gen"
	"nice-wiki/modules/data-protection/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "data-protection"

// BuildApplication 验证本域启动前置条件，建立 active/control/restore，
// 加载服务身份，读取运行权威并执行恢复换目录，然后绑定全部 OpenAPI 路由。
//
// 私有库迁移、journal、激活回执、事件循环和任务执行都还没有可调用的本域实现。
// 换目录后无法诚实登记回执，业务路由由 transport 失败关闭，不用空结果冒充备份成功。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
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
	if anchors.ServiceKeyID(ModuleSlug, key.KeyID) == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域服务身份未登记在信任锚中")
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
		// 回执需要 stageId、manifestHash 与 runtime revision。当前没有这些持久事实，
		// 不能调用 registerActivation，也不能把已换入目录当成可服务状态。
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖本域journal，尚未实现")
	}
	clients, err := assembleClients(config, key)
	if err != nil {
		return nil, err
	}
	catalog, err := protection.OpenCatalog(ctx, config.DataRoot, view.DataEpoch, key)
	if err != nil {
		return nil, err
	}
	clients.Catalog = catalog
	protection.InstallCatalog(catalog)
	application, err := transport.NewApplication(config, anchors, runtimeAuthority, clients)
	if err != nil {
		_ = catalog.Close()
		return nil, err
	}
	return application, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供进程模板监视恢复切换。
// 构造阶段不发起网络请求。
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
	if config.Dependencies["site-config"] == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

// assembleClients 只装配生成客户端里已经存在的生产方。
// 其余域还没有快照客户端，留空等待，不发起探测请求。
func assembleClients(config platform.ServiceConfig, key platform.SigningKey) (transport.Dependencies, error) {
	var clients transport.Dependencies
	var err error
	if clients.Identity, err = newIdentity(config, key); err != nil {
		return transport.Dependencies{}, err
	}
	if clients.Jobs, err = newJobs(config, key); err != nil {
		return transport.Dependencies{}, err
	}
	if clients.Content, err = newContent(config, key); err != nil {
		return transport.Dependencies{}, err
	}
	if clients.SiteConfig, err = newSiteConfig(config, key); err != nil {
		return transport.Dependencies{}, err
	}
	return clients, nil
}

func newIdentity(config platform.ServiceConfig, key platform.SigningKey) (*gen.IdentityClient, error) {
	baseURL := config.Dependencies["identity"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少identity依赖地址")
	}
	return gen.NewIdentityClient(baseURL, ModuleSlug, key), nil
}

func newJobs(config platform.ServiceConfig, key platform.SigningKey) (*gen.JobsClient, error) {
	baseURL := config.Dependencies["jobs"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少jobs依赖地址")
	}
	return gen.NewJobsClient(baseURL, ModuleSlug, key), nil
}

func newContent(config platform.ServiceConfig, key platform.SigningKey) (*gen.ContentClient, error) {
	baseURL := config.Dependencies["content"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少content依赖地址")
	}
	return gen.NewContentClient(baseURL, ModuleSlug, key), nil
}

func newSiteConfig(config platform.ServiceConfig, key platform.SigningKey) (*gen.SiteConfigClient, error) {
	baseURL := config.Dependencies["site-config"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return gen.NewSiteConfigClient(baseURL, ModuleSlug, key), nil
}

// ensureDataLayout 建立 active/control/restore。control 区不随业务恢复替换。
func ensureDataLayout(dataRoot string) error {
	if err := lifecycle.ValidatePrivatePath(dataRoot, "directory", true); err != nil {
		return err
	}
	for _, name := range [...]string{"active", "control", "restore"} {
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}
	for _, name := range [...]string{"", "active", "control", "restore"} {
		path := dataRoot
		if name != "" {
			path = filepath.Join(dataRoot, name)
		}
		if err := lifecycle.ValidatePrivatePath(path, "directory", false); err != nil {
			return err
		}
	}
	return nil
}

// loadTrustAnchors 读取公钥表。文件不可读时不使用占位身份继续启动。
func loadTrustAnchors(ref string) (platform.TrustAnchors, error) {
	if err := lifecycle.ValidatePrivatePath(ref, "regular-file", false); err != nil {
		return platform.TrustAnchors{}, err
	}
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

// loadSigningKey 读取外部密钥引用并确认私钥可用。私钥字节只留在平台缓存。
func loadSigningKey(ref string) (platform.SigningKey, error) {
	if err := lifecycle.ValidatePrivatePath(ref, "regular-file", false); err != nil {
		return platform.SigningKey{}, err
	}
	raw, err := os.ReadFile(ref)
	if err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不可读")
	}
	var key platform.SigningKey
	if err := json.Unmarshal(raw, &key); err != nil || key.KeyID == "" || !filepath.IsAbs(key.PrivateKeyRef) {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不合法")
	}
	if err := lifecycle.ValidatePrivatePath(key.PrivateKeyRef, "regular-file", false); err != nil {
		return platform.SigningKey{}, err
	}
	if _, err := serviceauth.LoadSigningKey(key); err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名私钥不可用")
	}
	return key, nil
}
