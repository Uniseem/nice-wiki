// Package app 装配 public-site 的启动依赖。
package app

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	publicsite "nice-wiki/modules/public-site/internal"
	"nice-wiki/modules/public-site/internal/gen"
	"nice-wiki/modules/public-site/internal/transport"
)

// ModuleSlug 是本域固定的服务身份。
const ModuleSlug = "public-site"

// BuildApplication 校验启动配置、恢复换目录和权威依赖，并挂载已实现的公开路由；
// 预览、屏障、快照等尚未实现的操作不注册。
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
	if err = requireEndpoints(config); err != nil {
		return nil, err
	}
	runtimeClient, err := newRuntimeClient(config, key)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeClient)
	if err != nil {
		return nil, err
	}
	clients := assembleClients(config, key)
	if err = clients.ready(); err != nil {
		return nil, err
	}
	application := transport.NewApplication(config, runtimeClient, view.DataEpoch, true, true, true)
	if err = transport.BindRoutes(application, anchors, clients.directory()); err != nil {
		return nil, err
	}
	return application, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端。
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
	if err := requireEndpoint(config.Dependencies["site-config"]); err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

type domainClients struct {
	content      *gen.ContentClient
	siteConfig   *gen.SiteConfigClient
	localization *gen.LocalizationClient
	identity     *gen.IdentityClient
	media        *gen.MediaClient
	analytics    *gen.AnalyticsClient
	qaBase       string
	qaKey        platform.SigningKey
}

func assembleClients(config platform.ServiceConfig, key platform.SigningKey) domainClients {
	clients := domainClients{
		content:      gen.NewContentClient(config.Dependencies["content"], ModuleSlug, key),
		siteConfig:   gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key),
		localization: gen.NewLocalizationClient(config.Dependencies["localization"], ModuleSlug, key),
		identity:     gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key),
		media:        gen.NewMediaClient(config.Dependencies["media"], ModuleSlug, key),
		qaBase:       config.Dependencies["qa"],
		qaKey:        key,
	}
	if config.Dependencies["analytics"] != "" {
		clients.analytics = gen.NewAnalyticsClient(config.Dependencies["analytics"], ModuleSlug, key)
	}
	return clients
}

func (c domainClients) ready() error {
	if c.content == nil || c.siteConfig == nil || c.localization == nil || c.identity == nil || c.media == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "权威依赖客户端未装配")
	}
	// analytics 是可选依赖，允许保持 nil；读取只为保留装配结果。
	_ = c.analytics
	return nil
}

func (c domainClients) directory() publicsite.LinkDirectory {
	return publicsite.LinkDirectory{
		SiteConfig:   c.siteConfig,
		Content:      c.content,
		Localization: c.localization,
		Media:        c.media,
		Identity:     c.identity,
		QA:           publicsite.NewQAProbe(c.qaBase, ModuleSlug, c.qaKey),
	}
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
	if config.Dependencies == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "公开站点权威依赖不可用")
	}
	return nil
}

func requireEndpoints(config platform.ServiceConfig) error {
	required := []string{"content", "site-config", "localization", "identity", "media"}
	for _, name := range required {
		if err := requireEndpoint(config.Dependencies[name]); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "缺少%s依赖地址", name)
		}
	}
	if raw := config.Dependencies["analytics"]; raw != "" {
		if err := requireEndpoint(raw); err != nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "analytics依赖地址不合法")
		}
	}
	return nil
}

func requireEndpoint(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "权威依赖地址不可用")
	}
	return nil
}

func ensureDataLayout(dataRoot string) error {
	for _, name := range []string{"active", "control"} {
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
	if err = json.Unmarshal(raw, &anchors); err != nil || len(anchors.Services) == 0 {
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
	if err = json.Unmarshal(raw, &key); err != nil || key.KeyID == "" || !filepath.IsAbs(key.PrivateKeyRef) {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不合法")
	}
	if _, err = serviceauth.LoadSigningKey(key); err != nil {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名私钥不可用")
	}
	return key, nil
}
