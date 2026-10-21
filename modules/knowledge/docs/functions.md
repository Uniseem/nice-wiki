# knowledge 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 210 个函数：约束 35 个，参考 175 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## knowledge.LoadRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/runtime/bootstrap.go`
- 作用：启动索引服务

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

1. 仅本域服务启动，私有DataDirectory必须匹配可信Runtime.DataRoot及同站同代index_identity
2. 读取SQLite真实quick_check及所有活动指针，在同一只读快照逐项核build/doc/代数/完整空间
3. 重算每代chunks/terms/vectors清单及manifestHash，任何错误返回失败并关闭就绪，不改active、不自动重建/调用模型
4. 完整迁移/恢复目录和后台租约恢复由BuildApplication与已登记backgroundTasks装配；本函数不伪造ready或活动代数

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：打开本服务索引文件不读其他服务存储
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.ReadGeneration`

## knowledge.PutIndexConfig

- 级别：约束
- 文件：`modules/knowledge/internal/config/config.go`
- 作用：更新索引配置

```go
func PutIndexConfig(ctx context.Context, op OperationContext, input PutIndexConfigInput) (PutIndexConfigResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutIndexConfigInput` | 封闭业务输入；逐字段见types $defs/PutIndexConfigInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutIndexConfigResult` | 成功结果；逐字段见types $defs/PutIndexConfigResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 即时入口核knowledge.manage，本域核可信portal管理主体；config.version必须等于expectedVersion，overlap小于chunkTokens，语言仅可信站点基准语言
2. 已保存原回执先返回，不因之后模型停用改变原操作结果；同键异输入CONFLICT
3. 开启时以knowledge本站纯服务读MG无secret profiles/models，严格生产方schema；profile主模型必须embedding且enabled，输入上限容纳chunkTokens，MG权威embeddingSpace全部五字段精确匹配
4. 可选重排profile必须配置有效rerank主模型；禁止本域复制或重算spaceId算法。明确关闭配置不依赖模型可达，不触发付费
5. 取得平台写窗口与本域身份事务后再次核幂等回执；首次expectedVersion=0创建version1，后续CAS且configId稳定
6. 配置及安全原回执同事务；空间变化只影响后续构建，不修改active、不自动花钱重建

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：配置变更outbox；既有索引保留
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.RuntimeFromContext`、`knowledge.validateConfiguredSpace`、`knowledge.BeginIndexWrite`、`knowledge.ReadOperationReceipt`、`knowledge.SaveOperationReceipt`

## knowledge.ReadIndexConfig

- 级别：约束
- 文件：`modules/knowledge/internal/config/config.go`
- 作用：读取索引配置

```go
func ReadIndexConfig(ctx context.Context, op OperationContext, input ReadIndexConfigInput) (ReadIndexConfigResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ReadIndexConfigInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadIndexConfigResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 仅已认证knowledge.manage管理员读取当前可信站点/代次私有配置
2. 无配置返回NOT_FOUND，不捏造模型/空间；返回真实当前version用于后续CAS

**错误**

- `FORBIDDEN`：权限不足（不可重试）

- 副作用：无
- 授权：permission：knowledge.manage；rule：后台管理员
- 并发：只读快照
- 幂等：只读
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.RuntimeFromContext`、`knowledge.ReadConfig`

## knowledge.validateConfiguredSpace

- 级别：参考
- 文件：`modules/knowledge/internal/config/config.go`
- 作用：用MG权威profile主模型与完整空间声明校验启用配置，不重算spaceId

```go
func validateConfiguredSpace(ctx context.Context, op OperationContext, runtime index.Runtime, config gen.IndexConfig) error
```

**输入**

无

**输出**

无

**步骤**

1. 用MG权威profile主模型与完整空间声明校验启用配置，不重算spaceId
2. 可选rerankProfileId只能选择QA费用类别的真实原生rerank档位；背景入库embedding与QA检索重排费用类别分离

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域装配或已授权用例内部调用，继承外层权限/事务
- 并发：复用本域短事务；外部权威读取不持有SQL事务
- 幂等：同原记录及规范输入约束
- 依赖：`knowledge.ReadModelCatalog`、`knowledge.EmbeddingProfile`

## knowledge.HandlePublication

- 级别：约束
- 文件：`modules/knowledge/internal/events/publication.go`
- 作用：接受权威发布事件

```go
func HandlePublication(ctx context.Context, op OperationContext, input HandlePublicationInput) (HandlePublicationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `HandlePublicationInput` | 封闭业务输入；逐字段见types $defs/HandlePublicationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandlePublicationResult` | 成功结果；逐字段见types $defs/HandlePublicationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 消费公共订阅边界已校验的原信封，并核eventId/document/site/epoch/visibility与payload及信封逐字段一致
2. 当前平台写窗口事务内最先ConsumeInbox，重复无操作；visibility水位不前进则只提交inbox
3. 保存本域可见水位；无配置或明确停用时skipped，不捏造嵌入空间或新任务
4. 已启用时QueueBuild固定配置/空间及后台批准，与原inbox/水位/派发意图同事务，不反向同步回源
5. 提交后尝试原DispatchBuild，jobs未知保留意图由恢复继续；真实当前来源在FetchPublishedSource及每次模型前再次证明

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写inbox/build/job事件，不处理草稿
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.BeginIndexWrite`、`knowledge.ConsumeInbox`、`knowledge.ReadConfig`、`knowledge.QueueBuild`、`knowledge.DispatchBuild`

## knowledge.CreateBuild

- 级别：约束
- 文件：`modules/knowledge/internal/build/create.go`
- 作用：人工或事件申请构建

```go
func CreateBuild(ctx context.Context, op OperationContext, input CreateBuildInput) (CreateBuildResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateBuildInput` | 封闭业务输入；逐字段见types $defs/CreateBuildInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateBuildResult` | 成功结果；逐字段见types $defs/CreateBuildResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 核已认证knowledge.manage管理员；同幂等键原回执优先返回，同键异输入CONFLICT
2. 先锁外content验证完整发布引用visible/current，创建提交前证明不超过一秒且不低于本域撤回水位
3. 在当前平台写窗口私有事务固定已启用配置及MG完整空间；首次或force使用独立持久代数高水位，GC不得回退代数
4. 同文档/完整修订/空间force=false复用已有构建；force=true只创建新staging，不改旧active
5. build、精确配置、后台批准元数据、完整原inputRef/原payload/去重键的持久派发意图及原回执同事务提交
6. HTTP202只确认本域持久接受，提交后尝试DispatchBuild；jobs不可达保留同一意图由恢复批次继续，不捏造job成功或新付费attempt

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写build+任务outbox
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.RuntimeFromContext`、`knowledge.ReadOperationReceipt`、`knowledge.SaveOperationReceipt`、`knowledge.ValidatePublication`、`knowledge.BeginIndexWrite`、`knowledge.ReadConfig`、`knowledge.QueueBuild`、`knowledge.DispatchBuild`

## knowledge.QueueBuild

- 级别：参考
- 文件：`modules/knowledge/internal/build/create.go`
- 作用：在已授权事务按完整修订/空间去重，单调分配代数并固定原任务inputRef与后台批准

```go
func QueueBuild(ctx context.Context, tx *sql.Tx, op OperationContext, config gen.IndexConfig, document gen.PublishedRevisionRef, force bool) (gen.IndexBuild, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在已授权事务按完整修订/空间去重，单调分配代数并固定原任务inputRef与后台批准
2. 新分配前核可信DataRoot的真实可用磁盘水位，≤10%停止新build，已有同源原结果复用不受影响；不造余额/额度

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：已授权业务或原事件链路内部调用，继承外层事务和批准
- 并发：本地短事务；外部RPC不持有SQL事务
- 幂等：原持久输入/去重键/版本唯一
- 依赖：`knowledge.ReadBuildRecord`、`knowledge.RuntimeFromContext`、`knowledge.EnsureBuildDiskBudget`

## knowledge.DispatchBuild

- 级别：参考
- 文件：`modules/knowledge/internal/build/create.go`
- 作用：读取原持久inputRef/去重键，锁外jobs创建并安全保存真实jobId/operationId，取消竞态不丢已接受引用

```go
func DispatchBuild(ctx context.Context, op OperationContext, buildID string) error
```

**输入**

无

**输出**

无

**步骤**

1. 读取原持久inputRef/去重键，锁外jobs创建并安全保存真实jobId/operationId，取消竞态不丢已接受引用
2. RPC前事务将pending置dispatching且核cancel_requested=0；取消可区分尚未派发与接受未知，重试仅原inputRef和去重键，绝不新增付费attempt
3. jobs CreateJob请求明确携原dataEpoch必填字段，严格核生产schema

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：已授权业务或原事件链路内部调用，继承外层事务和批准
- 并发：本地短事务；外部RPC不持有SQL事务
- 幂等：原持久输入/去重键/版本唯一
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.BeginIndexWrite`、`jobs:createJob`

## knowledge.FetchPublishedSource

- 级别：参考
- 文件：`modules/knowledge/internal/source/content.go`
- 作用：取得权威正文

```go
func FetchPublishedSource(ctx context.Context, op OperationContext, input FetchPublishedSourceInput) (FetchPublishedSourceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `FetchPublishedSourceInput` | 封闭业务输入；逐字段见types $defs/FetchPublishedSourceInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FetchPublishedSourceResult` | 成功结果；逐字段见types $defs/FetchPublishedSourceResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 先核原构建租约及不可变来源，释放SQL事务后才调用content精确publishedRevisionId
2. 严格生产方ReadPublishedRevisionResult schema，完整reference逐字段一致；canonical DocumentBody JSON字节≤16MiB且SHA256等于原contentHash
3. 返回完整结构JSON，保留稳定ID/块/marks/attrs；不把扁平文本摘要冒充权威正文
4. 回写前重新取得平台当前写窗口与同租约，原源JSON及hash同build唯一保存，异内容拒绝；不写外域私有文件

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写构建临时文件；不复制草稿
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.BeginBuildWrite`、`content:readPublishedRevision`

## knowledge.ListExtractableAssets

- 级别：参考
- 文件：`modules/knowledge/internal/source/media.go`
- 作用：筛选发布引用媒体

```go
func ListExtractableAssets(ctx context.Context, op OperationContext, input ListExtractableAssetsInput) (ListExtractableAssetsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ListExtractableAssetsInput` | 封闭业务输入；逐字段见types $defs/ListExtractableAssetsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListExtractableAssetsResult` | 成功结果；逐字段见types $defs/ListExtractableAssetsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 输入必须为已绑定发布引用的完整canonical DocumentBody，重算原contentHash，拒绝超过16MiB或旧epoch
2. 沿真实attrs及有序子树解析完整AssetVersionRef，保留最近稳定BlockRef/hash；不从URL/文件名/文本猜资源，不产生新块ID
3. 按assetId/version/hash去重，最多100资源；每件向media:readAsset传精确version并严格核生产schema、本站/ready及全字段相等，不换latest
4. MIME固定白名单且单件≤50MiB的入assets，其他授权引用明确unsupportedAssets；sources保留各原块来源与是否支持
5. 只读取资源授权，不预留/修改媒体引用，不覆盖人工caption

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.scanAssetSources`、`knowledge.extractableMediaType`、`media:readAsset`

## knowledge.scanAssetSources

- 级别：参考
- 文件：`modules/knowledge/internal/source/media.go`
- 作用：沿权威JSON的真实稳定块提取完整媒体版本及原块hash，不猜字符串资源

```go
func scanAssetSources(body []byte, document gen.PublishedRevisionRef) ([]AssetSource, error)
```

**输入**

无

**输出**

无

**步骤**

1. 沿权威JSON的真实稳定块提取完整媒体版本及原块hash，不猜字符串资源

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：已授权业务或原事件链路内部调用，继承外层事务和批准
- 并发：本地短事务；外部RPC不持有SQL事务
- 幂等：原持久输入/去重键/版本唯一
- 依赖：无

## knowledge.extractableMediaType

- 级别：参考
- 文件：`modules/knowledge/internal/source/media.go`
- 作用：固定支持PDF、纯文本/Markdown/CSV/JSON、受支持图像及视频MIME集合

```go
func extractableMediaType(mediaType string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 固定支持PDF、纯文本/Markdown/CSV/JSON、受支持图像及视频MIME集合

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：已授权业务或原事件链路内部调用，继承外层事务和批准
- 并发：本地短事务；外部RPC不持有SQL事务
- 幂等：原持久输入/去重键/版本唯一
- 依赖：无

## knowledge.CreateExtraction

- 级别：参考
- 文件：`modules/knowledge/internal/extract/create.go`
- 作用：按hash幂等提取资源

```go
func CreateExtraction(ctx context.Context, op OperationContext,input CreateExtractionInput)(CreateExtractionResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateExtractionInput` | 封闭业务输入；逐字段见types $defs/CreateExtractionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateExtractionResult` | 成功结果；逐字段见types $defs/CreateExtractionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 核验允许MIME
2. 按asset hash/extractorVersion去重
3. 记录来源授权
4. 排队独立租约
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 新build全量明批资源计划传入只内部非序列化ApprovedPlan；当前计划不匹配CONFLICT，已有关联同build仍先返回原owner
7. 意图改为稳定asset hash/MIME/size/task/extractor语义（不含可变记忆/模型），旧计划hash行回找最早真实owner再CAS建立语义索引；不同计划未终局拒新付费
8. 优先复用合法缓存或免费恢复原完整paid描述；只jobs真终局且完整原root费用无未决、无缺接受ID已外发、当前无可用完整结果时，新明批build可CAS替换意图新owner；原任务/paid输出/关联全部保留

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写extraction+job事件
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.BeginBuildWrite`、`knowledge.SameAsset`、`knowledge.ReadExtraction`、`knowledge.PrepareExtractionPlan`、`knowledge.DispatchExtraction`、`knowledge.InspectFailedExtraction`、`knowledge.SaveRecoveredExtraction`

## knowledge.PrepareExtractionPlan

- 级别：参考
- 文件：`modules/knowledge/internal/extract/create.go`
- 作用：首次冻结任务记忆/档位/覆盖/opaque声明；PDF免费解析、视频只原元数据、未知明确unsupported

```go
func PrepareExtractionPlan(ctx context.Context,op OperationContext,origin source.AssetSource,version,language string)(ExtractionPlan,error)
```

**输入**

无

**输出**

无

**步骤**

1. 首次冻结任务记忆/档位/覆盖/opaque声明；PDF免费解析、视频只原元数据、未知明确unsupported
2. 真实MG resolveTaskRoute返回档位/赋值/eligible候选/routeVersion和opaque模型hash，禁止本域手拼主备用；估价价格快照仅原全量明批注入，不重估

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ResolveTaskMemory`、`knowledge.ReadTaskRoute`

## knowledge.ReadExtraction

- 级别：参考
- 文件：`modules/knowledge/internal/extract/create.go`
- 作用：只读有精确当前来源授权关联的原结果/hash；缓存绑定新引用不覆盖原付费证明

```go
func ReadExtraction(ctx context.Context,op OperationContext,id string,origin source.AssetSource)(ExtractionArtifact,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读有精确当前来源授权关联的原结果/hash；缓存绑定新引用不覆盖原付费证明

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`

## knowledge.ExtractPdfText

- 级别：参考
- 文件：`modules/knowledge/internal/extract/pdf.go`
- 作用：提取PDF可用文字

```go
func ExtractPdfText(ctx context.Context, op OperationContext, input ExtractPdfTextInput) (ExtractPdfTextResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ExtractPdfTextInput` | 封闭业务输入；逐字段见types $defs/ExtractPdfTextInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExtractPdfTextResult` | 成功结果；逐字段见types $defs/ExtractPdfTextResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 执行器拥有独立提取租约和最终写资格；本函数不写DB或模型
2. 对完整AssetSource精确version/原发布引用通过media授权字节流，≤50MiB有界读/核hash及长度/Close，不取latest
3. 公共RunBoundedParser自身隔离子进程pdf-text-v1真实解析≤500页，输出≤16MiB，文字≤8MiB；完整真实页号与unsupported声明核对
4. 无文字明确unsupported，无OCR/自动转写，保留原资源/BlockRef；main已在任何配置密钥前接真实RunPDFParserChild

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNSUPPORTED_CAPABILITY` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：本地提取临时文件；无模型调用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数60000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.DownloadExtractionAsset`、`platform.RunBoundedParser`

## knowledge.RunPDFParserChild

- 级别：参考
- 文件：`modules/knowledge/internal/extract/pdf.go`
- 作用：配置/密钥前公共隔离解析入口；main真实装配未齐不计ready

```go
func RunPDFParserChild() (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 配置/密钥前公共隔离解析入口；main真实装配未齐不计ready

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`platform.RunParserChild`、`knowledge.ParsePDFPages`

## knowledge.ExtractImageDescription

- 级别：参考
- 文件：`modules/knowledge/internal/extract/image.go`
- 作用：显式生成图片检索描述

```go
func ExtractImageDescription(ctx context.Context, op OperationContext, input ExtractImageDescriptionInput) (ExtractImageDescriptionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ExtractImageDescriptionInput` | 封闭业务输入；逐字段见types $defs/ExtractImageDescriptionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExtractImageDescriptionResult` | 成功结果；逐字段见types $defs/ExtractImageDescriptionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只执行原发布任务批准的真实PNG/JPEG/WebP/GIF视觉资源，授权下载原version并核hash/长度；其他实际不支持的输入显式UNSUPPORTED_CAPABILITY
2. knowledge.describe-visual的任务记忆必须匹配源语言和原任务，vision档未设可交配置fast视觉能力，由MG主备用真实能力门禁决定，不推测外部模型成功
3. InvokeModel使用原提取拥有者/assetHash/task/profile/记忆版本键，真实付费步骤完整结果保留；跨构建缓存由提取创建/执行器拥有，不覆盖人工caption

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：可能产生MG费用；只保存派生文本
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.SameAsset`、`knowledge.DownloadExtractionAsset`、`knowledge.DescribeResource`

## knowledge.ExtractVideoMetadata

- 级别：参考
- 文件：`modules/knowledge/internal/extract/video.go`
- 作用：提取视频题名及授权字幕

```go
func ExtractVideoMetadata(ctx context.Context, op OperationContext, input ExtractVideoMetadataInput) (ExtractVideoMetadataResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ExtractVideoMetadataInput` | 封闭业务输入；逐字段见types $defs/ExtractVideoMetadataInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExtractVideoMetadataResult` | 成功结果；逐字段见types $defs/ExtractVideoMetadataResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 核固定正文canonical hash/epoch及真实原video blockId/hash，只读取正文已保存attrs.title/description和来源类型
2. metadata存在可ready，当前字幕schema不存在则transcriptState固定unsupported，无URL重抓、无自动转写
3. 上传视频保留真实asset；平台视频无视频asset时内部asset缺失，保留sourceBlock，绝不以poster冒视频

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写派生元数据，不请求外部平台密钥
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.FindVideoBlock`

## knowledge.FindVideoBlock

- 级别：参考
- 文件：`modules/knowledge/internal/extract/video.go`
- 作用：只按原有序正文树读取实际video块及规范节点

```go
func FindVideoBlock(raw json.RawMessage, blockID string) (json.RawMessage, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只按原有序正文树读取实际video块及规范节点

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.NormalizeSource

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/normalize.go`
- 作用：稳定规范化输入

```go
func NormalizeSource(ctx context.Context, op OperationContext, input NormalizeSourceInput) (NormalizeSourceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `NormalizeSourceInput` | 封闭业务输入；逐字段见types $defs/NormalizeSourceInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizeSourceResult` | 成功结果；逐字段见types $defs/NormalizeSourceResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 真实叶UTF8上限4MiB且schema最多1Mi Unicode字符；保留原叶稳定NFKC起始游标，不能从任意字节中部继续
2. 规范CRLF/CR到LF并用已批准x/text的NFKC稳定边界迭代，逐规范UTF8字节映射原叶最早源起点
3. 单次规范片段最多4MiB/1Mi Unicode字符；NFKC膨胀时在真实稳定归一化边界返回nextByte/source范围，不截任意字节、不伪造全叶结果
4. 消耗到EOF才省略nextByte，内部构建必须继续所有片段；没有付费、存储、模型token推测
5. 单次完整原机器页/描述最多8MiB，原content叶仍由来源入口限制4MiB/1Mi字符；每个真实稳定NFKC输出片段≤4MiB且≤1Mi字符后调用真实BPE，NextByte不截断

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.validBoundedText`、`knowledge.canonicalizeLineEndings`、`knowledge.checkContext`

## knowledge.validBoundedText

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/normalize.go`
- 作用：检查真实源叶非空、合法UTF8及4MiB字节界限

```go
func validBoundedText(value string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 检查真实源叶非空、合法UTF8及4MiB字节界限

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域规范化内部参考助手，不建立外部授权
- 并发：无共享状态
- 幂等：同输入或同取消状态同结果
- 依赖：无

## knowledge.canonicalizeLineEndings

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/normalize.go`
- 作用：统一CRLF/CR到LF并保留每个规范字节对应原UTF8起点

```go
func canonicalizeLineEndings(source string) ([]byte, []int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 统一CRLF/CR到LF并保留每个规范字节对应原UTF8起点

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域规范化内部参考助手，不建立外部授权
- 并发：无共享状态
- 幂等：同输入或同取消状态同结果
- 依赖：无

## knowledge.checkContext

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/normalize.go`
- 作用：传播规范化阶段已确定的取消和截止，错误不携带正文

```go
func checkContext(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 传播规范化阶段已确定的取消和截止，错误不携带正文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：本域规范化内部参考助手，不建立外部授权
- 并发：无共享状态
- 幂等：同输入或同取消状态同结果
- 依赖：无

## knowledge.SplitChunks

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/split.go`
- 作用：按源块及token边界分块

```go
func SplitChunks(ctx context.Context, op OperationContext, input SplitChunksInput) (SplitChunksResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SplitChunksInput` | 封闭业务输入；逐字段见types $defs/SplitChunksInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SplitChunksResult` | 成功结果；逐字段见types $defs/SplitChunksResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 输入为固定完整发布引用下可信SourceBlock真实叶片段，可共享原BlockRef但不得重复sourcePath/真实源范围
2. 真实BPE tokenEnds只核递增/覆盖，允许UTF8码点内部端点，不能以rune冒充token
3. 选择每块终点时不超过chunkTokens及64KiB并退到完整UTF8边界；重叠起点向前进至完整UTF8边界，实际重叠不超过配置，保留真实token序号计数
4. 单次最多256块并返回内部nextCursor，全构建ordinal保持；调用方必须续读所有片段，不能把有限批当完整完成
5. 新chunkId绑定完整BlockRef、sourcePath、真实源范围、规范区间、contextVersionHash及chunkHash，旧无path整块身份保持兼容；每字节sourceOffsets和asset来源不丢失

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

## knowledge.WriteStagingChunks

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：保存独立分块代数

```go
func WriteStagingChunks(ctx context.Context, op OperationContext, input WriteStagingChunksInput) (WriteStagingChunksResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `WriteStagingChunksInput` | 封闭业务输入；逐字段见types $defs/WriteStagingChunksInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteStagingChunksResult` | 成功结果；逐字段见types $defs/WriteStagingChunksResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验每chunk来源revision
2. 验证重复ID同hash
3. 一事务批量≤256
4. 不修改active pointer
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写chunks/terms staging
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.BeginBuildWrite`、`knowledge.storageError`

## knowledge.NewIndexRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：NewIndexRuntime的本域内部实现助手

```go
func NewIndexRuntime(runtime Runtime)(Runtime,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释
3. 实际装配严格MG profiles/models、jobs创建/取消及content源正文/发布事件运行载荷validator，缺生产schema启动失败
4. 严格GatewayQuoteCostResult生产schema用于真实全量费用分项，不本域复制价格计算
5. 严格GatewayResolveTaskRouteResult alias依生产方ResolveRouteResult，所有本域候选来自同一快照

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`platform.NewPayloadValidator`

## knowledge.WithIndexRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：WithIndexRuntime的本域内部实现助手

```go
func WithIndexRuntime(ctx context.Context, runtime Runtime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.RuntimeFromContext

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：RuntimeFromContext的本域内部实现助手

```go
func RuntimeFromContext(ctx context.Context) (Runtime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.ReadBuildRecord

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：ReadBuildRecord的本域内部实现助手

```go
func ReadBuildRecord(ctx context.Context, tx *sql.Tx, op OperationContext, id string) (BuildRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.storageError`

## knowledge.BeginBuildWrite

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：BeginBuildWrite的本域内部实现助手

```go
func BeginBuildWrite(ctx context.Context, op OperationContext, buildID, leaseID string) (context.Context, *sql.Tx, BuildRecord, context.CancelFunc, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.storageError`、`platform.NewWriteContext`

## knowledge.storageError

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：storageError的本域内部实现助手

```go
func storageError(err error) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.ReadChunks

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：ReadChunks的本域内部实现助手

```go
func ReadChunks(ctx context.Context, tx *sql.Tx, buildID string) ([]gen.Chunk, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.storageError`

## knowledge.ReadGeneration

- 级别：参考
- 文件：`modules/knowledge/internal/index/staging.go`
- 作用：ReadGeneration的本域内部实现助手

```go
func ReadGeneration(ctx context.Context, tx *sql.Tx, record BuildRecord) ([]GenerationEntry,string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释
3. 完整词项回验与BuildLexicalIndex一样基于独立机器prefix加原text的RetrievalText，不能按原text单独计算导致所有增强索引被错误拒绝

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.ReadChunks`、`knowledge.DecodeVector`、`knowledge.TokenizeText`、`knowledge.storageError`、`knowledge.RetrievalText`

## knowledge.BuildLexicalIndex

- 级别：参考
- 文件：`modules/knowledge/internal/index/lexical.go`
- 作用：建立词项索引

```go
func BuildLexicalIndex(ctx context.Context, op OperationContext, input BuildLexicalIndexInput) (BuildLexicalIndexResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BuildLexicalIndexInput` | 封闭业务输入；逐字段见types $defs/BuildLexicalIndexInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildLexicalIndexResult` | 成功结果；逐字段见types $defs/BuildLexicalIndexResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 固定unicode-code-cjk-bigram-v1：字母、数字、下划线组成保留大小写的代码词；组合标记接前词；标点分界
2. 中日韩使用连续bigram
3. 保留代码标识词
4. 写freq/length及BM25统计
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写staging terms及docfreq
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.BeginBuildWrite`、`knowledge.ReadChunks`、`knowledge.TokenizeText`、`knowledge.storageError`、`knowledge.RetrievalText`

## knowledge.TokenizeText

- 级别：参考
- 文件：`modules/knowledge/internal/index/lexical.go`
- 作用：TokenizeText的本域内部实现助手

```go
func TokenizeText(text string) []string
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.RetrievalText

- 级别：参考
- 文件：`modules/knowledge/internal/index/lexical.go`
- 作用：仅检索用机器背景加换行原text，保持原引文/偏移

```go
func RetrievalText(value gen.Chunk) string
```

**输入**

无

**输出**

无

**步骤**

1. 仅检索用机器背景加换行原text，保持原引文/偏移

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.RequestEmbeddings

- 级别：参考
- 文件：`modules/knowledge/internal/index/embedding.go`
- 作用：请求分块嵌入

```go
func RequestEmbeddings(ctx context.Context, op OperationContext, input RequestEmbeddingsInput) (RequestEmbeddingsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RequestEmbeddingsInput` | 封闭业务输入；逐字段见types $defs/RequestEmbeddingsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RequestEmbeddingsResult` | 成功结果；逐字段见types $defs/RequestEmbeddingsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 读取固定chunkHash
2. 64项且总输入≤模型预算
3. 继承ctx调用MG
4. 对每返回space和dimension严格校验
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 每次embedding请求显式携完整expectedEmbeddingSpace，由MG在费用预占及主备用派发前硬核；不只事后发现错空间
7. 每项完整检索文本重新真实BPE核embeddingTokenCount及权威模型实际总token预算；无装配假额度，输入摘要含完整检索文本hash

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG产生费用；本服务不预留额度
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.InvokeModel`、`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.ReadChunks`、`knowledge.storageError`、`knowledge.CompletedOutput`、`knowledge.WithModelOwner`、`knowledge.ReadModelCatalog`、`knowledge.EmbeddingProfile`、`knowledge.RetrievalText`、`knowledge.TokenizeSource`、`knowledge.BeginBuildWrite`

## knowledge.StoreEmbeddings

- 级别：参考
- 文件：`modules/knowledge/internal/index/embedding.go`
- 作用：持久化验证过的向量

```go
func StoreEmbeddings(ctx context.Context, op OperationContext, input StoreEmbeddingsInput) (StoreEmbeddingsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `StoreEmbeddingsInput` | 封闭业务输入；逐字段见types $defs/StoreEmbeddingsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `StoreEmbeddingsResult` | 成功结果；逐字段见types $defs/StoreEmbeddingsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 等长和维度检查
2. 拒绝非有限数
3. 校验lease/chunkHash未变
4. 原子写vectorHash与向量
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写staging vectors
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.BeginBuildWrite`、`knowledge.storageError`

## knowledge.DecodeVector

- 级别：参考
- 文件：`modules/knowledge/internal/index/embedding.go`
- 作用：DecodeVector的本域内部实现助手

```go
func DecodeVector(raw []byte, hash string, dimensions int) ([]float64,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.ValidateBuild

- 级别：参考
- 文件：`modules/knowledge/internal/build/validate.go`
- 作用：验证完整索引代数

```go
func ValidateBuild(ctx context.Context, op OperationContext, input ValidateBuildInput) (ValidateBuildResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateBuildInput` | 封闭业务输入；逐字段见types $defs/ValidateBuildInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateBuildResult` | 成功结果；逐字段见types $defs/ValidateBuildResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. count chunks==terms==vectors
2. 检查每chunk空间/hash
3. 校验索引文件checksum
4. 向content再次验证current/visible
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：标validating；不激活
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.ReadGeneration`、`knowledge.ValidatePublication`、`knowledge.BeginBuildWrite`

## knowledge.ActivateBuild

- 级别：参考
- 文件：`modules/knowledge/internal/build/activate.go`
- 作用：原子切换当前索引

```go
func ActivateBuild(ctx context.Context, op OperationContext, input ActivateBuildInput) (ActivateBuildResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ActivateBuildInput` | 封闭业务输入；逐字段见types $defs/ActivateBuildInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ActivateBuildResult` | 成功结果；逐字段见types $defs/ActivateBuildResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 读取权威验证结果且有效期≤1秒
2. 再校验dataEpoch
3. 文档/space锁CAS active generation
4. 成功写active及index.ready否则superseded
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 新退役状态同时记录真实retired_at_ns供24小时GC；旧未知退役时刻不回填

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：原子active pointer更新与outbox；旧文件延迟回收
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.ReadGeneration`、`knowledge.BeginBuildWrite`、`platform.EnqueueEvent`、`knowledge.CommitExecutionResult`

## knowledge.CancelBuild

- 级别：约束
- 文件：`modules/knowledge/internal/build/cancel.go`
- 作用：取消入库链路

```go
func CancelBuild(ctx context.Context, op OperationContext, input CancelBuildInput) (CancelBuildResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CancelBuildInput` | 封闭业务输入；逐字段见types $defs/CancelBuildInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CancelBuildResult` | 成功结果；逐字段见types $defs/CancelBuildResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 即时入口已核knowledge.manage，本域核可信portal管理主体；读取原幂等回执，同键异输入CONFLICT
2. 当前写窗口事务内先持久取消staging写资格、清lease并保留active；取消意图和原安全结果同事务，失败/终止构建仍可请求清理未知调用
3. 提交后锁外CancelBuildSideEffects，按持久原owner/site/epoch/root请求jobs及平台网关取消；不预占/退还费用，不删未知引用
4. 重放原结果仍继续未完成取消副作用；jobs接受结果未知时保留dispatching和取消意图并失败关闭，不假称已清理

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：终止worker并清理非active临时数据
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.BeginIndexWrite`、`knowledge.ReadOperationReceipt`、`knowledge.SaveOperationReceipt`、`knowledge.ReadBuildRecord`、`knowledge.CancelBuildSideEffects`、`knowledge.CancelUnusedExtractions`

## knowledge.RetireDocumentIndex

- 级别：约束
- 文件：`modules/knowledge/internal/index/retire.go`
- 作用：响应撤回/删除

```go
func RetireDocumentIndex(ctx context.Context, op OperationContext, input RetireDocumentIndexInput) (RetireDocumentIndexResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RetireDocumentIndexInput` | 封闭业务输入；逐字段见types $defs/RetireDocumentIndexInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RetireDocumentIndexResult` | 成功结果；逐字段见types $defs/RetireDocumentIndexResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 接原撤回信封核本站/文档/当前epoch及visibility源水位；同事务最先ConsumeInbox，不重构假事件
2. 只有更高水位才关闭active指针，并将未完成构建置superseded、清原lease且持久cancel_requested；已完成active历史状态保留，独立generation_state改retired
3. 捕获每个真实旧active完整发布/空间，按EventIndexRetiredPayload逐项写outbox，与水位/inbox/失效同事务
4. 提交后锁外CancelDocumentSideEffects请求jobs/MG取消，不退款、不丢费用未知引用；失败不确认，重复原事件仍继续原取消意图，禁止旧事件取消新发布构建

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：active失活；qa缓存通过事件失效
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.BeginIndexWrite`、`knowledge.ConsumeInbox`、`knowledge.ReadBuildRecord`、`knowledge.CancelDocumentSideEffects`、`platform.EnqueueEvent`、`knowledge.CancelUnusedExtractions`

## knowledge.NormalizeQuery

- 级别：参考
- 文件：`modules/knowledge/internal/search/query.go`
- 作用：规范检索请求

```go
func NormalizeQuery(ctx context.Context, op OperationContext, input NormalizeQueryInput) (NormalizeQueryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `NormalizeQueryInput` | 封闭业务输入；逐字段见types $defs/NormalizeQueryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizeQueryResult` | 成功结果；逐字段见types $defs/NormalizeQueryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. NFC/LF
2. trim首尾空白但保留代码大小写
3. 上限2048字符/8192字节
4. 空查询拒绝
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

## knowledge.EmbedQuery

- 级别：参考
- 文件：`modules/knowledge/internal/search/query.go`
- 作用：取得匹配空间查询向量

```go
func EmbedQuery(ctx context.Context, op OperationContext, input EmbedQueryInput) (EmbedQueryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `EmbedQueryInput` | 封闭业务输入；逐字段见types $defs/EmbedQueryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EmbedQueryResult` | 成功结果；逐字段见types $defs/EmbedQueryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 检查MG档位空间完全等同
2. ctx继承调用
3. dimension与有限数检查
4. 取消后不返回向量
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG可能收费；不改变文档索引
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.InvokeModel`、`knowledge.searchRuntime`、`knowledge.CreateQueryEmbedding`、`knowledge.validVector`

## knowledge.SearchLexical

- 级别：参考
- 文件：`modules/knowledge/internal/search/lexical.go`
- 作用：执行全文召回

```go
func SearchLexical(ctx context.Context, op OperationContext, input SearchLexicalInput) (SearchLexicalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SearchLexicalInput` | 封闭业务输入；逐字段见types $defs/SearchLexicalInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SearchLexicalResult` | 成功结果；逐字段见types $defs/SearchLexicalResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只读active generations快照
2. BM25 k1=1.2 b=0.75
3. 按score降序chunkId稳定排序
4. 取最多80项
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只读，候选此阶段未授权不能外发
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.activeSnapshot`、`knowledge.TokenizeText`

## knowledge.SearchVector

- 级别：参考
- 文件：`modules/knowledge/internal/search/vector.go`
- 作用：执行向量召回

```go
func SearchVector(ctx context.Context, op OperationContext, input SearchVectorInput) (SearchVectorResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SearchVectorInput` | 封闭业务输入；逐字段见types $defs/SearchVectorInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SearchVectorResult` | 成功结果；逐字段见types $defs/SearchVectorResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 空间完全匹配
2. cosine距离并排除零向量
3. 只active索引
4. 分数降序chunkId稳定排序最多80
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只读；不得跨space补足候选
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.activeSnapshot`、`knowledge.validVector`、`knowledge.unitVector`

## knowledge.validVector

- 级别：参考
- 文件：`modules/knowledge/internal/search/vector.go`
- 作用：validVector的本域内部实现助手

```go
func validVector(vector []float64, dimensions int) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.unitVector

- 级别：参考
- 文件：`modules/knowledge/internal/search/vector.go`
- 作用：unitVector的本域内部实现助手

```go
func unitVector(vector []float64) ([]float64, bool)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.FuseCandidates

- 级别：参考
- 文件：`modules/knowledge/internal/search/fusion.go`
- 作用：确定性融合两种召回

```go
func FuseCandidates(ctx context.Context, op OperationContext, input FuseCandidatesInput) (FuseCandidatesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `FuseCandidatesInput` | 封闭业务输入；逐字段见types $defs/FuseCandidatesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FuseCandidatesResult` | 成功结果；逐字段见types $defs/FuseCandidatesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 以chunkId去重
2. score=1/(60+lexicalRank)+1/(60+vectorRank)缺失项0
3. hash冲突拒绝
4. score/chunkId稳定排序40项
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：无

## knowledge.ValidateCandidates

- 级别：参考
- 文件：`modules/knowledge/internal/search/visibility.go`
- 作用：权威过滤检索材料

```go
func ValidateCandidates(ctx context.Context, op OperationContext, input ValidateCandidatesInput) (ValidateCandidatesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ValidateCandidatesInput` | 封闭业务输入；逐字段见types $defs/ValidateCandidatesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateCandidatesResult` | 成功结果；逐字段见types $defs/ValidateCandidatesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 按PublishedRevisionRef去重
2. content批量validate
3. visible且matchesCurrent且dataEpoch一致才留
4. 写visibilityRevision并拒绝不可用结果
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只读；不靠本地撤回事件兜底授权
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`knowledge.searchRuntime`、`knowledge.publicationKey`

## knowledge.RerankCandidates

- 级别：参考
- 文件：`modules/knowledge/internal/search/rerank.go`
- 作用：可选AI重排

```go
func RerankCandidates(ctx context.Context, op OperationContext, input RerankCandidatesInput) (RerankCandidatesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RerankCandidatesInput` | 封闭业务输入；逐字段见types $defs/RerankCandidatesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RerankCandidatesResult` | 成功结果；逐字段见types $defs/RerankCandidatesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 已配置的QA原生rerank先核档位成本类别；未配时从MG真实目录按可信costClass选择fast/chat，不使用空或background默认profile；最多20项和800输出token
2. 只接受既有候选ID的完整无重复排列，未处理尾项保持次序
3. 继承可信根任务、费用归属、已消费的QA许可与共享截止，调用平台模型助手
4. 仅格式不合格追加一次独立attempt，仍不合法回退原顺序；依赖或收费未知错误不重放

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：可选MG费用；不写索引
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.InvokeModel`、`knowledge.searchRuntime`、`knowledge.requestRerank`

## knowledge.requestRerank

- 级别：参考
- 文件：`modules/knowledge/internal/search/rerank.go`
- 作用：requestRerank的本域内部实现助手

```go
func requestRerank(ctx context.Context, op OperationContext, input RerankCandidatesInput, attempt int) ([]string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释
3. 原生档位必须属于可信成本类别，fast档位由真实MG目录选择；当前完整Text优先，收费未知有有效输出仍可消费，不重新付费重排

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：经模型网关产生独立attempt，收费证据保留于MG
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.InvokeModel`、`knowledge.CompletedOutput`、`knowledge.CompletedText`、`knowledge.RuntimeFromContext`、`knowledge.ReadModelCatalog`、`knowledge.TextProfile`

## knowledge.SelectContext

- 级别：参考
- 文件：`modules/knowledge/internal/search/context.go`
- 作用：选择答案材料

```go
func SelectContext(ctx context.Context, op OperationContext, input SelectContextInput) (SelectContextResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SelectContextInput` | 封闭业务输入；逐字段见types $defs/SelectContextInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SelectContextResult` | 成功结果；逐字段见types $defs/SelectContextResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 每文档最多3块
2. 按排序累计token
3. 超预算跳过整块不截断引用偏移
4. 记录truncated
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

## knowledge.Retrieve

- 级别：约束
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：统一混合检索

```go
func Retrieve(ctx context.Context, op OperationContext, input RetrieveInput) (RetrieveResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RetrieveInput` | 封闭业务输入；逐字段见types $defs/RetrieveInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RetrieveResult` | 成功结果；逐字段见types $defs/RetrieveResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 问答分支从本次输入绑定已消费准入许可至请求独立模型上下文；原rootOperationId、dataEpoch、访客subject必须一致，禁止启动时共享访客许可。最终签名由model-gateway验证。
2. NormalizeQuery规范query与基准语言
3. 请求携带的queryEmbedding校验queryHash/space/vectorHash/epoch后复用，否则EmbedQuery
4. SearchLexical与SearchVector各80→FuseCandidates取40→ValidateCandidates权威过滤→可选RerankCandidates
5. SelectContext按预算选材；当前页范围可优先，最终仍经content可见性校验

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：仅query embedding/rerank可能MG收费
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`memory:resolveContext`、`knowledge.NormalizeQuery`、`knowledge.EmbedQuery`、`knowledge.SearchLexical`、`knowledge.SearchVector`、`knowledge.FuseCandidates`、`knowledge.ValidateCandidates`、`knowledge.RerankCandidates`、`knowledge.SelectContext`、`knowledge.searchRuntime`、`knowledge.activeSnapshot`、`knowledge.BindAdmissionPermit`

## knowledge.NewSearchRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：NewSearchRuntime的本域内部实现助手

```go
func NewSearchRuntime(runtime Runtime)(Runtime,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`platform.NewPayloadValidator`

## knowledge.WithSearchRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：WithSearchRuntime的本域内部实现助手

```go
func WithSearchRuntime(ctx context.Context, runtime Runtime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.searchRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：searchRuntime的本域内部实现助手

```go
func searchRuntime(ctx context.Context) (Runtime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.activeSnapshot

- 级别：参考
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：activeSnapshot的本域内部实现助手

```go
func activeSnapshot(ctx context.Context, op OperationContext) (Snapshot, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.searchRuntime`、`knowledge.ReadActiveSnapshot`

## knowledge.publicationKey

- 级别：参考
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：publicationKey的本域内部实现助手

```go
func publicationKey(reference gen.PublishedRevisionRef) string
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.sortedSnapshotRows

- 级别：参考
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：sortedSnapshotRows的本域内部实现助手

```go
func sortedSnapshotRows(rows []SnapshotRow)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.ReadActiveSnapshot

- 级别：参考
- 文件：`modules/knowledge/internal/search/service.go`
- 作用：ReadActiveSnapshot的本域内部实现助手

```go
func (reader SQLSnapshotReader) ReadActiveSnapshot(ctx context.Context, op OperationContext, requested gen.EmbeddingSpace) (Snapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.ReadBuildRecord`、`knowledge.ReadGeneration`、`knowledge.sortedSnapshotRows`

## knowledge.GetBuild

- 级别：约束
- 文件：`modules/knowledge/internal/admin/read.go`
- 作用：读取入库状态

```go
func GetBuild(ctx context.Context, op OperationContext, input GetBuildInput) (GetBuildResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `GetBuildInput` | 封闭业务输入；逐字段见types $defs/GetBuildInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `GetBuildResult` | 成功结果；逐字段见types $defs/GetBuildResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 验证knowledge.manage
2. 读build快照
3. 不返回源全文
4. 状态未知不自动重跑
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.beginAdminRead`、`knowledge.ReadBuildRecord`

## knowledge.ListBuilds

- 级别：约束
- 文件：`modules/knowledge/internal/admin/read.go`
- 作用：列入库任务

```go
func ListBuilds(ctx context.Context, op OperationContext, input ListBuildsInput) (ListBuildsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ListBuildsInput` | 封闭业务输入；逐字段见types $defs/ListBuildsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListBuildsResult` | 成功结果；逐字段见types $defs/ListBuildsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限检查
2. 按buildId稳定keyset
3. limit默认20
4. 返回source ref与错误码
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.beginAdminRead`、`knowledge.ReadBuildRecord`

## knowledge.WithAdminSigningKey

- 级别：参考
- 文件：`modules/knowledge/internal/admin/read.go`
- 作用：WithAdminSigningKey的本域内部实现助手

```go
func WithAdminSigningKey(ctx context.Context,key platform.SigningKey)context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.AuthorizeAdmin

- 级别：参考
- 文件：`modules/knowledge/internal/admin/read.go`
- 作用：authorizeAdmin的本域内部实现助手

```go
func AuthorizeAdmin(op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.beginAdminRead

- 级别：参考
- 文件：`modules/knowledge/internal/admin/read.go`
- 作用：beginAdminRead的本域内部实现助手

```go
func beginAdminRead(ctx context.Context,op OperationContext)(*sql.Tx,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.RuntimeFromContext`

## knowledge.ConsumeEvents

- 级别：约束
- 文件：`modules/knowledge/internal/events/inbox.go`
- 作用：从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认

```go
func ConsumeEvents(ctx context.Context, op OperationContext, input ConsumeEventsInput) (ConsumeEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 本域后台worker以服务身份构建的操作上下文，含当前dataEpoch | callerService为本域；dataEpoch等于site-config当前active epoch |
| `input` | `ConsumeEventsInput` | 订阅循环参数 | 必须通过公共schema；consumerId等于本域slug |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeEventsResult` | 循环退出时的累计计数 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 只装配knowledge后台及events.json两种content发布事件，SiteID来自可信op
2. 复用平台NewEventConsumer适配真实gen.JobsClient订阅与严格AckResult，不伪造worker方法
3. ValidatePayload仅在公共订阅边界按生产schema校验一次；handler接原完整信封，dataEpoch/runtime/维护/过期上下文由公共循环控制
4. 本地handler提交后才确认，重复/业务skipped返回公共ErrEventSkipped；失败不确认，不伪填零统计
5. 复用RunConsumerWithResult返回实际Acknowledged/Skipped/Failed，不复制SSE/重连/并发/确认循环

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs订阅或确认不可用；重连后续传（可重试）
- `VALIDATION`：事件信封或载荷不符合登记schema；隔离并确认（不可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash；隔离报警（不可重试）

- 副作用：HTTP长连接jobs:subscribeEvents；调用本域登记的事件处理函数；HTTP调用jobs:acknowledgeEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个订阅循环；同subject串行，不同subject最多maxInFlight并发；事务与CAS由处理函数负责
- 幂等：jobs至少一次投递；本域inbox按eventId+payloadHash去重，重复投递只重新确认
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.HandlePublication`、`knowledge.RetireDocumentIndex`、`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`platform.NewEventConsumer`、`platform.RunConsumerWithResult`

## knowledge.PublishOutbox

- 级别：约束
- 文件：`modules/knowledge/internal/events/outbox.go`
- 作用：发布索引生命周期事件

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

1. 只接受知识后台，以当前runtime和同代DB装配平台NewEventPublisher/RunRelayBatch
2. 每批最多100，租约、原eventId发布、退避/隔离/确认及旧代清理由公共relay实现，不复制循环
3. 运行权威epoch变化时拒绝旧DB写入；结果published/pending仅为该批真实计数，workerId保留兼容不冒充租约身份

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：at-least-once持久事件
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.RuntimeFromContext`、`jobs:publishEvent`、`platform.NewEventPublisher`、`platform.RunRelayBatch`

## knowledge.RecoverBuildLeases

- 级别：参考
- 文件：`modules/knowledge/internal/runtime/recovery.go`
- 作用：恢复入库执行租约

```go
func RecoverBuildLeases(ctx context.Context, op OperationContext, input RecoverBuildLeasesInput) (RecoverBuildLeasesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RecoverBuildLeasesInput` | 封闭业务输入；逐字段见types $defs/RecoverBuildLeasesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverBuildLeasesResult` | 成功结果；逐字段见types $defs/RecoverBuildLeasesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 以本域服务有界扫描本站同代非终态到期原构建，以及尚未完成清理的持久取消意图（即使构建已终态）；取消清理不因旧镜像租约未到期而跳过，真实时间解析及literal CAS防续约竞态
2. 锁外以原PublishedRevisionRef查content权威；未知失败关闭，明确失效时CAS关闭旧写资格并保留独立取消意图，锁外实际取消原job/MG
3. 只对pending/dispatching意图使用原inputRef/root/dedupeKey/dataEpoch续派并保留真实job引用，no-op取消不计requeued
4. dispatched到期原任务保留原阶段/模型引用给jobs owner恢复证明，不另建dedupe绕过orphan，不把费用未知当0重算

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：更新build/lease和jobs；不重复未知收费调用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.ValidatePublication`、`knowledge.BeginIndexWrite`、`knowledge.CancelBuildSideEffects`、`knowledge.DispatchBuild`、`knowledge.CancelUnusedExtractions`

## knowledge.RecoverExtractionTasks

- 级别：参考
- 文件：`modules/knowledge/internal/runtime/recovery.go`
- 作用：有界续派原提取意图与原取消清理，已派发到期由jobs owner恢复证明

```go
func RecoverExtractionTasks(ctx context.Context,op OperationContext,limit int)error
```

**输入**

无

**输出**

无

**步骤**

1. 按原scope/完整持久输入组合真实已有机制，不换原key或批准

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域后台服务组合调用
- 并发：本站同代真实原拥有者与安全窗口
- 幂等：只用原输入/原key/原取消意图
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.DispatchExtraction`、`knowledge.CancelExtractionSideEffects`

## knowledge.CollectRetiredIndexes

- 级别：参考
- 文件：`modules/knowledge/internal/runtime/gc.go`
- 作用：回收失活索引代数

```go
func CollectRetiredIndexes(ctx context.Context, op OperationContext, input CollectRetiredIndexesInput) (CollectRetiredIndexesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CollectRetiredIndexesInput` | 封闭业务输入；逐字段见types $defs/CollectRetiredIndexesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CollectRetiredIndexesResult` | 成功结果；逐字段见types $defs/CollectRetiredIndexesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只接受本域后台服务；olderThan不得晚于服务当前时刻减24小时
2. 平台写窗口与同代私有SQL事务内最多扫描limit个已知retired_at_ns的retired代数，旧迁移未知退役时间不假填不删除
3. 检查active指针、未终结调度意图和未终结/收费未知模型步骤引用，有引用保留
4. 本域所有索引在SQL，读者在同一SQLite快照载入完整材料后释放事务，无额外文件reader句柄；SQLite为并发快照保留旧页
5. 按外键顺序删向量/词项/块/私有源/终结调度/代数，同事务提交，不删active或未完成结果
6. 已付费模型完成结果独立保留供哈希复用，不随索引GC删除；收费未知或未完成结果仍阻止回收

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：删除无引用派生数据；不删除权威正文
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.BeginIndexWrite`

## knowledge.RedactDiagnostics

- 级别：约束
- 文件：`modules/knowledge/internal/admin/diagnostics.go`
- 作用：返回安全诊断

```go
func RedactDiagnostics(ctx context.Context, op OperationContext, input RedactDiagnosticsInput) (RedactDiagnosticsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RedactDiagnosticsInput` | 封闭业务输入；逐字段见types $defs/RedactDiagnosticsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RedactDiagnosticsResult` | 成功结果；逐字段见types $defs/RedactDiagnosticsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只返状态/计数/版本
2. 不返全文或媒体凭证
3. 检查权限
4. 固定诊断无模型探测
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.beginAdminRead`、`knowledge.ReadBuildRecord`

## knowledge.ReadSections

- 级别：约束
- 文件：`modules/knowledge/internal/search/sections.go`
- 作用：按引用读取证据片段

```go
func ReadSections(ctx context.Context, op OperationContext, input ReadSectionsInput) (ReadSectionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadSectionsInput` | 封闭业务输入；逐字段见types $defs/ReadSectionsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadSectionsResult` | 成功结果；逐字段见types $defs/ReadSectionsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. service身份
2. chunk必须active且来源与传入ref相同
3. content批量校验current/visible/epoch
4. 返回完整sourceOffsets与hash
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
6. 新sourcePath逐项回读私有固定发布body hash/原块/JSON叶与原文范围，绝不取latest；资源提取证据视图尚未装配则该分支失败关闭并保持partial

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`knowledge.activeSnapshot`、`knowledge.publicationKey`、`knowledge.ValidateCandidates`、`knowledge.ValidateSectionSources`

## knowledge.AcquireBarrier

- 级别：约束
- 文件：`modules/knowledge/internal/protection/protocol.go`
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
3. 屏障停止新build、active切换和staging写入，等待reader lease排空
4. 导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建
5. 派生索引可由管理员另发重建任务。
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

## knowledge.ReleaseBarrier

- 级别：约束
- 文件：`modules/knowledge/internal/protection/protocol.go`
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

## knowledge.ExportSnapshot

- 级别：约束
- 文件：`modules/knowledge/internal/protection/protocol.go`
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
3. 屏障停止新build、active切换和staging写入，等待reader lease排空
4. 导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建
5. 派生索引可由管理员另发重建任务。
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

## knowledge.StageSnapshot

- 级别：约束
- 文件：`modules/knowledge/internal/protection/protocol.go`
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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`、`platform.SnapshotManager.Stage`

## knowledge.DiscardSnapshot

- 级别：约束
- 文件：`modules/knowledge/internal/protection/protocol.go`
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

## knowledge.EnrichChunkContext

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：发布后补检索片段上下文

```go
func EnrichChunkContext(ctx context.Context, op OperationContext, input EnrichChunkContextInput) (EnrichChunkContextResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `EnrichChunkContextInput` | 封闭业务输入；逐字段见types $defs/EnrichChunkContextInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EnrichChunkContextResult` | 成功结果；逐字段见types $defs/EnrichChunkContextResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 整篇固定发布正文canonical JSON原文核contentHash并保留全部消息；超过65536字符等固定消息/模型预算失败关闭，保留旧active和原付费结果，无基础索引降级/截断/额外摘要
2. fast知识上下文任务、可信background费用类别，最多16片段及500输出token，严格原chunkId与一至两句短背景，不修改原text/hash/偏移
3. contextVersionHash绑定文档hash/任务记忆hash及修订/真实模型声明；缓存另绑原块/path/原区间避免重复短句串背景，完整源相同重发布可复用
4. 付费前同事务认领原来源意图，其他构建等原结果而不重新调用；机器前缀/摘要/真实invocation持久，保留所有已付费结果
5. 机器背景独立字段由ApplyChunkContext再次真实BPE完整检索文本；词项/embedding使用前缀+换行+原文，源证据完全不变
6. 首次完整任务记忆/档位/声明模型计划在原build lease内持久冻结，重试漂移在认领新上下文付费意图前CONFLICT
7. 缓存按每项实际winning模型snapshot/完整源/记忆版本保存，任务覆盖与有序备用仅决定当前缓存资格；逐项prefixContextHashes/modelSnapshotHashes允许同批已有不同真实来源，不把计划hash冒充实际来源

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG费用及machine derived context记录
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.BeginBuildWrite`、`knowledge.WithModelOwner`、`knowledge.InvokeModel`、`knowledge.ReadContextPrefixes`、`knowledge.ContextSourceKey`、`knowledge.ClaimContextIntents`、`knowledge.ContextSemanticKey`、`knowledge.FreezeContextPlan`、`knowledge.ContextResultHash`、`knowledge.WinningModelSnapshot`、`knowledge.ReadTaskRoute`

## knowledge.ReadContextPrefixes

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：严格完整JSON/原ID/一至两句，不把机器背景当原引文

```go
func ReadContextPrefixes(outcome modelruntime.InvokeModelResult, chunks []gen.Chunk) (map[string]string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 严格完整JSON/原ID/一至两句，不把机器背景当原引文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.HasCompleteResult`、`knowledge.CompletedText`

## knowledge.ContextSourceKey

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：真实原块/path/区间与原文hash绑定，避免重复短句串背景

```go
func ContextSourceKey(value gen.Chunk) string
```

**输入**

无

**输出**

无

**步骤**

1. 真实原块/path/区间与原文hash绑定，避免重复短句串背景

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.ClaimContextIntents

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：跨构建同源付费意图只归原build/root/op，其他构建等待原结果不重花费

```go
func ClaimContextIntents(ctx context.Context, tx *sql.Tx, op OperationContext, buildID, contextHash string, chunks []gen.Chunk) error
```

**输入**

无

**输出**

无

**步骤**

1. 跨构建同源付费意图只归原build/root/op，其他构建等待原结果不重花费

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.ContextSourceKey`

## knowledge.ApplyChunkContext

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：只新增机器派生字段，完整检索文本真实BPE，保留原text/hash/证据偏移

```go
func ApplyChunkContext(ctx context.Context, op OperationContext, chunks []gen.Chunk, context EnrichChunkContextResult, modelID string, space gen.EmbeddingSpace) ([]gen.Chunk, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只新增机器派生字段，完整检索文本真实BPE，保留原text/hash/证据偏移
2. 逐项使用实际背景版本与winning模型证明，不把整批请求计划hash当每项真实模型版本

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.TokenizeSource`、`knowledge.RetrievalText`

## knowledge.ContextSemanticKey

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：原片段身份构成稳定上下文阶段key，不含模型/记忆配置

```go
func ContextSemanticKey(chunks []gen.Chunk) string
```

**输入**

无

**输出**

无

**步骤**

1. 原片段身份构成稳定上下文阶段key，不含模型/记忆配置

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.FreezeContextPlan

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：首次完整任务记忆/档位计划及hash冻结，重试漂移在新付费意图前CONFLICT

```go
func FreezeContextPlan(ctx context.Context, op OperationContext, buildID, leaseID string, plan ContextPlan) error
```

**输入**

无

**输出**

无

**步骤**

1. 首次完整任务记忆/档位计划及hash冻结，重试漂移在新付费意图前CONFLICT

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.BeginBuildWrite`

## knowledge.ContextResultHash

- 级别：参考
- 文件：`modules/knowledge/internal/chunk/context.go`
- 作用：本域机器背景版本绑定完整源/真实记忆/实际winning快照，不计算MG快照算法

```go
func ContextResultHash(documentHash string, memory platform.MemoryContext, snapshot string) string
```

**输入**

无

**输出**

无

**步骤**

1. 本域机器背景版本绑定完整源/真实记忆/实际winning快照，不计算MG快照算法

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.InspectEvidence

- 级别：约束
- 文件：`modules/knowledge/internal/search/inspect.go`
- 作用：快速检索员有限核查

```go
func InspectEvidence(ctx context.Context, op OperationContext, input InspectEvidenceInput) (InspectEvidenceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `InspectEvidenceInput` | 封闭业务输入；逐字段见types $defs/InspectEvidenceInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InspectEvidenceResult` | 成功结果；逐字段见types $defs/InspectEvidenceResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 问答分支从本次输入绑定已消费准入许可至请求独立模型上下文；原rootOperationId、dataEpoch、访客subject必须一致，禁止启动时共享访客许可。最终签名由model-gateway验证。
2. 根task qa.inspect owner=knowledge fast档max800output token并注入project overview
3. 只选择本次检索合法IDs，展开小节仍content当前校验
4. 3轮/5工具调用计数来自可信根任务状态CAS不由浏览器覆盖
5. 到上限用已有材料返回finished，费用继承qa costClass。
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG核查费用及可选knowledge工具调用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`memory:resolveContext`、`knowledge.InvokeModel`

## knowledge.CreateQueryEmbedding

- 级别：约束
- 文件：`modules/knowledge/internal/search/query-embedding.go`
- 作用：统一生成并复用查询嵌入

```go
func CreateQueryEmbedding(ctx context.Context, op OperationContext, input CreateQueryEmbeddingInput) (CreateQueryEmbeddingResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateQueryEmbeddingInput` | 封闭业务输入；逐字段见types $defs/CreateQueryEmbeddingInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateQueryEmbeddingResult` | 成功结果；逐字段见types $defs/CreateQueryEmbeddingResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 问答分支从本次输入绑定已消费准入许可至请求独立模型上下文；原rootOperationId、dataEpoch、访客subject必须一致，禁止启动时共享访客许可。最终签名由model-gateway验证。
2. rootOperationId/queryHash/space/dataEpoch singleflight且唯一
3. 根knowledge.embed embedding档0文本output
4. MG保留独立attempt费用证据
5. 返回完整空间/vectorHash供QA cache与本域search复用
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
7. 每次embedding请求显式携完整expectedEmbeddingSpace，由MG在费用预占及主备用派发前硬核；不只事后发现错空间
8. 原结果优先复用，新调用按可信op.costClass选择主模型完整同空间的embedding档位；QA不复用background档位或改费用类别

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG费用与本根短期向量记录，TTL10分钟
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.InvokeModel`、`knowledge.searchRuntime`、`knowledge.NormalizeQuery`、`knowledge.acquireQueryFlight`、`knowledge.RuntimeFromContext`、`knowledge.CompletedOutput`、`knowledge.validVector`、`platform.NewWriteContext`、`knowledge.BindAdmissionPermit`、`knowledge.ReadModelCatalog`、`knowledge.EmbeddingProfile`

## knowledge.acquireQueryFlight

- 级别：参考
- 文件：`modules/knowledge/internal/search/query-embedding.go`
- 作用：acquireQueryFlight的本域内部实现助手

```go
func acquireQueryFlight(ctx context.Context,key string) (func(),error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.DescribeAttachment

- 级别：参考
- 文件：`modules/knowledge/internal/extract/attachment.go`
- 作用：发布附件描述与非文本提取

```go
func DescribeAttachment(ctx context.Context, op OperationContext, input DescribeAttachmentInput) (DescribeAttachmentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `DescribeAttachmentInput` | 封闭业务输入；逐字段见types $defs/DescribeAttachmentInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DescribeAttachmentResult` | 成功结果；逐字段见types $defs/DescribeAttachmentResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 核精确原资产/版本授权下载与hash，文本MIME读取完整UTF8正文且≤65536字符，调用knowledge.describe fast800输出，不截断
2. 受控原生PDF文件分支使用kind=file/file AssetRef、vision能力及knowledge.extract-attachment1500输出，由MG核file+vision及真实原版bytes/供应方adapter；其他未声明MIME明确unsupported
3. 普通PDF无文字仍unsupported不自动OCR；本函数原生PDF分支只用于另一个已批准的原生附件任务，Memory.TaskID必须精确匹配，不能用普通PDF提取记忆绕过
4. 本域只组装任务与来源、原提取拥有者幂等键，模型机制/费用/备用全由平台与MG，结果机器派生且原调用保留，跨构建提取缓存归创建/执行器

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNSUPPORTED_CAPABILITY` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG可能收费及派生文本，不修改或翻译附件本体
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`knowledge.SameAsset`、`knowledge.DownloadExtractionAsset`、`knowledge.DescribeResource`

## knowledge.RunWorker

- 级别：约束
- 文件：`modules/knowledge/internal/runtime/worker.go`
- 作用：主动领取本域持久任务

```go
func RunWorker(ctx context.Context, op OperationContext,input RunWorkerInput)(RunWorkerResult,error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RunWorkerInput` | 封闭业务输入；逐字段见types $defs/RunWorkerInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RunWorkerResult` | 成功结果；逐字段见types $defs/RunWorkerResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 两个平台RunWorkerLoopWithResult分别按knowledge.build并发1与knowledge.extract并发3领取，jobs权威可选jobTypes筛选，总并发固定4；维护暂停由平台统一拥有
2. 平台Start后首次真实Heartbeat及每次续约回调MirrorExecutionLease；AdoptClaimedJob在执行前保存完整原claim，执行保留原批准root/op/deadline
3. OnCancellationObserved持久原cancel事实与写资格关闭，不CheckLease递归、不本地伪造心跳；ExecuteBuild/ExecuteExtraction使用实际registry
4. 真实jobs回执累计四种结果并等待两个循环退出；只取消协作ctx，不丢仍在途任务/费用事实

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED`：网关额度不足（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）

- 副作用：jobs租约/完成状态及本域工作链副作用
- 授权：permission：service；rule：仅knowledge纯后台服务；input.workerId只兼容标签，实际worker由平台生成
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`platform.RunWorkerLoopWithResult`、`knowledge.ResolveClaimedJobOperation`、`knowledge.MirrorExecutionLease`、`knowledge.RecordExecutionCancellation`、`knowledge.ExecuteBuild`、`knowledge.ExecuteExtraction`

## knowledge.NewWorkerRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/runtime/worker.go`
- 作用：真实jobs客户端与六个严格生产回执校验器，注入真实执行registry

```go
func NewWorkerRuntime(raw *gen.JobsClient,executors platform.JobExecutorRegistry,runtime func(context.Context)(platform.RuntimeView,error))(WorkerRuntime,error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实jobs客户端与六个严格生产回执校验器，注入真实执行registry

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`platform.NewPayloadValidator`、`platform.NewJobsClient`、`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`

## knowledge.WithWorkerRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/runtime/worker.go`
- 作用：绑定请求独立本域worker依赖

```go
func WithWorkerRuntime(ctx context.Context,runtime WorkerRuntime)context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 绑定请求独立本域worker依赖

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：无

## knowledge.main

- 级别：约束
- 文件：`modules/knowledge/cmd/knowledge/main.go`
- 作用：独立进程可执行入口

```go
func main()
```

**输入**

无

**输出**

无

**步骤**

1. 在任何配置/密钥/DB加载前调用RunPDFParserChild；handled直接exit0/1，不记录PDF或详细错误
2. 普通启动只装配本域Module/BuildApplication/RuntimeClient参数并调用platform.RunService统一配置/信号/后台/Drain/Shutdown/恢复重启
3. 完整app未齐时由真实handler/worker缺失门禁拒绝启动，不暴露假业务服务

**错误**

- `VALIDATION` / HTTP 400：配置未知字段/越界目录/非法端点（不可重试）
- `INTERNAL` / HTTP 500：装配或HTTP运行失败（不可重试）

- 副作用：打开本域资源、监听HTTP；关闭按持久安全点
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：一个进程一个Application；OS信号处理只触发一次drain
- 幂等：进程重启先journal恢复；不重放已付费任务
- 依赖：`knowledge.RunPDFParserChild`、`knowledge.BuildApplication`、`knowledge.NewRuntimeClient`、`platform.RunService`、`knowledge.Serve`、`knowledge.Shutdown`

## knowledge.BuildApplication

- 级别：约束
- 文件：`modules/knowledge/internal/app/wire.go`
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
4. 读取runtime epoch/maintenance；维护期间只开放健康/就绪与控制管理接口、不启动worker；本次启动完成换目录或换回时调用site-config:registerActivation登记回执；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源
6. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`platform.LoadServiceIdentity`、`platform.NewSiteConfigClient`、`platform.ReadRuntime`、`platform.ApplyRestoreSwap`、`platform.NewPayloadValidator`、`platform.NewIdentityAuthorizer`、`knowledge.OpenRepository`、`knowledge.NewIndexRuntime`、`knowledge.NewModelRuntime`、`knowledge.LoadRuntime`、`knowledge.WithIndexRuntime`、`knowledge.WithModelRuntime`、`knowledge.WithAdminSigningKey`、`knowledge.bindSearchRuntime`、`knowledge.assembleBackgroundTasks`、`knowledge.serviceOperation`、`knowledge.NewApplication`、`site-config:readSiteConfig`、`identity:checkAuthorization`、`knowledge.NewWorkerRuntime`、`knowledge.WithWorkerRuntime`、`knowledge.ExecuteBuild`、`knowledge.ExecuteExtraction`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`、`knowledge.RecoverBuildLeases`、`knowledge.CollectRetiredIndexes`、`knowledge.PublishOutbox`、`knowledge.ConsumeEvents`、`knowledge.RunWorker`、`platform.TaskGroup.SetAdmission`、`platform.TaskGroup.CancelActive`、`platform.TaskGroup.WaitIdle`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## knowledge.NewRuntimeClient

- 级别：参考
- 文件：`modules/knowledge/internal/app/wire.go`
- 作用：统一身份加载与运行权威客户端参数装配

```go
func NewRuntimeClient(config platform.ServiceConfig) (platform.SiteConfigClient, error)
```

**输入**

无

**输出**

无

**步骤**

1. 统一身份加载与运行权威客户端参数装配

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`platform.LoadServiceIdentity`、`platform.NewSiteConfigClient`

## knowledge.serviceOperation

- 级别：参考
- 文件：`modules/knowledge/internal/app/wire.go`
- 作用：本域服务控制读/后台批次当前请求元数据，不用于重写已批准任务root或费用

```go
func serviceOperation(ctx context.Context, siteID, epoch string) platform.OperationContext
```

**输入**

无

**输出**

无

**步骤**

1. 本域服务控制读/后台批次当前请求元数据，不用于重写已批准任务root或费用

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.bindSearchRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/app/wire.go`
- 作用：每请求读取实际私有配置，无配置不虚构空间，配置启用后装配当前检索上下文

```go
func bindSearchRuntime(ctx context.Context, db *sql.DB, indexes indexer.Runtime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 每请求读取实际私有配置，无配置不虚构空间，配置启用后装配当前检索上下文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.ReadConfig`、`knowledge.NewSearchRuntime`、`knowledge.WithSearchRuntime`

## knowledge.assembleBackgroundTasks

- 级别：参考
- 文件：`modules/knowledge/internal/app/wire.go`
- 作用：真实已有事件/转发/原构建恢复/10分钟GC装配，缺worker导致完整容器拒绝启动

```go
func assembleBackgroundTasks(bind func(context.Context) context.Context, siteID, epoch string, window platform.WriteWindowConfig) []platform.BackgroundTask
```

**输入**

无

**输出**

无

**步骤**

1. 真实已有事件/转发/原构建恢复/10分钟GC装配，缺worker导致完整容器拒绝启动

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`platform.RunRecoveryLoop`、`knowledge.serviceOperation`、`knowledge.PublishOutbox`、`knowledge.RecoverBuildLeases`、`knowledge.CollectRetiredIndexes`、`knowledge.ConsumeEvents`、`knowledge.RunWorker`、`knowledge.RecoverExtractionTasks`

## knowledge.Serve

- 级别：约束
- 文件：`modules/knowledge/internal/transport/server.go`
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

1. 只有完整prepared容器允许单次监听，net.ListenConfig继承ctx，HTTP请求继承进程取消
2. HTTP drain与后台等待由platform.RunService协调，Serve不抢先关闭私有DB；ErrServerClosed正常结束

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：无
- 其他依赖说明：本域OpenAPI路由

## knowledge.Shutdown

- 级别：约束
- 文件：`modules/knowledge/internal/transport/server.go`
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

1. platform先停止后台并Drain等待请求，再同次关闭私有DB
2. 关闭错误保留原安全失败并重复返回，不提前清理数据或journal

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：30秒期限仍有未安全在途步骤（可重试）
- `INTERNAL` / HTTP 500：flush/close失败（不可重试）

- 副作用：本域drain、资源关闭、durable恢复日志
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：sync.Once启动drain；其他调用等待同一次结果
- 幂等：重复Shutdown返回同一次关闭结果；已closed成功
- 依赖：无
- 其他依赖说明：本域barrier安全点函数；本域journal/outbox flush

## knowledge.Health

- 级别：约束
- 文件：`modules/knowledge/internal/transport/server.go`
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

1. 匿名只报knowledge版本、存活/排空与基础管理能力，不虚构模型或数据库业务成功

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：进程已closed或服务循环不可用（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：无锁或原子读取lifecycle；不进行外部调用
- 幂等：GET只读重复安全
- 依赖：无

## knowledge.Ready

- 级别：约束
- 文件：`modules/knowledge/internal/transport/server.go`
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

1. 读取当前runtime和实际DB可用性，核opened epoch与prepared/draining，完整未装配拒就绪
2. 公开允许严格要求非维护/running，同站已初始化与LoadRuntime完成才组装容器；真实备份safePoint控制未装配保持false，Ready仍partial

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：`knowledge.Health`、`platform.ReadRuntime`
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## knowledge.NewOperationHandlers

- 级别：参考
- 文件：`modules/knowledge/internal/transport/server.go`
- 作用：NewOperationHandlers的本域内部实现助手

```go
func NewOperationHandlers(bind func(context.Context)context.Context) map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释
3. CreateBuild实际绑定HTTP202，其他已完成操作保留其准确200；统一错误仍由公共Problem处理
4. CancelBuild显式核路径与正文同buildId后调用真实取消用例

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.jsonOperation`、`knowledge.operationContext`、`knowledge.writeResult`、`knowledge.Retrieve`、`knowledge.ReadSections`、`knowledge.CreateQueryEmbedding`、`knowledge.GetBuild`、`knowledge.ListBuilds`、`knowledge.RedactDiagnostics`、`knowledge.PutIndexConfig`、`knowledge.ReadIndexConfig`、`knowledge.CreateBuild`、`knowledge.CancelBuild`、`knowledge.ReadJobRecovery`、`knowledge.ReadIndexOptions`、`knowledge.EstimateRebuildPlan`、`knowledge.ApproveRebuildPlan`

## knowledge.jsonOperation

- 级别：参考
- 文件：`modules/knowledge/internal/transport/server.go`
- 作用：jsonOperation的本域内部实现助手

```go
func jsonOperation[I, R any](bind func(context.Context) context.Context, run func(context.Context, OperationContext, I) (R, error), status int) http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.operationContext`、`knowledge.writeResult`

## knowledge.operationContext

- 级别：参考
- 文件：`modules/knowledge/internal/transport/server.go`
- 作用：operationContext的本域内部实现助手

```go
func operationContext(r *http.Request,bind func(context.Context)context.Context)(context.Context,platform.OperationContext,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.writeResult

- 级别：参考
- 文件：`modules/knowledge/internal/transport/server.go`
- 作用：writeResult的本域内部实现助手

```go
func writeResult(w http.ResponseWriter, op OperationContext, result any, err error, status int)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.NewApplication

- 级别：参考
- 文件：`modules/knowledge/internal/transport/server.go`
- 作用：完整HTTP合同/固定caller策略/必要后台入口门禁，缺任一真实handler或worker拒绝容器

```go
func NewApplication(config platform.ServiceConfig, anchors platform.TrustAnchors, authority platform.SiteConfigClient, epoch string, db *sql.DB, bind func(context.Context) context.Context, authorize func(context.Context, platform.OperationContext, string, string) error, tasks []platform.BackgroundTask) (*Application, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整HTTP合同/固定caller策略/必要后台入口门禁，缺任一真实handler或worker拒绝容器

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.NewOperationHandlers`、`knowledge.Health`、`knowledge.Ready`、`platform.NewServiceHandler`、`platform.ReadRuntime`

## knowledge.InvokeModel

- 级别：参考
- 文件：`modules/knowledge/internal/model/client.go`
- 作用：经模型网关执行一次调用并读取到终态

```go
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `InvokeModelInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InvokeModelResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 本域先持久原请求规范摘要/原op/key，完整已保存终态原样复用，无论成功/失败/费用未知都不另建调用
2. 公共InvokeModel真实调用与续读机制不复制；强制内部事件回调先保存真实invocationId再转可选UI事件，授权许可不保存
3. 返回前完整终态及hash持久；网络/取消/崩溃未知保留dispatching/running及原key，后续仅原MG幂等键找回，不换操作或重扣
4. 内部非序列化SemanticKey来自原chunk/batch/资源阶段身份，不能包含可变配置本身；原MG key/op在本域持久，Root内独立QA查询以规范查询身份区分
5. 平台返回错误也保留实际InvocationID/CurrentSnapshot；短控制窗口只保存原观察；已保留完整结果不因新观察缺失而丢失，原费用未知仍由MG对账
6. 本域原意图先prepared（dispatch_started=0），真实租约门禁后、平台RPC前原子转dispatching/1；旧记录默认1保守unknown，不能用无InvocationID推零费用
7. 原全量明批ModelExpectation按task注入expectedRouteVersion/expectedPriceSnapshots，并纳入稳定原请求hash/去凭据DTO；任一版本漂移MG原子拒绝，不自动重估或新owner

**错误**

- `BUDGET_EXCEEDED`：网关额度不足（不可重试）
- `CANCELLED`：调用被取消（不可重试）
- `DEPENDENCY_UNAVAILABLE`：网关不可用或调用失败（可重试）
- `UNSUPPORTED_CAPABILITY`：任务未配置模型（不可重试）

- 副作用：HTTP调用model-gateway推理接口；模型费用由网关记账
- 授权：permission：service；rule：以本服务身份调用网关，费用类别继承可信op
- 并发：每次调用独立；同一根操作的子调用共享网关费用作用域
- 幂等：幂等键固定，重试返回原invocation
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`、`platform.InvokeModel`、`knowledge.BeginModelStep`、`knowledge.SaveModelProgress`、`knowledge.RecordModelObservation`、`knowledge.HasCompleteResult`、`knowledge.MarkModelDispatch`

## knowledge.NewModelRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/model/client.go`
- 作用：NewModelRuntime的本域内部实现助手

```go
func NewModelRuntime(raw *gen.ModelGatewayClient, db *sql.DB, window platform.WriteWindowConfig) (ModelRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释
3. NewGatewayClient严格生产校验器五项：CreateInvocationResult/CancelInvocationResult/StreamEvent/GetInvocationResult/LookupInvocationResult；原key只读观察经公共客户端，不复制HTTP/SSE机制
4. 严格GatewayQueryUsageResult生产schema用于恢复完整根费用终局证明

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`platform.NewGatewayClient`、`platform.NewPayloadValidator`、`model-gateway:readInference`、`model-gateway:lookupInference`

## knowledge.WithModelRuntime

- 级别：参考
- 文件：`modules/knowledge/internal/model/client.go`
- 作用：WithModelRuntime的本域内部实现助手

```go
func WithModelRuntime(ctx context.Context,runtime ModelRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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

## knowledge.CompletedOutput

- 级别：参考
- 文件：`modules/knowledge/internal/model/client.go`
- 作用：CompletedOutput的本域内部实现助手

```go
func CompletedOutput(outcome InvokeModelResult, capability string) (*platform.ModelOutput,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释
3. 优先读CurrentSnapshot实际完整结构Output并核能力；无当前Output才用原真实completed事件，不拼事件、不复制旧unknown

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.HasCompleteResult`

## knowledge.BindAdmissionPermit

- 级别：参考
- 文件：`modules/knowledge/internal/model/client.go`
- 作用：绑定本次已消费准入至独立模型上下文

```go
func BindAdmissionPermit(ctx context.Context, op OperationContext, permit *gen.AdmissionPermit) (context.Context, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅复制本次模型运行上下文，不修改共享runtime；问答分支核对非空已消费token、原访客/root/epoch和动作；网关负责签名与有效期权威验证。

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`knowledge.WithModelRuntime`

## knowledge.WithModelLease

- 级别：参考
- 文件：`modules/knowledge/internal/model/client.go`
- 作用：付费前绑定真实jobs租约检查，不复制续约机制

```go
func WithModelLease(ctx context.Context,check func(context.Context)error)(context.Context,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按原scope/完整持久输入组合真实已有机制，不换原key或批准

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域后台服务组合调用
- 并发：本站同代真实原拥有者与安全窗口
- 幂等：只用原输入/原key/原取消意图
- 依赖：`knowledge.WithModelRuntime`

## knowledge.ExecuteBuild

- 级别：约束
- 文件：`modules/knowledge/internal/build/execute.go`
- 作用：执行一次文档索引构建

```go
func ExecuteBuild(ctx context.Context, op OperationContext,input JobExecutionInput)(result JobExecutionResult,failure error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `JobExecutionInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `JobExecutionResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 精确原job/ref/root/worker/fence读已批准完整BuildJobInput；已完成只返回原持久结果；付费前与最终写前使用平台CheckLease
2. FetchPublishedSource核完整固定源；整篇超过固定context预算失败关闭，不截断、不降级；真实提取独立任务只等待同原ID
3. 完整raw chunks、首次真实task memory、增强chunks与固定单块embedding batches各阶段只冻结一次；背景语义用原整组ID，配置漂移不能绕原unknown
4. 原文字/offsets不改，机器context单独保存；同完整retrieval text/space真实向量复用，未命中按固定原单块批次付费
5. 完整词项/向量/hash/一秒content权威证明后ActivateBuild，active/outbox/最终原JobExecutionResult同事务；jobs回执未知后只重交原真实结果
6. 错误边界持久当前原fence真实failure_code/state并保完整已知paid refs；如果最终业务已原子完成则只重交真实成功结果，不覆盖；控制元数据窗口不延长原批准

**错误**

- `STALE_RESULT`：源修订已被替代（不可重试）
- `DEPENDENCY_UNAVAILABLE`：content/media/网关不可用（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED`：网关额度不足（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写staging与active索引；模型费用经网关
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：同document/space一次只运行一个build；所有写入核lease与epoch
- 幂等：按document/revision/space/generation唯一
- 依赖：`knowledge.ReadExecutionRecord`、`knowledge.WithModelLease`、`knowledge.ExecutionLeaseID`、`knowledge.SetBuildStage`、`knowledge.FetchPublishedSource`、`knowledge.RuntimeFromContext`、`knowledge.ReadBuildPlan`、`knowledge.FreezeBuildPlan`、`knowledge.ReadModelCatalog`、`knowledge.EmbeddingProfile`、`knowledge.PrepareBuildChunks`、`knowledge.ResolveTaskMemory`、`knowledge.EnrichChunkContext`、`knowledge.ApplyChunkContext`、`knowledge.WriteStagingChunks`、`knowledge.BuildLexicalIndex`、`knowledge.PrepareEmbeddingPlan`、`knowledge.ReuseEmbeddingBatch`、`knowledge.RequestEmbeddings`、`knowledge.StoreEmbeddings`、`knowledge.ValidateBuild`、`knowledge.ActivateBuild`、`knowledge.CheckEmbeddingPlan`、`knowledge.FinishExecutionFailure`、`knowledge.CheckRebuildApproval`、`knowledge.WithModelExpectations`

## knowledge.PrepareBuildChunks

- 级别：参考
- 文件：`modules/knowledge/internal/build/execute.go`
- 作用：固定正文续读全部真实叶及真实原提取文字；独立子任务等待不inline、不另付费

```go
func PrepareBuildChunks(ctx context.Context,op OperationContext,payload BuildJobInput,content source.FetchPublishedSourceResult,leaseID,modelID string)([]gen.Chunk,error)
```

**输入**

无

**输出**

无

**步骤**

1. 固定正文续读全部真实叶及真实原提取文字；独立子任务等待不inline、不另付费
2. 本地failed仅当前attempt失败，锁外查jobs原任务；queued/leased/running/retry-wait/cancel-requested/orphan等待原ID，只有dead-letter/cancelled终止，succeeded必须回读真实ready/unsupported结果

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.BuildSourceBlocks`、`knowledge.ListExtractableAssets`、`knowledge.CreateExtraction`、`knowledge.ReadExtraction`、`knowledge.BindExtractionViews`、`knowledge.BuildMaterialBlocks`、`knowledge.SplitChunks`、`knowledge.ReadExtractionJobState`、`knowledge.ReadBuildPlan`

## knowledge.EstimateRebuildPlan

- 级别：约束
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：全量重建前估算费用

```go
func EstimateRebuildPlan(ctx context.Context, op OperationContext, input EstimateRebuildPlanInput) (EstimateRebuildPlanResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `EstimateRebuildPlanInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EstimateRebuildPlanResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 即时knowledge.manage；免费读取完整当前公开树及固定正文/hash/真实BPE，源/策略16MiB、256文档/4096真实分块预算超限失败不截断
2. 冻结完整空间/主备用覆盖候选/任务记忆；核真实资源、背景和向量缓存后扣实际复用；未产资源文字明确chunkCountKnown=false
3. 逐真实模型固定输入窗口保守QuoteCost，真实整数分项汇总；缺价或完整数量未知则pricingKnown=false、省maximumCost并给warning，不以0代替
4. 不调用模型/不建资源或paid意图；当前配置版本CAS后保存固定来源集合、真实策略/报价hash和24小时有效期，原operation回放原结果

**错误**

- `FORBIDDEN`：权限不足（不可重试）
- `DEPENDENCY_UNAVAILABLE`：依赖不可用（可重试）

- 副作用：写重建计划
- 授权：permission：knowledge.manage；rule：后台管理员按登记权限
- 并发：只读快照或CAS
- 幂等：读取幂等；写入按operationId
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.RuntimeFromContext`、`knowledge.PrepareRebuildSnapshot`、`knowledge.BeginIndexWrite`、`knowledge.ReadConfig`

## knowledge.ApproveRebuildPlan

- 级别：约束
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：批准估价后创建全量重建任务

```go
func ApproveRebuildPlan(ctx context.Context, op OperationContext, input ApproveRebuildPlanInput) (ApproveRebuildPlanResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ApproveRebuildPlanInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApproveRebuildPlanResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 即时knowledge.manage和可信background，核本站同代原管理员、未过期计划与quoteHash；已批准同输入只回原回执
2. 锁外重新核完整源/配置/模型/记忆/报价/缓存集合不漂移，当前配置及原批准状态再CAS
3. 一个安全写事务中为完整原集合QueueBuild(force=true)，保存各构建原明批策略、批准人/root/时点和完整任务ID回执；任一失败全部回滚
4. 事务后只派发已提交原任务，未知保持久意图恢复；旧active保留至新代数真正核验/激活，不自动新批准

**错误**

- `VALIDATION`：计划过期或不符（不可重试）
- `CONFLICT`：已批准（不可重试）

- 副作用：创建构建任务
- 授权：permission：knowledge.manage；rule：后台管理员按登记权限
- 并发：只读快照或CAS
- 幂等：读取幂等；写入按operationId
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.RuntimeFromContext`、`knowledge.PrepareRebuildSnapshot`、`knowledge.BeginIndexWrite`、`knowledge.ReadConfig`、`knowledge.QueueBuild`、`knowledge.DispatchBuild`

## knowledge.ReadRebuildDocuments

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：严格公开树生产schema取完整真实发布修订/树水位，最多256文档超预算失败不截断

```go
func ReadRebuildDocuments(ctx context.Context,op OperationContext,runtime indexer.Runtime)([]gen.PublishedRevisionRef,int64,error)
```

**输入**

无

**输出**

无

**步骤**

1. 严格公开树生产schema取完整真实发布修订/树水位，最多256文档超预算失败不截断

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`content:readPublicTree`

## knowledge.PrepareRebuildPolicy

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：冻结原完整空间、嵌入/上下文/各资源任务记忆和真实声明策略，估价不创建模型或资源意图

```go
func PrepareRebuildPolicy(ctx context.Context,op OperationContext,runtime indexer.Runtime,config gen.IndexConfig,resources []source.AssetSource)(RebuildPolicy,error)
```

**输入**

无

**输出**

无

**步骤**

1. 嵌入、上下文、资源均以MG免费resolveTaskRoute真实eligible候选/routeVersion为准，不手拼主备用或任务覆盖；嵌入档位及每候选完整空间必须匹配配置
2. 冻结真实任务记忆与完整候选opaque快照；估价不建任何paid/资源意图

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`knowledge.ReadTaskRoute`、`knowledge.ResolveTaskMemory`、`knowledge.PrepareExtractionPlan`

## knowledge.CheckRebuildApproval

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：每次可能付费前重核原明批模型/任务/记忆；当前漂移CONFLICT，不换key或延批准，正常单篇无全量记录

```go
func CheckRebuildApproval(ctx context.Context,op OperationContext,buildID string,config gen.IndexConfig)error
```

**输入**

无

**输出**

无

**步骤**

1. 每次可能付费前重核原明批模型/任务/记忆；当前漂移CONFLICT，不换key或延批准，正常单篇无全量记录
2. 客户端重读只诊断策略漂移；真正付费Create必须携原expectedRouteVersion和逐候选expectedPriceSnapshots，MG事务内整路由/价格CAS，包含disabled→enabled变化；绝不拿客户端预读冒原子CAS

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`knowledge.ReadBuildPlan`、`knowledge.RuntimeFromContext`、`knowledge.PrepareRebuildPolicy`

## knowledge.AddRebuildQuoteRoute

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：按真实固定模型窗口与已知数量聚合各任务候选，最多256完整分项且整数溢出失败关闭

```go
func AddRebuildQuoteRoute(components map[string]*RebuildQuoteComponent,route RebuildRoute,quantity,output int64)error
```

**输入**

无

**输出**

无

**步骤**

1. 按真实固定模型窗口与已知数量聚合各任务候选，最多256完整分项且整数溢出失败关闭

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：无

## knowledge.QuoteRebuildComponents

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：严格MG报价生产schema逐真实模型报价并整数汇总；任一缺价或未来材料未知则总pricingKnown=false且省maximumCost，保全部真实分项

```go
func QuoteRebuildComponents(ctx context.Context,op OperationContext,runtime indexer.Runtime,components map[string]*RebuildQuoteComponent,countKnown bool)(RebuildQuote,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只消费本次已冻结的真实逐候选QuoteCost回执，严格生产schema核对后做整数求和；缺价或完整材料数量未知则pricingKnown=false且省最大费，不构造单一PriceSnapshot

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：无

## knowledge.PrepareRebuildSnapshot

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：免费固定正文/BPE/已付费背景和向量缓存，已知材料扣真实复用；未产资源文字chunkCountKnown=false且总上限未知，不调AI凑价；完整源/策略16MiB、真实分块4096预算不截断

```go
func PrepareRebuildSnapshot(ctx context.Context,op OperationContext,reason string)(RebuildSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 免费固定正文/BPE/已付费背景和向量缓存，已知材料扣真实复用；未产资源文字chunkCountKnown=false且总上限未知，不调AI凑价；完整源/策略16MiB、真实分块4096预算不截断

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadConfig`、`knowledge.ReadRebuildDocuments`、`knowledge.ReadRebuildSource`、`knowledge.ListExtractableAssets`、`knowledge.PrepareRebuildPolicy`、`knowledge.BuildSourceBlocks`、`knowledge.ReadRebuildExtractionCache`、`knowledge.BuildMaterialBlocks`、`knowledge.SplitChunks`、`knowledge.ReadRebuildCachedContext`、`knowledge.HasRebuildVector`、`knowledge.AddRebuildQuoteRoute`、`knowledge.QuoteRebuildComponents`、`knowledge.FreezeRebuildRoutePrices`

## knowledge.ReadRebuildSource

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：严格真实固定发布正文16MiB/hash，必需整篇上下文超过65536字符失败关闭不截断

```go
func ReadRebuildSource(ctx context.Context,op OperationContext,runtime indexer.Runtime,document gen.PublishedRevisionRef)([]byte,error)
```

**输入**

无

**输出**

无

**步骤**

1. 严格真实固定发布正文16MiB/hash，必需整篇上下文超过65536字符失败关闭不截断

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`content:readPublishedRevision`

## knowledge.FreezeRebuildRoutePrices

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild.go`
- 作用：逐权威eligible候选免费QuoteCost、核实际PriceSnapshot模型ID，冻结expectedPriceSnapshots及完整原报价；未来材料未知仍不虚构数量或总价

```go
func FreezeRebuildRoutePrices(ctx context.Context,op OperationContext,runtime indexer.Runtime,route *RebuildRoute,output int64)error
```

**输入**

无

**输出**

无

**步骤**

1. 逐权威eligible候选免费QuoteCost、核实际PriceSnapshot模型ID，冻结expectedPriceSnapshots及完整原报价；未来材料未知仍不虚构数量或总价

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只消费原已授权任务权威声明，不扩模型付费批准
- 并发：锁外读取真实权威；原scope/CAS写入，付费接受路由/价格原子CAS由MG拥有
- 幂等：原hash/key/op/root/route和价格快照保留，未知不换owner
- 依赖：`model-gateway:quoteCost`

## knowledge.OpenRepository

- 级别：参考
- 文件：`modules/knowledge/internal/index/repository.go`
- 作用：按本域active私有路径打开SQLite，执行内嵌只追加迁移和平台inbox/outbox建表，旧身份不重标

```go
func OpenRepository(ctx context.Context, dataRoot, site, epoch string) (*sql.DB, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按本域active私有路径打开SQLite，执行内嵌只追加迁移和平台inbox/outbox建表，旧身份不重标

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域装配或已授权用例内部调用，继承外层权限/事务
- 并发：复用本域短事务；外部权威读取不持有SQL事务
- 幂等：同原记录及规范输入约束
- 依赖：`knowledge.storageError`

## knowledge.BeginIndexWrite

- 级别：参考
- 文件：`modules/knowledge/internal/index/repository.go`
- 作用：SQL事务前取得平台当前写窗口，核对本域站点/代次

```go
func BeginIndexWrite(ctx context.Context, op OperationContext) (context.Context, *sql.Tx, context.CancelFunc, error)
```

**输入**

无

**输出**

无

**步骤**

1. SQL事务前取得平台当前写窗口，核对本域站点/代次

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域装配或已授权用例内部调用，继承外层权限/事务
- 并发：复用本域短事务；外部权威读取不持有SQL事务
- 幂等：同原记录及规范输入约束
- 依赖：`knowledge.RuntimeFromContext`、`platform.NewWriteContext`、`knowledge.storageError`

## knowledge.ReadConfig

- 级别：参考
- 文件：`modules/knowledge/internal/index/repository.go`
- 作用：读取同代私有配置，缺失NOT_FOUND而非伪默认

```go
func ReadConfig(ctx context.Context, tx *sql.Tx, op OperationContext) (gen.IndexConfig, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取同代私有配置，缺失NOT_FOUND而非伪默认

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域装配或已授权用例内部调用，继承外层权限/事务
- 并发：复用本域短事务；外部权威读取不持有SQL事务
- 幂等：同原记录及规范输入约束
- 依赖：`knowledge.storageError`

## knowledge.ReadOperationReceipt

- 级别：参考
- 文件：`modules/knowledge/internal/index/repository.go`
- 作用：按站点/代次/管理主体/函数/键读取安全原回执并核规范输入hash

```go
func ReadOperationReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, function, key string, input any, result any) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按站点/代次/管理主体/函数/键读取安全原回执并核规范输入hash

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域装配或已授权用例内部调用，继承外层权限/事务
- 并发：复用本域短事务；外部权威读取不持有SQL事务
- 幂等：同原记录及规范输入约束
- 依赖：`knowledge.storageError`

## knowledge.SaveOperationReceipt

- 级别：参考
- 文件：`modules/knowledge/internal/index/repository.go`
- 作用：与本域业务事务一起保存规范输入hash及安全结果

```go
func SaveOperationReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, function, key string, input, result any) error
```

**输入**

无

**输出**

无

**步骤**

1. 与本域业务事务一起保存规范输入hash及安全结果

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：可信本域装配或已授权用例内部调用，继承外层权限/事务
- 并发：复用本域短事务；外部权威读取不持有SQL事务
- 幂等：同原记录及规范输入约束
- 依赖：`knowledge.storageError`

## knowledge.EnsureBuildDiskBudget

- 级别：参考
- 文件：`modules/knowledge/internal/index/repository.go`
- 作用：实施本域已确认10%低水位，仅阻止新代数，不影响原结果读取或复用

```go
func EnsureBuildDiskBudget(dataRoot string) error
```

**输入**

无

**输出**

无

**步骤**

1. 对可信私有本地目录读取真实Statfs可用块，未知或≤10%拒绝新build，保留active

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只由已授权创建链路传可信Runtime.DataRoot
- 并发：只读文件系统水位，不另造预占账本
- 幂等：同真实水位同决定
- 依赖：无

## knowledge.ConsumeInbox

- 级别：参考
- 文件：`modules/knowledge/internal/index/inbox.go`
- 作用：事件去重与代际保护

```go
func ConsumeInbox(ctx context.Context, op OperationContext, input ConsumeInboxInput, tx *sql.Tx) (ConsumeInboxResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ConsumeInboxInput` | 封闭业务输入；逐字段见types $defs/ConsumeInboxInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeInboxResult` | 成功结果；逐字段见types $defs/ConsumeInboxResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只接公共订阅边界核验的原EventEnvelope，事务首项写入调用ClaimInboxBeforeWrite并提供真实tx.Commit回调
2. 同ID异hash只提交隔离且返回原完整性错误；已隔离始终拒绝；正常inbox和业务状态由外层事务共同提交
3. 源水位按producer/eventType/eventVersion/subject流单调；公共订阅只在handler成功提交后确认

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写inbox
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`platform.ClaimInboxBeforeWrite`

## knowledge.CancelBuildSideEffects

- 级别：参考
- 文件：`modules/knowledge/internal/index/cancellation.go`
- 作用：按已持久取消意图及原root请求已知jobs/MG停止，未知接受不假成功，不退款不丢引用

```go
func CancelBuildSideEffects(ctx context.Context, op OperationContext, buildID string) error
```

**输入**

无

**输出**

无

**步骤**

1. 按已持久取消意图及原root请求已知jobs/MG停止，未知接受不假成功，不退款不丢引用

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只由本域已授权取消或原撤回事件组合调用，原root由持久记录核归属
- 并发：SQL外调用jobs与公共网关；回写前重新取得当前安全窗口
- 幂等：原build/job/invocation和固定取消键，不重派模型
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadBuildRecord`、`knowledge.BeginIndexWrite`、`knowledge.storageError`、`jobs:cancelJob`、`model-gateway:cancelInference`、`knowledge.CancelExtractionSideEffects`

## knowledge.CancelDocumentSideEffects

- 级别：参考
- 文件：`modules/knowledge/internal/index/cancellation.go`
- 作用：只扫描本域同文档已持久取消意图，重放旧撤回不取消新构建

```go
func CancelDocumentSideEffects(ctx context.Context, op OperationContext, documentID string) error
```

**输入**

无

**输出**

无

**步骤**

1. 只扫描本域同文档已持久取消意图，重放旧撤回不取消新构建

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只由本域已授权取消或原撤回事件组合调用，原root由持久记录核归属
- 并发：SQL外调用jobs与公共网关；回写前重新取得当前安全窗口
- 幂等：原build/job/invocation和固定取消键，不重派模型
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.CancelBuildSideEffects`、`knowledge.storageError`

## knowledge.ReadIndexOptions

- 级别：约束
- 文件：`modules/knowledge/internal/admin/options.go`
- 作用：读取已有索引管理流程的真实安全选项

```go
func ReadIndexOptions(ctx context.Context, op OperationContext, input ReadIndexOptionsInput) (ReadIndexOptionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ReadIndexOptionsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadIndexOptionsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. knowledge.manage读取可信基准语言、MG纯服务无密钥目录及content当前公开树
2. 分别返回真实ready/unavailable状态与安全错误码，失败不伪装为空目录
3. embedding选项只声明完整权威space及精确支持tokenizer，不构造ID/hash；文档返回真实固定发布引用及可见性修订

**错误**

- `FORBIDDEN`：权限不足（不可重试）

- 副作用：无
- 授权：permission：knowledge.manage；rule：后台管理员
- 并发：只读快照
- 幂等：只读
- 依赖：`knowledge.AuthorizeAdmin`、`knowledge.RuntimeFromContext`、`knowledge.ReadModelCatalog`、`knowledge.optionErrorCode`、`content:readPublicTree`

## knowledge.optionErrorCode

- 级别：参考
- 文件：`modules/knowledge/internal/admin/options.go`
- 作用：只返回公共安全错误码，不透传跨域详情

```go
func optionErrorCode(err error) string
```

**输入**

无

**输出**

无

**步骤**

1. 只返回公共安全错误码，不透传跨域详情

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.ReadModelCatalog

- 级别：参考
- 文件：`modules/knowledge/internal/index/catalog.go`
- 作用：以本域纯服务读严格生产schema目录，不计算空间hash

```go
func ReadModelCatalog(ctx context.Context, op OperationContext, runtime Runtime) (ModelCatalog, error)
```

**输入**

无

**输出**

无

**步骤**

1. 以本域纯服务读严格生产schema目录，不计算空间hash

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`model-gateway:readModels`、`model-gateway:readProfiles`

## knowledge.EmbeddingProfile

- 级别：参考
- 文件：`modules/knowledge/internal/index/catalog.go`
- 作用：按可信费用类别/embedding主模型/完整五字段空间选择档位，不跨空间回退

```go
func EmbeddingProfile(catalog ModelCatalog, profileID, costClass string, space gen.EmbeddingSpace) (ProfileDeclaration, ModelDeclaration, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按可信费用类别/embedding主模型/完整五字段空间选择档位，不跨空间回退

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.TextProfile

- 级别：参考
- 文件：`modules/knowledge/internal/index/catalog.go`
- 作用：首个具备所需能力的配置声明预检查，不作为实际winning证明

```go
func TextProfile(catalog ModelCatalog, tier, costClass, capability string) (ProfileDeclaration, ModelDeclaration, error)
```

**输入**

无

**输出**

无

**步骤**

1. 首个具备所需能力的配置声明预检查，不作为实际winning证明

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.ReadTaskAssignment

- 级别：参考
- 文件：`modules/knowledge/internal/index/catalog.go`
- 作用：纯服务只读本域任务的严格生产schema覆盖声明，空列表为未设置，不伪造记录

```go
func ReadTaskAssignment(ctx context.Context, op OperationContext, runtime Runtime, taskID string) (*TaskAssignment, error)
```

**输入**

无

**输出**

无

**步骤**

1. 纯服务只读本域任务的严格生产schema覆盖声明，空列表为未设置，不伪造记录

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`model-gateway:readTaskAssignments`

## knowledge.ProfileModelSnapshots

- 级别：参考
- 文件：`modules/knowledge/internal/index/catalog.go`
- 作用：按已公开覆盖替换首项/保留备用语义收集当前缓存资格opaque模型证明，不计算MG hash或选择winning

```go
func ProfileModelSnapshots(catalog ModelCatalog, profile ProfileDeclaration, assignment *TaskAssignment) []string
```

**输入**

无

**输出**

无

**步骤**

1. 按已公开覆盖替换首项/保留备用语义收集当前缓存资格opaque模型证明，不计算MG hash或选择winning

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.TokenizeSource

- 级别：参考
- 文件：`modules/knowledge/internal/model/tokenize.go`
- 作用：免费模型真实BPE与完整固定空间/字节来源核对

```go
func TokenizeSource(ctx context.Context, op OperationContext, input TokenizeSourceInput) (TokenizeSourceResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 纯knowledge服务调用MG tokenizeText，严格核生产schema、完整空间、tokenizerVersion及原文hash
2. 真实tokenCount等于端点数，端点严格递增并覆盖完整原文；允许UTF8内部端点，不把rune冒充token

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`model-gateway:tokenizeText`

## knowledge.BuildSourceBlocks

- 级别：参考
- 文件：`modules/knowledge/internal/source/blocks.go`
- 作用：固定发布正文真实原叶的有界规范化/免费BPE片段

```go
func BuildSourceBlocks(ctx context.Context, op OperationContext, input BuildSourceBlocksInput) (BuildSourceBlocksResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核完整canonical正文16MiB/hash/epoch，按最近稳定块保留原BlockRef与真实JSON sourcePath
2. 原块文本视图为该最近稳定块拥有的text及alt叶按正文遍历直接串联，嵌套稳定块独立起算；sourceOffsets为未转义UTF8原文字节，不混入JSON字节
3. 每次仅处理一个真实叶的一个NFKC稳定片段，续读Next而不积压全篇膨胀映射；modelId/完整空间固定，真实BPE不猜测
4. contextVersionHash固定完整文档/原块/预处理版本的规范摘要，不虚构已执行模型上下文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.CollectSourceLeaves`、`knowledge.NormalizeSource`、`knowledge.TokenizeSource`

## knowledge.CollectSourceLeaves

- 级别：参考
- 文件：`modules/knowledge/internal/source/blocks.go`
- 作用：有序原叶、稳定最近块及原文偏移来源

```go
func CollectSourceLeaves(body []byte, document gen.PublishedRevisionRef) ([]SourceLeaf, error)
```

**输入**

无

**输出**

无

**步骤**

1. 最近稳定块拥有的text（content顺序）→attrs.alt→仅video的attrs.title→attrs.description，原叶直接串联无新增分隔符；嵌套稳定块独立起算
2. sourcePath指真实JSON字符串，offset指未转义原文字节；平台snapshot已在正文固定，不构建重抓URL，原块hash/id保留

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.WithModelOwner

- 级别：参考
- 文件：`modules/knowledge/internal/model/journal.go`
- 作用：请求独立绑定真实构建拥有者供取消/恢复/GC保护

```go
func WithModelOwner(ctx context.Context, ownerID string) (context.Context, error)
```

**输入**

无

**输出**

无

**步骤**

1. 请求独立绑定真实构建拥有者供取消/恢复/GC保护

**错误**

- 无

- 副作用：本域model_steps原结果与不可变引用
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.WithModelRuntime`

## knowledge.beginModelWrite

- 级别：参考
- 文件：`modules/knowledge/internal/model/journal.go`
- 作用：当前平台写窗口与本站同代私有身份核对

```go
func beginModelWrite(ctx context.Context, op OperationContext, runtime ModelRuntime) (context.Context, *sql.Tx, context.CancelFunc, error)
```

**输入**

无

**输出**

无

**步骤**

1. 当前平台写窗口与本站同代私有身份核对

**错误**

- 无

- 副作用：本域model_steps原结果与不可变引用
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`platform.NewWriteContext`

## knowledge.BeginModelStep

- 级别：参考
- 文件：`modules/knowledge/internal/model/journal.go`
- 作用：外部付费前持久原输入hash和原op/key，完整原结果hash核对后复用

```go
func BeginModelStep(ctx context.Context, op OperationContext, runtime ModelRuntime, key, semantic string, request platform.CreateInvocationInput) (string, InvokeModelResult, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. owner+稳定semantic_key唯一记录原MG key/op和实际去凭据请求hash，调用方临时hash/许可token不入实际请求语义
2. 同语义改输入/配置/记忆或原op不符CONFLICT，绝不新造key绕旧unknown；完整原结果核hash后复用
3. 旧空semantic记录仅在原op与完整原请求hash相同时绑定原MG key；其他旧阶段不能证明则拒新付费，保留引用和原结果
4. unknown没有真实完整结果时仅原key再观察，不永久冻结为终态；有真实完整结果保留使用，费用仍由MG持有未知预留
5. 只已知failed/cancelled或完整completed本地结果直接复用；running/pending/unknown（即使有ID）经平台原key只读观察，不能永远返回不完整旧unknown
6. 全新稳定语义准备状态prepared/dispatch_started0只证明尚未进行RPC；不覆盖旧dispatching/unknown。实际外发标记由MarkModelDispatch在真实租约门禁后提交
7. 仅background且明确owner首次prepared/0时冻结完整去凭据原Create DTO，实际RPC字段使用持久resolved key；QA正文/permit永不持久；重放原语义输入漂移CONFLICT

**错误**

- 无

- 副作用：本域model_steps原结果与不可变引用
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.beginModelWrite`、`knowledge.HasCompleteResult`

## knowledge.SaveModelProgress

- 级别：参考
- 文件：`modules/knowledge/internal/model/journal.go`
- 作用：真实网关事件引用与root/费用类别核对并持久，不保存许可或prompt

```go
func SaveModelProgress(ctx context.Context, op OperationContext, runtime ModelRuntime, key string, event platform.StreamEvent) error
```

**输入**

无

**输出**

无

**步骤**

1. 真实网关事件引用与root/费用类别核对并持久，不保存许可或prompt

**错误**

- 无

- 副作用：本域model_steps原结果与不可变引用
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.beginModelWrite`

## knowledge.CompleteModelStep

- 级别：参考
- 文件：`modules/knowledge/internal/model/journal.go`
- 作用：完整所有终态原结果≤16MiB及hash保存，拒不同原结果，不重算费用未知

```go
func CompleteModelStep(ctx context.Context, op OperationContext, runtime ModelRuntime, key string, outcome InvokeModelResult) error
```

**输入**

无

**输出**

无

**步骤**

1. 完整所有终态原结果≤16MiB及hash保存，拒不同原结果，不重算费用未知
2. 原unknown观察可推进，已完整有效原输出不可覆盖，对账须保持原winning快照；不是新attempt
3. 不完整原观察可推进到同ID真实新观察；已经完整paid输出按winner opaque hash及真实完整Text/结构Output核不变，原结果字节保留；最新真实费用终态仅投影state，不伪造费用或事件
4. 完整paid输出字节/hash不可变；最新真实CurrentSnapshot观察另列/hash保存，费用state由真实Invocation和全部attempt投影，不能用Outcome.completed当结算

**错误**

- 无

- 副作用：本域model_steps原结果与不可变引用
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.beginModelWrite`、`knowledge.HasCompleteResult`、`knowledge.WinningModelSnapshot`、`knowledge.CompletedText`、`knowledge.CompletedOutput`、`knowledge.ModelObservationState`

## knowledge.RecordModelObservation

- 级别：参考
- 文件：`modules/knowledge/internal/model/journal.go`
- 作用：取消/截止后的实际接受ID及当前对象只以短控制窗口持久观察，不续期付费、不重新模型

```go
func RecordModelObservation(ctx context.Context,op OperationContext,runtime ModelRuntime,key string,outcome InvokeModelResult)error
```

**输入**

无

**输出**

无

**步骤**

1. 取消/截止后的实际接受ID及当前对象只以短控制窗口持久观察，不续期付费、不重新模型

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只原已授权模型调用结果消费/保留，不新增批准
- 并发：固定原调用/site/epoch/root，完整结果只读或原scope短元数据窗口
- 幂等：原invocation/key不变，付费输出不可变，费用状态只投影真实观察
- 依赖：`knowledge.CompleteModelStep`

## knowledge.MarkModelDispatch

- 级别：参考
- 文件：`modules/knowledge/internal/model/journal.go`
- 作用：真实租约门禁后、平台RPC前持久原模型外发事实，旧未知不按无ID当零费

```go
func MarkModelDispatch(ctx context.Context,op OperationContext,runtime ModelRuntime,key string)error
```

**输入**

无

**输出**

无

**步骤**

1. 真实租约门禁后、平台RPC前持久原模型外发事实，旧未知不按无ID当零费
2. 同原key再次继续时清除旧时点负证明，dispatch_started保持1，不伪装从未外发

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权原本域任务内部组合，不新增模型批准
- 并发：原site/epoch/owner/op/fence CAS；失败元数据使用短控制窗口，原业务截止不变
- 幂等：原job/key/完整结果不变；未知保留原引用
- 依赖：`knowledge.beginModelWrite`

## knowledge.DownloadExtractionAsset

- 级别：参考
- 文件：`modules/knowledge/internal/extract/source.go`
- 作用：完整原发布/版本授权有界读50MiB，核真实长度/hash及Close，不取latest

```go
func DownloadExtractionAsset(ctx context.Context, op OperationContext, origin source.AssetSource) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整原发布/版本授权有界读50MiB，核真实长度/hash及Close，不取latest

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.RuntimeFromContext`、`media:readAssetBytes`

## knowledge.ParsePDFPages

- 级别：参考
- 文件：`modules/knowledge/internal/extract/pdf_worker.go`
- 作用：仅公共无网络沙箱自身子入口内解析，真实页号/阅读序/500页50MiB及8MiB字文预算，无OCR无伪空成功

```go
func ParsePDFPages(raw []byte) (PDFText, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅公共无网络沙箱自身子入口内解析，真实页号/阅读序/500页50MiB及8MiB字文预算，无OCR无伪空成功

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.ValidateSectionSources

- 级别：参考
- 文件：`modules/knowledge/internal/search/source-views.go`
- 作用：content sourcePath回读精确私有发布body hash/原块/实际JSON叶，资源路径读同发布/原块/资源hash的不可变机器提取原文视图，缺记录返回NOT_FOUND，不取latest或伪造路径

```go
func ValidateSectionSources(ctx context.Context, op OperationContext, chunks []gen.Chunk) error
```

**输入**

无

**输出**

无

**步骤**

1. content sourcePath回读精确私有发布body hash/原块/实际JSON叶，资源路径读同发布/原块/资源hash的不可变机器提取原文视图，缺记录返回NOT_FOUND，不取latest或伪造路径

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.CollectSourceLeaves`、`knowledge.publicationKey`、`knowledge.ReadExtractionSourceView`、`knowledge.ValidateSourceRange`

## knowledge.ValidateSourceRange

- 级别：参考
- 文件：`modules/knowledge/internal/search/source-views.go`
- 作用：原块/真实UTF8范围核验，不混淆机器原文与人工正文

```go
func ValidateSourceRange(value gen.Chunk, leaf source.SourceLeaf) error
```

**输入**

无

**输出**

无

**步骤**

1. 原块/真实UTF8范围核验，不混淆机器原文与人工正文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.ReadExtractionSourceView

- 级别：参考
- 文件：`modules/knowledge/internal/source/extraction-view.go`
- 作用：只读取同站同代/完整发布/原块/AssetRef/路径的不可变提取原文及hash，缺失不捏造

```go
func ReadExtractionSourceView(ctx context.Context, op OperationContext, value gen.Chunk) (SourceLeaf, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读取同站同代/完整发布/原块/AssetRef/路径的不可变提取原文及hash，缺失不捏造

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.RuntimeFromContext`

## knowledge.SameAsset

- 级别：参考
- 文件：`modules/knowledge/internal/extract/describe.go`
- 作用：完整资源hash/类型/长度与授权原version来源核对

```go
func SameAsset(asset platform.AssetRef, origin source.AssetSource) bool
```

**输入**

无

**输出**

无

**步骤**

1. 完整资源hash/类型/长度与授权原version来源核对

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.DescribeResource

- 级别：参考
- 文件：`modules/knowledge/internal/extract/describe.go`
- 作用：只原批准background任务/对应记忆/配置档位/真实资源消息；模型原结果持久，机器描述不改人工caption

```go
func DescribeResource(ctx context.Context, op OperationContext, id string, origin source.AssetSource, memory platform.MemoryContext, task, tier, capability, text string, parts []platform.GatewayPart, limit int64) (Extraction, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只原批准background任务/对应记忆/配置档位/真实资源消息；模型原结果持久，机器描述不改人工caption
2. 全量明批资源计划显式携原routeVersion/expectedPriceSnapshots；普通资源使用MG真实免费路由版本，原key重放不改期望

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadModelCatalog`、`knowledge.WithModelOwner`、`knowledge.InvokeModel`、`knowledge.DescriptionText`、`knowledge.WinningModelSnapshot`、`knowledge.TextProfile`

## knowledge.DescriptionText

- 级别：参考
- 文件：`modules/knowledge/internal/extract/describe.go`
- 作用：只读取真实完成模型完整UTF8输出，失败/费用未知无伪文本

```go
func DescriptionText(outcome modelruntime.InvokeModelResult) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读取真实完成模型完整UTF8输出，失败/费用未知无伪文本

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.HasCompleteResult`、`knowledge.CompletedText`

## knowledge.WinningModelSnapshot

- 级别：参考
- 文件：`modules/knowledge/internal/model/provenance.go`
- 作用：只用真实resultAttemptId在原Invocation找opaque快照hash，不按费用状态猜winning，旧缺证明失败关闭

```go
func WinningModelSnapshot(outcome InvokeModelResult) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只用真实resultAttemptId在原Invocation找opaque快照hash，不按费用状态猜winning，旧缺证明失败关闭
2. 真实CurrentSnapshot优先于历史SSE Invocation，只按其resultAttemptId精确取opaque模型证明；无当前对象才兼容原真实事件

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：无

## knowledge.HasCompleteResult

- 级别：参考
- 文件：`modules/knowledge/internal/model/provenance.go`
- 作用：完整真实结果与费用状态分离，unknown必须真实完整结果及winning证明

```go
func HasCompleteResult(outcome InvokeModelResult) bool
```

**输入**

无

**输出**

无

**步骤**

1. 完整真实结果与费用状态分离，unknown必须真实完整结果及winning证明
2. 完整当前Output与真实winning证明可消费，不要求伪造completed SSE；旧真实completed事件仅作兼容分支

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大服务权限
- 并发：只读固定权威声明，无跨域业务表
- 幂等：只读，无模型调用或费用
- 依赖：`knowledge.WinningModelSnapshot`

## knowledge.CompletedText

- 级别：参考
- 文件：`modules/knowledge/internal/model/provenance.go`
- 作用：只消费真实完整Output.Text指针；旧无snapshot已保存completed真实事件兼容，不造SSE，不把部分delta补成当前全文

```go
func CompletedText(outcome InvokeModelResult,maximumBytes int)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只消费真实完整Output.Text指针；旧无snapshot已保存completed真实事件兼容，不造SSE，不把部分delta补成当前全文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只原已授权模型调用结果消费/保留，不新增批准
- 并发：固定原调用/site/epoch/root，完整结果只读或原scope短元数据窗口
- 幂等：原invocation/key不变，付费输出不可变，费用状态只投影真实观察
- 依赖：`knowledge.HasCompleteResult`

## knowledge.ModelObservationState

- 级别：参考
- 文件：`modules/knowledge/internal/model/provenance.go`
- 作用：只从真实Invocation及每个attempt投影费用副作用状态，Outcome.completed仅表示完整结果可消费

```go
func ModelObservationState(outcome InvokeModelResult)string
```

**输入**

无

**输出**

无

**步骤**

1. 只从真实Invocation及每个attempt投影费用副作用状态，Outcome.completed仅表示完整结果可消费

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅原已授权结果内部纯投影
- 并发：真实固定对象纯计算
- 幂等：相同真实对象相同输出
- 依赖：无

## knowledge.ValidatePublication

- 级别：参考
- 文件：`modules/knowledge/internal/index/publication.go`
- 作用：validatePublication的本域内部实现助手

```go
func ValidatePublication(ctx context.Context, op OperationContext, client *gen.ContentClient, validate func(json.RawMessage) error, ref gen.PublishedRevisionRef) (time.Time, int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按精确Go签名处理已校验输入和可信依赖
2. 具体分支、固定参数与共同规则见本域data及实现注释

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
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
- 依赖：`content:validatePublications`

## knowledge.ExecutionLeaseID

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：只由真实jobs job/fence编码私有镜像标识，不产生租约权限

```go
func ExecutionLeaseID(jobID string,fence int64)string
```

**输入**

无

**输出**

无

**步骤**

1. 只由真实jobs job/fence编码私有镜像标识，不产生租约权限

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：无

## knowledge.ApprovedJobInput

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：按封闭本域jobTypes读原完整payload/ref/root及派发归属并核hash长度

```go
func ApprovedJobInput(ctx context.Context,tx *sql.Tx,jobType,objectID string)(ExecutionRecord,string,string,string,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按封闭本域jobTypes读原完整payload/ref/root及派发归属并核hash长度

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：无

## knowledge.AdoptClaimedJob

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：在任何业务执行前存真实claim拥有者与fence；执行保留原批准截止，控制写不续费批准

```go
func AdoptClaimedJob(ctx context.Context,job platform.ClaimedJob)(OperationContext,error)
```

**输入**

无

**输出**

无

**步骤**

1. 在任何业务执行前存真实claim拥有者与fence；执行保留原批准截止，控制写不续费批准
2. 只有jobs真实更高fence新claim且原批准未过期，可恢复本域同原对象failed/cancelled=0阶段；续约不能复活失败/取消对象，不延长期限

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ApprovedJobInput`、`knowledge.BeginIndexWrite`、`knowledge.SaveExecutionMirror`

## knowledge.SaveExecutionMirror

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：只按真实claim/续租回执镜像lease；不能调用CheckLease递归心跳

```go
func SaveExecutionMirror(ctx context.Context,tx *sql.Tx,job platform.ClaimedJob,expires string,cancelled bool)error
```

**输入**

无

**输出**

无

**步骤**

1. 只按真实claim/续租回执镜像lease；不能调用CheckLease递归心跳

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.ExecutionLeaseID`

## knowledge.LoadExecutionRecord

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：读原拥有者完整来源及不可变最终结果，核输入/结果hash长度和对象标识

```go
func LoadExecutionRecord(ctx context.Context,tx *sql.Tx,jobID string)(ExecutionRecord,error)
```

**输入**

无

**输出**

无

**步骤**

1. 读原拥有者完整来源及不可变最终结果，核输入/结果hash长度和对象标识

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：无

## knowledge.ReadExecutionRecord

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：执行器核原job/worker/fence/attempt/root/完整ref和当前有效镜像

```go
func ReadExecutionRecord(ctx context.Context,op OperationContext,input platform.JobExecutionInput)(ExecutionRecord,error)
```

**输入**

无

**输出**

无

**步骤**

1. 执行器核原job/worker/fence/attempt/root/完整ref和当前有效镜像

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.LoadExecutionRecord`

## knowledge.MirrorExecutionLease

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：只消费平台权威续租回执，同worker/fence/epoch CAS镜像

```go
func MirrorExecutionLease(ctx context.Context,op OperationContext,job platform.ClaimedJob,info platform.HeartbeatInfo)error
```

**输入**

无

**输出**

无

**步骤**

1. 仅真实heartbeat成功且公共worker已一次绑定原私有job执行scope后，使用真实ExpiresAt接管execution_records与同原job/fence镜像；不在只读factory先写，不复制心跳或续付费deadline。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.AdoptClaimedJob`

## knowledge.ReadPaidResultRefs

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：按原owner及真实关联提取完整返回真实invocation IDs，空数组无伪造

```go
func ReadPaidResultRefs(ctx context.Context,tx *sql.Tx,op OperationContext,objectID string)([]string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按原owner及真实关联提取完整返回真实invocation IDs，空数组无伪造

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：无

## knowledge.CommitExecutionResult

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：与最终本域业务状态同事务保存完整结果及真实ObjectRef/paidResultRefs

```go
func CommitExecutionResult(ctx context.Context,tx *sql.Tx,op OperationContext,jobType,objectID,leaseID string,body any)(platform.JobExecutionResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 与最终本域业务状态同事务保存完整结果及真实ObjectRef/paidResultRefs

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.LoadExecutionRecord`、`knowledge.ExecutionLeaseID`、`knowledge.ReadPaidResultRefs`

## knowledge.RecordExecutionCancellation

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：jobs真实取消观察先持久关闭原写资格，保留unknown和独立清理意图

```go
func RecordExecutionCancellation(ctx context.Context,op OperationContext,job platform.ClaimedJob)error
```

**输入**

无

**输出**

无

**步骤**

1. jobs真实取消观察先持久关闭原写资格，保留unknown和独立清理意图

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.BeginIndexWrite`、`knowledge.LoadExecutionRecord`、`knowledge.CancelUnusedExtractions`

## knowledge.FinishExecutionFailure

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：只当前原worker/fence写真实失败诊断与完整已知引用；最终业务已提交则重交真实成功，控制写不延长付费批准

```go
func FinishExecutionFailure(ctx context.Context,op OperationContext,input platform.JobExecutionInput,cause error)(platform.JobExecutionResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只当前原worker/fence写真实失败诊断与完整已知引用；最终业务已提交则重交真实成功，控制写不延长付费批准

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权原本域任务内部组合，不新增模型批准
- 并发：原site/epoch/owner/op/fence CAS；失败元数据使用短控制窗口，原业务截止不变
- 幂等：原job/key/完整结果不变；未知保留原引用
- 依赖：`knowledge.BeginIndexWrite`、`knowledge.LoadExecutionRecord`、`knowledge.ExecutionLeaseID`、`knowledge.ReadPaidResultRefs`

## knowledge.ResolveClaimedJobOperation

- 级别：参考
- 文件：`modules/knowledge/internal/index/execution.go`
- 作用：领取factory只读核原批准任务，不在未绑定job_claim内写业务

```go
func ResolveClaimedJobOperation(ctx context.Context,job platform.ClaimedJob)(OperationContext,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读真实原输入/引用/任务owner/root/epoch/op配对，恢复原批准deadline，不续授权；无执行记录或租约镜像写入。

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ApprovedJobInput`

## knowledge.ResolveTaskMemory

- 级别：参考
- 文件：`modules/knowledge/internal/index/memory.go`
- 作用：严格生产schema读取真实任务/语言完整记忆，不自行构造空版本

```go
func ResolveTaskMemory(ctx context.Context,op OperationContext,taskID,language string)(platform.MemoryContext,error)
```

**输入**

无

**输出**

无

**步骤**

1. 严格生产schema读取真实任务/语言完整记忆，不自行构造空版本

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`memory:resolveContext`

## knowledge.CancelUnusedExtractions

- 级别：参考
- 文件：`modules/knowledge/internal/index/extraction_cancel.go`
- 作用：原构建取消同事务只取消已无其他有效原批准构建使用的子任务

```go
func CancelUnusedExtractions(ctx context.Context,tx *sql.Tx,op OperationContext,buildID string)error
```

**输入**

无

**输出**

无

**步骤**

1. 原构建取消同事务只取消已无其他有效原批准构建使用的子任务

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：无

## knowledge.CancelExtractionSideEffects

- 级别：参考
- 文件：`modules/knowledge/internal/index/extraction_cancel.go`
- 作用：只取消持久原提取job/key/模型引用，不删除unknown，不重定向最新源

```go
func CancelExtractionSideEffects(ctx context.Context,op OperationContext,id string)error
```

**输入**

无

**输出**

无

**步骤**

1. 只取消持久原提取job/key/模型引用，不删除unknown，不重定向最新源

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ApprovedJobInput`、`knowledge.BeginIndexWrite`、`jobs:cancelJob`、`platform.CancelInference`

## knowledge.ReadJobRecovery

- 级别：约束
- 文件：`modules/knowledge/internal/runtime/job_recovery.go`
- 作用：只jobs服务核原完整来源及失效租约后给completed/safeToRetry/pending/unknown真实证明

```go
func ReadJobRecovery(ctx context.Context,op OperationContext,input ReadJobRecoveryInput)(ReadJobRecoveryResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整inputRef/root/epoch/job/worker/expiredFence/attempt/cancel逐项核持久来源；completed只返回本域最终事务真实ResultRef及非null完整paidRefs，不要求费用已结算
2. 模型只读观察在SQL锁外，经公共Observe/Lookup原ID/key；背景原DTO/hash与原op/root/site/epoch齐全才能lookup，不调用InvokeModel/Create/Cancel；旧缺body/ID保持pending
3. prepared且dispatch_started=0仅证明未RPC；1仍可能收费，只有专用INVOCATION_NOT_ACCEPTED时点观察可继续同原key，不推永久零费或新owner
4. 本域全部未完成模型调用真实终局且结果hash/专用观察完整，子任务全完成/取消；再读取MG完整原root窗口、显式终局/held/unknown/逐账单价格证明；任一缺失/旧epoch/超预算保持pending
5. 上游IO后重新核原worker/fence/attempt/cancel及真实租约；workflowVersion1才safeToRetry，原批准过期仅供jobs按DEADLINE_EXCEEDED分类，不续付费或复活原批准；取消依原事实终止
6. 新版完全未执行且无执行/模型/孩子/分块事实可证明从未执行；其余pending/unknown，不推零费用或新owner；原paidRefs始终保留
7. 免费恢复完整artifact必须逐hash/原真实paid输出/原资源与winning回验；只把业务完整缓存作为可免费使用结果，不伪造旧jobs完成；跨根无需其fee已结算，同根仍全root费用证明

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED`：网关额度不足（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：permission：service；rule：仅jobs纯服务、本站同代；path/body jobId相同，input.owner固定knowledge
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ApprovedJobInput`、`knowledge.LoadExecutionRecord`、`knowledge.ReadPaidResultRefs`、`knowledge.ObserveOwnerModelSteps`、`knowledge.ValidateModelNonAcceptance`、`knowledge.OwnerFeesFinal`、`knowledge.ValidateRecoveredExtractions`

## knowledge.ReadBuildPlan

- 级别：参考
- 文件：`modules/knowledge/internal/build/plan.go`
- 作用：读原构建首次完整阶段计划，核原hash与16MiB运行预算

```go
func ReadBuildPlan(ctx context.Context,op OperationContext,id,phase string,target any)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 读原构建首次完整阶段计划，核原hash与16MiB运行预算

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`

## knowledge.FreezeBuildPlan

- 级别：参考
- 文件：`modules/knowledge/internal/build/plan.go`
- 作用：真实原租约内冻结完整阶段计划；配置/分组漂移CONFLICT，不换key

```go
func FreezeBuildPlan(ctx context.Context,op OperationContext,id,leaseID,phase string,plan any)error
```

**输入**

无

**输出**

无

**步骤**

1. 真实原租约内冻结完整阶段计划；配置/分组漂移CONFLICT，不换key

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.BeginBuildWrite`

## knowledge.SetBuildStage

- 级别：参考
- 文件：`modules/knowledge/internal/build/plan.go`
- 作用：原租约内记录已经进入的实际阶段

```go
func SetBuildStage(ctx context.Context,op OperationContext,id,leaseID,stage string)error
```

**输入**

无

**输出**

无

**步骤**

1. 原租约内记录已经进入的实际阶段

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.BeginBuildWrite`

## knowledge.PrepareEmbeddingPlan

- 级别：参考
- 文件：`modules/knowledge/internal/build/embedding_plan.go`
- 作用：固定真实模型声明和单块批次，完整检索文本真实BPE预算，不因缓存变化改组

```go
func PrepareEmbeddingPlan(ctx context.Context,op OperationContext,config gen.IndexConfig,chunks []gen.Chunk,runtime indexer.Runtime)(EmbeddingPlan,error)
```

**输入**

无

**输出**

无

**步骤**

1. 固定真实模型声明和单块批次，完整检索文本真实BPE预算，不因缓存变化改组

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.ReadModelCatalog`、`knowledge.EmbeddingProfile`、`knowledge.RetrievalText`

## knowledge.ReuseEmbeddingBatch

- 级别：参考
- 文件：`modules/knowledge/internal/build/embedding_plan.go`
- 作用：同完整空间与完整检索文本匹配真实向量，核hash/维度后复用，不覆盖原结果

```go
func ReuseEmbeddingBatch(ctx context.Context,op OperationContext,buildID,leaseID string,ids []string,space gen.EmbeddingSpace)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 同完整空间与完整检索文本匹配真实向量，核hash/维度后复用，不覆盖原结果

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.RetrievalText`、`knowledge.DecodeVector`、`knowledge.StoreEmbeddings`

## knowledge.CheckEmbeddingPlan

- 级别：参考
- 文件：`modules/knowledge/internal/build/embedding_plan.go`
- 作用：首次嵌入付费前核冻结模型声明；原语义已存在仅找回原key

```go
func CheckEmbeddingPlan(ctx context.Context,op OperationContext,buildID string,ids []string,space gen.EmbeddingSpace,plan EmbeddingPlan)error
```

**输入**

无

**输出**

无

**步骤**

1. 核原site/epoch/owner/semantic存在则不新规划；不存在则当前权威profile/model必须与首次完整计划一致，变化CONFLICT

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已批准原构建内部调用
- 并发：原拥有者与固定分组
- 幂等：未知不另key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ReadModelCatalog`、`knowledge.EmbeddingProfile`

## knowledge.ExecuteExtraction

- 级别：约束
- 文件：`modules/knowledge/internal/extract/execute.go`
- 作用：按资源类型执行一次派生文本提取

```go
func ExecuteExtraction(ctx context.Context, op OperationContext,input JobExecutionInput)(result JobExecutionResult,failure error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `JobExecutionInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `JobExecutionResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 真实独立jobs租约读取完整原ExtractionJobInput；核原发布当前性、精确媒体version/hash授权及冻结任务计划
2. PDF公共隔离runner真实提取，图片真实视觉，文本附件完整描述，上传视频只原可信正文元数据；未声明格式/无文字unsupported，无自动OCR/字幕/URL重抓
3. 新付费前通过WithModelLease回核jobs；原owner/稳定语义/原记忆档位/key找回，unknown绝不新owner或换配置重算
4. 核当前镜像fence/lease/cancel与原计划，提取结果/hash/实际模型证明和最终JobExecutionResult同事务；人工caption不动
5. 错误边界持久当前原fence真实failure_code/state并保完整已知paid refs；如果最终业务已原子完成则只重交真实成功结果，不覆盖；控制元数据窗口不延长原批准
6. 来源明确unsupported时直接真实unsupported，不下载超限资源或调用模型；真实模型付费前再次CheckExtractionPlan原声明，已外发仅原key观察

**错误**

- `UNSUPPORTED_CAPABILITY`：类型不支持或未配置视觉档（不可重试）
- `DEPENDENCY_UNAVAILABLE`：media或网关不可用（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `BUDGET_EXCEEDED`：网关额度不足（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写提取结果
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：同assetHash/extractorVersion一次一个
- 幂等：按assetHash/extractorVersion幂等复用
- 依赖：`knowledge.ReadExecutionRecord`、`knowledge.ExecutionLeaseID`、`knowledge.RuntimeFromContext`、`knowledge.ValidatePublication`、`knowledge.CheckExtractionPlan`、`knowledge.WithModelLease`、`knowledge.ExtractPdfText`、`knowledge.ExtractImageDescription`、`knowledge.DescribeAttachment`、`knowledge.ExtractVideoMetadata`、`knowledge.BeginIndexWrite`、`knowledge.CommitExecutionResult`、`knowledge.FinishExecutionFailure`

## knowledge.CheckExtractionPlan

- 级别：参考
- 文件：`modules/knowledge/internal/extract/execute.go`
- 作用：首次付费前核冻结声明；已存在阶段只观察原key不重选计划

```go
func CheckExtractionPlan(ctx context.Context,op OperationContext,payload ExtractionJobInput)error
```

**输入**

无

**输出**

无

**步骤**

1. 首次付费前核冻结声明；已存在阶段只观察原key不重选计划

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.PrepareExtractionPlan`

## knowledge.DispatchExtraction

- 级别：参考
- 文件：`modules/knowledge/internal/extract/dispatch.go`
- 作用：先持久dispatching，再原input/root/op/epoch/dedupe派发独立jobs，未知不换key

```go
func DispatchExtraction(ctx context.Context,op OperationContext,id string)error
```

**输入**

无

**输出**

无

**步骤**

1. 先持久dispatching，再原input/root/op/epoch/dedupe派发独立jobs，未知不换key

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ApprovedJobInput`、`knowledge.BeginIndexWrite`、`jobs:createJob`

## knowledge.ReadExtractionJobState

- 级别：参考
- 文件：`modules/knowledge/internal/extract/dispatch.go`
- 作用：只按本域原job引用与纯owner身份读jobs权威状态，attempt失败不冒任务终局

```go
func ReadExtractionJobState(ctx context.Context,op OperationContext,id string)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 锁内只读原完整批准来源后释放SQL；外部只读原jobId，严格生产schema及已登记九状态，未知拒绝

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：knowledge纯服务仅自己持久owner/type任务；site/epoch取可信op及原批准，不接受自报owner
- 并发：固定原job/完整引用，网络锁外，不改原root或付费批准
- 幂等：纯读取原jobId，可重复
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ApprovedJobInput`、`jobs:readJob`

## knowledge.BindExtractionViews

- 级别：参考
- 文件：`modules/knowledge/internal/extract/views.go`
- 作用：真实提取页/描述按精确发布/块/asset关联不可变机器原文视图，页路径独立偏移零起算

```go
func BindExtractionViews(ctx context.Context,op OperationContext,buildID,leaseID string,origin source.AssetSource,artifact ExtractionArtifact)([]source.SourceLeaf,error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实提取页/描述按精确发布/块/asset关联不可变机器原文视图，页路径独立偏移零起算

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.BeginBuildWrite`

## knowledge.BuildMaterialBlocks

- 级别：参考
- 文件：`modules/knowledge/internal/source/material.go`
- 作用：完整机器文字NFKC真实稳定边界续读，4MiB/1Mi字符BPE片段与原页路径偏移不改

```go
func BuildMaterialBlocks(ctx context.Context,op OperationContext,leaf SourceLeaf,asset *gen.AssetRef,modelID string,space gen.EmbeddingSpace)([]chunk.SourceBlock,error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整机器文字NFKC真实稳定边界续读，4MiB/1Mi字符BPE片段与原页路径偏移不改

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅已授权本域流程组合调用，不扩大调用方权限
- 并发：本站同代原输入/原拥有者/fence与安全窗口；只读固定快照
- 幂等：原输入/hash/语义身份不变，未知不换key
- 依赖：`knowledge.NormalizeSource`、`knowledge.TokenizeSource`

## knowledge.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/knowledge/internal/snapshot_archive.go`
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

## knowledge.ObserveOwnerModelSteps

- 级别：参考
- 文件：`modules/knowledge/internal/model/recovery.go`
- 作用：锁外每次至多16项真实原模型观察；已知ID用公共Observe，未知ID仅完整背景原请求lookup，普通404保持未知；完整未决集合仍阻止safeToRetry

```go
func ObserveOwnerModelSteps(ctx context.Context,op OperationContext,objectID string)error
```

**输入**

无

**输出**

无

**步骤**

1. 锁外每次至多16项真实原模型观察；已知ID用公共Observe，未知ID仅完整背景原请求lookup，普通404保持未知；完整未决集合仍阻止safeToRetry

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅原已批准本域背景任务的观察与恢复证明；不新增模型付费批准
- 并发：SQL锁外真实只读上游；本地短元数据窗口只原site/epoch/owner/key，最终恢复重新核worker/fence/cancel
- 幂等：原完整请求/hash/op/root/key不变；保留原paid结果与真实观察，未知不换owner
- 依赖：`platform.ObserveInvocation`、`platform.LookupModelInvocation`、`knowledge.RecordModelObservation`、`knowledge.RecordModelNonAcceptance`

## knowledge.RecordModelNonAcceptance

- 级别：参考
- 文件：`modules/knowledge/internal/model/recovery.go`
- 作用：只保存MG专用INVOCATION_NOT_ACCEPTED时点观察及完整原scope/hash，保持dispatch_started=1；不推永久无费或新owner

```go
func RecordModelNonAcceptance(ctx context.Context,op OperationContext,runtime ModelRuntime,key,inputHash string)error
```

**输入**

无

**输出**

无

**步骤**

1. 只保存MG专用INVOCATION_NOT_ACCEPTED时点观察及完整原scope/hash，保持dispatch_started=1；不推永久无费或新owner

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅原已批准本域背景任务的观察与恢复证明；不新增模型付费批准
- 并发：SQL锁外真实只读上游；本地短元数据窗口只原site/epoch/owner/key，最终恢复重新核worker/fence/cancel
- 幂等：原完整请求/hash/op/root/key不变；保留原paid结果与真实观察，未知不换owner
- 依赖：`knowledge.beginModelWrite`

## knowledge.ValidateModelNonAcceptance

- 级别：参考
- 文件：`modules/knowledge/internal/model/recovery.go`
- 作用：核专用观察完整hash/原site/epoch/owner/key/op/root/input和真实时点，不能以普通404/缺ID代替

```go
func ValidateModelNonAcceptance(raw []byte,hash,site,epoch,owner,key,operation,root,inputHash string)error
```

**输入**

无

**输出**

无

**步骤**

1. 核专用观察完整hash/原site/epoch/owner/key/op/root/input和真实时点，不能以普通404/缺ID代替

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅原已批准本域背景任务的观察与恢复证明；不新增模型付费批准
- 并发：SQL锁外真实只读上游；本地短元数据窗口只原site/epoch/owner/key，最终恢复重新核worker/fence/cancel
- 幂等：原完整请求/hash/op/root/key不变；保留原paid结果与真实观察，未知不换owner
- 依赖：无

## knowledge.RootFeesFinal

- 级别：参考
- 文件：`modules/knowledge/internal/model/fee-proof.go`
- 作用：严格生产schema读原root QueryUsage有界完整窗；显式windowCoversRoot/allInvocationsFinal、held USD0、空unknown、每条pricingKnown与actualCost齐全才证明；缺字段/旧epoch/超预算未知

```go
func RootFeesFinal(ctx context.Context,op OperationContext,rootID string)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 严格生产schema读原root QueryUsage有界完整窗；显式windowCoversRoot/allInvocationsFinal、held USD0、空unknown、每条pricingKnown与actualCost齐全才证明；缺字段/旧epoch/超预算未知

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅原已批准本域背景任务的观察与恢复证明；不新增模型付费批准
- 并发：SQL锁外真实只读上游；本地短元数据窗口只原site/epoch/owner/key，最终恢复重新核worker/fence/cancel
- 幂等：原完整请求/hash/op/root/key不变；保留原paid结果与真实观察，未知不换owner
- 依赖：`model-gateway:queryUsage`

## knowledge.OwnerFeesFinal

- 级别：参考
- 文件：`modules/knowledge/internal/model/fee-proof.go`
- 作用：原已接受调用和未完成孩子按真实原root核全部费用；跨根已完成缓存免费复用无需重付证明，同根已完成孩子仍全root覆盖，paidRefs完整保留

```go
func OwnerFeesFinal(ctx context.Context,op OperationContext,objectID,rootID string)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 原已接受调用和未完成孩子按真实原root核全部费用；跨根已完成缓存免费复用无需重付证明，同根已完成孩子仍全root覆盖，paidRefs完整保留

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅原已批准本域背景任务的观察与恢复证明；不新增模型付费批准
- 并发：SQL锁外真实只读上游；本地短元数据窗口只原site/epoch/owner/key，最终恢复重新核worker/fence/cancel
- 幂等：原完整请求/hash/op/root/key不变；保留原paid结果与真实观察，未知不换owner
- 依赖：`knowledge.RootFeesFinal`

## knowledge.InspectFailedExtraction

- 级别：参考
- 文件：`modules/knowledge/internal/extract/cache.go`
- 作用：新明确批准build先只读观察原paid完整输出并核真实winner/原记忆计划；同build永远原owner；无可用输出仅jobs真终局及完整原root费用证明才允许CAS新owner；无ID已外发（含时点未接受）禁止换owner

```go
func InspectFailedExtraction(ctx context.Context,op OperationContext,buildID,intentKey string,origin source.AssetSource,plan ExtractionPlan)(FailedExtractionResolution,error)
```

**输入**

无

**输出**

无

**步骤**

1. 新明确批准build先只读观察原paid完整输出并核真实winner/原记忆计划；同build永远原owner；无可用输出仅jobs真终局及完整原root费用证明才允许CAS新owner；无ID已外发（含时点未接受）禁止换owner
2. 旧source/plan必须与ApprovedJobInput完整原inputRef hash一致；新memory/route不能改变稳定资源意图，合法完整paid输出可免费复用，即使仍费用unknown；不同计划未终局CONFLICT

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.ObserveOwnerModelSteps`、`knowledge.HasCompleteResult`、`knowledge.WinningModelSnapshot`、`knowledge.DescriptionText`、`knowledge.ReadExtractionJobState`、`knowledge.ApprovedJobInput`、`knowledge.RootFeesFinal`

## knowledge.SaveRecoveredExtraction

- 级别：参考
- 文件：`modules/knowledge/internal/extract/cache.go`
- 作用：只保存原完整paid输出hash证明的免费派生artifact；原失败任务、MG费用和execution_records不改，不伪造旧jobs完成回执

```go
func SaveRecoveredExtraction(ctx context.Context,tx *sql.Tx,op OperationContext,resolution FailedExtractionResolution)error
```

**输入**

无

**输出**

无

**步骤**

1. 只保存原完整paid输出hash证明的免费派生artifact；原失败任务、MG费用和execution_records不改，不伪造旧jobs完成回执

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：无

## knowledge.ValidateRecoveredExtractions

- 级别：参考
- 文件：`modules/knowledge/internal/extract/cache.go`
- 作用：拥有者恢复前逐项核免费恢复artifact与真正paid输出/hash/原asset块/文字/真实winning一致，存在行不代替完整证据

```go
func ValidateRecoveredExtractions(ctx context.Context,op OperationContext,objectID string)error
```

**输入**

无

**输出**

无

**步骤**

1. 最多原100资源及自身完整证据，逐hash回验原artifact/model结果/来源；不从恢复缓存存在推零费
2. 跨根完整免费缓存不再等旧任务执行回执，同根仍由完整RootFeesFinal涵盖费用，原paidRefs完整保留

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅原拥有者恢复证明内部组合，不新增批准
- 并发：固定本站同代原SQL快照
- 幂等：相同不可变证据相同结果，损坏失败关闭
- 依赖：`knowledge.RuntimeFromContext`、`knowledge.DescriptionText`、`knowledge.WinningModelSnapshot`、`knowledge.SameAsset`

## knowledge.ReadRebuildExtractionCache

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild_cache.go`
- 作用：只按原asset/任务记忆/提取器与当前模型真实snapshot读合法缓存，缺失保持未知材料

```go
func ReadRebuildExtractionCache(ctx context.Context,op OperationContext,runtime indexer.Runtime,resource RebuildResourcePlan)(extract.ExtractionArtifact,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只按原asset/任务记忆/提取器与当前模型真实snapshot读合法缓存，缺失保持未知材料

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：无

## knowledge.ReadRebuildCachedContext

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild_cache.go`
- 作用：按真实完整源/记忆/模型/原片段身份读取已付费前缀，核hash后派生检索文字

```go
func ReadRebuildCachedContext(ctx context.Context,op OperationContext,runtime indexer.Runtime,document gen.PublishedRevisionRef,route RebuildRoute,value gen.Chunk)(gen.Chunk,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按真实完整源/记忆/模型/原片段身份读取已付费前缀，核hash后派生检索文字

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`knowledge.ContextResultHash`、`knowledge.ContextSourceKey`

## knowledge.HasRebuildVector

- 级别：参考
- 文件：`modules/knowledge/internal/build/rebuild_cache.go`
- 作用：只有完整检索文字和完整空间均一致且真实向量维度/hash合法才扣可复用数量

```go
func HasRebuildVector(ctx context.Context,op OperationContext,runtime indexer.Runtime,target gen.Chunk,space gen.EmbeddingSpace)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只有完整检索文字和完整空间均一致且真实向量维度/hash合法才扣可复用数量

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅真实已授权本域流程组合；纯估价不调用模型，原未知不批准新owner
- 并发：SQL快照锁外读取权威，写入当前安全窗口与原scope/CAS；不复制平台机制
- 幂等：原输入/hash/owner/key不变；已付费结果保留，重复返回真实原回执
- 依赖：`knowledge.RetrievalText`、`knowledge.DecodeVector`

## knowledge.ReadTaskRoute

- 级别：参考
- 文件：`modules/knowledge/internal/index/route.go`
- 作用：严格MG resolveTaskRoute同快照resolved/candidates/routeVersion和候选modelSnapshotHash/embeddingSpace，不复制筛选/覆盖/快照hash算法

```go
func ReadTaskRoute(ctx context.Context,op OperationContext,runtime Runtime,taskID string)(TaskRoute,error)
```

**输入**

无

**输出**

无

**步骤**

1. 严格MG resolveTaskRoute同快照resolved/candidates/routeVersion和候选modelSnapshotHash/embeddingSpace，不复制筛选/覆盖/快照hash算法

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只消费原已授权任务权威声明，不扩模型付费批准
- 并发：锁外读取真实权威；原scope/CAS写入，付费接受路由/价格原子CAS由MG拥有
- 幂等：原hash/key/op/root/route和价格快照保留，未知不换owner
- 依赖：`model-gateway:resolveTaskRoute`、`knowledge.ReadTaskAssignment`、`knowledge.ProfileModelSnapshots`

## knowledge.WithModelExpectations

- 级别：参考
- 文件：`modules/knowledge/internal/model/expectation.go`
- 作用：只绑定原明确批准opaque路由和逐真实候选价格快照，接受事务CAS由MG统一拥有

```go
func WithModelExpectations(ctx context.Context,expectations map[string]ModelExpectation)(context.Context,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只绑定原明确批准opaque路由和逐真实候选价格快照，接受事务CAS由MG统一拥有

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：只消费原已授权任务权威声明，不扩模型付费批准
- 并发：锁外读取真实权威；原scope/CAS写入，付费接受路由/价格原子CAS由MG拥有
- 幂等：原hash/key/op/root/route和价格快照保留，未知不换owner
- 依赖：`knowledge.WithModelRuntime`

## knowledge.KnowledgePage

- 级别：约束
- 文件：`modules/knowledge/web/src/index.tsx`
- 作用：知识索引：入库配置、为已发布文档建立索引、索引任务进度与取消

```ts
export default function KnowledgePage(): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 读取索引选项、配置与任务
2. 建立索引前确认费用；有任务进行时定时刷新

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无
