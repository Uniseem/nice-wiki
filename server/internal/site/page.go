package site

import (
	"html/template"
	"net/http"
	"nice-wiki/internal/content"
	"nice-wiki/internal/doc"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/textutil"
)

type navigation struct {
	Title string
	URL   string
	Depth int
}
type page struct {
	CacheKey    string
	Canonical   string
	Alternates  []alternate
	Breadcrumb  template.JS
	TOC         []tocEntry
	Markdown    string
	MarkdownURL string
	Language    string
	Title       string
	Description string
	Site        settings.Site
	Home        settings.Home
	Banner      settings.Banner
	Body        template.HTML
	Navigation  []navigation
	Script      string
	CSS         []string
	Dev         bool
	Error       string
}

func (s *Site) page(lang, title string) page {
	value := page{Language: lang, Title: title, Site: s.Settings.Site(), Home: s.Settings.Home(), Banner: s.Settings.Banner(), Navigation: s.navigation(lang), Dev: s.Dev}
	if s.Dev {
		value.Script = "http://localhost:5173/src/site/main.ts"
	} else if entry, err := s.Manifest.Entry("src/site/main.ts"); err == nil {
		value.Script = "/static/" + entry.File
		for _, css := range entry.CSS {
			value.CSS = append(value.CSS, "/static/"+css)
		}
	}
	return value
}
func (s *Site) document(w http.ResponseWriter, r *http.Request, lang string, value content.File) {
	key, err := textutil.HashJSON([]any{lang, value.Meta, s.Content.List(), s.Settings.Site(), s.Settings.Home(), s.Settings.Banner(), s.Manifest, s.Dev})
	if err != nil {
		s.errorPage(w, r, 503, "文档暂时不可用")
		return
	}
	w.Header().Set("Link", "<"+r.URL.Path+".md>; rel=\"alternate\"; type=\"text/markdown\"")
	if body, etag, ok := s.cache.Get(key); ok {
		s.writeRendered(w, r, 200, body, etag)
		return
	}
	body, err := doc.RenderHTML(value.Document, doc.RenderOptions{Language: lang, Links: s})
	if err != nil {
		s.errorPage(w, r, 503, "文档暂时不可用")
		return
	}
	data := s.page(lang, value.Meta.Title)
	data.CacheKey = key
	data.Description = value.Meta.SEODescription
	data.Breadcrumb = breadcrumbJSON(data.Site, r.URL.Path, value.Meta.Title)
	data.Canonical = canonical(data.Site, r.URL.Path)
	data.Alternates = alternates(data.Site, r.URL.Path, s.Settings.Languages())
	data.Body = template.HTML(body)
	data.TOC = tableOfContents(value.Document)
	markdown, err := doc.RenderMarkdown(value.Document, doc.RenderOptions{Language: lang, Links: s})
	if err != nil {
		s.errorPage(w, r, 503, "文档暂时不可用")
		return
	}
	data.Markdown = "# " + value.Meta.Title + "\n\n" + markdown
	data.MarkdownURL = r.URL.Path + ".md"
	w.Header().Set("Link", "<"+r.URL.Path+".md>; rel=\"alternate\"; type=\"text/markdown\"")
	s.render(w, r, 200, "doc", data)
}
