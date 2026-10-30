// Package transport 实现 identity 域的 HTTP 生命周期：匿名健康/就绪、
// 优雅关闭与重复启动保护；业务路由随各用例工作包接入。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/libs/platform/secretbox"
	"nice-wiki/modules/identity/internal"
	"nice-wiki/modules/identity/internal/gen"
)

type OperationContext = platform.OperationContext

type operationExecutor[I any, O any] func(context.Context, identity.OperationContext, I) (O, error)

const (
	moduleName    = "identity"
	moduleVersion = "1"
)

// lifecycle 状态：ready 接受请求，draining 拒新入口，closed 资源已释放。
const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是 identity 装配完成后的内部容器（x-internal，不序列化）。
type Application struct {
	config              platform.ServiceConfig
	anchors             platform.TrustAnchors
	runtime             platform.SiteConfigClient
	store               *identity.Store
	openedEpoch         platform.Identifier
	sessionRuntime      identity.SessionRuntime
	loginRuntime        identity.LoginRuntime
	registrationRuntime identity.RegistrationClient
	secretBox           *secretbox.Box
	state               atomic.Int32
	serveStarted        atomic.Bool
	drainOnce           sync.Once
	drainErr            error
	shutdownOnce        sync.Once
	shutdownErr         error
	httpServer          *http.Server
	backgroundTasks     []platform.BackgroundTask
	contentGateway      *identity.ContentGateway
}

// NewApplication 组装真实认证组并要求公共入口核对完整OpenAPI覆盖。
// 缺其余handler明确失败；openedEpoch绑定当前打开的业务代次。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, store *identity.Store, openedEpoch platform.Identifier, sessionRuntime identity.SessionRuntime) (*Application, error) {
	application := &Application{config: config, anchors: anchors, runtime: runtimeAuthority, store: store,
		openedEpoch: openedEpoch, sessionRuntime: sessionRuntime,
		loginRuntime:        identity.LoginRuntime{Client: gen.NewTrafficGuardClient(config.Dependencies["traffic-guard"], moduleName, sessionRuntime.Key)},
		registrationRuntime: identity.RegistrationClient{Client: gen.NewSiteConfigClient(config.Dependencies["site-config"], moduleName, sessionRuntime.Key)}}
	if endpoint := config.Dependencies["content"]; endpoint != "" {
		application.contentGateway = identity.NewContentGateway(endpoint, moduleName, sessionRuntime.Key)
	}
	if config.MasterKeyRef != "" {
		box, err := secretbox.Load(config.MasterKeyRef)
		if err != nil {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "主密钥文件不可用")
		}
		application.secretBox = box
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "身份HTTP规格不可读取")
	}
	handlers := application.operationHandlers()
	callers := map[string][]string{}
	for id := range handlers {
		if id != "health" && id != "ready" {
			callers[id] = []string{"portal"}
		}
	}
	callers["resolveHost"] = []string{"portal", "public-site", "content", "qa", "traffic-guard"}
	callers["allowTlsHost"] = []string{"portal"}
	callers["redeemPrivateAccess"] = []string{"portal"}
	callers["readSiteAiPolicy"] = []string{"model-gateway", "qa"}
	callers["readSiteStoragePolicy"] = []string{"media"}
	callers["readMediaStorage"] = []string{"media"}
	callers["noteVisitorQaCap"] = []string{"qa"}
	callers["refreshDriveToken"] = []string{"content"}
	callers["checkAuthorization"] = []string{"portal", "identity", "site-config", "content", "media", "public-site", "model-gateway", "knowledge", "qa", "localization", "memory", "traffic-guard", "jobs", "analytics"}
	serviceCallers := map[string][]string{
		"resolveHost":  {"portal", "public-site", "content", "qa", "traffic-guard"},
		"allowTlsHost": {"portal"}, "redeemPrivateAccess": {"portal"}, "readSiteAiPolicy": {"model-gateway", "qa"}, "noteVisitorQaCap": {"qa"},
		"readSiteStoragePolicy": {"media"}, "readMediaStorage": {"media"},
	}
	limits := map[string]int64{}
	for id := range handlers {
		limits[id] = 16 << 10
	}
	limits["submitTheme"] = 2 << 20
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self: moduleName, Anchors: anchors, OpenedDataEpoch: openedEpoch, AllowedCallers: callers,
		ServiceActorCallers: serviceCallers,
		AdmitBusiness:       lifecycle.SimpleAdmission(runtimeAuthority.ReadRuntimeState, openedEpoch), OperationBodyLimits: limits,
		Runtime: func(ctx context.Context) (platform.RuntimeView, error) {
			return lifecycle.ReadRuntime(ctx, runtimeAuthority)
		},
		Authorize: application.authorizeOperation,
	}, handlers)
	if err != nil {
		return nil, err
	}
	application.httpServer = &http.Server{Addr: config.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 64 << 10}
	return application, nil
}

// identityContext 只装配进程持有的资源；可信op由平台入口一次建立。
func (a *Application) identityContext(ctx context.Context) context.Context {
	ctx = identity.WithIdentityStore(ctx, a.store)
	ctx = identity.WithSessionRuntime(ctx, a.sessionRuntime)
	ctx = identity.WithSessionCursorKey(ctx, a.sessionRuntime.Key)
	ctx = identity.WithLoginRuntime(ctx, a.loginRuntime)
	ctx = identity.WithRegistrationRuntime(ctx, a.registrationRuntime)
	if a.secretBox != nil {
		ctx = identity.WithSecretBox(ctx, a.secretBox)
	}
	if a.contentGateway != nil {
		ctx = identity.WithContentGateway(ctx, a.contentGateway)
	}
	return ctx
}

func (a *Application) authorizeOperation(ctx context.Context, op OperationContext, permission, action string) error {
	sessionID := op.Actor.SessionID
	decision, err := identity.CheckAuthorization(a.identityContext(ctx), op, identity.AuthorizationRequest{
		SubjectID: op.Actor.SubjectID, SessionID: &sessionID, SiteID: op.Actor.SiteID, Permission: permission, Action: action})
	if err != nil {
		return err
	}
	if !decision.Authorized {
		return platform.NewError(platform.CodeForbidden, false, "当前账户没有该操作权限")
	}
	return nil
}

// operationHandlers 只登记真实已完成用例；缺其他合同操作时公共工厂拒绝整个启动。
func (a *Application) operationHandlers() map[string]http.Handler {
	handlers := map[string]http.Handler{
		"readRegistrationState":   jsonOperation(a, identity.ReadRegistrationState),
		"registerFirstSuperadmin": jsonOperation(a, identity.RegisterFirstSuperadmin),
		"createSession":           jsonOperation(a, identity.CreateSession),
		"refreshSession":          jsonOperation(a, identity.RefreshSession),
		"createAccount":           jsonOperation(a, identity.CreateAccount),
		"setAccountStatus": jsonOperation(a, identity.SetAccountStatus, func(input *identity.SetAccountStatusInput, r *http.Request) {
			input.AccountID = r.PathValue("accountId")
		}),
		"setAccountPermissions": jsonOperation(a, identity.SetAccountPermissions, func(input *identity.SetAccountPermissionsInput, r *http.Request) {
			input.AccountID = r.PathValue("accountId")
		}),
		"resetAccountPassword": jsonOperation(a, identity.ResetAccountPassword, func(input *identity.ResetAccountPasswordInput, r *http.Request) {
			input.AccountID = r.PathValue("accountId")
		}),
		"readSelf":           jsonOperation(a, identity.ReadSelf),
		"readPreferences":    jsonOperation(a, identity.ReadPreferences),
		"updatePreferences":  jsonOperation(a, identity.UpdatePreferences),
		"changePassword":     jsonOperation(a, identity.ChangePassword),
		"verifySessionCSRF":  jsonOperation(a, identity.VerifySessionCSRF),
		"checkAuthorization": jsonOperation(a, identity.CheckAuthorization),
		"health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result, err := Health(r.Context(), a)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"ready": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result, err := Ready(r.Context(), a)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
	}
	handlers["listAccounts"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := a.identityContext(r.Context())
		op, _ := platform.OperationContextFrom(ctx)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		result, err := identity.ListAccounts(ctx, op, identity.ListAccountsInput{Page: identity.SessionPageRequest{Limit: limit, Cursor: r.URL.Query().Get("cursor")}})
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	handlers["readSessions"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := a.identityContext(r.Context())
		op, _ := platform.OperationContextFrom(ctx)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		result, err := identity.ReadSessions(ctx, op, identity.ReadSessionsInput{Page: identity.SessionPageRequest{Limit: limit, Cursor: r.URL.Query().Get("cursor")}})
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	handlers["revokeSession"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := a.identityContext(r.Context())
		op, _ := platform.OperationContextFrom(ctx)
		result, err := identity.RevokeSession(ctx, op, identity.RevokeSessionInput{SessionID: r.PathValue("sessionId")})
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	a.registerV2(handlers)
	return handlers
}

// jsonOperation 只解码平台已验证的输入；内部敏感结果仅返回已限定的portal服务。
func jsonOperation[I any, O any](a *Application, execute operationExecutor[I, O], bindInput ...func(*I, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := a.identityContext(r.Context())
		op, _ := platform.OperationContextFrom(ctx)
		var input I
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "身份输入不可解码"))
				return
			}
		}
		if len(bindInput) > 0 {
			bindInput[0](&input, r)
		}
		result, err := execute(ctx, op, input)
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

// Serve 与 Shutdown 实现 platform.Application，供进程模板调用。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Drain(ctx context.Context) error    { return Drain(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

var _ platform.BackgroundApplication = (*Application)(nil)

// registerHealthRoutes 匿名可访问；不暴露路径、端点或secret。
func (a *Application) registerHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		status, err := Health(r.Context(), a)
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		readiness, err := Ready(r.Context(), a)
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, readiness)
	})
}

// Serve 运行独立HTTP服务；ctx 取消触发的停止由进程模板调用 Shutdown
// 完成，Serve 只在监听结束后返回。
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

// BackgroundTasks 供统一进程模板启动和等待本域后台入口。
func (a *Application) BackgroundTasks() []platform.BackgroundTask {
	return append([]platform.BackgroundTask(nil), a.backgroundTasks...)
}

// InstallBackgroundTasks 只由完整启动装配调用；任一依赖缺失均拒绝装配。
func (a *Application) InstallBackgroundTasks(siteConfig platform.SiteConfigClient) error {
	if siteConfig == nil || !platform.ValidIdentifier(a.openedEpoch) {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "身份后台真实运行依赖未装配")
	}
	if len(a.backgroundTasks) != 0 {
		return platform.NewError(platform.CodeConflict, false, "身份后台入口重复装配")
	}
	tasks, err := identity.NewBackgroundTasks(a.store, siteConfig, a.openedEpoch, a.secretBox)
	if err != nil {
		return err
	}
	a.backgroundTasks = tasks
	return nil
}

// Drain 只停止HTTP入口并等待在途响应；后台任务先由上进程模板取消，
// 完成后再由Shutdown关闭数据库。
func Drain(ctx context.Context, app *Application) error {
	app.drainOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		err := app.httpServer.Shutdown(ctx)
		switch {
		case err == nil:
		case errors.Is(err, context.DeadlineExceeded):
			app.drainErr = platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全排空HTTP入口")
		default:
			app.drainErr = platform.NewError(platform.CodeInternal, false, "关闭HTTP入口失败")
		}
	})
	return app.drainErr
}

// Shutdown 停止接受新请求并等待在途响应到安全点；sync.Once 保证重复
// 调用返回同一次结果，deadline 未安全返回 DEADLINE_EXCEEDED。
func Shutdown(ctx context.Context, app *Application) error {
	app.shutdownOnce.Do(func() {
		if err := Drain(ctx, app); err != nil {
			app.shutdownErr = err
			return
		}
		if storeErr := app.store.Close(); storeErr != nil {
			app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭业务库失败")
			return
		}
		app.state.Store(stateClosed)
	})
	return app.shutdownErr
}

// Health 仅读本进程lifecycle，不查询远程依赖、模型或数据库。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app.state.Load() == stateClosed {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       http.StatusOK,
		Capabilities: []string{},
	}, nil
}

// Ready 确认本域配置、信任锚、业务库schema与决定日志完整且运行权威
// 可达；数据代次与打开时不一致返回503。就绪不代表维护已解除。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app.state.Load() == stateClosed {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭")
	}
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	view, err := lifecycle.ReadRuntime(ctx, app.runtime)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.openedEpoch != "" && view.DataEpoch != app.openedEpoch {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
	}
	initialized, err := app.store.HasSuperadmin(ctx)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	openedEpoch := string(app.openedEpoch)
	return gen.ServiceReadiness{
		Health:                health,
		Initialized:           initialized,
		JournalRecovered:      app.store.JournalRecovered,
		ActiveDataEpoch:       &openedEpoch,
		PublicRequestsAllowed: true,
		RuntimeRevision:       0,
	}, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, err error) {
	_ = problem.WriteProblem(w, problem.FromError(err, platform.NewIdentifier()))
}
