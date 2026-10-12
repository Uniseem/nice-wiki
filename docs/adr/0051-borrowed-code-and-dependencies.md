# D51. 借用代码与依赖的选择

- 状态：✅ 已定
- 日期：2026-10-12

依据见 [reference/README.md](../reference/README.md) 的"其他"部分。

- **编辑器扩展借用 Docmost 的代码**：同为 AGPL-3.0、Tiptap 版本一致（3.31）。文件开头注明出处，根目录 `THIRD_PARTY_NOTICES.md` 汇总。节点视图改写成 Vue，写法参考 Halo（不借用其代码）。
- **公开站点的挂载方式、Vite 配合方式**对照 Gitea。
- **模型协议适配器**参考 New API 对各家协议细节的处理，自己写精简版。
- **后台任务队列直接使用 Backlite**（用户选择）。合并重复任务、显示进度在外面包一层实现；它的表由它自己创建。
- **直接使用的库**：`golang.org/x/text/language`、maxminddb-golang、go-limiter、browser-image-compression；fracdex 的 Go 版和 JS 版（CC0）复制进项目。
- **检索员增加"查看文档目录树"的工具**（参考 PageIndex）。
- **HTML 页面的响应头加 `Link` 指向 Markdown 版本**（llms.txt 规范）。
- 借用外部代码的规则写进工程规范：只借用许可证兼容的代码；注明出处、登记到 `THIRD_PARTY_NOTICES.md`；借用新项目前先询问用户。
