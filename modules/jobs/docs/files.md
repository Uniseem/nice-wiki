# jobs 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=44；分类={"handwritten": 26, "ui": 4, "generated": 4, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/jobs/internal/validation.go` | handwritten | 验证任务入口；同文件职责限本域以下登记函数 | `M13-F001` |
| `modules/jobs/internal/queue.go` | handwritten | 持久提交任务；同文件职责限本域以下登记函数 | `M13-F002`, `M13-F010` |
| `modules/jobs/internal/leases.go` | handwritten | 领取待执行任务；同文件职责限本域以下登记函数 | `M13-F003`, `M13-F004`, `M13-F005`, `M13-F006` |
| `modules/jobs/internal/results.go` | handwritten | 提交成功结果；同文件职责限本域以下登记函数 | `M13-F007` |
| `modules/jobs/internal/retry.go` | handwritten | 固定错误重试分类；同文件职责限本域以下登记函数 | `M13-F008`, `M13-F009` |
| `modules/jobs/internal/scheduler.go` | handwritten | 到期重排；同文件职责限本域以下登记函数 | `M13-F011`, `M13-F012`, `M13-F028` |
| `modules/jobs/internal/cancellation.go` | handwritten | 请求取消任务；同文件职责限本域以下登记函数 | `M13-F013`, `M13-F014` |
| `modules/jobs/internal/query.go` | handwritten | 读取管理任务状态；同文件职责限本域以下登记函数 | `M13-F015` |
| `modules/jobs/internal/progress.go` | handwritten | 持久进度安全点；同文件职责限本域以下登记函数 | `M13-F016` |
| `modules/jobs/internal/http_progress.go` | handwritten | 任务进度SSE；同文件职责限本域以下登记函数 | `M13-F017` |
| `modules/jobs/internal/event_validation.go` | handwritten | 验证跨域事件；同文件职责限本域以下登记函数 | `M13-F018` |
| `modules/jobs/internal/event_store.go` | handwritten | 持久接收事件；同文件职责限本域以下登记函数 | `M13-F019` |
| `modules/jobs/internal/event_delivery.go` | handwritten | 至少一次投递；同文件职责限本域以下登记函数 | `M13-F020`, `M13-F021`, `M13-F022` |
| `modules/jobs/internal/owner_client.go` | handwritten | 构建owner主动领取响应，不发影子执行HTTP；同文件职责限本域以下登记函数 | `M13-F023` |
| `modules/jobs/internal/cron.go` | handwritten | 严格解析定时表达式；同文件职责限本域以下登记函数 | `M13-F024`, `M13-F025` |
| `modules/jobs/internal/schedule_store.go` | handwritten | 创建定时任务；同文件职责限本域以下登记函数 | `M13-F026`, `M13-F027` |
| `modules/jobs/internal/barrier_coordinator.go` | handwritten | 等待全域安全点；同文件职责限本域以下登记函数 | `M13-F029` |
| `modules/jobs/internal/barrier.go` | handwritten | 取得jobs本域屏障；同文件职责限本域以下登记函数 | `M13-F030`, `M13-F031` |
| `modules/jobs/internal/retention.go` | handwritten | 清理已完成记录；同文件职责限本域以下登记函数 | `M13-F032` |
| `modules/jobs/internal/snapshot.go` | handwritten | 导出队列与事件一致快照；同文件职责限本域以下登记函数 | `M13-F033` |
| `modules/jobs/internal/import.go` | handwritten | 验证任务快照；同文件职责限本域以下登记函数 | `M13-F034`, `M13-F035`, `M13-F036`, `M13-F037` |
| `modules/jobs/internal/recovery_authorization.go` | handwritten | 本地验证本次恢复专用控制许可；同文件职责限本域以下登记函数 | `M13-F038` |
| `modules/jobs/web/src/QueuePanel.vue` | ui | Vue3模块交互：RequestJobCancellation；同文件职责限本域以下登记函数 | `M13-F039` |
| `modules/jobs/web/src/ScheduleEditor.vue` | ui | Vue3模块交互：SaveSchedule；同文件职责限本域以下登记函数 | `M13-F040` |
| `modules/jobs/web/src/progress.ts` | ui | Vue3模块交互：ReconnectProgress；同文件职责限本域以下登记函数 | `M13-F041` |
| `modules/jobs/internal/http_event_subscription.go` | handwritten | 已登记消费者订阅事件SSE；同文件职责限本域以下登记函数 | `M13-F042` |
| `modules/jobs/internal/types.gen.go` | generated | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module jobs --target go-types，输出不得手改 |  |
| `modules/jobs/internal/http_bindings.gen.go` | generated | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module jobs --target go-http，x-handler映射仅转调手写函数 |  |
| `modules/jobs/config/defaults.json` | config | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/jobs/migrations/0001_domain.sql` | migration | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 |  |
| `modules/jobs/cmd/jobs/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M13-F901` |
| `modules/jobs/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M13-F902` |
| `modules/jobs/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M13-F903`, `M13-F904`, `M13-F905`, `M13-F906` |
| `modules/jobs/go.mod` | config | 独立module nice-wiki/modules/jobs；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/jobs/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/jobs/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/jobs；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/jobs/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/jobs与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/jobs/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/jobs/web/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/jobs/web/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/jobs/web/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/jobs/web/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/jobs/web/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/jobs/web/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M13-F907` |
