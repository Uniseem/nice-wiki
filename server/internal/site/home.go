package site

import "net/http"

func (s *Site) home(w http.ResponseWriter, r *http.Request, lang string) {
	data := s.page(lang, s.Settings.Site().Name)
	data.Description = data.Site.Description
	s.render(w, r, 200, "home", data)
}
