package api

import (
	"net/http"
	"nice-wiki/internal/jobs"
)

func (s *Server) ListJobs(w http.ResponseWriter, r *http.Request) {
	if s.Jobs == nil {
		writeJSON(w, []any{})
		return
	}
	writeJSON(w, s.Jobs.Progress())
}
func (s *Server) RebuildSearchIndex(w http.ResponseWriter, r *http.Request) {
	queue, ok := s.Jobs.(*jobs.Service)
	if !ok {
		writeError(w, 503, "jobs_unavailable", "任务队列不可用")
		return
	}
	if err := queue.Enqueue(r.Context(), jobs.RebuildIndex, "all"); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "rebuild_search_index", "") {
		return
	}
	writeJSON(w, struct{}{})
}
