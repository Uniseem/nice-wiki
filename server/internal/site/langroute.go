package site

import (
	"net/http"
	"nice-wiki/internal/i18n"
	"strings"
)

func (s *Site) root(w http.ResponseWriter, r *http.Request) {
	languages := s.Settings.Languages()
	chosen := languages.Base
	if cookie, err := r.Cookie("nw_language"); err == nil {
		for _, code := range languages.Enabled {
			if cookie.Value == code {
				chosen = code
				break
			}
		}
	} else {
		if match, err := i18n.Match(r.Header.Get("Accept-Language"), languages.Enabled, languages.Base); err == nil {
			chosen = match
		}
	}
	w.Header().Set("Vary", "Cookie, Accept-Language")
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, "/"+chosen+"/", http.StatusFound)
}
func (s *Site) languagePath(path string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	for _, code := range s.Settings.Languages().Enabled {
		if parts[0] == code {
			return code, parts[1], true
		}
	}
	return "", "", false
}
