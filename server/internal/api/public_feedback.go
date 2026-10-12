package api

import (
	"net/http"
	"nice-wiki/internal/ai/qa"
	"nice-wiki/internal/api/gen"
)

func (s *Server) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.FeedbackInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	session, err := s.qaSession(r)
	if err != nil {
		writeError(w, 401, "qa_session_required", "问答会话无效")
		return
	}
	if !qa.Feedback(s.Records, body.RecordId, body.Helpful) {
		writeError(w, 404, "not_found", "问答记录不存在")
		return
	}
	if s.SaveQAFeedback != nil {
		if err := s.SaveQAFeedback(r.Context(), body.RecordId, session, body.Helpful); err != nil {
			s.fail(w, err)
			return
		}
	}
	writeJSON(w, struct{}{})
}
