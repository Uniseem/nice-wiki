// Package transport 实现 analytics 域的HTTP生命周期、匿名健康/就绪和
// 未实现业务能力的失败关闭路由。
package transport

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/analytics/internal/gen"
)

const (
	moduleName    = "analytics"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是本域装配完成后的内部容器；不作为HTTP DTO序列化。
type Application struct {
	config       platform.ServiceConfig
	anchors      platform.TrustAnchors
	runtime      platform.SiteConfigClient
	serveStarted atomic.Bool
	state        atomic.Int32
	shutdownOnce sync.Once
	shutdownErr  error
	httpServer   *http.Server

	// 业务存储、journal、worker及恢复回执待后续工作包接入。
	journalRecovered bool
}

// NewApplication 绑定本域OpenAPI完整路由；业务路由在能力未实现期间
// 统一返回DEPENDENCY_UNAVAILABLE，不返回伪造统计或审计结果。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtime platform.SiteConfigClient) *Application {
	application := &Application{
		config:  config,
		anchors: anchors,
		runtime: runtime,
	}
	mux := http.NewServeMux()
	application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           gen.HandlerFromMux(&server{app: application}, mux),
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return application
}

// Serve 实现 platform.Application，供统一进程模板调用。
func (a *Application) Serve(ctx context.Context) error {
	return Serve(ctx, a)
}

// Shutdown 实现 platform.Application，供统一进程模板调用。
func (a *Application) Shutdown(ctx context.Context) error {
	return Shutdown(ctx, a)
}

// Serve 运行独立HTTP服务；ctx取消时触发同一条优雅关闭路径。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeInternal, false, "应用未完成装配")
	}
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// Shutdown 拒绝新请求并等待HTTP在途响应到安全点；sync.Once保证重复调用
// 返回同一次结果。本工作包没有业务DB或journal，相关资源由后续接入。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeInternal, false, "应用未完成装配")
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

// Health 仅读本进程生命周期，不查询远程依赖、模型或业务数据库。
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

// Ready 只在运行权威可读且本域journal已恢复时就绪；当前工作包没有
// analytics业务存储，因此按合同返回DEPENDENCY_UNAVAILABLE。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.state.Load() == stateClosed {
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
	if !app.journalRecovered {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "analytics本域存储与journal尚未实现")
	}
	return gen.ServiceReadiness{
		Health:                health,
		ActiveDataEpoch:       nil,
		RuntimeRevision:       0,
		Maintenance:           view.Maintenance,
		PublicRequestsAllowed: false,
		Initialized:           false,
		JournalRecovered:      app.journalRecovered,
		SafePoint:             true,
	}, nil
}

// 未接的新增协议保持生成的未实现响应；完整服务仍不可就绪。
type server struct {
	gen.UnimplementedServer
	app *Application
}

func (s *server) Health(w http.ResponseWriter, r *http.Request) {
	status, err := Health(r.Context(), s.app)
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	_ = gen.Health200JSONResponse(status).VisitHealthResponse(w)
}

func (s *server) Ready(w http.ResponseWriter, r *http.Request) {
	readiness, err := Ready(r.Context(), s.app)
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	_ = gen.Ready200JSONResponse(readiness).VisitReadyResponse(w)
}

func (s *server) QueryAudits(w http.ResponseWriter, r *http.Request, params gen.QueryAuditsParams) {
	writeUnavailable(w, r)
}

func (s *server) AppendAudit(w http.ResponseWriter, r *http.Request, params gen.AppendAuditParams) {
	writeUnavailable(w, r)
}

func (s *server) ReadAudit(w http.ResponseWriter, r *http.Request, auditID string, params gen.ReadAuditParams) {
	writeUnavailable(w, r)
}

func (s *server) AcquireBarrier(w http.ResponseWriter, r *http.Request, params gen.AcquireBarrierParams) {
	writeUnavailable(w, r)
}

func (s *server) ReleaseBarrier(w http.ResponseWriter, r *http.Request, barrierID string, params gen.ReleaseBarrierParams) {
	writeUnavailable(w, r)
}

func (s *server) IngestBeacon(w http.ResponseWriter, r *http.Request, params gen.IngestBeaconParams) {
	writeUnavailable(w, r)
}

func (s *server) RecordCrawlerAccess(w http.ResponseWriter, r *http.Request, params gen.RecordCrawlerAccessParams) {
	writeUnavailable(w, r)
}

func (s *server) DiscardSnapshot(w http.ResponseWriter, r *http.Request, params gen.DiscardSnapshotParams) {
	writeUnavailable(w, r)
}

func (s *server) ExportSnapshot(w http.ResponseWriter, r *http.Request, params gen.ExportSnapshotParams) {
	writeUnavailable(w, r)
}

func (s *server) StageSnapshot(w http.ResponseWriter, r *http.Request, params gen.StageSnapshotParams) {
	writeUnavailable(w, r)
}

func (s *server) QueryContentGaps(w http.ResponseWriter, r *http.Request, params gen.QueryContentGapsParams) {
	writeUnavailable(w, r)
}

func (s *server) QueryPageStatistics(w http.ResponseWriter, r *http.Request, params gen.QueryPageStatisticsParams) {
	writeUnavailable(w, r)
}

func (s *server) QueryUsageStatistics(w http.ResponseWriter, r *http.Request, params gen.QueryUsageStatisticsParams) {
	writeUnavailable(w, r)
}

func (s *server) CreateExport(w http.ResponseWriter, r *http.Request, params gen.CreateExportParams) {
	writeUnavailable(w, r)
}

func (s *server) ReconcileUsage(w http.ResponseWriter, r *http.Request, params gen.ReconcileUsageParams) {
	writeUnavailable(w, r)
}

func writeUnavailable(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, platform.NewError(platform.CodeDependencyUnavailable, true, "analytics业务能力尚未实现"))
}

func writeProblem(w http.ResponseWriter, r *http.Request, err error) {
	requestID := platform.Identifier(r.Header.Get(platform.HeaderRequestID))
	if requestID == "" {
		requestID = platform.NewIdentifier()
	}
	_ = problem.WriteProblem(w, problem.FromError(err, requestID))
}
