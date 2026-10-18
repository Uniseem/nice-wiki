package interfacecatalog

import (
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/localization/internal/language"
	"strconv"
)

func NewInterfaceHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"putInterfaceString": language.JSONHandler(PutInterfaceString, func(i *PutInterfaceStringInput, r *http.Request) error {
			if i.Language != r.PathValue("language") || i.String.Key != r.PathValue("key") {
				return platform.NewError(platform.CodeValidation, false, "界面字符串路径与正文不一致")
			}
			return nil
		}),
		"readInterfaceCatalog": language.JSONHandler(ReadInterfaceCatalog, func(i *ReadInterfaceCatalogInput, r *http.Request) error {
			i.Language = r.PathValue("language")
			if r.URL.Query().Has("knownGeneration") {
				v, _ := strconv.Atoi(r.URL.Query().Get("knownGeneration"))
				i.KnownGeneration = &v
			}
			return nil
		}),
	}
}
