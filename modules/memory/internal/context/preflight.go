package memorycontext

import (
	"context"
	"database/sql"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
	"sort"
)

type ValidateProjectedBudgetsInput struct {
	ProposedEntry   *gen.MemoryEntry `json:"proposedEntry,omitempty"`
	ProposedMapping *gen.TaskMapping `json:"proposedMapping,omitempty"`
}
type ValidateProjectedBudgetsResult struct {
	Valid             bool     `json:"valid"`
	AffectedTaskIds   []string `json:"affectedTaskIds"`
	ProjectedBytes    []int    `json:"projectedBytes"`
	OverBudgetTaskIds []string `json:"overBudgetTaskIds"`
}

// ValidateProjection 用与 resolve 同一渲染器投影全部受影响语言，任何超预算均拒绝提交。
func ValidateProjectedBudgets(ctx context.Context, s *memoryruntime.Store, q memoryruntime.Queryer, site string, entry *gen.MemoryEntry, mapping *gen.TaskMapping, budget *gen.TaskBudget) (ValidateProjectedBudgetsResult, error) {
	entries, err := s.ReadAllEntries(ctx, q, site)
	if err != nil {
		return ValidateProjectedBudgetsResult{}, err
	}
	mappings, err := ReadMappings(ctx, q, site)
	if err != nil {
		return ValidateProjectedBudgetsResult{}, err
	}
	affected := map[string]bool{}
	if entry != nil {
		found := false
		for i, e := range entries {
			if e.EntryId == entry.EntryId {
				entries[i] = *entry
				found = true
			}
		}
		if !found {
			entries = append(entries, *entry)
		}
		for _, m := range mappings {
			if m.EntryId == entry.EntryId {
				affected[m.TaskId] = true
			}
		}
	}
	if mapping != nil {
		found := false
		for i, m := range mappings {
			if m.MappingId == mapping.MappingId {
				affected[m.TaskId] = true
				mappings[i] = *mapping
				found = true
			}
		}
		if !found {
			mappings = append(mappings, *mapping)
		}
		affected[mapping.TaskId] = true
	}
	if budget != nil {
		affected[budget.TaskId] = true
	}
	tasks := make([]string, 0, len(affected))
	for task := range affected {
		tasks = append(tasks, task)
	}
	sort.Strings(tasks)
	result := ValidateProjectedBudgetsResult{Valid: true, AffectedTaskIds: tasks, ProjectedBytes: []int{}, OverBudgetTaskIds: []string{}}
	for _, task := range tasks {
		b, err := ReadTaskBudget(ctx, q, site, task)
		if err != nil {
			return result, err
		}
		if budget != nil && budget.TaskId == task {
			b = *budget
		}
		langs := map[string]bool{"und": true}
		for _, m := range mappings {
			if m.TaskId == task {
				langs[m.Language] = true
			}
		}
		for _, e := range entries {
			langs[e.Language] = true
		}
		maxBytes := 0
		over := false
		for language := range langs {
			selection := SelectContextEntries(entries, mappings, task, language)
			n := len([]byte(RenderText(selection.Entries)))
			if n > maxBytes {
				maxBytes = n
			}
			if n > b.MaximumBytes || len(selection.Entries) > b.MaximumEntries {
				over = true
			}
		}
		result.ProjectedBytes = append(result.ProjectedBytes, maxBytes)
		if over {
			result.Valid = false
			result.OverBudgetTaskIds = append(result.OverBudgetTaskIds, task)
		}
	}
	if !result.Valid {
		return result, platform.NewError(platform.CodeValidation, false, "记忆任务注入超出预算：%s", joinTaskIDs(result.OverBudgetTaskIds))
	}
	return result, nil
}
func joinTaskIDs(ids []string) string {
	v := ""
	for i, id := range ids {
		if i > 0 {
			v += ","
		}
		v += id
	}
	return v
}
func EmitContextChanged(ctx context.Context, tx *sql.Tx, op OperationContext, tasks []string) error {
	var generation, revision int
	if err := tx.QueryRowContext(ctx, `SELECT memory_generation,mapping_revision FROM runtime_state WHERE id=1`).Scan(&generation, &revision); err != nil {
		return memoryruntime.DBError(err)
	}
	return memoryruntime.Emit(tx, op, "memory.context.changed", "memory-context", generation, map[string]any{"memoryGeneration": generation, "mappingRevision": revision, "taskIds": tasks, "dataEpoch": op.DataEpoch})
}
