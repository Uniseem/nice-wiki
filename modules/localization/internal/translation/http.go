package translation

import (
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/language"
	"strconv"
)

func NewTranslationHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"readTranslation": language.JSONHandler(ReadTranslation),
		"listTranslationTitles": language.JSONHandler(ListTranslationTitles, func(i *ListTranslationTitlesInput, r *http.Request) error {
			i.TargetLanguage = r.URL.Query().Get("targetLanguage")
			return nil
		}),
		"createTranslation": language.JSONHandler(CreateTranslation),
		"cancelTranslation": language.JSONHandler(CancelTranslation, func(i *CancelTranslationInput, r *http.Request) error {
			if i.TranslationId != r.PathValue("translationId") {
				return platform.NewError(platform.CodeValidation, false, "取消路径与正文译文不一致")
			}
			return nil
		}),
		"estimateLanguagePlan": language.JSONHandler(EstimateLanguagePlan, func(i *EstimateLanguagePlanInput, r *http.Request) error {
			if i.Language != r.PathValue("tag") {
				return platform.NewError(platform.CodeValidation, false, "语言估价路径与正文语言不一致")
			}
			return nil
		}),
		"approveLanguagePlan": language.JSONHandler(ApproveLanguagePlan, func(i *ApproveLanguagePlanInput, r *http.Request) error {
			if i.Language != r.PathValue("tag") {
				return platform.NewError(platform.CodeValidation, false, "语言批准路径与正文语言不一致")
			}
			return nil
		}),
		"listTranslationProgress": language.JSONHandler(ListTranslationProgress, func(i *ListTranslationProgressInput, r *http.Request) error {
			if r.URL.Query().Has("language") {
				tag := r.URL.Query().Get("language")
				i.Language = &tag
			}
			return nil
		}),
		"readTranslationUnits": language.JSONHandler(ReadTranslationUnits, func(i *ReadTranslationUnitsInput, r *http.Request) error {
			i.TranslationId = r.PathValue("translationId")
			i.Page.Limit = 20
			if r.URL.Query().Has("limit") {
				i.Page.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
			}
			if r.URL.Query().Has("cursor") {
				cursor := r.URL.Query().Get("cursor")
				i.Page.Cursor = &cursor
			}
			return nil
		}),
		"editTranslationUnit": language.JSONHandler(EditTranslationUnit, func(i *EditTranslationUnitInput, r *http.Request) error {
			if i.TranslationId != r.PathValue("translationId") || i.UnitId != r.PathValue("unitId") {
				return platform.NewError(platform.CodeValidation, false, "译文路径与正文对象不一致")
			}
			return nil
		}),
		"setUnitProtection": language.JSONHandler(SetUnitProtection, func(i *SetUnitProtectionInput, r *http.Request) error {
			if i.TranslationId != r.PathValue("translationId") || i.UnitId != r.PathValue("unitId") {
				return platform.NewError(platform.CodeValidation, false, "保护操作路径与正文对象不一致")
			}
			return nil
		}),
	}
}
