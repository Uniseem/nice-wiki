package internal

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"nice-wiki/libs/platform"
	"nice-wiki/libs/platform/serviceauth"
)

// IdentityAuthorizationClient仅访问identity的版本化权威授权接口。
type IdentityAuthorizationClient interface {
	CheckAuthorization(context.Context, any) (json.RawMessage, error)
}

type IdentityAuthResolver struct {
	Client IdentityAuthorizationClient
}

// Resolve不解读浏览器自报主体；access token由identity验签并核对当前版本。
func (r IdentityAuthResolver) Resolve(ctx context.Context, cookies SessionCookies, required PermissionRequirement) (AuthContext, error) {
	if r.Client == nil {
		return AuthContext{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份授权客户端未装配")
	}
	raw, err := r.Client.CheckAuthorization(ctx, map[string]any{
		"accessToken": cookies.SessionToken, "permission": required.Permission,
		"action": required.Action,
	})
	if err != nil {
		return AuthContext{}, err
	}
	var decision struct {
		Authorized bool                 `json:"authorized"`
		Auth       platform.AuthContext `json:"auth"`
	}
	if json.Unmarshal(raw, &decision) != nil {
		return AuthContext{}, platform.NewError(platform.CodeDependencyUnavailable, true, "身份授权响应不合法")
	}
	if !decision.Authorized {
		return AuthContext{}, platform.NewError(platform.CodeForbidden, false, "当前账户无此操作权限")
	}
	return decision.Auth, nil
}

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

// BuildInternalProof 签发绑定会话主体、路由模板和正文摘要的十秒内用户委托。
// 拥有方、方法与具体路径由同一请求的服务断言（aud/method/path/bodyHash/jti）
// 绑定并防重放；委托证明按合同沿调用链原样传递，因此不绑定单一拥有方。
// Nonce 只作入口审计关联，不在委托载荷中签名。
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
		SessionVersion: int(input.Auth.SessionRevision),
		Route:          input.Route.Definition.PathTemplate,
		BodyHash:       input.RequestDigest,
		ExpiresAt:      input.ExpiresAt,
	}
	token, err := serviceauth.SignDelegation(key, claims)
	if err != nil {
		return BuildInternalProofResult{}, err
	}
	return BuildInternalProofResult{Proof: InternalProof{
		Token:         token,
		Audience:      input.Route.Definition.Owner,
		ExpiresAt:     input.ExpiresAt,
		RequestDigest: input.RequestDigest,
		Nonce:         nonce,
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

func proofRoute(route ResolvedRoute) string {
	return strings.TrimSpace(route.Definition.PathTemplate)
}
