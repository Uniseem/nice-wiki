// Package transport 提供已完成用例的 HTTP 映射；完整应用仍须绑定全部 OpenAPI 操作。
package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/knowledge/internal/admin"
	"nice-wiki/modules/knowledge/internal/build"
	indexconfig "nice-wiki/modules/knowledge/internal/config"
	"nice-wiki/modules/knowledge/internal/gen"
	domainruntime "nice-wiki/modules/knowledge/internal/runtime"
	"nice-wiki/modules/knowledge/internal/search"
)

// NewOperationHandlers 返回真实已完成处理函数，交给平台 NewServiceHandler 完整校验。
// bind 由服务装配注入存储、网关、基准配置及签名器；缺失的操作不伪造占位。
func NewOperationHandlers(bind func(context.Context) context.Context) map[string]http.Handler {
	return map[string]http.Handler{
		"estimateRebuildPlan": jsonOperation(bind, build.EstimateRebuildPlan, http.StatusOK),
		"approveRebuildPlan": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			var input build.ApproveRebuildPlanInput
			if json.NewDecoder(r.Body).Decode(&input) != nil || input.PlanId != r.PathValue("planId") {
				writeResult(w, op, nil, platform.NewError(platform.CodeValidation, false, "重建批准路径与正文不一致"), http.StatusOK)
				return
			}
			result, err := build.ApproveRebuildPlan(ctx, op, input)
			writeResult(w, op, result, err, http.StatusOK)
		}),
		"readJobRecovery": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			var input domainruntime.ReadJobRecoveryInput
			if json.NewDecoder(r.Body).Decode(&input) != nil || input.JobID != r.PathValue("jobId") {
				writeResult(w, op, nil, platform.NewError(platform.CodeValidation, false, "恢复路径与原任务不一致"), http.StatusOK)
				return
			}
			result, err := domainruntime.ReadJobRecovery(ctx, op, input)
			writeResult(w, op, result, err, http.StatusOK)
		}),
		"readIndexOptions": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			result, err := admin.ReadIndexOptions(ctx, op, admin.ReadIndexOptionsInput{})
			writeResult(w, op, result, err, http.StatusOK)
		}),
		"putIndexConfig": jsonOperation(bind, indexconfig.PutIndexConfig, http.StatusOK),
		"createBuild":    jsonOperation(bind, build.CreateBuild, http.StatusAccepted),
		"cancelBuild": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			var input build.CancelBuildInput
			if json.NewDecoder(r.Body).Decode(&input) != nil || input.BuildId != r.PathValue("buildId") {
				writeResult(w, op, nil, platform.NewError(platform.CodeValidation, false, "构建取消路径与正文不一致"), http.StatusOK)
				return
			}
			result, err := build.CancelBuild(ctx, op, input)
			writeResult(w, op, result, err, http.StatusOK)
		}),
		"readIndexConfig": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			result, err := indexconfig.ReadIndexConfig(ctx, op, indexconfig.ReadIndexConfigInput{})
			writeResult(w, op, result, err, http.StatusOK)
		}),
		"searchKnowledge":      jsonOperation(bind, search.Retrieve, http.StatusOK),
		"readSections":         jsonOperation(bind, search.ReadSections, http.StatusOK),
		"createQueryEmbedding": jsonOperation(bind, search.CreateQueryEmbedding, http.StatusOK),
		"getBuild": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			result, err := admin.GetBuild(ctx, op, admin.GetBuildInput{BuildID: r.PathValue("buildId")})
			writeResult(w, op, result, err, http.StatusOK)
		}),
		"listBuilds": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			result, err := admin.ListBuilds(ctx, op, admin.ListBuildsInput{DocumentID: r.URL.Query().Get("documentId"), Cursor: r.URL.Query().Get("cursor"), Limit: limit})
			writeResult(w, op, result, err, http.StatusOK)
		}),
		"redactDiagnostics": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, op, err := operationContext(r, bind)
			if err != nil {
				writeResult(w, op, nil, err, http.StatusOK)
				return
			}
			counts, _ := strconv.ParseBool(r.URL.Query().Get("includeCounts"))
			result, err := admin.RedactDiagnostics(ctx, op, admin.RedactDiagnosticsInput{BuildID: r.PathValue("buildId"), IncludeCounts: counts})
			writeResult(w, op, result, err, http.StatusOK)
		}),
	}
}

func jsonOperation[I, R any](bind func(context.Context) context.Context, run func(context.Context, platform.OperationContext, I) (R, error), status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, op, err := operationContext(r, bind)
		if err != nil {
			writeResult(w, op, nil, err, http.StatusOK)
			return
		}
		var input I
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			writeResult(w, op, nil, platform.NewError(platform.CodeValidation, false, "请求正文不可读"), http.StatusOK)
			return
		}
		result, err := run(ctx, op, input)
		writeResult(w, op, result, err, status)
	})
}
func operationContext(r *http.Request, bind func(context.Context) context.Context) (context.Context, platform.OperationContext, error) {
	op, ok := platform.OperationContextFrom(r.Context())
	if !ok || bind == nil {
		return nil, op, platform.NewError(platform.CodeUnauthenticated, false, "请求缺少可信服务入口")
	}
	return bind(r.Context()), op, nil
}
func writeResult(w http.ResponseWriter, op platform.OperationContext, result any, err error, status int) {
	if err != nil {
		_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeInternal, false, "结果编码失败"), op.RequestID))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// Application 的HTTP与资源关闭顺序由platform.RunService统一监督。
type Application struct {
	server     *http.Server
	db         *sql.DB
	runtime    platform.SiteConfigClient
	epoch      string
	tasks      []platform.BackgroundTask
	prepared   bool
	draining   atomic.Bool
	serving    atomic.Bool
	closeOnce  sync.Once
	closeError error
}
type HealthStatus struct {
	Module       string   `json:"module"`
	Version      string   `json:"version"`
	Status       string   `json:"status"`
	Capabilities []string `json:"capabilities"`
}
type ServiceReadiness struct {
	Health                HealthStatus `json:"health"`
	ActiveDataEpoch       *string      `json:"activeDataEpoch"`
	RuntimeRevision       int64        `json:"runtimeRevision"`
	Maintenance           bool         `json:"maintenance"`
	PublicRequestsAllowed bool         `json:"publicRequestsAllowed"`
	Initialized           bool         `json:"initialized"`
	JournalRecovered      bool         `json:"journalRecovered"`
	SafePoint             bool         `json:"safePoint"`
}

// NewApplication 核完整路由和后台入口；缺worker或控制操作时不返回假就绪容器。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, authority platform.SiteConfigClient, epoch string, db *sql.DB, bind func(context.Context) context.Context, authorize func(context.Context, platform.OperationContext, string, string) error, tasks []platform.BackgroundTask) (*Application, error) {
	app := &Application{db: db, runtime: authority, epoch: epoch, tasks: tasks}
	handlers := NewOperationHandlers(bind)
	handlers["health"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, err := Health(r.Context(), app)
		writeResult(w, platform.OperationContext{}, result, err, http.StatusOK)
	})
	handlers["ready"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, err := Ready(r.Context(), app)
		writeResult(w, platform.OperationContext{}, result, err, http.StatusOK)
	})
	schema, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域HTTP合同不可读")
	}
	router, err := apivalidate.NewServiceHandler(schema, platform.ServiceBoundaryConfig{Self: "knowledge", Anchors: anchors, Runtime: func(ctx context.Context) (platform.RuntimeView, error) { return lifecycle.ReadRuntime(ctx, authority) }, OpenedDataEpoch: epoch, Authorize: authorize, MaxBodyBytes: 2 << 20, AllowedCallers: map[string][]string{
		"readIndexConfig": {"portal"}, "putIndexConfig": {"portal"}, "readIndexOptions": {"portal"}, "createBuild": {"portal"}, "cancelBuild": {"portal"}, "listBuilds": {"portal"}, "getBuild": {"portal"}, "redactDiagnostics": {"portal"}, "estimateRebuildPlan": {"portal"}, "approveRebuildPlan": {"portal"}, "searchKnowledge": {"qa"}, "readSections": {"qa"}, "createQueryEmbedding": {"qa"}, "acquireBarrier": {"data-protection"}, "releaseBarrier": {"data-protection"}, "exportSnapshot": {"data-protection"}, "stageSnapshot": {"data-protection"}, "discardSnapshot": {"data-protection"}, "inspectEvidence": {"data-protection"}, "readJobRecovery": {"jobs"}}, ServiceActorCallers: map[string][]string{"searchKnowledge": {"qa"}, "readSections": {"qa"}, "createQueryEmbedding": {"qa"}, "readJobRecovery": {"jobs"}}, MaintenanceOperations: map[string]bool{"acquireBarrier": true, "releaseBarrier": true, "exportSnapshot": true, "stageSnapshot": true, "discardSnapshot": true, "inspectEvidence": true, "readJobRecovery": true}}, handlers)
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, task := range tasks {
		if task.Run != nil {
			present[task.Name] = true
		}
	}
	for _, name := range []string{"knowledge.worker", "knowledge.events", "knowledge.outbox", "knowledge.recovery", "knowledge.gc"} {
		if !present[name] {
			return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "本域完整后台入口尚未装配")
		}
	}
	app.prepared = true
	app.server = &http.Server{Addr: config.Listen, Handler: router, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 64 << 10}
	return app, nil
}
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }
func (a *Application) BackgroundTasks() []platform.BackgroundTask {
	return append([]platform.BackgroundTask{}, a.tasks...)
}
func (a *Application) Drain(ctx context.Context) error {
	a.draining.Store(true)
	if a.server == nil {
		return nil
	}
	if err := a.server.Shutdown(ctx); err != nil {
		return platform.NormalizeIOError(err, platform.CodeInternal, false, "HTTP安全关闭未完成")
	}
	return nil
}

func Serve(ctx context.Context, app *Application) error {
	if !app.prepared || app.server == nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "本域应用尚未完整装配")
	}
	if !app.serving.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同应用重复监听")
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", app.server.Addr)
	if err != nil {
		return platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, false, "HTTP监听失败")
	}
	app.server.BaseContext = func(net.Listener) context.Context { return ctx }
	err = app.server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return platform.NormalizeIOError(err, platform.CodeInternal, false, "HTTP服务停止")
}
func Shutdown(ctx context.Context, app *Application) error {
	app.closeOnce.Do(func() {
		if err := app.Drain(ctx); err != nil {
			app.closeError = err
			return
		}
		if err := app.db.Close(); err != nil {
			app.closeError = platform.NormalizeIOError(err, platform.CodeInternal, false, "私有存储关闭失败")
		}
	})
	return app.closeError
}
func Health(ctx context.Context, app *Application) (HealthStatus, error) {
	status := "ready"
	if app.draining.Load() {
		status = "unavailable"
	}
	return HealthStatus{Module: "knowledge", Version: "1", Status: status, Capabilities: []string{"knowledge.manage"}}, nil
}
func Ready(ctx context.Context, app *Application) (ServiceReadiness, error) {
	work, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	view, err := lifecycle.ReadRuntime(work, app.runtime)
	if err != nil {
		return ServiceReadiness{}, err
	}
	if !app.prepared || app.draining.Load() || view.DataEpoch != app.epoch {
		return ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, false, "本域应用或当前代次尚未就绪")
	}
	if err = app.db.PingContext(work); err != nil {
		return ServiceReadiness{}, platform.NormalizeIOError(err, platform.CodeDependencyUnavailable, false, "本域私有存储不可用")
	}
	health, _ := Health(work, app)
	epoch := app.epoch
	return ServiceReadiness{Health: health, ActiveDataEpoch: &epoch, RuntimeRevision: view.Revision, Maintenance: view.Maintenance, PublicRequestsAllowed: !view.Maintenance && view.Phase == platform.PhaseRunning, Initialized: true, JournalRecovered: true, SafePoint: false}, nil
}
