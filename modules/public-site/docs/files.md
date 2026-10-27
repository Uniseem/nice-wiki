# public-site 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 61 个：手写 49、UI 1、生成 4、配置 6、迁移 1。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/public-site/internal/routing.go` | 手写 | 规范化语言与 slug；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/locale.go` | 手写 | 确定展示语言；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/authority.go` | 手写 | 执行正文公开权威检查；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/content_client.go` | 手写 | 取得同一发布修订完整包；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/sanitize.go` | 手写 | 移除危险结构及协议；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/toc.go` | 手写 | 生成稳定锚点目录；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/links.go` | 手写 | 解析跨文档链接；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/media_client.go` | 手写 | 取得公开资源清单；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/navigation.go` | 手写 | 构建可访问公开导航树；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/breadcrumbs.go` | 手写 | 生成面包屑；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/seo.go` | 手写 | 生成规范地址；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/hreflang.go` | 手写 | 输出已有公开语言版本；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/metadata.go` | 手写 | 输出 SEO 和分享元数据；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/html.go` | 手写 | 构成完整响应文档；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/cache_key.go` | 手写 | 固定派生缓存身份；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/cache_store.go` | 手写 | 读取派生 HTML；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/invalidation.go` | 手写 | 处理发布和撤回事件；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/renderer.go` | 手写 | 编排公开正文输出；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_pages.go` | 手写 | 页面 HTTP 入口；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_home.go` | 手写 | 站点首页；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_navigation.go` | 手写 | 公开导航 HTTP；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_markdown.go` | 手写 | 导出公开 Markdown；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_robots.go` | 手写 | 机器人规则；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/llms.go` | 手写 | 生成模型可读公开索引；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_llms.go` | 手写 | llms.txt HTTP；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/sitemap.go` | 手写 | 生成规范 sitemap 项；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_sitemap.go` | 手写 | sitemap 索引 HTTP；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/preview.go` | 手写 | 签发草稿预览授权；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/preview_renderer.go` | 手写 | 渲染受保护修订；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_preview.go` | 手写 | 预览 HTTP 入口；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/beacon.go` | 手写 | AI爬虫Markdown访问单独计数；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/import.go` | 手写 | 验证待恢复快照；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_root.go` | 手写 | 根语言选择HTTP；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/tiptap.go` | 手写 | 校验权威Tiptap渲染输入；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/tiptap_html.go` | 手写 | 按节点输出完整HTML；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/tiptap_markdown.go` | 手写 | Tiptap输出AI可读Markdown；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/hero.go` | 手写 | 仅一屏默认主视觉；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/announcement.go` | 手写 | 公告翻译横幅；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/navigation_neighbors.go` | 手写 | 文末相邻文档；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_errors.go` | 手写 | 定制404页面；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_ask.go` | 手写 | 独立问答页面外壳；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/prewarm.go` | 手写 | 发布事件预渲染缓存；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_llms_root.go` | 手写 | 根llms语言索引；同文件职责限本域以下登记函数 |
| `modules/public-site/internal/http_language_sitemap.go` | 手写 | 每语言sitemap；同文件职责限本域以下登记函数 |
| `modules/public-site/config/defaults.json` | 配置 | 首版默认参数与环境覆盖允许列表；禁止secret原文，secret仅外部ref；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/public-site/migrations/0001_domain.sql` | 迁移 | 创建data.md登记私有SQLite表/索引/版本；迁移通过隔离验证后升级，失败不切活跃指针 |
| `modules/public-site/cmd/public-site/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/public-site/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/public-site/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/public-site/go.mod` | 配置 | 独立module nice-wiki/modules/public-site；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/public-site/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/public-site/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/public-site；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/public-site/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/public-site与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/public-site/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/public-site/internal/events/consume.go` | 手写 | 公开投影持久事件消费和恢复预热，不能改变内容权威 |
| `modules/public-site/internal/worker.go` | 手写 | 主动领取本域持久任务的循环 |
| `modules/public-site/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/public-site/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 |
| `modules/public-site/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/public-site/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 |
| `modules/public-site/web/src/site.tsx` | UI | 公开页交互岛：一个 React 根通过 portal 接管页头操作（问答、语言、明暗）、窄屏文档树抽屉与问答区；另附代码复制、目录高亮、公式、访问信标等增强 |
