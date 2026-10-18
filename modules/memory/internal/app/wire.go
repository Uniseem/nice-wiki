// Package app 装配 memory 域的目录布局、服务身份信任锚、运行权威
// 客户端与 HTTP 容器；不承载记忆条目、映射或助手业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/modelclient"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
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
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	if swap.Action != platform.SwapNone {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "恢复换目录回执依赖本域屏障与持久化，尚未实现")
	}

	id := platform.NewIdentifier()
	deadline := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
	startup := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, DataEpoch: view.DataEpoch, Deadline: deadline, Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system", ExpiresAt: deadline}}
	if _, err = memoryruntime.LoadRuntime(ctx, startup, memoryruntime.LoadRuntimeInput{ServiceID: ModuleSlug, DataDirectory: config.DataRoot}); err != nil {
		return nil, err
	}
	fail := func(cause error) (*transport.Application, error) {
		if store := memoryruntime.CurrentStore(); store != nil {
			_ = store.Close()
		}
		return nil, cause
	}
	err = memoryruntime.ConfigureWrites(platform.WriteWindowConfig{Runtime: runtimeAuthority.ReadRuntimeState, WriteWindow: func(work context.Context) (time.Time, error) {
		current, readErr := runtimeAuthority.ReadRuntimeState(work)
		if readErr != nil {
			return time.Time{}, readErr
		}
		if current.DataEpoch != view.DataEpoch {
			return time.Time{}, platform.NewError(platform.CodeVersionMismatch, false, "记忆写窗口代次已变化")
		}
		if current.Maintenance || current.Phase != platform.PhaseRunning {
			return time.Time{}, platform.NewError(platform.CodeLocked, true, "维护期间记忆写窗口关闭")
		}
		return time.Time{}, nil
	}})
	if err != nil {
		return fail(err)
	}
	jobs := gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key)
	if err = memoryruntime.ConfigurePublisher(outbox.NewEventPublisher(jobs)); err != nil {
		return fail(err)
	}
	validators := map[string]func(json.RawMessage) error{}
	for _, name := range []string{"CreateInvocationResult", "CancelInvocationResult", "StreamEvent", "ReadTaskAssignmentsResult", "ReadProfilesResult", "JobsCreateJobResult", "JobsRequestCancelResult", "AssistantModelOutput"} {
		validator, validationErr := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if validationErr != nil {
			return fail(validationErr)
		}
		validators[name] = validator
	}
	if err = memoryruntime.ConfigureValidators(validators); err != nil {
		return fail(err)
	}
	rawGateway := gen.NewModelGatewayClient(config.Dependencies["model-gateway"], ModuleSlug, key)
	gateway, gatewayErr := modelclient.NewGatewayClient(rawGateway, modelclient.GatewayResponseValidators{"CreateInvocationResult": validators["CreateInvocationResult"], "CancelInvocationResult": validators["CancelInvocationResult"], "StreamEvent": validators["StreamEvent"]})
	if gatewayErr != nil {
		return fail(gatewayErr)
	}
	if err = memoryruntime.ConfigureGateway(gateway); err != nil {
		return fail(err)
	}
	if err = memoryruntime.ConfigureAssistantClients(jobs, rawGateway); err != nil {
		return fail(err)
	}
	identity := gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key)
	application, err := transport.NewApplication(config, anchors, runtimeAuthority, view.DataEpoch, apivalidate.NewIdentityAuthorizer(identity))
	if err != nil {
		return fail(err)
	}
	return application, nil
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
	return lifecycle.NewSiteConfigClient(config, key)
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
