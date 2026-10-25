# 原生 Agent

内部 API 仅 qa 可调用，运行归属绑定站点、代次、调用服务和根操作。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/healthz` | `health` | `agent.Health` | `anonymous` | `—` → 200 `HealthStatus` | 进程存活 | `HTTP default` |
| GET `/readyz` | `ready` | `agent.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 原生运行时就绪 | `HTTP default` |
| POST `/v1/runs` | `startAgentRun` | `agent.StartRun` | `service` | `AgentInput` → 200 `AgentStarted` | 启动独立原生会话 | `HTTP default` |
| GET `/v1/runs/{runId}/events` | `readAgentEvents` | `agent.ReadEvents` | `service` | `—` → 200 `AgentEvents` | 读取模型及工具请求 | `HTTP default` |
| POST `/v1/runs/{runId}/replies/{requestId}` | `replyAgentRequest` | `agent.Reply` | `service` | `AgentReply` → 200 `AgentAccepted` | 提交模型或工具结果 | `HTTP default` |
| POST `/v1/runs/{runId}/cancel` | `cancelAgentRun` | `agent.Cancel` | `service` | `AgentEmpty` → 200 `AgentCancelled` | 取消并释放会话 | `HTTP default` |
<!-- openapi:end -->
