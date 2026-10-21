// Package transport 实现 analytics 域的HTTP生命周期、匿名健康/就绪和
// 未实现业务能力的失败关闭路由。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"nice-wiki/libs/platform/apivalidate"
	analytics "nice-wiki/modules/analytics/internal"
	"strconv"
	"strings"
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
	backgroundCancel context.CancelFunc
	backgroundDone   chan struct{}
	storage          *analytics.Runtime
	journalRecovered bool
}

// NewApplication 绑定本域OpenAPI完整路由；业务路由在能力未实现期间
// 统一返回DEPENDENCY_UNAVAILABLE，不返回伪造统计或审计结果。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtime platform.SiteConfigClient, storage *analytics.Runtime, identity *gen.IdentityClient) (*Application, error) {
	application := &Application{config: config, anchors: anchors, runtime: runtime, storage: storage, journalRecovered: storage != nil}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "统计接口规格不可读")
	}
	mux := apivalidate.NewPatternMux()
	routed := gen.HandlerFromMux(&server{app: application}, mux)
	handlers := map[string]http.Handler{}
	callers := map[string][]string{}
	services := map[string][]string{}
	maintenance := map[string]bool{}
	for _, path := range spec.Paths.Map() {
		for _, operation := range path.Operations() {
			// 内嵌规格的 operationId 是大写开头；先归一成小写开头，否则平台归一键时会和下面单独放开的条目
			// 撞成同一个键，谁生效取决于 map 遍历顺序（重启后随机拒绝审计、恢复归档等服务调用）。
			id := strings.ToLower(operation.OperationID[:1]) + operation.OperationID[1:]
			handlers[id] = routed
			if id != "health" && id != "ready" {
				callers[id] = []string{"portal"}
			}
		}
	}
	for _, id := range []string{"acquireBarrier", "releaseBarrier", "exportSnapshot", "stageSnapshot", "discardSnapshot"} {
		callers[id] = []string{"data-protection"}
		services[id] = []string{"data-protection"}
		maintenance[id] = true
	}
	callers["ingestTrustedBeacon"] = []string{"portal"}
	services["ingestTrustedBeacon"] = callers["ingestTrustedBeacon"]
	callers["recordCrawlerAccess"] = []string{"public-site"}
	services["recordCrawlerAccess"] = callers["recordCrawlerAccess"]
	callers["appendAudit"] = []string{"identity", "site-config", "jobs", "content", "media", "public-site", "model-gateway", "knowledge", "qa", "localization", "memory", "traffic-guard", "analytics", "data-protection", "portal"}
	services["appendAudit"] = callers["appendAudit"]
	for _, id := range []string{"queryPageStatistics", "queryUsageStatistics", "queryAudits", "readAudit", "queryContentGaps", "downloadStatisticsExport", "downloadAuditExport"} {
		maintenance[id] = true
	}
	protected, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, Runtime: func(ctx context.Context) (platform.RuntimeView, error) { return lifecycle.ReadRuntime(ctx, runtime) }, OpenedDataEpoch: storage.Epoch, AllowedCallers: callers, ServiceActorCallers: services, MaintenanceOperations: maintenance, OperationBodyLimits: map[string]int64{"ingestBeacon": 2048, "ingestTrustedBeacon": 4096}, Authorize: apivalidate.NewIdentityAuthorizer(identity)}, handlers)
	if err != nil {
		return nil, err
	}
	application.httpServer = &http.Server{Addr: config.Listen, Handler: protected, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 64 << 10}
	return application, nil
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
	background, cancelBackground := context.WithCancel(ctx)
	app.backgroundCancel = cancelBackground
	app.backgroundDone = make(chan struct{})
	go func() { defer close(app.backgroundDone); _ = analytics.RunPageBackground(background, app.storage) }()
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
		if app.backgroundCancel != nil {
			app.backgroundCancel()
			select {
			case <-app.backgroundDone:
			case <-ctx.Done():
				app.shutdownErr = platform.NewError(platform.CodeDeadlineExceeded, true, "后台任务未能在期限内停止")
				return
			}
		}
		if err := analytics.DrainBeacons(ctx, app.storage); err != nil {
			app.shutdownErr = err
			return
		}
		if app.storage != nil {
			if err := app.storage.Close(); err != nil {
				app.shutdownErr = err
				return
			}
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
// 完整快照/journal恢复，因此按合同返回DEPENDENCY_UNAVAILABLE。
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

type server struct {
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
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	limit := int64(20)
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, _ = strconv.ParseInt(value, 10, 64)
	}
	result, err := analytics.QueryAudits(analytics.WithRuntime(r.Context(), s.app.storage), op, analytics.QueryAuditsInput{Cursor: r.URL.Query().Get("cursor"), Limit: limit, Action: r.URL.Query().Get("action"), From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to"), Actor: op.Actor, DataEpoch: op.DataEpoch})
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *server) AppendAudit(w http.ResponseWriter, r *http.Request, params gen.AppendAuditParams) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	var body struct {
		AuditID           string  `json:"auditId"`
		ActorID           *string `json:"actorId"`
		Action            string  `json:"action"`
		ResourceID        string  `json:"resourceId"`
		ResultCode        string  `json:"resultCode"`
		Metadata          string  `json:"metadata"`
		ActorKind         *string `json:"actorKind"`
		OccurredAt        *string `json:"occurredAt"`
		SourceOperationID *string `json:"sourceOperationId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, r, platform.NewError(platform.CodeValidation, false, "审计正文不可读"))
		return
	}
	actor := op.Actor.SubjectID
	if body.ActorID != nil {
		actor = *body.ActorID
	}
	result, err := analytics.AppendAudit(analytics.WithRuntime(r.Context(), s.app.storage), op, analytics.AppendAuditInput{AuditID: body.AuditID, ActorID: actor, Action: body.Action, ResourceID: body.ResourceID, ResultCode: body.ResultCode, Metadata: body.Metadata, ActorKind: body.ActorKind, OccurredAt: body.OccurredAt, SourceOperationID: body.SourceOperationID, DataEpoch: op.DataEpoch})
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *server) ReadAudit(w http.ResponseWriter, r *http.Request, auditID string, params gen.ReadAuditParams) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	result, err := analytics.ReadAudit(analytics.WithRuntime(r.Context(), s.app.storage), op, analytics.ReadAuditInput{AuditID: auditID, Actor: op.Actor, DataEpoch: op.DataEpoch})
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
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
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	var input analytics.RecordCrawlerAccessInput
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeProblem(w, r, platform.NewError(platform.CodeValidation, false, "爬虫计数正文不可读"))
		return
	}
	input.DataEpoch = op.DataEpoch
	result, err := analytics.RecordCrawlerAccess(analytics.WithRuntime(r.Context(), s.app.storage), op, input)
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

// IngestTrustedBeacon的全部正文由portal服务签名；本域不读取代理链或原始UA头。
func (s *server) IngestTrustedBeacon(w http.ResponseWriter, r *http.Request, params gen.IngestTrustedBeaconParams) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	var input analytics.IngestBeaconInput
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeProblem(w, r, platform.NewError(platform.CodeValidation, false, "可信访问正文不可读"))
		return
	}
	if _, err := analytics.IngestBeacon(analytics.WithRuntime(r.Context(), s.app.storage), op, input); err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
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
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	q := r.URL.Query()
	result, err := analytics.QueryContentGaps(analytics.WithRuntime(r.Context(), s.app.storage), op, analytics.QueryContentGapsInput{From: q.Get("from"), To: q.Get("to"), Actor: op.Actor, DataEpoch: op.DataEpoch})
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *server) QueryPageStatistics(w http.ResponseWriter, r *http.Request, params gen.QueryPageStatisticsParams) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	q := r.URL.Query()
	result, err := analytics.QueryPageStatistics(analytics.WithRuntime(r.Context(), s.app.storage), op, analytics.QueryPageStatisticsInput{From: q.Get("from"), To: q.Get("to"), Locale: q.Get("locale"), DocumentID: q.Get("documentId"), Granularity: q.Get("granularity"), Actor: op.Actor, DataEpoch: op.DataEpoch})
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *server) QueryUsageStatistics(w http.ResponseWriter, r *http.Request, params gen.QueryUsageStatisticsParams) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	q := r.URL.Query()
	result, err := analytics.QueryUsageStatistics(analytics.WithRuntime(r.Context(), s.app.storage), op, analytics.QueryUsageStatisticsInput{From: q.Get("from"), To: q.Get("to"), CostClass: q.Get("costClass"), Actor: op.Actor, DataEpoch: op.DataEpoch})
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *server) CreateExport(w http.ResponseWriter, r *http.Request, params gen.CreateExportParams) {
	s.createExport(w, r, false)
}
func (s *server) CreateAuditExport(w http.ResponseWriter, r *http.Request, params gen.CreateAuditExportParams) {
	s.createExport(w, r, true)
}
func (s *server) createExport(w http.ResponseWriter, r *http.Request, audits bool) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	var body struct {
		Kind string `json:"kind"`
		From string `json:"from"`
		To   string `json:"to"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeProblem(w, r, platform.NewError(platform.CodeValidation, false, "导出条件不可读"))
		return
	}
	if (body.Kind == "audits") != audits {
		writeProblem(w, r, platform.NewError(platform.CodeForbidden, false, "导出类型与当前权限路由不同"))
		return
	}
	result, err := analytics.CreateExport(analytics.WithRuntime(r.Context(), s.app.storage), op, analytics.CreateExportInput{Kind: body.Kind, From: body.From, To: body.To, Actor: op.Actor, DataEpoch: op.DataEpoch})
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(result)
}
func (s *server) DownloadStatisticsExport(w http.ResponseWriter, r *http.Request, exportID string, params gen.DownloadStatisticsExportParams) {
	s.downloadExport(w, r, exportID, false)
}
func (s *server) DownloadAuditExport(w http.ResponseWriter, r *http.Request, exportID string, params gen.DownloadAuditExportParams) {
	s.downloadExport(w, r, exportID, true)
}
func (s *server) downloadExport(w http.ResponseWriter, r *http.Request, id string, audits bool) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok {
		writeProblem(w, r, platform.NewError(platform.CodeUnauthenticated, false, "缺少可信上下文"))
		return
	}
	kind, err := analytics.ExportKind(r.Context(), s.app.storage, id)
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	if (kind == "audits") != audits {
		writeProblem(w, r, platform.NewError(platform.CodeForbidden, false, "导出类型与当前权限路由不同"))
		return
	}
	file, ref, err := analytics.OpenExport(analytics.WithRuntime(r.Context(), s.app.storage), op, id, kind)
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(ref.ByteSize, 10))
	w.Header().Set("Content-Disposition", `attachment; filename="analytics.csv"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, file)
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

// ReadSnapshotArchive 等待真实归档manager，明确失败关闭。
func (s *server) ReadSnapshotArchive(w http.ResponseWriter, r *http.Request, snapshotID gen.Identifier, params gen.ReadSnapshotArchiveParams) {
	writeUnavailable(w, r)
}
