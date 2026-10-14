package internal

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// ResolveAuthContext 只接受显式装配的 identity 授权解析器；缺少权威
// 解析器时返回不可用，绝不把浏览器 header 当成权限上下文。
func ResolveAuthContext(ctx context.Context, op OperationContext, input ResolveAuthContextInput) (ResolveAuthContextResult, error) {
	if input.Cookies.SessionToken == "" || input.Required.Permission == "" {
		return ResolveAuthContextResult{}, platform.NewError(platform.CodeUnauthenticated, false, "会话或权限要求缺失")
	}
	resolver, ok := authResolverFromContext(ctx)
	if !ok {
		return ResolveAuthContextResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份授权解析器未装配")
	}
	auth, err := resolver.Resolve(ctx, input.Cookies, input.Required)
	if err != nil {
		return ResolveAuthContextResult{}, err
	}
	if auth.SubjectID == "" || auth.SiteID == "" || auth.ExpiresAt == "" {
		return ResolveAuthContextResult{}, platform.NewError(platform.CodeUnauthenticated, false, "身份上下文不完整")
	}
	return ResolveAuthContextResult{Auth: auth}, nil
}

// BuildInternalProof 签发绑定拥有方、方法、路径和正文摘要的十秒内委托。
func BuildInternalProof(ctx context.Context, op OperationContext, input BuildInternalProofInput) (BuildInternalProofResult, error) {
	if input.Auth.SubjectID == "" || input.Auth.SiteID == "" || input.Route.Definition.Owner == "" || !platform.ValidHash(input.RequestDigest) {
		return BuildInternalProofResult{}, platform.NewError(platform.CodeValidation, false, "委托证明输入不完整")
	}
	key, ok := portalSignerFromContext(ctx)
	if !ok {
		return BuildInternalProofResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "入口签名密钥未装配")
	}
	expiresAt, err := platform.ParseTimestamp(input.ExpiresAt)
	if err != nil {
		return BuildInternalProofResult{}, platform.NewError(platform.CodeValidation, false, "委托期限不合法")
	}
	now := time.Now().UTC()
	if expiresAt.Before(now) || expiresAt.After(now.Add(10*time.Second)) {
		return BuildInternalProofResult{}, platform.NewError(platform.CodeValidation, false, "委托期限超出十秒上限")
	}
	actorExpiry, err := platform.ParseTimestamp(input.Auth.ExpiresAt)
	if err != nil || expiresAt.After(actorExpiry) {
		return BuildInternalProofResult{}, platform.NewError(platform.CodeValidation, false, "委托期限超过会话期限")
	}
	if op.Deadline != "" {
		deadline, deadlineErr := platform.ParseTimestamp(op.Deadline)
		if deadlineErr != nil || expiresAt.After(deadline) {
			return BuildInternalProofResult{}, platform.NewError(platform.CodeValidation, false, "委托期限超过操作期限")
		}
	}
	nonce := platform.NewIdentifier()
	claims := platform.DelegationClaims{
		Actor:          input.Auth,
		SessionVersion: int(input.Auth.PermissionRevision),
		Route:          input.Route.Definition.PathTemplate,
		BodyHash:       input.RequestDigest,
		ExpiresAt:       input.ExpiresAt,
	}
	token, err := signDelegation(key, claims)
	if err != nil {
		return BuildInternalProofResult{}, err
	}
	return BuildInternalProofResult{Proof: InternalProof{
		Token:        token,
		Audience:     input.Route.Definition.Owner,
		ExpiresAt:    input.ExpiresAt,
		RequestDigest: input.RequestDigest,
		Nonce:        nonce,
	}}, nil
}

// AuthorizeRoute 是入口预检，拥有服务仍必须在服务端二次核权。
func AuthorizeRoute(input AuthorizeRouteInput) (AuthorizeRouteResult, error) {
	required := input.Route.Definition.Permission
	if required.Permission == "anonymous" {
		return AuthorizeRouteResult{Accepted: true}, nil
	}
	if required.Permission == "service" {
		if input.Auth.Kind == "service" {
			return AuthorizeRouteResult{Accepted: true}, nil
		}
		return AuthorizeRouteResult{}, platform.NewError(platform.CodeForbidden, false, "服务路由不接受浏览器主体")
	}
	if input.Auth.SubjectID == "" || input.Auth.SiteID == "" {
		return AuthorizeRouteResult{}, platform.NewError(platform.CodeForbidden, false, "授权主体不完整")
	}
	if required.SiteID != "" && required.SiteID != input.Auth.SiteID {
		return AuthorizeRouteResult{}, platform.NewError(platform.CodeForbidden, false, "站点作用域不匹配")
	}
	for _, grant := range input.Auth.Grants {
		if grant == required.Permission || grant == "superadmin" {
			return AuthorizeRouteResult{Accepted: true}, nil
		}
	}
	return AuthorizeRouteResult{}, platform.NewError(platform.CodeForbidden, false, "缺少路由权限")
}

func signDelegation(key platform.SigningKey, claims platform.DelegationClaims) (string, error) {
	privateKey, err := serviceauth.LoadSigningKey(key)
	if err != nil {
		return "", err
	}
	if len(privateKey) != ed25519.PrivateKeySize || key.KeyID == "" {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "入口签名密钥不可用")
	}
	header, err := json.Marshal(map[string]string{"alg": "EdDSA", "kid": key.KeyID, "typ": "JWT"})
	if err != nil {
		return "", platform.NewError(platform.CodeInternal, false, "委托头编码失败")
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", platform.NewError(platform.CodeInternal, false, "委托载荷编码失败")
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	input := encodedHeader + "." + encodedPayload
	signature := ed25519.Sign(privateKey, []byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func proofRoute(route ResolvedRoute) string {
	return strings.TrimSpace(route.Definition.PathTemplate)
}
