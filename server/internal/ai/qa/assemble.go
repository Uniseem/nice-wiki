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
func LimitUnits(units []retrieve.Unit, maximum int) []retrieve.Unit {
	result := []retrieve.Unit{}
	seen := map[string]bool{}
	size := 0
	for _, unit := range units {
		if seen[unit.ID] {
			continue
		}
		part := len("[块 "+unit.ID+"]\n") + len(unit.Text) + 2
		if size+part > maximum {
			break
		}
		seen[unit.ID] = true
		size += part
		result = append(result, unit)
	}
	return result
}
