package nodes

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

// Node 为共享序列化结构；基础包不能依赖文件存储或业务服务。
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}
type Format string

const (
	HTML     Format = "html"
	Text     Format = "text"
	Markdown Format = "markdown"
)

type Context struct {
	Format    Format
	Language  string
	Render    func(Node) (string, error)
	Highlight func(string, string) (string, error)
	AssetURL  func(string, string, string) string
	VideoURL  func(string, string) (string, bool)
}
type Renderer interface {
	Render(Node, Context) (string, error)
	Translate(Node) []string
}

var registry = map[string]Renderer{}

func Register(name string, renderer Renderer) {
	if _, exists := registry[name]; exists {
		panic("重复节点注册: " + name)
	}
	registry[name] = renderer
}
func Lookup(name string) (Renderer, bool) { r, ok := registry[name]; return r, ok }
func String(n Node, key string) string    { value, _ := n.Attrs[key].(string); return value }
func Number(n Node, key string, fallback int) int {
	switch value := n.Attrs[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return fallback
	}
}
func Bool(n Node, key string) bool { value, _ := n.Attrs[key].(bool); return value }
func Children(n Node, c Context) (string, error) {
	var result strings.Builder
	for _, child := range n.Content {
		value, err := c.Render(child)
		if err != nil {
			return "", err
		}
		result.WriteString(value)
	}
	return result.String(), nil
}
func Anchor(n Node) string {
	id := String(n, "id")
	if id == "" {
		return ""
	}
	return ` id="b-` + html.EscapeString(id) + `"`
}
func Wrap(n Node, c Context, tag, class string) (string, error) {
	value, err := Children(n, c)
	if err != nil {
		return "", err
	}
	if c.Format == HTML {
		return "<" + tag + Anchor(n) + ` class="` + class + `">` + value + "</" + tag + ">", nil
	}
	return value + "\n", nil
}
func InlineText(n Node) string {
	if n.Type == "text" {
		return n.Text
	}
	var result strings.Builder
	for _, child := range n.Content {
		result.WriteString(InlineText(child))
	}
	return result.String()
}

type TranslationSlot struct {
	Text string
	Set  func(string)
}

func TranslationSlots(n *Node) []TranslationSlot {
	result := []TranslationSlot{}
	add := func(text string, set func(string)) {
		if strings.TrimSpace(text) != "" {
			result = append(result, TranslationSlot{text, set})
		}
	}
	var walk func(*Node)
	walk = func(v *Node) {
		if v != n && String(*v, "id") != "" {
			return
		}
		if v.Type == "text" {
			for _, mark := range v.Marks {
				if mark.Type == "code" || mark.Type == "inlineMath" {
					return
				}
			}
			add(v.Text, func(text string) { v.Text = text })
			return
		}
		if v.Type == "inlineMath" || v.Type == "blockMath" || v.Type == "codeBlock" {
			return
		}
		for i := range v.Content {
			walk(&v.Content[i])
		}
	}
	switch n.Type {
	case "paragraph", "heading":
		walk(n)
	case "image", "drawing":
		for _, key := range []string{"alt", "caption"} {
			key := key
			add(String(*n, key), func(text string) { n.Attrs[key] = text })
		}
	case "attachment":
		add(String(*n, "description"), func(text string) { n.Attrs["description"] = text })
	case "video":
		for _, key := range []string{"title", "description"} {
			key := key
			add(String(*n, key), func(text string) { n.Attrs[key] = text })
		}
	}
	return result
}
func SlotTranslation(n Node) []string {
	slots := TranslationSlots(&n)
	if len(slots) == 0 {
		return nil
	}
	var out strings.Builder
	for i, slot := range slots {
		fmt.Fprintf(&out, "<nw-%d>%s</nw-%d>", i, html.EscapeString(slot.Text), i)
	}
	return []string{out.String()}
}
func OrdinaryTranslation(n Node) []string {
	return SlotTranslation(n)
}
func EscapeMarkdown(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "`", "\\`", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}
func PrefixLines(value, prefix string) string {
	return prefix + strings.ReplaceAll(strings.TrimSuffix(value, "\n"), "\n", "\n"+prefix) + "\n"
}
func Fence(value string) string {
	longest := 0
	run := 0
	for _, r := range value {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	if longest < 3 {
		longest = 2
	}
	return strings.Repeat("`", longest+1)
}
func Width(n Node) string {
	width := Number(n, "width", 100)
	if width < 10 || width > 100 {
		width = 100
	}
	return strconv.Itoa(width) + "%"
}
func Missing(name string) error { return fmt.Errorf("节点渲染器尚未实现: %s", name) }
