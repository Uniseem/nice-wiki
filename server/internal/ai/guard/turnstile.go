package guard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type Turnstile struct {
	Secret string
	Client *http.Client
}

func (t Turnstile) Verify(ctx context.Context, token, ip string) error {
	if t.Secret == "" {
		return errors.New("Turnstile 尚未配置")
	}
	form := url.Values{"secret": {t.Secret}, "response": {token}}
	if ip != "" {
		form.Set("remoteip", ip)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := t.Client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	var value struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, response.Body, 1<<20)).Decode(&value); err != nil {
		return err
	}
	if !value.Success {
		return errors.New("人机验证失败")
	}
	return nil
}
