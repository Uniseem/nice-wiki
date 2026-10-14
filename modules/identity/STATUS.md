# 身份权限当前进度

更新：2026-10-15。阶段：R1-01 实施中（纯规则工作包已提交）；业务验收与部署未开始。

- 启动装配组（第一包，提交 533a1239fa8c）：cmd 入口、app 装配、transport HTTP 生命周期、9 文件；Serve/Shutdown/Health/Ready verified。
- 私有持久化（第二包，分支 codex/identity-r1-01-storage，基线 bbb1e535c2f1，实现提交 28af316bf6a）：internal/storage.go 打开 active/identity.db（modernc.org/sqlite v1.59.0，WAL/foreign_keys/busy_timeout=3000ms，单连接串行）、migrations/ 只追加 SQL 迁移（编号连续校验、事务执行、自举 schema_migrations、集外版本拒绝启动）、control/identity-journal.jsonl 决定日志（append+fsync、末行状态生效、open 条目 INTEGRITY_ERROR 隔离）；BuildApplication 接入存储三步，Ready 的 initialized/journalRecovered/activeDataEpoch 与代次比对生效，Shutdown 关闭业务库。本地与隔离 VPS 检查通过。
- 纯规则工作包（分支 codex/identity-r1-04-pure-rules，基线 ac593c57861d2，实现提交 bdcb9128d3ec）：internal/account_validator.go 实现 NormalizeAccountName（NFC、有效 UTF-8、控制字符、首尾空白、64 字符上限）；internal/credential_service.go 实现 HashPassword、VerifyPassword、ValidatePasswordChange（Argon2id v19 固定参数、PHC 编码、成本上限、常量时间核对、敏感缓冲清理）；新增 x/crypto v0.28.0 与 x/text v0.20.0 直接依赖。仅源码已实现，未完成 Go 编译/格式验证。
- 本次超管不可变纯规则（分支 codex/identity-r1-05-superadmin，基线 82c83226b120，实现提交 7a059d4e95f8）：`internal/account_validator.go` 实现 `AssertSuperadminImmutable`，超管仅允许保持 active，子账号按 `active↔disabled→deleted` 状态机迁移，权限列表不改变超管身份。条目状态为 implemented；当前环境无 Go/gofmt，验证状态仍为未运行，M02-R01 业务验收未接受。
- 0001_domain.sql 建权威表：accounts（唯一超管部分唯一索引）/credentials/account_permissions/sessions/preferences/operation_receipts；refresh 族与令牌哈希列随会话工作包按需追加迁移；平台 nw_outbox/nw_inbox 由 EnsureSchema 创建不进本域迁移。
- main 与 BuildApplication 仍 blocked（accounts 子命令、x-handler 绑定、换向回执与后台循环），缺口逐条见 [progress.json](progress.json)；ChangePassword 与其余业务文件/函数仍 planned。
- 现网行为事实：无 site-config 依赖或不可达时按合同 DEPENDENCY_UNAVAILABLE 退出；换向后回执未实现前拒绝开放入口；未完成决定日志存在时 INTEGRITY_ERROR 拒绝启动。

本次未新增测试/样例/快照/E2E/真实接口或模型调用，未 push、未部署。根 THIRD_PARTY_NOTICES.md 尚未登记 x/crypto 与 x/text，docs/structure/framework.json 尚未登记两个新源码文件；公共资料交集成会话处理。Go 工具链、gofmt、make 在当前环境不可用，Python 结构检查仅因公共 framework 未同步而失败，详见 handoff。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
