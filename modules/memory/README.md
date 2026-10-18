# M11 长期记忆、任务映射与记忆助手

维护人工可审计记忆条目、任务映射、语言范围、版本化注入预算和只产生提议的记忆助手；批准后才应用记忆或术语变更。

阶段：R1 实施中。人工条目、映射、预算、上下文与助手部分链路已有源码；本轮尚待统一 GitLab CI，完整启动、行为验收和部署未完成。

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
