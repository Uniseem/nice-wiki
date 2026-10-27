# M09 Agent 问答、引用与记录

负责访客对话、主 Agent 自主查阅与逐事件流式回答，保留公开过程、引用、反馈和隔离诊断。问答记录默认保留 15 天，到期清除正文与工具历史；不提供共享答案缓存或前置模型判定。

主循环调用独立 M16 ZCode 原生服务，使用路由首选模型的默认窗口，接近上限时由原生机制压缩；旧固定窗口、轮数、工具次数设置不再约束主链路。保留嵌入与关键词混合检索，提示模型用英文关键词定位后自主读正文。最新检查点与本轮工具消息保存在问答私有记录中，跨轮重放前继续核对公开来源版本；原文历史不因压缩而改写。

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
