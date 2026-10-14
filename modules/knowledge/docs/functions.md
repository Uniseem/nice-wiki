# knowledge 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 57 个函数：约束 27 个，参考 30 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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

1. 检查DB与索引文件manifest
2. 核验空间/哈希一致
3. 不一致active标unavailable
4. 注册租约恢复
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：无

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

1. 校验chunkTokens/overlap
2. 解析模型空间声明
3. 空间变化只生成新代数不覆盖active
4. CAS保存
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`model-gateway:readModels`

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

1. inbox按eventId去重
2. 验证dataEpoch
3. 配置关闭或未配置档位则记录skipped
4. 创建唯一document/revision/space build与job outbox同事务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`jobs:createJob`、`content:validatePublications`

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

1. 向content验证visible/current
2. 固定config/space
3. 同修订同空间幂等
4. force新generation且保留旧active
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`content:validatePublications`、`jobs:createJob`

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

1. 校验build租约及deadline
2. 调content只读发布对象
3. 对比contentHash
4. 保存私有临时source引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`content:readPublishedRevision`

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

1. 按正文schema解析引用
2. media批量授权验证
3. 限制每文档100资源单件50MiB
4. 不支持类型标明unsupported
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
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`media:acquireResourceReferences`

## knowledge.CreateExtraction

- 级别：参考
- 文件：`modules/knowledge/internal/extract/create.go`
- 作用：按hash幂等提取资源

```go
func CreateExtraction(ctx context.Context, op OperationContext, input CreateExtractionInput) (CreateExtractionResult, error)
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
- 依赖：`media:readAsset`、`jobs:createJob`

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

1. media授权流下载并验证hash
2. 在无网络沙箱读取最多500页/50MiB
3. 保留页码按阅读序提取
4. 无文本标unsupported等待显式OCR任务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数60000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`media:readAsset`

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

1. 发布构建显式视觉描述配置
2. 根knowledge.describe-visual使用vision档（未设采用fast视觉能力）800output token
3. 图片/画板/视频封面assetHash+task/model/memory version幂等复用
4. 保存machineDerived仅检索不覆盖人工caption，取消旧revision后不写。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`model-gateway:createInference`、`media:readAsset`

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

1. 从media取得已授权平台元数据
2. 仅收录已有字幕与题名
3. 校验语言/编码
4. 无字幕不自动转写
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`media:readVideoMetadata`

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

1. 解析块级纯文本
2. Unicode NFKC并统一LF
3. 保留段落和代码大小写
4. 构建offset映射不丢弃引用源位置
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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

1. 同小节相邻块合并默认512token但不跨来源权限
2. 记录全部blockIds与原UTF8偏移
3. 图片描述+图注+相邻块，附件长文本再切分
4. chunk identity包括源block hashes与上下文版本，不将派生描述写回人工正文。
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
- 依赖：无

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

1. 固定Unicode词界分词版本
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
- 依赖：`model-gateway:createInference`

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
- 依赖：`content:validatePublications`

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
- 依赖：`content:validatePublications`

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

1. 设取消标记
2. 取消jobs及所有MG invocation
3. 取消staging写权限
4. 保留active与收费未知引用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`jobs:cancelJob`、`model-gateway:cancelInference`

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

1. inbox去重
2. 只接受更高visibilityRevision
3. active→retired
4. 取消相关build并事件失效
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`jobs:cancelJob`

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
- 依赖：`model-gateway:createInference`

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
- 依赖：无

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
- 依赖：`content:validatePublications`

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

1. 使用rerank档，未设时fast核查排序
2. 同候选最多20个且固定ID
3. 继承qa root/costClass与共享截止
4. 输出只能既有ID排序，最多一次格式重试再有序fallback每次独立MG预留。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`model-gateway:createInference`

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

1. query与base language规范
2. embedding存在时校验queryHash/space/vectorHash/epoch后复用，否则MG生成
3. lexical/vector各80→RRF40→可选rerank fast fallback
4. 当前页权威范围可优先且最终content可见性validate。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`content:validatePublications`、`model-gateway:createInference`、`memory:resolveContext`

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
- 依赖：无

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
- 依赖：无

## knowledge.ConsumeInbox

- 级别：参考
- 文件：`modules/knowledge/internal/events/inbox.go`
- 作用：事件去重与代际保护

```go
func ConsumeInbox(ctx context.Context, op OperationContext, input ConsumeInboxInput) (ConsumeInboxResult, error)
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

1. 校验事件类型/schema
2. dataEpoch==runtime
3. eventId/hash唯一
4. 同事务登记业务执行结果
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：无

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

1. 进程启动后以服务身份调用jobs:subscribeEvents（durable模式SSE），只订阅本域events.json中consume登记的事件名与版本；断线按Last-Event-ID续传，1/2/4秒指数退避（上限60秒）重连
2. 每条事件校验信封、生产方payloadSchema与dataEpoch：不在登记清单的事件隔离报警并确认；dataEpoch不等于当前active epoch时，若本域尚未换入该epoch则不确认等待重投，否则视为旧代事件记skipped并确认
3. 按events.json该事件的handler字段调用本域处理函数；处理函数在同一本地事务内完成inbox(eventId+payloadHash)去重、subject revision单调检查与业务写入
4. 处理函数成功提交（含重复或低revision的无操作）后才调用jobs:acknowledgeEvent；失败不确认，等jobs确认租约到期重投，连续失败由jobs转dead-letter供管理重放
5. 屏障draining时停止领取新事件，等在途处理完成后报告安全点；维护期间不确认未处理事件

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs订阅或确认不可用；重连后续传（可重试）
- `VALIDATION`：事件信封或载荷不符合登记schema；隔离并确认（不可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash；隔离报警（不可重试）

- 副作用：HTTP长连接jobs:subscribeEvents；调用本域登记的事件处理函数；HTTP调用jobs:acknowledgeEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个订阅循环；同subject串行，不同subject最多maxInFlight并发；事务与CAS由处理函数负责
- 幂等：jobs至少一次投递；本域inbox按eventId+payloadHash去重，重复投递只重新确认
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`knowledge.HandlePublication`、`knowledge.RetireDocumentIndex`

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

1. 租约领取批次
2. jobs持久发布
3. ACK才标记
4. 有限三次退避后留待下一轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`jobs:publishEvent`

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

1. 到期worker停止写
2. content验证当前发布
3. 已失效标superseded
4. 确定性已完成阶段复用，仅缺失embedding在原调用可确证时重续
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

- 副作用：更新build/lease和jobs；不重复未知收费调用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`jobs:createJob`

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

1. 检查retired且无reader lease
2. 无未完成build引用
3. 删除本服务索引文件并校验路径在私有目录
4. 删除元数据事务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：无

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
- 依赖：无

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
- 依赖：`content:validatePublications`

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

1. 校验data-protection服务身份及superadmin委托
2. 将本地control.db屏障draining并拒绝新业务写
3. 屏障停止新build、active切换和staging写入，等待reader lease排空
4. 导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建
5. 派生索引可由管理员另发重建任务。
6. 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。
7. 备份恢复控制只接受data-protection服务身份与superadmin委托，并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 授权：service身份+mTLS和superadmin委托；控制调用要求当前epoch与runtime阶段匹配；本地control区不随恢复替换。
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`

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
5. 备份恢复控制只接受data-protection服务身份与superadmin委托，并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 授权：service身份+mTLS和superadmin委托；控制调用要求当前epoch与runtime阶段匹配；本地control区不随恢复替换。
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`

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
7. 备份恢复控制只接受data-protection服务身份与superadmin委托，并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 授权：service身份+mTLS和superadmin委托；控制调用要求当前epoch与runtime阶段匹配；本地control区不随恢复替换。
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`

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

1. 校验data-protection服务身份与superadmin委托；site-config runtime为restore_staging且restoreId一致；本域restore屏障为safe
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
- 授权：permission：superadmin；rule：仅data-protection服务身份携带发起恢复的superadmin委托；普通业务调用拒绝
- 并发：control区按restoreId串行；不锁active读写；同一时间只允许一个恢复
- 幂等：restoreId+checksum唯一；重复调用返回已记录回执
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`

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

1. 校验data-protection服务身份与superadmin委托
2. scope=staged：仅当runtime不在restore_switching/restore_rolling_back、且control日志显示本域尚未换入该restoreId时删除restore/<restoreId>/staged
3. scope=before：仅当该恢复已completed或已整体换回、且control日志确认active不是该before时删除restore/<restoreId>/before
4. 删除后在control区记录discarded；目录不存在返回removed=false

**错误**

- `FORBIDDEN`：调用方不是data-protection或缺superadmin委托（不可重试）
- `LOCKED`：当前恢复阶段不允许删除该目录（不可重试）

- 副作用：删除<dataRoot>/restore/<restoreId>下的目录；写control区回执
- 授权：permission：superadmin；rule：仅data-protection服务身份携带superadmin委托
- 并发：control区按restoreId串行
- 幂等：重复删除返回removed=false
- 依赖：`site-config:readRuntimeState`

## knowledge.LoadAdminOverview

- 级别：参考
- 文件：`modules/knowledge/ui/src/views/OverviewView.vue`
- 作用：加载本域后台概览

```ts
export async function loadAdminOverview(input: LoadAdminOverviewInput, signal: AbortSignal): Promise<LoadAdminOverviewResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `LoadAdminOverviewInput` | 本域封闭输入；每个字段规格见types.json $defs/LoadAdminOverviewInput | schema required/additionalProperties/range全部强制；null禁止。 |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadAdminOverviewResult` | 成功结果；逐字段见types $defs/LoadAdminOverviewResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |

**步骤**

1. portal提供已认证apiClient
2. 读取本域管理API
3. 取消旧navigation请求
4. 基于server version显示状态且不在UI推测费用或任务成功
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只读请求与组件状态更新
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## knowledge.SubmitPrimaryEdit

- 级别：参考
- 文件：`modules/knowledge/ui/src/views/EditView.vue`
- 作用：提交本域明确人工修改

```ts
export async function submitPrimaryEdit(input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `SubmitPrimaryEditInput` | 本域封闭输入；每个字段规格见types.json $defs/SubmitPrimaryEditInput | schema required/additionalProperties/range全部强制；null禁止。 |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SubmitPrimaryEditResult` | 成功结果；逐字段见types $defs/SubmitPrimaryEditResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |

**步骤**

1. 表单逐字段验证本域schema
2. 生成幂等键并携expectedVersion
3. 调本域putIndexConfig API
4. 409保留用户输入并显示当前版本，不自动覆盖重发
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：本域HTTP写请求和组件状态更新
- 授权：权限 knowledge.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

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

1. task knowledge.context使用fast档
2. 同文整篇原文为固定输入前缀，最多16片段一批
3. 生成1至2句产品/功能/小节背景不改变源正文
4. 按chunkHash+documentHash+memory版本幂等复用，只为变更块付费
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`model-gateway:createInference`、`memory:resolveContext`

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

1. 根task qa.inspect owner=knowledge fast档max800output token并注入project overview
2. 只选择本次检索合法IDs，展开小节仍content当前校验
3. 3轮/5工具调用计数来自可信根任务状态CAS不由浏览器覆盖
4. 到上限用已有材料返回finished，费用继承qa costClass。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`model-gateway:createInference`、`content:validatePublications`、`memory:resolveContext`

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

1. rootOperationId/queryHash/space/dataEpoch singleflight且唯一
2. 根knowledge.embed embedding档0文本output
3. MG保留独立attempt费用证据
4. 返回完整空间/vectorHash供QA cache与本域search复用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`model-gateway:createInference`

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

1. 支持文件MIME/大小白名单且media授权
2. 文本附件根knowledge.describe fast档800tokens
3. 非文本附件根knowledge.extract-attachment vision文件能力1500tokens，不支持明确unsupported
4. assetHash/extractor/model/memory版本复用，描述完整再长文分块。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

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
- 依赖：`media:readAsset`、`model-gateway:createInference`、`memory:resolveContext`

## knowledge.RunWorker

- 级别：约束
- 文件：`modules/knowledge/internal/runtime/worker.go`
- 作用：主动领取本域持久任务

```go
func RunWorker(ctx context.Context, op OperationContext, input RunWorkerInput) (RunWorkerResult, error)
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

1. 向jobs claimJob仅领取本域JOB_HANDLERS登记jobType
2. 每10秒heartbeatJob续30秒租约并检查cancel/deadline
3. 依jobType执行同域已登记函数，所有中间/提交校验lease与epoch
4. completeJob/failJob持久报告，lease过期停止写，未知MG attempt留held不重放。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：jobs租约/完成状态及本域工作链副作用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:claimJob`、`jobs:heartbeatJob`、`jobs:completeJob`、`jobs:failJob`

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
- 依赖：`knowledge.BuildApplication`、`knowledge.Serve`、`knowledge.Shutdown`

## knowledge.BuildApplication

- 级别：约束
- 文件：`modules/knowledge/internal/app/wire.go`
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

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

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
- 依赖：`knowledge.BuildApplication`、`knowledge.Shutdown`
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

1. 仅读本进程lifecycle与HTTP循环是否存活；不查询远程依赖/模型/数据库
2. 存活且未closed返回200 HealthStatus；closed或server不可用返回503；不泄漏路径、端点或secret

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

## knowledge.mountModuleUI

- 级别：约束
- 文件：`modules/knowledge/ui/src/main.ts`
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
