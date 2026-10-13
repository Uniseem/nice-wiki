package site

import (
	"net/http"
	"nice-wiki/internal/content"
	"nice-wiki/internal/doc"
	translate "nice-wiki/internal/i18n/translate"
	"strings"
)

func (s *Site) markdown(w http.ResponseWriter, r *http.Request, lang string, value content.File) {
	if r.Method == http.MethodGet && s.RecordCrawler != nil {
		_ = s.RecordCrawler(r.Context(), r, r.URL.Path, lang)
	}
	renderDocument := value.Document
	title := value.Meta.Title
	if lang != s.Settings.Languages().Base && s.Translations != nil {
		if translated, err := s.Translations.ReadOptional(lang, value.Meta.ID); err == nil {
			renderDocument = translate.Apply(renderDocument, translated)
		}
		title, _ = s.translatedMetadata(lang, value.Meta.ID, title, "")
	}
	body, err := doc.RenderMarkdown(renderDocument, doc.RenderOptions{Language: lang, Links: s})
	if err != nil {
		http.Error(w, "文档暂时不可用", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte("# " + strings.ReplaceAll(title, "\n", " ") + "\n\n" + body))
	}
}
