// Package transport 提供 portal 的 HTTP 生命周期和入口代理。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/libs/platform/apivalidate"
	portal "nice-wiki/modules/portal/internal"
	"nice-wiki/modules/portal/internal/gen"
)

const (
	moduleName    = "portal"
	moduleVersion = "1"
	maxProxyBody  = 16 << 20
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

type Application struct {
	siteID platform.Identifier
	validateBeaconBody func(json.RawMessage) error
	config             platform.ServiceConfig
	anchors            platform.TrustAnchors
	signingKey         platform.SigningKey
	delegationKey      platform.SigningKey
	runtime            portal.RuntimeReader
	identity           portal.IdentityVerifier
	authResolver       portal.AuthResolver
	registry           portal.RouteRegistry
	openedEpoch        platform.Identifier
	localHandlersReady bool
	gate               *portal.Gate
	state              atomic.Int32
	serveStarted       atomic.Bool
	shutdownOnce       sync.Once
	shutdownErr        error
	httpServer         *http.Server
}

func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, signingKey platform.SigningKey, delegationKey platform.SigningKey, runtime portal.RuntimeReader, identity portal.IdentityVerifier, authResolver portal.AuthResolver, registry portal.RouteRegistry, openedEpoch platform.Identifier, siteID platform.Identifier) (*Application, error) {
	validateBeacon, err := apivalidate.NewPayloadValidator([]byte(gen.PayloadValidationSpec), "BeaconPublicRequest")
	if err != nil { return nil, err }
	application := &Application{
		siteID: siteID, validateBeaconBody: validateBeacon,
		config: config, anchors: anchors, signingKey: signingKey, delegationKey: delegationKey, runtime: runtime,
		identity: identity, authResolver: authResolver, registry: registry, openedEpoch: openedEpoch, gate: portal.NewGate(),
	}
	mux := http.NewServeMux()
	if err := application.registerLocalRoutes(mux); err != nil {
		return nil, err
	}
	mux.HandleFunc("/", application.proxyHandler)
	application.httpServer = &http.Server{
		Addr: config.Listen, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 64 << 10,
	}
	return application, nil
}

func (a *Application) Serve(ctx context.Context) error { return Serve(ctx, a) }

func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

// registerLocalRoutes 先核对完整覆盖；不把默认503占位当作已实现handler。
func (a *Application) registerLocalRoutes(mux *http.ServeMux) error {
	handlers := map[string]http.Handler{
		"GET /healthz": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, err := Health(r.Context(), a)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, status)
		}),
		"GET /readyz": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, err := Ready(r.Context(), a)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, status)
		}),
	}
	required := []string{
		"GET /healthz", "GET /readyz", "POST /v1/barriers", "DELETE /v1/barriers/{barrierId}",
		"GET /v1/bootstrap", "GET /v1/events", "GET /v1/preferences", "PUT /v1/preferences", "GET /v1/readiness",
	}
	for _, pattern := range required {
		if handlers[pattern] == nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "portal完整本域HTTP处理器尚未装配")
		}
	}
	for _, pattern := range required {
		mux.Handle(pattern, handlers[pattern])
	}
	a.localHandlersReady = true
	return nil
}

func (a *Application) proxyHandler(w http.ResponseWriter, r *http.Request) {
	if a.state.Load() != stateReady {
		writeProblem(w, platform.NewError(platform.CodeLocked, true, "入口正在关闭"))
		return
	}
	if r.URL.RawQuery != "" && len(r.URL.RawQuery) > 8192 {
		writeProblem(w, platform.NewError(platform.CodeBudgetExceeded, false, "查询参数超出上限"))
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxProxyBody+1))
	if err != nil || int64(len(body)) > maxProxyBody {
		writeProblem(w, platform.NewError(platform.CodeBudgetExceeded, false, "请求体超出入口上限"))
		return
	}
	_ = r.Body.Close()
	requestID := r.Header.Get(platform.HeaderRequestID)
	if !platform.ValidIdentifier(requestID) {
		requestID = platform.NewIdentifier()
	}
	operationID := r.Header.Get(platform.HeaderOperationID)
	if !platform.ValidIdentifier(operationID) {
		operationID = platform.NewIdentifier()
	}
	deadline := time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano)
	op := platform.OperationContext{
		RequestID: requestID, OperationID: operationID, RootOperationID: operationID,
		CallerService: moduleName, Deadline: platform.Timestamp(deadline), DataEpoch: a.openedEpoch,
		Actor: platform.AuthContext{Kind: "visitor", SubjectID: "anonymous", SiteID: "system", ExpiresAt: platform.Timestamp(deadline)},
	}
	headers := headerMap(r.Header)
	var origins []string
	if configured, err := url.Parse(a.config.PublicBaseURL); err == nil && configured.Host != "" {
		origins = []string{configured.Scheme + "://" + configured.Host}
	}
	runtime := &portal.ProxyRuntime{
		SiteID: a.siteID, ValidateBeaconBody: a.validateBeaconBody, Registry: a.registry, Dependencies: a.config.Dependencies, SigningKey: a.signingKey, DelegationKey: a.delegationKey,
		Issuer: moduleName, Request: r, Body: body, Query: r.URL.RawQuery, Headers: headers,
		Writer: w, Runtime: a.runtime, Identity: a.identity, Gate: a.gate,
		ConfiguredOrigins: origins, TrustedProxyCIDRs: a.config.TrustedProxyCIDRs,
	}
	ctx := portal.WithProxyRuntime(r.Context(), runtime)
	ctx = portal.WithAuthResolver(ctx, a.authResolver)
	result, err := portal.ProxyRequest(ctx, op, portal.ProxyRequestInput{
		Method: r.Method, RawPath: r.URL.EscapedPath(), ContentLength: r.ContentLength,
	})
	if err != nil {
		if !runtime.ResponseStarted {
			writeProblem(w, err)
		}
		return
	}
	if result.Status == 0 {
		writeProblem(w, platform.NewError(platform.CodeInternal, false, "入口未取得响应状态"))
	}
}

func Serve(ctx context.Context, app *Application) error {
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	err := app.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "HTTP服务不可恢复错误")
	}
	return nil
}

func Shutdown(ctx context.Context, app *Application) error {
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		if err := app.httpServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				app.shutdownErr = platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全停止在途操作")
			} else {
				app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭HTTP服务失败")
			}
			return
		}
		app.state.Store(stateClosed)
	})
	return app.shutdownErr
}

func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app.state.Load() == stateClosed {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭")
	}
	return gen.HealthStatus{Module: moduleName, Version: moduleVersion, Status: http.StatusOK, Capabilities: []string{"proxy", "route-gate"}}, nil
}

func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app.state.Load() == stateClosed || !app.localHandlersReady {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "入口处理器未齐或进程已关闭")
	}
	if app.runtime == nil {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行状态依赖未装配")
	}
	view, err := app.runtime.ReadRuntimeState(ctx)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.openedEpoch != "" && view.DataEpoch != app.openedEpoch {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
	}
	// RuntimeView只含epoch与维护阶段，不能证明初始化或本域恢复回执。
	// 对应权威装配尚未完成时不返回固定true的业务就绪声明。
	return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "入口初始化与恢复就绪权威尚未装配")
}

func headerMap(headers http.Header) portal.HeaderMap {
	result := portal.HeaderMap{Entries: make([]portal.HeaderEntry, 0, len(headers))}
	for name, values := range headers {
		result.Entries = append(result.Entries, portal.HeaderEntry{Name: name, Values: append([]string(nil), values...)})
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, err error) {
	_ = problem.WriteProblem(w, problem.FromError(err, platform.NewIdentifier()))
}
