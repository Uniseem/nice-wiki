# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

## acquireBarrier

`POST /v1/barriers` — 取得可恢复的数据屏障。

- handler：`content.AcquireBarrier`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：BarrierRequest；成功 200：BarrierStatus。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## exportSnapshot

`POST /v1/snapshots:export` — 导出屏障水位的一致快照。

- handler：`content.ExportSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotExportRequest；成功 200：SnapshotExportResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## validateSnapshot

`POST /v1/snapshots:validate` — 验证快照可恢复性。

- handler：`content.ValidateSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotValidateRequest；成功 200：SnapshotValidation。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## prepareSnapshot

`POST /v1/snapshots:prepare` — 在私有 staging 区准备恢复。

- handler：`content.PrepareSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotPrepareRequest；成功 200：SnapshotStageResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## commitSnapshot

`POST /v1/snapshots:commit` — 激活已准备恢复数据。

- handler：`content.CommitSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## rollbackSnapshot

`POST /v1/snapshots:rollback` — 按协调决定回滚恢复阶段。

- handler：`content.RollbackSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## releaseBarrier

`DELETE /v1/barriers/{barrierId}` — 释放屏障。

- handler：`content.ReleaseBarrier`；权限：`superadmin`。
- 路径参数：barrierId；查询参数：无。
- 请求：ReleaseBarrierInput；成功 200：ReleaseBarrierResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）。

## createDocument

`POST /v1/documents` — 新建共享草稿文档。

- handler：`content.CreateDocument`；权限：`content.edit`。
- 路径参数：无；查询参数：无。
- 请求：CreateDocumentInput；成功 200：CreateDocumentResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## listDocuments

`GET /v1/documents` — 文档及回收站分页列表。

- handler：`content.ListDocuments`；权限：`content.read`。
- 路径参数：无；查询参数：limit, cursor, assetId。
- 请求：无正文；成功 200：ListDocumentsResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## readDocumentTree

`GET /v1/tree` — 后台读取整站文档树。

- handler：`content.ReadDocumentTree`；权限：`content.read`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadDocumentTreeResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## moveDocument

`POST /v1/tree:move` — 拖拽移动层级和顺序。

- handler：`content.MoveDocument`；权限：`content.edit`。
- 路径参数：无；查询参数：无。
- 请求：MoveDocumentInput；成功 200：MoveDocumentResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## reorderChildren

`POST /v1/tree:reorder` — 拖拽调整同级顺序。

- handler：`content.ReorderChildren`；权限：`content.edit`。
- 路径参数：无；查询参数：无。
- 请求：ReorderChildrenInput；成功 200：ReorderChildrenResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## updateDocumentMetadata

`PUT /v1/documents/{documentId}/metadata` — 修改标题/路径/SEO描述/分享图。

- handler：`content.UpdateDocumentMetadata`；权限：`content.edit`。
- 路径参数：documentId；查询参数：无。
- 请求：UpdateDocumentMetadataInput；成功 200：UpdateDocumentMetadataResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## acquireEditLock

`POST /v1/documents/{documentId}/lock` — 打开文档自动获得内存编辑锁。

- handler：`content.AcquireEditLock`；权限：`content.edit`。
- 路径参数：documentId；查询参数：无。
- 请求：AcquireEditLockInput；成功 200：AcquireEditLockResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）。

## renewEditLock

`PUT /v1/documents/{documentId}/lock` — 心跳维持锁并处理长时间无输入。

- handler：`content.RenewEditLock`；权限：`content.edit`。
- 路径参数：documentId；查询参数：无。
- 请求：RenewEditLockInput；成功 200：RenewEditLockResult。
- 错误：LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## releaseEditLock

`DELETE /v1/documents/{documentId}/lock` — 关闭页面释放自身锁。

- handler：`content.ReleaseEditLock`；权限：`content.edit`。
- 路径参数：documentId；查询参数：无。
- 请求：ReleaseEditLockInput；成功 200：ReleaseEditLockResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）。

## forceReleaseEditLock

`POST /v1/documents/{documentId}/lock:force-release` — 只有超管强制释放别人锁。

- handler：`content.ForceReleaseEditLock`；权限：`superadmin`。
- 路径参数：documentId；查询参数：无。
- 请求：ForceReleaseEditLockInput；成功 200：ForceReleaseEditLockResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）。

## readDraft

`GET /v1/documents/{documentId}/draft` — 读取全篇共享草稿及编辑状态。

- handler：`content.ReadDraft`；权限：`content.read`。
- 路径参数：documentId；查询参数：无。
- 请求：无正文；成功 200：ReadDraftResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## saveDraft

`PUT /v1/documents/{documentId}/draft` — 自动保存完整Tiptap正文。

- handler：`content.SaveDraft`；权限：`content.edit`。
- 路径参数：documentId；查询参数：无。
- 请求：SaveDraftInput；成功 200：SaveDraftResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## publishDocument

`POST /v1/documents/{documentId}:publish` — 点击发布才公开精确共享草稿版本。

- handler：`content.PublishDocument`；权限：`content.publish`。
- 路径参数：documentId；查询参数：无。
- 请求：PublishDocumentInput；成功 200：PublishDocumentResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## withdrawDocument

`POST /v1/documents/{documentId}:withdraw` — 撤回后回草稿并让公开页面下线。

- handler：`content.WithdrawDocument`；权限：`content.publish`。
- 路径参数：documentId；查询参数：无。
- 请求：WithdrawDocumentInput；成功 200：WithdrawDocumentResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## validatePublications

`POST /v1/publications:validate` — 同步核公开引用是否仍可见且当前。

- handler：`content.ValidatePublications`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：ValidatePublicationsInput；成功 200：ValidatePublicationsResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readPublishedRevision

`GET /v1/documents/{documentId}/published` — 获取仍公开的精确正文快照。

- handler：`content.ReadPublishedRevision`；权限：`anonymous`。
- 路径参数：documentId；查询参数：publishedRevisionId。
- 请求：无正文；成功 200：ReadPublishedRevisionResult。
- 错误：NOT_FOUND（资源不存在或在当前主体作用域中不可见）；STALE_RESULT（事件/计算针对已过期源版本，拒绝覆盖新状态）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## trashDocument

`POST /v1/documents/{documentId}:trash` — 删除进回收站，可选旧URL替代跳转。

- handler：`content.TrashDocument`；权限：`content.publish`。
- 路径参数：documentId；查询参数：无。
- 请求：TrashDocumentInput；成功 200：TrashDocumentResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）。

## restoreDocument

`POST /v1/documents/{documentId}:restore` — 从回收站恢复为草稿。

- handler：`content.RestoreDocument`；权限：`content.publish`。
- 路径参数：documentId；查询参数：无。
- 请求：RestoreDocumentInput；成功 200：RestoreDocumentResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## purgeDocuments

`POST /v1/trash:purge` — 只有超管彻底删除/清空回收站。

- handler：`content.PurgeDocuments`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：PurgeDocumentsInput；成功 200：PurgeDocumentsResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readRevisionHistory

`GET /v1/documents/{documentId}/history` — 查看署名发布/git版本历史。

- handler：`content.ReadRevisionHistory`；权限：`content.publish`。
- 路径参数：documentId；查询参数：limit, cursor, assetId。
- 请求：无正文；成功 200：ReadRevisionHistoryResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## diffRevisions

`POST /v1/documents/{documentId}/history:diff` — 按稳定块ID对比历史版本。

- handler：`content.DiffRevisions`；权限：`content.publish`。
- 路径参数：documentId；查询参数：无。
- 请求：DiffRevisionsInput；成功 200：DiffRevisionsResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## rollbackPublishedRevision

`POST /v1/documents/{documentId}/history:rollback` — 回滚历史形成新发布/git提交。

- handler：`content.RollbackPublishedRevision`；权限：`content.publish`。
- 路径参数：documentId；查询参数：无。
- 请求：RollbackPublishedRevisionInput；成功 200：RollbackPublishedRevisionResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## createPreviewGrant

`POST /v1/previews` — 管理员预览草稿公开样式。

- handler：`content.CreatePreviewGrant`；权限：`content.read`。
- 路径参数：无；查询参数：无。
- 请求：CreatePreviewGrantInput；成功 200：CreatePreviewGrantResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## validatePreviewGrant

`POST /v1/previews:validate` — 只向对应管理员验证草稿预览。

- handler：`content.ValidatePreviewGrant`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：ValidatePreviewGrantInput；成功 200：ValidatePreviewGrantResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## listResourceReferences

`GET /v1/resource-references` — 向资源库列出哪些文档引用文件。

- handler：`content.ListResourceReferences`；权限：`service`。
- 路径参数：无；查询参数：limit, cursor, assetId。
- 请求：无正文；成功 200：ListResourceReferencesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readContentObject

`GET /v1/objects/{objectId}` — 经对象API读取私有文件而不共享目录。

- handler：`content.ReadContentObject`；权限：`service`。
- 路径参数：objectId；查询参数：version, expectedHash。
- 请求：无正文；成功 200：ReadContentObjectResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## readPublicTree

`GET /v1/public-tree` — 读取公开导航。

- handler：`content.ReadPublicTree`；权限：`anonymous`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadPublicTreeResult。
- 错误：DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## resolvePublicPath

`GET /v1/public-path:resolve` — 解析301/410公开路径。

- handler：`content.ResolvePublicPath`；权限：`anonymous`。
- 路径参数：无；查询参数：path。
- 请求：无正文；成功 200：ResolvePublicPathResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
