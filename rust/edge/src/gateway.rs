//! 入口代理（原 Go 的 portal 组件）：按路由表找到拥有服务，做入口预检后带 portal 身份的服务断言转发。
//!
//! 预检顺序：路由 → 请求包络（大小、媒体类型）→ 同源 → 写请求的 CSRF（identity 核对）→ 会话刷新改写正文 →
//! 登录的网络作用域 → 会话授权（identity 核权）并签 ≤10 秒的用户委托 → 风控 / 访客问答 / 统计信标的专门处理 → 转发。
//! 拥有服务仍在服务端二次核权；这里只是入口把关。

use std::net::{IpAddr, SocketAddr};
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::Duration;

use bytes::Bytes;
use http_body_util::{combinators::BoxBody, BodyExt, Full};
use hyper::body::Incoming;
use hyper::header::{self, HeaderMap, HeaderName, HeaderValue};
use hyper::{Method, Request, Response, StatusCode};

use crate::beacons::{self, Beacons};
use crate::net::{budget_scopes, client_address, login_scopes, Cidr};
use crate::platform::{self, canonical, new_id, now_plus, parse_time, sha256_hex, sign_delegation, timestamp, valid_identifier, AuthContext, Key, Problem, ServiceConfig, TrustAnchors};
use crate::routes::{Matched, Registry};
use crate::site::{request_scheme, HostCache};
use crate::upstream::{read_limited, Call, Operation, Upstream};

pub type Body = BoxBody<Bytes, hyper::Error>;

const MAX_BODY: usize = 16 << 20;

pub struct Gateway {
    anchors: TrustAnchors,
    pub upstream: Upstream,
    delegation: Key,
    budget: Option<Key>,
    registry: Registry,
    countries: Option<maxminddb::Reader<Vec<u8>>>,
    web_dir: Option<PathBuf>,
    trusted: Vec<Cidr>,
    pub hosts: HostCache,
    default_site: Mutex<String>,
    /// 部署时配置的公开地址（scheme://host[:port]）与它的主机名。
    public_origin: Option<String>,
    pub bootstrap_host: Option<String>,
    pub beacons: Beacons,
}

pub struct Paths<'a> {
    pub service: &'a str,
    pub routes: &'a str,
    pub web_dir: Option<&'a str>,
}

impl Gateway {
    pub fn new(paths: Paths) -> anyhow::Result<Self> {
        let config = ServiceConfig::load(paths.service, "portal")?;
        let anchors = TrustAnchors::load(&config.trust_anchor_ref)?;
        let key = Key::load(&config.signing_key_ref)?;
        anyhow::ensure!(anchors.service("portal", &key.id) == Some(key.verifying()), "服务签名身份与信任锚不一致");
        let delegation = Key::load(&config.delegation_key_ref)?;
        platform::check_delegation_key(&anchors, &delegation)?;
        let budget = if config.budget_key_ref.is_empty() { None } else { Some(Key::load(&config.budget_key_ref)?) };
        let countries = if config.geoip_country_db_ref.is_empty() { None } else { Some(maxminddb::Reader::open_readfile(&config.geoip_country_db_ref)?) };
        let trusted = config.trusted_proxy_cidrs.iter().map(|c| Cidr::parse(c)).collect::<anyhow::Result<_>>()?;
        let web_dir = paths.web_dir.map(PathBuf::from).filter(|dir| dir.join("index.html").is_file());
        let (public_origin, bootstrap_host) = match config.public_base_url.split_once("://") {
            Some((scheme, rest)) => {
                let authority = rest.split('/').next().unwrap_or_default().to_ascii_lowercase();
                let host = authority.rsplit_once(':').map(|(h, _)| h.to_owned()).unwrap_or(authority.clone());
                (Some(format!("{scheme}://{authority}")), Some(host))
            }
            None => (None, None),
        };
        Ok(Self {
            upstream: Upstream::new("portal", key, config.dependencies.clone()),
            registry: Registry::load(paths.routes)?,
            anchors, delegation, budget, countries, web_dir, trusted,
            hosts: HostCache::default(),
            default_site: Mutex::new(String::new()),
            public_origin, bootstrap_host,
            beacons: Beacons::default(),
        })
    }

    /// 当前数据代次（site-config 的运行状态，缓存 5 秒）；读不到时为空，下游按自己的运行状态处理。
    pub async fn data_epoch(&self) -> String {
        self.upstream.data_epoch().await.unwrap_or_default()
    }

    /// 平台默认站点（单站点用法、平台域名）：初始化完成前为空，读到后不再变。
    async fn default_site(&self) -> String {
        let current = self.default_site.lock().unwrap().clone();
        if !current.is_empty() {
            return current;
        }
        let op = Operation::new("system", &self.data_epoch().await);
        let site = match self.upstream.json("site-config", Method::GET, "/v1/config", None, &op).await {
            Ok(body) => body["config"]["siteId"].as_str().unwrap_or_default().to_owned(),
            Err(_) => String::new(),
        };
        if !site.is_empty() {
            *self.default_site.lock().unwrap() = site.clone();
        }
        site
    }

    fn country(&self, addr: IpAddr) -> Option<String> {
        let reader = self.countries.as_ref()?;
        let record: maxminddb::geoip2::Country = reader.lookup(addr).ok()?;
        record.country?.iso_code.map(str::to_owned)
    }
}

// ---------- 响应小工具 ----------

pub fn full(status: StatusCode, content_type: &'static str, body: impl Into<Bytes>) -> Response<Body> {
    let mut response = Response::new(Full::new(body.into()).map_err(|never| match never {}).boxed());
    *response.status_mut() = status;
    if !content_type.is_empty() {
        response.headers_mut().insert(header::CONTENT_TYPE, HeaderValue::from_static(content_type));
    }
    response
}

pub fn text(status: StatusCode, message: &'static str) -> Response<Body> {
    full(status, "text/plain; charset=utf-8", message)
}

pub fn html(status: StatusCode, page: String) -> Response<Body> {
    full(status, "text/html; charset=utf-8", page)
}

pub fn redirect(status: StatusCode, location: &str) -> Response<Body> {
    let mut response = full(status, "", Bytes::new());
    if let Ok(value) = HeaderValue::from_str(location) {
        response.headers_mut().insert(header::LOCATION, value);
    }
    response
}

fn problem_response(problem: &Problem, request_id: &str) -> Response<Body> {
    if problem.status() >= 500 {
        eprintln!("问题响应 status={} code={} detail={} requestId={request_id}", problem.status(), problem.code, problem.detail);
    }
    let mut response = full(StatusCode::from_u16(problem.status()).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR), "application/problem+json; charset=utf-8", problem.body(request_id));
    response.headers_mut().insert(header::X_CONTENT_TYPE_OPTIONS, HeaderValue::from_static("nosniff"));
    response
}

pub fn cookie(headers: &HeaderMap, name: &str) -> Option<String> {
    cookies(headers, name).into_iter().next()
}

fn cookies(headers: &HeaderMap, name: &str) -> Vec<String> {
    headers
        .get_all(header::COOKIE)
        .iter()
        .filter_map(|v| v.to_str().ok())
        .flat_map(|v| v.split(';'))
        .filter_map(|pair| pair.trim().split_once('='))
        .filter(|(k, _)| *k == name)
        .map(|(_, v)| v.trim_matches('"').to_owned())
        .collect()
}

fn header_str<'a>(headers: &'a HeaderMap, name: &str) -> &'a str {
    headers.get(name).and_then(|v| v.to_str().ok()).unwrap_or_default()
}

pub fn http_date(at: time::OffsetDateTime) -> String {
    let format = time::format_description::parse_borrowed::<2>("[weekday repr:short], [day] [month repr:short] [year] [hour]:[minute]:[second] GMT").expect("固定格式");
    at.to_offset(time::UtcOffset::UTC).format(&format).expect("UTC 时间总能格式化")
}

// ---------- 入口 ----------

impl Gateway {
    /// 一次入口请求；purge 在改变公开页内容的写入成功后被调用（参数是站点）。
    pub async fn handle(&self, peer: SocketAddr, request: Request<Incoming>, purge: impl Fn(&str)) -> Response<Body> {
        let (parts, body) = request.into_parts();
        let path = parts.uri.path().to_owned();
        let query = parts.uri.query().unwrap_or_default().to_owned();
        let read = matches!(parts.method, Method::GET | Method::HEAD);
        // 本地处理、不经站点门的路径（与原 portal 的固定路由一致）。
        match path.as_str() {
            "/healthz" | "/readyz" if read => return full(StatusCode::OK, "application/json", r#"{"module":"edge","status":200}"#),
            "/_report" if read => {
                let mut response = full(StatusCode::OK, "text/html; charset=utf-8", include_str!("report.html"));
                response.headers_mut().insert(header::CACHE_CONTROL, HeaderValue::from_static("no-cache"));
                response.headers_mut().insert(header::CONTENT_SECURITY_POLICY, HeaderValue::from_static("default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'"));
                return response;
            }
            "/_tls/allow" if read => return self.tls_allow(&query).await,
            _ => {}
        }
        if path.starts_with("/_pv/") {
            let body = read_limited(body, 4096).await.unwrap_or_default();
            return self.private_entry(&parts.method, &path, &query, &parts.headers, &body).await;
        }
        if read && (path == "/admin" || path.starts_with("/admin/") || path.starts_with("/_admin/") || path.starts_with("/_site/")) {
            if let Some(response) = self.static_web(&path).await {
                return response;
            }
        }
        let request_id = Some(header_str(&parts.headers, "x-request-id")).filter(|v| valid_identifier(v)).map(str::to_owned).unwrap_or_else(new_id);
        if query.len() > 8192 {
            return problem_response(&Problem::new("BUDGET_EXCEEDED", "查询参数超出上限"), &request_id);
        }
        let body = match read_limited(body, MAX_BODY + 1).await {
            Ok(bytes) if bytes.len() <= MAX_BODY => bytes,
            _ => return problem_response(&Problem::new("BUDGET_EXCEEDED", "请求体超出入口上限"), &request_id),
        };
        let host_site = self.host_site(&parts.headers).await;
        let uri = parts.uri.path_and_query().map(|p| p.as_str()).unwrap_or("/").to_owned();
        if let Some(site) = &host_site {
            if let Some(response) = self.site_gate(site, &parts.method, &path, &uri, &parts.headers) {
                return response;
            }
        }
        let mut site = match &host_site {
            Some(s) => s.site_id.clone(),
            None => self.default_site().await,
        };
        // 站点还没初始化（读不到站点身份）：打开根路径直接去后台的初始化向导。
        if site.is_empty() && parts.method == Method::GET && path == "/" {
            let mut response = redirect(StatusCode::FOUND, "/admin");
            response.headers_mut().insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));
            return response;
        }
        if site.is_empty() {
            site = "system".into();
        }
        let mut origins: Vec<String> = self.public_origin.iter().cloned().collect();
        // 已登记的站点域名：同域名发来的请求（访客提问、站点后台）本身就是同源。
        if host_site.is_some() {
            let host = header_str(&parts.headers, "host");
            if !host.is_empty() {
                origins.push(format!("https://{host}"));
                origins.push(format!("http://{host}"));
            }
        }
        let host_site_id = host_site.as_ref().filter(|s| !s.primary_host.is_empty()).map(|s| s.site_id.clone()).unwrap_or_default();
        let public_origin = host_site.as_ref().filter(|s| !s.primary_host.is_empty()).map(|s| format!("{}://{}", request_scheme(&parts.headers), s.primary_host)).unwrap_or_default();
        let incoming = Inbound { peer, method: parts.method.clone(), path, query, headers: parts.headers, body, site: site.clone(), host_site_id, public_origin, origins };
        let mut extra = HeaderMap::new();
        // 公开页响应带上站点标记，供本进程的缓存按站点清除（缓存层不会把它交给浏览器）。
        if incoming.method == Method::GET && site != "system" {
            if let Ok(value) = HeaderValue::from_str(&site) {
                extra.insert(HeaderName::from_static("x-nw-site"), value);
            }
        }
        let epoch = self.data_epoch().await;
        let mut op = Operation::new(&site, &epoch);
        op.request_id = request_id.clone();
        if let Some(id) = Some(header_str(&incoming.headers, "x-operation-id")).filter(|v| valid_identifier(v)) {
            op.operation_id = id.to_owned();
            op.root_operation_id = id.to_owned();
        }
        let result = self.proxy(&incoming, op, &mut extra).await;
        let mut response = match result {
            Ok((response, owner, operation)) => {
                let ok = response.status().as_u16() < 300;
                if !read && ok && PURGE_OPERATIONS.contains(&format!("{owner}:{operation}").as_str()) {
                    let target = Some(header_str(&incoming.headers, "x-nw-site")).filter(|v| valid_identifier(v)).unwrap_or(&incoming.host_site_id).to_ascii_lowercase();
                    if !target.is_empty() {
                        purge(&target);
                    }
                }
                if !read && ok && owner == "identity" {
                    self.hosts.forget();
                }
                response
            }
            Err(problem) => problem_response(&problem, &request_id),
        };
        for (name, value) in extra.iter() {
            if name == header::SET_COOKIE {
                response.headers_mut().append(name, value.clone());
            } else {
                response.headers_mut().insert(name, value.clone());
            }
        }
        response
    }
}

struct Inbound {
    peer: SocketAddr,
    method: Method,
    path: String,
    query: String,
    headers: HeaderMap,
    body: Bytes,
    site: String,
    host_site_id: String,
    /// 站点域名上的请求：站点的公开地址（主域名），公开页的规范链接、站点地图按它生成。
    public_origin: String,
    origins: Vec<String>,
}

impl Inbound {
    fn client(&self, trusted: &[Cidr]) -> Result<IpAddr, Problem> {
        let forwarded: Vec<&str> = self.headers.get_all("x-forwarded-for").iter().filter_map(|v| v.to_str().ok()).collect();
        client_address(self.peer, &forwarded.join(","), trusted)
    }
}

/// 会改变公开页内容或访问规则的写入操作：成功后清掉这个站点的页面缓存。
const PURGE_OPERATIONS: [&str; 27] = [
    "content:publishDocument", "content:withdrawDocument", "content:rollbackPublishedRevision", "content:updateDocumentMetadata",
    "content:trashDocument", "content:restoreDocument", "content:moveDocument", "content:reorderChildren", "content:purgeDocuments",
    "site-config:updateBanner", "site-config:updateBranding", "site-config:updateSiteConfig", "site-config:restoreConfigRevision",
    "site-config:updateFooter", "site-config:updateHome", "site-config:updateAppearance",
    "localization:putLanguage", "localization:setLanguageEnabled", "localization:approveLanguagePlan", "localization:putInterfaceString",
    "identity:updateSite", "identity:setPrimaryDomain", "identity:removeSiteDomain", "identity:setSiteStatus",
    "identity:setSitePassword", "identity:deleteSite", "identity:restoreSite",
];

/// 后台触发、会调用模型的路由及其费用类别：只由入口按路由写入，浏览器不能指定。
fn admin_cost_class(action: &str) -> &'static str {
    match action {
        "knowledge:createBuild" | "knowledge:approveRebuildPlan" | "localization:createTranslation" | "localization:requestInterfaceTranslation" | "qa:createDebugTurn" => "background",
        "model-gateway:createConnectionProbe" => "admin-probe",
        _ => "",
    }
}

/// 匿名路由里允许对已登录管理员附带委托的操作（含停用语言的列表；编辑器里刚上传、草稿还没保存的图片预览；后台导出所选站点的外观包）。
const OPTIONAL_ADMIN: [&str; 4] = ["localization:listLanguages", "media:readAssetBytes", "media:headAssetBytes", "site-config:readThemePackage"];

/// 浏览器请求里允许原样转发的业务头；身份、转发地址、平台可信头一律不转发，可信头由入口重新写入。
const FORWARDABLE: [&str; 14] = [
    "content-type", "accept-language", "if-match", "if-none-match", "if-modified-since", "if-unmodified-since", "if-range", "range",
    "last-event-id", "idempotency-key", "x-upload-offset", "x-chunk-sha256", "x-backup-download-token", "x-backup-passphrase",
];

const RESPONSE_HEADERS: [&str; 11] = ["content-type", "content-length", "content-range", "accept-ranges", "etag", "last-modified", "cache-control", "location", "retry-after", "www-authenticate", "x-request-id"];

fn authentication_operation(m: &Matched) -> &'static str {
    if m.route.owner != "identity" || m.route.method != "POST" {
        return "";
    }
    match m.route.operation.as_str() {
        "createSession" => "createSession",
        "registerFirstSuperadmin" => "registerFirstSuperadmin",
        "registerUser" => "registerUser",
        "oauthLogin" => "oauthLogin",
        "refreshSession" => "refreshSession",
        "changePassword" => "changePassword",
        _ => "",
    }
}

fn supported_media_type(raw: &str) -> bool {
    let media = raw.split(';').next().unwrap_or_default().trim().to_ascii_lowercase();
    matches!(media.as_str(), "application/json" | "application/problem+json" | "application/octet-stream" | "text/plain" | "text/event-stream" | "")
}

/// 规范化 Origin：scheme://host:port（缺省端口补齐）。
fn normalize_origin(raw: &str) -> Option<String> {
    let (scheme, rest) = raw.split_once("://")?;
    let scheme = scheme.to_ascii_lowercase();
    if (scheme != "http" && scheme != "https") || rest.is_empty() || rest.contains(['/', '?', '#', '@', '\\']) {
        return None;
    }
    let rest = rest.to_ascii_lowercase();
    let (host, port) = if let Some(v6) = rest.strip_prefix('[') {
        let (host, tail) = v6.split_once(']')?;
        (format!("[{host}]"), tail.strip_prefix(':').map(str::to_owned))
    } else {
        match rest.rsplit_once(':') {
            Some((h, p)) => (h.to_owned(), Some(p.to_owned())),
            None => (rest.clone(), None),
        }
    };
    let port: u16 = match port {
        Some(p) => p.parse().ok().filter(|p| *p > 0)?,
        None => if scheme == "http" { 80 } else { 443 },
    };
    (!host.is_empty()).then(|| format!("{scheme}://{host}:{port}"))
}

/// 写请求必须带完整且同源的 Origin：与配置的来源之一相同，并且就是这次请求的 Host。
fn same_origin(origin: &str, host: &str, configured: &[String]) -> bool {
    let Some(origin) = normalize_origin(origin) else { return false };
    let host = host.trim().to_ascii_lowercase();
    if host.is_empty() || host.contains(['/', '\\', '?', '#', '@']) {
        return false;
    }
    let scheme = origin.split("://").next().unwrap_or_default();
    let own = normalize_origin(&format!("{scheme}://{host}"));
    configured.iter().filter_map(|c| normalize_origin(c)).any(|c| c == origin) && own.as_deref() == Some(origin.as_str())
}

impl Gateway {
    async fn proxy(&self, req: &Inbound, mut op: Operation, extra: &mut HeaderMap) -> Result<(Response<Body>, String, String), Problem> {
        let matched = self.registry.resolve(req.method.as_str(), &req.path)?;
        let route = matched.route;
        let action = route.action();
        let public_ai = route.anonymous() && (route.owner == "qa" || route.owner == "traffic-guard");
        if public_ai {
            // 入口提前拒绝的响应同样不能被浏览器或中间缓存复用。
            extra.insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));
            extra.insert(HeaderName::from_static("cross-origin-resource-policy"), HeaderValue::from_static("same-origin"));
        }
        let authentication = authentication_operation(&matched);
        if !authentication.is_empty() || action == "identity:revokeSession" {
            extra.insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));
            extra.insert(header::PRAGMA, HeaderValue::from_static("no-cache"));
        }
        // 请求包络：头总大小、声明的正文长度、正文媒体类型、If-Match 版本。
        let header_bytes: usize = req.headers.iter().map(|(n, v)| n.as_str().len() + v.len()).sum();
        if header_bytes > 64 << 10 {
            return Err(Problem::new("BUDGET_EXCEEDED", "请求头超出上限"));
        }
        if !req.body.is_empty() && !supported_media_type(header_str(&req.headers, "content-type")) {
            return Err(Problem::new("VALIDATION", "请求媒体类型不受支持"));
        }
        if let Some(raw) = req.headers.get(header::IF_MATCH).and_then(|v| v.to_str().ok()) {
            if raw.trim_matches('"').parse::<u64>().is_err() {
                return Err(Problem::new("VALIDATION", "期望版本不合法"));
            }
        }
        let mut body = req.body.clone();
        let session = cookie(&req.headers, "nw_sid").filter(|v| !v.is_empty());
        let write = !matches!(req.method, Method::GET | Method::HEAD);
        let origin = header_str(&req.headers, "origin");
        // 公开问答及挑战接口只供本站页面使用；读取和事件流同样拒绝跨站浏览器上下文。
        if public_ai {
            let fetch_site = header_str(&req.headers, "sec-fetch-site");
            if !fetch_site.is_empty() && fetch_site != "same-origin" {
                return Err(Problem::new("FORBIDDEN", "问答接口只接受本站页面请求"));
            }
        }
        // 写请求必须同源；带会话的读取在浏览器同源 GET 不发 Origin 时放行，带了就必须同源（会话 cookie 为 SameSite=Strict）。
        if (write || ((session.is_some() || public_ai) && !origin.is_empty())) && !same_origin(origin, header_str(&req.headers, "host"), &req.origins) {
            return Err(Problem::new("FORBIDDEN", "请求来源不受信任"));
        }
        if let (Some(token), true, false) = (&session, write, route.anonymous()) {
            let supplied = header_str(&req.headers, "x-csrf-token");
            if supplied.is_empty() {
                return Err(Problem::new("UNAUTHENTICATED", "会话或CSRF凭据缺失"));
            }
            let verdict = self.upstream.json("identity", Method::POST, "/v1/csrf:verify", Some(serde_json::json!({"accessToken": token, "csrfToken": supplied, "origin": origin})), &op).await?;
            if verdict["valid"].as_bool() != Some(true) {
                return Err(Problem::new("UNAUTHENTICATED", "CSRF校验失败"));
            }
        }
        if authentication == "refreshSession" {
            // 刷新秘密只来自 HttpOnly cookie，浏览器正文必须是空对象。
            let json = header_str(&req.headers, "content-type").split(';').next().unwrap_or_default().trim().eq_ignore_ascii_case("application/json");
            let empty = serde_json::from_slice::<serde_json::Map<String, serde_json::Value>>(&body).is_ok_and(|m| m.is_empty());
            if !json || !empty {
                return Err(Problem::new("VALIDATION", "浏览器刷新请求必须为空对象"));
            }
            let refresh = cookies(&req.headers, "nw_refresh");
            if refresh.len() != 1 || refresh[0].len() < 32 || refresh[0].len() > 4096 {
                return Err(Problem::new("UNAUTHENTICATED", "会话刷新凭据缺失或不明确"));
            }
            body = Bytes::from(serde_json::json!({"refreshToken": refresh[0]}).to_string());
        }
        if matches!(authentication, "createSession" | "registerFirstSuperadmin" | "registerUser" | "oauthLogin") {
            op.login_scopes = login_scopes(&self.upstream.key, req.client(&self.trusted)?);
        }
        let mut proof = None;
        let mut current_session = String::new();
        let optional_admin = route.anonymous() && session.is_some() && OPTIONAL_ADMIN.contains(&action.as_str());
        if !route.anonymous() || optional_admin {
            let permission = if optional_admin { "portal.use" } else { route.permission.as_str() };
            let target = matched.parameter("siteId").filter(|v| valid_identifier(v)).or(Some(header_str(&req.headers, "x-nw-site")).filter(|v| valid_identifier(v))).unwrap_or(&req.host_site_id).to_owned();
            match self.authorize(session.as_deref(), permission, &target, &action, &op).await {
                Ok(auth) => {
                    current_session = auth.session_id.clone();
                    // 服务断言的站点必须与用户委托里的站点一致。
                    op.site_id = auth.site_id.clone();
                    op.cost_class = admin_cost_class(&action).to_owned();
                    proof = Some(self.delegate(&auth, &route.template, &body, &op)?);
                }
                Err(problem) if !optional_admin => return Err(problem),
                Err(_) => {}
            }
        }
        if route.owner == "traffic-guard" && route.anonymous() {
            let budget = self.budget.as_ref().ok_or_else(|| Problem::new("DEPENDENCY_UNAVAILABLE", "风控站点或预算密钥尚未装配"))?;
            op.budget_scopes = budget_scopes(budget, req.client(&self.trusted)?, &req.site);
            op.site_id = req.site.clone();
        }
        if route.owner == "qa" && route.anonymous() {
            let (rewritten, visitor_proof) = self.visitor_qa(req, &matched, body, &mut op).await?;
            body = rewritten;
            proof = Some(visitor_proof);
        } else if !route.admission.is_empty() {
            return Err(Problem::new("DEPENDENCY_UNAVAILABLE", "公开准入服务尚未装配"));
        }
        if action == "analytics:ingestBeacon" {
            let beacon = self.trusted_beacon(req, &body, &op, extra)?;
            self.beacons.push(&req.site, &op.data_epoch, beacon);
            return Ok((full(StatusCode::NO_CONTENT, "", Bytes::new()), route.owner.clone(), route.operation.clone()));
        }
        // 转发头：白名单内的业务头、语言切换器选的语言、语言建议用的访客国家。
        let mut headers = HeaderMap::new();
        for (name, value) in req.headers.iter() {
            if FORWARDABLE.contains(&name.as_str()) {
                if value.len() > 8192 {
                    return Err(Problem::new("VALIDATION", "请求头值不合法"));
                }
                headers.append(name.clone(), value.clone());
            }
        }
        if let Some(lang) = cookie(&req.headers, "lang").filter(|v| language_tag(v)) {
            headers.insert(HeaderName::from_static("x-language-preference"), HeaderValue::from_str(&lang).expect("语言标签只含 ASCII"));
        }
        if action == "localization:suggestLanguage" {
            if let Some(country) = req.client(&self.trusted).ok().and_then(|addr| self.country(addr)) {
                if let Ok(value) = HeaderValue::from_str(&country) {
                    headers.insert(HeaderName::from_static("x-visitor-country"), value);
                }
            }
        }
        if !req.public_origin.is_empty() {
            if let Ok(value) = HeaderValue::from_str(&req.public_origin) {
                headers.insert(HeaderName::from_static("x-nw-public-origin"), value);
            }
        }
        if let Some(accept) = req.headers.get(header::ACCEPT) {
            headers.insert(header::ACCEPT, accept.clone());
        }
        if !authentication.is_empty() {
            headers.insert(header::ACCEPT, HeaderValue::from_static("application/json"));
            headers.insert(header::CONTENT_TYPE, HeaderValue::from_static("application/json"));
        }
        if req.query.contains(['#', '\r', '\n']) {
            return Err(Problem::new("VALIDATION", "查询参数不合法"));
        }
        let target = if req.query.is_empty() { req.path.clone() } else { format!("{}?{}", req.path, req.query) };
        let timeout = Duration::from_millis(route.timeout_ms.max(100));
        let upstream = self.upstream.send(Call { owner: &route.owner, method: req.method.clone(), target: &target, body, headers, proof: proof.as_deref(), timeout }, &op).await?;
        let revocation = action == "identity:revokeSession" && !current_session.is_empty() && matched.parameter("sessionId") == Some(current_session.as_str());
        let response = if !authentication.is_empty() || revocation {
            adapt_authentication(upstream, authentication, revocation).await?
        } else {
            stream_response(upstream, req.method == Method::HEAD)
        };
        Ok((response, route.owner.clone(), route.operation.clone()))
    }

    /// 会话授权由 identity 核对 access token、当前版本与成员角色；入口再按路由权限预检一次。
    async fn authorize(&self, token: Option<&str>, permission: &str, site: &str, action: &str, op: &Operation) -> Result<AuthContext, Problem> {
        let token = token.ok_or_else(|| Problem::new("UNAUTHENTICATED", "会话或权限要求缺失"))?;
        let mut request = serde_json::json!({"accessToken": token, "permission": permission, "action": action});
        if !site.is_empty() {
            request["siteId"] = site.into();
        }
        let decision = self.upstream.json("identity", Method::POST, "/v1/authorizations/check", Some(request), op).await?;
        if decision["authorized"].as_bool() != Some(true) {
            return Err(Problem::new("FORBIDDEN", "当前账户无此操作权限"));
        }
        let auth: AuthContext = serde_json::from_value(decision["auth"].clone()).map_err(|_| Problem::new("DEPENDENCY_UNAVAILABLE", "身份授权响应不合法"))?;
        if auth.subject_id.is_empty() || auth.site_id.is_empty() || auth.expires_at.is_empty() {
            return Err(Problem::new("UNAUTHENTICATED", "身份上下文不完整"));
        }
        if permission == "service" {
            return Err(Problem::new("FORBIDDEN", "服务路由不接受浏览器主体"));
        }
        if !site.is_empty() && site != auth.site_id {
            return Err(Problem::new("FORBIDDEN", "站点作用域不匹配"));
        }
        if !auth.grants.iter().any(|g| g == permission || g == "superadmin") {
            return Err(Problem::new("FORBIDDEN", "缺少路由权限"));
        }
        Ok(auth)
    }

    /// 委托期限取 now+10 秒、操作期限、会话期限三者最早者。
    fn delegate(&self, auth: &AuthContext, route: &str, body: &[u8], op: &Operation) -> Result<String, Problem> {
        let mut expires = now_plus(10);
        for limit in [parse_time(&op.deadline), parse_time(&auth.expires_at)].into_iter().flatten() {
            expires = expires.min(limit);
        }
        if expires <= time::OffsetDateTime::now_utc() {
            return Err(Problem::new("UNAUTHENTICATED", "会话已过期"));
        }
        Ok(sign_delegation(&self.delegation, auth, route, &sha256_hex(body), &timestamp(expires)))
    }

    /// 公开问答的伪名访客身份：没过人机验证时按网络预算作用域推导主体，过了就用 traffic-guard 签的通过证明里的主体；
    /// 需要准入的写操作先向 traffic-guard 申请许可并写进请求体。
    async fn visitor_qa(&self, req: &Inbound, matched: &Matched<'_>, mut body: Bytes, op: &mut Operation) -> Result<(Bytes, String), Problem> {
        let budget = self.budget.as_ref().ok_or_else(|| Problem::new("DEPENDENCY_UNAVAILABLE", "问答访客身份的预算密钥尚未装配"))?;
        let client = req.client(&self.trusted)?;
        let scopes = budget_scopes(budget, client, &req.site);
        op.site_id = req.site.clone();
        let mut subject = budget.private_digest("guard.subject.v1", &canonical(&serde_json::json!({"siteId": req.site, "sessionId": format!("anonymous:{}", scopes[0])})));
        op.budget_scopes = scopes;
        let mut token = header_str(&req.headers, "x-guard-session").to_owned();
        match platform::verify_guard_session(&self.anchors, &token) {
            Some(claims) if claims.site_id == req.site => subject = claims.subject_id,
            _ => token.clear(),
        }
        // 同一访客对同一接口用同一幂等键重试时根操作不变：风控复用原准入，问答按回执返回原结果。
        let key = header_str(&req.headers, "idempotency-key");
        if valid_identifier(key) {
            op.root_operation_id = budget.private_digest("portal.visitor-qa.root.v1", &canonical(&serde_json::json!({"siteId": req.site, "subject": subject, "route": matched.route.action(), "key": key})));
        }
        if !matched.route.admission.is_empty() {
            let permit = self.admission(op, &matched.route.admission, &token, client).await?;
            subject = permit["subjectId"].as_str().unwrap_or_default().to_owned();
            let mut fields: serde_json::Map<String, serde_json::Value> = serde_json::from_slice(&body).map_err(|_| Problem::new("VALIDATION", "问答请求正文不可读"))?;
            fields.insert("admission".into(), permit);
            body = Bytes::from(serde_json::Value::Object(fields).to_string());
        }
        let mut expires = now_plus(9);
        if let Some(deadline) = parse_time(&op.deadline) {
            expires = expires.min(deadline);
        }
        let auth = AuthContext { kind: "visitor".into(), subject_id: subject, site_id: req.site.clone(), expires_at: timestamp(expires), ..Default::default() };
        let proof = sign_delegation(&self.delegation, &auth, &matched.route.template, &sha256_hex(&body), &auth.expires_at);
        Ok((body, proof))
    }

    /// 以入口身份向 traffic-guard 申请本次提问 / 建会话的准入许可。
    async fn admission(&self, op: &Operation, action: &str, visitor_token: &str, client: IpAddr) -> Result<serde_json::Value, Problem> {
        let mut headers = HeaderMap::new();
        headers.insert(header::CONTENT_TYPE, HeaderValue::from_static("application/json"));
        headers.insert(header::ACCEPT, HeaderValue::from_static("application/json"));
        headers.insert(HeaderName::from_static("idempotency-key"), HeaderValue::from_str(&op.root_operation_id).map_err(|_| Problem::new("VALIDATION", "幂等键不合法"))?);
        let body = Bytes::from(serde_json::json!({"requestId": op.root_operation_id, "action": action, "visitorToken": visitor_token, "ip": client.to_string()}).to_string());
        let response = self.upstream.send(Call { owner: "traffic-guard", method: Method::POST, target: "/v1/admissions", body, headers, proof: None, timeout: Duration::from_secs(5) }, op).await?;
        let status = response.status();
        let payload: serde_json::Value = serde_json::from_slice(&read_limited(response.into_body(), 64 << 10).await?).unwrap_or_default();
        if !status.is_success() {
            let code = match platform::known_code(payload["code"].as_str().unwrap_or_default()) {
                "INTERNAL" => "DEPENDENCY_UNAVAILABLE",
                code => code,
            };
            let mut problem = Problem::new(code, payload["detail"].as_str().unwrap_or_default().to_owned());
            problem.retryable = code == "RATE_LIMITED";
            return Err(problem);
        }
        let permit = payload["permit"].clone();
        if permit["admissionId"].as_str().is_none_or(str::is_empty) {
            return Err(Problem::new("INTEGRITY_ERROR", "准入许可响应不可读"));
        }
        Ok(permit)
    }

    /// 访问统计信标：校验访客正文，补上可信代理链取得的访客地址、客户端信息、数据代次和本站随机 cookie 区分的访客标识，交给攒批。
    fn trusted_beacon(&self, req: &Inbound, body: &[u8], op: &Operation, extra: &mut HeaderMap) -> Result<serde_json::Value, Problem> {
        if body.len() > 2048 {
            return Err(Problem::new("BUDGET_EXCEEDED", "统计正文超出上限"));
        }
        let peer = req.client(&self.trusted)?;
        let agent = req.headers.get(header::USER_AGENT).map(|v| v.to_str().map(str::to_owned)).unwrap_or(Ok(String::new())).map_err(|_| Problem::new("VALIDATION", "统计客户端信息不合法"))?;
        if agent.len() > 1024 {
            return Err(Problem::new("VALIDATION", "统计客户端信息不合法"));
        }
        let mut fields = beacons::parse(body)?;
        let visitor = self.analytics_visitor(req, extra);
        fields.insert("peerAddress".into(), peer.to_string().into());
        fields.insert("userAgent".into(), agent.into());
        fields.insert("dataEpoch".into(), op.data_epoch.clone().into());
        fields.insert("visitorId".into(), visitor.into());
        Ok(serde_json::Value::Object(fields))
    }

    /// 用本站随机 cookie 区分访客；不按 IP 或设备指纹合并，不作为授权身份。
    fn analytics_visitor(&self, req: &Inbound, extra: &mut HeaderMap) -> String {
        let sign = |id: &str| self.delegation.private_digest("portal.analytics-visitor.v1", format!("{}:{id}", req.site).as_bytes());
        if let Some((id, signature)) = cookie(&req.headers, "nw_visitor").as_deref().and_then(|v| v.split_once('.')) {
            if valid_identifier(id) && sign(id) == signature {
                return id.to_owned();
            }
        }
        let id = new_id();
        let secure = if req.origins.iter().any(|o| o.starts_with("https://")) { "; Secure" } else { "" };
        let value = format!("nw_visitor={id}.{}; Path=/; Max-Age={}; HttpOnly{secure}; SameSite=Lax", sign(&id), 180 * 24 * 3600);
        extra.append(header::SET_COOKIE, HeaderValue::from_str(&value).expect("cookie 只含 ASCII"));
        id
    }

    /// 后台单页应用（/admin 回退到 index.html）、其脚本与样式（/_admin/）、公开页样式与交互岛（/_site/，带版本段的长期缓存）。
    async fn static_web(&self, path: &str) -> Option<Response<Body>> {
        let dir = self.web_dir.as_ref()?;
        let (file, cache) = if path == "/admin" || path.starts_with("/admin/") {
            (dir.join("index.html"), "no-cache")
        } else if let Some(rest) = path.strip_prefix("/_admin/") {
            (dir.join(safe_relative(rest)?), "no-cache")
        } else {
            let rest = path.strip_prefix("/_site/")?;
            // 公开页按 /_site/v/<版本>/site.js 引用，去掉版本段后是同一批文件，可以长期缓存。
            match rest.strip_prefix("v/").and_then(|v| v.split_once('/')) {
                Some((_, file)) => (dir.join("site").join(safe_relative(file)?), "public, max-age=31536000, immutable"),
                None => (dir.join("site").join(safe_relative(rest)?), "no-cache"),
            }
        };
        let Ok(bytes) = tokio::fs::read(&file).await else {
            return Some(text(StatusCode::NOT_FOUND, "404 page not found"));
        };
        let mut response = full(StatusCode::OK, mime_type(&file), bytes);
        response.headers_mut().insert(header::CACHE_CONTROL, HeaderValue::from_static(cache));
        if file.file_name().is_some_and(|n| n == "index.html") && !path.starts_with("/_") {
            // worker-src 放行 blob：登录页的人机验证在 blob 创建的 Web Worker 里算工作量证明。
            response.headers_mut().insert(header::CONTENT_SECURITY_POLICY, HeaderValue::from_static("default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; worker-src 'self' blob:; frame-ancestors 'none'"));
        }
        Some(response)
    }
}

fn language_tag(value: &str) -> bool {
    let mut parts = value.split('-');
    let first = parts.next().unwrap_or_default();
    let rest: Vec<&str> = parts.collect();
    (2..=8).contains(&first.len()) && first.bytes().all(|b| b.is_ascii_alphabetic()) && rest.len() <= 4 && rest.iter().all(|p| (1..=8).contains(&p.len()) && p.bytes().all(|b| b.is_ascii_alphanumeric()))
}

/// 静态文件的相对路径：解码后不允许 .. 段、反斜杠和空字节。
fn safe_relative(rest: &str) -> Option<PathBuf> {
    let decoded = percent_encoding::percent_decode_str(rest).decode_utf8().ok()?;
    if decoded.is_empty() || decoded.contains(['\\', '\0']) || decoded.split('/').any(|s| s == ".." || s == ".") {
        return None;
    }
    Some(PathBuf::from(decoded.trim_start_matches('/')))
}

fn mime_type(file: &std::path::Path) -> &'static str {
    match file.extension().and_then(|e| e.to_str()).unwrap_or_default() {
        "html" => "text/html; charset=utf-8",
        "js" | "mjs" => "text/javascript; charset=utf-8",
        "css" => "text/css; charset=utf-8",
        "json" | "map" => "application/json",
        "svg" => "image/svg+xml",
        "png" => "image/png",
        "jpg" | "jpeg" => "image/jpeg",
        "webp" => "image/webp",
        "gif" => "image/gif",
        "ico" => "image/x-icon",
        "woff2" => "font/woff2",
        "woff" => "font/woff",
        "ttf" => "font/ttf",
        "otf" => "font/otf",
        "wasm" => "application/wasm",
        "txt" => "text/plain; charset=utf-8",
        _ => "application/octet-stream",
    }
}

/// 普通响应：只留白名单内的响应头，正文流式转发（事件流关闭代理缓冲）。
fn stream_response(upstream: Response<hyper::body::Incoming>, head: bool) -> Response<Body> {
    let (parts, body) = upstream.into_parts();
    let mut response = if head { full(parts.status, "", Bytes::new()) } else { Response::new(body.boxed()) };
    *response.status_mut() = parts.status;
    for (name, value) in parts.headers.iter() {
        if RESPONSE_HEADERS.contains(&name.as_str()) {
            response.headers_mut().append(name.clone(), value.clone());
        }
    }
    if header_str(&parts.headers, "content-type").to_ascii_lowercase().starts_with("text/event-stream") {
        response.headers_mut().insert(HeaderName::from_static("x-accel-buffering"), HeaderValue::from_static("no"));
    }
    response
}

/// 认证类操作：内部会话材料（access / refresh token）只变成 HttpOnly cookie，浏览器正文只拿到白名单字段；
/// 撤销当前设备的会话成功时清掉两枚 cookie。失败只转已知错误码，不转内部说明。
async fn adapt_authentication(upstream: Response<hyper::body::Incoming>, operation: &str, revocation: bool) -> Result<Response<Body>, Problem> {
    let status = upstream.status();
    let content_type = header_str(upstream.headers(), "content-type").split(';').next().unwrap_or_default().trim().to_ascii_lowercase();
    let retry_after = upstream.headers().get(header::RETRY_AFTER).cloned();
    let raw = read_limited(upstream.into_body(), 64 << 10).await?;
    let integrity = |detail: &str| Problem::new("INTEGRITY_ERROR", detail.to_owned());
    let (body, cookies, problem) = if status != StatusCode::OK {
        let upstream: serde_json::Value = serde_json::from_slice(&raw).map_err(|_| integrity("身份失败响应不符合Problem协议"))?;
        let code = platform::known_code(upstream["code"].as_str().unwrap_or_default());
        let request_id = upstream["requestId"].as_str().unwrap_or_default();
        if content_type != "application/problem+json" || platform::problem_status(code) != status.as_u16() || !valid_identifier(request_id) {
            return Err(integrity("身份失败响应不符合Problem协议"));
        }
        let detail = if code == "CONFLICT" { "认证操作已提交或发生冲突，请重新登录。" } else { "认证操作未能完成，请按错误码处理。" };
        let body = serde_json::json!({"type": "about:blank", "title": "认证操作未能完成", "status": status.as_u16(), "code": code, "detail": detail, "requestId": request_id, "retryable": upstream["retryable"].as_bool().unwrap_or(false)});
        (body.to_string(), Vec::new(), true)
    } else if revocation {
        let result: serde_json::Value = serde_json::from_slice(&raw).map_err(|_| integrity("会话撤销成功回执不符"))?;
        if content_type != "application/json" || result["revoked"].as_bool() != Some(true) {
            return Err(integrity("会话撤销成功回执不符"));
        }
        let gone = "Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Max-Age=0; HttpOnly; Secure; SameSite=Strict";
        (r#"{"revoked":true}"#.to_owned(), vec![format!("nw_sid=; {gone}"), format!("nw_refresh=; {gone}")], false)
    } else {
        let result: serde_json::Value = serde_json::from_slice(&raw).map_err(|_| integrity("身份成功响应不符合认证协议"))?;
        if content_type != "application/json" {
            return Err(integrity("身份成功响应媒体类型不符"));
        }
        let has = |name: &str| !result[name].is_null();
        let shape_ok = match operation {
            "createSession" => has("tokens") && !has("currentSessionTokens") && !has("account") && has("nextRoute"),
            "registerFirstSuperadmin" | "registerUser" | "oauthLogin" => has("tokens") && !has("currentSessionTokens") && has("account") && has("nextRoute"),
            "refreshSession" => has("tokens") && !has("currentSessionTokens") && !has("account") && !has("nextRoute"),
            _ => !has("tokens") && has("currentSessionTokens") && has("account") && !has("nextRoute"),
        };
        if !shape_ok {
            return Err(integrity("身份成功响应字段不符"));
        }
        let next = result["nextRoute"].as_str();
        if next.is_some_and(|r| !matches!(r, "/admin/me" | "/admin/setup" | "/admin")) {
            return Err(integrity("身份返回路由不在白名单"));
        }
        let tokens = if operation == "changePassword" { &result["currentSessionTokens"] } else { &result["tokens"] };
        let field = |name: &str| tokens[name].as_str().unwrap_or_default();
        let expires = parse_time(field("expiresAt")).filter(|t| *t > time::OffsetDateTime::now_utc()).ok_or_else(|| integrity("身份会话cookie材料不完整"))?;
        let auth = &tokens["auth"];
        let complete = !field("sessionId").is_empty() && !field("accessToken").is_empty() && field("refreshToken").len() >= 32 && !field("csrfToken").is_empty() && field("csrfToken").len() <= 2048
            && auth.is_object() && tokens["mustChangePassword"].is_boolean() && auth["sessionId"].as_str() == Some(field("sessionId")) && auth["expiresAt"].as_str() == Some(field("expiresAt"));
        let cookie_safe = |v: &str| v.bytes().all(|b| b.is_ascii_graphic() && !b"\";\\,".contains(&b));
        if !complete || !cookie_safe(field("accessToken")) || !cookie_safe(field("refreshToken")) {
            return Err(integrity("身份会话cookie材料不完整"));
        }
        let cookies = vec![
            format!("nw_sid={}; Path=/; Expires={}; HttpOnly; Secure; SameSite=Strict", field("accessToken"), http_date(expires)),
            format!("nw_refresh={}; Path=/; HttpOnly; Secure; SameSite=Strict", field("refreshToken")),
        ];
        let mut browser = serde_json::json!({"session": {"sessionId": field("sessionId"), "auth": auth, "mustChangePassword": tokens["mustChangePassword"], "expiresAt": field("expiresAt"), "csrfToken": field("csrfToken")}});
        if has("account") {
            browser["account"] = result["account"].clone();
        }
        if let Some(next) = next {
            browser["nextRoute"] = next.into();
        }
        (browser.to_string(), cookies, false)
    };
    let mut response = full(status, if problem { "application/problem+json" } else { "application/json; charset=utf-8" }, body);
    let headers = response.headers_mut();
    headers.insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));
    headers.insert(header::PRAGMA, HeaderValue::from_static("no-cache"));
    headers.insert(header::X_CONTENT_TYPE_OPTIONS, HeaderValue::from_static("nosniff"));
    if let Some(value) = retry_after {
        headers.insert(header::RETRY_AFTER, value);
    }
    for value in cookies {
        headers.append(header::SET_COOKIE, HeaderValue::from_str(&value).map_err(|_| integrity("身份会话cookie不符合浏览器协议"))?);
    }
    Ok(response)
}
