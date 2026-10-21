# memory 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 113 个函数：约束 43 个，参考 70 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## memory.LoadRuntime

- 级别：参考
- 文件：`modules/memory/internal/runtime/bootstrap.go`
- 作用：启动记忆服务

```go
func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `LoadRuntimeInput` | 封闭业务输入；逐字段见types $defs/LoadRuntimeInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadRuntimeResult` | 成功结果；逐字段见types $defs/LoadRuntimeResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 检查entry body/hash
2. 验证映射taskId允许集合
3. 载入固定预算
4. 恢复未确认批准命令
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：打开私有存储无自动模型调用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.All`、`memory.Control`、`memory.OpenStore`、`memory.validateEntries`

## memory.OpenStore

- 级别：参考
- 文件：`modules/memory/internal/runtime/bootstrap.go`
- 作用：本域参考级辅助 OpenStore；精确参数与返回见签名，调用边界随所属用例固定。

```go
func OpenStore(ctx context.Context, dataDirectory string, dataEpoch platform.Identifier) (*Store, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.All`、`memory.Control`

## memory.validateEntries

- 级别：参考
- 文件：`modules/memory/internal/runtime/bootstrap.go`
- 作用：本域参考级辅助 validateEntries；精确参数与返回见签名，调用边界随所属用例固定。

```go
func validateEntries(ctx context.Context,s *Store,siteID platform.Identifier)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.ReadEntryObject`

## memory.ValidateEntry

- 级别：参考
- 文件：`modules/memory/internal/entry/validate.go`
- 作用：验证记忆内容与预算

```go
func ValidateEntry(e gen.MemoryEntry)(gen.MemoryEntry,error)
```

**输入**

无

**输出**

无

**步骤**

1. NFC/LF/frontmatter合法，正文≤entry.maximumBytes默认1536
2. description/title/tag范围检查
3. 内容视作可公开，拒密钥/内部客户信息
4. readonly只约束助手，人工直接编辑仍可。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.NormalizeLineEndings`
- 其他依赖说明：参考级实际输入输出以本次精确Go签名为准；数据字段继续复用MemoryEntry/TaskMapping/TaskBudget与本域事务，不再建立独立读取事务。

## memory.ContainsPrivateMaterial

- 级别：参考
- 文件：`modules/memory/internal/entry/validate.go`
- 作用：本域参考级辅助 ContainsPrivateMaterial；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ContainsPrivateMaterial(text string)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.CreateEntry

- 级别：约束
- 文件：`modules/memory/internal/entry/write.go`
- 作用：创建人工记忆条目

```go
func CreateEntry(ctx context.Context, op OperationContext, input CreateEntryInput) (CreateEntryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateEntryInput` | 封闭业务输入；逐字段见types $defs/CreateEntryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateEntryResult` | 成功结果；逐字段见types $defs/CreateEntryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. ValidateEntry并人工授权
2. 投影所有受影响task注入量默认≤5120bytes否则VALIDATION并列taskId
3. CAS版本后写私有temp/fsync/rename权威Markdown并记录commit manifest，失败恢复旧指针
4. 数据库revision索引及outbox提交，人工编辑project overview后autoUpdateEnabled=false。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：entry/revision/outbox新增
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`identity:checkAuthorization`、`memory.ValidateEntry`、`memory.ValidateProjectedBudgets`、`memory.PersistVersion`、`memory.Write`、`memory.RequireManager`、`memory.DBError`

## memory.UpdateEntry

- 级别：约束
- 文件：`modules/memory/internal/entry/write.go`
- 作用：人工更新记忆

```go
func UpdateEntry(ctx context.Context, op OperationContext, input UpdateEntryInput) (UpdateEntryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `UpdateEntryInput` | 封闭业务输入；逐字段见types $defs/UpdateEntryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateEntryResult` | 成功结果；逐字段见types $defs/UpdateEntryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. ValidateEntry并人工授权
2. 投影所有受影响task注入量默认≤5120bytes否则VALIDATION并列taskId
3. CAS版本后写私有temp/fsync/rename权威Markdown并记录commit manifest，失败恢复旧指针
4. 数据库revision索引及outbox提交，人工编辑project overview后autoUpdateEnabled=false。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：更新entry并保存revision/outbox
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ValidateEntry`、`memory.ValidateProjectedBudgets`、`memory.PersistVersion`、`memory.Write`、`memory.RequireManager`、`memory.ReadEntryObject`

## memory.ArchiveEntry

- 级别：约束
- 文件：`modules/memory/internal/entry/write.go`
- 作用：归档停止未来注入

```go
func ArchiveEntry(ctx context.Context, op OperationContext, input ArchiveEntryInput) (ArchiveEntryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ArchiveEntryInput` | 封闭业务输入；逐字段见types $defs/ArchiveEntryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ArchiveEntryResult` | 成功结果；逐字段见types $defs/ArchiveEntryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限CAS
2. state archived
3. revision记录
4. context版本改变且已有快照可追溯
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写归档与上下文失效事件
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.PersistVersion`、`memory.Write`、`memory.RequireManager`、`memory.ReadEntryObject`

## memory.RestoreEntry

- 级别：约束
- 文件：`modules/memory/internal/entry/write.go`
- 作用：恢复归档记忆

```go
func RestoreEntry(ctx context.Context, op OperationContext, input RestoreEntryInput) (RestoreEntryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RestoreEntryInput` | 封闭业务输入；逐字段见types $defs/RestoreEntryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RestoreEntryResult` | 成功结果；逐字段见types $defs/RestoreEntryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 读取archived
2. CAS
3. active新version不复写旧revision
4. 更新memoryGeneration
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `CONFLICT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写新版本及事件
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ValidateEntry`、`memory.ValidateProjectedBudgets`、`memory.PersistVersion`、`memory.Write`、`memory.RequireManager`、`memory.ReadEntryObject`

## memory.PersistVersion

- 级别：参考
- 文件：`modules/memory/internal/entry/write.go`
- 作用：本域参考级辅助 PersistVersion；精确参数与返回见签名，调用边界随所属用例固定。

```go
func PersistVersion(ctx context.Context,s *memoryruntime.Store,tx *sql.Tx, op OperationContext,e gen.MemoryEntry,previous *int)(gen.MemoryEntry,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.ValidateEntry`、`memory.ValidateProjectedBudgets`、`memory.EmitContextChanged`、`memory.SaveEntryObject`、`memory.InsertEntryVersion`、`memory.Emit`

## memory.ReadEntry

- 级别：约束
- 文件：`modules/memory/internal/entry/read.go`
- 作用：读取人工记忆或历史

```go
func ReadEntry(ctx context.Context, op OperationContext, input ReadEntryInput) (ReadEntryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadEntryInput` | 封闭业务输入；逐字段见types $defs/ReadEntryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadEntryResult` | 成功结果；逐字段见types $defs/ReadEntryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. memory.manage
2. 缺version取current
3. 历史只读
4. body不写访问日志
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只读，记录脱敏审计
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ReadSnapshot`、`memory.RequireManager`、`memory.ReadEntryObject`

## memory.ListEntries

- 级别：约束
- 文件：`modules/memory/internal/entry/read.go`
- 作用：管理条目列表

```go
func ListEntries(ctx context.Context, op OperationContext, input ListEntriesInput) (ListEntriesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ListEntriesInput` | 封闭业务输入；逐字段见types $defs/ListEntriesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListEntriesResult` | 成功结果；逐字段见types $defs/ListEntriesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限
2. 稳定entryId keyset
3. limit默认20
4. 返回hash及正文仅管理会话
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.sameFilter`、`memory.ReadSnapshot`、`memory.RequireManager`、`memory.DBError`、`memory.ReadEntryObject`

## memory.sameFilter

- 级别：参考
- 文件：`modules/memory/internal/entry/read.go`
- 作用：本域参考级辅助 sameFilter；精确参数与返回见签名，调用边界随所属用例固定。

```go
func sameFilter(a,b *string)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ListEntryRevisions

- 级别：约束
- 文件：`modules/memory/internal/entry/history.go`
- 作用：读取不可变记忆历史

```go
func ListEntryRevisions(ctx context.Context, op OperationContext, input ListEntryRevisionsInput) (ListEntryRevisionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ListEntryRevisionsInput` | 封闭业务输入；逐字段见types $defs/ListEntryRevisionsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListEntryRevisionsResult` | 成功结果；逐字段见types $defs/ListEntryRevisionsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限
2. version降序
3. 各bodyHash校验
4. 不允许历史修改
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ReadSnapshot`、`memory.RequireManager`、`memory.DBError`、`memory.ReadEntryObject`

## memory.PutMapping

- 级别：约束
- 文件：`modules/memory/internal/mapping/write.go`
- 作用：维护任务映射

```go
func PutMapping(ctx context.Context, op OperationContext, input PutMappingInput) (PutMappingResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutMappingInput` | 封闭业务输入；逐字段见types $defs/PutMappingInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutMappingResult` | 成功结果；逐字段见types $defs/PutMappingResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. taskId引用根唯一AI task catalog
2. entry存在且语言范围合理
3. 投影该task完整注入量≤默认5120bytes否则拒写列taskId
4. CAS映射及global mappingRevision。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：映射写入与context.changed
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ValidateProjectedBudgets`、`memory.EmitContextChanged`、`memory.advance`、`memory.Write`、`memory.RequireManager`、`memory.DBError`、`memory.ValidateTask`、`memory.ReadEntryObject`

## memory.RemoveMapping

- 级别：约束
- 文件：`modules/memory/internal/mapping/write.go`
- 作用：移除任务映射

```go
func RemoveMapping(ctx context.Context, op OperationContext, input RemoveMappingInput) (RemoveMappingResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RemoveMappingInput` | 封闭业务输入；逐字段见types $defs/RemoveMappingInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RemoveMappingResult` | 成功结果；逐字段见types $defs/RemoveMappingResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限CAS
2. 软删除mapping
3. mappingRevision++
4. 只影响后续context
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写映射失效与事件
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.EmitContextChanged`、`memory.advance`、`memory.Write`、`memory.RequireManager`、`memory.DBError`

## memory.advance

- 级别：参考
- 文件：`modules/memory/internal/mapping/write.go`
- 作用：本域参考级辅助 advance；精确参数与返回见签名，调用边界随所属用例固定。

```go
func advance(ctx context.Context,tx *sql.Tx)(int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.ReorderMappings

- 级别：约束
- 文件：`modules/memory/internal/mapping/order.go`
- 作用：稳定重排任务记忆

```go
func ReorderMappings(ctx context.Context, op OperationContext, input ReorderMappingsInput) (ReorderMappingsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReorderMappingsInput` | 封闭业务输入；逐字段见types $defs/ReorderMappingsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReorderMappingsResult` | 成功结果；逐字段见types $defs/ReorderMappingsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 传入集合必须全量无重复
2. CAS全局revision
3. order=索引*10
4. 一事务写映射与版本
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写排序不修改条目正文
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ValidateProjectedBudgets`、`memory.ReadTaskBudget`、`memory.EmitContextChanged`、`memory.ReadMappings`、`memory.advance`、`memory.Write`、`memory.RequireManager`、`memory.DBError`、`memory.ValidateTask`

## memory.ListMappings

- 级别：约束
- 文件：`modules/memory/internal/mapping/read.go`
- 作用：读取任务映射

```go
func ListMappings(ctx context.Context, op OperationContext, input ListMappingsInput) (ListMappingsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ListMappingsInput` | 封闭业务输入；逐字段见types $defs/ListMappingsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListMappingsResult` | 成功结果；逐字段见types $defs/ListMappingsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限
2. taskId允许
3. order/mappingId稳定排序
4. 返回全局版本
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ReadMappings`、`memory.ReadSnapshot`、`memory.RequireManager`、`memory.DBError`、`memory.ValidateTask`

## memory.PutTaskBudget

- 级别：约束
- 文件：`modules/memory/internal/context/budget.go`
- 作用：配置任务注入上限

```go
func PutTaskBudget(ctx context.Context, op OperationContext, input PutTaskBudgetInput) (PutTaskBudgetResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutTaskBudgetInput` | 封闭业务输入；逐字段见types $defs/PutTaskBudgetInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutTaskBudgetResult` | 成功结果；逐字段见types $defs/PutTaskBudgetResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. bytes1024..65536 entries1..64
2. CAS
3. 预算版本进入context hash
4. 不扩大模型token额度
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写预算及context.changed
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ValidateProjectedBudgets`、`memory.ReadTaskBudget`、`memory.EmitContextChanged`、`memory.Write`、`memory.RequireManager`、`memory.DBError`、`memory.ValidateTask`

## memory.ReadTaskBudget

- 级别：参考
- 文件：`modules/memory/internal/context/budget.go`
- 作用：读取注入预算

```go
func ReadTaskBudget(ctx context.Context,q memoryruntime.Queryer,site,task string)(gen.TaskBudget,error)
```

**输入**

无

**输出**

无

**步骤**

1. 验证taskId
2. 未自定义使用5120bytes/64entries revision0
3. 返回有效预算
4. 无外部调用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.DBError`
- 其他依赖说明：参考级实际输入输出以本次精确Go签名为准；数据字段继续复用MemoryEntry/TaskMapping/TaskBudget与本域事务，不再建立独立读取事务。

## memory.ListTaskBudgets

- 级别：约束
- 文件：`modules/memory/internal/context/budget.go`
- 作用：读取各任务记忆注入预算

```go
func ListTaskBudgets(ctx context.Context, op OperationContext, input ListTaskBudgetsInput) (ListTaskBudgetsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ListTaskBudgetsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListTaskBudgetsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. memory.manage
2. 按根AI任务目录列出每任务有效预算（未自定义为默认5120字节/64条）与版本

**错误**

- `FORBIDDEN`：权限不足（不可重试）

- 副作用：无
- 授权：permission：memory.manage；rule：后台管理员
- 并发：只读快照
- 幂等：只读
- 依赖：`memory.ReadTaskBudget`、`memory.ReadSnapshot`、`memory.RequireManager`

## memory.SelectContextEntries

- 级别：参考
- 文件：`modules/memory/internal/context/select.go`
- 作用：按范围确定注入候选

```go
func SelectContextEntries(entries []gen.MemoryEntry,mappings []gen.TaskMapping,task,language string)SelectContextEntriesResult
```

**输入**

无

**输出**

无

**步骤**

1. 固定mappingRevision读取enabled
2. active条目且language目标优先und
3. priority降序/order/entryId稳定排序
4. 相同entry去重
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无
- 其他依赖说明：参考级实际输入输出以本次精确Go签名为准；数据字段继续复用MemoryEntry/TaskMapping/TaskBudget与本域事务，不再建立独立读取事务。

## memory.ReadMappings

- 级别：参考
- 文件：`modules/memory/internal/context/select.go`
- 作用：本域参考级辅助 ReadMappings；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ReadMappings(ctx context.Context,q memoryruntime.Queryer,site string)([]gen.TaskMapping,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.RenderContext

- 级别：参考
- 文件：`modules/memory/internal/context/render.go`
- 作用：生成防混淆记忆上下文

```go
func RenderContext(ctx context.Context, op OperationContext, input RenderContextInput)(RenderContextResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RenderContextInput` | 封闭业务输入；逐字段见types $defs/RenderContextInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderContextResult` | 成功结果；逐字段见types $defs/RenderContextResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 固定renderer先description再正文带entry/version边界
2. 作为system前部任务记忆但不是工具授权
3. 精确UTF8bytes≤task budget
4. canonical manifest/text SHA256形成versionHash。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ApplyContextBudget`、`memory.RenderText`

## memory.RenderText

- 级别：参考
- 文件：`modules/memory/internal/context/render.go`
- 作用：本域参考级辅助 RenderText；精确参数与返回见签名，调用边界随所属用例固定。

```go
func RenderText(entries []gen.MemoryEntry)string
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.Write`

## memory.ApplyContextBudget

- 级别：参考
- 文件：`modules/memory/internal/context/render.go`
- 作用：确定性截选完整条目

```go
func ApplyContextBudget(ctx context.Context, op OperationContext, input ApplyContextBudgetInput)(ApplyContextBudgetResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ApplyContextBudgetInput` | 封闭业务输入；逐字段见types $defs/ApplyContextBudgetInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApplyContextBudgetResult` | 成功结果；逐字段见types $defs/ApplyContextBudgetResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 精确计算全部选择条目的标题/description/正文边界UTF8bytes
2. 预算≤5120默认且每entry≤其limit
3. 超预算INTEGRITY_ERROR拒发不静默截条（写时已preflight）
4. 保持mapping顺序所有条目不截断。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.RenderText`

## memory.ResolveContext

- 级别：约束
- 文件：`modules/memory/internal/context/service.go`
- 作用：跨服务取得版本化记忆上下文

```go
func ResolveContext(ctx context.Context, op OperationContext, input ResolveContextInput) (MemoryContext, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ResolveContextInput` | 封闭业务输入；逐字段见types $defs/ResolveContextInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MemoryContext` | 成功结果；逐字段见types $defs/MemoryContext | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. callerService允许taskId
2. 一只读快照固定entry/mapping/budget
3. Select→Budget→Render
4. 回传公共MemoryContext无额外字段
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只读，可内容寻址记录manifest；不调模型
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.SelectContextEntries`、`memory.ReadTaskBudget`、`memory.ApplyContextBudget`、`memory.RenderContext`、`memory.ReadMappings`、`memory.allowedCaller`、`memory.ReadSnapshot`、`memory.DBError`、`memory.ReadAllEntries`、`memory.SaveContextManifest`

## memory.allowedCaller

- 级别：参考
- 文件：`modules/memory/internal/context/service.go`
- 作用：本域参考级辅助 allowedCaller；精确参数与返回见签名，调用边界随所属用例固定。

```go
func allowedCaller(caller,task string)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ReadContextManifest

- 级别：约束
- 文件：`modules/memory/internal/context/manifest.go`
- 作用：查询版本证据清单

```go
func ReadContextManifest(ctx context.Context, op OperationContext, input ReadContextManifestInput) (ReadContextManifestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadContextManifestInput` | 封闭业务输入；逐字段见types $defs/ReadContextManifestInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadContextManifestResult` | 成功结果；逐字段见types $defs/ReadContextManifestResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. memory.manage
2. versionHash命中不可变manifest
3. 返回IDs/versions不含正文
4. 可追溯预算及renderer
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ReadSnapshot`、`memory.RequireManager`、`memory.DBError`

## memory.CreateAssistantRun

- 级别：约束
- 文件：`modules/memory/internal/assistant/create.go`
- 作用：显式运行记忆助手

```go
func CreateAssistantRun(ctx context.Context, op OperationContext, input CreateAssistantRunInput) (CreateAssistantRunResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateAssistantRunInput` | 封闭业务输入；逐字段见types $defs/CreateAssistantRunInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateAssistantRunResult` | 成功结果；逐字段见types $defs/CreateAssistantRunResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅memory.manage即可使用已配置助手，不要求model.manage/superadmin且不修改渠道/档位/任务模型分配
2. 根memory.assistant映射读取已配置accurate模型，未配置明确禁调用；固定entry versions/memoryGeneration
3. 2a 按可信admin-probe读取readProfiles，以TaskAssignment.tier匹配唯一且已配置模型列表的实际profileId，不以tier伪造ID；清原portal proof，以memory服务身份保留root/site/epoch免费读取。
4. 输入≤16384bytes，readonly只建议人工直接编辑
5. 可信admin-probe根任务排队，统一MG执行及计费
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写assistant run与job；无自动修改
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`identity:checkAuthorization`、`jobs:createJob`、`model-gateway:readTaskAssignments`、`model-gateway:readProfiles`、`memory.BuildAssistantPrompt`、`memory.Write`、`memory.RequireManager`、`memory.DBError`、`memory.ReadEntryObject`、`memory.SelectAssistantProfile`、`memory.SaveAssistantInput`、`memory.ReplayReceipt`

## memory.BuildAssistantPrompt

- 级别：参考
- 文件：`modules/memory/internal/assistant/prompt.go`
- 作用：构造只允许提议助手输入

```go
func BuildAssistantPrompt(ctx context.Context, op OperationContext, input BuildAssistantPromptInput)(BuildAssistantPromptResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BuildAssistantPromptInput` | 封闭业务输入；逐字段见types $defs/BuildAssistantPromptInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildAssistantPromptResult` | 成功结果；逐字段见types $defs/BuildAssistantPromptResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 固定指令只输出proposal schema
2. 明示无写权限
3. 输入记忆作为数据隔离
4. 禁生成密钥/IP/会话记录
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## memory.RunAssistant

- 级别：约束
- 文件：`modules/memory/internal/assistant/execute.go`
- 作用：通过MG产生候选提议

```go
func RunAssistant(ctx context.Context, op OperationContext, input RunAssistantInput) (RunAssistantResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RunAssistantInput` | 封闭业务输入；逐字段见types $defs/RunAssistantInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RunAssistantResult` | 成功结果；逐字段见types $defs/RunAssistantResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 必须受信job绑定runId/inputRef SHA256/rootOperationId/dataEpoch/jobId；模型前及提交前锁外CheckLease，租约未知失败关闭；同op+assistant步骤复用MG持久结果不重新收费。
2. lease/cancel检查
3. 根memory.assistant accurate档max1500output token，可信admin-probe根
4. 严格解析new/update/delete/noop候选和readonly规则
5. 不写active，只保存pending差异候选。
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
7. 仅消费CurrentSnapshot.Output.Text非nil的真实完整winning结果，显式空字符串可用；不由历史delta拼接或把费用未知当作没有结果，原费用/paid refs继续保留。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG费用与pending proposal暂存
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.InvokeModel`、`memory.BuildAssistantPrompt`、`memory.ValidateProposal`、`memory.CommitAssistantProposals`、`memory.CheckJobBinding`、`memory.ValidateForeign`、`memory.CurrentAssistantState`

## memory.CheckJobBinding

- 级别：参考
- 文件：`modules/memory/internal/assistant/execute.go`
- 作用：本域参考级辅助 CheckJobBinding；精确参数与返回见签名，调用边界随所属用例固定。

```go
func CheckJobBinding(ctx context.Context, op OperationContext,runID string,job platform.JobExecutionInput)(memoryruntime.AssistantInputRecord,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.AssistantInput`

## memory.ValidateProposal

- 级别：参考
- 文件：`modules/memory/internal/proposal/validate.go`
- 作用：验证候选提议闭包

```go
func ValidateProposal(ctx context.Context, op OperationContext, input ValidateProposalInput)(ValidateProposalResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateProposalInput` | 封闭业务输入；逐字段见types $defs/ValidateProposalInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateProposalResult` | 成功结果；逐字段见types $defs/ValidateProposalResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. kind对应字段required且其他字段禁止
2. entry_update/archive必须目标及旧version
3. glossary必须目标语言/术语/旧glossaryGeneration
4. 3a glossary额外强制sourceLanguage，非glossary禁止；payloadHash为移除payloadHash后完整pending候选及sourceGeneration的canonical JSON SHA256。
5. 无批准人允许pending
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ContainsPrivateMaterial`

## memory.CommitAssistantProposals

- 级别：参考
- 文件：`modules/memory/internal/assistant/commit.go`
- 作用：提交候选防旧覆盖

```go
func CommitAssistantProposals(ctx context.Context, op OperationContext, input CommitAssistantProposalsInput) (CommitAssistantProposalsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CommitAssistantProposalsInput` | 封闭业务输入；逐字段见types $defs/CommitAssistantProposalsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitAssistantProposalsResult` | 成功结果；逐字段见types $defs/CommitAssistantProposalsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 同RunAssistant受信JobExecutionInput绑定且提交前锁外CheckLease；原子判断运行取消、输入代数及全部目标版本。
2. 当前memoryGeneration一致
3. 每提议目标expectedVersion仍一致
4. 不一致run superseded
5. pending proposals与run review同事务
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只写审核候选；不应用内容
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.CheckJobBinding`、`memory.Write`、`memory.DBError`、`memory.ReadEntryObject`

## memory.ReadAssistantRun

- 级别：约束
- 文件：`modules/memory/internal/assistant/read.go`
- 作用：管理助手提议

```go
func ReadAssistantRun(ctx context.Context, op OperationContext, input ReadAssistantRunInput) (ReadAssistantRunResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadAssistantRunInput` | 封闭业务输入；逐字段见types $defs/ReadAssistantRunInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadAssistantRunResult` | 成功结果；逐字段见types $defs/ReadAssistantRunResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. memory.manage
2. 原创建人或有权限管理员可读
3. 返回候选旧版本证据
4. 不含密钥/原模型敏感日志
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ReadSnapshot`、`memory.RequireManager`、`memory.DBError`

## memory.ApproveProposal

- 级别：约束
- 文件：`modules/memory/internal/proposal/approve.go`
- 作用：人工批准具体提议

```go
func ApproveProposal(ctx context.Context, op OperationContext, input ApproveProposalInput) (ApproveProposalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ApproveProposalInput` | 封闭业务输入；逐字段见types $defs/ApproveProposalInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApproveProposalResult` | 成功结果；逐字段见types $defs/ApproveProposalResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 必须actor.kind=admin且memory.manage
2. 展示payload hash与版本匹配
3. CAS pending→approved记录approvedBy
4. entry类调用应用事务，glossary写批准command outbox
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写批准记录；非人工身份403
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`identity:checkAuthorization`、`memory.ValidateProposal`、`memory.ApplyEntryProposal`、`memory.EnqueueGlossaryCommand`、`memory.ReadProposal`、`memory.SaveProposal`、`memory.Write`、`memory.RequireManager`、`memory.DBError`、`memory.Emit`

## memory.RejectProposal

- 级别：约束
- 文件：`modules/memory/internal/proposal/approve.go`
- 作用：人工拒绝未应用候选

```go
func RejectProposal(ctx context.Context, op OperationContext, input RejectProposalInput) (RejectProposalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RejectProposalInput` | 封闭业务输入；逐字段见types $defs/RejectProposalInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RejectProposalResult` | 成功结果；逐字段见types $defs/RejectProposalResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只pending可拒绝
2. CAS revision
3. rejected记录人/理由
4. 不影响active记忆
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：审核状态写入
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.Write`、`memory.RequireManager`、`memory.DBError`

## memory.ReadProposal

- 级别：参考
- 文件：`modules/memory/internal/proposal/approve.go`
- 作用：本域参考级辅助 ReadProposal；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ReadProposal(ctx context.Context,q memoryruntime.Queryer,site,id string)(gen.MemoryProposal,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.SaveProposal

- 级别：参考
- 文件：`modules/memory/internal/proposal/approve.go`
- 作用：本域参考级辅助 SaveProposal；精确参数与返回见签名，调用边界随所属用例固定。

```go
func SaveProposal(ctx context.Context,tx *sql.Tx, op OperationContext,p gen.MemoryProposal)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.ApplyEntryProposal

- 级别：参考
- 文件：`modules/memory/internal/proposal/apply-entry.go`
- 作用：应用批准的记忆内容

```go
func ApplyEntryProposal(ctx context.Context,s *memoryruntime.Store,tx *sql.Tx, op OperationContext,p gen.MemoryProposal,expected int)(ApplyEntryProposalResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 必须已人工批准且hash一致
2. readonly=true的助手候选禁止应用仅建议直接人工编辑
3. 2a entry_create默认description=proposedTitle、maximumBytes=1536、priority=100、tags=[]、readonly=false、source=assistant、autoUpdateEnabled=false，新entryId在批准事务生成；entry_update保留其余元数据。
4. CAS旧entry version并投影全部映射任务预算
5. 原子权威Markdown新对象+manifest与revision索引提交，proposal applied及context.changed同事务。
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：更新记忆与提议；不可绕过人工批准
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ValidateProjectedBudgets`、`memory.PersistVersion`、`memory.SaveProposal`、`memory.ReadEntryObject`、`memory.Emit`
- 其他依赖说明：参考级实际输入输出以本次精确Go签名为准；数据字段继续复用MemoryEntry/TaskMapping/TaskBudget与本域事务，不再建立独立读取事务。

## memory.EnqueueGlossaryCommand

- 级别：参考
- 文件：`modules/memory/internal/proposal/glossary-command.go`
- 作用：产生已批准术语命令

```go
func EnqueueGlossaryCommand(ctx context.Context,tx *sql.Tx, op OperationContext,p gen.MemoryProposal)(EnqueueGlossaryCommandResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 校验kind=glossary及admin approval
2. 1a glossary强制sourceLanguage；全局人工术语使用新termId/version0/source=manual/scope=global/generation=expectedGlossaryGeneration，禁止猜测源语言。
3. 规范ApprovedGlossaryProposal含批准修订/hash
4. 唯一proposalId command
5. applying与outbox同事务
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写发送localization命令队列；此函数不调用localization
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.SaveProposal`、`memory.DBError`
- 其他依赖说明：参考级实际输入输出以本次精确Go签名为准；数据字段继续复用MemoryEntry/TaskMapping/TaskBudget与本域事务，不再建立独立读取事务。

## memory.DispatchGlossaryCommand

- 级别：参考
- 文件：`modules/memory/internal/proposal/glossary-dispatch.go`
- 作用：异步发送批准术语命令

```go
func DispatchGlossaryCommand(ctx context.Context, op OperationContext, input DispatchGlossaryCommandInput) (DispatchGlossaryCommandResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `DispatchGlossaryCommandInput` | 封闭业务输入；逐字段见types $defs/DispatchGlossaryCommandInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DispatchGlossaryCommandResult` | 成功结果；逐字段见types $defs/DispatchGlossaryCommandResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 领取独立command租约
2. HTTP请求localization applyApprovedGlossaryProposal
3. 同proposal/hash重试最多3次500/1000/2000ms
4. ACK仅accepted，终态等事件
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：localization接收命令；无同步memory回调依赖
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization:applyApprovedGlossaryProposal`

## memory.HandleGlossaryProposalResult

- 级别：约束
- 文件：`modules/memory/internal/events/glossary-result.go`
- 作用：消费术语批准结果

```go
func HandleGlossaryProposalResult(ctx context.Context, op OperationContext, input HandleGlossaryProposalResultInput) (HandleGlossaryProposalResultResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `HandleGlossaryProposalResultInput` | 封闭业务输入；逐字段见types $defs/HandleGlossaryProposalResultInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleGlossaryProposalResultResult` | 成功结果；逐字段见types $defs/HandleGlossaryProposalResultResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. inbox去重
2. proposal/hash/approvalRevision匹配
3. applying→applied或conflicted
4. 不请求localization/memory resolve再循环
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写proposal终态与审计outbox
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`platform.ClaimInbox`

## memory.CancelAssistantRun

- 级别：约束
- 文件：`modules/memory/internal/assistant/cancel.go`
- 作用：取消未完成助手调用

```go
func CancelAssistantRun(ctx context.Context, op OperationContext, input CancelAssistantRunInput) (CancelAssistantRunResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CancelAssistantRunInput` | 封闭业务输入；逐字段见types $defs/CancelAssistantRunInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CancelAssistantRunResult` | 成功结果；逐字段见types $defs/CancelAssistantRunResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 人工权限
2. 写cancel
3. MG cancelInference/jobs取消
4. pending未批准候选标rejected，已批准不回滚
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：终止助手及子调用，不自行释放MG账单
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway:cancelInference`、`jobs:cancelJob`、`memory.CancelRunTransaction`、`memory.Write`、`memory.RequireManager`、`memory.DispatchAssistantCancellations`

## memory.CancelRunTransaction

- 级别：参考
- 文件：`modules/memory/internal/assistant/cancel.go`
- 作用：本域参考级辅助 CancelRunTransaction；精确参数与返回见签名，调用边界随所属用例固定。

```go
func CancelRunTransaction(ctx context.Context,tx *sql.Tx, op OperationContext,runID string)(gen.AssistantRun,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.RecoverAssistantLeases

- 级别：参考
- 文件：`modules/memory/internal/runtime/recovery.go`
- 作用：恢复助手执行与批准命令

```go
func RecoverAssistantLeases(ctx context.Context, op OperationContext, input RecoverAssistantLeasesInput)(RecoverAssistantLeasesResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RecoverAssistantLeasesInput` | 封闭业务输入；逐字段见types $defs/RecoverAssistantLeasesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverAssistantLeasesResult` | 成功结果；逐字段见types $defs/RecoverAssistantLeasesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 到期lease禁止旧写
2. 原MG流可续才续
3. sourceGeneration已变superseded
4. command ACK未终态重读事件状态不重复批准
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写恢复状态/重排command
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway:readInference`、`jobs:createJob`、`memory.Write`、`memory.DBError`、`memory.DispatchAssistantInputs`、`memory.DispatchAssistantCancellations`

## memory.PublishOutbox

- 级别：约束
- 文件：`modules/memory/internal/events/outbox.go`
- 作用：发布记忆引用式事件

```go
func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PublishOutboxInput` | 封闭业务输入；逐字段见types $defs/PublishOutboxInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishOutboxResult` | 成功结果；逐字段见types $defs/PublishOutboxResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 领取租约
2. payload只含entry/proposal ID与versionHash
3. jobs持久ACK标published
4. 有限重试后保留下一轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：持久事件不广播记忆正文
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:publishEvent`、`platform.RunRelay`、`memory.RelayBatch`

## memory.PruneContextSnapshots

- 级别：参考
- 文件：`modules/memory/internal/runtime/prune.go`
- 作用：回收无引用派生注入清单

```go
func PruneContextSnapshots(ctx context.Context, op OperationContext, input PruneContextSnapshotsInput) (PruneContextSnapshotsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PruneContextSnapshotsInput` | 封闭业务输入；逐字段见types $defs/PruneContextSnapshotsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PruneContextSnapshotsResult` | 成功结果；逐字段见types $defs/PruneContextSnapshotsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 不删除entry历史
2. 无活跃任务/审计保留引用才回收
3. 验证私有文件范围
4. 删除manifest记录
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：删除本服务派生快照
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## memory.AcquireBarrier

- 级别：约束
- 文件：`modules/memory/internal/protection/protocol.go`
- 作用：取得本地维护屏障

```go
func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BarrierRequest` | 封闭业务输入；逐字段见types $defs/BarrierRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 成功结果；逐字段见types $defs/BarrierStatus | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验data-protection服务身份（用户授权已由data-protection核验）
2. 将本地control.db屏障draining并拒绝新业务写
3. 屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令
4. 导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。
5. 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。
6. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：持久控制屏障；不纳入业务快照
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.EnterBarrier`

## memory.ReleaseBarrier

- 级别：约束
- 文件：`modules/memory/internal/protection/protocol.go`
- 作用：释放已授权维护屏障

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BarrierRequest` | 封闭业务输入；逐字段见types $defs/BarrierRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 成功结果；逐字段见types $defs/BarrierStatus | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验相同barrierId/operationId与data-protection服务身份
2. 暂存目录不影响释放；切换只在进程重启时发生
3. runtime处于restore_switching或restore_rolling_back时不恢复业务worker，等待重启换目录
4. released后开放该epoch业务写。
5. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：更新本地control.db；不修改导入业务
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.ReleaseBarrier`

## memory.ExportSnapshot

- 级别：约束
- 文件：`modules/memory/internal/protection/protocol.go`
- 作用：导出安全watermark快照

```go
func ExportSnapshot(ctx context.Context, op OperationContext, input SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SnapshotExportRequest` | 封闭业务输入；逐字段见types $defs/SnapshotExportRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 成功结果；逐字段见types $defs/SnapshotExportResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 验证safe屏障及dataEpoch
2. checkpoint本域私有数据并固定revisionWatermark
3. 屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令
4. 导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。
5. 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。
6. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：创建本服务不可变快照对象
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.Export`

## memory.StageSnapshot

- 级别：约束
- 文件：`modules/memory/internal/protection/protocol.go`
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
2. 经data-protection:serveRestoreArchive按restoreId拉取本域归档，边读边算SHA-256，写入<dataRoot>/restore/<restoreId>/staged.tmp
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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`、`platform.SnapshotManager.Stage`

## memory.DiscardSnapshot

- 级别：约束
- 文件：`modules/memory/internal/protection/protocol.go`
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
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.Discard`

## memory.ValidateProjectedBudgets

- 级别：参考
- 文件：`modules/memory/internal/context/preflight.go`
- 作用：保存前完整投影注入预算

```go
func ValidateProjectedBudgets(ctx context.Context,s *memoryruntime.Store,q memoryruntime.Queryer,site string,entry *gen.MemoryEntry,mapping *gen.TaskMapping,budget *gen.TaskBudget)(ValidateProjectedBudgetsResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 读固定entry/mapping snapshot
2. 应用候选变更到内存投影不持久
3. 按每任务/语言完整renderer bytes计算
4. 任一超budget返回VALIDATION field issues带taskId不含正文
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.ReadTaskBudget`、`memory.SelectContextEntries`、`memory.joinTaskIDs`、`memory.RenderText`、`memory.ReadMappings`、`memory.ReadAllEntries`
- 其他依赖说明：参考级实际输入输出以本次精确Go签名为准；数据字段继续复用MemoryEntry/TaskMapping/TaskBudget与本域事务，不再建立独立读取事务。

## memory.joinTaskIDs

- 级别：参考
- 文件：`modules/memory/internal/context/preflight.go`
- 作用：本域参考级辅助 joinTaskIDs；精确参数与返回见签名，调用边界随所属用例固定。

```go
func joinTaskIDs(ids []string)string
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.EmitContextChanged

- 级别：参考
- 文件：`modules/memory/internal/context/preflight.go`
- 作用：本域参考级辅助 EmitContextChanged；精确参数与返回见签名，调用边界随所属用例固定。

```go
func EmitContextChanged(ctx context.Context,tx *sql.Tx, op OperationContext,tasks []string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`、`memory.Emit`

## memory.UpdateProjectOverview

- 级别：约束
- 文件：`modules/memory/internal/assistant/overview.go`
- 作用：自动维护项目概述并保护人工修改

```go
func UpdateProjectOverview(ctx context.Context, op OperationContext, input UpdateProjectOverviewInput) (UpdateProjectOverviewResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `UpdateProjectOverviewInput` | 封闭业务输入；逐字段见types $defs/UpdateProjectOverviewInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateProjectOverviewResult` | 成功结果；逐字段见types $defs/UpdateProjectOverviewResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. source=auto且autoUpdateEnabled才继续
2. daily或publishedCount≥20触发
3. content当前公开标题摘要固定快照→fast task memory.overview
4. CAS entryVersion/autoUpdateEnabled并预算preflight保存，否则旧结果superseded
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG费用与自动概述文件更新，人工锁后不写
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:readPublishedRevision`、`memory.InvokeModel`

## memory.AppendAssistantMessage

- 级别：约束
- 文件：`modules/memory/internal/assistant/conversation.go`
- 作用：保存可连续的记忆助手对话

```go
func AppendAssistantMessage(ctx context.Context, op OperationContext, input AppendAssistantMessageInput) (AppendAssistantMessageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `AppendAssistantMessageInput` | 封闭业务输入；逐字段见types $defs/AppendAssistantMessageInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AppendAssistantMessageResult` | 成功结果；逐字段见types $defs/AppendAssistantMessageResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. actor admin且memory.manage
2. owner及CAS版本
3. body≤8192bytes且不接受内部密钥
4. 追加ordinal消息和30天可调TTL，需整理时另createAssistantRun
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写助手会话消息，无自动模型调用
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.readConversation`、`memory.ContainsPrivateMaterial`、`memory.Write`、`memory.RequireManager`、`memory.DBError`

## memory.ReadAssistantConversation

- 级别：约束
- 文件：`modules/memory/internal/assistant/conversation.go`
- 作用：读取记忆助手连续历史

```go
func ReadAssistantConversation(ctx context.Context, op OperationContext, input ReadAssistantConversationInput) (ReadAssistantConversationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadAssistantConversationInput` | 封闭业务输入；逐字段见types $defs/ReadAssistantConversationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadAssistantConversationResult` | 成功结果；逐字段见types $defs/ReadAssistantConversationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. owner/admin权限
2. 到期NOT_FOUND
3. ordinal keyset分页
4. 展示每run对应差异提议不自动应用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory.readConversation`、`memory.ReadSnapshot`、`memory.RequireManager`、`memory.DBError`

## memory.DeleteAssistantConversation

- 级别：约束
- 文件：`modules/memory/internal/assistant/conversation.go`
- 作用：清空助手对话

```go
func DeleteAssistantConversation(ctx context.Context, op OperationContext, input DeleteAssistantConversationInput) (DeleteAssistantConversationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `DeleteAssistantConversationInput` | 封闭业务输入；逐字段见types $defs/DeleteAssistantConversationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DeleteAssistantConversationResult` | 成功结果；逐字段见types $defs/DeleteAssistantConversationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. owner及memory.manage
2. 取消未完成关联run
3. 删除聊天正文保留批准审计与entry历史
4. 同key返回同结果
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：删除管理员聊天与取消子任务
- 授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway:cancelInference`、`memory.CancelRunTransaction`、`memory.readConversation`、`memory.Write`、`memory.RequireManager`、`memory.DBError`、`memory.DispatchAssistantCancellations`

## memory.readConversation

- 级别：参考
- 文件：`modules/memory/internal/assistant/conversation.go`
- 作用：本域参考级辅助 readConversation；精确参数与返回见签名，调用边界随所属用例固定。

```go
func readConversation(ctx context.Context,q memoryruntime.Queryer,site,id string)(string,int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.RunWorker

- 级别：约束
- 文件：`modules/memory/internal/runtime/worker.go`
- 作用：主动领取本域持久任务

```go
func RunWorker(ctx context.Context, op OperationContext, input RunWorkerInput) (RunWorkerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RunWorkerInput` | 封闭业务输入；逐字段见types $defs/RunWorkerInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RunWorkerResult` | 成功结果；逐字段见types $defs/RunWorkerResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 进程启动后循环调用jobs:claimJob，只领取本域implementation.json jobTypes登记的类型；维护或屏障非released时暂停领取
2. 领取后调用jobs:startJob确认开始，按jobType调用登记的执行函数；每10秒jobs:heartbeatJob续30秒租约、上报进度并读取cancelRequested
3. 执行函数写回结果前调用jobs:checkLease核对fence仍有效；成功jobs:completeJob，失败jobs:failJob并给出可重试分类；收到取消在安全点停止并以cancelled结束
4. lease过期或fence不符立即停止写入；已派发的模型调用不重放，由网关结算或对账

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：jobs租约/完成状态及本域工作链副作用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:claimJob`、`jobs:heartbeatJob`、`jobs:completeJob`、`jobs:failJob`、`jobs:startJob`、`jobs:checkLease`、`memory.RunAssistant`、`memory.UpdateProjectOverview`、`platform.RunWorkerLoop`

## memory.All

- 级别：参考
- 文件：`modules/memory/migrations/migrations.go`
- 作用：按编号升序返回本域业务库全部内嵌迁移并校验编号连续

```go
func All() ([]Migration, error)
```

**输入**

无

**输出**

无

**步骤**

1. go:embed读取NNNN_name.sql（既有NNN_name.sql兼容）；文件名或编号不合法、编号不连续即报错

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：只读内嵌迁移内容；执行由本域存储装配在事务中完成
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：纯读取，可并发调用
- 幂等：确定性输出
- 依赖：无

## memory.Control

- 级别：参考
- 文件：`modules/memory/migrations/migrations.go`
- 作用：按编号升序返回control区控制库全部内嵌迁移并校验编号连续

```go
func Control() ([]Migration, error)
```

**输入**

无

**输出**

无

**步骤**

1. go:embed读取NNNN_name.sql（既有NNN_name.sql兼容）；文件名或编号不合法、编号不连续即报错

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：只读内嵌迁移内容；执行由本域存储装配在事务中完成
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：纯读取，可并发调用
- 幂等：确定性输出
- 依赖：无

## memory.main

- 级别：约束
- 文件：`modules/memory/cmd/memory/main.go`
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
- 依赖：`memory.BuildApplication`、`memory.Serve`、`memory.Shutdown`、`platform.RunService`

## memory.BuildApplication

- 级别：约束
- 文件：`modules/memory/internal/app/wire.go`
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
6. 启动后按implementation.json backgroundTasks登记的周期运行后台函数，并以服务身份经jobs:createSchedule幂等登记本域系统默认schedule；维护期间暂停业务循环
7. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`memory.LoadRuntime`、`memory.RecoverAssistantLeases`、`memory.DispatchGlossaryCommand`、`memory.PruneContextSnapshots`、`memory.PublishOutbox`、`memory.ConsumeEvents`、`memory.RunWorker`、`jobs:createSchedule`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`memory.NewApplication`、`memory.ConfigureWrites`、`memory.ConfigurePublisher`、`memory.ConfigureGateway`、`memory.ConfigureAssistantClients`、`memory.ConfigureValidators`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`、`model-gateway:readInference`、`model-gateway:lookupInference`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## memory.Serve

- 级别：约束
- 文件：`modules/memory/internal/transport/server.go`
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
- 依赖：`memory.BuildApplication`、`memory.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## memory.Shutdown

- 级别：约束
- 文件：`modules/memory/internal/transport/server.go`
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

## memory.Health

- 级别：约束
- 文件：`modules/memory/internal/transport/server.go`
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

## memory.Ready

- 级别：约束
- 文件：`modules/memory/internal/transport/server.go`
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

## memory.NewApplication

- 级别：参考
- 文件：`modules/memory/internal/transport/server.go`
- 作用：本域参考级辅助 NewApplication；精确参数与返回见签名，调用边界随所属用例固定。

```go
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, openedEpoch platform.Identifier, authorize func(context.Context,platform.OperationContext,string,string)error) (*Application,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.operationHandlers`

## memory.ConsumeEvents

- 级别：约束
- 文件：`modules/memory/internal/events/subscription.go`
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
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`memory.HandleGlossaryProposalResult`、`memory.CountPublication`、`platform.RunConsumer`

## memory.InvokeModel

- 级别：参考
- 文件：`modules/memory/internal/runtime/model_client.go`
- 作用：经模型网关执行一次调用并读取到终态

```go
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput)(InvokeModelResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `InvokeModelInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InvokeModelResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 本域只装配可信op与原准入/批准，公共InvokeModel按原完整请求和同一幂等键先lookup并观察当前快照；仅生产方专用INVOCATION_NOT_ACCEPTED负证明且原批准截止仍有效时首次create。
2. Request.IDempotencyKey与外层原键完全相同；普通NOT_FOUND不视为未接受；Get/Lookup只读scope不延长付费deadline。
3. 完整结果完成与费用状态分别保留；取消、续读、真实接受后错误和已有引用均由公共适配器保留，不换新键或新owner重付。

**错误**

- `BUDGET_EXCEEDED`：网关额度不足（不可重试）
- `CANCELLED`：调用被取消（不可重试）
- `DEPENDENCY_UNAVAILABLE`：网关不可用或调用失败（可重试）
- `UNSUPPORTED_CAPABILITY`：任务未配置模型（不可重试）

- 副作用：HTTP调用model-gateway推理接口；模型费用由网关记账
- 授权：permission：service；rule：以本服务身份调用网关，费用类别继承可信op
- 并发：每次调用独立；同一根操作的子调用共享网关费用作用域
- 幂等：幂等键固定，重试返回原invocation
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`、`platform.InvokeModel`、`model-gateway:readInference`、`model-gateway:lookupInference`

## memory.ConfigureGateway

- 级别：参考
- 文件：`modules/memory/internal/runtime/model_client.go`
- 作用：本域参考级辅助 ConfigureGateway；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ConfigureGateway(client platform.GatewayClient)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.CountPublication

- 级别：约束
- 文件：`modules/memory/internal/events/publication-count.go`
- 作用：统计新发布数以触发项目概述更新

```go
func CountPublication(ctx context.Context, op OperationContext, input EventDelivery) (EventHandled, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `EventDelivery` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EventHandled` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. inbox按eventId+payloadHash去重，只计入更高publicationRevision
2. 累加自上次概述以来的新发布计数
3. 计数≥20且项目概述source=auto、autoUpdateEnabled=true时创建memory.overview任务（按计数窗口去重）并清零

**错误**

- `VERSION_MISMATCH`：旧代事件（不可重试）

- 副作用：写计数与任务
- 授权：permission：service；rule：仅由本域ConsumeEvents调用
- 并发：计数行CAS
- 幂等：eventId去重
- 依赖：`jobs:createJob`、`platform.ClaimInbox`

## memory.ConfigureWrites

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 ConfigureWrites；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ConfigureWrites(window platform.WriteWindowConfig) error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ReadSnapshot

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 ReadSnapshot；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ReadSnapshot(ctx context.Context, fn func(*Store,*sql.Tx) error) error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.Write

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 Write；精确参数与返回见签名，调用边界随所属用例固定。

```go
func Write(ctx context.Context,op platform.OperationContext,functionID,key string,input,result any,fn func(*Store,*sql.Tx) error) error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.RequireManager

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 RequireManager；精确参数与返回见签名，调用边界随所属用例固定。

```go
func RequireManager(op platform.OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.DBError

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 DBError；精确参数与返回见签名，调用边界随所属用例固定。

```go
func DBError(err error) error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ValidateTask

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 ValidateTask；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ValidateTask(task string) error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ReadEntryObject

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 ReadEntryObject；精确参数与返回见签名，调用边界随所属用例固定。

```go
func (s *Store) ReadEntryObject(ctx context.Context,q Queryer,siteID,id string,version *int)(gen.MemoryEntry,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`、`memory.decodeEntry`

## memory.decodeEntry

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 decodeEntry；精确参数与返回见签名，调用边界随所属用例固定。

```go
func decodeEntry(raw []byte)(gen.MemoryEntry,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.SaveEntryObject

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 SaveEntryObject；精确参数与返回见签名，调用边界随所属用例固定。

```go
func (s *Store) SaveEntryObject(ctx context.Context,q Queryer,site string,entry gen.MemoryEntry)(gen.MemoryEntry,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.Write`、`memory.DBError`

## memory.InsertEntryVersion

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 InsertEntryVersion；精确参数与返回见签名，调用边界随所属用例固定。

```go
func InsertEntryVersion(ctx context.Context,tx *sql.Tx,op platform.OperationContext,e gen.MemoryEntry,previous *int)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.Emit

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 Emit；精确参数与返回见签名，调用边界随所属用例固定。

```go
func Emit(tx *sql.Tx,op platform.OperationContext,eventType,subject string,revision int,payload any)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ReadAllEntries

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 ReadAllEntries；精确参数与返回见签名，调用边界随所属用例固定。

```go
func (s *Store) ReadAllEntries(ctx context.Context,q Queryer,site string)([]gen.MemoryEntry,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`、`memory.ReadEntryObject`

## memory.NormalizeLineEndings

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 NormalizeLineEndings；精确参数与返回见签名，调用边界随所属用例固定。

```go
func NormalizeLineEndings(s string)string
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.SaveContextManifest

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 SaveContextManifest；精确参数与返回见签名，调用边界随所属用例固定。

```go
func SaveContextManifest(ctx context.Context,op platform.OperationContext,id,task,language string,revision int,versionHash,bodyHash string,size int,manifest any)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.ConfigurePublisher

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 ConfigurePublisher；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ConfigurePublisher(publisher platform.EventPublisher)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.RelayBatch

- 级别：参考
- 文件：`modules/memory/internal/runtime/business.go`
- 作用：本域参考级辅助 RelayBatch；精确参数与返回见签名，调用边界随所属用例固定。

```go
func RelayBatch(ctx context.Context,limit int)(platform.RelayBatchResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ConfigureAssistantClients

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 ConfigureAssistantClients；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ConfigureAssistantClients(jobs *gen.JobsClient,gateway *gen.ModelGatewayClient)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.SelectAssistantProfile

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 SelectAssistantProfile；精确参数与返回见签名，调用边界随所属用例固定。

```go
func SelectAssistantProfile(ctx context.Context, op OperationContext)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.ValidateForeign`

## memory.SaveAssistantInput

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 SaveAssistantInput；精确参数与返回见签名，调用边界随所属用例固定。

```go
func SaveAssistantInput(ctx context.Context,tx *sql.Tx, op OperationContext,snapshot AssistantSnapshot)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.ReadAssistantInput

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 ReadAssistantInput；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ReadAssistantInput(ctx context.Context,q Queryer,runID string)(AssistantInputRecord,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.DispatchAssistantInputs

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 DispatchAssistantInputs；精确参数与返回见签名，调用边界随所属用例固定。

```go
func DispatchAssistantInputs(ctx context.Context,limit int)(int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.Write`、`memory.DBError`、`memory.ReadAssistantInput`、`memory.ValidateForeign`

## memory.AssistantInput

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 AssistantInput；精确参数与返回见签名，调用边界随所属用例固定。

```go
func AssistantInput(ctx context.Context,id string)(AssistantInputRecord,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.ReadAssistantInput`

## memory.ReplayReceipt

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 ReplayReceipt；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ReplayReceipt(ctx context.Context, op OperationContext,functionID,key string,input,result any)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.ValidateForeign

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 ValidateForeign；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ValidateForeign(name string,raw json.RawMessage)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ConfigureValidators

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 ConfigureValidators；精确参数与返回见签名，调用边界随所属用例固定。

```go
func ConfigureValidators(validators map[string]func(json.RawMessage)error)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.CurrentAssistantState

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 CurrentAssistantState；精确参数与返回见签名，调用边界随所属用例固定。

```go
func CurrentAssistantState(ctx context.Context,site,run string)(string,int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.DBError`

## memory.DispatchAssistantCancellations

- 级别：参考
- 文件：`modules/memory/internal/runtime/tasks.go`
- 作用：本域参考级辅助 DispatchAssistantCancellations；精确参数与返回见签名，调用边界随所属用例固定。

```go
func DispatchAssistantCancellations(ctx context.Context,limit int)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.Write`、`memory.DBError`、`memory.ReadAssistantInput`、`memory.ValidateForeign`

## memory.operationHandlers

- 级别：参考
- 文件：`modules/memory/internal/transport/business.go`
- 作用：本域参考级辅助 operationHandlers；精确参数与返回见签名，调用边界随所属用例固定。

```ts
func(a *Application)operationHandlers()map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.Health`、`memory.Ready`、`memory.jsonOperation`、`memory.respond`、`memory.matchPath`、`memory.optionalString`、`memory.optionalInt`、`memory.queryLimit`、`memory.CreateEntry`、`memory.UpdateEntry`、`memory.ArchiveEntry`、`memory.RestoreEntry`、`memory.ReadEntry`、`memory.ListEntries`、`memory.ListEntryRevisions`、`memory.PutMapping`、`memory.RemoveMapping`、`memory.ReorderMappings`、`memory.ListMappings`、`memory.PutTaskBudget`、`memory.ListTaskBudgets`、`memory.ResolveContext`、`memory.ReadContextManifest`、`memory.CreateAssistantRun`、`memory.ReadAssistantRun`、`memory.ApproveProposal`、`memory.RejectProposal`、`memory.CancelAssistantRun`、`memory.AppendAssistantMessage`、`memory.ReadAssistantConversation`、`memory.DeleteAssistantConversation`

## memory.jsonOperation

- 级别：参考
- 文件：`modules/memory/internal/transport/business.go`
- 作用：本域参考级辅助 jsonOperation；精确参数与返回见签名，调用边界随所属用例固定。

```go
func jsonOperation[I,R any](fn func(context.Context,platform.OperationContext,I)(R,error),status int,bind ...func(*I,*http.Request)error)http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：`memory.respond`

## memory.respond

- 级别：参考
- 文件：`modules/memory/internal/transport/business.go`
- 作用：本域参考级辅助 respond；精确参数与返回见签名，调用边界随所属用例固定。

```go
func respond(w http.ResponseWriter,r *http.Request,result any,err error,status int)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.matchPath

- 级别：参考
- 文件：`modules/memory/internal/transport/business.go`
- 作用：本域参考级辅助 matchPath；精确参数与返回见签名，调用边界随所属用例固定。

```go
func matchPath(body,path string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.optionalString

- 级别：参考
- 文件：`modules/memory/internal/transport/business.go`
- 作用：本域参考级辅助 optionalString；精确参数与返回见签名，调用边界随所属用例固定。

```go
func optionalString(r *http.Request,name string)*string
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.optionalInt

- 级别：参考
- 文件：`modules/memory/internal/transport/business.go`
- 作用：本域参考级辅助 optionalInt；精确参数与返回见签名，调用边界随所属用例固定。

```go
func optionalInt(r *http.Request,name string)*int
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.queryLimit

- 级别：参考
- 文件：`modules/memory/internal/transport/business.go`
- 作用：本域参考级辅助 queryLimit；精确参数与返回见签名，调用边界随所属用例固定。

```go
func queryLimit(r *http.Request)int
```

**输入**

无

**输出**

无

**步骤**

1. 按调用方已验证输入执行本域私有存储、纯规则或协议装配；保留错误，不输出正文或凭据。

**错误**

- `INTERNAL`：本域私有IO失败（不可重试）
- `VALIDATION`：登记的边界不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：受信运行依赖未装配（可重试）
- `INTEGRITY_ERROR`：持久摘要或引用不一致（不可重试）

- 副作用：无
- 授权：内部调用信任入口授权；管理与模型权限不得由浏览器覆盖。
- 并发：复用调用方事务、固定快照及平台取消上下文；未提交失败回滚。
- 幂等：按所属用例持久回执或不可变引用执行，不创建独立收费操作。
- 依赖：无

## memory.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/memory/internal/snapshot_archive.go`
- 作用：受控读取本域已完成且精确引用匹配的快照归档流

```go
func ReadSnapshotArchive(ctx context.Context, op OperationContext, input SnapshotArchiveReadInput) (SnapshotArchiveReadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 流取消与既有截止 | 透传IO |
| `op` | `OperationContext` | 统一入口可信上下文 | data-protection服务及superadmin委托 |
| `input` | `SnapshotArchiveReadInput` | 精确完成归档引用 | 严格生产方schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotArchiveReadResult` | 只读完成tar流 | 不暴露磁盘路径 |
| `error` | `error` | 未知或不匹配拒绝 | 不含秘密或原路径 |

**步骤**

1. 仅接受data-protection服务及已验证superadmin委托，当前维护backup/restore屏障与本域已完成归档绑定；不把匿名下载或任意URL转为文件读。
2. 从本域固定owner/version=1/mediaType=application/x-tar与snapshotId/sha256/byteSize构造期望ObjectRef，委托公共SnapshotManager.OpenArchive核真实complete记录和完整hash后输出。
3. 只有完整只读流，不写源数据、不删除历史归档；取消透传下游IO。

**错误**

- `FORBIDDEN`：调用方不是data-protection或缺superadmin委托（不可重试）
- `LOCKED`：runtime阶段或restoreId不符，或本域屏障未safe（不可重试）
- `INTEGRITY_ERROR`：归档hash、owner或manifest不一致；临时目录已删除（不可重试）
- `UNSUPPORTED_SCHEMA`：快照schema高于本版本可迁移范围（不可重试）
- `CONFLICT`：同restoreId已暂存不同checksum（不可重试）
- `DEPENDENCY_UNAVAILABLE`：拉取归档失败或磁盘空间不足；临时目录已删除（可重试）

- 副作用：读取本域已完成归档字节
- 授权：permission：superadmin；rule：仅data-protection服务身份加已验证superadmin委托；绑定本域精确完成归档与维护屏障
- 并发：只读流，接收方仍核完整ObjectRef/hash/EOF
- 幂等：同完整引用只读复用，不生成或重算归档
- 依赖：`platform.SnapshotManager.OpenArchive`

## memory.MemoryPage

- 级别：约束
- 文件：`modules/memory/web/src/index.tsx`
- 作用：长期记忆：记忆条目的新建、编辑、归档与恢复

```ts
export default function MemoryPage(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 按状态列出条目
2. 保存时计算正文哈希与字节数，超出上限不提交

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无
