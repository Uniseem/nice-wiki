# identity 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 152 个函数：约束 31 个，参考 121 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## identity.ReadRegistrationState

- 级别：约束
- 文件：`modules/identity/internal/registration_service.go`
- 作用：决定后台显示首次注册或登录

```go
func ReadRegistrationState(ctx context.Context, op OperationContext, input ReadRegistrationStateInput) (ReadRegistrationStateResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadRegistrationStateInput` | 决定后台显示首次注册或登录的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadRegistrationStateResult` | 决定后台显示首次注册或登录的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 读不可逆bootstrapClaim与唯一superadmin索引
2. 首次无账户返回registrationOpen=true，不要求初始化码
3. site-config状态只决定登录后的向导，不重开注册
4. active为空时另查不随恢复替换的control bootstrap_claim；已有reserved或committed都不向新请求开放注册

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：anonymous
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`site-config:readInitializationStatus`、`identity.readBootstrapClaim`、`identity.registrationRuntimeFromContext`、`identity.identityStoreFromContext`

## identity.RegisterFirstSuperadmin

- 级别：约束
- 文件：`modules/identity/internal/registration_service.go`
- 作用：首个注册账户原子成为唯一超管

```go
func RegisterFirstSuperadmin(ctx context.Context, op OperationContext, input RegisterFirstSuperadminInput) (RegisterFirstSuperadminResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RegisterFirstSuperadminInput` | 首个注册账户原子成为唯一超管的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RegisterFirstSuperadminResult` | 首个注册账户原子成为唯一超管的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 账户/控制决定/回执/索引或初始化归属完整性不一致；UNAUTHENTICATED: 当前会话失效或账号口令不正确；FORBIDDEN: 可信服务/网络作用域或本域当前功能授权不满足 |

**步骤**

1. 匿名凭证动作只接受已认证portal，回执/claim主体固定service portal/system，清可选用户proof；cookie或委托变化不改变同op幂等作用域
2. portal固定Origin与签名网络作用域由入口建立；以规范账号名调用guard begin action=bootstrap，不新增初始化码/MFA/邮箱
3. 两次口令一致且HashPassword；site-config未初始化且尚无协调站点才可首次注册
4. control.db先持久不可逆bootstrap_claim reserved：固定原operation/caller/actor/HMAC/keyId/dataEpoch/accountId/siteId；同原op同epoch可续未决，异请求或换epochConflict
5. 在本域账号互斥和active事务内核对从无账户，写唯一superadmin、凭据/功能、session与安全回执/outbox；guard authenticated成功后提交
6. active提交与账号索引文件不宣称跨库/跨文件原子：control journal补齐派生索引；active安全回执可恢复control reserved→committed，状态永不回退或删除
7. 首次成功返回raw token和/admin/setup；重放只读安全回执并提示重新登录，不重复建session；control committed而active为空失败关闭，不重开注册

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `RATE_LIMITED`：频率、并发或容量上限已达，返回 retryAfter（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：账户/控制决定/回执/索引或初始化归属完整性不一致（不可重试）
- `UNAUTHENTICATED`：当前会话失效或账号口令不正确（不可重试）
- `FORBIDDEN`：可信服务/网络作用域或本域当前功能授权不满足（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：anonymous；无需初始化码，部署后立即注册或先不暴露端口
- 并发：bootstrapClaim+isSuperadmin唯一约束同事务，首注册至多一个
- 幂等：相同operation/输入只存HMAC摘要与安全account/session/credential引用；仅首次事务返回敏感token，重复返回CONFLICT要求重新登录，不重复建会话；keyId不匹配失败关闭
- 依赖：`identity.HashPassword`、`identity.IssueSessionTokens`、`identity.NormalizeAccountName`、`identity.beginIdentityLogin`、`identity.finishIdentityLogin`、`identity.privateIdentityDigest`、`identity.readIdentityReceipt`、`identity.saveIdentityReceipt`、`identity.reserveBootstrap`、`identity.markBootstrapCommitted`、`identity.insertAccount`、`identity.selectedPermissionNames`、`identity.insertSession`、`identity.PublishIdentityEvent`、`identity.commitAccountWrite`、`identity.syncAccountIndex`、`identity.sensitiveReplayError`、`identity.registrationRuntimeFromContext`、`identity.anonymousIdentityOperation`、`identity.identityStoreFromContext`

## identity.WithRegistrationRuntime

- 级别：参考
- 文件：`modules/identity/internal/registration_service.go`
- 作用：注入初始化权威适配

```go
func WithRegistrationRuntime(ctx context.Context, runtime RegistrationRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 注入初始化权威适配

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.registrationRuntimeFromContext

- 级别：参考
- 文件：`modules/identity/internal/registration_service.go`
- 作用：取得进程注入的初始化适配，禁止未知当未初始化

```go
func registrationRuntimeFromContext(ctx context.Context) RegistrationRuntime
```

**输入**

无

**输出**

无

**步骤**

1. 取得进程注入的初始化适配，禁止未知当未初始化

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.WithRegistrationRuntime`、`identity.ReadInitializationStatus`

## identity.ReadInitializationStatus

- 级别：参考
- 文件：`modules/identity/internal/registration_service.go`
- 作用：通过版本化site-config客户端读取并核对安全阶段与归属

```go
func (runtime RegistrationClient) ReadInitializationStatus(ctx context.Context, op OperationContext) (InitializationStatus, error)
```

**输入**

无

**输出**

无

**步骤**

1. 通过版本化site-config客户端读取并核对安全阶段与归属

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`site-config:readInitializationStatus`

## identity.readBootstrapClaim

- 级别：参考
- 文件：`modules/identity/internal/registration_service.go`
- 作用：读取不随active恢复替换的不可逆注册决定

```go
func readBootstrapClaim(ctx context.Context, control *sql.DB) (bootstrapClaim, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取不随active恢复替换的不可逆注册决定

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.identityDBError`

## identity.reserveBootstrap

- 级别：参考
- 文件：`modules/identity/internal/registration_service.go`
- 作用：仅相同原op/caller/actor/HMAC/keyId/dataEpoch允许续未决，其他Conflict

```go
func reserveBootstrap(ctx context.Context, op OperationContext, store *Store, digest, keyID string) (bootstrapClaim, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅相同原op/caller/actor/HMAC/keyId/dataEpoch允许续未决，其他Conflict

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.readBootstrapClaim`、`identity.identityDBError`

## identity.markBootstrapCommitted

- 级别：参考
- 文件：`modules/identity/internal/registration_service.go`
- 作用：控制决定仅reserved到committed单向收敛

```go
func markBootstrapCommitted(ctx context.Context, store *Store, claim bootstrapClaim) error
```

**输入**

无

**输出**

无

**步骤**

1. 控制决定仅reserved到committed单向收敛

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.identityDBError`

## identity.recoverBootstrapClaim

- 级别：参考
- 文件：`modules/identity/internal/registration_service.go`
- 作用：核控制身份与active超管，按已提交安全回执补记控制终局

```go
func recoverBootstrapClaim(store *Store) error
```

**输入**

无

**输出**

无

**步骤**

1. 核控制身份与active超管，按已提交安全回执补记控制终局

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.readBootstrapClaim`、`identity.markBootstrapCommitted`、`identity.identityDBError`

## identity.CreateAccount

- 级别：约束
- 文件：`modules/identity/internal/account_service.go`
- 作用：超管创建子账号并标记强改

```go
func CreateAccount(ctx context.Context, op OperationContext, input CreateAccountInput) (CreateAccountResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CreateAccountInput` | 超管创建子账号并标记强改的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateAccountResult` | 超管创建子账号并标记强改的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 账户/控制决定/回执/索引或初始化归属完整性不一致 |

**步骤**

1. 唯一超管当前会话在事务内核对active、credential/permission/session/auth/data水位与superadmin；账号名规范不可变，省略权限默认既有10项全选
2. 私有输入HMAC含动作/caller/epoch并记录keyId，不存可离线猜口令的普通SHA；完整安全结果回执同op重放，不重复新建
3. HashPassword后在单个active事务插入mustChangePassword=true子账号、凭据、功能与身份outbox及完整安全回执；唯一名冲突Conflict，账号总数上限10000
4. 账号写提交前记identity.account-index决定，active提交后派生账号索引并完结；启动从active权威补齐未决索引，不宣称数据库事务覆盖文件

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：账户/控制决定/回执/索引或初始化归属完整性不一致（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：账户管理仅唯一superadmin；自身操作account.self；权限按功能全站，不按文档scope
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.NormalizeAccountName`、`identity.HashPassword`、`identity.MapFunctionGrants`、`identity.WriteAccountIndex`、`identity.privateIdentityDigest`、`identity.readIdentityReceipt`、`identity.readActorAccount`、`identity.insertAccount`、`identity.selectedPermissionNames`、`identity.saveIdentityReceipt`、`identity.commitAccountWrite`、`identity.syncAccountIndex`、`identity.hasGrant`、`identity.PublishIdentityEvent`、`identity.identityStoreFromContext`

## identity.ReadSelf

- 级别：约束
- 文件：`modules/identity/internal/account_service.go`
- 作用：读取当前账户

```go
func ReadSelf(ctx context.Context, op OperationContext, input ReadSelfInput) (ReadSelfResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadSelfInput` | 读取当前账户的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadSelfResult` | 读取当前账户的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 幂等HMAC/keyId、唯一账户、CAS或不可逆bootstrap决定冲突；敏感成功响应不可重放；INTEGRITY_ERROR: 账户/控制决定/回执/索引或初始化归属完整性不一致；DEPENDENCY_UNAVAILABLE: 私有存储、平台签名摘要、初始化或登录风控不可用；FORBIDDEN: 可信服务/网络作用域或本域当前功能授权不满足 |

**步骤**

1. 主体固定取op.actor，并在一致快照核当前账户、会话、凭据/权限/session/auth/data水位及访问/绝对期限
2. 受限初始口令只允许静态identity:readSelf必要资料；停用或删除立即UNAUTHENTICATED
3. 返回公开账户metadata和当前credentialRevision供改密CAS，不返回hash、refresh/CSRF摘要、token或IP

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `CONFLICT`：幂等HMAC/keyId、唯一账户、CAS或不可逆bootstrap决定冲突；敏感成功响应不可重放（不可重试）
- `INTEGRITY_ERROR`：账户/控制决定/回执/索引或初始化归属完整性不一致（不可重试）
- `DEPENDENCY_UNAVAILABLE`：私有存储、平台签名摘要、初始化或登录风控不可用（可重试）
- `FORBIDDEN`：可信服务/网络作用域或本域当前功能授权不满足（不可重试）

- 副作用：无
- 授权：account.self；强改会话仅允许必要资料
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`identity.readActorAccount`、`identity.identityStoreFromContext`

## identity.ListAccounts

- 级别：约束
- 文件：`modules/identity/internal/account_service.go`
- 作用：超管读取子账号列表

```go
func ListAccounts(ctx context.Context, op OperationContext, input ListAccountsInput) (ListAccountsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ListAccountsInput` | 超管读取子账号列表的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListAccountsResult` | 超管读取子账号列表的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 私有回执、账户权威或派生索引完整性不一致；UNAUTHENTICATED: 当前超管会话撤销、到期或水位失效；DEPENDENCY_UNAVAILABLE: 私有账户存储或平台游标签名器不可用 |

**步骤**

1. 同一只读事务使用当前session/账户权威核superadmin，唯一超管判断取账户row，不接受浏览器角色或客户端accountId作用域
2. 只查op.actor.siteId的非超管账号，active/disabled/deleted墓碑都保留；按不可变accountName/accountId升序键集分页，默认20、最多100
3. 平台SignCursor/VerifyCursor签名scope绑定identity.accounts.v1、site/account/dataEpoch与subaccounts筛选；载荷只含最后排序位置，不能跨站点/主体/恢复代次重用
4. 多读一条判断hasMore，再在同一快照读取该页完整安全metadata与既有功能；不返回凭据、token、摘要或IP

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `INTEGRITY_ERROR`：私有回执、账户权威或派生索引完整性不一致（不可重试）
- `UNAUTHENTICATED`：当前超管会话撤销、到期或水位失效（不可重试）
- `DEPENDENCY_UNAVAILABLE`：私有账户存储或平台游标签名器不可用（可重试）

- 副作用：无
- 授权：superadmin
- 并发：单个只读SQLite快照；名字/ID不可变且墓碑不删除，分页不用offset
- 幂等：签名cursor一致水位
- 依赖：`identity.requireAccountManagement`、`identity.readAccount`、`platform.SignCursor`、`platform.VerifyCursor`、`identity.identityStoreFromContext`

## identity.SetAccountStatus

- 级别：约束
- 文件：`modules/identity/internal/account_service.go`
- 作用：停用/恢复/删除子账号

```go
func SetAccountStatus(ctx context.Context, op OperationContext, input SetAccountStatusInput) (SetAccountStatusResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `SetAccountStatusInput` | 停用/恢复/删除子账号的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SetAccountStatusResult` | 停用/恢复/删除子账号的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；NOT_FOUND: 同一站点不存在该目标账户，不泄漏其他站点账户；INTEGRITY_ERROR: 私有回执、账户权威或派生索引完整性不一致 |

**步骤**

1. 账号管理互斥和单一active事务内核当前唯一超管row与会话授权；先读同op/caller/actor/epoch/HMAC/keyId完整原成功回执，重放不重复撤销后来会话
2. 限定同site目标，已删除墓碑禁止再管理；AssertSuperadminImmutable保持唯一超管仅active，拒停用/删除；核expectedRevision，不匹配VERSION_MISMATCH
3. 相同状态且CAS水位匹配仅保存安全成功回执，无版本提升/事件/会话变化；唯一超管active原样返回
4. 实际状态迁移按既有active↔disabled→deleted规则，账户revision和permissionRevision各增1；同事务RevokeAllSessions撤销全部session/refresh族并登记status事件，重新启用不复活旧登录
5. 完整安全回执与撤销/outbox同事务；候选索引先以共用16MiB编码检查，再控制决定/active提交/派生索引补齐，不物理删除墓碑姓名

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `NOT_FOUND`：同一站点不存在该目标账户，不泄漏其他站点账户（不可重试）
- `INTEGRITY_ERROR`：私有回执、账户权威或派生索引完整性不一致（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin
- 并发：accountMu覆盖权威写事务到索引补齐；账户/凭据CAS、业务变更、会话/事件和原结果回执同事务
- 幂等：site/actor/operation主键；私有HMAC绑定动作/caller/dataEpoch/规范输入和keyId；同键同摘要回原成功，不重做后来状态；异输入或缺原key版本Conflict
- 依赖：`identity.privateIdentityDigest`、`identity.beginAccountManagement`、`identity.replayAccountManagement`、`identity.readManagedAccount`、`identity.commitAccountManagement`、`identity.AssertSuperadminImmutable`、`identity.RevokeAllSessions`、`identity.identityStoreFromContext`

## identity.SetAccountPermissions

- 级别：约束
- 文件：`modules/identity/internal/account_service.go`
- 作用：修改10项功能权限

```go
func SetAccountPermissions(ctx context.Context, op OperationContext, input SetAccountPermissionsInput) (SetAccountPermissionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `SetAccountPermissionsInput` | 修改10项功能权限的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SetAccountPermissionsResult` | 修改10项功能权限的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；NOT_FOUND: 同一站点不存在该目标账户，不泄漏其他站点账户；INTEGRITY_ERROR: 私有回执、账户权威或派生索引完整性不一致 |

**步骤**

1. 仅当前唯一超管row可管理；同site目标须未删除且非超管，禁止修改accountName/isSuperadmin，功能只既有10项、无文档scope或继承
2. MapFunctionGrants验证既有选择，排序后HMAC规范请求；先重放同op完整原回执，再核expectedRevision；旧op不能覆盖后续权限
3. 选择集合未变化且CAS水位匹配只保存成功回执，不增水位、不发事件；变化时CAS替换功能、账户revision与permissionRevision各增1
4. 同事务写identity.authorization.changed permissions事件与最新credentialRevision及安全回执；旧access/委托由权威permissionRevision立即拒绝，不依赖事件窗口
5. 不撤销refresh族；refresh在既定凭据/认证/数据代次与绝对期限仍有效时同事务升级最新权限并轮换；索引候选16MiB检查与决定恢复复用既有实现

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `NOT_FOUND`：同一站点不存在该目标账户，不泄漏其他站点账户（不可重试）
- `INTEGRITY_ERROR`：私有回执、账户权威或派生索引完整性不一致（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin
- 并发：accountMu覆盖权威写事务到索引补齐；账户/凭据CAS、业务变更、会话/事件和原结果回执同事务
- 幂等：site/actor/operation主键；私有HMAC绑定动作/caller/dataEpoch/规范输入和keyId；同键同摘要回原成功，不重做后来状态；异输入或缺原key版本Conflict
- 依赖：`identity.privateIdentityDigest`、`identity.beginAccountManagement`、`identity.replayAccountManagement`、`identity.readManagedAccount`、`identity.commitAccountManagement`、`identity.MapFunctionGrants`、`identity.selectedPermissionNames`、`identity.PublishIdentityEvent`、`identity.identityStoreFromContext`

## identity.ResetAccountPassword

- 级别：约束
- 文件：`modules/identity/internal/account_service.go`
- 作用：超管重置子账号初始口令

```go
func ResetAccountPassword(ctx context.Context, op OperationContext, input ResetAccountPasswordInput) (ResetAccountPasswordResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ResetAccountPasswordInput` | 超管重置子账号初始口令的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResetAccountPasswordResult` | 超管重置子账号初始口令的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；NOT_FOUND: 同一站点不存在该目标账户，不泄漏其他站点账户；INTEGRITY_ERROR: 私有回执、账户权威或派生索引完整性不一致 |

**步骤**

1. 只允许当前唯一超管row管理同site未删除子账号；唯一超管自身不可走此入口，忘密仍走服务器恢复；先读安全原成功回执，禁止旧op在后续改密后重新重置
2. 私有输入摘要只用platform PrivateOperationDigest HMAC，记录keyId，原始initialPassword/token不入回执/日志；对原成功请求不重算Argon2或重复写凭据
3. 只读预核当前权限和目标expectedRevision后在短写事务外HashPassword；写事务再次核当前会话、原回执与账户CAS，防计算期间并发状态/权限/密码变化
4. CAS提升credentialRevision、账户revision，设mustChangePassword=true，保持原active/disabled状态；同事务RevokeAllSessions撤session/refresh族并发password事件，下次新登录只能改密
5. 返回与持久回执只含安全Account原结果；候选索引在不可反悔提交前16MiB检查，控制决定与派生索引恢复复用既有实现

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `NOT_FOUND`：同一站点不存在该目标账户，不泄漏其他站点账户（不可重试）
- `INTEGRITY_ERROR`：私有回执、账户权威或派生索引完整性不一致（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin
- 并发：accountMu覆盖权威写事务到索引补齐；账户/凭据CAS、业务变更、会话/事件和原结果回执同事务
- 幂等：site/actor/operation主键；私有HMAC绑定动作/caller/dataEpoch/规范输入和keyId；同键同摘要回原成功，不重做后来状态；异输入或缺原key版本Conflict
- 依赖：`identity.privateIdentityDigest`、`identity.beginAccountManagement`、`identity.replayAccountManagement`、`identity.readManagedAccount`、`identity.commitAccountManagement`、`identity.requireAccountManagement`、`identity.readIdentityReceipt`、`identity.syncAccountIndex`、`identity.HashPassword`、`identity.RevokeAllSessions`、`identity.identityStoreFromContext`

## identity.readAccount

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：读取账户安全元数据及既有功能选择

```go
func readAccount(ctx context.Context, query sessionQueryer, accountID, siteID string) (Account, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取账户安全元数据及既有功能选择

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.identityDBError`

## identity.readActorAccount

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：在本域事务快照核当前会话和静态动作权限

```go
func readActorAccount(ctx context.Context, op OperationContext, query sessionQueryer, permission, action string) (sessionSnapshot, Account, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在本域事务快照核当前会话和静态动作权限

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`identity.sessionAuthContext`、`identity.readAccount`、`identity.hasGrant`

## identity.insertAccount

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：在外层事务写账户凭据与既有功能

```go
func insertAccount(ctx context.Context, tx *sql.Tx, account Account, encodedHash string) error
```

**输入**

无

**输出**

无

**步骤**

1. 在外层事务写账户凭据与既有功能

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.identityDBError`

## identity.selectedPermissionNames

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：省略权限取既有10项默认全选，不增用户开关

```go
func selectedPermissionNames(input PermissionSelection) []string
```

**输入**

无

**输出**

无

**步骤**

1. 省略权限取既有10项默认全选，不增用户开关

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.privateIdentityDigest

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：动作域分离的私有HMAC摘要与keyId，不持久明文口令

```go
func privateIdentityDigest(ctx context.Context, op OperationContext, action string, input any) (platform.Hash, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 动作域分离的私有HMAC摘要与keyId，不持久明文口令

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.sessionRuntimeFromContext`、`platform.PrivateOperationDigest`

## identity.readIdentityReceipt

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：按操作键读取同HMAC和keyId的安全回执，校核结果哈希

```go
func readIdentityReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, digest, keyID string, result any) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按操作键读取同HMAC和keyId的安全回执，校核结果哈希

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.identityDBError`

## identity.saveIdentityReceipt

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：只保存安全完整metadata或引用回执，不接受调用方原始token

```go
func saveIdentityReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, digest, keyID string, safeResult any) error
```

**输入**

无

**输出**

无

**步骤**

1. 只保存安全完整metadata或引用回执，不接受调用方原始token

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.identityDBError`

## identity.commitAccountWrite

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：控制日志先记索引决定，active提交后补齐索引再完结

```go
func commitAccountWrite(ctx context.Context, op OperationContext, store *Store, tx *sql.Tx) error
```

**输入**

无

**输出**

无

**步骤**

1. 不可反悔active提交前，在相同事务读取候选权威并以共用编码器核对16MiB上限，超限回滚，不写出下一次不可读索引
2. 控制日志先记索引决定，active提交后补齐索引再完结

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.AppendDecision`、`identity.CompleteDecision`、`identity.syncAccountIndex`、`identity.readAccountAuthorityQuery`、`identity.encodeAccountIndex`

## identity.syncAccountIndex

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：从已提交active权威重建派生索引，受本域账号互斥保护

```go
func syncAccountIndex(ctx context.Context, op OperationContext, store *Store) error
```

**输入**

无

**输出**

无

**步骤**

1. 从已提交active权威重建派生索引，受本域账号互斥保护

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.WriteAccountIndex`

## identity.recoverAccountIndex

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：启动时从active补齐未决索引，未知或领先控制索引失败关闭

```go
func recoverAccountIndex(store *Store, entry journalEntry) error
```

**输入**

无

**输出**

无

**步骤**

1. 启动时从active补齐未决索引，未知或领先控制索引失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.syncAccountIndex`、`identity.CompleteDecision`、`identity.identityDBError`

## identity.identityDBError

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：输出不含SQL原值或私有路径的安全存储错误

```go
func identityDBError() error
```

**输入**

无

**输出**

无

**步骤**

1. 输出不含SQL原值或私有路径的安全存储错误

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.hasGrant

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：判断当前派生权限集合是否包含固定业务能力

```go
func hasGrant(grants []string, wanted string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 判断当前派生权限集合是否包含固定业务能力

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.beginAccountManagement

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：开启管理写事务并核当前权威超管，失败回滚和释放账号互斥

```go
func beginAccountManagement(ctx context.Context, op OperationContext, store *Store, digest, keyID, action string) (*accountManagementOperation, error)
```

**输入**

无

**输出**

无

**步骤**

1. 开启管理写事务并核当前权威超管，失败回滚和释放账号互斥

**错误**

- 无

- 副作用：无
- 授权：本域账号管理私有助手；当前超管权威由requireAccountManagement建立
- 并发：同一accountMu和外层SQLite事务，不嵌套事务
- 幂等：按原业务完整回执；只读/释放助手无业务副作用
- 依赖：`identity.requireAccountManagement`、`identity.closeAccountManagement`、`identity.identityDBError`

## identity.closeAccountManagement

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：释放外层管理事务与账号互斥

```go
func (work *accountManagementOperation) closeAccountManagement()
```

**输入**

无

**输出**

无

**步骤**

1. 释放外层管理事务与账号互斥

**错误**

- 无

- 副作用：无
- 授权：本域账号管理私有助手；当前超管权威由requireAccountManagement建立
- 并发：同一accountMu和外层SQLite事务，不嵌套事务
- 幂等：按原业务完整回执；只读/释放助手无业务副作用
- 依赖：无

## identity.requireAccountManagement

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：核当前账户session/grants与账户row.isSuperadmin，不从浏览器或权限名称猜身份

```go
func requireAccountManagement(ctx context.Context, op OperationContext, query sessionQueryer, action string) error
```

**输入**

无

**输出**

无

**步骤**

1. 核当前账户session/grants与账户row.isSuperadmin，不从浏览器或权限名称猜身份

**错误**

- 无

- 副作用：无
- 授权：本域账号管理私有助手；当前超管权威由requireAccountManagement建立
- 并发：同一accountMu和外层SQLite事务，不嵌套事务
- 幂等：按原业务完整回执；只读/释放助手无业务副作用
- 依赖：`identity.readActorAccount`

## identity.replayAccountManagement

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：只返回完整安全原成功回执；索引取当前权威，不重放旧状态/撤销

```go
func (work *accountManagementOperation) replayAccountManagement(ctx context.Context, op OperationContext, result any) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只返回完整安全原成功回执；索引取当前权威，不重放旧状态/撤销

**错误**

- 无

- 副作用：无
- 授权：本域账号管理私有助手；当前超管权威由requireAccountManagement建立
- 并发：同一accountMu和外层SQLite事务，不嵌套事务
- 幂等：按原业务完整回执；只读/释放助手无业务副作用
- 依赖：`identity.readIdentityReceipt`、`identity.syncAccountIndex`

## identity.commitAccountManagement

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：回执与实际变更一起提交；真正账号变更才经既有索引预检/决定恢复

```go
func (work *accountManagementOperation) commitAccountManagement(ctx context.Context, op OperationContext, result any, accountChanged bool) error
```

**输入**

无

**输出**

无

**步骤**

1. 回执与实际变更一起提交；真正账号变更才经既有索引预检/决定恢复

**错误**

- 无

- 副作用：无
- 授权：本域账号管理私有助手；当前超管权威由requireAccountManagement建立
- 并发：同一accountMu和外层SQLite事务，不嵌套事务
- 幂等：按原业务完整回执；只读/释放助手无业务副作用
- 依赖：`identity.saveIdentityReceipt`、`identity.commitAccountWrite`

## identity.readManagedAccount

- 级别：参考
- 文件：`modules/identity/internal/account_service.go`
- 作用：仅查当前站点，已删除墓碑拒绝再管理

```go
func readManagedAccount(ctx context.Context, op OperationContext, query sessionQueryer, accountID string) (Account, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅查当前站点，已删除墓碑拒绝再管理

**错误**

- 无

- 副作用：无
- 授权：本域账号管理私有助手；当前超管权威由requireAccountManagement建立
- 并发：同一accountMu和外层SQLite事务，不嵌套事务
- 幂等：按原业务完整回执；只读/释放助手无业务副作用
- 依赖：`identity.readAccount`

## identity.NormalizeAccountName

- 级别：参考
- 文件：`modules/identity/internal/account_validator.go`
- 作用：规范不可变账号名

```go
func NormalizeAccountName(input NormalizeAccountNameInput) (NormalizeAccountNameResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `NormalizeAccountNameInput` | 规范不可变账号名的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizeAccountNameResult` | 规范不可变账号名的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. NFC并拒绝控制字符与首尾空白
2. 检查字符/UTF-8长度
3. 作为站点唯一key，不按email或casefold语义改写

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## identity.AssertSuperadminImmutable

- 级别：参考
- 文件：`modules/identity/internal/account_validator.go`
- 作用：保护唯一超管永久有效

```go
func AssertSuperadminImmutable(input AssertSuperadminImmutableInput) (AssertSuperadminImmutableResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `AssertSuperadminImmutableInput` | 保护唯一超管永久有效的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AssertSuperadminImmutableResult` | 保护唯一超管永久有效的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配 |

**步骤**

1. isSuperadmin=true只允许active且不可删除/改身份
2. 子账号可以按状态机迁移
3. 不允许通过permissions字段模拟superadmin

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## identity.MapFunctionGrants

- 级别：参考
- 文件：`modules/identity/internal/permission_mapping.go`
- 作用：将已确认功能勾选映射成服务权限

```go
func MapFunctionGrants(input MapFunctionGrantsInput) (MapFunctionGrantsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `MapFunctionGrantsInput` | 将已确认功能勾选映射成服务权限的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MapFunctionGrantsResult` | 将已确认功能勾选映射成服务权限的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 严格读取根contracts/permission-groups.json固定10组映射：默认10项全选、authenticatedBaseGrants固定portal.use/account.self/site.read
2. manage_translations→localization.manage，与manage_languages→localization.languages分开；edit包含preview/upload/read，publish包含publish/history/trash；子账号不得获得superadminOnly任何grant
3. job.manage不增加用户开关：从根jobManagementGrants生成目录取能力集合，当前派生grants含任一项时追加job.manage，仅供portal入口粗预检；jobs必须按持久owner/type重新核具体业务权限，不允许任意任务或系统任务越权；唯一superadmin按账户标识获取登记管理权限

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无
- 其他依赖说明：根contracts/permission-groups.json的jobManagementGrants，经make gen生成gen.JobManagementGrants；本域不维护第二份任务权限目录

## identity.HashPassword

- 级别：参考
- 文件：`modules/identity/internal/credential_service.go`
- 作用：生成Argon2id口令哈希

```go
func HashPassword(ctx context.Context, op OperationContext, input HashPasswordInput) (HashPasswordResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `HashPasswordInput` | 生成Argon2id口令哈希的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HashPasswordResult` | 生成Argon2id口令哈希的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter |

**步骤**

1. 至少8位且不强制字符组合，不trim
2. CSPRNG16字节salt，Argon2idv19 memory65536KiB/time3/parallel4/hash32字节
3. 编码成本与hash，敏感缓冲短期清理，随机源失败不降级

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `RATE_LIMITED`：频率、并发或容量上限已达，返回 retryAfter（不可重试）

- 副作用：随机源/敏感内存
- 授权：私有credential流程
- 并发：密码计算信号量限并发
- 幂等：每次新salt；外层operation receipt禁止重复业务哈希
- 依赖：无

## identity.VerifyPassword

- 级别：参考
- 文件：`modules/identity/internal/credential_service.go`
- 作用：核对口令

```go
func VerifyPassword(input VerifyPasswordInput) (VerifyPasswordResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `VerifyPasswordInput` | 核对口令的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifyPasswordResult` | 核对口令的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 检查算法编码成本不超部署上限
2. 派生并常量时间比对；不存在账号走dummy hash相同工作量
3. 返回rehash标志，禁记原值

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：私有登录流程
- 并发：密码信号量
- 幂等：确定性
- 依赖：无

## identity.ValidatePasswordChange

- 级别：参考
- 文件：`modules/identity/internal/credential_service.go`
- 作用：验证改密三输入规则

```go
func ValidatePasswordChange(input ValidatePasswordChangeInput) (ValidatePasswordChangeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidatePasswordChangeInput` | 验证改密三输入规则的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePasswordChangeResult` | 验证改密三输入规则的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 新口令至少8位且两次完全相同
2. 常量时间拒绝新旧相同
3. 不新增组合、邮件或MFA前置条件

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## identity.ChangePassword

- 级别：约束
- 文件：`modules/identity/internal/credential_service.go`
- 作用：改口令并结束其他设备登录

```go
func ChangePassword(ctx context.Context, op OperationContext, input ChangePasswordInput) (ChangePasswordResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ChangePasswordInput` | 改口令并结束其他设备登录的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ChangePasswordResult` | 改口令并结束其他设备登录的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 账户/控制决定/回执/索引或初始化归属完整性不一致 |

**步骤**

1. 当前可信会话允许account.self的固定changePassword动作，初始口令也可；验证旧口令、两次新口令相同且不同于旧值，expectedCredentialRevision必须等于当前权威
2. 新Argon2哈希在短写事务外计算，写事务重新核对会话与凭据版本后CAS更新credentialRevision、清mustChangePassword并提升账户revision
3. 同一事务RevokeOtherSessions撤销其他设备，保留设备同步新credential/permission/restricted水位并提升session revision，IssueSessionTokens消费旧refresh且轮换独立秘密
4. 身份事件、私有HMAC/keyId及仅含账户/会话/凭据引用的安全回执同事务；账号索引由决定日志补齐，raw secret从不入普通回执
5. 仅首次成功返回当前设备新token；重复相同操作明确CONFLICT要求重新登录，不重发旧秘密或重新改密；切换key且无原key拒绝重新执行

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：账户/控制决定/回执/索引或初始化归属完整性不一致（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：account.self；mustChangePassword会话也可调用
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：相同operation/输入只存HMAC摘要与安全account/session/credential引用；仅首次事务返回敏感token，重复返回CONFLICT要求重新登录，不重复建会话；keyId不匹配失败关闭
- 依赖：`identity.VerifyPassword`、`identity.ValidatePasswordChange`、`identity.HashPassword`、`identity.RevokeOtherSessions`、`identity.IssueSessionTokens`、`identity.privateIdentityDigest`、`identity.readIdentityReceipt`、`identity.readActorAccount`、`identity.readSessionSnapshot`、`identity.sessionAuthContext`、`identity.saveIdentityReceipt`、`identity.commitAccountWrite`、`identity.PublishIdentityEvent`、`identity.sensitiveReplayError`、`identity.identityStoreFromContext`

## identity.sensitiveReplayError

- 级别：参考
- 文件：`modules/identity/internal/credential_service.go`
- 作用：敏感响应已提交但不重放，要求重新登录

```go
func sensitiveReplayError() error
```

**输入**

无

**输出**

无

**步骤**

1. 敏感响应已提交但不重放，要求重新登录

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.CreateSession

- 级别：约束
- 文件：`modules/identity/internal/session_service.go`
- 作用：账号名口令登录

```go
func CreateSession(ctx context.Context, op OperationContext, input CreateSessionInput) (CreateSessionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CreateSessionInput` | 账号名口令登录的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateSessionResult` | 账号名口令登录的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；CONFLICT: 幂等HMAC/keyId、唯一账户、CAS或不可逆bootstrap决定冲突；敏感成功响应不可重放；INTEGRITY_ERROR: 账户/控制决定/回执/索引或初始化归属完整性不一致；FORBIDDEN: 可信服务/网络作用域或本域当前功能授权不满足 |

**步骤**

1. 匿名凭证动作只接受已认证portal，回执/claim主体固定service portal/system，清可选用户proof；cookie或委托变化不改变同op幂等作用域
2. 仅接受portal已签名建立的1..2个LoginNetworkScopes；规范账号名后以platform.PrivateOperationDigest派生账号HMAC，调用traffic-guard.beginLoginAttempt，未知或拒绝不继续
3. 按站点/主体/operation读取带digest_key_id的私有HMAC回执；已提交仅提示重新登录，不重发raw cookie或再建session
4. 读取账号凭据，VerifyPassword；账号不存在、密码错误或非active统一UNAUTHENTICATED并finish rejected；损坏依赖或中断报aborted，不记录口令/原IP
5. 必要时按当前Argon2参数rehash且不提升凭据语义版本；读取site-config安全初始化权威，未知阶段或归属不一致失败关闭
6. guard finish authenticated成功后，在短写事务重新核active账户与credentialRevision，按本域当前权限插入会话并IssueSessionTokens；摘要与安全引用回执同事务
7. 首次提交才返回raw tokens，nextRoute只选/admin/me（强改专用）、/admin/setup或/admin；重复同op无新秘密，key版本变更不能按新操作执行

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `RATE_LIMITED`：频率、并发或容量上限已达，返回 retryAfter（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `CONFLICT`：幂等HMAC/keyId、唯一账户、CAS或不可逆bootstrap决定冲突；敏感成功响应不可重放（不可重试）
- `INTEGRITY_ERROR`：账户/控制决定/回执/索引或初始化归属完整性不一致（不可重试）
- `FORBIDDEN`：可信服务/网络作用域或本域当前功能授权不满足（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：anonymous + 同Origin；只显示账号名和口令，不新增MFA/邮箱
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：相同operation/输入只存HMAC摘要与安全account/session/credential引用；仅首次事务返回敏感token，重复返回CONFLICT要求重新登录，不重复建会话；keyId不匹配失败关闭
- 依赖：`identity.VerifyPassword`、`identity.IssueSessionTokens`、`site-config:readInitializationStatus`、`identity.NormalizeAccountName`、`identity.HashPassword`、`identity.privateIdentityDigest`、`identity.readIdentityReceipt`、`identity.beginIdentityLogin`、`identity.finishIdentityLogin`、`identity.readAccount`、`identity.insertSession`、`identity.saveIdentityReceipt`、`identity.sensitiveReplayError`、`identity.registrationRuntimeFromContext`、`identity.anonymousIdentityOperation`、`identity.identityStoreFromContext`

## identity.IssueSessionTokens

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：签发会话和CSRFcookie数据

```go
func IssueSessionTokens(ctx context.Context, op OperationContext, input IssueSessionTokensInput) (IssueSessionTokensResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `IssueSessionTokensInput` | 签发会话和CSRFcookie数据的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `IssueSessionTokensResult` | 签发会话和CSRFcookie数据的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 本域会话权威、期限、权限登记或完整结果回执损坏；CONFLICT: 会话或族CAS版本变化，或幂等键对应不同请求摘要；UNAUTHENTICATED: 会话或族不存在、撤销、到期或授权水位失效；FORBIDDEN: 调用方或本域私有事务不满足该函数边界 |

**步骤**

1. 仅接受WithIdentityTransaction传入的本域事务；会话与最新授权快照由外层用例建立
2. 读取或首次建立auth_state单行认证代次；生成独立access nonce、refresh与CSRFsecret，签名和随机数只调用libs/platform
3. 固定iss=identity、aud=portal、Ed25519与专用token类型，绑定session/account/site/credentialRevision/permissionRevision/authEpoch/dataEpoch/sessionRevision；访问期限15分钟且不超过绝对7天
4. 在同一事务建立一对一refresh族或消费旧refresh摘要并CAS轮换，保存access/refresh/CSRF摘要；不保存敏感响应明文

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：本域会话权威、期限、权限登记或完整结果回执损坏（不可重试）
- `CONFLICT`：会话或族CAS版本变化，或幂等键对应不同请求摘要（不可重试）
- `UNAUTHENTICATED`：会话或族不存在、撤销、到期或授权水位失效（不可重试）
- `FORBIDDEN`：调用方或本域私有事务不满足该函数边界（不可重试）

- 副作用：平台CSPRNG与签名；外部私有事务内追加摘要与轮换水位
- 授权：identity私有sessiontxn
- 并发：固定key版本
- 幂等：外层业务事务单次签发；不持久化明文敏感响应，重复旧refresh撤销族而不回放secret
- 依赖：`identity.sessionRuntimeFromContext`、`platform.GenerateSessionSecrets`、`platform.SignSessionAccess`
- 其他依赖说明：auth_state认证代次权威；session_families与consumed_refresh_tokens私有摘要表；libs/platform/serviceauth.GenerateSessionSecrets与SignSessionAccess：随机数和签名机制唯一来源

## identity.RefreshSession

- 级别：约束
- 文件：`modules/identity/internal/session_service.go`
- 作用：内部透明轮换session

```go
func RefreshSession(ctx context.Context, op OperationContext, input RefreshSessionInput) (RefreshSessionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RefreshSessionInput` | 内部透明轮换session的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RefreshSessionResult` | 内部透明轮换session的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 本域会话权威、期限、权限登记或完整结果回执损坏；CONFLICT: 会话或族CAS版本变化，或幂等键对应不同请求摘要；FORBIDDEN: 调用方或本域私有事务不满足该函数边界 |

**步骤**

1. 只接受portal服务转发的refresh cookie；同Origin与SameSite=Strict由入口建立，打开本域串行写事务
2. 按refresh SHA256摘要查当前族；若命中已消费摘要，则在同事务撤销该族和session、登记安全outbox，提交后返回UNAUTHENTICATED
3. 当前族须未撤销且authEpoch/dataEpoch与当前权威一致、session未撤销且未过绝对期限、账户active、credentialRevision未失效；旧permissionRevision仅用于刷新，不使已失效access恢复可用
4. 同一事务从本域权限表建立最新AuthContext，保留mustChangePassword限制，CAS将session.permissionRevision更新到当前权威并将session revision加一，再调用IssueSessionTokens；提交后仅向cookie adapter返回本次敏感值

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：本域会话权威、期限、权限登记或完整结果回执损坏（不可重试）
- `CONFLICT`：会话或族CAS版本变化，或幂等键对应不同请求摘要（不可重试）
- `FORBIDDEN`：调用方或本域私有事务不满足该函数边界（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：service:portal；refresh cookie + SameSite=Strict + Origin，入口只建立来源与服务身份，不要求有效access
- 并发：家族行互斥
- 幂等：refresh单次消费；并发或重放旧值撤销同族，敏感响应丢失需重新登录，浏览器singleflight
- 依赖：`identity.IssueSessionTokens`、`identity.RevokeSessionFamily`、`identity.MapFunctionGrants`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`identity.sessionAuthContext`、`identity.identityStoreFromContext`

## identity.ReadSessions

- 级别：约束
- 文件：`modules/identity/internal/session_service.go`
- 作用：读取自身登录设备

```go
func ReadSessions(ctx context.Context, op OperationContext, input ReadSessionsInput) (ReadSessionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadSessionsInput` | 读取自身登录设备的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadSessionsResult` | 读取自身登录设备的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 会话数据库或签名密钥不可用；INTEGRITY_ERROR: 会话记录或分页位置损坏；FORBIDDEN: 初始口令受限会话禁止读取会话列表 |

**步骤**

1. 主体必须是持有account.self的admin；查询条件固定为op.actor.subjectId与siteId，不接受其他账户参数
2. 自身当前委托的op.actor.sessionRevision必须为正数并等于session权威revision，缺失或陈旧失败关闭
3. 在一致只读事务读取当前session/账户/族/认证代次，确认主体站点、未撤销、访问与绝对期限及最新凭据权限水位；初始口令受限会话拒绝会话列表
4. 按issuedAt/sessionId升序键集分页，默认20条、最多100条；平台签名scope由identity.sessions、siteId、accountId、dataEpoch以Identifier不含的竖线分隔构成，载荷只含排序位置；验证相同scope后才解码
5. 显式选择会话元数据列，不返回access/refresh/CSRF secret、摘要或IP；多读一条判定hasMore，无下一页省略nextCursor

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `DEPENDENCY_UNAVAILABLE`：会话数据库或签名密钥不可用（可重试）
- `INTEGRITY_ERROR`：会话记录或分页位置损坏（不可重试）
- `FORBIDDEN`：初始口令受限会话禁止读取会话列表（不可重试）

- 副作用：无
- 授权：account.self
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`identity.WithSessionCursorKey`、`platform.SignCursor`、`platform.VerifyCursor`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`identity.identityStoreFromContext`
- 其他依赖说明：libs/platform/serviceauth.SignCursor与VerifyCursor实现签名游标；未装配密钥时只要请求或生成游标即DEPENDENCY_UNAVAILABLE

## identity.WithSessionCursorKey

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：向自身会话分页查询注入受信签名密钥引用

```go
func WithSessionCursorKey(ctx context.Context, key platform.SigningKey) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 仅由服务装配将本服务SigningKey放入进程私有上下文；ReadSessions调用平台签名机制，不接受浏览器传入密钥引用

**错误**

- 无

- 副作用：无
- 授权：身份服务私有装配
- 并发：上下文不可变派生
- 幂等：相同输入构造等价上下文
- 依赖：无
- 其他依赖说明：libs/platform/serviceauth.SignCursor与VerifyCursor

## identity.WithSessionRuntime

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：注入受信会话签名、公钥表与固定公开Origin

```go
func WithSessionRuntime(ctx context.Context, runtime SessionRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 仅启动装配可提供SessionRuntime，不读取请求中的密钥、信任锚或公开Origin

**错误**

- 无

- 副作用：无
- 授权：本域进程私有函数；不作为HTTP入口
- 并发：信任调用方传入的私有事务和运行配置
- 幂等：只读或不可变上下文派生
- 依赖：无

## identity.sessionRuntimeFromContext

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：读取已注入会话运行配置

```go
func sessionRuntimeFromContext(ctx context.Context) (SessionRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 未装配时DEPENDENCY_UNAVAILABLE，配置来自本域进程上下文

**错误**

- 无

- 副作用：无
- 授权：本域进程私有函数；不作为HTTP入口
- 并发：信任调用方传入的私有事务和运行配置
- 幂等：只读或不可变上下文派生
- 依赖：`identity.WithSessionRuntime`

## identity.WithLoginRuntime

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：注入真实版本化登录风控客户端

```go
func WithLoginRuntime(ctx context.Context, runtime LoginRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 注入真实版本化登录风控客户端

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.beginIdentityLogin

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：仅接受portal签名网络作用域，平台HMAC账号后申请幂等登录准入

```go
func beginIdentityLogin(ctx context.Context, op OperationContext, accountName, action string) (loginAttempt, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅接受portal签名网络作用域，平台HMAC账号后申请幂等登录准入

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.WithLoginRuntime`、`identity.sessionRuntimeFromContext`、`platform.PrivateOperationDigest`、`traffic-guard:beginLoginAttempt`

## identity.finishIdentityLogin

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：向guard提交authenticated/rejected/aborted幂等终局，未知不放行

```go
func finishIdentityLogin(ctx context.Context, op OperationContext, attempt loginAttempt, outcome string) error
```

**输入**

无

**输出**

无

**步骤**

1. 向guard提交authenticated/rejected/aborted幂等终局，未知不放行
2. authenticated/rejected表示已发送不可逆核验事实；发送后响应未知不改报aborted，aborted只用于此前依赖中断

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.WithLoginRuntime`、`traffic-guard:finishLoginAttempt`

## identity.insertSession

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：插入一次设备会话并从当前账户派生初始授权，不生成秘密

```go
func insertSession(ctx context.Context, tx *sql.Tx, account Account, credentialRevision platform.Revision) (Session, platform.AuthContext, error)
```

**输入**

无

**输出**

无

**步骤**

1. 插入一次设备会话并从当前账户派生初始授权，不生成秘密

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.MapFunctionGrants`、`identity.identityDBError`

## identity.anonymousIdentityOperation

- 级别：参考
- 文件：`modules/identity/internal/session_service.go`
- 作用：portal匿名凭证操作固定service主体与私有回执scope，清除可选用户proof

```go
func anonymousIdentityOperation(ctx context.Context, op OperationContext) (context.Context, OperationContext, error)
```

**输入**

无

**输出**

无

**步骤**

1. portal匿名凭证操作固定service主体与私有回执scope，清除可选用户proof

**错误**

- 无

- 副作用：无
- 授权：本域受信私有用例或恢复
- 并发：遵循本域账号互斥与外层事务
- 幂等：纯派生或只读
- 依赖：无

## identity.RevokeSession

- 级别：约束
- 文件：`modules/identity/internal/session_revocation.go`
- 作用：退出当前或指定自身登录

```go
func RevokeSession(ctx context.Context, op OperationContext, input RevokeSessionInput) (RevokeSessionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RevokeSessionInput` | 退出当前或指定自身登录的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RevokeSessionResult` | 退出当前或指定自身登录的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 本域会话权威、期限、权限登记或完整结果回执损坏；CONFLICT: 会话或族CAS版本变化，或幂等键对应不同请求摘要 |

**步骤**

1. 要求admin主体持account.self；查询session归属必须匹配actor site/account
2. 自身当前委托的op.actor.sessionRevision必须为正数并等于session权威revision，缺失或陈旧失败关闭
3. 按action/caller/actor/epoch/input摘要读取完整幂等回执；不同摘要CONFLICT，原回执重放不再写入
4. 回执不存在时在同一事务核验当前session最新权威；初始口令受限主体只可撤销当前设备；调用RevokeSessionFamily，旧迁移目标会话没有族时直接撤销并追加安全事件
5. 回执与撤销outbox原子提交；返回revoked=true供cookie adapter清除当前设备凭据

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：本域会话权威、期限、权限登记或完整结果回执损坏（不可重试）
- `CONFLICT`：会话或族CAS版本变化，或幂等键对应不同请求摘要（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：account.self
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.RevokeSessionFamily`、`identity.PublishIdentityEvent`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`identity.identityStoreFromContext`

## identity.RevokeSessionFamily

- 级别：参考
- 文件：`modules/identity/internal/session_revocation.go`
- 作用：使refresh族全部失效

```go
func RevokeSessionFamily(ctx context.Context, op OperationContext, input RevokeSessionFamilyInput) (RevokeSessionFamilyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RevokeSessionFamilyInput` | 使refresh族全部失效的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RevokeSessionFamilyResult` | 使refresh族全部失效的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 本域会话权威、期限、权限登记或完整结果回执损坏；UNAUTHENTICATED: 会话或族不存在、撤销、到期或授权水位失效；FORBIDDEN: 调用方或本域私有事务不满足该函数边界 |

**步骤**

1. 仅接受本域WithIdentityTransaction句柄；按familyId锁定一对一session与当前账户权威版本
2. 未撤销族写revokedAt并提升family revision；同步撤销session、提升session revision；authEpoch仅由全站恢复更新
3. 首次撤销追加identity.authorization.changed安全事件；已撤销返回0，外层事务负责提交

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：本域会话权威、期限、权限登记或完整结果回执损坏（不可重试）
- `UNAUTHENTICATED`：会话或族不存在、撤销、到期或授权水位失效（不可重试）
- `FORBIDDEN`：调用方或本域私有事务不满足该函数边界（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：仅本域私有事务，不接受HTTP直接调用
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：同族已撤销返回0，不重复提升水位或写outbox
- 依赖：`identity.PublishIdentityEvent`

## identity.RevokeAllSessions

- 级别：参考
- 文件：`modules/identity/internal/session_revocation.go`
- 作用：立即失效账户全部登录

```go
func RevokeAllSessions(ctx context.Context, op OperationContext, input RevokeAllSessionsInput) (RevokeAllSessionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RevokeAllSessionsInput` | 立即失效账户全部登录的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RevokeAllSessionsResult` | 立即失效账户全部登录的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 外部调用必须是superadmin；账户状态迁移可在由WithIdentityTransaction注入的身份私有事务内调用，事务句柄不从网络输入
2. 按accountId与当前siteId读取权威账户/凭据版本；同事务将该账户全部未撤销refresh族和session写入revokedAt并提升各自revision，不改已撤销行
3. 同事务登记identity.authorization.changed；disabled/deleted/status映射status、admin_reset映射password，server_reset/restore保持原值；授权仍须同步核session撤销及版本
4. 自有事务的幂等键绑定site/actor/operation、callerService、动作、dataEpoch与请求摘要；完整计数结果及哈希写入operation_receipts.result_json，重放返回原计数且不撤销后来建立的会话；嵌入事务由外层用例统一回执与提交
5. Count为非负int64，不设置与登录会话模型不一致的人为10000上限；计数不可读仍完整回滚

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `CONFLICT`：同一operationId绑定的调用方、代次或请求摘要不同（不可重试）
- `INTEGRITY_ERROR`：完整结果回执缺失、哈希不一致或撤销计数不可读（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin或身份私有状态迁移
- 并发：SQLite单连接串行化；更新、outbox和回执同一事务；外层事务由其调用方负责提交或回滚
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.PublishIdentityEvent`、`identity.identityStoreFromContext`
- 其他依赖说明：内嵌调用方须在开启事务前确保平台outbox schema已建立；撤销session和一对一refresh族同事务

## identity.RevokeOtherSessions

- 级别：参考
- 文件：`modules/identity/internal/session_revocation.go`
- 作用：改密后只保留当前设备

```go
func RevokeOtherSessions(ctx context.Context, op OperationContext, input RevokeOtherSessionsInput) (RevokeOtherSessionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RevokeOtherSessionsInput` | 改密后只保留当前设备的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RevokeOtherSessionsResult` | 改密后只保留当前设备的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；CONFLICT: 会话或族CAS版本变化，或幂等键对应不同请求摘要；UNAUTHENTICATED: 会话或族不存在、撤销、到期或授权水位失效；FORBIDDEN: 调用方或本域私有事务不满足该函数边界 |

**步骤**

1. 仅接受本域改密事务，核对keepSession归属、未撤销与绝对期限；当前credential可为改密前版本
2. 撤销其他session及其refresh族并提升各自行水位；不提升全站authEpoch
3. 当前session CAS到新credentialRevision与当前permissionRevision并解除或保留mustChangePassword限制，后续IssueSessionTokens消费旧摘要并签发新值
4. 外层改密用例统一保存幂等回执、写password事件并提交

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `CONFLICT`：会话或族CAS版本变化，或幂等键对应不同请求摘要（不可重试）
- `UNAUTHENTICATED`：会话或族不存在、撤销、到期或授权水位失效（不可重试）
- `FORBIDDEN`：调用方或本域私有事务不满足该函数边界（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：仅本域改密事务，keepSession须与actor当前session一致
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：由外层改密用例统一回执，本函数仅在外层事务调用一次
- 依赖：无

## identity.VerifySessionCSRF

- 级别：约束
- 文件：`modules/identity/internal/csrf_service.go`
- 作用：portal内部核对CSRF

```go
func VerifySessionCSRF(ctx context.Context, op OperationContext, input VerifySessionCSRFInput) (VerifySessionCSRFResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `VerifySessionCSRFInput` | portal内部核对CSRF的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifySessionCSRFResult` | portal内部核对CSRF的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 本域会话权威、期限、权限登记或完整结果回执损坏 |

**步骤**

1. 只接受已经验明的portal服务调用方；固定Origin由启动配置注入，与输入Origin精确匹配
2. 平台验签access，并在同一只读事务核对access摘要、session/account/site、session/credential/permission版本、auth/data epoch、未撤销与访问/绝对期限；若输入sessionId存在必须匹配
3. 只允许active账户；常量时间核对独立CSRF secret的SHA256摘要，不接受cookie与header自比；成功返回当前访问期限

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：本域会话权威、期限、权限登记或完整结果回执损坏（不可重试）

- 副作用：无
- 授权：service:portal
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`identity.sessionRuntimeFromContext`、`identity.verifySessionAccess`、`platform.VerifySessionAccess`、`identity.identityStoreFromContext`

## identity.CheckAuthorization

- 级别：约束
- 文件：`modules/identity/internal/authorization_service.go`
- 作用：按最新账号功能权限授权

```go
func CheckAuthorization(ctx context.Context, op OperationContext, input AuthorizationRequest) (AuthorizationDecision, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `AuthorizationRequest` | 按最新账号功能权限授权的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AuthorizationDecision` | 按最新账号功能权限授权的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 本域会话权威、期限、权限登记或完整结果回执损坏 |

**步骤**

1. portal accessToken模式只接受固定平台会话验签；subject/site/session可信委托模式要求与op.actor严格相等，两种来源由严格schema互斥
2. 同一只读事务读取session/账户/族与auth_state，核对access摘要或委托主体、最新凭据/权限/session水位（委托sessionRevision必须为正数且等于当前session revision）、auth/data epoch、撤销和期限；纯服务无账户session不获得账户功能
3. MapFunctionGrants只读取本域功能表派生grants；active和superadmin-only单独检查，expectedPermissionRevision不匹配即拒绝；resourceId仅审计不设文档scope
4. 初始口令AuthContext只持account.self；仅当permission为account.self且可信静态action是identity:changePassword、identity:revokeSession、identity:readSelf时允许，缺失或其他动作拒绝；action不扩充grants
5. 成功返回当前授权主体同一只读快照中的不可变accountName，供content锁显示；拒绝客户端显示名，不额外读取其他账户资料

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：本域会话权威、期限、权限登记或完整结果回执损坏（不可重试）

- 副作用：无
- 授权：service，主体必须来自可信委托
- 并发：权限一致快照，不能缓存长期允许
- 幂等：只读
- 依赖：`identity.MapFunctionGrants`、`identity.sessionRuntimeFromContext`、`identity.verifySessionAccess`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`identity.sessionAuthContext`、`platform.VerifySessionAccess`、`identity.identityStoreFromContext`

## identity.readSessionSnapshot

- 级别：参考
- 文件：`modules/identity/internal/authorization_service.go`
- 作用：读取一致会话权威快照

```go
func readSessionSnapshot(ctx context.Context, query sessionQueryer, sessionID platform.Identifier) (sessionSnapshot, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同一事务连接sessions、accounts、credentials、session_families、auth_state；缺失签发族拒绝，不返回秘密
2. 同一账户JOIN读取不可变account_name；AuthorizationDecision只返回当前已校验主体的安全显示名

**错误**

- 无

- 副作用：无
- 授权：本域进程私有函数；不作为HTTP入口
- 并发：信任调用方传入的私有事务和运行配置
- 幂等：只读或不可变上下文派生
- 依赖：无

## identity.validateSessionSnapshot

- 级别：参考
- 文件：`modules/identity/internal/authorization_service.go`
- 作用：按当前权威同步拒绝撤销或陈旧会话

```go
func validateSessionSnapshot(snapshot sessionSnapshot, dataEpoch platform.Identifier, requireAccess bool) error
```

**输入**

无

**输出**

无

**步骤**

1. 比较撤销、auth/data epoch、凭据水位和初始口令限制，检查绝对期限；access/委托场景另要求当前权限版本与访问期限，refresh允许旧权限版本但必须由外层同事务CAS更新后签发

**错误**

- 无

- 副作用：无
- 授权：本域进程私有函数；不作为HTTP入口
- 并发：信任调用方传入的私有事务和运行配置
- 幂等：只读或不可变上下文派生
- 依赖：无

## identity.sessionAuthContext

- 级别：参考
- 文件：`modules/identity/internal/authorization_service.go`
- 作用：从权威功能表派生单次授权上下文

```go
func sessionAuthContext(ctx context.Context, query sessionQueryer, snapshot sessionSnapshot) (platform.AuthContext, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取当前账号功能表并调用MapFunctionGrants；受限仅account.self，停用或删除无grants；返回真实sessionRevision供portal签发委托，旧缺版本委托失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域进程私有函数；不作为HTTP入口
- 并发：信任调用方传入的私有事务和运行配置
- 幂等：只读或不可变上下文派生
- 依赖：`identity.MapFunctionGrants`

## identity.verifySessionAccess

- 级别：参考
- 文件：`modules/identity/internal/authorization_service.go`
- 作用：联合平台验签和本域摘要水位核对

```go
func verifySessionAccess(ctx context.Context, query sessionQueryer, runtime SessionRuntime, dataEpoch platform.Identifier, token string) (sessionSnapshot, error)
```

**输入**

无

**输出**

无

**步骤**

1. 平台负责固定类型/issuer/audience/Ed25519与期限；本域常量时间核对access摘要并比较claims/current会话、凭据权限版本与认证代次

**错误**

- 无

- 副作用：无
- 授权：本域进程私有函数；不作为HTTP入口
- 并发：信任调用方传入的私有事务和运行配置
- 幂等：只读或不可变上下文派生
- 依赖：`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`platform.VerifySessionAccess`
- 其他依赖说明：libs/platform/serviceauth.VerifySessionAccess：固定协议验签唯一来源

## identity.ReadPreferences

- 级别：约束
- 文件：`modules/identity/internal/preference_service.go`
- 作用：读自身后台偏好

```go
func ReadPreferences(ctx context.Context, op OperationContext, input ReadPreferencesInput) (ReadPreferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadPreferencesInput` | 读自身后台偏好的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadPreferencesResult` | 读自身后台偏好的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；FORBIDDEN: 受限初始口令会话或当前账户没有portal.use |

**步骤**

1. 在同一只读事务以readActorAccount核当前admin session、site/account、全部认证/权限水位及portal.use；受限初始口令会话拒绝偏好读取
2. 仅按已核op.actor.subjectId读取私有preferences；无记录返回system/zh-Hans/dashboard/revision0，不插入默认行
3. 持久偏好按原记录返回；权限变动后不偷偷重写defaultRoute，偏好结果本身不授予业务权限

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `FORBIDDEN`：受限初始口令会话或当前账户没有portal.use（不可重试）

- 副作用：无
- 授权：portal.use
- 并发：单个SQLite只读事务同时核当前本人权限与读取偏好，站点来自当前权威session/account
- 幂等：只读；缺失默认值不产生持久副作用
- 依赖：`identity.identityStoreFromContext`、`identity.readActorAccount`、`identity.readPreferenceRecord`
- 其他依赖说明：contracts/common.schema.json的PortalPreferences、UILocale、AdminRouteKey为唯一偏好schema；根permission-groups目录生成gen.InterfaceLocales与gen.AdminRoutes，身份不导入portal私有逻辑

## identity.UpdatePreferences

- 级别：约束
- 文件：`modules/identity/internal/preference_service.go`
- 作用：保存自身主题/语言/默认后台页

```go
func UpdatePreferences(ctx context.Context, op OperationContext, input UpdatePreferencesInput) (UpdatePreferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdatePreferencesInput` | 保存自身主题/语言/默认后台页的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdatePreferencesResult` | 保存自身主题/语言/默认后台页的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 私有完整安全回执状态、JSON或哈希不一致 |

**步骤**

1. 以私有HMAC绑定caller/dataEpoch/动作与完整请求，单个写事务先核当前本人portal.use和认证/权限水位，不接受其他subject/site键
2. 先按site/actor/operation查原完整安全回执；同HMAC/keyId回放原结果，不再CAS、提升版本或覆盖后来偏好；异摘要或原key版本不可用冲突
3. 新写要求preferences.revision等于expectedRevision；允许locale与defaultRoute key已由入口按公共schema一次验证；目标页所需权限以当前权威账户功能及超管标志派生，不信任浏览器或portal前端授权
4. 无记录仅expectedRevision0可首次写revision1；已有记录须CAS匹配expectedRevision，每个新的成功操作即使值相同也只提升一次偏好revision
5. 同事务写theme/locale/defaultRoute/单调revision与完整安全preferences回执后提交；不改变account/credential/permission/session水位，不写账号索引或授权撤销事件

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：私有完整安全回执状态、JSON或哈希不一致（不可重试）

- 副作用：同一私有SQLite事务写本人preferences与完整安全operation_receipts；无账号索引、认证变更或outbox副作用
- 授权：portal.use
- 并发：单个SQLite写事务：当前权威会话/账户授权、原结果回放、偏好CAS和完整回执一致提交；恢复/维护写屏障由入口建立
- 幂等：site/actor/operation唯一键绑定caller/dataEpoch/动作/HMAC/keyId；同请求返回首次完整安全结果，原成功重放先于当前偏好CAS或目标页新授权检查，不覆盖后来修改
- 依赖：`identity.identityStoreFromContext`、`identity.readActorAccount`、`identity.privateIdentityDigest`、`identity.readIdentityReceipt`、`identity.readPreferenceRecord`、`identity.validatePreferenceSelection`、`identity.saveIdentityReceipt`
- 其他依赖说明：contracts/common.schema.json的PortalPreferences、UILocale、AdminRouteKey为唯一偏好schema；根permission-groups目录生成gen.InterfaceLocales与gen.AdminRoutes，身份不导入portal私有逻辑

## identity.readPreferenceRecord

- 级别：参考
- 文件：`modules/identity/internal/preference_service.go`
- 作用：读取已核本人偏好；无记录返回固定默认且found=false

```go
func readPreferenceRecord(ctx context.Context, query sessionQueryer, accountID string) (PortalPreferences, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取已核本人偏好；无记录返回固定默认且found=false

**错误**

- 无

- 副作用：无
- 授权：仅本域已核当前本人portal.use的私有用例调用
- 并发：与外层用例共用SQLite事务和权威账户快照
- 幂等：只读纯派生；不扩大任何grants
- 依赖：无

## identity.validatePreferenceSelection

- 级别：参考
- 文件：`modules/identity/internal/preference_service.go`
- 作用：从公共后台目录选择默认页所需能力，以当前权威账户派生grants核权

```go
func validatePreferenceSelection(preferences PortalPreferences, account Account) error
```

**输入**

无

**输出**

无

**步骤**

1. UILocale/AdminRouteKey允许集由公共入口严格schema一次验证；本函数信任结构，不重复枚举校验
2. 使用当前权威account.permissions/isSuperadmin派生grants，再按gen.AdminRoutes中defaultRoute的Permissions任一能力判断；未授权返回FORBIDDEN，不扩大grants

**错误**

- 无

- 副作用：无
- 授权：仅本域已核当前本人portal.use的私有用例调用
- 并发：与外层用例共用SQLite事务和权威账户快照
- 幂等：只读纯派生；不扩大任何grants
- 依赖：`identity.MapFunctionGrants`
- 其他依赖说明：contracts/common.schema.json的PortalPreferences、UILocale、AdminRouteKey为唯一偏好schema；根permission-groups目录生成gen.InterfaceLocales与gen.AdminRoutes，身份不导入portal私有逻辑

## identity.ReadServerAccountIndex

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：服务器运维查明文账号索引

```go
func ReadServerAccountIndex(ctx context.Context, op OperationContext, input ReadServerAccountIndexInput) (ReadServerAccountIndexResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadServerAccountIndexInput` | 服务器运维查明文账号索引的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadServerAccountIndexResult` | 服务器运维查明文账号索引的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅本机控制身份读取固定私有control/account-index.json
2. 核对索引revision与权威账号manifest
3. 发现滞后由WriteAccountIndex重建，不输出密码hash

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：本机OS文件权限；API仅superadmin受控内部调用
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`identity.WriteAccountIndex`

## identity.WriteAccountIndex

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：维持服务器查账号能力

```go
func WriteAccountIndex(ctx context.Context, op OperationContext, input WriteAccountIndexInput) (WriteAccountIndexResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `WriteAccountIndexInput` | 维持服务器查账号能力的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteAccountIndexResult` | 维持服务器查账号能力的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 抽取accountName/isSuperadmin/status，不复制credential
2. 固定control路径写临时JSON、fsync、rename并fsync目录
3. 索引不进git；服务签名密钥在control区，不随恢复替换；失败记录重建待办
4. 读写与恢复统一16MiB正规文件上限；编码器在安装前拒绝超限，业务提交前也以同一编码器检查，10000账户数量上限不变

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有可读索引文件
- 授权：私有账号提交/恢复
- 并发：进程内accountIndexWriteMu覆盖现有水位读取、临时文件安装与目录fsync；账号业务互斥覆盖active提交至索引补齐，禁止并发低水位覆盖高水位
- 幂等：同revision同hash不重复写
- 依赖：`identity.encodeAccountIndex`

## identity.ObserveSuperadminResetFile

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：检测服务器超管重置文件

```go
func ObserveSuperadminResetFile(ctx context.Context, op OperationContext, input ObserveSuperadminResetFileInput) (ObserveSuperadminResetFileResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ObserveSuperadminResetFileInput` | 检测服务器超管重置文件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ObserveSuperadminResetFileResult` | 检测服务器超管重置文件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 只访问identity私有control/reset-superadmin.txt固定路径，拒绝symlink/目录/权限异常
2. 有界UTF8读取并按8位Password校验，生成fileHash作operationId
3. 输入仅内存，不读任何部署凭据文件

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：读取固定重置请求文件
- 授权：本机OS控制身份
- 并发：单watcher，不并发消费同文件
- 幂等：同fileHash同candidate
- 依赖：`identity.readSuperadminResetFile`、`identity.ApplySuperadminReset`

## identity.ApplySuperadminReset

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：用服务器文件恢复唯一超管口令

```go
func ApplySuperadminReset(ctx context.Context, op OperationContext, input ApplySuperadminResetInput) (ApplySuperadminResetResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ApplySuperadminResetInput` | 用服务器文件恢复唯一超管口令的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApplySuperadminResetResult` | 用服务器文件恢复唯一超管口令的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 重新核对固定文件hash，定位唯一superadmin，并核对superadmin.siteId等于受信后台scope.siteId
2. HashPassword，事务更新凭据/mustChangePassword=true并撤销全部session
3. 提交回执后原子删除仅该固定file；删除失败下次读同hash回执只补删，不重复重置

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：凭据与mustChangePassword写入、全部会话撤销及server_reset事件，提交回执后删除固定reset文件
- 授权：本机OS控制身份；不需要已失效后台token
- 并发：超管credential行锁+fileHashreceipt
- 幂等：fileHash作为operationId；删除失败重复仅补清理
- 依赖：`identity.readSuperadminResetFile`、`identity.readUniqueSuperadmin`、`identity.HashPassword`、`identity.RevokeAllSessions`、`identity.privateIdentityDigest`、`identity.saveIdentityReceipt`、`identity.consumeSuperadminResetFile`

## identity.readSuperadminResetFile

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：只读固定control/reset-superadmin.txt，不读部署凭据

```go
func readSuperadminResetFile(dataRoot string) (*ServerResetCandidate, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只打开identity私有control/reset-superadmin.txt，拒绝符号链接、目录、硬链接和组/其他权限位
2. 有界读取UTF-8，按Password规则校验且不trim，以文件字节SHA-256作为fileHash和candidateId
3. 文件不存在返回缺失；口令只留在返回的内存候选中

**错误**

- 无

- 副作用：读取固定重置请求文件
- 授权：本域受信私有用例或恢复
- 并发：由超管重置互斥覆盖，不并发消费同文件
- 幂等：同文件字节得到同fileHash候选
- 依赖：无

## identity.readUniqueSuperadmin

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：读取当前唯一且仍为active的超管权威行

```go
func readUniqueSuperadmin(ctx context.Context, query sessionQueryer) (superadminResetAccount, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只从accounts权威表读取is_superadmin行，必须恰好一行且状态为active

**错误**

- 无

- 副作用：无
- 授权：本域受信私有用例或恢复
- 并发：遵循本域账号互斥与外层事务
- 幂等：纯派生或只读
- 依赖：无

## identity.consumeSuperadminResetFile

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：回执提交后只删除哈希仍匹配的固定重置文件

```go
func consumeSuperadminResetFile(dataRoot string, fileHash platform.Hash) error
```

**输入**

无

**输出**

无

**步骤**

1. 重新核对固定文件哈希，只unlink该普通文件
2. 文件已不存在视为已消费；哈希变化或类型异常时不删除

**错误**

- 无

- 副作用：删除固定reset文件
- 授权：本域受信私有用例或恢复
- 并发：由超管重置互斥覆盖，不并发消费同文件
- 幂等：同哈希重复删除只补清理
- 依赖：`identity.readSuperadminResetFile`

## identity.readAccountAuthorityQuery

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：复用同一权威读取实现，允许提交前事务读取候选索引

```go
func readAccountAuthorityQuery(ctx context.Context, query sessionQueryer) (accountAuthority, error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用同一权威读取实现，允许提交前事务读取候选索引

**错误**

- 无

- 副作用：无
- 授权：本域受信私有用例或恢复
- 并发：遵循本域账号互斥与外层事务
- 幂等：纯派生或只读
- 依赖：无

## identity.encodeAccountIndex

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：唯一编码与16MiB大小约束，提交前、写入、恢复一致

```go
func encodeAccountIndex(input WriteAccountIndexInput) (AccountIndex, []byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 唯一编码与16MiB大小约束，提交前、写入、恢复一致

**错误**

- 无

- 副作用：无
- 授权：本域受信私有用例或恢复
- 并发：遵循本域账号互斥与外层事务
- 幂等：纯派生或只读
- 依赖：无

## identity.NewBackgroundTasks

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：装配本域后台重置观察与outbox转发入口

```go
func NewBackgroundTasks(store *Store, siteConfig platform.SiteConfigClient, snapshot *lifecycle.SnapshotManager, publisher platform.EventPublisher, dataEpoch platform.Identifier) ([]platform.BackgroundTask, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `*Store` | identity active业务库句柄 | 非nil且数据库已打开 |
| `siteConfig` | `platform.SiteConfigClient` | site-config运行权威客户端 | 非nil可信客户端 |
| `snapshot` | `*lifecycle.SnapshotManager` | 本域唯一SnapshotManager | 非nil真实scope |
| `publisher` | `platform.EventPublisher` | jobs事件发布端口 | 非nil版本化客户端 |
| `dataEpoch` | `platform.Identifier` | 启动时绑定的数据代次 | 有效非空标识 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tasks` | `[]platform.BackgroundTask` | 本域已登记后台入口 | 仅包含两个真实任务 |
| `error` | `error` | 依赖缺失的安全错误 | 失败时不返回业务结果 |

**步骤**

1. 缺依赖不登记后台任务

**错误**

- 无

- 副作用：无
- 授权：仅完整启动装配
- 并发：受信后台串行或纯函数
- 幂等：按输入/既有持久回执
- 依赖：`identity.runSuperadminResetObserver`、`identity.runOutboxRelay`

## identity.runSuperadminResetObserver

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：观察固定超管重置文件

```go
func runSuperadminResetObserver(ctx context.Context, store *Store, siteConfig platform.SiteConfigClient, snapshot *lifecycle.SnapshotManager, dataEpoch platform.Identifier) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 后台任务上下文 | 取消即停止 |
| `store` | `*Store` | identity active业务库 | 非nil已打开 |
| `siteConfig` | `platform.SiteConfigClient` | 运行权威客户端 | 非nil |
| `snapshot` | `*lifecycle.SnapshotManager` | 本域保护scope | 非nil |
| `dataEpoch` | `platform.Identifier` | 任务绑定代次 | 有效且不从请求自报 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 后台循环终止错误 | 失败时不返回业务结果 |

**步骤**

1. 通过platform.RunRecoveryLoop每5秒执行观察与应用

**错误**

- 无

- 副作用：无
- 授权：仅完整启动装配
- 并发：受信后台串行或纯函数
- 幂等：按输入/既有持久回执
- 依赖：`identity.waitForIdentitySite`、`identity.identityBackgroundOperation`、`platform.RunRecoveryLoop`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`、`identity.ObserveSuperadminResetFile`

## identity.runOutboxRelay

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：持续转发已提交outbox

```go
func runOutboxRelay(ctx context.Context, store *Store, siteConfig platform.SiteConfigClient, snapshot *lifecycle.SnapshotManager, publisher platform.EventPublisher, dataEpoch platform.Identifier) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 后台任务上下文 | 取消即停止 |
| `store` | `*Store` | identity active业务库 | 非nil已打开 |
| `siteConfig` | `platform.SiteConfigClient` | 运行权威客户端 | 非nil |
| `snapshot` | `*lifecycle.SnapshotManager` | 本域保护scope | 非nil |
| `publisher` | `platform.EventPublisher` | jobs发布端口 | 非nil |
| `dataEpoch` | `platform.Identifier` | 任务绑定代次 | 有效且不从请求自报 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 后台循环终止错误 | 失败时不返回业务结果 |

**步骤**

1. 通过platform.RunRecoveryLoop每秒调用单批RelayOutbox

**错误**

- 无

- 副作用：无
- 授权：仅完整启动装配
- 并发：受信后台串行或纯函数
- 幂等：按输入/既有持久回执
- 依赖：`identity.waitForIdentitySite`、`identity.identityBackgroundOperation`、`identity.WithRelayRuntime`、`platform.RunRecoveryLoop`、`platform.SnapshotManager.AdmitEventBatch`、`platform.SnapshotManager.BindOutboxEventScope`、`platform.SnapshotManager.WriteWindow`、`identity.RelayOutbox`

## identity.waitForIdentitySite

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：读取后台固定站点scope

```go
func waitForIdentitySite(ctx context.Context, store *Store, interval time.Duration) (platform.Identifier, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 权威轮询上下文 | 取消即返回 |
| `store` | `*Store` | identity active业务库 | 非nil已打开 |
| `interval` | `time.Duration` | 轮询间隔 | 由固定后台常量提供 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `siteID` | `platform.Identifier` | 唯一超管所属站点 | 仅唯一active超管时成功 |
| `error` | `error` | 权威读取错误 | 失败时不返回业务结果 |

**步骤**

1. 读取唯一超管；无账号等待

**错误**

- 无

- 副作用：无
- 授权：仅完整启动装配
- 并发：受信后台串行或纯函数
- 幂等：按输入/既有持久回执
- 依赖：`identity.readUniqueSuperadmin`、`identity.identityDBError`

## identity.identityBackgroundOperation

- 级别：参考
- 文件：`modules/identity/internal/server_recovery.go`
- 作用：构造后台服务操作scope

```go
func identityBackgroundOperation(siteID, dataEpoch platform.Identifier, deadline time.Time) platform.OperationContext
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `siteID` | `platform.Identifier` | 后台固定站点 | 由本域权威读取取得 |
| `dataEpoch` | `platform.Identifier` | 后台绑定代次 | 启动装配提供有效值 |
| `deadline` | `time.Time` | 本轮后台操作截止 | UTC有效时间 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `op` | `platform.OperationContext` | 身份后台服务操作上下文 | CallerService为identity且Actor为service |

**步骤**

1. 生成并绑定operationId、siteId、dataEpoch、deadline

**错误**

- 无

- 副作用：无
- 授权：仅完整启动装配
- 并发：受信后台串行或纯函数
- 幂等：按输入/既有持久回执
- 依赖：无

## identity.WithIdentityStore

- 级别：参考
- 文件：`modules/identity/internal/operation_repository.go`
- 作用：把受信身份Store注入本域上下文

```go
func WithIdentityStore(ctx context.Context, store *Store) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 以私有context key绑定启动持有的Store，不将句柄编码到网络

**错误**

- 无

- 副作用：无
- 授权：仅本域受信启动装配或私有仓库链调用，句柄不来自网络
- 并发：上下文持有只读资源引用；存储并发由具体用例事务约束
- 幂等：不改变业务状态
- 依赖：无

## identity.identityStoreFromContext

- 级别：参考
- 文件：`modules/identity/internal/operation_repository.go`
- 作用：读取本域已装配Store引用

```go
func identityStoreFromContext(ctx context.Context) (*Store, bool)
```

**输入**

无

**输出**

无

**步骤**

1. 仅从本域私有context key取Store，缺失或nil返回false

**错误**

- 无

- 副作用：无
- 授权：仅本域受信启动装配或私有仓库链调用，句柄不来自网络
- 并发：上下文持有只读资源引用；存储并发由具体用例事务约束
- 幂等：不改变业务状态
- 依赖：无

## identity.checkIdentityRepositoryContext

- 级别：参考
- 文件：`modules/identity/internal/operation_repository.go`
- 作用：保留既有outbox/恢复仓库调用前置

```go
func checkIdentityRepositoryContext(ctx context.Context, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 现有outbox/恢复入口检查取消与可信operationId/dataEpoch/deadline；不从网络重建或扩大op

**错误**

- 无

- 副作用：无
- 授权：仅本域受信启动装配或私有仓库链调用，句柄不来自网络
- 并发：上下文持有只读资源引用；存储并发由具体用例事务约束
- 幂等：不改变业务状态
- 依赖：无

## identity.PublishIdentityEvent

- 级别：参考
- 文件：`modules/identity/internal/outbox_repository.go`
- 作用：与身份提交原子登记事件

```go
func PublishIdentityEvent(ctx context.Context, op OperationContext, input PublishIdentityEventInput) (PublishIdentityEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PublishIdentityEventInput` | 与身份提交原子登记事件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishIdentityEventResult` | 与身份提交原子登记事件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 在身份业务事务内按events.json登记唯一事件信封并调用平台outbox.EnqueueEvent
2. 受信WithIdentityTransaction存在时只追加outbox，由外层业务用例统一请求摘要、完整结果回执、提交或回滚；未注入外部事务时打开自有事务并保存事件登记操作的幂等回执
3. 仅提交后由RelayOutbox投递jobs，不直接广播；同事件ID不同载荷失败关闭

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：私有身份txn
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：外部事务由业务用例维护完整请求/结果回执；自有事务按事件输入摘要保存本操作回执；outbox按eventId去重
- 依赖：`platform.EnqueueEvent`、`identity.identityStoreFromContext`、`identity.checkIdentityRepositoryContext`

## identity.RelayOutbox

- 级别：约束
- 文件：`modules/identity/internal/outbox_repository.go`
- 作用：把已提交的outbox事件可靠转交jobs；进程启动先补发，之后后台循环

```go
func RelayOutbox(ctx context.Context, op OperationContext, input RelayOutboxInput) (RelayOutboxResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 本域后台worker以服务身份构建的操作上下文，含当前dataEpoch | callerService为本域；dataEpoch等于site-config当前active epoch |
| `input` | `RelayOutboxInput` | 单批转发参数 | 必须通过公共schema；batchLimit 1..100 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RelayOutboxResult` | 本批转发计数 | 无error时有效；relayed/pending/quarantined只描述本批 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 公共持续循环启动后立即调用一次，之后每秒调用；每次按outbox序号以30秒行租约领取≤batchLimit条未转发行，只读已提交事务
2. 逐条调用jobs:publishEvent，携带固定eventId、payloadHash、dataEpoch与subject revision；jobs持久确认后在本地事务把该行标relayed并记录jobs eventSeq
3. 网络失败或超时保留该行，按1/2/4秒指数退避（上限60秒）在下一轮重试；同eventId异hash或信封校验失败标quarantined并报警，不再重试也不删除
4. 屏障draining期间继续转发已提交行直至为空或屏障超时；快照包含未转发行；恢复换入新数据后丢弃旧dataEpoch未转发行，以恢复后的权威状态为准

**错误**

- `VALIDATION`：batchLimit不在1..100（不可重试）
- `FORBIDDEN`：不是identity服务后台调用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：jobs、site-config或本域运行依赖不可用；行保留待下一轮（可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash或信封不合法；行隔离（不可重试）
- `VERSION_MISMATCH`：dataEpoch与当前active epoch不一致；旧代行按恢复规则丢弃（不可重试）

- 副作用：读写本域outbox行状态与租约；HTTP调用jobs:publishEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个转发循环；行级租约防多实例重复领取；同subject按outbox序号串行发送
- 幂等：eventId在业务事务内固定；jobs按eventId+payloadHash去重，重复发送返回原确认
- 依赖：`jobs:publishEvent`、`platform.RunRelayBatch`、`site-config:readRuntimeState`、`platform.SnapshotManager.AdmitEventBatch`、`platform.SnapshotManager.BindOutboxEventScope`、`platform.SnapshotManager.WriteWindow`、`identity.checkIdentityRepositoryContext`

## identity.WithRelayRuntime

- 级别：参考
- 文件：`modules/identity/internal/outbox_repository.go`
- 作用：向后台RelayOutbox注入真实存储、jobs、运行权威和Manager scope

```go
func WithRelayRuntime(ctx context.Context, store *Store, publisher platform.EventPublisher, snapshot *lifecycle.SnapshotManager, siteConfig platform.SiteConfigClient) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 后台转发可信上下文 | 仅进程内上下文；不接受网络输入 |
| `store` | `*Store` | identity active业务库句柄 | 必须为非nil且持有数据库 |
| `publisher` | `platform.EventPublisher` | jobs事件发布端口 | 必须为非nil版本化客户端 |
| `snapshot` | `*lifecycle.SnapshotManager` | 本域唯一SnapshotManager | 必须为非nil真实scope |
| `siteConfig` | `platform.SiteConfigClient` | site-config运行权威客户端 | 必须为非nil可信客户端 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `context.Context` | 携带真实转发依赖的上下文 | 仅保存私有句柄；缺任一依赖时RelayOutbox失败关闭 |

**步骤**

1. 以私有context key保存进程内句柄；不从请求或配置正文接受替代依赖

**错误**

- 无

- 副作用：无
- 授权：仅完整启动后台装配
- 并发：只读句柄注入；实际IO由转发循环和Manager控制
- 幂等：确定性上下文转换
- 依赖：无

## identity.InvalidateRestoredSessions

- 级别：参考
- 文件：`modules/identity/internal/restore_security.go`
- 作用：恢复数据后使备份登录失效

```go
func InvalidateRestoredSessions(ctx context.Context, op OperationContext, input InvalidateRestoredSessionsInput) (InvalidateRestoredSessionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `InvalidateRestoredSessionsInput` | 恢复数据后使备份登录失效的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InvalidateRestoredSessionsResult` | 恢复数据后使备份登录失效的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅由identity.BuildApplication在本次启动换入恢复数据后调用一次
2. 生成新authEpoch并写auth_state单行权威；同事务撤销全部session和refresh族并提升水位；管理员需用备份中的账号重新登录
3. 保留账户密码/flags/超管唯一身份；服务签名密钥在control区，不随恢复替换

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：业务会话失效
- 授权：仅identity启动流程在换入恢复数据后内部调用
- 并发：启动阶段独占，入口开放前完成
- 幂等：同restoreId只执行一次，control日志记录
- 依赖：`identity.identityStoreFromContext`、`identity.checkIdentityRepositoryContext`

## identity.WithIdentityRestoreID

- 级别：参考
- 文件：`modules/identity/internal/restore_security.go`
- 作用：把公共恢复编排已核验的restoreId绑定到本域启动调用

```go
func WithIdentityRestoreID(ctx context.Context, restoreID platform.Identifier) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 启动恢复可信上下文 | 仅进程内注入，不接受网络输入 |
| `restoreID` | `platform.Identifier` | 公共恢复编排已核验的restoreId | 必须为有效非空标识；不从dataEpoch推导 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `context.Context` | 携带恢复标识的可信上下文 | 只保存私有context key；不序列化句柄 |

**步骤**

1. 以私有context key保存restoreId，禁止从请求字段或数据代次猜测

**错误**

- 无

- 副作用：无
- 授权：仅启动恢复装配
- 并发：纯上下文转换
- 幂等：确定性，不改变业务状态
- 依赖：无

## identity.AcquireBarrier

- 级别：约束
- 文件：`modules/identity/internal/barrier_service.go`
- 作用：取得可恢复的数据屏障

```go
func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `BarrierRequest` | 取得可恢复的数据屏障的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 取得可恢复的数据屏障的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准 |

**步骤**

1. 认证 data-protection 服务及 superadmin 委托；验证 epoch、operationId 与 deadline
2. 私有事务登记屏障并拒绝新的业务写入；等待已登记提交完成
3. 固定 revisionWatermark 并返回 safe；超时返回 draining/failed 且保持维护，不擅自释放

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `DEADLINE_EXCEEDED`：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准（可重试）

- 副作用：本服务屏障/快照/恢复日志与私有对象写入
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：屏障与暂存按 barrierId/restoreId 串行；活动事务计数达到零后才 safe；恢复在进程重启时整体换入active目录
- 幂等：operationId/backupId/restoreId+摘要去重；重复返回已记录阶段
- 依赖：`identity.requireProtectionService`、`identity.barrierMode`、`identity.snapshotSealProof`、`identity.barrierStatus`、`identity.snapshotManagerFromContext`、`site-config:readRuntimeState`、`platform.SnapshotManager.EnterBarrier`、`platform.SnapshotManager.ReadBarrierStatus`

## identity.ReleaseBarrier

- 级别：约束
- 文件：`modules/identity/internal/barrier_service.go`
- 作用：释放已协调完成的屏障

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, input ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReleaseBarrierInput` | 释放已协调完成的屏障的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 释放已协调完成的屏障的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变 |

**步骤**

1. 查询持久屏障
2. 验证维护退出决定、全部域切换/换回回执已齐及 epoch 一致
3. 原子改 released 并恢复业务接入

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）

- 副作用：私有屏障状态写入
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.requireProtectionService`、`identity.snapshotManagerFromContext`、`identity.barrierStatusValue`、`site-config:readRuntimeState`、`platform.SnapshotManager.ReleaseBarrier`

## identity.requireProtectionService

- 级别：参考
- 文件：`modules/identity/internal/barrier_service.go`
- 作用：核验data-protection调用及service或超管短委托授权

```go
func requireProtectionService(op OperationContext) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `op` | `OperationContext` | 平台入口建立的可信操作上下文，含CallerService与Actor | 仅接受已完成服务身份、dataEpoch和委托校验的上下文 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 授权失败或依赖缺失的安全错误 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 确认CallerService为data-protection
2. 接受data-protection服务主体或同调用链的有效superadmin委托，其他主体失败关闭

**错误**

- 无

- 副作用：无
- 授权：data-protection服务主体或其携带的已验证superadmin委托
- 并发：只读可信操作上下文
- 幂等：纯校验
- 依赖：`identity.validSuperadminDelegation`

## identity.barrierMode

- 级别：参考
- 文件：`modules/identity/internal/barrier_service.go`
- 作用：解析备份或恢复屏障模式

```go
func barrierMode(value interface{}) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `value` | `interface{}` | OpenAPI屏障模式原始值 | 只接受字符串backup或restore；不信任未校验网络值 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `mode` | `string` | 规范化屏障模式 | 仅为backup或restore |
| `error` | `error` | 安全失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 仅接受backup或restore枚举，未知值返回VALIDATION

**错误**

- 无

- 副作用：无
- 授权：本域屏障用例
- 并发：纯函数
- 幂等：确定性
- 依赖：无

## identity.snapshotSealProof

- 级别：参考
- 文件：`modules/identity/internal/barrier_service.go`
- 作用：把生成DTO转换为公共封口证明

```go
func snapshotSealProof(input gen.SnapshotSealProof) (lifecycle.SnapshotSealProof, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `gen.SnapshotSealProof` | 生成代码校验后的封口证明DTO | 字段已通过入口schema校验；仅做本域到公共类型转换 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `proof` | `lifecycle.SnapshotSealProof` | 公共SnapshotManager使用的封口证明 | 逐字段保留broker、owner、work和非jobs回执；输入不完整时无成功结果 |
| `error` | `error` | 安全失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 逐项转换broker、观察、非jobs回执与水位
2. 缺屏障或dataEpoch失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域屏障用例
- 并发：纯函数
- 幂等：确定性
- 依赖：无

## identity.barrierStatus

- 级别：参考
- 文件：`modules/identity/internal/barrier_service.go`
- 作用：把公共屏障状态映射为HTTP结果

```go
func barrierStatus(status lifecycle.SnapshotBarrierStatus, observation *lifecycle.SnapshotBarrierObservation) BarrierStatus
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `status` | `lifecycle.SnapshotBarrierStatus` | 公共SnapshotManager返回的屏障状态 | 仅使用公共Manager已证明的状态、代次、水位和seal hash |
| `observation` | `*lifecycle.SnapshotBarrierObservation` | 可选的本域观察事实 | nil表示调用方没有观察事实；不补造观察值 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 本域屏障状态DTO | 只映射公共状态；不增加未经Manager证明的字段 |

**步骤**

1. 映射状态、代次、水位、seal hash和observe事实

**错误**

- 无

- 副作用：无
- 授权：本域屏障用例
- 并发：纯函数
- 幂等：确定性
- 依赖：`identity.barrierStatusValue`

## identity.barrierStatusValue

- 级别：参考
- 文件：`modules/identity/internal/barrier_service.go`
- 作用：生成安全屏障DTO

```go
func barrierStatusValue(status lifecycle.SnapshotBarrierStatus, observation *lifecycle.SnapshotBarrierObservation) gen.BarrierStatus
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `status` | `lifecycle.SnapshotBarrierStatus` | 公共SnapshotManager返回的屏障状态 | 只读公共Manager结果 |
| `observation` | `*lifecycle.SnapshotBarrierObservation` | 可选的本域观察事实 | nil保持DTO观察字段为空；不生成默认事实 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `gen.BarrierStatus` | 生成目录定义的屏障状态DTO | 逐字段映射owner、state、dataEpoch、watermark、seal hash和observation |

**步骤**

1. 不添加未由公共Manager证明的字段

**错误**

- 无

- 副作用：无
- 授权：本域屏障用例
- 并发：纯函数
- 幂等：确定性
- 依赖：无

## identity.ExportSnapshot

- 级别：约束
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：导出屏障水位的一致快照

```go
func ExportSnapshot(ctx context.Context, op OperationContext, input SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `SnapshotExportRequest` | 导出屏障水位的一致快照的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 导出屏障水位的一致快照的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准 |

**步骤**

1. 验证本域 barrier=safe 和当前 dataEpoch；按 includeSecrets 策略封装敏感字段
2. 流式复制私有数据库一致视图及所有权威文件到本服务只读对象；manifest 列 hash/size/schema
3. fsync 完成后返回 SnapshotRef；失败只清理未完成临时对象，不修改活动数据

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `DEADLINE_EXCEEDED`：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准（可重试）

- 副作用：本服务屏障/快照/恢复日志与私有对象写入
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：屏障与暂存按 barrierId/restoreId 串行；活动事务计数达到零后才 safe；恢复在进程重启时整体换入active目录
- 幂等：operationId/backupId/restoreId+摘要去重；重复返回已记录阶段
- 依赖：`identity.requireProtectionService`、`identity.snapshotManagerFromContext`、`identity.snapshotRefFromLifecycle`、`site-config:readRuntimeState`、`platform.SnapshotManager.Export`

## identity.StageSnapshot

- 级别：约束
- 文件：`modules/identity/internal/snapshot_service.go`
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
- 依赖：`identity.validSuperadminDelegation`、`identity.restoreArchiveClientFromContext`、`identity.snapshotManagerFromContext`、`identity.snapshotRefToLifecycle`、`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`、`platform.SnapshotManager.Stage`

## identity.DiscardSnapshot

- 级别：约束
- 文件：`modules/identity/internal/snapshot_service.go`
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
- 依赖：`identity.validSuperadminDelegation`、`identity.snapshotManagerFromContext`、`site-config:readRuntimeState`、`platform.SnapshotManager.Discard`

## identity.WithSnapshotManager

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：向本域可信上下文注入同一快照管理器

```go
func WithSnapshotManager(ctx context.Context, manager *lifecycle.SnapshotManager) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信上下文 | 仅进程内上下文，不接受网络输入 |
| `manager` | `*lifecycle.SnapshotManager` | 启动装配的公共快照管理器 | 必须为本域唯一共享的非nil实例 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `context.Context` | 携带SnapshotManager句柄的上下文 | 只保存私有context key；不序列化或改变业务状态 |

**步骤**

1. 仅保存进程内manager句柄，不接受网络输入

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.WithSnapshotRuntime

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：注入site-config运行权威客户端

```go
func WithSnapshotRuntime(ctx context.Context, runtime platform.SiteConfigClient) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信上下文 | 仅进程内上下文，不接受网络输入 |
| `runtime` | `platform.SiteConfigClient` | site-config运行权威客户端 | 由启动装配提供非nil客户端；不接受请求方替代 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `context.Context` | 携带运行权威客户端的上下文 | 只保存私有context key；不复制运行状态 |

**步骤**

1. 仅保存进程内客户端句柄

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.WithRestoreArchiveClient

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：注入data-protection归档读取端口

```go
func WithRestoreArchiveClient(ctx context.Context, client RestoreArchiveClient) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信上下文 | 仅进程内上下文，不接受网络输入 |
| `client` | `RestoreArchiveClient` | data-protection受控归档读取端口 | 由启动装配提供版本化非nil客户端；不接受路径或URL |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `context.Context` | 携带受控归档端口的上下文 | 只保存私有context key；不暴露客户端句柄 |

**步骤**

1. 仅保存版本化受控客户端句柄

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.snapshotManagerFromContext

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：取得可信快照管理器

```go
func snapshotManagerFromContext(ctx context.Context) (*lifecycle.SnapshotManager, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信上下文 | 必须由启动装配注入SnapshotManager |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `manager` | `*lifecycle.SnapshotManager` | 可信公共快照管理器 | 成功时非nil；缺失时不返回空管理器 |
| `error` | `error` | 管理器未注入时的DEPENDENCY_UNAVAILABLE | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 缺失时DEPENDENCY_UNAVAILABLE，禁止伪造manager

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.snapshotRuntimeFromContext

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：取得可信运行权威客户端

```go
func snapshotRuntimeFromContext(ctx context.Context) (platform.SiteConfigClient, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信上下文 | 必须由启动装配注入site-config客户端 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `runtime` | `platform.SiteConfigClient` | 可信运行权威客户端 | 成功时非nil；缺失时失败关闭 |
| `error` | `error` | 运行权威未注入时的DEPENDENCY_UNAVAILABLE | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 缺失时失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.restoreArchiveClientFromContext

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：取得data-protection受控归档读取端口

```go
func restoreArchiveClientFromContext(ctx context.Context) (RestoreArchiveClient, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信上下文 | 必须由启动装配注入归档读取端口 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `client` | `RestoreArchiveClient` | data-protection受控归档读取端口 | 成功时非nil；缺失时失败关闭 |
| `error` | `error` | 归档端口未注入时的DEPENDENCY_UNAVAILABLE | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 缺失时失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.snapshotRefToLifecycle

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：把HTTP快照引用映射为公共类型

```go
func snapshotRefToLifecycle(input gen.SnapshotRef) lifecycle.SnapshotRef
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `gen.SnapshotRef` | 生成代码定义的快照引用DTO | 字段已由入口schema约束；只允许映射已声明owner、版本、hash、长度和数据代次 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `lifecycle.SnapshotRef` | 公共快照引用 | 逐字段映射，不生成ID、版本、hash或长度 |

**步骤**

1. 逐字段保留owner、版本、hash、长度、代次

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.snapshotRefFromLifecycle

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：把公共快照引用映射为HTTP类型

```go
func snapshotRefFromLifecycle(input lifecycle.SnapshotRef) gen.SnapshotRef
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `lifecycle.SnapshotRef` | 公共快照引用 | 只接受公共Manager返回的已核引用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `gen.SnapshotRef` | 生成代码定义的快照引用DTO | 逐字段映射公共ObjectRef；不生成新值 |

**步骤**

1. 逐字段映射公共ObjectRef，不生成新值

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：无

## identity.NewSnapshotManager

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：装配identity真实SnapshotConfig

```go
func NewSnapshotManager(store *Store, dataRoot string, openedEpoch platform.Identifier, runtime platform.SiteConfigClient) (*lifecycle.SnapshotManager, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `*Store` | identity active业务库与control句柄 | 必须为非nil且已打开业务数据库 |
| `dataRoot` | `string` | 本域私有数据根目录 | 启动装配提供绝对私有路径；不来自HTTP输入 |
| `openedEpoch` | `platform.Identifier` | 当前已打开的数据代次 | 必须为有效非空数据代次 |
| `runtime` | `platform.SiteConfigClient` | site-config运行权威客户端 | 必须为非nil可信客户端 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `manager` | `*lifecycle.SnapshotManager` | 装配好的本域公共快照管理器 | 成功时非nil并绑定全部本域回调；全域证明缺失时运行封口失败关闭 |
| `error` | `error` | SnapshotManager配置不完整或公共构造失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 装配Runtime、drain、watermark、ObserveWork、SealWork、ResumeWork、Capture、Stage迁移与校验回调
2. VerifyGlobalSeal在协调协议未提供时失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.NewSnapshotManager`、`platform.ReadRuntime`、`identity.beginIdentityDrain`、`identity.drainIdentityWork`、`identity.identityWatermark`、`identity.observeIdentityWork`、`identity.sealIdentityWork`、`identity.resumeIdentityWork`、`identity.captureIdentitySnapshot`、`identity.migrateIdentityStage`、`identity.validateIdentityStaged`、`identity.identityStageBudget`、`identity.validateIdentitySnapshotManifest`

## identity.beginIdentityDrain

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：验证identity库与outbox在进入drain时可读

```go
func beginIdentityDrain(ctx context.Context, store *Store) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager排空上下文 | 沿用外层截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 必须为已打开的非nil Store |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | outbox schema或业务库观察不可用时的安全错误 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 确保outbox schema存在
2. Ping业务库并读取真实outbox观察

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.EnsureSchema`、`platform.Store.Observe`

## identity.drainIdentityWork

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：等待identity真实未决work排空

```go
func drainIdentityWork(ctx context.Context, store *Store) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager排空上下文 | 沿用外层截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 必须为已打开的非nil Store |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 未决本域工作未在截止前排空或观察失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 反复读取同库work事实与outbox活动
2. 截止前未清零返回DEADLINE_EXCEEDED

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`identity.observeIdentityWork`

## identity.observeIdentityWork

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：在同一SQLite事务读取outbox和identity持久work事实

```go
func observeIdentityWork(ctx context.Context, store *Store) (lifecycle.SnapshotWorkState, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager观察上下文 | 沿用外层截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 必须为已打开的非nil Store；观察与outbox使用同一SQLite |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `work` | `lifecycle.SnapshotWorkState` | identity本域持久工作观察值 | 包含真实outbox revision、workRevision、pendingOutbox和pendingWork；任一水位未知则无成功结果 |
| `error` | `error` | 同库事务、outbox或统一workRevision不可读 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 调用platform.EnsureSchema和platform.Store.ObserveTx读取outbox事实
2. 读取统一持久workRevision与未决决定
3. 统一workRevision未冻结时失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.EnsureSchema`、`platform.Store.ObserveTx`、`identity.readIdentityWorkRevision`、`identity.readIdentityPendingWorkQuery`

## identity.readIdentityWorkRevision

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：复用同库outbox control的真实单调revision作为identity workRevision

```go
func readIdentityWorkRevision(ctx context.Context, store *Store, tx platform.Tx) (int64, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 同库事务查询上下文 | 沿用外层事务截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 非nil且绑定同一SQLite |
| `tx` | `platform.Tx` | 调用方持有的同库事务 | 必须是ObserveWork/SealWork当前事务；不由本函数提交或回滚 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `revision` | `int64` | 公共outbox control真实单调revision，作为identity workRevision | 来自同一事务platform.Store.ObserveTx；必须为正数 |
| `error` | `error` | 真实revision不可读的安全错误 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 在调用方事务中调用platform.Store.ObserveTx并返回真实Revision；不读取业务表MAX/COUNT

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：遵循调用方SQLite事务串行
- 幂等：只读同库事实
- 依赖：`platform.Store.ObserveTx`

## identity.readIdentityPendingWorkQuery

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：读取未决回执、bootstrap决定与journal open事实

```go
func readIdentityPendingWorkQuery(ctx context.Context, query interface { QueryRowContext(context.Context, string, ...any) *sql.Row }, store *Store) (int64, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 同库待决事实查询上下文 | 沿用外层事务截止和取消信号 |
| `query` | `interface { QueryRowContext(context.Context, string, ...any) *sql.Row }` | 提供QueryRowContext的identity业务库查询器 | 必须读取同一active SQLite事务 |
| `store` | `*Store` | identity control与journal句柄 | 必须为已打开的非nil Store；control决定与业务库按既有恢复协议读取 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `pending` | `int64` | 未决回执、bootstrap reserved决定和open journal条目数量 | 非负；不把稳定保留或未知状态当作已排空 |
| `error` | `error` | 待决持久事实不可读 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 从传入同库query读取未决回执
2. 读取control bootstrap reserved与journal open

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`identity.readJournal`

## identity.identityWatermark

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：读取identity真实安全截点水位

```go
func identityWatermark(ctx context.Context, store *Store) (int64, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager水位查询上下文 | 沿用外层截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 必须为已打开的非nil Store |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `watermark` | `int64` | identity安全截点revisionWatermark | 必须来自统一持久workRevision；缺口时失败关闭 |
| `error` | `error` | 水位事务或统一workRevision不可用 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 开启只读事务读取统一持久workRevision
2. 缺统一序号失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`identity.readIdentityWorkRevision`

## identity.sealIdentityWork

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：同库CAS identity work水位与outbox封口

```go
func sealIdentityWork(ctx context.Context, store *Store, expected lifecycle.SnapshotWorkState, proofHash string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager封口上下文 | 沿用外层截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 必须为已打开的非nil Store |
| `expected` | `lifecycle.SnapshotWorkState` | 同一次观察得到的本域工作水位 | 必须原样绑定outboxRevision、workRevision和pending计数 |
| `proofHash` | `string` | 公共封口证明摘要 | 必须是已由公共Manager计算的有效hash；不接受请求方自报替代 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 同库水位改变、统一CAS未装配或outbox事务封口失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 同一事务调用platform.Store.ObserveTx并核expected水位
2. 统一workRevision未装配或CAS不符失败
3. 调用platform.Store.SealTx并提交同一事务

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.Store.ObserveTx`、`identity.readIdentityWorkRevision`、`identity.readIdentityPendingWorkQuery`、`platform.Store.SealTx`

## identity.resumeIdentityWork

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：按原seal hash同库重开identity旧work

```go
func resumeIdentityWork(ctx context.Context, store *Store, proofHash string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager重开上下文 | 沿用外层截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 必须为已打开的非nil Store |
| `proofHash` | `string` | 原公共封口证明摘要 | 必须精确绑定原seal hash；不接受新摘要 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | outbox ResumeTx缺失、原封口绑定不符或事务提交失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 同一事务调用platform.Store.ResumeTx
2. 提交失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.Store.ResumeTx`

## identity.captureIdentitySnapshot

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：捕获identity active数据库与完整V1 manifest

```go
func captureIdentitySnapshot(ctx context.Context, store *Store, dataRoot string, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager文件捕获上下文 | 沿用外层截止和取消信号 |
| `store` | `*Store` | identity active业务库句柄 | 必须为已打开的非nil Store |
| `dataRoot` | `string` | 本域私有数据根目录 | 必须是启动装配绑定的绝对私有路径 |
| `cut` | `lifecycle.SnapshotSafeCut` | 公共Manager已证明的safe截点 | barrier、dataEpoch和watermark必须来自同一safe记录 |
| `workspace` | `string` | 公共Manager分配的隔离工作目录 | 必须是受控私有stage路径 |
| `input` | `lifecycle.SnapshotExportInput` | 公共导出输入 | 必须与cut的barrier和dataEpoch一致；不改变公共输入 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `manifest` | `lifecycle.DomainSnapshotManifest` | identity V1域快照清单 | owner为identity；文件、schema、源数据代次和revisionWatermark均来自真实捕获 |
| `error` | `error` | 水位变化、数据库捕获、迁移集或清单构造失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 核safe epoch、barrier和统一work watermark
2. 用platform.CaptureSnapshotFiles复制identity.db并fsync/hash
3. 前后水位不一致失败

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`identity.identityWatermark`、`platform.EnsureSchema`、`platform.CaptureSnapshotFiles`、`identity.All`

## identity.validateIdentitySnapshotManifest

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：严格校验identity V1 manifest canonical JSON

```go
func validateIdentitySnapshotManifest(raw []byte) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `raw` | `[]byte` | 待校验的域快照manifest JSON | 必须是完整原始清单；不得含未知字段或多余JSON值 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 清单schema、canonical编码或库存不合法 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. DisallowUnknownFields并核canonical编码
2. 只接受identity owner、V1和非nil基础库存

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.CanonicalJSON`

## identity.identityStageBudget

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：计算identity staged迁移峰值预算

```go
func identityStageBudget(ctx context.Context, manifest lifecycle.DomainSnapshotManifest) (int64, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager预算上下文 | 沿用外层截止和取消信号 |
| `manifest` | `lifecycle.DomainSnapshotManifest` | 已验证的identity域快照清单 | 只接受identity V1和单identity.db；文件大小受公共归档预算约束 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `budget` | `int64` | identity staged迁移峰值预算 | 正数且不超过公共最大归档预算 |
| `error` | `error` | manifest版本、文件库存或预算超限 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 只接受identity V1单identity.db
2. 给出源库、迁移和journal峰值上界

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.CanonicalJSON`

## identity.migrateIdentityStage

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：在认证隔离stage中迁移identity库并隔离恢复outbox

```go
func migrateIdentityStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager恢复迁移上下文 | 沿用外层截止和取消信号 |
| `manifest` | `lifecycle.DomainSnapshotManifest` | 已核验的identity域快照清单 | 必须是identity V1清单 |
| `targetEpoch` | `string` | 公共恢复目标数据代次 | 必须是有效代次且与manifest.SourceEpoch不同 |
| `root` | `string` | 公共Manager认证后的staged根目录 | 必须与platform.SnapshotStageBinding返回的私有root完全一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 核platform.SnapshotStageBinding
2. 调用identity.migrateIdentitySnapshotDB经platform.MigrateRestoredDatabase执行已登记迁移和旧资格隔离
3. 验证数据库

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.SnapshotStageBinding`、`identity.identityStageBudget`、`identity.openIdentitySnapshotDB`、`identity.migrateIdentitySnapshotDB`、`identity.validateIdentitySnapshotDatabase`

## identity.validateIdentityStaged

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：重启前验证identity staged库与outbox封口证据

```go
func validateIdentityStaged(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, targetEpoch, root string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 公共SnapshotManager恢复验证上下文 | 沿用外层截止和取消信号 |
| `manifest` | `lifecycle.DomainSnapshotManifest` | 已核验的identity域快照清单 | 必须是identity V1清单 |
| `targetEpoch` | `string` | 公共恢复目标数据代次 | 必须是有效代次且与manifest.SourceEpoch不同 |
| `root` | `string` | 公共Manager认证后的staged根目录 | 必须是同一restore stage私有路径 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 核target epoch和预算
2. 只读打开staged identity.db
3. 核schema、认证代次和platform.Store.Observe返回的outbox seal hash

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`identity.identityStageBudget`、`identity.openIdentitySnapshotDB`、`identity.validateIdentitySnapshotDatabase`、`platform.Store.Observe`

## identity.openIdentitySnapshotDB

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：以私有路径边界打开隔离identity数据库

```go
func openIdentitySnapshotDB(ctx context.Context, path string, readOnly bool) (*sql.DB, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 恢复数据库打开上下文 | 沿用外层截止和取消信号 |
| `path` | `string` | staged identity.db私有路径 | 必须经platform.ValidatePrivatePath核验的regular-file路径 |
| `readOnly` | `bool` | 是否以只读模式打开 | 只读验证为true；迁移写入为false |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `db` | `*sql.DB` | 隔离identity数据库句柄 | 成功时单连接；调用方负责关闭 |
| `error` | `error` | 路径、打开、Ping或journal模式失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 调用platform.ValidatePrivatePath
2. 单连接Ping并设置foreign_keys/busy_timeout
3. 写模式使用DELETE journal

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`platform.ValidatePrivatePath`

## identity.migrateIdentitySnapshotDB

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：经公共恢复数据库机制在同一事务应用已登记迁移并登记业务work表

```go
func migrateIdentitySnapshotDB(ctx context.Context, db *sql.DB) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 恢复迁移上下文 | 沿用外层截止和取消信号 |
| `db` | `*sql.DB` | 隔离staged identity数据库 | 必须为可写单连接数据库；不修改active数据库 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 调用platform.MigrateRestoredDatabase；回调只执行identity.All登记迁移
2. 迁移完成后按identityWorkTables调用platform.RegisterWorkTable；不重标原队列epoch

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：公共恢复数据库事务串行
- 幂等：按schema_migrations与公共恢复回执
- 依赖：`platform.MigrateRestoredDatabase`、`platform.RegisterWorkTable`、`identity.All`

## identity.validateIdentitySnapshotDatabase

- 级别：参考
- 文件：`modules/identity/internal/snapshot_service.go`
- 作用：校验隔离identity数据库schema、超管和认证代次

```go
func validateIdentitySnapshotDatabase(ctx context.Context, db *sql.DB, sourceEpoch string, sourceSchema int) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 恢复数据库验证上下文 | 沿用外层截止和取消信号 |
| `db` | `*sql.DB` | 隔离staged identity数据库 | 必须为只读或已完成迁移的数据库 |
| `sourceEpoch` | `string` | manifest声明的源数据代次 | 必须为有效数据代次；只绑定session_families.data_epoch |
| `sourceSchema` | `int` | manifest声明的源schema版本 | 必须为已登记且不超过当前迁移集的正数 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 核当前迁移完整
2. 核唯一超管上限
3. 核auth_state自身合法且session_families分别绑定同一auth_epoch与source data_epoch
4. 不把auth_epoch当作data_epoch

**错误**

- 无

- 副作用：无
- 授权：本域SnapshotManager回调或启动可信装配
- 并发：由外层SnapshotManager或恢复控制串行
- 幂等：按外层操作或持久阶段回执
- 依赖：`identity.All`

## identity.OpenDatabase

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：打开本域active区业务库与control区决定日志句柄

```go
func OpenDatabase(dataRoot string) (*Store, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按DSN设置WAL、foreign_keys=ON、busy_timeout=3000ms并打开active/identity.db；单连接串行化
2. 以0600追加模式打开control/identity-journal.jsonl；任一失败关闭已打开句柄
3. 独立打开control/control.db并核control schema=1；不可逆bootstrap claim不随active恢复替换，未知控制schema隔离启动
4. 任何数据目录建立/文件打开前以公共ValidatePrivatePath校验dataRoot绝对目录链，control目录在建立或打开前另核，拒绝链接

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：打开本域active区DB与control区决定日志；迁移在事务中写入
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：单连接串行；决定日志append串行加锁
- 幂等：迁移按已应用记录跳过；恢复按日志末行状态判定
- 依赖：`identity.openIdentityControl`、`platform.ValidatePrivatePath`

## identity.RunMigrations

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：按编号升序在事务中应用未执行迁移

```go
func (s *Store) RunMigrations() error
```

**输入**

无

**输出**

无

**步骤**

1. 读取schema_migrations已应用记录（缺表视为空集，首个迁移自举该表）
2. 逐个在事务中执行迁移SQL并登记版本；已应用版本超出迁移集返回INTEGRITY_ERROR

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：打开本域active区DB与control区决定日志；迁移在事务中写入
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：单连接串行；决定日志append串行加锁
- 幂等：迁移按已应用记录跳过；恢复按日志末行状态判定
- 依赖：`identity.All`

## identity.RecoverJournal

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：恢复决定日志并使未完成决定可见

```go
func (s *Store) RecoverJournal() error
```

**输入**

无

**输出**

无

**步骤**

1. 对identity.account-index未决决定从当前active权威补齐派生索引后完成日志；无账户但已有控制索引拒绝重新开户
2. 核bootstrap控制身份与active唯一超管，reserved且安全回执已提交时单向补记committed；committed缺active超管或身份不一致隔离
3. 其他未知open决定仍INTEGRITY_ERROR失败关闭；全部安全后才设置JournalRecovered，不把未知决定当回滚

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：打开本域active区DB与control区决定日志；迁移在事务中写入
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：单连接串行；决定日志append串行加锁
- 幂等：迁移按已应用记录跳过；恢复按日志末行状态判定
- 依赖：`identity.recoverAccountIndex`、`identity.recoverBootstrapClaim`

## identity.openIdentityControl

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：打开独立control库并初始化或核对受支持schema1

```go
func openIdentityControl(dataRoot string) (*sql.DB, error)
```

**输入**

无

**输出**

无

**步骤**

1. 打开独立control库并初始化或核对受支持schema1
2. sql.Open前以公共ValidatePrivatePath核control目录链、control.db及WAL/SHM正规文件（首次可缺），Ping建立DB后再核正规文件；拒绝链接/特殊或多硬链接，不自建路径校验机制

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：`identity.ControlSchema`、`identity.identityDBError`、`platform.ValidatePrivatePath`

## identity.AppendDecision

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：控制决定open追加并fsync，明文载荷只含安全引用

```go
func (s *Store) AppendDecision(kind string, payload any) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 控制决定open追加并fsync，明文载荷只含安全引用

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.CompleteDecision

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：对应控制决定done追加并fsync，不撤回既有决定

```go
func (s *Store) CompleteDecision(id string) error
```

**输入**

无

**输出**

无

**步骤**

1. 对应控制决定done追加并fsync，不撤回既有决定

**错误**

- 无

- 副作用：无
- 授权：本域受信用例/装配或启动恢复私有助手
- 并发：信任外层事务或本域账号互斥；不作为HTTP独立入口
- 幂等：按外层操作回执或决定水位，纯读取无副作用
- 依赖：无

## identity.readJournal

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：读取本域控制决定日志的有效末行状态

```go
func readJournal(file *os.File) (map[string]journalEntry, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按日志顺序解析决定条目并保留每个决定ID的最后状态；损坏或未知状态失败关闭

**错误**

- 无

- 副作用：无
- 授权：本域启动恢复或SnapshotManager回调
- 并发：由外层文件锁与启动恢复串行
- 幂等：只读确定性
- 依赖：无

## identity.EnsureOutbox

- 级别：参考
- 文件：`modules/identity/internal/storage.go`
- 作用：启动时建立同库outbox与全部identity业务表work gate

```go
func (s *Store) EnsureOutbox(ctx context.Context) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `s` | `*Store` | identity active业务库Store接收者 | 必须为非nil且持有已打开的业务库 |
| `ctx` | `context.Context` | 启动装配上下文 | 沿用启动取消信号 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全失败 | nil表示成功；非nil时业务结果无效 |

**步骤**

1. 调用platform.EnsureSchema；按identityWorkTables逐表调用platform.RegisterWorkTable
2. 不复制公共SQL，不使用MAX/COUNT构造版本

**错误**

- 无

- 副作用：建立本域同库outbox控制表与业务表work gate
- 授权：启动装配
- 并发：单连接串行
- 幂等：注册接口幂等
- 依赖：`platform.EnsureSchema`、`platform.RegisterWorkTable`

## identity.All

- 级别：参考
- 文件：`modules/identity/migrations/migrations.go`
- 作用：按编号升序返回全部内嵌迁移并校验编号连续

```go
func All() ([]Migration, error)
```

**输入**

无

**输出**

无

**步骤**

1. go:embed读取NNNN_name.sql；文件名或编号不合法、编号不连续即报错

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：打开本域active区DB与control区决定日志；迁移在事务中写入
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：单连接串行；决定日志append串行加锁
- 幂等：迁移按已应用记录跳过；恢复按日志末行状态判定
- 依赖：无

## identity.ControlSchema

- 级别：参考
- 文件：`modules/identity/migrations/migrations.go`
- 作用：返回独立control库schema1的内嵌唯一SQL来源

```go
func ControlSchema() (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取control/0001_bootstrap.sql；与active编号迁移隔离，不复制SQL

**错误**

- 无

- 副作用：无
- 授权：身份存储私有装配
- 并发：只读内嵌资源
- 幂等：确定性
- 依赖：无

## identity.main

- 级别：约束
- 文件：`modules/identity/cmd/identity/main.go`
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
4. 支持`accounts`运维子命令：只读调用ReadServerAccountIndex输出账号名与状态后退出，不启动HTTP

**错误**

- `VALIDATION` / HTTP 400：配置未知字段/越界目录/非法端点（不可重试）
- `INTERNAL` / HTTP 500：装配或HTTP运行失败（不可重试）

- 副作用：打开本域资源、监听HTTP；关闭按持久安全点
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：一个进程一个Application；OS信号处理只触发一次drain
- 幂等：进程重启先journal恢复；不重放已付费任务
- 依赖：`identity.BuildApplication`、`identity.Serve`、`identity.Shutdown`、`identity.BackgroundTasks`、`identity.ReadServerAccountIndex`、`platform.RunService`

## identity.BuildApplication

- 级别：约束
- 文件：`modules/identity/internal/app/wire.go`
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
3. 若本次启动换入了恢复数据，先调用identity.InvalidateRestoredSessions撤销全部会话，再开放入口
4. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
5. 读取runtime epoch/maintenance；维护期间只开放健康/就绪与控制管理接口、不启动worker；本次启动完成换目录或换回时调用site-config:registerActivation登记回执；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
6. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源
7. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环
8. 先装配所有已实现认证/账号真实handler与固定caller/静态action，交公共NewServiceHandler核完整OpenAPI；其余未实现handler不建占位，工厂明确拒绝启动；失败关闭store。内部敏感结果仅portal服务可取，浏览器cookie由portal白名单adapter处理
9. 任何数据目录建立/文件打开前以公共ValidatePrivatePath校验dataRoot绝对目录链，control目录在建立或打开前另核，拒绝链接
10. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`identity.restoreIdentityAfterSwap`、`identity.ObserveSuperadminResetFile`、`identity.RelayOutbox`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`identity.OpenDatabase`、`identity.RunMigrations`、`identity.EnsureOutbox`、`identity.RecoverJournal`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`identity.WithSessionRuntime`、`identity.WithLoginRuntime`、`identity.WithRegistrationRuntime`、`platform.CanonicalPublicOrigin`、`platform.NewServiceHandler`、`identity.NewApplication`、`identity.SnapshotManager`、`identity.InstallBackgroundTasks`、`platform.ValidatePrivatePath`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）；本域无独立web/UI；内部Go API继续由identity OpenAPI生成，portal浏览器认证副本由portal OpenAPI x-browser-auth-adapters与Browser*类型合成，禁止将内部token响应类型直接供浏览器

## identity.restoreIdentityAfterSwap

- 级别：参考
- 文件：`modules/identity/internal/app/wire.go`
- 作用：消费公共换目录证据并在换入后撤销恢复快照中的会话

```go
func restoreIdentityAfterSwap(ctx context.Context, dataRoot string, runtime platform.RuntimeView, swap platform.SwapResult, store *identity.Store, key platform.SigningKey, activationClient *gen.SiteConfigClient) (platform.Identifier, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 启动恢复上下文 | 沿用启动取消和截止；不携带敏感正文 |
| `dataRoot` | `string` | 本域私有数据根目录 | 由启动配置提供；仅交公共ReadRestoreSwapState核验 |
| `runtime` | `platform.RuntimeView` | 换目录前读取的site-config运行权威 | 必须包含当前restoreId、targetDataEpoch与维护阶段 |
| `swap` | `platform.SwapResult` | 公共ApplyRestoreSwap的真实动作结果 | 只接受switched或rolled-back；不从目录猜测动作 |
| `store` | `*identity.Store` | 已完成迁移和journal恢复的本域业务库句柄 | 必须为非nil；调用方负责失败时关闭 |
| `key` | `platform.SigningKey` | 本域受控服务签名密钥引用 | 只用于SignActivationReceipt；私钥不进入请求或日志 |
| `activationClient` | `*gen.SiteConfigClient` | site-config RegisterActivation版本化客户端 | 非nil；通过公共服务断言传输 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `openedEpoch` | `platform.Identifier` | 恢复后本域真实OpenedDataEpoch | switched返回evidence.TargetEpoch；rolled-back或none保留控制runtime.DataEpoch |
| `error` | `error` | 恢复证据、会话失效、签名或activation登记错误 | 失败时openedEpoch无效；不返回空成功 |

**步骤**

1. 调用platform.ReadRestoreSwapState核完整stage、目录组合与动作一致
2. switched时用独立NewAuthEpoch调用identity.InvalidateRestoredSessions；rolled-back不改认证代次
3. 用serviceauth.SignActivationReceipt签名现有UTC规范claims，并用运行控制host revision调用RegisterActivation

**错误**

- 无

- 副作用：switched时更新auth_state并撤销恢复数据中的全部session与refresh族
- 授权：启动可信装配；不接受HTTP输入
- 并发：启动恢复互斥；依赖本域事务与持久journal串行
- 幂等：按restoreId+state派生稳定activation operationId；会话失效与site-config登记均可重试
- 依赖：`platform.ReadRestoreSwapState`、`identity.WithIdentityStore`、`identity.WithIdentityRestoreID`、`identity.InvalidateRestoredSessions`、`platform.SignActivationReceipt`、`site-config:registerActivation`

## identity.Serve

- 级别：约束
- 文件：`modules/identity/internal/transport/server.go`
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
- 依赖：`identity.BuildApplication`、`identity.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## identity.Shutdown

- 级别：约束
- 文件：`modules/identity/internal/transport/server.go`
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

## identity.Health

- 级别：约束
- 文件：`modules/identity/internal/transport/server.go`
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

## identity.Ready

- 级别：约束
- 文件：`modules/identity/internal/transport/server.go`
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

## identity.NewApplication

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：装配真实已实现认证组；缺合同handler时公共工厂拒绝整体启动

```go
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, runtimeAuthority platform.SiteConfigClient, store *identity.Store, openedEpoch platform.Identifier, sessionRuntime identity.SessionRuntime) (*Application, error)
```

**输入**

无

**输出**

无

**步骤**

1. 装配真实已实现认证组；缺合同handler时公共工厂拒绝整体启动
2. 维护白名单仅checkAuthorization与verifySessionCSRF控制链只读核权；登录/刷新/账号写不绕过维护屏障
3. 后台入口不在此处伪造；完整启动取得真实SnapshotManager后通过InstallBackgroundTasks统一登记

**错误**

- 无

- 副作用：无
- 授权：固定服务caller与平台用户权威回调；健康就绪匿名
- 并发：只读应用容器，事务由本域用例负责
- 幂等：以业务用例回执为准
- 依赖：`platform.NewServiceHandler`、`identity.NewSnapshotManager`、`identity.operationHandlers`、`identity.authorizeOperation`

## identity.identityContext

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：注入进程受信本域存储和客户端，不重做入口schema或服务验签

```go
func (a *Application) identityContext(ctx context.Context) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 注入进程受信本域存储和客户端，不重做入口schema或服务验签

**错误**

- 无

- 副作用：无
- 授权：固定服务caller与平台用户权威回调；健康就绪匿名
- 并发：只读应用容器，事务由本域用例负责
- 幂等：以业务用例回执为准
- 依赖：`identity.WithSessionRuntime`、`identity.WithLoginRuntime`、`identity.WithRegistrationRuntime`、`identity.WithSessionCursorKey`、`identity.WithIdentityStore`、`identity.WithSnapshotManager`、`identity.WithSnapshotRuntime`、`identity.WithRestoreArchiveClient`

## identity.authorizeOperation

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：以固定静态动作调用本域单次授权，不HTTP递归

```go
func (a *Application) authorizeOperation(ctx context.Context, op OperationContext, permission, action string) error
```

**输入**

无

**输出**

无

**步骤**

1. 以固定静态动作调用本域单次授权，不HTTP递归

**错误**

- 无

- 副作用：无
- 授权：固定服务caller与平台用户权威回调；健康就绪匿名
- 并发：只读应用容器，事务由本域用例负责
- 幂等：以业务用例回执为准
- 依赖：`identity.identityContext`、`identity.CheckAuthorization`

## identity.operationHandlers

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：仅登记真实已完成用例，路径参数由平台入口提供

```go
func (a *Application) operationHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 仅登记真实已完成用例，路径参数由平台入口提供
2. 本轮绑定四个账号管理真用例；列表只从已验证query组装page，三个写入accountId固定取平台PathValue，正文不允许覆盖路径身份
3. 偏好GET/PUT绑定真实本人用例，复用公共入口的portal.use与严格schema，不从浏览器传subject/site键

**错误**

- 无

- 副作用：无
- 授权：固定服务caller与平台用户权威回调；健康就绪匿名
- 并发：只读应用容器，事务由本域用例负责
- 幂等：以业务用例回执为准
- 依赖：`identity.jsonOperation`、`identity.ReadRegistrationState`、`identity.RegisterFirstSuperadmin`、`identity.CreateSession`、`identity.RefreshSession`、`identity.CreateAccount`、`identity.ReadSelf`、`identity.ChangePassword`、`identity.VerifySessionCSRF`、`identity.CheckAuthorization`、`identity.ReadSessions`、`identity.RevokeSession`、`identity.Health`、`identity.Ready`、`identity.ListAccounts`、`identity.SetAccountStatus`、`identity.SetAccountPermissions`、`identity.ResetAccountPassword`、`identity.ReadPreferences`、`identity.UpdatePreferences`、`identity.AcquireBarrier`、`identity.ReleaseBarrier`、`identity.ExportSnapshot`、`identity.StageSnapshot`、`identity.DiscardSnapshot`、`identity.ReadSnapshotArchive`

## identity.SnapshotManager

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：返回BuildApplication已装配的同一SnapshotManager

```go
func (a *Application) SnapshotManager() *lifecycle.SnapshotManager
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `a` | `*Application` | identity应用容器接收者 | 必须为已成功装配的Application |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `manager` | `*lifecycle.SnapshotManager` | Application持有的同一公共快照管理器 | 与后台任务和HTTP上下文共享同一指针；不创建新实例 |

**步骤**

1. 只返回进程内manager句柄

**错误**

- 无

- 副作用：无
- 授权：启动装配
- 并发：只读应用容器
- 幂等：确定性
- 依赖：无

## identity.BackgroundTasks

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：向统一进程模板暴露本域后台入口

```go
func (a *Application) BackgroundTasks() []platform.BackgroundTask
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `a` | `*Application` | identity应用容器接收者 | 已装配后台任务列表 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tasks` | `[]platform.BackgroundTask` | 后台任务快照 | 返回副本；调用方不能修改Application内部列表 |

**步骤**

1. 复制已装配任务切片并返回

**错误**

- 无

- 副作用：无
- 授权：启动生命周期内部
- 并发：只读容器
- 幂等：确定性
- 依赖：无

## identity.InstallBackgroundTasks

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：在完整启动阶段一次性登记真实后台任务

```go
func (a *Application) InstallBackgroundTasks(siteConfig platform.SiteConfigClient, snapshot *lifecycle.SnapshotManager, publisher platform.EventPublisher) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `a` | `*Application` | identity应用容器接收者 | 已由NewApplication装配 |
| `siteConfig` | `platform.SiteConfigClient` | 运行权威客户端 | 非nil真实客户端 |
| `snapshot` | `*lifecycle.SnapshotManager` | 本域唯一SnapshotManager | 必须与Application内指针相同 |
| `publisher` | `platform.EventPublisher` | jobs发布端口 | 非nil版本化客户端 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 后台依赖或重复装配错误 | 缺依赖不登记任务；重复调用失败 |

**步骤**

1. 核对site-config、Manager、publisher和openedEpoch；调用identity.NewBackgroundTasks；拒绝重复登记

**错误**

- 无

- 副作用：更新Application内部后台任务列表
- 授权：仅完整启动装配
- 并发：单次装配串行
- 幂等：重复调用返回CONFLICT
- 依赖：`identity.NewBackgroundTasks`

## identity.jsonOperation

- 级别：参考
- 文件：`modules/identity/internal/transport/server.go`
- 作用：解码平台已验证输入并返回受信服务结果，敏感结果禁止浏览器通用透传

```go
func jsonOperation[I any, O any](a *Application, execute operationExecutor[I, O], bindInput ...func(*I, *http.Request)) http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 解码平台已验证输入并返回受信服务结果，敏感结果禁止浏览器通用透传
2. 静态可选binder在已校验正文解码后填入平台验证的路径字段；只有装配代码能传binder，不接受请求自报绑定策略

**错误**

- 无

- 副作用：无
- 授权：固定服务caller与平台用户权威回调；健康就绪匿名
- 并发：只读应用容器，事务由本域用例负责
- 幂等：以业务用例回执为准
- 依赖：`identity.identityContext`

## identity.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/identity/internal/snapshot_archive.go`
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
- 依赖：`identity.validSuperadminDelegation`、`identity.snapshotRuntimeFromContext`、`identity.snapshotManagerFromContext`、`platform.SnapshotManager.OpenArchive`

## identity.validSuperadminDelegation

- 级别：参考
- 文件：`modules/identity/internal/snapshot_archive.go`
- 作用：核data-protection调用链携带的超管短委托

```go
func validSuperadminDelegation(op OperationContext) bool
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `op` | `OperationContext` | 平台入口建立的受信操作上下文 | Actor必须为已验证admin；SubjectID、SiteID、SessionID均为有效标识且含superadmin grant |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `valid` | `bool` | 超管短委托是否完整有效 | 仅true时允许data-protection受控恢复或屏障动作 |

**步骤**

1. 要求admin主体、站点、session和superadmin grant完整

**错误**

- 无

- 副作用：无
- 授权：data-protection服务调用链
- 并发：纯校验
- 幂等：确定性
- 依赖：`identity.hasGrant`

## identity.AccountsPage

- 级别：约束
- 文件：`modules/identity/web/src/index.tsx`
- 作用：账号与权限：子账号的新建、功能权限、停用/恢复/删除、重置口令

```ts
export default function AccountsPage(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 列出账号，超管以外的账号可操作
2. 危险操作先确认；口令只在提交时出现

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无
