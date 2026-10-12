package nodes

import (
	"html"
	"strings"
)

type drawing struct{}

func init() { Register("drawing", drawing{}) }
func (drawing) Render(n Node, c Context) (string, error) {
	light := assetURL(c, "drawings", String(n, "lightHash"), ".svg")
	dark := assetURL(c, "drawings", String(n, "darkHash"), ".svg")
	alt, caption := String(n, "alt"), String(n, "caption")
	if c.Format == Text {
		return strings.TrimSpace(alt+" "+caption) + "\n", nil
	}
	if light == "" {
		return "", nil
	}
	if c.Format == Markdown {
		return "![" + EscapeMarkdown(alt) + "](" + light + ")\n\n" + EscapeMarkdown(caption) + "\n\n", nil
	}
	image := `<img class="nw-drawing-light" src="` + html.EscapeString(light) + `" alt="` + html.EscapeString(alt) + `" loading="lazy">`
	if dark != "" {
		image += `<img class="nw-drawing-dark" src="` + html.EscapeString(dark) + `" alt="` + html.EscapeString(alt) + `" loading="lazy">`
	}
	return "<figure" + Anchor(n) + ` class="nw-drawing">` + image + "<figcaption>" + html.EscapeString(caption) + "</figcaption></figure>", nil
}
func (drawing) Translate(n Node) []string { return (image{}).Translate(n) }
