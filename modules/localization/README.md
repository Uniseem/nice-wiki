# M10 语言、翻译与术语

管理站点语言启停、发布内容增量翻译、人工保护、术语库、界面字符串和语言建议；翻译是绑定权威源修订的派生资产。

阶段：design。所有业务源文件仅在未来文件manifest登记，本阶段不实现、不运行测试/真实模型、不推送。

- [requirements](docs/requirements.md)
- [architecture](docs/architecture.md)
- [data](docs/data.md)
- [api](docs/api.md)
- [workflows](docs/workflows.md)
- [files](docs/files.md)
- [functions](docs/functions.md)
- [operations](docs/operations.md)
- [product-matrix](docs/product-matrix.md)
- [acceptance](docs/acceptance.md)

- [types.json](contracts/types.json)
- [implementation.json](contracts/implementation.json)
- [openapi.json](contracts/openapi.json)
- [events.json](contracts/events.json)

依赖：[根文档标准](../../docs/documentation-standard.md)、[公共类型](../../contracts/common.schema.json)、[权限](../../contracts/permissions.json)、[唯一AI任务目录](../../contracts/ai-tasks.json)、[产品保持原则](../../docs/product-preservation.md)、[AI上下文与缓存](../../docs/ai-context-and-cache.md)。

## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
