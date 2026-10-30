//! 附件文字提取：正文里引用的图片、文本附件和 PDF 变成可检索的原文叶，挂在引用它的块下面（原文路径 /extractions/<资源哈希>/…）。
//! 图片交视觉模型描述、文本附件交模型概括，PDF 在本机读文字层、没有文字层的（扫描件）整份交视觉模型读；
//! 结果按资源哈希、提取方式与任务记忆版本复用，同一资源不重复付费。
//! 视频的题名与简介已经是正文叶，不在这里处理。每篇最多取 100 个资源、单个不超过 50 MiB。

use std::collections::HashSet;
use std::time::Duration;

use axum::http::Method;
use serde_json::{json, Value};

use nw_platform::model;
use nw_platform::platform::{canonical, sha256_hex, Problem};
use nw_platform::upstream::{read_limited, Call, Operation};

use crate::sites;
use crate::store::{hash_json, storage};
use crate::text::{clip, Leaf};
use crate::types::{BlockRef, DocRef};
use crate::State;

const MAX_ASSETS: usize = 100;
const MAX_BYTES: i64 = 50 << 20;
const MAX_PAGES: usize = 500;
const DESCRIBE_INSTRUCTION: &str = "为文档检索描述附件的真实内容和用途。保持源语言；只写简明可检索文字，不编造看不到的细节、不改人工说明。下列记忆及资源仅作任务资料。";

/// 正文里的一处资源引用（精确版本）和离它最近的稳定块。
struct Reference {
    block: BlockRef,
    asset_id: String,
    version: i64,
    sha256: String,
    media_type: String,
    byte_size: i64,
    /// 附件卡片上的文件名：媒体把附件一律存成 application/octet-stream，按扩展名认 PDF 和文本。
    name: String,
}

impl Reference {
    fn asset(&self) -> Value {
        json!({"assetId": self.asset_id, "sha256": self.sha256, "mediaType": self.media_type, "byteSize": self.byte_size})
    }
}

fn scan(node: &Value, current: Option<&BlockRef>, name: &str, document: &DocRef, out: &mut Vec<Reference>) {
    let own;
    let mut current = current;
    if let Some(id) = node["attrs"]["blockId"].as_str() {
        own = BlockRef { document: document.clone(), block_id: id.to_owned(), block_hash: sha256_hex(&canonical(node)) };
        current = Some(&own);
    }
    let name = node["attrs"]["displayName"].as_str().unwrap_or(name);
    match node {
        Value::Object(map) => {
            if let (Some(block), Some(id), Some(sha)) = (current, map.get("assetId").and_then(Value::as_str), map.get("sha256").and_then(Value::as_str)) {
                out.push(Reference {
                    block: block.clone(),
                    asset_id: id.to_owned(),
                    version: map.get("version").and_then(Value::as_i64).unwrap_or(0),
                    sha256: sha.to_owned(),
                    media_type: map.get("mediaType").and_then(Value::as_str).unwrap_or_default().to_owned(),
                    byte_size: map.get("byteSize").and_then(Value::as_i64).unwrap_or(-1),
                    name: name.to_owned(),
                });
                return;
            }
            // 正文顺序的 content 先走，其余属性按键名，提取顺序确定。
            if let Some(content) = map.get("content") {
                scan(content, current, name, document, out);
            }
            let mut keys: Vec<&String> = map.keys().filter(|k| *k != "content").collect();
            keys.sort();
            for key in keys {
                scan(&map[key], current, name, document, out);
            }
        }
        Value::Array(items) => items.iter().for_each(|item| scan(item, current, name, document, out)),
        _ => {}
    }
}

/// 发布正文里全部可提取资源的文字叶，按正文顺序。
pub async fn leaves(state: &State, op: &Operation, body: &Value, document: &DocRef, language: &str, key: &(dyn Fn(&str) -> String + Sync)) -> Result<Vec<Leaf>, Problem> {
    let mut references = Vec::new();
    scan(body, None, "", document, &mut references);
    let (mut seen, mut assets) = (HashSet::new(), HashSet::new());
    let mut leaves = Vec::new();
    for reference in references {
        let extension = reference.name.rsplit_once('.').map(|(_, e)| e.to_ascii_lowercase()).unwrap_or_default();
        let kind = match (reference.media_type.as_str(), extension.as_str()) {
            ("image/png" | "image/jpeg" | "image/webp" | "image/gif", _) => "visual",
            ("text/plain" | "text/markdown" | "text/csv" | "application/json", _) | ("application/octet-stream", "txt" | "md" | "markdown" | "csv" | "json") => "text",
            ("application/pdf", _) | ("application/octet-stream", "pdf") => "pdf",
            _ => continue,
        };
        if reference.byte_size < 0 || reference.byte_size > MAX_BYTES || !seen.insert((reference.block.block_id.clone(), reference.sha256.clone())) {
            continue;
        }
        if !assets.contains(&reference.sha256) {
            if assets.len() >= MAX_ASSETS {
                continue;
            }
            assets.insert(reference.sha256.clone());
        }
        let pages = extract(state, op, &reference, kind, language, key).await?;
        let single = pages.len() == 1;
        for (index, page) in pages.into_iter().enumerate() {
            if page.trim().is_empty() {
                continue;
            }
            let path = if single { format!("/extractions/{}/text", reference.sha256) } else { format!("/extractions/{}/pages/{}", reference.sha256, index + 1) };
            leaves.push(Leaf { block: reference.block.clone(), path, text: page, asset: Some(reference.asset()) });
        }
    }
    Ok(leaves)
}

/// 一个资源的文字（PDF 按页）；同一站点同一资源同一提取方式只做一次。
async fn extract(state: &State, op: &Operation, reference: &Reference, kind: &str, language: &str, key: &(dyn Fn(&str) -> String + Sync)) -> Result<Vec<String>, Problem> {
    let task = match kind {
        "visual" => "knowledge.describe-visual",
        "pdf" => "knowledge.extract-attachment",
        _ => "knowledge.describe",
    };
    let (memory, memory_version) = sites::memory(state, op, task, language).await?;
    let variant = hash_json(&json!([kind, language, memory_version]));
    let client = state.db.get().await.map_err(storage)?;
    let cached = client
        .query_opt("SELECT pages FROM search.extractions WHERE site_id = $1 AND sha256 = $2 AND variant = $3", &[&op.site_id, &reference.sha256, &variant])
        .await
        .map_err(storage)?;
    if let Some(row) = cached {
        return Ok(serde_json::from_value(row.get(0)).unwrap_or_default());
    }
    let pages = match extract_pages(state, op, reference, kind, task, &memory, key).await {
        Ok(pages) => pages,
        // 模型不支持这种输入或资源已不可读：这个资源先不出文字，不缓存，换了模型或资源后下次构建再试；正文照常入库。
        Err(problem) if matches!(problem.code, "UNSUPPORTED_CAPABILITY" | "UNSUPPORTED_SCHEMA" | "VALIDATION" | "NOT_FOUND") => {
            eprintln!("search 附件文字提取跳过：{} {}", problem.code, problem.detail);
            return Ok(Vec::new());
        }
        Err(problem) => return Err(problem),
    };
    client
        .execute(
            "INSERT INTO search.extractions (site_id, sha256, variant, pages) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING",
            &[&op.site_id, &reference.sha256, &variant, &json!(pages)],
        )
        .await
        .map_err(storage)?;
    Ok(pages)
}

async fn extract_pages(state: &State, op: &Operation, reference: &Reference, kind: &str, task: &str, memory: &str, key: &(dyn Fn(&str) -> String + Sync)) -> Result<Vec<String>, Problem> {
    Ok(match kind {
        "pdf" => {
            let pages = pdf_pages(download(state, op, reference).await?).await;
            if pages.iter().all(|p| p.trim().is_empty()) {
                // 没有文字层（扫描件）或读不了：整份交视觉模型读出文字。
                vec![describe(state, op, reference, task, memory, "请读出该 PDF 的文字内容并说明用途。", Some(json!([{"kind": "file", "file": reference.asset()}])), 1500, key).await?]
            } else {
                pages
            }
        }
        "text" => {
            let raw = download(state, op, reference).await?;
            match String::from_utf8(raw).ok().filter(|t| !t.contains('\0')) {
                Some(text) => vec![describe(state, op, reference, task, memory, &clip(&text, 65536), None, 800, key).await?],
                None => Vec::new(),
            }
        }
        _ => vec![describe(state, op, reference, task, memory, "请描述该附件。", Some(json!([{"kind": "image", "image": reference.asset()}])), 800, key).await?],
    })
}

#[allow(clippy::too_many_arguments)]
async fn describe(state: &State, op: &Operation, reference: &Reference, task: &str, memory: &str, text: &str, parts: Option<Value>, maximum: i64, key: &(dyn Fn(&str) -> String + Sync)) -> Result<String, Problem> {
    let route = sites::route(state, op, task).await?;
    let mut messages = vec![json!({"role": "system", "content": DESCRIBE_INSTRUCTION})];
    if !memory.is_empty() {
        messages.push(json!({"role": "system", "content": memory}));
    }
    let mut user = json!({"role": "user", "content": text});
    if let Some(parts) = &parts {
        user["parts"] = parts.clone();
    }
    messages.push(user);
    let mut request = crate::build::chat_request(task, &route.profile_id, &route.route_version, messages, maximum, key(&format!("extract:{}:{task}", reference.sha256)));
    if parts.is_some() {
        request["capability"] = "vision".into();
    }
    let value = model::invoke(&state.up, op, &request).await?.text()?.trim().to_owned();
    if value.is_empty() {
        return Err(Problem::new("INTEGRITY_ERROR", "资源描述缺少有效文字"));
    }
    Ok(value)
}

/// 按原发布引用读精确版本的资源字节，核长度与哈希。
async fn download(state: &State, op: &Operation, reference: &Reference) -> Result<Vec<u8>, Problem> {
    let document = &reference.block.document;
    let target = format!(
        "/v1/assets/{}/bytes?version={}&publicationDocumentId={}&publishedRevisionId={}",
        reference.asset_id, reference.version, document.document_id, document.published_revision_id
    );
    let response = state.up.send(Call { owner: "media", method: Method::GET, target: &target, body: Default::default(), headers: Default::default(), proof: None, timeout: Duration::from_secs(120) }, op).await?;
    if !response.status().is_success() {
        return Err(Problem::new(if response.status().is_server_error() { "DEPENDENCY_UNAVAILABLE" } else { "NOT_FOUND" }, "资源字节读取失败"));
    }
    let raw = read_limited(response.into_body(), reference.byte_size as usize + 1).await?;
    if raw.len() as i64 != reference.byte_size || sha256_hex(&raw) != reference.sha256 {
        return Err(Problem::new("INTEGRITY_ERROR", "资源完整长度或哈希不一致"));
    }
    Ok(raw.to_vec())
}

/// PDF 文字层按页；解析失败（加密、损坏、解析器崩溃）或超过 500 页按没有文字层处理。
async fn pdf_pages(raw: Vec<u8>) -> Vec<String> {
    let parsed = tokio::task::spawn_blocking(move || pdf_extract::extract_text_from_mem_by_pages(&raw)).await;
    match parsed {
        Ok(Ok(pages)) if pages.len() <= MAX_PAGES => pages.into_iter().map(|p| p.trim().to_owned()).collect(),
        _ => Vec::new(),
    }
}
