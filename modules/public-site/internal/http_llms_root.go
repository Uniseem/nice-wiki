package publicsite

import (
 "context"
 "net/url"
 "sort"
 "strings"
 "nice-wiki/libs/platform"
)

type HandleLLMsRootInput struct { DataEpoch string }
type HandleLLMsRootResult struct { Status int; Text string }

// HandleLLMsRoot 只列启用语言的索引入口，不混合正文。
func HandleLLMsRoot(ctx context.Context,op platform.OperationContext,in HandleLLMsRootInput)(HandleLLMsRootResult,error){
 directory,err:=directoryFrom(ctx);if err!=nil{return HandleLLMsRootResult{},err}
 site,err:=readSiteSnapshot(ctx,directory.SiteConfig);if err!=nil{return HandleLLMsRootResult{},err}
 languages,err:=readLanguageSnapshot(ctx,directory.Localization);if err!=nil{return HandleLLMsRootResult{},err}
 tags:=make([]string,0,len(languages.versions));for tag:=range languages.versions{tags=append(tags,tag)};sort.Strings(tags)
 base,err:=parsePublicBase(site.publicBaseURL);if err!=nil{return HandleLLMsRootResult{},err}
 var output strings.Builder;output.WriteString("# 文档语言索引\n\n")
 for _,tag:=range tags {prefix:=strings.TrimRight(base.String(),"/")+"/"+url.PathEscape(tag);output.WriteString("- ["+tag+"]("+prefix+"/llms.txt) · [完整正文]("+prefix+"/llms-full.txt)\n")}
 if err=confirmWatermarks(ctx,directory,site,languages);err!=nil{return HandleLLMsRootResult{},err}
 return HandleLLMsRootResult{Status:200,Text:output.String()},nil
}
