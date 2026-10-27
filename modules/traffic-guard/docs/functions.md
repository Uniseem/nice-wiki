# traffic-guard 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 84 个函数：约束 27 个，参考 57 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## traffic-guard.ValidatePolicy

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy.go`
- 作用：校验风控参数

```go
func ValidatePolicy(ctx context.Context, op OperationContext, in ValidatePolicyInput) (ValidatePolicyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidatePolicyInput` | 校验风控参数；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePolicyResult` | 校验风控参数成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 输入由严格GuardPolicy schema拒绝未知字段，无余额或费用字段
2. 校验主体/站点整数速率、burst≤主体rate与主体/站点并发1..128
3. GuardPolicy未提供挑战难度或ban TTL字段，不在此函数捏造额外配置
4. qaPerNetworkPerMinute兼容可选，缺失规范为600，显式值按站点速率同范围1..100000校验；不推断费用5倍次数关系

**错误**

- `VALIDATION` / HTTP 400：INVALID_POLICY：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：guard.manage；rule：guard.manage 入口内部
- 并发：纯函数
- 幂等：同 JSON 规范化结果一致
- 依赖：`traffic-guard.networkPolicyRate`

## traffic-guard.networkPolicyRate

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy.go`
- 作用：读取已确认兼容网络分钟次数默认600

```go
func networkPolicyRate(policy GuardPolicy) int
```

**输入**

无

**输出**

无

**步骤**

1. 读取已确认兼容网络分钟次数默认600

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：无

## traffic-guard.GetPolicy

- 级别：约束
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：读取生效策略

```go
func GetPolicy(ctx context.Context, op OperationContext, in GetPolicyInput) (GetPolicyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `GetPolicyInput` | 读取生效策略；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetPolicyResult` | 读取生效策略成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. siteId和dataEpoch须等于可信op.Actor.SiteID/op.DataEpoch；GET入口只从可信上下文构造，不新增客户端站点参数
2. guard.manage管理员即时查权，或traffic-guard本域纯服务可读；其他服务拒绝
3. 同代次一致快照读取，缺行NOT_FOUND；不临时构造未保存默认策略

**错误**

- `FORBIDDEN` / HTTP 403：可信站点不匹配或当前权限不足（不可重试）
- `NOT_FOUND` / HTTP 404：当前站点没有已保存策略（不可重试）
- `VERSION_MISMATCH` / HTTP 409：策略修订或active代次变化（不可重试）
- `CONFLICT` / HTTP 409：同操作绑定其他输入（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：存储或即时授权依赖未知（可重试）

- 副作用：无
- 授权：permission：guard.manage；rule：guard.manage 或 service
- 并发：SQLite 快照读
- 幂等：GET 幂等
- 依赖：`traffic-guard.authorizeGuardManagement`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readGuardPolicy`、`traffic-guard.guardTransaction.close`

## traffic-guard.UpdatePolicy

- 级别：约束
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：更新策略 CAS

```go
func UpdatePolicy(ctx context.Context, op OperationContext, in UpdatePolicyInput) (UpdatePolicyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `UpdatePolicyInput` | 更新策略 CAS；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdatePolicyResult` | 更新策略 CAS成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 即时核guard.manage管理员，siteId与epoch必须匹配可信op
2. 校验规范策略后建立当前公共写窗口，进入本域短事务
3. 先按site/epoch/caller/operationId核完整规范输入摘要，同输入返回原revision，不同输入CONFLICT
4. 缺行仅expectedRevision=0可首次创建revision1；已有行严格revision CAS，溢出拒绝
5. 策略、原回执与policy-changed.v1 outbox在同一事务提交；事件失败不提交策略

**错误**

- `FORBIDDEN` / HTTP 403：可信站点不匹配或当前权限不足（不可重试）
- `NOT_FOUND` / HTTP 404：当前站点没有已保存策略（不可重试）
- `VERSION_MISMATCH` / HTTP 409：策略修订或active代次变化（不可重试）
- `CONFLICT` / HTTP 409：同操作绑定其他输入（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：存储或即时授权依赖未知（可重试）

- 副作用：写策略/outbox
- 授权：permission：guard.manage；rule：guard.manage
- 并发：事务 CAS；旧 permit renew 用最新策略重检
- 幂等：operationId 去重且相同 key 不同 body 409
- 依赖：`traffic-guard.authorizeGuardManagement`、`traffic-guard.ValidatePolicy`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.enqueueGuardEvent`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`

## traffic-guard.OpenPolicyStore

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：打开私有SQLite及control journal并固定可信active代次

```go
func OpenPolicyStore(dataRoot string, openedDataEpoch Identifier) (*PolicyStore, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `dataRoot` | `string` | 配置注入的本域绝对数据根 | 仅可信装配或已通过边界的调用链使用 |
| `openedDataEpoch` | `Identifier` | startup runtime的active代次 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `*PolicyStore` | 固定打开代次的私有库 | 仅可信装配或已通过边界的调用链使用 |
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 打开私有SQLite及control journal并固定可信active代次

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无
- 其他依赖说明：platform.outbox.EnsureSchema；单一嵌入SQL迁移集

## traffic-guard.WithPolicyStore

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：可信装配将已打开代次的私有库注入调用上下文

```go
func WithPolicyStore(ctx context.Context, store *PolicyStore) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止透传IO | 仅可信装配或已通过边界的调用链使用 |
| `store` | `*PolicyStore` | 打开代次不可变的私有库 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 带本域私有库的上下文 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 可信装配将已打开代次的私有库注入调用上下文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：仅派生context，不改变库或全局变量
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.RunMigrations

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：逐个事务执行单一embed来源的只追加迁移

```go
func (s *PolicyStore) RunMigrations() error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `s` | `*PolicyStore` | 启动或关闭阶段的私有库接收者 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 逐个事务执行单一embed来源的只追加迁移

**错误**

- `VALIDATION`：句柄缺失（不可重试）
- `INTEGRITY_ERROR`：迁移集、已登记版本或数据库结构损坏（不可重试）
- `INTERNAL`：事务打开/版本登记/提交失败（不可重试）

- 副作用：每个追加迁移与schema_migrations版本同事务提交
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.All`、`traffic-guard.registerGuardWorkTables`

## traffic-guard.RecoverJournal

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：校验control journal并拒绝损坏或未决决定

```go
func (s *PolicyStore) RecoverJournal() error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `s` | `*PolicyStore` | 启动或关闭阶段的私有库接收者 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 校验control journal并拒绝损坏或未决决定

**错误**

- `VALIDATION`：journal句柄缺失（不可重试）
- `INTEGRITY_ERROR`：损坏行或未完成决定（不可重试）
- `INTERNAL`：日志IO失败（不可重试）

- 副作用：无未决/损坏条目才设置JournalRecovered
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.readPolicyJournal`

## traffic-guard.Close

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：等待在途登录事务后关闭journal与数据库

```go
func (s *PolicyStore) Close() error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `s` | `*PolicyStore` | 启动或关闭阶段的私有库接收者 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 等待在途登录事务后关闭journal与数据库

**错误**

- `INTERNAL`：同步或关闭资源失败（不可重试）

- 副作用：等待在途事务释放生命周期锁，关闭journal和DB
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：重复关闭安全，不回退已提交记录
- 依赖：无

## traffic-guard.readPolicyJournal

- 级别：参考
- 文件：`modules/traffic-guard/internal/policy_store.go`
- 作用：按条目ID聚合决定日志并拒绝损坏行

```go
func readPolicyJournal(name string) ([]policyJournalEntry, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `name` | `string` | control日志文件名 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `entries` | `[]policyJournalEntry` | 每个决定的最终状态 | 仅可信装配或已通过边界的调用链使用 |
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 按条目ID聚合决定日志并拒绝损坏行

**错误**

- `INTERNAL`：日志读取失败（不可重试）
- `INTEGRITY_ERROR`：损坏行（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.DeriveSubject

- 级别：参考
- 文件：`modules/traffic-guard/internal/subject.go`
- 作用：生成短期匿名风控主体

```go
func DeriveSubject(ctx context.Context, op OperationContext, in DeriveSubjectInput) (DeriveSubjectResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `DeriveSubjectInput` | 生成短期匿名风控主体；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DeriveSubjectResult` | 生成短期匿名风控主体成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只接受平台入口签名的op.NetworkBudgetScopes，current/previous固定30日、IPv6 /64；缺scope或独立预算key失败关闭，不传原IP
2. 有效visitorToken经平台VerifyGuardSession核SiteID/dataEpoch和subject/session；预算key与前期key只在可信Runtime提供
3. HMAC域分离得到稳定会话/主体scope，网络scope沿原可信别名，匿名未通过时按网络分组限挑战
4. 本域短事务保存匿名映射及当前/前期别名；不存原始IP或凭证token

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：SUBJECT_KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service ingress
- 并发：pepper 外部密钥容器，30 日轮换同时检查前一 epoch 未过期 ban
- 幂等：同 keyEpoch 同输入同主体
- 依赖：`platform.VerifyGuardSession`、`platform.PrivateOperationDigest`、`traffic-guard.beginGuardTransaction`、`traffic-guard.deriveSessionScopes`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`
- 其他依赖说明：identity visitor token verifier

## traffic-guard.deriveSessionScopes

- 级别：参考
- 文件：`modules/traffic-guard/internal/subject.go`
- 作用：通过独立预算key及保留旧key派生主体/会话scope

```go
func deriveSessionScopes(runtime GuardRuntime, site, sessionID, expectedSubject string) (string, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 通过独立预算key及保留旧key派生主体/会话scope

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：`platform.PrivateOperationDigest`

## traffic-guard.ClassifyAction

- 级别：参考
- 文件：`modules/traffic-guard/internal/classification.go`
- 作用：选择可信风控动作

```go
func ClassifyAction(ctx context.Context, op OperationContext, in ClassifyActionInput) (ClassifyActionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ClassifyActionInput` | 选择可信风控动作；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ClassifyActionResult` | 选择可信风控动作成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只接受qa.ask、qa.followup、qa.conversation、localization.suggest
2. 三个QA动作均ownerCaller=qa并执行AI挑战/ban且共享qa次数桶；qa.conversation只创建免费会话，不签模型token
3. 语言提示ownerCaller=localization，仅速率/并发，不要求AI挑战/AI ban，不创建费用授权

**错误**

- `VALIDATION` / HTTP 400：未知固定动作（不可重试）

- 副作用：无
- 授权：permission：service；rule：service
- 并发：只读注册表
- 幂等：同路由匹配同结果
- 依赖：无
- 其他依赖说明：服务注册配置

## traffic-guard.LoadSignals

- 级别：参考
- 文件：`modules/traffic-guard/internal/signals.go`
- 作用：读取有限匿名信号

```go
func LoadSignals(ctx context.Context, op OperationContext, in LoadSignalsInput) (LoadSignalsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `LoadSignalsInput` | 读取有限匿名信号；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadSignalsResult` | 读取有限匿名信号成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 同一已保存主体快照读取当前网络及前期别名的active ban，按服务时间即时判过期，不等sweeper
2. 通过证明未过10分钟、级别匹配且可疑模式少于5问才challengePassed；可疑状态保留10分钟
3. 读取近10分钟拒绝、最近最多8项问题hash重复、真实量化相似度与实测间隔次数；缺失观测保持未知
4. 过期abuse ban不让旧model_verdict永久拒绝，公开内容不受AI ban影响

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service
- 并发：统一事务读取一致 policy revision
- 幂等：只读
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`
- 其他依赖说明：私有 SQLite

## traffic-guard.ComputeRisk

- 级别：参考
- 文件：`modules/traffic-guard/internal/risk.go`
- 作用：确定挑战规则

```go
func ComputeRisk(ctx context.Context, op OperationContext, in ComputeRiskInput) (ComputeRiskResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ComputeRiskInput` | 确定挑战规则；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ComputeRiskResult` | 确定挑战规则成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 输出严格allow/deny/challenge；activeBan或abuse为deny
2. suspicious未通过匹配级别挑战时challenge难度22，通过且本域5问计数仍有效才allow
3. normal未通过时challenge难度18，通过则allow；缺失数值观测未知，不额外加分或新增阈值模型

**错误**

- `VALIDATION` / HTTP 400：INVALID_SIGNALS：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service 内部
- 并发：纯函数，不依据 UA 国别作身份判断
- 幂等：确定性同信号同结果
- 依赖：无
- 其他依赖说明：qa前置模型风险结果（已有一次调用），memory项目概述由qa取用

## traffic-guard.RefillBucket

- 级别：参考
- 文件：`modules/traffic-guard/internal/bucket.go`
- 作用：整数补充令牌

```go
func RefillBucket(ctx context.Context, op OperationContext, in RefillBucketInput) (RefillBucketResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RefillBucketInput` | 整数补充令牌；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RefillBucketResult` | 整数补充令牌成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. elapsedMs夹在0..60000，容量与速率取已校验策略边界1..100000
2. 补充floor(elapsedMs*ratePerMinute/60)千分令牌，结果不超过capacity*1000；时钟倒退补0

**错误**

- `VALIDATION` / HTTP 400：INVALID_BUCKET：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数；使用单调时钟差映射持久 wall-time，时钟倒退补0
- 幂等：同状态同结果
- 依赖：无

## traffic-guard.ConsumeBuckets

- 级别：参考
- 文件：`modules/traffic-guard/internal/bucket_store.go`
- 作用：原子扣频率令牌

```go
func ConsumeBuckets(ctx context.Context, op OperationContext, in ConsumeBucketsInput) (ConsumeBucketsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ConsumeBucketsInput` | 原子扣频率令牌；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeBucketsResult` | 原子扣频率令牌成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 同一事务读取会话、可信30日网络别名和站点三层桶及已保存策略，cost固定1个频率令牌
2. qaPerNetworkPerMinute缺失明确取兼容默认600，与站点默认相同，不采用费用5倍关系
3. 当前/前期网络别名共同代表同一网络，逐别名检查与扣减保持窗口轮换连续性
4. 补充整数千分令牌；时钟倒退补0且不倒退持久水位；三层及所有别名均足够才写入，否则全部不扣并返回最大等待
5. 正结果回执去重；负结果不锁死原请求，复用准入外层事务并发失败整体回滚；没有金额状态
6. qa.ask和qa.followup共享同一qa频率桶类别，防止两动作分别计数使额度翻倍；permit动作仍保留单次原问题语义

**错误**

- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写匿名 token bucket
- 授权：permission：service；rule：service
- 并发：SQLite 单事务；不能分别扣一层失败另一层
- 幂等：requestId 去重不重复扣
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.readGuardPolicy`、`traffic-guard.RefillBucket`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.networkPolicyRate`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`

## traffic-guard.AcquireConcurrency

- 级别：参考
- 文件：`modules/traffic-guard/internal/permit_store.go`
- 作用：创建持久并发租约

```go
func AcquireConcurrency(ctx context.Context, op OperationContext, in AcquireConcurrencyInput) (AcquireConcurrencyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AcquireConcurrencyInput` | 创建持久并发租约；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcquireConcurrencyResult` | 创建持久并发租约成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 同准入事务核原requestId=op.RootOperationID及已验证主体/原动作；QA owner=qa，免费suggest owner=localization
2. 只忽略已过期租约统计，按最新policy检查subject/site并发，QA强制每session1及FIFO队头
3. 插入30秒lease与300秒hard deadline，保存原根/完整scope与网络别名；同根同输入返回原行
4. 只分配元数据token为空，不赋模型调用权；桶/并发任一步失败整体回滚

**错误**

- `RATE_LIMITED` / HTTP 429：CONCURRENCY_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 permit 并发占位
- 授权：permission：service；rule：service
- 并发：与 bucket 消耗同最终 admission 事务；失败回滚扣令牌
- 幂等：requestId 唯一返回原 permit
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.readPermitRecord`、`traffic-guard.readGuardPolicy`、`traffic-guard.ClassifyAction`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`
- 其他依赖说明：私有 SQLite

## traffic-guard.RenewPermit

- 级别：约束
- 文件：`modules/traffic-guard/internal/permit_store.go`
- 作用：延长活跃并发租约

```go
func RenewPermit(ctx context.Context, op OperationContext, in RenewPermitInput) (RenewPermitResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenewPermitInput` | 延长活跃并发租约；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenewPermitResult` | 延长活跃并发租约成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 核持久ownerCaller及epoch，原QA request必须等于op.RootOperationID，QA必须已消费
2. 即时重查当前网络AI ban与最新policy并发；free suggest不套AI ban/挑战，不签模型token
3. 同operationId绑定原admission，重复返回同获准expiry；过期/释放/撤销拒绝，不能复活
4. CAS expiresAt=min(now+30s,hardDeadline)，回执只存获准期限不存token；QA随后平台重签同期限，不扣桶/钱
5. 免费会话qa.conversation和语言提示只续原持久租约并返回空token；永不签模型凭证

**错误**

- `STALE_RESULT` / HTTP 409：PERMIT_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：ACTIVE_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 expiresAt
- 授权：permission：service；rule：service 原 audience
- 并发：admission revision CAS，与 release 互斥
- 幂等：同续租 operationId 返回同 expiry
- 依赖：`traffic-guard.CheckBan`、`traffic-guard.SignPermit`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readPermitRecord`、`traffic-guard.readGuardPolicy`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.permitMetadata`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`

## traffic-guard.ReleasePermit

- 级别：约束
- 文件：`modules/traffic-guard/internal/permit_store.go`
- 作用：释放并发占位

```go
func ReleasePermit(ctx context.Context, op OperationContext, in ReleasePermitInput) (ReleasePermitResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReleasePermitInput` | 释放并发占位；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleasePermitResult` | 释放并发占位成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验服务绑定
2. CAS active→released
3. 不返还已经消耗的频率令牌
4. 核原owner/site/epoch/root；终止、过期、已撤销都已不占并发，幂等Released=true，不能使QA完成或取消重放永久失败

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 lease state
- 授权：permission：service；rule：service 原 audience
- 并发：release/renew 相同事务序列
- 幂等：重复 released 返回 true
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.readPermitRecord`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`
- 其他依赖说明：私有 SQLite

## traffic-guard.readPermitRecord

- 级别：参考
- 文件：`modules/traffic-guard/internal/permit_store.go`
- 作用：按可信站点/代次读取完整持久许可及捕获网络别名

```go
func readPermitRecord(t *guardTransaction, id string) (permitRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信站点/代次读取完整持久许可及捕获网络别名

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：无

## traffic-guard.permitMetadata

- 级别：参考
- 文件：`modules/traffic-guard/internal/permit_store.go`
- 作用：只构造空token分配元数据，不产生模型凭证

```go
func permitMetadata(row permitRecord, epoch string) platform.AdmissionPermit
```

**输入**

无

**输出**

无

**步骤**

1. 只构造空token分配元数据，不产生模型凭证

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：无

## traffic-guard.CheckBan

- 级别：参考
- 文件：`modules/traffic-guard/internal/permit_store.go`
- 作用：对原许可网络别名即时核有效AI禁令并取最晚期限

```go
func CheckBan(t *guardTransaction, networks []string) (bool, int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 对原许可网络别名即时核有效AI禁令并取最晚期限

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：无

## traffic-guard.SignPermit

- 级别：参考
- 文件：`modules/traffic-guard/internal/permit_sign.go`
- 作用：签名准入凭证

```go
func SignPermit(ctx context.Context, op OperationContext, in SignPermitInput) (SignPermitResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `SignPermitInput` | 签名准入凭证；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SignPermitResult` | 签名准入凭证成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只读取本域同站点/epoch持久lease；QA已consumed_request_id=原root、active且未过期才可签
2. 完整DTO从持久原subject/session/network/action/root组成，audience固定model-gateway；允许的receipt expiresAt只能早于等于当前获准期限
3. 只调用平台SignAdmissionPermit，不持久token；免费localization.suggest永不签模型token
4. 只有qa.ask/qa.followup可签nw-admission-v1；免费qa.conversation即使已消费也拒绝签名

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：SIGNING_KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service 内部
- 并发：只读密钥容器与 admission 快照
- 幂等：同 admission 可重复生成等价 token
- 依赖：`platform.SignAdmissionPermit`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readPermitRecord`、`traffic-guard.permitMetadata`、`traffic-guard.guardTransaction.close`
- 其他依赖说明：外部签名密钥

## traffic-guard.ValidatePermit

- 级别：参考
- 文件：`modules/traffic-guard/internal/permit_sign.go`
- 作用：消费前验证租约

```go
func ValidatePermit(ctx context.Context, op OperationContext, in ValidatePermitInput) (ValidatePermitResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidatePermitInput` | 消费前验证租约；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePermitResult` | 消费前验证租约成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅QA；平台VerifyAdmissionPermit验原token，逐原DTO字段/原RootOperationID/epoch/QA action核对
2. 查询同站同代active已消费lease及原ownerCaller，旧签名期限只能小于等于持久已获准期限
3. 核本次lease固有网络别名的当前AI ban；不续租、不扣桶或费用

**错误**

- `VALIDATION` / HTTP 400：PERMIT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：PERMIT_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：ACTIVE_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service 身份绑定
- 并发：服务在线查询不能只信过期的本地签名状态
- 幂等：只读重复验证
- 依赖：`platform.VerifyAdmissionPermit`、`traffic-guard.CheckBan`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readPermitRecord`、`traffic-guard.permitMetadata`、`traffic-guard.guardTransaction.close`

## traffic-guard.EvaluateAdmission

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：编排频率/挑战/并发

```go
func EvaluateAdmission(ctx context.Context, op OperationContext, in EvaluateAdmissionInput) (EvaluateAdmissionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `EvaluateAdmissionInput` | 编排频率/挑战/并发；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EvaluateAdmissionResult` | 编排频率/挑战/并发成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 原requestId必须等于可信op.RootOperationID；只用本域已验证subject，绑定同根规范输入防重绑
2. QA动作查首次/可疑挑战与当前网络AI ban；localization.suggest免费仅速率/并发，不套AI挑战/ban
3. 同事务双桶+subject/site并发+QA session1/FIFO；失败回滚已扣桶，拒绝可在解除限制后同根重试而不重复计数
4. 只保存成功分配的原行，拒绝审计按root/原因/策略去重；不隐式创建挑战
5. allow返回token为空的持久分配元数据；只有ConsumeAdmission之后平台签名才有模型凭证资格，guard不控制费用

**错误**

- `FORBIDDEN` / HTTP 403：ACTIVE_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `RATE_LIMITED` / HTTP 429：CHALLENGE_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `RATE_LIMITED` / HTTP 429：CONCURRENCY_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 token bucket/permit/outbox denial 引用
- 授权：permission：service；rule：service 白名单
- 并发：每站点事务；没有费用预占
- 幂等：requestId+inputHash 幂等，不同 body 冲突
- 依赖：`traffic-guard.ClassifyAction`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.readGuardPolicy`、`traffic-guard.LoadSignals`、`traffic-guard.ComputeRisk`、`traffic-guard.ConsumeBuckets`、`traffic-guard.AcquireConcurrency`、`traffic-guard.readPermitRecord`、`traffic-guard.permitMetadata`、`traffic-guard.recordAdmissionDenial`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`

## traffic-guard.BeginLoginAttempt

- 级别：约束
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：账号及网络登录失败保护BeginLoginAttempt

```go
func BeginLoginAttempt(ctx context.Context, op OperationContext, in BeginLoginAttemptInput) (BeginLoginAttemptResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 请求IO取消 | 原样传下游 |
| `op` | `OperationContext` | 经入口认证的identity服务上下文 | 来源及epoch不取正文 |
| `in` | `BeginLoginAttemptInput` | 已严格校验输入 | 不含原IP或秘密 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BeginLoginAttemptResult` | 完整持久结果 | 失败无结果 |
| `error` | `Error` | 公共安全错误 | 不回显原值 |

**步骤**

1. 要求identity纯服务，事务绑定PolicyStore打开代次；通用服务断言、当前runtime/维护与schema只由公共HTTP入口核验一次
2. requestId必须等于op.OperationID；networkKeys排序去重后与accountKey/action作规范摘要；按caller+epoch+requestId查原回执，同摘要返回原attempt、expiresAt和创建revision，包括已到期/终局，不增加预留也不延长
3. login/bootstrap共享账号和每个网络键：检查当前固定锁及滚动15分钟失败；网络20次即拒，账号5次后不拒绝而要求附一份新解出的login-pow挑战并在本事务核验消费（一次挑战换一次尝试）；既有锁及重放不重算锁起点或续期
4. 同事务检查未终局且expiresAt晚于当前服务时间的预留：账号最多1、每个网络键最多8；持久新attempt和全部网络关联，30秒到期；过期pending自然不占并发、不自动当失败
5. 提交成功才返回预留；未知存储失败关闭，不记录原账号/IP/口令或HMAC原料

**错误**

- `FORBIDDEN`：非identity纯服务调用（不可重试）
- `VERSION_MISMATCH`：op代次与打开的active库或持久attempt不一致（不可重试）
- `CONFLICT`：同operation/request异输入、异终局或过期authenticated（不可重试）
- `DEPENDENCY_UNAVAILABLE`：私有库未注入、已关闭或事务失败（可重试）
- `RATE_LIMITED`：任一网络失败阈值/锁已满，或账号/网络并发已满（不可重试）
- `CHALLENGE_REQUIRED`：账号失败达到阈值或在锁期内，且未附有效、未用过的login-pow挑战（不可重试）

- 副作用：本域SQLite保留、计数及回执事务
- 授权：permission：service；rule：只允许已认证CallerService=identity且Actor.Kind=service；可信入口核当前runtime，事务严格匹配PolicyStore打开代次，正文不声明身份或epoch
- 并发：单连接SQLite短事务串行；锁和失败窗按账号/每个网络键共享login/bootstrap，不持锁核验口令
- 幂等：原request/attempt和输入摘要绑定，终局不得改写
- 依赖：`traffic-guard.beginLoginTransaction`、`traffic-guard.loginInputHash`、`traffic-guard.checkLoginScope`、`traffic-guard.loginDBError`、`traffic-guard.loginTransaction.close`
- 其他依赖说明：本域私有登录尝试/失败/回执持久表

## traffic-guard.FinishLoginAttempt

- 级别：约束
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：账号及网络登录失败保护FinishLoginAttempt

```go
func FinishLoginAttempt(ctx context.Context, op OperationContext, attemptId Identifier, in FinishLoginAttemptInput) (FinishLoginAttemptResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 请求IO取消 | 原样传下游 |
| `op` | `OperationContext` | 经入口认证的identity服务上下文 | 来源及epoch不取正文 |
| `attemptId` | `Identifier` | 已校验路径尝试ID | 只认本域持久原尝试 |
| `in` | `FinishLoginAttemptInput` | 已严格校验输入 | 不含原IP或秘密 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FinishLoginAttemptResult` | 完整持久结果 | 失败无结果 |
| `error` | `Error` | 公共安全错误 | 不回显原值 |

**步骤**

1. 要求identity纯服务和打开代次；按持久attempt读取caller/epoch/账号及全部网络键，不从请求正文推导；旧代次禁止触碰当前active
2. Finish自身按caller+epoch+operationId绑定attempt/outcome，与Begin的operation命名空间分开；同attempt同终局即使换key也只补回执并返回原接受结果，异终局始终CONFLICT
3. pending超过30秒只禁止首次authenticated；首次late rejected仍接受并按本次Finish受信服务时间计入滚动15分钟窗；aborted无论是否到期只释放预留
4. authenticated只说明口令核验且账户active：清此账号失败及账号锁，不清任一网络失败/锁；后续session提交失败不可改写此终局
5. rejected对账号及每个持久网络键追加一次匿名失败；账号5次/网络20次阈值首次达到时锁15分钟；正在锁定时只追加失败不续锁，锁结束后按滚动窗正常判定；不因重放续锁
6. 终局、失败/锁或账号清除、预留释放及operation回执同事务提交；原终局不可改写，失败不返回半结果

**错误**

- `FORBIDDEN`：非identity纯服务调用（不可重试）
- `VERSION_MISMATCH`：op代次与打开的active库或持久attempt不一致（不可重试）
- `CONFLICT`：同operation/request异输入、异终局或过期authenticated（不可重试）
- `DEPENDENCY_UNAVAILABLE`：私有库未注入、已关闭或事务失败（可重试）
- `NOT_FOUND`：持久attempt不存在（不可重试）

- 副作用：本域SQLite保留、计数及回执事务
- 授权：permission：service；rule：只允许已认证CallerService=identity且Actor.Kind=service；可信入口核当前runtime，事务严格匹配PolicyStore打开代次，正文不声明身份或epoch
- 并发：单连接SQLite短事务串行；锁和失败窗按账号/每个网络键共享login/bootstrap，不持锁核验口令
- 幂等：原request/attempt和输入摘要绑定，终局不得改写
- 依赖：`traffic-guard.beginLoginTransaction`、`traffic-guard.loadLoginAttempt`、`traffic-guard.recordLoginFailure`、`traffic-guard.loginDBError`、`traffic-guard.loginTransaction.close`
- 其他依赖说明：本域私有登录尝试/失败/回执持久表

## traffic-guard.beginLoginTransaction

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：取得可信注入库并匹配identity纯服务与打开代次后开始短事务

```go
func beginLoginTransaction(ctx context.Context, op OperationContext) (*loginTransaction, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止透传IO | 仅可信装配或已通过边界的调用链使用 |
| `op` | `OperationContext` | 经统一入口建立的可信操作上下文 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `transaction` | `*loginTransaction` | 已持有生命周期读锁的私有短事务 | 仅可信装配或已通过边界的调用链使用 |
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 取得可信注入库并匹配identity纯服务与打开代次后开始短事务

**错误**

- `FORBIDDEN`：非identity纯服务（不可重试）
- `VERSION_MISMATCH`：op不等于打开代次（不可重试）
- `DEPENDENCY_UNAVAILABLE`：库缺失、关闭或事务打开失败（可重试）

- 副作用：持有库生命周期读锁与SQLite事务，调用方必须defer close
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.loginDBError`

## traffic-guard.loginTransaction.close

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：事务回滚（已提交为无副作用）并释放库生命周期读锁

```go
func (t *loginTransaction) close()
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `t` | `*loginTransaction` | 当前事务接收者 | 仅可信装配或已通过边界的调用链使用 |

**输出**

无

**步骤**

1. 事务回滚（已提交为无副作用）并释放库生命周期读锁

**错误**

- 无

- 副作用：未提交事务回滚并释放生命周期读锁；已提交事务不撤销
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.loginInputHash

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：对账号、action及排序去重网络键作稳定SHA-256摘要

```go
func loginInputHash(in BeginLoginAttemptInput) (string, []string)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `in` | `BeginLoginAttemptInput` | 已严格校验的HMAC输入 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `hash` | `string` | 规范输入SHA-256摘要 | 仅可信装配或已通过边界的调用链使用 |
| `networkKeys` | `[]string` | 排序去重的网络键集合 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 对账号、action及排序去重网络键作稳定SHA-256摘要

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：纯函数，不修改输入切片
- 幂等：相同集合与字段产生相同摘要
- 依赖：无

## traffic-guard.checkLoginScope

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：读取滚动失败、固定锁和未过期预留，任一满额拒绝

```go
func checkLoginScope(ctx context.Context, t *loginTransaction, kind string, key string, failureLimit int, concurrencyLimit int) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止透传IO | 仅可信装配或已通过边界的调用链使用 |
| `t` | `*loginTransaction` | 当前短事务 | 仅可信装配或已通过边界的调用链使用 |
| `kind` | `string` | 本域固定account/network | 仅可信装配或已通过边界的调用链使用 |
| `key` | `string` | 单个HMAC作用域键 | 仅可信装配或已通过边界的调用链使用 |
| `failureLimit` | `int` | 固定账号5/网络20 | 仅可信装配或已通过边界的调用链使用 |
| `concurrencyLimit` | `int` | 固定账号1/网络8 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 读取滚动失败、固定锁和未过期预留，任一满额拒绝

**错误**

- `DEPENDENCY_UNAVAILABLE`：私有存储或IO不可用（可重试）
- `RATE_LIMITED`：有效固定锁、滚动窗失败或未过期预留满额（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.loginDBError`

## traffic-guard.loadLoginAttempt

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：仅从同代次持久attempt及网络关联读取终局原料

```go
func loadLoginAttempt(ctx context.Context, t *loginTransaction, attemptID Identifier) (loginAttemptRecord, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止透传IO | 仅可信装配或已通过边界的调用链使用 |
| `t` | `*loginTransaction` | 当前短事务 | 仅可信装配或已通过边界的调用链使用 |
| `attemptID` | `Identifier` | 持久尝试引用 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `attempt` | `loginAttemptRecord` | 原尝试账号/网络/终局/到期 | 仅可信装配或已通过边界的调用链使用 |
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 仅从同代次持久attempt及网络关联读取终局原料

**错误**

- `DEPENDENCY_UNAVAILABLE`：私有存储或IO不可用（可重试）
- `NOT_FOUND`：不存在（不可重试）
- `VERSION_MISMATCH`：原尝试不属于打开代次（不可重试）
- `FORBIDDEN`：原尝试非identity来源（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.loginDBError`

## traffic-guard.recordLoginFailure

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：写匿名失败并只在无有效锁时按阈值固定首次15分钟锁

```go
func recordLoginFailure(ctx context.Context, t *loginTransaction, attemptID Identifier, kind string, key string, threshold int) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止透传IO | 仅可信装配或已通过边界的调用链使用 |
| `t` | `*loginTransaction` | 当前短事务 | 仅可信装配或已通过边界的调用链使用 |
| `attemptID` | `Identifier` | 原尝试 | 仅可信装配或已通过边界的调用链使用 |
| `kind` | `string` | 本域固定account/network | 仅可信装配或已通过边界的调用链使用 |
| `key` | `string` | 持久原HMAC作用域键 | 仅可信装配或已通过边界的调用链使用 |
| `threshold` | `int` | 固定账号5/网络20 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 写匿名失败并只在无有效锁时按阈值固定首次15分钟锁

**错误**

- `DEPENDENCY_UNAVAILABLE`：私有存储或IO不可用（可重试）

- 副作用：追加匿名失败；阈值已到且无有效锁时写固定15分钟锁，不延长有效锁
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.loginDBError`

## traffic-guard.loginDBError

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：把私有SQL错误转换为不含敏感详情的公共依赖错误

```go
func loginDBError() error
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败时返回安全公共错误，结果无效 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 把私有SQL错误转换为不含敏感详情的公共依赖错误

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.recordAdmissionDenial

- 级别：参考
- 文件：`modules/traffic-guard/internal/admission.go`
- 作用：原根/原因/策略去重，私有匿名计数与安全拒绝事件同事务

```go
func recordAdmissionDenial(t *guardTransaction, request, action, reason string, revision int64, network string) error
```

**输入**

无

**输出**

无

**步骤**

1. 原根/原因/策略去重，私有匿名计数与安全拒绝事件同事务

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：`platform.EnqueueEvent`

## traffic-guard.HandleAdmission

- 级别：约束
- 文件：`modules/traffic-guard/internal/http_admission.go`
- 作用：准入 HTTP

```go
func HandleAdmission(ctx context.Context, op OperationContext, in HandleAdmissionInput) (HandleAdmissionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleAdmissionInput` | 准入 HTTP；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleAdmissionResult` | 准入 HTTP成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅portal可信服务与签名预算scope；requestId=op.RootOperationID，动作封闭白名单
2. DeriveSubject验证通过证明并建立匿名scope，再EvaluateAdmission
3. 需挑战返回CHALLENGE_REQUIRED/429，不隐式issue；caller沿既有issueChallenge领取nonce
4. 纯频率/并发RATE_LIMITED，有真实等待值时声明Retry-After；AI ban FORBIDDEN；allow只返回token空的分配元数据

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CHALLENGE_REQUIRED` / HTTP 429：QA需既有人机挑战，未隐式创建挑战（不可重试）
- `RATE_LIMITED` / HTTP 429：纯频率或并发已满；仅实际频率等待附Retry-After（可重试）

- 副作用：同 EvaluateAdmission，不泄漏 subjectKey
- 授权：permission：service；rule：service
- 并发：请求deadline 1500ms，失败不颁 permit
- 幂等：requestId 幂等
- 依赖：`traffic-guard.EvaluateAdmission`、`traffic-guard.DeriveSubject`

## traffic-guard.NewLoginHandlers

- 级别：参考
- 文件：`modules/traffic-guard/internal/http_admission.go`
- 作用：提供两个真实登录业务handler映射；只供公共严格ServiceHandler装配，不能作为完整服务

```go
func NewLoginHandlers(store *PolicyStore) map[string]http.Handler
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `*PolicyStore` | 可信装配固定打开代次的库 | 仅可信装配或已通过边界的调用链使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `LoginHandlers` | 两个真实operationId处理器 | 仅可信装配或已通过边界的调用链使用 |

**步骤**

1. 只供公共apivalidate.NewServiceHandler装配；固定caller白名单identity，入口统一核服务断言、runtime/维护、schema和PathValue；本组不能裸挂
2. 绑定Begin/Finish输入，注入可信本域库并调用领域函数；只输出持久成功结果或公共Problem
3. 仅返回两个完成handler；完整其他handler/后台/控制恢复仍缺时不声明完整Application或ready

**错误**

- 无

- 副作用：每请求领域事务及HTTP输出
- 授权：permission：service；rule：公共统一边界限定identity；领域函数再执行特有的identity纯服务要求
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.WithPolicyStore`、`traffic-guard.BeginLoginAttempt`、`traffic-guard.FinishLoginAttempt`、`traffic-guard.writeLoginResponse`

## traffic-guard.writeLoginResponse

- 级别：参考
- 文件：`modules/traffic-guard/internal/http_admission.go`
- 作用：输出完整JSON结果或公共安全Problem

```go
func writeLoginResponse(w http.ResponseWriter, requestID Identifier, result any, err error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `w` | `ResponseWriter` | HTTP输出句柄 | 仅可信装配或已通过边界的调用链使用 |
| `requestID` | `Identifier` | 可信请求标量引用 | 仅可信装配或已通过边界的调用链使用 |
| `result` | `LoginResponse` | 持久业务结果，失败时忽略 | 仅可信装配或已通过边界的调用链使用 |
| `err` | `error` | 安全领域错误 | 仅可信装配或已通过边界的调用链使用 |

**输出**

无

**步骤**

1. 输出完整JSON结果或公共安全Problem

**错误**

- 无

- 副作用：一次HTTP响应，不记录原输入
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：单请求响应
- 幂等：只序列化已有结果，不增加持久副作用
- 依赖：无

## traffic-guard.admissionRateError.Error

- 级别：参考
- 文件：`modules/traffic-guard/internal/http_admission.go`
- 作用：转发公共限流错误文本，不泄漏内部状态

```go
func (e *admissionRateError) Error() string
```

**输入**

无

**输出**

无

**步骤**

1. 转发公共限流错误文本，不泄漏内部状态

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：无

## traffic-guard.admissionRateError.Unwrap

- 级别：参考
- 文件：`modules/traffic-guard/internal/http_admission.go`
- 作用：保持公共Problem错误码，同时供HTTP提取真实等待时间

```go
func (e *admissionRateError) Unwrap() error
```

**输入**

无

**输出**

无

**步骤**

1. 保持公共Problem错误码，同时供HTTP提取真实等待时间

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域组合调用，继承入口与外层事务保证
- 并发：复用调用方事务；纯函数不检查取消
- 幂等：继承原根/原记录约束
- 依赖：无

## traffic-guard.ExpireLeases

- 级别：参考
- 文件：`modules/traffic-guard/internal/sweeper.go`
- 作用：回收过期并发

```go
func ExpireLeases(ctx context.Context, op OperationContext, in ExpireLeasesInput) (ExpireLeasesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ExpireLeasesInput` | 回收过期并发；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExpireLeasesResult` | 回收过期并发成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按当前服务时间最多500个active已到期lease CAS expired，不退桶、不外调

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 expired state
- 授权：permission：service；rule：service worker
- 并发：批次事务；分布多实例通过 SQLite 单写协调
- 幂等：重复 sweep 无新增效果
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`
- 其他依赖说明：私有 SQLite

## traffic-guard.ExpireBans

- 级别：参考
- 文件：`modules/traffic-guard/internal/sweeper.go`
- 作用：回收临时禁令

```go
func ExpireBans(ctx context.Context, op OperationContext, in ExpireBansInput) (ExpireBansResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ExpireBansInput` | 回收临时禁令；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExpireBansResult` | 回收临时禁令成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 同批事务扫描最多500个到期active ban，CAS expired并写ban-changed outbox；幂等重扫无新增效果

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写状态/outbox
- 授权：permission：service；rule：service
- 并发：小事务，无同时策略修订
- 幂等：相同时间重扫幂等
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.enqueueGuardEvent`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`
- 其他依赖说明：私有 SQLite

## traffic-guard.IssueChallenge

- 级别：约束
- 文件：`modules/traffic-guard/internal/challenge.go`
- 作用：签发挑战

```go
func IssueChallenge(ctx context.Context, op OperationContext, in IssueChallengeInput) (IssueChallengeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `IssueChallengeInput` | 签发挑战；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `IssueChallengeResult` | 签发挑战成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. HTTP入口只接受portal，由DeriveSubject绑定可信主体，不接自报subject/network/IP
2. 按已保存policy核pow/turnstile开关；Turnstile必须已装配siteKey/secretRef/expectedHostname
3. 读取原请求幂等绑定，同key不同method返回CONFLICT，原有效issued行重构原nonce与期限
4. 通过LoadSignals/ComputeRisk拒绝有效AI ban；正常18bits，可疑22bits并要求interactive
5. 每主体6次/分钟、最多2个有效issued；保存120秒挑战和nonce哈希，nonce由独立预算HMAC派生且支持旧key过渡，不保存token
6. 返回method/公共siteKey/interactive供既有浏览器Turnstile及加载5秒失败后的PoW流程使用

**错误**

- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：CHALLENGE_DISABLED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 challenge 哈希
- 授权：permission：anonymous；rule：anonymous，经 ingress 派生主体
- 并发：同主体挑战最多2个 active
- 幂等：requestId 幂等，同 key method不同409
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`、`traffic-guard.readGuardPolicy`、`traffic-guard.LoadSignals`、`traffic-guard.ComputeRisk`、`traffic-guard.readChallengeRecord`、`traffic-guard.checkIssuedChallenge`、`traffic-guard.challengeNonce`、`traffic-guard.challengeResult`
- 其他依赖说明：Cloudflare Turnstile配置密钥ref

## traffic-guard.VerifyChallenge

- 级别：约束
- 文件：`modules/traffic-guard/internal/challenge.go`
- 作用：核验并消费挑战

```go
func VerifyChallenge(ctx context.Context, op OperationContext, in VerifyChallengeInput) (VerifyChallengeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `VerifyChallengeInput` | 核验并消费挑战；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifyChallengeResult` | 核验并消费挑战成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. HTTP入口DeriveSubject以原可选访客cookie及签名网络scope绑定主体，内部sessionId不接受用户自报
2. 短快照读取同站同代挑战，核issued/同subject/120秒/最多5次失败，释放SQL事务后才验PoW或调用真实外部供应方
3. PoW单次SHA256；Turnstile固定HTTPS、2.5秒、严格hostname/action/cdata/五分钟期限，未知外部结果不当成功
4. 外部验证后重新获取当前写窗口和事务，重读issued及失败次数；无效证明仅持久增加失败次数
5. 成功时即时核新ban，再CAS issued→consumed并写10分钟通过状态；首次匿名证明生成独立会话ID，原有效会话保持本人scope，凭证/session原值不落盘
6. 通过证明仅复用平台SignGuardSession；事务提交后才向portal返回passedToken，重放已消费返回CONFLICT，不再次授予

**错误**

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：TOKEN_REUSED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PROOF：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 consumed 与匿名 challenge signal
- 授权：permission：anonymous；rule：anonymous 主体必须绑定
- 并发：每 challenge最多5失败尝试；CAS阻止并发重放
- 幂等：重复成功不再次授予，返回 CONFLICT
- 依赖：`traffic-guard.readChallengeSnapshot`、`traffic-guard.checkIssuedChallenge`、`traffic-guard.challengeNonce`、`traffic-guard.VerifyPoW`、`traffic-guard.VerifyExternalChallenge`、`traffic-guard.deriveSessionScopes`、`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`、`traffic-guard.readChallengeRecord`、`traffic-guard.LoadSignals`、`platform.SignGuardSession`

## traffic-guard.readChallengeRecord

- 级别：参考
- 文件：`modules/traffic-guard/internal/challenge.go`
- 作用：按站点与代次读取挑战持久快照

```go
func readChallengeRecord(t *guardTransaction, id string) (challengeRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按站点与代次读取挑战持久快照

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信挑战链路内部调用，继承入口与外层事务
- 并发：短事务或纯函数；外部HTTP不持有事务
- 幂等：继承原挑战状态与哈希
- 依赖：无

## traffic-guard.readChallengeSnapshot

- 级别：参考
- 文件：`modules/traffic-guard/internal/challenge.go`
- 作用：短事务读取后释放，再调用外部供应方

```go
func readChallengeSnapshot(ctx context.Context, op OperationContext, id string) (challengeRecord, GuardRuntime, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 短事务读取后释放，再调用外部供应方

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信挑战链路内部调用，继承入口与外层事务
- 并发：短事务或纯函数；外部HTTP不持有事务
- 幂等：继承原挑战状态与哈希
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`、`traffic-guard.readChallengeRecord`

## traffic-guard.checkIssuedChallenge

- 级别：参考
- 文件：`modules/traffic-guard/internal/challenge.go`
- 作用：核issued、期限和最多5次失败，不授予通过状态

```go
func checkIssuedChallenge(row challengeRecord, now time.Time) error
```

**输入**

无

**输出**

无

**步骤**

1. 核issued、期限和最多5次失败，不授予通过状态

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信挑战链路内部调用，继承入口与外层事务
- 并发：短事务或纯函数；外部HTTP不持有事务
- 幂等：继承原挑战状态与哈希
- 依赖：无

## traffic-guard.challengeNonce

- 级别：参考
- 文件：`modules/traffic-guard/internal/challenge.go`
- 作用：以独立预算HMAC重构nonce并核持久哈希；保留旧key过渡

```go
func challengeNonce(runtime GuardRuntime, site, epoch string, row challengeRecord) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 以独立预算HMAC重构nonce并核持久哈希；保留旧key过渡

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信挑战链路内部调用，继承入口与外层事务
- 并发：短事务或纯函数；外部HTTP不持有事务
- 幂等：继承原挑战状态与哈希
- 依赖：`platform.PrivateOperationDigest`

## traffic-guard.challengeResult

- 级别：参考
- 文件：`modules/traffic-guard/internal/challenge.go`
- 作用：组装原有效挑战和公开widget配置，不输出secret

```go
func challengeResult(runtime GuardRuntime, row challengeRecord, nonce string) IssueChallengeResult
```

**输入**

无

**输出**

无

**步骤**

1. 组装原有效挑战和公开widget配置，不输出secret

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信挑战链路内部调用，继承入口与外层事务
- 并发：短事务或纯函数；外部HTTP不持有事务
- 幂等：继承原挑战状态与哈希
- 依赖：无

## traffic-guard.VerifyPoW

- 级别：参考
- 文件：`modules/traffic-guard/internal/pow.go`
- 作用：验证工作量证明

```go
func VerifyPoW(ctx context.Context, op OperationContext, in VerifyPoWInput) (VerifyPoWResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `VerifyPoWInput` | 验证工作量证明；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifyPoWResult` | 验证工作量证明成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只接受1..64可打印非空白ASCII solution
2. 计算一次SHA256(UTF8(nonce+":"+solution))，要求前difficulty位为0；调用链只传18/22，合同允许16..22
3. 无有效证明返回valid=false，纯函数不检查ctx且不执行无界搜索

**错误**

- `VALIDATION` / HTTP 400：INVALID_PROOF：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：challenge 内部
- 并发：纯函数、常数次数 hash；禁止无界搜索
- 幂等：同 proof 同结果
- 依赖：无

## traffic-guard.VerifyExternalChallenge

- 级别：参考
- 文件：`modules/traffic-guard/internal/captcha.go`
- 作用：适配外部验证

```go
func VerifyExternalChallenge(ctx context.Context, op OperationContext, in VerifyExternalChallengeInput) (VerifyExternalChallengeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `VerifyExternalChallengeInput` | 适配外部验证；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifyExternalChallengeResult` | 适配外部验证成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅可信运行装配提供siteKey、secretRef、expectedHostname；读取外部secret引用，不落日志/业务库
2. 仅POST固定Cloudflare HTTPS siteverify，不传原IP；2.5秒上限、禁止重定向、不自动重试同token
3. responseToken最大2048，响应最大64KiB；未知/不可读/非200返回DEPENDENCY_UNAVAILABLE，不冒充成功
4. success=false明确无效；成功还需hostname、固定traffic_guard action、challengeId对应cdata及challenge_ts五分钟内匹配
5. 浏览器加载失败按既有流程取得新PoW挑战，服务端不伪造外部成功

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CHALLENGE_PROVIDER_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PROOF：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：单次外部认证请求，禁止重试同token
- 授权：permission：anonymous；rule：anonymous challenge 请求
- 并发：凭证仅内存；无真实调用在文档阶段
- 幂等：供应方 token单用途，失败需新挑战
- 依赖：无
- 其他依赖说明：Cloudflare Turnstile服务端verification适配

## traffic-guard.CreateBan

- 级别：约束
- 文件：`modules/traffic-guard/internal/ban_store.go`
- 作用：管理员建立临时禁令

```go
func CreateBan(ctx context.Context, op OperationContext, in CreateBanInput) (CreateBanResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CreateBanInput` | 管理员建立临时禁令；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateBanResult` | 管理员建立临时禁令成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 校验guard.manage与当前站点代次，规范化单个IP地址
2. 按IP和指定时长创建AI禁令，最长30天；无需先有可疑事件
3. 同一事务撤销该IP的在途准入，保存回执并发布无原始IP的事件

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写ban/permit revoked/outbox
- 授权：permission：guard.manage；rule：guard.manage
- 并发：主体范围事务原子封禁与撤租约
- 幂等：operationId 去重
- 依赖：`traffic-guard.authorizeGuardManagement`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.createNetworkBan`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`

## traffic-guard.RevokeBan

- 级别：约束
- 文件：`modules/traffic-guard/internal/ban_store.go`
- 作用：撤销禁令

```go
func RevokeBan(ctx context.Context, op OperationContext, in RevokeBanInput) (RevokeBanResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RevokeBanInput` | 撤销禁令；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RevokeBanResult` | 撤销禁令成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 即时核guard.manage并读取同站点/epoch原回执
2. 严格expectedRevision CAS active→revoked；非active或冲突拒绝，重复同operationId返回原状态
3. ban状态、修订、回执与ban-changed outbox同事务

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：BAN_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写ban/outbox
- 授权：permission：guard.manage；rule：guard.manage
- 并发：revision CAS
- 幂等：相同 operationId复用结果
- 依赖：`traffic-guard.authorizeGuardManagement`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.enqueueGuardEvent`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`

## traffic-guard.ListBans

- 级别：约束
- 文件：`modules/traffic-guard/internal/ban_store.go`
- 作用：分页禁令

```go
func ListBans(ctx context.Context, op OperationContext, in ListBansInput) (ListBansResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ListBansInput` | 分页禁令；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListBansResult` | 分页禁令成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 即时核guard.manage，limit默认20最大100
2. 使用平台签名游标绑定site/epoch/权限主体和banId keyset；稳定banId排序
3. 不返回原IP；同时输出兼容banIds与匿名subjectRef/原因/恢复时间/状态items

**错误**

- `VALIDATION` / HTTP 400：INVALID_CURSOR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：guard.manage；rule：guard.manage
- 并发：稳定banId排序，不含IP
- 幂等：GET幂等
- 依赖：`traffic-guard.authorizeGuardManagement`、`traffic-guard.beginGuardTransaction`、`platform.SignCursor`、`platform.VerifyCursor`、`traffic-guard.guardTransaction.close`

## traffic-guard.createNetworkBan

- 级别：参考
- 文件：`modules/traffic-guard/internal/ban_store.go`
- 作用：同事务创建AI ban、撤匹配网络别名租约并登记安全outbox

```go
func createNetworkBan(t *guardTransaction, network string, ttl int64, reason string) (CreateBanResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同事务创建AI ban、撤匹配网络别名租约并登记安全outbox

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.enqueueGuardEvent`

## traffic-guard.ConsumeIdentityEvent

- 级别：约束
- 文件：`modules/traffic-guard/internal/events.go`
- 作用：处理权限撤销通知

```go
func ConsumeIdentityEvent(ctx context.Context, op OperationContext, in ConsumeIdentityEventInput) (ConsumeIdentityEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ConsumeIdentityEventInput` | 处理权限撤销通知；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeIdentityEventResult` | 处理权限撤销通知成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只供本域后台接受identity.authorization.changed v1；校验当前epoch、生产方payload schema和载荷site/account/permissionRevision与信封一致
2. 平台ClaimInbox与本地最大permissionRevision标记同事务；重复/低修订确认无操作
3. 本域管理写入没有可绕过即时identity权威的缓存，此标记只记录撤销通知，不将事件当授权真相
4. 本地提交后返回accepted，订阅/ack仍由后续公共适配完成；失败不确认
5. 事件认领为事务首项写入，统一ClaimInboxBeforeWrite只提交冲突隔离，不把业务错误回滚成未隔离。

**错误**

- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写inbox授权缓存标记
- 授权：permission：service；rule：service identity事件
- 并发：按 subject权限修订取max
- 幂等：eventId/revision幂等
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`、`platform.ClaimInboxBeforeWrite`

## traffic-guard.ConsumeEvents

- 级别：约束
- 文件：`modules/traffic-guard/internal/events.go`
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

1. 装配inbox.NewEventConsumer真实窄HTTP适配及严格AcknowledgeEventResult生产方schema；返回RunConsumerWithResult真实累计计数，不构造伪worker或空统计。
2. 进程启动后以服务身份调用jobs:subscribeEvents（durable模式SSE），只订阅本域events.json中consume登记的事件名与版本；断线按Last-Event-ID续传，1/2/4秒指数退避（上限60秒）重连
3. 每条事件校验信封、生产方payloadSchema与dataEpoch：不在登记清单的事件隔离报警并确认；dataEpoch不等于当前active epoch时，若本域尚未换入该epoch则不确认等待重投，否则视为旧代事件记skipped并确认
4. 按events.json该事件的handler字段调用本域处理函数；处理函数在同一本地事务内完成inbox(eventId+payloadHash)去重、subject revision单调检查与业务写入
5. 处理函数成功提交（含重复或低revision的无操作）后才调用jobs:acknowledgeEvent；失败不确认，等jobs确认租约到期重投，连续失败由jobs转dead-letter供管理重放
6. 屏障draining时停止领取新事件，等在途处理完成后报告安全点；维护期间不确认未处理事件

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs订阅或确认不可用；重连后续传（可重试）
- `VALIDATION`：事件信封或载荷不符合登记schema；隔离并确认（不可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash；隔离报警（不可重试）

- 副作用：HTTP长连接jobs:subscribeEvents；调用本域登记的事件处理函数；HTTP调用jobs:acknowledgeEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个订阅循环；同subject串行，不同subject最多maxInFlight并发；事务与CAS由处理函数负责
- 幂等：jobs至少一次投递；本域inbox按eventId+payloadHash去重，重复投递只重新确认
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`traffic-guard.ConsumeIdentityEvent`、`platform.NewEventConsumer`、`platform.RunConsumerWithResult`

## traffic-guard.PublishGuardEvents

- 级别：约束
- 文件：`modules/traffic-guard/internal/outbox.go`
- 作用：可靠派发风控引用事件

```go
func PublishGuardEvents(ctx context.Context, op OperationContext, in PublishGuardEventsInput) (PublishGuardEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PublishGuardEventsInput` | 可靠派发风控引用事件；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishGuardEventsResult` | 可靠派发风控引用事件成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅本域后台读取当前恢复过的同代私有库，单批最多100
2. 通过平台NewEventPublisher适配真实gen.JobsClient的publishEvent，不造无关worker方法
3. 复用平台RunRelayBatch处理领取/原eventId发布/安全窗口内确认/退避及旧代隔离，不在本域复制循环机制
4. 每次平台写窗口读取还核打开epoch，未知或切换后不修改旧active库；成功返回实际relayed数量

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：JOBS_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写outbox发送状态
- 授权：permission：service；rule：service
- 并发：outbox租约30s；至少一次
- 幂等：eventId固定不因重试变
- 依赖：`jobs:publishEvent`、`platform.NewEventPublisher`、`platform.RunRelayBatch`

## traffic-guard.PurgeAnonymousSignals

- 级别：参考
- 文件：`modules/traffic-guard/internal/retention.go`
- 作用：清理匿名风控状态

```go
func PurgeAnonymousSignals(ctx context.Context, op OperationContext, in PurgeAnonymousSignalsInput) (PurgeAnonymousSignalsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PurgeAnonymousSignalsInput` | 清理匿名风控状态；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PurgeAnonymousSignalsResult` | 清理匿名风控状态成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 最多清理500个主体记录，租约网络子关联随主体清除
2. 清过期challenge、10分钟问题信号/匿名风险引用、空闲24小时桶、终局7天permit与30天ban
3. active bans/leases禁止删除，不删除授权或预算key，不记录原IP/正文/token

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删除匿名状态
- 授权：permission：service；rule：service
- 并发：500项批事务；active bans/leases禁止删除
- 幂等：重复清理幂等
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`
- 其他依赖说明：私有 SQLite

## traffic-guard.ReportQuestionSignals

- 级别：约束
- 文件：`modules/traffic-guard/internal/question_signals.go`
- 作用：接收qa已有检测信号

```go
func ReportQuestionSignals(ctx context.Context, op OperationContext, in ReportQuestionSignalsInput) (ReportQuestionSignalsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReportQuestionSignalsInput` | 接收qa已有检测信号；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReportQuestionSignalsResult` | 接收qa已有检测信号成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅接受QA主Agent的结构化滥用报告，验证许可的会话与网络作用域
2. 按原轮次幂等去重；一次报告只记一次，不持久化问题正文
3. 单次探测要求挑战；10分钟内至少3轮明确攻击才自动封IP10分钟；真实IP来自portal可信入口
4. 风控独立保留频率、并发、人机验证和人工IP封禁；普通离题不计为滥用

**错误**

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写短期匿名signals或ban
- 授权：permission：service；rule：service qa
- 并发：same questionOperationId去重，不能凭客户端自报风险
- 幂等：questionOperationId/hash幂等
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.readGuardReceipt`、`traffic-guard.saveGuardReceipt`、`traffic-guard.createNetworkBan`、`traffic-guard.LoadSignals`、`traffic-guard.ComputeRisk`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`

## traffic-guard.QueueSessionAdmission

- 级别：约束
- 文件：`modules/traffic-guard/internal/session_queue.go`
- 作用：每会话1并发其余排队

```go
func QueueSessionAdmission(ctx context.Context, op OperationContext, in QueueSessionAdmissionInput) (QueueSessionAdmissionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `QueueSessionAdmissionInput` | 每会话1并发其余排队；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `QueueSessionAdmissionResult` | 每会话1并发其余排队成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只允许QA；sessionKey必须已经验证，guard只存request引用，不存正文、不扣桶/费用
2. mode可选enqueue默认；同session FIFO最多10等待，同request不重复插入
3. cancel仅删同站点/同代次/同session/同request的等待引用，绝不释放active lease
4. 空闲且为队头才queued=false；真正并发占位仍由准入事务完成，释放后QA按FIFO重新admission

**错误**

- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅短期匿名队列引用
- 授权：permission：service；rule：service qa
- 并发：每session最多10等待，超出429；无问题正文
- 幂等：requestId幂等
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`
- 其他依赖说明：qa排队权威

## traffic-guard.ConsumeAdmission

- 级别：约束
- 文件：`modules/traffic-guard/internal/admission_consumption.go`
- 作用：QA绑定准入使用权

```go
func ConsumeAdmission(ctx context.Context, op OperationContext, in ConsumeAdmissionInput) (ConsumeAdmissionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ConsumeAdmissionInput` | QA绑定准入使用权；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeAdmissionResult` | QA绑定准入使用权成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅已认证QA；API无token，不假装验证不存在的proof
2. 原requestId必须等于可信op.RootOperationID，并匹配持久owner=qa/site/epoch/root/active lease/当前网络ban
3. 同事务CAS标记consumed_request_id=原root；同root重放不再消费，不同root冲突
4. 只在消费状态可提交后调用SignPermit签完整QA DTO，提交成功才返回；崩溃重试只重签原获准期限，不扣桶/费用
5. qa.conversation持久消费后仅返回空token元数据，绝不调用SignPermit；只有单次问题qa.ask/followup可签模型凭证

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：PERMIT_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写admission consumer绑定
- 授权：permission：service；rule：service qa
- 并发：admissionId唯一request绑定，release后禁止再consume
- 幂等：admissionId/requestId幂等
- 依赖：`traffic-guard.beginGuardTransaction`、`traffic-guard.readPermitRecord`、`traffic-guard.CheckBan`、`traffic-guard.SignPermit`、`traffic-guard.guardTransaction.close`、`traffic-guard.guardTransaction.commit`、`traffic-guard.permitMetadata`
- 其他依赖说明：私有lease状态

## traffic-guard.VerifyAdmissionHTTP

- 级别：约束
- 文件：`modules/traffic-guard/internal/http_verify.go`
- 作用：QA在线确认准入租约

```go
func VerifyAdmissionHTTP(ctx context.Context, op OperationContext, in VerifyAdmissionHTTPInput) (VerifyAdmissionHTTPResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `VerifyAdmissionHTTPInput` | QA在线确认准入租约；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifyAdmissionHTTPResult` | QA在线确认准入租约成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅已认证QA；body requestId必须等于可信原RootOperationID
2. 有可选token时调用ValidatePermit验签并逐字段/持久scope核对；否则只核持久owner/root/epoch/active lease及当前网络AI ban
3. 只返回当前valid/expiry，不代表创建消费或模型费用授权，MG不反向调用此API
4. 无token路径同样要求consumed_request_id等于原QA根，不能把未消费分配查询当已消费核验；所有成功响应包含admissionId供QA精确核对

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：STALE_RESULT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service qa
- 并发：只读当前lease；不能把检查缓存为永久授权
- 幂等：GET语义重复安全
- 依赖：`traffic-guard.ValidatePermit`、`traffic-guard.beginGuardTransaction`、`traffic-guard.readPermitRecord`、`traffic-guard.CheckBan`、`traffic-guard.guardTransaction.close`
- 其他依赖说明：私有SQLite

## traffic-guard.All

- 级别：参考
- 文件：`modules/traffic-guard/migrations/migrations.go`
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

## traffic-guard.main

- 级别：约束
- 文件：`modules/traffic-guard/cmd/traffic-guard/main.go`
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
- 依赖：`traffic-guard.BuildApplication`、`traffic-guard.Serve`、`traffic-guard.Shutdown`、`platform.RunService`

## traffic-guard.BuildApplication

- 级别：约束
- 文件：`modules/traffic-guard/internal/app/wire.go`
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
7. 私有库固定view.DataEpoch；恢复store后实际构造NewLoginHandlers并交公共NewServiceHandler核完整内嵌spec，两个已完成operation只允许identity且普通维护不放行；任何其他handler缺失由工厂返回DEPENDENCY_UNAVAILABLE并关闭store，完整其余后台/恢复回执仍缺时也禁止返回Application
8. 管理/匿名信号真实HTTP映射通过NewGuardHandlers交公共ServiceHandler；授权由统一边界即时核一次，内部marker避免重复identity调用。运行写窗口/预算key/完整其余handler及后台尚缺时仍禁止启动。
9. 用生成的IdentityChanged运行载荷规格创建平台validator并注入GuardRuntime；jobs真实客户端用于公共relay。完整预算key/安全写窗口/恢复/订阅后台尚缺仍禁止启动
10. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`traffic-guard.ExpireLeases`、`traffic-guard.ExpireBans`、`traffic-guard.PurgeAnonymousSignals`、`traffic-guard.PublishGuardEvents`、`traffic-guard.ConsumeEvents`、`platform.ReadRuntime`、`traffic-guard.All`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`traffic-guard.OpenPolicyStore`、`traffic-guard.RunMigrations`、`traffic-guard.RecoverJournal`、`traffic-guard.Close`、`traffic-guard.NewLoginHandlers`、`platform.NewServiceHandler`、`traffic-guard.NewGuardHandlers`、`platform.NewPayloadValidator`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## traffic-guard.Serve

- 级别：约束
- 文件：`modules/traffic-guard/internal/transport/server.go`
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
- 依赖：`traffic-guard.BuildApplication`、`traffic-guard.Shutdown`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## traffic-guard.Shutdown

- 级别：约束
- 文件：`modules/traffic-guard/internal/transport/server.go`
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

## traffic-guard.Health

- 级别：约束
- 文件：`modules/traffic-guard/internal/transport/server.go`
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

## traffic-guard.Ready

- 级别：约束
- 文件：`modules/traffic-guard/internal/transport/server.go`
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

## traffic-guard.ListRiskEvents

- 级别：约束
- 文件：`modules/traffic-guard/internal/queries.go`
- 作用：查看最近可疑事件

```go
func ListRiskEvents(ctx context.Context, op OperationContext, input ListRiskEventsInput) (ListRiskEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ListRiskEventsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListRiskEventsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. guard.manage即时查权；since省略查询全部尚保留匿名记录
2. 按occurredAt/eventId倒序keyset分页，平台签名游标绑定站点/epoch/主体/since
3. 只返回事件引用/动作/封闭决策/不可逆网络key/时间，无原IP

**错误**

- `FORBIDDEN`：权限不足（不可重试）

- 副作用：无
- 授权：permission：guard.manage；rule：后台管理员按登记权限
- 并发：只读快照或CAS
- 幂等：读取幂等；写入按operationId
- 依赖：`traffic-guard.authorizeGuardManagement`、`traffic-guard.beginGuardTransaction`、`platform.SignCursor`、`platform.VerifyCursor`、`traffic-guard.guardTransaction.close`

## traffic-guard.WithGuardRuntime

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：可信装配注入私有库、即时权限、独立预算key与当前写窗口

```go
func WithGuardRuntime(ctx context.Context, runtime GuardRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 可信装配注入私有库、即时权限、独立预算key与当前写窗口

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.WithPolicyStore`

## traffic-guard.beginGuardTransaction

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：锁前取得当前安全写窗口；嵌套组合准入复用同一本地短事务

```go
func beginGuardTransaction(ctx context.Context, op OperationContext, write bool) (*guardTransaction, error)
```

**输入**

无

**输出**

无

**步骤**

1. 锁前取得当前安全写窗口；嵌套组合准入复用同一本地短事务

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`platform.NewWriteContext`

## traffic-guard.guardTransaction.close

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：只有事务拥有者回滚并释放库生命周期锁与取消句柄

```go
func (t *guardTransaction) close()
```

**输入**

无

**输出**

无

**步骤**

1. 只有事务拥有者回滚并释放库生命周期锁与取消句柄

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.guardTransaction.commit

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：只有最外层提交，嵌套桶/并发不单独提交

```go
func (t *guardTransaction) commit() error
```

**输入**

无

**输出**

无

**步骤**

1. 只有最外层提交，嵌套桶/并发不单独提交

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.authorizeGuardManagement

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：管理员权限每次向identity即时核验，不信任菜单或事件投影

```go
func authorizeGuardManagement(ctx context.Context, op OperationContext, action string) error
```

**输入**

无

**输出**

无

**步骤**

1. 管理员权限每次向identity即时核验，不信任菜单或事件投影

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`identity:checkAuthorization`、`platform.NewIdentityAuthorizer`

## traffic-guard.readGuardReceipt

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：同事务核规范输入摘要并读取不含token的原安全回执

```go
func readGuardReceipt(t *guardTransaction, kind string, input any, result any) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同事务核规范输入摘要并读取不含token的原安全回执

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.saveGuardReceipt

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：保存安全回执，与业务和outbox同事务；签名token不得传入持久回执

```go
func saveGuardReceipt(t *guardTransaction, kind string, input any, result any) error
```

**输入**

无

**输出**

无

**步骤**

1. 保存安全回执，与业务和outbox同事务；签名token不得传入持久回执

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.enqueueGuardEvent

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：使用公共outbox同事务登记安全事件引用

```go
func enqueueGuardEvent(t *guardTransaction, eventType, subject string, revision int64, payload any) error
```

**输入**

无

**输出**

无

**步骤**

1. 使用公共outbox同事务登记安全事件引用

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`platform.EnqueueEvent`

## traffic-guard.readGuardPolicy

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：读取已保存同代次策略，缺行NOT_FOUND，不猜默认

```go
func readGuardPolicy(t *guardTransaction) (GuardPolicy, int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取已保存同代次策略，缺行NOT_FOUND，不猜默认

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：无

## traffic-guard.registerGuardWorkTables

- 级别：参考
- 文件：`modules/traffic-guard/internal/guard_runtime.go`
- 作用：在本域迁移完成后登记全部真实风控业务表的公共写门

```go
func registerGuardWorkTables(ctx context.Context, db *sql.DB) error
```

**输入**

无

**输出**

无

**步骤**

1. 按固定guardWorkTables逐表调用platform.RegisterWorkTable；不登记schema_migrations或nw_outbox

**错误**

- 无

- 副作用：在本域业务表安装平台统一revision/sealed gate
- 授权：permission：service；rule：仅RunMigrations可信启动调用
- 并发：逐表短事务，由平台登记操作保证幂等
- 幂等：重复登记同一表不重复创建触发器
- 依赖：`platform.RegisterWorkTable`
- 其他依赖说明：traffic-guard.db

## traffic-guard.NewGuardHandlers

- 级别：参考
- 文件：`modules/traffic-guard/internal/http_management.go`
- 作用：只供统一服务边界绑定已完成管理/匿名信号API；内部marker复用该次即时授权

```go
func NewGuardHandlers(runtime GuardRuntime) map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 只供统一服务边界绑定已完成管理/匿名信号API；内部marker复用该次即时授权

**错误**

- `VALIDATION`：数据根非绝对（不可重试）
- `DEPENDENCY_UNAVAILABLE`：active代次尚未建立（可重试）
- `INTERNAL`：目录/资源建立失败（不可重试）
- `INTEGRITY_ERROR`：业务库不可用（不可重试）

- 副作用：建立私有目录，打开SQLite与control journal，复用platform outbox schema
- 授权：permission：service；rule：仅可信本域装配或已认证绑定链路内部调用
- 并发：本域短事务；不执行口令核验
- 幂等：以原绑定函数幂等约束为准
- 依赖：`traffic-guard.NewLoginHandlers`、`traffic-guard.GetPolicy`、`traffic-guard.UpdatePolicy`、`traffic-guard.CreateBan`、`traffic-guard.RevokeBan`、`traffic-guard.ListBans`、`traffic-guard.ListRiskEvents`、`traffic-guard.ReportQuestionSignals`、`traffic-guard.QueueSessionAdmission`、`traffic-guard.WithGuardRuntime`、`traffic-guard.HandleAdmission`、`traffic-guard.RenewPermit`、`traffic-guard.ReleasePermit`、`traffic-guard.ConsumeAdmission`、`traffic-guard.VerifyAdmissionHTTP`、`traffic-guard.admissionRateError.Error`、`traffic-guard.admissionRateError.Unwrap`、`traffic-guard.IssueChallenge`、`traffic-guard.VerifyChallenge`、`traffic-guard.DeriveSubject`

## traffic-guard.GuardPage

- 级别：约束
- 文件：`modules/traffic-guard/web/src/index.tsx`
- 作用：防滥用：访客提问准入策略、禁令、可疑事件与一键封禁

```ts
export default function GuardPage(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 读取策略（未保存过时给默认值）、禁令与可疑事件
2. 建立禁令只接受不可逆网络引用

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无
