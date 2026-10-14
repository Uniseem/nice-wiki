# jobs 流程

异步领域任务：领域保存不可变输入/操作记录与outbox→jobs.createJob→owner领取job/fence→owner验证epoch+lease后执行→每付费阶段先查询gateway幂等operation结果→持久业务结果→jobs complete。jobs只处理任务状态，不运行领域算法。

至少一次事件：源事务或文件恢复日志确定提交后固定eventId写outbox→生产方转发循环调用publishEvent→jobs按订阅目录（config/event-subscriptions.json）为每个consumer建delivery→消费方ConsumeEvents经subscribeEvents领取（30秒确认租约）→handler本地inbox去重及sourceRevision CAS并提交→acknowledgeEvent；未确认到期由RequeueExpiredDeliveries重排，超过10次转dead-letter；生产方或消费方停机都不丢事件。portal浏览器转发用ephemeral订阅，不建delivery。

维护安全点：site-config maintenance开启→jobs停止新入口→每域拒新写并等在途安全点→返回safe水位→全域safe才导出。超时保持维护并明确failed，不能宣布一致备份。

恢复管理校验：维护前由identity为每域签发RecoveryAuthorization；本域控制记录/屏障/信任锚独立于业务导入。维护中的prepare/commit/rollback按本地签名、audience、restore/barrier、两epoch、到期和决策状态校验，不回查已换代身份业务DB；普通业务不接受此许可；全域完成后撤销。

worker拉取映射：public-site.render-publication→public-site PrewarmPublication；analytics.aggregate-hourly/daily/usage/reconcile→AggregateHourly/AggregateDaily/AggregateUsage/ReconcileUsage；data-protection.backup/restore/mirror→CollectSnapshots/PrepareAllDomains+ActivateAllDomains/BuildMirror；各AI任务owner按根contracts/ai-tasks.json和本域implementation函数ID映射。owner主动claim/start/heartbeat/complete/fail，jobs不调影子execute-task接口。

状态迁移：

queued→leased→running→succeeded；失败running→retry-wait→queued或dead-letter；queued/retry-wait→cancelled；running→cancel-requested→cancelled/succeeded

lease fencingToken 单调增加；expired lease 重排时旧worker结果必须拒绝

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
