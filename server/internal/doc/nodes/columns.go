package nodes

import (
	"strconv"
	"strings"
)

type columns struct{}

func init() { Register("columns", columns{}); Register("column", columns{}) }
func (columns) Render(n Node, c Context) (string, error) {
	if n.Type == "column" {
		if c.Format == HTML {
			return Wrap(n, c, "div", "nw-column")
		}
		return Children(n, c)
	}
	if c.Format != HTML {
		return Children(n, c)
	}
	widths := []string{}
	raw, _ := n.Attrs["widths"].([]any)
	for index := range n.Content {
		width := float64(1)
		if index < len(raw) {
			if value, ok := raw[index].(float64); ok && value > 0 && value <= 100 {
				width = value
			}
		}
		widths = append(widths, strconv.FormatFloat(width, 'f', -1, 64)+"fr")
	}
	body, err := Children(n, c)
	if err != nil {
		return "", err
	}
	return "<div" + Anchor(n) + ` class="nw-columns" style="grid-template-columns:` + strings.Join(widths, " ") + `">` + body + "</div>", nil
}
func (columns) Translate(n Node) []string { return nil }
