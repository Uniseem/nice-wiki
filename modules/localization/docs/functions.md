# 语言、翻译与术语逐函数实现合同

每一函数与implementation.json一一对应；输入和结果的全部字段见本节对应JSON Schema，公共类型见根common.schema.json。

## localization.LoadRuntime

`func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)`

作用：启动本地化服务

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: LoadRuntimeInput`：封闭业务输入；逐字段见types $defs/LoadRuntimeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| serviceId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 身份 |
| dataDirectory | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 私有目录 |

返回：

- `result: LoadRuntimeResult`：成功结果；逐字段见types $defs/LoadRuntimeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| ready | 是 | `{"type":"boolean"}` | 就绪 |
| glossaryGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 术语代数 |

算法：

1. 1 检查语言字典schema
2. 2 校验默认语言唯一
3. 3 加载术语代数
4. 4 注册旧租约恢复
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

副作用：打开私有存储不自动翻译

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.NormalizeLanguageTag

`func NormalizeLanguageTag(ctx context.Context, op OperationContext, input NormalizeLanguageTagInput) (NormalizeLanguageTagResult, error)`

作用：规范语言标识

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: NormalizeLanguageTagInput`：封闭业务输入；逐字段见types $defs/NormalizeLanguageTagInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| tag | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 输入BCP47标签 |

返回：

- `result: NormalizeLanguageTagResult`：成功结果；逐字段见types $defs/NormalizeLanguageTagResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| tag | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范标签 |
| direction | 是 | `{"type":"string","enum":["ltr","rtl"]}` | 方向 |

算法：

1. 1 拒绝空/长度>35
2. 2 按BCP47解析大小写
3. 3 禁止私用未知UI语言
4. 4 已支持ar/he/fa/ur为rtl其余ltr
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
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

## localization.PutLanguage

`func PutLanguage(ctx context.Context, op OperationContext, input PutLanguageInput) (PutLanguageResult, error)`

作用：更新语言启用配置

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutLanguageInput`：封闭业务输入；逐字段见types $defs/PutLanguageInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: PutLanguageResult`：成功结果；逐字段见types $defs/PutLanguageResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"$ref":"#/$defs/Language"}` | 语言配置 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"$ref":"#/$defs/Language"}` | 保存配置 |

算法：

1. 1 NormalizeLanguageTag
2. 2 profile仅声明校验不探测模型
3. 3 默认语言不能disabled
4. 4 CAS保存
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：语言配置事件；不直接发翻译任务

授权：权限 localization.languages；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:readProfiles

## localization.SetLanguageEnabled

`func SetLanguageEnabled(ctx context.Context, op OperationContext, input SetLanguageEnabledInput) (SetLanguageEnabledResult, error)`

作用：启停公开语言

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SetLanguageEnabledInput`：封闭业务输入；逐字段见types $defs/SetLanguageEnabledInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: SetLanguageEnabledResult`：成功结果；逐字段见types $defs/SetLanguageEnabledResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| tag | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| enabled | 是 | `{"type":"boolean"}` | 状态 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"$ref":"#/$defs/Language"}` | 新状态 |

算法：

1. 1 默认语言停用拒绝
2. 2 enabled保存状态
3. 3 disabled取消该语言未完成翻译
4. 4 保留人工译文历史
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：配置变化和取消事件；不删除人工文本

授权：权限 localization.languages；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:cancelJob

## localization.ListLanguages

`func ListLanguages(ctx context.Context, op OperationContext, input ListLanguagesInput) (ListLanguagesResult, error)`

作用：读取语言列表

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListLanguagesInput`：封闭业务输入；逐字段见types $defs/ListLanguagesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| includeDisabled | 是 | `{"type":"boolean"}` | 管理视图开关 |

返回：

- `result: ListLanguagesResult`：成功结果；逐字段见types $defs/ListLanguagesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| languages | 是 | `{"type":"array","items":{"$ref":"#/$defs/Language"},"maxItems":256}` | 语言配置 |

算法：

1. 1 公共只启用语言
2. 2 includeDisabled需localization.manage
3. 3 默认语言先其余tag排序
4. 4 禁止返回模型密钥
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.HandlePublication

`func HandlePublication(ctx context.Context, op OperationContext, input HandlePublicationInput) (HandlePublicationResult, error)`

作用：仅发布触发已配置翻译

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: HandlePublicationInput`：封闭业务输入；逐字段见types $defs/HandlePublicationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 事件 |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 源发布引用 |

返回：

- `result: HandlePublicationResult`：成功结果；逐字段见types $defs/HandlePublicationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| accepted | 是 | `{"type":"boolean"}` | 已消费 |
| translationIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 新任务 |

算法：

1. 1 inbox去重/epoch校验
2. 2 按enabled且automatic且profile已设语言
3. 3 固定源revision和各generation
4. 4 与jobs任务同事务
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

副作用：翻译任务outbox；未配置则skipped记录

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:createJob, content:validatePublications

## localization.CreateTranslation

`func CreateTranslation(ctx context.Context, op OperationContext, input CreateTranslationInput) (CreateTranslationResult, error)`

作用：申请源修订翻译

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateTranslationInput`：封闭业务输入；逐字段见types $defs/CreateTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 源发布 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标语言 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CreateTranslationResult`：成功结果；逐字段见types $defs/CreateTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/Translation"}` | queued |

算法：

1. 1 验证language启用和profile
2. 2 content当前可见
3. 3 固定术语/memory/human代数
4. 4 source/target/generation唯一排队
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：创建派生任务不修改正文

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, memory:resolveContext, jobs:createJob

## localization.FetchTranslationSource

`func FetchTranslationSource(ctx context.Context, op OperationContext, input FetchTranslationSourceInput) (FetchTranslationSourceResult, error)`

作用：读取翻译源

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: FetchTranslationSourceInput`：封闭业务输入；逐字段见types $defs/FetchTranslationSourceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: FetchTranslationSourceResult`：成功结果；逐字段见types $defs/FetchTranslationSourceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| source | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 权威正文 |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 源引用 |

算法：

1. 1 校验租约
2. 2 content readPublishedRevision
3. 3 hash/schema匹配
4. 4 分块只读不复制草稿
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

副作用：私有临时输入引用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:readPublishedRevision

## localization.SplitTranslationUnits

`func SplitTranslationUnits(ctx context.Context, op OperationContext, input SplitTranslationUnitsInput) (SplitTranslationUnitsResult, error)`

作用：确定性切分翻译单元

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SplitTranslationUnitsInput`：封闭业务输入；逐字段见types $defs/SplitTranslationUnitsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 源 |
| source | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 正文 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |

返回：

- `result: SplitTranslationUnitsResult`：成功结果；逐字段见types $defs/SplitTranslationUnitsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 片段模板 |

算法：

1. 1 沿用content稳定blockId与完整块树
2. 2 inline粗体/链接/代码转编号标签，URL/code值保留在本服务不发模型
3. 3 code块/公式/嵌入不翻译但image alt/caption翻译
4. 4 单块超预算仅table按行拆并保留父blockId，最终还原树。
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

## localization.PlanIncrementalTranslation

`func PlanIncrementalTranslation(ctx context.Context, op OperationContext, input PlanIncrementalTranslationInput) (PlanIncrementalTranslationResult, error)`

作用：增量选择待翻译单元

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PlanIncrementalTranslationInput`：封闭业务输入；逐字段见types $defs/PlanIncrementalTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 新源片段 |

返回：

- `result: PlanIncrementalTranslationResult`：成功结果；逐字段见types $defs/PlanIncrementalTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| reusedIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 源hash未变复用 |
| translateIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 待机翻 |
| protectedIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 人工保护 |

算法：

1. 1 源hash没变复用
2. 2 changed/new块先同源hash且≥20字translation memory匹配
3. 3 human/protected源变不覆盖只review
4. 4 未复用块按1000预计输出token均衡顺序分批。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写plan与stale人工提示不覆盖译文

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.ResolveGlossary

`func ResolveGlossary(ctx context.Context, op OperationContext, input ResolveGlossaryInput) (ResolveGlossaryResult, error)`

作用：取得确定性术语快照

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResolveGlossaryInput`：封闭业务输入；逐字段见types $defs/ResolveGlossaryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sourceLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 源语言 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |
| documentId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 文档 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 固定代数 |

返回：

- `result: ResolveGlossaryResult`：成功结果；逐字段见types $defs/ResolveGlossaryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| terms | 是 | `{"type":"array","items":{"$ref":"#/$defs/GlossaryTerm"},"maxItems":256}` | 有效术语 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 固定代数 |

算法：

1. 1 固定generation读取
2. 2 document term优先global
3. 3 原词按长度降序避免短词抢占
4. 4 相同scope原词冲突拒绝
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
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

## localization.BuildTranslationPrompt

`func BuildTranslationPrompt(ctx context.Context, op OperationContext, input BuildTranslationPromptInput) (BuildTranslationPromptResult, error)`

作用：构造有界翻译请求

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BuildTranslationPromptInput`：封闭业务输入；逐字段见types $defs/BuildTranslationPromptInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sourceUnits | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 本批块 |
| documentText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 整篇原文 |
| previousTranslations | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 前批译文 |
| adjacentTranslations | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 相邻块现有译文 |
| sectionTitle | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 本节标题 |
| terms | 是 | `{"type":"array","items":{"$ref":"#/$defs/GlossaryTerm"},"maxItems":256}` | 出现的术语 |
| memory | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/MemoryContext"}` | 翻译注入 |

返回：

- `result: BuildTranslationPromptResult`：成功结果；逐字段见types $defs/BuildTranslationPromptResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 结构化翻译消息 |
| unitIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 输出白名单 |

算法：

1. 1 同文档按顺序批默认1000预计输出tokens，各批尽量均衡且优先标题边界
2. 2 整篇原文固定prefix仅本批标签块翻译，模型窗口不足明确缩上下文
3. 3 前批译文+相邻译文+section title+仅本批出现terms
4. 4 输出blockId完整和格式标签不变。
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

## localization.RunTranslationBatch

`func RunTranslationBatch(ctx context.Context, op OperationContext, input RunTranslationBatchInput) (RunTranslationBatchResult, error)`

作用：执行统一网关翻译

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RunTranslationBatchInput`：封闭业务输入；逐字段见types $defs/RunTranslationBatchInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| unitIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 片段白名单 |
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 请求 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: RunTranslationBatchResult`：成功结果；逐字段见types $defs/RunTranslationBatchResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | MG调用 |
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 候选机翻 |

算法：

1. 1 校验cancel/lease/source计划
2. 2 costClass/background可信继承
3. 3 MG createInference translation
4. 4 解析候选不直接写target
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用；保留调用ID和候选暂存

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## localization.ValidateTranslationOutput

`func ValidateTranslationOutput(ctx context.Context, op OperationContext, input ValidateTranslationOutputInput) (ValidateTranslationOutputResult, error)`

作用：校验模型输出格式保护

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateTranslationOutputInput`：封闭业务输入；逐字段见types $defs/ValidateTranslationOutputInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sourceUnits | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 原单元 |
| translatedUnits | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 返回候选 |

返回：

- `result: ValidateTranslationOutputResult`：成功结果；逐字段见types $defs/ValidateTranslationOutputResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 符合约束 |
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 规范候选 |

算法：

1. 1 blockId齐全唯一且没有未知ID
2. 2 编号inline标签完整有序/数量正确还原原URL及code
3. 3 足够长原文≥20字译文完全相同且不同语言则失败
4. 4 失败明确触发对半→逐块降级，最终失败用原文并后台显示。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
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

## localization.CommitTranslationBatch

`func CommitTranslationBatch(ctx context.Context, op OperationContext, input CommitTranslationBatchInput) (CommitTranslationBatchResult, error)`

作用：提交翻译防旧覆盖

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CommitTranslationBatchInput`：封闭业务输入；逐字段见types $defs/CommitTranslationBatchInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 候选 |
| expectedGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 目标代数 |
| expectedHumanGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 人工代数 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: CommitTranslationBatchResult`：成功结果；逐字段见types $defs/CommitTranslationBatchResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| written | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 写入数 |
| skippedProtected | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 保护数 |

算法：

1. 1 当前sourceRevision/epoch仍匹配
2. 2 CAS术语/memory/target/human代数
3. 3 protected单元跳过
4. 4 一事务写machine文本/进度
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

副作用：写派生译文；CAS失败标superseded

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## localization.CompleteTranslation

`func CompleteTranslation(ctx context.Context, op OperationContext, input CompleteTranslationInput) (CompleteTranslationResult, error)`

作用：完成派生翻译manifest

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CompleteTranslationInput`：封闭业务输入；逐字段见types $defs/CompleteTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| expectedGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 代数 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: CompleteTranslationResult`：成功结果；逐字段见types $defs/CompleteTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/Translation"}` | ready或review |

算法：

1. 1 所有计划单元终态
2. 2 protected且源变需review
3. 3 content当前可见再次检查
4. 4 manifest ready并持久事件
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：translation.ready事件；不更新content正文

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## localization.ReadTranslation

`func ReadTranslation(ctx context.Context, op OperationContext, input ReadTranslationInput) (ReadTranslationResult, error)`

作用：公开读取当前译文

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadTranslationInput`：封闭业务输入；逐字段见types $defs/ReadTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 公开来源 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |

返回：

- `result: ReadTranslationResult`：成功结果；逐字段见types $defs/ReadTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/Translation"}` | 源版本信息 |
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 可交付旧译文或原文 |
| partial | 是 | `{"type":"boolean"}` | 存在原文回退 |
| noticeKey | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 顶部分译提示key |

算法：

1. 1 enabled语言和content当前公开源验证
2. 2 按当前blockId匹配，新译文未就绪显示上一generation译文但标对应源hash
3. 3 新块无译文显示原文且partial顶栏目标语言提示
4. 4 人工源变保留译文review，撤回文档全部禁止交付。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读不隐式启动翻译

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## localization.EditTranslationUnit

`func EditTranslationUnit(ctx context.Context, op OperationContext, input EditTranslationUnitInput) (EditTranslationUnitResult, error)`

作用：人工编辑并自动保护

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: EditTranslationUnitInput`：封闭业务输入；逐字段见types $defs/EditTranslationUnitInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 译文 |
| unitId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 单元 |
| targetText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 人工文本 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 单元版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: EditTranslationUnitResult`：成功结果；逐字段见types $defs/EditTranslationUnitResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| unit | 是 | `{"$ref":"#/$defs/TranslationUnit"}` | human且protected |

算法：

1. 1 权限和插值校验
2. 2 单元CAS
3. 3 origin=human protected=true humanGeneration++
4. 4 取消旧待写批次
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写人工内容与generation事件

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.SetUnitProtection

`func SetUnitProtection(ctx context.Context, op OperationContext, input SetUnitProtectionInput) (SetUnitProtectionResult, error)`

作用：显式启停人工保护

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SetUnitProtectionInput`：封闭业务输入；逐字段见types $defs/SetUnitProtectionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 译文 |
| unitId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 单元 |
| protected | 是 | `{"type":"boolean"}` | 目标保护 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SetUnitProtectionResult`：成功结果；逐字段见types $defs/SetUnitProtectionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| unit | 是 | `{"$ref":"#/$defs/TranslationUnit"}` | 保护状态 |

算法：

1. 1 只能人工授权
2. 2 CAS单元
3. 3 humanGeneration++
4. 4 不自动重翻或覆盖现有文本
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写保护和审核日志

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.CancelTranslation

`func CancelTranslation(ctx context.Context, op OperationContext, input CancelTranslationInput) (CancelTranslationResult, error)`

作用：取消翻译子链路

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CancelTranslationInput`：封闭业务输入；逐字段见types $defs/CancelTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| reason | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原因 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CancelTranslationResult`：成功结果；逐字段见types $defs/CancelTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/Translation"}` | 取消状态 |

算法：

1. 1 写取消标志
2. 2 jobs CancelJob
3. 3 MG cancelInference
4. 4 拒绝所有旧lease写入
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：保留已有人工作品和已提交机翻；费用由MG处理

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:cancelJob, model-gateway:cancelInference

## localization.PutGlossaryTerm

`func PutGlossaryTerm(ctx context.Context, op OperationContext, input PutGlossaryTermInput) (PutGlossaryTermResult, error)`

作用：人工维护术语

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutGlossaryTermInput`：封闭业务输入；逐字段见types $defs/PutGlossaryTermInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| term | 是 | `{"$ref":"#/$defs/GlossaryTerm"}` | 术语 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutGlossaryTermResult`：成功结果；逐字段见types $defs/PutGlossaryTermResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| term | 是 | `{"$ref":"#/$defs/GlossaryTerm"}` | 新版本 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新代数 |

算法：

1. 1 BCP47和长度校验
2. 2 同scope原词唯一
3. 3 CAS并generation++
4. 4 事件使受影响机翻stale不改protected单元
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写glossary及glossary.changed

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.DeleteGlossaryTerm

`func DeleteGlossaryTerm(ctx context.Context, op OperationContext, input DeleteGlossaryTermInput) (DeleteGlossaryTermResult, error)`

作用：删除术语规则

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: DeleteGlossaryTermInput`：封闭业务输入；逐字段见types $defs/DeleteGlossaryTermInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| termId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 术语 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: DeleteGlossaryTermResult`：成功结果；逐字段见types $defs/DeleteGlossaryTermResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| deleted | 是 | `{"type":"boolean"}` | 已删除 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新代数 |

算法：

1. 1 校验权限
2. 2 CAS软删除
3. 3 generation++
4. 4 发布受影响范围
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：软删除保留审核记录

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.ListGlossary

`func ListGlossary(ctx context.Context, op OperationContext, input ListGlossaryInput) (ListGlossaryResult, error)`

作用：管理术语列表

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListGlossaryInput`：封闭业务输入；逐字段见types $defs/ListGlossaryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sourceLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 源 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: ListGlossaryResult`：成功结果；逐字段见types $defs/ListGlossaryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| terms | 是 | `{"type":"array","items":{"$ref":"#/$defs/GlossaryTerm"},"maxItems":256}` | 术语 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 读代数 |

算法：

1. 1 权限检查
2. 2 按termId稳定分页
3. 3 固定首屏generation到游标
4. 4 generation改变提示重新查询
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.ApplyApprovedGlossaryProposal

`func ApplyApprovedGlossaryProposal(ctx context.Context, op OperationContext, input ApplyApprovedGlossaryProposalInput) (ApplyApprovedGlossaryProposalResult, error)`

作用：应用memory已批准提议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ApplyApprovedGlossaryProposalInput`：封闭业务输入；逐字段见types $defs/ApplyApprovedGlossaryProposalInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposal | 是 | `{"$ref":"#/$defs/ApprovedGlossaryProposal"}` | 批准命令 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | proposalId一致幂等键 |

返回：

- `result: ApplyApprovedGlossaryProposalResult`：成功结果；逐字段见types $defs/ApplyApprovedGlossaryProposalResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| applied | 是 | `{"type":"boolean"}` | 应用成功 |
| termId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 落库术语 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 术语代数 |

算法：

1. 1 验证callerService=memory且approval签署委托
2. 2 proposalId/hash去重
3. 3 expectedGlossaryGeneration匹配才CAS term
4. 4 发glossary.proposal.applied或rejected终态事件
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写术语或拒绝记录；不同步回调memory

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization

## localization.HandleGlossaryChange

`func HandleGlossaryChange(ctx context.Context, op OperationContext, input HandleGlossaryChangeInput) (HandleGlossaryChangeResult, error)`

作用：使机翻版本按术语变化失效

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: HandleGlossaryChangeInput`：封闭业务输入；逐字段见types $defs/HandleGlossaryChangeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 事件 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新代数 |
| sourceLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 源 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |

返回：

- `result: HandleGlossaryChangeResult`：成功结果；逐字段见types $defs/HandleGlossaryChangeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| staleTranslations | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 标失效数 |

算法：

1. 1 inbox去重
2. 2 只更高generation
3. 3 受影响machine manifest stale
4. 4 protected文本保持并管理review提示
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：标stale不立即消费模型

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.ValidateInterfaceCatalog

`func ValidateInterfaceCatalog(ctx context.Context, op OperationContext, input ValidateInterfaceCatalogInput) (ValidateInterfaceCatalogResult, error)`

作用：校验界面字符串契约

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateInterfaceCatalogInput`：封闭业务输入；逐字段见types $defs/ValidateInterfaceCatalogInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| strings | 是 | `{"type":"array","items":{"$ref":"#/$defs/InterfaceString"},"maxItems":256}` | 字典条目 |

返回：

- `result: ValidateInterfaceCatalogResult`：成功结果；逐字段见types $defs/ValidateInterfaceCatalogResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 合法 |
| catalogHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范字典hash |

算法：

1. 1 key必须在固定源catalog
2. 2 placeholders与源逐字一致
3. 3 禁HTML与新增key
4. 4 规范hash
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
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

## localization.PutInterfaceString

`func PutInterfaceString(ctx context.Context, op OperationContext, input PutInterfaceStringInput) (PutInterfaceStringResult, error)`

作用：人工编辑界面文本

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutInterfaceStringInput`：封闭业务输入；逐字段见types $defs/PutInterfaceStringInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| string | 是 | `{"$ref":"#/$defs/InterfaceString"}` | 人工条目 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutInterfaceStringResult`：成功结果；逐字段见types $defs/PutInterfaceStringResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| string | 是 | `{"$ref":"#/$defs/InterfaceString"}` | human保护条目 |

算法：

1. 1 校验固定key/placeholder
2. 2 CAS
3. 3 origin=human protected=true
4. 4 更新catalog generation与事件
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写UI字典不重编译业务JS

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.TranslateInterfaceCatalog

`func TranslateInterfaceCatalog(ctx context.Context, op OperationContext, input TranslateInterfaceCatalogInput) (TranslateInterfaceCatalogResult, error)`

作用：显式翻译界面缺项

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: TranslateInterfaceCatalogInput`：封闭业务输入；逐字段见types $defs/TranslateInterfaceCatalogInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| keys | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 固定key列表 |
| expectedCatalogGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 代数 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: TranslateInterfaceCatalogResult`：成功结果；逐字段见types $defs/TranslateInterfaceCatalogResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| jobId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 翻译任务 |
| selectedKeys | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 未保护key |

算法：

1. 1 管理授权且profile配置
2. 2 排除protected/未变sourceHash
3. 3 固定catalog generation
4. 4 任务每批16key并输出严格schema
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：jobs翻译任务；可能MG费用

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:createJob, model-gateway:createInference

## localization.ReadInterfaceCatalog

`func ReadInterfaceCatalog(ctx context.Context, op OperationContext, input ReadInterfaceCatalogInput) (ReadInterfaceCatalogResult, error)`

作用：读取运行界面字典

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadInterfaceCatalogInput`：封闭业务输入；逐字段见types $defs/ReadInterfaceCatalogInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标语言 |
| knownGeneration | 否；省略不传null | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 客户端已知版本 |

返回：

- `result: ReadInterfaceCatalogResult`：成功结果；逐字段见types $defs/ReadInterfaceCatalogResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| strings | 是 | `{"type":"array","items":{"$ref":"#/$defs/InterfaceString"},"maxItems":256}` | 按默认fallback补全 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 当前代数 |
| notModified | 是 | `{"type":"boolean"}` | 版本未变 |

算法：

1. 1 language启用
2. 2 按key合并目标与默认builtin
3. 3 插值缺失视INTEGRITY
4. 4 版本相同返回空strings/notModified
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.SuggestLanguage

`func SuggestLanguage(ctx context.Context, op OperationContext, input SuggestLanguageInput) (SuggestLanguageResult, error)`

作用：给访客语言建议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SuggestLanguageInput`：封闭业务输入；逐字段见types $defs/SuggestLanguageInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| browserLanguages | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 最多10浏览器偏好 |
| timeZone | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | IANA时区 |
| country | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | guard离线GeoIP国家非IP |
| currentLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | URL语言 |
| explicitPreference | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 显式选择 |
| previouslyRejected | 是 | `{"type":"boolean"}` | 已拒建议 |

返回：

- `result: SuggestLanguageResult`：成功结果；逐字段见types $defs/SuggestLanguageResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| suggestedLanguage | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 目标建议 |
| confidence | 是 | `{"type":"number","minimum":0,"maximum":1}` | 置信度 |
| reason | 是 | `{"type":"string","enum":["model","rule","none"]}` | 路径 |
| promptText | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 建议语言提示 |

算法：

1. 1 explicitPreference或previouslyRejected返回none
2. 2 key=browserLanguages/timeZone/country+enabledLanguageGeneration缓存
3. 3 miss根task localization.suggest使用fast最多500output token且总5秒，timeout规则fallback
4. 4 confidence≥0.8且建议语言不同才提示，不存IP。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无存储、无模型、无强制跳转

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## localization.SaveLanguagePreference

`func SaveLanguagePreference(ctx context.Context, op OperationContext, input SaveLanguagePreferenceInput) (SaveLanguagePreferenceResult, error)`

作用：保存显式访客语言偏好

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SaveLanguagePreferenceInput`：封闭业务输入；逐字段见types $defs/SaveLanguagePreferenceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| visitorSubject | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | guard伪名 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 显式选择 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SaveLanguagePreferenceResult`：成功结果；逐字段见types $defs/SaveLanguagePreferenceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 保存值 |
| expiresAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 30天过期 |

算法：

1. 1 验证伪名owner
2. 2 language已启用
3. 3 保存30天TTL
4. 4 不保存IP或浏览轨迹
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写偏好

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.TranslateCachedAnswer

`func TranslateCachedAnswer(ctx context.Context, op OperationContext, input TranslateCachedAnswerInput) (TranslateCachedAnswerResult, error)`

作用：接受QA精选翻译命令

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: TranslateCachedAnswerInput`：封闭业务输入；逐字段见types $defs/TranslateCachedAnswerInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: TranslateCachedAnswerResult`：成功结果；逐字段见types $defs/TranslateCachedAnswerResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | active普通自动缓存或精选缓存，非仅人工精选 |
| answerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 源版本 |
| sourceLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 源 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |
| question | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 公开问题 |
| answer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 公开回答 |
| sourceHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 问题回答规范hash |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/CacheAnswerTranslation"}` | queued译文 |

算法：

1. 1 callerService=qa且源为active普通或精选cache，不新增人工批准或模型配置权限门槛
2. 2 核对sourceHash和基准answerRevision，已配置根qa.cache-translate fast任务并分别取翻译任务记忆
3. 3 继承原qa rootOperationId/costClass及同一签名admissionPermit到每个MG调用，费用计相同session/network小时作用域
4. 4 异步翻译完成自动ready，发cache-answer.translated，由QA自动保存对应语言答案
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写缓存译文任务；MG费用由background继承

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:createJob；memory:resolveContext；model-gateway:createInference

## localization.CommitCacheAnswerTranslation

`func CommitCacheAnswerTranslation(ctx context.Context, op OperationContext, input CommitCacheAnswerTranslationInput) (CommitCacheAnswerTranslationResult, error)`

作用：保存精选翻译供QA审核

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CommitCacheAnswerTranslationInput`：封闭业务输入；逐字段见types $defs/CommitCacheAnswerTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/CacheAnswerTranslation"}` | 候选结果 |
| expectedAnswerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 源版本 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: CommitCacheAnswerTranslationResult`：成功结果；逐字段见types $defs/CommitCacheAnswerTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/CacheAnswerTranslation"}` | review译文 |

算法：

1. 1 源cache answerRevision/hash/epoch匹配
2. 2 引用token和语言格式核验
3. 3 保存ready自动译文不加人工批准
4. 4 发cache-answer.translated引用事件供QA自动存localized答案。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写译文与outbox，不同步调用QA

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.InvalidateDocumentTranslations

`func InvalidateDocumentTranslations(ctx context.Context, op OperationContext, input InvalidateDocumentTranslationsInput) (InvalidateDocumentTranslationsResult, error)`

作用：处理源撤回

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: InvalidateDocumentTranslationsInput`：封闭业务输入；逐字段见types $defs/InvalidateDocumentTranslationsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 撤回事件 |
| documentId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 源文档 |
| visibilityRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 权威代数 |

返回：

- `result: InvalidateDocumentTranslationsResult`：成功结果；逐字段见types $defs/InvalidateDocumentTranslationsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invalidated | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 失效译文 |
| cancelled | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 取消任务 |

算法：

1. 1 inbox去重
2. 2 更高可见性代数
3. 3 manifest stale
4. 4 取消子任务并禁止公开读取
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：译文失活与取消；不删除人工历史

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:cancelJob, model-gateway:cancelInference

## localization.RecoverTranslationLeases

`func RecoverTranslationLeases(ctx context.Context, op OperationContext, input RecoverTranslationLeasesInput) (RecoverTranslationLeasesResult, error)`

作用：恢复翻译任务

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RecoverTranslationLeasesInput`：封闭业务输入；逐字段见types $defs/RecoverTranslationLeasesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| now | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 时间 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: RecoverTranslationLeasesResult`：成功结果；逐字段见types $defs/RecoverTranslationLeasesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| requeued | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 可恢复数 |
| superseded | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧结果数 |

算法：

1. 1 到期lease拒绝写
2. 2 固定source/current和人工代数
3. 3 旧任务superseded
4. 4 MG未知调用不重复消费只读其状态
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：任务状态恢复

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, model-gateway:readInference

## localization.PublishOutbox

`func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)`

作用：持久发布翻译事件

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PublishOutboxInput`：封闭业务输入；逐字段见types $defs/PublishOutboxInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |
| workerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | worker |

返回：

- `result: PublishOutboxResult`：成功结果；逐字段见types $defs/PublishOutboxResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| published | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 确认数 |
| pending | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 待发数 |

算法：

1. 1 租约领批
2. 2 只发布不可变引用和版本
3. 3 jobs ACK才标
4. 4 有限重试三次后留待下轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：at-least-once事件；无正文广播

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:publishEvent

## localization.AcquireBarrier

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
3. 3 屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本
4. 导入必须恢复保护位与源ref，sourceEpoch未激活前译文不能交付，不自动重翻。
5. 4 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。
6. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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

## localization.ReleaseBarrier

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

## localization.ExportSnapshot

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
3. 3 屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本
4. 导入必须恢复保护位与源ref，sourceEpoch未激活前译文不能交付，不自动重翻。
5. 4 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。
6. 恢复控制校验RecoveryAuthorization范围与本地safe屏障；op.dataEpoch可以在许可source/target epoch内，不依赖已导入的新identity会话；control.db及信任锚不导入覆盖。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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

## localization.ValidateSnapshot

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

## localization.PrepareSnapshot

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

## localization.CommitSnapshot

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

## localization.RollbackSnapshot

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

## localization.LoadAdminOverview

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
| items | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 后台概览条目 |
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

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.SubmitPrimaryEdit

`export async function submitPrimaryEdit(input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult>`

作用：提交本域明确人工修改

参数：

- `input: SubmitPrimaryEditInput`：本域封闭输入；每个字段规格见types.json $defs/SubmitPrimaryEditInput；schema required/additionalProperties/range全部强制；null禁止。
- `signal: AbortSignal`：界面导航及取消信号；取消后不更新已卸载组件。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 译文 |
| unitId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 单元 |
| targetText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 人工文本 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 单元版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SubmitPrimaryEditResult`：成功结果；逐字段见types $defs/SubmitPrimaryEditResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| unit | 是 | `{"$ref":"#/$defs/TranslationUnit"}` | human且protected |

算法：

1. 1 表单逐字段验证本域schema
2. 2 生成幂等键并携expectedVersion
3. 3 调本域editTranslationUnit API
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

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.ReuseTranslationMemory

`func ReuseTranslationMemory(ctx context.Context, op OperationContext, input ReuseTranslationMemoryInput) (ReuseTranslationMemoryResult, error)`

作用：精确翻译记忆复用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReuseTranslationMemoryInput`：封闭业务输入；逐字段见types $defs/ReuseTranslationMemoryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sourceHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 源块hash |
| sourceText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原文 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |
| minimumCharacters | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":20}` | 默认20 |

返回：

- `result: ReuseTranslationMemoryResult`：成功结果；逐字段见types $defs/ReuseTranslationMemoryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| unit | 否；省略不传null | `{"$ref":"#/$defs/TranslationUnit"}` | 可复用译文 |
| reused | 是 | `{"type":"boolean"}` | 命中 |

算法：

1. 1 字符数≥20配置阈值
2. 2 sourceHash及语言一致且译文正常/当前术语兼容
3. 3 只复用target text保留新blockId来源
4. 4 短块因上下文变化不复用
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

## localization.ExtractAutomaticGlossary

`func ExtractAutomaticGlossary(ctx context.Context, op OperationContext, input ExtractAutomaticGlossaryInput) (ExtractAutomaticGlossaryResult, error)`

作用：首次翻译直接抽取入术语表

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ExtractAutomaticGlossaryInput`：封闭业务输入；逐字段见types $defs/ExtractAutomaticGlossaryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 第一次译源 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |
| sourceText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 整篇源 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: ExtractAutomaticGlossaryResult`：成功结果；逐字段见types $defs/ExtractAutomaticGlossaryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| terms | 是 | `{"type":"array","items":{"$ref":"#/$defs/GlossaryTerm"},"maxItems":256}` | 自动术语 |
| batchId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 可撤销抽取批次 |

算法：

1. 1 document/target firstTranslation唯一
2. 2 根task localization.terms使用accurate档max1000output token抽取
3. 3 manual词条优先绝不覆盖，auto冲突保留高version
4. 4 新auto术语直接入库generation++且batchId可撤销。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用及自动术语写入，不引入人工批准产品步骤

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## localization.RollbackAutomaticGlossaryBatch

`func RollbackAutomaticGlossaryBatch(ctx context.Context, op OperationContext, input RollbackAutomaticGlossaryBatchInput) (RollbackAutomaticGlossaryBatchResult, error)`

作用：整批撤销自动术语

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RollbackAutomaticGlossaryBatchInput`：封闭业务输入；逐字段见types $defs/RollbackAutomaticGlossaryBatchInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| batchId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 抽取批次 |
| expectedGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 术语代数 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: RollbackAutomaticGlossaryBatchResult`：成功结果；逐字段见types $defs/RollbackAutomaticGlossaryBatchResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| removed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 仍属该auto批次数 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新代数 |

算法：

1. 1 localization.manage
2. 2 CAS generation
3. 3 仅删除source=auto且仍该batch，已转manual绝不删
4. 4 generation++并失效受影响机翻
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：术语批次撤销及事件

授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.PlanTranslationFallback

`func PlanTranslationFallback(ctx context.Context, op OperationContext, input PlanTranslationFallbackInput) (PlanTranslationFallbackResult, error)`

作用：逐级翻译失败降级

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PlanTranslationFallbackInput`：封闭业务输入；逐字段见types $defs/PlanTranslationFallbackInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| failedUnitIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 失败批 |
| failure | 是 | `{"type":"string","enum":["format","length","unchanged","terminology"]}` | 原因 |
| stage | 是 | `{"type":"string","enum":["batch","halves","single"]}` | 当前阶段 |

返回：

- `result: PlanTranslationFallbackResult`：成功结果；逐字段见types $defs/PlanTranslationFallbackResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| retryGroups | 是 | `{"type":"array","items":{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256},"maxItems":256}` | 有限降级分组 |
| reduceContext | 是 | `{"type":"boolean"}` | 长度失败缩上下文 |
| fallbackUnitIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 最终保留原文 |

算法：

1. 1 format/unchanged批对半
2. 2 halves仍失败逐块
3. 3 single仍失败保留原文并状态fallback
4. 4 length先减参考context仅1次，terminology提醒重译仅1次，所有新调用独立MG预留。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：保存降级计划非无限重试

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.CheckTerminologyCompliance

`func CheckTerminologyCompliance(ctx context.Context, op OperationContext, input CheckTerminologyComplianceInput) (CheckTerminologyComplianceResult, error)`

作用：检查规定译法符合度

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CheckTerminologyComplianceInput`：封闭业务输入；逐字段见types $defs/CheckTerminologyComplianceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 译文 |
| terms | 是 | `{"type":"array","items":{"$ref":"#/$defs/GlossaryTerm"},"maxItems":256}` | 原文实际出现术语 |
| alreadyRetried | 是 | `{"type":"boolean"}` | 已提醒重译 |

返回：

- `result: CheckTerminologyComplianceResult`：成功结果；逐字段见types $defs/CheckTerminologyComplianceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| retryUnitIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 首次不符合重译 |
| reviewUnitIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 二次不符提醒 |

算法：

1. 1 只检查源出现terms
2. 2 目标包含固定译法或允许语言词形变体
3. 3 第一次不符带提醒重译一次
4. 4 仍不符review不阻止发布已译文
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写review/重译计划

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## localization.ReflectTranslation

`func ReflectTranslation(ctx context.Context, op OperationContext, input ReflectTranslationInput) (ReflectTranslationResult, error)`

作用：可选反思润色

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReflectTranslationInput`：封闭业务输入；逐字段见types $defs/ReflectTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 译文 |
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 初译 |
| sourceText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 对应源 |
| enabled | 是 | `{"type":"boolean"}` | 语言配置默认false |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: ReflectTranslationResult`：成功结果；逐字段见types $defs/ReflectTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| units | 是 | `{"type":"array","items":{"$ref":"#/$defs/TranslationUnit"},"maxItems":256}` | 润色候选 |
| invocationIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 费用调用 |

算法：

1. 1 enabled=false直接原样
2. 2 精确档对照source提供修改意见再改写两次bounded调用
3. 3 输出仍固定blockIds/inline标签
4. 4 CAS人工保护/source/代数，费用约初译3倍以MG实际账本为准
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：可选MG费用，默认无调用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## localization.ReadCacheAnswerTranslation

`func ReadCacheAnswerTranslation(ctx context.Context, op OperationContext, input ReadCacheAnswerTranslationInput) (ReadCacheAnswerTranslationResult, error)`

作用：读取已完成的自动缓存翻译

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadCacheAnswerTranslationInput`：封闭业务输入；逐字段见types $defs/ReadCacheAnswerTranslationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 缓存回答翻译对象 |
| expectedAnswerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | QA所见源版本 |

返回：

- `result: ReadCacheAnswerTranslationResult`：成功结果；逐字段见types $defs/ReadCacheAnswerTranslationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translation | 是 | `{"$ref":"#/$defs/CacheAnswerTranslation"}` | ready公开译文 |

算法：

1. 1 callerService=qa
2. 2 源answerRevision精确匹配
3. 3 ready且dataEpoch当前才返回
4. 4 不改变审核策略、不隐式调模型
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

依赖：本域声明类型和存储；无跨域调用。

## localization.RunWorker

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

### M10-F901 main

文件 `modules/localization/cmd/localization/main.go`。独立进程可执行入口

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

### M10-F902 BuildApplication

文件 `modules/localization/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M10-F903 Serve

文件 `modules/localization/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M10-F904 Shutdown

文件 `modules/localization/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M10-F905 Health

文件 `modules/localization/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M10-F906 Ready

文件 `modules/localization/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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

### M10-F907 mountModuleUI

文件 `modules/localization/ui/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

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
