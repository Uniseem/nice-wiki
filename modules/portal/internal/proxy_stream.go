package internal

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// ProxyRequest 按固定顺序执行入口准入、权限预检和一次有界转发。
func ProxyRequest(ctx context.Context, op OperationContext, input ProxyRequestInput) (ProxyRequestResult, error) {
	ctx = platform.WithOperationContext(ctx, op)
	runtime, ok := proxyRuntimeFromContext(ctx)
	if !ok || runtime.Request == nil || runtime.Writer == nil {
		return ProxyRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "入口代理运行时未装配")
	}
	routeResult, err := ResolveRoute(ResolveRouteInput{Registry: runtime.Registry, Method: input.Method, EscapedPath: input.RawPath})
	if err != nil {
		return ProxyRequestResult{}, err
	}
	route := routeResult.Route
	gate, err := CheckEntranceGate(ctx, op, CheckEntranceGateInput{Route: route, RuntimeRevision: 0})
	if err != nil {
		return ProxyRequestResult{}, err
	}
	if !gate.Allowed {
		return ProxyRequestResult{}, platform.NewError(platform.CodeLocked, true, "站点正在维护")
	}
	bodyLimit := route.Definition.MaxBodyBytes
	if bodyLimit == 0 {
		bodyLimit = 16 << 20
	}
	headers := runtime.Headers
	validated, err := ValidateEnvelope(ctx, op, ValidateEnvelopeInput{
		Request: IncomingRequest{Method: input.Method, EscapedPath: input.RawPath, Headers: headers, BodyHandle: platform.NewIdentifier()},
		Limits:  RequestLimits{BodyBytes: bodyLimit, HeaderBytes: 64 << 10, QueryBytes: 8 << 10},
	})
	if err != nil {
		return ProxyRequestResult{}, err
	}
	validated.Envelope.BodyBytes = int64(len(runtime.Body))
	validated.Envelope.BodyDigest = platform.HashBytes(runtime.Body)
	if validated.Envelope.BodyBytes > bodyLimit {
		return ProxyRequestResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "请求体超出路由上限")
	}

	requestHasSession := cookieValue(runtime.Request, "nw_sid") != ""
	isWrite := input.Method != http.MethodGet && input.Method != http.MethodHead
	origin := runtime.Request.Header.Get("Origin")
	// 写入请求必须带完整且同源的 Origin；带会话的读取请求在浏览器同源 GET
	// 不发送 Origin 时放行，带了 Origin 就必须同源（会话 cookie 为 SameSite=Strict）。
	if isWrite || (requestHasSession && origin != "") {
		if _, err := AssertSameOrigin(AssertSameOriginInput{
			Origin: origin, Host: runtime.Request.Host, ConfiguredOrigins: runtime.ConfiguredOrigins,
		}); err != nil {
			return ProxyRequestResult{}, platform.NewError(platform.CodeForbidden, false, "请求来源不受信任")
		}
	}
	if requestHasSession && isWrite && route.Definition.Permission.Permission != "anonymous" {
		// 合同只定义 X-CSRF-Token 请求头，由 identity 核对其与会话绑定；
		// 没有可供双提交比对的独立 CSRF cookie，不再拿请求头与自身比较。
		_, err := VerifyCSRF(WithProxyRuntime(withOrigin(ctx, origin), runtime), op, VerifyCSRFInput{
			Supplied: runtime.Request.Header.Get("X-CSRF-Token"),
			Cookies:  SessionCookies{SessionToken: cookieValue(runtime.Request, "nw_sid")},
		})
		if err != nil {
			return ProxyRequestResult{}, err
		}
	}

	var proof InternalProof
	if route.Definition.Permission.Permission != "anonymous" {
		required := route.Definition.Permission
		required.Action = route.Definition.Owner + ":" + route.Definition.UpstreamOperationID
		authResult, authErr := ResolveAuthContext(ctx, op, ResolveAuthContextInput{
			Cookies:  SessionCookies{SessionToken: cookieValue(runtime.Request, "nw_sid"), CSRFCookie: runtime.Request.Header.Get("X-CSRF-Token")},
			Required: required,
		})
		if authErr != nil {
			return ProxyRequestResult{}, authErr
		}
		if _, err := AuthorizeRoute(AuthorizeRouteInput{Auth: authResult.Auth, Route: route}); err != nil {
			return ProxyRequestResult{}, err
		}
		proofResult, proofErr := BuildInternalProof(WithPortalSigner(ctx, runtime.DelegationKey), op, BuildInternalProofInput{
			Auth: inputAuth(authResult.Auth), Route: route, RequestDigest: validated.Envelope.BodyDigest,
			ExpiresAt: proofExpiry(op, authResult.Auth),
		})
		if proofErr != nil {
			return ProxyRequestResult{}, proofErr
		}
		proof = proofResult.Proof
	}
	if route.Definition.Admission != "" {
		return ProxyRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "公开准入服务尚未装配")
	}
	normalized, err := NormalizeForwardHeaders(NormalizeForwardHeadersInput{Headers: headers, Envelope: validated.Envelope})
	if err != nil {
		return ProxyRequestResult{}, err
	}
	runtime.Headers = normalized.Headers
	forwarded, err := ForwardRequest(WithProxyRuntime(ctx, runtime), op, ForwardRequestInput{Route: route, Request: validated.Envelope, Proof: proof})
	if err != nil {
		return ProxyRequestResult{}, err
	}
	if _, err := StreamResponse(ctx, op, StreamResponseInput{Upstream: forwarded.Response, WriterHandle: platform.NewIdentifier()}); err != nil {
		return ProxyRequestResult{}, err
	}
	return ProxyRequestResult{Status: forwarded.Response.Status, Owner: route.Definition.Owner}, nil
}

// ForwardRequest 只把路由解析结果拼接到受配置约束的依赖基址，禁止
// 请求把任意 URL 带入上游。
func ForwardRequest(ctx context.Context, op OperationContext, input ForwardRequestInput) (ForwardRequestResult, error) {
	runtime, ok := proxyRuntimeFromContext(ctx)
	if !ok || runtime.Request == nil {
		return ForwardRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "上游代理运行时未装配")
	}
	base, ok := runtime.Dependencies[input.Route.Definition.Owner]
	if !ok || base == "" {
		return ForwardRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有服务依赖未配置")
	}
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return ForwardRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有服务地址不可用")
	}
	target := strings.TrimRight(base, "/") + input.Route.RelativePath
	if runtime.Query != "" {
		if strings.ContainsAny(runtime.Query, "#\r\n") || len(runtime.Query) > 8192 {
			return ForwardRequestResult{}, platform.NewError(platform.CodeValidation, false, "查询参数不合法")
		}
		target += "?" + runtime.Query
	}
	requestCtx, cancel := context.WithCancel(ctx)
	if input.Route.Definition.TimeoutMS > 0 {
		cancel()
		requestCtx, cancel = context.WithTimeout(ctx, time.Duration(input.Route.Definition.TimeoutMS)*time.Millisecond)
	}
	request, err := http.NewRequestWithContext(requestCtx, input.Route.Definition.Method, target, bytes.NewReader(runtime.Body))
	if err != nil {
		cancel()
		return ForwardRequestResult{}, platform.NewError(platform.CodeValidation, false, "上游请求构造失败")
	}
	applyHeaderMap(request.Header, runtime.Headers)
	if input.Proof.Token != "" {
		request.Header.Set(platform.HeaderPortalProof, input.Proof.Token)
	}
	request.Header.Set("Accept", runtime.Request.Header.Get("Accept"))
	if runtime.SigningKey.KeyID == "" {
		cancel()
		return ForwardRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "入口服务密钥未装配")
	}
	client := &http.Client{Transport: serviceauth.SigningTransport("portal", input.Route.Definition.Owner, runtime.SigningKey, nil),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	request = request.WithContext(platform.WithOperationContext(request.Context(), op))
	response, err := client.Do(request)
	if err != nil {
		contextErr := requestCtx.Err()
		cancel()
		if contextErr == context.Canceled {
			return ForwardRequestResult{}, platform.NewError(platform.CodeCancelled, false, "上游请求已取消")
		}
		if contextErr == context.DeadlineExceeded {
			return ForwardRequestResult{}, platform.NewError(platform.CodeDeadlineExceeded, true, "上游请求超时")
		}
		return ForwardRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有服务不可达")
	}
	return ForwardRequestResult{Response: UpstreamResponse{
		Status:       response.StatusCode,
		Headers:      headerMapFromHTTP(response.Header),
		StreamHandle: platform.NewIdentifier(),
		Response:     response,
		Cancel:       cancel,
	}}, nil
}

// StreamResponse 以io.Copy提供背压；浏览器断开时请求context取消并关闭上游。
func StreamResponse(ctx context.Context, op OperationContext, input StreamResponseInput) (StreamResponseResult, error) {
	runtime, ok := proxyRuntimeFromContext(ctx)
	if !ok || runtime.Writer == nil || input.Upstream.Response == nil || input.WriterHandle == "" {
		return StreamResponseResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "响应流运行时未装配")
	}
	response := input.Upstream.Response
	defer input.Upstream.Cancel()
	defer response.Body.Close()
	for _, entry := range input.Upstream.Headers.Entries {
		if !responseHeaderAllowed(entry.Name) {
			continue
		}
		for _, value := range entry.Values {
			runtime.Writer.Header().Add(entry.Name, value)
		}
	}
	if strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		runtime.Writer.Header().Set("X-Accel-Buffering", "no")
	}
	runtime.Writer.WriteHeader(response.StatusCode)
	if runtime.Request.Method == http.MethodHead {
		return StreamResponseResult{Accepted: true}, nil
	}
	if _, err := io.Copy(runtime.Writer, response.Body); err != nil {
		if ctx.Err() != nil {
			return StreamResponseResult{}, platform.NewError(platform.CodeCancelled, true, "客户端已断开")
		}
		return StreamResponseResult{}, platform.NewError(platform.CodeInternal, false, "转发响应失败")
	}
	return StreamResponseResult{Accepted: true}, nil
}

func MapUpstreamFailure(input MapUpstreamFailureInput) (MapUpstreamFailureResult, error) {
	if input.RequestID == "" || input.FailureCode == "" {
		return MapUpstreamFailureResult{}, platform.NewError(platform.CodeValidation, false, "上游失败字段不完整")
	}
	code := input.FailureCode
	if _, ok := platform.ProblemHTTPStatus[code]; !ok {
		code = platform.CodeInternal
	}
	return MapUpstreamFailureResult{Problem: platform.Problem{
		Type: "about:blank", Title: "请求未能完成", Code: code,
		Status: platform.ProblemHTTPStatus[code], Detail: "上游请求未能完成。",
		RequestID: input.RequestID, Retryable: code == platform.CodeDependencyUnavailable || code == platform.CodeDeadlineExceeded,
	}}, nil
}

func applyHeaderMap(headers http.Header, values HeaderMap) {
	for _, entry := range values.Entries {
		for _, value := range entry.Values {
			headers.Add(entry.Name, value)
		}
	}
}

func headerMapFromHTTP(headers http.Header) HeaderMap {
	result := HeaderMap{Entries: make([]HeaderEntry, 0, len(headers))}
	for name, values := range headers {
		if !responseHeaderAllowed(name) {
			continue
		}
		result.Entries = append(result.Entries, HeaderEntry{Name: name, Values: append([]string(nil), values...)})
	}
	return result
}

func responseHeaderAllowed(name string) bool {
	switch strings.ToLower(name) {
	case "content-type", "content-length", "content-range", "accept-ranges", "etag", "last-modified", "cache-control", "location", "retry-after", "www-authenticate", "x-request-id":
		return true
	default:
		return false
	}
}

func cookieValue(request *http.Request, name string) string {
	if request == nil {
		return ""
	}
	cookie, err := request.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func inputAuth(auth AuthContext) AuthContext { return auth }

// maxProofLifetime 是用户委托证明的合同上限（≤10秒）。
const maxProofLifetime = 10 * time.Second

// proofExpiry 取 now+10秒、操作期限与会话期限三者最早者，保证委托证明
// 不超过合同上限；入口请求的操作期限通常长于 10 秒，不能直接沿用。
func proofExpiry(op OperationContext, auth AuthContext) Timestamp {
	expiry := time.Now().Add(maxProofLifetime)
	if deadline, err := platform.ParseTimestamp(op.Deadline); err == nil && deadline.Before(expiry) {
		expiry = deadline
	}
	if sessionExpiry, err := platform.ParseTimestamp(auth.ExpiresAt); err == nil && sessionExpiry.Before(expiry) {
		expiry = sessionExpiry
	}
	return Timestamp(expiry.UTC().Format(time.RFC3339Nano))
}
