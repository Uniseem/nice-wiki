# analytics 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=50；分类={"handwritten": 32, "ui": 4, "generated": 4, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/analytics/internal/beacon_validation.go` | handwritten | 严格匿名输入校验；同文件职责限本域以下登记函数 | `M14-F001` |
| `modules/analytics/internal/buckets.go` | handwritten | 固定时延桶；同文件职责限本域以下登记函数 | `M14-F002` |
| `modules/analytics/internal/privacy.go` | handwritten | IP+User-Agent+当日随机盐访客匿名hash；同文件职责限本域以下登记函数 | `M14-F003`, `M14-F011`, `M14-F029` |
| `modules/analytics/internal/http_beacons.go` | handwritten | 匿名HTTP接收；同文件职责限本域以下登记函数 | `M14-F004` |
| `modules/analytics/internal/beacon_store.go` | handwritten | 批量持久匿名条目；同文件职责限本域以下登记函数 | `M14-F005` |
| `modules/analytics/internal/aggregation.go` | handwritten | 精确重算封闭小时桶；同文件职责限本域以下登记函数 | `M14-F006`, `M14-F007` |
| `modules/analytics/internal/query_validation.go` | handwritten | 规范查询范围；同文件职责限本域以下登记函数 | `M14-F008` |
| `modules/analytics/internal/queries.go` | handwritten | 访问汇总查询；同文件职责限本域以下登记函数 | `M14-F009`, `M14-F014`, `M14-F015`, `M14-F019` |
| `modules/analytics/internal/audit_validation.go` | handwritten | 校验审计allowlist；同文件职责限本域以下登记函数 | `M14-F010` |
| `modules/analytics/internal/audit_chain.go` | handwritten | 计算审计完整性链；同文件职责限本域以下登记函数 | `M14-F012` |
| `modules/analytics/internal/audit_store.go` | handwritten | 追加安全审计；同文件职责限本域以下登记函数 | `M14-F013` |
| `modules/analytics/internal/usage_validation.go` | handwritten | 校验gateway钱账引用；同文件职责限本域以下登记函数 | `M14-F016` |
| `modules/analytics/internal/usage_store.go` | handwritten | 更新派生费用行；同文件职责限本域以下登记函数 | `M14-F017` |
| `modules/analytics/internal/usage_aggregation.go` | handwritten | 费用派生聚合；同文件职责限本域以下登记函数 | `M14-F018` |
| `modules/analytics/internal/gateway_client.go` | handwritten | 取得权威账本投影输入；同文件职责限本域以下登记函数 | `M14-F020` |
| `modules/analytics/internal/reconciliation.go` | handwritten | 对账修复派生统计；同文件职责限本域以下登记函数 | `M14-F021` |
| `modules/analytics/internal/event_handler.go` | handwritten | 消费安全审计和费用事件；同文件职责限本域以下登记函数 | `M14-F022` |
| `modules/analytics/internal/export.go` | handwritten | 受权限保护统计导出；同文件职责限本域以下登记函数 | `M14-F023` |
| `modules/analytics/internal/csv.go` | handwritten | 安全CSV单元格；同文件职责限本域以下登记函数 | `M14-F024` |
| `modules/analytics/internal/retention.go` | handwritten | 清理短期匿名行；同文件职责限本域以下登记函数 | `M14-F025`, `M14-F026`, `M14-F027` |
| `modules/analytics/internal/outbox.go` | handwritten | 派生事件outbox派发；同文件职责限本域以下登记函数 | `M14-F028` |
| `modules/analytics/internal/barrier.go` | handwritten | 进入analytics一致屏障；同文件职责限本域以下登记函数 | `M14-F030`, `M14-F031` |
| `modules/analytics/internal/snapshot.go` | handwritten | 导出统计快照；同文件职责限本域以下登记函数 | `M14-F032` |
| `modules/analytics/internal/import.go` | handwritten | 校验统计恢复；同文件职责限本域以下登记函数 | `M14-F033`, `M14-F034`, `M14-F035`, `M14-F036` |
| `modules/analytics/internal/recovery_authorization.go` | handwritten | 本地验证本次恢复专用控制许可；同文件职责限本域以下登记函数 | `M14-F037` |
| `modules/analytics/web/src/OverviewPanel.vue` | ui | Vue3模块交互：LoadStatistics；同文件职责限本域以下登记函数 | `M14-F038` |
| `modules/analytics/web/src/AuditExplorer.vue` | ui | Vue3模块交互：LoadAuditPage；同文件职责限本域以下登记函数 | `M14-F039` |
| `modules/analytics/web/src/client.ts` | ui | Vue3模块交互：DownloadAuthorizedExport；同文件职责限本域以下登记函数 | `M14-F040` |
| `modules/analytics/internal/crawler.go` | handwritten | 内存识别已知爬虫；同文件职责限本域以下登记函数 | `M14-F041` |
| `modules/analytics/internal/geoip.go` | handwritten | 离线GeoIP国家；同文件职责限本域以下登记函数 | `M14-F042` |
| `modules/analytics/internal/crawler_store.go` | handwritten | AI Markdown访问独立计数；同文件职责限本域以下登记函数 | `M14-F043` |
| `modules/analytics/internal/content_gaps.go` | handwritten | 热门文档与答不出问题联动；同文件职责限本域以下登记函数 | `M14-F044` |
| `modules/analytics/internal/types.gen.go` | generated | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module analytics --target go-types，输出不得手改 |  |
| `modules/analytics/internal/http_bindings.gen.go` | generated | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module analytics --target go-http，x-handler映射仅转调手写函数 |  |
| `modules/analytics/config/defaults.json` | config | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/analytics/migrations/0001_domain.sql` | migration | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 |  |
| `modules/analytics/cmd/analytics/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M14-F901` |
| `modules/analytics/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M14-F902` |
| `modules/analytics/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M14-F903`, `M14-F904`, `M14-F905`, `M14-F906` |
| `modules/analytics/go.mod` | config | 独立module nice-wiki/modules/analytics；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/analytics/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/analytics/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/analytics；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/analytics/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/analytics与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/analytics/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/analytics/web/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/analytics/web/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/analytics/web/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/analytics/web/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/analytics/web/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/analytics/web/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M14-F907` |
