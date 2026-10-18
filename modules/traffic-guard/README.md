# M12 traffic-guard

在请求进入高成本或高并发路径前执行频率、并发、挑战及封禁策略，只发准入凭证；model-gateway 仍是唯一实时额度和钱账权威。

状态：2026-10-18集中源码两批已交付26个既有主要函数，全部待根GitLab检查；既有登录保护和存储证据保留。完整服务仍失败关闭，accepted为0。

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

文件与函数清单及数量见 [docs/files.md](docs/files.md)（由合同生成）。实际源码、待编译和剩余缺口以本域STATUS/progress/handoff为准。共用规则见 [根文档标准](../../docs/documentation-standard.md)、[一致性](../../docs/consistency.md)。

## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
