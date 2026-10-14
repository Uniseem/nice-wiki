# 身份权限当前进度

更新：2026-10-14。阶段：R1-01 实施中（第二工作包已交付）；业务验收与部署未开始。

- 启动装配组（第一包，提交 533a1239fa8c）：cmd 入口、app 装配、transport HTTP 生命周期、9 文件；Serve/Shutdown/Health/Ready verified。
- 私有持久化（第二包，分支 codex/identity-r1-01-storage，基线 bbb1e535c2f1，实现提交 28af316bf6a）：internal/storage.go 打开 active/identity.db（modernc.org/sqlite v1.59.0，WAL/foreign_keys/busy_timeout=3000ms，单连接串行）、migrations/ 只追加 SQL 迁移（编号连续校验、事务执行、自举 schema_migrations、集外版本拒绝启动）、control/identity-journal.jsonl 决定日志（append+fsync、末行状态生效、open 条目 INTEGRITY_ERROR 隔离）；BuildApplication 接入存储三步，Ready 的 initialized/journalRecovered/activeDataEpoch 与代次比对生效，Shutdown 关闭业务库。本地与隔离 VPS 检查通过。
- 0001_domain.sql 建权威表：accounts（唯一超管部分唯一索引）/credentials/account_permissions/sessions/preferences/operation_receipts；refresh 族与令牌哈希列随会话工作包按需追加迁移；平台 nw_outbox/nw_inbox 由 EnsureSchema 创建不进本域迁移。
- main 与 BuildApplication 仍 blocked（accounts 子命令、x-handler 绑定、换向回执与后台循环），缺口逐条见 [progress.json](progress.json)；其余业务文件/函数 planned。
- 现网行为事实：无 site-config 依赖或不可达时按合同 DEPENDENCY_UNAVAILABLE 退出；换向后回执未实现前拒绝开放入口；未完成决定日志存在时 INTEGRITY_ERROR 拒绝启动。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
