package publicsite

import (
 "context"
 "encoding/json"
 "net/url"
 "sort"
 "strings"
 "nice-wiki/libs/platform"
)

type HandleRobotsInput struct { DataEpoch string }
type HandleRobotsResult struct { Status int; Text string }

// readIndexingPolicy 读取站点唯一 indexingEnabled，不从派生缓存猜配置。
func readIndexingPolicy(ctx context.Context)(bool,error){directory,err:=directoryFrom(ctx);if err!=nil{return false,err};raw,err:=directory.SiteConfig.ReadSiteConfig(ctx);if err!=nil{return false,err};var result struct{Config *struct{Enabled *bool `json:"indexingEnabled"`} `json:"config"`};if json.Unmarshal(raw,&result)!=nil||result.Config==nil||result.Config.Enabled==nil{return false,dependency("索引策略权威不完整")};return *result.Config.Enabled,nil}

// HandleRobots 显式列出 AI 爬虫；禁索引时所有主体均 Disallow，不输出私有服务地址。
func HandleRobots(ctx context.Context,op platform.OperationContext,in HandleRobotsInput)(HandleRobotsResult,error){
 directory,err:=directoryFrom(ctx);if err!=nil{return HandleRobotsResult{},err}
 site,err:=readSiteSnapshot(ctx,directory.SiteConfig);if err!=nil{return HandleRobotsResult{},err}
 languages,err:=readLanguageSnapshot(ctx,directory.Localization);if err!=nil{return HandleRobotsResult{},err}
 enabled,err:=readIndexingPolicy(ctx);if err!=nil{return HandleRobotsResult{},err}
 tags:=make([]string,0,len(languages.versions));for tag:=range languages.versions{tags=append(tags,tag)};sort.Strings(tags)
 base,err:=parsePublicBase(site.publicBaseURL);if err!=nil{return HandleRobotsResult{},err};prefix:=strings.TrimRight(base.EscapedPath(),"/")
 var output strings.Builder
 for _,agent:=range []string{"*","GPTBot","ChatGPT-User","ClaudeBot","PerplexityBot"}{output.WriteString("User-agent: "+agent+"\n");if enabled{output.WriteString("Allow: "+prefix+"/\nDisallow: "+prefix+"/admin/\nDisallow: "+prefix+"/v1/\n")}else{output.WriteString("Disallow: "+prefix+"/\n")};output.WriteString("\n")}
 if enabled{output.WriteString("Sitemap: "+strings.TrimRight(site.publicBaseURL,"/")+"/sitemap.xml\n");for _,tag:=range tags{output.WriteString("Sitemap: "+strings.TrimRight(site.publicBaseURL,"/")+"/"+url.PathEscape(tag)+"/sitemap.xml\n")}}
 if err=confirmWatermarks(ctx,directory,site,languages);err!=nil{return HandleRobotsResult{},err}
 return HandleRobotsResult{Status:200,Text:output.String()},nil
}
