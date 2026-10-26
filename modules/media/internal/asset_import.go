package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strings"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/media/internal/gen"
)

// ImportAssetInput 是内容域导入站点数据时交来的文件。
type ImportAssetInput struct {
	OriginalName string
	CreatedBy    Identifier
	Data         []byte
}

// importKind 按扩展名选上传类别；认不出的当附件。
func importKind(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg":
		return "image"
	case ".mp4", ".webm", ".mov", ".m4v":
		return "video"
	}
	return "attachment"
}

// ImportAsset 以导入者的身份走一遍完整的上传流程（声明 → 分块 → 完成），校验、配额与登记都和后台上传一致。
// 只给内容域的服务身份用；资源登记在 op 的站点下，创建者记为导入者。
func ImportAsset(ctx context.Context, op OperationContext, in ImportAssetInput) (gen.Asset, error) {
	if op.Actor.Kind != "service" || op.CallerService != "content" || !platform.ValidIdentifier(in.CreatedBy) {
		return gen.Asset{}, platform.NewError(platform.CodeForbidden, false, "资源导入只允许内容域代表用户发起")
	}
	if len(in.Data) == 0 || int64(len(in.Data)) > maxUploadBytes {
		return gen.Asset{}, platform.NewError(platform.CodeValidation, false, "导入文件为空或超过上传上限")
	}
	step := func() OperationContext {
		next := op
		id := platform.NewIdentifier()
		next.OperationID, next.RequestID = id, id
		next.Actor = platform.AuthContext{Kind: "admin", SubjectID: in.CreatedBy, SessionID: "import-" + id, SessionRevision: 1, PermissionRevision: 1, SiteID: op.Actor.SiteID, ExpiresAt: op.Actor.ExpiresAt, Grants: []string{"media.upload"}}
		return next
	}
	sum := sha256.Sum256(in.Data)
	begun, err := BeginUpload(ctx, step(), BeginUploadInput{Sha256: hex.EncodeToString(sum[:]), ByteSize: len(in.Data), OriginalName: in.OriginalName, Kind: gen.BeginUploadInputKind(importKind(in.OriginalName))})
	if err != nil {
		return gen.Asset{}, err
	}
	upload := begun.Upload
	for offset := 0; offset < len(in.Data); {
		end := offset + maxChunkBytes
		if end > len(in.Data) {
			end = len(in.Data)
		}
		piece := in.Data[offset:end]
		pieceSum := sha256.Sum256(piece)
		put, err := PutUploadChunk(ctx, step(), PutUploadChunkInput{UploadId: upload.UploadId, ExpectedRevision: upload.Revision,
			Chunk: BinaryChunk{Offset: offset, Length: len(piece), SHA256: hex.EncodeToString(pieceSum[:]), Reader: bytes.NewReader(piece)}})
		if err != nil {
			return gen.Asset{}, err
		}
		upload = put.Upload
		offset = end
	}
	done, err := CompleteUpload(ctx, step(), CompleteUploadInput{UploadId: upload.UploadId, ExpectedRevision: upload.Revision})
	if err != nil {
		return gen.Asset{}, err
	}
	return done.Asset, nil
}
