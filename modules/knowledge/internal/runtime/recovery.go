package runtime

import (
 "context"
 "database/sql"
 "errors"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/knowledge/internal/build"
 "nice-wiki/modules/knowledge/internal/extract"
 indexer "nice-wiki/modules/knowledge/internal/index"
)

type RecoverBuildLeasesInput struct { Now string `json:"now"`;Limit int `json:"limit"` }
type RecoverBuildLeasesResult struct { Requeued int `json:"requeued"`;Superseded int `json:"superseded"` }

// RecoverBuildLeases 只续派原意图或隔离失效来源；原jobs孤立任务由owner恢复证明决策，不造新dedupe绕过。
func RecoverBuildLeases(ctx context.Context,op OperationContext,input RecoverBuildLeasesInput)(RecoverBuildLeasesResult,error) {
 if op.CallerService!="knowledge"||op.Actor.Kind!="service" {return RecoverBuildLeasesResult{},platform.NewError(platform.CodeForbidden,false,"只允许本域后台恢复构建")}
 now,err:=platform.ParseTimestamp(input.Now);if err!=nil {return RecoverBuildLeasesResult{},err}
 if now.After(time.Now()) {return RecoverBuildLeasesResult{},platform.NewError(platform.CodeValidation,false,"恢复时点不能来自未来")}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return RecoverBuildLeasesResult{},err}
 if runtime.Content==nil||runtime.ValidatePublicationResult==nil {return RecoverBuildLeasesResult{},platform.NewError(platform.CodeDependencyUnavailable,false,"发布权威未装配")}
 rows,err:=runtime.DB.QueryContext(ctx,`SELECT b.build_id,d.dispatch_state,d.cancel_requested,b.lease_until FROM builds b JOIN build_dispatches d ON d.build_id=b.build_id WHERE b.site_id=? AND b.data_epoch=? AND ((d.cancel_requested=1 AND d.dispatch_state NOT IN ('cancelled','completed')) OR (b.state NOT IN ('active','failed','cancelled','superseded') AND (b.lease_until='' OR julianday(b.lease_until)<=julianday(?) OR julianday(b.lease_until) IS NULL))) ORDER BY b.created_at,b.build_id LIMIT ?`,op.Actor.SiteID,op.DataEpoch,now.UTC().Format(time.RFC3339Nano),input.Limit)
 if err!=nil {return RecoverBuildLeasesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建恢复扫描失败")}
 type candidate struct{id,state string;cancelled int;lease string};candidates:=[]candidate{}
 for rows.Next(){var value candidate;if err=rows.Scan(&value.id,&value.state,&value.cancelled,&value.lease);err!=nil {rows.Close();return RecoverBuildLeasesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建恢复扫描失败")};candidates=append(candidates,value)};err=rows.Err();rows.Close()
 if err!=nil {return RecoverBuildLeasesResult{},platform.NormalizeIOError(err,platform.CodeInternal,false,"构建恢复扫描失败")}
 result:=RecoverBuildLeasesResult{}
 for _,candidate:=range candidates {
  if candidate.cancelled==0&&candidate.lease!="" {expires,err:=platform.ParseTimestamp(candidate.lease);if err!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"持久租约到期时间损坏")};if expires.After(now) {continue}}
  if candidate.cancelled!=0 {if err=indexer.CancelBuildSideEffects(ctx,op,candidate.id);err!=nil {return result,err};continue}
  read,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"恢复来源读取失败")}
  record,err:=indexer.ReadBuildRecord(ctx,read,op,candidate.id);_=read.Rollback();if err!=nil {return result,err}
  _,_,err=indexer.ValidatePublication(ctx,op,runtime.Content,runtime.ValidatePublicationResult,record.Build.Document)
  if err!=nil {
   var stale *platform.Error
   if !errors.As(err,&stale)||stale.Code!=platform.CodeStaleResult {return result,err}
   work,tx,cancel,err:=indexer.BeginIndexWrite(ctx,op);if err!=nil {return result,err}
   updated,err:=tx.ExecContext(work,`UPDATE builds SET state='superseded',generation_state='retired',retired_at_ns=?,cancelled=1,lease_id='',lease_until='' WHERE build_id=? AND site_id=? AND data_epoch=? AND state NOT IN ('active','failed','cancelled','superseded') AND lease_until=?`,time.Now().UTC().UnixNano(),candidate.id,op.Actor.SiteID,op.DataEpoch,candidate.lease)
   if err!=nil {tx.Rollback();cancel();return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"失效构建隔离失败")}
   count,err:=updated.RowsAffected();if err!=nil {tx.Rollback();cancel();return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"恢复CAS检查失败")}
   if count>0 {_,err=tx.ExecContext(work,`UPDATE build_dispatches SET cancel_requested=1,updated_at=? WHERE build_id=?`,platform.Now(),candidate.id)}
   if err==nil&&count>0 {err=indexer.CancelUnusedExtractions(work,tx,op,candidate.id)}
   if err==nil {err=tx.Commit()};_=tx.Rollback();cancel();if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"恢复隔离提交失败")}
   if count>0 {result.Superseded++;if err=indexer.CancelBuildSideEffects(ctx,op,candidate.id);err!=nil {return result,err}}
   continue
  }
  if candidate.state=="pending"||candidate.state=="dispatching" {
   if err=build.DispatchBuild(ctx,op,candidate.id);err!=nil {return result,err}
   // 只有真实已保存job引用才记续派；DispatchBuild并发取消的no-op不计成功。
   var state string;var job sql.NullString
   err=runtime.DB.QueryRowContext(ctx,`SELECT dispatch_state,job_id FROM build_dispatches WHERE build_id=?`,candidate.id).Scan(&state,&job)
   if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"恢复派发回执读取失败")}
   if state=="dispatched"&&job.Valid&&job.String!="" {result.Requeued++}
  }
  // dispatched原任务的到期租约不在此新建job，阶段/付费结果保留给jobs原owner证明恢复。
 }
 return result,nil
}

// RecoverExtractionTasks 有界续派原意图及取消清理，已派发租约由jobs原owner证明恢复。
func RecoverExtractionTasks(ctx context.Context,op OperationContext,limit int)error {
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return err}
 rows,err:=runtime.DB.QueryContext(ctx,`SELECT d.extraction_id,d.cancel_requested FROM extraction_dispatches d JOIN extractions e ON e.extraction_id=d.extraction_id WHERE e.site_id=? AND e.data_epoch=? AND ((d.cancel_requested=1 AND d.dispatch_state NOT IN ('cancelled','completed')) OR (d.cancel_requested=0 AND d.dispatch_state IN ('pending','dispatching'))) ORDER BY d.created_at,d.extraction_id LIMIT ?`,op.Actor.SiteID,op.DataEpoch,limit)
 if err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取恢复扫描失败")}
 type candidate struct{id string;cancelled int};values:=[]candidate{}
 for rows.Next(){var value candidate;if err=rows.Scan(&value.id,&value.cancelled);err!=nil {rows.Close();return platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取恢复扫描失败")};values=append(values,value)};err=rows.Err();rows.Close();if err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取恢复扫描失败")}
 for _,value:=range values {if value.cancelled!=0 {err=indexer.CancelExtractionSideEffects(ctx,op,value.id)} else {err=extract.DispatchExtraction(ctx,op,value.id)};if err!=nil {return err}}
 return nil
}
