# Excalidraw

> 我们画板块使用的组件。

| 项目 | 信息 |
|---|---|
| 方向 | 编辑器 |
| 仓库 | excalidraw/excalidraw |
| 许可证 | MIT |
| 主要语言 | TypeScript |
| Star | 134k |
| 最近更新 | 2026-10 |

## 它是什么

手绘风格的白板，以 npm 包 `@excalidraw/excalidraw` 的形式嵌入其他应用。

## 和我们相关的部分

- 是 React 组件，需要 React 作为依赖。
- Docmost 已经把它集成进 Tiptap，可以对照它的做法。

## 借鉴什么

- 集成方式参考 Docmost 的 `excalidraw.ts` 扩展；导出 SVG 用官方的导出接口。

## 注意

- 只在后台打开画板时才按需加载 React 和 Excalidraw（已在设计中）。

## 资料

- [仓库](https://github.com/excalidraw/excalidraw)
- [集成文档](https://docs.excalidraw.com/)
