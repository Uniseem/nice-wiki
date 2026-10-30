//! 入站请求的可信操作上下文（与 Go 的 opctx.BuildOperationContext 相同）：
//! 服务断言确定调用方（核 aud / 方法 / 路径 / 正文摘要 / 有效期，jti 只收一次），
//! 带 X-Portal-Proof 时验用户委托得到主体，否则主体是调用服务本身；请求声明的数据代次必须是当前代次。
//! 管理接口的用户权限另经 identity 即时核对（authorize）。

use std::collections::HashMap;
use std::sync::Mutex;
use std::time::Duration;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;
use hyper::{HeaderMap, Method};
use serde::Deserialize;
use time::OffsetDateTime;

use crate::platform::{new_id, now_plus, parse_time, sha256_hex, timestamp, valid_identifier, verify_jws, AuthContext, Problem, TrustAnchors};
use crate::upstream::{Operation, Upstream};

const CLOCK_SKEW: i64 = 30;
const ASSERTION_TTL: i64 = 60;
const DELEGATION_TTL: i64 = 10;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct AssertionClaims {
    iss: String,
    aud: String,
    method: String,
    path: String,
    body_hash: String,
    issued_at: String,
    expires_at: String,
    jti: String,
    #[serde(default)]
    site_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct DelegationClaims {
    actor: AuthContext,
    session_version: i64,
    route: String,
    body_hash: String,
    expires_at: String,
}

pub struct Verifier {
    anchors: TrustAnchors,
    slug: String,
    replay: Mutex<HashMap<String, OffsetDateTime>>,
}

fn unauthenticated(detail: &str) -> Problem {
    Problem::new("UNAUTHENTICATED", detail)
}

fn header<'a>(headers: &'a HeaderMap, name: &str) -> &'a str {
    headers.get(name).and_then(|v| v.to_str().ok()).unwrap_or_default()
}

impl Verifier {
    pub fn new(anchors: TrustAnchors, slug: &str) -> Self {
        Self { anchors, slug: slug.to_owned(), replay: Mutex::new(HashMap::new()) }
    }

    /// path 是解码后的请求路径（Go 的 r.URL.Path）；data_epoch 是本服务读到的当前代次。
    pub fn operation(&self, method: &Method, path: &str, headers: &HeaderMap, body: &[u8], data_epoch: &str) -> Result<Operation, Problem> {
        let body_hash = sha256_hex(body);
        let token = header(headers, "authorization").strip_prefix("Bearer ").unwrap_or_default();
        let claims = self.assertion(token, method, path, &body_hash)?;
        let mut op = Operation { caller: claims.iss.clone(), data_epoch: data_epoch.to_owned(), ..Default::default() };
        let pick = |name: &str| Some(header(headers, name)).filter(|v| valid_identifier(v)).map(str::to_owned);
        op.request_id = pick("x-request-id").unwrap_or_else(new_id);
        op.operation_id = pick("x-operation-id").unwrap_or_else(new_id);
        op.root_operation_id = pick("x-root-operation-id").unwrap_or_else(|| op.operation_id.clone());
        let key = header(headers, "idempotency-key");
        if !key.is_empty() {
            if !valid_identifier(key) {
                return Err(Problem::new("VALIDATION", "请求幂等键不合法"));
            }
            op.operation_id = key.to_owned();
            if header(headers, "x-root-operation-id").is_empty() {
                op.root_operation_id = key.to_owned();
            }
        }
        op.deadline = match header(headers, "x-deadline") {
            "" => timestamp(now_plus(30)),
            raw => {
                let at = parse_time(raw).ok_or_else(|| Problem::new("VALIDATION", "请求截止不合法"))?;
                if at <= OffsetDateTime::now_utc() {
                    return Err(Problem::new("DEADLINE_EXCEEDED", "请求已超过截止"));
                }
                timestamp(at)
            }
        };
        match header(headers, "x-cost-class") {
            "" => {}
            class @ ("qa" | "background" | "admin-probe") => op.cost_class = class.to_owned(),
            _ => return Err(Problem::new("VALIDATION", "费用类别不合法")),
        }
        let declared = header(headers, "x-data-epoch");
        if !declared.is_empty() && declared != data_epoch {
            return Err(Problem::new("VERSION_MISMATCH", "数据代次与服务端不一致"));
        }
        let proof = header(headers, "x-portal-proof");
        if proof.is_empty() {
            op.actor = AuthContext { subject_id: claims.iss.clone(), kind: "service".into(), site_id: "system".into(), expires_at: op.deadline.clone(), ..Default::default() };
            if !claims.site_id.is_empty() {
                op.actor.site_id = claims.site_id.clone();
            }
        } else {
            let delegation = self.delegation(proof, &claims.iss, &body_hash)?;
            if !claims.site_id.is_empty() && claims.site_id != delegation.actor.site_id {
                return Err(Problem::new("FORBIDDEN", "服务断言站点与用户委托不一致"));
            }
            op.actor = delegation.actor;
        }
        op.site_id = op.actor.site_id.clone();
        Ok(op)
    }

    fn assertion(&self, token: &str, method: &Method, path: &str, body_hash: &str) -> Result<AssertionClaims, Problem> {
        let payload = token.split('.').nth(1).ok_or_else(|| unauthenticated("断言缺失或格式不合法"))?;
        let claims: AssertionClaims = URL_SAFE_NO_PAD.decode(payload).ok().and_then(|raw| serde_json::from_slice(&raw).ok()).ok_or_else(|| unauthenticated("断言载荷不合法"))?;
        verify_jws(token, 4096, |kid| self.anchors.service(&claims.iss, kid)).ok_or_else(|| unauthenticated("断言签名验证失败"))?;
        if !claims.site_id.is_empty() && !valid_identifier(&claims.site_id) {
            return Err(unauthenticated("断言站点作用域不合法"));
        }
        let (issued, expires) = match (parse_time(&claims.issued_at), parse_time(&claims.expires_at)) {
            (Some(a), Some(b)) => (a, b),
            _ => return Err(unauthenticated("断言声明不完整")),
        };
        let now = OffsetDateTime::now_utc();
        let skew = time::Duration::seconds(CLOCK_SKEW);
        if now + skew < issued || now - skew > expires || expires - issued > time::Duration::seconds(ASSERTION_TTL + CLOCK_SKEW) {
            return Err(unauthenticated("断言已过期或未生效"));
        }
        if claims.aud != self.slug || claims.method != method.as_str() || claims.path != path || claims.body_hash != body_hash {
            return Err(unauthenticated("断言与请求不符"));
        }
        let mut replay = self.replay.lock().unwrap();
        replay.retain(|_, expiry| *expiry + skew > now);
        let key = format!("{}\u{0}{}", claims.iss, claims.jti);
        if replay.contains_key(&key) {
            return Err(Problem::new("CONFLICT", "断言jti重放"));
        }
        replay.insert(key, expires);
        Ok(claims)
    }

    fn delegation(&self, proof: &str, caller: &str, body_hash: &str) -> Result<DelegationClaims, Problem> {
        let (header, payload) = verify_jws(proof, 8192, |kid| self.anchors.delegation(kid)).ok_or_else(|| unauthenticated("委托证明签名验证失败"))?;
        if header.typ != "JWT" {
            return Err(unauthenticated("委托证明算法不受支持"));
        }
        let claims: DelegationClaims = serde_json::from_slice(&payload).map_err(|_| unauthenticated("委托证明载荷不合法"))?;
        let expires = parse_time(&claims.expires_at).ok_or_else(|| unauthenticated("委托证明声明不完整"))?;
        let now = OffsetDateTime::now_utc();
        if expires <= now || expires > now + time::Duration::seconds(DELEGATION_TTL) {
            return Err(unauthenticated("委托证明已过期"));
        }
        let actor = &claims.actor;
        if actor.subject_id.is_empty() || actor.kind.is_empty() || actor.site_id.is_empty() || claims.route.is_empty() || claims.body_hash.len() != 64 {
            return Err(unauthenticated("委托声明不完整"));
        }
        if claims.session_version != actor.session_revision || (actor.kind == "admin" && (actor.session_id.is_empty() || claims.session_version < 1)) {
            return Err(unauthenticated("委托会话版本不完整或不一致"));
        }
        // portal 首跳绑定浏览器原文；后续各跳的正文由该跳的服务断言绑定。
        if caller == "portal" && claims.body_hash != body_hash {
            return Err(unauthenticated("委托证明与请求正文不符"));
        }
        Ok(claims)
    }
}

/// 管理接口的用户权限：要求当前后台会话，再请 identity 即时核对（与 apivalidate.NewIdentityAuthorizer 相同）。
pub async fn authorize(upstream: &Upstream, op: &Operation, proof: &str, permission: &str, action: &str) -> Result<(), Problem> {
    let actor = &op.actor;
    if actor.kind != "admin" || actor.session_id.is_empty() || actor.session_revision < 1 {
        return Err(Problem::new("UNAUTHENTICATED", "此接口要求当前后台会话"));
    }
    let body = serde_json::json!({
        "subjectId": actor.subject_id, "sessionId": actor.session_id, "siteId": actor.site_id,
        "permission": permission, "action": action, "resourceId": null, "expectedPermissionRevision": actor.permission_revision,
    });
    let proof = Some(proof).filter(|p| !p.is_empty());
    let result = upstream.request("identity", Method::POST, "/v1/authorizations/check", Some(&body), op, proof, Duration::from_secs(5)).await?;
    match result["authorized"].as_bool() {
        Some(true) => Ok(()),
        Some(false) => Err(Problem::new("FORBIDDEN", "当前账户无此操作权限")),
        None => Err(Problem::new("DEPENDENCY_UNAVAILABLE", "用户授权响应不可读")),
    }
}
