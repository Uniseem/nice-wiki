package translate

import (
	"strings"
)

func CheckTerms(text string, terms []Term) []Term {
	missing := []Term{}
	lower := strings.ToLower(text)
	for _, term := range terms {
		if !strings.Contains(lower, strings.ToLower(term.TargetText)) {
			missing = append(missing, term)
		}
	}
	return missing
}
