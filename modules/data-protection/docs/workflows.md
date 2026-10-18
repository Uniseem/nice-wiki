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

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `jobs.barrier-state-changed.v1` | 1 | `data-protection.ConsumeBarrierEvent` | 继续等待安全点，事件不是safe的唯一证据 |
| `content.publication.changed` | 1 | `data-protection.MirrorSourceRevision` | 每次发布写一次data git版本镜像 |

### 异步任务

#### `data-protection.backup`

- 执行函数：`data-protection.ExecuteBackup`
- 触发：CreateBackup与ScheduleLocalBackup登记的每日定时

1. 读取备份记录与发起方授权（手动=backup.manage，secret另需superadmin，定时=超管保存的定时设置）；AcquireMaintenance进入备份维护并让全部域屏障safe
2. CollectSnapshots逐域导出；FinalizeBackup校验、加密并置ready
3. ReleaseMaintenance释放屏障退出维护；成功后PruneBackups按保留策略清理
4. 任一域失败整体failed并释放维护，不产生部分可恢复包

#### `data-protection.restore`

- 执行函数：`data-protection.ExecuteRestore`
- 触发：CreateRestore与RestoreDuringSetup创建

1. 读取恢复记录与批准人；StageAllDomains让全部有状态域暂存归档
2. 全部暂存后SwitchAllDomains写切换决定并等待各域重启换入与回执
3. 本服务在切换中重启时按control日志续接，不重复已完成步骤

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `data-protection.RecoverProtectionJournal` | 启动时（入口开放前） | 恢复自身文件和决策操作 |
| `data-protection.PublishProtectionEvents` | 启动后持续，空闲每1秒 | 可靠发送操作状态 |
| `data-protection.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `data-protection.RunWorker` | 启动后持续 | 主动领取并执行本域登记的持久任务 |
<!-- flows:end -->

轻量manifest先按14域同一backup/barrier/epoch固定元数据并核源manifest指纹，再以真实对象/发布完整ref验证引用闭包；unknown/缺域拒绝，悬挂引用报告valid=false，所有封存/恢复入口必须拒绝。2MiB全局索引超限明确预算错误，不删引用凑合。安全ReadBackup只投影该版本1清单，不返回私有cipher header或wrappedDEK。

普通手动备份核backup.manage，secret另需superadmin，原批准事实固定保存在control。后台核当前epoch/jobs fence，不复用过期portal proof。只读恢复进度token不升级写权限；故障取消/回滚等待公共独立窄控制许可及append-only动作记录，不能自报actor。

远程配置实施：可信AllowedGitHosts精确匹配ASCII主机，仅HTTPS443/SSH22及refs/heads完整安全分支；HTTP credentialRef仅选部署GitCredentialRefs的ID，空表示无凭据，不读请求路径。配置按expectedRevision CAS，原操作摘要/结果同事务幂等；修改配置不触发网络或同步。

本域Vue库入口mountModuleUI使用portal注入ModuleUIBootstrap.apiClient，不构造认证头。三个既定面板支持完整备份目录/显式创建/ready流式下载、完整恢复预检与批准/只读进度、远程CAS配置与显式镜像同步；实际服务未齐的503明确失败。卸载派发nice-wiki:unmount，取消请求及轮询。只读进度token仅内存/固定header，不进URL或存储；新CreateRestoreResult兼容新增progressToken/progressExpiresAt，服务实际必写。

大型下载固定X-Backup-Download-Token头，原Response.body直接pipeTo浏览器文件WritableStream，任意失败abort，不整包Blob。当前需要File System Access支持；不支持时明确不可用。版本采用已批准Vue3.5.22/Vite7.1.7/plugin-vue6.0.1/TS6.0.0-beta/vue-tsc3.1.1/pnpm9.15.9，lock只CI生成。

control V3追加backup_jobs：canonical不可变{backupId}与其真实ObjectRef、原subject/operation/root/site/epoch/inputHash和jobs原回执。原批准与requested状态/outbox同事务持久，再以清portal proof的本域服务身份单次提交jobs；失败保留原意图供相同操作重试，不自建领取/重试队列。requested事件不含尚不存在的manifestHash；只有真实封存ready才必填并显式核验，不依赖生成器单独支持if/then。

ValidateBackupRequest只核管理员/secret额外超管、当前running同epoch、真实本机KEK及隔离目录，valid是基础条件通过，capacityVerified始终false；取得14域完整真实manifest后再按staging/密文/便携总预算与free bytes核容量，并在有界写前维持配额。
