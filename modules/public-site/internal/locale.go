package publicsite

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"nice-wiki/libs/platform"
)

type SelectLocaleInput struct {
	Requested      string
	Available      []string
	Fallback       string
	LanguageCookie string
	AcceptLanguage string
}
type SelectLocaleResult struct {
	Selected         string
	RequiresRedirect bool
}

// SelectLocale 仅在根路径选择偏好；显式语言只接受启用项。
func SelectLocale(ctx context.Context, op platform.OperationContext, in SelectLocaleInput) (SelectLocaleResult, error) {
	available := make(map[string]string, len(in.Available))
	for _, tag := range in.Available {
		available[strings.ToLower(tag)] = tag
	}
	if in.Requested != "" {
		tag, ok := available[strings.ToLower(in.Requested)]
		if !ok {
			return SelectLocaleResult{}, platform.NewError(platform.CodeValidation, false, "LOCALE_UNSUPPORTED")
		}
		return SelectLocaleResult{Selected: tag}, nil
	}
	if tag, ok := available[strings.ToLower(in.LanguageCookie)]; ok {
		return SelectLocaleResult{Selected: tag, RequiresRedirect: true}, nil
	}
	type preference struct {
		tag string
		q   float64
	}
	choices := make([]preference, 0)
	for _, entry := range strings.Split(in.AcceptLanguage, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ";")
		q := 1.0
		valid := true
		for _, parameter := range parts[1:] {
			name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if name != "q" || !found {
				valid = false
				break
			}
			var err error
			q, err = strconv.ParseFloat(value, 64)
			if err != nil || q < 0 || q > 1 || q != q {
				valid = false
				break
			}
		}
		if valid && q > 0 {
			choices = append(choices, preference{strings.ToLower(parts[0]), q})
		}
	}
	sort.SliceStable(choices, func(i, j int) bool { return choices[i].q > choices[j].q })
	for _, choice := range choices {
		tag := choice.tag
		switch tag {
		case "zh-tw", "zh-hk", "zh-mo":
			tag = "zh-hant"
		case "zh-cn", "zh-sg":
			tag = "zh-hans"
		case "pt-br":
			tag = "pt"
		}
		if selected, ok := available[tag]; ok {
			return SelectLocaleResult{selected, true}, nil
		}
		if index := strings.IndexByte(tag, '-'); index > 0 {
			if selected, ok := available[tag[:index]]; ok {
				return SelectLocaleResult{selected, true}, nil
			}
		}
	}
	if selected, ok := available[strings.ToLower(in.Fallback)]; ok {
		return SelectLocaleResult{selected, true}, nil
	}
	return SelectLocaleResult{}, platform.NewError(platform.CodeValidation, false, "LOCALE_UNSUPPORTED")
}
