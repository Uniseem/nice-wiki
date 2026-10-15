# 站点配置与初始化当前进度

更新：2026-10-15。阶段：R1-02 实施中；本轮初始化完成与 settings 权威提交及配置 IO 边界修复已实现，经隔离 VPS 全仓编译/生成/完整结构检查与集成复核，已交付函数记 verified（仅对应检查范围）。业务验收与部署未开始。

- 本轮源码提交 `5b6bc4e39fd72457d302031e0f369cece55ae1c5`，配置 IO 修复 `53156c7d74fb065b1355250ab916d03518c49d0b`，格式/生成与 vet 收口 `6396715d91a56bb6a12cb4ddd151b4117ff27520`、`8613254c72d63ddf30ce3710cf222d21a7738181`；尚未 push。
- 基线 `325f56d7b2f6c5567c16d7e21f66d5a88a97e342`，分支 `codex/site-config-initialization-complete-r1`，独占 `modules/site-config/`。
- 已写 InitializeSite/ResumeInitialization/AbortInitialization：唯一超管会话与 site 作用域、稳定业务摘要、原始结果回执、不可反悔决定、blocked 后幂等补完；不调用 AI、identity owner 准备或真实接口。
- 最终检查：隔离 VPS `make docs/gen` 成功；14 个 Go 模块 `make build`（readonly build/vet/gofmt，含本域）及完整 `make check` 均通过（850 文件/943 函数/372 操作/2186 类型）。此前 IO 错误包装 vet 失败已修复并经本轮复跑通过。
- 逐函数复核追加：配置 IO 使用 os.Root 目录能力逐段 Lstat，拒绝链接/特殊文件，正规文件打开前后身份核对，正文与 journal/head 的 JSON 上限 16 MiB；该追加修复已通过本轮 VPS 门槛。
- 已写 WriteConfigRevision/CommitConfigHead/RecoverConfigJournal：schemaVersion=1 权威文件、不可变 PREPARED/COMMITTED、排他 Link、head Rename、逐级 fsync、SQLite 写锁/CAS、崩溃回放、孤儿隔离、SQL 与文件水位核对。
- `0003_config_commit.sql` 只追加 config_operations；配置水位/回执和平台 outbox 同事务，不复制公共 outbox 表或转发机制。RelayOutbox 循环仍待 HTTP/后台装配工作包。
- 可信公开地址由公共 ServiceConfig 边界校验后 ConfigureInitialization 注入，缺失地址初始化失败关闭；不从请求 Host 推导。启动恢复只根据明确 COMMITTED 决定，旧准备决定缺身份回执明确失败关闭。
- 未完成：完整 OpenAPI 业务 x-handler、其他设置用例、RelayOutbox 循环、恢复换目录回执与快照/屏障装配。BuildApplication 的本组接线不等于完整模块装配完成。

本轮未在本机执行编译、格式化、lint、类型检查、测试或生成校验；未新增测试、样例、fixture 或临时验证程序；未运行服务、真实接口、模型、联调、镜像、部署或 push。
