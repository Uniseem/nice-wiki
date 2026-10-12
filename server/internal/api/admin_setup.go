package api

import (
	"errors"
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/auth"
	"nice-wiki/internal/settings"
)

func (s *Server) GetSetup(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.Setup.State()) }
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.Credentials](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := auth.ValidatePassword(body.Password); err != nil {
		badRequest(w, err)
		return
	}
	account, err := s.Auth.Register(body.Username, body.Password)
	if err != nil {
		s.fail(w, err)
		return
	}
	token, err := s.Auth.CreateSession(r.Context(), account.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	auth.SetCookie(w, token, s.secure())
	writeJSON(w, accountResponse(account))
}
func (s *Server) SetBaseLanguage(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.LanguageSettings](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if len(s.Content.List()) > 0 {
		badRequest(w, errors.New("已有文档时不能修改基准语言"))
		return
	}
	if err := s.Settings.SetBaseLanguage(settings.Languages{Base: body.Base, Enabled: body.Enabled}); err != nil {
		badRequest(w, err)
		return
	}
	writeJSON(w, s.Settings.Languages())
}
func (s *Server) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	if err := s.Setup.Complete(); err != nil {
		badRequest(w, err)
		return
	}
	writeJSON(w, s.Setup.State())
}
