package inbox

import (
 "context"
 "encoding/json"
 "net/url"
 "nice-wiki/libs/platform"
)

// RawEventClient与生成的JobsClient同形，不伪造任务领取/租约方法。
type RawEventClient interface {
 SubscribeEvents(context.Context,string,...url.Values)(platform.EventStream,error)
 AcknowledgeEvent(context.Context,string,any,string)(json.RawMessage,error)
}
type eventConsumer struct { raw RawEventClient; validateAck func(json.RawMessage)error }

// NewEventConsumer只装配生产方严格确认schema，不执行网络请求。
func NewEventConsumer(raw RawEventClient,validateAck func(json.RawMessage)error)(platform.EventConsumerClient,error){
 if raw==nil||validateAck==nil{return nil,platform.NewError(platform.CodeValidation,false,"事件客户端或确认schema未装配")}
 return &eventConsumer{raw,validateAck},nil
}
func(c *eventConsumer) SubscribeEvents(ctx context.Context,consumerID,lastID string)(platform.EventStream,error){
 return c.raw.SubscribeEvents(ctx,lastID,url.Values{"consumerId":{consumerID},"mode":{"durable"}})
}
func(c *eventConsumer) AcknowledgeEvent(ctx context.Context,consumerID,eventID string)error{
 op,ok:=platform.OperationContextFrom(ctx);if !ok{return platform.NewError(platform.CodeDependencyUnavailable,false,"事件确认缺少可信上下文")}
 key:="event.ack."+platform.HashBytes([]byte(consumerID+":"+eventID));op.OperationID=key;op.RootOperationID=eventID
 raw,err:=c.raw.AcknowledgeEvent(platform.WithOperationContext(ctx,op),eventID,struct{ConsumerID string `json:"consumerId"`}{consumerID},key);if err!=nil{return err}
 var result struct{Acknowledged bool `json:"acknowledged"`}
 if c.validateAck(raw)!=nil||json.Unmarshal(raw,&result)!=nil||!result.Acknowledged{return platform.NewError(platform.CodeIntegrityError,false,"事件确认回执不符合生产方合同")};return nil
}
