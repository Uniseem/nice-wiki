# 长期记忆、任务映射与记忆助手逐函数实现合同

每一函数与implementation.json一一对应；输入和结果的全部字段见本节对应JSON Schema，公共类型见根common.schema.json。

## memory.LoadRuntime

`func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)`

作用：启动记忆服务

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
| memoryGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 记忆代数 |

算法：

1. 1 检查entry body/hash
2. 2 验证映射taskId允许集合
3. 3 载入固定预算
4. 4 恢复未确认批准命令
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

副作用：打开私有存储无自动模型调用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ValidateEntry

`func ValidateEntry(ctx context.Context, op OperationContext, input ValidateEntryInput) (ValidateEntryResult, error)`

作用：验证记忆内容与预算

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateEntryInput`：封闭业务输入；逐字段见types $defs/ValidateEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | 人工条目 |

返回：

- `result: ValidateEntryResult`：成功结果；逐字段见types $defs/ValidateEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 合法 |
| normalizedBody | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范正文 |
| bodyHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 摘要 |

算法：

1. 1 NFC/LF/frontmatter合法，正文≤entry.maximumBytes默认1536
2. 2 description/title/tag范围检查
3. 3 内容视作可公开，拒密钥/内部客户信息
4. 4 readonly只约束助手，人工直接编辑仍可。
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

## memory.CreateEntry

`func CreateEntry(ctx context.Context, op OperationContext, input CreateEntryInput) (CreateEntryResult, error)`

作用：创建人工记忆条目

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateEntryInput`：封闭业务输入；逐字段见types $defs/CreateEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | 新条目version0 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CreateEntryResult`：成功结果；逐字段见types $defs/CreateEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | version1 |

算法：

1. 1 ValidateEntry并人工授权
2. 2 投影所有受影响task注入量默认≤5120bytes否则VALIDATION并列taskId
3. 3 CAS版本后写私有temp/fsync/rename权威Markdown并记录commit manifest，失败恢复旧指针
4. 4 数据库revision索引及outbox提交，人工编辑project overview后autoUpdateEnabled=false。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：entry/revision/outbox新增

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization

## memory.UpdateEntry

`func UpdateEntry(ctx context.Context, op OperationContext, input UpdateEntryInput) (UpdateEntryResult, error)`

作用：人工更新记忆

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: UpdateEntryInput`：封闭业务输入；逐字段见types $defs/UpdateEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | 完整新版本 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: UpdateEntryResult`：成功结果；逐字段见types $defs/UpdateEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | 新修订 |

算法：

1. 1 ValidateEntry并人工授权
2. 2 投影所有受影响task注入量默认≤5120bytes否则VALIDATION并列taskId
3. 3 CAS版本后写私有temp/fsync/rename权威Markdown并记录commit manifest，失败恢复旧指针
4. 4 数据库revision索引及outbox提交，人工编辑project overview后autoUpdateEnabled=false。
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

副作用：更新entry并保存revision/outbox

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ArchiveEntry

`func ArchiveEntry(ctx context.Context, op OperationContext, input ArchiveEntryInput) (ArchiveEntryResult, error)`

作用：归档停止未来注入

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ArchiveEntryInput`：封闭业务输入；逐字段见types $defs/ArchiveEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entryId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ArchiveEntryResult`：成功结果；逐字段见types $defs/ArchiveEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | archived |

算法：

1. 1 权限CAS
2. 2 state archived
3. 3 revision记录
4. 4 context版本改变且已有快照可追溯
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

副作用：写归档与上下文失效事件

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.RestoreEntry

`func RestoreEntry(ctx context.Context, op OperationContext, input RestoreEntryInput) (RestoreEntryResult, error)`

作用：恢复归档记忆

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RestoreEntryInput`：封闭业务输入；逐字段见types $defs/RestoreEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entryId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: RestoreEntryResult`：成功结果；逐字段见types $defs/RestoreEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | active |

算法：

1. 1 读取archived
2. 2 CAS
3. 3 active新version不复写旧revision
4. 4 更新memoryGeneration
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

副作用：写新版本及事件

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ReadEntry

`func ReadEntry(ctx context.Context, op OperationContext, input ReadEntryInput) (ReadEntryResult, error)`

作用：读取人工记忆或历史

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadEntryInput`：封闭业务输入；逐字段见types $defs/ReadEntryInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entryId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目 |
| version | 否；省略不传null | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 历史版本 |

返回：

- `result: ReadEntryResult`：成功结果；逐字段见types $defs/ReadEntryResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | 人工正文 |

算法：

1. 1 memory.manage
2. 2 缺version取current
3. 3 历史只读
4. 4 body不写访问日志
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读，记录脱敏审计

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ListEntries

`func ListEntries(ctx context.Context, op OperationContext, input ListEntriesInput) (ListEntriesResult, error)`

作用：管理条目列表

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListEntriesInput`：封闭业务输入；逐字段见types $defs/ListEntriesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| state | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 状态 |
| language | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: ListEntriesResult`：成功结果；逐字段见types $defs/ListEntriesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 条目 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |

算法：

1. 1 权限
2. 2 稳定entryId keyset
3. 3 limit默认20
4. 4 返回hash及正文仅管理会话
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

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ListEntryRevisions

`func ListEntryRevisions(ctx context.Context, op OperationContext, input ListEntryRevisionsInput) (ListEntryRevisionsResult, error)`

作用：读取不可变记忆历史

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListEntryRevisionsInput`：封闭业务输入；逐字段见types $defs/ListEntryRevisionsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entryId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目 |
| beforeVersion | 否；省略不传null | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 排他版本 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: ListEntryRevisionsResult`：成功结果；逐字段见types $defs/ListEntryRevisionsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 历史 |
| nextVersion | 否；省略不传null | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 下一游标 |

算法：

1. 1 权限
2. 2 version降序
3. 3 各bodyHash校验
4. 4 不允许历史修改
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.PutMapping

`func PutMapping(ctx context.Context, op OperationContext, input PutMappingInput) (PutMappingResult, error)`

作用：维护任务映射

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutMappingInput`：封闭业务输入；逐字段见types $defs/PutMappingInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| mapping | 是 | `{"$ref":"#/$defs/TaskMapping"}` | 映射 |
| expectedRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutMappingResult`：成功结果；逐字段见types $defs/PutMappingResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| mapping | 是 | `{"$ref":"#/$defs/TaskMapping"}` | 新版本 |
| mappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 全局修订 |

算法：

1. 1 taskId引用根唯一AI task catalog
2. 2 entry存在且语言范围合理
3. 3 投影该task完整注入量≤默认5120bytes否则拒写列taskId
4. 4 CAS映射及global mappingRevision。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：映射写入与context.changed

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.RemoveMapping

`func RemoveMapping(ctx context.Context, op OperationContext, input RemoveMappingInput) (RemoveMappingResult, error)`

作用：移除任务映射

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RemoveMappingInput`：封闭业务输入；逐字段见types $defs/RemoveMappingInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| mappingId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 映射 |
| expectedRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: RemoveMappingResult`：成功结果；逐字段见types $defs/RemoveMappingResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| removed | 是 | `{"type":"boolean"}` | 已移除 |
| mappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 全局修订 |

算法：

1. 1 权限CAS
2. 2 软删除mapping
3. 3 mappingRevision++
4. 4 只影响后续context
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

副作用：写映射失效与事件

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ReorderMappings

`func ReorderMappings(ctx context.Context, op OperationContext, input ReorderMappingsInput) (ReorderMappingsResult, error)`

作用：稳定重排任务记忆

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReorderMappingsInput`：封闭业务输入；逐字段见types $defs/ReorderMappingsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 任务 |
| mappingIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 全部有效映射完整排列 |
| expectedMappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 全局版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ReorderMappingsResult`：成功结果；逐字段见types $defs/ReorderMappingsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| mappings | 是 | `{"type":"array","items":{"$ref":"#/$defs/TaskMapping"},"maxItems":256}` | 重排 |
| mappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新修订 |

算法：

1. 1 传入集合必须全量无重复
2. 2 CAS全局revision
3. 3 order=索引*10
4. 4 一事务写映射与版本
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

副作用：写排序不修改条目正文

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ListMappings

`func ListMappings(ctx context.Context, op OperationContext, input ListMappingsInput) (ListMappingsResult, error)`

作用：读取任务映射

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ListMappingsInput`：封闭业务输入；逐字段见types $defs/ListMappingsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 任务 |
| language | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 目标语言 |

返回：

- `result: ListMappingsResult`：成功结果；逐字段见types $defs/ListMappingsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| mappings | 是 | `{"type":"array","items":{"$ref":"#/$defs/TaskMapping"},"maxItems":256}` | 映射 |
| mappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 读版本 |

算法：

1. 1 权限
2. 2 taskId允许
3. 3 order/mappingId稳定排序
4. 4 返回全局版本
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

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.PutTaskBudget

`func PutTaskBudget(ctx context.Context, op OperationContext, input PutTaskBudgetInput) (PutTaskBudgetResult, error)`

作用：配置任务注入上限

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutTaskBudgetInput`：封闭业务输入；逐字段见types $defs/PutTaskBudgetInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| budget | 是 | `{"$ref":"#/$defs/TaskBudget"}` | 预算 |
| expectedRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutTaskBudgetResult`：成功结果；逐字段见types $defs/PutTaskBudgetResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| budget | 是 | `{"$ref":"#/$defs/TaskBudget"}` | 新预算 |

算法：

1. 1 bytes1024..65536 entries1..64
2. 2 CAS
3. 3 预算版本进入context hash
4. 4 不扩大模型token额度
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

副作用：写预算及context.changed

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ReadTaskBudget

`func ReadTaskBudget(ctx context.Context, op OperationContext, input ReadTaskBudgetInput) (ReadTaskBudgetResult, error)`

作用：读取注入预算

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadTaskBudgetInput`：封闭业务输入；逐字段见types $defs/ReadTaskBudgetInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 固定任务 |

返回：

- `result: ReadTaskBudgetResult`：成功结果；逐字段见types $defs/ReadTaskBudgetResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| budget | 是 | `{"$ref":"#/$defs/TaskBudget"}` | 预算 |

算法：

1. 1 验证taskId
2. 2 未自定义使用5120bytes/64entries revision0
3. 3 返回有效预算
4. 4 无外部调用
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

## memory.SelectContextEntries

`func SelectContextEntries(ctx context.Context, op OperationContext, input SelectContextEntriesInput) (SelectContextEntriesResult, error)`

作用：按范围确定注入候选

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SelectContextEntriesInput`：封闭业务输入；逐字段见types $defs/SelectContextEntriesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 任务 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| mappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 固定映射代数 |

返回：

- `result: SelectContextEntriesResult`：成功结果；逐字段见types $defs/SelectContextEntriesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 选中候选 |
| mappingIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 对应映射 |

算法：

1. 1 固定mappingRevision读取enabled
2. 2 active条目且language目标优先und
3. 3 priority降序/order/entryId稳定排序
4. 4 相同entry去重
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

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ApplyContextBudget

`func ApplyContextBudget(ctx context.Context, op OperationContext, input ApplyContextBudgetInput) (ApplyContextBudgetResult, error)`

作用：确定性截选完整条目

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ApplyContextBudgetInput`：封闭业务输入；逐字段见types $defs/ApplyContextBudgetInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 排序候选 |
| budget | 是 | `{"$ref":"#/$defs/TaskBudget"}` | 注入预算 |

返回：

- `result: ApplyContextBudgetResult`：成功结果；逐字段见types $defs/ApplyContextBudgetResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 完整预算内条目 |
| omittedIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 因预算省略 |
| byteSize | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预计正文bytes |

算法：

1. 1 精确计算全部选择条目的标题/description/正文边界UTF8bytes
2. 2 预算≤5120默认且每entry≤其limit
3. 3 超预算INTEGRITY_ERROR拒发不静默截条（写时已preflight）
4. 4 保持mapping顺序所有条目不截断。
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

## memory.RenderContext

`func RenderContext(ctx context.Context, op OperationContext, input RenderContextInput) (RenderContextResult, error)`

作用：生成防混淆记忆上下文

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RenderContextInput`：封闭业务输入；逐字段见types $defs/RenderContextInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 预算内条目 |
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 任务 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| mappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 修订 |

返回：

- `result: RenderContextResult`：成功结果；逐字段见types $defs/RenderContextResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| text | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 有边界注入正文 |
| manifest | 是 | `{"$ref":"#/$defs/ContextManifest"}` | 完整版本manifest |

算法：

1. 1 固定renderer先description再正文带entry/version边界
2. 2 作为system前部任务记忆但不是工具授权
3. 3 精确UTF8bytes≤task budget
4. 4 canonical manifest/text SHA256形成versionHash。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
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

## memory.ResolveContext

`func ResolveContext(ctx context.Context, op OperationContext, input ResolveContextInput) (MemoryContext, error)`

作用：跨服务取得版本化记忆上下文

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResolveContextInput`：封闭业务输入；逐字段见types $defs/ResolveContextInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 固定任务 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | BCP47语言 |

返回：

- `result: MemoryContext`：成功结果；逐字段见types $defs/MemoryContext；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

引用：`../../../contracts/common.schema.json#/$defs/MemoryContext`。

算法：

1. 1 callerService允许taskId
2. 2 一只读快照固定entry/mapping/budget
3. 3 Select→Budget→Render
4. 4 回传公共MemoryContext无额外字段
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

副作用：只读，可内容寻址记录manifest；不调模型

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ReadContextManifest

`func ReadContextManifest(ctx context.Context, op OperationContext, input ReadContextManifestInput) (ReadContextManifestResult, error)`

作用：查询版本证据清单

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadContextManifestInput`：封闭业务输入；逐字段见types $defs/ReadContextManifestInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| versionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 上下文版本 |

返回：

- `result: ReadContextManifestResult`：成功结果；逐字段见types $defs/ReadContextManifestResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| manifest | 是 | `{"$ref":"#/$defs/ContextManifest"}` | 清单 |

算法：

1. 1 memory.manage
2. 2 versionHash命中不可变manifest
3. 3 返回IDs/versions不含正文
4. 4 可追溯预算及renderer
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

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.CreateAssistantRun

`func CreateAssistantRun(ctx context.Context, op OperationContext, input CreateAssistantRunInput) (CreateAssistantRunResult, error)`

作用：显式运行记忆助手

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateAssistantRunInput`：封闭业务输入；逐字段见types $defs/CreateAssistantRunInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: CreateAssistantRunResult`：成功结果；逐字段见types $defs/CreateAssistantRunResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| instruction | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 管理员整理目标 |
| entryIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 最多32条人工条目 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| run | 是 | `{"$ref":"#/$defs/AssistantRun"}` | queued |

算法：

1. 1 仅memory.manage即可使用已配置助手，不要求model.manage/superadmin且不修改渠道/档位/任务模型分配
2. 2 根memory.assistant映射读取已配置accurate模型，未配置明确禁调用；固定entry versions/memoryGeneration
3. 3 输入≤16384bytes，readonly只建议人工直接编辑
4. 4 可信admin-probe根任务排队，统一MG执行及计费
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写assistant run与job；无自动修改

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization；jobs:createJob；model-gateway:readTaskAssignments

## memory.BuildAssistantPrompt

`func BuildAssistantPrompt(ctx context.Context, op OperationContext, input BuildAssistantPromptInput) (BuildAssistantPromptResult, error)`

作用：构造只允许提议助手输入

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BuildAssistantPromptInput`：封闭业务输入；逐字段见types $defs/BuildAssistantPromptInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| runId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手 |
| instruction | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 整理目标 |
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 固定人工输入 |

返回：

- `result: BuildAssistantPromptResult`：成功结果；逐字段见types $defs/BuildAssistantPromptResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 结构化提示 |

算法：

1. 1 固定指令只输出proposal schema
2. 2 明示无写权限
3. 3 输入记忆作为数据隔离
4. 4 禁生成密钥/IP/会话记录
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

## memory.RunAssistant

`func RunAssistant(ctx context.Context, op OperationContext, input RunAssistantInput) (RunAssistantResult, error)`

作用：通过MG产生候选提议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RunAssistantInput`：封闭业务输入；逐字段见types $defs/RunAssistantInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| runId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手 |
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 输入 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: RunAssistantResult`：成功结果；逐字段见types $defs/RunAssistantResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| run | 是 | `{"$ref":"#/$defs/AssistantRun"}` | review |
| proposals | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryProposal"},"maxItems":256}` | 待审核 |

算法：

1. 1 lease/cancel检查
2. 2 根memory.assistant accurate档max1500output token，可信admin-probe根
3. 3 严格解析new/update/delete/noop候选和readonly规则
4. 4 不写active，只保存pending差异候选。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用与pending proposal暂存

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:createInference

## memory.ValidateProposal

`func ValidateProposal(ctx context.Context, op OperationContext, input ValidateProposalInput) (ValidateProposalResult, error)`

作用：验证候选提议闭包

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateProposalInput`：封闭业务输入；逐字段见types $defs/ValidateProposalInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposal | 是 | `{"$ref":"#/$defs/MemoryProposal"}` | 候选 |
| sourceGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 源记忆代数 |

返回：

- `result: ValidateProposalResult`：成功结果；逐字段见types $defs/ValidateProposalResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 合法 |
| proposalHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范摘要 |

算法：

1. 1 kind对应字段required且其他字段禁止
2. 2 entry_update/archive必须目标及旧version
3. 3 glossary必须目标语言/术语/旧glossaryGeneration
4. 4 无批准人允许pending
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
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

## memory.CommitAssistantProposals

`func CommitAssistantProposals(ctx context.Context, op OperationContext, input CommitAssistantProposalsInput) (CommitAssistantProposalsResult, error)`

作用：提交候选防旧覆盖

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CommitAssistantProposalsInput`：封闭业务输入；逐字段见types $defs/CommitAssistantProposalsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| runId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手 |
| proposals | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryProposal"},"maxItems":256}` | 已校验候选 |
| expectedSourceGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输入代数 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: CommitAssistantProposalsResult`：成功结果；逐字段见types $defs/CommitAssistantProposalsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| run | 是 | `{"$ref":"#/$defs/AssistantRun"}` | review或superseded |
| accepted | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 保存候选数 |

算法：

1. 1 当前memoryGeneration一致
2. 2 每提议目标expectedVersion仍一致
3. 3 不一致run superseded
4. 4 pending proposals与run review同事务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只写审核候选；不应用内容

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ReadAssistantRun

`func ReadAssistantRun(ctx context.Context, op OperationContext, input ReadAssistantRunInput) (ReadAssistantRunResult, error)`

作用：管理助手提议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadAssistantRunInput`：封闭业务输入；逐字段见types $defs/ReadAssistantRunInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| runId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务 |

返回：

- `result: ReadAssistantRunResult`：成功结果；逐字段见types $defs/ReadAssistantRunResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| run | 是 | `{"$ref":"#/$defs/AssistantRun"}` | 状态 |
| proposals | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryProposal"},"maxItems":256}` | 审核候选 |

算法：

1. 1 memory.manage
2. 2 原创建人或有权限管理员可读
3. 3 返回候选旧版本证据
4. 4 不含密钥/原模型敏感日志
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

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ApproveProposal

`func ApproveProposal(ctx context.Context, op OperationContext, input ApproveProposalInput) (ApproveProposalResult, error)`

作用：人工批准具体提议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ApproveProposalInput`：封闭业务输入；逐字段见types $defs/ApproveProposalInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposalId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 候选 |
| expectedApprovalRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 批准版本 |
| expectedPayloadHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 所见候选摘要 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ApproveProposalResult`：成功结果；逐字段见types $defs/ApproveProposalResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposal | 是 | `{"$ref":"#/$defs/MemoryProposal"}` | approved或applying |

算法：

1. 1 必须actor.kind=admin且memory.manage
2. 2 展示payload hash与版本匹配
3. 3 CAS pending→approved记录approvedBy
4. 4 entry类调用应用事务，glossary写批准command outbox
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写批准记录；非人工身份403

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization

## memory.RejectProposal

`func RejectProposal(ctx context.Context, op OperationContext, input RejectProposalInput) (RejectProposalResult, error)`

作用：人工拒绝未应用候选

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RejectProposalInput`：封闭业务输入；逐字段见types $defs/RejectProposalInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposalId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 候选 |
| reason | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 人工拒绝理由 |
| expectedApprovalRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: RejectProposalResult`：成功结果；逐字段见types $defs/RejectProposalResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposal | 是 | `{"$ref":"#/$defs/MemoryProposal"}` | rejected |

算法：

1. 1 只pending可拒绝
2. 2 CAS revision
3. 3 rejected记录人/理由
4. 4 不影响active记忆
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

副作用：审核状态写入

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ApplyEntryProposal

`func ApplyEntryProposal(ctx context.Context, op OperationContext, input ApplyEntryProposalInput) (ApplyEntryProposalResult, error)`

作用：应用批准的记忆内容

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ApplyEntryProposalInput`：封闭业务输入；逐字段见types $defs/ApplyEntryProposalInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposalId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 已批准entry提议 |
| expectedEntryVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧条目版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ApplyEntryProposalResult`：成功结果；逐字段见types $defs/ApplyEntryProposalResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposal | 是 | `{"$ref":"#/$defs/MemoryProposal"}` | applied或conflicted |
| entry | 否；省略不传null | `{"$ref":"#/$defs/MemoryEntry"}` | 新条目 |

算法：

1. 1 必须已人工批准且hash一致
2. 2 readonly=true的助手候选禁止应用仅建议直接人工编辑
3. 3 CAS旧entry version并投影全部映射任务预算
4. 4 原子权威Markdown新对象+manifest与revision索引提交，proposal applied及context.changed同事务。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：更新记忆与提议；不可绕过人工批准

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.EnqueueGlossaryCommand

`func EnqueueGlossaryCommand(ctx context.Context, op OperationContext, input EnqueueGlossaryCommandInput) (EnqueueGlossaryCommandResult, error)`

作用：产生已批准术语命令

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: EnqueueGlossaryCommandInput`：封闭业务输入；逐字段见types $defs/EnqueueGlossaryCommandInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposalId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 已批准术语 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: EnqueueGlossaryCommandResult`：成功结果；逐字段见types $defs/EnqueueGlossaryCommandResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| commandId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 命令ID |
| proposal | 是 | `{"$ref":"#/$defs/MemoryProposal"}` | applying |

算法：

1. 1 校验kind=glossary及admin approval
2. 2 规范ApprovedGlossaryProposal含批准修订/hash
3. 3 唯一proposalId command
4. 4 applying与outbox同事务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写发送localization命令队列；此函数不调用localization

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.DispatchGlossaryCommand

`func DispatchGlossaryCommand(ctx context.Context, op OperationContext, input DispatchGlossaryCommandInput) (DispatchGlossaryCommandResult, error)`

作用：异步发送批准术语命令

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: DispatchGlossaryCommandInput`：封闭业务输入；逐字段见types $defs/DispatchGlossaryCommandInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| commandId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 命令 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |

返回：

- `result: DispatchGlossaryCommandResult`：成功结果；逐字段见types $defs/DispatchGlossaryCommandResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| accepted | 是 | `{"type":"boolean"}` | localization持久接收 |
| receiptId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 回执 |

算法：

1. 1 领取独立command租约
2. 2 HTTP请求localization applyApprovedGlossaryProposal
3. 3 同proposal/hash重试最多3次500/1000/2000ms
4. 4 ACK仅accepted，终态等事件
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：localization接收命令；无同步memory回调依赖

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：localization:applyApprovedGlossaryProposal

## memory.HandleGlossaryProposalResult

`func HandleGlossaryProposalResult(ctx context.Context, op OperationContext, input HandleGlossaryProposalResultInput) (HandleGlossaryProposalResultResult, error)`

作用：消费术语批准结果

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: HandleGlossaryProposalResultInput`：封闭业务输入；逐字段见types $defs/HandleGlossaryProposalResultInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 回执事件 |
| proposalId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提议 |
| proposalHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 摘要 |
| result | 是 | `{"type":"string","enum":["applied","rejected"]}` | 终态 |
| termId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 术语 |
| glossaryGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 术语代数 |

返回：

- `result: HandleGlossaryProposalResultResult`：成功结果；逐字段见types $defs/HandleGlossaryProposalResultResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposal | 是 | `{"$ref":"#/$defs/MemoryProposal"}` | 终态 |

算法：

1. 1 inbox去重
2. 2 proposal/hash/approvalRevision匹配
3. 3 applying→applied或conflicted
4. 4 不请求localization/memory resolve再循环
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

副作用：写proposal终态与审计outbox

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.CancelAssistantRun

`func CancelAssistantRun(ctx context.Context, op OperationContext, input CancelAssistantRunInput) (CancelAssistantRunResult, error)`

作用：取消未完成助手调用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CancelAssistantRunInput`：封闭业务输入；逐字段见types $defs/CancelAssistantRunInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| runId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手 |
| reason | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原因 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CancelAssistantRunResult`：成功结果；逐字段见types $defs/CancelAssistantRunResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| run | 是 | `{"$ref":"#/$defs/AssistantRun"}` | 取消状态 |

算法：

1. 1 人工权限
2. 2 写cancel
3. 3 MG cancelInference/jobs取消
4. 4 pending未批准候选标rejected，已批准不回滚
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：终止助手及子调用，不自行释放MG账单

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:cancelInference, jobs:cancelJob

## memory.RecoverAssistantLeases

`func RecoverAssistantLeases(ctx context.Context, op OperationContext, input RecoverAssistantLeasesInput) (RecoverAssistantLeasesResult, error)`

作用：恢复助手执行与批准命令

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RecoverAssistantLeasesInput`：封闭业务输入；逐字段见types $defs/RecoverAssistantLeasesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| now | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 时间 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: RecoverAssistantLeasesResult`：成功结果；逐字段见types $defs/RecoverAssistantLeasesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| recovered | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 助手数 |
| superseded | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧候选数 |

算法：

1. 1 到期lease禁止旧写
2. 2 原MG流可续才续
3. 3 sourceGeneration已变superseded
4. 4 command ACK未终态重读事件状态不重复批准
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写恢复状态/重排command

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:readInference, jobs:createJob

## memory.PublishOutbox

`func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)`

作用：发布记忆引用式事件

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

1. 1 领取租约
2. 2 payload只含entry/proposal ID与versionHash
3. 3 jobs持久ACK标published
4. 4 有限重试后保留下一轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：持久事件不广播记忆正文

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:publishEvent

## memory.PruneContextSnapshots

`func PruneContextSnapshots(ctx context.Context, op OperationContext, input PruneContextSnapshotsInput) (PruneContextSnapshotsResult, error)`

作用：回收无引用派生注入清单

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PruneContextSnapshotsInput`：封闭业务输入；逐字段见types $defs/PruneContextSnapshotsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| olderThan | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 默认90天前 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: PruneContextSnapshotsResult`：成功结果；逐字段见types $defs/PruneContextSnapshotsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| deleted | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 删除数 |
| retained | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 仍引用数 |

算法：

1. 1 不删除entry历史
2. 2 无活跃任务/审计保留引用才回收
3. 3 验证私有文件范围
4. 4 删除manifest记录
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：删除本服务派生快照

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.AcquireBarrier

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
3. 3 屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令
4. 导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。
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

## memory.ReleaseBarrier

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

## memory.ExportSnapshot

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
3. 3 屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令
4. 导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。
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

## memory.ValidateSnapshot

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

## memory.PrepareSnapshot

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

## memory.CommitSnapshot

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

## memory.RollbackSnapshot

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

## memory.LoadAdminOverview

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
| items | 是 | `{"type":"array","items":{"$ref":"#/$defs/MemoryEntry"},"maxItems":256}` | 后台概览条目 |
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

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.SubmitPrimaryEdit

`export async function submitPrimaryEdit(input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult>`

作用：提交本域明确人工修改

参数：

- `input: SubmitPrimaryEditInput`：本域封闭输入；每个字段规格见types.json $defs/SubmitPrimaryEditInput；schema required/additionalProperties/range全部强制；null禁止。
- `signal: AbortSignal`：界面导航及取消信号；取消后不更新已卸载组件。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | 完整新版本 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SubmitPrimaryEditResult`：成功结果；逐字段见types $defs/SubmitPrimaryEditResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/MemoryEntry"}` | 新修订 |

算法：

1. 1 表单逐字段验证本域schema
2. 2 生成幂等键并携expectedVersion
3. 3 调本域updateEntry API
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

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ValidateProjectedBudgets

`func ValidateProjectedBudgets(ctx context.Context, op OperationContext, input ValidateProjectedBudgetsInput) (ValidateProjectedBudgetsResult, error)`

作用：保存前完整投影注入预算

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateProjectedBudgetsInput`：封闭业务输入；逐字段见types $defs/ValidateProjectedBudgetsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposedEntry | 否；省略不传null | `{"$ref":"#/$defs/MemoryEntry"}` | 候选条目 |
| proposedMapping | 否；省略不传null | `{"$ref":"#/$defs/TaskMapping"}` | 候选映射 |

返回：

- `result: ValidateProjectedBudgetsResult`：成功结果；逐字段见types $defs/ValidateProjectedBudgetsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 是否可保存 |
| affectedTaskIds | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 受影响任务 |
| projectedBytes | 是 | `{"type":"array","items":{"type":"integer","minimum":0,"maximum":9007199254740991},"maxItems":65536}` | 等长UTF8量 |
| overBudgetTaskIds | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 超预算任务 |

算法：

1. 1 读固定entry/mapping snapshot
2. 2 应用候选变更到内存投影不持久
3. 3 按每任务/语言完整renderer bytes计算
4. 4 任一超budget返回VALIDATION field issues带taskId不含正文
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

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.UpdateProjectOverview

`func UpdateProjectOverview(ctx context.Context, op OperationContext, input UpdateProjectOverviewInput) (UpdateProjectOverviewResult, error)`

作用：自动维护项目概述并保护人工修改

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: UpdateProjectOverviewInput`：封闭业务输入；逐字段见types $defs/UpdateProjectOverviewInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| publishedCountSinceUpdate | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新发布篇数 |
| scheduledDaily | 是 | `{"type":"boolean"}` | 每日触发 |
| expectedEntryVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 项目概述旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: UpdateProjectOverviewResult`：成功结果；逐字段见types $defs/UpdateProjectOverviewResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| updated | 是 | `{"type":"boolean"}` | 是否更新 |
| entry | 否；省略不传null | `{"$ref":"#/$defs/MemoryEntry"}` | 新概述 |

算法：

1. 1 source=auto且autoUpdateEnabled才继续
2. 2 daily或publishedCount≥20触发
3. 3 content当前公开标题摘要固定快照→fast task memory.overview
4. 4 CAS entryVersion/autoUpdateEnabled并预算preflight保存，否则旧结果superseded
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG费用与自动概述文件更新，人工锁后不写

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：content:readPublishedRevision, model-gateway:createInference

## memory.AppendAssistantMessage

`func AppendAssistantMessage(ctx context.Context, op OperationContext, input AppendAssistantMessageInput) (AppendAssistantMessageResult, error)`

作用：保存可连续的记忆助手对话

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: AppendAssistantMessageInput`：封闭业务输入；逐字段见types $defs/AppendAssistantMessageInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 空则新建 |
| body | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 管理员消息 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 会话版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: AppendAssistantMessageResult`：成功结果；逐字段见types $defs/AppendAssistantMessageResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手会话 |
| message | 是 | `{"$ref":"#/$defs/AssistantMessage"}` | 新消息 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 会话新版本 |

算法：

1. 1 actor admin且memory.manage
2. 2 owner及CAS版本
3. 3 body≤8192bytes且不接受内部密钥
4. 4 追加ordinal消息和30天可调TTL，需整理时另createAssistantRun
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写助手会话消息，无自动模型调用

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.ReadAssistantConversation

`func ReadAssistantConversation(ctx context.Context, op OperationContext, input ReadAssistantConversationInput) (ReadAssistantConversationResult, error)`

作用：读取记忆助手连续历史

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadAssistantConversationInput`：封闭业务输入；逐字段见types $defs/ReadAssistantConversationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: ReadAssistantConversationResult`：成功结果；逐字段见types $defs/ReadAssistantConversationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/AssistantMessage"},"maxItems":256}` | 对话 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |

算法：

1. 1 owner/admin权限
2. 2 到期NOT_FOUND
3. 3 ordinal keyset分页
4. 4 展示每run对应差异提议不自动应用
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

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## memory.DeleteAssistantConversation

`func DeleteAssistantConversation(ctx context.Context, op OperationContext, input DeleteAssistantConversationInput) (DeleteAssistantConversationResult, error)`

作用：清空助手对话

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: DeleteAssistantConversationInput`：封闭业务输入；逐字段见types $defs/DeleteAssistantConversationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: DeleteAssistantConversationResult`：成功结果；逐字段见types $defs/DeleteAssistantConversationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| deleted | 是 | `{"type":"boolean"}` | 已删除 |

算法：

1. 1 owner及memory.manage
2. 2 取消未完成关联run
3. 3 删除聊天正文保留批准审计与entry历史
4. 4 同key返回同结果
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：删除管理员聊天与取消子任务

授权：权限 memory.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway:cancelInference

## memory.RunWorker

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

### M11-F901 main

文件 `modules/memory/cmd/memory/main.go`。独立进程可执行入口

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

### M11-F902 BuildApplication

文件 `modules/memory/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M11-F903 Serve

文件 `modules/memory/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M11-F904 Shutdown

文件 `modules/memory/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M11-F905 Health

文件 `modules/memory/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M11-F906 Ready

文件 `modules/memory/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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

### M11-F907 mountModuleUI

文件 `modules/memory/ui/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

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
