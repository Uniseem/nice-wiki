// Package transport 实现 media 域的 HTTP 生命周期、匿名健康/就绪和
// 优雅关闭。业务路由待对应工作包实现后接入。
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
	mediaDomain "nice-wiki/modules/media/internal"
	"nice-wiki/modules/media/internal/gen"
)

const (
	moduleName    = "media"
	moduleVersion = "1"
)

const (
	stateReady int32 = iota
	stateDraining
	stateClosed
)

// Application 是 media 装配完成后的内部容器，不作为HTTP DTO序列化。
type Application struct {
	config              platform.ServiceConfig
	runtime             platform.SiteConfigClient
	activeDataEpoch     platform.Identifier
	journalRecovered    bool
	restoreControlReady bool
	state               atomic.Int32
	serveStarted        atomic.Bool
	shutdownOnce        sync.Once
	shutdownErr         error
	backgroundTasks     []platform.BackgroundTask
	drainOnce           sync.Once
	drainErr            error
	httpServer          *http.Server
}

// NewApplication 只绑定真实已实现handler；缺任一OpenAPI处理器则失败关闭。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, activeDataEpoch platform.Identifier,
	authorize func(context.Context, platform.OperationContext, string, string) error) (*Application, error) {
	application := &Application{config: config, runtime: runtimeAuthority, activeDataEpoch: activeDataEpoch, journalRecovered: mediaDomain.LocalStorageRecovered(activeDataEpoch), restoreControlReady: true}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体HTTP规格不可读")
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: moduleName, Anchors: anchors, Runtime: runtimeAuthority.ReadRuntimeState, OpenedDataEpoch: activeDataEpoch, AdmitBusiness: lifecycle.SimpleAdmission(runtimeAuthority.ReadRuntimeState, activeDataEpoch),
		AllowedCallers:        map[string][]string{"beginUpload": {"portal"}, "readUpload": {"portal"}, "abortUpload": {"portal"}, "putUploadChunk": {"portal"}, "completeUpload": {"portal"}, "attachImageVariants": {"portal"}, "readAsset": {"portal", "content", "site-config", "public-site", "localization", "knowledge", "model-gateway"}, "readAssetBytes": {"portal", "knowledge", "model-gateway"}, "headAssetBytes": {"portal", "knowledge", "model-gateway"}, "buildDeliveryManifest": {"public-site", "content", "portal"}, "acquireResourceReferences": {"content", "site-config"}, "confirmResourceReferences": {"content", "site-config"}, "releaseResourceReferences": {"content", "site-config"}, "readAssetReferences": {"portal"}, "listAssets": {"portal"}, "listUnusedAssets": {"portal"}, "listCleanupPlans": {"portal"}, "proposeManualCleanup": {"portal"}, "confirmManualCleanup": {"portal"}, "cancelManualCleanup": {"portal"}},
		ServiceActorCallers:   map[string][]string{"buildDeliveryManifest": {"public-site", "content"}, "readAsset": {"content", "site-config", "public-site", "localization", "knowledge", "model-gateway"}, "acquireResourceReferences": {"content", "site-config"}, "confirmResourceReferences": {"content", "site-config"}, "releaseResourceReferences": {"content", "site-config"}},
		MaintenanceOperations: map[string]bool{"putUploadChunk": true, "beginUpload": true, "completeUpload": true, "attachImageVariants": true, "acquireResourceReferences": true, "confirmResourceReferences": true, "releaseResourceReferences": true, "proposeManualCleanup": true, "confirmManualCleanup": true, "cancelManualCleanup": true}, OperationBodyLimits: map[string]int64{"putUploadChunk": 8 << 20}, Authorize: authorize}, application.serviceHandlers())
	if err != nil {
		return nil, err
	}
	application.httpServer = &http.Server{Addr: config.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 64 << 10}
	return application, nil
}

func (a *Application) serviceHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"health": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result, err := Health(r.Context(), a)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"ready": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result, err := Ready(r.Context(), a)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"beginUpload": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var input mediaDomain.BeginUploadInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "上传声明无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.BeginUpload(r.Context(), op, input)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readUpload": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ReadUpload(r.Context(), op, mediaDomain.ReadUploadInput{UploadId: r.PathValue("uploadId")})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"abortUpload": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.AbortUpload(r.Context(), op, mediaDomain.AbortUploadInput{UploadId: r.PathValue("uploadId")})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"putUploadChunk": http.HandlerFunc(serveUploadChunk),
		"readAssetBytes": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			if err := mediaDomain.ServeAssetBytes(w, r, op, r.PathValue("assetId"), false); err != nil {
				writeRequestProblem(w, r, err)
			}
		}),
		"headAssetBytes": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			if err := mediaDomain.ServeAssetBytes(w, r, op, r.PathValue("assetId"), true); err != nil {
				writeRequestProblem(w, r, err)
			}
		}),
		"buildDeliveryManifest": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var input mediaDomain.BuildDeliveryManifestInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "交付清单请求无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.BuildDeliveryManifest(r, op, input)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"completeUpload": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				ExpectedRevision gen.Revision `json:"expectedRevision"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "完成上传参数无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.CompleteUpload(r.Context(), op, mediaDomain.CompleteUploadInput{UploadId: r.PathValue("uploadId"), ExpectedRevision: body.ExpectedRevision})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"confirmResourceReferences": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var input mediaDomain.ConfirmResourceReferencesInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "确认引用参数无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ConfirmResourceReferences(r.Context(), op, input)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"releaseResourceReferences": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var input mediaDomain.ReleaseResourceReferencesInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "释放引用参数无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ReleaseResourceReferences(r.Context(), op, input)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"acquireResourceReferences": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var input mediaDomain.AcquireResourceReferencesInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "引用预留参数无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.AcquireResourceReferences(r.Context(), op, input)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"listAssets": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := mediaDomain.AssetPageRequest{Limit: 20, Cursor: r.URL.Query().Get("cursor")}
			if raw := r.URL.Query().Get("limit"); raw != "" {
				value, _ := strconv.ParseInt(raw, 0, 64)
				page.Limit = int(value)
			}
			var kind *gen.AssetKind
			if raw := r.URL.Query().Get("kind"); raw != "" {
				value := gen.AssetKind(raw)
				kind = &value
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ListAssets(r.Context(), op, mediaDomain.ListAssetsInput{Page: page, Kind: kind})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"listUnusedAssets": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := mediaDomain.AssetPageRequest{Limit: 20, Cursor: r.URL.Query().Get("cursor")}
			if raw := r.URL.Query().Get("limit"); raw != "" {
				value, _ := strconv.ParseInt(raw, 0, 64)
				page.Limit = int(value)
			}
			var kind *gen.AssetKind
			if raw := r.URL.Query().Get("kind"); raw != "" {
				value := gen.AssetKind(raw)
				kind = &value
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ListUnusedAssets(r.Context(), op, mediaDomain.ListUnusedAssetsInput{Page: page, Kind: kind})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"readAssetReferences": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ReadAssetReferences(r.Context(), op, mediaDomain.ReadAssetReferencesInput{AssetId: r.PathValue("assetId")})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"listCleanupPlans": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ListCleanupPlans(r.Context(), op, mediaDomain.ListCleanupPlansInput{})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"proposeManualCleanup": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var input mediaDomain.ProposeManualCleanupInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "人工清理选择无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ProposeManualCleanup(r.Context(), op, input)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"confirmManualCleanup": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var input mediaDomain.ConfirmManualCleanupInput
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "人工确认材料无法转换"))
				return
			}
			input.PlanId = r.PathValue("planId")
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ConfirmManualCleanup(r.Context(), op, input)
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"cancelManualCleanup": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.CancelManualCleanup(r.Context(), op, mediaDomain.CancelManualCleanupInput{PlanId: r.PathValue("planId")})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
		"attachImageVariants": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body gen.AttachImageVariantsRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "变体参数无法转换"))
				return
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.AttachImageVariants(r.Context(), op, mediaDomain.AttachImageVariantsInput{AssetId: r.PathValue("assetId"), Original: body.Original, Variants: body.Variants, ExpectedRevision: body.ExpectedRevision})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		}),
		"readAsset": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var version *gen.Revision
			if text := r.URL.Query().Get("version"); text != "" {
				value, err := strconv.Atoi(text)
				if err != nil {
					writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "资源版本无法转换"))
					return
				}
				version = &value
			}
			op, _ := platform.OperationContextFrom(r.Context())
			result, err := mediaDomain.ReadAsset(r.Context(), op, mediaDomain.ReadAssetInput{AssetId: r.PathValue("assetId"), Version: version})
			if err != nil {
				writeRequestProblem(w, r, err)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, result)
		}),
	}
}

func serveUploadChunk(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength < 1 || r.ContentLength > 8<<20 {
		writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "分块必须有1至8MiB的明确Content-Length"))
		return
	}
	offset, err := strconv.Atoi(r.Header.Get("X-Upload-Offset"))
	if err != nil {
		writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "上传offset无法转换"))
		return
	}
	revision, err := strconv.Atoi(r.Header.Get("If-Match"))
	if err != nil {
		writeRequestProblem(w, r, platform.NewError(platform.CodeValidation, false, "上传revision无法转换"))
		return
	}
	op, _ := platform.OperationContextFrom(r.Context())
	result, err := mediaDomain.PutUploadChunk(r.Context(), op, mediaDomain.PutUploadChunkInput{UploadId: r.PathValue("uploadId"), ExpectedRevision: revision,
		Chunk: mediaDomain.BinaryChunk{Offset: offset, Length: int(r.ContentLength), SHA256: r.Header.Get("X-Chunk-SHA256"), Reader: r.Body}})
	if err != nil {
		writeRequestProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeRequestProblem(w http.ResponseWriter, r *http.Request, err error) {
	op, _ := platform.OperationContextFrom(r.Context())
	_ = problem.WriteProblem(w, problem.FromError(err, op.RequestID))
}

// Serve 与 Shutdown 实现平台服务生命周期接口。
func (a *Application) Serve(ctx context.Context) error    { return Serve(ctx, a) }
func (a *Application) Shutdown(ctx context.Context) error { return Shutdown(ctx, a) }

// Serve 启动本域 HTTP server；平台进程模板负责在信号或恢复切换时调用
// Shutdown，重复启动不打开第二个监听器。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil || ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "HTTP应用句柄不完整")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if app.state.Load() != stateReady {
		return platform.NewError(platform.CodeConflict, false, "Application已不在可启动状态")
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

// Shutdown由平台在背景任务退出后调用；先复用Drain结果，再关闭私有库。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil || ctx == nil {
		return platform.NewError(platform.CodeValidation, false, "HTTP应用句柄不完整")
	}
	app.shutdownOnce.Do(func() {
		if err := app.Drain(ctx); err != nil {
			app.shutdownErr = err
			return
		}
		if err := mediaDomain.CloseLocalBlobStore(); err != nil {
			app.shutdownErr = err
			return
		}
		app.state.Store(stateClosed)
	})
	return app.shutdownErr
}

// Health 仅读本进程状态，不查询远程依赖、模型或本域业务存储。
func Health(ctx context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.httpServer == nil || app.state.Load() == stateClosed {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       http.StatusOK,
		Capabilities: []string{},
	}, nil
}

// Ready 确认运行权威与控制区安全状态；本工作包未接入业务journal时，
// 明确返回 DEPENDENCY_UNAVAILABLE，不以空状态冒充就绪。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.httpServer == nil || app.state.Load() != stateReady {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务尚未处于就绪生命周期")
	}
	health, err := Health(ctx, app)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if !app.restoreControlReady || !app.journalRecovered {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域控制区或业务journal尚未恢复")
	}
	view, err := lifecycle.ReadRuntime(ctx, app.runtime)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if app.activeDataEpoch != "" && view.DataEpoch != app.activeDataEpoch {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "数据代次已变化，等待重启换入")
	}
	var epoch *string
	if app.activeDataEpoch != "" {
		value := string(app.activeDataEpoch)
		epoch = &value
	}
	return gen.ServiceReadiness{
		Health:                health,
		ActiveDataEpoch:       epoch,
		Initialized:           false,
		JournalRecovered:      app.journalRecovered,
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

// InstallBackgroundTasks只由成功完成完整应用构造的启动链调用，不在此运行任务。
func (a *Application) InstallBackgroundTasks(tasks []platform.BackgroundTask) {
	a.backgroundTasks = append([]platform.BackgroundTask(nil), tasks...)
}

// BackgroundTasks供统一进程模板启动/取消与等待；返回独立切片避免运行期改注册表。
func (a *Application) BackgroundTasks() []platform.BackgroundTask {
	return append([]platform.BackgroundTask(nil), a.backgroundTasks...)
}

// Drain仅排空HTTP，不关闭后台还可能写入的私有库；关闭顺序由平台保证。
func (a *Application) Drain(ctx context.Context) error {
	a.drainOnce.Do(func() {
		a.state.CompareAndSwap(stateReady, stateDraining)
		if err := a.httpServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				a.drainErr = platform.NewError(platform.CodeDeadlineExceeded, true, "资源入口未在关闭期限内排空")
			} else {
				a.drainErr = platform.NewError(platform.CodeInternal, false, "停止资源HTTP入口失败")
			}
		}
	})
	return a.drainErr
}
