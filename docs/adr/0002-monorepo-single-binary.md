# D2. 仓库形态：单仓，部署为单个程序

- 状态：✅ 已定
- 日期：2026-10-12

前端放 `web/`，后端放 `server/`，同一个 git 仓库。前端构建产物直接输出到 `server/internal/webui/dist/`，由 `go:embed` 打进程序。最终部署只需要一个可执行文件加一个 `data/` 目录。

仓库布局见 [design/architecture.md](../design/architecture.md)。
