# 站点配置与初始化当前进度

更新：2026-10-15。阶段：R1-02 实施中；本轮完成初始化准备、状态读取与决定持久化代码，全仓编译门槛通过，根结构登记待集成会话完成。业务验收与部署未开始。

- 本轮基线 `00dd199239aba487bceb5227aac52f27ce949d74`，分支 `codex/site-config-init-r1`，独占 `modules/site-config/`。
- `PrepareDefaultConfig` 使用显式部署地址和可信 preparedAt，生成已明确默认值的完整未发布配置；`ReadInitializationStatus` 始终返回 initialized 与安全阶段，只向同站点超管或 identity/site-config 可信服务附完整 coordination。未开始的 operationId/siteId 为 null，无占位 ID。
- `PersistInitializationDecision` 在 active 私有事务中保存完整准备配置规范 JSON、SHA256、不可反悔决定、CAS 当前状态和原始结果回执；`0002_initialization.sql` 只追加迁移，SQLite synchronous=FULL。commit 仍停 preparing，后续恢复用例才可提交权威 settings 文件并完成初始化。
- 同步公共 nullable 生成修复后，本域 runtime 的恢复字段真实使用 null；读取、迁移、SQL 写入与平台 RuntimeView 适配已同步，退出维护后的回执与重新读取一致为空。
- 已有运行状态权威、active 迁移、公告 journal、语言规范化和健康/就绪代码继续保留；本域 build/vet/gofmt 已通过，全仓 14 模块 make build 也通过；make check 仅缺根 framework 的 4 文件登记，最终提交见本轮 handoff 和 progress.json。
- 未完成：InitializeSite/ResumeInitialization/AbortInitialization 完整用例、settings 权威文件提交与恢复、HTTP 业务 x-handler、outbox 循环、恢复换目录回执装配。publicBaseURL 的可信进程配置注入仍留 HTTP 工作包，不从 Host 猜测。

未新增测试、样例或临时验证程序；未运行业务进程、真实接口、模型、联调、镜像、部署或 push。业务验收保持未执行。
