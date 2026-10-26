//! nice-wiki 检索服务。
//!
//! - 全文：所有站点共用一个 tantivy 索引，站点、文档、向量空间都是索引字段，查询必须带站点；词项由调用方（knowledge）分好，
//!   这里只做倒排与 BM25 排名，不再自己分词，所以中文双字词项等规则与写入侧一致。
//! - 向量：pgvector，每种维度一张表（HNSW 索引），按站点过滤。
//! - 认证：调用方用共享密钥对 `时间戳 方法 路径 请求体摘要` 做 HMAC-SHA256（见 auth.rs）。

mod api;
mod auth;
mod config;
mod lexical;
mod vector;

use std::sync::Arc;

use anyhow::Context;

pub struct State {
    pub lexical: lexical::Lexical,
    pub vectors: vector::Vectors,
    pub secret: Vec<u8>,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let path = std::env::args().nth(1).context("用法：nw-search <配置文件.json>")?;
    let config = config::load(&path)?;
    let secret = std::fs::read(&config.secret_file).with_context(|| "读取服务密钥失败")?;
    let secret = String::from_utf8_lossy(&secret).trim().as_bytes().to_vec();
    anyhow::ensure!(secret.len() >= 16, "服务密钥太短");
    let dsn = std::fs::read_to_string(&config.database_url_file).with_context(|| "读取数据库连接串失败")?;
    let vectors = vector::Vectors::connect(dsn.trim()).await?;
    let lexical = lexical::Lexical::open(&config.data_dir)?;
    let state = Arc::new(State { lexical, vectors, secret });
    let app = api::router(state);
    let listener = tokio::net::TcpListener::bind(&config.listen).await.with_context(|| format!("监听 {} 失败", config.listen))?;
    eprintln!("nw-search 已启动 {}", config.listen);
    axum::serve(listener, app)
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
        })
        .await?;
    Ok(())
}
