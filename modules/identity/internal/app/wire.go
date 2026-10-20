// Package app 装配 identity 域的数据目录布局、服务身份信任锚、运行
// 权威客户端与 HTTP 容器；不承载账号、会话等业务规则。
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/outbox"
	"nice-wiki/libs/platform/serviceauth"
	"nice-wiki/modules/identity/internal"
	"nice-wiki/modules/identity/internal/gen"
	"nice-wiki/modules/identity/internal/transport"
)

// ModuleSlug 本域服务身份；配置指向其他域时拒绝装配。
const ModuleSlug = "identity"

// BuildApplication 验证本域配置，建立 active/control/restore 目录布局，
// 加载服务身份信任锚与签名密钥，读取运行权威状态并执行恢复换目录，
// 打开active区业务库执行schema迁移与决定日志恢复，最后返回注册了
// 匿名健康/就绪路由的应用容器。
//
// OpenAPI x-handler 绑定、恢复换目录后的会话失效与回执登记、后台
// 任务循环随对应业务工作包逐项接入。
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
		return nil, err
	}
	swap, err := lifecycle.ApplyRestoreSwap(config.DataRoot, view)
	if err != nil {
		return nil, err
	}
	store, err := identity.OpenDatabase(config.DataRoot)
	if err != nil {
		return nil, err
	}
	if err := store.RunMigrations(); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.EnsureOutbox(ctx); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.RecoverJournal(); err != nil {
		store.Close()
		return nil, err
	}
	activationClient := gen.NewSiteConfigClient(config.Dependencies["site-config"], ModuleSlug, key)
	openedEpoch, err := restoreIdentityAfterSwap(ctx, config.DataRoot, view, swap, store, key, activationClient)
	if err != nil {
		store.Close()
		return nil, err
	}
	origin, err := lifecycle.CanonicalPublicOrigin(config.PublicBaseURL)
	if err != nil {
		store.Close()
		return nil, err
	}
	application, err := transport.NewApplication(config, anchors, runtimeAuthority, store, openedEpoch, identity.SessionRuntime{Key: key, Anchors: anchors, PublicOrigin: origin})
	if err != nil {
		store.Close()
		return nil, err
	}
	if config.Dependencies["jobs"] == "" {
		store.Close()
		return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "缺少jobs依赖地址")
	}
	publisher := outbox.NewEventPublisher(gen.NewJobsClient(config.Dependencies["jobs"], ModuleSlug, key))
	if err := application.InstallBackgroundTasks(runtimeAuthority, application.SnapshotManager(), publisher); err != nil {
		store.Close()
		return nil, err
	}
	return application, nil
}

// restoreIdentityAfterSwap只消费公共换目录证据，完成本域恢复会话失效，
// 并用服务身份签名/登记activation receipt；不从私有stage文件解析字段，
// 不续用过期proof或伪造superadmin授权。
type activationReceiptRequest struct {
	ExpectedRevision platform.Revision     `json:"expectedRevision"`
	RestoreID        platform.Identifier   `json:"restoreId"`
	Receipt          activationReceiptBody `json:"receipt"`
}

type activationReceiptBody struct {
	ManifestHash    platform.Hash       `json:"manifestHash"`
	Module          string              `json:"module"`
	RecordedAt      platform.Timestamp  `json:"recordedAt"`
	Signature       string              `json:"signature"`
	StageID         platform.Identifier `json:"stageId"`
	State           string              `json:"state"`
	TargetDataEpoch platform.Identifier `json:"targetDataEpoch"`
}

func restoreIdentityAfterSwap(ctx context.Context, dataRoot string, runtime platform.RuntimeView, swap platform.SwapResult, store *identity.Store, key platform.SigningKey, activationClient *gen.SiteConfigClient) (platform.Identifier, error) {
	if swap.Action == platform.SwapNone {
		return runtime.DataEpoch, nil
	}
	evidence, err := lifecycle.ReadRestoreSwapState(dataRoot, swap.RestoreID)
	if err != nil {
		return "", err
	}
	if !evidence.Known || evidence.StageID == "" || !platform.ValidIdentifier(evidence.StageID) || !platform.ValidHash(evidence.ManifestHash) || !platform.ValidIdentifier(evidence.HostEpoch) || !platform.ValidIdentifier(evidence.TargetEpoch) || runtime.RestoreID != swap.RestoreID || string(runtime.DataEpoch) != evidence.HostEpoch || string(runtime.TargetDataEpoch) != evidence.TargetEpoch || !runtime.Maintenance {
		return "", platform.NewError(platform.CodeIntegrityError, false, "恢复换目录证据与当前控制runtime不一致")
	}
	state := ""
	openedEpoch := platform.Identifier(evidence.HostEpoch)
	switch swap.Action {
	case platform.SwapSwitched:
		if runtime.Phase != platform.PhaseRestoreSwitching || !evidence.Switched || evidence.RolledBack || evidence.Aborted {
			return "", platform.NewError(platform.CodeIntegrityError, false, "恢复换入证据与运行phase不一致")
		}
		state = "switched"
		openedEpoch = platform.Identifier(evidence.TargetEpoch)
		opID := platform.NewIdentifier()
		op := platform.OperationContext{
			RequestID:       opID,
			OperationID:     opID,
			RootOperationID: opID,
			CallerService:   ModuleSlug,
			Actor: platform.AuthContext{
				Kind:      "service",
				SubjectID: ModuleSlug,
			},
			Deadline:  time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano),
			DataEpoch: platform.Identifier(evidence.TargetEpoch),
		}
		restoreCtx := identity.WithIdentityStore(ctx, store)
		restoreCtx = identity.WithIdentityRestoreID(restoreCtx, swap.RestoreID)
		if _, err := identity.InvalidateRestoredSessions(restoreCtx, op, identity.InvalidateRestoredSessionsInput{TargetDataEpoch: platform.Identifier(evidence.TargetEpoch)}); err != nil {
			return "", err
		}
	case platform.SwapRolledBack:
		if runtime.Phase != platform.PhaseRestoreRollingBack || !evidence.RolledBack || evidence.Switched || evidence.Aborted {
			return "", platform.NewError(platform.CodeIntegrityError, false, "恢复换回证据与运行phase不一致")
		}
		state = "rolled-back"
	default:
		return "", platform.NewError(platform.CodeIntegrityError, false, "未知恢复换目录动作")
	}
	if activationClient == nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "site-config激活回执客户端未装配")
	}
	recordedAt := platform.Timestamp(time.Now().UTC().Format(time.RFC3339Nano))
	claims := serviceauth.ActivationReceiptClaims{
		Version:         1,
		RestoreID:       swap.RestoreID,
		Module:          ModuleSlug,
		StageID:         platform.Identifier(evidence.StageID),
		TargetDataEpoch: platform.Identifier(evidence.TargetEpoch),
		State:           state,
		ManifestHash:    platform.Hash(evidence.ManifestHash),
		RecordedAt:      recordedAt,
	}
	signature, err := serviceauth.SignActivationReceipt(key, claims)
	if err != nil {
		return "", err
	}
	activationID := platform.Identifier(platform.HashBytes([]byte("identity.activation." + string(swap.RestoreID) + "." + state)))
	activationOp := platform.OperationContext{
		RequestID:       activationID,
		OperationID:     activationID,
		RootOperationID: activationID,
		CallerService:   ModuleSlug,
		Actor: platform.AuthContext{
			Kind:      "service",
			SubjectID: ModuleSlug,
		},
		Deadline:  time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano),
		DataEpoch: runtime.DataEpoch,
	}
	activationCtx := platform.WithOperationContext(ctx, activationOp)
	rawResult, err := activationClient.RegisterActivation(activationCtx, activationReceiptRequest{
		ExpectedRevision: runtime.Revision,
		RestoreID:        swap.RestoreID,
		Receipt: activationReceiptBody{
			ManifestHash:    claims.ManifestHash,
			Module:          claims.Module,
			RecordedAt:      claims.RecordedAt,
			Signature:       signature,
			StageID:         claims.StageID,
			State:           claims.State,
			TargetDataEpoch: claims.TargetDataEpoch,
		},
	})
	if err != nil {
		return "", err
	}
	var activationResult struct {
		State struct {
			ActivationReceipts []activationReceiptBody `json:"activationReceipts"`
			DataEpoch          platform.Identifier     `json:"dataEpoch"`
			Phase              string                  `json:"phase"`
			RestoreID          *platform.Identifier    `json:"restoreId"`
			TargetDataEpoch    *platform.Identifier    `json:"targetDataEpoch"`
		} `json:"state"`
	}
	if json.Unmarshal(rawResult, &activationResult) != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "激活回执响应不完整")
	}
	expectedPhase := platform.PhaseRestoreSwitching
	if state == "rolled-back" {
		expectedPhase = platform.PhaseRestoreRollingBack
	}
	stateValue := activationResult.State
	if stateValue.DataEpoch != runtime.DataEpoch || stateValue.Phase != expectedPhase ||
		stateValue.RestoreID == nil || *stateValue.RestoreID != swap.RestoreID ||
		stateValue.TargetDataEpoch == nil || *stateValue.TargetDataEpoch != evidence.TargetEpoch {
		return "", platform.NewError(platform.CodeIntegrityError, false, "激活回执响应与运行控制状态不一致")
	}
	recordedAtTime, parseErr := platform.ParseTimestamp(claims.RecordedAt)
	if parseErr != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "激活回执时间不可核对")
	}
	found := false
	for _, receipt := range stateValue.ActivationReceipts {
		storedAt, storedErr := platform.ParseTimestamp(receipt.RecordedAt)
		if storedErr == nil && storedAt.Equal(recordedAtTime) && receipt.ManifestHash == claims.ManifestHash &&
			receipt.Module == claims.Module && receipt.StageID == claims.StageID &&
			receipt.State == claims.State && receipt.TargetDataEpoch == claims.TargetDataEpoch &&
			receipt.Signature == signature {
			found = true
			break
		}
	}
	if !found {
		return "", platform.NewError(platform.CodeIntegrityError, false, "激活回执未在运行状态中持久化")
	}
	return openedEpoch, nil
}

// NewRuntimeClient 构造 site-config 运行权威客户端（platform.SiteConfigClient），
// 供进程模板的运行状态监视使用；不发起网络请求。
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	if config.Module != ModuleSlug {
		return nil, platform.NewError(platform.CodeValidation, false, "配置module与本域服务身份不符")
	}
	base := config.Dependencies["site-config"]
	if base == "" {
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
	if err := lifecycle.ValidatePrivatePath(dataRoot, "directory", true); err != nil {
		return err
	}
	for _, name := range [...]string{"active", "control", "restore"} {
		if name == "control" {
			if err := lifecycle.ValidatePrivatePath(filepath.Join(dataRoot, name), "directory", true); err != nil {
				return err
			}
		}
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
