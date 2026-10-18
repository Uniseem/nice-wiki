// Package memorycontext 组装固定版本记忆；不调用模型或外域存储。
package memorycontext

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type OperationContext = platform.OperationContext
type ReadTaskBudgetInput struct {
	TaskId string `json:"taskId"`
}
type ReadTaskBudgetResult struct {
	Budget gen.TaskBudget `json:"budget"`
}
type ListTaskBudgetsInput struct{}
type ListTaskBudgetsResult = gen.ListTaskBudgetsResult
type PutTaskBudgetInput = gen.PutTaskBudgetInput
type PutTaskBudgetResult = gen.PutTaskBudgetResult

func ReadTaskBudget(ctx context.Context, q memoryruntime.Queryer, site, task string) (gen.TaskBudget, error) {
	b := gen.TaskBudget{TaskId: task, MaximumBytes: 5120, MaximumEntries: 64, Revision: 0}
	err := q.QueryRowContext(ctx, `SELECT maximum_bytes,maximum_entries,revision FROM task_budgets WHERE site_id=? AND task_id=?`, site, task).Scan(&b.MaximumBytes, &b.MaximumEntries, &b.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return b, nil
	}
	return b, memoryruntime.DBError(err)
}
func ListTaskBudgets(ctx context.Context, op OperationContext, input ListTaskBudgetsInput) (ListTaskBudgetsResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return ListTaskBudgetsResult{}, err
	}
	result := ListTaskBudgetsResult{Budgets: []gen.TaskBudget{}}
	err := memoryruntime.ReadSnapshot(ctx, func(_ *memoryruntime.Store, tx *sql.Tx) error {
		for _, task := range memoryruntime.TaskIDs {
			b, err := ReadTaskBudget(ctx, tx, op.Actor.SiteID, task)
			if err != nil {
				return err
			}
			result.Budgets = append(result.Budgets, b)
		}
		return nil
	})
	return result, err
}
func PutTaskBudget(ctx context.Context, op OperationContext, input PutTaskBudgetInput) (PutTaskBudgetResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return PutTaskBudgetResult{}, err
	}
	if err := memoryruntime.ValidateTask(input.Budget.TaskId); err != nil {
		return PutTaskBudgetResult{}, err
	}
	if input.Budget.MaximumBytes < 1024 || input.Budget.MaximumBytes > 65536 || input.Budget.MaximumEntries < 1 || input.Budget.MaximumEntries > 64 {
		return PutTaskBudgetResult{}, platform.NewError(platform.CodeValidation, false, "记忆任务预算超出范围")
	}
	var result PutTaskBudgetResult
	err := memoryruntime.Write(ctx, op, "memory.PutTaskBudget", input.IdempotencyKey, input, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		current, err := ReadTaskBudget(ctx, tx, op.Actor.SiteID, input.Budget.TaskId)
		if err != nil {
			return err
		}
		if current.Revision != input.ExpectedRevision {
			return platform.NewError(platform.CodeVersionMismatch, false, "任务预算版本已变化")
		}
		next := input.Budget
		next.Revision = current.Revision + 1
		if _, err = ValidateProjectedBudgets(ctx, s, tx, op.Actor.SiteID, nil, nil, &next); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO task_budgets(site_id,task_id,maximum_bytes,maximum_entries,revision) VALUES(?,?,?,?,?) ON CONFLICT(site_id,task_id) DO UPDATE SET maximum_bytes=excluded.maximum_bytes,maximum_entries=excluded.maximum_entries,revision=excluded.revision`, op.Actor.SiteID, next.TaskId, next.MaximumBytes, next.MaximumEntries, next.Revision)
		if err != nil {
			return memoryruntime.DBError(err)
		}
		_, err = tx.ExecContext(ctx, `UPDATE runtime_state SET memory_generation=memory_generation+1,updated_at=? WHERE id=1`, platform.Now())
		if err != nil {
			return memoryruntime.DBError(err)
		}
		result.Budget = next
		return EmitContextChanged(ctx, tx, op, []string{next.TaskId})
	})
	return result, err
}
