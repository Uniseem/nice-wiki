# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| POST `/v1/barriers` | `acquireBarrier` | `content.AcquireBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得可恢复的数据屏障 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `content.ExportSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出屏障水位的一致快照 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| DELETE `/v1/barriers/{barrierId}` | `releaseBarrier` | `content.ReleaseBarrier` | `service` | `releaseBarrierRequest` → 200 `ReleaseBarrierResult` | 释放屏障 | `FORBIDDEN`、`VERSION_MISMATCH` |
| GET `/v1/documents` | `listDocuments` | `content.ListDocuments` | `content.read` | `—` → 200 `ListDocumentsResult` | 文档及回收站分页列表 | `VALIDATION`、`FORBIDDEN` |
| POST `/v1/documents` | `createDocument` | `content.CreateDocument` | `content.edit` | `CreateDocumentInput` → 200 `CreateDocumentResult` | 新建共享草稿文档 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR` |
| GET `/v1/tree` | `readDocumentTree` | `content.ReadDocumentTree` | `content.read` | `—` → 200 `ReadDocumentTreeResult` | 后台读取整站文档树 | `FORBIDDEN`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/tree:move` | `moveDocument` | `content.MoveDocument` | `content.edit` | `MoveDocumentInput` → 200 `MoveDocumentResult` | 拖拽移动层级和顺序 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/tree:reorder` | `reorderChildren` | `content.ReorderChildren` | `content.edit` | `ReorderChildrenInput` → 200 `ReorderChildrenResult` | 拖拽调整同级顺序 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/documents/{documentId}/metadata` | `updateDocumentMetadata` | `content.UpdateDocumentMetadata` | `content.edit` | `updateDocumentMetadataRequest` → 200 `UpdateDocumentMetadataResult` | 修改标题/路径/SEO描述/分享图 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/documents/{documentId}/lock` | `acquireEditLock` | `content.AcquireEditLock` | `content.edit` | `acquireEditLockRequest` → 200 `AcquireEditLockResult` | 打开文档自动获得内存编辑锁 | `FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`UNAUTHENTICATED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/documents/{documentId}/lock` | `renewEditLock` | `content.RenewEditLock` | `content.edit` | `renewEditLockRequest` → 200 `RenewEditLockResult` | 心跳维持锁并处理长时间无输入 | `VALIDATION`、`FORBIDDEN`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`UNAUTHENTICATED`、`NOT_FOUND`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| DELETE `/v1/documents/{documentId}/lock` | `releaseEditLock` | `content.ReleaseEditLock` | `content.edit` | `releaseEditLockRequest` → 200 `ReleaseEditLockResult` | 关闭页面释放自身锁 | `FORBIDDEN`、`UNAUTHENTICATED`、`NOT_FOUND`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/documents/{documentId}/lock:force-release` | `forceReleaseEditLock` | `content.ForceReleaseEditLock` | `superadmin` | `forceReleaseEditLockRequest` → 200 `ForceReleaseEditLockResult` | 只有超管强制释放别人锁 | `FORBIDDEN`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`UNAUTHENTICATED`、`NOT_FOUND`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/documents/{documentId}/draft` | `readDraft` | `content.ReadDraft` | `content.read` | `—` → 200 `ReadDraftResult` | 读取全篇共享草稿及编辑状态 | `FORBIDDEN`、`NOT_FOUND`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`VERSION_MISMATCH`、`LOCKED` |
| PUT `/v1/documents/{documentId}/draft` | `saveDraft` | `content.SaveDraft` | `content.edit` | `saveDraftRequest` → 200 `SaveDraftResult` | 自动保存完整Tiptap正文 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR` |
| POST `/v1/documents/{documentId}:publish` | `publishDocument` | `content.PublishDocument` | `content.publish` | `publishDocumentRequest` → 200 `PublishDocumentResult` | 点击发布才公开精确共享草稿版本 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/documents/{documentId}:withdraw` | `withdrawDocument` | `content.WithdrawDocument` | `content.publish` | `withdrawDocumentRequest` → 200 `WithdrawDocumentResult` | 撤回后回草稿并让公开页面下线 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/publications:validate` | `validatePublications` | `content.ValidatePublications` | `service` | `ValidatePublicationsInput` → 200 `ValidatePublicationsResult` | 同步核公开引用是否仍可见且当前 | `FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/documents/{documentId}/published` | `readPublishedRevision` | `content.ReadPublishedRevision` | `anonymous` | `—` → 200 `ReadPublishedRevisionResult` | 获取仍公开的精确正文快照 | `NOT_FOUND`、`STALE_RESULT`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/documents/{documentId}:trash` | `trashDocument` | `content.TrashDocument` | `content.publish` | `trashDocumentRequest` → 200 `TrashDocumentResult` | 删除进回收站，可选旧URL替代跳转 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/documents/{documentId}:restore` | `restoreDocument` | `content.RestoreDocument` | `content.publish` | `restoreDocumentRequest` → 200 `RestoreDocumentResult` | 从回收站恢复为草稿 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/trash:purge` | `purgeDocuments` | `content.PurgeDocuments` | `superadmin` | `PurgeDocumentsInput` → 200 `PurgeDocumentsResult` | 只有超管彻底删除/清空回收站 | `FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/documents/{documentId}/history` | `readRevisionHistory` | `content.ReadRevisionHistory` | `content.publish` | `—` → 200 `ReadRevisionHistoryResult` | 查看署名发布/git版本历史 | `FORBIDDEN`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/documents/{documentId}/history:diff` | `diffRevisions` | `content.DiffRevisions` | `content.publish` | `diffRevisionsRequest` → 200 `DiffRevisionsResult` | 按稳定块ID对比历史版本 | `FORBIDDEN`、`NOT_FOUND`、`INTEGRITY_ERROR` |
| POST `/v1/documents/{documentId}/history:rollback` | `rollbackPublishedRevision` | `content.RollbackPublishedRevision` | `content.publish` | `rollbackPublishedRevisionRequest` → 200 `RollbackPublishedRevisionResult` | 回滚历史形成新发布/git提交 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/previews` | `createPreviewGrant` | `content.CreatePreviewGrant` | `content.read` | `CreatePreviewGrantInput` → 200 `CreatePreviewGrantResult` | 管理员预览草稿公开样式 | `FORBIDDEN`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/previews:validate` | `validatePreviewGrant` | `content.ValidatePreviewGrant` | `service` | `ValidatePreviewGrantInput` → 200 `ValidatePreviewGrantResult` | 只向对应管理员验证草稿预览 | `UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/resource-references` | `listResourceReferences` | `content.ListResourceReferences` | `service` | `—` → 200 `ListResourceReferencesResult` | 向资源库列出哪些文档引用文件 | `FORBIDDEN`、`HTTP 503`、`HTTP 400`、`HTTP 401`、`HTTP 409`、`HTTP 422` |
| GET `/v1/objects/{objectId}` | `readContentObject` | `content.ReadContentObject` | `service` | `—` → 200 `ReadContentObjectResult` | 经对象API读取私有文件而不共享目录 | `FORBIDDEN`、`NOT_FOUND`、`INTEGRITY_ERROR` |
| GET `/v1/public-tree` | `readPublicTree` | `content.ReadPublicTree` | `anonymous` | `—` → 200 `ReadPublicTreeResult` | 读取公开导航 | `INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/public-path:resolve` | `resolvePublicPath` | `content.ResolvePublicPath` | `anonymous` | `—` → 200 `ResolvePublicPathResult` | 解析301/410公开路径 | `VALIDATION`、`DEPENDENCY_UNAVAILABLE` |
| GET `/healthz` | `health` | `content.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `content.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `content.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `content.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
| GET `/v1/reference-decisions/{sourceOperationId}` | `readReferenceDecision` | `content.ReadReferenceDecision` | `service` | `—` → 200 `ReadReferenceDecisionResult` | 读取资源引用源持久提交决定 | `HTTP 400`、`HTTP 401`、`HTTP 403`、`HTTP 409`、`HTTP 422`、`HTTP 503`、`HTTP 404` |
<!-- openapi:end -->

