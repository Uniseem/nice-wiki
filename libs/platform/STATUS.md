# 2026-10-18 收尾检查与停止

自部署GitLab pipeline118：生成及重复生成、16模块readonly build/vet/gofmt、已有普通go test、完整make check、六前端类型/构建通过；无测试文件；仅源码检查，非业务验收。

本域本次明确完整清单78项更新verified，准确ID见progress.json；其余blocked/planned保留，整体服务和全域保护未完成。用户要求push后停止，代理已冻结，不自动启动下一批。

以下为历史记录。

# 公共scope、两阶段屏障与签名清理交付

源码 28882114d22b2df504cdf4101a48a016e385ed7c、68e5e33dc100ef1eca0d179b726b49d2d0bd8827、1904ea07ba2ebd4f20ad524ad9e6e4478d4fb4b2、94cdeeefd7f3436e780db141a0a476075a343d96。本批73个新/修改实际函数blocked待GitLab，其中43个新增登记参考函数（含原有EnsureSchema补登记），不冒称均为新增业务。未动旧verified保留原证据。本机只做Git/文字与隐私审阅，git diff --check通过，无本机/SSH编译生成、无新测试/样例/临时业务程序、无真实接口/模型/部署/push。

精确API、真实SQL水位/CAS、原root/费用/event来源隔离及HTTP/common片段见[屏障与清理范围](docs/barriers.md)。Root须接14域实际ObserveWork/SealWork/ResumeWork/VerifyGlobalSeal与真实HTTP策略/cleanup原对象授权；缺任何实际回调/全域协调证明均保持blocked，不把公共方法当整站safe/Ready。jobs最后封口，冻结收费queued job不运行清零，旧epoch投递撤资格且保全原费用/输入/输出/事件审计。portal仍独立配置gate，不占14有状态owner。

Root须保留工作区外926fd07的WorkerConfig.OnCancellationObserved及真实lease回调，本分支不覆盖。BarrierRequest省略step只prepare，step含prepare/drain/observe/seal/resume-drain；BarrierStatus需observation与sealProofHash可选字段，ROOT统一common/HTTP。cleanupScope带queryHash及可选eventControl，严格原root/site/epoch/costClass；来源/目标/分叉均保持event控制资格，不能变paid root。

完整Capture工具只允许真实safe callback；SQLite VACUUM INTO、普通文件有界copy/hash/fsync、未登记SQLite/链接/未知数据失败关闭。ManifestV2必填真实assets/invocations，V1原canonical不漂移；readSnapshotStageRecord也明确V1/2。PrepareRestoredOutbox只在真实Stage私有scope内迁移源gate并撤旧投递资格，保原行与payload/epoch/费用，目标activate前仍本次host hash封口。

SealTaskSecret/OpenTaskSecret只返回nonce+ciphertext，AES256GCM+固定私有原始32B key与原owner/site/epoch/root/purpose版本化AAD，明文≤64KiB；缺key503，不延长permit，持久/权限/回收由本域实现。新caller须Root在实际localization任务接线。

精确待检查IDs：

- `platform.SignAssertion`
- `platform.SignOutboundRequest`
- `platform.BuildOperationContext`
- `platform.EnsureSchema`
- `platform.RunRelayBatch`
- `platform.RunConsumerWithResult`
- `platform.consumerLoop.start`
- `platform.consumerLoop.worker`
- `platform.consumerLoop.queueFor`
- `platform.consumerLoop.dispatch`
- `platform.consumerLoop.acknowledge`
- `platform.consumerLoop.process`
- `platform.consumerLoop.classify`
- `platform.consumerLoop.pauseFor`
- `platform.inbox.readRuntime`
- `platform.RunWorkerLoopWithResult`
- `platform.workerLoop.run`
- `platform.workerLoop.execute`
- `platform.RunRecoveryLoop`
- `platform.NewServiceHandler`
- `platform.readSnapshotStageRecord`
- `platform.NewTaskGroup`
- `platform.TaskGroup.Start`
- `platform.TaskGroup.SetAdmission`
- `platform.TaskGroup.CancelActive`
- `platform.TaskGroup.WaitIdle`
- `platform.DomainSnapshotManifest.MarshalJSON`
- `platform.validateSnapshotManifestVersion`
- `platform.validateSnapshotInventory`
- `platform.NewSnapshotManager`
- `platform.SnapshotManager.signalLocked`
- `platform.SnapshotManager.AdmitBusiness`
- `platform.SnapshotManager.WriteWindow`
- `platform.SnapshotCleanup.Context`
- `platform.SnapshotManager.runDrainCallback`
- `platform.SnapshotManager.EnterBarrier`
- `platform.SnapshotManager.ReleaseBarrier`
- `platform.SnapshotManager.AdmitJobClaim`
- `platform.SnapshotManager.BindJobExecutionScope`
- `platform.SnapshotManager.PrepareBarrier`
- `platform.SnapshotManager.DrainBarrier`
- `platform.SnapshotManager.requireDraining`
- `platform.SnapshotManager.ObserveBarrier`
- `platform.SnapshotManager.SealBarrier`
- `platform.SnapshotManager.ResumeDrain`
- `platform.validateSnapshotSealProof`
- `platform.SnapshotManager.Export`
- `platform.SnapshotManager.Stage`
- `platform.WithCleanupIssuer`
- `platform.CleanupIssuerFrom`
- `platform.WithoutCleanupIssuer`
- `platform.noCleanupIssuer.IssueCleanup`
- `platform.MatchCleanupRoute`
- `platform.CleanupResourceID`
- `platform.ValidateCleanupScope`
- `platform.snapshotAdmission.IssueCleanup`
- `platform.SnapshotManager.ExistingWorkContext`
- `platform.SnapshotManager.currentBarrierID`
- `platform.SnapshotManager.newCleanupLocked`
- `platform.SnapshotManager.AdmitCleanup`
- `platform.SnapshotManager.AdmitEventBatch`
- `platform.SnapshotManager.AllowEventDrain`
- `platform.SnapshotManager.BindOutboxEventScope`
- `platform.CaptureSnapshotFiles`
- `platform.Store.Observe`
- `platform.Store.Seal`
- `platform.Store.Resume`
- `platform.SnapshotStageBinding`
- `platform.PrepareRestoredOutbox`
- `platform.SealTaskSecret`
- `platform.OpenTaskSecret`
- `platform.taskSecretAAD`
- `platform.taskSecretCipher`

# 原调用未接受专用否证明

源码dcba0de：公共CodeInvocationNotAccepted和404映射，Invoke仅认INVOCATION_NOT_ACCEPTED才可在原paid deadline有效时Create同原键。普通NOT_FOUND可能是旧部署/路由缺失，绝不作为未接受证明；只读Lookup保留专用码，已知ID Get404仍Integrity。待Root common/errors与MG producer统一GitLab，本机未检查/未真实调用，未push。下一项独立落地HTTP/worker/inbox/relay/TaskGroup scope hooks；两阶段全域闭包先向Root提精确协议片段，不把错时采样0当safe。

# owner恢复纯只读观察入口

源码db54fae，新增实际参考platform.ObserveInvocation、platform.LookupModelInvocation，均blocked待GitLab及owner真实装配。签名与返回沿前节公共API，二者只观察原已知InvocationID或完整原body/key；原截止有效也没有Create/Cancel/收费分支，过期也仅fresh5秒read。owner须锁外网络读取，并从原持久事实装配原root/site/epoch/costClass，不使用可能Create的InvokeModel作为恢复查询。Root登记知识等owner实际caller依赖。

无本机Go/生成/格式/编译/测试，无真实接口/模型/部署/push。前批模型核心7ccc81f、说明12810da及合同2278a56、台账65826a5；快照与全域协调边界保持原记录。

# 原模型调用只读观察小批

源码7ccc81f，前置Text字段7d37126，说明/精确依赖同步12810da与2278a56。21项新/修改函数blocked待GitLab，其中15实际新登记reference含原先已有未登记drain/terminalOf/cancelCall/bindDeadline，不冒称15新业务。原InvokeModel绑定行为完成源码，未真实接口/模型联调或部署。

API：platform.GatewayInvocationSnapshot{Invocation Invocation,Output *ModelOutput}；InvokeModelOutcome.CurrentSnapshot *GatewayInvocationSnapshot。可选CurrentInvocationReader.GetInference(ctx,id)、InvocationLookupReader.LookupInference(ctx,key,input)，Raw读接口ReadInference(ctx,id)/LookupInference(ctx,body)与真实生成端口一致。NewGatewayClient要求五校验器CreateInvocationResult、CancelInvocationResult、StreamEvent、GetInvocationResult、LookupInvocationResult，Root负责QA/knowledge/memory/localization真实alias及工厂装配。

先原键+完整原请求Lookup，仅可信tracking真404且原paid deadline有效才Create一次同键/同输入；Create失败也只lookup，不换键重付。5秒fresh只读/取消截止只用于控制RPC，保原owner/actor/root/site/epoch/costClass，不重签permit。CreateInput兼容LanguageSuggestionApprovalID；真实Text指针区分空/缺失。当前完整winning输出独立于Events，未知费用不抹结果、不编零账单。接受后错误/取消保真实ID/最后快照/已收事件；历史终态/EOF刷新Get，过时终态不回调成当前终态但原样保Events，不伪造SSE。旧无output文本只能按原持久序号和完成事实复用，证据不足Integrity而不404/重派。

无本机或SSH Go/gofmt/gen/build/vet/check/pnpm，无新增测试/fixture/mock/样例或临时业务验证程序，无真实模型/接口/业务服务/部署/push。已有本地git diff --check通过。下一步按Root GitLab反馈窄修本切片，恢复/14域闭环继续按上一节具体阻塞，不提前标verified。

精确待检查IDs：

- `platform.InvokeModel`
- `platform.NewGatewayClient`
- `platform.CreateInference`
- `platform.CancelInference`
- `platform.decodeInvocation`
- `platform.validatedModelNext`
- `platform.LookupInference`
- `platform.GetInference`
- `platform.normalizeGatewayInput`
- `platform.decodeGatewaySnapshot`
- `platform.validateModelOutput`
- `platform.gatewayReadContext`
- `platform.readCurrentGatewaySnapshot`
- `platform.finishKnownGatewaySnapshot`
- `platform.modelEventTerminal`
- `platform.notifyCurrentModelTerminal`
- `platform.drain`
- `platform.terminalOf`
- `platform.cancelCall`
- `platform.bindDeadline`
- `platform.modelInvokeSleep`

# 2026-10-18 公共机制第五批检查通过

GitLab pipeline109（源码eb84b64；生成507、三个Go分片508/509/510、前端511）全部成功；16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check和四个实际前端类型检查/构建通过。精确8050字节生成补丁已回传；普通测试无测试文件，不代表业务验收或部署。

本域64项独立机制或实际修改函数通过源码检查。14域完整回调、全域封口与完整服务未齐，保持未完成；新增归档接口尚未接真实manager。

# 恢复续接目录持久次序窄修

源码81730f7：snapshotSwapLog在写下一阶段日志前统一fsync数据根与本restore目录，覆盖崩溃后已发生rename但未完成日志的续接分支；snapshotRename仅负责不覆盖已存在目标的真实rename。四修改函数重新blocked待GitLab，无新函数。未本机Go/生成/格式/编译/测试、未真实业务/部署/push。随后按Root分配接公共modelclient原键只读观察。

# 公共快照与恢复源码切片

源码3211e23及删除边界窄修afae2ed，基线599cc7b，分支codex/finish-20261018-snapshot，独占libs/platform。实际57项函数登记待GitLab（修改原ApplyRestoreSwap；56实际新增登记参考，其中5个原先已有未登记助手，不冒称56新业务）。新增5个snapshot_*.go和重写restore_swap.go，无业务域导入。源码就绪不表示14域回调或全域安全点已完成。

完整USTAR有界归档/隔离解包、原输入与源清单及目标完整树stage journal、私有进程lease/drain、真实换目录日志与崩溃续接、ReadRestoreSwapState、先记意图后删除均已写。StageId为canonical[restoreId,snapshotId]全64hex；目标manifestHash包含文件和空目录，源清单hash及full tar checksum分开。公共common字段Root14b5c44，HTTP/14域Capture/MigrateStage/ValidateStaged和实际scope接线Root负责，未齐不记可运行。

全域屏障仍有明确缺口：jobs必须等全部生产者封口和最终outbox排空后才safe；旧epoch job/fence/event撤销活动资格，原未决费用、输入/输出/来源/队列事实只隔离审计留存，不通用改epoch或物理删证据后声称无调用。新版免费重建通知须真实生产者/restore证据。没有DP“全部加密持久”的回收回执协议，所以保留历史export，不以Release推删除。

本批只做文档读取、静态复核和git diff --check；未本机或SSH Go/gofmt/gen/build/vet/check/pnpm，未新增测试/fixture/mock/样例或临时业务验证程序，未真实接口/模型/业务服务/部署/push。下一步仅按根GitLab反馈窄修本切片，再接真实运行scope与协调证明，不以缺失回调空成功补齐。

精确待检查函数IDs：

- `platform.integrity`
- `platform.readSnapshotStageRecord`
- `platform.ApplyRestoreSwap`
- `platform.snapshotDirectoryExists`
- `platform.snapshotSwapPaths`
- `platform.verifySnapshotHash`
- `platform.snapshotRename`
- `platform.snapshotSwapLog`
- `platform.applySnapshotSwitch`
- `platform.applySnapshotRollback`
- `platform.ReadRestoreSwapState`
- `platform.restoreLogState`
- `platform.readRestoreLog`
- `platform.appendRestoreLog`
- `platform.syncDir`
- `platform.splitLines`
- `platform.snapshotIOError`
- `platform.snapshotBudget`
- `platform.snapshotUnsupported`
- `platform.decodeSnapshotCanonical`
- `platform.readSnapshotJSON`
- `platform.syncSnapshotDirectories`
- `platform.snapshotDirectory`
- `platform.writeSnapshotJSON`
- `platform.safeSnapshotRelative`
- `platform.CheckSnapshotDiskBudget`
- `platform.copySnapshotExact`
- `platform.verifySnapshotFile`
- `platform.snapshotCountedReader.Read`
- `platform.snapshotCountedWriter.Write`
- `platform.validateSnapshotInventory`
- `platform.verifySnapshotInventoryTree`
- `platform.snapshotArchiveByteSize`
- `platform.BuildSnapshotArchive`
- `platform.ExtractSnapshotArchive`
- `platform.NewSnapshotManager`
- `platform.SnapshotManager.persistBarrier`
- `platform.SnapshotManager.signalLocked`
- `platform.lockSnapshotControl`
- `platform.SnapshotManager.AdmitBusiness`
- `platform.SnapshotManager.WriteWindow`
- `platform.SnapshotCleanup.Context`
- `platform.SnapshotManager.runDrainCallback`
- `platform.SnapshotManager.waitCounts`
- `platform.SnapshotManager.EnterBarrier`
- `platform.SnapshotManager.ReleaseBarrier`
- `platform.SnapshotManager.beginFileOperation`
- `platform.SnapshotManager.Export`
- `platform.SnapshotManager.OpenArchive`
- `platform.removeSnapshotTree`
- `platform.fingerprintSnapshotTree`
- `platform.snapshotStageID`
- `platform.verifySnapshotTarget`
- `platform.SnapshotManager.Stage`
- `platform.SnapshotManager.finishStage`
- `platform.SnapshotManager.Discard`
- `platform.verifySnapshotExportRecord`

# 按登记类型领取公共小补

源码3f17df5：新增platform.ClaimJobTypes实际适配与TypedWorkerJobsClient可选能力，旧ClaimJob保留无筛选兼容；循环必须有typed能力并实际传config.JobTypes到原jobs:claimJob，缺能力失败关闭，不在领取后丢弃任务假筛选。四新/改函数blocked待根GitLab及生产方jobTypes/SQL同时检查。无本机Go/生成/格式/编译/测试、无真实任务/业务/模型/部署/push。

公共Portal加载源码2759c93、台账0d05ca0已独立交根。继续同包SnapshotManager真实归档与stage/swap证明，不改其它模块。

# Portal委托身份公共加载小补

源码2759c93，新增platform.LoadPortalDelegation，复用严格身份描述读取与私钥Loader，仅portal可调用，核本站委托公钥全值。仅源码ready，blocked待GitLab，未读取真实密钥、未本机Go/生成/格式/编译/测试、未部署/push。

当前独占libs/platform，分支codex/finish-20261018-snapshot，基线599cc7b；已读根及公共合同、全部快照草稿，继续真实归档/Stage证明/崩溃可续接换目录。全域屏障、jobs最终outbox排空与旧pending事件代次仍需协调证明，不能宣称全链safe。

# 实际取消回调待GitLab

worker收到真实cancelRequested时先调用OnCancellationObserved持久精确worker/fence/epoch取消事实，再取消执行；safePoint不当取消，失败停止，回调不触发业务/model。仅源码与合同，待Root后续集成GitLab，未本机编译或真实任务。

# 2026-10-18第三批源码检查通过

自部署GitLab pipeline95（源码1e65c5f；生成436、三个Go分片437/438/439、前端440）全部成功；16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及三个实际前端类型检查/构建通过。精确生成补丁零字节。普通测试均无测试文件；不代表业务验收或部署。

本域本批完整函数5项通过源码检查。完整服务、快照/恢复、业务验收按逐项台账继续未完成；不把部分装配记完整。未本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 2026-10-18预算与可信配置修正已通过

GitLab pipeline86（770b93c；生成391、Go三分片392/393/394、前端395）全部成功，预算未知金额修正与可信Git凭据索引已通过16模块readonly build/vet/gofmt、已有普通go test、完整make check和前端类型/构建；精确31561字节生成物回传到123f699f162abaee206e99fe00c3e07e89050538。普通测试无测试文件，不是业务验收。

完整服务仍继续实现。

# 2026-10-18 续写第二批检查通过

GitLab pipeline84（源码72219d7；生成381、三个Go分片382/383/384、前端385）全部成功，16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及前端类型检查/构建通过。pipeline83各编译分片通过但缺既有信封校验登记，已补并复检；实际普通测试无测试文件。源码生成物由GitLab工具回传，不代表业务验收、隔离行为验收或部署。

本域本批17项完整函数通过源码检查，完整启动/快照恢复/业务accepted状态保持未完成。模型网关、知识、数据保护三个worktree继续，未在本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 2026-10-18 继续全部剩余代码：首批检查通过

用户新授权继续三个子代理，目标完成全部剩余代码，及时GitLab检查后普通双推；此前75%停止仅为历史。GitLab pipeline76（8332b6c；生成341，三分片342/343/344，前端345）全部成功。16个Go模块readonly build/vet/gofmt、普通go test与完整make check、Vue类型检查/构建通过；159373字节生成物已精确回传至0881c49c085bbbfc643e43998e9fd673608ef534。普通go test无测试文件；不代表业务验收。

本域本批19项完整源码按精确交付清单verified，partial/完整启动保持原阻塞，accepted=0。未本机/SSH编译、未新增测试/样例/临时业务程序、未真实接口模型或部署。后续代码继续独占worktree实施，不因本次交付自动停止。

以下为历史记录。

# 2026-10-18 继续完成剩余代码：公共事件消费源码

本轮用户授权继续三个子代理实现全部剩余项，及时GitLab检查后双推。原75%停止指令已被取代。新EventConsumerClient只需真实SubscribeEvents/AcknowledgeEvent；inbox.NewEventConsumer绑定严格AcknowledgeEventResult生产方校验器，RunConsumerWithResult返回退出时真实累计计数。runtime及payload校验器必需，未知/屏障暂停，schema/handler/确认失败不确认；已知旧代和ErrEventSkipped仅确认记skipped。同subject串行，分片并发；每次RPC/handler建立当前epoch和新的有界纯服务上下文，清旧proof。未CI全部blocked，不执行真实事件或新增测试。

# 2026-10-18 第二批集成检查通过并已双推源码

自部署GitLab pipeline70（生成311，三个Go分片312/313/314，前端315）全部成功；精确879713字节生成物回传到3deb8ba9cd394ec43274999eaf4f5e0484e224ae；pipeline71对该精确SHA再次全部成功并核对生成零漂移。16个Go模块readonly build/vet/gofmt与go test、完整make check、Vue类型检查与构建通过；go test无测试文件，不代表业务验收。

本域本轮精确清单17项记verified，模块当前84 verified、0 implemented；accepted=0。源码交付 `3deb8ba9cd394ec43274999eaf4f5e0484e224ae` 已普通双推并核对两个远程main一致；本结果记录在同次授权下复查同步。完整服务装配、恢复与其余具体未完项继续按台账blocked/planned，不以编译通过代替业务验收。模块代理均已冻结。

以下为历史记录，其中待CI/真实设计阻塞以最新逐项台账为准。

# 2026-10-18 集成检查通过

pipeline60（生成261、三个Go分片262/263/264、前端265）完整通过，精确生成物已回传。逐项台账只把完整待编译清单转verified；partial、完整启动及既有设计阻塞保持blocked。所有Go模块readonly build/vet/gofmt与go test、完整make check通过；public-site前端类型检查及构建通过。go test没有测试文件，不构成业务验收。accepted=0，未部署。

本批源码汇总d344a37，记录提交等待零生成漂移复核后普通双推。持续向75%推进，用户最新要求及时分批检查/推送，历史停止指令不适用于本次授权。

以下为模块原始交付记录，其中“未CI”只描述交付时。

# 2026-10-17第三轮公共源码交付（候选GitLab通过）

基线d760e39194156403e2b82430d5018fa9f4425e19，根集成会话独占公共文件。用户要求再一轮、三个子代理同时编写，全部编译检查经GitLab，通过后普通双推。

- 服务断言新增可选签名siteId，仅取可信op.Actor.SiteID；省略仍兼容system。无委托时接收服务主体保留签名站点；有委托且同时有签名siteId时必须与actor站点一致。站点不授予用户/对象权限，未签名头无效；清空portal proof会显式删除出站旧头。
- 新WorkerJobsClient只含六个任务控制方法，原JobsClient仍组合事件端口；公共worker.NewJobsClient统一由生成HTTP端口和生产方六个严格Result schema适配。完整保留任务/root/inputRef/worker/fence/epoch/取消/paid refs，不在域复制机制。
- 每次Claim读取当前runtime，使用owner纯服务system、当前epoch和30秒或更早父截止建立独立控制op；这是队列控制，领取后才由域持久批准恢复真实本站，再经签名传到对象RPC。
- 现有生成器支持显式x-runtime-payload-schemas并输出独立PayloadValidationSpec，保留严格生产方schema闭包，避免未被HTTP引用的六任务响应被默认剪除；不扩大HTTP接口或全部内部DTO。
- media交审发现首候选将领域Input的jobId/dataEpoch误放HTTP body，违反生产方额外字段拒绝规则；已改为jobId仅路径、epoch仅可信头，严格body只发RequestBody字段。未经业务执行发现，最终证据以修正后完整GitLab候选为准。
- pipeline30因HTTP合同窄修更新而取消，不计通过。修正候选pipeline31的生成/格式/重复生成与14Go模块三个build/vet/gofmt分片通过，但完整check缺SigningKey/HeartbeatInfo两项类型登记；补机器类型后待新完整候选，不把编译通过当整体验证。
- 公共源码18449ccc9b474c8cf89f94abac5538beac8c2eb3，本轮18项完整新改函数、5实际源码文件按该SHA verified（11项函数首次完成）。pipeline32候选ae7b334dbd9f7c102ade242afe8be6c4ccd676f9生成/格式/重复生成148及三个同时分片149/150/151全部success，覆盖14Go模块readonly build/vet/gofmt和完整make check，生成补丁0；根精确回传后源码未再改。无本机或SSH Go、测试/样例/临时业务程序、SQL/HTTP行为验收、模型、worker/删除、服务或部署。

下一项：根与media顺序集成公共依赖，并在实际树重新运行全部GitLab检查，再逐域集成及本轮普通双推；不运行未齐完整服务。

---

# 2026-10-17 单轮公共前置（候选 GitLab CI 已通过）

本轮用户明确要求三个子代理同时编写一轮后推送，并追加全部编译改经自部署 GitLab、尽量三个并行任务。旧 VPS 三次 SSH 握手失败后停止直连检查，没有本机编译。私有机器管理仓库只读克隆到仓库外，接入凭据不进入本仓库、提交或交接；既有远端双推约定已从最新 main 接续。

- JobExecutionInput 保留完整 ObjectRef、WorkerID 与 fence；平台给执行器进程内 CheckLease 回调。不可逆决定前锁外串行读取 fresh heartbeat 的取消/安全点/有效期，再核 worker/fence；未知失败关闭。
- 单任务独立到期定时器不依赖心跳 RPC 返回；start/heartbeat/check 受已知租约截止限制，旧回执不能复活已停止执行。executor 返回后先排空在途心跳，最终 check/complete/fail 同用本地已知截止，保留 owner 原业务结果与费用根。
- RunRelayBatch 复用每次 IO 的当前运行/安全写窗口；只统计本批已领取事件，jobs 确认后才 relayed，隔离/退避/丢弃均持久完成才计数。持续 RunRelay 委托同一单批机制。
- 新增 GitLab CI 生成/格式阶段和三个 readonly build/vet/gofmt 分片，完整 make check 在基础分片执行；优先专用 Runner，不运行新增测试、业务接口、模型、服务或部署。
- pipeline 11 的生成、格式、重复生成和 AI 分片通过；content 分片因缺 jobs:publishEvent 直接依赖导致生成器裁剪客户端失败。基础分片 build/vet/gofmt 通过，结构检查发现 CI 临时 Git 身份建立顺序、缺 ClaimedJob 类型和实际结构登记。已修相应来源并保留失败事实，等待新候选完整 CI。

公共源码提交62ed7c398b6a080a64f52d8134916170589b3707，七项本轮新改函数与三个完整源码文件已按 pipeline12 完整候选检查转 verified。2026-10-17自部署GitLab pipeline12（候选fccc375）生成/格式/重复gen、三个同时分片覆盖14Go模块readonly build/vet/gofmt及完整make check通过（878文件/1435函数/379HTTP/2328类型）。首轮pipeline11客户端依赖、CI临时身份及结构登记失败已修复并完整重跑；对应公共源码与回传格式一致。只证明当前编译和设计结构，未新增或运行测试/SQL/HTTP行为验收、真实接口/模型、服务或部署。

下一项仅由根顺序集成三个模块，并在每个实际集成树重新通过 GitLab 生成、三分片编译和结构检查；本轮普通双远程推送已授权，R2 与部署未开始。

---

# 持续目标第二轮公共IO错误工具（已检查）

集成会话新增NormalizeIOError，统一已有或包裹的公共错误、原始ctx取消/截止，未知只用可信安全fallback，不传播原存储错误。补登记真实FromError并由jobs响应实际调用，公共Problem保留未知INTERNAL；新域不再复制取消/截止分类。无新路径/外部依赖。

源码7a4f55b5be024deb11b9df821db0c9f7ccaaea62，两个完整函数与对应源码文件已verified；只代表平台现阶段检查，不代替业务验收。

2026-10-16第二轮公共前置：隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，最终make docs/gen、14模块readonly build/vet/gofmt与完整make check通过（872/1332/379/2297）。首次HTTP业务DTO被裁剪编译失败及随后公共FromError未登记可达失败均已修正并完整重跑；仅现阶段检查，无业务验收/真实接口模型/服务部署。

# 两轮并行第二轮公共前置（2026-10-16，已检查）

ServiceConfig新增可选masterKeyRef及NW_MASTER_KEY_REF覆盖，非空仅校验绝对路径。加载配置不读取主密钥，model-gateway的免费读取可独立；密钥实际读取和加密由该域凭据用例负责。无新增函数、路径或依赖；源码993904524e4d7a792a6c84a80606de6215f7ace5，VPS docs/gen、14模块readonly build/vet/gofmt及完整check通过（865/1258/379/2277）；对应文件/函数准确引用源码。未读取真实密钥或执行行为验收。

# platform 状态

第四轮公共CanonicalJSON、有界写窗口/恢复循环与后台安全关闭已完成，源码9ff93ae。公共源和jobs分别集成后实际树均通过VPS docs/gen、14模块readonly build/vet/gofmt及完整check，最终858文件/1105函数/379操作/2253类型，生成零变化；调用方Home Go仍待完整检查，不称行为验收或部署。逐项以progress.json为准。下述早期过程记录保留溯源。

第二轮已实现公共机制通过最终VPS gen/docs、14模块build/vet/gofmt及完整check，函数台账已更新实际提交与证据；调用方未完成的完整启动、guard/peer/cookie和owner机制不因此变成可用。第二轮后已按要求停止；用户随后授权继续，第三轮可信代理来源、每日网络HMAC及浏览器认证类型合成已在冻结合成快照通过VPS docs/gen、14模块build/vet/gofmt与完整check，等待逐模块集成复核。


最新统一集成事实（2026-10-15）：会话签名、委托、可信配置和worker控制/结果链已通过统一复核；调用服务完整装配仍未完成。
VPS顺序集成后make docs/gen、14模块make build和完整make check通过；生成最终零变化。主线实际源码提交：dad2dce，逐项以progress.json为准。未执行业务验收或部署。
以下旧说明作为历史记录，当前状态以前述事实和台账为准。


本轮持续实施集成分支（未合main）：统一会话访问签名/独立秘密、专用委托签名、会话版本、可信公共地址与任务完整结果写回。worker控制操作使用独立幂等键，费用根与executor原操作不变；取消确认由owner负责，权威未知暂停领取。
VPS上一快照14模块build/vet/gofmt与make check通过；委托二跳与代理错误映射的最后修订等待最终复跑。业务验收和部署未执行。

最新集成结果：隔离VPS的14模块make build与完整make check通过；本轮已完成条目转verified，实际主分支SHA见progress.json。部分入口/用例仍未实现，业务验收和部署未开始。下列本机与待登记说明保留为交付时历史事实。

- 阶段：R1-00 已实现并检查（分支 codex/platform-r1-00，基线 d7cf792431d8，交付 fa70c657322c；台账全部 verified，行为验收待R2）。
- 范围：libs/platform 全部17个约束函数与共享类型包（platform.go）、nw-keys、scripts/gen_code.py 与 make gen；15域生成物可重现并编译通过。
- 检查：本地与隔离VPS（go1.25.5、8核）go vet ./...、go build ./... 通过；15个模块生成物经临时模块逐域编译验证（本地与VPS各15/15）；make check 本地与隔离VPS通过；make gen重复运行无差异。未运行业务验收。
- 契约修订（已同步 implementation.json/types.json 并 make docs）：新增 libs/platform/platform.go 共享类型包；BuildOperationContext 增加 self 参数；InvokeModelRequest 增加 onEvent 回调；RelayConfig/ConsumerConfig/WorkerConfig 增加 runtime 钩子（ConsumerConfig 另有 validatePayload）；WorkerConfig 增加 owner；DelegationExpectation.route 允许下游转发留空。跨域修订：jobs ClaimedJob 增加 attempt 与 dataEpoch（领取结果是JobExecutionInput唯一来源）。
- 实现时确认的口径：nw-keys 读取的 deploy/config/services.json 按 {version,services:[{slug,secretsDir}]} 解析并容忍其余字段，deploy实施时对齐；私钥为PKCS8 PEM（0600），公钥为base64url；trust-anchors.json排序输出；重放缓存与私钥缓存均在进程内。

详细交接见 docs/handoff.md；逐文件/函数台账见 progress.json。
- 2026-10-15 补充：新增 `lifecycle.NewSiteConfigClient` 与 `apivalidate.NewRequestValidator`（平台库新增 kin-openapi 依赖）；make build 与 make check 通过，详见 [handoff](docs/handoff.md)。

- 本轮三个工作包集成：新增作用域绑定的分页游标签名/验证，CheckLease补workerId；平台单模块编译/vet通过，新增函数已按隔离VPS全量检查转verified。

本次最终检查：2026-10-16持续目标第二轮公共前置：VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、14模块readonly build/vet/gofmt和完整make check通过（872/1332/379/2297）。HTTP用实际RequestBody，dataEpoch仅可信上下文；公共错误分类有真实FromError调用。无新测试/业务验收/真实接口模型/服务部署。
