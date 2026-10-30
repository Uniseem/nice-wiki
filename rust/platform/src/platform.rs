//! 与 Go 平台库（libs/platform）逐字节兼容的部分：服务配置与信任锚、Ed25519 签名材料、
//! JWS 形式的服务断言 / 用户委托 / 游标、私有摘要、标识与时间格式、Problem 错误响应。
//! edge 沿用 portal 的服务身份（iss=portal），search 沿用 knowledge 的，Go 服务的验证逻辑不变。

use std::collections::HashMap;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;
use bytes::Bytes;
use ed25519_dalek::{Signature, Signer, SigningKey, Verifier, VerifyingKey};
use hmac::{Hmac, Mac};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use time::format_description::well_known::Rfc3339;
use time::OffsetDateTime;

/// 服务配置（Go 的 platform.ServiceConfig 里 Rust 服务用到的字段）。
#[derive(Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct ServiceConfig {
    pub module: String,
    pub trust_anchor_ref: String,
    pub signing_key_ref: String,
    #[serde(default)]
    pub delegation_key_ref: String,
    #[serde(default)]
    pub budget_key_ref: String,
    pub dependencies: HashMap<String, String>,
    #[serde(default, rename = "publicBaseURL")]
    pub public_base_url: String,
    #[serde(default, rename = "trustedProxyCIDRs")]
    pub trusted_proxy_cidrs: Vec<String>,
    #[serde(default, rename = "geoipCountryDBRef")]
    pub geoip_country_db_ref: String,
    #[serde(default)]
    pub listen: String,
    #[serde(default)]
    pub data_root: String,
    /// 数据库连接串所在的私有文件。
    #[serde(default)]
    pub database_ref: String,
}

impl ServiceConfig {
    /// 读配置并核对 module：Rust 服务沿用对应 Go 组件的服务身份。
    pub fn load(path: &str, module: &str) -> anyhow::Result<Self> {
        let mut config: Self = serde_json::from_str(&std::fs::read_to_string(path)?)?;
        // 与 Go 侧 applyEnv 一致：部署时公开地址由环境变量给出。
        if let Ok(value) = std::env::var("NW_PUBLIC_BASE_URL") {
            config.public_base_url = value;
        }
        anyhow::ensure!(config.module == module, "服务配置的 module 必须是 {module}");
        Ok(config)
    }
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ServicePublicKey {
    slug: String,
    key_id: String,
    public_key: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct DelegationPublicKey {
    key_id: String,
    public_key: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TrustAnchors {
    services: Vec<ServicePublicKey>,
    #[serde(default)]
    portal_delegation_keys: Vec<DelegationPublicKey>,
}

impl TrustAnchors {
    pub fn load(path: &str) -> anyhow::Result<Self> {
        Ok(serde_json::from_str(&std::fs::read_to_string(path)?)?)
    }

    pub fn service(&self, slug: &str, key_id: &str) -> Option<VerifyingKey> {
        self.services.iter().find(|s| s.slug == slug && s.key_id == key_id).and_then(|s| public_key(&s.public_key))
    }

    pub fn delegation(&self, key_id: &str) -> Option<VerifyingKey> {
        self.portal_delegation_keys.iter().find(|k| k.key_id == key_id).and_then(|k| public_key(&k.public_key))
    }
}

fn public_key(encoded: &str) -> Option<VerifyingKey> {
    let raw: [u8; 32] = URL_SAFE_NO_PAD.decode(encoded).ok()?.try_into().ok()?;
    VerifyingKey::from_bytes(&raw).ok()
}

/// 签名密钥：{"keyId", "privateKeyRef"} 指向 PKCS#8 PEM 的 Ed25519 私钥。
pub struct Key {
    pub id: String,
    signing: SigningKey,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct KeyRef {
    key_id: String,
    private_key_ref: String,
}

/// Ed25519 的 PKCS#8 DER 是固定前缀加 32 字节种子。
const PKCS8_ED25519_PREFIX: [u8; 16] = [0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04, 0x20];

impl Key {
    pub fn load(path: &str) -> anyhow::Result<Self> {
        let reference: KeyRef = serde_json::from_str(&std::fs::read_to_string(path)?)?;
        let pem = std::fs::read_to_string(&reference.private_key_ref)?;
        let body: String = pem.lines().filter(|line| !line.starts_with("-----")).collect();
        let der = base64::engine::general_purpose::STANDARD.decode(body.trim())?;
        anyhow::ensure!(der.len() == 48 && der[..16] == PKCS8_ED25519_PREFIX, "私钥不是 PKCS#8 Ed25519");
        let seed: [u8; 32] = der[16..].try_into()?;
        Ok(Self { id: reference.key_id, signing: SigningKey::from_bytes(&seed) })
    }

    pub fn verifying(&self) -> VerifyingKey {
        self.signing.verifying_key()
    }

    /// JWS compact：头 {"alg":"EdDSA","kid":…,"typ":…}，载荷原样，Ed25519 签名。
    pub fn jws(&self, typ: &str, payload: &[u8]) -> String {
        let header = serde_json::json!({"alg": "EdDSA", "kid": self.id, "typ": typ});
        let input = format!("{}.{}", URL_SAFE_NO_PAD.encode(header.to_string()), URL_SAFE_NO_PAD.encode(payload));
        let signature = self.signing.sign(input.as_bytes());
        format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
    }

    /// 与 serviceauth.PrivateOperationDigest 相同：种子派生 HMAC 密钥，再对作用域长度、作用域、载荷做 HMAC-SHA256。
    pub fn private_digest(&self, scope: &str, payload: &[u8]) -> String {
        let mut derive = <Hmac<Sha256>>::new_from_slice(self.signing.as_bytes()).expect("HMAC 接受任意长度密钥");
        derive.update(b"nice-wiki/private-operation-key/v1");
        let secret = derive.finalize().into_bytes();
        let mut digest = <Hmac<Sha256>>::new_from_slice(&secret).expect("HMAC 接受任意长度密钥");
        digest.update(b"nice-wiki/private-operation-input/v1");
        digest.update(&(scope.len() as u32).to_be_bytes());
        digest.update(scope.as_bytes());
        digest.update(payload);
        hex::encode(digest.finalize().into_bytes())
    }

    /// serviceauth.SignCursor：载荷按 Go 的 []byte 编码（base64 标准字母表）。
    pub fn sign_cursor(&self, scope: &str, payload: &[u8]) -> String {
        let body = serde_json::json!({"scope": scope, "payload": base64::engine::general_purpose::STANDARD.encode(payload)});
        self.jws("NW-CURSOR", body.to_string().as_bytes())
    }

    pub fn verify_cursor(&self, scope: &str, token: &str) -> Option<Vec<u8>> {
        let (header, payload) = verify_jws(token, 4096, |kid| (kid == self.id).then(|| self.verifying()))?;
        if header.typ != "NW-CURSOR" {
            return None;
        }
        #[derive(Deserialize)]
        struct Envelope {
            scope: String,
            payload: String,
        }
        let envelope: Envelope = serde_json::from_slice(&payload).ok()?;
        let raw = base64::engine::general_purpose::STANDARD.decode(envelope.payload).ok()?;
        (envelope.scope == scope && !raw.is_empty() && raw.len() <= 1024).then_some(raw)
    }
}

#[derive(Deserialize)]
pub struct JwsHeader {
    pub alg: String,
    pub kid: String,
    pub typ: String,
}

/// 校验 EdDSA JWS 并返回头与载荷；公钥由 kid 查到。
pub fn verify_jws(token: &str, max_len: usize, key_for: impl Fn(&str) -> Option<VerifyingKey>) -> Option<(JwsHeader, Vec<u8>)> {
    if token.is_empty() || token.len() > max_len {
        return None;
    }
    let mut parts = token.split('.');
    let (h, p, s) = (parts.next()?, parts.next()?, parts.next()?);
    if parts.next().is_some() {
        return None;
    }
    let header: JwsHeader = serde_json::from_slice(&URL_SAFE_NO_PAD.decode(h).ok()?).ok()?;
    if header.alg != "EdDSA" || header.kid.is_empty() {
        return None;
    }
    let key = key_for(&header.kid)?;
    let signature: [u8; 64] = URL_SAFE_NO_PAD.decode(s).ok()?.try_into().ok()?;
    key.verify(format!("{h}.{p}").as_bytes(), &Signature::from_bytes(&signature)).ok()?;
    Some((header, URL_SAFE_NO_PAD.decode(p).ok()?))
}

/// AuthContext（与 Go 的 platform.AuthContext 字段一致）。
#[derive(Serialize, Deserialize, Clone, Default)]
#[serde(rename_all = "camelCase")]
pub struct AuthContext {
    pub subject_id: String,
    pub kind: String,
    pub site_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub session_id: String,
    #[serde(default, skip_serializing_if = "is_zero")]
    pub session_revision: i64,
    #[serde(default)]
    pub permission_revision: i64,
    #[serde(default)]
    pub grants: Vec<String>,
    pub expires_at: String,
}

fn is_zero(value: &i64) -> bool {
    *value == 0
}

/// 用户委托证明（≤10 秒），沿用 portal 的委托密钥。
pub fn sign_delegation(key: &Key, actor: &AuthContext, route: &str, body_hash: &str, expires_at: &str) -> String {
    let claims = serde_json::json!({"actor": actor, "sessionVersion": actor.session_revision, "route": route, "bodyHash": body_hash, "expiresAt": expires_at});
    key.jws("JWT", claims.to_string().as_bytes())
}

/// traffic-guard 签的挑战通过证明（GuardSessionClaims），只认本站点、未过期、≤10 分钟。
#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct GuardSession {
    pub subject_id: String,
    pub site_id: String,
    pub session_id: String,
    pub data_epoch: String,
    pub expires_at: String,
    pub audience: String,
}

pub fn verify_guard_session(anchors: &TrustAnchors, token: &str) -> Option<GuardSession> {
    let (header, payload) = verify_jws(token, 8192, |kid| anchors.service("traffic-guard", kid))?;
    if header.typ != "nw-guard-session-v1" {
        return None;
    }
    let claims: GuardSession = serde_json::from_slice(&payload).ok()?;
    let expires = parse_time(&claims.expires_at)?;
    let now = OffsetDateTime::now_utc();
    let valid = claims.audience == "portal" && [&claims.subject_id, &claims.session_id, &claims.site_id, &claims.data_epoch].iter().all(|v| valid_identifier(v));
    (valid && expires > now && expires <= now + time::Duration::minutes(10)).then_some(claims)
}

/// 启动时核对委托私钥与信任锚里登记的委托公钥一致。
pub fn check_delegation_key(anchors: &TrustAnchors, key: &Key) -> anyhow::Result<()> {
    anyhow::ensure!(anchors.delegation(&key.id) == Some(key.verifying()), "委托签名身份与信任锚不一致");
    Ok(())
}

pub fn new_id() -> String {
    let mut raw = [0u8; 16];
    getrandom::getrandom(&mut raw).expect("随机数不可用");
    hex::encode(raw)
}

pub fn sha256_hex(data: &[u8]) -> String {
    hex::encode(Sha256::digest(data))
}

pub fn valid_identifier(value: &str) -> bool {
    !value.is_empty() && value.len() <= 128 && value.bytes().all(|b| b.is_ascii_alphanumeric() || b"._:-".contains(&b))
}

pub fn timestamp(at: OffsetDateTime) -> String {
    at.format(&Rfc3339).expect("UTC 时间总能格式化")
}

pub fn now_plus(seconds: i64) -> OffsetDateTime {
    OffsetDateTime::now_utc() + time::Duration::seconds(seconds)
}

pub fn parse_time(value: &str) -> Option<OffsetDateTime> {
    OffsetDateTime::parse(value, &Rfc3339).ok()
}

/// 与 Go 的 platform.CanonicalJSON 逐字节相同：对象键按字节排序、紧凑，字符串按 Go 的 encoding/json 转义
/// （<、>、&、U+2028、U+2029 写成 \u 形式）。正文块的摘要要和 Go 服务算出的一致。
pub fn canonical(value: &serde_json::Value) -> Vec<u8> {
    let mut out = Vec::new();
    write_canonical(&mut out, value);
    out
}

fn write_canonical(out: &mut Vec<u8>, value: &serde_json::Value) {
    use serde_json::Value;
    match value {
        Value::Null => out.extend_from_slice(b"null"),
        Value::Bool(b) => out.extend_from_slice(if *b { b"true" } else { b"false" }),
        Value::Number(n) => out.extend_from_slice(n.to_string().as_bytes()),
        Value::String(s) => write_go_string(out, s),
        Value::Array(items) => {
            out.push(b'[');
            for (index, item) in items.iter().enumerate() {
                if index > 0 {
                    out.push(b',');
                }
                write_canonical(out, item);
            }
            out.push(b']');
        }
        Value::Object(map) => {
            let mut keys: Vec<&String> = map.keys().collect();
            keys.sort();
            out.push(b'{');
            for (index, key) in keys.into_iter().enumerate() {
                if index > 0 {
                    out.push(b',');
                }
                write_go_string(out, key);
                out.push(b':');
                write_canonical(out, &map[key]);
            }
            out.push(b'}');
        }
    }
}

fn write_go_string(out: &mut Vec<u8>, text: &str) {
    const HEX: &[u8; 16] = b"0123456789abcdef";
    out.push(b'"');
    for c in text.chars() {
        match c {
            '"' => out.extend_from_slice(b"\\\""),
            '\\' => out.extend_from_slice(b"\\\\"),
            '\n' => out.extend_from_slice(b"\\n"),
            '\r' => out.extend_from_slice(b"\\r"),
            '\t' => out.extend_from_slice(b"\\t"),
            '\u{8}' => out.extend_from_slice(b"\\b"),
            '\u{c}' => out.extend_from_slice(b"\\f"),
            c if (c as u32) < 0x20 || c == '<' || c == '>' || c == '&' => {
                let n = c as u32;
                out.extend_from_slice(b"\\u00");
                out.push(HEX[(n >> 4) as usize]);
                out.push(HEX[(n & 0xf) as usize]);
            }
            '\u{2028}' => out.extend_from_slice(b"\\u2028"),
            '\u{2029}' => out.extend_from_slice(b"\\u2029"),
            c => {
                let mut buffer = [0u8; 4];
                out.extend_from_slice(c.encode_utf8(&mut buffer).as_bytes());
            }
        }
    }
    out.push(b'"');
}

/// 平台错误：登记过的错误码 + 给浏览器看的说明。
#[derive(Debug)]
pub struct Problem {
    pub code: &'static str,
    pub detail: String,
    pub retryable: bool,
}

impl Problem {
    pub fn new(code: &'static str, detail: impl Into<String>) -> Self {
        let retryable = matches!(code, "DEPENDENCY_UNAVAILABLE" | "DEADLINE_EXCEEDED");
        Self { code, detail: detail.into(), retryable }
    }

    pub fn status(&self) -> u16 {
        problem_status(self.code)
    }

    pub fn body(&self, request_id: &str) -> Bytes {
        let value = serde_json::json!({"type": "about:blank", "title": "请求未能完成", "status": self.status(), "code": self.code, "detail": self.detail, "requestId": request_id, "retryable": self.retryable});
        Bytes::from(value.to_string())
    }
}

/// 上游 Problem 里的错误码映射到登记过的静态字符串（未知按 INTERNAL）。
pub fn known_code(code: &str) -> &'static str {
    const CODES: [&str; 20] = [
        "VALIDATION", "UNAUTHENTICATED", "FORBIDDEN", "NOT_FOUND", "INVOCATION_NOT_ACCEPTED", "GONE", "CHALLENGE_REQUIRED", "CONFLICT", "VERSION_MISMATCH", "LOCKED",
        "UNSUPPORTED_SCHEMA", "UNSUPPORTED_CAPABILITY", "RATE_LIMITED", "BUDGET_EXCEEDED", "CANCELLED", "DEADLINE_EXCEEDED", "DEPENDENCY_UNAVAILABLE", "STALE_RESULT", "INTEGRITY_ERROR", "INTERNAL",
    ];
    CODES.iter().find(|c| **c == code).copied().unwrap_or("INTERNAL")
}

pub fn problem_status(code: &str) -> u16 {
    match code {
        "VALIDATION" => 400,
        "UNAUTHENTICATED" => 401,
        "FORBIDDEN" => 403,
        "NOT_FOUND" | "INVOCATION_NOT_ACCEPTED" => 404,
        "GONE" => 410,
        "CHALLENGE_REQUIRED" | "RATE_LIMITED" | "BUDGET_EXCEEDED" => 429,
        "CONFLICT" | "VERSION_MISMATCH" | "LOCKED" | "CANCELLED" | "STALE_RESULT" => 409,
        "UNSUPPORTED_SCHEMA" | "UNSUPPORTED_CAPABILITY" | "INTEGRITY_ERROR" => 422,
        "DEADLINE_EXCEEDED" => 504,
        "DEPENDENCY_UNAVAILABLE" => 503,
        _ => 500,
    }
}
