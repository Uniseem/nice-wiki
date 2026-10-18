package cache

import(
 "context"
 "encoding/json"
 "nice-wiki/libs/platform/inbox"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/outbox"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
func EmitCacheInvalidated(tx *qaruntime.Transaction,op OperationContext,entry gen.CacheEntry,reason string)error{
 payload,_:=json.Marshal(struct{CacheID string `json:"cacheId"`;Version int `json:"version"`;Reason string `json:"reason"`;DataEpoch string `json:"dataEpoch"`}{entry.CacheId,entry.Version,reason,op.DataEpoch})
 return outbox.EnqueueEvent(tx.Tx,platform.EventEnvelope{EventID:"qa.cache.invalid."+platform.HashBytes(payload),EventType:"qa.cache.invalidated",EventVersion:1,Producer:"qa",SubjectID:entry.CacheId,Revision:int64(entry.Version),OccurredAt:platform.Now(),DataEpoch:op.DataEpoch,Payload:payload})
}

type InvalidateCacheInput struct{Event platform.EventEnvelope `json:"event"`}
type InvalidateCacheResult struct{Invalidated int `json:"invalidated"`}

// InvalidateCache 使用真实事件信封去重；记忆事件按当前任务/语言hash仅失效普通旧版本。
func InvalidateCache(ctx context.Context,op OperationContext,input InvalidateCacheInput)(InvalidateCacheResult,error){
 envelope:=input.Event
 if envelope.DataEpoch!=op.DataEpoch{return InvalidateCacheResult{},platform.NewError(platform.CodeStaleResult,false,"失效事件属于其他数据代际")}
 runtime,err:=qaruntime.FromContext(ctx);if err!=nil{return InvalidateCacheResult{},err}
 documentID:="";memoryChanged:=false;publicationChanged:=false
 switch envelope.EventType{
 case "content.publication.changed":
  var payload struct{SiteID string `json:"siteId"`;Reference gen.PublishedRevisionRef `json:"reference"`};if err:=qaruntime.DecodeResponse(runtime,"ContentPublicationChanged",envelope.Payload,&payload);err!=nil{return InvalidateCacheResult{},err};if payload.SiteID!=op.Actor.SiteID||payload.Reference.DataEpoch!=op.DataEpoch{return InvalidateCacheResult{},platform.NewError(platform.CodeStaleResult,false,"发布事件站点或代际不一致")};documentID=payload.Reference.DocumentId;publicationChanged=true
 case "content.publication.withdrawn":
  var payload struct{SiteID string `json:"siteId"`;DocumentID string `json:"documentId"`;DataEpoch string `json:"dataEpoch"`};if err:=qaruntime.DecodeResponse(runtime,"ContentPublicationWithdrawn",envelope.Payload,&payload);err!=nil{return InvalidateCacheResult{},err};if payload.SiteID!=op.Actor.SiteID||payload.DataEpoch!=op.DataEpoch{return InvalidateCacheResult{},platform.NewError(platform.CodeStaleResult,false,"撤回事件站点或代际不一致")};documentID=payload.DocumentID;publicationChanged=true
 case "index.retired":
  var payload struct{DocumentID string `json:"documentId"`;DataEpoch string `json:"dataEpoch"`};if err:=qaruntime.DecodeResponse(runtime,"KnowledgeIndexRetired",envelope.Payload,&payload);err!=nil{return InvalidateCacheResult{},err};if payload.DataEpoch!=op.DataEpoch{return InvalidateCacheResult{},platform.NewError(platform.CodeStaleResult,false,"索引退役事件代际不一致")};documentID=payload.DocumentID
 case "memory.context.changed":
  var payload struct{TaskIDs []string `json:"taskIds"`;DataEpoch string `json:"dataEpoch"`};if err:=qaruntime.DecodeResponse(runtime,"MemoryContextChanged",envelope.Payload,&payload);err!=nil{return InvalidateCacheResult{},err};if payload.DataEpoch!=op.DataEpoch{return InvalidateCacheResult{},platform.NewError(platform.CodeStaleResult,false,"记忆变更事件代际不一致")};for _,task:=range payload.TaskIDs{if task=="qa.answer"{memoryChanged=true}}
 default:return InvalidateCacheResult{},platform.NewError(platform.CodeValidation,false,"未登记的缓存失效事件")
 }
 read,err:=qaruntime.BeginRead(ctx,op);if err!=nil{return InvalidateCacheResult{},err}
 rows,err:=read.Tx.QueryContext(read.Context,`SELECT cache_id FROM answer_cache WHERE site_id=? AND data_epoch=? AND state='active' ORDER BY cache_id`,op.Actor.SiteID,op.DataEpoch);if err!=nil{read.Close();return InvalidateCacheResult{},qaruntime.StorageError(err)}
 ids:=make([]string,0);for rows.Next(){var id string;if err:=rows.Scan(&id);err!=nil{rows.Close();read.Close();return InvalidateCacheResult{},qaruntime.StorageError(err)};ids=append(ids,id)};err=rows.Err();rows.Close();if err!=nil{read.Close();return InvalidateCacheResult{},qaruntime.StorageError(err)}
 languages:=make(map[string]bool);if memoryChanged{for _,id:=range ids{record,err:=ReadCache(read,op,id);if err!=nil{read.Close();return InvalidateCacheResult{},err};if !record.Entry.Curated{languages[record.Entry.Language]=true}}};read.Close()
 currentMemory:=make(map[string]string)
 for language:=range languages{memory,err:=qaruntime.ResolveTaskMemory(ctx,op,"qa.answer",language);if err!=nil{return InvalidateCacheResult{},err};currentMemory[language]=memory.VersionHash}
 tx,err:=qaruntime.BeginWrite(ctx,op);if err!=nil{return InvalidateCacheResult{},err};defer tx.Close()
 claimed,err:=inbox.ClaimInbox(tx.Tx,envelope);if err!=nil{return InvalidateCacheResult{},err};if !claimed{return InvalidateCacheResult{},nil}
 result:=InvalidateCacheResult{}
 for _,id:=range ids{
  record,err:=ReadCache(tx,op,id);if err!=nil{return result,err};entry:=record.Entry;if string(entry.State)!="active"{continue}
  affected:=publicationChanged&&len(entry.Citations)==0
  for _,citation:=range entry.Citations{if documentID!=""&&citation.Document.DocumentId==documentID{affected=true}}
  if memoryChanged&&!entry.Curated{if hash,exists:=currentMemory[entry.Language];exists&&entry.MemoryVersionHash!=hash{affected=true}}
  if !affected{continue};record.Entry.State=gen.CacheEntryState("stale");record.Entry.Version++
  if err:=SaveCache(tx,op,record);err!=nil{return result,err};if err:=EmitCacheInvalidated(tx,op,record.Entry,envelope.EventType);err!=nil{return result,err};result.Invalidated++
 }
 if err:=tx.Commit();err!=nil{return InvalidateCacheResult{},err};return result,nil
}
