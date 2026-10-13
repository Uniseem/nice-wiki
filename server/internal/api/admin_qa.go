package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/ai/qa"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/memory"
	"nice-wiki/internal/settings"
	"time"
)

func (s *Server) ListQARecords(w http.ResponseWriter, r *http.Request) {
	values := s.Records.List()
	if s.ListQARecordRows != nil {
		var err error
		values, err = s.ListQARecordRows(r.Context())
		if err != nil {
			s.fail(w, err)
			return
		}
	}
	result := make([]gen.QARecord, len(values))
	for index, value := range values {
		result[index] = gen.QARecord{Id: value.ID, Question: value.Question, Answer: value.Answer, Language: value.Language, Cost: float32(value.Cost), CreatedAt: value.Created}
	}
	writeJSON(w, result)
}
func (s *Server) ClearAnswerCache(w http.ResponseWriter, r *http.Request) {
	if s.ClearAnswerCacheData != nil {
		if err := s.ClearAnswerCacheData(r.Context()); err != nil {
			s.fail(w, err)
			return
		}
	} else if s.AnswerCache != nil {
		s.AnswerCache.InvalidateAll()
	}
	if !s.audit(w, r, "clear_answer_cache", "") {
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) GetQADebug(w http.ResponseWriter, r *http.Request, id string) {
	if s.Debug == nil {
		writeError(w, 404, "not_found", "调试记录不存在")
		return
	}
	value, ok := s.Debug.Get(id)
	if !ok {
		writeError(w, 404, "not_found", "调试记录不存在")
		return
	}
	writeJSON(w, value)
}
func (s *Server) RunQADebug(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.DebugInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if s.QA == nil || s.Debug == nil {
		writeError(w, http.StatusServiceUnavailable, "ai_unavailable", "问答调试尚未配置")
		return
	}
	languages := s.Settings.Languages()
	enabled := false
	for _, language := range languages.Enabled {
		enabled = enabled || language == body.Language
	}
	if !enabled {
		writeError(w, http.StatusBadRequest, "invalid_language", "调试语言未启用")
		return
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		s.fail(w, err)
		return
	}
	id := hex.EncodeToString(raw)
	injected, version, err := memory.Inject(s.Memory, s.MemoryMapping, "qa-answer", body.Language)
	if err != nil {
		s.fail(w, err)
		return
	}
	debug := *s.QA
	debug.Cache = nil
	debug.CacheTranslator = nil
	debug.Conversations = qa.NewConversations(time.Hour)
	debug.Records = nil
	debug.Persist = nil
	debug.LoadConversation = nil
	debug.SaveSummary = nil
	debug.Budget = nil
	debug.IPBudget = nil
	debug.Bans = nil
	debug.Events = nil
	debug.StreamAnswers = false
	debug.PersistCache = nil
	debug.Limiter = guard.NewLimiter(100)
	options := s.Settings.AI()
	options.Enabled = true
	debug.Settings = func() settings.AI { return options }
	debug.Trace = func(step qa.DebugStep) { s.Debug.Add(id, step) }
	if err := debug.AskScoped(r.Context(), "debug:"+id, "debug", body.Question, body.Language, version, injected, func(qa.Event) error { return nil }); err != nil {
		s.Debug.Add(id, qa.DebugStep{Name: "error", Output: "调试执行失败", Duration: 0})
	}
	value, ok := s.Debug.Get(id)
	if !ok {
		writeError(w, http.StatusInternalServerError, "debug_failed", "未生成调试记录")
		return
	}
	writeJSON(w, value)
}
func (s *Server) FeatureAnswer(w http.ResponseWriter, r *http.Request, id string) {
	body, err := decode[gen.FeaturedInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if err := s.FeatureQAAnswer(r.Context(), id, body.Title, body.SortKey); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "feature_answer", id) {
		return
	}
	writeJSON(w, struct{}{})
}
func (s *Server) UnfeatureAnswer(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.UnfeatureQAAnswer(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "unfeature_answer", id) {
		return
	}
	writeJSON(w, struct{}{})
}
