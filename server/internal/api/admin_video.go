package api

import (
	"errors"
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/videoembed"
)

func (s *Server) ResolveVideo(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.VideoResolveInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Videos.Resolve(r.Context(), body.Url)
	if err != nil && !errors.Is(err, videoembed.ErrManualMetadata) {
		s.validationOrFailure(w, err)
		return
	}
	writeJSON(w, value)
}
