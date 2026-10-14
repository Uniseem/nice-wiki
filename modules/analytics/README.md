# M14 analytics

聚合匿名公开访问、管理操作审计和 model-gateway 费用引用，提供后台统计及导出；实时预算、账本余额仍由 model-gateway 决定。

状态：按old已确认产品行为重建微服务设计；未实现业务代码，未创建测试样例，未执行真实接口或模型，未推送远程。产品追踪见[保留矩阵](../../docs/product-preservation.md)。

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

文件与函数清单及数量见 [docs/files.md](docs/files.md)（由合同生成）。文件仅登记路径，没有创建未来源码。共用规则见 [根文档标准](../../docs/documentation-standard.md)、[一致性](../../docs/consistency.md)。

## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
