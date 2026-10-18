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

## 原模型调用只读观察

标准`modelclient.NewGatewayClient`的`GatewayResponseValidators`必须有`CreateInvocationResult`、`CancelInvocationResult`、`StreamEvent`、`GetInvocationResult`、`LookupInvocationResult`五个真实生产方schema校验器。旧`RawGatewayClient`三方法接口保持；可选`RawLookupGatewayClient.LookupInference(ctx,body)`和`RawCurrentGatewayClient.ReadInference(ctx,id)`提供只读端口。公共可选`InvocationLookupReader.LookupInference(ctx,key,input)`与`CurrentInvocationReader.GetInference(ctx,id)`返回`GatewayInvocationSnapshot{Invocation,Output?}`。

`InvokeModel`先以原完整请求和原幂等键查找。只有可信tracking证明真实404，且原付费截止仍有效才Create相同原键/原输入；缺tracking、孤立记录或读取失败不当作不存在，不更换键。原付费deadline只约束原Create/执行，过期后仍可用独立5秒的只读RPC截止观察；原owner、actor、root、site、epoch、qa/background费用类别及完整body保持，不重签/续消费permit。取消同原ID也用独立短控制截止，不能把过期付费op直接送去导致取消被拒。

`InvokeModelOutcome.CurrentSnapshot`只保留真实已观察的当前权威，与`Events`独立；错误或取消仍保留已接受ID、最后有效快照和已收事件，调用方不得丢弃这些事实。完整winning output可直接交付；`ModelOutput.Text`指针区分真实空文本与缺失，文本不超过64KiB UTF8，chat/vision/translation与embedding/rerank字段互斥。未知费用不抹去完整结果，也不伪造零账单。

首次历史读取从排他序号0开始，最多4096条，缺号或超限失败而不静默拼接。历史终态、断流及EOF后都读当前权威；过时的StreamError/charge_unknown仍保留在Events，但不作为当前终态回调，不掩盖后来完整结果。不会合成SSE事件。旧完整文本若只有真实持久事件，仍按原序号读完并保留；缺完整证据失败关闭，不将旧付费结果当404重派。当前实际Get/Lookup故障均不能触发新模型费用。

owner恢复查询使用`modelclient.ObserveInvocation(ctx,client,op,knownInvocationId)`或`LookupModelInvocation(ctx,client,op,originalInvokeRequest)`，只读原ID或完整原body/key。二者即使原付费截止仍有效也没有Create、Cancel或模型分支，不调用可创建的InvokeModel，不回调SSE；即使原截止已过也可按原scope获得fresh5秒读取。业务owner必须锁外执行网络观察，再以原持久输入/hash/阶段证明决定本地恢复，不从工具返回推导新的收费许可。
