package mapping

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	memorycontext "nice-wiki/modules/memory/internal/context"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type ReorderMappingsInput = gen.ReorderMappingsInput
type ReorderMappingsResult = gen.ReorderMappingsResult

func ReorderMappings(ctx context.Context, op OperationContext, input ReorderMappingsInput) (ReorderMappingsResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return ReorderMappingsResult{}, err
	}
	if err := memoryruntime.ValidateTask(input.TaskId); err != nil {
		return ReorderMappingsResult{}, err
	}
	var result ReorderMappingsResult
	err := memoryruntime.Write(ctx, op, "memory.ReorderMappings", input.IdempotencyKey, input, &result, func(s *memoryruntime.Store, tx *sql.Tx) error {
		var revision int
		if err := tx.QueryRowContext(ctx, `SELECT mapping_revision FROM runtime_state WHERE id=1`).Scan(&revision); err != nil {
			return memoryruntime.DBError(err)
		}
		if revision != input.ExpectedMappingRevision {
			return platform.NewError(platform.CodeVersionMismatch, false, "映射全局版本已变化")
		}
		all, err := memorycontext.ReadMappings(ctx, tx, op.Actor.SiteID)
		if err != nil {
			return err
		}
		byID := map[string]gen.TaskMapping{}
		for _, m := range all {
			if m.TaskId == input.TaskId {
				byID[m.MappingId] = m
			}
		}
		if len(byID) != len(input.MappingIds) {
			return platform.NewError(platform.CodeValidation, false, "映射重排必须包含完整集合")
		}
		result.Mappings = []gen.TaskMapping{}
		for i, id := range input.MappingIds {
			m, ok := byID[id]
			if !ok {
				return platform.NewError(platform.CodeValidation, false, "映射重排有重复或未知条目")
			}
			delete(byID, id)
			m.Order = i * 10
			m.Revision++
			_, err = tx.ExecContext(ctx, `UPDATE task_mappings SET order_value=?,revision=? WHERE site_id=? AND mapping_id=? AND deleted=0`, m.Order, m.Revision, op.Actor.SiteID, id)
			if err != nil {
				return memoryruntime.DBError(err)
			}
			result.Mappings = append(result.Mappings, m)
		}
		b, err := memorycontext.ReadTaskBudget(ctx, tx, op.Actor.SiteID, input.TaskId)
		if err != nil {
			return err
		}
		if _, err = memorycontext.ValidateProjectedBudgets(ctx, s, tx, op.Actor.SiteID, nil, nil, &b); err != nil {
			return err
		}
		result.MappingRevision, err = advance(ctx, tx)
		if err != nil {
			return err
		}
		return memorycontext.EmitContextChanged(ctx, tx, op, []string{input.TaskId})
	})
	return result, err
}
