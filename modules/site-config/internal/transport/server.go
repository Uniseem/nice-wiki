// Package transport 实现 site-config 域的 HTTP 生命周期、业务路由、后台
// 恢复与Relay装配；匿名健康/就绪和优雅关闭复用统一服务边界。
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	siteconfig "nice-wiki/modules/site-config/internal"
	"nice-wiki/modules/site-config/internal/gen"
)

const (
	moduleName    = "site-config"
	moduleVersion = "1"
)

// lifecycle 状态：ready 接受请求，draining 拒新入口，closed 资源已释放。
const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是 site-config 装配完成后的内部容器（x-internal，不序列化）。
// runtime 是本域权威读取器：本域即运行状态权威，读取自身控制库而非
// 远端 HTTP。
type Application struct {
	config          platform.ServiceConfig
	anchors         platform.TrustAnchors
	runtime         platform.SiteConfigClient
	active          *siteconfig.ActiveStore
	openedEpoch     platform.Identifier
	state           atomic.Int32
	serveStarted    atomic.Bool
	shutdownOnce    sync.Once
	shutdownErr     error
	backgroundTasks []platform.BackgroundTask
	drainOnce       sync.Once
	drainErr        error
	httpServer      *http.Server
}

// NewApplication 只向统一入口提供真实已实现的handler；缺登记操作时
// 平台工厂返回错误，不能把部分业务接线伪装成完整服务就绪。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, activeStore *siteconfig.ActiveStore, openedEpoch platform.Identifier,
	authorize func(context.Context, platform.OperationContext, string, string) error) (*Application, error) {
	application := &Application{config: config, anchors: anchors, runtime: runtimeAuthority, active: activeStore, openedEpoch: openedEpoch}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域HTTP规格不可读")
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{
		Self: moduleName, Anchors: anchors, Runtime: runtimeAuthority.ReadRuntimeState, OpenedDataEpoch: openedEpoch,
		AdmitBusiness:  lifecycle.SimpleAdmission(runtimeAuthority.ReadRuntimeState, openedEpoch),
		AllowedCallers: serviceCallers(),
		ServiceActorCallers: map[string][]string{
			"readSiteConfig":            {"portal", "public-site", "content", "media", "localization", "identity", "knowledge", "model-gateway", "memory", "qa", "analytics", "traffic-guard", "jobs"},
			"readConfigObject":          {"public-site"},
			"resumeInitialization":      {"site-config"},
			"readHomeReferenceDecision": {"media"},
			"readPublicHome":            {"portal", "public-site"},
			"listResourceReferences":    {"media"},
		},
		Authorize: authorize,
	}, application.serviceHandlers())
	if err != nil {
		return nil, err
	}
	application.httpServer = &http.Server{Addr: config.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 64 << 10}
	application.backgroundTasks = []platform.BackgroundTask{{Name: "site-config.home-reference-recovery", Run: func(ctx context.Context) error {
		return lifecycle.RunRecoveryLoop(ctx, platform.RecoveryLoopConfig{PollMillis: 1000, Runtime: runtimeAuthority.ReadRuntimeState, AdmitBatch: lifecycle.SimpleAdmission(runtimeAuthority.ReadRuntimeState, openedEpoch),
			WriteWindow: lifecycle.SimpleWriteWindow(runtimeAuthority.ReadRuntimeState), Recover: func(work context.Context) error {
				op, _ := application.homeRecoveryOperation(work)
				result, err := siteconfig.RecoverHomeReferences(work, op, siteconfig.RecoverHomeReferencesInput{})
				if err == nil && result.PendingCount > 0 {
					return platform.NewError(platform.CodeDependencyUnavailable, true, "首页资源引用仍待收敛")
				}
				var failure *platform.Error
				if errors.As(err, &failure) && failure.Code == platform.CodeLocked {
					return platform.NewError(platform.CodeLocked, true, "首页恢复写窗口已暂停")
				}
				return err
			}})
	}}}
	return application, nil
}

// Serve 与 Shutdown 实现 platform.Application，供进程模板调用。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

func (a *Application) serviceHandlers() map[string]http.Handler {
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
			status, err := Ready(r.Context(), a)
			if err != nil {
				writeProblem(w, err)
				return
			}
			writeJSON(w, http.StatusOK, status)
		}),
		"readSiteConfig":           serveJSON(siteconfig.ReadSiteConfig),
		"updateSiteConfig":         serveJSON(siteconfig.UpdateSiteConfig),
		"updateHome":               serveJSON(siteconfig.UpdateHome),
		"updateBranding":           serveJSON(siteconfig.UpdateBranding),
		"updateFooter":             serveJSON(siteconfig.UpdateFooter),
		"updateBanner":             serveJSON(siteconfig.UpdateBanner),
		"readConfigHistory":        http.HandlerFunc(serveConfigHistory),
		"restoreConfigRevision":    http.HandlerFunc(serveRestoreConfigRevision),
		"listResourceReferences":   http.HandlerFunc(serveListResourceReferences),
		"readRuntimeState":         serveJSON(siteconfig.ReadRuntimeState),
		"readInitializationStatus": serveJSON(siteconfig.ReadInitializationStatus),
		"initializeSite":           serveJSON(siteconfig.InitializeSite),
		"resumeInitialization":     serveJSON(siteconfig.ResumeInitialization),
		"abortInitialization":      serveJSON(siteconfig.AbortInitialization),
		"readConfigObject":         http.HandlerFunc(serveConfigObject),
		"readPublicHome": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			var language *string
			if values, ok := r.URL.Query()["language"]; ok {
				value := values[0]
				language = &value
			}
			result, err := siteconfig.ReadPublicHome(r.Context(), op, siteconfig.ReadPublicHomeInput{Language: language})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"readHomeReferenceDecision": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := siteconfig.ReadHomeReferenceDecision(r.Context(), op, siteconfig.ReadHomeReferenceDecisionInput{SourceOperationID: r.PathValue("sourceOperationId")})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
	}
}

func serveJSON[Input, Result any](handler func(context.Context, platform.OperationContext, Input) (Result, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 身份、期限与结构由NewServiceHandler一次建立；此处仅协议转换。
		op, _ := platform.OperationContextFrom(r.Context())
		var input Input
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "请求正文无法转换"))
				return
			}
		}
		result, err := handler(r.Context(), op, input)
		if err != nil {
			writeRequestProblem(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func serveConfigHistory(w http.ResponseWriter, r *http.Request) {
	op, _ := platform.OperationContextFrom(r.Context())
	var limit *int
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "历史分页参数不合法"))
			return
		}
		limit = &value
	}
	page := siteconfig.ConfigPageRequest{}
	if limit != nil {
		page.Limit = *limit
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		page.Cursor = raw
	}
	result, err := siteconfig.ReadConfigHistory(r.Context(), op, siteconfig.ReadConfigHistoryInput{Page: page})
	if err != nil {
		writeRequestProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func serveRestoreConfigRevision(w http.ResponseWriter, r *http.Request) {
	op, _ := platform.OperationContextFrom(r.Context())
	var body gen.RestoreConfigRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "请求正文无法转换"))
		return
	}
	result, err := siteconfig.RestoreConfigRevision(r.Context(), op, siteconfig.RestoreConfigRevisionInput{RevisionID: r.PathValue("revisionId"), ExpectedRevision: body.ExpectedRevision})
	if err != nil {
		writeRequestProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func serveListResourceReferences(w http.ResponseWriter, r *http.Request) {
	op, _ := platform.OperationContextFrom(r.Context())
	page := siteconfig.ResourceReferencePageRequest{Limit: 20}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "资源引用分页参数不合法"))
			return
		}
		page.Limit = limit
	}
	page.Cursor = r.URL.Query().Get("cursor")
	var assetID *gen.Identifier
	if values, ok := r.URL.Query()["assetId"]; ok {
		value := gen.Identifier(values[0])
		assetID = &value
	}
	result, err := siteconfig.ListResourceReferences(r.Context(), op, siteconfig.ListResourceReferencesInput{AssetID: assetID, Page: page})
	if err != nil {
		writeRequestProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func serveConfigObject(w http.ResponseWriter, r *http.Request) {
	op, _ := platform.OperationContextFrom(r.Context())
	version, err := strconv.Atoi(r.URL.Query().Get("version"))
	if err != nil {
		writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "对象版本无法转换"))
		return
	}
	result, err := siteconfig.ReadConfigObject(r.Context(), op, siteconfig.ReadConfigObjectInput{ObjectId: r.PathValue("objectId"), Version: version, ExpectedHash: r.URL.Query().Get("expectedHash")})
	if err != nil {
		writeRequestProblem(w, r, err)
		return
	}
	defer result.Reader.Close()
	if deadline, ok := r.Context().Deadline(); ok {
		if err := http.NewResponseController(w).SetWriteDeadline(deadline); err != nil {
			writeRequestProblem(w, r, platform.NewError(platform.CodeDependencyUnavailable, false, "对象响应不支持写截止"))
			return
		}
	}
	w.Header().Set("ETag", strconv.Quote(result.Object.Sha256))
	w.Header().Set("Content-Type", result.Object.MediaType)
	w.Header().Set("Content-Length", strconv.Itoa(result.Object.ByteSize))
	w.Header().Set("X-Object-Id", result.Object.ObjectId)
	w.Header().Set("X-Object-Version", strconv.Itoa(result.Object.Version))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	// 响应开始后不再附JSON错误；Content-Length令客户端识别截断。
	_, _ = io.CopyN(w, result.Reader, int64(result.Object.ByteSize))
}

func writeRequestProblem(w http.ResponseWriter, r *http.Request, err error) {
	op, _ := platform.OperationContextFrom(r.Context())
	_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
}

func serviceCallers() map[string][]string {
	stateful := []string{"identity", "site-config", "content", "media", "public-site", "model-gateway", "knowledge", "qa", "localization", "memory", "traffic-guard", "jobs", "analytics"}
	runtime := append(append([]string{}, stateful...), "portal", "agent")
	callers := map[string][]string{
		"readRuntimeState":         runtime,
		"readSiteConfig":           {"portal", "public-site", "content", "media", "localization", "identity", "knowledge", "model-gateway", "memory", "qa", "analytics", "traffic-guard", "jobs"},
		"readConfigObject":         {"portal", "public-site"},
		"readInitializationStatus": {"portal", "identity", "site-config", "knowledge", "model-gateway", "memory", "qa", "analytics", "traffic-guard", "jobs", "content", "media", "localization", "public-site"}, "resumeInitialization": {"portal", "site-config"},
		"readPublicHome": {"portal", "public-site"}, "listResourceReferences": {"portal", "media"}, "readHomeReferenceDecision": {"media"},
	}
	for _, id := range []string{"initializeSite", "abortInitialization", "updateSiteConfig", "updateHome", "updateBranding", "updateFooter", "updateBanner", "readConfigHistory", "restoreConfigRevision"} {
		callers[id] = []string{"portal"}
	}
	return callers
}

// Serve 运行独立HTTP服务；ctx 取消触发的停止由进程模板调用 Shutdown
// 完成，Serve 只在监听结束后返回。
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

// Shutdown 停止接受新请求并等待在途响应到安全点；sync.Once 保证重复
// 调用返回同一次结果，deadline 未安全返回 DEADLINE_EXCEEDED。
func Shutdown(ctx context.Context, app *Application) error {
	app.shutdownOnce.Do(func() {
		err := app.Drain(ctx)
		if err == nil {
			if app.active != nil {
				if closeErr := app.active.Close(); closeErr != nil {
					app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭active业务库失败")
					return
				}
			}
			if closer, ok := app.runtime.(interface{ Close() error }); ok {
				if closeErr := closer.Close(); closeErr != nil {
					app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭运行控制库失败")
					return
				}
			}
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

// Ready 确认本域配置与信任锚完整、自身运行权威可读；就绪不代表维护已
// 解除。本域即运行权威，经本地控制库读取（site-config本域读取自身权威），
// 控制库、active schema 或公告 journal 不可用时如实返回
// DEPENDENCY_UNAVAILABLE/INTEGRITY_ERROR（503/422），不以未知状态放行。
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
	activeEpoch := string(app.openedEpoch)
	if activeEpoch == "" {
		activeEpoch = string(view.DataEpoch)
	}
	if app.active == nil {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "active业务库未装配")
	}
	initialized, err := app.active.ValidateActiveReady(ctx, platform.Identifier(activeEpoch))
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	readiness := gen.ServiceReadiness{
		Health:                health,
		Initialized:           initialized,
		JournalRecovered:      true,
		PublicRequestsAllowed: true,
		RuntimeRevision:       0,
	}
	if initialized {
		readiness.ActiveDataEpoch = &activeEpoch
	}
	if detailed, ok := app.runtime.(interface {
		ReadRuntimeStateDetails(context.Context) (gen.RuntimeState, error)
	}); ok {
		state, err := detailed.ReadRuntimeStateDetails(ctx)
		if err != nil {
			return gen.ServiceReadiness{}, err
		}
		readiness.RuntimeRevision = state.Revision
	}
	return readiness, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, err error) {
	_ = problem.WriteProblem(w, problem.FromError(err, platform.NewIdentifier()))
}

// BackgroundTasks 仅返回已装配闭包；统一进程模板在完整构造成功后启动与停止。
func (a *Application) BackgroundTasks() []platform.BackgroundTask {
	return append([]platform.BackgroundTask(nil), a.backgroundTasks...)
}

// Drain 不关闭存储；公共生命周期先等后台结束，再调用Shutdown关闭资源。
func (a *Application) Drain(ctx context.Context) error {
	a.drainOnce.Do(func() {
		a.state.CompareAndSwap(stateReady, stateDraining)
		if err := a.httpServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				a.drainErr = platform.NewError(platform.CodeDeadlineExceeded, true, "入口请求未在期限内排空")
			} else {
				a.drainErr = platform.NewError(platform.CodeInternal, false, "停止HTTP入口失败")
			}
		}
	})
	return a.drainErr
}

func (a *Application) homeRecoveryOperation(ctx context.Context) (platform.OperationContext, time.Time) {
	deadline := time.Now().UTC().Add(30 * time.Second)
	if existing, ok := ctx.Deadline(); ok && existing.Before(deadline) {
		deadline = existing
	}
	id := platform.NewIdentifier()
	stamp := platform.Timestamp(deadline.UTC().Format(time.RFC3339Nano))
	return platform.OperationContext{RequestID: id, OperationID: id, RootOperationID: id, CallerService: moduleName, DataEpoch: a.openedEpoch, Deadline: stamp,
		Actor: platform.AuthContext{Kind: "service", SubjectID: moduleName, SiteID: "system", Grants: []string{}, ExpiresAt: stamp}}, deadline
}
