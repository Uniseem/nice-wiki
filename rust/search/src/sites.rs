//! 读其他服务的权威数据：站点基准语言（site-config）、模型目录与任务路由（model-gateway）、任务记忆（memory）、
//! 发布树、发布正文与发布有效性（content）。全部以 knowledge 的服务身份、按站点签断言调用。

use std::collections::HashMap;
use std::time::{Duration, Instant};

use axum::http::Method;
use serde::Deserialize;
use serde_json::Value;

use nw_platform::platform::Problem;
use nw_platform::upstream::Operation;

use crate::types::{Candidate, DocRef, IndexConfig, Space};
use crate::State;

fn decode<T: serde::de::DeserializeOwned>(value: Value, what: &str) -> Result<T, Problem> {
    serde_json::from_value(value).map_err(|_| Problem::new("INTEGRITY_ERROR", format!("{what}响应不可读")))
}

/// 本站点基准语言（缓存一分钟）。
pub async fn base_language(state: &State, op: &Operation) -> Result<String, Problem> {
    if let Some((language, at)) = state.languages.lock().unwrap().get(&op.site_id) {
        if at.elapsed() < Duration::from_secs(60) {
            return Ok(language.clone());
        }
    }
    let body = state.up.json("site-config", Method::GET, "/v1/config", None, op).await?;
    let language = body["config"]["baseLanguage"].as_str().filter(|l| !l.is_empty()).ok_or_else(|| Problem::new("DEPENDENCY_UNAVAILABLE", "站点基准语言未配置"))?.to_owned();
    state.languages.lock().unwrap().insert(op.site_id.clone(), (language.clone(), Instant::now()));
    Ok(language)
}

#[derive(Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Model {
    pub model_id: String,
    #[serde(default, rename = "providerModelVersion")]
    pub model_version: String,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default, rename = "maxInputTokens")]
    pub max_input_tokens: i64,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub embedding_space: Option<Space>,
}

#[derive(Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct Profile {
    pub profile_id: String,
    pub cost_class: String,
    pub tier: String,
    #[serde(default, rename = "orderedModelIds")]
    pub models: Vec<String>,
}

pub struct Catalog {
    pub models: Vec<Model>,
    pub profiles: Vec<Profile>,
}

impl Catalog {
    /// 指定费用类别下、首选模型的向量空间与索引完全相同的嵌入档位（profile 为空时任取一个）。
    pub fn embedding(&self, profile: &str, cost_class: &str, space: &Space) -> Result<(&Profile, &Model), Problem> {
        for candidate in &self.profiles {
            if (!profile.is_empty() && candidate.profile_id != profile) || candidate.cost_class != cost_class || candidate.tier != "embedding" {
                continue;
            }
            if let Some(model) = candidate.models.first().and_then(|id| self.models.iter().find(|m| &m.model_id == id)) {
                if model.enabled && model.embedding_space.as_ref() == Some(space) {
                    return Ok((candidate, model));
                }
            }
        }
        Err(Problem::new("DEPENDENCY_UNAVAILABLE", "当前费用类别未配置与索引完全相同的嵌入空间"))
    }

    /// 某一档位下有可用模型支持该能力的第一个档位。
    pub fn text(&self, tier: &str, cost_class: &str, capability: &str) -> Result<&Profile, Problem> {
        self.profiles
            .iter()
            .find(|p| p.tier == tier && p.cost_class == cost_class && p.models.iter().any(|id| self.models.iter().any(|m| &m.model_id == id && m.enabled && m.capabilities.iter().any(|c| c == capability))))
            .ok_or_else(|| Problem::new("DEPENDENCY_UNAVAILABLE", "本费用类别未配置所需文本模型档位"))
    }
}

pub async fn catalog(state: &State, op: &Operation) -> Result<Catalog, Problem> {
    #[derive(Deserialize)]
    struct Models {
        models: Vec<Model>,
    }
    #[derive(Deserialize)]
    struct Profiles {
        profiles: Vec<Profile>,
    }
    // 网关只给带可信费用类别的服务调用列档位；管理端读选项时请求本身没有费用类别，按后台算。
    let mut op = op.clone();
    if op.cost_class.is_empty() {
        op.cost_class = "background".into();
    }
    let models: Models = decode(state.up.json("model-gateway", Method::GET, "/v1/models", None, &op).await?, "模型目录")?;
    let profiles: Profiles = decode(state.up.json("model-gateway", Method::GET, "/v1/profiles", None, &op).await?, "模型档位")?;
    Ok(Catalog { models: models.models, profiles: profiles.profiles })
}

pub struct Route {
    pub profile_id: String,
    pub route_version: String,
    /// 候选模型里最小的输入窗口。
    pub window: usize,
}

/// 任务路由（按调用的费用类别解析出档位与候选模型）。
pub async fn route(state: &State, op: &Operation, task: &str) -> Result<Route, Problem> {
    let body = serde_json::json!({"taskId": task, "costClass": op.cost_class});
    let result = state.up.json("model-gateway", Method::POST, "/v1/task-routes:resolve", Some(body), op).await?;
    let profile_id = result["resolved"]["profile"]["profileId"].as_str().ok_or_else(|| Problem::new("INTEGRITY_ERROR", "任务路由响应不完整"))?.to_owned();
    let window = result["candidates"].as_array().into_iter().flatten().filter_map(|c| c["model"]["maxInputTokens"].as_i64()).filter(|w| *w > 0).min().unwrap_or(0) as usize;
    if window == 0 {
        return Err(Problem::new("UNSUPPORTED_CAPABILITY", "任务候选未声明实际输入窗口"));
    }
    Ok(Route { profile_id, route_version: result["routeVersion"].as_str().unwrap_or_default().to_owned(), window })
}

/// 任务记忆：返回 (正文, 版本摘要)。
pub async fn memory(state: &State, op: &Operation, task: &str, language: &str) -> Result<(String, String), Problem> {
    let body = serde_json::json!({"taskId": task, "language": language});
    let result = state.up.json("memory", Method::POST, "/v1/context:resolve", Some(body), op).await?;
    Ok((result["text"].as_str().unwrap_or_default().to_owned(), result["versionHash"].as_str().unwrap_or_default().to_owned()))
}

#[derive(Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
pub struct TreeNode {
    pub document_id: String,
    #[serde(default)]
    pub parent_id: Option<String>,
    pub title: String,
    #[serde(default)]
    pub canonical_path: String,
    pub reference: DocRef,
    #[serde(default)]
    pub visibility_revision: i64,
}

pub struct Tree {
    pub nodes: Vec<TreeNode>,
    pub revision: i64,
}

pub async fn public_tree(state: &State, op: &Operation) -> Result<Tree, Problem> {
    let mut body = state.up.json("content", Method::GET, "/v1/public-tree", None, op).await?;
    let revision = body["treeRevision"].as_i64().unwrap_or_default();
    Ok(Tree { nodes: decode(body["nodes"].take(), "发布树")?, revision })
}

/// 发布正文（固定修订）：返回 (引用, 正文树)。
pub async fn published(state: &State, op: &Operation, document: &DocRef) -> Result<(DocRef, Value), Problem> {
    let target = format!("/v1/documents/{}/published?publishedRevisionId={}", document.document_id, document.published_revision_id);
    let mut body = state.up.request("content", Method::GET, &target, None, op, None, Duration::from_secs(30)).await?;
    Ok((decode(body["reference"].take(), "发布正文")?, body["body"].take()))
}

/// 每个引用的发布是否仍然有效且就是当前修订：有效时给出可见性水位；每 100 个引用问一次 content。
pub async fn validate_refs(state: &State, op: &Operation, refs: &[DocRef]) -> Result<HashMap<DocRef, i64>, Problem> {
    let mut approved = HashMap::new();
    for batch in refs.chunks(100) {
        let body = serde_json::json!({"refs": batch});
        let result = state.up.json("content", Method::POST, "/v1/publications:validate", Some(body), op).await?;
        let validations = result["validations"].as_array().filter(|v| v.len() == batch.len()).ok_or_else(|| Problem::new("DEPENDENCY_UNAVAILABLE", "发布权威返回不完整"))?;
        for (reference, validation) in batch.iter().zip(validations) {
            let current = validation["currentRevision"].as_i64() == Some(reference.revision);
            if validation["visible"] == true && validation["matchesCurrent"] == true && current {
                approved.insert(reference.clone(), validation["visibilityRevision"].as_i64().unwrap_or_default());
            }
        }
    }
    Ok(approved)
}

/// 只留下发布仍然有效、且就是当前修订的候选，并填上可见性水位。
pub async fn validate(state: &State, op: &Operation, candidates: Vec<Candidate>) -> Result<Vec<Candidate>, Problem> {
    let mut refs: Vec<DocRef> = Vec::new();
    for candidate in &candidates {
        if candidate.chunk.document.data_epoch != op.data_epoch {
            return Err(Problem::new("STALE_RESULT", "候选来源代次不匹配"));
        }
        if !refs.contains(&candidate.chunk.document) {
            refs.push(candidate.chunk.document.clone());
        }
    }
    let approved = validate_refs(state, op, &refs).await?;
    Ok(candidates
        .into_iter()
        .filter_map(|mut candidate| {
            candidate.visibility_revision = *approved.get(&candidate.chunk.document)?;
            Some(candidate)
        })
        .collect())
}

/// 本站点的索引配置；还没配置或代次已变时为空。
pub async fn config(state: &State, op: &Operation) -> Result<Option<IndexConfig>, Problem> {
    let client = state.db.get().await.map_err(crate::store::storage)?;
    let row = client.query_opt("SELECT config, data_epoch FROM search.configs WHERE site_id = $1", &[&op.site_id]).await.map_err(crate::store::storage)?;
    match row {
        Some(row) if row.get::<_, String>(1) == op.data_epoch => Ok(Some(decode(row.get(0), "索引配置")?)),
        _ => Ok(None),
    }
}

/// 检索类接口要求本站点已启用索引。
pub async fn enabled_config(state: &State, op: &Operation) -> Result<IndexConfig, Problem> {
    config(state, op).await?.filter(|c| c.enabled).ok_or_else(|| Problem::new("DEPENDENCY_UNAVAILABLE", "本站点还没有启用知识库索引"))
}
