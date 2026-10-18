package extract

import (
 "context"
 "database/sql"
 "encoding/json"
 "nice-wiki/libs/platform"
 indexer "nice-wiki/modules/knowledge/internal/index"
)

// DispatchExtraction 只派发已持久批准的独立任务，网络未知后复用原完整输入和去重键。
func DispatchExtraction(ctx context.Context,op OperationContext,id string)error {
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return err}
 if runtime.Jobs==nil||runtime.Validators["JobsCreateJobResult"]==nil {return platform.NewError(platform.CodeDependencyUnavailable,false,"提取任务生产方未装配")}
 read,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取任务不可读")}
 original,state,_,site,cancelled,err:=indexer.ApprovedJobInput(ctx,read,"knowledge.extract",id);_=read.Rollback();if err!=nil {return err}
 if site!=op.Actor.SiteID||original.Operation.DataEpoch!=op.DataEpoch {return platform.NewError(platform.CodeForbidden,false,"原提取任务不属于本站同代")}
 if cancelled||(state!="pending"&&state!="dispatching") {return nil}
 service:=original.Operation;service.CallerService="knowledge";service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:site,ExpiresAt:service.Deadline,Grants:[]string{}}
 work,tx,closeWrite,err:=indexer.BeginIndexWrite(ctx,op);if err!=nil {return err}
 updated,err:=tx.ExecContext(work,`UPDATE extraction_dispatches SET dispatch_state='dispatching',updated_at=? WHERE extraction_id=? AND dispatch_state IN ('pending','dispatching') AND cancel_requested=0`,platform.Now(),id)
 if err!=nil {tx.Rollback();closeWrite();return platform.NormalizeIOError(err,platform.CodeInternal,false,"提取派发资格保存失败")};n,err:=updated.RowsAffected();if err!=nil {tx.Rollback();closeWrite();return platform.NormalizeIOError(err,platform.CodeInternal,false,"提取派发资格不可读")};if n==0 {tx.Rollback();closeWrite();return nil}
 if err=tx.Commit();err!=nil {closeWrite();return platform.NormalizeIOError(err,platform.CodeInternal,false,"提取派发资格提交失败")};closeWrite()
 key:="knowledge.extract."+id
 raw,err:=runtime.Jobs.CreateJob(platform.WithOperationContext(platform.WithPortalProof(ctx,""),service),struct{Owner string `json:"owner"`;Type string `json:"type"`;InputRef platform.ObjectRef `json:"inputRef"`;DedupeKey string `json:"dedupeKey"`;DataEpoch string `json:"dataEpoch"`}{"knowledge","knowledge.extract",original.InputRef,key,original.Operation.DataEpoch},key)
 if err!=nil {return err};if err=runtime.Validators["JobsCreateJobResult"](raw);err!=nil {return err}
 var accepted struct{Job struct{JobID string `json:"jobId"`;OperationID string `json:"operationId"`} `json:"job"`}
 if json.Unmarshal(raw,&accepted)!=nil||accepted.Job.OperationID!=original.Operation.OperationID {return platform.NewError(platform.CodeIntegrityError,false,"提取任务原operation回执不一致")}
 save,finish,cancel,err:=indexer.BeginIndexWrite(ctx,op);if err!=nil {return err};defer cancel();defer finish.Rollback()
 // RPC期间取消仍保留真实job引用；后续原任务清理/owner证明不能丢失已接受任务。
 _,err=finish.ExecContext(save,`UPDATE extraction_dispatches SET job_id=?,jobs_operation_id=?,dispatch_state='dispatched',updated_at=? WHERE extraction_id=? AND dispatch_state='dispatching'`,accepted.Job.JobID,accepted.Job.OperationID,platform.Now(),id)
 if err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"真实提取任务引用保存失败")};if err=finish.Commit();err!=nil {return platform.NormalizeIOError(err,platform.CodeInternal,false,"真实提取任务引用提交失败")};return nil
}

// ReadExtractionJobState 只按本域原任务引用读取jobs权威状态；本地attempt失败不等于整个任务终局。
func ReadExtractionJobState(ctx context.Context,op OperationContext,id string)(string,error) {
 runtime,err:=indexer.RuntimeFromContext(ctx);if err!=nil {return "",err}
 if runtime.Jobs==nil||runtime.Validators["JobsReadJobResult"]==nil {return "",platform.NewError(platform.CodeDependencyUnavailable,false,"任务owner只读状态未装配")}
 read,err:=runtime.DB.BeginTx(ctx,&sql.TxOptions{ReadOnly:true});if err!=nil {return "",platform.NormalizeIOError(err,platform.CodeInternal,false,"原提取任务引用不可读")}
 original,_,_,site,_,err:=indexer.ApprovedJobInput(ctx,read,"knowledge.extract",id);_=read.Rollback();if err!=nil {return "",err}
 if site!=op.Actor.SiteID||original.Operation.DataEpoch!=op.DataEpoch {return "",platform.NewError(platform.CodeForbidden,false,"原提取任务状态不属于本站同代")}
 if original.JobID=="" {return "",nil}
 service:=op;service.RootOperationID=original.Operation.RootOperationID;service.CallerService="knowledge";service.Actor=platform.AuthContext{SubjectID:"knowledge",Kind:"service",SiteID:site,ExpiresAt:op.Deadline,Grants:[]string{}}
 raw,err:=runtime.Jobs.ReadJob(platform.WithOperationContext(platform.WithPortalProof(ctx,""),service),original.JobID)
 if err!=nil {return "",err};if err=runtime.Validators["JobsReadJobResult"](raw);err!=nil {return "",err}
 var result struct{State string `json:"state"`}
 if json.Unmarshal(raw,&result)!=nil {return "",platform.NewError(platform.CodeIntegrityError,false,"原任务状态回执不可读")}
 switch result.State {case "queued","leased","running","retry-wait","dead-letter","cancel-requested","cancelled","succeeded","orphan":return result.State,nil}
 return "",platform.NewError(platform.CodeIntegrityError,false,"原任务状态不是已登记生产状态")
}
