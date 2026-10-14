package siteconfig

import (
	"strings"

	"nice-wiki/libs/platform"
)

// ValidateLanguageTagInput 是 ValidateLanguageTag 的严格输入。
// 该类型未出现在 OpenAPI 生成目录中，因为它只被内部纯函数使用；字段
// 与 contracts/types.json 的 ValidateLanguageTagInput 保持一致。
type ValidateLanguageTagInput struct {
	Language string `json:"language"`
}

// ValidateLanguageTagResult 是 ValidateLanguageTag 的规范化结果。
type ValidateLanguageTagResult struct {
	Canonical string `json:"canonical"`
}

// ValidateLanguageTag 按 BCP47 的结构校验语言标签，并规范语言、脚本、
// 区域及扩展子标签的大小写。它不访问模型或翻译供应方，也不把未知
// 子标签静默丢弃。
func ValidateLanguageTag(input ValidateLanguageTagInput) (ValidateLanguageTagResult, error) {
	parts := strings.Split(input.Language, "-")
	if len(input.Language) < 2 || len(input.Language) > 35 || len(parts) == 0 {
		return ValidateLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "语言标签格式不合法")
	}

	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if part == "" || !isLanguageSubtag(part) {
			return ValidateLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "语言标签格式不合法")
		}
		key := strings.ToLower(part)
		if _, exists := seen[key]; exists {
			return ValidateLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "语言标签含重复子标签")
		}
		seen[key] = struct{}{}
	}

	canonical, ok := canonicalLanguageParts(parts)
	if !ok || len(strings.Join(canonical, "-")) > 35 {
		return ValidateLanguageTagResult{}, platform.NewError(platform.CodeValidation, false, "语言标签格式不合法")
	}
	return ValidateLanguageTagResult{Canonical: strings.Join(canonical, "-")}, nil
}

func canonicalLanguageParts(parts []string) ([]string, bool) {
	if len(parts) == 0 || !isAlpha(parts[0], 2, 8) {
		return nil, false
	}
	canonical := []string{strings.ToLower(parts[0])}
	index := 1

	// language 后最多允许三个 extlang 子标签。
	for index < len(parts) && len(canonical) <= 3 && isAlpha(parts[index], 3, 3) {
		canonical = append(canonical, strings.ToLower(parts[index]))
		index++
	}
	if index < len(parts) && isAlpha(parts[index], 4, 4) {
		part := parts[index]
		canonical = append(canonical, strings.ToUpper(part[:1])+strings.ToLower(part[1:]))
		index++
	}
	if index < len(parts) && (isAlpha(parts[index], 2, 2) || isDigits(parts[index], 3, 3)) {
		part := parts[index]
		if isAlpha(part, 2, 2) {
			part = strings.ToUpper(part)
		}
		canonical = append(canonical, part)
		index++
	}

	for index < len(parts) && isVariant(parts[index]) {
		canonical = append(canonical, strings.ToLower(parts[index]))
		index++
	}

	for index < len(parts) && isSingleton(parts[index]) && !strings.EqualFold(parts[index], "x") {
		canonical = append(canonical, strings.ToLower(parts[index]))
		index++
		start := index
		for index < len(parts) && !isSingleton(parts[index]) {
			if !isExtensionSubtag(parts[index]) {
				return nil, false
			}
			canonical = append(canonical, strings.ToLower(parts[index]))
			index++
		}
		if start == index {
			return nil, false
		}
	}

	if index < len(parts) && strings.EqualFold(parts[index], "x") {
		canonical = append(canonical, "x")
		index++
		start := index
		for index < len(parts) {
			if !isExtensionSubtag(parts[index]) {
				return nil, false
			}
			canonical = append(canonical, strings.ToLower(parts[index]))
			index++
		}
		if start == index {
			return nil, false
		}
	}

	return canonical, index == len(parts)
}

func isLanguageSubtag(value string) bool {
	return isAlphaNumeric(value, 1, 8)
}

func isVariant(value string) bool {
	if len(value) >= 5 && len(value) <= 8 {
		return isLanguageSubtag(value)
	}
	return len(value) == 4 && value[0] >= '0' && value[0] <= '9' && isLanguageSubtag(value[1:])
}

func isExtensionSubtag(value string) bool {
	return isAlphaNumeric(value, 2, 8)
}

func isSingleton(value string) bool {
	return len(value) == 1 && isLanguageSubtag(value)
}

func isAlpha(value string, min, max int) bool {
	if len(value) < min || len(value) > max {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !((value[i] >= 'a' && value[i] <= 'z') || (value[i] >= 'A' && value[i] <= 'Z')) {
			return false
		}
	}
	return true
}

func isDigits(value string, min, max int) bool {
	if len(value) < min || len(value) > max {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func isAlphaNumeric(value string, min, max int) bool {
	if len(value) < min || len(value) > max {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !((value[i] >= 'a' && value[i] <= 'z') ||
			(value[i] >= 'A' && value[i] <= 'Z') ||
			(value[i] >= '0' && value[i] <= '9')) {
			return false
		}
	}
	return true
}
