# 公开目录交接（2026-10-17，BuildTOC 已实现，未跑 GitLab CI）

## 范围与事实

- 分支 `codex/public-site-20261017-r6`，基线 `9a10eff89c91095670f7d336e84e7a010348adc6`，源码提交 `7cb0bb9562bc7e332e120eaaed44268a38df3b5a`。只改 `modules/public-site/`。未 push。
- `BuildTOC` 是纯函数。`ctx` 与 `op` 不参与判断。H1 不进入目录。H2–H4 按输入顺序逐项保留原文，锚点字段使用原稳定 `blockId`，不使用输入 `anchor`，也不从文本生成 slug。
- `maxDepth` 只有 4 这一已写明刻度。不是 4 时返回 `VALIDATION`，条件名 `INVALID_TOC_DEPTH`，不按更小深度截断。级别为 1 时跳过。级别为 5、6 或不在 1..6 时同样返回 `INVALID_TOC_DEPTH`。级别都合法之后，任意重复 `blockId`（包括只出现在 H1 上的重复）返回 `INTEGRITY_ERROR`。失败返回零值。
- `TOCItem` 必填字段都能从标题得到：`level` 取 `Heading.level`，`label` 取 `Heading.text`，`anchor` 取 `Heading.blockId`。函数记 implemented，blocker 为空。verification 为 not_run，证据写明未跑 GitLab CI。未记 verified 或 accepted。
- `BuildApplication` 仍 blocked，不返回可运行应用。`main`、`ResolvePublishedLink`、`BuildCanonical`、`Serve`、`Shutdown`、`Health`、`Ready` 维持 pipeline 48 的 verified。accepted 为 0。

## 检查

未跑 GitLab CI。未在本机或 SSH 执行 go build、go vet、gofmt、make build、make gen。已运行 `python3 scripts/render_docs.py`，合同未改，生成文档无差异。新文件 `modules/public-site/internal/toc.go` 尚未写入根 `docs/structure/framework.json`，本会话不改该文件；集成会话登记前，`make check` 的结构核对会报未登记文件。

## 请集成会话登记

- `modules/public-site/internal/toc.go`

## 下一步（一项）

实现 `public-site.BuildBreadcrumbs`，文件 `modules/public-site/internal/breadcrumbs.go`。先读 implementation.json 的步骤与错误码，以及 types.json 的 `BuildBreadcrumbsInput`、`BuildBreadcrumbsResult` 和 `Breadcrumb`。不要改 `BuildApplication`，不要让它返回可运行应用。未跑 GitLab CI 前不要记 verified 或 accepted。
