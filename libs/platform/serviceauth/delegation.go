package serviceauth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"

	"nice-wiki/libs/platform"
)

const activationReceiptPurpose = "activation-receipt-v1"

// ActivationReceiptClaims 是有状态域完成恢复切换或换回后签名的规范载荷。
type ActivationReceiptClaims struct {
	Version         int                 `json:"version"`
	RestoreID       platform.Identifier `json:"restoreId"`
	Module          string              `json:"module"`
	StageID         platform.Identifier `json:"stageId"`
	TargetDataEpoch platform.Identifier `json:"targetDataEpoch"`
	State           string              `json:"state"`
	ManifestHash    platform.Hash       `json:"manifestHash"`
	RecordedAt      platform.Timestamp  `json:"recordedAt"`
}

type activationReceiptHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

type activationReceiptProof struct {
	Version     int           `json:"version"`
	Issuer      string        `json:"issuer"`
	Purpose     string        `json:"purpose"`
	PayloadHash platform.Hash `json:"payloadHash"`
}

// SignActivationReceipt 签名规范回执；restoreId及全部回执字段绑定到本服务。
func SignActivationReceipt(key platform.SigningKey, claims ActivationReceiptClaims) (string, error) {
	normalized, err := normalizeActivationReceiptClaims(claims)
	if err != nil {
		return "", err
	}
	if !platform.ValidIdentifier(key.KeyID) {
		return "", platform.NewError(platform.CodeValidation, false, "激活回执密钥ID不合法")
	}
	payload, err := platform.CanonicalJSON(normalized)
	if err != nil {
		return "", err
	}
	privateKey, err := LoadSigningKey(key)
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "激活回执签名密钥不可用")
	}
	header, err := platform.CanonicalJSON(activationReceiptHeader{Algorithm: algEdDSA, KeyID: key.KeyID, Type: "NW-ACTIVATION-RECEIPT"})
	if err != nil {
		return "", err
	}
	proof, err := platform.CanonicalJSON(activationReceiptProof{
		Version:     1,
		Issuer:      normalized.Module,
		Purpose:     activationReceiptPurpose,
		PayloadHash: platform.HashBytes(payload),
	})
	if err != nil {
		return "", err
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(proof)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(input))), nil
}

// VerifyActivationReceipt 验证持久回执来源；HTTP服务断言另行证明调用方。
func VerifyActivationReceipt(anchors platform.TrustAnchors, issuer string, claims ActivationReceiptClaims, signature string) error {
	invalid := func(message string) error {
		return platform.NewError(platform.CodeUnauthenticated, false, "%s", message)
	}
	normalized, err := normalizeActivationReceiptClaims(claims)
	if err != nil || issuer == "" || issuer != normalized.Module {
		return invalid("激活回执声明或发行方不合法")
	}
	if len(signature) == 0 || len(signature) > 4096 {
		return invalid("激活回执签名长度不合法")
	}
	parts := strings.Split(signature, ".")
	if len(parts) != 3 {
		return invalid("激活回执签名格式不合法")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return invalid("激活回执头不可解码")
	}
	var header activationReceiptHeader
	if err := decodeCanonicalReceiptJSON(headerRaw, &header); err != nil || header.Algorithm != algEdDSA || !platform.ValidIdentifier(header.KeyID) || header.Type != "NW-ACTIVATION-RECEIPT" {
		return invalid("激活回执头不合法")
	}
	publicKeyRaw := anchors.ServiceKeyID(issuer, header.KeyID)
	publicKey, err := base64.RawURLEncoding.DecodeString(publicKeyRaw)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return invalid("激活回执公钥未登记")
	}
	signatureRaw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(parts[0]+"."+parts[1]), signatureRaw) {
		return invalid("激活回执签名无效")
	}
	proofRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return invalid("激活回执证明不可解码")
	}
	var proof activationReceiptProof
	if err := decodeCanonicalReceiptJSON(proofRaw, &proof); err != nil || proof.Version != 1 || proof.Issuer != issuer || proof.Purpose != activationReceiptPurpose || !platform.ValidHash(proof.PayloadHash) {
		return invalid("激活回执证明声明不合法")
	}
	payload, err := platform.CanonicalJSON(normalized)
	if err != nil || platform.HashBytes(payload) != proof.PayloadHash {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执载荷与签名绑定不符")
	}
	return nil
}

// normalizeActivationReceiptClaims 统一回执时间表示后返回可摘要的Claims。
func normalizeActivationReceiptClaims(claims ActivationReceiptClaims) (ActivationReceiptClaims, error) {
	recordedAt, err := platform.ParseTimestamp(claims.RecordedAt)
	if err != nil || recordedAt.IsZero() {
		return ActivationReceiptClaims{}, platform.NewError(platform.CodeValidation, false, "激活回执时间不合法")
	}
	if claims.Version != 1 ||
		!platform.ValidIdentifier(claims.RestoreID) ||
		len(claims.Module) > 64 ||
		!platform.ValidIdentifier(claims.Module) ||
		!platform.ValidIdentifier(claims.StageID) ||
		!platform.ValidIdentifier(claims.TargetDataEpoch) ||
		(claims.State != "switched" && claims.State != "rolled-back") ||
		!platform.ValidHash(claims.ManifestHash) {
		return ActivationReceiptClaims{}, platform.NewError(platform.CodeValidation, false, "激活回执声明不合法")
	}
	claims.RecordedAt = platform.Timestamp(recordedAt.UTC().Format(time.RFC3339Nano))
	return claims, nil
}

func decodeCanonicalReceiptJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执JSON包含多余值")
	}
	canonical, err := platform.CanonicalJSON(target)
	if err != nil || !bytes.Equal(canonical, raw) {
		return platform.NewError(platform.CodeIntegrityError, false, "激活回执JSON不是规范编码")
	}
	return nil
}

// PrivateOperationDigest 为含秘密的幂等输入生成不可公开猜测的私有摘要。
// 调用方保存keyId并在轮换过渡期保留旧key；不保存payload或原始秘密。
func PrivateOperationDigest(key platform.SigningKey, scope string, payload []byte) (platform.Hash, error) {
	if scope == "" || len(scope) > 128 {
		return "", platform.NewError(platform.CodeValidation, false, "私有操作摘要作用域不合法")
	}
	privateKey, err := LoadSigningKey(key)
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "私有操作摘要密钥不可用")
	}
	seed := privateKey.Seed()
	defer clear(seed)
	derive := hmac.New(sha256.New, seed)
	derive.Write([]byte("nice-wiki/private-operation-key/v1"))
	secret := derive.Sum(nil)
	defer clear(secret)
	digest := hmac.New(sha256.New, secret)
	digest.Write([]byte("nice-wiki/private-operation-input/v1"))
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(scope)))
	digest.Write(size[:])
	digest.Write([]byte(scope))
	digest.Write(payload)
	return platform.Hash(hex.EncodeToString(digest.Sum(nil))), nil
}

// SignDelegation 签名机制由平台统一实现，portal只负责准备已授权声明。
func SignDelegation(key platform.SigningKey, claims platform.DelegationClaims) (string, error) {
	if err := checkDelegationClaims(claims); err != nil {
		return "", platform.NewError(platform.CodeValidation, false, "委托声明不完整")
	}
	expires, err := platform.ParseTimestamp(claims.ExpiresAt)
	if err != nil || !expires.After(time.Now()) || expires.After(time.Now().Add(DelegationTTL)) {
		return "", platform.NewError(platform.CodeValidation, false, "委托有效期必须在十秒以内")
	}
	privateKey, err := LoadSigningKey(key)
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "委托签名密钥不可用")
	}
	header, _ := json.Marshal(map[string]string{"alg": algEdDSA, "kid": key.KeyID, "typ": "JWT"})
	body, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(input))), nil
}

// GenerateSessionSecrets 不共享秘密；jti使用符合Identifier的十六进制编码。
func GenerateSessionSecrets() (platform.SessionSecrets, error) {
	var raw [96]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return platform.SessionSecrets{}, platform.NewError(platform.CodeDependencyUnavailable, true, "会话随机源不可用")
	}
	return platform.SessionSecrets{
		AccessNonce:  hex.EncodeToString(raw[:32]),
		RefreshToken: base64.RawURLEncoding.EncodeToString(raw[32:64]),
		CSRFToken:    base64.RawURLEncoding.EncodeToString(raw[64:]),
	}, nil
}

// SignSessionAccess 固定发行方、接收方和用途，不能用客户端参数选算法。
func SignSessionAccess(key platform.SigningKey, claims platform.SessionAccessClaims) (string, error) {
	claims.Issuer, claims.Audience = "identity", "portal"
	if !validSessionClaims(claims) {
		return "", platform.NewError(platform.CodeValidation, false, "会话访问声明不完整或期限不合法")
	}
	privateKey, err := LoadSigningKey(key)
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "会话签名密钥不可用")
	}
	header, _ := json.Marshal(map[string]string{"alg": algEdDSA, "kid": key.KeyID, "typ": "NW-SESSION"})
	body, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(input))), nil
}

// VerifySessionAccess 只证明令牌来源；撤销、权限和认证代次仍由identity核权。
func VerifySessionAccess(anchors platform.TrustAnchors, token string) (platform.SessionAccessClaims, error) {
	invalid := func() (platform.SessionAccessClaims, error) {
		return platform.SessionAccessClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "会话访问令牌无效或已过期")
	}
	if len(token) == 0 || len(token) > MaxTokenLength {
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
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if json.Unmarshal(headerRaw, &header) != nil || header.Alg != algEdDSA || header.Typ != "NW-SESSION" || header.Kid == "" {
		return invalid()
	}
	encodedKey := anchors.ServiceKeyID("identity", header.Kid)
	publicKey, err := base64.RawURLEncoding.DecodeString(encodedKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return invalid()
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(parts[0]+"."+parts[1]), signature) {
		return invalid()
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return invalid()
	}
	var claims platform.SessionAccessClaims
	if json.Unmarshal(body, &claims) != nil || !validSessionClaims(claims) {
		return invalid()
	}
	return claims, nil
}

func validSessionClaims(claims platform.SessionAccessClaims) bool {
	if claims.Issuer != "identity" || claims.Audience != "portal" ||
		!platform.ValidIdentifier(claims.SessionID) || !platform.ValidIdentifier(claims.AccountID) ||
		!platform.ValidIdentifier(claims.SiteID) || !platform.ValidIdentifier(claims.AuthEpoch) ||
		!platform.ValidIdentifier(claims.DataEpoch) || !platform.ValidIdentifier(claims.TokenID) ||
		claims.CredentialRevision < 0 || claims.PermissionRevision < 0 || claims.SessionRevision < 0 {
		return false
	}
	now := time.Now().Unix()
	return claims.IssuedAt >= 0 && claims.IssuedAt <= now+30 && claims.ExpiresAt > now &&
		claims.ExpiresAt > claims.IssuedAt && claims.ExpiresAt-claims.IssuedAt <= 900
}

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
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil || header.Alg != algEdDSA || header.Typ != "JWT" {
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
	if now := time.Now(); !expiresAt.After(now) || expiresAt.After(now.Add(DelegationTTL)) {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeUnauthenticated, false, "委托证明已过期")
	}
	if err := checkDelegationClaims(claims); err != nil {
		return platform.DelegationClaims{}, err
	}
	if expect.CallerService == "" {
		return platform.DelegationClaims{}, platform.NewError(platform.CodeInternal, false, "委托验证缺少已认证服务来源")
	}
	// portal首跳绑定浏览器原文；后续请求正文由该跳服务断言绑定，
	// 原委托只提供原始用户来源，各拥有服务仍即时核验权限和资源。
	if expect.CallerService == "portal" && claims.BodyHash != expect.BodyHash {
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
	if int64(claims.SessionVersion) != claims.Actor.SessionRevision ||
		(claims.Actor.Kind == "admin" && (claims.Actor.SessionID == "" || claims.SessionVersion < 1)) {
		return platform.NewError(platform.CodeUnauthenticated, false, "委托会话版本不完整或不一致")
	}
	return nil
}

// cursorEnvelope 把分页位置与调用方的查询、主体及数据代次绑定。
type cursorEnvelope struct {
	Scope   string `json:"scope"`
	Payload []byte `json:"payload"`
}

// SignCursor 用本服务密钥签发分页游标。scope 由可信调用方构造；游标
// 仅承载查询位置，不替代身份与资源授权。
func SignCursor(key platform.SigningKey, scope string, payload []byte) (string, error) {
	if scope == "" || len(scope) > 512 || len(payload) == 0 || len(payload) > 1024 {
		return "", platform.NewError(platform.CodeValidation, false, "游标作用域或载荷长度不合法")
	}
	privateKey, err := LoadSigningKey(key)
	if err != nil {
		return "", platform.NewError(platform.CodeDependencyUnavailable, true, "游标签名密钥不可用")
	}
	header, _ := json.Marshal(map[string]string{"alg": algEdDSA, "kid": key.KeyID, "typ": "NW-CURSOR"})
	body, _ := json.Marshal(cursorEnvelope{Scope: scope, Payload: payload})
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(input))), nil
}

// VerifyCursor 验证专用类型、固定密钥、签名和查询作用域后返回分页位置。
func VerifyCursor(key platform.SigningKey, scope string, token string) ([]byte, error) {
	invalid := func() ([]byte, error) {
		return nil, platform.NewError(platform.CodeValidation, false, "分页游标不合法或作用域不符")
	}
	if len(token) == 0 || len(token) > 4096 {
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
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if json.Unmarshal(headerRaw, &header) != nil || header.Alg != algEdDSA || header.Kid != key.KeyID || header.Typ != "NW-CURSOR" {
		return invalid()
	}
	privateKey, err := LoadSigningKey(key)
	if err != nil {
		return nil, platform.NewError(platform.CodeDependencyUnavailable, true, "游标签名密钥不可用")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(privateKey.Public().(ed25519.PublicKey), []byte(parts[0]+"."+parts[1]), signature) {
		return invalid()
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return invalid()
	}
	var envelope cursorEnvelope
	if json.Unmarshal(body, &envelope) != nil || envelope.Scope != scope || len(envelope.Payload) == 0 || len(envelope.Payload) > 1024 {
		return invalid()
	}
	return envelope.Payload, nil
}
