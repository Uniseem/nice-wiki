package doc

import (
	"html"
	"nice-wiki/internal/doc/nodes"
	"strings"
)

func renderMarks(value string, marks []Mark, options RenderOptions, format nodes.Format) string {
	if format == nodes.Text {
		return value
	}
	for index := len(marks) - 1; index >= 0; index-- {
		mark := marks[index]
		if mark.Type == "link" || mark.Type == "internalLink" {
			raw, _ := mark.Attrs["href"].(string)
			if mark.Type == "internalLink" {
				if options.Links == nil {
					continue
				}
				id, _ := mark.Attrs["documentId"].(string)
				block, _ := mark.Attrs["blockId"].(string)
				resolved, ok := options.Links.ResolveDocument(id, block, options.Language)
				if !ok {
					continue
				}
				raw = resolved
			}
			safe, ok := SafeURL(raw)
			if !ok {
				continue
			}
			if format == nodes.HTML {
				value = `<a href="` + html.EscapeString(safe) + `" rel="noopener noreferrer">` + value + "</a>"
			} else {
				safe = strings.NewReplacer("(", "%28", ")", "%29", " ", "%20").Replace(safe)
				value = "[" + value + "](" + safe + ")"
			}
			continue
		}
		if format == nodes.HTML {
			tags := map[string]string{"bold": "strong", "italic": "em", "underline": "u", "strike": "s", "code": "code", "highlight": "mark"}
			if tag, ok := tags[mark.Type]; ok {
				value = "<" + tag + ">" + value + "</" + tag + ">"
			}
		} else {
			wrappers := map[string]string{"bold": "**", "italic": "*", "strike": "~~", "highlight": "=="}
			if mark.Type == "code" {
				plain := strings.NewReplacer("\\`", "`", "\\*", "*", "\\_", "_").Replace(value)
				fence := nodes.Fence(plain)
				value = fence + " " + plain + " " + fence
			} else if wrapper, ok := wrappers[mark.Type]; ok {
				value = wrapper + value + wrapper
			}
		}
	}
	return value
}
