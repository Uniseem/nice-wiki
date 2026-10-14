// Package transport 实现 jobs 域的 HTTP 生命周期：匿名健康/就绪、
// 优雅关闭与重复启动保护；业务路由和后台循环随后续工作包接入。
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
	"nice-wiki/modules/jobs/internal/gen"
)

const (
	moduleName    = "jobs"
	moduleVersion = "1"
)

// lifecycle 状态：ready 接受请求，draining 拒新入口，closed 资源已释放。
const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是 jobs 装配完成后的内部容器（x-internal，不序列化）。
// handlerBindingsComplete 在所有本域 OpenAPI handler 与后台任务落地前
// 保持false，Ready不会把不完整装配报告成可用。
type Application struct {
	config                 platform.ServiceConfig
	anchors                platform.TrustAnchors
	runtime                platform.SiteConfigClient
	openedEpoch            platform.Identifier
	handlerBindingsComplete bool
	state                  atomic.Int32
	serveStarted           atomic.Bool
	shutdownOnce           sync.Once
	shutdownErr            error
	httpServer             *http.Server
}

// NewApplication 组装仅包含控制面路由的 HTTP 容器；业务 handler 全部
// 接入后由 BuildApplication 传入完整绑定状态。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, openedEpoch platform.Identifier) *Application {
	application := &Application{
		config:       config,
		anchors:      anchors,
		runtime:      runtimeAuthority,
		openedEpoch:  openedEpoch,
		httpServer:   &http.Server{},
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

// Serve 与 Shutdown 实现 platform.Application，供进程模板调用。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

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

// Serve 运行独立HTTP服务；worker和业务路由在对应工作包完成后接入。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "Application未装配")
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

// Shutdown 停止接受新请求并等待在途响应到安全点；sync.Once 保证重复
// 调用返回同一次结果，deadline 未安全返回 DEADLINE_EXCEEDED。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "Application未装配")
	}
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		err := app.httpServer.Shutdown(ctx)
		if err == nil {
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

// Health 仅读本进程lifecycle，不查询远程依赖、模型或数据库。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.state.Load() == stateClosed {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       http.StatusOK,
		Capabilities: []string{},
	}, nil
}

// Ready 确认本域配置、信任锚、运行权威与完整业务绑定；当前任务/事件
// 处理函数尚未实现，因此按合同返回 DEPENDENCY_UNAVAILABLE。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.state.Load() == stateClosed {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭")
	}
	if !app.handlerBindingsComplete {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域业务handler尚未装配")
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
	activeEpoch := string(app.openedEpoch)
	return gen.ServiceReadiness{
		Health:                health,
		Initialized:           false,
		JournalRecovered:      true,
		ActiveDataEpoch:       &activeEpoch,
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
