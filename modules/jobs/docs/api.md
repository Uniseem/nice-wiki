# jobs HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext由可信控制通道转发，不能由匿名请求自声明。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/v1/jobs` | `listJobs` | `jobs.ListJobs` | `job.manage` | `—` → 200 `ListJobsResult` | 查看任务队列 | `FORBIDDEN`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`LOCKED`、`INTERNAL` |
| POST `/v1/jobs` | `createJob` | `jobs.CreateJob` | `service` | `CreateJobRequestBody` → 202 `CreateJobResult` | 持久提交任务 | `CONFLICT` |
| POST `/v1/jobs:claim` | `claimJob` | `jobs.ClaimJob` | `service` | `ClaimJobRequestBody` → 200 `ClaimJobResult` | 领取待执行任务 | `VERSION_MISMATCH` |
| POST `/v1/jobs/{jobId}:heartbeat` | `heartbeatJob` | `jobs.HeartbeatJob` | `service` | `HeartbeatJobRequestBody` → 200 `HeartbeatJobResult` | 维持租约 | `STALE_RESULT` |
| POST `/v1/jobs/{jobId}:complete` | `completeJob` | `jobs.CompleteJob` | `service` | `CompleteJobRequestBody` → 200 `CompleteJobResult` | 提交成功结果 | `STALE_RESULT`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/jobs/{jobId}:fail` | `failJob` | `jobs.FailJob` | `service` | `FailJobRequestBody` → 200 `FailJobResult` | 失败写回 | `STALE_RESULT`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/jobs/{jobId}:cancel` | `cancelJob` | `jobs.RequestCancel` | `job.manage` | `RequestCancelRequestBody` → 200 `RequestCancelResult` | 请求取消任务 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/jobs/{jobId}` | `readJob` | `jobs.ReadJob` | `job.manage` | `—` → 200 `ReadJobResult` | 读取当前管理授权或本域所属任务状态 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE`、`CONFLICT`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/jobs/{jobId}/progress` | `streamProgress` | `jobs.StreamProgress` | `job.manage` | `—` → 200 `string` | 任务进度SSE | `FORBIDDEN`、`VALIDATION`、`VERSION_MISMATCH`、`UNAUTHENTICATED`、`NOT_FOUND`、`RATE_LIMITED`、`INTEGRITY_ERROR`、`INTERNAL`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/events` | `publishEvent` | `jobs.PublishEvent` | `service` | `PublishEventRequestBody` → 202 `PublishEventResult` | 持久接收事件 | `CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`VALIDATION`、`FORBIDDEN`、`DEPENDENCY_UNAVAILABLE`、`UNSUPPORTED_CAPABILITY`、`INTEGRITY_ERROR` |
| POST `/v1/events/{eventId}:ack` | `acknowledgeEvent` | `jobs.AcknowledgeEvent` | `service` | `AcknowledgeEventRequestBody` → 200 `AcknowledgeEventResult` | 消费者确认投递 | `FORBIDDEN`、`NOT_FOUND`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`STALE_RESULT`、`DEPENDENCY_UNAVAILABLE`、`UNSUPPORTED_CAPABILITY`、`INTEGRITY_ERROR` |
| GET `/v1/schedules` | `listSchedules` | `jobs.ListSchedules` | `job.manage` | `—` → 200 `ListSchedulesResult` | 查看定时任务 | `FORBIDDEN` |
| POST `/v1/schedules` | `createSchedule` | `jobs.CreateSchedule` | `job.manage` | `CreateScheduleRequestBody` → 200 `CreateScheduleResult` | 创建定时任务 | `FORBIDDEN`、`VALIDATION` |
| PUT `/v1/schedules/{scheduleId}` | `updateSchedule` | `jobs.UpdateSchedule` | `job.manage` | `UpdateScheduleRequestBody` → 200 `UpdateScheduleResult` | 修改定时CAS | `CONFLICT`、`FORBIDDEN` |
| POST `/v1/barriers:coordinate` | `coordinateBarrier` | `jobs.CoordinateBarrier` | `service` | `CoordinateBarrierRequestBody` → 200 `CoordinateBarrierResult` | 等待全域安全点 | `DEADLINE_EXCEEDED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/jobs/{jobId}:start` | `startJob` | `jobs.StartJob` | `service` | `StartJobRequestBody` → 200 `StartJobResult` | 确认开始执行 | `STALE_RESULT` |
| GET `/v1/jobs/{jobId}/lease` | `checkLease` | `jobs.CheckLease` | `service` | `—` → 200 `CheckLeaseResult` | 写回前检验执行权 | `STALE_RESULT` |
| GET `/v1/events:subscribe` | `subscribeEvents` | `jobs.SubscribeEvents` | `service` | `—` → 200 `string` | 已登记消费者订阅事件SSE | `FORBIDDEN`、`VERSION_MISMATCH`、`NOT_FOUND`、`VALIDATION`、`DEPENDENCY_UNAVAILABLE`、`UNSUPPORTED_CAPABILITY`、`INTEGRITY_ERROR`、`RATE_LIMITED` |
| POST `/v1/barriers` | `acquireBarrier` | `jobs.EnterBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得jobs本域屏障 | `DEADLINE_EXCEEDED` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `jobs.ReleaseBarrier` | `service` | `ReleaseBarrierRequestBody` → 200 `ReleaseBarrierResult` | 解除全域任务暂停 | `LOCKED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `jobs.CreateSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出队列与事件一致快照 | `LOCKED`、`INTERNAL` |
| GET `/healthz` | `health` | `jobs.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `jobs.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `jobs.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `jobs.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
| POST `/v1/deliveries/{deliveryId}:replay` | `replayDelivery` | `jobs.ReplayDelivery` | `job.manage` | `ReplayDeliveryRequestBody` → 200 `ReplayDeliveryResult` | 重放死信投递 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`VALIDATION`、`DEPENDENCY_UNAVAILABLE`、`UNSUPPORTED_CAPABILITY`、`INTEGRITY_ERROR` |
| POST `/v1/jobs/{jobId}:retry` | `retryJob` | `jobs.RetryJob` | `job.manage` | `RetryJobInput` → 200 `RetryJobResult` | 重试失败任务 | `FORBIDDEN`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`LOCKED`、`INTERNAL`、`NOT_FOUND` |
| GET `/v1/deliveries` | `listDeliveries` | `jobs.ListDeliveries` | `job.manage` | `—` → 200 `ListDeliveriesResult` | 查看死信投递 | `FORBIDDEN`、`VALIDATION`、`CONFLICT`、`VERSION_MISMATCH`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`LOCKED`、`INTERNAL` |
| GET `/v1/snapshots/{snapshotId}/archive` | `readSnapshotArchive` | `jobs.ReadSnapshotArchive` | `superadmin` | `—` → 200 `application/x-tar` | 受控读取已完成域快照归档 | `HTTP 403`、`HTTP 409`、`HTTP 503` |
<!-- openapi:end -->

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；stage只写restore/<restoreId>/staged，不动active；discard只删本次恢复的暂存或旧目录；切换在进程重启时完成（ADR0003）。

claimJob可选jobTypes真实SQL筛选且只本owner登记类型；用于分开父构建/子提取worker执行名额，禁止领取后本地丢弃。省略保留旧全owner语义。六个worker租约HTTP入口只绑定已严格校验输入，epoch来自可信op，维护期间仅原heartbeat/check/complete/fail收尾由原租约权威裁定。完整服务装配仍未齐。
