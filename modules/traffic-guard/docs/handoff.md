# 2026-10-21 SnapshotManager drain 复核

`BeginDrain`/`Drain` 的真实边界已核对：它们读取并轮询 `guard_permits` 的 active 行、`guard_challenges` 的 issued 行、`guard_waiters` 和 outbox 活动水位，不自动清理这些事实。当前已有的 `ExpireLeases` 只处理普通后台到期许可；挑战没有独立到期终结入口，等待队列只有原 QA 会话按 requestId 取消/消费路径。

`AuthorizeCleanupOrigin` 保持 `FORBIDDEN`。traffic-guard 没有可验证的旧付费执行 origin，也没有 barrier 专用 cleanup 资格，不能把 data-protection/jobs 的随机 root 当成清理授权。active permit、issued challenge 或 waiter 未归零时，Drain 会继续等待，截止到达后失败关闭；它不会把本域标为 safe。

本域已在 [guard_runtime.go](../internal/guard_runtime.go) 构造真实 SnapshotManager 并从 BuildApplication 注入，但 safe/export 仍 partial，完整 BuildApplication 继续 blocked。全域 `VerifyGlobalSeal` 继续返回依赖不可用，等待 jobs 协调 writer。未运行本机/SSH Go、gofmt、build、vet、test、pnpm、make check 或CI。

# 2026-10-20 续作修正：复用集成平台统一 outbox work gate

上一版源码提交 `921537adc04010bc7abaa6909a197c3467ea2770` 新增了未在原合同登记的 `0004_work_gate.sql` 与 `snapshot_work.go`；该方案已由 `09328bf3797fa2923a1f9a24bd57f32a6a527de3` 精确撤销。当前新增源码提交为 `e611720a33f56a22e012dc4b45849437a649f8fa`，只改已登记的 `internal/guard_runtime.go`、`internal/policy_store.go`、本域合同和官方生成文档。

## 当前实现

- `RunMigrations` 完成后调用集成平台 `outbox.RegisterWorkTable` 登记本域全部业务表：策略、回执、登录表、主体/别名/桶、许可、挑战、封禁、问答信号、风险事件、等待队列、身份修订及许可网络关联。
- `ObserveWork` 以同一 SQLite 读事务调用 `outbox.Store.ObserveTx`，把 `WorkRevision` 绑定 `nw_outbox_control.revision`，逐表读取 pending login attempt、active permit、issued challenge、等待队列；未知表或读取失败关闭。
- `SealWork` 在同一事务重读 revision 和待决事实，再调用 `SealTx`；`ResumeWork` 在同一事务调用 `ResumeTx`。平台共享 gate 负责阻止登记业务表和 outbox 写入，因此没有第二份本域水位或触发器。

## 状态与阻塞

本轮源码未进入已交集成的 `9b359c0e` 精确 CI；本机/SSH Go、gofmt、build、vet、test、pnpm、make check 均未运行，相关文件和三个函数保持 `blocked`。当前分支没有保留 `0004_work_gate.sql` 或 `snapshot_work.go`。

若平台集成 SHA 未进入本域实际构建树，`RegisterWorkTable`/`ObserveTx`/`SealTx`/`ResumeTx` 会使本域构建保持待CI；该公共依赖由主代理集成，不在本域复制。

# 2026-10-20 续作：受控快照归档读取源码单元

基线为 `main=6204dfc94510f28752fe8da81df9f3a4d3ce781d`，工作树分支为 `codex/traffic-guard-20261020-r1`，独占范围仍为 `modules/traffic-guard/`。旧分支 `codex/traffic-guard-20261018-bulk` 仅保留作历史引用；主线已含三批风控源码的等价提交，因此本次没有强行解决跨域合并冲突。

## 本次交付

- 源码提交：`0c0e2e17e00adf2bb159de97c31f7bf8b58892b0`（`traffic-guard：接入受控快照归档读取`）。
- [internal/snapshot_archive.go](../internal/snapshot_archive.go)：实现 `ReadSnapshotArchive`。它只接受 `data-protection` 调用方及可信 admin 会话，读取 site-config 运行权威，要求维护中的 `backup_barrier` 或带 restoreId 的 `restore_staging`，核请求代次，固定本域 `ObjectRef`，再调用平台 `SnapshotAPI.OpenArchive`。平台负责完成回执、manifest 绑定、文件长度和 SHA-256 核对；本域不接受路径、不写入、不删除、不重算归档。
- [internal/http_management.go](../internal/http_management.go)：把 `readSnapshotArchive` 接入统一 handler map，成功响应只写固定媒体类型和长度的 tar 流；错误仍走公共 Problem。
- [internal/app/wire.go](../internal/app/wire.go)、[internal/guard_runtime.go](../internal/guard_runtime.go)：登记 data-protection 调用方与 maintenance/controller scope，并把可信运行权威和 `SnapshotAPI` 留作启动装配输入。当前 `SnapshotAPI` 尚未由 `BuildApplication` 构造，缺依赖时保持失败关闭。

## 检查与状态

只运行了 `git diff --check` 和 JSON/文本静态复核，未运行 Go、gofmt、build、vet、test、pnpm、make check，也未调用真实接口、模型、服务或部署。源码单元、被改的 handler/runtime/wire 文件和台账对应函数均保持 `blocked`，不能据此记 `implemented` 或 `verified`。

## 阻塞与下一项

完整启动仍受真实 `SnapshotManager` 回调、WriteWindow、独立预算key、剩余 snapshot/barrier handler、后台/事件循环和 activation/restore 回执阻塞。下一项只做一件事：由主代理在本域启动装配中对照 `libs/platform/lifecycle.SnapshotConfig`，接入 traffic-guard 的真实 `ObserveWork`、`SealWork`、`Capture`、`MigrateStage`、`ValidateStaged`、`StageBudget` 和跨域封口证明回调；任何回调缺失继续拒绝返回可运行 `Application`。

# 2026-10-18 续写第二批检查通过

GitLab pipeline84（源码72219d7；生成381、三个Go分片382/383/384、前端385）全部成功，16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及前端类型检查/构建通过。pipeline83各编译分片通过但缺既有信封校验登记，已补并复检；实际普通测试无测试文件。源码生成物由GitLab工具回传，不代表业务验收、隔离行为验收或部署。

本域本批1项完整函数通过源码检查，完整启动/快照恢复/业务accepted状态保持未完成。模型网关、知识、数据保护三个worktree继续，未在本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 2026-10-18 第二批集成检查通过并已双推源码

自部署GitLab pipeline70（生成311，三个Go分片312/313/314，前端315）全部成功；精确879713字节生成物回传到3deb8ba9cd394ec43274999eaf4f5e0484e224ae；pipeline71对该精确SHA再次全部成功并核对生成零漂移。16个Go模块readonly build/vet/gofmt与go test、完整make check、Vue类型检查与构建通过；go test无测试文件，不代表业务验收。

本域本轮精确清单58项记verified，模块当前75 verified、4 implemented；accepted=0。源码交付 `3deb8ba9cd394ec43274999eaf4f5e0484e224ae` 已普通双推并核对两个远程main一致；本结果记录在同次授权下复查同步。完整服务装配、恢复与其余具体未完项继续按台账blocked/planned，不以编译通过代替业务验收。模块代理均已冻结。

以下为历史记录，其中待CI/真实设计阻塞以最新逐项台账为准。

# 风控持续集中源码交接（2026-10-18）

基线0a69a5a5d7f903cc411d7556f5c3a6269b5db5c2；独占分支codex/traffic-guard-20261018-bulk，仅modules/traffic-guard/。首批源码e8d9215，未push。根公共96ed769/6b6d9d2以13a5cfd/f60caa6合入本worktree；framework只保留当前完整清单并集补路径，未私改公共业务规则。集成时只需cherry-pick本域源码及记录，不必重复合入这两个公共等价提交。

## 首批真实边界

- GET策略站点仅可信op.Actor.SiteID；缺行NOT_FOUND，不临时捏造默认。Update先即时核管理员，首次expectedRevision=0→1，后续CAS；策略、原安全回执与outbox同事务。HTTP统一入口已即时查权，内部私有marker只复用该次结果避免第二次identity调用。
- Ban仅已知匿名网络引用、ai scope、manual/abuse原因；0取600秒，最长30日。状态/修订、匹配网络别名租约撤销、回执/outbox同事务。只限制AI，公开阅读不受影响。
- 三层桶整数补充（第二批补齐网络），时钟倒退补0且水位不倒退；三层同时足够才扣固定1个频率令牌，可复用准入外层事务，后续并发失败回滚。没有钱账或金额状态。
- QA报告只收sessionKey/networkKey/questionHash/modelVerdict，原QA risk=abusive显式映射abuse；similarityScore/intervalMs可选，缺失SQL NULL且不计0观测。根questionOperationId以可信op.RootOperationID去重。返回决策封闭allow/deny/challenge；未知响应必须失败关闭。
- 队列mode兼容可选enqueue/cancel；cancel只删同站同代同session同request等待引用，不释放active lease。正文/FIFO执行由QA拥有，guard不在排队时扣桶或费用。最多10等待。
- 扫描最多500主体项，过期租约不返桶；过期ban同事务outbox。匿名问题/风险信号10分钟、空闲桶24小时、终局permit7天、终局ban30天；活跃ban/lease不删除。

## 未完成及集成要求

完整服务仍blocked。NewGuardHandlers已把真实管理/报告/队列绑定交公共NewServiceHandler，但运行写窗口、独立预算key、许可/挑战、控制恢复和后台等仍缺，BuildApplication不返回可运行容器。未跑GitLab前所有新代码blocked，不能因本域有源码或前序库检查通过而记verified。

根统一make gen/docs、readonly build/vet/gofmt和make check，本机未执行Go或SQL/HTTP行为验收，未新增测试/fixture/mock/样例/临时验证程序，未启动服务或调用真实模型。

预算key独立于服务签名key，固定30日current+previous网络别名必须覆盖最长30日ban过渡。不得因服务签名key轮换丢旧ban；预算key及旧key过渡材料在control/外部secret中，不随业务快照替换。缺可信op.NetworkBudgetScopes或预算key时准入必须失败关闭；不回退原IP、UA或客户端自报network。

第一批下一项（已由第二批完成）：internal/permit_store.go实现AcquireConcurrency/RenewPermit/ReleasePermit，以及permit_sign.go复用平台SignAdmissionPermit/VerifyAdmissionPermit。持久ownerCaller与固定模型网关audience分开；续租/消费只用原request/subject/network/epoch，不再扣桶或费用。

## 首批待CI精确函数ID

- `traffic-guard.GetPolicy`
- `traffic-guard.UpdatePolicy`
- `traffic-guard.ClassifyAction`
- `traffic-guard.LoadSignals`
- `traffic-guard.ComputeRisk`
- `traffic-guard.RefillBucket`
- `traffic-guard.ConsumeBuckets`
- `traffic-guard.ExpireLeases`
- `traffic-guard.ExpireBans`
- `traffic-guard.CreateBan`
- `traffic-guard.RevokeBan`
- `traffic-guard.ListBans`
- `traffic-guard.PurgeAnonymousSignals`
- `traffic-guard.ReportQuestionSignals`
- `traffic-guard.QueueSessionAdmission`
- `traffic-guard.ListRiskEvents`
- `traffic-guard.createNetworkBan`
- `traffic-guard.WithGuardRuntime`
- `traffic-guard.beginGuardTransaction`
- `traffic-guard.guardTransaction.close`
- `traffic-guard.guardTransaction.commit`
- `traffic-guard.authorizeGuardManagement`
- `traffic-guard.readGuardReceipt`
- `traffic-guard.saveGuardReceipt`
- `traffic-guard.enqueueGuardEvent`
- `traffic-guard.readGuardPolicy`
- `traffic-guard.NewGuardHandlers`


## 第二批准入许可源码5b44ea5

新增10个原有主业务函数及8个必要助手；均blocked待根GitLab。根98928f0已以effeef0合入，集成只pick本域5b44ea5及其记录提交。新增字段需先统一gen，qaPerNetworkPerMinute兼容可选，预计Go生成为*int。已同步真实WithGuardRuntime/事务收尾依赖，修复首批孤儿。HTTP新增create/renew/release/consume/verify的真实映射，caller来自可信op不收用户自报；Verify可选token。

独立预算key与保留旧key仅由可信GuardRuntime提供，完整配置/写窗口/挑战/后台/恢复仍未齐，BuildApplication保持失败关闭，不能据本批将服务启动或行为验收标完成。未新增依赖/测试/fixture/mock/样例或临时验证程序，未本机运行Go/SQL/服务/真实接口；仅git diff --check。下一项继续挑战PoW与事件，不补测试。

### 第二批新增待CI精确函数ID

- `traffic-guard.DeriveSubject`
- `traffic-guard.EvaluateAdmission`
- `traffic-guard.HandleAdmission`
- `traffic-guard.AcquireConcurrency`
- `traffic-guard.RenewPermit`
- `traffic-guard.ReleasePermit`
- `traffic-guard.SignPermit`
- `traffic-guard.ValidatePermit`
- `traffic-guard.ConsumeAdmission`
- `traffic-guard.VerifyAdmissionHTTP`
- `traffic-guard.networkPolicyRate`
- `traffic-guard.readPermitRecord`
- `traffic-guard.permitMetadata`
- `traffic-guard.CheckBan`
- `traffic-guard.deriveSessionScopes`
- `traffic-guard.recordAdmissionDenial`
- `traffic-guard.admissionRateError.Error`
- `traffic-guard.admissionRateError.Unwrap`

### 第二批已修改待重新CI

- `traffic-guard.ValidatePolicy`
- `traffic-guard.UpdatePolicy`
- `traffic-guard.ClassifyAction`
- `traffic-guard.ConsumeBuckets`
- `traffic-guard.QueueSessionAdmission`
- `traffic-guard.readGuardPolicy`
- `traffic-guard.NewGuardHandlers`

消费核验窄修e61d899：VerifyAdmissionHTTP成功响应增加必需admissionId；无token路径也须consumed_request_id=原QA根，未消费行失败关闭。同步types/implementation，仍blocked待CI。

动作/释放窄修2e3074d：qa.ask/qa.followup统一qa次数桶类别，单次问题root不因追问换桶；qa.conversation不签许可。Release核原owner/site/epoch/root，终止/过期/已撤销均已无并发占用，幂等released=true。仍待CI。


## 第三批源码f5f036b，冻结待根统一检查

新增6个原有主业务和5个必要挑战助手，累计32个原有主业务源码，全部blocked待CI。本域不再扩功能或push；根统一gen/docs、readonly build/vet/gofmt和完整check。本机仅Git/文本审阅，无Go/SQL/接口/服务/模型调用，未新增依赖、测试、fixture、mock、样例或临时验证程序。

挑战18/22bits、120秒、6次/分钟、2active及5失败上限真实持久；外部适配固定供应方HTTPS并真实校验success/hostname/action/cdata/期限，未实际调用。HTTP可选visitorToken重验原人，passedToken只由平台签10分钟证明。portal需设置nw_visitor HttpOnly/Secure/SameSite=Lax并移除浏览器JSON passedToken；供应方widget action=traffic_guard、cData=challengeId。TurnstileConfig的siteKey/secretRef/expectedHostname与预算keyring/WriteWindow尚需可信启动配置，未装配失败关闭，不把未来完整配置当现有可运行服务。

身份事件载荷通过生成IdentityChanged规格+平台NewPayloadValidator，inbox/最大权限修订同事务，管理仍每次即时查identity。PublishGuardEvents只复用平台RunRelayBatch和NewEventPublisher；ConsumeEvents缺公共窄client与准确退出统计，按根决定保持partial，不造worker假方法或空计数。完整控制恢复/后台装配仍未齐。

根结构审查修复：删除旧NormalizePeer函数/类型/台账，代理链及网络预算由平台唯一实现；CheckBan只保留permit_store.go实际助手，旧空internal/ban.go未来文件合同删除，根framework需同步去除该路径。真实SignGuardSession调用/依赖由VerifyChallenge补齐。

最终保留qa.conversation免费准入动作，owner=qa且执行AI挑战/ban和共享qa次数桶；Consume/Renew仅元数据空token。SignPermit与ValidatePermit只接受qa.ask/followup。ReportQuestionSignals optional banExpiresAt使用omitempty，无ban不再输出空Timestamp。所有直接HTTP结果可选时间字段已按此审阅，EvaluateAdmission仅内部可保留显式null引用。

### 第三批新增待CI精确函数ID

- `traffic-guard.IssueChallenge`
- `traffic-guard.VerifyChallenge`
- `traffic-guard.VerifyPoW`
- `traffic-guard.VerifyExternalChallenge`
- `traffic-guard.ConsumeIdentityEvent`
- `traffic-guard.PublishGuardEvents`
- `traffic-guard.readChallengeRecord`
- `traffic-guard.readChallengeSnapshot`
- `traffic-guard.checkIssuedChallenge`
- `traffic-guard.challengeNonce`
- `traffic-guard.challengeResult`

### 第三批已修改待重新CI

- `traffic-guard.ClassifyAction`
- `traffic-guard.ConsumeBuckets`
- `traffic-guard.SignPermit`
- `traffic-guard.ValidatePermit`
- `traffic-guard.ConsumeAdmission`
- `traffic-guard.RenewPermit`
- `traffic-guard.OpenPolicyStore`
- `traffic-guard.NewGuardHandlers`
- `traffic-guard.ReportQuestionSignals`
