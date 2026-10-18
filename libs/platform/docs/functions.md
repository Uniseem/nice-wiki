# platform 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 108 个函数：约束 20 个，参考 88 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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
2. 可选siteId仅为合法Identifier，绑定进同一服务签名；省略保留旧system兼容，不改变用户授权
3. 以EdDSA签名生成JWS compact，header含keyId
4. 返回token；不缓存、不记录

**错误**

- `VALIDATION`：声明不完整或有效期过长（不可重试）

- 副作用：无
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：纯函数，可并发
- 幂等：每次生成新jti
- 依赖：`platform.validLoginScopes`、`platform.validBudgetScopes`

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
3. 校验签名内可选siteId合法；不存在保留旧服务调用兼容，不接受未签名站点头
4. jti在有效期内只接受一次（进程内重放缓存）
5. 返回已验证的调用服务slug

**错误**

- `UNAUTHENTICATED`：签名、有效期或声明不符（不可重试）
- `CONFLICT`：jti重放（不可重试）

- 副作用：写进程内重放缓存
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：重放缓存加锁
- 幂等：jti唯一
- 依赖：`platform.validLoginScopes`、`platform.validBudgetScopes`

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

## platform.PrivateNetworkScopes

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：统一瞬时网络来源信任链与每日私有作用域派生

```go
func PrivateNetworkScopes(key SigningKey, remoteAddr, forwardedFor string, trustedProxyCIDRs []string, now time.Time) ([]Hash, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | portal进程私有签名密钥引用 | 受信部署装配 |
| `remoteAddr` | `string` | 真实HTTP连接RemoteAddr | IP:port；禁止zone，不取浏览器头替代 |
| `forwardedFor` | `string` | 请求X-Forwarded-For完整值 | 只有真实peer命中受信网段才读取；最多4096字节、16跳 |
| `trustedProxyCIDRs` | `[]string` | 部署代理信任边界 | 已由LoadServiceConfig校验；最多32条规范CIDR，默认空 |
| `now` | `time.Time` | 可信服务时钟 | 按UTC日期派生 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `scopes` | `[]Hash` | 当前和前一UTC日伪名网络HMAC | 仅返回Hash，不能反推出原IP |
| `error` | `error` | 来源或密钥失败 | 不含地址、头或秘密 |

**步骤**

1. 从真实socket解析peer并Unmap IPv4映射；无部署信任时完全忽略转发头
2. 仅当socket peer受信时要求非空有界XFF；从最右端依次跳过受信代理，取第一个不受信地址，忽略其左侧不可信声明；读取的地址拒zone和非IP
3. 用PrivateOperationDigest和network/day/v1/UTC日期域分别派生当前日及前一日HMAC；原地址仅驻留调用内存，永不记录或返回

**错误**

- `VALIDATION`：实际连接或所需转发链不合法（不可重试）
- `DEPENDENCY_UNAVAILABLE`：私有摘要密钥不可用（可重试）

- 副作用：读取已缓存的私有密钥；不写业务数据
- 授权：仅受信portal入口调用；原IP不跨服务
- 并发：无可写业务状态，可并发
- 幂等：同密钥、规范网络地址和UTC日返回相同HMAC；前一日重叠覆盖短窗口
- 依赖：`platform.PrivateOperationDigest`、`platform.privateNetworkPeer`

## platform.SignOutboundRequest

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：统一服务断言和真实站点传递

```go
func SignOutboundRequest(ctx context.Context, r *http.Request, issuer, audience string, key SigningKey) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信链上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `r` | `*http.Request` | 本次出站请求 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `issuer` | `string` | 调用服务 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `audience` | `string` | 拥有服务 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `key` | `SigningKey` | 受控私钥引用 | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 从可信op复制可选Actor.SiteID进签名claims，不读站点HTTP头；登录网络作用域仍仅portal到identity
2. 按本次方法/path/body重新签名；保留已有root/epoch/costClass及更早截止
3. 统一覆盖或删除X-Portal-Proof，空证明不能残留请求对象旧头；不创建用户委托

**错误**

- `VALIDATION`：声明站点/时效等不合法（不可重试）
- `INTERNAL`：正文或私钥不可读（不可重试）

- 副作用：读取受控服务私钥并修改出站Authorization/上下文头；正文哈希后原样回填，不执行网络
- 授权：permission：service；rule：库函数，只取已建立可信OperationContext；签名站点不授予用户或对象权限
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：每次传输签发新jti，业务op与root不由此重建
- 依赖：`platform.SignAssertion`、`platform.applyOperationHeaders`

## platform.applyOperationHeaders

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：传递操作头并清除已撤去的用户证明

```go
func applyOperationHeaders(ctx context.Context, r *http.Request)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信链 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `r` | `*http.Request` | 出站请求 | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

无

**步骤**

1. 复制可信操作上下文头；proof为空时显式删除旧X-Portal-Proof

**错误**

- 无

- 副作用：覆盖或删除出站请求头，不发送网络请求
- 授权：permission：service；rule：仅使用可信调用链上下文，不从请求自报身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：同可信上下文产生相同操作头；空proof删除残留
- 依赖：无

## platform.privateNetworkPeer

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：唯一可信代理链解析，非受信连接忽略转发头，原地址仅内存

```go
func privateNetworkPeer(remoteAddr,forwardedFor string,trustedProxyCIDRs []string)(netip.Addr,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `remoteAddr` | `string` | 可信装配输入 | 不接受客户端自报授权 |
| `forwardedFor` | `string` | 可信装配输入 | 不接受客户端自报授权 |
| `trustedProxyCIDRs` | `[]string` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `NetAddr` | 成功结果或安全错误 | 失败不返回半结果 |
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 唯一可信代理链解析，非受信连接忽略转发头，原地址仅内存

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

## platform.PrivateBudgetNetworkScopes

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：站点隔离30日当前/前期HMAC，IPv6固定64位；预算key轮换保留旧key过渡；登录日范围不变

```go
func PrivateBudgetNetworkScopes(key SigningKey,remoteAddr,forwardedFor string,trustedProxyCIDRs []string,siteID Identifier,now time.Time)([]Hash,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 可信装配输入 | 不接受客户端自报授权 |
| `remoteAddr` | `string` | 可信装配输入 | 不接受客户端自报授权 |
| `forwardedFor` | `string` | 可信装配输入 | 不接受客户端自报授权 |
| `trustedProxyCIDRs` | `[]string` | 可信装配输入 | 不接受客户端自报授权 |
| `siteID` | `Identifier` | 可信装配输入 | 不接受客户端自报授权 |
| `now` | `time.Time` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `[]Hash` | 成功结果或安全错误 | 失败不返回半结果 |
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 站点隔离30日当前/前期HMAC，IPv6固定64位；预算key轮换保留旧key过渡；登录日范围不变

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：`platform.privateNetworkPeer`、`platform.PrivateOperationDigest`
- 其他依赖说明：受控Ed25519密钥引用

## platform.validBudgetScopes

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：仅允许portal给traffic-guard签至多四个有效Hash

```go
func validBudgetScopes(claims AssertionClaims)bool
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `AssertionClaims` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `bool` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 仅允许portal给traffic-guard签至多四个有效Hash

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

## platform.LoadSigningKey

- 级别：参考
- 文件：`libs/platform/serviceauth/assertion.go`
- 作用：唯一Ed25519私钥文件加载与元数据缓存

```go
func LoadSigningKey(key SigningKey) (ed25519.PrivateKey, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 外部Ed25519私钥引用 | 只读私钥文件，不记录原文 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `PrivateSigningKey` | 仅内存私钥 | 不可序列化或记录 |
| `error` | `error` | 安全错误 | 不泄漏路径 |

**步骤**

1. 读取外部PKCS8 PEM引用并验证Ed25519；缓存按文件元数据失效，不进入业务快照

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

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
7. 无proof时service actor的siteId取已验签可选siteId，省略为system；有proof时若同时有签名siteId则必须与委托actor站点相等，否则FORBIDDEN；站点声明不授予任何用户或对象权限

**错误**

- `UNAUTHENTICATED`：断言或委托无效（不可重试）
- `VERSION_MISMATCH`：dataEpoch不一致（不可重试）
- `FORBIDDEN`：签名站点与委托站点矛盾（不可重试）

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

## platform.FromError

- 级别：参考
- 文件：`libs/platform/problem/problem.go`
- 作用：把库错误归一为无敏感数据的公共Problem

```go
func FromError(err error, requestID Identifier) Problem
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 库或IO错误 | 原始文本不输出 |
| `requestID` | `Identifier` | 可信请求标识 | 平台入口建立 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `problem` | `Problem` | 公共安全错误响应 | 未知INTERNAL；按登记映射status，detail过滤敏感内容 |

**步骤**

1. 以固定INTERNAL安全缺省建立Problem
2. NormalizeIOError统一原始context及包裹公共错误，公共code/retryable保留，未知不暴露原文本
3. 按公共HTTP映射填status，detail沿既有sanitizeDetail处理

**错误**

- 无

- 副作用：无
- 授权：错误纯映射不授予权限
- 并发：无业务可变状态
- 幂等：相同错误及requestId输出同一安全描述
- 依赖：`platform.NormalizeIOError`

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

1. 校验轮询间隔并在进程取消前持续调用RunRelayBatch；每轮后按PollMillis等待
2. 单批领取、旧代丢弃、逐条发布与安全窗口内确认统一由RunRelayBatch实现；配置错误返回，运行依赖失败保留事件继续下一轮

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）
- `LOCKED`：维护无有效control写窗口、窗口截止或恢复阶段禁止写入（可重试）
- `VALIDATION`：必需运行权威/写窗口或循环参数未装配（不可重试）

- 副作用：读写outbox；HTTP调用jobs
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：行级租约防多实例重复
- 幂等：eventId固定
- 依赖：`platform.RunRelayBatch`

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

## platform.relayWriteContext

- 级别：参考
- 文件：`libs/platform/outbox/relay.go`
- 作用：relayWriteContext公共边界

```go
func relayWriteContext(ctx context.Context, config RelayConfig) (context.Context, RuntimeView, CancelFunc, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 背景生命周期取消 | 可信装配 |
| `config` | `RelayConfig` | 受信运行权威和写窗口 | 可信装配 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 受IO和屏障截止约束 | 失败无结果 |
| `view` | `RuntimeView` | 最新权威 | 失败无结果 |
| `cancel` | `CancelFunc` | 完成后释放 | 失败无结果 |
| `error` | `error` | 失败无写权限 | 失败无结果 |

**步骤**

1. 每次IO前读取runtime和control写窗口；未知、不允许恢复阶段或维护无窗口时拒写
2. IO最长30秒且服从父ctx和屏障更早截止，更新本次控制请求deadline而不更改既有费用根

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）
- `LOCKED`：维护无有效control写窗口、窗口截止或恢复阶段禁止写入（可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：无

## platform.RunRelayBatch

- 级别：参考
- 文件：`libs/platform/outbox/relay.go`
- 作用：单批转发已提交outbox事件并返回真实本批计数

```go
func RunRelayBatch(ctx context.Context, store OutboxStore, client EventPublisher, config RelayConfig) (RelayBatchResult, error)
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
| `result` | `RelayBatchResult` | 本批经本地持久确认后的relayed/pending/quarantined行数；不计未领取的全域积压 | 只在error为空时有效 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 校验batchLimit及必需runtime/writeWindow；每次active写前由NewWriteContext读取当前权威窗口
2. 先按当前epoch丢弃旧代pending，再重新取得窗口领取本批30秒行租约；不持SQL事务调用jobs
3. 逐条发布固定信封；网络返回后重取运行/窗口，只有同代且窗口有效才标relayed、隔离或安排1/2/4秒退避（上限60秒）
4. 原代改变则只丢弃原代行；本批已持久确认/隔离/保留pending分别计数，未知本地提交或窗口错误不返回成功计数

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）
- `LOCKED`：维护无有效control写窗口、窗口截止或恢复阶段禁止写入（可重试）
- `VALIDATION`：必需运行权威/写窗口或循环参数未装配（不可重试）

- 副作用：读写outbox；HTTP调用jobs
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：行级租约防多实例重复
- 幂等：eventId固定
- 依赖：`jobs:publishEvent`、`platform.relayWriteContext`、`platform.NewWriteContext`

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
func RunConsumer(ctx context.Context, client EventConsumerClient, config ConsumerConfig, handlers EventHandlerRegistry) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `EventConsumerClient` | jobs客户端 | 带服务断言 |
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
- 依赖：`platform.RunConsumerWithResult`

## platform.RunConsumerWithResult

- 级别：约束
- 文件：`libs/platform/inbox/consumer.go`
- 作用：以真实窄事件端口运行订阅并返回准确计数

```go
func RunConsumerWithResult(ctx context.Context, client EventConsumerClient, config ConsumerConfig, handlers EventHandlerRegistry) (ConsumerRunResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `EventConsumerClient` | jobs客户端 | 带服务断言 |
| `config` | `ConsumerConfig` | 订阅的事件名/版本与并发 | 来自本域events.json |
| `handlers` | `EventHandlerRegistry` | 事件名→处理函数 | 与events.json handler一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumerRunResult` | 所有分片退出后的实际累计处理/跳过确认与失败尝试数 | 不伪填零；失败不确认 |
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 启动校验runtime/payload校验器、完整登记handler与并发上限；只接真实事件窄客户端
2. 每次订阅、handler与确认建立当前epoch的有界纯服务上下文，不复用启动时失效proof/deadline
3. 同subject串行，分片间并发；权威未知/维护屏障暂停；新代不确认，旧代确认计skipped
4. 信封/schema错误、handler失败或缺失、确认失败均不确认且计failed；handler明确ErrEventSkipped只确认计skipped
5. 正常提交且确认成功计acknowledged；ctx退出排空分片后返回累计计数

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：HTTP长连接与确认
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一个订阅
- 幂等：依赖处理函数inbox去重
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`platform.consumerLoop.start`、`platform.consumerLoop.stop`、`platform.consumerLoop.operation`、`platform.consumerLoop.dispatch`、`platform.inbox.readRuntime`、`platform.inbox.pauseFor`、`platform.inbox.sleep`、`platform.inbox.isConfigRejected`

## platform.consumerLoop.start

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.start

```ts
func(c *consumerLoop) start(ctx context.Context)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：`platform.consumerLoop.worker`

## platform.consumerLoop.stop

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.stop

```ts
func(c *consumerLoop) stop()
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.consumerLoop.worker

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.worker

```ts
func(c *consumerLoop) worker(ctx context.Context,index int)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：`platform.consumerLoop.process`

## platform.consumerLoop.queueFor

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.queueFor

```ts
func(c *consumerLoop) queueFor(subject string)chan EventEnvelope
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.consumerLoop.operation

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.operation

```ts
func(c *consumerLoop) operation(ctx context.Context,epoch,root string)(context.Context,context.CancelFunc)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.consumerLoop.dispatch

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.dispatch

```ts
func(c *consumerLoop) dispatch(ctx context.Context,stream EventStream,lastID *string)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：`platform.consumerLoop.classify`、`platform.consumerLoop.queueFor`、`platform.consumerLoop.acknowledge`

## platform.consumerLoop.acknowledge

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.acknowledge

```ts
func(c *consumerLoop) acknowledge(ctx context.Context,event EventEnvelope,skipped bool)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：`platform.inbox.readRuntime`、`platform.inbox.pauseFor`、`platform.consumerLoop.operation`

## platform.consumerLoop.process

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.process

```ts
func(c *consumerLoop) process(ctx context.Context,event EventEnvelope)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：`platform.inbox.readRuntime`、`platform.inbox.pauseFor`、`platform.consumerLoop.operation`、`platform.consumerLoop.acknowledge`

## platform.consumerLoop.classify

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：consumerLoop.classify

```ts
func(c *consumerLoop) classify(ctx context.Context,event EventEnvelope)outcome
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：`platform.inbox.readRuntime`、`platform.inbox.pauseFor`、`platform.inbox.eventKey`

## platform.inbox.eventKey

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：inbox.eventKey

```go
func eventKey(name string,version int)string
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.inbox.pauseFor

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：inbox.pauseFor

```go
func pauseFor(view RuntimeView)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.inbox.readRuntime

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：inbox.readRuntime

```go
func readRuntime(ctx context.Context,provider func(context.Context)(RuntimeView,error))(RuntimeView,bool)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.inbox.sleep

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：inbox.sleep

```go
func sleep(ctx context.Context,d time.Duration)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.inbox.isConfigRejected

- 级别：参考
- 文件：`libs/platform/inbox/consumer.go`
- 作用：事件消费内部实际助手：inbox.isConfigRejected

```go
func isConfigRejected(err error)bool
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.RunWorkerLoop

- 级别：约束
- 文件：`libs/platform/worker/loop.go`
- 作用：任务领取循环的通用实现

```go
func RunWorkerLoop(ctx context.Context, client WorkerJobsClient, config WorkerConfig, executors JobExecutorRegistry) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程生命周期 | 取消即退出 |
| `client` | `WorkerJobsClient` | jobs客户端 | 带服务断言 |
| `config` | `WorkerConfig` | 本域jobTypes与并发 | 来自implementation.json |
| `executors` | `JobExecutorRegistry` | jobType→执行函数 | 与jobTypes一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 失败原因；调用方映射为公共Problem | 不含密钥、正文或IP |

**步骤**

1. 每次领取先读当前runtime，维护或空epoch暂停；以owner纯服务system建立30秒或更早父截止、独立op/root和当前epoch，清原用户proof；领取不访问本站业务对象，领取后由域factory读取真实站点/任务授权
2. jobs:claimJob只领取登记类型；维护期间暂停
3. jobs:startJob后调用执行函数；每10秒jobs:heartbeatJob续租、上报进度并读取取消
4. 写回前jobs:checkLease；成功jobs:completeJob，失败jobs:failJob
5. lease过期立即停止写入，不重放已派发模型调用
6. 写回前CheckLease同时传jobId、workerId与fencingToken，防止同owner的其他worker借用租约
7. Complete/Fail携带workerId、完整可空结果引用与paidResultRefs；executor错误不得忽略；用户取消与失去租约分开，取消仍可在有效租约内确认并保存已付费结果
8. WorkerConfig.operationContext从本域权威任务读取获准主体/预算/截止；核operationId/rootOperationId/dataEpoch与领取结果一致，任务入口一次设置ctx期限，不借启动上下文生成新付费根操作
9. 每次start/heartbeat/check/complete/fail使用独立子OperationID，同一请求传输重试保同ID；executor仍用原业务操作；取消信号不能覆盖owner已成功结果，也不能代替owner明确安全取消确认
10. runtime必须装配，权威读取失败或维护均暂停领取；不得将未知状态视为允许
11. 向executor传完整ObjectRef、JobID、WorkerID与fence；进程内CheckLease回调在不可逆决定前锁外串行读取HeartbeatJob当前取消/安全点及有效期，再核CheckLease；取消/未知/过期均失败关闭
12. 领取后即建立独立本地租约到期定时器；start/续租/检查RPC截止不超过已知租约。续租回执只在旧租约仍有效且新截止有效时更新定时器，续租与不可逆前检查共用串行通道，拒绝旧回执覆盖新截止；领域仍负责短事务CAS和持久恢复决定
13. executor返回后占用同一控制互斥排空在途心跳，显式核本地租约仍有效；最终check/complete/fail均用当前本地expiry截止，不提前取消抢占leaseLost原因；只有原业务已持久成功才能报告成功
14. start成功后先真实Heartbeat并调用可选OnLeaseRenewed；以后每次权威续租成功更新本域镜像，回调失败取消执行。同worker/fence/epoch本地CAS，不在域内另造心跳或递归CheckLease。

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用（可重试）

- 副作用：HTTP调用jobs租约接口
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：并发≤maxConcurrency
- 幂等：jobs按jobId与fence去重
- 依赖：`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`、`platform.workerControlContext`、`platform.newExecutionLease`、`platform.executionLeaseContext`、`platform.renewExecutionLease`、`platform.ClaimJob`、`platform.StartJob`、`platform.HeartbeatJob`、`platform.CheckLease`、`platform.CompleteJob`、`platform.FailJob`

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
2. 所有owner租约控制撤去上下文PortalProof，避免旧用户委托被重新发送；保留任务已核站点/root/费用和更早截止

**错误**

- 无

- 副作用：无
- 授权：进程内可信worker调用
- 并发：每次生成独立ID
- 幂等：同一次下游请求重试复用返回上下文，不跨心跳或fence复用
- 依赖：无

## platform.newExecutionLease

- 级别：参考
- 文件：`libs/platform/worker/loop.go`
- 作用：建立领取结果的本地到期边界

```go
func newExecutionLease(ctx context.Context, op OperationContext, client WorkerJobsClient, workerID string, job ClaimedJob, cancel CancelCauseFunc) (*ExecutionLease, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 任务取消上下文 | 仅可信进程内调用 |
| `op` | `OperationContext` | 原业务操作 | 仅可信进程内调用 |
| `client` | `WorkerJobsClient` | 可信租约客户端 | 仅可信进程内调用 |
| `workerID` | `string` | 本次worker | 仅可信进程内调用 |
| `job` | `ClaimedJob` | 真实完整领取材料 | 仅可信进程内调用 |
| `cancel` | `CancelCauseFunc` | 取消当前执行 | 仅可信进程内调用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `lease` | `ExecutionLease` | 本地有限执行权 | 仅可信进程内调用 |
| `error` | `error` | 失败原因 | 仅可信进程内调用 |

**步骤**

1. 解析真实claim截止，已过期或坏格式拒绝
2. 独立AfterFunc在执行权截止取消任务；不等待心跳RPC完成

**错误**

- `LOCKED`：本地执行权或权威租约已过期/维护安全点（不可重试）
- `CANCELLED`：任务取消或执行已停止（不可重试）

- 副作用：管理进程内执行权与有限IO上下文
- 授权：进程内可信worker调用
- 并发：续租与fresh检查串行；截止受独立互斥与定时器保护
- 幂等：不生成或重放业务副作用；每次控制RPC独立子操作
- 依赖：`platform.expireExecutionLease`

## platform.expireExecutionLease

- 级别：参考
- 文件：`libs/platform/worker/loop.go`
- 作用：到期时取消执行权

```go
func expireExecutionLease(lease *ExecutionLease)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `lease` | `ExecutionLease` | 当前执行权 | 仅可信进程内调用 |

**输出**

无

**步骤**

1. 持截止互斥重新核当前expires；只有当前已过期才取消，旧定时器回调不能取消已正确续期的执行

**错误**

- `LOCKED`：本地执行权或权威租约已过期/维护安全点（不可重试）
- `CANCELLED`：任务取消或执行已停止（不可重试）

- 副作用：管理进程内执行权与有限IO上下文
- 授权：进程内可信worker调用
- 并发：续租与fresh检查串行；截止受独立互斥与定时器保护
- 幂等：不生成或重放业务副作用；每次控制RPC独立子操作
- 依赖：无

## platform.executionLeaseContext

- 级别：参考
- 文件：`libs/platform/worker/loop.go`
- 作用：把控制IO截止限制在已知执行权内

```go
func executionLeaseContext(lease *ExecutionLease, ctx context.Context) (context.Context, CancelFunc)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `lease` | `ExecutionLease` | 当前执行权 | 仅可信进程内调用 |
| `ctx` | `context.Context` | 下游IO取消来源 | 仅可信进程内调用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 不超过当前租约截止的IO上下文 | 仅可信进程内调用 |
| `cancel` | `CancelFunc` | 释放IO资源 | 仅可信进程内调用 |

**步骤**

1. 持互斥读取expires并使用context.WithDeadline；保留父上下文更早取消

**错误**

- `LOCKED`：本地执行权或权威租约已过期/维护安全点（不可重试）
- `CANCELLED`：任务取消或执行已停止（不可重试）

- 副作用：管理进程内执行权与有限IO上下文
- 授权：进程内可信worker调用
- 并发：续租与fresh检查串行；截止受独立互斥与定时器保护
- 幂等：不生成或重放业务副作用；每次控制RPC独立子操作
- 依赖：无

## platform.renewExecutionLease

- 级别：参考
- 文件：`libs/platform/worker/loop.go`
- 作用：串行续租并在不可逆决定前核取消与fence

```go
func renewExecutionLease(ctx context.Context, lease *ExecutionLease, verify bool) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 下游IO取消 | 仅可信进程内调用 |
| `lease` | `ExecutionLease` | 当前执行权 | 仅可信进程内调用 |
| `verify` | `bool` | 不可逆决定前额外checkLease | 仅可信进程内调用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 未知、取消、过期或维护均失败关闭 | 仅可信进程内调用 |

**步骤**

1. 续租互斥覆盖整个请求与回执安装，RPC使用当前lease截止
2. 即时HeartbeatJob读取cancelRequested/safePointRequested；停止或未知则取消执行权并拒绝
3. 旧本地截止仍有效且新expiry有效时才重置独立到期定时器；不得复活已取消执行权
4. verify=true时继续按新截止读取CheckLease，绑定jobId/workerId/fence；RPC返回后再次核本地任务是否仍有效

**错误**

- `LOCKED`：本地执行权或权威租约已过期/维护安全点（不可重试）
- `CANCELLED`：任务取消或执行已停止（不可重试）

- 副作用：管理进程内执行权与有限IO上下文
- 授权：进程内可信worker调用
- 并发：续租与fresh检查串行；截止受独立互斥与定时器保护
- 幂等：不生成或重放业务副作用；每次控制RPC独立子操作
- 依赖：`jobs:heartbeatJob`、`jobs:checkLease`、`platform.executionLeaseContext`、`platform.workerControlContext`

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
5. 首次从排他序号0重读已持久事件；已succeeded调用也重读既有付费结果，不按当前lastSequence跳过
6. 超过4096事件返回BUDGET_EXCEEDED并取消，不静默截断

**错误**

- `CANCELLED`：已取消（不可重试）
- `DEPENDENCY_UNAVAILABLE`：网关失败（可重试）

- 副作用：HTTP调用网关
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每次调用独立
- 幂等：幂等键固定
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`、`platform.NewGatewayClient`

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
6. 读取trustedProxyCIDRs及逗号分隔NW_TRUSTED_PROXY_CIDRS；空值清空信任；最多32条规范IP网段，拒绝host位或IPv4映射CIDR；默认不信任任何代理
7. 读取可选masterKeyRef与NW_MASTER_KEY_REF覆盖；非空必须绝对路径，仅传递引用，不打开密钥；免费读取不强制主密钥可用

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
2. 拒credential/query/fragment/控制字符、连续斜杠和路径穿越，保留basePath；拒绝解码后连续斜杠，避免下游根相对链接成为协议相对URL

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

## platform.ValidatePrivatePath

- 级别：参考
- 文件：`libs/platform/lifecycle/config.go`
- 作用：ValidatePrivatePath公共边界

```go
func ValidatePrivatePath(path, kind string, allowMissing bool) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `path` | `string` | 绝对私有权威路径 | 可信装配 |
| `kind` | `string` | directory或regular-file | 可信装配 |
| `allowMissing` | `bool` | 仅首次创建可缺失尾部 | 可信装配 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全失败 | 失败无结果 |

**步骤**

1. 逐段Lstat拒链接与非目录，正规文件拒特殊文件及多硬链接；拒相对跳转
2. 仅allowMissing允许首次创建尚不存在路径；调用方在打开前和创建后核对，不改变现有路径语义

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
3. RuntimeView保留生产方真实revision/barrierId；revision缺失不得由0或时间伪造就绪。

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
4. RuntimeView保留生产方真实revision/barrierId；revision缺失不得由0或时间伪造就绪。

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺少site-config地址、不可达或响应不合法（可重试）

- 副作用：HTTP调用site-config
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：客户端可并发使用；缓存由ReadRuntime负责
- 幂等：只读
- 依赖：`site-config:readRuntimeState`、`platform.SignOutboundRequest`

## platform.NewWriteContext

- 级别：参考
- 文件：`libs/platform/lifecycle/runtime.go`
- 作用：共用后台IO运行权威、维护安全窗口与截止

```go
func NewWriteContext(ctx context.Context, config WriteWindowConfig) (context.Context, RuntimeView, context.CancelFunc, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 父IO与取消 | 保留更早截止 |
| `config` | `WriteWindowConfig` | 启动边界已校验回调 | runtime/writeWindow非nil；每IO重新读取 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `work` | `context.Context` | 本批有效IO窗口 | 最多30秒和更早安全截止 |
| `view` | `RuntimeView` | 本次权威快照 | 不能长期复用到后续确认 |
| `cancel` | `context.CancelFunc` | 关闭本次窗口 | 无error时必须调用 |
| `error` | `error` | 未知或窗口关闭 | 失败无可写ctx |

**步骤**

1. 运行权威及本域control窗口都未知则失败关闭，dataEpoch非空
2. 仅running/backup-barrier/restore-staging可进入，维护须明确未过安全截止
3. 限制父ctx/30秒/屏障更早截止，并更新已有OperationContext.deadline；不替域建立业务主体或猜epoch
4. 每外部IO后写本域确认前须重新调用，旧窗口不能替代当前权威

**错误**

- `DEPENDENCY_UNAVAILABLE`：运行权威未知（可重试）
- `LOCKED`：维护或恢复窗口关闭（可重试）

- 副作用：无
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：无

## platform.RunRecoveryLoop

- 级别：参考
- 文件：`libs/platform/lifecycle/runtime.go`
- 作用：以公共循环周期推进本域持久恢复决定

```go
func RunRecoveryLoop(ctx context.Context, config RecoveryLoopConfig) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程任务ctx | 由RunService创建取消 |
| `config` | `RecoveryLoopConfig` | 本域恢复装配 | poll100..60000ms；回调非nil |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 永久错误 | 进程取消正常nil，其他永久错误交还生命周期 |

**步骤**

1. 启动校验周期与回调；立即执行一批，之后每周期一批且不重叠
2. NewWriteContext核当前epoch/阶段/维护窗口；单批最多30秒，业务每次RPC前后仍核自己的权威
3. 仅显式retryable平台错误及批次deadline在下周期继续；未知或永久错误上报退出，持久决定不丢
4. 取消立即停止新批次，原ctx取消传下游IO；不建立新的模型/付费尝试

**错误**

- `VALIDATION`：装配参数缺失（不可重试）
- `INTEGRITY_ERROR`：本域回调永久完整性错误（不可重试）

- 副作用：无
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：`platform.NewWriteContext`

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

1. EventEnvelope逐条Marshal成原业务JSON，LastID为帧id、固定message事件名，不增加业务包装
2. 薄委托ServeJSONEventStream统一心跳/取消/写截止与背压；响应开始后不追加JSON Problem

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）
- `CANCELLED`：客户端断开或写截止不可用（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：`platform.ServeJSONEventStream`、`platform.envelopeJSONStream.Next`、`platform.envelopeJSONStream.Close`

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

## platform.ServeJSONEventStream

- 级别：参考
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：统一任意已登记业务JSON的SSE传输

```go
func ServeJSONEventStream(ctx context.Context, w http.ResponseWriter, stream JSONEventStream) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 请求取消 | 受信装配或调用边界 |
| `w` | `http.ResponseWriter` | SSE响应 | 受信装配或调用边界 |
| `stream` | `JSONEventStream` | 本域已核权限与持久续传的JSON流 | 受信装配或调用边界 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 流结束或安全IO失败 | 错误时结果无效 |

**步骤**

1. 最多一条待写JSON帧，保留域端ID与事件名；15秒注释心跳并flush；写截止取min(now+30秒,请求ctx更早截止)
2. 校id上限4096且无NUL/CR/LF、事件名合法标识符，data上限16MiB；json.Compact生成合法单行JSON，防SSE字段注入
3. 取消/结束关闭底层流归还资源，持久游标/授权/epoch由域端负责；响应开始后只结束连接不再追加JSON Problem
4. ctx主动取消时把底层写截止推进至now唤醒阻塞IO；截止设置与取消回调互斥，防未来截止覆盖取消；返回前停止并等取消回调，禁止响应结束后再碰writer

**错误**

- `VALIDATION`：输入不符合边界（不可重试）
- `DEPENDENCY_UNAVAILABLE`：基础依赖不可用（可重试）
- `INTEGRITY_ERROR`：持久规格或载荷损坏（不可重试）
- `CANCELLED`：客户端断开或写截止不可用（不可重试）
- `BUDGET_EXCEEDED`：单条JSON data超16MiB（不可重试）

- 副作用：无
- 授权：平台受信调用
- 并发：无共享可写业务状态，可并发
- 幂等：纯转换或按调用上下文执行，不改变业务费用根
- 依赖：无

## platform.envelopeJSONStream.Next

- 级别：参考
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：事件信封到JSON帧的薄适配

```go
func (s envelopeJSONStream) Next(ctx context.Context) (SSEFrame, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 请求取消 | 传给底层Next |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `frame` | `SSEFrame` | 原信封JSON帧 | 错误时无效 |
| `error` | `error` | 原流或编码错误 | 失败关闭 |

**步骤**

1. 底层Next读取一条，Marshal原信封并取同一条LastID，事件名固定message

**错误**

- `INTEGRITY_ERROR`：事件编码失败（不可重试）

- 副作用：无
- 授权：平台受信流装配
- 并发：由公共pump单goroutine串行读
- 幂等：只读序列或幂等关闭，不写业务数据
- 依赖：无

## platform.envelopeJSONStream.Close

- 级别：参考
- 文件：`libs/platform/apivalidate/validator.go`
- 作用：透传底层事件流关闭

```go
func (s envelopeJSONStream) Close() error
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 底层关闭结果 | 不追加业务写入 |

**步骤**

1. 只调用底层Close释放读取资源

**错误**

- `INTEGRITY_ERROR`：事件编码失败（不可重试）

- 副作用：无
- 授权：平台受信流装配
- 并发：由公共pump单goroutine串行读
- 幂等：只读序列或幂等关闭，不写业务数据
- 依赖：无

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
5. Application可选实现BackgroundApplication：完整构造成功后核唯一任务名/非空回调，由本进程统一启动后台；无RuntimeClient时不制造nil事件阻塞信号
6. 退出共用30秒预算：取消主ctx，BackgroundApplication.Drain停新HTTP并等请求、再等全部后台退出，最后Shutdown关闭私有资源；无法停止时保留恢复决定失败退出
7. 后台永久错误/无故提前返回停止服务，恢复切换保留退出75；不在服务入口自建timer/调度循环

**错误**

- `INTERNAL`：装配或运行失败（不可重试）

- 副作用：监听HTTP；启动后台任务
- 授权：permission：service；rule：库函数，在调用服务的进程内执行，权限由调用方决定
- 并发：每进程一次
- 幂等：重启按journal恢复
- 依赖：`platform.LoadServiceConfig`、`platform.WatchRuntime`
- 其他依赖说明：可选BackgroundApplication/BackgroundTask；既有Application服务仍兼容

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

## platform.CanonicalJSON

- 级别：参考
- 文件：`libs/platform/platform.go`
- 作用：统一跨服务引用摘要的纯JSON编码

```go
func CanonicalJSON(value any) ([]byte, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `value` | `any` | 已受入口约束的内部JSON可序列化值 | 调用方固定数组排序和空集[]，不得把secret当可公开hash输入 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `bytes` | `[]byte` | 规范JSON字节 | Go JSON编码；对象键排序；UseNumber保留整数精度，不声称RFC8785 |
| `error` | `error` | 不可规范序列化 | 不回显原值 |

**步骤**

1. Marshal后UseNumber转换通用值，再Marshal按对象键排序；保留Go JSON转义和数字约定
2. 空数组/空对象/空值由调用域预先明确；不替调用方排序数组

**错误**

- `INTEGRITY_ERROR`：内部值不能按约定编码（不可重试）

- 副作用：无
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：无

## platform.NormalizeIOError

- 级别：参考
- 文件：`libs/platform/platform.go`
- 作用：统一IO取消/截止与公共错误语义，未知错误只用可信安全说明

```go
func NormalizeIOError(err error, fallbackCode string, retryable bool, safeMessage string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 原IO错误或nil | 不输出原始错误文本 |
| `fallbackCode` | `string` | 可信调用方选择的登记错误码 | 静态代码提供，非请求输入 |
| `retryable` | `bool` | 未知错误的重试声明 | 已有公共错误与取消语义优先 |
| `safeMessage` | `string` | 可信安全常量说明 | 不得插入密码、密钥、正文、真实IP或私有路径 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | nil或公共platform.Error | 保留原公共错误；取消/截止统一；未知不带原文本 |

**步骤**

1. nil保持nil；已有或包裹的platform.Error原样保留
2. errors.Is识别context取消/截止，返回公共CANCELLED/DEADLINE_EXCEEDED且不自动重试
3. 其余未知错误使用可信fallbackCode/retryable/safeMessage，不保留原错误文本或调用栈

**错误**

- 无

- 副作用：无
- 授权：公共纯工具，只接可信参数，不增加业务授权
- 并发：无可变共享状态
- 幂等：纯映射
- 依赖：无

## platform.NewJobsClient

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：装配真实任务六端口

```go
func NewJobsClient(client RawJobsClient, validators ResponseValidators) (WorkerJobsClient, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `client` | `RawJobsClient` | 真实生成客户端 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `validators` | `ResponseValidators` | 六生产方成功schema | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `client` | `WorkerJobsClient` | 窄机制端口 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 启动检查真实客户端和六Result校验器齐全，复制校验器map避免外部替换；不执行网络或业务操作

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.ClaimJob`、`platform.StartJob`、`platform.HeartbeatJob`、`platform.CheckLease`、`platform.CompleteJob`、`platform.FailJob`

## platform.workerOperation

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：取得平台任务控制上下文

```go
func workerOperation(ctx context.Context) (OperationContext, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信任务控制上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `op` | `OperationContext` | 本次可信控制 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 缺少已装配op、operationId或当前epoch失败关闭，不从请求材料猜授权

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：无

## platform.decodeWorkerResponse

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：按生产方schema校验一次成功响应

```go
func (c *jobsClient) decodeWorkerResponse(schema string, raw json.RawMessage, result any) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `schema` | `string` | 六固定schema之一 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `raw` | `bytes` | 原始成功JSON | 仅可信装配或已校验机制链；错误不返回半结果 |
| `result` | `any` | 本方法投影目标 | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 先运行启动提供的严格生产方schema，再解码本方法机制投影；失败保留安全错误，不使用部分结果

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：无

## platform.ClaimJob

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：真实jobs ClaimJob机制适配

```go
func (c *jobsClient) ClaimJob(ctx context.Context, owner, workerID string, limit int) ([]ClaimedJob, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信任务控制上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `owner` | `string` | 本域owner | 仅可信装配或已校验机制链；错误不返回半结果 |
| `workerID` | `string` | 当前worker | 仅可信装配或已校验机制链；错误不返回半结果 |
| `limit` | `int` | 有界数量 | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `jobs` | `[]ClaimedJob` | 完整映射 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 严格ClaimJobRequestBody仅发送owner/workerId/limit；epoch只经可信上下文头传递，幂等键为本次op
2. 严格ClaimJobResult后核owner/epoch、并行fencingTokens一致及唯一jobId，完整映射job.operationId/root/inputRef/attempt/expiresAt；不得截断多领任务

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：仅执行版本化jobs RPC，不在本库持久业务状态
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.workerOperation`、`platform.decodeWorkerResponse`、`jobs:claimJob`

## platform.StartJob

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：真实jobs StartJob机制适配

```go
func (c *jobsClient) StartJob(ctx context.Context, jobID, workerID string, fencingToken int64) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信任务控制上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `jobID` | `string` | 原任务 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `workerID` | `string` | 原worker | 仅可信装配或已校验机制链；错误不返回半结果 |
| `fencingToken` | `int64` | 原fence | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 发送完整job/worker/fence/epoch且幂等键为本次op；生产方schema后必须state=running
2. 遵守生产方HTTP RequestBody：jobId仅路径、dataEpoch仅可信上下文头，body不重复领域Input的jobId/dataEpoch；lease body仅workerId/fencingToken，终局再加实际结果/错误与paid refs

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：仅执行版本化jobs RPC，不在本库持久业务状态
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.workerOperation`、`platform.decodeWorkerResponse`、`jobs:startJob`

## platform.HeartbeatJob

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：真实jobs HeartbeatJob机制适配

```go
func (c *jobsClient) HeartbeatJob(ctx context.Context, jobID, workerID string, fencingToken int64) (HeartbeatInfo, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信任务控制上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `jobID` | `string` | 原任务 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `workerID` | `string` | 原worker | 仅可信装配或已校验机制链；错误不返回半结果 |
| `fencingToken` | `int64` | 原fence | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `info` | `HeartbeatInfo` | 到期/取消/安全点 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 完整控制身份及epoch，严格响应保留三字段，不伪造新到期；独立本地租约截止仍由循环约束
2. 遵守生产方HTTP RequestBody：jobId仅路径、dataEpoch仅可信上下文头，body不重复领域Input的jobId/dataEpoch；lease body仅workerId/fencingToken，终局再加实际结果/错误与paid refs

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：仅执行版本化jobs RPC，不在本库持久业务状态
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.workerOperation`、`platform.decodeWorkerResponse`、`jobs:heartbeatJob`

## platform.CheckLease

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：真实jobs CheckLease机制适配

```go
func (c *jobsClient) CheckLease(ctx context.Context, jobID, workerID string, fencingToken int64) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信任务控制上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `jobID` | `string` | 原任务 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `workerID` | `string` | 原worker | 仅可信装配或已校验机制链；错误不返回半结果 |
| `fencingToken` | `int64` | 原fence | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `valid` | `bool` | 真实valid | 仅可信装配或已校验机制链；错误不返回半结果 |
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 以登记query workerId/fencingToken调用原任务路径，当前epoch由可信操作头；严格响应返回真实valid

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：仅执行版本化jobs RPC，不在本库持久业务状态
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.workerOperation`、`platform.decodeWorkerResponse`、`jobs:checkLease`

## platform.CompleteJob

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：真实jobs CompleteJob机制适配

```go
func (c *jobsClient) CompleteJob(ctx context.Context, jobID, workerID string, fencingToken int64, result *ObjectRef, paidResultRefs []Identifier) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信任务控制上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `jobID` | `string` | 原任务 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `workerID` | `string` | 原worker | 仅可信装配或已校验机制链；错误不返回半结果 |
| `fencingToken` | `int64` | 原fence | 仅可信装配或已校验机制链；错误不返回半结果 |
| `result` | `*ObjectRef` | 可选真实结果 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `paidResultRefs` | `[]Identifier` | 已有付费结果 | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 发送完整job/worker/fence/epoch、可选真实result及非null paidResultRefs，幂等键为本次op；生产方成功schema后必须succeeded
2. 遵守生产方HTTP RequestBody：jobId仅路径、dataEpoch仅可信上下文头，body不重复领域Input的jobId/dataEpoch；lease body仅workerId/fencingToken，终局再加实际结果/错误与paid refs

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：仅执行版本化jobs RPC，不在本库持久业务状态
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.workerOperation`、`platform.decodeWorkerResponse`、`jobs:completeJob`

## platform.FailJob

- 级别：参考
- 文件：`libs/platform/worker/client.go`
- 作用：真实jobs FailJob机制适配

```go
func (c *jobsClient) FailJob(ctx context.Context, jobID, workerID string, fencingToken int64, errorCode string, paidResultRefs []Identifier) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信任务控制上下文 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `jobID` | `string` | 原任务 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `workerID` | `string` | 原worker | 仅可信装配或已校验机制链；错误不返回半结果 |
| `fencingToken` | `int64` | 原fence | 仅可信装配或已校验机制链；错误不返回半结果 |
| `errorCode` | `string` | 公共安全码 | 仅可信装配或已校验机制链；错误不返回半结果 |
| `paidResultRefs` | `[]Identifier` | 已有付费结果 | 仅可信装配或已校验机制链；错误不返回半结果 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全错误 | 仅可信装配或已校验机制链；错误不返回半结果 |

**步骤**

1. 发送完整控制身份、epoch、安全错误码和已有付费引用；严格成功响应只接cancelled/orphan/retry-wait/dead-letter，不冒称再次执行或释放费用
2. 遵守生产方HTTP RequestBody：jobId仅路径、dataEpoch仅可信上下文头，body不重复领域Input的jobId/dataEpoch；lease body仅workerId/fencingToken，终局再加实际结果/错误与paid refs

**错误**

- `DEPENDENCY_UNAVAILABLE`：缺可信上下文或生产方响应矛盾（不可重试）
- `INTEGRITY_ERROR`：生产方成功响应不符schema或语义绑定（不可重试）

- 副作用：仅执行版本化jobs RPC，不在本库持久业务状态
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.workerOperation`、`platform.decodeWorkerResponse`、`jobs:failJob`

## platform.NewGatewayClient

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：启动要求CreateInvocationResult/CancelInvocationResult/StreamEvent校验器并复制映射；不执行RPC

```go
func NewGatewayClient(raw RawGatewayClient, validators GatewayResponseValidators) (GatewayClient, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `raw` | `RawGatewayClient` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `validators` | `GatewayResponseValidators` | 可信装配或已校验输入 | 不接受浏览器身份 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `client` | `GatewayClient` | 严格生产方合同结果 | 成功时有效 |
| `error` | `error` | 安全公共错误 | 不包含原始输入 |

**步骤**

1. 启动要求CreateInvocationResult/CancelInvocationResult/StreamEvent校验器并复制映射；不执行RPC

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`platform.CreateInference`、`platform.CancelInference`、`platform.ResumeInference`

## platform.decodeInvocation

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：按生产方严格schema核响应再解析invocation

```go
func (c *gatewayClient) decodeInvocation(name string, raw JSONBytes) (Invocation, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `name` | `string` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `raw` | `JSONBytes` | 可信装配或已校验输入 | 不接受浏览器身份 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `invocation` | `Invocation` | 严格生产方合同结果 | 成功时有效 |
| `error` | `error` | 安全公共错误 | 不包含原始输入 |

**步骤**

1. 按生产方严格schema核响应再解析invocation

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：无

## platform.CreateInference

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：幂等键入生产方body；三空数组显式[]；按真实HTTP客户端发出，不重试

```go
func (c *gatewayClient) CreateInference(ctx context.Context, key string, input CreateInvocationInput) (Invocation, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `key` | `string` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `input` | `CreateInvocationInput` | 可信装配或已校验输入 | 不接受浏览器身份 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `invocation` | `Invocation` | 严格生产方合同结果 | 成功时有效 |
| `error` | `error` | 安全公共错误 | 不包含原始输入 |

**步骤**

1. 幂等键入生产方body；三空数组显式[]；按真实HTTP客户端发出，不重试
2. 响应严格校验，再核rootOperationId/costClass与可信op一致

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`model-gateway:createInference`、`platform.decodeInvocation`

## platform.CancelInference

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：生产方body仅invocationId/reason/idempotencyKey，路径与body一致

```go
func (c *gatewayClient) CancelInference(ctx context.Context, key string, id string, reason string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `key` | `string` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `id` | `string` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `reason` | `string` | 可信装配或已校验输入 | 不接受浏览器身份 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全公共错误 | 不包含原始输入 |

**步骤**

1. 生产方body仅invocationId/reason/idempotencyKey，路径与body一致
2. 严格响应校验并核取消对象

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`model-gateway:cancelInference`、`platform.decodeInvocation`

## platform.ResumeInference

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：afterSequence为排他游标；limit100

```go
func (c *gatewayClient) ResumeInference(ctx context.Context, id string, after int64) (ModelEventStream, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `id` | `string` | 可信装配或已校验输入 | 不接受浏览器身份 |
| `after` | `int64` | 可信装配或已校验输入 | 不接受浏览器身份 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `stream` | `ModelEventStream` | 严格生产方合同结果 | 成功时有效 |
| `error` | `error` | 安全公共错误 | 不包含原始输入 |

**步骤**

1. afterSequence为排他游标；limit100
2. 生成流必须提供NextRaw，平台先严格schema后解码；缺端口关闭流并失败

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：`model-gateway:resumeInference`、`platform.validatedModelNext`、`platform.validatedModelClose`、`platform.validatedModelLastID`

## platform.validatedModelNext

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：原始JSON先严格schema校验，再解析；对象与流invocationId一致

```go
func (s *validatedModelStream) Next(ctx context.Context) (StreamEvent, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 可信装配或已校验输入 | 不接受浏览器身份 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `event` | `StreamEvent` | 严格生产方合同结果 | 成功时有效 |
| `error` | `error` | 安全公共错误 | 不包含原始输入 |

**步骤**

1. 原始JSON先严格schema校验，再解析；对象与流invocationId一致
2. output仅completed，向量空间维度一致且有限，重排与嵌入数据不得混合

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：无

## platform.validatedModelClose

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：关闭底层SSE

```go
func (s *validatedModelStream) Close() error
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 安全公共错误 | 不包含原始输入 |

**步骤**

1. 关闭底层SSE

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：无

## platform.validatedModelLastID

- 级别：参考
- 文件：`libs/platform/modelclient/client.go`
- 作用：保留生产方SSE续传标识

```go
func (s *validatedModelStream) LastID() string
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `id` | `string` | 最后SSE事件标识 | 由底层流产生 |

**步骤**

1. 原样返回底层流LastID，不改写序号

**错误**

- `VALIDATION`：客户端或六个校验器缺失（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接可信启动配置/平台上下文；域负责真实任务对象授权；不接收浏览器身份
- 并发：无共享可变业务状态；每次RPC独立上下文
- 幂等：控制写入使用本次可信op.OperationID；重传复用相同调用，不自行重试
- 依赖：无

## platform.SignAdmissionPermit

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：签完整费用许可，固定用途与audience，清token后规范JSON，期限不超过五分钟

```go
func SignAdmissionPermit(key SigningKey,permit AdmissionPermit)(AdmissionPermit,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 可信装配输入 | 不接受客户端自报授权 |
| `permit` | `AdmissionPermit` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `AdmissionPermit` | 成功结果或安全错误 | 失败不返回半结果 |
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 签完整费用许可，固定用途与audience，清token后规范JSON，期限不超过五分钟

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：`platform.validateAdmissionClaims`、`platform.signGuardClaims`
- 其他依赖说明：受控Ed25519密钥引用

## platform.VerifyAdmissionPermit

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：只信traffic-guard公钥与固定用途；代次/归属/持久消费由调用域负责

```go
func VerifyAdmissionPermit(anchors TrustAnchors,token string)(AdmissionPermit,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 可信装配输入 | 不接受客户端自报授权 |
| `token` | `string` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `AdmissionPermit` | 成功结果或安全错误 | 失败不返回半结果 |
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 只信traffic-guard公钥与固定用途；代次/归属/持久消费由调用域负责

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：`platform.validateAdmissionClaims`、`platform.verifyGuardClaims`
- 其他依赖说明：受控Ed25519密钥引用

## platform.validateAdmissionClaims

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：核许可ID、作用域、固定audience和五分钟期限

```go
func validateAdmissionClaims(permit AdmissionPermit)error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `permit` | `AdmissionPermit` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 核许可ID、作用域、固定audience和五分钟期限

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

## platform.SignGuardSession

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：固定portal与独立session用途，最多十分钟挑战通过证明

```go
func SignGuardSession(key SigningKey,claims GuardSessionClaims)(string,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 可信装配输入 | 不接受客户端自报授权 |
| `claims` | `GuardSessionClaims` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `string` | 成功结果或安全错误 | 失败不返回半结果 |
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 固定portal与独立session用途，最多十分钟挑战通过证明

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：`platform.validateGuardSession`、`platform.signGuardClaims`
- 其他依赖说明：受控Ed25519密钥引用

## platform.VerifyGuardSession

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：严格验签挑战证明与期限，不授予账号或模型权限

```go
func VerifyGuardSession(anchors TrustAnchors,token string)(GuardSessionClaims,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 可信装配输入 | 不接受客户端自报授权 |
| `token` | `string` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `GuardSessionClaims` | 成功结果或安全错误 | 失败不返回半结果 |
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 严格验签挑战证明与期限，不授予账号或模型权限

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：`platform.validateGuardSession`、`platform.verifyGuardClaims`
- 其他依赖说明：受控Ed25519密钥引用

## platform.validateGuardSession

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：核portal用途、主体/站点/会话/代次与十分钟期限

```go
func validateGuardSession(claims GuardSessionClaims)error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `claims` | `GuardSessionClaims` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 核portal用途、主体/站点/会话/代次与十分钟期限

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

## platform.signGuardClaims

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：加载平台私钥，规范JSON，以带kid/alg/typ的EdDSA JWS签名

```go
func signGuardClaims(key SigningKey,purpose string,claims any)(string,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 可信装配输入 | 不接受客户端自报授权 |
| `purpose` | `string` | 可信装配输入 | 不接受客户端自报授权 |
| `claims` | `any` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result0` | `string` | 成功结果或安全错误 | 失败不返回半结果 |
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 加载平台私钥，规范JSON，以带kid/alg/typ的EdDSA JWS签名

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：`platform.LoadSigningKey`、`platform.CanonicalJSON`
- 其他依赖说明：受控Ed25519密钥引用

## platform.verifyGuardClaims

- 级别：参考
- 文件：`libs/platform/serviceauth/admission.go`
- 作用：8192字节上限；固定用途/算法/traffic-guard key，先验签再严格解码

```go
func verifyGuardClaims(anchors TrustAnchors,token,purpose string,result any)error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `anchors` | `TrustAnchors` | 可信装配输入 | 不接受客户端自报授权 |
| `token` | `string` | 可信装配输入 | 不接受客户端自报授权 |
| `purpose` | `string` | 可信装配输入 | 不接受客户端自报授权 |
| `result` | `any` | 可信装配输入 | 不接受客户端自报授权 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功结果或安全错误 | 失败不返回半结果 |

**步骤**

1. 8192字节上限；固定用途/算法/traffic-guard key，先验签再严格解码

**错误**

- `VALIDATION`：凭证或配置不符合上述规则（不可重试）
- `UNAUTHENTICATED`：凭证或配置不符合上述规则（不可重试）
- `DEPENDENCY_UNAVAILABLE`：凭证或配置不符合上述规则（不可重试）

- 副作用：只读取外部私钥引用并在内存计算；不记录凭证或原始地址
- 授权：permission：service；rule：可信进程内机制；准入/当前代次/消费由调用方负责
- 并发：无共享可变查询状态；密钥缓存由LoadSigningKey统一管理
- 幂等：同key/scope/payload生成相同游标；游标只用于查询，不作为授权凭据
- 依赖：无
- 其他依赖说明：受控Ed25519密钥引用

## platform.NewEventConsumer

- 级别：参考
- 文件：`libs/platform/inbox/client.go`
- 作用：事件消费内部实际助手：NewEventConsumer

```go
func NewEventConsumer(raw RawEventClient,validateAck func(json.RawMessage)error)(EventConsumerClient,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：`platform.eventConsumer.SubscribeEvents`、`platform.eventConsumer.AcknowledgeEvent`

## platform.eventConsumer.SubscribeEvents

- 级别：参考
- 文件：`libs/platform/inbox/client.go`
- 作用：事件消费内部实际助手：eventConsumer.SubscribeEvents

```ts
func(c *eventConsumer) SubscribeEvents(ctx context.Context,consumerID,lastID string)(EventStream,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.eventConsumer.AcknowledgeEvent

- 级别：参考
- 文件：`libs/platform/inbox/client.go`
- 作用：事件消费内部实际助手：eventConsumer.AcknowledgeEvent

```ts
func(c *eventConsumer) AcknowledgeEvent(ctx context.Context,consumerID,eventID string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按可信本域配置执行此签名职责；具体失败关闭、当前epoch、真实确认及计数规则见inbox源码及RunConsumerWithResult合同。

**错误**

- `DEPENDENCY_UNAVAILABLE`：真实运行权威或端口缺失（可重试）

- 副作用：无
- 授权：permission：service；rule：仅受信进程装配；不接受浏览器自声明
- 并发：仅运行时自己的分片与原子计数
- 幂等：确认沿consumerId/eventId稳定幂等键
- 依赖：无

## platform.NewTaskGroup

- 级别：参考
- 文件：`libs/platform/lifecycle/task_group.go`
- 作用：构造进程任务集合，不执行回调

```go
func NewTaskGroup(parent context.Context, maxConcurrent int) (*TaskGroup, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只管理本进程已接受任务；Start固定输入ctx原deadline和值并脱离HTTP取消，进程取消/显式Cancel/原deadline仍终止；同key在途去重、满并发真实限流；关闭后不再接受，等待全部回调退出。回调负责本域持久终态与费用，回调业务error不使其它任务取消或引起重派。

**错误**

- `VALIDATION`：装配参数缺失（不可重试）
- `INTEGRITY_ERROR`：本域回调永久完整性错误（不可重试）

- 副作用：本进程goroutine与取消信号，不读写业务表或费用账
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：`platform.TaskGroup.stop`

## platform.TaskGroup.Start

- 级别：参考
- 文件：`libs/platform/lifecycle/task_group.go`
- 作用：按唯一key启动已持久接受的有界任务

```go
func (g *TaskGroup) Start(ctx context.Context, key string, run func(context.Context) error) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只管理本进程已接受任务；Start固定输入ctx原deadline和值并脱离HTTP取消，进程取消/显式Cancel/原deadline仍终止；同key在途去重、满并发真实限流；关闭后不再接受，等待全部回调退出。回调负责本域持久终态与费用，回调业务error不使其它任务取消或引起重派。

**错误**

- `VALIDATION`：装配参数缺失（不可重试）
- `INTEGRITY_ERROR`：本域回调永久完整性错误（不可重试）

- 副作用：本进程goroutine与取消信号，不读写业务表或费用账
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：无

## platform.TaskGroup.Cancel

- 级别：参考
- 文件：`libs/platform/lifecycle/task_group.go`
- 作用：只取消指定在途任务

```go
func (g *TaskGroup) Cancel(key string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 只管理本进程已接受任务；Start固定输入ctx原deadline和值并脱离HTTP取消，进程取消/显式Cancel/原deadline仍终止；同key在途去重、满并发真实限流；关闭后不再接受，等待全部回调退出。回调负责本域持久终态与费用，回调业务error不使其它任务取消或引起重派。

**错误**

- `VALIDATION`：装配参数缺失（不可重试）
- `INTEGRITY_ERROR`：本域回调永久完整性错误（不可重试）

- 副作用：本进程goroutine与取消信号，不读写业务表或费用账
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：无

## platform.TaskGroup.Run

- 级别：参考
- 文件：`libs/platform/lifecycle/task_group.go`
- 作用：接进程后台生命周期，停止并等待全部任务

```go
func (g *TaskGroup) Run(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 只管理本进程已接受任务；Start固定输入ctx原deadline和值并脱离HTTP取消，进程取消/显式Cancel/原deadline仍终止；同key在途去重、满并发真实限流；关闭后不再接受，等待全部回调退出。回调负责本域持久终态与费用，回调业务error不使其它任务取消或引起重派。

**错误**

- `VALIDATION`：装配参数缺失（不可重试）
- `INTEGRITY_ERROR`：本域回调永久完整性错误（不可重试）

- 副作用：本进程goroutine与取消信号，不读写业务表或费用账
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：`platform.TaskGroup.stop`

## platform.TaskGroup.Drain

- 级别：参考
- 文件：`libs/platform/lifecycle/task_group.go`
- 作用：拒绝新任务、取消在途并有界等待全部退出

```go
func (g *TaskGroup) Drain(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 只管理本进程已接受任务；Start固定输入ctx原deadline和值并脱离HTTP取消，进程取消/显式Cancel/原deadline仍终止；同key在途去重、满并发真实限流；关闭后不再接受，等待全部回调退出。回调负责本域持久终态与费用，回调业务error不使其它任务取消或引起重派。

**错误**

- `VALIDATION`：装配参数缺失（不可重试）
- `INTEGRITY_ERROR`：本域回调永久完整性错误（不可重试）

- 副作用：本进程goroutine与取消信号，不读写业务表或费用账
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：`platform.TaskGroup.stop`

## platform.TaskGroup.stop

- 级别：参考
- 文件：`libs/platform/lifecycle/task_group.go`
- 作用：内部一次关闭接收并取消、排空

```go
func (g *TaskGroup) stop()
```

**输入**

无

**输出**

无

**步骤**

1. 只管理本进程已接受任务；Start固定输入ctx原deadline和值并脱离HTTP取消，进程取消/显式Cancel/原deadline仍终止；同key在途去重、满并发真实限流；关闭后不再接受，等待全部回调退出。回调负责本域持久终态与费用，回调业务error不使其它任务取消或引起重派。

**错误**

- `VALIDATION`：装配参数缺失（不可重试）
- `INTEGRITY_ERROR`：本域回调永久完整性错误（不可重试）

- 副作用：本进程goroutine与取消信号，不读写业务表或费用账
- 授权：平台受信装配；不读业务秘密，不授予业务权限
- 并发：不产生并发本域业务批次；IO只用传入ctx
- 幂等：纯转换或由本域持久决定保证恢复幂等，平台不重算付费结果
- 依赖：无
