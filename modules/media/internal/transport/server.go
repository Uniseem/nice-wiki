// Package transport 实现 media 域的 HTTP 生命周期、匿名健康/就绪和
// 优雅关闭。业务路由待对应工作包实现后接入。
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
	"nice-wiki/modules/media/internal/gen"
)

const (
	moduleName    = "media"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是 media 装配完成后的内部容器，不作为HTTP DTO序列化。
type Application struct {
	config              platform.ServiceConfig
	runtime             platform.SiteConfigClient
	activeDataEpoch     platform.Identifier
	journalRecovered    bool
	restoreControlReady bool
	state               atomic.Int32
	serveStarted        atomic.Bool
	shutdownOnce        sync.Once
	shutdownErr         error
	workerCancel        context.CancelFunc
	httpServer          *http.Server
}

// NewApplication 创建只含启动控制路由的 HTTP 容器。业务依赖未接入时
// journalRecovered 应保持 false，Ready 会按合同返回 DEPENDENCY_UNAVAILABLE。
func NewApplication(
	config platform.ServiceConfig,
	runtimeAuthority platform.SiteConfigClient,
	activeDataEpoch platform.Identifier,
	journalRecovered bool,
	restoreControlReady bool,
) *Application {
	application := &Application{
		config:              config,
		runtime:             runtimeAuthority,
		activeDataEpoch:     activeDataEpoch,
		journalRecovered:    journalRecovered,
		restoreControlReady: restoreControlReady,
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

// Serve 与 Shutdown 实现平台服务生命周期接口。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
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

// Serve 启动本域 HTTP server；平台进程模板负责在信号或恢复切换时调用
// Shutdown，重复启动不打开第二个监听器。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil || ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "HTTP应用句柄不完整")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if app.state.Load() != stateReady {
		return platform.NewError(platform.CodeConflict, false, "Application已不在可启动状态")
	}
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

// Shutdown 停止入口、取消本域worker并等待HTTP在途请求到达安全点。
// 业务journal/outbox句柄在对应工作包接入后追加到同一sync.Once路径。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil || ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "HTTP应用句柄不完整")
	}
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		if app.workerCancel != nil {
			app.workerCancel()
		}
		if err := app.httpServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
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

// Health 仅读本进程状态，不查询远程依赖、模型或本域业务存储。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       http.StatusOK,
		Capabilities: []string{},
	}, nil
}

// Ready 确认运行权威与控制区安全状态；本工作包未接入业务journal时，
// 明确返回 DEPENDENCY_UNAVAILABLE，不以空状态冒充就绪。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.httpServer == nil || app.state.Load() != stateReady {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务尚未处于就绪生命周期")
	}
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if !app.restoreControlReady || !app.journalRecovered {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域控制区或业务journal尚未恢复")
	}
	view, err := lifecycle.ReadRuntime(ctx, app.runtime)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.activeDataEpoch != "" && view.DataEpoch != app.activeDataEpoch {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
	}
	var epoch *string
	if app.activeDataEpoch != "" {
		value := string(app.activeDataEpoch)
		epoch = &value
	}
	return gen.ServiceReadiness{
		Health:                health,
		ActiveDataEpoch:       epoch,
		Initialized:           false,
		JournalRecovered:      app.journalRecovered,
		Maintenance:           view.Maintenance,
		PublicRequestsAllowed: !view.Maintenance && view.Phase == platform.PhaseRunning,
		RuntimeRevision:       0,
		SafePoint:             true,
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
