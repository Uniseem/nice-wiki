# data-protection 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=57；分类={"handwritten": 39, "ui": 4, "generated": 4, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/data-protection/internal/backup_validation.go` | handwritten | 备份入口校验；同文件职责限本域以下登记函数 | `M15-F001` |
| `modules/data-protection/internal/backup.go` | handwritten | 建立备份操作；同文件职责限本域以下登记函数 | `M15-F002`, `M15-F006`, `M15-F011` |
| `modules/data-protection/internal/maintenance.go` | handwritten | 打开站点维护屏障；同文件职责限本域以下登记函数 | `M15-F003`, `M15-F004`, `M15-F012` |
| `modules/data-protection/internal/snapshot_client.go` | handwritten | 统一导出单域；同文件职责限本域以下登记函数 | `M15-F005` |
| `modules/data-protection/internal/manifest.go` | handwritten | 构建统一备份manifest；同文件职责限本域以下登记函数 | `M15-F007` |
| `modules/data-protection/internal/integrity.go` | handwritten | 校验文件完整性；同文件职责限本域以下登记函数 | `M15-F008`, `M15-F009` |
| `modules/data-protection/internal/crypto.go` | handwritten | 加密备份对象；同文件职责限本域以下登记函数 | `M15-F010`, `M15-F016` |
| `modules/data-protection/internal/queries.go` | handwritten | 分页备份目录；同文件职责限本域以下登记函数 | `M15-F013`, `M15-F014`, `M15-F037` |
| `modules/data-protection/internal/download.go` | handwritten | 下载私有备份授权；同文件职责限本域以下登记函数 | `M15-F015` |
| `modules/data-protection/internal/restore_validation.go` | handwritten | 全域恢复预检；同文件职责限本域以下登记函数 | `M15-F017` |
| `modules/data-protection/internal/restore.go` | handwritten | 建立完整恢复；同文件职责限本域以下登记函数 | `M15-F018`, `M15-F020`, `M15-F024` |
| `modules/data-protection/internal/import_client.go` | handwritten | 准备单域导入；同文件职责限本域以下登记函数 | `M15-F019`, `M15-F023`, `M15-F027` |
| `modules/data-protection/internal/decision_log.go` | handwritten | 持久全局提交决定；同文件职责限本域以下登记函数 | `M15-F021`, `M15-F025` |
| `modules/data-protection/internal/runtime_client.go` | handwritten | 提交运行epoch权威；同文件职责限本域以下登记函数 | `M15-F022` |
| `modules/data-protection/internal/restore_recovery.go` | handwritten | 继续中断恢复；同文件职责限本域以下登记函数 | `M15-F026`, `M15-F028` |
| `modules/data-protection/internal/journal.go` | handwritten | 恢复自身文件和决策操作；同文件职责限本域以下登记函数 | `M15-F029` |
| `modules/data-protection/internal/mirror_validation.go` | handwritten | 检查git镜像白名单；同文件职责限本域以下登记函数 | `M15-F030` |
| `modules/data-protection/internal/mirror.go` | handwritten | 提取允许镜像投影；同文件职责限本域以下登记函数 | `M15-F031`, `M15-F032` |
| `modules/data-protection/internal/git_local.go` | handwritten | 提交确定性git树；同文件职责限本域以下登记函数 | `M15-F033` |
| `modules/data-protection/internal/git_config.go` | handwritten | 设置远程同步目标；同文件职责限本域以下登记函数 | `M15-F034` |
| `modules/data-protection/internal/git_validation.go` | handwritten | 校验远程传输参数；同文件职责限本域以下登记函数 | `M15-F035` |
| `modules/data-protection/internal/git_remote.go` | handwritten | 显式远程fast-forward同步；同文件职责限本域以下登记函数 | `M15-F036` |
| `modules/data-protection/internal/retention.go` | handwritten | 安全保留清理；同文件职责限本域以下登记函数 | `M15-F038` |
| `modules/data-protection/internal/outbox.go` | handwritten | 可靠发送操作状态；同文件职责限本域以下登记函数 | `M15-F039` |
| `modules/data-protection/internal/events.go` | handwritten | 屏障通知消费；同文件职责限本域以下登记函数 | `M15-F040` |
| `modules/data-protection/internal/barrier.go` | handwritten | 取得本域快照安全截点；同文件职责限本域以下登记函数 | `M15-F041`, `M15-F042` |
| `modules/data-protection/internal/snapshot.go` | handwritten | 导出本域操作目录快照；同文件职责限本域以下登记函数 | `M15-F043` |
| `modules/data-protection/internal/import.go` | handwritten | 校验保护域快照；同文件职责限本域以下登记函数 | `M15-F044`, `M15-F045`, `M15-F046`, `M15-F047` |
| `modules/data-protection/internal/recovery_authorization.go` | handwritten | 本地验证本次恢复专用控制许可；同文件职责限本域以下登记函数 | `M15-F048` |
| `modules/data-protection/internal/recovery_control.go` | handwritten | 维护前取得已批准恢复许可；同文件职责限本域以下登记函数 | `M15-F049` |
| `modules/data-protection/web/src/BackupPanel.vue` | ui | Vue3模块交互：RequestBackup；同文件职责限本域以下登记函数 | `M15-F050` |
| `modules/data-protection/web/src/RestoreWizard.vue` | ui | Vue3模块交互：ApproveFullRestore；同文件职责限本域以下登记函数 | `M15-F051` |
| `modules/data-protection/web/src/GitSyncPanel.vue` | ui | Vue3模块交互：RequestMirrorSync；同文件职责限本域以下登记函数 | `M15-F052` |
| `modules/data-protection/internal/backup_schedule.go` | handwritten | 每日本地备份；同文件职责限本域以下登记函数 | `M15-F053` |
| `modules/data-protection/internal/publication_mirror.go` | handwritten | 每次发布写data git镜像；同文件职责限本域以下登记函数 | `M15-F054` |
| `modules/data-protection/internal/setup_restore.go` | handwritten | 初始化向导从完整备份恢复；同文件职责限本域以下登记函数 | `M15-F055` |
| `modules/data-protection/internal/portable_download.go` | handwritten | 下载可移植完整备份；同文件职责限本域以下登记函数 | `M15-F056` |
| `modules/data-protection/internal/portable_archive.go` | handwritten | 校验上传便携ZIP；同文件职责限本域以下登记函数 | `M15-F057` |
| `modules/data-protection/internal/portable_upload.go` | handwritten | 恢复入口接收完整备份；同文件职责限本域以下登记函数 | `M15-F058` |
| `modules/data-protection/internal/types.gen.go` | generated | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module data-protection --target go-types，输出不得手改 |  |
| `modules/data-protection/internal/http_bindings.gen.go` | generated | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module data-protection --target go-http，x-handler映射仅转调手写函数 |  |
| `modules/data-protection/config/defaults.json` | config | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/data-protection/migrations/0001_domain.sql` | migration | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 |  |
| `modules/data-protection/cmd/data-protection/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M15-F901` |
| `modules/data-protection/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M15-F902` |
| `modules/data-protection/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M15-F903`, `M15-F904`, `M15-F905`, `M15-F906` |
| `modules/data-protection/go.mod` | config | 独立module nice-wiki/modules/data-protection；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/data-protection/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/data-protection/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/data-protection；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/data-protection/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/data-protection与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/data-protection/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/data-protection/web/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/data-protection/web/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/data-protection/web/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/data-protection/web/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/data-protection/web/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/data-protection/web/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M15-F907` |
