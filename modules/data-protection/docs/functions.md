# data-protection 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 66 个函数：约束 38 个，参考 28 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：site级备份/恢复互斥；不阻止正常读直到maintenance建立
- 幂等：operationId/inputHash幂等
- 依赖：`data-protection.ValidateBackupRequest`、`jobs:createJob`

## data-protection.CollectSnapshots

- 级别：约束
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
- 授权：permission：service；rule：service备份worker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：所有snapshot同barrier/sourceEpoch；data-protection自身只快照操作截点之前catalog不递归备份对象
- 幂等：backupId/owner幂等
- 依赖：`data-protection.ExportDomainSnapshot`

## data-protection.FinalizeBackup

- 级别：约束
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
- 授权：permission：service；rule：service备份worker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：文件与SQLite无单事务；ready只在恢复日志验证完成后
- 幂等：backupId/hash幂等
- 依赖：`data-protection.BuildManifest`、`data-protection.ValidateCrossDomainRefs`、`data-protection.EncryptBackupObject`

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
- 授权：permission：service；rule：service+superadmin委托，backup有备份委托；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：service编排；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
2. 恢复必须全域activated/ready
3. runtime CAS到normal然后通知各域release

**错误**

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：runtime/barrier释放
- 授权：permission：service；rule：service+超管委托；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：service+超管委托；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：读取64KiB块，禁止加载整个archive
- 幂等：同bytes同hash
- 依赖：无
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

1. 通过各域validateSnapshot结果校验content→media refs/derived published refs/gateway paid result refs
2. 所有引用在同manifest存在或明确外部非secret指纹

**错误**

- `INTEGRITY_ERROR` / HTTP 422：DANGLING_REFERENCE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：MIXED_EPOCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：隔离校验报告
- 授权：permission：service；rule：service备份/恢复worker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只读离线snapshot引用索引，不查旧live世界补全缺失
- 幂等：manifestHash幂等
- 依赖：无
- 其他依赖说明：owner.validateSnapshot

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
- 授权：permission：service；rule：service backup加密角色；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：service恢复角色+superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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

1. check backup.manage
2. limit20 max100
3. 按createdAt/id游标
4. 显示状态/完整性摘要

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_CURSOR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只读catalog
- 幂等：GET幂等
- 依赖：`identity:checkAuthorization`

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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只读已持久catalog，不宣称failed可恢复
- 幂等：GET幂等
- 依赖：`identity:checkAuthorization`

## data-protection.ReadMirrorStatus

- 级别：参考
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

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：MIRROR_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只读本域catalog，不能声称远端最新
- 幂等：GET幂等
- 依赖：`identity:checkAuthorization`

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
- 授权：permission：superadmin；rule：backup.manage；含secret包需superadmin原授权上下文；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
2. ready备份完整域清单/epoch/schema/hash/keys
3. 所有域validateSnapshot及crossrefs
4. 不切runtime

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写校验报告与隔离stage
- 授权：permission：superadmin；rule：superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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

1. validate全通过
2. targetEpoch生成新opaque ID不能等source/current
3. 持久requested
4. jobs任务
5. 维护前保留校验hash

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：INVALID_TARGET_EPOCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写restore/outbox
- 授权：permission：superadmin；rule：superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：site级备份/恢复互斥
- 幂等：operationId/backupHash幂等
- 依赖：`data-protection.ValidateRestore`、`jobs:createJob`

## data-protection.PrepareAllDomains

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：准备所有域原子可见前置

```go
func PrepareAllDomains(ctx context.Context, op OperationContext, in PrepareAllDomainsInput) (PrepareAllDomainsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PrepareAllDomainsInput` | 准备所有域原子可见前置；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PrepareAllDomainsResult` | 准备所有域原子可见前置成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 遍历必需域prepare
2. 存所有stage IDs和snapshot hashes
3. 全ready再prepared-all
4. 失败rollback已准备并保持维护至确认

**错误**

- `INTERNAL` / HTTP 500：DOMAIN_PREPARE_FAILED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写stage/restore state
- 授权：permission：service；rule：service恢复worker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：不能允许部分域跳过，sourceEpoch相同
- 幂等：restoreId逐域续接幂等
- 依赖：`data-protection.PrepareDomain`、`data-protection.RollbackDomain`

## data-protection.ActivateAllDomains

- 级别：约束
- 文件：`modules/data-protection/internal/restore.go`
- 作用：完成全域激活

```go
func ActivateAllDomains(ctx context.Context, op OperationContext, in ActivateAllDomainsInput) (ActivateAllDomainsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ActivateAllDomainsInput` | 完成全域激活；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ActivateAllDomainsResult` | 完成全域激活成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 先identity/site-config关键域再其余域按依赖
2. 每域ack targetEpoch
3. 全部readiness检查后activated-all，才可退出维护

**错误**

- `INTERNAL` / HTTP 500：DOMAIN_ACTIVATION_FAILED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写restore/stage状态
- 授权：permission：service；rule：service恢复worker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：失败无限受控重试同stage，不重算业务或新建付费任务
- 幂等：restoreId/decisionId逐域续接
- 依赖：`data-protection.ActivateDomain`、`data-protection.ReleaseMaintenance`

## data-protection.PrepareDomain

- 级别：参考
- 文件：`modules/data-protection/internal/import_client.go`
- 作用：准备单域导入

```go
func PrepareDomain(ctx context.Context, op OperationContext, in PrepareDomainInput) (PrepareDomainResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PrepareDomainInput` | 准备单域导入；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PrepareDomainResult` | 准备单域导入成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 全域safe后传snapshot/targetEpoch/barrier
2. 校验result.owner透过stage登记
3. ready必须true

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：领域隔离stage+本域stage记录
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：最多3并发；任何域失败不切代
- 幂等：restoreId/owner/hash幂等
- 依赖：无
- 其他依赖说明：owner.prepareSnapshot

## data-protection.ActivateDomain

- 级别：参考
- 文件：`modules/data-protection/internal/import_client.go`
- 作用：激活单域stage

```go
func ActivateDomain(ctx context.Context, op OperationContext, in ActivateDomainInput) (ActivateDomainResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ActivateDomainInput` | 激活单域stage；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ActivateDomainResult` | 激活单域stage成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 传已登记stage/barrier/targetEpoch
2. 各域读取签名commit决定
3. 成功记录committed，失联读域stage状态确认

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：域存储指针切换+stage状态
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：任一失效保持全站维护；commit后不能rollback
- 幂等：stageId/targetEpoch幂等
- 依赖：`data-protection.GetRestoreDecision`
- 其他依赖说明：owner.commitSnapshot

## data-protection.RollbackDomain

- 级别：参考
- 文件：`modules/data-protection/internal/import_client.go`
- 作用：撤销单域隔离stage

```go
func RollbackDomain(ctx context.Context, op OperationContext, in RollbackDomainInput) (RollbackDomainResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RollbackDomainInput` | 撤销单域隔离stage；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RollbackDomainResult` | 撤销单域隔离stage成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 确定decision非commit
2. 调用rollbackSnapshot
3. 确认活跃指针未变
4. 记录rolled-back

**错误**

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删除领域隔离stage
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：任何commit证据即拒rollback
- 幂等：stageId幂等
- 依赖：无
- 其他依赖说明：owner.rollbackSnapshot

## data-protection.WriteRestoreDecision

- 级别：约束
- 文件：`modules/data-protection/internal/decision_log.go`
- 作用：持久全局提交决定

```go
func WriteRestoreDecision(ctx context.Context, op OperationContext, in WriteRestoreDecisionInput) (WriteRestoreDecisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `WriteRestoreDecisionInput` | 持久全局提交决定；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteRestoreDecisionResult` | 持久全局提交决定成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 确认全域prepared
2. 写append-only commit决定 fsync
3. 签名decisionId/restore/target/hash
4. 之后禁止任何rollback

**错误**

- `LOCKED` / HTTP 409：PREPARE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：append决策日志+catalog状态
- 授权：permission：service；rule：service恢复worker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：restoreId唯一决策，journal先于runtimeepoch CAS；崩溃从decision继续
- 幂等：同restore/hash返回同commit；不同hash拒409
- 依赖：`data-protection.PrepareAllDomains`
- 其他依赖说明：外部签名密钥

## data-protection.GetRestoreDecision

- 级别：约束
- 文件：`modules/data-protection/internal/decision_log.go`
- 作用：读持久签名决策

```go
func GetRestoreDecision(ctx context.Context, op OperationContext, in GetRestoreDecisionInput) (GetRestoreDecisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `GetRestoreDecisionInput` | 读持久签名决策；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetRestoreDecisionResult` | 读持久签名决策成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验service属于必需域
2. 读journal/catalog一致记录
3. 无决定返回not-found，不推测commit

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：DECISION_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service域身份；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只读fsynced决策；runtime未切代仍返回commit供续执行
- 幂等：GET幂等
- 依赖：无
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

1. 验持久commit决定
2. site-config CAS保留maintenance activating但epoch变target
3. 失联先读runtime确认结果

**错误**

- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：RUNTIME_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：site-config运行epoch写
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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

1. superadmin重验
2. 读decision/runtime/stages
3. commit前继续完整prepare或回滚
4. commit后只补epochCAS/activate/ready

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：RESTORE_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：状态/激活调用
- 授权：permission：superadmin；rule：superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：restoreId独占恢复锁；不能创建第二决策
- 幂等：operationId幂等且同restore阶段续接
- 依赖：`data-protection.GetRestoreDecision`、`data-protection.ActivateAllDomains`

## data-protection.AbortRestore

- 级别：约束
- 文件：`modules/data-protection/internal/restore_recovery.go`
- 作用：commit前撤销全域恢复

```go
func AbortRestore(ctx context.Context, op OperationContext, in AbortRestoreInput) (AbortRestoreResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AbortRestoreInput` | commit前撤销全域恢复；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AbortRestoreResult` | commit前撤销全域恢复成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. superadmin
2. journal查未commit
3. 写abort决定
4. 所有stage rollback确认
5. 当前epoch仍旧且安全后退出maintenance

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：决策日志/stage清理/runtime释放
- 授权：permission：superadmin；rule：superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：abort与commit同restore锁互斥
- 幂等：重复abort续清理，不重复决策
- 依赖：`data-protection.RollbackDomain`、`data-protection.ReleaseMaintenance`

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
3. commit决定尚未全域完成强制maintenance并继续，不默许正常启动

**错误**

- `INTEGRITY_ERROR` / HTTP 422：JOURNAL_CORRUPT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：修复自身catalog/清temp/恢复维护
- 授权：permission：service；rule：service启动；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：启动readiness必须在恢复审计完成后；源历史不修改
- 幂等：journalOpId去重
- 依赖：`data-protection.GetRestoreDecision`、`site-config:readRuntimeState`

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
- 授权：permission：service；rule：service mirrorworker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage委托；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：service mirrorworker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage明确sync操作；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage或固定service策略；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：30s租约，至少一次
- 幂等：eventId固定
- 依赖：`jobs:publishEvent`

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
- 授权：permission：service；rule：service jobs事件；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：sourceRevision取max
- 幂等：eventId/revision幂等
- 依赖：`data-protection.WaitAllSafe`

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
2. 每条事件校验信封、生产方payloadSchema与dataEpoch：不在登记清单的事件隔离报警并确认；dataEpoch不等于当前active epoch时，若本域尚未激活该epoch则不确认等待重投，否则视为旧代事件记skipped并确认
3. 按events.json该事件的handler字段调用本域处理函数；处理函数在同一本地事务内完成inbox(eventId+payloadHash)去重、subject revision单调检查与业务写入
4. 处理函数成功提交（含重复或低revision的无操作）后才调用jobs:acknowledgeEvent；失败不确认，等jobs确认租约到期重投，连续失败由jobs转dead-letter供管理重放
5. 屏障draining时停止领取新事件，等在途处理完成后报告安全点；维护期间不确认未处理事件

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs订阅或确认不可用；重连后续传（可重试）
- `VALIDATION`：事件信封或载荷不符合登记schema；隔离并确认（不可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash；隔离报警（不可重试）

- 副作用：HTTP长连接jobs:subscribeEvents；调用本域登记的事件处理函数；HTTP调用jobs:acknowledgeEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：每进程一个订阅循环；同subject串行，不同subject最多maxInFlight并发；事务与CAS由处理函数负责
- 幂等：jobs至少一次投递；本域inbox按eventId+payloadHash去重，重复投递只重新确认
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`data-protection.ConsumeBarrierEvent`

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
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
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
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
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
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：备份不无限嵌套；本轮操作journal保留控制面外部恢复锚
- 幂等：backupId/barrierId幂等
- 依赖：无
- 其他依赖说明：本域catalog

## data-protection.ValidateImport

- 级别：约束
- 文件：`modules/data-protection/internal/import.go`
- 作用：校验保护域快照

```go
func ValidateImport(ctx context.Context, op OperationContext, request SnapshotValidateRequest) (SnapshotValidation, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotValidateRequest` | 校验保护域快照；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotValidation` | 校验保护域快照成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. hash/schema/无secret/remote白名单
2. 历史decision标historical不可指挥当前恢复
3. 当前restore控制journal不得被输入覆盖

**错误**

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：隔离验证报告
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：校验restore控制面与数据面分区
- 幂等：snapshotHash/targetEpoch幂等
- 依赖：`data-protection.ValidateRemote`

## data-protection.PrepareImport

- 级别：约束
- 文件：`modules/data-protection/internal/import.go`
- 作用：准备保护域数据导入

```go
func PrepareImport(ctx context.Context, op OperationContext, request SnapshotPrepareRequest) (SnapshotStageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotPrepareRequest` | 准备保护域数据导入；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotStageResult` | 准备保护域数据导入成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 建隔离历史catalog
2. 保留当前restore/decision/keys外部控制区
3. 历史remote默认disabled待管理员确认

**错误**

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写隔离DB
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：当前controller不被自己恢复抹除
- 幂等：restoreId/hash幂等
- 依赖：`data-protection.ValidateImport`

## data-protection.ActivateImport

- 级别：约束
- 文件：`modules/data-protection/internal/import.go`
- 作用：激活本域新catalog

```go
func ActivateImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotCommitRequest` | 激活本域新catalog；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotApplyResult` | 激活本域新catalog成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验证当前commitjournal
2. 切历史catalog指针但保留控制journal
3. 报告targetEpoch ready，不先退出全域维护

**错误**

- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：切历史catalog指针
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：控制日志独立目录/连接，不包含在替换目标
- 幂等：stageId/epoch幂等
- 依赖：`data-protection.GetRestoreDecision`

## data-protection.AbortImport

- 级别：约束
- 文件：`modules/data-protection/internal/import.go`
- 作用：撤销保护域未commit导入

```go
func AbortImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotCommitRequest` | 撤销保护域未commit导入；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotApplyResult` | 撤销保护域未commit导入成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 查控制journal未commit
2. 删历史隔离catalog
3. 当前run/decision仍存活

**错误**

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删隔离DB
- 授权：permission：service；rule：service+superadmin委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：当前controller锁优先
- 幂等：重复rolled-back成功
- 依赖：`data-protection.GetRestoreDecision`

## data-protection.ValidateRecoveryAuthorization

- 级别：参考
- 文件：`modules/data-protection/internal/recovery_authorization.go`
- 作用：本地验证本次恢复专用控制许可

```go
func ValidateRecoveryAuthorization(ctx context.Context, op OperationContext, in ValidateRecoveryAuthorizationInput) (ValidateRecoveryAuthorizationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateRecoveryAuthorizationInput` | 本地验证本次恢复专用控制许可；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateRecoveryAuthorizationResult` | 本地验证本次恢复专用控制许可成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 先校验不可被业务导入覆盖的信任锚签名/算法/keyId
2. 核audience本域/expiry/restoreId/barrierId/旧新epoch
3. 读取独立控制记录确认已批准同restore且action只在prepare/commit/rollback/barrier
4. commit核持久commit决定，rollback拒任何commit证据
5. 拒普通业务使用

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service+superadmin预先批准委托；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：校验期间不查询已换代identity业务库；控制记录与信任锚独立目录不被快照覆盖
- 幂等：同许可同控制阶段结果；提交决策变化会使rollback失效
- 依赖：`data-protection.GetRestoreDecision`
- 其他依赖说明：不可变控制信任锚

## data-protection.IssueRestoreControlPermit

- 级别：参考
- 文件：`modules/data-protection/internal/recovery_control.go`
- 作用：维护前取得已批准恢复许可

```go
func IssueRestoreControlPermit(ctx context.Context, op OperationContext, in IssueRestoreControlPermitInput) (IssueRestoreControlPermitResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `IssueRestoreControlPermitInput` | 维护前取得已批准恢复许可；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `IssueRestoreControlPermitResult` | 维护前取得已批准恢复许可成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 普通runtime下superadmin重新认证
2. identity签发每域独立audience/restore/两epoch/30分钟许可
3. 写control token加密存储，向各域prepare-control登记
4. 进入维护后仅同restore续期且需外部信任锚验证批准记录

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写独立restore控制区不入导入快照
- 授权：permission：superadmin；rule：superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：许可签发在identity换代之前；token不可日志/git；全部audience登记完成后才能维护
- 幂等：restoreId/audience/source-target幂等
- 依赖：`identity:issueRecoveryAuthorization`

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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：superadmin；rule：superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：backup.manage；rule：backup.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只一个enabled本地backupSchedule，CAS修改
- 幂等：scheduleKey site/local-backup幂等
- 依赖：`jobs:createSchedule`、`data-protection.CreateBackup`、`data-protection.PruneBackups`

## data-protection.MirrorSourceRevision

- 级别：参考
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
2. 读取其源历史不可变portable正文/对应译文术语设置refs
3. author=发布账号timestamp=发布时间
4. 同revision构建版本commit，mirror失败只告警不回滚源发布

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：本地go-git commit，不自动push
- 授权：permission：service；rule：service content事件；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：按sourceRevision排序，缺前序先拉源历史不能重排删历史
- 幂等：publicationId/hash幂等
- 依赖：`data-protection.CommitMirrorTree`
- 其他依赖说明：content源历史portable API

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
3. 签仅本restore控制permit
4. 走同validate/prepare/commit全域协议
5. 没有AI渠道也完成，不要求真实模型探测

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：完整恢复控制与数据stage
- 授权：permission：superadmin；rule：superadmin或受控首次setup授权；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：superadmin；rule：backup.manage含secret要求已授权superadmin；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：superadmin；rule：superadmin或首次setup受控授权；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 授权：permission：superadmin；rule：superadmin或首次setup受控授权；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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
- 依赖：`data-protection.BuildApplication`、`data-protection.Serve`、`data-protection.Shutdown`

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
2. 打开本域DB并运行已有schema迁移/文件journal恢复；保留restore控制记录/barrier/trust-anchor
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；已commit恢复未全完成时仅开放控制管理；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖/控制信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：无
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
3. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：`data-protection.BuildApplication`、`data-protection.Shutdown`
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

1. 确认配置/本域schema/journal/control信任锚完整并在lifecycle ready或维护控制ready
2. 已初始化时核activeDataEpoch等于runtime目标代次；关键运行权威不可达或旧epoch返回503；site-config本域读取自身权威
3. 维护中本域新epoch已激活可返回200 control-ready但publicRequestsAllowed=false，使全域激活检查不会和维护门禁互相等待；所有域完成后runtime解除维护才开放
4. 未初始化可仅带初始化能力ready，activeDataEpoch=null；无AI渠道不使基础内容能力失败，能力缺项显式呈现

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：无
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

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
