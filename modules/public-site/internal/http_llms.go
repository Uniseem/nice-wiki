package publicsite

import (
 "context"
 "nice-wiki/libs/platform"
)

type HandleLLMsIndexInput struct { Locale string; DataEpoch string }
type HandleLLMsIndexResult struct { Status int; Text string }

// HandleLLMsIndex 禁索引时 404；公开摘要只能取同次权威快照。
func HandleLLMsIndex(ctx context.Context,op platform.OperationContext,in HandleLLMsIndexInput)(HandleLLMsIndexResult,error){
 enabled,err:=readIndexingPolicy(ctx);if err!=nil{return HandleLLMsIndexResult{},err};if !enabled{return HandleLLMsIndexResult{Status:404,Text:""},nil}
 index,err:=BuildLLMsIndex(ctx,op,BuildLLMsIndexInput{in.Locale,string(in.DataEpoch),publishedBodyLimit});if err!=nil{return HandleLLMsIndexResult{},err}
 if enabled,err=readIndexingPolicy(ctx);err!=nil{return HandleLLMsIndexResult{},err}else if !enabled{return HandleLLMsIndexResult{Status:404,Text:""},nil}
 return HandleLLMsIndexResult{Status:200,Text:index.Text},nil
}
