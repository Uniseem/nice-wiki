package publicsite

import (
 "context"
 "net/url"
 "strings"

 "golang.org/x/net/html"
 "golang.org/x/net/html/atom"
 "nice-wiki/libs/platform"
)

type SanitizeHTMLInput struct { HTML string; BaseURL string }
type SanitizeHTMLResult struct { SafeHTML string }

// safeLink 只接受明确协议和单斜杠同源路径；不输出未知资源地址。
func safeLink(raw string) bool {
 if raw==""||strings.TrimSpace(raw)!=raw||strings.ContainsAny(raw,"\\\x00\r\n\t"){return false}
 if strings.HasPrefix(raw,"/"){return !strings.HasPrefix(raw,"//")}
 parsed,err:=url.Parse(raw);if err!=nil||parsed.User!=nil{return false}
 switch parsed.Scheme{case "https","http":return parsed.Host!="";case "mailto":return parsed.Opaque!=""};return false
}

// SanitizeHTML 使用 HTML 解析树过滤静态排版，危险容器连同内容删除。
func SanitizeHTML(ctx context.Context,op platform.OperationContext,in SanitizeHTMLInput)(SanitizeHTMLResult,error){
 if _,err:=parsePublicBase(in.BaseURL);err!=nil{return SanitizeHTMLResult{},err}
 root:=&html.Node{Type:html.ElementNode,Data:"div",DataAtom:atom.Div}
 nodes,err:=html.ParseFragment(strings.NewReader(in.HTML),root);if err!=nil{return SanitizeHTMLResult{},platform.NewError(platform.CodeValidation,false,"INVALID_HTML")}
 allowed:=map[string]bool{};for _,tag:=range strings.Fields("p h1 h2 h3 h4 blockquote hr ul ol li table thead tbody tr td th pre code span div strong em u s mark a br img figure figcaption video source picture details summary input button"){allowed[tag]=true}
 var clean func(*html.Node)
 clean=func(parent *html.Node){for node:=parent.FirstChild;node!=nil;{next:=node.NextSibling
 if node.Type==html.CommentNode{parent.RemoveChild(node);node=next;continue}
 if node.Type==html.ElementNode {
 if node.Namespace!=""||!allowed[node.Data]{parent.RemoveChild(node);node=next;continue}
 attrs:=node.Attr[:0];for _,attribute:=range node.Attr{
 if attribute.Namespace!=""{continue};name:=attribute.Key
 keep:=name=="id"||name=="class"||name=="title"||name=="lang"||name=="dir"||name=="aria-label"
 switch node.Data{case "a":if name=="href"{keep=safeLink(attribute.Val)}
 case "img","source","video":if name=="src"||name=="poster"{keep=safeLink(attribute.Val)};if name=="alt"||name=="width"||name=="height"||name=="loading"||name=="controls"||name=="muted"||name=="loop"||name=="playsinline"||name=="type"||name=="media"{keep=true}
 case "td","th":if name=="colspan"||name=="rowspan"{keep=true}
 case "ol":if name=="start"{keep=true}
 case "input":if name=="checked"||name=="disabled"{keep=true};if name=="type"&&attribute.Val=="checkbox"{keep=true}}
 if keep{attrs=append(attrs,attribute)}
 };node.Attr=attrs
 };clean(node);node=next}}
 for _,node:=range nodes{root.AppendChild(node)};clean(root)
 var output strings.Builder;for node:=root.FirstChild;node!=nil;node=node.NextSibling{if err=html.Render(&output,node);err!=nil{return SanitizeHTMLResult{},platform.NewError(platform.CodeInternal,false,"HTML_RENDER_FAILED")}}
 return SanitizeHTMLResult{output.String()},nil
}
