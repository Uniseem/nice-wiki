package serviceauth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

// DelegationTTL 用户委托证明的最长有效期。
const DelegationTTL = 10 * time.Second

// MaxProofLength 委托证明长度上限。
const MaxProofLength = 8192

// VerifyDelegation 验证portal签发的≤10秒用户委托证明：
// 用portal委托公钥验签，拒绝未知key与过期；核对bodyHash，
// route仅在期望值非空时核对（下游链路原样转发同一证明）。
// 权限仍须由业务函数经identity:checkAuthorization核实。
func VerifyDelegation(anchors platform.TrustAnchors, proof string, expect platform.DelegationExpectation) (platform.DelegationClaims, error) {
	if proof == "" || len(proof) > MaxProofLength {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明缺失或超长")
	}
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明格式不合法")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明头不合法")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil || header.Alg != algEdDSA {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明算法不受支持")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明载荷不合法")
	}
	var claims platform.DelegationClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明载荷不合法")
	}
	publicKeyRaw := anchors.DelegationKeyID(header.Kid)
	if publicKeyRaw == "" {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托公钥未登记")
	}
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(publicKeyRaw)
	if err != nil || len(publicKeyBytes) != ed25519.PublicKeySize {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "登记委托公钥不合法")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明签名不合法")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), []byte(parts[0]+"."+parts[1]), signature) {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明签名验证失败")
	}
	expiresAt, err := platform.ParseTimestamp(claims.ExpiresAt)
	if err != nil {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明声明不完整")
	}
	if time.Now().Add(-ClockSkew).After(expiresAt) {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明已过期")
	}
	if err := checkDelegationClaims(claims); err != nil {
		return platform.DelegationClaims{}, err
	}
	if claims.BodyHash != expect.BodyHash {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明与请求正文不符")
	}
	if expect.Route != "" && claims.Route != expect.Route {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明与路由不符")
	}
	return claims, nil
}

func checkDelegationClaims(claims platform.DelegationClaims) error {
	if claims.Actor.SubjectID == "" || claims.Actor.Kind == "" || claims.Actor.SiteID == "" {
		return platform.NewError(platform.CodeUnauthenticated, false, "委托主体不完整")
	}
	if claims.Route == "" || !platform.ValidHash(claims.BodyHash) || claims.SessionVersion < 0 {
		return platform.NewError(platform.CodeUnauthenticated, false, "委托声明不完整")
	}
	return nil
}
