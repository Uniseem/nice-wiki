package api

import (
	"net/http"
)

func (s *Server) GetStats(w http.ResponseWriter, r *http.Request) {
	if s.StatsSummary == nil {
		writeJSON(w, map[string]any{"views": 0, "visitors": 0, "aiCrawls": 0, "popular": []any{}})
		return
	}
	value, err := s.StatsSummary(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
