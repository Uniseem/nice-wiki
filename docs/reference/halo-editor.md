# Halo 的编辑器

> 国内流行的建站系统，自带一个 Vue 版的 Tiptap 编辑器。

| 项目 | 信息 |
|---|---|
| 方向 | 编辑器 |
| 仓库 | halo-dev/halo |
| 许可证 | GPL-3.0 |
| 主要语言 | Java（编辑器是 Vue + TypeScript） |
| Star | 39.9k |
| 最近更新 | 2026-10 |

## 它是什么

Halo 是 Java 写的建站系统，它的富文本编辑器在 `ui/packages/editor/`，基于 Vue 和 Tiptap。

## 和我们相关的部分

- `ui/packages/editor/src/extensions/` 下有多栏（columns）、视频、音频、iframe、图库、上传、表格、任务列表、代码块、折叠块等扩展，以及命令菜单。

## 借鉴什么

- **Vue 版节点视图的写法**：Docmost 是 React，Halo 是 Vue，两者对照着看，正好覆盖"扩展定义 + Vue 节点视图"。
- 命令菜单（对应我们的 `/` 菜单）的实现。

## 注意

- GPL-3.0 的代码可以和 AGPL-3.0 的代码组合使用（两个许可证的第 13 条），但借用代码前仍要确认具体文件的许可证。

## 资料

- [仓库](https://github.com/halo-dev/halo)
- [编辑器扩展目录](https://github.com/halo-dev/halo/tree/main/ui/packages/editor/src/extensions)
