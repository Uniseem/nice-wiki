# 函数填空合同

每个条目与 [implementation.json](../contracts/implementation.json) 一一对应。签名中的领域类型见 types.json；标准 Problem 描述返回 error。未来新增任何私有助手也必须先登记契约。

## identity.ReadRegistrationState

文件：`modules/identity/internal/registration_service.go`。

`func ReadRegistrationState(ctx RequestContext, input ReadRegistrationStateInput) (ReadRegistrationStateResult, error)`

决定后台显示首次注册或登录。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadRegistrationStateInput。决定后台显示首次注册或登录的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadRegistrationStateResult。决定后台显示首次注册或登录的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 读不可逆bootstrapClaim与唯一superadmin索引。
2. 首次无账户返回registrationOpen=true，不要求初始化码。
3. site-config状态只决定登录后的向导，不重开注册。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：anonymous。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：site-config:readInitializationStatus。

## identity.RegisterFirstSuperadmin

文件：`modules/identity/internal/registration_service.go`。

`func RegisterFirstSuperadmin(ctx RequestContext, input RegisterFirstSuperadminInput) (RegisterFirstSuperadminResult, error)`

首个注册账户原子成为唯一超管。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RegisterFirstSuperadminInput。首个注册账户原子成为唯一超管的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RegisterFirstSuperadminResult。首个注册账户原子成为唯一超管的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. Origin与登录准入检查，但不要求初始化码、邮件或MFA。
2. 确认两次口令一致，站点注册唯一锁下核对从无账户；HashPassword。
3. 同事务生成siteId和不可逆bootstrapClaim、唯一superadmin账户及session；竞争失败CONFLICT转登录，之后永不重开注册。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；RATE_LIMITED：频率、并发或容量上限已达，返回 retryAfter；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：anonymous；无需初始化码，部署后立即注册或先不暴露端口。

**并发**：bootstrapClaim+isSuperadmin唯一约束同事务，首注册至多一个。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.HashPassword；identity.IssueSessionTokens；traffic-guard:createAdmission。

## identity.CreateAccount

文件：`modules/identity/internal/account_service.go`。

`func CreateAccount(ctx RequestContext, input CreateAccountInput) (CreateAccountResult, error)`

超管创建子账号并标记强改。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CreateAccountInput。超管创建子账号并标记强改的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CreateAccountResult。超管创建子账号并标记强改的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 仅superadmin，规范不可变账号名与唯一键。
2. 权限省略时默认全部10项；不能赋予superadmin-only技术权限。
3. HashPassword，同事务建active、mustChangePassword=true，写账户索引与审计。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：账户管理仅唯一superadmin；自身操作account.self；权限按功能全站，不按文档scope。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.NormalizeAccountName；identity.HashPassword；identity.MapFunctionGrants；identity.WriteAccountIndex。

## identity.ReadSelf

文件：`modules/identity/internal/account_service.go`。

`func ReadSelf(ctx RequestContext, input ReadSelfInput) (ReadSelfResult, error)`

读取当前账户。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadSelfInput。读取当前账户的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadSelfResult。读取当前账户的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；NOT_FOUND: 资源不存在或在当前主体作用域中不可见。

**步骤**

1. 使用proof主体与site。
2. 返回可见metadata。
3. 删除/停用账户UNAUTHENTICATED；不返hash。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false。

**副作用**：无。

**授权**：account.self；强改会话仅允许必要资料。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## identity.ListAccounts

文件：`modules/identity/internal/account_service.go`。

`func ListAccounts(ctx RequestContext, input ListAccountsInput) (ListAccountsResult, error)`

超管读取子账号列表。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ListAccountsInput。超管读取子账号列表的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ListAccountsResult。超管读取子账号列表的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 核对唯一superadmin。
2. 按accountName/id稳定分页。
3. 保留deleted署名，不返凭据。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：superadmin。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：签名cursor一致水位。

**依赖**：无跨函数/跨域依赖。

## identity.SetAccountStatus

文件：`modules/identity/internal/account_service.go`。

`func SetAccountStatus(ctx RequestContext, input SetAccountStatusInput) (SetAccountStatusResult, error)`

停用/恢复/删除子账号。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SetAccountStatusInput。停用/恢复/删除子账号的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SetAccountStatusResult。停用/恢复/删除子账号的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. AssertSuperadminImmutable拒绝对唯一超管停用或删除。
2. 检查目标revision，deleted不可复活为同账户新实体。
3. 事务改状态+permissionRevision，立即RevokeAllSessions，删除仅软墓碑保留署名。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：superadmin。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.AssertSuperadminImmutable；identity.RevokeAllSessions；identity.WriteAccountIndex。

## identity.SetAccountPermissions

文件：`modules/identity/internal/account_service.go`。

`func SetAccountPermissions(ctx RequestContext, input SetAccountPermissionsInput) (SetAccountPermissionsResult, error)`

修改10项功能权限。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SetAccountPermissionsInput。修改10项功能权限的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SetAccountPermissionsResult。修改10项功能权限的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 仅超管；禁止修改isSuperadmin/accountName。
2. MapFunctionGrants不接收文档scope/继承角色。
3. CAS替换flags并增permissionRevision；受保护请求同步查新版本。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：superadmin。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.MapFunctionGrants；identity.PublishIdentityEvent。

## identity.ResetAccountPassword

文件：`modules/identity/internal/account_service.go`。

`func ResetAccountPassword(ctx RequestContext, input ResetAccountPasswordInput) (ResetAccountPasswordResult, error)`

超管重置子账号初始口令。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ResetAccountPasswordInput。超管重置子账号初始口令的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ResetAccountPasswordResult。超管重置子账号初始口令的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 仅superadmin，超管自身忘密走服务器恢复。
2. HashPassword，事务写新凭据与mustChangePassword=true。
3. 立即使全部session失效；下次只能改密。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：superadmin。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.HashPassword；identity.RevokeAllSessions。

## identity.NormalizeAccountName

文件：`modules/identity/internal/account_validator.go`。

`func NormalizeAccountName(input NormalizeAccountNameInput) (NormalizeAccountNameResult, error)`

规范不可变账号名。

**输入**

- input：NormalizeAccountNameInput。规范不可变账号名的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：NormalizeAccountNameResult。规范不可变账号名的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. NFC并拒绝控制字符与首尾空白。
2. 检查字符/UTF-8长度。
3. 作为站点唯一key，不按email或casefold语义改写。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## identity.AssertSuperadminImmutable

文件：`modules/identity/internal/account_validator.go`。

`func AssertSuperadminImmutable(input AssertSuperadminImmutableInput) (AssertSuperadminImmutableResult, error)`

保护唯一超管永久有效。

**输入**

- input：AssertSuperadminImmutableInput。保护唯一超管永久有效的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AssertSuperadminImmutableResult。保护唯一超管永久有效的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配。

**步骤**

1. isSuperadmin=true只允许active且不可删除/改身份。
2. 子账号可以按状态机迁移。
3. 不允许通过permissions字段模拟superadmin。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## identity.MapFunctionGrants

文件：`modules/identity/internal/permission_mapping.go`。

`func MapFunctionGrants(input MapFunctionGrantsInput) (MapFunctionGrantsResult, error)`

将已确认功能勾选映射成服务权限。

**输入**

- input：MapFunctionGrantsInput。将已确认功能勾选映射成服务权限的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：MapFunctionGrantsResult。将已确认功能勾选映射成服务权限的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 严格读取根contracts/permission-groups.json固定10组映射：默认10项全选、authenticatedBaseGrants固定portal.use/account.self/site.read。
2. manage_translations→localization.manage，与manage_languages→localization.languages分开；edit包含preview/upload/read，publish包含publish/history/trash；子账号不得获得superadminOnly任何grant。
3. job.manage不是自由功能开关：仅服务委托且owner业务对象权限已通过时派生，禁止越域/系统任务；resourceId只审计不创建文档scope；唯一superadmin按账户标识获取全部登记管理权限。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## identity.HashPassword

文件：`modules/identity/internal/credential_service.go`。

`func HashPassword(ctx RequestContext, input HashPasswordInput) (HashPasswordResult, error)`

生成Argon2id口令哈希。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：HashPasswordInput。生成Argon2id口令哈希的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：HashPasswordResult。生成Argon2id口令哈希的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter。

**步骤**

1. 至少8位且不强制字符组合，不trim。
2. CSPRNG16字节salt，Argon2idv19 memory65536KiB/time3/parallel4/hash32字节。
3. 编码成本与hash，敏感缓冲短期清理，随机源失败不降级。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；RATE_LIMITED：频率、并发或容量上限已达，返回 retryAfter；retryable=false。

**副作用**：随机源/敏感内存。

**授权**：私有credential流程。

**并发**：密码计算信号量限并发。

**幂等**：每次新salt；外层operation receipt禁止重复业务哈希。

**依赖**：无跨函数/跨域依赖。

## identity.VerifyPassword

文件：`modules/identity/internal/credential_service.go`。

`func VerifyPassword(input VerifyPasswordInput) (VerifyPasswordResult, error)`

核对口令。

**输入**

- input：VerifyPasswordInput。核对口令的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：VerifyPasswordResult。核对口令的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 检查算法编码成本不超部署上限。
2. 派生并常量时间比对；不存在账号走dummy hash相同工作量。
3. 返回rehash标志，禁记原值。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：私有登录流程。

**并发**：密码信号量。

**幂等**：确定性。

**依赖**：无跨函数/跨域依赖。

## identity.ValidatePasswordChange

文件：`modules/identity/internal/credential_service.go`。

`func ValidatePasswordChange(input ValidatePasswordChangeInput) (ValidatePasswordChangeResult, error)`

验证改密三输入规则。

**输入**

- input：ValidatePasswordChangeInput。验证改密三输入规则的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidatePasswordChangeResult。验证改密三输入规则的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 新口令至少8位且两次完全相同。
2. 常量时间拒绝新旧相同。
3. 不新增组合、邮件或MFA前置条件。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## identity.ChangePassword

文件：`modules/identity/internal/credential_service.go`。

`func ChangePassword(ctx RequestContext, input ChangePasswordInput) (ChangePasswordResult, error)`

改口令并结束其他设备登录。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ChangePasswordInput。改口令并结束其他设备登录的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ChangePasswordResult。改口令并结束其他设备登录的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. VerifyPassword与ValidatePasswordChange，强改会话允许此唯一业务写入。
2. HashPassword后CAS凭据，清mustChangePassword。
3. 同事务撤销其他设备全部session，当前session升credential版本并重新发cookie；保持当前设备可进入后台。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：account.self；mustChangePassword会话也可调用。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.VerifyPassword；identity.ValidatePasswordChange；identity.HashPassword；identity.RevokeOtherSessions；identity.IssueSessionTokens。

## identity.CreateSession

文件：`modules/identity/internal/session_service.go`。

`func CreateSession(ctx RequestContext, input CreateSessionInput) (CreateSessionResult, error)`

账号名口令登录。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CreateSessionInput。账号名口令登录的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CreateSessionResult。账号名口令登录的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；RATE_LIMITED: 频率、并发或容量上限已达，返回 retryAfter；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. traffic-guard按账号/IP失败保护准入；账号不存在/口令错误统一UNAUTHENTICATED。
2. VerifyPassword，active才可session，必要时安全rehash。
3. 初始口令session restrictedToPasswordChange=true，仅返回改密路由；否则查询初始化状态后向导/后台。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；RATE_LIMITED：频率、并发或容量上限已达，返回 retryAfter；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：anonymous + 同Origin；只显示账号名和口令，不新增MFA/邮箱。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.VerifyPassword；identity.IssueSessionTokens；traffic-guard:createAdmission；site-config:readInitializationStatus。

## identity.IssueSessionTokens

文件：`modules/identity/internal/session_service.go`。

`func IssueSessionTokens(ctx RequestContext, input IssueSessionTokensInput) (IssueSessionTokensResult, error)`

签发会话和CSRFcookie数据。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：IssueSessionTokensInput。签发会话和CSRFcookie数据的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：IssueSessionTokensResult。签发会话和CSRFcookie数据的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 生成独立access/refresh/CSRFsecret，只存digest。
2. 签名绑定session/credential/permission/auth epoch。
3. 返回cookieadapter敏感值，不保存明文secret响应。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：CSPRNG与签名。

**授权**：identity私有sessiontxn。

**并发**：固定key版本。

**幂等**：私有单次生成；业务外层复用session回执但不重复泄露旧refresh。

**依赖**：无跨函数/跨域依赖。

## identity.RefreshSession

文件：`modules/identity/internal/session_service.go`。

`func RefreshSession(ctx RequestContext, input RefreshSessionInput) (RefreshSessionResult, error)`

内部透明轮换session。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RefreshSessionInput。内部透明轮换session的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RefreshSessionResult。内部透明轮换session的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 锁家族digest检查期限/账户active/credentialepoch。
2. 单次消费refresh，旧值复用撤销族。
3. 事务轮换与最新permissionRevision，保留mustChangePassword限制。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：refresh cookie + SameSite=Strict + Origin。

**并发**：家族行互斥。

**幂等**：单次refresh；并发旧值复用保护，浏览器singleflight。

**依赖**：identity.IssueSessionTokens；identity.RevokeSessionFamily。

## identity.ReadSessions

文件：`modules/identity/internal/session_service.go`。

`func ReadSessions(ctx RequestContext, input ReadSessionsInput) (ReadSessionsResult, error)`

读取自身登录设备。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadSessionsInput。读取自身登录设备的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadSessionsResult。读取自身登录设备的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 主体限制为自己。
2. 排序issuedAt/sessionId。
3. 不返token/IP。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：account.self。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## identity.RevokeSession

文件：`modules/identity/internal/session_revocation.go`。

`func RevokeSession(ctx RequestContext, input RevokeSessionInput) (RevokeSessionResult, error)`

退出当前或指定自身登录。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RevokeSessionInput。退出当前或指定自身登录的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RevokeSessionResult。退出当前或指定自身登录的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 验证session归属。
2. 事务revokedAt与族失效水位。
3. 清cookieadapter状态，发安全事件。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：account.self。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## identity.RevokeSessionFamily

文件：`modules/identity/internal/session_revocation.go`。

`func RevokeSessionFamily(ctx RequestContext, input RevokeSessionFamilyInput) (RevokeSessionFamilyResult, error)`

使refresh族全部失效。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RevokeSessionFamilyInput。使refresh族全部失效的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RevokeSessionFamilyResult。使refresh族全部失效的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 私有锁族。
2. 将revoked与authEpoch提升。
3. 同步撤销session并outbox。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：私有身份流程。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：已撤销返回既有count。

**依赖**：无跨函数/跨域依赖。

## identity.RevokeAllSessions

文件：`modules/identity/internal/session_revocation.go`。

`func RevokeAllSessions(ctx RequestContext, input RevokeAllSessionsInput) (RevokeAllSessionsResult, error)`

立即失效账户全部登录。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RevokeAllSessionsInput。立即失效账户全部登录的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RevokeAllSessionsResult。立即失效账户全部登录的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 内部状态事务或superadmin核对。
2. 按accountId锁全部族。
3. 撤销并提高失效revision，授权读取不能继续接受旧proof。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：superadmin或身份私有状态迁移。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## identity.RevokeOtherSessions

文件：`modules/identity/internal/session_revocation.go`。

`func RevokeOtherSessions(ctx RequestContext, input RevokeOtherSessionsInput) (RevokeOtherSessionsResult, error)`

改密后只保留当前设备。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RevokeOtherSessionsInput。改密后只保留当前设备的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RevokeOtherSessionsResult。改密后只保留当前设备的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核对keepSession归属与有效性。
2. 撤销所有其他族/session。
3. 当前session CAS到新credentialRevision，由IssueSessionTokens轮换。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：私有改密txn。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## identity.VerifySessionCSRF

文件：`modules/identity/internal/csrf_service.go`。

`func VerifySessionCSRF(ctx RequestContext, input VerifySessionCSRFInput) (VerifySessionCSRFResult, error)`

portal内部核对CSRF。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：VerifySessionCSRFInput。portal内部核对CSRF的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：VerifySessionCSRFResult。portal内部核对CSRF的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 只接受portal audience。
2. 检查session active、origin与Strictcookie上下文。
3. 常量時間比对CSRF摘要，失败关闭。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：service:portal。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## identity.CheckAuthorization

文件：`modules/identity/internal/authorization_service.go`。

`func CheckAuthorization(ctx RequestContext, input AuthorizationRequest) (AuthorizationDecision, error)`

按最新账号功能权限授权。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：AuthorizationRequest。按最新账号功能权限授权的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AuthorizationDecision。按最新账号功能权限授权的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 验caller/audience和subject/session，确认siteId；有专用恢复许可仅进入限定snapshot控制分支。
2. 普通请求读当前active账户与permissionRevision；初始口令仅改密/登出/必要自身资料。
3. MapFunctionGrants核对功能权限，superadmin-only单独检查；resourceId仅审计不设文档scope。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：service，主体必须来自可信委托；恢复许可只覆盖声明control action。

**并发**：权限一致快照，不能缓存长期允许。

**幂等**：只读。

**依赖**：identity.MapFunctionGrants；identity.ValidateRecoveryAuthorization。

## identity.ReadPreferences

文件：`modules/identity/internal/preference_service.go`。

`func ReadPreferences(ctx RequestContext, input ReadPreferencesInput) (ReadPreferencesResult, error)`

读自身后台偏好。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadPreferencesInput。读自身后台偏好的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadPreferencesResult。读自身后台偏好的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 按当前subjectId/siteId。
2. 无记录明确default system/zh-Hans/dashboard/revision0。
3. 不授权任何业务操作。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：portal.use。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## identity.UpdatePreferences

文件：`modules/identity/internal/preference_service.go`。

`func UpdatePreferences(ctx RequestContext, input UpdatePreferencesInput) (UpdatePreferencesResult, error)`

保存自身主题/语言/默认后台页。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：UpdatePreferencesInput。保存自身主题/语言/默认后台页的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：UpdatePreferencesResult。保存自身主题/语言/默认后台页的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 检查允许界面locale/route。
2. 默认页功能必须仍被授权。
3. CAS私有记录；其他主体键不允许输入。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：portal.use。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：identity.CheckAuthorization。

## identity.ReadServerAccountIndex

文件：`modules/identity/internal/server_recovery.go`。

`func ReadServerAccountIndex(ctx RequestContext, input ReadServerAccountIndexInput) (ReadServerAccountIndexResult, error)`

服务器运维查明文账号索引。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadServerAccountIndexInput。服务器运维查明文账号索引的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadServerAccountIndexResult。服务器运维查明文账号索引的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 仅本机控制身份读取固定私有control/account-index.json。
2. 核对索引revision与权威账号manifest。
3. 发现滞后由WriteAccountIndex重建，不输出密码hash。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：本机OS文件权限；API仅superadmin受控内部调用。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：identity.WriteAccountIndex。

## identity.WriteAccountIndex

文件：`modules/identity/internal/server_recovery.go`。

`func WriteAccountIndex(ctx RequestContext, input WriteAccountIndexInput) (WriteAccountIndexResult, error)`

维持服务器查账号能力。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：WriteAccountIndexInput。维持服务器查账号能力的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：WriteAccountIndexResult。维持服务器查账号能力的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 抽取accountName/isSuperadmin/status，不复制credential。
2. 固定control路径写临时JSON、fsync、rename并fsync目录。
3. 索引不进git、不恢复覆盖controltrust；失败记录重建待办。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有可读索引文件。

**授权**：私有账号提交/恢复。

**并发**：索引revision单调，低版本不覆盖。

**幂等**：同revision同hash不重复写。

**依赖**：无跨函数/跨域依赖。

## identity.ObserveSuperadminResetFile

文件：`modules/identity/internal/server_recovery.go`。

`func ObserveSuperadminResetFile(ctx RequestContext, input ObserveSuperadminResetFileInput) (ObserveSuperadminResetFileResult, error)`

检测服务器超管重置文件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ObserveSuperadminResetFileInput。检测服务器超管重置文件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ObserveSuperadminResetFileResult。检测服务器超管重置文件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 只访问identity私有control/reset-superadmin.txt固定路径，拒绝symlink/目录/权限异常。
2. 有界UTF8读取并按8位Password校验，生成fileHash作operationId。
3. 输入仅内存，不读任何部署凭据文件。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：读取固定重置请求文件。

**授权**：本机OS控制身份。

**并发**：单watcher，不并发消费同文件。

**幂等**：同fileHash同candidate。

**依赖**：无跨函数/跨域依赖。

## identity.ApplySuperadminReset

文件：`modules/identity/internal/server_recovery.go`。

`func ApplySuperadminReset(ctx RequestContext, input ApplySuperadminResetInput) (ApplySuperadminResetResult, error)`

用服务器文件恢复唯一超管口令。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ApplySuperadminResetInput。用服务器文件恢复唯一超管口令的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ApplySuperadminResetResult。用服务器文件恢复唯一超管口令的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 重新核对固定文件hash，定位唯一superadmin。
2. HashPassword，事务更新凭据/mustChangePassword=true并撤销全部session。
3. 提交回执后原子删除仅该固定file；删除失败下次读同hash回执只补删，不重复重置。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：凭据写入和提交后固定reset文件删除。

**授权**：本机OS控制身份；不需要已失效后台token。

**并发**：超管credential行锁+fileHashreceipt。

**幂等**：fileHash作为operationId；删除失败重复仅补清理。

**依赖**：identity.HashPassword；identity.RevokeAllSessions。

## identity.IssueRecoveryAuthorization

文件：`modules/identity/internal/recovery_authorization.go`。

`func IssueRecoveryAuthorization(ctx RequestContext, input IssueRecoveryAuthorizationInput) (IssueRecoveryAuthorizationResult, error)`

超管批准后签发有限恢复许可。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：IssueRecoveryAuthorizationInput。超管批准后签发有限恢复许可的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：IssueRecoveryAuthorizationResult。超管批准后签发有限恢复许可的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 确认唯一active superadmin、最近登录与VerifyPassword；当前operation epoch正常。
2. 许可绑定restore/barrier/旧新epoch/各audience/允许控制actions和期限，禁止任意业务读写。
3. 在不被快照覆盖control.db记录approval/hash/撤销状态，用独立recovery信任锚签发；data-protection持此逐域执行。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：独立control.db批准记录与签名。

**授权**：superadmin + 当前口令step-up；不能普通service自签。

**并发**：restoreId唯一批准锁。

**幂等**：同restoreId同摘要返回同许可，异摘要CONFLICT。

**依赖**：identity.VerifyPassword。

## identity.ValidateRecoveryAuthorization

文件：`modules/identity/internal/recovery_authorization.go`。

`func ValidateRecoveryAuthorization(ctx RequestContext, input ValidateRecoveryAuthorizationInput) (ValidateRecoveryAuthorizationResult, error)`

本地验证不会被identity换代阻断的恢复许可。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidateRecoveryAuthorizationInput。本地验证不会被identity换代阻断的恢复许可的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateRecoveryAuthorizationResult。本地验证不会被identity换代阻断的恢复许可的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变。

**步骤**

1. 由各域保留信任锚验签、期限和issuer批准记录/撤销hash。
2. 严格核restore/barrier/旧新epoch/audience/action；只能barrier/snapshot/runtimeactivation。
3. 普通业务路径即使携此许可也FORBIDDEN；不读导入后的session/role来延续恢复，但审批主体证据永久保留。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false。

**副作用**：无。

**授权**：注册service对应audience；本地控制记录不被恢复覆盖。

**并发**：本地只读控制库/信任锚。

**幂等**：验证无状态。

**依赖**：无跨函数/跨域依赖。

## identity.RevokeRecoveryAuthorization

文件：`modules/identity/internal/recovery_authorization.go`。

`func RevokeRecoveryAuthorization(ctx RequestContext, input RevokeRecoveryAuthorizationInput) (RevokeRecoveryAuthorizationResult, error)`

撤销尚未完成的恢复许可。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RevokeRecoveryAuthorizationInput。撤销尚未完成的恢复许可的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RevokeRecoveryAuthorizationResult。撤销尚未完成的恢复许可的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 唯一超管或带外OS恢复主体明确撤销。
2. control.db CAS撤销许可并通知所有域。
3. 正在进行恢复保持维护；禁止撤销顺带关闭屏障。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：control.db撤销与控制事件。

**授权**：superadmin或带外OS控制身份。

**并发**：approval行CAS。

**幂等**：重复撤销无效果。

**依赖**：无跨函数/跨域依赖。

## identity.ResolveIdentityOperation

文件：`modules/identity/internal/operation_repository.go`。

`func ResolveIdentityOperation(ctx RequestContext, input ResolveIdentityOperationInput) (ResolveIdentityOperationResult, error)`

读取幂等回执。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ResolveIdentityOperationInput。读取幂等回执的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ResolveIdentityOperationResult。读取幂等回执的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 查询site/actor/operation唯一键。
2. 异摘要CONFLICT，敏感input仅salted digest不存明文。
3. 已提交返回安全结果hash供上层重读。

**错误**：CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：私有事务。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## identity.PublishIdentityEvent

文件：`modules/identity/internal/outbox_repository.go`。

`func PublishIdentityEvent(ctx RequestContext, input PublishIdentityEventInput) (PublishIdentityEventResult, error)`

与身份提交原子登记事件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PublishIdentityEventInput。与身份提交原子登记事件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PublishIdentityEventResult。与身份提交原子登记事件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核对无账号口令/IP正文。
2. 同业务txn写eventId唯一outbox。
3. dispatcher经jobs可靠投递，ack后标完成。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务、receipt和撤销outbox。

**授权**：私有身份txn。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：eventId去重。

**依赖**：jobs:publishEvent。

## identity.InvalidateRestoredSessions

文件：`modules/identity/internal/restore_security.go`。

`func InvalidateRestoredSessions(ctx RequestContext, input InvalidateRestoredSessionsInput) (InvalidateRestoredSessionsResult, error)`

恢复数据后使备份登录失效。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：InvalidateRestoredSessionsInput。恢复数据后使备份登录失效的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：InvalidateRestoredSessionsResult。恢复数据后使备份登录失效的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 仅snapshotcommit调用，恢复control记录保持独立。
2. 生成新authEpoch/signingkey并将所有恢复session撤销。
3. 保留账户密码/flags/超管唯一身份，专用恢复许可仍有效到协调结束。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：业务会话失效与独立签名控制key。

**授权**：service + 专用RecoveryAuthorization限定commit阶段。

**并发**：commit屏障独占。

**幂等**：targetEpoch唯一receipt。

**依赖**：无跨函数/跨域依赖。

## identity.AcquireBarrier

文件：`modules/identity/internal/barrier_service.go`。

`func AcquireBarrier(ctx RequestContext, input BarrierRequest) (BarrierStatus, error)`

取得可恢复的数据屏障。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：BarrierRequest。取得可恢复的数据屏障的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：BarrierStatus。取得可恢复的数据屏障的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 认证 data-protection 服务及 superadmin 委托；验证 epoch、operationId 与 deadline。
2. 私有事务登记屏障并拒绝新的业务写入；等待已登记提交完成。
3. 固定 revisionWatermark 并返回 safe；超时返回 draining/failed 且保持维护，不擅自释放。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection + 已批准且绑定 restoreId/barrierId/旧新epoch/audience/action/期限的专用RecoveryAuthorization；备份走当前superadmin委托。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState；identity.ValidateRecoveryAuthorization。

## identity.ReleaseBarrier

文件：`modules/identity/internal/barrier_service.go`。

`func ReleaseBarrier(ctx RequestContext, input ReleaseBarrierInput) (ReleaseBarrierResult, error)`

释放已协调完成的屏障。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReleaseBarrierInput。释放已协调完成的屏障的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReleaseBarrierResult。释放已协调完成的屏障的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变。

**步骤**

1. 查询持久屏障。
2. 验证维护退出决定、所有激活/回滚步骤已完成及 epoch 一致。
3. 原子改 released 并恢复业务接入。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false。

**副作用**：私有屏障状态写入。

**授权**：service:data-protection + 已批准且绑定 restoreId/barrierId/旧新epoch/audience/action/期限的专用RecoveryAuthorization；备份走当前superadmin委托。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config:readRuntimeState；identity.ValidateRecoveryAuthorization。

## identity.ExportSnapshot

文件：`modules/identity/internal/snapshot_service.go`。

`func ExportSnapshot(ctx RequestContext, input SnapshotExportRequest) (SnapshotExportResult, error)`

导出屏障水位的一致快照。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotExportRequest。导出屏障水位的一致快照的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotExportResult。导出屏障水位的一致快照的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 验证本域 barrier=safe 和当前 dataEpoch；按 includeSecrets 策略封装敏感字段。
2. 流式复制私有数据库一致视图及所有权威文件到本服务只读对象；manifest 列 hash/size/schema。
3. fsync 完成后返回 SnapshotRef；失败只清理未完成临时对象，不修改活动数据。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection + 已批准且绑定 restoreId/barrierId/旧新epoch/audience/action/期限的专用RecoveryAuthorization；备份走当前superadmin委托。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState；identity.ValidateRecoveryAuthorization。

## identity.PrepareSnapshot

文件：`modules/identity/internal/snapshot_service.go`。

`func PrepareSnapshot(ctx RequestContext, input SnapshotPrepareRequest) (SnapshotStageResult, error)`

在私有 staging 区准备恢复。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotPrepareRequest。在私有 staging 区准备恢复的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotStageResult。在私有 staging 区准备恢复的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 要求 restore barrier=safe 和 site-config 维护状态。
2. 重新验证快照并复制到以 restoreId 命名的 staging 根，路径由服务生成。
3. 迁移到当前 schema，记录 stageId、目标 epoch 和旧活动 manifest；重复同摘要复用 stage。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection + 已批准且绑定 restoreId/barrierId/旧新epoch/audience/action/期限的专用RecoveryAuthorization；备份走当前superadmin委托。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState；identity.ValidateRecoveryAuthorization。

## identity.ValidateSnapshot

文件：`modules/identity/internal/snapshot_validator.go`。

`func ValidateSnapshot(ctx RequestContext, input SnapshotValidateRequest) (SnapshotValidation, error)`

验证快照可恢复性。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotValidateRequest。验证快照可恢复性的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotValidation。验证快照可恢复性的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 只经对象读取 API 获取本域快照，拒绝其他域目录访问。
2. 核对 owner、hash、schemaVersion、引用与状态机；检查目标 epoch。
3. 列出精确 field issues 和受支持迁移计划；不写活动数据。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：无。

**授权**：service:data-protection + 已批准且绑定 restoreId/barrierId/旧新epoch/audience/action/期限的专用RecoveryAuthorization；备份走当前superadmin委托。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState；identity.ValidateRecoveryAuthorization。

## identity.CommitSnapshot

文件：`modules/identity/internal/snapshot_repository.go`。

`func CommitSnapshot(ctx RequestContext, input SnapshotCommitRequest) (SnapshotApplyResult, error)`

激活已准备恢复数据。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotCommitRequest。激活已准备恢复数据的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotApplyResult。激活已准备恢复数据的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 要求维护、匹配 barrier/targetDataEpoch 与完整 stage 校验摘要。
2. 持久记录激活决定，原子切换活动 manifest/私有库连接到 stage 并 fsync。
3. 更新本域 epoch 和 commit receipt；保留旧 manifest 可回滚；进程崩溃按决定日志继续，不能部分激活。
4. 调用 InvalidateRestoredSessions；独立control.db、recovery信任锚与审批记录不被导入覆盖。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection + 已批准且绑定 restoreId/barrierId/旧新epoch/audience/action/期限的专用RecoveryAuthorization；备份走当前superadmin委托。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState；identity.ValidateRecoveryAuthorization；identity.InvalidateRestoredSessions。

## identity.RollbackSnapshot

文件：`modules/identity/internal/snapshot_repository.go`。

`func RollbackSnapshot(ctx RequestContext, input SnapshotCommitRequest) (SnapshotApplyResult, error)`

按协调决定回滚恢复阶段。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotCommitRequest。按协调决定回滚恢复阶段的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotApplyResult。按协调决定回滚恢复阶段的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 要求维护和同一 restore stage；仅接受协调者 rollback 决定。
2. 未激活 stage 删除；已激活则按持久旧 manifest 原子切回并验证 hash。
3. 记录 rolled-back 回执与原 epoch；无法回滚保持屏障并返回 INTEGRITY_ERROR。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection + 已批准且绑定 restoreId/barrierId/旧新epoch/audience/action/期限的专用RecoveryAuthorization；备份走当前superadmin委托。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState；identity.ValidateRecoveryAuthorization。

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M02-F901 main

文件 `modules/identity/cmd/identity/main.go`。独立进程可执行入口

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

### M02-F902 BuildApplication

文件 `modules/identity/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M02-F903 Serve

文件 `modules/identity/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M02-F904 Shutdown

文件 `modules/identity/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M02-F905 Health

文件 `modules/identity/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M02-F906 Ready

文件 `modules/identity/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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
<!-- service-bootstrap:end -->
