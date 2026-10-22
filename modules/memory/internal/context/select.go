package memorycontext

import (
	"context"
	"nice-wiki/modules/memory/internal/gen"
	memoryruntime "nice-wiki/modules/memory/internal/runtime"
	"sort"
)

type SelectContextEntriesInput struct {
	TaskId          string `json:"taskId"`
	Language        string `json:"language"`
	MappingRevision int    `json:"mappingRevision"`
}
type SelectContextEntriesResult struct {
	Entries    []gen.MemoryEntry `json:"entries"`
	MappingIds []string          `json:"mappingIds"`
}

func ReadMappings(ctx context.Context, q memoryruntime.Queryer, site string) ([]gen.TaskMapping, error) {
	rows, err := q.QueryContext(ctx, `SELECT mapping_id,task_id,entry_id,order_value,enabled,language,revision FROM task_mappings WHERE site_id=? AND deleted=0 ORDER BY order_value,mapping_id`, site)
	if err != nil {
		return nil, memoryruntime.DBError(err)
	}
	defer rows.Close()
	list := []gen.TaskMapping{}
	for rows.Next() {
		var m gen.TaskMapping
		if err = rows.Scan(&m.MappingId, &m.TaskId, &m.EntryId, &m.Order, &m.Enabled, &m.Language, &m.Revision); err != nil {
			return nil, memoryruntime.DBError(err)
		}
		list = append(list, m)
	}
	return list, memoryruntime.DBError(rows.Err())
}

// SelectContextEntries 选出注入的记忆：所有启用的条目都注入给每个 AI 任务，不分任务、不分语言（用户自己写的纯文本，数量有限）。
// 优先级高的在前，同优先级按条目 ID 稳定排序。mappings 等参数保留只为兼容旧调用。
func SelectContextEntries(entries []gen.MemoryEntry, mappings []gen.TaskMapping, task, language string) SelectContextEntriesResult {
	list := make([]gen.MemoryEntry, 0, len(entries))
	for _, e := range entries {
		if e.State == "active" {
			list = append(list, e)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Priority != list[j].Priority {
			return list[i].Priority > list[j].Priority
		}
		return list[i].EntryId < list[j].EntryId
	})
	return SelectContextEntriesResult{Entries: list, MappingIds: []string{}}
}
