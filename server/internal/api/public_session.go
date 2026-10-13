package api

import (
	"net/http"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/httpx"
	"time"
)

func (s *Server) GetPowChallenge(w http.ResponseWriter, r *http.Request) {
	if s.PoW == nil {
		writeError(w, 503, "ai_unavailable", "问答验证尚未配置")
		return
	}
	now := time.Now()
	difficulty := 2
	if s.Checks != nil && s.GuardScope != nil {
		difficulty = s.Checks.Difficulty(s.GuardScope(httpx.ClientIP(r.Context())), now)
	}
	challenge := s.PoW.Challenge(httpx.ClientIP(r.Context()), difficulty, now)
	writeJSON(w, gen.PowChallenge{Challenge: challenge, Difficulty: difficulty})
}
func (s *Server) CreatePublicSession(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.PublicSessionInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	verified := false
	turnstile := s.getTurnstile()
	if body.TurnstileToken != "" && turnstile != nil {
		verified = turnstile.Verify(r.Context(), body.TurnstileToken, httpx.ClientIP(r.Context())) == nil
	}
	if !verified && body.PowChallenge != "" && s.PoW != nil {
		verified = s.PoW.Verify(body.PowChallenge, body.PowNonce, time.Now(), httpx.ClientIP(r.Context())) == nil
	}
	if !verified {
		writeError(w, 403, "verification_required", "需要完成人机验证")
		return
	}
	if s.Sessions == nil {
		writeError(w, 503, "ai_unavailable", "问答会话尚未配置")
		return
	}
	token := ""
	id := ""
	if cookie, cookieErr := r.Cookie("nw_qa"); cookieErr == nil {
		if current, verifyErr := s.Sessions.Verify(cookie.Value); verifyErr == nil {
			id = current
			token, err = s.Sessions.Renew(id)
		}
	}
	if token == "" {
		token, err = s.Sessions.New()
		if err == nil {
			id, err = s.Sessions.Verify(token)
		}
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	if s.Checks != nil {
		s.Checks.Reset("session:" + id)
		if s.GuardScope != nil {
			s.Checks.Reset(s.GuardScope(httpx.ClientIP(r.Context())))
		}
	}
	expires := time.Now().Add(24 * time.Hour)
	http.SetCookie(w, &http.Cookie{Name: "nw_qa", Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode})
	writeJSON(w, gen.PublicSession{Id: id, ExpiresAt: expires.Unix()})
}
