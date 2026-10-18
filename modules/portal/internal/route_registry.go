package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"nice-wiki/libs/platform"
)

var (
	methodPattern           = regexp.MustCompile(`^(GET|HEAD|POST|PUT|PATCH|DELETE)$`)
	variablePattern         = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	templateVariablePattern = regexp.MustCompile(`\{[A-Za-z][A-Za-z0-9_]{0,63}\}`)
)

var allowedOwners = map[string]bool{
	"identity": true, "site-config": true, "content": true, "media": true,
	"public-site": true, "model-gateway": true, "knowledge": true,
	"qa": true, "localization": true, "memory": true, "traffic-guard": true,
	"jobs": true, "analytics": true, "data-protection": true,
}

// CompileRouteRegistry 校验并固定入口路由表。输出按最具体优先排序，
// 因而并发解析不依赖生成文件的原始顺序。
func CompileRouteRegistry(input CompileRouteRegistryInput) (CompileRouteRegistryResult, error) {
	if len(input.Definitions) > 300 {
		return CompileRouteRegistryResult{}, platform.NewError(platform.CodeValidation, false, "路由数量超限")
	}
	definitions := make([]RouteDefinition, len(input.Definitions))
	copy(definitions, input.Definitions)
	seen := make(map[string]struct{}, len(definitions))
	patterns := make(map[string]struct{}, len(definitions))
	for i := range definitions {
		route := &definitions[i]
		if err := validateRouteDefinition(*route); err != nil {
			return CompileRouteRegistryResult{}, err
		}
		key := route.Method + "\x00" + route.PathTemplate
		if _, exists := seen[key]; exists {
			return CompileRouteRegistryResult{}, platform.NewError(platform.CodeValidation, false, "路由定义重复")
		}
		seen[key] = struct{}{}
		patternKey := route.Method + "\x00" + normalizedTemplate(route.PathTemplate)
		if _, exists := patterns[patternKey]; exists {
			return CompileRouteRegistryResult{}, platform.NewError(platform.CodeValidation, false, "路由变量重叠")
		}
		patterns[patternKey] = struct{}{}
		route.Method = strings.ToUpper(route.Method)
		route.AllowedHeaders = normalizeHeaderNames(route.AllowedHeaders)
	}
	sort.SliceStable(definitions, func(i, j int) bool {
		left, right := definitions[i], definitions[j]
		leftStatic, rightStatic := staticSegmentCount(left.PathTemplate), staticSegmentCount(right.PathTemplate)
		if leftStatic != rightStatic {
			return leftStatic > rightStatic
		}
		leftSegments, rightSegments := segmentCount(left.PathTemplate), segmentCount(right.PathTemplate)
		if leftSegments != rightSegments {
			return leftSegments > rightSegments
		}
		if len(left.PathTemplate) != len(right.PathTemplate) {
			return len(left.PathTemplate) > len(right.PathTemplate)
		}
		return left.RouteID < right.RouteID
	})
	canonical, err := json.Marshal(definitions)
	if err != nil {
		return CompileRouteRegistryResult{}, platform.NewError(platform.CodeInternal, false, "路由索引编码失败")
	}
	return CompileRouteRegistryResult{Registry: RouteRegistry{
		Definitions: definitions,
		ConfigHash:  platform.HashBytes(canonical),
	}}, nil
}

// ResolveRoute 只接受已编译的静态路由，不把输入解释为上游 URL。
func ResolveRoute(input ResolveRouteInput) (ResolveRouteResult, error) {
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if !methodPattern.MatchString(method) || input.EscapedPath == "" || len(input.EscapedPath) > 2048 {
		return ResolveRouteResult{}, platform.NewError(platform.CodeValidation, false, "请求方法或路径不合法")
	}
	if input.EscapedPath[0] != '/' || strings.ContainsAny(input.EscapedPath, "?#\\") {
		return ResolveRouteResult{}, platform.NewError(platform.CodeValidation, false, "请求路径不合法")
	}
	lower := strings.ToLower(input.EscapedPath)
	if strings.Contains(lower, "%25") || strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
		return ResolveRouteResult{}, platform.NewError(platform.CodeValidation, false, "请求路径编码不安全")
	}
	decoded, err := url.PathUnescape(input.EscapedPath)
	if err != nil || decoded == "" || strings.ContainsAny(decoded, "\\?#") || strings.Contains(decoded, "//") {
		return ResolveRouteResult{}, platform.NewError(platform.CodeValidation, false, "请求路径编码不合法")
	}
	if hasTraversal(decoded) {
		return ResolveRouteResult{}, platform.NewError(platform.CodeValidation, false, "请求路径包含穿越片段")
	}
	for _, definition := range input.Registry.Definitions {
		if definition.Method != method {
			continue
		}
		parameters, matched := matchTemplate(definition.PathTemplate, decoded)
		if matched {
			return ResolveRouteResult{Route: ResolvedRoute{
				Definition:   definition,
				RelativePath: input.EscapedPath,
				Parameters:   parameters,
			}}, nil
		}
	}
	return ResolveRouteResult{}, platform.NewError(platform.CodeNotFound, false, "路由未登记")
}

func validateRouteDefinition(route RouteDefinition) error {
	if !platform.ValidIdentifier(route.RouteID) || !methodPattern.MatchString(strings.ToUpper(route.Method)) {
		return platform.NewError(platform.CodeValidation, false, "路由标识或方法不合法")
	}
	if route.PathTemplate == "" || len(route.PathTemplate) > 512 || route.PathTemplate[0] != '/' ||
		strings.ContainsAny(route.PathTemplate, "?#\\") || strings.Contains(route.PathTemplate, "://") ||
		strings.Contains(route.PathTemplate, "//") || hasTraversal(route.PathTemplate) {
		return platform.NewError(platform.CodeValidation, false, "路由路径模板不安全")
	}
	if !allowedOwners[route.Owner] || route.UpstreamOperationID == "" || len(route.UpstreamOperationID) > 128 {
		return platform.NewError(platform.CodeValidation, false, "路由拥有方或操作标识不合法")
	}
	if route.Permission.Permission == "" || len(route.Permission.Permission) > 64 {
		return platform.NewError(platform.CodeValidation, false, "路由权限声明不合法")
	}
	if route.TimeoutMS < 100 || route.TimeoutMS > 300000 || route.MaxBodyBytes < 0 || route.MaxBodyBytes > 2<<30 {
		return platform.NewError(platform.CodeValidation, false, "路由预算不合法")
	}
	if len(route.AllowedHeaders) > 40 {
		return platform.NewError(platform.CodeValidation, false, "路由头白名单超限")
	}
	for _, segment := range splitPath(route.PathTemplate) {
		if strings.Contains(segment, "{") || strings.Contains(segment, "}") {
			if !validTemplateSegment(segment) {
				return platform.NewError(platform.CodeValidation, false, "路由变量模板不合法")
			}
		}
	}
	return nil
}

func validTemplateSegment(segment string) bool {
	if strings.Count(segment, "{") != strings.Count(segment, "}") {
		return false
	}
	for len(segment) > 0 {
		start := strings.IndexByte(segment, '{')
		if start < 0 {
			return !strings.Contains(segment, "}")
		}
		end := strings.IndexByte(segment[start:], '}')
		if end < 0 || !variablePattern.MatchString(segment[start+1:start+end]) {
			return false
		}
		segment = segment[start+end+1:]
	}
	return true
}

func splitPath(path string) []string {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func hasTraversal(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." || strings.Contains(segment, "\x00") {
			return true
		}
	}
	return false
}

func normalizedTemplate(path string) string {
	return templateVariablePattern.ReplaceAllString(path, "{}")
}

func segmentCount(path string) int { return len(splitPath(path)) }

func staticSegmentCount(path string) int {
	count := 0
	for _, segment := range splitPath(path) {
		if !strings.Contains(segment, "{") {
			count++
		}
	}
	return count
}

func normalizeHeaderNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		name = httpCanonicalHeader(name)
		if name == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(name)]; ok {
			continue
		}
		seen[strings.ToLower(name)] = struct{}{}
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func httpCanonicalHeader(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "\r\n:") {
		return ""
	}
	parts := strings.Split(name, "-")
	for i := range parts {
		if parts[i] == "" {
			return ""
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, "-")
}

func matchTemplate(template, decodedPath string) ([]RouteParameter, bool) {
	templateSegments := splitPath(template)
	pathSegments := splitPath(decodedPath)
	if len(templateSegments) != len(pathSegments) {
		return nil, false
	}
	parameters := make([]RouteParameter, 0, 4)
	for i, templateSegment := range templateSegments {
		pattern := "^"
		remaining := templateSegment
		names := make([]string, 0, 2)
		for len(remaining) > 0 {
			start := strings.IndexByte(remaining, '{')
			if start < 0 {
				pattern += regexp.QuoteMeta(remaining)
				break
			}
			pattern += regexp.QuoteMeta(remaining[:start])
			end := strings.IndexByte(remaining[start:], '}')
			if end < 0 {
				return nil, false
			}
			end += start
			name := remaining[start+1 : end]
			pattern += "([^/]+)"
			remaining = remaining[end+1:]
			names = append(names, name)
		}
		pattern += "$"
		matched, err := regexp.MatchString(pattern, pathSegments[i])
		if err != nil || !matched {
			return nil, false
		}
		if len(names) > 0 {
			values, compileErr := regexp.Compile(strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$"))
			if compileErr != nil {
				return nil, false
			}
			matches := values.FindStringSubmatch(pathSegments[i])
			if len(matches) != len(names)+1 {
				return nil, false
			}
			for j, name := range names {
				parameters = append(parameters, RouteParameter{Name: name, Value: matches[j+1]})
			}
		}
	}
	return parameters, true
}

type (
	OperationContext = platform.OperationContext
	AuthContext      = platform.AuthContext
	Identifier       = platform.Identifier
	Hash             = platform.Hash
	Timestamp        = platform.Timestamp
	Revision         = platform.Revision
)

type PermissionRequirement struct {
	Permission string
	ResourceID *string
	SiteID     string
	Action     string
}

type RouteDefinition struct {
	RouteID             string
	Method              string
	PathTemplate        string
	Owner               string
	UpstreamOperationID string
	Permission          PermissionRequirement
	TimeoutMS           int
	MaxBodyBytes        int64
	Streaming           bool
	AllowedHeaders      []string
	Admission           string
}

type RouteRegistry struct {
	Definitions []RouteDefinition
	ConfigHash  Hash
}

type CompileRouteRegistryInput struct{ Definitions []RouteDefinition }
type CompileRouteRegistryResult struct{ Registry RouteRegistry }
type ResolveRouteInput struct {
	Registry    RouteRegistry
	Method      string
	EscapedPath string
}
type RouteParameter struct{ Name, Value string }
type ResolvedRoute struct {
	Definition   RouteDefinition
	RelativePath string
	Parameters   []RouteParameter
}
type ResolveRouteResult struct{ Route ResolvedRoute }

type HeaderEntry struct {
	Name   string
	Values []string
}
type HeaderMap struct{ Entries []HeaderEntry }
type IncomingRequest struct {
	Method      string
	EscapedPath string
	Headers     HeaderMap
	BodyHandle  string
}
type RequestLimits struct{ BodyBytes, HeaderBytes, QueryBytes int64 }
type ValidatedEnvelope struct {
	RequestID        Identifier
	OperationID      Identifier
	ExpectedRevision *Revision
	Headers          HeaderMap
	BodyHandle       string
	BodyDigest       Hash
	BodyBytes        int64
}
type ValidateEnvelopeInput struct {
	Request IncomingRequest
	Limits  RequestLimits
}
type ValidateEnvelopeResult struct{ Envelope ValidatedEnvelope }
type NormalizeForwardHeadersInput struct {
	Headers  HeaderMap
	Envelope ValidatedEnvelope
}
type NormalizeForwardHeadersResult struct{ Headers HeaderMap }
type SessionCookies struct{ SessionToken, CSRFCookie string }
type AssertSameOriginInput struct {
	Origin            string
	Host              string
	ConfiguredOrigins []string
}
type AssertSameOriginResult struct{ Accepted bool }
type VerifyCSRFInput struct {
	Supplied string
	Cookies  SessionCookies
}
type VerifyCSRFResult struct{ Accepted bool }
type ResolveAuthContextInput struct {
	Cookies  SessionCookies
	Required PermissionRequirement
}
type ResolveAuthContextResult struct{ Auth AuthContext }
type InternalProof struct {
	Token         string
	Audience      string
	ExpiresAt     Timestamp
	RequestDigest Hash
	Nonce         Identifier
}
type BuildInternalProofInput struct {
	Auth          AuthContext
	Route         ResolvedRoute
	RequestDigest Hash
	ExpiresAt     Timestamp
}
type BuildInternalProofResult struct{ Proof InternalProof }
type AuthorizeRouteInput struct {
	Auth  AuthContext
	Route ResolvedRoute
}
type AuthorizeRouteResult struct{ Accepted bool }
type UpstreamResponse struct {
	Status       int
	Headers      HeaderMap
	StreamHandle string
	Response     *http.Response
	Cancel       context.CancelFunc `json:"-"`
}
type ForwardRequestInput struct {
	Route   ResolvedRoute
	Request ValidatedEnvelope
	Proof   InternalProof
}
type ForwardRequestResult struct{ Response UpstreamResponse }
type StreamResponseInput struct {
	Upstream         UpstreamResponse
	WriterHandle     string
	Route            ResolvedRoute
	CurrentSessionID Identifier
}
type StreamResponseResult struct{ Accepted bool }
type MapUpstreamFailureInput struct {
	FailureCode string
	RequestID   Identifier
}
type MapUpstreamFailureResult struct{ Problem platform.Problem }
type ProxyRequestInput struct {
	Method        string
	RawPath       string
	ContentLength int64
}
type ProxyRequestResult struct {
	Status int
	Owner  string
}
type ReadRuntimeStateInput struct{}
type ReadRuntimeStateResult struct {
	DataEpoch   platform.Identifier
	Maintenance bool
	Revision    platform.Revision
}
type CheckEntranceGateInput struct {
	Route           ResolvedRoute
	RuntimeRevision platform.Revision
}
type CheckEntranceGateResult struct {
	Allowed           bool
	RetryAfterSeconds int
}
type BarrierRequest struct {
	BarrierID   Identifier
	Deadline    Timestamp
	Mode        string
	OperationID Identifier
}
type BarrierStatus struct {
	BarrierID         Identifier
	DataEpoch         Identifier
	Owner             string
	RevisionWatermark Revision
	State             string
}
type ReleaseBarrierInput struct {
	BarrierID     Identifier
	ExpectedEpoch Identifier
}
type ReleaseBarrierResult struct{ Status BarrierStatus }
type SiteFeatureFlags struct {
	QAEnabled            bool
	LocalizationEnabled  bool
	AnnouncementsEnabled bool
}
type Capability struct {
	Key     Identifier
	Enabled bool
	Reason  *string
}
type NavigationItem struct {
	Key                Identifier
	Route              string
	Label              string
	RequiredCapability Identifier
}
type BuildCapabilitiesInput struct {
	Grants []string
	Flags  SiteFeatureFlags
}
type BuildCapabilitiesResult struct{ Capabilities []Capability }
type BuildNavigationInput struct {
	Capabilities []Capability
	Locale       string
}
type BuildNavigationResult struct{ Navigation []NavigationItem }

type RuntimeReader interface {
	ReadRuntimeState(context.Context) (platform.RuntimeView, error)
}
type IdentityVerifier interface {
	VerifySessionCSRF(context.Context, string, string, string) (bool, error)
}
type proxyRuntimeKey struct{}
type ProxyRuntime struct {
	ValidateBeaconBody func(json.RawMessage) error
	// SiteID与预算key仅由可信启动装配；预算pepper不随HTTP签名key自动轮换。
	SiteID            platform.Identifier
	NetworkScopeKeys  []platform.SigningKey
	Registry          RouteRegistry
	Dependencies      map[string]string
	SigningKey        platform.SigningKey
	DelegationKey     platform.SigningKey
	Issuer            string
	Request           *http.Request
	Body              []byte
	Query             string
	Headers           HeaderMap
	Writer            http.ResponseWriter
	Runtime           RuntimeReader
	Identity          IdentityVerifier
	Gate              *Gate
	ConfiguredOrigins []string
	TrustedProxyCIDRs []string
	ResponseStarted   bool
}

func WithProxyRuntime(ctx context.Context, runtime *ProxyRuntime) context.Context {
	return context.WithValue(ctx, proxyRuntimeKey{}, runtime)
}
func proxyRuntimeFromContext(ctx context.Context) (*ProxyRuntime, bool) {
	runtime, ok := ctx.Value(proxyRuntimeKey{}).(*ProxyRuntime)
	return runtime, ok && runtime != nil
}

type originKey struct{}

func withOrigin(ctx context.Context, origin string) context.Context {
	return context.WithValue(ctx, originKey{}, origin)
}
func originFromContext(ctx context.Context) string {
	origin, _ := ctx.Value(originKey{}).(string)
	return origin
}

type authResolverKey struct{}
type AuthResolver interface {
	Resolve(context.Context, SessionCookies, PermissionRequirement) (AuthContext, error)
}

func WithAuthResolver(ctx context.Context, resolver AuthResolver) context.Context {
	return context.WithValue(ctx, authResolverKey{}, resolver)
}
func authResolverFromContext(ctx context.Context) (AuthResolver, bool) {
	resolver, ok := ctx.Value(authResolverKey{}).(AuthResolver)
	return resolver, ok && resolver != nil
}

type signerKey struct{}

func WithPortalSigner(ctx context.Context, key platform.SigningKey) context.Context {
	return context.WithValue(ctx, signerKey{}, key)
}
func portalSignerFromContext(ctx context.Context) (platform.SigningKey, bool) {
	key, ok := ctx.Value(signerKey{}).(platform.SigningKey)
	return key, ok && key.KeyID != "" && key.PrivateKeyRef != ""
}

// BrowserSession 不包含access/refresh；CSRF仅供当前浏览器内存与请求头使用。
type BrowserSession struct {
	SessionID          Identifier  `json:"sessionId"`
	Auth               AuthContext `json:"auth"`
	MustChangePassword bool        `json:"mustChangePassword"`
	ExpiresAt          Timestamp   `json:"expiresAt"`
	CSRFToken          string      `json:"csrfToken"`
}

// 内部敏感wire DTO只从identity成功响应读取，不持久化或写入浏览器body。
type identitySessionTokens struct {
	SessionID          Identifier   `json:"sessionId"`
	AccessToken        string       `json:"accessToken"`
	RefreshToken       string       `json:"refreshToken"`
	CSRFToken          string       `json:"csrfToken"`
	Auth               *AuthContext `json:"auth"`
	MustChangePassword *bool        `json:"mustChangePassword"`
	ExpiresAt          Timestamp    `json:"expiresAt"`
}

type identityBrowserAccount struct {
	AccountID          Identifier `json:"accountId"`
	SiteID             Identifier `json:"siteId"`
	AccountName        string     `json:"accountName"`
	IsSuperadmin       bool       `json:"isSuperadmin"`
	Permissions        []string   `json:"permissions"`
	MustChangePassword bool       `json:"mustChangePassword"`
	Status             string     `json:"status"`
	Revision           Revision   `json:"revision"`
	PermissionRevision Revision   `json:"permissionRevision"`
	CreatedAt          Timestamp  `json:"createdAt"`
	UpdatedAt          Timestamp  `json:"updatedAt"`
}

type identityAuthenticationResult struct {
	Tokens               *identitySessionTokens  `json:"tokens"`
	CurrentSessionTokens *identitySessionTokens  `json:"currentSessionTokens"`
	Account              *identityBrowserAccount `json:"account"`
	NextRoute            *string                 `json:"nextRoute"`
}

type browserAuthenticationResult struct {
	Session   BrowserSession          `json:"session"`
	Account   *identityBrowserAccount `json:"account,omitempty"`
	NextRoute *string                 `json:"nextRoute,omitempty"`
}
