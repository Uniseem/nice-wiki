//! 按访问域名定位站点（identity:resolveHost，短时缓存），并处理站点状态：
//! 非主域名跳转、下架或删除返回 410、私有站点要求访问凭据（成员一次性码、访问密码、分享链接）。
//! 另有 Caddy 按需证书的询问接口 /_tls/allow。

use std::collections::HashMap;
use std::sync::Mutex;
use std::time::{Duration, Instant};

use bytes::Bytes;
use hyper::header::{self, HeaderMap, HeaderValue};
use hyper::{Method, Response, StatusCode};
use percent_encoding::{utf8_percent_encode, NON_ALPHANUMERIC};

use crate::gateway::{cookie, html, redirect, Body, Gateway};
use crate::platform::Problem;
use crate::upstream::Operation;

#[derive(Clone, Default)]
pub struct HostSite {
    pub site_id: String,
    pub status: String,
    pub visibility: String,
    pub primary_host: String,
    pub redirect_host: String,
    pub platform_host: String,
    /// 私有站点设了访问密码，访客可以在站点域名上输入密码进入。
    pub password_enabled: bool,
}

#[derive(Default)]
pub struct HostCache {
    entries: Mutex<HashMap<String, (Option<HostSite>, Instant)>>,
}

impl HostCache {
    /// 站点登记改了（可见性、密码、域名、状态）：立即作废，免得私有化后还按公开放行。
    pub fn forget(&self) {
        self.entries.lock().unwrap().clear();
    }
}

/// 访问域名：小写、去端口、去末尾的点。
pub fn request_host(headers: &HeaderMap) -> String {
    let host = headers.get(header::HOST).and_then(|v| v.to_str().ok()).unwrap_or_default().to_ascii_lowercase();
    let host = if let Some(rest) = host.strip_prefix('[') {
        rest.split(']').next().unwrap_or_default().to_owned()
    } else {
        host.rsplit_once(':').filter(|(_, port)| port.bytes().all(|b| b.is_ascii_digit())).map(|(h, _)| h.to_owned()).unwrap_or(host)
    };
    host.trim_end_matches('.').to_owned()
}

pub fn request_scheme(headers: &HeaderMap) -> &'static str {
    match headers.get("x-forwarded-proto").and_then(|v| v.to_str().ok()) {
        Some("https") => "https",
        _ => "http",
    }
}

fn escape(value: &str) -> String {
    utf8_percent_encode(value, NON_ALPHANUMERIC).to_string()
}

impl Gateway {
    /// 域名 → 站点，正向缓存 20 秒、未登记 5 秒；identity 出错时不缓存、按未登记处理。
    pub async fn host_site(&self, headers: &HeaderMap) -> Option<HostSite> {
        let host = request_host(headers);
        if let Some((site, expires)) = self.hosts.entries.lock().unwrap().get(&host) {
            if *expires > Instant::now() {
                return site.clone();
            }
        }
        let op = Operation::new("system", &self.data_epoch().await);
        let resolved = match self.upstream.json("identity", Method::GET, &format!("/v1/hosts/{}", escape(&host)), None, &op).await {
            Ok(body) => {
                let text = |name: &str| body[name].as_str().unwrap_or_default().to_owned();
                let site = HostSite {
                    site_id: text("siteId"), status: text("status"), visibility: text("visibility"), primary_host: text("primaryHost"),
                    redirect_host: text("redirectHost"), platform_host: text("platformHost"), password_enabled: body["passwordEnabled"].as_bool().unwrap_or(false),
                };
                (!site.site_id.is_empty()).then_some(site)
            }
            Err(problem) if problem.code == "NOT_FOUND" => None,
            Err(_) => return None,
        };
        let ttl = Duration::from_secs(if resolved.is_some() { 20 } else { 5 });
        let mut entries = self.hosts.entries.lock().unwrap();
        if entries.len() > 10_000 {
            entries.clear();
        }
        entries.insert(host, (resolved.clone(), Instant::now() + ttl));
        resolved
    }

    /// 已解析站点的状态门：返回 Some 表示已经给出响应。
    pub fn site_gate(&self, site: &HostSite, method: &Method, path: &str, uri: &str, headers: &HeaderMap) -> Option<Response<Body>> {
        let admin_path = path.starts_with("/admin") || path.starts_with("/_admin/") || path.starts_with("/v1/") || path.starts_with("/_site/");
        if !site.status.is_empty() && site.status != "active" && !admin_path {
            return Some(html(StatusCode::GONE, "<!doctype html><meta charset=utf-8><title>站点不可用</title><p>该站点已下架或已删除。</p>".into()));
        }
        if !site.redirect_host.is_empty() && method == Method::GET && !admin_path {
            let mut response = redirect(StatusCode::MOVED_PERMANENTLY, &format!("{}://{}{uri}", request_scheme(headers), site.redirect_host));
            response.headers_mut().insert(header::CACHE_CONTROL, HeaderValue::from_static("public, max-age=300"));
            return Some(response);
        }
        if site.visibility == "private" && !private_open_path(path) && !self.has_private_access(headers, &site.site_id) && cookie(headers, "nw_sid").is_none_or(|v| v.is_empty()) {
            return Some(self.deny_private(site, method, uri, headers));
        }
        None
    }

    fn has_private_access(&self, headers: &HeaderMap, site: &str) -> bool {
        let Some(token) = cookie(headers, "nw_pv") else { return false };
        let Some(payload) = self.upstream.key.verify_cursor(&private_scope(site), &token) else { return false };
        std::str::from_utf8(&payload).ok().and_then(|v| v.parse::<i64>().ok()).is_some_and(|expires| time::OffsetDateTime::now_utc().unix_timestamp() < expires)
    }

    fn member_link(site: &HostSite, headers: &HeaderMap, back: &str) -> String {
        if site.platform_host.is_empty() {
            return String::new();
        }
        format!("{}://{}/admin/private-access?site={}&return={}", request_scheme(headers), site.platform_host, escape(&site.site_id), escape(back))
    }

    /// 设了访问密码的站点给访客一个输入密码的页面（成员可以从这里去登录）；没设的，页面请求带去平台域名确认身份；其余请求直接 401。
    fn deny_private(&self, site: &HostSite, method: &Method, uri: &str, headers: &HeaderMap) -> Response<Body> {
        let page = method == Method::GET && headers.get(header::ACCEPT).and_then(|v| v.to_str().ok()).is_some_and(|v| v.contains("text/html"));
        let member = Self::member_link(site, headers, uri);
        if page && site.password_enabled {
            return password_page(StatusCode::UNAUTHORIZED, uri, &member, "");
        }
        if page && !member.is_empty() {
            return no_store(redirect(StatusCode::FOUND, &member));
        }
        no_store(html(StatusCode::UNAUTHORIZED, "<!doctype html><meta charset=utf-8><title>私有站点</title><p>该站点仅限成员访问，请先登录。</p>".into()))
    }

    async fn redeem(&self, site: &str, credential: serde_json::Value) -> Result<(), Problem> {
        let op = Operation::new(site, &self.data_epoch().await);
        self.upstream.json("identity", Method::POST, &format!("/v1/sites/{}/private-access:redeem", escape(site)), Some(credential), &op).await.map(|_| ())
    }

    /// 发站点专用的短期访问 cookie（12 小时）并回到原页面。
    fn grant_private(&self, site: &HostSite, headers: &HeaderMap, back: &str) -> Response<Body> {
        let expires = time::OffsetDateTime::now_utc() + time::Duration::hours(12);
        let token = self.upstream.key.sign_cursor(&private_scope(&site.site_id), expires.unix_timestamp().to_string().as_bytes());
        let secure = if request_scheme(headers) == "https" { "; Secure" } else { "" };
        let mut response = no_store(redirect(StatusCode::FOUND, back));
        let value = format!("nw_pv={token}; Path=/; Expires={}; HttpOnly{secure}; SameSite=Lax", crate::gateway::http_date(expires));
        response.headers_mut().append(header::SET_COOKIE, HeaderValue::from_str(&value).expect("cookie 只含 ASCII"));
        response
    }

    /// 站点域名上进入私有站点的三个入口：成员带一次性码回来、访客提交访问密码、打开分享链接。
    pub async fn private_entry(&self, method: &Method, path: &str, query: &str, headers: &HeaderMap, body: &Bytes) -> Response<Body> {
        let bad = || crate::gateway::text(StatusCode::BAD_REQUEST, "bad request");
        let site = match self.host_site(headers).await {
            Some(site) if site.visibility == "private" => site,
            _ => return bad(),
        };
        let params = |source: &[u8], name: &str| form_urlencoded::parse(source).find(|(k, _)| k == name).map(|(_, v)| v.into_owned()).unwrap_or_default();
        match (method.as_str(), path) {
            ("GET", "/_pv/accept") => {
                let (code, back) = (params(query.as_bytes(), "code"), params(query.as_bytes(), "return"));
                if code.is_empty() || !safe_return(&back) {
                    return bad();
                }
                match self.redeem(&site.site_id, serde_json::json!({"code": code})).await {
                    Ok(()) => self.grant_private(&site, headers, &back),
                    Err(_) => crate::gateway::text(StatusCode::FORBIDDEN, "forbidden"),
                }
            }
            ("POST", "/_pv/password") => {
                if !site.password_enabled || body.len() > 4096 {
                    return bad();
                }
                let (mut back, password) = (params(body, "return"), params(body, "password"));
                if !safe_return(&back) {
                    back = "/".into();
                }
                let member = Self::member_link(&site, headers, &back);
                if password.is_empty() || password.len() > 128 {
                    return password_page(StatusCode::BAD_REQUEST, &back, &member, "请输入访问密码。");
                }
                match self.redeem(&site.site_id, serde_json::json!({"password": password})).await {
                    Ok(()) => self.grant_private(&site, headers, &back),
                    Err(problem) => {
                        let message = if problem.code == "RATE_LIMITED" { "试错太多，请过几分钟再试。" } else { "访问密码不对。" };
                        password_page(StatusCode::FORBIDDEN, &back, &member, message)
                    }
                }
            }
            ("GET", "/_pv/share") => {
                let token = params(query.as_bytes(), "token");
                if token.is_empty() || token.len() > 128 {
                    return bad();
                }
                match self.redeem(&site.site_id, serde_json::json!({"shareToken": token})).await {
                    Ok(()) => self.grant_private(&site, headers, "/"),
                    Err(_) => no_store(html(StatusCode::FORBIDDEN, "<!doctype html><meta charset=utf-8><title>私有站点</title><p>分享链接无效或已过期，请向站点所有者要一个新的。</p>".into())),
                }
            }
            _ => crate::gateway::text(StatusCode::NOT_FOUND, "not found"),
        }
    }

    /// Caddy 签证书前问：平台域名、已验证的站点域名可以签；部署时配置的公开域名一定放行（站主还没填平台域名时靠它拿第一张证书）。
    pub async fn tls_allow(&self, query: &str) -> Response<Body> {
        let domain = form_urlencoded::parse(query.as_bytes()).find(|(k, _)| k == "domain").map(|(_, v)| v.to_ascii_lowercase()).unwrap_or_default();
        let domain = domain.trim_end_matches('.');
        if domain.is_empty() || domain.len() > 253 {
            return crate::gateway::text(StatusCode::BAD_REQUEST, "bad domain");
        }
        if self.bootstrap_host.as_deref() == Some(domain) {
            return crate::gateway::text(StatusCode::OK, "");
        }
        let op = Operation::new("system", &self.data_epoch().await);
        match self.upstream.json("identity", Method::GET, &format!("/v1/tls/hosts/{}", escape(domain)), None, &op).await {
            Ok(body) if body["allowed"].as_bool() == Some(true) => crate::gateway::text(StatusCode::OK, ""),
            Ok(_) => crate::gateway::text(StatusCode::FORBIDDEN, "not allowed"),
            Err(_) => crate::gateway::text(StatusCode::SERVICE_UNAVAILABLE, "unavailable"),
        }
    }
}

fn private_scope(site: &str) -> String {
    format!("private-site:{site}")
}

/// 私有站点上不需要站点访问凭据的路径：后台页面与登录接口（登录本身要能打开）、静态资源、进入私有站点的入口。
fn private_open_path(path: &str) -> bool {
    ["/admin", "/_admin/", "/_site/", "/_pv/", "/_tls/", "/_report", "/v1/sessions", "/v1/auth/", "/v1/oauth/", "/v1/users:", "/v1/registration", "/v1/challenges", "/v1/self", "/v1/initialization"]
        .iter()
        .any(|prefix| path.starts_with(prefix))
}

/// 只接受站内相对路径，防止被用来跳到别的网站。
fn safe_return(back: &str) -> bool {
    back.starts_with('/') && !back.starts_with("//") && !back.contains('\\')
}

fn no_store(mut response: Response<Body>) -> Response<Body> {
    response.headers_mut().insert(header::CACHE_CONTROL, HeaderValue::from_static("no-store"));
    response
}

fn escape_html(text: &str) -> String {
    text.replace('&', "&amp;").replace('<', "&lt;").replace('>', "&gt;").replace('"', "&#34;").replace('\'', "&#39;")
}

/// 私有站点的访问密码页（纯表单，不依赖脚本）。
fn password_page(status: StatusCode, back: &str, member: &str, problem: &str) -> Response<Body> {
    let mut page = String::from(r#"<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>私有站点</title>"#);
    page.push_str(r#"<style>body{font:16px/1.6 system-ui,sans-serif;max-width:22rem;margin:15vh auto;padding:0 1rem;color:#222}input,button{font:inherit;width:100%;box-sizing:border-box;padding:.5rem;margin:.25rem 0}button{cursor:pointer}.e{color:#b00020}a{color:inherit}@media(prefers-color-scheme:dark){body{background:#111;color:#eee}}</style>"#);
    page.push_str(r#"<h1 style="font-size:1.25rem">这个站点需要访问密码</h1>"#);
    if !problem.is_empty() {
        page.push_str(&format!(r#"<p class="e">{}</p>"#, escape_html(problem)));
    }
    page.push_str(&format!(r#"<form method="post" action="/_pv/password"><input type="hidden" name="return" value="{}">"#, escape_html(back)));
    page.push_str(r#"<input type="password" name="password" autocomplete="current-password" required autofocus aria-label="访问密码"><button type="submit">进入</button></form>"#);
    if !member.is_empty() {
        page.push_str(&format!(r#"<p><a href="{}">我是站点成员，去登录</a></p>"#, escape_html(member)));
    }
    no_store(html(status, page))
}
