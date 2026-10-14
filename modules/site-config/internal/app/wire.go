// Package app 装配 site-config 域的数据目录布局、服务身份信任锚、
// 本域运行权威读取器与 HTTP 容器；不承载站点配置、初始化等业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/site-config/internal/transport"
)

// ModuleSlug 本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "site-config"

// BuildApplication 验证本域配置，建立 active/control/restore 目录布局，
// 加载服务身份信任锚与签名密钥，读取本域运行权威并执行恢复换目录，
// 最后返回注册了匿名健康/就绪路由的应用容器。
//
// 本域即运行状态权威：运行状态从自身控制库（独立 control.db，随
// runtime_state_service 工作包落地）读取，不经 HTTP 自调用。控制库
// 读取函数落地前 localRuntime 如实返回 DEPENDENCY_UNAVAILABLE，装配
// 失败交进程守护重试，不用占位数据冒充权威。
//
// 业务存储打开、OpenAPI x-handler 绑定、初始化/公告 journal 恢复、
// 换目录回执登记与后台任务循环随对应业务工作包逐项接入。
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
		// 回执需要 stageId/manifestHash/域签名，只能由暂存与屏障函数
		// （后续工作包）提供；在此之前换入数据拒绝开放入口。
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖暂存/屏障函数，尚未实现")
	}
	return transport.NewApplication(config, anchors, runtimeAuthority), nil
}

// NewRuntimeClient 构造本域运行权威读取器（platform.SiteConfigClient），
// 供进程模板的运行状态监视使用。site-config 是运行状态权威，读取目标
// 是本域控制库而非远端 HTTP，因此不需要自身依赖地址；控制库打开与
// 读取随 runtime_state_service 工作包接入。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	return &localRuntime{}, nil
}

// localRuntime 从本域控制库读取权威运行状态，供进程模板监视与Ready
// 使用；控制库读取（site-config.ReadRuntimeState 的存储实现）落地前
// 如实返回 DEPENDENCY_UNAVAILABLE，不返回占位状态。
type localRuntime struct{}

func (l *localRuntime) ReadRuntimeState(ctx context.Context) (platform.RuntimeView, error) {
	return platform.RuntimeView{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域运行控制库读取尚未实现")
}

// ensureDataLayout 建立 active/control/restore 三区；control 区承载独立
// control.db 与恢复日志，永不随业务恢复替换，restore 供 ADR0003 换目录
// 协议使用。
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
