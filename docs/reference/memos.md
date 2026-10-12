# Memos

> Go 后端 + 前端打包进程序 + SQLite 的轻量自托管应用，部署方式和我们一样。

| 项目 | 信息 |
|---|---|
| 方向 | 整体架构 |
| 仓库 | usememos/memos |
| 许可证 | MIT |
| 主要语言 | Go |
| Star | 63.7k |
| 最近更新 | 2026-10 |

## 它是什么

自托管的笔记应用。

## 和我们相关的部分

- 前端构建产物由 `server/frontend/frontend.go` 嵌入 Go 程序。
- 数据库支持 SQLite、MySQL、PostgreSQL（`store/db/`）。

## 借鉴什么

- 参考"前端产物嵌入 Go 程序"的写法，以及 SQLite 存储层的组织方式。

## 资料

- [仓库](https://github.com/usememos/memos)
