//! 接口与存储共用的类型，字段与 modules/knowledge/contracts/types.json 一致（问答服务按该 schema 严格校验响应）。

use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, PartialEq, Eq, Hash, Debug)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct DocRef {
    pub document_id: String,
    pub published_revision_id: String,
    pub revision: i64,
    pub content_hash: String,
    pub schema_version: i64,
    pub data_epoch: String,
}

#[derive(Serialize, Deserialize, Clone, PartialEq, Debug)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Space {
    pub space_id: String,
    pub provider: String,
    pub model_version: String,
    pub dimensions: i64,
    pub preprocessing_version: String,
}

#[derive(Serialize, Deserialize, Clone, PartialEq, Debug)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct IndexConfig {
    pub config_id: String,
    pub version: i64,
    pub enabled: bool,
    pub embedding_profile_id: String,
    pub space: Space,
    pub chunk_tokens: i64,
    pub overlap_tokens: i64,
    pub language: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rerank_profile_id: Option<String>,
}

#[derive(Serialize, Deserialize, Clone, PartialEq, Debug)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct BlockRef {
    pub document: DocRef,
    pub block_id: String,
    pub block_hash: String,
}

#[derive(Serialize, Deserialize, Clone, PartialEq, Debug)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Chunk {
    pub chunk_id: String,
    pub document: DocRef,
    pub block: BlockRef,
    pub ordinal: i64,
    pub chunk_hash: String,
    pub text: String,
    pub token_count: i64,
    pub byte_start: i64,
    pub byte_end: i64,
    pub source_offsets: Vec<i64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context_prefix: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context_version_hash: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub embedding_token_count: Option<i64>,
}

impl Chunk {
    /// 检索用的文字：机器背景短句在前，原文不变（引用仍指向原文）。
    pub fn retrieval_text(&self) -> String {
        match self.context_prefix.as_deref() {
            Some(prefix) if !prefix.is_empty() => format!("{prefix}\n{}", self.text),
            _ => self.text.clone(),
        }
    }
}

#[derive(Serialize, Clone, Debug)]
#[serde(rename_all = "camelCase")]
pub struct Candidate {
    pub chunk: Chunk,
    pub lexical_rank: i64,
    pub vector_rank: i64,
    pub score: f64,
    pub visibility_revision: i64,
}

#[derive(Serialize, Clone, Debug)]
#[serde(rename_all = "camelCase")]
pub struct IndexBuild {
    pub build_id: String,
    pub document: DocRef,
    pub space: Space,
    pub state: String,
    pub generation: i64,
    pub completed_chunks: i64,
    pub total_chunks: i64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub failure_code: Option<String>,
}

#[derive(Serialize, Deserialize, Clone, PartialEq, Debug)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct QueryEmbedding {
    pub query_hash: String,
    pub space: Space,
    pub vector: Vec<f64>,
    pub vector_hash: String,
    pub invocation_id: String,
    pub data_epoch: String,
}

/// 文档摘要卡片（英文），给问答选文档用。
#[derive(Serialize, Deserialize, Clone, Default, Debug)]
pub struct Card {
    pub title: String,
    pub summary: String,
    #[serde(default)]
    pub topics: Vec<String>,
    #[serde(default)]
    pub questions: Vec<String>,
}
