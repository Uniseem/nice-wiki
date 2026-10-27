// Package transport 实现 traffic-guard 的 HTTP 生命周期和匿名健康/就绪
// 路由；业务路由在对应工作包实现后再接入。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/traffic-guard/internal/gen"
)

const (
	moduleName    = "traffic-guard"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是启动装配后的内部容器，不能作为 HTTP DTO 序列化。
type Application struct {
	config              platform.ServiceConfig
	anchors             platform.TrustAnchors
	runtime             platform.SiteConfigClient
	activeDataEpoch     platform.Identifier
	registeredHandlers  []string
	workerFunctionIDs   []string
	journalRecovered    bool
	restoreControlReady bool

	state        atomic.Int32
	serveStarted atomic.Bool
	serveActive  atomic.Bool
	shutdownOnce sync.Once
	shutdownErr  error
	httpServer   *http.Server
	workerCancel context.CancelFunc
	router       *apivalidate.PathRouter
	closers      []func()
}

// NewApplication 只创建生命周期容器与匿名探针。调用方必须在所有业务
// handler、存储恢复与后台循环具备后才可返回该容器。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtime platform.SiteConfigClient, activeDataEpoch platform.Identifier) *Application {
	application := &Application{
		config:              config,
		anchors:             anchors,
		runtime:             runtime,
		activeDataEpoch:     activeDataEpoch,
		registeredHandlers:  []string{"traffic-guard.Health", "traffic-guard.Ready"},
		workerFunctionIDs:   nil,
		journalRecovered:    false,
		restoreControlReady: true,
	}
	mux := http.NewServeMux()
	application.registerHealthRoutes(mux)
	application.router = apivalidate.NewPathRouter(mux)
	application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           application.router,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return application
}

// Mount 把业务处理器挂到规格里除健康/就绪外的每个路由，并标记 journal 已恢复；
// closer 在关闭时释放存储等资源。
func (a *Application) Mount(handler http.Handler, paths map[string][]string, closer func()) {
	for path, methods := range paths {
		if path == "/healthz" || path == "/readyz" {
			continue
		}
		for _, method := range methods {
			a.router.Handle(method, path, handler)
		}
	}
	a.journalRecovered = true
	a.closers = append(a.closers, closer)
}

// Serve 实现 platform.Application，运行单一 HTTP listener。
func (a *Application) Serve(ctx context.Context) error { return Serve(ctx, a) }

// Shutdown 实现 platform.Application，保证重复关闭复用同一结果。
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

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

// Serve 启动 HTTP 服务；上下文取消时主动进入 Shutdown，监听结束时只
// 把 ErrServerClosed 视为正常关闭。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "Application未装配")
	}
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	app.serveActive.Store(true)
	defer app.serveActive.Store(false)

	serverErr := make(chan error, 1)
	go func() { serverErr <- app.httpServer.ListenAndServe() }()
	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return platform.NewError(platform.CodeInternal, false, "HTTP服务不可恢复错误")
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return Shutdown(shutdownCtx, app)
	}
}

// Shutdown 停止新入口并关闭 HTTP 资源；业务 worker、journal 和数据库
// 在对应工作包接入后挂到同一安全点，此处不假造已完成的持久化。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "Application未装配")
	}
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		if app.workerCancel != nil {
			app.workerCancel()
		}
		err := app.httpServer.Shutdown(ctx)
		if err == nil {
			for _, closer := range app.closers {
				closer()
			}
			app.state.Store(stateClosed)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			app.shutdownErr = platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全停止在途操作")
			return
		}
		app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭HTTP服务失败")
	})
	return app.shutdownErr
}

// Health 仅读本进程状态，不查询远程依赖、模型或业务数据库。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.state.Load() == stateClosed || !app.serveActive.Load() {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务循环不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       "ready",
		Capabilities: []string{},
	}, nil
}

// Ready 检查运行权威和本域控制状态；缺少业务存储/journal 时保持失败，
// 不以空数据宣称就绪。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.state.Load() == stateClosed || !app.serveActive.Load() {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务循环不可用")
	}
	if !app.restoreControlReady || !app.journalRecovered {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域恢复控制或journal尚未就绪")
	}
	view, err := lifecycle.ReadRuntime(ctx, app.runtime)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.activeDataEpoch != "" && view.DataEpoch != app.activeDataEpoch {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
	}
	activeEpoch := app.activeDataEpoch
	if activeEpoch == "" {
		activeEpoch = view.DataEpoch
	}
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	return gen.ServiceReadiness{
		Health:                health,
		ActiveDataEpoch:       &activeEpoch,
		RuntimeRevision:       0,
		PublicRequestsAllowed: true,
		Initialized:           false,
		JournalRecovered:      app.journalRecovered,
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
