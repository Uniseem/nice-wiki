package memorycontext

import (
 "context"
 "encoding/json"
 "strings"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/gen"
)

const RendererVersion="memory-context-v1"
type ContextManifest struct {
 SnapshotId string `json:"snapshotId"`;TaskId string `json:"taskId"`;Language string `json:"language"`
 EntryIds []string `json:"entryIds"`;EntryVersions []int `json:"entryVersions"`
 MappingRevision int `json:"mappingRevision"`;RendererVersion string `json:"rendererVersion"`
 BodyHash string `json:"bodyHash"`;VersionHash string `json:"versionHash"`;ByteSize int `json:"byteSize"`
 BudgetRevision int `json:"budgetRevision"`;MaximumBytes int `json:"maximumBytes"`;MaximumEntries int `json:"maximumEntries"`
 EntryLanguages []string `json:"entryLanguages"`;EntryBodyHashes []string `json:"entryBodyHashes"`
}
type ApplyContextBudgetInput struct {Entries []gen.MemoryEntry `json:"entries"`;Budget gen.TaskBudget `json:"budget"`}
type ApplyContextBudgetResult struct {Entries []gen.MemoryEntry `json:"entries"`;OmittedIds []string `json:"omittedIds"`;ByteSize int `json:"byteSize"`}
type RenderContextInput struct {Entries []gen.MemoryEntry;TaskId string;Language string;MappingRevision int;Budget gen.TaskBudget}
type RenderContextResult struct {Text string `json:"text"`;Manifest ContextManifest `json:"manifest"`}

// RenderText 每条以 JSON 标识边界，正文保持原义；按字符串字节计算预算。
func RenderText(entries []gen.MemoryEntry)string{
 var text strings.Builder
 for _,e:=range entries{
  meta,_:=platform.CanonicalJSON(map[string]any{"entryId":e.EntryId,"version":e.Version,"title":e.Title,"description":e.Description})
  text.WriteString("<memory-entry ");text.Write(meta);text.WriteString(">\n");text.WriteString(e.Body);text.WriteString("\n</memory-entry>\n")
 };return text.String()
}
func ApplyContextBudget(ctx context.Context,op OperationContext,input ApplyContextBudgetInput)(ApplyContextBudgetResult,error){
 for _,e:=range input.Entries{if len([]byte(e.Body))>e.MaximumBytes{return ApplyContextBudgetResult{},platform.NewError(platform.CodeIntegrityError,false,"记忆条目超出已保存预算")}}
 n:=len([]byte(RenderText(input.Entries)))
 if n>input.Budget.MaximumBytes||len(input.Entries)>input.Budget.MaximumEntries{return ApplyContextBudgetResult{},platform.NewError(platform.CodeIntegrityError,false,"完整记忆注入超过任务预算")}
 return ApplyContextBudgetResult{Entries:input.Entries,OmittedIds:[]string{},ByteSize:n},nil
}
func RenderContext(ctx context.Context,op OperationContext,input RenderContextInput)(RenderContextResult,error){
 bounded,err:=ApplyContextBudget(ctx,op,ApplyContextBudgetInput{Entries:input.Entries,Budget:input.Budget});if err!=nil{return RenderContextResult{},err}
 text:=RenderText(bounded.Entries)
 m:=ContextManifest{TaskId:input.TaskId,Language:input.Language,EntryIds:[]string{},EntryVersions:[]int{},MappingRevision:input.MappingRevision,RendererVersion:RendererVersion,BodyHash:platform.HashBytes([]byte(text)),ByteSize:len([]byte(text)),BudgetRevision:input.Budget.Revision,MaximumBytes:input.Budget.MaximumBytes,MaximumEntries:input.Budget.MaximumEntries,EntryLanguages:[]string{},EntryBodyHashes:[]string{}}
 for _,e:=range input.Entries{m.EntryIds=append(m.EntryIds,e.EntryId);m.EntryVersions=append(m.EntryVersions,e.Version);m.EntryLanguages=append(m.EntryLanguages,e.Language);m.EntryBodyHashes=append(m.EntryBodyHashes,e.BodyHash)}
 // hash 不包含自引用的 versionHash/snapshotId；相同输入生成相同版本。
 raw,_:=json.Marshal(m);var obj map[string]any;_ = json.Unmarshal(raw,&obj);delete(obj,"snapshotId");delete(obj,"versionHash")
 canonical,err:=platform.CanonicalJSON(obj);if err!=nil{return RenderContextResult{},platform.NewError(platform.CodeInternal,false,"记忆清单不可编码")}
 m.VersionHash=platform.HashBytes(canonical);m.SnapshotId="ctx-"+m.VersionHash
 return RenderContextResult{Text:text,Manifest:m},nil
}
