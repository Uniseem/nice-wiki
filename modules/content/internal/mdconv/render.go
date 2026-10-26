package mdconv

import (
	"fmt"
	"strings"
)

// RenderOptions 是序列化时由调用方提供的链接与资源路径解析。
type RenderOptions struct {
	// Link 把站内链接指向的文档 ID 解析成相对当前文件的路径。
	Link func(documentID string) (path string, ok bool)
	// Asset 把资源引用（AssetVersionRef 的 JSON 形态）解析成相对当前文件的路径。
	Asset func(asset map[string]any) (path string, ok bool)
}

type renderer struct{ opt RenderOptions }

// Render 把正文块序列化成 GitHub 风格 Markdown。表格的合并单元格、分栏等无法表达的结构会被展平。
func Render(blocks []any, opt RenderOptions) string {
	r := &renderer{opt: opt}
	return strings.TrimRight(r.blocks(blocks), "\n") + "\n"
}

func (r *renderer) blocks(blocks []any) string {
	var parts []string
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if text := r.block(block); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func children(block map[string]any) []any {
	list, _ := block["content"].([]any)
	return list
}

func attrs(block map[string]any) map[string]any {
	m, _ := block["attrs"].(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	return m
}

func (r *renderer) block(b map[string]any) string {
	kind, _ := b["type"].(string)
	a := attrs(b)
	switch kind {
	case "paragraph":
		return escapeBlockStart(r.inline(children(b)))
	case "heading":
		level := 2
		if v, ok := a["level"].(float64); ok {
			level = int(v)
		} else if v, ok := a["level"].(int); ok {
			level = v
		}
		return strings.Repeat("#", level) + " " + r.inline(children(b))
	case "blockquote":
		return prefixLines(r.blocks(children(b)), "> ")
	case "callout":
		marker := map[string]string{"info": "NOTE", "tip": "TIP", "warning": "WARNING", "danger": "CAUTION"}[fmt.Sprint(a["kind"])]
		if marker == "" {
			marker = "NOTE"
		}
		return prefixLines("[!"+marker+"]\n"+r.blocks(children(b)), "> ")
	case "bulletList", "orderedList", "taskList":
		return r.list(b, kind, a)
	case "codeBlock":
		var code strings.Builder
		for _, c := range children(b) {
			if node, ok := c.(map[string]any); ok {
				code.WriteString(fmt.Sprint(node["text"]))
			}
		}
		fence := "```"
		for strings.Contains(code.String(), fence) {
			fence += "`"
		}
		language := ""
		if v, ok := a["language"].(string); ok {
			language = v
		}
		return fence + language + "\n" + code.String() + "\n" + fence
	case "mathBlock":
		return "$$\n" + fmt.Sprint(a["latex"]) + "\n$$"
	case "horizontalRule":
		return "---"
	case "table":
		return r.table(b)
	case "image":
		alt, _ := a["alt"].(string)
		path := r.assetPath(a["asset"])
		if path == "" {
			return "<!-- 图片未导出 -->"
		}
		text := "![" + escapeText(alt) + "](" + escapePath(path) + ")"
		if caption := r.inline(listOf(a["caption"])); caption != "" {
			text += "\n\n*" + caption + "*"
		}
		return text
	case "attachment":
		path := r.assetPath(a["asset"])
		name := fmt.Sprint(a["displayName"])
		if path == "" {
			return escapeText(name)
		}
		return "[" + escapeText(name) + "](" + escapePath(path) + ")"
	case "video":
		title := fmt.Sprint(a["title"])
		if title == "" {
			title = "视频"
		}
		source, _ := a["source"].(map[string]any)
		if asset, ok := source["asset"]; ok {
			if path := r.assetPath(asset); path != "" {
				return "[" + escapeText(title) + "](" + escapePath(path) + ")"
			}
		}
		for _, key := range []string{"url", "href", "pageUrl"} {
			if u, ok := source[key].(string); ok && u != "" {
				return "[" + escapeText(title) + "](" + u + ")"
			}
		}
		return "<!-- 视频未导出 -->"
	case "drawing":
		for _, key := range []string{"lightSVG", "scene"} {
			if path := r.assetPath(a[key]); path != "" && key == "lightSVG" {
				return "![绘图](" + escapePath(path) + ")"
			}
		}
		return "<!-- 绘图未导出 -->"
	case "columns":
		var parts []string
		for _, c := range children(b) {
			if column, ok := c.(map[string]any); ok {
				if text := r.blocks(children(column)); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func listOf(v any) []any {
	l, _ := v.([]any)
	return l
}

func (r *renderer) assetPath(v any) string {
	asset, ok := v.(map[string]any)
	if !ok || r.opt.Asset == nil {
		return ""
	}
	path, _ := r.opt.Asset(asset)
	return path
}

func (r *renderer) list(b map[string]any, kind string, a map[string]any) string {
	var lines []string
	number := 1
	if kind == "orderedList" {
		if v, ok := a["start"].(float64); ok {
			number = int(v)
		} else if v, ok := a["start"].(int); ok {
			number = v
		}
	}
	for _, raw := range children(b) {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		marker := "- "
		switch kind {
		case "orderedList":
			marker = fmt.Sprintf("%d. ", number)
			number++
		case "taskList":
			if attrs(item)["checked"] == true {
				marker = "- [x] "
			} else {
				marker = "- [ ] "
			}
		}
		body := r.itemBlocks(children(item))
		pad := strings.Repeat(" ", len(marker))
		if kind == "taskList" {
			pad = "  "
		}
		for i, line := range strings.Split(body, "\n") {
			switch {
			case i == 0:
				lines = append(lines, marker+line)
			case line == "":
				lines = append(lines, "")
			default:
				lines = append(lines, pad+line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// itemBlocks 渲染列表项的子块：嵌套列表紧跟在上一块后面，不留空行。
func (r *renderer) itemBlocks(blocks []any) string {
	var out strings.Builder
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		text := r.block(block)
		if text == "" {
			continue
		}
		if out.Len() > 0 {
			switch block["type"] {
			case "bulletList", "orderedList", "taskList":
				out.WriteString("\n")
			default:
				out.WriteString("\n\n")
			}
		}
		out.WriteString(text)
	}
	return out.String()
}

func (r *renderer) table(b map[string]any) string {
	var rows [][]string
	for _, raw := range children(b) {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		var cells []string
		for _, c := range children(row) {
			cell, ok := c.(map[string]any)
			if !ok {
				continue
			}
			var parts []string
			for _, inner := range children(cell) {
				if block, ok := inner.(map[string]any); ok {
					if text := r.block(block); text != "" {
						parts = append(parts, strings.ReplaceAll(text, "\n", " "))
					}
				}
			}
			cells = append(cells, strings.ReplaceAll(strings.Join(parts, "<br>"), "|", "\\|"))
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		return ""
	}
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	if width == 0 {
		return ""
	}
	line := func(cells []string) string {
		for len(cells) < width {
			cells = append(cells, "")
		}
		return "| " + strings.Join(cells, " | ") + " |"
	}
	out := []string{line(rows[0]), "|" + strings.Repeat(" --- |", width)}
	for _, row := range rows[1:] {
		out = append(out, line(row))
	}
	return strings.Join(out, "\n")
}

// ---- 行内 ----

func (r *renderer) inline(nodes []any) string {
	var builder strings.Builder
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch node["type"] {
		case "text":
			builder.WriteString(r.text(fmt.Sprint(node["text"]), listOf(node["marks"])))
		case "hardBreak":
			builder.WriteString("\\\n")
		case "inlineMath":
			builder.WriteString("$" + fmt.Sprint(attrs(node)["latex"]) + "$")
		}
	}
	return builder.String()
}

func (r *renderer) text(value string, marks []any) string {
	var link map[string]any
	var wraps []string
	code := false
	for _, raw := range marks {
		mark, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch mark["type"] {
		case "bold":
			wraps = append(wraps, "**")
		case "italic":
			wraps = append(wraps, "*")
		case "strike":
			wraps = append(wraps, "~~")
		case "code":
			code = true
		case "underline":
			wraps = append(wraps, "<u>")
		case "highlight":
			wraps = append(wraps, "<mark>")
		case "link":
			link = attrs(mark)
		}
	}
	// 空白放在标记外面，否则强调符号不生效。
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	lead := value[:len(value)-len(strings.TrimLeft(value, " \t\n"))]
	trail := value[len(strings.TrimRight(value, " \t\n")):]
	body := escapeText(trimmed)
	if code {
		fence := "`"
		for strings.Contains(trimmed, fence) {
			fence += "`"
		}
		body = fence + trimmed + fence
	}
	for i := len(wraps) - 1; i >= 0; i-- {
		switch wraps[i] {
		case "<u>":
			body = "<u>" + body + "</u>"
		case "<mark>":
			body = "<mark>" + body + "</mark>"
		default:
			body = wraps[i] + body + wraps[i]
		}
	}
	if link != nil {
		if target, ok := link["target"].(map[string]any); ok {
			switch target["kind"] {
			case "external":
				body = "[" + body + "](" + escapePath(fmt.Sprint(target["href"])) + ")"
			case "internal":
				if r.opt.Link != nil {
					if path, ok := r.opt.Link(fmt.Sprint(target["documentId"])); ok {
						body = "[" + body + "](" + escapePath(path) + ")"
					}
				}
			}
		}
	}
	return lead + body + trail
}

var textEscaper = strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "`", "\\`", "[", "\\[", "]", "\\]", "<", "\\<", ">", "\\>", "$", "\\$")

func escapeText(s string) string { return textEscaper.Replace(s) }

func escapePath(s string) string {
	return strings.NewReplacer(" ", "%20", "(", "%28", ")", "%29").Replace(s)
}

// escapeBlockStart 防止段落开头被当成标题、列表或引用。
func escapeBlockStart(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '#', '+', '-', '=':
		return "\\" + s
	}
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			continue
		}
		if i > 0 && (s[i] == '.' || s[i] == ')') && i+1 < len(s) && s[i+1] == ' ' {
			return s[:i] + "\\" + s[i:]
		}
		break
	}
	return s
}

func prefixLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = strings.TrimRight(prefix, " ")
		} else {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
