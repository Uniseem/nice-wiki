# data-protection 流程

一致备份：备份请求→runtime maintenance CAS→jobs停新任务→所有有状态域安全点→统一export→hash/schema/跨域ref校验→AES-GCM私有备份→ready→全域解除屏障。失败记录具体域与errorCode，不能提供部分域成功包作为可恢复备份。

统一恢复（ADR0003）：预检全通过→新targetEpoch→maintenance+全safe→全部域暂存归档（不改active）→持久切换决定→runtime begin_switch→各服务重启时换入并登记switched→全域readiness确认→退出maintenance。任一域启动失败保持维护，可continue或整体换回；不允许部分回滚。

崩溃续接：restore决策日志在control区，不随恢复替换；启动先检查切换决定/runtime/各域回执。没有决定则丢弃全部暂存；有决定则等待全部域换入或整体换回，维护期间不对外暴露部分新域。

data git：ready backup→每域portable allowlist→源revision/hash元数据+文本历史投影→确定性本地commit→管理员明确sync且remote配置齐备后fast-forward push；分叉保留本地并显示冲突，镜像永远不反写各域源历史。

恢复管理（ADR0003）：本域control区（屏障、恢复日志）不随业务恢复替换。stage/discard只接受data-protection服务身份与superadmin委托；runtime进入restore_switching或restore_rolling_back后，本服务安全退出并由进程守护重启，启动时换入或换回active目录并向site-config登记回执。

状态迁移：

backup requested→barrier-draining→exporting→validating→ready/failed；ready之前禁止下载宣称可恢复

restore requested→verified→barrier-draining→staged-all→switch-decided→switching→switched-all→completed；切换决定前可aborted（丢弃暂存），之后失败保持maintenance，可continue或整体rolled-back

mirror idle→building→committed→syncing→synced/conflict/failed；镜像失败不回滚已创建备份

manifest决定所有域同一个sourceEpoch与backupId，portal无业务数据只参与maintenance

并发和幂等：每个修改函数明确CAS、fencing/租约或事务边界，见functions；公共事件至少一次，不能依赖全局顺序。接收eventId相同但hash不同拒CONFLICT；较低sourceRevision记已见但不应用；旧dataEpoch拒绝并不能重绑新epoch。
