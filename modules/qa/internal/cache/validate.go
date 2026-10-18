package cache

import(
 "context"
 "errors"
 "time"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/qa/internal/gen"
 qaruntime "nice-wiki/modules/qa/internal/runtime"
)
type ValidateCachedAnswerInput struct{Entry gen.CacheEntry `json:"entry"`;ContextHash string `json:"contextHash"`}
type ValidateCachedAnswerResult struct{Valid bool `json:"valid"`;Entry *gen.CacheEntry `json:"entry,omitempty"`}

func ValidateCachedAnswer(ctx context.Context,op OperationContext,input ValidateCachedAnswerInput)(ValidateCachedAnswerResult,error){
 tx,err:=qaruntime.BeginRead(ctx,op);if err!=nil{return ValidateCachedAnswerResult{},err};defer tx.Close()
 policy,err:=qaruntime.ReadPolicy(tx);if err!=nil{return ValidateCachedAnswerResult{},err};if !policy.CacheEnabled{return ValidateCachedAnswerResult{},nil}
 record,err:=ReadCache(tx,op,input.Entry.CacheId);if err!=nil{return ValidateCachedAnswerResult{},err};tx.Close()
 entry:=record.Entry
 if entry.Version!=input.Entry.Version||entry.State!=gen.CacheEntryState("active")||!entry.QuestionComplete{return ValidateCachedAnswerResult{},nil}
 if !entry.Curated{
  if entry.MemoryVersionHash!=input.ContextHash||entry.ExpiresAt==nil{return ValidateCachedAnswerResult{},nil}
  if !entry.ExpiresAt.After(time.Now()){return ValidateCachedAnswerResult{},nil}
 }
 citations,err:=qaruntime.ValidateCitations(ctx,op,entry.Citations,entry.Language);if err!=nil{var failure *platform.Error;if errors.As(err,&failure)&&(failure.Code==platform.CodeNotFound||failure.Code==platform.CodeGone||failure.Code==platform.CodeStaleResult){return ValidateCachedAnswerResult{},nil};return ValidateCachedAnswerResult{},err};entry.Citations=citations
 if len(citations)==0{fingerprint,err:=qaruntime.PublicationFingerprint(ctx,op);if err!=nil{return ValidateCachedAnswerResult{},err};if record.PublicationHash==""||record.PublicationHash!=fingerprint{return ValidateCachedAnswerResult{},nil}}
 confirm,err:=qaruntime.BeginRead(ctx,op);if err!=nil{return ValidateCachedAnswerResult{},err};defer confirm.Close()
 current,err:=ReadCache(confirm,op,entry.CacheId);if err!=nil{return ValidateCachedAnswerResult{},err};if current.Entry.Version!=entry.Version||current.Entry.State!=gen.CacheEntryState("active"){return ValidateCachedAnswerResult{},nil}
 return ValidateCachedAnswerResult{Valid:true,Entry:&entry},nil
}
