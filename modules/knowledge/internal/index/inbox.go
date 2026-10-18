package index

import (
 "context"
 "database/sql"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/inbox"
)

type ConsumeInboxInput struct { Event platform.EventEnvelope }
type ConsumeInboxResult struct { Duplicate bool `json:"duplicate"`;Accepted bool `json:"accepted"` }

// ConsumeInbox 接收公共订阅边界已按生产schema核验的原信封；不重构替代事件。
// tx必须是handler最先建立的业务事务，inbox和业务提交保持原子。
func ConsumeInbox(ctx context.Context,op OperationContext,input ConsumeInboxInput,tx *sql.Tx)(ConsumeInboxResult,error) {
 event:=input.Event
 if op.CallerService!="knowledge"||op.Actor.Kind!="service"||event.Producer!="content"||event.EventVersion!=1||(event.EventType!="content.publication.changed"&&event.EventType!="content.publication.withdrawn") { return ConsumeInboxResult{},platform.NewError(platform.CodeForbidden,false,"索引只接受登记的content发布事件") }
 if event.DataEpoch!=op.DataEpoch { return ConsumeInboxResult{},platform.NewError(platform.CodeStaleResult,false,"索引事件代次不一致") }
 claimed,err:=inbox.ClaimInboxBeforeWrite(ctx,tx,event,tx.Commit)
 if err!=nil { return ConsumeInboxResult{},err }
 return ConsumeInboxResult{Duplicate:!claimed,Accepted:claimed},nil
}
