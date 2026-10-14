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
	if requestHasSession || input.Method != http.MethodGet && input.Method != http.MethodHead {
		origin := runtime.Request.Header.Get("Origin")
		originCtx := withOrigin(ctx, origin)
		if _, err := AssertSameOrigin(AssertSameOriginInput{
			Origin: origin, Host: runtime.Request.Host, ConfiguredOrigins: runtime.ConfiguredOrigins,
		}); err != nil {
			return ProxyRequestResult{}, platform.NewError(platform.CodeForbidden, false, "请求来源不受信任")
		}
		if requestHasSession && input.Method != http.MethodGet && input.Method != http.MethodHead {
			csrf := runtime.Request.Header.Get("X-CSRF-Token")
			_, err := VerifyCSRF(WithProxyRuntime(originCtx, runtime), op, VerifyCSRFInput{
				Supplied: csrf,
				Cookies: SessionCookies{SessionToken: cookieValue(runtime.Request, "nw_sid"), CSRFCookie: csrf},
			})
			if err != nil {
				return ProxyRequestResult{}, err
			}
		}
	}

	var proof InternalProof
	if route.Definition.Permission.Permission != "anonymous" {
		authResult, authErr := ResolveAuthContext(ctx, op, ResolveAuthContextInput{
			Cookies: SessionCookies{SessionToken: cookieValue(runtime.Request, "nw_sid"), CSRFCookie: runtime.Request.Header.Get("X-CSRF-Token")},
			Required: route.Definition.Permission,
		})
		if authErr != nil {
			return ProxyRequestResult{}, authErr
		}
		if _, err := AuthorizeRoute(AuthorizeRouteInput{Auth: authResult.Auth, Route: route}); err != nil {
			return ProxyRequestResult{}, err
		}
		proofResult, proofErr := BuildInternalProof(WithPortalSigner(ctx, runtime.SigningKey), op, BuildInternalProofInput{
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
	requestCtx := ctx
	if input.Route.Definition.TimeoutMS > 0 {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, time.Duration(input.Route.Definition.TimeoutMS)*time.Millisecond)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(requestCtx, input.Route.Definition.Method, target, bytes.NewReader(runtime.Body))
	if err != nil {
		return ForwardRequestResult{}, platform.NewError(platform.CodeValidation, false, "上游请求构造失败")
	}
	applyHeaderMap(request.Header, runtime.Headers)
	if input.Proof.Token != "" {
		request.Header.Set(platform.HeaderPortalProof, input.Proof.Token)
	}
	request.Header.Set("Accept", runtime.Request.Header.Get("Accept"))
	if runtime.SigningKey.KeyID == "" {
		return ForwardRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "入口服务密钥未装配")
	}
	client := &http.Client{Transport: serviceauth.SigningTransport("portal", input.Route.Definition.Owner, runtime.SigningKey, nil)}
	request = request.WithContext(platform.WithOperationContext(request.Context(), op))
	response, err := client.Do(request)
	if err != nil {
		if requestCtx.Err() != nil {
			return ForwardRequestResult{}, platform.NewError(platform.CodeDeadlineExceeded, true, "上游请求超时")
		}
		return ForwardRequestResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "拥有服务不可达")
	}
	return ForwardRequestResult{Response: UpstreamResponse{
		Status: response.StatusCode,
		Headers: headerMapFromHTTP(response.Header),
		StreamHandle: platform.NewIdentifier(),
		Response: response,
	}}, nil
}

// StreamResponse 以io.Copy提供背压；浏览器断开时请求context取消并关闭上游。
func StreamResponse(ctx context.Context, op OperationContext, input StreamResponseInput) (StreamResponseResult, error) {
	runtime, ok := proxyRuntimeFromContext(ctx)
	if !ok || runtime.Writer == nil || input.Upstream.Response == nil || input.WriterHandle == "" {
		return StreamResponseResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "响应流运行时未装配")
	}
	response := input.Upstream.Response
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

func proofExpiry(op OperationContext, auth AuthContext) Timestamp {
	deadline, err := platform.ParseTimestamp(op.Deadline)
	if err != nil {
		deadline = time.Now().Add(10 * time.Second)
	}
	if expiry, err := platform.ParseTimestamp(auth.ExpiresAt); err == nil && expiry.Before(deadline) {
		deadline = expiry
	}
	return Timestamp(deadline.UTC().Format(time.RFC3339Nano))
}
