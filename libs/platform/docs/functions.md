# platform 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 37 个函数：约束 19 个，参考 18 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## platform.SignAssertion

- 级别：约束
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：为一次服务间请求签发短期断言

```go
func SignAssertion(key SigningKey, claims AssertionClaims) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 本服务部署时生成的Ed25519私钥引用 | 只从secret文件读取，不进日志 |
| `claims` | `AssertionClaims` | 断言内容 | expiresAt-issuedAt≤60秒 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `token` | `string` | JWS compact断言 | 只放Authorization头 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 校验claims：issuer为本服务slug，audience为目标服务，method/path/bodyHash齐全，有效期≤60秒
2. 以EdDSA签名生成JWS compact，header含keyId
3. 返回token；不缓存、不记录

**错误**

- `VALIDATION`：声明不完整或有效期过长（不可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：纯函数，可并发
- 幂等：每次生成新jti
- 依赖：`platform.validLoginScopes`

## platform.VerifyAssertion

- 级别：约束
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：验证服务间请求断言

```go
func VerifyAssertion(anchors TrustAnchors, token string, expect AssertionExpectation) (AssertionClaims, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 服务公钥表（trustAnchorRef加载） | 只接受EdDSA |
| `token` | `string` | 请求携带的断言 | 长度≤4096 |
| `expect` | `AssertionExpectation` | 本请求的期望值 | audience为本服务 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `AssertionClaims` | 已验证声明 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按keyId查公钥，拒绝未知key、非EdDSA算法与alg=none
2. 核audience、method、path、bodyHash、有效期（允许30秒时钟偏差）
3. jti在有效期内只接受一次（进程内重放缓存）
4. 返回已验证的调用服务slug

**错误**

- `UNAUTHENTICATED`：签名、有效期或声明不符（不可重试）
- `CONFLICT`：jti重放（不可重试）

- 副作用：写进程内重放缓存
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：重放缓存加锁
- 幂等：jti唯一
- 依赖：`platform.validLoginScopes`

## platform.validLoginScopes

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：限制私有登录网络来源

```go
func validLoginScopes(claims AssertionClaims) bool
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `AssertionClaims` | 已解码或待签服务声明 | 可信启动配置或入口校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `valid` | `bool` | 登录网络作用域合法来源 | 错误时结果无效 |

**步骤**

1. 仅portal至identity的服务断言允许1..2个合法HMAC hash；无作用域为普通服务请求

**错误**

- `DEPENDENCY_UNAVAILABLE`：路由/权威依赖未装配（可重试）
- `VALIDATION`：入口配置或请求不合法（不可重试）
- `UNAUTHENTICATED`：服务或用户身份无效（不可重试）
- `FORBIDDEN`：调用范围不允许（不可重试）
- `LOCKED`：维护门禁阻止业务（可重试）
- `VERSION_MISMATCH`：数据代次不一致（不可重试）

- 副作用：按函数职责访问底层IO，不输出秘密
- 授权：入口验证服务身份与用户即时授权，策略只来自本域合同装配
- 并发：只读启动配置，单请求上下文隔离
- 幂等：以可信Idempotency-Key建立operationId，不改已有费用rootOperationId
- 依赖：无

## platform.VerifyDelegation

- 级别：约束
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：验证portal签发的用户委托证明

```go
func VerifyDelegation(anchors TrustAnchors, proof string, expect DelegationExpectation) (DelegationClaims, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 含portal委托公钥 | 只接受EdDSA |
| `proof` | `string` | X-Portal-Proof头 | 长度≤8192 |
| `expect` | `DelegationExpectation` | 本请求的路由与正文摘要 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `DelegationClaims` | 已验证的用户委托 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 用portal委托公钥验签，拒绝未知key与过期（≤10秒）
2. 核route与bodyHash；下游链路允许原样转发同一证明
3. 返回actor、grants与sessionVersion；权限仍须由业务函数经identity:checkAuthorization核实
4. SessionVersion必须等于actor.sessionRevision；admin委托必须携带非空sessionId与正会话版本，缺失即拒绝
5. 委托过期按当前时间严格拒绝，不套用服务断言30秒时钟容差；剩余期限不得超过10秒；header typ固定JWT
6. CallerService必须取已验签服务断言issuer；portal首跳核proof原bodyHash，后续已认证服务跳原样保留原proof，当前正文只由本跳服务断言绑定，各域再核权限和资源

**错误**

- `UNAUTHENTICATED`：证明无效或过期（不可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：纯函数
- 幂等：无状态
- 依赖：无

## platform.SignCursor

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：签发作用域绑定的分页游标

```go
func SignCursor(key SigningKey, scope string, payload []byte) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 本服务受控Ed25519密钥引用；只在服务进程内使用 | 私钥不进入日志或返回值 |
| `scope` | `string` | 调用方构造的查询与授权范围 | 绑定服务/查询/站点/主体/dataEpoch，最多512字节 |
| `payload` | `[]byte` | 签名载荷 | 1至1024字节 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `token` | `string` | 不透明签名游标 | 无error时有效 |
| `error` | `error` | 安全错误；不回显载荷或密钥 | 仅公共错误码 |

**步骤**

1. 限制scope为非空且不超过512字节、payload为1至1024字节；调用方把服务、查询、站点、主体和dataEpoch绑定进scope
2. 复用LoadSigningKey读取固定key版本；以Ed25519签名NW-CURSOR专用头与scope/payload信封
3. 返回不透明游标；不存储、不记录私钥或载荷

**错误**

- `VALIDATION`：游标格式、长度、签名或作用域不符（不可重试）
- `DEPENDENCY_UNAVAILABLE`：服务私有签名密钥不可用（可重试）

- 副作用：读取本服务secret引用的私钥，复用平台密钥缓存
- 授权：permission：service；rule：库函数；调用方负责构造已授权查询的scope，不从客户端直接采用scope
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

## platform.VerifyCursor

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：验证分页游标并返回载荷

```go
func VerifyCursor(key SigningKey, scope string, token string) ([]byte, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 本服务受控Ed25519密钥引用；只在服务进程内使用 | 私钥不进入日志或返回值 |
| `scope` | `string` | 调用方构造的查询与授权范围 | 绑定服务/查询/站点/主体/dataEpoch，最多512字节 |
| `token` | `string` | 请求携带的不透明游标 | 最多4096字节 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `payload` | `[]byte` | 验证后的原始游标载荷 | 无error时有效 |
| `error` | `error` | 安全错误；不回显载荷或密钥 | 仅公共错误码 |

**步骤**

1. 限制输入不超过4096字节，解析三段compact游标；只接受EdDSA、NW-CURSOR类型与当前keyId
2. 复用LoadSigningKey取得本地公钥验签，拒绝格式错误、签名错误或scope不符
3. 仅在验证后返回原始payload；跨主体、站点、查询或dataEpoch由调用方scope隔离

**错误**

- `VALIDATION`：游标格式、长度、签名或作用域不符（不可重试）
- `DEPENDENCY_UNAVAILABLE`：服务私有签名密钥不可用（可重试）

- 副作用：读取本服务secret引用的私钥，复用平台密钥缓存
- 授权：permission：service；rule：库函数；调用方负责构造已授权查询的scope，不从客户端直接采用scope
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

## platform.SignSessionAccess

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：签发固定用途的会话访问令牌

```go
func SignSessionAccess(key SigningKey, claims SessionAccessClaims) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | identity受控签名密钥引用 | 不记录私钥 |
| `claims` | `SessionAccessClaims` | 会话及版本声明 | 期限不超过900秒 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `string` | 成功结果 | 无error时有效 |
| `error` | `error` | 公共安全错误 | 不含secret/原文 |

**步骤**

1. 固定iss=identity、aud=portal、alg=EdDSA、typ=NW-SESSION；核对标识、单调版本和iat/exp
2. 复用LoadSigningKey签发，返回compact JWS；不读取会话业务库

**错误**

- `VALIDATION`：输入不满足本函数约束（不可重试）
- `DEPENDENCY_UNAVAILABLE`：密钥或随机源不可用（可重试）

- 副作用：读取受控密钥引用或CSPRNG；不持有业务状态
- 授权：permission：service；rule：由服务可信装配调用；会话发行仅identity，委托发行仅portal，业务授权由调用链检查
- 并发：无共享业务可变状态；密钥缓存复用LoadSigningKey
- 幂等：签名同key/claims确定；随机秘密每次独立，不重放旧refresh明文
- 依赖：`platform.validSessionClaims`

## platform.VerifySessionAccess

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：验证固定用途的会话访问令牌

```go
func VerifySessionAccess(anchors TrustAnchors, token string) (SessionAccessClaims, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 受控identity公钥表 | 固定发行方identity |
| `token` | `string` | 浏览器访问令牌 | 最多4096字节 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SessionAccessClaims` | 成功结果 | 无error时有效 |
| `error` | `error` | 公共安全错误 | 不含secret/原文 |

**步骤**

1. 只接受EdDSA与NW-SESSION，按固定identity/kid取得公钥验签
2. 核固定issuer/audience、全部标识/版本、iat/exp；到期即拒绝，签发时间只容许30秒未来偏差
3. 返回已验证声明，调用方仍须同步检查会话、账户、凭据、权限和认证代次权威

**错误**

- `UNAUTHENTICATED`：声明或签名不符（不可重试）
- `DEPENDENCY_UNAVAILABLE`：密钥或随机源不可用（可重试）

- 副作用：读取受控密钥引用或CSPRNG；不持有业务状态
- 授权：permission：service；rule：由服务可信装配调用；会话发行仅identity，委托发行仅portal，业务授权由调用链检查
- 并发：无共享业务可变状态；密钥缓存复用LoadSigningKey
- 幂等：签名同key/claims确定；随机秘密每次独立，不重放旧refresh明文
- 依赖：`platform.validSessionClaims`

## platform.GenerateSessionSecrets

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：生成会话独立随机秘密

```go
func GenerateSessionSecrets() (SessionSecrets, error)
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SessionSecrets` | 成功结果 | 无error时有效 |
| `error` | `error` | 公共安全错误 | 不含secret/原文 |

**步骤**

1. crypto/rand分别产生三个独立32字节随机值；accessNonce使用hex编码，refresh/csrf使用无填充base64url
2. 秘密只在内存中交给identity，不落日志或通用回执；随机源失败关闭

**错误**

- `VALIDATION`：输入不满足本函数约束（不可重试）
- `DEPENDENCY_UNAVAILABLE`：密钥或随机源不可用（可重试）

- 副作用：读取受控密钥引用或CSPRNG；不持有业务状态
- 授权：permission：service；rule：由服务可信装配调用；会话发行仅identity，委托发行仅portal，业务授权由调用链检查
- 并发：无共享业务可变状态；密钥缓存复用LoadSigningKey
- 幂等：签名同key/claims确定；随机秘密每次独立，不重放旧refresh明文
- 依赖：无

## platform.SignDelegation

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：统一签发portal用户委托证明

```go
func SignDelegation(key SigningKey, claims DelegationClaims) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | portal受控委托密钥引用 | 不记录私钥 |
| `claims` | `DelegationClaims` | 已授权主体与路由/正文摘要 | 有效期最多10秒 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `string` | 成功结果 | 无error时有效 |
| `error` | `error` | 公共安全错误 | 不含secret/原文 |

**步骤**

1. 核委托声明与十秒期限，复用LoadSigningKey签发固定EdDSA JWS
2. portal只装配参数，不自写签名机制

**错误**

- `VALIDATION`：输入不满足本函数约束（不可重试）
- `DEPENDENCY_UNAVAILABLE`：密钥或随机源不可用（可重试）

- 副作用：读取受控密钥引用或CSPRNG；不持有业务状态
- 授权：permission：service；rule：由服务可信装配调用；会话发行仅identity，委托发行仅portal，业务授权由调用链检查
- 并发：无共享业务可变状态；密钥缓存复用LoadSigningKey
- 幂等：签名同key/claims确定；随机秘密每次独立，不重放旧refresh明文
- 依赖：无

## platform.validSessionClaims

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：共用会话访问声明的标识和期限核对

```go
func validSessionClaims(claims SessionAccessClaims) bool
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `SessionAccessClaims` | 已解码或准备签名的会话声明 | 固定会话用途 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `valid` | `bool` | 关键声明满足访问令牌约束 | 不替代业务权威授权 |

**步骤**

1. 核固定发行方接收方、标识及非负版本，iat最多未来30秒，exp未过且有效期最多900秒

**错误**

- 无

- 副作用：无
- 授权：库内纯校验
- 并发：只读，可并发
- 幂等：相同声明在相同时间水位下结果一致
- 依赖：无

## platform.PrivateOperationDigest

- 级别：参考
- 文件：`libs/platform/serviceauth/delegation.go`
- 作用：避免低熵秘密幂等摘要被离线猜测

```go
func PrivateOperationDigest(key SigningKey, scope string, payload []byte) (Hash, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 私有签名密钥引用 | 受信装配或调用边界 |
| `scope` | `string` | 业务动作/格式版本域分离 | 受信装配或调用边界 |
| `payload` | `[]byte` | 仅内存含秘密规范输入 | 受信装配或调用边界 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `digest` | `Hash` | 私有HMAC摘要 | 错误时结果无效 |
| `error` | `error` | 安全失败 | 错误时结果无效 |

**步骤**

1. 从Ed25519 seed以固定域分离派生HMAC key；再次域分离和scope长度编码后HMAC payload
2. 调用方保存digest_key_id并保留旧key过渡；无原key明确拒重放，不保存或返回raw payload/摘要

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：无

## platform.BuildOperationContext

- 级别：约束
- 文件：`libs/platform/opctx/context.go`
- 作用：从入站请求构建可信OperationContext

```go
func BuildOperationContext(r *http.Request, anchors TrustAnchors, self string, runtime RuntimeView) (OperationContext, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `r` | `*http.Request` | 入站请求 | 读取头部；正文以副本流读取SHA-256后原样回填，不缓存全文 |
| `anchors` | `TrustAnchors` | 公钥表 | 必填 |
| `self` | `string` | 本服务slug，作为断言audience期望 | 必填 |
| `runtime` | `RuntimeView` | 当前运行状态 | 1秒内读取 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `op` | `OperationContext` | 可信操作上下文 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. VerifyAssertion确定callerService；有X-Portal-Proof时VerifyDelegation得到actor
2. 读取X-Operation-Id、X-Root-Operation-Id、X-Cost-Class（只接受可信调用方传递）、截止与取消标识
3. dataEpoch取runtime当前active epoch；请求声明的epoch不一致返回VERSION_MISMATCH
4. 浏览器自报的身份、费用类别、网络作用域一律忽略
5. 先验证服务断言绑定当前方法/path/body，再将已认证issuer传委托验证；portal首跳核原bodyHash，后续跳不把衍生请求正文当原浏览器正文
6. 从已验签portal→identity断言复制Go私有LoginNetworkScopes，不从浏览器字段/header取；已过期/非法deadline明确拒绝，不能重置为默认30秒

**错误**

- `UNAUTHENTICATED`：断言或委托无效（不可重试）
- `VERSION_MISMATCH`：dataEpoch不一致（不可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每请求独立
- 幂等：无状态
- 依赖：`platform.VerifyAssertion`、`platform.VerifyDelegation`

## platform.WriteProblem

- 级别：约束
- 文件：`libs/platform/problem/problem.go`
- 作用：按RFC9457输出公共错误

```go
func WriteProblem(w http.ResponseWriter, problem Problem) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `w` | `http.ResponseWriter` | 响应 | 未写头 |
| `problem` | `Problem` | 公共Problem | code登记于errors.json |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按errors.json映射HTTP状态
2. 输出application/problem+json，detail去除路径、堆栈与密钥
3. 附requestId与retryable

**错误**

- 无

- 副作用：写HTTP响应
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每请求一次
- 幂等：无状态
- 依赖：无

## platform.EnqueueEvent

- 级别：约束
- 文件：`libs/platform/outbox/outbox.go`
- 作用：在业务事务内登记outbox事件

```go
func EnqueueEvent(tx Tx, envelope EventEnvelope) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tx` | `Tx` | 调用方业务事务 | 必须是同一提交 |
| `envelope` | `EventEnvelope` | 事件信封 | eventType已在本域events.json登记 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 校验信封字段与载荷不含正文、密钥、IP
2. 计算payloadHash，按eventId唯一插入outbox行（状态pending）
3. 随业务事务一起提交；事务回滚则事件不存在

**错误**

- `VALIDATION`：信封不合法（不可重试）
- `CONFLICT`：同eventId异hash（不可重试）

- 副作用：写本域outbox表
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：与业务事务同锁
- 幂等：eventId唯一
- 依赖：无

## platform.RunRelay

- 级别：约束
- 文件：`libs/platform/outbox/relay.go`
- 作用：outbox转发循环的通用实现

```go
func RunRelay(ctx context.Context, store OutboxStore, client EventPublisher, config RelayConfig) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `store` | `OutboxStore` | 本域outbox访问 | 同一数据库 |
| `client` | `EventPublisher` | jobs客户端 | 带服务断言 |
| `config` | `RelayConfig` | 批量与轮询参数 | batchLimit≤100 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按序号以30秒行租约领取已提交pending行
2. 逐条调用jobs:publishEvent，确认后标relayed
3. 失败按1/2/4秒退避（上限60秒）；同ID异hash标quarantined
4. 屏障draining时继续转发直到空；恢复换入后丢弃旧dataEpoch行

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：读写outbox；HTTP调用jobs
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：行级租约防多实例重复
- 幂等：eventId固定
- 依赖：`jobs:publishEvent`

## platform.NewEventPublisher

- 级别：参考
- 文件：`libs/platform/outbox/relay.go`
- 作用：适配生成客户端的真实publishEvent协议

```go
func NewEventPublisher(client PublishClient) EventPublisher
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `client` | `PublishClient` | 版本化raw Jobs客户端 | 受信装配或调用边界 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `publisher` | `EventPublisher` | 只提供事件发布端口 | 错误时结果无效 |

**步骤**

1. 封装event、eventId幂等键，只有accepted=true且eventId一致才转发成功

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：`platform.PublishEvent`

## platform.PublishEvent

- 级别：参考
- 文件：`libs/platform/outbox/relay.go`
- 作用：typed事件发布适配

```go
func (p eventPublisher) PublishEvent(ctx context.Context, event EventEnvelope) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信调用链 | 受信装配或调用边界 |
| `event` | `EventEnvelope` | 业务提交outbox信封 | 受信装配或调用边界 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全失败 | 错误时结果无效 |

**步骤**

1. 调用生成客户端并验证持久接收回执，不要求虚构租约方法

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：`jobs:publishEvent`

## platform.ClaimInbox

- 级别：约束
- 文件：`libs/platform/inbox/inbox.go`
- 作用：事件处理函数的inbox去重

```go
func ClaimInbox(tx Tx, envelope EventEnvelope) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tx` | `Tx` | 处理函数的业务事务 | 与业务写入同一提交 |
| `envelope` | `EventEnvelope` | 收到的事件 | 已校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `fresh` | `bool` | true表示首次处理 | false时调用方只确认不写业务 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 按eventId查inbox；同ID同hash返回false
2. 同ID异hash返回INTEGRITY_ERROR并隔离
3. 同subject低于已处理revision返回false
4. 首次则插入inbox行并返回true

**错误**

- `INTEGRITY_ERROR`：同eventId异payloadHash（不可重试）

- 副作用：写本域inbox表
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：与业务事务同锁
- 幂等：eventId唯一
- 依赖：无

## platform.RunConsumer

- 级别：约束
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件订阅循环的通用实现

```go
func RunConsumer(ctx context.Context, client JobsClient, config ConsumerConfig, handlers EventHandlerRegistry) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `JobsClient` | jobs客户端 | 带服务断言 |
| `config` | `ConsumerConfig` | 订阅的事件名/版本与并发 | 来自本域events.json |
| `handlers` | `EventHandlerRegistry` | 事件名→处理函数 | 与events.json handler一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. durable模式jobs:subscribeEvents，Last-Event-ID续传
2. 校验信封、payloadSchema与dataEpoch；旧代事件确认跳过，未换入的新代事件不确认
3. 按事件名调用登记处理函数；成功后jobs:acknowledgeEvent，失败不确认等待重投
4. 同subject串行，不同subject并发≤maxInFlight；屏障draining时停止领取

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：HTTP长连接与确认
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一个订阅
- 幂等：依赖处理函数inbox去重
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`

## platform.RunWorkerLoop

- 级别：约束
- 文件：`libs/platform/worker/loop.go`
- 作用：任务领取循环的通用实现

```go
func RunWorkerLoop(ctx context.Context, client JobsClient, config WorkerConfig, executors JobExecutorRegistry) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `JobsClient` | jobs客户端 | 带服务断言 |
| `config` | `WorkerConfig` | 本域jobTypes与并发 | 来自implementation.json |
| `executors` | `JobExecutorRegistry` | jobType→执行函数 | 与jobTypes一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. jobs:claimJob只领取登记类型；维护期间暂停
2. jobs:startJob后调用执行函数；每10秒jobs:heartbeatJob续租、上报进度并读取取消
3. 写回前jobs:checkLease；成功jobs:completeJob，失败jobs:failJob
4. lease过期立即停止写入，不重放已派发模型调用
5. 写回前CheckLease同时传jobId、workerId与fencingToken，防止同owner的其他worker借用租约
6. Complete/Fail携带workerId、完整可空结果引用与paidResultRefs；executor错误不得忽略；用户取消与失去租约分开，取消仍可在有效租约内确认并保存已付费结果
7. WorkerConfig.operationContext从本域权威任务读取获准主体/预算/截止；核operationId/rootOperationId/dataEpoch与领取结果一致，任务入口一次设置ctx期限，不借启动上下文生成新付费根操作
8. 每次start/heartbeat/check/complete/fail使用独立子OperationID，同一请求传输重试保同ID；executor仍用原业务操作；取消信号不能覆盖owner已成功结果，也不能代替owner明确安全取消确认
9. runtime必须装配，权威读取失败或维护均暂停领取；不得将未知状态视为允许

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：HTTP调用jobs租约接口
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：并发≤maxConcurrency
- 幂等：jobs按jobId与fence去重
- 依赖：`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`、`platform.workerControlContext`

## platform.workerControlContext

- 级别：参考
- 文件：`libs/platform/worker/loop.go`
- 作用：为独立worker控制请求分配幂等键

```go
func workerControlContext(ctx context.Context, op OperationContext) context.Context
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 下游IO取消与截止 | 原样传递 |
| `op` | `OperationContext` | 原任务可信操作 | 费用根不变 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 携带新控制子操作的上下文 | 保留rootOperationId/actor/dataEpoch/预算/期限 |

**步骤**

1. 只更换OperationID，保留原费用根、主体、预算、截止和dataEpoch

**错误**

- 无

- 副作用：无
- 授权：进程内可信worker调用
- 并发：每次生成独立ID
- 幂等：同一次下游请求重试复用返回上下文，不跨心跳或fence复用
- 依赖：无

## platform.InvokeModel

- 级别：约束
- 文件：`libs/platform/modelclient/invoke.go`
- 作用：模型调用助手的通用实现

```go
func InvokeModel(ctx context.Context, client GatewayClient, op OperationContext, input InvokeModelRequest) (InvokeModelOutcome, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止 | 取消即调用cancelInference |
| `client` | `GatewayClient` | 模型网关客户端 | 带服务断言 |
| `op` | `OperationContext` | 可信上下文 | 费用类别与根操作继承 |
| `input` | `InvokeModelRequest` | 调用请求 | taskId在根AI任务目录 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `outcome` | `InvokeModelOutcome` | 终态与事件 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. model-gateway:createInference（幂等键=操作ID+步骤名）
2. model-gateway:resumeInference按序号读取到终态，流式时逐条回调
3. 取消或截止时model-gateway:cancelInference
4. failed/charge_unknown按网关Problem返回，不在调用方重放

**错误**

- `CANCELLED`：已取消（不可重试）
- `DEPENDENCY_UNAVAILABLE`：网关失败（可重试）

- 副作用：HTTP调用网关
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每次调用独立
- 幂等：幂等键固定
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`

## platform.LoadServiceConfig

- 级别：约束
- 文件：`libs/platform/lifecycle/config.go`
- 作用：按CLI>env>文件>默认值加载并严格校验服务配置

```go
func LoadServiceConfig(args []string, env []string) (ServiceConfig, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `args` | `[]string` | 命令行参数 | --config/--data-dir/--listen |
| `env` | `[]string` | 环境变量 | 只读取NW_前缀 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `config` | `ServiceConfig` | 严格解析后的配置 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 读取配置文件，拒绝未知字段
2. 按优先级合并并校验端口、数据根、依赖地址与secret引用
3. secret只保存引用路径，不读入日志
4. 读取NW_PUBLIC_BASE_URL和NW_ALLOW_LOCAL_HTTP；配置来源不可被请求Host覆盖；非空地址禁userinfo/query/fragment，HTTP仅显式loopback本地开发
5. portal可配置delegationKeyRef绝对引用，与服务断言私钥区分；NW_DELEGATION_KEY_REF覆盖文件

**错误**

- `VALIDATION`：配置不合法（不可重试）

- 副作用：读配置文件
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：启动时调用一次
- 幂等：纯函数
- 依赖：`platform.NormalizePublicURL`

## platform.NormalizePublicURL

- 级别：参考
- 文件：`libs/platform/lifecycle/config.go`
- 作用：统一公开地址与国际域名规范

```go
func NormalizePublicURL(raw string, allowLocalHTTP bool) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `raw` | `string` | 受信部署URL | 受信装配或调用边界 |
| `allowLocalHTTP` | `bool` | 受信loopback HTTP开关 | 受信装配或调用边界 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `url` | `string` | IDNA主机/有效端口/尾斜杠规范地址 | 错误时结果无效 |
| `error` | `error` | 安全失败 | 错误时结果无效 |

**步骤**

1. IDNA Lookup规范域名，IPv6和默认端口统一；仅HTTPS或显式loopback HTTP
2. 拒credential/query/fragment/控制字符和路径穿越，保留basePath

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：无

## platform.CanonicalPublicOrigin

- 级别：参考
- 文件：`libs/platform/lifecycle/config.go`
- 作用：CanonicalPublicOrigin公共适配

```go
func CanonicalPublicOrigin(publicBaseURL string) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `publicBaseURL` | `string` | 已边界校验规范的部署地址 | 受信装配来源 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `origin` | `string` | 规范scheme://Host | 失败结果无效 |
| `error` | `error` | 安全失败 | 失败结果无效 |

**步骤**

1. 从受信规范URL去basePath提取Origin，不从Host请求头猜测，不重复引入另一套IDNA策略

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：无

## platform.ReadRuntime

- 级别：约束
- 文件：`libs/platform/lifecycle/runtime.go`
- 作用：读取site-config权威运行状态（最多缓存1秒）

```go
func ReadRuntime(ctx context.Context, client SiteConfigClient) (RuntimeView, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 截止 | ≤2秒 |
| `client` | `SiteConfigClient` | site-config客户端 | 带服务断言 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `runtime` | `RuntimeView` | 运行状态 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 调用site-config:readRuntimeState并核签名
2. 缓存≤1秒；失联时写请求一律关闭

**错误**

- `DEPENDENCY_UNAVAILABLE`：site-config不可达（可重试）

- 副作用：HTTP调用site-config
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：缓存加锁
- 幂等：只读
- 依赖：`site-config:readRuntimeState`

## platform.WatchRuntime

- 级别：约束
- 文件：`libs/platform/lifecycle/runtime.go`
- 作用：运行中发现恢复切换时请求安全退出

```go
func WatchRuntime(ctx context.Context, client SiteConfigClient, dataRoot string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `SiteConfigClient` | site-config客户端 | 带服务断言 |
| `dataRoot` | `string` | 本域数据根 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 每5秒ReadRuntime
2. phase为restore_switching或restore_rolling_back且本域有对应staged或已换入目录时返回ErrRestartForRestore
3. 调用方据此Shutdown并以退出码75退出，由进程守护重启

**错误**

- `DEPENDENCY_UNAVAILABLE`：site-config不可达（可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一个
- 幂等：只读
- 依赖：`platform.ReadRuntime`

## platform.NewSiteConfigClient

- 级别：约束
- 文件：`libs/platform/lifecycle/runtime.go`
- 作用：构造各域共用的site-config运行状态客户端，供ReadRuntime/WatchRuntime使用

```go
func NewSiteConfigClient(cfg ServiceConfig, key SigningKey) (SiteConfigClient, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `cfg` | `ServiceConfig` | 本服务配置 | dependencies含site-config的http(s)地址；module为本服务身份 |
| `key` | `SigningKey` | 本服务签名密钥引用 | 用于服务断言 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `client` | `SiteConfigClient` | 运行状态客户端 | 无error时有效 |
| `error` | `error` | 失败原因 | 不含密钥 |

**步骤**

1. 以本服务断言调用site-config:readRuntimeState（GET /v1/runtime-state）
2. 解开响应{"state": RuntimeState}外层，字段与RuntimeView同名；非2xx映射为公共错误码
3. 各域NewRuntimeClient只装配参数，不自写适配

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺少site-config地址、不可达或响应不合法（可重试）

- 副作用：HTTP调用site-config
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：客户端可并发使用；缓存由ReadRuntime负责
- 幂等：只读
- 依赖：`site-config:readRuntimeState`

## platform.NewRequestValidator

- 级别：约束
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：在HTTP入口按本域OpenAPI规格统一校验请求（ADR0005 kin-openapi；ADR0006边界一次校验）

```go
func NewRequestValidator(spec *openapi3.T) (func(http.Handler) http.Handler, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `spec` | `OpenAPIDocument` | 本域生成代码gen.GetSwagger()返回的内嵌规格 | 启动时构建一次 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `middleware` | `HTTPMiddleware` | 请求校验中间件 | 无error时有效 |
| `error` | `error` | 规格无法建立校验路由 | 启动失败，不放行未校验请求 |

**步骤**

1. 按路径匹配规格路由（忽略servers）；未登记路径交给后续路由返回404/405
2. 校验路径/查询/请求头参数与请求体：additionalProperties:false拒绝未知字段，长度、范围、枚举、必填按schema
3. 失败返回VALIDATION Problem，issues只给字段位置与schema关键字，不回显取值；身份与委托由BuildOperationContext校验，这里不重复

**错误**

- `VALIDATION`：请求不符合接口规格（不可重试）
- `INTEGRITY_ERROR`：内嵌规格无法建立校验路由（启动时）（不可重试）

- 副作用：读取并复原请求体
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：中间件无共享可变状态，可并发
- 幂等：纯校验
- 依赖：无

## platform.NewPayloadValidator

- 级别：参考
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：复用统一schema机制核事件载荷

```go
func NewPayloadValidator(document []byte, schemaName string) (PayloadValidator, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `document` | `[]byte` | 内嵌自包含OpenAPI载荷规格 | 受信装配或调用边界 |
| `schemaName` | `string` | 登记组件名 | 受信装配或调用边界 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `validate` | `PayloadValidator` | 严格载荷schema校验 | 错误时结果无效 |
| `error` | `error` | 安全失败 | 错误时结果无效 |

**步骤**

1. 启动解析自包含规格，禁止联网外部引用，schema缺失失败关闭
2. 每次JSON解码并VisitJSON，错误不回显原值

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：无

## platform.ServeEventStream

- 级别：参考
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：统一SSE事件流与心跳背压

```go
func ServeEventStream(ctx context.Context, w http.ResponseWriter, stream EventStream) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 请求取消 | 受信装配或调用边界 |
| `w` | `http.ResponseWriter` | SSE响应 | 受信装配或调用边界 |
| `stream` | `EventStream` | 本域事件流 | 受信装配或调用边界 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 流结束或安全IO失败 | 错误时结果无效 |

**步骤**

1. 每次最多一条待写事件，使用LastID输出id，15秒注释心跳并flush
2. 取消/结束关闭底层流归还资源；响应开始后不再写JSON Problem

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：无

## platform.NewServiceHandler

- 级别：参考
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：统一身份/上下文/schema/epoch/维护HTTP入口

```go
func NewServiceHandler(spec OpenAPIDocument, config ServiceBoundaryConfig, handlers HTTPHandlers) (HTTPHandler, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `spec` | `OpenAPIDocument` | 本域嵌入OpenAPI | 可信启动配置或入口校验 |
| `config` | `ServiceBoundaryConfig` | 固定服务来源/运行状态/授权/体积策略 | 可信启动配置或入口校验 |
| `handlers` | `HTTPHandlers` | 完整operationId到处理器 | 可信启动配置或入口校验 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handler` | `HTTPHandler` | 已组装入口 | 错误时结果无效 |
| `error` | `error` | 安全失败 | 错误时结果无效 |

**步骤**

1. 启动拒缺handler/允许调用方或授权回调，不以空handler假成功；health/ready匿名
2. 请求体有界，先验服务断言/原委托、数据epoch和固定caller白名单，再设一次截止与调用链ctx
3. 普通业务受维护门禁；管理操作只由固定MaintenanceOperations放行，函数内部仍履行自身屏障
4. 合法Idempotency-Key作为operationId；原root保留；注入原PortalProof供下游原样传递
5. 一次严格schema验证后交本域适配；用户权限由Authorize当前权威回调确认
6. 仅显式ServiceActorCallers子白名单且Actor.Kind=service允许业务service读取；有用户委托仍要求当前用户权威，不借服务名绕过

**错误**

- `DEPENDENCY_UNAVAILABLE`：路由/权威依赖未装配（可重试）
- `VALIDATION`：入口配置或请求不合法（不可重试）
- `UNAUTHENTICATED`：服务或用户身份无效（不可重试）
- `FORBIDDEN`：调用范围不允许（不可重试）
- `LOCKED`：维护门禁阻止业务（可重试）
- `VERSION_MISMATCH`：数据代次不一致（不可重试）

- 副作用：按函数职责访问底层IO，不输出秘密
- 授权：入口验证服务身份与用户即时授权，策略只来自本域合同装配
- 并发：只读启动配置，单请求上下文隔离
- 幂等：以可信Idempotency-Key建立operationId，不改已有费用rootOperationId
- 依赖：`platform.NewRequestValidator`、`platform.BuildOperationContext`、`platform.WriteProblem`

## platform.NewIdentityAuthorizer

- 级别：参考
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：NewIdentityAuthorizer公共适配

```go
func NewIdentityAuthorizer(client AuthorizationClient) Authorizer
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `client` | `AuthorizationClient` | 版本化identity客户端 | 受信装配来源 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `authorize` | `Authorizer` | 即时核权回调 | 失败结果无效 |

**步骤**

1. 发送可信op主体/站点/session/权限版本，静态action和null资源，调用identity:checkAuthorization
2. 核authorized必须存在且为true；透传公共身份错误，不创建会话/权限影子状态

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：`identity:checkAuthorization`

## platform.ApplyRestoreSwap

- 级别：约束
- 文件：`libs/platform/lifecycle/restore_swap.go`
- 作用：启动时按恢复阶段换入或换回active目录（ADR0003）

```go
func ApplyRestoreSwap(dataRoot string, runtime RuntimeView) (SwapResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `dataRoot` | `string` | 本域数据根 | 含active、control、restore子目录 |
| `runtime` | `RuntimeView` | 启动时读取的运行状态 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SwapResult` | 是否换入或换回 | 无error时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 读control区恢复日志
2. restore_switching且存在restore/<id>/staged且未换入：active→restore/<id>/before，staged→active，fsync父目录，写日志
3. restore_rolling_back且已换入：反向换回并写日志
4. 其余情况不动目录；目录状态与日志矛盾时拒绝启动

**错误**

- `INTEGRITY_ERROR`：目录与日志不一致（不可重试）

- 副作用：重命名数据目录；写control日志
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：启动阶段独占，入口开放前完成
- 幂等：按restoreId与日志状态幂等
- 依赖：无

## platform.RunService

- 级别：约束
- 文件：`libs/platform/lifecycle/service.go`
- 作用：各服务main的通用进程模板

```go
func RunService(spec ServiceSpec) int
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `spec` | `ServiceSpec` | 本域slug与装配回调 | 必填 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `exitCode` | `int` | 0正常、1失败、2配置错误、75恢复重启 | main直接os.Exit |

**步骤**

1. LoadServiceConfig；配置错误返回2
2. 注册SIGINT/SIGTERM；调用spec的BuildApplication（其中先ApplyRestoreSwap）
3. 运行Serve与WatchRuntime；收到信号或恢复切换时以30秒上限Shutdown
4. 返回对应退出码；不能安全关闭返回1并保留journal

**错误**

- `INTERNAL`：装配或运行失败（不可重试）

- 副作用：监听HTTP；启动后台任务
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一次
- 幂等：重启按journal恢复
- 依赖：`platform.LoadServiceConfig`、`platform.WatchRuntime`

## platform.main

- 级别：约束
- 文件：`libs/platform/cmd/nw-keys/main.go`
- 作用：生成各服务Ed25519密钥对与公钥表的运维工具

```go
func main()
```

**输入**

无

**输出**

无

**步骤**

1. 读取deploy/config/services.json中的服务清单
2. 为每个服务与portal委托各生成一对Ed25519密钥，私钥写入各自secret目录（0600）
3. 汇总公钥写deploy/config/trust-anchors.json；已存在的密钥不覆盖，--rotate时新增keyId并保留旧公钥一个过渡期

**错误**

- `VALIDATION`：服务清单不合法（不可重试）

- 副作用：写secret文件与公钥表
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：离线运行
- 幂等：已存在密钥不覆盖
- 依赖：无
