# public-site HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext由可信控制通道转发，不能由匿名请求自声明。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/robots.txt` | `handleRobots` | `public-site.HandleRobots` | `anonymous` | `—` → 200 `string` | 机器人规则 | `DEPENDENCY_UNAVAILABLE` |
| GET `/llms.txt` | `handleLLMsRoot` | `public-site.HandleLLMsRoot` | `anonymous` | `—` → 200 `HandleLLMsRootResult` | 根llms语言索引 | `DEPENDENCY_UNAVAILABLE` |
| GET `/sitemap.xml` | `handleSitemapIndex` | `public-site.HandleSitemapIndex` | `anonymous` | `—` → 200 `string` | sitemap 索引 HTTP | `DEPENDENCY_UNAVAILABLE` |
| GET `/sitemap/{locale}/{shard}.xml` | `handleSitemapShard` | `public-site.HandleSitemapShard` | `anonymous` | `—` → 200 `string` | 单分片 sitemap | `VALIDATION`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/preview/tokens` | `issuePreviewToken` | `public-site.IssuePreviewToken` | `content.preview` | `IssuePreviewTokenRequestBody` → 200 `IssuePreviewTokenResult` | 签发草稿预览授权 | `FORBIDDEN`、`NOT_FOUND` |
| GET `/v1/preview/{token}` | `handlePreview` | `public-site.HandlePreview` | `content.preview` | `—` → 200 `string` | 预览 HTTP 入口 | `STALE_RESULT`、`FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| GET `/` | `handleRootRedirect` | `public-site.HandleRootRedirect` | `anonymous` | `—` → — | 根语言选择HTTP | `HTTP 302` |
| GET `/{locale}/` | `handleHome` | `public-site.HandleHome` | `anonymous` | `—` → 200 `string` | 站点首页 | `VALIDATION`、`DEPENDENCY_UNAVAILABLE` |
| GET `/{locale}/{slug}` | `handlePage` | `public-site.HandlePage` | `anonymous` | `—` → 200 `string` | 页面 HTTP 入口 | `VALIDATION`、`NOT_FOUND`、`DEPENDENCY_UNAVAILABLE`、`HTTP 304`、`HTTP 301`、`HTTP 410` |
| GET `/{locale}/{slug}.md` | `handleMarkdown` | `public-site.HandleMarkdown` | `anonymous` | `—` → 200 `string` | 导出公开 Markdown | `NOT_FOUND`、`DEPENDENCY_UNAVAILABLE` |
| GET `/{locale}/navigation` | `handleNavigation` | `public-site.HandleNavigation` | `anonymous` | `—` → 200 `HandleNavigationResult` | 公开导航 HTTP | `DEPENDENCY_UNAVAILABLE`、`VALIDATION` |
| GET `/{locale}/llms.txt` | `handleLLMsIndex` | `public-site.HandleLLMsIndex` | `anonymous` | `—` → 200 `string` | llms.txt HTTP | `VALIDATION`、`DEPENDENCY_UNAVAILABLE` |
| GET `/{locale}/llms-full.txt` | `handleLLMsFull` | `public-site.HandleLLMsFull` | `anonymous` | `—` → 200 `string` | 输出合并正文或分片索引 | `DEPENDENCY_UNAVAILABLE`、`VERSION_MISMATCH` |
| GET `/{locale}/ask` | `handleAskShell` | `public-site.HandleAskShell` | `anonymous` | `—` → 200 `HandleAskShellResult` | 独立问答页面外壳 | `DEPENDENCY_UNAVAILABLE` |
| GET `/{locale}/sitemap.xml` | `handleLanguageSitemap` | `public-site.HandleLanguageSitemap` | `anonymous` | `—` → 200 `string` | 每语言sitemap | `DEPENDENCY_UNAVAILABLE` |
| POST `/v1/barriers` | `acquireBarrier` | `public-site.EnterBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 公开渲染域进入安全屏障 | `DEADLINE_EXCEEDED` |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `public-site.ReleaseBarrier` | `service` | `ReleaseBarrierRequestBody` → 200 `ReleaseBarrierResult` | 释放公开渲染维护 | `LOCKED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `public-site.CreateSnapshot` | `service` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 创建本域一致快照 | `LOCKED`、`INTERNAL` |
| GET `/healthz` | `health` | `public-site.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `public-site.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `public-site.StageSnapshot` | `service` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `public-site.DiscardSnapshot` | `service` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
<!-- openapi:end -->

| GET `/robots.txt` | `handleRobots` | `public-site.HandleRobots` | `anonymous` | 站点级索引策略，不输出私有地址 |
| GET `/llms.txt` | `handleLLMsRoot` | `public-site.HandleLLMsRoot` | `anonymous` | 仅可公开引用的已发布文档摘要索引 |
| GET `/sitemap.xml` | `handleSitemapIndex` | `public-site.HandleSitemapIndex` | `anonymous` | 站点 sitemap 分片索引 |
| GET `/sitemap/{locale}/{shard}.xml` | `handleSitemapShard` | `public-site.HandleSitemapShard` | `anonymous` | 公开、规范、已发布地址和更新时间 |
| GET `/` | `handleRootRedirect` | `public-site.HandleRootRedirect` | `anonymous` | 仅根路径302，cookie/Accept-Language/基准语言优先，Vary正确 |
| GET `/{locale}/` | `handleHome` | `public-site.HandleHome` | `anonymous` | 仅全屏首页hero |
| GET `/{locale}/{slug}` | `handlePage` | `public-site.HandlePage` | `anonymous` | 完整Go三栏HTML、no-cache+etag、旧slug301、删除410 |
| GET `/{locale}/{slug}.md` | `handleMarkdown` | `public-site.HandleMarkdown` | `anonymous` | Tiptap→Markdown，alternate链接，权威撤回检查 |
| GET `/{locale}/navigation` | `handleNavigation` | `public-site.HandleNavigation` | `anonymous` | 公开语言文档树 |
| GET `/{locale}/llms.txt` | `handleLLMsIndex` | `public-site.HandleLLMsIndex` | `anonymous` | 该语言文档树索引 |
| GET `/{locale}/llms-full.txt` | `handleLLMsFull` | `public-site.HandleLLMsFull` | `anonymous` | 该语言全正文或明确分片链接 |
| GET `/{locale}/ask` | `handleAskShell` | `public-site.HandleAskShell` | `anonymous` | 独立问答页，与文档右下浮窗共用qa会话 |
| GET `/{locale}/sitemap.xml` | `handleLanguageSitemap` | `public-site.HandleLanguageSitemap` | `anonymous` | 每语言sitemap，root sitemap.xml总索引 |

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；stage只写restore/<restoreId>/staged，不动active；discard只删本次恢复的暂存或旧目录；切换在进程重启时完成（ADR0003）。
