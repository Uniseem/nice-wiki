# traffic-guard HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext由可信控制通道转发，不能由匿名请求自声明。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| POST `/v1/admissions` | `createAdmission` | `traffic-guard.HandleAdmission` | `service` | `HandleAdmissionRequestBody` → 200 `HandleAdmissionResult` | 准入 HTTP | `FORBIDDEN`、`RATE_LIMITED`、`CHALLENGE_REQUIRED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/admissions/{admissionId}:renew` | `renewAdmission` | `traffic-guard.RenewPermit` | `service` | `RenewPermitRequestBody` → 200 `RenewPermitResult` | 延长活跃并发租约 | `STALE_RESULT`、`FORBIDDEN` |
| POST `/v1/admissions/{admissionId}:release` | `releaseAdmission` | `traffic-guard.ReleasePermit` | `service` | `ReleasePermitRequestBody` → 200 `ReleasePermitResult` | 释放并发占位 | `FORBIDDEN`、`VERSION_MISMATCH` |
| POST `/v1/challenges` | `issueChallenge` | `traffic-guard.IssueChallenge` | `anonymous` | `IssueChallengeRequestBody` → 200 `IssueChallengeResult` | 签发挑战 | `RATE_LIMITED`、`VALIDATION` |
| POST `/v1/challenges/{challengeId}:verify` | `verifyChallenge` | `traffic-guard.VerifyChallenge` | `anonymous` | `VerifyChallengeRequestBody` → 200 `VerifyChallengeResult` | 核验并消费挑战 | `STALE_RESULT`、`VALIDATION` |
| GET `/v1/policies` | `getPolicy` | `traffic-guard.GetPolicy` | `guard.manage` | `—` → 200 `GetPolicyResult` | 读取生效策略 | `VERSION_MISMATCH`、`HTTP 400`、`HTTP 403`、`HTTP 404`、`HTTP 429`、`HTTP 503` |
| PUT `/v1/policies` | `updatePolicy` | `traffic-guard.UpdatePolicy` | `guard.manage` | `UpdatePolicyRequestBody` → 200 `UpdatePolicyResult` | 更新策略 CAS | `FORBIDDEN`、`CONFLICT`、`HTTP 400`、`HTTP 404`、`HTTP 429`、`HTTP 503` |
| GET `/v1/bans` | `listBans` | `traffic-guard.ListBans` | `guard.manage` | `—` → 200 `ListBansResult` | 分页禁令 | `VALIDATION`、`FORBIDDEN`、`HTTP 404`、`HTTP 409`、`HTTP 429`、`HTTP 503` |
| POST `/v1/bans` | `createBan` | `traffic-guard.CreateBan` | `guard.manage` | `CreateBanRequestBody` → 200 `CreateBanResult` | 管理员建立临时禁令 | `FORBIDDEN`、`VALIDATION`、`HTTP 404`、`HTTP 409`、`HTTP 429`、`HTTP 503` |
| DELETE `/v1/bans/{banId}` | `revokeBan` | `traffic-guard.RevokeBan` | `guard.manage` | `RevokeBanRequestBody` → 200 `RevokeBanResult` | 撤销禁令 | `FORBIDDEN`、`CONFLICT`、`HTTP 400`、`HTTP 404`、`HTTP 429`、`HTTP 503` |
| POST `/v1/admissions/{admissionId}:consume` | `consumeAdmission` | `traffic-guard.ConsumeAdmission` | `service` | `ConsumeAdmissionRequestBody` → 200 `ConsumeAdmissionResult` | QA绑定准入使用权 | `FORBIDDEN`、`STALE_RESULT` |
| POST `/v1/admissions/{admissionId}:verify` | `verifyAdmission` | `traffic-guard.VerifyAdmissionHTTP` | `service` | `VerifyAdmissionHTTPRequestBody` → 200 `VerifyAdmissionHTTPResult` | QA在线确认准入租约 | `FORBIDDEN`、`STALE_RESULT` |
| POST `/v1/question-risk` | `reportModelRisk` | `traffic-guard.ReportQuestionSignals` | `service` | `ReportQuestionSignalsRequestBody` → 200 `ReportQuestionSignalsResult` | 接收qa已有检测信号 | `VALIDATION`、`FORBIDDEN`、`HTTP 404`、`HTTP 409`、`HTTP 429`、`HTTP 503` |
| POST `/v1/session-queue` | `queueSessionAdmission` | `traffic-guard.QueueSessionAdmission` | `service` | `QueueSessionAdmissionRequestBody` → 200 `QueueSessionAdmissionResult` | 每会话1并发其余排队 | `RATE_LIMITED`、`VERSION_MISMATCH`、`HTTP 400`、`HTTP 403`、`HTTP 404`、`HTTP 503` |
| POST `/v1/barriers` | `acquireBarrier` | `traffic-guard.EnterBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 停止准入并等安全点 | `DEADLINE_EXCEEDED`、`FORBIDDEN` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `traffic-guard.ReleaseBarrier` | `service` | `ReleaseBarrierRequestBody` → 200 `ReleaseBarrierResult` | 解除风控维护 | `LOCKED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `traffic-guard.CreateSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出风控一致快照 | `LOCKED`、`INTERNAL` |
| GET `/healthz` | `health` | `traffic-guard.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `traffic-guard.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `traffic-guard.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `traffic-guard.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
| GET `/v1/risk-events` | `listRiskEvents` | `traffic-guard.ListRiskEvents` | `guard.manage` | `—` → 200 `ListRiskEventsResult` | 查看可疑事件 | `FORBIDDEN`、`HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 429`、`HTTP 503` |
| POST `/v1/login-attempts` | `beginLoginAttempt` | `traffic-guard.BeginLoginAttempt` | `service` | `BeginLoginAttemptInput` → 200 `BeginLoginAttemptResult` | 登录失败保护受信准入 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/login-attempts/{attemptId}:finish` | `finishLoginAttempt` | `traffic-guard.FinishLoginAttempt` | `service` | `FinishLoginAttemptInput` → 200 `FinishLoginAttemptResult` | 登录失败保护受信核验结果 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/snapshots/{snapshotId}/archive` | `readSnapshotArchive` | `traffic-guard.ReadSnapshotArchive` | `superadmin` | `—` → 200 `application/x-tar` | 受控读取已完成域快照归档 | `HTTP 403`、`HTTP 409`、`HTTP 503` |
<!-- openapi:end -->

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；stage只写restore/<restoreId>/staged，不动active；discard只删本次恢复的暂存或旧目录；切换在进程重启时完成（ADR0003）。
