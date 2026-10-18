package protection

import (
	"context"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/data-protection/internal/gen"
)

type SnapshotExportRequest = gen.SnapshotExportRequest
type SnapshotExportResult = gen.SnapshotExportResult

// CreateSnapshot只导出本域由公共SnapshotManager固定的安全截点，不递归控制库或既有归档。
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error) {
	if op.CallerService != "data-protection" || op.Actor.Kind != "service" || op.Actor.SubjectID != "data-protection" || op.CostClass != "" {
		return SnapshotExportResult{}, platform.NewError(platform.CodeForbidden, false, "本域快照导出只接受保护服务身份")
	}
	if !platform.ValidIdentifier(request.BackupId) || !platform.ValidIdentifier(request.BarrierId) || request.DataEpoch != op.DataEpoch {
		return SnapshotExportResult{}, platform.NewError(platform.CodeValidation, false, "快照导出绑定不合法")
	}
	store, err := Catalog(op)
	if err != nil {
		return SnapshotExportResult{}, err
	}
	if store.Snapshot == nil {
		return SnapshotExportResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域快照管理器未装配")
	}
	exported, err := store.Snapshot.Export(ctx, lifecycle.SnapshotExportInput{
		BackupID:       request.BackupId,
		BarrierID:      request.BarrierId,
		DataEpoch:      request.DataEpoch,
		IncludeSecrets: request.IncludeSecrets,
	})
	if err != nil {
		return SnapshotExportResult{}, err
	}
	result := SnapshotExportResult{
		Checksum: exported.Checksum,
		Snapshot: gen.SnapshotRef{
			ContainsSecrets:   exported.Snapshot.ContainsSecrets,
			DataEpoch:         exported.Snapshot.DataEpoch,
			DataSchemaVersion: exported.Snapshot.DataSchemaVersion,
			Object: gen.ObjectRef{
				ByteSize:  int(exported.Snapshot.Object.ByteSize),
				MediaType: exported.Snapshot.Object.MediaType,
				ObjectId:  exported.Snapshot.Object.ObjectID,
				Owner:     exported.Snapshot.Object.Owner,
				Sha256:    exported.Snapshot.Object.SHA256,
				Version:   int(exported.Snapshot.Object.Version),
			},
			Owner:             exported.Snapshot.Owner,
			RevisionWatermark: int(exported.Snapshot.RevisionWatermark),
			SnapshotId:        exported.Snapshot.SnapshotID,
		},
	}
	if err = validateSnapshotExportResult(request, "data-protection", result); err != nil {
		return SnapshotExportResult{}, err
	}
	return result, nil
}
