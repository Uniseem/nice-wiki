package modelclient

import (
 "context"
 "encoding/json"
 "net/url"
 "strconv"
 "math"

 "nice-wiki/libs/platform"
)

// RawGatewayClient 是生成客户端的 HTTP 端口；签名由其传输层负责。
type RawGatewayClient interface {
 CreateInference(context.Context,any)(json.RawMessage,error)
 CancelInference(context.Context,string,any)(json.RawMessage,error)
 ResumeInference(context.Context,string,string,...url.Values)(platform.ModelEventStream,error)
}

type GatewayResponseValidators = map[string]func(json.RawMessage)error

type gatewayClient struct {raw RawGatewayClient;validators GatewayResponseValidators}
type validatedModelStream struct {source platform.ModelEventStream;raw interface{NextRaw(context.Context)(json.RawMessage,error)};validate func(json.RawMessage)error;invocationID string}

// NewGatewayClient 统一适配生成端口，要求三个生产方严格响应校验器。
func NewGatewayClient(raw RawGatewayClient,validators GatewayResponseValidators)(platform.GatewayClient,error){
 if raw==nil{return nil,platform.NewError(platform.CodeValidation,false,"模型网关HTTP客户端缺失")}
 copied:=GatewayResponseValidators{}
 for _,name:=range []string{"CreateInvocationResult","CancelInvocationResult","StreamEvent"}{if validators[name]==nil{return nil,platform.NewError(platform.CodeValidation,false,"模型网关响应校验器缺失")};copied[name]=validators[name]}
 return &gatewayClient{raw:raw,validators:copied},nil
}

func (c *gatewayClient) decodeInvocation(name string,raw json.RawMessage)(platform.Invocation,error){
 var result struct {Invocation platform.Invocation `json:"invocation"`}
 if c.validators[name](raw)!=nil||json.Unmarshal(raw,&result)!=nil{return platform.Invocation{},platform.NewError(platform.CodeIntegrityError,false,"模型网关成功响应不符合合同")}
 return result.Invocation,nil
}

func (c *gatewayClient) CreateInference(ctx context.Context,key string,input platform.CreateInvocationInput)(platform.Invocation,error){
 op,ok:=platform.OperationContextFrom(ctx);if !ok{return platform.Invocation{},platform.NewError(platform.CodeDependencyUnavailable,false,"模型调用缺可信操作上下文")}
 input.IDempotencyKey=key
 if input.Messages==nil{input.Messages=[]platform.GatewayMessage{}};if input.Texts==nil{input.Texts=[]string{}};if input.CandidateIDs==nil{input.CandidateIDs=[]string{}}
 raw,err:=c.raw.CreateInference(ctx,input);if err!=nil{return platform.Invocation{},err}
 invocation,err:=c.decodeInvocation("CreateInvocationResult",raw);if err!=nil{return platform.Invocation{},err}
 if invocation.RootOperationID!=op.RootOperationID||invocation.CostClass!=op.CostClass{return platform.Invocation{},platform.NewError(platform.CodeIntegrityError,false,"模型调用根操作或费用归属不匹配")}
 return invocation,nil
}

func (c *gatewayClient) CancelInference(ctx context.Context,key,id,reason string)error{
 // 此生产方RequestBody明确包含invocationId及幂等键，与路径必须一致。
 raw,err:=c.raw.CancelInference(ctx,id,struct {InvocationID string `json:"invocationId"`;Reason string `json:"reason"`;Key string `json:"idempotencyKey"`}{id,reason,key});if err!=nil{return err}
 invocation,err:=c.decodeInvocation("CancelInvocationResult",raw);if err!=nil{return err};if invocation.InvocationID!=id{return platform.NewError(platform.CodeIntegrityError,false,"模型取消回执对象不匹配")};return nil
}

func (c *gatewayClient) ResumeInference(ctx context.Context,id string,after int64)(platform.ModelEventStream,error){
 source,err:=c.raw.ResumeInference(ctx,id,"",url.Values{"afterSequence":{strconv.FormatInt(after,10)},"limit":{"100"}});if err!=nil{return nil,err}
 raw,ok:=source.(interface{NextRaw(context.Context)(json.RawMessage,error)});if !ok{_ = source.Close();return nil,platform.NewError(platform.CodeUnsupportedCapability,false,"模型流缺原始严格校验端口")}
 return &validatedModelStream{source:source,raw:raw,validate:c.validators["StreamEvent"],invocationID:id},nil
}

func (s *validatedModelStream) Next(ctx context.Context)(platform.StreamEvent,error){
 raw,err:=s.raw.NextRaw(ctx);if err!=nil{return platform.StreamEvent{},err}
 var event platform.StreamEvent
 if s.validate(raw)!=nil||json.Unmarshal(raw,&event)!=nil{return platform.StreamEvent{},platform.NewError(platform.CodeIntegrityError,false,"模型事件不符合生产方合同")}
 if event.Invocation!=nil&&event.Invocation.InvocationID!=s.invocationID{return platform.StreamEvent{},platform.NewError(platform.CodeIntegrityError,false,"模型事件对象不匹配")}
 if event.Output!=nil&&event.Event!=platform.StreamCompleted{return platform.StreamEvent{},platform.NewError(platform.CodeIntegrityError,false,"结构化模型结果不在完成事件中")}
 if output:=event.Output;output!=nil {
 switch output.Capability {
 case "embedding":
  if output.EmbeddingSpace==nil||output.EmbeddingSpace.Dimensions<1||len(output.CandidateIDs)>0{return platform.StreamEvent{},platform.NewError(platform.CodeIntegrityError,false,"向量结果空间缺失或包含混合结果")}
  for _,vector:=range output.Vectors {if int64(len(vector))!=output.EmbeddingSpace.Dimensions{return platform.StreamEvent{},platform.NewError(platform.CodeIntegrityError,false,"向量维度与空间不一致")};for _,v:=range vector{if math.IsInf(v,0)||math.IsNaN(v){return platform.StreamEvent{},platform.NewError(platform.CodeIntegrityError,false,"向量包含非有限值")}}}
 case "rerank":if output.EmbeddingSpace!=nil||len(output.Vectors)>0{return platform.StreamEvent{},platform.NewError(platform.CodeIntegrityError,false,"重排结果包含向量数据")}
 default:return platform.StreamEvent{},platform.NewError(platform.CodeUnsupportedSchema,false,"结构化结果能力未知")
 }
 }
 return event,nil
}
func(s *validatedModelStream)Close()error{return s.source.Close()}

func(s *validatedModelStream)LastID()string{return s.source.LastID()}
