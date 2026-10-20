// Package app 装配 media 域的数据目录、服务身份与运行权威客户端。
// 产品业务处理器和私有存储在后续工作包中接入，本文件不提供伪处理器。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nice-wiki/libs/platform/apivalidate"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
	mediaDomain "nice-wiki/modules/media/internal"
	"nice-wiki/modules/media/internal/gen"
	"nice-wiki/modules/media/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "media"

var requiredDependencies = [...]string{"identity", "site-config", "content", "jobs"}

// BuildApplication 验证启动边界、建立本域目录并接入恢复/运行权威。
//
// 本工作包尚未实现 media 私有存储、OpenAPI 业务处理器、事件订阅和任务
// 执行器。按启动合同，缺少这些约束级依赖时返回
// DEPENDENCY_UNAVAILABLE，不创建空路由或伪造就绪结果。
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error) {
	if ctx == nil {
		return nil, platform.NewError(platform.CodeValidation, false, "启动上下文缺失")
	}
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
	if err := validateDependencyEndpoints(config); err != nil {
		return nil, err
	}
	runtimeClient, err := newRuntimeClient(config, key)
	if err != nil {
		return nil, err
	}
	siteConfig := gen.NewSiteConfigClient(strings.TrimSpace(config.Dependencies["site-config"]), ModuleSlug, key)
	if err := mediaDomain.InstallSiteConfigReader(siteConfig); err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeClient)
	if err != nil {
		return nil, err
	}
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖后续屏障工作包")
	}

	if err := mediaDomain.InstallLocalBlobStore(config.DataRoot, view.DataEpoch); err != nil {
		return nil, err
	}

	complete := false
	defer func() {
		if !complete {
			_ = mediaDomain.CloseLocalBlobStore()
		}
	}()
	if err := mediaDomain.InstallReferenceDecisionClients(gen.NewContentClient(config.Dependencies["content"], ModuleSlug, key), siteConfig); err != nil {
		return nil, err
	}
	jobs := gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key)
	if err := mediaDomain.InstallCleanupJobsClient(jobs); err != nil {
		return nil, err
	}
	if err := mediaDomain.InstallAssetListingKey(key); err != nil {
		return nil, err
	}
	if err := mediaDomain.ConfigureWriteWindow(platform.WriteWindowConfig{Runtime: runtimeClient.ReadRuntimeState, WriteWindow: func(work context.Context) (time.Time, error) {
		current, err := runtimeClient.ReadRuntimeState(work)
		if err != nil {
			return time.Time{}, err
		}
		if current.DataEpoch == "" {
			return time.Time{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体写窗口当前代次未知")
		}
		if current.DataEpoch != view.DataEpoch {
			return time.Time{}, platform.NewError(platform.CodeVersionMismatch, false, "媒体写窗口已切换数据代次")
		}
		if current.Maintenance || current.Phase != platform.PhaseRunning {
			return time.Time{}, platform.NewError(platform.CodeLocked, true, "媒体维护写窗口已关闭")
		}
		return time.Time{}, nil
	}}); err != nil {
		return nil, err
	}
	id := platform.NewIdentifier()
	startup := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, DataEpoch: view.DataEpoch,
		Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system"}}
	if _, err := mediaDomain.RecoverBlobJournal(ctx, startup, mediaDomain.RecoverBlobJournalInput{}); err != nil {
		return nil, err
	}
	if err := mediaDomain.RecoverUploadChunks(ctx, startup); err != nil {
		return nil, err
	}
	identity := gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key)
	application, err := transport.NewApplication(config, anchors, runtimeClient, view.DataEpoch, apivalidate.NewIdentityAuthorizer(identity))
	if err != nil {
		return nil, err
	}
	background, err := mediaDomain.NewMediaBackgroundTasks(startup, jobs)
	if err != nil {
		return nil, err
	}
	application.InstallBackgroundTasks(background)
	complete = true
	return application, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端，供平台进程模板读取
// dataEpoch、维护状态和恢复阶段；不共享 site-config 的内部存储。
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
	baseURL := strings.TrimSpace(config.Dependencies["site-config"])
	if baseURL == "" {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "缺少site-config依赖地址")
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

func validateDependencyEndpoints(config platform.ServiceConfig) error {
	for _, module := range requiredDependencies {
		if strings.TrimSpace(config.Dependencies[module]) == "" {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "缺少本域启动依赖")
		}
	}
	return nil
}

// ensureDataLayout 建立 active/control/restore 三区；control 区不随业务
// 恢复替换，restore 区供平台换目录协议使用。
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

// loadTrustAnchors 读取服务公钥表；引用不可读或内容不完整时阻止启动。
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

// loadSigningKey 只读取签名密钥引用并验证私钥可解析，私钥内容不进入
// 日志或配置结构的序列化结果。
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
