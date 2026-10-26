use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use axum::body::{to_bytes, Body};
use axum::extract::{Request, State};
use axum::http::StatusCode;
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};
use hmac::{Hmac, Mac};
use sha2::{Digest, Sha256};
use subtle::ConstantTimeEq;

type HmacSha256 = Hmac<Sha256>;

const MAX_BODY: usize = 64 << 20;
const MAX_SKEW_SECONDS: u64 = 120;

/// 调用方签名：HMAC-SHA256(密钥, "{时间戳}\n{方法}\n{路径与查询}\n{请求体SHA256}")，十六进制放在
/// `X-Nw-Signature`，时间戳（秒）放在 `X-Nw-Timestamp`。时间差超过 120 秒的请求拒绝，防回放。
pub fn sign(secret: &[u8], timestamp: u64, method: &str, target: &str, body: &[u8]) -> String {
    let digest = hex::encode(Sha256::digest(body));
    let mut mac = HmacSha256::new_from_slice(secret).expect("HMAC 接受任意长度密钥");
    mac.update(format!("{timestamp}\n{method}\n{target}\n{digest}").as_bytes());
    hex::encode(mac.finalize().into_bytes())
}

pub async fn require_signature(State(state): State<Arc<crate::State>>, request: Request, next: Next) -> Response {
    if request.uri().path() == "/healthz" {
        return next.run(request).await;
    }
    let (parts, body) = request.into_parts();
    let timestamp = parts.headers.get("x-nw-timestamp").and_then(|v| v.to_str().ok()).and_then(|v| v.parse::<u64>().ok());
    let supplied = parts.headers.get("x-nw-signature").and_then(|v| v.to_str().ok()).map(str::to_owned);
    let (Some(timestamp), Some(supplied)) = (timestamp, supplied) else {
        return (StatusCode::UNAUTHORIZED, "缺少签名").into_response();
    };
    let now = SystemTime::now().duration_since(UNIX_EPOCH).map(|d| d.as_secs()).unwrap_or(0);
    if now.abs_diff(timestamp) > MAX_SKEW_SECONDS {
        return (StatusCode::UNAUTHORIZED, "签名已过期").into_response();
    }
    let Ok(bytes) = to_bytes(body, MAX_BODY).await else {
        return (StatusCode::PAYLOAD_TOO_LARGE, "请求体过大").into_response();
    };
    let target = parts.uri.path_and_query().map(|p| p.as_str().to_owned()).unwrap_or_default();
    let expected = sign(&state.secret, timestamp, parts.method.as_str(), &target, &bytes);
    if expected.as_bytes().ct_eq(supplied.as_bytes()).unwrap_u8() != 1 {
        return (StatusCode::UNAUTHORIZED, "签名不符").into_response();
    }
    next.run(Request::from_parts(parts, Body::from(bytes))).await
}
