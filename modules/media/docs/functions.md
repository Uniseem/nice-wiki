# media 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 184 个函数：约束 37 个，参考 147 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## media.BeginUpload

- 级别：约束
- 文件：`modules/media/internal/upload_service.go`
- 作用：开始内容寻址可续传上传

```go
func BeginUpload(ctx context.Context, op OperationContext, input BeginUploadInput) (BeginUploadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `BeginUploadInput` | 开始内容寻址可续传上传的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BeginUploadResult` | 开始内容寻址可续传上传的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 当前media.upload会话和opened epoch先核；kind/caller/actor/site/epoch及原严格输入CanonicalJSON摘要，operationId异材料CONFLICT
2. 配额/名称候选/offset/CAS前核完整旧回执，owner/site/创建epoch一致才返回原UploadSession；旧缺原快照或NULL epoch拒绝，不从mutable会话猜原结果
3. 新操作公共NewWriteContext只允许running；NormalizeOriginalName纯语义，读取site-config版本化配额，生成24小时receiving候选
4. 创建前重核窗口和本域并发/站点配额，普通新临时文件及目录fsync后session/创建epoch/原结果hash同事务，失败不自动重建旧上传

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久原始结果或来源元数据损坏（不可重试）
- `LOCKED`：新操作遇维护或写窗口关闭（可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：上传属于编辑文档功能media.upload；资源库读取/清理media.manage；公开交付仅已公开暴露hash或有效管理员preview
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：actor/caller/site/epoch及原始输入稳定摘要；原成功返回首次快照，旧缺快照/创建epoch不猜
- 依赖：`media.NormalizeOriginalName`、`site-config:readSiteConfig`、`platform.CanonicalJSON`、`media.validateUploadActor`、`media.hashBeginUploadRequest`、`media.replayBeginUpload`、`media.mediaWriteContext`、`media.CreateUpload`、`media.readUploadQuota`

## media.ReadUpload

- 级别：约束
- 文件：`modules/media/internal/upload_service.go`
- 作用：查询续传offset

```go
func ReadUpload(ctx context.Context, op OperationContext, input ReadUploadInput) (ReadUploadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadUploadInput` | 查询续传offset的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadUploadResult` | 查询续传offset的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见 |

**步骤**

1. 限定上传者或资源管理功能
2. 一致读取receivedBytes/state/expiry
3. 不返回tmp路径/secret

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）

- 副作用：无
- 授权：media.upload本人或media.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## media.AbortUpload

- 级别：约束
- 文件：`modules/media/internal/upload_service.go`
- 作用：取消未完成上传释放配额

```go
func AbortUpload(ctx context.Context, op OperationContext, input AbortUploadInput) (AbortUploadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `AbortUploadInput` | 取消未完成上传释放配额的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AbortUploadResult` | 取消未完成上传释放配额的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；NOT_FOUND: 资源不存在或在当前主体作用域中不可见 |

**步骤**

1. 限定owner，ready不接受自动删
2. 事务stateaborted与预留释放
3. 只清该receivingtmp；并发complete谁先commit由CAS决定
4. 本轮仅receiving可取消，blocked明确拒绝并持续计入存储预留；不沿忽略unlink错误路径释放blocked配额

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：media.upload本人
- 并发：upload mutex
- 幂等：重复abort成功
- 依赖：无

## media.hashBeginUploadRequest

- 级别：参考
- 文件：`modules/media/internal/upload_service.go`
- 作用：kind/caller/actor/site/epoch及原严格输入规范摘要，不含新时钟/名称策略

```go
func hashBeginUploadRequest(op OperationContext,input BeginUploadInput) (Hash,error)
```

**输入**

无

**输出**

无

**步骤**

1. kind/caller/actor/site/epoch及原严格输入规范摘要，不含新时钟/名称策略

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`platform.CanonicalJSON`

## media.validateUploadActor

- 级别：参考
- 文件：`modules/media/internal/upload_service.go`
- 作用：仅当前管理会话和业务grant，不重复入口身份/期限

```go
func validateUploadActor(op OperationContext, grant string) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅当前管理会话和业务grant，不重复入口身份/期限

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.readUploadQuota

- 级别：参考
- 文件：`modules/media/internal/upload_service.go`
- 作用：仅免费只读站点配额；调用点派生固定media纯服务并清原PortalProof，不要求上传者site.read

```go
func readUploadQuota(ctx context.Context, reader siteConfigReader, siteID Identifier) (uploadQuota, error)
```

**输入**

无

**输出**

无

**步骤**

1. 调用点先核当前用户media.upload及scope/hash/旧回执，再从bounded op保留deadline/epoch/root/costClass，只替换为固定media纯服务Actor并清PortalProof，不携用户session/grants
2. 只调已登记site-config.readSiteConfig，响应siteId必须等于原用户站点，配额未知或非法失败关闭

**错误**

- 无

- 副作用：无
- 授权：已授权上传链中的固定media服务只读配额；原用户站点不可改变
- 并发：只读远端权威，ctx取消传播
- 幂等：只读，不触发模型/付费任务
- 依赖：`site-config:readSiteConfig`

## media.hasGrant

- 级别：参考
- 文件：`modules/media/internal/upload_service.go`
- 作用：在可信授权集合中精确查找所需权限

```go
func hasGrant(grants []string, expected string) bool
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `grants` | `[]string` | 边界已验证的授权条目 | 信任入口授权材料，不重新查询身份 |
| `expected` | `string` | 本函数调用者要求的精确授权值 | 精确匹配，不支持隐式通配 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `found` | `bool` | 是否存在完全相同的授权条目 | 只比较给定集合 |

**步骤**

1. 遍历grants并以字符串相等匹配expected，存在返回true，否则false

**错误**

- 无

- 副作用：无
- 授权：纯规则，仅供已验证调用链使用
- 并发：纯读取，无共享可变状态
- 幂等：相同输入相同结果
- 依赖：无

## media.NormalizeOriginalName

- 级别：参考
- 文件：`modules/media/internal/upload_validator.go`
- 作用：清理展示文件名而不用于路径

```go
func NormalizeOriginalName(input NormalizeOriginalNameInput) (NormalizeOriginalNameResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `NormalizeOriginalNameInput` | 清理展示文件名而不用于路径的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizeOriginalNameResult` | 清理展示文件名而不用于路径的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 去路径组件/控制字符/CRLF/双向控制符
2. 保Unicode合法basename，空值拒绝
3. 所有OSpath仍仅服务生成，下载RFC5987编码

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## media.HashUpload

- 级别：参考
- 文件：`modules/media/internal/upload_validator.go`
- 作用：流计算对象真相摘要

```go
func HashUpload(ctx context.Context, op OperationContext, input HashUploadInput) (HashUploadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `HashUploadInput` | 流计算对象真相摘要的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HashUploadResult` | 流计算对象真相摘要的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；CANCELLED: 请求在持久提交前被取消；已提交则返回原提交回执；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 打开服务生成tmp readonly
2. 固定64KiBbuffer流hash并核长度
3. 取消/期限中止不提交任何ready元数据

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `CANCELLED`：请求在持久提交前被取消；已提交则返回原提交回执（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：私有upload完成
- 并发：只在upload完成锁下读
- 幂等：相同bytes同SHA
- 依赖：无

## media.PutUploadChunk

- 级别：约束
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：顺序原子写有界上传块

```go
func PutUploadChunk(ctx context.Context, op OperationContext, input PutUploadChunkInput) (PutUploadChunkResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PutUploadChunkInput` | 顺序原子写有界上传块的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutUploadChunkResult` | 顺序原子写有界上传块的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 当前media.upload管理会话、同站点本人且op.dataEpoch等于受信打开active代次；先核同op已绑定的actor/site/epoch/upload/稳定摘要，成功只读返回原UploadSession，不重查当前quota/offset/CAS/不消费reader
2. 新/未完成先由公共NewWriteContext取得真实running窗口，维护直接LOCKED；upload独占下核receiving/expiry/offset/expectedRevision及创建epoch，旧NULL epoch不猜测继续
3. HTTP明确Content-Length 1..8MiB绑定内部io.Reader；有界读length+1，精确长度与SHA一致才fsync私有临时chunk。摘要固定upload/offset/length/sha/expectedRevision，不含reader或新时钟
4. 完整材料/私有stage标识PREPARED先持久，再按SQL权威offset截去未提交尾并追加已核chunk、fsync；确认窗口仍running后offset+revision CAS与COMMITTED原结果/hash同SQL事务
5. 崩溃后或下次写前只按已提交SQL水位截尾，短文件/坏材料失败关闭；未提交prepared/temp保留或隔离，不计数成功，不自动发布ready
6. 整条HTTP链公共硬body预算优先413 BUDGET_EXCEEDED；预算内缺失/未知或不合法Content-Length为400，内部不复制入口schema

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `FORBIDDEN`：非当前上传权限或非本人（不可重试）
- `NOT_FOUND`：上传不存在（不可重试）
- `LOCKED`：维护或安全写窗口关闭（可重试）
- `BUDGET_EXCEEDED`：公共HTTP入口body硬预算超限，优先于长度schema错误（不可重试）

- 副作用：本域有界临时chunk、fsync、准备决定、上传offset/CAS和原始操作回执；不发布资源或事件
- 授权：media.upload当前管理会话且仅同站点上传者；当前授权/epoch先于旧成功回放
- 并发：uploadId独占，offset更新同journal事务
- 幂等：operationId绑定actor/site/epoch/upload及稳定业务hash；原成功返回原不可变UploadSession回执，异材料CONFLICT
- 依赖：`platform.NewWriteContext`、`platform.CanonicalJSON`、`media.mediaWriteContext`、`media.hashChunkRequest`、`media.readChunkOperation`、`media.readUploadRow`、`media.openMediaRoot`、`media.reconcileUploadFile`、`media.writeChunkStage`、`media.openMediaRegular`、`media.chunkStageName`、`media.commitChunk`

## media.mediaWriteContext

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：公共有界写窗口核op/opened epoch，仅running非维护可写

```go
func (s *localBlobStore) mediaWriteContext(ctx context.Context,op OperationContext) (context.Context,context.CancelFunc,error)
```

**输入**

无

**输出**

无

**步骤**

1. 公共有界写窗口核op/opened epoch，仅running非维护可写

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`platform.NewWriteContext`

## media.hashChunkRequest

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：actor/site/epoch/upload/offset/length/SHA/CAS规范摘要，排除reader/时钟

```go
func hashChunkRequest(op OperationContext,input PutUploadChunkInput) (Hash,error)
```

**输入**

无

**输出**

无

**步骤**

1. actor/site/epoch/upload/offset/length/SHA/CAS规范摘要，排除reader/时钟

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`platform.CanonicalJSON`

## media.readChunkOperation

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：16MiB有界核阶段、材料与原UploadSession/hash及水位

```go
func (s *localBlobStore) readChunkOperation(ctx context.Context,q sqlExecer,id Identifier) (chunkOperation,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 16MiB有界核阶段、材料与原UploadSession/hash及水位

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.chunkContextReader.Read

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：向IO传播ctx取消，不重复入口期限机制

```go
func (r chunkContextReader) Read(buffer []byte) (int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 向IO传播ctx取消，不重复入口期限机制

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.chunkStageName

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：服务生成stage标识构造私有相对文件名

```go
func chunkStageName(id Identifier) string
```

**输入**

无

**输出**

无

**步骤**

1. 服务生成stage标识构造私有相对文件名

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.writeChunkStage

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：length+1有界读取，核精确长度/SHA，新文件及目录fsync后才准备

```go
func writeChunkStage(ctx context.Context,root *os.Root,input BinaryChunk) (Identifier,error)
```

**输入**

无

**输出**

无

**步骤**

1. length+1有界读取，核精确长度/SHA，新文件及目录fsync后才准备

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.chunkContextReader.Read`、`media.chunkStageName`

## media.reconcileUploadFile

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：短于SQL水位失败，未提交尾只在真实窗口截尾fsync；启动核已提交前缀

```go
func (s *localBlobStore) reconcileUploadFile(ctx context.Context,op OperationContext,root *os.Root,record storedUpload,verify bool) (*os.File,error)
```

**输入**

无

**输出**

无

**步骤**

1. 短于SQL水位失败，未提交尾只在真实窗口截尾fsync；启动核已提交前缀

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.openMediaRegular`、`media.mediaWriteContext`、`media.verifyUploadChunks`

## media.verifyUploadChunks

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：启动核连续offset/revision/来源及每段SHA，普通追加不反复重哈全文件

```go
func (s *localBlobStore) verifyUploadChunks(ctx context.Context,file *os.File,record storedUpload) error
```

**输入**

无

**输出**

无

**步骤**

1. 启动核连续offset/revision/来源及每段SHA，普通追加不反复重哈全文件

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.readChunkOperation`、`media.chunkContextReader.Read`

## media.commitChunk

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：追加fsync后重取running窗口/expiry，offset+revision CAS与COMMITTED原结果同事务

```go
func (s *localBlobStore) commitChunk(ctx context.Context,op OperationContext,input PutUploadChunkInput,record storedUpload,hash Hash) (gen.UploadSession,error)
```

**输入**

无

**输出**

无

**步骤**

1. 追加fsync后重取running窗口/expiry，offset+revision CAS与COMMITTED原结果同事务

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.mediaWriteContext`、`platform.CanonicalJSON`

## media.RecoverUploadChunks

- 级别：参考
- 文件：`modules/media/internal/chunk_writer.go`
- 作用：受信启动只按SQL确认水位核前缀/截尾，不自动计数PREPARED；旧NULL epoch保留不可续传

```go
func RecoverUploadChunks(ctx context.Context,op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 受信启动只按SQL确认水位核前缀/截尾，不自动计数PREPARED；旧NULL epoch保留不可续传

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.openMediaRoot`、`media.readUploadRow`、`media.reconcileUploadFile`

## media.CompleteUpload

- 级别：约束
- 文件：`modules/media/internal/upload_completion.go`
- 作用：完成hash/MIME验证后发布ready资源

```go
func CompleteUpload(ctx context.Context, op OperationContext, input CompleteUploadInput) (CompleteUploadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CompleteUploadInput` | 完成hash/MIME验证后发布ready资源的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CompleteUploadResult` | 完成hash/MIME验证后发布ready资源的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 当前media.upload本人管理会话与opened epoch先核；kind/caller/actor/site/epoch及原CompleteUploadInput规范摘要，operationId异材料CONFLICT
2. 原完成回执先于新写窗口/revision/expiry读取：来源upload owner/site/创建epoch与hash匹配后，只返回首次Asset快照或首次安全错误；不从当前mutable Asset重建，不要求被同站点去重资源的原creator等于本次上传者
3. 新操作核公共running写窗口、来源同站点owner/创建epoch、receiving/未到期、精确expectedRevision及receivedBytes==expectedBytes；上传互斥边界内先按SQL已提交chunk核前缀/截尾，再HashUpload核完整SHA/长度
4. SniffAssetType验证签名/扩展/类别与有界元数据；ValidateAssetPayload核SVG/scene。确定的VALIDATION/UNSUPPORTED_SCHEMA/UNSUPPORTED_CAPABILITY/INTEGRITY_ERROR用新写窗口和revision CAS，同事务记blocked/revision+1及原安全错误；依赖/取消/未知不猜作校验失败
5. CommitAssetBlob先PREPARED→安全普通暂存复制同时复核SHA/长度→file fsync/rename/两目录fsync；新写窗口再次核epoch/维护，单SQL事务CAS complete/revision、ready元数据、COMMITTED、原成功回执及新资源outbox。Commit未知留所有文件仍报错，同op查回执确认；无文件持久证明不ready

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久回执、来源、字节或元数据损坏（不可重试）
- `UNSUPPORTED_SCHEMA`：画板版本未知（不可重试）
- `UNSUPPORTED_CAPABILITY`：资源格式缺少必需能力（不可重试）
- `LOCKED`：新操作遇维护或屏障（可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：media.upload本人
- 并发：本域upload互斥；完成/拒绝以owner/site/epoch/receiving/revision/字节SQL CAS；每次持久副作用复用公共running窗口
- 幂等：既有upload_receipts中version=1/kind=complete_upload封套绑定固定原成功或安全错误；请求摘要含caller/actor/site/epoch；Begin与Chunk查同一操作绑定，不把封套当Begin结果
- 依赖：`media.validateUploadActor`、`media.hashCompleteRequest`、`media.replayCompletion`、`media.mediaWriteContext`、`media.reconcileUploadFile`、`media.HashUpload`、`media.SniffAssetType`、`media.ValidateAssetPayload`、`media.rejectCompletion`、`media.CommitAssetBlob`

## media.hashCompleteRequest

- 级别：参考
- 文件：`modules/media/internal/upload_completion.go`
- 作用：固定kind/caller/actor/site/epoch与原完成输入规范摘要

```go
func hashCompleteRequest(op OperationContext,input CompleteUploadInput)(Hash,error)
```

**输入**

无

**输出**

无

**步骤**

1. 固定kind/caller/actor/site/epoch与原完成输入规范摘要

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`platform.CanonicalJSON`

## media.replayCompletion

- 级别：参考
- 文件：`modules/media/internal/upload_completion.go`
- 作用：回放固定版本封套，来源授权/epoch/hash验证后返回原成功或原安全错误

```go
func (s *localBlobStore) replayCompletion(ctx context.Context,op OperationContext,input CompleteUploadInput,hash Hash)(CompleteUploadResult,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 回放固定版本封套，来源授权/epoch/hash验证后返回原成功或原安全错误

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.readUploadRow`、`media.completionRejectionCode`

## media.completionRejectionCode

- 级别：参考
- 文件：`modules/media/internal/upload_completion.go`
- 作用：确定可持久blocked的校验拒绝码白名单

```go
func completionRejectionCode(code string)bool
```

**输入**

无

**输出**

无

**步骤**

1. 确定可持久blocked的校验拒绝码白名单

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.rejectCompletion

- 级别：参考
- 文件：`modules/media/internal/upload_completion.go`
- 作用：新写窗口+revision CAS同事务blocked及首次安全错误

```go
func (s *localBlobStore) rejectCompletion(ctx context.Context,op OperationContext,input CompleteUploadInput,record storedUpload,hash Hash,problem *platform.Error)error
```

**输入**

无

**输出**

无

**步骤**

1. 新写窗口+revision CAS同事务blocked及首次安全错误

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.mediaWriteContext`、`media.casCompleteSession`、`media.writeCompletionReceipt`

## media.casCompleteSession

- 级别：参考
- 文件：`modules/media/internal/upload_completion.go`
- 作用：精确owner/site/epoch/receiving/revision/已收齐字节CAS完成或blocked状态

```go
func (s *localBlobStore) casCompleteSession(ctx context.Context,tx *sql.Tx,op OperationContext,input CompleteUploadInput,record storedUpload)error
```

**输入**

无

**输出**

无

**步骤**

1. 精确owner/site/epoch/receiving/revision/已收齐字节CAS完成或blocked状态

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.writeCompletionReceipt

- 级别：参考
- 文件：`modules/media/internal/upload_completion.go`
- 作用：在业务事务写version1完成封套与完整hash，不覆盖Begin原快照

```go
func writeCompletionReceipt(ctx context.Context,tx *sql.Tx,op OperationContext,uploadID Identifier,hash Hash,receipt completionReceipt)error
```

**输入**

无

**输出**

无

**步骤**

1. 在业务事务写version1完成封套与完整hash，不覆盖Begin原快照

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.ExpireUploadSessions

- 级别：参考
- 文件：`modules/media/internal/upload_maintenance.go`
- 作用：仅回收过期未完成临时字节

```go
func ExpireUploadSessions(ctx context.Context, op OperationContext, input ExpireUploadSessionsInput) (ExpireUploadSessionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ExpireUploadSessionsInput` | 仅回收过期未完成临时字节的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExpireUploadSessionsResult` | 仅回收过期未完成临时字节的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 只扫描receiving expiry，明确不扫描ready assets
2. 锁upload标expired/释放reservation
3. 删除服务生成tmp路径并journal；complete已ready不删除

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：receiving临时文件删除，ready不自动删除
- 授权：service:media maintenance
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：expired去重
- 依赖：无

## media.SniffAssetType

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：从字节识别类型

```go
func SniffAssetType(ctx context.Context, op OperationContext, input SniffAssetTypeInput) (SniffAssetTypeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `SniffAssetTypeInput` | 从字节识别类型的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SniffAssetTypeResult` | 从字节识别类型的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力 |

**步骤**

1. 已受信上传只经OpenUpload安全普通文件读取；附件一律application/octet-stream下载，忽略claimedMediaType；其他资源签名/类别与扩展不一致明确拒绝
2. PNG/JPEG/GIF/WebP/AVIF只读容器/尺寸/动画声明；单维1..100000，JPEG头读取≤16MiB，容器节点/块≤65536，BMFF box≤16384且嵌套≤12，非媒体载荷元数据≤16MiB；不解码像素或重编码
3. MP4只接受isom/iso2/iso4/iso5/iso6/mp41/mp42/avc1/M4V品牌及H.264/AV1/VP9视频和AAC/Opus样本声明；WebM仅webm DocType与VP8/VP9/AV1及Opus/Vorbis，禁止压缩/加密轨道，未知编码明确提示；不解码帧或调用ffmpeg
4. 返回确证MIME/kind/animated及图片width/height；SVG/scene/视频不伪造尺寸，交后续payload核语义

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `UNSUPPORTED_CAPABILITY`：当前依赖不提供声明的必需能力（不可重试）

- 副作用：无
- 授权：私有payload验证
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：纯读取
- 依赖：`media.OpenUpload`、`media.readUploadRow`、`media.inspectPNG`、`media.inspectGIF`、`media.inspectWebP`、`media.inspectBrands`、`media.inspectBMFF`、`media.inspectWebM`、`media.invalidMedia`

## media.invalidMedia

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：输出不含文件内容或路径的格式错误

```go
func invalidMedia(message string) error
```

**输入**

无

**输出**

无

**步骤**

1. 输出不含文件内容或路径的格式错误

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.inspectPNG

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界PNG块/尺寸/动画元数据

```go
func inspectPNG(ctx context.Context, r io.ReaderAt, size int64) (int,int,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界PNG块/尺寸/动画元数据

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.inspectGIF

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界GIF帧/子块/尺寸/动画元数据

```go
func inspectGIF(ctx context.Context,r io.ReaderAt,size int64) (int,int,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界GIF帧/子块/尺寸/动画元数据

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.inspectWebP

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界RIFF/WebP帧/尺寸/动画元数据

```go
func inspectWebP(ctx context.Context,r io.ReaderAt,size int64)(int,int,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界RIFF/WebP帧/尺寸/动画元数据

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.readMediaBox

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界BMFF box头及扩展长度

```go
func readMediaBox(r io.ReaderAt,offset,end int64)(mediaBox,error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界BMFF box头及扩展长度

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.inspectBrands

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界ftyp品牌白名单与AVIF动画标记

```go
func inspectBrands(r io.ReaderAt,size int64)(bool,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界ftyp品牌白名单与AVIF动画标记

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.inspectBMFF

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界BMFF层级/元数据及视频样本编码声明

```go
func inspectBMFF(ctx context.Context,r io.ReaderAt,size int64,avif bool)(int,int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界BMFF层级/元数据及视频样本编码声明

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.readMediaBox`、`media.validateMP4SampleEntry`

## media.readEBMLNumber

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：EBML ID/长度有界读取

```go
func readEBMLNumber(r io.ReaderAt,offset,end int64,id bool)(uint64,int64,error)
```

**输入**

无

**输出**

无

**步骤**

1. EBML ID/长度有界读取

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.inspectWebM

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界WebM DocType/轨道/编码/帧容器核验

```go
func inspectWebM(ctx context.Context,r io.ReaderAt,size int64)error
```

**输入**

无

**输出**

无

**步骤**

1. 有界WebM DocType/轨道/编码/帧容器核验

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.readEBMLNumber`、`media.validateWebMTrack`

## media.validateMP4SampleEntry

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：样本类型与对应avcC/av1C/vpcC/esds/dOps声明一致，配置≤64KiB，禁止只凭fourcc宣称编码

```go
func validateMP4SampleEntry(ctx context.Context,r io.ReaderAt,entry mediaBox)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 样本类型与对应avcC/av1C/vpcC/esds/dOps声明一致，配置≤64KiB，禁止只凭fourcc宣称编码

**错误**

- 无

- 副作用：无
- 授权：已授权私有payload校验链，不重新验入口身份/schema
- 并发：有界元数据读取，不解码压缩载荷
- 幂等：相同字节结果确定
- 依赖：`media.readMediaBox`、`media.validateAACDescriptor`

## media.readMP4Descriptor

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界MPEG-4描述符tag/最多4字节变长长度，精确payload与剩余边界

```go
func readMP4Descriptor(data []byte)(byte,[]byte,[]byte,error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界MPEG-4描述符tag/最多4字节变长长度，精确payload与剩余边界

**错误**

- 无

- 副作用：无
- 授权：已授权私有payload校验链，不重新验入口身份/schema
- 并发：有界元数据读取，不解码压缩载荷
- 幂等：相同字节结果确定
- 依赖：无

## media.validateAACDescriptor

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：有界ES/DecoderConfig/DecoderSpecificInfo声明，OTI=0x40且音频streamType=5，不接受外部URL

```go
func validateAACDescriptor(data []byte)error
```

**输入**

无

**输出**

无

**步骤**

1. 有界ES/DecoderConfig/DecoderSpecificInfo声明，OTI=0x40且音频streamType=5，不接受外部URL

**错误**

- 无

- 副作用：无
- 授权：已授权私有payload校验链，不重新验入口身份/schema
- 并发：有界元数据读取，不解码压缩载荷
- 幂等：相同字节结果确定
- 依赖：`media.readMP4Descriptor`、`media.validateAACConfig`

## media.validateAACConfig

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：AudioSpecificConfig只接受AAC LC/HE/HEv2及有效采样频率/声道/核心对象类型

```go
func validateAACConfig(data []byte)error
```

**输入**

无

**输出**

无

**步骤**

1. AudioSpecificConfig只接受AAC LC/HE/HEv2及有效采样频率/声道/核心对象类型

**错误**

- 无

- 副作用：无
- 授权：已授权私有payload校验链，不重新验入口身份/schema
- 并发：有界元数据读取，不解码压缩载荷
- 幂等：相同字节结果确定
- 依赖：无

## media.validateWebMTrack

- 级别：参考
- 文件：`modules/media/internal/mime_validator.go`
- 作用：每个TrackEntry同时核TrackType与CodecID、拒未知/音轨冒充视频及加密压缩轨道

```go
func validateWebMTrack(ctx context.Context,r io.ReaderAt,start,end int64)(bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 每个TrackEntry同时核TrackType与CodecID、拒未知/音轨冒充视频及加密压缩轨道

**错误**

- 无

- 副作用：无
- 授权：已授权私有payload校验链，不重新验入口身份/schema
- 并发：有界元数据读取，不解码压缩载荷
- 幂等：相同字节结果确定
- 依赖：`media.readEBMLNumber`

## media.ValidateAssetPayload

- 级别：参考
- 文件：`modules/media/internal/payload_validator.go`
- 作用：验证SVG/场景/附件/视频安全交付

```go
func ValidateAssetPayload(ctx context.Context, op OperationContext, input ValidateAssetPayloadInput) (ValidateAssetPayloadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidateAssetPayloadInput` | 验证SVG/场景/附件/视频安全交付的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateAssetPayloadResult` | 验证SVG/场景/附件/视频安全交付的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 附件只接受application/octet-stream，后续交付必须attachment+nosniff；非SVG/scene字节类型已由SniffAssetType验证，不重复全链身份/schema
2. SVG≤16MiB、节点≤200000/深度≤128有界XML，拒DTD/entity/处理指令、脚本/事件/foreignObject、未知元素与网络资源；href仅本地片段、受限raster data URL或安全a链接，样式仅本地片段及≤2MiB内嵌woff2，拒混淆/网络载入
3. SVG原字节保持且返回cspSandbox=true，交付必须sandbox+nosniff且不内联；scene≤64MiB按ValidateDrawingScene验证，保留全部受支持字段；服务不处理像素/视频帧或转码

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_SCHEMA`：版本或块 schema 不受支持；禁止静默丢弃未知属性（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：私有payload验证
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`media.OpenUpload`、`media.ValidateDrawingScene`、`media.validateSVG`

## media.validateSVG

- 级别：参考
- 文件：`modules/media/internal/payload_validator.go`
- 作用：有界严格SVG XML、元素/属性/资源安全语义

```go
func validateSVG(ctx context.Context, reader io.Reader) error
```

**输入**

无

**输出**

无

**步骤**

1. 在既有SVG字节、节点和深度预算内解析XML，核元素、属性、引用与嵌入raster；拒DTD、处理指令、未知元素及不安全外部资源
2. style仅允许文本；合并其全部XML CharData片段（包含CDATA展开、忽略XML注释）后，在style结束时一次调用validateSVGCSS，不能按单个Token判定CSS关键字/URL；各style独立重置缓冲
3. 保持原SVG字节不变，不执行样式或抓取URL；原子上传和交付CSP仍由调用链负责

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.validateSVGCSS`、`media.decodeDrawingRaster`、`media.validateDrawingRaster`、`media.safeDrawingLink`

## media.validateSVGCSS

- 级别：参考
- 文件：`modules/media/internal/payload_validator.go`
- 作用：SVG样式本地引用与受限字体白名单，拒混淆网络语法

```go
func validateSVGCSS(value string,style bool)error
```

**输入**

无

**输出**

无

**步骤**

1. SVG样式本地引用与受限字体白名单，拒混淆网络语法

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.safeDrawingLink

- 级别：参考
- 文件：`modules/media/internal/payload_validator.go`
- 作用：只允许http/https/mailto安全链接

```go
func safeDrawingLink(value string)bool
```

**输入**

无

**输出**

无

**步骤**

1. 只允许http/https/mailto安全链接

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.CommitAssetBlob

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：将已验证文件提交不可变内容寻址存储

```go
func CommitAssetBlob(ctx context.Context, op OperationContext, input CommitAssetBlobInput) (CommitAssetBlobResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CommitAssetBlobInput` | 将已验证文件提交不可变内容寻址存储的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitAssetBlobResult` | 将已验证文件提交不可变内容寻址存储的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅CompleteUpload持锁且完整校验后调用，信任input/op，不重新解析deadline或schema
2. CommitVerifiedBlob复核文件SHA/长度并完成PREPARED、不可变文件持久化及ready/session/receipt/COMMITTED/outbox同事务；未知提交保留字节

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：不可变blob/提交journal
- 授权：已授权本域CompleteUpload持锁链，HTTP不可直接调用
- 并发：(site,sha)mutex+唯一blobkey
- 幂等：同SHA/bytes复用，异bytes完整性拒绝
- 依赖：`media.CommitVerifiedBlob`

## media.InstallLocalBlobStore

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：受信配置安装私有SQLite与opened epoch

```go
func InstallLocalBlobStore(dataRoot string, openedEpoch Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 受信配置安装私有SQLite与opened epoch

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.openLocalBlobStore`

## media.openLocalBlobStore

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：目录/DB拒链接特殊对象，FULL同步只追加迁移，唯一生成schema校验持久Asset/UploadSession

```go
func openLocalBlobStore(activeRoot string, openedEpoch Identifier) (*localBlobStore, error)
```

**输入**

无

**输出**

无

**步骤**

1. 目录/DB拒链接特殊对象，FULL同步只追加迁移，唯一生成schema校验持久Asset/UploadSession
2. media.db打开前允许缺失、Ping后必须存在，复用ValidatePrivatePath拒硬链接/链接/特殊文件，并核既有DB前后SameFile
3. 装配CleanupApproval与jobs CreateJobResult唯一schema校验器，不手工复制跨域校验规则
4. 运行校验仅使用x-runtime-payload-schemas登记并由make gen生成的PayloadValidationSpec自包含闭包，不从已被HTTP prune的GetSwagger读取孤立组件，不手写或复制schema；字段仍唯一来自contracts

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.ensureMediaDirectory`、`media.All`、`platform.NewPayloadValidator`、`platform.ValidatePrivatePath`

## media.readUploadRow

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：16MiB有界读取唯一UploadSession schema，核索引/创建主体及水位

```go
func readUploadRow(ctx context.Context, exec sqlExecer, uploadID Identifier,validate func(json.RawMessage)error) (storedUpload, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 16MiB有界读取唯一UploadSession schema，核索引/创建主体及水位

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.writeUploadRow

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：新上传持久创建epoch，更新不改归属

```go
func writeUploadRow(ctx context.Context, exec sqlExecer, record storedUpload, insert bool) error
```

**输入**

无

**输出**

无

**步骤**

1. 新上传持久创建epoch，更新不改归属

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.ensureMediaDirectory

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：逐段Lstat拒链接/非目录，创建后同步父目录

```go
func ensureMediaDirectory(path string) error
```

**输入**

无

**输出**

无

**步骤**

1. 逐段Lstat拒链接/非目录，创建后同步父目录

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.openMediaRoot

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：标准库目录能力约束私有路径，上传/staging父目录拒链接

```go
func (s *localBlobStore) openMediaRoot() (*os.Root,error)
```

**输入**

无

**输出**

无

**步骤**

1. 标准库目录能力约束私有路径，上传/staging父目录拒链接

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.ensureMediaDirectory`

## media.openMediaRegular

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：普通文件NOFOLLOW/NONBLOCK并核打开前后身份

```go
func openMediaRegular(root *os.Root, absoluteRoot, name string, flags int) (*os.File, error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用ValidatePrivatePath于受信绝对root+相对生成路径的打开前后，拒硬链接/符号链接/特殊对象；保留Root/fd/绝对路径SameFile身份核对与NOFOLLOW，不复制公共路径规则

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`platform.ValidatePrivatePath`

## media.ConfigureWriteWindow

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：仅受信装配真实Runtime/WriteWindow，缺回调失败关闭

```go
func ConfigureWriteWindow(config platform.WriteWindowConfig) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅受信装配真实Runtime/WriteWindow，缺回调失败关闭

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.CloseLocalBlobStore

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：装配失败或HTTP排空后撤下并关闭store

```go
func CloseLocalBlobStore() error
```

**输入**

无

**输出**

无

**步骤**

1. 装配失败或HTTP排空后撤下并关闭store

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.LocalStorageRecovered

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：真实blob/chunk恢复完成且epoch匹配才报告journal恢复

```go
func LocalStorageRecovered(epoch Identifier) bool
```

**输入**

无

**输出**

无

**步骤**

1. 真实blob/chunk恢复完成且epoch匹配才报告journal恢复

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.replayBeginUpload

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：owner/site/创建epoch及原快照/hash一致才只读回放，旧缺快照不猜mutable结果

```go
func (s *localBlobStore) replayBeginUpload(ctx context.Context,op OperationContext,q sqlExecer,requestHash Hash) (gen.UploadSession,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. owner/site/创建epoch及原快照/hash一致才只读回放，旧缺快照不猜mutable结果

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.readUploadRow`

## media.CreateUpload

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：复查原回执后真实窗口/配额，普通新临时文件fsync，session/epoch/原快照同事务

```go
func (s *localBlobStore) CreateUpload(ctx context.Context,op OperationContext,session gen.UploadSession,quota uploadQuota,requestHash Hash) (gen.UploadSession,error)
```

**输入**

无

**输出**

无

**步骤**

1. 复查原回执后真实窗口/配额，普通新临时文件fsync，session/epoch/原快照同事务
2. 调用SQL Commit前标记保留候选文件；Commit错误继续返回错误但不删可能已提交对象，同op以持久原回执确认，不把未知当成功
3. 站点存储预留统计receiving与blocked的expected_bytes，blocked字节未明确清理前不释放；上传并发数只计receiving

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.replayBeginUpload`、`media.mediaWriteContext`、`media.openMediaRoot`、`media.writeUploadRow`、`platform.CanonicalJSON`

## media.OpenUpload

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：私有根目录能力/NOFOLLOW/公共硬链接及inode核验读取临时对象

```go
func (s *localBlobStore) OpenUpload(ctx context.Context, uploadID Identifier) (ioReadCloser, error)
```

**输入**

无

**输出**

无

**步骤**

1. 私有根目录能力/NOFOLLOW/公共硬链接及inode核验读取临时对象

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.openMediaRoot`、`media.openMediaRegular`

## media.CommitVerifiedBlob

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：持上传锁，PREPARED/file/rename/fsync后，同事务CAS complete、ready、原结果、COMMITTED与outbox

```go
func (s *localBlobStore) CommitVerifiedBlob(ctx context.Context,op OperationContext,input CommitAssetBlobInput)(gen.Asset,error)
```

**输入**

无

**输出**

无

**步骤**

1. 调用方已持store互斥，同站点同SHA任意未完成DELETING存在时LOCKED，禁止重建或复用可能unlink的bytes；终局后可创建新assetId
2. 持上传锁，PREPARED/file/rename/fsync后，同事务CAS complete、ready、原结果、COMMITTED与outbox
3. 创建新ready资源的同事务建立reference_objects逐对象fence=0/referenceGeneration=0；既有迁移只在0003首次应用时回填，不为丢失fence的旧对象运行时补造

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.readUploadRow`、`media.openMediaRoot`、`media.mediaWriteContext`、`media.ensureMediaDirectory`、`media.openMediaRegular`、`media.findReadyAsset`、`media.casCompleteSession`、`media.writeCompletionReceipt`、`media.PublishAssetEvent`

## media.findReadyAsset

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：精确同站点/hash/bytes/MIME/kind选择ready资源；16MiB有界原元数据、唯一Asset schema及SQL主键/索引一致后复用

```go
func (s *localBlobStore) findReadyAsset(ctx context.Context, exec sqlExecer, siteID Identifier, sha256Hash Hash, byteSize int, mediaType string, kind gen.AssetKind) (gen.Asset, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 精确同站点/hash/bytes/MIME/kind选择ready资源；16MiB有界原元数据、唯一Asset schema及SQL主键/索引一致后复用

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.hashFile

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：恢复时对私有普通文件流算SHA/长度；复用公共ValidatePrivatePath拒链接/硬链接，NOFOLLOW及打开前后inode身份一致，未知失败关闭

```go
func hashFile(path string) (Hash, int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取前公共ValidatePrivatePath及Lstat普通文件
2. NOFOLLOW打开并核fd与命名路径SameFile，固定缓冲流算SHA与长度
3. 读取后再次公共私有路径与inode核验，禁止被换路径或硬链接的字节通过恢复确认

**错误**

- 无

- 副作用：无
- 授权：仅本域受信恢复链生成路径，不接收HTTP路径
- 并发：恢复入口开放前独占；文件身份及SHA核对
- 幂等：相同私有文件确定结果；损坏或身份变化拒绝
- 依赖：`platform.ValidatePrivatePath`

## media.currentBlobStore

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：在共享读锁下取得构造期装配的本域存储接口

```go
func currentBlobStore() blobStore
```

**输入**

无

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `blobStore` | 当前本域私有上传字节读取能力，未装配为nil | 仅本域可信调用链使用，不序列化到HTTP |

**步骤**

1. 取得activeBlobStoreMu读锁并返回当前activeBlobStore；不造缺失存储、不执行IO

**错误**

- 无

- 副作用：无
- 授权：仅本域可信实现调用，不建立跨服务权限
- 并发：activeBlobStoreMu共享读锁
- 幂等：只读当前接口引用
- 依赖：无

## media.appendJournal

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：同事务写本域PREPARED/COMMITTED/DELETING最小持久日志

```go
func appendJournal(ctx context.Context, exec sqlExecer, event, siteID, uploadID Identifier, sha256Hash Hash, assetID Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 同事务写本域PREPARED/COMMITTED/DELETING最小持久日志

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链，仅服务生成路径及材料
- 并发：调用方维护业务锁与事务
- 幂等：原日志与文件步骤重复安全，未知保留材料
- 依赖：无

## media.blobPath

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：仅受信siteId/hash构造不可变blob路径

```go
func (s *localBlobStore) blobPath(siteID Identifier, sha256Hash Hash) string
```

**输入**

无

**输出**

无

**步骤**

1. 仅受信siteId/hash构造不可变blob路径

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链，仅服务生成路径及材料
- 并发：调用方维护业务锁与事务
- 幂等：原日志与文件步骤重复安全，未知保留材料
- 依赖：无

## media.syncDirectory

- 级别：参考
- 文件：`modules/media/internal/blob_repository.go`
- 作用：fsync服务生成目录，失败不能记文件步骤完成

```go
func syncDirectory(path string) error
```

**输入**

无

**输出**

无

**步骤**

1. fsync服务生成目录，失败不能记文件步骤完成

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链，仅服务生成路径及材料
- 并发：调用方维护业务锁与事务
- 幂等：原日志与文件步骤重复安全，未知保留材料
- 依赖：无

## media.ReadAsset

- 级别：约束
- 文件：`modules/media/internal/asset_service.go`
- 作用：读取资源/variants安全元数据

```go
func ReadAsset(ctx context.Context, op OperationContext, input ReadAssetInput) (ReadAssetResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadAssetInput` | 读取资源/variants安全元数据的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadAssetResult` | 读取资源/variants安全元数据的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅当前media.read/upload/manage管理会话或静态content/site-config/public-site/localization/knowledge纯服务；域入口当前授权，用户按返回siteId核同站点
2. 核op.dataEpoch与本域受信打开active代次相同；有界读真实assets行，按唯一持久Asset schema校验且索引/id/site/sha/size/type/state一致
3. 只返回ready安全DTO，无路径/内部句柄；deleted/notready为NOT_FOUND，未知持久状态或坏元数据INTEGRITY_ERROR
4. 省略version为当前；提供严格非负Revision且必须等于真实不可变ref.version；当前库仅一版本，不存在历史版本NOT_FOUND，不伪造历史；注册服务调用域仍核返回siteId，不将元数据读取当发布授权

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久元数据损坏或与索引不符（不可重试）
- `VERSION_MISMATCH`：非当前已打开active代次（不可重试）

- 副作用：无
- 授权：media.read或注册引用/检索service
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`platform.NewPayloadValidator`

## media.ListAssets

- 级别：约束
- 文件：`modules/media/internal/asset_service.go`
- 作用：资源库按类别列真实安全元数据，引用详情独立读取

```go
func ListAssets(ctx context.Context, op OperationContext, input ListAssetsInput) (ListAssetsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ListAssetsInput` | 资源库按类别列安全元数据的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListAssetsResult` | 资源库按类别列安全元数据的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 当前本站media.manage管理员，cleanupStore核opened epoch，受信启动已装配原服务签名key；输入边界严格limit/kind，缺省limit归一为20
2. 平台VerifyCursor在SQL外校验专用scope绑定caller/actor/site/epoch/session及权限修订/limit/kind/普通或unused类别；严格首版规范载荷、排序位置和int64本地水位，未知或串用拒绝
3. 短只读SQL快照读取独立asset_listing_state与最多limit+1条真实Asset/fence/generation，按kind/createdAt/assetId稳定keyset；当前站点及kind筛选，不从上传会话合成资源。游标水位变化要求从头分页，schema/索引/对象水位损坏失败关闭
4. 只读快照显式结束后平台SignCursor签发下一页；返回原安全Asset，不泄露私有路径/句柄，不增加虚构引用摘要，引用详情独立ReadAssetReferences
5. 不调用模型、批准、worker、清理或写入业务状态，列表从不代替人工删除授权

**错误**

- `FORBIDDEN`：非当前本站media.manage管理员（不可重试）
- `VALIDATION`：游标签名/scope/版本或严格查询参数不合法（不可重试）
- `VERSION_MISMATCH`：opened epoch、本地或引用来源水位改变，须重头分页（可重试）
- `INTEGRITY_ERROR`：持久Asset/索引/fence/generation或权威材料损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储/密钥/权威来源未知，或风险响应预算超限（可重试）

- 副作用：无
- 授权：当前本站media.manage或superadmin管理员，HTTP仅portal；每页经平台当前授权，游标仅位置不代替权限
- 并发：本地短只读一致快照；签名密钥IO在SQL外，无asset/store锁
- 幂等：同查询同水位稳定keyset，无业务写入
- 依赖：`media.assetListingStore`、`media.readAssetListingCursor`、`media.readAssetListingPage`、`media.assetListingPageInfo`

## media.InstallAssetListingKey

- 级别：参考
- 文件：`modules/media/internal/asset_service.go`
- 作用：可信启动装配已验证服务签名key，仅存内存，不持久化私钥引用

```go
func InstallAssetListingKey(key platform.SigningKey) error
```

**输入**

无

**输出**

无

**步骤**

1. 可信启动装配已验证服务签名key，仅存内存，不持久化私钥引用

**错误**

- 无

- 副作用：仅内存装配已验证签名key引用，不进入active业务库或日志
- 授权：可信启动或已授权本站列表调用链；入口schema/身份一次
- 并发：短只读快照，所有RPC与密钥IO在SQL/业务锁外
- 幂等：保留精确水位与来源，不重置、不产生删除许可
- 依赖：无

## media.assetListingStore

- 级别：参考
- 文件：`modules/media/internal/asset_service.go`
- 作用：核当前本站media.manage/opened epoch及真实列表签名依赖

```go
func assetListingStore(op OperationContext) (*localBlobStore, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核当前本站media.manage/opened epoch及真实列表签名依赖

**错误**

- 无

- 副作用：无
- 授权：可信启动或已授权本站列表调用链；入口schema/身份一次
- 并发：短只读快照，所有RPC与密钥IO在SQL/业务锁外
- 幂等：保留精确水位与来源，不重置、不产生删除许可
- 依赖：`media.cleanupStore`

## media.readAssetListingCursor

- 级别：参考
- 文件：`modules/media/internal/asset_service.go`
- 作用：SQL外平台验签，完整scope绑定与首版规范游标载荷校验

```go
func (s *localBlobStore) readAssetListingCursor(op OperationContext, input ListAssetsInput, unused bool) (assetListingCursor, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. SQL外平台验签，完整scope绑定与首版规范游标载荷校验

**错误**

- 无

- 副作用：无
- 授权：可信启动或已授权本站列表调用链；入口schema/身份一次
- 并发：短只读快照，所有RPC与密钥IO在SQL/业务锁外
- 幂等：保留精确水位与来源，不重置、不产生删除许可
- 依赖：`platform.VerifyCursor`

## media.readAssetListingRevision

- 级别：参考
- 文件：`modules/media/internal/asset_service.go`
- 作用：读取独立真实int64列表水位，缺失/损坏不补造

```go
func readAssetListingRevision(ctx context.Context, q sqlExecer) (int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取独立真实int64列表水位，缺失/损坏不补造

**错误**

- 无

- 副作用：无
- 授权：可信启动或已授权本站列表调用链；入口schema/身份一次
- 并发：短只读快照，所有RPC与密钥IO在SQL/业务锁外
- 幂等：保留精确水位与来源，不重置、不产生删除许可
- 依赖：无

## media.readAssetListingPage

- 级别：参考
- 文件：`modules/media/internal/asset_service.go`
- 作用：短一致只读水位/Asset/fence/generation与有界lookahead，结束快照后才允许RPC/签名

```go
func (s *localBlobStore) readAssetListingPage(ctx context.Context, op OperationContext, input ListAssetsInput, position assetListingCursor, unused bool) (assetListingPage, error)
```

**输入**

无

**输出**

无

**步骤**

1. 短一致只读水位/Asset/fence/generation与有界lookahead，结束快照后才允许RPC/签名

**错误**

- 无

- 副作用：无
- 授权：可信启动或已授权本站列表调用链；入口schema/身份一次
- 并发：短只读快照，所有RPC与密钥IO在SQL/业务锁外
- 幂等：保留精确水位与来源，不重置、不产生删除许可
- 依赖：`media.readAssetListingRevision`

## media.assetListingPageInfo

- 级别：参考
- 文件：`modules/media/internal/asset_service.go`
- 作用：用最后扫描候选产生严格前进keyset，SQL外平台签名，不把空匹配误作末页

```go
func (s *localBlobStore) assetListingPageInfo(scope string, position assetListingCursor, page assetListingPage) (gen.PageInfo, error)
```

**输入**

无

**输出**

无

**步骤**

1. 用最后扫描候选产生严格前进keyset，SQL外平台签名，不把空匹配误作末页

**错误**

- 无

- 副作用：无
- 授权：可信启动或已授权本站列表调用链；入口schema/身份一次
- 并发：短只读快照，所有RPC与密钥IO在SQL/业务锁外
- 幂等：保留精确水位与来源，不重置、不产生删除许可
- 依赖：`platform.SignCursor`

## media.AttachImageVariants

- 级别：约束
- 文件：`modules/media/internal/image_variants.go`
- 作用：关联浏览器多宽WebP原图

```go
func AttachImageVariants(ctx context.Context, op OperationContext, input AttachImageVariantsInput) (AttachImageVariantsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `AttachImageVariantsInput` | 关联浏览器多宽WebP原图的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AttachImageVariantsResult` | 关联浏览器多宽WebP原图的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配 |

**步骤**

1. 核所有ready同site、SHA一致、variant为静态WebP
2. 宽度最多640/1280/1920且不超过原尺寸，动态GIF/WebP/AVIF拒绝variants保持原图
3. 事务关联exact refs，用于srcset；服务不生成/重编码

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：media.upload
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：无

## media.ValidateDrawingScene

- 级别：参考
- 文件：`modules/media/internal/drawing_validator.go`
- 作用：校验Excalidraw场景和提取文字

```go
func ValidateDrawingScene(ctx context.Context, op OperationContext, input ValidateDrawingSceneInput) (ValidateDrawingSceneResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidateDrawingSceneInput` | 校验Excalidraw场景和提取文字的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateDrawingSceneResult` | 校验Excalidraw场景和提取文字的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 原DrawingScene schema经OpenAPI components引用进入唯一生成PayloadValidationSpec，由平台NewPayloadValidator完整验证；版本只接受2，未知字段/类型/坐标原schema拒绝，files等价additionalProperties值schema+propertyNames模式不改变持久版本
2. Drawing专属语义核文件键原精确^[A-Za-z0-9._:-]{1,128}$与key/id一致、元素ID唯一、frame/bound/container/binding/file存在与类型、frame无环、group无重复、freedraw压力长度与points一致
3. files只受限raster base64 data URL≤32MiB，MIME与签名及元数据一致，禁止SVG/外部URL；元素链接仅http/https/mailto，不执行HTML；按原element顺序收非deleted text，≤1048576字符，不写日志
4. 运行校验仅使用x-runtime-payload-schemas登记并由make gen生成的PayloadValidationSpec自包含闭包，不从已被HTTP prune的GetSwagger读取孤立组件，不手写或复制schema；字段仍唯一来自contracts

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_SCHEMA`：版本或块 schema 不受支持；禁止静默丢弃未知属性（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：私有asset验证
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：纯验证
- 依赖：`platform.NewPayloadValidator`、`media.decodeDrawingRaster`、`media.validateDrawingRaster`、`media.safeDrawingLink`

## media.decodeDrawingRaster

- 级别：参考
- 文件：`modules/media/internal/drawing_validator.go`
- 作用：精确raster MIME/base64 data URL与大小预算

```go
func decodeDrawingRaster(value,expected string)([]byte,string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 精确raster MIME/base64 data URL与大小预算

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：无

## media.validateDrawingRaster

- 级别：参考
- 文件：`modules/media/internal/drawing_validator.go`
- 作用：嵌入raster真实签名与有界尺寸/格式一致

```go
func validateDrawingRaster(ctx context.Context,data []byte,mediaType string)error
```

**输入**

无

**输出**

无

**步骤**

1. 嵌入raster真实签名与有界尺寸/格式一致

**错误**

- 无

- 副作用：无
- 授权：本域已授权上传链；入口schema/身份一次，边界负责来源/epoch/CAS/持久完整性
- 并发：调用方持上传互斥；SQL事务精确CAS；纯规则不重复ctx校验
- 幂等：原材料/原结果稳定，未知失败关闭
- 依赖：`media.inspectPNG`、`media.inspectGIF`、`media.inspectWebP`、`media.inspectBrands`、`media.inspectBMFF`

## media.AcquireResourceReferences

- 级别：约束
- 文件：`modules/media/internal/reference_service.go`
- 作用：正文/config提交前保护资源迟到引用

```go
func AcquireResourceReferences(ctx context.Context, op OperationContext, input AcquireResourceReferencesInput) (AcquireResourceReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `AcquireResourceReferencesInput` | 正文/config提交前保护资源迟到引用的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcquireResourceReferencesResult` | 正文/config提交前保护资源迟到引用的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 固定注册content/site-config已认证服务；siteId与每个asset权威站点匹配，顶层sourceObjectId/sourceRevisionId/sourceVersion逐一匹配完整references元素，sourceService取caller不取冒名字段
2. 完整引用数组按referenceId、assetId、asset.version、asset.sha256排序，空集合固定[]；用platform.CanonicalJSON编码后SHA256等于referenceDigest；输入pending_pin/confirmed=false/expiresAt=null，媒体自己确定pin到期
3. 按caller/dataEpoch/sourceOperationId先取源互斥，核已绑定所有顶层源字段和digest；明确sealed终止来源返回CONFLICT且不创建pin；未封存的相同成功返回原pinId/原expiresAt不续期，异材料冲突
4. 保持源互斥，先在无asset锁状态展开相交有效人工计划的完整资产集合，再按assetId稳定锁及store互斥；SQL内重核计划代次/集合，已有DELETING或未知delete_pending失败关闭，已知可撤销计划整份cancelled/解除标记与新pin同事务；资源站点/精确版本/hash/长度/MIME及fence完整，CAS fence/generation并保存至少24h原pin，不推测缺失材料
5. 即使references为空也生成绑定源身份的pin，以后confirm推进该对象完整集合并保留旧history；不以空数组跳过提交协议
6. ReferencePin.assets按assetId唯一升序；deletionFence仅为本次逐asset新fence的最大值（空集合0），不是全局删除能力；原pin回放只核原材料/回执不查当前asset状态，不续期、不重复升generation

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `CONFLICT`：同源操作不同材料，或来源已经明确终止封存（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域存储或运行写窗口不可用（可重试）
- `VERSION_MISMATCH`：操作epoch不同于已打开active代次（不可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：仅已认证content/site-config纯服务，Actor.SubjectID必须等于caller；来源站点来自已绑定input.siteId，不从system服务Actor.SiteID推测
- 并发：源caller/epoch/sourceOperationId互斥先于assetId稳定顺序mutex；Acquire/Confirm/Release同序，最终delete只取asset锁/fence
- 幂等：同源完整材料未sealed时回原pin；sealed检查先于任何旧成功快路径，终止后不再创建或返回可提交pin
- 依赖：`platform.CanonicalJSON`、`media.validateReferenceSource`、`media.canonicalReferenceInput`、`media.lockReferenceKey`、`media.lockReferenceAssets`、`media.readReferenceSource`、`media.acquireReferencePin`、`media.mediaWriteContext`、`media.referenceCleanupAssets`

## media.ConfirmResourceReferences

- 级别：约束
- 文件：`modules/media/internal/reference_service.go`
- 作用：源提交后确认pin为引用

```go
func ConfirmResourceReferences(ctx context.Context, op OperationContext, input ConfirmResourceReferencesInput) (ConfirmResourceReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ConfirmResourceReferencesInput` | 源提交后确认pin为引用的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConfirmResourceReferencesResult` | 源提交后确认pin为引用的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 固定content/site-config纯服务及已打开epoch；从pinId查真实完整来源，caller/源修订/水位必须与原acquire一致，原pin/原资产保护完整；已confirmed只核持久原终局回执，不重复generation
2. 不持source/对象/store/SQL锁回注册源最小决定；严格公共ReferenceSourceDecision schema、当前epoch及全部site/object/revision/version/operation/digest一致，必须committed，404/pending/未知保持pin并失败关闭
3. 回源后先以公共NewWriteContext重新取running窗口；sourceOperation互斥→sourceObject水位互斥→旧current与新pin资产唯一升序锁→store提交互斥→SQL；重读源绑定/原pin/sealed/终局及current水位
4. 同事务持久新完整集合与confirmed原回执，较高sourceVersion推进对象水位并将旧current保留history，较低已提交版本只补history不回退；相同水位不同源材料完整性冲突，空集合仍推进并保留旧history
5. 新引用须精确版本/hash/长度/MIME/站点及fence完整；先在无asset锁状态展开相交计划完整锁集合，SQL内核计划代次/集合并与确认同事务取消已知可撤销计划、恢复ready；未知delete_pending或已DELETING失败关闭。受影响asset fence/generation只提升一次，耗尽LOCKED；confirmed后原pin_assets永久保留供Acquire合法回放，pending保护由source.state决定
6. 来源sealed却源证明COMMITTED是完整性矛盾，保持封存；不因pin到期丢已提交历史；未实现的真实清理计划取消仍不宣称完成

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：源持久决定未知或依赖失败，保留pin等待恢复（可重试）
- `CONFLICT`：来源材料、pin或终止reason冲突（不可重试）
- `NOT_FOUND`：没有真实来源pin（不可重试）
- `VERSION_MISMATCH`：来源或已打开epoch不同（不可重试）
- `LOCKED`：维护、引用fence/generation耗尽或未知删除状态（可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：service对应source
- 并发：所有RPC不持业务锁；sourceOperation→sourceObject→旧current与新pin资产升序→store→SQL CAS
- 幂等：同pin同revision复用，异源CONFLICT
- 依赖：`media.validateReferenceSource`、`media.readReferenceRecord`、`media.readReferenceDecision`、`media.readReferenceResolution`、`media.confirmReferencePin`、`media.mediaWriteContext`

## media.ReleaseResourceReferences

- 级别：约束
- 文件：`modules/media/internal/reference_service.go`
- 作用：明确源未提交后释放或确认无pin并持久封存来源

```go
func ReleaseResourceReferences(ctx context.Context, op OperationContext, input ReleaseResourceReferencesInput) (ReleaseResourceReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReleaseResourceReferencesInput` | 仅明确源未提交才释放pin的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseResourceReferencesResult` | 仅明确源未提交才释放pin的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 固定content/site-config纯服务和已打开epoch；完整site/object/revision/version/operation/digest规范绑定；已知pinId只核真实原pin一致，不授权也不证明无pin，null仅未知提示
2. 同材料已sealed只核原reason/原终局回执；其他情况不持source/object/store/SQL锁回固定源API，严格共享schema且当前epoch/完整材料一致，必须reason对应aborted/rejected；committed/current/history拒绝，pending/404/未知/到期均保留
3. 回源后先重新公共running窗口，再sourceOperation→sourceObject→原pin资产稳定序→store→SQL；重核来源绑定/confirmed或sealed和终局回执，不能释放确认引用或以调用方hint造出无pin
4. 同事务持久sealed完整来源/reason/原安全回执，并由状态退出pending保护、逐原pin资产generation提升；从未有pin也持久sealed，不查当前asset ready来证明无pin，不造pin；原pin_assets和原acquire快照保留不可变
5. Acquire与Release同source锁与事务CAS，封存后所有迟到Acquire失败；只有事务成功才返回released=true，未知Commit报错并以同完整材料读原回执确认，不删除封存证据

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `NOT_FOUND`：没有真实来源pin（不可重试）
- `VERSION_MISMATCH`：来源或已打开epoch不同（不可重试）
- `LOCKED`：维护、引用fence/generation耗尽或未知删除状态（可重试）
- `INTEGRITY_ERROR`：源决定、原材料/回执或引用水位矛盾（不可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：service对应source
- 并发：所有RPC不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL CAS
- 幂等：同源完整材料+终止reason持久封存并重复成功，pinId可null；异材料冲突，无pin也不能省略来源核验
- 依赖：`media.validateReferenceSource`、`media.referenceBindingHash`、`media.readReferenceRecord`、`media.readReferenceDecision`、`media.readReferenceResolution`、`media.releaseReferencePin`、`media.mediaWriteContext`

## media.validateReferenceSource

- 级别：参考
- 文件：`modules/media/internal/reference_service.go`
- 作用：只接受固定content/site-config纯服务及相同SubjectID

```go
func validateReferenceSource(op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 只接受固定content/site-config纯服务及相同SubjectID

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：无

## media.canonicalReferenceInput

- 级别：参考
- 文件：`modules/media/internal/reference_service.go`
- 作用：核完整来源/pending元数据和重复条目，按referenceId/asset/version/hash固定排序与空数组，核原digest，再对caller/epoch/完整input规范摘要

```go
func canonicalReferenceInput(op OperationContext, input AcquireResourceReferencesInput) (AcquireResourceReferencesInput, Hash, []gen.AssetVersionRef, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核完整来源/pending元数据和重复条目，按referenceId/asset/version/hash固定排序与空数组，核原digest，再对caller/epoch/完整input规范摘要

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：`platform.CanonicalJSON`、`media.referenceBindingHash`

## media.referenceBindingHash

- 级别：参考
- 文件：`modules/media/internal/reference_service.go`
- 作用：规范绑定可信caller/epoch和完整site/object/revision/version/operation/digest，refs已由digest绑定

```go
func referenceBindingHash(op OperationContext, input AcquireResourceReferencesInput) (Hash, error)
```

**输入**

无

**输出**

无

**步骤**

1. 规范绑定可信caller/epoch和完整site/object/revision/version/operation/digest，refs已由digest绑定

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：`platform.CanonicalJSON`

## media.ApplyReferenceEvent

- 级别：约束
- 文件：`modules/media/internal/reference_repository.go`
- 作用：消费引用事件防重复/乱序

```go
func ApplyReferenceEvent(ctx context.Context, op OperationContext, input ApplyReferenceEventInput) (ApplyReferenceEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ApplyReferenceEventInput` | 消费引用事件防重复/乱序的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApplyReferenceEventResult` | 消费引用事件防重复/乱序的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态 |

**步骤**

1. 验证生产方为已登记content、活动epoch，并用平台校验器核与pin相同的完整来源身份和引用schema；空集合也必须带sourceOperationId/siteId/sourceObjectId/sourceRevisionId/sourceVersion/referenceDigest，另保留eventId
2. 同一事务调用platform.ClaimInbox按eventId与subject修订去重；sourceVersion低于当前水位只确认不覆写
3. 事务把完整快照写入既有reference_heads/reference_entries并做history分类，提升generation，只取消已经存在的相交人工清理计划
4. 已经明确终止sealed的同源修订不得由迟到事件复活；与持久终止来源矛盾时INTEGRITY_ERROR，保留原引用保护
5. 事件认领为事务首项写入，统一ClaimInboxBeforeWrite只提交冲突隔离，不把业务错误回滚成未隔离。

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）

- 副作用：本域inbox与reference_heads/reference_entries同事务；不写删除许可
- 授权：service:jobs可信事件投递
- 并发：sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：eventId唯一+sourceversion单调
- 依赖：`platform.ClaimInboxBeforeWrite`、`media.mediaWriteContext`、`media.lockReferenceKey`、`media.lockReferenceAssets`、`media.canonicalReferenceInput`、`media.referenceBindingHash`、`media.referenceObjectAssets`、`media.referenceCleanupAssets`、`media.cancelReferenceCleanupPlans`、`media.readReferenceRecord`、`media.readReferenceAsset`、`media.advanceReferenceGeneration`、`media.writeReferenceResolution`

## media.lockReferenceKey

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：本域来源/资源键互斥引用计数，最后释放删除锁表项

```go
func (s *localBlobStore) lockReferenceKey(key string) func()
```

**输入**

无

**输出**

无

**步骤**

1. 本域来源/资源键互斥引用计数，最后释放删除锁表项

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：无

## media.lockReferenceAssets

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：输入按assetId唯一升序锁定对象，逆序释放

```go
func (s *localBlobStore) lockReferenceAssets(assets []gen.AssetVersionRef) func()
```

**输入**

无

**输出**

无

**步骤**

1. 输入按assetId唯一升序锁定对象，逆序释放

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：`media.lockReferenceKey`

## media.readReferenceSource

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：读取同源原绑定，sealed先冲突，核完整材料/原pin快照哈希及唯一schema，不以当前asset重建原回执

```go
func (s *localBlobStore) readReferenceSource(ctx context.Context, q sqlExecer, op OperationContext, input AcquireResourceReferencesInput, bindingHash Hash) (AcquireResourceReferencesResult, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取同源原绑定，sealed先冲突，核完整材料/原pin快照哈希及唯一schema，不以当前asset重建原回执

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：无

## media.acquireReferencePin

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：完整来源与原pin同事务持久，先取消已知相交人工计划，再逐对象提升fence/generation

```go
func (s *localBlobStore) acquireReferencePin(ctx context.Context, op OperationContext, input AcquireResourceReferencesInput, bindingHash Hash, assets, locked []gen.AssetVersionRef, plans []Identifier) (AcquireResourceReferencesResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整来源与原pin同事务持久，先取消已知相交人工计划，再逐对象提升fence/generation

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：`media.readReferenceSource`、`media.mediaWriteContext`、`media.readReferenceAsset`、`media.cancelReferenceCleanupPlans`

## media.readReferenceAsset

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：精确站点/ready/版本/hash/长度/MIME及索引/唯一schema；缺fence或负值INTEGRITY_ERROR，合法math.MaxInt64耗尽LOCKED，返回真实当前逐对象fence

```go
func (s *localBlobStore) readReferenceAsset(ctx context.Context, q sqlExecer, siteID Identifier, expected gen.AssetVersionRef) (int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 精确站点/ready/版本/hash/长度/MIME及索引/唯一schema；缺fence或负值INTEGRITY_ERROR，合法math.MaxInt64耗尽LOCKED，返回真实当前逐对象fence

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：无

## media.readReferenceRecord

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：核私有完整来源及canonical input，已知原pin仍核首次原结果及逐资产保护；sealed无pin允许无input

```go
func (s *localBlobStore) readReferenceRecord(ctx context.Context, q sqlExecer, op OperationContext, sourceOperationID Identifier) (referenceRecord, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核私有完整来源及canonical input，已知原pin仍核首次原结果及逐资产保护；sealed无pin允许无input

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：`media.referenceBindingHash`、`media.canonicalReferenceInput`、`media.readReferenceSource`

## media.readReferenceResolution

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：终局原结果/hash/resolve时刻必须真实持久且与confirmed/sealed对应，缺旧回执不重建

```go
func readReferenceResolution(ctx context.Context, q sqlExecer, op OperationContext, record referenceRecord, kind string) error
```

**输入**

无

**输出**

无

**步骤**

1. 终局原结果/hash/resolve时刻必须真实持久且与confirmed/sealed对应，缺旧回执不重建

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：无

## media.writeReferenceResolution

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：固定安全成功结果与hash/时刻同来源终局事务持久

```go
func writeReferenceResolution(ctx context.Context, tx *sql.Tx, op OperationContext, record referenceRecord, kind string) error
```

**输入**

无

**输出**

无

**步骤**

1. 固定安全成功结果与hash/时刻同来源终局事务持久

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：`platform.CanonicalJSON`

## media.referenceObjectAssets

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：持源对象锁后枚举旧current并与新pin资产去重升序，用于稳定锁完整受影响对象

```go
func (s *localBlobStore) referenceObjectAssets(ctx context.Context, op OperationContext, input AcquireResourceReferencesInput, assets []gen.AssetVersionRef) ([]gen.AssetVersionRef, error)
```

**输入**

无

**输出**

无

**步骤**

1. 持源对象锁读取旧head，核原confirmed来源/终局回执以及每个current项和完整计数，缺行/影子current失败关闭；旧完整资产与新pin去重升序，不信不完整反向索引

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：`media.readReferenceRecord`、`media.readReferenceResolution`

## media.advanceReferenceGeneration

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：读真实逐asset水位，缺/负损坏，int64耗尽LOCKED，CAS增generation且按确认语义提升fence

```go
func advanceReferenceGeneration(ctx context.Context, tx *sql.Tx, assetID Identifier, bumpFence bool) error
```

**输入**

无

**输出**

无

**步骤**

1. 读真实逐asset水位，缺/负损坏，int64耗尽LOCKED，CAS增generation且按确认语义提升fence

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：无

## media.confirmReferencePin

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：回源后窗口内source/对象/asset/store锁及SQL事务，完整集合current/history单调与原confirmed回执同提交

```go
func (s *localBlobStore) confirmReferencePin(ctx context.Context, op OperationContext, record referenceRecord) (ConfirmResourceReferencesResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 回源后窗口内source/对象/asset/store锁及SQL事务，完整集合current/history单调与原confirmed回执同提交

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：`media.lockReferenceKey`、`media.lockReferenceAssets`、`media.referenceObjectAssets`、`media.readReferenceRecord`、`media.readReferenceResolution`、`media.writeReferenceResolution`、`media.advanceReferenceGeneration`、`media.readReferenceAsset`、`media.referenceCleanupAssets`、`media.cancelReferenceCleanupPlans`

## media.releaseReferencePin

- 级别：参考
- 文件：`modules/media/internal/reference_repository.go`
- 作用：回源后同source/对象/asset/store序，重核来源并CAS sealed/原回执，缺pin亦封存，保留原pin_assets

```go
func (s *localBlobStore) releaseReferencePin(ctx context.Context, op OperationContext, input ReleaseResourceReferencesInput, material AcquireResourceReferencesInput, bindingHash Hash) (ReleaseResourceReferencesResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 回源后同source/对象/asset/store序，重核来源并CAS sealed/原回执，缺pin亦封存，保留原pin_assets

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：`media.lockReferenceKey`、`media.lockReferenceAssets`、`media.readReferenceRecord`、`media.readReferenceResolution`、`media.writeReferenceResolution`、`media.advanceReferenceGeneration`

## media.ReadAssetReferences

- 级别：约束
- 文件：`modules/media/internal/reference_reader.go`
- 作用：列哪些文档当前/历史引用资源

```go
func ReadAssetReferences(ctx context.Context, op OperationContext, input ReadAssetReferencesInput) (ReadAssetReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadAssetReferencesInput` | 列哪些文档当前/历史引用资源的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadAssetReferencesResult` | 列哪些文档当前/历史引用资源的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；VERSION_MISMATCH: 活动epoch或扫描/计划水位已改变；INTEGRITY_ERROR: 持久材料、唯一schema、计划摘要、对象fence或来源分页不一致；LOCKED: 资源引用、删除决定、对象水位或未知计划阻止操作 |

**步骤**

1. 核media.manage管理员或本域纯服务worker与已打开epoch，读取真实同站点ready/delete_pending不可变资源及逐对象generation；不把deleted/未知当作空引用
2. 无本地锁按固定content/site-config生成客户端读取assetId过滤的全部分页；派生media纯服务op并清原proof/用户委托，保留epoch/root/deadline；每源所有页面watermark必须一致、cursor前进、分页完整，总集合不得超唯一snapshot预算
3. content保留retained=true的current_draft/current_published映射current、history映射history，blockId作referenceId；site-config只接受对应来源confirmed且expiresAt=null的current/history；每项精确asset相等，重复同材料去重、冲突失败关闭
4. 回源后重新核本地asset/generation未变，合并真实pending pin原材料（过期不擅自释放）；去重身份包含sourceService/sourceObjectId/sourceRevisionId/sourceVersion/referenceId/kind，保留合法不同单调版本，只有同身份异材料才损坏；精确相同draft/published投影可合并；完整引用排序及两个complete水位通过唯一schema，未知/未完整失败关闭
5. content远端pending_pin与本地原pin按完整来源身份（含sourceVersion）去重；两条均未confirmed的pending仅允许expiry投影差异，去expiry后完整规范字段尤其Asset必须一致；任一未知expiry保留nil，两个已知取真实较晚展示；不改写/续期/释放任何原pin，过期仍保护；其他材料矛盾失败关闭，current/history仍严格一致

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `VERSION_MISMATCH`：活动epoch或扫描/计划水位已改变（不可重试）
- `INTEGRITY_ERROR`：持久材料、唯一schema、计划摘要、对象fence或来源分页不一致（不可重试）
- `LOCKED`：资源引用、删除决定、对象水位或未知计划阻止操作（不可重试）

- 副作用：无
- 授权：media.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`media.cleanupStore`、`media.readCleanupAsset`、`media.readSourceReferences`、`media.pendingAssetReferences`

## media.readSourceReferences

- 级别：参考
- 文件：`modules/media/internal/reference_reader.go`
- 作用：完整分页读取固定权威源并把已保留引用映射为本域统一引用；每源最多20000扫描行、watermark固定、完整游标前进、逐项精确asset及终局材料

```go
func (s *localBlobStore) readSourceReferences(ctx context.Context, op OperationContext, asset gen.AssetVersionRef, source string) ([]gen.AssetReference, gen.Revision, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整分页读取固定权威源并把已保留引用映射为本域统一引用；每源最多20000扫描行、watermark固定、完整游标前进、逐项精确asset及终局材料
2. content kind仅current_draft/current_published/history/pending_pin；pending_pin转未confirmed且未知expiry仍保护，未知kind拒绝

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：`content:listResourceReferences`、`site-config:listResourceReferences`

## media.pendingAssetReferences

- 级别：参考
- 文件：`modules/media/internal/reference_reader.go`
- 作用：从真实pending来源读取不可变pin回执与完整引用；即使过期仍保护，不按时间推断可释放

```go
func (s *localBlobStore) pendingAssetReferences(ctx context.Context, q sqlExecer, op OperationContext, asset gen.AssetVersionRef) ([]gen.AssetReference, error)
```

**输入**

无

**输出**

无

**步骤**

1. 从真实pending来源读取不可变pin回执与完整引用；即使过期仍保护，不按时间推断可释放

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：`media.readReferenceRecord`、`media.readReferenceSource`

## media.ListUnusedAssets

- 级别：约束
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：后台单列当前未引用文件

```go
func ListUnusedAssets(ctx context.Context, op OperationContext, input ListUnusedAssetsInput) (ListUnusedAssetsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ListUnusedAssetsInput` | 后台单列当前未引用文件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListUnusedAssetsResult` | 后台单列当前未引用文件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 使用同一真实资产目录和签名分页，保留既有可选kind，当前本站media.manage/同opened epoch；只扫描ready候选，每次最多limit条并读取一条lookahead
2. 本地只读快照显式结束后逐候选ValidateCleanupSources(expectedGeneration)；content/site-config均完整且两个来源全页及跨页固定水位，任一来源未知、分页变化或材料错误直接失败关闭，不输出部分未引用结论
3. current/pending非空排除；pending即使到期或expiry未知仍保护。仅safe候选加入输出，其真实content历史文档ID唯一排序汇总，不静默丢历史风险；超过10000风险响应预算明确要求减小limit
4. 全部回源后独立本地只读快照重核列表水位及完整候选Asset/ref/revision/ready/fence/generation，变化要求从头分页；所有RPC/签名在SQL和业务锁外
5. 游标按最后扫描候选前进，不按最后匹配资产。空匹配页可hasMore=true且nextCursor前进，最终页才hasMore=false；本地完全无候选可直接返回空，不宣称任何实际资产未引用
6. 只展示，不生成批准/计划，不触发模型、任务或删除；后续确认/执行仍须重新完整核源及人工授权

**错误**

- `FORBIDDEN`：非当前本站media.manage管理员（不可重试）
- `VALIDATION`：游标签名/scope/版本或严格查询参数不合法（不可重试）
- `VERSION_MISMATCH`：opened epoch、本地或引用来源水位改变，须重头分页（可重试）
- `INTEGRITY_ERROR`：持久Asset/索引/fence/generation或权威材料损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：存储/密钥/权威来源未知，或风险响应预算超限（可重试）
- `NOT_FOUND`：候选资产或权威来源在回源期间消失，不能当作未引用（不可重试）
- `LOCKED`：候选进入不可清理状态或权威源锁定，不能当作未引用（可重试）

- 副作用：无
- 授权：当前本站media.manage或superadmin管理员，HTTP仅portal；每页经平台当前授权，游标仅位置不代替权限
- 并发：本地短只读快照→全部锁外源RPC→独立本地复核→SQL外签名；不用一条长事务跨RPC
- 幂等：按候选稳定keyset，固定本地与两个来源水位，任一变化从头，不缓存删除许可
- 依赖：`media.assetListingStore`、`media.readAssetListingCursor`、`media.readAssetListingPage`、`media.ValidateCleanupSources`、`media.recheckAssetListingPage`、`media.assetListingPageInfo`

## media.ProposeManualCleanup

- 级别：约束
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：人工清理前生成精确风险计划

```go
func ProposeManualCleanup(ctx context.Context, op OperationContext, input ProposeManualCleanupInput) (ProposeManualCleanupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ProposeManualCleanupInput` | 人工清理前生成精确风险计划的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ProposeManualCleanupResult` | 人工清理前生成精确风险计划的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 持久材料、唯一schema、计划摘要、对象fence或来源分页不一致；LOCKED: 资源引用、删除决定、对象水位或未知计划阻止操作 |

**步骤**

1. 核media.manage管理员、同站点及已打开epoch；固定caller/actor/site/epoch/完整输入规范摘要，同operationId先回原plan结果/hash，不借mutable计划重建
2. 对assetId升序唯一精确资产逐个调用ValidateCleanupSources；仅真实ready且current/pending0、完整源水位，历史文档单独汇总风险；未知失败关闭；单计划原扫描材料合计最多16MiB，超出明确要求分批
3. 无锁回源后先取得新running窗口，再按全部assetId稳定锁及store互斥，在SQL重核asset/ref/fence/generation与pending/current保护；任何扫描后变化拒绝
4. 同事务写proposed plan revision=0、精确资产/逐对象fence/generation/完整source快照、confirmationHash和原结果/hash；notBefore仅提案展示值，未来确认须重定至少24h；不标delete_pending、不删bytes、不自动确认

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久材料、唯一schema、计划摘要、对象fence或来源分页不一致（不可重试）
- `LOCKED`：资源引用、删除决定、对象水位或未知计划阻止操作（不可重试）

- 副作用：人工清理计划metadata
- 授权：media.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：operationId复用精确plan
- 依赖：`media.cleanupStore`、`media.cleanupRequestHash`、`media.readCleanupReceipt`、`media.readCleanupAsset`、`media.ValidateCleanupSources`、`media.mediaWriteContext`、`media.lockReferenceAssets`、`media.writeCleanupReceipt`

## media.ConfirmManualCleanup

- 级别：约束
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：管理员确认后进入迟到引用保护窗口

```go
func ConfirmManualCleanup(ctx context.Context, op OperationContext, input ConfirmManualCleanupInput) (ConfirmManualCleanupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ConfirmManualCleanupInput` | 管理员确认后进入迟到引用保护窗口的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConfirmManualCleanupResult` | 管理员确认后进入迟到引用保护窗口的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 当前media.manage管理员/同站点与opened epoch后，按同operationId/caller/actor/site/epoch/输入摘要读取独立批准桥接；原waiting成功只返回固定原结果，其他kind占用操作拒绝，不从mutable计划重建
2. 新批准只接受原proposed且expectedRevision/confirmationHash精确匹配；readCleanupPlan复核原扫描摘要、历史风险集合及不可变完整资产，历史非空须明确acknowledgeHistoryLoss
3. 所有RPC无asset/store锁：重新ValidateCleanupSources读取两源完整固定水位与本地pending；快照必须逐字节规范等于原扫描，current/pin出现则原proposed计划CAS blocked且不写批准；其他水位/历史/资产变化拒绝并要求新提案，未知绝不生成授权
4. 重新公共running写窗口，再计划完整assetId稳定序锁→store→SQL，重读原plan/revision/精确扫描及原批准；逐资产核ready、同原fence/generation、无DELETING/任何marks/current entries/pending pins，任何变化失败关闭
5. 同事务提升逐资产fence/generation、写delete_pending及精确manual_cleanup_marks、计划confirmed/revision+1与批准对象；批准对象绑定原输入/批准人/原root/固定UTC confirmedAt及notBefore≥confirmedAt+24h/完整精确marks；manual_cleanup_assets原扫描不覆盖
6. 批准事务提交后释放全部业务锁；独立只读SQL快照重核完整confirmed计划/marks，显式Rollback结束后，按持久原operationId/root/dedupeKey/ObjectRef/notBefore调用jobs:createJob真实延迟入队。任何网络/提交未知保留confirmed及原批准，同键重试，绝不生成新的批准或任务输入
7. 任务成功须唯一jobs CreateJobResult schema、queued及原operationId一致；重新写窗口及完整asset锁→store→SQL核同原批准、完整计划/marks；同事务保存原AcceptedJob并以confirmed修订CAS waiting/revision+1及固定原成功结果。并发Cancel/Acquire已取消时仅保存原任务回执，不复活计划、不返回未完成确认成功
8. 原批准与回执永久保留；执行另核当前waiting/原任务回执/原批准/notBefore/精确marks及最终两源检查，旧对象本身不是删除许可

**错误**

- `FORBIDDEN`：主体没有media.manage或跨站点（不可重试）
- `NOT_FOUND`：原人工计划不存在（不可重试）
- `CONFLICT`：计划非proposed、确认材料/operationId冲突或已取消（不可重试）
- `VERSION_MISMATCH`：原扫描/expectedRevision/asset/source水位或dataEpoch变化（不可重试）
- `LOCKED`：新引用/pending保护/标记/DELETING或水位耗尽阻止确认（不可重试）
- `INTEGRITY_ERROR`：原扫描/批准/标记或任务回执损坏、不符合唯一schema（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖/任务提交/存储未知，不授予删除许可（可重试）

- 副作用：只追加人工批准与任务桥接、plan revision CAS、精确marks/delete_pending；无bytes删除
- 授权：permission：media.manage；rule：当前同站点有media.manage或superadmin的已验证管理员，必须人工明确确认原摘要与历史风险；jobs提交派生固定media纯服务身份且清除原用户委托
- 并发：权威/运行/jobs RPC均无asset/store锁；完整assetId稳定序→store→SQL；plan revision及逐对象fence/generation CAS
- 幂等：一个plan只接受一次持久人工批准；同operationId/原摘要恢复原批准与同jobs dedupe，不重置notBefore；原waiting结果固定
- 依赖：`media.cleanupStore`、`media.cleanupRequestHash`、`media.readCleanupApproval`、`media.readCleanupPlan`、`media.scanCleanupConfirmation`、`media.approveCleanupPlan`、`media.submitCleanupApproval`、`media.completeCleanupApproval`、`media.ValidateCleanupSources`、`jobs:createJob`

## media.CancelManualCleanup

- 级别：约束
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：删除前取消人工清理

```go
func CancelManualCleanup(ctx context.Context, op OperationContext, input CancelManualCleanupInput) (CancelManualCleanupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CancelManualCleanupInput` | 删除前取消人工清理的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CancelManualCleanupResult` | 删除前取消人工清理的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: 活动epoch或扫描/计划水位已改变；INTEGRITY_ERROR: 持久材料、唯一schema、计划摘要、对象fence或来源分页不一致；LOCKED: 资源引用、删除决定、对象水位或未知计划阻止操作；DEPENDENCY_UNAVAILABLE: 权威依赖、完整分页或存储不可用，未知失败关闭；NOT_FOUND: 精确资源或计划不存在 |

**步骤**

1. 核media.manage管理员、已打开epoch及计划同站点；固定actor/site/epoch/planId请求摘要，已存原成功优先，同operationId异材料CONFLICT
2. 无锁取得running窗口，按计划完整资产稳定锁及store互斥，再SQL重读原计划/修订/完整资产，deleted或任一已持久DELETING决定CONFLICT
3. proposed只改cancelled/revision+1；confirmed/waiting要求全部精确delete_pending标记；blocked允许已阻断但仍保留部分本计划标记，逐项核精确对象fence/generation后只恢复本计划pending项，其余项须真实ready或明确属于另一计划；已有DELETING不能取消；plan.revision达公共Revision上限9007199254740991时LOCKED，不收紧逐对象int64水位
4. 计划cancelled和原cancelled=true/hash同事务，原提案回执永久保留；不调用jobs删除，未来任务只能重读已持久取消计划拒绝执行

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：活动epoch或扫描/计划水位已改变（不可重试）
- `INTEGRITY_ERROR`：持久材料、唯一schema、计划摘要、对象fence或来源分页不一致（不可重试）
- `LOCKED`：资源引用、删除决定、对象水位或未知计划阻止操作（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、完整分页或存储不可用，未知失败关闭（可重试）
- `NOT_FOUND`：精确资源或计划不存在（不可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：media.manage
- 并发：与executedelete同fence
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`media.cleanupStore`、`media.cleanupRequestHash`、`media.readCleanupReceipt`、`media.readCleanupPlan`、`media.mediaWriteContext`、`media.lockReferenceAssets`、`media.cancelCleanupPlan`、`media.writeCleanupReceipt`

## media.cleanupStore

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：核管理员media.manage或仅只读本域纯服务worker及已打开epoch，不让服务身份代替人工授权

```go
func cleanupStore(op OperationContext, requireAdmin bool) (*localBlobStore, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核管理员media.manage或仅只读本域纯服务worker及已打开epoch，不让服务身份代替人工授权

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：`media.currentBlobStore`、`media.hasGrant`

## media.readCleanupAsset

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：核同站点真实ready/delete_pending资产唯一schema、索引与逐对象fence/generation，未知或损坏失败关闭

```go
func (s *localBlobStore) readCleanupAsset(ctx context.Context, q sqlExecer, siteID, assetID Identifier) (gen.Asset, int, int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核同站点真实ready/delete_pending资产唯一schema、索引与逐对象fence/generation，未知或损坏失败关闭

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：无

## media.cleanupRequestHash

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：对kind/caller/actor/site/epoch/完整输入计算固定规范摘要，排除当前时钟

```go
func cleanupRequestHash(op OperationContext, kind string, input any) (Hash, error)
```

**输入**

无

**输出**

无

**步骤**

1. 对kind/caller/actor/site/epoch/完整输入计算固定规范摘要，排除当前时钟

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：无

## media.readCleanupReceipt

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：核同operationId摘要及原结果/hash，提案内层schema与原revision=0，取消原成功固定；不从mutable计划重建

```go
func (s *localBlobStore) readCleanupReceipt(ctx context.Context, q sqlExecer, op OperationContext, requestHash Hash, kind string, target any) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核同operationId摘要及原结果/hash，提案内层schema与原revision=0，取消原成功固定；不从mutable计划重建
2. 无旧提案/取消回执时仍核独立批准表operationId，其他kind不得复用批准操作

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：无

## media.writeCleanupReceipt

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：将原成功结果与规范摘要/hash写入调用方SQL事务，未知Commit由相同operationId恢复

```go
func writeCleanupReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, requestHash Hash, kind string, planID Identifier, value any) error
```

**输入**

无

**输出**

无

**步骤**

1. 将原成功结果与规范摘要/hash写入调用方SQL事务，未知Commit由相同operationId恢复

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：无

## media.readCleanupPlan

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：核计划代次/站点/唯一schema/hash/修订及精确完整资产与原扫描水位，重算confirmationHash与历史风险集合；扫描总材料最多16MiB

```go
func (s *localBlobStore) readCleanupPlan(ctx context.Context, q sqlExecer, op OperationContext, planID Identifier) (gen.CleanupPlan, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核计划代次/站点/唯一schema/hash/修订及精确完整资产与原扫描水位，重算confirmationHash与历史风险集合；扫描总材料最多16MiB

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：无

## media.cancelCleanupPlan

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：调用方持完整计划资产锁及store互斥；SQL内核无DELETING决定和有效精确pending标记，同事务恢复ready/解除标记/提升fence与generation及取消planCAS，未知不撤销

```go
func (s *localBlobStore) cancelCleanupPlan(ctx context.Context, tx *sql.Tx, op OperationContext, plan gen.CleanupPlan) error
```

**输入**

无

**输出**

无

**步骤**

1. 调用方持完整计划资产锁与store互斥；SQL核计划未deleted/无DELETING；confirmed/waiting必须完整本计划marks，blocked可能保留部分marks，逐项真实核资产/fence/generation并只解除本计划标记；无标记的delete_pending仍失败关闭；整份cancelled与ready/fence/generation同事务，plan revision按公共上限失败关闭，对象水位仍原int64边界

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：`media.readCleanupAsset`、`media.advanceReferenceGeneration`、`media.readCleanupPlan`

## media.referenceCleanupAssets

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：持任何asset锁之前展开相交计划完整不可变资产集合并排序；异材料失败，后续SQL重核不在事务内补锁

```go
func (s *localBlobStore) referenceCleanupAssets(ctx context.Context, op OperationContext, siteID Identifier, requested, base []gen.AssetVersionRef) ([]gen.AssetVersionRef, []Identifier, error)
```

**输入**

无

**输出**

无

**步骤**

1. 持任何asset锁之前展开相交计划完整不可变资产集合并排序；异材料失败，后续SQL重核不在事务内补锁

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：`media.assetCleanupPlans`、`media.readCleanupPlan`

## media.assetCleanupPlans

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：只读列出相交proposed/confirmed/waiting/blocked真实计划ID，保留未知错误

```go
func (s *localBlobStore) assetCleanupPlans(ctx context.Context, q sqlExecer, assetID Identifier) ([]Identifier, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读列出相交proposed/confirmed/waiting/blocked真实计划ID，保留未知错误

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：无

## media.cancelReferenceCleanupPlans

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：在SQL内核当前相交计划只属于预先扩锁集合，重读代次/修订/精确资产后与新增引用同事务取消整份计划

```go
func (s *localBlobStore) cancelReferenceCleanupPlans(ctx context.Context, tx *sql.Tx, op OperationContext, siteID Identifier, requested, locked []gen.AssetVersionRef, planIDs []Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 在SQL内核当前相交计划只属于预先扩锁集合，重读代次/修订/精确资产后与新增引用同事务取消整份计划

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，scope/epoch/CAS由本边界负责
- 并发：本域source/object/asset稳定锁及store互斥；SQL内不得回源或补锁
- 幂等：原材料和规范摘要绑定，未知失败关闭
- 依赖：`media.assetCleanupPlans`、`media.readCleanupPlan`、`media.cancelCleanupPlan`

## media.InstallCleanupJobsClient

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：构造期装配固定生成jobs客户端，不造缺失依赖

```go
func InstallCleanupJobsClient(jobs *gen.JobsClient) error
```

**输入**

无

**输出**

无

**步骤**

1. 构造期装配固定生成jobs客户端，不造缺失依赖

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：无

## media.cleanupJobRequest

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：由原批准规范字节构造精确ObjectRef/plan+epoch+hash去重键与固定notBefore；不重新选时钟

```go
func cleanupJobRequest(approval CleanupApproval, raw string) cleanupJobInput
```

**输入**

无

**输出**

无

**步骤**

1. 由原批准规范字节构造精确ObjectRef/plan+epoch+hash去重键与固定notBefore；不重新选时钟

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：无

## media.loadCleanupApproval

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：16MiB有界读取批准/固定任务输入/原jobs回执/原waiting结果；唯一schema、规范字节、摘要、原输入绑定、时间/修订/完整marks及索引逐项核对

```go
func (s *localBlobStore) loadCleanupApproval(ctx context.Context, q sqlExecer, planID Identifier) (cleanupApprovalRecord, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 16MiB有界读取批准/固定任务输入/原jobs回执/原waiting结果；唯一schema、规范字节、摘要、原输入绑定、时间/修订/完整marks及索引逐项核对

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.cleanupRequestHash`、`media.cleanupJobRequest`

## media.readCleanupApproval

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：确认操作与其他清理kind互斥；原plan一次批准，caller/actor/site/epoch/op/root/输入摘要完全相同才恢复原批准或原成功

```go
func (s *localBlobStore) readCleanupApproval(ctx context.Context, q sqlExecer, op OperationContext, requestHash Hash, input ConfirmManualCleanupInput) (cleanupApprovalRecord, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 确认操作与其他清理kind互斥；原plan一次批准，caller/actor/site/epoch/op/root/输入摘要完全相同才恢复原批准或原成功

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.loadCleanupApproval`

## media.scanCleanupConfirmation

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：无锁逐资产完整回源并比较原规范扫描（含两源固定watermark/历史/pending）；出现current/pin先阻断原提案，其他变化要求新提案

```go
func (s *localBlobStore) scanCleanupConfirmation(ctx context.Context, op OperationContext, plan gen.CleanupPlan) ([]cleanupScan, error)
```

**输入**

无

**输出**

无

**步骤**

1. 无锁逐资产完整回源并比较原规范扫描（含两源固定watermark/历史/pending）；出现current/pin先阻断原提案，其他变化要求新提案

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.ValidateCleanupSources`、`media.blockCleanupProposal`

## media.blockCleanupProposal

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：已知新引用只把同原修订proposed CAS blocked，完整资产锁与SQL内重读；不写批准/marks/delete_pending

```go
func (s *localBlobStore) blockCleanupProposal(ctx context.Context, op OperationContext, plan gen.CleanupPlan) error
```

**输入**

无

**输出**

无

**步骤**

1. 已知新引用只把同原修订proposed CAS blocked，完整资产锁与SQL内重读；不写批准/marks/delete_pending

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.mediaWriteContext`、`media.lockReferenceAssets`、`media.readCleanupPlan`、`media.updateCleanupState`

## media.updateCleanupState

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：已授权事务链按原epoch/site/revision/state精确CAS计划及规范hash；调用方已核修订未耗尽

```go
func (s *localBlobStore) updateCleanupState(ctx context.Context, tx *sql.Tx, op OperationContext, plan gen.CleanupPlan, state string) error
```

**输入**

无

**输出**

无

**步骤**

1. 已授权事务链按原epoch/site/revision/state精确CAS计划及规范hash；调用方已核修订未耗尽

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：无

## media.approveCleanupPlan

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：RPC后running窗口→完整asset锁→store→SQL；原计划/资产/fence/generation、current/pending/marks/DELETING重核，批准/精确marks/delete_pending/confirmed与固定任务输入同事务；未知Commit报错留原批准

```go
func (s *localBlobStore) approveCleanupPlan(ctx context.Context, op OperationContext, hash Hash, input ConfirmManualCleanupInput, plan gen.CleanupPlan, scans []cleanupScan) (cleanupApprovalRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. RPC后running窗口→完整asset锁→store→SQL；原计划/资产/fence/generation、current/pending/marks/DELETING重核，批准/精确marks/delete_pending/confirmed与固定任务输入同事务；未知Commit报错留原批准

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.mediaWriteContext`、`media.lockReferenceAssets`、`media.readCleanupApproval`、`media.readCleanupPlan`、`media.readCleanupAsset`、`media.advanceReferenceGeneration`、`media.updateCleanupState`、`media.cleanupJobRequest`

## media.checkCleanupApprovalPending

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：核原confirmed规范计划与完整marks数量、逐资产待删/原fence/generation/current/pending/DELETING；只证明待提交活动计划，不能跳过最终回源

```go
func (s *localBlobStore) checkCleanupApprovalPending(ctx context.Context, q sqlExecer, op OperationContext, record cleanupApprovalRecord) error
```

**输入**

无

**输出**

无

**步骤**

1. 核原confirmed规范计划与完整marks数量、逐资产待删/原fence/generation/current/pending/DELETING；只证明待提交活动计划，不能跳过最终回源

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.readCleanupPlan`、`media.readCleanupAsset`

## media.submitCleanupApproval

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：无asset/store锁时以独立只读SQL事务固定完整计划/marks/assets快照，显式Rollback释放后才提交固定jobs任务；未知不重批

```go
func (s *localBlobStore) submitCleanupApproval(ctx context.Context, op OperationContext, hash Hash, input ConfirmManualCleanupInput, record cleanupApprovalRecord) (ConfirmManualCleanupResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 无asset/store锁建立只读SQL事务，checkCleanupApprovalPending全部读取使用同一快照，避免Cancel并发被跨事务拼读为损坏；无论检查成功与否显式Rollback结束，失败返回错误，绝不把读事务带过RPC
2. 固定原op/root/dedupe/inputRef/notBefore及纯media服务上下文，清用户proof；未知不重批，唯一schema/原op回执才进入持久完成

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.checkCleanupApprovalPending`、`media.completeCleanupApproval`、`jobs:createJob`

## media.completeCleanupApproval

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：真实任务回执后重新窗口/完整asset锁→store→SQL；原批准/回执稳定，confirmed完整marks仍有效才waiting CAS和原结果同事务；已取消/阻断仅存真实回执并返回错误，绝不复活

```go
func (s *localBlobStore) completeCleanupApproval(ctx context.Context, op OperationContext, hash Hash, input ConfirmManualCleanupInput, record cleanupApprovalRecord, receipt cleanupJobReceipt) (ConfirmManualCleanupResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实任务回执后重新窗口/完整asset锁→store→SQL；原批准/回执稳定，confirmed完整marks仍有效才waiting CAS和原结果同事务；已取消/阻断仅存真实回执并返回错误，绝不复活

**错误**

- 无

- 副作用：无
- 授权：仅本域可信人工清理批准/任务桥接链；入口schema/身份一次，原scope/epoch/CAS/持久完整性由本函数核
- 并发：所有RPC无asset/store锁；完整assetId稳定锁→store→SQL，锁内不补锁/回源
- 幂等：原批准/输入/回执固定，未知失败关闭
- 依赖：`media.mediaWriteContext`、`media.lockReferenceAssets`、`media.readCleanupApproval`、`media.checkCleanupApprovalPending`、`media.updateCleanupState`

## media.ReadCleanupApprovalObject

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：同进程media worker只读精确持久人工批准对象，不注册HTTP入口

```go
func ReadCleanupApprovalObject(ctx context.Context, op OperationContext, input ReadCleanupApprovalObjectInput) (ReadCleanupApprovalObjectResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与期限传播 | 传入下游IO |
| `op` | `OperationContext` | 可信media纯服务上下文 | 只允许media服务身份，本站活动dataEpoch |
| `input` | `ReadCleanupApprovalObjectInput` | 精确plan/ObjectRef版本/hash | 严格schema，拒未知字段 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadCleanupApprovalObjectResult` | 原规范UTF8及ObjectRef | 仅返回同域原规范字节与精确ObjectRef，不含token/grants/私有路径，不作为HTTP DTO |
| `error` | `Error` | 标准Problem，失败结果无效 | 未知失败关闭 |

**步骤**

1. 仅当前同进程media worker可信服务上下文、同opened epoch，读取独立真实批准桥接并严格核schema/hash/规范字节/原任务输入
2. 逐项核批准site/dataEpoch及请求version/hash等于ObjectRef；返回原approval_json字节和精确ObjectRef，不从mutable计划构造、不产生句柄
3. 已取消仍可读原批准证据；本读取不决定当前计划/任务执行资格，执行函数仍须完整计划、原批准/notBefore和最终回源检查

**错误**

- `FORBIDDEN`：非media纯服务或跨站点/代次（不可重试）
- `NOT_FOUND`：未持久批准（不可重试）
- `VERSION_MISMATCH`：对象版本/摘要或opened epoch不符（不可重试）
- `INTEGRITY_ERROR`：批准/任务输入/回执损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：持久存储不可用（可重试）

- 副作用：无
- 授权：仅同进程media worker可信调用链；media服务subject、同site/dataEpoch及精确版本/hash；不注册HTTP/浏览器入口
- 并发：只读持久快照；无RPC/业务资产锁，不改变计划
- 幂等：同精确ObjectRef返回原不可变规范字节；取消不改写批准对象
- 依赖：`media.cleanupStore`、`media.loadCleanupApproval`

## media.recheckAssetListingPage

- 级别：参考
- 文件：`modules/media/internal/cleanup_service.go`
- 作用：全部回源后新短只读快照重核本地水位及完整候选精确状态

```go
func (s *localBlobStore) recheckAssetListingPage(ctx context.Context, op OperationContext, page assetListingPage) error
```

**输入**

无

**输出**

无

**步骤**

1. 全部回源后新短只读快照重核本地水位及完整候选精确状态

**错误**

- 无

- 副作用：无
- 授权：可信启动或已授权本站列表调用链；入口schema/身份一次
- 并发：短只读快照，所有RPC与密钥IO在SQL/业务锁外
- 幂等：保留精确水位与来源，不重置、不产生删除许可
- 依赖：`media.readAssetListingRevision`、`media.readCleanupAsset`

## media.ValidateCleanupSources

- 级别：参考
- 文件：`modules/media/internal/cleanup_sources.go`
- 作用：确认所有权威引用源后才认定可清理

```go
func ValidateCleanupSources(ctx context.Context, op OperationContext, input ValidateCleanupSourcesInput) (ValidateCleanupSourcesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidateCleanupSourcesInput` | 确认所有权威引用源后才认定可清理的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateCleanupSourcesResult` | 确认所有权威引用源后才认定可清理的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变 |

**步骤**

1. 调用ReadAssetReferences完整读取content/site-config与本地pending保护，expectedGeneration非null必须等于真实逐对象水位
2. snapshot恰有content/site-config两个complete水位且current集合为空才safeToDelete=true，pending_pin同样阻止；history只保留风险提示，未知/未完整不得返回safe=true

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）

- 副作用：无
- 授权：media私有人工清理流程
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`media.ReadAssetReferences`

## media.ExecuteManualCleanup

- 级别：约束
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：只执行已人工确认的安全清理计划

```go
func ExecuteManualCleanup(ctx context.Context, op OperationContext, input ExecuteManualCleanupInput) (ExecuteManualCleanupResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ExecuteManualCleanupInput` | 只执行已人工确认的安全清理计划的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExecuteManualCleanupResult` | 只执行已人工确认的安全清理计划的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅本域纯media worker及平台CheckLease guard；完整JobExecutionInput保留原ObjectRef/jobId/workerId/fence，ReadCleanupApprovalObject精确读取原规范字节，核原operationId/root/dataEpoch/站点、原AcceptedJob与固定waiting结果；领取时新建十分钟执行期限
2. 终局原结果只回放，原DELETING只收敛；新执行只接受当前完整计划规范字节等于原waiting、notBefore=confirmedAt+24h且已到期、完整精确marks，取消/blocked/未知均拒绝
3. 所有RPC无asset/store/SQL锁；逐资产ValidateCleanupSources取得两源固定水位完整分页及本地pending；新current/pending或原批准历史风险变化则整份CAS blocked并保留marks；未知失败关闭
4. 最终平台CheckLease与公共running写窗口后，完整assetId稳定序锁→store→SQL；核原waiting/完整marks/fence/generation、最终快照generation、无current/pending/DELETING和activeReader及资源/计划修订空间，变化失败关闭
5. 同一事务固定schemaVersion=1执行材料、原批准摘要/job/op/root/site/epoch/waiting修订/decidedAt及全部逐资产决定与DELETING journal；同站点同SHA计划外任一未删除asset存在保留共享bytes，否则仅一项拥有bytes删除；未知Commit不unlink
6. 原决定恢复前完整核批准/marks/资产/水位及保护，store互斥覆盖generated blob普通文件/身份/SHA/长度核对、unlink/目录fsync及终局SQL；Complete同store互斥且未完成同SHA决定阻止新bytes提交/复用
7. 仅原持久决定允许缺失blob恢复；deleted/revision+1、精确mark解除、真实generation的deleted outbox、计划终局及完整原结果/hash同事务。Cancel遇任一DELETING始终CONFLICT，提交未知保留材料
8. 启动exclusive先恢复同opened epoch明确批准决定再核COMMITTED；deleted必须具完整原终局证明，未知/孤立DELETING失败关闭，不扫描ready自动产生授权

**错误**

- `FORBIDDEN`：非持平台guard的本域worker或跨站点（不可重试）
- `NOT_FOUND`：原人工批准不存在（不可重试）
- `CONFLICT`：原waiting/任务/对象或操作材料变化（不可重试）
- `LOCKED`：保护期、引用/读者/marks/fence变化或修订耗尽（不可重试）
- `VERSION_MISMATCH`：原generation、ObjectRef或dataEpoch不符（不可重试）
- `INTEGRITY_ERROR`：原执行/批准/决定/journal/终局或blob损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：来源、运行权威、租约或持久存储未知（可重试）

- 副作用：经人工确认不可逆ready文件删除
- 授权：仅可信同进程media纯服务任务；原批准ObjectRef、原jobs回执、原waiting、24小时notBefore及平台有效租约guard共同约束，不注册HTTP
- 并发：RPC锁外；完整assetId稳定序→store→SQL；全计划DELETING原子，store覆盖unlink/终局并与Complete互斥
- 幂等：planId/原approval hash/jobId固定；整份DELETING和逐资产日志唯一，终局原结果/hash固定
- 依赖：`media.cleanupStore`、`media.loadCleanupApproval`、`media.ReadCleanupApprovalObject`、`media.checkCleanupJobBinding`、`media.readCleanupExecution`、`media.cleanupWaitingEqual`、`media.ValidateCleanupSources`、`media.blockCleanupExecution`、`media.decideCleanupExecution`、`media.finishCleanupExecutionLocked`、`media.mediaWriteContext`

## media.checkCleanupJobBinding

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：核完整ObjectRef、原jobId/op/root/site/epoch和原waiting回执，不从任务造授权

```go
func checkCleanupJobBinding(op OperationContext, record cleanupApprovalRecord, job platform.JobExecutionInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 核完整ObjectRef、原jobId/op/root/site/epoch和原waiting回执，不从任务造授权

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：无

## media.cleanupExecutionMaterial

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：从原批准/AcceptedJob/waiting构造执行材料及决定时刻

```go
func cleanupExecutionMaterial(record cleanupApprovalRecord) cleanupExecution
```

**输入**

无

**输出**

无

**步骤**

1. 从原批准/AcceptedJob/waiting构造执行材料及决定时刻

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：无

## media.readCleanupExecution

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：核原规范材料/hash、全部决定/原资产/mark、唯一DELETING与固定终局

```go
func (s *localBlobStore) readCleanupExecution(ctx context.Context, q sqlExecer, approval cleanupApprovalRecord) (cleanupExecutionRecord, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核原规范材料/hash、全部决定/原资产/mark、唯一DELETING与固定终局

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.cleanupExecutionMaterial`

## media.cleanupWaitingEqual

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：完整当前计划规范字节必须等于原waiting

```go
func cleanupWaitingEqual(plan gen.CleanupPlan, approval cleanupApprovalRecord) bool
```

**输入**

无

**输出**

无

**步骤**

1. 完整当前计划规范字节必须等于原waiting

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：无

## media.blockCleanupExecution

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：锁下仅阻断无DELETING原waiting，保留全部marks

```go
func (s *localBlobStore) blockCleanupExecution(ctx context.Context, op OperationContext, approval cleanupApprovalRecord) (ExecuteManualCleanupResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 锁下仅阻断无DELETING原waiting，保留全部marks

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.mediaWriteContext`、`media.lockReferenceAssets`、`media.readCleanupExecution`、`media.readCleanupPlan`、`media.updateCleanupState`、`media.cleanupWaitingEqual`

## media.decideCleanupExecution

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：最后原waiting/marks/generation/pending/readers0校验，全计划DELETING与共享bytes决定同事务

```go
func (s *localBlobStore) decideCleanupExecution(ctx context.Context, op OperationContext, approval cleanupApprovalRecord, snapshots []gen.ReferenceSnapshot) (cleanupExecutionRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 最后原waiting/marks/generation/pending/readers0校验，全计划DELETING与共享bytes决定同事务

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.mediaWriteContext`、`media.lockReferenceAssets`、`media.readCleanupExecution`、`media.readCleanupPlan`、`media.cleanupWaitingEqual`、`media.cleanupExecutionMaterial`、`media.readCleanupAsset`、`media.appendJournal`

## media.finishCleanupExecutionLocked

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：完整决定保护核对后unlink/目录fsync，deleted/marks/outbox/原结果及计划终局同事务

```go
func (s *localBlobStore) finishCleanupExecutionLocked(ctx context.Context, op OperationContext, approval cleanupApprovalRecord, expected cleanupExecutionRecord) (ExecuteManualCleanupResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整决定保护核对后unlink/目录fsync，deleted/marks/outbox/原结果及计划终局同事务

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.readCleanupExecution`、`media.readCleanupPlan`、`media.cleanupWaitingEqual`、`media.readCleanupAsset`、`media.removeDecidedBlob`、`media.PublishAssetEvent`、`media.updateCleanupState`

## media.removeDecidedBlob

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：generated路径普通文件/身份/SHA/长度核对后unlink/fsync，缺失仅原决定允许恢复

```go
func (s *localBlobStore) removeDecidedBlob(ctx context.Context, decision cleanupDeletion) error
```

**输入**

无

**输出**

无

**步骤**

1. generated路径普通文件/身份/SHA/长度核对后unlink/fsync，缺失仅原决定允许恢复

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.openMediaRoot`、`media.openMediaRegular`、`media.syncDirectory`、`media.blobPath`

## media.recoverCleanupExecutionsLocked

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：启动exclusive只恢复同opened epoch原批准/执行决定，不扫描产生授权

```go
func (s *localBlobStore) recoverCleanupExecutionsLocked(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 启动exclusive只恢复同opened epoch原批准/执行决定，不扫描产生授权

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.loadCleanupApproval`、`media.readCleanupExecution`、`media.finishCleanupExecutionLocked`

## media.confirmDeletedAssetLocked

- 级别：参考
- 文件：`modules/media/internal/cleanup_worker.go`
- 作用：历史COMMITTED deleted资产须符合完整人工决定与终局元数据

```go
func (s *localBlobStore) confirmDeletedAssetLocked(ctx context.Context, asset gen.Asset) error
```

**输入**

无

**输出**

无

**步骤**

1. 历史COMMITTED deleted资产须符合完整人工决定与终局元数据

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.loadCleanupApproval`、`media.readCleanupExecution`

## media.ParseByteRange

- 级别：参考
- 文件：`modules/media/internal/range_service.go`
- 作用：规范Range支持iOS视频

```go
func ParseByteRange(input ParseByteRangeInput) (ParseByteRangeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ParseByteRangeInput` | 规范Range支持iOS视频的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ParseByteRangeResult` | 规范Range支持iOS视频的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 无range或If-Range不匹配返回完整200
2. 仅单bytes范围支持N-M/N-/-N，归一到总长度，末端截断
3. 多范围/语法错误/不可满足返回VALIDATION映射416和Content-Range bytes */total

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## media.ReadAssetBytes

- 级别：约束
- 文件：`modules/media/internal/asset_delivery.go`
- 作用：鉴权读取immutable bytes和Range

```go
func ReadAssetBytes(ctx context.Context, op OperationContext, input ReadAssetBytesInput) (ReadAssetBytesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadAssetBytesInput` | 鉴权读取immutable bytes和Range的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadAssetBytesResult` | 鉴权读取immutable bytes和Range的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 公开请求须带manifest生成的documentId/publishedRevisionId，content当前发布门禁验证visible/matchesCurrent且资产属于该精确body/config引用；草稿仅绑定已登录管理员previewproof，未知关闭
2. AcquireReaderLease与ParseByteRange，读取immutable文件对应offset，保持Content-Length/Range/Accept-Ranges/ETag
3. image/publichash immutable永久缓存；SVG CSP sandbox/nosniff，附件安全Disposition；断开关流释放reader

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：reader lease/stream
- 授权：anonymous仅绑定仍可见精确发布引用与资源成员关系；管理员preview须已登录主体proof
- 并发：readerlease与final delete共享fence
- 幂等：同version+range确定bytes
- 依赖：`media.ParseByteRange`、`media.AcquireReaderLease`、`content:validatePublications`

## media.HeadAssetBytes

- 级别：约束
- 文件：`modules/media/internal/asset_delivery.go`
- 作用：HEAD同GET授权只返header

```go
func HeadAssetBytes(ctx context.Context, op OperationContext, input HeadAssetBytesInput) (HeadAssetBytesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `HeadAssetBytesInput` | HEAD同GET授权只返header的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HeadAssetBytesResult` | HEAD同GET授权只返header的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见 |

**步骤**

1. 执行与GET完全相同scope/state/version校验
2. 不打开全bytes stream
3. 返回ETag/长度/Accept-Ranges/CSP与cachepolicy

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）

- 副作用：无
- 授权：与ReadAssetBytes相同
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## media.AcquireReaderLease

- 级别：参考
- 文件：`modules/media/internal/reader_lease.go`
- 作用：防正在下载资源被最终删除

```go
func AcquireReaderLease(ctx context.Context, op OperationContext, input AcquireReaderLeaseInput) (AcquireReaderLeaseResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `AcquireReaderLeaseInput` | 防正在下载资源被最终删除的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcquireReaderLeaseResult` | 防正在下载资源被最终删除的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；NOT_FOUND: 资源不存在或在当前主体作用域中不可见 |

**步骤**

1. 仅已完成下载授权的本域纯media内部流程；流deadline在未来十分钟内且不超过op已获准deadline
2. 公共写窗口后asset锁→store→SQL核精确ready/version/hash及无持久删除决定，同事务保存有限reader lease
3. 仅Go结果返回精确readerId/expiresAt和Release回调，未来bytes入口必须以该期限取消流且close后Release，无心跳续期；当前bytes入口仍planned

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）

- 副作用：私有readerlease
- 授权：仅已完成下载授权的本域media纯服务内部链，无HTTP
- 并发：asset稳定锁→store→SQL，与最终删除决定互斥
- 幂等：独立有限readerId，Release按精确readerId/epoch重复安全
- 依赖：`media.cleanupStore`、`media.mediaWriteContext`、`media.readCleanupAsset`、`media.lockReferenceAssets`、`media.releaseReaderLease`

## media.releaseReaderLease

- 级别：参考
- 文件：`modules/media/internal/reader_lease.go`
- 作用：流close回调仅释放精确readerId/epoch，未知保留有限到期保护

```go
func (s *localBlobStore) releaseReaderLease(ctx context.Context, op OperationContext, readerID Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 流close回调仅释放精确readerId/epoch，未知保留有限到期保护

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：无

## media.BuildDeliveryManifest

- 级别：约束
- 文件：`modules/media/internal/manifest_service.go`
- 作用：给公开/管理员预览返回SHA URLs/srcset/封面

```go
func BuildDeliveryManifest(ctx context.Context, op OperationContext, input BuildDeliveryManifestInput) (BuildDeliveryManifestResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `BuildDeliveryManifestInput` | 给公开/管理员预览返回SHA URLs/srcset/封面的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildDeliveryManifestResult` | 给公开/管理员预览返回SHA URLs/srcset/封面的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 必须公开ref或登录绑定preview二者恰一，content validatePublications/validatePreviewGrant核权威
2. 每assetversion/hash必须ready且属于该精确body/config资源集合，不准任意借preview读文件
3. 公开生成带精确documentId/publishedRevisionId的稳定SHA资源URL，读取仍查当前发布门禁，字节immutable长期缓存；preview独立proofURL/no-store；imagevariants/srcset按exactref

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：service:public-site；actor/refs必须权威匹配
- 并发：publication验证后传输前再查visible
- 幂等：相同hash公开URL稳定
- 依赖：`content:validatePublications`、`content:validatePreviewGrant`、`media.ReadAsset`

## media.ReadVideoMetadata

- 级别：约束
- 文件：`modules/media/internal/video_service.go`
- 作用：插入时一次获取平台公开信息并本地封面

```go
func ReadVideoMetadata(ctx context.Context, op OperationContext, input ReadVideoMetadataInput) (ReadVideoMetadataResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadVideoMetadataInput` | 插入时一次获取平台公开信息并本地封面的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadVideoMetadataResult` | 插入时一次获取平台公开信息并本地封面的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力 |

**步骤**

1. RecognizePlatformURL识别四平台，不携账号cookie/auth，FetchProviderPublicMetadata bounded egress
2. 成功公开标题简介作者时长+ImportRemoteCover本地asset，BuildProviderEmbed核白名单
3. 失败返回manual_required并保留手填字段，不能假成功；源snapshot随块存，之后不自动重抓

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `UNSUPPORTED_CAPABILITY`：当前依赖不提供声明的必需能力（不可重试）

- 副作用：一次公开metadata网络抓取+本地封面上传；本轮不真实调用
- 授权：media.upload编辑文档功能
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：插入operationId+canonicalURL复用snapshot，之后不自动刷新
- 依赖：`media.RecognizePlatformURL`、`media.FetchProviderPublicMetadata`、`media.ImportRemoteCover`、`media.BuildProviderEmbed`

## media.RecognizePlatformURL

- 级别：参考
- 文件：`modules/media/internal/provider_registry.go`
- 作用：识别Bilibili/YouTube/抖音/TikTok链接

```go
func RecognizePlatformURL(ctx context.Context, op OperationContext, input RecognizePlatformURLInput) (RecognizePlatformURLResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RecognizePlatformURLInput` | 识别Bilibili/YouTube/抖音/TikTok链接的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecognizePlatformURLResult` | 识别Bilibili/YouTube/抖音/TikTok链接的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力 |

**步骤**

1. parse HTTPS URL禁userinfo/控制字符/本地地址
2. 精确匹配固定host与视频path/query规则，只提providerVideoId
3. 短链接经ResolveProviderRedirect有界白名单解析；未知平台UNSUPPORTED_CAPABILITY

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_CAPABILITY`：当前依赖不提供声明的必需能力（不可重试）

- 副作用：无
- 授权：私有插入validator
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：确定性
- 依赖：`media.ResolveProviderRedirect`

## media.ResolveProviderRedirect

- 级别：参考
- 文件：`modules/media/internal/provider_url_guard.go`
- 作用：仅平台短链接受控重定向解析

```go
func ResolveProviderRedirect(ctx context.Context, op OperationContext, input ResolveProviderRedirectInput) (ResolveProviderRedirectResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ResolveProviderRedirectInput` | 仅平台短链接受控重定向解析的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveProviderRedirectResult` | 仅平台短链接受控重定向解析的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 每hop校验scheme/host/DNS结果非loopback/private/linklocal；端口443
2. 最多3hop/5秒/64KiB，拒绝跨provider或开放重定向目标
3. 不发送cookie/auth，不记录原query敏感值；解析失败manualfallback

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：短URL有限HTTP读取
- 授权：私有平台适配
- 并发：每provider有限并发
- 幂等：operation内一次解析
- 依赖：无

## media.FetchProviderPublicMetadata

- 级别：参考
- 文件：`modules/media/internal/provider_fetch.go`
- 作用：provider接口抓公开非登录信息

```go
func FetchProviderPublicMetadata(ctx context.Context, op OperationContext, input FetchProviderPublicMetadataInput) (FetchProviderPublicMetadataResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `FetchProviderPublicMetadataInput` | provider接口抓公开非登录信息的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `FetchProviderPublicMetadataResult` | provider接口抓公开非登录信息的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. YouTube官方oEmbed，Bili公开view，TikTok官方oEmbed，抖音公开页面元信息；适配实现前静态profile核实capability
2. endpoint由固定provider构造，所有DNS/redirect按egressguard，超时5秒/响应≤1MiB/无cookieauth
3. 只保title/description/author/duration/cover候选；错误显式manual_required，不周期刷新

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `UNSUPPORTED_CAPABILITY`：当前依赖不提供声明的必需能力（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：一次有限公开HTTP读取
- 授权：私有provider adapter
- 并发：provider信号量限制
- 幂等：插入operationID去重
- 依赖：无

## media.BuildProviderEmbed

- 级别：参考
- 文件：`modules/media/internal/provider_embed.go`
- 作用：生成四平台白名单embed资料

```go
func BuildProviderEmbed(input BuildProviderEmbedInput) (BuildProviderEmbedResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `BuildProviderEmbedInput` | 生成四平台白名单embed资料的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildProviderEmbedResult` | 生成四平台白名单embed资料的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力 |

**步骤**

1. YouTube host youtube-nocookie、Bili player.bilibili、TikTok官方播放器、抖音开放平台profile
2. 视频ID仅URL编码后放固定template参数，不接受用户embedURL
3. 未核实profile输出ready=false/manual_required；public-site点击前只封面，不加载第三方script/tracker

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_CAPABILITY`：当前依赖不提供声明的必需能力（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## media.ImportRemoteCover

- 级别：参考
- 文件：`modules/media/internal/cover_import.go`
- 作用：下载平台封面到本地内容寻址资源

```go
func ImportRemoteCover(ctx context.Context, op OperationContext, input ImportRemoteCoverInput) (ImportRemoteCoverResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ImportRemoteCoverInput` | 下载平台封面到本地内容寻址资源的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ImportRemoteCoverResult` | 下载平台封面到本地内容寻址资源的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 只provider profile允许coverhosts，每DNS/hop禁SSRF，最长5s/10MiB
2. 流hash/MIME像素元数据校验，禁止HTML/SVG主动内容，服务不重编码
3. 通过本域blob完成流程生成ready平台封面；失败返回null和manual_required，不外链防盗链图

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：公开封面下载/私有blob
- 授权：私有metadata插入流程
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：同SHA复用
- 依赖：`media.CommitAssetBlob`

## media.RecoverBlobJournal

- 级别：参考
- 文件：`modules/media/internal/blob_recovery.go`
- 作用：恢复blob完成/删除持久日志

```go
func RecoverBlobJournal(ctx context.Context, op OperationContext, input RecoverBlobJournalInput) (RecoverBlobJournalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RecoverBlobJournalInput` | 恢复blob完成/删除持久日志的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverBlobJournalResult` | 恢复blob完成/删除持久日志的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 启动exclusive先恢复明确人工批准且同opened epoch的DELETING，未知/跨epoch失败关闭
2. 每条DELETING必须对应完整原执行与终局；COMMITTED ready/delete_pending核真实bytes，deleted核完整原人工终局证明
3. PREPARED孤儿隔离，ready/delete_pending共享bytes保留，不按年龄扫描ready

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：本域日志与孤儿隔离
- 授权：service:media启动控制
- 并发：exclusive gate
- 幂等：重复不产生新delete授权
- 依赖：`media.hashFile`、`media.recoverCleanupExecutionsLocked`、`media.confirmDeletedAssetLocked`

## media.PublishAssetEvent

- 级别：参考
- 文件：`modules/media/internal/outbox_repository.go`
- 作用：资源ready或人工deleted事务写最小outbox，真实generation由调用链提供

```go
func PublishAssetEvent(ctx context.Context, op OperationContext, input PublishAssetEventInput) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PublishAssetEventInput` | 原子写最小资源状态outbox的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `Error` | 同事务事件登记失败；业务事务整体回滚 | 无错误才可提交ready事务 |

**步骤**

1. 同业务事务写media.asset.changed最小outbox，ready为0，deleted为原人工批准精确generation，不执行RPC

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：私有assettxn
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：eventId去重
- 依赖：`platform.EnqueueEvent`

## media.RelayOutbox

- 级别：约束
- 文件：`modules/media/internal/outbox_repository.go`
- 作用：本域已提交outbox的真实单批转发，机制统一由平台执行

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

1. 仅可信本域media纯服务后台与opened epoch；raw jobs客户端和真实运行/写窗口已装配，不接受HTTP或用户委托触发
2. 将本域私有SQLite outbox、真实gen.JobsClient的publishEvent端口及本域窗口装配给platform.RunRelayBatch；按公共batchLimit 1..100，本函数只执行一批，不自造调度/租约/重试机制
3. 公共机制按已提交行领取、原eventId/dataEpoch幂等真实提交，jobs持久确认后本地标relayed；jobs序号由jobs持久权威保存，不在本域复制。失败保留/退避、冲突隔离、旧epoch处理均由平台统一
4. 持续调度由受信背景注册以平台RunRecoveryLoop每秒串行调用本单批；调度预检窗口和批内逐IO窗口分别建立，任何RPC后本地确认仍重核窗口，不持本域业务锁跨RPC

**错误**

- `FORBIDDEN`：非本域可信纯服务后台（不可重试）
- `DEPENDENCY_UNAVAILABLE`：客户端/存储/窗口或jobs未知，行保留（可重试）
- `VALIDATION`：单批参数或窗口装配不合法（不可重试）
- `INTEGRITY_ERROR`：本域持久outbox材料损坏，未知失败关闭；成功隔离由quarantined计数报告（不可重试）
- `VERSION_MISMATCH`：op不属于opened epoch；公共机制只在合法窗口处理旧代行（不可重试）

- 副作用：单批本域outbox租约/状态推进；仅调用版本化jobs:publishEvent，不执行ready文件删除
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个转发循环；行级租约防多实例重复领取；同subject按outbox序号串行发送
- 幂等：eventId在业务事务内固定；jobs按eventId+payloadHash去重，重复发送返回原确认
- 依赖：`jobs:publishEvent`、`platform.RunRelayBatch`、`platform.NewEventPublisher`、`media.cleanupStore`

## media.AcquireBarrier

- 级别：约束
- 文件：`modules/media/internal/barrier_service.go`
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
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.EnterBarrier`

## media.ReleaseBarrier

- 级别：约束
- 文件：`modules/media/internal/barrier_service.go`
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
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.ReleaseBarrier`

## media.ExportSnapshot

- 级别：约束
- 文件：`modules/media/internal/snapshot_service.go`
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
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.Export`

## media.StageSnapshot

- 级别：约束
- 文件：`modules/media/internal/snapshot_service.go`
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

## media.DiscardSnapshot

- 级别：约束
- 文件：`modules/media/internal/snapshot_service.go`
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

## media.main

- 级别：约束
- 文件：`modules/media/cmd/media/main.go`
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
- 依赖：`media.BuildApplication`、`media.Serve`、`media.Shutdown`、`platform.RunService`

## media.BuildApplication

- 级别：约束
- 文件：`modules/media/internal/app/wire.go`
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
7. 受信安装opened epoch与真实running写窗口，启动核chunk水位；统一工厂只绑定真实已实现handler，缺其余操作失败并关本地库，不填占位
8. WriteWindow回调第二次runtime读取先核DataEpoch：未知503，已知不等于Build打开epoch为409，再判phase/maintenance；不能只信NewWriteContext第一次view
9. 真实raw jobs客户端同时用于原人工批准CreateJob和公共窄worker适配，生产者响应schema由本域规格别名提供；构造两个背景闭包但只在全量handler/恢复门槛成功后安装，不启动部分应用，loop/relay由平台启动和停止

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`media.RecoverBlobJournal`、`media.ExpireUploadSessions`、`media.RelayOutbox`、`media.ConsumeEvents`、`media.RunWorker`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`media.All`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`media.InstallLocalBlobStore`、`media.ConfigureWriteWindow`、`media.RecoverUploadChunks`、`media.NewApplication`、`media.CloseLocalBlobStore`、`identity:checkAuthorization`、`platform.NewIdentityAuthorizer`、`platform.NewServiceHandler`、`media.InstallReferenceDecisionClients`、`media.InstallCleanupJobsClient`、`media.InstallAssetListingKey`、`media.NewCleanupWorkerClient`、`media.NewMediaBackgroundTasks`、`media.InstallBackgroundTasks`、`platform.NewSnapshotManager`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## media.Serve

- 级别：约束
- 文件：`modules/media/internal/transport/server.go`
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
- 依赖：`media.BuildApplication`、`media.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## media.Shutdown

- 级别：约束
- 文件：`modules/media/internal/transport/server.go`
- 作用：平台等待背景任务结束后关闭本域资源，重复调用返回同一次结果

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

1. 公共RunService先取消背景任务，再调用Drain停HTTP入口且不关库，并等待全部任务至持久安全点
2. 平台确认背景均退出后调用本函数；同一次Drain结果优先，随后CloseLocalBlobStore关闭私有DB/句柄并标closed
3. Drain或资源关闭失败保留恢复决定并返回真实错误；不在后台仍可能写库时抢先关闭或清理持久人工决定

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：30秒期限仍有未安全在途步骤（可重试）
- `INTERNAL` / HTTP 500：flush/close失败（不可重试）

- 副作用：本域drain、资源关闭、durable恢复日志
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：sync.Once启动drain；其他调用等待同一次结果
- 幂等：重复Shutdown返回同一次关闭结果；已closed成功
- 依赖：`media.Drain`、`media.CloseLocalBlobStore`
- 其他依赖说明：本域barrier安全点函数；本域journal/outbox flush

## media.Health

- 级别：约束
- 文件：`modules/media/internal/transport/server.go`
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

## media.Ready

- 级别：约束
- 文件：`modules/media/internal/transport/server.go`
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

## media.NewApplication

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：真实HTTP入口，缺任一handler平台失败关闭，control未完成不假造Ready

```go
func NewApplication(config platform.ServiceConfig,anchors platform.TrustAnchors,runtimeAuthority platform.SiteConfigClient,activeDataEpoch platform.Identifier, authorize func(context.Context,platform.OperationContext,string,string)error) (*Application,error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实HTTP入口，缺任一handler平台失败关闭，control未完成不假造Ready

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`platform.NewServiceHandler`、`media.serviceHandlers`、`media.LocalStorageRecovered`、`media.BackgroundTasks`、`media.Drain`

## media.serviceHandlers

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：真实已实现上传/引用/人工计划及ListAssets/ListUnusedAssets HTTP转换，严格查询不填占位

```go
func (a *Application) serviceHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 真实已实现上传/引用/人工计划及ListAssets/ListUnusedAssets HTTP转换，严格查询不填占位
2. 列表limit仅转换入口已验证整数，按固定kin-openapi同base0解析，不重复schema/范围校验；省略为20

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.Health`、`media.Ready`、`media.BeginUpload`、`media.ReadUpload`、`media.AbortUpload`、`media.serveUploadChunk`、`media.ReadAsset`、`media.writeRequestProblem`、`media.CompleteUpload`、`media.AcquireResourceReferences`、`media.ConfirmResourceReferences`、`media.ReleaseResourceReferences`、`media.ReadAssetReferences`、`media.ProposeManualCleanup`、`media.CancelManualCleanup`、`media.ConfirmManualCleanup`、`media.ListAssets`、`media.ListUnusedAssets`

## media.serveUploadChunk

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：严格入口后转换header/path，用解析Content-Length绑定io.Reader，无handle表

```go
func serveUploadChunk(w http.ResponseWriter,r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 严格入口后转换header/path，用解析Content-Length绑定io.Reader，无handle表

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.PutUploadChunk`、`media.writeRequestProblem`

## media.writeRequestProblem

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：受信requestId输出安全Problem，不回显正文/私有路径

```go
func writeRequestProblem(w http.ResponseWriter,r *http.Request,err error)
```

**输入**

无

**输出**

无

**步骤**

1. 受信requestId输出安全Problem，不回显正文/私有路径

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：无

## media.InstallBackgroundTasks

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：仅完整构造成功后安装复制的背景闭包，不运行任务

```go
func (a *Application) InstallBackgroundTasks(tasks []platform.BackgroundTask)
```

**输入**

无

**输出**

无

**步骤**

1. 仅完整构造成功后安装复制的背景闭包，不运行任务

**错误**

- 无

- 副作用：仅初始化时安装独立背景闭包切片，不启动任务
- 授权：仅受信启动或平台生命周期；不提供HTTP/浏览器入口
- 并发：初始化顺序装配；任务由平台取消/等待，Drain不关库，Shutdown最后关闭
- 幂等：保留原任务/事件/决定；不产生新人工批准，不执行时不产生业务副作用
- 依赖：无

## media.BackgroundTasks

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：公共生命周期读取已安装闭包；此处不claim、不转发、不删除

```go
func (a *Application) BackgroundTasks() []platform.BackgroundTask
```

**输入**

无

**输出**

无

**步骤**

1. 公共生命周期读取已安装闭包；此处不claim、不转发、不删除

**错误**

- 无

- 副作用：无
- 授权：仅受信启动或平台生命周期；不提供HTTP/浏览器入口
- 并发：初始化顺序装配；任务由平台取消/等待，Drain不关库，Shutdown最后关闭
- 幂等：保留原任务/事件/决定；不产生新人工批准，不执行时不产生业务副作用
- 依赖：无

## media.Drain

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：仅排空HTTP入口，不关闭私有库；平台取消并等待背景后再Shutdown

```go
func (a *Application) Drain(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 公共RunService先取消背景任务；此方法只CAS ready→draining并http.Server.Shutdown排空在途HTTP，不关闭私有库或清理决定
2. 同实例sync.Once保存真实排空结果；背景安全点由公共生命周期等待，完成后才Shutdown关库

**错误**

- 无

- 副作用：HTTP入口drain与生命周期状态
- 授权：仅受信启动或平台生命周期；不提供HTTP/浏览器入口
- 并发：初始化顺序装配；任务由平台取消/等待，Drain不关库，Shutdown最后关闭
- 幂等：同实例返回同一次排空结果，超时保留恢复材料
- 依赖：无

## media.ConsumeEvents

- 级别：约束
- 文件：`modules/media/internal/event_subscription.go`
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
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`media.ApplyReferenceEvent`、`platform.RunConsumer`

## media.RunWorker

- 级别：约束
- 文件：`modules/media/internal/worker.go`
- 作用：主动领取并执行本域登记的持久任务

```go
func RunWorker(ctx context.Context, op OperationContext, input WorkerLoopInput) (WorkerLoopResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `WorkerLoopInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WorkerLoopResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 受信启动以NewCleanupWorkerClient构建真实platform.WorkerJobsClient窄端口，严格使用jobs生产方六Result schema；缺客户端/运行权威明确失败关闭，不补虚假订阅方法
2. 只注册media.cleanup并调用platform.RunWorkerLoop；claim/start、十秒续租三十秒、取消/fence/checkLease与complete/fail全部由平台处理
3. cleanupWorkerOperation从完整ClaimedJob.InputRef读取原批准/原AcceptedJob/waiting，绑定jobId/op/root/site/epoch；领取时新建十分钟期限及纯media主体，清proof/grants
4. 平台向执行器保留完整ObjectRef/WorkerID/fence及CheckLease guard，同进程直接Execute；统计owner执行结果，jobs写回仍由平台真实租约核对
5. 执行器首次批准读取、缺原材料及Execute错误统一进入cleanupWorkerFailure；仅当前task ctx已取消或公共CANCELLED cause时，用一秒独立只读快照核原批准/job/计划/execution，原完整终局优先Succeeded，无execution才Cancelled，未终局DELETING/未知不谎报安全取消；op缺失仍FORBIDDEN
6. 公共循环每次Claim使用当前runtime及独立纯media/system服务上下文，Claim不读取本站对象；领取后cleanupWorkerOperation仍从原批准恢复真实site/epoch/root，所有任务对象RPC沿该已核上下文并由服务断言签名，不猜payload站点

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用；退避后重试领取（可重试）

- 副作用：HTTP调用jobs租约接口；执行本域登记的任务函数
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个领取循环，并发不超过maxConcurrency；同一任务只由持有有效fence者写回
- 幂等：jobs按jobId与fence去重；执行函数自身按输入引用幂等
- 依赖：`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`、`media.ExecuteManualCleanup`、`platform.RunWorkerLoop`、`media.ReadCleanupApprovalObject`、`media.cleanupStore`、`media.cleanupWorkerOperation`、`media.loadCleanupApproval`、`media.cleanupWorkerFailure`

## media.cleanupWorkerOperation

- 级别：参考
- 文件：`modules/media/internal/worker.go`
- 作用：完整ClaimedJob绑定原批准/任务回执，领取时新建期限及纯media主体

```go
func (s *localBlobStore) cleanupWorkerOperation(ctx context.Context, claimed platform.ClaimedJob) (OperationContext, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整ClaimedJob绑定原批准/任务回执，领取时新建期限及纯media主体

**错误**

- 无

- 副作用：无
- 授权：可信内部调用链；入口身份/schema一次，批准/epoch/决定由本边界核对
- 并发：RPC锁外；asset稳定序→store→SQL，纯转换不加锁
- 幂等：保留原规范材料/原结果，未知失败关闭
- 依赖：`media.loadCleanupApproval`、`media.checkCleanupJobBinding`

## media.cleanupWorkerFailure

- 级别：参考
- 文件：`modules/media/internal/worker.go`
- 作用：任务真实取消后只读同快照证明安全取消，原完整终局成功优先，未知/未终局决定不作安全取消

```go
func (s *localBlobStore) cleanupWorkerFailure(ctx context.Context, op OperationContext, job platform.JobExecutionInput, executeErr error) (platform.JobExecutionResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅任务ctx已取消或公共CANCELLED cause才进入；原task ctx取消后新建一秒独立只读事务，不触发RPC/写入
2. 同快照核完整原批准/JobExecutionInput/真实计划与execution；显式Rollback结束快照后，完整终局JobSucceeded，无execution才JobCancelled，未终局DELETING或任何未知保留原决定并失败

**错误**

- 无

- 副作用：无
- 授权：已绑定原批准与jobs任务的可信本域worker链，安全取消不产生删除许可
- 并发：独立有界只读SQL快照，不持asset/store锁，不触发RPC
- 幂等：仅报告原持久事实，不修改计划、决定或原结果
- 依赖：`media.loadCleanupApproval`、`media.checkCleanupJobBinding`、`media.readCleanupPlan`、`media.readCleanupExecution`

## media.NewCleanupWorkerClient

- 级别：参考
- 文件：`modules/media/internal/worker.go`
- 作用：用拥有方六Result唯一schema装配公共真实worker适配，不复制协议转换

```go
func NewCleanupWorkerClient(client *gen.JobsClient) (platform.WorkerJobsClient, error)
```

**输入**

无

**输出**

无

**步骤**

1. OpenAPI顶级x-runtime-payload-schemas显式登记jobs拥有的六Result别名；使用生成gen.PayloadValidationSpec的自包含schema闭包创建严格校验器，不借被HTTP prune裁剪的GetSwagger，不扩大HTTP DTO
2. 按生产者六Result原名map装配platform.NewJobsClient，协议转换/租约机制由公共实现，域不手写响应投影或伪造接口

**错误**

- 无

- 副作用：无
- 授权：仅受信启动或平台生命周期；不提供HTTP/浏览器入口
- 并发：初始化顺序装配；任务由平台取消/等待，Drain不关库，Shutdown最后关闭
- 幂等：保留原任务/事件/决定；不产生新人工批准，不执行时不产生业务副作用
- 依赖：`platform.NewJobsClient`

## media.NewMediaBackgroundTasks

- 级别：参考
- 文件：`modules/media/internal/worker.go`
- 作用：只构造两个可信背景任务：平台worker和统一周期调用本域relay单批，未完整装配不启动

```go
func NewMediaBackgroundTasks(op OperationContext, jobs platform.WorkerJobsClient) ([]platform.BackgroundTask, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只构造两个可信背景任务：平台worker和统一周期调用本域relay单批，未完整装配不启动

**错误**

- 无

- 副作用：无
- 授权：仅受信启动或平台生命周期；不提供HTTP/浏览器入口
- 并发：初始化顺序装配；任务由平台取消/等待，Drain不关库，Shutdown最后关闭
- 幂等：保留原任务/事件/决定；不产生新人工批准，不执行时不产生业务副作用
- 依赖：`media.cleanupStore`、`media.RunWorker`、`media.RelayOutbox`、`platform.RunRecoveryLoop`

## media.All

- 级别：参考
- 文件：`modules/media/migrations/migrations.go`
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

## media.InstallReferenceDecisionClients

- 级别：参考
- 文件：`modules/media/internal/reference_decision_client.go`
- 作用：构造期在已打开store安装生成的固定content/site-config只读源客户端

```go
func InstallReferenceDecisionClients(content *gen.ContentClient, config *gen.SiteConfigClient) error
```

**输入**

无

**输出**

无

**步骤**

1. 构造期在已打开store安装生成的固定content/site-config只读源客户端

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：无

## media.readReferenceDecision

- 级别：参考
- 文件：`modules/media/internal/reference_decision_client.go`
- 作用：无锁纯media服务上下文经固定来源API读取公共schema最小决定，逐字段核caller来源/epoch/完整材料；404及pending不作为释放证据

```go
func (s *localBlobStore) readReferenceDecision(ctx context.Context, op OperationContext, material AcquireResourceReferencesInput) (ReferenceSourceDecision, error)
```

**输入**

无

**输出**

无

**步骤**

1. 无锁纯media服务上下文经固定来源API读取公共schema最小决定，逐字段核caller来源/epoch/完整材料；404及pending不作为释放证据
2. 唯一公共schema仍由内嵌规格严格校验；本域ReferenceSourceDecision只作解码投影，不依赖oapi-codegen保留未用于HTTP的DTO，不扩大生成器或添加伪HTTP端点

**错误**

- 无

- 副作用：无
- 授权：固定来源纯服务已在引用入口核实；schema仅边界/持久与远端材料处核
- 并发：源决定RPC与运行窗口不持业务锁；sourceOperation→sourceObject→asset升序→store→SQL
- 幂等：完整来源绑定与终局原回执，不从mutable状态重建
- 依赖：`content:readReferenceDecision`、`site-config:readHomeReferenceDecision`

## media.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/media/internal/snapshot_archive.go`
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
