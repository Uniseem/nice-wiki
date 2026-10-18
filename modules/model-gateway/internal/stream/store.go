package stream

import (
 "context"
 "encoding/json"
 "unicode/utf8"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/model-gateway/internal/gen"
 gatewayruntime "nice-wiki/modules/model-gateway/internal/runtime"
)
type OperationContext=platform.OperationContext
type AppendStreamEventInput struct{InvocationID string `json:"invocationId"`;Event gatewayruntime.StreamEvent `json:"event"`;LeaseID string `json:"leaseId"`}
type AppendStreamEventResult struct{Sequence int `json:"sequence"`}
type ResumeStreamInput struct{InvocationID string `json:"invocationId"`;AfterSequence int `json:"afterSequence"`;Limit int `json:"limit"`}
type ResumeStreamResult struct{Events []gatewayruntime.StreamEvent `json:"events"`;LastSequence int `json:"lastSequence"`;Terminal bool `json:"terminal"`}
func AppendStreamEvent(ctx context.Context,op OperationContext,input AppendStreamEventInput)(AppendStreamEventResult,error){
 if input.Event.Event!="delta"&&input.Event.Event!="usage"{return AppendStreamEventResult{},platform.NewError(platform.CodeValidation,false,"接受及终态事件必须与对应状态同事务保存")}
 if input.Event.Output!=nil{return AppendStreamEventResult{},platform.NewError(platform.CodeValidation,false,"结构输出只允许completed事件")}
 if input.Event.Delta!=nil&&(!utf8.ValidString(*input.Event.Delta)||len(*input.Event.Delta)>16384){return AppendStreamEventResult{},platform.NewError(platform.CodeValidation,false,"模型delta必须完整UTF8且不超过16KiB")}
 tx,err:=gatewayruntime.BeginExecution(ctx,op,true);if err!=nil{return AppendStreamEventResult{},err};defer tx.Close();record,err:=gatewayruntime.ReadInvocationRecord(tx,op,input.InvocationID);if err!=nil{return AppendStreamEventResult{},err}
 if record.LeaseID!=input.LeaseID||record.LeaseUntilNS<=time.Now().UnixNano()||record.CancelRequested||InvocationTerminal(string(record.Invocation.State)){return AppendStreamEventResult{},platform.NewError(platform.CodeStaleResult,false,"模型流租约、取消或状态已经变化")}
 if input.Event.Event=="delta"&&record.Invocation.State=="dispatching"{attempt,err:=gatewayruntime.ReadAttemptRecord(tx,record.ActiveAttemptID);if err!=nil{return AppendStreamEventResult{},err};attempt.Attempt.State=gen.AttemptState("streaming");if err:=gatewayruntime.SaveAttemptRecord(tx,attempt);err!=nil{return AppendStreamEventResult{},err};record.Invocation.State=gen.InvocationState("streaming");for i:=range record.Invocation.Attempts{if record.Invocation.Attempts[i].AttemptId==attempt.Attempt.AttemptId{record.Invocation.Attempts[i]=attempt.Attempt}}}
 if err:=gatewayruntime.SaveInvocationEvent(tx,&record,input.Event);err!=nil{return AppendStreamEventResult{},err};if err:=tx.Commit();err!=nil{return AppendStreamEventResult{},err};return AppendStreamEventResult{Sequence:record.Invocation.LastSequence},nil
}
func ResumeStream(ctx context.Context,op OperationContext,input ResumeStreamInput)(ResumeStreamResult,error){
 tx,err:=gatewayruntime.BeginExecution(ctx,op,false);if err!=nil{return ResumeStreamResult{},err};defer tx.Close();record,err:=gatewayruntime.ReadInvocationRecord(tx,op,input.InvocationID);if err!=nil{return ResumeStreamResult{},err}
 if input.AfterSequence>record.Invocation.LastSequence{return ResumeStreamResult{},platform.NewError(platform.CodeValidation,false,"续读游标超过最后持久序号")};if input.Limit==0{input.Limit=20}
 var minimum int;if err:=tx.Tx.QueryRowContext(tx.Context,`SELECT COALESCE(MIN(sequence),0) FROM stream_events WHERE invocation_id=?`,input.InvocationID).Scan(&minimum);err!=nil{return ResumeStreamResult{},gatewayruntime.ExecutionStorageError(err)}
 if input.AfterSequence<record.Invocation.LastSequence&&(minimum==0||input.AfterSequence<minimum-1){return ResumeStreamResult{},platform.NewError(platform.CodeGone,false,"所需模型事件已超过保留区")}
 rows,err:=tx.Tx.QueryContext(tx.Context,`SELECT payload_json FROM stream_events WHERE invocation_id=? AND sequence>? ORDER BY sequence LIMIT ?`,input.InvocationID,input.AfterSequence,input.Limit);if err!=nil{return ResumeStreamResult{},gatewayruntime.ExecutionStorageError(err)};defer rows.Close();result:=ResumeStreamResult{Events:make([]gatewayruntime.StreamEvent,0),LastSequence:record.Invocation.LastSequence,Terminal:record.TerminalNS>0};sequence:=input.AfterSequence
 for rows.Next(){var raw []byte;var event gatewayruntime.StreamEvent;if err:=rows.Scan(&raw);err!=nil{return result,gatewayruntime.ExecutionStorageError(err)};if json.Unmarshal(raw,&event)!=nil||event.Sequence!=sequence+1{return ResumeStreamResult{},platform.NewError(platform.CodeIntegrityError,false,"模型持久流序号不连续")};sequence=event.Sequence;result.Events=append(result.Events,event)};if err:=rows.Err();err!=nil{return ResumeStreamResult{},gatewayruntime.ExecutionStorageError(err)};return result,nil
}
func InvocationTerminal(state string)bool{return state=="succeeded"||state=="failed"||state=="cancelled"||state=="charge_unknown"||state=="reconciled"}
