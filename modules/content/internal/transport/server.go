// Package transport 实现 content 域的 HTTP 生命周期与匿名状态端点。
// 业务路由待本域 x-handler 工作包完成后接入；此阶段不返回伪成功。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	content "nice-wiki/modules/content/internal"
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
	closers             []func() error
	router              *apivalidate.PathRouter
	background          []platform.BackgroundTask
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
	application.router = apivalidate.NewPathRouter(mux)
	application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           application.router,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return application
}

// SetBackground 登记随进程运行的后台任务（事件转发等）。
func (a *Application) SetBackground(tasks []platform.BackgroundTask) { a.background = tasks }

// BackgroundTasks 实现 platform.BackgroundApplication。
func (a *Application) BackgroundTasks() []platform.BackgroundTask {
	return append([]platform.BackgroundTask{}, a.background...)
}

// Drain 只停止接收新 HTTP 并等待在途请求；后台任务退出后才由 Shutdown 关闭存储。
func (a *Application) Drain(ctx context.Context) error {
	a.state.CompareAndSwap(stateReady, stateDraining)
	if err := a.httpServer.Shutdown(ctx); err != nil {
		return platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全停止在途操作")
	}
	return nil
}

// OnShutdown 登记随进程关闭的资源（数据库、编辑锁等）。
func (a *Application) OnShutdown(closer func() error) { a.closers = append(a.closers, closer) }

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
		for i := len(app.closers) - 1; i >= 0; i-- {
			_ = app.closers[i]()
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
		PublicRequestsAllowed: true,
		Initialized:           true,
		JournalRecovered:      app.journalRecovered,
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

// RegisterReferenceRoutes只装配本轮两个真实只读路由，完整应用仍由BuildApplication门槛决定。
func RegisterReferenceRoutes(app *Application, runtime content.ReferenceRuntime, anchors platform.TrustAnchors) error {
	spec, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "引用来源HTTP规格不可读")
	}
	for path := range spec.Paths.Map() {
		if path != "/v1/resource-references" && path != "/v1/reference-decisions/{sourceOperationId}" {
			spec.Paths.Delete(path)
		}
	}
	handlers := map[string]http.Handler{
		"listResourceReferences": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			limit := 20
			if value := r.URL.Query().Get("limit"); value != "" {
				parsed, _ := strconv.ParseInt(value, 0, 64)
				limit = int(parsed)
			}
			input := content.ListResourceReferencesInput{Page: content.ReferencePageRequest{Limit: limit, Cursor: r.URL.Query().Get("cursor")}}
			if value := r.URL.Query().Get("documentId"); value != "" {
				input.DocumentId = &value
			}
			if value := r.URL.Query().Get("assetId"); value != "" {
				input.AssetId = &value
			}
			result, err := content.ListResourceReferences(content.WithReferenceRuntime(r.Context(), runtime), op, input)
			w.Header().Set("Cache-Control", "no-store")
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readReferenceDecision": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := content.ReadReferenceDecision(content.WithReferenceRuntime(r.Context(), runtime), op, content.ReadReferenceDecisionInput{SourceOperationId: r.PathValue("sourceOperationId")})
			w.Header().Set("Cache-Control", "no-store")
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, Runtime: app.runtime.ReadRuntimeState, OpenedDataEpoch: app.openedEpoch, AdmitBusiness: lifecycle.SimpleAdmission(app.runtime.ReadRuntimeState, app.openedEpoch),
		AllowedCallers:      map[string][]string{"listResourceReferences": {"media", "site-config", "portal"}, "readReferenceDecision": {"media"}},
		ServiceActorCallers: map[string][]string{"listResourceReferences": {"media", "site-config"}}}, handlers)
	if err != nil {
		return err
	}
	app.router.Handle("GET", "/v1/resource-references", handler)
	app.router.Handle("GET", "/v1/reference-decisions/{sourceOperationId}", handler)
	return nil
}

// RegisterTreeRoutes只注册五个已实际实现核心入口；即时权限由核心在任何DB前核一次。
func RegisterTreeRoutes(app *Application, runtime content.TreeRuntime, anchors platform.TrustAnchors) error {
	spec, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "文档树HTTP规格不可读")
	}
	for path := range spec.Paths.Map() {
		if path != "/v1/documents" && path != "/v1/tree" && path != "/v1/tree:move" && path != "/v1/tree:reorder" && path != "/v1/documents/{documentId}/metadata" {
			spec.Paths.Delete(path)
		}
	}
	// /v1/documents共享GET/POST，只装本轮真实GET；其余完整handler仍由BuildApplication关闭启动。
	spec.Paths.Value("/v1/documents").Post = nil
	handlers := map[string]http.Handler{
		"updateDocumentMetadata": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.UpdateDocumentMetadataRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "元数据编辑参数无法转换"))
				return
			}
			input := content.UpdateDocumentMetadataInput{DocumentId: r.PathValue("documentId"), Title: body.Title, PathSegment: body.PathSegment, SeoDescription: body.SeoDescription, ShareImage: body.ShareImage, ExpectedRevision: body.ExpectedRevision}
			result, err := content.UpdateDocumentMetadata(content.WithTreeRuntime(r.Context(), runtime), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readDocumentTree": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := content.ReadDocumentTree(content.WithTreeRuntime(r.Context(), runtime), op, content.ReadDocumentTreeInput{})
			w.Header().Set("Cache-Control", "no-store")
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"listDocuments": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			limit := 20
			if value := r.URL.Query().Get("limit"); value != "" {
				parsed, _ := strconv.ParseInt(value, 0, 64)
				limit = int(parsed)
			}
			input := content.ListDocumentsInput{State: r.URL.Query().Get("state"), Page: content.ReferencePageRequest{Limit: limit, Cursor: r.URL.Query().Get("cursor")}}
			result, err := content.ListDocuments(content.WithTreeRuntime(r.Context(), runtime), op, input)
			w.Header().Set("Cache-Control", "no-store")
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"moveDocument": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input content.MoveDocumentInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "移动文档参数无法转换"))
				return
			}
			result, err := content.MoveDocument(content.WithTreeRuntime(r.Context(), runtime), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"reorderChildren": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var input content.ReorderChildrenInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "重排文档参数无法转换"))
				return
			}
			result, err := content.ReorderChildren(content.WithTreeRuntime(r.Context(), runtime), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
	}
	generalAuthorize := apivalidate.NewIdentityAuthorizer(runtime.Reference.Identity)
	// 根已确认这五个固定action在真实核心中按state选择read/publish并即时核权威一次；
	// 仅对本注册表跳过平台的重复RPC，服务身份/epoch/schema仍在统一边界严格执行。
	authorize := func(ctx context.Context, op platform.OperationContext, permission, action string) error {
		switch action {
		case "content:readDocumentTree", "content:listDocuments", "content:moveDocument", "content:reorderChildren", "content:updateDocumentMetadata":
			return nil
		}
		return generalAuthorize(ctx, op, permission, action)
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, Runtime: app.runtime.ReadRuntimeState, OpenedDataEpoch: app.openedEpoch, AdmitBusiness: lifecycle.SimpleAdmission(app.runtime.ReadRuntimeState, app.openedEpoch), Authorize: authorize,
		AllowedCallers: map[string][]string{"readDocumentTree": {"portal"}, "listDocuments": {"portal"}, "moveDocument": {"portal"}, "reorderChildren": {"portal"}, "updateDocumentMetadata": {"portal"}}}, handlers)
	if err != nil {
		return err
	}
	app.router.Handle("GET", "/v1/tree", handler)
	app.router.Handle("GET", "/v1/documents", handler)
	app.router.Handle("POST", "/v1/tree:move", handler)
	app.router.Handle("POST", "/v1/tree:reorder", handler)
	app.router.Handle("PUT", "/v1/documents/{documentId}/metadata", handler)
	return nil
}

// RegisterPublicationRoutes注册四个已实现的发布路由。启动仍由BuildApplication失败关闭。
func RegisterPublicationRoutes(app *Application, runtime content.TreeRuntime, anchors platform.TrustAnchors) error {
	spec, err := gen.GetSwagger()
	if err != nil {
		return platform.NewError(platform.CodeIntegrityError, false, "发布HTTP规格不可读")
	}
	keep := map[string]bool{
		"/v1/documents/{documentId}:publish":             true,
		"/v1/documents/{documentId}:suggest-publication": true,
		"/v1/documents/{documentId}:withdraw":            true,
		"/v1/publications:validate":                      true,
		"/v1/public-tree":                                true, "/v1/public-path:resolve": true,
		"/v1/documents/{documentId}/history": true, "/v1/documents/{documentId}/history/{historyId}": true, "/v1/documents/{documentId}/history:diff": true,
		"/v1/documents/{documentId}/published": true,
		"/v1/documents/{documentId}:trash":     true, "/v1/documents/{documentId}:restore": true, "/v1/trash:purge": true,
	}
	for path := range spec.Paths.Map() {
		if !keep[path] {
			spec.Paths.Delete(path)
		}
	}
	handlers := map[string]http.Handler{
		"publishDocument": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.PublishDocumentRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "发布参数无法转换"))
				return
			}
			input := content.PublishDocumentInput{DocumentId: r.PathValue("documentId"), DraftRevisionId: body.DraftRevisionId, ExpectedDraftVersion: body.ExpectedDraftVersion, ExpectedRevision: body.ExpectedRevision, ManualPathSegment: body.ManualPathSegment, ManualSEO: body.ManualSEO}
			result, err := content.PublishDocument(content.WithTreeRuntime(r.Context(), runtime), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"suggestPublicationMetadata": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.SuggestPublicationMetadataRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "发布建议参数无法转换"))
				return
			}
			result, err := content.SuggestPublicationMetadata(content.WithTreeRuntime(r.Context(), runtime), op, content.SuggestPublicationMetadataInput{DocumentId: r.PathValue("documentId"), DraftRevisionId: body.DraftRevisionId})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"withdrawDocument": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.WithdrawDocumentRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "撤回参数无法转换"))
				return
			}
			result, err := content.WithdrawDocument(content.WithTreeRuntime(r.Context(), runtime), op, content.WithdrawDocumentInput{DocumentId: r.PathValue("documentId"), ExpectedRevision: body.ExpectedRevision})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"trashDocument": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.TrashDocumentRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "删除参数无法转换"))
				return
			}
			result, err := content.TrashDocument(content.WithTreeRuntime(r.Context(), runtime), op, content.TrashDocumentInput{DocumentId: r.PathValue("documentId"), IncludeSubtree: body.IncludeSubtree, ReplacementDocumentId: body.ReplacementDocumentId, ExpectedRevision: body.ExpectedRevision, ExpectedTreeRevision: body.ExpectedTreeRevision})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"restoreDocument": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var body gen.RestoreDocumentRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "恢复参数无法转换"))
				return
			}
			result, err := content.RestoreDocument(content.WithTreeRuntime(r.Context(), runtime), op, content.RestoreDocumentInput{DocumentId: r.PathValue("documentId"), ParentId: body.ParentId, PathSegment: body.PathSegment, ExpectedRevision: body.ExpectedRevision})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"purgeDocuments": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var input content.PurgeDocumentsInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "彻底删除参数无法转换"))
				return
			}
			result, err := content.PurgeDocuments(content.WithTreeRuntime(r.Context(), runtime), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readRevisionHistory": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			result, err := content.ReadRevisionHistory(content.WithTreeRuntime(r.Context(), runtime), op, content.ReadRevisionHistoryInput{DocumentId: r.PathValue("documentId"), Page: content.ReferencePageRequest{Limit: limit, Cursor: r.URL.Query().Get("cursor")}})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readHistoricalRevision": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := content.ReadHistoricalRevision(content.WithTreeRuntime(r.Context(), runtime), op, content.ReadHistoricalRevisionInput{DocumentId: r.PathValue("documentId"), HistoryId: r.PathValue("historyId")})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"diffRevisions": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			// 请求体只有左右两个发布修订（schema 已校验），文档取自路径。
			var input content.DiffRevisionsInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "历史对比请求不可读"))
				return
			}
			input.DocumentId = r.PathValue("documentId")
			result, err := content.DiffRevisions(content.WithTreeRuntime(r.Context(), runtime), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readPublicTree": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := content.ReadPublicTree(content.WithTreeRuntime(r.Context(), runtime), op, content.ReadPublicTreeInput{})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"resolvePublicPath": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := content.ResolvePublicPath(content.WithTreeRuntime(r.Context(), runtime), op, content.ResolvePublicPathInput{Path: r.URL.Query().Get("path")})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"validatePublications": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var input content.ValidatePublicationsInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeProblem(w, platform.NewError(platform.CodeValidation, false, "发布校验参数无法转换"))
				return
			}
			result, err := content.ValidatePublications(content.WithTreeRuntime(r.Context(), runtime), op, input)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readPublishedRevision": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			op, _ := platform.OperationContextFrom(r.Context())
			var published *content.Identifier
			if value := r.URL.Query().Get("publishedRevisionId"); value != "" {
				id := content.Identifier(value)
				published = &id
			}
			result, err := content.ReadPublishedRevision(content.WithTreeRuntime(r.Context(), runtime), op, content.ReadPublishedRevisionInput{DocumentId: r.PathValue("documentId"), PublishedRevisionId: published})
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
	}
	authorize := func(ctx context.Context, op platform.OperationContext, permission, action string) error {
		switch action {
		case "content:publishDocument", "content:suggestPublicationMetadata", "content:withdrawDocument", "content:readRevisionHistory", "content:readHistoricalRevision", "content:diffRevisions", "content:trashDocument", "content:restoreDocument", "content:purgeDocuments":
			return nil
		}
		return platform.NewError(platform.CodeForbidden, false, "发布路由没有该操作的用户授权")
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, Runtime: app.runtime.ReadRuntimeState, OpenedDataEpoch: app.openedEpoch, AdmitBusiness: lifecycle.SimpleAdmission(app.runtime.ReadRuntimeState, app.openedEpoch), Authorize: authorize,
		AllowedCallers: map[string][]string{
			"publishDocument": {"portal"}, "suggestPublicationMetadata": {"portal"},
			"readRevisionHistory": {"portal"}, "readHistoricalRevision": {"portal"}, "diffRevisions": {"portal"},
			"withdrawDocument": {"portal"}, "trashDocument": {"portal"}, "restoreDocument": {"portal"}, "purgeDocuments": {"portal"},
			"validatePublications": {"public-site", "knowledge", "qa", "localization", "site-config", "memory"},
			"readPublicTree":       {"public-site", "knowledge", "qa", "localization", "site-config", "memory"}, "resolvePublicPath": {"public-site", "knowledge", "qa", "localization", "site-config", "memory"},
			"readPublishedRevision": {"public-site", "knowledge", "qa", "localization", "memory"},
		}}, handlers)
	if err != nil {
		return err
	}
	app.router.Handle("POST", "/v1/documents/{documentId}:publish", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}:suggest-publication", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}:withdraw", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}:trash", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}:restore", handler)
	app.router.Handle("POST", "/v1/trash:purge", handler)
	app.router.Handle("POST", "/v1/publications:validate", handler)
	app.router.Handle("GET", "/v1/documents/{documentId}/published", handler)
	app.router.Handle("GET", "/v1/documents/{documentId}/history", handler)
	app.router.Handle("GET", "/v1/documents/{documentId}/history/{historyId}", handler)
	app.router.Handle("POST", "/v1/documents/{documentId}/history:diff", handler)
	app.router.Handle("GET", "/v1/public-tree", handler)
	app.router.Handle("GET", "/v1/public-path:resolve", handler)
	return nil
}
