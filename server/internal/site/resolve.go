package site

import (
	"net/http"
	"strings"
)

func (s *Site) resolve(w http.ResponseWriter, r *http.Request) {
	lang, path, ok := s.languagePath(r.URL.Path)
	if !ok {
		s.errorPage(w, r, 404, "页面不存在")
		return
	}
	switch path {
	case "sitemap.xml":
		s.sitemap(w, r, lang)
		return
	case "llms.txt":
		s.llms(w, r, lang, false)
		return
	case "llms-full.txt":
		s.llms(w, r, lang, true)
		return
	}
	if path == "" {
		s.home(w, r, lang)
		return
	}
	if path == "ask" {
		s.ask(w, r, lang)
		return
	}
	markdown := strings.HasSuffix(path, ".md")
	lookup := strings.TrimSuffix(path, ".md")
	value, found := s.Content.ResolvePath(lookup)
	if found {
		if markdown {
			s.markdown(w, r, lang, value)
			return
		}
		s.document(w, r, lang, value)
		return
	}
	redirect, exists, err := s.Content.Redirect(lookup)
	if err != nil {
		s.errorPage(w, r, 503, "页面暂时不可用")
		return
	}
	if exists {
		if redirect.Gone {
			s.errorPage(w, r, 410, "文档已删除")
			return
		}
		path, err := s.Content.Path(redirect.DocumentID)
		if err == nil {
			target, err := s.Content.Get(redirect.DocumentID)
			if err == nil && target.Meta.Published {
				suffix := ""
				if markdown {
					suffix = ".md"
				}
				http.Redirect(w, r, "/"+lang+"/"+path+suffix, http.StatusMovedPermanently)
				return
			}
		}
	}
	s.errorPage(w, r, 404, "页面不存在")
}
