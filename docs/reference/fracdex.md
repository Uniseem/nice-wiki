# fracdex（分数索引的 Go 实现）

> 文档树同级排序要用的"分数索引"的 Go 实现。

| 项目 | 信息 |
|---|---|
| 方向 | 编辑器 |
| 仓库 | rocicorp/fracdex |
| 许可证 | CC0-1.0 |
| 主要语言 | Go |
| Star | 48 |
| 最近更新 | 2024-12 |

## 它是什么

基于 David Greenspan 的《Implementing Fractional Indexing》，用于实时协作等场景下的排序。JavaScript 版是 `rocicorp/fractional-indexing`（同为 CC0）。

## 和我们相关的部分

- 在两个排序键之间生成一个新键，插入或移动元素时只改它自己的键。

## 借鉴什么

- 文档树排序（structure/README.md 的小决定第 6 条）直接使用，前后端分别用 Go 版和 JS 版，算法一致。

## 注意

- star 不多，但算法简单、许可证是公有领域，可以直接复制进项目。

## 资料

- [Go 实现](https://github.com/rocicorp/fracdex)
- [JavaScript 实现](https://github.com/rocicorp/fractional-indexing)
- [算法原文](https://observablehq.com/@dgreensp/implementing-fractional-indexing)
