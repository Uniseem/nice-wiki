# public-site 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 70 个：手写 52、UI 4、生成 4、配置 9、迁移 1。函数 67 个：约束 32，参考 35。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/public-site/internal/routing.go` | 手写 | 规范化语言与 slug；同文件职责限本域以下登记函数 | `public-site.ResolveRoute`（参考） |
| `modules/public-site/internal/locale.go` | 手写 | 确定展示语言；同文件职责限本域以下登记函数 | `public-site.SelectLocale`（参考） |
| `modules/public-site/internal/authority.go` | 手写 | 执行正文公开权威检查；同文件职责限本域以下登记函数 | `public-site.CheckPublication`（参考） |
| `modules/public-site/internal/content_client.go` | 手写 | 取得同一发布修订完整包；同文件职责限本域以下登记函数 | `public-site.LoadPublishedBundle`（参考） |
| `modules/public-site/internal/markdown.go` | 手写 | 构建确定性 Markdown AST；同文件职责限本域以下登记函数 | `public-site.ParseMarkdown`（参考） |
| `modules/public-site/internal/sanitize.go` | 手写 | 移除危险结构及协议；同文件职责限本域以下登记函数 | `public-site.SanitizeHTML`（参考） |
| `modules/public-site/internal/toc.go` | 手写 | 生成稳定锚点目录；同文件职责限本域以下登记函数 | `public-site.BuildTOC`（参考） |
| `modules/public-site/internal/links.go` | 手写 | 解析跨文档链接；同文件职责限本域以下登记函数 | `public-site.RewriteInternalLinks`（参考） |
| `modules/public-site/internal/media_client.go` | 手写 | 取得公开资源清单；同文件职责限本域以下登记函数 | `public-site.ResolvePublicMedia`（参考） |
| `modules/public-site/internal/navigation.go` | 手写 | 构建可访问公开导航树；同文件职责限本域以下登记函数 | `public-site.BuildNavigation`（参考） |
| `modules/public-site/internal/breadcrumbs.go` | 手写 | 生成面包屑；同文件职责限本域以下登记函数 | `public-site.BuildBreadcrumbs`（参考） |
| `modules/public-site/internal/seo.go` | 手写 | 生成规范地址；同文件职责限本域以下登记函数 | `public-site.BuildCanonical`（参考） |
| `modules/public-site/internal/hreflang.go` | 手写 | 输出已有公开语言版本；同文件职责限本域以下登记函数 | `public-site.BuildHreflang`（参考） |
| `modules/public-site/internal/metadata.go` | 手写 | 输出 SEO 和分享元数据；同文件职责限本域以下登记函数 | `public-site.BuildSEOMetadata`（参考） |
| `modules/public-site/internal/html.go` | 手写 | 构成完整响应文档；同文件职责限本域以下登记函数 | `public-site.ComposeHTML`（参考） |
| `modules/public-site/internal/cache_key.go` | 手写 | 固定派生缓存身份；同文件职责限本域以下登记函数 | `public-site.BuildCacheKey`（参考） |
| `modules/public-site/internal/cache_store.go` | 手写 | 读取派生 HTML；同文件职责限本域以下登记函数 | `public-site.ReadRenderCache`（参考）、`public-site.WriteRenderCache`（参考） |
| `modules/public-site/internal/invalidation.go` | 手写 | 处理发布和撤回事件；同文件职责限本域以下登记函数 | `public-site.InvalidatePublication` |
| `modules/public-site/internal/renderer.go` | 手写 | 编排公开正文输出；同文件职责限本域以下登记函数 | `public-site.RenderPublicPage`（参考） |
| `modules/public-site/internal/http_pages.go` | 手写 | 页面 HTTP 入口；同文件职责限本域以下登记函数 | `public-site.HandlePage` |
| `modules/public-site/internal/http_home.go` | 手写 | 站点首页；同文件职责限本域以下登记函数 | `public-site.HandleHome` |
| `modules/public-site/internal/http_navigation.go` | 手写 | 公开导航 HTTP；同文件职责限本域以下登记函数 | `public-site.HandleNavigation` |
| `modules/public-site/internal/http_markdown.go` | 手写 | 导出公开 Markdown；同文件职责限本域以下登记函数 | `public-site.HandleMarkdown` |
| `modules/public-site/internal/http_robots.go` | 手写 | 机器人规则；同文件职责限本域以下登记函数 | `public-site.HandleRobots` |
| `modules/public-site/internal/llms.go` | 手写 | 生成模型可读公开索引；同文件职责限本域以下登记函数 | `public-site.BuildLLMsIndex`（参考） |
| `modules/public-site/internal/http_llms.go` | 手写 | llms.txt HTTP；同文件职责限本域以下登记函数 | `public-site.HandleLLMsIndex`、`public-site.HandleLLMsFull` |
| `modules/public-site/internal/sitemap.go` | 手写 | 生成规范 sitemap 项；同文件职责限本域以下登记函数 | `public-site.BuildSitemapEntries`（参考） |
| `modules/public-site/internal/http_sitemap.go` | 手写 | sitemap 索引 HTTP；同文件职责限本域以下登记函数 | `public-site.HandleSitemapIndex`、`public-site.HandleSitemapShard` |
| `modules/public-site/internal/preview.go` | 手写 | 签发草稿预览授权；同文件职责限本域以下登记函数 | `public-site.IssuePreviewToken`、`public-site.ValidatePreviewToken`（参考） |
| `modules/public-site/internal/preview_renderer.go` | 手写 | 渲染受保护修订；同文件职责限本域以下登记函数 | `public-site.RenderPreview`（参考） |
| `modules/public-site/internal/http_preview.go` | 手写 | 预览 HTTP 入口；同文件职责限本域以下登记函数 | `public-site.HandlePreview` |
| `modules/public-site/internal/beacon.go` | 手写 | AI爬虫Markdown访问单独计数；同文件职责限本域以下登记函数 | `public-site.RecordAnonymousView`（参考） |
| `modules/public-site/internal/snapshot.go` | 手写 | 创建本域一致快照；同文件职责限本域以下登记函数 | `public-site.CreateSnapshot` |
| `modules/public-site/internal/import.go` | 手写 | 验证待恢复快照；同文件职责限本域以下登记函数 | `public-site.ValidateImport`、`public-site.PrepareImport`、`public-site.ActivateImport`、`public-site.AbortImport` |
| `modules/public-site/internal/barrier.go` | 手写 | 公开渲染域进入安全屏障；同文件职责限本域以下登记函数 | `public-site.EnterBarrier`、`public-site.ReleaseBarrier` |
| `modules/public-site/internal/recovery_authorization.go` | 手写 | 本地验证本次恢复专用控制许可；同文件职责限本域以下登记函数 | `public-site.ValidateRecoveryAuthorization`（参考） |
| `modules/public-site/web/src/PreviewPanel.vue` | UI | Vue3模块交互：LoadPreview；同文件职责限本域以下登记函数 | `public-site.LoadPreview`（参考） |
| `modules/public-site/web/src/PublicSettingsPanel.vue` | UI | Vue3模块交互：LoadPublicCapabilities；同文件职责限本域以下登记函数 | `public-site.LoadPublicCapabilities`（参考） |
| `modules/public-site/web/src/client.ts` | UI | Vue3模块交互：RequestPublicNavigation；同文件职责限本域以下登记函数 | `public-site.RequestPublicNavigation`（参考） |
| `modules/public-site/internal/http_root.go` | 手写 | 根语言选择HTTP；同文件职责限本域以下登记函数 | `public-site.HandleRootRedirect` |
| `modules/public-site/internal/tiptap.go` | 手写 | 校验权威Tiptap渲染输入；同文件职责限本域以下登记函数 | `public-site.ParseTiptapDocument`（参考） |
| `modules/public-site/internal/tiptap_html.go` | 手写 | 按节点输出完整HTML；同文件职责限本域以下登记函数 | `public-site.RenderTiptapHTML`（参考） |
| `modules/public-site/internal/tiptap_markdown.go` | 手写 | Tiptap输出AI可读Markdown；同文件职责限本域以下登记函数 | `public-site.RenderTiptapMarkdown`（参考） |
| `modules/public-site/internal/hero.go` | 手写 | 仅一屏默认主视觉；同文件职责限本域以下登记函数 | `public-site.RenderHero`（参考） |
| `modules/public-site/internal/announcement.go` | 手写 | 公告翻译横幅；同文件职责限本域以下登记函数 | `public-site.RenderAnnouncement`（参考） |
| `modules/public-site/internal/navigation_neighbors.go` | 手写 | 文末相邻文档；同文件职责限本域以下登记函数 | `public-site.BuildPreviousNext`（参考） |
| `modules/public-site/internal/http_errors.go` | 手写 | 定制404页面；同文件职责限本域以下登记函数 | `public-site.RenderNotFound`（参考） |
| `modules/public-site/internal/http_ask.go` | 手写 | 独立问答页面外壳；同文件职责限本域以下登记函数 | `public-site.HandleAskShell` |
| `modules/public-site/internal/prewarm.go` | 手写 | 发布事件预渲染缓存；同文件职责限本域以下登记函数 | `public-site.PrewarmPublication` |
| `modules/public-site/internal/http_llms_root.go` | 手写 | 根llms语言索引；同文件职责限本域以下登记函数 | `public-site.HandleLLMsRoot` |
| `modules/public-site/internal/http_language_sitemap.go` | 手写 | 每语言sitemap；同文件职责限本域以下登记函数 | `public-site.HandleLanguageSitemap` |
| `modules/public-site/internal/types.gen.go` | 生成 | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module public-site --target go-types，输出不得手改 | 无 |
| `modules/public-site/internal/http_bindings.gen.go` | 生成 | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module public-site --target go-http，x-handler映射仅转调手写函数 | 无 |
| `modules/public-site/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/public-site/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 | 无 |
| `modules/public-site/cmd/public-site/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `public-site.main` |
| `modules/public-site/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `public-site.BuildApplication` |
| `modules/public-site/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `public-site.Serve`、`public-site.Shutdown`、`public-site.Health`、`public-site.Ready` |
| `modules/public-site/go.mod` | 配置 | 独立module nice-wiki/modules/public-site；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/public-site/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/public-site/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/public-site；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/public-site/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/public-site与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/public-site/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/public-site/web/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/public-site/web/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/public-site/web/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/public-site/web/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/public-site/web/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/public-site/web/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `public-site.mountModuleUI` |
| `modules/public-site/internal/events/consume.go` | 手写 | 公开投影持久事件消费和恢复预热，不能改变内容权威 | `public-site.ConsumeEvents` |
