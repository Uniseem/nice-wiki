# 知识入库与混合检索逐函数实现合同

每一函数与implementation.json一一对应；输入和结果的全部字段见本节对应JSON Schema，公共类型见根common.schema.json。

## knowledge.LoadRuntime

`func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)`

作用：启动索引服务

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: LoadRuntimeInput`：封闭业务输入；逐字段见types $defs/LoadRuntimeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| dataDirectory | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 私有目录 |
| serviceId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 服务身份 |

返回：

- `result: LoadRuntimeResult`：成功结果；逐字段见types $defs/LoadRuntimeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| ready | 是 | `{"type":"boolean"}` | 就绪 |
| activeGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 已载入代数 |

算法：

1. 1 检查DB与索引文件manifest
2. 2 核验空间/哈希一致
3. 3 不一致active标unavailable
4. 4 注册租约恢复
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：打开本服务索引文件不读其他服务存储

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.PutIndexConfig

`func PutIndexConfig(ctx context.Context, op OperationContext, input PutIndexConfigInput) (PutIndexConfigResult, error)`

作用：更新索引配置

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutIndexConfigInput`：封闭业务输入；逐字段见types $defs/PutIndexConfigInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| config | 是 | `{"$ref":"#/$defs/IndexConfig"}` | 新配置 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutIndexConfigResult`：成功结果；逐字段见types $defs/PutIndexConfigResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| config | 是 | `{"$ref":"#/$defs/IndexConfig"}` | 新版本 |

算法：

1. 1 校验chunkTokens/overlap
2. 2 解析模型空间声明
3. 3 空间变化只生成新代数不覆盖active
4. 4 CAS保存
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：配置变更outbox；既有索引保留

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:readModels

## knowledge.HandlePublication

`func HandlePublication(ctx context.Context, op OperationContext, input HandlePublicationInput) (HandlePublicationResult, error)`

作用：接受权威发布事件

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: HandlePublicationInput`：封闭业务输入；逐字段见types $defs/HandlePublicationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 事件ID |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 发布引用 |

返回：

- `result: HandlePublicationResult`：成功结果；逐字段见types $defs/HandlePublicationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| accepted | 是 | `{"type":"boolean"}` | 已消费 |
| buildId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 创建构建 |

算法：

1. 1 inbox按eventId去重
2. 2 验证dataEpoch
3. 3 配置关闭或未配置档位则记录skipped
4. 4 创建唯一document/revision/space build与job outbox同事务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写inbox/build/job事件，不处理草稿

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:createJob, content:validatePublications

## knowledge.CreateBuild

`func CreateBuild(ctx context.Context, op OperationContext, input CreateBuildInput) (CreateBuildResult, error)`

作用：人工或事件申请构建

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateBuildInput`：封闭业务输入；逐字段见types $defs/CreateBuildInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 发布引用 |
| force | 是 | `{"type":"boolean"}` | 是否同revision重建 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CreateBuildResult`：成功结果；逐字段见types $defs/CreateBuildResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| build | 是 | `{"$ref":"#/$defs/IndexBuild"}` | queued构建 |

算法：

1. 1 向content验证visible/current
2. 2 固定config/space
3. 3 同修订同空间幂等
4. 4 force新generation且保留旧active
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写build+任务outbox

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, jobs:createJob

## knowledge.FetchPublishedSource

`func FetchPublishedSource(ctx context.Context, op OperationContext, input FetchPublishedSourceInput) (FetchPublishedSourceResult, error)`

作用：取得权威正文

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: FetchPublishedSourceInput`：封闭业务输入；逐字段见types $defs/FetchPublishedSourceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: FetchPublishedSourceResult`：成功结果；逐字段见types $defs/FetchPublishedSourceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 已确认引用 |
| source | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 权威正文UTF8 |
| contentHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 正文hash |

算法：

1. 1 校验build租约及deadline
2. 2 调content只读发布对象
3. 3 对比contentHash
4. 4 保存私有临时source引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写构建临时文件；不复制草稿

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:readPublishedRevision

## knowledge.ListExtractableAssets

`func ListExtractableAssets(ctx context.Context, op OperationContext, input ListExtractableAssetsInput) (ListExtractableAssetsResult, error)`

作用：筛选发布引用媒体

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListExtractableAssetsInput`：封闭业务输入；逐字段见types $defs/ListExtractableAssetsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 来源 |
| source | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 权威正文 |

返回：

- `result: ListExtractableAssetsResult`：成功结果；逐字段见types $defs/ListExtractableAssetsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| assets | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"},"maxItems":256}` | 去重可提取资源 |

算法：

1. 1 按正文schema解析引用
2. 2 media批量授权验证
3. 3 限制每文档100资源单件50MiB
4. 4 不支持类型标明unsupported
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：media:acquireResourceReferences

## knowledge.CreateExtraction

`func CreateExtraction(ctx context.Context, op OperationContext, input CreateExtractionInput) (CreateExtractionResult, error)`

作用：按hash幂等提取资源

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateExtractionInput`：封闭业务输入；逐字段见types $defs/CreateExtractionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| asset | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 资源 |
| extractorVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 提取版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CreateExtractionResult`：成功结果；逐字段见types $defs/CreateExtractionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extraction | 是 | `{"$ref":"#/$defs/Extraction"}` | 提取状态 |

算法：

1. 1 核验允许MIME
2. 2 按asset hash/extractorVersion去重
3. 3 记录来源授权
4. 4 排队独立租约
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写extraction+job事件

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：media:readAsset, jobs:createJob

## knowledge.ExtractPdfText

`func ExtractPdfText(ctx context.Context, op OperationContext, input ExtractPdfTextInput) (ExtractPdfTextResult, error)`

作用：提取PDF可用文字

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ExtractPdfTextInput`：封闭业务输入；逐字段见types $defs/ExtractPdfTextInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extractionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提取 |
| asset | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | PDF |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: ExtractPdfTextResult`：成功结果；逐字段见types $defs/ExtractPdfTextResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extraction | 是 | `{"$ref":"#/$defs/Extraction"}` | ready或unsupported |

算法：

1. 1 media授权流下载并验证hash
2. 2 在无网络沙箱读取最多500页/50MiB
3. 3 保留页码按阅读序提取
4. 4 无文本标unsupported等待显式OCR任务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数60000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNSUPPORTED_CAPABILITY (HTTP422)：声明能力或资源提取类型不受支持。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：本地提取临时文件；无模型调用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数60000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：media:readAsset

## knowledge.ExtractImageDescription

`func ExtractImageDescription(ctx context.Context, op OperationContext, input ExtractImageDescriptionInput) (ExtractImageDescriptionResult, error)`

作用：显式生成图片检索描述

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ExtractImageDescriptionInput`：封闭业务输入；逐字段见types $defs/ExtractImageDescriptionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extractionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提取 |
| asset | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 图片 |
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 视觉档位 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: ExtractImageDescriptionResult`：成功结果；逐字段见types $defs/ExtractImageDescriptionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extraction | 是 | `{"$ref":"#/$defs/Extraction"}` | 视觉描述 |

算法：

1. 1 发布构建显式视觉描述配置
2. 2 根knowledge.describe-visual使用vision档（未设采用fast视觉能力）800output token
3. 3 图片/画板/视频封面assetHash+task/model/memory version幂等复用
4. 4 保存machineDerived仅检索不覆盖人工caption，取消旧revision后不写。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：可能产生MG费用；只保存派生文本

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference, media:readAsset

## knowledge.ExtractVideoMetadata

`func ExtractVideoMetadata(ctx context.Context, op OperationContext, input ExtractVideoMetadataInput) (ExtractVideoMetadataResult, error)`

作用：提取视频题名及授权字幕

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ExtractVideoMetadataInput`：封闭业务输入；逐字段见types $defs/ExtractVideoMetadataInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extractionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提取 |
| asset | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 视频资源 |

返回：

- `result: ExtractVideoMetadataResult`：成功结果；逐字段见types $defs/ExtractVideoMetadataResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extraction | 是 | `{"$ref":"#/$defs/Extraction"}` | 元数据提取 |

算法：

1. 1 从media取得已授权平台元数据
2. 2 仅收录已有字幕与题名
3. 3 校验语言/编码
4. 4 无字幕不自动转写
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写派生元数据，不请求外部平台密钥

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：media:readVideoMetadata

## knowledge.NormalizeSource

`func NormalizeSource(ctx context.Context, op OperationContext, input NormalizeSourceInput) (NormalizeSourceResult, error)`

作用：稳定规范化输入

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: NormalizeSourceInput`：封闭业务输入；逐字段见types $defs/NormalizeSourceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| source | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 正文或提取文本 |
| preprocessingVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 固定预处理版本 |

返回：

- `result: NormalizeSourceResult`：成功结果；逐字段见types $defs/NormalizeSourceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| normalized | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范文本 |
| sourceOffsets | 是 | `{"type":"array","items":{"type":"integer","minimum":0,"maximum":9007199254740991},"maxItems":65536}` | 规范UTF8到源偏移 |

算法：

1. 1 解析块级纯文本
2. 2 Unicode NFKC并统一LF
3. 3 保留段落和代码大小写
4. 4 构建offset映射不丢弃引用源位置
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.SplitChunks

`func SplitChunks(ctx context.Context, op OperationContext, input SplitChunksInput) (SplitChunksResult, error)`

作用：按源块及token边界分块

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SplitChunksInput`：封闭业务输入；逐字段见types $defs/SplitChunksInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 来源 |
| normalized | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范文本 |
| chunkTokens | 是 | `{"type":"integer","minimum":128,"maximum":1024,"default":512}` | 块预算 |
| overlapTokens | 是 | `{"type":"integer","minimum":0,"maximum":128,"default":64}` | 重叠预算 |

返回：

- `result: SplitChunksResult`：成功结果；逐字段见types $defs/SplitChunksResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| chunks | 是 | `{"type":"array","items":{"$ref":"#/$defs/Chunk"},"maxItems":256}` | 确定性分块 |

算法：

1. 1 同小节相邻块合并默认512token但不跨来源权限
2. 2 记录全部blockIds与原UTF8偏移
3. 3 图片描述+图注+相邻块，附件长文本再切分
4. 4 chunk identity包括源block hashes与上下文版本，不将派生描述写回人工正文。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.WriteStagingChunks

`func WriteStagingChunks(ctx context.Context, op OperationContext, input WriteStagingChunksInput) (WriteStagingChunksResult, error)`

作用：保存独立分块代数

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: WriteStagingChunksInput`：封闭业务输入；逐字段见types $defs/WriteStagingChunksInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| chunks | 是 | `{"type":"array","items":{"$ref":"#/$defs/Chunk"},"maxItems":256}` | 分块 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: WriteStagingChunksResult`：成功结果；逐字段见types $defs/WriteStagingChunksResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| written | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 写入数 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 目标代数 |

算法：

1. 1 校验每chunk来源revision
2. 2 验证重复ID同hash
3. 3 一事务批量≤256
4. 4 不修改active pointer
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写chunks/terms staging

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.BuildLexicalIndex

`func BuildLexicalIndex(ctx context.Context, op OperationContext, input BuildLexicalIndexInput) (BuildLexicalIndexResult, error)`

作用：建立词项索引

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BuildLexicalIndexInput`：封闭业务输入；逐字段见types $defs/BuildLexicalIndexInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | BCP47语言 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: BuildLexicalIndexResult`：成功结果；逐字段见types $defs/BuildLexicalIndexResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| indexed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 分块数 |
| tokenizerVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 词项分词版本 |

算法：

1. 1 固定Unicode词界分词版本
2. 2 中日韩使用连续bigram
3. 3 保留代码标识词
4. 4 写freq/length及BM25统计
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写staging terms及docfreq

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.RequestEmbeddings

`func RequestEmbeddings(ctx context.Context, op OperationContext, input RequestEmbeddingsInput) (RequestEmbeddingsResult, error)`

作用：请求分块嵌入

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RequestEmbeddingsInput`：封闭业务输入；逐字段见types $defs/RequestEmbeddingsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| chunkIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 最多64个块 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: RequestEmbeddingsResult`：成功结果；逐字段见types $defs/RequestEmbeddingsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | MG调用 |
| vectors | 是 | `{"type":"array","items":{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536},"maxItems":64}` | 向量 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 确认空间 |

算法：

1. 1 读取固定chunkHash
2. 2 64项且总输入≤模型预算
3. 3 继承ctx调用MG
4. 4 对每返回space和dimension严格校验
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG产生费用；本服务不预留额度

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## knowledge.StoreEmbeddings

`func StoreEmbeddings(ctx context.Context, op OperationContext, input StoreEmbeddingsInput) (StoreEmbeddingsResult, error)`

作用：持久化验证过的向量

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: StoreEmbeddingsInput`：封闭业务输入；逐字段见types $defs/StoreEmbeddingsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| chunkIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 与vectors等长 |
| vectors | 是 | `{"type":"array","items":{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536},"maxItems":64}` | 向量 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: StoreEmbeddingsResult`：成功结果；逐字段见types $defs/StoreEmbeddingsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| stored | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 写入向量数 |

算法：

1. 1 等长和维度检查
2. 2 拒绝非有限数
3. 3 校验lease/chunkHash未变
4. 4 原子写vectorHash与向量
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写staging vectors

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.ValidateBuild

`func ValidateBuild(ctx context.Context, op OperationContext, input ValidateBuildInput) (ValidateBuildResult, error)`

作用：验证完整索引代数

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateBuildInput`：封闭业务输入；逐字段见types $defs/ValidateBuildInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: ValidateBuildResult`：成功结果；逐字段见types $defs/ValidateBuildResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 构建完整 |
| manifestHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 文件清单摘要 |

算法：

1. 1 count chunks==terms==vectors
2. 2 检查每chunk空间/hash
3. 3 校验索引文件checksum
4. 4 向content再次验证current/visible
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：标validating；不激活

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## knowledge.ActivateBuild

`func ActivateBuild(ctx context.Context, op OperationContext, input ActivateBuildInput) (ActivateBuildResult, error)`

作用：原子切换当前索引

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ActivateBuildInput`：封闭业务输入；逐字段见types $defs/ActivateBuildInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| expectedActiveGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧active代数 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: ActivateBuildResult`：成功结果；逐字段见types $defs/ActivateBuildResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| build | 是 | `{"$ref":"#/$defs/IndexBuild"}` | active或superseded |

算法：

1. 1 读取权威验证结果且有效期≤1秒
2. 2 再校验dataEpoch
3. 3 文档/space锁CAS active generation
4. 4 成功写active及index.ready否则superseded
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：原子active pointer更新与outbox；旧文件延迟回收

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## knowledge.CancelBuild

`func CancelBuild(ctx context.Context, op OperationContext, input CancelBuildInput) (CancelBuildResult, error)`

作用：取消入库链路

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CancelBuildInput`：封闭业务输入；逐字段见types $defs/CancelBuildInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| reason | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 取消原因 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CancelBuildResult`：成功结果；逐字段见types $defs/CancelBuildResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| build | 是 | `{"$ref":"#/$defs/IndexBuild"}` | 取消状态 |

算法：

1. 1 设取消标记
2. 2 取消jobs及所有MG invocation
3. 3 取消staging写权限
4. 4 保留active与收费未知引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：终止worker并清理非active临时数据

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:cancelJob, model-gateway:cancelInference

## knowledge.RetireDocumentIndex

`func RetireDocumentIndex(ctx context.Context, op OperationContext, input RetireDocumentIndexInput) (RetireDocumentIndexResult, error)`

作用：响应撤回/删除

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RetireDocumentIndexInput`：封闭业务输入；逐字段见types $defs/RetireDocumentIndexInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| documentId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 撤回文档 |
| visibilityRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 权威可见性版本 |
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 撤回事件 |

返回：

- `result: RetireDocumentIndexResult`：成功结果；逐字段见types $defs/RetireDocumentIndexResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| retired | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 失活索引数 |

算法：

1. 1 inbox去重
2. 2 只接受更高visibilityRevision
3. 3 active→retired
4. 4 取消相关build并事件失效
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：active失活；qa缓存通过事件失效

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:cancelJob

## knowledge.NormalizeQuery

`func NormalizeQuery(ctx context.Context, op OperationContext, input NormalizeQueryInput) (NormalizeQueryResult, error)`

作用：规范检索请求

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: NormalizeQueryInput`：封闭业务输入；逐字段见types $defs/NormalizeQueryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| query | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 用户检索词 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标语言 |

返回：

- `result: NormalizeQueryResult`：成功结果；逐字段见types $defs/NormalizeQueryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| normalized | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范query |
| queryHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范摘要 |

算法：

1. 1 NFC/LF
2. 2 trim首尾空白但保留代码大小写
3. 3 上限2048字符/8192字节
4. 4 空查询拒绝
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.EmbedQuery

`func EmbedQuery(ctx context.Context, op OperationContext, input EmbedQueryInput) (EmbedQueryResult, error)`

作用：取得匹配空间查询向量

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: EmbedQueryInput`：封闭业务输入；逐字段见types $defs/EmbedQueryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| normalized | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 查询 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 所查空间 |
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 嵌入档位 |

返回：

- `result: EmbedQueryResult`：成功结果；逐字段见types $defs/EmbedQueryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| vector | 是 | `{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536}` | 查询向量 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 确认空间 |

算法：

1. 1 检查MG档位空间完全等同
2. 2 ctx继承调用
3. 3 dimension与有限数检查
4. 4 取消后不返回向量
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG可能收费；不改变文档索引

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## knowledge.SearchLexical

`func SearchLexical(ctx context.Context, op OperationContext, input SearchLexicalInput) (SearchLexicalResult, error)`

作用：执行全文召回

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SearchLexicalInput`：封闭业务输入；逐字段见types $defs/SearchLexicalInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| normalized | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 词项查询 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..80 |

返回：

- `result: SearchLexicalResult`：成功结果；逐字段见types $defs/SearchLexicalResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | BM25候选 |

算法：

1. 1 只读active generations快照
2. 2 BM25 k1=1.2 b=0.75
3. 3 按score降序chunkId稳定排序
4. 4 取最多80项
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读，候选此阶段未授权不能外发

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.SearchVector

`func SearchVector(ctx context.Context, op OperationContext, input SearchVectorInput) (SearchVectorResult, error)`

作用：执行向量召回

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SearchVectorInput`：封闭业务输入；逐字段见types $defs/SearchVectorInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| vector | 是 | `{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536}` | 查询向量 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..80 |

返回：

- `result: SearchVectorResult`：成功结果；逐字段见types $defs/SearchVectorResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | cosine候选 |

算法：

1. 1 空间完全匹配
2. 2 cosine距离并排除零向量
3. 3 只active索引
4. 4 分数降序chunkId稳定排序最多80
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读；不得跨space补足候选

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.FuseCandidates

`func FuseCandidates(ctx context.Context, op OperationContext, input FuseCandidatesInput) (FuseCandidatesResult, error)`

作用：确定性融合两种召回

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: FuseCandidatesInput`：封闭业务输入；逐字段见types $defs/FuseCandidatesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| lexical | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 词项候选 |
| vector | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 向量候选 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..40 |

返回：

- `result: FuseCandidatesResult`：成功结果；逐字段见types $defs/FuseCandidatesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | RRF合并候选 |

算法：

1. 1 以chunkId去重
2. 2 score=1/(60+lexicalRank)+1/(60+vectorRank)缺失项0
3. 3 hash冲突拒绝
4. 4 score/chunkId稳定排序40项
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.ValidateCandidates

`func ValidateCandidates(ctx context.Context, op OperationContext, input ValidateCandidatesInput) (ValidateCandidatesResult, error)`

作用：权威过滤检索材料

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateCandidatesInput`：封闭业务输入；逐字段见types $defs/ValidateCandidatesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 未授权候选 |

返回：

- `result: ValidateCandidatesResult`：成功结果；逐字段见types $defs/ValidateCandidatesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 当前可见候选 |

算法：

1. 1 按PublishedRevisionRef去重
2. 2 content批量validate
3. 3 visible且matchesCurrent且dataEpoch一致才留
4. 4 写visibilityRevision并拒绝不可用结果
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读；不靠本地撤回事件兜底授权

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## knowledge.RerankCandidates

`func RerankCandidates(ctx context.Context, op OperationContext, input RerankCandidatesInput) (RerankCandidatesResult, error)`

作用：可选AI重排

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RerankCandidatesInput`：封闭业务输入；逐字段见types $defs/RerankCandidatesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| query | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 用户查询 |
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 已过滤材料 |
| profileId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 可选重排档位 |

返回：

- `result: RerankCandidatesResult`：成功结果；逐字段见types $defs/RerankCandidatesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 重排后材料 |
| degraded | 是 | `{"type":"boolean"}` | 未配置或安全降级 |

算法：

1. 1 使用rerank档，未设时fast核查排序
2. 2 同候选最多20个且固定ID
3. 3 继承qa root/costClass与共享截止
4. 4 输出只能既有ID排序，最多一次格式重试再有序fallback每次独立MG预留。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：可选MG费用；不写索引

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## knowledge.SelectContext

`func SelectContext(ctx context.Context, op OperationContext, input SelectContextInput) (SelectContextResult, error)`

作用：选择答案材料

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SelectContextInput`：封闭业务输入；逐字段见types $defs/SelectContextInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 排序材料 |
| maximumTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":4096}` | 材料预算默认4096 |
| maximumChunks | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":8}` | 默认8上限20 |

返回：

- `result: SelectContextResult`：成功结果；逐字段见types $defs/SelectContextResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 预算内材料 |
| truncated | 是 | `{"type":"boolean"}` | 预算截断 |

算法：

1. 1 每文档最多3块
2. 2 按排序累计token
3. 3 超预算跳过整块不截断引用偏移
4. 4 记录truncated
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.Retrieve

`func Retrieve(ctx context.Context, op OperationContext, input RetrieveInput) (RetrieveResult, error)`

作用：统一混合检索

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RetrieveInput`：封闭业务输入；逐字段见types $defs/RetrieveInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| query | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 完整基准语言查询 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 必须基准语言 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |
| embedding | 否；省略不传null | `{"$ref":"#/$defs/QueryEmbedding"}` | QA本轮可复用向量 |
| maximumTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 材料预算 |
| lexicalOnly | 是 | `{"type":"boolean"}` | 显式词项 |

返回：

- `result: RetrieveResult`：成功结果；逐字段见types $defs/RetrieveResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| result | 是 | `{"$ref":"#/$defs/SearchResult"}` | 可引用结果 |

算法：

1. 1 query与base language规范
2. 2 embedding存在时校验queryHash/space/vectorHash/epoch后复用，否则MG生成
3. 3 lexical/vector各80→RRF40→可选rerank fast fallback
4. 4 当前页权威范围可优先且最终content可见性validate。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：仅query embedding/rerank可能MG收费

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, model-gateway:createInference, memory:resolveContext

## knowledge.GetBuild

`func GetBuild(ctx context.Context, op OperationContext, input GetBuildInput) (GetBuildResult, error)`

作用：读取入库状态

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: GetBuildInput`：封闭业务输入；逐字段见types $defs/GetBuildInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |

返回：

- `result: GetBuildResult`：成功结果；逐字段见types $defs/GetBuildResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| build | 是 | `{"$ref":"#/$defs/IndexBuild"}` | 状态 |

算法：

1. 1 验证knowledge.manage
2. 2 读build快照
3. 3 不返回源全文
4. 4 状态未知不自动重跑
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.ListBuilds

`func ListBuilds(ctx context.Context, op OperationContext, input ListBuildsInput) (ListBuildsResult, error)`

作用：列入库任务

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListBuildsInput`：封闭业务输入；逐字段见types $defs/ListBuildsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| documentId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 文档过滤 |
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: ListBuildsResult`：成功结果；逐字段见types $defs/ListBuildsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| builds | 是 | `{"type":"array","items":{"$ref":"#/$defs/IndexBuild"},"maxItems":256}` | 状态记录 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |

算法：

1. 1 权限检查
2. 2 按buildId稳定keyset
3. 3 limit默认20
4. 4 返回source ref与错误码
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.ConsumeInbox

`func ConsumeInbox(ctx context.Context, op OperationContext, input ConsumeInboxInput) (ConsumeInboxResult, error)`

作用：事件去重与代际保护

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ConsumeInboxInput`：封闭业务输入；逐字段见types $defs/ConsumeInboxInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 事件 |
| eventType | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 已登记类型 |
| payloadHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 摘要 |
| dataEpoch | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 事件代际 |

返回：

- `result: ConsumeInboxResult`：成功结果；逐字段见types $defs/ConsumeInboxResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| duplicate | 是 | `{"type":"boolean"}` | 已消费 |
| accepted | 是 | `{"type":"boolean"}` | 允许处理 |

算法：

1. 1 校验事件类型/schema
2. 2 dataEpoch==runtime
3. 3 eventId/hash唯一
4. 4 同事务登记业务执行结果
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写inbox

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.PublishOutbox

`func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)`

作用：发布索引生命周期事件

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PublishOutboxInput`：封闭业务输入；逐字段见types $defs/PublishOutboxInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |
| workerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 工作者 |

返回：

- `result: PublishOutboxResult`：成功结果；逐字段见types $defs/PublishOutboxResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| published | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 确认数 |
| pending | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 待发数 |

算法：

1. 1 租约领取批次
2. 2 jobs持久发布
3. 3 ACK才标记
4. 4 有限三次退避后留待下一轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：at-least-once持久事件

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:publishEvent

## knowledge.RecoverBuildLeases

`func RecoverBuildLeases(ctx context.Context, op OperationContext, input RecoverBuildLeasesInput) (RecoverBuildLeasesResult, error)`

作用：恢复入库执行租约

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RecoverBuildLeasesInput`：封闭业务输入；逐字段见types $defs/RecoverBuildLeasesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| now | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 时间 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: RecoverBuildLeasesResult`：成功结果；逐字段见types $defs/RecoverBuildLeasesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| requeued | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 重排队数 |
| superseded | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 过期修订数 |

算法：

1. 1 到期worker停止写
2. 2 content验证当前发布
3. 3 已失效标superseded
4. 4 确定性已完成阶段复用，仅缺失embedding在原调用可确证时重续
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：更新build/lease和jobs；不重复未知收费调用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, jobs:createJob

## knowledge.CollectRetiredIndexes

`func CollectRetiredIndexes(ctx context.Context, op OperationContext, input CollectRetiredIndexesInput) (CollectRetiredIndexesResult, error)`

作用：回收失活索引代数

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CollectRetiredIndexesInput`：封闭业务输入；逐字段见types $defs/CollectRetiredIndexesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| olderThan | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 至少24小时 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: CollectRetiredIndexesResult`：成功结果；逐字段见types $defs/CollectRetiredIndexesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| deleted | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 回收代数 |
| retained | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 仍被引用数 |

算法：

1. 1 检查retired且无reader lease
2. 2 无未完成build引用
3. 3 删除本服务索引文件并校验路径在私有目录
4. 4 删除元数据事务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：删除无引用派生数据；不删除权威正文

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.RedactDiagnostics

`func RedactDiagnostics(ctx context.Context, op OperationContext, input RedactDiagnosticsInput) (RedactDiagnosticsResult, error)`

作用：返回安全诊断

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RedactDiagnosticsInput`：封闭业务输入；逐字段见types $defs/RedactDiagnosticsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| includeCounts | 是 | `{"type":"boolean"}` | 计数开关 |

返回：

- `result: RedactDiagnosticsResult`：成功结果；逐字段见types $defs/RedactDiagnosticsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建 |
| state | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 状态 |
| counts | 是 | `{"type":"array","items":{"type":"integer","minimum":0,"maximum":9007199254740991},"maxItems":65536}` | 分块向量词项计数 |
| spaceId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 空间摘要 |

算法：

1. 1 只返状态/计数/版本
2. 2 不返全文或媒体凭证
3. 3 检查权限
4. 4 固定诊断无模型探测
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.ReadSections

`func ReadSections(ctx context.Context, op OperationContext, input ReadSectionsInput) (ReadSectionsResult, error)`

作用：按引用读取证据片段

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadSectionsInput`：封闭业务输入；逐字段见types $defs/ReadSectionsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| chunkIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 最多20个检索块 |
| documentRefs | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"},"maxItems":256}` | 调用所见来源 |

返回：

- `result: ReadSectionsResult`：成功结果；逐字段见types $defs/ReadSectionsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| chunks | 是 | `{"type":"array","items":{"$ref":"#/$defs/Chunk"},"maxItems":256}` | 当前可见分块 |

算法：

1. 1 service身份
2. 2 chunk必须active且来源与传入ref相同
3. 3 content批量校验current/visible/epoch
4. 4 返回完整sourceOffsets与hash
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## knowledge.AcquireBarrier

`func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)`

作用：取得本地维护屏障

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BarrierRequest`：封闭业务输入；逐字段见types $defs/BarrierRequest；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

引用：`../../../contracts/common.schema.json#/$defs/BarrierRequest`。

返回：

- `result: BarrierStatus`：成功结果；逐字段见types $defs/BarrierStatus；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/BarrierStatus`。

算法：

1. 1 校验服务身份及superadmin委托或限定RecoveryAuthorization
2. 2 将本地control.db屏障draining并拒绝新业务写
3. 3 屏障停止新build、active切换和staging写入，等待reader lease排空
4. 导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建
5. 派生索引可由管理员另发重建任务。
6. 4 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。
7. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：持久控制屏障；不纳入业务快照

授权：service身份+mTLS和superadmin预授权；正常控制调用要求当前epoch，恢复控制只接受op.recoveryAuthorization限定的restoreId/barrierId/sourceEpoch/targetEpoch/audience/expiresAt及预冻结信任锚；本地control.db不被导入覆盖。

并发：本域control.db按barrierId/restoreId串行CAS；业务worker屏障排空，stage指针切换独占锁，reader固定代数；恢复许可不解除其他业务权限。

幂等：相同barrierId/stageId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；commit/rollback均可安全重放。

依赖：site-config:readRuntimeState

## knowledge.ReleaseBarrier

`func ReleaseBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)`

作用：释放已授权维护屏障

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BarrierRequest`：封闭业务输入；逐字段见types $defs/BarrierRequest；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

引用：`../../../contracts/common.schema.json#/$defs/BarrierRequest`。

返回：

- `result: BarrierStatus`：成功结果；逐字段见types $defs/BarrierStatus；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/BarrierStatus`。

算法：

1. 1 校验相同barrierId/operationId恢复许可
2. 2 不允许未决prepared stage直接释放
3. 3 runtime dataEpoch与最终commit/rollback结果一致
4. 4 released后开放该epoch业务写。
5. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：更新本地control.db；不修改导入业务

授权：service身份+mTLS和superadmin预授权；正常控制调用要求当前epoch，恢复控制只接受op.recoveryAuthorization限定的restoreId/barrierId/sourceEpoch/targetEpoch/audience/expiresAt及预冻结信任锚；本地control.db不被导入覆盖。

并发：本域control.db按barrierId/restoreId串行CAS；业务worker屏障排空，stage指针切换独占锁，reader固定代数；恢复许可不解除其他业务权限。

幂等：相同barrierId/stageId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；commit/rollback均可安全重放。

依赖：site-config:readRuntimeState

## knowledge.ExportSnapshot

`func ExportSnapshot(ctx context.Context, op OperationContext, input SnapshotExportRequest) (SnapshotExportResult, error)`

作用：导出安全watermark快照

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SnapshotExportRequest`：封闭业务输入；逐字段见types $defs/SnapshotExportRequest；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotExportRequest`。

返回：

- `result: SnapshotExportResult`：成功结果；逐字段见types $defs/SnapshotExportResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotExportResult`。

算法：

1. 1 验证safe屏障及dataEpoch
2. 2 checkpoint本域私有数据并固定revisionWatermark
3. 3 屏障停止新build、active切换和staging写入，等待reader lease排空
4. 导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建
5. 派生索引可由管理员另发重建任务。
6. 4 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。
7. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：创建本服务不可变快照对象

授权：service身份+mTLS和superadmin预授权；正常控制调用要求当前epoch，恢复控制只接受op.recoveryAuthorization限定的restoreId/barrierId/sourceEpoch/targetEpoch/audience/expiresAt及预冻结信任锚；本地control.db不被导入覆盖。

并发：本域control.db按barrierId/restoreId串行CAS；业务worker屏障排空，stage指针切换独占锁，reader固定代数；恢复许可不解除其他业务权限。

幂等：相同barrierId/stageId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；commit/rollback均可安全重放。

依赖：site-config:readRuntimeState

## knowledge.ValidateSnapshot

`func ValidateSnapshot(ctx context.Context, op OperationContext, input SnapshotValidateRequest) (SnapshotValidation, error)`

作用：只读检查候选快照

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SnapshotValidateRequest`：封闭业务输入；逐字段见types $defs/SnapshotValidateRequest；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotValidateRequest`。

返回：

- `result: SnapshotValidation`：成功结果；逐字段见types $defs/SnapshotValidation；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotValidation`。

算法：

1. 1 校验owner/对象SHA256/byteSize且不接受本地任意路径
2. 2 schemaVersion仅支持1否则UNSUPPORTED_SCHEMA
3. 3 检查本域字段/唯一约束/引用/密钥兼容性
4. 4 返回field issues无业务写，任何integrity失败valid=false。
5. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：仅记录验证审计；不载入active数据

授权：service身份+mTLS和superadmin预授权；正常控制调用要求当前epoch，恢复控制只接受op.recoveryAuthorization限定的restoreId/barrierId/sourceEpoch/targetEpoch/audience/expiresAt及预冻结信任锚；本地control.db不被导入覆盖。

并发：本域control.db按barrierId/restoreId串行CAS；业务worker屏障排空，stage指针切换独占锁，reader固定代数；恢复许可不解除其他业务权限。

幂等：相同barrierId/stageId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；commit/rollback均可安全重放。

依赖：site-config:readRuntimeState

## knowledge.PrepareSnapshot

`func PrepareSnapshot(ctx context.Context, op OperationContext, input SnapshotPrepareRequest) (SnapshotStageResult, error)`

作用：隔离准备恢复数据

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SnapshotPrepareRequest`：封闭业务输入；逐字段见types $defs/SnapshotPrepareRequest；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotPrepareRequest`。

返回：

- `result: SnapshotStageResult`：成功结果；逐字段见types $defs/SnapshotStageResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotStageResult`。

算法：

1. 1 验证限定restoreId/barrierId/旧新epoch/audience/期限许可及safe屏障
2. 2 重复validate并读可信ObjectRef
3. 3 写独立staging DB/文件且不覆盖control.db、屏障、信任锚
4. 4 staging checksum一致才ready并记录恢复前指针。
5. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：创建隔离stage及本地control恢复记录

授权：service身份+mTLS和superadmin预授权；正常控制调用要求当前epoch，恢复控制只接受op.recoveryAuthorization限定的restoreId/barrierId/sourceEpoch/targetEpoch/audience/expiresAt及预冻结信任锚；本地control.db不被导入覆盖。

并发：本域control.db按barrierId/restoreId串行CAS；业务worker屏障排空，stage指针切换独占锁，reader固定代数；恢复许可不解除其他业务权限。

幂等：相同barrierId/stageId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；commit/rollback均可安全重放。

依赖：site-config:readRuntimeState

## knowledge.CommitSnapshot

`func CommitSnapshot(ctx context.Context, op OperationContext, input SnapshotCommitRequest) (SnapshotApplyResult, error)`

作用：原子激活隔离恢复数据

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SnapshotCommitRequest`：封闭业务输入；逐字段见types $defs/SnapshotCommitRequest；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotCommitRequest`。

返回：

- `result: SnapshotApplyResult`：成功结果；逐字段见types $defs/SnapshotApplyResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotApplyResult`。

算法：

1. 1 校验stage属于许可restore且目标epoch匹配
2. 2 safe屏障下禁止业务worker
3. 3 原子切换本域数据指针并持久旧指针
4. 4 committed幂等回执，屏障仍safe等待全域runtime epoch激活。
5. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：原子active指针切换；保留rollback数据

授权：service身份+mTLS和superadmin预授权；正常控制调用要求当前epoch，恢复控制只接受op.recoveryAuthorization限定的restoreId/barrierId/sourceEpoch/targetEpoch/audience/expiresAt及预冻结信任锚；本地control.db不被导入覆盖。

并发：本域control.db按barrierId/restoreId串行CAS；业务worker屏障排空，stage指针切换独占锁，reader固定代数；恢复许可不解除其他业务权限。

幂等：相同barrierId/stageId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；commit/rollback均可安全重放。

依赖：site-config:readRuntimeState

## knowledge.RollbackSnapshot

`func RollbackSnapshot(ctx context.Context, op OperationContext, input SnapshotCommitRequest) (SnapshotApplyResult, error)`

作用：回退未完成全域恢复

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SnapshotCommitRequest`：封闭业务输入；逐字段见types $defs/SnapshotCommitRequest；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotCommitRequest`。

返回：

- `result: SnapshotApplyResult`：成功结果；逐字段见types $defs/SnapshotApplyResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/SnapshotApplyResult`。

算法：

1. 1 验证同一许可restore/barrier
2. 2 读取control保存旧指针
3. 3 原子恢复旧数据且stage rolled-back
4. 4 回执旧dataEpoch并保持safe，重复rollback返回同回执。
5. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：回退本域active指针；保留审计及失败stage

授权：service身份+mTLS和superadmin预授权；正常控制调用要求当前epoch，恢复控制只接受op.recoveryAuthorization限定的restoreId/barrierId/sourceEpoch/targetEpoch/audience/expiresAt及预冻结信任锚；本地control.db不被导入覆盖。

并发：本域control.db按barrierId/restoreId串行CAS；业务worker屏障排空，stage指针切换独占锁，reader固定代数；恢复许可不解除其他业务权限。

幂等：相同barrierId/stageId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；commit/rollback均可安全重放。

依赖：site-config:readRuntimeState

## knowledge.LoadAdminOverview

`export async function loadAdminOverview(input: LoadAdminOverviewInput, signal: AbortSignal): Promise<LoadAdminOverviewResult>`

作用：加载本域后台概览

参数：

- `input: LoadAdminOverviewInput`：本域封闭输入；每个字段规格见types.json $defs/LoadAdminOverviewInput；schema required/additionalProperties/range全部强制；null禁止。
- `signal: AbortSignal`：界面导航及取消信号；取消后不更新已卸载组件。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 分页游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: LoadAdminOverviewResult`：成功结果；逐字段见types $defs/LoadAdminOverviewResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| items | 是 | `{"type":"array","items":{"$ref":"#/$defs/IndexConfig"},"maxItems":256}` | 后台概览条目 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |

算法：

1. 1 portal提供已认证apiClient
2. 2 读取本域管理API
3. 3 取消旧navigation请求
4. 4 基于server version显示状态且不在UI推测费用或任务成功
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读请求与组件状态更新

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.SubmitPrimaryEdit

`export async function submitPrimaryEdit(input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult>`

作用：提交本域明确人工修改

参数：

- `input: SubmitPrimaryEditInput`：本域封闭输入；每个字段规格见types.json $defs/SubmitPrimaryEditInput；schema required/additionalProperties/range全部强制；null禁止。
- `signal: AbortSignal`：界面导航及取消信号；取消后不更新已卸载组件。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| config | 是 | `{"$ref":"#/$defs/IndexConfig"}` | 新配置 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SubmitPrimaryEditResult`：成功结果；逐字段见types $defs/SubmitPrimaryEditResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| config | 是 | `{"$ref":"#/$defs/IndexConfig"}` | 新版本 |

算法：

1. 1 表单逐字段验证本域schema
2. 2 生成幂等键并携expectedVersion
3. 3 调本域putIndexConfig API
4. 4 409保留用户输入并显示当前版本，不自动覆盖重发
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：本域HTTP写请求和组件状态更新

授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## knowledge.EnrichChunkContext

`func EnrichChunkContext(ctx context.Context, op OperationContext, input EnrichChunkContextInput) (EnrichChunkContextResult, error)`

作用：发布后补检索片段上下文

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: EnrichChunkContextInput`：封闭业务输入；逐字段见types $defs/EnrichChunkContextInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 发布构建 |
| chunks | 是 | `{"type":"array","items":{"$ref":"#/$defs/Chunk"},"maxItems":256}` | 片段 |
| documentText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 整篇原文 |
| memory | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/MemoryContext"}` | 项目概述 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: EnrichChunkContextResult`：成功结果；逐字段见types $defs/EnrichChunkContextResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| prefixes | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 等长1至2句背景 |
| contextVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 源及任务记忆版本 |

算法：

1. 1 task knowledge.context使用fast档
2. 2 同文整篇原文为固定输入前缀，最多16片段一批
3. 3 生成1至2句产品/功能/小节背景不改变源正文
4. 4 按chunkHash+documentHash+memory版本幂等复用，只为变更块付费
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用及machine derived context记录

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference, memory:resolveContext

## knowledge.InspectEvidence

`func InspectEvidence(ctx context.Context, op OperationContext, input InspectEvidenceInput) (InspectEvidenceResult, error)`

作用：快速检索员有限核查

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: InspectEvidenceInput`：封闭业务输入；逐字段见types $defs/InspectEvidenceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| plan | 是 | `{"$ref":"#/$defs/QuestionPlan"}` | 问题计划 |
| candidateIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | knowledge候选 |
| round | 是 | `{"type":"integer","minimum":1,"maximum":3}` | 1..3 |
| toolCallsUsed | 是 | `{"type":"integer","minimum":0,"maximum":5}` | 0..5 |

返回：

- `result: InspectEvidenceResult`：成功结果；逐字段见types $defs/InspectEvidenceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| selectedIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 相关ID |
| additionalQueries | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 最多剩余子查询 |
| expandSectionIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 需展开小节 |
| finished | 是 | `{"type":"boolean"}` | 停止 |

算法：

1. 1 根task qa.inspect owner=knowledge fast档max800output token并注入project overview
2. 2 只选择本次检索合法IDs，展开小节仍content当前校验
3. 3 3轮/5工具调用计数来自可信根任务状态CAS不由浏览器覆盖
4. 4 到上限用已有材料返回finished，费用继承qa costClass。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG核查费用及可选knowledge工具调用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference, content:validatePublications, memory:resolveContext

## knowledge.CreateQueryEmbedding

`func CreateQueryEmbedding(ctx context.Context, op OperationContext, input CreateQueryEmbeddingInput) (CreateQueryEmbeddingResult, error)`

作用：统一生成并复用查询嵌入

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateQueryEmbeddingInput`：封闭业务输入；逐字段见types $defs/CreateQueryEmbeddingInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| query | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 完整基准查询 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | rootOperation/queryHash/space一致键 |

返回：

- `result: CreateQueryEmbeddingResult`：成功结果；逐字段见types $defs/CreateQueryEmbeddingResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| embedding | 是 | `{"$ref":"#/$defs/QueryEmbedding"}` | 可复用向量 |

算法：

1. 1 rootOperationId/queryHash/space/dataEpoch singleflight且唯一
2. 2 根knowledge.embed embedding档0文本output
3. 3 MG保留独立attempt费用证据
4. 4 返回完整空间/vectorHash供QA cache与本域search复用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用与本根短期向量记录，TTL10分钟

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## knowledge.DescribeAttachment

`func DescribeAttachment(ctx context.Context, op OperationContext, input DescribeAttachmentInput) (DescribeAttachmentResult, error)`

作用：发布附件描述与非文本提取

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: DescribeAttachmentInput`：封闭业务输入；逐字段见types $defs/DescribeAttachmentInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extractionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提取任务 |
| asset | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 附件 |
| sourceText | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 本地可提取文本 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: DescribeAttachmentResult`：成功结果；逐字段见types $defs/DescribeAttachmentResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extraction | 是 | `{"$ref":"#/$defs/Extraction"}` | 描述加可检索文本 |

算法：

1. 1 支持文件MIME/大小白名单且media授权
2. 2 文本附件根knowledge.describe fast档800tokens
3. 3 非文本附件根knowledge.extract-attachment vision文件能力1500tokens，不支持明确unsupported
4. 4 assetHash/extractor/model/memory版本复用，描述完整再长文分块。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNSUPPORTED_CAPABILITY (HTTP422)：声明能力或资源提取类型不受支持。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG可能收费及派生文本，不修改或翻译附件本体

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：media:readAsset, model-gateway:createInference, memory:resolveContext

## knowledge.RunWorker

`func RunWorker(ctx context.Context, op OperationContext, input RunWorkerInput) (RunWorkerResult, error)`

作用：主动领取本域持久任务

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RunWorkerInput`：封闭业务输入；逐字段见types $defs/RunWorkerInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| workerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 本域worker |
| maximumConcurrentJobs | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":4}` | 默认4范围1..32 |

返回：

- `result: RunWorkerResult`：成功结果；逐字段见types $defs/RunWorkerResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| claimed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 本轮领取 |
| completed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 成功 |
| failed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 失败 |
| cancelled | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 取消 |

算法：

1. 1 向jobs claimJob仅领取本域JOB_HANDLERS登记jobType
2. 2 每10秒heartbeatJob续30秒租约并检查cancel/deadline
3. 3 依jobType执行同域已登记函数，所有中间/提交校验lease与epoch
4. 4 completeJob/failJob持久报告，lease过期停止写，未知MG attempt留held不重放。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：jobs租约/完成状态及本域工作链副作用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:claimJob, jobs:heartbeatJob, jobs:completeJob, jobs:failJob

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M08-F901 main

文件 `modules/knowledge/cmd/knowledge/main.go`。独立进程可执行入口

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

### M08-F902 BuildApplication

文件 `modules/knowledge/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M08-F903 Serve

文件 `modules/knowledge/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M08-F904 Shutdown

文件 `modules/knowledge/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M08-F905 Health

文件 `modules/knowledge/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M08-F906 Ready

文件 `modules/knowledge/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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

### M08-F907 mountModuleUI

文件 `modules/knowledge/ui/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

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
