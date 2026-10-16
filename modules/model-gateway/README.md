# M07 统一模型网关

唯一拥有供应方、密钥、模型版本、功能档位、费用报价、额度预留、执行与费用账本。所有AI消费必须经过此服务。

阶段：R1实施。用户授权两轮三子代理并行、每轮根统一检查并普通push main；本域第一轮PutProfile及紧密辅助/真实HTTP映射已交付并通过根VPS生成/格式/编译/完整检查，源码SHA见progress.json。profile.go及wire.go整体partial、完整启动blocked；视觉/重排读取回退、凭据、模型与费用执行未完成，未做SQL行为验证或业务验收。

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
