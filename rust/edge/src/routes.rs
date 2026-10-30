//! 入口路由表：modules/portal/config/routes.json（由各服务的 OpenAPI 生成）。
//! 只按已登记的静态模板匹配，路径不会被当成上游 URL；越具体的模板越先匹配。

use percent_encoding::percent_decode_str;
use serde::Deserialize;

use crate::platform::Problem;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct CatalogRoute {
    method: String,
    path: String,
    owner: String,
    operation_id: String,
    permission: String,
    admission: Option<String>,
    timeout_ms: Option<u64>,
    /// 是否流式按上游响应头判断，这个标志不用。
    #[allow(dead_code)]
    streaming: bool,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Catalog {
    version: u32,
    routes: Vec<CatalogRoute>,
}

pub struct Route {
    pub method: String,
    pub template: String,
    pub owner: String,
    pub operation: String,
    pub permission: String,
    pub admission: String,
    pub timeout_ms: u64,
    segments: Vec<String>,
}

impl Route {
    /// “拥有方:操作名”，各处的操作清单都按它查。
    pub fn action(&self) -> String {
        format!("{}:{}", self.owner, self.operation)
    }

    pub fn anonymous(&self) -> bool {
        self.permission == "anonymous"
    }
}

pub struct Matched<'a> {
    pub route: &'a Route,
    pub parameters: Vec<(String, String)>,
}

impl Matched<'_> {
    pub fn parameter(&self, name: &str) -> Option<&str> {
        self.parameters.iter().find(|(n, _)| n == name).map(|(_, v)| v.as_str())
    }
}

pub struct Registry {
    routes: Vec<Route>,
}

/// 原先由 portal 组件本地处理、尚未实现的路由（bootstrap、事件流、偏好、就绪汇总）不进表，请求得到 404。
const LOCAL: [&str; 7] = ["GET /healthz", "GET /readyz", "GET /v1/bootstrap", "GET /v1/events", "GET /v1/preferences", "PUT /v1/preferences", "GET /v1/readiness"];

impl Registry {
    pub fn load(path: &str) -> anyhow::Result<Self> {
        let catalog: Catalog = serde_json::from_str(&std::fs::read_to_string(path)?)?;
        anyhow::ensure!(catalog.version == 1 && !catalog.routes.is_empty(), "入口路由表格式不合法");
        let mut routes = Vec::new();
        for entry in catalog.routes {
            let method = entry.method.to_ascii_uppercase();
            if entry.owner == "portal" || LOCAL.contains(&format!("{method} {}", entry.path).as_str()) {
                continue;
            }
            routes.push(Route {
                segments: split(&entry.path).map(str::to_owned).collect(),
                method,
                template: entry.path,
                owner: entry.owner,
                operation: entry.operation_id,
                permission: entry.permission,
                admission: entry.admission.unwrap_or_default(),
                timeout_ms: entry.timeout_ms.unwrap_or(5000),
            });
        }
        // 静态段多的优先，其次段数多的、模板长的。
        routes.sort_by(|a, b| {
            let statics = |r: &Route| r.segments.iter().filter(|s| !s.contains('{')).count();
            statics(b).cmp(&statics(a)).then(b.segments.len().cmp(&a.segments.len())).then(b.template.len().cmp(&a.template.len())).then(a.action().cmp(&b.action()))
        });
        Ok(Self { routes })
    }

    pub fn resolve(&self, method: &str, escaped: &str) -> Result<Matched<'_>, Problem> {
        if escaped.is_empty() || escaped.len() > 2048 || !escaped.starts_with('/') || escaped.contains(['?', '#', '\\']) {
            return Err(Problem::new("VALIDATION", "请求路径不合法"));
        }
        let lower = escaped.to_ascii_lowercase();
        if lower.contains("%25") || lower.contains("%2f") || lower.contains("%5c") {
            return Err(Problem::new("VALIDATION", "请求路径编码不安全"));
        }
        let decoded = percent_decode_str(escaped).decode_utf8().map_err(|_| Problem::new("VALIDATION", "请求路径编码不合法"))?;
        if decoded.contains(['\\', '?', '#']) || decoded.contains("//") || decoded.split('/').any(|s| s == "." || s == ".." || s.contains('\0')) {
            return Err(Problem::new("VALIDATION", "请求路径编码不合法"));
        }
        let path: Vec<&str> = split(&decoded).collect();
        for route in self.routes.iter().filter(|r| r.method == method) {
            if let Some(parameters) = match_template(&route.segments, &path) {
                return Ok(Matched { route, parameters });
            }
        }
        Err(Problem::new("NOT_FOUND", "路由未登记"))
    }
}

fn split(path: &str) -> impl Iterator<Item = &str> {
    path.strip_prefix('/').unwrap_or(path).split('/').filter(|s| !s.is_empty())
}

fn match_template(template: &[String], path: &[&str]) -> Option<Vec<(String, String)>> {
    let mut path: Vec<String> = path.iter().map(|s| s.to_string()).collect();
    // 公开页面的末段 {slug} 可以是多段（嵌套文档的路径），合并成一个参数。
    let n = template.len();
    if n > 0 && template[n - 1] == "{slug}" && path.len() > n {
        let tail = path.split_off(n - 1).join("/");
        path.push(tail);
    }
    if template.len() != path.len() {
        return None;
    }
    let mut parameters = Vec::new();
    for (segment, value) in template.iter().zip(path.iter()) {
        match segment.find('{') {
            None if segment == value => {}
            None => return None,
            // 一个段里只有一个变量（路由表里没有多变量段），前后是固定文字，如 {documentId}:publish。
            Some(start) => {
                let end = segment.find('}')?;
                let (prefix, name, suffix) = (&segment[..start], &segment[start + 1..end], &segment[end + 1..]);
                let inner = value.strip_prefix(prefix)?.strip_suffix(suffix)?;
                if inner.is_empty() || inner.contains('/') {
                    return None;
                }
                parameters.push((name.to_owned(), inner.to_owned()));
            }
        }
    }
    Some(parameters)
}
