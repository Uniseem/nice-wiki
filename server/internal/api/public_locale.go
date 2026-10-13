package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/httpx"
	"nice-wiki/internal/i18n"
	"nice-wiki/internal/i18n/suggest"
	"strings"
	"time"
)

func (s *Server) SuggestLocale(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.LocaleSignals](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if len(body.Languages) > 10 || len(body.Timezone) > 64 || len(body.Current) > 32 {
		writeError(w, http.StatusBadRequest, "invalid_signals", "语言信号超过上限")
		return
	}
	for _, code := range body.Languages {
		if len(code) > 32 {
			writeError(w, http.StatusBadRequest, "invalid_signals", "语言信号超过上限")
			return
		}
	}
	scope := httpx.ClientIP(r.Context())
	if s.GuardScope != nil {
		scope = s.GuardScope(scope)
	}
	if s.LocaleLimiter != nil {
		if err := s.LocaleLimiter.Begin(scope, time.Now()); err != nil {
			writeError(w, http.StatusTooManyRequests, "too_many_requests", "语言建议请求过于频繁")
			return
		}
		defer s.LocaleLimiter.End(scope)
	}
	languages := s.Settings.Languages()
	country := ""
	if s.CountryLookup != nil {
		country, _ = s.CountryLookup(httpx.ClientIP(r.Context()))
	}
	language := i18n.Accept(strings.Join(body.Languages, ","), languages.Enabled, languages.Base)
	if s.Locale != nil {
		language = s.Locale.Decide(r.Context(), suggest.Signals{Languages: body.Languages, Timezone: body.Timezone, Country: country, Current: body.Current}, languages.Enabled, languages.Base)
	}
	message := map[string]string{"zh-Hans": "是否切换到简体中文？", "zh-Hant": "是否切換到繁體中文？", "en": "Switch to English?", "ja": "日本語に切り替えますか？", "ko": "한국어로 전환할까요?"}[language]
	if message == "" {
		message = "Switch to " + language + "?"
	}
	writeJSON(w, gen.LocaleSuggestion{Language: language, Message: message})
}
