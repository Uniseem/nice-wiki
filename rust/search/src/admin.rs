//! 后台管理接口：索引配置（CAS 版本）、配置选项（模型档位与发布文档）、构建的创建 / 列表 / 查看 / 取消。
//! 换配置不自动触发付费重建。

use serde::Deserialize;
use serde_json::{json, Value};

use nw_platform::platform::Problem;

use crate::api::Ctx;
use crate::build::{self, BUILD_COLUMNS};
use crate::sites;
use crate::store::{receipt, save_receipt, storage};
use crate::types::{DocRef, IndexConfig};

pub const PREPROCESSING: [&str; 2] = ["none;tiktoken-go@v0.7.0;cl100k_base", "none;tiktoken-go@v0.7.0;o200k_base"];

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct PutInput {
    config: IndexConfig,
    expected_version: i64,
    idempotency_key: String,
}

pub async fn put_config(ctx: &Ctx) -> Result<Value, Problem> {
    let input: PutInput = ctx.input()?;
    let (state, op) = (&ctx.state, &ctx.op);
    let raw: Value = serde_json::from_slice(&ctx.body).unwrap_or_default();
    if let Some(original) = receipt(&state.db, &op.site_id, "putIndexConfig", &input.idempotency_key, &raw).await? {
        return Ok(original);
    }
    let config = input.config;
    if config.overlap_tokens >= config.chunk_tokens || !(128..=1024).contains(&config.chunk_tokens) || !(0..=128).contains(&config.overlap_tokens) || config.version != input.expected_version {
        return Err(Problem::new("VALIDATION", "分块重叠或提交版本不合法"));
    }
    if config.language != sites::base_language(state, op).await? {
        return Err(Problem::new("VALIDATION", "只允许站点基准语言入库"));
    }
    if config.enabled {
        validate_space(ctx, &config).await?;
    }
    let mut client = state.db.get().await.map_err(storage)?;
    let tx = client.transaction().await.map_err(storage)?;
    let row = tx.query_opt("SELECT version, config, data_epoch FROM search.configs WHERE site_id = $1 FOR UPDATE", &[&op.site_id]).await.map_err(storage)?;
    let (version, id) = match &row {
        Some(row) if row.get::<_, String>(2) == op.data_epoch => (row.get::<_, i64>(0), row.get::<_, Value>(1)["configId"].as_str().unwrap_or_default().to_owned()),
        _ => (0, String::new()),
    };
    if version != input.expected_version || (!id.is_empty() && id != config.config_id) {
        return Err(Problem::new("VERSION_MISMATCH", "索引配置版本已变化"));
    }
    let mut next = config;
    next.version = version + 1;
    let stored = serde_json::to_value(&next).expect("配置可序列化");
    tx.execute(
        "INSERT INTO search.configs (site_id, data_epoch, version, config) VALUES ($1, $2, $3, $4)
         ON CONFLICT (site_id) DO UPDATE SET data_epoch = EXCLUDED.data_epoch, version = EXCLUDED.version, config = EXCLUDED.config, updated_at = now()",
        &[&op.site_id, &op.data_epoch, &next.version, &stored],
    )
    .await
    .map_err(storage)?;
    tx.commit().await.map_err(storage)?;
    let result = json!({"config": stored});
    save_receipt(&state.db, &op.site_id, "putIndexConfig", &input.idempotency_key, &raw, &result).await?;
    Ok(result)
}

/// 固定档位的首选模型与完整向量空间都要兼容，不能只比模型名或维度。
async fn validate_space(ctx: &Ctx, config: &IndexConfig) -> Result<(), Problem> {
    let catalog = sites::catalog(&ctx.state, &ctx.op).await?;
    let (_, model) = catalog.embedding(&config.embedding_profile_id, "background", &config.space).map_err(|_| Problem::new("VALIDATION", "嵌入档位与当前完整空间不兼容"))?;
    if !PREPROCESSING.contains(&config.space.preprocessing_version.as_str()) {
        return Err(Problem::new("UNSUPPORTED_SCHEMA", "嵌入模型未声明本域支持的真实分词器"));
    }
    if !model.capabilities.iter().any(|c| c == "embedding") || model.max_input_tokens < config.chunk_tokens {
        return Err(Problem::new("VALIDATION", "嵌入主模型能力或输入预算不兼容"));
    }
    if let Some(rerank) = &config.rerank_profile_id {
        let found = catalog.profiles.iter().any(|p| {
            &p.profile_id == rerank && p.cost_class == "qa" && p.tier == "rerank" && p.models.first().is_some_and(|id| catalog.models.iter().any(|m| &m.model_id == id && m.enabled && m.capabilities.iter().any(|c| c == "rerank")))
        });
        if !found {
            return Err(Problem::new("VALIDATION", "重排档位没有兼容主模型"));
        }
    }
    Ok(())
}

pub async fn read_config(ctx: &Ctx) -> Result<Value, Problem> {
    let config = sites::config(&ctx.state, &ctx.op).await?.ok_or_else(|| Problem::new("NOT_FOUND", "还没有索引配置"))?;
    Ok(json!({"version": config.version, "config": config}))
}

fn error_code(problem: &Problem) -> &'static str {
    problem.code
}

/// 后台选择框用的真实选项；跨服务读取失败时如实给出错误码，不伪装成空目录。
pub async fn options(ctx: &Ctx) -> Result<Value, Problem> {
    let (state, op) = (&ctx.state, &ctx.op);
    let language = sites::base_language(state, op).await?;
    let mut result = json!({"baseLanguage": language, "modelsState": "unavailable", "documentsState": "unavailable", "profiles": [], "documents": []});
    {
        let client = state.db.get().await.map_err(storage)?;
        let row = client
            .query_opt("SELECT overview, to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') FROM search.overviews WHERE site_id = $1 AND overview <> ''", &[&op.site_id])
            .await
            .map_err(storage)?;
        if let Some(row) = row {
            result["siteOverview"] = row.get::<_, String>(0).into();
            result["siteOverviewUpdatedAt"] = row.get::<_, String>(1).into();
        }
    }
    match sites::catalog(state, op).await {
        Err(problem) => result["modelErrorCode"] = error_code(&problem).into(),
        Ok(catalog) => {
            result["modelsState"] = "ready".into();
            let mut profiles = Vec::new();
            for profile in catalog.profiles.iter().filter(|p| matches!(p.tier.as_str(), "embedding" | "rerank" | "fast")) {
                let mut option = json!({"profileId": profile.profile_id, "tier": profile.tier, "costClass": profile.cost_class, "label": profile.tier, "usable": false, "reasonCode": "NOT_FOUND"});
                if let Some(model) = profile.models.first().and_then(|id| catalog.models.iter().find(|m| &m.model_id == id)) {
                    let capability = match profile.tier.as_str() {
                        "embedding" => "embedding",
                        "rerank" => "rerank",
                        _ => "chat",
                    };
                    let mut usable = model.enabled && model.capabilities.iter().any(|c| c == capability) && (profile.tier != "embedding" || model.embedding_space.is_some());
                    let mut reason = if usable { "" } else { "UNSUPPORTED_CAPABILITY" };
                    if let (true, Some(space)) = (profile.tier == "embedding", &model.embedding_space) {
                        if !PREPROCESSING.contains(&space.preprocessing_version.as_str()) {
                            usable = false;
                            reason = "UNSUPPORTED_SCHEMA";
                        }
                    }
                    option["modelId"] = model.model_id.clone().into();
                    option["label"] = model.model_version.clone().into();
                    if let Some(space) = &model.embedding_space {
                        option["space"] = serde_json::to_value(space).expect("空间可序列化");
                    }
                    option["usable"] = usable.into();
                    match reason {
                        "" => {
                            option.as_object_mut().expect("对象").remove("reasonCode");
                        }
                        code => option["reasonCode"] = code.into(),
                    }
                }
                profiles.push(option);
            }
            result["profiles"] = profiles.into();
        }
    }
    match sites::public_tree(state, op).await {
        Err(problem) => result["documentErrorCode"] = error_code(&problem).into(),
        Ok(tree) => {
            if tree.nodes.iter().any(|n| n.reference.data_epoch != op.data_epoch || n.reference.document_id != n.document_id) {
                result["documentErrorCode"] = "STALE_RESULT".into();
                return Ok(result);
            }
            result["documents"] = tree.nodes.iter().map(|n| json!({"documentId": n.document_id, "title": n.title, "reference": n.reference, "visibilityRevision": n.visibility_revision})).collect();
            result["treeRevision"] = tree.revision.into();
            result["documentsState"] = "ready".into();
        }
    }
    Ok(result)
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct CreateInput {
    document: DocRef,
    force: bool,
    idempotency_key: String,
}

/// 确认这是当前公开修订后接受构建（202 只表示已持久排队）。
pub async fn create_build(ctx: &Ctx) -> Result<Value, Problem> {
    let input: CreateInput = ctx.input()?;
    let (state, op) = (&ctx.state, &ctx.op);
    let raw: Value = serde_json::from_slice(&ctx.body).unwrap_or_default();
    if let Some(original) = receipt(&state.db, &op.site_id, "createBuild", &input.idempotency_key, &raw).await? {
        return Ok(original);
    }
    let visibility = *sites::validate_refs(state, op, std::slice::from_ref(&input.document))
        .await?
        .get(&input.document)
        .ok_or_else(|| Problem::new("STALE_RESULT", "发布引用不是当前公开修订"))?;
    let config = sites::config(state, op).await?.filter(|c| c.enabled).ok_or_else(|| Problem::new("VALIDATION", "知识入库未启用"))?;
    let mut client = state.db.get().await.map_err(storage)?;
    let tx = client.transaction().await.map_err(storage)?;
    let mark = tx
        .query_opt("SELECT visibility_revision, withdrawn FROM search.watermarks WHERE site_id = $1 AND document_id = $2", &[&op.site_id, &input.document.document_id])
        .await
        .map_err(storage)?;
    if let Some(mark) = mark {
        let (known, withdrawn): (i64, bool) = (mark.get(0), mark.get(1));
        if known > visibility || (known == visibility && withdrawn) {
            return Err(Problem::new("STALE_RESULT", "发布证明已被撤回水位替代"));
        }
    }
    let build = build::queue_build(&tx, &op.site_id, &config, &input.document, input.force).await?;
    build::raise_watermark(&tx, &op.site_id, &input.document.document_id, visibility, false).await?;
    tx.commit().await.map_err(storage)?;
    state.wake.notify_one();
    let result = json!({"build": build});
    save_receipt(&state.db, &op.site_id, "createBuild", &input.idempotency_key, &raw, &result).await?;
    Ok(result)
}

/// 按 buildId 顺序分页；游标就是上一页最后一个 buildId（查询总按站点限定）。
pub async fn list_builds(ctx: &Ctx) -> Result<Value, Problem> {
    let limit: i64 = match ctx.param("limit") {
        None => 20,
        Some(raw) => raw.parse().ok().filter(|l| (1..=100).contains(l)).ok_or_else(|| Problem::new("VALIDATION", "索引管理分页数量不合法"))?,
    };
    let after = ctx.param("cursor").unwrap_or_default();
    let document = ctx.param("documentId").unwrap_or_default();
    let client = ctx.state.db.get().await.map_err(storage)?;
    let rows = client
        .query(
            &format!("SELECT {BUILD_COLUMNS} FROM search.builds WHERE site_id = $1 AND build_id > $2 AND ($3 = '' OR document_id = $3) ORDER BY build_id LIMIT $4"),
            &[&ctx.op.site_id, &after, &document, &(limit + 1)],
        )
        .await
        .map_err(storage)?;
    let builds: Vec<_> = rows.iter().take(limit as usize).map(build::from_row).collect();
    let mut result = json!({"builds": builds});
    if rows.len() as i64 > limit {
        result["nextCursor"] = builds.last().map(|b| b.build_id.clone()).into();
    }
    Ok(result)
}

pub async fn get_build(ctx: &Ctx, id: &str) -> Result<Value, Problem> {
    let client = ctx.state.db.get().await.map_err(storage)?;
    Ok(json!({"build": build::read(&client, &ctx.op.site_id, id).await?}))
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct CancelInput {
    build_id: String,
    #[allow(dead_code)]
    reason: String,
    #[allow(dead_code)]
    idempotency_key: String,
}

/// 未结束的构建标为取消（工作循环在下一个阶段边界发现并停下）；已结束的原样返回。
pub async fn cancel_build(ctx: &Ctx, id: &str) -> Result<Value, Problem> {
    let input: CancelInput = ctx.input()?;
    if input.build_id != id {
        return Err(Problem::new("VALIDATION", "路径与正文的构建不一致"));
    }
    let client = ctx.state.db.get().await.map_err(storage)?;
    client
        .execute(
            "UPDATE search.builds SET state = 'cancelled', updated_at = now() WHERE site_id = $1 AND build_id = $2 AND state NOT IN ('active', 'failed', 'superseded', 'cancelled')",
            &[&ctx.op.site_id, &id],
        )
        .await
        .map_err(storage)?;
    Ok(json!({"build": build::read(&client, &ctx.op.site_id, id).await?}))
}
