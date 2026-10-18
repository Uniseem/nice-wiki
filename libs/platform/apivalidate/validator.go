// Package apivalidate 在 HTTP 入口按本域生成代码内嵌的 OpenAPI 规格统一校验请求
// （ADR0005 kin-openapi；ADR0006 边界一次校验）。入口之后的函数信任已校验的输入。
package apivalidate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/opctx"
	"nice-wiki/libs/platform/problem"
)

// maxIssues 限制一次返回的字段问题数量。
const maxIssues = 20

// NewIdentityAuthorizer 只适配版本化即时授权，不复制会话或权限权威。
func NewIdentityAuthorizer(client platform.AuthorizationClient) func(context.Context, platform.OperationContext, string, string) error {
	return func(ctx context.Context, op platform.OperationContext, permission, action string) error {
		raw, err := client.CheckAuthorization(platform.WithOperationContext(ctx, op), map[string]any{
			"subjectId": op.Actor.SubjectID, "sessionId": op.Actor.SessionID, "siteId": op.Actor.SiteID,
			"permission": permission, "action": action, "resourceId": nil, "expectedPermissionRevision": op.Actor.PermissionRevision,
		})
		if err != nil {
			return err
		}
		var result struct {
			Authorized *bool `json:"authorized"`
		}
		if json.Unmarshal(raw, &result) != nil || result.Authorized == nil {
			return platform.NewError(platform.CodeDependencyUnavailable, true, "用户授权响应不可读")
		}
		if !*result.Authorized {
			return platform.NewError(platform.CodeForbidden, false, "当前账户无此操作权限")
		}
		return nil
	}
}

// NewServiceHandler 装配完整版本化接口；不完整handler/调用方策略在启动时失败。
// 健康与就绪保持匿名，其余接口均先验服务身份，再一次校验请求schema。
func NewServiceHandler(spec *openapi3.T, config platform.ServiceBoundaryConfig, handlers map[string]http.Handler) (http.Handler, error) {
	if spec == nil || config.Self == "" || config.Runtime == nil || config.MaxBodyBytes < 0 {
		return nil, platform.NewError(platform.CodeValidation, false, "服务入口配置不完整")
	}
	validate, err := NewRequestValidator(spec)
	if err != nil {
		return nil, err
	}
	router, err := legacyrouter.NewRouter(spec, openapi3.DisableExamplesValidation(), openapi3.DisableSchemaDefaultsValidation())
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "服务入口路由不可读")
	}
	knownOperations:=map[string]bool{}
 for _, item := range spec.Paths.Map() {
		for _, operation := range item.Operations() {
			id := operation.OperationID
 knownOperations[id]=true
			if id == "" || handlers[id] == nil {
				return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "存在未绑定的接口处理函数")
			}
			if id == "health" || id == "ready" {
				continue
			}
			if len(config.AllowedCallers[id]) == 0 {
				return nil, platform.NewError(platform.CodeIntegrityError, false, "接口缺少固定调用服务策略")
			}
			if !config.ControllerOperations[id]&&config.AdmitBusiness==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"非控制HTTP独立业务scope未装配")}
 permission, _ := operation.Extensions["x-permission"].(string)
			if permission != "anonymous" && permission != "service" && config.Authorize == nil {
				return nil, platform.NewError(platform.CodeDependencyUnavailable, false, "用户权威授权回调未装配")
			}
		}
	}
 for id,policy:=range config.CleanupOperations{if !knownOperations[id]||config.ControllerOperations[id]||len(policy.Callers)==0||config.AdmitCleanup==nil||config.AuthorizeCleanup==nil{return nil,platform.NewError(platform.CodeDependencyUnavailable,false,"固定清理端口、归属授权或私有lease未完整装配")}}
	for id:=range config.ControllerOperations{if !knownOperations[id]{return nil,platform.NewError(platform.CodeIntegrityError,false,"控制操作分类不属于真实OpenAPI操作")}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, params, err := router.FindRoute(r)
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeNotFound, false, "接口路径不存在"), platform.NewIdentifier()))
			return
		}
		id := route.Operation.OperationID
		for name, value := range params {
			r.SetPathValue(name, value)
		}
		handler := handlers[id]
		if id == "health" || id == "ready" {
			validate(handler).ServeHTTP(w, r)
			return
		}
		limit := config.MaxBodyBytes
		if limit == 0 {
			limit = 16 << 20
		}
		if declared := config.OperationBodyLimits[id]; declared > 0 {
			limit = declared
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		view, err := config.Runtime(r.Context())
		if err != nil {
			_ = problem.WriteProblem(w, problem.FromError(err, platform.NewIdentifier()))
			return
		}
		if config.OpenedDataEpoch != "" && view.DataEpoch != config.OpenedDataEpoch {
			_ = problem.WriteProblem(w, problem.FromError(platform.NewError(platform.CodeVersionMismatch, false, "进程数据代次已变化，等待重启"), platform.NewIdentifier()))
			return
		}
		op, err := opctx.BuildOperationContext(r, config.Anchors, config.Self, view)
		if key := r.Header.Get("Idempotency-Key"); err == nil && key != "" {
			if !platform.ValidIdentifier(key) {
				err = platform.NewError(platform.CodeValidation, false, "请求幂等键不合法")
			} else {
				op.OperationID = key
				if r.Header.Get(platform.HeaderRootOperationID) == "" {
					op.RootOperationID = key
				}
			}
		}
		if err == nil && !slices.Contains(config.AllowedCallers[id], op.CallerService) {
			err = platform.NewError(platform.CodeForbidden, false, "该服务不允许调用此接口")
		}
		if err == nil && (view.Maintenance || view.Phase != platform.PhaseRunning) && !config.MaintenanceOperations[id] && op.CleanupScope==nil {
			err = platform.NewError(platform.CodeLocked, true, "当前维护阶段不开放该接口")
		}
		if err != nil {
			requestID := op.RequestID
			if requestID == "" {
				requestID = platform.NewIdentifier()
			}
			_ = problem.WriteProblem(w, problem.FromError(err, requestID))
			return
		}
		deadline, _ := platform.ParseTimestamp(op.Deadline)
		ctx, cancel := context.WithDeadline(r.Context(), deadline)
		defer cancel()
		ctx = platform.WithOperationContext(ctx, op)
		if proof := r.Header.Get(platform.HeaderPortalProof); proof != "" {
			ctx = platform.WithPortalProof(ctx, proof)
		}
		permission, _ := route.Operation.Extensions["x-permission"].(string)
		if permission != "anonymous" && permission != "service" {
			serviceAllowed := op.Actor.Kind == "service" && slices.Contains(config.ServiceActorCallers[id], op.CallerService)
			if !serviceAllowed {
				if op.Actor.Kind != "admin" || op.Actor.SessionID == "" || op.Actor.SessionRevision < 1 {
					err = platform.NewError(platform.CodeUnauthenticated, false, "此接口要求当前后台会话")
				} else {
					err = config.Authorize(ctx, op, permission, config.Self+":"+id)
				}
			}
			if err != nil {
				requestID := op.RequestID
				if requestID == "" {
					requestID = platform.NewIdentifier()
				}
				_ = problem.WriteProblem(w, problem.FromError(err, requestID))
				return
			}
		}
		validatedScope:=http.HandlerFunc(func(w http.ResponseWriter,request *http.Request){
 if scope:=op.CleanupScope;scope!=nil{
 policy,ok:=config.CleanupOperations[id];if !ok||!slices.Contains(policy.Callers,op.CallerService)||scope.Action!=config.Self+":"+id||!view.Maintenance||(view.Phase!=platform.PhaseBackupBarrier&&view.Phase!=platform.PhaseRestoreStaging)||scope.BarrierID!=view.BarrierID{_=problem.WriteProblem(w,problem.FromError(platform.NewError(platform.CodeForbidden,false,"清理不属于固定本域旧引用操作"),op.RequestID));return}
 body,readErr:=io.ReadAll(request.Body);if readErr!=nil{_=problem.WriteProblem(w,problem.FromError(readErr,op.RequestID));return};request.Body=io.NopCloser(bytes.NewReader(body));resource,scopeErr:=platform.CleanupResourceID(policy.Resource,params,body);if scopeErr==nil&&resource!=scope.ResourceID{scopeErr=platform.NewError(platform.CodeForbidden,false,"清理资源与签名不符")};if scopeErr==nil{scopeErr=config.AuthorizeCleanup(request.Context(),op,*scope,resource)};if scopeErr!=nil{_=problem.WriteProblem(w,problem.FromError(scopeErr,op.RequestID));return};scoped,release,scopeErr:=config.AdmitCleanup(request.Context(),op,*scope);if scopeErr!=nil{_=problem.WriteProblem(w,problem.FromError(scopeErr,op.RequestID));return};defer release();handler.ServeHTTP(w,request.WithContext(scoped));return
 }
 if config.ControllerOperations[id]{handler.ServeHTTP(w,request);return}
 scoped,release,scopeErr:=config.AdmitBusiness(request.Context());if scopeErr!=nil{_=problem.WriteProblem(w,problem.FromError(scopeErr,op.RequestID));return};defer release();handler.ServeHTTP(w,request.WithContext(scoped))
 })
 validate(validatedScope).ServeHTTP(w, r.WithContext(ctx))
	}), nil
}

// ServeEventStream 将单条增量事件写入SSE；读取、写入与心跳共享请求取消。
// 返回时流已经关闭；响应开始后调用方只记录错误码，不再写JSON错误。
func ServeEventStream(ctx context.Context, w http.ResponseWriter, stream platform.EventStream) error {
	return ServeJSONEventStream(ctx, w, envelopeJSONStream{source: stream})
}

type envelopeJSONStream struct{ source platform.EventStream }

func (s envelopeJSONStream) Next(ctx context.Context) (platform.SSEFrame, error) {
	event, err := s.source.Next(ctx)
	if err != nil {
		return platform.SSEFrame{}, err
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return platform.SSEFrame{}, platform.NewError(platform.CodeIntegrityError, false, "事件无法编码")
	}
	return platform.SSEFrame{ID: s.source.LastID(), Event: "message", Data: raw}, nil
}

func (s envelopeJSONStream) Close() error { return s.source.Close() }

// ServeJSONEventStream 统一业务JSON流的SSE帧、心跳与背压；不增加领域包装。
// 返回前取消读取并关闭流，响应开始后由调用方结束连接，不追加Problem。
func ServeJSONEventStream(ctx context.Context, w http.ResponseWriter, stream platform.JSONEventStream) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() { cancel(); _ = stream.Close() }()
	type eventResult struct {
		frame platform.SSEFrame
		err   error
	}
	next := make(chan eventResult)
	go func() {
		for {
			frame, err := stream.Next(ctx)
			result := eventResult{frame: frame, err: err}
			select {
			case next <- result:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	var deadlineMu sync.Mutex
	setWriteDeadline := func() error {
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		deadline := time.Now().Add(30 * time.Second)
		if requested, ok := ctx.Deadline(); ok && requested.Before(deadline) {
			deadline = requested
		}
		if ctx.Err() != nil {
			deadline = time.Now()
		}
		return controller.SetWriteDeadline(deadline)
	}
	writeCancelled := make(chan struct{})
	stopWriteCancellation := context.AfterFunc(ctx, func() {
		defer close(writeCancelled)
		deadlineMu.Lock()
		defer deadlineMu.Unlock()
		_ = controller.SetWriteDeadline(time.Now())
	})
	defer func() {
		if !stopWriteCancellation() {
			<-writeCancelled
		}
	}()
	if err := setWriteDeadline(); err != nil {
		return platform.NewError(platform.CodeDependencyUnavailable, false, "事件响应不支持写截止")
	}
	if err := controller.Flush(); err != nil {
		return platform.NewError(platform.CodeCancelled, false, "事件连接已断开")
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := setWriteDeadline(); err != nil {
				return platform.NewError(platform.CodeCancelled, false, "事件连接写截止不可用")
			}
			if _, err := io.WriteString(w, ": heartbeat\n\n"); err != nil {
				return platform.NewError(platform.CodeCancelled, false, "事件连接已断开")
			}
			if err := controller.Flush(); err != nil {
				return platform.NewError(platform.CodeCancelled, false, "事件连接已断开")
			}
		case result := <-next:
			if errors.Is(result.err, io.EOF) {
				return nil
			}
			if result.err != nil {
				return result.err
			}
			frame := result.frame
			if len(frame.ID) > 4096 || strings.ContainsAny(frame.ID, "\x00\r\n") {
				return platform.NewError(platform.CodeIntegrityError, false, "事件游标不合法")
			}
			if !platform.ValidIdentifier(frame.Event) {
				return platform.NewError(platform.CodeIntegrityError, false, "事件名不合法")
			}
			if len(frame.Data) > 16<<20 {
				return platform.NewError(platform.CodeBudgetExceeded, false, "事件帧超过载荷上限")
			}
			var raw bytes.Buffer
			if err := json.Compact(&raw, frame.Data); err != nil {
				return platform.NewError(platform.CodeIntegrityError, false, "事件无法编码")
			}
			if err := setWriteDeadline(); err != nil {
				return platform.NewError(platform.CodeCancelled, false, "事件连接写截止不可用")
			}
			if _, err := fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", frame.ID, frame.Event, raw.Bytes()); err != nil {
				return platform.NewError(platform.CodeCancelled, false, "事件连接已断开")
			}
			if err := controller.Flush(); err != nil {
				return platform.NewError(platform.CodeCancelled, false, "事件连接已断开")
			}
		}
	}
}

// NewPayloadValidator 复用内嵌自包含规格校验持久消息载荷；不会联网取引用。
// schema错误在启动时失败，载荷错误只返回安全说明，不回显取值。
func NewPayloadValidator(document []byte, schemaName string) (func(json.RawMessage) error, error) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(document)
	if err != nil || spec.Components == nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "消息载荷规格不可读")
	}
	ref := spec.Components.Schemas[schemaName]
	if ref == nil || ref.Value == nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "消息载荷schema未登记")
	}
	if err := ref.Value.Validate(context.Background()); err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "消息载荷schema不合法")
	}
	return func(raw json.RawMessage) error {
		var value any
		if json.Unmarshal(raw, &value) != nil || ref.Value.VisitJSON(value) != nil {
			return platform.NewError(platform.CodeValidation, false, "消息载荷不符合登记schema")
		}
		return nil
	}, nil
}

// NewRequestValidator 用本域生成代码 gen.GetSwagger() 返回的规格构建请求校验中间件。
// 路径、查询、请求头参数与请求体按规格校验（additionalProperties:false 拒绝未知字段，
// 长度、范围、枚举与必填按 schema），失败返回 VALIDATION Problem，字段问题只给位置与
// schema 关键字，不回显取值。服务身份、委托与运行代次由 opctx.BuildOperationContext
// 校验，这里不重复；规格未登记的路径交给后续路由返回 404/405。
func NewRequestValidator(spec *openapi3.T) (func(http.Handler) http.Handler, error) {
	if spec == nil {
		return nil, platform.NewError(platform.CodeValidation, false, "接口规格缺失")
	}
	// 内嵌规格的 servers 是部署地址，入口只按路径匹配。
	spec.Servers = nil
	router, err := legacyrouter.NewRouter(spec, openapi3.DisableExamplesValidation(), openapi3.DisableSchemaDefaultsValidation())
	if err != nil {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "接口规格无法建立校验路由")
	}
	options := &openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		MultiError:         true,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, params, err := router.FindRoute(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			input := &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: options}
			if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
				requestID := platform.Identifier(r.Header.Get(platform.HeaderRequestID))
				if !platform.ValidIdentifier(requestID) {
					requestID = platform.NewIdentifier()
				}
				p := problem.FromError(platform.NewError(platform.CodeValidation, false, "请求不符合接口规格"), requestID)
				p.Issues = fieldIssues(err)
				_ = problem.WriteProblem(w, p)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// fieldIssues 把校验错误转为字段位置与原因，不回显请求取值。
func fieldIssues(err error) []platform.FieldIssue {
	errs := []error{err}
	var multi openapi3.MultiError
	if errors.As(err, &multi) {
		errs = multi
	}
	issues := make([]platform.FieldIssue, 0, len(errs))
	for _, e := range errs {
		field, reason := "request", "invalid"
		var requestErr *openapi3filter.RequestError
		if errors.As(e, &requestErr) {
			switch {
			case requestErr.Parameter != nil:
				field = requestErr.Parameter.In + "." + requestErr.Parameter.Name
			case requestErr.RequestBody != nil:
				field = "body"
			}
		}
		var schemaErr *openapi3.SchemaError
		if errors.As(e, &schemaErr) {
			if pointer := schemaErr.JSONPointer(); len(pointer) > 0 {
				field += "/" + strings.Join(pointer, "/")
			}
			if schemaErr.SchemaField != "" {
				reason = schemaErr.SchemaField
			}
		}
		issues = append(issues, platform.FieldIssue{Field: field, Reason: reason})
		if len(issues) == maxIssues {
			break
		}
	}
	return issues
}
