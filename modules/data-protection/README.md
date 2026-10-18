# M15 data-protection

在全域维护安全点创建一致备份、校验并恢复统一代次；把各域已有历史投影成 data git 版本镜像并管理远程同步。

阶段：R1实施。目录/加密等既有源码及本轮快照导出适配已通过GitLab编译检查；本轮只计完整快照回执校验helper，真实回调、13域端口、短委托及完整备份/恢复协调仍blocked。业务验收与部署未开始。

- [需求与边界](docs/requirements.md)
- [架构与依赖](docs/architecture.md)
- [存储与迁移](docs/data.md)
- [私有分片加密格式](docs/crypto-format.md)
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

文件与函数清单及数量见 [docs/files.md](docs/files.md)（由合同生成）。启动入口、装配、HTTP 生命周期和部署配置已经按登记路径创建；其余业务文件仍未实现。共用规则见 [根文档标准](../../docs/documentation-standard.md)、[一致性](../../docs/consistency.md)。

## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
