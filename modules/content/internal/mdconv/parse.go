// Package mdconv 在 Markdown 与正文块（Tiptap JSON）之间转换，供站点导入、导出和 Git 同步共用。
//
// 导入侧用 goldmark（MIT）解析，GitBook 的 {% hint %} 等标签先改写成标准 Markdown；HTML 一律清洗成纯文本。
// 导出侧手写序列化，输出 GitHub 风格 Markdown。块的结构以 modules/content/contracts/types.json 为准。
package mdconv

import (
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"nice-wiki/libs/platform"
)

// Options 是转换时由调用方提供的文档与资源解析。
type Options struct {
	// Link 把 Markdown 里的相对链接（已去掉锚点和查询）解析成站内文档 ID。
	Link func(target string) (documentID string, ok bool)
	// Image 把图片地址解析成已上传的资源引用（AssetVersionRef 的 JSON 形态）；解析不到的图片降级成链接。
	Image func(src string) (asset map[string]any, ok bool)
}

// Result 是一篇 Markdown 的转换结果。
type Result struct {
	Title    string
	Blocks   []any
	Warnings []string
}

const (
	maxBlocks   = 10000
	maxDepth    = 30
	maxText     = 60000
	maxListSize = 1000
)

type parser struct {
	src      []byte
	opt      Options
	maths    []string
	title    string
	titleOK  bool
	count    int
	warnings []string
	warned   map[string]bool
}

// Parse 把 Markdown 转成块。第一个出现在正文最前面的一级标题当作文档标题，不进正文。
func Parse(src []byte, opt Options) Result {
	prepared, maths, frontTitle := preprocess(string(src))
	p := &parser{src: []byte(prepared), opt: opt, maths: maths, titleOK: true, warned: map[string]bool{}}
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(p.src))
	blocks := p.blocks(doc, 1)
	title := p.title
	if title == "" {
		title = frontTitle
	}
	return Result{Title: title, Blocks: blocks, Warnings: p.warnings}
}

func (p *parser) warn(message string) {
	if !p.warned[message] && len(p.warnings) < 50 {
		p.warned[message] = true
		p.warnings = append(p.warnings, message)
	}
}

func newID() string { return platform.NewIdentifier() }

func (p *parser) blocks(parent ast.Node, depth int) []any {
	var out []any
	if depth > maxDepth {
		p.warn("嵌套层级过深，超出部分已省略")
		return out
	}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		if p.count >= maxBlocks {
			p.warn("块数超过上限，后面的内容已省略")
			break
		}
		switch node := n.(type) {
		case *ast.Heading:
			if block := p.heading(node); block != nil {
				out = append(out, block)
				p.titleOK = false
			}
		case *ast.Paragraph, *ast.TextBlock:
			out = append(out, p.paragraph(node)...)
			p.titleOK = false
		case *ast.FencedCodeBlock:
			out = append(out, p.code(node, string(node.Language(p.src)), node.Lines()))
			p.titleOK = false
		case *ast.CodeBlock:
			out = append(out, p.code(node, "", node.Lines()))
			p.titleOK = false
		case *ast.Blockquote:
			out = append(out, p.quote(node, depth))
			p.titleOK = false
		case *ast.List:
			out = append(out, p.list(node, depth))
			p.titleOK = false
		case *ast.ThematicBreak:
			p.count++
			out = append(out, map[string]any{"type": "horizontalRule", "attrs": map[string]any{"blockId": newID()}})
			p.titleOK = false
		case *ast.HTMLBlock:
			out = append(out, p.htmlBlock(node)...)
			p.titleOK = false
		case *east.Table:
			if block := p.table(node); block != nil {
				out = append(out, block)
			}
			p.titleOK = false
		}
	}
	return out
}

func (p *parser) heading(n *ast.Heading) any {
	level := n.Level
	if level == 1 && p.titleOK && p.title == "" {
		p.title = strings.TrimSpace(plainText(n, p.src))
		return nil
	}
	switch {
	case level <= 2:
		level = 2
	case level >= 4:
		level = 4
	}
	p.count++
	return map[string]any{"type": "heading", "attrs": map[string]any{"blockId": newID(), "level": level}, "content": p.inlines(n, nil)}
}

var mathToken = regexp.MustCompile(`^NWMATHBLOCK(\d+)END$`)

func (p *parser) paragraph(n ast.Node) []any {
	// 只含一张图片：做成图片块。
	if image := p.soleImage(n); image != nil {
		if block := p.imageBlock(image); block != nil {
			return []any{block}
		}
	}
	if m := mathToken.FindStringSubmatch(strings.TrimSpace(plainText(n, p.src))); m != nil {
		if index := atoi(m[1]); index < len(p.maths) {
			p.count++
			return []any{map[string]any{"type": "mathBlock", "attrs": map[string]any{"blockId": newID(), "latex": p.maths[index]}}}
		}
	}
	p.count++
	return []any{map[string]any{"type": "paragraph", "attrs": map[string]any{"blockId": newID()}, "content": p.inlines(n, nil)}}
}

func atoi(s string) int {
	value := 0
	for _, r := range s {
		value = value*10 + int(r-'0')
		if value > 1<<20 {
			return 1 << 20
		}
	}
	return value
}

func (p *parser) soleImage(n ast.Node) *ast.Image {
	var found *ast.Image
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Image:
			if found != nil {
				return nil
			}
			found = node
		case *ast.Text:
			if strings.TrimSpace(string(node.Segment.Value(p.src))) != "" {
				return nil
			}
		default:
			return nil
		}
	}
	return found
}

func (p *parser) imageBlock(image *ast.Image) any {
	if p.opt.Image == nil {
		return nil
	}
	asset, ok := p.opt.Image(string(image.Destination))
	if !ok {
		return nil
	}
	alt := strings.TrimSpace(plainText(image, p.src))
	var altValue any
	source := "auto"
	if alt != "" {
		altValue, source = clip(alt, 4000), "manual"
	}
	p.count++
	return map[string]any{"type": "image", "attrs": map[string]any{"blockId": newID(), "asset": asset, "alt": altValue, "altSource": source, "caption": []any{}, "widthPercent": nil}}
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	runes := []rune(s)
	for len(string(runes)) > max {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

var languagePattern = regexp.MustCompile(`^[a-zA-Z0-9_+.-]{1,32}$`)

func (p *parser) code(_ ast.Node, language string, lines *text.Segments) any {
	var builder strings.Builder
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		builder.Write(segment.Value(p.src))
	}
	code := strings.TrimSuffix(builder.String(), "\n")
	if len(code) > 1<<20 {
		code = clip(code, 1<<20)
		p.warn("代码块超过 1MB，已截断")
	}
	var lang any
	if fields := strings.Fields(strings.ToLower(language)); len(fields) > 0 && languagePattern.MatchString(fields[0]) {
		lang = fields[0]
	}
	content := []any{}
	if code != "" {
		content = append(content, map[string]any{"type": "text", "text": strings.ReplaceAll(code, "\x00", "")})
	}
	p.count++
	return map[string]any{"type": "codeBlock", "attrs": map[string]any{"blockId": newID(), "language": lang}, "content": content}
}

var calloutKinds = map[string]string{"NOTE": "info", "INFO": "info", "IMPORTANT": "info", "TIP": "tip", "SUCCESS": "tip", "WARNING": "warning", "CAUTION": "danger", "DANGER": "danger"}
var calloutMarker = regexp.MustCompile(`^\s*\[!([A-Za-z]+)\]\s*`)

func (p *parser) quote(n *ast.Blockquote, depth int) any {
	children := p.blocks(n, depth+1)
	kind := ""
	if len(children) > 0 {
		if first, ok := children[0].(map[string]any); ok && first["type"] == "paragraph" {
			content, _ := first["content"].([]any)
			if len(content) > 0 {
				if node, ok := content[0].(map[string]any); ok && node["type"] == "text" {
					if m := calloutMarker.FindStringSubmatch(node["text"].(string)); m != nil {
						if mapped, known := calloutKinds[strings.ToUpper(m[1])]; known {
							kind = mapped
							rest := node["text"].(string)[len(m[0]):]
							if rest == "" {
								content = content[1:]
							} else {
								node["text"] = rest
							}
							// 标记后面紧跟的换行不保留。
							for len(content) > 0 {
								if brk, ok := content[0].(map[string]any); ok && brk["type"] == "hardBreak" {
									content = content[1:]
									continue
								}
								break
							}
							if len(content) == 0 {
								children = children[1:]
							} else {
								first["content"] = content
							}
						}
					}
				}
			}
		}
	}
	if len(children) == 0 {
		children = []any{map[string]any{"type": "paragraph", "attrs": map[string]any{"blockId": newID()}, "content": []any{}}}
	}
	p.count++
	if kind != "" {
		return map[string]any{"type": "callout", "attrs": map[string]any{"blockId": newID(), "kind": kind}, "content": children}
	}
	return map[string]any{"type": "blockquote", "attrs": map[string]any{"blockId": newID()}, "content": children}
}

func (p *parser) list(l *ast.List, depth int) any {
	type item struct {
		blocks  []any
		task    bool
		checked bool
	}
	var items []item
	allTasks := true
	for c := l.FirstChild(); c != nil; c = c.NextSibling() {
		li, ok := c.(*ast.ListItem)
		if !ok {
			continue
		}
		if len(items) >= maxListSize {
			p.warn("列表项超过 1000 条，后面的已省略")
			break
		}
		checked, isTask := taskState(li)
		blocks := p.blocks(li, depth+1)
		if len(blocks) == 0 || blockType(blocks[0]) != "paragraph" {
			blocks = append([]any{map[string]any{"type": "paragraph", "attrs": map[string]any{"blockId": newID()}, "content": []any{}}}, blocks...)
		}
		items = append(items, item{blocks: blocks, task: isTask, checked: checked})
		if !isTask {
			allTasks = false
		}
	}
	if len(items) == 0 {
		return map[string]any{"type": "paragraph", "attrs": map[string]any{"blockId": newID()}, "content": []any{}}
	}
	p.count++
	content := make([]any, 0, len(items))
	for _, it := range items {
		switch {
		case allTasks:
			content = append(content, map[string]any{"type": "taskItem", "attrs": map[string]any{"itemId": newID(), "checked": it.checked}, "content": it.blocks})
		default:
			content = append(content, map[string]any{"type": "listItem", "attrs": map[string]any{"itemId": newID()}, "content": it.blocks})
		}
	}
	switch {
	case allTasks:
		return map[string]any{"type": "taskList", "attrs": map[string]any{"blockId": newID()}, "content": content}
	case l.IsOrdered():
		start := l.Start
		if start < 1 {
			start = 1
		}
		if start > 1000000 {
			start = 1000000
		}
		return map[string]any{"type": "orderedList", "attrs": map[string]any{"blockId": newID(), "start": start}, "content": content}
	default:
		return map[string]any{"type": "bulletList", "attrs": map[string]any{"blockId": newID()}, "content": content}
	}
}

func blockType(block any) string {
	if m, ok := block.(map[string]any); ok {
		if t, ok := m["type"].(string); ok {
			return t
		}
	}
	return ""
}

func taskState(li *ast.ListItem) (checked, isTask bool) {
	first := li.FirstChild()
	if first == nil {
		return false, false
	}
	if box, ok := first.FirstChild().(*east.TaskCheckBox); ok {
		return box.IsChecked, true
	}
	return false, false
}

func (p *parser) table(t *east.Table) any {
	var rows []any
	width := 0
	type cellSpec struct {
		content []any
		header  bool
	}
	var grid [][]cellSpec
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		_, isHeader := r.(*east.TableHeader)
		var cells []cellSpec
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			cells = append(cells, cellSpec{content: unescapePipes(p.inlines(c, nil)), header: isHeader})
		}
		if len(cells) > 30 {
			cells = cells[:30]
			p.warn("表格列数超过 30，多余的列已省略")
		}
		if len(cells) > width {
			width = len(cells)
		}
		grid = append(grid, cells)
	}
	if len(grid) > 200 {
		grid = grid[:200]
		p.warn("表格行数超过 200，多余的行已省略")
	}
	if width == 0 || len(grid) == 0 {
		return nil
	}
	for _, cells := range grid {
		for len(cells) < width {
			cells = append(cells, cellSpec{})
		}
		var content []any
		for _, cell := range cells {
			kind := "tableCell"
			if cell.header {
				kind = "tableHeader"
			}
			inline := cell.content
			if inline == nil {
				inline = []any{}
			}
			content = append(content, map[string]any{"type": kind, "attrs": map[string]any{"cellId": newID(), "colspan": 1, "rowspan": 1, "colwidth": nil, "backgroundColor": nil},
				"content": []any{map[string]any{"type": "paragraph", "attrs": map[string]any{"blockId": newID()}, "content": inline}}})
		}
		rows = append(rows, map[string]any{"type": "tableRow", "attrs": map[string]any{"rowId": newID()}, "content": content})
	}
	p.count++
	return map[string]any{"type": "table", "attrs": map[string]any{"blockId": newID()}, "content": rows}
}

// unescapePipes 去掉表格单元格里 goldmark 保留的 \| 转义反斜杠。
func unescapePipes(nodes []any) []any {
	for _, node := range nodes {
		if m, ok := node.(map[string]any); ok && m["type"] == "text" {
			m["text"] = strings.ReplaceAll(m["text"].(string), "\\|", "|")
		}
	}
	return nodes
}

// ---- 行内 ----

type mark struct {
	kind  string
	attrs map[string]any
}

func (p *parser) inlines(parent ast.Node, marks []mark) []any {
	var nodes []any
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.Text:
			value := string(n.Segment.Value(p.src))
			if n.SoftLineBreak() {
				value += " "
			}
			nodes = append(nodes, p.textNodes(value, marks)...)
			if n.HardLineBreak() {
				nodes = append(nodes, map[string]any{"type": "hardBreak"})
			}
		case *ast.String:
			nodes = append(nodes, p.textNodes(string(n.Value), marks)...)
		case *ast.CodeSpan:
			nodes = append(nodes, p.codeText(plainText(n, p.src), marks)...)
		case *ast.Emphasis:
			kind := "italic"
			if n.Level >= 2 {
				kind = "bold"
			}
			nodes = append(nodes, p.inlines(n, addMark(marks, mark{kind: kind}))...)
		case *east.Strikethrough:
			nodes = append(nodes, p.inlines(n, addMark(marks, mark{kind: "strike"}))...)
		case *ast.Link:
			if target := p.linkTarget(string(n.Destination)); target != nil {
				var title any
				if len(n.Title) > 0 {
					title = clip(string(n.Title), 200)
				}
				nodes = append(nodes, p.inlines(n, addMark(marks, mark{kind: "link", attrs: map[string]any{"target": target, "title": title}}))...)
			} else {
				nodes = append(nodes, p.inlines(n, marks)...)
			}
		case *ast.AutoLink:
			label := string(n.Label(p.src))
			if target := p.linkTarget(string(n.URL(p.src))); target != nil {
				nodes = append(nodes, p.textNodes(label, addMark(marks, mark{kind: "link", attrs: map[string]any{"target": target, "title": nil}}))...)
			} else {
				nodes = append(nodes, p.textNodes(label, marks)...)
			}
		case *ast.Image:
			// 行内图片没法做成块：降级成带链接的 alt 文字。
			alt := strings.TrimSpace(plainText(n, p.src))
			if alt == "" {
				alt = "图片"
			}
			if target := p.linkTarget(string(n.Destination)); target != nil {
				nodes = append(nodes, p.textNodes(alt, addMark(marks, mark{kind: "link", attrs: map[string]any{"target": target, "title": nil}}))...)
			} else {
				nodes = append(nodes, p.textNodes(alt, marks)...)
			}
			p.warn("行内图片已转成文字链接：" + clip(string(n.Destination), 120))
		case *ast.RawHTML:
			var raw strings.Builder
			for i := 0; i < n.Segments.Len(); i++ {
				segment := n.Segments.At(i)
				raw.Write(segment.Value(p.src))
			}
			tag := strings.ToLower(strings.TrimSpace(raw.String()))
			if tag == "<br>" || tag == "<br/>" || tag == "<br />" {
				nodes = append(nodes, map[string]any{"type": "hardBreak"})
			}
		case *east.TaskCheckBox:
		default:
			nodes = append(nodes, p.inlines(c, marks)...)
		}
	}
	return mergeText(nodes)
}

func addMark(marks []mark, m mark) []mark {
	for _, existing := range marks {
		if existing.kind == m.kind {
			return marks
		}
	}
	if len(marks) >= 7 {
		return marks
	}
	return append(append([]mark{}, marks...), m)
}

func marksJSON(marks []mark) []any {
	out := make([]any, 0, len(marks))
	for _, m := range marks {
		item := map[string]any{"type": m.kind}
		if m.attrs != nil {
			item["attrs"] = m.attrs
		}
		out = append(out, item)
	}
	return out
}

var inlineMath = regexp.MustCompile(`\$([^\s$](?:[^$\n]*[^\s$])?)\$`)

func (p *parser) textNodes(value string, marks []mark) []any {
	value = strings.ReplaceAll(value, "\x00", "")
	if value == "" {
		return nil
	}
	var out []any
	emit := func(s string) {
		for s != "" {
			part := s
			if len(part) > maxText {
				part = clip(part, maxText)
			}
			s = s[len(part):]
			out = append(out, map[string]any{"type": "text", "text": part, "marks": marksJSON(marks)})
		}
	}
	hasCode := false
	for _, m := range marks {
		if m.kind == "code" {
			hasCode = true
		}
	}
	if hasCode {
		emit(value)
		return out
	}
	last := 0
	for _, loc := range inlineMath.FindAllStringSubmatchIndex(value, -1) {
		emit(value[last:loc[0]])
		latex := value[loc[2]:loc[3]]
		if len(latex) <= 8192 {
			out = append(out, map[string]any{"type": "inlineMath", "attrs": map[string]any{"latex": latex}})
		} else {
			emit(value[loc[0]:loc[1]])
		}
		last = loc[1]
	}
	emit(value[last:])
	return out
}

func (p *parser) codeText(value string, marks []mark) []any {
	return p.textNodes(value, addMark(marks, mark{kind: "code"}))
}

// mergeText 合并相邻且样式相同的文字节点。
func mergeText(nodes []any) []any {
	var out []any
	for _, node := range nodes {
		current, ok := node.(map[string]any)
		if ok && current["type"] == "text" && len(out) > 0 {
			if previous, ok := out[len(out)-1].(map[string]any); ok && previous["type"] == "text" && sameMarks(previous["marks"], current["marks"]) {
				merged := previous["text"].(string) + current["text"].(string)
				if len(merged) <= 65536 {
					previous["text"] = merged
					continue
				}
			}
		}
		out = append(out, node)
	}
	if out == nil {
		out = []any{}
	}
	return out
}

func sameMarks(a, b any) bool {
	left, _ := a.([]any)
	right, _ := b.([]any)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		l, _ := left[i].(map[string]any)
		r, _ := right[i].(map[string]any)
		if l["type"] != r["type"] || l["attrs"] != nil || r["attrs"] != nil {
			return false
		}
	}
	return true
}

// linkTarget 把链接地址变成链接目标；站内相对链接按 Options.Link 解析，不认识的相对链接丢掉（只留文字）。
func (p *parser) linkTarget(dest string) map[string]any {
	dest = strings.TrimSpace(dest)
	if dest == "" || strings.HasPrefix(dest, "#") {
		return nil
	}
	lower := strings.ToLower(dest)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
		if parsed, err := url.Parse(dest); err != nil || parsed.User != nil || len(dest) > 2048 {
			return nil
		}
		return map[string]any{"kind": "external", "href": dest}
	}
	if strings.Contains(lower, ":") && !strings.HasPrefix(dest, ".") && !strings.HasPrefix(dest, "/") {
		return nil
	}
	if p.opt.Link != nil {
		clean := dest
		if i := strings.IndexAny(clean, "#?"); i >= 0 {
			clean = clean[:i]
		}
		if decoded, err := url.PathUnescape(clean); err == nil {
			clean = decoded
		}
		if id, ok := p.opt.Link(clean); ok {
			return map[string]any{"kind": "internal", "documentId": id, "blockId": nil}
		}
	}
	p.warn("没找到链接指向的文档：" + clip(dest, 120))
	return nil
}

func plainText(n ast.Node, src []byte) string {
	var builder strings.Builder
	var walk func(ast.Node)
	walk = func(node ast.Node) {
		switch v := node.(type) {
		case *ast.Text:
			builder.Write(v.Segment.Value(src))
			if v.SoftLineBreak() || v.HardLineBreak() {
				builder.WriteByte(' ')
			}
			return
		case *ast.String:
			builder.Write(v.Value)
			return
		}
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(n)
	return builder.String()
}

// ---- HTML 清洗 ----

var (
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlScript  = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	htmlBreak   = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>|</li>|</tr>|</h[1-6]>`)
	htmlTag     = regexp.MustCompile(`<[^>]+>`)
	htmlImg     = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	imgSrc      = regexp.MustCompile(`(?i)\bsrc\s*=\s*"([^"]*)"|\bsrc\s*=\s*'([^']*)'`)
	imgAlt      = regexp.MustCompile(`(?i)\balt\s*=\s*"([^"]*)"|\balt\s*=\s*'([^']*)'`)
)

func firstGroup(m []string) string {
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

func (p *parser) htmlBlock(n *ast.HTMLBlock) []any {
	var raw strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		segment := n.Lines().At(i)
		raw.Write(segment.Value(p.src))
	}
	if n.HasClosure() {
		raw.Write(n.ClosureLine.Value(p.src))
	}
	return p.htmlToBlocks(raw.String())
}

func (p *parser) htmlToBlocks(raw string) []any {
	var out []any
	raw = htmlComment.ReplaceAllString(raw, "")
	raw = htmlScript.ReplaceAllString(raw, "")
	// 图片先单独取出，其余标签去掉只留文字。
	for _, img := range htmlImg.FindAllString(raw, -1) {
		src := firstGroup(imgSrc.FindStringSubmatch(img))
		if src == "" || p.opt.Image == nil {
			continue
		}
		if asset, ok := p.opt.Image(html.UnescapeString(src)); ok {
			alt := html.UnescapeString(firstGroup(imgAlt.FindStringSubmatch(img)))
			var altValue any
			source := "auto"
			if strings.TrimSpace(alt) != "" {
				altValue, source = clip(strings.TrimSpace(alt), 4000), "manual"
			}
			p.count++
			out = append(out, map[string]any{"type": "image", "attrs": map[string]any{"blockId": newID(), "asset": asset, "alt": altValue, "altSource": source, "caption": []any{}, "widthPercent": nil}})
		}
	}
	raw = htmlImg.ReplaceAllString(raw, "")
	raw = htmlBreak.ReplaceAllString(raw, "\n\n")
	raw = htmlTag.ReplaceAllString(raw, "")
	raw = html.UnescapeString(raw)
	for _, paragraph := range regexp.MustCompile(`\n\s*\n`).Split(raw, -1) {
		line := strings.Join(strings.Fields(paragraph), " ")
		if line == "" {
			continue
		}
		p.count++
		out = append(out, map[string]any{"type": "paragraph", "attrs": map[string]any{"blockId": newID()}, "content": p.textNodes(line, nil)})
	}
	if len(out) > 0 {
		p.warn("HTML 已清洗成纯文字")
	}
	return out
}
