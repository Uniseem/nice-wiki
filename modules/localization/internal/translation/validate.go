package translation

import (
	"context"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/gen"
	"regexp"
	"strings"
	"unicode/utf8"
)

var inlineTokenPattern = regexp.MustCompile(`</?i[0-9]+>|<v[0-9]+/>`)
var markupPattern = regexp.MustCompile(`<[^>]*>`)

// CheckInlineTokens 要求编号标签完整有序；其余HTML不进入译文。
func CheckInlineTokens(source, target string) error {
	a, b := inlineTokenPattern.FindAllString(source, -1), inlineTokenPattern.FindAllString(target, -1)
	if len(a) != len(b) {
		return platform.NewError(platform.CodeIntegrityError, false, "译文编号标签数量不一致")
	}
	for i := range a {
		if a[i] != b[i] {
			return platform.NewError(platform.CodeIntegrityError, false, "译文编号标签顺序不一致")
		}
	}
	if markupPattern.MatchString(inlineTokenPattern.ReplaceAllString(target, "")) {
		return platform.NewError(platform.CodeIntegrityError, false, "译文包含未登记的HTML标签")
	}
	return nil
}

type ValidateTranslationOutputInput struct {
	SourceUnits     []gen.TranslationUnit `json:"sourceUnits"`
	TranslatedUnits []gen.TranslationUnit `json:"translatedUnits"`
	SourceLanguage  string                `json:"sourceLanguage"`
}
type ValidateTranslationOutputResult struct {
	Valid bool                  `json:"valid"`
	Units []gen.TranslationUnit `json:"units"`
}

func ValidateTranslationOutput(ctx context.Context, op OperationContext, input ValidateTranslationOutputInput) (ValidateTranslationOutputResult, error) {
	if input.SourceLanguage == "" {
		return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "译文校验缺少已固定原文语言")
	}
	if len(input.SourceUnits) != len(input.TranslatedUnits) {
		return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeIntegrityError, false, "译文块集合不完整")
	}
	byID := map[string]gen.TranslationUnit{}
	for _, u := range input.TranslatedUnits {
		if _, exists := byID[u.UnitId]; exists {
			return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeIntegrityError, false, "译文包含重复块标识")
		}
		byID[u.UnitId] = u
	}
	result := ValidateTranslationOutputResult{Valid: true, Units: []gen.TranslationUnit{}}
	for _, source := range input.SourceUnits {
		translated, ok := byID[source.UnitId]
		if !ok {
			return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeIntegrityError, false, "译文缺少本批源块")
		}
		if source.SourceText == nil {
			return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeDependencyUnavailable, false, "源块没有固定原文文本")
		}
		if source.Protected {
			return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeForbidden, false, "机翻候选不能写入人工保护单元")
		}
		if err := CheckInlineTokens(*source.SourceText, translated.TargetText); err != nil {
			return ValidateTranslationOutputResult{}, err
		}
		if !utf8.ValidString(translated.TargetText) || strings.TrimSpace(translated.TargetText) == "" {
			return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeIntegrityError, false, "译文正文为空或不是UTF8")
		}
		if utf8.RuneCountInString(*source.SourceText) >= 20 && input.SourceLanguage != source.TargetLanguage && *source.SourceText == translated.TargetText {
			return ValidateTranslationOutputResult{}, platform.NewError(platform.CodeIntegrityError, false, "跨语言长片段未发生翻译")
		}
		unit := source
		unit.TargetText = translated.TargetText
		unit.TargetHash = platform.HashBytes([]byte(unit.TargetText))
		unit.Origin = "machine"
		unit.Status = "normal"
		result.Units = append(result.Units, unit)
	}
	return result, nil
}
