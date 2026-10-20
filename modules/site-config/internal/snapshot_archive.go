package siteconfig

import (
	"context"
	"io"

	"nice-wiki/libs/platform"
	"nice-wiki/modules/site-config/internal/gen"
)

type SnapshotArchiveReadInput struct {
	SnapshotID gen.Identifier `json:"snapshotId"`
	SHA256     gen.Hash       `json:"sha256"`
	ByteSize   int            `json:"byteSize"`
}

type SnapshotArchiveReadResult struct {
	Reader io.ReadCloser
	Object platform.ObjectRef
}

// ReadSnapshotArchive 只打开本域公共Manager已完成且hash/size绑定的tar归档。
func ReadSnapshotArchive(ctx context.Context, op platform.OperationContext, input SnapshotArchiveReadInput) (SnapshotArchiveReadResult, error) {
	if op.CallerService != "data-protection" {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeForbidden, false, "归档读取仅允许data-protection服务")
	}
	if !platform.ValidIdentifier(platform.Identifier(input.SnapshotID)) || !platform.ValidHash(platform.Hash(input.SHA256)) || input.ByteSize < 1 {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeValidation, false, "归档引用不合法")
	}
	runtime, err := snapshotRuntimeFrom(ctx)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	expected := platform.ObjectRef{
		Owner: string("site-config"), ObjectID: platform.Identifier(input.SnapshotID), Version: 1,
		SHA256: platform.Hash(input.SHA256), ByteSize: int64(input.ByteSize), MediaType: "application/x-tar",
	}
	reader, err := runtime.API.OpenArchive(ctx, string(input.SnapshotID), expected)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	return SnapshotArchiveReadResult{Reader: reader, Object: expected}, nil
}
