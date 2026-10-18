# jobs 流程

异步领域任务：领域保存不可变输入/操作记录与outbox→jobs.createJob→owner领取job/fence→owner验证epoch+lease后执行→每付费阶段先查询gateway幂等operation结果→持久业务结果→jobs complete。jobs只处理任务状态，不运行领域算法。

至少一次事件：源事务或文件恢复日志确定提交后固定eventId写outbox→生产方转发循环调用publishEvent→jobs按订阅目录（config/event-subscriptions.json）为每个consumer建delivery→消费方ConsumeEvents经subscribeEvents领取（30秒确认租约）→handler本地inbox去重及sourceRevision CAS并提交→acknowledgeEvent；未确认到期由RequeueExpiredDeliveries重排，超过10次转dead-letter；生产方或消费方停机都不丢事件。portal浏览器转发用ephemeral订阅，不建delivery。

维护安全点：site-config maintenance开启→jobs停止新入口→每域拒新写并等在途安全点→返回safe水位→全域safe才导出。超时保持维护并明确failed，不能宣布一致备份。

恢复管理（ADR0003）：本域control区（屏障、恢复日志）不随业务恢复替换。stage/discard只接受data-protection服务身份与superadmin委托；runtime进入restore_switching或restore_rolling_back后，本服务安全退出并由进程守护重启，启动时换入或换回active目录并向site-config登记回执。

worker拉取映射：public-site.render-publication→public-site PrewarmPublication；analytics.aggregate-hourly/daily/usage/reconcile→AggregateHourly/AggregateDaily/AggregateUsage/ReconcileUsage；data-protection.backup/restore/mirror→CollectSnapshots/PrepareAllDomains+ActivateAllDomains/BuildMirror；各AI任务owner按根contracts/ai-tasks.json和本域implementation函数ID映射。owner主动claim/start/heartbeat/complete/fail，jobs不调影子execute-task接口。

状态迁移：

queued→leased→running→succeeded；失败running→retry-wait→queued或dead-letter；queued/retry-wait→cancelled；running→cancel-requested→cancelled/succeeded

lease fencingToken 单调增加；计数保存在job记录且不随lease释放丢失。claim每批请求1～10个，实际取min(limit,10,owner剩余在途容量8)，按notBefore/createdAt/jobId排序。同claim操作重放原批次，不续租；expired lease 重排时旧worker结果必须拒绝。

心跳可选携带安全阶段代码与计数；HeartbeatJob先在本事务核对owner/epoch/worker/fence和有效expiry，再调用AppendProgress并统一提交续租、进度、outbox及回执。维护期间续租截止不晚于control屏障deadline；重复心跳不延长原expiry；当前屏障截止更早时返回缩短后的有效expiry，取消/安全点标志读取当前状态。

事件accepted→delivering→acknowledged；消费者独立按eventId/revision去重，顺序仅producer/subject修订

屏障open→draining→safe→released；timeout→failed后不自动解维护

并发和幂等：每个修改函数明确CAS、fencing/租约或事务边界，见functions；公共事件至少一次，不能依赖全局顺序。接收eventId相同但hash不同拒CONFLICT；较低sourceRevision记已见但不应用；旧dataEpoch拒绝并不能重绑新epoch。


本轮登记的平台任务类型与owner实现函数ID（仅owner worker执行）：

| jobType | owner | 本域函数ID |
|---|---|---|
| `public-site.render-publication` | `public-site` | `public-site.PrewarmPublication` |
| `analytics.aggregate-hourly` | `analytics` | `analytics.AggregateHourly` |
| `analytics.aggregate-daily` | `analytics` | `analytics.AggregateDaily` |
| `analytics.aggregate-usage` | `analytics` | `analytics.AggregateUsage` |
| `analytics.reconcile-usage` | `analytics` | `analytics.ReconcileUsage` |
| `data-protection.backup` | `data-protection` | `data-protection.CollectSnapshots`, `data-protection.FinalizeBackup` |
| `data-protection.restore` | `data-protection` | `data-protection.PrepareAllDomains`, `data-protection.WriteRestoreDecision`, `data-protection.ActivateAllDomains` |
| `data-protection.mirror` | `data-protection` | `data-protection.BuildMirror` |

AI任务归属及上限由根contracts/ai-tasks.json唯一登记；各AI owner的工作流登记其函数ID。jobs只校验目录，不能嵌入领域算法或发影子execute-task请求。

管理列表：当前后台身份与入口job.manage粗能力→当前epoch安全元数据→释放SQL事务→按持久目录具体权限逐资源核权→只用有权条目形成分页和签名游标。job状态或权限可跨页变化，每页重新核权；游标不承诺跨请求冻结快照，也不代替权限。

人工重试：当前具体业务权限→原操作回执优先→当前dead-letter快照→释放锁后owner持久确认→重核当前epoch/control与revision/fence→completed复用到succeeded或safeToRetry沿已知原根排队→结果、fence、outbox、原回执同事务。orphan/取消意图、未知费用不重排；原结果矛盾不覆盖。完整启动与owner适配仍未完成。

进度流：统一入口核服务/当前会话/epoch及三个游标别名结构→绑定处统一数值（冲突400）→持久job具体权限→续接/保留范围预检→actor/site连接槽→每批当前权限/epoch与最多64行读取→直接progress JSON帧。无数据释放事务再等，公共pump统一心跳/背压/写截止；取消、断连、权限失效或epoch变化关闭，重新开流重新核权。原用户请求deadline不延长。终局追平仅EOF，不造完成进度或返回空数组冒充live流。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

本域不消费事件。

### 异步任务

本域没有登记的异步任务类型。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `jobs.RequeueDue` | 每1秒 | 到期重排 |
| `jobs.ReapExpiredLeases` | 每5秒 | 处理失联worker |
| `jobs.TickSchedules` | 每1秒 | 定时生成任务 |
| `jobs.RequeueExpiredDeliveries` | 每5秒 | 把已经推送给订阅者但确认租约到期的投递重新排队或转死信 |
| `jobs.PurgeExpiredHistory` | 每日 | 清理已完成记录 |
| `jobs.RelayOutbox` | 进程生命周期持续转发 | 本域outbox转发薄入口，完整复用平台SQL store与转发循环 |
<!-- flows:end -->


终局与过期恢复：owner以workerId/fence写回已经持久的完成或安全失败；jobs只存引用并原子释放lease、写状态与回执。无结果任务可完成；已取消模型的费用仍由owner/gateway收敛，CANCELLED只在cancel-requested且有效租约下确认。完成先提交则取消返回已有succeeded。

过期先失效持久fence并隔离orphan，不直接假定未收费。受信OwnerRecoveryChecker返回completed时复用已有持久结果，safeToRetry证明无未决收费且保留根/checkpoint时才有限重排；pending/unknown/不可达继续orphan。实际owner版本化适配仍待装配，不能用健康检查或空返回假装确认。历史根未知不生成新付费根。


任务管理最终授权按持久owner/type匹配jobTypes的managementPermission，入口job.manage只表示可进入队列界面，不能代替领域功能授权。授权HTTP请求不持有进程Store锁或SQLite事务，随后读/取消仍在当前epoch事务内核对同一任务作用域。取消已持久但owner尚未看到心跳时，临时失败也不能重排；先orphan等待可信安全确认。


第二轮事件核心：producer outbox以固定eventId提交，jobs同事务保存完整信封/hash、全局eventSeq、所有登记consumer的queued投递与回执后确认。consumer只领取自己的订阅，30秒内完成本域inbox/业务事务后ack；ack推进连续水位，传输Last-Event-ID不能替代它。到期重排只改对应consumer，死信保留并阻塞相同producer/subject后序，需按具体生产方管理权限重放原投递。

jobs自身outbox同样经RelayOutbox→平台RunRelay→版本化self publishEvent，不能直接跳过持久接收。durable流逐次拉取、单事件背压并可取消；浏览器ephemeral等待独立安全投影，当前明确不可用，不以空列表或全量payload广播冒充。

claimJob可选jobTypes真实SQL筛选且只本owner登记类型；用于分开父构建/子提取worker执行名额，禁止领取后本地丢弃。省略保留旧全owner语义。六个worker租约HTTP入口只绑定已严格校验输入，epoch来自可信op，维护期间仅原heartbeat/check/complete/fail收尾由原租约权威裁定。完整服务装配仍未齐。
