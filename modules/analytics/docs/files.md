# analytics 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 71 个：手写 49、UI 5、生成 5、配置 10、迁移 2。函数 147 个：约束 33，参考 114。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/analytics/internal/beacon_validation.go` | 手写 | 严格匿名输入校验；同文件职责限本域以下登记函数 | `analytics.ValidateBeacon`（参考） |
| `modules/analytics/internal/buckets.go` | 手写 | 固定时延桶；同文件职责限本域以下登记函数 | `analytics.BucketLatency`（参考） |
| `modules/analytics/internal/privacy.go` | 手写 | IP+User-Agent+当日随机盐访客匿名hash；同文件职责限本域以下登记函数 | `analytics.RedactAuditMetadata`（参考）、`analytics.CheckPrivacyFields`（参考）、`analytics.auditStateAllowed`（参考） |
| `modules/analytics/internal/http_beacons.go` | 手写 | 匿名HTTP接收；同文件职责限本域以下登记函数 | `analytics.IngestBeacon` |
| `modules/analytics/internal/beacon_store.go` | 手写 | 批量持久匿名条目；同文件职责限本域以下登记函数 | `analytics.FlushBeacons`（参考） |
| `modules/analytics/internal/aggregation.go` | 手写 | 精确重算封闭小时桶；同文件职责限本域以下登记函数 | `analytics.AggregateHourly`、`analytics.AggregateDaily`、`analytics.enqueueAggregate`（参考）、`analytics.readAggregate`（参考）、`analytics.saveAggregate`（参考） |
| `modules/analytics/internal/query_validation.go` | 手写 | 规范查询范围；同文件职责限本域以下登记函数 | `analytics.ValidateDateRange`（参考）、`analytics.pageInterval`（参考） |
| `modules/analytics/internal/queries.go` | 手写 | 访问汇总查询；同文件职责限本域以下登记函数 | `analytics.QueryPageStatistics`、`analytics.QueryAudits`、`analytics.ReadAudit`、`analytics.readPageSnapshot`（参考） |
| `modules/analytics/internal/audit_validation.go` | 手写 | 校验审计allowlist；同文件职责限本域以下登记函数 | `analytics.ValidateAudit`（参考） |
| `modules/analytics/internal/audit_chain.go` | 手写 | 计算审计完整性链；同文件职责限本域以下登记函数 | `analytics.ComputeAuditHash`（参考） |
| `modules/analytics/internal/audit_store.go` | 手写 | 追加安全审计；同文件职责限本域以下登记函数 | `analytics.AppendAudit`、`analytics.appendTrustedAudit`（参考）、`analytics.insertAuditRecord`（参考） |
| `modules/analytics/internal/usage_store.go` | 手写 | 更新派生费用行；同文件职责限本域以下登记函数 | `analytics.ValidateUsageEvent`（参考）、`analytics.ProjectUsage`（参考）、`analytics.ProjectUsageTx`（参考）、`analytics.recordUsageFact`（参考）、`analytics.projectUsageLedger`（参考）、`analytics.projectUsageObservation`（参考） |
| `modules/analytics/internal/usage_aggregation.go` | 手写 | 费用派生聚合；同文件职责限本域以下登记函数 | `analytics.AggregateUsage` |
| `modules/analytics/internal/gateway_client.go` | 手写 | 取得权威账本投影输入；同文件职责限本域以下登记函数 | `analytics.FetchUsagePage`（参考）、`analytics.WithUsageGateway`（参考）、`analytics.readUsageGatewayPage`（参考）、`analytics.validateUsageGatewayPage`（参考） |
| `modules/analytics/internal/reconciliation.go` | 手写 | 对账修复派生统计；同文件职责限本域以下登记函数 | `analytics.ReconcileUsage` |
| `modules/analytics/internal/event_handler.go` | 手写 | 消费安全审计和费用事件；同文件职责限本域以下登记函数 | `analytics.ConsumeAnalyticsEvent`、`analytics.ProjectQaTurn`、`analytics.applyQaCounts`（参考） |
| `modules/analytics/internal/csv.go` | 手写 | 安全CSV单元格；同文件职责限本域以下登记函数 | `analytics.ValidateCSVCell`（参考） |
| `modules/analytics/internal/retention.go` | 手写 | 清理短期匿名行；同文件职责限本域以下登记函数 | `analytics.PurgeBeacons`（参考）、`analytics.PurgeAuditHistory`（参考）、`analytics.PruneDaySalts`（参考） |
| `modules/analytics/internal/outbox.go` | 手写 | 派生事件outbox派发；同文件职责限本域以下登记函数 | `analytics.PublishAggregateEvents` |
| `modules/analytics/internal/barrier.go` | 手写 | 进入analytics一致屏障；同文件职责限本域以下登记函数 | `analytics.EnterBarrier`、`analytics.ReleaseBarrier` |
| `modules/analytics/internal/snapshot.go` | 手写 | 导出统计快照；同文件职责限本域以下登记函数 | `analytics.CreateSnapshot` |
| `modules/analytics/internal/import.go` | 手写 | 校验统计恢复；同文件职责限本域以下登记函数 | `analytics.StageSnapshot`、`analytics.DiscardSnapshot` |
| `modules/analytics/web/src/OverviewPanel.vue` | UI | Vue3模块交互：LoadStatistics；同文件职责限本域以下登记函数 | `analytics.clearOverview`（参考）、`analytics.loadOverview`（参考）、`analytics.exportOverviewCSV`（参考）、`analytics.downloadOverviewCSV`（参考） |
| `modules/analytics/web/src/AuditExplorer.vue` | UI | Vue3模块交互：LoadAuditPage；同文件职责限本域以下登记函数 | `analytics.clearAudit`（参考）、`analytics.loadAudit`（参考）、`analytics.readAuditDetail`（参考）、`analytics.exportAuditCSV`（参考）、`analytics.downloadAuditCSV`（参考） |
| `modules/analytics/web/src/client.ts` | UI | Vue3模块交互：DownloadAuthorizedExport；同文件职责限本域以下登记函数 | `analytics.DownloadAuthorizedExport`（参考）、`analytics.LoadStatistics`（参考）、`analytics.LoadAuditPage`（参考）、`analytics.problemCode`（参考）、`analytics.isAccessFailure`（参考）、`analytics.problemText`（参考）、`analytics.moneyText`（参考）、`analytics.utcInput`（参考）、`analytics.createAnalyticsClient`（参考）、`analytics.FetchAuditRecord`（参考）、`analytics.RequestExport`（参考） |
| `modules/analytics/internal/crawler.go` | 手写 | 内存识别已知爬虫；同文件职责限本域以下登记函数 | `analytics.ClassifyCrawler`（参考） |
| `modules/analytics/internal/geoip.go` | 手写 | 离线GeoIP国家；同文件职责限本域以下登记函数 | `analytics.LookupCountry`（参考）、`analytics.trustedPeer`（参考） |
| `modules/analytics/internal/crawler_store.go` | 手写 | AI Markdown访问独立计数；同文件职责限本域以下登记函数 | `analytics.RecordCrawlerAccess`、`analytics.readReceipt`（参考）、`analytics.saveReceipt`（参考） |
| `modules/analytics/internal/content_gaps.go` | 手写 | 热门文档与答不出问题联动；同文件职责限本域以下登记函数 | `analytics.QueryContentGaps` |
| `modules/analytics/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/analytics/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 | 无 |
| `modules/analytics/cmd/analytics/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `analytics.main` |
| `modules/analytics/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `analytics.BuildApplication`、`analytics.NewRuntimeClient`（参考）、`analytics.readSiteIdentity`（参考） |
| `modules/analytics/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `analytics.Serve`、`analytics.Shutdown`、`analytics.Health`、`analytics.Ready`、`analytics.NewApplication`（参考）、`analytics.server.QueryAudits`（参考）、`analytics.server.ReadAudit`（参考）、`analytics.server.QueryPageStatistics`（参考）、`analytics.server.AppendAudit`（参考）、`analytics.server.IngestTrustedBeacon`（参考）、`analytics.server.RecordCrawlerAccess`（参考）、`analytics.server.QueryContentGaps`（参考）、`analytics.server.QueryUsageStatistics`（参考）、`analytics.server.CreateExport`（参考）、`analytics.server.CreateAuditExport`（参考）、`analytics.server.createExport`（参考）、`analytics.server.DownloadStatisticsExport`（参考）、`analytics.server.DownloadAuditExport`（参考）、`analytics.server.downloadExport`（参考）、`analytics.server.ReadSnapshotArchive`（参考） |
| `modules/analytics/go.mod` | 配置 | 独立module nice-wiki/modules/analytics；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/analytics/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/analytics/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/analytics；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/analytics/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/analytics与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/analytics/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/analytics/web/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/analytics/web/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/analytics/web/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/analytics/web/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/analytics/web/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/analytics/web/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `analytics.mountModuleUI`、`analytics.unmountAnalytics`（参考） |
| `modules/analytics/internal/worker.go` | 手写 | 主动领取本域持久任务的循环 | `analytics.RunWorker` |
| `modules/analytics/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/analytics/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/analytics/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/analytics/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
| `modules/analytics/internal/store.go` | 手写 | 本域真实持久与只读查询支持。 | `analytics.WithRuntime`（参考）、`analytics.OpenStore`（参考）、`analytics.Runtime.Close`（参考）、`analytics.runtimeFrom`（参考）、`analytics.storageError`（参考）、`analytics.syncDirectory`（参考）、`analytics.beginWrite`（参考）、`analytics.advanceWatermark`（参考） |
| `modules/analytics/internal/types.go` | 手写 | 与业务合同一致的本域内部DTO，HTTP仍以生成schema为准。 | 无 |
| `modules/analytics/migrations/schema.go` | 手写 | 只内嵌本域V1真实迁移，不执行外部脚本。 | 无 |
| `modules/analytics/internal/audit_events.go` | 手写 | 安全审计历史事实与事件共享实现。 | `analytics.normalizeAuditFact`（参考）、`analytics.ConsumeAuditEvent` |
| `modules/analytics/migrations/0002_origins_and_periods.sql` | 迁移 | V2来源事实/hash版本与计数策略/period水位；旧V1 hash不改。 | 无 |
| `modules/analytics/internal/beacon_privacy.go` | 手写 | 可信日界线绑定与不进入快照的独立随机日盐 | `analytics.HashDayToken`（参考）、`analytics.BindReportingTimeZone`（参考）、`analytics.loadDaySalt`（参考） |
| `modules/analytics/internal/aggregate_jobs.go` | 手写 | 本域统计任务输入/装配参考支持 | `analytics.SchedulePageAggregates`（参考）、`analytics.ensureAggregateIntent`（参考）、`analytics.submitAggregateIntents`（参考）、`analytics.backgroundOperation`（参考）、`analytics.aggregateJobOperation`（参考）、`analytics.recordAggregateLease`（参考）、`analytics.cancelAggregateLease`（参考）、`analytics.checkAggregateLease`（参考）、`analytics.verifyAggregateLease`（参考）、`analytics.executePageAggregate`（参考）、`analytics.admitAggregateScheduling`（参考）、`analytics.BackgroundOperation`（参考） |
| `modules/analytics/internal/background.go` | 手写 | 本域统计任务输入/装配参考支持 | `analytics.RunPageBackground`（参考）、`analytics.DrainBeacons`（参考）、`analytics.RunRetentionBackground`（参考）、`analytics.runAnalyticsPeriodic`（参考）、`analytics.launchPageLoop`（参考） |
| `modules/analytics/migrations/0003_aggregate_jobs.sql` | 手写 | 连续V3不可变业务任务输入与真实租约镜像 | 无 |
| `modules/analytics/internal/consumer.go` | 手写 | 统一jobs订阅装配；未具备费用精确协议分支保留重投 | `analytics.ConsumeEvents` |
| `modules/analytics/migrations/0004_qa_projection.sql` | 手写 | V4问答来源修订/日分类；旧非空未知来源汇总拒绝静默迁移 | 无 |
| `modules/analytics/internal/exports.go` | 手写 | 本域私有导出/保留期参考支持 | `analytics.CreateExport`、`analytics.writeCSVRecord`（参考）、`analytics.streamExportRows`（参考）、`analytics.installPreparedExport`（参考）、`analytics.verifyExportFile`（参考）、`analytics.OpenExport` |
| `modules/analytics/internal/usage_queries.go` | 手写 | 本域私有导出/保留期参考支持 | `analytics.QueryUsageStatistics` |
| `modules/analytics/internal/export_recovery.go` | 手写 | 本域私有导出/保留期参考支持 | `analytics.PurgeExports`（参考）、`analytics.RecoverExports`（参考）、`analytics.ExportKind`（参考）、`analytics.exportObjectNameValid`（参考） |
| `modules/analytics/migrations/0005_export_journal.sql` | 手写 | V5导出原主体/类型/代次/准备安装journal；旧未知记录拒绝静默迁移 | 无 |
| `modules/analytics/web/src/style.css` | UI | 固定批准版本的独立Vue库构建/样式，不含凭据 | 无 |
| `modules/analytics/internal/snapshot_archive.go` | 手写 | 受控读取已完成域快照归档流 | `analytics.ReadSnapshotArchive` |
| `modules/analytics/internal/usage_gateway_types.go` | 手写 | model-gateway V2安全统计页、完整范围和可信注入窄接口的本域无损类型；HTTP机器来源仍引用生产方types | 无 |
| `modules/analytics/internal/usage_events.go` | 手写 | 费用事件原引用解析与权威页绑定；不接受事件金额或修订为账项真相 | `analytics.parseUsageEventReference`（参考）、`analytics.validateUsageEventReference`（参考） |
| `modules/analytics/internal/usage_models.go` | 手写 | model-gateway V2费用账项、观察和投影输入的单一本域无损事实模型 | 无 |
| `modules/analytics/migrations/0006_usage_statistics_v2.sql` | 手写 | 连续V6完整费用事实、账项/观察当前投影；不改写V1旧费用表 | 无 |
