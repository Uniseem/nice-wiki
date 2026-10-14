# 知识入库与混合检索 HTTP契约

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| PUT `/v1/index-config` | `putIndexConfig` | `knowledge.PutIndexConfig` | `knowledge.manage` | `PutIndexConfigInput` → 200 `PutIndexConfigResult` | 更新索引配置 | `HTTP 400`、`HTTP 409`、`HTTP 422`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/builds` | `listBuilds` | `knowledge.ListBuilds` | `knowledge.manage` | `—` → 200 `ListBuildsResult` | 列入库任务 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/builds` | `createBuild` | `knowledge.CreateBuild` | `knowledge.manage` | `CreateBuildInput` → 202 `CreateBuildResult` | 人工或事件申请构建 | `HTTP 404`、`HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/builds/{buildId}` | `getBuild` | `knowledge.GetBuild` | `knowledge.manage` | `—` → 200 `GetBuildResult` | 读取入库状态 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/builds/{buildId}/cancel` | `cancelBuild` | `knowledge.CancelBuild` | `knowledge.manage` | `CancelBuildInput` → 200 `CancelBuildResult` | 取消入库链路 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/retrievals` | `searchKnowledge` | `knowledge.Retrieve` | `service` | `RetrieveInput` → 200 `RetrieveResult` | 统一混合检索 | `HTTP 503`、`HTTP 422`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/builds/{buildId}/diagnostics` | `redactDiagnostics` | `knowledge.RedactDiagnostics` | `knowledge.manage` | `—` → 200 `RedactDiagnosticsResult` | 返回安全诊断 | `HTTP 403`、`HTTP 404`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/sections:read` | `readSections` | `knowledge.ReadSections` | `service` | `ReadSectionsInput` → 200 `ReadSectionsResult` | 按引用读取证据片段 | `HTTP 403`、`HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/barriers` | `acquireBarrier` | `knowledge.AcquireBarrier` | `superadmin` | `BarrierRequest` → 200 `BarrierStatus` | 取得本地维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `knowledge.ReleaseBarrier` | `superadmin` | `BarrierRequest` → 200 `BarrierStatus` | 释放已授权维护屏障 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `knowledge.ExportSnapshot` | `superadmin` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出安全watermark快照 | `HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 504`、`HTTP 401`、`HTTP 500`、`HTTP default` |
| POST `/v1/evidence:inspect` | `inspectEvidence` | `knowledge.InspectEvidence` | `service` | `InspectEvidenceInput` → 200 `InspectEvidenceResult` | 快速检索员有限核查 | `HTTP 503`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/query-embeddings` | `createQueryEmbedding` | `knowledge.CreateQueryEmbedding` | `service` | `CreateQueryEmbeddingInput` → 200 `CreateQueryEmbeddingResult` | 统一生成并复用查询嵌入 | `HTTP 503`、`HTTP 422`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/healthz` | `health` | `knowledge.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `knowledge.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `knowledge.StageSnapshot` | `superadmin` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `knowledge.DiscardSnapshot` | `superadmin` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
<!-- openapi:end -->

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证；备份恢复只用barrier/export/stage/discard，切换在进程重启时完成（ADR0003）。
