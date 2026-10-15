# jobs 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 50 个：手写 27、UI 4、生成 8、配置 10、迁移 1。函数 51 个：约束 33，参考 18。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/jobs/internal/validation.go` | 手写 | 验证任务入口；同文件职责限本域以下登记函数 | `jobs.ValidateJobSpec`（参考） |
| `modules/jobs/internal/queue.go` | 手写 | 持久提交任务；同文件职责限本域以下登记函数 | `jobs.CreateJob`、`jobs.FailJob` |
| `modules/jobs/internal/leases.go` | 手写 | 领取待执行任务；同文件职责限本域以下登记函数 | `jobs.ClaimJob`、`jobs.StartJob`、`jobs.HeartbeatJob`、`jobs.CheckLease` |
| `modules/jobs/internal/results.go` | 手写 | 提交成功结果；同文件职责限本域以下登记函数 | `jobs.CompleteJob` |
| `modules/jobs/internal/retry.go` | 手写 | 固定错误重试分类；同文件职责限本域以下登记函数 | `jobs.ClassifyFailure`（参考）、`jobs.ComputeRetryAt`（参考） |
| `modules/jobs/internal/scheduler.go` | 手写 | 到期重排；同文件职责限本域以下登记函数 | `jobs.RequeueDue`（参考）、`jobs.ReapExpiredLeases`（参考）、`jobs.TickSchedules`（参考） |
| `modules/jobs/internal/cancellation.go` | 手写 | 请求取消任务；同文件职责限本域以下登记函数 | `jobs.RequestCancel`、`jobs.ConfirmCancelled`（参考） |
| `modules/jobs/internal/query.go` | 手写 | 读取管理任务状态；同文件职责限本域以下登记函数 | `jobs.ReadJob` |
| `modules/jobs/internal/progress.go` | 手写 | 持久进度安全点；同文件职责限本域以下登记函数 | `jobs.AppendProgress`（参考） |
| `modules/jobs/internal/http_progress.go` | 手写 | 任务进度SSE；同文件职责限本域以下登记函数 | `jobs.StreamProgress` |
| `modules/jobs/internal/event_validation.go` | 手写 | 验证跨域事件；同文件职责限本域以下登记函数 | `jobs.ValidateEnvelope`（参考） |
| `modules/jobs/internal/event_store.go` | 手写 | 持久接收事件；同文件职责限本域以下登记函数 | `jobs.PublishEvent` |
| `modules/jobs/internal/event_delivery.go` | 手写 | 至少一次投递；同文件职责限本域以下登记函数 | `jobs.RequeueExpiredDeliveries`（参考）、`jobs.AcknowledgeEvent`、`jobs.ReplayDelivery` |
| `modules/jobs/internal/owner_client.go` | 手写 | 构建owner主动领取响应，不发影子执行HTTP；同文件职责限本域以下登记函数 | `jobs.DispatchOwnerTask`（参考） |
| `modules/jobs/internal/cron.go` | 手写 | 严格解析定时表达式；同文件职责限本域以下登记函数 | `jobs.ValidateCron`（参考）、`jobs.NextFireAt`（参考） |
| `modules/jobs/internal/schedule_store.go` | 手写 | 创建定时任务；同文件职责限本域以下登记函数 | `jobs.CreateSchedule`、`jobs.UpdateSchedule`、`jobs.ListSchedules` |
| `modules/jobs/internal/barrier_coordinator.go` | 手写 | 等待全域安全点；同文件职责限本域以下登记函数 | `jobs.CoordinateBarrier` |
| `modules/jobs/internal/barrier.go` | 手写 | 取得jobs本域屏障；同文件职责限本域以下登记函数 | `jobs.EnterBarrier`、`jobs.ReleaseBarrier` |
| `modules/jobs/internal/retention.go` | 手写 | 清理已完成记录；同文件职责限本域以下登记函数 | `jobs.PurgeExpiredHistory`（参考） |
| `modules/jobs/internal/snapshot.go` | 手写 | 导出队列与事件一致快照；同文件职责限本域以下登记函数 | `jobs.CreateSnapshot` |
| `modules/jobs/internal/import.go` | 手写 | 验证任务快照；同文件职责限本域以下登记函数 | `jobs.StageSnapshot`、`jobs.DiscardSnapshot` |
| `modules/jobs/web/src/QueuePanel.vue` | UI | Vue3模块交互：RequestJobCancellation；同文件职责限本域以下登记函数 | `jobs.RequestJobCancellation`（参考） |
| `modules/jobs/web/src/ScheduleEditor.vue` | UI | Vue3模块交互：SaveSchedule；同文件职责限本域以下登记函数 | `jobs.SaveSchedule`（参考） |
| `modules/jobs/web/src/progress.ts` | UI | Vue3模块交互：ReconnectProgress；同文件职责限本域以下登记函数 | `jobs.ReconnectProgress`（参考） |
| `modules/jobs/internal/http_event_subscription.go` | 手写 | 已登记消费者订阅事件SSE；同文件职责限本域以下登记函数 | `jobs.SubscribeEvents` |
| `modules/jobs/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/jobs/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 | 无 |
| `modules/jobs/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 | `jobs.All`（参考） |
| `modules/jobs/cmd/jobs/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `jobs.main` |
| `modules/jobs/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `jobs.BuildApplication` |
| `modules/jobs/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `jobs.Serve`、`jobs.Shutdown`、`jobs.Health`、`jobs.Ready` |
| `modules/jobs/go.mod` | 配置 | 独立module nice-wiki/modules/jobs；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/jobs/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/jobs/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/jobs；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/jobs/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/jobs与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/jobs/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/jobs/web/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/jobs/web/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/jobs/web/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/jobs/web/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/jobs/web/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/jobs/web/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `jobs.mountModuleUI` |
| `modules/jobs/config/event-subscriptions.json` | 生成 | 事件订阅目录：由全部模块contracts/events.json的consume条目生成（事件名/版本→消费模块与处理函数），jobs启动时加载；未登记订阅拒绝 | 无 |
| `modules/jobs/config/job-types.json` | 生成 | 任务类型目录：由全部模块implementation.json的jobTypes生成（类型→owner、执行函数、系统默认schedule）；jobs启动时加载，未登记类型拒绝创建 | 无 |
| `modules/jobs/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/jobs/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/jobs/internal/gen/job_types.gen.go` | 生成 | make gen由全部模块implementation.json的jobTypes生成任务类型Go表，与config/job-types.json同源；ValidateJobSpec只读此表，不手改 | 无 |
| `modules/jobs/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/jobs/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
| `modules/jobs/internal/queries.go` | 手写 | 查看任务队列 | `jobs.ListJobs`、`jobs.RetryJob`、`jobs.ListDeliveries` |
