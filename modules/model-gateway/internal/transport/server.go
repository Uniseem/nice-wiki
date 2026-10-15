// Package transport 实现 model-gateway 的HTTP生命周期与匿名状态端点。
// 业务路由、worker、journal和费用流程在后续工作包接入。
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
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/model-gateway/internal/gen"
)

const (
	moduleName    = "model-gateway"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是本域启动后的内部容器，不能作为HTTP DTO序列化。
type Application struct {
	config              platform.ServiceConfig
	anchors             platform.TrustAnchors
	runtime             platform.SiteConfigClient
	activeDataEpoch     platform.Identifier
	journalRecovered    bool
	restoreControlReady bool
	businessReady       bool

	state        atomic.Int32
	serveStarted atomic.Bool
	serveActive  atomic.Bool
	shutdownOnce sync.Once
	shutdownErr  error
	httpServer   *http.Server
	workerCancel context.CancelFunc
}

// NewApplication 创建生命周期容器与匿名健康/就绪路由。只有在本域业务
// 依赖完成后，调用方才应把 businessReady 设为true并返回该容器。
func NewApplication(
	config platform.ServiceConfig,
	anchors platform.TrustAnchors,
	runtime platform.SiteConfigClient,
	activeDataEpoch platform.Identifier,
	journalRecovered, restoreControlReady, businessReady bool,
) *Application {
	application := &Application{
		config:              config,
		anchors:             anchors,
		runtime:             runtime,
		activeDataEpoch:     activeDataEpoch,
		journalRecovered:    journalRecovered,
		restoreControlReady: restoreControlReady,
		businessReady:       businessReady,
	}
	mux := http.NewServeMux()
	application.registerStatusRoutes(mux)
	application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return application
}

// Serve 与 Shutdown 实现 platform.Application，供进程模板调用。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

// Serve 启动单个HTTP监听；ctx取消时进入统一安全关闭流程。
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

// Shutdown 拒绝新入口并等待HTTP在途请求到安全点；未来业务资源挂到同一
// sync.Once关闭序列，不在当前工作包伪造journal、outbox或费用结算。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "Application未装配")
	}
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		if app.workerCancel != nil {
			app.workerCancel()
		}
		if err := app.httpServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				app.shutdownErr = platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全停止在途操作")
				return
			}
			app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭HTTP服务失败")
			return
		}
		app.state.Store(stateClosed)
	})
	return app.shutdownErr
}

// Health 仅读本进程状态，不查询远程依赖、模型或业务数据库。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed || !app.serveActive.Load() {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务循环不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       "ready",
		Capabilities: []string{},
	}, nil
}

// Ready 检查控制区、运行权威、恢复状态和当前数据代次；业务依赖未完成
// 时保持失败，不以空数据库或未绑定handler宣称就绪。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed || !app.serveActive.Load() {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务循环不可用")
	}
	if !app.restoreControlReady || !app.journalRecovered || !app.businessReady {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "model-gateway启动依赖尚未完成")
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
		ActiveDataEpoch:       stringPointer(activeEpoch),
		RuntimeRevision:       0,
		Maintenance:           view.Maintenance,
		PublicRequestsAllowed: !view.Maintenance && view.Phase == platform.PhaseRunning,
		Initialized:           false,
		JournalRecovered:      app.journalRecovered,
		SafePoint:             true,
	}, nil
}

func (a *Application) registerStatusRoutes(mux *http.ServeMux) {
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

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, err error) {
	_ = problem.WriteProblem(w, problem.FromError(err, platform.NewIdentifier()))
}
