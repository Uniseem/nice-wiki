package invocation

import (
 "context"
 "encoding/json"
 "io"
 "net/http"
 "strconv"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/apivalidate"
 "nice-wiki/libs/platform/problem"
 "nice-wiki/modules/model-gateway/internal/billing"
 "nice-wiki/modules/model-gateway/internal/stream"
)

func NewExecutionHandlers()map[string]http.Handler{return map[string]http.Handler{
 "readInference":http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){op,_:=platform.OperationContextFrom(r.Context());result,err:=GetInvocation(r.Context(),op,GetInvocationInput{InvocationID:r.PathValue("invocationId")});writeExecutionResult(w,op,result,err)}),
 "reconcileCharge":http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){op,_:=platform.OperationContextFrom(r.Context());var input billing.ReconcileChargeInput;if json.NewDecoder(r.Body).Decode(&input)!=nil{writeExecutionResult(w,op,nil,platform.NewError(platform.CodeValidation,false,"费用证明请求不可读"));return};if input.InvocationID!=r.PathValue("invocationId"){writeExecutionResult(w,op,nil,platform.NewError(platform.CodeValidation,false,"费用证明路径与对象不一致"));return};result,err:=billing.ReconcileCharge(r.Context(),op,input);writeExecutionResult(w,op,result,err)}),
 "resumeInference":http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){op,_:=platform.OperationContextFrom(r.Context());after,_:=strconv.ParseInt(r.URL.Query().Get("afterSequence"),0,64);limit,_:=strconv.ParseInt(r.URL.Query().Get("limit"),0,64);if value:=r.Header.Get("Last-Event-ID");value!=""{cursor,err:=strconv.ParseInt(value,10,64);if err!=nil||cursor<0{writeExecutionResult(w,op,nil,platform.NewError(platform.CodeValidation,false,"模型流续读游标不合法"));return};after=max(after,cursor)};input:=stream.ResumeStreamInput{InvocationID:r.PathValue("invocationId"),AfterSequence:int(after),Limit:int(limit)};batch,err:=stream.ResumeStream(r.Context(),op,input);if err!=nil{writeExecutionResult(w,op,nil,err);return};ctx,cancel:=context.WithTimeout(r.Context(),5*time.Minute);reader:=&invocationEventStream{op:op,input:input,batch:batch,cancel:cancel};_=apivalidate.ServeJSONEventStream(ctx,w,reader)}),
}}
func writeExecutionResult(w http.ResponseWriter,op OperationContext,result any,err error){w.Header().Set("Cache-Control","no-store");if err!=nil{_=problem.WriteProblem(w,problem.FromError(err,op.RequestID));return};w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(result)}
type invocationEventStream struct{op OperationContext;input stream.ResumeStreamInput;batch stream.ResumeStreamResult;cancel context.CancelFunc}
func(s *invocationEventStream)Next(ctx context.Context)(platform.SSEFrame,error){for{if len(s.batch.Events)>0{event:=s.batch.Events[0];s.batch.Events=s.batch.Events[1:];s.input.AfterSequence=event.Sequence;raw,err:=json.Marshal(event);if err!=nil{return platform.SSEFrame{},platform.NewError(platform.CodeIntegrityError,false,"模型流事件无法编码")};return platform.SSEFrame{ID:strconv.Itoa(event.Sequence),Event:event.Event,Data:raw},nil};if s.batch.Terminal{return platform.SSEFrame{},io.EOF};timer:=time.NewTimer(250*time.Millisecond);select{case<-ctx.Done():timer.Stop();return platform.SSEFrame{},ctx.Err();case<-timer.C:};batch,err:=stream.ResumeStream(ctx,s.op,s.input);if err!=nil{return platform.SSEFrame{},err};s.batch=batch}}
func(s *invocationEventStream)Close()error{s.cancel();return nil}
// 编译期约束流实现仅返回持久的本域结构，不转发供应方原始正文。
var _ platform.JSONEventStream=(*invocationEventStream)(nil)
