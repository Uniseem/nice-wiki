package api

import (
	"net/http"
)

func (s *Server) ListGuardEvents(w http.ResponseWriter, r *http.Request) {
	if s.GuardEvents == nil {
		writeJSON(w, []any{})
		return
	}
	writeJSON(w, s.GuardEvents.List(200))
}
func (s *Server) Unban(w http.ResponseWriter, r *http.Request, key string) {
	if s.Bans != nil {
		s.Bans.Unban(key)
	}
	if !s.audit(w, r, "unban", key) {
		return
	}
	writeJSON(w, struct{}{})
}
