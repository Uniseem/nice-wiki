package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
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
	authentication := authenticationOperation(route)
	if authentication != "" || (route.Definition.Owner == "identity" && route.Definition.UpstreamOperationID == "revokeSession") {
		runtime.Writer.Header().Set("Cache-Control", "no-store")
		runtime.Writer.Header().Set("Pragma", "no-cache")
	}
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

	if authentication == "refreshSession" {
		if err := prepareAuthenticationRequest(runtime); err != nil {
			return ProxyRequestResult{}, err
		}
		// 刷新由HttpOnly cookie构造内部正文，断言必须绑定改写后的真实字节。
		validated.Envelope.BodyBytes = int64(len(runtime.Body))
		validated.Envelope.BodyDigest = platform.HashBytes(runtime.Body)
		if validated.Envelope.BodyBytes > bodyLimit {
			return ProxyRequestResult{}, platform.NewError(platform.CodeBudgetExceeded, false, "内部认证请求超出路由上限")
		}
	}
	if authentication == "createSession" || authentication == "registerFirstSuperadmin" {
		scopes, err := serviceauth.PrivateNetworkScopes(runtime.SigningKey, runtime.Request.RemoteAddr,
			strings.Join(runtime.Request.Header.Values("X-Forwarded-For"), ","), runtime.TrustedProxyCIDRs, time.Now())
		if err != nil {
			return ProxyRequestResult{}, err
		}
		op.LoginNetworkScopes = scopes
		ctx = platform.WithOperationContext(ctx, op)
	}

	var proof InternalProof
	var currentSessionID Identifier
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
		currentSessionID = authResult.Auth.SessionID
		proofResult, proofErr := BuildInternalProof(WithPortalSigner(ctx, runtime.DelegationKey), op, BuildInternalProofInput{
			Auth: inputAuth(authResult.Auth), Route: route, RequestDigest: validated.Envelope.BodyDigest,
			ExpiresAt: proofExpiry(op, authResult.Auth),
		})
		if proofErr != nil {
			return ProxyRequestResult{}, proofErr
		}
		proof = proofResult.Proof
	}
 if route.Definition.Owner=="traffic-guard" && route.Definition.Permission.Permission=="anonymous" {
  if runtime.SiteID=="" || len(runtime.NetworkScopeKeys)==0 || len(runtime.NetworkScopeKeys)>2 {return ProxyRequestResult{},platform.NewError(platform.CodeDependencyUnavailable,true,"风控站点或预算密钥尚未装配")}
  scopes:=[]platform.Hash{}
  for _,key:=range runtime.NetworkScopeKeys {values,err:=serviceauth.PrivateBudgetNetworkScopes(key,runtime.Request.RemoteAddr,strings.Join(runtime.Request.Header.Values("X-Forwarded-For"),","),runtime.TrustedProxyCIDRs,runtime.SiteID,time.Now());if err!=nil{return ProxyRequestResult{},err};scopes=append(scopes,values...)}
  op.Actor.SiteID=runtime.SiteID;op.NetworkBudgetScopes=scopes
  ctx=platform.WithOperationContext(ctx,op)
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
	if _, err := StreamResponse(ctx, op, StreamResponseInput{Upstream: forwarded.Response, WriterHandle: platform.NewIdentifier(), Route: route, CurrentSessionID: currentSessionID}); err != nil {
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
	if authenticationOperation(input.Route) != "" {
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")
	}
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
	operation := authenticationOperation(input.Route)
	currentRevocation := isCurrentSessionRevocation(input.Route, input.CurrentSessionID)
	if operation != "" || currentRevocation {
		var body []byte
		var cookies []*http.Cookie
		var err error
		if currentRevocation {
			body, cookies, err = adaptSessionRevocation(response)
		} else {
			body, cookies, err = adaptAuthenticationResponse(response, operation)
		}
		if err != nil {
			return StreamResponseResult{}, err
		}
		// 敏感结果完成解码和cookie构造后才写头；不沿用内部body的长度或缓存水位。
		runtime.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		if response.StatusCode != http.StatusOK {
			runtime.Writer.Header().Set("Content-Type", "application/problem+json")
		}
		runtime.Writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		runtime.Writer.Header().Set("Cache-Control", "no-store")
		runtime.Writer.Header().Set("Pragma", "no-cache")
		runtime.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
			runtime.Writer.Header().Set("Retry-After", retryAfter)
		}
		for _, cookie := range cookies {
			http.SetCookie(runtime.Writer, cookie)
		}
		runtime.ResponseStarted = true
		runtime.Writer.WriteHeader(response.StatusCode)
		if _, err := runtime.Writer.Write(body); err != nil {
			return StreamResponseResult{}, platform.NewError(platform.CodeCancelled, false, "认证响应未能完整送达")
		}
		return StreamResponseResult{Accepted: true}, nil
	}
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
	runtime.ResponseStarted = true
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

// authenticationOperation 只选择已登记的identity认证操作，不从路径片段猜用途。
func authenticationOperation(route ResolvedRoute) string {
	if route.Definition.Owner != "identity" || route.Definition.Method != http.MethodPost {
		return ""
	}
	switch route.Definition.UpstreamOperationID {
	case "createSession", "registerFirstSuperadmin", "refreshSession", "changePassword":
		return route.Definition.UpstreamOperationID
	default:
		return ""
	}
}

// prepareAuthenticationRequest 保持口令请求原字节；刷新秘密只允许来自HttpOnly cookie。
func prepareAuthenticationRequest(runtime *ProxyRuntime) error {
	mediaType, _, err := mime.ParseMediaType(runtime.Request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return platform.NewError(platform.CodeValidation, false, "浏览器刷新请求必须为JSON")
	}
	var empty struct{}
	trimmed := bytes.TrimSpace(runtime.Body)
	if len(trimmed) == 0 || trimmed[0] != '{' || decodeAuthenticationJSON(trimmed, &empty) != nil {
		return platform.NewError(platform.CodeValidation, false, "浏览器刷新请求必须为空对象")
	}
	var refresh string
	var count int
	for _, cookie := range runtime.Request.Cookies() {
		if cookie.Name == "nw_refresh" {
			refresh = cookie.Value
			count++
		}
	}
	if count != 1 || len(refresh) < 32 || len(refresh) > 4096 {
		return platform.NewError(platform.CodeUnauthenticated, false, "会话刷新凭据缺失或不明确")
	}
	runtime.Body, _ = json.Marshal(struct {
		RefreshToken string `json:"refreshToken"`
	}{RefreshToken: refresh})
	return nil
}

const maxAuthenticationResponseBytes = 64 << 10

// adaptAuthenticationResponse 隔离内部秘密DTO，只构造明确白名单中的浏览器字段。
func adaptAuthenticationResponse(response *http.Response, operation string) ([]byte, []*http.Cookie, error) {
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxAuthenticationResponseBytes+1))
	if err != nil || len(raw) > maxAuthenticationResponseBytes {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "身份响应不完整或超出上限")
	}
	if response.StatusCode != http.StatusOK {
		body, err := authenticationProblem(response, raw)
		return body, nil, err
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "身份成功响应媒体类型不符")
	}
	var result identityAuthenticationResult
	if decodeAuthenticationJSON(raw, &result) != nil {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "身份成功响应不符合认证协议")
	}
	tokens := result.Tokens
	switch operation {
	case "createSession":
		if tokens == nil || result.CurrentSessionTokens != nil || result.Account != nil || result.NextRoute == nil {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "登录成功响应字段不符")
		}
	case "registerFirstSuperadmin":
		if tokens == nil || result.CurrentSessionTokens != nil || result.Account == nil || result.NextRoute == nil {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "注册成功响应字段不符")
		}
	case "refreshSession":
		if tokens == nil || result.CurrentSessionTokens != nil || result.Account != nil || result.NextRoute != nil {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "刷新成功响应字段不符")
		}
	case "changePassword":
		if tokens != nil || result.CurrentSessionTokens == nil || result.Account == nil || result.NextRoute != nil {
			return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "改密成功响应字段不符")
		}
		tokens = result.CurrentSessionTokens
	}
	if result.NextRoute != nil && *result.NextRoute != "/admin/me" && *result.NextRoute != "/admin/setup" && *result.NextRoute != "/admin" {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "身份返回路由不在白名单")
	}
	cookies, err := authenticationCookies(*tokens)
	if err != nil {
		return nil, nil, err
	}
	browser := browserAuthenticationResult{
		Session: BrowserSession{SessionID: tokens.SessionID, Auth: *tokens.Auth,
			MustChangePassword: *tokens.MustChangePassword, ExpiresAt: tokens.ExpiresAt, CSRFToken: tokens.CSRFToken},
		Account: result.Account, NextRoute: result.NextRoute,
	}
	body, _ := json.Marshal(browser)
	return body, cookies, nil
}

// decodeAuthenticationJSON 只在敏感wire边界解码一次，拒绝未知字段与尾随JSON。
func decodeAuthenticationJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return platform.NewError(platform.CodeIntegrityError, false, "认证JSON包含尾随内容")
	}
	return nil
}

// authenticationCookies 不把refresh期限猜成访问期限或固定七天，服务端仍核绝对期限。
func authenticationCookies(tokens identitySessionTokens) ([]*http.Cookie, error) {
	expires, err := platform.ParseTimestamp(tokens.ExpiresAt)
	if err != nil || !expires.After(time.Now()) || tokens.SessionID == "" || tokens.AccessToken == "" ||
		len(tokens.RefreshToken) < 32 || tokens.CSRFToken == "" || len(tokens.CSRFToken) > 2048 ||
		tokens.Auth == nil || tokens.MustChangePassword == nil || tokens.Auth.SessionID != tokens.SessionID ||
		tokens.Auth.ExpiresAt != tokens.ExpiresAt {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "身份会话cookie材料不完整")
	}
	cookies := []*http.Cookie{
		{Name: "nw_sid", Value: tokens.AccessToken, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: expires},
		{Name: "nw_refresh", Value: tokens.RefreshToken, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode},
	}
	for _, cookie := range cookies {
		if cookie.Valid() != nil || len(cookie.String()) > 4096 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "身份会话cookie不符合浏览器协议")
		}
	}
	return cookies, nil
}

// authenticationProblem 保留已知错误决定，拒绝把内部原始detail或未知字段转给浏览器。
func authenticationProblem(response *http.Response, raw []byte) ([]byte, error) {
	var upstream platform.Problem
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/problem+json" || decodeAuthenticationJSON(raw, &upstream) != nil || upstream.Status != response.StatusCode ||
		platform.ProblemHTTPStatus[upstream.Code] != response.StatusCode || !platform.ValidIdentifier(upstream.RequestID) {
		return nil, platform.NewError(platform.CodeIntegrityError, false, "身份失败响应不符合Problem协议")
	}
	detail := "认证操作未能完成，请按错误码处理。"
	if upstream.Code == platform.CodeConflict {
		detail = "认证操作已提交或发生冲突，请重新登录。"
	}
	body, _ := json.Marshal(platform.Problem{Type: "about:blank", Title: "认证操作未能完成", Status: upstream.Status,
		Code: upstream.Code, Detail: detail, RequestID: upstream.RequestID, Retryable: upstream.Retryable})
	return body, nil
}

// isCurrentSessionRevocation 只按权威会话与静态路由参数决定是否清本设备cookie。
func isCurrentSessionRevocation(route ResolvedRoute, currentSessionID Identifier) bool {
	if route.Definition.Owner != "identity" || route.Definition.Method != http.MethodDelete ||
		route.Definition.UpstreamOperationID != "revokeSession" || currentSessionID == "" {
		return false
	}
	for _, parameter := range route.Parameters {
		if parameter.Name == "sessionId" {
			return parameter.Value == currentSessionID
		}
	}
	return false
}

// adaptSessionRevocation 只有当前设备撤销的权威成功回执才删除两枚同属性cookie。
func adaptSessionRevocation(response *http.Response) ([]byte, []*http.Cookie, error) {
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxAuthenticationResponseBytes+1))
	if err != nil || len(raw) > maxAuthenticationResponseBytes {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "会话撤销响应不完整或超出上限")
	}
	if response.StatusCode != http.StatusOK {
		body, err := authenticationProblem(response, raw)
		return body, nil, err
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	var result struct {
		Revoked *bool `json:"revoked"`
	}
	if err != nil || mediaType != "application/json" || decodeAuthenticationJSON(raw, &result) != nil || result.Revoked == nil || !*result.Revoked {
		return nil, nil, platform.NewError(platform.CodeIntegrityError, false, "会话撤销成功回执不符")
	}
	cookies := []*http.Cookie{
		{Name: "nw_sid", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()},
		{Name: "nw_refresh", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()},
	}
	return []byte(`{"revoked":true}`), cookies, nil
}
