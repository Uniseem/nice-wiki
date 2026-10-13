# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

## acquireBarrier

`POST /v1/barriers` — 取得可恢复的数据屏障。

- handler：`media.AcquireBarrier`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：BarrierRequest；成功 200：BarrierStatus。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## exportSnapshot

`POST /v1/snapshots:export` — 导出屏障水位的一致快照。

- handler：`media.ExportSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotExportRequest；成功 200：SnapshotExportResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## validateSnapshot

`POST /v1/snapshots:validate` — 验证快照可恢复性。

- handler：`media.ValidateSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotValidateRequest；成功 200：SnapshotValidation。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## prepareSnapshot

`POST /v1/snapshots:prepare` — 在私有 staging 区准备恢复。

- handler：`media.PrepareSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotPrepareRequest；成功 200：SnapshotStageResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## commitSnapshot

`POST /v1/snapshots:commit` — 激活已准备恢复数据。

- handler：`media.CommitSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## rollbackSnapshot

`POST /v1/snapshots:rollback` — 按协调决定回滚恢复阶段。

- handler：`media.RollbackSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## releaseBarrier

`DELETE /v1/barriers/{barrierId}` — 释放屏障。

- handler：`media.ReleaseBarrier`；权限：`superadmin`。
- 路径参数：barrierId；查询参数：无。
- 请求：ReleaseBarrierInput；成功 200：ReleaseBarrierResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）。

## beginUpload

`POST /v1/uploads` — 开始内容寻址可续传上传。

- handler：`media.BeginUpload`；权限：`media.upload`。
- 路径参数：无；查询参数：无。
- 请求：BeginUploadInput；成功 200：BeginUploadResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readUpload

`GET /v1/uploads/{uploadId}` — 查询续传offset。

- handler：`media.ReadUpload`；权限：`media.upload`。
- 路径参数：uploadId；查询参数：无。
- 请求：无正文；成功 200：ReadUploadResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）。

## putUploadChunk

`PUT /v1/uploads/{uploadId}/chunks` — 顺序原子写有界上传块。

- handler：`media.PutUploadChunk`；权限：`media.upload`。
- 路径参数：uploadId；查询参数：无。
- 请求：PutUploadChunkInput；成功 200：PutUploadChunkResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## completeUpload

`POST /v1/uploads/{uploadId}:complete` — 完成hash/MIME验证后发布ready资源。

- handler：`media.CompleteUpload`；权限：`media.upload`。
- 路径参数：uploadId；查询参数：无。
- 请求：CompleteUploadInput；成功 200：CompleteUploadResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## abortUpload

`DELETE /v1/uploads/{uploadId}` — 取消未完成上传释放配额。

- handler：`media.AbortUpload`；权限：`media.upload`。
- 路径参数：uploadId；查询参数：无。
- 请求：AbortUploadInput；成功 200：AbortUploadResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）。

## listAssets

`GET /v1/assets` — 资源库列图片/附件/视频/画板及引用摘要。

- handler：`media.ListAssets`；权限：`media.manage`。
- 路径参数：无；查询参数：limit, cursor, kind。
- 请求：无正文；成功 200：ListAssetsResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## readAsset

`GET /v1/assets/{assetId}` — 读取资源/variants安全元数据。

- handler：`media.ReadAsset`；权限：`media.read`。
- 路径参数：assetId；查询参数：version。
- 请求：无正文；成功 200：ReadAssetResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## attachImageVariants

`PUT /v1/assets/{assetId}/variants` — 关联浏览器多宽WebP原图。

- handler：`media.AttachImageVariants`；权限：`media.upload`。
- 路径参数：assetId；查询参数：无。
- 请求：AttachImageVariantsInput；成功 200：AttachImageVariantsResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）。

## acquireResourceReferences

`POST /v1/references:acquire` — 正文/config提交前保护资源迟到引用。

- handler：`media.AcquireResourceReferences`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：AcquireResourceReferencesInput；成功 200：AcquireResourceReferencesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## confirmResourceReferences

`POST /v1/references:confirm` — 源提交后确认pin为引用。

- handler：`media.ConfirmResourceReferences`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：ConfirmResourceReferencesInput；成功 200：ConfirmResourceReferencesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；STALE_RESULT（事件/计算针对已过期源版本，拒绝覆盖新状态）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## releaseResourceReferences

`POST /v1/references:release` — 仅明确源未提交才释放pin。

- handler：`media.ReleaseResourceReferences`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：ReleaseResourceReferencesInput；成功 200：ReleaseResourceReferencesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readAssetReferences

`GET /v1/assets/{assetId}/references` — 列哪些文档当前/历史引用资源。

- handler：`media.ReadAssetReferences`；权限：`media.manage`。
- 路径参数：assetId；查询参数：无。
- 请求：无正文；成功 200：ReadAssetReferencesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## listUnusedAssets

`GET /v1/assets/unused` — 后台单列当前未引用文件。

- handler：`media.ListUnusedAssets`；权限：`media.manage`。
- 路径参数：无；查询参数：limit, cursor, kind。
- 请求：无正文；成功 200：ListUnusedAssetsResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## proposeManualCleanup

`POST /v1/cleanup-plans` — 人工清理前生成精确风险计划。

- handler：`media.ProposeManualCleanup`；权限：`media.manage`。
- 路径参数：无；查询参数：无。
- 请求：ProposeManualCleanupInput；成功 200：ProposeManualCleanupResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## confirmManualCleanup

`POST /v1/cleanup-plans/{planId}:confirm` — 管理员确认后进入迟到引用保护窗口。

- handler：`media.ConfirmManualCleanup`；权限：`media.manage`。
- 路径参数：planId；查询参数：无。
- 请求：ConfirmManualCleanupInput；成功 200：ConfirmManualCleanupResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## cancelManualCleanup

`DELETE /v1/cleanup-plans/{planId}` — 删除前取消人工清理。

- handler：`media.CancelManualCleanup`；权限：`media.manage`。
- 路径参数：planId；查询参数：无。
- 请求：CancelManualCleanupInput；成功 200：CancelManualCleanupResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）。

## readAssetBytes

`GET /v1/assets/{assetId}/bytes` — 鉴权读取immutable bytes和Range。

- handler：`media.ReadAssetBytes`；权限：`anonymous`。
- 路径参数：assetId；查询参数：version, previewToken, publicationDocumentId, publishedRevisionId。
- 请求：无正文；成功 200：ReadAssetBytesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## headAssetBytes

`HEAD /v1/assets/{assetId}/bytes` — HEAD同GET授权只返header。

- handler：`media.HeadAssetBytes`；权限：`anonymous`。
- 路径参数：assetId；查询参数：version, previewToken。
- 请求：无正文；成功 200：HeadAssetBytesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）。

## buildDeliveryManifest

`POST /v1/delivery-manifests` — 给公开/管理员预览返回SHA URLs/srcset/封面。

- handler：`media.BuildDeliveryManifest`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：BuildDeliveryManifestInput；成功 200：BuildDeliveryManifestResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；STALE_RESULT（事件/计算针对已过期源版本，拒绝覆盖新状态）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readVideoMetadata

`POST /v1/videos/metadata` — 插入时一次获取平台公开信息并本地封面。

- handler：`media.ReadVideoMetadata`；权限：`media.upload`。
- 路径参数：无；查询参数：无。
- 请求：ReadVideoMetadataInput；成功 200：ReadVideoMetadataResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；UNSUPPORTED_CAPABILITY（当前依赖不提供声明的必需能力）。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
