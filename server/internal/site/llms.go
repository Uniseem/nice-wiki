package site

import (
	"net/http"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/doc/nodes"
	translate "nice-wiki/internal/i18n/translate"
	"strings"
)

func (s *Site) llms(w http.ResponseWriter, r *http.Request, lang string, full bool) {
	if lang == "" {
		lang = s.Settings.Languages().Base
	}
	if r.Method == http.MethodGet && s.RecordCrawler != nil {
		_ = s.RecordCrawler(r.Context(), r, r.URL.Path, lang)
	}
	site := s.Settings.Site()
	if lang != s.Settings.Languages().Base && s.Translations != nil {
		if translated, err := s.Translations.ReadStrings(lang, "site"); err == nil {
			if translated["site.name"] != "" {
				site.Name = translated["site.name"]
			}
			if translated["site.description"] != "" {
				site.Description = translated["site.description"]
			}
		}
	}
	var output strings.Builder
	output.WriteString("# " + nodes.EscapeMarkdown(site.Name) + "\n\n")
	if site.Description != "" {
		output.WriteString("> " + strings.ReplaceAll(nodes.EscapeMarkdown(site.Description), "\n", " ") + "\n\n")
	}
	output.WriteString("## 文档\n\n")
	for _, meta := range s.Content.List() {
		if !meta.Published {
			continue
		}
		path, err := s.Content.Path(meta.ID)
		if err != nil {
			http.Error(w, "文档索引暂时不可用", http.StatusServiceUnavailable)
			return
		}
		target := "/" + lang + "/" + path + ".md"
		if site.BaseURL != "" {
			target = canonical(site, target)
		}
		title, _ := s.translatedMetadata(lang, meta.ID, meta.Title, "")
		output.WriteString("- [" + nodes.EscapeMarkdown(title) + "](" + target + ")\n")
		if full {
			file, err := s.Content.Get(meta.ID)
			if err != nil {
				http.Error(w, "文档暂时不可用", http.StatusServiceUnavailable)
				return
			}
			document := file.Document
			if lang != s.Settings.Languages().Base && s.Translations != nil {
				if translated, readErr := s.Translations.ReadOptional(lang, meta.ID); readErr == nil {
					document = translate.Apply(document, translated)
				}
			}
			markdown, err := doc.RenderMarkdown(document, doc.RenderOptions{Language: lang, Links: s})
			if err != nil {
				http.Error(w, "文档暂时不可用", http.StatusServiceUnavailable)
				return
			}
			output.WriteString("\n## " + nodes.EscapeMarkdown(title) + "\n\n" + markdown + "\n")
		}
		if output.Len() > 32<<20 {
			http.Error(w, "完整文档超过导出上限", http.StatusRequestEntityTooLarge)
			return
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(output.String()))
	}
}
