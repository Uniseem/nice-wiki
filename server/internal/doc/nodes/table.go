package nodes

import (
	"strconv"
	"strings"
)

type table struct{}

func init() {
	for _, name := range []string{"table", "tableRow", "tableCell", "tableHeader"} {
		Register(name, table{})
	}
}
func (table) Render(n Node, c Context) (string, error) {
	if c.Format == HTML {
		switch n.Type {
		case "table":
			value, err := Children(n, c)
			return "<table" + Anchor(n) + ` class="nw-table"><tbody>` + value + "</tbody></table>", err
		case "tableRow":
			return Wrap(n, c, "tr", "nw-table-row")
		default:
			tag := "td"
			if n.Type == "tableHeader" {
				tag = "th"
			}
			value, err := Children(n, c)
			if err != nil {
				return "", err
			}
			span := ""
			for _, key := range []string{"colspan", "rowspan"} {
				v := Number(n, key, 1)
				if v > 1 && v <= 100 {
					span += " " + key + `="` + strconv.Itoa(v) + `"`
				}
			}
			return "<" + tag + Anchor(n) + span + ` class="nw-table-cell">` + value + "</" + tag + ">", nil
		}
	}
	if n.Type != "table" {
		return Children(n, c)
	}
	var result strings.Builder
	for rowIndex, row := range n.Content {
		values := []string{}
		for _, cell := range row.Content {
			value, err := Children(cell, c)
			if err != nil {
				return "", err
			}
			value = strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
			value = strings.ReplaceAll(value, "|", "\\|")
			values = append(values, value)
		}
		if c.Format == Markdown {
			result.WriteString("| " + strings.Join(values, " | ") + " |\n")
			if rowIndex == 0 {
				result.WriteString("|" + strings.Repeat(" --- |", len(values)) + "\n")
			}
		} else {
			result.WriteString(strings.Join(values, "\t") + "\n")
		}
	}
	return result.String() + "\n", nil
}
func (table) Translate(n Node) []string { return nil }
