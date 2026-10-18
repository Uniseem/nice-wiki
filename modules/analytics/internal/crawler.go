package analytics

import (
 "context"
 "strings"
 "nice-wiki/libs/platform"
)

// ClassifyCrawler 在内存识别固定名单，不持久化或记录 User-Agent。
func ClassifyCrawler(ctx context.Context,op OperationContext,in ClassifyCrawlerInput)(ClassifyCrawlerResult,error){
 if len(in.UserAgent)>1024 {return ClassifyCrawlerResult{},platform.NewError(platform.CodeValidation,false,"客户端标识超过长度限制")}
 ua:=strings.ToLower(in.UserAgent)
 for _,family:=range []string{"GPTBot","ChatGPT-User","ClaudeBot","PerplexityBot","Googlebot","Bingbot"}{if strings.Contains(ua,strings.ToLower(family)){return ClassifyCrawlerResult{KnownCrawler:true,CrawlerFamily:family},nil}}
 return ClassifyCrawlerResult{},nil
}
