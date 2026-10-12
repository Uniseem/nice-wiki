package i18n

import (
	"errors"
	"golang.org/x/text/language"
)

type Language struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"nativeName"`
	RTL        bool   `json:"rtl"`
}

var Languages = []Language{{"zh-Hans", "简体中文", "简体中文", false}, {"zh-Hant", "繁体中文", "繁體中文", false}, {"en", "英语", "English", false}, {"ja", "日语", "日本語", false}, {"ko", "韩语", "한국어", false}, {"fr", "法语", "Français", false}, {"de", "德语", "Deutsch", false}, {"es", "西班牙语", "Español", false}, {"pt-BR", "葡萄牙语", "Português", false}, {"ru", "俄语", "Русский", false}}

func Known(code string) bool {
	for _, v := range Languages {
		if v.Code == code {
			return true
		}
	}
	return false
}
func Match(accept string, enabled []string, base string) (string, error) {
	tags := []language.Tag{}
	codes := []string{base}
	for _, code := range enabled {
		if code != base {
			codes = append(codes, code)
		}
	}
	for _, code := range codes {
		if !Known(code) {
			return "", errors.New("未知启用语言")
		}
		tag, err := language.Parse(code)
		if err != nil {
			return "", err
		}
		tags = append(tags, tag)
	}
	wanted, _, err := language.ParseAcceptLanguage(accept)
	if err != nil {
		return base, nil
	}
	_, index, _ := language.NewMatcher(tags).Match(wanted...)
	return codes[index], nil
}
