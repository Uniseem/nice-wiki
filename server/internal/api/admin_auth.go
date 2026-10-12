package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/auth"
	"nice-wiki/internal/httpx"
)

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.Credentials](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	account, token, err := s.Auth.Login(r.Context(), body.Username, body.Password, httpx.ClientIP(r.Context()))
	if err != nil {
		s.fail(w, err)
		return
	}
	auth.SetCookie(w, token, s.secure())
	writeJSON(w, accountResponse(account))
}
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, accountResponse(current(r).Account))
}
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.Logout(r.Context(), current(r).Token); err != nil {
		s.fail(w, err)
		return
	}
	auth.ClearCookie(w, s.secure())
	writeJSON(w, struct{}{})
}
func (s *Server) LogoutAll(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.LogoutAll(r.Context(), current(r).Account.ID); err != nil {
		s.fail(w, err)
		return
	}
	auth.ClearCookie(w, s.secure())
	writeJSON(w, struct{}{})
}
func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.ChangePassword](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := auth.ValidatePassword(body.NewPassword); err != nil {
		badRequest(w, err)
		return
	}
	principal := current(r)
	if err := s.Auth.ChangePassword(r.Context(), principal.Account.ID, principal.Token, body.OldPassword, body.NewPassword, body.Confirmation); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "change_password", principal.Account.ID) {
		return
	}
	writeJSON(w, struct{}{})
}
