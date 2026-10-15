# M13 jobs

提供持久任务、带版本事件、租约、重试、进度与定时触发，协调安全点屏障；领域算法由所属服务通过 HTTP 执行。

状态：R1持续实施第四轮任务管理查询/安全人工重试源码与合同已准备，待VPS统一检查。前两轮任务与durable事件核心已通过对应检查；完整启动、真实owner适配、control屏障/后台/定时及ephemeral投影仍未完成，业务验收和部署未开始。产品追踪见[保留矩阵](../../docs/product-preservation.md)。

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
