package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
)

func (s *Server) ListAudit(w http.ResponseWriter, r *http.Request) {
	values, err := s.Auth.ListAudit(r.Context(), 100, 0)
	if err != nil {
		s.fail(w, err)
		return
	}
	result := make([]gen.AuditEvent, len(values))
	for i, v := range values {
		result[i] = gen.AuditEvent{Id: v.ID, AccountId: v.AccountID, AccountName: v.AccountName, Action: v.Action, TargetId: v.TargetID, CreatedAt: v.CreatedAt}
	}
	writeJSON(w, result)
}
