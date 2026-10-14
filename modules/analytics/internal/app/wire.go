// Package app 装配 analytics 域的启动依赖与 HTTP 容器；不承载统计、审计、
// 导出或匿名访问业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/analytics/internal/gen"
	"nice-wiki/modules/analytics/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "analytics"

// BuildApplication 验证启动配置，建立本域控制区与业务区，加载服务身份，
// 读取运行权威并执行启动时恢复换目录，然后绑定完整 OpenAPI 路由。
//
// 本工作包不打开 analytics 业务数据库、不启动统计任务，也不实现业务
// handler。未落地的能力由 transport 以 DEPENDENCY_UNAVAILABLE 失败关闭。
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
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执与本域存储尚未实现")
	}
	return transport.NewApplication(config, anchors, runtimeAuthority), nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供进程模板和Ready使用。
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

// newRuntimeClient 只通过 site-config 的版本化HTTP接口读取运行权威。
func newRuntimeClient(config platform.ServiceConfig, key platform.SigningKey) (platform.SiteConfigClient, error) {
	baseURL := config.Dependencies["site-config"]
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return &runtimeAuthority{client: gen.NewSiteConfigClient(baseURL, ModuleSlug, key)}, nil
}

type runtimeAuthority struct {
	client *gen.SiteConfigClient
}

func (r *runtimeAuthority) ReadRuntimeState(ctx context.Context) (platform.RuntimeView, error) {
	raw, err := r.client.ReadRuntimeState(ctx)
	if err != nil {
		return platform.RuntimeView{}, err
	}
	var view platform.RuntimeView
	if err := json.Unmarshal(raw, &view); err != nil {
		return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行状态响应不合法")
	}
	return view, nil
}

// ensureDataLayout 建立 active/control/restore 三区；control区不随业务恢复替换。
func ensureDataLayout(dataRoot string) error {
	if !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	for _, name := range [...]string{"active", "control", "restore"} {
		if err := os.MkdirAll(filepath.Join(dataRoot, name), 0o700); err != nil {
			return platform.NewError(platform.CodeInternal, false, "建立本域数据目录失败")
		}
	}
	return nil
}

// loadTrustAnchors 读取公钥表；不可用时不以占位身份继续启动。
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

// loadSigningKey 读取外部密钥引用并校验私钥可用；只在内存中使用私钥。
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
