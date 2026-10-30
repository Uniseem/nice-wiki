//! 模型网关调用：同一幂等键先查（lookup），没有再创建，然后轮询到终态取完整结果。
//! 幂等键由调用方按业务步骤给出，重试同一步骤不会重复付费。

use std::time::Duration;

use hyper::Method;
use serde_json::Value;

use crate::platform::{parse_time, Problem};
use crate::upstream::{Operation, Upstream};

pub struct Outcome {
    pub invocation_id: String,
    /// 网关的 ModelOutput（capability 与 text / vectors / candidateIds 之一）。
    pub output: Value,
}

impl Outcome {
    /// 聊天类的完整文本。
    pub fn text(&self) -> Result<&str, Problem> {
        self.output["text"].as_str().ok_or_else(|| Problem::new("INTEGRITY_ERROR", "模型缺少完整文本"))
    }
}

const TERMINAL: [&str; 5] = ["succeeded", "failed", "cancelled", "charge_unknown", "reconciled"];

/// request 是 CreateInvocationInput（含 idempotencyKey）。期限取操作期限。
pub async fn invoke(upstream: &Upstream, op: &Operation, request: &Value) -> Result<Outcome, Problem> {
    let short = Duration::from_secs(10);
    let accepted = match upstream.request("model-gateway", Method::POST, "/v1/inferences/lookup", Some(request), op, None, short).await {
        Ok(found) => found,
        Err(problem) if problem.code == "INVOCATION_NOT_ACCEPTED" || problem.code == "NOT_FOUND" => {
            upstream.request("model-gateway", Method::POST, "/v1/inferences", Some(request), op, None, Duration::from_secs(30)).await?
        }
        Err(problem) => return Err(problem),
    };
    let id = accepted["invocation"]["invocationId"].as_str().ok_or_else(|| Problem::new("INTEGRITY_ERROR", "模型调用缺少调用标识"))?.to_owned();
    let deadline = parse_time(&op.deadline).ok_or_else(|| Problem::new("VALIDATION", "操作期限不合法"))?;
    let mut snapshot = accepted;
    let mut wait = Duration::from_millis(200);
    loop {
        let state = snapshot["invocation"]["state"].as_str().unwrap_or_default().to_owned();
        if TERMINAL.contains(&state.as_str()) {
            if snapshot["output"].is_object() {
                return Ok(Outcome { invocation_id: id, output: snapshot["output"].take() });
            }
            if state == "succeeded" || state == "reconciled" || state == "charge_unknown" {
                // 终态刚写入、结果稍后可读：再取一次。
                snapshot = upstream.request("model-gateway", Method::GET, &format!("/v1/inferences/{id}"), None, op, None, short).await?;
                if snapshot["output"].is_object() {
                    return Ok(Outcome { invocation_id: id, output: snapshot["output"].take() });
                }
            }
            return Err(Problem::new("DEPENDENCY_UNAVAILABLE", format!("模型调用未成功完成（{state}）")));
        }
        if time::OffsetDateTime::now_utc() + time::Duration::milliseconds(wait.as_millis() as i64) > deadline {
            return Err(Problem::new("DEADLINE_EXCEEDED", "模型调用超过期限"));
        }
        tokio::time::sleep(wait).await;
        wait = (wait * 3 / 2).min(Duration::from_secs(2));
        snapshot = upstream.request("model-gateway", Method::GET, &format!("/v1/inferences/{id}"), None, op, None, short).await?;
    }
}
