package site

import (
	"net/http"
	"strings"
)

func (s *Site) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	result := "User-agent: *\nAllow: /\nDisallow: /admin/\nDisallow: /api/\n"
	if base := s.Settings.Site().BaseURL; base != "" {
		result += "Sitemap: " + strings.TrimSuffix(base, "/") + "/sitemap.xml\n"
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(result))
	}
}
