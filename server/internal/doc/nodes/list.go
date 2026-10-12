package nodes

import (
	"strconv"
	"strings"
)

type list struct{}

func init() {
	Register("bulletList", list{})
	Register("orderedList", list{})
	Register("listItem", list{})
}
func (list) Render(n Node, c Context) (string, error) {
	if c.Format == HTML {
		if n.Type == "listItem" {
			return Wrap(n, c, "li", "nw-list-item")
		}
		tag := "ul"
		attr := ""
		if n.Type == "orderedList" {
			tag = "ol"
			attr = ` start="` + strconv.Itoa(Number(n, "start", 1)) + `"`
		}
		value, err := Children(n, c)
		if err != nil {
			return "", err
		}
		return "<" + tag + Anchor(n) + attr + ` class="nw-list">` + value + "</" + tag + ">", nil
	}
	if n.Type == "listItem" {
		return Children(n, c)
	}
	var result strings.Builder
	for i, child := range n.Content {
		value, err := c.Render(child)
		if err != nil {
			return "", err
		}
		prefix := "- "
		if n.Type == "orderedList" {
			prefix = strconv.Itoa(Number(n, "start", 1)+i) + ". "
		}
		lines := strings.Split(strings.TrimSpace(value), "\n")
		for index, line := range lines {
			if index == 0 {
				result.WriteString(prefix)
			} else {
				result.WriteString(strings.Repeat(" ", len(prefix)))
			}
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}
	result.WriteByte('\n')
	return result.String(), nil
}
func (list) Translate(n Node) []string { return nil }
