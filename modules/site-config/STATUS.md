# 站点配置与初始化当前进度

更新：2026-10-15。阶段：R1-02 实施中；本轮初始化完成与 settings 权威提交已实现，隔离 VPS 全仓编译门槛通过，完整结构检查与逐函数复核待集成会话收口；其后配置 IO 边界追加修复仍为 in_progress，等待 VPS 复跑。业务验收与部署未开始。

- 本轮源码提交 `5b6bc4e39fd72457d302031e0f369cece55ae1c5`；格式/生成/台账补交付随本轮收口提交，尚未 push。
- 基线 `325f56d7b2f6c5567c16d7e21f66d5a88a97e342`，分支 `codex/site-config-initialization-complete-r1`，独占 `modules/site-config/`。
- 已写 InitializeSite/ResumeInitialization/AbortInitialization：唯一超管会话与 site 作用域、稳定业务摘要、原始结果回执、不可反悔决定、blocked 后幂等补完；不调用 AI、identity owner 准备或真实接口。
- 逐函数复核追加：配置 IO 使用 os.Root 目录能力逐段 Lstat，拒绝链接/特殊文件，正规文件打开前后身份核对，正文与 journal/head 的 JSON 上限 16 MiB；该追加修复待 VPS 复跑。
- 已写 WriteConfigRevision/CommitConfigHead/RecoverConfigJournal：schemaVersion=1 权威文件、不可变 PREPARED/COMMITTED、排他 Link、head Rename、逐级 fsync、SQLite 写锁/CAS、崩溃回放、孤儿隔离、SQL 与文件水位核对。
- `0003_config_commit.sql` 只追加 config_operations；配置水位/回执和平台 outbox 同事务，不复制公共 outbox 表或转发机制。RelayOutbox 循环仍待 HTTP/后台装配工作包。
- 可信公开地址由公共 ServiceConfig 边界校验后 ConfigureInitialization 注入，缺失地址初始化失败关闭；不从请求 Host 推导。启动恢复只根据明确 COMMITTED 决定，旧准备决定缺身份回执明确失败关闭。
- 未完成：完整 OpenAPI 业务 x-handler、其他设置用例、RelayOutbox 循环、恢复换目录回执与快照/屏障装配。BuildApplication 的本组接线不等于完整模块装配完成。

检查：集成会话在隔离 VPS 执行 `make build`，14 个 Go 模块 readonly build/vet/gofmt 通过（BUILD_STATUS=0），`make docs/gen` 成功并回传格式与生成物；`make check` 仅余公共可达性与新增实际结构登记（本域 4 个），等待统一复核。当前不记 verified。

本轮未在本机执行编译、格式化、lint、类型检查、测试或生成校验；未新增测试、样例、fixture 或临时验证程序；未运行服务、真实接口、模型、联调、镜像、部署或 push。
