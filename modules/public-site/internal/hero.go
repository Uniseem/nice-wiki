package publicsite

import (
 "context"
 "fmt"
 "html"
 "strings"
 "nice-wiki/libs/platform"
)

type RenderHeroInput struct { Title string; Subtitle string; BackgroundKind string; BackgroundURL string; PosterURL string; OverlayPercent int64; ButtonLabels []string; ButtonURLs []string; BackgroundText string; TextColor string }
type RenderHeroResult struct { HTML string }

// RenderHero 仅输出一屏；媒体地址必须来自已经验证的公开投影。
func RenderHero(ctx context.Context,op platform.OperationContext,in RenderHeroInput)(RenderHeroResult,error){
 if len(in.ButtonLabels)>3||len(in.ButtonLabels)!=len(in.ButtonURLs)||in.OverlayPercent>100||in.OverlayPercent<0{return RenderHeroResult{},platform.NewError(platform.CodeValidation,false,"INVALID_HERO")}
 decorative:=in.BackgroundText
 color:="light";if in.TextColor=="dark"{color="dark"}
 var out strings.Builder
 out.WriteString("<style>.hero{height:100svh;position:relative;display:grid;place-items:center;overflow:hidden}.hero-media,.hero-shade{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}.hero-content{position:relative;text-align:center}.hero.light{color:white;background:#172033}.hero.dark{color:#172033;background:#f5f7fa}.hero-welcome{opacity:.2;font-size:clamp(3rem,10vw,9rem)}.hero-poster{display:none}@media(prefers-reduced-motion:reduce){.hero-video{display:none}.hero-poster{display:block}}</style>")
 out.WriteString("<main class=\"hero "+color+"\">")
 switch in.BackgroundKind {
 case "solid","color":
 case "image","animated","animation":if !safeLink(in.BackgroundURL){return RenderHeroResult{},platform.NewError(platform.CodeValidation,false,"INVALID_BACKGROUND")};out.WriteString("<link rel=\"preload\" as=\"image\" href=\""+html.EscapeString(in.BackgroundURL)+"\"><img class=\"hero-media\" src=\""+html.EscapeString(in.BackgroundURL)+"\" alt=\"\">")
 case "video":if !safeLink(in.BackgroundURL)||!safeLink(in.PosterURL){return RenderHeroResult{},platform.NewError(platform.CodeValidation,false,"INVALID_BACKGROUND")};out.WriteString("<video class=\"hero-media hero-video\" autoplay muted loop playsinline poster=\""+html.EscapeString(in.PosterURL)+"\" src=\""+html.EscapeString(in.BackgroundURL)+"\"></video><img class=\"hero-media hero-poster\" src=\""+html.EscapeString(in.PosterURL)+"\" alt=\"\">")
 default:return RenderHeroResult{},platform.NewError(platform.CodeValidation,false,"INVALID_BACKGROUND")
 }
 fmt.Fprintf(&out,"<div class=\"hero-shade\" style=\"background:rgb(0 0 0 / %d%%)\"></div>",in.OverlayPercent)
 out.WriteString("<div class=\"hero-content\"><div class=\"hero-welcome\" aria-hidden=\"true\">"+html.EscapeString(decorative)+"</div><h1>"+html.EscapeString(in.Title)+"</h1><p>"+html.EscapeString(in.Subtitle)+"</p><nav aria-label=\"首页入口\">")
 for i,label:=range in.ButtonLabels{if !safeLink(in.ButtonURLs[i]){return RenderHeroResult{},platform.NewError(platform.CodeValidation,false,"INVALID_BUTTON_URL")};out.WriteString("<a href=\""+html.EscapeString(in.ButtonURLs[i])+"\">"+html.EscapeString(label)+"</a>")}
 out.WriteString("</nav></div></main>");return RenderHeroResult{out.String()},nil
}
