package site

import (
	"bytes"
	"net/http"
	"nice-wiki/internal/textutil"
)

func (s *Site) render(w http.ResponseWriter, r *http.Request, status int, name string, data page) {
	var output bytes.Buffer
	if err := s.templates.ExecuteTemplate(&output, name, data); err != nil {
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	etag := `"` + textutil.Hash(output.Bytes()) + `"`
	if status == 200 && data.CacheKey != "" {
		s.cache.Set(data.CacheKey, output.Bytes(), etag)
	}
	s.writeRendered(w, r, status, output.Bytes(), etag)
}
func (s *Site) writeRendered(w http.ResponseWriter, r *http.Request, status int, body []byte, etag string) {
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	if status == 200 && r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}
