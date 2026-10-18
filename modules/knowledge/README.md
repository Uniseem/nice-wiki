# M08 知识入库与混合检索

将content权威发布修订与media授权资源转换为版本化分块、词项和向量索引；只返回经权威可见性校验的材料。

阶段：R1实施。已有检索与索引基础链通过GitLab，继续配置、构建/提取、worker、恢复和界面；本模块仅交付源码，由集成会话统一检查及双推，不运行真实模型或部署。

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
