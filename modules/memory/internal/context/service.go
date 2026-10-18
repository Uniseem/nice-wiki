package memorycontext

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/memory/internal/gen"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
)

type ResolveContextInput=gen.ResolveContextInput
type MemoryContext=gen.MemoryContext
func allowedCaller(caller,task string)bool{
 switch caller{
 case "qa":return task=="qa.preflight"||task=="qa.answer"||task=="qa.cache-match"||task=="qa.cache-translate"||task=="qa.compact"
 case "knowledge":return task=="qa.inspect"||task=="knowledge.context"||task=="knowledge.describe"||task=="knowledge.describe-visual"||task=="knowledge.extract-attachment"||task=="knowledge.embed"||task=="knowledge.rerank"
 case "localization":return task=="localization.document"||task=="localization.ui"||task=="localization.terms"||task=="localization.review"||task=="localization.suggest"
 case "memory":return task=="memory.assistant"||task=="memory.overview"
 case "content":return task=="content.slug"||task=="content.seo"
 };return false
}
func ResolveContext(ctx context.Context,op OperationContext,input ResolveContextInput)(MemoryContext,error){
 if op.Actor.Kind!="service"||!allowedCaller(op.CallerService,input.TaskId){return MemoryContext{},platform.NewError(platform.CodeForbidden,false,"调用服务无权解析此任务记忆")}
 var result MemoryContext;var rendered RenderContextResult
 err:=memoryruntime.ReadSnapshot(ctx,func(s *memoryruntime.Store,tx *sql.Tx)error{
  var revision int;var epoch string;if err:=tx.QueryRowContext(ctx,`SELECT mapping_revision,data_epoch FROM runtime_state WHERE id=1`).Scan(&revision,&epoch);err!=nil{return memoryruntime.DBError(err)};if epoch!=op.DataEpoch{return platform.NewError(platform.CodeVersionMismatch,false,"记忆快照数据代次已变化")}
  entries,err:=s.ReadAllEntries(ctx,tx,op.Actor.SiteID);if err!=nil{return err};mappings,err:=ReadMappings(ctx,tx,op.Actor.SiteID);if err!=nil{return err};budget,err:=ReadTaskBudget(ctx,tx,op.Actor.SiteID,input.TaskId);if err!=nil{return err}
  selected:=SelectContextEntries(entries,mappings,input.TaskId,input.Language);rendered,err=RenderContext(ctx,op,RenderContextInput{Entries:selected.Entries,TaskId:input.TaskId,Language:input.Language,MappingRevision:revision,Budget:budget});if err!=nil{return err}
  result=MemoryContext{TaskId:input.TaskId,Language:input.Language,Text:rendered.Text,VersionHash:rendered.Manifest.VersionHash,MappingRevision:revision,ByteSize:rendered.Manifest.ByteSize};return nil
 });if err!=nil{return MemoryContext{},err}
 // 派生清单与业务快照分开保存；清单只引用已经核验的不可变修订。
 err=memoryruntime.SaveContextManifest(ctx,op,rendered.Manifest.SnapshotId,input.TaskId,input.Language,rendered.Manifest.MappingRevision,rendered.Manifest.VersionHash,rendered.Manifest.BodyHash,rendered.Manifest.ByteSize,rendered.Manifest)
 if err!=nil{return MemoryContext{},err};return result,nil
}
