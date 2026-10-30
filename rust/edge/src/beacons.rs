//! 访问统计信标攒批：入口校验访客正文、补上可信字段后先放在内存里，每 2 秒（或某站点攒满一批）按站点与数据代次
//! 成批交给 analytics。统计允许少量丢失：积压超过上限的新信标直接丢弃，进程退出时没送出的也不补。

use std::collections::HashMap;
use std::sync::Mutex;
use std::time::Duration;

use bytes::Bytes;
use hyper::header::{HeaderMap, HeaderName, HeaderValue, CONTENT_TYPE};
use hyper::Method;
use serde::Deserialize;
use serde_json::{json, Map, Value};
use tokio::sync::Notify;

use crate::platform::{new_id, Problem};
use crate::upstream::{Call, Operation, Upstream};

const BATCH: usize = 200;
const BACKLOG: usize = 20_000;

/// 浏览器发来的信标正文（与 analytics 的 IngestBeaconRequestBody 一致）；入口是访客正文的边界，在这里校验一次，
/// 成批转交时 analytics 的结构校验就不会因为某一条坏数据拒掉整批。
#[derive(Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct Visitor {
    beacon_id: String,
    document_id: String,
    locale: String,
    event_type: String,
    status: u16,
    elapsed_ms: u32,
    referrer_domain: String,
    // 客户端信息以请求头为准，正文里的只为兼容旧脚本。
    #[serde(rename = "userAgent")]
    _user_agent: Option<String>,
}

pub fn parse(body: &[u8]) -> Result<Map<String, Value>, Problem> {
    let invalid = || Problem::new("VALIDATION", "统计正文不合法");
    let v: Visitor = serde_json::from_slice(body).map_err(|_| invalid())?;
    let id = |s: &str| (1..=128).contains(&s.len());
    let locale = (1..=35).contains(&v.locale.len())
        && v.locale.split('-').enumerate().all(|(i, part)| if i == 0 { (2..=8).contains(&part.len()) && part.bytes().all(|b| b.is_ascii_alphabetic()) } else { (1..=8).contains(&part.len()) && part.bytes().all(|b| b.is_ascii_alphanumeric()) });
    if !id(&v.beacon_id) || !id(&v.document_id) || !locale || v.event_type.len() > 8192 || !(100..=599).contains(&v.status) || v.elapsed_ms > 60_000 || v.referrer_domain.len() > 8192 {
        return Err(invalid());
    }
    let fields = json!({"beaconId": v.beacon_id, "documentId": v.document_id, "locale": v.locale, "eventType": v.event_type,
        "status": v.status, "elapsedMs": v.elapsed_ms, "referrerDomain": v.referrer_domain});
    Ok(fields.as_object().expect("对象").clone())
}

#[derive(Default)]
pub struct Beacons {
    /// (站点, 数据代次) → 待送出的信标。
    pending: Mutex<HashMap<(String, String), Vec<Value>>>,
    wake: Notify,
}

impl Beacons {
    pub fn push(&self, site: &str, epoch: &str, beacon: Value) {
        let mut pending = self.pending.lock().unwrap();
        if pending.values().map(Vec::len).sum::<usize>() >= BACKLOG {
            return;
        }
        let queue = pending.entry((site.to_owned(), epoch.to_owned())).or_default();
        queue.push(beacon);
        if queue.len() >= BATCH {
            self.wake.notify_one();
        }
    }

    pub async fn run(&self, upstream: &Upstream) {
        loop {
            tokio::select! {
                _ = tokio::time::sleep(Duration::from_secs(2)) => {}
                _ = self.wake.notified() => {}
            }
            let batches = std::mem::take(&mut *self.pending.lock().unwrap());
            for ((site, epoch), beacons) in batches {
                for chunk in beacons.chunks(BATCH) {
                    if let Err(problem) = send(upstream, &site, &epoch, chunk).await {
                        eprintln!("访问信标转交 analytics 失败，丢弃 {} 条：{}", chunk.len(), problem.code);
                    }
                }
            }
        }
    }
}

async fn send(upstream: &Upstream, site: &str, epoch: &str, beacons: &[Value]) -> Result<(), Problem> {
    let op = Operation::new(site, epoch);
    let mut headers = HeaderMap::new();
    headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
    headers.insert(HeaderName::from_static("idempotency-key"), HeaderValue::from_str(&new_id()).expect("标识只含 ASCII"));
    let body = Bytes::from(json!({"beacons": beacons}).to_string());
    let response = upstream.send(Call { owner: "analytics", method: Method::POST, target: "/v1/internal/beacons", body, headers, proof: None, timeout: Duration::from_secs(10) }, &op).await?;
    if !response.status().is_success() {
        return Err(Problem::new("DEPENDENCY_UNAVAILABLE", format!("analytics 返回 {}", response.status())));
    }
    Ok(())
}
