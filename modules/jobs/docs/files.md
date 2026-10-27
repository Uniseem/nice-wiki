# jobs 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 46 个：手写 24、UI 1、生成 7、配置 6、迁移 8。函数 116 个：约束 28，参考 88。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/jobs/internal/validation.go` | 手写 | 验证任务入口；同文件职责限本域以下登记函数 | `jobs.ValidateJobSpec`（参考） |
| `modules/jobs/internal/queue.go` | 手写 | 持久提交任务；同文件职责限本域以下登记函数 | `jobs.CreateJob`、`jobs.FailJob`、`jobs.InstallStore`（参考）、`jobs.OpenDatabase`（参考）、`jobs.runMigrations`（参考）、`jobs.registerJobsWorkTables`（参考）、`jobs.CloseStore`（参考）、`jobs.createJob`（参考）、`jobs.hashCreateJobInput`（参考）、`jobs.NewQueueHandlers`（参考）、`jobs.ConfigureBackgroundScope`（参考）、`jobs.currentStore`（参考） |
| `modules/jobs/internal/leases.go` | 手写 | 领取待执行任务；同文件职责限本域以下登记函数 | `jobs.ClaimJob`、`jobs.StartJob`、`jobs.HeartbeatJob`、`jobs.CheckLease`、`jobs.beginLeaseOperation`（参考）、`jobs.close`（参考）、`jobs.readLease`（参考）、`jobs.claimCandidates`（参考）、`jobs.receipt`（参考）、`jobs.saveReceipt`（参考）、`jobs.enqueueState`（参考）、`jobs.leaseLost`（参考）、`jobs.countActiveLeases`（参考）、`jobs.NewLeaseHandlers`（参考） |
| `modules/jobs/internal/results.go` | 手写 | 提交成功结果；同文件职责限本域以下登记函数 | `jobs.CompleteJob`、`jobs.readJobRecord`（参考）、`jobs.terminalReplay`（参考）、`jobs.saveTerminal`（参考）、`jobs.mergePaidResults`（参考）、`jobs.finishJob`（参考） |
| `modules/jobs/internal/retry.go` | 手写 | 固定错误重试分类；同文件职责限本域以下登记函数 | `jobs.ClassifyFailure`（参考）、`jobs.ComputeRetryAt`（参考） |
| `modules/jobs/internal/scheduler.go` | 手写 | 到期重排；同文件职责限本域以下登记函数 | `jobs.RequeueDue`（参考）、`jobs.ReapExpiredLeases`（参考）、`jobs.TickSchedules`（参考）、`jobs.lastDueTick`（参考）、`jobs.collectOrphans`（参考）、`jobs.reconcileOrphan`（参考） |
| `modules/jobs/internal/cancellation.go` | 手写 | 请求取消任务；同文件职责限本域以下登记函数 | `jobs.RequestCancel`、`jobs.ConfirmCancelled`（参考） |
| `modules/jobs/internal/query.go` | 手写 | 读取管理任务状态；同文件职责限本域以下登记函数 | `jobs.ReadJob`、`jobs.authorizeJobManagement`（参考）、`jobs.authorizePermission`（参考）、`jobs.jobManagementPermission`（参考）、`jobs.authorizeJobRead`（参考） |
| `modules/jobs/internal/progress.go` | 手写 | 持久进度安全点；同文件职责限本域以下登记函数 | `jobs.AppendProgress`（参考） |
| `modules/jobs/internal/http_progress.go` | 手写 | 任务进度SSE；同文件职责限本域以下登记函数 | `jobs.StreamProgress`、`jobs.progressStream.Next`（参考）、`jobs.progressStream.Close`（参考）、`jobs.readProgressFrame`（参考）、`jobs.NewProgressHandlers`（参考）、`jobs.bindProgressCursor`（参考） |
| `modules/jobs/internal/event_validation.go` | 手写 | 验证跨域事件；同文件职责限本域以下登记函数 | `jobs.ValidateEnvelope`（参考）、`jobs.findEventDefinition`（参考）、`jobs.validateEventPayload`（参考）、`jobs.consumerHasEvent`（参考） |
| `modules/jobs/internal/event_store.go` | 手写 | 持久接收事件；同文件职责限本域以下登记函数 | `jobs.PublishEvent`、`jobs.encodeEvent`（参考）、`jobs.readEventRecord`（参考）、`jobs.requireEventWrite`（参考）、`jobs.eventReceipt`（参考）、`jobs.saveEventReceipt`（参考）、`jobs.RelayOutbox` |
| `modules/jobs/internal/event_delivery.go` | 手写 | 至少一次投递；同文件职责限本域以下登记函数 | `jobs.RequeueExpiredDeliveries`（参考）、`jobs.AcknowledgeEvent`、`jobs.ReplayDelivery`、`jobs.readDeliveryRecord`（参考）、`jobs.advanceConsumerOffset`（参考）、`jobs.authorizeDeliveryManagement`（参考） |
| `modules/jobs/internal/cron.go` | 手写 | 严格解析定时表达式；同文件职责限本域以下登记函数 | `jobs.ValidateCron`（参考）、`jobs.NextFireAt`（参考） |
| `modules/jobs/internal/schedule_store.go` | 手写 | 创建定时任务；同文件职责限本域以下登记函数 | `jobs.CreateSchedule`、`jobs.UpdateSchedule`、`jobs.ListSchedules`、`jobs.NewScheduleHandlers`（参考） |
| `modules/jobs/internal/retention.go` | 手写 | 清理已完成记录；同文件职责限本域以下登记函数 | `jobs.PurgeExpiredHistory`（参考） |
| `modules/jobs/internal/http_event_subscription.go` | 手写 | 已登记消费者订阅事件SSE；同文件职责限本域以下登记函数 | `jobs.SubscribeEvents`、`jobs.Next`（参考）、`jobs.LastID`（参考）、`jobs.Close`（参考）、`jobs.nextDelivery`（参考） |
| `modules/jobs/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/jobs/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 | 无 |
| `modules/jobs/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 | `jobs.All`（参考） |
| `modules/jobs/cmd/jobs/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `jobs.main` |
| `modules/jobs/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `jobs.BuildApplication`、`jobs.newRuntimeClient`（参考）、`jobs.ensureDataLayout`（参考）、`jobs.loadTrustAnchors`（参考）、`jobs.loadSigningKey`（参考） |
| `modules/jobs/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `jobs.Serve`、`jobs.Shutdown`、`jobs.Health`、`jobs.Ready` |
| `modules/jobs/go.mod` | 配置 | 独立module nice-wiki/modules/jobs；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/jobs/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/jobs/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/jobs；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/jobs/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/jobs与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/jobs/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/jobs/config/event-subscriptions.json` | 生成 | 事件订阅目录：由全部模块contracts/events.json的consume条目生成（事件名/版本→消费模块与处理函数），jobs启动时加载；未登记订阅拒绝 | 无 |
| `modules/jobs/config/job-types.json` | 生成 | 任务类型目录：由全部模块implementation.json的jobTypes生成（类型→owner、执行函数、系统默认schedule）；jobs启动时加载，未登记类型拒绝创建 | 无 |
| `modules/jobs/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/jobs/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/jobs/internal/gen/job_types.gen.go` | 生成 | make gen由全部模块implementation.json的jobTypes生成任务类型Go表，与config/job-types.json同源；ValidateJobSpec只读此表，不手改；追加全部events.json的唯一生产者/版本/管理权限/消费者与内嵌payload规格EventTypes，生成目录不手改 | 无 |
| `modules/jobs/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/jobs/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
| `modules/jobs/internal/queries.go` | 手写 | 查看任务队列 | `jobs.ListJobs`、`jobs.RetryJob`、`jobs.ListDeliveries`、`jobs.WithManagementKey`（参考）、`jobs.managementPage`（参考）、`jobs.finishManagementPage`（参考）、`jobs.listJobBatch`（参考）、`jobs.listDeliveryBatch`（参考）、`jobs.filterManagementBatch`（参考）、`jobs.inspectManualRetry`（参考）、`jobs.applyManualRetry`（参考）、`jobs.readRetryReceipt`（参考）、`jobs.retryRequestHash`（参考）、`jobs.preserveRetryResults`（参考）、`jobs.readJobSummary`（参考） |
| `modules/jobs/migrations/0002_leases.sql` | 迁移 | 只追加租约持久fence计数、领取/心跳幂等回执与进度checkpoint唯一键 | 无 |
| `modules/jobs/migrations/0003_terminal_jobs.sql` | 迁移 | 只追加终局回执与安全错误码；扩展租约操作回执种类，不更改既有fence；持久保存根操作，历史未知根不伪造 | 无 |
| `modules/jobs/migrations/0004_orphan_jobs.sql` | 迁移 | 只追加重建jobs状态CHECK纳入orphan；逐列保留既有任务及关联FK，登记失效租约恢复引用 | 无 |
| `modules/jobs/migrations/0005_event_deliveries.sql` | 迁移 | 只追加事件唯一元数据/hash、全局seq、持久deliveryId/确认租约/CAS、连续确认水位与事件操作回执，保留既有事件/投递 | 无 |
| `modules/jobs/internal/http_events.go` | 手写 | 已实现事件HTTP绑定组，交公共NewServiceHandler统一边界；其余handler未完成不伪造完整Application | `jobs.NewEventHandlers`（参考）、`jobs.writeEventResponse`（参考）、`jobs.NewManagementHandlers`（参考） |
| `modules/jobs/migrations/0006_job_management.sql` | 迁移 | 仅追加人工retry幂等回执；不重建jobs、不新增无生产分支failed持久态 | 无 |
| `modules/jobs/migrations/0007_progress_retention.sql` | 迁移 | 只追加nullable进度发生时刻；同job/seq/epoch及安全字段的唯一一致持久源回填，未知不猜时间 | 无 |
| `modules/jobs/migrations/0008_schedules.sql` | 迁移 | 只追加定时任务的类型、输入引用、代次、操作回执和到期索引；不重建schedules，不删除已有行 | 无 |
| `modules/jobs/internal/app/owner_recovery.go` | 手写 | 只读当前真实站点身份 | `jobs.ReadQueueSiteID`（参考）、`jobs.NewOwnerRecoveryChecker`（参考） |
| `modules/jobs/web/src/index.tsx` | UI | 任务队列：任务列表与详情（进度流断线重连）、死信投递重放、定时调度编辑 | `jobs.JobsPage` |
