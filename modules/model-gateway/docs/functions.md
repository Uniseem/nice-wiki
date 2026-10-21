# model-gateway 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 250 个函数：约束 45 个，参考 205 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## model-gateway.LoadRuntime

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：启动并验证本服务私有持久化与密钥引用

```go
func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `LoadRuntimeInput` | 封闭业务输入；逐字段见types $defs/LoadRuntimeInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadRuntimeResult` | 成功结果；逐字段见types $defs/LoadRuntimeResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只读取本服务配置和迁移版本
2. masterKeyRef可缺省；非空须绝对路径，仅装配引用，不在启动读secret，凭据写入时安全有界读取32字节
3. 注册恢复调度
4. 所有关键依赖可用才ready
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：建立本域持久化并装配可信主密钥引用，不读取secret或调用供应方
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.OpenDatabase`、`model-gateway.InstallStore`

## model-gateway.OpenDatabase

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：打开私有active库并执行严格只追加迁移

```go
func OpenDatabase(dataDirectory string) (*Store, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `dataDirectory` | `string` | 私有部署目录 | 可信本域装配或已通过入口的只读链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `Store` | 本域句柄 | 错误时不使用结果；不暴露私有配置原值 |
| `err` | `error` | 错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 仅建立本域active/control/restore目录，WAL/外键/单连接
2. 校验全部已应用迁移版本再逐事务执行未应用迁移
3. 失败关闭数据库，不安装未知句柄
4. 只有O_EXCL真实新建active/model-gateway.db文件才能产database_origin；拒未知active文件/孤立WAL/符号链接，旧库无来源不推首次空历史。
5. control/database-origin.json以O_EXCL真实创建证书并文件/目录fsync；原控制证书存在而active主库缺失时维护拒绝，不因重新新建文件推历史从未接受。首次迁移中断而来源未完成仍失败关闭，不清控制事实。

**错误**

- `VALIDATION`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）
- `DEPENDENCY_UNAVAILABLE`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（可重试）
- `INTEGRITY_ERROR`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）
- `INTERNAL`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）

- 副作用：建立本域目录并打开SQLite，执行只追加迁移和平台outbox建表
- 授权：permission：service；rule：仅本域可信装配或ReadTaskAvailability已核纯QA服务链路
- 并发：启动时单连接迁移；错误逆序关闭资源
- 幂等：已应用编号/名称不重复迁移，未知版本拒绝
- 依赖：`model-gateway.Store.runMigrations`

## model-gateway.CurrentStore

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：读取进程已安装Store

```go
func CurrentStore() *Store
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `Store` | 当前安装句柄或nil | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 只在进程RWMutex内取指针，不持锁执行查询

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域可信启动装配或已通过各自授权的任务/档位/凭据配置读写链路；不暴露私有句柄给外域。
- 并发：进程RWMutex仅取当前Store指针，无SQL或网络
- 幂等：无写入，可重复
- 依赖：无

## model-gateway.InstallStore

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：安装既有本域Store供业务读取

```go
func InstallStore(store *Store, masterKeyRef string)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `Store` | 可信启动已打开的本域库 | 可信本域装配或已通过入口的只读链路使用 |
| `masterKeyRef` | `string` | 可信启动可选主密钥绝对路径引用 | 只装配引用，不读取文件；空引用不阻断免费配置读取 |

**输出**

无

**步骤**

1. 仅启动装配在安装前保存主密钥引用，再设置当前Store；不读取secret、不写runtime_meta、不宣称服务ready。

**错误**

- 无

- 副作用：更新进程内当前Store指针
- 授权：permission：service；rule：仅本域可信启动装配或已通过各自授权的任务/档位/凭据配置读写链路；不暴露私有句柄给外域。
- 并发：仅可信启动装配持进程写锁设置指针，不持锁执行IO
- 幂等：重复安装同指针不改持久数据；不证明服务ready
- 依赖：无

## model-gateway.Store.BeginTx

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：在未关闭Store开始事务

```go
func (s *Store) BeginTx(ctx context.Context) (*sql.Tx, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消和截止 | 可信本域装配或已通过入口的只读链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tx` | `SQLTransaction` | 本域事务 | 错误时不使用结果；不暴露私有配置原值 |
| `err` | `error` | 错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 核Store未关闭后调用已有SQL连接BeginTx；上下文贯穿

**错误**

- `VALIDATION`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）
- `DEPENDENCY_UNAVAILABLE`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（可重试）
- `CANCELLED`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）
- `DEADLINE_EXCEEDED`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）
- `INTERNAL`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）

- 副作用：建立本域SQL事务
- 授权：permission：service；rule：仅本域可信启动装配或已通过各自授权的任务/档位/凭据配置读写链路；不暴露私有句柄给外域。
- 并发：先短读锁核关闭状态，再SQL BeginTx；ctx贯穿查询
- 幂等：无业务写入；事务由调用方结束
- 依赖：无

## model-gateway.Store.Close

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：关闭并撤销当前安装Store

```go
func (s *Store) Close() error
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 关闭错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 幂等标关闭；只撤销同一指针，再关闭数据库

**错误**

- `INTERNAL`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）

- 副作用：幂等关闭数据库并撤销同一已安装指针
- 授权：permission：service；rule：仅本域可信启动装配或已通过各自授权的任务/档位/凭据配置读写链路；不暴露私有句柄给外域。
- 并发：Store写锁标关闭后释放，进程写锁仅撤销同指针，然后关闭SQL连接
- 幂等：重复关闭不重复撤销或关闭资源
- 依赖：无

## model-gateway.Store.runMigrations

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：执行唯一embed来源只追加迁移

```go
func (s *Store) runMigrations() error
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 迁移错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 调用All核编号连续，读取全部已应用版本/名称并拒绝未知
2. 每个未应用迁移单独事务执行和登记；初始runtime_meta保留未知空代次
3. 唯一调用平台outbox.EnsureSchema；不创建默认配置或种子凭据

**错误**

- `INTEGRITY_ERROR`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）
- `INTERNAL`：既有私有存储装配/IO或迁移失败，输出脱敏公共错误（不可重试）

- 副作用：事务执行未应用SQL迁移、迁移回执与初始空代次单例；平台outbox表
- 授权：permission：service；rule：仅本域可信装配或ReadTaskAvailability已核纯QA服务链路
- 并发：仅可信启动单连接执行迁移，版本未知先拒绝；无网络IO
- 幂等：编号/名称匹配已应用项跳过，未应用项事务提交
- 依赖：`model-gateway.All`

## model-gateway.Store.MasterKeyRef

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：读取可信不可变主密钥引用

```go
func (s *Store) MasterKeyRef() string
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ref` | `string` | 启动装配引用 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. 只返回可信InstallStore保存的路径引用，不读文件或secret。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：无

## model-gateway.Store.SetDataEpoch

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/bootstrap.go`
- 作用：Store.SetDataEpoch本域真实来源和隔离数据回调

```go
func (s *Store) SetDataEpoch(ctx context.Context, dataEpoch platform.Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 权威runtime首次绑定真实database_origin，原事务写fresh_activation完整tracking和credential_tracking；同代重复无写，异代必须走认证stage，不把旧库清表或重基。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.ValidateProvider

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/provider.go`
- 作用：验证供应方配置

```go
func ValidateProvider(ctx context.Context, op OperationContext, input ValidateProviderInput) (ValidateProviderResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateProviderInput` | 封闭业务输入；逐字段见types $defs/ValidateProviderInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateProviderResult` | 成功结果；逐字段见types $defs/ValidateProviderResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 解析URL并按管理员可信network allowlist验证目标
2. 外部供应方仅HTTPS，显式本地部署allowlist可HTTP且不允许重定向到其他网段
3. 拒绝URL userinfo/fragment及危险附加Authorization重复头
4. 固定连接/总/流空闲超时。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## model-gateway.PutProvider

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/provider.go`
- 作用：CAS保存供应方

```go
func PutProvider(ctx context.Context, op OperationContext, input PutProviderInput) (PutProviderResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutProviderInput` | 封闭业务输入；逐字段见types $defs/PutProviderInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutProviderResult` | 成功结果；逐字段见types $defs/PutProviderResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仍只portal当前superadmin；同代provider版本/credential指针CAS，原完整幂等结果先于新写屏障返回
2. secretHeaders缺失保留原映射，显式[]清空；Provider内names/refs只读兼容，不采信客户端refs或读取本机路径
3. 非空secretHeaders在SQL外用部署主密钥和域分离HMAC生成保密输入摘要；AES-GCM每项随机nonce/AAD绑定同provider/version/name/ref/epoch
4. 短事务CAS后写不可变header_secrets、服务器生成的不透明refs、provider.changed及仅含安全结果的resultHash原回执，不返回明文
5. 旧attempt继续原固定refs，密文不因轮换或表暂空销毁；其他provider启停/配置职责不变

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）

- 副作用：更新providers并发出provider.changed
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：同一当前代次SQL事务先核有效原结果，再核所有非released屏障及provider版本/代次/active指针；条件UPSERT及RowsAffected核对，配置、平台outbox和回执一起提交，不做网络IO。
- 幂等：siteId+actorId+函数ID+key唯一、7天有效；普通配置hash绑定callerService/dataEpoch/provider/expectedVersion，同代同hash返回首次规范结果，异hash CONFLICT。过期仅成功新配置事务替换，不因后续屏障/指针阻断有效无副作用重放。
- 依赖：`model-gateway.ValidateProvider`、`model-gateway.CurrentStore`、`model-gateway.beginAvailabilitySnapshot`、`model-gateway.readProviderReceipt`、`model-gateway.PrepareHeaderSecrets`、`model-gateway.SaveHeaderSecrets`

## model-gateway.readProviderReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/provider.go`
- 作用：只读7天渠道配置原结果并核同代普通输入摘要

```go
func readProviderReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, key string, inputHash platform.Hash, expected Provider) (PutProviderResult, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 原入口取消 | 已授权PutProvider当前同代事务 |
| `op` | `OperationContext` | 可信范围 | 已授权PutProvider当前同代事务 |
| `tx` | `SQLTransaction` | 当前事务 | 已授权PutProvider当前同代事务 |
| `key` | `string` | 幂等键 | 已授权PutProvider当前同代事务 |
| `inputHash` | `Hash` | 含caller和epoch摘要 | 已授权PutProvider当前同代事务 |
| `expected` | `Provider` | URL规范化首次结果 | 已授权PutProvider当前同代事务 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutProviderResult` | 原结果 | 失败结果不可使用；不输出私有原值 |
| `found` | `bool` | 是否命中有效原回执 | 失败结果不可使用；不输出私有原值 |
| `err` | `error` | 脱敏公共错误 | 失败结果不可使用；不输出私有原值 |

**步骤**

1. 同scope/函数/key只读hash/created_at/result_json；坏时间失败，过期未命中不写。
2. 有效hash不匹配CONFLICT；完整规范结果编码必须与持久原结果精确一致，否则INTEGRITY_ERROR。
3. 命中先返原结果，不做后续写屏障/当前指针/CAS校验；仅上层成功新事务替换过期键。

**错误**

- `CONFLICT`：幂等输入不一致（不可重试）
- `INTEGRITY_ERROR`：持久模型能力或回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域查询未知（可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅PutProvider已核superadmin的同代事务链路。
- 并发：共用PutProvider的同代SQL事务，只读原回执，不独立提交。
- 幂等：同scope/函数/key只读7天原结果；异输入/异代冲突，过期不写，由上层成功新配置事务替换。
- 依赖：`model-gateway.availabilityReadError`

## model-gateway.NewCatalogHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/provider.go`
- 作用：绑定既有供应方与模型配置保存及enabled启停

```go
func NewCatalogHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `CatalogHandlers` | 两项真实HTTP映射 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 仅映射既有putProvider/putModel，从平台取得可信OperationContext并绑定已生成严格校验JSON。
2. 核providerId/modelId路径与正文对象ID一致，非空Idempotency-Key头与正文key一致；分别调用真实PutProvider/PutModel。
3. 返回no-store JSON及公共Problem；统一入口即时superadmin授权一次。缺其他handler仍拒绝完整服务启动，不新增启停HTTP。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：交公共完整HTTP边界；具体函数分别校验QA、owner服务或superadmin范围。
- 并发：每请求独立事务；不构造部分可运行服务。
- 幂等：HTTP仅绑定；写入由真实函数持久回执与CAS保证。
- 依赖：`model-gateway.PutProvider`、`model-gateway.PutModel`

## model-gateway.PutCredential

- 级别：约束
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：加密存储密钥

```go
func PutCredential(ctx context.Context, op OperationContext, input PutCredentialInput) (PutCredentialResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutCredentialInput` | 封闭业务输入；逐字段见types $defs/PutCredentialInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutCredentialResult` | 成功结果；逐字段见types $defs/PutCredentialResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只接受已通过公共入口即时授权的admin+superadmin，不重复schema/服务身份/deadline校验；禁止记录secret或请求正文。
2. 从已安装Store取得可信masterKeyRef；仅本凭据写入按私有路径、O_NOFOLLOW/文件身份与33字节有界读取验证恰好32字节部署主密钥。配置可缺省；缺失/不可读取失败关闭，不影响免费任务读取。
3. 规范输入以主密钥派生的域分离HMAC-SHA256形成敏感摘要，绑定callerService/dataEpoch/函数/providerId/expectedVersion/secret；明文、可离线猜测的普通SHA和masterKey不入持久记录。7天回执协议固定同一部署主密钥，本轮不是主密钥轮换，换键导致旧回执不能匹配时CONFLICT，不自动覆盖。
4. 同一SQL事务严格核非空当前epoch，再读取site/actor/函数/key的7天原结果；同代同HMAC返回原结果，不重核后来供应方/屏障；异输入/异代CONFLICT，时间或原结果损坏INTEGRITY_ERROR；过期只读未命中。
5. 新操作拒绝所有非released本地屏障；读取同代provider及其真实version/credential_version，expectedVersion只比较当前凭据版本。不存在NOT_FOUND，旧代或CAS变化VERSION_MISMATCH；不得复用已存在历史版本，历史冲突INTEGRITY_ERROR。
6. 新credentialVersion=expectedVersion+1；每次新操作随机96bit nonce，AES-256-GCM以规范JSON {format:credential-aes256gcm-v1,providerId,credentialVersion}作完整AAD。只写nonce/ciphertext/dataEpoch，不解密或连接供应方。
7. providers更新同时条件绑定providerId/version/credential_version/dataEpoch，provider.version递增；密文、active指针、provider.changed/v1最小元数据及完整脱敏原结果回执同事务提交。只成功的新配置事务替换过期键，失败回滚，Commit未知不自行重试。
8. 保留全部旧密文供既有派发引用；不因当前invocations为空推断可销毁。临时可变key/plaintext/HMAC规范编码字节清零，不承诺擦除immutable Go string或标准库内部副本。
9. PutCredential可首次写入或替换当前凭据，允许修复active材料缺失但保留真实版本CAS。fingerprint为域分离HMAC摘要与掩码；仅超过8个Unicode字符时显示末4位，短secret不完整回显。响应不含明文。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：同事务增加AES-GCM密文、推进供应方active指针/版本、平台provider.changed及脱敏原回执；保留旧密文，不调用模型、供应方或费用任务
- 授权：permission：superadmin；rule：仅已通过公共入口即时授权的admin+superadmin；可信主密钥引用仅来自本域启动装配，不接受浏览器路径。
- 并发：同一本域SQL事务严格epoch、7天HMAC回执、全部屏障、真实凭据与供应方双版本CAS、历史唯一性、密文、平台outbox和回执；主密钥文件在事务之前只读，不执行网络IO。
- 幂等：siteId+actorId+函数ID+key唯一，7天同代同HMAC返回完整原结果；摘要以同一部署主密钥域分离派生，异输入/异代或换主密钥导致摘要不匹配均CONFLICT；过期仅成功新事务替换。
- 依赖：`model-gateway.writeCredential`

## model-gateway.RotateCredential

- 级别：约束
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：轮转新请求使用的密钥

```go
func RotateCredential(ctx context.Context, op OperationContext, input RotateCredentialInput) (RotateCredentialResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RotateCredentialInput` | 封闭业务输入；逐字段见types $defs/RotateCredentialInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RotateCredentialResult` | 成功结果；逐字段见types $defs/RotateCredentialResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只接受已通过公共入口即时授权的admin+superadmin，不重复schema/服务身份/deadline校验；禁止记录secret或请求正文。
2. 从已安装Store取得可信masterKeyRef；仅本凭据写入按私有路径、O_NOFOLLOW/文件身份与33字节有界读取验证恰好32字节部署主密钥。配置可缺省；缺失/不可读取失败关闭，不影响免费任务读取。
3. 规范输入以主密钥派生的域分离HMAC-SHA256形成敏感摘要，绑定callerService/dataEpoch/函数/providerId/expectedVersion/secret；明文、可离线猜测的普通SHA和masterKey不入持久记录。7天回执协议固定同一部署主密钥，本轮不是主密钥轮换，换键导致旧回执不能匹配时CONFLICT，不自动覆盖。
4. 同一SQL事务严格核非空当前epoch，再读取site/actor/函数/key的7天原结果；同代同HMAC返回原结果，不重核后来供应方/屏障；异输入/异代CONFLICT，时间或原结果损坏INTEGRITY_ERROR；过期只读未命中。
5. 新操作拒绝所有非released本地屏障；读取同代provider及其真实version/credential_version，expectedVersion只比较当前凭据版本。不存在NOT_FOUND，旧代或CAS变化VERSION_MISMATCH；不得复用已存在历史版本，历史冲突INTEGRITY_ERROR。
6. 新credentialVersion=expectedVersion+1；每次新操作随机96bit nonce，AES-256-GCM以规范JSON {format:credential-aes256gcm-v1,providerId,credentialVersion}作完整AAD。只写nonce/ciphertext/dataEpoch，不解密或连接供应方。
7. providers更新同时条件绑定providerId/version/credential_version/dataEpoch，provider.version递增；密文、active指针、provider.changed/v1最小元数据及完整脱敏原结果回执同事务提交。只成功的新配置事务替换过期键，失败回滚，Commit未知不自行重试。
8. 保留全部旧密文供既有派发引用；不因当前invocations为空推断可销毁。临时可变key/plaintext/HMAC规范编码字节清零，不承诺擦除immutable Go string或标准库内部副本。
9. RotateCredential要求expectedVersion>=1且当前active凭据存在、同代且nonce/ciphertext长度完整；返回新credentialVersion和retiredVersion，保留旧密文。
10. 退休材料销毁已接DestroyRetiredSecrets，只有完整应用显式tracking且无legacy缺证据才执行；unknown/held保留。完整启动tracking初始化待下一批，整函数暂不ready。
11. 仅snapshot_material_redactions有原完整hash来源且nonce/ciphertext都空才视明确未含密钥；普通空值损坏严格失败，管理员新版本配置不复活旧attempt。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：同事务增加AES-GCM密文、推进供应方active指针/版本、平台provider.changed及脱敏原回执；保留旧密文，不调用模型、供应方或费用任务
- 授权：permission：superadmin；rule：仅已通过公共入口即时授权的admin+superadmin；可信主密钥引用仅来自本域启动装配，不接受浏览器路径。
- 并发：同一本域SQL事务严格epoch、7天HMAC回执、全部屏障、真实凭据与供应方双版本CAS、历史唯一性、密文、平台outbox和回执；主密钥文件在事务之前只读，不执行网络IO。
- 幂等：siteId+actorId+函数ID+key唯一，7天同代同HMAC返回完整原结果；摘要以同一部署主密钥域分离派生，异输入/异代或换主密钥导致摘要不匹配均CONFLICT；过期仅成功新事务替换。
- 依赖：`model-gateway.writeCredential`

## model-gateway.writeCredential

- 级别：参考
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：共用凭据密文与原结果事务

```go
func writeCredential(ctx context.Context, op OperationContext, input credentialCommand) (credentialWriteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅已验证的本域凭据写入链路 |
| `op` | `OperationContext` | 可信主体与epoch | 仅已验证的本域凭据写入链路 |
| `input` | `credentialCommand` | Put或Rotate不可持久明文命令 | 仅已验证的本域凭据写入链路 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `credentialWriteResult` | 脱敏结果 | 失败结果不可用，不输出明文或私有路径 |
| `err` | `error` | 公共错误 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. 落实Put/Rotate共同步骤：只读主密钥、HMAC敏感摘要、严格epoch/回执/屏障、真实provider及凭据CAS、历史唯一性、AES-GCM、同事务outbox和脱敏回执。
2. Rotate额外核active材料；任何分支均不删除旧密文。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：`model-gateway.CurrentStore`、`model-gateway.DestroyRetiredSecrets`、`model-gateway.Store.BeginTx`、`model-gateway.Store.MasterKeyRef`、`model-gateway.credentialDigest`、`model-gateway.credentialFingerprint`、`model-gateway.credentialStorageError`、`model-gateway.loadCredentialMasterKey`、`model-gateway.readCredentialReceipt`、`model-gateway.IsRedactedMaterial`

## model-gateway.loadCredentialMasterKey

- 级别：参考
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：只在凭据写入安全有界读取主密钥

```go
func loadCredentialMasterKey(ref string) ([]byte, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ref` | `string` | 已装配部署secret引用 | 仅已验证的本域凭据写入链路 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `bytes` | 恰好32字节可变缓冲，由上层清零 | 失败结果不可用，不输出明文或私有路径 |
| `err` | `error` | 公共错误 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. 引用必须非空绝对路径；复用平台ValidatePrivatePath拒绝符号链接/多硬链接并核regular-file。
2. Lstat后O_RDONLY/O_NOFOLLOW/O_CLOEXEC/O_NONBLOCK打开，核SameFile；读取最多33字节，必须32；末次再核路径/打开文件身份、长度和mtime未变，失败关闭且清临时字节。
3. 不记录引用或密钥，不在免费读取/启动时读密钥。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：`platform.ValidatePrivatePath`

## model-gateway.credentialDigest

- 级别：参考
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：生成不可离线猜测的域分离敏感摘要

```go
func credentialDigest(key []byte, purpose string, value any) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `bytes` | 32字节主密钥 | 仅已验证的本域凭据写入链路 |
| `purpose` | `string` | 固定域分离用途 | 仅已验证的本域凭据写入链路 |
| `value` | `any` | 待摘要规范对象 | 仅已验证的本域凭据写入链路 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `digest` | `string` | HMAC十六进制摘要 | 失败结果不可用，不输出明文或私有路径 |
| `err` | `error` | 公共错误 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. CanonicalJSON产生临时字节，HMAC-SHA256先按固定purpose派生子键，再对规范输入HMAC；清零临时编码和子键。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：无

## model-gateway.credentialFingerprint

- 级别：参考
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：生成不会完整回显短密钥的脱敏指纹

```go
func credentialFingerprint(key []byte, providerID, secret string) (string, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `bytes` | 32字节主密钥 | 仅已验证的本域凭据写入链路 |
| `providerID` | `string` | 供应方范围 | 仅已验证的本域凭据写入链路 |
| `secret` | `string` | 本次不可持久明文 | 仅已验证的本域凭据写入链路 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `fingerprint` | `string` | 掩码与截短HMAC | 失败结果不可用，不输出明文或私有路径 |
| `err` | `error` | 公共错误 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. fingerprint域HMAC绑定providerId/secret；显示12个十六进制摘要字符；secret超过8个Unicode字符才保留末4位，否则全掩码。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：`model-gateway.credentialDigest`

## model-gateway.readCredentialReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：只读7天凭据脱敏原结果并失败关闭

```go
func readCredentialReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, functionID, key, digest string, expectedJSON []byte) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 仅已验证的本域凭据写入链路 |
| `op` | `OperationContext` | 可信范围 | 仅已验证的本域凭据写入链路 |
| `tx` | `SQLTransaction` | 当前同代事务 | 仅已验证的本域凭据写入链路 |
| `functionID` | `string` | Put或Rotate函数ID | 仅已验证的本域凭据写入链路 |
| `key` | `string` | 幂等键 | 仅已验证的本域凭据写入链路 |
| `digest` | `string` | 域分离HMAC | 仅已验证的本域凭据写入链路 |
| `expectedJSON` | `bytes` | 确定脱敏原结果编码 | 仅已验证的本域凭据写入链路 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `found` | `bool` | 是否存在有效精确原结果 | 失败结果不可用，不输出明文或私有路径 |
| `err` | `error` | 公共错误 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. 同scope/函数/key只读HMAC、result_json与created_at；坏时间失败，过期只返回未命中不删除。
2. 有效HMAC不匹配CONFLICT，脱敏原结果精确字节不符INTEGRITY_ERROR；命中不执行后续写屏障/供应方查询。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：`model-gateway.credentialStorageError`

## model-gateway.credentialStorageError

- 级别：参考
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：映射本域凭据存储未知与取消

```go
func credentialStorageError(err error) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 下游错误 | 仅已验证的本域凭据写入链路 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 保留取消/截止的脱敏错误 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. 保留已脱敏公共错误；原始context取消/截止分别映射公共CANCELLED/DEADLINE_EXCEEDED，其余SQL错误统一DEPENDENCY_UNAVAILABLE，不输出原SQL/秘密。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：无

## model-gateway.NewCredentialHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/security/credential.go`
- 作用：装配凭据写入真实HTTP映射

```go
func NewCredentialHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `CredentialHandlers` | 真实Put/Rotate HTTP映射 | 失败结果不可用，不输出明文或私有路径 |

**步骤**

1. 仅putCredential/rotateCredential；取平台可信op并绑定已严格校验的JSON。
2. 核providerId路径/正文和非空Idempotency-Key头/正文一致；调用真实Put/Rotate；响应no-store、Put 201、Rotate 200与公共Problem。
3. 授权由公共ServiceBoundaryConfig.Authorize即时核superadmin一次，不记录正文，不构造部分可运行服务。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅本域可信装配或PutCredential/RotateCredential已核admin+superadmin的链路
- 并发：同域上下文传播，密钥只在请求内临时持有；不调用网络
- 幂等：业务写入由同事务HMAC回执与CAS保证；纯计算可重复
- 依赖：`model-gateway.PutCredential`、`model-gateway.RotateCredential`

## model-gateway.PutModel

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：保存不可变模型身份版本

```go
func PutModel(ctx context.Context, op OperationContext, input PutModelInput) (PutModelResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutModelInput` | 封闭业务输入；逐字段见types $defs/PutModelInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutModelResult` | 成功结果；逐字段见types $defs/PutModelResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 公共入口即时校验superadmin，内部信任严格生成校验过的输入与可信op；model.version必须expectedVersion+1且不超过安全整数上限。
2. beginAvailabilitySnapshot严格核非空runtime_meta当前代次；普通输入摘要绑定callerService/dataEpoch/model/expectedVersion。先读取7天完整原结果，同代同hash先返回，不因后续配置或本地屏障阻断；异输入/异代CONFLICT，坏时间或结果INTEGRITY_ERROR，过期只读未命中。 可选manualPrice以omitempty字段追加规范摘要尾部；省略时旧JSON字节不变，兼容原回执。
3. 读取真实模型version/epoch及providerId/providerModelVersion/dimension/preprocessorVersion；缺行仅expectedVersion0可创建。已有模型以上四项身份字段任一变化返回CONFLICT并要求调用方用新modelId，原ID不可改写。
4. 条件UPSERT以models.version/data_epoch绑定CAS并核RowsAffected恰好1，只更新可变能力、token上限、目录来源、人工覆盖、enabled与version/time；停用只影响后续路由，不清既有任务分配、档位、invocations、held或ledger。
5. 非embedding模型、现有model.changed平台outbox和完整PutModelResult回执同事务；embedding新写在前置失败，不写库/回执/outbox。仅成功新配置事务替换过期键；未知Commit不自行重试；不调用模型、不自动重算付费结果。 可选manualPrice对象是整份人工价目替换，三个费率可缺（unknown不回退），服务器当前有效时刻，SaveManualPrice与模型CAS/事件/原回执同事务；省略不写价目。
6. 嵌入预处理默认none已收口，embedding可沿同一ValidateModel/CAS/事件保存；原空间身份算法不变，版本变更须新ID
7. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。catalogRef双列同步读写，旧空值不猜填，固定路线快照保留源引用。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：同事务写models、model.changed平台outbox与7天完整回执；enabled停用不改在途调用或费用状态。
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：同一严格当前代次事务先回执后全屏障/真实provider与model身份/CAS；条件UPSERT及RowsAffected核对，配置/事件/回执原子提交，无跨域IO。
- 幂等：siteId/actorId/函数/key唯一，7天原结果；hash绑定callerService/dataEpoch/model/expectedVersion，有效同hash先返回，异hash CONFLICT。旧格式hash不默认为新摘要；过期仅随成功新事务替换。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.SaveManualPrice`、`model-gateway.ValidateModel`、`model-gateway.availabilityReadError`、`model-gateway.beginAvailabilitySnapshot`、`model-gateway.readModelReceipt`

## model-gateway.ValidateModel

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：纯声明验证模型能力

```go
func ValidateModel(ctx context.Context, op OperationContext, input ValidateModelInput) (ValidateModelResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateModelInput` | 封闭业务输入；逐字段见types $defs/ValidateModelInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateModelResult` | 成功结果；逐字段见types $defs/ValidateModelResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 纯规则检查能力集合、embedding维度和模型输入输出声明
2. embedding空间使用原五字段有序JSONhash；预处理显式默认none，不做默示L2归一化
3. 供应方/不可变模型版本/维度/预处理版本任一改变必须新modelId/space；不得沿旧space混用
4. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。catalogRef双列同步读写，旧空值不猜填，固定路线快照保留源引用。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无网络探测；无收费
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.embeddingSpaceID`、`model-gateway.validCapability`、`model-gateway.validCatalogOrigin`、`model-gateway.validModelString`

## model-gateway.readModelReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：只读7天模型配置原结果并核同代普通输入摘要

```go
func readModelReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, key string, inputHash platform.Hash, expected Model) (PutModelResult, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 原入口取消 | 已授权PutModel当前同代事务 |
| `op` | `OperationContext` | 可信范围 | 已授权PutModel当前同代事务 |
| `tx` | `SQLTransaction` | 当前事务 | 已授权PutModel当前同代事务 |
| `key` | `string` | 幂等键 | 已授权PutModel当前同代事务 |
| `inputHash` | `Hash` | 含caller和epoch摘要 | 已授权PutModel当前同代事务 |
| `expected` | `Model` | 输入模型完整原结果 | 已授权PutModel当前同代事务 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutModelResult` | 原结果 | 失败结果不可使用；不输出私有原值 |
| `found` | `bool` | 是否命中有效原回执 | 失败结果不可使用；不输出私有原值 |
| `err` | `error` | 脱敏公共错误 | 失败结果不可使用；不输出私有原值 |

**步骤**

1. 同scope/函数/key只读hash/created_at/result_json；坏时间失败，过期未命中不写。
2. 有效hash不匹配CONFLICT；完整规范结果编码必须与持久原结果精确一致，否则INTEGRITY_ERROR。
3. 命中先返原结果，不做后续写屏障/当前指针/CAS校验；仅上层成功新事务替换过期键。

**错误**

- `CONFLICT`：幂等输入不一致（不可重试）
- `INTEGRITY_ERROR`：持久模型能力或回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域查询未知（可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅PutModel已核superadmin的同代事务链路。
- 并发：共用PutModel的同代SQL事务，只读原回执，不独立提交。
- 幂等：同scope/函数/key只读7天原结果；异输入/异代冲突，过期不写，由上层成功新配置事务替换。
- 依赖：`model-gateway.availabilityReadError`

## model-gateway.ModelEmbeddingSpace

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：本域唯一五字段空间权威生产者，保持既有有序JSONhash算法

```go
func ModelEmbeddingSpace(model Model) (EmbeddingSpace, error)
```

**输入**

无

**输出**

无

**步骤**

1. 模型维度与四项身份材料必须完整
2. 复用原embeddingSpaceID算法返回五字段空间，不新增或猜测归一化身份

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.embeddingSpaceID`

## model-gateway.embeddingSpaceID

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：保留原有序JSON空间身份算法

```go
func embeddingSpaceID(model Model) (platform.Hash, error)
```

**输入**

无

**输出**

无

**步骤**

1. 保留原有序JSON空间身份算法

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.validModelString

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：validModelString的实际本域生产职责

```go
func validModelString(value string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.validCatalogOrigin

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：validCatalogOrigin的实际本域生产职责

```go
func validCatalogOrigin(origin gen.ModelCatalogOrigin) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.validCapability

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：validCapability的实际本域生产职责

```go
func validCapability(capability string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ReadInputBudgetModel

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/model.go`
- 作用：ReadInputBudgetModel受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。

```go
func ReadInputBudgetModel(ctx context.Context, op OperationContext, modelID string) (Model, error)
```

**输入**

无

**输出**

无

**步骤**

1. 受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.beginAvailabilitySnapshot`

## model-gateway.PutProfile

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：管理AI档位与费用限制

```go
func PutProfile(ctx context.Context, op OperationContext, input PutProfileInput) (PutProfileResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutProfileInput` | 封闭业务输入；逐字段见types $defs/PutProfileInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutProfileResult` | 成功结果；逐字段见types $defs/PutProfileResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅admin且持superadmin可写；入口即时identity授权一次。profile.version必须等于expectedVersion+1；profile.costClass为配置目标，不覆盖可信op.costClass。
2. 复用本域Store开启同事务严格非空runtime_meta代次快照；以site/actor/函数/key读取7天原回执，hash绑定callerService/dataEpoch/profile/expectedVersion。同代同输入先返回原结果，不重查后来模型或屏障；异输入/异代CONFLICT、坏回执INTEGRITY_ERROR。
3. 新操作拒绝任意非released本地屏障；读取profile当前version/epoch并比较CAS。未创建只允许expectedVersion=0；同代costClass+tier唯一槽已被其他profileId占用为CONFLICT，允许移动到未占用槽。
4. orderedModelIds保留管理员给定顺序及公开schema唯一性，最多256项，空数组合法用于清配置。按根唯一目录生成AITierCapabilities逐模型核同代、enabled及全部直接能力；缺模型或停用/能力不足VALIDATION，未知查询与持久损坏失败关闭。保存不要求价格、供应方启用或凭据，不探测或调用模型。
5. INSERT首次创建或UPDATE带profileId/expectedVersion/dataEpoch条件，RowsAffected必须1；同事务写profile.changed/v1最小profileId/version/dataEpoch及根目录中旧/新tier受影响taskIds；任一旧/新tier为fast时保守包含defaultTier为vision/rerank的回退任务，即使其当前有独立配置（去重按taskId排序），不广播候选、费用或秘密。
6. 写入完整PutProfileResult原始回执并提交；仅成功新配置事务可替换过期回执，失败回滚；Commit未知不自动重试。只影响后续新调用，不自动重算嵌入或已有付费结果。
7. 视觉未配时使用fast中支持vision的候选、无override的rerank未配时由fast/chat代替。ResolveProfile先应用override再判断空列表回退，不在保存时填充配置。执行SelectCandidates先过滤完整本地eligible列表再取至多16个候选。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）

- 副作用：同事务写profiles、平台profile.changed outbox及7天完整原结果回执；仅新调用采用配置，不启动模型、索引重建或费用任务
- 授权：permission：superadmin；rule：仅已通过公共入口即时授权的admin+superadmin；函数核本域主体范围，不重做服务身份/schema/deadline。
- 并发：本域单连接SQLite同事务严格epoch、7天回执、全部非released屏障、唯一费用类别/档位槽、条件CAS、平台outbox及完整回执；不持事务执行网络IO。
- 幂等：siteId+actorId+函数ID+key唯一且保留7天；规范hash绑定callerService/dataEpoch/profile/expectedVersion。同代同hash直接返回原结果，异hash CONFLICT；过期仅在成功新配置事务中替换。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.beginAvailabilitySnapshot`、`model-gateway.readProfileReceipt`、`model-gateway.validateProfileModels`、`model-gateway.availabilityReadError`
- 其他依赖说明：根contracts/ai-tasks.json生成AITierCapabilities与AITaskCatalog；不手抄目录

## model-gateway.ResolveProfile

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：按唯一任务目录与可信费用类别解析真实档位配置快照

```go
func ResolveProfile(ctx context.Context, op OperationContext, snapshot *availabilitySnapshot, input ResolveProfileInput) (ResolveProfileResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `snapshot` | `availabilitySnapshot` | SelectCandidates持有的同代SQL事务 | 只在本域同快照链路使用，不另开事务、不提交 |
| `input` | `ResolveProfileInput` | 封闭业务输入；逐字段见types $defs/ResolveProfileInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveProfileResult` | 成功结果；逐字段见types $defs/ResolveProfileResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. taskId须根生成AITaskCatalog登记且owner等于可信op.callerService；input.costClass必须逐字等于op.costClass，不接受自由tier/能力。
2. 使用SelectCandidates已开启的严格非空当前epoch SQL快照，必须存在同代task_assignments且tier等于目录defaultTier；缺赋值NOT_FOUND，旧代VERSION_MISMATCH，损坏INTEGRITY_ERROR。
3. 读取同costClass/defaultTier真实profiles；先应用override替换首项或在空列表成为唯一候选并保序去重，不把被替换主模型加回。
4. 仅vision/rerank在目标槽缺失或覆盖后仍空时读取同costClass真实fast槽；缺fast NOT_FOUND，不合成档位/version。目标槽缺失但有override时由真实fast承载，vision仍需vision，rerank有override仍需rerank、仅无override空档才fast/chat代替。其他档不回退。
5. 返回原profile、真实assignment版本、派生有序引用、目录有效能力及fallback；纯读取不修改配置，不过滤供应方或读取密文/主密钥/价格，不作费用准入。

**错误**

- `VALIDATION`：未知任务目录ID（不可重试）
- `FORBIDDEN`：任务非调用服务owner或costClass不一致（不可重试）
- `NOT_FOUND`：任务赋值或所需真实档位缺失（不可重试）
- `VERSION_MISMATCH`：本地域或持久引用异代（不可重试）
- `INTEGRITY_ERROR`：持久元数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域SQL或依赖未装配、无eligible候选（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：安全元数据摘要编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：公共入口建立可信身份；本函数只核任务owner与costClass，允许owner服务继承已认证主体，不重复身份/schema/deadline。
- 并发：一次本域只读SQL事务，所有赋值与档位同代；结束回滚释放，不持事务做网络IO。
- 幂等：纯读取可重复，不写回执；快照版本按实际持久行返回。
- 依赖：`model-gateway.readRouteProfile`、`model-gateway.applyTaskOverride`、`model-gateway.availabilityReadError`
- 其他依赖说明：根contracts/ai-tasks.json生成AITaskCatalog/AITierCapabilities；无HTTP变更

## model-gateway.readProfileReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：读取并核对7天档位配置原结果回执

```go
func readProfileReceipt(ctx context.Context, op OperationContext, snapshot *availabilitySnapshot, input PutProfileInput, inputHash platform.Hash) (PutProfileResult, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 已通过本域档位写入授权与严格代次快照 |
| `op` | `OperationContext` | 可信调用范围 | 已通过本域档位写入授权与严格代次快照 |
| `snapshot` | `availabilitySnapshot` | 当前同代事务 | 已通过本域档位写入授权与严格代次快照 |
| `input` | `PutProfileInput` | 原请求 | 已通过本域档位写入授权与严格代次快照 |
| `inputHash` | `Hash` | 含代次与调用方摘要 | 已通过本域档位写入授权与严格代次快照 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutProfileResult` | 原结果 | 失败结果不可使用；不输出私有原值 |
| `found` | `bool` | 是否命中有效原回执 | 失败结果不可使用；不输出私有原值 |
| `err` | `error` | 脱敏公共错误 | 失败结果不可使用；不输出私有原值 |

**步骤**

1. 按site/actor/PutProfile/key只读hash、result_json、created_at；缺行表示未执行。
2. 坏时间或原结果损坏INTEGRITY_ERROR；超过7天只返回未命中且不写。有效期异hash CONFLICT；原结果逐字段等于本次规范profile才返回。
3. 不核后来模型/屏障，不删除过期回执；仅上层成功新配置事务末尾替换。

**错误**

- `CONFLICT`：幂等输入不一致（不可重试）
- `INTEGRITY_ERROR`：持久模型能力或回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域查询未知（可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅PutProfile已核admin+superadmin的同事务链路。
- 并发：共用PutProfile的同代SQL事务，不独立提交。
- 幂等：由上层同事务原回执与CAS保证。
- 依赖：`model-gateway.availabilityReadError`

## model-gateway.validateProfileModels

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：按唯一生成目录校验档位候选直接能力

```go
func validateProfileModels(ctx context.Context, snapshot *availabilitySnapshot, profile Profile) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 原入口IO取消 | 本域PutProfile已授权且输入通过公共schema |
| `snapshot` | `availabilitySnapshot` | 严格同代SQL事务 | 本域PutProfile已授权且输入通过公共schema |
| `profile` | `Profile` | 已验证候选与目标档位 | 本域PutProfile已授权且输入通过公共schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 脱敏公共错误 | 失败结果不可使用；不输出私有原值 |

**步骤**

1. 从唯一根目录生成AITierCapabilities按profile.tier取直接能力；不将structured或translation等建议变成额外硬门槛。
2. 按orderedModelIds原顺序调用已有validateTaskOverride核同代启用模型与全部能力，空数组合法、不查价格/凭据/供应方可用性。

**错误**

- `VALIDATION`：覆盖模型未启用或能力不满足（不可重试）
- `VERSION_MISMATCH`：模型或回执代次不匹配（不可重试）
- `CONFLICT`：幂等输入不一致（不可重试）
- `INTEGRITY_ERROR`：持久模型能力或回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域查询未知（可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅PutProfile已核admin+superadmin的同事务链路。
- 并发：共用PutProfile的同代SQL事务，不做网络IO。
- 幂等：由上层同事务原回执与CAS保证。
- 依赖：`model-gateway.validateTaskOverride`

## model-gateway.NewProfileHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：装配真实档位保存HTTP映射

```go
func NewProfileHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `ProfileHandlers` | 单项真实HTTP映射 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 仅映射putProfile；从平台取得可信OperationContext，绑定公共边界已严格验证的JSON。
2. 核profileId路径与profile.profileId一致，非空Idempotency-Key头与正文key一致；调用真实PutProfile。
3. 返回no-store JSON及公共Problem；写入口由公共ServiceBoundaryConfig.Authorize即时校验一次，其他handler缺失仍拒绝完整服务启动。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：交公共完整HTTP边界；具体函数分别校验QA、owner服务或superadmin范围。
- 并发：每请求独立事务；不构造部分可运行服务。
- 幂等：HTTP仅绑定；写入由真实函数持久回执与CAS保证。
- 依赖：`model-gateway.PutProfile`

## model-gateway.readRouteProfile

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：读取当前费用类别和档位的真实配置行

```go
func readRouteProfile(ctx context.Context, snapshot *availabilitySnapshot, costClass string, tier string) (Profile, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 已通过公共入口；仅执行本函数登记规则 |
| `snapshot` | `availabilitySnapshot` | 当前同代事务 | 已通过公共入口；仅执行本函数登记规则 |
| `costClass` | `string` | 可信费用类别 | 已通过公共入口；仅执行本函数登记规则 |
| `tier` | `string` | 目录或合法回退档位 | 已通过公共入口；仅执行本函数登记规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `profile` | `Profile` | 真实原始配置 | 失败结果不可用；不输出密文/秘密/正文 |
| `found` | `bool` | 是否存在 | 失败结果不可用；不输出密文/秘密/正文 |
| `err` | `error` | 公共错误 | 失败结果不可用；不输出密文/秘密/正文 |

**步骤**

1. 读取当前epoch/costClass/tier唯一行；缺行返回found=false，不创建默认记录。
2. 校验持久版本、费用整数范围和有序唯一引用；费用数值仅返回配置声明，绝不查询价目/余额/使用量或判断收费准入。

**错误**

- `VALIDATION`：未知任务目录ID（不可重试）
- `FORBIDDEN`：任务非调用服务owner或costClass不一致（不可重试）
- `NOT_FOUND`：任务赋值或所需真实档位缺失（不可重试）
- `VERSION_MISMATCH`：本地域或持久引用异代（不可重试）
- `INTEGRITY_ERROR`：持久元数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域SQL或依赖未装配、无eligible候选（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：安全元数据摘要编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅已认证任务owner内部链路；身份/schema/deadline由公共边界建立，不重复校验。
- 并发：同一严格当前epoch只读SQL事务；纯计算不检查ctx，不做网络IO。
- 幂等：纯读取/计算可重复，无回执或费用写入。
- 依赖：`model-gateway.availabilityReadError`

## model-gateway.applyTaskOverride

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：纯函数应用主模型覆盖并保序去重

```go
func applyTaskOverride(ordered []string, override *string) []string
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ordered` | `[]string` | 持久合法有序引用 | 已通过公共入口；仅执行本函数登记规则 |
| `override` | `*string` | 已核合法引用或nil | 已通过公共入口；仅执行本函数登记规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `modelIds` | `[]string` | 非null有序唯一候选 | 失败结果不可用；不输出密文/秘密/正文 |

**步骤**

1. 复制有序引用，override替换首项或在空数组成为唯一候选。
2. 保留原第二项起备用并去重，不修改原始profile、不加入原首项。

**错误**

- `VALIDATION`：未知任务目录ID（不可重试）
- `FORBIDDEN`：任务非调用服务owner或costClass不一致（不可重试）
- `NOT_FOUND`：任务赋值或所需真实档位缺失（不可重试）
- `VERSION_MISMATCH`：本地域或持久引用异代（不可重试）
- `INTEGRITY_ERROR`：持久元数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域SQL或依赖未装配、无eligible候选（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：安全元数据摘要编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅已认证任务owner内部链路；身份/schema/deadline由公共边界建立，不重复校验。
- 并发：同一严格当前epoch只读SQL事务；纯计算不检查ctx，不做网络IO。
- 幂等：纯读取/计算可重复，无回执或费用写入。
- 依赖：无

## model-gateway.SelectCandidates

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：在同一SQL快照解析真实执行候选元数据

```go
func SelectCandidates(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 已通过公共入口；仅执行本函数登记规则 |
| `op` | `OperationContext` | 可信范围 | 已通过公共入口；仅执行本函数登记规则 |
| `input` | `ResolveRouteInput` | 任务与费用类别 | 已通过公共入口；仅执行本函数登记规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveRouteResult` | 真实有序候选元数据 | 失败结果不可用；不输出密文/秘密/正文 |
| `err` | `error` | 公共错误 | 失败结果不可用；不输出密文/秘密/正文 |

**步骤**

1. CurrentStore与beginAvailabilitySnapshot建立一次严格当前epoch SQL快照，ResolveProfile在该事务内解析真实赋值/档位、先覆盖再回退；不先ResolveProfile后另开快照。
2. 遍历完整至多256项派生配置，readRouteCandidate核同代模型/供应方/当前凭据引用，仅过滤缺模型、停用、有效能力不符或凭据未配；未知SQL/损坏/异代失败关闭。
3. embedding全部eligible候选须同一现有embeddingSpaceID，发现异空间INTEGRITY_ERROR，不能混用或静默跳过；先核完整候选再取16项。
4. 保序返回至多16项模型/供应方版本/凭据版本；无候选DEPENDENCY_UNAVAILABLE。routeVersion为epoch、真实解析配置与最终候选的固定JSON字段顺序SHA256。
5. 不解密、不读主密钥/价目/余额、不调用供应方、不获取派发引用；费用配置仅声明，执行、费用预留与最后引用释放仍由未来执行链保证。
6. routeVersion包含全eligible列表，响应保序16项；不能只对已选项CAS漏掉原不eligible变可用。

**错误**

- `VALIDATION`：未知任务目录ID（不可重试）
- `FORBIDDEN`：任务非调用服务owner或costClass不一致（不可重试）
- `NOT_FOUND`：任务赋值或所需真实档位缺失（不可重试）
- `VERSION_MISMATCH`：本地域或持久引用异代（不可重试）
- `INTEGRITY_ERROR`：持久元数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域SQL或依赖未装配、无eligible候选（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：安全元数据摘要编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅已认证任务owner内部链路；身份/schema/deadline由公共边界建立，不重复校验。
- 并发：同一严格当前epoch只读SQL事务；纯计算不检查ctx，不做网络IO。
- 幂等：纯读取/计算可重复，无回执或费用写入。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.beginAvailabilitySnapshot`、`model-gateway.ResolveProfile`、`model-gateway.readRouteCandidate`
- 其他依赖说明：复用本域现有embeddingSpaceID与hashJSON纯摘要工具，不新增算法副本。

## model-gateway.readRouteCandidate

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：读取安全候选元数据与当前凭据引用完整性

```go
func readRouteCandidate(ctx context.Context, snapshot *availabilitySnapshot, modelID string, requiredCapabilities []string) (RouteCandidate, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 已通过公共入口；仅执行本函数登记规则 |
| `snapshot` | `availabilitySnapshot` | 当前同代事务 | 已通过公共入口；仅执行本函数登记规则 |
| `modelID` | `string` | 合法模型引用 | 已通过公共入口；仅执行本函数登记规则 |
| `requiredCapabilities` | `[]string` | 有效目录能力 | 已通过公共入口；仅执行本函数登记规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `candidate` | `RouteCandidate` | 安全模型与版本引用 | 失败结果不可用；不输出密文/秘密/正文 |
| `eligible` | `bool` | 本地元数据满足 | 失败结果不可用；不输出密文/秘密/正文 |
| `err` | `error` | 公共错误 | 失败结果不可用；不输出密文/秘密/正文 |

**步骤**

1. 读取完整模型安全元数据及供应方enabled/version/credentialVersion/epoch和附加头引用元数据，不SELECT供应方URL或密文正文。
2. 缺模型、停用、有效能力不符、active凭据版本0或材料缺行返回eligible=false；模型/供应方/凭据异代VERSION_MISMATCH、缺供应方或损坏字段/长度INTEGRITY_ERROR、SQL未知失败关闭。
3. 凭据只查data_epoch和nonce/ciphertext长度，12字节nonce与大于16字节密文仅表示材料具备；不表示能解密、供应方在线或派发引用已持久获取。
4. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。catalogRef双列同步读写，旧空值不猜填，固定路线快照保留源引用。
5. 生产方给完整Model不透明快照hash；仅embedding附真实五字段空间；明确脱密的缺key模型不eligible，普通坏材料仍失败。

**错误**

- `VALIDATION`：未知任务目录ID（不可重试）
- `FORBIDDEN`：任务非调用服务owner或costClass不一致（不可重试）
- `NOT_FOUND`：任务赋值或所需真实档位缺失（不可重试）
- `VERSION_MISMATCH`：本地域或持久引用异代（不可重试）
- `INTEGRITY_ERROR`：持久元数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域SQL或依赖未装配、无eligible候选（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：安全元数据摘要编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅已认证任务owner内部链路；身份/schema/deadline由公共边界建立，不重复校验。
- 并发：同一严格当前epoch只读SQL事务；纯计算不检查ctx，不做网络IO。
- 幂等：纯读取/计算可重复，无回执或费用写入。
- 依赖：`model-gateway.availabilityReadError`、`model-gateway.ModelSnapshotHash`、`model-gateway.ModelEmbeddingSpace`、`model-gateway.IsRedactedMaterial`

## model-gateway.SelectCandidatesInTransaction

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/profile.go`
- 作用：生产方免费固定路线及接受事务一致性支持

```go
func SelectCandidatesInTransaction(ctx context.Context, op OperationContext, tx *sql.Tx, input ResolveRouteInput) (ResolveRouteResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅task真实owner纯服务/原可信costClass和当前epoch读取，不产生费用；在同一真实SQL快照解析完整eligible列表，返回原opaque Model hash与embedding真实空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.ResolveProfile`、`model-gateway.readRouteCandidate`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.ResolveRoute

- 级别：参考
- 文件：`modules/model-gateway/internal/routing/route.go`
- 作用：确定不可变调用路线

```go
func ResolveRoute(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ResolveRouteInput` | 封闭业务输入；逐字段见types $defs/ResolveRouteInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveRouteResult` | 成功结果；逐字段见types $defs/ResolveRouteResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 在同一次SelectCandidates只读SQL快照内解析任务/档位及全部候选，返回实际有序至多16项；不先ResolveProfile后另开快照。
2. 只读取本地配置/安全元数据；不创建invocation、费用预留、派发引用或供应方调用。

**错误**

- `VALIDATION`：未知任务目录ID（不可重试）
- `FORBIDDEN`：任务非调用服务owner或costClass不一致（不可重试）
- `NOT_FOUND`：任务赋值或所需真实档位缺失（不可重试）
- `VERSION_MISMATCH`：本地域或持久引用异代（不可重试）
- `INTEGRITY_ERROR`：持久元数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域SQL或依赖未装配、无eligible候选（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：安全元数据摘要编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：公共入口建立可信身份；本函数只核任务owner与costClass，允许owner服务继承已认证主体，不重复身份/schema/deadline。
- 并发：一次本域只读SQL事务，所有赋值与档位同代；结束回滚释放，不持事务做网络IO。
- 幂等：纯读取可重复，不写回执；快照版本按实际持久行返回。
- 依赖：`model-gateway.SelectCandidates`

## model-gateway.ResolveTaskRoute

- 级别：约束
- 文件：`modules/model-gateway/internal/routing/route.go`
- 作用：生产方免费固定路线及接受事务一致性支持

```go
func ResolveTaskRoute(ctx context.Context, op OperationContext, input ResolveRouteInput) (ResolveRouteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ResolveRouteInput` | 封闭业务输入；逐字段见types $defs/CreateInvocationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveRouteResult` | 成功结果；逐字段见types $defs/CreateInvocationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅task真实owner纯服务/原可信costClass和当前epoch读取，不产生费用；在同一真实SQL快照解析完整eligible列表，返回原opaque Model hash与embedding真实空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只允许AITaskCatalog已登记owner纯服务；TaskID owner及body costClass等可信op，免费读取不扩收费授权。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.ResolveRoute`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.NewRouteHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/routing/route.go`
- 作用：生产方免费固定路线及接受事务一致性支持

```go
func NewRouteHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 仅task真实owner纯服务/原可信costClass和当前epoch读取，不产生费用；在同一真实SQL快照解析完整eligible列表，返回原opaque Model hash与embedding真实空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.ResolveTaskRoute`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.QuoteCost

- 级别：约束
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：计算可保守预留的最大费用

```go
func QuoteCost(ctx context.Context, op OperationContext, input QuoteCostInput) (QuoteCostResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `QuoteCostInput` | 封闭业务输入；逐字段见types $defs/QuoteCostInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `QuoteCostResult` | 成功结果；逐字段见types $defs/QuoteCostResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只接受根目录登记的AI任务owner纯服务，同一次严格当前epoch快照核真实模型及已持久价目，不要求供应方在线/启用/凭据，不联网。
2. 同model/epoch读取不晚于服务器当前时刻的manual>remote>bundled首条不可变价目；同source取最新秒/纳秒与模型版本。manual不完整也不回退；缺生产来源不合成目录价。
3. 三费率都显式USD且非负安全整数才pricingKnown=true；输入项与输出项各用大整数计算ceil(tokens*每百万micros/1000000)，再加显式fixed，结果超安全整数返回VALIDATION，不溢出或浮点。
4. 缺项/无价目返回pricingKnown=false及明确warning，maximumCost省略，不伪造零；已存不可变价目直接返回全值，无配置使用模型持久updated_at/版本构造确定性unknown全值，不把它当作已登记历史价目。
5. 全链只读，不写价目/报价记录、不占额度、不碰held/ledger或模型；上下文由入口建立并传给SQL。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：model.manage；rule：portal当前model.manage，或精确根目录AI owner纯服务只读；knowledge必须background，不扩大任何付费准入。
- 并发：同一次严格当前epoch只读SQL快照，无外部IO；免费报价不受业务写屏障拦截，不做额度准入。
- 幂等：读取固定不可变价目全值；未配置unknown身份由持久模型版本/updated_at/epoch确定，不生成随机ID或新写。
- 依赖：`model-gateway.ReadPriceSnapshot`、`model-gateway.RequirePriceCaller`

## model-gateway.ReadPriceSnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：严格同代读取真实模型和manual优先的不可变历史价目；无配置仅报价允许确定性unknown

```go
func ReadPriceSnapshot(ctx context.Context, op OperationContext, modelID string, effectiveAt time.Time, allowUnknown bool) (PriceSnapshot, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿取消 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `op` | `OperationContext` | 可信操作上下文 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `modelID` | `string` | 真实模型ID | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `effectiveAt` | `TimeInstant` | 截止生效时刻 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `allowUnknown` | `bool` | 无价目是否允许确定性unknown | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `snapshot` | `PriceSnapshot` | 不可变价目全值 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 严格同代读取真实模型和manual优先的不可变历史价目；无配置仅报价允许确定性unknown
2. 已认证source价目只读沿原dataEpoch重算核原SnapshotID，配置迁到target不重写旧价/fee hash。

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已验证入口或模型CAS调用链；不自行取得跨域权限
- 并发：跟随调用者SQL快照或纯计算；无外部供应方IO
- 幂等：不可变价目全值与内容地址可重读，写入仅在模型CAS同事务
- 依赖：`model-gateway.beginPriceSnapshot`、`model-gateway.MakePriceSnapshot`、`model-gateway.ReadPriceSnapshotInTransaction`

## model-gateway.SaveManualPrice

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：模型CAS事务内按服务器当前时刻追加完整人工价目替换；不写其他source

```go
func SaveManualPrice(ctx context.Context, op OperationContext, tx *sql.Tx, modelID string, modelVersion int, price ManualPrice) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿取消 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `op` | `OperationContext` | 可信操作上下文 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `tx` | `SQLTransaction` | 模型CAS事务 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `modelID` | `string` | 真实模型ID | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `modelVersion` | `int` | CAS后模型版本 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `price` | `ManualPrice` | 完整人工替换 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 模型CAS事务内按服务器当前时刻追加完整人工价目替换；不写其他source

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：仅在模型CAS事务中追加price_book_entries，不更新历史或费用状态
- 授权：permission：service；rule：仅本域已验证入口或模型CAS调用链；不自行取得跨域权限
- 并发：跟随调用者SQL快照或纯计算；无外部供应方IO
- 幂等：不可变价目全值与内容地址可重读，写入仅在模型CAS同事务
- 依赖：`model-gateway.MakePriceSnapshot`

## model-gateway.MakePriceSnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：构造规范不可变价目全值及域分离内容摘要；缺任一费率为unknown

```go
func MakePriceSnapshot(ctx context.Context, op OperationContext, modelID string, modelVersion int, source string, effectiveAt time.Time, price ManualPrice) (PriceSnapshot, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿取消 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `op` | `OperationContext` | 可信操作上下文 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `modelID` | `string` | 模型ID | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `modelVersion` | `int` | 价目生成模型版本 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `source` | `string` | 受信来源 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `effectiveAt` | `TimeInstant` | 服务器或已存时刻 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `price` | `ManualPrice` | 完整费率投影 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `snapshot` | `PriceSnapshot` | 确定性不可变全值 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 构造规范不可变价目全值及域分离内容摘要；缺任一费率为unknown

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已验证入口或模型CAS调用链；不自行取得跨域权限
- 并发：跟随调用者SQL快照或纯计算；无外部供应方IO
- 幂等：不可变价目全值与内容地址可重读，写入仅在模型CAS同事务
- 依赖：无

## model-gateway.beginPriceSnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：同事务核非空runtime_meta当前代次

```go
func beginPriceSnapshot(ctx context.Context, op OperationContext) (*sql.Tx, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿取消 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `op` | `OperationContext` | 可信操作上下文 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tx` | `SQLTransaction` | 当前代次快照 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 同事务核非空runtime_meta当前代次

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已验证入口或模型CAS调用链；不自行取得跨域权限
- 并发：跟随调用者SQL快照或纯计算；无外部供应方IO
- 幂等：不可变价目全值与内容地址可重读，写入仅在模型CAS同事务
- 依赖：`model-gateway.CurrentStore`

## model-gateway.RequirePriceCaller

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：仅根AI任务目录owner纯服务可读取免费报价

```go
func RequirePriceCaller(ctx context.Context, op OperationContext) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿取消 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |
| `op` | `OperationContext` | 可信操作上下文 | 可信入口/调用链已建立结构与身份；持久材料完整性按本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 无权限公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. portal当前model.manage管理员由公共边界即时授权；原已登记AI task owners纯服务免费读价能力保留（含localization）。
2. knowledge额外Subject==CallerService==knowledge且可信costClass=background，按实际多模型计划只读固定模型/价目，不因此取得新Create/额度/费用权限。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已验证入口或模型CAS调用链；不自行取得跨域权限
- 并发：跟随调用者SQL快照或纯计算；无外部供应方IO
- 幂等：不可变价目全值与内容地址可重读，写入仅在模型CAS同事务
- 依赖：无

## model-gateway.NewPriceHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：绑定既有quoteCost与可信OperationContext/no-store

```go
func NewPriceHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `PriceHandlers` | 真实报价HTTP映射 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 绑定既有quoteCost与可信OperationContext/no-store

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证入口或模型CAS调用链；不自行取得跨域权限
- 并发：跟随调用者SQL快照或纯计算；无外部供应方IO
- 幂等：不可变价目全值与内容地址可重读，写入仅在模型CAS同事务
- 依赖：`model-gateway.QuoteCost`

## model-gateway.ReadPriceSnapshotInTransaction

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/quote.go`
- 作用：ReadPriceSnapshotInTransaction本域真实来源和隔离数据回调

```go
func ReadPriceSnapshotInTransaction(ctx context.Context, op OperationContext, tx *sql.Tx, modelID string, effectiveAt time.Time, allowUnknown bool) (PriceSnapshot, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.MakePriceSnapshot`、`model-gateway.IsRestoredEpoch`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.CreateInvocation

- 级别：约束
- 文件：`modules/model-gateway/internal/invocation/create.go`
- 作用：接受统一AI调用

```go
func CreateInvocation(ctx context.Context, op OperationContext, input CreateInvocationInput) (CreateInvocationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateInvocationInput` | 封闭业务输入；逐字段见types $defs/CreateInvocationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateInvocationResult` | 成功结果；逐字段见types $defs/CreateInvocationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只允许根AI任务目录owner纯服务及原qa/background费用；按能力核特有消息/文本/query与输出预算。幂等摘要含原owner/site/epoch/root/costClass及完整语义输入，排除可续期permit token/expiry；同键先复用原结果，不因旧签名过期重算。
2. 新QA调用必须有guard已consume签名，platform.VerifyAdmissionPermit严格比原JSON、root/epoch及qa.ask/followup；subject保留匿名访客，构造私有费用scope。后台不得带QA许可。
3. localization.suggest只接受background及languageSuggestionApprovalId，其他任务带此字段拒绝；每次新attempt预留/派发前从localization读取当前持久管理员批准，全值核原站点代际与enabled，不信任自报。
4. 在同一模型路线快照选候选，固定任务/档位/模型/渠道/原credential版本及全部原额外头refs；每候选各固定不可变价目；写前核CAS。vision在accepted前经media元数据固定授权资源version，派发再核原hash与完整字节。
5. expectedEmbeddingSpace仅embedding可选；接受时核主模型，主/备各自预留及真实派发前核权威五字段精确相等，不匹配不付费。
6. 同事务持久accepted、原业务deadline、不可变execution hash、路线凭据引用、SSE accepted/outbox及幂等回执。先取得公共TaskGroup容量再提交，回调等待事务commit信号；失败不假accepted；不修改原业务截止。
7. 恢复历史投影只读及人工费用结算，不新派发旧调用或恢复旧准入scope。
8. 原创建键索引和accepted/回执同事务；新创建前也核完整tracking，避旧SDK绕过lookup重付。
9. 共享AcceptPreparedInvocation原子接受持久化，不改变原task owner/QA/background/route授权与tracking门禁。
10. 保留既有memory.assistant+owner memory的已明确批准admin-probe AI任务；仍用真实profile，不当私有portal连接probe。其他公开创建仍仅原QA/background owner，不扩大匿名或scope。
11. 若expectedRouteVersion/expectedPriceSnapshots提供，接受写事务内完整重算含此前被过滤/禁用项的eligible列表并比较真实routeVersion；同事务逐候选复读真实当前priceSnapshotId，期待列表必须非空唯一并与整有序候选一一同序相同。预读和仅选中CAS不替此条件。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写invocation accepted；不直接调用供应方
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`identity:checkAuthorization`、`media:readAsset`、`model-gateway.ExecuteInvocation`、`platform.VerifyAdmissionPermit`、`platform.TaskGroup.Start`、`localization:readLanguageSuggestionApproval`、`model-gateway.AuthorizeImages`、`model-gateway.BeginExecution`、`model-gateway.CanonicalInvocationInput`、`model-gateway.CheckLanguageSuggestionApproval`、`model-gateway.EnqueueExecutionEvent`、`model-gateway.ExecutionDependencies`、`model-gateway.ExecutionSnapshotHash`、`model-gateway.ExecutionStorageError`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.ModelEmbeddingSpace`、`model-gateway.ReadDispatchProvider`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadPriceSnapshot`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.SaveExecutionReceipt`、`model-gateway.SaveInvocationEvent`、`model-gateway.StartAcceptedInvocation`、`model-gateway.ValidateInvocationPayload`、`model-gateway.ValidateInputModel`、`model-gateway.ResolveRoute`、`model-gateway.RequireInvocationLookupTracking`、`model-gateway.AcceptPreparedInvocation`、`model-gateway.SelectCandidatesInTransaction`、`model-gateway.ReadPriceSnapshotInTransaction`

## model-gateway.StartAcceptedInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/create.go`
- 作用：StartAcceptedInvocation的实际本域生产职责

```go
func StartAcceptedInvocation(ctx context.Context, options gatewayruntime.ExecutionOptions, record gatewayruntime.InvocationRecord) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 公共TaskGroup以原invocationID去重、原业务deadline保留、脱离HTTP取消；没有新根/续期/重算，回调从真实Execute到达。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ExecuteInvocation`

## model-gateway.CanonicalInvocationInput

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/create.go`
- 作用：规范原输入幂等摘要排除可续签token

```go
func CanonicalInvocationInput(op OperationContext, input CreateInvocationInput) any
```

**输入**

无

**输出**

无

**步骤**

1. 规范原输入幂等摘要排除可续签token

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ValidateInvocationPayload

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/create.go`
- 作用：核模型能力特有载荷规则

```go
func ValidateInvocationPayload(input CreateInvocationInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 核模型能力特有载荷规则

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.MediaPartAsset`

## model-gateway.ReadDispatchProvider

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/create.go`
- 作用：读取固定渠道及原refs

```go
func ReadDispatchProvider(tx *gatewayruntime.ExecutionTransaction, op OperationContext, id string) (gen.Provider, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取固定渠道及原refs

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.ValidateInputModel

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/create.go`
- 作用：ValidateInputModel受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。

```go
func ValidateInputModel(input CreateInvocationInput, model gen.Model, adapter string) error
```

**输入**

无

**输出**

无

**步骤**

1. 受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。PDF另须file+vision，adapter仅openai_chat/anthropic_messages/gemini；每主/备候选预留/派发都核。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.AcceptPreparedInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/create.go`
- 作用：AcceptPreparedInvocation实际探测执行支持

```go
func AcceptPreparedInvocation(ctx context.Context, op OperationContext, tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord, fixed gatewayruntime.FixedRoute, requestRaw []byte, profileRaw []byte, function string, key string, canonical any, options gatewayruntime.ExecutionOptions) (CreateInvocationResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 普通AI与显式probe各自已校验后共享原invocation持久化/引用/accepted事件/幂等/TaskGroup先容量后commit机制，不扩大入口授权。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG真实供应方调用可能收费；当前设计不执行
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ExecutionSnapshotHash`、`model-gateway.SaveInvocationEvent`、`model-gateway.EnqueueExecutionEvent`、`model-gateway.SaveExecutionReceipt`、`model-gateway.ExecuteInvocation`、`model-gateway.CheckProviderMaterial`

## model-gateway.ReserveCost

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/reservation.go`
- 作用：原子扣占额度

```go
func ReserveCost(ctx context.Context, op OperationContext, input ReserveCostInput) (ReserveCostResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReserveCostInput` | 封闭业务输入；逐字段见types $defs/ReserveCostInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReserveCostResult` | 成功结果；逐字段见types $defs/ReserveCostResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 从持久调用核原owner/site/epoch/root/costClass及当前独立attempt，只读MG已验证FeeScopeBinding，不接受自由scope
2. 固定attempt价目按真实保守token预算计算maximumCost并核输入；价目未知金额NULL且显式未知用量，不伪零，允许调用
3. 同一短写事务核profile请求上限、site+UTC日+profile日预算；qa另核站点日及已启用会话/网络小时预算，network倍率按策略；0表示不限
4. 全部held含旧日未知继续占同scope预算，不因跨日或超时释放；background不占QA额度
5. 每attempt唯一预留及fee_commitments、attempt/reserved调用状态、原回执同事务，重复原输入返回原结果

**错误**

- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：增加held，写reservation和outbox
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadAttemptRecord`、`model-gateway.ComputeUsageCost`、`model-gateway.CheckFeeBudgets`、`model-gateway.ReadReservation`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`、`model-gateway.SaveExecutionReceipt`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`

## model-gateway.ComputeUsageCost

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/reservation.go`
- 作用：ComputeUsageCost的本域实际执行职责

```go
func ComputeUsageCost(price gen.PriceSnapshot, inputTokens, outputTokens int) (*gen.Money, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.ReadReservation

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/reservation.go`
- 作用：ReadReservation的本域实际执行职责

```go
func ReadReservation(tx *gatewayruntime.ExecutionTransaction, id string) (gen.Reservation, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.CheckFeeBudgets

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/reservation.go`
- 作用：CheckFeeBudgets的本域实际执行职责

```go
func CheckFeeBudgets(tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord, additional *gen.Money, now time.Time) error
```

**输入**

无

**输出**

无

**步骤**

1. 只在调用方费用预留短写事务内核固定档位单请求与UTC日额度；单请求合计原invocation全部held/settled尝试，不仅当前attempt。
2. QA另核真实已消费许可scope的全站日/会话小时/网络小时额度；后台不占QA额度；旧日held仍占原scope。
3. additional为nil表示本次最大金额真实未知，既有amount_micro_usd为NULL表示范围内未知；任一适用额度有限时两者均失败关闭，不按零累计。
4. 额度值0明确不限；只有所有实际适用额度均不限时才允许未知预留；不释放既有held，不篡改未知金额；已知整数用big.Int求和防溢出。
5. 恢复当前target授权投影的原source unknown held仍占原site/profile/QA scope，不因epoch过滤清零；用EXISTS语义避多次恢复重复累加。
6. admin-probe独立日限dailyProbeLimit兼容缺字段=用户明确默认0不限；有限额度遇同scope未知held失败关闭；探测限额变更同QA日限须当前superadmin。
7. 专用连接probe仅以record.Probe显式内部策略辨识；memory.assistant原admin-probe AI任务保留真实档位、原备用与预算；连接probe日限仅connection_probes真实invocation集合。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅ReserveCost/ExtendReservation已核原调用owner、site、epoch和原QA许可范围的费用链路，不获取新付费权限。
- 并发：调用方SQLite同一短写事务内读取范围并持久预留，禁止检查后另事务插入。
- 幂等：纯额度核对；预留由调用方同事务命令回执幂等。
- 依赖：无

## model-gateway.DispatchInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/dispatch.go`
- 作用：派发供应方请求

```go
func DispatchInvocation(ctx context.Context, op OperationContext, input DispatchInvocationInput) (DispatchInvocationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `DispatchInvocationInput` | 封闭业务输入；逐字段见types $defs/DispatchInvocationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DispatchInvocationResult` | 成功结果；逐字段见types $defs/DispatchInvocationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 核当前reserved attempt、有效执行lease及取消状态，语言建议再次核持久管理员批准，embedding再次核固定space；获取原provider/profile并发slot。
2. BuildProviderRequest使用原immutable模型/渠道/credential/header refs及media原version/hash字节，不追随新active；SQL外解密并只在请求内持有。
3. 短写事务再次核当前lease与原attempt，先记dispatched_ns及dispatching再发一次HTTP；禁止重定向、自动重试、任意URL和原始错误正文日志。
4. 流式完整UTF8 delta最多16KiB，累计文本64KiB；供应方JSON32MiB/SSE64MiB，空/截断/无终态失败关闭。只从真实协议字段取使用量/收费ID；真实USD按整数微额上取整。
5. 用私有attempt_evidence hash记录脱敏真实终态/费用事实及结构输出，slot在finalization释放；网络未知保留原派发证据，不凭超时释放。
6. 只有完整有效结果与evidence同事务持久resultAttemptId；即使费用仍unknown也精确指向真实结果尝试。
7. 恢复历史投影只读及人工费用结算，不新派发旧调用或恢复旧准入scope。

**错误**

- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：唯一实际供应方调用入口，可能产生费用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.AcquireProviderSlot`、`model-gateway.AppendStreamEvent`、`model-gateway.BeginExecution`、`model-gateway.BeginFinalization`、`model-gateway.BuildProviderRequest`、`model-gateway.CheckLanguageSuggestionApproval`、`model-gateway.ErrorCode`、`model-gateway.ExecutionStorageError`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.ModelEmbeddingSpace`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadExecutionAttempt`、`model-gateway.ReadExecutionRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReleaseProviderSlot`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`、`model-gateway.SendProviderRequest`、`model-gateway.ValidateEmbeddingSpace`、`model-gateway.ValidateInputModel`

## model-gateway.ReadAttemptEvidence

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/dispatch.go`
- 作用：验证原供应方脱敏终态证据hash

```go
func ReadAttemptEvidence(tx *gatewayruntime.ExecutionTransaction, id string) (provider.ProviderOutcome, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核原outcome_json SHA256，再核typed完整canonical仍等原hash；未知/缺失字段不得通过丢信息读取。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.BuildProviderRequest

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/adapter.go`
- 作用：映射统一调用到供应方协议

```go
func BuildProviderRequest(ctx context.Context, op OperationContext, input BuildProviderRequestInput) (BuildProviderRequestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BuildProviderRequestInput` | 封闭业务输入；逐字段见types $defs/BuildProviderRequestInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildProviderRequestResult` | 成功结果；逐字段见types $defs/BuildProviderRequestResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只用原invocation/attempt固定快照，LoadImages经media原version/hash，OpenAttemptCredential/OpenAttemptHeaders读取同epoch/provider原不可变refs，不任意文件路径。
2. 按原adapter编码OpenAI chat/embedding、Anthropic messages、Gemini chat/embedding及native rerank的显式query+候选；保留系统文本和多模态parts；请求URL仅原HTTPS base与固定协议路径。
3. 编码JSON最多32MiB UTF8；敏感Headers/body仅内存，不写事件/回执/日志。固定OpenRouter或显式USD协议才接受供应方金额，其他未知币种不猜兑换。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNSUPPORTED_CAPABILITY` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：短暂解密密钥；返回值禁止日志/持久化
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.OpenAttemptCredential`、`model-gateway.OpenAttemptHeaders`、`model-gateway.BeginExecution`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.LoadImages`、`model-gateway.MapAnthropicRequest`、`model-gateway.MapGeminiRequest`、`model-gateway.ProviderContent`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`

## model-gateway.ClassifyFailure

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/adapter.go`
- 作用：决定有限重试与收费未知

```go
func ClassifyFailure(ctx context.Context, op OperationContext, input ClassifyFailureInput) (ClassifyFailureResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ClassifyFailureInput` | 封闭业务输入；逐字段见types $defs/ClassifyFailureInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ClassifyFailureResult` | 成功结果；逐字段见types $defs/ClassifyFailureResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. acceptance未知或已接受但usage未持久时返回charge_unknown并保留旧held
2. 已有输出则终止，不把后继模型delta拼入已交付结果
3. 已有真实usage且结构输出格式失败可独立structured_retry一次；0/429/5xx可走后继fallback，不重置原deadline
4. 每模型最多一次加一次格式尝试、fallback预算及原held处理由执行器持久次数控制

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：无副作用；不将网络timeout等同免费
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## model-gateway.AppendStreamEvent

- 级别：参考
- 文件：`modules/model-gateway/internal/stream/store.go`
- 作用：持久保存流序列

```go
func AppendStreamEvent(ctx context.Context, op OperationContext, input AppendStreamEventInput) (AppendStreamEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `AppendStreamEventInput` | 封闭业务输入；逐字段见types $defs/AppendStreamEventInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AppendStreamEventResult` | 成功结果；逐字段见types $defs/AppendStreamEventResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅当前原lease的delta/usage可追加，结构output仅完成事件；UTF8完整delta<=16KiB，首delta同事务将原invocation/attempt转streaming，序号连续先持久再交付。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：stream_events与invocation序号原子更新
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.InvocationTerminal`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationEvent`

## model-gateway.ResumeStream

- 级别：约束
- 文件：`modules/model-gateway/internal/stream/store.go`
- 作用：重连读取原流

```go
func ResumeStream(ctx context.Context, op OperationContext, input ResumeStreamInput) (ResumeStreamResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ResumeStreamInput` | 封闭业务输入；逐字段见types $defs/ResumeStreamInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResumeStreamResult` | 成功结果；逐字段见types $defs/ResumeStreamResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅原owner/root或当前管理员读取，排他游标有界连续读取原持久SSE，缺保留游标410，不重新请求供应方。
2. terminal由真实terminal_ns提交判定；中途费用charge_unknown不等于输出终态，避免完整结果尚未completed便提前EOF。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：只读，不重启模型
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ExecutionStorageError`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.InvocationTerminal`、`model-gateway.ReadInvocationRecord`

## model-gateway.InvocationTerminal

- 级别：参考
- 文件：`modules/model-gateway/internal/stream/store.go`
- 作用：InvocationTerminal的本域实际执行职责

```go
func InvocationTerminal(state string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.RecordUsage

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/usage.go`
- 作用：接受供应方用量证据

```go
func RecordUsage(ctx context.Context, op OperationContext, input RecordUsageInput) (RecordUsageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RecordUsageInput` | 封闭业务输入；逐字段见types $defs/RecordUsageInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecordUsageResult` | 成功结果；逐字段见types $defs/RecordUsageResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只接受原已派发attempt的真实providerRequestId、完整input/output token与可选USD金额，usageHash包含pricingComplete；不从文本估算补真实计数。
2. 同provider+requestId唯一、同attempt/hash幂等，不同hash留冲突事实并失败，不静默覆盖。
3. 仅既有IO后的当前权威finalization记账，更新原attempt与调用费用ID，不新模型派发、不延长原deadline。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：追加usage evidence；不直接修改最终账本
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.UsageDigest`、`model-gateway.BeginExecution`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadAttemptRecord`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`

## model-gateway.UsageDigest

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/usage.go`
- 作用：UsageDigest的本域实际执行职责

```go
func UsageDigest(input RecordUsageInput) string
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.ReadUsage

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/usage.go`
- 作用：ReadUsage的本域实际执行职责

```go
func ReadUsage(tx *gatewayruntime.ExecutionTransaction, attempt gatewayruntime.AttemptRecord, hash string) (RecordUsageInput, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.SettleInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/settlement.go`
- 作用：结算已知费用

```go
func SettleInvocation(ctx context.Context, op OperationContext, input SettleInvocationInput) (SettleInvocationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SettleInvocationInput` | 封闭业务输入；逐字段见types $defs/SettleInvocationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SettleInvocationResult` | 成功结果；逐字段见types $defs/SettleInvocationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅原attempt真实usageHash且held预留结算；原价目不可变，真实明确USD金额优先；固定费率不足覆盖实际usage时不能假普通费率。
2. 独立账单含原attempt固定priceSnapshot；金额未知status=price_unknown且NULL，不能伪0；同事务更新fee_commitments及outbox/原回执，后续有限范围遇未知仍失败关闭。
3. 晚到真实金额可修此前charge_unknown attempt，已取消/成功结果不重写；只有全部费用resolved后释放历史凭据引用。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：写不可变ledger并触发invocation.settled
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadUsage`、`model-gateway.ReadReservation`、`model-gateway.ComputeUsageCost`、`model-gateway.InsertLedgerEntry`、`model-gateway.EnqueueExecutionEvent`、`model-gateway.SaveExecutionReceipt`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`

## model-gateway.InsertLedgerEntry

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/settlement.go`
- 作用：InsertLedgerEntry的本域实际执行职责

```go
func InsertLedgerEntry(tx *gatewayruntime.ExecutionTransaction, entry gen.LedgerEntry, hash string, price gen.PriceSnapshot, supersedes string) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.ReadAttemptLedger

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/settlement.go`
- 作用：ReadAttemptLedger的本域实际执行职责

```go
func ReadAttemptLedger(tx *gatewayruntime.ExecutionTransaction, attemptID string) (gen.LedgerEntry, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.MarkUnknownCharge

- 级别：参考
- 文件：`modules/model-gateway/internal/billing/reconcile.go`
- 作用：保留不确定收费责任

```go
func MarkUnknownCharge(ctx context.Context, op OperationContext, input MarkUnknownChargeInput) (MarkUnknownChargeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `MarkUnknownChargeInput` | 封闭业务输入；逐字段见types $defs/MarkUnknownChargeInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MarkUnknownChargeResult` | 成功结果；逐字段见types $defs/MarkUnknownChargeResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 必须已派发且当前尝试预留仍held，固定原providerRequestId及原根
2. attempt标charge_unknown；若是当前attempt则调用同标；全部原held不释放
3. 同事务写invocation.charge_unknown和reconciliation.requested，只有ID/原因/范围无正文
4. 晚到费用未知只改attempt事实，terminal_ns已持久不回退已完成/取消原结果。

**错误**

- `CONFLICT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：保留额度和创建对账任务
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadReservation`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`、`model-gateway.EnqueueExecutionEvent`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`

## model-gateway.ReconcileCharge

- 级别：约束
- 文件：`modules/model-gateway/internal/billing/reconcile.go`
- 作用：证据驱动完成未知费用对账

```go
func ReconcileCharge(ctx context.Context, op OperationContext, input ReconcileChargeInput) (ReconcileChargeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReconcileChargeInput` | 封闭业务输入；逐字段见types $defs/ReconcileChargeInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReconcileChargeResult` | 成功结果；逐字段见types $defs/ReconcileChargeResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 当前portal superadmin显式提交proofHash，原attempt唯一才可省略attemptId，多候选必须指明。
2. charged必须真实USD金额及完整真实usage；缺原usage需要同时给inputTokens/outputTokens；原providerRequestId缺失须显式提供真实ID，已存在必须全等。
3. not_charged只明确证明释放held并保留审计，不造0 token账单、不造收费ID；已有账单通过新更正/void引用保留历史。
4. 只更正原attempt费用及scope投影，不覆盖已成功输出/原hash；全部费用解决才释放原密文引用。
5. 人工对账不更改resultAttemptId/原结果来源，即使winner费用原来unknown。
6. 当前超管可对精确投影原费用结算，不改原结果/模型/winner身份；事件代际与原费用代际分开记录，等ROOT同步消费方例外后再接实现。
7. invocation.reconciled/v1同原更正/void/回执事务outbox，charged才真实entryId/actualCost，not_charged不造账项或金额。target dataEpoch对齐envelope，sourceDataEpoch保原费用身份。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：最终结算或释放预留；保留审计
- 授权：权限 model.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadReservation`、`model-gateway.ReadAttemptLedger`、`model-gateway.ReadUsage`、`model-gateway.InsertLedgerEntry`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`、`model-gateway.SaveExecutionReceipt`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.EnqueueExecutionEvent`

## model-gateway.CancelInvocation

- 级别：约束
- 文件：`modules/model-gateway/internal/invocation/cancel.go`
- 作用：贯穿取消调用

```go
func CancelInvocation(ctx context.Context, op OperationContext, input CancelInvocationInput) (CancelInvocationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CancelInvocationInput` | 封闭业务输入；逐字段见types $defs/CancelInvocationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CancelInvocationResult` | 成功结果；逐字段见types $defs/CancelInvocationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 同一原owner/site/epoch/root及命令输入回执幂等；先持久cancel_requested，再调用公共TaskGroup.Cancel停止真实IO，不宣称上游取消等于不收费。
2. 未派发或已有持久明确not_accepted证明才释放held；已派发且held标charge_unknown，cancelled终态可仍含unknown attempt；晚到真实usage可结算费用。
3. 重放补取消/费用终态清理并返回当前原调用，不重新收费；既有成功结果保留，不因取消请求覆盖。

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：终止本地输出和取消子请求；收费依证据处理
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`platform.TaskGroup.Cancel`、`model-gateway.BeginFinalization`、`model-gateway.ExecutionDependencies`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.FinishCancelledInvocation`、`model-gateway.ReadExecutionRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.SaveExecutionReceipt`、`model-gateway.SaveInvocationRecord`

## model-gateway.GetInvocation

- 级别：约束
- 文件：`modules/model-gateway/internal/invocation/read.go`
- 作用：读取调用状态

```go
func GetInvocation(ctx context.Context, op OperationContext, input GetInvocationInput) (GetInvocationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `GetInvocationInput` | 封闭业务输入；逐字段见types $defs/GetInvocationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetInvocationResult` | 成功结果；逐字段见types $defs/GetInvocationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 按原owner或portal当前model.manage/superadmin、同site/epoch读取安全调用快照
2. 核持久hash、root和costClass一致，返回当前状态及最后序号，不返回prompt/secret
3. 只读兼容output为原持久完整文本/结构结果；winning尝试和调用列表精确一致，原evidence Completed/HTTP2xx/accepted/Code空及全值摘要必须与record.Output一致。
4. 旧完整文本仅单一initial ordinal1/真实winner/无错完整evidence及完整连续accepted→delta/usage→completed原SSE可只读免费派生；多尝试、缺流/缺winning不猜空/404、不重付，不写历史证据/费用/hash。
5. 恢复旧费用仅经当前target权威投影精确读原结果/费用/原hash/winner，旧scope不变成新准入，具体回调待公共snapshot实现后接。

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `GONE` / HTTP 410：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ErrorCode`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ReadAttemptEvidence`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadInvocationSnapshot`

## model-gateway.ReadInvocationSnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/read.go`
- 作用：读取原结果的当前权威快照

```go
func ReadInvocationSnapshot(tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord) (GetInvocationResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 返回原Invocation；完整结果需真实winning attempt与调用列表/派发证明一致，完整HTTP2xx/accepted/Code空evidence全值hash匹配record.Output；未知字段或显式null输出不损失信息读取。
2. 旧文本只调用ReadLegacyTextResult保守证明，active无结果正常省略，旧成功缺winner明确DependencyUnavailable；不把费用未知当结果失败或让调用方重付。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：由已完成owner/site/epoch/root授权的Get或Lookup调用。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ReadAttemptEvidence`、`model-gateway.ErrorCode`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadLegacyTextResult`

## model-gateway.ReadLegacyTextResult

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/read.go`
- 作用：保守只读还原旧已付费完整文本

```go
func ReadLegacyTextResult(tx *gatewayruntime.ExecutionTransaction, record gatewayruntime.InvocationRecord, evidence provider.ProviderOutcome) (*platform.ModelOutput, error)
```

**输入**

无

**输出**

无

**步骤**

1. 调用方已核真实winning/完整无错HTTP成功evidence；Invocation与实际表都仅一个initial ordinal1 attempt，旧成功终态。
2. 完整原SSE从1到LastSequence连续，最多65540条/每条1MiB/聚合64MiB；accepted首条、completed末条，同原invocation/root/class/deadline/price/winner；不接受错误/取消/额外完整输出分支。
3. 每个原delta有效UTF8且≤16KiB，累计≤64KiB；真实OutputStarted与是否有字节一致；全原流无增量且完整证明时才返回显式空文本。
4. 纯读不改原证据/fee/ledger/paid hash，不填404或0、不重跑；缺任何来源证明明确失败关闭。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：由已完成owner/site/epoch/root授权的Get或Lookup调用。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.EstimateTokens

- 级别：约束
- 文件：`modules/model-gateway/internal/tokens/budget.go`
- 作用：保守估算请求预算

```go
func EstimateTokens(ctx context.Context, op OperationContext, input EstimateTokensInput) (EstimateTokensResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `EstimateTokensInput` | 封闭业务输入；逐字段见types $defs/EstimateTokensInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EstimateTokensResult` | 成功结果；逐字段见types $defs/EstimateTokensResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。fixed-model-input-window-v1仅当前epoch模型上界，结算仍取真实usage。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅localization纯服务且SubjectID=CallerService，可信入口验证一次；纯计算不重做身份或取消轮询。
- 并发：纯函数；ctx只保持统一签名，无IO、锁或取消检查。
- 幂等：同文本/消息给同估算；不存正文或结果。
- 依赖：`model-gateway.ReadInputBudgetModel`

## model-gateway.ValidateEmbeddingSpace

- 级别：参考
- 文件：`modules/model-gateway/internal/tokens/budget.go`
- 作用：阻止错空间向量入库

```go
func ValidateEmbeddingSpace(ctx context.Context, op OperationContext, input ValidateEmbeddingSpaceInput) (ValidateEmbeddingSpaceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateEmbeddingSpaceInput` | 封闭业务输入；逐字段见types $defs/ValidateEmbeddingSpaceInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateEmbeddingSpaceResult` | 成功结果；逐字段见types $defs/ValidateEmbeddingSpaceResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 使用本域唯一ModelEmbeddingSpace重算原五字段有序JSON身份并精确比较spaceId，不更改既有算法
2. 每条原始向量维度必须等于固定space.dimension，逐项拒绝NaN/Infinity
3. 默认预处理none，保留原始向量，不擅自L2归一化；算法/预处理版本改变必须新空间身份

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ModelEmbeddingSpace`

## model-gateway.NewTokenHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/tokens/budget.go`
- 作用：装配localization免费文本预算真实HTTP入口

```go
func NewTokenHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `TokenHandlers` | 真实文本预算HTTP映射 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 公共入口固定localization纯服务，绑定严格JSON后调用EstimateTokens，no-store及公共Problem；不存正文或估算结果。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证入口或模型CAS调用链；不自行取得跨域权限
- 并发：跟随调用者SQL快照或纯计算；无外部供应方IO
- 幂等：不可变价目全值与内容地址可重读，写入仅在模型CAS同事务
- 依赖：`model-gateway.EstimateTokens`

## model-gateway.AcquireProviderSlot

- 级别：参考
- 文件：`modules/model-gateway/internal/concurrency/lease.go`
- 作用：限制供应方并发

```go
func AcquireProviderSlot(ctx context.Context, op OperationContext, input AcquireProviderSlotInput) (AcquireProviderSlotResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `AcquireProviderSlotInput` | 封闭业务输入；逐字段见types $defs/AcquireProviderSlotInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcquireProviderSlotResult` | 成功结果；逐字段见types $defs/AcquireProviderSlotResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 在短权威写事务内核原owner/root/epoch/current reserved attempt，30秒租约固定
2. 清到期slot，分别核真实固定provider和profile并发上限；0上限按数量小于配置规则表示暂无槽位
3. 同调用已有未到期槽位返回原租约，不重复领取；新槽位期限不超过原调用deadline
4. 派发期间执行器每10秒续租，租约未知或取消停止后继供应方IO
5. 显式admin-probe无AI profile，仅核原冻结provider并发；QA/background仍同时核原真实profile并发，不造profile ID。
6. 专用连接probe仅以record.Probe显式内部策略辨识；memory.assistant原admin-probe AI任务保留真实档位、原备用与预算；连接probe日限仅connection_probes真实invocation集合。

**错误**

- `RATE_LIMITED` / HTTP 429：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：持久租约；拒绝超并发
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadAttemptRecord`

## model-gateway.ReleaseProviderSlot

- 级别：参考
- 文件：`modules/model-gateway/internal/concurrency/lease.go`
- 作用：释放供应方槽位

```go
func ReleaseProviderSlot(ctx context.Context, op OperationContext, input ReleaseProviderSlotInput) (ReleaseProviderSlotResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReleaseProviderSlotInput` | 封闭业务输入；逐字段见types $defs/ReleaseProviderSlotInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseProviderSlotResult` | 成功结果；逐字段见types $defs/ReleaseProviderSlotResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 核租约属于原调用，同owner/root/epoch
2. 不存在为released=true，存在仅删除该slot；不改held或账单，不把释放slot当未收费证明

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：删除本服务租约
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReadInvocationRecord`

## model-gateway.PublishOutbox

- 级别：约束
- 文件：`modules/model-gateway/internal/events/outbox.go`
- 作用：持久发布事件

```go
func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PublishOutboxInput` | 封闭业务输入；逐字段见types $defs/PublishOutboxInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishOutboxResult` | 成功结果；逐字段见types $defs/PublishOutboxResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅本域纯服务后台调用RelayOutbox，返回真实published/pending，不伪造payload。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：jobs at-least-once投递
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.RelayOutbox`

## model-gateway.GetLedger

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：管理端查询费用账本

```go
func GetLedger(ctx context.Context, op OperationContext, input GetLedgerInput) (GetLedgerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `GetLedgerInput` | 封闭业务输入；逐字段见types $defs/GetLedgerInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetLedgerResult` | 成功结果；逐字段见types $defs/GetLedgerResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅portal后台admin，经公共入口当前model.manage授权一次；不存在model.billing.read新grant。时间窗[from,until)必须正且最多90天。
2. SQL外验签专用scope游标，绑定caller/actor/site/epoch/session及权限修订、查询窗/limit/costClass；载荷首版/规范编码/精确UTC纳秒位置和int64账本水位。
3. 同一严格当前epoch只读快照先核旧账本/预留归属覆盖：孤立子行422，缺owner/site/created_at/epoch或空固定价目前置503，禁止回填、忽略或当空。UTC账本时间损坏422；读取水位不存在/耗尽失败关闭。 UTC历法/整秒用SQLite内建+0 seconds规范化往返，非法元数据view与部分索引覆盖窗外坏行；只检查小数1..9位，不手写日历或用日期浮点处理纳秒。
4. 筛本站和查询时间窗，精确UTC秒/补齐九位纳秒/entryId keyset取limit+1并验证全部候选；不可变账单与调用原价目全值逐条投影，不按现价重算。游标水位变化409从头；未知金额省略actualCost，不伪造0。
5. 显式结束SQL后才签下一游标，末页省略nextCursor。只读费用事实，不写审计、状态、调用、预留或账单；实际付费生产链仍planned。
6. 恢复旧费用仅经当前target权威投影精确读原结果/费用/原hash/winner，旧scope不变成新准入，具体回调待公共snapshot实现后接。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：model.manage；rule：HTTP仅portal，公共Authorize当前校验一次；内部只读本站真实账本。
- 并发：验签/签名IO均在SQL外；每页固定只读快照及水位，不持事务跨网络。
- 幂等：同查询/同水位稳定keyset，变更后旧游标409；无写入。
- 依赖：`model-gateway.readLedgerPage`

## model-gateway.GetPriceBook

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取历史固定价目

```go
func GetPriceBook(ctx context.Context, op OperationContext, input GetPriceBookInput) (GetPriceBookResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `GetPriceBookInput` | 封闭业务输入；逐字段见types $defs/GetPriceBookInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetPriceBookResult` | 成功结果；逐字段见types $defs/GetPriceBookResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 公共入口对portal当前model.manage一次授权；本函数只接受后台admin委托。
2. 严格当前epoch只读快照核真实模型与价目历史；按不晚于effectiveAt的manual>remote>bundled优先，同source最新生效秒/纳秒/模型版本选择，缺行NOT_FOUND，不猜价格。
3. 读取不可变完整快照并核规范编码/hash、模型/epoch/生效时刻和完整率；unknown仍保持金额缺失和warning；不修改或新增报价记录。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）

- 副作用：无
- 授权：权限 model.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：同一严格当前epoch快照只读历史，无供应方IO或业务写入。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ReadPriceSnapshot`

## model-gateway.ReadModels

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取模型能力声明

```go
func ReadModels(ctx context.Context, op OperationContext, input ReadModelsInput) (ReadModelsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadModelsInput` | 封闭业务输入；逐字段见types $defs/ReadModelsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadModelsResult` | 成功结果；逐字段见types $defs/ReadModelsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 根目录owner纯服务或portal当前model.manage管理员，一次严格当前epoch快照读模型，停用模型仍显示
2. 能力可选过滤，按modelId排序，超过256失败不截断；损坏/异代失败关闭，不探测或读秘密
3. 每条embedding模型配置完整才附可选embeddingSpace五字段；只用MG原有有序JSON算法权威生产，不让消费者跨域复制hash
4. ModelWithSpace同一生产助手附modelSnapshotHash，供缓存筛选，不要求外域复制算法。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：根任务目录owner纯服务，或portal当前model.manage管理员。
- 并发：严格当前epoch快照；必要权限RPC前结束预读事务，短写事务重核回执/屏障/CAS。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ModelEmbeddingSpace`、`model-gateway.ModelSnapshotHash`、`model-gateway.beginAdminSnapshot`、`model-gateway.readCatalogModels`、`model-gateway.requireCatalogReader`
- 其他依赖说明：本域私有目录配置/策略/屏障/原回执/审计表，根唯一AI任务目录生成表

## model-gateway.ReadProfiles

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取档位声明

```go
func ReadProfiles(ctx context.Context, op OperationContext, input ReadProfilesInput) (ReadProfilesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadProfilesInput` | 封闭业务输入；逐字段见types $defs/ReadProfilesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadProfilesResult` | 成功结果；逐字段见types $defs/ReadProfilesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 根任务目录owner纯服务只读可信op.costClass，必须已登记费用类；portal当前model.manage管理员全量，HTTP映射即时授权。
2. 一次严格当前epoch事务，SQL按可选profileId和纯服务costClass过滤，profileId排序；返回真实版本/顺序候选/自身费用。
3. 不合成缺失配置或回退，不暴露秘密/全站余额；超256条、损坏JSON、异代或未知存储报错。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：根任务目录owner纯服务且只可信op.costClass；portal当前model.manage全量。
- 并发：严格当前epoch快照；必要权限RPC前结束预读事务，短写事务重核回执/屏障/CAS。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.requireCatalogReader`、`model-gateway.beginAdminSnapshot`、`model-gateway.readCatalogProfiles`
- 其他依赖说明：本域私有目录配置/策略/屏障/原回执/审计表，根唯一AI任务目录生成表

## model-gateway.ReadFeeBudgetPolicy

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取费用限额配置

```go
func ReadFeeBudgetPolicy(ctx context.Context, op OperationContext, input ReadFeeBudgetPolicyInput) (ReadFeeBudgetPolicyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadFeeBudgetPolicyInput` | 费用配置封闭输入 | 完整字段见types，额外字段/null拒绝。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadFeeBudgetPolicyResult` | 费用配置成功结果 | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. portal当前guard.manage管理员，公共入口一次核身份/schema/权限/期限。
2. 严格当前epoch快照读真实fee_budget_policy并核siteId；缺行NOT_FOUND，不合成old未明确的会话美元默认。
3. 返回完整已存策略；daily0不限、background独立，不读取IP/guard频率/held/ledger，不探测。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：guard.manage；rule：portal当前guard.manage管理员；同siteId。
- 并发：严格当前epoch快照；必要权限RPC前结束预读事务，短写事务重核回执/屏障/CAS。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.requireFeePolicyActor`、`model-gateway.beginAdminSnapshot`、`model-gateway.readFeePolicy`
- 其他依赖说明：本域私有目录配置/策略/屏障/原回执/审计表，根唯一AI任务目录生成表

## model-gateway.PutFeeBudgetPolicy

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：维护old费用开关与会话小时、网络5倍配置

```go
func PutFeeBudgetPolicy(ctx context.Context, op OperationContext, input PutFeeBudgetPolicyInput) (PutFeeBudgetPolicyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutFeeBudgetPolicyInput` | 费用配置封闭输入 | 完整字段见types，额外字段/null拒绝。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutFeeBudgetPolicyResult` | 费用配置成功结果 | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. portal当前guard.manage管理员；原回放前公共入口仍即时核guard.manage，内部不重复schema/身份/期限；版本expectedVersion+1且安全整数。
2. 预读事务先核7天完整原回执；摘要绑定callerService/dataEpoch/policy/expectedVersion。同hash原样返回，不受后续屏障/CAS/日额度变化影响；异hash冲突，损坏失败；过期只随成功新写替换。
3. 预读真实policy并核site/epoch/版本；首次expectedVersion0完整创建或既有daily变化必须先结束预读事务，再调用平台identity当前superadmin，无SQL/进程锁。普通guard.manage仅保留日额度调整小时策略。
4. 授权后开启短写事务，重新先核原回执，再核全部非released屏障、当前policy和CAS；期间需超管而未授权则VERSION_MISMATCH，不自动重试，不持事务RPC。
5. 条件INSERT/UPDATE核RowsAffected1；策略、完整回执及本地fee_policy_audit同事务。审计仅actor/rootOperation/epoch/version/time/日额度变化标记；跨域投影仍未完成，不宣称通知analytics。
6. 仅影响后续预留，不改held/ledger，不调用模型。Commit未知由相同键回读原结果，不自动重试提交。
7. admin-probe独立日限dailyProbeLimit兼容缺字段=用户明确默认0不限；有限额度遇同scope未知held失败关闭；探测限额变更同QA日限须当前superadmin。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：同事务写fee_budget_policy/fee_policy_audit/operation_receipts，不改既有费用；整体跨域审计投影仍待实施。
- 授权：permission：guard.manage；rule：portal当前guard.manage；首次或daily变化另无锁即时superadmin，短事务重核。
- 并发：严格当前epoch快照；必要权限RPC前结束预读事务，短写事务重核回执/屏障/CAS。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.requireFeePolicyActor`、`model-gateway.beginAdminSnapshot`、`model-gateway.readFeePolicy`、`model-gateway.readFeePolicyReceipt`、`identity:checkAuthorization`、`model-gateway.FeeProbeLimit`
- 其他依赖说明：本域私有目录配置/策略/屏障/原回执/审计表，根唯一AI任务目录生成表

## model-gateway.ReadAdminCatalog

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取渠道、模型、档位与任务分配

```go
func ReadAdminCatalog(ctx context.Context, op OperationContext, input ReadAdminCatalogInput) (ReadAdminCatalogResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ReadAdminCatalogInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadAdminCatalogResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. portal当前model.manage管理员，公共HTTP边界即时授权。
2. 同一严格当前epoch快照读取四项目录，按ID排序；只返回真实已存配置，不合成任务默认。
3. ProviderSummary只查非秘密元数据、附加头名称、active凭据版本和材料存在/代次/长度；不SELECT秘密引用或密文正文。
4. 附各供应方启停状态与最近连接测试结果；本轮启停/材料状态已写，最近probe生产者/投影未齐，整函数in_progress，不伪造空或成功结果。
5. 响应上限模型1000、供应方/档位64、任务256；超限报错不截断；异代或损坏失败关闭。
6. 同一SQL快照附真实最近probe及原冻结版本/原调用/实际响应latency；无历史省略lastProbe，不探测或填成功。
7. 仅snapshot_material_redactions有原完整hash来源且nonce/ciphertext都空才视明确未含密钥；普通空值损坏严格失败，管理员新版本配置不复活旧attempt。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：model.manage；rule：后台管理员
- 并发：严格当前epoch快照；必要权限RPC前结束预读事务，短写事务重核回执/屏障/CAS。
- 幂等：只读
- 依赖：`model-gateway.beginAdminSnapshot`、`model-gateway.readCatalogProviders`、`model-gateway.readCatalogModels`、`model-gateway.readCatalogProfiles`、`model-gateway.readCatalogAssignments`、`model-gateway.ReadLatestProbes`
- 其他依赖说明：本域私有目录配置/策略/屏障/原回执/审计表，根唯一AI任务目录生成表

## model-gateway.requireCatalogReader

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：限制目录主体

```go
func requireCatalogReader(ctx context.Context, op OperationContext) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用链上下文；纯函数不检查取消 | 可信内部调用 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. 根任务目录owner纯服务或portal model.manage/superadmin管理员，当前管理权限由HTTP映射核。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：无

## model-gateway.beginAdminSnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：建立严格代次快照

```go
func beginAdminSnapshot(ctx context.Context, op OperationContext) (*sql.Tx, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `tx` | `SQLTransaction` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. CurrentStore.BeginTx后同事务核非空runtime_meta等于op.dataEpoch，失败回滚，不覆盖未知水位。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.Store.BeginTx`、`model-gateway.adminStorageError`

## model-gateway.readCatalogModels

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取模型声明

```go
func readCatalogModels(ctx context.Context, op OperationContext, tx *sql.Tx, capability *string, limit int) ([]gen.Model, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `tx` | `SQLTransaction` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `capability` | `string` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `limit` | `int` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `models` | `[]Model` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. 同代读取完整模型声明/能力JSON，可选能力过滤、modelId排序；超限失败，不计算缺归一化规则的space，不探测。
2. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。catalogRef双列同步读写，旧空值不猜填，固定路线快照保留源引用。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.adminStorageError`

## model-gateway.readCatalogProfiles

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取范围内档位

```go
func readCatalogProfiles(ctx context.Context, op OperationContext, tx *sql.Tx, profileID, costClass *string, limit int) ([]gen.Profile, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `tx` | `SQLTransaction` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `profileID` | `string` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `costClass` | `string` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `limit` | `int` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `profiles` | `[]Profile` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. SQL精确过滤，同代解析真实候选/版本/费用；超限失败，不合成缺失配置或全站余额。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.adminStorageError`

## model-gateway.readCatalogProviders

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取供应方安全投影

```go
func readCatalogProviders(ctx context.Context, op OperationContext, tx *sql.Tx) ([]gen.ProviderSummary, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `tx` | `SQLTransaction` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `providers` | `[]ProviderSummary` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. 不SELECT秘密引用/nonce/密文正文，只核active材料存在/代次/长度；异代坏长度失败，材料缺失false不表示最近探测结果。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.adminStorageError`

## model-gateway.readCatalogAssignments

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取真实任务分配

```go
func readCatalogAssignments(ctx context.Context, op OperationContext, tx *sql.Tx) ([]gen.TaskAssignment, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `tx` | `SQLTransaction` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `assignments` | `[]TaskAssignment` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. 同代/版本/defaultTier/引用完整性检查，taskId排序；缺记录不生成默认配置。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.adminStorageError`

## model-gateway.requireFeePolicyActor

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：限制策略调用范围

```go
func requireFeePolicyActor(ctx context.Context, op OperationContext) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用链上下文；纯函数不检查取消 | 可信内部调用 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. 仅portal/admin，当前guard.manage由公共边界核；日额度特权由真实比较后即时核。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：无

## model-gateway.readFeePolicy

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取真实策略归属

```go
func readFeePolicy(ctx context.Context, op OperationContext, tx *sql.Tx) (FeeBudgetPolicy, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `tx` | `SQLTransaction` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `policy` | `FeeBudgetPolicy` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. 无行NOT_FOUND；核site/epoch/版本/金额范围，不编造默认。
2. admin-probe独立日限dailyProbeLimit兼容缺字段=用户明确默认0不限；有限额度遇同scope未知held失败关闭；探测限额变更同QA日限须当前superadmin。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.adminStorageError`

## model-gateway.readFeePolicyReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：读取完整七天原回执

```go
func readFeePolicyReceipt(ctx context.Context, op OperationContext, tx *sql.Tx, input PutFeeBudgetPolicyInput, inputHash string) (PutFeeBudgetPolicyResult, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `op` | `OperationContext` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `tx` | `SQLTransaction` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `input` | `PutFeeBudgetPolicyInput` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |
| `inputHash` | `string` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutFeeBudgetPolicyResult` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `found` | `bool` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |
| `err` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. site/actor/function/key定位，坏时间失败、过期未命中，同hash解码匹配完整原结果，异hash冲突，不改任何行。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.adminStorageError`

## model-gateway.adminStorageError

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：映射脱敏持久错误

```go
func adminStorageError(err error) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `public` | `error` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. 直接委托公共platform.NormalizeIOError保留平台错误、归类取消/截止并以固定DEPENDENCY_UNAVAILABLE安全常量回退；本域不复制通用映射。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`platform.NormalizeIOError`

## model-gateway.NewAdminHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/read.go`
- 作用：装配五项真实管理HTTP映射

```go
func NewAdminHandlers(authorize AdminAuthorizer) map[string]http.Handler
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `authorize` | `AdminAuthorizer` | 本域可信输入 | 已核入口身份/schema/期限，只实现本函数规则 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `AdminHandlers` | 本域结果或公共错误 | 失败结果不可用，不含秘密正文 |

**步骤**

1. GET绑定可选查询，响应no-store，严格schema由公共入口核。
2. readModels/readProfiles管理员即时model.manage，其余依公共边界权限核一次。
3. Put核幂等头一致，用进程内context注入平台授权回调，供首次/日额度变化超管检查。
4. 仅交统一服务边界，直接取得已存在的可信op；不添加不可达的缺失上下文分支。
5. getPriceBook绑定modelId路径与effectiveAt查询，公共入口即时model.manage授权，no-store。

**错误**

- `FORBIDDEN`：调用主体或当前权限不足（不可重试）
- `VERSION_MISMATCH`：本地水位、配置版本/代次或预授权条件变化（不可重试）
- `NOT_FOUND`：费用策略尚未配置（不可重试）
- `CONFLICT`：幂等键绑定不同输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久元数据或原回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储、代次或即时授权不可用（可重试）
- `BUDGET_EXCEEDED`：结果超过响应上限，不截断伪装完整结果（不可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）
- `INTERNAL`：规范JSON编码失败（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域已授权调用链
- 并发：共用上层快照；纯函数无IO；权限RPC不持SQL事务。
- 幂等：读取或映射无独立副作用。
- 依赖：`model-gateway.ReadModels`、`model-gateway.ReadProfiles`、`model-gateway.ReadAdminCatalog`、`model-gateway.ReadFeeBudgetPolicy`、`model-gateway.PutFeeBudgetPolicy`、`model-gateway.GetPriceBook`、`model-gateway.CreateConnectionProbe`

## model-gateway.QueryUsage

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：跨服务读取权威费用状态

```go
func QueryUsage(ctx context.Context, op OperationContext, input QueryUsageInput) (QueryUsageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `QueryUsageInput` | 封闭业务输入；逐字段见types $defs/QueryUsageInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `QueryUsageResult` | 成功结果；逐字段见types $defs/QueryUsageResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 公共入口model.manage：portal后台admin即时授权；ServiceActorCallers只允许根AI任务owner纯服务。域内核SubjectID=CallerService及catalog owner，不能靠传root取得权限。
2. 单一严格当前epoch只读快照完整读取请求root的全部调用，最多256，逐条核真实owner/site/epoch/created_at/固定价目；无root404，缺前置503、损坏422。纯服务要求全部root同caller，跨owner即403；任何跨站拒绝，不过滤成部分结果。
3. 调用created_at落在[from,until)且当前held的预留汇总为held；ledger.created_at落在窗内账单汇总为settled，均是当前快照，不声称历史时点余额。金额用big.Int精确累加，超Money安全整数或256响应预算失败，不截断。 同root全部预留先核状态只为held/settled/released；非法状态422，不当空；合法非held不汇总，也不套用held金额规则。
4. 某组任一金额未知则整项heldCost或settledCost省略，禁止返回部分已知和冒充总额；unknownInvocationIds包括窗内当前charge_unknown调用、未知held和未知账单，完整去重排序；已知空集合显式0仅表示实际空的已知组。 窗内charge_unknown调用必须有实际held行或未知金额账单证明；缺任一必要证据503，不返回无证据held=0。
5. 返回每条账单原固定priceSnapshot全值，不按当前价重算；只读且无模型、额度、预留、账本或审计副作用。
6. 恢复旧费用仅经当前target权威投影精确读原结果/费用/原hash/winner，旧scope不变成新准入，具体回调待公共snapshot实现后接。
7. analytics纯服务可读同实际site/current epoch安全用量，不限AI owner原root归属，只返统计DTO不带正文/输出/秘密。其他AI owner仍只读自己全root，不扩新付费权限。
8. 兼容可选windowCoversRoot实际必给：当前SQL快照窗覆盖全部原root调用创建时间与所有账本时间（含已superseded/voided历史）才true。
9. 兼容可选allInvocationsFinal实际必给：完整原调用/尝试记录hash及列表精确一致，原TerminalNS真实且无活动lease，outer succeeded仍须全部attempt succeeded/failed/reconciled；缺事实false或严格原错误。
10. 两标记仅当前原root观察，不把safeToRetry/held0/零金额替新批准收费；消费者另需unknown空、held显式0、所有ledger金额已知及明确新用户批准。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：model.manage；rule：portal当前model.manage或列入平台ServiceActorCallers的AI task owner纯服务；纯服务root全部持久归属必须同caller/site/epoch。
- 并发：整个查询共享一次严格当前epoch的SQL读快照，不跨RPC。
- 幂等：只读当前快照，不补旧归属、重算价格或生成费用状态。
- 依赖：`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadStatisticsInvocationRecord`、`model-gateway.beginAdminSnapshot`、`model-gateway.ledgerRuntime`、`model-gateway.readLedgerEntries`、`model-gateway.readRootMetadata`、`model-gateway.sumLedgerMoney`、`model-gateway.validateLedgerCoverage`

## model-gateway.ledgerRuntime

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：读取真实签名与价目schema依赖

```go
func ledgerRuntime(ctx context.Context, op OperationContext) (ledgerReadRuntime, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `runtime` | `LedgerReadRuntime` | 真实只读依赖 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 读取真实签名与价目schema依赖

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.validateLedgerCoverage

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：核账本/预留到调用的完整覆盖、旧行必要归属和UTC时刻，缺前置不伪空

```go
func validateLedgerCoverage(ctx context.Context, op OperationContext, tx *sql.Tx) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `tx` | `SQLTransaction` | 当前快照 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 核账本/预留到调用的完整覆盖、旧行必要归属和UTC时刻，缺前置不伪空
2. 只查非法UTC时间元数据view/部分索引；SQLite内建strftime取前19位加0秒规范化往返，单独核小数ASCII位与末Z；不得因时间窗过滤隐去坏行，不扫描正文。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.readLedgerRevision

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：读真实int64分页水位，不猜补缺失

```go
func readLedgerRevision(ctx context.Context, op OperationContext, tx *sql.Tx) (int64, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `tx` | `SQLTransaction` | 当前快照 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `revision` | `int64` | 固定水位 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 读真实int64分页水位，不猜补缺失

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.readLedgerMetadata

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：一次核持久调用归属/时刻/状态及原价目，缺前置503、矛盾422

```go
func readLedgerMetadata(ctx context.Context, op OperationContext, runtime ledgerReadRuntime, id, owner, site, epoch, root, costClass, state, created string, price []byte) (ledgerReadMetadata, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `runtime` | `LedgerReadRuntime` | 本域schema | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `id` | `string` | 调用ID | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `owner` | `string` | 原owner | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `site` | `string` | 原站点 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `epoch` | `string` | 原代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `root` | `string` | 原根 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `costClass` | `string` | 可信费用类 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `state` | `string` | 原状态 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `created` | `string` | 原创建时刻 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `price` | `JSONBytes` | 原固定价目 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `metadata` | `LedgerReadMetadata` | 已核持久归属投影 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 一次核持久调用归属/时刻/状态及原价目，缺前置503、矛盾422

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.readLedgerEntries

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：读取并一次投影真实账单和调用原价目；所有业务筛选值参数绑定

```go
func readLedgerEntries(ctx context.Context, op OperationContext, tx *sql.Tx, runtime ledgerReadRuntime, condition string, args []any, limit int) ([]gen.LedgerEntry, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `tx` | `SQLTransaction` | 当前快照 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `runtime` | `LedgerReadRuntime` | schema依赖 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `condition` | `string` | 本域固定SQL片段 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `args` | `LedgerReadArguments` | 参数值 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `limit` | `int` | 含lookahead的上限 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `entries` | `LedgerReadEntries` | 完整真实条目 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 读取并一次投影真实账单和调用原价目；所有业务筛选值参数绑定

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.readLedgerMetadata`、`model-gateway.IsRestoredEpoch`

## model-gateway.readLedgerPage

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：验签绑定游标、严格同代固定水位精确keyset、SQL外签下一页

```go
func readLedgerPage(ctx context.Context, op OperationContext, input GetLedgerInput) (GetLedgerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `input` | `GetLedgerInput` | 可信查询 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetLedgerResult` | 分页 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 验签绑定游标、严格同代固定水位精确keyset、SQL外签下一页

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ledgerRuntime`、`model-gateway.beginAdminSnapshot`、`model-gateway.validateLedgerCoverage`、`model-gateway.readLedgerRevision`、`model-gateway.readLedgerEntries`

## model-gateway.readRootMetadata

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：完整root最多256，逐全部归属核同站点/epoch，服务多owner拒绝

```go
func readRootMetadata(ctx context.Context, op OperationContext, tx *sql.Tx, runtime ledgerReadRuntime, root string) ([]ledgerReadMetadata, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `tx` | `SQLTransaction` | 当前快照 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `runtime` | `LedgerReadRuntime` | schema依赖 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `root` | `string` | 请求根 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `metadata` | `LedgerReadMetadataList` | 完整根归属 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 完整root最多256，逐全部归属核同站点/epoch，服务多owner拒绝

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.readLedgerMetadata`、`model-gateway.IsRestoredEpoch`

## model-gateway.sumLedgerMoney

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：仅将精确非负且不超安全整数的总和投影Money

```go
func sumLedgerMoney(ctx context.Context, op OperationContext, total *big.Int) (*gen.Money, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 贯穿IO取消 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `op` | `OperationContext` | 可信当前主体/站点/代次 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |
| `total` | `LedgerInteger` | 精确已知总和 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `money` | `Money` | 完整已知总额 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 仅将精确非负且不超安全整数的总和投影Money

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.NewLedgerHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/usage.go`
- 作用：构造价目持久schema与真实HTTP绑定，不在领域重复公共授权

```go
func NewLedgerHandlers(key platform.SigningKey) (map[string]http.Handler, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `key` | `SigningKey` | 可信启动签名引用 | 来自可信调用链或本次私有持久快照；不从浏览器猜补归属 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `LedgerReadHandlers` | 两项真实HTTP映射 | 失败时结果不可用，不含秘密或正文 |
| `err` | `error` | 公共错误 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 构造价目持久schema与真实HTTP绑定，不在领域重复公共授权
2. limit按与kin-openapi入口一致的ParseInt(raw,0,64)转换已验证整数，避免十六进制/八进制输入被转成0或不同值。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.GetLedger`、`model-gateway.QueryUsage`、`model-gateway.ReadUsageStatisticsPage`

## model-gateway.AcquireBarrier

- 级别：约束
- 文件：`modules/model-gateway/internal/protection/protocol.go`
- 作用：取得本地维护屏障

```go
func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BarrierRequest` | 封闭业务输入；逐字段见types $defs/BarrierRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 成功结果；逐字段见types $defs/BarrierStatus | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验data-protection服务身份（用户授权已由data-protection核验）
2. 将本地control.db屏障draining并拒绝新业务写
3. 屏障停止新预留/新派发
4. 在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封
5. 部署主密钥不导出。缺密钥恢复后所有供应方disabled。
6. 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。
7. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：持久控制屏障；不纳入业务快照
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.EnterBarrier`

## model-gateway.ReleaseBarrier

- 级别：约束
- 文件：`modules/model-gateway/internal/protection/protocol.go`
- 作用：释放已授权维护屏障

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BarrierRequest` | 封闭业务输入；逐字段见types $defs/BarrierRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 成功结果；逐字段见types $defs/BarrierStatus | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验相同barrierId/operationId与data-protection服务身份
2. 暂存目录不影响释放；切换只在进程重启时发生
3. runtime处于restore_switching或restore_rolling_back时不恢复业务worker，等待重启换目录
4. released后开放该epoch业务写。
5. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：更新本地control.db；不修改导入业务
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.ReleaseBarrier`

## model-gateway.ExportSnapshot

- 级别：约束
- 文件：`modules/model-gateway/internal/protection/protocol.go`
- 作用：导出安全watermark快照

```go
func ExportSnapshot(ctx context.Context, op OperationContext, input SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SnapshotExportRequest` | 封闭业务输入；逐字段见types $defs/SnapshotExportRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 成功结果；逐字段见types $defs/SnapshotExportResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 验证safe屏障及dataEpoch
2. checkpoint本域私有数据并固定revisionWatermark
3. 屏障停止新预留/新派发
4. 在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封
5. 部署主密钥不导出。缺密钥恢复后所有供应方disabled。
6. 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。
7. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：创建本服务不可变快照对象
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.Export`

## model-gateway.StageSnapshot

- 级别：约束
- 文件：`modules/model-gateway/internal/protection/protocol.go`
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
5. 不通用重基原invocation/operation/route/feeBinding/价目/执行hash；对真实原费用建立RestoreInvocationProjection绑targetEpoch+原invocation/sourceEpoch/executionHash+owner/site/root+restore/backup/archive/manifest证据。
6. 多次恢复sourceEpoch仍保原费用身份，sourceBackupEpoch为本次归档active epoch；不同时需认证归档内原合法投影，不伪造继承。
7. 缺原不可变证据/owner/site绑定即Stage/Ready失败并保源数据，不以空投影伪成功。

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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`、`platform.SnapshotManager.Stage`

## model-gateway.DiscardSnapshot

- 级别：约束
- 文件：`modules/model-gateway/internal/protection/protocol.go`
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
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.Discard`

## model-gateway.PutTaskAssignment

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：超管维护单任务模型覆盖

```go
func PutTaskAssignment(ctx context.Context, op OperationContext, input PutTaskAssignmentInput) (PutTaskAssignmentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutTaskAssignmentInput` | 封闭业务输入；逐字段见types $defs/PutTaskAssignmentInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutTaskAssignmentResult` | 成功结果；逐字段见types $defs/PutTaskAssignmentResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅已验证admin且含superadmin授权可写；入口已统一完成服务身份、identity当前授权、schema、期限与权威epoch校验，内部不重复边界解析
2. taskId必须在根contracts/ai-tasks.json生成的AITaskCatalog；assignment.tier必须等于目录defaultTier，assignment.version必须为expectedVersion+1，不修改任务目录默认档位
3. 使用beginAvailabilitySnapshot在同一事务核本地非空当前epoch；先读有效原回执，再检查任何非released屏障；同代有效重放不产生写入，屏障只阻止新配置事务
4. 幂等键按siteId+actorId+函数ID+key定位，规范输入摘要同时绑定callerService、dataEpoch、assignment与expectedVersion；7天内同hash返回原始回执而不重查可能已停用的覆盖模型，不同hash为CONFLICT；坏时间或原结果不匹配为INTEGRITY_ERROR，过期只视为未命中，确认无屏障后仅在同一成功事务末尾替换旧回执
5. 设置override时只读本域models的enabled/capabilities/dataEpoch，必须存在、已启用、同epoch并满足目录RequiredCapabilities全部能力；不读凭据、不要求价格或在线探测；省略override清除覆盖
6. 任务不存在仅expectedVersion=0可创建，存在行必须同epoch；SQL INSERT ON CONFLICT DO NOTHING或带version/dataEpoch的UPDATE并核RowsAffected=1执行CAS，冲突返回VERSION_MISMATCH，不覆写异代行
7. 同事务保存assignment、原结果回执与平台outbox assignment.changed/v1，载荷只含taskId/version/dataEpoch；提交后才返回，失败回滚，Commit未知不重试或重建操作

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CONFLICT`：幂等键绑定不同规范输入或代次（不可重试）
- `INTEGRITY_ERROR`：持久配置或回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本地权威代次或存储不可用（可重试）

- 副作用：同事务写task_assignments、operation_receipts、nw_outbox；不调用供应方、不创建费用或后台任务
- 授权：permission：superadmin；rule：已验证admin且含superadmin；HTTP入口以identity核实当前授权，服务身份不替代管理员委托。
- 并发：严格同epoch SQL事务；本地非released屏障失败关闭；任务版本条件写并核受影响行数，幂等键与outbox唯一约束。
- 幂等：siteId+actorId+函数ID+key唯一；摘要含callerService/dataEpoch/规范assignment/expectedVersion；7天原结果先于屏障与当前模型校验返回且无写入；异hash CONFLICT，过期仅同成功新配置事务替换。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.beginAvailabilitySnapshot`、`model-gateway.readTaskAssignmentReceipt`、`model-gateway.validateTaskOverride`、`platform.EnqueueEvent`、`model-gateway.availabilityReadError`
- 其他依赖说明：根AI任务目录生成表AITaskCatalog；本域runtime_meta/barriers/task_assignments/models/operation_receipts/nw_outbox

## model-gateway.ReadTaskAssignments

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：读取任务模型映射

```go
func ReadTaskAssignments(ctx context.Context, op OperationContext, input ReadTaskAssignmentsInput) (ReadTaskAssignmentsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadTaskAssignmentsInput` | 封闭业务输入；逐字段见types $defs/ReadTaskAssignmentsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadTaskAssignmentsResult` | 成功结果；逐字段见types $defs/ReadTaskAssignmentsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只接受根AI目录owner范围内纯服务actor；按callerService与生成目录owner过滤，portal管理读取另要求admin+superadmin；其他调用方拒绝
2. 显式taskId必须在根目录且属于调用方owner，否则分别VALIDATION/FORBIDDEN；不因不存在赋值而放宽服务范围
3. beginAvailabilitySnapshot取得同一严格当前epoch快照；读取本域task_assignments按taskId排序，不合成默认已配置记录，未配置返回非null空数组
4. 持久行校验根taskId、目录defaultTier、正版本与覆盖引用；损坏INTEGRITY_ERROR，所读行异代VERSION_MISMATCH；不读取其他owner的持久值
5. 输出固定version及已存可选覆盖字段；不读密钥、费用、供应方或执行模型，不创建任务或回执

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION`：指定任务未登记（不可重试）
- `VERSION_MISMATCH`：本地或任务行代次不匹配（不可重试）
- `INTEGRITY_ERROR`：持久任务配置损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：当前权威快照未知（可重试）

- 副作用：无
- 授权：permission：service；rule：纯服务仅callerService=任务owner的范围；portal admin+superadmin可读全部；跨owner显式单项FORBIDDEN。
- 并发：一次短SQL快照；按owner过滤后投影，不以未知或异代数据返回空结果。
- 幂等：纯读取当前配置，无写入或费用副作用。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.beginAvailabilitySnapshot`、`model-gateway.availabilityReadError`
- 其他依赖说明：根AI任务目录生成表AITaskCatalog；本域runtime_meta/task_assignments只读SQL快照

## model-gateway.ReadTaskAvailability

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：免费读取QA任务配置能力

```go
func ReadTaskAvailability(ctx context.Context, op OperationContext, input ReadTaskAvailabilityInput) (ReadTaskAvailabilityResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消和截止 | 已在服务入口严格校验；内部不重复schema解析 |
| `op` | `OperationContext` | 已验证服务身份、当前epoch和期限 | 已在服务入口严格校验；内部不重复schema解析 |
| `input` | `ReadTaskAvailabilityInput` | 严格封闭输入 | 已在服务入口严格校验；内部不重复schema解析 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadTaskAvailabilityResult` | 安全完整响应 | 已在服务入口严格校验；内部不重复schema解析 |
| `error` | `Error` | 公共错误，不返回私密原值 | 已在服务入口严格校验；内部不重复schema解析 |

**步骤**

1. 仅接受已验证callerService=qa且actor.kind=service；请求任务固定qa.preflight/qa.inspect/qa.answer，不要求此免费操作op.costClass=qa，不授予执行权限
2. 通过已安装privateStore开始一次只读SQL快照；runtime_meta代次空或缺失为DEPENDENCY_UNAVAILABLE，异代为VERSION_MISMATCH，不沿用允许空水位的通用CheckDataEpoch
3. 按请求顺序逐项读同代task_assignments及cost_class=qa、tier匹配的唯一profiles；缺任务分配或QA档位才返回not_configured，不从根默认值猜已配置
4. 有overrideModelId时替换orderedModelIds首项，保留第二项起备用顺序并去重；原列表空时覆盖模型为唯一候选，不把被替换主模型加回备用；公开配置最多256项，完整配置先作本地eligible过滤，不先截断为16项
5. 候选仅核模型启用/chat能力、供应方启用、当前credentialVersion同代凭据存在及AES-GCM材料长度；附加头只核已持久名称与引用数组结构一致，不取秘密正文、密文、主密钥或供应方地址
6. 已配置但无可用候选返回unavailable，有候选元数据具备返回available；坏JSON/私有引用矛盾为INTEGRITY_ERROR，查询或缺表未知为DEPENDENCY_UNAVAILABLE，所读引用行异代失败关闭，不用false掩盖未知
7. 所有任务共享当前快照，无写入、费用/额度/价格读取或供应方HTTP；结果仅taskId/available/reason，no-store，不返回模型或供应方ID、URL、凭据引用、预算/余额，不保证可解密或在线

**错误**

- `VALIDATION`：请求不符合协议（不可重试）
- `UNAUTHENTICATED`：服务证明无效（不可重试）
- `FORBIDDEN`：非固定调用服务或越过只读范围（不可重试）
- `VERSION_MISMATCH`：当前dataEpoch不匹配（不可重试）
- `INTEGRITY_ERROR`：私有权威数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖或存储未知（可重试）
- `CANCELLED`：查询上下文已取消（不可重试）
- `DEADLINE_EXCEEDED`：入口原截止已到（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接受已验证纯服务actor，固定调用方qa；不授予模型调用或管理写入权限
- 并发：当前权威快照一致读取；不持锁执行跨服务IO
- 幂等：无写入或费用副作用；每次读当前权威，no-store，不创建后台任务
- 依赖：`model-gateway.CurrentStore`、`model-gateway.beginAvailabilitySnapshot`、`model-gateway.readAvailabilityTask`
- 其他依赖说明：同一privateStore只读SQL快照：runtime_meta/task_assignments/profiles/models/providers/credentials；仅本域私有表

## model-gateway.beginAvailabilitySnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：建立严格当前代次任务配置事务快照

```go
func beginAvailabilitySnapshot(ctx context.Context, op OperationContext, store *PrivateStore) (*availabilitySnapshot, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 原入口取消/截止 | 可信本域装配或已通过入口的只读链路使用 |
| `op` | `OperationContext` | 可信QA调用及当前代次 | 可信本域装配或已通过入口的只读链路使用 |
| `store` | `PrivateStore` | 当前安装的本域Store | 可信本域装配或已通过入口的只读链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `snapshot` | `availabilitySnapshot` | 一次只读事务与代次 | 错误时不使用结果；不暴露私有配置原值 |
| `err` | `error` | 公共错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 调用既有Store.BeginTx；开始事务本身不写业务字段
2. 同事务读取runtime_meta singleton，空/缺失为503，异代409；失败回滚
3. 返回句柄由上层defer Rollback结束；只读共用快照，写入链在同事务检查屏障、CAS、回执/outbox后提交

**错误**

- `DEPENDENCY_UNAVAILABLE`：数据库或查询未知（可重试）
- `INTEGRITY_ERROR`：私有配置损坏（不可重试）
- `VERSION_MISMATCH`：快照代次不匹配（不可重试）
- `CANCELLED`：查询已取消（不可重试）
- `DEADLINE_EXCEEDED`：查询已到原截止（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域任务与档位配置链路，调用入口分别落实superadmin、owner服务或固定QA免费读取范围。
- 并发：一次SQL事务；不持事务执行网络IO。
- 幂等：建立事务无独立副作用；业务写入由调用方同事务决定提交或回滚。
- 依赖：`model-gateway.Store.BeginTx`、`model-gateway.availabilityReadError`

## model-gateway.readAvailabilityTask

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：从同一快照判定一个QA任务配置

```go
func readAvailabilityTask(ctx context.Context, snapshot *availabilitySnapshot, taskID string) (TaskAvailability, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 原查询上下文 | 可信本域装配或已通过入口的只读链路使用 |
| `snapshot` | `availabilitySnapshot` | 严格代次事务 | 可信本域装配或已通过入口的只读链路使用 |
| `taskID` | `string` | 入口已校验固定任务ID | 可信本域装配或已通过入口的只读链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TaskAvailability` | 单项安全投影 | 错误时不使用结果；不暴露私有配置原值 |
| `err` | `error` | 公共错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 读取同代assignment，缺行not_configured
2. 读取当前epoch/cost_class=qa/tier唯一profile，缺行not_configured；只读候选JSON，不读费用字段
3. 解析最多256个有序唯一Identifier，坏JSON/null/重复/越界为422
4. 覆盖只替换首项，保留原第二项起备用并去重；空档位列表可仅覆盖
5. 按顺序核所有候选元数据，任何未知失败关闭；至少一个可用为available，否则unavailable

**错误**

- `DEPENDENCY_UNAVAILABLE`：数据库或查询未知（可重试）
- `INTEGRITY_ERROR`：私有配置损坏（不可重试）
- `VERSION_MISMATCH`：快照代次不匹配（不可重试）
- `CANCELLED`：查询已取消（不可重试）
- `DEADLINE_EXCEEDED`：查询已到原截止（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域可信装配或ReadTaskAvailability已核纯QA服务链路
- 并发：一次短SQL快照；不持本域锁执行网络IO
- 幂等：只读每次读取当前快照，不创建费用或操作回执
- 依赖：`model-gateway.readAvailabilityCandidate`、`model-gateway.availabilityReadError`

## model-gateway.readAvailabilityCandidate

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：只读取候选配置存在性与能力

```go
func readAvailabilityCandidate(ctx context.Context, snapshot *availabilitySnapshot, modelID string) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 原查询上下文 | 可信本域装配或已通过入口的只读链路使用 |
| `snapshot` | `availabilitySnapshot` | 当前快照 | 可信本域装配或已通过入口的只读链路使用 |
| `modelID` | `string` | 仅内部候选引用 | 可信本域装配或已通过入口的只读链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `available` | `bool` | 元数据配置具备 | 错误时不使用结果；不暴露私有配置原值 |
| `err` | `error` | 公共错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 复用readRouteCandidate在原QA免费快照内仅要求chat安全元数据，投影eligible布尔；不返回模型、供应方版本或凭据引用，不增加执行/费用权限。

**错误**

- `DEPENDENCY_UNAVAILABLE`：数据库或查询未知（可重试）
- `INTEGRITY_ERROR`：私有配置损坏（不可重试）
- `VERSION_MISMATCH`：快照代次不匹配（不可重试）
- `CANCELLED`：查询已取消（不可重试）
- `DEADLINE_EXCEEDED`：查询已到原截止（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域可信装配或ReadTaskAvailability已核纯QA服务链路
- 并发：一次短SQL快照；不持本域锁执行网络IO
- 幂等：只读每次读取当前快照，不创建费用或操作回执
- 依赖：`model-gateway.readRouteCandidate`

## model-gateway.availabilityReadError

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：保留取消/截止并将其他查询未知映射503

```go
func availabilityReadError(err error) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | SQL或既有Store错误 | 可信本域装配或已通过入口的只读链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 脱敏公共错误 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 保留context取消与截止或既有相应公共错误
2. 其他数据库错误只输出固定503，不携SQL、表值、路径或驱动原文

**错误**

- `DEPENDENCY_UNAVAILABLE`：读取未知（可重试）
- `CANCELLED`：已取消（不可重试）
- `DEADLINE_EXCEEDED`：已到截止（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅本域任务配置读写链路，调用入口分别落实superadmin、owner服务或固定QA免费读取范围。
- 并发：纯错误投影，无锁或IO
- 幂等：只读每次读取当前快照，不创建费用或操作回执
- 依赖：无

## model-gateway.NewTaskAvailabilityHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：创建任务配置读写及免费可用性读取真实HTTP映射

```go
func NewTaskAvailabilityHandlers(authorize TaskAssignmentAuthorizer) map[string]http.Handler
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `authorize` | `TaskAssignmentAuthorizer` | 平台identity当前授权回调 | 从本域既有identity客户端装配，仅HTTP管理员读取调用；缺失失败关闭 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `AvailabilityHandlers` | 单项真实HTTP映射 | 错误时不使用结果；不暴露私有配置原值 |

**步骤**

1. 映射readTaskAvailability、putTaskAssignment、readTaskAssignments；从平台读取可信OperationContext
2. 绑定公共入口已严格校验的JSON或query；本域写入绑定校验taskId路径与assignment.taskId，以及非空Idempotency-Key头与正文键一致
3. readTaskAssignments的admin入口额外通过平台authorize核superadmin当前会话授权；纯owner服务不做管理员回查；写入口由公共ServiceBoundaryConfig.Authorize统一校验一次
4. 调用真实本域任务函数，所有响应no-store，错误输出公共Problem，缺其余handler仍不能启动完整服务

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：交公共完整HTTP边界；具体函数分别校验QA、owner服务或superadmin范围。
- 并发：每请求独立事务；不构造部分可运行服务。
- 幂等：HTTP仅绑定；写入由真实函数持久回执与CAS保证。
- 依赖：`model-gateway.ReadTaskAvailability`、`model-gateway.PutTaskAssignment`、`model-gateway.ReadTaskAssignments`、`identity:checkAuthorization`

## model-gateway.readTaskAssignmentReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：读取7天任务赋值原始回执并验证准确结果

```go
func readTaskAssignmentReceipt(ctx context.Context, op OperationContext, snapshot *availabilitySnapshot, input PutTaskAssignmentInput, inputHash platform.Hash) (PutTaskAssignmentResult, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域已验证任务配置链路使用 |
| `op` | `OperationContext` | 可信调用范围 | 本域已验证任务配置链路使用 |
| `snapshot` | `availabilitySnapshot` | 当前同代事务 | 本域已验证任务配置链路使用 |
| `input` | `PutTaskAssignmentInput` | 原请求 | 本域已验证任务配置链路使用 |
| `inputHash` | `Hash` | 含代次与调用方摘要 | 本域已验证任务配置链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutTaskAssignmentResult` | 原结果 | 失败结果不可使用；不输出私有原值 |
| `found` | `bool` | 是否命中有效原回执 | 失败结果不可使用；不输出私有原值 |
| `err` | `error` | 脱敏公共错误 | 失败结果不可使用；不输出私有原值 |

**步骤**

1. 按site/actor/函数/key读取hash、原结果与创建时间，缺行表示未执行
2. 坏时间失败关闭；超过7天仅视为未命中，不删除或提交；旧回执由上层在released且新配置成功事务末尾替换
3. 有效期内异hash CONFLICT；解码原结果并与本次规范assignment核同，损坏INTEGRITY_ERROR；返回已存原结果

**错误**

- `VALIDATION`：覆盖模型未启用或能力不满足（不可重试）
- `VERSION_MISMATCH`：模型或回执代次不匹配（不可重试）
- `CONFLICT`：幂等输入不一致（不可重试）
- `INTEGRITY_ERROR`：持久模型能力或回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域查询未知（可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅PutTaskAssignment已核superadmin的同事务链路。
- 并发：与PutTaskAssignment共用当前SQL事务；不执行跨域IO。
- 幂等：由上层同事务原回执与CAS保证。
- 依赖：`model-gateway.availabilityReadError`

## model-gateway.validateTaskOverride

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/task.go`
- 作用：校验任务覆盖或档位候选同代启用与目录能力

```go
func validateTaskOverride(ctx context.Context, snapshot *availabilitySnapshot, modelID string, requiredCapabilities []string) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消 | 本域已验证任务配置链路使用 |
| `snapshot` | `availabilitySnapshot` | 当前同代事务 | 本域已验证任务配置链路使用 |
| `modelID` | `string` | 覆盖模型ID | 本域已验证任务配置链路使用 |
| `requiredCapabilities` | `string[]` | 唯一目录生成能力集合 | 本域已验证任务配置链路使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `err` | `error` | 脱敏公共错误 | 失败结果不可使用；不输出私有原值 |

**步骤**

1. 只SELECT enabled/capabilities_json/data_epoch；缺模型或停用VALIDATION，异代VERSION_MISMATCH
2. 解码持久能力数组并核集合结构，损坏INTEGRITY_ERROR；满足全部目录requiredCapabilities才成功
3. 不读供应方地址、密钥、价格或余额，不探测模型

**错误**

- `VALIDATION`：覆盖模型未启用或能力不满足（不可重试）
- `VERSION_MISMATCH`：模型或回执代次不匹配（不可重试）
- `CONFLICT`：幂等输入不一致（不可重试）
- `INTEGRITY_ERROR`：持久模型能力或回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域查询未知（可重试）
- `CANCELLED`：IO取消（不可重试）
- `DEADLINE_EXCEEDED`：IO到截止（不可重试）

- 副作用：无
- 授权：permission：superadmin；rule：仅PutTaskAssignment或PutProfile已核superadmin的同事务链路。
- 并发：与PutTaskAssignment或PutProfile共用当前SQL事务；不执行跨域IO。
- 幂等：由上层同事务原回执与CAS保证。
- 依赖：`model-gateway.availabilityReadError`

## model-gateway.RefreshModelCatalog

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/refresh.go`
- 作用：刷新公开模型目录

```go
func RefreshModelCatalog(ctx context.Context, op OperationContext, input RefreshModelCatalogInput) (RefreshModelCatalogResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RefreshModelCatalogInput` | 封闭业务输入；逐字段见types $defs/RefreshModelCatalogInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RefreshModelCatalogResult` | 成功结果；逐字段见types $defs/RefreshModelCatalogResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 管理员显式刷新或MG内部免费每日调度，固定https://models.dev/api.json单GET最多10MiB/30s，不携站点数据，禁重定向。
2. 解析公开API明确费率/上界，不把缺费率或FixedCost变成0；上游扩展字段不变成本站能力事实。
3. 同短写事务内核epoch/屏障/CAS，不可变下载版本、每模型规范摘要、模型CAS/价目/事件/回执原子提交，单条无变不刷全部模型版本。
4. 本次用户批准models.dev MIT公开数据固定编译快照；只采用原始JSON与来源/版本/hash/原许可，不复制外部业务代码。
5. 先固定公开远端下载并走同一Normalize；只在下载不可用时使用当前epoch最近已验证持久normalized源，没有持久源才用经过真实hash校验的编译源；远端损坏/MIME异常不冒成功。
6. 缓存保留原origin/sourceHash/downloaded时刻；新真实远端origin/rawHash变化也追加下载记录，即使规范业务事实相同。返回source/sourceVersion，降级时明确fallback=cached或bundled。
7. Apply同一真实source事实/人工override优先，无catalogRef旧配置不改；bundled未给固定每次费用仍nil，不补0；同原模型CAS/价目/事件/幂等回执事务。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：下载公开目录不发送站点数据
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.DownloadModelCatalog`、`model-gateway.NormalizeModelCatalog`、`model-gateway.ApplyCatalogSnapshot`、`model-gateway.BeginExecution`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.SaveExecutionReceipt`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.ReadCatalogSource`

## model-gateway.DownloadModelCatalog

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/refresh.go`
- 作用：DownloadModelCatalog明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

```go
func DownloadModelCatalog(ctx context.Context) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.NormalizeModelCatalog

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/refresh.go`
- 作用：NormalizeModelCatalog明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

```go
func NormalizeModelCatalog(raw []byte) (CatalogSnapshot, error)
```

**输入**

无

**输出**

无

**步骤**

1. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ProviderReportedCost`

## model-gateway.ApplyCatalogSnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/refresh.go`
- 作用：ApplyCatalogSnapshot明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

```go
func ApplyCatalogSnapshot(tx *gatewayruntime.ExecutionTransaction, op OperationContext, snapshot CatalogSnapshot, origin string) (int, int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.SaveCatalogPrice`、`model-gateway.ExecutionStorageError`、`model-gateway.EnqueueExecutionEvent`

## model-gateway.SaveCatalogPrice

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/refresh.go`
- 作用：SaveCatalogPrice明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

```go
func SaveCatalogPrice(tx *gatewayruntime.ExecutionTransaction, op OperationContext, modelID string, version int, price billing.ManualPrice, origin string) error
```

**输入**

无

**输出**

无

**步骤**

1. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.MakePriceSnapshot`、`model-gateway.ExecutionStorageError`

## model-gateway.NewCatalogRefreshHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/refresh.go`
- 作用：NewCatalogRefreshHandlers明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

```go
func NewCatalogRefreshHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.RefreshModelCatalog`

## model-gateway.ReadCatalogSource

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/refresh.go`
- 作用：真实公开源下载及持久缓存/编译固定源选择

```go
func ReadCatalogSource(ctx context.Context, op OperationContext) (CatalogSource, error)
```

**输入**

无

**输出**

无

**步骤**

1. 远端成功Normalize并保存真实raw hash；远端不可用先读当前代最近持久normalized与hash，损坏失败关闭；无持久才校验编译原始数据hash并走同一Normalize。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.DownloadModelCatalog`、`model-gateway.NormalizeModelCatalog`、`model-gateway.ReadSnapshot`、`model-gateway.BeginExecution`

## model-gateway.CreateConnectionProbe

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/probe.go`
- 作用：显式测试连接

```go
func CreateConnectionProbe(ctx context.Context, op OperationContext, input CreateConnectionProbeInput) (CreateConnectionProbeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateConnectionProbeInput` | 封闭业务输入；逐字段见types $defs/CreateConnectionProbeInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateConnectionProbeResult` | 成功结果；逐字段见types $defs/CreateConnectionProbeResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅portal当前superadmin明确点击，私有admin-probe lane，公开Create仍仅QA/background task owner；不可伪造task/profile/QA许可。
2. 固定OK极小消息与最多1输出token；embedding仅单OK，rerank仅单候选OK；按模型已声明能力和实际adapter选择，不读取站点正文。
3. 持久内部version1 admin-probe执行策略含approvedBy/permissionRevision，profile_json按costClass明确tagged union且原executionHash覆盖；无AI profile与task assignment。
4. 原root/site/epoch/模型/渠道/密钥和额外头引用/price全部冻结；模型及provider/credential版本同写事务CAS，原输入/原回执/accepted/outbox/引用原子提交。
5. 真实平台TaskGroup执行单attempt，明确禁止fallback/structured retry；未知费用沿原held，不重付；原idempotency仅接续未派发阶段或返回当前原调用。
6. 独立dailyProbeLimit缺失按用户明确默认0不限；有限额度遇未知最大金额或原admin-probe未知held失败关闭，不占QA或background额度。
7. 最近探测只读真实持久调用/attempt/响应时间，原版本明确展示，不凭空探测成功。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG真实供应方调用可能收费；当前设计不执行
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.CreateProbeInvocation`、`model-gateway.beginAdminSnapshot`、`model-gateway.readCatalogModels`

## model-gateway.ReadLatestProbes

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/probe.go`
- 作用：ReadLatestProbes实际探测执行支持

```go
func ReadLatestProbes(ctx context.Context, op OperationContext, tx *sql.Tx, providers []gen.ProviderSummary) error
```

**输入**

无

**输出**

无

**步骤**

1. 同真实管理快照取每provider最近probe原记录及完整原调用；逐条核原模型/渠道/密钥/epoch，实际evidence时间才给latency。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG真实供应方调用可能收费；当前设计不执行
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ReadInvocationRecord`、`model-gateway.ReadAttemptRecord`

## model-gateway.FeeProbeLimit

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/probe.go`
- 作用：读取独立探测日限，兼容缺失按用户明确默认0不限

```go
func FeeProbeLimit(policy gen.FeeBudgetPolicy) int
```

**输入**

无

**输出**

无

**步骤**

1. 字段缺失与明确0均为已批准不限；不借QA/background额度。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG真实供应方调用可能收费；当前设计不执行
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## model-gateway.CreateFallbackAttempt

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/fallback.go`
- 作用：费用安全保留备用与格式重试

```go
func CreateFallbackAttempt(ctx context.Context, op OperationContext, input CreateFallbackAttemptInput) (CreateFallbackAttemptResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateFallbackAttemptInput` | 封闭业务输入；逐字段见types $defs/CreateFallbackAttemptInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateFallbackAttemptResult` | 成功结果；逐字段见types $defs/CreateFallbackAttemptResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只用accepted固定有序候选；前驱必须当前attempt，直接后继才是fallback，同模型structured_retry最多一次，不重复付费尝试；总attempt上限固定路线长度+一次格式重试。
2. 先核管理员批准/固定嵌入space并准备真实新attempt，原子Reserve追加新attempt及预留，预算不足停止；旧未知attempt与held不释放，不更换root/总deadline。
3. admin-probe明确拒绝新增任何paid attempt。
4. 专用连接probe仅以record.Probe显式内部策略辨识；memory.assistant原admin-probe AI任务保留真实档位、原备用与预算；连接probe日限仅connection_probes真实invocation集合。

**错误**

- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：新增独立尝试/费用预留
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ComputeUsageCost`、`model-gateway.PrepareInvocationAttempt`、`model-gateway.ReadExecutionAttempt`、`model-gateway.ReadExecutionRecord`、`model-gateway.ReserveCost`

## model-gateway.MapAnthropicRequest

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/anthropic.go`
- 作用：映射Anthropic消息协议

```go
func MapAnthropicRequest(ctx context.Context, op OperationContext, input MapAnthropicRequestInput) (MapAnthropicRequestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `MapAnthropicRequestInput` | 封闭业务输入；逐字段见types $defs/MapAnthropicRequestInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MapAnthropicRequestResult` | 成功结果；逐字段见types $defs/MapAnthropicRequestResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. system文本及text parts合并独立system字段，空系统段合法
2. user/assistant原序，max_tokens固定，stream按可信执行请求；图像只用已核AssetRef字节base64 source
3. 密钥不进body，不接外部URL/本机路径，系统段不静默丢图像

**错误**

- `UNSUPPORTED_CAPABILITY` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ProviderContent`

## model-gateway.ImageKey

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/anthropic.go`
- 作用：ImageKey的真实本域材料处理

```go
func ImageKey(asset gen.AssetRef) string
```

**输入**

无

**输出**

无

**步骤**

1. 按可信调用者固定的原provider/version/epoch材料处理
2. 精确加密、引用、内容边界见data/workflows与源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.ProviderContent

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/anthropic.go`
- 作用：ProviderContent的真实本域材料处理

```go
func ProviderContent(message gen.GatewayMessage, images map[string][]byte, protocol string) ([]map[string]any, error)
```

**输入**

无

**输出**

无

**步骤**

1. text/image/file互斥，原生PDF仅application/pdf，受media原版本/hash读取。
2. OpenAI Chat: file/file_data PDF data URI; Anthropic: document/base64 application/pdf; Gemini: inlineData application/pdf.不调供应方Files API，不取外部URL。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ImageKey`、`model-gateway.MediaPartAsset`

## model-gateway.MapGeminiRequest

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/gemini.go`
- 作用：映射Gemini协议

```go
func MapGeminiRequest(ctx context.Context, op OperationContext, input MapGeminiRequestInput) (MapGeminiRequestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `MapGeminiRequestInput` | 封闭业务输入；逐字段见types $defs/MapGeminiRequestInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MapGeminiRequestResult` | 成功结果；逐字段见types $defs/MapGeminiRequestResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. assistant映射model，systemInstruction独立并保留全部system text parts
2. chat/vision/translation按generateContent或streamGenerateContent，图像只来自已核资源inlineData
3. embedding固定模型版本/维度，单项embedContent、多项batchEmbedContents，不把多段文本猜拼成单向量

**错误**

- `UNSUPPORTED_CAPABILITY` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.ProviderContent`

## model-gateway.ParseRerankResponse

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/rerank.go`
- 作用：解析rerank协议

```go
func ParseRerankResponse(ctx context.Context, op OperationContext, input ParseRerankResponseInput) (ParseRerankResponseResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ParseRerankResponseInput` | 封闭业务输入；逐字段见types $defs/ParseRerankResponseInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ParseRerankResponseResult` | 成功结果；逐字段见types $defs/ParseRerankResponseResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. results必须显式有index/relevance_score且index在输入范围，无重复，score有限
2. 按score稳定降序，仅返回原candidate IDs；缺失项按输入原序排末
3. 结果只返回orderedIds，不对未返回项伪造0或无穷分数，不生成正文

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## model-gateway.All

- 级别：参考
- 文件：`modules/model-gateway/migrations/migrations.go`
- 作用：按编号升序返回本域业务库全部内嵌迁移并校验编号连续

```go
func All() ([]Migration, error)
```

**输入**

无

**输出**

无

**步骤**

1. go:embed读取NNNN_name.sql（既有NNN_name.sql兼容）；文件名或编号不合法、编号不连续即报错

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：只读内嵌迁移内容；执行由本域存储装配在事务中完成
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：纯读取，可并发调用
- 幂等：确定性输出
- 依赖：无

## model-gateway.main

- 级别：约束
- 文件：`modules/model-gateway/cmd/model-gateway/main.go`
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
- 依赖：`model-gateway.BuildApplication`、`model-gateway.Serve`、`model-gateway.Shutdown`、`platform.RunService`、`model-gateway.NewRuntimeClient`

## model-gateway.BuildApplication

- 级别：约束
- 文件：`modules/model-gateway/internal/app/wire.go`
- 作用：装配本域存储、客户端、函数路由与worker

```go
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*transport.Application, error)
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
4. 本轮任务配置映射：恢复前置通过后OpenDatabase→InstallStore→defer Close；不调用需主密钥LoadRuntime或覆写未知runtime_meta代次；从既有IdentityClient装配平台NewIdentityAuthorizer，作为ServiceBoundaryConfig.Authorize及NewTaskAvailabilityHandlers参数；任务写调用固定portal，读取按根生成目录owner白名单加portal，免费可用性固定qa；完整其他handler缺失仍拒绝启动 档位写映射由NewProfileHandlers合并，putProfile仅portal调用且复用同一公共即时superadmin授权；不扩完整启动能力。 InstallStore接收config.MasterKeyRef可选可信引用；不在启动读取主密钥。合并NewCredentialHandlers的putCredential/rotateCredential，调用方固定portal。 合并NewCatalogHandlers既有putProvider/putModel，均仅portal且复用统一即时superadmin授权；enabled沿原配置字段，不增独立HTTP。 本轮合并NewPriceHandlers的quoteCost（根AI任务owner纯服务），getPriceBook仅portal，estimateTokens仅localization。 通过NewTokenHandlers绑定estimateTokens，不复制造假调用或影子入口。 本轮NewLedgerHandlers装配getLedger和queryUsage；getLedger仅portal，queryUsage对portal当前model.manage授权，根AI任务owner纯服务列入ServiceActorCallers并逐root核归属；不装配Reserve/Create。
5. 读取runtime epoch/maintenance；维护期间只开放健康/就绪与控制管理接口、不启动worker；本次启动完成换目录或换回时调用site-config:registerActivation登记回执；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
6. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源
7. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环
8. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`model-gateway.LoadRuntime`、`model-gateway.RecoverLeases`、`model-gateway.PublishOutbox`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`model-gateway.OpenDatabase`、`model-gateway.InstallStore`、`model-gateway.Store.Close`、`model-gateway.NewTaskAvailabilityHandlers`、`platform.NewServiceHandler`、`platform.NewIdentityAuthorizer`、`identity:checkAuthorization`、`model-gateway.NewProfileHandlers`、`model-gateway.NewCredentialHandlers`、`model-gateway.NewAdminHandlers`、`model-gateway.NewCatalogHandlers`、`model-gateway.NewPriceHandlers`、`model-gateway.EstimateTokens`、`model-gateway.NewTokenHandlers`、`model-gateway.NewLedgerHandlers`、`model-gateway.NewExecutionHandlers`、`platform.NewTaskGroup`、`platform.TaskGroup.Run`、`model-gateway.InstallExecutionOptions`、`model-gateway.NewTokenizerHandlers`、`platform.LoadServiceIdentity`、`model-gateway.ensureDataLayout`、`model-gateway.newRuntimeClient`、`model-gateway.validateConfig`、`model-gateway.NewCatalogRefreshHandlers`、`model-gateway.ReadInitializationState`、`model-gateway.RefreshModelCatalog`、`model-gateway.Application.ConfigureBusiness`、`model-gateway.Application.BackgroundTasks`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`、`site-config:readSiteConfig`、`model-gateway.ReadGatewaySiteID`、`model-gateway.ReadUsageStatisticsPage`、`model-gateway.Store.SetDataEpoch`、`model-gateway.GatewaySnapshotConfig`、`model-gateway.NewRouteHandlers`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## model-gateway.NewRuntimeClient

- 级别：参考
- 文件：`modules/model-gateway/internal/app/wire.go`
- 作用：NewRuntimeClient的实际本域生产职责

```go
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.newRuntimeClient`、`model-gateway.validateConfig`、`platform.LoadServiceIdentity`

## model-gateway.newRuntimeClient

- 级别：参考
- 文件：`modules/model-gateway/internal/app/wire.go`
- 作用：newRuntimeClient的实际本域生产职责

```go
func newRuntimeClient(config platform.ServiceConfig, key platform.SigningKey) (platform.SiteConfigClient, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.validateConfig

- 级别：参考
- 文件：`modules/model-gateway/internal/app/wire.go`
- 作用：validateConfig的实际本域生产职责

```go
func validateConfig(config platform.ServiceConfig) error
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ensureDataLayout

- 级别：参考
- 文件：`modules/model-gateway/internal/app/wire.go`
- 作用：ensureDataLayout的实际本域生产职责

```go
func ensureDataLayout(dataRoot string) error
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ReadInitializationState

- 级别：参考
- 文件：`modules/model-gateway/internal/app/wire.go`
- 作用：ReadInitializationState明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。

```go
func ReadInitializationState(ctx context.Context, client *gen.SiteConfigClient, validate func(json.RawMessage) error) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅以MG纯服务读site-config实际initialized，严格runtime schema验整个生产方响应；不从epoch推测，不获取私有coordination。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`site-config:readInitializationStatus`

## model-gateway.ReadGatewaySiteID

- 级别：参考
- 文件：`modules/model-gateway/internal/app/wire.go`
- 作用：安全统计生产支持：ReadGatewaySiteID

```go
func ReadGatewaySiteID(ctx context.Context, client *gen.SiteConfigClient, epoch string, validate func(json.RawMessage) error) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. MG纯服务清用户proof，固定2.5秒新操作读site-config:readSiteConfig，严格生产完整schema只取真实Config.SiteID，不用system或旧fee行猜站点。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`site-config:readSiteConfig`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.Serve

- 级别：约束
- 文件：`modules/model-gateway/internal/transport/server.go`
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

1. 单HTTP监听，ctx停止时仅Drain请求而不提前关DB；RunService取消并等完所有后台再Shutdown。

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：`model-gateway.Application.Drain`
- 其他依赖说明：本域OpenAPI路由

## model-gateway.Shutdown

- 级别：约束
- 文件：`modules/model-gateway/internal/transport/server.go`
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

1. 幂等Drain之后才由完整装配的closeResources等任务退出并关私有库，超时不伪安全或已关闭。

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：30秒期限仍有未安全在途步骤（可重试）
- `INTERNAL` / HTTP 500：flush/close失败（不可重试）

- 副作用：本域drain、资源关闭、durable恢复日志
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：sync.Once启动drain；其他调用等待同一次结果
- 幂等：重复Shutdown返回同一次关闭结果；已closed成功
- 依赖：`platform.TaskGroup.Drain`、`model-gateway.Application.Drain`
- 其他依赖说明：本域barrier安全点函数；本域journal/outbox flush

## model-gateway.Health

- 级别：约束
- 文件：`modules/model-gateway/internal/transport/server.go`
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

## model-gateway.Ready

- 级别：约束
- 文件：`modules/model-gateway/internal/transport/server.go`
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

1. 返回实际RuntimeView.Revision；initialized和safePoint必须有真实权威读取装配，不再硬编码0/false/true。
2. 完整初始化状态与保护权威接口等ROOT交付，本函数整体partial不伪就绪。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：无
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## model-gateway.Application.ConfigureBusiness

- 级别：参考
- 文件：`modules/model-gateway/internal/transport/server.go`
- 作用：本域HTTP/后台关闭装配ConfigureBusiness

```go
func (a *Application) ConfigureBusiness(handler http.Handler, tasks []platform.BackgroundTask, closeResources func(context.Context) error, safePoint func(context.Context) (bool, error), readInitialization func(context.Context) (bool, error))
```

**输入**

无

**输出**

无

**步骤**

1. 只装配本域生命周期，通用后台监督由RunService完成。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.Application.BackgroundTasks

- 级别：参考
- 文件：`modules/model-gateway/internal/transport/server.go`
- 作用：本域HTTP/后台关闭装配BackgroundTasks

```go
func (a *Application) BackgroundTasks() []platform.BackgroundTask
```

**输入**

无

**输出**

无

**步骤**

1. 只装配本域生命周期，通用后台监督由RunService完成。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.Application.Drain

- 级别：参考
- 文件：`modules/model-gateway/internal/transport/server.go`
- 作用：本域HTTP/后台关闭装配Drain

```go
func (a *Application) Drain(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 只装配本域生命周期，通用后台监督由RunService完成。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.InstallExecutionOptions

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：InstallExecutionOptions的本域实际执行职责

```go
func InstallExecutionOptions(options ExecutionOptions)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.CurrentStore`

## model-gateway.ExecutionDependencies

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ExecutionDependencies的本域实际执行职责

```go
func ExecutionDependencies() (ExecutionOptions, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.CurrentStore`

## model-gateway.BeginExecution

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：BeginExecution的本域实际执行职责

```go
func BeginExecution(ctx context.Context, op OperationContext, write bool) (*ExecutionTransaction, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.ExecutionDependencies`、`model-gateway.Store.BeginTx`、`model-gateway.beginExecution`、`platform.NewWriteContext`

## model-gateway.ExecutionTransaction.Close

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ExecutionTransaction.Close的本域实际执行职责

```ts
func(t *ExecutionTransaction) Close()
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.ExecutionTransaction.Commit

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ExecutionTransaction.Commit的本域实际执行职责

```ts
func(t *ExecutionTransaction) Commit() error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.ExecutionStorageError

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ExecutionStorageError的本域实际执行职责

```go
func ExecutionStorageError(err error) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.ReadInvocationRecord

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ReadInvocationRecord的本域实际执行职责

```go
func ReadInvocationRecord(tx *ExecutionTransaction, op OperationContext, id string) (InvocationRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 先核site/epoch及当前管理员或原caller owner，服务读取保持原root/costClass；原operation与snapshot/scalar归属必须一致。
2. 动态invocation hash与不可变execution hash独立核request/operation/route/profile/fee binding/created/owner/site/epoch，旧缺证据不猜填；结果结构只能从本域原持久输出读取。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.readInvocationRecord`

## model-gateway.SaveInvocationRecord

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：SaveInvocationRecord的本域实际执行职责

```go
func SaveInvocationRecord(tx *ExecutionTransaction, record InvocationRecord) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.ReadAttemptRecord

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ReadAttemptRecord的本域实际执行职责

```go
func ReadAttemptRecord(tx *ExecutionTransaction, id string) (AttemptRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ExecutionStorageError`、`model-gateway.ModelSnapshotHash`

## model-gateway.SaveAttemptRecord

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：SaveAttemptRecord的本域实际执行职责

```go
func SaveAttemptRecord(tx *ExecutionTransaction, record AttemptRecord) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.ReplayExecutionReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ReplayExecutionReceipt的本域实际执行职责

```go
func ReplayExecutionReceipt(tx *ExecutionTransaction, op OperationContext, function, key string, input, result any) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.SaveExecutionReceipt

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：SaveExecutionReceipt的本域实际执行职责

```go
func SaveExecutionReceipt(tx *ExecutionTransaction, op OperationContext, function, key string, input, result any) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.SaveInvocationEvent

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：SaveInvocationEvent的本域实际执行职责

```go
func SaveInvocationEvent(tx *ExecutionTransaction, record *InvocationRecord, event StreamEvent) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ExecutionStorageError`、`model-gateway.SaveInvocationRecord`

## model-gateway.EnqueueExecutionEvent

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：EnqueueExecutionEvent的本域实际执行职责

```go
func EnqueueExecutionEvent(tx *ExecutionTransaction, op OperationContext, eventType, subject string, revision int, payload any) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`platform.EnqueueEvent`

## model-gateway.BeginFinalization

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：BeginFinalization的实际本域生产职责

```go
func BeginFinalization(ctx context.Context, op OperationContext) (*ExecutionTransaction, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅已派发用量、费用、终态与释放；脱离HTTP取消建立5秒当前权威记账窗口，仍核site-config epoch和本地屏障，只有draining允许收尾。
2. 绝不用于新attempt、新预留、模型派发或延长持久原deadline；safe/failed屏障禁止所有active写。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.beginExecution`

## model-gateway.beginExecution

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：beginExecution的实际本域生产职责

```go
func beginExecution(ctx context.Context, op OperationContext, write, finalizing bool) (*ExecutionTransaction, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.ExecutionDependencies`、`model-gateway.ExecutionStorageError`

## model-gateway.ReleaseResolvedReferences

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ReleaseResolvedReferences的实际本域生产职责

```go
func ReleaseResolvedReferences(tx *ExecutionTransaction, record InvocationRecord) error
```

**输入**

无

**输出**

无

**步骤**

1. 只对已终态且全部attempt无held/charge_unknown的原调用释放route credential/header与attempt引用，不删除仍被引用密文。
2. 还要排除未更正/未void的price_unknown账单及未人工reconciled用量冲突，不按outer成功退休未解决费用的密钥。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.DestroyRetiredSecrets`、`model-gateway.ExecutionStorageError`

## model-gateway.ExecutionSnapshotHash

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：ExecutionSnapshotHash的实际本域生产职责

```go
func ExecutionSnapshotHash(request, operation, route, profile []byte, binding *FeeScopeBinding, createdNS int64, owner, site, epoch string) string
```

**输入**

无

**输出**

无

**步骤**

1. 规范JSON绑定不可变原request/operation/route/profile/fee scope、createdNS与owner/site/epoch；动态结果另有invocation_hash，恢复必须同时核对。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ModelSnapshotHash

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：MG唯一生产实际固定模型配置快照摘要

```go
func ModelSnapshotHash(model gen.Model) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. CanonicalJSON(原固定gen.Model) SHA256，包含配置版本、模型身份、能力、预处理及源绑定；不代表探测供应方内部权重版本。
2. 只在真实模型读取和准备attempt时生产，外域只比较opaque hash，旧缺证据不伪造。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ReadStatisticsInvocationRecord

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：为analytics读取安全用量证据

```go
func ReadStatisticsInvocationRecord(tx *ExecutionTransaction, op OperationContext, id string) (InvocationRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 明确只允许analytics纯服务，保留同站/当前epoch和不可变摘要核对。
2. 仅为QueryUsage安全投影使用，不扩大模型执行或其他AI owner权限。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：analytics纯服务，同站当前epoch。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.readInvocationRecord`

## model-gateway.readInvocationRecord

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/invocation.go`
- 作用：readInvocationRecord的原调用安全只读职责

```go
func readInvocationRecord(tx *ExecutionTransaction, op OperationContext, id string, statistics bool) (InvocationRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅原记录证据或安全费用投影，不创新付费权限或模型IO。
2. 原terminal_output_json与完整typed ModelOutput规范值必须等hash，拒未知字段/显式null，不把它们默默省略。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ExecutionStorageError`、`model-gateway.ExecutionSnapshotHash`、`model-gateway.IsRestoredEpoch`

## model-gateway.NewExecutionHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/http.go`
- 作用：NewExecutionHandlers的本域实际执行职责

```go
func NewExecutionHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.CancelInvocation`、`model-gateway.CreateInvocation`、`model-gateway.GetInvocation`、`model-gateway.LookupInvocation`、`model-gateway.ReconcileCharge`、`model-gateway.ResumeStream`、`model-gateway.invocationEventStream.Close`、`model-gateway.invocationEventStream.Next`、`model-gateway.writeExecutionResult`、`platform.ServeJSONEventStream`

## model-gateway.writeExecutionResult

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/http.go`
- 作用：writeExecutionResult的本域实际执行职责

```go
func writeExecutionResult(w http.ResponseWriter, op OperationContext, result any, err error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.invocationEventStream.Next

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/http.go`
- 作用：invocationEventStream.Next的本域实际执行职责

```ts
func(s *invocationEventStream) Next(ctx context.Context) (platform.SSEFrame, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.ResumeStream`

## model-gateway.invocationEventStream.Close

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/http.go`
- 作用：invocationEventStream.Close的本域实际执行职责

```ts
func(s *invocationEventStream) Close() error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名消费已校验业务输入、可信op及真实私有事务
2. 算法、证据与副作用边界见本域data/workflows及源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `GONE` / HTTP 410：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.SecretHeaderName

- 级别：参考
- 文件：`modules/model-gateway/internal/security/headers.go`
- 作用：SecretHeaderName的真实本域材料处理

```go
func SecretHeaderName(name string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按可信调用者固定的原provider/version/epoch材料处理
2. 精确加密、引用、内容边界见data/workflows与源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：无

## model-gateway.PrepareHeaderSecrets

- 级别：参考
- 文件：`modules/model-gateway/internal/security/headers.go`
- 作用：PrepareHeaderSecrets的真实本域材料处理

```go
func PrepareHeaderSecrets(ctx context.Context, op OperationContext, providerID string, providerVersion int, headers []SecretHeader, command any) ([]SealedHeader, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信调用者固定的原provider/version/epoch材料处理
2. 精确加密、引用、内容边界见data/workflows与源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.loadCredentialMasterKey`、`model-gateway.credentialDigest`、`model-gateway.SecretHeaderName`

## model-gateway.SaveHeaderSecrets

- 级别：参考
- 文件：`modules/model-gateway/internal/security/headers.go`
- 作用：SaveHeaderSecrets的真实本域材料处理

```go
func SaveHeaderSecrets(ctx context.Context, op OperationContext, tx *sql.Tx, providerID string, version int, headers []SealedHeader) error
```

**输入**

无

**输出**

无

**步骤**

1. 按可信调用者固定的原provider/version/epoch材料处理
2. 精确加密、引用、内容边界见data/workflows与源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.credentialStorageError`

## model-gateway.OpenAttemptCredential

- 级别：参考
- 文件：`modules/model-gateway/internal/security/headers.go`
- 作用：OpenAttemptCredential的真实本域材料处理

```go
func OpenAttemptCredential(ctx context.Context, op OperationContext, attemptID string) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信调用者固定的原provider/version/epoch材料处理
2. 精确加密、引用、内容边界见data/workflows与源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.CurrentStore`、`model-gateway.loadCredentialMasterKey`

## model-gateway.OpenAttemptHeaders

- 级别：参考
- 文件：`modules/model-gateway/internal/security/headers.go`
- 作用：OpenAttemptHeaders的真实本域材料处理

```go
func OpenAttemptHeaders(ctx context.Context, op OperationContext, attemptID string) (http.Header, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信调用者固定的原provider/version/epoch材料处理
2. 精确加密、引用、内容边界见data/workflows与源码注释

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `RATE_LIMITED` / HTTP 429：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核当前model.manage或root owner的真实只读链路内；不取得新权限。
- 并发：同一只读快照；签名/验签IO在SQL外，纯计算不轮询ctx。
- 幂等：只读真实状态与固定原价目，无费用写入。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.CurrentStore`、`model-gateway.loadCredentialMasterKey`、`model-gateway.SecretHeaderName`、`model-gateway.IsRestoredEpoch`

## model-gateway.TokenizeText

- 级别：约束
- 文件：`modules/model-gateway/internal/tokens/tokenize.go`
- 作用：校验本函数知识纯服务权限和4MiB字节预算；字符数由生成入口校验。

```go
func TokenizeText(ctx context.Context, op OperationContext, input TokenizeTextInput) (TokenizeTextResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `TokenizeTextInput` | 封闭分词输入 | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TokenizeTextResult` | 原文真实BPE偏移和固定空间身份 | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验本函数知识纯服务权限和4MiB字节预算；字符数由生成入口校验。
2. ReadTokenizerModel读取已启用同代嵌入模型，ModelEmbeddingSpace保持原五字段身份算法。
3. 仅支持不可变preprocessorVersion精确none;tiktoken-go@v0.7.0;cl100k_base或o200k_base；不进行隐式正规化、猜模型编码或fallback。
4. 使用已批准固定MIT依赖本地Encode，逐片验证覆盖原文并给出真实字节结束偏移；计算原文SHA256，无记录正文、供应方或费用IO。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅知识纯服务从已验证入口调用，不接受用户自报空间或编码。
- 并发：纯计算跟随调用者；只读模型与运行代次从同一SQL语句取得，无收费或业务写入。
- 幂等：相同模型不可变声明与相同输入返回相同BPE字节偏移。
- 依赖：`model-gateway.ReadTokenizerModel`、`model-gateway.ModelEmbeddingSpace`

## model-gateway.NewTokenizerHandlers

- 级别：参考
- 文件：`modules/model-gateway/internal/tokens/tokenize.go`
- 作用：绑定tokenizeText真实HTTP映射；可信边界严格校验，统一Problem、no-store，不记录正文。

```go
func NewTokenizerHandlers() map[string]http.Handler
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `handlers` | `TokenHandlers` | 真实文本预算HTTP映射 | 失败时结果不可用，不含秘密或正文 |

**步骤**

1. 绑定tokenizeText真实HTTP映射；可信边界严格校验，统一Problem、no-store，不记录正文。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅知识纯服务从已验证入口调用，不接受用户自报空间或编码。
- 并发：纯计算跟随调用者；只读模型与运行代次从同一SQL语句取得，无收费或业务写入。
- 幂等：相同模型不可变声明与相同输入返回相同BPE字节偏移。
- 依赖：`model-gateway.TokenizeText`

## model-gateway.ReadTokenizerModel

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/tokenizer.go`
- 作用：一次只读SQL取得已启用模型身份与实际runtime_meta代次，拒绝非同代或非嵌入模型。

```go
func ReadTokenizerModel(ctx context.Context, op OperationContext, modelID string) (Model, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `modelID` | `Identifier` | 固定模型ID | 入口已验证标识符 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `model` | `Model` | 同代模型不可变身份 | 非嵌入或停用拒绝 |
| `err` | `error` | 公共错误 | 失败结果不可用 |

**步骤**

1. 一次只读SQL取得已启用模型身份与实际runtime_meta代次，拒绝非同代或非嵌入模型。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅知识纯服务从已验证入口调用，不接受用户自报空间或编码。
- 并发：纯计算跟随调用者；只读模型与运行代次从同一SQL语句取得，无收费或业务写入。
- 幂等：相同模型不可变声明与相同输入返回相同BPE字节偏移。
- 依赖：`model-gateway.CurrentStore`

## model-gateway.InvocationSafeToRetry

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/recovery.go`
- 作用：InvocationSafeToRetry的实际本域生产职责

```go
func InvocationSafeToRetry(ctx context.Context, op OperationContext, record gatewayruntime.InvocationRecord) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只对完整原调用accepted/reserved且全部attempt确实未派发为true，未知/成功/对账不安全，仍仅继续原hash阶段。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ReadAttemptRecord`

## model-gateway.RecoverLeases

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/recovery.go`
- 作用：恢复崩溃中的调用

```go
func RecoverLeases(ctx context.Context, op OperationContext, input RecoverLeasesInput) (RecoverLeasesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RecoverLeasesInput` | 封闭业务输入；逐字段见types $defs/RecoverLeasesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverLeasesResult` | 成功结果；逐字段见types $defs/RecoverLeasesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅model-gateway纯服务后台，当前epoch有界读取失效lease，先校验不可变execution hash、原owner/root/费用和输入归属。
2. safeToRetry只接受accepted/reserved且全部attempt未派发，沿原输入hash/路线/deadline继续；旧未知证据不能true；过期未派发明确释放并失败，不延长截止。
3. 已派发只复用持久真实evidence/结果/usage，完整结果恢复交付及费用记账；缺用量保留held/unknown，未有可靠终态收敛charge_unknown，绝不重新请求供应方。
4. 取消/过期保守清理；证据损坏与IO未知失败关闭，不当缺证据过滤；已成功输出与人工费用对账分离。
5. 恢复历史投影只读及人工费用结算，不新派发旧调用或恢复旧准入scope。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：更新调用状态并对账outbox
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ErrorCode`、`model-gateway.ExecutionDependencies`、`model-gateway.ExecutionStorageError`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.FinishCancelledInvocation`、`model-gateway.FinishInvocation`、`model-gateway.InvocationSafeToRetry`、`model-gateway.MarkUnknownCharge`、`model-gateway.Nonempty`、`model-gateway.ReadAttemptEvidence`、`model-gateway.ReadExecutionAttempt`、`model-gateway.ReadExecutionRecord`、`model-gateway.RecordUsage`、`model-gateway.ReleaseUnacceptedAttempt`、`model-gateway.SettleInvocation`、`model-gateway.StartAcceptedInvocation`、`model-gateway.UsageDigest`

## model-gateway.InvocationResultState

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：映射持久状态到执行器回执

```go
func InvocationResultState(state string) string
```

**输入**

无

**输出**

无

**步骤**

1. 映射持久状态到执行器回执

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ExecuteInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：按固定路线执行已受理调用：预留、派发、流式、结算与备用

```go
func ExecuteInvocation(ctx context.Context, op OperationContext, input ExecuteInvocationInput) (ExecuteInvocationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ExecuteInvocationInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExecuteInvocationResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 从Create或恢复入口到达，领取30秒原调用执行租约并每10秒续租；保留存储原root/epoch/costClass/deadline，HTTP断开不抹掉任务；进程停止/管理员取消/原截止会取消模型IO。
2. 只用accepted固定路线；Prepare产生内存attempt，Reserve在一次事务内追加attempt/credential引用/reservation/fee_commitment；额度不足无孤立reserved尝试。完整保守输入及最大输出预留，不以delta字符数捏造实际token。
3. 先预留再派发，每尝试最多一次供应方HTTP请求；文本delta先持久再续读，embedding全空间/维度/有限值及rerank完整ID校验后才可输出结构结果。
4. 真实供应方usage含收费ID才RecordUsage/Settle；完整结果且缺usage/ID仍可succeeded+completed，attempt独立charge_unknown并保留held。结果不完整且费用未知outer charge_unknown。缓存等费率未覆盖不伪按普通价；缺币种不猜USD。
5. 未向调用方输出且未完整成功时依固定路线允许备用；每次新attempt新预留，旧未知费用继续held；格式重试原模型一次且不重置总deadline。已完整付费结果绝不自动重算。
6. 只有既有派发用量/终态/释放可以进入当前权威许可的5秒finalization记账窗口，不扩原模型deadline、不新派发；IO失败保守收敛原已派发事实，不猜未收费。
7. admin-probe单attempt失败/unknown立即按原链终态，不执行fallback或structured retry。
8. 专用连接probe仅以record.Probe显式内部策略辨识；memory.assistant原admin-probe AI任务保留真实档位、原备用与预算；连接probe日限仅connection_probes真实invocation集合。

**错误**

- `BUDGET_EXCEEDED`：额度不足（不可重试）
- `DEPENDENCY_UNAVAILABLE`：全部候选失败（可重试）

- 副作用：供应方请求；费用预留与结算；流事件与outbox
- 授权：permission：service；rule：仅由CreateInvocation在进程内启动
- 并发：每调用一个执行协程，持调用租约；崩溃后由RecoverLeases收敛
- 幂等：每attempt唯一；已派发attempt不重放
- 依赖：`model-gateway.ClaimInvocationLease`、`model-gateway.ClassifyFailure`、`model-gateway.ComputeUsageCost`、`model-gateway.CreateFallbackAttempt`、`model-gateway.DispatchInvocation`、`model-gateway.ErrorCode`、`model-gateway.FinishCancelledInvocation`、`model-gateway.FinishInterruptedInvocation`、`model-gateway.FinishInvocation`、`model-gateway.InvocationResultState`、`model-gateway.MarkUnknownCharge`、`model-gateway.Nonempty`、`model-gateway.PrepareInvocationAttempt`、`model-gateway.ReadExecutionAttempt`、`model-gateway.ReadExecutionRecord`、`model-gateway.RecordUsage`、`model-gateway.ReleaseUnacceptedAttempt`、`model-gateway.RenewInvocationLease`、`model-gateway.ReserveCost`、`model-gateway.SettleInvocation`、`model-gateway.UsageDigest`

## model-gateway.ReadExecutionRecord

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：读取并核原执行归属和完整不可变快照

```go
func ReadExecutionRecord(ctx context.Context, op OperationContext, id string) (gatewayruntime.InvocationRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取并核原执行归属和完整不可变快照

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ReadInvocationRecord`

## model-gateway.ReadExecutionAttempt

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：读取独立原attempt快照

```go
func ReadExecutionAttempt(ctx context.Context, op OperationContext, id string) (gatewayruntime.AttemptRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取独立原attempt快照

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ReadAttemptRecord`

## model-gateway.ClaimInvocationLease

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：领取原调用排他执行租约

```go
func ClaimInvocationLease(ctx context.Context, op OperationContext, id string) (gatewayruntime.InvocationRecord, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 领取原调用排他执行租约

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`

## model-gateway.RenewInvocationLease

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：保留原截止续租执行与provider slot

```go
func RenewInvocationLease(ctx context.Context, op OperationContext, id, leaseID string) error
```

**输入**

无

**输出**

无

**步骤**

1. 保留原截止续租执行与provider slot

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.ExecutionStorageError`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`

## model-gateway.PrepareInvocationAttempt

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：准备固定候选的真实内存尝试

```go
func PrepareInvocationAttempt(ctx context.Context, op OperationContext, record gatewayruntime.InvocationRecord, index int, reason string) (gatewayruntime.AttemptRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核语言建议权威批准及固定space；按原路线模型、价目和lease准备内存候选，vision以模型明示完整输入窗口作费用保守上界，不伪造实际token。
2. 序号与确定attemptId由原invocation/固定模型/原因生成，尚不写SQL；真正attempt/凭据引用与预留只由Reserve同事务产生。
3. 按真实固定模型快照生产modelSnapshotHash与attempt同预留原子持久。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.CheckLanguageSuggestionApproval`、`model-gateway.EstimateTokens`、`model-gateway.ModelEmbeddingSpace`、`model-gateway.ModelSnapshotHash`、`model-gateway.ValidateInputModel`

## model-gateway.ErrorCode

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：提取脱敏公共错误code

```go
func ErrorCode(err error) string
```

**输入**

无

**输出**

无

**步骤**

1. 提取脱敏公共错误code

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.Nonempty

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/execute.go`
- 作用：保留缺失收费ID为nil

```go
func Nonempty(value string) *string
```

**输入**

无

**输出**

无

**步骤**

1. 保留缺失收费ID为nil

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ReleaseUnacceptedAttempt

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/finish.go`
- 作用：凭明确未接收证据释放原held

```go
func ReleaseUnacceptedAttempt(ctx context.Context, op OperationContext, invocationID, attemptID string) error
```

**输入**

无

**输出**

无

**步骤**

1. 凭明确未接收证据释放原held

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginFinalization`、`model-gateway.ExecutionStorageError`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.ReadAttemptEvidence`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadReservation`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationRecord`

## model-gateway.FinishCancelledInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/finish.go`
- 作用：收敛取消后费用与输出终态

```go
func FinishCancelledInvocation(ctx context.Context, op OperationContext, record gatewayruntime.InvocationRecord) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 收敛取消后费用与输出终态

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginFinalization`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.FinishInvocation`、`model-gateway.MarkUnknownCharge`、`model-gateway.ReadExecutionAttempt`、`model-gateway.ReadReservation`、`model-gateway.ReleaseUnacceptedAttempt`

## model-gateway.FinishInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/finish.go`
- 作用：FinishInvocation的实际本域生产职责

```go
func FinishInvocation(ctx context.Context, op OperationContext, id, state, code string) error
```

**输入**

无

**输出**

无

**步骤**

1. 同事务核原状态及持久供应方完成evidence，固定终态/SSE序号/输出，取消请求优先；结果与费用独立，未知attempt不抹掉held。
2. 只有原调用终态且全部费用无held/unknown才释放历史credential/header引用；成功结果不可被对账或重复终态覆盖。
3. 完成事件必须有持久winning attempt且与证据一致，不从attempt.state猜成功模型。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginFinalization`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.ExecutionTransaction.Commit`、`model-gateway.Nonempty`、`model-gateway.ReadAttemptEvidence`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReleaseResolvedReferences`、`model-gateway.SaveAttemptRecord`、`model-gateway.SaveInvocationEvent`

## model-gateway.FinishInterruptedInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/finish.go`
- 作用：FinishInterruptedInvocation的实际本域生产职责

```go
func FinishInterruptedInvocation(ctx context.Context, op OperationContext, id, state, code string) error
```

**输入**

无

**输出**

无

**步骤**

1. 本地IO失败后读取已持久attempt事实，未派发才释放；已派发的held保守MarkUnknown并写原终态，绝不重新派发或猜费用零。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.BeginFinalization`、`model-gateway.ExecutionTransaction.Close`、`model-gateway.FinishInvocation`、`model-gateway.MarkUnknownCharge`、`model-gateway.ReadAttemptRecord`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadReservation`、`model-gateway.ReleaseUnacceptedAttempt`

## model-gateway.CheckLanguageSuggestionApproval

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/approval.go`
- 作用：CheckLanguageSuggestionApproval的实际本域生产职责

```go
func CheckLanguageSuggestionApproval(ctx context.Context, op OperationContext, input gen.CreateInvocationInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅localization.suggest+background需要非空languageSuggestionApprovalId，其他任务带该字段拒绝。
2. 以model-gateway纯服务调用固定localization readLanguageSuggestionApproval，严格runtime schema校验enabled=true及approvalId/siteId/dataEpoch全等；停用/换版/缺失/未知失败关闭，不造批准。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`localization:readLanguageSuggestionApproval`、`model-gateway.ExecutionDependencies`

## model-gateway.ProviderReportedCost

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/response.go`
- 作用：ProviderReportedCost的实际本域生产职责

```go
func ProviderReportedCost(raw json.Number) (*gen.Money, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅供应方明确USD金额，用big.Rat解析十进制、乘一百万向上取整，无float舍入、无币种猜测。
2. 数字表示最多128字符、指数绝对值最多128，在big.Rat解析前拒绝超大指数资源消耗，仍不使float64金额。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.MergeProviderUsage

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/response.go`
- 作用：MergeProviderUsage的实际本域生产职责

```go
func MergeProviderUsage(outcome *ProviderOutcome, usage *providerUsage, nonText bool, currency string) error
```

**输入**

无

**输出**

无

**步骤**

1. 合并供应方真实明确字段，Anthropic输入包含实际cache创建/读取，其他cache特殊计价设pricingComplete=false；所有计数非负有界。
2. embedding/rerank无生成输出时输出0属协议语义，缺真实输入/ID仍未知；不以文本估算补实际用量。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ProviderReportedCost`

## model-gateway.ConsumeProviderFrame

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/response.go`
- 作用：ConsumeProviderFrame的实际本域生产职责

```go
func ConsumeProviderFrame(raw []byte, adapter, capability, expectedModel, currency string, outcome *ProviderOutcome, emit func(string) error) error
```

**输入**

无

**输出**

无

**步骤**

1. 按固定adapter消费真实JSON/SSE字段，有限唯一向量索引与模型版本、单候选、明确终态，所有文本经有界UTF8 emit。
2. 4xx且协议明确拒绝才not_accepted，其余错误保留unknown；原始正文不进日志/事件；token/金额只读真实字段。
3. 实际嵌入返回模型与固定版本不一致为STALE_RESULT终止，不当普通格式错误自动重付费。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.MergeProviderUsage`

## model-gateway.SendProviderRequest

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/response.go`
- 作用：SendProviderRequest的实际本域生产职责

```go
func SendProviderRequest(ctx context.Context, request BuildProviderRequestResult, provider gen.Provider, capability string, candidateIDs []string, emit func(string) error) (ProviderOutcome, error)
```

**输入**

无

**输出**

无

**步骤**

1. 执行一次固定原attempt HTTP，不重定向/重试；httptrace仅在尚未写headers时证明未发出，错误后费用保守unknown。
2. 响应JSON32MiB/SSE64MiB，空/断流/超时无可靠终态失败；真实idle timer由收到字节续期，不延长原业务截止；文本delta完整UTF8及64KiB上限。
3. 将同一批原UTF8 delta在既有总64KiB内真实累加；只有HTTP2xx、完整协议结束帧/完整终态、Code空才给原chat/vision/translation能力的ModelOutput.Text指针，显式空文本与没有结果区分。
4. 累计文本随既有attempt_evidence/resultAttemptId/record.Output同事务持久；不新增收费或改旧证据，SSE缺结束帧即使见过finish_reason也不当完整流。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ConsumeProviderFrame`、`model-gateway.ParseRerankResponse`、`model-gateway.providerIdleReader.Read`

## model-gateway.providerIdleReader.Read

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/response.go`
- 作用：按真实收到字节续期idle timeout

```ts
func(r *providerIdleReader) Read(data []byte) (int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按真实收到字节续期idle timeout

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.ServiceProviderContext

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/images.go`
- 作用：ServiceProviderContext的实际本域生产职责

```go
func ServiceProviderContext(ctx context.Context, op OperationContext) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按真实类型消费本域已固定输入与原执行证据；具体数据规则见本域执行闭环data/workflows。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.AuthorizeImages

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/images.go`
- 作用：AuthorizeImages的实际本域生产职责

```go
func AuthorizeImages(ctx context.Context, op OperationContext, messages []gen.GatewayMessage) (map[string]int, error)
```

**输入**

无

**输出**

无

**步骤**

1. media原site授权元数据必须ready，ref的asset/hash/type/size全等，固定当时version；未固定资源不接受为可付费输入。
2. 受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。MIME/hash/size和%PDF-头一致，不把DOCX伪装PDF/image。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`media:readAsset`、`model-gateway.ExecutionDependencies`、`model-gateway.ImageKey`、`model-gateway.MediaPartAsset`、`model-gateway.ServiceProviderContext`

## model-gateway.LoadImages

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/images.go`
- 作用：LoadImages的实际本域生产职责

```go
func LoadImages(ctx context.Context, op OperationContext, messages []gen.GatewayMessage, versions map[string]int) (map[string][]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅原accepted授权version经media readAssetBytes取字节，累计24MiB内联预算；字节size/MIME/hash全等，不任意URL/重定向。
2. 受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。MIME/hash/size和%PDF-头一致，不把DOCX伪装PDF/image。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`media:readAssetBytes`、`model-gateway.ExecutionDependencies`、`model-gateway.ImageKey`、`model-gateway.MediaPartAsset`、`model-gateway.ReadImageBytes`、`model-gateway.ServiceProviderContext`

## model-gateway.ReadImageBytes

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/images.go`
- 作用：ReadImageBytes的实际本域生产职责

```go
func ReadImageBytes(response *http.Response, asset gen.AssetRef) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界读取完整原asset字节核MIME/size/hash并始终Close响应；不能把截断字节交模型。
2. 受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。MIME/hash/size和%PDF-头一致，不把DOCX伪装PDF/image。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.MediaPartAsset

- 级别：参考
- 文件：`modules/model-gateway/internal/provider/images.go`
- 作用：MediaPartAsset受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。

```go
func MediaPartAsset(part gen.GatewayPart) (gen.AssetRef, error)
```

**输入**

无

**输出**

无

**步骤**

1. 受控原生PDF输入，付费前核原能力与完整窗口，不猜实际token。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.DestroyRetiredSecrets

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/retirement.go`
- 作用：根据完整引用证据销毁已退休密文并保留版本审计

```go
func DestroyRetiredSecrets(ctx context.Context, op OperationContext, tx *sql.Tx) error
```

**输入**

无

**输出**

无

**步骤**

1. 完整应用显式开启tracking后才允许；任何legacy缺execution证据均保留，不从空invocations推断可销毁。
2. 非active主凭据及全部route/attempt引用已释放时，同事务审计版本/销毁时刻再删材料，版本不复用。
3. header无active映射及未释放引用才清空密文保留FK墓碑；outer成功但unknown/held不释放。
4. 凭据版本在本库全source epoch的真实未释放引用都阻止销毁；不能因config迁到target而漏算旧未知费用引用。
5. 已认证历史source header也可在全scope原引用都释放且无任何当前provider引用后退休；原AAD/版本不变。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：已证明退休材料销毁及不可复用版本审计，未知/在途材料保留
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.RelayOutbox

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/relay.go`
- 作用：复用平台outbox真实批次转发

```go
func RelayOutbox(ctx context.Context, op OperationContext, limit int) (platform.RelayBatchResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. model-gateway纯服务继承site/epoch，复用平台私有store/publisher/relay，不复制通用机制；jobs或权威窗口未知保留pending。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：平台outbox领取/确认/退避及jobs事件发布
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.CurrentStore`、`model-gateway.ExecutionDependencies`、`platform.NewEventPublisher`、`platform.RunRelayBatch`、`jobs:publishEvent`

## model-gateway.RequireInvocationLookupTracking

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/lookup.go`
- 作用：确认原调用查找的完整追踪来源

```go
func RequireInvocationLookupTracking(tx *gatewayruntime.ExecutionTransaction, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 只读当前epoch的真实fresh_activation或verified_restore证明。
2. 无来源或遗留调用缺创建键/不可变摘要/归属时失败关闭，不从空库自建证明。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅原任务owner已授权的执行或只读查找调用。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：`model-gateway.ExecutionStorageError`

## model-gateway.LookupInvocation

- 级别：约束
- 文件：`modules/model-gateway/internal/invocation/lookup.go`
- 作用：按原键及完整语义输入查找原调用

```go
func LookupInvocation(ctx context.Context, op OperationContext, input LookupInvocationInput) (LookupInvocationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `LookupInvocationInput` | 封闭业务输入；逐字段见types $defs/CreateInvocationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LookupInvocationResult` | 当前原调用快照，见types $defs/LookupInvocationResult | 只读原记录与完整输出证据，不新建或提交业务状态。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 严格生产方schema消费原完整CreateInvocationInput和key，按原规范请求摘要比原创建回执，异输入CONFLICT。
2. 只读实际当前Invocation/完整真实output，对原持久请求再核键/语义hash，不改原paid deadline，不重验过期准入来付款。
3. 回执存在但原记录缺失，或创建键有原行但无回执，均Integrity不伪404。
4. 只有当前epoch完整tracking真实来源，且无未索引/未知归属/待恢复投影旧行，原key/root/site/body确未接受才INVOCATION_NOT_ACCEPTED（HTTP404）；普通NOT_FOUND/旧部署路由404绝非负证明，marker缺失503，纯读不建证明。
5. 保留既有memory.assistant+owner memory的已明确批准admin-probe AI任务；仍用真实profile，不当私有portal连接probe。其他公开创建仍仅原QA/background owner，不扩大匿名或scope。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INVOCATION_NOT_ACCEPTED` / HTTP 404：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅AI目录任务owner纯服务，保原root/site/epoch/costClass；不用旧permit给新付费授权。
- 并发：一个只读持久事务快照；保留当前读取ctx截止，不改原付费截止或租约。
- 幂等：只读原CreateInvocation持久回执与原创建键索引；原键异输入CONFLICT，不写新回执、时限或付费事实。
- 依赖：`model-gateway.BeginExecution`、`model-gateway.CanonicalInvocationInput`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.ReadInvocationRecord`、`model-gateway.ReadInvocationSnapshot`、`model-gateway.RequireInvocationLookupTracking`、`model-gateway.ExecutionTransaction.Close`

## model-gateway.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/model-gateway/internal/snapshot_archive.go`
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
- 依赖：`platform.SnapshotManager.OpenArchive`

## model-gateway.ReadUsageStatisticsPage

- 级别：约束
- 文件：`modules/model-gateway/internal/admin/statistics.go`
- 作用：安全统计生产支持：ReadUsageStatisticsPage

```go
func ReadUsageStatisticsPage(ctx context.Context, op OperationContext, input ReadUsageStatisticsPageInput) (ReadUsageStatisticsPageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadUsageStatisticsPageInput` | 固定历史上界与游标，逐字段见types $defs/ReadUsageStatisticsPageInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadUsageStatisticsPageResult` | 安全posted账本变更页，逐字段见types $defs/ReadUsageStatisticsPageResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅analytics纯服务，Actor.SiteID必须等受信启动site-config完整严格响应装配的ExecutionOptions.SiteID，当前runtime_meta epoch严格匹配。
2. 初次固定实际append-only changeRevision上界，cursor平台签名绑定service/site/epoch/after/limit及固定upper，独立24h到期；每页≤100/2.5秒，后续新写不改变旧页。
3. 按真实变化顺序返entry独立ledgerRevision、原owner/site/sourceEpoch/root/op/class/冻结provider/model/真实amountKnown+amount/state/occurredAt；缺任何原事实严格失败，不猜event.revision或当前配置。
4. 升级现存真实行第一次观察baseline version1，与原收费历史版本不同；insert/update/delete、supersede旧项、void全部写同原事务历史，账本金额原行不变。
5. 旧sourceEpoch跨target恢复若尚缺已认证只读投影明确503，不重基原execution/fee/hash；全域保护未齐不计完整服务。
6. V2共用真实usage_statistics_timeline全局上界，ledger及invocation/attempt/reservation原事务append-only完整捕获，单页两数组合计100/2.5s。基线仅真实第一次观察，不造历史event revision。
7. 响应posted-and-inflight且completeFeeCoverage=true仅表示真实来源/无账项未知负债完整覆盖；amountUnknown或charge_unknown/held仍明确未知，消费者不将已知部分和冒全站总额。
8. 原调用/attempt财务权威拒删除，held reservation拒删除；关闭reservation删改保真实旧事实和deleted标记。无ledger invoc/attempt/held不得消失成0，原来源缺失严格失败。
9. 需完整fresh/verified-restore tracking，当前runtime/site授权；两数组按实际changeRevision合并顺序，消费分页只更新隔离统计版本，整个固定上界完成才激活，不公开半页统计。
10. 完整页≤2MiB UTF8响应；pageHash为省略pageHash字段后的其余完整响应canonical JSON SHA256，含cursor/上界/全部安全原事实，不截断source或伪造摘要。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.ExecutionDependencies`、`model-gateway.ledgerRuntime`、`model-gateway.beginAdminSnapshot`、`model-gateway.readStatisticsLineage`、`model-gateway.validateUsageChange`、`platform.SignCursor`、`platform.VerifyCursor`、`model-gateway.ReadUsageObservation`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.readStatisticsLineage

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/statistics.go`
- 作用：安全统计生产支持：readStatisticsLineage

```go
func readStatisticsLineage(ctx context.Context, op OperationContext, tx *sql.Tx, stored storedUsageChange) (statisticsLineage, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核原调用完整execution hash及原attempt实际快照，与原不可变路线candidate/provider/model/credential/price全值一致；不访问当前配置补历史。
2. ModelSnapshotHash只从原冻结Model计算，旧缺stored hash仍需原冻结route完整证明；来源跨target未有认证投影时失败，不过滤或改写。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.ReadStatisticsInvocationRecord`、`model-gateway.ReadAttemptRecord`、`model-gateway.ModelSnapshotHash`、`model-gateway.IsRestoredEpoch`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.validateUsageChange

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/statistics.go`
- 作用：安全统计生产支持：validateUsageChange

```go
func validateUsageChange(stored storedUsageChange, lineage statisticsLineage, revision, version int64, observed string) (UsageLedgerChange, error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实变更版本与原冻结安全字段逐项相符，金额known与实际nullable字段一致；USD安全整数/真实原账本时间，未知省略不当0。
2. superseded/voided/deleted保留原身份与金额观察，不伪收费账单；无正文/输出/收费请求ID/凭据。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.ReadSnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/catalog/bundled/catalog.go`
- 作用：返回真实固定数据副本和已核来源内容版本

```go
func ReadSnapshot() ([]byte, Source, error)
```

**输入**

无

**输出**

无

**步骤**

1. 校验固定5806411字节与实际SHA256/sourceVersion/retrievedAt/MIT来源元数据；数据版本取实际内容hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：只在已核原owner/site/epoch/root/costClass的内部执行链或可信启动装配，不取得新权限。
- 并发：按调用方持久事务/原执行租约处理；纯计算无ctx轮询，SQL及网络贯穿ctx。
- 幂等：沿原调用/attempt唯一事实重复读取或同事务原命令回执，不自动重复模型付费。
- 依赖：无

## model-gateway.CreateProbeInvocation

- 级别：参考
- 文件：`modules/model-gateway/internal/invocation/probe.go`
- 作用：CreateProbeInvocation实际探测执行支持

```go
func CreateProbeInvocation(ctx context.Context, op OperationContext, input gen.CreateConnectionProbeInput, model gen.Model) (gen.CreateConnectionProbeResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实固定私有probe payload/执行policy，不进AI任务目录；同原接受链冻结并执行一次。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG真实供应方调用可能收费；当前设计不执行
- 授权：权限 superadmin；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`model-gateway.AcceptPreparedInvocation`、`model-gateway.ReadDispatchProvider`、`model-gateway.ReadPriceSnapshot`、`model-gateway.ReplayExecutionReceipt`、`model-gateway.StartAcceptedInvocation`、`model-gateway.RequireInvocationLookupTracking`

## model-gateway.IsRestoredEpoch

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/restore_projection.go`
- 作用：IsRestoredEpoch本域真实来源和隔离数据回调

```go
func IsRestoredEpoch(ctx context.Context, tx *sql.Tx, source string, target string) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.IsRedactedMaterial

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/restore_projection.go`
- 作用：IsRedactedMaterial本域真实来源和隔离数据回调

```go
func IsRedactedMaterial(ctx context.Context, tx *sql.Tx, kind string, key string) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.CheckProviderMaterial

- 级别：参考
- 文件：`modules/model-gateway/internal/runtime/restore_projection.go`
- 作用：CheckProviderMaterial本域真实来源和隔离数据回调

```go
func CheckProviderMaterial(ctx context.Context, tx *sql.Tx, op OperationContext, provider gen.Provider) error
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.IsRestoredEpoch`、`model-gateway.IsRedactedMaterial`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.GatewaySnapshotConfig

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：GatewaySnapshotConfig本域真实来源和隔离数据回调

```go
func GatewaySnapshotConfig(store *gatewayruntime.Store, root string) lifecycle.SnapshotConfig
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.GatewayWatermark`、`model-gateway.CaptureGatewaySnapshot`、`model-gateway.MigrateGatewayStage`、`model-gateway.ValidateGatewayStage`、`model-gateway.GatewayStageBudget`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.GatewayWatermark

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：GatewayWatermark本域真实来源和隔离数据回调

```go
func GatewayWatermark(ctx context.Context, db *sql.DB) (int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.CaptureGatewaySnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：CaptureGatewaySnapshot本域真实来源和隔离数据回调

```go
func CaptureGatewaySnapshot(ctx context.Context, store *gatewayruntime.Store, root string, cut lifecycle.SnapshotSafeCut, workspace string, input lifecycle.SnapshotExportInput) (lifecycle.DomainSnapshotManifest, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.GatewaySnapshotInventory`、`model-gateway.OpenGatewaySnapshotDB`、`model-gateway.GatewaySnapshotFile`、`model-gateway.RedactGatewaySnapshot`、`model-gateway.GatewayWatermark`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.GatewaySnapshotInventory

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：GatewaySnapshotInventory本域真实来源和隔离数据回调

```go
func GatewaySnapshotInventory(ctx context.Context, db *sql.DB, epoch string) ([]platform.AssetVersionRef, []lifecycle.SnapshotInvocationRef, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.ReadInvocationRecord`、`model-gateway.IsRestoredEpoch`、`model-gateway.ImageKey`、`model-gateway.MediaPartAsset`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.OpenGatewaySnapshotDB

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：OpenGatewaySnapshotDB本域真实来源和隔离数据回调

```go
func OpenGatewaySnapshotDB(ctx context.Context, path string, readonly bool) (*sql.DB, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.GatewaySnapshotFile

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：GatewaySnapshotFile本域真实来源和隔离数据回调

```go
func GatewaySnapshotFile(ctx context.Context, path string) (lifecycle.DomainSnapshotFile, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.RedactGatewaySnapshot

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：RedactGatewaySnapshot本域真实来源和隔离数据回调

```go
func RedactGatewaySnapshot(ctx context.Context, db *sql.DB, path string) error
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。
2. 原header destroyed_at明确销毁审计允许保持空material；必须有合法原时间，不当新红action或普通空值。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.GatewaySnapshotFile`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.GatewayStageBudget

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：GatewayStageBudget本域真实来源和隔离数据回调

```go
func GatewayStageBudget(ctx context.Context, manifest lifecycle.DomainSnapshotManifest) (int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.MigrateGatewayStage

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：MigrateGatewayStage本域真实来源和隔离数据回调

```go
func MigrateGatewayStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, target string, root string) error
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。
2. 复用公共PrepareRestoredOutbox，仅认证Stage私有上下文内隔离原outbox活动资格、保原行与原epoch，并把gate绑本次host seal，不自行删表或重投。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.GatewayStageBudget`、`model-gateway.OpenGatewaySnapshotDB`、`model-gateway.ValidateGatewayDatabase`、`model-gateway.GatewaySnapshotInventory`、`model-gateway.IsRestoredEpoch`、`platform.PrepareRestoredOutbox`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.ValidateGatewayStage

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：ValidateGatewayStage本域真实来源和隔离数据回调

```go
func ValidateGatewayStage(ctx context.Context, manifest lifecycle.DomainSnapshotManifest, target string, root string) error
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.GatewayStageBudget`、`model-gateway.OpenGatewaySnapshotDB`、`model-gateway.ValidateGatewayDatabase`、`model-gateway.GatewaySnapshotInventory`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.ValidateGatewayDatabase

- 级别：参考
- 文件：`modules/model-gateway/internal/protection/snapshot.go`
- 作用：ValidateGatewayDatabase本域真实来源和隔离数据回调

```go
func ValidateGatewayDatabase(ctx context.Context, db *sql.DB, epoch string, schema int) error
```

**输入**

无

**输出**

无

**步骤**

1. 按本域data/workflows的O_EXCL来源/认证source→target/完整typed SQL库存规则实现；不造原调用/付费/资源引用，不重基原执行hash。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：无
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.ReadUsageObservation

- 级别：参考
- 文件：`modules/model-gateway/internal/admin/observations.go`
- 作用：真实无账项调用/attempt/held安全观察，只读原完整冻结来源

```go
func ReadUsageObservation(ctx context.Context, op OperationContext, tx *sql.Tx, kind string, entity string, raw []byte, revision int64, version int64, deleted bool, observed string) (UsageObservation, error)
```

**输入**

无

**输出**

无

**步骤**

1. 闭合类型及原owner/site/root/sourceepoch/完整执行hash严格比对；金额未知不补0，原冻结模型/价目只读，不送prompt/output/secret。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 429：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅analytics纯服务且当前真实site/epoch；免费只读，不扩Create/预算权限。
- 并发：单页2.5秒SQL快照/固定不可变上界；最多100，60秒任务保存nextCursor续接
- 幂等：固定watermark/cursor重放原历史页，新写不失效原上界
- 依赖：`model-gateway.ReadStatisticsInvocationRecord`、`model-gateway.ReadAttemptRecord`、`model-gateway.ModelSnapshotHash`
- 其他依赖说明：本域append-only统计历史/原冻结调用和尝试

## model-gateway.ModelsPage

- 级别：约束
- 文件：`modules/model-gateway/web/src/index.tsx`
- 作用：模型接入：供应方与密钥、模型、档位、任务分配，一键生成档位与分配

```ts
export default function ModelsPage(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 读取管理目录，分标签页配置
2. 密钥只在提交时传输一次

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无

## model-gateway.ReadProviderCatalog

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/discover.go`
- 作用：读取可直接接入的供应方目录

```go
func ReadProviderCatalog(ctx context.Context, op OperationContext, input ReadProviderCatalogInput) (ReadProviderCatalogResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播 | 截止到达即停止外呼 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造 |
| `input` | `ReadProviderCatalogInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadProviderCatalogResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含密钥或供应方响应正文 |

**步骤**

1. 解析内置 models.dev 快照，进程内只解析一次。
2. 只保留本网关协议能调用的供应方：OpenAI 兼容（含 OpenAI、OpenRouter）、Anthropic、Gemini；没有可用接口地址的跳过。
3. 热门供应方按固定顺序排前，其余按名称排序。

**错误**

- `INTEGRITY_ERROR`：内置目录损坏（不可重试）

- 副作用：无
- 授权：permission：model.manage；rule：后台管理员
- 并发：只读内置数据
- 幂等：只读
- 依赖：无
- 其他依赖说明：内置 models.dev 快照

## model-gateway.DiscoverProviderModels

- 级别：约束
- 文件：`modules/model-gateway/internal/catalog/discover.go`
- 作用：拉取供应方当前可用的模型

```go
func DiscoverProviderModels(ctx context.Context, op OperationContext, input DiscoverProviderModelsInput) (DiscoverProviderModelsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播 | 截止到达即停止外呼 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造 |
| `input` | `DiscoverProviderModelsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DiscoverProviderModelsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含密钥或供应方响应正文 |

**步骤**

1. 读取供应方协议、地址和当前有效密钥；没有密钥时只返回目录里的模型。
2. 按协议请求供应方的模型列表，15 秒超时，响应上限 8MiB；失败只记原因，不带响应正文。
3. 按接口地址匹配目录供应方，用目录补全名称、上下文、输出上限和输入模态，推断对话/视觉/向量/重排/推理能力。
4. 去掉明显不是文本模型的条目（语音、实时、图像或视频生成），按名称排序。

**错误**

- `NOT_FOUND`：供应方不存在（不可重试）
- `VERSION_MISMATCH`：数据代次已变化（不可重试）
- `INTEGRITY_ERROR`：密钥材料或内置目录损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储不可用（可重试）

- 副作用：向供应方发一次只读的模型列表请求
- 授权：permission：superadmin；rule：使用已存密钥，仅超级管理员
- 并发：短读事务取出密钥后结束，外呼不持有事务
- 幂等：只读
- 依赖：无
- 其他依赖说明：本域供应方与密钥表；内置 models.dev 快照
