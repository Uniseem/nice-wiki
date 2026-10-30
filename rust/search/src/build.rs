//! 索引构建：发布事件或后台手动创建时排队（search.builds），进程内工作循环按到期时间领取（15 分钟租约），
//! 依次取发布正文 → 原文叶规范化与真实分词 → 切块 → 背景短句 → 文档卡片 → 嵌入 → 再核发布 → 生效。
//! 背景短句与向量按来源哈希缓存，重试与重发布不重复付费。生效时先整篇替换全文索引，再在一个事务里换向量、分块与生效指针。
//! 失败按退避重排，5 次后记为失败；每个阶段边界都核构建没有被取消或替代。

use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use deadpool_postgres::{GenericClient, Transaction};
use axum::http::Method;
use serde_json::{json, Value};

use nw_platform::model;
use nw_platform::platform::{canonical, sha256_hex, Problem};
use nw_platform::queue::{Consumer, Event};
use nw_platform::upstream::Operation;

use crate::lexical::Entry;
use crate::sites;
use crate::store::{hash_json, purge_site, storage};
use crate::text::{self, Source};
use crate::types::{Chunk, DocRef, IndexBuild, IndexConfig, Space};
use crate::{vector, State};

const TERMINAL: &str = "('active', 'cancelled', 'failed', 'superseded')";
const MAX_ATTEMPTS: i32 = 5;

pub const BUILD_COLUMNS: &str = "build_id, document, config->'space', state, generation, completed_chunks, total_chunks, failure_code";

pub fn from_row(row: &tokio_postgres::Row) -> IndexBuild {
    IndexBuild {
        build_id: row.get(0),
        document: serde_json::from_value(row.get(1)).expect("构建记录的发布引用由本服务写入"),
        space: serde_json::from_value(row.get(2)).expect("构建记录的空间由本服务写入"),
        state: row.get(3),
        generation: row.get(4),
        completed_chunks: row.get::<_, i32>(5) as i64,
        total_chunks: row.get::<_, i32>(6) as i64,
        failure_code: row.get(7),
    }
}

pub async fn read(client: &impl GenericClient, site: &str, id: &str) -> Result<IndexBuild, Problem> {
    let row = client.query_opt(&format!("SELECT {BUILD_COLUMNS} FROM search.builds WHERE site_id = $1 AND build_id = $2"), &[&site, &id]).await.map_err(storage)?;
    row.as_ref().map(from_row).ok_or_else(|| Problem::new("NOT_FOUND", "构建不存在"))
}

/// 排队一次构建：不强制时复用同一发布修订与向量空间的排队中、进行中或已生效构建；同一文档其他未结束的构建被替代。
pub async fn queue_build(client: &impl GenericClient, site: &str, config: &IndexConfig, document: &DocRef, force: bool) -> Result<IndexBuild, Problem> {
    let doc = serde_json::to_value(document).expect("发布引用可序列化");
    if !force {
        let row = client
            .query_opt(
                &format!(
                    "SELECT {BUILD_COLUMNS} FROM search.builds WHERE site_id = $1 AND document_id = $2 AND document = $3 AND config->'space'->>'spaceId' = $4
                     AND state NOT IN ('failed', 'cancelled', 'superseded') ORDER BY generation DESC LIMIT 1"
                ),
                &[&site, &document.document_id, &doc, &config.space.space_id],
            )
            .await
            .map_err(storage)?;
        if let Some(row) = row {
            return Ok(from_row(&row));
        }
    }
    client
        .execute(&format!("UPDATE search.builds SET state = 'superseded', updated_at = now() WHERE site_id = $1 AND document_id = $2 AND state NOT IN {TERMINAL}"), &[&site, &document.document_id])
        .await
        .map_err(storage)?;
    let id = nw_platform::platform::new_id();
    let stored = serde_json::to_value(config).expect("配置可序列化");
    let row = client
        .query_one(
            &format!(
                "INSERT INTO search.builds (build_id, site_id, document_id, document, config, state, generation)
                 SELECT $1, $2, $3, $4, $5, 'queued', COALESCE(MAX(generation), 0) + 1 FROM search.builds WHERE site_id = $2 AND document_id = $3
                 RETURNING {BUILD_COLUMNS}"
            ),
            &[&id, &site, &document.document_id, &doc, &stored],
        )
        .await
        .map_err(storage)?;
    Ok(from_row(&row))
}

/// 文档可见性水位只升不降。
pub async fn raise_watermark(client: &impl GenericClient, site: &str, document: &str, visibility: i64, withdrawn: bool) -> Result<(), Problem> {
    client
        .execute(
            "INSERT INTO search.watermarks (site_id, document_id, visibility_revision, withdrawn) VALUES ($1, $2, $3, $4)
             ON CONFLICT (site_id, document_id) DO UPDATE SET visibility_revision = EXCLUDED.visibility_revision, withdrawn = EXCLUDED.withdrawn
             WHERE search.watermarks.visibility_revision <= EXCLUDED.visibility_revision",
            &[&site, &document, &visibility, &withdrawn],
        )
        .await
        .map_err(storage)?;
    Ok(())
}

fn failure(problem: Problem) -> anyhow::Error {
    anyhow::anyhow!("{}：{}", problem.code, problem.detail)
}

// ---------- 事件 ----------

const EVENTS: [&str; 4] = ["content.publication.changed@1", "content.publication.withdrawn@1", "identity.site.changed@1", "identity.site.purged@1"];

pub async fn consume(state: Arc<State>) {
    let consumer = Consumer::new(state.db.clone(), crate::MODULE);
    consumer
        .run(&EVENTS, |event| {
            let state = state.clone();
            async move { handle(&state, event).await.map_err(failure) }
        })
        .await;
}

async fn handle(state: &State, event: Event) -> Result<(), Problem> {
    let payload: Value = serde_json::from_slice(&event.payload).map_err(|_| Problem::new("VALIDATION", "事件载荷不可读"))?;
    let site = payload["siteId"].as_str().unwrap_or(&event.site_id).to_owned();
    match event.event_type.as_str() {
        "content.publication.changed" => {
            let document: DocRef = serde_json::from_value(payload["reference"].clone()).map_err(|_| Problem::new("VALIDATION", "发布事件引用不可读"))?;
            let visibility = payload["visibilityRevision"].as_i64().unwrap_or(event.revision);
            let op = Operation::background(&site, &document.data_epoch, 30);
            let mut client = state.db.get().await.map_err(storage)?;
            let tx = client.transaction().await.map_err(storage)?;
            let known = tx.query_opt("SELECT visibility_revision FROM search.watermarks WHERE site_id = $1 AND document_id = $2", &[&site, &document.document_id]).await.map_err(storage)?;
            if known.is_some_and(|row| row.get::<_, i64>(0) >= visibility) {
                return Ok(());
            }
            raise_watermark(&tx, &site, &document.document_id, visibility, false).await?;
            let queued = match sites::config(state, &op).await? {
                Some(config) if config.enabled => {
                    queue_build(&tx, &site, &config, &document, false).await?;
                    true
                }
                _ => false,
            };
            tx.commit().await.map_err(storage)?;
            if queued {
                state.wake.notify_one();
            }
            Ok(())
        }
        "content.publication.withdrawn" => {
            let document = payload["documentId"].as_str().unwrap_or(&event.subject_id).to_owned();
            let visibility = payload["visibilityRevision"].as_i64().unwrap_or(event.revision);
            withdraw(state, &site, &document, visibility).await
        }
        "identity.site.changed" => {
            let status = payload["status"].as_str().unwrap_or_default();
            let revision = payload["revision"].as_i64().unwrap_or(event.revision);
            let client = state.db.get().await.map_err(storage)?;
            client
                .execute(
                    "INSERT INTO search.sites (site_id, status, revision) VALUES ($1, $2, $3)
                     ON CONFLICT (site_id) DO UPDATE SET status = EXCLUDED.status, revision = EXCLUDED.revision WHERE search.sites.revision < EXCLUDED.revision",
                    &[&site, &status, &revision],
                )
                .await
                .map_err(storage)?;
            Ok(())
        }
        "identity.site.purged" => {
            tokio::task::block_in_place(|| state.lexical.delete_site(&site)).map_err(storage)?;
            let client = state.db.get().await.map_err(storage)?;
            vector::delete_site(&client, &site).await.map_err(storage)?;
            purge_site(&state.db, &site).await
        }
        _ => Ok(()),
    }
}

/// 撤回：文档退出检索（全文、向量、生效指针、卡片），未结束的构建被替代。
async fn withdraw(state: &State, site: &str, document: &str, visibility: i64) -> Result<(), Problem> {
    tokio::task::block_in_place(|| state.lexical.delete_document(site, document)).map_err(storage)?;
    let mut client = state.db.get().await.map_err(storage)?;
    let tx = client.transaction().await.map_err(storage)?;
    raise_watermark(&tx, site, document, visibility, true).await?;
    tx.execute(&format!("UPDATE search.builds SET state = 'superseded', updated_at = now() WHERE site_id = $1 AND document_id = $2 AND state NOT IN {TERMINAL}"), &[&site, &document])
        .await
        .map_err(storage)?;
    vector::delete_document(&tx, site, document).await.map_err(storage)?;
    for table in ["documents", "chunks", "cards"] {
        tx.execute(&format!("DELETE FROM search.{table} WHERE site_id = $1 AND document_id = $2"), &[&site, &document]).await.map_err(storage)?;
    }
    tx.commit().await.map_err(storage)
}

// ---------- 工作循环 ----------

struct Job {
    build_id: String,
    site: String,
    document: DocRef,
    config: IndexConfig,
    attempts: i32,
}

pub async fn work(state: Arc<State>) {
    loop {
        match claim(&state).await {
            Ok(Some(job)) => {
                let (id, attempts) = (job.build_id.clone(), job.attempts);
                let outcome = execute(&state, &job).await;
                if let Err(problem) = outcome {
                    if let Err(error) = settle_failure(&state, &id, attempts, problem).await {
                        eprintln!("search 构建失败状态保存失败：{}", error.detail);
                    }
                }
            }
            Ok(None) => {
                let _ = tokio::time::timeout(Duration::from_secs(5), state.wake.notified()).await;
            }
            Err(problem) => {
                eprintln!("search 领取构建失败：{}", problem.detail);
                tokio::time::sleep(Duration::from_secs(2)).await;
            }
        }
    }
}

async fn claim(state: &State) -> Result<Option<Job>, Problem> {
    let client = state.db.get().await.map_err(storage)?;
    let row = client
        .query_opt(
            &format!(
                "UPDATE search.builds b SET attempts = b.attempts + 1, visible_at = now() + interval '15 minutes', updated_at = now()
                 FROM (SELECT build_id FROM search.builds WHERE state NOT IN {TERMINAL} AND visible_at <= now() ORDER BY visible_at LIMIT 1 FOR UPDATE SKIP LOCKED) due
                 WHERE b.build_id = due.build_id RETURNING b.build_id, b.site_id, b.document, b.config, b.attempts"
            ),
            &[],
        )
        .await
        .map_err(storage)?;
    let Some(row) = row else { return Ok(None) };
    let job = Job {
        build_id: row.get(0),
        site: row.get(1),
        document: serde_json::from_value(row.get(2)).map_err(|_| Problem::new("INTEGRITY_ERROR", "构建记录损坏"))?,
        config: serde_json::from_value(row.get(3)).map_err(|_| Problem::new("INTEGRITY_ERROR", "构建记录损坏"))?,
        attempts: row.get(4),
    };
    if !crate::store::site_active(&state.db, &job.site).await? {
        // 站点下架期间不做付费处理，一小时后再看。
        client.execute("UPDATE search.builds SET attempts = attempts - 1, visible_at = now() + interval '1 hour' WHERE build_id = $1", &[&job.build_id]).await.map_err(storage)?;
        return Ok(None);
    }
    Ok(Some(job))
}

async fn settle_failure(state: &State, id: &str, attempts: i32, problem: Problem) -> Result<(), Problem> {
    let client = state.db.get().await.map_err(storage)?;
    if problem.code == "CANCELLED" {
        return Ok(());
    }
    if matches!(problem.code, "NOT_FOUND" | "STALE_RESULT") {
        // 排队或读取期间文档被撤回、删除或换了新版本：这一版无内容可索引，不算失败。
        client.execute(&format!("UPDATE search.builds SET state = 'superseded', updated_at = now() WHERE build_id = $1 AND state NOT IN {TERMINAL}"), &[&id]).await.map_err(storage)?;
        return Ok(());
    }
    eprintln!("search 构建 {id} 第 {attempts} 次失败：{} {}", problem.code, problem.detail);
    if attempts >= MAX_ATTEMPTS || !(problem.retryable || problem.code == "INTERNAL") {
        client
            .execute(&format!("UPDATE search.builds SET state = 'failed', failure_code = $2, updated_at = now() WHERE build_id = $1 AND state NOT IN {TERMINAL}"), &[&id, &problem.code])
            .await
            .map_err(storage)?;
    } else {
        let wait = 30.0 * 2f64.powi(attempts - 1);
        client
            .execute("UPDATE search.builds SET visible_at = now() + make_interval(secs => $2), updated_at = now() WHERE build_id = $1", &[&id, &wait])
            .await
            .map_err(storage)?;
    }
    Ok(())
}

/// 进入下一阶段；构建已被取消或替代时停下。
async fn stage(state: &State, job: &Job, name: &str, total: Option<i32>) -> Result<(), Problem> {
    let client = state.db.get().await.map_err(storage)?;
    let changed = client
        .execute(
            &format!("UPDATE search.builds SET state = $2, total_chunks = COALESCE($3, total_chunks), updated_at = now() WHERE build_id = $1 AND state NOT IN {TERMINAL}"),
            &[&job.build_id, &name, &total],
        )
        .await
        .map_err(storage)?;
    if changed == 0 {
        return Err(Problem::new("CANCELLED", "构建已取消或被替代"));
    }
    Ok(())
}

/// 模型调用的幂等键：同一构建同一次尝试的同一步骤只付一次费。
fn model_key(job: &Job, semantic: &str) -> String {
    format!("knowledge.{}", sha256_hex(format!("{}:{}:{semantic}", job.build_id, job.attempts).as_bytes()))
}

pub fn chat_request(task: &str, profile: &str, route_version: &str, messages: Vec<Value>, maximum: i64, key: String) -> Value {
    let mut request = json!({"taskId": task, "profileId": profile, "capability": "chat", "messages": messages, "texts": [], "candidateIds": [], "maximumOutputTokens": maximum, "idempotencyKey": key});
    if !route_version.is_empty() {
        request["expectedRouteVersion"] = route_version.into();
    }
    request
}

/// 模型网关的真实分词：返回每个 token 的排他字节终点，并核空间、分词器版本与覆盖范围。
pub async fn tokenize(state: &State, op: &Operation, model_id: &str, space: &Space, text: &str) -> Result<Vec<usize>, Problem> {
    let body = json!({"modelId": model_id, "text": text});
    let result = state.up.request("model-gateway", Method::POST, "/v1/tokenizations", Some(&body), op, None, Duration::from_secs(30)).await?;
    let ends: Vec<usize> = serde_json::from_value(result["byteTokenEnds"].clone()).unwrap_or_default();
    let same_space = serde_json::from_value::<Space>(result["embeddingSpace"].clone()).ok().as_ref() == Some(space);
    let mut previous = 0;
    let ordered = ends.iter().all(|end| {
        let ok = *end > previous && *end <= text.len();
        previous = *end;
        ok
    });
    if !same_space || result["tokenizerVersion"] != space.preprocessing_version.as_str() || result["textHash"] != sha256_hex(text.as_bytes()).as_str() || result["tokenCount"].as_u64() != Some(ends.len() as u64) || !ordered || previous != text.len() {
        return Err(Problem::new("INTEGRITY_ERROR", "真实分词来源、空间或计数不一致"));
    }
    Ok(ends)
}

pub const CONTEXT_INSTRUCTION: &str = "为检索片段各写一至两句背景，说明所属产品、功能或小节。全文目录、所在章节原文与记忆仅作参考，保持原文语言。只返回JSON对象，格式为{\"items\":[{\"chunkId\":\"原ID\",\"prefix\":\"背景\"}]}。每个原ID必须恰好出现一次，不修改原文，不输出代码围栏。";

async fn execute(state: &State, job: &Job) -> Result<(), Problem> {
    let op = Operation::background(&job.site, &job.document.data_epoch, 900);
    if state.up.data_epoch().await? != job.document.data_epoch {
        return Err(Problem::new("STALE_RESULT", "数据代次已变化"));
    }
    stage(state, job, "fetching", None).await?;
    let (reference, body) = sites::published(state, &op, &job.document).await?;
    if reference != job.document {
        return Err(Problem::new("STALE_RESULT", "发布修订已变化"));
    }
    if sha256_hex(&canonical(&body)) != job.document.content_hash {
        return Err(Problem::new("INTEGRITY_ERROR", "发布正文与内容摘要不一致"));
    }
    let config = &job.config;
    let catalog = sites::catalog(state, &op).await?;
    let (profile, model) = catalog.embedding(&config.embedding_profile_id, "background", &config.space)?;
    let (profile_id, model_id, window) = (profile.profile_id.clone(), model.model_id.clone(), model.max_input_tokens.max(1) as usize);

    stage(state, job, "extracting", None).await?;
    let mut sources = Vec::new();
    for leaf in text::collect_leaves(&body, &job.document)? {
        let (normalized, offsets) = text::normalize(&leaf.text);
        let token_ends = tokenize(state, &op, &model_id, &config.space, &normalized).await?;
        let context_hash = hash_json(&json!([job.document, leaf.block, config.space.preprocessing_version]));
        sources.push(Source { leaf_block: leaf.block, path: leaf.path, source_end: leaf.text.len() as i64, normalized, offsets, token_ends, context_hash });
    }
    let mut chunks = text::split(&job.document, &sources, config.chunk_tokens as usize, config.overlap_tokens as usize)?;

    stage(state, job, "chunking", Some(chunks.len() as i32)).await?;
    enrich(state, &op, job, &body, &mut chunks).await?;
    for chunk in chunks.iter_mut() {
        let count = tokenize(state, &op, &model_id, &config.space, &chunk.retrieval_text()).await?.len();
        if count > window || chunk.retrieval_text().chars().count() > 4096 {
            return Err(Problem::new("BUDGET_EXCEEDED", "完整嵌入文本超过固定模型输入预算"));
        }
        chunk.embedding_token_count = Some(count as i64);
    }
    // 文档卡片只是问答选文档的辅助：失败时保留旧卡片，索引照常上线。
    if let Err(problem) = crate::docindex::refresh_card(state, &op, &model_key(job, "card"), &job.document, &chunks).await {
        eprintln!("search 文档卡片未更新：{} {}", problem.code, problem.detail);
    }

    stage(state, job, "embedding", None).await?;
    let vectors = embed(state, &op, job, &profile_id, window, &chunks).await?;

    stage(state, job, "validating", None).await?;
    let current = sites::validate_refs(state, &op, std::slice::from_ref(&job.document)).await?;
    if !current.contains_key(&job.document) {
        return Err(Problem::new("STALE_RESULT", "发布已撤回或换了新修订"));
    }
    activate(state, job, &chunks, &vectors).await
}

/// 背景短句：按章节分批，每批带全文目录和所在章节原文；命中缓存的片段不再调用模型。
async fn enrich(state: &State, op: &Operation, job: &Job, body: &Value, chunks: &mut [Chunk]) -> Result<(), Problem> {
    let (memory, memory_version) = sites::memory(state, op, "knowledge.context", &job.config.language).await?;
    let route = sites::route(state, op, "knowledge.context").await?;
    let version = hash_json(&json!([job.document.content_hash, memory_version]));
    let fixed = CONTEXT_INSTRUCTION.len() + memory.len();
    let batches = text::plan_context_batches(body, chunks, |batch| text::context_room(route.window, fixed, batch));
    let key_of = |chunk: &Chunk| hash_json(&json!([version, chunk.block.block_id, chunk.block.block_hash, chunk.source_path, chunk.byte_start, chunk.byte_end, chunk.chunk_hash]));
    let client = state.db.get().await.map_err(storage)?;
    for batch in batches {
        let keys: Vec<String> = batch.chunks.iter().map(|&i| key_of(&chunks[i])).collect();
        let rows = client.query("SELECT context_key, prefix FROM search.contexts WHERE site_id = $1 AND context_key = ANY($2)", &[&job.site, &keys]).await.map_err(storage)?;
        let mut prefixes: HashMap<String, String> = rows.iter().map(|r| (r.get(0), r.get(1))).collect();
        let missing: Vec<usize> = batch.chunks.iter().copied().filter(|&i| !prefixes.contains_key(&key_of(&chunks[i]))).collect();
        if !missing.is_empty() {
            let group: Vec<&Chunk> = missing.iter().map(|&i| &chunks[i]).collect();
            let mut messages = vec![json!({"role": "system", "content": CONTEXT_INSTRUCTION})];
            if !memory.is_empty() {
                messages.push(json!({"role": "system", "content": memory}));
            }
            messages.push(json!({"role": "user", "content": batch.context}));
            messages.push(json!({"role": "user", "content": text::batch_json(&group)}));
            let ids: Vec<&str> = group.iter().map(|c| c.chunk_id.as_str()).collect();
            let key = model_key(job, &format!("context:{}", hash_json(&json!(ids))));
            let request = chat_request("knowledge.context", &route.profile_id, &route.route_version, messages, 200 + 160 * group.len() as i64, key);
            let outcome = model::invoke(&state.up, op, &request).await?;
            let parsed = read_prefixes(outcome.text()?, &group)?;
            for (&i, chunk) in missing.iter().zip(&group) {
                let prefix = parsed[&chunk.chunk_id].clone();
                let key = key_of(&chunks[i]);
                client
                    .execute("INSERT INTO search.contexts (site_id, context_key, prefix) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", &[&job.site, &key, &prefix])
                    .await
                    .map_err(storage)?;
                prefixes.insert(key, prefix);
            }
        }
        for &i in &batch.chunks {
            let prefix = prefixes[&key_of(&chunks[i])].clone();
            chunks[i].context_prefix = Some(prefix);
            chunks[i].context_version_hash = Some(version.clone());
        }
    }
    Ok(())
}

/// 严格匹配原 ID 与完整输出：每个片段一至两句、单行。
fn read_prefixes(text: &str, chunks: &[&Chunk]) -> Result<HashMap<String, String>, Problem> {
    #[derive(serde::Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Item {
        #[serde(rename = "chunkId")]
        chunk_id: String,
        prefix: String,
    }
    #[derive(serde::Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Reply {
        items: Vec<Item>,
    }
    let bad = || Problem::new("INTEGRITY_ERROR", "背景模型输出结构、数量或短句不合法");
    let reply: Reply = serde_json::from_str(text.trim()).map_err(|_| bad())?;
    if reply.items.len() != chunks.len() {
        return Err(bad());
    }
    let mut result = HashMap::new();
    for item in reply.items {
        let known = chunks.iter().any(|c| c.chunk_id == item.chunk_id);
        if !known || result.contains_key(&item.chunk_id) || item.prefix.trim().is_empty() || item.prefix.len() > 4096 || item.prefix.contains(['\r', '\n', '\0']) {
            return Err(bad());
        }
        let (mut sentences, mut inside) = (0, false);
        for c in item.prefix.chars() {
            if "。！？.!?".contains(c) {
                if inside {
                    sentences += 1;
                    inside = false;
                }
            } else if c != ' ' && c != '\t' {
                inside = true;
            }
        }
        if inside {
            sentences += 1;
        }
        if !(1..=2).contains(&sentences) {
            return Err(bad());
        }
        result.insert(item.chunk_id, item.prefix);
    }
    Ok(result)
}

/// 嵌入：同一检索文字与向量空间的向量直接复用；其余每批最多 64 段、合计不超过模型输入窗口。
async fn embed(state: &State, op: &Operation, job: &Job, profile: &str, window: usize, chunks: &[Chunk]) -> Result<Vec<Vec<f32>>, Problem> {
    let space = &job.config.space;
    let hashes: Vec<String> = chunks.iter().map(|c| sha256_hex(c.retrieval_text().as_bytes())).collect();
    let client = state.db.get().await.map_err(storage)?;
    let rows = client
        .query("SELECT text_hash, vector FROM search.embeddings WHERE site_id = $1 AND space_id = $2 AND text_hash = ANY($3)", &[&job.site, &space.space_id, &hashes])
        .await
        .map_err(storage)?;
    let mut known: HashMap<String, Vec<f32>> = rows.iter().filter_map(|r| Some((r.get(0), serde_json::from_value(r.get(1)).ok()?))).collect();
    let mut seen = std::collections::HashSet::new();
    let pending: Vec<usize> = (0..chunks.len()).filter(|&i| !known.contains_key(&hashes[i]) && seen.insert(hashes[i].clone())).collect();
    let mut batches: Vec<Vec<usize>> = Vec::new();
    let mut tokens = 0;
    for i in pending {
        let count = chunks[i].embedding_token_count.unwrap_or_default() as usize;
        match batches.last_mut() {
            Some(batch) if batch.len() < 64 && tokens + count <= window => {
                batch.push(i);
                tokens += count;
            }
            _ => {
                batches.push(vec![i]);
                tokens = count;
            }
        }
    }
    let expected = serde_json::to_value(space).expect("空间可序列化");
    let mut done = 0;
    for batch in batches {
        let texts: Vec<String> = batch.iter().map(|&i| chunks[i].retrieval_text()).collect();
        let semantic = format!("embed:{}", hash_json(&json!(batch.iter().map(|&i| &hashes[i]).collect::<Vec<_>>())));
        let request = json!({"taskId": "knowledge.embed", "profileId": profile, "capability": "embedding", "messages": [], "texts": texts, "candidateIds": [],
            "maximumOutputTokens": 0, "idempotencyKey": model_key(job, &semantic), "expectedEmbeddingSpace": expected});
        let outcome = model::invoke(&state.up, op, &request).await?;
        let vectors: Vec<Vec<f64>> = serde_json::from_value(outcome.output["vectors"].clone()).unwrap_or_default();
        if outcome.output["embeddingSpace"] != expected || vectors.len() != batch.len() || vectors.iter().any(|v| v.len() as i64 != space.dimensions || v.iter().any(|x| !x.is_finite())) {
            return Err(Problem::new("INTEGRITY_ERROR", "分块嵌入数量、空间或维度不一致"));
        }
        for (&i, vector) in batch.iter().zip(vectors) {
            let vector: Vec<f32> = vector.iter().map(|x| *x as f32).collect();
            client
                .execute(
                    "INSERT INTO search.embeddings (site_id, space_id, text_hash, vector) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING",
                    &[&job.site, &space.space_id, &hashes[i], &json!(vector)],
                )
                .await
                .map_err(storage)?;
            known.insert(hashes[i].clone(), vector);
        }
        done += batch.len() as i32;
        client.execute("UPDATE search.builds SET completed_chunks = $2 WHERE build_id = $1", &[&job.build_id, &done]).await.map_err(storage)?;
    }
    Ok(hashes.iter().map(|h| known[h].clone()).collect())
}

/// 生效：先整篇替换全文索引，再在一个事务里换向量、分块与生效指针；之前生效的构建记为被替代。
async fn activate(state: &State, job: &Job, chunks: &[Chunk], vectors: &[Vec<f32>]) -> Result<(), Problem> {
    let (site, document, space) = (&job.site, &job.document.document_id, &job.config.space.space_id);
    let entries: Vec<Entry> = chunks.iter().map(|c| Entry { chunk_id: c.chunk_id.clone(), build_id: job.build_id.clone(), terms: text::term_counts(&c.retrieval_text()) }).collect();
    tokio::task::block_in_place(|| state.lexical.replace_document(site, document, space, &entries)).map_err(storage)?;
    let mut client = state.db.get().await.map_err(storage)?;
    let tx = client.transaction().await.map_err(storage)?;
    let changed = tx
        .execute("UPDATE search.builds SET state = 'active', completed_chunks = total_chunks, updated_at = now() WHERE build_id = $1 AND state = 'validating'", &[&job.build_id])
        .await
        .map_err(storage)?;
    if changed == 0 {
        return Err(Problem::new("CANCELLED", "构建已取消或被替代"));
    }
    write_active(&tx, job, chunks, vectors).await.map_err(storage)?;
    tx.commit().await.map_err(storage)
}

async fn write_active(tx: &Transaction<'_>, job: &Job, chunks: &[Chunk], vectors: &[Vec<f32>]) -> Result<(), tokio_postgres::Error> {
    let (site, document, space) = (&job.site, &job.document.document_id, &job.config.space.space_id);
    tx.execute("UPDATE search.builds SET state = 'superseded', updated_at = now() WHERE site_id = $1 AND document_id = $2 AND state = 'active' AND build_id <> $3", &[site, document, &job.build_id]).await?;
    let entries: Vec<(String, Vec<f32>)> = chunks.iter().zip(vectors).map(|(c, v)| (c.chunk_id.clone(), v.clone())).collect();
    vector::replace_document(tx, site, document, space, &job.build_id, &entries).await?;
    tx.execute("DELETE FROM search.chunks WHERE site_id = $1 AND document_id = $2", &[site, document]).await?;
    let insert = tx.prepare("INSERT INTO search.chunks (site_id, build_id, chunk_id, document_id, ordinal, chunk) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING").await?;
    for chunk in chunks {
        let value = serde_json::to_value(chunk).expect("分块可序列化");
        tx.execute(&insert, &[site, &job.build_id, &chunk.chunk_id, document, &(chunk.ordinal as i32), &value]).await?;
    }
    tx.execute(
        "INSERT INTO search.documents (site_id, document_id, build_id, space_id, chunks) VALUES ($1, $2, $3, $4, $5)
         ON CONFLICT (site_id, document_id) DO UPDATE SET build_id = EXCLUDED.build_id, space_id = EXCLUDED.space_id, chunks = EXCLUDED.chunks, updated_at = now()",
        &[site, document, &job.build_id, space, &(chunks.len() as i32)],
    )
    .await?;
    Ok(())
}
