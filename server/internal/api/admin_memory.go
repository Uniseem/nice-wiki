package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/memory"
)

func (s *Server) ListMemory(w http.ResponseWriter, r *http.Request) {
	if s.Memory == nil {
		writeJSON(w, []memory.Item{})
		return
	}
	values, err := s.Memory.List()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, values)
}
func (s *Server) SaveMemory(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.MemoryItem](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	value := memory.Item{ID: body.Id, Title: body.Title, Description: body.Description, Languages: body.Languages, Source: memory.Source(body.Source), ReadOnly: body.ReadOnly, Limit: body.Limit, Content: body.Content}
	if err := s.Memory.Save(value); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_memory", body.Id) {
		return
	}
	writeJSON(w, value)
}
func (s *Server) GetMemoryTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.MemoryMapping.All())
}
func (s *Server) SaveMemoryTasks(w http.ResponseWriter, r *http.Request) {
	body, err := decode[map[string][]string](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.MemoryMapping.Replace(body); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "save_memory_tasks", "") {
		return
	}
	writeJSON(w, body)
}
func (s *Server) AskMemoryAssistant(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.MemoryAssistantInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	values, err := s.MemoryAssistant.Ask(r.Context(), body.Conversation, body.Message)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, values)
}
func (s *Server) ApplyMemoryProposals(w http.ResponseWriter, r *http.Request, conversation string) {
	body, err := decode[gen.MemoryApplyInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.MemoryAssistant.Apply(conversation, body.Accepted); err != nil {
		s.validationOrFailure(w, err)
		return
	}
	if !s.audit(w, r, "apply_memory_proposals", conversation) {
		return
	}
	writeJSON(w, struct{}{})
}
