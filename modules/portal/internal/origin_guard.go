package internal

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/url"
	"strings"

	"nice-wiki/libs/platform"
)

// IdentityCSRFClient 是已登记 identity:verifySessionCSRF 的最小适配；
// portal 不保存会话或CSRF状态。
type IdentityCSRFClient interface {
	VerifySessionCSRF(context.Context, any) (json.RawMessage, error)
}

type CSRFVerifier struct {
	Client IdentityCSRFClient
}

func (v CSRFVerifier) VerifySessionCSRF(ctx context.Context, sessionID, csrfToken, origin string) (bool, error) {
	if v.Client == nil {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "身份校验客户端未装配")
	}
	if sessionID == "" || csrfToken == "" || origin == "" {
		return false, platform.NewError(platform.CodeValidation, false, "CSRF校验请求不完整")
	}
	raw, err := v.Client.VerifySessionCSRF(platform.WithOperationContext(ctx, operationFromContext(ctx)), map[string]string{
		"sessionId": sessionID,
		"csrfToken": csrfToken,
		"origin":    origin,
	})
	if err != nil {
		return false, err
	}
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, platform.NewError(platform.CodeDependencyUnavailable, true, "CSRF校验响应不合法")
	}
	return result.Valid, nil
}

func operationFromContext(ctx context.Context) OperationContext {
	if op, ok := platform.OperationContextFrom(ctx); ok {
		return op
	}
	return OperationContext{
		CallerService: "portal", RequestID: platform.NewIdentifier(),
		OperationID: platform.NewIdentifier(), RootOperationID: platform.NewIdentifier(),
		DataEpoch: "system", Deadline: platform.Now(),
	}
}

// AssertSameOrigin 要求写入请求带完整 Origin，并按规范化的 scheme、host
// 和有效端口比较；不接受浏览器自造的任意来源。
func AssertSameOrigin(input AssertSameOriginInput) (AssertSameOriginResult, error) {
	if input.Origin == "" || input.Host == "" || len(input.ConfiguredOrigins) == 0 {
		return AssertSameOriginResult{}, platform.NewError(platform.CodeValidation, false, "来源缺失")
	}
	origin, err := normalizeOrigin(input.Origin)
	if err != nil {
		return AssertSameOriginResult{}, err
	}
	host := strings.ToLower(strings.TrimSpace(input.Host))
	if host == "" || strings.ContainsAny(host, "/\\?#") || strings.Contains(host, "@") {
		return AssertSameOriginResult{}, platform.NewError(platform.CodeValidation, false, "入口host不合法")
	}
	for _, configured := range input.ConfiguredOrigins {
		candidate, candidateErr := normalizeOrigin(configured)
		if candidateErr != nil {
			continue
		}
		if candidate == origin && sameHost(origin, host) {
			return AssertSameOriginResult{Accepted: true}, nil
		}
	}
	return AssertSameOriginResult{}, platform.NewError(platform.CodeValidation, false, "来源与入口不一致")
}

// VerifyCSRF 先做常量时间的浏览器 token 对比，再调用 identity 权威校验。
func VerifyCSRF(ctx context.Context, op OperationContext, input VerifyCSRFInput) (VerifyCSRFResult, error) {
	if err := ctx.Err(); err != nil {
		return VerifyCSRFResult{}, platform.NewError(platform.CodeCancelled, true, "请求已取消")
	}
	if input.Supplied == "" || input.Cookies.SessionToken == "" || input.Cookies.CSRFCookie == "" {
		return VerifyCSRFResult{}, platform.NewError(platform.CodeUnauthenticated, false, "会话或CSRF凭据缺失")
	}
	if subtle.ConstantTimeCompare([]byte(input.Supplied), []byte(input.Cookies.CSRFCookie)) != 1 {
		return VerifyCSRFResult{}, platform.NewError(platform.CodeUnauthenticated, false, "CSRF校验失败")
	}
	origin := originFromContext(ctx)
	if origin == "" {
		return VerifyCSRFResult{}, platform.NewError(platform.CodeValidation, false, "CSRF来源缺失")
	}
	identity, ok := proxyRuntimeFromContext(ctx)
	if !ok || identity.Identity == nil {
		return VerifyCSRFResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份校验依赖未装配")
	}
	valid, err := identity.Identity.VerifySessionCSRF(ctx, input.Cookies.SessionToken, input.Supplied, origin)
	if err != nil {
		return VerifyCSRFResult{}, err
	}
	if !valid {
		return VerifyCSRFResult{}, platform.NewError(platform.CodeUnauthenticated, false, "CSRF校验失败")
	}
	return VerifyCSRFResult{Accepted: true}, nil
}

func normalizeOrigin(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", platform.NewError(platform.CodeValidation, false, "Origin格式不合法")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", platform.NewError(platform.CodeValidation, false, "Origin协议不受支持")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", platform.NewError(platform.CodeValidation, false, "Origin主机不合法")
	}
	port := parsed.Port()
	if port == "" || (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port == "" {
		return scheme + "://" + host, nil
	}
	return scheme + "://" + host + ":" + port, nil
}

func sameHost(origin, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if parsed.Port() == "" {
		return host == parsed.Hostname() || host == parsed.Host
	}
	return host == parsed.Host
}
