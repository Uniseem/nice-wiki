// Package app 装配版本化服务边界；模型凭据及文档权限仍属于调用方。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/apivalidate"
	"nice-wiki/libs/platform/lifecycle"
	"nice-wiki/libs/platform/problem"
	"nice-wiki/libs/platform/zcode"
	"nice-wiki/modules/agent/internal/gen"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const ModuleSlug = "agent"

type Application struct {
	server        *http.Server
	manager       *zcode.Manager
	authority     platform.SiteConfigClient
	epoch         string
	once          sync.Once
	shutdownError error
}

func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*Application, error) {
	if config.Module != ModuleSlug || !filepath.IsAbs(config.DataRoot) || config.Listen == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "Agent 服务配置不完整")
	}
	directory := filepath.Join(config.DataRoot, "active")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "建立 Agent 运行目录失败")
	}
	anchors, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	authority, err := lifecycle.NewSiteConfigClient(config, key)
	if err != nil {
		return nil, err
	}
	view, err := lifecycle.ReadRuntime(ctx, authority)
	if err != nil {
		return nil, err
	}
	worker := os.Getenv("NW_ZCODE_WORKER")
	if worker == "" {
		worker = "/opt/agent/worker.mjs"
	}
	worker, err = filepath.Abs(worker)
	if err != nil {
		return nil, platform.NewError(platform.CodeValidation, false, "Agent 运行文件位置无效")
	}
	a := &Application{manager: zcode.NewManager(worker, directory), authority: authority, epoch: view.DataEpoch}
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, err
	}
	handlers := map[string]http.Handler{"health": http.HandlerFunc(a.Health), "ready": http.HandlerFunc(a.Ready), "startAgentRun": http.HandlerFunc(a.StartRun), "readAgentEvents": http.HandlerFunc(a.ReadEvents), "replyAgentRequest": http.HandlerFunc(a.Reply), "cancelAgentRun": http.HandlerFunc(a.Cancel)}
	callers := map[string][]string{}
	for _, name := range []string{"startAgentRun", "readAgentEvents", "replyAgentRequest", "cancelAgentRun"} {
		callers[name] = []string{"qa"}
	}
	handler, err := apivalidate.NewServiceHandler(spec, platform.ServiceBoundaryConfig{Self: ModuleSlug, Anchors: anchors, Runtime: func(work context.Context) (platform.RuntimeView, error) {
		return lifecycle.ReadRuntime(work, authority)
	}, OpenedDataEpoch: view.DataEpoch, AllowedCallers: callers, MaxBodyBytes: 64 << 20, MaintenanceOperations: map[string]bool{"readAgentEvents": true, "replyAgentRequest": true, "cancelAgentRun": true}}, handlers)
	if err != nil {
		return nil, err
	}
	a.server = &http.Server{Addr: config.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 64 << 10}
	return a, nil
}

func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error) {
	_, key, err := lifecycle.LoadServiceIdentity(config)
	if err != nil {
		return nil, err
	}
	return lifecycle.NewSiteConfigClient(config, key)
}

func (a *Application) Serve(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- a.server.ListenAndServe() }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return platform.NewError(platform.CodeInternal, false, "Agent HTTP 服务不可用")
	case <-ctx.Done():
		work, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return a.Shutdown(work)
	}
}

func (a *Application) Shutdown(ctx context.Context) error {
	a.once.Do(func() { _ = a.manager.Close(); a.shutdownError = a.server.Shutdown(ctx) })
	return a.shutdownError
}

func owner(r *http.Request) string {
	op, _ := platform.OperationContextFrom(r.Context())
	raw, _ := json.Marshal([]string{op.Actor.SiteID, op.DataEpoch, op.Actor.SubjectID, op.RootOperationID})
	return platform.HashBytes(raw)
}
func write(w http.ResponseWriter, value any, err error) {
	if err != nil {
		_ = problem.WriteProblem(w, problem.FromError(err, ""))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func (a *Application) StartRun(w http.ResponseWriter, r *http.Request) {
	var input zcode.Input
	_ = json.NewDecoder(r.Body).Decode(&input)
	op, _ := platform.OperationContextFrom(r.Context())
	deadline, _ := platform.ParseTimestamp(op.Deadline)
	id, err := a.manager.Start(owner(r), input, deadline)
	write(w, map[string]string{"runId": id}, err)
}
func (a *Application) ReadEvents(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.Atoi(r.URL.Query().Get("afterSequence"))
	events, err := a.manager.Read(r.Context(), owner(r), r.PathValue("runId"), after)
	write(w, events, err)
}
func (a *Application) Reply(w http.ResponseWriter, r *http.Request) {
	var input zcode.Reply
	_ = json.NewDecoder(r.Body).Decode(&input)
	err := a.manager.Reply(owner(r), r.PathValue("runId"), r.PathValue("requestId"), input)
	write(w, map[string]bool{"accepted": err == nil}, err)
}
func (a *Application) Cancel(w http.ResponseWriter, r *http.Request) {
	err := a.manager.Cancel(owner(r), r.PathValue("runId"))
	write(w, map[string]bool{"cancelled": err == nil}, err)
}
func (a *Application) Health(w http.ResponseWriter, r *http.Request) {
	write(w, gen.HealthStatus{Module: ModuleSlug, Version: "1", Status: "ready", Capabilities: []string{}}, nil)
}
func (a *Application) Ready(w http.ResponseWriter, r *http.Request) {
	if err := a.manager.Ready(); err != nil {
		write(w, nil, err)
		return
	}
	view, err := lifecycle.ReadRuntime(r.Context(), a.authority)
	if err != nil {
		write(w, nil, err)
		return
	}
	if view.DataEpoch != a.epoch {
		write(w, nil, platform.NewError(platform.CodeDependencyUnavailable, false, "Agent 运行代次待更新"))
		return
	}
	write(w, map[string]any{"health": gen.HealthStatus{Module: ModuleSlug, Version: "1", Status: "ready", Capabilities: []string{"zcode-native"}}, "activeDataEpoch": a.epoch, "runtimeRevision": view.Revision, "maintenance": view.Maintenance, "publicRequestsAllowed": !view.Maintenance, "initialized": view.DataEpoch != "", "journalRecovered": true, "safePoint": false}, nil)
}
