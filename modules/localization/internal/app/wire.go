// Package app 装配 localization 域的数据目录、服务身份信任锚、
// 运行权威客户端与 HTTP 容器；不承载翻译、术语或模型业务规则。
package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/modelclient"
	"nice-wiki/libs/platform/pgdb"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/localization/internal/gen"
	"nice-wiki/modules/localization/internal/glossary"
	interfacecatalog "nice-wiki/modules/localization/internal/interface"
	"nice-wiki/modules/localization/internal/language"
	localizationruntime "nice-wiki/modules/localization/internal/runtime"
	"nice-wiki/modules/localization/internal/suggestion"
	"nice-wiki/modules/localization/internal/translation"
	"nice-wiki/modules/localization/internal/transport"
)

// ModuleSlug 是本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "localization"

// BuildApplication 验证本域配置，建立 active/control/restore 目录布局，
// 加载服务身份材料，读取运行权威并执行恢复换目录，然后装配 HTTP 容器。
//
// 已实现的处理函数全部挂载；事件订阅、outbox 转发和任务循环留待后续。
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
	retain := false
	defer func() {
		if !retain {
			localizationruntime.ClearLanguageGateway()
			_ = store.Close()
		}
	}()
	validators := modelclient.GatewayResponseValidators{}
	for _, name := range []string{"CreateInvocationResult", "CancelInvocationResult", "StreamEvent", "GetInvocationResult", "LookupInvocationResult"} {
		validator, validationErr := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if validationErr != nil {
			return nil, validationErr
		}
		validators[name] = validator
	}
	rawGateway := gen.NewModelGatewayClient(config.Dependencies["model-gateway"], ModuleSlug, key)
	gateway, gatewayErr := modelclient.NewGatewayClient(rawGateway, validators)
	if gatewayErr != nil {
		return nil, gatewayErr
	}
	jobsClient := gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key)
	if err = localizationruntime.ConfigureExecutionAdapters(gateway); err != nil {
		return nil, err
	}
	planValidators := map[string]func(json.RawMessage) error{}
	for _, name := range []string{"GatewayResolveTaskRouteResult", "GatewayEstimateTokensResult", "GatewayQuoteCostResult", "JobsCreateJobResult"} {
		validator, validationErr := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if validationErr != nil {
			return nil, validationErr
		}
		planValidators[name] = validator
	}
	if err = localizationruntime.ConfigurePlanAdapters(rawGateway, jobsClient, planValidators); err != nil {
		return nil, err
	}
	sourceValidators := map[string]func(json.RawMessage) error{}
	for _, name := range []string{"ContentDocumentBody", "ContentReadPublishedRevisionResult", "ContentValidatePublicationsResult", "ContentReadPublicTreeResult"} {
		validator, validationErr := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), name)
		if validationErr != nil {
			return nil, validationErr
		}
		sourceValidators[name] = validator
	}
	if err = localizationruntime.ConfigureSourceClient(gen.NewContentClient(config.Dependencies["content"], ModuleSlug, key), sourceValidators, key); err != nil {
		return nil, err
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	identity := gen.NewIdentityClient(config.Dependencies["identity"], ModuleSlug, key)
	handlers := language.NewLanguageHandlers()
	for _, additional := range []map[string]http.Handler{language.NewSuggestionPolicyHandlers(), glossary.NewGlossaryHandlers(), translation.NewTranslationHandlers(), interfacecatalog.NewInterfaceHandlers(), suggestion.NewPreferenceHandlers(), suggestion.NewSuggestionHandlers()} {
		for id, handler := range additional {
			handlers[id] = handler
		}
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self:    ModuleSlug,
		Anchors: anchors,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		OpenedDataEpoch: view.DataEpoch,
		AdmitBusiness:   lifecycle.SimpleAdmission(runtimeAuthority.ReadRuntimeState, view.DataEpoch),
		Authorize:       apivalidate.NewIdentityAuthorizer(identity),
		ServiceActorCallers: map[string][]string{"readLanguageSuggestionApproval": {"model-gateway"},
			"listLanguages": {"public-site"}, "readInterfaceCatalog": {"public-site"}, "listTranslationTitles": {"public-site"},
		},
		AllowedCallers: map[string][]string{"readLanguageSuggestionPolicy": {"portal"}, "putLanguageSuggestionPolicy": {"portal"}, "readLanguageSuggestionApproval": {"model-gateway"},
			"putLanguage":          {"portal"},
			"setLanguageEnabled":   {"portal"},
			"estimateLanguagePlan": {"portal"},
			"approveLanguagePlan":  {"portal"},
			"listLanguages":        {"portal", "public-site"},
			"putGlossaryTerm":      {"portal"}, "deleteGlossaryTerm": {"portal"}, "listGlossary": {"portal"}, "rollbackAutomaticGlossaryBatch": {"portal"}, "applyApprovedGlossaryProposal": {"memory"},
			"saveLanguagePreference": {"public-site"}, "suggestLanguage": {"portal"}, "readTranslation": {"public-site", "qa"}, "listTranslationTitles": {"public-site"}, "listTranslationProgress": {"portal"}, "createTranslation": {"portal"}, "cancelTranslation": {"portal"}, "readTranslationUnits": {"portal"}, "editTranslationUnit": {"portal"}, "setUnitProtection": {"portal"}, "putInterfaceString": {"portal"}, "readInterfaceCatalog": {"portal", "public-site"}, "requestInterfaceTranslation": {"portal"},
		},
	}, handlers)
	if err != nil {
		return nil, err
	}
	paths := map[string][]string{}
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			paths[path] = append(paths[path], method)
		}
	}
	application := transport.NewApplication(config, anchors, runtimeAuthority, view.DataEpoch)
	// 逐个站点做：补英文、自动翻译。站点列表取自本库里有语言记录的站点。
	application.SetBackgroundTasks([]platform.BackgroundTask{{Name: "localization.auto-translate", Run: func(work context.Context) error {
		return lifecycle.RunRecoveryLoop(work, platform.RecoveryLoopConfig{PollMillis: 10000, Runtime: func(c context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(c, runtimeAuthority)
		}, WriteWindow: func(context.Context) (time.Time, error) { return time.Time{}, nil }, Recover: func(tick context.Context) error {
			sites, err := pgdb.Sites(tick, store.DB(), "languages")
			if err != nil {
				return nil
			}
			for _, site := range sites {
				deadline := time.Now().UTC().Add(30 * time.Second)
				id := platform.NewIdentifier()
				op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug, Deadline: deadline.Format(time.RFC3339Nano), DataEpoch: view.DataEpoch, CostClass: platform.CostClassBackground,
					Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: site, ExpiresAt: deadline.Format(time.RFC3339Nano), Grants: []string{}}}
				scoped := platform.WithOperationContext(platform.WithPortalProof(tick, ""), op)
				// 站点要求：基准语言之外，英文始终启用并自动翻译。
				if err := language.EnsureEnglish(scoped, op); err != nil {
					return err
				}
				if err := translation.AutoTranslate(scoped, op); err != nil {
					return err
				}
			}
			return nil
		}})
	}}})
	retain = true
	application.Mount(handler, paths, func() {
		localizationruntime.ClearLanguageGateway()
		_ = store.Close()
	})
	return application, nil
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
	for _, name := range [...]string{"active", "control"} {
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
