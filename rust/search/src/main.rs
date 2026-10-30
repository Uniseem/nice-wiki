//! nice-wiki 检索服务，沿用 knowledge 的服务身份与接口（modules/knowledge/contracts/openapi.json）：
//!
//! - 构建：订阅发布变更事件，按站点索引配置取发布正文，切块、写背景短句、嵌入，生效时整篇替换全文与向量；
//!   构建记录在本服务的 search.builds 表里，由进程内的工作循环领取。
//! - 检索：全文（tantivy，所有站点共用一个索引，站点是索引字段）+ 向量（pgvector），倒数排名融合后可选重排，
//!   入选前后都经 content 核对发布仍然有效。
//! - 文档卡片、文档目录视图、层级摘要与全站概括，给问答选文档用。
//!
//! 全部关系数据在 PostgreSQL 的 search 模式，每个查询都按 site_id 限定。

mod admin;
mod api;
mod build;
mod docindex;
mod extract;
mod lexical;
mod overview;
mod retrieve;
mod sites;
mod store;
mod text;
mod types;
mod vector;

use std::collections::{HashMap, HashSet};
use std::sync::{Arc, Mutex};
use std::time::Instant;

use anyhow::Context;
use deadpool_postgres::Pool;
use tokio::sync::Notify;

use nw_platform::inbound::Verifier;
use nw_platform::platform::{Key, ServiceConfig, TrustAnchors};
use nw_platform::upstream::Upstream;

pub const MODULE: &str = "knowledge";

pub struct State {
    pub up: Upstream,
    pub verifier: Verifier,
    pub db: Pool,
    pub lexical: lexical::Lexical,
    /// 站点基准语言的短缓存。
    pub languages: Mutex<HashMap<String, (String, Instant)>>,
    /// 有新构建入队时叫醒工作循环。
    pub wake: Notify,
    /// 正在重算全站概括的站点（同一站点同一时刻只算一份）。
    pub overviews: Mutex<HashSet<String>>,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let path = std::env::args().nth(1).context("用法：nw-search <knowledge 服务配置.json>")?;
    let config = ServiceConfig::load(&path, MODULE)?;
    let anchors = TrustAnchors::load(&config.trust_anchor_ref)?;
    let key = Key::load(&config.signing_key_ref)?;
    anyhow::ensure!(anchors.service(MODULE, &key.id) == Some(key.verifying()), "签名身份与信任锚不一致");
    let db = nw_platform::db::pool(&config.database_ref, 16)?;
    store::migrate(&db).await.context("建立 search 模式失败")?;
    let lexical = lexical::Lexical::open(&format!("{}/index", config.data_root))?;
    let state = Arc::new(State {
        up: Upstream::new(MODULE, key, config.dependencies.clone()),
        verifier: Verifier::new(anchors, MODULE),
        db,
        lexical,
        languages: Mutex::new(HashMap::new()),
        wake: Notify::new(),
        overviews: Mutex::new(HashSet::new()),
    });
    tokio::spawn(build::consume(state.clone()));
    tokio::spawn(build::work(state.clone()));
    tokio::spawn(overview::schedule(state.clone()));
    let listener = tokio::net::TcpListener::bind(&config.listen).await.with_context(|| format!("监听 {} 失败", config.listen))?;
    eprintln!("nw-search（knowledge）已启动 {}", config.listen);
    axum::serve(listener, api::router(state))
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
        })
        .await?;
    Ok(())
}
