# analytics 函数合同

共44函数，与implementation.json一一对应。这里包含所有已设计手写函数及UI交互；未来新增私有helper必须同时登记。输入和结果字段完整schema在types.json，无null默认业务对象。

## M14-F001 ValidateBeacon

文件 `modules/analytics/internal/beacon_validation.go`。严格匿名输入校验

```go
func ValidateBeacon(ctx context.Context, op OperationContext, in ValidateBeaconInput) (ValidateBeaconResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidateBeaconInput` — 严格匿名输入校验；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidateBeaconResult` — 严格匿名输入校验成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`beaconId` beaconId；UTF-8；不得为 null, `documentId` documentId；UTF-8；不得为 null, `locale` locale；UTF-8；不得为 null, `eventType` eventType；UTF-8；不得为 null, `status` status 非负 int64；不用浮点数, `elapsedMs` elapsedMs 非负 int64；不用浮点数, `referrerDomain` referrerDomain；UTF-8；不得为 null

成功结果字段：`valid` valid 的明确真假结果，不可 null

步骤：

1. body2048 allowlist
2. page-view首次加载sendBeacon一次，navigation-click独立
3. 来源仅解析合法域名，不含path/query
4. 延迟0..60000
5. 已知crawler由服务端UA内存过滤
6. 没有统计cookie

错误：

- `VALIDATION` / HTTP 400：INVALID_BEACON：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数；拒未知message/query/referrer/IP字段。

幂等：same beacon同结果。

依赖：无。

## M14-F002 BucketLatency

文件 `modules/analytics/internal/buckets.go`。固定时延桶

```go
func BucketLatency(ctx context.Context, op OperationContext, in BucketLatencyInput) (BucketLatencyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BucketLatencyInput` — 固定时延桶；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BucketLatencyResult` — 固定时延桶成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`elapsedMs` elapsedMs 非负 int64；不用浮点数

成功结果字段：`bucket` bucket；UTF-8；不得为 null

步骤：

1. 0..99/100..499/500..999/1000..2999/3000+
2. 超60000拒绝

错误：

- `VALIDATION` / HTTP 400：INVALID_LATENCY：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：同ms同bucket。

依赖：无。

## M14-F003 HashDayToken

文件 `modules/analytics/internal/privacy.go`。IP+User-Agent+当日随机盐访客匿名hash

```go
func HashDayToken(ctx context.Context, op OperationContext, in HashDayTokenInput) (HashDayTokenResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HashDayTokenInput` — IP+User-Agent+当日随机盐访客匿名hash；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HashDayTokenResult` — IP+User-Agent+当日随机盐访客匿名hash成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`peerAddress` peerAddress；UTF-8；不得为 null, `userAgent` userAgent；UTF-8；不得为 null, `day` day；UTF-8；不得为 null, `siteId` siteId；UTF-8；不得为 null

成功结果字段：`dayHash` dayHash；UTF-8；不得为 null

步骤：

1. 只信可信proxy解析peer
2. User-Agent仅内存规范化
3. HMAC每日随机独立salt/IP/UA/site
4. 只存daily hash，不存原IP/UA
5. 日盐每天更换第二天无法关联

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：PRIVACY_KEY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous ingress内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：salt每日轮换；不跨日关联。

幂等：same day/token同hash。

依赖：本地secret日盐容器，入口可信peer中间件。

## M14-F011 RedactAuditMetadata

文件 `modules/analytics/internal/privacy.go`。输出允许元数据

```go
func RedactAuditMetadata(ctx context.Context, op OperationContext, in RedactAuditMetadataInput) (RedactAuditMetadataResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RedactAuditMetadataInput` — 输出允许元数据；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RedactAuditMetadataResult` — 输出允许元数据成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`action` action；UTF-8；不得为 null, `metadata` metadata；UTF-8；不得为 null

成功结果字段：`safeMetadata` safeMetadata；UTF-8；不得为 null

步骤：

1. 按action白名单仅字段revision/fromState/toState/count
2. 未知字段直接拒绝而非保留
3. 不截取正文摘要

错误：

- `FORBIDDEN` / HTTP 403：AUDIT_FIELD_FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "service内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数，≤4096bytes。

幂等：同metadata白名单同结果。

依赖：审计动作登记表。

## M14-F029 CheckPrivacyFields

文件 `modules/analytics/internal/privacy.go`。验证日志与导出字段

```go
func CheckPrivacyFields(ctx context.Context, op OperationContext, in CheckPrivacyFieldsInput) (CheckPrivacyFieldsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: CheckPrivacyFieldsInput` — 验证日志与导出字段；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: CheckPrivacyFieldsResult` — 验证日志与导出字段成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`fieldNames` 有序列表；空列表表示没有可用项

成功结果字段：`allowed` allowed 的明确真假结果，不可 null, `forbidden` 有序列表；空列表表示没有可用项

步骤：

1. 仅登记allowlist
2. 禁止ip/peer/question/prompt/apiKey/token/referrer/query
3. 输出字段名不输出值

错误：

- `FORBIDDEN` / HTTP 403：PRIVACY_FIELD_FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：同names同result。

依赖：无。

## M14-F004 IngestBeacon

文件 `modules/analytics/internal/http_beacons.go`。匿名HTTP接收

```go
func IngestBeacon(ctx context.Context, op OperationContext, in IngestBeaconInput) (IngestBeaconResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: IngestBeaconInput` — 匿名HTTP接收；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: IngestBeaconResult` — 匿名HTTP接收成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`beaconId` beaconId；UTF-8；不得为 null, `documentId` documentId；UTF-8；不得为 null, `locale` locale；UTF-8；不得为 null, `eventType` eventType；UTF-8；不得为 null, `status` status 非负 int64；不用浮点数, `elapsedMs` elapsedMs 非负 int64；不用浮点数, `referrerDomain` referrerDomain；UTF-8；不得为 null, `peerAddress` peerAddress；UTF-8；不得为 null, `userAgent` userAgent；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`accepted` accepted 的明确真假结果，不可 null

步骤：

1. JS sendBeacon每page加载一次
2. 验证字段
3. 内存filter已知UA crawler
4. trusted peer+UA日盐hash
5. 本地GeoIP国家
6. 来源仅domain
7. 队列满记漏数后204
8. 原IP/UA不持久不log

错误：

- `VALIDATION` / HTTP 400：INVALID_BEACON：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：仅写有界内存队列。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：4096队列；每site3000/min入口限制；不影响页面返回。

幂等：beaconId去重在flush落盘。

依赖：ValidateBeacon、HashDayToken、ClassifyCrawler、LookupCountry。

## M14-F005 FlushBeacons

文件 `modules/analytics/internal/beacon_store.go`。批量持久匿名条目

```go
func FlushBeacons(ctx context.Context, op OperationContext, in FlushBeaconsInput) (FlushBeaconsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: FlushBeaconsInput` — 批量持久匿名条目；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: FlushBeaconsResult` — 批量持久匿名条目成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`batchSize` batchSize 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`persisted` persisted 非负 int64；不用浮点数, `dropped` dropped 非负 int64；不用浮点数

步骤：

1. 每5秒取≤500
2. 事务insert ignore beaconId
3. 每条只保留分桶允许字段
4. 失败保留队列最多5秒然后漏数计数

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写匿名inbox。

授权：{"permission": "service", "rule": "service worker", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：单flush worker lease；不记录丢弃payload。

幂等：beaconId唯一幂等。

依赖：私有SQLite。

## M14-F006 AggregateHourly

文件 `modules/analytics/internal/aggregation.go`。精确重算封闭小时桶

```go
func AggregateHourly(ctx context.Context, op OperationContext, in AggregateHourlyInput) (AggregateHourlyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: AggregateHourlyInput` — 精确重算封闭小时桶；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: AggregateHourlyResult` — 精确重算封闭小时桶成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`hour` RFC3339 UTC 时间；不含 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`revision` revision 非负 int64；不用浮点数, `rows` 有序列表；空列表表示没有可用项

步骤：

1. 选择hour范围当前inbox
2. 按document/locale group count并dayHash去重
3. 事务替换该hour派生聚合及watermark

错误：

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：INVALID_HOUR：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写聚合/watermark/outbox。

授权：{"permission": "service", "rule": "service worker", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：关闭小时后保留24h允许补到；事务replace不会累计重复。

幂等：hour/sourceWatermark幂等。

依赖：私有SQLite。

## M14-F007 AggregateDaily

文件 `modules/analytics/internal/aggregation.go`。按站点日聚合

```go
func AggregateDaily(ctx context.Context, op OperationContext, in AggregateDailyInput) (AggregateDailyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: AggregateDailyInput` — 按站点日聚合；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: AggregateDailyResult` — 按站点日聚合成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`day` day；UTF-8；不得为 null, `timezone` timezone；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`revision` revision 非负 int64；不用浮点数

步骤：

1. 按站点IANA时区日边界
2. 明细同dailyvisitorHash distinct得UV
3. group views/UV/热门文档语言/sourceDomain/country/语言分布
4. AI Markdown crawler计数独立不混真人
5. 日汇总长期保存

错误：

- `VALIDATION` / HTTP 400：INVALID_TIMEZONE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：AGGREGATE_INCOMPLETE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写day派生。

授权：{"permission": "service", "rule": "service worker", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：日分界与DST用IANA时区固定配置revision。

幂等：day/timezone/sourceWatermark幂等。

依赖：AggregateHourly。

## M14-F008 ValidateDateRange

文件 `modules/analytics/internal/query_validation.go`。规范查询范围

```go
func ValidateDateRange(ctx context.Context, op OperationContext, in ValidateDateRangeInput) (ValidateDateRangeResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidateDateRangeInput` — 规范查询范围；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidateDateRangeResult` — 规范查询范围成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`from` RFC3339 UTC 时间；不含 null, `to` RFC3339 UTC 时间；不含 null, `maxDays` maxDays 非负 int64；不用浮点数

成功结果字段：`fromUTC` RFC3339 UTC 时间；不含 null, `toUTC` RFC3339 UTC 时间；不含 null

步骤：

1. RFC3339含zone
2. from<to
3. 最大366日
4. 按[begin,end)且默认7日

错误：

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "查询内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：同range同结果。

依赖：无。

## M14-F009 QueryPageStatistics

文件 `modules/analytics/internal/queries.go`。访问汇总查询

```go
func QueryPageStatistics(ctx context.Context, op OperationContext, in QueryPageStatisticsInput) (QueryPageStatisticsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: QueryPageStatisticsInput` — 访问汇总查询；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: QueryPageStatisticsResult` — 访问汇总查询成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`from` RFC3339 UTC 时间；不含 null, `to` RFC3339 UTC 时间；不含 null, `locale` locale；UTF-8；不得为 null, `documentId` documentId；UTF-8；不得为 null, `actor` 公共 AuthContext 合同，遵守其字段和空值规则, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`rows` 有序列表；空列表表示没有可用项, `watermark` watermark 非负 int64；不用浮点数, `dropCount` dropCount 非负 int64；不用浮点数

步骤：

1. check stats.read
2. validate range
3. 只读派生小时/日聚合
4. 附漏数与去重访次解释

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "stats.read", "rule": "stats.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：SQLite快照读，最多1000聚合行分页。

幂等：GET幂等。

依赖：identity.checkAuthorization、ValidateDateRange。

## M14-F014 QueryAudits

文件 `modules/analytics/internal/queries.go`。审计分页

```go
func QueryAudits(ctx context.Context, op OperationContext, in QueryAuditsInput) (QueryAuditsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: QueryAuditsInput` — 审计分页；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: QueryAuditsResult` — 审计分页成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`cursor` cursor；UTF-8；不得为 null, `limit` limit 非负 int64；不用浮点数, `action` action；UTF-8；不得为 null, `actor` 公共 AuthContext 合同，遵守其字段和空值规则, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`auditIds` 有序列表；空列表表示没有可用项, `nextCursor` nextCursor；UTF-8；不得为 null

步骤：

1. 验audit.read
2. 签名cursor绑定filters/epoch
3. limit20 max100按seq稳定分页

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：INVALID_CURSOR：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "audit.read", "rule": "audit.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：快照读；不跨域查原正文。

幂等：GET幂等。

依赖：identity.checkAuthorization。

## M14-F015 ReadAudit

文件 `modules/analytics/internal/queries.go`。读取单条审计

```go
func ReadAudit(ctx context.Context, op OperationContext, in ReadAuditInput) (ReadAuditResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ReadAuditInput` — 读取单条审计；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ReadAuditResult` — 读取单条审计成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`auditId` auditId；UTF-8；不得为 null, `actor` 公共 AuthContext 合同，遵守其字段和空值规则, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`auditJSON` AuditRecord领域数据合同, `chainValid` chainValid 的明确真假结果，不可 null

步骤：

1. 验audit.read
2. 读取safe字段
3. 计算本条与prev链接一致性，异常标false而不悄悄修复

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：AUDIT_NOT_FOUND：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "audit.read", "rule": "audit.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：只读。

幂等：GET幂等。

依赖：ComputeAuditHash、identity.checkAuthorization。

## M14-F019 QueryUsageStatistics

文件 `modules/analytics/internal/queries.go`。费用汇总查询

```go
func QueryUsageStatistics(ctx context.Context, op OperationContext, in QueryUsageStatisticsInput) (QueryUsageStatisticsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: QueryUsageStatisticsInput` — 费用汇总查询；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: QueryUsageStatisticsResult` — 费用汇总查询成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`from` RFC3339 UTC 时间；不含 null, `to` RFC3339 UTC 时间；不含 null, `costClass` costClass；UTF-8；不得为 null, `actor` 公共 AuthContext 合同，遵守其字段和空值规则, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`amountMicros` amountMicros 非负 int64；不用浮点数, `entries` 有序列表；空列表表示没有可用项, `watermark` watermark 非负 int64；不用浮点数, `lastReconciledAt` lastReconciledAt；UTF-8；不得为 null

步骤：

1. 验stats.read
2. range/class校验
3. 读取派生汇总与最后对账水位
4. 显示可能有事件延迟

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：INVALID_RANGE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "stats.read", "rule": "stats.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：只读快照，不作允许模型调用的实时决策。

幂等：GET幂等。

依赖：identity.checkAuthorization。

## M14-F010 ValidateAudit

文件 `modules/analytics/internal/audit_validation.go`。校验审计allowlist

```go
func ValidateAudit(ctx context.Context, op OperationContext, in ValidateAuditInput) (ValidateAuditResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidateAuditInput` — 校验审计allowlist；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidateAuditResult` — 校验审计allowlist成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`auditId` auditId；UTF-8；不得为 null, `actorId` actorId；UTF-8；不得为 null, `action` action；UTF-8；不得为 null, `resourceId` resourceId；UTF-8；不得为 null, `resultCode` resultCode；UTF-8；不得为 null, `occurredAt` RFC3339 UTC 时间；不含 null

成功结果字段：`valid` valid 的明确真假结果，不可 null

步骤：

1. action/resultCode注册枚举
2. 资源只接受不透明ID/修订
3. 拒body/message/IP/token
4. 时间允许±5min

错误：

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "service鉴权生产者", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数，不通过日志修复非法payload。

幂等：同audit同结果。

依赖：根contracts/errors.json。

## M14-F012 ComputeAuditHash

文件 `modules/analytics/internal/audit_chain.go`。计算审计完整性链

```go
func ComputeAuditHash(ctx context.Context, op OperationContext, in ComputeAuditHashInput) (ComputeAuditHashResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ComputeAuditHashInput` — 计算审计完整性链；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ComputeAuditHashResult` — 计算审计完整性链成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`previousHash` previousHash；UTF-8；不得为 null, `auditJSON` AuditRecord领域数据合同

成功结果字段：`hash` hash；UTF-8；不得为 null

步骤：

1. JSON字段规范排序
2. SHA256(previousHash||canonicalJSON)
3. 排除链外本地传输时间

错误：

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "service内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数；链不声称防拥有磁盘和密钥的管理员篡改。

幂等：同prev/data同hash。

依赖：无。

## M14-F013 AppendAudit

文件 `modules/analytics/internal/audit_store.go`。追加安全审计

```go
func AppendAudit(ctx context.Context, op OperationContext, in AppendAuditInput) (AppendAuditResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: AppendAuditInput` — 追加安全审计；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: AppendAuditResult` — 追加安全审计成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`auditId` auditId；UTF-8；不得为 null, `actorId` actorId；UTF-8；不得为 null, `action` action；UTF-8；不得为 null, `resourceId` resourceId；UTF-8；不得为 null, `resultCode` resultCode；UTF-8；不得为 null, `metadata` metadata；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`seq` seq 非负 int64；不用浮点数, `hash` hash；UTF-8；不得为 null

步骤：

1. 服务生产者鉴权
2. 校验/redact
3. 事务锁链尾并insert唯一auditId/prevHash/hash

错误：

- `VALIDATION` / HTTP 400：INVALID_AUDIT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：AUDIT_CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：追加audit链。

授权：{"permission": "service", "rule": "service指定action生产者", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：单链append事务，禁止更新原审计。

幂等：auditId+payloadHash幂等。

依赖：ValidateAudit、RedactAuditMetadata、ComputeAuditHash。

## M14-F016 ValidateUsageEvent

文件 `modules/analytics/internal/usage_validation.go`。校验gateway钱账引用

```go
func ValidateUsageEvent(ctx context.Context, op OperationContext, in ValidateUsageEventInput) (ValidateUsageEventResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidateUsageEventInput` — 校验gateway钱账引用；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidateUsageEventResult` — 校验gateway钱账引用成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`ledgerEntryId` ledgerEntryId；UTF-8；不得为 null, `ledgerRevision` ledgerRevision 非负 int64；不用浮点数, `operationId` operationId；UTF-8；不得为 null, `costClass` costClass；UTF-8；不得为 null, `amountMicros` amountMicros 非负 int64；不用浮点数, `currency` currency；UTF-8；不得为 null

成功结果字段：`valid` valid 的明确真假结果，不可 null

步骤：

1. 只允许model-gateway签名生产者
2. class qa/background/admin-probe
3. currency USD整数micros≥0
4. entryrevision单增

错误：

- `VALIDATION` / HTTP 400：INVALID_USAGE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "service model-gateway", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯校验；不接受任意服务报告扣款。

幂等：同event同结果。

依赖：model-gateway.queryUsage。

## M14-F017 ProjectUsage

文件 `modules/analytics/internal/usage_store.go`。更新派生费用行

```go
func ProjectUsage(ctx context.Context, op OperationContext, in ProjectUsageInput) (ProjectUsageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ProjectUsageInput` — 更新派生费用行；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ProjectUsageResult` — 更新派生费用行成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`ledgerEntryId` ledgerEntryId；UTF-8；不得为 null, `ledgerRevision` ledgerRevision 非负 int64；不用浮点数, `operationId` operationId；UTF-8；不得为 null, `costClass` costClass；UTF-8；不得为 null, `amountMicros` amountMicros 非负 int64；不用浮点数, `currency` currency；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`applied` applied 的明确真假结果，不可 null

步骤：

1. 查event inbox
2. 相同entry revision hash相同跳过
3. 更新高修订settled/reversed及watermark，低修订忽略

错误：

- `CONFLICT` / HTTP 409：USAGE_CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写projection/inbox。

授权：{"permission": "service", "rule": "service model-gateway事件", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：事务按ledgerEntryId revision CAS。

幂等：entryId/revision/hash幂等。

依赖：ValidateUsageEvent。

## M14-F018 AggregateUsage

文件 `modules/analytics/internal/usage_aggregation.go`。费用派生聚合

```go
func AggregateUsage(ctx context.Context, op OperationContext, in AggregateUsageInput) (AggregateUsageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: AggregateUsageInput` — 费用派生聚合；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: AggregateUsageResult` — 费用派生聚合成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`day` day；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`amountMicros` amountMicros 非负 int64；不用浮点数, `entryCount` entryCount 非负 int64；不用浮点数, `watermark` watermark 非负 int64；不用浮点数

步骤：

1. 按gateway账项最终state取整数micros
2. 按class/provider/model group
3. 事务replace day聚合

错误：

- `INTERNAL` / HTTP 500：OVERFLOW：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写daily_usage。

授权：{"permission": "service", "rule": "service worker", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：checked int64 sum溢出拒绝，不用float。

幂等：day/sourceWatermark幂等。

依赖：usage_projection。

## M14-F020 FetchUsagePage

文件 `modules/analytics/internal/gateway_client.go`。取得权威账本投影输入

```go
func FetchUsagePage(ctx context.Context, op OperationContext, in FetchUsagePageInput) (FetchUsagePageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: FetchUsagePageInput` — 取得权威账本投影输入；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: FetchUsagePageResult` — 取得权威账本投影输入成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`cursor` cursor；UTF-8；不得为 null, `watermark` watermark 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`entries` 有序列表；空列表表示没有可用项, `nextCursor` nextCursor；UTF-8；不得为 null

步骤：

1. service调用gateway.queryUsage固定watermark分页100
2. 只读取安全账项不读取prompt/token

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：GATEWAY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：WATERMARK_CHANGED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：HTTP只读。

授权：{"permission": "service", "rule": "service analytics", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：每页2.5s，总任务deadline60s；继续cursor不重建模型请求。

幂等：watermark/cursor可重试。

依赖：model-gateway.queryUsage。

## M14-F021 ReconcileUsage

文件 `modules/analytics/internal/reconciliation.go`。对账修复派生统计

```go
func ReconcileUsage(ctx context.Context, op OperationContext, in ReconcileUsageInput) (ReconcileUsageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ReconcileUsageInput` — 对账修复派生统计；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ReconcileUsageResult` — 对账修复派生统计成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`from` RFC3339 UTC 时间；不含 null, `to` RFC3339 UTC 时间；不含 null, `actor` 公共 AuthContext 合同，遵守其字段和空值规则, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`watermark` watermark 非负 int64；不用浮点数, `mismatchCount` mismatchCount 非负 int64；不用浮点数

步骤：

1. 验system.manage
2. 获取gateway固定ledgerwatermark
3. 分页校验缺漏/修订
4. 事务replace projection/aggregate并写discrepancy事件

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：GATEWAY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：修改analytics派生数据，禁止改gateway余额。

授权：{"permission": "system.manage", "rule": "system.manage或service定时任务", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：单reconcile锁，生产事件在更高watermark保留等待合并。

幂等：range/watermark幂等。

依赖：FetchUsagePage、ProjectUsage、AggregateUsage。

## M14-F022 ConsumeAnalyticsEvent

文件 `modules/analytics/internal/event_handler.go`。消费安全审计和费用事件

```go
func ConsumeAnalyticsEvent(ctx context.Context, op OperationContext, in ConsumeAnalyticsEventInput) (ConsumeAnalyticsEventResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ConsumeAnalyticsEventInput` — 消费安全审计和费用事件；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ConsumeAnalyticsEventResult` — 消费安全审计和费用事件成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`event` 公共 EventEnvelope 合同，遵守其字段和空值规则

成功结果字段：`accepted` accepted 的明确真假结果，不可 null

步骤：

1. 验model-gateway invocation.settled或登记直接安全审计请求/epoch
2. inbox eventId去重
3. settled只携引用，调用gateway.queryUsage固定watermark取得账项整数金额
4. ProjectUsage/AppendAudit持久后ack，失败保留delivery重试

错误：

- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写inbox/audit/projection。

授权：{"permission": "service", "rule": "service白名单", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：按subject/entry修订取max，乱序不回退。

幂等：eventId/revision/hash幂等。

依赖：model-gateway.queryUsage、ProjectUsage、AppendAudit、jobs.acknowledgeEvent。

## M14-F023 CreateExport

文件 `modules/analytics/internal/export.go`。受权限保护统计导出

```go
func CreateExport(ctx context.Context, op OperationContext, in CreateExportInput) (CreateExportResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: CreateExportInput` — 受权限保护统计导出；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: CreateExportResult` — 受权限保护统计导出成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`kind` kind；UTF-8；不得为 null, `from` RFC3339 UTC 时间；不含 null, `to` RFC3339 UTC 时间；不含 null, `actor` 公共 AuthContext 合同，遵守其字段和空值规则, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`exportId` exportId；UTF-8；不得为 null, `objectRef` 公共 ObjectRef 合同，遵守其字段和空值规则, `expiresAt` RFC3339 UTC 时间；不含 null

步骤：

1. 根据kind查stats.read/audit.read
2. 按固定watermark查询≤100000行
3. CSV公式危险前缀加单引号
4. 生成私有引用24h

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：EXPORT_LIMIT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写私有导出文件/run。

授权：{"permission": "stats.read", "rule": "stats.read或audit.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：内存最多1000行块写；文件temp fsync rename再DB记录。

幂等：operationId/filterHash/actor幂等。

依赖：QueryPageStatistics、QueryAudits、QueryUsageStatistics。

## M14-F024 ValidateCSVCell

文件 `modules/analytics/internal/csv.go`。安全CSV单元格

```go
func ValidateCSVCell(ctx context.Context, op OperationContext, in ValidateCSVCellInput) (ValidateCSVCellResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidateCSVCellInput` — 安全CSV单元格；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidateCSVCellResult` — 安全CSV单元格成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`value` value；UTF-8；不得为 null

成功结果字段：`escaped` escaped；UTF-8；不得为 null

步骤：

1. 首字符=+@-制表符则加单引号
2. 双引号倍写
3. 换行逗号引号用quote包裹

错误：

- `VALIDATION` / HTTP 400：CELL_TOO_LARGE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "export内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数，每cell≤8192bytes。

幂等：同value同result。

依赖：无。

## M14-F025 PurgeBeacons

文件 `modules/analytics/internal/retention.go`。清理短期匿名行

```go
func PurgeBeacons(ctx context.Context, op OperationContext, in PurgeBeaconsInput) (PurgeBeaconsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: PurgeBeaconsInput` — 清理短期匿名行；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: PurgeBeaconsResult` — 清理短期匿名行成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`now` RFC3339 UTC 时间；不含 null, `batchSize` batchSize 非负 int64；不用浮点数

成功结果字段：`deleted` deleted 非负 int64；不用浮点数

步骤：

1. 默认30天明细已按日汇总后删除
2. 保留只含日hash的明细仍不可跨天识别人
3. 每日随机salt替换且旧salt24h删除
4. 日聚合长期保留
5. 未聚合异常记漏数后删除

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：删除匿名行/dayHash。

授权：{"permission": "service", "rule": "service worker", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：≤500批事务，不删除未来watermark。

幂等：重复purge幂等。

依赖：aggregation_watermarks。

## M14-F026 PurgeAuditHistory

文件 `modules/analytics/internal/retention.go`。审计保留期清理

```go
func PurgeAuditHistory(ctx context.Context, op OperationContext, in PurgeAuditHistoryInput) (PurgeAuditHistoryResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: PurgeAuditHistoryInput` — 审计保留期清理；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: PurgeAuditHistoryResult` — 审计保留期清理成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`now` RFC3339 UTC 时间；不含 null, `retentionDays` retentionDays 非负 int64；不用浮点数

成功结果字段：`deleted` deleted 非负 int64；不用浮点数, `anchorHash` anchorHash；UTF-8；不得为 null

步骤：

1. retention默认180最少30
2. 创建删除前链锚hash
3. 删过期记录并追加retention审计

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTERNAL` / HTTP 500：INVALID_RETENTION：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：删除过期audit+写锚点。

授权：{"permission": "system.manage", "rule": "system.manage或固定service策略", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：维护事务锁链，不打断链验证。

幂等：cutoff/anchor幂等。

依赖：AppendAudit。

## M14-F027 PurgeExports

文件 `modules/analytics/internal/retention.go`。清理导出临时对象

```go
func PurgeExports(ctx context.Context, op OperationContext, in PurgeExportsInput) (PurgeExportsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: PurgeExportsInput` — 清理导出临时对象；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: PurgeExportsResult` — 清理导出临时对象成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`now` RFC3339 UTC 时间；不含 null

成功结果字段：`deleted` deleted 非负 int64；不用浮点数

步骤：

1. 选择expiresAt≤now已ready
2. 删除文件后markexpired
3. 遗留文件扫描按引用回收

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：STORE_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：删导出文件/状态。

授权：{"permission": "service", "rule": "service worker", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：文件/DB非单事务，重启扫描可重复补偿。

幂等：exportId幂等。

依赖：私有对象目录。

## M14-F028 PublishAggregateEvents

文件 `modules/analytics/internal/outbox.go`。派生事件outbox派发

```go
func PublishAggregateEvents(ctx context.Context, op OperationContext, in PublishAggregateEventsInput) (PublishAggregateEventsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: PublishAggregateEventsInput` — 派生事件outbox派发；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: PublishAggregateEventsResult` — 派生事件outbox派发成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`batchSize` batchSize 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`sent` sent 非负 int64；不用浮点数

步骤：

1. ≤100固定eventId
2. 向jobs.publishEvent
3. 成功mark delivered，失败有限退避但不重复聚合

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：JOBS_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：外部通知/写outbox。

授权：{"permission": "service", "rule": "service", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：30s发送租约，至少一次。

幂等：eventId固定。

依赖：jobs.publishEvent。

## M14-F030 EnterBarrier

文件 `modules/analytics/internal/barrier.go`。进入analytics一致屏障

```go
func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: BarrierRequest` — 进入analytics一致屏障；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BarrierStatus` — 进入analytics一致屏障成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`operationId` #/$defs/Identifier, `barrierId` #/$defs/Identifier, `mode` enum, `deadline` #/$defs/Timestamp

成功结果字段：`barrierId` #/$defs/Identifier, `owner` , `state` enum, `dataEpoch` #/$defs/Identifier, `revisionWatermark` #/$defs/Revision

步骤：

1. 拒新审计/费用事件让生产outbox保留
2. flush匿名queue到持久安全点
3. 停止聚合/export写
4. checkpoint数据库

错误：

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写barrier，匿名队列可以明确漏数。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：等待所有在途事务和export file恢复日志到安全点。

幂等：barrierId幂等。

依赖：site-config.readRuntimeState。

## M14-F031 ReleaseBarrier

文件 `modules/analytics/internal/barrier.go`。解除统计维护

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ReleaseBarrierInput` — 解除统计维护；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ReleaseBarrierResult` — 解除统计维护成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`barrierId` barrierId；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` 公共 BarrierStatus 合同，遵守其字段和空值规则

步骤：

1. 检查新epoch全域ready
2. 重新拉gateway watermark对账任务
3. 恢复事件接收但不自动重调用模型

错误：

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写barrier/对账引用任务。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：不释放到混代读取。

幂等：barrierId幂等。

依赖：site-config.readRuntimeState、jobs.createJob。

## M14-F032 CreateSnapshot

文件 `modules/analytics/internal/snapshot.go`。导出统计快照

```go
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotExportRequest` — 导出统计快照；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotExportResult` — 导出统计快照成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`backupId` #/$defs/Identifier, `barrierId` #/$defs/Identifier, `dataEpoch` #/$defs/Identifier, `includeSecrets` boolean

成功结果字段：`snapshot` #/$defs/SnapshotRef, `checksum` #/$defs/Hash

步骤：

1. 要求safe
2. checkpoint
3. 包含聚合/审计安全元数据/费用投影/inbox/链锚
4. 匿名dayHash和导出文件排除

错误：

- `LOCKED` / HTTP 409：BARRIER_REQUIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写snapshot对象。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：固定watermark无写入。

幂等：backupId/barrierId幂等。

依赖：私有SQLite。

## M14-F033 ValidateImport

文件 `modules/analytics/internal/import.go`。校验统计恢复

```go
func ValidateImport(ctx context.Context, op OperationContext, request SnapshotValidateRequest) (SnapshotValidation, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotValidateRequest` — 校验统计恢复；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotValidation` — 校验统计恢复成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`snapshot` #/$defs/SnapshotRef, `targetDataEpoch` #/$defs/Identifier

成功结果字段：`valid` boolean, `issues` array, `migrationPlanId` #/$defs/Identifier

步骤：

1. 校验hash/schema
2. 审计链与privacyallowlist
3. micros整数和projection unique
4. 缺gateway最终账项只标需对账

错误：

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：隔离验证报告。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：不得依赖live旧gateway来确认新snapshot。

幂等：snapshotHash/targetEpoch幂等。

依赖：CheckPrivacyFields、ComputeAuditHash。

## M14-F034 PrepareImport

文件 `modules/analytics/internal/import.go`。准备统计导入

```go
func PrepareImport(ctx context.Context, op OperationContext, request SnapshotPrepareRequest) (SnapshotStageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotPrepareRequest` — 准备统计导入；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotStageResult` — 准备统计导入成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`restoreId` #/$defs/Identifier, `snapshot` #/$defs/SnapshotRef, `barrierId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier

成功结果字段：`stageId` #/$defs/Identifier, `restoreId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier, `ready` boolean

步骤：

1. 验证快照
2. 建isolated DB重绑targetEpoch
3. 清beacon/dayHash/export
4. 费用汇总标pending-reconcile

错误：

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写隔离stage。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：不影响live统计。

幂等：restoreId/hash幂等。

依赖：ValidateImport。

## M14-F035 ActivateImport

文件 `modules/analytics/internal/import.go`。激活统计新代次

```go
func ActivateImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotCommitRequest` — 激活统计新代次；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotApplyResult` — 激活统计新代次成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`stageId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier, `barrierId` #/$defs/Identifier

成功结果字段：`stageId` #/$defs/Identifier, `dataEpoch` #/$defs/Identifier, `state` enum

步骤：

1. 验证commit决策
2. 原子DB指针
3. 所有读仍被全站维护挡住直到全域激活

错误：

- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：切数据库。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：恢复pending reconcile不阻止余额权威gateway安全启动。

幂等：stageId/epoch幂等。

依赖：data-protection.getRestoreDecision。

## M14-F036 AbortImport

文件 `modules/analytics/internal/import.go`。撤销未提交stage

```go
func AbortImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotCommitRequest` — 撤销未提交stage；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotApplyResult` — 撤销未提交stage成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`stageId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier, `barrierId` #/$defs/Identifier

成功结果字段：`stageId` #/$defs/Identifier, `dataEpoch` #/$defs/Identifier, `state` enum

步骤：

1. 查decision未commit
2. 删除隔离DB
3. 已commit拒绝并继续激活

错误：

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：删除隔离stage。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：stage锁。

幂等：重复rolled-back成功。

依赖：data-protection.getRestoreDecision。

## M14-F037 ValidateRecoveryAuthorization

文件 `modules/analytics/internal/recovery_authorization.go`。本地验证本次恢复专用控制许可

```go
func ValidateRecoveryAuthorization(ctx context.Context, op OperationContext, in ValidateRecoveryAuthorizationInput) (ValidateRecoveryAuthorizationResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidateRecoveryAuthorizationInput` — 本地验证本次恢复专用控制许可；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidateRecoveryAuthorizationResult` — 本地验证本次恢复专用控制许可成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`authorization` 公共 RecoveryAuthorization 合同，遵守其字段和空值规则, `restoreId` restoreId；UTF-8；不得为 null, `barrierId` barrierId；UTF-8；不得为 null, `sourceEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧, `targetEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧, `action` action；UTF-8；不得为 null

成功结果字段：`valid` valid 的明确真假结果，不可 null

步骤：

1. 先校验不可被业务导入覆盖的信任锚签名/算法/keyId
2. 核audience本域/expiry/restoreId/barrierId/旧新epoch
3. 读取独立控制记录确认已批准同restore且action只在prepare/commit/rollback/barrier
4. commit核持久commit决定，rollback拒任何commit证据
5. 拒普通业务使用

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "service+superadmin预先批准委托", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：校验期间不查询已换代identity业务库；控制记录与信任锚独立目录不被快照覆盖。

幂等：同许可同控制阶段结果；提交决策变化会使rollback失效。

依赖：不可变控制信任锚、data-protection.getRestoreDecision。

## M14-F038 LoadStatistics

文件 `modules/analytics/web/src/OverviewPanel.vue`。Vue3模块交互：LoadStatistics

```typescript
function LoadStatistics(op: OperationContext, input: LoadStatisticsInput): Promise<LoadStatisticsResult>
```

参数：

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: LoadStatisticsInput` — Vue3模块交互：LoadStatistics；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: LoadStatisticsResult` — Vue3模块交互：LoadStatistics成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: Problem` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`from` RFC3339 UTC 时间；不含 null, `to` RFC3339 UTC 时间；不含 null

成功结果字段：`rows` 有序列表；空列表表示没有可用项, `watermark` watermark 非负 int64；不用浮点数

步骤：

1. 并发读取访问和费用派生统计，展示漏数/watermark/对账时间，不显示实时可花余额
2. 处理401重新登录、403清空敏感界面
3. 按operationId重试不重复提交

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新组件本地状态；仅明确提交动作调用已授权API。

授权：{"permission": "stats.read", "rule": "stats.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：组件AbortController取消旧请求；同组件同动作单飞。

幂等：mutation稳定operationId/query只读重复安全。

依赖：analytics OpenAPI HTTP/JSON/SSE。

## M14-F039 LoadAuditPage

文件 `modules/analytics/web/src/AuditExplorer.vue`。Vue3模块交互：LoadAuditPage

```typescript
function LoadAuditPage(op: OperationContext, input: LoadAuditPageInput): Promise<LoadAuditPageResult>
```

参数：

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: LoadAuditPageInput` — Vue3模块交互：LoadAuditPage；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: LoadAuditPageResult` — Vue3模块交互：LoadAuditPage成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: Problem` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`cursor` cursor；UTF-8；不得为 null, `limit` limit 非负 int64；不用浮点数

成功结果字段：`auditIds` 有序列表；空列表表示没有可用项, `nextCursor` nextCursor；UTF-8；不得为 null

步骤：

1. 按允许action/日期查询安全审计，403清除已加载列表，游标换epoch重置
2. 处理401重新登录、403清空敏感界面
3. 按operationId重试不重复提交

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新组件本地状态；仅明确提交动作调用已授权API。

授权：{"permission": "audit.read", "rule": "audit.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：组件AbortController取消旧请求；同组件同动作单飞。

幂等：mutation稳定operationId/query只读重复安全。

依赖：analytics OpenAPI HTTP/JSON/SSE。

## M14-F040 DownloadAuthorizedExport

文件 `modules/analytics/web/src/client.ts`。Vue3模块交互：DownloadAuthorizedExport

```typescript
function DownloadAuthorizedExport(op: OperationContext, input: DownloadAuthorizedExportInput): Promise<DownloadAuthorizedExportResult>
```

参数：

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: DownloadAuthorizedExportInput` — Vue3模块交互：DownloadAuthorizedExport；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: DownloadAuthorizedExportResult` — Vue3模块交互：DownloadAuthorizedExport成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: Problem` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`exportId` exportId；UTF-8；不得为 null

成功结果字段：`downloaded` downloaded 的明确真假结果，不可 null

步骤：

1. 重新校验导出权限并下载私有CSV，24小时过期要求重建，同actor绑定
2. 处理401重新登录、403清空敏感界面
3. 按operationId重试不重复提交

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新组件本地状态；仅明确提交动作调用已授权API。

授权：{"permission": "stats.read", "rule": "stats.read", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：组件AbortController取消旧请求；同组件同动作单飞。

幂等：mutation稳定operationId/query只读重复安全。

依赖：analytics OpenAPI HTTP/JSON/SSE。

## M14-F041 ClassifyCrawler

文件 `modules/analytics/internal/crawler.go`。内存识别已知爬虫

```go
func ClassifyCrawler(ctx context.Context, op OperationContext, in ClassifyCrawlerInput) (ClassifyCrawlerResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ClassifyCrawlerInput` — 内存识别已知爬虫；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ClassifyCrawlerResult` — 内存识别已知爬虫成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`userAgent` userAgent；UTF-8；不得为 null

成功结果字段：`knownCrawler` knownCrawler 的明确真假结果，不可 null, `crawlerFamily` crawlerFamily；UTF-8；不得为 null

步骤：

1. UA字符串max1024
2. 大小写不敏感登记GPTBot/ChatGPT-User/ClaudeBot/PerplexityBot/Googlebot/Bingbot
3. 返回family后丢弃UA
4. 真人beacon排除knownCrawler

错误：

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无UA原文存储。

授权：{"permission": "anonymous", "rule": "anonymous ingress内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数；UA不是可靠权限身份，仅统计过滤。

幂等：同UA规则version同结果。

依赖：内置规则配置。

## M14-F042 LookupCountry

文件 `modules/analytics/internal/geoip.go`。离线GeoIP国家

```go
func LookupCountry(ctx context.Context, op OperationContext, in LookupCountryInput) (LookupCountryResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: LookupCountryInput` — 离线GeoIP国家；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: LookupCountryResult` — 离线GeoIP国家成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`peerAddress` peerAddress；UTF-8；不得为 null

成功结果字段：`countryCode` countryCode；UTF-8；不得为 null

步骤：

1. 可信peer内存查询DB-IP本地mmdb
2. 无库/未知返回ZZ
3. 立即丢弃IP
4. 不调用外部定位API

错误：

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无IP落盘。

授权：{"permission": "anonymous", "rule": "anonymous ingress内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：本地read-onlyDB，启动读version。

幂等：同DB版本/IP同结果。

依赖：离线GeoIP数据库。

## M14-F043 RecordCrawlerAccess

文件 `modules/analytics/internal/crawler_store.go`。AI Markdown访问独立计数

```go
func RecordCrawlerAccess(ctx context.Context, op OperationContext, in RecordCrawlerAccessInput) (RecordCrawlerAccessResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RecordCrawlerAccessInput` — AI Markdown访问独立计数；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RecordCrawlerAccessResult` — AI Markdown访问独立计数成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `locale` locale；UTF-8；不得为 null, `crawlerFamily` crawlerFamily；UTF-8；不得为 null, `status` status 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`accepted` accepted 的明确真假结果，不可 null

步骤：

1. public-site service身份
2. family登记
3. 成功.md请求按文档语言family日计数
4. 不传IP/UA/prompt
5. 与浏览器PV/UV表分离

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写crawler聚合。

授权：{"permission": "service", "rule": "service public-site", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：事务按crawlerBeaconId去重。

幂等：crawlerBeaconId幂等。

依赖：public-site Markdown完成响应。

## M14-F044 QueryContentGaps

文件 `modules/analytics/internal/content_gaps.go`。热门文档与答不出问题联动

```go
func QueryContentGaps(ctx context.Context, op OperationContext, in QueryContentGapsInput) (QueryContentGapsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: QueryContentGapsInput` — 热门文档与答不出问题联动；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: QueryContentGapsResult` — 热门文档与答不出问题联动成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`from` RFC3339 UTC 时间；不含 null, `to` RFC3339 UTC 时间；不含 null, `actor` 公共 AuthContext 合同，遵守其字段和空值规则, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`hotDocuments` 有序列表；空列表表示没有可用项, `unansweredRecordIds` 有序列表；空列表表示没有可用项

步骤：

1. checkstats.read和qa记录读取权限
2. 热门文档按语言
3. qa只返回安全recordId/分类计数，问题正文仍由qa权限页面读取
4. 显示QA量/缓存命中率/无答案比率

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "qa.manage", "rule": "stats.read+qa.manage资源范围", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：两源固定watermark附更新时间。

幂等：GET幂等。

依赖：QueryPageStatistics、qa统计引用API。

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M14-F901 main

文件 `modules/analytics/cmd/analytics/main.go`。独立进程可执行入口

```go
func main()
```

输入：

无参数/返回值。

输出：

无参数/返回值。

步骤：

1. 读取--config/--data-dir/--listen及环境覆盖，严格解析ServiceConfig；配置错误安全stderr后exit2
2. signal.NotifyContext注册SIGINT/SIGTERM；调用BuildApplication，失败exit1且只输出errorCode
3. 调用Serve；正常关闭或signal后以30秒context调用Shutdown；安全退出0，无法安全关闭exit1且保留journal

错误：

- `VALIDATION`：配置未知字段/越界目录/非法端点；HTTP 400；retryable=false
- `INTERNAL`：装配或HTTP运行失败；HTTP 500；retryable=false

副作用：打开本域资源、监听HTTP；关闭按持久安全点。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：一个进程一个Application；OS信号处理只触发一次drain

幂等：进程重启先journal恢复；不重放已付费任务

依赖：BuildApplication、Serve、Shutdown

### M14-F902 BuildApplication

文件 `modules/analytics/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

```go
func BuildApplication(ctx context.Context, config ServiceConfig) (*Application, error)
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `config: ServiceConfig`：已严格解析配置；约束：module固定本域；secret只ref；依赖只HTTP

输出：

- `application: *Application`：只返回完整装配后的内部容器；约束：失败返回nil，已打开资源逆序关闭

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 验证module/listen/data目录/secretRef；建立本域控制区和业务数据区；portal不虚构业务DB
2. 打开本域DB并运行已有schema迁移/文件journal恢复；保留restore控制记录/barrier/trust-anchor
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；已commit恢复未全完成时仅开放控制管理；注册本域主动claim worker并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源

错误：

- `VALIDATION`：配置/缺路由绑定；HTTP 400；retryable=false
- `INTEGRITY_ERROR`：迁移或journal校验失败；HTTP 422；retryable=false
- `DEPENDENCY_UNAVAILABLE`：基础关键依赖/控制信任锚不可用；HTTP 503；retryable=true

副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法

幂等：启动重入按journal和epoch恢复，不新建paid operation

依赖：本域implementation全部x-handler、本域journal恢复函数、site-config.readRuntimeState（本域site-config读取自身权威）

### M14-F903 Serve

文件 `modules/analytics/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

```go
func Serve(ctx context.Context, app *Application) error
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 仅HTTP启动net/http.Server，ReadHeaderTimeout=5s、MaxHeaderBytes=64KiB；写deadline由各路由决定，SSE不用统一短WriteTimeout
2. 启动本域worker循环，匿名Health/Ready优先可访问；普通路由执行权限/epoch/maintenance门禁，SSE响应X-Accel-Buffering:no
3. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

错误：

- `INTERNAL`：监听地址冲突或HTTP server不可恢复错误；HTTP 500；retryable=false
- `CONFLICT`：同Application重复Serve启动；HTTP 409；retryable=false

副作用：监听本域HTTP端口、服务请求和本域worker。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：只有一Serve调用；每请求取消独立，worker只本域

幂等：重复Serve返回CONFLICT语义，不开第二监听

依赖：BuildApplication、Shutdown、本域OpenAPI路由

### M14-F904 Shutdown

文件 `modules/analytics/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

```go
func Shutdown(ctx context.Context, app *Application) error
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. CAS lifecycle ready→draining，拒新写/claim/paid步骤并关闭SSE接受新订阅
2. 取消本域worker的新步骤；等待已有DB事务、文件journal、模型可结算阶段安全确认；不能因超时假定供应方免费或删除paid refs
3. http.Server.Shutdown等在途响应；flush本域outbox/journal、close DB与文件句柄；正常lifecycle closed
4. deadline未安全返回DEADLINE_EXCEEDED并保留完整journal/未决费用引用，启动恢复收敛；不清掉活跃控制记录

错误：

- `DEADLINE_EXCEEDED`：30秒期限仍有未安全在途步骤；HTTP 504；retryable=true
- `INTERNAL`：flush/close失败；HTTP 500；retryable=false

副作用：本域drain、资源关闭、durable恢复日志。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：sync.Once启动drain；其他调用等待同一次结果

幂等：重复Shutdown返回同一次关闭结果；已closed成功

依赖：本域barrier安全点函数、本域journal/outbox flush

### M14-F905 Health

文件 `modules/analytics/internal/transport/server.go`。GET /healthz普通HTTP存活检查

```go
func Health(ctx context.Context, app *Application) (HealthStatus, error)
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `status: HealthStatus`：公共liveness安全字段；约束：仅module/version/status/capabilities

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 仅读本进程lifecycle与HTTP循环是否存活；不查询远程依赖/模型/数据库
2. 存活且未closed返回200 HealthStatus；closed或server不可用返回503；不泄漏路径、端点或secret

错误：

- `DEPENDENCY_UNAVAILABLE`：进程已closed或服务循环不可用；HTTP 503；retryable=true

副作用：无。

授权：{"permission": "anonymous", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：无锁或原子读取lifecycle；不进行外部调用

幂等：GET只读重复安全

依赖：无

### M14-F906 Ready

文件 `modules/analytics/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

```go
func Ready(ctx context.Context, app *Application) (ServiceReadiness, error)
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `readiness: ServiceReadiness`：本域启动/恢复就绪与公开门禁状态；约束：ready不等于维护已解除，publicRequestsAllowed必须显式false

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 确认配置/本域schema/journal/control信任锚完整并在lifecycle ready或维护控制ready
2. 已初始化时核activeDataEpoch等于runtime目标代次；关键运行权威不可达或旧epoch返回503；site-config本域读取自身权威
3. 维护中本域新epoch已激活可返回200 control-ready但publicRequestsAllowed=false，使全域激活检查不会和维护门禁互相等待；所有域完成后runtime解除维护才开放
4. 未初始化可仅带初始化能力ready，activeDataEpoch=null；无AI渠道不使基础内容能力失败，能力缺项显式呈现

错误：

- `DEPENDENCY_UNAVAILABLE`：关键依赖/本域journal未恢复/epoch未匹配；HTTP 503；retryable=true

副作用：无。

授权：{"permission": "anonymous", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测

幂等：GET只读重复安全

依赖：site-config.readRuntimeState（site-config本域自身读取）

### M14-F907 mountModuleUI

文件 `modules/analytics/web/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

```typescript
function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void
```

输入：

- `rootElement: HTMLElement`：已经存在的模块挂载节点；约束：DOM标准库；为空或不属于当前document拒绝

- `bootstrap: ModuleUIBootstrap`：已由portal/Go输出的安全启动元数据；约束：module固定本域，不含service凭证/secret

输出：

无参数/返回值。

步骤：

1. 校验rootElement与bootstrap module/capabilities；按本域已登记Vue页面导入模块路由，createApp挂载，不注册别域业务组件
2. 只装配同源HTTP/JSON/SSE客户端和已有权限清理/语言逻辑，复用identity/qa会话，缺AI能力保持关闭提示
3. 同DOM重复mount先复用既有实例；unmount停止AbortController/SSE/listeners，不能清服务端业务记录

错误：

- `VALIDATION`：bootstrap不匹配本域或DOM无效；HTTP 400；retryable=false

副作用：挂载本域Vue组件和浏览器事件；不新建业务记录。

授权：{"permission": "anonymous", "rule": "只挂载UI；业务mutation仍按各本域OpenAPI权限，不能以能力字段绕过权限"}

并发：同DOM一个Vue实例，AbortController取消旧请求

幂等：同DOM/buildRevision重复挂载复用实例

依赖：本域已登记UI页面函数
<!-- service-bootstrap:end -->
