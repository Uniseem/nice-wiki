# 统一模型网关 HTTP契约

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| PUT `/v1/admin/providers/{providerId}` | `putProvider` | `model-gateway.PutProvider` | `superadmin` | `PutProviderInput` → 200 `PutProviderResult` | CAS保存供应方 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 503`、`HTTP 422` |
| POST `/v1/admin/providers/{providerId}/credentials` | `putCredential` | `model-gateway.PutCredential` | `superadmin` | `PutCredentialInput` → 201 `PutCredentialResult` | 加密存储密钥 | `HTTP 400`、`HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 404`、`HTTP 422` |
| POST `/v1/admin/providers/{providerId}/credentials/rotate` | `rotateCredential` | `model-gateway.RotateCredential` | `superadmin` | `RotateCredentialInput` → 200 `RotateCredentialResult` | 轮转新请求使用的密钥 | `HTTP 400`、`HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 404`、`HTTP 422` |
| PUT `/v1/admin/models/{modelId}` | `putModel` | `model-gateway.PutModel` | `superadmin` | `PutModelInput` → 200 `PutModelResult` | 保存不可变模型身份版本 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 422`、`HTTP 503` |
| PUT `/v1/admin/profiles/{profileId}` | `putProfile` | `model-gateway.PutProfile` | `superadmin` | `PutProfileInput` → 200 `PutProfileResult` | 管理AI档位与费用限制 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 422`、`HTTP 503` |
| POST `/v1/inferences` | `createInference` | `model-gateway.CreateInvocation` | `service` | `CreateInvocationInput` → 202 `CreateInvocationResult` | 接受统一AI调用 | `HTTP 400`、`HTTP 409`、`HTTP 403`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/inferences/{invocationId}` | `readInference` | `model-gateway.GetInvocation` | `service` | `—` → 200 `GetInvocationResult` | 读取调用状态 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/inferences/{invocationId}/cancel` | `cancelInference` | `model-gateway.CancelInvocation` | `service` | `CancelInvocationInput` → 200 `CancelInvocationResult` | 贯穿取消调用 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/inferences/{invocationId}/events` | `resumeInference` | `model-gateway.ResumeStream` | `service` | `—` → 200 `string` | 重连读取原流 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`GONE` |
| POST `/v1/admin/invocations/{invocationId}/reconcile` | `reconcileCharge` | `model-gateway.ReconcileCharge` | `model.manage` | `ReconcileChargeInput` → 200 `ReconcileChargeResult` | 证据驱动完成未知费用对账 | `HTTP 422`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/admin/ledger` | `getLedger` | `model-gateway.GetLedger` | `model.manage` | `—` → 200 `GetLedgerResult` | 管理端查询费用账本 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 404`、`HTTP 422`、`HTTP 429`、`HTTP 503` |
| GET `/v1/admin/prices/{modelId}` | `getPriceBook` | `model-gateway.GetPriceBook` | `model.manage` | `—` → 200 `GetPriceBookResult` | 读取历史固定价目 | `HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 400`、`HTTP 404`、`HTTP 422` |
| GET `/v1/usage` | `queryUsage` | `model-gateway.QueryUsage` | `model.manage` | `—` → 200 `QueryUsageResult` | 跨服务读取权威费用状态 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 404`、`HTTP 422`、`HTTP 429`、`HTTP 503` |
| GET `/v1/models` | `readModels` | `model-gateway.ReadModels` | `service` | `—` → 200 `ReadModelsResult` | 读取模型能力声明 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 400`、`HTTP 404`、`HTTP 429`、`HTTP 503` |
| GET `/v1/profiles` | `readProfiles` | `model-gateway.ReadProfiles` | `service` | `—` → 200 `ReadProfilesResult` | 读取档位声明 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 400`、`HTTP 404`、`HTTP 429`、`HTTP 503` |
| POST `/v1/barriers` | `acquireBarrier` | `model-gateway.AcquireBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得本地维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `model-gateway.ReleaseBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 释放已授权维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `model-gateway.ExportSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出安全watermark快照 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| PUT `/v1/task-assignments/{taskId}` | `putTaskAssignment` | `model-gateway.PutTaskAssignment` | `superadmin` | `PutTaskAssignmentInput` → 200 `PutTaskAssignmentResult` | 超管维护单任务模型覆盖 | `HTTP 400`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/task-assignments` | `readTaskAssignments` | `model-gateway.ReadTaskAssignments` | `service` | `—` → 200 `ReadTaskAssignmentsResult` | 读取任务模型映射 | `HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/catalog:refresh` | `refreshModelCatalog` | `model-gateway.RefreshModelCatalog` | `superadmin` | `RefreshModelCatalogInput` → 200 `RefreshModelCatalogResult` | 刷新公开模型目录 | `HTTP 503`、`HTTP 422`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/providers/{providerId}:probe` | `createConnectionProbe` | `model-gateway.CreateConnectionProbe` | `superadmin` | `CreateConnectionProbeInput` → 202 `CreateConnectionProbeResult` | 显式测试连接 | `HTTP 403`、`HTTP 503`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/cost-quotes` | `quoteCost` | `model-gateway.QuoteCost` | `service` | `QuoteCostInput` → 200 `QuoteCostResult` | 计算可保守预留的最大费用 | `HTTP 400`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 404`、`HTTP 422` |
| GET `/v1/budget-policy` | `readFeeBudgetPolicy` | `model-gateway.ReadFeeBudgetPolicy` | `guard.manage` | `—` → 200 `ReadFeeBudgetPolicyResult` | 读取费用限额配置 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 429`、`HTTP 503` |
| PUT `/v1/budget-policy` | `putFeeBudgetPolicy` | `model-gateway.PutFeeBudgetPolicy` | `guard.manage` | `PutFeeBudgetPolicyInput` → 200 `PutFeeBudgetPolicyResult` | 维护old费用开关与会话小时、网络5倍配置 | `HTTP 400`、`HTTP 404`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default`、`HTTP 429`、`HTTP 503` |
| GET `/healthz` | `health` | `model-gateway.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `model-gateway.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `model-gateway.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `model-gateway.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
| GET `/v1/admin/catalog` | `readAdminCatalog` | `model-gateway.ReadAdminCatalog` | `model.manage` | `—` → 200 `ReadAdminCatalogResult` | 读取渠道、模型、档位与任务分配 | `FORBIDDEN`、`HTTP 400`、`HTTP 401`、`HTTP 404`、`HTTP 409`、`HTTP 429`、`HTTP 500`、`HTTP 503`、`HTTP 504` |
| POST `/v1/task-availability:read` | `readTaskAvailability` | `model-gateway.ReadTaskAvailability` | `service` | `ReadTaskAvailabilityInput` → 200 `ReadTaskAvailabilityResult` | 免费读取QA任务配置能力 | `HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 503`、`HTTP 500`、`HTTP 504` |
| POST `/v1/token-estimates` | `estimateTokens` | `model-gateway.EstimateTokens` | `service` | `EstimateTokensInput` → 200 `EstimateTokensResult` | 免费保守估算文本token | `HTTP 400`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/tokenizations` | `tokenizeText` | `model-gateway.TokenizeText` | `service` | `TokenizeTextInput` → 200 `TokenizeTextResult` | 使用固定模型分词声明离线分词 | `HTTP 400`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
<!-- openapi:end -->

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；备份恢复只用barrier/export/stage/discard，切换在进程重启时完成（ADR0003）。

## 免费配置读取边界

readTaskAvailability仅允许QA纯服务与固定三任务，不接受浏览器委托或管理入口扩大范围。响应按请求顺序逐项，仅taskId/available/reason，Cache-Control为no-store；严格代次、存储未知或损坏分别失败关闭。available是配置元数据具备，不证明供应方在线、凭据可解密、费用额度或执行授权；缺价格不阻断。

## 任务赋值入口

putTaskAssignment的taskId路径必须与assignment.taskId一致；非空Idempotency-Key头必须等于正文idempotencyKey。assignment.tier固定目录defaultTier，版本为expectedVersion+1。统一入口处理严格schema、服务身份和管理员即时授权，真实写入不触发模型。

readTaskAssignments对纯服务按根目录owner限制；portal携超管委托可读全部，并在本域HTTP map调用平台identity即时授权。显式跨owner FORBIDDEN、未知目录任务VALIDATION；结果只含已存赋值，未配置返回空数组，所有响应no-store。

## 档位配置写入

putProfile仅portal携当前superadmin委托；统一入口严格schema及即时授权，HTTP绑定核profileId路径/正文与非空幂等头/正文一致。费用类别字段表示要编辑的配置目标，不覆盖可信操作费用类别。保存空数组合法、最多256个有序唯一模型，直接能力来自根唯一目录生成表；模型未启用/能力不足或缺模型为VALIDATION，槽占用与幂等输入冲突为CONFLICT，未知存储失败关闭。所有响应no-store，不触发模型或付费重建。

## 凭据配置写入

putCredential/rotateCredential仅portal当前superadmin委托；HTTP核providerId路径/正文和非空幂等头/正文一致，响应no-store。Put返回201及新版本/掩码指纹，Rotate返回200及新/旧版本；secret超过8个Unicode字符才显示末4位，其余只掩码及HMAC摘要。expectedVersion指凭据当前版本；Rotate至少1，成功结果版本必须仍在公共整数范围内。

同代同HMAC的7天原回执先返回；新操作才核屏障及真实provider双版本CAS。未配主密钥/存储未知失败关闭，缺供应方NOT_FOUND，凭据历史/active材料损坏INTEGRITY_ERROR，不探测供应方或调用模型。Rotate版本切换已写，最后引用释放销毁尚未完成，不构成完整生命周期验收。

## 管理读取与费用策略入口

readProfiles/readModels允许根任务目录owner纯服务，portal管理员通过映射即时model.manage。profiles纯服务只读可信op.costClass，未知费用类拒绝；管理员全量。目录固定同代SQL快照、稳定ID排序，不静默截断响应上限。ReadAdminCatalog使用ProviderSummary四目录真实投影，最近probe权威生产者与投影仍未交付；ReadModels完整space分支亦未齐，两个整函数保持partial。

readFeeBudgetPolicy/putFeeBudgetPolicy仅portal当前guard.manage管理员，全部no-store。Put核非空Idempotency-Key与正文一致；首次expectedVersion0或已配置dailyQaLimit变化另当前superadmin，普通账号保留日额度仅改小时设置。无policy返回NOT_FOUND，不合成默认美元。公共边界当前guard.manage授权先于所有原回放。

## 免费固定分词

POST /v1/tokenizations仅knowledge纯服务。输入modelId与原始text，最多1048576 Unicode字符及4MiB字节；返回同代模型完整embeddingSpace、原文textHash、真实tokenCount与byteTokenEnds。结束偏移可落在Unicode字符内部，消费者保留原token序号选择合法字符边界。模型preprocessorVersion须明确为`none;tiktoken-go@v0.7.0;cl100k_base`或`none;tiktoken-go@v0.7.0;o200k_base`；未声明/不支持失败，不推测模型、按字符替代或联网。声明属于既有不可变空间身份，改声明须新模型ID。该操作不创建invocation、不收取费用、不存正文。
