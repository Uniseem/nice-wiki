package memorycontext

import (
 "context"
 "sort"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type SelectContextEntriesInput struct {TaskId string `json:"taskId"`;Language string `json:"language"`;MappingRevision int `json:"mappingRevision"`}
type SelectContextEntriesResult struct {Entries []gen.MemoryEntry `json:"entries"`;MappingIds []string `json:"mappingIds"`}

func ReadMappings(ctx context.Context,q memoryruntime.Queryer,site string)([]gen.TaskMapping,error){
 rows,err:=q.QueryContext(ctx,`SELECT mapping_id,task_id,entry_id,order_value,enabled,language,revision FROM task_mappings WHERE site_id=? AND deleted=0 ORDER BY order_value,mapping_id`,site);if err!=nil{return nil,memoryruntime.DBError(err)};defer rows.Close()
 list:=[]gen.TaskMapping{};for rows.Next(){var m gen.TaskMapping;if err=rows.Scan(&m.MappingId,&m.TaskId,&m.EntryId,&m.Order,&m.Enabled,&m.Language,&m.Revision);err!=nil{return nil,memoryruntime.DBError(err)};list=append(list,m)};return list,memoryruntime.DBError(rows.Err())
}

// Select 固定目标语言后采用 und 回退，优先级和映射顺序均稳定。
func SelectContextEntries(entries []gen.MemoryEntry,mappings []gen.TaskMapping,task,language string)SelectContextEntriesResult{
 byID:=map[string]gen.MemoryEntry{};for _,e:=range entries{if e.State=="active"&&(e.Language==language||e.Language=="und"){byID[e.EntryId]=e}}
 selected:=map[string]gen.TaskMapping{}
 for _,m:=range mappings{
  e,exists:=byID[m.EntryId];if !exists||!m.Enabled||m.TaskId!=task||(m.Language!=language&&m.Language!="und")||!(e.Language==m.Language||e.Language=="und"){continue}
  old,seen:=selected[m.EntryId];if !seen||(m.Language==language&&old.Language!=language)||(m.Language==old.Language&&(m.Order<old.Order||(m.Order==old.Order&&m.MappingId<old.MappingId))){selected[m.EntryId]=m}
 }
 list:=make([]gen.TaskMapping,0,len(selected));for _,m:=range selected{list=append(list,m)}
 sort.Slice(list,func(i,j int)bool{a,b:=list[i],list[j];ea,eb:=byID[a.EntryId],byID[b.EntryId];if ea.Priority!=eb.Priority{return ea.Priority>eb.Priority};if a.Order!=b.Order{return a.Order<b.Order};return a.EntryId<b.EntryId})
 result:=SelectContextEntriesResult{Entries:[]gen.MemoryEntry{},MappingIds:[]string{}}
 for _,m:=range list{result.Entries=append(result.Entries,byID[m.EntryId]);result.MappingIds=append(result.MappingIds,m.MappingId)};return result
}
