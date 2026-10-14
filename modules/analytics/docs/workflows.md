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
