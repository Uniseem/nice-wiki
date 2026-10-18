package trafficguard

import (
 "context"
 "database/sql"
 "errors"
)

type ConsumeBucketsInput struct {
 SubjectKey string
 Action string
 Cost int64
 NowMs int64
 DataEpoch string
}
type ConsumeBucketsResult struct {
 Allowed bool `json:"allowed"`
 RetryAfterMs int64 `json:"retryAfterMs"`
}

// ConsumeBuckets 可嵌入准入事务；两个桶及后续并发预留最终一起提交。
func ConsumeBuckets(ctx context.Context,op OperationContext,in ConsumeBucketsInput)(ConsumeBucketsResult,error) {
 t,err:=beginGuardTransaction(ctx,op,true)
 if err!=nil { return ConsumeBucketsResult{},err }
 defer t.close()
 receiptInput:=struct{ SubjectKey,Action string; Cost int64 }{in.SubjectKey,in.Action,in.Cost}
 var result ConsumeBucketsResult
 if found,err:=readGuardReceipt(t,"buckets.consume",receiptInput,&result);err!=nil { return ConsumeBucketsResult{},err } else if found { return result,nil }
 policy,_,err:=readGuardPolicy(t)
 if err!=nil { return ConsumeBucketsResult{},err }
 type bucket struct { key string; capacity,rate,tokens,updated int64 }
 buckets:=[]bucket{
  {key:"subject:"+in.SubjectKey,capacity:int64(policy.QaBurst),rate:int64(policy.QaPerSubjectPerMinute)},
  {key:"site",capacity:int64(policy.QaPerSitePerMinute),rate:int64(policy.QaPerSitePerMinute)},
 }
 required:=in.Cost*1000
 result.Allowed=true
 for i:=range buckets {
  current:=&buckets[i]
  var tokens,updated int64
  err=t.tx.QueryRowContext(t.ctx,`SELECT tokens_milli,updated_at_ms FROM guard_buckets
   WHERE site_id=? AND data_epoch=? AND action=? AND scope_key=?`,t.site,op.DataEpoch,in.Action,current.key).Scan(&tokens,&updated)
  if errors.Is(err,sql.ErrNoRows) { tokens=current.capacity*1000;updated=in.NowMs } else if err!=nil { return ConsumeBucketsResult{},loginDBError() }
  refilled,err:=RefillBucket(t.ctx,op,RefillBucketInput{tokens,current.capacity,in.NowMs-updated,current.rate})
  if err!=nil { return ConsumeBucketsResult{},err }
  current.tokens=refilled.TokensMilli
  current.updated=in.NowMs;if updated>current.updated { current.updated=updated }
  if current.tokens<required {
   result.Allowed=false
   retry:=((required-current.tokens)*60+current.rate-1)/current.rate
   if retry>result.RetryAfterMs { result.RetryAfterMs=retry }
  }
 }
 if result.Allowed {
  for _,current:=range buckets {
   _,err=t.tx.ExecContext(t.ctx,`INSERT INTO guard_buckets(site_id,data_epoch,action,scope_key,tokens_milli,updated_at_ms)
    VALUES(?,?,?,?,?,?) ON CONFLICT(site_id,data_epoch,action,scope_key)
    DO UPDATE SET tokens_milli=excluded.tokens_milli,updated_at_ms=excluded.updated_at_ms`,t.site,op.DataEpoch,in.Action,current.key,current.tokens-required,current.updated)
   if err!=nil { return ConsumeBucketsResult{},loginDBError() }
  }
 }
 if err=saveGuardReceipt(t,"buckets.consume",receiptInput,result);err!=nil { return ConsumeBucketsResult{},err }
 if err=t.commit();err!=nil { return ConsumeBucketsResult{},err }
 return result,nil
}
