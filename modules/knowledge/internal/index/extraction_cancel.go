package index

import (
 "context"
 "database/sql"
 "encoding/json"
 "nice-wiki/libs/platform"
)

// CancelUnusedExtractions 与原构建取消同事务；仍有另一原批准构建使用的提取继续独立执行。
func CancelUnusedExtractions(ctx context.Context,tx *sql.Tx,op OperationContext,buildID string)error {
 if _,err:=tx.ExecContext(ctx,`UPDATE execution_records SET cancel_requested=1 WHERE object_id=? AND job_type='knowledge.build' AND site_id=? AND data_epoch=?`,buildID,op.Actor.SiteID,op.DataEpoch);err!=nil {return storageError(err)}
 _,err:=tx.ExecContext(ctx,`UPDATE extractions SET cancelled=1,state='failed',failure_code=?,lease_id='',lease_until='' WHERE site_id=? AND data_epoch=? AND state NOT IN ('ready','unsupported') AND extraction_id IN (SELECT extraction_id FROM extraction_origins WHERE build_id=?) AND NOT EXISTS(SELECT 1 FROM extraction_origins o JOIN builds b ON b.build_id=o.build_id WHERE o.extraction_id=extractions.extraction_id AND b.cancelled=0 AND b.state NOT IN ('failed','superseded','cancelled'))`,platform.CodeCancelled,op.Actor.SiteID,op.DataEpoch,buildID)
 if err!=nil {return storageError(err)}
 if _,err=tx.ExecContext(ctx,`UPDATE extraction_dispatches SET cancel_requested=1,updated_at=? WHERE extraction_id IN (SELECT extraction_id FROM extractions WHERE site_id=? AND data_epoch=? AND cancelled=1)`,platform.Now(),op.Actor.SiteID,op.DataEpoch);err!=nil {return storageError(err)}
 if _,err=tx.ExecContext(ctx,`UPDATE execution_records SET cancel_requested=1 WHERE job_type='knowledge.extract' AND site_id=? AND data_epoch=? AND object_id IN (SELECT extraction_id FROM extractions WHERE site_id=? AND data_epoch=? AND cancelled=1)`,op.Actor.SiteID,op.DataEpoch,op.Actor.SiteID,op.DataEpoch);err!=nil {return storageError(err)}
 return nil
}

// CancelExtractionSideEffects 只取消原已持久意图及真实调用，不删除unknown或共享的有效任务。
func CancelExtractionSideEffects(ctx context.Context,op OperationContext,id string)error {
 runtime,err:=RuntimeFromContext(ctx);if err!=nil {return err}
 read,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return storageError(err)}
 original,state,_,site,cancelled,err:=ApprovedJobInput(ctx,read,"knowledge.extract",id)
 if err!=nil {read.Rollback();return err};if site!=op.Actor.SiteID||original.Operation.DataEpoch!=op.DataEpoch {read.Rollback();return platform.NewError(platform.CodeForbidden,false,"提取取消不属于本站同代")}
 if !cancelled||state=="cancelled"||state=="completed" {read.Rollback();return nil}
 ids:=[]string{};rows,err:=read.QueryContext(ctx,`SELECT invocation_id FROM model_steps WHERE site_id=? AND data_epoch=? AND owner_id=? AND invocation_id IS NOT NULL AND state NOT IN ('completed','failed','cancelled') ORDER BY invocation_id`,site,op.DataEpoch,id)
 if err!=nil {read.Rollback();return storageError(err)}
 for rows.Next(){var invocation string;if err=rows.Scan(&invocation);err!=nil {rows.Close();read.Rollback();return storageError(err)};ids=append(ids,invocation)};err=rows.Err();rows.Close();read.Rollback();if err!=nil {return storageError(err)}
 if state=="dispatching"&&original.JobID=="" {return platform.NewError(platform.CodeDependencyUnavailable,false,"提取原接受结果未知，保留取消意图")}
 service:=op;service.RootOperationID=original.Operation.RootOperationID;service.CallerService="knowledge";service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:site,ExpiresAt:op.Deadline,Grants:[]string{}}
 work:=platform.WithOperationContext(platform.WithPortalProof(ctx,""),service)
 if original.JobID!="" {
  if runtime.Jobs==nil||runtime.Validators["JobsCancelJobResult"]==nil {return platform.NewError(platform.CodeDependencyUnavailable,false,"提取取消生产方未装配")}
  raw,err:=runtime.Jobs.CancelJob(work,original.JobID,struct{}{},"knowledge.cancel.extract."+id);if err!=nil {return err};if err=runtime.Validators["JobsCancelJobResult"](raw);err!=nil {return err}
  var response struct{State string `json:"state"`};if json.Unmarshal(raw,&response)!=nil {return platform.NewError(platform.CodeIntegrityError,false,"提取取消回执不可读")};switch response.State {case "cancelled","cancel-requested","succeeded","failed","dead-letter","orphan":default:return platform.NewError(platform.CodeIntegrityError,false,"提取取消回执未知")}
 }
 for _,invocation:=range ids {if runtime.InferenceClient==nil {return platform.NewError(platform.CodeDependencyUnavailable,false,"提取模型取消未装配")};if err=runtime.InferenceClient.CancelInference(work,"knowledge.cancel.extract.model."+platform.HashBytes([]byte(id+":"+invocation)),invocation,"knowledge.cancelled");err!=nil {return err}}
 save,tx,cancel,err:=BeginIndexWrite(ctx,op);if err!=nil {return err};defer cancel();defer tx.Rollback()
 if _,err=tx.ExecContext(save,`UPDATE extraction_dispatches SET dispatch_state='cancelled',updated_at=? WHERE extraction_id=? AND cancel_requested=1 AND dispatch_state<>'dispatching'`,platform.Now(),id);err!=nil {return storageError(err)}
 if err=tx.Commit();err!=nil {return storageError(err)};return nil
}
