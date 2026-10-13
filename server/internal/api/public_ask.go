package api

import (
	"net/http"
	"nice-wiki/internal/ai/qa"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/httpx"
	"nice-wiki/internal/memory"
)

func (s *Server) qaSession(r *http.Request) (string, error) {
	cookie, err := r.Cookie("nw_qa")
	if err != nil {
		return "", err
	}
	return s.Sessions.Verify(cookie.Value)
}
func (s *Server) Ask(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.AskInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if s.QA == nil || s.Settings == nil || !s.Settings.AI().Enabled || s.QA.Ready != nil && !s.QA.Ready() {
		writeError(w, 503, "ai_unavailable", "AI 问答尚未启用")
		return
	}
	session, err := s.qaSession(r)
	if err != nil {
		writeError(w, 401, "qa_session_required", "请先建立问答会话")
		return
	}
	languages := s.Settings.Languages()
	language := languages.Base
	if body.Language != nil && *body.Language != "" {
		language = *body.Language
	}
	enabled := false
	for _, code := range languages.Enabled {
		enabled = enabled || code == language
	}
	if !enabled {
		writeError(w, 400, "invalid_language", "问答语言未启用")
		return
	}
	injected, version, err := memory.Inject(s.Memory, s.MemoryMapping, "qa-answer", language)
	if err != nil {
		s.fail(w, err)
		return
	}
	stream, err := httpx.NewSSE(w)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.QA.Ask(r.Context(), session, body.Question, language, version, injected, func(event qa.Event) error { return stream.Event(event.Type, event) }); err != nil {
		_ = stream.Event("error", qa.Event{Type: "error", Message: "问答处理失败"})
	}
}
func (s *Server) GetConversation(w http.ResponseWriter, r *http.Request, id string) {
	session, err := s.qaSession(r)
	if err != nil || session != id {
		writeError(w, 401, "qa_session_required", "问答会话无效")
		return
	}
	if s.Conversations == nil {
		writeError(w, 503, "ai_unavailable", "对话存储尚未配置")
		return
	}
	value, ok := s.Conversations.Get(id)
	if !ok {
		writeError(w, 404, "not_found", "对话不存在")
		return
	}
	writeJSON(w, value)
}
func (s *Server) ClearConversation(w http.ResponseWriter, r *http.Request, id string) {
	session, err := s.qaSession(r)
	if err != nil || session != id {
		writeError(w, 401, "qa_session_required", "问答会话无效")
		return
	}
	if s.Conversations != nil {
		s.Conversations.Clear(id)
	}
	writeJSON(w, struct{}{})
}
