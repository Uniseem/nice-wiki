# 公开交付当前进度

更新：2026-10-17。`BuildTOC` 已实现，尚未跑 GitLab CI。accepted 为 0。未做业务验收，未部署。

- `main`、`ResolvePublishedLink`、`BuildCanonical`、`Serve`、`Shutdown`、`Health`、`Ready` 记 verified。`BuildCanonical` 只拼接调用方传入的 `publicBaseURL` 与规范路径。
- `BuildTOC` 记 implemented，verification 为 not_run。H1 不进目录；H2–H4 按原顺序保留全文，锚点使用稳定 blockId。maxDepth 不是 4，或级别不是 1–4，返回 `INVALID_TOC_DEPTH`。重复 blockId 返回 `INTEGRITY_ERROR`。
- `BuildApplication` 在缺登记操作、journal、schema 迁移或 activation 回执时失败关闭，不返回可运行应用，保持 blocked。`RewriteInternalLinks` 仍未实现，链接文件保持 blocked。
- 依赖未知为 503。水位变化使用公共错误表 `STALE_RESULT`/409。

下一项：实现 `public-site.BuildBreadcrumbs`（`modules/public-site/internal/breadcrumbs.go`）。先读合同步骤、错误码和 types.json。不要改 `BuildApplication`，不要让它返回可运行应用。
