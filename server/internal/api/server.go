package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"nice-wiki/internal/ai/cache"
	"nice-wiki/internal/ai/guard"
	"nice-wiki/internal/ai/qa"
	"nice-wiki/internal/api/gen"
	"nice-wiki/internal/assets"
	"nice-wiki/internal/auth"
	"nice-wiki/internal/config"
	"nice-wiki/internal/content"
	"nice-wiki/internal/history"
	"nice-wiki/internal/i18n/suggest"
	itranslate "nice-wiki/internal/i18n/translate"
	"nice-wiki/internal/jobs"
	"nice-wiki/internal/llm"
	"nice-wiki/internal/memory"
	"nice-wiki/internal/settings"
	"nice-wiki/internal/setup"
	"nice-wiki/internal/stats"
	"nice-wiki/internal/videoembed"
	"strings"
	"sync"
)

type Server struct {
	contract          *openapi3.T
	Preview           func(http.ResponseWriter, *http.Request, string)
	Assets            *assets.Store
	Auth              *auth.Service
	Content           *content.Repo
	History           *history.Repo
	Settings          *settings.Store
	Runtime           *config.RuntimeStore
	Secrets           *config.Secrets
	Channels          *llm.Channels
	Setup             *setup.Service
	Logger            *slog.Logger
	Dev               bool
	Videos            *videoembed.Registry
	QA                *qa.Service
	Conversations     *qa.Conversations
	Records           *qa.Records
	Sessions          *guard.Sessions
	PoW               *guard.PoW
	Turnstile         *guard.Turnstile
	turnstileMu       sync.RWMutex
	Bans              *guard.Bans
	GuardEvents       *guard.Events
	Debug             *qa.DebugStore
	AnswerCache       *cache.Store
	Memory            *memory.Store
	MemoryMapping     *memory.Mapping
	MemoryAssistant   *memory.Assistant
	Catalog           *llm.Catalog
	Translations      *itranslate.Files
	Glossary          *itranslate.Glossary
	Locale            *suggest.Decider
	Jobs              interface{ Progress() []jobs.Progress }
	StatsSummary      func(context.Context) (stats.Summary, error)
	RecordVisit       func(context.Context, *http.Request, string, string, string) error
	CreateBackupFile  func(context.Context, bool) (BackupResult, error)
	ListBackupFiles   func(context.Context) ([]BackupResult, error)
	OpenBackupFile    func(context.Context, string) (io.ReadCloser, int64, error)
	ListQARecordRows  func(context.Context) ([]qa.Record, error)
	SaveQAFeedback    func(context.Context, string, string, bool) error
	FeatureQAAnswer   func(context.Context, string, string, string) error
	UnfeatureQAAnswer func(context.Context, string) error
}

type BackupResult struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"createdAt"`
}

func (s *Server) getTurnstile() *guard.Turnstile {
	s.turnstileMu.RLock()
	defer s.turnstileMu.RUnlock()
	return s.Turnstile
}
func (s *Server) setTurnstile(value *guard.Turnstile) {
	s.turnstileMu.Lock()
	defer s.turnstileMu.Unlock()
	s.Turnstile = value
}

var _ gen.ServerInterface = (*Server)(nil)

func (s *Server) Handler() (http.Handler, error) {
	contract, err := gen.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("加载 API 契约: %w", err)
	}
	if err := s.checkPolicies(contract); err != nil {
		return nil, err
	}
	s.contract = contract
	return gen.HandlerWithOptions(s, gen.StdHTTPServerOptions{
		Middlewares: []gen.MiddlewareFunc{s.validateRequest, s.authorize},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, 400, "invalid_request", "请求路径参数无效")
		},
	}), nil
}

// JSON 请求先由唯一契约校验，再进入类型化解码和领域校验。
func (s *Server) validateRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operation, err := s.operation(r)
		if err != nil {
			writeError(w, 500, "contract_error", "接口契约不完整")
			return
		}
		if operation.RequestBody == nil {
			next.ServeHTTP(w, r)
			return
		}
		body := operation.RequestBody.Value
		if body == nil {
			writeError(w, 500, "contract_error", "接口契约不完整")
			return
		}
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			writeError(w, 415, "unsupported_media_type", "请求需要声明内容类型")
			return
		}
		media, exists := body.Content[contentType]
		if !exists {
			writeError(w, 415, "unsupported_media_type", "请求内容类型不符合接口约定")
			return
		}
		if contentType == "multipart/form-data" {
			// 文件在专用上传入口流控与校验，不让通用 JSON 校验器复制大文件。
			next.ServeHTTP(w, r)
			return
		}
		if contentType != "application/json" || media.Schema == nil || media.Schema.Value == nil {
			writeError(w, 500, "contract_error", "接口契约不完整")
			return
		}
		limit := int64(1 << 20)
		if strings.HasSuffix(r.URL.Path, "/draft") {
			limit = 10 << 20
		}
		original := r.Body
		bounded := http.MaxBytesReader(w, original, limit)
		data, readErr := io.ReadAll(bounded)
		closeErr := original.Close()
		if readErr != nil || closeErr != nil {
			writeError(w, 400, "invalid_request", "请求内容不完整或超过上限")
			return
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(data))
		if err := decoder.Decode(&value); err != nil {
			writeError(w, 400, "invalid_request", "请求 JSON 无效")
			return
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writeError(w, 400, "invalid_request", "请求含尾随内容")
			return
		}
		if err := media.Schema.Value.VisitJSON(value); err != nil {
			// 校验错误可能携带密码、密钥等值，响应和日志都不回显错误正文。
			writeError(w, 400, "invalid_request", "请求字段不符合接口约定")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		next.ServeHTTP(w, r)
	})
}
func (s *Server) operation(r *http.Request) (*openapi3.Operation, error) {
	pattern := r.Pattern
	separator := strings.IndexByte(pattern, ' ')
	if separator < 0 {
		return nil, errors.New("路由模式没有方法")
	}
	method := pattern[:separator]
	path := pattern[separator+1:]
	item := s.contract.Paths.Value(path)
	if item == nil {
		return nil, errors.New("路由不在契约中")
	}
	operation := item.GetOperation(method)
	if operation == nil {
		return nil, errors.New("方法不在契约中")
	}
	return operation, nil
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var value T
	limit := int64(1 << 20)
	if strings.HasSuffix(r.URL.Path, "/draft") {
		limit = 10 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, errors.New("请求 JSON 无效")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return value, errors.New("请求含尾随内容")
	}
	return value, nil
}
func writeJSON(w http.ResponseWriter, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		writeError(w, 500, "internal_error", "响应编码失败")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(data, '\n'))
}
func accountResponse(v auth.Account) gen.Account {
	permissions := make([]gen.Permission, len(v.Permissions))
	for i, p := range v.Permissions {
		permissions[i] = gen.Permission(p)
	}
	return gen.Account{Id: v.ID, Username: v.Username, Superadmin: v.Superadmin, Disabled: v.Disabled, MustChangePassword: v.MustChangePassword, Permissions: permissions}
}
func permissions(values []gen.Permission) []auth.Permission {
	result := make([]auth.Permission, len(values))
	for i, v := range values {
		result[i] = auth.Permission(v)
	}
	return result
}
func current(r *http.Request) auth.Principal { v, _ := auth.Current(r.Context()); return v }
func (s *Server) secure() bool               { return !s.Dev }
func (s *Server) audit(w http.ResponseWriter, r *http.Request, action, target string) bool {
	if err := s.Auth.Audit(r.Context(), current(r).Account, action, target); err != nil {
		s.fail(w, err)
		return false
	}
	return true
}
func (s *Server) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, gen.Health{Status: gen.Ok})
}
