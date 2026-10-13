> 产品需求基线快照：来源 old:docs/adr/0004-editor-tiptap.md，blob 12accd46b9395136d3cd8d0e447a3a4b0190a629。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D4. 编辑器：照思源的交互，用 Tiptap 实现

- 状态：✅ 已定
- 日期：2026-10-12

- 交互参考思源笔记：块拖拽手柄、`/` 插入菜单、Markdown 快捷输入。
- 用 Tiptap 3（MIT）实现。不移植思源的编辑器代码，原因：
  - 思源采用 AGPL-3.0，用了它的代码，整个项目都必须以 AGPL 开源；
  - 它的编辑器和思源的窗口布局、后端接口耦合得很紧，没法单独拿出来用。
- 内容以 Tiptap 的 JSON 文档格式存储。

后端需要能把 JSON 转成两种形式：
- HTML，用于公开页面渲染；
- 纯文本，用于 AI 问答取材和翻译。

详见 [design/editor.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/editor.md)。
