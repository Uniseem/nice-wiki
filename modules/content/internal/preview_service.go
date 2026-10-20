package content

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"sync"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/content/internal/gen"
)

type CreatePreviewGrantInput = gen.CreatePreviewGrantInput
type CreatePreviewGrantResult = gen.CreatePreviewGrantResult
type ValidatePreviewGrantInput = gen.ValidatePreviewGrantInput
type ValidatePreviewGrantResult = gen.ValidatePreviewGrantResult

// 预览授权只存在内存里：有效期不超过 30 分钟，进程重启后失效，管理员重新点预览即可。
type previewRecord struct {
	grant     gen.PreviewGrant
	tokenHash [32]byte
	siteID    Identifier
	epoch     Identifier
}

var previews = struct {
	sync.Mutex
	byID map[Identifier]previewRecord
}{byID: map[Identifier]previewRecord{}}

const previewLifetime = 30 * time.Minute

// CreatePreviewGrant 给当前管理员签发草稿预览授权；草稿必须是文档当前共享草稿。
func CreatePreviewGrant(ctx context.Context, op OperationContext, input CreatePreviewGrantInput) (CreatePreviewGrantResult, error) {
	runtime, err := draftRuntimeFromContext(ctx)
	if err != nil {
		return CreatePreviewGrantResult{}, err
	}
	if _, err = authorizeEditLock(ctx, op, runtime.Identity, "content.preview", "createPreviewGrant", input.DocumentId); err != nil {
		return CreatePreviewGrantResult{}, err
	}
	_, draft, err := runtime.Store.readDraftSnapshot(ctx, op, input.DocumentId)
	if err != nil {
		return CreatePreviewGrantResult{}, err
	}
	if draft.RevisionId != input.DraftRevisionId {
		return CreatePreviewGrantResult{}, platform.NewError(platform.CodeVersionMismatch, false, "预览的草稿已不是当前草稿")
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return CreatePreviewGrantResult{}, platform.NewError(platform.CodeInternal, false, "生成预览令牌失败")
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	grant := gen.PreviewGrant{PreviewId: platform.NewIdentifier(), DocumentId: input.DocumentId, DraftRevisionId: input.DraftRevisionId,
		HolderAccountId: op.Actor.SubjectID, ExpiresAt: time.Now().UTC().Add(previewLifetime), Token: token}
	previews.Lock()
	defer previews.Unlock()
	now := time.Now()
	for id, record := range previews.byID {
		if record.grant.ExpiresAt.Before(now) {
			delete(previews.byID, id)
		}
	}
	previews.byID[grant.PreviewId] = previewRecord{grant: grant, tokenHash: sha256.Sum256([]byte(token)), siteID: op.Actor.SiteID, epoch: op.DataEpoch}
	return CreatePreviewGrantResult{Grant: grant}, nil
}

// ValidatePreviewGrant 每次渲染预览都重验：同一管理员、未过期、令牌一致、草稿仍是当前草稿。
func ValidatePreviewGrant(ctx context.Context, op OperationContext, input ValidatePreviewGrantInput) (ValidatePreviewGrantResult, error) {
	runtime, err := draftRuntimeFromContext(ctx)
	if err != nil {
		return ValidatePreviewGrantResult{}, err
	}
	if op.Actor.Kind != "admin" || input.AccountId != op.Actor.SubjectID {
		return ValidatePreviewGrantResult{}, platform.NewError(platform.CodeForbidden, false, "预览只对签发它的管理员有效")
	}
	previews.Lock()
	record, found := previews.byID[input.PreviewId]
	previews.Unlock()
	digest := sha256.Sum256([]byte(input.Token))
	if !found || subtle.ConstantTimeCompare(digest[:], record.tokenHash[:]) != 1 || record.grant.HolderAccountId != op.Actor.SubjectID ||
		record.siteID != op.Actor.SiteID || record.epoch != op.DataEpoch || time.Now().After(record.grant.ExpiresAt) {
		return ValidatePreviewGrantResult{}, platform.NewError(platform.CodeNotFound, false, "预览不存在或已过期")
	}
	metadata, draft, err := runtime.Store.readDraftSnapshot(ctx, op, record.grant.DocumentId)
	if err != nil {
		return ValidatePreviewGrantResult{}, err
	}
	if draft.RevisionId != record.grant.DraftRevisionId {
		return ValidatePreviewGrantResult{}, platform.NewError(platform.CodeStaleResult, false, "草稿已更新，请重新预览")
	}
	files := runtime.Store.ConfigureRevisionStorage(RevisionStorageConfig{})
	body, err := ReadRevisionBody(WithRevisionFileStore(ctx, files), op, ReadRevisionBodyInput{DocumentID: record.grant.DocumentId, RevisionID: draft.RevisionId, ExpectedHash: &draft.BodyHash})
	if err != nil {
		return ValidatePreviewGrantResult{}, err
	}
	return ValidatePreviewGrantResult{Body: body.Body, Metadata: metadata}, nil
}
