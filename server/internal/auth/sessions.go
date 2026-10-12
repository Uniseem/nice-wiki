package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"nice-wiki/internal/store/sqlc"
	"nice-wiki/internal/textutil"
	"time"
)

const CookieName = "nw_session"
const SessionLifetime = 7 * 24 * time.Hour

func (s *Service) CreateSession(ctx context.Context, accountID string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	now := time.Now()
	err := s.queries.CreateSession(ctx, sqlc.CreateSessionParams{TokenHash: textutil.Hash([]byte(token)), AccountID: accountID, CreatedAt: now.Unix(), ExpiresAt: now.Add(SessionLifetime).Unix(), LastSeenAt: now.Unix()})
	return token, err
}
func (s *Service) Session(ctx context.Context, token string) (Account, error) {
	if len(token) != 43 {
		return Account{}, ErrCredentials
	}
	session, err := s.queries.GetSession(ctx, sqlc.GetSessionParams{TokenHash: textutil.Hash([]byte(token)), ExpiresAt: time.Now().Unix()})
	if err != nil {
		return Account{}, ErrCredentials
	}
	account, err := s.ByID(session.AccountID)
	if err != nil || account.Disabled {
		return Account{}, ErrCredentials
	}
	return account, nil
}
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.queries.DeleteSession(ctx, textutil.Hash([]byte(token)))
}
func (s *Service) LogoutAll(ctx context.Context, id string) error {
	return s.queries.DeleteAccountSessions(ctx, id)
}
func SetCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: int(SessionLifetime.Seconds())})
}
func ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func (s *Service) ChangePassword(ctx context.Context, id, token, oldPassword, newPassword, confirmation string) error {
	if newPassword != confirmation {
		return invalid("两次新密码不一致")
	}
	if oldPassword == newPassword {
		return invalid("新密码不能与旧密码相同")
	}
	account, err := s.ByID(id)
	if err != nil {
		return err
	}
	if !VerifyPassword(oldPassword, account.PasswordHash) {
		return ErrCredentials
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Account{}, s.accounts...)
	for i, current := range next {
		if current.ID != id {
			continue
		}
		if current.PasswordHash != account.PasswordHash {
			return ErrConflict
		}
		if err := s.queries.DeleteOtherSessions(ctx, sqlc.DeleteOtherSessionsParams{AccountID: id, TokenHash: textutil.Hash([]byte(token))}); err != nil {
			return err
		}
		current.PasswordHash = hash
		current.MustChangePassword = false
		next[i] = current
		return s.saveLocked(next)
	}
	return ErrNotFound
}
