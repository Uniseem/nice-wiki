//! 向量：pgvector，每种维度一张表（HNSW 索引），按站点与向量空间过滤。
//! 写入在构建生效的事务里做（与分块、生效指针同一事务）。

use deadpool_postgres::{GenericClient, Pool, Transaction};
use nw_platform::platform::Problem;

use crate::store::storage;

/// pgvector 的 vector 类型最多建 2000 维的 HNSW 索引，更高维度用 halfvec（最多 4000 维）。
fn column_type(dimension: usize) -> &'static str {
    if dimension <= 2000 {
        "vector"
    } else {
        "halfvec"
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

pub struct VectorHit {
    pub chunk_id: String,
}

async fn tables(client: &impl GenericClient) -> Result<Vec<String>, tokio_postgres::Error> {
    let rows = client.query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'search' AND table_name LIKE 'vectors\\_%'", &[]).await?;
    Ok(rows.iter().map(|row| format!("search.{}", row.get::<_, String>(0))).collect())
}

async fn ensure_table(client: &impl GenericClient, dimension: usize) -> Result<String, tokio_postgres::Error> {
    let name = format!("search.vectors_{dimension}");
    let ops = if dimension <= 2000 { "vector_cosine_ops" } else { "halfvec_cosine_ops" };
    client
        .batch_execute(&format!(
            "CREATE TABLE IF NOT EXISTS {name} (
                 site_id text NOT NULL, space_id text NOT NULL, chunk_id text NOT NULL, document_id text NOT NULL, build_id text NOT NULL,
                 embedding {kind}({dimension}) NOT NULL, PRIMARY KEY (site_id, space_id, chunk_id));
             CREATE INDEX IF NOT EXISTS vectors_{dimension}_doc ON {name} (site_id, document_id);
             CREATE INDEX IF NOT EXISTS vectors_{dimension}_hnsw ON {name} USING hnsw (embedding {ops});",
            kind = column_type(dimension)
        ))
        .await?;
    Ok(name)
}

/// 整篇文档替换：删掉所有维度表里这篇文档的旧向量，再写新的。
pub async fn replace_document(tx: &Transaction<'_>, site: &str, document: &str, space: &str, build: &str, entries: &[(String, Vec<f32>)]) -> Result<(), tokio_postgres::Error> {
    delete_document(tx, site, document).await?;
    let Some(dimension) = entries.first().map(|(_, v)| v.len()) else { return Ok(()) };
    let name = ensure_table(tx, dimension).await?;
    let statement = tx
        .prepare(&format!(
            "INSERT INTO {name} (site_id, space_id, chunk_id, document_id, build_id, embedding) VALUES ($1, $2, $3, $4, $5, $6::text::{kind})
             ON CONFLICT (site_id, space_id, chunk_id) DO UPDATE SET document_id = EXCLUDED.document_id, build_id = EXCLUDED.build_id, embedding = EXCLUDED.embedding",
            kind = column_type(dimension)
        ))
        .await?;
    for (chunk, vector) in entries {
        tx.execute(&statement, &[&site, &space, chunk, &document, &build, &literal(vector)]).await?;
    }
    Ok(())
}

pub async fn delete_document(client: &impl GenericClient, site: &str, document: &str) -> Result<(), tokio_postgres::Error> {
    for name in tables(client).await? {
        client.execute(&format!("DELETE FROM {name} WHERE site_id = $1 AND document_id = $2"), &[&site, &document]).await?;
    }
    Ok(())
}

pub async fn delete_site(client: &impl GenericClient, site: &str) -> Result<(), tokio_postgres::Error> {
    for name in tables(client).await? {
        client.execute(&format!("DELETE FROM {name} WHERE site_id = $1"), &[&site]).await?;
    }
    Ok(())
}

/// 余弦相似度排名。HNSW 带过滤时开迭代扫描，避免过滤后结果不足。
pub async fn search(pool: &Pool, site: &str, space: &str, vector: &[f32], documents: &[String], limit: i64) -> Result<Vec<VectorHit>, Problem> {
    let mut client = pool.get().await.map_err(storage)?;
    let name = format!("search.vectors_{}", vector.len());
    if !tables(&client).await.map_err(storage)?.contains(&name) {
        return Ok(Vec::new());
    }
    let kind = column_type(vector.len());
    let tx = client.transaction().await.map_err(storage)?;
    tx.batch_execute("SET LOCAL hnsw.iterative_scan = relaxed_order; SET LOCAL hnsw.ef_search = 100;").await.map_err(storage)?;
    let query = literal(vector);
    let filter = if documents.is_empty() { "" } else { "AND document_id = ANY($4)" };
    let sql = format!("SELECT chunk_id FROM {name} WHERE site_id = $1 AND space_id = $2 {filter} ORDER BY embedding <=> $3::text::{kind} LIMIT {limit}");
    let rows = if documents.is_empty() { tx.query(&sql, &[&site, &space, &query]).await } else { tx.query(&sql, &[&site, &space, &query, &documents]).await }.map_err(storage)?;
    tx.commit().await.map_err(storage)?;
    Ok(rows.iter().map(|row| VectorHit { chunk_id: row.get(0) }).collect())
}
