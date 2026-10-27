# portal 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 49 个函数：约束 13 个，参考 36 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## portal.CompileRouteRegistry

- 级别：参考
- 文件：`modules/portal/internal/route_registry.go`
- 作用：校验并编译公开与管理 API 的静态路由表

```go
func CompileRouteRegistry(input CompileRouteRegistryInput) (CompileRouteRegistryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `CompileRouteRegistryInput` | 校验并编译公开与管理 API 的静态路由表的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CompileRouteRegistryResult` | 校验并编译公开与管理 API 的静态路由表的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 检查方法、路径变量与最长匹配规则
2. 拒绝上游未注册、重叠危险路由及任意 URL
3. 产出只读路由索引

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：启动单线程构造后只读共享
- 幂等：同一配置产生同一索引
- 依赖：无

## portal.ResolveRoute

- 级别：参考
- 文件：`modules/portal/internal/route_registry.go`
- 作用：将请求解析为唯一服务及相对路径

```go
func ResolveRoute(input ResolveRouteInput) (ResolveRouteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ResolveRouteInput` | 将请求解析为唯一服务及相对路径的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveRouteResult` | 将请求解析为唯一服务及相对路径的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 验证路径编码且拒绝双重编码和路径穿越
2. 精确匹配方法与路由
3. 输出变量及权限要求

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：不可变索引并发读取
- 幂等：纯函数
- 依赖：无

## portal.ValidateEnvelope

- 级别：参考
- 文件：`modules/portal/internal/request_validation.go`
- 作用：限制请求体、请求头与 operationId

```go
func ValidateEnvelope(ctx context.Context, op OperationContext, input ValidateEnvelopeInput) (ValidateEnvelopeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidateEnvelopeInput` | 限制请求体、请求头与 operationId的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateEnvelopeResult` | 限制请求体、请求头与 operationId的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | BUDGET_EXCEEDED: 配额或字节预留不足，禁止部分提交；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 检查 Content-Length 与流读取上限
2. 校验请求标识和媒体类型
3. 提取 expectedRevision 与 operationId

**错误**

- `BUDGET_EXCEEDED`：配额或字节预留不足，禁止部分提交（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：只读取有界请求体
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：每请求独立缓冲；不得完整缓存超限流
- 幂等：纯验证
- 依赖：无

## portal.NormalizeForwardHeaders

- 级别：参考
- 文件：`modules/portal/internal/request_validation.go`
- 作用：创建可信上游请求头

```go
func NormalizeForwardHeaders(input NormalizeForwardHeadersInput) (NormalizeForwardHeadersResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `NormalizeForwardHeadersInput` | 创建可信上游请求头的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizeForwardHeadersResult` | 创建可信上游请求头的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 删除客户端身份、转发地址与 hop-by-hop 头
2. 仅复制路由允许的业务头
3. 注入服务器 requestId 与 traceparent

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：请求局部副本
- 幂等：相同输入输出相同
- 依赖：无

## portal.AssertSameOrigin

- 级别：参考
- 文件：`modules/portal/internal/origin_guard.go`
- 作用：对写入及凭据请求检查来源

```go
func AssertSameOrigin(input AssertSameOriginInput) (AssertSameOriginResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `AssertSameOriginInput` | 对写入及凭据请求检查来源的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AssertSameOriginResult` | 对写入及凭据请求检查来源的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 解析完整 Origin
2. 按 scheme/host/port 规范比较
3. 拒绝缺失或不匹配的写入 Origin
4. 规范化scheme/主机/有效端口，IPv6用方括号与net.JoinHostPort；Host显式默认端口与省略默认端口等价

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.VerifyCSRF

- 级别：参考
- 文件：`modules/portal/internal/origin_guard.go`
- 作用：验证身份服务绑定会话的 CSRF token

```go
func VerifyCSRF(ctx context.Context, op OperationContext, input VerifyCSRFInput) (VerifyCSRFResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `VerifyCSRFInput` | 验证身份服务绑定会话的 CSRF token的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `VerifyCSRFResult` | 验证身份服务绑定会话的 CSRF token的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 拒绝缺失 token
2. 调用 identity 验证会话绑定及有效期
3. 常量时间核对结果并闭锁失败

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：identity 验证读请求
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：请求级；不能复用其他会话结果
- 幂等：重复检查无状态变化
- 依赖：`identity:verifySessionCSRF`

## portal.ResolveAuthContext

- 级别：参考
- 文件：`modules/portal/internal/auth_proof.go`
- 作用：取得经 identity 校验的当前权限上下文

```go
func ResolveAuthContext(ctx context.Context, op OperationContext, input ResolveAuthContextInput) (ResolveAuthContextResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ResolveAuthContextInput` | 取得经 identity 校验的当前权限上下文的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveAuthContextResult` | 取得经 identity 校验的当前权限上下文的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 按 cookie 查会话
2. 向 identity 查询 session 与权限 revision
3. 校验站点和资源权限并返回 proof
4. 使用identity:checkAuthorization的portal accessToken模式，action固定为当前静态owner:operationId，不能由浏览器自报主体或动作

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：有限身份查询与审计
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：每请求结果不可跨站点缓存；缓存上限 5 秒且强制撤回检查
- 幂等：读操作
- 依赖：`identity:checkAuthorization`、`portal.Resolve`

## portal.BuildInternalProof

- 级别：参考
- 文件：`modules/portal/internal/auth_proof.go`
- 作用：生成仅当前路由可使用的内部身份声明

```go
func BuildInternalProof(ctx context.Context, op OperationContext, input BuildInternalProofInput) (BuildInternalProofResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `BuildInternalProofInput` | 生成仅当前路由可使用的内部身份声明的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildInternalProofResult` | 生成仅当前路由可使用的内部身份声明的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 绑定 service audience、method、path、body digest
2. 约束 expiry 不超过 10 秒及源 auth expiry
3. 使用入口专有密钥签名
4. SessionVersion取identity权威返回的auth.sessionRevision，不能用permissionRevision代替

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：密钥签名
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：签名器线程安全；nonce 每请求唯一
- 幂等：不同请求 nonce 不同
- 依赖：`portal.AuthorizeRoute`、`platform.SignDelegation`

## portal.AuthorizeRoute

- 级别：参考
- 文件：`modules/portal/internal/auth_proof.go`
- 作用：入口按声明执行提前授权

```go
func AuthorizeRoute(input AuthorizeRouteInput) (AuthorizeRouteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `AuthorizeRouteInput` | 入口按声明执行提前授权的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AuthorizeRouteResult` | 入口按声明执行提前授权的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 检查 siteId
2. 匹配 route permissions 与 resource scope
3. 缺项拒绝并保留服务端二次授权

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：不可变 auth
- 幂等：纯函数
- 依赖：无

## portal.Resolve

- 级别：参考
- 文件：`modules/portal/internal/auth_proof.go`
- 作用：通过identity版本化接口解析当前会话与动作授权

```go
func (r IdentityAuthResolver) Resolve(ctx context.Context, cookies SessionCookies, required PermissionRequirement) (AuthContext, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 传播请求取消与可信op | 由入口构造 |
| `cookies` | `SessionCookies` | 仅提取会话访问token | 禁止日志 |
| `required` | `PermissionRequirement` | 静态路由所需权限与动作 | 不能由客户端覆盖 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `auth` | `AuthContext` | identity当前权威授权结果 | 无error才有效 |
| `error` | `error` | 公共安全失败 | 不含token |

**步骤**

1. 用accessToken/permission/action调用identity:checkAuthorization，不传浏览器自报subject/site
2. 身份依赖失败关闭，authorized=false拒绝；成功返回权威AuthContext
3. access模式仅发送accessToken/permission/action，不夹带subject/site/session/resource/expectedPermissionRevision即使为null

**错误**

- `DEPENDENCY_UNAVAILABLE`：identity不可用或响应格式不合法（可重试）
- `FORBIDDEN`：当前主体无操作权限（不可重试）

- 副作用：HTTP调用identity
- 授权：由已解析静态路由的portal入口调用
- 并发：客户端可并发，不缓存允许结果
- 幂等：只读
- 依赖：`identity:checkAuthorization`

## portal.ForwardRequest

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：发送一次有界请求到拥有服务

```go
func ForwardRequest(ctx context.Context, op OperationContext, input ForwardRequestInput) (ForwardRequestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ForwardRequestInput` | 发送一次有界请求到拥有服务的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ForwardRequestResult` | 发送一次有界请求到拥有服务的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 构建固定上游地址
2. 设置 route timeout、取消与流限制
3. 执行调用并保留可审计 requestId
4. 禁止自动跟随重定向，透传拥有服务原3xx；上游请求ctx取消句柄随响应移交StreamResponse，不能在返回响应头时取消

**错误**

- `DEADLINE_EXCEEDED`：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准（可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：上游调用可能提交业务写入
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：同一 operationId 不盲重放非幂等请求；取消不视为未提交
- 幂等：仅拥有服务决定业务幂等
- 依赖：`portal.NormalizeForwardHeaders`、`platform.SignAssertion`

## portal.StreamResponse

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：有背压地输出上游内容及 Range 响应

```go
func StreamResponse(ctx context.Context, op OperationContext, input StreamResponseInput) (StreamResponseResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `StreamResponseInput` | 有背压地输出上游内容及 Range 响应的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StreamResponseResult` | 有背压地输出上游内容及 Range 响应的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 校验状态/头白名单
2. 保持 Content-Range、ETag 与内容长度
3. 分块复制并在断开时取消读取
4. 关闭上游Body后释放ForwardRequest的请求取消句柄，SSE和大响应仍受同一请求截止及浏览器取消控制
5. 固定四个identity认证响应在64KiB总界内读取，严格解码受信内部DTO并核对cookie字段/期限，在写任何响应头前完成safe body与cookie构造
6. 认证非200只输出已登记code/status/retryable/requestId及固定安全错误说明，不透传原始detail/未知字段，不设或清cookie；畸形/截断/超限响应失败关闭
7. 认证成功只回安全session和按操作允许的account/nextRoute；csrfToken仅供浏览器内存；丢弃上游Content-Length/ETag/Location并设置实际JSON长度及no-store
8. 仅当前权威session撤销成功才清两枚同属性cookie；撤其他设备和失败不清
9. 在WriteHeader前标记ResponseStarted，部分写/取消后上层不追加第二个Problem

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：写客户端响应
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：固定缓冲；一个 reader 与 writer；禁止响应开始后切状态
- 幂等：流不可重放
- 依赖：`portal.MapUpstreamFailure`、`portal.authenticationOperation`、`portal.adaptAuthenticationResponse`、`portal.isCurrentSessionRevocation`、`portal.adaptSessionRevocation`

## portal.MapUpstreamFailure

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：转换内部失败为标准公开错误

```go
func MapUpstreamFailure(input MapUpstreamFailureInput) (MapUpstreamFailureResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `MapUpstreamFailureInput` | 转换内部失败为标准公开错误的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MapUpstreamFailureResult` | 转换内部失败为标准公开错误的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 保留已知业务 code、retryable 和 requestId
2. 去掉内部主机、路径及堆栈
3. 不确定写结果返回 COMMIT_STATUS_UNKNOWN

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.ProxyRequest

- 级别：约束
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：同源入口：把浏览器请求安全转发到拥有服务

```go
func ProxyRequest(ctx context.Context, op OperationContext, input ProxyRequestInput) (ProxyRequestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ProxyRequestInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ProxyRequestResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. ResolveRoute在启动时编译的路由表中精确匹配方法与路径；ValidateEnvelope限制体积、媒体类型与operationId；CheckEntranceGate执行维护与epoch门禁
2. 写入或带凭据请求先AssertSameOrigin与VerifyCSRF；管理路由ResolveAuthContext经identity核会话与权限，AuthorizeRoute预检，BuildInternalProof签发≤10秒用户委托证明
3. 路由标记x-admission的公开AI请求：从可信代理链取访客地址（只在内存），携访客会话、动作与请求摘要调用traffic-guard:createAdmission；拒绝返回429/403与retryAfter或挑战要求，通过则附X-Admission-Permit头
4. ForwardRequest以本服务断言与委托证明转发；StreamResponse有背压地回传（SSE与Range透传），失败经MapUpstreamFailure映射
5. RedactRequest生成最小访问字段；管理与安全相关路由经EmitAccessAudit登记审计
6. 账户权限写请求检查CSRF；anonymous凭证登录/刷新和公开请求仍要求固定部署Origin但不要求有效access，避免过期令牌阻断refresh
7. 用户proof只用专用委托密钥，服务HTTP断言仍使用服务密钥，两个用途不互换
8. 仅静态identity POST四操作适配认证；浏览器刷新严格空对象，从nw_refresh构造内部秘密正文后重算bytes/hash，匿名refresh不签用户proof
9. 登录/首次注册只用platform.PrivateNetworkScopes从真实socket及受信CIDR代理链派生当前+前一UTC日网络HMAC，更新受信op后由服务断言签入LoginNetworkScopes；不记录地址或接受浏览器scope
10. 认证响应统一Cache-Control:no-store；成功raw tokens只进入Secure HttpOnly SameSite=Strict host-only Path=/ cookie，浏览器body只保安全session/account/nextRoute；失败和敏感回执冲突不改已有cookie
11. 当前设备撤销只用ResolveAuthContext的权威sessionId传入响应适配；非当前设备不删除cookie
12. 匿名traffic-guard调用只从真实连接/部署信任链生成站点隔离30日预算范围；当前/前期预算key最多两把，未配置站点/key失败关闭，不用HTTP签名key默默替代预算pepper
13. 仅analytics:ingestBeacon匿名公开路由：以启动装配的严格公开schema校验实际正文，2048字节上限；用唯一可信代理解析与真实HTTP User-Agent覆写来源，写当前epoch并重算签名正文hash。转固定/v1/internal/beacons，仅本机portal纯服务身份，禁止转浏览器身份或原IP日志。
14. 匿名问答和风控接口含读取及SSE统一no-store/Cross-Origin-Resource-Policy:same-origin；存在Sec-Fetch-Site时只接受same-origin，存在Origin时核对配置来源和入口Host。原会话、挑战、签名准入及网络限额继续校验。

**错误**

- `NOT_FOUND`：未登记路由（不可重试）
- `FORBIDDEN`：来源、CSRF或权限不符（不可重试）
- `RATE_LIMITED`：风控拒绝（不可重试）
- `LOCKED`：维护中（不可重试）
- `DEPENDENCY_UNAVAILABLE`：上游不可用（可重试）

- 副作用：转发HTTP请求；调用identity、traffic-guard；访问审计
- 授权：permission：anonymous；rule：入口本身匿名可达；路由权限按路由表与上游服务二次校验
- 并发：每请求独立；上游取消随客户端断开传播
- 幂等：幂等由上游operationId保证；入口不重放写请求
- 依赖：`portal.ResolveRoute`、`portal.ValidateEnvelope`、`portal.CheckEntranceGate`、`portal.AssertSameOrigin`、`portal.VerifyCSRF`、`portal.ResolveAuthContext`、`portal.AuthorizeRoute`、`portal.BuildInternalProof`、`traffic-guard:createAdmission`、`portal.ForwardRequest`、`portal.StreamResponse`、`portal.RedactRequest`、`portal.EmitAccessAudit`、`platform.PrivateNetworkScopes`、`portal.authenticationOperation`、`portal.prepareAuthenticationRequest`、`platform.PrivateBudgetNetworkScopes`、`platform.ResolveVisitorAddress`、`portal.prepareTrustedBeacon`、`analytics:ingestTrustedBeacon`

## portal.authenticationOperation

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：识别固定四个identity敏感认证operation

```go
func authenticationOperation(route ResolvedRoute) string
```

**输入**

无

**输出**

无

**步骤**

1. 仅owner=identity、POST且operationId为createSession/registerFirstSuperadmin/refreshSession/changePassword时返回该操作；不按用户路径片段猜测

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：无

## portal.prepareAuthenticationRequest

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：从HttpOnly刷新cookie建立内部请求

```go
func prepareAuthenticationRequest(runtime *ProxyRuntime) error
```

**输入**

无

**输出**

无

**步骤**

1. 浏览器Content-Type为application/json且严格空对象，拒未知字段/null/尾随JSON
2. nw_refresh必须唯一且32..4096字节；不从浏览器正文或可信头读取secret
3. 只在内存编码identity.RefreshSessionInput，由调用方按实际新正文重算bytes/hash

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`portal.decodeAuthenticationJSON`

## portal.adaptAuthenticationResponse

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：把内部敏感认证DTO转换为安全浏览器JSON与cookie

```go
func adaptAuthenticationResponse(response *http.Response, operation string) ([]byte, []*http.Cookie, error)
```

**输入**

无

**输出**

无

**步骤**

1. 响应体总界64KiB，非200只映射标准Problem且不改cookie
2. 200必须JSON；固定operation各自校验tokens/currentSessionTokens/account/nextRoute的组合，未知字段或尾随内容失败关闭
3. nextRoute只准固定后台路径；构造cookie后只序列化安全session和允许的account/nextRoute，不输出access/refresh

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`portal.decodeAuthenticationJSON`、`portal.authenticationCookies`、`portal.authenticationProblem`

## portal.decodeAuthenticationJSON

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：敏感wire边界有界DTO解码

```go
func decodeAuthenticationJSON(raw []byte, target any) error
```

**输入**

无

**输出**

无

**步骤**

1. 只解码已受总字节上限约束的正文，DisallowUnknownFields并要求尾随EOF，不回显敏感取值

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：无

## portal.authenticationCookies

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：构造两枚同源敏感会话cookie

```go
func authenticationCookies(tokens identitySessionTokens) ([]*http.Cookie, error)
```

**输入**

无

**输出**

无

**步骤**

1. 要求内部token/CSRF/会话与访问期限材料完整、auth同session且同访问期限
2. nw_sid与nw_refresh均host-only Path=/ Secure HttpOnly SameSite=Strict，完整序列化不超过4096字节且由net/http校验cookie
3. access Expires使用identity权威expiresAt；refresh为浏览器session cookie，不延长或推测服务端绝对期限

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：无

## portal.authenticationProblem

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：隔离认证失败中的内部原始字段

```go
func authenticationProblem(response *http.Response, raw []byte) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 要求application/problem+json与有效code/status/requestId
2. 仅保留已登记code/status/retryable/requestId并写固定安全说明；不透传原始detail/title/issues，CONFLICT提示重新登录

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`portal.decodeAuthenticationJSON`

## portal.isCurrentSessionRevocation

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：精确识别当前设备撤销

```go
func isCurrentSessionRevocation(route ResolvedRoute, currentSessionID Identifier) bool
```

**输入**

无

**输出**

无

**步骤**

1. 只接受identity DELETE revokeSession，静态参数sessionId等于ResolveAuthContext权威sessionId；其他设备不清cookie

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：无

## portal.adaptSessionRevocation

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：当前设备撤销成功后清除两枚cookie

```go
func adaptSessionRevocation(response *http.Response) ([]byte, []*http.Cookie, error)
```

**输入**

无

**输出**

无

**步骤**

1. 64KiB总界读取；非200安全Problem且不清cookie
2. 仅JSON 200 revoked=true权威回执构造两枚同属性cookie删除值，MaxAge=-1且Expires过去；先构造再写头

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`portal.decodeAuthenticationJSON`、`portal.authenticationProblem`

## portal.prepareTrustedBeacon

- 级别：参考
- 文件：`modules/portal/internal/proxy_stream.go`
- 作用：以严格公开正文和可信连接构造签名内部beacon，不记录原IP

```go
func prepareTrustedBeacon(ctx context.Context, op OperationContext, runtime *ProxyRuntime) ([]byte, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 当前真实请求上下文 | 随下游IO传递 |
| `op` | `OperationContext` | 本次真实入口上下文 | epoch由权威读取 |
| `runtime` | `ProxyRuntime` | 真实请求内存上下文 | 可信启动装配 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `body` | `[]byte` | 仅内部HTTP传输的canonical正文 | 不日志、不持久 |
| `error` | `error` | 未知身份或正文不合法拒绝 |  |

**步骤**

1. 实际公开正文≤2048字节并以生产方严格公开schema校验；客户端peer/epoch等未知字段拒绝。
2. 公共ResolveVisitorAddress只从真实socket与部署信任链取得规范地址；User-Agent只用真实HTTP头且≤1024字节有效UTF8，绝不使用客户端JSON自报值。
3. 注入本次dataEpoch，canonical JSON≤4096；仅返回内存正文，由ProxyRequest重算bodyHash并签入portal纯服务调用。

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`platform.ResolveVisitorAddress`、`platform.CanonicalJSON`

## portal.LoadBootstrap

- 级别：约束
- 文件：`modules/portal/internal/bootstrap_service.go`
- 作用：聚合当前站点与用户后台壳信息

```go
func LoadBootstrap(ctx context.Context, op OperationContext, input LoadBootstrapInput) (LoadBootstrapResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `LoadBootstrapInput` | 聚合当前站点与用户后台壳信息的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadBootstrapResult` | 聚合当前站点与用户后台壳信息的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 并行读取 identity profile 与 site-config public/admin summary
2. 按权限筛选 capabilities 和导航
3. 标记可选组件 degraded；身份失败直接拒绝

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：服务读调用
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：只返回同一个 siteId 和 permissionRevision；结果期限 5 秒
- 幂等：读操作
- 依赖：`identity:readSelf`、`site-config:readSiteConfig`、`portal.BuildCapabilities`、`portal.BuildNavigation`、`identity:resolveHost`、`identity:allowTlsHost`、`identity:redeemPrivateAccess`

## portal.BuildCapabilities

- 级别：参考
- 文件：`modules/portal/internal/bootstrap_service.go`
- 作用：将权限及站点功能转为 UI capabilities

```go
func BuildCapabilities(input BuildCapabilitiesInput) (BuildCapabilitiesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `BuildCapabilitiesInput` | 将权限及站点功能转为 UI capabilities的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildCapabilitiesResult` | 将权限及站点功能转为 UI capabilities的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 去重 grant
2. 计算展示与禁用原因
3. 对未知 grant 默认隐藏
4. 仅取本域gen.AdminRoutes公共目录；permissions任一满足，路径/中文标签不手写第二表；zh-Hans使用中文标签

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无
- 其他依赖说明：公共contracts/permission-groups.json的adminRoutes/interfaceLocales，经make gen生成本域只读数据

## portal.BuildNavigation

- 级别：参考
- 文件：`modules/portal/internal/bootstrap_service.go`
- 作用：构建稳定导航与可访问路由

```go
func BuildNavigation(input BuildNavigationInput) (BuildNavigationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `BuildNavigationInput` | 构建稳定导航与可访问路由的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildNavigationResult` | 构建稳定导航与可访问路由的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 遍历固定导航定义
2. 按 capability 过滤
3. 选择用户语言的标题与 fallback
4. 仅取本域gen.AdminRoutes公共目录；permissions任一满足，路径/中文标签不手写第二表；zh-Hans使用中文标签

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无
- 其他依赖说明：公共contracts/permission-groups.json的adminRoutes/interfaceLocales，经make gen生成本域只读数据

## portal.ReadPreferences

- 级别：约束
- 文件：`modules/portal/internal/preferences_service.go`
- 作用：读取用户的显示偏好

```go
func ReadPreferences(ctx context.Context, op OperationContext, input ReadPreferencesInput) (ReadPreferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadPreferencesInput` | 读取用户的显示偏好的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadPreferencesResult` | 读取用户的显示偏好的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 由 identity 按当前 subjectId/siteId 读取偏好
2. 无记录使用已声明默认值
3. portal 不建立副本权威

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：identity 读取
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：版本快照读取
- 幂等：读操作
- 依赖：`identity:readPreferences`

## portal.UpdatePreferences

- 级别：约束
- 文件：`modules/portal/internal/preferences_service.go`
- 作用：更新主题、语言及默认后台页

```go
func UpdatePreferences(ctx context.Context, op OperationContext, input UpdatePreferencesInput) (UpdatePreferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdatePreferencesInput` | 更新主题、语言及默认后台页的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdatePreferencesResult` | 更新主题、语言及默认后台页的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用 |

**步骤**

1. 校验主题、语言和授权 route
2. 转发 expectedRevision 与 operationId 到 identity
3. 返回 identity 新 revision，禁止入口自行写入

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）

- 副作用：identity 拥有偏好写入
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：用户/站点键事务 CAS
- 幂等：同 operationId 同 digest 返回原结果
- 依赖：`identity:updatePreferences`、`portal.ValidatePreferenceRoute`

## portal.ValidatePreferenceRoute

- 级别：参考
- 文件：`modules/portal/internal/preferences_service.go`
- 作用：保证首页偏好对应当前可访问路由

```go
func ValidatePreferenceRoute(input ValidatePreferenceRouteInput) (ValidatePreferenceRouteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidatePreferenceRouteInput` | 保证首页偏好对应当前可访问路由的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePreferenceRouteResult` | 保证首页偏好对应当前可访问路由的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配 |

**步骤**

1. 仅接受内部路由 key
2. 核对固定 route catalog
3. 验证所需 capability

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：`portal.BuildNavigation`

## portal.OpenEventStream

- 级别：约束
- 文件：`modules/portal/internal/event_bridge.go`
- 作用：提供后台权限过滤 SSE

```go
func OpenEventStream(ctx context.Context, op OperationContext, input OpenEventStreamInput) (OpenEventStreamResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `OpenEventStreamInput` | 提供后台权限过滤 SSE的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `OpenEventStreamResult` | 提供后台权限过滤 SSE的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 验证 cursor 的用户与站点绑定
2. 以ephemeral模式调用jobs:subscribeEvents订阅用户事件流：从当前水位开始，不建投递、不确认
3. 发送有限 snapshot 后开始序列流

**错误**

- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：jobs 订阅连接
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：每用户最多 3 流；背压队列 256 条；超限断开并给重连 cursor
- 幂等：相同 cursor 可重新读取
- 依赖：`jobs:subscribeEvents`、`portal.FilterPortalEvent`、`portal.EncodeSSEFrame`、`portal.ResumeEventCursor`

## portal.FilterPortalEvent

- 级别：参考
- 文件：`modules/portal/internal/event_bridge.go`
- 作用：移除当前用户不可读取事件

```go
func FilterPortalEvent(input FilterPortalEventInput) (FilterPortalEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `FilterPortalEventInput` | 移除当前用户不可读取事件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FilterPortalEventResult` | 移除当前用户不可读取事件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 核对 siteId 与 audience
2. 检查 domain/resource grant
3. 最小化负载并隐藏敏感内容

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：每事件使用最新权限 revision
- 幂等：纯过滤
- 依赖：`identity:checkAuthorization`

## portal.EncodeSSEFrame

- 级别：参考
- 文件：`modules/portal/internal/event_bridge.go`
- 作用：安全编码 SSE frame

```go
func EncodeSSEFrame(input EncodeSSEFrameInput) (EncodeSSEFrameResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `EncodeSSEFrameInput` | 安全编码 SSE frame的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EncodeSSEFrameResult` | 安全编码 SSE frame的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 规范 id、event 与 JSON data
2. 禁止事件名称及 id 的换行注入
3. 输出 UTF-8 frame

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.ResumeEventCursor

- 级别：参考
- 文件：`modules/portal/internal/event_bridge.go`
- 作用：验证重连游标的签名和上下文

```go
func ResumeEventCursor(input ResumeEventCursorInput) (ResumeEventCursorResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ResumeEventCursorInput` | 验证重连游标的签名和上下文的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResumeEventCursorResult` | 验证重连游标的签名和上下文的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态 |

**步骤**

1. 验签
2. 检查 expiry 与主体/站点
3. 验证权限 revision 变化时重取 snapshot

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：同 cursor 同结果
- 依赖：无

## portal.CollectReadiness

- 级别：约束
- 文件：`modules/portal/internal/health_service.go`
- 作用：输出入口自身及必要依赖可用性

```go
func CollectReadiness(ctx context.Context, op OperationContext, input CollectReadinessInput) (CollectReadinessResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CollectReadinessInput` | 输出入口自身及必要依赖可用性的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CollectReadinessResult` | 输出入口自身及必要依赖可用性的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 检查路由和签名密钥配置
2. 以严格 deadline 检测 identity/site-config
3. 区分 required 与 optional 依赖

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：有界探测
- 授权：service/运维健康身份，禁止公开依赖地址
- 并发：禁止每流量请求触发；健康调用共享 2 秒快照
- 幂等：读操作
- 依赖：`site-config:readRuntimeState`

## portal.RedactRequest

- 级别：参考
- 文件：`modules/portal/internal/audit_redaction.go`
- 作用：按静态路由生成最小访问审计字段；未写明剥除或保留规则的字段失败关闭

```go
func RedactRequest(input RedactRequestInput) (RedactRequestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `RedactRequestInput` | 生成不会泄漏密码、token 或正文的访问日志字段的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RedactRequestResult` | 生成不会泄漏密码、token 或正文的访问日志字段的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 保留字段不合法，或出现合同未写明应剥除或保留的请求头、query 与路径变量；不记录敏感原值 |

**步骤**

1. 只登记静态路由的 method 与 routeId，以及请求的 requestId 与 bodyBytes；耗时不在输入或 RedactedAuditRequest 中，不自造
2. 剥除名为 cookie 与 authorization 的头，值不进入结果
3. query 敏感键与路径变量敏感标识合同未写明；出现 query、路径变量或其他请求头时失败关闭，不自选脱敏规则

**错误**

- `VALIDATION`：保留字段不合法，或出现合同未写明应剥除或保留的请求头、query 与路径变量；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.EmitAccessAudit

- 级别：参考
- 文件：`modules/portal/internal/audit_redaction.go`
- 作用：可靠投递入口访问结果

```go
func EmitAccessAudit(ctx context.Context, op OperationContext, input EmitAccessAuditInput) (EmitAccessAuditResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `EmitAccessAuditInput` | 可靠投递入口访问结果的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EmitAccessAuditResult` | 可靠投递入口访问结果的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 生成不含凭据与正文的字段
2. 调用analytics:appendAudit登记访问元数据，并写本机结构化运行日志
3. 投递失败标记 degraded；业务审计仍由 owning service 的提交 outbox 保证

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：运行日志；HTTP调用analytics:appendAudit
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：requestId 唯一键；审计失败时管理写请求返回受控不可用
- 幂等：requestId 去重
- 依赖：`analytics:appendAudit`

## portal.ReadRuntimeState

- 级别：参考
- 文件：`modules/portal/internal/runtime_state_client.go`
- 作用：读取唯一权威运行状态

```go
func ReadRuntimeState(ctx context.Context, op OperationContext, input ReadRuntimeStateInput) (ReadRuntimeStateResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadRuntimeStateInput` | 读取唯一权威运行状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadRuntimeStateResult` | 读取唯一权威运行状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 调用 site-config readRuntimeState
2. 验证返回 epoch、维护状态与签名
3. 仅缓存最多 1 秒；失联时写请求关闭

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：service:portal
- 并发：短期单飞；维护开始事件立即失效缓存
- 幂等：只读
- 依赖：`site-config:readRuntimeState`

## portal.CheckEntranceGate

- 级别：参考
- 文件：`modules/portal/internal/maintenance_gate.go`
- 作用：对维护和恢复 epoch 实施入口屏障

```go
func CheckEntranceGate(ctx context.Context, op OperationContext, input CheckEntranceGateInput) (CheckEntranceGateResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CheckEntranceGateInput` | 对维护和恢复 epoch 实施入口屏障的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CheckEntranceGateResult` | 对维护和恢复 epoch 实施入口屏障的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 读取当前权威 runtimeState
2. 维护时仅允许登录/恢复/健康固定路由
3. 业务写入携带活动 dataEpoch；不匹配或处于恢复维护时拒绝请求

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：所有请求按路由自身权限
- 并发：每请求检查，缓存维护变更必须立即失效
- 幂等：读取判定
- 依赖：`portal.ReadRuntimeState`

## portal.main

- 级别：约束
- 文件：`modules/portal/cmd/portal/main.go`
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
- 依赖：`portal.BuildApplication`、`portal.Serve`、`portal.Shutdown`、`platform.RunService`

## portal.BuildApplication

- 级别：约束
- 文件：`modules/portal/internal/app/wire.go`
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
6. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环
7. 装配IdentityAuthResolver与CSRFVerifier；可信Origin只来自platform.ServiceConfig.publicBaseURL，不从请求Host反推
8. 分别加载服务断言与delegationKeyRef专用委托签名引用；委托keyId必须在portalDelegationKeys登记，缺失失败关闭
9. 完整本域handler覆盖未齐时NewApplication明确DEPENDENCY_UNAVAILABLE，禁止用默认503占位handler或固定Ready成功冒充完整服务；TrustedProxyCIDRs只从受信ServiceConfig注入
10. 入口公开基址必须经platform.NormalizePublicURL校验；https为默认，显式AllowLocalHTTP只开放loopback本地HTTP；cookie仍为Secure
11. 用严格生产方readSiteConfig返回值取得真实siteId，绑定真实运行epoch；不可用不以system或随机ID代替。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`portal.CompileRouteRegistry`、`platform.ReadRuntime`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`portal.NewApplication`、`platform.NormalizePublicURL`、`platform.LoadPortalDelegation`、`portal.readSiteIdentity`、`platform.LoadServiceIdentity`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## portal.readSiteIdentity

- 级别：参考
- 文件：`modules/portal/internal/app/wire.go`
- 作用：启动从生产方只读权威取得真实站点身份

```go
func readSiteIdentity(ctx context.Context, config platform.ServiceConfig, key platform.SigningKey, epoch platform.Identifier) (platform.Identifier, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 启动IO取消 |  |
| `config` | `ServiceConfig` | 可信部署依赖 |  |
| `key` | `SigningKey` | 已加载服务签名 |  |
| `epoch` | `Identifier` | 真实runtime epoch |  |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `siteID` | `Identifier` | 真实站点ID |  |
| `error` | `error` | 依赖或完整性错误 |  |

**步骤**

1. 构造当前epoch、30秒截止的portal纯服务只读上下文，撤用户证明且费用类别为空。
2. 调用site-config:readSiteConfig，以生产方PortalSiteConfigResponse严格schema验证并读取config.siteId；未知不猜站点ID。

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`site-config:readSiteConfig`、`platform.NewPayloadValidator`

## portal.Serve

- 级别：约束
- 文件：`modules/portal/internal/transport/server.go`
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
5. 除本域接口外，其余路径全部交给ProxyRequest；路由表外的路径返回404

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：`portal.BuildApplication`、`portal.Shutdown`、`portal.ProxyRequest`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## portal.Shutdown

- 级别：约束
- 文件：`modules/portal/internal/transport/server.go`
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

## portal.Health

- 级别：约束
- 文件：`modules/portal/internal/transport/server.go`
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

## portal.Ready

- 级别：约束
- 文件：`modules/portal/internal/transport/server.go`
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
5. 完整handler或初始化/恢复权威尚未装配时返回DEPENDENCY_UNAVAILABLE，不把常量Initialized/JournalRecovered/SafePoint当事实

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：无
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## portal.NewApplication

- 级别：参考
- 文件：`modules/portal/internal/transport/server.go`
- 作用：完整本域HTTP覆盖未齐时拒绝构造应用

```go
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, signingKey platform.SigningKey, delegationKey platform.SigningKey, runtime portal.RuntimeReader, identity portal.IdentityVerifier, authResolver portal.AuthResolver, registry portal.RouteRegistry, openedEpoch platform.Identifier, siteID platform.Identifier) (*Application, error)
```

**输入**

无

**输出**

无

**步骤**

1. 注入受信配置/服务与专用委托密钥/epoch/权威客户端；TrustedProxyCIDRs不从浏览器取
2. registerLocalRoutes按静态required清单核真实非nil处理器，缺任何本域handler返回DEPENDENCY_UNAVAILABLE；不注册默认503占位
3. 只有本域完整覆盖后才挂静态代理catchall并返回Application
4. 仅由启动传入真实siteID，编译登记BeaconPublicRequest严格校验器并传入请求内存上下文；失败拒绝装配。

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`portal.registerLocalRoutes`、`portal.proxyHandler`、`platform.NewPayloadValidator`

## portal.registerLocalRoutes

- 级别：参考
- 文件：`modules/portal/internal/transport/server.go`
- 作用：校验并注册实际本域处理器

```go
func (a *Application) registerLocalRoutes(mux *http.ServeMux) error
```

**输入**

无

**输出**

无

**步骤**

1. 实际非nil handler map与九项静态required路径核齐才注册并标记覆盖完成；缺失则失败关闭，无默认占位成功

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`portal.Health`、`portal.Ready`

## portal.proxyHandler

- 级别：参考
- 文件：`modules/portal/internal/transport/server.go`
- 作用：从真实HTTP请求建立受信portal运行时

```go
func (a *Application) proxyHandler(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 读取有限正文/查询及入口标识，受信epoch与30秒deadline由进程建立
2. Origin白名单只取固定部署PublicBaseURL；注入真实socket请求与受信TrustedProxyCIDRs
3. 调用ProxyRequest并只在响应尚未开始时写Problem，已写出的cookie/body不再追加第二个错误响应

**错误**

- 无

- 副作用：无
- 授权：仅受信进程运行时与静态路由；用户写仍同Origin、CSRF和identity当前权威
- 并发：每请求局部值，完成验证与构造前不写浏览器头或正文
- 幂等：不重放上游写；只适配本次权威结果，失败不改变已有cookie
- 依赖：`portal.ProxyRequest`

## portal.App

- 级别：约束
- 文件：`modules/portal/web/src/App.tsx`
- 作用：后台单页应用根：登录/初始化向导、后台外壳与按 permission-groups.adminRoutes 装配的各域页面

```ts
export function App(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 未登录时显示注册首个超管/登录/站点初始化向导；登录后读 /v1/self 与授权项
2. 外壳按授权项显示导航（只是少展示，权限仍在各服务端校验），页面懒加载自 modules/<域>/web/src
3. 提供同源 API 客户端（CSRF、单次静默刷新、401 回登录）、HeroUI Toast 与确认/输入对话框

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无
