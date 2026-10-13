package site

import (
	"encoding/json"
	"html/template"
	"net/http"
	"nice-wiki/internal/content"
	"nice-wiki/internal/doc"
	translate "nice-wiki/internal/i18n/translate"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/textutil"
	"time"
)

type navigation struct {
	Title  string
	URL    string
	Depth  int
	ID     string
	Active bool
}
type heroLink struct{ Label, URL, Style string }
type page struct {
	CacheKey       string
	Canonical      string
	Alternates     []alternate
	Breadcrumb     template.JS
	TOC            []tocEntry
	Markdown       string
	MarkdownURL    string
	Language       string
	Title          string
	Description    string
	Site           settings.Site
	Home           settings.Home
	Banner         settings.Banner
	Body           template.HTML
	Navigation     []navigation
	Previous       *navigation
	Next           *navigation
	HeroLinks      []heroLink
	HeroBackground string
	HeroPoster     string
	Script         string
	CSS            []string
	Dev            bool
	Error          string
	HomePage       bool
	AIEnabled      bool
	Untranslated   bool
	MultiLanguage  bool
	ShareImage     string
	UIMessages     template.JS
}

func (s *Site) page(lang, title string) page {
	value := page{Language: lang, Title: title, Site: s.Settings.Site(), Home: s.Settings.Home(), Banner: s.Settings.Banner(), Navigation: s.navigation(lang), Dev: s.Dev}
	value.AIEnabled = s.Settings.AI().Enabled
	value.MultiLanguage = len(s.Settings.Languages().Enabled) > 1
	if lang != s.Settings.Languages().Base && s.Translations != nil {
		if translated, err := s.Translations.ReadStrings(lang, "site"); err == nil {
			if text := translated["site.name"]; text != "" {
				value.Site.Name = text
			}
			if text := translated["site.description"]; text != "" {
				value.Site.Description = text
			}
			if text := translated["home.title"]; text != "" {
				value.Home.Title = text
			}
			if text := translated["home.subtitle"]; text != "" {
				value.Home.Subtitle = text
			}
			if text := translated["banner.text"]; text != "" {
				value.Banner.Text = text
			}
		}
		if messages, err := s.Translations.ReadStrings(lang, "ui"); err == nil {
			if encoded, err := json.Marshal(messages); err == nil {
				value.UIMessages = template.JS(encoded)
			}
		}
	}
	if !value.Banner.Active(time.Now().Unix()) {
		value.Banner.Enabled = false
	}
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
	renderDocument := value.Document
	untranslated := false
	if lang != s.Settings.Languages().Base && s.Translations != nil {
		if translated, err := s.Translations.ReadOptional(lang, value.Meta.ID); err == nil && len(translated.Blocks) > 0 {
			renderDocument = translate.Apply(value.Document, translated)
		} else {
			untranslated = true
		}
	}
	data := s.page(lang, value.Meta.Title)
	key, err := textutil.HashJSON([]any{lang, value.Meta, renderDocument, untranslated, data, s.Settings.Languages(), s.Content.List(), s.Manifest, s.Dev})
	if err != nil {
		s.errorPage(w, r, 503, "文档暂时不可用")
		return
	}
	w.Header().Set("Link", "<"+r.URL.Path+".md>; rel=\"alternate\"; type=\"text/markdown\"")
	if body, etag, ok := s.cache.Get(key); ok {
		s.writeRendered(w, r, 200, body, etag)
		return
	}
	body, err := doc.RenderHTML(renderDocument, doc.RenderOptions{Language: lang, Links: s, VideoURL: s.videoURL})
	if err != nil {
		s.errorPage(w, r, 503, "文档暂时不可用")
		return
	}
	data.Untranslated = untranslated
	data.Navigation, data.Previous, data.Next = s.documentNavigation(lang, value.Meta.ID)
	data.CacheKey = key
	data.Description = value.Meta.SEODescription
	if data.Site.ShareImageHash != "" {
		data.ShareImage = canonical(data.Site, "/assets/images/"+data.Site.ShareImageHash+".webp")
	}
	data.Breadcrumb = breadcrumbJSON(data.Site, r.URL.Path, value.Meta.Title)
	data.Canonical = canonical(data.Site, r.URL.Path)
	data.Alternates = alternates(data.Site, r.URL.Path, s.Settings.Languages())
	data.Body = template.HTML(body)
	data.TOC = tableOfContents(renderDocument)
	markdown, err := doc.RenderMarkdown(renderDocument, doc.RenderOptions{Language: lang, Links: s})
	if err != nil {
		s.errorPage(w, r, 503, "文档暂时不可用")
		return
	}
	data.Markdown = "# " + value.Meta.Title + "\n\n" + markdown
	data.MarkdownURL = r.URL.Path + ".md"
	w.Header().Set("Link", "<"+r.URL.Path+".md>; rel=\"alternate\"; type=\"text/markdown\"")
	s.render(w, r, 200, "doc", data)
}
