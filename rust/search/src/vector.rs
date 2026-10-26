use std::collections::HashMap;

use anyhow::Context;
use deadpool_postgres::{Config, Pool, Runtime};
use tokio_postgres::NoTls;

pub struct VectorEntry {
    pub chunk_id: String,
    pub build_id: String,
    pub vector: Vec<f32>,
}

pub struct VectorHit {
    pub chunk_id: String,
    pub build_id: String,
    pub document_id: String,
    pub score: f64,
}

pub struct Vectors {
    pool: Pool,
}

/// pgvector 的 vector 类型最多建 2000 维的 HNSW 索引，更高维度用 halfvec（最多 4000 维）。
fn column_type(dimension: usize) -> &'static str {
    if dimension <= 2000 {
        "vector"
    } else {
        "halfvec"
    }
}

fn ops_class(dimension: usize) -> &'static str {
    if dimension <= 2000 {
        "vector_cosine_ops"
    } else {
        "halfvec_cosine_ops"
    }
}

fn literal(vector: &[f32]) -> String {
    let mut text = String::with_capacity(vector.len() * 10 + 2);
    text.push('[');
    for (index, value) in vector.iter().enumerate() {
        if index > 0 {
            text.push(',');
        }
        text.push_str(&value.to_string());
    }
    text.push(']');
    text
}

impl Vectors {
    pub async fn connect(dsn: &str) -> anyhow::Result<Self> {
        let mut config = Config::new();
        config.url = Some(dsn.to_owned());
        config.pool = Some(deadpool_postgres::PoolConfig::new(16));
        let pool = config.create_pool(Some(Runtime::Tokio1), NoTls).with_context(|| "创建数据库连接池失败")?;
        let client = pool.get().await.with_context(|| "连接数据库失败")?;
        client
            .batch_execute(
                "CREATE SCHEMA IF NOT EXISTS search;
                 CREATE TABLE IF NOT EXISTS search.documents (
                     site_id text NOT NULL, document_id text NOT NULL, build_id text NOT NULL, space_id text NOT NULL, chunks integer NOT NULL,
                     updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (site_id, document_id));",
            )
            .await
            .with_context(|| "初始化 search 模式失败")?;
        Ok(Self { pool })
    }

    async fn ensure_table(&self, dimension: usize) -> anyhow::Result<String> {
        anyhow::ensure!((1..=4000).contains(&dimension), "向量维度超出范围");
        let name = format!("search.vectors_{dimension}");
        let client = self.pool.get().await?;
        let kind = column_type(dimension);
        client
            .batch_execute(&format!(
                "CREATE TABLE IF NOT EXISTS {name} (
                     site_id text NOT NULL, space_id text NOT NULL, chunk_id text NOT NULL, document_id text NOT NULL, build_id text NOT NULL,
                     embedding {kind}({dimension}) NOT NULL, PRIMARY KEY (site_id, space_id, chunk_id));
                 CREATE INDEX IF NOT EXISTS vectors_{dimension}_doc ON {name} (site_id, document_id);
                 CREATE INDEX IF NOT EXISTS vectors_{dimension}_hnsw ON {name} USING hnsw (embedding {ops}) ;",
                ops = ops_class(dimension)
            ))
            .await?;
        Ok(name)
    }

    async fn tables(&self) -> anyhow::Result<Vec<String>> {
        let client = self.pool.get().await?;
        let rows = client.query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'search' AND table_name LIKE 'vectors\\_%'", &[]).await?;
        Ok(rows.iter().map(|row| format!("search.{}", row.get::<_, String>(0))).collect())
    }

    /// 整篇文档替换：同事务里删掉所有维度表里这篇文档的旧向量，再写新的，并更新清单。
    pub async fn replace_document(&self, site: &str, document: &str, space: &str, build: &str, entries: &[VectorEntry], chunk_count: usize) -> anyhow::Result<()> {
        let dimension = entries.iter().map(|e| e.vector.len()).find(|n| *n > 0);
        let table = match dimension {
            Some(n) => Some((n, self.ensure_table(n).await?)),
            None => None,
        };
        let existing = self.tables().await?;
        let mut client = self.pool.get().await?;
        let transaction = client.transaction().await?;
        for name in &existing {
            transaction.execute(&format!("DELETE FROM {name} WHERE site_id = $1 AND document_id = $2"), &[&site, &document]).await?;
        }
        if let Some((dimension, name)) = &table {
            let kind = column_type(*dimension);
            let statement = transaction
                .prepare(&format!("INSERT INTO {name} (site_id, space_id, chunk_id, document_id, build_id, embedding) VALUES ($1, $2, $3, $4, $5, $6::text::{kind}) ON CONFLICT (site_id, space_id, chunk_id) DO UPDATE SET document_id = EXCLUDED.document_id, build_id = EXCLUDED.build_id, embedding = EXCLUDED.embedding"))
                .await?;
            for entry in entries {
                if entry.vector.len() != *dimension {
                    anyhow::bail!("同一篇文档的向量维度不一致");
                }
                transaction.execute(&statement, &[&site, &space, &entry.chunk_id, &document, &entry.build_id, &literal(&entry.vector)]).await?;
            }
        }
        transaction
            .execute(
                "INSERT INTO search.documents (site_id, document_id, build_id, space_id, chunks) VALUES ($1, $2, $3, $4, $5)
                 ON CONFLICT (site_id, document_id) DO UPDATE SET build_id = EXCLUDED.build_id, space_id = EXCLUDED.space_id, chunks = EXCLUDED.chunks, updated_at = now()",
                &[&site, &document, &build, &space, &(chunk_count as i32)],
            )
            .await?;
        transaction.commit().await?;
        Ok(())
    }

    pub async fn delete_document(&self, site: &str, document: &str) -> anyhow::Result<()> {
        let existing = self.tables().await?;
        let mut client = self.pool.get().await?;
        let transaction = client.transaction().await?;
        for name in &existing {
            transaction.execute(&format!("DELETE FROM {name} WHERE site_id = $1 AND document_id = $2"), &[&site, &document]).await?;
        }
        transaction.execute("DELETE FROM search.documents WHERE site_id = $1 AND document_id = $2", &[&site, &document]).await?;
        transaction.commit().await?;
        Ok(())
    }

    pub async fn delete_site(&self, site: &str) -> anyhow::Result<()> {
        let existing = self.tables().await?;
        let mut client = self.pool.get().await?;
        let transaction = client.transaction().await?;
        for name in &existing {
            transaction.execute(&format!("DELETE FROM {name} WHERE site_id = $1"), &[&site]).await?;
        }
        transaction.execute("DELETE FROM search.documents WHERE site_id = $1", &[&site]).await?;
        transaction.commit().await?;
        Ok(())
    }

    /// 站点当前索引里每篇文档对应的构建，knowledge 用它做对账。
    pub async fn manifest(&self, site: &str) -> anyhow::Result<HashMap<String, String>> {
        let client = self.pool.get().await?;
        let rows = client.query("SELECT document_id, build_id FROM search.documents WHERE site_id = $1", &[&site]).await?;
        Ok(rows.iter().map(|row| (row.get::<_, String>(0), row.get::<_, String>(1))).collect())
    }

    /// 余弦相似度排名。HNSW 带过滤时开迭代扫描，避免过滤后结果不足。
    pub async fn search(&self, site: &str, space: &str, vector: &[f32], documents: &[String], limit: i64) -> anyhow::Result<Vec<VectorHit>> {
        if vector.is_empty() || limit <= 0 {
            return Ok(Vec::new());
        }
        let dimension = vector.len();
        let name = format!("search.vectors_{dimension}");
        if !self.tables().await?.contains(&name) {
            return Ok(Vec::new());
        }
        let kind = column_type(dimension);
        let mut client = self.pool.get().await?;
        let transaction = client.transaction().await?;
        transaction.batch_execute("SET LOCAL hnsw.iterative_scan = relaxed_order; SET LOCAL hnsw.ef_search = 100;").await?;
        let query = literal(vector);
        let rows = if documents.is_empty() {
            transaction
                .query(
                    &format!("SELECT chunk_id, build_id, document_id, 1 - (embedding <=> $1::text::{kind}) AS score FROM {name} WHERE site_id = $2 AND space_id = $3 ORDER BY embedding <=> $1::text::{kind} LIMIT $4"),
                    &[&query, &site, &space, &limit],
                )
                .await?
        } else {
            transaction
                .query(
                    &format!("SELECT chunk_id, build_id, document_id, 1 - (embedding <=> $1::text::{kind}) AS score FROM {name} WHERE site_id = $2 AND space_id = $3 AND document_id = ANY($4) ORDER BY embedding <=> $1::text::{kind} LIMIT $5"),
                    &[&query, &site, &space, &documents, &limit],
                )
                .await?
        };
        transaction.commit().await?;
        Ok(rows.iter().map(|row| VectorHit { chunk_id: row.get(0), build_id: row.get(1), document_id: row.get(2), score: row.get(3) }).collect())
    }
}
