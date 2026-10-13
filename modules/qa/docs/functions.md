# 问答、引用与回答缓存逐函数实现合同

每一函数与implementation.json一一对应；输入和结果的全部字段见本节对应JSON Schema，公共类型见根common.schema.json。

## qa.LoadRuntime

`func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)`

作用：启动问答服务

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: LoadRuntimeInput`：封闭业务输入；逐字段见types $defs/LoadRuntimeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| serviceId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 服务身份 |
| dataDirectory | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 私有目录 |

返回：

- `result: LoadRuntimeResult`：成功结果；逐字段见types $defs/LoadRuntimeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| ready | 是 | `{"type":"boolean"}` | 就绪 |
| policyVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 策略版本 |

算法：

1. 1 载入schema/policy
2. 2 检查SSE事件序号索引
3. 3 恢复取消与终态
4. 4 不恢复派发未知MG调用
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

副作用：打开本服务连接池

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.PutQaPolicy

`func PutQaPolicy(ctx context.Context, op OperationContext, input PutQaPolicyInput) (PutQaPolicyResult, error)`

作用：管理公开问答策略

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutQaPolicyInput`：封闭业务输入；逐字段见types $defs/PutQaPolicyInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| policy | 是 | `{"$ref":"#/$defs/QaPolicy"}` | 公开策略 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutQaPolicyResult`：成功结果；逐字段见types $defs/PutQaPolicyResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| policy | 是 | `{"$ref":"#/$defs/QaPolicy"}` | 新策略 |

算法：

1. 1 限制各token预算
2. 2 embedding space与MG/knowledge匹配
3. 3 semantic阈值≥0.94
4. 4 CAS更新并失效scope
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

副作用：写policy+缓存失效事件

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:readProfiles

## qa.CreateConversation

`func CreateConversation(ctx context.Context, op OperationContext, input CreateConversationInput) (CreateConversationResult, error)`

作用：创建伪名访客会话

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateConversationInput`：封闭业务输入；逐字段见types $defs/CreateConversationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| admission | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AdmissionPermit"}` | guard凭证 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CreateConversationResult`：成功结果；逐字段见types $defs/CreateConversationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversation | 是 | `{"$ref":"#/$defs/Conversation"}` | 会话 |

算法：

1. 1 验证guard签名action与subject
2. 2 不保存IP
3. 3 生成会话ID及30天过期
4. 4 同visitor/key返回同会话
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写conversation

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：traffic-guard:verifyAdmission

## qa.GetConversation

`func GetConversation(ctx context.Context, op OperationContext, input GetConversationInput) (GetConversationResult, error)`

作用：读取自有访客历史

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: GetConversationInput`：封闭业务输入；逐字段见types $defs/GetConversationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 消息游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: GetConversationResult`：成功结果；逐字段见types $defs/GetConversationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversation | 是 | `{"$ref":"#/$defs/Conversation"}` | 快照 |
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/Message"},"maxItems":256}` | 消息 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |

算法：

1. 1 校验ctx.actor.subjectId与visitorSubject
2. 2 过期返回410
3. 3 ordinal分页
4. 4 只返回本会话消息
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

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.StartTurn

`func StartTurn(ctx context.Context, op OperationContext, input StartTurnInput) (StartTurnResult, error)`

作用：准入后开始问答轮次

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: StartTurnInput`：封闭业务输入；逐字段见types $defs/StartTurnInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| question | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | ≤500Unicode字符且≤8192bytes |
| currentPage | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 当前页提示引用 |
| admission | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AdmissionPermit"}` | 准入 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: StartTurnResult`：成功结果；逐字段见types $defs/StartTurnResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turn | 是 | `{"$ref":"#/$defs/Turn"}` | queued轮次 |
| streamPath | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 流订阅路径 |

算法：

1. 1 owner及guard permit验证
2. 2 question≤500Unicode字符并固定可信qa上下文
3. 3 同会话最多1活跃轮次，其余请求持久排队最多5项按顺序执行
4. 4 写原问题/当前页hint/queued状态，前置仅收权威校验提示。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- RATE_LIMITED (HTTP429)：guard准入或本服务供应方并发槽位达到上限。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写question与任务；发模型前固定可信costClass=qa

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：traffic-guard:consumeAdmission, jobs:createJob

## qa.ValidateAdmission

`func ValidateAdmission(ctx context.Context, op OperationContext, input ValidateAdmissionInput) (ValidateAdmissionResult, error)`

作用：检查准入绑定

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateAdmissionInput`：封闭业务输入；逐字段见types $defs/ValidateAdmissionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| admission | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AdmissionPermit"}` | 凭证 |
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| action | 是 | `{"type":"string","enum":["qa.turn","qa.conversation"]}` | 动作 |

返回：

- `result: ValidateAdmissionResult`：成功结果；逐字段见types $defs/ValidateAdmissionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 通过 |
| visitorSubject | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 伪名 |

算法：

1. 1 验签及expiresAt
2. 2 action/requestId/subjectId匹配
3. 3 guard消费一次凭证
4. 4 不接受客户端costClass
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- RATE_LIMITED (HTTP429)：guard准入或本服务供应方并发槽位达到上限。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：消费guard permit；不建费用预留

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：traffic-guard:consumeAdmission

## qa.NormalizeQuestion

`func NormalizeQuestion(ctx context.Context, op OperationContext, input NormalizeQuestionInput) (NormalizeQuestionResult, error)`

作用：稳定规范问题和cache锚点

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: NormalizeQuestionInput`：封闭业务输入；逐字段见types $defs/NormalizeQuestionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| question | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原问题 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |

返回：

- `result: NormalizeQuestionResult`：成功结果；逐字段见types $defs/NormalizeQuestionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| normalized | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范文本 |
| questionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 语言加规范文本hash |
| anchors | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 实体数字否定锚点 |

算法：

1. 1 NFKC/LF/全半角空白规范化
2. 2 自然语言大小写折叠但代码/产品ID/数字/否定词保留
3. 3 ≤500Unicode字符且≤8192bytes
4. 4 stable算法提取锚点并输出hash。
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

## qa.ResolveTurnContext

`func ResolveTurnContext(ctx context.Context, op OperationContext, input ResolveTurnContextInput) (ResolveTurnContextResult, error)`

作用：固定回答上下文版本

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResolveTurnContextInput`：封闭业务输入；逐字段见types $defs/ResolveTurnContextInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |

返回：

- `result: ResolveTurnContextResult`：成功结果；逐字段见types $defs/ResolveTurnContextResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| memory | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/MemoryContext"}` | 注入快照 |
| policy | 是 | `{"$ref":"#/$defs/QaPolicy"}` | 策略快照 |
| contextHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 版本摘要 |

算法：

1. 1 读取当前policy/dataEpoch
2. 2 memory resolveContext taskId=qa.answer
3. 3 固定mapping/versionHash
4. 4 canonical hash形成本轮上下文基线
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

副作用：写turn context摘要不记录memory正文到日志

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：memory:resolveContext, site-config:readRuntimeState

## qa.LookupExactCache

`func LookupExactCache(ctx context.Context, op OperationContext, input LookupExactCacheInput) (LookupExactCacheResult, error)`

作用：查询精确精选缓存

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: LookupExactCacheInput`：封闭业务输入；逐字段见types $defs/LookupExactCacheInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: LookupExactCacheResult`：成功结果；逐字段见types $defs/LookupExactCacheResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| questionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 完整基准问题hash |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 访客语言 |
| scopeHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 站点范围 |
| complete | 是 | `{"type":"boolean"}` | 已判完整 |
| firstTurn | 是 | `{"type":"boolean"}` | 第一轮 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 否，省略且禁止null | `{"$ref":"#/$defs/CacheEntry"}` | 命中 |
| hit | 是 | `{"type":"boolean"}` | 是否命中 |

算法：

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1 complete=false绝不查询
3. 2 第一轮已有standalone exact条目可在前置前命中，追问须PlanQuestion补全后才查
4. 3 优先curated再未过期普通缓存
5. 4 精选豁免memory版本但全部来源仍需ValidateCachedAnswer。
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读，返回前仍需验证来源

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：

## qa.LookupSemanticCache

`func LookupSemanticCache(ctx context.Context, op OperationContext, input LookupSemanticCacheInput) (LookupSemanticCacheResult, error)`

作用：保守查询语义缓存

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: LookupSemanticCacheInput`：封闭业务输入；逐字段见types $defs/LookupSemanticCacheInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: LookupSemanticCacheResult`：成功结果；逐字段见types $defs/LookupSemanticCacheResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| plan | 是 | `{"$ref":"#/$defs/QuestionPlan"}` | 完整问题计划 |
| embedding | 是 | `{"$ref":"#/$defs/QueryEmbedding"}` | 本轮复用向量 |
| scopeHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 范围 |
| mode | 是 | `{"type":"string","enum":["ai_equivalence","threshold"]}` | 模式 |
| threshold | 是 | `{"type":"number","minimum":0,"maximum":1}` | 阈值 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 否，省略且禁止null | `{"$ref":"#/$defs/CacheEntry"}` | 匹配 |
| similarity | 是 | `{"type":"number","minimum":0,"maximum":1}` | 相似度 |
| hit | 是 | `{"type":"boolean"}` | 命中 |
| embedding | 是 | `{"$ref":"#/$defs/QueryEmbedding"}` | 原向量复用 |

算法：

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1a 默认AI同义判断分别memory:resolveContext taskId=qa.cache-match，不复用qa.answer；threshold模式不额外取模型task记忆。
3. 1 plan.complete/cacheEligible必须true
4. 2 同space/base canonical范围cosine≥0.8最多3项
5. 3 默认fast task qa.cache-match判断可直接同义回答，禁止rerank替代
6. 4 threshold模式用配置阈值及否定实体锚点保护
7. miss返回原embedding供knowledge复用。
8. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：可能MG查询embedding费用；不动态新增cache

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference；memory:resolveContext

## qa.ValidateCachedAnswer

`func ValidateCachedAnswer(ctx context.Context, op OperationContext, input ValidateCachedAnswerInput) (ValidateCachedAnswerResult, error)`

作用：验证缓存仍可公开交付

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateCachedAnswerInput`：封闭业务输入；逐字段见types $defs/ValidateCachedAnswerInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/CacheEntry"}` | 候选缓存 |
| contextHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 本轮上下文 |

返回：

- `result: ValidateCachedAnswerResult`：成功结果；逐字段见types $defs/ValidateCachedAnswerResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 有效 |
| entry | 否；省略不传null | `{"$ref":"#/$defs/CacheEntry"}` | 可交付缓存 |

算法：

1. 1 active且未过TTL，curated免TTL
2. 2 content校验current/visible且所有块hash相同
3. 3 普通cache校验qa.answer memoryVersionHash，curated豁免仅该项
4. 4 无引用答案在任一发布revision变化即stale，依赖不可用不得返回。
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

副作用：可能标cache stale；绝不返回撤回答案

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## qa.RetrieveEvidence

`func RetrieveEvidence(ctx context.Context, op OperationContext, input RetrieveEvidenceInput) (RetrieveEvidenceResult, error)`

作用：取有源可见证据

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RetrieveEvidenceInput`：封闭业务输入；逐字段见types $defs/RetrieveEvidenceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| normalized | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 问题 |
| policy | 是 | `{"$ref":"#/$defs/QaPolicy"}` | 策略 |

返回：

- `result: RetrieveEvidenceResult`：成功结果；逐字段见types $defs/RetrieveEvidenceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| citations | 是 | `{"type":"array","items":{"$ref":"#/$defs/Citation"},"maxItems":256}` | 引用候选 |
| evidenceText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 内部上下文 |
| indexVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 索引摘要 |

算法：

1. 1 knowledge searchKnowledge按固定space
2. 2 最多8块/4096token
3. 3 对IDs调用readSections
4. 4 映射可公开Citation且保留ref
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

副作用：写turn证据快照hash，不拥有正文权威

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：knowledge:searchKnowledge, knowledge:readSections

## qa.BuildAnswerPrompt

`func BuildAnswerPrompt(ctx context.Context, op OperationContext, input BuildAnswerPromptInput) (BuildAnswerPromptResult, error)`

作用：构造有界回答提示

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BuildAnswerPromptInput`：封闭业务输入；逐字段见types $defs/BuildAnswerPromptInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| question | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范问题 |
| memory | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/MemoryContext"}` | 注入 |
| evidenceText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 证据 |
| history | 是 | `{"type":"array","items":{"$ref":"#/$defs/Message"},"maxItems":256}` | 历史 |

返回：

- `result: BuildAnswerPromptResult`：成功结果；逐字段见types $defs/BuildAnswerPromptResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 结构化模型消息 |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输出预算 |

算法：

1. 1 任务记忆在固定system prefix最前，各条带description，任务指令随后
2. 2 summary/最近对话/当前页验证片段/检索材料分别标数据边界
3. 3 访客语言回答、优先对应块译文且引用当前语言href
4. 4 无证据明确文档未提及，固定最大1500输出tokens。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无副作用；不将未授权材料发送MG

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.GenerateAnswer

`func GenerateAnswer(ctx context.Context, op OperationContext, input GenerateAnswerInput) (GenerateAnswerResult, error)`

作用：执行流式答案调用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: GenerateAnswerInput`：封闭业务输入；逐字段见types $defs/GenerateAnswerInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 消息 |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 上限 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | worker租约 |

返回：

- `result: GenerateAnswerResult`：成功结果；逐字段见types $defs/GenerateAnswerResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | MG调用 |
| turn | 是 | `{"$ref":"#/$defs/Turn"}` | generating状态 |

算法：

1. 1 校验lease/cancel/deadline
2. 2 ctx继承root/costClass=qa
3. 3 MG createInference
4. 4 消费原流并记录invocationId不重放未知请求
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG产生费用；写生成状态及事件

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## qa.ValidateCitation

`func ValidateCitation(ctx context.Context, op OperationContext, input ValidateCitationInput) (ValidateCitationResult, error)`

作用：逐条保护引用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateCitationInput`：封闭业务输入；逐字段见types $defs/ValidateCitationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| citationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型选择引用 |
| allowed | 是 | `{"type":"array","items":{"$ref":"#/$defs/Citation"},"maxItems":256}` | 白名单 |

返回：

- `result: ValidateCitationResult`：成功结果；逐字段见types $defs/ValidateCitationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| citation | 是 | `{"$ref":"#/$defs/Citation"}` | 已验证引用 |

算法：

1. 1 citationId必须在白名单
2. 2 content当前visible/matchesCurrent
3. 3 href由public route规则生成
4. 4 缺失返回STALE_RESULT并停止该证据回答
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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

副作用：只读；不可创造源外引用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## qa.AppendAnswerEvent

`func AppendAnswerEvent(ctx context.Context, op OperationContext, input AppendAnswerEventInput) (AppendAnswerEventResult, error)`

作用：持久流式输出事件

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: AppendAnswerEventInput`：封闭业务输入；逐字段见types $defs/AppendAnswerEventInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| event | 是 | `{"$ref":"#/$defs/AnswerEvent"}` | 无序号事件 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: AppendAnswerEventResult`：成功结果；逐字段见types $defs/AppendAnswerEventResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 已持久化序号 |

算法：

1. 1 租约/cancel状态检查
2. 2 delta≤16KiB且UTF8完整
3. 3 事务递增序号和事件
4. 4 提交后广播
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：turn_events写入；严格有序

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.ResumeAnswerStream

`func ResumeAnswerStream(ctx context.Context, op OperationContext, input ResumeAnswerStreamInput) (ResumeAnswerStreamResult, error)`

作用：SSE断线重连

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResumeAnswerStreamInput`：封闭业务输入；逐字段见types $defs/ResumeAnswerStreamInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| afterSequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | LastEventID |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..256 |

返回：

- `result: ResumeAnswerStreamResult`：成功结果；逐字段见types $defs/ResumeAnswerStreamResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| events | 是 | `{"type":"array","items":{"$ref":"#/$defs/AnswerEvent"},"maxItems":256}` | 有序事件 |
| terminal | 是 | `{"type":"boolean"}` | 终态 |
| lastSequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 最后序号 |

算法：

1. 1 会话owner授权
2. 2 读cursor后事件
3. 3 每15秒heartbeat无持久序号
4. 4 超24小时保留期返回410
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读，不重新生成答案

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.CompleteTurn

`func CompleteTurn(ctx context.Context, op OperationContext, input CompleteTurnInput) (CompleteTurnResult, error)`

作用：原子结束可见答案

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CompleteTurnInput`：封闭业务输入；逐字段见types $defs/CompleteTurnInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| answer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 完整模型结果 |
| citations | 是 | `{"type":"array","items":{"$ref":"#/$defs/Citation"},"maxItems":256}` | 实际引用 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: CompleteTurnResult`：成功结果；逐字段见types $defs/CompleteTurnResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turn | 是 | `{"$ref":"#/$defs/Turn"}` | completed |
| message | 是 | `{"$ref":"#/$defs/Message"}` | 答案消息 |

算法：

1. 1 content终态前再validate全部引用
2. 2 校验cancel及lease
3. 3 事务assistant message/answerRevision/completed事件
4. 4 释放guard并发permit
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：保存消息与终态事件；不自动批准cache

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, traffic-guard:releaseAdmission

## qa.CancelTurn

`func CancelTurn(ctx context.Context, op OperationContext, input CancelTurnInput) (CancelTurnResult, error)`

作用：取消问答及所有子调用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CancelTurnInput`：封闭业务输入；逐字段见types $defs/CancelTurnInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| reason | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原因 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CancelTurnResult`：成功结果；逐字段见types $defs/CancelTurnResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turn | 是 | `{"$ref":"#/$defs/Turn"}` | 终态或cancel状态 |

算法：

1. 1 owner校验
2. 2 写cancel标记
3. 3 MG cancelInference与jobs取消
4. 4 发布cancelled并释放guard slot但MG费用依其证据
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：停止新delta，取消子任务不自行释放费用

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:cancelInference, jobs:cancelJob, traffic-guard:releaseAdmission

## qa.RecordFeedback

`func RecordFeedback(ctx context.Context, op OperationContext, input RecordFeedbackInput) (RecordFeedbackResult, error)`

作用：保存访客单条反馈

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RecordFeedbackInput`：封闭业务输入；逐字段见types $defs/RecordFeedbackInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| answerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 所评版本 |
| value | 是 | `{"type":"string","enum":["helpful","unhelpful"]}` | 评价 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: RecordFeedbackResult`：成功结果；逐字段见types $defs/RecordFeedbackResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 反馈版本 |
| value | 是 | `{"type":"string","enum":["helpful","unhelpful"]}` | 已保存评价 |

算法：

1. 1 owner及completed校验
2. 2 answerRevision准确匹配
3. 3 每visitor/turn唯一upsert
4. 4 审计只传评分无问题正文
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写feedback及聚合事件

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.ProposeCacheEntry

`func ProposeCacheEntry(ctx context.Context, op OperationContext, input ProposeCacheEntryInput) (ProposeCacheEntryResult, error)`

作用：人工创建精选候选

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ProposeCacheEntryInput`：封闭业务输入；逐字段见types $defs/ProposeCacheEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 原轮次 |
| publicQuestion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 清理后公开问题 |
| publicAnswer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 清理后公开答案 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ProposeCacheEntryResult`：成功结果；逐字段见types $defs/ProposeCacheEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/CacheEntry"}` | pending精选 |

算法：

1. 1 qa.manage授权
2. 2 completed且引用有效
3. 3 清理访客身份/私密上下文
4. 4 保存pending不会命中
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写cache候选及审核记录

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## qa.ApproveCacheEntry

`func ApproveCacheEntry(ctx context.Context, op OperationContext, input ApproveCacheEntryInput) (ApproveCacheEntryResult, error)`

作用：批准缓存公开内容

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ApproveCacheEntryInput`：封闭业务输入；逐字段见types $defs/ApproveCacheEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 候选 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ApproveCacheEntryResult`：成功结果；逐字段见types $defs/ApproveCacheEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/CacheEntry"}` | active精选 |

算法：

1. 1 qa.manage人工编辑后标curated
2. 2 核对source引用有效
3. 3 curated免TTL与未来memory变化但仍受引用失效
4. 4 active后exact优先，semantic embedding另建。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：启用exact缓存；semantic embedding单独任务

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, memory:resolveContext

## qa.BuildCacheEmbedding

`func BuildCacheEmbedding(ctx context.Context, op OperationContext, input BuildCacheEmbeddingInput) (BuildCacheEmbeddingResult, error)`

作用：为已批准缓存构建语义向量

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BuildCacheEmbeddingInput`：封闭业务输入；逐字段见types $defs/BuildCacheEmbeddingInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 批准版本 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: BuildCacheEmbeddingResult`：成功结果；逐字段见types $defs/BuildCacheEmbeddingResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目 |
| vectorHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 向量摘要 |

算法：

1. 1 只active且原version
2. 2 ctx costClass=background继承批准根
3. 3 MG生成向量
4. 4 返回时CAS版本/space/epoch再写
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用和cache_vector写入；旧结果丢弃

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## qa.InvalidateCache

`func InvalidateCache(ctx context.Context, op OperationContext, input InvalidateCacheInput) (InvalidateCacheResult, error)`

作用：失效受版本变化影响缓存

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: InvalidateCacheInput`：封闭业务输入；逐字段见types $defs/InvalidateCacheInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: InvalidateCacheResult`：成功结果；逐字段见types $defs/InvalidateCacheResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 失效事件 |
| documentId | 否，省略且禁止null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 文档范围 |
| memoryVersionHash | 否，省略且禁止null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 旧记忆版本 |
| policyVersion | 否，省略且禁止null | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧策略版本 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invalidated | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 条目数 |

算法：

1. 1 inbox按eventId去重并检查epoch
2. 2 引用变化/撤回/删除使全部受影响cache含精选stale；memory变化仅匹配curated=false和旧memoryVersionHash，curated=true豁免；回答生成模型变化不自动失效
3. 3 active→stale仅作用于上述筛选集；TTL仅普通cache；无引用答案在任一新发布时失效
4. 4 删除失效向量可命中索引，保留审核历史与账单引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：cache stale及cache.invalidated事件

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：

## qa.DisableCacheEntry

`func DisableCacheEntry(ctx context.Context, op OperationContext, input DisableCacheEntryInput) (DisableCacheEntryResult, error)`

作用：人工停用缓存

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: DisableCacheEntryInput`：封闭业务输入；逐字段见types $defs/DisableCacheEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: DisableCacheEntryResult`：成功结果；逐字段见types $defs/DisableCacheEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/CacheEntry"}` | disabled |

算法：

1. 1 校验qa.manage
2. 2 CAS版本
3. 3 active/pending/stale均可disabled
4. 4 禁用向量命中
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

副作用：写cache状态/审核

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.ListCacheEntries

`func ListCacheEntries(ctx context.Context, op OperationContext, input ListCacheEntriesInput) (ListCacheEntriesResult, error)`

作用：管理精选缓存

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListCacheEntriesInput`：封闭业务输入；逐字段见types $defs/ListCacheEntriesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| state | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 状态筛选 |
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: ListCacheEntriesResult`：成功结果；逐字段见types $defs/ListCacheEntriesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/CacheEntry"},"maxItems":256}` | 精选条目 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |

算法：

1. 1 权限检查
2. 2 稳定cacheId分页
3. 3 返回人工审核字段
4. 4 不关联公开访客身份
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

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.TranslateCacheEntry

`func TranslateCacheEntry(ctx context.Context, op OperationContext, input TranslateCacheEntryInput) (TranslateCacheEntryResult, error)`

作用：请求精选答案翻译

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: TranslateCacheEntryInput`：封闭业务输入；逐字段见types $defs/TranslateCacheEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 源精选 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标语言 |
| expectedAnswerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: TranslateCacheEntryResult`：成功结果；逐字段见types $defs/TranslateCacheEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| jobId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 翻译任务 |
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 来源 |

算法：

1. 1 命中cache缺目标语言才发localization translateCachedAnswer
2. 2 快速档task qa.cache-translate且可信qa costClass继承
3. 3 翻译返回核对基准answerRevision与引用token
4. 4 自动保存localizedAnswer无新增人工批准流程。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：localization任务；不在QA直接调用翻译模型

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：localization:translateCachedAnswer

## qa.CreateCompaction

`func CreateCompaction(ctx context.Context, op OperationContext, input CreateCompactionInput) (CreateCompactionResult, error)`

作用：申请有界历史摘要

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateCompactionInput`：封闭业务输入；逐字段见types $defs/CreateCompactionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| sourceGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 历史代数 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CreateCompactionResult`：成功结果；逐字段见types $defs/CreateCompactionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| compactionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| state | 是 | `{"type":"string","enum":["queued"]}` | 状态 |

算法：

1. 1 取preflight与answer模型较小context窗口
2. 2 总输入达到90%阈值才自动压缩
3. 3 原样保留最近15000token，小窗口retain=min(15000,可用历史预算)
4. 4 固定前summary与未压缩message范围，唯一conversation/sourceGeneration排队。
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

副作用：写compaction和jobs任务

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:createJob

## qa.RunCompaction

`func RunCompaction(ctx context.Context, op OperationContext, input RunCompactionInput) (RunCompactionResult, error)`

作用：执行历史压缩

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RunCompactionInput`：封闭业务输入；逐字段见types $defs/RunCompactionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| compactionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: RunCompactionResult`：成功结果；逐字段见types $defs/RunCompactionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| compaction | 是 | `{"$ref":"#/$defs/Compaction"}` | 摘要 |

算法：

1. 1 输入previousSummary加newMessage范围增量更新，不从头重写全部历史
2. 2 fast task qa.compact且不注入长期记忆
3. 3 继承本轮qa根与费用额度
4. 4 固定recent retained范围，取消后不提交摘要。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用；不删除原始消息

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## qa.CommitCompaction

`func CommitCompaction(ctx context.Context, op OperationContext, input CommitCompactionInput) (CommitCompactionResult, error)`

作用：保护新历史不被旧摘要覆盖

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CommitCompactionInput`：封闭业务输入；逐字段见types $defs/CommitCompactionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| compaction | 是 | `{"$ref":"#/$defs/Compaction"}` | 结果 |
| expectedHistoryGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 源代数 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: CommitCompactionResult`：成功结果；逐字段见types $defs/CommitCompactionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| committed | 是 | `{"type":"boolean"}` | 已提交 |
| currentGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 当前代数 |

算法：

1. 1 比较historyGeneration
2. 2 不相等标superseded
3. 3 CAS summary generation
4. 4 保存hash和输入范围
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写派生摘要，保留原始message

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.CreateDebugTurn

`func CreateDebugTurn(ctx context.Context, op OperationContext, input CreateDebugTurnInput) (CreateDebugTurnResult, error)`

作用：隔离管理诊断

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateDebugTurnInput`：封闭业务输入；逐字段见types $defs/CreateDebugTurnInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| question | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 诊断问题 |
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 显式诊断档位 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CreateDebugTurnResult`：成功结果；逐字段见types $defs/CreateDebugTurnResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| debug | 是 | `{"$ref":"#/$defs/DebugTurn"}` | 独立诊断轮次 |

算法：

1. 1 qa.debug及model.manage委托验证
2. 2 专用namespace和costClass=admin-probe
3. 3 使用真实发布证据且禁cache
4. 4 7天TTL和审计
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：debug_turn写入与MG费用；不进入普通统计/精选

授权：权限 qa.debug；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization, model-gateway:createInference

## qa.ReadDebugTurn

`func ReadDebugTurn(ctx context.Context, op OperationContext, input ReadDebugTurnInput) (ReadDebugTurnResult, error)`

作用：读取管理诊断

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadDebugTurnInput`：封闭业务输入；逐字段见types $defs/ReadDebugTurnInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| debugId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 诊断 |
| afterSequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 游标 |

返回：

- `result: ReadDebugTurnResult`：成功结果；逐字段见types $defs/ReadDebugTurnResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| debug | 是 | `{"$ref":"#/$defs/DebugTurn"}` | 隔离状态 |
| events | 是 | `{"type":"array","items":{"$ref":"#/$defs/AnswerEvent"},"maxItems":256}` | 诊断事件 |

算法：

1. 1 仅创建管理员或superadmin
2. 2 独立namespace查询
3. 3 过期410
4. 4 不查询公开cache/message表
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

授权：权限 qa.debug；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.DeleteConversation

`func DeleteConversation(ctx context.Context, op OperationContext, input DeleteConversationInput) (DeleteConversationResult, error)`

作用：删除自有对话数据

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: DeleteConversationInput`：封闭业务输入；逐字段见types $defs/DeleteConversationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: DeleteConversationResult`：成功结果；逐字段见types $defs/DeleteConversationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| deleted | 是 | `{"type":"boolean"}` | 已删除 |
| cancelledTurns | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 取消轮次数 |

算法：

1. 1 owner校验
2. 2 取消活动turn
3. 3 删除消息/summary/反馈正文
4. 4 保留无正文审计与MG账单引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：会话deleted并移除个人文本

授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:cancelInference

## qa.RecoverTurns

`func RecoverTurns(ctx context.Context, op OperationContext, input RecoverTurnsInput) (RecoverTurnsResult, error)`

作用：恢复崩溃问答

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RecoverTurnsInput`：封闭业务输入；逐字段见types $defs/RecoverTurnsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| now | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 时间 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: RecoverTurnsResult`：成功结果；逐字段见types $defs/RecoverTurnsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| recovered | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 轮次 |
| failed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 失败数 |

算法：

1. 1 读取过期lease
2. 2 MG原invocation状态/流可续才续
3. 3 未派发阶段可继续
4. 4 已派发无法确认则failed不重调用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写turn终态与guard释放

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:readInference, traffic-guard:releaseAdmission

## qa.PublishOutbox

`func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)`

作用：发布问答脱敏事件

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
| pending | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 待发送数 |

算法：

1. 1 领取outbox租约
2. 2 只发IDs/评分/耗时/状态
3. 3 jobs持久ACK后标记
4. 4 三次有限重试后留待下一轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：持久事件，不广播问题正文

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:publishEvent

## qa.PruneExpiredRecords

`func PruneExpiredRecords(ctx context.Context, op OperationContext, input PruneExpiredRecordsInput) (PruneExpiredRecordsResult, error)`

作用：清理到期记录

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PruneExpiredRecordsInput`：封闭业务输入；逐字段见types $defs/PruneExpiredRecordsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| now | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 时间 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: PruneExpiredRecordsResult`：成功结果；逐字段见types $defs/PruneExpiredRecordsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| deletedConversations | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 会话数 |
| deletedDebug | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 诊断数 |
| deletedEvents | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 事件数 |

算法：

1. 1 会话30天且无活动turn
2. 2 debug7天
3. 3 SSE24小时且终态
4. 4 跳过审核cache和费用引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：删除本服务过期个人数据

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.AcquireBarrier

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
3. 3 屏障停止新turn/cache审批/压缩提交
4. 取消活动turn并持久终态和原MG调用引用
5. 导入的visitor cookie不得恢复授权，恢复dataEpoch使旧流/permit失效，原消息只经重新授权可读。
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

## qa.ReleaseBarrier

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

## qa.ExportSnapshot

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
3. 3 屏障停止新turn/cache审批/压缩提交
4. 取消活动turn并持久终态和原MG调用引用
5. 导入的visitor cookie不得恢复授权，恢复dataEpoch使旧流/permit失效，原消息只经重新授权可读。
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

## qa.ValidateSnapshot

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

## qa.PrepareSnapshot

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

## qa.CommitSnapshot

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

## qa.RollbackSnapshot

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

## qa.LoadAdminOverview

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
| items | 是 | `{"type":"array","items":{"$ref":"#/$defs/CacheEntry"},"maxItems":256}` | 后台概览条目 |
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

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.SubmitPrimaryEdit

`export async function submitPrimaryEdit(input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult>`

作用：提交本域明确人工修改

参数：

- `input: SubmitPrimaryEditInput`：本域封闭输入；每个字段规格见types.json $defs/SubmitPrimaryEditInput；schema required/additionalProperties/range全部强制；null禁止。
- `signal: AbortSignal`：界面导航及取消信号；取消后不更新已卸载组件。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 候选 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SubmitPrimaryEditResult`：成功结果；逐字段见types $defs/SubmitPrimaryEditResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/CacheEntry"}` | active精选 |

算法：

1. 1 表单逐字段验证本域schema
2. 2 生成幂等键并携expectedVersion
3. 3 调本域approveCacheEntry API
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

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.ResolveCurrentPageHint

`func ResolveCurrentPageHint(ctx context.Context, op OperationContext, input ResolveCurrentPageHintInput) (ResolveCurrentPageHintResult, error)`

作用：验证当前页面上下文

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResolveCurrentPageHintInput`：封闭业务输入；逐字段见types $defs/ResolveCurrentPageHintInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 浏览器当前页 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 访客语言 |

返回：

- `result: ResolveCurrentPageHintResult`：成功结果；逐字段见types $defs/ResolveCurrentPageHintResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| document | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 验证后来源 |
| pageText | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 有界当前页片段 |
| valid | 是 | `{"type":"boolean"}` | 可用于指代 |

算法：

1. 1 hint不是权限证据
2. 2 content current/visible验证
3. 3 读取标题及被指代小节≤2048tokens
4. 4 页面已撤回/旧revision丢弃hint不扩大可见范围
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

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications, content:readPublishedRevision

## qa.PlanQuestion

`func PlanQuestion(ctx context.Context, op OperationContext, input PlanQuestionInput) (PlanQuestionResult, error)`

作用：补全问题、分类、风险与缓存资格

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PlanQuestionInput`：封闭业务输入；逐字段见types $defs/PlanQuestionInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: PlanQuestionResult`：成功结果；逐字段见types $defs/PlanQuestionResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| question | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原问题 |
| history | 是 | `{"type":"array","items":{"$ref":"#/$defs/Message"},"maxItems":256}` | 近期历史 |
| previousSummary | 否，省略且禁止null | `{"type":"string","minLength":1,"maxLength":65536}` | 摘要 |
| currentPage | 否，省略且禁止null | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 已验证页面 |
| memory | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/MemoryContext"}` | 项目概述 |
| guardSignals | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 无IP风险信号 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| plan | 是 | `{"$ref":"#/$defs/QuestionPlan"}` | 严格前置计划 |

算法：

1. 1 memory:resolveContext taskId=qa.preflight取项目概述，不复用qa.answer记忆
2. 2 fast qa.preflight最多500output tokens结合摘要/历史/经验证当前页补全基准语言问题
3. 3 严格分类/complete/risk，最多4子问题；仍缺指代needs_clarification且禁cache
4. 4 风险交guard，QA不自行封禁IP
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用；保存原问题与canonical问题

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：memory:resolveContext；model-gateway:createInference；traffic-guard:reportModelRisk

## qa.EvaluateCacheEligibility

`func EvaluateCacheEligibility(ctx context.Context, op OperationContext, input EvaluateCacheEligibilityInput) (EvaluateCacheEligibilityResult, error)`

作用：严格缓存完整问题门

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: EvaluateCacheEligibilityInput`：封闭业务输入；逐字段见types $defs/EvaluateCacheEligibilityInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| plan | 是 | `{"$ref":"#/$defs/QuestionPlan"}` | 前置计划 |
| firstTurn | 是 | `{"type":"boolean"}` | 第一轮 |
| currentPageDependent | 是 | `{"type":"boolean"}` | 仍依赖页面 |
| privateContextDependent | 是 | `{"type":"boolean"}` | 依赖访客个人历史 |

返回：

- `result: EvaluateCacheEligibilityResult`：成功结果；逐字段见types $defs/EvaluateCacheEligibilityResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eligible | 是 | `{"type":"boolean"}` | 可查写cache |
| reason | 是 | `{"type":"string","enum":["complete","ambiguous","private","out_of_scope"]}` | 原因 |

算法：

1. 1 必须plan.complete
2. 2 独立canonicalQuestion不残留这/它/第二步未命名来源
3. 3 private/dynamic上下文拒绝共享
4. 4 out_of_scope不写可回答cache
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

## qa.AcquireQueryEmbedding

`func AcquireQueryEmbedding(ctx context.Context, op OperationContext, input AcquireQueryEmbeddingInput) (AcquireQueryEmbeddingResult, error)`

作用：同查询只生成一次嵌入

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: AcquireQueryEmbeddingInput`：封闭业务输入；逐字段见types $defs/AcquireQueryEmbeddingInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| canonicalQuestion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 完整基准查询 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |

返回：

- `result: AcquireQueryEmbeddingResult`：成功结果；逐字段见types $defs/AcquireQueryEmbeddingResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| embedding | 是 | `{"$ref":"#/$defs/QueryEmbedding"}` | 一次向量结果 |

算法：

1. 1 本turn根queryHash/space singleflight
2. 2 knowledge createQueryEmbedding产一次向量
3. 3 校验queryHash/space/vectorHash/dataEpoch
4. 4 同结果供semantic cache和searchKnowledge，cache miss不重新发嵌入。
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

副作用：MG费用一次与本轮短期向量记录

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：knowledge:createQueryEmbedding

## qa.InspectEvidence

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

1. 1 读取本turn持久检索轮数/工具预算
2. 2 委托knowledge inspectEvidence而非直接调用其拥有的MG task
3. 3 校验返回selected IDs仍属于原证据引用
4. 4 原子更新轮数/tool计数，超3轮5次使用已收集材料停止。
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

依赖：knowledge:inspectEvidence, content:validatePublications

## qa.WriteAutomaticCache

`func WriteAutomaticCache(ctx context.Context, op OperationContext, input WriteAutomaticCacheInput) (WriteAutomaticCacheResult, error)`

作用：按old语义自动保存合格答案

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: WriteAutomaticCacheInput`：封闭业务输入；逐字段见types $defs/WriteAutomaticCacheInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: WriteAutomaticCacheResult`：成功结果；逐字段见types $defs/WriteAutomaticCacheResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 完整轮次 |
| plan | 是 | `{"$ref":"#/$defs/QuestionPlan"}` | 完整计划 |
| answer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 答案 |
| citations | 是 | `{"type":"array","items":{"$ref":"#/$defs/Citation"},"maxItems":256}` | 来源 |
| memoryVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 回答记忆 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 否，省略且禁止null | `{"$ref":"#/$defs/CacheEntry"}` | 自动缓存 |
| written | 是 | `{"type":"boolean"}` | 可写 |

算法：

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1 complete/cacheEligible且completed
3. 2 当前反馈unhelpful禁止写且后续负反馈使普通cache失效
4. 3 固定引用hash/memory/space及7天TTL
5. 4 canonical问题基准语言键，访客语言答案子记录，curated=false
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写普通cache不新增人工审核

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:validatePublications

## qa.StoreLocalizedCacheAnswer

`func StoreLocalizedCacheAnswer(ctx context.Context, op OperationContext, input StoreLocalizedCacheAnswerInput) (StoreLocalizedCacheAnswerResult, error)`

作用：保存缓存的多语言回答

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: StoreLocalizedCacheAnswerInput`：封闭业务输入；逐字段见types $defs/StoreLocalizedCacheAnswerInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: StoreLocalizedCacheAnswerResult`：成功结果；逐字段见types $defs/StoreLocalizedCacheAnswerResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 共享问题 |
| sourceAnswerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 基准版本 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标 |
| answer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 自动翻译结果 |
| translationVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规则版本 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/CacheEntry"}` | 新增语言答案 |

算法：

1. 0 cacheEnabled=false直接miss/不写cache，问答记录照常保存。
2. 1 caller localization持久结果引用验证
3. 2 CAS sourceAnswerRevision
4. 3 引用token不变且source仍有效
5. 4 localized answer自动保存并可下次命中
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写语言答案无新增人工审核

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：localization:readCacheAnswerTranslation；content:validatePublications

## qa.ReadCacheCalibration

`func ReadCacheCalibration(ctx context.Context, op OperationContext, input ReadCacheCalibrationInput) (ReadCacheCalibrationResult, error)`

作用：缓存阈值校准

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadCacheCalibrationInput`：封闭业务输入；逐字段见types $defs/ReadCacheCalibrationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| from | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 起点 |
| until | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 终点 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 嵌入空间 |

返回：

- `result: ReadCacheCalibrationResult`：成功结果；逐字段见types $defs/ReadCacheCalibrationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| similarityBuckets | 是 | `{"type":"array","items":{"type":"integer","minimum":0,"maximum":9007199254740991},"maxItems":65536}` | 0.05分桶命中 |
| unhelpfulBuckets | 是 | `{"type":"array","items":{"type":"integer","minimum":0,"maximum":9007199254740991},"maxItems":65536}` | 同桶没帮助次数 |
| recommendedThreshold | 是 | `{"type":"number","minimum":0,"maximum":1}` | 按数据建议阈值 |
| sampleCount | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 观测数 |

算法：

1. 1 qa.manage
2. 2 同space分桶，记录只有similarity/feedback非IP
3. 3 选误命中率≤1%的最小阈值且样本<100保留配置
4. 4 输出建议供人工选择不自动改设置
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读聚合

授权：权限 qa.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## qa.RunWorker

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

### M09-F901 main

文件 `modules/qa/cmd/qa/main.go`。独立进程可执行入口

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

### M09-F902 BuildApplication

文件 `modules/qa/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M09-F903 Serve

文件 `modules/qa/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M09-F904 Shutdown

文件 `modules/qa/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M09-F905 Health

文件 `modules/qa/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M09-F906 Ready

文件 `modules/qa/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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

### M09-F907 mountModuleUI

文件 `modules/qa/ui/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

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
