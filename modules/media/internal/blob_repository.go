package media

import (
	"context"
	"errors"
	"unicode/utf8"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// CommitAssetBlobInput 是已完成校验的上传对象提交输入。
type CommitAssetBlobInput struct {
	UploadID  Identifier `json:"uploadId"`
	SHA256    Hash       `json:"sha256"`
	MediaType string     `json:"mediaType"`
}

// CommitAssetBlobResult 是不可变资源提交结果。
type CommitAssetBlobResult struct {
	Asset gen.Asset `json:"asset"`
}

// blobCommitRequest 是私有存储提交边界；它不暴露操作系统路径。
// 实现必须在同一 site 的 sha256 命名空间内核对已有对象字节，临时对象
// fsync 后执行同文件系统 rename 和目录 fsync，并在元数据事务成功前不返回
// ready 资源。具体本地目录、数据库和 journal 由后续存储装配提供。
type blobCommitRequest struct {
	SiteID    Identifier
	UploadID  Identifier
	SHA256    Hash
	MediaType string
	ByteSize  int
}

// blobStore 是 media 私有存储与本函数之间的最小边界。它不允许调用方传入
// 任意路径，也不连接外部对象存储；真实实现由本域后续存储工作包装配。
type blobStore interface {
	OpenUpload(ctx context.Context, uploadID Identifier) (ioReadCloser, error)
	CommitVerifiedBlob(ctx context.Context, request blobCommitRequest) (gen.Asset, error)
}

// ioReadCloser 只保留 HashUpload 所需的受限读取能力，避免把文件句柄或路径
// 泄露到函数合同之外。
type ioReadCloser interface {
	Read([]byte) (int, error)
	Close() error
}

// activeBlobStore 由本域存储装配在接受业务请求前设置；nil 时所有依赖该
// 存储的函数都返回 DEPENDENCY_UNAVAILABLE，不能伪造提交成功。
var activeBlobStore blobStore

// CommitAssetBlob 将已验证上传提交到不可变内容寻址边界。哈希在提交前再次
// 从临时对象读取，防止调用方仅凭声明的 SHA 绕过校验；文件落盘、journal
// 和资源元数据事务由 blobStore 的私有实现原子完成。
func CommitAssetBlob(ctx context.Context, op OperationContext, input CommitAssetBlobInput) (CommitAssetBlobResult, error) {
	workCtx, cancel, err := uploadOperationContext(ctx, op)
	if err != nil {
		return CommitAssetBlobResult{}, err
	}
	defer cancel()

	if !validIdentifier(input.UploadID) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeValidation, false, "上传标识不合法")
	}
	if !validHash(input.SHA256) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传摘要不合法")
	}
	if !validMediaType(input.MediaType) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeValidation, false, "媒体类型不合法")
	}
	if !validIdentifier(op.Actor.SiteID) {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeValidation, false, "站点标识不合法")
	}

	store := activeBlobStore
	if store == nil {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储尚未装配")
	}

	hashResult, err := HashUpload(workCtx, op, HashUploadInput{UploadID: input.UploadID})
	if err != nil {
		return CommitAssetBlobResult{}, err
	}
	if hashResult.SHA256 != input.SHA256 {
		return CommitAssetBlobResult{}, platform.NewError(platform.CodeIntegrityError, false, "上传对象摘要与声明不一致")
	}

	select {
	case <-workCtx.Done():
		return CommitAssetBlobResult{}, contextError(workCtx.Err())
	default:
	}
	asset, err := store.CommitVerifiedBlob(workCtx, blobCommitRequest{
		SiteID:    op.Actor.SiteID,
		UploadID:  input.UploadID,
		SHA256:    input.SHA256,
		MediaType: input.MediaType,
		ByteSize:  hashResult.ByteSize,
	})
	if err != nil {
		return CommitAssetBlobResult{}, mapBlobStoreError(err, "提交不可变资源失败")
	}
	if err := validateCommittedAsset(asset, op.Actor.SiteID, input, hashResult.ByteSize); err != nil {
		return CommitAssetBlobResult{}, err
	}
	return CommitAssetBlobResult{Asset: asset}, nil
}

func validateCommittedAsset(asset gen.Asset, siteID Identifier, input CommitAssetBlobInput, byteSize int) error {
	if asset.SiteId != siteID || asset.State != gen.AssetStateReady ||
		!validIdentifier(asset.Ref.AssetId) || asset.Ref.Sha256 != input.SHA256 ||
		asset.Ref.ByteSize != byteSize || asset.Ref.MediaType != input.MediaType {
		return platform.NewError(platform.CodeIntegrityError, false, "不可变资源提交回执不一致")
	}
	if !validIdentifier(asset.CreatedBy) || !utf8ValidAndNonEmpty(asset.OriginalName) ||
		!validAssetKind(asset.Kind) || asset.CreatedAt.IsZero() || asset.Revision < 0 {
		return platform.NewError(platform.CodeIntegrityError, false, "资源元数据回执不完整")
	}
	return nil
}

func validAssetKind(kind gen.AssetKind) bool {
	switch kind {
	case gen.AssetKindAttachment, gen.AssetKindDrawingScene, gen.AssetKindDrawingSvg,
		gen.AssetKindImage, gen.AssetKindPlatformCover, gen.AssetKindVideo:
		return true
	default:
		return false
	}
}

func utf8ValidAndNonEmpty(value string) bool {
	return value != "" && utf8.ValidString(value)
}

func mapBlobStoreError(err error, fallback string) error {
	if err == nil {
		return nil
	}
	var platformErr *platform.Error
	if errors.As(err, &platformErr) {
		return platformErr
	}
	return platform.NewError(platform.CodeDependencyUnavailable, true, fallback)
}
