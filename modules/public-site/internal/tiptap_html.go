package publicsite

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"nice-wiki/libs/platform"
)

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
						text = "<span class=\"link-unavailable\" title=\"目标暂不可用\">" + text + "</span>"
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
			if err := quick.Highlight(&output, nodeText(node), language, "html", "github"); err != nil {
				return "", platform.NewError(platform.CodeInternal, false, "CODE_HIGHLIGHT_FAILED")
			}
			return "<div" + idAttribute + " class=\"code-block\">" + output.String() + "</div>", nil
		}
		if node.Type == "image" || node.Type == "drawing" || node.Type == "attachment" || node.Type == "video" {
			value := node.Attrs["asset"]
			label := attrString(node.Attrs, "alt")
			if node.Type == "drawing" {
				value = node.Attrs["lightSVG"]
				label = "画板"
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
						link = "<a href=\"" + html.EscapeString(href) + "\" rel=\"noopener noreferrer\">打开视频</a>"
					}
					embed := attrString(snapshot, "embedURL")
					if safeLink(embed) && strings.HasPrefix(embed, "https://") {
						link = "<button type=\"button\" data-embed=\"" + html.EscapeString(embed) + "\" data-title=\"" + html.EscapeString(label) + "\">点击播放</button>" + link
					}
					return "<figure" + idAttribute + " class=\"platform-video\">" + picture + "<figcaption>" + html.EscapeString(label) + "</figcaption>" + link + "</figure>", nil
				}
			}
			if !ok {
				return "<div" + idAttribute + " class=\"media-unavailable\" style=\"width:640px;max-width:100%;min-height:180px\">" + html.EscapeString(label) + "（资源暂不可用）</div>", nil
			}
			if node.Type == "attachment" {
				return "<a" + idAttribute + " class=\"attachment\" href=\"" + html.EscapeString(delivery.URL) + "\" download>" + html.EscapeString(label) + "</a>", nil
			}
			if node.Type == "video" {
				poster, _ := media(node.Attrs["poster"])
				return "<video" + idAttribute + " controls playsinline preload=\"metadata\" src=\"" + html.EscapeString(delivery.URL) + "\" poster=\"" + html.EscapeString(poster.URL) + "\"></video><p>" + html.EscapeString(attrString(node.Attrs, "description")) + "</p>", nil
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
			image := "<img" + idAttribute + " loading=\"lazy\" src=\"" + html.EscapeString(delivery.URL) + "\" alt=\"" + html.EscapeString(label) + "\" srcset=\"" + srcset.String() + "\"" + dimensions + ">"
			if node.Type == "drawing" {
				dark, available := media(node.Attrs["darkSVG"])
				if available {
					image = "<picture><source media=\"(prefers-color-scheme:dark)\" srcset=\"" + html.EscapeString(dark.URL) + "\">" + image + "</picture>"
				}
			}
			return "<figure>" + image + "</figure>", nil
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
			text = "<input type=\"checkbox\" disabled" + checked + ">" + text
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
