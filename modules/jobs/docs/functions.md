# jobs 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 161 个函数：约束 35 个，参考 126 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## jobs.ValidateJobSpec

- 级别：参考
- 文件：`modules/jobs/internal/validation.go`
- 作用：验证任务入口

```go
func ValidateJobSpec(ctx context.Context, op OperationContext, in ValidateJobSpecInput) (ValidateJobSpecResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateJobSpecInput` | 验证任务入口；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateJobSpecResult` | 验证任务入口成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. owner/type须在config/job-types.json（由各域jobTypes生成）登记
2. 引用owner与hash/size检查
3. dataEpoch匹配运行权威

**错误**

- `VALIDATION` / HTTP 400：INVALID_JOB：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service所属owner
- 并发：纯校验+runtime只读
- 幂等：同jobSpec同结果
- 依赖：`site-config:readRuntimeState`

## jobs.CreateJob

- 级别：约束
- 文件：`modules/jobs/internal/queue.go`
- 作用：持久提交任务

```go
func CreateJob(ctx context.Context, op OperationContext, in CreateJobInput) (CreateJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CreateJobInput` | 持久提交任务；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateJobResult` | 持久提交任务成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验证spec
2. 通过LeaseRuntimeProvider核对当前运行epoch与control屏障；维护或safePointRequested拒绝新任务，不读active旧barriers表
3. 事务按owner/epoch/dedupeKey查重复
4. 同inputHash复用，否则409
5. 插入queued及outbox
6. 持久保存可信op.rootOperationId，供以后领取/回收复用；不得以新的领取操作替换根
7. notBefore省略时原输入摘要字节不变且首次not_before=当前时刻；显式时将UTC RFC3339Nano时刻纳入摘要并持久入队，过去时刻立即具备领取资格；同键不同显式时刻CONFLICT，不用op.deadline作为延迟任务期限

**错误**

- `CONFLICT` / HTTP 409：JOB_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：LOCKED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写job/outbox
- 授权：permission：service；rule：service所属owner
- 并发：SQLite单事务；barrier draining拒绝新任务
- 幂等：dedupeKey/inputHash幂等
- 依赖：`jobs.ValidateJobSpec`、`jobs.beginLeaseOperation`、`jobs.createJob`

## jobs.FailJob

- 级别：约束
- 文件：`modules/jobs/internal/queue.go`
- 作用：失败写回

```go
func FailJob(ctx context.Context, op OperationContext, in FailJobInput) (FailJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `FailJobInput` | 失败写回；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FailJobResult` | 失败写回成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 同事务核对本service所属owner、当前epoch、worker、持久fence与未过期租约；只接受running/cancel-requested
2. 固定公共错误码分类；持久cancel_requested优先：CANCELLED为owner已确认安全取消；其他错误先orphan并释放lease、登记恢复引用，不能因临时失败重新排队；未请求取消才按原有限退避
3. CANCELLED只在cancel-requested且owner确认模型已停止或结算进入持久可恢复状态时由ConfirmCancelled完成
4. 合并保留已有paidResultRefs；CAS目标状态、notBefore、最后安全错误码，释放lease、写状态outbox与回执一起提交
5. 同failure operationId同输入重放原结果，异输入CONFLICT；不记录原始错误或正文
6. 主动报告失败并进入orphan时交还当前lease，保留fencing_counter用于原操作只读回执；旧worker因无lease/非活动状态失去执行权，不能恢复或重新领取取消任务

**错误**

- `STALE_RESULT` / HTTP 409：LEASE_LOST：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_ERROR_CODE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `VERSION_MISMATCH` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：操作键冲突、旧epoch或可信依赖不可用（可重试）
- `LOCKED` / HTTP 409：当前屏障已到安全点后不能新增操作回执（不可重试）

- 副作用：写状态/outbox
- 授权：permission：service；rule：service所属owner
- 并发：fence CAS
- 幂等：owner/epoch/failureOperationId绑定规范输入；同job/fence终局回执不能更换worker或结果
- 依赖：`jobs.ClassifyFailure`、`jobs.ComputeRetryAt`、`jobs.ConfirmCancelled`、`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.readLease`、`jobs.terminalReplay`、`jobs.saveTerminal`、`jobs.mergePaidResults`、`jobs.finishJob`

## jobs.InstallStore

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：安装本域Store及可信runtime/control屏障读取钩子

```go
func InstallStore(store *Store, runtime LeaseRuntimeProvider, identity *gen.IdentityClient, recovery OwnerRecoveryChecker, siteID SiteIDProvider)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `*Store` | 已打开的私有存储 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `runtime` | `LeaseRuntimeProvider` | 运行权威和control屏障读取 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `identity` | `*IdentityClient` | 本域生成identity HTTP客户端 | 未装配时管理读写返回DEPENDENCY_UNAVAILABLE |
| `recovery` | `OwnerRecoveryChecker` | 可信owner恢复确认边界 | 实际owner版本化适配未接通时为空；过期任务保持orphan，不自动重跑 |

**输出**

无

**步骤**

1. 安装按需读取真实siteID的provider及已验证runtime/recovery/identity客户端；启动阶段不得调用readSiteConfig，只有真实服务读取或恢复任务需要站点作用域时才读取，未初始化、空值或读取失败均失败关闭并从任务旧行推断站点。

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.OpenDatabase

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：打开私有库并执行全部已知追加迁移，随后登记全部运行期业务写门，完整装配前不可用

```go
func OpenDatabase(dataRoot string) (*Store, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `dataRoot` | `string` | 本域绝对数据根 | 仅当前授权链路或可信启动装配，取消原样传IO |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `*Store` | 已完成全部已知迁移的私有库 | 仅当前授权链路或可信启动装配，取消原样传IO |
| `error` | `error` | 失败时结果无效，响应开始后仅结束流 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 打开私有库并执行全部已知追加迁移；迁移完成后按本域可信业务表清单调用platform.RegisterWorkTable；随后完整装配前不可用

**错误**

- `VALIDATION`：数据根不合法（不可重试）
- `INTERNAL`：业务库打开/迁移失败（不可重试）
- `INTEGRITY_ERROR`：业务库或已应用迁移未知（不可重试）

- 副作用：打开WAL/外键/单连接业务库并应用已知迁移，失败关闭
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：`jobs.runMigrations`、`jobs.registerJobsWorkTables`

## jobs.runMigrations

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：先核全部已应用迁移版本，再唯一EnsureSchema并按embed追加迁移

```go
func (s *Store) runMigrations() error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `s` | `*Store` | 已打开私有库 | 仅当前授权链路或可信启动装配，取消原样传IO |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时结果无效，响应开始后仅结束流 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 先核全部已应用迁移版本，再唯一EnsureSchema并按embed追加迁移

**错误**

- `INTEGRITY_ERROR`：迁移集、版本或SQL源不合法（不可重试）
- `INTERNAL`：迁移事务IO失败（不可重试）

- 副作用：先拒绝未知已应用版本；唯一调用platform outbox.EnsureSchema后执行embed迁移与版本登记事务
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：`jobs.All`

## jobs.registerJobsWorkTables

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：将本域可信登记表纳入platform统一revision和sealed gate

```go
func registerJobsWorkTables(ctx context.Context, db *sql.DB) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域启动或恢复阶段的取消上下文 | 仅可信装配调用；取消透传平台IO，不携带业务正文 |
| `db` | `*sql.DB` | 已完成本域迁移的jobs私有数据库 | 仅当前active或已认证隔离stage数据库；表名不由配置提供 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 任一业务表写门登记失败 | 成功nil；失败不宣称本域已可写 |

**步骤**

1. 按jobsWorkTables逐表调用platform.RegisterWorkTable；不登记schema_migrations或nw_outbox

**错误**

- 无

- 副作用：在本域数据库安装平台业务写门与revision触发器
- 授权：permission：service；rule：可信启动装配
- 并发：逐表短事务，由platform登记操作保证幂等
- 幂等：重复登记同一表不重复创建触发器
- 依赖：`platform.RegisterWorkTable`
- 其他依赖说明：本域SQLite

## jobs.CloseStore

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：关闭私有库并撤销安装句柄，重复关闭幂等

```go
func (s *Store) Close() error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `s` | `*Store` | 本域私有库 | 仅当前授权链路或可信启动装配，取消原样传IO |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时结果无效，响应开始后仅结束流 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 关闭私有库并撤销安装句柄，重复关闭幂等

**错误**

- `INTERNAL`：业务库关闭失败（不可重试）

- 副作用：撤销当前安装并关闭数据库，重复安全
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：无

## jobs.createJob

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：持久提交原任务与可选最早领取时刻

```go
func (s *Store) createJob(ctx context.Context, op OperationContext, in CreateJobInput) (CreateJobResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按原有owner/epoch/dedupe在同一短SQL事务返回原AcceptedJob或CONFLICT
2. 显式notBefore格式化为UTC RFC3339Nano，省略首次仅取当前时间；queued任务、原输入摘要和初始状态outbox同事务写入
3. 不存创建RPC deadline为任务执行期限，领取仍由既有ClaimJob按真实not_before筛选

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：显式延迟属于去重材料，原成功不重新分配时刻
- 依赖：`jobs.hashCreateJobInput`、`jobs.beginLeaseOperation`、`platform.EnqueueEvent`

## jobs.hashCreateJobInput

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：规范任务去重摘要，保持旧无延迟请求兼容

```go
func hashCreateJobInput(in CreateJobInput) (Hash, []byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 保持原owner/type/inputRef/dedupeKey/dataEpoch字段顺序与编码
2. notBefore缺失时omitempty不新增任何字节；显式时按UTC RFC3339Nano固定时刻编码，使等价时区表达一致而不把重试当前时间写入摘要

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：显式延迟属于去重材料，原成功不重新分配时刻
- 依赖：无

## jobs.NewQueueHandlers

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：绑定真实任务提交到统一服务入口

```go
func NewQueueHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `EventHandlers` | 仅createJob的真实HTTP映射，复用本域处理器map内部类型 | 只交统一NewServiceHandler，不独立裸挂 |

**步骤**

1. 仅提供createJob处理器，生成CreateJobRequestBody绑定内部CreateJobInput的owner/type/inputRef/dedupeKey/notBefore，dataEpoch只来自可信op，不接受正文自声明
2. 成功202返回原AcceptedJob，错误经既有统一Problem写出；不补其他handler或假服务就绪

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：显式延迟属于去重材料，原成功不重新分配时刻
- 依赖：`jobs.CreateJob`、`jobs.writeEventResponse`、`jobs.ReadJob`

## jobs.ConfigureBackgroundScope

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：安装后台恢复与outbox共享的真实业务scope

```go
func (s *Store) ConfigureBackgroundScope(admit platform.WorkAdmission, bindEventScope func(context.Context, platform.OutboxRow) (context.Context, func(), error)) error
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝任一scope为空；保存同一SnapshotManager派生的准入与事件绑定

**错误**

- 无

- 副作用：更新进程内后台scope引用
- 授权：permission：service；rule：可信启动装配
- 并发：Store mutex
- 幂等：同一装配重复写入幂等
- 依赖：`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.BindOutboxEventScope`

## jobs.currentStore

- 级别：参考
- 文件：`modules/jobs/internal/queue.go`
- 作用：读取启动期已安装的本域句柄

```go
func currentStore() *Store
```

**输入**

无

**输出**

无

**步骤**

1. 持activeQueue读锁返回本域句柄，不构造新存储。

**错误**

- `VALIDATION` / HTTP 400：INVALID_JOB：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service所属owner
- 并发：纯校验+runtime只读
- 幂等：同jobSpec同结果
- 依赖：无

## jobs.ClaimJob

- 级别：约束
- 文件：`modules/jobs/internal/leases.go`
- 作用：领取待执行任务

```go
func ClaimJob(ctx context.Context, op OperationContext, in ClaimJobInput) (ClaimJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ClaimJobInput` | 领取待执行任务；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ClaimJobResult` | 领取待执行任务成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 校验service所属owner，读取可信runtime与control屏障；维护/屏障拒绝领取，输入/op/runtime代次必须相同
2. 可选jobTypes每项必须命中registeredJobTypes的owner/type配对；在queued候选SQL按真实type筛选，领取回执原输入绑定集合，省略保持旧全owner领取。
3. 同owner/epoch/claim operationId按inputHash读取原批次，同键不同输入返回CONFLICT；重放不续租
4. limit省略时显式归一化为10，不依赖生成器填default；精确读取未到期的leased/running/cancel-requested数量；实际批次为min(limit,10,max(0,8-owner在途数))
5. 只在本owner/epoch的queued任务中按notBefore/createdAt/jobId排序，过滤未来notBefore
6. 单事务CAS queued→leased、attempt和revision递增；持久fencing_counter递增后写30秒租约，计数不随lease释放丢失
7. 同事务写状态outbox与claim回执；返回原任务operationId和不可变输入引用，不执行外域算法

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：LOCKED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `FORBIDDEN` / HTTP 403：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：本函数对应权限、回执、存储或运行权威检查失败（可重试）；调用方事务回滚，不返回半成功结果
- `INTEGRITY_ERROR` / HTTP 500：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果

- 副作用：写job/lease/lease_operations/outbox
- 授权：permission：service；rule：service所属owner worker
- 并发：本域单连接串行短事务；state/revision CAS，owner在途上限8；时间按RFC3339精确比较
- 幂等：同claim operationId返回原batch
- 依赖：`jobs.beginLeaseOperation`、`jobs.close`、`jobs.receipt`、`jobs.countActiveLeases`、`jobs.claimCandidates`、`jobs.enqueueState`、`jobs.saveReceipt`
- 其他依赖说明：私有SQLite

## jobs.StartJob

- 级别：约束
- 文件：`modules/jobs/internal/leases.go`
- 作用：确认开始执行

```go
func StartJob(ctx context.Context, op OperationContext, in StartJobInput) (StartJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `StartJobInput` | 确认开始执行；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StartJobResult` | 确认开始执行成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取可信runtime/control屏障并比对输入/op/runtime代次；维护或屏障期间拒绝开始新任务
2. 验活跃lease/fence
3. CAS leased→running
4. 写状态事件
5. 已持久cancel_requested拒绝开始；不能在owner心跳尚未读取取消前启动新任务执行

**错误**

- `STALE_RESULT` / HTTP 409：LEASE_LOST：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `FORBIDDEN` / HTTP 403：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：本函数对应权限、回执、存储或运行权威检查失败（可重试）；调用方事务回滚，不返回半成功结果
- `INTEGRITY_ERROR` / HTTP 500：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果

- 副作用：写running/outbox
- 授权：permission：service；rule：service所属owner
- 并发：fence严格相等
- 幂等：同fence重复start成功
- 依赖：`jobs.beginLeaseOperation`、`jobs.close`、`jobs.readLease`、`jobs.enqueueState`、`jobs.leaseLost`
- 其他依赖说明：私有SQLite

## jobs.HeartbeatJob

- 级别：约束
- 文件：`modules/jobs/internal/leases.go`
- 作用：维持租约

```go
func HeartbeatJob(ctx context.Context, op OperationContext, in HeartbeatJobInput) (HeartbeatJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HeartbeatJobInput` | 维持租约；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HeartbeatJobResult` | 维持租约成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取可信runtime和control屏障；核对job owner/active state/worker/fence/输入与当前epoch及未到期expiry
2. maintenance必须有control安全点请求和有效deadline；续租至min(now+30s,屏障deadline)，到期不能复活
3. 同owner/epoch/operationId按inputHash读取回执；同输入保留原expiry，不再次续租或追加进度；新的屏障截止更早时返回min(原expiry,当前deadline)作为有效执行权截止；同键不同输入返回CONFLICT
4. 可选progress在已验租约的同一事务调用AppendProgress；续租、进度、outbox与回执一起提交，任一失败全部回滚
5. 返回当前cancelRequested/safePointRequested，重复回执不能掩盖后发生的取消或屏障
6. cancelRequested取jobs.cancel_requested持久标志，即使其他状态字段尚未收敛也不能丢取消意图

**错误**

- `STALE_RESULT` / HTTP 409：LEASE_LOST：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `FORBIDDEN` / HTTP 403：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：本函数对应权限、回执、存储或运行权威检查失败（可重试）；调用方事务回滚，不返回半成功结果
- `INTEGRITY_ERROR` / HTTP 500：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `VALIDATION` / HTTP 400：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果

- 副作用：写expiry/lease_operations；可选写progress/outbox
- 授权：permission：service；rule：service所属owner
- 并发：过期后不允许复活
- 幂等：operationId/inputHash去重不延长原expiry；当前屏障可缩短有效expiry，返回标志取当前状态
- 依赖：`jobs.beginLeaseOperation`、`jobs.close`、`jobs.readLease`、`jobs.receipt`、`jobs.saveReceipt`、`jobs.AppendProgress`
- 其他依赖说明：私有SQLite

## jobs.CheckLease

- 级别：约束
- 文件：`modules/jobs/internal/leases.go`
- 作用：写回前检验执行权

```go
func CheckLease(ctx context.Context, op OperationContext, in CheckLeaseInput) (CheckLeaseResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CheckLeaseInput` | 写回前检验执行权；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CheckLeaseResult` | 写回前检验执行权成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取可信runtime与control屏障，核对输入/op/runtime/job epoch、所属owner、worker、fence和活动state
2. 租约expiry或control屏障截止到期均拒绝；维护缺少control安全点或deadline时失败关闭
3. 只返回安全的valid=true，不输出租约内部凭据；owner写回仍在本域CAS再次核对fence

**错误**

- `STALE_RESULT` / HTTP 409：LEASE_LOST：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：本函数对应权限、回执、存储或运行权威检查失败（可重试）；调用方事务回滚，不返回半成功结果
- `INTEGRITY_ERROR` / HTTP 500：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果

- 副作用：无
- 授权：permission：service；rule：service所属owner
- 并发：读后结果写必须在owner自己的CAS中再次fence校验
- 幂等：只读幂等
- 依赖：`jobs.beginLeaseOperation`、`jobs.close`、`jobs.readLease`
- 其他依赖说明：私有SQLite

## jobs.beginLeaseOperation

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：取得已核代次的运行视图和单连接业务事务

```go
func beginLeaseOperation(ctx context.Context, op OperationContext, epoch Identifier) (*leaseOperation, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `op` | `OperationContext` | 可信操作 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `epoch` | `Identifier` | 调用代次 | 仅本域内部使用，信任调用方已完成的边界校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `work` | `*leaseOperation` | 事务及资源锁 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `err` | `error` | 失败原因 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 取得已核代次的运行视图和单连接业务事务

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.close

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：回滚未提交事务并释放Store读锁

```go
func (work *leaseOperation) close()
```

**输入**

无

**输出**

无

**步骤**

1. 回滚未提交事务并释放Store读锁

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.readLease

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：在同事务核owner、epoch、worker、fence、活动状态与精确expiry

```go
func (work *leaseOperation) readLease(ctx context.Context, op OperationContext, in StartJobInput) (leaseRecord, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `op` | `OperationContext` | 可信操作 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `in` | `StartJobInput` | 租约标识 | 仅本域内部使用，信任调用方已完成的边界校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `lease` | `leaseRecord` | 租约状态、revision及原expiry | 仅本域内部使用，信任调用方已完成的边界校验 |
| `err` | `error` | 失败原因 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 在同事务核owner、epoch、worker、fence、活动状态与精确expiry
2. 读取持久cancel_requested标志，心跳直接返回此标志，不仅依赖可变state字符串

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：`jobs.leaseLost`

## jobs.claimCandidates

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：有界选择同owner/epoch最早到期的10个queued引用，保持精确时序

```go
func (work *leaseOperation) claimCandidates(ctx context.Context, in ClaimJobInput) ([]claimCandidate, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `in` | `ClaimJobInput` | 领取过滤 | 仅本域内部使用，信任调用方已完成的边界校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `jobs` | `[]claimCandidate` | 按notBefore/createdAt/jobId排序的引用 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `err` | `error` | 失败原因 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 按owner/epoch/queued及SQLite毫秒时间预筛到期候选超集，再由Go精确过滤未来notBefore
2. 按notBefore/createdAt/jobId有界插入，只保留最早10个引用；不把全量队列加载到内存
3. 回传持久rootOperationId；缺失根操作的历史未终局任务不参与领取，由回收事务隔离orphan并写outbox
4. cancel_requested=1的任务不参与领取，取消不能被调度状态覆盖

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.receipt

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：按owner/epoch/种类/operationId读取并核对租约回执hash

```go
func (work *leaseOperation) receipt(ctx context.Context, op OperationContext, kind string, request []byte) ([]byte, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `op` | `OperationContext` | 幂等作用域 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `kind` | `string` | claim/heartbeat/complete/fail/cancel | 仅本域内部使用，信任调用方已完成的边界校验 |
| `request` | `[]byte` | 规范化输入 | 仅本域内部使用，信任调用方已完成的边界校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `[]byte` | 原回执，未命中为nil | 仅本域内部使用，信任调用方已完成的边界校验 |
| `err` | `error` | 失败原因 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 按owner/epoch/种类/operationId读取并核对租约回执hash

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.saveReceipt

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：在调用方事务保存输入hash与成功结果回执

```go
func (work *leaseOperation) saveReceipt(ctx context.Context, op OperationContext, kind string, request []byte, result any) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `op` | `OperationContext` | 幂等作用域 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `kind` | `string` | claim/heartbeat/complete/fail/cancel | 仅本域内部使用，信任调用方已完成的边界校验 |
| `request` | `[]byte` | 规范化输入 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `result` | `LeaseReceiptResult` | 可编码的领取或心跳结果 | 仅本域内部使用，信任调用方已完成的边界校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 失败原因 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 在调用方事务保存输入hash与成功结果回执

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.enqueueState

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：在同事务调用平台outbox保存安全任务状态事件

```go
func (work *leaseOperation) enqueueState(jobID, owner, epoch, state string, revision int64) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `jobID` | `Identifier` | 任务 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `owner` | `string` | 所属服务 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `epoch` | `Identifier` | 数据代次 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `state` | `string` | 新状态 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `revision` | `int64` | 任务修订 | 仅本域内部使用，信任调用方已完成的边界校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 失败原因 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 在同事务调用平台outbox保存安全任务状态事件

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.leaseLost

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：构造无执行权的STALE_RESULT错误

```go
func leaseLost() error
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 安全错误 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 构造无执行权的STALE_RESULT错误

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.countActiveLeases

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：逐项精确比较租约时间并统计owner在途数量，避免数据库毫秒截断放宽并发上限

```go
func (work *leaseOperation) countActiveLeases(ctx context.Context, in ClaimJobInput) (int, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `in` | `ClaimJobInput` | owner/epoch | 仅本域内部使用，信任调用方已完成的边界校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `count` | `int` | 有效租约数量 | 仅本域内部使用，信任调用方已完成的边界校验 |
| `err` | `error` | 失败原因 | 仅本域内部使用，信任调用方已完成的边界校验 |

**步骤**

1. 逐项精确比较租约时间并统计owner在途数量，避免数据库毫秒截断放宽并发上限

**错误**

- 无

- 副作用：只承担登记步骤中的本域事务或资源操作
- 授权：permission：service；rule：由调用方已验操作上下文限定所属owner，不重新鉴权
- 并发：复用调用方串行SQLite事务及资源锁
- 幂等：由调用方决定事务提交与幂等边界
- 依赖：无

## jobs.NewLeaseHandlers

- 级别：参考
- 文件：`modules/jobs/internal/leases.go`
- 作用：只装配已统一鉴权和严格校验的worker六个租约HTTP请求

```go
func NewLeaseHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `EventHandlers` | 仅createJob的真实HTTP映射，复用本域处理器map内部类型 | 只交统一NewServiceHandler，不独立裸挂 |

**步骤**

1. claim正文绑定可选真实jobTypes；start/heartbeat/complete/fail从已校验路径取jobId，dataEpoch只取可信op。
2. checkLease只读取已校验workerId/fencingToken查询；调用原领域函数，不复制身份、租约和CAS规则。
3. 只返回原领域真实结果与安全Problem；完整jobs装配仍缺失时不宣称服务已就绪。

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：显式延迟属于去重材料，原成功不重新分配时刻
- 依赖：`jobs.ClaimJob`、`jobs.StartJob`、`jobs.HeartbeatJob`、`jobs.CheckLease`、`jobs.CompleteJob`、`jobs.FailJob`、`jobs.writeEventResponse`

## jobs.CompleteJob

- 级别：约束
- 文件：`modules/jobs/internal/results.go`
- 作用：提交成功结果

```go
func CompleteJob(ctx context.Context, op OperationContext, in CompleteJobInput) (CompleteJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CompleteJobInput` | 提交成功结果；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CompleteJobResult` | 提交成功结果成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 同事务核对本service所属owner、当前epoch、worker、持久fence与未过期租约；只接受running/cancel-requested
2. owner通过可选不可变结果引用确认已持久完成；无结果任务省略resultRef，存在时resultRef.owner须为本owner；不生成空ObjectRef
3. jobId/fence/worker及规范结果哈希重放相同成功，不同结果CONFLICT；operationId不同输入CONFLICT
4. CAS running/cancel-requested→succeeded；合并保留已有paidResultRefs、存resultRef、释放lease、写状态outbox与终局/操作回执一起提交

**错误**

- `STALE_RESULT` / HTTP 409：LEASE_LOST：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：RESULT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `VERSION_MISMATCH` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：操作键冲突、旧epoch或可信依赖不可用（可重试）
- `LOCKED` / HTTP 409：当前屏障已到安全点后不能新增操作回执（不可重试）

- 副作用：写result/state/lease/outbox
- 授权：permission：service；rule：service所属owner
- 并发：结果提交CAS；cancel与完成竞争以事务提交先后为准
- 幂等：jobId/fence/resultHash幂等，重复不同hash409
- 依赖：`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.readLease`、`jobs.terminalReplay`、`jobs.saveTerminal`、`jobs.mergePaidResults`、`jobs.finishJob`

## jobs.readJobRecord

- 级别：参考
- 文件：`modules/jobs/internal/results.go`
- 作用：同事务读取owner/epoch/state/revision/attempt/持久fence，任务不存在NOT_FOUND、旧epoch拒绝

```go
func (work *leaseOperation) readJobRecord(ctx context.Context, jobID string, epoch Identifier) (jobRecord, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域内部，信任入口已完成的校验 |
| `jobID` | `string` | 任务引用 | 本域内部，信任入口已完成的校验 |
| `epoch` | `Identifier` | 当前代次 | 本域内部，信任入口已完成的校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `job` | `jobRecord` | 状态记录 | 成功有效，失败无半结果 |
| `err` | `error` | 失败原因 | 成功有效，失败无半结果 |

**步骤**

1. 同事务读取owner/epoch/state/revision/attempt/持久fence，任务不存在NOT_FOUND、旧epoch拒绝

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：调用方负责可信身份；只落实本函数资源核对
- 并发：复用调用方短事务，不嵌套事务
- 幂等：由调用方事务及回执保证
- 依赖：无

## jobs.terminalReplay

- 级别：参考
- 文件：`modules/jobs/internal/results.go`
- 作用：核owner及当前fence，按操作键绑定输入，并按job/fence核终局worker/输入hash；命中时解码原响应

```go
func (work *leaseOperation) terminalReplay(ctx context.Context, op OperationContext, job jobRecord, in StartJobInput, kind string, request []byte, result any) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域内部，信任入口已完成的校验 |
| `op` | `OperationContext` | 可信owner与操作键 | 本域内部，信任入口已完成的校验 |
| `job` | `jobRecord` | 本事务当前任务 | 本域内部，信任入口已完成的校验 |
| `in` | `StartJobInput` | worker/fence | 本域内部，信任入口已完成的校验 |
| `kind` | `string` | 终局类型 | 本域内部，信任入口已完成的校验 |
| `request` | `[]byte` | 规范输入 | 本域内部，信任入口已完成的校验 |
| `result` | `LeaseReceiptResult` | 成功回执输出指针 | 本域内部，信任入口已完成的校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `found` | `bool` | 是否重放 | 成功有效，失败无半结果 |
| `err` | `error` | 失败原因 | 成功有效，失败无半结果 |

**步骤**

1. 核owner及当前fence，按操作键绑定输入，并按job/fence核终局worker/输入hash；命中时解码原响应
2. 新操作键命中同fence原终局时也登记该键并提交回执，不能留下未绑定操作键
3. 既有操作回执只读重放；新操作键补写前必须核当前维护安全点与屏障截止，缺权威失败关闭、截止后LOCKED，不能在safe水位后修改active回执

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：调用方负责可信身份；只落实本函数资源核对
- 并发：复用调用方短事务，不嵌套事务
- 幂等：由调用方事务及回执保证
- 依赖：`jobs.receipt`、`jobs.saveReceipt`

## jobs.saveTerminal

- 级别：参考
- 文件：`modules/jobs/internal/results.go`
- 作用：在同一事务保存job/fence/worker终局回执及operationId回执

```go
func (work *leaseOperation) saveTerminal(ctx context.Context, op OperationContext, in StartJobInput, kind string, request []byte, result any) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域内部，信任入口已完成的校验 |
| `op` | `OperationContext` | 操作键 | 本域内部，信任入口已完成的校验 |
| `in` | `StartJobInput` | worker/fence | 本域内部，信任入口已完成的校验 |
| `kind` | `string` | 终局类型 | 本域内部，信任入口已完成的校验 |
| `request` | `[]byte` | 规范输入 | 本域内部，信任入口已完成的校验 |
| `result` | `LeaseReceiptResult` | 成功结果 | 本域内部，信任入口已完成的校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 失败原因 | 成功有效，失败无半结果 |

**步骤**

1. 在同一事务保存job/fence/worker终局回执及operationId回执

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：调用方负责可信身份；只落实本函数资源核对
- 并发：复用调用方短事务，不嵌套事务
- 幂等：由调用方事务及回执保证
- 依赖：`jobs.saveReceipt`

## jobs.mergePaidResults

- 级别：参考
- 文件：`modules/jobs/internal/results.go`
- 作用：读取并去重合并已有付费引用，保留已有主结果；成功时写不可变主结果JSON和哈希，不读取正文

```go
func (work *leaseOperation) mergePaidResults(ctx context.Context, jobID string, refs []string, result *ObjectRef) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域内部，信任入口已完成的校验 |
| `jobID` | `string` | 任务引用 | 本域内部，信任入口已完成的校验 |
| `refs` | `[]string` | 已有付费结果引用 | 本域内部，信任入口已完成的校验 |
| `result` | `*ObjectRef` | 成功结果引用，可空 | 本域内部，信任入口已完成的校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 失败原因 | 成功有效，失败无半结果 |

**步骤**

1. 读取并去重合并已有付费引用，保留已有主结果；成功时写不可变主结果JSON和哈希，不读取正文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：调用方负责可信身份；只落实本函数资源核对
- 并发：复用调用方短事务，不嵌套事务
- 幂等：由调用方事务及回执保证
- 依赖：无

## jobs.finishJob

- 级别：参考
- 文件：`modules/jobs/internal/results.go`
- 作用：按revision/epoch/fence CAS更新状态及安全错误码，释放租约并写同事务状态outbox

```go
func (work *leaseOperation) finishJob(ctx context.Context, job jobRecord, state string, retryAt *time.Time, errorCode string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域内部，信任入口已完成的校验 |
| `job` | `jobRecord` | 本事务当前任务 | 本域内部，信任入口已完成的校验 |
| `state` | `string` | 目标状态 | 本域内部，信任入口已完成的校验 |
| `retryAt` | `*time.Time` | 重试时间，可空 | 本域内部，信任入口已完成的校验 |
| `errorCode` | `string` | 公共安全错误码 | 本域内部，信任入口已完成的校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 失败原因 | 成功有效，失败无半结果 |

**步骤**

1. 按revision/epoch/fence CAS更新状态及安全错误码，释放租约并写同事务状态outbox

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：调用方负责可信身份；只落实本函数资源核对
- 并发：复用调用方短事务，不嵌套事务
- 幂等：由调用方事务及回执保证
- 依赖：`jobs.enqueueState`

## jobs.ClassifyFailure

- 级别：参考
- 文件：`modules/jobs/internal/retry.go`
- 作用：固定错误重试分类

```go
func ClassifyFailure(ctx context.Context, op OperationContext, in ClassifyFailureInput) (ClassifyFailureResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ClassifyFailureInput` | 固定错误重试分类；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ClassifyFailureResult` | 固定错误重试分类成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只接受公共errors.json登记码，未知码VALIDATION
2. CANCELLED必须有持久cancelRequested才返回cancelled，否则VALIDATION
3. 已持久请求取消但未报告CANCELLED的失败一律orphan，不把未明安全状态伪造cancelled或重排
4. 未请求取消时，DEPENDENCY_UNAVAILABLE/DEADLINE_EXCEEDED且attempt<5可重试；其余及耗尽为dead-letter

**错误**

- `VALIDATION` / HTTP 400：INVALID_ERROR_CODE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数
- 幂等：同code/attempt同结果
- 依赖：无
- 其他依赖说明：公共errors.json

## jobs.ComputeRetryAt

- 级别：参考
- 文件：`modules/jobs/internal/retry.go`
- 作用：计算有限重试延迟

```go
func ComputeRetryAt(ctx context.Context, op OperationContext, in ComputeRetryAtInput) (ComputeRetryAtResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ComputeRetryAtInput` | 计算有限重试延迟；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ComputeRetryAtResult` | 计算有限重试延迟成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. attempt1..4查5/30/120/600秒
2. 加hash(jobId,attempt)%1000ms确定抖动
3. attempt5无重试

**错误**

- `VALIDATION` / HTTP 400：ATTEMPTS_EXHAUSTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数
- 幂等：同jobId/attempt/now同结果
- 依赖：无

## jobs.RequeueDue

- 级别：参考
- 文件：`modules/jobs/internal/scheduler.go`
- 作用：到期重排

```go
func RequeueDue(ctx context.Context, op OperationContext, in RequeueDueInput) (RequeueDueResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RequeueDueInput` | 到期重排；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RequeueDueResult` | 到期重排成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅jobs后台可信服务上下文，维护或control屏障期间不调度
2. 选当前epoch精确notBefore到期的retry-wait，每批min(batchSize,100)
3. revision CAS queued并写状态outbox，不增加attempt、不改rootOperationId/inputRef/paidRefs
4. 只重排cancel_requested=0；历史带取消标志的queued/retry-wait由回收隔离，不重新执行

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写queued
- 授权：permission：service；rule：service worker
- 并发：批次事务
- 幂等：重复tick幂等
- 依赖：`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.enqueueState`
- 其他依赖说明：私有SQLite

## jobs.ReapExpiredLeases

- 级别：参考
- 文件：`modules/jobs/internal/scheduler.go`
- 作用：处理失联worker

```go
func ReapExpiredLeases(ctx context.Context, op OperationContext, in ReapExpiredLeasesInput) (ReapExpiredLeasesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReapExpiredLeasesInput` | 处理失联worker；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReapExpiredLeasesResult` | 处理失联worker成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅jobs后台可信服务上下文；维护或control屏障期间暂停active回收写入
2. 每批最多100个当前epoch精确到期的活动租约或历史缺根未终局任务，CAS state为orphan、job.fencing_counter与revision各增1，删除旧租约、保留worker/旧fence恢复引用、写状态outbox同事务提交；历史queued/retry-wait带取消标记同样隔离，不能再执行
3. 保留相同job/inputRef/rootOperationId和全部paidResultRefs；orphan不被Claim领取，旧worker旧fence不能写回
4. 事务外调用受信OwnerRecoveryChecker：completed确认持久结果并提供可选resultRef/paidRefs；safeToRetry确认无未决付费且checkpoint/根operation可复用；pending/unknown或不可达保持orphan
5. 重新获取当前epoch/control权威并CAS同orphan/fence：completed复用结果为succeeded；safeToRetry按attempt1..4有限退避或耗尽dead-letter，已有cancel请求则确认cancelled；状态/refs/outbox同事务提交
6. 实际owner版本化适配未接通时只隔离并保持orphan；启动装配仍blocked，不宣称恢复链路可用
7. 历史任务若缺rootOperationId保持orphan，completed可复用已完成结果；不得猜测旧operationId就是根来自动重试

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写lease/state/outbox
- 授权：permission：service；rule：service worker
- 并发：短事务隔离旧fence，owner HTTP检查在事务外；恢复结果仅以当前epoch/orphan/fence CAS应用
- 幂等：jobId/oldFence去重
- 依赖：`jobs.collectOrphans`、`jobs.reconcileOrphan`

## jobs.TickSchedules

- 级别：参考
- 文件：`modules/jobs/internal/scheduler.go`
- 作用：定时生成任务

```go
func TickSchedules(ctx context.Context, op OperationContext, in TickSchedulesInput) (TickSchedulesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `TickSchedulesInput` | 定时生成任务；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TickSchedulesResult` | 定时生成任务成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 维护时暂停
2. 对到期schedule计算最后一到期tick
3. 只合并一次，dedupe scheduleId/tick/epoch
4. 事务推进nextAt

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：QUEUE_FULL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：调用方不是jobs后台（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写jobs/schedule tick
- 授权：permission：service；rule：service scheduler
- 并发：schedule revision CAS，多scheduler不双造任务
- 幂等：tick dedupeKey幂等
- 依赖：`jobs.CreateJob`、`jobs.NextFireAt`、`jobs.lastDueTick`

## jobs.lastDueTick

- 级别：参考
- 文件：`modules/jobs/internal/scheduler.go`
- 作用：计算到期定时的最后一次触发

```go
func lastDueTick(expression string, timezone string, nextAt time.Time, now time.Time) (time.Time, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `expression` | `string` | 已保存的定时表达式 | 仅本域内部使用 |
| `timezone` | `string` | 已保存的IANA时区 | 仅本域内部使用 |
| `nextAt` | `time.Time` | 当前保存的下次触发 | 仅本域内部使用 |
| `now` | `time.Time` | 本次扫描时刻 | 仅本域内部使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tick` | `time.Time` | 最后一次已到期触发 | 成功有效，失败不返回半结果 |
| `ok` | `bool` | nextAt尚未到期时为false | 成功有效，失败不返回半结果 |
| `err` | `error` | 表达式或记录无法证明触发时刻 | 成功有效，失败不返回半结果 |

**步骤**

1. nextAt晚于now时未到期
2. 在366天窗口内按与NextFireAt相同的本地分钟规则取最后一次匹配
3. 窗口之外只接受原nextAt本身仍匹配的时刻

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅TickSchedules使用
- 并发：纯函数，迭代上限527040
- 幂等：同表达式、时区、nextAt和now得到同一次tick
- 依赖：`jobs.NextFireAt`

## jobs.collectOrphans

- 级别：参考
- 文件：`modules/jobs/internal/scheduler.go`
- 作用：同事务隔离最多100个过期租约、递增持久fence/写outbox，并读取最多100个orphan安全引用后提交

```go
func collectOrphans(ctx context.Context, op OperationContext, in ReapExpiredLeasesInput) ([]orphanCandidate, int, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用 |
| `op` | `OperationContext` | jobs服务上下文 | 仅本域内部使用 |
| `in` | `ReapExpiredLeasesInput` | 当前代次及tick时间 | 仅本域内部使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `items` | `[]orphanCandidate` | 本批恢复检查引用 | 成功有效，失败无半结果 |
| `count` | `int` | 本批隔离数量 | 成功有效，失败无半结果 |
| `err` | `error` | 失败原因 | 成功有效，失败无半结果 |

**步骤**

1. 同事务隔离最多100个过期租约或根未知历史任务、递增持久fence/写outbox，并轮转读取最多100个orphan安全引用后提交
2. 历史queued/retry-wait若cancel_requested=1，也以同事务fence失效/orphan/outbox隔离，等待owner确认安全结束

**错误**

- 无

- 副作用：登记步骤中的本域事务变化
- 授权：permission：service；rule：仅jobs可信后台上下文；owner结果经受信恢复边界确认
- 并发：短事务CAS，网络调用不持有事务
- 幂等：job/epoch/fence CAS，重复tick无重复状态事件
- 依赖：`jobs.beginLeaseOperation`、`jobs.enqueueState`

## jobs.reconcileOrphan

- 级别：参考
- 文件：`modules/jobs/internal/scheduler.go`
- 作用：重新核当前epoch/control，按orphan/fence CAS应用owner结果或安全有限重试，保留付费引用，不接受未知状态假成功

```go
func reconcileOrphan(ctx context.Context, op OperationContext, candidate orphanCandidate, recovery OwnerRecoveryResult) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部使用 |
| `op` | `OperationContext` | jobs服务上下文 | 仅本域内部使用 |
| `candidate` | `orphanCandidate` | 已失效租约恢复引用 | 仅本域内部使用 |
| `recovery` | `OwnerRecoveryResult` | 可信owner持久确认 | 仅本域内部使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `changed` | `bool` | 是否已收敛状态 | 成功有效，失败无半结果 |
| `err` | `error` | 失败原因 | 成功有效，失败无半结果 |

**步骤**

1. 重新核当前epoch/control，按orphan/fence CAS应用owner结果或安全有限重试，保留付费引用，不接受未知状态假成功

**错误**

- 无

- 副作用：登记步骤中的本域事务变化
- 授权：permission：service；rule：仅jobs可信后台上下文；owner结果经受信恢复边界确认
- 并发：短事务CAS，网络调用不持有事务
- 幂等：job/epoch/fence CAS，重复tick无重复状态事件
- 依赖：`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.ClassifyFailure`、`jobs.ComputeRetryAt`、`jobs.mergePaidResults`、`jobs.finishJob`

## jobs.RequestCancel

- 级别：约束
- 文件：`modules/jobs/internal/cancellation.go`
- 作用：请求取消任务

```go
func RequestCancel(ctx context.Context, op OperationContext, in RequestCancelInput) (RequestCancelResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RequestCancelInput` | 请求取消任务；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RequestCancelResult` | 请求取消任务成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 任务所属服务（服务身份且主体=调用方=持久owner）直接放行；否则从持久job owner/type/epoch映射gen.JobTypes.managementPermission并请求identity最新具体功能授权，job.manage只作入口粗能力；进入事务后重核已授权owner/type/epoch
2. 核对当前epoch并读任务；queued/retry-wait立即cancelled，leased/running写cancel-requested，终局与既有cancel-requested原样返回；orphan只持久cancelRequested意图并等待owner安全确认，不伪造cancelled
3. 与完成以同事务revision CAS竞争；同operationId保存输入hash及原响应
4. 状态变更、outbox与回执同事务提交；不代替owner调用模型取消

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：JOB_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `VERSION_MISMATCH` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：操作键冲突、旧epoch或可信依赖不可用（可重试）

- 副作用：写cancel状态/outbox
- 授权：permission：job.manage；rule：job.manage入口粗能力；终点按持久任务类型登记的managementPermission实时核权，不用用户自报owner/type；任务所属服务以自己的服务身份取消自己名下的任务时不需要管理员授权
- 并发：CAS与完成竞争，完成先提交则返回succeeded
- 幂等：operationId幂等
- 依赖：`identity:checkAuthorization`、`jobs.authorizeJobManagement`、`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.receipt`、`jobs.saveReceipt`、`jobs.enqueueState`

## jobs.ConfirmCancelled

- 级别：参考
- 文件：`modules/jobs/internal/cancellation.go`
- 作用：owner确认安全取消

```go
func ConfirmCancelled(ctx context.Context, op OperationContext, work *leaseOperation, in ConfirmCancelledInput) (ConfirmCancelledResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `work` | `*leaseOperation` | FailJob已建立并验证当前租约的事务 | 内部句柄，不嵌套事务，不重新鉴权 |
| `in` | `ConfirmCancelledInput` | owner确认安全取消；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConfirmCancelledResult` | owner确认安全取消成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 信任FailJob已验证的当前租约与cancel-requested状态及owner取消确认
2. 同事务保留paidResultRefs并CAS cancelled、释放lease、写状态outbox；由FailJob写回执及提交

**错误**

- `STALE_RESULT` / HTTP 409：LEASE_LOST：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：UNSAFE_CANCEL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写状态/结果refs
- 授权：permission：service；rule：service所属owner
- 并发：不能以超时假设供应方未收费
- 幂等：jobId/fence幂等
- 依赖：`jobs.mergePaidResults`、`jobs.finishJob`
- 其他依赖说明：model-gateway.cancelInference 经owner调用

## jobs.ReadJob

- 级别：约束
- 文件：`modules/jobs/internal/query.go`
- 作用：读取管理任务状态

```go
func ReadJob(ctx context.Context, op OperationContext, in ReadJobInput) (ReadJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReadJobInput` | 读取管理任务状态；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadJobResult` | 读取管理任务状态成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 从持久job owner/type/epoch映射gen.JobTypes.managementPermission并请求identity最新具体功能授权，job.manage只作入口粗能力；进入事务后重核已授权owner/type/epoch
2. 核对当前epoch，在同一SQLite快照读取任务状态、revision及已持久结果引用
3. 不打开正文，不返回任务输入或原始错误

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：JOB_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `VERSION_MISMATCH` / HTTP 409：操作键冲突、旧epoch或可信依赖不可用（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：操作键冲突、旧epoch或可信依赖不可用（可重试）

- 副作用：无
- 授权：permission：job.manage；rule：管理员按持久owner/type当前功能权限；纯服务只准actor.kind=service、subjectId=callerService=持久owner，真实启动siteID与actor.siteId、当前epoch一致且owner/type登记。服务读取不替代管理权限，不读取他域或跨站任务。
- 并发：SQLite快照读
- 幂等：GET幂等
- 依赖：`identity:checkAuthorization`、`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.authorizeJobRead`

## jobs.authorizeJobManagement

- 级别：参考
- 文件：`modules/jobs/internal/query.go`
- 作用：按持久任务类型映射具体功能权限，事务外请求最新identity授权并返回作用域

```go
func authorizeJobManagement(ctx context.Context, op OperationContext, jobID string) (jobRecord, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域内部，信任入口已完成的校验 |
| `op` | `OperationContext` | 可信actor | 本域内部，信任入口已完成的校验 |
| `jobID` | `string` | 审计资源 | 本域内部，信任入口已完成的校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `job` | `jobRecord` | 已获具体功能权限授权的持久owner/type/epoch作用域 | 仅本域内部；调用方进事务后仍核对任务身份与epoch |
| `err` | `error` | 权限、任务、代次或依赖失败 | 成功nil，失败不返回半结果 |

**步骤**

1. 在Store进程读锁内按jobId从私有库读取owner/type/epoch，核本次op.dataEpoch；不采用客户端owner/type
2. 从gen.JobTypes按持久owner/type精确匹配managementPermission，缺映射失败关闭；job.manage仅入口粗能力，不作为最终业务授权
3. 读取客户端引用后释放Store锁，再调用identity按可信actor请求具体managementPermission；网络调用不持有Store锁或SQLite事务
4. 成功返回已授权持久作用域，由ReadJob/RequestCancel在后续事务重核owner/type/epoch；未知响应/拒绝/不可达不授权

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：调用方负责可信身份；只落实本函数资源核对
- 并发：Store锁仅覆盖本域短读及客户端取值，外部identity调用不持有Store锁或SQLite事务
- 幂等：由调用方事务及回执保证
- 依赖：`jobs.authorizePermission`、`jobs.jobManagementPermission`

## jobs.authorizePermission

- 级别：参考
- 文件：`modules/jobs/internal/query.go`
- 作用：用本域版本化identity客户端作最新具体功能授权；调用方必须已释放存储锁

```go
func authorizePermission(ctx context.Context, op OperationContext, client *gen.IdentityClient, permission string, resourceID string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |
| `op` | `OperationContext` | 可信actor | 仅本域内部，信任边界已验证字段 |
| `client` | `*IdentityClient` | 已读取生成客户端 | 仅本域内部，信任边界已验证字段 |
| `permission` | `string` | 持久目录得出的具体业务权限 | 仅本域内部，信任边界已验证字段 |
| `resourceID` | `string` | 审计资源 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 权限/依赖失败 | 成功有效，失败不返回半结果 |

**步骤**

1. 用本域版本化identity客户端作最新具体功能授权；调用方必须已释放存储锁

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：`identity:checkAuthorization`

## jobs.jobManagementPermission

- 级别：参考
- 文件：`modules/jobs/internal/query.go`
- 作用：从生成目录精确匹配持久任务owner/type的具体权限

```go
func jobManagementPermission(owner string, jobType string) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `owner` | `string` | 持久owner | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `jobType` | `string` | 持久type | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `permission` | `string` | 生成目录具体管理权限 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 从生成目录精确匹配持久任务owner/type的具体权限

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：无

## jobs.authorizeJobRead

- 级别：参考
- 文件：`modules/jobs/internal/query.go`
- 作用：本域纯服务只读任务归属，管理访问继续当前授权

```go
func authorizeJobRead(ctx context.Context, op OperationContext, jobID string) (jobRecord,error)
```

**输入**

无

**输出**

无

**步骤**

1. 纯服务从当前真实Store.siteID核站点并按持久owner/type/epoch核本人所属及登记配对。
2. 管理员完整复用authorizeJobManagement，未知身份与配置不作默认允许。

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：显式延迟属于去重材料，原成功不重新分配时刻
- 依赖：`jobs.currentStore`、`jobs.authorizeJobManagement`、`jobs.jobManagementPermission`

## jobs.AppendProgress

- 级别：参考
- 文件：`modules/jobs/internal/progress.go`
- 作用：持久进度安全点

```go
func AppendProgress(ctx context.Context, op OperationContext, tx *ProgressTransaction, in AppendProgressInput) (AppendProgressResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `tx` | `*ProgressTransaction` | HeartbeatJob已核对owner/epoch/worker/fence及有效expiry的同一事务 | 仅本域内部调用；不新开事务，不提交调用方事务，不重复入口校验 |
| `in` | `AppendProgressInput` | 持久进度安全点；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AppendProgressResult` | 持久进度安全点成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 信任调用方同事务有效租约检查与入口安全phase代码、非负计数校验；验证completed≤total的业务关系
2. 按jobId/checkpointKey和inputHash查重；同输入返回原seq，异输入CONFLICT
3. 同job同phase completed不能低于历史最大值；新phase允许从0开始
4. 分配单调seq，同事务保存progress与jobs.job-progress.v1 outbox；调用方统一提交
5. 同一次platform.Now写progress.created_at与对应outbox.occurredAt，不伪造legacy发生时间

**错误**

- `STALE_RESULT` / HTTP 409：LEASE_LOST：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PROGRESS：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：本函数对应权限、回执、存储或运行权威检查失败（不可重试）；调用方事务回滚，不返回半成功结果

- 副作用：同调用方事务写progress/outbox
- 授权：permission：service；rule：service所属owner
- 并发：同一任务事务串行；同phase completed历史最大值不得倒退，seq全任务单增
- 幂等：checkpointKey幂等不重复seq
- 依赖：无

## jobs.StreamProgress

- 级别：约束
- 文件：`modules/jobs/internal/http_progress.go`
- 作用：任务进度SSE

```go
func StreamProgress(ctx context.Context, op OperationContext, in StreamProgressInput) (StreamProgressResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `StreamProgressInput` | 任务进度SSE；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StreamProgressResult` | 任务进度SSE成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅当前后台主体；从持久job owner/type/epoch匹配生成ManagementPermission并在锁外identity核当前权限，不使用正文actor或入口job.manage代替
2. 打开即核任务作用域、epoch和afterSequence范围；0首次只读当前7天保留项，正游标不得跳未来水位或缺失/过期进度；旧created_at只从唯一一致持久源时间回填，未知/矛盾失败关闭
3. 按actor/site全连接最多10个，归还幂等；每次Next重新核当前具体任务权限和epoch后短事务最多64行读取，只输出jobId/seq/phase/计数，不读取输入、结果正文或原始错误
4. 直接返回platform.JSONEventStream，SSEFrame.Event固定progress，ID为seq十进制，Data仅ProgressEvent；公共ServeJSONEventStream负责15秒心跳/单帧背压/30秒写截止
5. 无数据不持SQL/进程锁等待；终局且已追平返回EOF；慢client按公共写截止断开，seq可精确续接；权限/epoch/原请求ctx截止/取消或断连关闭并归还槽，不延长原委托，不在响应开始后写Problem

**错误**

- `FORBIDDEN`：当前具体权限不足（不可重试）
- `UNAUTHENTICATED`：当前会话失效（不可重试）
- `NOT_FOUND`：任务不存在（不可重试）
- `VALIDATION`：游标冲突、超水位、缺失或过期（不可重试）
- `VERSION_MISMATCH`：数据代次或作用域变化（不可重试）
- `RATE_LIMITED`：当前actor已10流（不可重试）
- `INTEGRITY_ERROR`：进度时刻或安全材料损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威或私有存储未知（可重试）
- `INTERNAL`：私有IO失败（可重试）
- `CONFLICT`：当前任务授权scope在核权与读取之间改变（不可重试）

- 副作用：仅本进程连接配额和流句柄，不写业务progress或投递确认
- 授权：permission：job.manage；rule：打开及每次继续读取从持久任务目录取得当前具体业务管理权限，经identity即时核权；job.manage仅入口粗能力
- 并发：actor/site最多10流；域内每次最多64行内存，公共单帧背压，无额外异步队列；SQL/进程锁外核权与等待
- 幂等：三个游标别名同时提供须相等；只读seq精确重放，流不确认/修改progress，不编造事件
- 依赖：`jobs.authorizeJobManagement`、`jobs.readProgressFrame`、`jobs.progressStream.Close`、`jobs.progressStream.Next`
- 其他依赖说明：私有progress表

## jobs.progressStream.Next

- 级别：参考
- 文件：`modules/jobs/internal/http_progress.go`
- 作用：每批当前具体权限/epoch复核，最多64行扫描后输出单条安全JSON，空闲无锁等待

```go
func (stream *progressStream) Next(ctx context.Context) (SSEFrame, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `stream` | `*progressStream` | 单连接句柄 | 仅当前授权链路或可信启动装配，取消原样传IO |
| `ctx` | `context.Context` | 原请求取消与更早截止 | 仅当前授权链路或可信启动装配，取消原样传IO |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `frame` | `SSEFrame` | 安全ProgressEvent JSON及十进制seq | 仅当前授权链路或可信启动装配，取消原样传IO |
| `error` | `error` | 失败时结果无效，响应开始后仅结束流 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 串行按调用ctx读取，并由Close取消在途IO；每个最多64行批次重核具体权限/epoch
2. 只更新本连接读取位置，不修改progress或确认水位；无数据在事务外等1秒，追平终局返回EOF
3. 权限/epoch/原截止/取消/关闭即归还槽；单帧背压与慢client断开交平台，不造进度或关闭后的Problem

**错误**

- `FORBIDDEN`：当前具体任务权限不足（不可重试）
- `NOT_FOUND`：目标任务不存在（不可重试）
- `VALIDATION`：进度游标缺失/过期/超水位（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：任务授权作用域变化（不可重试）
- `INTEGRITY_ERROR`：进度时刻或源材料不可证明（不可重试）
- `DEPENDENCY_UNAVAILABLE`：授权或运行权威未知（可重试）
- `INTERNAL`：私有IO失败（可重试）

- 副作用：无
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：`jobs.readProgressFrame`、`jobs.progressStream.Close`

## jobs.progressStream.Close

- 级别：参考
- 文件：`modules/jobs/internal/http_progress.go`
- 作用：关闭流、取消在途读取并只归还一次actor槽

```go
func (stream *progressStream) Close() error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `stream` | `*progressStream` | 单连接句柄 | 仅当前授权链路或可信启动装配，取消原样传IO |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时结果无效，响应开始后仅结束流 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 关闭流、取消在途读取并只归还一次actor槽

**错误**

- 无

- 副作用：幂等取消本连接读取并归还一次actor/site连接计数
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：无

## jobs.readProgressFrame

- 级别：参考
- 文件：`modules/jobs/internal/http_progress.go`
- 作用：锁外核当前任务权限，事务核scope/epoch并扫描最多64行段，7天窗口精确复核

```go
func readProgressFrame(ctx context.Context, op OperationContext, jobID string, after int64, initial bool) (progressRead, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 原请求取消与更早截止 | 仅当前授权链路或可信启动装配，取消原样传IO |
| `op` | `OperationContext` | 可信主体与epoch | 仅当前授权链路或可信启动装配，取消原样传IO |
| `jobID` | `string` | 目标任务 | 仅当前授权链路或可信启动装配，取消原样传IO |
| `after` | `int64` | 上次已读进度位置 | 仅当前授权链路或可信启动装配，取消原样传IO |
| `initial` | `bool` | 首次可跳已过期历史 | 仅当前授权链路或可信启动装配，取消原样传IO |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `snapshot` | `progressRead` | 一次最多64行安全读取的单帧/扫描位置 | 仅当前授权链路或可信启动装配，取消原样传IO |
| `error` | `error` | 失败时结果无效，响应开始后仅结束流 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 读取前在Store锁外按持久owner/type向identity核当前具体权限；随后短事务重新核scope与epoch
2. MAX(seq)为当前已登记水位，正游标不能跳未来；最多64行按seq读取，RFC3339Nano解析后按实际时刻与当前7天截止比较，不做TEXT日期边界比较
3. 初始0跳已知过期项，未知/矛盾created_at完整性失败；正游标遇缺失seq或过期段返回VALIDATION，不跳过未能续接的段
4. 只返回第一条安全ProgressEvent；当前批次全过期且尚有后续时释放事务再回Next重核权限取下一批，无SQL锁内网络或等待

**错误**

- `FORBIDDEN`：当前具体任务权限不足（不可重试）
- `NOT_FOUND`：目标任务不存在（不可重试）
- `VALIDATION`：进度游标缺失/过期/超水位（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：任务授权作用域变化（不可重试）
- `INTEGRITY_ERROR`：进度时刻或源材料不可证明（不可重试）
- `DEPENDENCY_UNAVAILABLE`：授权或运行权威未知（可重试）
- `INTERNAL`：私有IO失败（可重试）

- 副作用：无
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：`jobs.authorizeJobManagement`、`jobs.beginLeaseOperation`、`jobs.readJobRecord`

## jobs.NewProgressHandlers

- 级别：参考
- 文件：`modules/jobs/internal/http_progress.go`
- 作用：只把已实现进度SSE交公共完整入口，不造心跳/写截止

```go
func NewProgressHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `EventHandlers` | 仅streamProgress真实处理器 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 只把已实现进度SSE交公共完整入口，不造心跳/写截止

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：`jobs.bindProgressCursor`、`jobs.StreamProgress`、`jobs.writeEventResponse`、`platform.ServeJSONEventStream`

## jobs.bindProgressCursor

- 级别：参考
- 文件：`modules/jobs/internal/http_progress.go`
- 作用：统一afterSequence、兼容lastEventId及Last-Event-ID；不同值或int64溢出400

```go
func bindProgressCursor(r *http.Request) (int64, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `r` | `http.Request` | 已严格验证结构的HTTP请求 | 仅当前授权链路或可信启动装配，取消原样传IO |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `after` | `int64` | 统一的续接位置 | 仅当前授权链路或可信启动装配，取消原样传IO |
| `error` | `error` | 失败时结果无效，响应开始后仅结束流 | 仅当前授权链路或可信启动装配，取消原样传IO |

**步骤**

1. 统一afterSequence、兼容lastEventId及Last-Event-ID；不同值或int64溢出400

**错误**

- `VALIDATION`：别名不相等、溢出或非法序号（不可重试）

- 副作用：无
- 授权：permission：service；rule：可信装配或已授权进度流内部
- 并发：短事务；网络核权/等待不持数据库锁
- 幂等：只读原进度；配额幂等归还
- 依赖：无

## jobs.ValidateEnvelope

- 级别：参考
- 文件：`modules/jobs/internal/event_validation.go`
- 作用：验证跨域事件

```go
func ValidateEnvelope(ctx context.Context, op OperationContext, in ValidateEnvelopeInput) (ValidateEnvelopeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateEnvelopeInput` | 验证跨域事件；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateEnvelopeResult` | 验证跨域事件成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 生成EventTypes按eventType/version匹配唯一producer，既要等于event.producer也要等于可信op.callerService；拒未知类型/版本
2. 只调用platform.NewPayloadValidator编译内嵌无联网引用规格，按严格payloadSchema校验，不复制schema机制
3. event.dataEpoch须等于入口可信op.dataEpoch；其余通用信封字段由严格入口校验，PublishEvent再在同一事务核当前runtime/control

**错误**

- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：生产者身份、生成规格或续传游标不满足登记约束（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：生产者身份、生成规格或续传游标不满足登记约束（可重试）
- `INTEGRITY_ERROR` / HTTP 422：生产者身份、生成规格或续传游标不满足登记约束（不可重试）

- 副作用：无
- 授权：permission：service；rule：service生产方
- 并发：纯校验+runtime读取
- 幂等：同event同结果
- 依赖：`jobs.findEventDefinition`、`jobs.validateEventPayload`
- 其他依赖说明：各域contracts/events.json

## jobs.findEventDefinition

- 级别：参考
- 文件：`modules/jobs/internal/event_validation.go`
- 作用：仅从生成EventTypes精确匹配唯一生产方、版本、权限与订阅

```go
func findEventDefinition(name string, version int) (EventDefinition, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `name` | `string` | 事件类型 | 仅本域内部，信任边界已验证字段 |
| `version` | `int` | 事件版本 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `definition` | `EventDefinition` | 唯一生成目录项 | 成功有效，失败不返回半结果 |
| `err` | `error` | 未知类型/版本 | 成功有效，失败不返回半结果 |

**步骤**

1. 仅从生成EventTypes精确匹配唯一生产方、版本、权限与订阅

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.validateEventPayload

- 级别：参考
- 文件：`modules/jobs/internal/event_validation.go`
- 作用：按事件名/版本缓存平台编译器，仅调用platform.NewPayloadValidator，无外部联网引用

```go
func validateEventPayload(definition EventDefinition, payload []byte) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `definition` | `EventDefinition` | 生成规格 | 仅本域内部，信任边界已验证字段 |
| `payload` | `[]byte` | 载荷 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 严格schema错误 | 成功有效，失败不返回半结果 |

**步骤**

1. 按事件名/版本缓存平台编译器，仅调用platform.NewPayloadValidator，无外部联网引用

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：`platform.NewPayloadValidator`

## jobs.consumerHasEvent

- 级别：参考
- 文件：`modules/jobs/internal/event_validation.go`
- 作用：按生成目录精确检查consumer对该事件/版本的订阅范围

```go
func consumerHasEvent(consumer string, name string, version int) bool
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `consumer` | `string` | 服务消费者 | 仅本域内部，信任边界已验证字段 |
| `name` | `string` | 事件类型 | 仅本域内部，信任边界已验证字段 |
| `version` | `int` | 版本 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `registered` | `bool` | 是否静态登记 | 成功有效，失败不返回半结果 |

**步骤**

1. 按生成目录精确检查consumer对该事件/版本的订阅范围

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.PublishEvent

- 级别：约束
- 文件：`modules/jobs/internal/event_store.go`
- 作用：持久接收事件

```go
func PublishEvent(ctx context.Context, op OperationContext, in PublishEventInput) (PublishEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PublishEventInput` | 持久接收事件；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishEventResult` | 持久接收事件成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. ValidateEnvelope校验唯一生产者/版本/严格载荷/可信epoch
2. 标准JSON数值不经浮点转换，规范化payload并对完整不可变信封求hash，eventId同hash重放；不同producer/subject/revision等元数据同样CONFLICT
3. 同事务读/写event操作回执；新键不同信封CONFLICT，事件重复不重建delivery
4. 用持久全局event_sequence原子分配eventSeq，不因历史清理归零；写event并按生成目录为每个consumer建唯一queued delivery，无消费者也durable存档
5. 发布可在维护draining有效control截止前完成，safe水位后拒写；事件/全部delivery/回执一起提交后才accepted

**错误**

- `CONFLICT` / HTTP 409：EVENT_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `VERSION_MISMATCH` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `LOCKED` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `STALE_RESULT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `NOT_FOUND` / HTTP 404：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（可重试）
- `INTEGRITY_ERROR` / HTTP 422：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）

- 副作用：写event/deliveries
- 授权：permission：service；rule：service生产方
- 并发：单事务，不复制源数据库
- 幂等：eventId+规范完整envelope hash及caller/epoch/operationId绑定；相同事件不重复创建投递
- 依赖：`jobs.ValidateEnvelope`、`jobs.encodeEvent`、`jobs.beginLeaseOperation`、`jobs.readEventRecord`、`jobs.eventReceipt`、`jobs.saveEventReceipt`、`jobs.requireEventWrite`

## jobs.encodeEvent

- 级别：参考
- 文件：`modules/jobs/internal/event_store.go`
- 作用：使用JSON Number保留整数精度，标准JSON键排序规范化payload后编码完整信封并哈希

```go
func encodeEvent(event EventEnvelope) ([]byte, Hash)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `event` | `EventEnvelope` | 已验信封 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `encoded` | `[]byte` | 规范不可变信封JSON | 成功有效，失败不返回半结果 |
| `hash` | `Hash` | 完整信封哈希 | 成功有效，失败不返回半结果 |

**步骤**

1. 使用JSON Number保留整数精度，标准JSON键排序规范化payload后编码完整信封并哈希

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.readEventRecord

- 级别：参考
- 文件：`modules/jobs/internal/event_store.go`
- 作用：读取事件固定信封及seq，核持久JSON/哈希并按当前epoch隔离

```go
func (work *leaseOperation) readEventRecord(ctx context.Context, eventID string) (eventRecord, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |
| `eventID` | `string` | 不可变事件ID | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `event` | `eventRecord` | 持久事件及seq/hash | 成功有效，失败不返回半结果 |
| `err` | `error` | 不存在/损坏 | 成功有效，失败不返回半结果 |

**步骤**

1. 读取事件固定信封及seq，核持久JSON/哈希并按当前epoch隔离
2. 旧envelope_hash为空时还须核生成目录唯一producer、信封Identifier/非负revision/时间基本完整性，才验证payload并计算hash；未知或损坏失败关闭，不把旧producer复制到可信目录

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.requireEventWrite

- 级别：参考
- 文件：`modules/jobs/internal/event_store.go`
- 作用：发布/确认只可在有效control截止前收敛；其余调度/重放在maintenance或safePoint拒绝

```go
func (work *leaseOperation) requireEventWrite(draining bool) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `draining` | `bool` | 是否只收敛既有发布/确认 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 屏障/维护拒绝 | 成功有效，失败不返回半结果 |

**步骤**

1. 发布/确认只可在有效control截止前收敛；其余调度/重放在maintenance或safePoint拒绝

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.eventReceipt

- 级别：参考
- 文件：`modules/jobs/internal/event_store.go`
- 作用：按caller/epoch/种类/operationId核输入hash并读取安全成功回执

```go
func (work *leaseOperation) eventReceipt(ctx context.Context, op OperationContext, kind string, request []byte, result any) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |
| `op` | `OperationContext` | 可信操作键 | 仅本域内部，信任边界已验证字段 |
| `kind` | `string` | publish/ack/replay | 仅本域内部，信任边界已验证字段 |
| `request` | `[]byte` | 规范输入 | 仅本域内部，信任边界已验证字段 |
| `result` | `LeaseReceiptResult` | 输出结果指针 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `found` | `bool` | 命中完整回执 | 成功有效，失败不返回半结果 |
| `err` | `error` | 冲突或损坏 | 成功有效，失败不返回半结果 |

**步骤**

1. 按caller/epoch/种类/operationId核输入hash并读取安全成功回执

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.saveEventReceipt

- 级别：参考
- 文件：`modules/jobs/internal/event_store.go`
- 作用：在当前事件事务写输入hash及安全成功响应，不存正文/secret

```go
func (work *leaseOperation) saveEventReceipt(ctx context.Context, op OperationContext, kind string, request []byte, result any) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |
| `op` | `OperationContext` | 可信操作键 | 仅本域内部，信任边界已验证字段 |
| `kind` | `string` | publish/ack/replay | 仅本域内部，信任边界已验证字段 |
| `request` | `[]byte` | 规范输入 | 仅本域内部，信任边界已验证字段 |
| `result` | `LeaseReceiptResult` | 成功结果 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 写失败 | 成功有效，失败不返回半结果 |

**步骤**

1. 在当前事件事务写输入hash及安全成功响应，不存正文/secret

**错误**

- 无

- 副作用：写event_operations
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.RelayOutbox

- 级别：约束
- 文件：`modules/jobs/internal/event_store.go`
- 作用：本域outbox转发薄入口，完整复用平台SQL store与转发循环

```go
func RelayOutbox(ctx context.Context, op OperationContext, client EventPublisher) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域生命周期取消 | 停止时由平台循环退出并保留持久outbox |
| `op` | `OperationContext` | jobs可信后台上下文 | callerService=jobs，不使用浏览器actor |
| `client` | `EventPublisher` | 生成self JobsClient经platform.NewEventPublisher适配；由启动装配按可选self地址提供 | 如配置Dependencies[jobs]则必须是受信self地址；不从Host推断。缺失时本域后台保持休眠，直接调用本函数仍返回DEPENDENCY_UNAVAILABLE |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 装配失败或循环终止原因 | ctx取消正常退出，其他使用公共安全错误码 |

**步骤**

1. 核jobs后台来源、本域Store/runtime与窄发布客户端已装配
2. 复用outbox.NewStore(db)，以WithOperationContext(ctx,op)和运行权威调用platform.RunRelay；batch=100、poll=1000ms
3. 原事件ID与payload保持，平台NewEventPublisher按每event建立稳定操作键并签名self jobs:publishEvent，确认durable后平台标relayed；不复制循环、租约、重试或隔离机制
4. Runtime与必需WriteWindow均直接重读本域LeaseRuntimeView；provider错误透传，SafePointRequested必须有BarrierDeadline并原样返回；维护无safe请求明确LOCKED，正常running返回零time.Time。领取、发布前后及回执/重试/隔离/旧代丢弃的窗口重查和受限IO ctx全部由platform.RunRelay负责，不丢弃本域截止或把未知权威当running

**错误**

- `FORBIDDEN` / HTTP 403：非jobs生命周期来源（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：存储、运行权威或自指客户端未装配（可重试）

- 副作用：由platform循环更新本域outbox投递状态，经版本化self HTTP进入事件存储
- 授权：permission：service；rule：仅jobs可信后台，self服务断言生产者仍为jobs
- 并发：复用平台有界串行转发，不持Store锁调用HTTP
- 幂等：原eventId/完整信封哈希；平台每event稳定控制键
- 依赖：`platform.RunRelay`、`platform.NewEventPublisher`、`jobs:publishEvent`
- 其他依赖说明：platform.Store；可选config.Dependencies[jobs]

## jobs.RequeueExpiredDeliveries

- 级别：参考
- 文件：`modules/jobs/internal/event_delivery.go`
- 作用：把已经推送给订阅者但确认租约到期的投递重新排队或转死信

```go
func RequeueExpiredDeliveries(ctx context.Context, op OperationContext, in RequeueExpiredDeliveriesInput) (RequeueExpiredDeliveriesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RequeueExpiredDeliveriesInput` | 到期确认租约扫描参数 | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RequeueExpiredDeliveriesResult` | 本轮重排与死信计数 | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅jobs可信后台上下文；维护/control屏障时暂停active投递调度变更
2. 每轮最多limit个当前epoch delivering且精确30秒租约到期的行，按revision/attempt/state CAS
3. 过期次数加一；新次数不超过10则queued并按1/2/4秒指数退避（上限300秒），超过10转dead-letter，仅保存安全DEADLINE_EXCEEDED码
4. 已ack不受影响；不跳过同producer/subject前序未ack投递，不自动重放死信

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `LOCKED` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `CONFLICT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `STALE_RESULT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `NOT_FOUND` / HTTP 404：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（可重试）
- `INTEGRITY_ERROR` / HTTP 422：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）

- 副作用：写delivery状态与退避时间
- 授权：permission：service；rule：service事件worker
- 并发：单jobs进程一个扫描循环；行级CAS防重复改写
- 幂等：按deliveryId与attempt CAS；重复扫描不重复计数
- 依赖：`jobs.beginLeaseOperation`

## jobs.AcknowledgeEvent

- 级别：约束
- 文件：`modules/jobs/internal/event_delivery.go`
- 作用：消费者确认投递

```go
func AcknowledgeEvent(ctx context.Context, op OperationContext, in AcknowledgeEventInput) (AcknowledgeEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AcknowledgeEventInput` | 消费者确认投递；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcknowledgeEventResult` | 消费者确认投递成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. consumerId须等于可信caller，且事件类型/版本已登记给该consumer；epoch与当前runtime一致
2. 读取对应持久delivery，只允许已实际投递的delivering或到期queued确认；从未投递与dead-letter拒绝，已ack只读重复成功
3. CAS对应consumer的投递acknowledged，不影响其他consumer；本域handler是否已提交由consumer持久inbox保证
4. 在同事务推进该consumer/epoch连续确认水位，不能越过更早未ack或dead-letter投递；保存操作回执
5. 已发投递确认可在draining且有效control截止前收敛，safe水位后不新增ack/回执

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：EVENT_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `LOCKED` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `CONFLICT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `STALE_RESULT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（可重试）
- `INTEGRITY_ERROR` / HTTP 422：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）

- 副作用：写delivery
- 授权：permission：service；rule：service指定consumer
- 并发：event/consumer复合主键
- 幂等：重复ack成功
- 依赖：`jobs.beginLeaseOperation`、`jobs.readDeliveryRecord`、`jobs.consumerHasEvent`、`jobs.requireEventWrite`、`jobs.advanceConsumerOffset`、`jobs.eventReceipt`、`jobs.saveEventReceipt`
- 其他依赖说明：私有SQLite

## jobs.ReplayDelivery

- 级别：约束
- 文件：`modules/jobs/internal/event_delivery.go`
- 作用：重放死信通知

```go
func ReplayDelivery(ctx context.Context, op OperationContext, in ReplayDeliveryInput) (ReplayDeliveryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReplayDeliveryInput` | 重放死信通知；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReplayDeliveryResult` | 重放死信通知成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 由路径deliveryId读取持久event/consumer/epoch，按生成目录producer/type管理权限请求identity最新授权；job.manage只作入口粗能力
2. 身份/epoch不从正文提供；网络请求前释放Store锁，进入事务再核同一投递及当前epoch/control
3. 只允许dead-letter，已ack或其他活动状态不能覆盖；保留原eventId，重置过期次数并排队原consumer，清租约与最后错误
4. 操作键绑定可信caller/actor/epoch及deliveryId；重复键重放原结果，不重复复活；回执和状态同事务

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `CONFLICT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `STALE_RESULT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `NOT_FOUND` / HTTP 404：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（可重试）
- `INTEGRITY_ERROR` / HTTP 422：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）

- 副作用：写delivery/outbox audit
- 授权：permission：job.manage；rule：job.manage入口粗能力；终点按持久事件唯一producer/type目录的managementPermission实时核权
- 并发：revision CAS只恢复dead-letter，网络授权不持Store锁或数据库事务
- 幂等：operationId幂等
- 依赖：`jobs.authorizeDeliveryManagement`、`jobs.beginLeaseOperation`、`jobs.readDeliveryRecord`、`jobs.requireEventWrite`、`jobs.eventReceipt`、`jobs.saveEventReceipt`

## jobs.readDeliveryRecord

- 级别：参考
- 文件：`modules/jobs/internal/event_delivery.go`
- 作用：按持久投递ID或event/consumer复合键读取同一行与事件权限字段，不采用客户端producer

```go
func (work *leaseOperation) readDeliveryRecord(ctx context.Context, deliveryID string, eventID string, consumer string) (deliveryRecord, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |
| `deliveryID` | `string` | 重放路径ID，或空串 | 仅本域内部，信任边界已验证字段 |
| `eventID` | `string` | 确认事件ID，或空串 | 仅本域内部，信任边界已验证字段 |
| `consumer` | `string` | 确认消费者，或空串 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `delivery` | `deliveryRecord` | 当前投递及不可变事件作用域 | 成功有效，失败不返回半结果 |
| `err` | `error` | 不存在/旧epoch/损坏 | 成功有效，失败不返回半结果 |

**步骤**

1. 按持久投递ID或event/consumer复合键读取同一行与事件权限字段，不采用客户端producer

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.advanceConsumerOffset

- 级别：参考
- 文件：`modules/jobs/internal/event_delivery.go`
- 作用：计算最早未ack之前的连续全局seq水位，永不跨过死信/未确认，按consumer/epoch单调保存

```go
func (work *leaseOperation) advanceConsumerOffset(ctx context.Context, consumer string, epoch Identifier) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |
| `consumer` | `string` | 已验consumer | 仅本域内部，信任边界已验证字段 |
| `epoch` | `Identifier` | 当前代次 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 水位写失败 | 成功有效，失败不返回半结果 |

**步骤**

1. 计算最早未ack之前的连续全局seq水位，永不跨过死信/未确认，按consumer/epoch单调保存

**错误**

- 无

- 副作用：写consumer_offsets
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.authorizeDeliveryManagement

- 级别：参考
- 文件：`modules/jobs/internal/event_delivery.go`
- 作用：锁内短读持久投递及生成producer/type管理权限，释放锁后请求identity，再由重放事务复核

```go
func authorizeDeliveryManagement(ctx context.Context, op OperationContext, deliveryID string) (deliveryRecord, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |
| `op` | `OperationContext` | 可信actor/epoch | 仅本域内部，信任边界已验证字段 |
| `deliveryID` | `string` | 投递引用 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `delivery` | `deliveryRecord` | 已授权持久事件/consumer作用域 | 成功有效，失败不返回半结果 |
| `err` | `error` | 拒绝/损坏/不可达 | 成功有效，失败不返回半结果 |

**步骤**

1. 锁内短读持久投递及生成producer/type管理权限，释放锁后请求identity，再由重放事务复核

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：`jobs.findEventDefinition`、`jobs.authorizePermission`

## jobs.ValidateCron

- 级别：参考
- 文件：`modules/jobs/internal/cron.go`
- 作用：严格解析定时表达式

```go
func ValidateCron(ctx context.Context, op OperationContext, in ValidateCronInput) (ValidateCronResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateCronInput` | 严格解析定时表达式；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateCronResult` | 严格解析定时表达式成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 5字段分/时/日/月/周
2. 拒秒/宏/随机
3. IANA时区需存在
4. 分钟最小间隔1

**错误**

- `VALIDATION` / HTTP 400：INVALID_CRON：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_TIMEZONE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：job.manage；rule：job.manage内部
- 并发：纯函数，禁止任意脚本
- 幂等：同表达式同结果
- 依赖：无
- 其他依赖说明：Go time zone database

## jobs.NextFireAt

- 级别：参考
- 文件：`modules/jobs/internal/cron.go`
- 作用：计算下一分钟触发

```go
func NextFireAt(ctx context.Context, op OperationContext, in NextFireAtInput) (NextFireAtResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `NextFireAtInput` | 计算下一分钟触发；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NextFireAtResult` | 计算下一分钟触发成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 遍历下一UTC分钟至366天
2. 按本地时区匹配
3. DST重复本地分钟仅第1次，缺失本地时间跳过

**错误**

- `VALIDATION` / HTTP 400：NO_NEXT_FIRE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数，迭代上限527040
- 幂等：same表达式/after同结果
- 依赖：`jobs.ValidateCron`

## jobs.CreateSchedule

- 级别：约束
- 文件：`modules/jobs/internal/schedule_store.go`
- 作用：创建定时任务

```go
func CreateSchedule(ctx context.Context, op OperationContext, in CreateScheduleInput) (CreateScheduleResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CreateScheduleInput` | 创建定时任务；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateScheduleResult` | 创建定时任务成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. job.manage管理员，或owner服务身份只为本域jobTypes登记系统默认schedule（固定scheduleId，重复登记返回原记录）
2. 校验cron
3. 存nextAt/启用状态和审计事件

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_CRON：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：同operationId不同inputHash（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：当前代次定时数量达到1000（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写schedule/outbox
- 授权：permission：job.manage；rule：job.manage
- 并发：每站点最多1000 schedules
- 幂等：operationId+inputHash幂等
- 依赖：`jobs.ValidateCron`、`jobs.NextFireAt`

## jobs.UpdateSchedule

- 级别：约束
- 文件：`modules/jobs/internal/schedule_store.go`
- 作用：修改定时CAS

```go
func UpdateSchedule(ctx context.Context, op OperationContext, in UpdateScheduleInput) (UpdateScheduleResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `UpdateScheduleInput` | 修改定时CAS；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateScheduleResult` | 修改定时CAS成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验权限/revision
2. 重新计算future nextAt
3. 已排队任务不删除，停用仅阻止未来tick

**错误**

- `CONFLICT` / HTTP 409：SCHEDULE_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写schedule/outbox
- 授权：permission：job.manage；rule：job.manage
- 并发：CAS revision
- 幂等：operationId幂等
- 依赖：`jobs.NextFireAt`、`identity:checkAuthorization`

## jobs.ListSchedules

- 级别：约束
- 文件：`modules/jobs/internal/schedule_store.go`
- 作用：查看定时任务

```go
func ListSchedules(ctx context.Context, op OperationContext, input ListSchedulesInput) (ListSchedulesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ListSchedulesInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListSchedulesResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. job.manage
2. 列出系统默认与管理员登记的schedule、下次触发时间与启用状态

**错误**

- `FORBIDDEN`：权限不足（不可重试）
- `VALIDATION`：当前代次超过256条，失败关闭且不截断（不可重试）

- 副作用：无
- 授权：permission：job.manage；rule：后台管理员
- 并发：只读快照
- 幂等：只读
- 依赖：无

## jobs.NewScheduleHandlers

- 级别：参考
- 文件：`modules/jobs/internal/schedule_store.go`
- 作用：绑定真实定时管理路由到统一服务入口

```go
func NewScheduleHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `EventHandlers` | createSchedule、updateSchedule、listSchedules的真实HTTP映射 | 只交统一NewServiceHandler，不独立裸挂 |

**步骤**

1. createSchedule绑定CreateScheduleRequestBody，dataEpoch只来自可信op
2. updateSchedule从路径取scheduleId，正文不重复声明
3. listSchedules无正文；成功200，错误经既有统一Problem写出

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：创建和修改沿用operationId回执，列表只读
- 依赖：`jobs.CreateSchedule`、`jobs.UpdateSchedule`、`jobs.ListSchedules`、`jobs.writeEventResponse`

## jobs.CoordinateBarrier

- 级别：约束
- 文件：`modules/jobs/internal/barrier_coordinator.go`
- 作用：读取或推进全域安全点

```go
func CoordinateBarrier(ctx context.Context, op OperationContext, in CoordinateBarrierInput) (CoordinateBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor由服务断言构建；observe要求actor.kind=service且caller在已登记非jobs owner集合；服务身份不接受客户端自声明；deadline不超过上游 |
| `in` | `CoordinateBarrierInput` | 等待全域安全点；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CoordinateBarrierResult` | 等待全域安全点成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. step=observe时只允许已登记13个非jobs有状态域的service actor；读取jobs barriers持久行与v1 domain_status_json，核barrierId、dataEpoch、draining状态、截止、phase、14个owner集合、selectedProofHash及前后相同的零broker基础观察，返回selectedProof；缺任何证据失败关闭且不写入
2. prepare/drain/seal/resume-drain仍只接受data-protection服务身份，用户授权由data-protection既有流程核验；本域不因observe分支放宽变更授权
3. 变更步骤的全域stop/acquire/等待仍由协调器依赖提供；本函数不把本域SnapshotManager状态或sealProofHash冒充全域基础proof

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DOMAIN_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：observe调用者不是已登记非jobs服务，或变更步骤不是data-protection（不可重试）；无写入，无需回滚
- `INTEGRITY_ERROR` / HTTP 422：domain_status_json缺字段、owner集合、phase、hash或broker零水位不一致（不可重试）；无写入，无需回滚

- 副作用：observe无副作用；变更步骤写barrier/领域维护调用
- 授权：permission：service；rule：observe仅已登记13个非jobs有状态域service actor且caller属于selectedProof.owners；prepare/drain/seal/resume-drain仅data-protection服务身份，用户授权仍由data-protection在发起时核验
- 并发：observe为单SQLite只读事务；变更步骤不能只等jobs lease，每域实际在途写入/文件恢复日志都到安全点
- 幂等：observe按barrierId/dataEpoch/selectedProofHash重复读取；变更步骤按barrierId重复查询并继续原协调
- 依赖：`jobs.coordinateBarrierRequest`、`jobs.observeBarrier`、`site-config:readRuntimeState`
- 其他依赖说明：jobs.barriers.domain_status_json；各域acquireBarrier

## jobs.observeBarrier

- 级别：参考
- 文件：`modules/jobs/internal/barrier_coordinator.go`
- 作用：只读返回协调器已持久选择的稳定零broker基础proof

```go
func observeBarrier(ctx context.Context, op OperationContext, input lifecycle.SnapshotBarrierInput) (CoordinateBarrierResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在单一SQLite只读事务读取barriers行，不读取control私有文件，不调用SnapshotManager的本域safe封口hash
2. 严格解码v1 domain_status_json，核barrierId/dataEpoch、draining、deadline、mode对应phase、14个唯一owner且caller属于其中、selectedProofHash、revisionWatermark及broker前后完全相同且pendingDeliveries/activeLeases均为0
3. 调用observeBrokerTx复用本函数已持有的同一SQLite事务读取当前broker观察，并与持久brokerBefore/brokerAfter逐字段比较；返回CoordinateBarrierResult.status与selectedProof；任一行缺失、过期、字段不完整或水位变化失败关闭

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：协调器尚未持久选择基础proof或barrier行不可读（可重试）
- `INTEGRITY_ERROR` / HTTP 422：持久基础proof字段或broker稳定零水位不一致（不可重试）
- `FORBIDDEN` / HTTP 403：caller不在14域proof owner集合（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅已登记非jobs service actor；caller必须属于持久selectedProof.owners
- 并发：单SQLite只读事务
- 幂等：相同barrierId/dataEpoch返回同一持久selectedProof或同一失败
- 依赖：`jobs.currentStore`、`jobs.SnapshotDatabase`、`jobs.observeBrokerTx`、`jobs.validateBarrierObserveEvidence`
- 其他依赖说明：本域SQLite barriers表

## jobs.observeBrokerTx

- 级别：参考
- 文件：`modules/jobs/internal/barrier_coordinator.go`
- 作用：复用调用方同一SQLite事务读取jobs broker观察，禁止重新BeginTx

```go
func observeBrokerTx(ctx context.Context, tx *sql.Tx, dataEpoch platform.Identifier) (gen.SnapshotBrokerObservation, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在传入事务内调用platform outbox.Store.ObserveTx读取共享revision、全部非终态outbox行及outbox lease
2. 在同一事务内扫描全部deliveries、jobs与leases，并执行observeBroker全部状态、epoch、expiry、orphan和计数校验
3. 返回与持久基础broker逐字段可比较的SnapshotBrokerObservation；任一事实变化或未知失败关闭

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅CoordinateBarrier.observe或observeBrokerTx内部调用
- 并发：复用调用方单一SQLite事务
- 幂等：同一事务快照返回同一观察或同一失败
- 依赖：`platform.Store.ObserveTx`
- 其他依赖说明：本域SQLite；本域16张业务表；nw_outbox_control

## jobs.coordinateBarrierRequest

- 级别：参考
- 文件：`modules/jobs/internal/barrier_coordinator.go`
- 作用：绑定全域协调请求的可信operation、epoch、模式、deadline和step

```go
func coordinateBarrierRequest(op OperationContext, request BarrierRequest) (lifecycle.SnapshotBarrierInput, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域BarrierRequest严格绑定，不创建新operation或跨域事实

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：依赖公共入口可信op
- 并发：纯函数
- 幂等：确定性转换
- 依赖：`jobs.barrierInput`

## jobs.validateBarrierObserveEvidence

- 级别：参考
- 文件：`modules/jobs/internal/barrier_coordinator.go`
- 作用：核对协调器持久基础proof的owner、阶段、代次和稳定零broker观察

```go
func validateBarrierObserveEvidence(evidence persistedBarrierObserveEvidence, input lifecycle.SnapshotBarrierInput, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 严格核对持久证据字段、调用服务owner、phase和broker前后水位；缺失或变化失败关闭

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅observeBarrier内部调用
- 并发：纯读取
- 幂等：确定性校验
- 依赖：无

## jobs.EnterBarrier

- 级别：约束
- 文件：`modules/jobs/internal/barrier.go`
- 作用：取得jobs本域屏障

```go
func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `BarrierRequest` | 取得jobs本域屏障；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 取得jobs本域屏障成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 禁止新claim/schedule/submission
2. 等待dispatcher在途接受完成和owner安全确认
3. checkpoint DB/事件游标水位

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写本域barrier
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：事件接收暂拒503让producer outbox保留；ack安全落点后冻结
- 幂等：barrierId幂等
- 依赖：`jobs.runBarrierStep`

## jobs.ReleaseBarrier

- 级别：约束
- 文件：`modules/jobs/internal/barrier.go`
- 作用：解除全域任务暂停

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReleaseBarrierInput` | 解除全域任务暂停；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 解除全域任务暂停成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 核验site-config当前epoch及maintenance退出决策
2. 旧epoch任务标stale
3. 全部owner ready后恢复claims和tick

**错误**

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写barrier/job stale
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：旧任务不能改epoch后自动继续
- 幂等：barrierId幂等
- 依赖：`site-config:readRuntimeState`、`jobs.snapshotClients`、`jobs.toBarrierStatus`、`platform.SnapshotManager.ReleaseBarrier`

## jobs.ReadLeaseRuntime

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：组合公开运行权威与同一SnapshotManager的后台写窗口

```go
func (s *Store) ReadLeaseRuntime(runtime platform.RuntimeView) (LeaseRuntimeView, error)
```

**输入**

无

**输出**

无

**步骤**

1. 正常running复用runtime；维护或屏障态读取platform.SnapshotManager.ReadBarrierStatus的内存状态和截止时间；状态缺失、截止已过、代次/屏障不符或未知状态失败关闭，不读取platform私有持久格式

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信启动装配
- 并发：只读
- 幂等：重复读取同一运行权威
- 依赖：`platform.ReadRuntime`、`jobs.currentSnapshotManager`、`platform.SnapshotManager.ReadBarrierStatus`

## jobs.ReadReadinessEvidence

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：返回真实journal恢复与就绪门禁；正常running可ready但不冒充snapshot safe-point

```go
func (s *Store) ReadReadinessEvidence(runtime platform.RuntimeView, initialized bool) (bool, bool, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核本域journal；正常running返回journal=true、safePoint=false、公开门按初始化决定；维护态读取platform.SnapshotManager.ReadBarrierStatus，截止已过或证据缺失失败关闭，draining返回真实safePoint=false，safe返回true

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信启动装配
- 并发：只读
- 幂等：重复读取同一状态
- 依赖：`jobs.OpenDatabase`、`jobs.currentSnapshotManager`、`platform.SnapshotManager.ReadBarrierStatus`

## jobs.ConfigureSnapshotManager

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：安装本域唯一SnapshotManager供控制与快照路由复用

```go
func (s *Store) ConfigureSnapshotManager(manager *lifecycle.SnapshotManager) error
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝空manager或已关闭Store；保存同一实例

**错误**

- 无

- 副作用：更新进程内Store装配引用
- 授权：permission：service；rule：可信启动装配
- 并发：Store mutex
- 幂等：同一实例重复安装幂等
- 依赖：`platform.NewSnapshotManager`

## jobs.ConfigureDataProtectionClient

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：安装可选data-protection归档客户端

```go
func (s *Store) ConfigureDataProtectionClient(client *gen.DataProtectionClient)
```

**输入**

无

**输出**

无

**步骤**

1. 保存启动装配传入的版本化HTTP客户端，不复制跨域存储

**错误**

- 无

- 副作用：更新进程内客户端引用
- 授权：permission：service；rule：可信启动装配
- 并发：Store mutex
- 幂等：重复安装替换同一装配槽
- 依赖：无

## jobs.snapshotClients

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：读取已装配的本域SnapshotManager与data-protection客户端

```go
func (s *Store) snapshotClients() (*lifecycle.SnapshotManager, *gen.DataProtectionClient, error)
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝关闭或未装配Store；返回已登记客户端引用

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部控制入口
- 并发：Store读锁
- 幂等：只读重复安全
- 依赖：`jobs.currentSnapshotManager`

## jobs.barrierInput

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：把已过公共入口的屏障请求绑定可信操作、模式与截止

```go
func barrierInput(op OperationContext, request BarrierRequest) (lifecycle.SnapshotBarrierInput, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 绑定operationId、barrierId、mode和UTC deadline

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：依赖公共入口可信op
- 并发：纯函数
- 幂等：确定性转换
- 依赖：无

## jobs.toBarrierStatus

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：转换platform屏障结果为生成HTTP类型

```go
func toBarrierStatus(status lifecycle.SnapshotBarrierStatus) gen.BarrierStatus
```

**输入**

无

**输出**

无

**步骤**

1. 保留真实状态、水位和proof hash

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部转换
- 并发：纯函数
- 幂等：确定性转换
- 依赖：`jobs.optionalHash`

## jobs.optionalHash

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：把空proof hash投影为可选HTTP字段

```go
func optionalHash(value string) *gen.Hash
```

**输入**

无

**输出**

无

**步骤**

1. 空值返回nil，非空保留原值

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部转换
- 并发：纯函数
- 幂等：确定性转换
- 依赖：无

## jobs.toObservation

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：转换platform本域观察为生成HTTP类型

```go
func toObservation(observation lifecycle.SnapshotBarrierObservation) gen.SnapshotBarrierObservation
```

**输入**

无

**输出**

无

**步骤**

1. 保留真实活动计数、水位与工作状态

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部转换
- 并发：纯函数
- 幂等：确定性转换
- 依赖：无

## jobs.runBarrierStep

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：把data-protection控制请求转交同一SnapshotManager步骤

```go
func runBarrierStep(ctx context.Context, op OperationContext, request BarrierRequest) (gen.BarrierStatus, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按step调用prepare、drain、observe、seal或resume-drain；缺proof失败关闭

**错误**

- 无

- 副作用：由SnapshotManager写control屏障
- 授权：permission：service；rule：仅data-protection服务身份
- 并发：控制入口串行
- 幂等：barrierId与proof幂等
- 依赖：`jobs.snapshotClients`、`jobs.barrierInput`、`jobs.toBarrierStatus`、`jobs.toObservation`、`platform.SnapshotManager.PrepareBarrier`、`platform.SnapshotManager.DrainBarrier`、`platform.SnapshotManager.ObserveBarrier`、`platform.SnapshotManager.SealBarrier`、`platform.SnapshotManager.ResumeDrain`

## jobs.NewBarrierHandlers

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：绑定屏障控制HTTP适配

```go
func NewBarrierHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 只做生成请求绑定和公共Problem输出，不复制入口校验

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：依赖公共NewServiceHandler
- 并发：返回固定handler表
- 幂等：重复构造安全
- 依赖：`jobs.EnterBarrier`、`jobs.ReleaseBarrier`、`jobs.CoordinateBarrier`

## jobs.currentSnapshotManager

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：读取本域已安装SnapshotManager

```go
func (s *Store) currentSnapshotManager() (*lifecycle.SnapshotManager, error)
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝nil、关闭或未装配Store

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部控制入口
- 并发：Store读锁
- 幂等：只读重复安全
- 依赖：无

## jobs.SnapshotDatabase

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：返回本域active SQLite连接供platform一致捕获

```go
func (s *Store) SnapshotDatabase() (*sql.DB, error)
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝nil、关闭或未装配DB

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域快照装配
- 并发：Store读锁
- 幂等：只读重复安全
- 依赖：无
- 其他依赖说明：本域SQLite

## jobs.SnapshotObserveWork

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：读取jobs-wide真实持久工作观察

```go
func (s *Store) SnapshotObserveWork(ctx context.Context) (lifecycle.SnapshotWorkState, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在同一SQLite事务中调用jobs.SnapshotDatabase、platform.Store.ObserveTx并读取leases与job_orphans待决事实；workRevision复用outbox control revision，不使用业务表MAX或platform私有control格式

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：SnapshotManager内部调用
- 并发：单SQLite短读事务
- 幂等：观察不写业务事实
- 依赖：`jobs.SnapshotDatabase`、`jobs.observeSnapshotWorkTx`
- 其他依赖说明：本域SQLite

## jobs.SnapshotWatermark

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：返回本域真实统一修订水位

```go
func (s *Store) SnapshotWatermark(ctx context.Context) (int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在同一SQLite事务中复用platform.Store.ObserveTx返回的真实control revision，不从多个表MAX推导

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：SnapshotManager内部调用
- 并发：单SQLite短读事务
- 幂等：重复读取安全
- 依赖：`jobs.SnapshotDatabase`、`jobs.observeSnapshotWorkTx`
- 其他依赖说明：本域SQLite

## jobs.SnapshotSealWork

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：在同库事务CAS全部jobs-wide水位并封门

```go
func (s *Store) SnapshotSealWork(ctx context.Context, work lifecycle.SnapshotWorkState, proofHash string) error
```

**输入**

无

**输出**

无

**步骤**

1. 同一SQLite事务重读platform.Store.ObserveTx与业务待决事实，精确比较原观察结果，再调用platform.Store.SealTx；任一水位或待决事实变化即失败关闭

**错误**

- 无

- 副作用：在本域同库提交outbox与已登记业务表sealed gate
- 授权：permission：service；rule：SnapshotManager内部调用
- 并发：单SQLite事务CAS
- 幂等：proof绑定原观察水位
- 依赖：`jobs.SnapshotDatabase`、`jobs.observeSnapshotWorkTx`、`platform.Store.SealTx`
- 其他依赖说明：本域SQLite

## jobs.SnapshotResumeWork

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：按原封口proof恢复旧执行清理窗口

```go
func (s *Store) SnapshotResumeWork(ctx context.Context, proofHash string) error
```

**输入**

无

**输出**

无

**步骤**

1. 同库事务调用platform.Store.ResumeTx精确绑定原proof；不直接改写业务epoch或释放未知活动

**错误**

- 无

- 副作用：在本域同库重开旧outbox与业务写门
- 授权：permission：service；rule：SnapshotManager内部调用
- 并发：控制入口串行SQLite事务
- 幂等：proof精确绑定
- 依赖：`jobs.SnapshotDatabase`、`platform.Store.ResumeTx`
- 其他依赖说明：本域SQLite

## jobs.observeSnapshotWorkTx

- 级别：参考
- 文件：`modules/jobs/internal/barrier.go`
- 作用：在一个已持有事务内组合platform outbox观察和jobs旧执行资格

```go
func observeSnapshotWorkTx(ctx context.Context, outboxStore *outbox.Store, tx *sql.Tx) (lifecycle.SnapshotWorkState, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 同库观察事务的取消上下文 | 仅本域SnapshotManager调用；取消透传查询 |
| `outboxStore` | `*outbox.Store` | 与jobs数据库绑定的平台outbox存储 | 仅用于调用platform.Store.ObserveTx，不独立开启读事务 |
| `tx` | `*sql.Tx` | 调用方已开启的jobs SQLite事务 | 观察outbox control与jobs待决事实必须复用同一事务 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `work` | `lifecycle.SnapshotWorkState` | 真实统一revision与本域待决执行事实 | OutboxRevision与WorkRevision同为platform control revision；失败返回零值 |
| `error` | `error` | 平台观察或本域事实不可核验 | 成功nil；不以未知水位返回成功 |

**步骤**

1. 调用platform.Store.ObserveTx；读取leases与job_orphans作为PendingWork；复用同一revision，不使用MAX推导版本

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：SnapshotManager内部调用
- 并发：复用调用方单一SQLite事务
- 幂等：纯读取
- 依赖：`platform.Store.ObserveTx`
- 其他依赖说明：本域SQLite

## jobs.PurgeExpiredHistory

- 级别：参考
- 文件：`modules/jobs/internal/retention.go`
- 作用：清理已完成记录

```go
func PurgeExpiredHistory(ctx context.Context, op OperationContext, in PurgeExpiredHistoryInput) (PurgeExpiredHistoryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PurgeExpiredHistoryInput` | 清理已完成记录；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PurgeExpiredHistoryResult` | 清理已完成记录成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 维护或屏障安全点期间不删除任何历史；一次短事务内所有SELECT/DELETE都绑定可信op.DataEpoch，batchSize分别限制每类最多500行，deletedCount为实际删除的progress、event和终局job主记录总数
2. 只删除created_at真实非null、可由Go time.Parse(RFC3339Nano)严格解析且早于7天的当前epoch终局job进度；活动job进度、时刻未知或损坏进度永久保留
3. 只删除当前epoch中envelope.occurredAt可由Go time.Parse(RFC3339Nano)严格解析、早于30天且全部consumer delivery均已acknowledged的事件；同时删除其delivery元数据，event_sequence水位不回收
4. 只删除当前epoch中state为succeeded/cancelled/dead-letter且updated_at严格解析并早于30天、无lease、无orphan、存在job_results与job_outcomes、resultRef为空且paidResultRefs经json_type确认为真实空数组的job；job_results缺失、JSON非数组、未知结构或任何付费/对象引用都保留
5. 删除符合上述条件的job时只同时删除其progress、空job_results和job_outcomes；lease_operations、event_operations、consumer_offsets、schedule_operations等幂等/审计回执不删除

**错误**

- `FORBIDDEN` / HTTP 403：调用方不是jobs维护worker（不可重试）
- `VALIDATION` / HTTP 400：清理时间或batchSize越界（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：事件发生时间或持久JSON无法严格核验（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删除有限历史
- 授权：permission：service；rule：service维护worker
- 并发：单短事务；每类≤500行，active/orphan/带引用job禁止删除，网络与等待不持事务
- 幂等：重复purge幂等
- 依赖：无
- 其他依赖说明：私有SQLite

## jobs.CreateSnapshot

- 级别：约束
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：导出队列与事件一致快照

```go
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotExportRequest` | 导出队列与事件一致快照；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 导出队列与事件一致快照成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. safe屏障
2. checkpoint
3. 保存任务/结果refs/进度/事件/调度
4. 活跃lease标注不可迁移，含领域已付费结果refs

**错误**

- `LOCKED` / HTTP 409：BARRIER_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写snapshot对象
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：barrier水位之后无事件提交
- 幂等：backupId/barrierId幂等
- 依赖：`platform.SnapshotManager.Export`、`jobs.snapshotRefFromLifecycle`
- 其他依赖说明：各域安全点确认

## jobs.ValidateJobsSnapshotManifest

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：核验jobs快照清单版本与完整库存

```go
func ValidateJobsSnapshotManifest(raw []byte) error
```

**输入**

无

**输出**

无

**步骤**

1. 严格解码并交Value校验，不把未知库存当空数组

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：platform SnapshotManager调用
- 并发：纯读取
- 幂等：相同清单相同结果
- 依赖：`jobs.ValidateJobsSnapshotManifestValue`

## jobs.ValidateJobsSnapshotManifestValue

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：核验jobs清单归属、schema、文件路径和库存集合

```go
func ValidateJobsSnapshotManifestValue(manifest lifecycle.DomainSnapshotManifest) error
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝错误owner、版本、路径、hash或未知空库存

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域恢复内部校验
- 并发：纯读取
- 幂等：确定性校验
- 依赖：无

## jobs.DrainJobs

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：等待jobs真实租约与outbox行租约到安全点，保留queued/retry事实

```go
func DrainJobs(ctx context.Context, store *Store) error
```

**输入**

无

**输出**

无

**步骤**

1. 轮询真实SQLite leases与公共outbox观察；截止到达失败关闭

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：SnapshotManager drain回调
- 并发：短读查询，不持锁等待
- 幂等：重复等待安全
- 依赖：`jobs.SnapshotDatabase`、`platform.Store.ObserveTx`
- 其他依赖说明：本域SQLite

## jobs.MigrateJobsStage

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：在隔离副本撤销旧执行资格并保留原业务事实，不改写epoch、payload、根或费用引用

```go
func MigrateJobsStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error
```

**输入**

无

**输出**

无

**步骤**

1. 调用platform.MigrateRestoredDatabase；在其同一事务内执行本域已登记追加迁移并删除旧lease；保留jobs/events/schedules/operations原epoch、事件envelope/hash、root和费用引用；验证无目标代次事实

**错误**

- `INTEGRITY_ERROR`：源事实、outbox或旧执行资格不可核验（不可重试）

- 副作用：仅修改已认证隔离副本的可重建schema与执行资格
- 授权：permission：service；rule：platform SnapshotManager已认证stage scope
- 并发：隔离SQLite短事务；不改active；hostHash由platform恢复内核重封
- 幂等：同一stage重复校验安全
- 依赖：`jobs.ValidateJobsSnapshotManifestValue`、`platform.MigrateRestoredDatabase`、`jobs.runRestoredJobsMigrations`、`jobs.revokeRestoredEligibility`、`jobs.validateJobsPreservedFacts`
- 其他依赖说明：隔离jobs.db

## jobs.ValidateJobsStaged

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：证明隔离jobs库完整且未引入目标代次业务事实

```go
func ValidateJobsStaged(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error
```

**输入**

无

**输出**

无

**步骤**

1. integrity_check；核outbox无可重放活动、lease已撤销、事件envelope与索引/hash一致；不要求全部原业务行epoch相等

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：platform SnapshotManager已认证stage scope
- 并发：只读SQLite快照
- 幂等：重复校验安全
- 依赖：`jobs.validateJobsPreservedFacts`
- 其他依赖说明：隔离jobs.db

## jobs.revokeRestoredEligibility

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：撤销隔离副本旧租约，保留job_orphans恢复事实

```go
func revokeRestoredEligibility(ctx context.Context, tx *sql.Tx) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 恢复事务取消上下文 | 仅platform.MigrateRestoredDatabase回调使用；取消透传SQL |
| `tx` | `*sql.Tx` | platform恢复内核持有的隔离数据库事务 | 与恢复gate、域迁移同一事务；不自行提交或回滚 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 旧租约撤销失败 | 成功nil；失败由外层事务回滚 |

**步骤**

1. 在调用方恢复事务内删除leases，不删除jobs、job_orphans、结果或费用引用

**错误**

- 无

- 副作用：修改隔离副本leases
- 授权：permission：service；rule：MigrateJobsStage内部
- 并发：复用单一恢复事务
- 幂等：重复删除幂等
- 依赖：无
- 其他依赖说明：隔离jobs.db

## jobs.runRestoredJobsMigrations

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：在恢复内核同一事务中应用本域追加schema迁移

```go
func runRestoredJobsMigrations(ctx context.Context, tx *sql.Tx) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 恢复迁移取消上下文 | 仅platform恢复事务回调使用；取消透传SQL |
| `tx` | `*sql.Tx` | 已认证隔离jobs数据库事务 | 不得自行提交；只执行本域已登记追加迁移 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 迁移集或SQL执行失败 | 成功nil；失败由platform恢复事务回滚 |

**步骤**

1. 核对schema_migrations已应用版本和名称；按embed迁移顺序补schema与可重建派生索引，不改业务epoch、事件payload、root或费用引用

**错误**

- 无

- 副作用：修改隔离jobs数据库schema与schema_migrations
- 授权：permission：service；rule：MigrateJobsStage内部
- 并发：复用单一恢复事务
- 幂等：已登记版本跳过，重复stage安全
- 依赖：`jobs.All`
- 其他依赖说明：隔离jobs.db

## jobs.validateJobsPreservedFacts

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：核验原epoch事实未被改为target并核验旧资格已撤销

```go
func validateJobsPreservedFacts(ctx context.Context, db *sql.DB, targetEpoch string) error
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝目标代次业务行、pending/quarantined outbox、残留lease；调用事件事实核验

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：恢复内部校验
- 并发：只读SQLite快照
- 幂等：确定性校验
- 依赖：`jobs.validatePreservedEvents`
- 其他依赖说明：隔离jobs.db

## jobs.validatePreservedEvents

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：核验持久事件envelope与列索引、非空hash一致，不重写事件

```go
func validatePreservedEvents(ctx context.Context, db *sql.DB) error
```

**输入**

无

**输出**

无

**步骤**

1. 逐行解码并比较eventId、epoch、producer、type、subject、revision；非空hash按原字节核对

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：恢复内部校验
- 并发：只读SQLite快照
- 幂等：确定性校验
- 依赖：无
- 其他依赖说明：隔离jobs.db

## jobs.JobsStageBudget

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：按真实源清单计算隔离迁移峰值预算

```go
func JobsStageBudget(ctx context.Context, manifest lifecycle.DomainSnapshotManifest) (int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 累加文件大小并核上限与溢出

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：platform SnapshotManager调用
- 并发：纯读取
- 幂等：确定性计算
- 依赖：`jobs.ValidateJobsSnapshotManifestValue`

## jobs.snapshotRefToLifecycle

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：把生成快照引用转换为platform内部类型

```go
func snapshotRefToLifecycle(ref gen.SnapshotRef) lifecycle.SnapshotRef
```

**输入**

无

**输出**

无

**步骤**

1. 逐字段保留对象hash、大小、epoch和水位

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部转换
- 并发：纯函数
- 幂等：确定性转换
- 依赖：无

## jobs.snapshotRefFromLifecycle

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：把platform快照引用转换为生成HTTP类型

```go
func snapshotRefFromLifecycle(ref lifecycle.SnapshotRef) gen.SnapshotRef
```

**输入**

无

**输出**

无

**步骤**

1. 逐字段保留对象hash、大小、epoch和水位

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部转换
- 并发：纯函数
- 幂等：确定性转换
- 依赖：无

## jobs.optionalString

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：把空字符串投影为可选HTTP字段

```go
func optionalString(value string) *string
```

**输入**

无

**输出**

无

**步骤**

1. 空值返回nil，非空返回独立字符串指针

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域内部转换
- 并发：纯函数
- 幂等：确定性转换
- 依赖：无

## jobs.NewSnapshotHandlers

- 级别：参考
- 文件：`modules/jobs/internal/snapshot.go`
- 作用：绑定快照导出HTTP适配

```go
func NewSnapshotHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 只做生成请求绑定和公共Problem输出，不复制SnapshotManager机制

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：依赖公共NewServiceHandler
- 并发：返回固定handler表
- 幂等：重复构造安全
- 依赖：`jobs.CreateSnapshot`、`jobs.StageSnapshot`、`jobs.DiscardSnapshot`

## jobs.StageSnapshot

- 级别：约束
- 文件：`modules/jobs/internal/import.go`
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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`、`platform.SnapshotManager.Stage`、`jobs.snapshotRefToLifecycle`、`jobs.optionalString`、`jobs.optionalHash`

## jobs.DiscardSnapshot

- 级别：约束
- 文件：`modules/jobs/internal/import.go`
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

## jobs.SubscribeEvents

- 级别：约束
- 文件：`modules/jobs/internal/http_event_subscription.go`
- 作用：已登记消费者订阅事件SSE

```go
func SubscribeEvents(ctx context.Context, op OperationContext, in SubscribeEventsInput) (SubscribeEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `SubscribeEventsInput` | 已登记消费者订阅事件SSE；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SubscribeEventsResult` | 已登记消费者订阅事件SSE成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. durable consumerId必须与可信caller一致且在生成目录有订阅；拒绝客户端actor/epoch声明；ephemeral等待专门安全投影，本轮UNSUPPORTED_CAPABILITY，不广播原payload
2. 取得每consumer最多2个连接槽，返回真正增量platform.EventStream；关闭/取消归还连接槽，无预装空events数组
3. 每次Next读取当前runtime/control及epoch，用短事务领取最早可见queued delivery为delivering并登记30秒确认租约；同producer/subject前序未ack或dead-letter阻塞后序
4. Last-Event-ID只校验合法eventSeq范围并供SSE传输续接，不跳过持久未ack；回收后旧seq可以重投，由consumer inbox去重
5. 单事件拉取背压、不建大于64的应用积压；公共ServeEventStream负责15秒注释心跳/帧ID/取消关闭；epoch变更立即结束，维护/屏障停止新投递

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `LOCKED` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `CONFLICT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `STALE_RESULT` / HTTP 409：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（可重试）
- `INTEGRITY_ERROR` / HTTP 422：登记生产方/consumer/当前epoch/屏障/CAS/回执或存储权威不满足（不可重试）
- `UNSUPPORTED_CAPABILITY` / HTTP 422：ephemeral安全投影尚未收口，不广播原payload（不可重试）
- `RATE_LIMITED` / HTTP 429：消费者连接槽达到2条（可重试）
- `VALIDATION` / HTTP 400：生产者身份、生成规格或续传游标不满足登记约束（不可重试）

- 副作用：占用有界连接槽；将本consumer queued delivery CAS为带30秒确认租约的delivering
- 授权：permission：service；rule：service注册consumer
- 并发：每consumer最多2流；epoch变化立即关闭旧流
- 幂等：eventSeq重放至少一次，consumer inbox/eventId去重后ack
- 依赖：`jobs.beginLeaseOperation`、`jobs.consumerHasEvent`、`jobs.Next`、`jobs.CloseStore`、`jobs.LastID`

## jobs.Next

- 级别：参考
- 文件：`modules/jobs/internal/http_event_subscription.go`
- 作用：反复短事务领取单事件，无可见行时可取消等待；维护暂停，epoch变更关闭流；不持锁等待网络

```go
func (stream *eventSubscription) Next(ctx context.Context) (EventEnvelope, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消与流生命周期 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `event` | `EventEnvelope` | 下一条持久投递 | 成功有效，失败不返回半结果 |
| `err` | `error` | EOF/代次变更/依赖故障 | 成功有效，失败不返回半结果 |

**步骤**

1. 反复短事务领取单事件，无可见行时可取消等待；维护暂停，epoch变更关闭流；不持锁等待网络

**错误**

- 无

- 副作用：写当前consumer投递租约
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：`jobs.nextDelivery`、`jobs.Close`

## jobs.LastID

- 级别：参考
- 文件：`modules/jobs/internal/http_event_subscription.go`
- 作用：只读当前流最后实际返回seq，用于SSE id帧

```go
func (stream *eventSubscription) LastID() string
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `id` | `string` | 上次实际返回的eventSeq | 成功有效，失败不返回半结果 |

**步骤**

1. 只读当前流最后实际返回seq，用于SSE id帧

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.Close

- 级别：参考
- 文件：`modules/jobs/internal/http_event_subscription.go`
- 作用：幂等停止Next等待并归还本consumer连接槽，不提前确认/释放已发投递

```go
func (stream *eventSubscription) Close() error
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 关闭结果 | 成功有效，失败不返回半结果 |

**步骤**

1. 幂等停止Next等待并归还本consumer连接槽，不提前确认/释放已发投递

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：无

## jobs.nextDelivery

- 级别：参考
- 文件：`modules/jobs/internal/http_event_subscription.go`
- 作用：当前epoch/control下选最早queued可见投递，前序同producer/subject未ack则阻塞，CAS delivering和30秒租约后提交再返回

```go
func (stream *eventSubscription) nextDelivery(ctx context.Context) (EventEnvelope, int64, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅本域内部，信任边界已验证字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `event` | `EventEnvelope` | 下一事件 | 成功有效，失败不返回半结果 |
| `seq` | `int64` | 持久接收序号 | 成功有效，失败不返回半结果 |
| `found` | `bool` | 是否本轮可投递 | 成功有效，失败不返回半结果 |
| `err` | `error` | 失败原因 | 成功有效，失败不返回半结果 |

**步骤**

1. 当前epoch/control下选最早queued可见投递，前序同producer/subject未ack则阻塞，CAS delivering和30秒租约后提交再返回

**错误**

- 无

- 副作用：更新delivery租约与revision
- 授权：permission：service；rule：落实登记的生产者/consumer/任务作用域，通用身份来自可信op
- 并发：复用调用方短事务；网络不持有Store锁或SQLite事务
- 幂等：按登记持久主键、CAS和操作回执
- 依赖：`jobs.beginLeaseOperation`、`jobs.readEventRecord`

## jobs.All

- 级别：参考
- 文件：`modules/jobs/migrations/migrations.go`
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

## jobs.main

- 级别：约束
- 文件：`modules/jobs/cmd/jobs/main.go`
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
- 依赖：`jobs.BuildApplication`、`jobs.Serve`、`jobs.Shutdown`、`platform.RunService`

## jobs.BuildApplication

- 级别：约束
- 文件：`modules/jobs/internal/app/wire.go`
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
2. 先读control区恢复日志与site-config runtime：phase为restore_switching且本域有该restoreId的staged目录、尚未换入时，把active改名为restore/<restoreId>/before、staged改名为active并fsync父目录后写control日志；phase为restore_rolling_back且已换入时反向换回；换目录后丢弃旧dataEpoch未转发outbox、租约与可重建缓存。随后调用platform.ReadRestoreSwapState读取完整Known/Switched/RolledBack/Aborted/stageId/manifestHash/hostHash/hostEpoch/targetEpoch，严格核当前控制host epoch、restoreId与phase；切换后OpenedDataEpoch为target，换回后为host，不把空epoch作为绕过。
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 只读取runtime epoch/maintenance，不以site-config配置已初始化作为broker启动条件；维护期间只开放健康/就绪与控制管理接口、不启动worker。siteId由真实服务读取或待恢复任务按需读取，fresh站点不得阻塞broker健康/控制启动。本次启动完成换目录或换回时以platform.SignActivationReceipt签名真实stage/target树回执并调用site-config:registerActivation；不续签同一已登记回执，不复制portal授权或把service伪装superadmin。有登记生产事件时启动本域outbox转发，目录无jobTypes或订阅时不启动空worker/consumer，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源
6. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环
7. 实际构造三项管理HTTP映射交公共完整NewServiceHandler核全表；caller固定portal、runtime/openedEpoch当前，其余handler/store/control/owner后台未齐仍DEPENDENCY_UNAVAILABLE
8. 私有库迁移真实打开/失败关闭资源；合并管理与进度真实映射再交完整入口工厂，streamProgress固定portal；其余handler/control/owner后台未齐仍blocked
9. 合并NewQueueHandlers真实createJob映射，调用服务白名单仅生成任务目录owner；其他HTTP/运行存储/control/后台缺口仍使完整启动失败
10. 合并NewScheduleHandlers真实createSchedule/updateSchedule/listSchedules映射；createSchedule允许portal和管理员路径以及带系统默认定时的owner服务，list/update仅portal；其余handler、存储安装、control和后台循环未齐仍使完整启动失败
11. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与6个后台入口；BeginDrain复用本域DrainJobs，Observe/Watermark/Seal/Resume统一复用platform.Store事务revision与CAS。Install必须同时收到完整OpenAPI handler、6个后台task、store与真实就绪证据读取器，缺任一业务scope、回调或后台仍失败关闭，不能仅HTTP全表进入ready。
12. self事件发布地址为可选配置；仅配置Dependencies[jobs]时装配publisher，缺失时RelayOutbox入口保持休眠等待进程取消，不使公共RunService终止其它后台任务。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`jobs.All`、`jobs.DrainJobs`、`jobs.InstallStore`、`jobs.ensureDataLayout`、`jobs.loadTrustAnchors`、`jobs.loadSigningKey`、`jobs.newRuntimeClient`、`jobs.NewEventHandlers`、`jobs.NewLeaseHandlers`、`jobs.NewManagementHandlers`、`jobs.NewOwnerRecoveryChecker`、`jobs.NewProgressHandlers`、`jobs.NewQueueHandlers`、`jobs.NewScheduleHandlers`、`jobs.NewBarrierHandlers`、`jobs.NewSnapshotHandlers`、`jobs.OpenDatabase`、`jobs.PurgeExpiredHistory`、`jobs.ReadQueueSiteID`、`jobs.ValidateJobsSnapshotManifest`、`jobs.ValidateJobsSnapshotManifestValue`、`jobs.MigrateJobsStage`、`jobs.ValidateJobsStaged`、`jobs.JobsStageBudget`、`jobs.ReapExpiredLeases`、`jobs.RelayOutbox`、`jobs.RequeueDue`、`jobs.RequeueExpiredDeliveries`、`jobs.CloseStore`、`jobs.ConfigureBackgroundScope`、`jobs.ConfigureDataProtectionClient`、`jobs.ConfigureSnapshotManager`、`jobs.ReadLeaseRuntime`、`jobs.ReadReadinessEvidence`、`jobs.SnapshotDatabase`、`jobs.SnapshotObserveWork`、`jobs.SnapshotSealWork`、`jobs.SnapshotResumeWork`、`jobs.SnapshotWatermark`、`jobs.TickSchedules`、`jobs.registerActivation`、`jobs.restoreOpenedDataEpoch`、`analytics:readJobRecovery`、`knowledge:readJobRecovery`、`media:readJobRecovery`、`platform.ApplyRestoreSwap`、`platform.NewIdentityAuthorizer`、`platform.NewRequestValidator`、`platform.NewServiceHandler`、`platform.NewSiteConfigClient`、`platform.NewSnapshotManager`、`platform.ReadRestoreSwapState`、`platform.ReadRuntime`、`platform.SignActivationReceipt`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`、`site-config:readInitializationStatus`、`site-config:readSiteConfig`、`site-config:registerActivation`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## jobs.restoreOpenedDataEpoch

- 级别：参考
- 文件：`modules/jobs/internal/app/wire.go`
- 作用：绑定恢复换目录终态与进程实际OpenedDataEpoch

```go
func restoreOpenedDataEpoch(control platform.RuntimeView, swap platform.SwapResult, evidence lifecycle.SnapshotSwapEvidence) (platform.Identifier, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `control` | `platform.RuntimeView` | ApplyRestoreSwap前后读取的site-config控制runtime | 必须是真实当前host epoch、restoreId、target epoch、phase和maintenance |
| `swap` | `platform.SwapResult` | platform.ApplyRestoreSwap返回的终态动作 | 仅接受switched或rolled-back；restoreId不可为空 |
| `evidence` | `lifecycle.SnapshotSwapEvidence` | platform.ReadRestoreSwapState返回的完整只读换目录证据 | Known=true；stage/tree/host/target字段完整；不得读取私有日志或猜测成功 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `openedDataEpoch` | `platform.Identifier` | 本进程active目录实际打开的代次 | switched返回targetDataEpoch；rolled-back返回hostEpoch；失败为空 |
| `error` | `error` | 控制runtime与换目录证据不一致 | 成功nil；失败不绕过epoch或phase |

**步骤**

1. 严格核Known、Switched/RolledBack、restoreId、host/target epoch、控制phase与host epoch；按终态返回target或host

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信启动装配
- 并发：纯读取
- 幂等：相同证据确定性结果
- 依赖：无

## jobs.registerActivation

- 级别：参考
- 文件：`modules/jobs/internal/app/wire.go`
- 作用：签名并登记本域真实恢复激活回执

```go
func registerActivation(ctx context.Context, client *gen.SiteConfigClient, key platform.SigningKey, control platform.RuntimeView, swap platform.SwapResult, evidence lifecycle.SnapshotSwapEvidence) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 启动取消与截止 | 不延长上游截止；取消透传HTTP与签名依赖 |
| `client` | `SiteConfigClient` | 已版本化site-config客户端；实际Go类型为*gen.SiteConfigClient | 只调用readRuntimeState与registerActivation；不读取control私有目录 |
| `key` | `platform.SigningKey` | jobs本域服务签名密钥引用 | 仅由serviceauth.SignActivationReceipt使用；不复制portal私钥或委托 |
| `control` | `platform.RuntimeView` | 当前控制host runtime | epoch/restoreId/phase已由restoreOpenedDataEpoch核对 |
| `swap` | `platform.SwapResult` | 已完成的切换或换回动作 | 动作与证据一致 |
| `evidence` | `lifecycle.SnapshotSwapEvidence` | 真实stage/tree/target证据 | 来源为platform.ReadRestoreSwapState；不从私有JSON另读 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 读取控制状态、签名或登记失败 | 成功nil；未知状态、CAS变化或响应不完整失败关闭 |

**步骤**

1. 以当前service actor读取控制状态；相同stage/state/tree已有回执直接复用；否则以UTC规范化RecordedAt调用platform.SignActivationReceipt，按当前control revision调用site-config:registerActivation；核登记响应revision与host/target/phase

**错误**

- 无

- 副作用：写site-config控制库activation receipt
- 授权：permission：service；rule：仅本域启动service身份；不接受superadmin或portal委托
- 并发：短HTTP请求；由site-config revision CAS串行
- 幂等：同restore/module/stage/state/tree复用已登记回执，不续签或覆盖不同回执
- 依赖：`platform.SignActivationReceipt`、`site-config:readRuntimeState`、`site-config:registerActivation`

## jobs.newRuntimeClient

- 级别：参考
- 文件：`modules/jobs/internal/app/wire.go`
- 作用：按登记site-config地址构造运行客户端

```go
func newRuntimeClient(config platform.ServiceConfig, key platform.SigningKey) (platform.SiteConfigClient, error)
```

**输入**

无

**输出**

无

**步骤**

1. 拒绝缺失site-config地址；不从self或Host推断

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信进程装配
- 并发：纯构造
- 幂等：同配置同客户端语义
- 依赖：`platform.NewSiteConfigClient`

## jobs.ensureDataLayout

- 级别：参考
- 文件：`modules/jobs/internal/app/wire.go`
- 作用：建立本域active/control/restore目录，不替换control

```go
func ensureDataLayout(dataRoot string) error
```

**输入**

无

**输出**

无

**步骤**

1. 核绝对数据根并创建固定目录权限

**错误**

- 无

- 副作用：创建本域数据目录
- 授权：permission：service；rule：可信进程装配
- 并发：启动单flight
- 幂等：目录已存在时幂等
- 依赖：无
- 其他依赖说明：本域dataRoot

## jobs.loadTrustAnchors

- 级别：参考
- 文件：`modules/jobs/internal/app/wire.go`
- 作用：读取本域服务身份信任锚

```go
func loadTrustAnchors(ref string) (platform.TrustAnchors, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取并严格解码公钥表；缺失或空表失败关闭

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信进程装配
- 并发：纯读取
- 幂等：相同文件同结果
- 依赖：`platform.LoadServiceIdentity`
- 其他依赖说明：外部secret引用

## jobs.loadSigningKey

- 级别：参考
- 文件：`modules/jobs/internal/app/wire.go`
- 作用：读取并解析本域服务签名密钥引用

```go
func loadSigningKey(ref string) (platform.SigningKey, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只保存引用与平台密钥句柄，不把私钥写日志或配置

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信进程装配
- 并发：纯读取
- 幂等：相同引用同结果
- 依赖：`platform.LoadSigningKey`
- 其他依赖说明：外部secret引用

## jobs.Serve

- 级别：约束
- 文件：`modules/jobs/internal/transport/server.go`
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
- 依赖：`jobs.BuildApplication`、`jobs.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## jobs.Shutdown

- 级别：约束
- 文件：`modules/jobs/internal/transport/server.go`
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

## jobs.Health

- 级别：约束
- 文件：`modules/jobs/internal/transport/server.go`
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

## jobs.Ready

- 级别：约束
- 文件：`modules/jobs/internal/transport/server.go`
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

1. 只有handler表、必需scope/后台资源、store与ReadinessReader均安装后才允许Ready返回200；缺证据返回DEPENDENCY_UNAVAILABLE。
2. 从ReadinessReader取得真实runtime revision、initialized、journalRecovered、safePoint和publicRequestsAllowed；不得在transport硬编码这些值。未初始化时activeDataEpoch=null且publicRequestsAllowed=false；正常running只要求真实journal/epoch证据，safePoint可为false；维护态必须有真实safe-point才可返回控制面ready。
3. 已初始化时activeDataEpoch必须等于Build实际打开的openedEpoch；epoch失配、安全检查未知或journal未恢复继续不可就绪。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：无
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## jobs.ListJobs

- 级别：约束
- 文件：`modules/jobs/internal/queries.go`
- 作用：查看任务队列

```go
func ListJobs(ctx context.Context, op OperationContext, input ListJobsInput) (ListJobsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ListJobsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListJobsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 按jobId正序批读当前epoch安全元数据，active含queued/leased/running/retry-wait/cancel-requested/orphan；failed无当前生产分支而返回空；不读任务输入正文
2. 按持久owner/type匹配生成目录具体管理权限，锁外逐条identity核权，仅FORBIDDEN过滤；失效会话或未知权威整体失败
3. 持续批读至limit+1个有权条目或穷尽，仅有权条目决定hasMore与最后返回ID，不泄露无权位置/数量；返回安全进度和公共错误码
4. 平台SignCursor/VerifyCursor作用域摘要绑定查询类型、actor/site/session及修订、caller/epoch、owner/state/limit；每页重新核权，签名不替代权限

**错误**

- `FORBIDDEN`：非当前后台主体或具体功能权限不足（不可重试）
- `VALIDATION`：分页游标或路径正文绑定无效（不可重试）
- `VERSION_MISMATCH`：旧代次（不可重试）
- `CONFLICT`：状态/CAS或幂等输入变化（不可重试）
- `INTEGRITY_ERROR`：目录、结果引用或回执矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、runtime、identity、签名器或owner确认未知（可重试）
- `INTERNAL`：私有SQL或outbox提交失败（可重试）
- `LOCKED`：数据库占用或维护/安全点暂停（可重试）
- `CANCELLED`：上下文取消（可重试）
- `DEADLINE_EXCEEDED`：IO超时（可重试）

- 副作用：无
- 授权：permission：job.manage；rule：入口job.manage仅粗能力；按持久任务owner/type或事件producer/version目录向identity核当前具体权限；筛选参数仅缩窄
- 并发：SQL短快照后释放事务/进程锁再请求identity或owner；写回重核epoch、作用域与revision/fence CAS
- 幂等：读取游标绑定主体/site/session/epoch/filters；人工操作按caller/epoch/operation及actor/site/input摘要回原回执
- 依赖：`jobs.managementPage`、`jobs.listJobBatch`、`jobs.filterManagementBatch`、`jobs.finishManagementPage`

## jobs.RetryJob

- 级别：约束
- 文件：`modules/jobs/internal/queries.go`
- 作用：人工重试失败或死信任务

```go
func RetryJob(ctx context.Context, op OperationContext, input RetryJobInput) (RetryJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `RetryJobInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RetryJobResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 锁外按持久任务owner/type/epoch核当前具体权限；事务先查caller/epoch/operation绑定actor/site/job的原回执，同op只回原结果，后续状态变化不再check owner或重排
2. 新操作仅当前dead-letter失败终局；orphan、活动、成功、取消意图或残存租约拒绝；读取原inputRef/root与revision/fence后关闭事务请求owner，根未知只可复用completed，不能safeToRetry
3. nil/不可达/pending/unknown失败关闭；completed复用已持久结果，safeToRetry才表示无未决费用且能沿同根复用付费阶段；不调用模型或执行领域算法
4. owner返回后重新核当前runtime/control、原owner/type/epoch/state/revision/fence；并发同op先回原回执；维护/安全点暂停新写入
5. 完整旧ObjectRef与owner新ref任一字段矛盾完整性失败，不静默覆盖；paid IDs去重合并保留，root/input/operation/已有进度不清空
6. safeToRetry原子CAS到queued且attempt=0并提升持久fence，completed收敛succeeded；结果/状态outbox/人工回执同事务，返回真实JobSummary

**错误**

- `FORBIDDEN`：非当前后台主体或具体功能权限不足（不可重试）
- `VALIDATION`：分页游标或路径正文绑定无效（不可重试）
- `VERSION_MISMATCH`：旧代次（不可重试）
- `CONFLICT`：状态/CAS或幂等输入变化（不可重试）
- `INTEGRITY_ERROR`：目录、结果引用或回执矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、runtime、identity、签名器或owner确认未知（可重试）
- `NOT_FOUND`：任务不存在（不可重试）
- `LOCKED`：维护/屏障期间暂停新的人工处理写入（可重试）
- `INTERNAL`：私有SQL或outbox提交失败（可重试）
- `CANCELLED`：上下文取消（可重试）
- `DEADLINE_EXCEEDED`：IO超时（可重试）

- 副作用：CAS任务状态/持久fence；结果合并、状态outbox和人工retry回执同事务
- 授权：permission：job.manage；rule：入口job.manage仅粗能力；按持久任务owner/type或事件producer/version目录向identity核当前具体权限；筛选参数仅缩窄
- 并发：SQL短快照后释放事务/进程锁再请求identity或owner；写回重核epoch、作用域与revision/fence CAS
- 幂等：读取游标绑定主体/site/session/epoch/filters；人工操作按caller/epoch/operation及actor/site/input摘要回原回执
- 依赖：`jobs.authorizeJobManagement`、`jobs.inspectManualRetry`、`jobs.applyManualRetry`

## jobs.ListDeliveries

- 级别：约束
- 文件：`modules/jobs/internal/queries.go`
- 作用：查看死信投递

```go
func ListDeliveries(ctx context.Context, op OperationContext, input ListDeliveriesInput) (ListDeliveriesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ListDeliveriesInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListDeliveriesResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 只批读当前epoch dead-letter的投递ID、事件类型版本、producer/consumer、attempt和安全错误码，不读或返回原payload
2. 按持久producer/type/version匹配生成唯一事件目录，锁外请求该具体管理权限；producer参数只缩窄，未知目录失败关闭
3. 权限过滤后计算hasMore和游标；平台签名绑定当前主体/epoch/producer/limit，每页重新核权

**错误**

- `FORBIDDEN`：非当前后台主体或具体功能权限不足（不可重试）
- `VALIDATION`：分页游标或路径正文绑定无效（不可重试）
- `VERSION_MISMATCH`：旧代次（不可重试）
- `CONFLICT`：状态/CAS或幂等输入变化（不可重试）
- `INTEGRITY_ERROR`：目录、结果引用或回执矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、runtime、identity、签名器或owner确认未知（可重试）
- `INTERNAL`：私有SQL或outbox提交失败（可重试）
- `LOCKED`：数据库占用或维护/安全点暂停（可重试）
- `CANCELLED`：上下文取消（可重试）
- `DEADLINE_EXCEEDED`：IO超时（可重试）

- 副作用：无
- 授权：permission：job.manage；rule：入口job.manage仅粗能力；按持久任务owner/type或事件producer/version目录向identity核当前具体权限；筛选参数仅缩窄
- 并发：SQL短快照后释放事务/进程锁再请求identity或owner；写回重核epoch、作用域与revision/fence CAS
- 幂等：读取游标绑定主体/site/session/epoch/filters；人工操作按caller/epoch/operation及actor/site/input摘要回原回执
- 依赖：`jobs.managementPage`、`jobs.listDeliveryBatch`、`jobs.filterManagementBatch`、`jobs.finishManagementPage`

## jobs.WithManagementKey

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：可信装配注入本服务游标签名密钥

```go
func WithManagementKey(ctx context.Context, key SigningKey) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `key` | `SigningKey` | 启动配置游标签名密钥引用 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 包含可信签名配置的上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 可信装配注入本服务游标签名密钥

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：无

## jobs.managementPage

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：验证后台主体及签名游标并建立完整查询作用域

```go
func managementPage(ctx context.Context, op OperationContext, kind string, filters string, page ManagementPageRequest) (managementCursor, string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `op` | `OperationContext` | 可信操作上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `kind` | `string` | 列表类型 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `filters` | `string` | 规范查询筛选JSON | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `page` | `ManagementPageRequest` | 分页输入 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `position` | `managementCursor` | 已验签位置 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `scope` | `string` | 绑定主体和查询的摘要作用域 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 验证后台主体及签名游标并建立完整查询作用域

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`platform.VerifyCursor`

## jobs.finishManagementPage

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：只按有权条目计算hasMore并签最后返回ID

```go
func finishManagementPage(ctx context.Context, scope string, items []managementItem, limit int) (ManagementPageInfo, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `scope` | `string` | 已绑定的查询作用域 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `items` | `[]managementItem` | 已经当前权限过滤的安全条目 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `limit` | `int` | 规范分页上限 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `pageInfo` | `ManagementPageInfo` | 仅依据有权条目的后续页信息 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 只按有权条目计算hasMore并签最后返回ID

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`platform.SignCursor`

## jobs.listJobBatch

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：批读当前epoch安全任务与最后进度

```go
func listJobBatch(ctx context.Context, op OperationContext, input ListJobsInput, after string) ([]managementItem, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `op` | `OperationContext` | 可信操作上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `input` | `ListJobsInput` | 筛选 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `after` | `string` | 上一批最后扫描ID | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `items` | `[]managementItem` | 最多100条安全元数据 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 批读当前epoch安全任务与最后进度

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.beginLeaseOperation`、`jobs.jobManagementPermission`、`jobs.readJobSummary`

## jobs.listDeliveryBatch

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：批读当前epoch死信元数据，核唯一producer目录

```go
func listDeliveryBatch(ctx context.Context, op OperationContext, input ListDeliveriesInput, after string) ([]managementItem, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `op` | `OperationContext` | 可信操作上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `input` | `ListDeliveriesInput` | 筛选 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `after` | `string` | 上一批最后扫描ID | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `items` | `[]managementItem` | 最多100条安全元数据 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 批读当前epoch死信元数据，核唯一producer目录

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.beginLeaseOperation`、`jobs.findEventDefinition`

## jobs.filterManagementBatch

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：无事务锁下逐条请求具体业务权限，仅过滤FORBIDDEN

```go
func filterManagementBatch(ctx context.Context, op OperationContext, batch []managementItem, needed int) ([]managementItem, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `op` | `OperationContext` | 可信操作上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `batch` | `[]managementItem` | 持久目录确定权限的安全批次 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `needed` | `int` | 本页尚需有权条目数量 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `items` | `[]managementItem` | 当前具体权限允许的前缀 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 调用前SQL批次已关闭；逐条向identity请求持久目录产生的具体业务权限，仅FORBIDDEN跳过；到本页所需limit+1个有权条目即停止，其他权威错误整体失败

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.authorizePermission`

## jobs.inspectManualRetry

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：重放原操作或收集dead-letter安全owner确认原料

```go
func inspectManualRetry(ctx context.Context, op OperationContext, input RetryJobInput, authorized jobRecord) (retryCandidate, *RetryJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `op` | `OperationContext` | 可信操作上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `input` | `RetryJobInput` | 原任务ID | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `authorized` | `jobRecord` | 当前权限已核的持久作用域 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `candidate` | `retryCandidate` | 用于锁外owner确认的持久原料 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `prior` | `*RetryJobResult` | 已有原操作回执或nil | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 重放原操作或收集dead-letter安全owner确认原料

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.readRetryReceipt`、`jobs.requireEventWrite`

## jobs.applyManualRetry

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：owner确认后重核作用域和CAS，原子保存结果、状态、outbox和回执

```go
func applyManualRetry(ctx context.Context, op OperationContext, input RetryJobInput, candidate retryCandidate, recovery OwnerRecoveryResult) (RetryJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `op` | `OperationContext` | 可信操作上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `input` | `RetryJobInput` | 原任务ID | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `candidate` | `retryCandidate` | 原确认快照 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `recovery` | `OwnerRecoveryResult` | 受信owner持久确认 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RetryJobResult` | 原子提交后的真实摘要 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. owner确认后重核作用域和CAS，原子保存结果、状态、outbox和回执

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：任务状态/fence、结果与paid引用合并、状态outbox、原操作回执同事务
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.beginLeaseOperation`、`jobs.readJobRecord`、`jobs.readRetryReceipt`、`jobs.preserveRetryResults`、`jobs.readJobSummary`、`jobs.retryRequestHash`、`jobs.requireEventWrite`、`jobs.enqueueState`

## jobs.readRetryReceipt

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：按actor/site/job摘要读取人工操作回执

```go
func readRetryReceipt(ctx context.Context, op OperationContext, work *leaseOperation, input RetryJobInput) (*RetryJobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `op` | `OperationContext` | 可信操作上下文 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `work` | `*leaseOperation` | 当前短事务 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `input` | `RetryJobInput` | 原任务ID | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `prior` | `*RetryJobResult` | 原操作结果或nil | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 按actor/site/job摘要读取人工操作回执

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.retryRequestHash`

## jobs.retryRequestHash

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：对可信主体/site和jobID求稳定输入摘要

```go
func retryRequestHash(input RetryJobInput, subjectID string, siteID string) string
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `RetryJobInput` | 原任务ID | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `subjectID` | `string` | 可信主体 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `siteID` | `string` | 可信站点 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `hash` | `string` | 规范操作输入摘要 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 对可信主体/site和jobID求稳定输入摘要

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：无

## jobs.preserveRetryResults

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：完整结果任何矛盾失败关闭，既有paid ID合并保留

```go
func preserveRetryResults(ctx context.Context, work *leaseOperation, job jobRecord, recovery OwnerRecoveryResult) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `work` | `*leaseOperation` | 当前短事务 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `job` | `jobRecord` | 原任务作用域 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `recovery` | `OwnerRecoveryResult` | owner持久引用 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 完整结果任何矛盾失败关闭，既有paid ID合并保留

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：只在无矛盾时合并保留完整ObjectRef和paid IDs
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.mergePaidResults`

## jobs.readJobSummary

- 级别：参考
- 文件：`modules/jobs/internal/queries.go`
- 作用：读取当前事务安全任务摘要及最后进度

```go
func readJobSummary(ctx context.Context, work *leaseOperation, jobID string) (JobSummary, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止原样传IO | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `work` | `*leaseOperation` | 当前短事务 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `jobID` | `string` | 当前任务ID | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `job` | `JobSummary` | 安全任务和最后进度摘要 | 仅本域可信装配或已通过统一入口的管理链路使用 |
| `error` | `error` | 安全公共失败，结果无效 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 读取当前epoch任务安全字段与最后进度；root未知映射null，绝不补猜或输出空Identifier，不读取inputRef或任何正文

**错误**

- `VALIDATION`：游标或输入绑定不可用（不可重试）
- `FORBIDDEN`：当前权限或主体不允许（不可重试）
- `VERSION_MISMATCH`：代次变化（不可重试）
- `CONFLICT`：操作或状态/CAS变化（不可重试）
- `INTEGRITY_ERROR`：持久数据或结果引用矛盾（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖或私有IO不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：无

## jobs.NewEventHandlers

- 级别：参考
- 文件：`modules/jobs/internal/http_events.go`
- 作用：装配已实现事件发布、确认、重放和durable SSE的真实HTTP适配

```go
func NewEventHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `EventHandlers` | 四个实际事件operationId处理器 | 只用于统一HTTP边界；其他操作不能用占位填充 |

**步骤**

1. 信任公共NewServiceHandler已建立op/caller/epoch/schema/PathValue，只做生成输入形状到领域输入绑定，不重复身份或schema校验
2. publish/ack/replay转领域函数并用公共Problem写失败，成功状态与OpenAPI一致；actor/epoch只取可信op
3. subscribe成功后调用platform.ServeEventStream输出真实增量流；pump开始后不再写JSON Problem
4. 只返回本组四个operation，不补其余handler假成功；BuildApplication全表/控制存储/后台未完整时保持blocked

**错误**

- 无

- 副作用：由已登记领域函数写事件/投递与回执；SSE由平台输出
- 授权：permission：service；rule：公共NewServiceHandler先固定caller/委托/epoch/维护与静态权限，本组不能独立裸挂
- 并发：每请求独立绑定，流单事件背压由平台pump执行
- 幂等：沿用领域函数与可信Idempotency-Key建立的操作键
- 依赖：`jobs.PublishEvent`、`jobs.AcknowledgeEvent`、`jobs.ReplayDelivery`、`jobs.SubscribeEvents`、`jobs.writeEventResponse`、`platform.ServeEventStream`
- 其他依赖说明：platform.apivalidate.NewServiceHandler已验证请求上下文

## jobs.writeEventResponse

- 级别：参考
- 文件：`modules/jobs/internal/http_events.go`
- 作用：统一本组JSON响应，失败复用平台Problem，不自行泄露原始输入

```go
func writeEventResponse(w http.ResponseWriter, requestID Identifier, result any, err error, status int)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `w` | `ResponseWriter` | HTTP响应句柄 | SSE开始后不调用 |
| `requestID` | `Identifier` | 可信入口产生的requestId标量，不传完整操作上下文 | 入口注入 |
| `result` | `LeaseReceiptResult` | 已验证业务结果 | 失败时忽略 |
| `err` | `error` | 领域错误 | 公共安全错误码 |
| `status` | `int` | 成功HTTP码 | OpenAPI登记码 |

**输出**

无

**步骤**

1. 失败交platform.problem.FromError/WriteProblem，成功按登记码输出JSON；网络写失败结束当前请求，不追加第二个Problem

**错误**

- 无

- 副作用：写HTTP响应
- 授权：permission：service；rule：只由本组已通过统一边界的handler调用
- 并发：单响应单次输出
- 幂等：只序列化已有结果，不增加业务副作用
- 依赖：`platform.FromError`、`platform.WriteProblem`
- 其他依赖说明：platform.problem

## jobs.NewManagementHandlers

- 级别：参考
- 文件：`modules/jobs/internal/http_events.go`
- 作用：提供四项真实管理HTTP映射，公共完整工厂缺项必须拒绝

```go
func NewManagementHandlers(key SigningKey) map[string]http.Handler
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 可信服务签名配置 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `EventHandlers` | 四项真实管理操作映射 | 仅本域可信装配或已通过统一入口的管理链路使用 |

**步骤**

1. 提供listJobs、listDeliveries、retryJob和cancelJob四项真实管理HTTP映射；cancelJob只从可信op与路径绑定RequestCancel，不提供保护控制占位，公共完整工厂缺项必须拒绝

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：经已认证管理绑定链路或可信装配调用
- 并发：短SQL，网络核权不持事务/锁
- 幂等：遵循入口原回执或签名游标规则
- 依赖：`jobs.WithManagementKey`、`jobs.RequestCancel`、`jobs.ListJobs`、`jobs.ListDeliveries`、`jobs.RetryJob`、`jobs.writeEventResponse`

## jobs.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/jobs/internal/snapshot_archive.go`
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

## jobs.ReadQueueSiteID

- 级别：参考
- 文件：`modules/jobs/internal/app/owner_recovery.go`
- 作用：只读当前真实站点身份

```go
func ReadQueueSiteID(ctx context.Context, client *gen.SiteConfigClient, epoch string, validate func(json.RawMessage)error)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅在真实服务读取或待恢复任务需要站点作用域时调用site-config:readSiteConfig，消费严格生产方响应并只取真实config.siteId；未初始化、空值或不可用均失败关闭且不得缓存为默认站点。

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：显式延迟属于去重材料，原成功不重新分配时刻
- 依赖：`site-config:readSiteConfig`

## jobs.NewOwnerRecoveryChecker

- 级别：参考
- 文件：`modules/jobs/internal/app/owner_recovery.go`
- 作用：将真实原任务恢复请求送本域已登记owner只读接口

```go
func NewOwnerRecoveryChecker(config platform.ServiceConfig,key platform.SigningKey,siteID jobs.SiteIDProvider)(jobs.OwnerRecoveryChecker,error)
```

**输入**

无

**输出**

无

**步骤**

1. 当前实际只有knowledge:readJobRecovery；未登记的owner返回依赖不可用并保留orphan，不能凭进程存活或错误判断可重试。
2. 只有选中真实待恢复任务时才按需取得可信siteID；fresh/未初始化站点不触发启动依赖，取得失败保留orphan。随后发起新的5秒纯读取op，actor jobs绑定该siteID、输入原root/epoch；不继承scheduler的system站点、不发Create/Cancel或续费许可。
3. 结果严格按生产方OwnerRecoveryResult解码并保持实际完整paidResultRefs，检查scope/结果引用，不猜unknown为safeToRetry。

**错误**

- 无

- 副作用：无
- 授权：信任严格入口已验证上下文；不新增通用身份/schema/截止校验
- 并发：沿已有单连接短事务与当前运行窗口；HTTP每请求独立绑定
- 幂等：显式延迟属于去重材料，原成功不重新分配时刻
- 依赖：`analytics:readJobRecovery`、`knowledge:readJobRecovery`、`media:readJobRecovery`、`site-config:readSiteConfig`、`platform.NewPayloadValidator`

## jobs.JobsPage

- 级别：约束
- 文件：`modules/jobs/web/src/index.tsx`
- 作用：任务队列：任务列表与详情（进度流断线重连）、死信投递重放、定时调度编辑

```ts
export default function JobsPage(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 按状态与所属服务筛选任务
2. 取消只记录意图，由任务所属服务确认安全终态

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无
