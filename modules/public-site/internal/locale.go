package publicsite

import (
	"context"
	"strings"

	"golang.org/x/text/language"
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
	// Accept-Language 交给标准语言匹配：按权重顺序，zh-CN/zh-Hans-CN/zh→简体、zh-TW/zh-Hant-TW→繁体（站点没有繁体时退到简体）、en-GB→en。
	tags := make([]language.Tag, 0, len(in.Available))
	originals := make([]string, 0, len(in.Available))
	for _, tag := range in.Available {
		if parsed, err := language.Parse(tag); err == nil {
			tags = append(tags, parsed)
			originals = append(originals, tag)
		}
	}
	if wanted, _, err := language.ParseAcceptLanguage(in.AcceptLanguage); err == nil && len(wanted) > 0 && len(tags) > 0 {
		if _, index, confidence := language.NewMatcher(tags).Match(wanted...); confidence != language.No {
			return SelectLocaleResult{originals[index], true}, nil
		}
	}
	if selected, ok := available[strings.ToLower(in.Fallback)]; ok {
		return SelectLocaleResult{selected, true}, nil
	}
	return SelectLocaleResult{}, platform.NewError(platform.CodeValidation, false, "LOCALE_UNSUPPORTED")
}
