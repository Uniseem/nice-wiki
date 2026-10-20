package internal

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
)

// StageSnapshot把已认证data-protection归档流交给同一SnapshotManager暂存。
func StageSnapshot(ctx context.Context, op OperationContext, input SnapshotStageRequest) (SnapshotStageResult, error) {
	if op.CallerService != "data-protection" {
		return SnapshotStageResult{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可以暂存jobs快照")
	}
	store := currentStore()
	if store == nil {
		return SnapshotStageResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	manager, client, err := store.snapshotClients()
	if err != nil {
		return SnapshotStageResult{}, err
	}
	if client == nil {
		return SnapshotStageResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "data-protection归档读取客户端尚未装配")
	}
	response, err := client.ServeRestoreArchive(platform.WithOperationContext(ctx, op), input.RestoreId, "jobs")
	if err != nil {
		return SnapshotStageResult{}, err
	}
	defer response.Body.Close()
	result, err := manager.Stage(ctx, lifecycle.SnapshotStageInput{RestoreID: input.RestoreId, Snapshot: snapshotRefToLifecycle(input.Snapshot), BarrierID: input.BarrierId, TargetDataEpoch: input.TargetDataEpoch, Checksum: input.Checksum}, response.Body)
	if err != nil {
		return SnapshotStageResult{}, err
	}
	return SnapshotStageResult{RestoreId: result.RestoreID, Owner: result.Owner, SnapshotId: result.SnapshotID, TargetDataEpoch: result.TargetDataEpoch, Checksum: result.Checksum, Staged: result.Staged, StageId: optionalString(result.StageID), ManifestHash: optionalHash(result.ManifestHash)}, nil
}

// DiscardSnapshot只允许删除本次恢复范围内的staged或before目录。
func DiscardSnapshot(ctx context.Context, op OperationContext, input SnapshotDiscardRequest) (SnapshotDiscardResult, error) {
	if op.CallerService != "data-protection" {
		return SnapshotDiscardResult{}, platform.NewError(platform.CodeForbidden, false, "仅data-protection可以丢弃jobs恢复目录")
	}
	store := currentStore()
	if store == nil {
		return SnapshotDiscardResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "任务存储尚未装配")
	}
	manager, _, err := store.snapshotClients()
	if err != nil {
		return SnapshotDiscardResult{}, err
	}
	scope, ok := input.Scope.(string)
	if !ok || (scope != "staged" && scope != "before") {
		return SnapshotDiscardResult{}, platform.NewError(platform.CodeValidation, false, "恢复目录范围不合法")
	}
	result, err := manager.Discard(ctx, lifecycle.SnapshotDiscardInput{RestoreID: input.RestoreId, Scope: scope})
	if err != nil {
		return SnapshotDiscardResult{}, err
	}
	return SnapshotDiscardResult{RestoreId: result.RestoreID, Scope: result.Scope, Removed: result.Removed}, nil
}
