use std::collections::HashMap;
use std::sync::Arc;

use axum::extract::{DefaultBodyLimit, Path, State as AxumState};
use axum::http::StatusCode;
use axum::middleware;
use axum::routing::{delete, get, post, put};
use axum::{Json, Router};
use serde::{Deserialize, Serialize};

use crate::lexical::Entry;
use crate::vector::VectorEntry;
use crate::State;

type Shared = AxumState<Arc<State>>;
type ApiError = (StatusCode, Json<serde_json::Value>);

fn fail(status: StatusCode, message: &str) -> ApiError {
    (status, Json(serde_json::json!({ "error": message })))
}

fn internal(error: impl std::fmt::Display) -> ApiError {
    eprintln!("内部错误：{error}");
    fail(StatusCode::INTERNAL_SERVER_ERROR, "内部错误")
}

/// 站点、文档、空间标识都是系统生成的不透明 ID；这里只放行安全字符，避免拼进键里出歧义。
fn check(value: &str, what: &str) -> Result<(), ApiError> {
    let ok = !value.is_empty() && value.len() <= 128 && value.bytes().all(|b| b.is_ascii_alphanumeric() || matches!(b, b'.' | b'_' | b':' | b'-'));
    if ok {
        Ok(())
    } else {
        Err(fail(StatusCode::BAD_REQUEST, &format!("{what}标识不合法")))
    }
}

pub fn router(state: Arc<State>) -> Router {
    Router::new()
        .route("/healthz", get(|| async { "ok" }))
        .route("/v1/stats", get(stats))
        .route("/v1/sites/:site", delete(delete_site))
        .route("/v1/sites/:site/manifest", get(manifest))
        .route("/v1/sites/:site/documents/:document", put(replace_document).delete(delete_document))
        .route("/v1/sites/:site/search", post(search))
        .layer(DefaultBodyLimit::max(64 << 20))
        .layer(middleware::from_fn_with_state(state.clone(), crate::auth::require_signature))
        .with_state(state)
}

async fn stats(state: Shared) -> Json<serde_json::Value> {
    Json(serde_json::json!({ "lexicalChunks": state.lexical.doc_count() }))
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ChunkBody {
    chunk_id: String,
    terms: HashMap<String, u32>,
    #[serde(default)]
    vector: Vec<f32>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReplaceBody {
    space_id: String,
    build_id: String,
    chunks: Vec<ChunkBody>,
}

async fn replace_document(state: Shared, Path((site, document)): Path<(String, String)>, Json(body): Json<ReplaceBody>) -> Result<Json<serde_json::Value>, ApiError> {
    check(&site, "站点")?;
    check(&document, "文档")?;
    check(&body.space_id, "空间")?;
    check(&body.build_id, "构建")?;
    let count = body.chunks.len();
    let lexical_entries: Vec<Entry> = body
        .chunks
        .iter()
        .map(|chunk| Entry { chunk_id: chunk.chunk_id.clone(), build_id: body.build_id.clone(), terms: chunk.terms.iter().map(|(term, n)| (term.clone(), *n)).collect() })
        .collect();
    let vector_entries: Vec<VectorEntry> = body
        .chunks
        .iter()
        .filter(|chunk| !chunk.vector.is_empty())
        .map(|chunk| VectorEntry { chunk_id: chunk.chunk_id.clone(), build_id: body.build_id.clone(), vector: chunk.vector.clone() })
        .collect();
    state.vectors.replace_document(&site, &document, &body.space_id, &body.build_id, &vector_entries, count).await.map_err(internal)?;
    let shared = state.0.clone();
    let (s, d, sp) = (site.clone(), document.clone(), body.space_id.clone());
    tokio::task::spawn_blocking(move || shared.lexical.replace_document(&s, &d, &sp, &lexical_entries)).await.map_err(internal)?.map_err(internal)?;
    Ok(Json(serde_json::json!({ "chunks": count, "vectors": vector_entries.len() })))
}

async fn delete_document(state: Shared, Path((site, document)): Path<(String, String)>) -> Result<StatusCode, ApiError> {
    check(&site, "站点")?;
    check(&document, "文档")?;
    state.vectors.delete_document(&site, &document).await.map_err(internal)?;
    let shared = state.0.clone();
    tokio::task::spawn_blocking(move || shared.lexical.delete_document(&site, &document)).await.map_err(internal)?.map_err(internal)?;
    Ok(StatusCode::NO_CONTENT)
}

async fn delete_site(state: Shared, Path(site): Path<String>) -> Result<StatusCode, ApiError> {
    check(&site, "站点")?;
    state.vectors.delete_site(&site).await.map_err(internal)?;
    let shared = state.0.clone();
    tokio::task::spawn_blocking(move || shared.lexical.delete_site(&site)).await.map_err(internal)?.map_err(internal)?;
    Ok(StatusCode::NO_CONTENT)
}

async fn manifest(state: Shared, Path(site): Path<String>) -> Result<Json<serde_json::Value>, ApiError> {
    check(&site, "站点")?;
    let documents = state.vectors.manifest(&site).await.map_err(internal)?;
    Ok(Json(serde_json::json!({ "documents": documents })))
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct SearchBody {
    space_id: String,
    #[serde(default)]
    terms: Vec<String>,
    #[serde(default)]
    vector: Vec<f32>,
    #[serde(default)]
    documents: Vec<String>,
    limit: Option<usize>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct HitOut {
    chunk_id: String,
    build_id: String,
    document_id: String,
    score: f64,
}

async fn search(state: Shared, Path(site): Path<String>, Json(body): Json<SearchBody>) -> Result<Json<serde_json::Value>, ApiError> {
    check(&site, "站点")?;
    check(&body.space_id, "空间")?;
    for document in &body.documents {
        check(document, "文档")?;
    }
    let limit = body.limit.unwrap_or(80).clamp(1, 200);
    let shared = state.0.clone();
    let (s, sp, terms, documents) = (site.clone(), body.space_id.clone(), body.terms.clone(), body.documents.clone());
    let lexical = tokio::task::spawn_blocking(move || shared.lexical.search(&s, &sp, &terms, &documents, limit)).await.map_err(internal)?.map_err(internal)?;
    let vector = state.vectors.search(&site, &body.space_id, &body.vector, &body.documents, limit as i64).await.map_err(internal)?;
    Ok(Json(serde_json::json!({
        "lexical": lexical.into_iter().map(|h| HitOut { chunk_id: h.chunk_id, build_id: h.build_id, document_id: h.document_id, score: h.score as f64 }).collect::<Vec<_>>(),
        "vector": vector.into_iter().map(|h| HitOut { chunk_id: h.chunk_id, build_id: h.build_id, document_id: h.document_id, score: h.score }).collect::<Vec<_>>(),
    })))
}
