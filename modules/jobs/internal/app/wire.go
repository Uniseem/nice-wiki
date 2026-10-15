// Package app 装配 jobs 域的目录布局、服务身份信任锚、运行权威客户端
// 与 HTTP 容器；不承载任务、事件、死信、定时或屏障领域规则。
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
	jobs "nice-wiki/modules/jobs/internal"
	"nice-wiki/modules/jobs/internal/gen"
	"nice-wiki/modules/jobs/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "jobs"

// BuildApplication 验证本域启动前置条件，建立 active/control/restore
// 目录布局，加载服务身份材料，读取运行权威并执行恢复换目录。
//
// 任务/事件核心与管理切片已有实现，其余handler及控制/后台装配未齐，不能以空 handler
// 冒充完整路由。因此在完成平台前置装配后按合同返回
// DEPENDENCY_UNAVAILABLE，交由进程守护重试。
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
		// 换入/换回后的回执需要后续屏障与恢复函数提供完整的
		// stageId、manifestHash 和域签名，当前不能开放入口。
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖本域屏障函数，尚未实现")
	}

	// 实际建立已完成管理映射，并由统一工厂核完整规格；缺其他handler
	// 仍拒绝启动，不能把已完成切片裸挂成可用服务。
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	identityBase := config.Dependencies["identity"]
	if identityBase == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "任务管理缺少identity地址")
	}
	identity := gen.NewIdentityClient(identityBase, ModuleSlug, key)
	_, err = apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		OpenedDataEpoch: view.DataEpoch,
		AllowedCallers: map[string][]string{
			"listJobs":       {"portal"},
			"listDeliveries": {"portal"},
			"retryJob":       {"portal"},
		},
		Authorize: apivalidate.NewIdentityAuthorizer(identity),
	}, jobs.NewManagementHandlers(key))
	if err != nil {
		return nil, err
	}
	return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "jobs任务、事件、死信、定时和屏障处理尚未装配")
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供进程模板的运行
// 状态监视使用；不在构造阶段发起网络请求。
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

// ensureDataLayout 建立 active/control/restore 三区；control 区不随
// 业务恢复替换，restore 供 ADR0003 换目录协议使用。
func ensureDataLayout(dataRoot string) error {
	if dataRoot == "" || !filepath.IsAbs(dataRoot) {
		return platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	for _, name := range [...]string{"active", "control", "restore"} {
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

// loadSigningKey 读取密钥引用文件 {keyId, privateKeyRef} 并验证私钥可解析；
// 私钥字节只在平台鉴权缓存内，不写入日志或配置。
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
