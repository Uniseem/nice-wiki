// Package app 装配 memory 域的目录布局、服务身份信任锚、运行权威
// 客户端与 HTTP 容器；不承载记忆条目、映射或助手业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/memory/internal/gen"
	"nice-wiki/modules/memory/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "memory"

var requiredDependencies = [...]string{
	"identity",
	"model-gateway",
	"localization",
	"jobs",
	"site-config",
}

// BuildApplication 按启动合同完成本域基础装配，并在业务依赖尚未落地
// 时如实返回 DEPENDENCY_UNAVAILABLE，不注册空业务路由或伪造就绪结果。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	if !filepath.IsAbs(config.DataRoot) {
		return nil, platform.NewError(platform.CodeValidation, false, "数据根必须为绝对路径")
	}
	for _, dependency := range requiredDependencies {
		if config.Dependencies[dependency] == "" {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域必需依赖尚未配置")
		}
	}
	if err := ensureDataLayout(config.DataRoot); err != nil {
		return nil, err
	}
	anchors, err := loadTrustAnchors(config.TrustAnchorRef)
	if err != nil {
		return nil, err
	}
	if _, err := loadSigningKey(config.SigningKeyRef); err != nil {
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
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖本域屏障与持久化，尚未实现")
	}

	// 业务库、OpenAPI x-handler、outbox/事件订阅、任务执行器与后台循环
	// 属于后续工作包；合同要求缺少任一绑定时拒绝启动，不能返回空应用。
	_ = anchors
	return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "记忆业务依赖与处理器尚未实现")
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供进程模板读取
// active dataEpoch 与恢复阶段；不访问 memory 私有存储。
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
	return &runtimeAuthority{client: gen.NewSiteConfigClient(baseURL, ModuleSlug, key)}, nil
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
	// site-config readRuntimeState 返回 {"state": RuntimeState}；RuntimeState 的
	// dataEpoch/phase/restoreId/targetDataEpoch/maintenance 字段与 RuntimeView 同名。
	var response struct {
		State platform.RuntimeView `json:"state"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || response.State.DataEpoch == "" || response.State.Phase == "" {
		return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行状态响应不合法")
	}
	return response.State, nil
}

// ensureDataLayout 建立 active/control/restore 三区；control 区不随业务
// 恢复替换，restore 供 ADR0003 换目录协议使用。
func ensureDataLayout(dataRoot string) error {
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

// loadSigningKey 读取签名密钥引用并验证私钥可解析；密钥原文只驻留
// serviceauth 内存缓存，不进入日志或业务配置。
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
