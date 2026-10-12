# Docmost

> **最重要的参考**：开源的 Confluence/Notion 替代品，基于 Tiptap，扩展清单和我们的块类型高度重合，而且同为 AGPL-3.0。

| 项目 | 信息 |
|---|---|
| 方向 | 编辑器 |
| 仓库 | docmost/docmost |
| 许可证 | AGPL-3.0（企业版功能另行授权） |
| 主要语言 | TypeScript |
| Star | 21.9k |
| 最近更新 | 2026-10 |

## 它是什么

开源的协作 wiki 和文档系统，编辑器基于 Tiptap，前端是 React。

## 和我们相关的部分

- `packages/editor-ext/src/lib/` 下的 Tiptap 扩展包括：提示框（callout）、多栏（columns）、公式（math）、Excalidraw、draw.io、视频平台嵌入（embed-provider、embed）、视频、附件、图片、自定义代码块、唯一 ID（unique-id）、折叠块（details）、标签页、脚注、Markdown 转换等。
- README 列出的功能包括实时协作，以及 Draw.io、Excalidraw、Mermaid 图表。

## 借鉴什么

- **我们的块类型几乎都能在这里找到成熟的实现**：提示框、多栏、公式、Excalidraw、视频嵌入的提供方设计、附件。实现时逐个对照。
- 这些扩展是纯 Tiptap 的定义，和 React 无关；节点视图部分要改写成 Vue。
- **许可证同为 AGPL-3.0**，可以直接借用这些扩展的代码，只需保留版权声明并在借用处注明出处。

## 注意

- 企业版功能的文件另行授权，借用前先确认文件不属于企业版。

## 资料

- [仓库](https://github.com/docmost/docmost)
- [编辑器扩展目录](https://github.com/docmost/docmost/tree/main/packages/editor-ext/src/lib)
