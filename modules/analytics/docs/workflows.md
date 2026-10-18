# analytics 流程

old D31：浏览器加载sendBeacon一次→UA过滤已知爬虫→可信peer/IP+UA+每日随机盐hash→离线GeoIP国家/来源domain→30天匿名明细→每日PV/UV/热门文档/来源/国家/语言汇总长期保存；无cookie/第三方/原IP落盘。Markdown AI爬虫独立计数。

审计：领域操作提交与outbox固定eventId→analytics校验生产者/action/资源ref→允许元数据→追加哈希链→权限受控查询；领域审计源随自身快照保留。

费用：gateway账项settled事件→按ledgerEntryId/revision投影→整数micros聚合→每日固定watermark只读对账；汇总缺漏不影响实时gateway额度判定。

费用V2消费候选：model-gateway事件只提供引用；analytics按固定watermark收齐V2 `changes+observations`，每页经 `AnalyticsUsageStatisticsPageResult` 生成的严格schema与 pageHash 核验，只有末页游标为空且throughRevision到达watermark才形成完整快照。settled须匹配同entry权威账项，reconciled not_charged须匹配同attempt观察；随后以inbox首写和V2事实投影同一事务提交，提交后才确认事件。金额unknown、held和charge_unknown保留原未知状态，不从事件金额或revision补猜。源码候选为blocked，待GitLab恢复统一gen/build。

恢复管理（ADR0003）：本域control区（屏障、恢复日志）不随业务恢复替换。stage/discard只接受data-protection服务身份与superadmin委托；runtime进入restore_switching或restore_rolling_back后，本服务安全退出并由进程守护重启，启动时换入或换回active目录并向site-config登记回执。

状态迁移：

beacon validated→queued→persisted/dropped；dropping只影响匿名计数

审计accepted→append-only；保留期清理也写retention操作引用

费用projection absent→settled/reversed/reconciled；只接受gateway账项修订，不猜估金额

并发和幂等：每个修改函数明确CAS、fencing/租约或事务边界，见functions；公共事件至少一次，不能依赖全局顺序。接收eventId相同但hash不同拒CONFLICT；较低sourceRevision记已见但不应用；旧dataEpoch拒绝并不能重绑新epoch。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `invocation.settled` | 1 | `analytics.ConsumeAnalyticsEvent` | 仅账项引用；收到后调用gateway.queryUsage拉权威安全ledger投影，不猜金额 |
| `qa.turn.completed` | 1 | `analytics.ProjectQaTurn` | 统计问答量、缓存命中与无答案比率 |
| `qa.turn.failed` | 1 | `analytics.ProjectQaTurn` | 统计问答量、缓存命中与无答案比率 |
| `invocation.reconciled` | 1 | `analytics.ConsumeAnalyticsEvent` | 人工证明更正费用原引用，缺权威安全统计页时保持未确认，不猜金额或账项修订。 |

### 异步任务

#### `analytics.aggregate-hourly`

- 执行函数：`analytics.AggregateHourly`
- 触发：系统定时
- 系统定时：`5 * * * *`

1. 可信本域worker注入JobExecutionInput与不可变SourceWatermark，HTTP不得自报。
2. 锁外CheckLease，事务内当前epoch/同job/worker/fence/attempt/expiry；当前period来源必须等原输入，改变则STALE_RESULT，新输入另计划。
3. 完整已封闭UTC小时按document/locale count与dailyHash distinct，时延分桶；同事务替换桶/真实水位/outbox/原结果，提交前再核镜像截止。
4. 超过原明细30天且缺原聚合失败关闭，保留历史真实聚合不重建0。

#### `analytics.aggregate-daily`

- 执行函数：`analytics.AggregateDaily`
- 触发：系统定时（站点时区）
- 系统定时：`10 0 * * *`

1. 唯一可信ReportingTimeZone必须非空且等任务timezone，DST以AddDate日历日完整封闭边界。
2. 可信worker注入真实执行与不可变来源水位；锁外CheckLease/事务内fence及来源CAS，迟到变化新输入不改旧输入。
3. document/locale总UV在所有维度间distinct；source/country维度另表，不SUM维度UV；维度最多100000否则事务失败。
4. 完整替换日总量/维度与原结果/outbox/真实修订；提交前核租约镜像；原明细已过保留而缺原结果不可重建0。

#### `analytics.aggregate-usage`

- 执行函数：`analytics.AggregateUsage`
- 触发：系统定时
- 系统定时：`20 * * * *`

1. 按gateway账项最终state取整数micros
2. 按class/provider/model group
3. 事务replace day聚合

#### `analytics.reconcile-usage`

- 执行函数：`analytics.ReconcileUsage`
- 触发：系统定时；ReleaseBarrier恢复后创建
- 系统定时：`30 3 * * *`

1. 验system.manage
2. 获取gateway固定ledgerwatermark
3. 分页校验缺漏/修订
4. 事务replace projection/aggregate并写discrepancy事件

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `analytics.FlushBeacons` | 每5秒 | 批量持久匿名条目 |
| `analytics.PurgeBeacons` | 每分钟有界续批；仅30天前明细 | 清理短期匿名行 |
| `analytics.PurgeAuditHistory` | 每日 | 审计保留期清理 |
| `analytics.PurgeExports` | 每小时 | 清理导出临时对象 |
| `analytics.PublishAggregateEvents` | 启动后持续，空闲每1秒 | 派生事件outbox派发 |
| `analytics.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `analytics.RunWorker` | 启动后持续 | 主动领取并执行本域登记的持久任务 |
| `analytics.SchedulePageAggregates` | 每分钟，完整封闭小时5分钟/本地日10分钟后，迟到真实period水位修订补新输入 | 统计任务参考支持：SchedulePageAggregates |
<!-- flows:end -->

R1收尾首切片：私有SQLite V1只在空未版本化库事务建表，未知旧库维护拒绝；siteId严格来自site-config:readSiteConfig，epoch来自runtime，不造身份。控制屏障由公共机制持有，不放active表；盐不进入快照。日报总PV/UV独立于source/country维度表，UV不可SUM维度distinct。

页面查询兼容粒度hour/day：完整UTC小时或唯一部署ReportingTimeZone的完整日历日，默认7完整日/168小时；显式边界必须对齐，day以日历366日和DST计算。返回真实粒度/fromUTC/toUTC，水位取已聚合pages，不拿新beacon水位冒充已聚合。日盐与日汇总共用可信报表时区，未配不假设UTC。审计游标签名复用平台，固定seq上界和保留锚；身份/过滤/锚变化拒混页。审计链V1以UTF8前hash十六进制串拼固定canonical payload，含内部安全metadata但不将原metadata作为HTTP输出。

V2审计事实：固定producer/action，HTTP历史三字段同现且纯服务提交原actorId；旧实时仍当前主体。异步只原业务事务outbox→jobs→ConsumeAuditEvent，origin显式取已验证Envelope.Producer，不伪op调用方；payload采用AuditRecordedPayload，原site/epoch/operation/time全部哈希。新成功OK，兼容HTTP SUCCESS别名；V1 hash只读保留，新增行schemaVersion=2。链内来源与安全metadata、inbox和唯一auditId同事务；重复信封不重新插入被保留期删除的旧事实。

首批readSiteIdentity误将“免费控制RPC”写成非法costClass=free，现清为空；合法非空类别仅qa/background/admin-probe。只做静态修正，未以编译当接口验收。

匿名统计实施格式：ReportingTimeZone在当前代次持久固定，首次仅running且无未知旧匿名明细绑定，不默认UTC；HMAC载荷canonical[siteId,dataEpoch,day,规范IP,小写折叠空白UA]，每日本机control独立32字节盐，盐与原IP/UA不进快照。portal私有签名正文POST /v1/internal/beacons为唯一真实访客地址来源，本域不解析代理头；公开原正文2048、签名内部正文4096，队列4096、每分钟3000。flush真实新增计数，空队列漏数亦持久，小时/本地日分别记来源水位；历史聚合不能因全局新访问和原明细过期被重算清零。Crawler只固定public-site纯服务成功读取按可信本地日计，独立幂等回执。

小时/日报任务V3：aggregate_jobs仅不可变业务输入/原提交意图与真实执行镜像，不是领取队列。canonical AggregateJobBody schemaVersion1绑定kind/period/timezone/sourceWatermark/siteId/dataEpoch，真实字节/长度hash构造本域ObjectRef；固定原operation/root/dedupeKey，未知create只原key续接，禁止修改旧输入。每分钟有界扫描闭合周期（小时5分钟、日10分钟）与迟到真实水位，jobs唯一调度。worker用平台真实续租/取消钩子，事务前锁外CheckLease，提交前核本域同job/worker/fence/epoch/attempt/expiry镜像；恢复不得复活旧镜像执行权。总UV与各source/country维度分开distinct。超过原明细保留且缺原聚合不可重建0；历史水位不受新全局访问影响。费用任务仍缺完整协议，RunWorker整体与快照/Ready继续blocked。

问答来源投影V4：qa_turn_projection仅真实轮次/来源修订/分类/hash/时间，inbox同事务。重复和低修订不增；更高修订撤旧再新计，同修订异完整事实冲突。旧非空汇总无来源修订拒绝静默迁移。failed事件生产schema无language，内部locale空表示未提供，不造语言。内容缺口只完整可信日历日，同快照返page/qa水位、QA分子分母/最近提交时间，0样本显示无样本；访次累加非跨日独立访客，问题正文仍qa权限入口。费用事件缺完整安全统计页仍不确认，审计producer事件尚未装配；整体订阅/Ready仍blocked。

当前V2小闭环仅覆盖页拉取、事务持久和两个费用事件的本地handler接线；AggregateUsage、ReconcileUsage、usage query、完整consumer/worker循环、真实scope与ready仍未完成。

私有CSV与保留期V5：统计pages/usage和审计audits使用固定不同权限创建/下载路由；边界即时授权，同原actor/site/epoch、24h与完整hash。固定安全列≤100000行/32MiB，公式首字符=+@-、tab/CR/LF加单引号，CSV转义。preparing原操作/hash/期限→part完整文件及目录fsync→prepared真实ref/原结果→rename目录fsync→ready；prepared只续原字节不重查。启动续journal/未知旧记录对象拒绝；到期保留幂等墓碑不重授24h。原明细30天有界续清，未完整派生page-view另记真实漏数而不改变period水位；原salt新日及每分钟擦旧。审计只删逐条核链的最老连续过期前缀，默认180最少30，原锚/删除/retention审计同事务，晚到原时间旧事实可能更久保留以维持链，绝不删中间或重写原hash。费用页面仅已有投影/真实对账时间，不冒全站实时余额。

统计UI库：固定已批准Vue/Vite/TS版本，mountModuleUI只消费portal同源apiClient；同DOM/build/capabilities/端口一致复用，统一nice-wiki:unmount停止控制器/监听/下载并清实例。访问/费用并行但费用不可用明确标记，显示真实双水位/漏数/最后完整对账；不说实时余额、分桶访次非跨桶独立人。审计按action/UTC日期、固定上界签名游标与真实链校验；日期过滤进入服务SQL及游标绑定，权限失效清列表详情。导出固定权限路由、稳定operationId重试、32MiB有界流读取后交浏览器，不把未知kind默认pages，不声明用户最终保存成功。
