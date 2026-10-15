package siteconfig

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/modules/site-config/internal/gen"
)

type NormalizeBaseURLInput struct {
	URL            gen.SafeURL `json:"url"`
	AllowLocalHTTP bool        `json:"allowLocalHTTP"`
}
type NormalizeBaseURLResult struct {
	Normalized gen.SafeURL `json:"normalized"`
}

type ValidateSiteConfigInput struct {
	Config         gen.SiteConfig `json:"config"`
	AllowLocalHTTP bool           `json:"allowLocalHTTP"`
}
type ValidateSiteConfigResult struct {
	Valid bool `json:"valid"`
}

// NormalizeBaseURL 将受信能力传给平台统一URL/IDNA边界，不手造URL解析机制。
func NormalizeBaseURL(input NormalizeBaseURLInput) (NormalizeBaseURLResult, error) {
	normalized, err := lifecycle.NormalizePublicURL(input.URL, input.AllowLocalHTTP)
	if err != nil {
		return NormalizeBaseURLResult{}, err
	}
	return NormalizeBaseURLResult{Normalized: normalized}, nil
}

// ValidateSiteConfig 信任入口或持久IO已经校验的结构，仅检查聚合语义。
func ValidateSiteConfig(input ValidateSiteConfigInput) (ValidateSiteConfigResult, error) {
	config := input.Config
	if _, err := NormalizeBaseURL(NormalizeBaseURLInput{URL: config.PublicBaseURL, AllowLocalHTTP: input.AllowLocalHTTP}); err != nil {
		return ValidateSiteConfigResult{}, err
	}
	language, err := ValidateLanguageTag(ValidateLanguageTagInput{Language: config.BaseLanguage})
	if err != nil {
		return ValidateSiteConfigResult{}, err
	}
	if language.Canonical != config.BaseLanguage {
		return ValidateSiteConfigResult{}, platform.NewError(platform.CodeValidation, false, "配置基准语言必须使用规范tag")
	}
	template := strings.NewReplacer("{title}", "", "{site}", "").Replace(config.TitleTemplate)
	if strings.ContainsAny(template, "{}") {
		return ValidateSiteConfigResult{}, platform.NewError(platform.CodeValidation, false, "标题模板只允许title/site占位符")
	}
	if _, _, err := validateHomeStructure(config.Home); err != nil {
		return ValidateSiteConfigResult{}, err
	}
	ids := map[string]bool{}
	for _, link := range config.Footer.Links {
		if ids[link.LinkId] {
			return ValidateSiteConfigResult{}, platform.NewError(platform.CodeValidation, false, "页脚链接ID重复")
		}
		ids[link.LinkId] = true
		if err := validateConfigLink(link.Target); err != nil {
			return ValidateSiteConfigResult{}, err
		}
	}
	if err := validateConfigWebLink(config.Footer.SourceCodeURL, false); err != nil {
		return ValidateSiteConfigResult{}, err
	}
	if config.Footer.IcpLink != nil {
		if err := validateConfigWebLink(*config.Footer.IcpLink, false); err != nil {
			return ValidateSiteConfigResult{}, err
		}
	}
	if config.Banner != nil {
		if config.Banner.SiteId != config.SiteId {
			return ValidateSiteConfigResult{}, platform.NewError(platform.CodeValidation, false, "公告不属于当前站点")
		}
		if config.Banner.StartsAt != nil && config.Banner.EndsAt != nil && config.Banner.EndsAt.Before(*config.Banner.StartsAt) {
			return ValidateSiteConfigResult{}, platform.NewError(platform.CodeValidation, false, "公告结束时间早于开始时间")
		}
	}
	return ValidateSiteConfigResult{Valid: true}, nil
}

func validateConfigLink(target gen.LinkTarget) error {
	raw, err := target.MarshalJSON()
	if err != nil {
		return platform.NewError(platform.CodeValidation, false, "配置链接无法序列化")
	}
	var link struct {
		Kind string `json:"kind"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(raw, &link); err != nil {
		return platform.NewError(platform.CodeValidation, false, "配置链接不合法")
	}
	if link.Kind == "external" {
		return validateConfigWebLink(link.URL, true)
	}
	return nil
}

func validateConfigWebLink(raw string, allowMail bool) error {
	if strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return platform.NewError(platform.CodeValidation, false, "配置链接含控制字符")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil {
		return platform.NewError(platform.CodeValidation, false, "配置链接不合法")
	}
	if parsed.Scheme == "mailto" && allowMail && parsed.Opaque != "" {
		return nil
	}
	if (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.Opaque != "" {
		return platform.NewError(platform.CodeValidation, false, "配置链接协议不受支持")
	}
	return nil
}
