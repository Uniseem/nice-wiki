# 公开交付当前进度

更新：2026-10-17。启动装配和公开链接解析已编写，阶段为 implementation。accepted 为 0。未编译，不能记 verified。

- 分支 `codex/public-site-20261017-r5`，基线 `a3e6af980d5288e42bc8d023410faf8923ff21e8`，源码提交 `1a126fd06bdbebe7c02e0306ea3b99e635a19b3a`。只改 `modules/public-site/`。
- `main` 把本域装配交给 `platform.RunService`。`ResolvePublishedLink` 已绑定真实函数。`BuildCanonical` 只拼接调用方传入的 `publicBaseURL` 与规范路径。
- `BuildApplication` 在缺登记操作、journal、schema 迁移或 activation 回执时失败关闭，不安装部分路由，不返回可运行应用。`RewriteInternalLinks` 仍未实现。
- 依赖未知为 503。水位变化使用公共错误表 `STALE_RESULT`/409。`go.sum` 未生成。
- 本轮没有跑 go、gofmt、make build、make gen 或 SSH。`make docs` 已按合同重生成函数与接口清单。`make check` 只失败在六个新文件未进 `docs/structure/framework.json`，以及 `go.mod` 缺少 `go.sum`。这两项留给集成会话，不在本域手改根清单或手写校验和。

下一项：请集成会话登记本轮新文件，并在 CI 上生成 `go.sum`、刷新规格嵌入后再编译。
