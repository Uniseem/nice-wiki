# 问答、引用与回答缓存 HTTP契约

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| POST `/v1/conversations` | `createConversation` | `qa.CreateConversation` | `anonymous` | `CreateConversationInput` → 201 `CreateConversationResult` | 创建伪名访客会话 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/conversations/{conversationId}` | `getConversation` | `qa.GetConversation` | `anonymous` | `—` → 200 `GetConversationResult` | 读取自有访客历史 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`GONE` |
| DELETE `/v1/conversations/{conversationId}` | `deleteConversation` | `qa.DeleteConversation` | `anonymous` | `—` → 200 `DeleteConversationResult` | 删除自有对话数据 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`GONE` |
| POST `/v1/turns` | `startTurn` | `qa.StartTurn` | `anonymous` | `StartTurnInput` → 202 `StartTurnResult` | 准入后开始问答轮次 | `HTTP 429`、`HTTP 403`、`HTTP 409`、`HTTP 400`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/turns/{turnId}/events` | `resumeAnswerStream` | `qa.ResumeAnswerStream` | `anonymous` | `—` → 200 `string` | SSE断线重连 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`GONE` |
| POST `/v1/turns/{turnId}/cancel` | `cancelTurn` | `qa.CancelTurn` | `anonymous` | `CancelTurnInput` → 200 `CancelTurnResult` | 取消问答及所有子调用 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`GONE` |
| PUT `/v1/turns/{turnId}/feedback` | `recordFeedback` | `qa.RecordFeedback` | `anonymous` | `RecordFeedbackInput` → 200 `RecordFeedbackResult` | 保存访客单条反馈 | `HTTP 403`、`HTTP 409`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/admin/policy` | `readQaPolicy` | `qa.ReadQaPolicy` | `qa.manage` | `—` → 200 `ReadQaPolicyResult` | 读取问答策略 | `FORBIDDEN`、`HTTP default` |
| PUT `/v1/admin/policy` | `putQaPolicy` | `qa.PutQaPolicy` | `qa.manage` | `PutQaPolicyInput` → 200 `PutQaPolicyResult` | 管理公开问答策略 | `HTTP 400`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/admin/debug-turns` | `createDebugTurn` | `qa.CreateDebugTurn` | `qa.debug` | `CreateDebugTurnInput` → 202 `CreateDebugTurnResult` | 隔离管理诊断 | `HTTP 403`、`HTTP 503`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/admin/debug-turns/{debugId}` | `readDebugTurn` | `qa.ReadDebugTurn` | `qa.debug` | `—` → 200 `ReadDebugTurnResult` | 读取管理诊断 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`GONE` |
| GET `/healthz` | `health` | `qa.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `qa.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| GET `/v1/admin/turns` | `listTurnRecords` | `qa.ListTurnRecords` | `qa.manage` | `—` → 200 `ListTurnRecordsResult` | 浏览与筛选问答记录 | `FORBIDDEN` |
| GET `/v1/admin/turns/{turnId}` | `readTurnRecord` | `qa.ReadTurnRecord` | `qa.manage` | `—` → 200 `ReadTurnRecordResult` | 查看问答记录 | `FORBIDDEN`、`NOT_FOUND`、`GONE` |
| GET `/v1/public-capabilities` | `readPublicCapability` | `qa.ReadPublicCapability` | `service` | `—` → 200 `ReadPublicCapabilityResult` | 免费读取公开问答入口配置能力 | `HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 503`、`HTTP default` |
| POST `/v1/search` | `searchDocuments` | `qa.SearchDocuments` | `anonymous` | `SearchDocumentsInput` → 200 `SearchDocumentsResult` | 访客全文搜索（关键词与向量混合检索） | `HTTP 429`、`HTTP 403`、`HTTP 409`、`HTTP 400`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
<!-- openapi:end -->

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；备份恢复只用barrier/export/stage/discard，切换在进程重启时完成（ADR0003）。
