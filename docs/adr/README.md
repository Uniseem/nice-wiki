# 新架构决策索引

| 编号 | 决策 |
|---|---|
| 0001 | [从零重建与合同优先](0001-rebuild.md) |
| 0002 | [事件统一拉取投递与两级函数合同](0002-event-delivery-and-contract-levels.md) |
| 0003 | [恢复改为暂存后重启换目录（已废弃）](0003-restore-by-restart.md) |
| 0004 | [流程接线完整性、服务身份与入口准入](0004-wiring-and-service-identity.md) |
| 0005 | [工程选型、平台公共库与单机部署](0005-engineering-and-deployment.md) |
| 0006 | [实现尺度：边界一次校验，内部信任输入，默认不写测试](0006-implementation-scope.md) |

旧决策随old归档，不自动当作新架构依据。新决策必须写状态、来源、选择、代价与影响。


2026-10-27 起新架构设计（[architecture-v2](../architecture-v2.md)）删除了跨域屏障、快照、换目录恢复协议和 data-protection 服务：ADR0003 作废，ADR0002、0004、0005、0006 里涉及它们的部分同样作废，其余仍有效。