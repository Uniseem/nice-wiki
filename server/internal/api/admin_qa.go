package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
)

func (s *Server) ListQARecords(w http.ResponseWriter, r *http.Request) {
	values := s.Records.List()
	if s.ListQARecordRows != nil {
		var err error
		values, err = s.ListQARecordRows(r.Context())
		if err != nil {
			s.fail(w, err)
			return
		}
	}
	result := make([]gen.QARecord, len(values))
	for index, value := range values {
		result[index] = gen.QARecord{Id: value.ID, Question: value.Question, Answer: value.Answer, Language: value.Language, Cost: float32(value.Cost), CreatedAt: value.Created}
	}
	writeJSON(w, result)
}
func (s *Server) ClearAnswerCache(w http.ResponseWriter, r *http.Request) {
	if s.AnswerCache != nil {
		s.AnswerCache.InvalidateAll()
	}
	if !s.audit(w, r, "clear_answer_cache", "") {
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) GetQADebug(w http.ResponseWriter, r *http.Request, id string) {
	if s.Debug == nil {
		writeError(w, 404, "not_found", "调试记录不存在")
		return
	}
	value, ok := s.Debug.Get(id)
	if !ok {
		writeError(w, 404, "not_found", "调试记录不存在")
		return
	}
	writeJSON(w, value)
}
func (s *Server) FeatureAnswer(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.FeaturedInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.FeatureQAAnswer(r.Context(), id, body.Title, body.SortKey); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "feature_answer", id) {
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) UnfeatureAnswer(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.UnfeatureQAAnswer(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "unfeature_answer", id) {
		return
	}
	writeJSON(w, struct{}{})
}
