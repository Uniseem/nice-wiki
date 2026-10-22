package publicsite

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"nice-wiki/libs/platform"
)

const attachmentIcon = `<svg class="size-4 shrink-0 text-muted" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m13.5 7.5-5.3 5.3a3.25 3.25 0 0 1-4.6-4.6l5.6-5.6a2.1 2.1 0 0 1 3 3L6.6 11.2a1 1 0 0 1-1.4-1.4L10.5 4.5"/></svg>`

type RenderTiptapHTMLInput struct {
	Document TiptapDocument
	Locale   string
	Assets   []PublicAsset
	Links    map[string]string
}
type RenderTiptapHTMLResult struct {
	SafeHTML     string
	ContainsMath bool
}

// RenderTiptapHTML 完整呈现所有已登记块；平台播放器只输出本地封面及点击入口。
func RenderTiptapHTML(ctx context.Context, op platform.OperationContext, in RenderTiptapHTMLInput) (RenderTiptapHTMLResult, error) {
	assets := make(map[string]PublicAsset)
	for _, asset := range in.Assets {
		assets[asset.Asset.AssetID+":"+asset.Asset.SHA256] = asset
	}
	containsMath := false
	// 正文里少量固定提示（目标不可用、播放入口等）按页面语言显示，和页面外壳用同一套界面文字。
	catalog := loadUIStrings(ctx, in.Locale)
	tr := func(key string) string { return uiText(in.Locale, catalog, key) }
	var render func(TiptapNode) (string, error)
	var children func(TiptapNode) (string, error)
	children = func(node TiptapNode) (string, error) {
		var output strings.Builder
		for _, child := range node.Content {
			markup, err := render(child)
			if err != nil {
				return "", err
			}
			output.WriteString(markup)
		}
		return output.String(), nil
	}
	// 图注是行内内容数组，和正文行内节点一样渲染；显示宽度与编辑器一致，作为最大宽度。
	figure := func(node TiptapNode, inner string) (string, error) {
		style := ""
		if width := attrInt(node.Attrs, "widthPercent"); width > 0 && width < 100 {
			style = fmt.Sprintf(" style=\"max-width:%d%%\"", width)
		}
		caption := ""
		if items, ok := node.Attrs["caption"].([]any); ok && len(items) > 0 {
			var inline []TiptapNode
			if raw, err := json.Marshal(items); err == nil && json.Unmarshal(raw, &inline) == nil {
				markup, err := children(TiptapNode{Content: inline})
				if err != nil {
					return "", err
				}
				if markup != "" {
					caption = "<figcaption>" + markup + "</figcaption>"
				}
			}
		}
		if node.Type == "video" && caption == "" {
			if title := attrString(node.Attrs, "title"); title != "" {
				caption = "<figcaption>" + html.EscapeString(title) + "</figcaption>"
			}
		}
		return "<figure" + style + ">" + inner + caption + "</figure>", nil
	}
	media := func(value any) (PublicAsset, bool) {
		asset, ok := assetFrom(value)
		if !ok {
			return PublicAsset{}, false
		}
		delivery, ok := assets[asset.AssetID+":"+asset.SHA256]
		return delivery, ok && safeLink(delivery.URL)
	}
	render = func(node TiptapNode) (string, error) {
		id := attrString(node.Attrs, "blockId")
		idAttribute := ""
		if id != "" {
			idAttribute = " id=\"" + html.EscapeString(id) + "\""
		}
		if node.Type == "text" {
			text := html.EscapeString(node.Text)
			for _, mark := range node.Marks {
				tag := ""
				switch mark.Type {
				case "bold":
					tag = "strong"
				case "italic":
					tag = "em"
				case "underline":
					tag = "u"
				case "strike":
					tag = "s"
				case "code":
					tag = "code"
				case "highlight":
					tag = "mark"
				case "link":
					href := markHref(mark, in.Links)
					if href != "" {
						text = "<a href=\"" + html.EscapeString(href) + "\">" + text + "</a>"
					} else {
						text = "<span class=\"link-unavailable\" title=\"" + html.EscapeString(tr("目标暂不可用")) + "\">" + text + "</span>"
					}
					continue
				default:
					return "", platform.NewError(platform.CodeValidation, false, "UNSUPPORTED_MARK")
				}
				text = "<" + tag + ">" + text + "</" + tag + ">"
			}
			return text, nil
		}
		if node.Type == "inlineMath" || node.Type == "mathBlock" {
			containsMath = true
			tag := "span"
			if node.Type == "mathBlock" {
				tag = "div"
			}
			return "<" + tag + idAttribute + " class=\"math-source\">" + html.EscapeString(attrString(node.Attrs, "latex")) + "</" + tag + ">", nil
		}
		if node.Type == "codeBlock" {
			var output strings.Builder
			language := attrString(node.Attrs, "language")
			if language == "" {
				language = "plaintext"
			}
			lexer := lexers.Get(language)
			if lexer == nil {
				lexer = lexers.Fallback
			}
			iterator, err := lexer.Tokenise(nil, nodeText(node))
			if err == nil {
				// 用类名输出，配色（浅色 github / 深色 github-dark）由页面头部按明暗主题提供。
				err = chromahtml.New(chromahtml.WithClasses(true)).Format(&output, styles.Get("github"), iterator)
			}
			if err != nil {
				return "", platform.NewError(platform.CodeInternal, false, "CODE_HIGHLIGHT_FAILED")
			}
			return "<div" + idAttribute + " class=\"code-block\" data-language=\"" + html.EscapeString(language) + "\">" + output.String() + "</div>", nil
		}
		if node.Type == "image" || node.Type == "drawing" || node.Type == "attachment" || node.Type == "video" {
			value := node.Attrs["asset"]
			label := attrString(node.Attrs, "alt")
			if node.Type == "drawing" {
				value = node.Attrs["lightSVG"]
				label = tr("画板")
			}
			if node.Type == "attachment" {
				label = attrString(node.Attrs, "displayName")
			}
			if node.Type == "video" {
				label = attrString(node.Attrs, "title")
				source, _ := node.Attrs["source"].(map[string]any)
				value = source["asset"]
			}
			delivery, ok := media(value)
			if node.Type == "video" {
				source, _ := node.Attrs["source"].(map[string]any)
				if attrString(source, "kind") == "platform" {
					poster, hasPoster := media(node.Attrs["poster"])
					picture := ""
					if hasPoster {
						picture = "<img loading=\"lazy\" src=\"" + html.EscapeString(poster.URL) + "\" alt=\"" + html.EscapeString(label) + "\">"
					}
					snapshot, _ := source["snapshot"].(map[string]any)
					href := attrString(snapshot, "canonicalURL")
					link := ""
					if safeLink(href) {
						link = "<a href=\"" + html.EscapeString(href) + "\" rel=\"noopener noreferrer\">" + html.EscapeString(tr("打开视频")) + "</a>"
					}
					embed := attrString(snapshot, "embedURL")
					if safeLink(embed) && strings.HasPrefix(embed, "https://") {
						link = "<button type=\"button\" data-embed=\"" + html.EscapeString(embed) + "\" data-title=\"" + html.EscapeString(label) + "\">" + html.EscapeString(tr("点击播放")) + "</button>" + link
					}
					return "<figure" + idAttribute + " class=\"platform-video\">" + picture + "<figcaption>" + html.EscapeString(label) + "</figcaption>" + link + "</figure>", nil
				}
			}
			if !ok {
				notice := tr("资源暂不可用")
				if label != "" {
					notice = label + " · " + notice
				}
				return "<div" + idAttribute + " class=\"media-unavailable\" style=\"width:640px;max-width:100%;min-height:180px\">" + html.EscapeString(notice) + "</div>", nil
			}
			if node.Type == "attachment" {
				return "<p><a" + idAttribute + " class=\"attachment\" href=\"" + html.EscapeString(delivery.URL) + "\" download>" + attachmentIcon + html.EscapeString(label) + "</a></p>", nil
			}
			if node.Type == "video" {
				poster, _ := media(node.Attrs["poster"])
				posterAttr, description := "", ""
				if poster.URL != "" {
					posterAttr = " poster=\"" + html.EscapeString(poster.URL) + "\""
				}
				if text := attrString(node.Attrs, "description"); text != "" {
					description = "<p>" + html.EscapeString(text) + "</p>"
				}
				video, err := figure(node, "<video"+idAttribute+" controls playsinline preload=\"metadata\" src=\""+html.EscapeString(delivery.URL)+"\""+posterAttr+"></video>")
				return video + description, err
			}
			if label == "" {
				label = delivery.Alt
			}
			var srcset strings.Builder
			for _, variant := range delivery.Srcset {
				if !safeLink(variant.URL) {
					continue
				}
				if srcset.Len() > 0 {
					srcset.WriteString(", ")
				}
				fmt.Fprintf(&srcset, "%s %dw", html.EscapeString(variant.URL), variant.Width)
			}
			dimensions := ""
			if delivery.Width > 0 && delivery.Height > 0 {
				dimensions = fmt.Sprintf(" width=\"%d\" height=\"%d\"", delivery.Width, delivery.Height)
			}
			responsive := ""
			if srcset.Len() > 0 {
				responsive = " srcset=\"" + srcset.String() + "\" sizes=\"(max-width:900px) 100vw, 760px\""
			}
			image := "<img" + idAttribute + " loading=\"lazy\" src=\"" + html.EscapeString(delivery.URL) + "\" alt=\"" + html.EscapeString(label) + "\"" + responsive + dimensions + ">"
			if node.Type == "drawing" {
				// 浅色与深色两张图都输出，按页面的明暗主题（而不是系统设置）显示其中一张。
				image = "<img" + idAttribute + " class=\"drawing drawing-light\" loading=\"lazy\" src=\"" + html.EscapeString(delivery.URL) + "\" alt=\"" + html.EscapeString(label) + "\">"
				if dark, available := media(node.Attrs["darkSVG"]); available {
					image += "<img class=\"drawing drawing-dark\" loading=\"lazy\" src=\"" + html.EscapeString(dark.URL) + "\" alt=\"" + html.EscapeString(label) + "\">"
				}
			}
			return figure(node, image)
		}
		text, err := children(node)
		if err != nil {
			return "", err
		}
		tag := ""
		attributes := idAttribute
		switch node.Type {
		case "paragraph":
			tag = "p"
		case "heading":
			tag = fmt.Sprintf("h%d", attrInt(node.Attrs, "level"))
		case "blockquote":
			tag = "blockquote"
		case "horizontalRule":
			return "<hr" + attributes + ">", nil
		case "hardBreak":
			return "<br>", nil
		case "bulletList":
			tag = "ul"
		case "orderedList":
			tag = "ol"
			attributes += fmt.Sprintf(" start=\"%d\"", attrInt(node.Attrs, "start"))
		case "taskList":
			tag = "ul"
			attributes += " class=\"task-list\""
		case "listItem":
			tag = "li"
		case "taskItem":
			tag = "li"
			checked := ""
			if value, _ := node.Attrs["checked"].(bool); value {
				checked = " checked"
			}
			// 正文包一层 div：任务项可含多段和嵌套子任务，竖排在勾选框右侧（与编辑器结构一致）。
			text = "<input type=\"checkbox\" disabled" + checked + "><div>" + text + "</div>"
		case "callout":
			tag = "aside"
			attributes += " class=\"callout " + html.EscapeString(attrString(node.Attrs, "kind")) + "\""
		case "table":
			tag = "table"
		case "tableRow":
			tag = "tr"
		case "tableCell", "tableHeader":
			tag = "td"
			if node.Type == "tableHeader" {
				tag = "th"
			}
			attributes += fmt.Sprintf(" colspan=\"%d\" rowspan=\"%d\"", attrInt(node.Attrs, "colspan"), attrInt(node.Attrs, "rowspan"))
		case "columns":
			tag = "div"
			var columns strings.Builder
			if values, ok := node.Attrs["widthRatios"].([]any); ok {
				for _, value := range values {
					if ratio, ok := value.(float64); ok {
						fmt.Fprintf(&columns, "%gfr ", ratio)
					}
				}
			}
			attributes += " class=\"columns\" style=\"grid-template-columns:" + columns.String() + "\""
		case "column":
			tag = "div"
			attributes += " class=\"column\""
		default:
			return "", platform.NewError(platform.CodeValidation, false, "UNSUPPORTED_NODE")
		}
		if tag == "table" {
			// 宽表格在窄屏上横向滚动，不撑破版面。
			return "<div class=\"table-wrap\"><table" + attributes + ">" + text + "</table></div>", nil
		}
		return "<" + tag + attributes + ">" + text + "</" + tag + ">", nil
	}
	markup, err := children(TiptapNode{Content: in.Document.Content})
	if err != nil {
		return RenderTiptapHTMLResult{}, err
	}
	if len(markup) > publishedBodyLimit {
		return RenderTiptapHTMLResult{}, platform.NewError(platform.CodeValidation, false, "BODY_TOO_LARGE")
	}
	return RenderTiptapHTMLResult{markup, containsMath}, nil
}
