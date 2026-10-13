package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/ai/qa"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/httpx"
	"nice-wiki/internal/memory"
	"strings"
	"time"
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
	now := time.Now()
	ipScope := "ip:unknown"
	if s.GuardScope != nil {
		ipScope = s.GuardScope(httpx.ClientIP(r.Context()))
	}
	if s.Bans != nil {
		if until, blocked := s.Bans.Blocked(ipScope, now); blocked {
			writeError(w, http.StatusTooManyRequests, "qa_banned", fmt.Sprintf("AI 问答已暂时封禁，请在 %s 后重试", until.Format(time.RFC3339)))
			return
		}
	}
	if s.Checks != nil {
		signal := s.Checks.Inspect("session:"+session, body.Question, now)
		risk := guard.Combine(signal.Frequency, signal.Repetition, guard.RiskLow)
		if risk != guard.RiskLow && s.GuardEvents != nil {
			reason := "frequency"
			if signal.Repetition {
				reason = "repetition"
			}
			s.GuardEvents.Add(guard.Event{Scope: ipScope, Reason: reason, Risk: risk, Created: now})
		}
		if risk != guard.RiskLow {
			s.Checks.Escalate(ipScope, now)
		}
		switch risk {
		case guard.RiskHigh:
			if s.Bans != nil {
				s.Bans.Ban(ipScope, now)
			}
			writeError(w, http.StatusTooManyRequests, "qa_banned", "AI 问答已暂时封禁，请在 10 分钟后重试")
			return
		case guard.RiskMedium:
			writeError(w, http.StatusForbidden, "verification_required", "检测到异常提问节奏，请重新完成人机验证")
			return
		}
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
	pageID := ""
	if body.Page != "" {
		if parsed, parseErr := url.Parse(body.Page); parseErr == nil {
			path := strings.TrimPrefix(parsed.Path, "/")
			if strings.HasPrefix(path, language+"/") {
				path = strings.TrimPrefix(path, language+"/")
				if file, ok := s.Content.ResolvePath(strings.TrimSuffix(path, ".md")); ok {
					pageID = file.Meta.ID
				}
			}
		}
	}
	stream, err := httpx.NewSSE(w)
	if err != nil {
		s.fail(w, err)
		return
	}
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-r.Context().Done():
				return
			case <-ticker.C:
				_ = stream.Comment("heartbeat")
			}
		}
	}()
	if err := s.QA.AskPage(r.Context(), session, ipScope, pageID, body.Question, language, version, injected, func(event qa.Event) error { return stream.Event(event.Type, event) }); err != nil {
		message := "问答处理失败"
		code := "qa_failed"
		switch {
		case errors.Is(err, guard.ErrBudgetExceeded):
			message, code = "问答花费已达到限制，请稍后重试", "budget_exceeded"
		case errors.Is(err, guard.ErrVerificationRequired):
			message, code = "请求需要重新完成人机验证", "verification_required"
		case errors.Is(err, guard.ErrBanned):
			message, code = "AI 问答已暂时封禁，请在 10 分钟后重试", "qa_banned"
		case errors.Is(err, guard.ErrOutOfScope):
			message, code = "这个问题不在本站文档范围内", "out_of_scope"
		}
		_ = stream.Event("error", qa.Event{Type: "error", Message: message, Code: code})
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
	if !ok && s.LoadQAConversation != nil {
		value, ok, err = s.LoadQAConversation(r.Context(), id)
		if err != nil {
			s.fail(w, err)
			return
		}
		if ok {
			if err := s.Conversations.Put(value); err != nil {
				s.fail(w, err)
				return
			}
		}
	}
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
	if s.ClearQAConversation != nil {
		if err := s.ClearQAConversation(r.Context(), id); err != nil {
			s.fail(w, err)
			return
		}
	}
	if s.Conversations != nil {
		s.Conversations.Clear(id)
	}
	writeJSON(w, struct{}{})
}
