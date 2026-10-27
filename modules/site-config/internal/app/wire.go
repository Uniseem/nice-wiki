// Package app 装配 site-config 域的数据目录布局、服务身份信任锚、
// 本域运行权威读取器与 HTTP 容器；不承载站点配置、初始化等业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/pgdb"
	"nice-wiki/libs/platform/serviceauth"
	siteconfig "nice-wiki/modules/site-config/internal"
	"nice-wiki/modules/site-config/internal/gen"
	"nice-wiki/modules/site-config/internal/transport"
)

// ModuleSlug 本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "site-config"

// BuildApplication 验证本域配置，建立 active/control 目录布局，
// 加载服务身份信任锚与签名密钥，读取本域运行权威，
// 最后返回注册了匿名健康/就绪路由的应用容器。
//
// 本域即运行状态权威：运行状态从自身控制库（独立 control.db）读取，
// 不经 HTTP 自调用。控制库不可用时 localRuntime 如实返回
// DEPENDENCY_UNAVAILABLE，装配失败交进程守护重试，不用占位数据冒充权威。
//
// 打开 active 业务库、恢复本域 journal。
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
	runtimeAuthority, err := NewRuntimeClient(config)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	openedEpoch := view.DataEpoch
	activeStore, err := siteconfig.OpenActiveStore(ctx, config.DataRoot, openedEpoch)
	if err != nil {
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	activeStore.ConfigureInitialization(config.PublicBaseURL, config.AllowLocalHTTP)
	homeRuntime := siteconfig.HomeRuntime{Runtime: runtimeAuthority.ReadRuntimeState}
	if base := config.Dependencies["media"]; base != "" {
		homeRuntime.Media = gen.NewMediaClient(base, ModuleSlug, key)
	}
	if base := config.Dependencies["public-site"]; base != "" {
		homeRuntime.PublicLinks = gen.NewPublicSiteClient(base, ModuleSlug, key)
	}
	if base := config.Dependencies["qa"]; base != "" {
		homeRuntime.QA = gen.NewQaClient(base, ModuleSlug, key)
	}
	activeStore.ConfigureCursorKey(key)
	homeRuntime.WriteWindow = lifecycle.SimpleWriteWindow(runtimeAuthority.ReadRuntimeState)
	activeStore.ConfigureHome(homeRuntime)
	startupOperation := platform.OperationContext{
		RequestID:       "site-config-startup",
		OperationID:     "site-config-startup-journal",
		RootOperationID: "site-config-startup-journal",
		CallerService:   ModuleSlug,
		Deadline:        platform.Timestamp(time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339Nano)),
		DataEpoch:       openedEpoch,
		Actor:           platform.AuthContext{Kind: "service", SubjectID: ModuleSlug},
	}
	if _, err := siteconfig.RecoverAnnouncementJournal(ctx, startupOperation, siteconfig.RecoverAnnouncementJournalInput{}); err != nil {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	if _, err := siteconfig.RecoverConfigJournal(ctx, startupOperation, siteconfig.RecoverConfigJournalInput{}); err != nil {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	initialization, err := siteconfig.ReadInitializationStatus(ctx, startupOperation, siteconfig.ReadInitializationStatusInput{})
	if err != nil {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	if initialization.Coordination != nil && initialization.Coordination.Decision == gen.Commit && initialization.Coordination.Phase != gen.InitializationStatePhaseCompleted {
		if _, err := siteconfig.ResumeInitialization(ctx, startupOperation, gen.ResumeInitializationInput{OperationId: *initialization.Coordination.OperationId}); err != nil {
			_ = activeStore.Close()
			_ = closeRuntime(runtimeAuthority)
			return nil, err
		}
	}
	// 首页引用恢复跨全部站点，显式用系统视角。
	if err := activeStore.RecoverHomeReferenceState(pgdb.WithSite(ctx, ""), startupOperation); err != nil {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	identityBase := config.Dependencies["identity"]
	if identityBase == "" {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "身份权威地址未配置")
	}
	identity := gen.NewIdentityClient(identityBase, ModuleSlug, key)
	application, err := transport.NewApplication(config, anchors, runtimeAuthority, activeStore, openedEpoch, apivalidate.NewIdentityAuthorizer(identity))
	if err != nil {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	return application, nil
}

// NewRuntimeClient 构造本域运行权威读取器（platform.SiteConfigClient），
// 供进程模板的运行状态监视使用。site-config是运行状态权威，读取目标
// 是本域控制库而非远端HTTP；它不接收或覆盖激活回执验签锚。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	key := filepath.Clean(config.DataRoot)
	runtimeClientsMu.Lock()
	defer runtimeClientsMu.Unlock()
	if existing, ok := runtimeClients[key]; ok {
		return existing, nil
	}
	store, err := siteconfig.OpenRuntimeStore(context.Background(), config.DataRoot)
	if err != nil {
		return nil, err
	}
	client := &localRuntime{key: key, store: store}
	runtimeClients[key] = client
	return client, nil
}

// localRuntime 从本域控制库读取权威运行状态，供进程模板监视与Ready
// 使用；不通过HTTP自调用，也不返回占位状态。
type localRuntime struct {
	key   string
	store *siteconfig.RuntimeStore
}

func (l *localRuntime) ReadRuntimeState(ctx context.Context) (platform.RuntimeView, error) {
	return l.store.ReadRuntimeView(ctx)
}

// ReadRuntimeStateDetails 为Ready提供revision与本域活动epoch的同源快照。
func (l *localRuntime) ReadRuntimeStateDetails(ctx context.Context) (gen.RuntimeState, error) {
	return l.store.ReadRuntimeState(ctx)
}

// Close 关闭BuildApplication与RunService共享的本地控制库客户端。
func (l *localRuntime) Close() error {
	if l == nil || l.store == nil {
		return nil
	}
	err := l.store.Close()
	runtimeClientsMu.Lock()
	if runtimeClients[l.key] == l {
		delete(runtimeClients, l.key)
	}
	runtimeClientsMu.Unlock()
	return err
}

var (
	runtimeClientsMu sync.Mutex
	runtimeClients   = map[string]*localRuntime{}
)

func closeRuntime(client platform.SiteConfigClient) error {
	if closer, ok := client.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// ensureDataLayout 建立 active/control 两区；control 区承载独立 control.db 与恢复日志。
func ensureDataLayout(dataRoot string) error {
	for _, name := range [...]string{"active", "control"} {
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
