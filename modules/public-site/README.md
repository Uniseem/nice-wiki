# M06 public-site

将 content 已发布版本渲染成可索引、可访问的完整 HTML，并提供导航、SEO、多语言路由、Markdown、llms、sitemap 与受保护预览。

状态：启动装配和公开链接解析已编写，进程仍失败关闭；未编译，accepted 为 0，未验收，未部署。产品追踪见[保留矩阵](../../docs/product-preservation.md)。

- [需求与边界](docs/requirements.md)
- [架构与依赖](docs/architecture.md)
- [存储与迁移](docs/data.md)
- [HTTP/SSE协议](docs/api.md)
- [流程和状态机](docs/workflows.md)
- [未来文件目录](docs/files.md)
- [函数输入输出与行为](docs/functions.md)
- [运维及恢复](docs/operations.md)
- [未来行为验收](docs/acceptance.md)

- [types](contracts/types.json)
- [implementation](contracts/implementation.json)
- [openapi](contracts/openapi.json)
- [events](contracts/events.json)

文件与函数清单及数量见 [docs/files.md](docs/files.md)（由合同生成）。本轮已创建启动入口、装配、HTTP 生命周期、规范地址和公开链接解析；其余登记路径仍未创建。共用规则见 [根文档标准](../../docs/documentation-standard.md)、[一致性](../../docs/consistency.md)。

## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。

`resolvePublishedLink` 已绑定真实 `ResolvePublishedLink`。缺处理函数或权威依赖时启动失败，不返回可运行应用。依赖未知为 503，水位变化为公共错误表 `STALE_RESULT`/409。未编译的函数保持 implemented 或 blocked，没有 verified，accepted 为 0。
