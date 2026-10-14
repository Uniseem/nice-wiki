# platform 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 17 个函数：约束 17 个，参考 0 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

## platform.SignAssertion

- 级别：约束
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：为一次服务间请求签发短期断言

```go
func SignAssertion(key SigningKey, claims AssertionClaims) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 本服务部署时生成的Ed25519私钥引用 | 只从secret文件读取，不进日志 |
| `claims` | `AssertionClaims` | 断言内容 | expiresAt-issuedAt≤60秒 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `token` | `string` | JWS compact断言 | 只放Authorization头 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 校验claims：issuer为本服务slug，audience为目标服务，method/path/bodyHash齐全，有效期≤60秒
2. 以EdDSA签名生成JWS compact，header含keyId
3. 返回token；不缓存、不记录

**错误**

- `VALIDATION`：声明不完整或有效期过长（不可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：纯函数，可并发
- 幂等：每次生成新jti
- 依赖：无

## platform.VerifyAssertion

- 级别：约束
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：验证服务间请求断言

```go
func VerifyAssertion(anchors TrustAnchors, token string, expect AssertionExpectation) (AssertionClaims, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 服务公钥表（trustAnchorRef加载） | 只接受EdDSA |
| `token` | `string` | 请求携带的断言 | 长度≤4096 |
| `expect` | `AssertionExpectation` | 本请求的期望值 | audience为本服务 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `AssertionClaims` | 已验证声明 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按keyId查公钥，拒绝未知key、非EdDSA算法与alg=none
2. 核audience、method、path、bodyHash、有效期（允许30秒时钟偏差）
3. jti在有效期内只接受一次（进程内重放缓存）
4. 返回已验证的调用服务slug

**错误**

- `UNAUTHENTICATED`：签名、有效期或声明不符（不可重试）
- `CONFLICT`：jti重放（不可重试）

- 副作用：写进程内重放缓存
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：重放缓存加锁
- 幂等：jti唯一
- 依赖：无

## platform.VerifyDelegation

- 级别：约束
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：验证portal签发的用户委托证明

```go
func VerifyDelegation(anchors TrustAnchors, proof string, expect DelegationExpectation) (DelegationClaims, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 含portal委托公钥 | 只接受EdDSA |
| `proof` | `string` | X-Portal-Proof头 | 长度≤8192 |
| `expect` | `DelegationExpectation` | 本请求的路由与正文摘要 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `DelegationClaims` | 已验证的用户委托 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 用portal委托公钥验签，拒绝未知key与过期（≤10秒）
2. 核route与bodyHash；下游链路允许原样转发同一证明
3. 返回actor、grants与sessionVersion；权限仍须由业务函数经identity:checkAuthorization核实

**错误**

- `UNAUTHENTICATED`：证明无效或过期（不可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：纯函数
- 幂等：无状态
- 依赖：无

## platform.BuildOperationContext

- 级别：约束
- 文件：`libs/platform/opctx/context.go`
- 作用：从入站请求构建可信OperationContext

```go
func BuildOperationContext(r *http.Request, anchors TrustAnchors, runtime RuntimeView) (OperationContext, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `r` | `*http.Request` | 入站请求 | 只读头部 |
| `anchors` | `TrustAnchors` | 公钥表 | 必填 |
| `runtime` | `RuntimeView` | 当前运行状态 | 1秒内读取 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `op` | `OperationContext` | 可信操作上下文 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. VerifyAssertion确定callerService；有X-Portal-Proof时VerifyDelegation得到actor
2. 读取X-Operation-Id、X-Root-Operation-Id、X-Cost-Class（只接受可信调用方传递）、截止与取消标识
3. dataEpoch取runtime当前active epoch；请求声明的epoch不一致返回VERSION_MISMATCH
4. 浏览器自报的身份、费用类别、网络作用域一律忽略

**错误**

- `UNAUTHENTICATED`：断言或委托无效（不可重试）
- `VERSION_MISMATCH`：dataEpoch不一致（不可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每请求独立
- 幂等：无状态
- 依赖：`platform.VerifyAssertion`、`platform.VerifyDelegation`

## platform.WriteProblem

- 级别：约束
- 文件：`libs/platform/problem/problem.go`
- 作用：按RFC9457输出公共错误

```go
func WriteProblem(w http.ResponseWriter, problem Problem) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `w` | `http.ResponseWriter` | 响应 | 未写头 |
| `problem` | `Problem` | 公共Problem | code登记于errors.json |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按errors.json映射HTTP状态
2. 输出application/problem+json，detail去除路径、堆栈与密钥
3. 附requestId与retryable

**错误**

- 无

- 副作用：写HTTP响应
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每请求一次
- 幂等：无状态
- 依赖：无

## platform.EnqueueEvent

- 级别：约束
- 文件：`libs/platform/outbox/outbox.go`
- 作用：在业务事务内登记outbox事件

```go
func EnqueueEvent(tx Tx, envelope EventEnvelope) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tx` | `Tx` | 调用方业务事务 | 必须是同一提交 |
| `envelope` | `EventEnvelope` | 事件信封 | eventType已在本域events.json登记 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 校验信封字段与载荷不含正文、密钥、IP
2. 计算payloadHash，按eventId唯一插入outbox行（状态pending）
3. 随业务事务一起提交；事务回滚则事件不存在

**错误**

- `VALIDATION`：信封不合法（不可重试）
- `CONFLICT`：同eventId异hash（不可重试）

- 副作用：写本域outbox表
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：与业务事务同锁
- 幂等：eventId唯一
- 依赖：无

## platform.RunRelay

- 级别：约束
- 文件：`libs/platform/outbox/relay.go`
- 作用：outbox转发循环的通用实现

```go
func RunRelay(ctx context.Context, store OutboxStore, client JobsClient, config RelayConfig) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `store` | `OutboxStore` | 本域outbox访问 | 同一数据库 |
| `client` | `JobsClient` | jobs客户端 | 带服务断言 |
| `config` | `RelayConfig` | 批量与轮询参数 | batchLimit≤100 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按序号以30秒行租约领取已提交pending行
2. 逐条调用jobs:publishEvent，确认后标relayed
3. 失败按1/2/4秒退避（上限60秒）；同ID异hash标quarantined
4. 屏障draining时继续转发直到空；恢复换入后丢弃旧dataEpoch行

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：读写outbox；HTTP调用jobs
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：行级租约防多实例重复
- 幂等：eventId固定
- 依赖：`jobs:publishEvent`

## platform.ClaimInbox

- 级别：约束
- 文件：`libs/platform/inbox/inbox.go`
- 作用：事件处理函数的inbox去重

```go
func ClaimInbox(tx Tx, envelope EventEnvelope) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tx` | `Tx` | 处理函数的业务事务 | 与业务写入同一提交 |
| `envelope` | `EventEnvelope` | 收到的事件 | 已校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `fresh` | `bool` | true表示首次处理 | false时调用方只确认不写业务 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按eventId查inbox；同ID同hash返回false
2. 同ID异hash返回INTEGRITY_ERROR并隔离
3. 同subject低于已处理revision返回false
4. 首次则插入inbox行并返回true

**错误**

- `INTEGRITY_ERROR`：同eventId异payloadHash（不可重试）

- 副作用：写本域inbox表
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：与业务事务同锁
- 幂等：eventId唯一
- 依赖：无

## platform.RunConsumer

- 级别：约束
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件订阅循环的通用实现

```go
func RunConsumer(ctx context.Context, client JobsClient, config ConsumerConfig, handlers EventHandlerRegistry) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `JobsClient` | jobs客户端 | 带服务断言 |
| `config` | `ConsumerConfig` | 订阅的事件名/版本与并发 | 来自本域events.json |
| `handlers` | `EventHandlerRegistry` | 事件名→处理函数 | 与events.json handler一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. durable模式jobs:subscribeEvents，Last-Event-ID续传
2. 校验信封、payloadSchema与dataEpoch；旧代事件确认跳过，未换入的新代事件不确认
3. 按事件名调用登记处理函数；成功后jobs:acknowledgeEvent，失败不确认等待重投
4. 同subject串行，不同subject并发≤maxInFlight；屏障draining时停止领取

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：HTTP长连接与确认
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一个订阅
- 幂等：依赖处理函数inbox去重
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`

## platform.RunWorkerLoop

- 级别：约束
- 文件：`libs/platform/worker/loop.go`
- 作用：任务领取循环的通用实现

```go
func RunWorkerLoop(ctx context.Context, client JobsClient, config WorkerConfig, executors JobExecutorRegistry) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `JobsClient` | jobs客户端 | 带服务断言 |
| `config` | `WorkerConfig` | 本域jobTypes与并发 | 来自implementation.json |
| `executors` | `JobExecutorRegistry` | jobType→执行函数 | 与jobTypes一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. jobs:claimJob只领取登记类型；维护期间暂停
2. jobs:startJob后调用执行函数；每10秒jobs:heartbeatJob续租、上报进度并读取取消
3. 写回前jobs:checkLease；成功jobs:completeJob，失败jobs:failJob
4. lease过期立即停止写入，不重放已派发模型调用

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：HTTP调用jobs租约接口
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：并发≤maxConcurrency
- 幂等：jobs按jobId与fence去重
- 依赖：`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`

## platform.InvokeModel

- 级别：约束
- 文件：`libs/platform/modelclient/invoke.go`
- 作用：模型调用助手的通用实现

```go
func InvokeModel(ctx context.Context, client GatewayClient, op OperationContext, input InvokeModelRequest) (InvokeModelOutcome, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止 | 取消即调用cancelInference |
| `client` | `GatewayClient` | 模型网关客户端 | 带服务断言 |
| `op` | `OperationContext` | 可信上下文 | 费用类别与根操作继承 |
| `input` | `InvokeModelRequest` | 调用请求 | taskId在根AI任务目录 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `outcome` | `InvokeModelOutcome` | 终态与事件 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. model-gateway:createInference（幂等键=操作ID+步骤名）
2. model-gateway:resumeInference按序号读取到终态，流式时逐条回调
3. 取消或截止时model-gateway:cancelInference
4. failed/charge_unknown按网关Problem返回，不在调用方重放

**错误**

- `CANCELLED`：已取消（不可重试）
- `DEPENDENCY_UNAVAILABLE`：网关失败（可重试）

- 副作用：HTTP调用网关
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每次调用独立
- 幂等：幂等键固定
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`

## platform.LoadServiceConfig

- 级别：约束
- 文件：`libs/platform/lifecycle/config.go`
- 作用：按CLI>env>文件>默认值加载并严格校验服务配置

```go
func LoadServiceConfig(args []string, env []string) (ServiceConfig, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `args` | `[]string` | 命令行参数 | --config/--data-dir/--listen |
| `env` | `[]string` | 环境变量 | 只读取NW_前缀 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `config` | `ServiceConfig` | 严格解析后的配置 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 读取配置文件，拒绝未知字段
2. 按优先级合并并校验端口、数据根、依赖地址与secret引用
3. secret只保存引用路径，不读入日志

**错误**

- `VALIDATION`：配置不合法（不可重试）

- 副作用：读配置文件
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：启动时调用一次
- 幂等：纯函数
- 依赖：无

## platform.ReadRuntime

- 级别：约束
- 文件：`libs/platform/lifecycle/runtime.go`
- 作用：读取site-config权威运行状态（最多缓存1秒）

```go
func ReadRuntime(ctx context.Context, client SiteConfigClient) (RuntimeView, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 截止 | ≤2秒 |
| `client` | `SiteConfigClient` | site-config客户端 | 带服务断言 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `runtime` | `RuntimeView` | 运行状态 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 调用site-config:readRuntimeState并核签名
2. 缓存≤1秒；失联时写请求一律关闭

**错误**

- `DEPENDENCY_UNAVAILABLE`：site-config不可达（可重试）

- 副作用：HTTP调用site-config
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：缓存加锁
- 幂等：只读
- 依赖：`site-config:readRuntimeState`

## platform.WatchRuntime

- 级别：约束
- 文件：`libs/platform/lifecycle/runtime.go`
- 作用：运行中发现恢复切换时请求安全退出

```go
func WatchRuntime(ctx context.Context, client SiteConfigClient, dataRoot string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `SiteConfigClient` | site-config客户端 | 带服务断言 |
| `dataRoot` | `string` | 本域数据根 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 每5秒ReadRuntime
2. phase为restore_switching或restore_rolling_back且本域有对应staged或已换入目录时返回ErrRestartForRestore
3. 调用方据此Shutdown并以退出码75退出，由进程守护重启

**错误**

- `DEPENDENCY_UNAVAILABLE`：site-config不可达（可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一个
- 幂等：只读
- 依赖：`platform.ReadRuntime`

## platform.ApplyRestoreSwap

- 级别：约束
- 文件：`libs/platform/lifecycle/restore_swap.go`
- 作用：启动时按恢复阶段换入或换回active目录（ADR0003）

```go
func ApplyRestoreSwap(dataRoot string, runtime RuntimeView) (SwapResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `dataRoot` | `string` | 本域数据根 | 含active、control、restore子目录 |
| `runtime` | `RuntimeView` | 启动时读取的运行状态 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SwapResult` | 是否换入或换回 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 读control区恢复日志
2. restore_switching且存在restore/<id>/staged且未换入：active→restore/<id>/before，staged→active，fsync父目录，写日志
3. restore_rolling_back且已换入：反向换回并写日志
4. 其余情况不动目录；目录状态与日志矛盾时拒绝启动

**错误**

- `INTEGRITY_ERROR`：目录与日志不一致（不可重试）

- 副作用：重命名数据目录；写control日志
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：启动阶段独占，入口开放前完成
- 幂等：按restoreId与日志状态幂等
- 依赖：无

## platform.RunService

- 级别：约束
- 文件：`libs/platform/lifecycle/service.go`
- 作用：各服务main的通用进程模板

```go
func RunService(spec ServiceSpec) int
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `spec` | `ServiceSpec` | 本域slug与装配回调 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `exitCode` | `int` | 0正常、1失败、2配置错误、75恢复重启 | main直接os.Exit |

**步骤**

1. LoadServiceConfig；配置错误返回2
2. 注册SIGINT/SIGTERM；调用spec的BuildApplication（其中先ApplyRestoreSwap）
3. 运行Serve与WatchRuntime；收到信号或恢复切换时以30秒上限Shutdown
4. 返回对应退出码；不能安全关闭返回1并保留journal

**错误**

- `INTERNAL`：装配或运行失败（不可重试）

- 副作用：监听HTTP；启动后台任务
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一次
- 幂等：重启按journal恢复
- 依赖：`platform.LoadServiceConfig`、`platform.WatchRuntime`

## platform.main

- 级别：约束
- 文件：`libs/platform/cmd/nw-keys/main.go`
- 作用：生成各服务Ed25519密钥对与公钥表的运维工具

```go
func main()
```

**输入**

无

**输出**

无

**步骤**

1. 读取deploy/config/services.json中的服务清单
2. 为每个服务与portal委托各生成一对Ed25519密钥，私钥写入各自secret目录（0600）
3. 汇总公钥写deploy/config/trust-anchors.json；已存在的密钥不覆盖，--rotate时新增keyId并保留旧公钥一个过渡期

**错误**

- `VALIDATION`：服务清单不合法（不可重试）

- 副作用：写secret文件与公钥表
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：离线运行
- 幂等：已存在密钥不覆盖
- 依赖：无
