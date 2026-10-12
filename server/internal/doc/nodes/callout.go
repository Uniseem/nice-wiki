package nodes

import "html"

type callout struct{}

func init() { Register("callout", callout{}) }
func (callout) Render(n Node, c Context) (string, error) {
	value, err := Children(n, c)
	if err != nil {
		return "", err
	}
	kind := String(n, "kind")
	if c.Format == HTML {
		return "<aside" + Anchor(n) + ` class="nw-callout nw-callout-` + html.EscapeString(kind) + `">` + value + "</aside>", nil
	}
	return PrefixLines(value, "> ") + "\n", nil
}
func (callout) Translate(n Node) []string { return nil }
