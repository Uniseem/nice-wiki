// Package app 装配 identity 域的数据目录布局、服务身份信任锚、运行
// 权威客户端与 HTTP 容器；不承载账号、会话等业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/identity/internal"
	"nice-wiki/modules/identity/internal/gen"
	"nice-wiki/modules/identity/internal/transport"
)

// ModuleSlug 本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "identity"

// BuildApplication 验证本域配置，建立 active/control 目录布局，
// 加载服务身份信任锚与签名密钥，读取运行权威状态，
// 打开active区业务库执行schema迁移与决定日志恢复，最后返回注册了
// 匿名健康/就绪路由的应用容器。
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
	runtimeAuthority, err := NewRuntimeClient(config)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		return nil, err
	}
	store, err := identity.OpenDatabase(config.DataRoot)
	if err != nil {
		return nil, err
	}
	if err := store.RunMigrations(); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.EnsureOutbox(ctx); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.RecoverJournal(); err != nil {
		store.Close()
		return nil, err
	}
	openedEpoch := view.DataEpoch
	origin, err := lifecycle.CanonicalPublicOrigin(config.PublicBaseURL)
	if err != nil {
		store.Close()
		return nil, err
	}
	application, err := transport.NewApplication(config, anchors, runtimeAuthority, store, openedEpoch, identity.SessionRuntime{Key: key, Anchors: anchors, PublicOrigin: origin})
	if err != nil {
		store.Close()
		return nil, err
	}
	if config.Dependencies["jobs"] == "" {
		store.Close()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "缺少jobs依赖地址")
	}
	publisher := outbox.NewEventPublisher(gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key))
	if err := application.InstallBackgroundTasks(runtimeAuthority, publisher); err != nil {
		store.Close()
		return nil, err
	}
	return application, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端（platform.SiteConfigClient），
// 供进程模板的运行状态监视使用；不发起网络请求。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	base := config.Dependencies["site-config"]
	if base == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	key, err := loadSigningKey(config.SigningKeyRef)
	if err != nil {
		return nil, err
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

// ensureDataLayout 建立 active/control/restore 三区；control 区不随业务
// 恢复替换，restore 供 ADR0003 换目录协议使用。
func ensureDataLayout(dataRoot string) error {
	if err := lifecycle.ValidatePrivatePath(dataRoot, "directory", true); err != nil {
		return err
	}
	for _, name := range [...]string{"active", "control"} {
		if name == "control" {
			if err := lifecycle.ValidatePrivatePath(filepath.Join(dataRoot, name), "directory", true); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}
	return nil
}

// loadTrustAnchors 读取 nw-keys 生成的公钥表；不可用时按合同返回
// DEPENDENCY_UNAVAILABLE，启动失败交由进程守护重试。
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

// loadSigningKey 读取签名密钥引用文件 {keyId, privateKeyRef} 并验证
// 私钥可解析；secret 只保存引用路径，不把私钥内容写入日志。
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
