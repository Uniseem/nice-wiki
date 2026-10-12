package nodes

import "html"

type math struct{}

func init() { Register("inlineMath", math{}); Register("blockMath", math{}) }
func (math) Render(n Node, c Context) (string, error) {
	value := String(n, "latex")
	if c.Format == HTML {
		tag := "span"
		display := "false"
		if n.Type == "blockMath" {
			tag = "div"
			display = "true"
		}
		return "<" + tag + Anchor(n) + ` class="nw-math" data-display="` + display + `" data-latex="` + html.EscapeString(value) + `">` + html.EscapeString(value) + "</" + tag + ">", nil
	}
	if c.Format == Markdown {
		if n.Type == "blockMath" {
			return "$$\n" + value + "\n$$\n\n", nil
		}
		return "$" + value + "$", nil
	}
	return value, nil
}
func (math) Translate(n Node) []string { return nil }
