package internal

import (
	"context"
	"io"

	"nice-wiki/libs/platform"
)

type SnapshotArchiveReadInput struct {
	SnapshotID string
	SHA256     string
	ByteSize   int64
}

type SnapshotArchiveReadResult struct {
	Reader io.ReadCloser
	Object platform.ObjectRef
}

// ReadSnapshotArchive只读取platform SnapshotManager确认完成且引用完全匹配的归档。
func ReadSnapshotArchive(ctx context.Context, op OperationContext, input SnapshotArchiveReadInput) (SnapshotArchiveReadResult, error) {
	if op.CallerService != "data-protection" {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可以读取jobs快照归档")
	}
	if !platform.ValidIdentifier(input.SnapshotID) || !platform.ValidHash(input.SHA256) || input.ByteSize < 1 {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeValidation, false, "快照归档引用不合法")
	}
	store := currentStore()
	if store == nil {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	manager, _, err := store.snapshotClients()
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	object := platform.ObjectRef{Owner: "jobs", ObjectID: input.SnapshotID, Version: 1, SHA256: input.SHA256, MediaType: "application/x-tar", ByteSize: input.ByteSize}
	reader, err := manager.OpenArchive(ctx, input.SnapshotID, object)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	return SnapshotArchiveReadResult{Reader: reader, Object: object}, nil
}
