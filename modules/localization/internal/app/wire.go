// Package app 装配 localization 域的数据目录、服务身份信任锚、
// 运行权威客户端与 HTTP 容器；不承载翻译、术语或模型业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
 "net/http"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/serviceauth"
 "nice-wiki/libs/platform/modelclient"
 "nice-wiki/libs/platform/outbox"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/language"
 "nice-wiki/modules/localization/internal/glossary"
 "nice-wiki/modules/localization/internal/translation"
 "nice-wiki/modules/localization/internal/suggestion"
 interfacecatalog "nice-wiki/modules/localization/internal/interface"
 cacheanswer "nice-wiki/modules/localization/internal/cache-answer"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
	"nice-wiki/modules/localization/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "localization"

// BuildApplication 验证本域配置，建立 active/control/restore 目录布局，
// 加载服务身份材料，读取运行权威并执行恢复换目录，然后装配 HTTP 容器。
//
// 语言配置的三个操作会绑定真实处理函数。其余 OpenAPI 操作、事件订阅、
// outbox 转发和任务循环仍未实现，因此本函数继续失败关闭。
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
	if err := store.SetDataEpoch(ctx, view.DataEpoch); err != nil {
		_ = store.Close()
		return nil, err
	}
	localizationruntime.InstallLanguageGateway(store, runtimeAuthority)
	defer func() {
		localizationruntime.ClearLanguageGateway()
		_ = store.Close()
	}()
	validators:=modelclient.GatewayResponseValidators{}
 for _,name:=range []string{"CreateInvocationResult","CancelInvocationResult","StreamEvent"}{validator,validationErr:=apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec),name);if validationErr!=nil{return nil,validationErr};validators[name]=validator}
 gateway,gatewayErr:=modelclient.NewGatewayClient(gen.NewModelGatewayClient(config.Dependencies["model-gateway"],ModuleSlug,key),validators);if gatewayErr!=nil{return nil,gatewayErr}
 publisher:=outbox.NewEventPublisher(gen.NewJobsClient(config.Dependencies["jobs"],ModuleSlug,key))
 if err=localizationruntime.ConfigureExecutionAdapters(gateway,publisher);err!=nil{return nil,err}
 sourceValidators:=map[string]func(json.RawMessage)error{}
 for _,name:=range []string{"ContentDocumentBody","ContentReadPublishedRevisionResult","ContentValidatePublicationsResult"}{validator,validationErr:=apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec),name);if validationErr!=nil{return nil,validationErr};sourceValidators[name]=validator}
 if err=localizationruntime.ConfigureSourceClient(gen.NewContentClient(config.Dependencies["content"],ModuleSlug,key),sourceValidators,key);err!=nil{return nil,err}
 spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	identity := gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key)
	handlers:=language.NewLanguageHandlers()
 for _,additional:=range []map[string]http.Handler{glossary.NewGlossaryHandlers(),translation.NewTranslationHandlers(),interfacecatalog.NewInterfaceHandlers(),cacheanswer.NewCacheTranslationHandlers(),suggestion.NewPreferenceHandlers()}{for id,handler:=range additional{handlers[id]=handler}}
 _, err = apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		OpenedDataEpoch: view.DataEpoch,
		Authorize:       apivalidate.NewIdentityAuthorizer(identity),
		ServiceActorCallers: map[string][]string{
			"listLanguages": {"public-site"},
		},
		AllowedCallers: map[string][]string{
			"putLanguage":        {"portal"},
			"setLanguageEnabled": {"portal"},
			"listLanguages":      {"portal", "public-site"},
 "putGlossaryTerm":{"portal"},"deleteGlossaryTerm":{"portal"},"listGlossary":{"portal"},"rollbackAutomaticGlossaryBatch":{"portal"},"applyApprovedGlossaryProposal":{"memory"},
 "saveLanguagePreference":{"public-site"},"readTranslation":{"public-site"},"listTranslationProgress":{"portal"},"readTranslationUnits":{"portal"},"editTranslationUnit":{"portal"},"setUnitProtection":{"portal"},"putInterfaceString":{"portal"},"readInterfaceCatalog":{"portal","public-site"},"readCacheAnswerTranslation":{"qa"},
		},
	}, handlers)
	if err != nil {
		return nil, err
	}
	return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "本域其余业务处理器、事件订阅与任务循环尚未实现")
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
	return lifecycle.NewSiteConfigClient(config, key)
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
