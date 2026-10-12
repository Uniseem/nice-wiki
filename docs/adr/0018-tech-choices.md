# D18. 技术选型

- 状态：✅ 已定
- 日期：2026-10-12

- Go：标准库 `net/http` 做路由；数据库访问用 sqlc；SQLite 驱动用 modernc.org/sqlite（纯 Go，不需要 CGO）。
- Go module 路径：`nice-wiki`（`server/go.mod`）。
- 前端：TypeScript、pnpm、Vite；shadcn-vue + Tailwind CSS。后台、公开页面的 Go 模板、Vue 局部组件共用一套 Tailwind 和配色。
- TypeScript 暂时固定在 6.0.x。7.x 是用 Go 重写的原生版本，要等 vue-tsc 明确支持后再升级。
