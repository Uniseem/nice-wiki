package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

type contextKey struct{}
type Principal struct {
	Account Account
	Token   string
}

func Current(ctx context.Context) (Principal, bool) {
	value, ok := ctx.Value(contextKey{}).(Principal)
	return value, ok
}
func WithPrincipal(ctx context.Context, value Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, value)
}
func (s *Service) Authenticate(r *http.Request) (Principal, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return Principal{}, ErrCredentials
	}
	account, err := s.Session(r.Context(), cookie.Value)
	return Principal{Account: account, Token: cookie.Value}, err
}
func ValidateOrigin(r *http.Request, baseURL string, dev bool) error {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return nil
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return errors.New("修改请求缺少 Origin")
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("请求来源无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("请求来源协议无效")
	}
	if baseURL != "" {
		base, err := url.Parse(baseURL)
		if err == nil && base.Scheme == parsed.Scheme && base.Host == parsed.Host {
			return nil
		}
	}
	scheme := "https"
	if dev {
		scheme = "http"
	}
	if parsed.Scheme == scheme && parsed.Host == r.Host {
		return nil
	}
	if dev && (origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173") {
		return nil
	}
	return errors.New("请求来源不属于当前站点")
}
