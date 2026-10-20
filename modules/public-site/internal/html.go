package publicsite

import (
	"context"
	"html"
	"net/url"
	"nice-wiki/libs/platform"
	"strings"
)

type ComposeHTMLInput struct {
	SafeHTML     string
	Title        string
	TOC          []TOCItem
	Navigation   []NavigationNode
	Metadata     string
	Locale       string
	DocumentID   string
	Breadcrumbs  []Breadcrumb
	Previous     []Breadcrumb
	Next         []Breadcrumb
	Announcement string
	Footer       string
	Untranslated bool
	AIEnabled    bool
	AskURL       string
	MarkdownURL  string
}
type ComposeHTMLResult struct{ HTML string }

// ComposeHTML 正文、导航及目录由 Go 完整输出，脚本只增强交互。
func ComposeHTML(ctx context.Context, op platform.OperationContext, in ComposeHTMLInput) (ComposeHTMLResult, error) {
	var output strings.Builder
	output.WriteString("<!doctype html><html lang=\"" + html.EscapeString(in.Locale) + "\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">")
	output.WriteString(in.Metadata)
	output.WriteString(`<style>` + announcementCSS + `:root{color-scheme:light dark;font:16px/1.65 system-ui}*{box-sizing:border-box}body{margin:0}a{color:inherit}header{padding:1rem 2rem;border-bottom:1px solid #8885;display:flex;gap:1rem;align-items:center}header>a:first-child{margin-right:auto}.layout{display:grid;grid-template-columns:minmax(12rem,18rem) minmax(0,1fr) minmax(10rem,16rem);max-width:1600px;margin:auto;min-height:75vh}.tree,.toc{padding:1.5rem;overflow-wrap:anywhere}.tree ul{padding-left:1rem}.current{font-weight:bold}main{min-width:0;max-width:900px;padding:2rem}main img,main video,main svg{max-width:100%;height:auto}table{border-collapse:collapse;display:block;overflow:auto}td,th{border:1px solid #8885;padding:.5rem}pre{padding:1rem;overflow:auto}.columns{display:grid;gap:1rem}.callout{border-left:4px solid #2563eb;padding:1rem}.warning{border-color:#d97706}.danger{border-color:#dc2626}.neighbors{display:flex;justify-content:space-between;gap:2rem;margin-top:3rem}footer{padding:2rem;text-align:center;border-top:1px solid #8885}.toc a{display:block}.mobile-menu{display:none}button{font:inherit}body[data-theme=dark]{color-scheme:dark}body[data-theme=light]{color-scheme:light}@media(max-width:900px){.layout{display:block}.tree,.toc{display:none}.tree[data-open],.toc[data-open]{display:block}.mobile-menu{display:inline-block}main{padding:1rem}.columns{display:block}header{padding:1rem;flex-wrap:wrap}}@media print{header,.tree,.toc,.ai-island{display:none}.layout{display:block}}</style></head><body data-doc="` + html.EscapeString(in.DocumentID) + `" data-locale="` + html.EscapeString(in.Locale) + `">`)
	output.WriteString(in.Announcement)
	output.WriteString("<header><a href=\"./\">文档</a><button class=\"mobile-menu\" type=\"button\" data-toggle=\"tree\" aria-label=\"打开文档树\">文档树</button><button class=\"mobile-menu\" type=\"button\" data-toggle=\"toc\" aria-label=\"打开本页目录\">本页目录</button>")
	if in.AIEnabled && safeLink(in.AskURL) {
		output.WriteString("<a href=\"" + html.EscapeString(in.AskURL) + "\">AI 问答</a>")
	} else {
		output.WriteString("<span title=\"尚未配置 AI 能力\">AI 未启用</span>")
	}
	output.WriteString("<span>" + html.EscapeString(in.Locale) + "</span><button type=\"button\" data-theme-toggle>亮 / 暗</button></header><div class=\"layout\"><nav class=\"tree\" aria-label=\"文档树\">")
	nodes, err := navigationOrder(in.Navigation)
	if err != nil {
		return ComposeHTMLResult{}, err
	}
	parents := make(map[string][]NavigationNode)
	for _, node := range nodes {
		parent := ""
		if node.ParentID != nil {
			parent = *node.ParentID
		}
		parents[parent] = append(parents[parent], node)
	}
	currentAncestors := map[string]bool{in.DocumentID: true}
	byID := map[string]NavigationNode{}
	for _, node := range nodes {
		byID[node.DocumentID] = node
	}
	for id := in.DocumentID; id != ""; {
		node, ok := byID[id]
		if !ok || node.ParentID == nil {
			break
		}
		id = *node.ParentID
		currentAncestors[id] = true
	}
	var tree func(string)
	tree = func(parent string) {
		if len(parents[parent]) == 0 {
			return
		}
		output.WriteString("<ul>")
		for _, node := range parents[parent] {
			output.WriteString("<li>")
			hasChildren := len(parents[node.DocumentID]) > 0
			if hasChildren {
				open := ""
				if currentAncestors[node.DocumentID] {
					open = " open"
				}
				output.WriteString("<details" + open + "><summary>")
			}
			class := ""
			if node.DocumentID == in.DocumentID {
				class = " class=\"current\" aria-current=\"page\""
			}
			output.WriteString("<a" + class + " href=\"" + html.EscapeString(node.CanonicalPath) + "\">" + html.EscapeString(node.Title) + "</a>")
			if hasChildren {
				output.WriteString("</summary>")
				tree(node.DocumentID)
				output.WriteString("</details>")
			}
			output.WriteString("</li>")
		}
		output.WriteString("</ul>")
	}
	tree("")
	output.WriteString("</nav><main><nav aria-label=\"面包屑\">")
	for i, crumb := range in.Breadcrumbs {
		if i > 0 {
			output.WriteString(" / ")
		}
		output.WriteString("<a href=\"" + html.EscapeString(crumb.URL) + "\">" + html.EscapeString(crumb.Label) + "</a>")
	}
	output.WriteString("</nav><h1>" + html.EscapeString(in.Title) + "</h1>")
	if in.Untranslated {
		output.WriteString("<aside role=\"status\">当前语言的译文尚未完成，部分内容显示旧译文或原文。</aside>")
	}
	output.WriteString(in.SafeHTML)
	output.WriteString("<nav class=\"neighbors\" aria-label=\"相邻文档\">")
	for _, group := range []struct {
		label string
		items []Breadcrumb
	}{{"上一篇", in.Previous}, {"下一篇", in.Next}} {
		for _, item := range group.items {
			output.WriteString("<a href=\"" + html.EscapeString(item.URL) + "\">" + group.label + "：" + html.EscapeString(item.Label) + "</a>")
		}
	}
	output.WriteString("</nav>")
	if safeLink(in.MarkdownURL) {
		output.WriteString("<a data-markdown href=\"" + html.EscapeString(in.MarkdownURL) + "\">Markdown 版本</a> <button type=\"button\" data-copy-markdown>复制为 Markdown</button>")
	}
	output.WriteString("</main><nav class=\"toc\" aria-label=\"本页目录\">")
	for _, item := range in.TOC {
		output.WriteString("<a href=\"#" + html.EscapeString(url.PathEscape(item.Anchor)) + "\" data-level=\"" + strings.Repeat("·", item.Level-1) + "\">" + html.EscapeString(item.Label) + "</a>")
	}
	output.WriteString("</nav></div><footer>" + in.Footer + " <a href=\"https://github.com/Uniseem/nice-wiki\">源代码</a></footer>")
	if in.AIEnabled {
		output.WriteString("<div class=\"ai-island\" data-qa-session=\"shared\" data-document-id=\"" + html.EscapeString(in.DocumentID) + "\"></div>")
	}
	if strings.Contains(in.SafeHTML, "math-source") {
		// 公式由浏览器端 KaTeX 渲染；脚本失败时保留 LaTeX 源码。
		output.WriteString(`<link rel="stylesheet" href="/_admin/katex/katex.min.css"><script src="/_admin/katex/katex.min.js"></script><script>document.querySelectorAll('.math-source').forEach(function(e){try{katex.render(e.textContent,e,{displayMode:e.tagName==='DIV',throwOnError:false})}catch(x){}})</script>`)
	}
	output.WriteString(`<script>document.querySelectorAll('[data-toggle]').forEach(function(b){b.onclick=function(){document.querySelector('.'+b.dataset.toggle).toggleAttribute('data-open')}});document.querySelector('[data-theme-toggle]').onclick=function(){document.body.dataset.theme=document.body.dataset.theme==='dark'?'light':'dark'};var copy=document.querySelector('[data-copy-markdown]');if(copy)copy.onclick=async function(){var a=document.querySelector('[data-markdown]');try{var r=await fetch(a.href,{cache:'no-store'});if(!r.ok)throw Error();await navigator.clipboard.writeText(await r.text());copy.textContent='已复制'}catch(e){copy.textContent='复制失败，请打开 Markdown 版本'}};document.querySelectorAll('[data-embed]').forEach(function(b){b.onclick=function(){var f=document.createElement('iframe');f.src=b.dataset.embed;f.title=b.dataset.title||'视频播放器';f.allowFullscreen=true;f.referrerPolicy='no-referrer';f.setAttribute('sandbox','allow-scripts allow-same-origin allow-presentation');b.replaceWith(f)}});(function(){var d=document.body.dataset.doc;if(!d||!window.fetch)return;var r='';try{r=new URL(document.referrer).hostname}catch(e){}if(/^[0-9.]+$/.test(r)||r.indexOf(':')>=0)r='';var p={beaconId:crypto.randomUUID().split('-').join(''),documentId:d,locale:document.body.dataset.locale,eventType:'page-view',status:200,elapsedMs:Math.min(60000,Math.round(performance.now())),referrerDomain:r,userAgent:navigator.userAgent.slice(0,1000)};fetch('/v1/beacons',{method:'POST',keepalive:true,headers:{'Content-Type':'application/json','Idempotency-Key':p.beaconId},body:JSON.stringify(p)}).catch(function(){})})();</script></body></html>`)
	return ComposeHTMLResult{output.String()}, nil
}
