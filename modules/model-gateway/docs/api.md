# 统一模型网关 HTTP契约

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| PUT `/v1/admin/providers/{providerId}` | `putProvider` | `model-gateway.PutProvider` | `superadmin` | `PutProviderInput` → 200 `PutProviderResult` | CAS保存供应方 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/admin/providers/{providerId}/credentials` | `putCredential` | `model-gateway.PutCredential` | `superadmin` | `PutCredentialInput` → 201 `PutCredentialResult` | 加密存储密钥 | `HTTP 400`、`HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/admin/providers/{providerId}/credentials/rotate` | `rotateCredential` | `model-gateway.RotateCredential` | `superadmin` | `RotateCredentialInput` → 200 `RotateCredentialResult` | 轮转新请求使用的密钥 | `HTTP 400`、`HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/admin/models/{modelId}` | `putModel` | `model-gateway.PutModel` | `superadmin` | `PutModelInput` → 200 `PutModelResult` | 保存不可变模型身份版本 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/admin/profiles/{profileId}` | `putProfile` | `model-gateway.PutProfile` | `superadmin` | `PutProfileInput` → 200 `PutProfileResult` | 管理AI档位与费用限制 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/inferences` | `createInference` | `model-gateway.CreateInvocation` | `service` | `CreateInvocationInput` → 202 `CreateInvocationResult` | 接受统一AI调用 | `HTTP 400`、`HTTP 409`、`HTTP 403`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/inferences/{invocationId}` | `readInference` | `model-gateway.GetInvocation` | `service` | `—` → 200 `GetInvocationResult` | 读取调用状态 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/inferences/{invocationId}/cancel` | `cancelInference` | `model-gateway.CancelInvocation` | `service` | `CancelInvocationInput` → 200 `CancelInvocationResult` | 贯穿取消调用 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/inferences/{invocationId}/events` | `resumeInference` | `model-gateway.ResumeStream` | `service` | `—` → 200 `string` | 重连读取原流 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/admin/invocations/{invocationId}/reconcile` | `reconcileCharge` | `model-gateway.ReconcileCharge` | `model.manage` | `ReconcileChargeInput` → 200 `ReconcileChargeResult` | 证据驱动完成未知费用对账 | `HTTP 422`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/admin/ledger` | `getLedger` | `model-gateway.GetLedger` | `model.manage` | `—` → 200 `GetLedgerResult` | 管理端查询费用账本 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/admin/prices/{modelId}` | `getPriceBook` | `model-gateway.GetPriceBook` | `model.manage` | `—` → 200 `GetPriceBookResult` | 读取历史固定价目 | `HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/usage` | `queryUsage` | `model-gateway.QueryUsage` | `service` | `—` → 200 `QueryUsageResult` | 跨服务读取权威费用状态 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/models` | `readModels` | `model-gateway.ReadModels` | `service` | `—` → 200 `ReadModelsResult` | 读取模型能力声明 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/profiles` | `readProfiles` | `model-gateway.ReadProfiles` | `service` | `—` → 200 `ReadProfilesResult` | 读取档位声明 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/barriers` | `acquireBarrier` | `model-gateway.AcquireBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得本地维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `model-gateway.ReleaseBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 释放已授权维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `model-gateway.ExportSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出安全watermark快照 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| PUT `/v1/task-assignments/{taskId}` | `putTaskAssignment` | `model-gateway.PutTaskAssignment` | `superadmin` | `PutTaskAssignmentInput` → 200 `PutTaskAssignmentResult` | 超管维护单任务模型覆盖 | `HTTP 400`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/task-assignments` | `readTaskAssignments` | `model-gateway.ReadTaskAssignments` | `service` | `—` → 200 `ReadTaskAssignmentsResult` | 读取任务模型映射 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/catalog:refresh` | `refreshModelCatalog` | `model-gateway.RefreshModelCatalog` | `superadmin` | `RefreshModelCatalogInput` → 200 `RefreshModelCatalogResult` | 刷新公开模型目录 | `HTTP 503`、`HTTP 422`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/providers/{providerId}:probe` | `createConnectionProbe` | `model-gateway.CreateConnectionProbe` | `superadmin` | `CreateConnectionProbeInput` → 202 `CreateConnectionProbeResult` | 显式测试连接 | `HTTP 403`、`HTTP 503`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/cost-quotes` | `quoteCost` | `model-gateway.QuoteCost` | `service` | `QuoteCostInput` → 200 `QuoteCostResult` | 计算可保守预留的最大费用 | `HTTP 400`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/budget-policy` | `readFeeBudgetPolicy` | `model-gateway.ReadFeeBudgetPolicy` | `guard.manage` | `—` → 200 `ReadFeeBudgetPolicyResult` | 读取费用限额配置 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| PUT `/v1/budget-policy` | `putFeeBudgetPolicy` | `model-gateway.PutFeeBudgetPolicy` | `guard.manage` | `PutFeeBudgetPolicyInput` → 200 `PutFeeBudgetPolicyResult` | 维护old费用开关与会话小时、网络5倍配置 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/healthz` | `health` | `model-gateway.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `model-gateway.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `model-gateway.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `model-gateway.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
| GET `/v1/admin/catalog` | `readAdminCatalog` | `model-gateway.ReadAdminCatalog` | `model.manage` | `—` → 200 `ReadAdminCatalogResult` | 读取渠道、模型、档位与任务分配 | `FORBIDDEN` |
| POST `/v1/task-availability:read` | `readTaskAvailability` | `model-gateway.ReadTaskAvailability` | `service` | `ReadTaskAvailabilityInput` → 200 `ReadTaskAvailabilityResult` | 免费读取QA任务配置能力 | `HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 503` |
<!-- openapi:end -->

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；备份恢复只用barrier/export/stage/discard，切换在进程重启时完成（ADR0003）。
