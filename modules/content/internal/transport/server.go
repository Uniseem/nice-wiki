// Package transport 实现 content 域的 HTTP 生命周期与匿名状态端点。
// 业务路由待本域 x-handler 工作包完成后接入；此阶段不返回伪成功。
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
	"nice-wiki/modules/content/internal/gen"
)

const (
	moduleName    = "content"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是本域内部运行容器，不作为HTTP DTO序列化。
type Application struct {
	config              platform.ServiceConfig
	runtime             platform.SiteConfigClient
	openedEpoch         platform.Identifier
	journalRecovered    bool
	restoreControlReady bool
	businessReady       bool
	state               atomic.Int32
	serveStarted        atomic.Bool
	shutdownOnce        sync.Once
	shutdownErr         error
	httpServer          *http.Server
}

// NewApplication 创建仅含匿名状态路由的容器。businessReady 必须由后续
// 工作包在所有本域 handler、存储和后台任务均接入后置为true。
func NewApplication(config platform.ServiceConfig, runtime platform.SiteConfigClient, openedEpoch platform.Identifier, journalRecovered, restoreControlReady, businessReady bool) *Application {
	application := &Application{
		config:              config,
		runtime:             runtime,
		openedEpoch:         openedEpoch,
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

// Serve 启动单个HTTP监听；ctx取消时请求安全关闭。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "content应用未装配")
	}
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	shutdownDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = Shutdown(shutdownCtx, app)
			cancel()
		case <-shutdownDone:
		}
	}()

	err := app.httpServer.ListenAndServe()
	close(shutdownDone)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "HTTP服务不可恢复错误")
	}
	return nil
}

// Shutdown 拒绝新入口并等待HTTP在途请求到安全点。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "content应用未装配")
	}
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
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

// Health 仅读本进程生命周期，不查询远程依赖、模型或业务存储。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭或服务循环不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       http.StatusOK,
		Capabilities: []string{},
	}, nil
}

// Ready 检查控制区、运行权威和当前数据代次；业务依赖未完成时不返回就绪。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭或服务循环不可用")
	}
	if !app.journalRecovered || !app.restoreControlReady || !app.businessReady {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "content启动依赖尚未完成")
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
		Maintenance:           view.Maintenance,
		PublicRequestsAllowed: !view.Maintenance && view.Phase == platform.PhaseRunning,
		Initialized:           true,
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, err error) {
	_ = problem.WriteProblem(w, problem.FromError(err, platform.NewIdentifier()))
}
