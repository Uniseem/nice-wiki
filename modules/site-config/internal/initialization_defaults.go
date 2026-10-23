package siteconfig

import (
	"time"

	"nice-wiki/modules/site-config/internal/gen"
)

// PrepareDefaultConfigInput 的部署地址和时钟由可信用例显式传入；
// 纯函数不读取环境、请求Host或当前时间。
type PrepareDefaultConfigInput struct {
	SiteID        gen.Identifier `json:"siteId"`
	Name          string         `json:"name"`
	PublicBaseURL gen.SafeURL    `json:"publicBaseURL"`
	BaseLanguage  string         `json:"baseLanguage,omitempty"`
	PreparedAt    time.Time      `json:"preparedAt"`
}

type PrepareDefaultConfigResult struct {
	Config gen.SiteConfig `json:"config"`
}

// PrepareDefaultConfig 生成可持久恢复的完整准备配置；返回值尚未发布，
// initialized 与修订水位只有后续权威文件提交才能推进。
func PrepareDefaultConfig(input PrepareDefaultConfigInput) (PrepareDefaultConfigResult, error) {
	language := input.BaseLanguage
	if language == "" {
		language = "zh-Hans"
	}
	normalized, err := ValidateLanguageTag(ValidateLanguageTagInput{Language: language})
	if err != nil {
		return PrepareDefaultConfigResult{}, err
	}
	var background gen.HeroBackground
	if err := background.FromHeroBackground3(gen.HeroBackground3{Kind: "theme"}); err != nil {
		return PrepareDefaultConfigResult{}, err
	}
	return PrepareDefaultConfigResult{Config: gen.SiteConfig{
		SiteId:          input.SiteID,
		Name:            input.Name,
		ThemeColor:      "#2563EB",
		PublicBaseURL:   input.PublicBaseURL,
		BaseLanguage:    normalized.Canonical,
		TitleTemplate:   "{title} | {site}",
		IndexingEnabled: true,
		Home: gen.HomeConfig{
			MainTitle:      input.Name,
			BackgroundText: "Welcome To",
			Buttons:        []gen.HeroButton{},
			Background:     background,
			TextColor:      "light",
		},
		Footer: gen.FooterConfig{
			Links:         []gen.FooterLink{},
			SourceCodeURL: "https://github.com/Uniseem/nice-wiki",
		},
		ResourceLimits: gen.ResourceLimits{
			MaxDocumentBytes:     16 << 20,
			MaxUploadBytes:       128 << 20,
			SiteStorageBytes:     10 << 30,
			MaxUploadConcurrency: 4,
		},
		SecurityPolicy: gen.SecurityPolicy{
			PreviewTTLSeconds:         900,
			SessionAbsoluteTTLSeconds: 604800,
		},
		UpdatedAt: input.PreparedAt.UTC(),
	}}, nil
}
