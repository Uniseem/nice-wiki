# analytics 流程

old D31：浏览器加载sendBeacon一次→UA过滤已知爬虫→可信peer/IP+UA+每日随机盐hash→离线GeoIP国家/来源domain→30天匿名明细→每日PV/UV/热门文档/来源/国家/语言汇总长期保存；无cookie/第三方/原IP落盘。Markdown AI爬虫独立计数。

审计：领域操作提交与outbox固定eventId→analytics校验生产者/action/资源ref→允许元数据→追加哈希链→权限受控查询；领域审计源随自身快照保留。

费用：gateway账项settled事件→按ledgerEntryId/revision投影→整数micros聚合→每日固定watermark只读对账；汇总缺漏不影响实时gateway额度判定。

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

### 异步任务

#### `analytics.aggregate-hourly`

- 执行函数：`analytics.AggregateHourly`
- 触发：系统定时
- 系统定时：`5 * * * *`

1. 选择hour范围当前inbox
2. 按document/locale group count并dayHash去重
3. 事务替换该hour派生聚合及watermark

#### `analytics.aggregate-daily`

- 执行函数：`analytics.AggregateDaily`
- 触发：系统定时（站点时区）
- 系统定时：`10 0 * * *`

1. 按站点IANA时区日边界
2. 明细同dailyvisitorHash distinct得UV
3. group views/UV/热门文档语言/sourceDomain/country/语言分布
4. AI Markdown crawler计数独立不混真人
5. 日汇总长期保存

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
| `analytics.PurgeBeacons` | 每日 | 清理短期匿名行 |
| `analytics.PurgeAuditHistory` | 每日 | 审计保留期清理 |
| `analytics.PurgeExports` | 每小时 | 清理导出临时对象 |
| `analytics.PublishAggregateEvents` | 启动后持续，空闲每1秒 | 派生事件outbox派发 |
| `analytics.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `analytics.RunWorker` | 启动后持续 | 主动领取并执行本域登记的持久任务 |
<!-- flows:end -->
