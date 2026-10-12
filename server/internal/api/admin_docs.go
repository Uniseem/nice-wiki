package api

import (
	"encoding/json"
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/doc"
)

func (s *Server) ListDocuments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Content.List())
}
func (s *Server) CreateDocument(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.CreateDocument](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Content.Create(r.Context(), body.ParentId, body.Title, body.Slug)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) GetDraft(w http.ResponseWriter, r *http.Request, id string) {
	value, err := s.Content.Draft(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) SaveDraft(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.SaveDraft](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	data, err := json.Marshal(body.Document)
	if err != nil {
		badRequest(w, err)
		return
	}
	document, err := doc.Parse(data)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Content.SaveDraft(id, current(r).Account.ID, body.LockToken, body.Version, document)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) AcquireLock(w http.ResponseWriter, r *http.Request, id string) {
	if _, err := s.Content.Get(id); err != nil {
		s.fail(w, err)
		return
	}
	principal := current(r)
	value, err := s.Content.Locks().Acquire(id, principal.Account.ID, principal.Account.Username)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) HeartbeatLock(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.LockToken](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Content.Locks().Heartbeat(id, current(r).Account.ID, body.Token)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) ReleaseLock(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.LockToken](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.Content.Locks().Release(id, current(r).Account.ID, body.Token); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) ForceReleaseLock(w http.ResponseWriter, r *http.Request, id string) {
	s.Content.Locks().ForceRelease(id)
	if !s.audit(w, r, "force_release_lock", id) {
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) PublishDocument(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.PublishDocument](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	principal := current(r)
	value, err := s.Content.Publish(r.Context(), id, principal.Account.ID, principal.Account.Username, body.LockToken, body.DraftVersion, body.Title, body.Slug, body.SeoDescription)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "publish_document", id) {
		return
	}
	writeJSON(w, value)
}
func (s *Server) WithdrawDocument(w http.ResponseWriter, r *http.Request, id string) {
	value, err := s.Content.Withdraw(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "withdraw_document", id) {
		return
	}
	writeJSON(w, value)
}
func (s *Server) MoveDocument(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.MoveDocument](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value, err := s.Content.Move(r.Context(), id, body.ParentId, body.BeforeId, body.AfterId)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) TrashDocument(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.Content.Trash(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "trash_document", id) {
		return
	}
	writeJSON(w, struct{}{})
}

func (s *Server) PreviewDocument(w http.ResponseWriter, r *http.Request, id string) {
	s.Preview(w, r, id)
}
