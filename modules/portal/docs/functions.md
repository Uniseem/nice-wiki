# portal 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 51 个函数：约束 14 个，参考 37 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：有限身份查询与审计
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：每请求结果不可跨站点缓存；缓存上限 5 秒且强制撤回检查
- 幂等：读操作
- 依赖：`identity:checkAuthorization`

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

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：密钥签名
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：签名器线程安全；nonce 每请求唯一
- 幂等：不同请求 nonce 不同
- 依赖：`portal.AuthorizeRoute`

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

**错误**

- `DEADLINE_EXCEEDED`：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准（可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：上游调用可能提交业务写入
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：同一 operationId 不盲重放非幂等请求；取消不视为未提交
- 幂等：仅拥有服务决定业务幂等
- 依赖：`portal.NormalizeForwardHeaders`

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

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：写客户端响应
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：固定缓冲；一个 reader 与 writer；禁止响应开始后切状态
- 幂等：流不可重放
- 依赖：`portal.MapUpstreamFailure`

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
- 依赖：`identity:readSelf`、`site-config:readSiteConfig`、`portal.BuildCapabilities`、`portal.BuildNavigation`

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

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

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

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：portal.use；匿名静态入口及初始化状态按 route catalog 白名单开放
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

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
- 依赖：`identity:updatePreferences`

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
- 依赖：`jobs:subscribeEvents`

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
- 作用：生成不会泄漏密码、token 或正文的访问日志字段

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 只登记方法、routeId、耗时与体字节数
2. 屏蔽 cookie、authorization 与敏感 query
3. 按路由敏感标识清理路径变量

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

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

## portal.sendApi

- 级别：参考
- 文件：`modules/portal/web/src/api/api-client.ts`
- 作用：统一浏览器 API 请求与错误状态

```ts
sendApi(input: sendApiInput, signal: AbortSignal): Promise<sendApiResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `sendApiInput` | 统一浏览器 API 请求与错误状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `sendApiResult` | 统一浏览器 API 请求与错误状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 加入 requestId、operationId、CSRF 与 If-Match
2. 将 abortSignal 绑定 fetch
3. 解析标准 envelope 并保留 retryAfter

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：浏览器 fetch
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：每个写命令唯一 operationId；共享 refreshSingleFlight
- 幂等：写失败不自动重复
- 依赖：`portal.refreshSingleFlight`、`portal.classifyApiError`

## portal.refreshSingleFlight

- 级别：参考
- 文件：`modules/portal/web/src/api/api-client.ts`
- 作用：对并发过期请求只刷新一次会话

```ts
refreshSingleFlight(input: refreshSingleFlightInput, signal: AbortSignal): Promise<refreshSingleFlightResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `refreshSingleFlightInput` | 对并发过期请求只刷新一次会话的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `refreshSingleFlightResult` | 对并发过期请求只刷新一次会话的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 复用正在进行 refresh promise
2. 轮换成功后更新 cookie 相关 CSRF 状态
3. 失败清空状态并要求登录

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：identity refresh 请求与内存 session 更新
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：整个浏览器 tab 单飞；跨 tab 通过 BroadcastChannel 协调
- 幂等：同一 refresh 只发送一次
- 依赖：`identity:refreshSession`

## portal.classifyApiError

- 级别：参考
- 文件：`modules/portal/web/src/api/api-client.ts`
- 作用：把业务错误映射到可恢复用户状态

```ts
classifyApiError(input: classifyApiErrorInput, signal: AbortSignal): Promise<classifyApiErrorResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `classifyApiErrorInput` | 把业务错误映射到可恢复用户状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `classifyApiErrorResult` | 把业务错误映射到可恢复用户状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 按 code 区分冲突、权限、限流与网络
2. 保留 requestId 与提交状态未知
3. 产生本地化提示 key

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.hydrateSession

- 级别：参考
- 文件：`modules/portal/web/src/state/session-store.ts`
- 作用：将服务端 session 和权限装入界面状态

```ts
hydrateSession(input: hydrateSessionInput, signal: AbortSignal): Promise<hydrateSessionResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `hydrateSessionInput` | 将服务端 session 和权限装入界面状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `hydrateSessionResult` | 将服务端 session 和权限装入界面状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 验证 siteId 与 expiry
2. 原子替换主体、grants 与 navigation
3. 发布 permissionRevision 更新

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：内存状态写
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：单 store mutation；旧 fetch 响应不可覆盖高 revision
- 幂等：相同 bootstrap revision 不重复发布
- 依赖：无

## portal.applyPermissionEvent

- 级别：参考
- 文件：`modules/portal/web/src/state/session-store.ts`
- 作用：及时撤去被取消权限的 UI 与编辑能力

```ts
applyPermissionEvent(input: applyPermissionEventInput, signal: AbortSignal): Promise<applyPermissionEventResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `applyPermissionEventInput` | 及时撤去被取消权限的 UI 与编辑能力的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `applyPermissionEventResult` | 及时撤去被取消权限的 UI 与编辑能力的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 比较 permissionRevision
2. 重新获取 bootstrap
3. 取消失效请求与编辑操作

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：重新加载和状态替换
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：高 revision 覆盖低 revision；重入合并
- 幂等：eventId 去重
- 依赖：`portal.LoadBootstrap`、`portal.clearSession`

## portal.clearSession

- 级别：参考
- 文件：`modules/portal/web/src/state/session-store.ts`
- 作用：清理退出或失效后的本地身份状态

```ts
clearSession(input: clearSessionInput, signal: AbortSignal): Promise<clearSessionResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `clearSessionInput` | 清理退出或失效后的本地身份状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `clearSessionResult` | 清理退出或失效后的本地身份状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 中止受保护请求
2. 清空身份、CSRF 和受保护视图缓存
3. 通知其他 tab

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：内存与 tab 通知
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：一次原子 store mutation
- 幂等：重复清空无效果
- 依赖：无

## portal.resolveAdminRoute

- 级别：参考
- 文件：`modules/portal/web/src/router/router.ts`
- 作用：解析后台地址并检查能力

```ts
resolveAdminRoute(input: resolveAdminRouteInput, signal: AbortSignal): Promise<resolveAdminRouteResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `resolveAdminRouteInput` | 解析后台地址并检查能力的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `resolveAdminRouteResult` | 解析后台地址并检查能力的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 规范站点内路由
2. 验证 params 与 required capability
3. 返回 allow/login/forbidden/notFound

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.guardUnsavedNavigation

- 级别：参考
- 文件：`modules/portal/web/src/router/router.ts`
- 作用：在离开页面前保留未保存编辑

```ts
guardUnsavedNavigation(input: guardUnsavedNavigationInput, signal: AbortSignal): Promise<guardUnsavedNavigationResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `guardUnsavedNavigationInput` | 在离开页面前保留未保存编辑的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `guardUnsavedNavigationResult` | 在离开页面前保留未保存编辑的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行 |

**步骤**

1. 收集 dirty editor
2. 等待用户选择保存、保留草稿或取消
3. 保存失败保留当前页面与错误

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）

- 副作用：可能发起已授权草稿保存
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：同时一个导航确认；pending intent 替换必须显式决定
- 幂等：同 intent 不重复弹窗
- 依赖：`content:saveDraft`

## portal.restoreReturnLocation

- 级别：参考
- 文件：`modules/portal/web/src/router/router.ts`
- 作用：安全恢复登录前的内部页面

```ts
restoreReturnLocation(input: restoreReturnLocationInput, signal: AbortSignal): Promise<restoreReturnLocationResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `restoreReturnLocationInput` | 安全恢复登录前的内部页面的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `restoreReturnLocationResult` | 安全恢复登录前的内部页面的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 仅解析同站点相对 route key
2. 去掉身份 query 与外部地址
3. 无效则回默认 dashboard

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.connectEventStream

- 级别：参考
- 文件：`modules/portal/web/src/events/event-client.ts`
- 作用：连接并自动恢复后台 SSE

```ts
connectEventStream(input: connectEventStreamInput, signal: AbortSignal): Promise<connectEventStreamResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `connectEventStreamInput` | 连接并自动恢复后台 SSE的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `connectEventStreamResult` | 连接并自动恢复后台 SSE的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态 |

**步骤**

1. 发送绑定用户游标
2. 处理 heartbeat 与 sequence
3. 退避重连并在游标失效时重取 snapshot

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）

- 副作用：网络连接与本地 cursor 写
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：同 tab 单连接；logout 中止；队列有界
- 幂等：eventId 去重
- 依赖：`portal.dispatchPortalEvent`

## portal.dispatchPortalEvent

- 级别：参考
- 文件：`modules/portal/web/src/events/event-client.ts`
- 作用：把校验后的事件路由到相关界面状态

```ts
dispatchPortalEvent(input: dispatchPortalEventInput, signal: AbortSignal): Promise<dispatchPortalEventResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `dispatchPortalEventInput` | 把校验后的事件路由到相关界面状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `dispatchPortalEventResult` | 把校验后的事件路由到相关界面状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 校验 schemaVersion 和 audience
2. 按 event type 分发
3. 未知 type 记录支持状态并忽略

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：各 store 可控状态更新
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：按 sequence 串行；权限事件优先取消请求
- 幂等：eventId 1280 条有界去重缓存
- 依赖：无

## portal.loadPreferences

- 级别：参考
- 文件：`modules/portal/web/src/preferences/preferences-store.ts`
- 作用：加载并应用用户显示偏好

```ts
loadPreferences(input: loadPreferencesInput, signal: AbortSignal): Promise<loadPreferencesResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `loadPreferencesInput` | 加载并应用用户显示偏好的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `loadPreferencesResult` | 加载并应用用户显示偏好的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 请求当前用户偏好
2. 校验版本与语言
3. 更新主题和默认首页

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：fetch 和主题状态更新
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：仅最新 request generation 可应用
- 幂等：同 revision 无重复 UI 更新
- 依赖：`portal.ReadPreferences`

## portal.savePreferences

- 级别：参考
- 文件：`modules/portal/web/src/preferences/preferences-store.ts`
- 作用：以 CAS 保存显示偏好

```ts
savePreferences(input: savePreferencesInput, signal: AbortSignal): Promise<savePreferencesResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `savePreferencesInput` | 以 CAS 保存显示偏好的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `savePreferencesResult` | 以 CAS 保存显示偏好的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变 |

**步骤**

1. 合并合法 patch
2. 携带当前 expectedRevision 与 operationId
3. 冲突时重载并保留待确认差异

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）

- 副作用：服务偏好写入
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：单用户 store 串行保存队列
- 幂等：operationId 服务去重
- 依赖：`portal.UpdatePreferences`

## portal.deriveShellState

- 级别：参考
- 文件：`modules/portal/web/src/layout/admin-shell.ts`
- 作用：计算初始化、登录、后台及降级壳状态

```ts
deriveShellState(input: deriveShellStateInput, signal: AbortSignal): Promise<deriveShellStateResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `deriveShellStateInput` | 计算初始化、登录、后台及降级壳状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `deriveShellStateResult` | 计算初始化、登录、后台及降级壳状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 优先检查初始化状态
2. 再检查 session 和 route decision
3. 组合可恢复依赖故障与导航

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：纯函数
- 幂等：纯函数
- 依赖：无

## portal.updateDocumentMetadata

- 级别：参考
- 文件：`modules/portal/web/src/layout/admin-shell.ts`
- 作用：更新后台页面标题及无索引声明

```ts
updateDocumentMetadata(input: updateDocumentMetadataInput, signal: AbortSignal): Promise<updateDocumentMetadataResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `updateDocumentMetadataInput` | 更新后台页面标题及无索引声明的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `updateDocumentMetadataResult` | 更新后台页面标题及无索引声明的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 使用固定站点名与页面标题
2. 设置管理页 robots noindex,nofollow
3. 清除不属于当前路由的旧 meta

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：DOM 文档头更新
- 授权：浏览器当前主体；入口和 owning service 必须重新授权，不信任前端隐藏状态
- 并发：单应用实例串行
- 幂等：相同 state 不重复 DOM 写
- 依赖：无

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
3. 业务写入携带活动 dataEpoch；错误或未全激活拒绝请求

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：所有请求按路由自身权限
- 并发：每请求检查，缓存维护变更必须立即失效
- 幂等：读取判定
- 依赖：`portal.ReadRuntimeState`

## portal.AcquireBarrier

- 级别：约束
- 文件：`modules/portal/internal/maintenance_gate.go`
- 作用：停止入口业务写入并排空在途写请求

```go
func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `BarrierRequest` | 停止入口业务写入并排空在途写请求的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 停止入口业务写入并排空在途写请求的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 验证 data-protection 与 superadmin 委托
2. 记录运行内屏障且向 site-config 验证维护已开启
3. 拒绝新业务写入，排空在途写请求后返回 safe；重启仍从 runtimeState 恢复关闭

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：本进程接入 gate 与在途计数
- 授权：service:data-protection + superadmin 委托
- 并发：请求登记与 gate 切换同一互斥区
- 幂等：barrierId 相同返回当前 gate 状态；不生成数据快照
- 依赖：`site-config:readRuntimeState`

## portal.ReleaseBarrier

- 级别：约束
- 文件：`modules/portal/internal/maintenance_gate.go`
- 作用：按权威状态恢复业务入口

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, input ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReleaseBarrierInput` | 按权威状态恢复业务入口的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 按权威状态恢复业务入口的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变 |

**步骤**

1. 检查 site-config 所有服务已完成激活且维护关闭
2. 验证期望 epoch
3. 清除本进程 gate 并接受新请求

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）

- 副作用：进程入口 gate 开启
- 授权：service:data-protection + superadmin 委托
- 并发：与新请求登记原子切换
- 幂等：同 barrierId 重复释放无副作用
- 依赖：`site-config:readRuntimeState`

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
- 依赖：`portal.BuildApplication`、`portal.Serve`、`portal.Shutdown`

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
2. 打开本域DB并运行已有schema迁移/文件journal恢复；保留restore控制记录/barrier/trust-anchor
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；已commit恢复未全完成时仅开放控制管理；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖/控制信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：无
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

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
3. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：`portal.BuildApplication`、`portal.Shutdown`
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

1. 确认配置/本域schema/journal/control信任锚完整并在lifecycle ready或维护控制ready
2. 已初始化时核activeDataEpoch等于runtime目标代次；关键运行权威不可达或旧epoch返回503；site-config本域读取自身权威
3. 维护中本域新epoch已激活可返回200 control-ready但publicRequestsAllowed=false，使全域激活检查不会和维护门禁互相等待；所有域完成后runtime解除维护才开放
4. 未初始化可仅带初始化能力ready，activeDataEpoch=null；无AI渠道不使基础内容能力失败，能力缺项显式呈现

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：无
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## portal.mountModuleUI

- 级别：约束
- 文件：`modules/portal/web/src/main.ts`
- 作用：本域Vue3应用/岛的唯一UI构建入口

```ts
function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `rootElement` | `HTMLElement` | 已经存在的模块挂载节点 | DOM标准库；为空或不属于当前document拒绝 |
| `bootstrap` | `ModuleUIBootstrap` | 已由portal/Go输出的安全启动元数据 | module固定本域，不含service凭证/secret |

**输出**

无

**步骤**

1. 校验rootElement与bootstrap module/capabilities；按本域已登记Vue页面导入模块路由，createApp挂载，不注册别域业务组件
2. 只装配同源HTTP/JSON/SSE客户端和已有权限清理/语言逻辑，复用identity/qa会话，缺AI能力保持关闭提示
3. 同DOM重复mount先复用既有实例；unmount停止AbortController/SSE/listeners，不能清服务端业务记录

**错误**

- `VALIDATION` / HTTP 400：bootstrap不匹配本域或DOM无效（不可重试）

- 副作用：挂载本域Vue组件和浏览器事件；不新建业务记录
- 授权：permission：anonymous；rule：只挂载UI；业务mutation仍按各本域OpenAPI权限，不能以能力字段绕过权限
- 并发：同DOM一个Vue实例，AbortController取消旧请求
- 幂等：同DOM/buildRevision重复挂载复用实例
- 依赖：无
- 其他依赖说明：本域已登记UI页面函数
