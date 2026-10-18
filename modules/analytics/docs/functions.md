# analytics 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 147 个函数：约束 33 个，参考 114 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## analytics.ValidateBeacon

- 级别：参考
- 文件：`modules/analytics/internal/beacon_validation.go`
- 作用：严格匿名输入校验

```go
func ValidateBeacon(ctx context.Context, op OperationContext, in ValidateBeaconInput) (ValidateBeaconResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateBeaconInput` | 严格匿名输入校验；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateBeaconResult` | 严格匿名输入校验成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 原2048字节由公共入口一次控制，不把重编码大小冒充原大小；结构数值范围由严格schema校验。
2. 事件仅page-view/navigation-click；来源仅合法ASCII域名，不含路径、端口、查询、IP或凭据。
3. 已知crawler由服务端内存过滤，无统计cookie。

**错误**

- `VALIDATION` / HTTP 400：INVALID_BEACON：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：纯函数；拒未知message/query/referrer/IP字段
- 幂等：same beacon同结果
- 依赖：无

## analytics.BucketLatency

- 级别：参考
- 文件：`modules/analytics/internal/buckets.go`
- 作用：固定时延桶

```go
func BucketLatency(ctx context.Context, op OperationContext, in BucketLatencyInput) (BucketLatencyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BucketLatencyInput` | 固定时延桶；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BucketLatencyResult` | 固定时延桶成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 0..99/100..499/500..999/1000..2999/3000+
2. 超60000拒绝

**错误**

- `VALIDATION` / HTTP 400：INVALID_LATENCY：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数
- 幂等：同ms同bucket
- 依赖：无

## analytics.RedactAuditMetadata

- 级别：参考
- 文件：`modules/analytics/internal/privacy.go`
- 作用：输出允许元数据

```go
func RedactAuditMetadata(ctx context.Context, op OperationContext, in RedactAuditMetadataInput) (RedactAuditMetadataResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RedactAuditMetadataInput` | 输出允许元数据；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RedactAuditMetadataResult` | 输出允许元数据成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 动作必须固定registry登记；metadata≤2048字节、严格UTF8 JSON对象。
2. 仅revision/count非负int64和登记状态fromState/toState，拒重复/未知/null/浮点/尾随内容，不取原文摘要。
3. canonical UTF8安全编码；未知或错误返回零SafeMetadata，不输出输入内容。

**错误**

- `FORBIDDEN` / HTTP 403：AUDIT_FIELD_FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service内部
- 并发：纯函数，≤4096bytes
- 幂等：同metadata白名单同结果
- 依赖：`analytics.auditStateAllowed`
- 其他依赖说明：审计动作登记表

## analytics.CheckPrivacyFields

- 级别：参考
- 文件：`modules/analytics/internal/privacy.go`
- 作用：验证日志与导出字段

```go
func CheckPrivacyFields(ctx context.Context, op OperationContext, in CheckPrivacyFieldsInput) (CheckPrivacyFieldsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CheckPrivacyFieldsInput` | 验证日志与导出字段；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CheckPrivacyFieldsResult` | 验证日志与导出字段成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅登记allowlist
2. 禁止ip/peer/question/prompt/apiKey/token/referrer/query
3. 输出字段名不输出值

**错误**

- `FORBIDDEN` / HTTP 403：PRIVACY_FIELD_FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部
- 并发：纯函数
- 幂等：同names同result
- 依赖：无

## analytics.auditStateAllowed

- 级别：参考
- 文件：`modules/analytics/internal/privacy.go`
- 作用：审计/持久参考支持：auditStateAllowed

```go
func auditStateAllowed(state string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 仅承认机器合同AuditMetadataV1登记的状态闭集，不接受原文。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service鉴权生产者
- 并发：纯函数，不通过日志修复非法payload
- 幂等：同audit同结果
- 依赖：无
- 其他依赖说明：根contracts/errors.json

## analytics.IngestBeacon

- 级别：约束
- 文件：`modules/analytics/internal/http_beacons.go`
- 作用：匿名HTTP接收

```go
func IngestBeacon(ctx context.Context, op OperationContext, in IngestBeaconInput) (IngestBeaconResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `IngestBeaconInput` | 匿名HTTP接收；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `IngestBeaconResult` | 匿名HTTP接收成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 浏览器公共路由由portal转POST /v1/internal/beacons；固定portal纯服务身份签全部正文，直接analytics公共入口缺可信元数据失败关闭。
2. 原公开正文入口2048字节，内部正文4096字节；严格结构边界一次验证；正文dataEpoch必须等于可信op。
3. ValidateBeacon仅核业务事件/来源域语义；每site当前分钟3000、有界队列4096，超限只计漏数回204。
4. 已知crawler只内存过滤；同一可信ReportingTimeZone日盐hash/离线国家/时延桶后只保允许字段。
5. 不读代理头，不持久原IP/UA；GeoIP缺库未知ZZ；队列满不暴露排队结果。

**错误**

- `VALIDATION` / HTTP 400：INVALID_BEACON：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅写有界内存队列
- 授权：permission：anonymous；rule：anonymous
- 并发：4096队列；每site3000/min入口限制；不影响页面返回
- 幂等：beaconId去重在flush落盘
- 依赖：`analytics.ValidateBeacon`、`analytics.HashDayToken`、`analytics.ClassifyCrawler`、`analytics.LookupCountry`、`analytics.BucketLatency`

## analytics.FlushBeacons

- 级别：参考
- 文件：`modules/analytics/internal/beacon_store.go`
- 作用：批量持久匿名条目

```go
func FlushBeacons(ctx context.Context, op OperationContext, in FlushBeaconsInput) (FlushBeaconsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `FlushBeaconsInput` | 批量持久匿名条目；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FlushBeaconsResult` | 批量持久匿名条目成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 单flush互斥，每批最多500，insert ignore真实beaconId；只对实际新增行计Persisted。
2. 新增行影响的UTC小时与可信本地日各自更新beacon_period_watermarks，不用全局beacons修订使历史桶失效。
3. 漏数即使空队列也同事务持久；事务失败条目最多内存保留5秒，超期计漏数；不记录payload。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写匿名inbox
- 授权：permission：service；rule：service worker
- 并发：单flush worker lease；不记录丢弃payload
- 幂等：beaconId唯一幂等
- 依赖：`analytics.advanceWatermark`、`analytics.beginWrite`
- 其他依赖说明：私有SQLite

## analytics.AggregateHourly

- 级别：约束
- 文件：`modules/analytics/internal/aggregation.go`
- 作用：精确重算封闭小时桶

```go
func AggregateHourly(ctx context.Context, op OperationContext, in AggregateHourlyInput) (AggregateHourlyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AggregateHourlyInput` | 精确重算封闭小时桶；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AggregateHourlyResult` | 精确重算封闭小时桶成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 可信本域worker注入JobExecutionInput与不可变SourceWatermark，HTTP不得自报。
2. 锁外CheckLease，事务内当前epoch/同job/worker/fence/attempt/expiry；当前period来源必须等原输入，改变则STALE_RESULT，新输入另计划。
3. 完整已封闭UTC小时按document/locale count与dailyHash distinct，时延分桶；同事务替换桶/真实水位/outbox/原结果，提交前再核镜像截止。
4. 超过原明细30天且缺原聚合失败关闭，保留历史真实聚合不重建0。

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_HOUR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写聚合/watermark/outbox
- 授权：permission：service；rule：service worker
- 并发：关闭小时后保留24h允许补到；事务replace不会累计重复
- 幂等：hour/sourceWatermark幂等
- 依赖：`analytics.checkAggregateLease`、`analytics.verifyAggregateLease`、`analytics.readAggregate`、`analytics.saveAggregate`、`analytics.enqueueAggregate`
- 其他依赖说明：私有SQLite

## analytics.AggregateDaily

- 级别：约束
- 文件：`modules/analytics/internal/aggregation.go`
- 作用：按站点日聚合

```go
func AggregateDaily(ctx context.Context, op OperationContext, in AggregateDailyInput) (AggregateDailyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AggregateDailyInput` | 按站点日聚合；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AggregateDailyResult` | 按站点日聚合成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 唯一可信ReportingTimeZone必须非空且等任务timezone，DST以AddDate日历日完整封闭边界。
2. 可信worker注入真实执行与不可变来源水位；锁外CheckLease/事务内fence及来源CAS，迟到变化新输入不改旧输入。
3. document/locale总UV在所有维度间distinct；source/country维度另表，不SUM维度UV；维度最多100000否则事务失败。
4. 完整替换日总量/维度与原结果/outbox/真实修订；提交前核租约镜像；原明细已过保留而缺原结果不可重建0。

**错误**

- `VALIDATION` / HTTP 400：INVALID_TIMEZONE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：AGGREGATE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写day派生
- 授权：permission：service；rule：service worker
- 并发：日分界与DST用IANA时区固定配置revision
- 幂等：day/timezone/sourceWatermark幂等
- 依赖：`analytics.checkAggregateLease`、`analytics.verifyAggregateLease`、`analytics.readAggregate`、`analytics.saveAggregate`、`analytics.enqueueAggregate`

## analytics.enqueueAggregate

- 级别：参考
- 文件：`modules/analytics/internal/aggregation.go`
- 作用：统计任务参考支持：enqueueAggregate

```go
func enqueueAggregate(tx *sql.Tx, op OperationContext, revision int64, day, kind string) error
```

**输入**

无

**输出**

无

**步骤**

1. 与聚合同事务写平台outbox，真实修订；事件只经jobs。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`platform.EnqueueEvent`
- 其他依赖说明：私有SQLite

## analytics.readAggregate

- 级别：参考
- 文件：`modules/analytics/internal/aggregation.go`
- 作用：统计任务参考支持：readAggregate

```go
func readAggregate(ctx context.Context, tx *sql.Tx, kind, key string, expected int64, out any) (bool, int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取对应period来源水位，必须等不可变输入；同已聚合水位返回真实原结果，较新来源要求新任务。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：无
- 其他依赖说明：私有SQLite

## analytics.saveAggregate

- 级别：参考
- 文件：`modules/analytics/internal/aggregation.go`
- 作用：统计任务参考支持：saveAggregate

```go
func saveAggregate(ctx context.Context, tx *sql.Tx, kind, key string, source int64, result any) error
```

**输入**

无

**输出**

无

**步骤**

1. 与聚合替换/outbox同事务持久来源水位和真实结果。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：无
- 其他依赖说明：私有SQLite

## analytics.ValidateDateRange

- 级别：参考
- 文件：`modules/analytics/internal/query_validation.go`
- 作用：规范查询范围

```go
func ValidateDateRange(ctx context.Context, op OperationContext, in ValidateDateRangeInput) (ValidateDateRangeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateDateRangeInput` | 规范查询范围；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateDateRangeResult` | 规范查询范围成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. RFC3339含zone转UTC，左闭右开from<to；省略终点使用当前时间，省略起点用终点前7日。
2. MaxDays=0表示366；显式只能1..366，范围超过拒绝，不做整数溢出或静默截断。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：查询内部
- 并发：纯函数
- 幂等：同range同结果
- 依赖：无

## analytics.pageInterval

- 级别：参考
- 文件：`modules/analytics/internal/query_validation.go`
- 作用：本域参考支持：pageInterval

```go
func pageInterval(ctx context.Context, op OperationContext, rt *Runtime, in QueryPageStatisticsInput) (time.Time, time.Time, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. UTC完整小时或唯一可信部署报表时区完整日历桶，默认7日；日模式按AddDate处理DST，未知时区不可用。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：`analytics.ValidateDateRange`

## analytics.QueryPageStatistics

- 级别：约束
- 文件：`modules/analytics/internal/queries.go`
- 作用：访问汇总查询

```go
func QueryPageStatistics(ctx context.Context, op OperationContext, in QueryPageStatisticsInput) (QueryPageStatisticsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `QueryPageStatisticsInput` | 访问汇总查询；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `QueryPageStatisticsResult` | 访问汇总查询成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 生成边界一次核stats.read，内部只取受信本域runtime。
2. 读取真实SQLite一致快照及已聚合pages水位/真实漏数；hour只完整UTC小时，day只唯一部署报表时区日历桶，缺配置不可用。
3. 默认最近7完整日/168完整UTC小时；显式边界必须对齐，day最多366日历日支持DST；输出实际fromUTC/toUTC/granularity。
4. 日报UV从独立总表读取，不求和source/country维度distinct；上限10000行超预算错误，不截断。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：stats.read；rule：stats.read
- 并发：SQLite快照读，最多1000聚合行分页
- 幂等：GET幂等
- 依赖：`analytics.runtimeFrom`、`analytics.pageInterval`、`analytics.readPageSnapshot`

## analytics.QueryAudits

- 级别：约束
- 文件：`modules/analytics/internal/queries.go`
- 作用：审计分页

```go
func QueryAudits(ctx context.Context, op OperationContext, in QueryAuditsInput) (QueryAuditsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `QueryAuditsInput` | 审计分页；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `QueryAuditsResult` | 审计分页成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 生成边界一次核audit.read；limit默认20最大100。
2. 复用平台SignCursor/VerifyCursor固定scope，绑定稳定subject/session及修订/site/epoch/action/limit，不绑每次proof expiry。
3. 首次固定真实seq上界与保留锚；按seq升序keyset，不混入后来追加。锚变化、过期或身份筛选变化STALE_RESULT，不跳过被删页。
4. 可选from/to必须同现，规范UTC固定区间纳入游标绑定与SQL筛选；不每页用当前时刻重算默认日期。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_CURSOR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：audit.read；rule：audit.read
- 并发：快照读；不跨域查原正文
- 幂等：GET幂等
- 依赖：`analytics.runtimeFrom`、`analytics.storageError`、`analytics.ValidateDateRange`

## analytics.ReadAudit

- 级别：约束
- 文件：`modules/analytics/internal/queries.go`
- 作用：读取单条审计

```go
func ReadAudit(ctx context.Context, op OperationContext, in ReadAuditInput) (ReadAuditResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReadAuditInput` | 读取单条审计；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadAuditResult` | 读取单条审计成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 生成边界一次核audit.read；同一真实快照读当前安全记录及最近前驱或保留链锚。
2. V1链hash使用UTF8前hash十六进制串拼canonical payload，排除自身hash/previousHash，内部metadata参与hash但不作为原文返回。
3. hash格式损坏INTEGRITY_ERROR；合法格式的hash或前驱不符返回chainValid=false，不修复。
4. 读取真实hash_schema_version及来源事实；先按原始存储payload核链，再只输出严格安全metadata，异常省略并chainValid=false。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：AUDIT_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：audit.read；rule：audit.read
- 并发：只读
- 幂等：GET幂等
- 依赖：`analytics.runtimeFrom`、`analytics.ComputeAuditHash`

## analytics.readPageSnapshot

- 级别：参考
- 文件：`modules/analytics/internal/queries.go`
- 作用：本域参考支持：readPageSnapshot

```go
func readPageSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, in QueryPageStatisticsInput, begin, end time.Time, location *time.Location) (QueryPageStatisticsResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同快照读取真实已聚合水位/漏数与完整小时或日总表，10000行预算，空集合只来自真实库。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：无

## analytics.ValidateAudit

- 级别：参考
- 文件：`modules/analytics/internal/audit_validation.go`
- 作用：校验审计allowlist

```go
func ValidateAudit(ctx context.Context, op OperationContext, in ValidateAuditInput) (ValidateAuditResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateAuditInput` | 校验审计allowlist；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateAuditResult` | 校验审计allowlist成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只接受固定动作/生产者归属，origin仅由已核HTTP或EventEnvelope提供的内部字段，不改op.CallerService。
2. 实时模式原主体必须等于当前可信主体且发生时间±5min；历史允许过去原事实，不允许未来超过5min。
3. 不透明audit/resource/actor引用、成功OK或兼容SUCCESS/公共错误码；不接任意message。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service鉴权生产者
- 并发：纯函数，不通过日志修复非法payload
- 幂等：同audit同结果
- 依赖：无
- 其他依赖说明：根contracts/errors.json

## analytics.ComputeAuditHash

- 级别：参考
- 文件：`modules/analytics/internal/audit_chain.go`
- 作用：计算审计完整性链

```go
func ComputeAuditHash(ctx context.Context, op OperationContext, in ComputeAuditHashInput) (ComputeAuditHashResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ComputeAuditHashInput` | 计算审计完整性链；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ComputeAuditHashResult` | 计算审计完整性链成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. V1固定payload字段auditId/actorId/action/resourceId/resultCode/occurredAt及内部safeMetadata字段metadata（空亦保留）；排除链自身hash和previousHash。
2. SHA256(UTF8(previousHash十六进制串)||canonicalJSON(payload))；纯函数，不含本地链外接收时间。
3. V2额外完整涵盖schemaVersion/siteId/producer/actorKind/sourceOperationId/originEpoch；版本超过2失败，不改V1原链。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service内部
- 并发：纯函数；链不声称防拥有磁盘和密钥的管理员篡改
- 幂等：同prev/data同hash
- 依赖：无

## analytics.AppendAudit

- 级别：约束
- 文件：`modules/analytics/internal/audit_store.go`
- 作用：追加安全审计

```go
func AppendAudit(ctx context.Context, op OperationContext, in AppendAuditInput) (AppendAuditResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AppendAuditInput` | 追加安全审计；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AppendAuditResult` | 追加安全审计成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 固定action→producer只允许已验证原owner；历史actorKind/occurredAt/sourceOperationId三字段同现，必须纯服务SubjectID==CallerService，原actorId显式提供。
2. 旧无三字段分支仍actorId==当前可信主体；部分组合拒绝，不复用过期portal proof，不拿服务主体假冒原用户。
3. 严格元数据白名单，V2 canonical涵盖原producer/site/epoch/actorKind/sourceOperation/time与结果；HTTP旧SUCCESS标准化OK，V1旧hash只读不改。
4. 共享appendTrustedAudit在业务事务追加唯一auditId/prevHash/hash；同原事实回原结果，异原事实CONFLICT。生产者异步只能原事务outbox→jobs→消费，不改成直连旁路。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：AUDIT_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：追加audit链
- 授权：permission：service；rule：service指定action生产者
- 并发：单链append事务，禁止更新原审计
- 幂等：auditId+payloadHash幂等
- 依赖：`analytics.runtimeFrom`、`analytics.normalizeAuditFact`、`analytics.appendTrustedAudit`

## analytics.appendTrustedAudit

- 级别：参考
- 文件：`modules/analytics/internal/audit_store.go`
- 作用：审计/持久参考支持：appendTrustedAudit

```go
func appendTrustedAudit(ctx context.Context, op OperationContext, fact trustedAuditFact, envelope *platform.EventEnvelope) (AppendAuditResult, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同事务平台ClaimInbox与追加原事实；已处理信封不重新插入被保留期清掉的旧记录。
2. 唯一auditId原摘要不同CONFLICT；链尾和保留锚、hash与全来源事实同事务；固定原事实重放只返回旧真实结果。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service鉴权生产者
- 并发：纯函数，不通过日志修复非法payload
- 幂等：同audit同结果
- 依赖：`analytics.beginWrite`、`analytics.ComputeAuditHash`、`analytics.advanceWatermark`、`analytics.insertAuditRecord`
- 其他依赖说明：根contracts/errors.json

## analytics.insertAuditRecord

- 级别：参考
- 文件：`modules/analytics/internal/audit_store.go`
- 作用：私有导出/保留期支持：insertAuditRecord

```go
func insertAuditRecord(work context.Context, op OperationContext, tx *sql.Tx, record AuditRecord, historical bool, envelope *platform.EventEnvelope, digest string) (AppendAuditResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 已认证安全审计事实接真实尾hash/锚，写V2原来源事实；允许调用方与保留期锚/删除同事务。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.ComputeAuditHash`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.ValidateUsageEvent

- 级别：参考
- 文件：`modules/analytics/internal/usage_store.go`
- 作用：校验V2账项金额与状态语义

```go
func ValidateUsageEvent(change UsageLedgerChange) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `change` | `UsageLedgerChange` | 已通过生成schema的V2权威账项 | 只核金额known与状态语义，不重复字段/enum schema校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 金额或状态关系矛盾 | 成功nil；失败INTEGRITY_ERROR |

**步骤**

1. amountKnown与amount严格同现；已知金额必须USD非负安全整数。
2. price_unknown不得带已知金额；settled/reconciled/price_unknown状态须与ledgerStatus一致，superseded/voided/deleted保留原状态。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：金额known/值或state与ledgerStatus关系矛盾（不可重试）；调用方不持久半事实

- 副作用：无
- 授权：permission：service；rule：仅本域已核验V2页/投影链
- 并发：纯校验；不接受任意服务报告扣款
- 幂等：同change同结果
- 依赖：无

## analytics.ProjectUsage

- 级别：参考
- 文件：`modules/analytics/internal/usage_store.go`
- 作用：在独立事务持久已核验的完整V2费用事实

```go
func ProjectUsage(ctx context.Context, op OperationContext, in UsageProjectionInput) (ProjectUsageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `UsageProjectionInput` | 已核验完整V2 changes与observations；当前DataEpoch与每个原事实sourceDataEpoch分别保留 | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ProjectUsageResult` | 更新派生费用行成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 核当前站点/代次与schemaVersion6；账项和未入账观察逐事实保存原owner/site/sourceDataEpoch/root/op/class/provider/model独立revision、amountKnown/USD微额/state/time/captureKind，unknown/hold不补0。
2. 按dataEpoch+changeRevision将每个原时间线修订无损写入事实表；同修订异事实、同实体修订不同时间线/事实冲突拒绝，旧修订只留历史，不使当前投影倒退。
3. 仅新事实更新ledger/observation当前投影与真实最大changeRevision水位；不建立usage-v2-complete，不激活V1查询，不猜旧行来源。
4. 事件入口必须调用ProjectUsageTx并复用inbox首写后的同一事务；本参考函数独立开启安全写事务时同样全部成功提交、任一失败回滚。

**错误**

- `CONFLICT` / HTTP 409：USAGE_CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写V2与历史费用事实、当前投影和真实changeRevision水位；不写inbox（由事件入口同事务调用）
- 授权：permission：service；rule：service model-gateway事件
- 并发：单SQLite短事务；按全局/实体修订与payload hash冲突校验收紧，事务内旧修订不覆盖新投影
- 幂等：completePage/changeRevision/originalFactHash幂等
- 依赖：`analytics.ProjectUsageTx`、`platform.CanonicalJSON`

## analytics.ProjectUsageTx

- 级别：参考
- 文件：`modules/analytics/internal/usage_store.go`
- 作用：供事件入口把inbox首写与费用投影纳入同一事务

```go
func ProjectUsageTx(ctx context.Context, op OperationContext, tx *sql.Tx, in UsageProjectionInput) (ProjectUsageResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核schema/site/epoch后逐事实幂等持久；任一冲突返回错误，不提交。

**错误**

- `CONFLICT` / HTTP 409：同修订或同实体对应不同原事实（不可重试）；由调用方回滚事务

- 副作用：在调用方事务内写V2事实、投影与水位
- 授权：permission：service；rule：仅本域可信事件处理链
- 并发：不创建事务，由调用方串行短事务控制
- 幂等：changeRevision与原始事实hash幂等
- 依赖：`analytics.recordUsageFact`、`analytics.projectUsageLedger`、`analytics.projectUsageObservation`、`analytics.ValidateUsageEvent`

## analytics.recordUsageFact

- 级别：参考
- 文件：`modules/analytics/internal/usage_store.go`
- 作用：按时间线修订无损保存费用事实并检测冲突

```go
func recordUsageFact(ctx context.Context, tx *sql.Tx, epoch string, fact usageStoredFact) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同修订同hash跳过，同修订或同实体修订异事实拒绝；乱序旧修订只留历史。

**错误**

- `CONFLICT` / HTTP 409：原事实修订或hash冲突（不可重试）；由调用方回滚事务

- 副作用：写usage_facts_v2
- 授权：permission：service；rule：仅本域事务内
- 并发：单事务
- 幂等：changeRevision+payloadHash幂等
- 依赖：无

## analytics.projectUsageLedger

- 级别：参考
- 文件：`modules/analytics/internal/usage_store.go`
- 作用：更新费用账项当前投影且不倒退

```go
func projectUsageLedger(ctx context.Context, tx *sql.Tx, epoch string, change UsageLedgerChange, fact usageStoredFact) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 保存完整原账项字段；仅更高ledgerRevision覆盖当前行，unknown金额保持NULL。

**错误**

- `VALIDATION` / HTTP 400：金额known与原字段不一致（不可重试）；由调用方回滚事务

- 副作用：upsert usage_projection_v2
- 授权：permission：service；rule：仅本域事务内
- 并发：按ledgerRevision单调
- 幂等：更高修订CAS
- 依赖：无

## analytics.projectUsageObservation

- 级别：参考
- 文件：`modules/analytics/internal/usage_store.go`
- 作用：更新无账项原观察当前投影且不倒退

```go
func projectUsageObservation(ctx context.Context, tx *sql.Tx, epoch string, observation UsageObservation, fact usageStoredFact) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 保留完整嵌套原事实；仅更高entityRevision覆盖当前行，held/unknown不折算金额。

**错误**

- `VALIDATION` / HTTP 400：预留金额known与字段不一致（不可重试）；由调用方回滚事务

- 副作用：upsert usage_observations_v2
- 授权：permission：service；rule：仅本域事务内
- 并发：按entityRevision单调
- 幂等：更高实体修订CAS
- 依赖：无

## analytics.AggregateUsage

- 级别：约束
- 文件：`modules/analytics/internal/usage_aggregation.go`
- 作用：费用派生聚合

```go
func AggregateUsage(ctx context.Context, op OperationContext, in AggregateUsageInput) (AggregateUsageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `AggregateUsageInput` | 费用派生聚合；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AggregateUsageResult` | 费用派生聚合成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按gateway账项最终state取整数micros
2. 按class/provider/model group
3. 事务replace day聚合

**错误**

- `INTERNAL` / HTTP 500：OVERFLOW：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写daily_usage
- 授权：permission：service；rule：service worker
- 并发：checked int64 sum溢出拒绝，不用float
- 幂等：day/sourceWatermark幂等
- 依赖：无
- 其他依赖说明：usage_projection

## analytics.FetchUsagePage

- 级别：参考
- 文件：`modules/analytics/internal/gateway_client.go`
- 作用：经可注入生成HTTP端口收齐固定V2费用上界，保留原事实供隔离投影

```go
func FetchUsagePage(ctx context.Context, op OperationContext, in UsageGatewayFetchInput) (UsageGatewaySnapshot, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `UsageGatewayFetchInput` | 原起点、可选固定上界与当前代次；不接受半途cursor作为完整范围 | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UsageGatewaySnapshot` | 固定范围收齐后的全部V2原页；每页保留hash/游标、账项变更及无账项观察，错误返回零值 | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 从可信WithUsageGateway读取生成端口；未装配失败关闭。固定SiteID/DataEpoch须与可信op和输入一致，op必须为analytics纯服务；不创建或续用用户证明。
2. 从原afterRevision起点，以limit=100调用model-gateway:readUsageStatisticsPage；首请求watermark=0时采用首响应上界，之后每页固定该watermark与原afterRevision，仅续原nextCursor。总调用60s、单页2.5s，context继续传给HTTP。 累计每页原始HTTP JSON字节数，完整范围最多32MiB，单页最多2MiB；超限返回BUDGET_EXCEEDED并丢弃全部页，不截断或返回部分范围。
3. readUsageGatewayPage在HTTP响应边界先用生成的OpenAPI x-runtime-payload-schemas回调一次校验生产方V2 schema，再对完整原JSON仅删pageHash后CanonicalJSON作SHA256核原摘要；严格解码拒未知字段/额外JSON，并核本地模型canonical与完整原页一致，不转换原时间或补字段。
4. validateUsageGatewayPage核schemaVersion=2、两数组非null且合计不超100、feeCoverage=posted-and-inflight及completeFeeCoverage=true；跨页watermark固定、throughRevision单调不越界，两类事实按changeRevision共享时间线，不重复、不倒退、不漏分页进度。
5. 原账项独立ledgerRevision、原owner/site/sourceDataEpoch/root/op/class/provider/model/金额状态/原时间/captureKind和全部观察事实无损保留。unknown不补0，原sourceDataEpoch不改当前代次；网关已认证恢复投影的历史来源代次允许保留。
6. 保留每页原hash/游标和完整changes/observations；仅nextCursor为空且throughRevision等于watermark才返回完整范围。游标重复、schema/hash/epoch/上界/网络错误均返回零值，无投影写入和部分激活；afterRevision非零只表示增量范围完整，不能冒完整全站基线。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：端口或校验未装配、生产方不可达；原生产方Problem保持失败（可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `VALIDATION` / HTTP 400：请求固定上界早于原起点或负修订（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `FORBIDDEN` / HTTP 403：不是analytics纯服务（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `VERSION_MISMATCH` / HTTP 409：注入的真实site/epoch与当前操作或输入不一致，或生产方拒绝旧epoch（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `UNSUPPORTED_SCHEMA` / HTTP 422：不是完整V2 posted-and-inflight页或未知观察种类（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `INTEGRITY_ERROR` / HTTP 422：原hash/schema/无损模型/共享时间线顺序/原事实关联/金额已知状态/末页封口矛盾（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `STALE_RESULT` / HTTP 409：固定上界改变或分页水位越界/倒退，或生产方游标过期（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `BUDGET_EXCEEDED` / HTTP 429：单页超过2MiB、完整范围原始HTTP JSON累计超过32MiB，或生产方预算失败；不截断、不返回部分范围（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `CANCELLED` / HTTP 409：上游取消中止HTTP读取（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `DEADLINE_EXCEEDED` / HTTP 504：总任务或单页IO截止（可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果

- 副作用：HTTP只读
- 授权：permission：service；rule：本域可信注入真实site/epoch与analytics纯服务操作；服务断言由生成客户端/平台负责，原来源epoch由生产方认证
- 并发：每页2.5s，总调用60s；顺序读取固定上界；与调用方ctx共享取消，不重试或重建模型请求；单页2MiB，全部返回原始HTTP JSON累计32MiB，超限不截断
- 幂等：固定afterRevision/watermark只读；每次从原起点完整收齐，不保存或激活半页
- 依赖：`model-gateway:readUsageStatisticsPage`、`analytics.WithUsageGateway`、`analytics.readUsageGatewayPage`、`analytics.validateUsageGatewayPage`、`platform.CanonicalJSON`、`platform.NewPayloadValidator`
- 其他依赖说明：UsageStatisticsPageReader（由*gen.ModelGatewayClient满足）；UsageGateway.ValidatePage（AnalyticsUsageStatisticsPageResult生产方V2 schema）

## analytics.WithUsageGateway

- 级别：参考
- 文件：`modules/analytics/internal/gateway_client.go`
- 作用：注入本域可信只读费用端口

```go
func WithUsageGateway(ctx context.Context, gateway *UsageGateway) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信调用上下文 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |
| `gateway` | `*UsageGateway` | 真实站点/当前代次与生成读取端口 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 包含只读端口的调用上下文 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |

**步骤**

1. 将UsageGateway指针放入私有context key，继承已有取消和授权；不创建身份、用户证明、HTTP请求或默认配置。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域可信调用链装配或FetchUsagePage响应边界使用
- 并发：无共享可变状态；纯本地操作
- 幂等：相同输入产生相同结果
- 依赖：无

## analytics.readUsageGatewayPage

- 级别：参考
- 文件：`modules/analytics/internal/gateway_client.go`
- 作用：核生产方V2 schema、完整原JSON摘要与无损模型

```go
func readUsageGatewayPage(raw json.RawMessage, validate func(json.RawMessage) error) (UsageGatewayPage, error)
```

**输入**

无

**输出**

无

**步骤**

1. 限制原响应2MiB，并以wire装配的生产方V2 schema校验原始JSON一次。
2. 对原JSON只删除pageHash字段后调用platform.CanonicalJSON与SHA256核原摘要；严格解码拒未知字段/额外JSON，再核本地模型canonical不损失原字段和空数组语义。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：端口或校验未装配、生产方不可达；原生产方Problem保持失败（可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `VALIDATION` / HTTP 400：请求固定上界早于原起点或负修订（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `FORBIDDEN` / HTTP 403：不是analytics纯服务（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `VERSION_MISMATCH` / HTTP 409：注入的真实site/epoch与当前操作或输入不一致，或生产方拒绝旧epoch（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `UNSUPPORTED_SCHEMA` / HTTP 422：不是完整V2 posted-and-inflight页或未知观察种类（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `INTEGRITY_ERROR` / HTTP 422：原hash/schema/无损模型/共享时间线顺序/原事实关联/金额已知状态/末页封口矛盾（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `STALE_RESULT` / HTTP 409：固定上界改变或分页水位越界/倒退，或生产方游标过期（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `BUDGET_EXCEEDED` / HTTP 429：单页超过2MiB、完整范围原始HTTP JSON累计超过32MiB，或生产方预算失败；不截断、不返回部分范围（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `CANCELLED` / HTTP 409：上游取消中止HTTP读取（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `DEADLINE_EXCEEDED` / HTTP 504：总任务或单页IO截止（可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果

- 副作用：无
- 授权：permission：service；rule：仅本域可信调用链装配或FetchUsagePage响应边界使用
- 并发：无共享可变状态；纯本地操作
- 幂等：相同输入产生相同结果
- 依赖：`platform.CanonicalJSON`、`platform.NewPayloadValidator`

## analytics.validateUsageGatewayPage

- 级别：参考
- 文件：`modules/analytics/internal/gateway_client.go`
- 作用：核共享时间线进度及原费用关联

```go
func validateUsageGatewayPage(page UsageGatewayPage, siteID string, upper, through int64) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `page` | `UsageGatewayPage` | 已核schema/hash的原页 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |
| `siteID` | `string` | 可信当前站点 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |
| `upper` | `int64` | 本轮固定上界 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |
| `through` | `int64` | 上一页已读取水位 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功nil；失败返回公共安全错误且不带半页 | 可信内部依赖；不接受浏览器构造，错误说明不含原业务数据 |

**步骤**

1. 核schema2/完整覆盖标记/空数组与两类事实合计≤100；水位不退不越界且上界固定。
2. 分别核原账项/观察顺序和来源site；保留认证历史sourceDataEpoch，不以当前配置或事件修订补历史。核已知金额仅带真实USD微额，未知金额与未知预留不带金额；保留原独立版本和全部in-flight事实。
3. 按changeRevision合并两数组，拒重复/倒序/越界；有cursor必须有真实进度且未到上界；无cursor必须throughRevision=watermark。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：端口或校验未装配、生产方不可达；原生产方Problem保持失败（可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `VALIDATION` / HTTP 400：请求固定上界早于原起点或负修订（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `FORBIDDEN` / HTTP 403：不是analytics纯服务（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `VERSION_MISMATCH` / HTTP 409：注入的真实site/epoch与当前操作或输入不一致，或生产方拒绝旧epoch（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `UNSUPPORTED_SCHEMA` / HTTP 422：不是完整V2 posted-and-inflight页或未知观察种类（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `INTEGRITY_ERROR` / HTTP 422：原hash/schema/无损模型/共享时间线顺序/原事实关联/金额已知状态/末页封口矛盾（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `STALE_RESULT` / HTTP 409：固定上界改变或分页水位越界/倒退，或生产方游标过期（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `BUDGET_EXCEEDED` / HTTP 429：单页超过2MiB、完整范围原始HTTP JSON累计超过32MiB，或生产方预算失败；不截断、不返回部分范围（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `CANCELLED` / HTTP 409：上游取消中止HTTP读取（不可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果
- `DEADLINE_EXCEEDED` / HTTP 504：总任务或单页IO截止（可重试）；只读操作无持久副作用；丢弃全部内存页并返回零值，不激活半结果

- 副作用：无
- 授权：permission：service；rule：仅本域可信调用链装配或FetchUsagePage响应边界使用
- 并发：无共享可变状态；纯本地操作
- 幂等：相同输入产生相同结果
- 依赖：`analytics.ValidateUsageEvent`

## analytics.ReconcileUsage

- 级别：约束
- 文件：`modules/analytics/internal/reconciliation.go`
- 作用：对账修复派生统计

```go
func ReconcileUsage(ctx context.Context, op OperationContext, in ReconcileUsageInput) (ReconcileUsageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReconcileUsageInput` | 对账修复派生统计；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReconcileUsageResult` | 对账修复派生统计成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验system.manage
2. 获取gateway固定ledgerwatermark
3. 分页校验缺漏/修订
4. 事务replace projection/aggregate并写discrepancy事件

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：GATEWAY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：修改analytics派生数据，禁止改gateway余额
- 授权：permission：system.manage；rule：system.manage或service定时任务
- 并发：单reconcile锁，生产事件在更高watermark保留等待合并
- 幂等：range/watermark幂等
- 依赖：`analytics.FetchUsagePage`、`analytics.ProjectUsage`、`analytics.AggregateUsage`

## analytics.ConsumeAnalyticsEvent

- 级别：约束
- 文件：`modules/analytics/internal/event_handler.go`
- 作用：消费安全审计和费用事件

```go
func ConsumeAnalyticsEvent(ctx context.Context, op OperationContext, in ConsumeAnalyticsEventInput) (ConsumeAnalyticsEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ConsumeAnalyticsEventInput` | 消费安全审计和费用事件；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeAnalyticsEventResult` | 消费安全审计和费用事件成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅接受model-gateway invocation.settled/invocation.reconciled v1；核事件/载荷dataEpoch、subject=invocationId、settled entryId及reconciled proofKind原引用，事件actualCost不作为账项真相。
2. FetchUsagePage从原起点收齐固定V2上界并核每页pageHash、完整来源闭包与32MiB总预算；未收齐不得进入投影，不确认。
3. beginWrite后以inbox.ClaimInboxBeforeWrite作为首项写入，同eventId+payloadHash幂等；同ID异hash严格隔离，低修订跳过，旧dataEpoch拒绝。
4. Claim成功后在同一事务核事件所引账项或not_charged原attempt存在且来源一致，再调用ProjectUsageTx持久完整changes/observations与真实watermark，更新V2事件水位后提交；提交成功才返回accepted=true，由analytics.ConsumeEvents确认。引用不符或失败返回错误等待重投，不公开半页或猜金额。

**错误**

- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写platform inbox、V2费用事实/当前投影和真实事件水位
- 授权：permission：service；rule：service白名单
- 并发：权威页读取在事务外；inbox首写后费用事实投影同SQLite事务提交，按changeRevision不回退
- 幂等：eventId/payloadHash/changeRevision/originalFactHash幂等
- 依赖：`analytics.FetchUsagePage`、`analytics.parseUsageEventReference`、`analytics.validateUsageEventReference`、`analytics.ProjectUsageTx`、`platform.ClaimInboxBeforeWrite`

## analytics.ProjectQaTurn

- 级别：约束
- 文件：`modules/analytics/internal/event_handler.go`
- 作用：把问答轮次事件投影为按日计数

```go
func ProjectQaTurn(ctx context.Context, op OperationContext, input EventDelivery) (EventHandled, error)
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

1. 生产方严格schema边界已验证；只qa v1，epoch/subject==turnId，completed.answerRevision==envelope.revision。
2. inbox与turn来源修订同事务：重复不写，低修订跳过，同修订不同完整来源hash冲突；更高修订先撤旧分类再应用新事实。
3. 日界线唯一可信ReportingTimeZone；失败payload无language时内部locale空仅表示未提供，不假造用户语言。
4. 只保存turnId、分类、时间与计数，未答引用opaque；不保存问题正文/访客身份。

**错误**

- `VERSION_MISMATCH`：旧代事件（不可重试）

- 副作用：写问答日计数
- 授权：permission：service；rule：仅由本域ConsumeEvents调用
- 并发：日计数行CAS
- 幂等：eventId去重
- 依赖：`platform.ClaimInbox`、`analytics.applyQaCounts`

## analytics.applyQaCounts

- 级别：参考
- 文件：`modules/analytics/internal/event_handler.go`
- 作用：问答统计参考支持：applyQaCounts

```go
func applyQaCounts(ctx context.Context, tx *sql.Tx, epoch string, fact qaTurnProjection, delta int64) error
```

**输入**

无

**输出**

无

**步骤**

1. 按真实旧/新来源分类撤旧计数或增新；撤旧必须存在正计数，否则完整性失败。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：仅已验证入口/consumer调用，不接受客户端自报来源。
- 并发：本域同事务
- 幂等：原事件来源事实绑定
- 依赖：无
- 其他依赖说明：本域私有SQLite

## analytics.ValidateCSVCell

- 级别：参考
- 文件：`modules/analytics/internal/csv.go`
- 作用：安全CSV单元格

```go
func ValidateCSVCell(ctx context.Context, op OperationContext, in ValidateCSVCellInput) (ValidateCSVCellResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidateCSVCellInput` | 安全CSV单元格；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateCSVCellResult` | 安全CSV单元格成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 首字符=+@-或tab/CR/LF加单引号；含逗号/引号/换行按CSV双引号包裹并转义，不接受客户端自定义列。

**错误**

- `VALIDATION` / HTTP 400：CELL_TOO_LARGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：export内部
- 并发：纯函数，每cell≤8192bytes
- 幂等：同value同result
- 依赖：无

## analytics.PurgeBeacons

- 级别：参考
- 文件：`modules/analytics/internal/retention.go`
- 作用：清理短期匿名行

```go
func PurgeBeacons(ctx context.Context, op OperationContext, in PurgeBeaconsInput) (PurgeBeaconsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PurgeBeaconsInput` | 清理短期匿名行；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PurgeBeaconsResult` | 清理短期匿名行成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 有界500条，仅真实30天前明细；page-view若小时/日派生来源水位未完整匹配，删前真实漏数+1不双计。
2. 同事务删原明细/真实漏数/保留水位；不推进period水位使长期聚合失效，不清真实长期日汇总。
3. 旧日期随机盐新日首次hash前/每分钟独立擦除control，不将原IP/UA或盐进快照。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删除匿名行/dayHash
- 授权：permission：service；rule：service worker
- 并发：≤500批事务，不删除未来watermark
- 幂等：重复purge幂等
- 依赖：无
- 其他依赖说明：aggregation_watermarks

## analytics.PurgeAuditHistory

- 级别：参考
- 文件：`modules/analytics/internal/retention.go`
- 作用：审计保留期清理

```go
func PurgeAuditHistory(ctx context.Context, op OperationContext, in PurgeAuditHistoryInput) (PurgeAuditHistoryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PurgeAuditHistoryInput` | 审计保留期清理；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PurgeAuditHistoryResult` | 审计保留期清理成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 默认180最少30，固定analytics纯服务策略或当前system.manage；只删除逐条核payload/hash/previousHash的最老连续过期前缀最多10000，乱序晚到旧事实可更久保留。
2. 同事务写真实最后删除seq/hash锚、删该前缀并追加V2 analytics.retention安全count审计；不按时间删链中间或重算旧hash。
3. 主体/函数/操作原回执幂等，零删除仍记录真实操作事实；原事件inbox保留避免旧事实重新插入。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：INVALID_RETENTION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删除过期audit+写锚点
- 授权：permission：system.manage；rule：system.manage或固定service策略
- 并发：维护事务锁链，不打断链验证
- 幂等：cutoff/anchor幂等
- 依赖：`analytics.insertAuditRecord`、`analytics.ComputeAuditHash`、`analytics.readReceipt`、`analytics.saveReceipt`、`analytics.beginWrite`

## analytics.PruneDaySalts

- 级别：参考
- 文件：`modules/analytics/internal/retention.go`
- 作用：私有导出/保留期支持：PruneDaySalts

```go
func PruneDaySalts(ctx context.Context, rt *Runtime, now time.Time) error
```

**输入**

无

**输出**

无

**步骤**

1. 按可信计数当前日，擦除旧日期独立control盐/内存key，目录fsync；新日首次hash前和每分钟运行，不进快照。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.syncDirectory`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.PublishAggregateEvents

- 级别：约束
- 文件：`modules/analytics/internal/outbox.go`
- 作用：派生事件outbox派发

```go
func PublishAggregateEvents(ctx context.Context, op OperationContext, in PublishAggregateEventsInput) (PublishAggregateEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PublishAggregateEventsInput` | 派生事件outbox派发；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishAggregateEventsResult` | 派生事件outbox派发成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 实际本域outbox用平台RunRelayBatch；只装配批次/真实runtime与private写窗口，领取/重试/隔离不复制。
2. 返回真实Relayed数量，失败不冒接收成功。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：JOBS_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：外部通知/写outbox
- 授权：permission：service；rule：service
- 并发：30s发送租约，至少一次
- 幂等：eventId固定
- 依赖：`jobs:publishEvent`、`platform.RunRelayBatch`

## analytics.EnterBarrier

- 级别：约束
- 文件：`modules/analytics/internal/barrier.go`
- 作用：进入analytics一致屏障

```go
func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `BarrierRequest` | 进入analytics一致屏障；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 进入analytics一致屏障成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒新审计/费用事件让生产outbox保留
2. flush匿名queue到持久安全点
3. 停止聚合/export写
4. checkpoint数据库

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写barrier，匿名队列可以明确漏数
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：等待所有在途事务和export file恢复日志到安全点
- 幂等：barrierId幂等
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.EnterBarrier`

## analytics.ReleaseBarrier

- 级别：约束
- 文件：`modules/analytics/internal/barrier.go`
- 作用：解除统计维护

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReleaseBarrierInput` | 解除统计维护；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 解除统计维护成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 检查新epoch全域ready
2. 重新拉gateway watermark对账任务
3. 恢复事件接收但不自动重调用模型

**错误**

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写barrier/对账引用任务
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：不释放到混代读取
- 幂等：barrierId幂等
- 依赖：`site-config:readRuntimeState`、`jobs:createJob`、`platform.SnapshotManager.ReleaseBarrier`

## analytics.CreateSnapshot

- 级别：约束
- 文件：`modules/analytics/internal/snapshot.go`
- 作用：导出统计快照

```go
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotExportRequest` | 导出统计快照；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 导出统计快照成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 要求safe
2. checkpoint
3. 包含聚合/审计安全元数据/费用投影/inbox/链锚
4. 匿名dayHash和导出文件排除

**错误**

- `LOCKED` / HTTP 409：BARRIER_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写snapshot对象
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：固定watermark无写入
- 幂等：backupId/barrierId幂等
- 依赖：`platform.SnapshotManager.Export`
- 其他依赖说明：私有SQLite

## analytics.StageSnapshot

- 级别：约束
- 文件：`modules/analytics/internal/import.go`
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

## analytics.DiscardSnapshot

- 级别：约束
- 文件：`modules/analytics/internal/import.go`
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

## analytics.clearOverview

- 级别：参考
- 文件：`modules/analytics/web/src/OverviewPanel.vue`
- 作用：统计UI实际支持：clearOverview

```ts
function clearOverview(): void
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：stats.read；rule：菜单仅显示能力；实际HTTP当前权限由服务核验，不在浏览器伪造凭据。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.loadOverview

- 级别：参考
- 文件：`modules/analytics/web/src/OverviewPanel.vue`
- 作用：统计UI实际支持：loadOverview

```ts
function loadOverview(): Promise<void>
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：stats.read；rule：菜单仅显示能力；实际HTTP当前权限由服务核验，不在浏览器伪造凭据。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.exportOverviewCSV

- 级别：参考
- 文件：`modules/analytics/web/src/OverviewPanel.vue`
- 作用：统计UI实际支持：exportOverviewCSV

```ts
function exportOverviewCSV(kind: 'pages' | 'usage'): Promise<void>
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：stats.read；rule：菜单仅显示能力；实际HTTP当前权限由服务核验，不在浏览器伪造凭据。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.downloadOverviewCSV

- 级别：参考
- 文件：`modules/analytics/web/src/OverviewPanel.vue`
- 作用：统计UI实际支持：downloadOverviewCSV

```ts
function downloadOverviewCSV(): Promise<void>
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：stats.read；rule：菜单仅显示能力；实际HTTP当前权限由服务核验，不在浏览器伪造凭据。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.clearAudit

- 级别：参考
- 文件：`modules/analytics/web/src/AuditExplorer.vue`
- 作用：统计UI实际支持：clearAudit

```ts
function clearAudit(): void
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：audit.read；rule：界面只显示能力；实际审计读/导出由服务即时audit.read核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.loadAudit

- 级别：参考
- 文件：`modules/analytics/web/src/AuditExplorer.vue`
- 作用：统计UI实际支持：loadAudit

```ts
function loadAudit(next?: boolean): Promise<void>
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：audit.read；rule：界面只显示能力；实际审计读/导出由服务即时audit.read核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.readAuditDetail

- 级别：参考
- 文件：`modules/analytics/web/src/AuditExplorer.vue`
- 作用：统计UI实际支持：readAuditDetail

```ts
function readAuditDetail(id: string): Promise<void>
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：audit.read；rule：界面只显示能力；实际审计读/导出由服务即时audit.read核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.exportAuditCSV

- 级别：参考
- 文件：`modules/analytics/web/src/AuditExplorer.vue`
- 作用：统计UI实际支持：exportAuditCSV

```ts
function exportAuditCSV(): Promise<void>
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：audit.read；rule：界面只显示能力；实际审计读/导出由服务即时audit.read核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.downloadAuditCSV

- 级别：参考
- 文件：`modules/analytics/web/src/AuditExplorer.vue`
- 作用：统计UI实际支持：downloadAuditCSV

```ts
function downloadAuditCSV(): Promise<void>
```

**输入**

无

**输出**

无

**步骤**

1. 组件自身输入/原状态动作，旧请求和导出读取有独立AbortController；权限失败清已加载数据，卸载不更新。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：audit.read；rule：界面只显示能力；实际审计读/导出由服务即时audit.read核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.createAnalyticsClient`、`analytics.problemText`、`analytics.utcInput`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.DownloadAuthorizedExport

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：Vue3模块交互：DownloadAuthorizedExport

```ts
function DownloadAuthorizedExport(input: DownloadAuthorizedExportInput, signal: AbortSignal): Promise<DownloadAuthorizedExportResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `DownloadAuthorizedExportInput` | Vue3模块交互：DownloadAuthorizedExport；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DownloadAuthorizedExportResult` | Vue3模块交互：DownloadAuthorizedExport成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. kind取本次create真实回执或当前明确输入，不默认pages；按固定当前权限路由请求受控CSV。
2. responseType stream逐片最多32MiB，长度不符失败，不用无界blob；完整读完后交浏览器下载，不声称用户最终保存完成。
3. 401/403清数据，过期要求显式新建，卸载取消读取，URL交浏览器后及时revoke。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：stats.read；rule：stats.read
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：analytics OpenAPI HTTP/JSON/SSE

## analytics.LoadStatistics

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：Vue3模块交互：LoadStatistics

```ts
function LoadStatistics(input: LoadStatisticsInput, signal: AbortSignal): Promise<LoadStatisticsResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `LoadStatisticsInput` | Vue3模块交互：LoadStatistics；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadStatisticsResult` | Vue3模块交互：LoadStatistics成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 同源端口并发读访问和费用投影；访问失败返回错误，费用独立不可用用明确usageProblem，不以空/0冒费用成功。
2. 显示真实漏数/page水位/usage水位/最后完整对账，不显示实时可花余额；分桶访次累加不称独立访客。
3. 401/403清已加载敏感数据，旧请求/卸载取消，不自动重试写动作。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：stats.read；rule：stats.read
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：`analytics.isAccessFailure`、`analytics.problemCode`
- 其他依赖说明：analytics OpenAPI HTTP/JSON/SSE

## analytics.LoadAuditPage

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：Vue3模块交互：LoadAuditPage

```ts
function LoadAuditPage(input: LoadAuditPageInput, signal: AbortSignal): Promise<LoadAuditPageResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `LoadAuditPageInput` | Vue3模块交互：LoadAuditPage；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadAuditPageResult` | Vue3模块交互：LoadAuditPage成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `Problem` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按允许action及完整from/to日期对查询；page≤100，游标绑定真实身份/权限/epoch/过滤，stale重载。
2. 401/403清列表和详情，链校验真实展示；原文只安全字段文本，不渲染HTML。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新组件本地状态；仅明确提交动作调用已授权API
- 授权：permission：audit.read；rule：audit.read
- 并发：组件AbortController取消旧请求；同组件同动作单飞
- 幂等：mutation稳定operationId/query只读重复安全
- 依赖：无
- 其他依赖说明：analytics OpenAPI HTTP/JSON/SSE

## analytics.problemCode

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：problemCode

```ts
function problemCode(error: unknown): string
```

**输入**

无

**输出**

无

**步骤**

1. 只抽取安全公共错误code，不把原错误详情/输入输出到界面。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：service；rule：本域UI参考支持，不在浏览器授予权限；实际调用按stats.read或audit.read当前入口核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：无
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.isAccessFailure

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：isAccessFailure

```ts
function isAccessFailure(error: unknown): boolean
```

**输入**

无

**输出**

无

**步骤**

1. 识别401/403权限失效以清组件已加载数据。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：service；rule：本域UI参考支持，不在浏览器授予权限；实际调用按stats.read或audit.read当前入口核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.problemCode`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.problemText

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：problemText

```ts
function problemText(error: unknown): string
```

**输入**

无

**输出**

无

**步骤**

1. 固定中文安全错误说明，保留stale/过期/预算语义。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：service；rule：本域UI参考支持，不在浏览器授予权限；实际调用按stats.read或audit.read当前入口核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.problemCode`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.moneyText

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：moneyText

```ts
function moneyText(micros: number): string
```

**输入**

无

**输出**

无

**步骤**

1. USD整数micros以BigInt拆6位小数，不当实时预算。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：service；rule：本域UI参考支持，不在浏览器授予权限；实际调用按stats.read或audit.read当前入口核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：无
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.utcInput

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：utcInput

```ts
function utcInput(value: string): string
```

**输入**

无

**输出**

无

**步骤**

1. 明确UTC分钟输入，拒日期自动归一化，发送规范RFC3339。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：service；rule：本域UI参考支持，不在浏览器授予权限；实际调用按stats.read或audit.read当前入口核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：无
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.createAnalyticsClient

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：createAnalyticsClient

```ts
function createAnalyticsClient(api: ModuleAPIClient): AnalyticsClient
```

**输入**

无

**输出**

无

**步骤**

1. 每挂载实例闭包保存同源端口与真实导出kind，不保存凭据或默认未知kind。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：service；rule：本域UI参考支持，不在浏览器授予权限；实际调用按stats.read或audit.read当前入口核验。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：`analytics.LoadStatistics`、`analytics.LoadAuditPage`、`analytics.FetchAuditRecord`、`analytics.RequestExport`、`analytics.DownloadAuthorizedExport`
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.FetchAuditRecord

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：FetchAuditRecord

```ts
function FetchAuditRecord(id: string, signal: AbortSignal): Promise<AuditResult>
```

**输入**

无

**输出**

无

**步骤**

1. 读真实安全审计详情与链校验，不导入别域业务数据。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：stats.read；rule：菜单仅显示能力；实际HTTP当前权限由服务核验，不在浏览器伪造凭据。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：无
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.RequestExport

- 级别：参考
- 文件：`modules/analytics/web/src/client.ts`
- 作用：统计UI实际支持：RequestExport

```ts
function RequestExport(input: { kind: ExportKind; from: string; to: string; operationId: string }, signal: AbortSignal): Promise<ExportResult>
```

**输入**

无

**输出**

无

**步骤**

1. 选择固定stats/audit当前权限路由，原operationId未知响应续接；kind只取原回执。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅组件内存或明确下载动作
- 授权：permission：stats.read；rule：菜单仅显示能力；实际HTTP当前权限由服务核验，不在浏览器伪造凭据。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：无
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.ClassifyCrawler

- 级别：参考
- 文件：`modules/analytics/internal/crawler.go`
- 作用：内存识别已知爬虫

```go
func ClassifyCrawler(ctx context.Context, op OperationContext, in ClassifyCrawlerInput) (ClassifyCrawlerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ClassifyCrawlerInput` | 内存识别已知爬虫；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ClassifyCrawlerResult` | 内存识别已知爬虫成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. UA字符串max1024
2. 大小写不敏感登记GPTBot/ChatGPT-User/ClaudeBot/PerplexityBot/Googlebot/Bingbot
3. 返回family后丢弃UA
4. 真人beacon排除knownCrawler

**错误**

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无UA原文存储
- 授权：permission：anonymous；rule：anonymous ingress内部
- 并发：纯函数；UA不是可靠权限身份，仅统计过滤
- 幂等：同UA规则version同结果
- 依赖：无
- 其他依赖说明：内置规则配置

## analytics.LookupCountry

- 级别：参考
- 文件：`modules/analytics/internal/geoip.go`
- 作用：离线GeoIP国家

```go
func LookupCountry(ctx context.Context, op OperationContext, in LookupCountryInput) (LookupCountryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `LookupCountryInput` | 离线GeoIP国家；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LookupCountryResult` | 离线GeoIP国家成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 可信peer内存查询DB-IP本地mmdb
2. 无库/未知返回ZZ
3. 立即丢弃IP
4. 不调用外部定位API

**错误**

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无IP落盘
- 授权：permission：anonymous；rule：anonymous ingress内部
- 并发：本地read-onlyDB，启动读version
- 幂等：同DB版本/IP同结果
- 依赖：`analytics.trustedPeer`
- 其他依赖说明：离线GeoIP数据库

## analytics.trustedPeer

- 级别：参考
- 文件：`modules/analytics/internal/geoip.go`
- 作用：匿名统计参考支持：trustedPeer

```go
func trustedPeer(peer string) (netip.Addr, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅标准netip.ParseAddr，拒端口及zone；Unmap用于HMAC/离线查询统一表示。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：由已验证本域入口调用；不接受客户端自报信任状态。
- 并发：本域锁/事务与可信窗口
- 幂等：同实际输入保持原事实
- 依赖：无

## analytics.RecordCrawlerAccess

- 级别：约束
- 文件：`modules/analytics/internal/crawler_store.go`
- 作用：AI Markdown访问独立计数

```go
func RecordCrawlerAccess(ctx context.Context, op OperationContext, in RecordCrawlerAccessInput) (RecordCrawlerAccessResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RecordCrawlerAccessInput` | AI Markdown访问独立计数；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecordCrawlerAccessResult` | AI Markdown访问独立计数成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 固定public-site纯服务身份，family必须登记；成功2xx Markdown读取才计数。
2. 按可信ReportingTimeZone文档/语言/family日计数，与真人PV/UV分离；空时区不可用。
3. subject/function/operation复合幂等，同原输入原回执，异原输入CONFLICT；计数/回执/真实水位同事务。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写crawler聚合
- 授权：permission：service；rule：service public-site
- 并发：事务按crawlerBeaconId去重
- 幂等：crawlerBeaconId幂等
- 依赖：`analytics.readReceipt`、`analytics.saveReceipt`、`analytics.advanceWatermark`
- 其他依赖说明：public-site Markdown完成响应

## analytics.readReceipt

- 级别：参考
- 文件：`modules/analytics/internal/crawler_store.go`
- 作用：匿名统计参考支持：readReceipt

```go
func readReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, function, digest string, out any) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. subject/function/operation精确绑定原请求哈希/epoch，返回原持久结果。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：由已验证本域入口调用；不接受客户端自报信任状态。
- 并发：本域锁/事务与可信窗口
- 幂等：同实际输入保持原事实
- 依赖：`analytics.storageError`

## analytics.saveReceipt

- 级别：参考
- 文件：`modules/analytics/internal/crawler_store.go`
- 作用：匿名统计参考支持：saveReceipt

```go
func saveReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, function, digest string, result any) error
```

**输入**

无

**输出**

无

**步骤**

1. 与计数同事务持久原回执，不以外部副作用猜结果。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：由已验证本域入口调用；不接受客户端自报信任状态。
- 并发：本域锁/事务与可信窗口
- 幂等：同实际输入保持原事实
- 依赖：`analytics.storageError`

## analytics.QueryContentGaps

- 级别：约束
- 文件：`modules/analytics/internal/content_gaps.go`
- 作用：热门文档与答不出问题联动

```go
func QueryContentGaps(ctx context.Context, op OperationContext, in QueryContentGapsInput) (QueryContentGapsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `QueryContentGapsInput` | 热门文档与答不出问题联动；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `QueryContentGapsResult` | 热门文档与答不出问题联动成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 入口stats.read已验证；额外即时identity qa.manage核权，ID跳转正文仍在qa资源权限页面。
2. 只完整可信日历日与DST边界；同快照热门文档按实际daily总表汇总，访次为各日去重累加不是跨日独立访客。
3. 同快照返回真实completed/failed/cache/unanswered分子分母及page/qa水位与最近投影时间，0样本不伪报比率。
4. 无答案只opaque引用，超过10000/计数整数预算失败不截断；来源延迟用不同水位明确。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：qa.manage；rule：stats.read+qa.manage资源范围
- 并发：两源固定watermark附更新时间
- 幂等：GET幂等
- 依赖：`analytics.pageInterval`、`analytics.readPageSnapshot`、`identity:checkAuthorization`

## analytics.main

- 级别：约束
- 文件：`modules/analytics/cmd/analytics/main.go`
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
- 依赖：`analytics.BuildApplication`、`analytics.Serve`、`analytics.Shutdown`、`platform.RunService`、`analytics.NewRuntimeClient`

## analytics.BuildApplication

- 级别：约束
- 文件：`modules/analytics/internal/app/wire.go`
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
7. 复用公共LoadServiceIdentity；纯服务严格消费readSiteConfig真实siteId，按epoch打开私有库。ReportingTimeZone只取受信部署，空不假设UTC。只读路由真实接通，其他能力未齐Ready仍失败关闭。
8. model-gateway地址存在时，以apivalidate.NewPayloadValidator(gen.PayloadValidationSpec, AnalyticsUsageStatisticsPageResult)装配UsageGateway.ValidatePage；生成物未包含该登记时失败关闭。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`analytics.FlushBeacons`、`analytics.PurgeBeacons`、`analytics.PurgeAuditHistory`、`analytics.PurgeExports`、`analytics.PublishAggregateEvents`、`analytics.ConsumeEvents`、`analytics.RunWorker`、`jobs:createSchedule`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`site-config:readSiteConfig`、`platform.LoadServiceIdentity`、`analytics.OpenStore`、`analytics.readSiteIdentity`、`analytics.NewApplication`、`analytics.BindReportingTimeZone`、`analytics.RunPageBackground`、`analytics.SchedulePageAggregates`、`jobs:createJob`、`analytics.BackgroundOperation`、`analytics.RecoverExports`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`、`analytics.server.ReadSnapshotArchive`、`jobs:checkLease`、`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:completeJob`、`jobs:failJob`、`platform.NewPayloadValidator`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## analytics.NewRuntimeClient

- 级别：参考
- 文件：`modules/analytics/internal/app/wire.go`
- 作用：本域参考支持：NewRuntimeClient

```go
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用公共身份加载及运行权威客户端，不复制密钥/信任锚解析。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：`platform.LoadServiceIdentity`

## analytics.readSiteIdentity

- 级别：参考
- 文件：`modules/analytics/internal/app/wire.go`
- 作用：本域参考支持：readSiteIdentity

```go
func readSiteIdentity(ctx context.Context, client *gen.SiteConfigClient, epoch string, validate func(json.RawMessage) error) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 纯服务消费生产方严格ReadSiteConfigResult完整schema，仅取真实siteId，不造默认身份。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：`site-config:readSiteConfig`

## analytics.Serve

- 级别：约束
- 文件：`modules/analytics/internal/transport/server.go`
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
- 依赖：`analytics.BuildApplication`、`analytics.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`、`analytics.RunPageBackground`
- 其他依赖说明：本域OpenAPI路由

## analytics.Shutdown

- 级别：约束
- 文件：`modules/analytics/internal/transport/server.go`
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
- 依赖：`analytics.DrainBeacons`、`analytics.Runtime.Close`
- 其他依赖说明：本域barrier安全点函数；本域journal/outbox flush

## analytics.Health

- 级别：约束
- 文件：`modules/analytics/internal/transport/server.go`
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

## analytics.Ready

- 级别：约束
- 文件：`modules/analytics/internal/transport/server.go`
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

## analytics.NewApplication

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：本域参考支持：NewApplication

```go
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtime platform.SiteConfigClient, storage *analytics.Runtime, identity *gen.IdentityClient) (*Application, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整生成路由通过公共严格服务边界核签名/委托/权限/epoch；控制只DP、crawler只public-site，未实现路由明确不可用。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：`analytics.server.QueryAudits`、`analytics.server.ReadAudit`、`analytics.server.QueryPageStatistics`、`analytics.server.AppendAudit`、`analytics.server.IngestTrustedBeacon`、`analytics.server.RecordCrawlerAccess`、`analytics.server.QueryContentGaps`、`analytics.server.QueryUsageStatistics`、`analytics.server.CreateExport`、`analytics.server.CreateAuditExport`、`analytics.server.createExport`、`analytics.server.DownloadStatisticsExport`、`analytics.server.DownloadAuditExport`、`analytics.server.downloadExport`、`analytics.server.ReadSnapshotArchive`、`analytics.WithRuntime`

## analytics.server.QueryAudits

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：受信HTTP转换：QueryAudits

```go
func (s *server) QueryAudits(w http.ResponseWriter, r *http.Request, params gen.QueryAuditsParams)
```

**输入**

无

**输出**

无

**步骤**

1. 生成边界已严格校验；只用可信op绑定actor/epoch与本域句柄，输出实际JSON，no-store。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_CURSOR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：audit.read；rule：audit.read
- 并发：快照读；不跨域查原正文
- 幂等：GET幂等
- 依赖：`analytics.QueryAudits`

## analytics.server.ReadAudit

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：受信HTTP转换：ReadAudit

```go
func (s *server) ReadAudit(w http.ResponseWriter, r *http.Request, auditID string, params gen.ReadAuditParams)
```

**输入**

无

**输出**

无

**步骤**

1. 生成边界已严格校验；只用可信op绑定actor/epoch与本域句柄，输出实际JSON，no-store。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：AUDIT_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：audit.read；rule：audit.read
- 并发：只读
- 幂等：GET幂等
- 依赖：`analytics.ReadAudit`

## analytics.server.QueryPageStatistics

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：受信HTTP转换：QueryPageStatistics

```go
func (s *server) QueryPageStatistics(w http.ResponseWriter, r *http.Request, params gen.QueryPageStatisticsParams)
```

**输入**

无

**输出**

无

**步骤**

1. 生成边界已严格校验；只用可信op绑定actor/epoch与本域句柄，输出实际JSON，no-store。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：stats.read；rule：stats.read
- 并发：SQLite快照读，最多1000聚合行分页
- 幂等：GET幂等
- 依赖：`analytics.QueryPageStatistics`

## analytics.server.AppendAudit

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：固定来源审计HTTP转换

```go
func (s *server) AppendAudit(w http.ResponseWriter, r *http.Request, params gen.AppendAuditParams)
```

**输入**

无

**输出**

无

**步骤**

1. 生成边界已严格校验；只用可信op绑定actor/epoch与本域句柄，输出实际JSON，no-store。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：AUDIT_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：audit.read；rule：audit.read
- 并发：只读
- 幂等：GET幂等
- 依赖：`analytics.AppendAudit`

## analytics.server.IngestTrustedBeacon

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：匿名统计参考支持：server.IngestTrustedBeacon

```go
func (s *server) IngestTrustedBeacon(w http.ResponseWriter, r *http.Request, params gen.IngestTrustedBeaconParams)
```

**输入**

无

**输出**

无

**步骤**

1. 固定portal纯服务边界已鉴权签完整正文；不读XFF/UA头；调用内部IngestBeacon后统一204。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：由已验证本域入口调用；不接受客户端自报信任状态。
- 并发：本域锁/事务与可信窗口
- 幂等：同实际输入保持原事实
- 依赖：`analytics.IngestBeacon`

## analytics.server.RecordCrawlerAccess

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：匿名统计参考支持：server.RecordCrawlerAccess

```go
func (s *server) RecordCrawlerAccess(w http.ResponseWriter, r *http.Request, params gen.RecordCrawlerAccessParams)
```

**输入**

无

**输出**

无

**步骤**

1. 严格入口解析实际公开交付爬虫计数，调用本域持久链并输出真实结果。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：由已验证本域入口调用；不接受客户端自报信任状态。
- 并发：本域锁/事务与可信窗口
- 幂等：同实际输入保持原事实
- 依赖：`analytics.RecordCrawlerAccess`

## analytics.server.QueryContentGaps

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：问答统计参考支持：server.QueryContentGaps

```go
func (s *server) QueryContentGaps(w http.ResponseWriter, r *http.Request, params gen.QueryContentGapsParams)
```

**输入**

无

**输出**

无

**步骤**

1. 严格边界已建stats权限，实际缺口查询额外核qa权限，输出真实安全统计与opaque引用。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：仅已验证入口/consumer调用，不接受客户端自报来源。
- 并发：本域同事务
- 幂等：原事件来源事实绑定
- 依赖：`analytics.QueryContentGaps`
- 其他依赖说明：本域私有SQLite

## analytics.server.QueryUsageStatistics

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：私有导出/保留期支持：server.QueryUsageStatistics

```go
func (s *server) QueryUsageStatistics(w http.ResponseWriter, r *http.Request, params gen.QueryUsageStatisticsParams)
```

**输入**

无

**输出**

无

**步骤**

1. 严格stats.read入口调用已有费用投影查询，返回真实投影水位/对账时间，非实时余额。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.QueryUsageStatistics`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.server.CreateExport

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：私有导出/保留期支持：server.CreateExport

```go
func (s *server) CreateExport(w http.ResponseWriter, r *http.Request, params gen.CreateExportParams)
```

**输入**

无

**输出**

无

**步骤**

1. 固定stats.read入口只pages/usage。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.server.createExport`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.server.CreateAuditExport

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：私有导出/保留期支持：server.CreateAuditExport

```go
func (s *server) CreateAuditExport(w http.ResponseWriter, r *http.Request, params gen.CreateAuditExportParams)
```

**输入**

无

**输出**

无

**步骤**

1. 固定audit.read入口只audits，不增加额外用户流程。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.server.createExport`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.server.createExport

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：私有导出/保留期支持：server.createExport

```go
func (s *server) createExport(w http.ResponseWriter, r *http.Request, audits bool)
```

**输入**

无

**输出**

无

**步骤**

1. 路由决定允许kind，严格输入与当前委托；调用真实原操作journal生成CSV并输出原结果。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.CreateExport`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.server.DownloadStatisticsExport

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：私有导出/保留期支持：server.DownloadStatisticsExport

```go
func (s *server) DownloadStatisticsExport(w http.ResponseWriter, r *http.Request, exportID string, params gen.DownloadStatisticsExportParams)
```

**输入**

无

**输出**

无

**步骤**

1. 固定stats.read下载，仅pages/usage。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.server.downloadExport`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.server.DownloadAuditExport

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：私有导出/保留期支持：server.DownloadAuditExport

```go
func (s *server) DownloadAuditExport(w http.ResponseWriter, r *http.Request, exportID string, params gen.DownloadAuditExportParams)
```

**输入**

无

**输出**

无

**步骤**

1. 固定audit.read下载，仅audits。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.server.downloadExport`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.server.downloadExport

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：私有导出/保留期支持：server.downloadExport

```go
func (s *server) downloadExport(w http.ResponseWriter, r *http.Request, id string, audits bool)
```

**输入**

无

**输出**

无

**步骤**

1. 真实kind匹配固定当前权限路由，OpenExport核原主体/epoch/expiry/hash；no-store/nosniff/固定安全文件名受控流。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.ExportKind`、`analytics.OpenExport`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.server.ReadSnapshotArchive

- 级别：参考
- 文件：`modules/analytics/internal/transport/server.go`
- 作用：新增归档HTTP接口在真实装配前明确失败关闭

```go
func (s *server) ReadSnapshotArchive(w http.ResponseWriter, r *http.Request, snapshotID gen.Identifier, params gen.ReadSnapshotArchiveParams)
```

**输入**

无

**输出**

无

**步骤**

1. 尚无实际owner SnapshotManager装配，统一writeUnavailable，不输出归档、不声明已完成；真实业务ReadSnapshotArchive仍planned。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：进程已closed或服务循环不可用（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：无锁或原子读取lifecycle；不进行外部调用
- 幂等：GET只读重复安全
- 依赖：无

## analytics.mountModuleUI

- 级别：约束
- 文件：`modules/analytics/web/src/main.ts`
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
- 依赖：`analytics.createAnalyticsClient`、`analytics.loadOverview`、`analytics.loadAudit`、`analytics.unmountAnalytics`
- 其他依赖说明：本域已登记UI页面函数

## analytics.unmountAnalytics

- 级别：参考
- 文件：`modules/analytics/web/src/main.ts`
- 作用：统计UI实际支持：unmountAnalytics

```ts
function unmountAnalytics(): void
```

**输入**

无

**输出**

无

**步骤**

1. 统一nice-wiki:unmount取消所有控制器/监听并卸载Vue，清本域挂载实例；不清服务端事实。

**错误**

- 无

- 副作用：仅组件内存或明确下载动作
- 授权：permission：stats.read；rule：菜单仅显示能力；实际HTTP当前权限由服务核验，不在浏览器伪造凭据。
- 并发：组件AbortController取消旧请求，卸载停止监听和下载读取
- 幂等：读取重复安全，导出原operationId重试
- 依赖：无
- 其他依赖说明：portal注入同源ModuleAPIClient

## analytics.RunWorker

- 级别：约束
- 文件：`modules/analytics/internal/worker.go`
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

1. 仅装配本域登记jobTypes与platform.RunWorkerLoopWithResult；hour/day真实不可变输入执行，费用聚合/对账尚缺完整权威协议仍失败关闭，整体不得记完整ready。
2. OperationContext精确取原业务输入/operation/root；OnLeaseRenewed真实续租写镜像，OnCancellationObserved撤同fence。
3. completed/failed仅公共jobs完成/失败/取消真实成功回执计数，未知写回不计。

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用；退避后重试领取（可重试）

- 副作用：HTTP调用jobs租约接口；执行本域登记的任务函数
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个领取循环，并发不超过maxConcurrency；同一任务只由持有有效fence者写回
- 幂等：jobs按jobId与fence去重；执行函数自身按输入引用幂等
- 依赖：`analytics.aggregateJobOperation`、`analytics.recordAggregateLease`、`analytics.cancelAggregateLease`、`analytics.executePageAggregate`、`platform.RunWorkerLoopWithResult`、`analytics.AggregateHourly`、`analytics.AggregateDaily`、`analytics.AggregateUsage`、`analytics.ReconcileUsage`

## analytics.WithRuntime

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：本域参考支持：WithRuntime

```go
func WithRuntime(ctx context.Context, runtime *Runtime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 只在可信调用链注入本域句柄，不从HTTP反序列化。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：无

## analytics.OpenStore

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：本域参考支持：OpenStore

```go
func OpenStore(ctx context.Context, root, siteID, epoch string, key platform.SigningKey) (*Runtime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 本域私有路径平台校验，逐级fsync；SQLite WAL/FULL/FK/busy3000，单连接短事务V1迁移。
2. 未知高版本或非空未版本化库拒绝；绑定真实siteId/epoch，不自己生成站点。inbox/outbox复用平台。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：`analytics.syncDirectory`、`analytics.storageError`

## analytics.Runtime.Close

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：本域参考支持：Runtime.Close

```go
func (rt *Runtime) Close() error
```

**输入**

无

**输出**

无

**步骤**

1. 关闭一次数据库并清内存日盐，不修改权威业务记录。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：`analytics.storageError`

## analytics.runtimeFrom

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：本域参考支持：runtimeFrom

```go
func runtimeFrom(ctx context.Context, op OperationContext) (*Runtime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 取可信注入句柄并检查已打开site/epoch，缺失或旧代失败。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：无

## analytics.storageError

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：本域参考支持：storageError

```go
func storageError(err error) error
```

**输入**

无

**输出**

无

**步骤**

1. 复用平台IO错误归一化，不输出私有路径。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：无

## analytics.syncDirectory

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：本域参考支持：syncDirectory

```go
func syncDirectory(path, root string) error
```

**输入**

无

**输出**

无

**步骤**

1. 逐级目录fsync至可信dataRoot。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：可信入口已建op；仅执行本域site/epoch/私有存储约束。
- 并发：纯函数
- 幂等：同range同结果
- 依赖：`analytics.storageError`

## analytics.beginWrite

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：审计/持久参考支持：beginWrite

```go
func beginWrite(ctx context.Context, op OperationContext) (context.Context, *Runtime, *sql.Tx, context.CancelFunc, error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用platform.NewWriteContext与本域site/epoch，缺窗口失败；不在本域再建屏障。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service鉴权生产者
- 并发：纯函数，不通过日志修复非法payload
- 幂等：同audit同结果
- 依赖：`analytics.runtimeFrom`、`analytics.storageError`
- 其他依赖说明：根contracts/errors.json

## analytics.advanceWatermark

- 级别：参考
- 文件：`modules/analytics/internal/store.go`
- 作用：审计/持久参考支持：advanceWatermark

```go
func advanceWatermark(ctx context.Context, tx *sql.Tx, source string) (int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同业务事务前进真实全局修订和对应来源水位，int64上限失败。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service鉴权生产者
- 并发：纯函数，不通过日志修复非法payload
- 幂等：同audit同结果
- 依赖：`analytics.storageError`
- 其他依赖说明：根contracts/errors.json

## analytics.normalizeAuditFact

- 级别：参考
- 文件：`modules/analytics/internal/audit_events.go`
- 作用：审计/持久参考支持：normalizeAuditFact

```go
func normalizeAuditFact(ctx context.Context, op OperationContext, origin string, payload AuditRecordedPayload, historical bool) (AuditRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 共用字段/来源/元数据校验，规范原时间和旧成功别名，不伪actor/epoch。

**错误**

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：service鉴权生产者
- 并发：纯函数，不通过日志修复非法payload
- 幂等：同audit同结果
- 依赖：`analytics.ValidateAudit`、`analytics.RedactAuditMetadata`
- 其他依赖说明：根contracts/errors.json

## analytics.ConsumeAuditEvent

- 级别：约束
- 文件：`modules/analytics/internal/audit_events.go`
- 作用：消费原生产者已提交审计事实

```go
func ConsumeAuditEvent(ctx context.Context, op OperationContext, input EventDelivery) (EventHandled, error)
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

1. 公共订阅边界一次校验AuditRecordedPayload；Envelope类型为producer.audit-recorded/v1、subjectId=auditId、revision=1。
2. 核payload site/epoch与可信本域一致；origin显式取已验证envelope.Producer，不重写op.CallerService。
3. 共用normalizeAuditFact/appendTrustedAudit，ClaimInbox与追加同事务；失败不确认，重复只applied=false。

**错误**

- `VERSION_MISMATCH`：旧代事件（不可重试）

- 副作用：写问答日计数
- 授权：permission：service；rule：仅由本域ConsumeEvents调用
- 并发：日计数行CAS
- 幂等：eventId去重
- 依赖：`analytics.runtimeFrom`、`analytics.normalizeAuditFact`、`analytics.appendTrustedAudit`

## analytics.HashDayToken

- 级别：参考
- 文件：`modules/analytics/internal/beacon_privacy.go`
- 作用：IP+User-Agent+当日随机盐访客匿名hash

```go
func HashDayToken(ctx context.Context, op OperationContext, in HashDayTokenInput) (HashDayTokenResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HashDayTokenInput` | IP+User-Agent+当日随机盐访客匿名hash；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HashDayTokenResult` | IP+User-Agent+当日随机盐访客匿名hash成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 可信portal签名正文交付规范IP，不读XFF；日界线只取可信ReportingTimeZone，空配置不可用。
2. 当前代次固定时区；HMAC SHA256每日独立32字节随机盐与canonical[siteId,dataEpoch,day,IP,规范UA]；UA只内存小写折叠空白。
3. control/visitor-YYYY-MM-DD.key为0600固定32字节盐，写fsync和目录fsync；残缺旧盐拒绝，不重造；快照不包含control。
4. 内存跨日清旧盐，旧磁盘日盐由保留任务24h删除；原地址/UA不持久不日志。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：PRIVACY_KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous ingress内部
- 并发：salt每日轮换；不跨日关联
- 幂等：same day/token同hash
- 依赖：`analytics.loadDaySalt`、`analytics.trustedPeer`、`analytics.PruneDaySalts`
- 其他依赖说明：本地secret日盐容器，入口可信peer中间件

## analytics.BindReportingTimeZone

- 级别：参考
- 文件：`modules/analytics/internal/beacon_privacy.go`
- 作用：匿名统计参考支持：BindReportingTimeZone

```go
func BindReportingTimeZone(ctx context.Context, rt *Runtime, name string, view platform.RuntimeView) error
```

**输入**

无

**输出**

无

**步骤**

1. 当前代次持久固定可信IANA日界线；首次只在running且无无法证明时区的旧匿名明细时绑定，维护或不同已绑定时区失败。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：由已验证本域入口调用；不接受客户端自报信任状态。
- 并发：本域锁/事务与可信窗口
- 幂等：同实际输入保持原事实
- 依赖：`analytics.advanceWatermark`

## analytics.loadDaySalt

- 级别：参考
- 文件：`modules/analytics/internal/beacon_privacy.go`
- 作用：匿名统计参考支持：loadDaySalt

```go
func loadDaySalt(root, day string) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 固定已验证日期私有control引用，随机32字节，O_EXCL/0600/文件目录fsync；残缺旧盐不重建。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：由已验证本域入口调用；不接受客户端自报信任状态。
- 并发：本域锁/事务与可信窗口
- 幂等：同实际输入保持原事实
- 依赖：`analytics.syncDirectory`

## analytics.SchedulePageAggregates

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：SchedulePageAggregates

```go
func SchedulePageAggregates(ctx context.Context, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 每分钟有界扫描真实变动已封闭period及当前应计划空桶；小时封闭5分钟/本地日10分钟后计划，迟到访问新水位产生新不可变输入。
2. 只保存本域业务输入/提交意图，不领取排队；jobs是唯一调度与租约权威。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.ensureAggregateIntent`、`analytics.submitAggregateIntents`、`analytics.admitAggregateScheduling`
- 其他依赖说明：私有SQLite

## analytics.ensureAggregateIntent

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：ensureAggregateIntent

```go
func ensureAggregateIntent(ctx context.Context, op OperationContext, kind, key, period, zone string) error
```

**输入**

无

**输出**

无

**步骤**

1. 同事务固定真实period/sourceWatermark/schemaVersion1/site/epoch；canonical字节构成真实ObjectRef；同输入哈希确定ID和operation/root，原输入不修改。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.beginWrite`、`analytics.advanceWatermark`
- 其他依赖说明：私有SQLite

## analytics.submitAggregateIntents

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：submitAggregateIntents

```go
func submitAggregateIntents(ctx context.Context, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 有界100原意图经jobs:createJob，固定原dedupeKey/operation/root；未知响应只用原key续接，严格生产响应校验，真实原回执CAS存入。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`jobs:createJob`、`analytics.backgroundOperation`、`analytics.beginWrite`、`analytics.admitAggregateScheduling`
- 其他依赖说明：私有SQLite

## analytics.backgroundOperation

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：backgroundOperation

```go
func backgroundOperation(ctx context.Context, rt *Runtime, operation, root string, duration time.Duration) (context.Context, context.CancelFunc, OperationContext)
```

**输入**

无

**输出**

无

**步骤**

1. 清用户proof，本站纯服务，精确原操作与根，每次IO新截止；免费控制costClass空，不创造free枚举。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：无
- 其他依赖说明：私有SQLite

## analytics.aggregateJobOperation

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：aggregateJobOperation

```go
func aggregateJobOperation(ctx context.Context, rt *Runtime, job platform.ClaimedJob) (OperationContext, error)
```

**输入**

无

**输出**

无

**步骤**

1. 领取必须精确匹配真实本域不可变引用/type/epoch/原operation/root；未知提交回执可以真实claim绑定但不伪创建回执。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.backgroundOperation`
- 其他依赖说明：私有SQLite

## analytics.recordAggregateLease

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：recordAggregateLease

```go
func recordAggregateLease(ctx context.Context, op OperationContext, job platform.ClaimedJob, info platform.HeartbeatInfo) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅公共OnLeaseRenewed真实回执更新同job/input/op/root/worker/fence/epoch/attempt/expiry；旧fence拒绝，新真实fence可以取代旧已撤执行权。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.beginWrite`
- 其他依赖说明：私有SQLite

## analytics.cancelAggregateLease

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：cancelAggregateLease

```go
func cancelAggregateLease(ctx context.Context, op OperationContext, job platform.ClaimedJob) error
```

**输入**

无

**输出**

无

**步骤**

1. 只撤销同job/worker/fence/epoch；不把safePoint当用户取消，不重调CheckLease。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.beginWrite`
- 其他依赖说明：私有SQLite

## analytics.checkAggregateLease

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：checkAggregateLease

```go
func checkAggregateLease(ctx context.Context, execution *platform.JobExecutionInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 不可逆事务前锁外调用平台真实CheckLease，缺失失败关闭。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：无
- 其他依赖说明：私有SQLite

## analytics.verifyAggregateLease

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：verifyAggregateLease

```go
func verifyAggregateLease(ctx context.Context, tx *sql.Tx, op OperationContext, execution *platform.JobExecutionInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 事务内提交前精确CAS本域持久真实租约镜像/expiry，取消/旧fence/epoch拒绝。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：无
- 其他依赖说明：私有SQLite

## analytics.executePageAggregate

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：统计任务参考支持：executePageAggregate

```go
func executePageAggregate(ctx context.Context, rt *Runtime, execution platform.JobExecutionInput) (platform.JobExecutionResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取/核对原不可变字节hash/长度/本站schema/代次，按登记hour/day实际执行；不冒费用任务完成。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.AggregateHourly`、`analytics.AggregateDaily`
- 其他依赖说明：私有SQLite

## analytics.admitAggregateScheduling

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：新统计任务提交前读取真实运行权威

```go
func admitAggregateScheduling(ctx context.Context, rt *Runtime) error
```

**输入**

无

**输出**

无

**步骤**

1. 每次外部提交前真实runtime确认当前epoch/running/非maintenance；未知或屏障关闭不发新任务。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：无
- 其他依赖说明：私有SQLite

## analytics.BackgroundOperation

- 级别：参考
- 文件：`modules/analytics/internal/aggregate_jobs.go`
- 作用：私有导出/保留期支持：BackgroundOperation

```go
func BackgroundOperation(ctx context.Context, rt *Runtime, duration time.Duration) (context.Context, context.CancelFunc, OperationContext)
```

**输入**

无

**输出**

无

**步骤**

1. 受信启动只装配本域纯服务journal恢复上下文，不继承历史用户证明。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.backgroundOperation`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.RunPageBackground

- 级别：参考
- 文件：`modules/analytics/internal/background.go`
- 作用：统计任务参考支持：RunPageBackground

```go
func RunPageBackground(ctx context.Context, rt *Runtime) error
```

**输入**

无

**输出**

无

**步骤**

1. 本域每5秒flush/每分钟计划及公共worker并发装配，取消时等循环退出；快照safe还需公共Manager private cleanup集成，未假就绪。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.SchedulePageAggregates`、`analytics.FlushBeacons`、`analytics.RunWorker`、`analytics.ConsumeEvents`、`analytics.PublishAggregateEvents`、`analytics.RunRetentionBackground`、`analytics.runAnalyticsPeriodic`、`analytics.launchPageLoop`
- 其他依赖说明：私有SQLite

## analytics.DrainBeacons

- 级别：参考
- 文件：`modules/analytics/internal/background.go`
- 作用：统计任务参考支持：DrainBeacons

```go
func DrainBeacons(ctx context.Context, rt *Runtime) error
```

**输入**

无

**输出**

无

**步骤**

1. 停止新请求/后台后有界批次排空队列和漏数；仍必须真实平台写窗口，safe后拒普通业务写。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`analytics.FlushBeacons`、`analytics.backgroundOperation`
- 其他依赖说明：私有SQLite

## analytics.RunRetentionBackground

- 级别：参考
- 文件：`modules/analytics/internal/background.go`
- 作用：私有导出/保留期支持：RunRetentionBackground

```go
func RunRetentionBackground(ctx context.Context, rt *Runtime) error
```

**输入**

无

**输出**

无

**步骤**

1. 每分钟有限50批过30天明细续清、旧盐擦除；每小时过期导出；每日180天审计前缀，满10000继续下轮，不误称每次500能跟上入口流量。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.PurgeBeacons`、`analytics.PurgeAuditHistory`、`analytics.PurgeExports`、`analytics.PruneDaySalts`、`analytics.backgroundOperation`、`analytics.runAnalyticsPeriodic`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.runAnalyticsPeriodic

- 级别：参考
- 文件：`modules/analytics/internal/background.go`
- 作用：装配本域可信操作到公共周期机制

```go
func runAnalyticsPeriodic(ctx context.Context, rt *Runtime, millis int, run func(context.Context, OperationContext) error) error
```

**输入**

无

**输出**

无

**步骤**

1. 复用lifecycle.RunRecoveryLoop的轮询/运行权威/安全窗口，域只创建每次回调可信op；不复制ticker/重试机制。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：`platform.RunRecoveryLoop`、`analytics.backgroundOperation`
- 其他依赖说明：私有SQLite

## analytics.launchPageLoop

- 级别：参考
- 文件：`modules/analytics/internal/background.go`
- 作用：本域后台回调生命周期启动装配

```go
func launchPageLoop(run func(context.Context) error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅在RunPageBackground闭包启动已装配公共循环，首个明确失败取消同组并等待，错误不吞成成功；不负责领取/租约/轮询。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：按职责更新本域私有元数据；所有任务领取仅jobs
- 授权：permission：service；rule：仅本域可信后台装配，不接受HTTP自报租约。
- 并发：事务精确epoch/fence/CAS；网络租约检查在事务外
- 幂等：不可变任务输入哈希/原回执绑定
- 依赖：无
- 其他依赖说明：私有SQLite

## analytics.ConsumeEvents

- 级别：约束
- 文件：`modules/analytics/internal/consumer.go`
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

1. 固定consumerId analytics，平台RunConsumerWithResult接真实jobs订阅/确认，所有登记QA/费用事件保留；每次IO新可信site/epoch/op，不复用proof。
2. 生产schema仅公共订阅边界一次验证；QA转ProjectQaTurn，重复/旧修订明确ErrEventSkipped后实际确认。
3. 费用缺完整权威安全统计页时不确认，不能填0或猜账项修订；审计producer registry尚未装配，不计完整事件链ready。
4. 退出返回平台真实Acknowledged/Skipped/Failed，失败未确认等待jobs重投。

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs订阅或确认不可用；重连后续传（可重试）
- `VALIDATION`：事件信封或载荷不符合登记schema；隔离并确认（不可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash；隔离报警（不可重试）

- 副作用：HTTP长连接jobs:subscribeEvents；调用本域登记的事件处理函数；HTTP调用jobs:acknowledgeEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个订阅循环；同subject串行，不同subject最多maxInFlight并发；事务与CAS由处理函数负责
- 幂等：jobs至少一次投递；本域inbox按eventId+payloadHash去重，重复投递只重新确认
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`analytics.ProjectQaTurn`、`platform.RunConsumerWithResult`、`analytics.ConsumeAuditEvent`

## analytics.CreateExport

- 级别：约束
- 文件：`modules/analytics/internal/exports.go`
- 作用：受权限保护统计导出

```go
func CreateExport(ctx context.Context, op OperationContext, in CreateExportInput) (CreateExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CreateExportInput` | 受权限保护统计导出；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateExportResult` | 受权限保护统计导出成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 原POST statistics:export严格pages|usage且stats.read；专用POST audit-exports严格audits且audit.read，边界即时当前权限，域不重用过期proof。
2. 固定当前SQLite读快照和对应来源水位，最多100000行/32MiB；固定列白名单不含正文/IP/凭据，公式前缀防护和CSV标准转义。
3. preparing原操作输入hash/actor/site/epoch/原24h期限先持久；完整part fsync与目录fsync后prepared存真实ref/result，再rename目录fsync后ready。prepared后重试不查询重算原字节。
4. 重试同操作同输入原结果，异输入CONFLICT，过期GONE要求新操作；保留过期幂等墓碑，不因删文件再授24h。未知旧export记录拒绝静默迁移。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：EXPORT_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写私有导出文件/run
- 授权：permission：stats.read；rule：stats.read或audit.read
- 并发：内存最多1000行块写；文件temp fsync rename再DB记录
- 幂等：operationId/filterHash/actor幂等
- 依赖：`analytics.ValidateDateRange`、`analytics.pageInterval`、`analytics.streamExportRows`、`analytics.installPreparedExport`、`analytics.beginWrite`、`analytics.CheckPrivacyFields`

## analytics.writeCSVRecord

- 级别：参考
- 文件：`modules/analytics/internal/exports.go`
- 作用：私有导出/保留期支持：writeCSVRecord

```go
func writeCSVRecord(ctx context.Context, op OperationContext, writer *exportWriter, cells []string) error
```

**输入**

无

**输出**

无

**步骤**

1. 固定安全列逐单元公式防护/标准CSV转义，文件IO检查取消；累计字节≤32MiB，不截断成功。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.ValidateCSVCell`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.streamExportRows

- 级别：参考
- 文件：`modules/analytics/internal/exports.go`
- 作用：私有导出/保留期支持：streamExportRows

```go
func streamExportRows(ctx context.Context, tx *sql.Tx, op OperationContext, kind string, begin, end time.Time, writer *exportWriter) (int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 固定SQL读快照/来源真实水位，允许列闭集不带元数据原文/正文/IP/凭据；最多100000行，未知费用不导出成0。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.writeCSVRecord`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.installPreparedExport

- 级别：参考
- 文件：`modules/analytics/internal/exports.go`
- 作用：私有导出/保留期支持：installPreparedExport

```go
func installPreparedExport(ctx context.Context, op OperationContext, rt *Runtime, id string) error
```

**输入**

无

**输出**

无

**步骤**

1. 只装已prepared完整字节与真实ObjectRef；核part或既有final完整hash，rename/目录fsync后才ready；不查询重算prepared内容。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.verifyExportFile`、`analytics.beginWrite`、`analytics.syncDirectory`、`analytics.exportObjectNameValid`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.verifyExportFile

- 级别：参考
- 文件：`modules/analytics/internal/exports.go`
- 作用：私有导出/保留期支持：verifyExportFile

```go
func verifyExportFile(path string, ref platform.ObjectRef) (*os.File, error)
```

**输入**

无

**输出**

无

**步骤**

1. 私有普通文件/精确ObjectRef长度/完整SHA256/32MiB上限，核后seek0供受控流，不接任意请求路径。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：无
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.OpenExport

- 级别：约束
- 文件：`modules/analytics/internal/exports.go`
- 作用：私有导出/保留期支持：OpenExport

```go
func OpenExport(ctx context.Context, op OperationContext, id, kind string) (*os.File, platform.ObjectRef, error)
```

**输入**

无

**输出**

无

**步骤**

1. 固定stats.read统计路由或audit.read审计路由已即时核权；存储真实kind必须与路由相容，不以未知默认pages。
2. 当前本域site/epoch、原actor、固定24h未过期、ready状态及精确引用ID；完整长度/hash核对后才流出，不能跨主体/任意路径。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：stats.read；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.verifyExportFile`、`analytics.exportObjectNameValid`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.QueryUsageStatistics

- 级别：约束
- 文件：`modules/analytics/internal/usage_queries.go`
- 作用：费用汇总查询

```go
func QueryUsageStatistics(ctx context.Context, op OperationContext, in QueryUsageStatisticsInput) (QueryUsageStatisticsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `QueryUsageStatisticsInput` | 费用汇总查询；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `QueryUsageStatisticsResult` | 费用汇总查询成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 实际V2已入账和未入账负债完整上界尚未原子激活为usage-v2-complete时，DEPENDENCY_UNAVAILABLE；禁止以空投影0或已知部分冒充全站总额。消费者未完成，当前函数保持partial blocked。
2. 只读本域已持久的账项投影、真实usage水位与最后完整对账时间；不称全站实时余额。
3. 过滤左闭右开窗/class，同SQL快照最多10000条，USD整数精确累加且不超Money安全整数；未知费用/币种不造0或部分和。
4. gateway不可用仍可读已有投影；无已对账时间明确空，不伪造新鲜度。

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：stats.read；rule：stats.read
- 并发：只读快照，不作允许模型调用的实时决策
- 幂等：GET幂等
- 依赖：`analytics.ValidateDateRange`、`analytics.runtimeFrom`

## analytics.PurgeExports

- 级别：参考
- 文件：`modules/analytics/internal/export_recovery.go`
- 作用：清理导出临时对象

```go
func PurgeExports(ctx context.Context, op OperationContext, in PurgeExportsInput) (PurgeExportsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PurgeExportsInput` | 清理导出临时对象；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PurgeExportsResult` | 清理导出临时对象成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只清真实过期原导出，私有已登记固定对象名，文件删除/目录fsync后标expired并清派生引用。
2. 保留actor/epoch/原操作输入hash/原到期墓碑，旧操作GONE不得重新授24h；每批500有界。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：删导出文件/状态
- 授权：permission：service；rule：service worker
- 并发：文件/DB非单事务，重启扫描可重复补偿
- 幂等：exportId幂等
- 依赖：`analytics.exportObjectNameValid`、`analytics.syncDirectory`、`analytics.beginWrite`
- 其他依赖说明：私有对象目录

## analytics.RecoverExports

- 级别：参考
- 文件：`modules/analytics/internal/export_recovery.go`
- 作用：私有导出/保留期支持：RecoverExports

```go
func RecoverExports(ctx context.Context, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 恢复真实prepared对象，preparing仅清已登记未封存part；ready核完整hash，未知旧记录/目录对象拒绝，不假就绪。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：`analytics.installPreparedExport`、`analytics.verifyExportFile`、`analytics.exportObjectNameValid`
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.ExportKind

- 级别：参考
- 文件：`modules/analytics/internal/export_recovery.go`
- 作用：私有导出/保留期支持：ExportKind

```go
func ExportKind(ctx context.Context, rt *Runtime, id string) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读真实存储kind供固定权限路由匹配，不把未知/缺失当pages。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：无
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.exportObjectNameValid

- 级别：参考
- 文件：`modules/analytics/internal/export_recovery.go`
- 作用：私有导出/保留期支持：exportObjectNameValid

```go
func exportObjectNameValid(id string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 仅服务生成export.+完整64位hash命名，不把SQL损坏ID拼成路径。

**错误**

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：仅按真实journal/当前epoch执行本域私有状态
- 授权：permission：service；rule：由精确权限路由/可信本域后台调用；下载必须同原主体/epoch/站点/类型且当前权限。
- 并发：本域exportMu或短SQLite事务；文件与SQLite分别持久journal
- 幂等：原操作/输入hash/不可变文件；过期不重造授权
- 依赖：无
- 其他依赖说明：本域私有SQLite/exports对象

## analytics.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/analytics/internal/snapshot_archive.go`
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

## analytics.parseUsageEventReference

- 级别：参考
- 文件：`modules/analytics/internal/usage_events.go`
- 作用：核费用事件生产者、类型、代次与证明种类引用

```go
func parseUsageEventReference(event platform.EventEnvelope, epoch string) (usageEventReference, error)
```

**输入**

无

**输出**

无

**步骤**

1. settled必须有entryId；reconciled charged必须有entryId，not_charged不得有；subject须等于invocationId。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域可信费用消费链；不接受浏览器构造。
- 并发：无共享可变状态。
- 幂等：相同已核验输入产生相同结果。
- 依赖：无

## analytics.validateUsageEventReference

- 级别：参考
- 文件：`modules/analytics/internal/usage_events.go`
- 作用：把事件引用绑定到完整权威V2页原事实

```go
func validateUsageEventReference(ref usageEventReference, siteID string, changes []UsageLedgerChange, observations []UsageObservation) error
```

**输入**

无

**输出**

无

**步骤**

1. 账项入口必须匹配entry/invocation/attempt/root/class/site/sourceEpoch；not_charged只匹配原attempt观察。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域可信费用消费链；不接受浏览器构造。
- 并发：无共享可变状态。
- 幂等：相同已核验输入产生相同结果。
- 依赖：无
