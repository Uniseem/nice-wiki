# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| POST `/v1/barriers` | `acquireBarrier` | `site-config.AcquireBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得可恢复的数据屏障 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `site-config.ExportSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出屏障水位的一致快照 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| DELETE `/v1/barriers/{barrierId}` | `releaseBarrier` | `site-config.ReleaseBarrier` | `service` | `releaseBarrierRequest` → 200 `ReleaseBarrierResult` | 释放屏障 | `FORBIDDEN`、`VERSION_MISMATCH` |
| GET `/v1/config` | `readSiteConfig` | `site-config.ReadSiteConfig` | `site.read` | `—` → 200 `ReadSiteConfigResult` | 读取后台站点配置 | `FORBIDDEN`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/config` | `updateSiteConfig` | `site-config.UpdateSiteConfig` | `site.manage` | `UpdateSiteConfigInput` → 200 `UpdateSiteConfigResult` | 更新站点常规配置 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/home` | `updateHome` | `site-config.UpdateHome` | `site.manage` | `UpdateHomeInput` → 200 `UpdateHomeResult` | 配置首页介绍、卡片和精选文档 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/home/public` | `readPublicHome` | `site-config.ReadPublicHome` | `anonymous` | `—` → 200 `ReadPublicHomeResult` | 读取可公开的首页配置 | `NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/runtime-state` | `readRuntimeState` | `site-config.ReadRuntimeState` | `service` | `—` → 200 `ReadRuntimeStateResult` | 读取独立控制库权威运行状态 | `INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/runtime-state:transition` | `transitionRuntimeState` | `site-config.TransitionRuntimeState` | `service` | `RuntimeTransition` → 200 `TransitionRuntimeStateResult` | 按CAS切换epoch与维护阶段 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/runtime-state/activations` | `registerActivation` | `site-config.RegisterActivation` | `service` | `RegisterActivationInput` → 200 `RegisterActivationResult` | 登记域恢复切换/换回回执 | `FORBIDDEN`、`VERSION_MISMATCH`、`INTEGRITY_ERROR` |
| GET `/v1/initialization` | `readInitializationStatus` | `site-config.ReadInitializationStatus` | `anonymous` | `—` → 200 `ReadInitializationStatusResult` | 读取安全初始化阶段 | `DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR` |
| POST `/v1/initialization` | `initializeSite` | `site-config.InitializeSite` | `superadmin` | `InitializeSiteInput` → 200 `InitializeSiteResult` | 无需AI完成首站点初始化 | `VALIDATION`、`FORBIDDEN`、`CONFLICT`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/initialization:resume` | `resumeInitialization` | `site-config.ResumeInitialization` | `site.manage` | `ResumeInitializationInput` → 200 `ResumeInitializationResult` | 恢复崩溃或依赖断开的初始化阶段 | `CONFLICT`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/initialization:abort` | `abortInitialization` | `site-config.AbortInitialization` | `superadmin` | `AbortInitializationInput` → 200 `AbortInitializationResult` | 取消尚未持久commit的初始化 | `FORBIDDEN`、`CONFLICT`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/objects/{objectId}` | `readConfigObject` | `site-config.ReadConfigObject` | `service` | `—` → 200 `ReadConfigObjectResult` | 经协议读取本域正文或快照对象 | `FORBIDDEN`、`NOT_FOUND`、`INTEGRITY_ERROR` |
| PUT `/v1/branding` | `updateBranding` | `site-config.UpdateBranding` | `site.manage` | `UpdateBrandingInput` → 200 `UpdateBrandingResult` | 更新站名/logo/图标/主题色/默认分享图 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/footer` | `updateFooter` | `site-config.UpdateFooter` | `site.manage` | `UpdateFooterInput` → 200 `UpdateFooterResult` | 更新页脚版权/自定义链接/ICP备案/源码链接 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/banner` | `updateBanner` | `site-config.UpdateBanner` | `site.manage` | `UpdateBannerInput` → 200 `UpdateBannerResult` | 保存单条可关闭公告横幅 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/config/history` | `readConfigHistory` | `site-config.ReadConfigHistory` | `site.manage` | `—` → 200 `ReadConfigHistoryResult` | 站点设置历史 | `FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/config/history/{revisionId}:restore` | `restoreConfigRevision` | `site-config.RestoreConfigRevision` | `site.manage` | `restoreConfigRevisionRequest` → 200 `RestoreConfigRevisionResult` | 回滚站点设置 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/resource-references` | `listResourceReferences` | `site-config.ListResourceReferences` | `service` | `—` → 200 `ListResourceReferencesResult` | 站点设置完整资源引用 | `FORBIDDEN`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| GET `/healthz` | `health` | `site-config.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `site-config.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `site-config.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `site-config.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
<!-- openapi:end -->

