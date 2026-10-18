package publicsite

import (
 "context"
 "encoding/json"
 "net/url"
 "reflect"
 "sort"
 "time"
 "nice-wiki/libs/platform"
)

type SitemapEntry struct { Loc string `json:"loc"`; Lastmod time.Time `json:"lastmod"`; Alternates []AlternateLocale `json:"alternates"` }
type BuildSitemapEntriesInput struct { Locale string; DataEpoch string }
type BuildSitemapEntriesResult struct { Entries []SitemapEntry; Revision int }

// BuildSitemapEntries 的 lastmod 只取精确发布正文的公开元数据，不用缓存时间。
func BuildSitemapEntries(ctx context.Context,op platform.OperationContext,in BuildSitemapEntriesInput)(BuildSitemapEntriesResult,error){
 nav,err:=BuildNavigation(ctx,op,BuildNavigationInput{in.Locale,in.DataEpoch,10000});if err!=nil{return BuildSitemapEntriesResult{},err}
 directory,err:=directoryFrom(ctx);if err!=nil{return BuildSitemapEntriesResult{},err}
 site,err:=readSiteSnapshot(ctx,directory.SiteConfig);if err!=nil{return BuildSitemapEntriesResult{},err}
 base,err:=parsePublicBase(site.publicBaseURL);if err!=nil{return BuildSitemapEntriesResult{},err}
 entries:=make([]SitemapEntry,0,len(nav.Tree))
 for _,node:=range nav.Tree {
  query:=url.Values{"publishedRevisionId":[]string{node.Publication.PublishedRevisionId}}
  raw,err:=directory.Content.ReadPublishedRevision(ctx,node.DocumentID,query);if err!=nil{return BuildSitemapEntriesResult{},err}
  var body struct{Reference *struct{DocumentID string `json:"documentId"`; PublishedRevisionID string `json:"publishedRevisionId"`} `json:"reference"`;Metadata *struct{LastPublishedAt *time.Time `json:"lastPublishedAt"`} `json:"metadata"`}
  if err=json.Unmarshal(raw,&body);err!=nil||body.Metadata==nil||body.Metadata.LastPublishedAt==nil||body.Reference==nil{return BuildSitemapEntriesResult{},dependency("发布更新时间权威不完整")}
  if body.Reference.DocumentID!=node.DocumentID||body.Reference.PublishedRevisionID!=node.Publication.PublishedRevisionId{return BuildSitemapEntriesResult{},stale()}
  relative,err:=url.Parse(node.CanonicalPath);if err!=nil{return BuildSitemapEntriesResult{},err}
  entries=append(entries,SitemapEntry{base.ResolveReference(relative).String(),*body.Metadata.LastPublishedAt,[]AlternateLocale{}})
 }
 again,err:=BuildNavigation(ctx,op,BuildNavigationInput{in.Locale,in.DataEpoch,10000});if err!=nil{return BuildSitemapEntriesResult{},err};if again.Revision!=nav.Revision||!reflect.DeepEqual(again.Tree,nav.Tree){return BuildSitemapEntriesResult{},stale()}
 if current,err:=readSiteSnapshot(ctx,directory.SiteConfig);err!=nil{return BuildSitemapEntriesResult{},err}else if current!=site{return BuildSitemapEntriesResult{},stale()}
 sort.Slice(entries,func(i,j int)bool{return entries[i].Loc<entries[j].Loc})
 return BuildSitemapEntriesResult{entries,nav.Revision},nil
}
