package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/i18n"
	"strings"
)

func (s *Server) SuggestLocale(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.LocaleSignals](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	languages := s.Settings.Languages()
	language := i18n.Accept(strings.Join(body.Languages, ","), languages.Enabled, languages.Base)
	writeJSON(w, gen.LocaleSuggestion{Language: language, Message: "可切换到 " + language + " 阅读"})
}
