package serviceauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

const admissionTokenPurpose = "nw-admission-v1"
const guardSessionPurpose = "nw-guard-session-v1"

// SignAdmissionPermit 对完整费用许可签名，token本身不进入签名输入。
func SignAdmissionPermit(key platform.SigningKey, permit platform.AdmissionPermit) (platform.AdmissionPermit, error) {
	permit.Token = ""
	if err := validateAdmissionClaims(permit); err != nil {
		return platform.AdmissionPermit{}, err
	}
	token, err := signGuardClaims(key, admissionTokenPurpose, permit)
	if err != nil {
		return platform.AdmissionPermit{}, err
	}
	permit.Token = token
	return permit, nil
}

// VerifyAdmissionPermit 固定使用traffic-guard公钥与模型网关用途；不重新签发或续命。
// 调用方仍核对当前epoch、请求/主体归属和已消费许可事实。
func VerifyAdmissionPermit(anchors platform.TrustAnchors, token string) (platform.AdmissionPermit, error) {
	var permit platform.AdmissionPermit
	if err := verifyGuardClaims(anchors, token, admissionTokenPurpose, &permit); err != nil {
		return platform.AdmissionPermit{}, err
	}
	if permit.Token != "" {
		return platform.AdmissionPermit{}, platform.NewError(platform.CodeUnauthenticated, false, "许可签名载荷不合法")
	}
	if err := validateAdmissionClaims(permit); err != nil {
		return platform.AdmissionPermit{}, err
	}
	permit.Token = token
	return permit, nil
}

func validateAdmissionClaims(permit platform.AdmissionPermit) error {
	if permit.Audience != "model-gateway" || permit.Action == "" || len(permit.Action) > 128 || !platform.ValidIdentifier(permit.AdmissionID) || !platform.ValidIdentifier(permit.RequestID) || !platform.ValidIdentifier(permit.SubjectID) || !platform.ValidIdentifier(permit.SessionBudgetScope) || !platform.ValidIdentifier(permit.NetworkBudgetScope) || !platform.ValidIdentifier(permit.DataEpoch) {
		return platform.NewError(platform.CodeUnauthenticated, false, "费用许可声明不完整")
	}
	expires, err := platform.ParseTimestamp(permit.ExpiresAt)
	if err != nil || !expires.After(time.Now()) || expires.After(time.Now().Add(5*time.Minute)) {
		return platform.NewError(platform.CodeUnauthenticated, false, "费用许可已到期或有效期超限")
	}
	return nil
}

// SignGuardSession 签发至多十分钟的挑战通过证明；服务令牌不可充当此证明。
func SignGuardSession(key platform.SigningKey, claims platform.GuardSessionClaims) (string, error) {
	claims.Audience = "portal"
	if err := validateGuardSession(claims); err != nil {
		return "", err
	}
	return signGuardClaims(key, guardSessionPurpose, claims)
}

func VerifyGuardSession(anchors platform.TrustAnchors, token string) (platform.GuardSessionClaims, error) {
	var claims platform.GuardSessionClaims
	if err := verifyGuardClaims(anchors, token, guardSessionPurpose, &claims); err != nil {
		return platform.GuardSessionClaims{}, err
	}
	if err := validateGuardSession(claims); err != nil {
		return platform.GuardSessionClaims{}, err
	}
	return claims, nil
}

func validateGuardSession(claims platform.GuardSessionClaims) error {
	if claims.Audience != "portal" || !platform.ValidIdentifier(claims.SubjectID) || !platform.ValidIdentifier(claims.SessionID) || !platform.ValidIdentifier(claims.SiteID) || !platform.ValidIdentifier(claims.DataEpoch) {
		return platform.NewError(platform.CodeUnauthenticated, false, "挑战通过证明声明不完整")
	}
	expires, err := platform.ParseTimestamp(claims.ExpiresAt)
	if err != nil || !expires.After(time.Now()) || expires.After(time.Now().Add(10*time.Minute)) {
		return platform.NewError(platform.CodeUnauthenticated, false, "挑战通过证明已到期或有效期超限")
	}
	return nil
}

func signGuardClaims(key platform.SigningKey, purpose string, claims any) (string, error) {
	private, err := LoadSigningKey(key)
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "风控签名密钥不可用")
	}
	header, err := platform.CanonicalJSON(map[string]string{"alg": algEdDSA, "kid": key.KeyID, "typ": purpose})
	if err != nil {
		return "", err
	}
	payload, err := platform.CanonicalJSON(claims)
	if err != nil {
		return "", err
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(input))), nil
}

func verifyGuardClaims(anchors platform.TrustAnchors, token, purpose string, result any) error {
	invalid := func() error {
		return platform.NewError(platform.CodeUnauthenticated, false, "风控凭证验签失败")
	}
	if len(token) > 8192 {
		return invalid()
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return invalid()
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return invalid()
	}
	var header struct {
		Alg   string `json:"alg"`
		KeyID string `json:"kid"`
		Type  string `json:"typ"`
	}
	decoder := json.NewDecoder(bytes.NewReader(headerRaw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&header) != nil || header.Alg != algEdDSA || header.Type != purpose || header.KeyID == "" {
		return invalid()
	}
	if decoder.Decode(new(any)) != io.EOF {
		return invalid()
	}
	public, err := base64.RawURLEncoding.DecodeString(anchors.ServiceKeyID("traffic-guard", header.KeyID))
	if err != nil || len(public) != ed25519.PublicKeySize {
		return invalid()
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(ed25519.PublicKey(public), []byte(parts[0]+"."+parts[1]), signature) {
		return invalid()
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return invalid()
	}
	decoder = json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(result) != nil || decoder.Decode(new(any)) != io.EOF {
		return invalid()
	}
	return nil
}
