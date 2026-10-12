package qa

import (
	"nice-wiki/internal/ai/retrieve"
	"strings"
)

func Assemble(units []retrieve.Unit, maximum int) string {
	var out strings.Builder
	seen := map[string]bool{}
	for _, v := range units {
		if seen[v.ID] {
			continue
		}
		seen[v.ID] = true
		part := "[块 " + v.ID + "]\n" + v.Text + "\n\n"
		if out.Len()+len(part) > maximum {
			break
		}
		out.WriteString(part)
	}
	return out.String()
}
