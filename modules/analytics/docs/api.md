# analytics HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext和恢复许可由可信控制通道转发，不能由匿名请求自声明。

| 方法与路径 | operationId | handler ID | 权限 | 响应/行为 |
|---|---|---|---|---|
| POST `/v1/beacons` | `ingestBeacon` | `analytics.IngestBeacon` | `anonymous` | 204；严格allowlist，未知字段拒绝400，不携带IP/正文 |
| GET `/v1/statistics/pages` | `queryPageStatistics` | `analytics.QueryPageStatistics` | `stats.read` | 按UTC小时或站点时区日聚合访问 |
| GET `/v1/statistics/usage` | `queryUsageStatistics` | `analytics.QueryUsageStatistics` | `stats.read` | 派生费用汇总附gateway watermark与lastReconciledAt |
| GET `/v1/audits` | `queryAudits` | `analytics.QueryAudits` | `audit.read` | 管理操作审计游标查询，无正文/密钥 |
| GET `/v1/audits/{auditId}` | `readAudit` | `analytics.ReadAudit` | `audit.read` | 读取安全审计记录与链校验结果 |
| POST `/v1/audits` | `appendAudit` | `analytics.AppendAudit` | `service` | 已鉴权服务审计事件；不能提交任意message |
| POST `/v1/statistics:export` | `createExport` | `analytics.CreateExport` | `stats.read` | 已授权过滤范围生成CSV引用，最多10万行 |
| POST `/v1/usage:reconcile` | `reconcileUsage` | `analytics.ReconcileUsage` | `system.manage` | 只读gateway完整ledger watermark对账；不修改gateway |
| POST `/v1/crawler-access` | `recordCrawlerAccess` | `analytics.RecordCrawlerAccess` | `service` | AI爬虫.md访问独立统计，不记录IP/UA原文 |
| GET `/v1/statistics/content-gaps` | `queryContentGaps` | `analytics.QueryContentGaps` | `stats.read` | 热门文档+答不上来问题引用、QA量/缓存命中率/无答案比 |
| POST `/v1/barriers` | `acquireBarrier` | `analytics.EnterBarrier` | `service` | 公共BarrierRequest→BarrierStatus；持久draining/safe，等待本域实际安全点 |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `analytics.ReleaseBarrier` | `service` | 仅全域恢复完成或commit前全部abort确认后释放 |
| POST `/v1/snapshots:export` | `exportSnapshot` | `analytics.CreateSnapshot` | `service` | 公共SnapshotExportRequest→SnapshotExportResult；必须safe |
| POST `/v1/snapshots:validate` | `validateSnapshot` | `analytics.ValidateImport` | `service` | 公共SnapshotValidateRequest→SnapshotValidation；只写隔离报告 |
| POST `/v1/snapshots:prepare` | `prepareSnapshot` | `analytics.PrepareImport` | `service` | 公共SnapshotPrepareRequest→SnapshotStageResult；不切指针 |
| POST `/v1/snapshots:commit` | `commitSnapshot` | `analytics.ActivateImport` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；需持久commit决定 |
| POST `/v1/snapshots:rollback` | `rollbackSnapshot` | `analytics.AbortImport` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；commit后禁止 |

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；validate只读隔离；prepare不切active；commit校验签名决策；rollback只可commit前。已commit失败必须继续同一targetEpoch并保持维护。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
