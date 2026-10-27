use std::collections::HashMap;
use std::convert::Infallible;
use std::net::{IpAddr, SocketAddr};
use std::pin::Pin;
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll};
use std::time::Duration;

use bytes::Bytes;
use http_body_util::{combinators::BoxBody, BodyExt, Full};
use hyper::body::{Body as HttpBody, Frame, Incoming};
use hyper::header::{self, HeaderMap, HeaderName, HeaderValue};
use hyper::{Method, Request, Response, StatusCode};
use hyper_util::client::legacy::connect::HttpConnector;
use hyper_util::client::legacy::Client;
use hyper_util::rt::TokioExecutor;
use tokio::sync::Notify;

use crate::cache::{Cache, Entry};
use crate::config::Config;
use crate::disk::DiskCache;
use crate::limiter::Limiter;

type Body = BoxBody<Bytes, hyper::Error>;

pub struct State {
    pub config: Config,
    cache: Cache,
    disk: Option<Arc<DiskCache>>,
    limiter: Limiter,
    client: Client<HttpConnector, Incoming>,
    inflight: Mutex<HashMap<String, Arc<Notify>>>,
    purge_secret: Option<Vec<u8>>,
}

impl State {
    pub fn new(config: Config) -> anyhow::Result<Self> {
        let purge_secret = match &config.purge_secret_file {
            Some(path) => Some(std::fs::read_to_string(path)?.trim().as_bytes().to_vec()),
            None => None,
        };
        let mut connector = HttpConnector::new();
        connector.set_nodelay(true);
        connector.set_connect_timeout(Some(Duration::from_secs(3)));
        let client = Client::builder(TokioExecutor::new()).pool_idle_timeout(Duration::from_secs(30)).build(connector);
        let disk = match &config.media_disk_dir {
            Some(dir) => Some(Arc::new(DiskCache::open(dir, config.media_disk_bytes)?)),
            None => None,
        };
        Ok(Self {
            cache: Cache::new(config.cache_bytes),
            disk,
            limiter: Limiter::new(config.read_rate.clone(), config.write_rate.clone()),
            client,
            inflight: Mutex::new(HashMap::new()),
            purge_secret,
            config,
        })
    }
}

fn text(status: StatusCode, message: &'static str) -> Response<Body> {
    let mut response = Response::new(Full::new(Bytes::from_static(message.as_bytes())).map_err(|never| match never {}).boxed());
    *response.status_mut() = status;
    response.headers_mut().insert(header::CONTENT_TYPE, HeaderValue::from_static("text/plain; charset=utf-8"));
    response
}

pub async fn handle(state: Arc<State>, peer: SocketAddr, request: Request<Incoming>) -> Result<Response<Body>, Infallible> {
    Ok(match route(&state, peer, request).await {
        Ok(response) => response,
        Err(error) => {
            eprintln!("转发失败：{error}");
            text(StatusCode::BAD_GATEWAY, "上游暂不可用")
        }
    })
}

const HOP_BY_HOP: [&str; 8] = ["connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailers", "transfer-encoding", "upgrade"];

fn copy_headers(from: &HeaderMap, to: &mut HeaderMap) {
    for (name, value) in from {
        if !HOP_BY_HOP.contains(&name.as_str()) {
            to.append(name.clone(), value.clone());
        }
    }
}

fn client_ip(state: &State, peer: SocketAddr, headers: &HeaderMap) -> IpAddr {
    if state.config.trust_forwarded {
        if let Some(first) = headers.get("x-forwarded-for").and_then(|v| v.to_str().ok()).and_then(|v| v.split(',').next()).and_then(|v| v.trim().parse::<IpAddr>().ok()) {
            return first;
        }
    }
    peer.ip()
}

fn host_of(headers: &HeaderMap) -> String {
    let host = headers.get(header::HOST).and_then(|v| v.to_str().ok()).unwrap_or_default().to_ascii_lowercase();
    host.rsplit_once(':').filter(|(_, port)| port.bytes().all(|b| b.is_ascii_digit())).map(|(h, _)| h.to_owned()).unwrap_or(host)
}

fn has_session(headers: &HeaderMap) -> bool {
    headers.get_all(header::COOKIE).iter().any(|v| v.to_str().map(|c| c.contains("nw_sid=")).unwrap_or(true))
}

/// 公开页和不可变资源可以缓存；接口、后台、证书询问不缓存（媒体字节带内容哈希，例外）。
fn cacheable_path(path: &str, query: Option<&str>) -> bool {
    if let Some(rest) = path.strip_prefix("/v1/assets/") {
        return rest.ends_with("/bytes") && query.map(|q| q.contains("sha=")).unwrap_or(false);
    }
    !(path.starts_with("/v1/") || path.starts_with("/admin") || path.starts_with("/_edge") || path.starts_with("/_tls") || path.starts_with("/_report"))
}

/// 带内容哈希的媒体字节（cacheable_path 已核对 sha 参数）。
fn is_media(path: &str) -> bool {
    path.strip_prefix("/v1/assets/").is_some_and(|rest| rest.ends_with("/bytes"))
}

async fn route(state: &Arc<State>, peer: SocketAddr, request: Request<Incoming>) -> anyhow::Result<Response<Body>> {
    let path = request.uri().path().to_owned();
    if path == "/_edge/healthz" {
        return Ok(text(StatusCode::OK, "ok"));
    }
    if path == "/_edge/purge" {
        return Ok(purge(state, &request));
    }
    if path.starts_with("/_edge/") {
        return Ok(text(StatusCode::NOT_FOUND, "not found"));
    }
    let ip = client_ip(state, peer, request.headers());
    let write = !matches!(*request.method(), Method::GET | Method::HEAD | Method::OPTIONS);
    if let Err(wait) = state.limiter.take(ip, write) {
        let mut response = text(StatusCode::TOO_MANY_REQUESTS, "请求太频繁，请稍后再试");
        response.headers_mut().insert(header::RETRY_AFTER, HeaderValue::from_str(&wait.to_string())?);
        return Ok(response);
    }
    let method = request.method().clone();
    let query = request.uri().query().map(str::to_owned);
    let host = host_of(request.headers());
    let cache_it = matches!(method, Method::GET | Method::HEAD) && cacheable_path(&path, query.as_deref()) && !has_session(request.headers());
    if !cache_it {
        return Ok(passthrough(forward(state, peer, request).await?));
    }
    let language = request.headers().get(header::ACCEPT_LANGUAGE).and_then(|v| v.to_str().ok()).unwrap_or_default().to_owned();
    let cookie = request.headers().get(header::COOKIE).and_then(|v| v.to_str().ok()).unwrap_or_default().to_owned();
    if cookie.len() > 200 {
        return Ok(passthrough(forward(state, peer, request).await?));
    }
    let key = format!("{host}|{path}|{}|{language}|{cookie}", query.as_deref().unwrap_or_default());
    if let Some(entry) = state.cache.get(&key) {
        return Ok(from_entry(entry, &method, "HIT"));
    }
    let media = is_media(&path);
    if let (true, Some(disk)) = (media, state.disk.clone()) {
        let lookup = key.clone();
        if let Some(entry) = tokio::task::spawn_blocking(move || disk.get(&lookup)).await? {
            if entry.body.len() <= state.config.max_object_bytes {
                state.cache.put(key, entry.clone(), Duration::from_secs(state.config.asset_ttl_seconds));
            }
            return Ok(from_entry(entry, &method, "HIT-DISK"));
        }
    }
    // 同一页面的并发未命中只放一个去上游，其余等它填好缓存。
    let notify = {
        let mut inflight = state.inflight.lock().unwrap();
        match inflight.get(&key) {
            Some(existing) => Some(existing.clone()),
            None => {
                inflight.insert(key.clone(), Arc::new(Notify::new()));
                None
            }
        }
    };
    if let Some(notify) = notify {
        let _ = tokio::time::timeout(Duration::from_secs(10), notify.notified()).await;
        if let Some(entry) = state.cache.get(&key) {
            return Ok(from_entry(entry, &method, "HIT"));
        }
        return Ok(passthrough(forward(state, peer, request).await?));
    }
    let _flight = Flight { state: state.clone(), key: key.clone() };
    let response = forward(state, peer, request).await?;
    let (mut parts, body) = response.into_parts();
    if let Some(ttl) = ttl_for(state, &path, &parts.status, &parts.headers) {
        let max = if media && state.disk.is_some() { state.config.media_max_object_bytes } else { state.config.max_object_bytes };
        let length = parts.headers.get(header::CONTENT_LENGTH).and_then(|v| v.to_str().ok()).and_then(|v| v.parse::<usize>().ok());
        // 上游（portal）的页面是分块传输、没有 Content-Length：先攒到上限，攒得下就缓存，攒不下就把已读的部分接上剩余的继续流式转发。
        let gathered = match length {
            Some(n) if n > max => Gathered::Skip(body.boxed()),
            _ => gather(body, max).await?,
        };
        match gathered {
            Gathered::Full(collected) => {
                let mut headers = HeaderMap::new();
                copy_headers(&parts.headers, &mut headers);
                headers.remove(header::CONTENT_LENGTH);
                headers.remove(header::TRANSFER_ENCODING);
                let site = headers.remove("x-nw-site").and_then(|v| v.to_str().ok().map(str::to_owned)).unwrap_or_default();
                let entry = Entry { status: parts.status, headers: headers.into_iter().filter_map(|(name, value)| name.map(|n| (n, value))).collect(), body: collected, expires: std::time::Instant::now(), host, site };
                if let (true, Some(disk)) = (media, state.disk.clone()) {
                    let (stored_key, stored, seconds) = (key.clone(), entry.clone(), ttl.as_secs());
                    let _ = tokio::task::spawn_blocking(move || disk.put(&stored_key, &stored, seconds)).await;
                }
                if entry.body.len() <= state.config.max_object_bytes {
                    state.cache.put(key, entry.clone(), ttl);
                }
                return Ok(from_entry(entry, &method, "MISS"));
            }
            Gathered::Skip(rest) => {
                parts.headers.remove("x-nw-site");
                return Ok(Response::from_parts(parts, rest));
            }
        }
    }
    parts.headers.remove("x-nw-site");
    Ok(Response::from_parts(parts, body.boxed()))
}

enum Gathered {
    Full(Bytes),
    Skip(Body),
}

/// 把响应体最多攒到 max 字节；攒不下时返回“已读部分 + 剩余”的流，调用方原样转发。
async fn gather(mut body: Incoming, max: usize) -> anyhow::Result<Gathered> {
    let mut buffer = bytes::BytesMut::new();
    loop {
        match body.frame().await {
            None => return Ok(Gathered::Full(buffer.freeze())),
            Some(Err(error)) => return Err(anyhow::anyhow!("读取上游响应失败：{error}")),
            Some(Ok(frame)) => {
                let Ok(data) = frame.into_data() else { continue };
                if buffer.len() + data.len() > max {
                    buffer.extend_from_slice(&data);
                    return Ok(Gathered::Skip(Replay { prefix: Some(buffer.freeze()), rest: body }.boxed()));
                }
                buffer.extend_from_slice(&data);
            }
        }
    }
}

/// 先吐出已经读出的前缀，再接着读剩余的响应体。
struct Replay {
    prefix: Option<Bytes>,
    rest: Incoming,
}

impl HttpBody for Replay {
    type Data = Bytes;
    type Error = hyper::Error;

    fn poll_frame(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Option<Result<Frame<Bytes>, hyper::Error>>> {
        if let Some(prefix) = self.prefix.take() {
            return Poll::Ready(Some(Ok(Frame::data(prefix))));
        }
        Pin::new(&mut self.rest).poll_frame(cx)
    }
}
/// 不缓存的响应原样透传（流式），只去掉入口内部用的站点标记头。
fn passthrough(response: Response<Incoming>) -> Response<Body> {
    let (mut parts, body) = response.into_parts();
    parts.headers.remove("x-nw-site");
    Response::from_parts(parts, body.boxed())
}

/// 离开作用域时（无论成功失败）通知等待者。
struct Flight {
    state: Arc<State>,
    key: String,
}

impl Drop for Flight {
    fn drop(&mut self) {
        if let Some(notify) = self.state.inflight.lock().unwrap().remove(&self.key) {
            notify.notify_waiters();
        }
    }
}

fn ttl_for(state: &State, path: &str, status: &StatusCode, headers: &HeaderMap) -> Option<Duration> {
    if *status != StatusCode::OK || headers.contains_key(header::SET_COOKIE) {
        return None;
    }
    // 事件流要边到边转发，不攒包也不缓存。
    if headers.get(header::CONTENT_TYPE).and_then(|v| v.to_str().ok()).is_some_and(|v| v.starts_with("text/event-stream")) {
        return None;
    }
    let control = headers.get(header::CACHE_CONTROL).and_then(|v| v.to_str().ok()).unwrap_or_default().to_ascii_lowercase();
    if control.contains("no-store") || control.contains("private") {
        return None;
    }
    let immutable = control.contains("immutable") || path.starts_with("/v1/assets/");
    let max_age = control.split(',').filter_map(|part| part.trim().strip_prefix("max-age=")).filter_map(|v| v.parse::<u64>().ok()).next();
    let seconds = match (immutable, max_age) {
        (true, Some(age)) => age.min(state.config.asset_ttl_seconds),
        (true, None) => state.config.asset_ttl_seconds,
        (false, Some(age)) if age > state.config.html_ttl_seconds => age.min(state.config.asset_ttl_seconds),
        _ => state.config.html_ttl_seconds,
    };
    (seconds > 0).then(|| Duration::from_secs(seconds))
}

fn from_entry(entry: Entry, method: &Method, label: &'static str) -> Response<Body> {
    let body = if *method == Method::HEAD { Bytes::new() } else { entry.body };
    let mut response = Response::new(Full::new(body).map_err(|never| match never {}).boxed());
    *response.status_mut() = entry.status;
    for (name, value) in entry.headers {
        response.headers_mut().append(name, value);
    }
    response.headers_mut().insert(HeaderName::from_static("x-nw-cache"), HeaderValue::from_static(label));
    response
}

async fn forward(state: &State, peer: SocketAddr, request: Request<Incoming>) -> anyhow::Result<Response<Incoming>> {
    let (mut parts, body) = request.into_parts();
    let target = parts.uri.path_and_query().map(|p| p.as_str()).unwrap_or("/");
    let uri: hyper::Uri = format!("http://{}{}", state.config.upstream, target).parse()?;
    let mut headers = HeaderMap::new();
    copy_headers(&parts.headers, &mut headers);
    let forwarded = match parts.headers.get("x-forwarded-for").and_then(|v| v.to_str().ok()) {
        Some(previous) if state.config.trust_forwarded => previous.to_owned(),
        _ => peer.ip().to_string(),
    };
    headers.insert(HeaderName::from_static("x-forwarded-for"), HeaderValue::from_str(&forwarded)?);
    parts.headers = headers;
    parts.uri = uri;
    let upstream = Request::from_parts(parts, body);
    match tokio::time::timeout(Duration::from_secs(300), state.client.request(upstream)).await {
        Ok(Ok(response)) => {
            let (mut parts, body) = response.into_parts();
            let mut headers = HeaderMap::new();
            copy_headers(&parts.headers, &mut headers);
            parts.headers = headers;
            Ok(Response::from_parts(parts, body))
        }
        Ok(Err(error)) => Err(anyhow::anyhow!("上游请求失败：{error}")),
        Err(_) => Err(anyhow::anyhow!("上游响应超时")),
    }
}

fn purge(state: &State, request: &Request<Incoming>) -> Response<Body> {
    let Some(secret) = &state.purge_secret else {
        return text(StatusCode::NOT_FOUND, "not found");
    };
    if request.method() != Method::POST {
        return text(StatusCode::METHOD_NOT_ALLOWED, "method not allowed");
    }
    let supplied = request.headers().get("x-nw-purge").and_then(|v| v.to_str().ok()).unwrap_or_default().as_bytes();
    if supplied.len() != secret.len() || supplied.iter().zip(secret.iter()).fold(0u8, |acc, (a, b)| acc | (a ^ b)) != 0 {
        return text(StatusCode::UNAUTHORIZED, "unauthorized");
    }
    let query = request.uri().query().unwrap_or_default();
    let param = |name: &str| query.split('&').find_map(|pair| pair.strip_prefix(name)).unwrap_or_default().to_ascii_lowercase();
    let (host, site) = (param("host="), param("site="));
    let count = if !site.is_empty() {
        state.cache.purge_site(&site) + state.disk.as_ref().map_or(0, |disk| disk.purge_site(&site))
    } else if !host.is_empty() {
        state.cache.purge_host(&host) + state.disk.as_ref().map_or(0, |disk| disk.purge_host(&host))
    } else {
        return text(StatusCode::BAD_REQUEST, "missing host or site");
    };
    let (entries, bytes) = state.cache.stats();
    let body = format!("{{\"purged\":{count},\"entries\":{entries},\"bytes\":{bytes}}}");
    let mut response = Response::new(Full::new(Bytes::from(body)).map_err(|never| match never {}).boxed());
    response.headers_mut().insert(header::CONTENT_TYPE, HeaderValue::from_static("application/json"));
    response
}
