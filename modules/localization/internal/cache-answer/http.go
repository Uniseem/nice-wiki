package cacheanswer

import (
	"net/http"
	"nice-wiki/modules/localization/internal/language"
	"strconv"
)

func NewCacheTranslationHandlers() map[string]http.Handler {
	return map[string]http.Handler{"readCacheAnswerTranslation": language.JSONHandler(ReadCacheAnswerTranslation, func(i *ReadCacheAnswerTranslationInput, r *http.Request) error {
		i.TranslationId = r.PathValue("translationId")
		i.ExpectedAnswerRevision, _ = strconv.Atoi(r.URL.Query().Get("expectedAnswerRevision"))
		return nil
	})}
}
