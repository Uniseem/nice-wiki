package publicsite

import (
	"context"
	"fmt"
	"nice-wiki/libs/platform"
	"strings"
)

type PublicAsset struct {
	Asset  AssetVersionRef `json:"asset"`
	URL    string          `json:"url"`
	Width  int             `json:"width"`
	Height int             `json:"height"`
	Alt    string          `json:"alt"`
	Srcset []ImageSource   `json:"srcset"`
}
type ImageSource struct {
	Width int    `json:"width"`
	URL   string `json:"url"`
}
type RenderTiptapMarkdownInput struct {
	Document TiptapDocument
	Locale   string
	Assets   []PublicAsset
	Links    map[string]string
}
type RenderTiptapMarkdownResult struct{ Markdown string }

// RenderTiptapMarkdown 不运行模型；未知资源只显示文字，不输出未经 manifest 授权的地址。
func RenderTiptapMarkdown(ctx context.Context, op platform.OperationContext, in RenderTiptapMarkdownInput) (RenderTiptapMarkdownResult, error) {
	assets := make(map[string]PublicAsset)
	for _, asset := range in.Assets {
		assets[asset.Asset.AssetID+":"+asset.Asset.SHA256] = asset
	}
	var render func(TiptapNode) (string, error)
	var children func(TiptapNode) (string, error)
	children = func(node TiptapNode) (string, error) {
		var output strings.Builder
		for _, child := range node.Content {
			text, err := render(child)
			if err != nil {
				return "", err
			}
			output.WriteString(text)
		}
		return output.String(), nil
	}
	render = func(node TiptapNode) (string, error) {
		if node.Type == "text" {
			text := markdownEscape(node.Text)
			for _, mark := range node.Marks {
				switch mark.Type {
				case "bold":
					text = "**" + text + "**"
				case "italic":
					text = "*" + text + "*"
				case "strike":
					text = "~~" + text + "~~"
				case "code":
					fence := codeFence(node.Text, "`")
					text = fence + " " + node.Text + " " + fence
				case "underline":
					text = "<u>" + text + "</u>"
				case "highlight":
					text = "==" + text + "=="
				case "link":
					href := markHref(mark, in.Links)
					if href != "" {
						text = "[" + text + "](" + markdownDestination(href) + ")"
					}
				default:
					return "", platform.NewError(platform.CodeValidation, false, "UNSUPPORTED_MARK")
				}
			}
			return text, nil
		}
		if node.Type == "codeBlock" {
			text := nodeText(node)
			fence := codeFence(text, "`")
			if len(fence) < 3 {
				fence = "```"
			}
			return fence + attrString(node.Attrs, "language") + "\n" + text + "\n" + fence + "\n\n", nil
		}
		if node.Type == "inlineMath" {
			return "$" + attrString(node.Attrs, "latex") + "$", nil
		}
		if node.Type == "mathBlock" {
			return "$$\n" + attrString(node.Attrs, "latex") + "\n$$\n\n", nil
		}
		if node.Type == "image" || node.Type == "attachment" || node.Type == "drawing" || node.Type == "video" {
			value := node.Attrs["asset"]
			label := attrString(node.Attrs, "displayName")
			description := ""
			// 图片和画板的图注（行内内容）以纯文本跟在图后面，和视频描述一样。
			if items, ok := node.Attrs["caption"].([]any); ok {
				for _, item := range items {
					if inline, ok := item.(map[string]any); ok {
						description += attrString(inline, "text")
					}
				}
			}
			switch node.Type {
			case "image":
				label = attrString(node.Attrs, "alt")
			case "drawing":
				value = node.Attrs["lightSVG"]
				label = uiText(in.Locale, nil, "画板") // 内置译文即可，不为 Markdown 再读界面字典
			case "video":
				value = node.Attrs["poster"]
				label = attrString(node.Attrs, "title")
				description = attrString(node.Attrs, "description")
			}
			asset, ok := assetFrom(value)
			if !ok {
				return markdownLabel(label) + "\n\n", nil
			}
			delivery, ok := assets[asset.AssetID+":"+asset.SHA256]
			if !ok || !safeLink(delivery.URL) {
				return markdownLabel(label) + "\n\n", nil
			}
			if label == "" {
				label = delivery.Alt
			}
			prefix := "!"
			if node.Type == "attachment" {
				prefix = ""
			}
			return prefix + "[" + markdownLabel(label) + "](" + markdownDestination(delivery.URL) + ")\n" + markdownEscape(description) + "\n\n", nil
		}
		if node.Type == "table" {
			var out strings.Builder
			for i, row := range node.Content {
				out.WriteString("|")
				for _, cell := range row.Content {
					text, err := children(cell)
					if err != nil {
						return "", err
					}
					out.WriteString(" " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "<br>") + " |")
				}
				out.WriteString("\n")
				if i == 0 {
					out.WriteString("|")
					for range row.Content {
						out.WriteString(" --- |")
					}
					out.WriteString("\n")
				}
			}
			return out.String() + "\n", nil
		}
		text, err := children(node)
		if err != nil {
			return "", err
		}
		switch node.Type {
		case "paragraph":
			return text + "\n\n", nil
		case "heading":
			return strings.Repeat("#", attrInt(node.Attrs, "level")) + " " + text + "\n\n", nil
		case "hardBreak":
			return "  \n", nil
		case "horizontalRule":
			return "---\n\n", nil
		case "blockquote":
			return "> " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n> ") + "\n\n", nil
		case "callout":
			return "> [!" + strings.ToUpper(attrString(node.Attrs, "kind")) + "]\n> " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n> ") + "\n\n", nil
		case "bulletList", "orderedList", "taskList":
			var out strings.Builder
			for i, child := range node.Content {
				value, err := children(child)
				if err != nil {
					return "", err
				}
				prefix := "- "
				if node.Type == "orderedList" {
					prefix = fmt.Sprintf("%d. ", i+attrInt(node.Attrs, "start"))
				}
				if node.Type == "taskList" {
					prefix = "- [ ] "
					if checked, _ := child.Attrs["checked"].(bool); checked {
						prefix = "- [x] "
					}
				}
				out.WriteString(prefix + strings.ReplaceAll(strings.TrimSpace(value), "\n", "\n    ") + "\n")
			}
			return out.String() + "\n", nil
		case "listItem", "taskItem", "columns", "column", "tableRow", "tableCell", "tableHeader":
			return text, nil
		default:
			return "", platform.NewError(platform.CodeValidation, false, "UNSUPPORTED_NODE")
		}
	}
	root := TiptapNode{Content: in.Document.Content}
	markdown, err := children(root)
	if err != nil {
		return RenderTiptapMarkdownResult{}, err
	}
	if len(markdown) > publishedBodyLimit {
		return RenderTiptapMarkdownResult{}, platform.NewError(platform.CodeValidation, false, "BODY_TOO_LARGE")
	}
	return RenderTiptapMarkdownResult{strings.TrimSpace(markdown) + "\n"}, nil
}

func markdownEscape(value string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "#", "\\#", "<", "&lt;", ">", "&gt;", "|", "\\|", "~", "\\~", "`", "\\`").Replace(value)
}
func markdownDestination(value string) string {
	return "<" + strings.NewReplacer("<", "%3C", ">", "%3E", "\n", "%0A", "\r", "%0D").Replace(value) + ">"
}
func codeFence(value, character string) string {
	longest, current := 0, 0
	for _, r := range value {
		if string(r) == character {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	return strings.Repeat(character, longest+1)
}
func markHref(mark TiptapMark, links map[string]string) string {
	target, _ := mark.Attrs["target"].(map[string]any)
	if target == nil {
		return ""
	}
	kind, _ := target["kind"].(string)
	if kind == "external" {
		href, _ := target["href"].(string)
		if safeLink(href) {
			return href
		}
		return ""
	}
	document, _ := target["documentId"].(string)
	block, _ := target["blockId"].(string)
	return links[document+"#"+block]
}
