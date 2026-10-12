package api

import (
	"encoding/json"
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/content"
	"nice-wiki/internal/doc"
)

func (s *Server) ListDocumentHistory(w http.ResponseWriter, r *http.Request, id string) {
	path, err := s.Content.HistoryPath(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	values, err := s.History.Log(r.Context(), path, 100)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, values)
}

func (s *Server) DiffDocumentHistory(w http.ResponseWriter, r *http.Request, id string, params gen.DiffDocumentHistoryParams) {
	path, err := s.Content.HistoryPath(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	value, err := s.History.Diff(r.Context(), params.Before, params.After, path)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, value)
}

func (s *Server) RestoreDocumentHistory(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.RestoreHistory](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	path, err := s.Content.HistoryPath(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	data, err := s.History.Read(r.Context(), body.Revision, path)
	if err != nil {
		s.fail(w, err)
		return
	}
	var snapshot content.File
	if err := json.Unmarshal(data, &snapshot); err != nil {
		s.fail(w, err)
		return
	}
	if snapshot.Meta.ID != id {
		badRequest(w, content.ErrConflict)
		return
	}
	encoded, err := json.Marshal(snapshot.Document)
	if err != nil {
		s.fail(w, err)
		return
	}
	document, err := doc.Parse(encoded)
	if err != nil {
		s.fail(w, err)
		return
	}
	value, err := s.Content.SaveDraft(id, current(r).Account.ID, body.LockToken, body.DraftVersion, document)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "restore_history_draft", id) {
		return
	}
	writeJSON(w, value)
}
