# qa 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 91 个函数：约束 28 个，参考 63 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## qa.LoadRuntime

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：启动问答服务

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

1. 载入schema/policy
2. 检查SSE事件序号索引
3. 恢复取消与终态
4. 不恢复派发未知MG调用
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

- 副作用：打开本服务连接池
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## qa.NewRuntime

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：装配私有数据库、所有真实生产方严格响应校验器与公共模型网关适配

```go
func NewRuntime(runtime Runtime)(Runtime,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`platform.NewGatewayClient`、`platform.NewPayloadValidator`、`model-gateway:readInference`、`model-gateway:lookupInference`

## qa.WithRuntime

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：WithRuntime的本域内部实现助手

```go
func WithRuntime(ctx context.Context,runtime Runtime)context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.FromContext

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：FromContext的本域内部实现助手

```go
func FromContext(ctx context.Context)(Runtime,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.DecodeResponse

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：DecodeResponse的本域内部实现助手

```go
func DecodeResponse(runtime Runtime,name string,raw json.RawMessage,target any)error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.StorageError

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：StorageError的本域内部实现助手

```go
func StorageError(err error)error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.RequireAdmin

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：RequireAdmin的本域内部实现助手

```go
func RequireAdmin(op OperationContext,permission string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.Transaction.Close

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：回滚未提交事务并释放平台写窗口，允许defer安全关闭

```ts
func(t *Transaction)Close()
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.Transaction.Commit

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：提交本域短事务并转换脱敏存储错误

```ts
func(t *Transaction)Commit()error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.StorageError`

## qa.BeginRead

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：BeginRead的本域内部实现助手

```go
func BeginRead(ctx context.Context,op OperationContext)(*Transaction,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.beginTransaction`

## qa.BeginWrite

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：BeginWrite的本域内部实现助手

```go
func BeginWrite(ctx context.Context,op OperationContext)(*Transaction,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.beginTransaction`

## qa.beginTransaction

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：通过平台写窗口或只读事务固定本域站点/代次，返回有界事务

```go
func beginTransaction(ctx context.Context,op OperationContext,write bool)(*Transaction,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.FromContext`、`qa.StorageError`、`platform.NewWriteContext`

## qa.ReplayReceipt

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：按站点、代次、主体、函数及幂等键返回七天内同输入的原回执，异输入冲突

```go
func ReplayReceipt(t *Transaction,op OperationContext,function,key string,input,result any)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.StorageError`

## qa.SaveReceipt

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：在同一业务事务保存输入摘要与完整原结果及其摘要

```go
func SaveReceipt(t *Transaction,op OperationContext,function,key string,input,result any)error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.StorageError`

## qa.ReadPolicy

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：ReadPolicy的本域内部实现助手

```go
func ReadPolicy(t *Transaction)(gen.TypesQaPolicy,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.StorageError`、`qa.DecodeResponse`

## qa.ReadConversation

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：ReadConversation的本域内部实现助手

```go
func ReadConversation(t *Transaction,op OperationContext,id string,owner bool)(gen.Conversation,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.StorageError`

## qa.ReadTurn

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：读取限定本站代次的轮次，按需核owner并校验持久状态schema

```go
func ReadTurn(t *Transaction,op OperationContext,id string,owner bool)(TurnRecord,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.StorageError`、`qa.DecodeResponse`

## qa.IsTerminal

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：IsTerminal的本域内部实现助手

```go
func IsTerminal(state string)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.CheckTurnLease

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：CheckTurnLease的本域内部实现助手

```go
func CheckTurnLease(record TurnRecord,lease string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.IsTerminal`

## qa.SaveTurn

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：在调用方同一短事务保存轮次状态、答案、上下文、引用和费用调用引用

```go
func SaveTurn(t *Transaction,record TurnRecord)error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.StorageError`

## qa.ServiceReadContext

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：为已授权公开权威读取装配qa纯服务主体并清除旧用户proof，保留可信站点/根/截止

```go
func ServiceReadContext(ctx context.Context,op OperationContext)context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.ReadPublishedDocument

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：经content版本化API读取精确发布正文，验证整体摘要并提取稳定块摘要

```go
func ReadPublishedDocument(ctx context.Context,op OperationContext,ref gen.PublishedRevisionRef)(PublishedDocument,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.FromContext`、`qa.DecodeResponse`、`qa.ServiceReadContext`、`content:readPublishedRevision`

## qa.ValidateCitations

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：核对全部引用的发布来源和块hash，并经公开服务解析当前语言精确href

```go
func ValidateCitations(ctx context.Context,op OperationContext,citations []gen.Citation,language string)([]gen.Citation,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.ReadPublishedDocument`、`public-site:resolvePublishedLink`

## qa.ResolveTaskMemory

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：读取单独task和语言的权威记忆，复核渲染text的UTF8字节数及5120B预算，空匹配合法

```go
func ResolveTaskMemory(ctx context.Context,op OperationContext,task,language string)(MemoryContext,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理可信输入和已装配依赖
2. 详细边界和固定算法见data/workflows及源码注释

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
- 依赖：`qa.FromContext`、`qa.DecodeResponse`、`qa.ServiceReadContext`、`memory:resolveContext`

## qa.CursorActor

- 级别：参考
- 文件：`modules/qa/internal/runtime/bootstrap.go`
- 作用：生成稳定授权游标身份，排除每次委托变化的expiresAt

```go
func CursorActor(op OperationContext)any
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理可信输入和已装配依赖
2. 详细边界和固定算法见data/workflows及源码注释

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

## qa.PutQaPolicy

- 级别：约束
- 文件：`modules/qa/internal/config/policy.go`
- 作用：管理公开问答策略

```go
func PutQaPolicy(ctx context.Context, op OperationContext, input PutQaPolicyInput) (PutQaPolicyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutQaPolicyInput` | 封闭业务输入；逐字段见types $defs/PutQaPolicyInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutQaPolicyResult` | 成功结果；逐字段见types $defs/PutQaPolicyResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 限制各token预算
2. embedding space与MG/knowledge匹配
3. semantic阈值≥0.94
4. CAS更新并失效scope；recordRetentionDays变化时进行中会话的过期时间按差值同步缩短或顺延
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

- 副作用：写policy+缓存失效事件
- 授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway:readProfiles`

## qa.CreateConversation

- 级别：约束
- 文件：`modules/qa/internal/conversation/create.go`
- 作用：创建伪名访客会话

```go
func CreateConversation(ctx context.Context, op OperationContext, input CreateConversationInput) (CreateConversationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateConversationInput` | 封闭业务输入；逐字段见types $defs/CreateConversationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateConversationResult` | 成功结果；逐字段见types $defs/CreateConversationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅portal委托的原伪名访客创建；免费qa.conversation元数据绑定原RootOperationID、subject、site/epoch且token为空
2. 首次创建由ValidateAdmission消费并verify已消费的免费元数据；免费消费后仍无模型token，不调用模型
3. 事务生成会话ID、按当前问答策略recordRetentionDays计算的过期时间与同visitor/key原回执，不保存IP
4. 本地提交或原回执读取结束后，按原root/owner/site/epoch调用guard releaseAdmission空RequestBody释放占位；重放幂等补释放，不占首问并发位
5. 释放未知或Released=false返回依赖失败但保留原会话和回执；同输入重试不再次消费或创建

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）

- 副作用：写conversation
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`traffic-guard:verifyAdmission`、`qa.BeginWrite`、`qa.ReplayReceipt`、`qa.SaveReceipt`、`qa.ValidateAdmission`、`qa.Transaction.Close`、`qa.Transaction.Commit`、`qa.StorageError`、`qa.FromContext`、`qa.DecodeResponse`、`qa.ServiceReadContext`、`traffic-guard:releaseAdmission`

## qa.GetConversation

- 级别：约束
- 文件：`modules/qa/internal/conversation/read.go`
- 作用：读取自有访客历史

```go
func GetConversation(ctx context.Context, op OperationContext, input GetConversationInput) (GetConversationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `GetConversationInput` | 封闭业务输入；逐字段见types $defs/GetConversationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetConversationResult` | 成功结果；逐字段见types $defs/GetConversationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验ctx.actor.subjectId与visitorSubject
2. 过期返回410
3. ordinal分页
4. 只返回本会话消息
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）

- 副作用：无
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginRead`、`qa.ReadConversation`、`qa.Transaction.Close`、`qa.StorageError`

## qa.StartTurn

- 级别：约束
- 文件：`modules/qa/internal/turn/start.go`
- 作用：准入后开始问答轮次

```go
func StartTurn(ctx context.Context, op OperationContext, input StartTurnInput) (StartTurnResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `StartTurnInput` | 封闭业务输入；逐字段见types $defs/StartTurnInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StartTurnResult` | 成功结果；逐字段见types $defs/StartTurnResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 会话owner校验，ValidateAdmission核验并消费portal附带的准入许可
2. question≤500Unicode字符并固定可信qa上下文
3. 同会话最多1活跃轮次：空闲时创建qa.turn任务；忙时调用traffic-guard:queueSessionAdmission登记排队许可，持久排队最多5项
4. 写原问题/当前页hint/queued状态，前置仅收权威校验提示

**错误**

- `RATE_LIMITED` / HTTP 429：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写question与任务；发模型前固定可信costClass=qa
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:createJob`、`qa.ValidateAdmission`、`traffic-guard:queueSessionAdmission`、`identity:readSiteAiPolicy`、`identity:noteVisitorQaCap`

## qa.ValidateAdmission

- 级别：参考
- 文件：`modules/qa/internal/admission/permit.go`
- 作用：检查准入绑定

```go
func ValidateAdmission(ctx context.Context, op OperationContext, input ValidateAdmissionInput) (ValidateAdmissionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateAdmissionInput` | 封闭业务输入；逐字段见types $defs/ValidateAdmissionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateAdmissionResult` | 成功结果；逐字段见types $defs/ValidateAdmissionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅接受qa.conversation、qa.ask、qa.followup动作；核原subject、稳定RootOperationID、本站epoch、audience及有效expiry
2. qa纯服务调用consumeAdmission，body.requestId为原RootOperationID；消费去重、桶和签名归guard
3. qa.conversation输入及消费回执token必须为空；qa.ask/followup消费回执必须非空签名token；两者都需Consumed=true及有效expiry
4. 其他不可变元数据与原分配一致，再在线verify已消费状态与原root，不改原subject为服务ID
5. 返回原消费回执；仅付费签名许可向knowledge/MG传播，免费创建由用例提交后释放占位

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）

- 副作用：消费guard permit；不建费用预留
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`traffic-guard:consumeAdmission`、`qa.FromContext`、`qa.DecodeResponse`、`traffic-guard:verifyAdmission`

## qa.NormalizeQuestion

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：稳定规范问题和cache锚点

```go
func NormalizeQuestion(ctx context.Context, op OperationContext, input NormalizeQuestionInput) (NormalizeQuestionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `NormalizeQuestionInput` | 封闭业务输入；逐字段见types $defs/NormalizeQuestionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizeQuestionResult` | 成功结果；逐字段见types $defs/NormalizeQuestionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. NFKC/LF/全半角空白规范化
2. 自然语言大小写折叠但代码/产品ID/数字/否定词保留
3. ≤500Unicode字符且≤8192bytes
4. stable算法提取锚点并输出hash。
5. 本函数是纯计算，信任入口与ctx，不解析通用deadline、不检查ctx取消；不产生IO或费用

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
- 依赖：`qa.validInputText`、`qa.normalizeWhitespace`、`qa.foldNaturalCase`、`qa.extractAnchors`

## qa.validInputText

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：validInputText的本域内部实现助手

```go
func validInputText(value string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入与可信依赖
2. 分支规则与固定参数见本域data/workflows与源码注释

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

## qa.normalizeWhitespace

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：normalizeWhitespace的本域内部实现助手

```go
func normalizeWhitespace(value string) string
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入与可信依赖
2. 分支规则与固定参数见本域data/workflows与源码注释

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

## qa.foldNaturalCase

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：foldNaturalCase的本域内部实现助手

```go
func foldNaturalCase(value string) string
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入与可信依赖
2. 分支规则与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.isProtectedToken`

## qa.isProtectedToken

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：isProtectedToken的本域内部实现助手

```go
func isProtectedToken(token string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入与可信依赖
2. 分支规则与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.hasSignificantUppercase`

## qa.hasSignificantUppercase

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：hasSignificantUppercase的本域内部实现助手

```go
func hasSignificantUppercase(value string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入与可信依赖
2. 分支规则与固定参数见本域data/workflows与源码注释

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

## qa.extractAnchors

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：extractAnchors的本域内部实现助手

```go
func extractAnchors(value string) []string
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入与可信依赖
2. 分支规则与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.isAnchorToken`

## qa.isAnchorToken

- 级别：参考
- 文件：`modules/qa/internal/normalize/question.go`
- 作用：isAnchorToken的本域内部实现助手

```go
func isAnchorToken(token string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入与可信依赖
2. 分支规则与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.hasSignificantUppercase`

## qa.ResolveTurnContext

- 级别：参考
- 文件：`modules/qa/internal/turn/context.go`
- 作用：固定回答上下文版本

```go
func ResolveTurnContext(ctx context.Context, op OperationContext, input ResolveTurnContextInput) (ResolveTurnContextResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ResolveTurnContextInput` | 封闭业务输入；逐字段见types $defs/ResolveTurnContextInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveTurnContextResult` | 成功结果；逐字段见types $defs/ResolveTurnContextResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 读取当前policy/dataEpoch
2. memory resolveContext taskId=qa.answer
3. 固定mapping/versionHash
4. canonical hash形成本轮上下文基线
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 输入显式携带本轮leaseId；锁外读取记忆后，在平台当前窗口和相同租约/策略版本下固定Turn.contextHash与独立memoryHash，已有不同上下文拒绝覆盖

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：写turn context摘要不记录memory正文到日志
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`memory:resolveContext`、`site-config:readRuntimeState`、`qa.BeginRead`、`qa.ReadTurn`、`qa.ReadConversation`、`qa.ReadPolicy`、`qa.CheckTurnLease`、`qa.ResolveTaskMemory`、`qa.BeginWrite`、`qa.SaveTurn`、`qa.Transaction.Close`、`qa.Transaction.Commit`

## qa.RetrieveEvidence

- 级别：参考
- 文件：`modules/qa/internal/answer/retrieve.go`
- 作用：取有源可见证据

```go
func RetrieveEvidence(ctx context.Context, op OperationContext, input RetrieveEvidenceInput) (RetrieveEvidenceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RetrieveEvidenceInput` | 封闭业务输入；逐字段见types $defs/RetrieveEvidenceInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RetrieveEvidenceResult` | 成功结果；逐字段见types $defs/RetrieveEvidenceResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. knowledge searchKnowledge按固定space
2. 最多8块/4096token
3. 对IDs调用readSections
4. 映射可公开Citation且保留ref
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 复用调用链已有embedding，否则仅AcquireQueryEmbedding一次；空候选且truncated时BUDGET_EXCEEDED，不把预算不足说成文档没提及；完整读取chunk、偏移和hash后逐引用解析当前语言href

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：写turn证据快照hash，不拥有正文权威
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge:searchKnowledge`、`knowledge:readSections`、`qa.FromContext`、`qa.ServiceReadContext`、`qa.DecodeResponse`、`qa.ReadPublishedDocument`、`qa.ValidateCitations`、`knowledge:readDocumentIndex`、`localization:readTranslation`、`knowledge:readDocumentChunks`、`knowledge:readSiteOverview`、`qa.AcquireQueryEmbedding`

## qa.SearchDocuments

- 级别：约束
- 文件：`modules/qa/internal/answer/search.go`
- 作用：访客全文搜索（关键词与向量混合检索，不重排）

```go
func SearchDocuments(ctx context.Context, op OperationContext, input SearchDocumentsInput) (SearchDocumentsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SearchDocumentsInput` | 封闭业务输入；逐字段见types $defs/SearchDocumentsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SearchDocumentsResult` | 成功结果；逐字段见types $defs/SearchDocumentsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 问答已开启且query≤200Unicode字符，否则释放portal附带的准入并拒绝
2. ValidateAdmission核验并消费qa.ask准入，本次检索的向量与检索调用都带这份已签名许可
3. 复用问答查询向量缓存，在知识索引上做关键词+向量融合检索，跳过重排模型
4. 逐段核对公开可见性并生成访客语言的公开链接，返回标题、链接和节选；结束时释放准入并发占位

**错误**

- `RATE_LIMITED` / HTTP 429：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：消费并释放一次qa.ask准入；可能生成并缓存查询向量（发模型前固定可信costClass=qa）；不写问答会话
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：只读固定快照；截止=min(ctx deadline,op.deadline,本函数15000ms)，每一外部调用前检查取消。
- 幂等：纯检索，可重复；准入按portal根操作复用，查询向量按规范查询哈希复用。
- 依赖：`jobs:createJob`、`qa.ValidateAdmission`、`traffic-guard:queueSessionAdmission`

## qa.BuildAnswerPrompt

- 级别：参考
- 文件：`modules/qa/internal/answer/prompt.go`
- 作用：构造有界回答提示

```go
func BuildAnswerPrompt(ctx context.Context, op OperationContext, input BuildAnswerPromptInput) (BuildAnswerPromptResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BuildAnswerPromptInput` | 封闭业务输入；逐字段见types $defs/BuildAnswerPromptInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildAnswerPromptResult` | 成功结果；逐字段见types $defs/BuildAnswerPromptResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 固定system前缀先安全规则再任务说明；站点记忆、概括和导航编码为独立JSON资料，不能覆盖规则
2. summary/最近对话/当前页验证片段/检索材料分别标数据边界
3. 访客语言回答、优先对应块译文且引用当前语言href
4. 无证据明确文档未提及，默认最大2000输出tokens。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 本函数只组装调用链已按实际模型窗口裁剪的输入；访客原话、摘要、背景和正文使用数据边界，不把访客JSON或角色标签解释成系统消息。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：无副作用；不将未授权材料发送MG
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## qa.GenerateAnswer

- 级别：参考
- 文件：`modules/qa/internal/answer/generate.go`
- 作用：执行流式答案调用

```go
func GenerateAnswer(ctx context.Context, op OperationContext, input GenerateAnswerInput) (GenerateAnswerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `GenerateAnswerInput` | 封闭业务输入；逐字段见types $defs/GenerateAnswerInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GenerateAnswerResult` | 成功结果；逐字段见types $defs/GenerateAnswerResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验lease/cancel/deadline
2. ctx继承root/costClass=qa与已消费准入许可
3. qa.InvokeModel以stream=true调用qa.answer任务
4. 每个delta经AppendAnswerEvent持久并推送访客；记录invocationId，不重放未知请求

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG产生费用；写生成状态及事件
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.InvokeModel`、`qa.AppendAnswerEvent`

## qa.ValidateCitation

- 级别：参考
- 文件：`modules/qa/internal/answer/citation.go`
- 作用：逐条保护引用

```go
func ValidateCitation(ctx context.Context, op OperationContext, input ValidateCitationInput) (ValidateCitationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateCitationInput` | 封闭业务输入；逐字段见types $defs/ValidateCitationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateCitationResult` | 成功结果；逐字段见types $defs/ValidateCitationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. citationId必须在白名单
2. content当前visible/matchesCurrent
3. href由public route规则生成
4. 缺失返回STALE_RESULT并停止该证据回答
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：只读；不可创造源外引用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`qa.ValidateCitations`

## qa.AppendAnswerEvent

- 级别：参考
- 文件：`modules/qa/internal/stream/store.go`
- 作用：持久流式输出事件

```go
func AppendAnswerEvent(ctx context.Context, op OperationContext, input AppendAnswerEventInput) (AppendAnswerEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `AppendAnswerEventInput` | 封闭业务输入；逐字段见types $defs/AppendAnswerEventInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AppendAnswerEventResult` | 成功结果；逐字段见types $defs/AppendAnswerEventResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 租约/cancel状态检查
2. delta≤16KiB且UTF8完整
3. 事务递增序号和事件
4. 提交后广播
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 可信调用链可给sourceStep/sourceSequence，按turnId+sourceStep+sourceSequence唯一；同源同内容返回原序号，异内容INTEGRITY_ERROR；新事件序号仅本事务生成
7. 本函数仅接受accepted/progress/step/delta/citation/invalidate；completed/failed/error/cancelled必须由终态用例与Turn状态同事务写入

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：turn_events写入；严格有序
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginWrite`、`qa.ReadTurn`、`qa.CheckTurnLease`、`qa.SaveEvent`、`qa.Transaction.Close`、`qa.Transaction.Commit`、`qa.StorageError`

## qa.SaveEvent

- 级别：参考
- 文件：`modules/qa/internal/stream/store.go`
- 作用：SaveEvent的本域内部实现助手

```go
func SaveEvent(tx *qaruntime.Transaction,record *qaruntime.TurnRecord,event gen.AnswerEvent,source string,sequence *int64)error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.SaveTurn`、`qa.StorageError`

## qa.ResumeAnswerStream

- 级别：约束
- 文件：`modules/qa/internal/stream/read.go`
- 作用：SSE断线重连

```go
func ResumeAnswerStream(ctx context.Context, op OperationContext, input ResumeAnswerStreamInput) (ResumeAnswerStreamResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ResumeAnswerStreamInput` | 封闭业务输入；逐字段见types $defs/ResumeAnswerStreamInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResumeAnswerStreamResult` | 成功结果；逐字段见types $defs/ResumeAnswerStreamResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 会话owner授权
2. 读cursor后事件
3. 每15秒heartbeat无持久序号
4. 超24小时保留期返回410
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）

- 副作用：只读，不重新生成答案
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginRead`、`qa.ReadTurn`、`qa.IsTerminal`、`qa.StorageError`、`qa.Transaction.Close`

## qa.CompleteTurn

- 级别：参考
- 文件：`modules/qa/internal/turn/complete.go`
- 作用：原子结束可见答案

```go
func CompleteTurn(ctx context.Context, op OperationContext, input CompleteTurnInput) (CompleteTurnResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CompleteTurnInput` | 封闭业务输入；逐字段见types $defs/CompleteTurnInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CompleteTurnResult` | 成功结果；逐字段见types $defs/CompleteTurnResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 原稳定root绑定且有效未取消lease；终态提交前通过content/public-site重新核对全部引用和真实语言href
2. 同一事务插入assistant消息及真实tokenCount、answerRevision、answerKind、completed终态事件和qa.turn.completed outbox，不改变用户原消息
3. 保存固定completion input hash（排除lease）；completed重放只读取原消息，异答案/来源/类型/tokenCount拒绝，不重复追加
4. 提交后幂等释放guard slot；失败保留已完成状态，原结果重放继续释放，不重做模型费用

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）

- 副作用：保存消息与终态事件；不自动批准cache
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginRead`、`qa.BeginWrite`、`qa.ReadTurn`、`qa.ReadConversation`、`qa.CheckTurnLease`、`qa.ValidateCitations`、`qa.SaveEvent`、`qa.replayCompletedTurn`、`qa.releaseTurnPermit`、`platform.EnqueueEvent`

## qa.replayCompletedTurn

- 级别：参考
- 文件：`modules/qa/internal/turn/complete.go`
- 作用：核完成输入hash并读取原assistant消息后补做准入释放

```go
func replayCompletedTurn(ctx context.Context,op OperationContext,record qaruntime.TurnRecord,input CompleteTurnInput)(CompleteTurnResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按调用者持有的可信上下文和事务执行本域职责
2. 精确规则见data/workflows和源码注释

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
- 依赖：`qa.BeginRead`、`qa.releaseTurnPermit`

## qa.releaseTurnPermit

- 级别：参考
- 文件：`modules/qa/internal/turn/complete.go`
- 作用：继承原轮次root，调用guard既有空RequestBody释放原准入

```go
func releaseTurnPermit(ctx context.Context,record qaruntime.TurnRecord)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用者持有的可信上下文和事务执行本域职责
2. 精确规则见data/workflows和源码注释

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
- 依赖：`qa.FromContext`、`qa.ServiceReadContext`、`qa.DecodeResponse`、`traffic-guard:releaseAdmission`

## qa.CancelTurn

- 级别：约束
- 文件：`modules/qa/internal/turn/cancel.go`
- 作用：取消问答及所有子调用

```go
func CancelTurn(ctx context.Context, op OperationContext, input CancelTurnInput) (CancelTurnResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CancelTurnInput` | 封闭业务输入；逐字段见types $defs/CancelTurnInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CancelTurnResult` | 成功结果；逐字段见types $defs/CancelTurnResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. owner校验，按原输入幂等回执；未终态时在事务内写cancelRequested及唯一cancelled事件，终态保持原结果
2. 提交后按原轮次root/subject/epoch调用jobs取消及平台Gateway取消全部持久invocation IDs，不认为取消可免账单
3. 按原session scope/root调用guard queue mode=cancel仅删排队引用，随后ReleaseAdmission释放原slot
4. 幂等回放仍补做取消依赖与释放；下游未知失败不报告清理完成，不重复终态事件

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）

- 副作用：停止新delta，取消子任务不自行释放费用
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginWrite`、`qa.ReadTurn`、`qa.ReplayReceipt`、`qa.SaveReceipt`、`qa.SaveEvent`、`qa.cancelTurnDependencies`

## qa.cancelTurnDependencies

- 级别：参考
- 文件：`modules/qa/internal/turn/cancel.go`
- 作用：按原root幂等取消任务和模型，清排队后释放并发占位

```go
func cancelTurnDependencies(ctx context.Context,record qaruntime.TurnRecord)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用者持有的可信上下文和事务执行本域职责
2. 精确规则见data/workflows和源码注释

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
- 依赖：`qa.FromContext`、`qa.ServiceReadContext`、`qa.DecodeResponse`、`qa.releaseTurnPermit`、`jobs:cancelJob`、`model-gateway:cancelInference`、`traffic-guard:queueSessionAdmission`

## qa.RecordFeedback

- 级别：约束
- 文件：`modules/qa/internal/feedback/feedback.go`
- 作用：保存访客单条反馈

```go
func RecordFeedback(ctx context.Context, op OperationContext, input RecordFeedbackInput) (RecordFeedbackResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RecordFeedbackInput` | 封闭业务输入；逐字段见types $defs/RecordFeedbackInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecordFeedbackResult` | 成功结果；逐字段见types $defs/RecordFeedbackResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验访客对已完成答案版本的权限
2. 保存反馈并发布不含正文的事件；不写答案缓存

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）

- 副作用：写feedback及聚合事件
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginWrite`、`qa.ReplayReceipt`、`qa.ReadTurn`、`qa.SaveReceipt`、`qa.StorageError`、`qa.Transaction.Close`、`qa.Transaction.Commit`、`platform.EnqueueEvent`

## qa.CreateCompaction

- 级别：参考
- 文件：`modules/qa/internal/history/compaction.go`
- 作用：申请有界历史摘要

```go
func CreateCompaction(ctx context.Context, op OperationContext, input CreateCompactionInput) (CreateCompactionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateCompactionInput` | 封闭业务输入；逐字段见types $defs/CreateCompactionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateCompactionResult` | 成功结果；逐字段见types $defs/CreateCompactionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 取preflight与answer模型较小context窗口
2. 总输入达到90%阈值才自动压缩
3. 原样保留最近15000token，小窗口retain=min(15000,可用历史预算)
4. 固定前summary与未压缩message范围，唯一conversation/sourceGeneration排队。
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

- 副作用：写compaction和jobs任务
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:createJob`

## qa.RunCompaction

- 级别：约束
- 文件：`modules/qa/internal/history/compaction.go`
- 作用：执行历史压缩

```go
func RunCompaction(ctx context.Context, op OperationContext, input RunCompactionInput) (RunCompactionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RunCompactionInput` | 封闭业务输入；逐字段见types $defs/RunCompactionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RunCompactionResult` | 成功结果；逐字段见types $defs/RunCompactionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 输入previousSummary加newMessage范围增量更新，不从头重写全部历史
2. fast task qa.compact且不注入长期记忆
3. 继承本轮qa根与费用额度
4. 固定recent retained范围，取消后不提交摘要。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG费用；不删除原始消息
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.InvokeModel`、`qa.CommitCompaction`、`qa.ModelText`

## qa.CommitCompaction

- 级别：参考
- 文件：`modules/qa/internal/history/commit.go`
- 作用：保护新历史不被旧摘要覆盖

```go
func CommitCompaction(ctx context.Context, op OperationContext, input CommitCompactionInput) (CommitCompactionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CommitCompactionInput` | 封闭业务输入；逐字段见types $defs/CommitCompactionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitCompactionResult` | 成功结果；逐字段见types $defs/CommitCompactionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 比较historyGeneration
2. 不相等标superseded
3. CAS summary generation
4. 保存hash和输入范围
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 原摘要hash、连续增量消息范围、retainedFrom和源generation均不得被结果改写；原消息不删除，终态重放保原结果，过期/旧租约不覆盖

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：写派生摘要，保留原始message
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginWrite`、`qa.ReadConversation`、`qa.DecodeResponse`、`qa.StorageError`、`qa.Transaction.Close`、`qa.Transaction.Commit`

## qa.CreateDebugTurn

- 级别：约束
- 文件：`modules/qa/internal/debug/create.go`
- 作用：隔离管理诊断

```go
func CreateDebugTurn(ctx context.Context, op OperationContext, input CreateDebugTurnInput) (CreateDebugTurnResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateDebugTurnInput` | 封闭业务输入；逐字段见types $defs/CreateDebugTurnInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateDebugTurnResult` | 成功结果；逐字段见types $defs/CreateDebugTurnResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. qa.debug及model.manage委托验证
2. 专用namespace和costClass=admin-probe
3. 使用真实发布证据且禁cache
4. 7天TTL和审计
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：debug_turn写入与MG费用；不进入普通统计/精选
- 授权：权限 qa.debug；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`identity:checkAuthorization`、`qa.InvokeModel`

## qa.ReadDebugTurn

- 级别：约束
- 文件：`modules/qa/internal/debug/read.go`
- 作用：读取管理诊断

```go
func ReadDebugTurn(ctx context.Context, op OperationContext, input ReadDebugTurnInput) (ReadDebugTurnResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadDebugTurnInput` | 封闭业务输入；逐字段见types $defs/ReadDebugTurnInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadDebugTurnResult` | 成功结果；逐字段见types $defs/ReadDebugTurnResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅创建管理员或superadmin
2. 独立namespace查询
3. 过期410
4. 不查询公开cache/message表
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 严格读取独立debug_turns/debug_events；superadmin按已核model.manage专属权限识别，不读取公开会话或缓存表

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：无
- 授权：权限 qa.debug；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.RequireAdmin`、`qa.BeginRead`、`qa.DecodeResponse`、`qa.StorageError`、`qa.Transaction.Close`

## qa.DeleteConversation

- 级别：约束
- 文件：`modules/qa/internal/conversation/delete.go`
- 作用：删除自有对话数据

```go
func DeleteConversation(ctx context.Context, op OperationContext, input DeleteConversationInput) (DeleteConversationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `DeleteConversationInput` | 封闭业务输入；逐字段见types $defs/DeleteConversationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DeleteConversationResult` | 成功结果；逐字段见types $defs/DeleteConversationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 先核owner及原输入回执，逐轮调用CancelTurn并完成下游清理；不在本域SQL事务持锁期间调用外部
2. 重新核owner及无活动轮次，原子清除messages/summary/feedback/流正文/压缩/currentPage/plan与permit token
3. 轮次记redacted并保留无正文状态、root及MG账单引用；所有正文读取对已清除轮次返回GONE
4. 写删除回执，同键重放不再次清理；清理期间新活动轮次阻止删除

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）

- 副作用：会话deleted并移除个人文本
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginWrite`、`qa.ReadConversation`、`qa.ReplayReceipt`、`qa.SaveReceipt`、`qa.CancelTurn`、`qa.RedactConversation`

## qa.RecoverTurns

- 级别：参考
- 文件：`modules/qa/internal/runtime/recovery.go`
- 作用：恢复崩溃问答

```go
func RecoverTurns(ctx context.Context, op OperationContext, input RecoverTurnsInput) (RecoverTurnsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RecoverTurnsInput` | 封闭业务输入；逐字段见types $defs/RecoverTurnsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverTurnsResult` | 成功结果；逐字段见types $defs/RecoverTurnsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 读取过期lease
2. MG原invocation状态/流可续才续
3. 未派发阶段可继续
4. 已派发无法确认则failed不重调用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写turn终态与guard释放
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway:readInference`、`traffic-guard:releaseAdmission`

## qa.PublishOutbox

- 级别：约束
- 文件：`modules/qa/internal/events/outbox.go`
- 作用：发布问答脱敏事件

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

1. 领取outbox租约
2. 只发IDs/评分/耗时/状态
3. jobs持久ACK后标记
4. 三次有限重试后留待下一轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 返回platform.RunRelayBatch的本批Relayed/Pending，非全域积压，不自写租约或重试机制

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：持久事件，不广播问题正文
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:publishEvent`、`platform.RunRelay`、`qa.FromContext`、`qa.ServiceReadContext`、`platform.RunRelayBatch`、`platform.NewEventPublisher`

## qa.PruneExpiredRecords

- 级别：参考
- 文件：`modules/qa/internal/runtime/prune.go`
- 作用：清理到期记录

```go
func PruneExpiredRecords(ctx context.Context, op OperationContext, input PruneExpiredRecordsInput) (PruneExpiredRecordsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PruneExpiredRecordsInput` | 封闭业务输入；逐字段见types $defs/PruneExpiredRecordsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PruneExpiredRecordsResult` | 成功结果；逐字段见types $defs/PruneExpiredRecordsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 按权威持久expires_at清理已到期且无活动turn的会话，每类最多input.limit；清除正文与派生内容并保留无正文费用审计
2. 诊断按expires_ns删除隔离表和事件；普通SSE只删终态且terminal_ns超过24小时，最多input.limit条
3. 不删除精选/普通cache审核记录或模型费用引用；全部变更在本域写窗口事务内提交

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）

- 副作用：删除本服务过期个人数据
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`qa.BeginWrite`、`qa.RedactConversation`

## qa.RedactConversation

- 级别：参考
- 文件：`modules/qa/internal/runtime/prune.go`
- 作用：清除会话正文及派生上下文，保留无正文状态和模型账单引用

```go
func RedactConversation(tx *Transaction,id,state string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按调用者持有的可信上下文和事务执行本域职责
2. 精确规则见data/workflows和源码注释

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

## qa.ResolveCurrentPageHint

- 级别：参考
- 文件：`modules/qa/internal/context/page.go`
- 作用：验证当前页面上下文

```go
func ResolveCurrentPageHint(ctx context.Context, op OperationContext, input ResolveCurrentPageHintInput) (ResolveCurrentPageHintResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ResolveCurrentPageHintInput` | 封闭业务输入；逐字段见types $defs/ResolveCurrentPageHintInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveCurrentPageHintResult` | 成功结果；逐字段见types $defs/ResolveCurrentPageHintResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. hint不是权限证据
2. content current/visible验证
3. 读取标题及被指代小节≤2048tokens
4. 页面已撤回/旧revision丢弃hint不扩大可见范围
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
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
- 依赖：`content:validatePublications`、`content:readPublishedRevision`

## qa.InspectEvidence

- 级别：参考
- 文件：`modules/qa/internal/answer/inspect.go`
- 作用：快速检索员有限核查

```go
func InspectEvidence(ctx context.Context, op OperationContext, input InspectEvidenceInput) (InspectEvidenceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `InspectEvidenceInput` | 封闭业务输入；逐字段见types $defs/InspectEvidenceInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InspectEvidenceResult` | 成功结果；逐字段见types $defs/InspectEvidenceResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 读取本turn持久检索轮数/工具预算
2. 委托knowledge inspectEvidence而非直接调用其拥有的MG task
3. 校验返回selected IDs仍属于原证据引用
4. 原子更新轮数/tool计数，超3轮5次使用已收集材料停止。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG核查费用及可选knowledge工具调用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge:inspectEvidence`、`content:validatePublications`

## qa.RunWorker

- 级别：约束
- 文件：`modules/qa/internal/runtime/worker.go`
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
- 依赖：`jobs:claimJob`、`jobs:heartbeatJob`、`jobs:completeJob`、`jobs:failJob`、`jobs:startJob`、`jobs:checkLease`、`qa.ExecuteTurn`、`qa.RunCompaction`、`platform.RunWorkerLoop`

## qa.main

- 级别：约束
- 文件：`modules/qa/cmd/qa/main.go`
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
- 依赖：`qa.BuildApplication`、`qa.Serve`、`qa.Shutdown`、`platform.RunService`

## qa.BuildApplication

- 级别：约束
- 文件：`modules/qa/internal/app/wire.go`
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
7. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:readSiteConfig`、`qa.LoadRuntime`、`qa.RecoverTurns`、`qa.PruneExpiredRecords`、`qa.PublishOutbox`、`qa.ConsumeEvents`、`qa.RunWorker`、`platform.ReadRuntime`、`platform.NewRequestValidator`、`qa.NewRuntime`、`qa.WithRuntime`、`qa.NewOperationHandlers`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## qa.Serve

- 级别：约束
- 文件：`modules/qa/internal/transport/server.go`
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
- 依赖：`qa.BuildApplication`、`qa.Shutdown`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## qa.Shutdown

- 级别：约束
- 文件：`modules/qa/internal/transport/server.go`
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

## qa.Health

- 级别：约束
- 文件：`modules/qa/internal/transport/server.go`
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

## qa.Ready

- 级别：约束
- 文件：`modules/qa/internal/transport/server.go`
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

## qa.NewOperationHandlers

- 级别：参考
- 文件：`modules/qa/internal/transport/server.go`
- 作用：NewOperationHandlers的本域内部实现助手

```go
func NewOperationHandlers(bind func(context.Context)context.Context)map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.CreateConversation`、`qa.GetConversation`、`qa.RecordFeedback`、`qa.ReadQaPolicy`、`qa.ReadPublicCapability`、`qa.serveAnswerStream`、`qa.jsonOperation`、`qa.operationContext`、`qa.writeResult`、`qa.ListTurnRecords`、`qa.ReadTurnRecord`、`qa.ReadDebugTurn`、`qa.CancelTurn`、`qa.DeleteConversation`

## qa.jsonOperation

- 级别：参考
- 文件：`modules/qa/internal/transport/server.go`
- 作用：jsonOperation的本域内部实现助手

```go
func jsonOperation[I,R any](bind func(context.Context)context.Context,status int,run func(context.Context,platform.OperationContext,I)(R,error),path func(*http.Request,*I)error)http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.operationContext`、`qa.writeResult`

## qa.operationContext

- 级别：参考
- 文件：`modules/qa/internal/transport/server.go`
- 作用：operationContext的本域内部实现助手

```go
func operationContext(r *http.Request,bind func(context.Context)context.Context)(context.Context,platform.OperationContext,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.writeResult

- 级别：参考
- 文件：`modules/qa/internal/transport/server.go`
- 作用：writeResult的本域内部实现助手

```go
func writeResult(w http.ResponseWriter,op platform.OperationContext,status int,result any,err error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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

## qa.serveAnswerStream

- 级别：参考
- 文件：`modules/qa/internal/transport/server.go`
- 作用：serveAnswerStream的本域内部实现助手

```go
func serveAnswerStream(w http.ResponseWriter,r *http.Request,bind func(context.Context)context.Context)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验业务输入、可信依赖与事务
2. 具体分支与固定参数见本域data/workflows与源码注释

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
- 依赖：`qa.operationContext`、`qa.writeResult`、`qa.ResumeAnswerStream`、`qa.answerEventStream.Next`、`qa.answerEventStream.Close`、`platform.ServeJSONEventStream`

## qa.answerEventStream.Next

- 级别：参考
- 文件：`modules/qa/internal/transport/server.go`
- 作用：按持久序号读下一条本域事件并转换平台SSE帧，不实现通用心跳和背压

```ts
func(reader *answerEventStream)Next(ctx context.Context)(platform.SSEFrame,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理可信输入和已装配依赖
2. 详细边界和固定算法见data/workflows及源码注释

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
- 依赖：`qa.ResumeAnswerStream`

## qa.answerEventStream.Close

- 级别：参考
- 文件：`modules/qa/internal/transport/server.go`
- 作用：取消本域事件读取；HTTP、心跳、背压和流关闭交公共平台

```ts
func(reader *answerEventStream)Close()error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理可信输入和已装配依赖
2. 详细边界和固定算法见data/workflows及源码注释

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

## qa.ConsumeEvents

- 级别：约束
- 文件：`modules/qa/internal/events/subscription.go`
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
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`platform.RunConsumer`

## qa.InvokeModel

- 级别：参考
- 文件：`modules/qa/internal/runtime/model_client.go`
- 作用：经模型网关执行一次调用并读取到终态

```go
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error)
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
- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）

- 副作用：HTTP调用model-gateway推理接口；模型费用由网关记账
- 授权：permission：service；rule：以本服务身份调用网关，费用类别继承可信op
- 并发：每次调用独立；同一根操作的子调用共享网关费用作用域
- 幂等：幂等键固定，重试返回原invocation
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`、`platform.InvokeModel`、`qa.FromContext`、`model-gateway:lookupInference`、`model-gateway:readInference`、`model-gateway:resolveTaskRoute`

## qa.ModelText

- 级别：参考
- 文件：`modules/qa/internal/runtime/model_client.go`
- 作用：读取已持久化的完整winning文本，不把缺失结果当空答案，不把结果完成等同费用已结算

```go
func ModelText(outcome InvokeModelResult)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理可信输入和已装配依赖
2. 详细边界和固定算法见data/workflows及源码注释
3. 仅消费CurrentSnapshot.Output.Text非nil的真实完整winning结果，显式空字符串可用；不由历史delta拼接或把费用未知当作没有结果，原费用/paid refs继续保留。

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

## qa.ExecuteTurn

- 级别：约束
- 文件：`modules/qa/internal/turn/execute.go`
- 作用：主Agent直接查阅与回答，投影逐次活动，按需上报滥用信号，不查写答案缓存或调用前置判定模型

```go
func ExecuteTurn(ctx context.Context, op OperationContext, turnID, currentDocument string)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `turnID` | `string` | 已持久化的原轮次ID | 通过本域轮次记录、租约与原根校验 |
| `currentDocument` | `string` | 访客提问时所在的文档ID；首页为空 | 仅可读当前公开发布正文，不能作为私有文件路径 |

**输出**

无

**步骤**

1. 校验原准入和轮次归属，续租并贯穿取消。
2. 加载历史时复核公开来源版本，读取有效原生检查点及其后的消息；预读当前页和站点导航。
3. 交由独立 ZCode AgentRuntime 使用模型默认窗口，自主以英文关键词进行嵌入混合检索并继续查阅相关正文；接近窗口上限时由原生机制压缩。
4. 模型调用仍由问答服务经模型网关执行，工具继续在本域核对公开权限与发布版本；不应用旧固定上下文、轮数或工具次数限制。
5. 保存本轮原始工具消息及最新原生检查点，校验最终引用并完成问答、释放准入；失败不重新派发原模型调用。

**错误**

- `CANCELLED`：访客取消（不可重试）
- `BUDGET_EXCEEDED`：问答额度不足（不可重试）
- `RATE_LIMITED`：准入失效（不可重试）
- `DEPENDENCY_UNAVAILABLE`：检索或网关不可用（可重试）

- 副作用：写问答记录、流事件与缓存；模型费用经网关
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：同会话同时只执行一轮；所有写入核lease、取消与epoch
- 幂等：按turnId唯一；已派发调用不重放
- 依赖：`qa.NormalizeQuestion`、`qa.ResolveTurnContext`、`qa.ResolveCurrentPageHint`、`qa.RetrieveEvidence`、`qa.InspectEvidence`、`qa.BuildAnswerPrompt`、`qa.GenerateAnswer`、`qa.ValidateCitation`、`qa.CompleteTurn`、`qa.CreateCompaction`、`traffic-guard:renewAdmission`、`traffic-guard:queueSessionAdmission`、`jobs:createJob`、`qa.reportAgentAbuse`、`agent:startAgentRun`、`agent:readAgentEvents`、`agent:replyAgentRequest`、`agent:cancelAgentRun`

## qa.ListTurnRecords

- 级别：约束
- 文件：`modules/qa/internal/admin/records.go`
- 作用：浏览与筛选问答记录

```go
func ListTurnRecords(ctx context.Context, op OperationContext, input ListTurnRecordsInput) (ListTurnRecordsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ListTurnRecordsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListTurnRecordsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. qa.manage
2. 按筛选条件分页读取本域问答记录，只返回问题前200字与分类
3. 不返回访客标识或IP

**错误**

- `FORBIDDEN`：权限不足（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）

- 副作用：无
- 授权：permission：qa.manage；rule：后台管理员按登记权限
- 并发：只读快照或CAS
- 幂等：读取幂等；写入按operationId
- 依赖：`qa.RequireAdmin`、`qa.BeginRead`、`qa.CursorActor`、`qa.StorageError`、`qa.Transaction.Close`

## qa.ReadTurnRecord

- 级别：约束
- 文件：`modules/qa/internal/admin/records.go`
- 作用：查看单条问答记录以设为精选

```go
func ReadTurnRecord(ctx context.Context, op OperationContext, input ReadTurnRecordInput) (ReadTurnRecordResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ReadTurnRecordInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadTurnRecordResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 校验qa.manage与站点代次后读取未过期的轮次
2. 返回原问题、原回答和原始引用快照；来源当前状态不影响历史记录查看
3. 不返回准入许可、原始模型输入和内部上下文

**错误**

- `FORBIDDEN`：权限不足（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）

- 副作用：无
- 授权：permission：qa.manage；rule：后台管理员按登记权限
- 并发：只读快照或CAS
- 幂等：读取幂等；写入按operationId
- 依赖：`content:validatePublications`、`qa.RequireAdmin`、`qa.BeginRead`、`qa.ReadTurn`、`qa.ValidateCitations`、`qa.Transaction.Close`

## qa.ReadQaPolicy

- 级别：约束
- 文件：`modules/qa/internal/admin/policy.go`
- 作用：读取问答策略

```go
func ReadQaPolicy(ctx context.Context, op OperationContext, input ReadQaPolicyInput) (ReadQaPolicyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ReadQaPolicyInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadQaPolicyResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. qa.manage
2. 返回缓存开关、命中方式、阈值、有效期、压缩参数与版本

**错误**

- `FORBIDDEN`：权限不足（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）

- 副作用：无
- 授权：permission：qa.manage；rule：后台管理员
- 并发：只读快照
- 幂等：只读
- 依赖：`qa.RequireAdmin`、`qa.BeginRead`、`qa.ReadPolicy`、`qa.Transaction.Close`

## qa.ReadPublicCapability

- 级别：约束
- 文件：`modules/qa/internal/admin/policy.go`
- 作用：免费读取公开问答入口配置能力

```go
func ReadPublicCapability(ctx context.Context, op OperationContext, input ReadPublicCapabilityInput) (ReadPublicCapabilityResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消和截止 | 已在服务入口严格校验；内部不重复schema解析 |
| `op` | `OperationContext` | 已验证服务身份、当前epoch和期限 | 已在服务入口严格校验；内部不重复schema解析 |
| `input` | `ReadPublicCapabilityInput` | 严格封闭输入 | 已在服务入口严格校验；内部不重复schema解析 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadPublicCapabilityResult` | 安全完整响应 | 已在服务入口严格校验；内部不重复schema解析 |
| `error` | `Error` | 公共错误，不返回私密原值 | 已在服务入口严格校验；内部不重复schema解析 |

**步骤**

1. 只接受site-config/public-site固定纯服务caller，当前epoch一致；不接受用户自造费用/主体字段
2. 调用model-gateway.readTaskAvailability精确固定qa.preflight/qa.inspect/qa.answer；严格检查无遗漏/重复/额外任务
3. 三项均available才true；已知任一未配置false/not_configured，其他已配不可用false/unavailable；网关未知/超时/错误503，不把未知说成已知未配
4. 只返回available/reason且no-store，不返回模型配置、费用信息或原始输入，不调用模型、检索、guard准入或创建任务

**错误**

- `VALIDATION`：请求不符合协议（不可重试）
- `UNAUTHENTICATED`：服务证明无效（不可重试）
- `FORBIDDEN`：非固定调用服务或越过只读范围（不可重试）
- `VERSION_MISMATCH`：当前dataEpoch不匹配（不可重试）
- `INTEGRITY_ERROR`：私有权威数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖或存储未知（可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接受已验证纯服务actor，固定调用方site-config,public-site；不授予模型调用或管理写入权限
- 并发：当前权威快照一致读取；不持锁执行跨服务IO
- 幂等：无写入或费用副作用；每次读当前权威，no-store，不创建后台任务
- 依赖：`model-gateway:readTaskAvailability`、`qa.BeginRead`、`qa.DecodeResponse`、`qa.ServiceReadContext`、`qa.Transaction.Close`

## qa.QaPage

- 级别：约束
- 文件：`modules/qa/web/src/index.tsx`
- 作用：问答管理：策略、问答记录与精选、答案缓存、调试台

```ts
export default function QaPage(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 按授权项显示标签页
2. 调试台结果只读取隔离的诊断记录

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无

## qa.QaFloating

- 级别：约束
- 文件：`modules/qa/web/src/island/index.tsx`
- 作用：公开页问答岛：文档页右下角的问答面板，独立问答页用同一会话的内嵌形态（QaInline）

```ts
export function QaFloating(props: { open: boolean; onOpenChange: (open: boolean) => void; locale: string; t: (key: string) => string }): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 需要人机验证时在 Web Worker 里完成工作量证明后重试
2. 流式读取回答与引用，可停止、可评价；会话与记录只存浏览器会话存储

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无

## qa.AcquireQueryEmbedding

- 级别：参考
- 文件：`modules/qa/internal/answer/query-embedding.go`
- 作用：同查询只生成一次嵌入

```go
func AcquireQueryEmbedding(ctx context.Context, op OperationContext, input AcquireQueryEmbeddingInput) (AcquireQueryEmbeddingResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `AcquireQueryEmbeddingInput` | 封闭业务输入；逐字段见types $defs/AcquireQueryEmbeddingInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcquireQueryEmbeddingResult` | 成功结果；逐字段见types $defs/AcquireQueryEmbeddingResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 本turn根queryHash/space singleflight
2. knowledge createQueryEmbedding产一次向量
3. 校验queryHash/space/vectorHash/dataEpoch
4. 同结果供semantic cache和searchKnowledge，cache miss不重新发嵌入。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：MG费用一次与本轮短期向量记录
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge:createQueryEmbedding`、`qa.FromContext`、`qa.DecodeResponse`、`qa.ServiceReadContext`

## qa.reportAgentAbuse

- 级别：参考
- 文件：`modules/qa/internal/answer/abuse.go`
- 作用：主Agent上报本轮滥用信号；对象固定于许可，封禁由风控决定

```go
func reportAgentAbuse(ctx context.Context, op OperationContext, turnID, verdict string) (AgentToolResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PlanQuestionInput` | 封闭业务输入；逐字段见types $defs/PlanQuestionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PlanQuestionResult` | 成功结果；逐字段见types $defs/PlanQuestionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 单独解析qa.preflight任务记忆，不复用qa.answer；内存/历史/摘要/verifiedPage都来自本轮已固定可信输入
2. fast档最多500输出token，严格解析PlannerDecision，canonicalQuestion经共享基准语言规范化；仅格式失败追加一次独立attempt
3. resolvedReferences缺失false；任何缺指代或needs_clarification强制禁cache；私人/动态页依赖也禁cache，不做关键词扫描
4. 仅将原permit scopes、问题hash和normal/suspicious/abuse映射报告guard，不发正文和伪造测量；deny停止付费，challenge返回CHALLENGE_REQUIRED沿既有issueChallenge流程，未知决定失败关闭

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `CHALLENGE_REQUIRED` / HTTP 429：（不可重试）

- 副作用：MG费用；保存原问题与canonical问题
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`traffic-guard:reportModelRisk`
