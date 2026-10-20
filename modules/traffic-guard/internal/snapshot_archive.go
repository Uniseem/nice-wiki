package trafficguard

import (
	"context"
	"io"

	"nice-wiki/libs/platform"
)

// SnapshotArchiveReadInput 是 data-protection 读取本域完成归档时的精确引用。
// owner、版本和媒体类型由本域固定，不从请求或磁盘路径取得。
type SnapshotArchiveReadInput struct {
	SnapshotID string
	SHA256     platform.Hash
	ByteSize   int64
}

// SnapshotArchiveReadResult 只保留受控归档流和供HTTP响应使用的固定引用。
// 不暴露归档在本机上的路径，也不把流编码成JSON。
type SnapshotArchiveReadResult struct {
	Archive io.ReadCloser
	Object  platform.ObjectRef
}

// ReadSnapshotArchive 只读取平台快照管理器已经写入并完整校验的归档。
// 当前维护阶段、代次和委托身份均由可信运行上下文决定；归档未完整完成时
// 由平台OpenArchive失败关闭。
func ReadSnapshotArchive(ctx context.Context, op OperationContext, input SnapshotArchiveReadInput) (SnapshotArchiveReadResult, error) {
	if op.CallerService != "data-protection" || op.Actor.Kind != "admin" || op.Actor.SubjectID == "" || op.Actor.SiteID == "" || op.Actor.SessionID == "" || op.Actor.SessionRevision < 1 {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeForbidden, false, "快照归档只接受保护服务携带的超管委托")
	}
	runtime, ok := ctx.Value(guardRuntimeKey{}).(GuardRuntime)
	if !ok || runtime.Snapshot == nil || runtime.Runtime == nil {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "风控快照读取依赖未装配")
	}
	view, err := runtime.Runtime(ctx)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	if view.DataEpoch != op.DataEpoch {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeVersionMismatch, false, "快照归档请求代次已变化")
	}
	if !view.Maintenance || view.BarrierID == "" || (view.Phase != platform.PhaseBackupBarrier && view.Phase != platform.PhaseRestoreStaging) || (view.Phase == platform.PhaseRestoreStaging && view.RestoreID == "") {
		return SnapshotArchiveReadResult{}, platform.NewError(platform.CodeLocked, false, "当前不在可读取快照归档的维护屏障")
	}
	expected := platform.ObjectRef{
		Owner:     ModuleSlug,
		ObjectID:  platform.Identifier(input.SnapshotID),
		Version:   1,
		SHA256:    input.SHA256,
		MediaType: "application/x-tar",
		ByteSize:  input.ByteSize,
	}
	archive, err := runtime.Snapshot.OpenArchive(ctx, input.SnapshotID, expected)
	if err != nil {
		return SnapshotArchiveReadResult{}, err
	}
	return SnapshotArchiveReadResult{Archive: archive, Object: expected}, nil
}
