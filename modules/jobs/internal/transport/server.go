// Package transport 实现 jobs 域的 HTTP 生命周期：匿名健康/就绪、
// 优雅关闭与重复启动保护；业务路由和后台循环随后续工作包接入。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/jobs/internal/gen"
)

const (
	moduleName    = "jobs"
	moduleVersion = "1"

	requiredBackgroundTaskCount = 4
)

var requiredBackgroundTasks = map[string]bool{
	"jobs.RequeueDue":          true,
	"jobs.ReapExpiredLeases":   true,
	"jobs.TickSchedules":       true,
	"jobs.PurgeExpiredHistory": true,
}

// ReadinessEvidence 由完整启动装配从 site-config 与真实本域状态联合读取；
// 缺任一权威证据时 Ready 必须返回不可用。
type ReadinessEvidence struct {
	Runtime               platform.RuntimeView
	Initialized           bool
	JournalRecovered      bool
	PublicRequestsAllowed bool
}

type ReadinessReader func(context.Context) (ReadinessEvidence, error)

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
	config                  platform.ServiceConfig
	anchors                 platform.TrustAnchors
	runtime                 platform.SiteConfigClient
	openedEpoch             platform.Identifier
	handlerBindingsComplete bool
	state                   atomic.Int32
	serveStarted            atomic.Bool
	drainOnce               sync.Once
	drainErr                error
	shutdownOnce            sync.Once
	shutdownErr             error
	httpServer              *http.Server
	resources               io.Closer
	backgroundTasks         []platform.BackgroundTask
	readiness               ReadinessReader
	serveActive             atomic.Bool
}

// NewApplication 建立进程生命周期容器；完整路由和资源只在装配全部
// OpenAPI handler 后通过 Install 移交给 Application。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, openedEpoch platform.Identifier) *Application {
	return &Application{
		config:      config,
		anchors:     anchors,
		runtime:     runtimeAuthority,
		openedEpoch: openedEpoch,
	}
}

// ControlHandlers 提供完整 OpenAPI 工厂所需的 health/ready 映射。
func (a *Application) ControlHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, err := Health(r.Context(), a)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, status)
		}),
		"ready": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			readiness, err := Ready(r.Context(), a)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, readiness)
		}),
	}
}

// Install 只接受完整服务路由；成功后由 Application 独占 HTTP 和存储资源。
func (a *Application) Install(handler http.Handler, resources io.Closer, backgroundTasks []platform.BackgroundTask, readiness ReadinessReader) error {
	if a == nil || handler == nil || resources == nil || readiness == nil || len(backgroundTasks) != requiredBackgroundTaskCount {
		return platform.NewError(platform.CodeValidation, false, "Application完整路由未装配")
	}
	taskNames := map[string]bool{}
	for _, task := range backgroundTasks {
		if !platform.ValidIdentifier(task.Name) || !requiredBackgroundTasks[task.Name] || task.Run == nil || taskNames[task.Name] {
			return platform.NewError(platform.CodeValidation, false, "Application后台任务未完整装配")
		}
		taskNames[task.Name] = true
	}
	if len(taskNames) != requiredBackgroundTaskCount {
		return platform.NewError(platform.CodeValidation, false, "Application后台任务未完整装配")
	}
	if a.httpServer != nil || a.resources != nil || len(a.backgroundTasks) != 0 {
		return platform.NewError(platform.CodeConflict, false, "Application重复安装运行资源")
	}
	a.resources = resources
	a.backgroundTasks = append([]platform.BackgroundTask(nil), backgroundTasks...)
	a.readiness = readiness
	a.handlerBindingsComplete = true
	a.httpServer = &http.Server{
		Addr:              a.config.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	a.state.Store(stateReady)
	return nil
}

// Serve 与 Shutdown 实现 platform.Application，供进程模板调用。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

// BackgroundTasks 返回装配期固定登记的后台入口。
func (a *Application) BackgroundTasks() []platform.BackgroundTask {
	if a == nil {
		return nil
	}
	return append([]platform.BackgroundTask(nil), a.backgroundTasks...)
}

// Drain 停止新 HTTP 并等待在途响应；不释放后台和存储，由统一进程模板
// 等待后台退出后再调用 Shutdown。
func (a *Application) Drain(ctx context.Context) error {
	if a == nil || a.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "Application未装配")
	}
	a.drainOnce.Do(func() {
		a.state.CompareAndSwap(stateReady, stateDraining)
		if err := a.httpServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				a.drainErr = platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全停止在途操作")
				return
			}
			a.drainErr = platform.NewError(platform.CodeInternal, false, "停止HTTP服务失败")
		}
	})
	return a.drainErr
}

// Serve 运行已通过完整路由和资源安装的HTTP服务。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeValidation, false, "Application未装配")
	}
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	app.serveActive.Store(true)
	defer app.serveActive.Store(false)
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
		if err := app.Drain(ctx); err != nil {
			app.shutdownErr = err
			return
		}
		if app.resources != nil {
			if err := app.resources.Close(); err != nil {
				app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭本域存储失败")
				return
			}
		}
		app.state.Store(stateClosed)
	})
	return app.shutdownErr
}

// Health 仅读本进程lifecycle，不查询远程依赖、模型或数据库。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed || !app.serveActive.Load() {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务循环不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       http.StatusOK,
		Capabilities: []string{},
	}, nil
}

// Ready 确认完整业务绑定、运行权威和当前打开的数据代次。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.httpServer == nil || app.state.Load() != stateReady || !app.serveActive.Load() {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务循环不可用")
	}
	if !app.handlerBindingsComplete || len(app.backgroundTasks) != requiredBackgroundTaskCount || app.resources == nil || app.readiness == nil {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域完整业务、后台或就绪证据尚未装配")
	}
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	evidence, err := app.readiness(ctx)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if evidence.Runtime.DataEpoch == "" || evidence.Runtime.Revision < 1 {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行权威代次或revision不可用")
	}
	if evidence.Runtime.DataEpoch != app.openedEpoch {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
	}
	if !evidence.JournalRecovered {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域journal尚未完成恢复")
	}
	var activeEpoch *string
	if evidence.Initialized {
		value := string(app.openedEpoch)
		activeEpoch = &value
	}
	publicRequestsAllowed := evidence.PublicRequestsAllowed
	if !evidence.Initialized {
		publicRequestsAllowed = false
	}
	return gen.ServiceReadiness{
		Health:                health,
		Initialized:           evidence.Initialized,
		JournalRecovered:      evidence.JournalRecovered,
		ActiveDataEpoch:       activeEpoch,
		PublicRequestsAllowed: publicRequestsAllowed,
		RuntimeRevision:       gen.Revision(evidence.Runtime.Revision),
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
