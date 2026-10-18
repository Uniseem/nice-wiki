package translation

import (
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/language"
)

func NewTranslationHandlers() map[string]http.Handler {
	return map[string]http.Handler{
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
