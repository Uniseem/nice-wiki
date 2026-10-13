package nodes

import (
	"html"
	"strconv"
	"strings"
)

type attachment struct{}

func init() { Register("attachment", attachment{}) }
func (attachment) Render(n Node, c Context) (string, error) {
	name := String(n, "name")
	description := String(n, "description")
	url := assetURL(c, "files", String(n, "hash"), String(n, "extension"))
	if c.Format == Text {
		return strings.TrimSpace(name+" "+description) + "\n", nil
	}
	if url == "" {
		return "", nil
	}
	if c.Format == Markdown {
		return "[" + EscapeMarkdown(name) + "](" + url + ")\n\n" + EscapeMarkdown(description) + "\n\n", nil
	}
	size := Number(n, "size", 0)
	sizeText := ""
	if size > 0 {
		sizeText = strconv.Itoa(size) + " 字节"
	}
	return "<div" + Anchor(n) + ` class="nw-attachment"><a href="` + html.EscapeString(url) + `" download><span class="nw-attachment-name">` + html.EscapeString(name) + `</span><span class="nw-attachment-size">` + sizeText + "</span></a><p>" + html.EscapeString(description) + "</p></div>", nil
}
func (attachment) Translate(n Node) []string { return SlotTranslation(n) }
