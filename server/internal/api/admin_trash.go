package api

import "net/http"

func (s *Server) ListTrash(w http.ResponseWriter, r *http.Request) {
	values, err := s.Content.ListTrash()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, values)
}
func (s *Server) RestoreDocument(w http.ResponseWriter, r *http.Request, id string) {
	value, err := s.Content.Restore(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "restore_document", id) {
		return
	}
	writeJSON(w, value)
}
