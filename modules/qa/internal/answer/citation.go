package answer

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/qa/internal/gen"
	qaruntime "nice-wiki/modules/qa/internal/runtime"
	"regexp"
	"strings"
)

var citationSpelling = regexp.MustCompile(`(?i)\bcitation[.:]([0-9a-f]{8,64})\b`)
var citationMarker = regexp.MustCompile(`(?i)\[\[(citation[.:][a-z0-9_.:-]{1,256})\]\]`)

// normalizeCitationSpelling 只修正分隔符与大小写；来源仍须精确匹配本轮引用白名单，不猜测或补齐ID。
func normalizeCitationSpelling(text string) string {
	return citationSpelling.ReplaceAllStringFunc(text, func(marker string) string {
		return "citation." + strings.ToLower(marker[len("citation."):])
	})
}

// FinalizeCitations 只保留完整且精确命中来源白名单的引用；裸ID和虚构标识不会使答案进入有依据缓存。
func FinalizeCitations(text string, allowed []gen.Citation) (string, []gen.Citation) {
	byID := map[string]gen.Citation{}
	for _, citation := range allowed {
		byID[citation.CitationId] = citation
	}
	used := []gen.Citation{}
	seen := map[string]bool{}
	text = citationMarker.ReplaceAllStringFunc(normalizeCitationSpelling(text), func(marker string) string {
		id := marker[2 : len(marker)-2]
		citation, ok := byID[id]
		if !ok {
			return ""
		}
		if !seen[id] {
			seen[id] = true
			used = append(used, citation)
		}
		return marker
	})
	return text, used
}

type ValidateCitationInput struct {
	CitationID string         `json:"citationId"`
	Allowed    []gen.Citation `json:"allowed"`
	Language   string         `json:"language"`
}
type ValidateCitationResult struct {
	Citation gen.Citation `json:"citation"`
}

func ValidateCitation(ctx context.Context, op OperationContext, input ValidateCitationInput) (ValidateCitationResult, error) {
	var selected *gen.Citation
	for _, citation := range input.Allowed {
		if citation.CitationId == input.CitationID {
			if selected != nil {
				return ValidateCitationResult{}, platform.NewError(platform.CodeIntegrityError, false, "证据白名单含重复引用标识")
			}
			copy := citation
			selected = &copy
		}
	}
	if selected == nil {
		return ValidateCitationResult{}, platform.NewError(platform.CodeStaleResult, false, "引用不属于本轮证据白名单")
	}
	result, err := qaruntime.ValidateCitations(ctx, op, []gen.Citation{*selected}, input.Language)
	if err != nil {
		return ValidateCitationResult{}, err
	}
	return ValidateCitationResult{Citation: result[0]}, nil
}
