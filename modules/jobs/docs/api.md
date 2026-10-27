# jobs HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext由可信控制通道转发，不能由匿名请求自声明。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/v1/jobs` | `listJobs` | `jobs.ListJobs` | `job.manage` | `—` → 200 `ListJobsResult` | 查看任务队列 | `FORBIDDEN`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`LOCKED`、`INTERNAL` |
| POST `/v1/jobs` | `createJob` | `jobs.CreateJob` | `service` | `CreateJobRequestBody` → 202 `CreateJobResult` | 持久提交任务 | `CONFLICT` |
| POST `/v1/jobs/{jobId}:cancel` | `cancelJob` | `jobs.RequestCancel` | `job.manage` | `RequestCancelRequestBody` → 200 `RequestCancelResult` | 请求取消任务 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/jobs/{jobId}` | `readJob` | `jobs.ReadJob` | `job.manage` | `—` → 200 `ReadJobResult` | 读取当前管理授权或本域所属任务状态 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/jobs/{jobId}/progress` | `streamProgress` | `jobs.StreamProgress` | `job.manage` | `—` → 200 `string` | 任务进度SSE | `FORBIDDEN`、`VALIDATION`、`VERSION_MISMATCH`、`UNAUTHENTICATED`、`NOT_FOUND`、`RATE_LIMITED`、`INTEGRITY_ERROR`、`INTERNAL`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/schedules` | `listSchedules` | `jobs.ListSchedules` | `job.manage` | `—` → 200 `ListSchedulesResult` | 查看定时任务 | `FORBIDDEN` |
| POST `/v1/schedules` | `createSchedule` | `jobs.CreateSchedule` | `job.manage` | `CreateScheduleRequestBody` → 200 `CreateScheduleResult` | 创建定时任务 | `FORBIDDEN`、`VALIDATION` |
| PUT `/v1/schedules/{scheduleId}` | `updateSchedule` | `jobs.UpdateSchedule` | `job.manage` | `UpdateScheduleRequestBody` → 200 `UpdateScheduleResult` | 修改定时CAS | `CONFLICT`、`FORBIDDEN` |
| GET `/healthz` | `health` | `jobs.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `jobs.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/deliveries/{deliveryId}:replay` | `replayDelivery` | `jobs.ReplayDelivery` | `job.manage` | `ReplayDeliveryRequestBody` → 200 `ReplayDeliveryResult` | 重放死信投递 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`VALIDATION`、`DEPENDENCY_UNAVAILABLE`、`UNSUPPORTED_CAPABILITY`、`INTEGRITY_ERROR` |
| POST `/v1/jobs/{jobId}:retry` | `retryJob` | `jobs.RetryJob` | `job.manage` | `RetryJobInput` → 200 `RetryJobResult` | 重试失败任务 | `FORBIDDEN`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`LOCKED`、`INTERNAL`、`NOT_FOUND` |
| GET `/v1/deliveries` | `listDeliveries` | `jobs.ListDeliveries` | `job.manage` | `—` → 200 `ListDeliveriesResult` | 查看死信投递 | `FORBIDDEN`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`LOCKED`、`INTERNAL` |
<!-- openapi:end -->

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

export要求safe；discard只删本次恢复的暂存或旧目录

claimJob可选jobTypes真实SQL筛选且只本owner登记类型；用于分开父构建/子提取worker执行名额，禁止领取后本地丢弃。省略保留旧全owner语义。六个worker租约HTTP入口只绑定已严格校验输入，epoch来自可信op，维护期间仅原heartbeat/check/complete/fail收尾由原租约权威裁定。完整服务装配仍未齐。
