// Package transport 实现 localization 域的 HTTP 生命周期：匿名健康/就绪、
// 优雅关闭与重复启动保护；业务路由随后续工作包接入。
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
	"nice-wiki/modules/localization/internal/gen"
)

const (
	moduleName    = "localization"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是 localization 装配完成后的内部容器，不序列化到 HTTP。
type Application struct {
	config              platform.ServiceConfig
	anchors             platform.TrustAnchors
	runtime             platform.SiteConfigClient
	openedEpoch         platform.Identifier
	journalRecovered    bool
	restoreControlReady bool
	state               atomic.Int32
	serveStarted        atomic.Bool
	shutdownOnce        sync.Once
	shutdownErr         error
	httpServer          *http.Server
}

// NewApplication 组装 HTTP 容器并注册匿名健康/就绪路由。业务路由与
// worker 必须等对应合同函数实现后再接入。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, openedEpoch platform.Identifier) *Application {
	application := &Application{
		config:              config,
		anchors:             anchors,
		runtime:             runtimeAuthority,
		openedEpoch:         openedEpoch,
		restoreControlReady: len(anchors.Services) > 0,
	}
	mux := http.NewServeMux()
	application.registerHealthRoutes(mux)
	application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return application
}

// Serve 运行独立 HTTP 服务；关闭由 platform 生命周期模板调用 Shutdown。
func (a *Application) Serve(ctx context.Context) error { return Serve(ctx, a) }

// Shutdown 停止入口并等待 HTTP 在途请求到达安全点。
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

// Serve 只启动本域 HTTP server；不启动未实现的翻译、模型或任务循环。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "HTTP服务尚未装配")
	}
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	_ = ctx
	err := app.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "HTTP服务不可恢复错误")
	}
	return nil
}

// Shutdown 使用sync.Once确保重复关闭返回同一结果；当前启动工作包只
// 关闭 HTTP 资源，业务数据库、journal、outbox 与 worker 待后续接入。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil {
		return platform.NewError(platform.CodeValidation, false, "应用句柄缺失")
	}
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		if app.httpServer == nil {
			app.state.Store(stateClosed)
			return
		}
		err := app.httpServer.Shutdown(ctx)
		if err == nil {
			app.state.Store(stateClosed)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			app.shutdownErr = platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全停止在途操作")
			return
		}
		app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭HTTP服务失败")
	})
	return app.shutdownErr
}

// Health 只读取本进程生命周期，不查询远程依赖、模型或业务数据库。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	_ = ctx
	if app == nil || app.state.Load() == stateClosed || app.httpServer == nil {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭或HTTP服务不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       "ready",
		Capabilities: []string{"health", "ready"},
	}, nil
}

// Ready 返回运行权威快照；未恢复本域 journal 或控制信任锚时不宣称就绪。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.state.Load() == stateClosed {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭")
	}
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.runtime == nil {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行权威不可用")
	}
	view, err := lifecycle.ReadRuntime(ctx, app.runtime)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.openedEpoch != "" && view.DataEpoch != app.openedEpoch {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
	}
	if !app.restoreControlReady || !app.journalRecovered {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域恢复资源尚未完成")
	}
	activeEpoch := string(view.DataEpoch)
	if app.openedEpoch != "" {
		activeEpoch = string(app.openedEpoch)
	}
	return gen.ServiceReadiness{
		Health:                health,
		ActiveDataEpoch:       &activeEpoch,
		Initialized:           false,
		JournalRecovered:      app.journalRecovered,
		Maintenance:           view.Maintenance,
		PublicRequestsAllowed: !view.Maintenance && view.Phase == platform.PhaseRunning,
		RuntimeRevision:       0,
		SafePoint:             true,
	}, nil
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
