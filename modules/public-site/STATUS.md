# 公开交付当前进度

更新：2026-10-17。启动装配和公开链接解析已由 pipeline 48 编译。accepted 为 0。未做业务验收，未部署。

- `main`、`ResolvePublishedLink`、`BuildCanonical`、`Serve`、`Shutdown`、`Health`、`Ready` 记 verified。`BuildCanonical` 只拼接调用方传入的 `publicBaseURL` 与规范路径。
- `BuildApplication` 在缺登记操作、journal、schema 迁移或 activation 回执时失败关闭，不返回可运行应用，保持 blocked。`RewriteInternalLinks` 仍未实现，链接文件保持 blocked。
- 依赖未知为 503。水位变化使用公共错误表 `STALE_RESULT`/409。生成规格嵌入已随 pipeline 47 的补丁回传。

下一项：补齐 journal、schema 迁移和其余处理函数后，再让 `BuildApplication` 返回可运行应用。
