package i18n

import "golang.org/x/text/language"

func Accept(header string, enabled []string, base string) string {
	tags := make([]language.Tag, 0, len(enabled))
	codes := make([]string, 0, len(enabled))
	for _, code := range enabled {
		tag, err := language.Parse(code)
		if err == nil {
			tags = append(tags, tag)
			codes = append(codes, code)
		}
	}
	if len(tags) == 0 {
		return base
	}
	wanted, _, err := language.ParseAcceptLanguage(header)
	if err != nil {
		return base
	}
	_, index, _ := language.NewMatcher(tags).Match(wanted...)
	return codes[index]
}
