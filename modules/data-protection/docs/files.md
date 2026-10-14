# data-protection 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 55 个：手写 37、UI 4、生成 4、配置 9、迁移 1。函数 63 个：约束 38，参考 25。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/data-protection/internal/backup_validation.go` | 手写 | 备份入口校验；同文件职责限本域以下登记函数 | `data-protection.ValidateBackupRequest`（参考） |
| `modules/data-protection/internal/backup.go` | 手写 | 建立备份操作；同文件职责限本域以下登记函数 | `data-protection.CreateBackup`、`data-protection.CollectSnapshots`、`data-protection.FinalizeBackup` |
| `modules/data-protection/internal/maintenance.go` | 手写 | 打开站点维护屏障；同文件职责限本域以下登记函数 | `data-protection.AcquireMaintenance`（参考）、`data-protection.WaitAllSafe`（参考）、`data-protection.ReleaseMaintenance`（参考） |
| `modules/data-protection/internal/snapshot_client.go` | 手写 | 统一导出单域；同文件职责限本域以下登记函数 | `data-protection.ExportDomainSnapshot`（参考） |
| `modules/data-protection/internal/manifest.go` | 手写 | 构建统一备份manifest；同文件职责限本域以下登记函数 | `data-protection.BuildManifest`（参考） |
| `modules/data-protection/internal/integrity.go` | 手写 | 校验文件完整性；同文件职责限本域以下登记函数 | `data-protection.VerifyObjectHash`（参考）、`data-protection.ValidateCrossDomainRefs`（参考） |
| `modules/data-protection/internal/crypto.go` | 手写 | 加密备份对象；同文件职责限本域以下登记函数 | `data-protection.EncryptBackupObject`（参考）、`data-protection.DecryptForValidation`（参考） |
| `modules/data-protection/internal/queries.go` | 手写 | 分页备份目录；同文件职责限本域以下登记函数 | `data-protection.ListBackups`、`data-protection.ReadBackup`、`data-protection.ReadMirrorStatus`（参考） |
| `modules/data-protection/internal/download.go` | 手写 | 下载私有备份授权；同文件职责限本域以下登记函数 | `data-protection.AuthorizeBackupDownload` |
| `modules/data-protection/internal/restore_validation.go` | 手写 | 全域恢复预检；同文件职责限本域以下登记函数 | `data-protection.ValidateRestore` |
| `modules/data-protection/internal/restore.go` | 手写 | 建立完整恢复；同文件职责限本域以下登记函数 | `data-protection.CreateRestore`、`data-protection.StageAllDomains`、`data-protection.SwitchAllDomains`、`data-protection.ServeRestoreArchive`、`data-protection.CleanupRestore` |
| `modules/data-protection/internal/import_client.go` | 手写 | 单域暂存；同文件职责限本域以下登记函数 | `data-protection.StageDomain`（参考）、`data-protection.DiscardDomain`（参考） |
| `modules/data-protection/internal/decision_log.go` | 手写 | 持久全局切换决定；同文件职责限本域以下登记函数 | `data-protection.WriteRestoreDecision`、`data-protection.ReadRestoreProgress` |
| `modules/data-protection/internal/runtime_client.go` | 手写 | 提交运行epoch权威；同文件职责限本域以下登记函数 | `data-protection.TransitionRuntimeEpoch`（参考） |
| `modules/data-protection/internal/restore_recovery.go` | 手写 | 继续中断恢复；同文件职责限本域以下登记函数 | `data-protection.ContinueRestore`、`data-protection.AbortRestore` |
| `modules/data-protection/internal/journal.go` | 手写 | 恢复自身文件和决策操作；同文件职责限本域以下登记函数 | `data-protection.RecoverProtectionJournal`（参考） |
| `modules/data-protection/internal/mirror_validation.go` | 手写 | 检查git镜像白名单；同文件职责限本域以下登记函数 | `data-protection.ValidateMirrorProjection`（参考） |
| `modules/data-protection/internal/mirror.go` | 手写 | 提取允许镜像投影；同文件职责限本域以下登记函数 | `data-protection.CollectPortableFiles`（参考）、`data-protection.BuildMirror` |
| `modules/data-protection/internal/git_local.go` | 手写 | 提交确定性git树；同文件职责限本域以下登记函数 | `data-protection.CommitMirrorTree`（参考） |
| `modules/data-protection/internal/git_config.go` | 手写 | 设置远程同步目标；同文件职责限本域以下登记函数 | `data-protection.ConfigureRemote` |
| `modules/data-protection/internal/git_validation.go` | 手写 | 校验远程传输参数；同文件职责限本域以下登记函数 | `data-protection.ValidateRemote`（参考） |
| `modules/data-protection/internal/git_remote.go` | 手写 | 显式远程fast-forward同步；同文件职责限本域以下登记函数 | `data-protection.SyncMirror` |
| `modules/data-protection/internal/retention.go` | 手写 | 安全保留清理；同文件职责限本域以下登记函数 | `data-protection.PruneBackups`（参考） |
| `modules/data-protection/internal/outbox.go` | 手写 | 可靠发送操作状态；同文件职责限本域以下登记函数 | `data-protection.PublishProtectionEvents` |
| `modules/data-protection/internal/events.go` | 手写 | 屏障通知消费；同文件职责限本域以下登记函数 | `data-protection.ConsumeBarrierEvent`、`data-protection.ConsumeEvents` |
| `modules/data-protection/internal/barrier.go` | 手写 | 取得本域快照安全截点；同文件职责限本域以下登记函数 | `data-protection.EnterBarrier`、`data-protection.ReleaseBarrier` |
| `modules/data-protection/internal/snapshot.go` | 手写 | 导出本域操作目录快照；同文件职责限本域以下登记函数 | `data-protection.CreateSnapshot` |
| `modules/data-protection/internal/import.go` | 手写 | 校验保护域快照；同文件职责限本域以下登记函数 | `data-protection.StageSnapshot`、`data-protection.DiscardSnapshot` |
| `modules/data-protection/web/src/BackupPanel.vue` | UI | Vue3模块交互：RequestBackup；同文件职责限本域以下登记函数 | `data-protection.RequestBackup`（参考） |
| `modules/data-protection/web/src/RestoreWizard.vue` | UI | Vue3模块交互：ApproveFullRestore；同文件职责限本域以下登记函数 | `data-protection.ApproveFullRestore`（参考） |
| `modules/data-protection/web/src/GitSyncPanel.vue` | UI | Vue3模块交互：RequestMirrorSync；同文件职责限本域以下登记函数 | `data-protection.RequestMirrorSync`（参考） |
| `modules/data-protection/internal/backup_schedule.go` | 手写 | 每日本地备份；同文件职责限本域以下登记函数 | `data-protection.ScheduleLocalBackup` |
| `modules/data-protection/internal/publication_mirror.go` | 手写 | 每次发布写data git镜像；同文件职责限本域以下登记函数 | `data-protection.MirrorSourceRevision`（参考） |
| `modules/data-protection/internal/setup_restore.go` | 手写 | 初始化向导从完整备份恢复；同文件职责限本域以下登记函数 | `data-protection.RestoreDuringSetup` |
| `modules/data-protection/internal/portable_download.go` | 手写 | 下载可移植完整备份；同文件职责限本域以下登记函数 | `data-protection.StreamPortableBackup` |
| `modules/data-protection/internal/portable_archive.go` | 手写 | 校验上传便携ZIP；同文件职责限本域以下登记函数 | `data-protection.ValidatePortableArchive`（参考） |
| `modules/data-protection/internal/portable_upload.go` | 手写 | 恢复入口接收完整备份；同文件职责限本域以下登记函数 | `data-protection.ImportPortableBackup` |
| `modules/data-protection/internal/types.gen.go` | 生成 | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module data-protection --target go-types，输出不得手改 | 无 |
| `modules/data-protection/internal/http_bindings.gen.go` | 生成 | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module data-protection --target go-http，x-handler映射仅转调手写函数 | 无 |
| `modules/data-protection/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/data-protection/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 | 无 |
| `modules/data-protection/cmd/data-protection/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `data-protection.main` |
| `modules/data-protection/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `data-protection.BuildApplication` |
| `modules/data-protection/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `data-protection.Serve`、`data-protection.Shutdown`、`data-protection.Health`、`data-protection.Ready` |
| `modules/data-protection/go.mod` | 配置 | 独立module nice-wiki/modules/data-protection；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/data-protection/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/data-protection/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/data-protection；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/data-protection/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/data-protection与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/data-protection/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/data-protection/web/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/data-protection/web/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/data-protection/web/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/data-protection/web/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/data-protection/web/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/data-protection/web/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `data-protection.mountModuleUI` |
