# M08 知识入库与混合检索

将content权威发布修订转换为版本化分块、词项和向量索引；只返回经权威可见性校验的材料。

运行实现是 Rust 的 [search](../../rust/search)：它沿用 knowledge 的服务身份、端口与接口（config/knowledge.json），数据在 PostgreSQL 的 search 模式。本目录只留合同、文档和后台前端，不再有 Go 代码。资源（图片、PDF、视频）的文字提取暂未移植。

- [requirements](docs/requirements.md)
- [architecture](docs/architecture.md)
- [data](docs/data.md)
- [api](docs/api.md)
- [workflows](docs/workflows.md)
- [files](docs/files.md)
- [operations](docs/operations.md)
- [product-matrix](docs/product-matrix.md)
- [acceptance](docs/acceptance.md)

- [types.json](contracts/types.json)
- [implementation.json](contracts/implementation.json)
- [openapi.json](contracts/openapi.json)
- [events.json](contracts/events.json)

依赖：[根文档标准](../../docs/documentation-standard.md)、[公共类型](../../contracts/common.schema.json)、[权限](../../contracts/permissions.json)、[唯一AI任务目录](../../contracts/ai-tasks.json)、[产品保持原则](../../docs/product-preservation.md)、[AI上下文与缓存](../../docs/ai-context-and-cache.md)。

## 进度与新会话交接

进度见根 [STATUS.md](../../STATUS.md)；有规格不代表已有实现，以代码为准。
