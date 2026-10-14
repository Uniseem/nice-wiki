# traffic-guard 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 53 个函数：约束 31 个，参考 22 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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

1. 拒绝钱账字段/未知动作
2. 限制 burst≤rate、concurrency 1..128
3. 挑战难度 16..22 与 ban TTL 范围

**错误**

- `VALIDATION` / HTTP 400：INVALID_POLICY：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：guard.manage；rule：guard.manage 入口内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：纯函数
- 幂等：同 JSON 规范化结果一致
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

1. 校验 epoch
2. 读取当前 revision
3. 返回无 secret 策略

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：guard.manage；rule：guard.manage 或 service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：SQLite 快照读
- 幂等：GET 幂等
- 依赖：`site-config:readRuntimeState`

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

1. 身份查权
2. 校验策略
3. 事务比较 revision/写策略/outbox
4. 冲突不覆盖

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：POLICY_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写策略/outbox
- 授权：permission：guard.manage；rule：guard.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：事务 CAS；旧 permit renew 用最新策略重检
- 幂等：operationId 去重且相同 key 不同 body 409
- 依赖：`identity:checkAuthorization`、`traffic-guard.ValidatePolicy`

## traffic-guard.NormalizePeer

- 级别：参考
- 文件：`modules/traffic-guard/internal/subject.go`
- 作用：只在内存选择可信来源

```go
func NormalizePeer(ctx context.Context, op OperationContext, in NormalizePeerInput) (NormalizePeerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `NormalizePeerInput` | 只在内存选择可信来源；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizePeerResult` | 只在内存选择可信来源成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 实际peer必须为配置可信代理才接受X-Forwarded-For/CF-Connecting-IP
2. 从可信链选真实peer
3. IPv4单址IPv6归一化/64用于ban
4. 原始IP仅内存，禁止记录

**错误**

- `VALIDATION` / HTTP 400：INVALID_PEER：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：不写日志/数据库
- 授权：permission：service；rule：service ingress；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：每请求内存，返回值禁止进入 tracing attributes
- 幂等：纯解析同输入同输出
- 依赖：无
- 其他依赖说明：site-config 服务安全配置

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

1. HMAC各独立domain pepper：规范IP IPv4单址IPv6/64得networkKey、有效签名visitorSession得sessionKey
2. subjectKey组合
3. 原始IP/token不落盘
4. ban按networkKey不按可换visitorToken
5. 当期与前期别名保留过渡

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：SUBJECT_KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service ingress；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：pepper 外部密钥容器，30 日轮换同时检查前一 epoch 未过期 ban
- 幂等：同 keyEpoch 同输入同主体
- 依赖：无
- 其他依赖说明：identity visitor token verifier

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

1. 匹配服务白名单 qa.ask/search/export/admin
2. 拒绝客户端覆盖
3. 公共读取仅受边缘基本速率

**错误**

- `INTERNAL` / HTTP 500：UNKNOWN_ACTION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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

1. 按主体/action 查询 10 分钟 counters
2. 验证 challenge expiry
3. 读取 active ban

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：统一事务读取一致 policy revision
- 幂等：只读
- 依赖：无
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

1. program信号=问题hash重复/qa已有embedding相似度0..10000/最近时间间隔规律
2. 前置模型同一次输出normal/suspicious/abuse，不单独新增模型
3. activeBan或abuse拒绝
4. suspicious要求可见Turnstile
5. normal仍需首次无感验证
6. 前置模型已判无关直接拒答

**错误**

- `VALIDATION` / HTTP 400：INVALID_SIGNALS：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service 内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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

1. elapsed clamp 0..60000
2. 补充 floor(elapsedMs*rate/60) 千分令牌
3. 容量封顶

**错误**

- `VALIDATION` / HTTP 400：INVALID_BUCKET：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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

1. 读取主体+站点 buckets
2. 整数 refill
3. 两层都足够才扣，否则均不扣并计算最大等待

**错误**

- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写匿名 token bucket
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：SQLite 单事务；不能分别扣一层失败另一层
- 幂等：requestId 去重不重复扣
- 依赖：`traffic-guard.RefillBucket`、`traffic-guard.GetPolicy`

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

1. 同事务清过期 lease
2. 统计 subject/site active
3. 限额内插入30s lease及300s hard deadline

**错误**

- `RATE_LIMITED` / HTTP 429：CONCURRENCY_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 permit 并发占位
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：与 bucket 消耗同最终 admission 事务；失败回滚扣令牌
- 幂等：requestId 唯一返回原 permit
- 依赖：无
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

1. 验证 actor绑定与 epoch
2. 重查 ban/最新 policy
3. CAS expiresAt=min(now+30s,hardDeadline)，到期必须重新 admission

**错误**

- `STALE_RESULT` / HTTP 409：PERMIT_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：ACTIVE_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 expiresAt
- 授权：permission：service；rule：service 原 audience；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：admission revision CAS，与 release 互斥
- 幂等：同续租 operationId 返回同 expiry
- 依赖：`traffic-guard.CheckBan`、`traffic-guard.SignPermit`

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

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 lease state
- 授权：permission：service；rule：service 原 audience；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：release/renew 相同事务序列
- 幂等：重复 released 返回 true
- 依赖：无
- 其他依赖说明：私有 SQLite

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

1. 绑定服务 audience/动作/epoch/lease ID
2. Ed25519 短期签名
3. 只返回 token 不记日志

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：SIGNING_KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service 内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只读密钥容器与 admission 快照
- 幂等：同 admission 可重复生成等价 token
- 依赖：无
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

1. 验签/audience/epoch/expiry
2. 查询 active lease及最新 ban
3. 匹配 request operation

**错误**

- `VALIDATION` / HTTP 400：PERMIT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：PERMIT_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：ACTIVE_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service 身份绑定；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：服务在线查询不能只信过期的本地签名状态
- 幂等：只读重复验证
- 依赖：`traffic-guard.GetPolicy`、`traffic-guard.CheckBan`

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

1. 可信runtime epoch/maintenance
2. 首次人机验证session
3. 按networkKey查10min AI禁令
4. 检查IP+session频率/重复/规律信号
5. model风险由qa已有前置结果回报
6. 同事务bucket+session1并发lease
7. 新问题在qa排队不抢掉旧问题
8. 签permit只准入不预占费用

**错误**

- `FORBIDDEN` / HTTP 403：ACTIVE_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `RATE_LIMITED` / HTTP 429：CHALLENGE_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `RATE_LIMITED` / HTTP 429：CONCURRENCY_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 token bucket/permit/outbox denial 引用
- 授权：permission：service；rule：service 白名单；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：每站点事务；没有费用预占
- 幂等：requestId+inputHash 幂等，不同 body 冲突
- 依赖：`site-config:readRuntimeState`、`traffic-guard.ComputeRisk`、`traffic-guard.ConsumeBuckets`、`traffic-guard.AcquireConcurrency`、`traffic-guard.SignPermit`

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

1. 服务鉴权
2. 内存 peer 规范化与匿名 hash
3. EvaluateAdmission
4. 输出 permit 或公共 Problem+retryAfter

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：同 EvaluateAdmission，不泄漏 subjectKey
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：请求deadline 1500ms，失败不颁 permit
- 幂等：requestId 幂等
- 依赖：`traffic-guard.EvaluateAdmission`、`traffic-guard.DeriveSubject`

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

1. 每5秒扫描最多500 active expire
2. CAS expired
3. 只更新状态不重发外部调用

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 expired state
- 授权：permission：service；rule：service worker；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：批次事务；分布多实例通过 SQLite 单写协调
- 幂等：重复 sweep 无新增效果
- 依赖：无
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

1. 扫描≤500到期active
2. CAS expired
3. 写ban-changed outbox

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写状态/outbox
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：小事务，无同时策略修订
- 幂等：相同时间重扫幂等
- 依赖：无
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

1. 第一次提问优先Turnstile invis
2. 可疑要求interactive，之后默认每5问重验
3. 脚本加载5秒失败用PoW18bits正常/≤22可疑
4. 每主体6次/min签nonce120s，已通过proof签问答功能session

**错误**

- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：CHALLENGE_DISABLED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 challenge 哈希
- 授权：permission：anonymous；rule：anonymous，经 ingress 派生主体；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：同主体挑战最多2个 active
- 幂等：requestId 幂等，同 key method不同409
- 依赖：`traffic-guard.ComputeRisk`
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

1. 查询 issued/同主体/epoch/120s
2. 验proof
3. 事务CAS issued→consumed并发10min签名通过证明

**错误**

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：TOKEN_REUSED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PROOF：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 consumed 与匿名 challenge signal
- 授权：permission：anonymous；rule：anonymous 主体必须绑定；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：每 challenge最多5失败尝试；CAS阻止并发重放
- 幂等：重复成功不再次授予，返回 CONFLICT
- 依赖：`traffic-guard.VerifyPoW`、`traffic-guard.VerifyExternalChallenge`

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

1. solution 最大64 ASCII
2. SHA256(nonce||colon||solution)
3. 要求前 difficulty bit全0，16..22

**错误**

- `VALIDATION` / HTTP 400：INVALID_PROOF：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：challenge 内部；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
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

1. 仅服务端配置Cloudflare Turnstile验证接口2.5s
2. 校验token success/hostname/action/过期
3. 响应token不落盘
4. 组件加载失败走PoW备用而不是额外验证方案选择

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CHALLENGE_PROVIDER_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_PROOF：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：单次外部认证请求，禁止重试同token
- 授权：permission：anonymous；rule：anonymous challenge 请求；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：凭证仅内存；无真实调用在文档阶段
- 幂等：供应方 token单用途，失败需新挑战
- 依赖：无
- 其他依赖说明：Cloudflare Turnstile服务端verification适配

## traffic-guard.CheckBan

- 级别：参考
- 文件：`modules/traffic-guard/internal/ban.go`
- 作用：读取作用域禁令

```go
func CheckBan(ctx context.Context, op OperationContext, in CheckBanInput) (CheckBanResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CheckBanInput` | 读取作用域禁令；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CheckBanResult` | 读取作用域禁令成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 检查当期/前期主体 aliases
2. site-wide与action匹配
3. 过期禁令忽略

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：准入/续租每次查询，无跨请求安全 TTL
- 幂等：只读幂等
- 依赖：无
- 其他依赖说明：私有 SQLite

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

1. check guard.manage
2. 匿名networkKey指IPv4单址/IPv6 /64
3. default600s ban只限制AI
4. 同事务ban+撤租约+outbox
5. 返回原因/恢复时间，公开页面继续可读

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_BAN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写ban/permit revoked/outbox
- 授权：permission：guard.manage；rule：guard.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：主体范围事务原子封禁与撤租约
- 幂等：operationId 去重
- 依赖：`identity:checkAuthorization`

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

1. 校验权限与revision
2. CAS active→revoked
3. 写审计outbox

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：BAN_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写ban/outbox
- 授权：permission：guard.manage；rule：guard.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：revision CAS
- 幂等：相同 operationId复用结果
- 依赖：`identity:checkAuthorization`

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

1. limit默认20最大100
2. 签名游标绑定epoch/filter
3. 只返回subjectRef和理由枚举

**错误**

- `VALIDATION` / HTTP 400：INVALID_CURSOR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：guard.manage；rule：guard.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：稳定banId排序，不含IP
- 幂等：GET幂等
- 依赖：`identity:checkAuthorization`

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

1. 验生产者/版本/epoch
2. inbox去重
3. 使管理授权上下文失效，不把通知当授权真相

**错误**

- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写inbox授权缓存标记
- 授权：permission：service；rule：service identity事件；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：按 subject权限修订取max
- 幂等：eventId/revision幂等
- 依赖：无

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
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`traffic-guard.ConsumeIdentityEvent`

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

1. 选≤100 pending
2. 向jobs提交带版本envelope
3. 成功mark delivered，失败5/30/120/600秒重试

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：JOBS_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写outbox发送状态
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：outbox租约30s；至少一次
- 幂等：eventId固定不因重试变
- 依赖：`jobs:publishEvent`

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

1. 清过期challenge+10min signals
2. bucket空闲24h删除
3. expired permits7d/ban30d删除

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删除匿名状态
- 授权：permission：service；rule：service；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：500项批事务；active bans/leases禁止删除
- 幂等：重复清理幂等
- 依赖：无
- 其他依赖说明：私有 SQLite

## traffic-guard.EnterBarrier

- 级别：约束
- 文件：`modules/traffic-guard/internal/barrier.go`
- 作用：停止准入并等安全点

```go
func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `BarrierRequest` | 停止准入并等安全点；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 停止准入并等安全点成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验证超管委托与epoch
2. 标 draining拒新准入
3. 等待active lease释放/300s硬到期或deadline
4. 不偷偷清活跃租约

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：持久维护barrier状态
- 授权：permission：service；rule：service+superadmin 委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：barrier独占；renew只能至barrier deadline
- 幂等：barrierId重复返回同状态
- 依赖：`site-config:readRuntimeState`、`jobs:coordinateBarrier`

## traffic-guard.ReleaseBarrier

- 级别：约束
- 文件：`modules/traffic-guard/internal/barrier.go`
- 作用：解除风控维护

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReleaseBarrierInput` | 解除风控维护；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 解除风控维护成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 核验运行epoch一致和恢复全域已完成
2. CAS safe→released

**错误**

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写barrier状态
- 授权：permission：service；rule：service+superadmin 委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：不提前恢复准入
- 幂等：重复release幂等
- 依赖：`site-config:readRuntimeState`

## traffic-guard.CreateSnapshot

- 级别：约束
- 文件：`modules/traffic-guard/internal/snapshot.go`
- 作用：导出风控一致快照

```go
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotExportRequest` | 导出风控一致快照；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 导出风控一致快照成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 要求本域safe
2. SQLite checkpoint
3. 包含policy/bans及匿名hash但不含token/pepper/live租约
4. 加密备份含匿名hash，git仅policy

**错误**

- `LOCKED` / HTTP 409：BARRIER_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写快照对象
- 授权：permission：service；rule：service+superadmin 委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：维护屏障下固定水位
- 幂等：backupId/barrierId幂等
- 依赖：`jobs:coordinateBarrier`

## traffic-guard.ValidateImport

- 级别：约束
- 文件：`modules/traffic-guard/internal/import.go`
- 作用：校验恢复输入

```go
func ValidateImport(ctx context.Context, op OperationContext, request SnapshotValidateRequest) (SnapshotValidation, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotValidateRequest` | 校验恢复输入；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotValidation` | 校验恢复输入成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验证hash/schema/无secret
2. 策略全量校验
3. 租约/challenge必须为空，ban期限RFC3339

**错误**

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：隔离校验报告
- 授权：permission：service；rule：service+superadmin 委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：隔离目录，禁止改现有准入
- 幂等：snapshot hash/targetEpoch同结果
- 依赖：`traffic-guard.ValidatePolicy`

## traffic-guard.PrepareImport

- 级别：约束
- 文件：`modules/traffic-guard/internal/import.go`
- 作用：准备本域导入

```go
func PrepareImport(ctx context.Context, op OperationContext, request SnapshotPrepareRequest) (SnapshotStageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotPrepareRequest` | 准备本域导入；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotStageResult` | 准备本域导入成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 校验快照
2. 建立隔离DB
3. 清除租约/challenge/counters并重新绑定targetEpoch
4. 有效ban保留

**错误**

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写隔离DB/stage
- 授权：permission：service；rule：service+superadmin 委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：本域safe且restoreId独占
- 幂等：restoreId/snapshotHash去重
- 依赖：`traffic-guard.ValidateImport`

## traffic-guard.ActivateImport

- 级别：约束
- 文件：`modules/traffic-guard/internal/import.go`
- 作用：激活新代次

```go
func ActivateImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotCommitRequest` | 激活新代次；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotApplyResult` | 激活新代次成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 检查data-protection commit决策与stage
2. 原子切DB指针
3. 所有旧permit即失效

**错误**

- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：PREPARE_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：切存储指针，禁用旧permit
- 授权：permission：service；rule：service+superadmin 委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：全站继续维护到全部域完成
- 幂等：stageId/targetEpoch幂等
- 依赖：`data-protection:getRestoreDecision`

## traffic-guard.AbortImport

- 级别：约束
- 文件：`modules/traffic-guard/internal/import.go`
- 作用：撤销未提交stage

```go
func AbortImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotCommitRequest` | 撤销未提交stage；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotApplyResult` | 撤销未提交stage成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 检查恢复决策未commit
2. 删除隔离DB
3. 已commit拒绝rollback要求续激活

**错误**

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删除隔离stage
- 授权：permission：service；rule：service+superadmin 委托；recoveryAuthorization：barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可
- 并发：stage锁
- 幂等：重复rolled-back幂等
- 依赖：`data-protection:getRestoreDecision`

## traffic-guard.ValidateRecoveryAuthorization

- 级别：参考
- 文件：`modules/traffic-guard/internal/recovery_authorization.go`
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
- 依赖：`data-protection:getRestoreDecision`
- 其他依赖说明：不可变控制信任锚

## traffic-guard.SaveGuardPolicy

- 级别：参考
- 文件：`modules/traffic-guard/web/src/PolicyEditor.vue`
- 作用：Vue3模块交互：SaveGuardPolicy

```ts
function SaveGuardPolicy(input: SaveGuardPolicyInput, signal: AbortSignal): Promise<SaveGuardPolicyResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `SaveGuardPolicyInput` | Vue3模块交互：SaveGuardPolicy；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SaveGuardPolicyResult` | Vue3模块交互：SaveGuardPolicy成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 提交策略CAS
2. 409保留表单并展示服务器revision，界面不显示费用额度
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：guard.manage；rule：guard.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：traffic-guard OpenAPI HTTP/JSON/SSE

## traffic-guard.SubmitBan

- 级别：参考
- 文件：`modules/traffic-guard/web/src/BanList.vue`
- 作用：Vue3模块交互：SubmitBan

```ts
function SubmitBan(input: SubmitBanInput, signal: AbortSignal): Promise<SubmitBanResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `SubmitBanInput` | Vue3模块交互：SubmitBan；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SubmitBanResult` | Vue3模块交互：SubmitBan成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 提交匿名主体引用临时封禁
2. 只展示reasonCode/expiry，不输入原始IP
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：guard.manage；rule：guard.manage；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：traffic-guard OpenAPI HTTP/JSON/SSE

## traffic-guard.RunPoWChallenge

- 级别：参考
- 文件：`modules/traffic-guard/web/src/client.ts`
- 作用：Vue3模块交互：RunPoWChallenge

```ts
function RunPoWChallenge(input: RunPoWChallengeInput, signal: AbortSignal): Promise<RunPoWChallengeResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `RunPoWChallengeInput` | Vue3模块交互：RunPoWChallenge；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RunPoWChallengeResult` | Vue3模块交互：RunPoWChallenge成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. Web Worker每4096次hash让出事件循环
2. 截止expiresAt或用户取消立即停止
3. 验证结果仍需服务端CAS
4. 处理401重新登录、403清空敏感界面
5. 按operationId重试不重复提交

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：anonymous；rule：anonymous；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：traffic-guard OpenAPI HTTP/JSON/SSE

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

1. qa服务鉴权
2. 只允许问题hash/量化相似度/时间间隔/已有前置模型normal/suspicious/abuse
3. 不接正文
4. abuse建networkKey600秒AIban
5. suspicious要求可见验证
6. normal保留最多最近8时间戳和hash10min

**错误**

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写短期匿名signals或ban
- 授权：permission：service；rule：service qa；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：same questionOperationId去重，不能凭客户端自报风险
- 幂等：questionOperationId/hash幂等
- 依赖：`traffic-guard.ComputeRisk`、`traffic-guard.CreateBan`

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

1. qa持有请求队列内容
2. guard只返回activeConcurrency=1和队列许可引用
3. release后qa按FIFO重新admission
4. 排队时不预扣频率/费用

**错误**

- `RATE_LIMITED` / HTTP 429：RATE_LIMITED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅短期匿名队列引用
- 授权：permission：service；rule：service qa；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：每session最多10等待，超出429；无问题正文
- 幂等：requestId幂等
- 依赖：`traffic-guard.ReleasePermit`
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

1. 验签permit audience/action/epoch和activelease
2. 事务CAS minted→consumedByRequest
3. 同request重复成功，不同request重用拒409
4. 不重扣bucket也不扣钱

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：PERMIT_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写admission consumer绑定
- 授权：permission：service；rule：service qa；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：admissionId唯一request绑定，release后禁止再consume
- 幂等：admissionId/requestId幂等
- 依赖：`traffic-guard.SignPermit`
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

1. 服务qa身份
2. 核admission active/request/epoch和当前networkAIban
3. 只返回valid/expiry
4. model-gateway不调用此API而本地验permit签名

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：STALE_RESULT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service qa；recoveryAuthorization：不适用；本函数不得扩大已鉴权调用上下文
- 并发：只读当前lease；不能把检查缓存为永久授权
- 幂等：GET语义重复安全
- 依赖：`traffic-guard.CheckBan`
- 其他依赖说明：私有SQLite

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
- 依赖：`traffic-guard.BuildApplication`、`traffic-guard.Serve`、`traffic-guard.Shutdown`

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
3. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：`traffic-guard.BuildApplication`、`traffic-guard.Shutdown`
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

## traffic-guard.mountModuleUI

- 级别：约束
- 文件：`modules/traffic-guard/web/src/main.ts`
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
