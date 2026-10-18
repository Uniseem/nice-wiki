package stream

import(
 "context"
 "encoding/json"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
type ResumeAnswerStreamInput struct{TurnID string `json:"turnId"`;AfterSequence int `json:"afterSequence"`;Limit int `json:"limit"`}
type ResumeAnswerStreamResult struct{Events []gen.AnswerEvent `json:"events"`;Terminal bool `json:"terminal"`;LastSequence int `json:"lastSequence"`}

// ResumeAnswerStream 只续读持久事件，不触发业务或模型；HTTP层另发不占序号的心跳。
func ResumeAnswerStream(ctx context.Context,op OperationContext,input ResumeAnswerStreamInput)(ResumeAnswerStreamResult,error){
 tx,err:=qaruntime.BeginRead(ctx,op);if err!=nil{return ResumeAnswerStreamResult{},err};defer tx.Close()
 record,err:=qaruntime.ReadTurn(tx,op,input.TurnID,true);if err!=nil{return ResumeAnswerStreamResult{},err}
 if record.TerminalNS>0&&time.Since(time.Unix(0,record.TerminalNS))>24*time.Hour{return ResumeAnswerStreamResult{},platform.NewError(platform.CodeGone,false,"流事件已超过保留期")}
 if input.AfterSequence>record.Turn.LastSequence{return ResumeAnswerStreamResult{},platform.NewError(platform.CodeValidation,false,"流游标超过最后持久事件")}
 rows,err:=tx.Tx.QueryContext(tx.Context,`SELECT event_json FROM turn_events WHERE turn_id=? AND sequence>? ORDER BY sequence LIMIT ?`,input.TurnID,input.AfterSequence,input.Limit);if err!=nil{return ResumeAnswerStreamResult{},qaruntime.StorageError(err)};defer rows.Close()
 result:=ResumeAnswerStreamResult{Events:make([]gen.AnswerEvent,0),LastSequence:input.AfterSequence}
 for rows.Next(){var raw []byte;var event gen.AnswerEvent;if err:=rows.Scan(&raw);err!=nil{return ResumeAnswerStreamResult{},qaruntime.StorageError(err)};if json.Unmarshal(raw,&event)!=nil||event.Sequence!=result.LastSequence+1{return ResumeAnswerStreamResult{},platform.NewError(platform.CodeIntegrityError,false,"流事件序号不连续")};result.Events=append(result.Events,event);result.LastSequence=event.Sequence}
 if err:=rows.Err();err!=nil{return ResumeAnswerStreamResult{},qaruntime.StorageError(err)}
 result.Terminal=qaruntime.IsTerminal(record.State)&&result.LastSequence==record.Turn.LastSequence
 return result,nil
}
