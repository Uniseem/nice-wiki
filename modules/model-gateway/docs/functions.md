# 统一模型网关逐函数实现合同

每一函数与implementation.json一一对应；输入和结果的全部字段见本节对应JSON Schema，公共类型见根common.schema.json。

## model-gateway.LoadRuntime

`func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)`

作用：启动并验证本服务私有持久化与密钥引用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: LoadRuntimeInput`：封闭业务输入；逐字段见types $defs/LoadRuntimeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| serviceId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 本服务身份 |
| masterKeyRef | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 部署secret引用 |
| dataDirectory | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 私有数据目录 |

返回：

- `result: LoadRuntimeResult`：成功结果；逐字段见types $defs/LoadRuntimeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| ready | 是 | `{"type":"boolean"}` | 是否就绪 |
| catalogVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 载入版本 |

算法：

1. 1 只读取本服务配置和迁移版本
2. 2 校验masterKeyRef可取32字节密钥
3. 3 注册恢复调度
4. 4 所有关键依赖可用才ready
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：建立连接池和worker，不调用供应方

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ValidateProvider

`func ValidateProvider(ctx context.Context, op OperationContext, input ValidateProviderInput) (ValidateProviderResult, error)`

作用：验证供应方配置

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateProviderInput`：封闭业务输入；逐字段见types $defs/ValidateProviderInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| provider | 是 | `{"$ref":"#/$defs/Provider"}` | 待验证供应方 |

返回：

- `result: ValidateProviderResult`：成功结果；逐字段见types $defs/ValidateProviderResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 有效 |
| normalizedBaseUrl | 是 | `{"type":"string","format":"uri","maxLength":2048}` | 归一根URL |

算法：

1. 1 解析URL并按管理员可信network allowlist验证目标
2. 2 外部供应方仅HTTPS，显式本地部署allowlist可HTTP且不允许重定向到其他网段
3. 3 拒绝URL userinfo/fragment及危险附加Authorization重复头
4. 4 固定连接/总/流空闲超时。
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

## model-gateway.PutProvider

`func PutProvider(ctx context.Context, op OperationContext, input PutProviderInput) (PutProviderResult, error)`

作用：CAS保存供应方

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutProviderInput`：封闭业务输入；逐字段见types $defs/PutProviderInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| provider | 是 | `{"$ref":"#/$defs/Provider"}` | 完整配置 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预期版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutProviderResult`：成功结果；逐字段见types $defs/PutProviderResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| provider | 是 | `{"$ref":"#/$defs/Provider"}` | 保存配置 |

算法：

1. 1 校验配置
2. 2 锁provider行
3. 3 比较版本
4. 4 写配置和审计outbox后提交
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：更新providers并发出provider.changed

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.SetProviderEnabled

`func SetProviderEnabled(ctx context.Context, op OperationContext, input SetProviderEnabledInput) (SetProviderEnabledResult, error)`

作用：启停新调用入口

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SetProviderEnabledInput`：封闭业务输入；逐字段见types $defs/SetProviderEnabledInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| providerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 供应方 |
| enabled | 是 | `{"type":"boolean"}` | 新状态 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预期版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SetProviderEnabledResult`：成功结果；逐字段见types $defs/SetProviderEnabledResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| provider | 是 | `{"$ref":"#/$defs/Provider"}` | 新配置 |

算法：

1. 1 读取供应方
2. 2 CAS版本
3. 3 disabled禁止新路由
4. 4 保留正在执行调用记录
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写enabled与版本，不终止已接受供应方请求

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.PutCredential

`func PutCredential(ctx context.Context, op OperationContext, input PutCredentialInput) (PutCredentialResult, error)`

作用：加密存储密钥

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutCredentialInput`：封闭业务输入；逐字段见types $defs/PutCredentialInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| providerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 供应方 |
| secret | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 明文密钥仅本次输入 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预期密钥版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutCredentialResult`：成功结果；逐字段见types $defs/PutCredentialResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| credentialVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新版本 |
| fingerprint | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 末4位及摘要 |

算法：

1. 1 禁止日志请求体
2. 2 读取部署主密钥
3. 3 用随机96bit nonce与providerId/version AAD加密
4. 4 事务写密文并擦除临时缓冲
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：更新credential密文与审计，响应无密钥

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.RotateCredential

`func RotateCredential(ctx context.Context, op OperationContext, input RotateCredentialInput) (RotateCredentialResult, error)`

作用：轮转新请求使用的密钥

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RotateCredentialInput`：封闭业务输入；逐字段见types $defs/RotateCredentialInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| providerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 供应方 |
| newSecret | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 新密钥 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: RotateCredentialResult`：成功结果；逐字段见types $defs/RotateCredentialResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| credentialVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 新版本 |
| retiredVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 旧版本 |

算法：

1. 1 加密新secret
2. 2 CAS替换active版本
3. 3 已派发调用保留旧版本引用
4. 4 最后引用释放后销毁旧密文
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：密钥版本切换；不重复派发供应方请求

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.PutModel

`func PutModel(ctx context.Context, op OperationContext, input PutModelInput) (PutModelResult, error)`

作用：保存不可变模型身份版本

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutModelInput`：封闭业务输入；逐字段见types $defs/PutModelInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| model | 是 | `{"$ref":"#/$defs/Model"}` | 模型配置 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预期版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutModelResult`：成功结果；逐字段见types $defs/PutModelResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| model | 是 | `{"$ref":"#/$defs/Model"}` | 新模型 |

算法：

1. 1 核查provider存在
2. 2 校验能力与维度
3. 3 供应方模型版本/预处理变化创建新modelId
4. 4 CAS保存
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写models及model.changed

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ValidateModel

`func ValidateModel(ctx context.Context, op OperationContext, input ValidateModelInput) (ValidateModelResult, error)`

作用：纯声明验证模型能力

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateModelInput`：封闭业务输入；逐字段见types $defs/ValidateModelInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| model | 是 | `{"$ref":"#/$defs/Model"}` | 声明模型 |

返回：

- `result: ValidateModelResult`：成功结果；逐字段见types $defs/ValidateModelResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 声明合法 |
| embeddingSpace | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 嵌入空间 |

算法：

1. 1 校验能力非空且在允许集合
2. 2 embedding维度1..65536
3. 3 chat/translation输出上限≥1
4. 4 计算嵌入身份
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无网络探测；无收费

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.PutProfile

`func PutProfile(ctx context.Context, op OperationContext, input PutProfileInput) (PutProfileResult, error)`

作用：管理AI档位与费用限制

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutProfileInput`：封闭业务输入；逐字段见types $defs/PutProfileInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profile | 是 | `{"$ref":"#/$defs/Profile"}` | 档位 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预期版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: PutProfileResult`：成功结果；逐字段见types $defs/PutProfileResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profile | 是 | `{"$ref":"#/$defs/Profile"}` | 新档位 |

算法：

1. 1 五档按能力校验
2. 2 orderedModelIds保留管理员顺序且去重最多16个
3. 3 视觉未设可用fast中支持vision模型，rerank未设可用fast
4. 4 CAS保存，仅superadmin可分配。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写profiles；费用规则只在新调用使用

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ResolveProfile

`func ResolveProfile(ctx context.Context, op OperationContext, input ResolveProfileInput) (ResolveProfileResult, error)`

作用：解析服务授权档位

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResolveProfileInput`：封闭业务输入；逐字段见types $defs/ResolveProfileInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 档位ID |
| costClass | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/CostClass"}` | 可信继承费用类 |
| capability | 是 | `{"type":"string","enum":["chat","vision","embedding","rerank","translation"]}` | 能力 |

返回：

- `result: ResolveProfileResult`：成功结果；逐字段见types $defs/ResolveProfileResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profile | 是 | `{"$ref":"#/$defs/Profile"}` | 固定档位快照 |

算法：

1. 1 读取profile快照
2. 2 与ctx.costClass逐字比较
3. 3 校验调用服务允许能力
4. 4 返回版本
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization

## model-gateway.ResolveRoute

`func ResolveRoute(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error)`

作用：确定不可变调用路线

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResolveRouteInput`：封闭业务输入；逐字段见types $defs/ResolveRouteInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profile | 是 | `{"$ref":"#/$defs/Profile"}` | 档位快照 |
| capability | 是 | `{"type":"string","enum":["chat","vision","embedding","rerank","translation"]}` | 目标能力 |

返回：

- `result: ResolveRouteResult`：成功结果；逐字段见types $defs/ResolveRouteResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| models | 是 | `{"type":"array","items":{"$ref":"#/$defs/Model"},"maxItems":256}` | 有序候选 |
| routeVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 固定路线版本 |

算法：

1. 1 固定档位/任务覆盖版本
2. 2 按orderedModelIds剔除disabled并保留顺序
3. 3 embedding必须同一公共space，空间变化新索引而非混用
4. 4 返回最多16个候选，已派发attempt各自独立计费。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
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

## model-gateway.QuoteCost

`func QuoteCost(ctx context.Context, op OperationContext, input QuoteCostInput) (QuoteCostResult, error)`

作用：计算可保守预留的最大费用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: QuoteCostInput`：封闭业务输入；逐字段见types $defs/QuoteCostInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型 |
| inputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 估计输入token |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输出上限 |

返回：

- `result: QuoteCostResult`：成功结果；逐字段见types $defs/QuoteCostResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| maximumCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 已知价保守报价 |
| priceSnapshot | 是 | `{"$ref":"#/$defs/PriceSnapshot"}` | 包含pricingKnown |
| pricingKnown | 是 | `{"type":"boolean"}` | 是否可预留金额 |
| warning | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 原产品缺价警告 |

算法：

1. 1 读取manual优先目录价目
2. 2 有完整费率用整数向上取整
3. 3 缺费率固定price_unknown快照且保留usage，不伪造0成本或阻止调用
4. 4 warning说明未知价调用无法完整受正金额额度约束。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写price snapshot的内容寻址记录

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.CreateInvocation

`func CreateInvocation(ctx context.Context, op OperationContext, input CreateInvocationInput) (CreateInvocationResult, error)`

作用：接受统一AI调用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateInvocationInput`：封闭业务输入；逐字段见types $defs/CreateInvocationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: CreateInvocationResult`：成功结果；逐字段见types $defs/CreateInvocationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 根AI task catalog |
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 任务档位 |
| capability | 是 | `{"type":"string","enum":["chat","vision","embedding","rerank","translation"]}` | 能力 |
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | chat/vision消息其余空 |
| texts | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | embedding/translation/rerank输入其余空 |
| candidateIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | rerank等长ID其余空 |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输出上限embedding为0 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |
| admissionPermit | 否，省略且禁止null | `{"$ref":"../../../contracts/common.schema.json#/$defs/AdmissionPermit"}` | qa费用类必需继承同一个已消费的guard签名permit，MG验证签名/audience/epoch/期限/root归属；其他费用类省略。 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocation | 是 | `{"$ref":"#/$defs/Invocation"}` | accepted状态 |

算法：

1. 1 可信op固定root/costClass/task授权，qa费用类必须继承已消费guard签名admissionPermit
2. 2 MG验证permit签名/audience=model-gateway/dataEpoch/expiresAt/root request归属，从其构造内部FeeScopeBinding；不接受浏览器指定scope
3. 3 按能力检查payload与模型声明，vision资源经media授权；canonical hash含task/model/content及已验证admissionId
4. 4 同幂等键同hash返回原调用，不同CONFLICT；此时不派发供应方请求
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写invocation accepted；不直接调用供应方

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization；media:readAsset

## model-gateway.ReserveCost

`func ReserveCost(ctx context.Context, op OperationContext, input ReserveCostInput) (ReserveCostResult, error)`

作用：原子扣占额度

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReserveCostInput`：封闭业务输入；逐字段见types $defs/ReserveCostInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

返回：

- `result: ReserveCostResult`：成功结果；逐字段见types $defs/ReserveCostResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 统一调用 |
| attemptId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 本次尝试 |
| maximumCost | 否，省略且禁止null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 已知价最大金额 |
| pricingKnown | 是 | `{"type":"boolean"}` | 价目状态 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| reservation | 是 | `{"$ref":"#/$defs/Reservation"}` | held预留 |

算法：

1. 1 从invocation读取MG内部已验证FeeScopeBinding，调用者不能传或覆盖session/network scopes
2. 2 已知价锁site/day/costClass；qa且hourlyLimitsEnabled再锁session/hour和network/hour，network额度=sessionHourlyLimit×networkMultiplier默认5，额度0或总开关关闭表示不限
3. 3 background独立统计不占qa日/访客小时额度；未知价保存price_unknown且金额省略，显示不能完整纳入金额限额warning但允许调用
4. 4 同attempt唯一，新fallback独立预留；旧未知held继续计入同作用域，不释放或伪零
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- BUDGET_EXCEEDED (HTTP429)：MG费用预留触及金额额度，新派发拒绝。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：增加held，写reservation和outbox

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：

## model-gateway.ExtendReservation

`func ExtendReservation(ctx context.Context, op OperationContext, input ExtendReservationInput) (ExtendReservationResult, error)`

作用：预留额不足时扩展

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ExtendReservationInput`：封闭业务输入；逐字段见types $defs/ExtendReservationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| reservationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 预留 |
| additionalCost | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 增量额度；Money.currency=USD且micros非负整数 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ExtendReservationResult`：成功结果；逐字段见types $defs/ExtendReservationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| reservation | 是 | `{"$ref":"#/$defs/Reservation"}` | 扩大预留 |

算法：

1. 1 验证held状态
2. 2 锁额度及预留
3. 3 新额仍在档位/日额度内
4. 4 写版本
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- BUDGET_EXCEEDED (HTTP429)：MG费用预留触及金额额度，新派发拒绝。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只增加额度占用；失败不得继续超额输出

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.DispatchInvocation

`func DispatchInvocation(ctx context.Context, op OperationContext, input DispatchInvocationInput) (DispatchInvocationResult, error)`

作用：派发供应方请求

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: DispatchInvocationInput`：封闭业务输入；逐字段见types $defs/DispatchInvocationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 执行租约 |

返回：

- `result: DispatchInvocationResult`：成功结果；逐字段见types $defs/DispatchInvocationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocation | 是 | `{"$ref":"#/$defs/Invocation"}` | 当前状态 |

算法：

1. 1 验证reserved及lease
2. 2 获取供应方slot
3. 3 将dispatching与attempt持久化后发送
4. 4 记录响应或charge_unknown
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- STALE_RESULT (HTTP409)：dataEpoch、源修订、人工代数、租约或上下文版本已失效；结果丢弃不覆盖。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：唯一实际供应方调用入口，可能产生费用

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：model-gateway.BuildProviderRequest

## model-gateway.BuildProviderRequest

`func BuildProviderRequest(ctx context.Context, op OperationContext, input BuildProviderRequestInput) (BuildProviderRequestResult, error)`

作用：映射统一调用到供应方协议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: BuildProviderRequestInput`：封闭业务输入；逐字段见types $defs/BuildProviderRequestInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| attempt | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 尝试编号1或2 |

返回：

- `result: BuildProviderRequestResult`：成功结果；逐字段见types $defs/BuildProviderRequestResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| method | 是 | `{"type":"string","enum":["POST"]}` | 方法 |
| url | 是 | `{"type":"string","format":"uri","maxLength":2048}` | 供应方目标 |
| headers | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 经内存拼装敏感头 |
| body | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | JSON编码体 |

算法：

1. 1 读取固定模型/价目/密钥版本
2. 2 规范编码能力payload
3. 3 加供应方支持的幂等键
4. 4 返回只供executor内存消费
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNSUPPORTED_CAPABILITY (HTTP422)：声明能力或资源提取类型不受支持。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：短暂解密密钥；返回值禁止日志/持久化

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ClassifyFailure

`func ClassifyFailure(ctx context.Context, op OperationContext, input ClassifyFailureInput) (ClassifyFailureResult, error)`

作用：决定有限重试与收费未知

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ClassifyFailureInput`：封闭业务输入；逐字段见types $defs/ClassifyFailureInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| httpStatus | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 响应码0表示未响应 |
| providerCode | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 协议错误码 |
| bytesSent | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 发送字节数 |
| responseStarted | 是 | `{"type":"boolean"}` | 是否收到响应体 |
| acceptanceProof | 是 | `{"type":"string","enum":["not_accepted","accepted","unknown"]}` | 接收证据 |

返回：

- `result: ClassifyFailureResult`：成功结果；逐字段见types $defs/ClassifyFailureResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| classification | 是 | `{"type":"string","enum":["retry_same","fallback","terminal","charge_unknown"]}` | 处理 |
| retryAfterMs | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 等待毫秒 |
| retainPreviousReservation | 是 | `{"type":"boolean"}` | 未知费用继续held |

算法：

1. 1 responseStarted后不把新模型delta拼进同一已交付结果
2. 2 timeout/限流/格式错误依有序策略可新attempt
3. 3 acceptance未知标旧attempt charge_unknown保留held
4. 4 新attempt先预留后派发，最多每模型一次及格式重试一次且deadline不重置。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无副作用；不将网络timeout等同免费

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.AppendStreamEvent

`func AppendStreamEvent(ctx context.Context, op OperationContext, input AppendStreamEventInput) (AppendStreamEventResult, error)`

作用：持久保存流序列

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: AppendStreamEventInput`：封闭业务输入；逐字段见types $defs/AppendStreamEventInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| event | 是 | `{"$ref":"#/$defs/StreamEvent"}` | 未编号事件 |
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 执行租约 |

返回：

- `result: AppendStreamEventResult`：成功结果；逐字段见types $defs/AppendStreamEventResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 已提交序号 |

算法：

1. 1 验证租约和非终态
2. 2 事务递增sequence
3. 3 持久事件
4. 4 提交后向订阅者广播
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

副作用：stream_events与invocation序号原子更新

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ResumeStream

`func ResumeStream(ctx context.Context, op OperationContext, input ResumeStreamInput) (ResumeStreamResult, error)`

作用：重连读取原流

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ResumeStreamInput`：封闭业务输入；逐字段见types $defs/ResumeStreamInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| afterSequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 排他游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..256 |

返回：

- `result: ResumeStreamResult`：成功结果；逐字段见types $defs/ResumeStreamResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| events | 是 | `{"type":"array","items":{"$ref":"#/$defs/StreamEvent"},"maxItems":256}` | 升序事件 |
| lastSequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 最后序号 |
| terminal | 是 | `{"type":"boolean"}` | 已结束 |

算法：

1. 1 验证调用所有者服务
2. 2 查询sequence>cursor
3. 3 返回最多256项
4. 4 游标超出保留区返回410
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读，不重启模型

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.RecordUsage

`func RecordUsage(ctx context.Context, op OperationContext, input RecordUsageInput) (RecordUsageResult, error)`

作用：接受供应方用量证据

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RecordUsageInput`：封闭业务输入；逐字段见types $defs/RecordUsageInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| providerRequestId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 收费请求标识 |
| inputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 供应方输入token |
| outputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 供应方输出token |
| reportedCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 供应方金额；Money.currency=USD且micros非负整数 |
| usageHash | 是 | `{"type":"string","minLength":64,"maxLength":64,"pattern":"^[0-9a-f]{64}$"}` | 规范使用量hash |

返回：

- `result: RecordUsageResult`：成功结果；逐字段见types $defs/RecordUsageResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| accepted | 是 | `{"type":"boolean"}` | 已记录 |
| duplicate | 是 | `{"type":"boolean"}` | 重复报告 |

算法：

1. 1 校验token与金额范围
2. 2 按provider/request/hash唯一
3. 3 冲突证据进入人工对账
4. 4 保存原始已脱敏证据hash
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

副作用：追加usage evidence；不直接修改最终账本

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.SettleInvocation

`func SettleInvocation(ctx context.Context, op OperationContext, input SettleInvocationInput) (SettleInvocationResult, error)`

作用：结算已知费用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: SettleInvocationInput`：封闭业务输入；逐字段见types $defs/SettleInvocationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 统一调用 |
| attemptId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 本次尝试 |
| usageHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 供应方证据 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SettleInvocationResult`：成功结果；逐字段见types $defs/SettleInvocationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 是 | `{"$ref":"#/$defs/LedgerEntry"}` | 唯一账本 |
| reservation | 是 | `{"$ref":"#/$defs/Reservation"}` | 已结算 |

算法：

1. 1 供应方实际金额优先
2. 2 否则已知固定价目按usage整数算费
3. 3 两者未知保存price_unknown ledger实际金额字段省略不伪0
4. 4 独立attempt结算，晚到证据通过reconciliation补不可变更正entry并更新额度。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写不可变ledger并触发invocation.settled

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.MarkUnknownCharge

`func MarkUnknownCharge(ctx context.Context, op OperationContext, input MarkUnknownChargeInput) (MarkUnknownChargeResult, error)`

作用：保留不确定收费责任

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: MarkUnknownChargeInput`：封闭业务输入；逐字段见types $defs/MarkUnknownChargeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| reason | 是 | `{"type":"string","enum":["timeout","cancel_after_dispatch","connection_lost","usage_missing"]}` | 原因 |
| providerRequestId | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 已知供应方标识 |

返回：

- `result: MarkUnknownChargeResult`：成功结果；逐字段见types $defs/MarkUnknownChargeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocation | 是 | `{"$ref":"#/$defs/Invocation"}` | charge_unknown |

算法：

1. 1 检查已派发
2. 2 设置charge_unknown与证据
3. 3 维持全部held
4. 4 发布reconciliation.requested
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：保留额度和创建对账任务

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:publishEvent

## model-gateway.ReconcileCharge

`func ReconcileCharge(ctx context.Context, op OperationContext, input ReconcileChargeInput) (ReconcileChargeResult, error)`

作用：证据驱动完成未知费用对账

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReconcileChargeInput`：封闭业务输入；逐字段见types $defs/ReconcileChargeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| proofKind | 是 | `{"type":"string","enum":["charged","not_charged"]}` | 供应方证据 |
| actualCost | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 最终金额；Money.currency=USD且micros非负整数 |
| proofHash | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 证据摘要 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: ReconcileChargeResult`：成功结果；逐字段见types $defs/ReconcileChargeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entry | 否；省略不传null | `{"$ref":"#/$defs/LedgerEntry"}` | 已收费账本 |
| reservation | 是 | `{"$ref":"#/$defs/Reservation"}` | 终态预留 |

算法：

1. 1 校验可验证供应方证据/授权人工证明
2. 2 charged走唯一结算
3. 3 not_charged才释放held
4. 4 终态reconciled
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：最终结算或释放预留；保留审计

授权：权限 model.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.CancelInvocation

`func CancelInvocation(ctx context.Context, op OperationContext, input CancelInvocationInput) (CancelInvocationResult, error)`

作用：贯穿取消调用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CancelInvocationInput`：封闭业务输入；逐字段见types $defs/CancelInvocationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| reason | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 取消原因 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: CancelInvocationResult`：成功结果；逐字段见types $defs/CancelInvocationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocation | 是 | `{"$ref":"#/$defs/Invocation"}` | 取消状态 |

算法：

1. 1 未派发则cancelled释放held
2. 2 已派发设cancel_requested
3. 3 发送供应方取消若协议支持
4. 4 无未收费证明则charge_unknown
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：终止本地输出和取消子请求；收费依证据处理

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.GetInvocation

`func GetInvocation(ctx context.Context, op OperationContext, input GetInvocationInput) (GetInvocationResult, error)`

作用：读取调用状态

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: GetInvocationInput`：封闭业务输入；逐字段见types $defs/GetInvocationInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |

返回：

- `result: GetInvocationResult`：成功结果；逐字段见types $defs/GetInvocationResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocation | 是 | `{"$ref":"#/$defs/Invocation"}` | 状态 |

算法：

1. 1 验证服务租户和拥有者
2. 2 读取状态快照
3. 3 不返回prompt/secret
4. 4 返回最近序号
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

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.EstimateTokens

`func EstimateTokens(ctx context.Context, op OperationContext, input EstimateTokensInput) (EstimateTokensResult, error)`

作用：保守估算请求预算

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: EstimateTokensInput`：封闭业务输入；逐字段见types $defs/EstimateTokensInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型 |
| texts | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 输入文本 |
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 消息 |

返回：

- `result: EstimateTokensResult`：成功结果；逐字段见types $defs/EstimateTokensResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| inputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 保守token |
| estimatorVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 估算算法 |

算法：

1. 1 使用声明tokenizer版本
2. 2 未内置tokenizer则ceil(UTF8 bytes/2)+消息开销32
3. 3 乘1.2向上取整
4. 4 不调用供应方
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

## model-gateway.ValidateEmbeddingSpace

`func ValidateEmbeddingSpace(ctx context.Context, op OperationContext, input ValidateEmbeddingSpaceInput) (ValidateEmbeddingSpaceResult, error)`

作用：阻止错空间向量入库

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ValidateEmbeddingSpaceInput`：封闭业务输入；逐字段见types $defs/ValidateEmbeddingSpaceInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 空间 |
| vectors | 是 | `{"type":"array","items":{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536},"maxItems":64}` | 返回向量 |

返回：

- `result: ValidateEmbeddingSpaceResult`：成功结果；逐字段见types $defs/ValidateEmbeddingSpaceResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| valid | 是 | `{"type":"boolean"}` | 校验成功 |
| embeddingSpaceId | 是 | `{"type":"string","minLength":64,"maxLength":64,"pattern":"^[0-9a-f]{64}$"}` | 规范空间ID |

算法：

1. 1 重算空间SHA256
2. 2 每向量维度严格等于dimension
3. 3 拒绝NaN/Infinity
4. 4 l2模式范数误差≤1e-5
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

## model-gateway.AcquireProviderSlot

`func AcquireProviderSlot(ctx context.Context, op OperationContext, input AcquireProviderSlotInput) (AcquireProviderSlotResult, error)`

作用：限制供应方并发

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: AcquireProviderSlotInput`：封闭业务输入；逐字段见types $defs/AcquireProviderSlotInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| providerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 供应方 |
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| leaseSeconds | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 固定30秒 |

返回：

- `result: AcquireProviderSlotResult`：成功结果；逐字段见types $defs/AcquireProviderSlotResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |
| expiresAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 到期 |

算法：

1. 1 锁供应方槽位计数
2. 2 清除过期租约
3. 3 数量小于配置才取得
4. 4 设置30秒租约每10秒续约
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- RATE_LIMITED (HTTP429)：guard准入或本服务供应方并发槽位达到上限。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：持久租约；拒绝超并发

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ReleaseProviderSlot

`func ReleaseProviderSlot(ctx context.Context, op OperationContext, input ReleaseProviderSlotInput) (ReleaseProviderSlotResult, error)`

作用：释放供应方槽位

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReleaseProviderSlotInput`：封闭业务输入；逐字段见types $defs/ReleaseProviderSlotInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| leaseId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 租约 |
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |

返回：

- `result: ReleaseProviderSlotResult`：成功结果；逐字段见types $defs/ReleaseProviderSlotResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| released | 是 | `{"type":"boolean"}` | 释放成功或已释放 |

算法：

1. 1 验证租约绑定调用
2. 2 删除租约
3. 3 对不存在返回released=true
4. 4 不修改账单
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：删除本服务租约

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.EnqueueOutbox

`func EnqueueOutbox(ctx context.Context, op OperationContext, input EnqueueOutboxInput) (EnqueueOutboxResult, error)`

作用：原子记录业务事件

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: EnqueueOutboxInput`：封闭业务输入；逐字段见types $defs/EnqueueOutboxInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventType | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 声明事件名 |
| aggregateId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 聚合ID |
| aggregateVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| payloadHash | 是 | `{"type":"string","minLength":64,"maxLength":64,"pattern":"^[0-9a-f]{64}$"}` | 封闭payload摘要 |

返回：

- `result: EnqueueOutboxResult`：成功结果；逐字段见types $defs/EnqueueOutboxResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| eventId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 事件ID |

算法：

1. 1 必须在业务事务内
2. 2 验证事件schema
3. 3 eventId唯一
4. 4 与聚合变更一并提交
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- UNSUPPORTED_SCHEMA (HTTP422)：输入/快照/模型输出不符合已登记版本或固定字段结构。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：写outbox

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.PublishOutbox

`func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)`

作用：持久发布事件

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
| published | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 已确认数 |
| pending | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 尚未确认数 |

算法：

1. 1 SKIP LOCKED领取100项
2. 2 发jobs并携eventId
3. 3 只有持久ACK后标published
4. 4 失败500/1000/2000ms三次后留待后轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：jobs at-least-once投递

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:publishEvent

## model-gateway.RecoverLeases

`func RecoverLeases(ctx context.Context, op OperationContext, input RecoverLeasesInput) (RecoverLeasesResult, error)`

作用：恢复崩溃中的调用

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RecoverLeasesInput`：封闭业务输入；逐字段见types $defs/RecoverLeasesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| now | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 恢复时刻 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |

返回：

- `result: RecoverLeasesResult`：成功结果；逐字段见types $defs/RecoverLeasesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| recovered | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 恢复数 |
| unknownCharges | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 未知账单数 |

算法：

1. 1 锁到期租约调用
2. 2 reserved未派发重新排队
3. 3 dispatching/streaming证据不足标charge_unknown
4. 4 不自动重新消费模型
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：更新调用状态并对账outbox

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：jobs:publishEvent

## model-gateway.GetLedger

`func GetLedger(ctx context.Context, op OperationContext, input GetLedgerInput) (GetLedgerResult, error)`

作用：管理端查询费用账本

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: GetLedgerInput`：封闭业务输入；逐字段见types $defs/GetLedgerInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 分页游标 |
| limit | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":20}` | 1..100 |
| costClass | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 费用类 |
| from | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 含起点 |
| until | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 排他终点 |

返回：

- `result: GetLedgerResult`：成功结果；逐字段见types $defs/GetLedgerResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/LedgerEntry"},"maxItems":256}` | 费用记录 |
| nextCursor | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 下一页 |

算法：

1. 1 核验model.billing.read
2. 2 限制时间窗≤90天
3. 3 按createdAt/entryId keyset分页
4. 4 返回价目可追溯记录
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：只读且审计读取

授权：权限 model.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.GetPriceBook

`func GetPriceBook(ctx context.Context, op OperationContext, input GetPriceBookInput) (GetPriceBookResult, error)`

作用：读取历史固定价目

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: GetPriceBookInput`：封闭业务输入；逐字段见types $defs/GetPriceBookInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型 |
| effectiveAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 价目时间 |

返回：

- `result: GetPriceBookResult`：成功结果；逐字段见types $defs/GetPriceBookResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| priceSnapshot | 是 | `{"$ref":"#/$defs/PriceSnapshot"}` | 有效价目 |

算法：

1. 1 查不晚于effectiveAt最新不可变价目
2. 2 缺少不猜测价格
3. 3 返回快照ID
4. 4 读取不触发报价
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：无。

授权：权限 model.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.RedactLog

`func RedactLog(ctx context.Context, op OperationContext, input RedactLogInput) (RedactLogResult, error)`

作用：生成最小运行日志

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RedactLogInput`：封闭业务输入；逐字段见types $defs/RedactLogInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| fieldNames | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 字段名称 |
| fieldValues | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 等长字段值 |

返回：

- `result: RedactLogResult`：成功结果；逐字段见types $defs/RedactLogResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| safeNames | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 允许字段 |
| safeValues | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 脱敏值 |

算法：

1. 1 只允许invocationId/rootOperationId/state/code/durationMs/providerId/modelId
2. 2 secret/header/prompt/content全部删除
3. 3 ID合法性检查
4. 4 输出安全字段
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

## model-gateway.QueryUsage

`func QueryUsage(ctx context.Context, op OperationContext, input QueryUsageInput) (QueryUsageResult, error)`

作用：跨服务读取权威费用状态

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: QueryUsageInput`：封闭业务输入；逐字段见types $defs/QueryUsageInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| rootOperationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 可信根操作 |
| from | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 含起点 |
| until | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 排他终点 |

返回：

- `result: QueryUsageResult`：成功结果；逐字段见types $defs/QueryUsageResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entries | 是 | `{"type":"array","items":{"$ref":"#/$defs/LedgerEntry"},"maxItems":256}` | 本根账本 |
| heldCost | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 仍held总额 |
| settledCost | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 已结算总额 |
| unknownInvocationIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 未知收费调用 |

算法：

1. 1 仅调用服务自有rootOperationId或model.manage
2. 2 固定ledger读快照
3. 3 将settled与held分别聚合
4. 4 unknown不能折算零费
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

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ReadModels

`func ReadModels(ctx context.Context, op OperationContext, input ReadModelsInput) (ReadModelsResult, error)`

作用：读取模型能力声明

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadModelsInput`：封闭业务输入；逐字段见types $defs/ReadModelsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| capability | 否；省略不传null | `{"type":"string","enum":["chat","vision","embedding","rerank","translation"]}` | 能力过滤 |

返回：

- `result: ReadModelsResult`：成功结果；逐字段见types $defs/ReadModelsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| models | 是 | `{"type":"array","items":{"$ref":"#/$defs/Model"},"maxItems":256}` | 不含密钥声明 |

算法：

1. 1 service或model.manage
2. 2 仅声明无探测
3. 3 返回不可变模型版本/space
4. 4 disabled仍可读但标不可调用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
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

## model-gateway.ReadProfiles

`func ReadProfiles(ctx context.Context, op OperationContext, input ReadProfilesInput) (ReadProfilesResult, error)`

作用：读取档位声明

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadProfilesInput`：封闭业务输入；逐字段见types $defs/ReadProfilesInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profileId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 可选档位过滤 |

返回：

- `result: ReadProfilesResult`：成功结果；逐字段见types $defs/ReadProfilesResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profiles | 是 | `{"type":"array","items":{"$ref":"#/$defs/Profile"},"maxItems":256}` | 档位声明 |

算法：

1. 1 验caller capability权限
2. 2 过滤allowed costClass
3. 3 返回配置版本
4. 4 不暴露密钥和全站余额
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
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

## model-gateway.AcquireBarrier

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
3. 3 屏障停止新预留/新派发
4. 在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封
5. 部署主密钥不导出。缺密钥恢复后所有供应方disabled。
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

## model-gateway.ReleaseBarrier

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

## model-gateway.ExportSnapshot

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
3. 3 屏障停止新预留/新派发
4. 在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封
5. 部署主密钥不导出。缺密钥恢复后所有供应方disabled。
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

## model-gateway.ValidateSnapshot

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

## model-gateway.PrepareSnapshot

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

## model-gateway.CommitSnapshot

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

## model-gateway.RollbackSnapshot

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

## model-gateway.LoadAdminOverview

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
| items | 是 | `{"type":"array","items":{"$ref":"#/$defs/Provider"},"maxItems":256}` | 后台概览条目 |
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

授权：权限 model.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.SubmitPrimaryEdit

`export async function submitPrimaryEdit(input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult>`

作用：提交本域明确人工修改

参数：

- `input: SubmitPrimaryEditInput`：本域封闭输入；每个字段规格见types.json $defs/SubmitPrimaryEditInput；schema required/additionalProperties/range全部强制；null禁止。
- `signal: AbortSignal`：界面导航及取消信号；取消后不更新已卸载组件。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| provider | 是 | `{"$ref":"#/$defs/Provider"}` | 完整配置 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预期版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 幂等键 |

返回：

- `result: SubmitPrimaryEditResult`：成功结果；逐字段见types $defs/SubmitPrimaryEditResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| provider | 是 | `{"$ref":"#/$defs/Provider"}` | 保存配置 |

算法：

1. 1 表单逐字段验证本域schema
2. 2 生成幂等键并携expectedVersion
3. 3 调本域putProvider API
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

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.PutTaskAssignment

`func PutTaskAssignment(ctx context.Context, op OperationContext, input PutTaskAssignmentInput) (PutTaskAssignmentResult, error)`

作用：超管维护单任务模型覆盖

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutTaskAssignmentInput`：封闭业务输入；逐字段见types $defs/PutTaskAssignmentInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| assignment | 是 | `{"$ref":"#/$defs/TaskAssignment"}` | 任务分配 |
| expectedVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: PutTaskAssignmentResult`：成功结果；逐字段见types $defs/PutTaskAssignmentResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| assignment | 是 | `{"$ref":"#/$defs/TaskAssignment"}` | 新分配 |

算法：

1. 1 taskId在根唯一AI task catalog
2. 2 default tier按任务登记
3. 3 override检查能力
4. 4 CAS保存并发assignment.changed
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

副作用：写任务映射

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.ReadTaskAssignments

`func ReadTaskAssignments(ctx context.Context, op OperationContext, input ReadTaskAssignmentsInput) (ReadTaskAssignmentsResult, error)`

作用：读取任务模型映射

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadTaskAssignmentsInput`：封闭业务输入；逐字段见types $defs/ReadTaskAssignmentsInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 可选任务 |

返回：

- `result: ReadTaskAssignmentsResult`：成功结果；逐字段见types $defs/ReadTaskAssignmentsResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| assignments | 是 | `{"type":"array","items":{"$ref":"#/$defs/TaskAssignment"},"maxItems":256}` | 任务/档位/覆盖声明 |

算法：

1. 1 service能力范围过滤
2. 2 与根目录taskId校验
3. 3 输出固定version
4. 4 禁止模型调用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
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

## model-gateway.RefreshModelCatalog

`func RefreshModelCatalog(ctx context.Context, op OperationContext, input RefreshModelCatalogInput) (RefreshModelCatalogResult, error)`

作用：刷新公开模型目录

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: RefreshModelCatalogInput`：封闭业务输入；逐字段见types $defs/RefreshModelCatalogInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sourceUrl | 是 | `{"type":"string","format":"uri","maxLength":2048}` | 必须https://models.dev/api.json |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: RefreshModelCatalogResult`：成功结果；逐字段见types $defs/RefreshModelCatalogResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| catalogVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 目录代数 |
| updated | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 目录条目 |
| preservedOverrides | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 手动覆盖数 |

算法：

1. 1 host/path固定allowlist下载≤10MiB
2. 2 验证目录封闭schema/价目范围
3. 3 bundled fallback不被删
4. 4 更新remote记录且manualOverride优先
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- INTEGRITY_ERROR (HTTP422)：内容hash、向量空间、证据、插值或不可变记录不一致；隔离结果。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：下载公开目录不发送站点数据

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.CreateConnectionProbe

`func CreateConnectionProbe(ctx context.Context, op OperationContext, input CreateConnectionProbeInput) (CreateConnectionProbeResult, error)`

作用：显式测试连接

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateConnectionProbeInput`：封闭业务输入；逐字段见types $defs/CreateConnectionProbeInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| providerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 供应方 |
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: CreateConnectionProbeResult`：成功结果；逐字段见types $defs/CreateConnectionProbeResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocation | 是 | `{"$ref":"#/$defs/Invocation"}` | 小额诊断调用 |

算法：

1. 1 仅superadmin点击
2. 2 构造固定不含站点内容的1token请求
3. 3 costClass=admin-probe并费用预留
4. 4 显示成功/耗时/账单状态
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- DEPENDENCY_UNAVAILABLE (HTTP503)：必需上游/供应方不可达；收费可能未知时retryable=false且返回调用ID状态查询路径。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：MG真实供应方调用可能收费；当前设计不执行

授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.CreateFallbackAttempt

`func CreateFallbackAttempt(ctx context.Context, op OperationContext, input CreateFallbackAttemptInput) (CreateFallbackAttemptResult, error)`

作用：费用安全保留备用与格式重试

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: CreateFallbackAttemptInput`：封闭业务输入；逐字段见types $defs/CreateFallbackAttemptInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 统一调用 |
| previousAttemptId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 旧尝试 |
| nextModelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 路线后继 |
| reason | 是 | `{"type":"string","enum":["fallback","structured_retry"]}` | 重试原因 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 键 |

返回：

- `result: CreateFallbackAttemptResult`：成功结果；逐字段见types $defs/CreateFallbackAttemptResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| attempt | 是 | `{"$ref":"#/$defs/Attempt"}` | 新reserved尝试 |

算法：

1. 1 确认nextModel属于固定路线且未耗尽
2. 2 旧尝试未知费用标charge_unknown保留held
3. 3 预算不足停止而非释放旧金额
4. 4 创建新attempt/预留后才dispatch
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- BUDGET_EXCEEDED (HTTP429)：MG费用预留触及金额额度，新派发拒绝。
- CONFLICT (HTTP409)：幂等键不同请求、状态迁移或唯一约束冲突，返回原状态证据。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：新增独立尝试/费用预留

授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：本域声明类型和存储；无跨域调用。

## model-gateway.MapAnthropicRequest

`func MapAnthropicRequest(ctx context.Context, op OperationContext, input MapAnthropicRequestInput) (MapAnthropicRequestResult, error)`

作用：映射Anthropic消息协议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: MapAnthropicRequestInput`：封闭业务输入；逐字段见types $defs/MapAnthropicRequestInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 统一输入 |
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型 |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预算 |

返回：

- `result: MapAnthropicRequestResult`：成功结果；逐字段见types $defs/MapAnthropicRequestResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| body | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | Messages JSON |
| system | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 独立系统段 |

算法：

1. 1 system合并独立字段
2. 2 user/assistant保持原序
3. 3 max_tokens固定
4. 4 不将密钥写body
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- UNSUPPORTED_CAPABILITY (HTTP422)：声明能力或资源提取类型不受支持。
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

## model-gateway.MapGeminiRequest

`func MapGeminiRequest(ctx context.Context, op OperationContext, input MapGeminiRequestInput) (MapGeminiRequestResult, error)`

作用：映射Gemini协议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: MapGeminiRequestInput`：封闭业务输入；逐字段见types $defs/MapGeminiRequestInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messages | 是 | `{"type":"array","items":{"$ref":"#/$defs/GatewayMessage"},"maxItems":256}` | 统一输入 |
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型 |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 预算 |

返回：

- `result: MapGeminiRequestResult`：成功结果；逐字段见types $defs/MapGeminiRequestResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| body | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | contents JSON |
| method | 是 | `{"type":"string","enum":["generateContent","streamGenerateContent","embedContent"]}` | 端点能力 |

算法：

1. 1 role assistant→model
2. 2 systemInstruction独立
3. 3 模型能力选端点
4. 4 embedding维度/模型版本固定
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- UNSUPPORTED_CAPABILITY (HTTP422)：声明能力或资源提取类型不受支持。
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

## model-gateway.ParseRerankResponse

`func ParseRerankResponse(ctx context.Context, op OperationContext, input ParseRerankResponseInput) (ParseRerankResponseResult, error)`

作用：解析rerank协议

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ParseRerankResponseInput`：封闭业务输入；逐字段见types $defs/ParseRerankResponseInput；required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidateIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 输入ID |
| responseBody | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 供应方JSON |

返回：

- `result: ParseRerankResponseResult`：成功结果；逐字段见types $defs/ParseRerankResponseResult；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| orderedIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 候选排序 |
| scores | 是 | `{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536}` | 等长分数 |

算法：

1. 1 响应index必须在输入范围
2. 2 无重复且仅既有候选
3. 3 scores有限且排序稳定
4. 4 缺失候选排末不生成正文
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

## model-gateway.RunWorker

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

## model-gateway.ReadFeeBudgetPolicy

`func ReadFeeBudgetPolicy(ctx context.Context, op OperationContext, input ReadFeeBudgetPolicyInput) (ReadFeeBudgetPolicyResult, error)`

作用：读取费用限额配置

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: ReadFeeBudgetPolicyInput`：费用配置封闭输入；完整字段见types，额外字段/null拒绝。

返回：

- `result: ReadFeeBudgetPolicyResult`：费用配置成功结果；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| policy | 是 | `{"$ref":"#/$defs/FeeBudgetPolicy"}` | 费用配置结果 |

算法：

1. 1 校验guard.manage/siteId/active epoch
2. 2 固定policy快照读取，dailyQaLimit=0不限且background独立
3. 3 返回policy，不处理IP或guard频率
4. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：

授权：guard.manage：沿old AI设置账号可维护费用防滥用配置，identity校验；不修改渠道/密钥/档位/任务模型分配。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization


## model-gateway.PutFeeBudgetPolicy

若dailyQaLimit与当前值不同，另校验actor具有superadmin；普通manage_ai只能调整小时策略，不能修改old超管专属全站日费用额度。策略全量输入中的未改日额度不扩大权限。

`func PutFeeBudgetPolicy(ctx context.Context, op OperationContext, input PutFeeBudgetPolicyInput) (PutFeeBudgetPolicyResult, error)`

作用：维护old费用开关与会话小时、网络5倍配置

参数：

- `ctx: context.Context`：Go取消与截止控制；不可nil；与op.deadline取最早截止。
- `op: OperationContext`：可信执行元数据；公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。
- `input: PutFeeBudgetPolicyInput`：费用配置封闭输入；完整字段见types，额外字段/null拒绝。

返回：

- `result: PutFeeBudgetPolicyResult`：费用配置成功结果；仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。
- `err: error`：nil表示成功；非nil映射公共Problem与本函数错误集合；只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。

输入全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| policy | 是 | `{"$ref":"#/$defs/FeeBudgetPolicy"}` | 完整费用配置 |
| expectedVersion | 是 | `{"type":"integer","minimum":0}` | CAS旧版本 |
| idempotencyKey | 是 | `{"type":"string","minLength":1,"maxLength":128}` | 幂等键 |

成功输出全部字段：

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| policy | 是 | `{"$ref":"#/$defs/FeeBudgetPolicy"}` | 费用配置结果 |

算法：

1. 1 校验guard.manage、Money/开关/倍率1..100；network默认5倍，不伪造会话额度美元默认
2. 2 锁policy并CAS expectedVersion；同键同hash返回原配置，不同CONFLICT
3. 3 保存新version和审计，仅影响新预留，既有held/ledger不清零
4. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

错误：

- VALIDATION (HTTP400)：输入字段、范围、字节预算或声明配置不合法，业务事务不提交。
- NOT_FOUND (HTTP404)：对象不存在、游标已过保留期或公开派生结果尚不可用。
- VERSION_MISMATCH (HTTP409)：expectedVersion/expectedRevision不等于当前版本，旧结果不写入。
- UNAUTHENTICATED (HTTP401)：服务凭证、访客凭证或管理会话缺失/过期。
- FORBIDDEN (HTTP403)：身份、租户、对象拥有者或登记权限不满足。
- LOCKED (HTTP409)：本地屏障阻止新业务写入或执行，恢复控制许可例外。
- CANCELLED (HTTP409)：贯穿取消已经生效，禁止后续业务写入与流输出。
- DEADLINE_EXCEEDED (HTTP504)：全链路截止已到，取消子调用；费用未知仍交MG对账。
- INTERNAL (HTTP500)：本服务内部错误，事务回滚且记录脱敏requestId。

副作用：更新MG fee policy与脱敏审计outbox

授权：guard.manage：沿old AI设置账号可维护费用防滥用配置，identity校验；不修改渠道/密钥/档位/任务模型分配。

并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。

依赖：identity:checkAuthorization

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M07-F901 main

文件 `modules/model-gateway/cmd/model-gateway/main.go`。独立进程可执行入口

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

### M07-F902 BuildApplication

文件 `modules/model-gateway/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M07-F903 Serve

文件 `modules/model-gateway/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M07-F904 Shutdown

文件 `modules/model-gateway/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M07-F905 Health

文件 `modules/model-gateway/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M07-F906 Ready

文件 `modules/model-gateway/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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

### M07-F907 mountModuleUI

文件 `modules/model-gateway/ui/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

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
