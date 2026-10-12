package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/auth"
)

func (s *Server) ListAccounts(w http.ResponseWriter, r *http.Request) {
	values := s.Auth.List()
	result := make([]gen.Account, len(values))
	for i, v := range values {
		result[i] = accountResponse(v)
	}
	writeJSON(w, result)
}
func (s *Server) CreateAccount(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.CreateAccount](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := auth.ValidatePassword(body.Password); err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Auth.Create(body.Username, body.Password, permissions(body.Permissions))
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "create_account", value.ID) {
		return
	}
	writeJSON(w, accountResponse(value))
}
func (s *Server) UpdateAccount(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.UpdateAccount](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Auth.Update(r.Context(), id, body.Disabled, permissions(body.Permissions))
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "update_account", id) {
		return
	}
	writeJSON(w, accountResponse(value))
}
func (s *Server) DeleteAccount(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.Auth.Delete(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "delete_account", id) {
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) ResetPassword(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.ResetPassword](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := auth.ValidatePassword(body.Password); err != nil {
		badRequest(w, err)
		return
	}
	if err := s.Auth.ResetPassword(r.Context(), id, body.Password); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "reset_password", id) {
		return
	}
	writeJSON(w, struct{}{})
}
