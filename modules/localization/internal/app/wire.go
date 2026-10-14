// Package app 装配 localization 域的数据目录、服务身份信任锚、
// 运行权威客户端与 HTTP 容器；不承载翻译、术语或模型业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/localization/internal/gen"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
	"nice-wiki/modules/localization/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "localization"

// BuildApplication 验证本域配置，建立 active/control/restore 目录布局，
// 加载服务身份材料，读取运行权威并执行恢复换目录，然后装配 HTTP 容器。
//
// 本启动工作包不创建业务数据库、不绑定翻译与事件 handler，也不启动
// worker。缺少这些合同依赖时必须返回 DEPENDENCY_UNAVAILABLE，不能用
// 空处理器或占位结果伪装成可用服务。
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
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖本域屏障与持久化函数，尚未实现")
	}
	store, err := localizationruntime.OpenStore(config.DataRoot)
	if err != nil {
		return nil, err
	}
	loadInput := localizationruntime.NewLoadRuntimeInput(ModuleSlug, config.DataRoot, store)
	startupOperation := platform.OperationContext{
		RequestID:       platform.NewIdentifier(),
		OperationID:     platform.NewIdentifier(),
		RootOperationID: platform.NewIdentifier(),
		CallerService:   ModuleSlug,
		Deadline:        time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano),
		DataEpoch:       view.DataEpoch,
		CostClass:       platform.CostClassBackground,
	}
	if _, err := localizationruntime.LoadRuntime(ctx, startupOperation, loadInput); err != nil {
		_ = store.Close()
		return nil, err
	}
	_ = store.Close()

	// OpenAPI handler、业务存储、journal、outbox、订阅和任务循环尚未
	// 完整装配。持久化启动校验已完成，但缺少这些合同依赖时仍必须
	// 保持启动失败，避免健康路由掩盖不完整的服务装配。
	_ = transport.NewApplication(config, anchors, runtimeAuthority, view.DataEpoch)
	return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域业务处理器与持久化依赖尚未实现")
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供进程模板与 Ready
// 读取当前 dataEpoch。它只装配客户端，不在构造阶段发起网络请求。
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
	return &runtimeAuthority{
		client: gen.NewSiteConfigClient(baseURL, ModuleSlug, key),
	}, nil
}

// runtimeAuthority 把生成客户端的原始JSON响应适配为 platform.RuntimeView。
type runtimeAuthority struct {
	client *gen.SiteConfigClient
}

func (r *runtimeAuthority) ReadRuntimeState(ctx context.Context) (platform.RuntimeView, error) {
	raw, err := r.client.ReadRuntimeState(ctx)
	if err != nil {
		return platform.RuntimeView{}, err
	}
	var view platform.RuntimeView
	if err := json.Unmarshal(raw, &view); err != nil || view.DataEpoch == "" || view.Phase == "" {
		return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行状态响应不合法")
	}
	return view, nil
}

// ensureDataLayout 建立 active/control/restore 三区；control 区不随业务
// 恢复替换，restore 供 ADR0003 换目录协议使用。
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

// loadTrustAnchors 读取公钥表；文件内容不进入日志。
func loadTrustAnchors(ref string) (platform.TrustAnchors, error) {
	if ref == "" {
		return platform.TrustAnchors{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚不可用")
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

// loadSigningKey 读取签名密钥引用并验证私钥可解析；私钥只驻留内存。
func loadSigningKey(ref string) (platform.SigningKey, error) {
	if ref == "" {
		return platform.SigningKey{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务签名密钥引用不可用")
	}
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
