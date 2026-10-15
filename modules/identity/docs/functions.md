# identity 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 57 个函数：约束 29 个，参考 28 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：anonymous
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`site-config:readInitializationStatus`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. Origin与登录准入检查，但不要求初始化码、邮件或MFA
2. 确认两次口令一致，站点注册唯一锁下核对从无账户；HashPassword
3. 同事务生成siteId和不可逆bootstrapClaim、唯一superadmin账户及session；竞争失败CONFLICT转登录，之后永不重开注册

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `RATE_LIMITED`：频率、并发或容量上限已达，返回 retryAfter（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：anonymous；无需初始化码，部署后立即注册或先不暴露端口
- 并发：bootstrapClaim+isSuperadmin唯一约束同事务，首注册至多一个
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.HashPassword`、`identity.IssueSessionTokens`、`traffic-guard:createAdmission`、`identity.ResolveIdentityOperation`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅superadmin，规范不可变账号名与唯一键
2. 权限省略时默认全部10项；不能赋予superadmin-only技术权限
3. HashPassword，同事务建active、mustChangePassword=true，写账户索引与审计

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：账户管理仅唯一superadmin；自身操作account.self；权限按功能全站，不按文档scope
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.NormalizeAccountName`、`identity.HashPassword`、`identity.MapFunctionGrants`、`identity.WriteAccountIndex`、`identity.ResolveIdentityOperation`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；NOT_FOUND: 资源不存在或在当前主体作用域中不可见 |

**步骤**

1. 使用proof主体与site
2. 返回可见metadata
3. 删除/停用账户UNAUTHENTICATED；不返hash

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）

- 副作用：无
- 授权：account.self；强改会话仅允许必要资料
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 核对唯一superadmin
2. 按accountName/id稳定分页
3. 保留deleted署名，不返凭据

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：superadmin
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：签名cursor一致水位
- 依赖：无

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. AssertSuperadminImmutable拒绝对唯一超管停用或删除
2. 检查目标revision，deleted不可复活为同账户新实体
3. 事务改状态+permissionRevision，立即RevokeAllSessions，删除仅软墓碑保留署名

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.AssertSuperadminImmutable`、`identity.RevokeAllSessions`、`identity.WriteAccountIndex`、`identity.ResolveIdentityOperation`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅超管；禁止修改isSuperadmin/accountName
2. MapFunctionGrants不接收文档scope/继承角色
3. CAS替换flags并增permissionRevision；受保护请求同步查新版本

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.MapFunctionGrants`、`identity.PublishIdentityEvent`、`identity.ResolveIdentityOperation`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅superadmin，超管自身忘密走服务器恢复
2. HashPassword，事务写新凭据与mustChangePassword=true
3. 立即使全部session失效；下次只能改密

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.HashPassword`、`identity.RevokeAllSessions`、`identity.ResolveIdentityOperation`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. VerifyPassword与ValidatePasswordChange，强改会话允许此唯一业务写入
2. HashPassword后CAS凭据，清mustChangePassword
3. 同事务撤销其他设备全部session，当前session升credential版本并重新发cookie；保持当前设备可进入后台

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：account.self；mustChangePassword会话也可调用
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.VerifyPassword`、`identity.ValidatePasswordChange`、`identity.HashPassword`、`identity.RevokeOtherSessions`、`identity.IssueSessionTokens`、`identity.ResolveIdentityOperation`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. traffic-guard按账号/IP失败保护准入；账号不存在/口令错误统一UNAUTHENTICATED
2. VerifyPassword，active才可session，必要时安全rehash
3. 初始口令session restrictedToPasswordChange=true，仅返回改密路由；否则查询初始化状态后向导/后台

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `RATE_LIMITED`：频率、并发或容量上限已达，返回 retryAfter（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：anonymous + 同Origin；只显示账号名和口令，不新增MFA/邮箱
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.VerifyPassword`、`identity.IssueSessionTokens`、`traffic-guard:createAdmission`、`site-config:readInitializationStatus`

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
- 依赖：`identity.IssueSessionTokens`、`identity.RevokeSessionFamily`、`identity.MapFunctionGrants`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`identity.sessionAuthContext`

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
- 依赖：`identity.WithSessionCursorKey`、`platform.SignCursor`、`platform.VerifyCursor`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`
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
- 依赖：`identity.RevokeSessionFamily`、`identity.PublishIdentityEvent`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`

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
3. 在同一事务登记identity.authorization.changed；disabled/deleted映射status、admin_reset映射password，server_reset/restore保持原值；授权读取仍须同步检查sessions.revokedAt与revision
4. 自有事务的幂等键绑定site/actor/operation、callerService、动作、dataEpoch与请求摘要；完整计数结果及哈希写入operation_receipts.result_json，重放返回原计数且不撤销后来建立的会话；嵌入事务由外层用例统一回执与提交

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `CONFLICT`：同一operationId绑定的调用方、代次或请求摘要不同（不可重试）
- `INTEGRITY_ERROR`：完整结果回执缺失、哈希不一致或撤销数量超出合同（不可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：superadmin或身份私有状态迁移
- 并发：SQLite单连接串行化；更新、outbox和回执同一事务；外层事务由其调用方负责提交或回滚
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.PublishIdentityEvent`
- 其他依赖说明：内嵌调用方须在开启事务前确保平台outbox schema已建立；会话家族撤销/签发字段仍待后续工作包

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
- 依赖：`identity.sessionRuntimeFromContext`、`identity.verifySessionAccess`、`platform.VerifySessionAccess`

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

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：本域会话权威、期限、权限登记或完整结果回执损坏（不可重试）

- 副作用：无
- 授权：service，主体必须来自可信委托
- 并发：权限一致快照，不能缓存长期允许
- 幂等：只读
- 依赖：`identity.MapFunctionGrants`、`identity.sessionRuntimeFromContext`、`identity.verifySessionAccess`、`identity.readSessionSnapshot`、`identity.validateSessionSnapshot`、`identity.sessionAuthContext`、`platform.VerifySessionAccess`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 按当前subjectId/siteId
2. 无记录明确default system/zh-Hans/dashboard/revision0
3. 不授权任何业务操作

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：portal.use
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 检查允许界面locale/route
2. 默认页功能必须仍被授权
3. CAS私有记录；其他主体键不允许输入

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务、receipt和撤销outbox
- 授权：portal.use
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`identity.CheckAuthorization`

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

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有可读索引文件
- 授权：私有账号提交/恢复
- 并发：索引revision单调，低版本不覆盖
- 幂等：同revision同hash不重复写
- 依赖：无

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
- 依赖：`identity.ApplySuperadminReset`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 重新核对固定文件hash，定位唯一superadmin
2. HashPassword，事务更新凭据/mustChangePassword=true并撤销全部session
3. 提交回执后原子删除仅该固定file；删除失败下次读同hash回执只补删，不重复重置

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：凭据写入和提交后固定reset文件删除
- 授权：本机OS控制身份；不需要已失效后台token
- 并发：超管credential行锁+fileHashreceipt
- 幂等：fileHash作为operationId；删除失败重复仅补清理
- 依赖：`identity.HashPassword`、`identity.RevokeAllSessions`

## identity.ResolveIdentityOperation

- 级别：参考
- 文件：`modules/identity/internal/operation_repository.go`
- 作用：读取幂等回执

```go
func ResolveIdentityOperation(ctx context.Context, op OperationContext, input ResolveIdentityOperationInput) (ResolveIdentityOperationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ResolveIdentityOperationInput` | 读取幂等回执的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveIdentityOperationResult` | 读取幂等回执的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 查询site/actor/operation唯一键
2. 异摘要CONFLICT，敏感input仅salted digest不存明文
3. 已提交返回安全结果hash供上层重读

**错误**

- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：私有事务
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
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
- 依赖：`platform.EnqueueEvent`

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
| `result` | `RelayOutboxResult` | 本批转发计数 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 进程启动完成journal恢复后立即运行一次，之后每次本域提交通知或1秒轮询运行；按outbox序号以30秒行租约领取≤batchLimit条未转发行，只读已提交事务
2. 逐条调用jobs:publishEvent，携带固定eventId、payloadHash、dataEpoch与subject revision；jobs持久确认后在本地事务把该行标relayed并记录jobs eventSeq
3. 网络失败或超时保留该行，按1/2/4秒指数退避（上限60秒）在下一轮重试；同eventId异hash或信封校验失败标quarantined并报警，不再重试也不删除
4. 屏障draining期间继续转发已提交行直至为空或屏障超时；快照包含未转发行；恢复换入新数据后丢弃旧dataEpoch未转发行，以恢复后的权威状态为准

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用或超时；行保留待下一轮（可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash或信封不合法；行隔离（不可重试）
- `VERSION_MISMATCH`：dataEpoch与当前active epoch不一致；旧代行按恢复规则丢弃（不可重试）

- 副作用：读写本域outbox行状态与租约；HTTP调用jobs:publishEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个转发循环；行级租约防多实例重复领取；同subject按outbox序号串行发送
- 幂等：eventId在业务事务内固定；jobs按eventId+payloadHash去重，重复发送返回原确认
- 依赖：`jobs:publishEvent`、`platform.RunRelay`

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
- 依赖：`site-config:readRuntimeState`

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
- 依赖：`site-config:readRuntimeState`

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
- 依赖：`site-config:readRuntimeState`

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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`

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
- 依赖：`site-config:readRuntimeState`

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

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：打开本域active区DB与control区决定日志；迁移在事务中写入
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：单连接串行；决定日志append串行加锁
- 幂等：迁移按已应用记录跳过；恢复按日志末行状态判定
- 依赖：无

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

1. 逐行读取决定日志，同一id以最后一行状态生效
2. 存在open状态条目返回INTEGRITY_ERROR隔离启动；全部完结则标记JournalRecovered

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：打开本域active区DB与control区决定日志；迁移在事务中写入
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：单连接串行；决定日志append串行加锁
- 幂等：迁移按已应用记录跳过；恢复按日志末行状态判定
- 依赖：无

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
- 依赖：`identity.BuildApplication`、`identity.Serve`、`identity.Shutdown`、`identity.ReadServerAccountIndex`、`platform.RunService`

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

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`identity.InvalidateRestoredSessions`、`identity.ObserveSuperadminResetFile`、`identity.RelayOutbox`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`identity.OpenDatabase`、`identity.RunMigrations`、`identity.RecoverJournal`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`identity.WithSessionRuntime`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

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
