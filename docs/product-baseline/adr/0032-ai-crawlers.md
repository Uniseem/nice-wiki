> 产品需求基线快照：来源 old:docs/adr/0032-ai-crawlers.md，blob 6e12e7af355174a8a6e0dcbf8a01c51c59309dd9。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D32. 欢迎 AI 爬虫，提供专门的版本

- 状态：✅ 已定
- 日期：2026-10-12

- 每个页面都有 Markdown 版本（地址后面加 `.md`）。
- 提供 `llms.txt` 和 `llms-full.txt`。
- `robots.txt` 明确允许 AI 爬虫。

因此 Go 端要多一个 Tiptap JSON → Markdown 的渲染器。
