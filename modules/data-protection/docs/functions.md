# data-protection 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 80 个函数：约束 42 个，参考 38 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## data-protection.ValidateBackupRequest

- 级别：参考
- 文件：`modules/data-protection/internal/backup_validation.go`
- 作用：备份入口校验

```go
func ValidateBackupRequest(ctx context.Context, op OperationContext, in ValidateBackupRequestInput) (ValidateBackupRequestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateBackupRequestInput` | 备份入口校验；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateBackupRequestResult` | 备份入口校验成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. check backup.manage
2. includeSecrets=false默认，true需superadmin明确选择
3. 读取runtime正常epoch且无restore
4. 容量预检
5. 外部加密key可用，secret仅拥有者受控加密export

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：SECRETS_NOT_EXPORTABLE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：LOCKED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：backup.manage；rule：backup.manage
- 并发：只读runtime与容量
- 幂等：同请求同结果
- 依赖：`identity:checkAuthorization`、`site-config:readRuntimeState`

## data-protection.CreateBackup

- 级别：约束
- 文件：`modules/data-protection/internal/backup.go`
- 作用：建立备份操作

```go
func CreateBackup(ctx context.Context, op OperationContext, in CreateBackupInput) (CreateBackupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CreateBackupInput` | 建立备份操作；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateBackupResult` | 建立备份操作成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 校验
2. 事务建requested run/outbox
3. 提交jobs task只含backupId
4. job失败保留run可继续

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：BACKUP_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写run/outbox
- 授权：permission：backup.manage；rule：backup.manage
- 并发：site级备份/恢复互斥；不阻止正常读直到maintenance建立
- 幂等：operationId/inputHash幂等
- 依赖：`data-protection.ValidateBackupRequest`、`jobs:createJob`

## data-protection.CollectSnapshots

- 级别：参考
- 文件：`modules/data-protection/internal/backup.go`
- 作用：收集全域快照

```go
func CollectSnapshots(ctx context.Context, op OperationContext, in CollectSnapshotsInput) (CollectSnapshotsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CollectSnapshotsInput` | 收集全域快照；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CollectSnapshotsResult` | 收集全域快照成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 固定全有状态域清单
2. 统一exportSnapshot携用户includeSecrets选择
3. secret只有显式选择且superadmin委托才由拥有者封装
4. 每域同barrier/sourceEpoch
5. 自身catalog安全截点不递归当前archive
6. 缺任何域整体failed

**错误**

- `VALIDATION` / HTTP 400：MISSING_DOMAIN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写backup_objects
- 授权：permission：service；rule：service备份worker
- 并发：所有snapshot同barrier/sourceEpoch；data-protection自身只快照操作截点之前catalog不递归备份对象
- 幂等：backupId/owner幂等
- 依赖：`data-protection.ExportDomainSnapshot`

## data-protection.FinalizeBackup

- 级别：参考
- 文件：`modules/data-protection/internal/backup.go`
- 作用：把完整备份置ready

```go
func FinalizeBackup(ctx context.Context, op OperationContext, in FinalizeBackupInput) (FinalizeBackupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `FinalizeBackupInput` | 把完整备份置ready；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FinalizeBackupResult` | 把完整备份置ready成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 全对象hash/schema/跨refs校验通过
2. manifest持久fsync/rename后事务ready/outbox
3. 文件journal补偿中途故障

**错误**

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：文件manifest+run/outbox
- 授权：permission：service；rule：service备份worker
- 并发：文件与SQLite无单事务；ready只在恢复日志验证完成后
- 幂等：backupId/hash幂等
- 依赖：`data-protection.BuildManifest`、`data-protection.ValidateCrossDomainRefs`、`data-protection.EncryptBackupObject`

## data-protection.ExecuteBackup

- 级别：约束
- 文件：`modules/data-protection/internal/backup.go`
- 作用：执行一次全站一致备份

```go
func ExecuteBackup(ctx context.Context, op OperationContext, input JobExecutionInput) (JobExecutionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `JobExecutionInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `JobExecutionResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 读取备份记录与发起方授权（手动=superadmin，定时=超管保存的定时设置）；AcquireMaintenance进入备份维护并让全部域屏障safe
2. CollectSnapshots逐域导出；FinalizeBackup校验、加密并置ready
3. ReleaseMaintenance释放屏障退出维护；成功后PruneBackups按保留策略清理
4. 任一域失败整体failed并释放维护，不产生部分可恢复包

**错误**

- `DEPENDENCY_UNAVAILABLE`：有状态域不可达或屏障超时（可重试）
- `INTEGRITY_ERROR`：导出校验失败（不可重试）

- 副作用：全站维护；写备份对象
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：全站同时只一个备份或恢复
- 幂等：按backupId唯一；失败可续
- 依赖：`data-protection.AcquireMaintenance`、`data-protection.CollectSnapshots`、`data-protection.FinalizeBackup`、`data-protection.ReleaseMaintenance`、`data-protection.PruneBackups`

## data-protection.AcquireMaintenance

- 级别：参考
- 文件：`modules/data-protection/internal/maintenance.go`
- 作用：打开站点维护屏障

```go
func AcquireMaintenance(ctx context.Context, op OperationContext, in AcquireMaintenanceInput) (AcquireMaintenanceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AcquireMaintenanceInput` | 打开站点维护屏障；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcquireMaintenanceResult` | 打开站点维护屏障成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. runtime CAS设置maintenance draining/操作owner
2. portal停止新入口
3. jobs协调所有域barrier
4. deadline360s

**错误**

- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：runtime维护状态/各域barrier
- 授权：permission：service；rule：service+superadmin委托，backup有备份委托
- 并发：site级独占；并发操作返回LOCKED
- 幂等：operationId固定CAS去重
- 依赖：`site-config:transitionRuntimeState`、`jobs:coordinateBarrier`

## data-protection.WaitAllSafe

- 级别：参考
- 文件：`modules/data-protection/internal/maintenance.go`
- 作用：验证所有域安全点

```go
func WaitAllSafe(ctx context.Context, op OperationContext, in WaitAllSafeInput) (WaitAllSafeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `WaitAllSafeInput` | 验证所有域安全点；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WaitAllSafeResult` | 验证所有域安全点成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 逐域读取barrier状态并核epoch
2. 等待在途事务/文件日志/模型安全点
3. 任何missing/failed不宣布safe

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅持久进度
- 授权：permission：service；rule：service编排
- 并发：HTTP并发3，SSE唤醒后仍读权威状态
- 幂等：重复查询幂等
- 依赖：`jobs:coordinateBarrier`
- 其他依赖说明：各域acquireBarrier

## data-protection.ReleaseMaintenance

- 级别：参考
- 文件：`modules/data-protection/internal/maintenance.go`
- 作用：退出完整安全维护

```go
func ReleaseMaintenance(ctx context.Context, op OperationContext, in ReleaseMaintenanceInput) (ReleaseMaintenanceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReleaseMaintenanceInput` | 退出完整安全维护；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseMaintenanceResult` | 退出完整安全维护成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 备份成功或commit前abort确认所有stage清理
2. 恢复必须全部有状态域登记switched回执并ready
3. runtime CAS到normal然后通知各域release

**错误**

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：runtime/barrier释放
- 授权：permission：service；rule：service+超管委托
- 并发：全域ready之前runtime禁止公开入口；释放失败维持maintenance
- 幂等：operationId/barrierId幂等
- 依赖：`site-config:transitionRuntimeState`
- 其他依赖说明：各域releaseBarrier

## data-protection.ExportDomainSnapshot

- 级别：参考
- 文件：`modules/data-protection/internal/snapshot_client.go`
- 作用：统一导出单域

```go
func ExportDomainSnapshot(ctx context.Context, op OperationContext, in ExportDomainSnapshotInput) (ExportDomainSnapshotResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ExportDomainSnapshotInput` | 统一导出单域；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExportDomainSnapshotResult` | 统一导出单域成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验证owner清单和safe
2. HTTP exportSnapshot
3. 核snapshot owner/epoch/schema/hash
4. 流式接收对象到隔离目录

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写本地备份临时对象
- 授权：permission：service；rule：service+超管委托
- 并发：最多3并发，5s接受长任务引用；禁止直读域私有盘
- 幂等：backupId/owner同请求复用snapshot
- 依赖：无
- 其他依赖说明：owner.exportSnapshot

## data-protection.BuildManifest

- 级别：参考
- 文件：`modules/data-protection/internal/manifest.go`
- 作用：构建统一备份manifest

```go
func BuildManifest(ctx context.Context, op OperationContext, in BuildManifestInput) (BuildManifestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildManifestInput` | 构建统一备份manifest；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildManifestResult` | 构建统一备份manifest成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按owner字典排序
2. 校验每必需域恰一次/同epoch
3. 记录schema、水位、hash、依赖ref清单和外部keyId

**错误**

- `VALIDATION` / HTTP 400：MISSING_DOMAIN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：MIXED_EPOCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数，不嵌secret/materialized token
- 幂等：同snapshot清单同hash
- 依赖：无
- 其他依赖说明：根contracts/modules.json

## data-protection.VerifyObjectHash

- 级别：参考
- 文件：`modules/data-protection/internal/integrity.go`
- 作用：校验文件完整性

```go
func VerifyObjectHash(ctx context.Context, op OperationContext, in VerifyObjectHashInput) (VerifyObjectHashResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `VerifyObjectHashInput` | 校验文件完整性；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifyObjectHashResult` | 校验文件完整性成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒路径出隔离根/符号链接
2. 流式SHA256和size比对
3. hash常量时间比较

**错误**

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PATH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：读取64KiB块，禁止加载整个archive
- 幂等：同bytes同hash
- 依赖：`data-protection.Catalog`、`data-protection.PrivateObjectPath`、`data-protection.storageError`
- 其他依赖说明：本域对象存储

## data-protection.ValidateCrossDomainRefs

- 级别：参考
- 文件：`modules/data-protection/internal/integrity.go`
- 作用：备份跨域引用一致性

```go
func ValidateCrossDomainRefs(ctx context.Context, op OperationContext, in ValidateCrossDomainRefsInput) (ValidateCrossDomainRefsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateCrossDomainRefsInput` | 备份跨域引用一致性；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateCrossDomainRefsResult` | 备份跨域引用一致性成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 解密各域归档manifest后校验content→media refs/derived published refs/gateway paid result refs
2. 所有引用在同manifest存在或明确外部非secret指纹

**错误**

- `INTEGRITY_ERROR` / HTTP 422：DANGLING_REFERENCE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：MIXED_EPOCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：隔离校验报告
- 授权：permission：service；rule：service备份/恢复worker
- 并发：只读离线snapshot引用索引，不查旧live世界补全缺失
- 幂等：manifestHash幂等
- 依赖：无

## data-protection.EncryptBackupObject

- 级别：参考
- 文件：`modules/data-protection/internal/crypto.go`
- 作用：加密备份对象

```go
func EncryptBackupObject(ctx context.Context, op OperationContext, in EncryptBackupObjectInput) (EncryptBackupObjectResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `EncryptBackupObjectInput` | 加密备份对象；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EncryptBackupObjectResult` | 加密备份对象成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 外部KEK封装随机DEK
2. 每对象随机nonce
3. AES256GCM AAD backupId/owner/hash/schema
4. 封装DEK只在私有加密header

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：ENCRYPTION_FAILED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写加密对象，不记录密钥
- 授权：permission：service；rule：service backup加密角色
- 并发：每对象nonce唯一；失败删未完成ciphertext
- 幂等：完成objectHash复用原密文，不重用nonce重加密
- 依赖：无
- 其他依赖说明：外部密钥提供器

## data-protection.DecryptForValidation

- 级别：参考
- 文件：`modules/data-protection/internal/crypto.go`
- 作用：解密到私有恢复隔离区

```go
func DecryptForValidation(ctx context.Context, op OperationContext, in DecryptForValidationInput) (DecryptForValidationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `DecryptForValidationInput` | 解密到私有恢复隔离区；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DecryptForValidationResult` | 解密到私有恢复隔离区成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 本域at-rest密文用外部KEK透明解密
2. 用户上传便携ZIP已包含选定原始对象无需源key
3. 逐对象SHA256/schema/ZIP路径检查后写私有隔离目录
4. 失败清临时明文
5. 目标重新使用自身at-rest key

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写隔离明文，不进git/日志
- 授权：permission：service；rule：service恢复角色+superadmin
- 并发：每restore独占目录，path验证
- 幂等：同restore/objectHash复用已校验stage
- 依赖：`data-protection.ValidatePortableArchive`
- 其他依赖说明：本实例外部at-rest key提供器

## data-protection.ListBackups

- 级别：约束
- 文件：`modules/data-protection/internal/queries.go`
- 作用：分页备份目录

```go
func ListBackups(ctx context.Context, op OperationContext, in ListBackupsInput) (ListBackupsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ListBackupsInput` | 分页备份目录；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListBackupsResult` | 分页备份目录成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 后台管理员种类约束；功能权限只由平台入口即时核一次。
2. limit默认20最大100；固定目录revision及createdAt/id倒序keyset分页。
3. 游标复用serviceauth签名，绑定稳定站点/subject/session及权限修订/epoch/limit，10分钟过期；目录revision变化STALE_RESULT，不混页。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_CURSOR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：签名游标过期/绑定或目录revision变化（不可重试）

- 副作用：无
- 授权：permission：backup.manage；rule：backup.manage
- 并发：只读catalog
- 幂等：GET幂等
- 依赖：`data-protection.Catalog`、`data-protection.requireAdministrator`、`data-protection.storageError`、`platform.SignCursor`、`platform.VerifyCursor`

## data-protection.ReadBackup

- 级别：约束
- 文件：`modules/data-protection/internal/queries.go`
- 作用：读取备份描述

```go
func ReadBackup(ctx context.Context, op OperationContext, in ReadBackupInput) (ReadBackupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReadBackupInput` | 读取备份描述；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadBackupResult` | 读取备份描述成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. check backup.manage
2. 只返回安全manifest/keyId指纹和报告
3. 不返回wrappedDEK/secret

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：BACKUP_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：backup.manage；rule：backup.manage
- 并发：只读已持久catalog，不宣称failed可恢复
- 幂等：GET幂等
- 依赖：`data-protection.Catalog`、`data-protection.requireAdministrator`、`data-protection.storageError`

## data-protection.ReadMirrorStatus

- 级别：约束
- 文件：`modules/data-protection/internal/queries.go`
- 作用：读取镜像同步状态

```go
func ReadMirrorStatus(ctx context.Context, op OperationContext, in ReadMirrorStatusInput) (ReadMirrorStatusResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReadMirrorStatusInput` | 读取镜像同步状态；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadMirrorStatusResult` | 读取镜像同步状态成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. checkbackup.manage
2. 安全状态和无凭证URL
3. 未知push状态标needs-reconcile
4. 未指定mirrorId读真实最新记录；没有记录NOT_FOUND，syncing只报needs-reconcile不推定远程成功。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：MIRROR_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：backup.manage；rule：backup.manage
- 并发：只读本域catalog，不能声称远端最新
- 幂等：GET幂等
- 依赖：`data-protection.Catalog`、`data-protection.requireAdministrator`、`data-protection.storageError`

## data-protection.ReadProtectionSettings

- 级别：约束
- 文件：`modules/data-protection/internal/queries.go`
- 作用：读取备份与镜像设置

```go
func ReadProtectionSettings(ctx context.Context, op OperationContext, input ReadProtectionSettingsInput) (ReadProtectionSettingsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ReadProtectionSettingsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadProtectionSettingsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. backup.manage
2. 返回定时备份设置、保留份数、是否包含密钥与远程仓库地址（不含凭据）

**错误**

- `FORBIDDEN`：权限不足（不可重试）

- 副作用：无
- 授权：permission：backup.manage；rule：后台管理员
- 并发：只读快照
- 幂等：只读
- 依赖：无

## data-protection.AuthorizeBackupDownload

- 级别：约束
- 文件：`modules/data-protection/internal/download.go`
- 作用：下载私有备份授权

```go
func AuthorizeBackupDownload(ctx context.Context, op OperationContext, in AuthorizeBackupDownloadInput) (AuthorizeBackupDownloadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AuthorizeBackupDownloadInput` | 下载私有备份授权；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AuthorizeBackupDownloadResult` | 下载私有备份授权成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 重验已有backup.manage，含secret包核原superadmin
2. 仅ready
3. 后台一键操作透明得到120s单用途stream token
4. 下载源端透明解密封存对象输出portableZIP，不要求用户密码或客户端密钥，不新增审批

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：BACKUP_NOT_READY：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写token hash
- 授权：permission：superadmin；rule：backup.manage；含secret包需superadmin原授权上下文
- 并发：token原文禁止日志/快照/git；消费CAS
- 幂等：operationId幂等复用未消费token
- 依赖：`identity:checkAuthorization`、`data-protection.StreamPortableBackup`

## data-protection.ValidateRestore

- 级别：约束
- 文件：`modules/data-protection/internal/restore_validation.go`
- 作用：全域恢复预检

```go
func ValidateRestore(ctx context.Context, op OperationContext, in ValidateRestoreInput) (ValidateRestoreResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateRestoreInput` | 全域恢复预检；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateRestoreResult` | 全域恢复预检成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. superadmin查权
2. ready备份完整域清单/epoch/schema/hash/keys可解密
3. 各域dataSchemaVersion不高于对应服务readyz报告的可迁移版本；跨域引用一致
4. 只写校验报告，不改runtime

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写校验报告
- 授权：permission：superadmin；rule：superadmin
- 并发：并行3，任何域不通过全局valid=false
- 幂等：backupHash/targetEpoch幂等
- 依赖：`data-protection.DecryptForValidation`、`data-protection.ValidateCrossDomainRefs`

## data-protection.CreateRestore

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：建立完整恢复

```go
func CreateRestore(ctx context.Context, op OperationContext, in CreateRestoreInput) (CreateRestoreResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CreateRestoreInput` | 建立完整恢复；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateRestoreResult` | 建立完整恢复成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. ValidateRestore全通过
2. targetEpoch生成新opaque ID，不能等于source/current
3. 持久requested，并签发绑定restoreId、24小时有效的只读进度令牌
4. 创建jobs任务依次执行StageAllDomains与SwitchAllDomains

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：INVALID_TARGET_EPOCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写restore记录/进度令牌/outbox
- 授权：permission：superadmin；rule：superadmin
- 并发：site级备份/恢复互斥
- 幂等：operationId/backupHash幂等
- 依赖：`data-protection.ValidateRestore`、`jobs:createJob`

## data-protection.StageAllDomains

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：让全部有状态域暂存备份归档

```go
func StageAllDomains(ctx context.Context, op OperationContext, in StageAllDomainsInput) (StageAllDomainsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `StageAllDomainsInput` | 全部域暂存；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StageAllDomainsResult` | 全部域暂存成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. site-config:transitionRuntimeState begin_restore进入维护
2. jobs:coordinateBarrier让全部有状态域进入restore屏障并safe
3. 并行≤3调用StageDomain，记录每域暂存回执与manifest hash
4. 任一失败：对已暂存域DiscardDomain(scope=staged)，释放屏障，abort_restore退出维护；active数据不变

**错误**

- `INTERNAL` / HTTP 500：DOMAIN_PREPARE_FAILED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：各域暂存目录与本域restore状态
- 授权：permission：service；rule：service恢复worker
- 并发：不能允许部分域跳过，sourceEpoch相同
- 幂等：restoreId逐域续接幂等
- 依赖：`site-config:transitionRuntimeState`、`jobs:coordinateBarrier`、`data-protection.StageDomain`、`data-protection.DiscardDomain`

## data-protection.SwitchAllDomains

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：让全部有状态域重启换入暂存数据

```go
func SwitchAllDomains(ctx context.Context, op OperationContext, in SwitchAllDomainsInput) (SwitchAllDomainsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `SwitchAllDomainsInput` | 全部域重启换入；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SwitchAllDomainsResult` | 全部域重启换入成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. WriteRestoreDecision后经TransitionRuntimeEpoch执行begin_switch，记录restoreId/targetEpoch
2. 各服务轮询到restore_switching后安全退出并由进程守护重启，启动时换入staged并向site-config登记switched回执；本服务也在此时重启并按control日志续接
3. 等待≤10分钟：全部有状态域switched且readyz就绪后complete_restore解除维护，发布restore-state事件
4. 超时或任一域启动失败则mark_blocked保持维护，由管理员continue或回滚

**错误**

- `INTERNAL` / HTTP 500：DOMAIN_ACTIVATION_FAILED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：runtime切换与等待各域回执
- 授权：permission：service；rule：service恢复worker
- 并发：失败无限受控重试同stage，不重算业务或新建付费任务
- 幂等：restoreId/decisionId逐域续接
- 依赖：`data-protection.WriteRestoreDecision`、`data-protection.TransitionRuntimeEpoch`、`site-config:readRuntimeState`

## data-protection.ServeRestoreArchive

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：向正在暂存的域流式提供其自己的解密归档

```go
func ServeRestoreArchive(ctx context.Context, op OperationContext, in ServeRestoreArchiveInput) (ServeRestoreArchiveResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 请求暂存的域以服务身份调用 | callerService必须等于owner |
| `in` | `ServeRestoreArchiveInput` | restoreId与owner | 严格schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ServeRestoreArchiveResult` | 流式输出的归档描述 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含路径、密钥或正文 |

**步骤**

1. 只允许callerService等于owner，runtime为restore_staging且restoreId一致
2. 从备份卷解密该域归档并流式输出，附SHA-256与字节数
3. 不输出其他域归档；记录读取审计

**错误**

- `FORBIDDEN`：调用方不是该归档所属域（不可重试）
- `LOCKED`：不在暂存阶段（不可重试）
- `NOT_FOUND`：恢复或归档不存在（不可重试）
- `INTEGRITY_ERROR`：解密或hash校验失败（不可重试）

- 副作用：读取备份卷并解密流式输出；写读取审计
- 授权：permission：service；rule：仅归档所属域的服务身份
- 并发：每域同时一个读取流
- 幂等：只读，可重复
- 依赖：`data-protection.DecryptForValidation`

## data-protection.CleanupRestore

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：恢复完成或整体换回后，释放各域保留的旧数据或暂存目录

```go
func CleanupRestore(ctx context.Context, op OperationContext, in CleanupRestoreInput) (CleanupRestoreResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | superadmin请求上下文 | 近期step-up |
| `in` | `CleanupRestoreInput` | restoreId | 严格schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CleanupRestoreResult` | 已清理的域 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含路径、密钥或正文 |

**步骤**

1. superadmin
2. 仅completed或已整体换回的恢复
3. 对全部有状态域DiscardDomain：completed删before，换回删staged
4. 记录cleaned；未清理前界面持续提示占用的磁盘空间

**错误**

- `FORBIDDEN`：非superadmin（不可重试）
- `LOCKED`：恢复尚未结束（不可重试）
- `DEPENDENCY_UNAVAILABLE`：某域不可达，可重试（可重试）

- 副作用：各域restore目录删除；写restore记录
- 授权：permission：superadmin；rule：超管确认清理
- 并发：按restoreId串行
- 幂等：重复清理返回已清理域
- 依赖：`data-protection.DiscardDomain`

## data-protection.ExecuteRestore

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：执行已批准的恢复：暂存后重启换目录

```go
func ExecuteRestore(ctx context.Context, op OperationContext, input JobExecutionInput) (JobExecutionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `JobExecutionInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `JobExecutionResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 读取恢复记录与批准人；StageAllDomains让全部有状态域暂存归档
2. 全部暂存后SwitchAllDomains写切换决定并等待各域重启换入与回执
3. 本服务在切换中重启时按control日志续接，不重复已完成步骤

**错误**

- `DEPENDENCY_UNAVAILABLE`：域不可达（可重试）
- `INTEGRITY_ERROR`：归档校验失败（不可重试）

- 副作用：全站维护与切换
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：全站同时只一个备份或恢复
- 幂等：按restoreId唯一
- 依赖：`data-protection.StageAllDomains`、`data-protection.SwitchAllDomains`

## data-protection.StageDomain

- 级别：参考
- 文件：`modules/data-protection/internal/import_client.go`
- 作用：让单个域暂存其归档

```go
func StageDomain(ctx context.Context, op OperationContext, in StageDomainInput) (StageDomainResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `StageDomainInput` | 单域暂存；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StageDomainResult` | 单域暂存成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按owner调用该域snapshots:stage，传restoreId、SnapshotRef、checksum与targetEpoch
2. 校验回执owner/checksum且staged=true
3. 失联时重放同一请求确认，不推定成功

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：域内暂存目录与本域stage记录
- 授权：permission：service；rule：service+superadmin委托
- 并发：最多3并发；任何域失败不切代
- 幂等：restoreId/owner/hash幂等
- 依赖：无
- 其他依赖说明：各有状态域stageSnapshot

## data-protection.DiscardDomain

- 级别：参考
- 文件：`modules/data-protection/internal/import_client.go`
- 作用：让单个域删除暂存或换出的旧目录

```go
func DiscardDomain(ctx context.Context, op OperationContext, in DiscardDomainInput) (DiscardDomainResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `DiscardDomainInput` | 单域丢弃暂存或旧目录；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DiscardDomainResult` | 单域丢弃暂存或旧目录成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按owner调用该域snapshots:discard（scope=staged或before）
2. 目录不存在视为已删除
3. 记录discarded

**错误**

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：域内restore目录删除
- 授权：permission：service；rule：service+superadmin委托
- 并发：任何commit证据即拒rollback
- 幂等：stageId幂等
- 依赖：无
- 其他依赖说明：各有状态域discardSnapshot

## data-protection.WriteRestoreDecision

- 级别：约束
- 文件：`modules/data-protection/internal/decision_log.go`
- 作用：持久全局切换决定

```go
func WriteRestoreDecision(ctx context.Context, op OperationContext, in WriteRestoreDecisionInput) (WriteRestoreDecisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `WriteRestoreDecisionInput` | 持久全局切换决定；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteRestoreDecisionResult` | 持久全局切换决定成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 确认全部有状态域已暂存
2. control区append-only写switch决定并fsync，不随恢复替换
3. 之后只能完成切换或整体换回，不能部分回滚

**错误**

- `LOCKED` / HTTP 409：PREPARE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：append决策日志+catalog状态
- 授权：permission：service；rule：service恢复worker
- 并发：restoreId唯一决策，journal先于runtimeepoch CAS；崩溃从decision继续
- 幂等：同restore/hash返回同commit；不同hash拒409
- 依赖：`data-protection.StageAllDomains`
- 其他依赖说明：外部签名密钥

## data-protection.ReadRestoreProgress

- 级别：约束
- 文件：`modules/data-protection/internal/decision_log.go`
- 作用：读取恢复进度

```go
func ReadRestoreProgress(ctx context.Context, op OperationContext, in ReadRestoreProgressInput) (ReadRestoreProgressResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReadRestoreProgressInput` | 读取恢复进度；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadRestoreProgressResult` | 读取恢复进度成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 凭CreateRestore签发的只读进度令牌或superadmin会话访问；身份域换入后旧会话失效，令牌仍可查看进度
2. 读control区journal与site-config runtime，返回阶段、各域暂存/切换状态与失败域，不含数据内容
3. 无此restoreId或令牌不匹配返回NOT_FOUND

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：DECISION_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：必须携带匹配restoreId的只读进度令牌，或superadmin会话
- 并发：只读fsynced决策；runtime未切代仍返回commit供续执行
- 幂等：GET幂等
- 依赖：`site-config:readRuntimeState`
- 其他依赖说明：本域决策日志

## data-protection.TransitionRuntimeEpoch

- 级别：参考
- 文件：`modules/data-protection/internal/runtime_client.go`
- 作用：提交运行epoch权威

```go
func TransitionRuntimeEpoch(ctx context.Context, op OperationContext, in TransitionRuntimeEpochInput) (TransitionRuntimeEpochResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `TransitionRuntimeEpochInput` | 提交运行epoch权威；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TransitionRuntimeEpochResult` | 提交运行epoch权威成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只在WriteRestoreDecision之后或整体换回时调用
2. site-config CAS执行begin_switch/complete_restore/begin_rollback/complete_rollback
3. 失联先读runtime确认结果

**错误**

- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：RUNTIME_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：site-config运行epoch写
- 授权：permission：service；rule：service+superadmin委托
- 并发：CAS retry只重新读取revision不生成新epoch；无法退出maintenance
- 幂等：restoreId/decisionId幂等
- 依赖：`site-config:transitionRuntimeState`

## data-protection.ContinueRestore

- 级别：约束
- 文件：`modules/data-protection/internal/restore_recovery.go`
- 作用：继续中断恢复

```go
func ContinueRestore(ctx context.Context, op OperationContext, in ContinueRestoreInput) (ContinueRestoreResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ContinueRestoreInput` | 继续中断恢复；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ContinueRestoreResult` | 继续中断恢复成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. superadmin或进度令牌持有者重验身份（恢复后需用备份中的超管账号）
2. 读journal与runtime
3. restore_staging：重试未完成的StageDomain；restore_blocked：再次begin_switch等待未就绪域重启
4. 不重复已完成步骤

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：RESTORE_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：状态/暂存与切换调用
- 授权：permission：superadmin；rule：superadmin
- 并发：restoreId独占恢复锁；不能创建第二决策
- 幂等：operationId幂等且同restore阶段续接
- 依赖：`data-protection.StageAllDomains`、`data-protection.SwitchAllDomains`

## data-protection.AbortRestore

- 级别：约束
- 文件：`modules/data-protection/internal/restore_recovery.go`
- 作用：切换前撤销或切换后整体换回

```go
func AbortRestore(ctx context.Context, op OperationContext, in AbortRestoreInput) (AbortRestoreResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AbortRestoreInput` | 切换前撤销或切换后整体换回；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AbortRestoreResult` | 切换前撤销或切换后整体换回成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. superadmin
2. restore_staging：全部域DiscardDomain(scope=staged)，释放屏障，abort_restore退出维护
3. restore_switching/restore_blocked：begin_rollback，各服务重启时把before换回active并登记rolled-back，回执齐全后complete_rollback退出维护
4. 原dataEpoch保持不变

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：决策日志/stage清理/runtime释放
- 授权：permission：superadmin；rule：superadmin
- 并发：abort与commit同restore锁互斥
- 幂等：重复abort续清理，不重复决策
- 依赖：`data-protection.DiscardDomain`、`data-protection.ReleaseMaintenance`、`data-protection.TransitionRuntimeEpoch`

## data-protection.RecoverProtectionJournal

- 级别：参考
- 文件：`modules/data-protection/internal/journal.go`
- 作用：恢复自身文件和决策操作

```go
func RecoverProtectionJournal(ctx context.Context, op OperationContext, in RecoverProtectionJournalInput) (RecoverProtectionJournalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RecoverProtectionJournalInput` | 恢复自身文件和决策操作；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverProtectionJournalResult` | 恢复自身文件和决策操作成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 扫manifest temp/journal
2. ready缺文件降failed
3. 切换决定尚未全域完成时保持maintenance并继续等待各域回执，不默许正常启动

**错误**

- `INTEGRITY_ERROR` / HTTP 422：JOURNAL_CORRUPT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：修复自身catalog/清temp/恢复维护
- 授权：permission：service；rule：service启动
- 并发：启动readiness必须在恢复审计完成后；源历史不修改
- 幂等：journalOpId去重
- 依赖：`data-protection.ReadRestoreProgress`、`site-config:readRuntimeState`

## data-protection.ValidateMirrorProjection

- 级别：参考
- 文件：`modules/data-protection/internal/mirror_validation.go`
- 作用：检查git镜像白名单

```go
func ValidateMirrorProjection(ctx context.Context, op OperationContext, in ValidateMirrorProjectionInput) (ValidateMirrorProjectionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateMirrorProjectionInput` | 检查git镜像白名单；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateMirrorProjectionResult` | 检查git镜像白名单成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只允许docs/i18n/术语表/站点公开设置/redirects.json的拥有者portable投影
2. 排除DB/drafts/assets/config秘密/账号/memory私有条目/token/IP/问答正文/lease
3. 所有文件≤10MiB
4. secret字段schema禁止，疑似凭证文本经allowlist扫描拒镜像并标失败

**错误**

- `FORBIDDEN` / HTTP 403：MIRROR_FIELD_FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PATH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service mirrorworker
- 并发：纯函数且拒symlink/traversal；git仅metadata引用media大文件
- 幂等：same projection同结果
- 依赖：无
- 其他依赖说明：各域snapshot portable projection schema

## data-protection.CollectPortableFiles

- 级别：参考
- 文件：`modules/data-protection/internal/mirror.go`
- 作用：提取允许镜像投影

```go
func CollectPortableFiles(ctx context.Context, op OperationContext, in CollectPortableFilesInput) (CollectPortableFilesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CollectPortableFilesInput` | 提取允许镜像投影；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CollectPortableFilesResult` | 提取允许镜像投影成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只ready backup已校验snapshot
2. 各域portable目录白名单
3. 保留源revision与sourceEpoch
4. 无live数据库读取

**错误**

- `LOCKED` / HTTP 409：BACKUP_NOT_READY：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：MIRROR_FIELD_FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写本域镜像staging
- 授权：permission：backup.manage；rule：backup.manage委托
- 并发：确定性owner/path排序，私有history仍归owner
- 幂等：backupId/manifestHash幂等
- 依赖：`data-protection.ValidateMirrorProjection`

## data-protection.BuildMirror

- 级别：约束
- 文件：`modules/data-protection/internal/mirror.go`
- 作用：建立data git版本镜像

```go
func BuildMirror(ctx context.Context, op OperationContext, in BuildMirrorInput) (BuildMirrorResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildMirrorInput` | 建立data git版本镜像；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildMirrorResult` | 建立data git版本镜像成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. check backup.manage
2. 读取各域源历史portable投影
3. 隐私allowlist扫描
4. 每次发布revision以原发布账号/时间构建commit
5. ready backup可聚合快照树但不能替代源历史
6. 本地镜像后仅用户sync触发远程推送

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：MIRROR_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写镜像工作区和本地Git commit
- 授权：permission：backup.manage；rule：backup.manage
- 并发：单mirror锁；未来运行由本域服务创建隔离repo，不修改源码仓库
- 幂等：manifestHash/treeHash唯一，重复复用commit
- 依赖：`data-protection.CollectPortableFiles`、`data-protection.CommitMirrorTree`

## data-protection.CommitMirrorTree

- 级别：参考
- 文件：`modules/data-protection/internal/git_local.go`
- 作用：提交确定性git树

```go
func CommitMirrorTree(ctx context.Context, op OperationContext, in CommitMirrorTreeInput) (CommitMirrorTreeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CommitMirrorTreeInput` | 提交确定性git树；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitMirrorTreeResult` | 提交确定性git树成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 绝对mirrorRoot范围验证
2. go-git内存/文件Storage构造登记path树对象
3. 每源发布revision记录原发布账号和时间
4. 提交hash唯一
5. 不运行systemgit/hooks/外部filters，不丢源历史

**错误**

- `INTERNAL` / HTTP 500：GIT_FAILED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PATH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写本地git objects/ref
- 授权：permission：service；rule：service mirrorworker
- 并发：禁hooks，禁外部diff/filters；单repo锁
- 幂等：manifestHash相同树复用commit
- 依赖：无
- 其他依赖说明：go-git库未来适配器；各域源历史portable引用

## data-protection.ConfigureRemote

- 级别：约束
- 文件：`modules/data-protection/internal/git_config.go`
- 作用：设置远程同步目标

```go
func ConfigureRemote(ctx context.Context, op OperationContext, in ConfigureRemoteInput) (ConfigureRemoteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ConfigureRemoteInput` | 设置远程同步目标；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConfigureRemoteResult` | 设置远程同步目标成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. checkbackup.manage
2. 仅https/ssh白名单无URL凭证
3. branch refs/heads/<安全名>
4. credentialRef外部
5. CAS配置

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_REMOTE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：REMOTE_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写无secret remote_config
- 授权：permission：backup.manage；rule：backup.manage
- 并发：revisionCAS；首次remote未设置时sync409
- 幂等：operationId/inputHash幂等
- 依赖：`identity:checkAuthorization`

## data-protection.ValidateRemote

- 级别：参考
- 文件：`modules/data-protection/internal/git_validation.go`
- 作用：校验远程传输参数

```go
func ValidateRemote(ctx context.Context, op OperationContext, in ValidateRemoteInput) (ValidateRemoteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateRemoteInput` | 校验远程传输参数；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateRemoteResult` | 校验远程传输参数成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒file/ext/git协议、本地路径、shell参数注入
2. host管理员allowlist
3. 禁止URLuser/password/query
4. branch拒..及特殊字符

**错误**

- `VALIDATION` / HTTP 400：INVALID_REMOTE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数；执行Git参数数组而非shell拼接
- 幂等：同remote同结果
- 依赖：无
- 其他依赖说明：站点出口配置

## data-protection.SyncMirror

- 级别：约束
- 文件：`modules/data-protection/internal/git_remote.go`
- 作用：显式远程fast-forward同步

```go
func SyncMirror(ctx context.Context, op OperationContext, in SyncMirrorInput) (SyncMirrorResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `SyncMirrorInput` | 显式远程fast-forward同步；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SyncMirrorResult` | 显式远程fast-forward同步成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. check backup.manage与显式已配置remote
2. go-git fetch核expectedRemoteHead/ancestor
3. 使用外部credential provider仅fast-forward push指定branch
4. 分叉409保留本地无force
5. 60s deadline，未知结果fetch确认

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：REMOTE_DIVERGED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：REMOTE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：远程Git push在未来显式授权运行，本轮禁止
- 授权：permission：backup.manage；rule：backup.manage明确sync操作
- 并发：60s deadline；凭证进程内外部provider；git输出redact
- 幂等：mirrorcommit/expectedHead幂等；未知结果fetch核对
- 依赖：`data-protection.ValidateRemote`
- 其他依赖说明：go-git transport；外部credential provider

## data-protection.PruneBackups

- 级别：参考
- 文件：`modules/data-protection/internal/retention.go`
- 作用：安全保留清理

```go
func PruneBackups(ctx context.Context, op OperationContext, in PruneBackupsInput) (PruneBackupsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PruneBackupsInput` | 安全保留清理；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PruneBackupsResult` | 安全保留清理成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 默认保留最近N=20成功完整包
2. keepDays默认0不额外延长
3. restore引用/active mirror引用/最近成功包受保护
4. 满足N条件先markdeleting后按refcount删对象
5. 失败journal续清理

**错误**

- `VALIDATION` / HTTP 400：BACKUP_IN_USE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删加密备份/manifest
- 授权：permission：backup.manage；rule：backup.manage或固定service策略
- 并发：site维护锁；按refcount删除，失败可续清理
- 幂等：backupId删除journal幂等
- 依赖：无
- 其他依赖说明：本域catalog

## data-protection.PublishProtectionEvents

- 级别：约束
- 文件：`modules/data-protection/internal/outbox.go`
- 作用：可靠发送操作状态

```go
func PublishProtectionEvents(ctx context.Context, op OperationContext, in PublishProtectionEventsInput) (PublishProtectionEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PublishProtectionEventsInput` | 可靠发送操作状态；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishProtectionEventsResult` | 可靠发送操作状态成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. ≤100固定eventId
2. payload仅状态引用
3. jobs durable确认后markdelivered，失败重试不重做backup

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：JOBS_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：外部event/写outbox
- 授权：permission：service；rule：service
- 并发：30s租约，至少一次
- 幂等：eventId固定
- 依赖：`jobs:publishEvent`、`platform.RunRelay`

## data-protection.ConsumeBarrierEvent

- 级别：约束
- 文件：`modules/data-protection/internal/events.go`
- 作用：屏障通知消费

```go
func ConsumeBarrierEvent(ctx context.Context, op OperationContext, in ConsumeBarrierEventInput) (ConsumeBarrierEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ConsumeBarrierEventInput` | 屏障通知消费；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeBarrierEventResult` | 屏障通知消费成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验jobsproducer/type/epoch
2. inbox去重
3. 通知编排重新读各域权威barrier，不把event当全safe证明

**错误**

- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写inbox/唤醒
- 授权：permission：service；rule：service jobs事件
- 并发：sourceRevision取max
- 幂等：eventId/revision幂等
- 依赖：`data-protection.WaitAllSafe`、`platform.ClaimInbox`

## data-protection.ConsumeEvents

- 级别：约束
- 文件：`modules/data-protection/internal/events.go`
- 作用：从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认

```go
func ConsumeEvents(ctx context.Context, op OperationContext, input ConsumeEventsInput) (ConsumeEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 本域后台worker以服务身份构建的操作上下文，含当前dataEpoch | callerService为本域；dataEpoch等于site-config当前active epoch |
| `input` | `ConsumeEventsInput` | 订阅循环参数 | 必须通过公共schema；consumerId等于本域slug |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeEventsResult` | 循环退出时的累计计数 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 进程启动后以服务身份调用jobs:subscribeEvents（durable模式SSE），只订阅本域events.json中consume登记的事件名与版本；断线按Last-Event-ID续传，1/2/4秒指数退避（上限60秒）重连
2. 每条事件校验信封、生产方payloadSchema与dataEpoch：不在登记清单的事件隔离报警并确认；dataEpoch不等于当前active epoch时，若本域尚未换入该epoch则不确认等待重投，否则视为旧代事件记skipped并确认
3. 按events.json该事件的handler字段调用本域处理函数；处理函数在同一本地事务内完成inbox(eventId+payloadHash)去重、subject revision单调检查与业务写入
4. 处理函数成功提交（含重复或低revision的无操作）后才调用jobs:acknowledgeEvent；失败不确认，等jobs确认租约到期重投，连续失败由jobs转dead-letter供管理重放
5. 屏障draining时停止领取新事件，等在途处理完成后报告安全点；维护期间不确认未处理事件

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs订阅或确认不可用；重连后续传（可重试）
- `VALIDATION`：事件信封或载荷不符合登记schema；隔离并确认（不可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash；隔离报警（不可重试）

- 副作用：HTTP长连接jobs:subscribeEvents；调用本域登记的事件处理函数；HTTP调用jobs:acknowledgeEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个订阅循环；同subject串行，不同subject最多maxInFlight并发；事务与CAS由处理函数负责
- 幂等：jobs至少一次投递；本域inbox按eventId+payloadHash去重，重复投递只重新确认
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`data-protection.ConsumeBarrierEvent`、`data-protection.MirrorSourceRevision`、`platform.RunConsumer`

## data-protection.EnterBarrier

- 级别：约束
- 文件：`modules/data-protection/internal/barrier.go`
- 作用：取得本域快照安全截点

```go
func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `BarrierRequest` | 取得本域快照安全截点；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 取得本域快照安全截点成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒新backup/restore/mirror写
2. 活动backup允许本次controller达到catalog安全截点
3. 冻结无关outbox/catalog，记录不含自身进行中archive的水位

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写本域barrier
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：controller元数据journal与快照冻结区分；不递归自己对象
- 幂等：barrierId幂等
- 依赖：无
- 其他依赖说明：本域journal

## data-protection.ReleaseBarrier

- 级别：约束
- 文件：`modules/data-protection/internal/barrier.go`
- 作用：解除本域维护

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReleaseBarrierInput` | 解除本域维护；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 解除本域维护成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 确认全域runtime完成并epoch匹配
2. 恢复catalog/mirror任务
3. commit未完成保持锁

**错误**

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写barrier
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：不会解除commit-decided恢复锁
- 幂等：barrierId幂等
- 依赖：`site-config:readRuntimeState`

## data-protection.CreateSnapshot

- 级别：约束
- 文件：`modules/data-protection/internal/snapshot.go`
- 作用：导出本域操作目录快照

```go
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotExportRequest` | 导出本域操作目录快照；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 导出本域操作目录快照成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. safe截点
2. 导出历史ready catalog/remote无secret配置/已完成decisions
3. 排除当前backup/restore运行与archive实物，archive由全manifest对象收录

**错误**

- `LOCKED` / HTTP 409：BARRIER_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写本域snapshot对象
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：备份不无限嵌套；本轮操作journal保留控制面外部恢复锚
- 幂等：backupId/barrierId幂等
- 依赖：无
- 其他依赖说明：本域catalog

## data-protection.StageSnapshot

- 级别：约束
- 文件：`modules/data-protection/internal/import.go`
- 作用：把本域备份归档暂存到restore/<restoreId>/staged，不触碰active数据

```go
func StageSnapshot(ctx context.Context, op OperationContext, input SnapshotStageRequest) (SnapshotStageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | data-protection服务身份携带发起恢复的superadmin委托 | callerService=data-protection；dataEpoch等于site-config当前active epoch |
| `input` | `SnapshotStageRequest` | 暂存请求 | 必须通过公共schema；snapshot.owner等于本域 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotStageResult` | 暂存回执 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含路径、密钥或正文 |

**步骤**

1. 校验data-protection服务身份（用户授权已由data-protection核验）；site-config runtime为restore_staging且restoreId一致；本域restore屏障为safe
2. 经data-protection.ServeRestoreArchive按restoreId拉取本域归档，边读边算SHA-256，写入<dataRoot>/restore/<restoreId>/staged.tmp
3. 校验checksum、owner、manifest内文件hash，以及dataSchemaVersion不高于本版本可迁移范围；任一不符删除临时目录并返回INTEGRITY_ERROR或UNSUPPORTED_SCHEMA
4. fsync后改名为staged，在control区记录暂存回执（restoreId、snapshotId、checksum）；active目录与数据库连接保持不变

**错误**

- `FORBIDDEN`：调用方不是data-protection或缺superadmin委托（不可重试）
- `LOCKED`：runtime阶段或restoreId不符，或本域屏障未safe（不可重试）
- `INTEGRITY_ERROR`：归档hash、owner或manifest不一致；临时目录已删除（不可重试）
- `UNSUPPORTED_SCHEMA`：快照schema高于本版本可迁移范围（不可重试）
- `CONFLICT`：同restoreId已暂存不同checksum（不可重试）
- `DEPENDENCY_UNAVAILABLE`：拉取归档失败或磁盘空间不足；临时目录已删除（可重试）

- 副作用：写<dataRoot>/restore/<restoreId>/staged与control区回执；HTTP读取data-protection恢复归档
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：control区按restoreId串行；不锁active读写；同一时间只允许一个恢复
- 幂等：restoreId+checksum唯一；重复调用返回已记录回执
- 依赖：`data-protection.ServeRestoreArchive`、`site-config:readRuntimeState`

## data-protection.DiscardSnapshot

- 级别：约束
- 文件：`modules/data-protection/internal/import.go`
- 作用：删除某次恢复的暂存目录或恢复完成后换出的旧数据目录

```go
func DiscardSnapshot(ctx context.Context, op OperationContext, input SnapshotDiscardRequest) (SnapshotDiscardResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | data-protection服务身份携带发起恢复的superadmin委托 | callerService=data-protection；dataEpoch等于site-config当前active epoch |
| `input` | `SnapshotDiscardRequest` | 丢弃请求 | 必须通过公共schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotDiscardResult` | 丢弃结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含路径、密钥或正文 |

**步骤**

1. 校验data-protection服务身份（用户授权已由data-protection核验）
2. scope=staged：仅当runtime不在restore_switching/restore_rolling_back、且control日志显示本域尚未换入该restoreId时删除restore/<restoreId>/staged
3. scope=before：仅当该恢复已completed或已整体换回、且control日志确认active不是该before时删除restore/<restoreId>/before
4. 删除后在control区记录discarded；目录不存在返回removed=false

**错误**

- `FORBIDDEN`：调用方不是data-protection或缺superadmin委托（不可重试）
- `LOCKED`：当前恢复阶段不允许删除该目录（不可重试）

- 副作用：删除<dataRoot>/restore/<restoreId>下的目录；写control区回执
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：control区按restoreId串行
- 幂等：重复删除返回removed=false
- 依赖：`site-config:readRuntimeState`

## data-protection.RequestBackup

- 级别：参考
- 文件：`modules/data-protection/web/src/BackupPanel.vue`
- 作用：Vue3模块交互：RequestBackup

```ts
function RequestBackup(input: RequestBackupInput, signal: AbortSignal): Promise<RequestBackupResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `RequestBackupInput` | Vue3模块交互：RequestBackup；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RequestBackupResult` | Vue3模块交互：RequestBackup成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 展示全域水位和失败域
2. 仅ready允许申请下载，不允许半份快照被标成功
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：backup.manage；rule：backup.manage
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：data-protection OpenAPI HTTP/JSON/SSE

## data-protection.ApproveFullRestore

- 级别：参考
- 文件：`modules/data-protection/web/src/RestoreWizard.vue`
- 作用：Vue3模块交互：ApproveFullRestore

```ts
function ApproveFullRestore(input: ApproveFullRestoreInput, signal: AbortSignal): Promise<ApproveFullRestoreResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ApproveFullRestoreInput` | Vue3模块交互：ApproveFullRestore；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApproveFullRestoreResult` | Vue3模块交互：ApproveFullRestore成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 先展示完整校验结果及维护影响
2. 明确批准仅该backup/target后提交restore，不支持选部分域
3. 提交后进入进度页：恢复期间后台会话会失效，进度页凭只读令牌刷新，完成后提示用备份中的账号重新登录
4. 按operationId重试不重复提交

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：superadmin；rule：superadmin
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：data-protection OpenAPI HTTP/JSON/SSE

## data-protection.RequestMirrorSync

- 级别：参考
- 文件：`modules/data-protection/web/src/GitSyncPanel.vue`
- 作用：Vue3模块交互：RequestMirrorSync

```ts
function RequestMirrorSync(input: RequestMirrorSyncInput, signal: AbortSignal): Promise<RequestMirrorSyncResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `RequestMirrorSyncInput` | Vue3模块交互：RequestMirrorSync；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RequestMirrorSyncResult` | Vue3模块交互：RequestMirrorSync成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 展示无secret remote/branch/local commit后显式触发sync
2. 分叉提示冲突无force选项
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：backup.manage；rule：backup.manage
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：data-protection OpenAPI HTTP/JSON/SSE

## data-protection.ScheduleLocalBackup

- 级别：约束
- 文件：`modules/data-protection/internal/backup_schedule.go`
- 作用：每日本地备份

```go
func ScheduleLocalBackup(ctx context.Context, op OperationContext, in ScheduleLocalBackupInput) (ScheduleLocalBackupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ScheduleLocalBackupInput` | 每日本地备份；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ScheduleLocalBackupResult` | 每日本地备份成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. checkbackup.manage
2. jobs注册02:00站点IANA时区cron 0 2 * * *
3. includeSecrets遵守已保存管理员选择
4. 每日一个tick去重
5. 成功后保留最近N份，失败包不占成功N

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写schedule引用
- 授权：permission：backup.manage；rule：backup.manage
- 并发：只一个enabled本地backupSchedule，CAS修改
- 幂等：scheduleKey site/local-backup幂等
- 依赖：`jobs:createSchedule`、`data-protection.CreateBackup`、`data-protection.PruneBackups`

## data-protection.MirrorSourceRevision

- 级别：约束
- 文件：`modules/data-protection/internal/publication_mirror.go`
- 作用：每次发布写data git镜像

```go
func MirrorSourceRevision(ctx context.Context, op OperationContext, in MirrorSourceRevisionInput) (MirrorSourceRevisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `MirrorSourceRevisionInput` | 每次发布写data git镜像；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MirrorSourceRevisionResult` | 每次发布写data git镜像成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. content已完成发布事件
2. 经content:readContentObject读取该发布修订的不可变正文，site-config:readConfigObject读取相关设置文件；译文与术语只取引用
3. author=发布账号timestamp=发布时间
4. 同revision构建版本commit，mirror失败只告警不回滚源发布

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：本地go-git commit，不自动push
- 授权：permission：service；rule：service content事件
- 并发：按sourceRevision排序，缺前序先拉源历史不能重排删历史
- 幂等：publicationId/hash幂等
- 依赖：`data-protection.CommitMirrorTree`、`content:readContentObject`、`site-config:readConfigObject`、`platform.ClaimInbox`

## data-protection.RestoreDuringSetup

- 级别：约束
- 文件：`modules/data-protection/internal/setup_restore.go`
- 作用：初始化向导从完整备份恢复

```go
func RestoreDuringSetup(ctx context.Context, op OperationContext, in RestoreDuringSetupInput) (RestoreDuringSetupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RestoreDuringSetupInput` | 初始化向导从完整备份恢复；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RestoreDuringSetupResult` | 初始化向导从完整备份恢复成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. identity校验未初始化首次setup独占授权
2. 检查完整备份含账号/secret选择
3. 走同一CreateRestore暂存→重启换入流程，返回进度令牌
4. 没有AI渠道也完成，不要求真实模型探测

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：完整恢复流程
- 授权：permission：superadmin；rule：superadmin或受控首次setup授权
- 并发：setup锁由identity权威，不能让任意匿名restore
- 幂等：setupOperationId/backupHash幂等
- 依赖：`data-protection.ValidateRestore`、`data-protection.CreateRestore`
- 其他依赖说明：identity初始化授权

## data-protection.StreamPortableBackup

- 级别：约束
- 文件：`modules/data-protection/internal/portable_download.go`
- 作用：下载可移植完整备份

```go
func StreamPortableBackup(ctx context.Context, op OperationContext, in StreamPortableBackupInput) (StreamPortableBackupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `StreamPortableBackupInput` | 下载可移植完整备份；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StreamPortableBackupResult` | 下载可移植完整备份成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 核已有权限和单用途token
2. at-rest透明解密对象
3. 流式ZIP含manifest/checksums/schema及选定secret对象
4. 不把源KEK/瞬态restorecontrol许可装入包
5. TLS传输，输出文件恢复不依赖源key或源服务

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：下载流/消费token；不增加用户密码解密或第二审批
- 授权：permission：superadmin；rule：backup.manage含secret要求已授权superadmin
- 并发：固定manifest快照读取，无业务写；断开终止，原ready包保留
- 幂等：同token只允许一次stream，重下载新内部token
- 依赖：`data-protection.DecryptForValidation`、`data-protection.VerifyObjectHash`

## data-protection.ValidatePortableArchive

- 级别：参考
- 文件：`modules/data-protection/internal/portable_archive.go`
- 作用：校验上传便携ZIP

```go
func ValidatePortableArchive(ctx context.Context, op OperationContext, in ValidatePortableArchiveInput) (ValidatePortableArchiveResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidatePortableArchiveInput` | 校验上传便携ZIP；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePortableArchiveResult` | 校验上传便携ZIP成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 绝对路径受控
2. 拒绝ZIP绝对路径/../symlink/重复owner/zipbomb ratio>100及总展开上限20GiB
3. 流式SHA256核manifest全域对象/schema
4. 原服务加密key非必需
5. 不执行包内脚本

**错误**

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：隔离解包对象
- 授权：permission：superadmin；rule：superadmin或首次setup受控授权
- 并发：隔离目录，不改live数据
- 幂等：archiveHash固定校验结果
- 依赖：`data-protection.VerifyObjectHash`、`data-protection.ValidateCrossDomainRefs`

## data-protection.ImportPortableBackup

- 级别：约束
- 文件：`modules/data-protection/internal/portable_upload.go`
- 作用：恢复入口接收完整备份

```go
func ImportPortableBackup(ctx context.Context, op OperationContext, in ImportPortableBackupInput) (ImportPortableBackupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ImportPortableBackupInput` | 恢复入口接收完整备份；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ImportPortableBackupResult` | 恢复入口接收完整备份成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 已登录superadmin或identity首次setup独占权限
2. ValidatePortableArchive
3. 目标自有key透明at-rest封存
4. 登记imported ready供同全域restore流程
5. 用户不填源key/额外密码

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：上传隔离对象/目录登记
- 授权：permission：superadmin；rule：superadmin或首次setup受控授权
- 并发：单上传operation总限20GiB，schema全过才ready
- 幂等：archiveHash/operationId幂等
- 依赖：`data-protection.ValidatePortableArchive`、`data-protection.EncryptBackupObject`
- 其他依赖说明：identity初始化授权

## data-protection.main

- 级别：约束
- 文件：`modules/data-protection/cmd/data-protection/main.go`
- 作用：独立进程可执行入口

```go
func main()
```

**输入**

无

**输出**

无

**步骤**

1. 读取--config/--data-dir/--listen及环境覆盖，严格解析ServiceConfig；配置错误安全stderr后exit2
2. signal.NotifyContext注册SIGINT/SIGTERM；调用BuildApplication，失败exit1且只输出errorCode
3. 调用Serve；正常关闭或signal后以30秒context调用Shutdown；安全退出0，无法安全关闭exit1且保留journal

**错误**

- `VALIDATION` / HTTP 400：配置未知字段/越界目录/非法端点（不可重试）
- `INTERNAL` / HTTP 500：装配或HTTP运行失败（不可重试）

- 副作用：打开本域资源、监听HTTP；关闭按持久安全点
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：一个进程一个Application；OS信号处理只触发一次drain
- 幂等：进程重启先journal恢复；不重放已付费任务
- 依赖：`data-protection.BuildApplication`、`data-protection.Serve`、`data-protection.Shutdown`、`platform.RunService`

## data-protection.BuildApplication

- 级别：约束
- 文件：`modules/data-protection/internal/app/wire.go`
- 作用：装配本域存储、客户端、函数路由与worker

```go
func BuildApplication(ctx context.Context, config ServiceConfig) (*Application, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `config` | `ServiceConfig` | 已严格解析配置 | module固定本域；secret只ref；依赖只HTTP |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `application` | `*Application` | 只返回完整装配后的内部容器 | 失败返回nil，已打开资源逆序关闭 |
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 验证module/listen/data目录/secretRef；建立本域控制区和业务数据区；portal不虚构业务DB
2. 先读control区恢复日志与site-config runtime：phase为restore_switching且本域有该restoreId的staged目录、尚未换入时，把active改名为restore/<restoreId>/before、staged改名为active并fsync父目录后写control日志；phase为restore_rolling_back且已换入时反向换回；换目录后丢弃旧dataEpoch未转发outbox、租约与可重建缓存。随后打开active区DB运行schema迁移与文件journal恢复；control区不随恢复替换
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；维护期间只开放健康/就绪与控制管理接口、不启动worker；本次启动完成换目录或换回时调用site-config:registerActivation登记回执；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源
6. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环
7. 打开真实active目录库与control决定库，事务应用各自版本1迁移并装配已实现目录读取；其余路由保持具体不可用，完整就绪仍blocked。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`data-protection.RecoverProtectionJournal`、`data-protection.PublishProtectionEvents`、`data-protection.ConsumeEvents`、`data-protection.RunWorker`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewRequestValidator`、`data-protection.OpenCatalog`、`data-protection.InstallCatalog`、`data-protection.NewApplication`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## data-protection.Serve

- 级别：约束
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：运行独立HTTP/JSON/SSE服务

```go
func Serve(ctx context.Context, app *Application) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 仅HTTP启动net/http.Server，ReadHeaderTimeout=5s、MaxHeaderBytes=64KiB；写deadline由各路由决定，SSE不用统一短WriteTimeout
2. 启动本域worker循环，匿名Health/Ready优先可访问；普通路由执行权限/epoch/maintenance门禁，SSE响应X-Accel-Buffering:no
3. 运行中每5秒读取site-config runtime；phase进入restore_switching或restore_rolling_back且本域有该restoreId的staged或已换入目录时，停止新请求、在途操作到安全点后以退出码75退出，由进程守护重启并在BuildApplication换目录
4. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：`data-protection.BuildApplication`、`data-protection.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## data-protection.Shutdown

- 级别：约束
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：停止入口并使在途操作到可恢复安全点

```go
func Shutdown(ctx context.Context, app *Application) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. CAS lifecycle ready→draining，拒新写/claim/paid步骤并关闭SSE接受新订阅
2. 取消本域worker的新步骤；等待已有DB事务、文件journal、模型可结算阶段安全确认；不能因超时假定供应方免费或删除paid refs
3. http.Server.Shutdown等在途响应；flush本域outbox/journal、close DB与文件句柄；正常lifecycle closed
4. deadline未安全返回DEADLINE_EXCEEDED并保留完整journal/未决费用引用，启动恢复收敛；不清掉活跃控制记录

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：30秒期限仍有未安全在途步骤（可重试）
- `INTERNAL` / HTTP 500：flush/close失败（不可重试）

- 副作用：本域drain、资源关闭、durable恢复日志
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：sync.Once启动drain；其他调用等待同一次结果
- 幂等：重复Shutdown返回同一次关闭结果；已closed成功
- 依赖：无
- 其他依赖说明：本域barrier安全点函数；本域journal/outbox flush

## data-protection.Health

- 级别：约束
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：GET /healthz普通HTTP存活检查

```go
func Health(ctx context.Context, app *Application) (HealthStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `status` | `HealthStatus` | 公共liveness安全字段 | 仅module/version/status/capabilities |
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 仅读本进程lifecycle与HTTP循环是否存活；不查询远程依赖/模型/数据库
2. 存活且未closed返回200 HealthStatus；closed或server不可用返回503；不泄漏路径、端点或secret

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：进程已closed或服务循环不可用（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：无锁或原子读取lifecycle；不进行外部调用
- 幂等：GET只读重复安全
- 依赖：无

## data-protection.Ready

- 级别：约束
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：GET /readyz普通HTTP就绪检查

```go
func Ready(ctx context.Context, app *Application) (ServiceReadiness, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `readiness` | `ServiceReadiness` | 本域启动/恢复就绪与公开门禁状态 | ready不等于维护已解除，publicRequestsAllowed必须显式false |
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 确认配置/本域schema/journal/control区与服务身份信任锚完整并在lifecycle ready或维护控制ready；返回当前active数据代次与本域恢复回执状态
2. 已初始化时核activeDataEpoch等于runtime目标代次；关键运行权威不可达或旧epoch返回503；site-config本域读取自身权威
3. 维护中本域已换入目标数据可返回200 control-ready但publicRequestsAllowed=false，使全域回执检查不会和维护门禁互相等待；所有域完成后runtime解除维护才开放
4. 未初始化可仅带初始化能力ready，activeDataEpoch=null；无AI渠道不使基础内容能力失败，能力缺项显式呈现

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：无
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## data-protection.NewApplication

- 级别：参考
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：实际修改的本域HTTP装配支持；域逻辑调用对应合同函数。

```go
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtime platform.SiteConfigClient, clients Dependencies) (*Application, error)
```

**输入**

无

**输出**

无

**步骤**

1. 生成结构校验与身份/epoch/权限只经平台入口；具体HTTP方法转换查询/路径并调用真实目录函数，未实现操作仍失败关闭。

**错误**

- 无

- 副作用：无
- 授权：平台NewServiceHandler固定caller白名单及即时identity授权，内部使用可信op。
- 并发：上下文传给本域IO；不发起隐式备份或模型。
- 幂等：只读目录固定快照；不制造历史或成功状态。
- 依赖：`data-protection.server.ListBackups`、`data-protection.server.ReadBackup`、`data-protection.server.ReadMirrorStatus`

## data-protection.server.ListBackups

- 级别：参考
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：实际修改的本域HTTP装配支持；域逻辑调用对应合同函数。

```go
func (s *server) ListBackups(w http.ResponseWriter, r *http.Request, params gen.ListBackupsParams)
```

**输入**

无

**输出**

无

**步骤**

1. 生成结构校验与身份/epoch/权限只经平台入口；具体HTTP方法转换查询/路径并调用真实目录函数，未实现操作仍失败关闭。

**错误**

- 无

- 副作用：无
- 授权：平台NewServiceHandler固定caller白名单及即时identity授权，内部使用可信op。
- 并发：上下文传给本域IO；不发起隐式备份或模型。
- 幂等：只读目录固定快照；不制造历史或成功状态。
- 依赖：`data-protection.ListBackups`

## data-protection.server.ReadBackup

- 级别：参考
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：实际修改的本域HTTP装配支持；域逻辑调用对应合同函数。

```go
func (s *server) ReadBackup(w http.ResponseWriter, r *http.Request, id string, params gen.ReadBackupParams)
```

**输入**

无

**输出**

无

**步骤**

1. 生成结构校验与身份/epoch/权限只经平台入口；具体HTTP方法转换查询/路径并调用真实目录函数，未实现操作仍失败关闭。

**错误**

- 无

- 副作用：无
- 授权：平台NewServiceHandler固定caller白名单及即时identity授权，内部使用可信op。
- 并发：上下文传给本域IO；不发起隐式备份或模型。
- 幂等：只读目录固定快照；不制造历史或成功状态。
- 依赖：`data-protection.ReadBackup`

## data-protection.server.ReadMirrorStatus

- 级别：参考
- 文件：`modules/data-protection/internal/transport/server.go`
- 作用：实际修改的本域HTTP装配支持；域逻辑调用对应合同函数。

```go
func (s *server) ReadMirrorStatus(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 生成结构校验与身份/epoch/权限只经平台入口；具体HTTP方法转换查询/路径并调用真实目录函数，未实现操作仍失败关闭。

**错误**

- 无

- 副作用：无
- 授权：平台NewServiceHandler固定caller白名单及即时identity授权，内部使用可信op。
- 并发：上下文传给本域IO；不发起隐式备份或模型。
- 幂等：只读目录固定快照；不制造历史或成功状态。
- 依赖：`data-protection.ReadMirrorStatus`

## data-protection.mountModuleUI

- 级别：约束
- 文件：`modules/data-protection/web/src/main.ts`
- 作用：本域Vue3应用/岛的唯一UI构建入口

```ts
function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `rootElement` | `HTMLElement` | 已经存在的模块挂载节点 | DOM标准库；为空或不属于当前document拒绝 |
| `bootstrap` | `ModuleUIBootstrap` | 已由portal/Go输出的安全启动元数据 | module固定本域，不含service凭证/secret |

**输出**

无

**步骤**

1. 校验rootElement与bootstrap module/capabilities；按本域已登记Vue页面导入模块路由，createApp挂载，不注册别域业务组件
2. 只装配同源HTTP/JSON/SSE客户端和已有权限清理/语言逻辑，复用identity/qa会话，缺AI能力保持关闭提示
3. 同DOM重复mount先复用既有实例；unmount停止AbortController/SSE/listeners，不能清服务端业务记录

**错误**

- `VALIDATION` / HTTP 400：bootstrap不匹配本域或DOM无效（不可重试）

- 副作用：挂载本域Vue组件和浏览器事件；不新建业务记录
- 授权：permission：anonymous；rule：只挂载UI；业务mutation仍按各本域OpenAPI权限，不能以能力字段绕过权限
- 并发：同DOM一个Vue实例，AbortController取消旧请求
- 幂等：同DOM/buildRevision重复挂载复用实例
- 依赖：无
- 其他依赖说明：本域已登记UI页面函数

## data-protection.RunWorker

- 级别：约束
- 文件：`modules/data-protection/internal/worker.go`
- 作用：主动领取并执行本域登记的持久任务

```go
func RunWorker(ctx context.Context, op OperationContext, input WorkerLoopInput) (WorkerLoopResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `WorkerLoopInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WorkerLoopResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 进程启动后循环调用jobs:claimJob，只领取本域implementation.json jobTypes登记的类型；维护或屏障非released时暂停领取
2. 领取后调用jobs:startJob确认开始，按jobType调用登记的执行函数；每10秒jobs:heartbeatJob续30秒租约、上报进度并读取cancelRequested
3. 执行函数写回结果前调用jobs:checkLease核对fence仍有效；成功jobs:completeJob，失败jobs:failJob并给出可重试分类；收到取消在安全点停止并以cancelled结束
4. lease过期或fence不符立即停止写入；已派发的模型调用不重放，由网关结算或对账

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用；退避后重试领取（可重试）

- 副作用：HTTP调用jobs租约接口；执行本域登记的任务函数
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个领取循环，并发不超过maxConcurrency；同一任务只由持有有效fence者写回
- 幂等：jobs按jobId与fence去重；执行函数自身按输入引用幂等
- 依赖：`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`、`data-protection.ExecuteBackup`、`data-protection.ExecuteRestore`、`platform.RunWorkerLoop`

## data-protection.OpenCatalog

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：OpenCatalog

```go
func OpenCatalog(ctx context.Context, root, epoch string, key platform.SigningKey) (*Store, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：`data-protection.openDatabase`、`data-protection.applyMigration`、`data-protection.Store.Close`、`data-protection.storageError`

## data-protection.openDatabase

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：openDatabase

```go
func openDatabase(ctx context.Context, path string) (*sql.DB, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：`data-protection.storageError`

## data-protection.applyMigration

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：applyMigration

```go
func applyMigration(ctx context.Context, db *sql.DB, name string) error
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：`data-protection.storageError`

## data-protection.Store.Close

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：Store.Close

```go
func (s *Store) Close() error
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：`data-protection.storageError`

## data-protection.InstallCatalog

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：InstallCatalog

```go
func InstallCatalog(s *Store)
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：无

## data-protection.Catalog

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：Catalog

```go
func Catalog(op OperationContext) (*Store, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：无

## data-protection.storageError

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：storageError

```go
func storageError(err error) error
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：无

## data-protection.requireAdministrator

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：requireAdministrator

```go
func requireAdministrator(op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：无

## data-protection.PrivateObjectPath

- 级别：参考
- 文件：`modules/data-protection/internal/store.go`
- 作用：本域私有持久目录与路径支持：PrivateObjectPath

```go
func PrivateObjectPath(store *Store, path string) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只处理本域active/control隔离目录及真实SQLite事务/版本；路径采用平台逐级无symlink校验，失败不输出路径或原IO信息。

**错误**

- `INTERNAL`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VALIDATION`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `VERSION_MISMATCH`：本域持久/代次/路径条件不满足时失败关闭（不可重试）
- `UNSUPPORTED_SCHEMA`：本域持久/代次/路径条件不满足时失败关闭（不可重试）

- 副作用：仅登记职责范围内私有IO；构造不调用真实业务服务
- 授权：入口已核可信op，内部只执行本函数epoch/人工主体或本域路径约束。
- 并发：SQLite最大1连接短事务；control不随active替换；Close只执行一次。
- 幂等：版本1迁移仅user_version=0事务应用；已有版本拒绝降级。
- 依赖：无
