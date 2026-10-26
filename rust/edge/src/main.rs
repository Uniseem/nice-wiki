//! nice-wiki 公开入口（edge）：Caddy 之后、portal 之前的一层。
//!
//! - 公开页缓存：只缓存匿名 GET 的成功响应，键是 域名 + 路径 + 查询 + 语言头；同一时刻对同一页面的并发未命中只放一个去上游。
//! - 限流：按客户端 IP 的令牌桶，读写分开；超限返回 429。
//! - 流式透传：不缓存的响应（问答的 SSE 等）原样流式转发，不攒包。
//! - 清缓存：发布或改站点设置后，业务侧用共享密钥调 `POST /_edge/purge?host=…`。
//! 不做 TLS（证书由前面的 Caddy 按需签发）。

mod cache;
mod config;
mod limiter;
mod proxy;

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
    eprintln!("nw-edge 已启动 {} → {}", state.config.listen, state.config.upstream);
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
