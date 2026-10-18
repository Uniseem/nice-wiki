package mapping

import (
 "context"
 "database/sql"
 "errors"
 "golang.org/x/text/language"
 "nice-wiki/libs/platform"
 memorycontext "nice-wiki/modules/memory/internal/context"
 memoryruntime "nice-wiki/modules/memory/internal/runtime"
 "nice-wiki/modules/memory/internal/gen"
)

type PutMappingInput=gen.PutMappingInput
type PutMappingResult=gen.PutMappingResult
type RemoveMappingInput struct{MappingId string `json:"mappingId"`;ExpectedRevision int `json:"expectedRevision"`;IdempotencyKey string `json:"idempotencyKey"`}
type RemoveMappingResult=gen.RemoveMappingResult
func PutMapping(ctx context.Context,op OperationContext,input PutMappingInput)(PutMappingResult,error){
 if err:=memoryruntime.RequireManager(op);err!=nil{return PutMappingResult{},err};if err:=memoryruntime.ValidateTask(input.Mapping.TaskId);err!=nil{return PutMappingResult{},err};if _,err:=language.Parse(input.Mapping.Language);err!=nil{return PutMappingResult{},platform.NewError(platform.CodeValidation,false,"映射语言标签不合法")}
 var result PutMappingResult
 err:=memoryruntime.Write(ctx,op,"memory.PutMapping",input.IdempotencyKey,input,&result,func(s *memoryruntime.Store,tx *sql.Tx)error{
  m:=input.Mapping;var revision int
  err:=tx.QueryRowContext(ctx,`SELECT revision FROM task_mappings WHERE site_id=? AND mapping_id=?`,op.Actor.SiteID,m.MappingId).Scan(&revision);if err!=nil&&!errors.Is(err,sql.ErrNoRows){return memoryruntime.DBError(err)}
  if revision!=input.ExpectedRevision{return platform.NewError(platform.CodeVersionMismatch,false,"映射版本已变化")};m.Revision=revision+1
  e,err:=s.ReadEntryObject(ctx,tx,op.Actor.SiteID,m.EntryId,nil);if err!=nil{return err};if e.Language!=m.Language&&e.Language!="und"{return platform.NewError(platform.CodeValidation,false,"映射语言与记忆条目不相容")}
  projection,err:=memorycontext.ValidateProjectedBudgets(ctx,s,tx,op.Actor.SiteID,nil,&m,nil);if err!=nil{return err}
  var other string;err=tx.QueryRowContext(ctx,`SELECT mapping_id FROM task_mappings WHERE site_id=? AND task_id=? AND entry_id=? AND language=? AND deleted=0 AND mapping_id<>?`,op.Actor.SiteID,m.TaskId,m.EntryId,m.Language,m.MappingId).Scan(&other);if err==nil{return platform.NewError(platform.CodeConflict,false,"同任务、条目与语言映射已存在")};if !errors.Is(err,sql.ErrNoRows){return memoryruntime.DBError(err)}
  _,err=tx.ExecContext(ctx,`INSERT INTO task_mappings(site_id,mapping_id,task_id,entry_id,order_value,enabled,language,revision,deleted) VALUES(?,?,?,?,?,?,?,?,0) ON CONFLICT(site_id,mapping_id) DO UPDATE SET task_id=excluded.task_id,entry_id=excluded.entry_id,order_value=excluded.order_value,enabled=excluded.enabled,language=excluded.language,revision=excluded.revision,deleted=0`,op.Actor.SiteID,m.MappingId,m.TaskId,m.EntryId,m.Order,m.Enabled,m.Language,m.Revision);if err!=nil{return memoryruntime.DBError(err)}
  result.Mapping=m;result.MappingRevision,err=advance(ctx,tx);if err!=nil{return err};return memorycontext.EmitContextChanged(ctx,tx,op,projection.AffectedTaskIds)
 });return result,err
}
func RemoveMapping(ctx context.Context,op OperationContext,input RemoveMappingInput)(RemoveMappingResult,error){
 if err:=memoryruntime.RequireManager(op);err!=nil{return RemoveMappingResult{},err};var result RemoveMappingResult
 err:=memoryruntime.Write(ctx,op,"memory.RemoveMapping",input.IdempotencyKey,input,&result,func(_ *memoryruntime.Store,tx *sql.Tx)error{
  var revision int;var task string;err:=tx.QueryRowContext(ctx,`SELECT revision,task_id FROM task_mappings WHERE site_id=? AND mapping_id=? AND deleted=0`,op.Actor.SiteID,input.MappingId).Scan(&revision,&task);if errors.Is(err,sql.ErrNoRows){return platform.NewError(platform.CodeNotFound,false,"任务映射不存在")};if err!=nil{return memoryruntime.DBError(err)};if revision!=input.ExpectedRevision{return platform.NewError(platform.CodeVersionMismatch,false,"映射版本已变化")}
  _,err=tx.ExecContext(ctx,`UPDATE task_mappings SET deleted=1,enabled=0,revision=revision+1 WHERE site_id=? AND mapping_id=? AND revision=? AND deleted=0`,op.Actor.SiteID,input.MappingId,revision);if err!=nil{return memoryruntime.DBError(err)};result.Removed=true;result.MappingRevision,err=advance(ctx,tx);if err!=nil{return err};return memorycontext.EmitContextChanged(ctx,tx,op,[]string{task})
 });return result,err
}
func advance(ctx context.Context,tx *sql.Tx)(int,error){
 _,err:=tx.ExecContext(ctx,`UPDATE runtime_state SET memory_generation=memory_generation+1,mapping_revision=mapping_revision+1,updated_at=? WHERE id=1`,platform.Now());if err!=nil{return 0,memoryruntime.DBError(err)};var revision int;err=tx.QueryRowContext(ctx,`SELECT mapping_revision FROM runtime_state WHERE id=1`).Scan(&revision);return revision,memoryruntime.DBError(err)
}
