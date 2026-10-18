package debug

import(
 "context"
 "database/sql"
 "encoding/json"
 "slices"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
type OperationContext=platform.OperationContext
type ReadDebugTurnInput struct{DebugID string `json:"debugId"`;AfterSequence int `json:"afterSequence"`}
type ReadDebugTurnResult=gen.ReadDebugTurnResult

func ReadDebugTurn(ctx context.Context,op OperationContext,input ReadDebugTurnInput)(ReadDebugTurnResult,error){
 if err:=qaruntime.RequireAdmin(op,"qa.debug");err!=nil{return ReadDebugTurnResult{},err}
 tx,err:=qaruntime.BeginRead(ctx,op);if err!=nil{return ReadDebugTurnResult{},err};defer tx.Close()
 var raw []byte;var actor string;var expires int64
 err=tx.Tx.QueryRowContext(tx.Context,`SELECT debug_json,actor_id,expires_ns FROM debug_turns WHERE debug_id=? AND site_id=? AND data_epoch=?`,input.DebugID,op.Actor.SiteID,op.DataEpoch).Scan(&raw,&actor,&expires)
 if err==sql.ErrNoRows{return ReadDebugTurnResult{},platform.NewError(platform.CodeNotFound,false,"隔离诊断不存在")};if err!=nil{return ReadDebugTurnResult{},qaruntime.StorageError(err)}
 if actor!=op.Actor.SubjectID&&!slices.Contains(op.Actor.Grants,"model.manage"){return ReadDebugTurnResult{},platform.NewError(platform.CodeForbidden,false,"只能读取自己创建的隔离诊断")}
 if expires<=time.Now().UnixNano(){return ReadDebugTurnResult{},platform.NewError(platform.CodeGone,false,"隔离诊断已超过保留期")}
 var record gen.DebugTurn;if err:=qaruntime.DecodeResponse(tx.Runtime,"QaDebugTurn",raw,&record);err!=nil{return ReadDebugTurnResult{},err};if record.DebugId!=input.DebugID||record.ActorId!=actor||string(record.Namespace)!="debug"{return ReadDebugTurnResult{},platform.NewError(platform.CodeIntegrityError,false,"隔离诊断记录身份不一致")}
 rows,err:=tx.Tx.QueryContext(tx.Context,`SELECT event_json FROM debug_events WHERE debug_id=? AND sequence>? ORDER BY sequence LIMIT 256`,input.DebugID,input.AfterSequence);if err!=nil{return ReadDebugTurnResult{},qaruntime.StorageError(err)};defer rows.Close()
 result:=ReadDebugTurnResult{Debug:record,Events:make([]gen.AnswerEvent,0)};last:=input.AfterSequence
 for rows.Next(){var raw []byte;var event gen.AnswerEvent;if err:=rows.Scan(&raw);err!=nil{return ReadDebugTurnResult{},qaruntime.StorageError(err)};if json.Unmarshal(raw,&event)!=nil||event.Sequence!=last+1{return ReadDebugTurnResult{},platform.NewError(platform.CodeIntegrityError,false,"隔离诊断事件不连续")};last=event.Sequence;result.Events=append(result.Events,event)}
 if err:=rows.Err();err!=nil{return ReadDebugTurnResult{},qaruntime.StorageError(err)};return result,nil
}
