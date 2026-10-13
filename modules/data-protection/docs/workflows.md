# data-protection 流程

一致备份：备份请求→runtime maintenance CAS→jobs停新任务→所有有状态域安全点→统一export→hash/schema/跨域ref校验→AES-GCM私有备份→ready→全域解除屏障。失败记录具体域与errorCode，不能提供部分域成功包作为可恢复备份。

统一恢复：离线预检全通过→新targetEpoch→maintenance+全safe→全域prepare→持久fsync commit决定→runtime epoch CAS但仍maintenance→按依赖全域activate→全域readiness确认→退出maintenance。任一activate失败保持维护并可continue；commit之后不允许回滚部分域。

崩溃续接：控制面restore决策日志不进入被替换数据区；启动先检查commit决定/runtime epoch/stages。没有决定可rollback全部stage；有commit必须补完同一targetEpoch，不能向用户暴露部分新域。

data git：ready backup→每域portable allowlist→源revision/hash元数据+文本历史投影→确定性本地commit→管理员明确sync且remote配置齐备后fast-forward push；分叉保留本地并显示冲突，镜像永远不反写各域源历史。

恢复管理校验：维护前由identity为每域签发RecoveryAuthorization；本域控制记录/屏障/信任锚独立于业务导入。维护中的prepare/commit/rollback按本地签名、audience、restore/barrier、两epoch、到期和决策状态校验，不回查已换代身份业务DB；普通业务不接受此许可；全域完成后撤销。

状态迁移：

backup requested→barrier-draining→exporting→validating→ready/failed；ready之前禁止下载宣称可恢复

restore requested→verified→barrier-draining→prepared-all→commit-decided→activating→activated-all→completed；commit前可aborted，commit后失败保持maintenance并续激活

mirror idle→building→committed→syncing→synced/conflict/failed；镜像失败不回滚已创建备份

manifest决定所有域同一个sourceEpoch与backupId，portal无业务数据只参与maintenance

并发和幂等：每个修改函数明确CAS、fencing/租约或事务边界，见functions；公共事件至少一次，不能依赖全局顺序。接收eventId相同但hash不同拒CONFLICT；较低sourceRevision记已见但不应用；旧dataEpoch拒绝并不能重绑新epoch。
