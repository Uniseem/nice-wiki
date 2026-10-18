// Package mapping 维护任务映射；所有写入投影完整注入预算。
package mapping

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	memorycontext "nice-wiki/modules/memory/internal/context"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type OperationContext = platform.OperationContext
type ListMappingsInput struct {
	TaskId   string  `json:"taskId"`
	Language *string `json:"language,omitempty"`
}
type ListMappingsResult = gen.ListMappingsResult

func ListMappings(ctx context.Context, op OperationContext, input ListMappingsInput) (ListMappingsResult, error) {
	if err := memoryruntime.RequireManager(op); err != nil {
		return ListMappingsResult{}, err
	}
	if err := memoryruntime.ValidateTask(input.TaskId); err != nil {
		return ListMappingsResult{}, err
	}
	result := ListMappingsResult{Mappings: []gen.TaskMapping{}}
	err := memoryruntime.ReadSnapshot(ctx, func(_ *memoryruntime.Store, tx *sql.Tx) error {
		list, err := memorycontext.ReadMappings(ctx, tx, op.Actor.SiteID)
		if err != nil {
			return err
		}
		for _, m := range list {
			if m.TaskId == input.TaskId && (input.Language == nil || m.Language == *input.Language) {
				result.Mappings = append(result.Mappings, m)
			}
		}
		return memoryruntime.DBError(tx.QueryRowContext(ctx, `SELECT mapping_revision FROM runtime_state WHERE id=1`).Scan(&result.MappingRevision))
	})
	return result, err
}
