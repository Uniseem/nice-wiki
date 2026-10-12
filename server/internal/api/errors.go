package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"nice-wiki/internal/assets"
	"nice-wiki/internal/auth"
	"nice-wiki/internal/config"
	"nice-wiki/internal/content"
	"nice-wiki/internal/settings"
)

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}
func (s *Server) fail(w http.ResponseWriter, err error) {
	if message, ok := validationMessage(err); ok {
		writeError(w, 400, "invalid_request", message)
		return
	}

	switch {
	case errors.Is(err, auth.ErrCredentials):
		writeError(w, 401, "invalid_credentials", "账号或密码错误")
	case errors.Is(err, auth.ErrLocked):
		writeError(w, 429, "login_locked", "登录暂时锁定，请稍后重试")
	case errors.Is(err, auth.ErrNotFound), errors.Is(err, content.ErrNotFound):
		writeError(w, 404, "not_found", "请求的内容不存在")
	case errors.Is(err, auth.ErrConflict), errors.Is(err, content.ErrConflict):
		writeError(w, 409, "conflict", "内容已改变，请重新加载")
	case errors.Is(err, content.ErrLocked):
		writeError(w, 409, "edit_lock_lost", "编辑锁已失效或由其他账号持有")
	default:
		// 错误详细内容仅进入后台日志；公开响应不泄露路径或配置。
		s.Logger.Error("操作失败", "errorType", fmt.Sprintf("%T", err))
		writeError(w, 500, "internal_error", "操作失败，请检查服务状态")
	}
}
func badRequest(w http.ResponseWriter, err error) { writeError(w, 400, "invalid_request", err.Error()) }

func validationMessage(err error) (string, bool) {
	var account *auth.ValidationError
	if errors.As(err, &account) {
		return account.Message, true
	}
	var document *content.ValidationError
	if errors.As(err, &document) {
		return document.Message, true
	}
	var asset *assets.ValidationError
	if errors.As(err, &asset) {
		return asset.Message, true
	}
	var setting *settings.ValidationError
	if errors.As(err, &setting) {
		return setting.Message, true
	}
	var runtime *config.ValidationError
	if errors.As(err, &runtime) {
		return runtime.Message, true
	}
	return "", false
}
func (s *Server) validationOrFailure(w http.ResponseWriter, err error) {
	if message, ok := validationMessage(err); ok {
		writeError(w, 400, "invalid_request", message)
		return
	}
	s.fail(w, err)
}
