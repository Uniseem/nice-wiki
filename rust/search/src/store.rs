//! 本服务在 PostgreSQL 里的 search 模式：索引配置、构建、生效分块与向量、背景短句与向量的复用缓存、
//! 文档卡片、层级摘要与全站概括、站点状态。每张表都带 site_id，所有查询都按站点限定。

use deadpool_postgres::Pool;
use serde_json::Value;

use nw_platform::platform::{canonical, sha256_hex, Problem};

const SCHEMA: &str = "
CREATE SCHEMA IF NOT EXISTS search;
CREATE TABLE IF NOT EXISTS search.configs (
    site_id text PRIMARY KEY, data_epoch text NOT NULL, version bigint NOT NULL, config jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS search.receipts (
    site_id text NOT NULL, operation text NOT NULL, key text NOT NULL, input_hash text NOT NULL, result jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (site_id, operation, key));
CREATE TABLE IF NOT EXISTS search.builds (
    build_id text PRIMARY KEY, site_id text NOT NULL, document_id text NOT NULL, document jsonb NOT NULL, config jsonb NOT NULL,
    state text NOT NULL, generation bigint NOT NULL, completed_chunks integer NOT NULL DEFAULT 0, total_chunks integer NOT NULL DEFAULT 0,
    failure_code text, attempts integer NOT NULL DEFAULT 0, visible_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS builds_site ON search.builds (site_id, created_at DESC);
CREATE INDEX IF NOT EXISTS builds_due ON search.builds (visible_at) WHERE state NOT IN ('active', 'cancelled', 'failed', 'superseded');
CREATE TABLE IF NOT EXISTS search.documents (
    site_id text NOT NULL, document_id text NOT NULL, build_id text NOT NULL, space_id text NOT NULL, chunks integer NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (site_id, document_id));
CREATE TABLE IF NOT EXISTS search.chunks (
    site_id text NOT NULL, build_id text NOT NULL, chunk_id text NOT NULL, document_id text NOT NULL, ordinal integer NOT NULL, chunk jsonb NOT NULL,
    PRIMARY KEY (build_id, chunk_id));
CREATE INDEX IF NOT EXISTS chunks_site ON search.chunks (site_id, chunk_id);
CREATE TABLE IF NOT EXISTS search.watermarks (
    site_id text NOT NULL, document_id text NOT NULL, visibility_revision bigint NOT NULL, withdrawn boolean NOT NULL DEFAULT false,
    PRIMARY KEY (site_id, document_id));
CREATE TABLE IF NOT EXISTS search.contexts (
    site_id text NOT NULL, context_key text NOT NULL, prefix text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (site_id, context_key));
CREATE TABLE IF NOT EXISTS search.embeddings (
    site_id text NOT NULL, space_id text NOT NULL, text_hash text NOT NULL, vector jsonb NOT NULL, PRIMARY KEY (site_id, space_id, text_hash));
CREATE TABLE IF NOT EXISTS search.query_embeddings (
    site_id text NOT NULL, key text NOT NULL, result jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (site_id, key));
CREATE TABLE IF NOT EXISTS search.cards (
    site_id text NOT NULL, document_id text NOT NULL, source_hash text NOT NULL, card jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (site_id, document_id));
CREATE TABLE IF NOT EXISTS search.tree_summaries (
    site_id text NOT NULL, document_id text NOT NULL, input_hash text NOT NULL, summary text NOT NULL, PRIMARY KEY (site_id, document_id));
CREATE TABLE IF NOT EXISTS search.overviews (
    site_id text PRIMARY KEY, input_hash text NOT NULL, overview text NOT NULL, overview_en text NOT NULL,
    updated_at timestamptz NOT NULL, checked_at timestamptz NOT NULL);
CREATE TABLE IF NOT EXISTS search.sites (site_id text PRIMARY KEY, status text NOT NULL, revision bigint NOT NULL);
";

/// 建表在建议锁内做，几个进程同时启动也不互相踩。
pub async fn migrate(pool: &Pool) -> anyhow::Result<()> {
    let client = pool.get().await?;
    client.execute("SELECT pg_advisory_lock(7310120)", &[]).await?;
    let result = client.batch_execute(SCHEMA).await;
    client.execute("SELECT pg_advisory_unlock(7310120)", &[]).await?;
    Ok(result?)
}

pub fn storage(error: impl std::fmt::Display) -> Problem {
    eprintln!("search 存储错误：{error}");
    Problem::new("INTERNAL", "检索存储读写失败")
}

pub fn hash_json(value: &Value) -> String {
    sha256_hex(&canonical(value))
}

/// 幂等回执：同一站点、接口、幂等键只执行一次；键相同而输入不同返回 CONFLICT。
pub async fn receipt(pool: &Pool, site: &str, operation: &str, key: &str, input: &Value) -> Result<Option<Value>, Problem> {
    let client = pool.get().await.map_err(storage)?;
    let row = client
        .query_opt("SELECT input_hash, result FROM search.receipts WHERE site_id = $1 AND operation = $2 AND key = $3", &[&site, &operation, &key])
        .await
        .map_err(storage)?;
    match row {
        None => Ok(None),
        Some(row) if row.get::<_, String>(0) == hash_json(input) => Ok(Some(row.get(1))),
        Some(_) => Err(Problem::new("CONFLICT", "幂等键已用于其他输入")),
    }
}

pub async fn save_receipt(pool: &Pool, site: &str, operation: &str, key: &str, input: &Value, result: &Value) -> Result<(), Problem> {
    let client = pool.get().await.map_err(storage)?;
    client
        .execute(
            "INSERT INTO search.receipts (site_id, operation, key, input_hash, result) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING",
            &[&site, &operation, &key, &hash_json(input), result],
        )
        .await
        .map_err(storage)?;
    Ok(())
}

/// 站点是否在服务中（被下架、删除的站点不再做后台处理）；没收到过状态事件的按正常处理。
pub async fn site_active(pool: &Pool, site: &str) -> Result<bool, Problem> {
    let client = pool.get().await.map_err(storage)?;
    let row = client.query_opt("SELECT status FROM search.sites WHERE site_id = $1", &[&site]).await.map_err(storage)?;
    Ok(row.map(|r| r.get::<_, String>(0) == "active").unwrap_or(true))
}

/// 站点保留期满后清掉它在本服务的全部数据（向量表、全文索引由调用方另清）。
pub async fn purge_site(pool: &Pool, site: &str) -> Result<(), Problem> {
    let mut client = pool.get().await.map_err(storage)?;
    let tx = client.transaction().await.map_err(storage)?;
    for table in ["configs", "receipts", "builds", "documents", "chunks", "watermarks", "contexts", "embeddings", "query_embeddings", "cards", "tree_summaries", "overviews"] {
        tx.execute(&format!("DELETE FROM search.{table} WHERE site_id = $1"), &[&site]).await.map_err(storage)?;
    }
    tx.commit().await.map_err(storage)
}
