package site

import "net/http"

func (s *Site) errorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	data := s.page(s.Settings.Languages().Base, message)
	data.Error = message
	s.render(w, r, status, "error", data)
}
