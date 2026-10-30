# 知识入库与混合检索 HTTP契约

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/v1/index-config` | `readIndexConfig` | `knowledge.ReadIndexConfig` | `knowledge.manage` | `—` → 200 `ReadIndexConfigResult` | 读取索引配置 | `FORBIDDEN`、`HTTP 404` |
| PUT `/v1/index-config` | `putIndexConfig` | `knowledge.PutIndexConfig` | `knowledge.manage` | `PutIndexConfigInput` → 200 `PutIndexConfigResult` | 更新索引配置 | `HTTP 400`、`HTTP 409`、`HTTP 422`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/builds` | `listBuilds` | `knowledge.ListBuilds` | `knowledge.manage` | `—` → 200 `ListBuildsResult` | 列入库任务 | `HTTP 403`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/builds` | `createBuild` | `knowledge.CreateBuild` | `knowledge.manage` | `CreateBuildInput` → 202 `CreateBuildResult` | 人工或事件申请构建 | `HTTP 404`、`HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/v1/builds/{buildId}` | `getBuild` | `knowledge.GetBuild` | `knowledge.manage` | `—` → 200 `GetBuildResult` | 读取入库状态 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/builds/{buildId}/cancel` | `cancelBuild` | `knowledge.CancelBuild` | `knowledge.manage` | `CancelBuildInput` → 200 `CancelBuildResult` | 取消入库链路 | `HTTP 404`、`HTTP 403`、`HTTP 401`、`HTTP 409`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/retrievals` | `searchKnowledge` | `knowledge.Retrieve` | `service` | `RetrieveInput` → 200 `RetrieveResult` | 统一混合检索 | `HTTP 503`、`HTTP 422`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/sections:read` | `readSections` | `knowledge.ReadSections` | `service` | `ReadSectionsInput` → 200 `ReadSectionsResult` | 按引用读取证据片段 | `HTTP 403`、`HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/query-embeddings` | `createQueryEmbedding` | `knowledge.CreateQueryEmbedding` | `service` | `CreateQueryEmbeddingInput` → 200 `CreateQueryEmbeddingResult` | 统一生成并复用查询嵌入 | `HTTP 503`、`HTTP 422`、`HTTP 409`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| GET `/healthz` | `health` | `knowledge.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `knowledge.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| GET `/v1/index-options` | `readIndexOptions` | `knowledge.ReadIndexOptions` | `knowledge.manage` | `—` → 200 `ReadIndexOptionsResult` | 读取知识管理真实选项 | `FORBIDDEN` |
| POST `/v1/document-index:read` | `readDocumentIndex` | `knowledge.ReadDocumentIndex` | `service` | `ReadDocumentIndexInput` → 200 `ReadDocumentIndexResult` | 读取文档索引视图 | `HTTP 403`、`HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/document-chunks:read` | `readDocumentChunks` | `knowledge.ReadDocumentChunks` | `service` | `ReadDocumentChunksInput` → 200 `ReadDocumentChunksResult` | 按文档读取全部分块 | `HTTP 403`、`HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/site-overview:read` | `readSiteOverview` | `knowledge.ReadSiteOverview` | `service` | `ReadSiteOverviewInput` → 200 `ReadSiteOverviewResult` | 读取全站概括 | `HTTP 403`、`HTTP 409`、`HTTP 404`、`HTTP 401`、`HTTP 504`、`HTTP 500`、`HTTP default` |
| POST `/v1/site-overview:refresh` | `refreshSiteOverview` | `knowledge.RefreshSiteOverview` | `knowledge.manage` | `RefreshSiteOverviewInput` → 202 `RefreshSiteOverviewResult` | 立即重算全站概括 | `HTTP 404`、`HTTP 409`、`HTTP 503`、`HTTP 401`、`HTTP 403`、`HTTP 504`、`HTTP 500`、`HTTP default` |
<!-- openapi:end -->

完整字段、requestBody/query/path/header、错误HTTP状态和security见openapi.json。请求JSON上限1MiB，UTF8，Content-Type=application/json，拒绝额外字段/null。私网所有请求服务认证；管理写请求Origin/CSRF检查。HTTP202只确认持久接受，后续状态/账单以事件和读取操作为准。

SSE：id严格递增，重连从Last-Event-ID排他续读；15秒comment heartbeat；终态completed/error/cancelled后关闭。读流不重启业务；超过保留期410，不自动补跑模型。

控制API：superadmin委托和service身份双验证


全量重建费用闭包：`POST /v1/rebuild-plans`只免费估价，返回真实多组件`RebuildQuote`，不复用单一PriceSnapshot形状。`chunkCountKnown=false`时分块数只描述已知材料；`pricingKnown=false`省略`maximumCost`并给warning，前端不显示零费用。`POST /v1/rebuild-plans/{planId}:approve`必须原planId/quoteHash、有效期与管理员一致；明确勾选未知提示后仍可明批，但不跳过真实MG额度或route/price CAS。重复批准返回真实原任务数，不再创建任务。HTTP均180秒完整预算，任何超预算/缺事实错误不截断。

本域免费消费MG `POST /v1/task-routes:resolve`（`resolveTaskRoute`，taskId/costClass），严格ResolveRouteResult生产schema；Create可选expectedRouteVersion与expectedPriceSnapshots按原明批填，原请求hash/原key同时冻结。MG候选同快照提供opaque modelSnapshotHash及可选embeddingSpace，知识不复制路由或hash算法。
