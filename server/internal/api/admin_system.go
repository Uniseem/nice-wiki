package api

import (
	"net/http"
	"nice-wiki/internal/ai/guard"
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
	value := config.Runtime{TrustedProxies: body.TrustedProxies, TurnstileSiteKey: body.TurnstileSiteKey, DailySpendLimit: body.DailySpendLimit, SessionHourlySpendLimit: body.SessionHourlySpendLimit, IPHourlySpendLimit: body.IpHourlySpendLimit, GitRemote: body.GitRemote, BackupHour: body.BackupHour, BackupKeep: body.BackupKeep}
	if err := value.Validate(); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if body.TurnstileSecret != nil {
		if err := s.Secrets.Set("turnstile", *body.TurnstileSecret); err != nil {
			s.fail(w, err)
			return
		}
		if *body.TurnstileSecret == "" {
			s.setTurnstile(nil)
		} else {
			s.setTurnstile(&guard.Turnstile{Secret: *body.TurnstileSecret, Client: http.DefaultClient})
		}
	}
	if body.GitUsername != nil {
		if err := s.Secrets.Set("git-username", *body.GitUsername); err != nil {
			s.fail(w, err)
			return
		}
	}
	if body.GitPassword != nil {
		if err := s.Secrets.Set("git-password", *body.GitPassword); err != nil {
			s.fail(w, err)
			return
		}
	}
	if err := s.Runtime.Save(value); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_runtime", "") {
		return
	}
	writeJSON(w, s.Runtime.Get())
}
