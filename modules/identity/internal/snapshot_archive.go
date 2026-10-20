package identity

import (
	"context"
	"io"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
)

// SnapshotArchiveReadInput只保留生产方登记的完整ObjectRef字段；不接受路径或URL。
type SnapshotArchiveReadInput struct {
	SnapshotID platform.Identifier `json:"snapshotId"`
	SHA256     platform.Hash       `json:"sha256"`
	ByteSize   int64               `json:"byteSize"`
}

// SnapshotArchiveReadResult是x-internal流结果，HTTP层只透传完整tar字节。
type SnapshotArchiveReadResult struct {
	Stream io.ReadCloser
}

// ReadSnapshotArchive只读取本域已完成归档；归档记录、owner、版本、长度和hash
// 均交公共SnapshotManager核验，未知状态失败关闭。
func ReadSnapshotArchive(ctx context.Context, op OperationContext, input SnapshotArchiveReadInput) (SnapshotArchiveReadResult, error) {
	if op.CallerService != "data-protection" || !validSuperadminDelegation(op) {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeForbidden, false, "归档读取需要data-protection及超管委托")
	}
	if !platform.ValidIdentifier(input.SnapshotID) || !platform.ValidHash(input.SHA256) || input.ByteSize < 1024 || input.ByteSize > lifecycle.MaxArchiveBytes {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeValidation, false, "归档引用不合法")
	}
	runtime, err := snapshotRuntimeFromContext(ctx)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	view, err := lifecycle.ReadRuntime(ctx, runtime)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	if !view.Maintenance || (view.Phase != platform.PhaseBackupBarrier && view.Phase != platform.PhaseRestoreStaging) || view.DataEpoch != op.DataEpoch {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeLocked, true, "当前不在身份归档读取屏障")
	}
	manager, err := snapshotManagerFromContext(ctx)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	stream, err := manager.OpenArchive(ctx, string(input.SnapshotID), platform.ObjectRef{
		Owner:    "identity",
		ObjectID: string(input.SnapshotID),
		Version:  1,
		SHA256:   input.SHA256,
		MediaType: "application/x-tar",
		ByteSize: input.ByteSize,
	})
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	return SnapshotArchiveReadResult{Stream: stream}, nil
}

func validSuperadminDelegation(op OperationContext) bool {
	return op.Actor.Kind == "admin" && platform.ValidIdentifier(op.Actor.SubjectID) && platform.ValidIdentifier(op.Actor.SiteID) && platform.ValidIdentifier(op.Actor.SessionID) && hasGrant(op.Actor.Grants, "superadmin")
}
