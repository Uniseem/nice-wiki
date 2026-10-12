package site

import (
	"net/http"
	"nice-wiki/internal/content"
	"nice-wiki/internal/doc"
	"strings"
)

func (s *Site) markdown(w http.ResponseWriter, r *http.Request, lang string, value content.File) {
	body, err := doc.RenderMarkdown(value.Document, doc.RenderOptions{Language: lang, Links: s})
	if err != nil {
		http.Error(w, "文档暂时不可用", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte("# " + strings.ReplaceAll(value.Meta.Title, "\n", " ") + "\n\n" + body))
	}
}
