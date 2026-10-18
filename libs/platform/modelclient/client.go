package modelclient

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strconv"
 "unicode/utf8"

	"nice-wiki/libs/platform"
)

// RawGatewayClient 是生成客户端的 HTTP 端口；签名由其传输层负责。
type RawGatewayClient interface {
	CreateInference(context.Context, any) (json.RawMessage, error)
	CancelInference(context.Context, string, any) (json.RawMessage, error)
	ResumeInference(context.Context, string, string, ...url.Values) (platform.ModelEventStream, error)
}


// 兼容窄读接口，不要求旧RawGatewayClient实现新方法。
type RawCurrentGatewayClient interface {ReadInference(context.Context,string)(json.RawMessage,error)}
type RawLookupGatewayClient interface {LookupInference(context.Context,any)(json.RawMessage,error)}

type GatewayResponseValidators = map[string]func(json.RawMessage) error

type gatewayClient struct {
	raw        RawGatewayClient
	validators GatewayResponseValidators
}
type validatedModelStream struct {
	source platform.ModelEventStream
	raw    interface {
		NextRaw(context.Context) (json.RawMessage, error)
	}
	validate     func(json.RawMessage) error
	invocationID string
}

// NewGatewayClient统一适配生成端口，要求五个生产方严格响应校验器。
func NewGatewayClient(raw RawGatewayClient, validators GatewayResponseValidators) (platform.GatewayClient, error) {
	if raw == nil {
		return nil, platform.NewError(platform.CodeValidation, false, "模型网关HTTP客户端缺失")
	}
	copied := GatewayResponseValidators{}
	for _, name := range []string{"CreateInvocationResult", "CancelInvocationResult", "StreamEvent", "GetInvocationResult", "LookupInvocationResult"} {
		if validators[name] == nil {
			return nil, platform.NewError(platform.CodeValidation, false, "模型网关响应校验器缺失")
		}
		copied[name] = validators[name]
	}
	return &gatewayClient{raw: raw, validators: copied}, nil
}

func (c *gatewayClient) decodeInvocation(name string, raw json.RawMessage) (platform.Invocation, error) {
	var result struct {
		Invocation platform.Invocation `json:"invocation"`
	}
	if !utf8.Valid(raw) || c.validators[name](raw) != nil || json.Unmarshal(raw, &result) != nil {
		return platform.Invocation{}, platform.NewError(platform.CodeIntegrityError, false, "模型网关成功响应不符合合同")
	}
	return result.Invocation, nil
}

func (c *gatewayClient) CreateInference(ctx context.Context, key string, input platform.CreateInvocationInput) (platform.Invocation, error) {
	op, ok := platform.OperationContextFrom(ctx)
	if !ok {
		return platform.Invocation{}, platform.NewError(platform.CodeDependencyUnavailable, false, "模型调用缺可信操作上下文")
	}
	input,err:=normalizeGatewayInput(key,input);if err!=nil{return platform.Invocation{},err}
	raw, err := c.raw.CreateInference(ctx, input)
	if err != nil {
		return platform.Invocation{}, err
	}
	invocation, err := c.decodeInvocation("CreateInvocationResult", raw)
	if err != nil {
		return platform.Invocation{}, err
	}
	if invocation.RootOperationID != op.RootOperationID || invocation.CostClass != op.CostClass {
		return platform.Invocation{}, platform.NewError(platform.CodeIntegrityError, false, "模型调用根操作或费用归属不匹配")
	}
	return invocation, nil
}

func (c *gatewayClient) CancelInference(ctx context.Context, key, id, reason string) error {
 op,ok:=platform.OperationContextFrom(ctx);if !ok{return platform.NewError(platform.CodeDependencyUnavailable,false,"模型取消缺原可信操作")}
	// 此生产方RequestBody明确包含invocationId及幂等键，与路径必须一致。
	raw, err := c.raw.CancelInference(ctx, id, struct {
		InvocationID string `json:"invocationId"`
		Reason       string `json:"reason"`
		Key          string `json:"idempotencyKey"`
	}{id, reason, key})
	if err != nil {
		return err
	}
	invocation, err := c.decodeInvocation("CancelInvocationResult", raw)
	if err != nil {
		return err
	}
	if invocation.InvocationID != id || invocation.RootOperationID!=op.RootOperationID || invocation.CostClass!=op.CostClass {
		return platform.NewError(platform.CodeIntegrityError, false, "模型取消回执对象不匹配")
	}
	return nil
}

func (c *gatewayClient) ResumeInference(ctx context.Context, id string, after int64) (platform.ModelEventStream, error) {
	source, err := c.raw.ResumeInference(ctx, id, "", url.Values{"afterSequence": {strconv.FormatInt(after, 10)}, "limit": {"100"}})
	if err != nil {
		return nil, err
	}
	raw, ok := source.(interface {
		NextRaw(context.Context) (json.RawMessage, error)
	})
	if !ok {
		_ = source.Close()
		return nil, platform.NewError(platform.CodeUnsupportedCapability, false, "模型流缺原始严格校验端口")
	}
	return &validatedModelStream{source: source, raw: raw, validate: c.validators["StreamEvent"], invocationID: id}, nil
}

func (s *validatedModelStream) Next(ctx context.Context) (platform.StreamEvent, error) {
	raw, err := s.raw.NextRaw(ctx)
	if err != nil {
		return platform.StreamEvent{}, err
	}
	var event platform.StreamEvent
	if !utf8.Valid(raw) || s.validate(raw) != nil || json.Unmarshal(raw, &event) != nil {
		return platform.StreamEvent{}, platform.NewError(platform.CodeIntegrityError, false, "模型事件不符合生产方合同")
	}
	if event.Invocation != nil && event.Invocation.InvocationID != s.invocationID {
		return platform.StreamEvent{}, platform.NewError(platform.CodeIntegrityError, false, "模型事件对象不匹配")
	}
	if event.Output != nil && event.Event != platform.StreamCompleted {
		return platform.StreamEvent{}, platform.NewError(platform.CodeIntegrityError, false, "结构化模型结果不在完成事件中")
	}
	if err:=validateModelOutput(event.Output);err!=nil{return platform.StreamEvent{},err}
	return event, nil
}
func (s *validatedModelStream) Close() error { return s.source.Close() }

func (s *validatedModelStream) LastID() string { return s.source.LastID() }

func validateModelOutput(output *platform.ModelOutput)error{
	if output!=nil {
		switch output.Capability {
		case "chat","vision","translation":
 if output.Text==nil||!utf8.ValidString(*output.Text)||len(*output.Text)>64<<10||output.EmbeddingSpace!=nil||len(output.Vectors)>0||len(output.CandidateIDs)>0{return platform.NewError(platform.CodeIntegrityError,false,"文本结果缺失、超UTF8预算或混入结构化数据")}
 case "embedding":
			if output.Text!=nil||output.EmbeddingSpace == nil || output.EmbeddingSpace.Dimensions < 1 || len(output.CandidateIDs) > 0 {
				return platform.NewError(platform.CodeIntegrityError, false, "向量结果空间缺失或包含混合结果")
			}
			for _, vector := range output.Vectors {
				if int64(len(vector)) != output.EmbeddingSpace.Dimensions {
					return platform.NewError(platform.CodeIntegrityError, false, "向量维度与空间不一致")
				}
				for _, v := range vector {
					if math.IsInf(v, 0) || math.IsNaN(v) {
						return platform.NewError(platform.CodeIntegrityError, false, "向量包含非有限值")
					}
				}
			}
		case "rerank":
			if output.Text!=nil||output.EmbeddingSpace != nil || len(output.Vectors) > 0 {
				return platform.NewError(platform.CodeIntegrityError, false, "重排结果包含向量数据")
			}
		default:
			return platform.NewError(platform.CodeUnsupportedSchema, false, "结构化结果能力未知")
		}
	}

return nil
}

func normalizeGatewayInput(key string,input platform.CreateInvocationInput)(platform.CreateInvocationInput,error){
 if input.IDempotencyKey!=""&&input.IDempotencyKey!=key{return input,platform.NewError(platform.CodeConflict,false,"模型原请求键与调用步骤键不一致")}
 input.IDempotencyKey=key
 if input.Messages==nil{input.Messages=[]platform.GatewayMessage{}}
 if input.Texts==nil{input.Texts=[]string{}}
 if input.CandidateIDs==nil{input.CandidateIDs=[]platform.Identifier{}}
 return input,nil
}
func(c *gatewayClient)decodeGatewaySnapshot(name string,raw json.RawMessage)(platform.GatewayInvocationSnapshot,error){
 var result platform.GatewayInvocationSnapshot
 if !utf8.Valid(raw)||c.validators[name](raw)!=nil||json.Unmarshal(raw,&result)!=nil{return result,platform.NewError(platform.CodeIntegrityError,false,"模型当前权威响应不符合生产方合同")}
 if err:=validateModelOutput(result.Output);err!=nil{return platform.GatewayInvocationSnapshot{},err}
 if result.Output!=nil{if result.Invocation.ResultAttemptID==nil{return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeIntegrityError,false,"真实输出缺winning attempt来源")};found:=false;for _,attempt:=range result.Invocation.Attempts{if attempt.AttemptID==*result.Invocation.ResultAttemptID&&attempt.InvocationID==result.Invocation.InvocationID{found=true}};if !found{return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeIntegrityError,false,"真实输出winning来源不属于当前调用")}}
 return result,nil
}
func(c *gatewayClient)LookupInference(ctx context.Context,key string,input platform.CreateInvocationInput)(platform.GatewayInvocationSnapshot,error){
 rawClient,ok:=c.raw.(RawLookupGatewayClient);if !ok{return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeUnsupportedCapability,false,"模型客户端缺原键只读查找能力")}
 op,ok:=platform.OperationContextFrom(ctx);if !ok{return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeDependencyUnavailable,false,"模型查找缺原可信操作")}
 input,err:=normalizeGatewayInput(key,input);if err!=nil{return platform.GatewayInvocationSnapshot{},err}
 raw,err:=rawClient.LookupInference(ctx,input);if err!=nil{return platform.GatewayInvocationSnapshot{},err};snapshot,err:=c.decodeGatewaySnapshot("LookupInvocationResult",raw);if err!=nil{return snapshot,err}
 if snapshot.Invocation.RootOperationID!=op.RootOperationID||snapshot.Invocation.CostClass!=op.CostClass||(snapshot.Output!=nil&&snapshot.Output.Capability!=input.Capability){return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeIntegrityError,false,"模型原键查找的根/费用/结果能力不匹配")};return snapshot,nil
}
func(c *gatewayClient)GetInference(ctx context.Context,id string)(platform.GatewayInvocationSnapshot,error){
 rawClient,ok:=c.raw.(RawCurrentGatewayClient);if !ok{return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeUnsupportedCapability,false,"模型客户端缺当前权威只读能力")}
 op,ok:=platform.OperationContextFrom(ctx);if !ok{return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeDependencyUnavailable,false,"模型当前读取缺原可信操作")}
 raw,err:=rawClient.ReadInference(ctx,id);if err!=nil{return platform.GatewayInvocationSnapshot{},err};snapshot,err:=c.decodeGatewaySnapshot("GetInvocationResult",raw);if err!=nil{return snapshot,err}
 if snapshot.Invocation.InvocationID!=id||snapshot.Invocation.RootOperationID!=op.RootOperationID||snapshot.Invocation.CostClass!=op.CostClass{return platform.GatewayInvocationSnapshot{},platform.NewError(platform.CodeIntegrityError,false,"模型当前读取的对象/原根/费用归属不匹配")};return snapshot,nil
}
