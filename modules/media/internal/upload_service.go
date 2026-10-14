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

type uploadSessionStore interface {
	CreateUpload(ctx context.Context, session gen.UploadSession, createdBy Identifier, quota uploadQuota, operationID Identifier, requestHash Hash) (gen.UploadSession, error)
	ReadUpload(ctx context.Context, uploadID Identifier) (gen.UploadSession, Identifier, error)
	AbortUpload(ctx context.Context, siteID, uploadID Identifier) error
}

var (
	activeSiteConfigReader   siteConfigReader
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
	workCtx, cancel, err := uploadOperationContext(ctx, op)
	if err != nil {
		return BeginUploadResult{}, err
	}
	defer cancel()
	if err := validateUploadActor(op, "media.upload"); err != nil {
		return BeginUploadResult{}, err
	}
	if !validHash(input.Sha256) || input.ByteSize < 1 || input.ByteSize > int(maxUploadBytes) || !validUploadKind(input.Kind) {
		return BeginUploadResult{}, platform.NewError(platform.CodeValidation, false, "上传声明不合法")
	}
	name, err := NormalizeOriginalName(NormalizeOriginalNameInput{OriginalName: input.OriginalName})
	if err != nil {
		return BeginUploadResult{}, err
	}
	requestHash, err := hashBeginUploadRequest(op.Actor.SiteID, input, name.SafeName)
	if err != nil {
		return BeginUploadResult{}, err
	}
	reader := currentSiteConfigReader()
	if reader == nil {
		return BeginUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "site-config配置读取能力尚未装配")
	}
	quota, err := readUploadQuota(workCtx, reader, op.Actor.SiteID)
	if err != nil {
		return BeginUploadResult{}, err
	}
	store := currentBlobStore()
	uploadStore, ok := store.(uploadSessionStore)
	if !ok {
		return BeginUploadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体上传存储尚未装配")
	}
	session := gen.UploadSession{
		ExpectedBytes:  input.ByteSize,
		ExpectedSha256: input.Sha256,
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
		Kind:           gen.UploadSessionKind(input.Kind),
		OriginalName:   name.SafeName,
		ReceivedBytes:  0,
		Revision:       0,
		SiteId:         op.Actor.SiteID,
		State:          gen.Receiving,
		UploadId:       platform.NewIdentifier(),
	}
	createdSession, err := uploadStore.CreateUpload(workCtx, session, op.Actor.SubjectID, quota, op.OperationID, requestHash)
	if err != nil {
		return BeginUploadResult{}, err
	}
	return BeginUploadResult{Upload: createdSession}, nil
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
	if !validIdentifier(op.OperationID) || !validIdentifier(op.Actor.SiteID) || !validIdentifier(op.Actor.SubjectID) {
		return platform.NewError(platform.CodeValidation, false, "授权主体不合法")
	}
	if !hasGrant(op.Actor.Grants, grant) {
		return platform.NewError(platform.CodeForbidden, false, "主体缺少上传权限")
	}
	if op.Actor.ExpiresAt != "" {
		expiresAt, err := platform.ParseTimestamp(op.Actor.ExpiresAt)
		if err != nil || !expiresAt.After(time.Now()) {
			return platform.NewError(platform.CodeForbidden, false, "授权已过期")
		}
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

func validUploadKind(kind gen.BeginUploadInputKind) bool {
	switch kind {
	case gen.BeginUploadInputKindAttachment, gen.BeginUploadInputKindDrawingScene,
		gen.BeginUploadInputKindDrawingSvg, gen.BeginUploadInputKindImage,
		gen.BeginUploadInputKindPlatformCover, gen.BeginUploadInputKindVideo:
		return true
	default:
		return false
	}
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

func hashBeginUploadRequest(siteID Identifier, input BeginUploadInput, safeName string) (Hash, error) {
	canonical := struct {
		SiteID       Identifier               `json:"siteId"`
		SHA256       Hash                     `json:"sha256"`
		ByteSize     int                      `json:"byteSize"`
		OriginalName string                   `json:"originalName"`
		SafeName     string                   `json:"safeName"`
		Kind         gen.BeginUploadInputKind `json:"kind"`
	}{
		SiteID:       siteID,
		SHA256:       input.Sha256,
		ByteSize:     input.ByteSize,
		OriginalName: input.OriginalName,
		SafeName:     safeName,
		Kind:         input.Kind,
	}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", platform.NewError(platform.CodeIntegrityError, false, "生成上传请求摘要失败")
	}
	return platform.HashBytes(raw), nil
}
