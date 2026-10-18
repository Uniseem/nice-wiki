package runtime

import (
 "context"
 "database/sql"
 "time"
 "nice-wiki/libs/platform"
)

type RecoverAssistantLeasesInput struct{Now time.Time `json:"now"`;Limit int `json:"limit"`}
type RecoverAssistantLeasesResult struct{Recovered int `json:"recovered"`;Superseded int `json:"superseded"`}
// RecoverAssistantLeases 只重投原任务和取消；租约由jobs/平台worker维护，MG以原操作幂等续读。
func RecoverAssistantLeases(ctx context.Context,op OperationContext,input RecoverAssistantLeasesInput)(RecoverAssistantLeasesResult,error){
 if op.CallerService!="memory"||op.Actor.Kind!="service"||op.Actor.SubjectID!="memory"{return RecoverAssistantLeasesResult{},platform.NewError(platform.CodeForbidden,false,"助手恢复只接受本域启动身份")}
 s:=CurrentStore();if s==nil{return RecoverAssistantLeasesResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"记忆存储未打开")};result:=RecoverAssistantLeasesResult{}
 rows,err:=s.db.QueryContext(ctx,`SELECT r.run_id,r.site_id FROM assistant_runs r CROSS JOIN runtime_state s WHERE s.id=1 AND r.source_generation<>s.memory_generation AND r.state IN('queued','running','review') ORDER BY r.run_id LIMIT ?`,input.Limit);if err!=nil{return result,DBError(err)}
 type target struct{run,site string};targets:=[]target{};for rows.Next(){var item target;if err=rows.Scan(&item.run,&item.site);err!=nil{rows.Close();return result,DBError(err)};targets=append(targets,item)};err=rows.Err();rows.Close();if err!=nil{return result,DBError(err)}
 for _,item:=range targets{taskOp:=op;taskOp.Actor.SiteID=item.site;var changed struct{Changed bool};err=Write(ctx,taskOp,"memory.SupersedeAssistantRun",item.run,item.run,&changed,func(_ *Store,tx *sql.Tx)error{
  res,err:=tx.ExecContext(ctx,`UPDATE assistant_runs SET state='superseded' WHERE site_id=? AND run_id=? AND state IN('queued','running','review') AND source_generation<>(SELECT memory_generation FROM runtime_state WHERE id=1)`,item.site,item.run);if err!=nil{return DBError(err)};n,_:=res.RowsAffected();changed.Changed=n>0
  if changed.Changed{_,err=tx.ExecContext(ctx,`UPDATE assistant_inputs SET dispatch_state='cancelled' WHERE run_id=?`,item.run)};return DBError(err)
 });if err!=nil{return result,err};if changed.Changed{result.Superseded++}}
 if err=DispatchAssistantCancellations(ctx,input.Limit);err!=nil{return result,err}
 result.Recovered,err=DispatchAssistantInputs(ctx,input.Limit);return result,err
}
