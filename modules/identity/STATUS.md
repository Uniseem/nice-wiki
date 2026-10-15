# 身份权限当前进度

更新：2026-10-15。阶段：R1-01 实施中（本轮补自身会话分页和账户全部会话撤销，待集成全量编译复核）；业务验收与部署未开始。

- 本轮会话撤销工作包：新增 `ReadSessions`、`WithSessionCursorKey`、`RevokeAllSessions` 和追加迁移 `0002_session_revocation.sql`；修正 `PublishIdentityEvent` 外部事务的回执归属，实际实现等待集成全量 `make build` 通过后转 implemented/verified。identity 本域 build/vet/gofmt 已通过；全量构建仅被公共 nullable 生成修复后的 content 适配编译错误阻断，结构检查仅缺根框架登记3个实际文件。未新增测试或业务验收。
- 2026-10-15 集成复核修复：运行状态适配解开 `state` 外层。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本轮并行切片已合并到当前集成分支：账号索引/`accounts` 运维子命令（`138a2b45ae08`）、注册状态与功能权限映射及账号前置校验（`484aa7fb79d5`）、幂等回执查询、身份事件入outbox、恢复后会话失效（`1a6725d3eca0`）。合并时统一了受信 `WithIdentityStore` 上下文，并拒绝覆盖损坏的账号索引文件。
- `ReadRegistrationState`、`MapFunctionGrants`、`ReadServerAccountIndex`、`WriteAccountIndex`、`ResolveIdentityOperation`、`PublishIdentityEvent`、`InvalidateRestoredSessions` 状态为 implemented；仅完成前置校验的 `RegisterFirstSuperadmin`、`CreateAccount` 与缺少 jobs 客户端接线的 `RelayOutbox` 保持 in_progress。
- `main` 的 `accounts` 子命令已解除原有 `ReadServerAccountIndex` 阻塞；`BuildApplication` 仍因完整 OpenAPI handler、恢复回执、`restoreId` 注入、`ObserveSuperadminResetFile` 和 `RelayOutbox` 后台循环而 blocked，不能提前开放完整服务。

- 启动装配组（第一包，提交 533a1239fa8c）：cmd 入口、app 装配、transport HTTP 生命周期、9 文件；Serve/Shutdown/Health/Ready verified。
- 私有持久化（第二包，分支 codex/identity-r1-01-storage，基线 bbb1e535c2f1，实现提交 28af316bf6a）：internal/storage.go 打开 active/identity.db（modernc.org/sqlite v1.59.0，WAL/foreign_keys/busy_timeout=3000ms，单连接串行）、migrations/ 只追加 SQL 迁移（编号连续校验、事务执行、自举 schema_migrations、集外版本拒绝启动）、control/identity-journal.jsonl 决定日志（append+fsync、末行状态生效、open 条目 INTEGRITY_ERROR 隔离）；BuildApplication 接入存储三步，Ready 的 initialized/journalRecovered/activeDataEpoch 与代次比对生效，Shutdown 关闭业务库。本地与隔离 VPS 检查通过。
- 纯规则工作包（分支 codex/identity-r1-04-pure-rules，基线 ac593c57861d2，实现提交 bdcb9128d3ec）：internal/account_validator.go 实现 NormalizeAccountName（NFC、有效 UTF-8、控制字符、首尾空白、64 字符上限）；internal/credential_service.go 实现 HashPassword、VerifyPassword、ValidatePasswordChange（Argon2id v19 固定参数、PHC 编码、成本上限、常量时间核对、敏感缓冲清理）；新增 x/crypto v0.28.0 与 x/text v0.20.0 直接依赖。仅源码已实现，未完成 Go 编译/格式验证。
- 本次超管不可变纯规则（分支 codex/identity-r1-05-superadmin，基线 82c83226b120，实现提交 7a059d4e95f8）：`internal/account_validator.go` 实现 `AssertSuperadminImmutable`，超管仅允许保持 active，子账号按 `active↔disabled→deleted` 状态机迁移，权限列表不改变超管身份。条目状态为 implemented；当前环境无 Go/gofmt，验证状态仍为未运行，M02-R01 业务验收未接受。
- 0001_domain.sql 建权威表：accounts（唯一超管部分唯一索引）/credentials/account_permissions/sessions/preferences/operation_receipts；refresh 族与令牌哈希列随会话工作包按需追加迁移；平台 nw_outbox/nw_inbox 由 EnsureSchema 创建不进本域迁移。
- `ChangePassword`、会话签发/刷新/单会话撤销/CSRF、账号写事务、完整 HTTP handler、超管重置 watcher、屏障/快照仍未实现；具体状态和阻塞逐条见 [progress.json](progress.json)。
- 现网行为事实：无 site-config 依赖或不可达时按合同 DEPENDENCY_UNAVAILABLE 退出；换向后回执未实现前拒绝开放入口；未完成决定日志存在时 INTEGRITY_ERROR 拒绝启动。

本次未新增测试/样例/快照/E2E/真实接口或模型调用，未 push、未部署。历史切片的“无Go工具链”记录保留为当时事实；本轮已使用本地Go完成本域编译/vet/格式检查，集成会话统一完成VPS全量复核与根框架登记，详见 handoff。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
