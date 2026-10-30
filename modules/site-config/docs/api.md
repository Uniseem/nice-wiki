# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/v1/config` | `readSiteConfig` | `site-config.ReadSiteConfig` | `site.read` | `—` → 200 `ReadSiteConfigResult` | 读取后台站点配置 | `FORBIDDEN`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR`、`VERSION_MISMATCH` |
| PUT `/v1/config` | `updateSiteConfig` | `site-config.UpdateSiteConfig` | `site.manage` | `UpdateSiteConfigInput` → 200 `UpdateSiteConfigResult` | 更新站点常规配置 | `VALIDATION`、`FORBIDDEN`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR`、`NOT_FOUND` |
| PUT `/v1/home` | `updateHome` | `site-config.UpdateHome` | `site.manage` | `UpdateHomeInput` → 200 `UpdateHomeResult` | 配置首页介绍、卡片和精选文档 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE`、`NOT_FOUND`、`INTEGRITY_ERROR`、`LOCKED`、`DEADLINE_EXCEEDED` |
| GET `/v1/home/public` | `readPublicHome` | `site-config.ReadPublicHome` | `anonymous` | `—` → 200 `ReadPublicHomeResult` | 读取可公开的首页配置 | `NOT_FOUND`、`DEPENDENCY_UNAVAILABLE`、`VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`INTEGRITY_ERROR` |
| GET `/v1/runtime-state` | `readRuntimeState` | `site-config.ReadRuntimeState` | `service` | `—` → 200 `ReadRuntimeStateResult` | 读取独立控制库权威运行状态 | `INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/initialization` | `readInitializationStatus` | `site-config.ReadInitializationStatus` | `anonymous` | `—` → 200 `ReadInitializationStatusResult` | 读取安全初始化阶段 | `DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR` |
| POST `/v1/initialization` | `initializeSite` | `site-config.InitializeSite` | `site.manage` | `InitializeSiteInput` → 200 `InitializeSiteResult` | 无需AI完成首站点初始化 | `VALIDATION`、`FORBIDDEN`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR` |
| POST `/v1/initialization:resume` | `resumeInitialization` | `site-config.ResumeInitialization` | `site.manage` | `ResumeInitializationInput` → 200 `ResumeInitializationResult` | 恢复崩溃或依赖断开的初始化阶段 | `CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`FORBIDDEN` |
| POST `/v1/initialization:abort` | `abortInitialization` | `site-config.AbortInitialization` | `superadmin` | `AbortInitializationInput` → 200 `AbortInitializationResult` | 取消尚未持久commit的初始化 | `FORBIDDEN`、`CONFLICT`、`VERSION_MISMATCH`、`LOCKED`、`DEPENDENCY_UNAVAILABLE`、`INTEGRITY_ERROR` |
| GET `/v1/objects/{objectId}` | `readConfigObject` | `site-config.ReadConfigObject` | `site.read` | `—` → 200 `ConfigDocument` | 读取已提交的本域不可变对象原始字节 | `FORBIDDEN`、`NOT_FOUND`、`INTEGRITY_ERROR`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/branding` | `updateBranding` | `site-config.UpdateBranding` | `site.manage` | `UpdateBrandingInput` → 200 `UpdateBrandingResult` | 更新站名/logo/图标/主题色/默认分享图 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/footer` | `updateFooter` | `site-config.UpdateFooter` | `site.manage` | `UpdateFooterInput` → 200 `UpdateFooterResult` | 更新页脚版权/自定义链接/ICP备案/源码链接 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/banner` | `updateBanner` | `site-config.UpdateBanner` | `site.manage` | `UpdateBannerInput` → 200 `UpdateBannerResult` | 保存单条可关闭公告横幅 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/config/history` | `readConfigHistory` | `site-config.ReadConfigHistory` | `site.manage` | `—` → 200 `ReadConfigHistoryResult` | 站点设置历史 | `FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/config/history/{revisionId}:restore` | `restoreConfigRevision` | `site-config.RestoreConfigRevision` | `site.manage` | `restoreConfigRevisionRequest` → 200 `RestoreConfigRevisionResult` | 回滚站点设置 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/resource-references` | `listResourceReferences` | `site-config.ListResourceReferences` | `site.manage` | `—` → 200 `ListResourceReferencesResult` | 站点设置完整资源引用 | `VALIDATION`、`FORBIDDEN`、`VERSION_MISMATCH`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| GET `/healthz` | `health` | `site-config.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `site-config.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| GET `/v1/reference-decisions/{sourceOperationId}` | `readHomeReferenceDecision` | `site-config.ReadHomeReferenceDecision` | `service` | `—` → 200 `ReadHomeReferenceDecisionResult` | 媒体核对首页资源来源持久决定 | `HTTP 403`、`HTTP 404`、`HTTP 409`、`HTTP 422`、`HTTP 503` |
| GET `/v1/appearance` | `readAppearance` | `site-config.ReadAppearance` | `site.manage` | `—` → 200 `ReadAppearanceResult` | 读取站点外观与可选外观 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/appearance` | `updateAppearance` | `site-config.UpdateAppearance` | `site.manage` | `UpdateAppearanceInput` → 200 `Appearance` | 切换站点外观或访客默认明暗 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/appearance/themes` | `installTheme` | `site-config.InstallTheme` | `site.manage` | `application/octet-stream` → 200 `InstallThemeResult` | 上传外观包（zip）装到本站 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| DELETE `/v1/appearance/themes/{digest}` | `deleteTheme` | `site-config.DeleteTheme` | `site.manage` | `—` → 200 `DeleteThemeResult` | 删除本站装的外观包（正在用的不能删） | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/appearance/public` | `readPublicAppearance` | `site-config.ReadPublicAppearance` | `anonymous` | `—` → 200 `PublicAppearance` | 读取公开站使用的外观 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/appearance/packages/{digest}` | `readThemePackage` | `site-config.ReadThemePackage` | `anonymous` | `—` → 200 `application/zip` | 按摘要下载外观包（内置或本站装的） | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`NOT_FOUND`、`CONFLICT`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
<!-- openapi:end -->

