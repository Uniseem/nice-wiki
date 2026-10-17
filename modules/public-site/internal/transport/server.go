// Package transport 实现 public-site 的 HTTP 生命周期与匿名状态端点。
// 未绑定的业务路由不在这里用占位结果冒充成功。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	publicsite "nice-wiki/modules/public-site/internal"
	"nice-wiki/modules/public-site/internal/gen"
)

const (
	moduleName    = "public-site"
	moduleVersion = "1"
	readyBudget   = 1500 * time.Millisecond
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是本域内部运行容器，不作为 HTTP DTO 序列化。
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

// NewApplication 创建尚未监听的容器。businessReady 只有全部处理函数、
// journal 与后台循环都接入后才能为 true。
func NewApplication(config platform.ServiceConfig, runtime platform.SiteConfigClient, openedEpoch platform.Identifier, journalRecovered, restoreControlReady, businessReady bool) *Application {
	application := &Application{
		config:              config,
		runtime:             runtime,
		openedEpoch:         openedEpoch,
		journalRecovered:    journalRecovered,
		restoreControlReady: restoreControlReady,
		businessReady:       businessReady,
	}
	application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return application
}

// Serve 与 Shutdown 实现 platform.Application。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

// Serve 启动单个 HTTP 监听。运行状态轮询与退出码 75 由平台 RunService 负责。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeInternal, false, "公开站点应用未装配")
	}
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
		defer cancel()
		_ = Shutdown(shutdownCtx, app)
	}()
	err := app.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return platform.NewError(platform.CodeInternal, false, "HTTP服务不可恢复错误")
	}
	return nil
}

// Shutdown 拒绝新入口并等待在途 HTTP 响应。重复调用返回同一次结果。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeInternal, false, "公开站点应用未装配")
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

// Health 只读本进程生命周期，不查询远程依赖、模型或数据库。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	_ = ctx
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

// Ready 在 journal、业务路由或数据代次未对齐时失败关闭。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭或服务循环不可用")
	}
	if !app.journalRecovered || !app.restoreControlReady || !app.businessReady {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "公开站点journal或业务路由尚未就绪")
	}
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	readCtx, cancel := context.WithTimeout(ctx, readyBudget)
	defer cancel()
	view, err := lifecycle.ReadRuntime(readCtx, app.runtime)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.openedEpoch != "" && view.DataEpoch != app.openedEpoch {
		if !(view.Maintenance && view.TargetDataEpoch == app.openedEpoch) {
			return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
		}
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
		Initialized:           app.openedEpoch != "",
		JournalRecovered:      app.journalRecovered,
		SafePoint:             true,
	}, nil
}

// BindPublishedLink 把 resolvePublishedLink、健康和就绪接到真实函数，并核对规格里的每个操作都有处理函数。
// 缺任何一个操作时返回校验错误，不安装部分路由。
func BindPublishedLink(app *Application, anchors platform.TrustAnchors, directory publicsite.LinkDirectory) error {
	if app == nil || app.runtime == nil || directory.SiteConfig == nil || directory.Content == nil || directory.Localization == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, true, "公开链接权威依赖未装配")
	}
	handlers := map[string]http.Handler{
		"health":               healthHandler(app),
		"ready":                readyHandler(app),
		"resolvePublishedLink": publishedLinkHandler(directory),
	}
	subset, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "公开站点接口规格不可读")
	}
	for path := range subset.Paths.Map() {
		if path != "/healthz" && path != "/readyz" && path != "/v1/public-links:resolve" {
			subset.Paths.Delete(path)
		}
	}
	if _, err = apivalidate.NewServiceHandler(subset, platform.ServiceBoundaryConfig{
		Self:                moduleName,
		Anchors:             anchors,
		Runtime:             app.runtime.ReadRuntimeState,
		OpenedDataEpoch:     app.openedEpoch,
		AllowedCallers:      map[string][]string{"resolvePublishedLink": {"site-config"}},
		ServiceActorCallers: map[string][]string{"resolvePublishedLink": {"site-config"}},
	}, handlers); err != nil {
		return err
	}
	full, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "公开站点接口规格不可读")
	}
	return requireAllHandlers(full, handlers)
}

func requireAllHandlers(spec *openapi3.T, handlers map[string]http.Handler) error {
	if spec == nil || spec.Paths == nil {
		return platform.NewError(platform.CodeIntegrityError, false, "公开站点接口规格不可读")
	}
	for _, item := range spec.Paths.Map() {
		if item == nil {
			continue
		}
		for _, operation := range item.Operations() {
			if operation == nil || operation.OperationID == "" || handlers[operation.OperationID] == nil {
				return platform.NewError(platform.CodeValidation, false, "缺路由绑定")
			}
		}
	}
	return nil
}

func healthHandler(app *Application) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := Health(r.Context(), app)
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	})
}

func readyHandler(app *Application) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		readiness, err := Ready(r.Context(), app)
		if err != nil {
			writeProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, readiness)
	})
}

func publishedLinkHandler(directory publicsite.LinkDirectory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := platform.OperationContextFrom(r.Context())
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input gen.ResolvePublishedLinkInput
		if err := decoder.Decode(&input); err != nil || decoder.More() {
			writeProblem(w, platform.NewError(platform.CodeValidation, false, "请求不符合接口规格"))
			return
		}
		result, err := publicsite.ResolvePublishedLink(publicsite.WithLinkDirectory(r.Context(), directory), op, input)
		if err != nil {
			writeProblem(w, err)
			return
		}
		raw, err := result.MarshalJSON()
		if err != nil {
			writeProblem(w, platform.NewError(platform.CodeInternal, false, "公开链接结果无法编码"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, err error) {
	requestID := platform.NewIdentifier()
	_ = problem.WriteProblem(w, problem.FromError(err, requestID))
}
