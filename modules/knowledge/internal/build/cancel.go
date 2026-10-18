package build

import (
 "context"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/admin"
 "nice-wiki/modules/knowledge/internal/gen"
 indexer "nice-wiki/modules/knowledge/internal/index"
)

type CancelBuildInput = gen.CancelBuildInput
type CancelBuildResult = gen.CancelBuildResult

// CancelBuild 先持久关闭staging资格，再锁外请求jobs及MG取消；重放继续未完副作用。
func CancelBuild(ctx context.Context,op OperationContext,input CancelBuildInput)(CancelBuildResult,error) {
 if err:=admin.AuthorizeAdmin(op);err!=nil { return CancelBuildResult{},err }
 work,tx,cancel,err:=indexer.BeginIndexWrite(ctx,op)
 if err!=nil { return CancelBuildResult{},err };defer cancel();defer tx.Rollback()
 var original CancelBuildResult
 found,err:=indexer.ReadOperationReceipt(work,tx,op,"CancelBuild",input.IdempotencyKey,input,&original)
 if err!=nil { return CancelBuildResult{},err }
 if found { _=tx.Rollback();cancel();if err=indexer.CancelBuildSideEffects(ctx,op,input.BuildId);err!=nil { return CancelBuildResult{},err };return original,nil }
 record,err:=indexer.ReadBuildRecord(work,tx,op,input.BuildId)
 if err!=nil { return CancelBuildResult{},err }
 switch string(record.Build.State) {
 case "active","failed","superseded","cancelled":
 default:
  _,err=tx.ExecContext(work,`UPDATE builds SET cancelled=1,state='cancelled',generation_state='retired',retired_at_ns=?,lease_id='',lease_until='' WHERE build_id=? AND site_id=? AND data_epoch=?`,time.Now().UTC().UnixNano(),input.BuildId,op.Actor.SiteID,op.DataEpoch)
  if err!=nil { return CancelBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建取消标记保存失败") }
  record.Build.State=gen.IndexBuildState("cancelled")
 }
 if string(record.Build.State)!="active" {
  if _,err=tx.ExecContext(work,`UPDATE build_dispatches SET cancel_requested=1,updated_at=? WHERE build_id=?`,platform.Now(),input.BuildId);err!=nil { return CancelBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建取消意图保存失败") }
 }
 if string(record.Build.State)!="active" {if err=indexer.CancelUnusedExtractions(work,tx,op,input.BuildId);err!=nil {return CancelBuildResult{},err}}
 result:=CancelBuildResult{Build:record.Build}
 if err=indexer.SaveOperationReceipt(work,tx,op,"CancelBuild",input.IdempotencyKey,input,result);err!=nil { return CancelBuildResult{},err }
 if err=tx.Commit();err!=nil { return CancelBuildResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建取消决定提交失败") };cancel()
 if err=indexer.CancelBuildSideEffects(ctx,op,input.BuildId);err!=nil { return CancelBuildResult{},err };return result,nil
}
