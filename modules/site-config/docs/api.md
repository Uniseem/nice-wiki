# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

## acquireBarrier

`POST /v1/barriers` — 取得可恢复的数据屏障。

- handler：`site-config.AcquireBarrier`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：BarrierRequest；成功 200：BarrierStatus。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## exportSnapshot

`POST /v1/snapshots:export` — 导出屏障水位的一致快照。

- handler：`site-config.ExportSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotExportRequest；成功 200：SnapshotExportResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## validateSnapshot

`POST /v1/snapshots:validate` — 验证快照可恢复性。

- handler：`site-config.ValidateSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotValidateRequest；成功 200：SnapshotValidation。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## prepareSnapshot

`POST /v1/snapshots:prepare` — 在私有 staging 区准备恢复。

- handler：`site-config.PrepareSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotPrepareRequest；成功 200：SnapshotStageResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## commitSnapshot

`POST /v1/snapshots:commit` — 激活已准备恢复数据。

- handler：`site-config.CommitSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## rollbackSnapshot

`POST /v1/snapshots:rollback` — 按协调决定回滚恢复阶段。

- handler：`site-config.RollbackSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## releaseBarrier

`DELETE /v1/barriers/{barrierId}` — 释放屏障。

- handler：`site-config.ReleaseBarrier`；权限：`superadmin`。
- 路径参数：barrierId；查询参数：无。
- 请求：ReleaseBarrierInput；成功 200：ReleaseBarrierResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）。

## readSiteConfig

`GET /v1/config` — 读取后台站点配置。

- handler：`site-config.ReadSiteConfig`；权限：`site.read`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadSiteConfigResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## updateSiteConfig

`PUT /v1/config` — 更新站点常规配置。

- handler：`site-config.UpdateSiteConfig`；权限：`site.manage`。
- 路径参数：无；查询参数：无。
- 请求：UpdateSiteConfigInput；成功 200：UpdateSiteConfigResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## updateHome

`PUT /v1/home` — 配置首页介绍、卡片和精选文档。

- handler：`site-config.UpdateHome`；权限：`site.manage`。
- 路径参数：无；查询参数：无。
- 请求：UpdateHomeInput；成功 200：UpdateHomeResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readPublicHome

`GET /v1/home/public` — 读取可公开的首页配置。

- handler：`site-config.ReadPublicHome`；权限：`anonymous`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadPublicHomeResult。
- 错误：NOT_FOUND（资源不存在或在当前主体作用域中不可见）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readRuntimeState

`GET /v1/runtime-state` — 读取独立控制库权威运行状态。

- handler：`site-config.ReadRuntimeState`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadRuntimeStateResult。
- 错误：DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## transitionRuntimeState

`POST /v1/runtime-state:transition` — 按CAS切换epoch与维护阶段。

- handler：`site-config.TransitionRuntimeState`；权限：`system.manage`。
- 路径参数：无；查询参数：无。
- 请求：RuntimeTransition；成功 200：TransitionRuntimeStateResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## registerActivation

`POST /v1/runtime-state/activations` — 登记域恢复激活签名回执。

- handler：`site-config.RegisterActivation`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：RegisterActivationInput；成功 200：RegisterActivationResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；STALE_RESULT（事件/计算针对已过期源版本，拒绝覆盖新状态）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## initializeSite

`POST /v1/initialization` — 无需AI完成首站点初始化。

- handler：`site-config.InitializeSite`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：InitializeSiteInput；成功 200：InitializeSiteResult。
- 错误：CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；FORBIDDEN（非唯一超管或未注册会话）。

## readInitializationStatus

`GET /v1/initialization` — 读取安全初始化阶段。

- handler：`site-config.ReadInitializationStatus`；权限：`anonymous`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadInitializationStatusResult。
- 错误：DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## resumeInitialization

`POST /v1/initialization:resume` — 恢复崩溃或依赖断开的初始化阶段。

- handler：`site-config.ResumeInitialization`；权限：`site.manage`。
- 路径参数：无；查询参数：无。
- 请求：ResumeInitializationInput；成功 200：ResumeInitializationResult。
- 错误：DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）。

## abortInitialization

`POST /v1/initialization:abort` — 取消尚未持久commit的初始化。

- handler：`site-config.AbortInitialization`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：AbortInitializationInput；成功 200：AbortInitializationResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readConfigObject

`GET /v1/objects/{objectId}` — 经协议读取本域正文或快照对象。

- handler：`site-config.ReadConfigObject`；权限：`service`。
- 路径参数：objectId；查询参数：version, expectedHash。
- 请求：无正文；成功 200：ReadConfigObjectResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

## updateBranding

`PUT /v1/branding` — 更新站名/logo/图标/主题色/默认分享图。

- handler：`site-config.UpdateBranding`；权限：`site.manage`。
- 路径参数：无；查询参数：无。
- 请求：UpdateBrandingInput；成功 200：UpdateBrandingResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## updateFooter

`PUT /v1/footer` — 更新页脚版权/自定义链接/ICP备案/源码链接。

- handler：`site-config.UpdateFooter`；权限：`site.manage`。
- 路径参数：无；查询参数：无。
- 请求：UpdateFooterInput；成功 200：UpdateFooterResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## updateBanner

`PUT /v1/banner` — 保存单条可关闭公告横幅。

- handler：`site-config.UpdateBanner`；权限：`site.manage`。
- 路径参数：无；查询参数：无。
- 请求：UpdateBannerInput；成功 200：UpdateBannerResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readConfigHistory

`GET /v1/config/history` — 站点设置历史。

- handler：`site-config.ReadConfigHistory`；权限：`site.manage`。
- 路径参数：无；查询参数：limit, cursor。
- 请求：无正文；成功 200：ReadConfigHistoryResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## restoreConfigRevision

`POST /v1/config/history/{revisionId}:restore` — 回滚站点设置。

- handler：`site-config.RestoreConfigRevision`；权限：`site.manage`。
- 路径参数：revisionId；查询参数：无。
- 请求：RestoreConfigRevisionInput；成功 200：RestoreConfigRevisionResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## listResourceReferences

`GET /v1/resource-references` — 站点设置完整资源引用。

- handler：`site-config.ListResourceReferences`；权限：`service`。
- 路径参数：无；查询参数：assetId, limit, cursor。
- 请求：无正文；成功 200：ListResourceReferencesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
