package runtime

import (
 "context"
 "database/sql"
 "errors"
 "encoding/json"
 "time"
 "nice-wiki/libs/platform"
 indexer "nice-wiki/modules/knowledge/internal/index"
)

type ReadJobRecoveryInput = platform.OwnerRecoveryRequest
type ReadJobRecoveryResult = platform.OwnerRecoveryResult

// ReadJobRecovery 只证明原任务事实；未能证明安全的中间状态保持pending，不发新付费许可。
func ReadJobRecovery(ctx context.Context,op OperationContext,input ReadJobRecoveryInput)(ReadJobRecoveryResult,error) {
 result:=ReadJobRecoveryResult{Status:"unknown",PaidResultRefs:[]string{}}
 if op.CallerService!="jobs"||op.Actor.Kind!="service"||input.Owner!="knowledge" {return result,platform.NewError(platform.CodeForbidden,false,"恢复证明只允许jobs读取本域原任务")}
 if input.DataEpoch!=op.DataEpoch {return result,platform.NewError(platform.CodeStaleResult,false,"恢复请求不是当前数据代次")}
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return result,err}
 tx,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原任务恢复快照不可读")};defer tx.Rollback()
 // GC不删除最终拥有者证明；即使退役索引和原派发行已回收，仍能恢复真实已完成结果。
 completed,lookup:=indexer.LoadExecutionRecord(ctx,tx,input.JobID)
 if lookup==nil&&completed.State=="completed"&&completed.ResultRef!=nil {
  if completed.JobType!=input.Type||completed.ObjectID!=input.InputRef.ObjectID||completed.InputRef!=input.InputRef||completed.Operation.RootOperationID!=input.RootOperationID||completed.Operation.Actor.SiteID!=op.Actor.SiteID||completed.Operation.DataEpoch!=input.DataEpoch||completed.Fence!=input.ExpiredFence||completed.WorkerID!=input.WorkerID||int64(completed.Attempt)!=input.Attempt||completed.CancelRequested!=input.CancelRequested {return result,platform.NewError(platform.CodeIntegrityError,false,"原完成证明与恢复来源或失效租约不一致")}
  result.Status="completed";result.ResultRef=completed.ResultRef;result.PaidResultRefs=completed.PaidRefs;return result,nil
 }
 if lookup!=nil {var failure *platform.Error;if !errors.As(lookup,&failure)||failure.Code!=platform.CodeNotFound {return result,lookup}}
 original,state,jobsOperation,site,cancelled,err:=indexer.ApprovedJobInput(ctx,tx,input.Type,input.InputRef.ObjectID)
 if err!=nil {var failure *platform.Error;if errors.As(err,&failure)&&failure.Code==platform.CodeNotFound {return result,nil};return result,err}
 if site!=op.Actor.SiteID||original.Operation.DataEpoch!=input.DataEpoch||original.InputRef!=input.InputRef||original.Operation.RootOperationID!=input.RootOperationID||original.JobID!=input.JobID||jobsOperation!=original.Operation.OperationID||cancelled!=input.CancelRequested {return result,platform.NewError(platform.CodeIntegrityError,false,"恢复请求与本站原批准输入或取消事实不一致")}
 result.Status="pending"
 record,err:=indexer.LoadExecutionRecord(ctx,tx,input.JobID)
 if err==nil {
  if record.JobType!=input.Type||record.ObjectID!=input.InputRef.ObjectID||record.InputRef!=input.InputRef||record.Operation.RootOperationID!=input.RootOperationID||record.Operation.OperationID!=jobsOperation||record.Operation.Actor.SiteID!=site||record.Operation.DataEpoch!=input.DataEpoch||record.Fence!=input.ExpiredFence||record.WorkerID!=input.WorkerID||int64(record.Attempt)!=input.Attempt||record.CancelRequested!=input.CancelRequested {return result,platform.NewError(platform.CodeIntegrityError,false,"恢复请求与原worker/fence/attempt不一致")}
  result.PaidResultRefs,err=indexer.ReadPaidResultRefs(ctx,tx,original.Operation,record.ObjectID);if err!=nil {return result,err}
  if record.State=="completed"&&record.ResultRef!=nil {result.Status="completed";result.ResultRef=record.ResultRef;return result,nil}
  if record.LeaseUntil.After(time.Now()) {return result,nil}
  var unsettled int
  err=tx.QueryRowContext(ctx,`SELECT (SELECT count(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND (owner_id=? OR owner_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?)) AND dispatch_started=1 AND (state NOT IN ('completed','failed','cancelled') OR result_json IS NULL))+(SELECT count(*) FROM extraction_dispatches d WHERE d.extraction_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?) AND d.dispatch_state NOT IN ('completed','cancelled'))`,site,input.DataEpoch,record.ObjectID,record.ObjectID,record.ObjectID).Scan(&unsettled)
  if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原费用及子任务恢复事实不可读")}
  if unsettled==0 {
   // 最终paid结果hash逐项回验；不以终态字符串代替原结果完整性。
   rows,err:=tx.QueryContext(ctx,`SELECT result_json,result_hash FROM model_steps WHERE site_id=? AND data_epoch=? AND (owner_id=? OR owner_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?)) AND dispatch_started=1`,site,input.DataEpoch,record.ObjectID,record.ObjectID)
   if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原模型结果证明不可读")}
   for rows.Next(){var body []byte;var hash string;if err=rows.Scan(&body,&hash);err!=nil {rows.Close();return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原模型结果证明不可读")};if hash!=platform.HashBytes(body)||!json.Valid(body) {rows.Close();return result,platform.NewError(platform.CodeIntegrityError,false,"原模型终态结果hash损坏")}}
   err=rows.Err();rows.Close();if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原模型结果证明不可读")}
   var workflowVersion int
   query:=`SELECT workflow_version FROM builds WHERE build_id=? AND site_id=? AND data_epoch=?`;if input.Type=="knowledge.extract" {query=`SELECT workflow_version FROM extractions WHERE extraction_id=? AND site_id=? AND data_epoch=?`}
   if err=tx.QueryRowContext(ctx,query,record.ObjectID,site,input.DataEpoch).Scan(&workflowVersion);err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原任务恢复版本不可读")}
   deadline,err:=platform.ParseTimestamp(record.Operation.Deadline);if err!=nil {return result,platform.NewError(platform.CodeIntegrityError,false,"原任务批准截止损坏")}
   if workflowVersion==1&&(record.CancelRequested||deadline.After(time.Now())) {result.Status="safeToRetry";return result,nil} // 原job/key/检查点继续，取消则jobs直接终止。
  }
  // 已执行阶段可能含原MG未知费用或已写中间结果；只保留事实，不猜测其可重派。
  return result,nil
 }
 var failure *platform.Error
 if !errors.As(err,&failure)||failure.Code!=platform.CodeNotFound {return result,err}
 if input.ExpiredFence!=0||input.WorkerID!=""||input.Attempt!=0 {return result,nil}
 var workflowVersion int;var objectState string;var qualified int
 switch input.Type {
 case "knowledge.build":
  err=tx.QueryRowContext(ctx,`SELECT workflow_version,state,CASE WHEN lease_id='' AND lease_until='' THEN 1 ELSE 0 END FROM builds WHERE build_id=? AND site_id=? AND data_epoch=?`,input.InputRef.ObjectID,site,input.DataEpoch).Scan(&workflowVersion,&objectState,&qualified)
 case "knowledge.extract":
  err=tx.QueryRowContext(ctx,`SELECT workflow_version,state,CASE WHEN lease_id='' AND lease_until='' AND result_json IS NULL THEN 1 ELSE 0 END FROM extractions WHERE extraction_id=? AND site_id=? AND data_epoch=?`,input.InputRef.ObjectID,site,input.DataEpoch).Scan(&workflowVersion,&objectState,&qualified)
 }
 if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原任务执行标记不可读")}
 if workflowVersion!=1||qualified!=1||(objectState!="queued"&&objectState!="cancelled"&&objectState!="failed")||(state!="dispatched"&&state!="cancelled") {return result,nil}
 var facts int
 err=tx.QueryRowContext(ctx,`SELECT (SELECT count(*) FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=?)+(SELECT count(*) FROM extraction_origins WHERE build_id=?)+(SELECT count(*) FROM chunks WHERE build_id=?)`,site,input.DataEpoch,input.InputRef.ObjectID,input.InputRef.ObjectID,input.InputRef.ObjectID).Scan(&facts)
 if err!=nil {return result,platform.NormalizeIOError(err,platform.CodeInternal,false,"原任务执行证据不可读")}
 if facts!=0 {return result,nil}
 // 新版入口在任何业务执行前先原子存execution_records；缺行且无阶段事实才证明从未执行。
 result.Status="safeToRetry"
 return result,nil
}
