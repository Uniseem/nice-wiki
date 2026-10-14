# AI 接入当前进度

更新：2026-10-15。阶段：R1实施中；本域已完成目录校验、模型身份纯规则和私有SQLite最小持久化切片，尚未业务验收或部署。

- 已实现：`LoadRuntime` 建立 active/control/restore 目录、严格迁移和32字节主密钥引用检查；`ValidateProvider`/`ValidateModel` 完成无网络纯校验；`PutProvider`/`PutModel` 完成CAS、幂等回执、屏障检查和变更outbox事务。
- 本地提交：`4eec945320f8cf20bfc6673a8c3c8490385aca2f`（实现切片）。
- 当前仍阻塞：BuildApplication 尚未接入本域完整数据库/所有OpenAPI handler、site-config代次接线、activation回执和后台循环；真实模型、连接探测、密钥加密/轮换与费用预留/结算均未实现。
- `EmbeddingSpace` 现有合同没有独立归一化规则字段，本切片只按 provider/modelVersion/dimensions/preprocessingVersion 计算spaceId，矛盾已记录交接；不得自行扩展公共合同。
- 本机无Go、gofmt、make，未执行编译、迁移运行、静态检查或行为验收；未新增测试/样例、未调用真实接口或模型、未push。
- 下一步：先在有Go工具链的VPS完成格式/编译验证，再接入BuildApplication与全部handler；随后实现凭据加密、档位/路由和预留账本。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
