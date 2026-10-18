// Package transport 实现 data-protection 的 HTTP 生命周期、匿名健康/就绪，
// 以及尚未实现的备份、恢复和镜像路由的失败关闭。
package transport

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
 "strconv"
 "encoding/json"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/modules/data-protection/internal/gen"
 protection "nice-wiki/modules/data-protection/internal"
)

const (
	moduleName    = "data-protection"
	moduleVersion = "1"
	readyBudget   = 1500 * time.Millisecond
)

const (
	stateConstructed int32 = iota
	stateReady
	stateDraining
	stateClosed
)

// Dependencies 只保存生成客户端已经提供的生产方。没有客户端的域保持等待。
type Dependencies struct {
 Catalog *protection.Store
 JournalRecovered bool
 JournalControlReady bool
 RequiresMaintenance bool
	Identity   *gen.IdentityClient
	Jobs       *gen.JobsClient
	Content    *gen.ContentClient
	SiteConfig *gen.SiteConfigClient
}

// Application 是本域装配完成后的内部容器，不作为 HTTP DTO。
type Application struct {
	config              platform.ServiceConfig
	anchors             platform.TrustAnchors
	runtime             platform.SiteConfigClient
	dependencies        Dependencies
	serveStarted        atomic.Bool
	state               atomic.Int32
	shutdownOnce        sync.Once
	shutdownErr         error
	httpServer          *http.Server
	database            *sql.DB
	workerCancel        context.CancelFunc
	registeredHandlers  []string
	workerFunctionIds   []string
	activeDataEpoch     *string
	journalRecovered    bool
 maintenanceRequired bool
	restoreControlReady bool
	handlersFailClosed  bool
}

// 已绑定的 OpenAPI x-handler。业务函数仍未实现，对应路由失败关闭。
var boundHandlers = []string{
	"data-protection.Health",
	"data-protection.Ready",
	"data-protection.ListBackups",
	"data-protection.CreateBackup",
	"data-protection.ReadBackup",
	"data-protection.StreamPortableBackup",
	"data-protection.AuthorizeBackupDownload",
	"data-protection.ImportPortableBackup",
	"data-protection.EnterBarrier",
	"data-protection.ReleaseBarrier",
	"data-protection.ConfigureRemote",
	"data-protection.ScheduleLocalBackup",
	"data-protection.BuildMirror",
	"data-protection.ReadMirrorStatus",
	"data-protection.SyncMirror",
	"data-protection.CreateRestore",
	"data-protection.ServeRestoreArchive",
	"data-protection.ReadRestoreProgress",
	"data-protection.AbortRestore",
	"data-protection.CleanupRestore",
	"data-protection.ContinueRestore",
	"data-protection.ValidateRestore",
	"data-protection.RestoreDuringSetup",
	"data-protection.DiscardSnapshot",
	"data-protection.CreateSnapshot",
	"data-protection.StageSnapshot",
	"data-protection.ReadProtectionSettings",
}

// NewApplication 绑定本域全部生成路由。缺 handler 时无法实现 ServerInterface，编译即失败。
// 请求体按内嵌 OpenAPI 校验；备份、恢复和镜像处理函数统一失败关闭。
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtime platform.SiteConfigClient, clients Dependencies) (*Application, error) {
	if runtime == nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "site-config运行权威缺失")
	}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "本域内嵌接口规格不可读")
	}
	application := &Application{
		config:              config,
		anchors:             anchors,
		runtime:             runtime,
		dependencies:        clients,
		registeredHandlers:  append([]string(nil), boundHandlers...),
		workerFunctionIds:   []string{},
		journalRecovered:    clients.JournalRecovered,
  maintenanceRequired: clients.RequiresMaintenance,
		restoreControlReady: len(anchors.Services) > 0 && clients.JournalControlReady,
		handlersFailClosed:  true,
	}
 if clients.Catalog!=nil{epoch:=clients.Catalog.Epoch;application.activeDataEpoch=&epoch}
	mux := http.NewServeMux()
	routed := gen.HandlerWithOptions(&server{app: application}, gen.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			writeProblem(w, r, platform.NewError(platform.CodeValidation, false, "请求不符合接口规格"))
		},
	})
 handlers:=map[string]http.Handler{};callers:=map[string][]string{};services:=map[string][]string{};maintenance:=map[string]bool{}
 for _,item:=range spec.Paths.Map(){for _,operation:=range item.Operations(){id:=operation.OperationID;handlers[id]=routed;if id!="health"&&id!="ready"{callers[id]=[]string{"portal"}}}}
 for _,id:=range []string{"acquireBarrier","releaseBarrier","exportSnapshot","stageSnapshot","discardSnapshot"}{callers[id]=[]string{"data-protection"};services[id]=[]string{"data-protection"};maintenance[id]=true}
 callers["serveRestoreArchive"]=[]string{"identity","site-config","jobs","content","media","public-site","model-gateway","knowledge","qa","localization","memory","traffic-guard","analytics","data-protection"};services["serveRestoreArchive"]=callers["serveRestoreArchive"];maintenance["serveRestoreArchive"]=true
 for _,id:=range []string{"readRestoreProgress","continueRestore","abortRestore","cleanupRestore","readBackup","listBackups","readMirrorStatus","readProtectionSettings"}{maintenance[id]=true}
 opened:="";if clients.Catalog!=nil{opened=clients.Catalog.Epoch}
 protected,err:=apivalidate.NewServiceHandler(spec,platform.ServiceBoundaryConfig{Self:moduleName,Anchors:anchors,Runtime:func(ctx context.Context)(platform.RuntimeView,error){return lifecycle.ReadRuntime(ctx,runtime)},OpenedDataEpoch:opened,AllowedCallers:callers,ServiceActorCallers:services,MaintenanceOperations:maintenance,Authorize:apivalidate.NewIdentityAuthorizer(clients.Identity)},handlers);if err!=nil{return nil,err}
 application.httpServer = &http.Server{
		Addr:              config.Listen,
		Handler:           protected,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
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

// Serve 运行独立 HTTP 服务。重复启动返回 CONFLICT，不打开第二监听。
// 运行中的恢复监视由 platform.WatchRuntime 负责；本函数不查询模型。
func Serve(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeInternal, false, "应用未完成装配")
	}
	if app.state.Load() == stateClosed {
		return platform.NewError(platform.CodeInternal, false, "应用已关闭")
	}
	if !app.serveStarted.CompareAndSwap(false, true) {
		return platform.NewError(platform.CodeConflict, false, "同Application重复启动Serve")
	}
	app.state.Store(stateReady)
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

// Shutdown 拒绝新请求并等待在途 HTTP 响应。重复调用返回同一次结果。
// 本阶段没有数据库、journal 或 outbox；不删除 control 记录，也不把未实现的费用记为免费。
func Shutdown(ctx context.Context, app *Application) error {
	if app == nil || app.httpServer == nil {
		return platform.NewError(platform.CodeInternal, false, "应用未完成装配")
	}
	if app.state.Load() == stateClosed {
		return nil
	}
	app.shutdownOnce.Do(func() {
		app.state.CompareAndSwap(stateReady, stateDraining)
		app.state.CompareAndSwap(stateConstructed, stateDraining)
		if app.workerCancel != nil {
			app.workerCancel()
		}
		if err := app.httpServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				app.shutdownErr = platform.NewError(platform.CodeDeadlineExceeded, true, "关闭期限内未能安全停止在途操作")
				return
			}
			app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭HTTP服务失败")
			return
		}
  if app.dependencies.Catalog!=nil{if err:=app.dependencies.Catalog.Close();err!=nil{app.shutdownErr=err;return}}
		if app.database != nil {
			if err := app.database.Close(); err != nil {
				app.shutdownErr = platform.NewError(platform.CodeInternal, false, "关闭本域数据库失败")
				return
			}
		}
		app.state.Store(stateClosed)
	})
	return app.shutdownErr
}

// Health 只读本进程生命周期。不查询远程依赖、模型或数据库。
func Health(_ context.Context, app *Application) (gen.HealthStatus, error) {
	if app == nil || app.httpServer == nil {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭或服务循环不可用")
	}
	state := app.state.Load()
	if state == stateClosed || state == stateConstructed {
		return gen.HealthStatus{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程已关闭或服务循环不可用")
	}
	return gen.HealthStatus{
		Module:       moduleName,
		Version:      moduleVersion,
		Status:       "ready",
		Capabilities: []string{},
	}, nil
}

// Ready 确认 control 区、信任锚和运行权威。journal 或业务处理未实现时返回 503。
// 不把维护中的控制就绪报告成公开业务已开放。
func Ready(ctx context.Context, app *Application) (gen.ServiceReadiness, error) {
	if app == nil || app.state.Load() != stateReady {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "进程未处于可就绪生命周期")
	}
	if _, err := Health(ctx, app); err != nil {
		return gen.ServiceReadiness{}, err
	}
	if !app.restoreControlReady || len(app.anchors.Services) == 0 {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "服务身份信任锚或control区不可用")
	}
	readCtx, cancel := context.WithTimeout(ctx, readyBudget)
	defer cancel()
	view, err := lifecycle.ReadRuntime(readCtx, app.runtime)
	if err != nil {
		return gen.ServiceReadiness{}, err
	}
	if !app.journalRecovered {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域schema与journal尚未恢复")
	}
 if app.maintenanceRequired&&!view.Maintenance{return gen.ServiceReadiness{},platform.NewError(platform.CodeDependencyUnavailable,true,"未决控制journal要求保持维护，禁止正常开放")}
	if app.handlersFailClosed {
		return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "备份、恢复与镜像业务尚未实现")
	}
	if app.activeDataEpoch != nil {
		target := view.DataEpoch
		if view.TargetDataEpoch != "" {
			target = view.TargetDataEpoch
		}
		if *app.activeDataEpoch != target {
			return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "本域数据代次与运行权威不一致")
		}
	}
	// 平台运行视图没有修订号。不能用 0 冒充权威修订，也不能在此返回公开就绪。
	return gen.ServiceReadiness{}, platform.NewError(platform.CodeDependencyUnavailable, true, "运行权威修订号不可用")
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

func (s *server) ListBackups(w http.ResponseWriter, r *http.Request, _ gen.ListBackupsParams) {
 op,ok:=platform.OperationContextFrom(r.Context());if !ok{writeProblem(w,r,platform.NewError(platform.CodeUnauthenticated,false,"缺少可信操作上下文"));return};limit:=20;if raw:=r.URL.Query().Get("limit");raw!=""{limit,_=strconv.Atoi(raw)}
 result,err:=protection.ListBackups(r.Context(),op,protection.ListBackupsInput{Cursor:r.URL.Query().Get("cursor"),Limit:limit,Actor:op.Actor});if err!=nil{writeProblem(w,r,err);return};w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");_ = json.NewEncoder(w).Encode(result)
}

func (s *server) CreateBackup(w http.ResponseWriter, r *http.Request, _ gen.CreateBackupParams) {
	writeUnavailable(w, r)
}

func (s *server) ReadBackup(w http.ResponseWriter, r *http.Request, id string, _ gen.ReadBackupParams) {
 op,ok:=platform.OperationContextFrom(r.Context());if !ok{writeProblem(w,r,platform.NewError(platform.CodeUnauthenticated,false,"缺少可信操作上下文"));return};result,err:=protection.ReadBackup(r.Context(),op,protection.ReadBackupInput{BackupId:id,Actor:op.Actor});if err!=nil{writeProblem(w,r,err);return};w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");_ = json.NewEncoder(w).Encode(result)
}

func (s *server) StreamPortableBackup(w http.ResponseWriter, r *http.Request, _ string, _ gen.StreamPortableBackupParams) {
	writeUnavailable(w, r)
}

func (s *server) AuthorizeBackupDownload(w http.ResponseWriter, r *http.Request, _ string, _ gen.AuthorizeBackupDownloadParams) {
	writeUnavailable(w, r)
}

func (s *server) ImportPortableBackup(w http.ResponseWriter, r *http.Request, _ gen.ImportPortableBackupParams) {
	writeUnavailable(w, r)
}

func (s *server) AcquireBarrier(w http.ResponseWriter, r *http.Request, _ gen.AcquireBarrierParams) {
	writeUnavailable(w, r)
}

func (s *server) ReleaseBarrier(w http.ResponseWriter, r *http.Request, _ string, _ gen.ReleaseBarrierParams) {
	writeUnavailable(w, r)
}

func (s *server) ConfigureRemote(w http.ResponseWriter, r *http.Request, _ gen.ConfigureRemoteParams) {
 op,ok:=platform.OperationContextFrom(r.Context());if !ok{writeProblem(w,r,platform.NewError(platform.CodeUnauthenticated,false,"缺少可信操作上下文"));return};var body gen.ConfigureRemoteRequestBody;if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{writeProblem(w,r,platform.NewError(platform.CodeValidation,false,"远程配置正文不可读"));return};result,err:=protection.ConfigureRemote(r.Context(),op,protection.ConfigureRemoteInput{URL:body.Url,Branch:body.Branch,CredentialRef:body.CredentialRef,ExpectedRevision:body.ExpectedRevision,Actor:op.Actor});if err!=nil{writeProblem(w,r,err);return};w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");_ = json.NewEncoder(w).Encode(result)
}

func (s *server) ScheduleLocalBackup(w http.ResponseWriter, r *http.Request, _ gen.ScheduleLocalBackupParams) {
	writeUnavailable(w, r)
}

func (s *server) BuildMirror(w http.ResponseWriter, r *http.Request, _ gen.BuildMirrorParams) {
	writeUnavailable(w, r)
}

func (s *server) ReadMirrorStatus(w http.ResponseWriter, r *http.Request) {
 op,ok:=platform.OperationContextFrom(r.Context());if !ok{writeProblem(w,r,platform.NewError(platform.CodeUnauthenticated,false,"缺少可信操作上下文"));return};result,err:=protection.ReadMirrorStatus(r.Context(),op,protection.ReadMirrorStatusInput{Actor:op.Actor});if err!=nil{writeProblem(w,r,err);return};w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");_ = json.NewEncoder(w).Encode(result)
}

func (s *server) SyncMirror(w http.ResponseWriter, r *http.Request, _ string, _ gen.SyncMirrorParams) {
	writeUnavailable(w, r)
}

func (s *server) CreateRestore(w http.ResponseWriter, r *http.Request, _ gen.CreateRestoreParams) {
	writeUnavailable(w, r)
}

func (s *server) ServeRestoreArchive(w http.ResponseWriter, r *http.Request, _ string, _ string, _ gen.ServeRestoreArchiveParams) {
	writeUnavailable(w, r)
}

func (s *server) ReadRestoreProgress(w http.ResponseWriter, r *http.Request, _ string, _ gen.ReadRestoreProgressParams) {
	writeUnavailable(w, r)
}

func (s *server) AbortRestore(w http.ResponseWriter, r *http.Request, _ string, _ gen.AbortRestoreParams) {
	writeUnavailable(w, r)
}

func (s *server) CleanupRestore(w http.ResponseWriter, r *http.Request, _ string, _ gen.CleanupRestoreParams) {
	writeUnavailable(w, r)
}

func (s *server) ContinueRestore(w http.ResponseWriter, r *http.Request, _ string, _ gen.ContinueRestoreParams) {
	writeUnavailable(w, r)
}

func (s *server) ValidateRestore(w http.ResponseWriter, r *http.Request, _ gen.ValidateRestoreParams) {
	writeUnavailable(w, r)
}

func (s *server) ReadProtectionSettings(w http.ResponseWriter, r *http.Request) {
 op,ok:=platform.OperationContextFrom(r.Context());if !ok{writeProblem(w,r,platform.NewError(platform.CodeUnauthenticated,false,"缺少可信操作上下文"));return};result,err:=protection.ReadProtectionSettings(r.Context(),op,protection.ReadProtectionSettingsInput{});if err!=nil{writeProblem(w,r,err);return};w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");_ = json.NewEncoder(w).Encode(result)
}

func (s *server) RestoreDuringSetup(w http.ResponseWriter, r *http.Request, _ gen.RestoreDuringSetupParams) {
	writeUnavailable(w, r)
}

func (s *server) DiscardSnapshot(w http.ResponseWriter, r *http.Request, _ gen.DiscardSnapshotParams) {
	writeUnavailable(w, r)
}

func (s *server) ExportSnapshot(w http.ResponseWriter, r *http.Request, _ gen.ExportSnapshotParams) {
	writeUnavailable(w, r)
}

func (s *server) StageSnapshot(w http.ResponseWriter, r *http.Request, _ gen.StageSnapshotParams) {
	writeUnavailable(w, r)
}

func writeUnavailable(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, platform.NewError(platform.CodeDependencyUnavailable, true, "备份、恢复与镜像业务尚未实现"))
}

func writeProblem(w http.ResponseWriter, r *http.Request, err error) {
	requestID := platform.Identifier(r.Header.Get(platform.HeaderRequestID))
	if !platform.ValidIdentifier(requestID) {
		requestID = platform.NewIdentifier()
	}
	_ = problem.WriteProblem(w, problem.FromError(err, requestID))
}
