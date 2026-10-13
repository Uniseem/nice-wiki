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
	challenge := s.PoW.Challenge(httpx.ClientIP(r.Context()), 2, time.Now())
	writeJSON(w, gen.PowChallenge{Challenge: challenge, Difficulty: 2})
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
		verified = turnstile.Verify(r.Context(), body.TurnstileToken, "") == nil
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
	token, err := s.Sessions.New()
	if err != nil {
		s.fail(w, err)
		return
	}
	id, err := s.Sessions.Verify(token)
	if err != nil {
		s.fail(w, err)
		return
	}
	expires := time.Now().Add(24 * time.Hour)
	http.SetCookie(w, &http.Cookie{Name: "nw_qa", Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: s.secure(), SameSite: http.SameSiteLaxMode})
	writeJSON(w, gen.PublicSession{Id: id, ExpiresAt: expires.Unix()})
}
