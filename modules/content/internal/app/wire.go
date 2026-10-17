// Package app 装配 content 域的启动依赖与运行权威客户端。
// 文档、草稿、发布、历史和事件业务由后续工作包接入，本文件不提供伪处理器。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	content "nice-wiki/modules/content/internal"
	"nice-wiki/modules/content/internal/gen"
	"nice-wiki/modules/content/internal/transport"
)

// ModuleSlug 是本域固定的服务身份。
const ModuleSlug = "content"

// BuildApplication 按服务启动蓝图校验本域启动前置条件。
//
// 本域已接入私有SQLite打开、迁移和正文日志恢复。完整草稿事务、
// OpenAPI x-handler、恢复activation和后台任务尚未装配，按合同返回
// DEPENDENCY_UNAVAILABLE，不返回不完整的Application。
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

	runtimeClient, err := NewRuntimeClient(config)
	if err != nil {
		return nil, err
	}
	runtimeView, err := lifecycle.ReadRuntime(ctx, runtimeClient)
	if err != nil {
		return nil, err
	}
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, runtimeView)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		// 换目录后的 activation 回执、journal 恢复和业务资源重建尚未接入。
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖本域后续工作包")
	}

	store, err := content.OpenDatabase(ctx, config.DataRoot, runtimeView.DataEpoch)
	if err != nil {
		return nil, err
	}
	defer store.CloseDatabase()
	editLocks, err := content.NewEditLockManager(store)
	if err != nil {
		return nil, err
	}
	defer editLocks.CloseEditLocks()
	if err := store.RecoverDatabaseJournal(ctx); err != nil {
		return nil, err
	}

	key, err := loadSigningKey(config.SigningKeyRef)
	if err != nil {
		return nil, err
	}
	var identity *gen.IdentityClient
	if endpoint := config.Dependencies["identity"]; endpoint != "" {
		identity = gen.NewIdentityClient(endpoint, ModuleSlug, key)
	}
	referenceRuntime, err := content.NewReferenceRuntime(store, identity, key)
	if err != nil {
		return nil, err
	}
	application := transport.NewApplication(config, runtimeClient, runtimeView.DataEpoch, true, false, false)
	if err = transport.RegisterReferenceRoutes(application, referenceRuntime, anchors); err != nil {
		return nil, err
	}
	// 两个只读来源路由的真实装配不满足全域handler/恢复activation/后台生命周期门槛。
	return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "content完整业务路由、恢复activation与后台生命周期尚未装配")
}

// NewRuntimeClient 构造 site-config 运行权威客户端；只保存外部引用，
// 不把密钥原文或服务端点写入日志。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	baseURL := config.Dependencies["site-config"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	key, err := loadSigningKey(config.SigningKeyRef)
	if err != nil {
		return nil, err
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

func validateConfig(config platform.ServiceConfig) error {
	if config.Module != ModuleSlug {
		return platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	if config.Listen == "" || config.DataRoot == "" || !filepath.IsAbs(config.DataRoot) {
		return platform.NewError(platform.CodeValidation, false, "本域监听或数据目录配置不合法")
	}
	if config.TrustAnchorRef == "" || config.SigningKeyRef == "" {
		return platform.NewError(platform.CodeValidation, false, "本域服务身份引用不完整")
	}
	if config.Dependencies == nil || config.Dependencies["site-config"] == "" {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "content运行权威不可用")
	}
	return nil
}

func ensureDataLayout(dataRoot string) error {
	for _, name := range []string{"active", "control", "restore"} {
		path := filepath.Join(dataRoot, name)
		if err := lifecycle.ValidatePrivatePath(path, "directory", true); err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
		if err := lifecycle.ValidatePrivatePath(path, "directory", false); err != nil {
			return err
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
