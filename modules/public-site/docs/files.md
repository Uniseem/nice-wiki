# public-site 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=69；分类={"handwritten": 51, "ui": 4, "generated": 4, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/public-site/internal/routing.go` | handwritten | 规范化语言与 slug；同文件职责限本域以下登记函数 | `M06-F001` |
| `modules/public-site/internal/locale.go` | handwritten | 确定展示语言；同文件职责限本域以下登记函数 | `M06-F002` |
| `modules/public-site/internal/authority.go` | handwritten | 执行正文公开权威检查；同文件职责限本域以下登记函数 | `M06-F003` |
| `modules/public-site/internal/content_client.go` | handwritten | 取得同一发布修订完整包；同文件职责限本域以下登记函数 | `M06-F004` |
| `modules/public-site/internal/markdown.go` | handwritten | 构建确定性 Markdown AST；同文件职责限本域以下登记函数 | `M06-F005` |
| `modules/public-site/internal/sanitize.go` | handwritten | 移除危险结构及协议；同文件职责限本域以下登记函数 | `M06-F006` |
| `modules/public-site/internal/toc.go` | handwritten | 生成稳定锚点目录；同文件职责限本域以下登记函数 | `M06-F007` |
| `modules/public-site/internal/links.go` | handwritten | 解析跨文档链接；同文件职责限本域以下登记函数 | `M06-F008` |
| `modules/public-site/internal/media_client.go` | handwritten | 取得公开资源清单；同文件职责限本域以下登记函数 | `M06-F009` |
| `modules/public-site/internal/navigation.go` | handwritten | 构建可访问公开导航树；同文件职责限本域以下登记函数 | `M06-F010` |
| `modules/public-site/internal/breadcrumbs.go` | handwritten | 生成面包屑；同文件职责限本域以下登记函数 | `M06-F011` |
| `modules/public-site/internal/seo.go` | handwritten | 生成规范地址；同文件职责限本域以下登记函数 | `M06-F012` |
| `modules/public-site/internal/hreflang.go` | handwritten | 输出已有公开语言版本；同文件职责限本域以下登记函数 | `M06-F013` |
| `modules/public-site/internal/metadata.go` | handwritten | 输出 SEO 和分享元数据；同文件职责限本域以下登记函数 | `M06-F014` |
| `modules/public-site/internal/html.go` | handwritten | 构成完整响应文档；同文件职责限本域以下登记函数 | `M06-F015` |
| `modules/public-site/internal/cache_key.go` | handwritten | 固定派生缓存身份；同文件职责限本域以下登记函数 | `M06-F016` |
| `modules/public-site/internal/cache_store.go` | handwritten | 读取派生 HTML；同文件职责限本域以下登记函数 | `M06-F017`, `M06-F018` |
| `modules/public-site/internal/invalidation.go` | handwritten | 处理发布和撤回事件；同文件职责限本域以下登记函数 | `M06-F019` |
| `modules/public-site/internal/renderer.go` | handwritten | 编排公开正文输出；同文件职责限本域以下登记函数 | `M06-F020` |
| `modules/public-site/internal/http_pages.go` | handwritten | 页面 HTTP 入口；同文件职责限本域以下登记函数 | `M06-F021` |
| `modules/public-site/internal/http_home.go` | handwritten | 站点首页；同文件职责限本域以下登记函数 | `M06-F022` |
| `modules/public-site/internal/http_navigation.go` | handwritten | 公开导航 HTTP；同文件职责限本域以下登记函数 | `M06-F023` |
| `modules/public-site/internal/http_markdown.go` | handwritten | 导出公开 Markdown；同文件职责限本域以下登记函数 | `M06-F024` |
| `modules/public-site/internal/http_robots.go` | handwritten | 机器人规则；同文件职责限本域以下登记函数 | `M06-F025` |
| `modules/public-site/internal/llms.go` | handwritten | 生成模型可读公开索引；同文件职责限本域以下登记函数 | `M06-F026` |
| `modules/public-site/internal/http_llms.go` | handwritten | llms.txt HTTP；同文件职责限本域以下登记函数 | `M06-F027`, `M06-F028` |
| `modules/public-site/internal/sitemap.go` | handwritten | 生成规范 sitemap 项；同文件职责限本域以下登记函数 | `M06-F029` |
| `modules/public-site/internal/http_sitemap.go` | handwritten | sitemap 索引 HTTP；同文件职责限本域以下登记函数 | `M06-F030`, `M06-F031` |
| `modules/public-site/internal/preview.go` | handwritten | 签发草稿预览授权；同文件职责限本域以下登记函数 | `M06-F032`, `M06-F033` |
| `modules/public-site/internal/preview_renderer.go` | handwritten | 渲染受保护修订；同文件职责限本域以下登记函数 | `M06-F034` |
| `modules/public-site/internal/http_preview.go` | handwritten | 预览 HTTP 入口；同文件职责限本域以下登记函数 | `M06-F035` |
| `modules/public-site/internal/beacon.go` | handwritten | AI爬虫Markdown访问单独计数；同文件职责限本域以下登记函数 | `M06-F036` |
| `modules/public-site/internal/snapshot.go` | handwritten | 创建本域一致快照；同文件职责限本域以下登记函数 | `M06-F037` |
| `modules/public-site/internal/import.go` | handwritten | 验证待恢复快照；同文件职责限本域以下登记函数 | `M06-F038`, `M06-F039`, `M06-F040`, `M06-F041` |
| `modules/public-site/internal/barrier.go` | handwritten | 公开渲染域进入安全屏障；同文件职责限本域以下登记函数 | `M06-F042`, `M06-F043` |
| `modules/public-site/internal/recovery_authorization.go` | handwritten | 本地验证本次恢复专用控制许可；同文件职责限本域以下登记函数 | `M06-F044` |
| `modules/public-site/web/src/PreviewPanel.vue` | ui | Vue3模块交互：LoadPreview；同文件职责限本域以下登记函数 | `M06-F045` |
| `modules/public-site/web/src/PublicSettingsPanel.vue` | ui | Vue3模块交互：LoadPublicCapabilities；同文件职责限本域以下登记函数 | `M06-F046` |
| `modules/public-site/web/src/client.ts` | ui | Vue3模块交互：RequestPublicNavigation；同文件职责限本域以下登记函数 | `M06-F047` |
| `modules/public-site/internal/http_root.go` | handwritten | 根语言选择HTTP；同文件职责限本域以下登记函数 | `M06-F048` |
| `modules/public-site/internal/tiptap.go` | handwritten | 校验权威Tiptap渲染输入；同文件职责限本域以下登记函数 | `M06-F049` |
| `modules/public-site/internal/tiptap_html.go` | handwritten | 按节点输出完整HTML；同文件职责限本域以下登记函数 | `M06-F050` |
| `modules/public-site/internal/tiptap_markdown.go` | handwritten | Tiptap输出AI可读Markdown；同文件职责限本域以下登记函数 | `M06-F051` |
| `modules/public-site/internal/hero.go` | handwritten | 仅一屏默认主视觉；同文件职责限本域以下登记函数 | `M06-F052` |
| `modules/public-site/internal/announcement.go` | handwritten | 公告翻译横幅；同文件职责限本域以下登记函数 | `M06-F053` |
| `modules/public-site/internal/navigation_neighbors.go` | handwritten | 文末相邻文档；同文件职责限本域以下登记函数 | `M06-F054` |
| `modules/public-site/internal/http_errors.go` | handwritten | 定制404页面；同文件职责限本域以下登记函数 | `M06-F055` |
| `modules/public-site/internal/http_ask.go` | handwritten | 独立问答页面外壳；同文件职责限本域以下登记函数 | `M06-F056` |
| `modules/public-site/internal/prewarm.go` | handwritten | 发布事件预渲染缓存；同文件职责限本域以下登记函数 | `M06-F057` |
| `modules/public-site/internal/http_llms_root.go` | handwritten | 根llms语言索引；同文件职责限本域以下登记函数 | `M06-F058` |
| `modules/public-site/internal/http_language_sitemap.go` | handwritten | 每语言sitemap；同文件职责限本域以下登记函数 | `M06-F059` |
| `modules/public-site/internal/types.gen.go` | generated | 从contracts/types.json和公共schema生成Go字段模型；生成器接口 contractgen --module public-site --target go-types，输出不得手改 |  |
| `modules/public-site/internal/http_bindings.gen.go` | generated | 从OpenAPI生成严格入参绑定和Problem输出；接口 contractgen --module public-site --target go-http，x-handler映射仅转调手写函数 |  |
| `modules/public-site/config/defaults.json` | config | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/public-site/migrations/0001_domain.sql` | migration | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 |  |
| `modules/public-site/cmd/public-site/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M06-F901` |
| `modules/public-site/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M06-F902` |
| `modules/public-site/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M06-F903`, `M06-F904`, `M06-F905`, `M06-F906` |
| `modules/public-site/go.mod` | config | 独立module nice-wiki/modules/public-site；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/public-site/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/public-site/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/public-site；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/public-site/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/public-site与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/public-site/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/public-site/web/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/public-site/web/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/public-site/web/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/public-site/web/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/public-site/web/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/public-site/web/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M06-F907` |

| `modules/public-site/internal/events/consume.go` | handwritten | 公开投影事件消费、inbox与有界预热 | `M06-F060` |
