package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func Inject(store *Store, mapping *Mapping, task, language string) (string, string, error) {
	values := []Item{}
	for _, id := range mapping.Get(task) {
		v, err := store.Get(id)
		if err != nil {
			return "", "", err
		}
		if len(v.Languages) > 0 {
			allowed := false
			for _, lang := range v.Languages {
				if lang == language {
					allowed = true
				}
			}
			if !allowed {
				continue
			}
		}
		values = append(values, v)
	}
	if err := InjectionSize(values); err != nil {
		return "", "", err
	}
	var out strings.Builder
	for _, v := range values {
		out.WriteString("## " + v.Title + "\n" + v.Description + "\n\n" + v.Content + "\n\n")
	}
	sum := sha256.Sum256([]byte(out.String()))
	return out.String(), hex.EncodeToString(sum[:]), nil
}
