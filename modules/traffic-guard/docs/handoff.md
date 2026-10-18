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
