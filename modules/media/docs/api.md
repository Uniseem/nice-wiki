# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| POST `/v1/barriers` | `acquireBarrier` | `media.AcquireBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得可恢复的数据屏障 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `media.ExportSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出屏障水位的一致快照 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| DELETE `/v1/barriers/{barrierId}` | `releaseBarrier` | `media.ReleaseBarrier` | `service` | `releaseBarrierRequest` → 200 `ReleaseBarrierResult` | 释放屏障 | `FORBIDDEN`、`VERSION_MISMATCH` |
| POST `/v1/uploads` | `beginUpload` | `media.BeginUpload` | `media.upload` | `BeginUploadInput` → 200 `BeginUploadResult` | 开始内容寻址可续传上传 | `VALIDATION`、`FORBIDDEN`、`NOT_FOUND`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/uploads/{uploadId}` | `readUpload` | `media.ReadUpload` | `media.upload` | `—` → 200 `ReadUploadResult` | 查询续传offset | `FORBIDDEN`、`NOT_FOUND` |
| DELETE `/v1/uploads/{uploadId}` | `abortUpload` | `media.AbortUpload` | `media.upload` | `abortUploadRequest` → 200 `AbortUploadResult` | 取消未完成上传释放配额 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT` |
| PUT `/v1/uploads/{uploadId}/chunks` | `putUploadChunk` | `media.PutUploadChunk` | `media.upload` | `application/octet-stream` → 200 `PutUploadChunkResult` | 顺序原子写有界上传块 | `VALIDATION`、`CONFLICT`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/uploads/{uploadId}:complete` | `completeUpload` | `media.CompleteUpload` | `media.upload` | `completeUploadRequest` → 200 `CompleteUploadResult` | 完成hash/MIME验证后发布ready资源 | `VALIDATION`、`FORBIDDEN`、`NOT_FOUND`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/assets` | `listAssets` | `media.ListAssets` | `media.manage` | `—` → 200 `ListAssetsResult` | 资源库列图片/附件/视频/画板及引用摘要 | `VALIDATION`、`FORBIDDEN` |
| GET `/v1/assets/{assetId}` | `readAsset` | `media.ReadAsset` | `media.read` | `—` → 200 `ReadAssetResult` | 读取资源/variants安全元数据 | `FORBIDDEN`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/assets/{assetId}/variants` | `attachImageVariants` | `media.AttachImageVariants` | `media.upload` | `attachImageVariantsRequest` → 200 `AttachImageVariantsResult` | 关联浏览器多宽WebP原图 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`INTEGRITY_ERROR` |
| POST `/v1/references:acquire` | `acquireResourceReferences` | `media.AcquireResourceReferences` | `service` | `AcquireResourceReferencesInput` → 200 `AcquireResourceReferencesResult` | 正文/config提交前保护资源迟到引用 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`LOCKED`、`INTEGRITY_ERROR` |
| POST `/v1/references:confirm` | `confirmResourceReferences` | `media.ConfirmResourceReferences` | `service` | `ConfirmResourceReferencesInput` → 200 `ConfirmResourceReferencesResult` | 源提交后确认pin为引用 | `FORBIDDEN`、`STALE_RESULT`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/references:release` | `releaseResourceReferences` | `media.ReleaseResourceReferences` | `service` | `ReleaseResourceReferencesInput` → 200 `ReleaseResourceReferencesResult` | 仅明确源未提交才释放pin | `FORBIDDEN`、`CONFLICT`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/assets/{assetId}/references` | `readAssetReferences` | `media.ReadAssetReferences` | `media.manage` | `—` → 200 `ReadAssetReferencesResult` | 列哪些文档当前/历史引用资源 | `FORBIDDEN`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/assets/unused` | `listUnusedAssets` | `media.ListUnusedAssets` | `media.manage` | `—` → 200 `ListUnusedAssetsResult` | 后台单列当前未引用文件 | `FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/cleanup-plans` | `proposeManualCleanup` | `media.ProposeManualCleanup` | `media.manage` | `ProposeManualCleanupInput` → 200 `ProposeManualCleanupResult` | 人工清理前生成精确风险计划 | `VALIDATION`、`FORBIDDEN`、`NOT_FOUND`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/cleanup-plans/{planId}:confirm` | `confirmManualCleanup` | `media.ConfirmManualCleanup` | `media.manage` | `confirmManualCleanupRequest` → 200 `ConfirmManualCleanupResult` | 管理员确认后进入迟到引用保护窗口 | `FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| DELETE `/v1/cleanup-plans/{planId}` | `cancelManualCleanup` | `media.CancelManualCleanup` | `media.manage` | `cancelManualCleanupRequest` → 200 `CancelManualCleanupResult` | 删除前取消人工清理 | `FORBIDDEN`、`CONFLICT` |
| GET `/v1/assets/{assetId}/bytes` | `readAssetBytes` | `media.ReadAssetBytes` | `anonymous` | `—` → 200 `application/octet-stream` | 鉴权读取immutable bytes和Range | `VALIDATION`、`FORBIDDEN`、`NOT_FOUND`、`HTTP 416`、`INTEGRITY_ERROR` |
| HEAD `/v1/assets/{assetId}/bytes` | `headAssetBytes` | `media.HeadAssetBytes` | `anonymous` | `—` → 200 `HeadAssetBytesResult` | HEAD同GET授权只返header | `FORBIDDEN`、`NOT_FOUND` |
| POST `/v1/delivery-manifests` | `buildDeliveryManifest` | `media.BuildDeliveryManifest` | `service` | `BuildDeliveryManifestInput` → 200 `BuildDeliveryManifestResult` | 给公开/管理员预览返回SHA URLs/srcset/封面 | `FORBIDDEN`、`NOT_FOUND`、`STALE_RESULT`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/videos/metadata` | `readVideoMetadata` | `media.ReadVideoMetadata` | `media.upload` | `ReadVideoMetadataInput` → 200 `ReadVideoMetadataResult` | 插入时一次获取平台公开信息并本地封面 | `VALIDATION`、`UNSUPPORTED_CAPABILITY`、`DEPENDENCY_UNAVAILABLE` |
| GET `/healthz` | `health` | `media.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `media.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `media.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `media.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
<!-- openapi:end -->

