# public-site 需求

**M06-R001** 将 content 已发布版本渲染成可索引、可访问的完整 HTML，并提供导航、SEO、多语言路由、Markdown、llms、sitemap 与受保护预览。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R002** 完整 HTML 输出、无 JavaScript 可阅读的导航及正文

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R003** 按语言和发布修订派生的渲染缓存及公开导航投影

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R004** 公开页面、Markdown、机器人索引与预览的权限边界

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R005** 保留old D12/D14/D17/D19/D20/D26/D29/D32/D34/D54：Go完整三栏、首页单屏、Markdown .md、AI浮窗/独立页、各语言索引、localStorage公告关闭、标题不折叠

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R006** 翻译未完成显示旧译/原文并在页首当前语言提示；blockId跨语言保持稳定锚点，切换同文档同位置

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R007** 不持有正文源历史，不修改发布状态

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R008** 不根据缓存或异步事件单独判断页面仍可公开

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R009** 不执行翻译算法、不调用付费模型、不控制费用账本

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R010** 服务间HTTP/JSON/SSE；事件使用公共EventEnvelope并标版本，至少一次按eventId和源修订去重

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R011** 所有写入验证当前dataEpoch，恢复后旧任务/租约/token/缓存不得修改或授权新数据

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R012** 私有存储；外域只通过拥有者HTTP合同读写，不共享数据库或直接读私有文件

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R013** 所有有状态域实现统一barrier/export/stage/discard；恢复时重启换入暂存数据，全部域登记switched回执后才解除维护

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R014** 日志、事件与镜像不记录IP、提问正文或secret；结构化字段仅allowlist

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R015** 未来Go后端、Vue3模块界面按文件/函数合同手写；本阶段只登记设计，无业务源码和测试样例

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R016** content 停机：所有正文/导航/sitemap/llms/Markdown 返回 503，不能把旧 cache 当安全授权；静态 CSS/图标可继续读。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R017** media 不可达：可公开正文使用固定尺寸占位且不输出未知资源 URL；受保护预览资源失败仍不落公开缓存。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R018** analytics 停机：匿名 beacon 丢弃；HTML 不因此失败。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M06-R019** public-site 重启：扫描 cache 索引文件哈希与遗留 tmp，损坏命中转 miss；authority 检查始终启用。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

首版默认参数：

```json
{
  "defaultLocale": "zh-Hans",
  "cacheTTLSeconds": 300,
  "authorityTimeoutMs": 1500,
  "dependencyTimeoutMs": 2500,
  "renderDeadlineMs": 5000,
  "maxMarkdownBytes": 2097152,
  "maxNavNodes": 10000,
  "sitemapUrlsPerShard": 10000,
  "publicCacheBytes": 268435456,
  "previewTokenTTLSeconds": 120,
  "etagAlgorithm": "sha256(epoch, documentId, publicationRevision, locale, themeRevision)",
  "rootRedirectStatus": 302,
  "renamedRouteRedirectStatus": 301,
  "htmlCacheControl": "no-cache",
  "staticHashedCacheControl": "public, max-age=31536000, immutable",
  "rootLocalePriority": [
    "explicit-language-cookie",
    "Accept-Language地区变体匹配",
    "base-language"
  ],
  "rootVary": "Accept-Language, Cookie",
  "homepage": "只有一屏全屏hero，不追加目录或文章内容",
  "heroButtonsMaximum": 3,
  "heroDecorativeText": "Welcome To",
  "markdownURL": "/<locale>/<hierarchicalSlug>.md",
  "llmsURLs": [
    "/llms.txt",
    "/<locale>/llms.txt",
    "/<locale>/llms-full.txt"
  ],
  "headings": "标题H1，正文H2-H4；永不折叠；锚点使用稳定blockId",
  "renderInput": "content权威Tiptap JSON，而非Markdown正文",
  "firstPageWithoutAI": "正常创建/编辑/预览/发布；AI问答入口显示未启用条件，不探测渠道"
}
```

精确参数约束以types.json为准；不从客户端接受成本分类、可信actor、IP或secret。
