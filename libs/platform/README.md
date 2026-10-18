# 平台公共库（L01）

各服务共用的基础设施实现，Go module `nice-wiki/libs/platform`。决策见 [ADR0005](../../docs/adr/0005-engineering-and-deployment.md)。

只放与业务无关的机制：服务断言与用户委托、可信操作上下文、公共错误输出、outbox 登记与转发、inbox 去重与事件订阅、任务领取循环、模型调用助手、配置加载、运行状态监视、恢复换目录、进程模板，以及生成服务密钥的 `nw-keys` 工具。

规则：

- 不含任何业务规则，不访问任何服务的私有数据，不导入服务代码；服务只经本库公开函数使用这些机制。
- 各服务的 RelayOutbox、ConsumeEvents、RunWorker、InvokeModel、main、BuildApplication、Serve 等约束函数只装配本域参数并调用本库。
- 改动本库视为公共契约变更，由集成会话统一修改并同步所有服务。

资料：[函数合同](docs/functions.md)、[文件目录](docs/files.md)、[机器合同](contracts/implementation.json)、[类型](contracts/types.json)、[进度台账](progress.json)。

ServiceConfig的可选`masterKeyRef`（环境覆盖`NW_MASTER_KEY_REF`）只传递部署主密钥绝对路径；配置加载不打开密钥，使用域按具体加密操作读取。

第三轮公共候选补签名站点传递和任务六方法窄适配，已通过根GitLab pipeline32候选检查，实际集成树仍按顺序复检；状态与未完成边界见STATUS及handoff。

## 公共快照与恢复

`lifecycle.NewSnapshotManager(SnapshotConfig)`只装配机制。各域提供真实Runtime、BeginDrain、Drain、Watermark、Capture、MigrateStage、ValidateStaged、StageBudget和严格ValidateManifest；缺一失败，不用未知空数组、水位0或回调空成功代替完整业务证明。`Capture(ctx,cut,workspace,input)`将一致SQL副本及全部权威文件写到`workspace/files`，返回完整`DomainSnapshotManifest`，源对象、发布及引用必须穷尽。平台不读业务表、不复制live SQLite、不改块ID或已付费blob。

所有实际业务、任务、订阅、后台及最终持久收尾都需同实例`AdmitBusiness`取得并持有私有lease，直到实际IO全部结束才release。`EnterBarrier`先关闭准入并持久draining，再BeginDrain停领取/取消旧执行、等待business退出、Drain结清原费用/回执/最终outbox、关闭新cleanup签发并等待既有cleanup退出，核真实水位和当前运行决定后才记本进程safe。`SnapshotCleanup.Context`只在当前可信drain回调有效期签既有执行的收尾lease，保留原actor/root/费用，不创建模型请求。每次写仍复用`NewWriteContext`和`WriteWindow`。

该机制不自证全域安全。jobs必须持续接受其它域最后outbox直到全域生产者均已封口，之后才能完成自身排空；恢复旧pending事件的epoch不能通用重写或丢弃。协调器尚未给出完整证明时，各域完整服务与备份恢复保持partial。

`BuildSnapshotArchive`、`ExtractSnapshotArchive`仅支持普通USTAR V1：首`manifest.json`为canonical JSON且不超过16MiB，之后只有声明的`files/<path>`普通文件，含清单总文件数不超过100000，完整tar不超过20GiB。拒链接、路径跳转、PAX/GNU、重复/未声明项，逐文件hash/长度和全归档hash/精确物理长度/EOF都核对。容量来自真实Statfs和域迁移峰值计划，未知不当作0通过。

`SnapshotManager.Export`按原输入journal固定snapshotId，在真实Capture和完整归档fsync后写archive-prepared，rename和双方目录持久后写complete。`OpenArchive(ctx,snapshotId,expectedObjectRef)`只返回与真实complete回执全值相符且全文件hash核对通过的流，调用方负责关闭，接收方仍核完整引用。尚无DP“全部加密持久”的版本化回收回执，所以本机制不清历史export归档。

`Stage(ctx,input,reader)`只写隔离目录。stageId固定为SHA256(canonical `[restoreId,snapshotId]`)的完整64字符；源备份SourceEpoch和本机HostEpoch分别保存。迁移后完整文件与目录（包括空目录）分别排序后以`SnapshotTree` canonical JSON计算目标manifestHash；源manifestHash另存SourceManifestHash，Checksum始终为完整源tar。prepared-stage回执保存原输入摘要、完整源清单、源/host/target epochs与完整目标树，fsync后才rename到staged并持久完成；重启续接仍核完整树和域ValidateStaged。StageResult新版始终返回stageId与目标manifestHash。

`ApplyRestoreSwap`在开放入口前核完整stage、原输入、源清单、目标整树和精确Runtime决定。prepared-switch先固定原active树；active→before与staged→active每一步fsync双方父目录后再写对应完整日志；换回反向同机制。旧`restore-log.jsonl`仍是权威，追加通过有界完整文件fsync和原子替换防半行。只有与持久阶段精确相容的物理组合可续接，不删除、覆盖或猜测未知目录。`ReadRestoreSwapState`只读该真实日志与完整stage及目录组合，中间态不给删除许可。进入切换直至激活回执前，业务域不得引入未登记的active业务变化。

`Discard`仅在当前权威允许的原restore staging、全域换回后的running HostEpoch、或全域完成后的running TargetEpoch清对应staged/before；先持久discarding意图再删除/fsync/记discarded。未知记录、代次、日志或目录不清理。外部身份与schema只在既有HTTP边界校验，以上内部API不引入新的业务权限。
