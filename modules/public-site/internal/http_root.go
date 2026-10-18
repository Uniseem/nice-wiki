package publicsite

import (
	"context"
	"net/url"
	"nice-wiki/libs/platform"
	"sort"
	"strings"
)

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type HandleRootRedirectInput struct {
	LanguageCookie string
	AcceptLanguage string
	DataEpoch      string
}
type HandleRootRedirectResult struct {
	Status   int
	Location string
	Headers  []Header
}

// HandleRootRedirect 只由根路由调用；显式前缀页不会进入偏好选择。
func HandleRootRedirect(ctx context.Context, op platform.OperationContext, in HandleRootRedirectInput) (HandleRootRedirectResult, error) {
	directory, err := directoryFrom(ctx)
	if err != nil {
		return HandleRootRedirectResult{}, err
	}
	site, err := readSiteSnapshot(ctx, directory.SiteConfig)
	if err != nil {
		return HandleRootRedirectResult{}, err
	}
	languages, err := readLanguageSnapshot(ctx, directory.Localization)
	if err != nil {
		return HandleRootRedirectResult{}, err
	}
	tags := make([]string, 0, len(languages.versions))
	for tag := range languages.versions {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	selected, err := SelectLocale(ctx, op, SelectLocaleInput{Available: tags, Fallback: site.baseLanguage, LanguageCookie: in.LanguageCookie, AcceptLanguage: in.AcceptLanguage})
	if err != nil {
		return HandleRootRedirectResult{}, err
	}
	base, err := parsePublicBase(site.publicBaseURL)
	if err != nil {
		return HandleRootRedirectResult{}, err
	}
	location := strings.TrimRight(base.EscapedPath(), "/") + "/" + url.PathEscape(selected.Selected) + "/"
	if err = confirmWatermarks(ctx, directory, site, languages); err != nil {
		return HandleRootRedirectResult{}, err
	}
	return HandleRootRedirectResult{302, location, []Header{{"Vary", "Accept-Language, Cookie"}, {"Cache-Control", "no-store"}}}, nil
}
