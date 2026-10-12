package site

import (
	"embed"
	"html/template"
	"net/http"
	"nice-wiki/internal/content"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/webui"
)

//go:embed templates/*.html templates/partials/*.html
var templates embed.FS

type Site struct {
	Content   *content.Repo
	Settings  *settings.Store
	Manifest  webui.Manifest
	Dev       bool
	templates *template.Template
	cache     *Cache
}

func New(repo *content.Repo, settings *settings.Store, manifest webui.Manifest, dev bool) (*Site, error) {
	parsed, err := template.ParseFS(templates, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	return &Site{Content: repo, Settings: settings, Manifest: manifest, Dev: dev, templates: parsed, cache: NewCache(128)}, nil
}
func (s *Site) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "请求方法不允许", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/robots.txt":
			s.robots(w, r)
			return
		case "/sitemap.xml":
			s.sitemap(w, r, "")
			return
		case "/llms.txt":
			s.llms(w, r, "", false)
			return
		case "/llms-full.txt":
			s.llms(w, r, "", true)
			return
		}
		if r.URL.Path == "/" {
			s.root(w, r)
			return
		}
		s.resolve(w, r)
	})
}
