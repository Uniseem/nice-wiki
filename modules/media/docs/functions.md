# 函数填空合同

每个条目与 [implementation.json](../contracts/implementation.json) 一一对应。签名中的领域类型见 types.json；标准 Problem 描述返回 error。未来新增任何私有助手也必须先登记契约。

## media.BeginUpload

文件：`modules/media/internal/upload_service.go`。

`func BeginUpload(ctx RequestContext, input BeginUploadInput) (BeginUploadResult, error)`

开始内容寻址可续传上传。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：BeginUploadInput。开始内容寻址可续传上传的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：BeginUploadResult。开始内容寻址可续传上传的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. media.upload+epoch验证，NormalizeOriginalName与站点大小/并发/配额预留。
2. 同site+sha已ready返回完成复用；不同内容不泄漏其他站点dedup。
3. 创建24h receiving与服务生成tmp路径，operationId唯一receipt。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：上传属于编辑文档功能media.upload；资源库读取/清理media.manage；公开交付仅已公开暴露hash或有效管理员preview。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：media.NormalizeOriginalName；site-config:readSiteConfig。

## media.ReadUpload

文件：`modules/media/internal/upload_service.go`。

`func ReadUpload(ctx RequestContext, input ReadUploadInput) (ReadUploadResult, error)`

查询续传offset。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadUploadInput。查询续传offset的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadUploadResult。查询续传offset的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见。

**步骤**

1. 限定上传者或资源管理功能。
2. 一致读取receivedBytes/state/expiry。
3. 不返回tmp路径/secret。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false。

**副作用**：无。

**授权**：media.upload本人或media.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## media.AbortUpload

文件：`modules/media/internal/upload_service.go`。

`func AbortUpload(ctx RequestContext, input AbortUploadInput) (AbortUploadResult, error)`

取消未完成上传释放配额。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：AbortUploadInput。取消未完成上传释放配额的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AbortUploadResult。取消未完成上传释放配额的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；NOT_FOUND: 资源不存在或在当前主体作用域中不可见。

**步骤**

1. 限定owner，ready不接受自动删。
2. 事务stateaborted与预留释放。
3. 只清该receivingtmp；并发complete谁先commit由CAS决定。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：media.upload本人。

**并发**：upload mutex。

**幂等**：重复abort成功。

**依赖**：无跨函数/跨域依赖。

## media.NormalizeOriginalName

文件：`modules/media/internal/upload_validator.go`。

`func NormalizeOriginalName(input NormalizeOriginalNameInput) (NormalizeOriginalNameResult, error)`

清理展示文件名而不用于路径。

**输入**

- input：NormalizeOriginalNameInput。清理展示文件名而不用于路径的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：NormalizeOriginalNameResult。清理展示文件名而不用于路径的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 去路径组件/控制字符/CRLF/双向控制符。
2. 保Unicode合法basename，空值拒绝。
3. 所有OSpath仍仅服务生成，下载RFC5987编码。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## media.HashUpload

文件：`modules/media/internal/upload_validator.go`。

`func HashUpload(ctx RequestContext, input HashUploadInput) (HashUploadResult, error)`

流计算对象真相摘要。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：HashUploadInput。流计算对象真相摘要的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：HashUploadResult。流计算对象真相摘要的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；CANCELLED: 请求在持久提交前被取消；已提交则返回原提交回执；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 打开服务生成tmp readonly。
2. 固定64KiBbuffer流hash并核长度。
3. 取消/期限中止不提交任何ready元数据。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；CANCELLED：请求在持久提交前被取消；已提交则返回原提交回执；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：私有upload完成。

**并发**：只在upload完成锁下读。

**幂等**：相同bytes同SHA。

**依赖**：无跨函数/跨域依赖。

## media.PutUploadChunk

文件：`modules/media/internal/chunk_writer.go`。

`func PutUploadChunk(ctx RequestContext, input PutUploadChunkInput) (PutUploadChunkResult, error)`

顺序原子写有界上传块。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PutUploadChunkInput。顺序原子写有界上传块的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PutUploadChunkResult。顺序原子写有界上传块的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 锁upload核state/expiry/expectedOffset+revision，单chunk≤8MiB。
2. 有界流写临时chunk核SHA，重复chunkoperationId同digest返回原offset。
3. fsync chunk后原子append/offset日志，不同offset/version冲突无部分计数，崩溃按chunkjournal补齐或截尾。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：media.upload本人。

**并发**：uploadId独占，offset更新同journal事务。

**幂等**：chunkoperationId+offset+SHA去重。

**依赖**：无跨函数/跨域依赖。

## media.CompleteUpload

文件：`modules/media/internal/upload_completion.go`。

`func CompleteUpload(ctx RequestContext, input CompleteUploadInput) (CompleteUploadResult, error)`

完成hash/MIME验证后发布ready资源。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CompleteUploadInput。完成hash/MIME验证后发布ready资源的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CompleteUploadResult。完成hash/MIME验证后发布ready资源的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. receivedBytes==expected，流算sha对照声明，不信任Content-Type。
2. SniffAssetType及ValidateAssetPayload检查类型/视频格式/SVG/scene，失败blocked不ready。
3. CommitAssetBlob fsync/rename内容寻址，再事务ready metadata/ref/配额receipt；文件失败不提交ready。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：media.upload本人。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：media.HashUpload；media.SniffAssetType；media.ValidateAssetPayload；media.CommitAssetBlob。

## media.ExpireUploadSessions

文件：`modules/media/internal/upload_maintenance.go`。

`func ExpireUploadSessions(ctx RequestContext, input ExpireUploadSessionsInput) (ExpireUploadSessionsResult, error)`

仅回收过期未完成临时字节。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ExpireUploadSessionsInput。仅回收过期未完成临时字节的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ExpireUploadSessionsResult。仅回收过期未完成临时字节的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 只扫描receiving expiry，明确不扫描ready assets。
2. 锁upload标expired/释放reservation。
3. 删除服务生成tmp路径并journal；complete已ready不删除。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：receiving临时文件删除，ready不自动删除。

**授权**：service:media maintenance。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：expired去重。

**依赖**：无跨函数/跨域依赖。

## media.SniffAssetType

文件：`modules/media/internal/mime_validator.go`。

`func SniffAssetType(ctx RequestContext, input SniffAssetTypeInput) (SniffAssetTypeResult, error)`

从字节识别类型。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SniffAssetTypeInput。从字节识别类型的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SniffAssetTypeResult。从字节识别类型的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力。

**步骤**

1. 签名字节和扩展不一致拒绝或强制attachment安全下载。
2. PNG/JPEG/WebP/GIF/AVIF尺寸帧元数据有界解析，不做图像重编码。
3. 上传视频只验证MP4或WebM容器/允许播放编码声明，未知回明确格式提示，无ffmpeg。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；UNSUPPORTED_CAPABILITY：当前依赖不提供声明的必需能力；retryable=false。

**副作用**：无。

**授权**：私有payload验证。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：纯读取。

**依赖**：无跨函数/跨域依赖。

## media.ValidateAssetPayload

文件：`modules/media/internal/payload_validator.go`。

`func ValidateAssetPayload(ctx RequestContext, input ValidateAssetPayloadInput) (ValidateAssetPayloadResult, error)`

验证SVG/场景/附件/视频安全交付。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidateAssetPayloadInput。验证SVG/场景/附件/视频安全交付的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateAssetPayloadResult。验证SVG/场景/附件/视频安全交付的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. SVG XML有界解析，拒绝外部entity/网络资源；上传SVG和ExcalidrawSVG均CSP sandbox且不内联执行。
2. scene按DrawingScene schema验证；附件可保存但Content-Disposition attachment+nosniff。
3. 视频只格式核对，浏览器封面/派生图由另上传提供，服务不转码/处理图片。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：私有payload验证。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：media.ValidateDrawingScene。

## media.CommitAssetBlob

文件：`modules/media/internal/blob_repository.go`。

`func CommitAssetBlob(ctx RequestContext, input CommitAssetBlobInput) (CommitAssetBlobResult, error)`

将已验证文件提交不可变内容寻址存储。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CommitAssetBlobInput。将已验证文件提交不可变内容寻址存储的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CommitAssetBlobResult。将已验证文件提交不可变内容寻址存储的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 服务生成site/blobs/sha256路径；同hash存在先核bytes/hash，不跨site泄漏。
2. tmpfsync→同FSrename→dirfsync，DBjournal记录COMMITTED object版本。
3. 事务assetmetadataready/预留转实际quota/receipt/outbox；失败孤儿隔离不会ready。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：不可变blob/提交journal。

**授权**：私有completeuploadtxn。

**并发**：(site,sha)mutex+唯一blobkey。

**幂等**：同SHA/bytes复用，异bytes完整性拒绝。

**依赖**：无跨函数/跨域依赖。

## media.ReadAsset

文件：`modules/media/internal/asset_service.go`。

`func ReadAsset(ctx RequestContext, input ReadAssetInput) (ReadAssetResult, error)`

读取资源/variants安全元数据。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadAssetInput。读取资源/variants安全元数据的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadAssetResult。读取资源/variants安全元数据的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. media.read/upload/manage或注册content/site/public/localization/knowledge服务。
2. 精确version/hash/state读取，不返回path。
3. deleted/notready明确状态或NOT_FOUND，不能将未知视ready。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：media.read或注册引用/检索service。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## media.ListAssets

文件：`modules/media/internal/asset_service.go`。

`func ListAssets(ctx RequestContext, input ListAssetsInput) (ListAssetsResult, error)`

资源库列图片/附件/视频/画板及引用摘要。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ListAssetsInput。资源库列图片/附件/视频/画板及引用摘要的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ListAssetsResult。资源库列图片/附件/视频/画板及引用摘要的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. media.manage功能权限。
2. 一致水位按kind/createdAt/id分页。
3. 引用详情单独ReadAssetReferences按docId显示，不自动删。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：media.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## media.AttachImageVariants

文件：`modules/media/internal/image_variants.go`。

`func AttachImageVariants(ctx RequestContext, input AttachImageVariantsInput) (AttachImageVariantsResult, error)`

关联浏览器多宽WebP原图。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：AttachImageVariantsInput。关联浏览器多宽WebP原图的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AttachImageVariantsResult。关联浏览器多宽WebP原图的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配。

**步骤**

1. 核所有ready同site、SHA一致、variant为静态WebP。
2. 宽度最多640/1280/1920且不超过原尺寸，动态GIF/WebP/AVIF拒绝variants保持原图。
3. 事务关联exact refs，用于srcset；服务不生成/重编码。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：media.upload。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## media.ValidateDrawingScene

文件：`modules/media/internal/drawing_validator.go`。

`func ValidateDrawingScene(ctx RequestContext, input ValidateDrawingSceneInput) (ValidateDrawingSceneResult, error)`

校验Excalidraw场景和提取文字。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidateDrawingSceneInput。校验Excalidraw场景和提取文字的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateDrawingSceneResult。校验Excalidraw场景和提取文字的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 完整schema/元素ID唯一/引用group/frame/bound/file存在/坐标有界。
2. files仅受限rasterdataURL，禁svg脚本/外部URL；无可执行HTML。
3. 按element顺序收集非deletedtext给knowledge，场景+亮暗SVG各按SHA上传。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：私有asset验证。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：纯验证。

**依赖**：无跨函数/跨域依赖。

## media.AcquireResourceReferences

文件：`modules/media/internal/reference_service.go`。

`func AcquireResourceReferences(ctx RequestContext, input AcquireResourceReferencesInput) (AcquireResourceReferencesResult, error)`

正文/config提交前保护资源迟到引用。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：AcquireResourceReferencesInput。正文/config提交前保护资源迟到引用的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AcquireResourceReferencesResult。正文/config提交前保护资源迟到引用的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 只注册source服务，核site/assetversion/SHA ready；deleted拒绝。
2. 对象mutex下取消delete_pending计划或增加deletionFence，创建≥24h pendingpin。
3. 返回pin，持久commit后confirm；未知操作pin保留到expiry不能立即当未引用。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：service:content/site-config。

**并发**：与最终delete同assetmutex/fence。

**幂等**：sourceOperationId+refdigest复用pin。

**依赖**：无跨函数/跨域依赖。

## media.ConfirmResourceReferences

文件：`modules/media/internal/reference_service.go`。

`func ConfirmResourceReferences(ctx RequestContext, input ConfirmResourceReferencesInput) (ConfirmResourceReferencesResult, error)`

源提交后确认pin为引用。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ConfirmResourceReferencesInput。源提交后确认pin为引用的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ConfirmResourceReferencesResult。源提交后确认pin为引用的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 核caller/sourceOperation/ref归属。
2. sourceVersion必须≥当前水位，低版本STALE_RESULT。
3. 事务pendingpin转confirmedrefs并增referenceGeneration，取消deletepending。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；STALE_RESULT：事件/计算针对已过期源版本，拒绝覆盖新状态；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：service对应source。

**并发**：source key CAS+assetfence。

**幂等**：同pin同revision复用，异源CONFLICT。

**依赖**：无跨函数/跨域依赖。

## media.ReleaseResourceReferences

文件：`modules/media/internal/reference_service.go`。

`func ReleaseResourceReferences(ctx RequestContext, input ReleaseResourceReferencesInput) (ReleaseResourceReferencesResult, error)`

仅明确源未提交才释放pin。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReleaseResourceReferencesInput。仅明确源未提交才释放pin的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReleaseResourceReferencesResult。仅明确源未提交才释放pin的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. caller/source核对，confirmed ref不能release冒充abort。
2. 查询sourceoperation COMMITTED未知时保留，不依网络失败释放。
3. 明确未提交事务删除pin并增generation。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：service对应source。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：released重复成功。

**依赖**：无跨函数/跨域依赖。

## media.ApplyReferenceEvent

文件：`modules/media/internal/reference_repository.go`。

`func ApplyReferenceEvent(ctx RequestContext, input ApplyReferenceEventInput) (ApplyReferenceEventResult, error)`

消费引用事件防重复/乱序。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ApplyReferenceEventInput。消费引用事件防重复/乱序的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ApplyReferenceEventResult。消费引用事件防重复/乱序的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态。

**步骤**

1. 验证producer与payloadschema/epoch。
2. eventId去重，sourceVersion低于当前只ack不覆写。
3. 事务应用完整snapshot与history分类，增generation并取消冲突清理计划。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；STALE_RESULT：事件/计算针对已过期源版本，拒绝覆盖新状态；retryable=false。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：service:jobs可信事件投递。

**并发**：sourceaggregate+asset锁稳定排序。

**幂等**：eventId唯一+sourceversion单调。

**依赖**：无跨函数/跨域依赖。

## media.ReadAssetReferences

文件：`modules/media/internal/reference_reader.go`。

`func ReadAssetReferences(ctx RequestContext, input ReadAssetReferencesInput) (ReadAssetReferencesResult, error)`

列哪些文档当前/历史引用资源。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadAssetReferencesInput。列哪些文档当前/历史引用资源的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadAssetReferencesResult。列哪些文档当前/历史引用资源的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. media.manage或本域cleanup调用。
2. 从content与site-config两个资源权威拥有源获取完整当前/历史快照和watermark；localization/public-site只派生同一不可变引用，不独立增减ref。
3. 未知/未完整返回DEPENDENCY_UNAVAILABLE，不能显示未引用；history单列风险。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：media.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content:listResourceReferences；site-config:listResourceReferences。

## media.ListUnusedAssets

文件：`modules/media/internal/cleanup_service.go`。

`func ListUnusedAssets(ctx RequestContext, input ListUnusedAssetsInput) (ListUnusedAssetsResult, error)`

后台单列当前未引用文件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ListUnusedAssetsInput。后台单列当前未引用文件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ListUnusedAssetsResult。后台单列当前未引用文件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. media.manage且ValidateCleanupSources完成全部source快照。
2. 当前confirmedref和pendingpin必须0；历史ref只产生历史缺图风险提示。
3. 只列表无删除副作用，不设周期自动清ready任务。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：media.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：media.ValidateCleanupSources。

## media.ProposeManualCleanup

文件：`modules/media/internal/cleanup_service.go`。

`func ProposeManualCleanup(ctx RequestContext, input ProposeManualCleanupInput) (ProposeManualCleanupResult, error)`

人工清理前生成精确风险计划。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ProposeManualCleanupInput。人工清理前生成精确风险计划的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ProposeManualCleanupResult。人工清理前生成精确风险计划的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核media.manage，读取每asset当前完整引用且无pending。
2. 列出历史涉及文档，明确清理可能让历史/回滚缺图片文件。
3. 写proposed plan+hash+source watermarks，不删bytes、不自动确认。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：人工清理计划metadata。

**授权**：media.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：operationId复用精确plan。

**依赖**：media.ValidateCleanupSources。

## media.ConfirmManualCleanup

文件：`modules/media/internal/cleanup_service.go`。

`func ConfirmManualCleanup(ctx RequestContext, input ConfirmManualCleanupInput) (ConfirmManualCleanupResult, error)`

管理员确认后进入迟到引用保护窗口。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ConfirmManualCleanupInput。管理员确认后进入迟到引用保护窗口的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ConfirmManualCleanupResult。管理员确认后进入迟到引用保护窗口的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 确认摘要/精确集合，历史非空必须ack。
2. 再次核current refs/pins/sourcewatermarks，一有新引用计划blocked。
3. 事务confirmed/waiting与notBefore≥24h，只有此人工批准计划可创建jobs延迟执行。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：media.manage。

**并发**：planCAS+assetdeletionfence。

**幂等**：同confirmationhash仅一次授权。

**依赖**：media.ValidateCleanupSources；jobs:createJob。

## media.CancelManualCleanup

文件：`modules/media/internal/cleanup_service.go`。

`func CancelManualCleanup(ctx RequestContext, input CancelManualCleanupInput) (CancelManualCleanupResult, error)`

删除前取消人工清理。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CancelManualCleanupInput。删除前取消人工清理的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CancelManualCleanupResult。删除前取消人工清理的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用。

**步骤**

1. planstate尚非deleted。
2. 核media.manage与计划scope。
3. 改cancelled并解除标记，已deleting完成则返回CONFLICT。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：media.manage。

**并发**：与executedelete同fence。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## media.ValidateCleanupSources

文件：`modules/media/internal/cleanup_sources.go`。

`func ValidateCleanupSources(ctx RequestContext, input ValidateCleanupSourcesInput) (ValidateCleanupSourcesResult, error)`

确认所有权威引用源后才认定可清理。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidateCleanupSourcesInput。确认所有权威引用源后才认定可清理的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateCleanupSourcesResult。确认所有权威引用源后才认定可清理的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变。

**步骤**

1. 逐source调用完整snapshot，对齐epoch与watermark；不读取对方目录。
2. current或pending任一>0返回safe=false，history仅警告。
3. 任一unknown/retryfailure/分页未完成都false且DEPENDENCY_UNAVAILABLE，不降级忽略。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false。

**副作用**：无。

**授权**：media私有人工清理流程。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：media.ReadAssetReferences。

## media.ExecuteManualCleanup

文件：`modules/media/internal/cleanup_worker.go`。

`func ExecuteManualCleanup(ctx RequestContext, input ExecuteManualCleanupInput) (ExecuteManualCleanupResult, error)`

只执行已人工确认的安全清理计划。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ExecuteManualCleanupInput。只执行已人工确认的安全清理计划的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ExecuteManualCleanupResult。只执行已人工确认的安全清理计划的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. jobs worker claim确认plan/notBefore/approvalhash，未知或未确认拒绝。
2. ValidateCleanupSources全量权威检查；assetmutex再CAS generation/fence，pendingpin/newref/activeReader出现取消该asset。
3. 先持久delete决定再unlink本服务generated blob；dedup其他asset/ref非0不删共享bytes；journal恢复补删，事件deleted。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：经人工确认不可逆ready文件删除。

**授权**：service:media worker；必须人工confirmed plan，不是自动清理。

**并发**：对象mutex/generation/fence/readers0最后检查。

**幂等**：planId/assetversion决定日志去重。

**依赖**：media.ValidateCleanupSources；media.PublishAssetEvent。

## media.ParseByteRange

文件：`modules/media/internal/range_service.go`。

`func ParseByteRange(input ParseByteRangeInput) (ParseByteRangeResult, error)`

规范Range支持iOS视频。

**输入**

- input：ParseByteRangeInput。规范Range支持iOS视频的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ParseByteRangeResult。规范Range支持iOS视频的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 无range或If-Range不匹配返回完整200。
2. 仅单bytes范围支持N-M/N-/-N，归一到总长度，末端截断。
3. 多范围/语法错误/不可满足返回VALIDATION映射416和Content-Range bytes */total。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## media.ReadAssetBytes

文件：`modules/media/internal/asset_delivery.go`。

`func ReadAssetBytes(ctx RequestContext, input ReadAssetBytesInput) (ReadAssetBytesResult, error)`

鉴权读取immutable bytes和Range。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadAssetBytesInput。鉴权读取immutable bytes和Range的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadAssetBytesResult。鉴权读取immutable bytes和Range的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 公开请求须带manifest生成的documentId/publishedRevisionId，content当前发布门禁验证visible/matchesCurrent且资产属于该精确body/config引用；草稿仅绑定已登录管理员previewproof，未知关闭。
2. AcquireReaderLease与ParseByteRange，读取immutable文件对应offset，保持Content-Length/Range/Accept-Ranges/ETag。
3. image/publichash immutable永久缓存；SVG CSP sandbox/nosniff，附件安全Disposition；断开关流释放reader。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：reader lease/stream。

**授权**：anonymous仅绑定仍可见精确发布引用与资源成员关系；管理员preview须已登录主体proof。

**并发**：readerlease与final delete共享fence。

**幂等**：同version+range确定bytes。

**依赖**：media.ParseByteRange；media.AcquireReaderLease；content:validatePublications。

## media.HeadAssetBytes

文件：`modules/media/internal/asset_delivery.go`。

`func HeadAssetBytes(ctx RequestContext, input HeadAssetBytesInput) (HeadAssetBytesResult, error)`

HEAD同GET授权只返header。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：HeadAssetBytesInput。HEAD同GET授权只返header的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：HeadAssetBytesResult。HEAD同GET授权只返header的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见。

**步骤**

1. 执行与GET完全相同scope/state/version校验。
2. 不打开全bytes stream。
3. 返回ETag/长度/Accept-Ranges/CSP与cachepolicy。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false。

**副作用**：无。

**授权**：与ReadAssetBytes相同。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## media.AcquireReaderLease

文件：`modules/media/internal/reader_lease.go`。

`func AcquireReaderLease(ctx RequestContext, input AcquireReaderLeaseInput) (AcquireReaderLeaseResult, error)`

防正在下载资源被最终删除。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：AcquireReaderLeaseInput。防正在下载资源被最终删除的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AcquireReaderLeaseResult。防正在下载资源被最终删除的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；NOT_FOUND: 资源不存在或在当前主体作用域中不可见。

**步骤**

1. assetmutex核ready/version/hash且无committeddelete决定。
2. 记录短期readerlease并增加fenceguard。
3. 流close释放；deadline过期前heartbeat保持，过期断流，不无限阻塞cleanup。

**错误**：LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false。

**副作用**：私有readerlease。

**授权**：media私有download流程。

**并发**：对象mutex。

**幂等**：请求readerId单次。

**依赖**：无跨函数/跨域依赖。

## media.BuildDeliveryManifest

文件：`modules/media/internal/manifest_service.go`。

`func BuildDeliveryManifest(ctx RequestContext, input BuildDeliveryManifestInput) (BuildDeliveryManifestResult, error)`

给公开/管理员预览返回SHA URLs/srcset/封面。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：BuildDeliveryManifestInput。给公开/管理员预览返回SHA URLs/srcset/封面的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：BuildDeliveryManifestResult。给公开/管理员预览返回SHA URLs/srcset/封面的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 必须公开ref或登录绑定preview二者恰一，content validatePublications/validatePreviewGrant核权威。
2. 每assetversion/hash必须ready且属于该精确body/config资源集合，不准任意借preview读文件。
3. 公开生成带精确documentId/publishedRevisionId的稳定SHA资源URL，读取仍查当前发布门禁，字节immutable长期缓存；preview独立proofURL/no-store；imagevariants/srcset按exactref。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；STALE_RESULT：事件/计算针对已过期源版本，拒绝覆盖新状态；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：service:public-site；actor/refs必须权威匹配。

**并发**：publication验证后传输前再查visible。

**幂等**：相同hash公开URL稳定。

**依赖**：content:validatePublications；content:validatePreviewGrant；media.ReadAsset。

## media.ReadVideoMetadata

文件：`modules/media/internal/video_service.go`。

`func ReadVideoMetadata(ctx RequestContext, input ReadVideoMetadataInput) (ReadVideoMetadataResult, error)`

插入时一次获取平台公开信息并本地封面。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadVideoMetadataInput。插入时一次获取平台公开信息并本地封面的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadVideoMetadataResult。插入时一次获取平台公开信息并本地封面的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力。

**步骤**

1. RecognizePlatformURL识别四平台，不携账号cookie/auth，FetchProviderPublicMetadata bounded egress。
2. 成功公开标题简介作者时长+ImportRemoteCover本地asset，BuildProviderEmbed核白名单。
3. 失败返回manual_required并保留手填字段，不能假成功；源snapshot随块存，之后不自动重抓。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；UNSUPPORTED_CAPABILITY：当前依赖不提供声明的必需能力；retryable=false。

**副作用**：一次公开metadata网络抓取+本地封面上传；本轮不真实调用。

**授权**：media.upload编辑文档功能。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：插入operationId+canonicalURL复用snapshot，之后不自动刷新。

**依赖**：media.RecognizePlatformURL；media.FetchProviderPublicMetadata；media.ImportRemoteCover；media.BuildProviderEmbed。

## media.RecognizePlatformURL

文件：`modules/media/internal/provider_registry.go`。

`func RecognizePlatformURL(ctx RequestContext, input RecognizePlatformURLInput) (RecognizePlatformURLResult, error)`

识别Bilibili/YouTube/抖音/TikTok链接。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RecognizePlatformURLInput。识别Bilibili/YouTube/抖音/TikTok链接的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RecognizePlatformURLResult。识别Bilibili/YouTube/抖音/TikTok链接的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力。

**步骤**

1. parse HTTPS URL禁userinfo/控制字符/本地地址。
2. 精确匹配固定host与视频path/query规则，只提providerVideoId。
3. 短链接经ResolveProviderRedirect有界白名单解析；未知平台UNSUPPORTED_CAPABILITY。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_CAPABILITY：当前依赖不提供声明的必需能力；retryable=false。

**副作用**：无。

**授权**：私有插入validator。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：确定性。

**依赖**：media.ResolveProviderRedirect。

## media.ResolveProviderRedirect

文件：`modules/media/internal/provider_url_guard.go`。

`func ResolveProviderRedirect(ctx RequestContext, input ResolveProviderRedirectInput) (ResolveProviderRedirectResult, error)`

仅平台短链接受控重定向解析。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ResolveProviderRedirectInput。仅平台短链接受控重定向解析的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ResolveProviderRedirectResult。仅平台短链接受控重定向解析的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 每hop校验scheme/host/DNS结果非loopback/private/linklocal；端口443。
2. 最多3hop/5秒/64KiB，拒绝跨provider或开放重定向目标。
3. 不发送cookie/auth，不记录原query敏感值；解析失败manualfallback。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：短URL有限HTTP读取。

**授权**：私有平台适配。

**并发**：每provider有限并发。

**幂等**：operation内一次解析。

**依赖**：无跨函数/跨域依赖。

## media.FetchProviderPublicMetadata

文件：`modules/media/internal/provider_fetch.go`。

`func FetchProviderPublicMetadata(ctx RequestContext, input FetchProviderPublicMetadataInput) (FetchProviderPublicMetadataResult, error)`

provider接口抓公开非登录信息。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：FetchProviderPublicMetadataInput。provider接口抓公开非登录信息的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：FetchProviderPublicMetadataResult。provider接口抓公开非登录信息的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. YouTube官方oEmbed，Bili公开view，TikTok官方oEmbed，抖音公开页面元信息；适配实现前静态profile核实capability。
2. endpoint由固定provider构造，所有DNS/redirect按egressguard，超时5秒/响应≤1MiB/无cookieauth。
3. 只保title/description/author/duration/cover候选；错误显式manual_required，不周期刷新。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；UNSUPPORTED_CAPABILITY：当前依赖不提供声明的必需能力；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：一次有限公开HTTP读取。

**授权**：私有provider adapter。

**并发**：provider信号量限制。

**幂等**：插入operationID去重。

**依赖**：无跨函数/跨域依赖。

## media.BuildProviderEmbed

文件：`modules/media/internal/provider_embed.go`。

`func BuildProviderEmbed(input BuildProviderEmbedInput) (BuildProviderEmbedResult, error)`

生成四平台白名单embed资料。

**输入**

- input：BuildProviderEmbedInput。生成四平台白名单embed资料的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：BuildProviderEmbedResult。生成四平台白名单embed资料的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力。

**步骤**

1. YouTube host youtube-nocookie、Bili player.bilibili、TikTok官方播放器、抖音开放平台profile。
2. 视频ID仅URL编码后放固定template参数，不接受用户embedURL。
3. 未核实profile输出ready=false/manual_required；public-site点击前只封面，不加载第三方script/tracker。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_CAPABILITY：当前依赖不提供声明的必需能力；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## media.ImportRemoteCover

文件：`modules/media/internal/cover_import.go`。

`func ImportRemoteCover(ctx RequestContext, input ImportRemoteCoverInput) (ImportRemoteCoverResult, error)`

下载平台封面到本地内容寻址资源。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ImportRemoteCoverInput。下载平台封面到本地内容寻址资源的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ImportRemoteCoverResult。下载平台封面到本地内容寻址资源的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 只provider profile允许coverhosts，每DNS/hop禁SSRF，最长5s/10MiB。
2. 流hash/MIME像素元数据校验，禁止HTML/SVG主动内容，服务不重编码。
3. 通过本域blob完成流程生成ready平台封面；失败返回null和manual_required，不外链防盗链图。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：公开封面下载/私有blob。

**授权**：私有metadata插入流程。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：同SHA复用。

**依赖**：media.CommitAssetBlob。

## media.RecoverBlobJournal

文件：`modules/media/internal/blob_recovery.go`。

`func RecoverBlobJournal(ctx RequestContext, input RecoverBlobJournalInput) (RecoverBlobJournalResult, error)`

恢复blob完成/删除持久日志。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RecoverBlobJournalInput。恢复blob完成/删除持久日志的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RecoverBlobJournalResult。恢复blob完成/删除持久日志的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 启动exclusive检查COMMITTEDblob哈希/metadata。
2. PREPARED未visible孤儿隔离；DELETING仅已人工批准决定可补删。
3. 未知日志关闭对应asset，绝不扫描ready文件自动清。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：本域日志与孤儿隔离。

**授权**：service:media启动控制。

**并发**：exclusive gate。

**幂等**：重复不产生新delete授权。

**依赖**：无跨函数/跨域依赖。

## media.PublishAssetEvent

文件：`modules/media/internal/outbox_repository.go`。

`func PublishAssetEvent(ctx RequestContext, input PublishAssetEventInput) (PublishAssetEventResult, error)`

原子写最小资源状态outbox。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PublishAssetEventInput。原子写最小资源状态outbox的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PublishAssetEventResult。原子写最小资源状态outbox的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 确认payload无filepath/secret/usertext。
2. 同ready/delete事务eventId唯一写outbox。
3. jobs publish ack后完成，消费方sourcewatermark单调。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/文件/receipt/outbox。

**授权**：私有assettxn。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：eventId去重。

**依赖**：jobs:publishEvent。

## media.AcquireBarrier

文件：`modules/media/internal/barrier_service.go`。

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

**授权**：service:data-protection，备份超管委托；恢复专用RecoveryAuthorization严格restore/barrier/epoch/audience/action，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## media.ReleaseBarrier

文件：`modules/media/internal/barrier_service.go`。

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

**授权**：service:data-protection，备份超管委托；恢复专用RecoveryAuthorization严格restore/barrier/epoch/audience/action，本地control信任锚不被快照覆盖。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config:readRuntimeState。

## media.ExportSnapshot

文件：`modules/media/internal/snapshot_service.go`。

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

**授权**：service:data-protection，备份超管委托；恢复专用RecoveryAuthorization严格restore/barrier/epoch/audience/action，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## media.PrepareSnapshot

文件：`modules/media/internal/snapshot_service.go`。

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

**授权**：service:data-protection，备份超管委托；恢复专用RecoveryAuthorization严格restore/barrier/epoch/audience/action，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## media.ValidateSnapshot

文件：`modules/media/internal/snapshot_validator.go`。

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

**授权**：service:data-protection，备份超管委托；恢复专用RecoveryAuthorization严格restore/barrier/epoch/audience/action，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## media.CommitSnapshot

文件：`modules/media/internal/snapshot_repository.go`。

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

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection，备份超管委托；恢复专用RecoveryAuthorization严格restore/barrier/epoch/audience/action，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## media.RollbackSnapshot

文件：`modules/media/internal/snapshot_repository.go`。

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

**授权**：service:data-protection，备份超管委托；恢复专用RecoveryAuthorization严格restore/barrier/epoch/audience/action，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M05-F901 main

文件 `modules/media/cmd/media/main.go`。独立进程可执行入口

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

### M05-F902 BuildApplication

文件 `modules/media/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M05-F903 Serve

文件 `modules/media/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M05-F904 Shutdown

文件 `modules/media/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M05-F905 Health

文件 `modules/media/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M05-F906 Ready

文件 `modules/media/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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
