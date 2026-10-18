package runtime

import (
 "context"
 "time"
 "nice-wiki/libs/platform"
 indexer "nice-wiki/modules/knowledge/internal/index"
)

type CollectRetiredIndexesInput struct { OlderThan string `json:"olderThan"`;Limit int `json:"limit"` }
type CollectRetiredIndexesResult struct { Deleted int `json:"deleted"`;Retained int `json:"retained"` }

// CollectRetiredIndexes 只回收24小时以前的已知退役代数；SQL快照拥有读者生命周期。
func CollectRetiredIndexes(ctx context.Context,op OperationContext,input CollectRetiredIndexesInput)(CollectRetiredIndexesResult,error) {
 if op.CallerService!="knowledge"||op.Actor.Kind!="service" { return CollectRetiredIndexesResult{},platform.NewError(platform.CodeForbidden,false,"只允许本域后台回收索引") }
 cutoff,err:=platform.ParseTimestamp(input.OlderThan)
 if err!=nil { return CollectRetiredIndexesResult{},err }
 if cutoff.After(time.Now().UTC().Add(-24*time.Hour)) { return CollectRetiredIndexesResult{},platform.NewError(platform.CodeValidation,false,"退役索引至少保留24小时") }
 work,tx,cancel,err:=indexer.BeginIndexWrite(ctx,op)
 if err!=nil { return CollectRetiredIndexesResult{},err };defer cancel();defer tx.Rollback()
 rows,err:=tx.QueryContext(work,`SELECT build_id FROM builds WHERE site_id=? AND data_epoch=? AND generation_state='retired' AND retired_at_ns>0 AND retired_at_ns<=? ORDER BY retired_at_ns,build_id LIMIT ?`,op.Actor.SiteID,op.DataEpoch,cutoff.UnixNano(),input.Limit)
 if err!=nil { return CollectRetiredIndexesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"退役索引扫描失败") }
 ids:=[]string{}
 for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil { rows.Close();return CollectRetiredIndexesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"退役索引扫描失败") };ids=append(ids,id)}
 err=rows.Err();rows.Close()
 if err!=nil { return CollectRetiredIndexesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"退役索引扫描失败") }
 result:=CollectRetiredIndexesResult{}
 for _,id:=range ids {
  var refs int
  err=tx.QueryRowContext(work,`SELECT (SELECT COUNT(*) FROM active_indexes WHERE build_id=?)+(SELECT COUNT(*) FROM build_dispatches WHERE build_id=? AND dispatch_state NOT IN ('completed','cancelled'))+(SELECT COUNT(*) FROM model_steps WHERE owner_id=? AND state NOT IN ('completed','failed','cancelled'))`,id,id,id).Scan(&refs)
  if err!=nil { return CollectRetiredIndexesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"退役索引引用检查失败") }
  if refs!=0 { result.Retained++;continue }
  // 全部索引均在本域SQL，不存在额外文件句柄；SQLite保留并发只读快照所需旧页。
  for _,table:=range []string{"vectors","terms","chunks","build_sources","build_dispatches","build_execution_plans"} {
   if _,err=tx.ExecContext(work,"DELETE FROM "+table+" WHERE build_id=?",id);err!=nil { return CollectRetiredIndexesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"退役索引删除失败") }
  }
  // 已付费完成结果独立保留，供哈希复用；回收索引不删除模型结果或费用未知引用。
  if _,err=tx.ExecContext(work,`DELETE FROM builds WHERE build_id=? AND site_id=? AND data_epoch=? AND generation_state='retired'`,id,op.Actor.SiteID,op.DataEpoch);err!=nil { return CollectRetiredIndexesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"退役代数删除失败") };result.Deleted++
 }
 if err=tx.Commit();err!=nil { return CollectRetiredIndexesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"退役回收提交失败") };return result,nil
}
