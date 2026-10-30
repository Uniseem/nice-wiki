//! 调用 Go 服务：每个请求带 portal 身份的服务断言（≤60 秒、绑定方法 / 路径 / 正文摘要 / jti），
//! 再按操作上下文写入请求标识、期限、数据代次、费用类别和用户委托证明（与 serviceauth.SignOutboundRequest 相同）。

use std::time::Duration;

use bytes::Bytes;
use http_body_util::{BodyExt, Full};
use hyper::body::Incoming;
use hyper::header::{HeaderName, HeaderValue};
use hyper::{HeaderMap, Method, Request, Response};
use hyper_util::client::legacy::connect::HttpConnector;
use hyper_util::client::legacy::Client;
use hyper_util::rt::TokioExecutor;
use percent_encoding::percent_decode_str;

use crate::platform::{new_id, now_plus, sha256_hex, timestamp, Key, Problem};

/// 一次入口请求的可信操作上下文（OperationContext 里 edge 会往下传的部分）。
#[derive(Clone, Default)]
pub struct Operation {
    pub request_id: String,
    pub operation_id: String,
    pub root_operation_id: String,
    pub deadline: String,
    pub data_epoch: String,
    pub cost_class: String,
    /// 断言里的站点作用域；有用户委托时必须与委托主体的站点一致。
    pub site_id: String,
    pub login_scopes: Vec<String>,
    pub budget_scopes: Vec<String>,
}

impl Operation {
    pub fn new(site_id: &str, data_epoch: &str) -> Self {
        let id = new_id();
        Self {
            request_id: new_id(),
            operation_id: id.clone(),
            root_operation_id: id,
            deadline: timestamp(now_plus(30)),
            data_epoch: data_epoch.to_owned(),
            site_id: site_id.to_owned(),
            ..Default::default()
        }
    }
}

pub struct Upstream {
    client: Client<HttpConnector, Full<Bytes>>,
    pub key: Key,
    dependencies: std::collections::HashMap<String, String>,
}

pub struct Call<'a> {
    pub owner: &'a str,
    pub method: Method,
    /// 已转义的路径加查询，如 /v1/hosts/a.example.com?x=1。
    pub target: &'a str,
    pub body: Bytes,
    pub headers: HeaderMap,
    pub proof: Option<&'a str>,
    pub timeout: Duration,
}

impl Upstream {
    pub fn new(key: Key, dependencies: std::collections::HashMap<String, String>) -> Self {
        let mut connector = HttpConnector::new();
        connector.set_nodelay(true);
        connector.set_connect_timeout(Some(Duration::from_secs(3)));
        let client = Client::builder(TokioExecutor::new()).pool_idle_timeout(Duration::from_secs(30)).build(connector);
        Self { client, key, dependencies }
    }

    fn assertion(&self, audience: &str, method: &Method, path: &str, body_hash: &str, op: &Operation) -> String {
        let issued = time::OffsetDateTime::now_utc();
        let mut claims = serde_json::json!({
            "iss": "portal", "aud": audience, "method": method.as_str(), "path": path, "bodyHash": body_hash,
            "issuedAt": timestamp(issued), "expiresAt": timestamp(issued + time::Duration::seconds(60)), "jti": new_id(),
        });
        if !op.site_id.is_empty() {
            claims["siteId"] = op.site_id.clone().into();
        }
        if audience == "identity" && !op.login_scopes.is_empty() {
            claims["loginNetworkScopes"] = op.login_scopes.clone().into();
        }
        if audience == "traffic-guard" && !op.budget_scopes.is_empty() {
            claims["networkBudgetScopes"] = op.budget_scopes.clone().into();
        }
        self.key.jws("JWT", claims.to_string().as_bytes())
    }

    pub async fn send(&self, call: Call<'_>, op: &Operation) -> Result<Response<Incoming>, Problem> {
        let base = self.dependencies.get(call.owner).filter(|b| b.starts_with("http://") || b.starts_with("https://")).ok_or_else(|| Problem::new("DEPENDENCY_UNAVAILABLE", "拥有服务依赖未配置"))?;
        let uri: hyper::Uri = format!("{}{}", base.trim_end_matches('/'), call.target).parse().map_err(|_| Problem::new("VALIDATION", "上游请求构造失败"))?;
        // Go 服务按解码后的路径（r.URL.Path）核对断言。
        let path = percent_decode_str(uri.path()).decode_utf8_lossy().into_owned();
        let token = self.assertion(call.owner, &call.method, &path, &sha256_hex(&call.body), op);
        let mut request = Request::builder().method(call.method).uri(uri).body(Full::new(call.body)).expect("请求各部分已校验");
        let headers = request.headers_mut();
        *headers = call.headers;
        let mut set = |name: &'static str, value: &str| {
            if let Ok(value) = HeaderValue::from_str(value) {
                headers.insert(HeaderName::from_static(name), value);
            }
        };
        set("authorization", &format!("Bearer {token}"));
        set("x-request-id", &op.request_id);
        set("x-operation-id", &op.operation_id);
        set("x-root-operation-id", &op.root_operation_id);
        set("x-deadline", &op.deadline);
        if !op.data_epoch.is_empty() {
            set("x-data-epoch", &op.data_epoch);
        }
        if !op.cost_class.is_empty() {
            set("x-cost-class", &op.cost_class);
        }
        if let Some(proof) = call.proof {
            set("x-portal-proof", proof);
        }
        match tokio::time::timeout(call.timeout, self.client.request(request)).await {
            Ok(Ok(response)) => Ok(response),
            Ok(Err(_)) => Err(Problem::new("DEPENDENCY_UNAVAILABLE", "拥有服务不可达")),
            Err(_) => Err(Problem::new("DEADLINE_EXCEEDED", "上游请求超时")),
        }
    }

    /// 服务间的小请求：读完响应体（有上限），失败的 Problem 按上游错误码还原。
    pub async fn json(&self, owner: &str, method: Method, target: &str, body: Option<serde_json::Value>, op: &Operation) -> Result<serde_json::Value, Problem> {
        let mut headers = HeaderMap::new();
        headers.insert(hyper::header::ACCEPT, HeaderValue::from_static("application/json"));
        let body = match body {
            Some(value) => {
                headers.insert(hyper::header::CONTENT_TYPE, HeaderValue::from_static("application/json"));
                Bytes::from(value.to_string())
            }
            None => Bytes::new(),
        };
        let response = self.send(Call { owner, method, target, body, headers, proof: None, timeout: Duration::from_secs(5) }, op).await?;
        let status = response.status();
        let raw = read_limited(response.into_body(), 1 << 20).await?;
        let value: serde_json::Value = serde_json::from_slice(&raw).unwrap_or_default();
        if !status.is_success() {
            let code = crate::platform::known_code(value["code"].as_str().unwrap_or("DEPENDENCY_UNAVAILABLE"));
            return Err(Problem::new(code, value["detail"].as_str().unwrap_or("上游请求未能完成").to_owned()));
        }
        Ok(value)
    }
}

pub async fn read_limited(body: Incoming, max: usize) -> Result<Bytes, Problem> {
    let collected = http_body_util::Limited::new(body, max).collect().await.map_err(|_| Problem::new("INTEGRITY_ERROR", "上游响应不完整或超出上限"))?;
    Ok(collected.to_bytes())
}
