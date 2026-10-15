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
<!-- flows:end -->
