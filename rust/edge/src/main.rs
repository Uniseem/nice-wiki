//! nice-wiki 的唯一入口（edge），在 Caddy 之后：
//!
//! - 入口代理（gateway）：按域名定位站点、站点状态与私有站点门、按路由表转发到各服务；
//!   会话授权和 CSRF 交 identity 核对，签 ≤10 秒的用户委托；访客问答的伪名身份与准入、访问统计信标、登录的网络作用域。
//!   以 portal 的服务身份签服务断言，Go 服务的验证逻辑不变。
//! - 公开页缓存：只缓存匿名 GET 的成功响应，键是 域名 + 路径 + 查询 + 语言头；同一时刻对同一页面的并发未命中只放一个去上游；
//!   发布、改站点设置等写入成功后按站点清掉。
//! - 限流：按客户端 IP 的令牌桶，读写分开；超限返回 429。
//! - 媒体磁盘缓存：带内容哈希的媒体字节落盘，按最近使用淘汰（配置 mediaDiskDir 才开启）。
//! - 流式透传：不缓存的响应（问答的 SSE 等）原样流式转发，不攒包。
//! - 后台单页应用与公开页静态文件、举报页、Caddy 按需证书的询问接口。
//! 不做 TLS（证书由前面的 Caddy 按需签发）。

mod cache;
mod config;
mod disk;
mod gateway;
mod limiter;
mod net;
mod proxy;
mod routes;
mod site;

use nw_platform::{platform, upstream};

use std::sync::Arc;

use anyhow::Context;
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper_util::rt::TokioIo;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let path = std::env::args().nth(1).context("用法：nw-edge <配置文件.json>")?;
    let config = config::load(&path)?;
    let listener = tokio::net::TcpListener::bind(&config.listen).await.with_context(|| format!("监听 {} 失败", config.listen))?;
    let state = Arc::new(proxy::State::new(config)?);
    eprintln!("nw-edge 已启动 {}", state.config.listen);
    loop {
        tokio::select! {
            accepted = listener.accept() => {
                let Ok((stream, peer)) = accepted else { continue };
                let state = state.clone();
                tokio::spawn(async move {
                    let service = service_fn(move |request| proxy::handle(state.clone(), peer, request));
                    let _ = http1::Builder::new().keep_alive(true).serve_connection(TokioIo::new(stream), service).await;
                });
            }
            _ = tokio::signal::ctrl_c() => break,
        }
    }
    Ok(())
}
