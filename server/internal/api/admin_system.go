package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/config"
)

func (s *Server) GetRuntime(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.Runtime.Get()) }
func (s *Server) SaveRuntime(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.RuntimeSettings](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value := config.Runtime{TrustedProxies: body.TrustedProxies, TurnstileSiteKey: body.TurnstileSiteKey, DailySpendLimit: body.DailySpendLimit, GitRemote: body.GitRemote, BackupHour: body.BackupHour, BackupKeep: body.BackupKeep}
	if err := s.Runtime.Save(value); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_runtime", "") {
		return
	}
	writeJSON(w, s.Runtime.Get())
}
