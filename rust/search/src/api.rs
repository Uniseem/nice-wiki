//! HTTP 入口：路由表与 openapi.json 一致。每个业务请求先验服务断言得到操作上下文，再核调用方；
//! 管理接口另经 identity 核对后台用户的 knowledge.manage 权限。

use std::sync::Arc;

use axum::body::{to_bytes, Body};
use axum::extract::{Request, State as AxumState};
use axum::http::{HeaderValue, StatusCode};
use axum::response::Response;
use axum::Router;
use percent_encoding::percent_decode_str;
use serde::de::DeserializeOwned;
use serde_json::Value;

use nw_platform::inbound::authorize;
use nw_platform::platform::Problem;
use nw_platform::upstream::Operation;

use crate::State;

const BODY_LIMIT: usize = 16 << 20;

pub fn router(state: Arc<State>) -> Router {
    Router::new().fallback(handle).with_state(state)
}

pub struct Ctx {
    pub state: Arc<State>,
    pub op: Operation,
    pub body: Vec<u8>,
    pub query: String,
}

impl Ctx {
    pub fn input<T: DeserializeOwned>(&self) -> Result<T, Problem> {
        serde_json::from_slice(&self.body).map_err(|e| Problem::new("VALIDATION", format!("请求正文不合法：{e}")))
    }

    pub fn param(&self, name: &str) -> Option<String> {
        self.query.split('&').filter_map(|pair| pair.split_once('=')).find(|(k, _)| *k == name).map(|(_, v)| percent_decode_str(v).decode_utf8_lossy().into_owned())
    }
}

fn json(status: StatusCode, value: &Value) -> Response {
    let mut response = Response::new(Body::from(value.to_string()));
    *response.status_mut() = status;
    response.headers_mut().insert("content-type", HeaderValue::from_static("application/json"));
    response
}

fn problem(problem: &Problem, request_id: &str) -> Response {
    let mut response = Response::new(Body::from(problem.body(request_id)));
    *response.status_mut() = StatusCode::from_u16(problem.status()).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR);
    response.headers_mut().insert("content-type", HeaderValue::from_static("application/problem+json"));
    response
}

async fn handle(AxumState(state): AxumState<Arc<State>>, request: Request) -> Response {
    let (parts, body) = request.into_parts();
    let path = percent_decode_str(parts.uri.path()).decode_utf8_lossy().into_owned();
    let request_id = parts.headers.get("x-request-id").and_then(|v| v.to_str().ok()).unwrap_or_default().to_owned();
    match (parts.method.as_str(), path.as_str()) {
        ("GET", "/healthz") => return json(StatusCode::OK, &serde_json::json!({"module": crate::MODULE, "version": env!("CARGO_PKG_VERSION"), "status": "ready", "capabilities": []})),
        ("GET", "/readyz") => {
            let epoch = state.up.data_epoch().await.ok();
            let health = serde_json::json!({"module": crate::MODULE, "version": env!("CARGO_PKG_VERSION"), "status": if epoch.is_some() { "ready" } else { "degraded" }, "capabilities": []});
            return json(StatusCode::OK, &serde_json::json!({"health": health, "activeDataEpoch": epoch, "runtimeRevision": 1, "publicRequestsAllowed": true, "initialized": true, "journalRecovered": true}));
        }
        _ => {}
    }
    let body = match to_bytes(body, BODY_LIMIT).await {
        Ok(body) => body,
        Err(_) => return problem(&Problem::new("VALIDATION", "请求正文超过上限"), &request_id),
    };
    let epoch = match state.up.data_epoch().await {
        Ok(epoch) => epoch,
        Err(p) => return problem(&p, &request_id),
    };
    let op = match state.verifier.operation(&parts.method, &path, &parts.headers, &body, &epoch) {
        Ok(op) => op,
        Err(p) => return problem(&p, &request_id),
    };
    let request_id = op.request_id.clone();
    let proof = parts.headers.get("x-portal-proof").and_then(|v| v.to_str().ok()).unwrap_or_default().to_owned();
    let ctx = Ctx { state, op, body: body.to_vec(), query: parts.uri.query().unwrap_or_default().to_owned() };
    match dispatch(ctx, parts.method.as_str(), &path, &proof).await {
        Ok((status, value)) => json(status, &value),
        Err(p) => problem(&p, &request_id),
    }
}

type Reply = Result<(StatusCode, Value), Problem>;

fn ok(value: Value) -> Reply {
    Ok((StatusCode::OK, value))
}

fn segments(path: &str) -> Vec<&str> {
    path.trim_start_matches('/').split('/').collect()
}

async fn dispatch(ctx: Ctx, method: &str, path: &str, proof: &str) -> Reply {
    let parts = segments(path);
    // (操作、允许的调用方)；portal 调的都是后台管理接口。
    let (operation, callers): (&str, &[&str]) = match (method, parts.as_slice()) {
        ("PUT", ["v1", "index-config"]) => ("putIndexConfig", &["portal"]),
        ("GET", ["v1", "index-config"]) => ("readIndexConfig", &["portal"]),
        ("GET", ["v1", "index-options"]) => ("readIndexOptions", &["portal"]),
        ("POST", ["v1", "builds"]) => ("createBuild", &["portal"]),
        ("GET", ["v1", "builds"]) => ("listBuilds", &["portal"]),
        ("GET", ["v1", "builds", _]) => ("getBuild", &["portal"]),
        ("POST", ["v1", "builds", _, "cancel"]) => ("cancelBuild", &["portal"]),
        ("POST", ["v1", "site-overview:refresh"]) => ("refreshSiteOverview", &["portal"]),
        ("POST", ["v1", "retrievals"]) => ("searchKnowledge", &["qa"]),
        ("POST", ["v1", "sections:read"]) => ("readSections", &["qa"]),
        ("POST", ["v1", "query-embeddings"]) => ("createQueryEmbedding", &["qa"]),
        ("POST", ["v1", "document-index:read"]) => ("readDocumentIndex", &["qa"]),
        ("POST", ["v1", "document-chunks:read"]) => ("readDocumentChunks", &["qa"]),
        ("POST", ["v1", "site-overview:read"]) => ("readSiteOverview", &["qa"]),
        _ => return Err(Problem::new("NOT_FOUND", "接口不存在")),
    };
    if !callers.contains(&ctx.op.caller.as_str()) {
        return Err(Problem::new("FORBIDDEN", "调用方无权使用此接口"));
    }
    if callers == ["portal"] {
        authorize(&ctx.state.up, &ctx.op, proof, "knowledge.manage", &format!("knowledge:{operation}")).await?;
    }
    match operation {
        "putIndexConfig" => ok(crate::admin::put_config(&ctx).await?),
        "readIndexConfig" => ok(crate::admin::read_config(&ctx).await?),
        "readIndexOptions" => ok(crate::admin::options(&ctx).await?),
        "createBuild" => Ok((StatusCode::ACCEPTED, crate::admin::create_build(&ctx).await?)),
        "listBuilds" => ok(crate::admin::list_builds(&ctx).await?),
        "getBuild" => ok(crate::admin::get_build(&ctx, parts[2]).await?),
        "cancelBuild" => ok(crate::admin::cancel_build(&ctx, parts[2]).await?),
        "refreshSiteOverview" => Ok((StatusCode::ACCEPTED, crate::overview::refresh(&ctx).await?)),
        "searchKnowledge" => ok(crate::retrieve::search(&ctx).await?),
        "readSections" => ok(crate::retrieve::read_sections(&ctx).await?),
        "createQueryEmbedding" => ok(crate::retrieve::query_embedding(&ctx).await?),
        "readDocumentChunks" => ok(crate::retrieve::read_document_chunks(&ctx).await?),
        "readDocumentIndex" => ok(crate::docindex::read(&ctx).await?),
        "readSiteOverview" => ok(crate::overview::read(&ctx).await?),
        _ => unreachable!("上面的路由表只给出这些操作"),
    }
}
