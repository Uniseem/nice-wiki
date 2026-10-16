# media 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 119 个函数：约束 36 个，参考 83 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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

1. 持上传锁，PREPARED/file/rename/fsync后，同事务CAS complete、ready、原结果、COMMITTED与outbox
2. 创建新ready资源的同事务建立reference_objects逐对象fence=0/referenceGeneration=0；既有迁移只在0003首次应用时回填，不为丢失fence的旧对象运行时补造

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
- 作用：资源库列图片/附件/视频/画板及引用摘要

```go
func ListAssets(ctx context.Context, op OperationContext, input ListAssetsInput) (ListAssetsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ListAssetsInput` | 资源库列图片/附件/视频/画板及引用摘要的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListAssetsResult` | 资源库列图片/附件/视频/画板及引用摘要的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. media.manage功能权限
2. 一致水位按kind/createdAt/id分页
3. 引用详情单独ReadAssetReferences按docId显示，不自动删

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：media.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

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

1. 原DrawingScene schema经OpenAPI components引用进入唯一生成GetSwagger，由平台NewPayloadValidator完整验证；版本只接受2，未知字段/类型/坐标原schema拒绝，files等价additionalProperties值schema+propertyNames模式不改变持久版本
2. Drawing专属语义核文件键原精确^[A-Za-z0-9._:-]{1,128}$与key/id一致、元素ID唯一、frame/bound/container/binding/file存在与类型、frame无环、group无重复、freedraw压力长度与points一致
3. files只受限raster base64 data URL≤32MiB，MIME与签名及元数据一致，禁止SVG/外部URL；元素链接仅http/https/mailto，不执行HTML；按原element顺序收非deleted text，≤1048576字符，不写日志

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
4. 保持源互斥后按assetId稳定顺序取得对象mutex，再取本域store提交互斥，在写事务内重查来源未sealed；资产必须真实ready且站点/版本/hash/长度/MIME与唯一schema及索引一致，缺逐对象fence或delete_pending一律失败关闭，不推测尚未实现的清理计划；CAS逐对象deletionFence与referenceGeneration加一，完整来源、逐asset fence和至少24小时pending pin原回执同事务
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
- 依赖：`platform.CanonicalJSON`、`media.validateReferenceSource`、`media.canonicalReferenceInput`、`media.lockReferenceKey`、`media.lockReferenceAssets`、`media.readReferenceSource`、`media.acquireReferencePin`、`media.mediaWriteContext`

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

1. 核caller与持久pin的源site/object/operation/revision/version/digest归属；sourceRevisionId/sourceVersion与原acquire声明一致
2. 读取注册源最小持久决定，必须当前epoch且COMMITTED、材料一致；未知/404/pending保留pin并失败关闭
3. 同事务将该sourceObjectId的新完整集合确认，旧current保留history；较低sourceVersion的已提交pin只补history，不回退current水位，不以STALE丢历史
4. 空集合也推进source水位并将旧current转history；同pin重复确认完整原回执，不重复generation；确认取消delete_pending并增referenceGeneration
5. 与Acquire/Release使用同源互斥及asset稳定顺序；来源已sealed却被证明COMMITTED属于完整性矛盾，失败关闭，不复活来源

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：源持久决定未知或依赖失败，保留pin等待恢复（可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：service对应source
- 并发：source key CAS+assetfence
- 幂等：同pin同revision复用，异源CONFLICT
- 依赖：`site-config:readHomeReferenceDecision`、`content:readReferenceDecision`

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

1. 核固定content/site-config纯服务和完整site/object/revision/version/operation/digest材料；已知pinId必须属于同材料，pinId为null只表示调用方未知，不是无pin证据
2. 不持本地源/对象/SQL写锁，按固定源API读取持久决定；必须同epoch与所有源材料且reason对应明确aborted/rejected，COMMITTED、404、pending、超时、依赖或完整性未知均保留且不得封存
3. 回源后重新核当前epoch/维护写窗口；先取caller/epoch/sourceOperationId源互斥，再取已存pin的assetId稳定顺序mutex，事务内核既有来源绑定；确认current/history或COMMITTED矛盾失败关闭，不得删除或降级；nullable pinId仅是已知原pin的一致性提示，不参与授权，也不作为无pin推断
4. 在同事务持久sealed终止来源和完整材料、释放该来源pending pin并增generation；若在同源锁/CAS下确实没有pin也持久sealed，返回released=true；不查当前asset存在/ready来证明无pin，不需要重新acquire；保留来源材料与已知原pinId的终局回执，不删除封存依据
5. Acquire和Release同源锁且写前重查sealed，覆盖从未有pin、远端成功本域未收到pin、与释放并发的在途acquire；只有封存/释放事务成功才回成功，重复同来源/同材料只回原安全结果，异材料冲突
6. 到期不跳过源决定核对；旧已提交版本须历史确认，不因current更高或expiry自动删除ready文件

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：service对应source
- 并发：回源RPC不持写锁；源caller/epoch/sourceOperationId互斥先于assetId稳定顺序，封存与pin释放同事务并阻止迟到acquire
- 幂等：同源完整材料+终止reason持久封存并重复成功，pinId可null；异材料冲突，无pin也不能省略来源核验
- 依赖：`site-config:readHomeReferenceDecision`、`content:readReferenceDecision`

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

1. 验证producer与payloadschema/epoch
2. eventId去重，sourceVersion低于当前只ack不覆写
3. 事务应用完整snapshot与history分类，增generation并取消冲突清理计划
4. 已经明确终止sealed的同源修订不得由迟到事件复活；与持久终止来源矛盾时INTEGRITY_ERROR，保留原引用保护

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：service:jobs可信事件投递
- 并发：sourceaggregate+asset锁稳定排序
- 幂等：eventId唯一+sourceversion单调
- 依赖：`platform.ClaimInbox`

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
- 作用：已持source/asset锁后store串行；原回执优先，新窗口内事务核ready及逐对象fence，CAS+24h pin/原回执同事务

```go
func (s *localBlobStore) acquireReferencePin(ctx context.Context, op OperationContext, input AcquireResourceReferencesInput, bindingHash Hash, assets []gen.AssetVersionRef) (AcquireResourceReferencesResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 已持source/asset锁后store串行；原回执优先，新窗口内事务核ready及逐对象fence，CAS+24h pin/原回执同事务
2. 逐对象fence/generation只以SQLite int64最大值为耗尽边界；达到最大值LOCKED，不把合法最大值记作损坏，不使用额外JavaScript安全整数上限

**错误**

- 无

- 副作用：无
- 授权：已授权本域引用链或受信装配，入口身份/schema一次
- 并发：源锁先于asset稳定序锁，再取store提交互斥；SQL CAS失败关闭
- 幂等：完整源材料和原回执持久绑定，不从当前状态重建
- 依赖：`media.readReferenceSource`、`media.mediaWriteContext`、`media.readReferenceAsset`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. media.manage或本域cleanup调用
2. 从content与site-config两个资源权威拥有源获取完整当前/历史快照和watermark；localization/public-site只派生同一不可变引用，不独立增减ref
3. 未知/未完整返回DEPENDENCY_UNAVAILABLE，不能显示未引用；history单列风险

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：media.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content:listResourceReferences`、`site-config:listResourceReferences`

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

1. media.manage且ValidateCleanupSources完成全部source快照
2. 当前confirmedref和pendingpin必须0；历史ref只产生历史缺图风险提示
3. 只列表无删除副作用，不设周期自动清ready任务

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：media.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`media.ValidateCleanupSources`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 核media.manage，读取每asset当前完整引用且无pending
2. 列出历史涉及文档，明确清理可能让历史/回滚缺图片文件
3. 写proposed plan+hash+source watermarks，不删bytes、不自动确认

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：人工清理计划metadata
- 授权：media.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：operationId复用精确plan
- 依赖：`media.ValidateCleanupSources`

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

1. 确认摘要/精确集合，历史非空必须ack
2. 再次核current refs/pins/sourcewatermarks，一有新引用计划blocked
3. 事务confirmed/waiting与notBefore≥24h，只有此人工批准计划可创建jobs延迟执行

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：media.manage
- 并发：planCAS+assetdeletionfence
- 幂等：同confirmationhash仅一次授权
- 依赖：`media.ValidateCleanupSources`、`jobs:createJob`

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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用 |

**步骤**

1. planstate尚非deleted
2. 核media.manage与计划scope
3. 改cancelled并解除标记，已deleting完成则返回CONFLICT

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）

- 副作用：私有事务/文件/receipt/outbox
- 授权：media.manage
- 并发：与executedelete同fence
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：无

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

1. 逐source调用完整snapshot，对齐epoch与watermark；不读取对方目录
2. current或pending任一>0返回safe=false，history仅警告
3. 任一unknown/retryfailure/分页未完成都false且DEPENDENCY_UNAVAILABLE，不降级忽略

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

1. jobs worker claim确认plan/notBefore/approvalhash，未知或未确认拒绝
2. ValidateCleanupSources全量权威检查；assetmutex再CAS generation/fence，pendingpin/newref/activeReader出现取消该asset
3. 先持久delete决定再unlink本服务generated blob；dedup其他asset/ref非0不删共享bytes；journal恢复补删，事件deleted

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：经人工确认不可逆ready文件删除
- 授权：service:media worker；必须人工confirmed plan，不是自动清理
- 并发：对象mutex/generation/fence/readers0最后检查
- 幂等：planId/assetversion决定日志去重
- 依赖：`media.ValidateCleanupSources`、`media.PublishAssetEvent`

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

1. assetmutex核ready/version/hash且无committeddelete决定
2. 记录短期readerlease并增加fenceguard
3. 流close释放；deadline过期前heartbeat保持，过期断流，不无限阻塞cleanup

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）

- 副作用：私有readerlease
- 授权：media私有download流程
- 并发：对象mutex
- 幂等：请求readerId单次
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

1. 启动exclusive检查COMMITTEDblob哈希/metadata
2. PREPARED未visible孤儿隔离；DELETING仅已人工批准决定可补删
3. 未知日志关闭对应asset，绝不扫描ready文件自动清

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：本域日志与孤儿隔离
- 授权：service:media启动控制
- 并发：exclusive gate
- 幂等：重复不产生新delete授权
- 依赖：`media.hashFile`

## media.PublishAssetEvent

- 级别：参考
- 文件：`modules/media/internal/outbox_repository.go`
- 作用：原子写最小资源状态outbox

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

1. 仅新建ready资源时从安全Asset构造siteId/ref/state/referenceGeneration=0/occurredAt最小载荷
2. 在调用方同一本域sql.Tx用platform.EnqueueEvent写固定eventId；业务回滚则事件也不存在；不直接调用jobs，RelayOutbox尚未实施

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
- 依赖：`site-config:readRuntimeState`

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
- 依赖：`site-config:readRuntimeState`

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
- 依赖：`site-config:readRuntimeState`

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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`

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
- 依赖：`site-config:readRuntimeState`

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

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`media.RecoverBlobJournal`、`media.ExpireUploadSessions`、`media.RelayOutbox`、`media.ConsumeEvents`、`media.RunWorker`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`media.All`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`media.InstallLocalBlobStore`、`media.ConfigureWriteWindow`、`media.RecoverUploadChunks`、`media.NewApplication`、`media.CloseLocalBlobStore`、`identity:checkAuthorization`、`platform.NewIdentityAuthorizer`、`platform.NewServiceHandler`
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
- 依赖：`media.CloseLocalBlobStore`
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
- 依赖：`platform.NewServiceHandler`、`media.serviceHandlers`、`media.LocalStorageRecovered`

## media.serviceHandlers

- 级别：参考
- 文件：`modules/media/internal/transport/server.go`
- 作用：真实health/ready及Begin/Read/Abort/Chunk/Complete/ReadAsset转换，不填占位

```go
func (a *Application) serviceHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 真实health/ready及Begin/Read/Abort/Chunk/Complete/ReadAsset转换，不填占位

**错误**

- 无

- 副作用：无
- 授权：受信装配或已授权调用链；入口身份/schema一次，文件/epoch/CAS由本边界负责
- 并发：本域私有store互斥及SQL CAS
- 幂等：原来源及规范摘要绑定，未知失败关闭
- 依赖：`media.Health`、`media.Ready`、`media.BeginUpload`、`media.ReadUpload`、`media.AbortUpload`、`media.serveUploadChunk`、`media.ReadAsset`、`media.writeRequestProblem`、`media.CompleteUpload`、`media.AcquireResourceReferences`

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

1. 进程启动后循环调用jobs:claimJob，只领取本域implementation.json jobTypes登记的类型；维护或屏障非released时暂停领取
2. 领取后调用jobs:startJob确认开始，按jobType调用登记的执行函数；每10秒jobs:heartbeatJob续30秒租约、上报进度并读取cancelRequested
3. 执行函数写回结果前调用jobs:checkLease核对fence仍有效；成功jobs:completeJob，失败jobs:failJob并给出可重试分类；收到取消在安全点停止并以cancelled结束
4. lease过期或fence不符立即停止写入；已派发的模型调用不重放，由网关结算或对账

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用；退避后重试领取（可重试）

- 副作用：HTTP调用jobs租约接口；执行本域登记的任务函数
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个领取循环，并发不超过maxConcurrency；同一任务只由持有有效fence者写回
- 幂等：jobs按jobId与fence去重；执行函数自身按输入引用幂等
- 依赖：`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`、`media.ExecuteManualCleanup`、`platform.RunWorkerLoop`

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
