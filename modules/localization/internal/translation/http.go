package translation

import (
 "net/http"
 "strconv"
 "nice-wiki/libs/platform"
 "nice-wiki/modules/localization/internal/language"
)

func NewTranslationHandlers()map[string]http.Handler{
 return map[string]http.Handler{
 "readTranslation":language.JSONHandler(ReadTranslation),
 "listTranslationProgress":language.JSONHandler(ListTranslationProgress,func(i *ListTranslationProgressInput,r *http.Request)error{if r.URL.Query().Has("language"){tag:=r.URL.Query().Get("language");i.Language=&tag};return nil}),
 "readTranslationUnits":language.JSONHandler(ReadTranslationUnits,func(i *ReadTranslationUnitsInput,r *http.Request)error{i.TranslationId=r.PathValue("translationId");i.Page.Limit=20;if r.URL.Query().Has("limit"){i.Page.Limit,_=strconv.Atoi(r.URL.Query().Get("limit"))};if r.URL.Query().Has("cursor"){cursor:=r.URL.Query().Get("cursor");i.Page.Cursor=&cursor};return nil}),
  "editTranslationUnit":language.JSONHandler(EditTranslationUnit,func(i *EditTranslationUnitInput,r *http.Request)error{if i.TranslationId!=r.PathValue("translationId")||i.UnitId!=r.PathValue("unitId"){return platform.NewError(platform.CodeValidation,false,"译文路径与正文对象不一致")};return nil}),
  "setUnitProtection":language.JSONHandler(SetUnitProtection,func(i *SetUnitProtectionInput,r *http.Request)error{if i.TranslationId!=r.PathValue("translationId")||i.UnitId!=r.PathValue("unitId"){return platform.NewError(platform.CodeValidation,false,"保护操作路径与正文对象不一致")};return nil}),
 }
}
