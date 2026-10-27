// Package transport 实现 memory 域的 HTTP 生命周期：匿名健康/就绪、
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
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/memory/internal/gen"
)

const (
	moduleName    = "memory"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是 memory 装配完成后的内部容器，不可作为HTTP业务DTO。
type Application struct {
	config      platform.ServiceConfig
	anchors     platform.TrustAnchors
	runtime     platform.SiteConfigClient
	openedEpoch platform.Identifier
	state       atomic.Int32

	serveStarted atomic.Bool
	shutdownOnce sync.Once
	shutdownErr  error
	httpServer   *http.Server
	tasks        []platform.BackgroundTask
}

// SetBackgroundTasks 装配后台任务；统一进程模板在装配成功后启动，关闭前先停入口、等任务退出。
func (a *Application) SetBackgroundTasks(tasks []platform.BackgroundTask) { a.tasks = tasks }

// BackgroundTasks 返回已装配的后台任务。
func (a *Application) BackgroundTasks() []platform.BackgroundTask {
	return append([]platform.BackgroundTask(nil), a.tasks...)
}

// Drain 停止 HTTP 入口并等待在途响应（与 Shutdown 同一次关闭，记忆库由运行存储自己关）。
func (a *Application) Drain(ctx context.Context) error { return Shutdown(ctx, a) }

// NewApplication 组装匿名控制路由。当前业务 handler 尚未实现，
// BuildApplication 会在调用该构造器前按合同返回 DEPENDENCY_UNAVAILABLE。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, openedEpoch platform.Identifier, authorize func(context.Context, platform.OperationContext, string, string) error) (*Application, error) {
	application := &Application{
		config:      config,
		anchors:     anchors,
		runtime:     runtimeAuthority,
		openedEpoch: openedEpoch,
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "记忆 HTTP 规格不可读取")
	}
	callers := map[string][]string{}
	for id := range application.operationHandlers() {
		callers[id] = []string{"portal"}
	}
	callers["resolveContext"] = []string{"qa", "knowledge", "localization", "memory", "content"}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, OpenedDataEpoch: openedEpoch, Runtime: runtimeAuthority.ReadRuntimeState, AllowedCallers: callers, Authorize: authorize}, application.operationHandlers())
	if err != nil {
		return nil, err
	}
	application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return application, nil
}

// Serve 运行独立HTTP服务；进程模板在收到信号或恢复切换时调用
// Shutdown 完成安全关闭。
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

// Shutdown 停止接受新请求并等待在途响应到安全点；sync.Once 保证
// 重复调用返回同一次结果，deadline 未安全返回 DEADLINE_EXCEEDED。
func Shutdown(ctx context.Context, app *Application) error {
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
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

// Ready 确认本域控制区、信任锚与运行权威可读；业务schema/journal
// 尚未实现时不会由该函数伪造成功，BuildApplication也不会返回Application。
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
	var activeEpoch *string
	if app.openedEpoch != "" {
		value := string(app.openedEpoch)
		activeEpoch = &value
	}
	return gen.ServiceReadiness{
		Health:                health,
		ActiveDataEpoch:       activeEpoch,
		RuntimeRevision:       0,
		PublicRequestsAllowed: false,
		Initialized:           false,
		JournalRecovered:      false,
	}, nil
}

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

func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, err error) {
	_ = problem.WriteProblem(w, problem.FromError(err, platform.NewIdentifier()))
}
