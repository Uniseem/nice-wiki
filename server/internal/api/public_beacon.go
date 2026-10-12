package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
)

func (s *Server) RecordBeacon(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.BeaconInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if s.RecordVisit != nil {
		if err := s.RecordVisit(r.Context(), r, body.Path, body.Language, body.Referrer); err != nil {
			s.fail(w, err)
			return
		}
	}
	writeJSON(w, struct{}{})
}
