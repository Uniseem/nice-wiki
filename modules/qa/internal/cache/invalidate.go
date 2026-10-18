package cache

import(
 "encoding/json"
 "nice-wiki/libs/platform"
 "nice-wiki/libs/platform/outbox"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
func EmitCacheInvalidated(tx *qaruntime.Transaction,op OperationContext,entry gen.CacheEntry,reason string)error{
 payload,_:=json.Marshal(struct{CacheID string `json:"cacheId"`;Version int `json:"version"`;Reason string `json:"reason"`;DataEpoch string `json:"dataEpoch"`}{entry.CacheId,entry.Version,reason,op.DataEpoch})
 return outbox.EnqueueEvent(tx.Tx,platform.EventEnvelope{EventID:"qa.cache.invalid."+platform.HashBytes(payload),EventType:"qa.cache.invalidated",EventVersion:1,Producer:"qa",SubjectID:entry.CacheId,Revision:int64(entry.Version),OccurredAt:platform.Now(),DataEpoch:op.DataEpoch,Payload:payload})
}
