package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"nice-wiki/libs/platform"
	indexer "nice-wiki/modules/knowledge/internal/index"
)

// ReadBuildPlan 只读原构建首次冻结的真实阶段输入，重启不重新规划模型或分组。
func ReadBuildPlan(ctx context.Context, op OperationContext, id, phase string, target any) (bool, error) {
	runtime, err := indexer.RuntimeFromContext(ctx)
	if err != nil {
		return false, err
	}
	var raw []byte
	var hash string
	err = runtime.DB.QueryRowContext(ctx, `SELECT p.plan_json,p.plan_hash FROM build_execution_plans p JOIN builds b ON b.build_id=p.build_id WHERE p.build_id=? AND p.phase=? AND b.site_id=? AND b.data_epoch=?`, id, phase, op.Actor.SiteID, op.DataEpoch).Scan(&raw, &hash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, platform.NormalizeIOError(err, platform.CodeInternal, false, "原阶段计划不可读")
	}
	if len(raw) > 16<<20 || platform.HashBytes(raw) != hash || json.Unmarshal(raw, target) != nil {
		return false, platform.NewError(platform.CodeIntegrityError, false, "原阶段计划hash或结构损坏")
	}
	return true, nil
}

// FreezeBuildPlan 在当前原租约内只写一次完整阶段计划；配置漂移不换分组绕过未知费用。
func FreezeBuildPlan(ctx context.Context, op OperationContext, id, leaseID, phase string, plan any) error {
	raw, err := platform.CanonicalJSON(plan)
	if err != nil {
		return err
	}
	if len(raw) > 16<<20 {
		return platform.NewError(platform.CodeBudgetExceeded, false, "完整构建阶段计划超过16MiB运行预算")
	}
	work, tx, _, cancel, err := indexer.BeginBuildWrite(ctx, op, id, leaseID)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	_, err = tx.ExecContext(work, `INSERT INTO build_execution_plans(build_id,phase,plan_json,plan_hash) VALUES(?,?,?,?) ON CONFLICT DO NOTHING`, id, phase, raw, platform.HashBytes(raw))
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原阶段计划保存失败")
	}
	var saved []byte
	var hash string
	err = tx.QueryRowContext(work, `SELECT plan_json,plan_hash FROM build_execution_plans WHERE build_id=? AND phase=?`, id, phase).Scan(&saved, &hash)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原阶段计划确认失败")
	}
	if hash != platform.HashBytes(saved) || string(saved) != string(raw) {
		return platform.NewError(platform.CodeConflict, false, "原构建阶段已冻结为不同计划")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "原阶段计划提交失败")
	}
	return nil
}

// SetBuildStage 只在原真实租约下记录已经进入的执行阶段，不制造新的租约或进度。
func SetBuildStage(ctx context.Context, op OperationContext, id, leaseID, stage string) error {
	work, tx, _, cancel, err := indexer.BeginBuildWrite(ctx, op, id, leaseID)
	if err != nil {
		return err
	}
	defer cancel()
	defer tx.Rollback()
	if _, err = tx.ExecContext(work, `UPDATE builds SET state=? WHERE build_id=?`, stage, id); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "真实构建阶段保存失败")
	}
	if err = tx.Commit(); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "真实构建阶段提交失败")
	}
	return nil
}
