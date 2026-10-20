// Package app 装配 site-config 域的数据目录布局、服务身份信任锚、
// 本域运行权威读取器与 HTTP 容器；不承载站点配置、初始化等业务规则。
package app

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/serviceauth"
	siteconfig "nice-wiki/modules/site-config/internal"
	"nice-wiki/modules/site-config/internal/gen"
	"nice-wiki/modules/site-config/internal/transport"
)

// ModuleSlug 本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "site-config"

// BuildApplication 验证本域配置，建立 active/control/restore 目录布局，
// 加载服务身份信任锚与签名密钥，读取本域运行权威并执行恢复换目录，
// 最后返回注册了匿名健康/就绪路由的应用容器。
//
// 本域即运行状态权威：运行状态从自身控制库（独立 control.db）读取，
// 不经 HTTP 自调用。控制库不可用时 localRuntime 如实返回
// DEPENDENCY_UNAVAILABLE，装配失败交进程守护重试，不用占位数据冒充权威。
//
// 本工作包打开 active 业务库、恢复本域 journal、装配快照/屏障/Relay 与
// 换目录回执；跨域GlobalSeal核验仍由集成启动装配提供，缺失时保持失败关闭。
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
	if err := siteconfig.ConfigureActivationVerifier(anchors); err != nil {
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtimeAuthority)
	if err != nil {
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	openedEpoch := view.DataEpoch
	if swap.Action != platform.SwapNone {
		evidence, evidenceErr := lifecycle.ReadRestoreSwapState(config.DataRoot, swap.RestoreID)
		if evidenceErr != nil {
			_ = closeRuntime(runtimeAuthority)
			return nil, evidenceErr
		}
		if err := registerActivationAfterSwap(ctx, runtimeAuthority, key, view, swap, evidence); err != nil {
			_ = closeRuntime(runtimeAuthority)
			return nil, err
		}
		if swap.Action == platform.SwapSwitched {
			openedEpoch = platform.Identifier(evidence.TargetEpoch)
		}
	}
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
	jobsBase := config.Dependencies["jobs"]
	dataProtectionBase := config.Dependencies["data-protection"]
	if jobsBase == "" || dataProtectionBase == "" {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "快照与事件依赖地址未配置")
	}
	jobsClient := gen.NewJobsClient(jobsBase, ModuleSlug, key)
	dataProtectionClient := gen.NewDataProtectionClient(dataProtectionBase, ModuleSlug, key)
	var relayRuntime siteconfig.RelayRuntime
	snapshotManager, err := siteconfig.NewSnapshotManager(config.DataRoot, openedEpoch, runtimeAuthority.ReadRuntimeState, activeStore, func(work context.Context) error {
		return drainRelay(work, relayRuntime, runtimeAuthority)
	})
	if err != nil {
		_ = activeStore.Close()
		_ = closeRuntime(runtimeAuthority)
		return nil, err
	}
	homeRuntime.WriteWindow = snapshotManager.WriteWindow
	activeStore.ConfigureHome(homeRuntime)
	relayRuntime = siteconfig.RelayRuntime{
		Store:          activeStore.OutboxStore(),
		Publisher:      outbox.NewEventPublisher(jobsClient),
		Runtime:        runtimeAuthority.ReadRuntimeState,
		WriteWindow:    snapshotManager.WriteWindow,
		AdmitBatch:     snapshotManager.AdmitBusiness,
		BindEventScope: snapshotManager.BindOutboxEventScope,
	}
	snapshotRuntime := siteconfig.SnapshotRuntime{
		API: snapshotManager,
		RestoreArchive: func(restoreCtx context.Context, restoreID, owner string) (io.ReadCloser, error) {
			response, responseErr := dataProtectionClient.ServeRestoreArchive(restoreCtx, restoreID, owner)
			if responseErr != nil {
				return nil, responseErr
			}
			return response.Body, nil
		},
	}
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
	if !view.Maintenance && initialization.Coordination != nil && initialization.Coordination.Decision == gen.Commit && initialization.Coordination.Phase != gen.InitializationStatePhaseCompleted {
		if _, err := siteconfig.ResumeInitialization(ctx, startupOperation, gen.ResumeInitializationInput{OperationId: *initialization.Coordination.OperationId}); err != nil {
			_ = activeStore.Close()
			_ = closeRuntime(runtimeAuthority)
			return nil, err
		}
	}
	if err := activeStore.RecoverHomeReferenceState(ctx, startupOperation); err != nil {
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
	application, err := transport.NewApplication(config, anchors, runtimeAuthority, activeStore, snapshotRuntime, relayRuntime, openedEpoch, !view.Maintenance, apivalidate.NewIdentityAuthorizer(identity))
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

func registerActivationAfterSwap(ctx context.Context, runtime platform.SiteConfigClient, key platform.SigningKey, view platform.RuntimeView, swap platform.SwapResult, evidence platform.SnapshotSwapEvidence) error {
	if !evidence.Known || !platform.ValidIdentifier(platform.Identifier(swap.RestoreID)) || !platform.ValidIdentifier(platform.Identifier(evidence.StageID)) || !platform.ValidHash(platform.Hash(evidence.ManifestHash)) || !platform.ValidHash(platform.Hash(evidence.HostManifestHash)) || !platform.ValidIdentifier(platform.Identifier(evidence.HostEpoch)) || !platform.ValidIdentifier(platform.Identifier(evidence.TargetEpoch)) || evidence.HostEpoch != string(view.DataEpoch) || evidence.TargetEpoch != string(view.TargetDataEpoch) || evidence.TargetEpoch == string(view.DataEpoch) {
		return platform.NewError(platform.CodeIntegrityError, false, "恢复换目录证据不完整")
	}
	state := gen.ActivationReceiptState("switched")
	if swap.Action == platform.SwapRolledBack {
		if !evidence.RolledBack || evidence.Switched || evidence.Aborted {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复换回证据动作不一致")
		}
		state = gen.ActivationReceiptState("rolled-back")
	} else if swap.Action == platform.SwapSwitched {
		if !evidence.Switched || evidence.RolledBack || evidence.Aborted {
			return platform.NewError(platform.CodeIntegrityError, false, "恢复切换证据动作不一致")
		}
	} else {
		return platform.NewError(platform.CodeIntegrityError, false, "未知恢复换目录动作")
	}
	opID := platform.Identifier(platform.HashBytes([]byte("site-config.activation." + string(swap.RestoreID) + "." + string(state))))
	op := platform.OperationContext{RequestID: opID, OperationID: opID, RootOperationID: opID, CallerService: ModuleSlug,
		DataEpoch: view.DataEpoch, Deadline: platform.Timestamp(time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)),
		Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system"}}
	current, err := siteconfig.ReadRuntimeState(ctx, op, siteconfig.ReadRuntimeStateInput{})
	if err != nil {
		return err
	}
	for _, existing := range current.State.ActivationReceipts {
		if existing.Module == gen.ActivationReceiptModuleSiteConfig && existing.StageId == gen.Identifier(evidence.StageID) && existing.TargetDataEpoch == gen.Identifier(evidence.TargetEpoch) && existing.State == state && existing.ManifestHash == gen.Hash(evidence.ManifestHash) {
			return nil
		}
	}
	recordedAt := time.Now().UTC()
	claims := serviceauth.ActivationReceiptClaims{
		Version:         1,
		RestoreID:       platform.Identifier(swap.RestoreID),
		Module:          ModuleSlug,
		StageID:         platform.Identifier(evidence.StageID),
		TargetDataEpoch: platform.Identifier(evidence.TargetEpoch),
		State:           string(state),
		ManifestHash:    platform.Hash(evidence.ManifestHash),
		RecordedAt:      platform.Timestamp(recordedAt.Format(time.RFC3339Nano)),
	}
	signature, err := serviceauth.SignActivationReceipt(key, claims)
	if err != nil {
		return err
	}
	_, err = siteconfig.RegisterActivation(ctx, op, gen.RegisterActivationInput{RestoreId: gen.Identifier(swap.RestoreID), ExpectedRevision: gen.Revision(view.Revision), Receipt: gen.ActivationReceipt{
		Module: gen.ActivationReceiptModuleSiteConfig, StageId: gen.Identifier(evidence.StageID), TargetDataEpoch: gen.Identifier(evidence.TargetEpoch), State: state,
		ManifestHash: gen.Hash(evidence.ManifestHash), RecordedAt: recordedAt, Signature: signature,
	}})
	return err
}

func drainRelay(ctx context.Context, runtime siteconfig.RelayRuntime, client platform.SiteConfigClient) error {
	if runtime.Store == nil || runtime.Publisher == nil || runtime.Runtime == nil || runtime.WriteWindow == nil || runtime.AdmitBatch == nil || runtime.BindEventScope == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "事件转发排空依赖未装配")
	}
	for {
		view, err := client.ReadRuntimeState(ctx)
		if err != nil {
			return err
		}
		id := platform.NewIdentifier()
		op := platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: ModuleSlug,
			DataEpoch: view.DataEpoch, Deadline: platform.Timestamp(time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)),
			Actor: platform.AuthContext{Kind: "service", SubjectID: ModuleSlug, SiteID: "system"}}
		result, err := siteconfig.RelayOutbox(siteconfig.WithRelayRuntime(platform.WithOperationContext(ctx, op), runtime), op, siteconfig.RelayOutboxInput{BatchLimit: 100})
		if err != nil {
			return err
		}
		if result.Pending == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
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
