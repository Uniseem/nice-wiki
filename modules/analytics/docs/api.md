# analytics HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext由可信控制通道转发，不能由匿名请求自声明。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| POST `/v1/beacons` | `ingestBeacon` | `analytics.IngestBeacon` | `anonymous` | `IngestBeaconRequestBody` → 204 `—` | 匿名HTTP接收 | `VALIDATION`、`VERSION_MISMATCH` |
| GET `/v1/statistics/pages` | `queryPageStatistics` | `analytics.QueryPageStatistics` | `stats.read` | `—` → 200 `QueryPageStatisticsResult` | 访问汇总查询 | `FORBIDDEN`、`VALIDATION` |
| GET `/v1/statistics/usage` | `queryUsageStatistics` | `analytics.QueryUsageStatistics` | `stats.read` | `—` → 200 `QueryUsageStatisticsResult` | 费用汇总查询 | `FORBIDDEN`、`VALIDATION` |
| GET `/v1/audits` | `queryAudits` | `analytics.QueryAudits` | `audit.read` | `—` → 200 `QueryAuditsResult` | 审计分页 | `FORBIDDEN`、`VALIDATION` |
| POST `/v1/audits` | `appendAudit` | `analytics.AppendAudit` | `service` | `AppendAuditRequestBody` → 200 `AppendAuditResult` | 追加安全审计 | `VALIDATION`、`CONFLICT` |
| GET `/v1/audits/{auditId}` | `readAudit` | `analytics.ReadAudit` | `audit.read` | `—` → 200 `ReadAuditResult` | 读取单条审计 | `FORBIDDEN`、`NOT_FOUND` |
| POST `/v1/statistics:export` | `createExport` | `analytics.CreateExport` | `stats.read` | `CreateExportRequestBody` → 200 `CreateExportResult` | 受权限保护统计导出 | `FORBIDDEN`、`VALIDATION` |
| POST `/v1/usage:reconcile` | `reconcileUsage` | `analytics.ReconcileUsage` | `system.manage` | `ReconcileUsageRequestBody` → 200 `ReconcileUsageResult` | 对账修复派生统计 | `FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/crawler-access` | `recordCrawlerAccess` | `analytics.RecordCrawlerAccess` | `service` | `RecordCrawlerAccessRequestBody` → 200 `RecordCrawlerAccessResult` | AI Markdown访问独立计数 | `FORBIDDEN`、`VALIDATION` |
| GET `/v1/statistics/content-gaps` | `queryContentGaps` | `analytics.QueryContentGaps` | `stats.read` | `—` → 200 `QueryContentGapsResult` | 热门文档与答不出问题联动 | `FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| GET `/healthz` | `health` | `analytics.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `analytics.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/internal/beacons` | `ingestTrustedBeacon` | `analytics.IngestBeacon` | `service` | `IngestBeaconInput` → 204 `—` | 可信portal匿名访问转交 | `VALIDATION`、`VERSION_MISMATCH` |
| POST `/v1/audit-exports` | `createAuditExport` | `analytics.CreateExport` | `audit.read` | `CreateAuditExportRequestBody` → 200 `CreateExportResult` | 当前审计权限私有CSV导出 | `FORBIDDEN`、`VALIDATION` |
| GET `/v1/statistics/exports/{exportId}` | `downloadStatisticsExport` | `analytics.OpenExport` | `stats.read` | `—` → 200 `text/csv` | 当前权限同主体私有CSV下载 | `HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 404`、`HTTP 409`、`HTTP 410`、`HTTP 422`、`HTTP 503` |
| GET `/v1/audit-exports/{exportId}` | `downloadAuditExport` | `analytics.OpenExport` | `audit.read` | `—` → 200 `text/csv` | 当前权限同主体私有CSV下载 | `HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 404`、`HTTP 409`、`HTTP 410`、`HTTP 422`、`HTTP 503` |
| POST `/v1/jobs/{jobId}/recovery` | `readJobRecovery` | `analytics.ReadJobRecovery` | `service` | `ReadJobRecoveryInput` → 200 `ReadJobRecoveryResult` | 读取原统计聚合任务恢复证明 | `HTTP default` |
| GET `/v1/statistics/visitors` | `listVisitors` | `analytics.ListVisitors` | `stats.read` | `—` → 200 `ListVisitorsResult` | 按唯一访客查看活动和设备信息 | `FORBIDDEN`、`VALIDATION` |
<!-- openapi:end -->

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；stage只写restore/<restoreId>/staged，不动active；discard只删本次恢复的暂存或旧目录；切换在进程重启时完成（ADR0003）。

`POST /v1/internal/beacons`仅portal纯服务签名完整IngestBeaconInput，可信IP/UA只内存；正文epoch必须等于op，成功统一204。公共`/v1/beacons`直接到本域仍缺可信元数据失败关闭。

导出：POST `/v1/statistics:export` (pages|usage, stats.read)，POST `/v1/audit-exports` (audits,audit.read)。对应GET `/v1/statistics/exports/{exportId}` 和 `/v1/audit-exports/{exportId}`，仅当前相同权限与原主体/代次、24小时内完整CSV流；未知kind不默认pages。
