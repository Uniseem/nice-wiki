package lifecycle

import (
	"context"
	"nice-wiki/libs/platform"
)

type snapshotStageScopeKey struct{}
type snapshotStageScope struct {
	manager *SnapshotManager
	input   SnapshotStageInput
	root    string
}

// SnapshotStageBinding只认Manager已核完整源归档后给MigrateStage的私有隔离scope。
// 返回路径只供公共IO机制核本域staged数据库归属，不授权active写/付费。
func SnapshotStageBinding(ctx context.Context) (string, string, string, error) {
	scope, ok := ctx.Value(snapshotStageScopeKey{}).(*snapshotStageScope)
	if !ok {
		return "", "", "", platform.NewError(platform.CodeLocked, false, "恢复迁移缺已认证私有stage scope")
	}
	m := scope.manager
	view, err := m.config.Runtime(platform.WithoutCleanupIssuer(ctx))
	if err != nil {
		return "", "", "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fileOps < 1 || m.record == nil || m.record.Status.State != "safe" || !platform.ValidHash(m.record.SealHash) || view.Phase != platform.PhaseRestoreStaging || !view.Maintenance || view.BarrierID != scope.input.BarrierID || view.RestoreID != scope.input.RestoreID || view.TargetDataEpoch != scope.input.TargetDataEpoch || view.DataEpoch != m.record.Epoch {
		return "", "", "", platform.NewError(platform.CodeLocked, true, "隔离stage迁移运行权威已改变")
	}
	return scope.root, scope.input.TargetDataEpoch, m.record.SealHash, nil
}
