// Package transport 提供 portal 的 HTTP 生命周期和入口代理。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/geoip"
	"nice-wiki/libs/platform/problem"
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
	siteMu             sync.Mutex
	siteID             platform.Identifier
	budgetKey          platform.SigningKey
	countries          *geoip.Countries
	loadSite           func(context.Context) (platform.Identifier, error)
	webDir             string
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
	if err != nil {
		return nil, err
	}
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

// SetBudgetKey 装配匿名预算密钥（与 traffic-guard 共用），没有时匿名风控路由不可用。
func (a *Application) SetBudgetKey(key platform.SigningKey) { a.budgetKey = key }

// SetCountries 装配可选的离线 IP 国家库。
func (a *Application) SetCountries(countries *geoip.Countries) { a.countries = countries }

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
	// 屏障、bootstrap、events、preferences、readiness 等本地路由留待后续装配，不阻止启动。
	required := []string{"GET /healthz", "GET /readyz"}
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

// MountWeb 提供后台静态页面：/admin/* 回退到后台单页应用的 index.html，/_admin/* 提供其脚本与样式，/_site/* 提供公开页的样式与交互岛。
// 目录缺失时不挂载（后台页面不可用，接口代理照常工作）。
func (a *Application) MountWeb(dir string) {
	if info, err := os.Stat(filepath.Join(dir, "index.html")); err != nil || info.IsDir() {
		return
	}
	a.webDir = dir
	files := http.StripPrefix("/_admin/", http.FileServer(http.Dir(dir)))
	mux := a.httpServer.Handler.(*http.ServeMux)
	mux.HandleFunc("GET /_admin/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	// 公开页的样式表、交互岛脚本和 KaTeX：前端构建产出在 site/ 子目录。
	// 公开页按 /_site/v/<版本>/site.js 引用（版本随发版变化），去掉版本段后是同一批文件，可以长期缓存。
	site := http.FileServer(http.Dir(filepath.Join(dir, "site")))
	mux.HandleFunc("GET /_site/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/_site/")
		w.Header().Set("Cache-Control", "no-cache")
		if rest, ok := strings.CutPrefix(name, "v/"); ok {
			if _, file, found := strings.Cut(rest, "/"); found {
				name = file
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}
		request := r.Clone(r.Context())
		request.URL.Path = "/" + name
		request.URL.RawPath = ""
		site.ServeHTTP(w, request)
	})
	// worker-src 放行 blob：登录页的人机验证在 blob 创建的 Web Worker 里算工作量证明。
	shell := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; worker-src 'self' blob:; frame-ancestors 'none'")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}
	mux.HandleFunc("GET /admin", shell)
	mux.HandleFunc("GET /admin/{rest...}", shell)
}

// SetSiteLoader 登记站点身份读取函数；站点尚未初始化时先用空身份启动，
// 初始化完成后的首个请求再读取，不需要重启入口。
func (a *Application) SetSiteLoader(load func(context.Context) (platform.Identifier, error)) {
	a.loadSite = load
}

func (a *Application) currentSite(ctx context.Context) platform.Identifier {
	a.siteMu.Lock()
	defer a.siteMu.Unlock()
	if a.siteID == "" && a.loadSite != nil {
		if id, err := a.loadSite(ctx); err == nil {
			a.siteID = id
		}
	}
	return a.siteID
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
	site := a.currentSite(r.Context())
	// 站点还没初始化（读不到站点身份）：打开根路径直接去后台的初始化向导。
	if site == "" && r.Method == http.MethodGet && r.URL.Path == "/" {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	if site == "" {
		site = "system"
	}
	op := platform.OperationContext{
		RequestID: requestID, OperationID: operationID, RootOperationID: operationID,
		CallerService: moduleName, Deadline: platform.Timestamp(deadline), DataEpoch: a.openedEpoch,
		Actor: platform.AuthContext{Kind: "visitor", SubjectID: "anonymous", SiteID: site, ExpiresAt: platform.Timestamp(deadline)},
	}
	headers := headerMap(r.Header)
	var origins []string
	if configured, err := url.Parse(a.config.PublicBaseURL); err == nil && configured.Host != "" {
		origins = []string{configured.Scheme + "://" + configured.Host}
	}
	runtime := &portal.ProxyRuntime{
		SiteID: a.currentSite(r.Context()), ValidateBeaconBody: a.validateBeaconBody, Registry: a.registry, Dependencies: a.config.Dependencies, SigningKey: a.signingKey, DelegationKey: a.delegationKey,
		Issuer: moduleName, Request: r, Body: body, Query: r.URL.RawQuery, Headers: headers,
		Writer: w, Runtime: a.runtime, Identity: a.identity, Gate: a.gate,
		ConfiguredOrigins: origins, TrustedProxyCIDRs: a.config.TrustedProxyCIDRs, Countries: a.countries,
	}
	runtime.Anchors = a.anchors
	if a.budgetKey.KeyID != "" {
		runtime.NetworkScopeKeys = []platform.SigningKey{a.budgetKey}
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
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	epoch := string(view.DataEpoch)
	return gen.ServiceReadiness{
		Health: health, ActiveDataEpoch: &epoch, Initialized: app.currentSite(ctx) != "", JournalRecovered: true,
		Maintenance: view.Maintenance, PublicRequestsAllowed: !view.Maintenance && view.Phase == platform.PhaseRunning, SafePoint: true,
	}, nil
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
