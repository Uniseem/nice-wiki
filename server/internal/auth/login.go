package auth

import (
	"context"
	"database/sql"
	"errors"
	"nice-wiki/internal/store/sqlc"
	"nice-wiki/internal/textutil"
	"time"
)

// 登录失败状态由数据库持久化，串行更新避免同时失败丢失计数。
func (s *Service) Login(ctx context.Context, username, password, ip string) (Account, string, error) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	now := time.Now().Unix()
	subjects := []sqlc.GetLoginFailureParams{{Scope: "account", SubjectHash: textutil.Hash([]byte(username))}, {Scope: "ip", SubjectHash: textutil.Hash([]byte(ip))}}
	for _, subject := range subjects {
		failure, err := s.queries.GetLoginFailure(ctx, subject)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Account{}, "", err
		}
		if failure.LockedUntil > now {
			return Account{}, "", ErrLocked
		}
	}
	account, lookupErr := s.byName(username)
	encoded := account.PasswordHash
	if lookupErr != nil {
		encoded = s.dummyHash
	}
	valid := VerifyPassword(password, encoded)
	if lookupErr != nil || !valid || account.Disabled {
		for _, subject := range subjects {
			previous, err := s.queries.GetLoginFailure(ctx, subject)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return Account{}, "", err
			}
			if previous.WindowStart < now-900 {
				previous.Attempts = 0
				previous.WindowStart = now
			}
			previous.Attempts++
			until := int64(0)
			if previous.Attempts >= 5 {
				until = now + 900
			}
			if err := s.queries.SaveLoginFailure(ctx, sqlc.SaveLoginFailureParams{Scope: subject.Scope, SubjectHash: subject.SubjectHash, Attempts: previous.Attempts, WindowStart: previous.WindowStart, LockedUntil: until}); err != nil {
				return Account{}, "", err
			}
		}
		if err := s.Audit(ctx, Account{}, "login_failed", ""); err != nil {
			return Account{}, "", err
		}
		return Account{}, "", ErrCredentials
	}
	for _, subject := range subjects {
		if err := s.queries.DeleteLoginFailure(ctx, sqlc.DeleteLoginFailureParams(subject)); err != nil {
			return Account{}, "", err
		}
	}
	latest, err := s.ByID(account.ID)
	if err != nil || latest.Disabled || latest.PasswordHash != account.PasswordHash {
		return Account{}, "", ErrCredentials
	}
	account = latest
	token, err := s.CreateSession(ctx, account.ID)
	if err != nil {
		return Account{}, "", err
	}
	if err := s.Audit(ctx, account, "login", ""); err != nil {
		return Account{}, "", err
	}
	return account, token, nil
}
