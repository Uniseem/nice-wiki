// Package serviceauth 实现服务断言与用户委托证明的签发和验证。
//
// 服务间每个请求携带≤60秒的Ed25519 JWS断言（iss=调用服务、aud=目标服务、
// method、path、bodyHash、jti），接收方按公钥表验证并拒绝重放。
package serviceauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"nice-wiki/libs/platform"
)

// 断言有效期与验证偏差上限。
const (
	AssertionTTL   = 60 * time.Second
	ClockSkew      = 30 * time.Second
	MaxTokenLength = 4096
	// algNone出现即拒绝；本库只接受EdDSA。
	algEdDSA = "EdDSA"
)

type cacheEntry struct {
	key     ed25519.PrivateKey
	modTime time.Time
	size    int64
}

var keyCache sync.Map // privateKeyRef -> cacheEntry

// PrivateNetworkScopes 只从真实连接及部署信任链派生当前和前一UTC日网络HMAC。
// 原始地址仅在本次调用内使用，错误不含地址；未受信连接的转发头完全忽略。
func PrivateNetworkScopes(key platform.SigningKey, remoteAddr, forwardedFor string, trustedProxyCIDRs []string, now time.Time) ([]platform.Hash, error) {
	peer, err := privateNetworkPeer(remoteAddr, forwardedFor, trustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	day := now.UTC()
	scopes := make([]platform.Hash, 0, 2)
	for _, date := range []time.Time{day, day.AddDate(0, 0, -1)} {
		digest, err := PrivateOperationDigest(key, "network/day/v1/"+date.Format("2006-01-02"), []byte(peer.String()))
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, digest)
	}
	return scopes, nil
}

// LoadSigningKey 读取并解析secret文件中的Ed25519私钥（PKCS8 PEM）；
// 按路径与修改时间缓存，私钥字节只驻留内存、不进日志。
func LoadSigningKey(key platform.SigningKey) (ed25519.PrivateKey, error) {
	if key.PrivateKeyRef == "" || key.KeyID == "" {
		return nil, platform.NewError(platform.CodeValidation, false, "私钥引用不完整")
	}
	info, err := os.Stat(key.PrivateKeyRef)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "私钥文件不可读")
	}
	if cached, ok := keyCache.Load(key.PrivateKeyRef); ok {
		entry := cached.(cacheEntry)
		if entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
			return entry.key, nil
		}
	}
	raw, err := os.ReadFile(key.PrivateKeyRef)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "私钥文件不可读")
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, platform.NewError(platform.CodeInternal, false, "私钥文件格式不合法")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, platform.NewError(platform.CodeInternal, false, "私钥解析失败")
	}
	keyBytes, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, platform.NewError(platform.CodeInternal, false, "私钥不是Ed25519")
	}
	entry := cacheEntry{key: keyBytes, modTime: info.ModTime(), size: info.Size()}
	keyCache.Store(key.PrivateKeyRef, entry)
	return keyBytes, nil
}

// SignAssertion 为一次服务间请求签发短期JWS compact断言。
// 每次生成新jti；不缓存、不记录。
func SignAssertion(key platform.SigningKey, claims platform.AssertionClaims) (string, error) {
	issuedAt, err := platform.ParseTimestamp(claims.IssuedAt)
	if err != nil {
		return "", platform.NewError(platform.CodeValidation, false, "断言签发时间不合法")
	}
	expiresAt, err := platform.ParseTimestamp(claims.ExpiresAt)
	if err != nil {
		return "", platform.NewError(platform.CodeValidation, false, "断言过期时间不合法")
	}
	switch {
	case claims.SiteID != "" && !platform.ValidIdentifier(claims.SiteID):
		return "", platform.NewError(platform.CodeValidation, false, "断言站点作用域不合法")
	case !validLoginScopes(claims):
		return "", platform.NewError(platform.CodeValidation, false, "登录网络作用域来源不合法")
	case claims.Issuer == "" || claims.Audience == "" || claims.Method == "" || claims.Path == "":
		return "", platform.NewError(platform.CodeValidation, false, "断言声明不完整")
	case !platform.ValidHash(claims.BodyHash):
		return "", platform.NewError(platform.CodeValidation, false, "断言正文摘要不合法")
	case !platform.ValidIdentifier(claims.JTI):
		return "", platform.NewError(platform.CodeValidation, false, "断言jti不合法")
	case expiresAt.Sub(issuedAt) <= 0 || expiresAt.Sub(issuedAt) > AssertionTTL:
		return "", platform.NewError(platform.CodeValidation, false, "断言有效期必须大于0且不超过60秒")
	}
	privateKey, err := LoadSigningKey(key)
	if err != nil {
		return "", err
	}
	header, err := json.Marshal(map[string]string{"alg": algEdDSA, "kid": key.KeyID, "typ": "JWT"})
	if err != nil {
		return "", platform.NewError(platform.CodeInternal, false, "断言头编码失败")
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", platform.NewError(platform.CodeInternal, false, "断言载荷编码失败")
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	signature := ed25519.Sign(privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// VerifyAssertion 验证服务间请求断言：只接受EdDSA与已登记keyId，
// 核audience、method、path、bodyHash与有效期（±30秒偏差），
// jti在有效期内只接受一次。返回已验证的调用服务slug所在的声明。
func VerifyAssertion(anchors platform.TrustAnchors, token string, expect platform.AssertionExpectation) (platform.AssertionClaims, error) {
	if token == "" || len(token) > MaxTokenLength {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言缺失或超长")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言格式不合法")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言头不合法")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言头不合法")
	}
	if header.Alg != algEdDSA || header.Kid == "" {
		// 拒绝未知key与非EdDSA算法（含alg=none）。
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言算法或密钥不受支持")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言载荷不合法")
	}
	var claims platform.AssertionClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言载荷不合法")
	}
	publicKeyRaw := anchors.ServiceKeyID(claims.Issuer, header.Kid)
	if publicKeyRaw == "" {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言签名密钥未登记")
	}
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(publicKeyRaw)
	if err != nil || len(publicKeyBytes) != ed25519.PublicKeySize {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "登记公钥不合法")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言签名不合法")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), []byte(parts[0]+"."+parts[1]), signature) {
		return platform.AssertionClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "断言签名验证失败")
	}
	if err := checkAssertionClaims(claims, expect); err != nil {
		return platform.AssertionClaims{}, err
	}
	if err := claimReplay(claims); err != nil {
		return platform.AssertionClaims{}, err
	}
	return claims, nil
}

func checkAssertionClaims(claims platform.AssertionClaims, expect platform.AssertionExpectation) error {
	if claims.SiteID != "" && !platform.ValidIdentifier(claims.SiteID) {
		return platform.NewError(platform.CodeUnauthenticated, false, "断言站点作用域不合法")
	}
	if !validLoginScopes(claims) || !validBudgetScopes(claims) {
		return platform.NewError(platform.CodeUnauthenticated, false, "登录网络作用域来源不合法")
	}
	issuedAt, err := platform.ParseTimestamp(claims.IssuedAt)
	if err != nil || claims.Audience == "" {
		return platform.NewError(platform.CodeUnauthenticated, false, "断言声明不完整")
	}
	expiresAt, err := platform.ParseTimestamp(claims.ExpiresAt)
	if err != nil {
		return platform.NewError(platform.CodeUnauthenticated, false, "断言声明不完整")
	}
	now := time.Now()
	if now.Add(ClockSkew).Before(issuedAt) || now.Add(-ClockSkew).After(expiresAt) {
		return platform.NewError(platform.CodeUnauthenticated, false, "断言已过期或未生效")
	}
	if expiresAt.Sub(issuedAt) > AssertionTTL+ClockSkew {
		return platform.NewError(platform.CodeUnauthenticated, false, "断言有效期超长")
	}
	if claims.Audience != expect.Audience || claims.Method != expect.Method || claims.Path != expect.Path || claims.BodyHash != expect.BodyHash {
		return platform.NewError(platform.CodeUnauthenticated, false, "断言与请求不符")
	}
	return nil
}

func validLoginScopes(claims platform.AssertionClaims) bool {
	if len(claims.LoginNetworkScopes) == 0 {
		return true
	}
	if claims.Issuer != "portal" || claims.Audience != "identity" || len(claims.LoginNetworkScopes) > 2 {
		return false
	}
	for _, scope := range claims.LoginNetworkScopes {
		if !platform.ValidHash(scope) {
			return false
		}
	}
	return true
}

var replayMu sync.Mutex

// claimReplay 维护进程内jti重放缓存；重复断言返回CONFLICT。
func claimReplay(claims platform.AssertionClaims) error {
	expiresAt, err := platform.ParseTimestamp(claims.ExpiresAt)
	if err != nil {
		return platform.NewError(platform.CodeUnauthenticated, false, "断言声明不完整")
	}
	replayMu.Lock()
	defer replayMu.Unlock()
	now := time.Now()
	key := claims.Issuer + "\x00" + claims.JTI
	if cached, ok := replayCache[key]; ok && cached.After(now.Add(-ClockSkew)) {
		return platform.NewError(platform.CodeConflict, false, "断言jti重放")
	}
	for k, expiry := range replayCache {
		if expiry.Before(now.Add(-ClockSkew)) {
			delete(replayCache, k)
		}
	}
	replayCache[key] = expiresAt
	return nil
}

var replayCache = map[string]time.Time{}

// SignOutboundRequest 为出站请求签发并附加服务断言与操作上下文头。
// 请求正文读取SHA-256后原样回填，供后续传输使用。
func SignOutboundRequest(ctx context.Context, r *http.Request, issuer, audience string, key platform.SigningKey) error {
	var bodyHash platform.Hash
	var body []byte
	if r.Body != nil && r.ContentLength != 0 {
		readBody, err := io.ReadAll(r.Body)
		if err != nil {
			return platform.NewError(platform.CodeInternal, false, "读取请求正文失败")
		}
		r.Body.Close()
		body = readBody
		bodyHash = platform.HashBytes(body)
		r.Body = io.NopCloser(bytes.NewReader(body))
	} else {
		bodyHash = platform.HashBytes(nil)
	}
	now := time.Now().UTC()
	claims := platform.AssertionClaims{
		Issuer:    issuer,
		Audience:  audience,
		Method:    r.Method,
		Path:      r.URL.Path,
		BodyHash:  bodyHash,
		IssuedAt:  platform.Timestamp(now.Format(time.RFC3339Nano)),
		ExpiresAt: platform.Timestamp(now.Add(AssertionTTL).Format(time.RFC3339Nano)),
		JTI:       platform.NewIdentifier(),
	}
	if op, ok := platform.OperationContextFrom(ctx); ok {
		claims.SiteID = op.Actor.SiteID
		if issuer == "portal" && audience == "traffic-guard" {
			claims.NetworkBudgetScopes = append([]platform.Hash(nil), op.NetworkBudgetScopes...)
		}
		if issuer == "portal" && audience == "identity" {
			claims.LoginNetworkScopes = append([]platform.Hash(nil), op.LoginNetworkScopes...)
		}
	}
	token, err := SignAssertion(key, claims)
	if err != nil {
		return err
	}
	r.Header.Set(platform.HeaderAuthorization, platform.AssertionScheme+token)
	applyOperationHeaders(ctx, r)
	return nil
}

// applyOperationHeaders 把调用链上的操作上下文与委托证明写入出站请求头。
func applyOperationHeaders(ctx context.Context, r *http.Request) {
	if ctx == nil {
		return
	}
	if op, ok := platform.OperationContextFrom(ctx); ok {
		r.Header.Set(platform.HeaderRequestID, op.RequestID)
		r.Header.Set(platform.HeaderOperationID, op.OperationID)
		r.Header.Set(platform.HeaderRootOperationID, op.RootOperationID)
		if op.CostClass != "" {
			r.Header.Set(platform.HeaderCostClass, op.CostClass)
		}
		if op.Deadline != "" {
			r.Header.Set(platform.HeaderDeadline, string(op.Deadline))
		}
		if op.CancellationID != "" {
			r.Header.Set(platform.HeaderCancellationID, op.CancellationID)
		}
		if op.DataEpoch != "" {
			r.Header.Set(platform.HeaderDataEpoch, op.DataEpoch)
		}
	}
	if proof := platform.PortalProofFrom(ctx); proof != "" {
		r.Header.Set(platform.HeaderPortalProof, proof)
	} else {
		r.Header.Del(platform.HeaderPortalProof)
	}
}

// SigningTransport 返回自动附加服务断言的http.RoundTripper，
// 供生成的生产方客户端复用；每次请求签发新断言。
func SigningTransport(issuer, audience string, key platform.SigningKey, next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &signingTransport{issuer: issuer, audience: audience, key: key, next: next}
}

type signingTransport struct {
	issuer   string
	audience string
	key      platform.SigningKey
	next     http.RoundTripper
}

func (t *signingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := SignOutboundRequest(r.Context(), r, t.issuer, t.audience, t.key); err != nil {
		return nil, err
	}
	return t.next.RoundTrip(r)
}

// privateNetworkPeer 是唯一的可信代理链解析，原地址只在内存使用。
// ClientAddress 按受信代理链取出访客的真实地址；只给入口做离线国家查询等不落地的用途。
func ClientAddress(remoteAddr, forwardedFor string, trustedProxyCIDRs []string) (netip.Addr, error) {
	return privateNetworkPeer(remoteAddr, forwardedFor, trustedProxyCIDRs)
}

func privateNetworkPeer(remoteAddr, forwardedFor string, trustedProxyCIDRs []string) (netip.Addr, error) {
	invalid := func() (netip.Addr, error) {
		return netip.Addr{}, platform.NewError(platform.CodeValidation, false, "网络来源或代理链不合法")
	}
	remote, err := netip.ParseAddrPort(remoteAddr)
	if err != nil || remote.Addr().Zone() != "" {
		return invalid()
	}
	peer := remote.Addr().Unmap()
	// 配置已由LoadServiceConfig在启动边界校验，内部只转成只读前缀。
	prefixes := make([]netip.Prefix, len(trustedProxyCIDRs))
	for i, cidr := range trustedProxyCIDRs {
		prefixes[i] = netip.MustParsePrefix(cidr)
	}
	trusted := func(addr netip.Addr) bool {
		for _, prefix := range prefixes {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}
	if trusted(peer) {
		if forwardedFor == "" || len(forwardedFor) > 4096 {
			return invalid()
		}
		hops := strings.Split(forwardedFor, ",")
		if len(hops) > 16 {
			return invalid()
		}
		for i := len(hops) - 1; i >= 0 && trusted(peer); i-- {
			addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
			if err != nil || addr.Zone() != "" {
				return invalid()
			}
			peer = addr.Unmap()
		}
	}
	return peer, nil
}

// PrivateBudgetNetworkScopes 固定30日当前/前期别名，IPv6使用/64，登录日范围不受影响。
func PrivateBudgetNetworkScopes(key platform.SigningKey, remoteAddr, forwardedFor string, trustedProxyCIDRs []string, siteID platform.Identifier, now time.Time) ([]platform.Hash, error) {
	peer, err := privateNetworkPeer(remoteAddr, forwardedFor, trustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	if !platform.ValidIdentifier(siteID) {
		return nil, platform.NewError(platform.CodeValidation, false, "预算站点标识不合法")
	}
	if peer.Is6() {
		prefix, err := peer.Prefix(64)
		if err != nil {
			return nil, platform.NewError(platform.CodeValidation, false, "预算网络前缀不合法")
		}
		peer = prefix.Masked().Addr()
	}
	period := now.UTC().Unix() / (30 * 24 * 60 * 60)
	site := platform.HashBytes([]byte(siteID))
	scopes := make([]platform.Hash, 0, 2)
	for _, index := range []int64{period, period - 1} {
		scope := "guard/network/30d/v1/" + site + "/" + strconv.FormatInt(index, 10)
		digest, err := PrivateOperationDigest(key, scope, []byte(peer.String()))
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, digest)
	}
	return scopes, nil
}

func validBudgetScopes(claims platform.AssertionClaims) bool {
	if len(claims.NetworkBudgetScopes) == 0 {
		return true
	}
	if claims.Issuer != "portal" || claims.Audience != "traffic-guard" || len(claims.NetworkBudgetScopes) > 4 {
		return false
	}
	for _, scope := range claims.NetworkBudgetScopes {
		if !platform.ValidHash(scope) {
			return false
		}
	}
	return true
}

// ResolveVisitorAddress 复用唯一可信代理链；结果只可在内存转交受签名内部请求。
func ResolveVisitorAddress(remoteAddr, forwardedFor string, trustedProxyCIDRs []string) (string, error) {
	peer, err := privateNetworkPeer(remoteAddr, forwardedFor, trustedProxyCIDRs)
	if err != nil {
		return "", err
	}
	return peer.String(), nil
}
