package media

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// BeginUploadInput、BeginUploadResult、ReadUploadInput、ReadUploadResult、
// AbortUploadInput 和 AbortUploadResult 直接使用生成契约类型。
type BeginUploadInput = gen.BeginUploadInput
type BeginUploadResult = gen.BeginUploadResult
type ReadUploadInput struct {
	UploadId Identifier `json:"uploadId"`
}
type ReadUploadResult = gen.ReadUploadResult
type AbortUploadInput struct {
	UploadId Identifier `json:"uploadId"`
}
type AbortUploadResult = gen.AbortUploadResult

type siteConfigReader interface {
	ReadSiteConfig(ctx context.Context) (json.RawMessage, error)
}

// storagePolicyReader 是 identity 的 readSiteStoragePolicy：站点所有者的存储上限和名下站点。
type storagePolicyReader interface {
	ReadSiteStoragePolicy(ctx context.Context, siteId string) (json.RawMessage, error)
}

type uploadSessionStore interface {
	CreateUpload(ctx context.Context, op OperationContext, session gen.UploadSession, quota uploadQuota, requestHash Hash) (gen.UploadSession, error)
	ReadUpload(ctx context.Context, uploadID Identifier) (gen.UploadSession, Identifier, error)
	AbortUpload(ctx context.Context, siteID, uploadID Identifier) error
}

var (
	activeSiteConfigReader   siteConfigReader
	activeStoragePolicy      storagePolicyReader
	activeSiteConfigReaderMu sync.RWMutex
)

// InstallSiteConfigReader 装配 site-config 的只读配置客户端。上传配额只
// 来自版本化 site-config 响应，不从请求或本地默认值推断。
func InstallSiteConfigReader(reader siteConfigReader) error {
	if reader == nil {
		return platform.NewError(platform.CodeValidation, false, "site-config客户端缺失")
	}
	activeSiteConfigReaderMu.Lock()
	activeSiteConfigReader = reader
	activeSiteConfigReaderMu.Unlock()
	return nil
}

// ownerStorage 是 identity 告诉的站点所有者存储上限和名下站点。
type ownerStorage struct {
	SiteId       string   `json:"siteId"`
	StorageBytes int64    `json:"storageBytes"`
	SiteIds      []string `json:"siteIds"`
}

// readOwnerStorage 以本服务身份向 identity 读站点所有者的存储上限；ctx 要带服务身份的操作上下文。
func readOwnerStorage(ctx context.Context, siteID Identifier) (ownerStorage, error) {
	activeSiteConfigReaderMu.RLock()
	reader := activeStoragePolicy
	activeSiteConfigReaderMu.RUnlock()
	if reader == nil {
		return ownerStorage{}, platform.NewError(platform.CodeDependencyUnavailable, true, "用户存储上限未装配")
	}
	raw, err := reader.ReadSiteStoragePolicy(ctx, string(siteID))
	if err != nil {
		return ownerStorage{}, mapBlobStoreError(err, "读取用户存储上限失败")
	}
	var policy ownerStorage
	if json.Unmarshal(raw, &policy) != nil || policy.SiteId != string(siteID) {
		return ownerStorage{}, platform.NewError(platform.CodeIntegrityError, false, "用户存储上限响应不合法")
	}
	return policy, nil
}

// serviceContext 把当前请求换成本服务身份（仍限定在该站点），用来调别的服务。
func serviceContext(ctx context.Context, siteID Identifier) context.Context {
	op, _ := platform.OperationContextFrom(ctx)
	op.CallerService = "media"
	op.Actor = platform.AuthContext{Kind: "service", SubjectID: "media", SiteID: siteID, ExpiresAt: op.Deadline}
	return platform.WithOperationContext(platform.WithPortalProof(ctx, ""), op)
}

// ReadStorageUsage 给后台显示存储用量：本站点已用、所有者名下合计已用和上限。
func ReadStorageUsage(ctx context.Context, op OperationContext) (gen.StorageUsage, error) {
	if err := validateUploadActor(op, "media.upload"); err != nil {
		return gen.StorageUsage{}, err
	}
	store, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return gen.StorageUsage{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体存储未装配")
	}
	policy, err := readOwnerStorage(serviceContext(ctx, op.Actor.SiteID), op.Actor.SiteID)
	if err != nil {
		return gen.StorageUsage{}, err
	}
	site, err := store.ownerStorageUsed(ctx, []string{string(op.Actor.SiteID)})
	if err != nil {
		return gen.StorageUsage{}, err
	}
	owner, err := store.ownerStorageUsed(ctx, policy.SiteIds)
	if err != nil {
		return gen.StorageUsage{}, err
	}
	return gen.StorageUsage{SiteBytes: int(site), OwnerBytes: int(owner), OwnerLimitBytes: int(policy.StorageBytes)}, nil
}

// InstallStoragePolicyReader 装配 identity 客户端：上传时按所有者名下全部站点合计存储用量。
func InstallStoragePolicyReader(reader storagePolicyReader) {
	activeSiteConfigReaderMu.Lock()
	activeStoragePolicy = reader
	activeSiteConfigReaderMu.Unlock()
}

func currentSiteConfigReader() siteConfigReader {
	activeSiteConfigReaderMu.RLock()
	defer activeSiteConfigReaderMu.RUnlock()
	return activeSiteConfigReader
}

type siteConfigQuotaResponse struct {
	Config struct {
		SiteID         Identifier `json:"siteId"`
		ResourceLimits struct {
			MaxUploadBytes       int   `json:"maxUploadBytes"`
			SiteStorageBytes     int64 `json:"siteStorageBytes"`
			MaxUploadConcurrency int   `json:"maxUploadConcurrency"`
		} `json:"resourceLimits"`
	} `json:"config"`
}

// BeginUpload 校验上传声明、读取站点配额并创建 receiving 会话。具体 HTTP
// JSON/权限入口仍由本域 OpenAPI 装配工作包接入；本函数不暴露任何本地路径。
func BeginUpload(ctx context.Context, op OperationContext, input BeginUploadInput) (BeginUploadResult, error) {
	if err := validateUploadActor(op, "media.upload"); err != nil {
		return BeginUploadResult{}, err
	}
	store, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return BeginUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体上传存储未装配")
	}
	if op.DataEpoch != store.openedEpoch {
		return BeginUploadResult{}, platform.NewError(platform.CodeVersionMismatch, false, "上传不属于已打开活动代次")
	}
	requestHash, err := hashBeginUploadRequest(op, input)
	if err != nil {
		return BeginUploadResult{}, err
	}
	original, found, err := store.replayBeginUpload(ctx, op, store.db, requestHash)
	if err != nil {
		return BeginUploadResult{}, err
	}
	if found {
		return BeginUploadResult{Upload: original}, nil
	}
	work, cancel, err := store.mediaWriteContext(ctx, op)
	if err != nil {
		return BeginUploadResult{}, err
	}
	defer cancel()
	name, err := NormalizeOriginalName(NormalizeOriginalNameInput{OriginalName: input.OriginalName})
	if err != nil {
		return BeginUploadResult{}, err
	}
	reader := currentSiteConfigReader()
	if reader == nil {
		return BeginUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "站点资源配额未装配")
	}
	quotaCtx := serviceContext(work, op.Actor.SiteID)
	quota, err := readUploadQuota(quotaCtx, reader, op.Actor.SiteID)
	if err != nil {
		return BeginUploadResult{}, err
	}
	policy, err := readOwnerStorage(quotaCtx, op.Actor.SiteID)
	if err != nil {
		return BeginUploadResult{}, err
	}
	quota.OwnerStorageBytes, quota.OwnerSites = policy.StorageBytes, policy.SiteIds
	session := gen.UploadSession{ExpectedBytes: input.ByteSize, ExpectedSha256: input.Sha256, ExpiresAt: time.Now().UTC().Add(24 * time.Hour), Kind: gen.UploadSessionKind(input.Kind), OriginalName: name.SafeName, ReceivedBytes: 0, Revision: 0, SiteId: op.Actor.SiteID, State: gen.Receiving, UploadId: platform.NewIdentifier()}
	created, err := store.CreateUpload(ctx, op, session, quota, requestHash)
	return BeginUploadResult{Upload: created}, err
}

// ReadUpload 只返回上传状态、offset和期限，不返回临时路径或内部句柄。
func ReadUpload(ctx context.Context, op OperationContext, input ReadUploadInput) (ReadUploadResult, error) {
	workCtx, cancel, err := uploadOperationContext(ctx, op)
	if err != nil {
		return ReadUploadResult{}, err
	}
	defer cancel()
	if !validIdentifier(input.UploadId) || !validIdentifier(op.Actor.SiteID) || !validIdentifier(op.Actor.SubjectID) {
		return ReadUploadResult{}, platform.NewError(platform.CodeValidation, false, "上传查询参数不合法")
	}
	store := currentBlobStore()
	uploadStore, ok := store.(uploadSessionStore)
	if !ok {
		return ReadUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体上传存储尚未装配")
	}
	session, owner, err := uploadStore.ReadUpload(workCtx, input.UploadId)
	if err != nil {
		return ReadUploadResult{}, err
	}
	if session.SiteId != op.Actor.SiteID || (!hasGrant(op.Actor.Grants, "media.manage") &&
		(!hasGrant(op.Actor.Grants, "media.upload") || owner != op.Actor.SubjectID)) {
		return ReadUploadResult{}, platform.NewError(platform.CodeForbidden, false, "无权读取该上传")
	}
	return ReadUploadResult{Upload: session}, nil
}

// AbortUpload 只允许上传者取消 receiving 会话；已完成资源不通过此入口
// 删除，重复取消已结束临时会话保持成功幂等。
func AbortUpload(ctx context.Context, op OperationContext, input AbortUploadInput) (AbortUploadResult, error) {
	workCtx, cancel, err := uploadOperationContext(ctx, op)
	if err != nil {
		return AbortUploadResult{}, err
	}
	defer cancel()
	if err := validateUploadActor(op, "media.upload"); err != nil {
		return AbortUploadResult{}, err
	}
	if !validIdentifier(input.UploadId) {
		return AbortUploadResult{}, platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	store := currentBlobStore()
	uploadStore, ok := store.(uploadSessionStore)
	if !ok {
		return AbortUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体上传存储尚未装配")
	}
	session, owner, err := uploadStore.ReadUpload(workCtx, input.UploadId)
	if err != nil {
		return AbortUploadResult{}, err
	}
	if session.SiteId != op.Actor.SiteID || owner != op.Actor.SubjectID {
		return AbortUploadResult{}, platform.NewError(platform.CodeForbidden, false, "无权取消该上传")
	}
	if err := uploadStore.AbortUpload(workCtx, op.Actor.SiteID, input.UploadId); err != nil {
		return AbortUploadResult{}, err
	}
	return AbortUploadResult{Aborted: true}, nil
}

func validateUploadActor(op OperationContext, grant string) error {
	if op.Actor.Kind != "admin" || op.Actor.SessionID == "" || (!hasGrant(op.Actor.Grants, grant) && !hasGrant(op.Actor.Grants, "superadmin")) {
		return platform.NewError(platform.CodeForbidden, false, "当前管理会话缺少上传权限")
	}
	return nil
}

func hasGrant(grants []string, expected string) bool {
	for _, grant := range grants {
		if grant == expected {
			return true
		}
	}
	return false
}

func readUploadQuota(ctx context.Context, reader siteConfigReader, siteID Identifier) (uploadQuota, error) {
	raw, err := reader.ReadSiteConfig(ctx)
	if err != nil {
		return uploadQuota{}, mapBlobStoreError(err, "读取站点资源配额失败")
	}
	var response siteConfigQuotaResponse
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(&response); err != nil || response.Config.SiteID != siteID {
		return uploadQuota{}, platform.NewError(platform.CodeIntegrityError, false, "站点资源配额响应不合法")
	}
	limits := response.Config.ResourceLimits
	if limits.MaxUploadBytes < 1 || limits.MaxUploadBytes > int(maxUploadBytes) ||
		limits.SiteStorageBytes < 1 || limits.MaxUploadConcurrency < 1 {
		return uploadQuota{}, platform.NewError(platform.CodeIntegrityError, false, "站点资源配额范围不合法")
	}
	return uploadQuota{
		MaxUploadBytes:       limits.MaxUploadBytes,
		SiteStorageBytes:     limits.SiteStorageBytes,
		MaxUploadConcurrency: limits.MaxUploadConcurrency,
	}, nil
}

func hashBeginUploadRequest(op OperationContext, input BeginUploadInput) (Hash, error) {
	raw, err := platform.CanonicalJSON(struct {
		Kind      string           `json:"kind"`
		Caller    string           `json:"caller"`
		Actor     Identifier       `json:"actor"`
		SiteID    Identifier       `json:"siteId"`
		DataEpoch Identifier       `json:"dataEpoch"`
		Input     BeginUploadInput `json:"input"`
	}{"begin_upload", op.CallerService, op.Actor.SubjectID, op.Actor.SiteID, op.DataEpoch, input})
	if err != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "上传业务摘要无法编码")
	}
	return platform.HashBytes(raw), nil
}
