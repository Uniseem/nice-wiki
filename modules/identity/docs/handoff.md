# 身份权限会话交接

## 本次事实（R1-01 私有持久化，2026-10-14）

- 分支：codex/identity-r1-01-storage（worktree `../nice-wiki-identity`）；基线 bbb1e535c2f1（main）；实现提交 28af316bf6a42a2378f30f17a2fe036380af4ff9；台账提交紧随其后。
- 独占范围：modules/identity/。公共文件仅在交付提交中同步 docs/structure/framework.json（+3 文件）、THIRD_PARTY_NOTICES.md（modernc.org/sqlite v1.59.0），请集成会话复核。
- 未提交文件：无。未新增测试/样例/真实接口调用；未 push、未部署。
- 沿用说明：migrations/migrations.go 与 0001_domain.sql 初稿来自并行子代理（因用量上限中断），本会话核对其符合 data.md/ADR0005 后沿用并补齐存储层、装配接线与台账。

## 本次完成

- internal/storage.go：identity.OpenDatabase（WAL/foreign_keys/busy_timeout=3000ms、单连接串行、control 区决定日志 0600 追加句柄）、identity.RunMigrations（migrations.All 校验编号连续，事务执行并登记 schema_migrations，集外版本 INTEGRITY_ERROR）、identity.RecoverJournal（逐行末行状态生效，open 条目隔离）；AppendDecision/CompleteDecision/HasSuperadmin/Close 为未登记参考级助手，随首个调用方交付时登记。
- BuildApplication 在 ApplyRestoreSwap 后打开业务库并执行迁移与恢复；Ready 补 initialized（唯一超管存在）/journalRecovered/activeDataEpoch（打开时代次，运行代次不一致 503）；Shutdown 在 HTTP 关闭后关闭业务库。
- 合同同步：implementation.json 增 internal/storage.go、migrations/migrations.go（函数 identity.All）与 0001_domain.sql（migration 类），BuildApplication 依赖补三个存储函数；make docs 已重生成 files.md/functions.md；progress.json contractSha256 已重算并新增 4 函数/3 文件行。

## 实现口径（供后续与集成对齐）

- 业务库 active/identity.db，决定日志 control/identity-journal.jsonl（control 区不随恢复替换）。0001 权威表按 types.json 公开 DTO 建列；sessions 的 refresh 族/令牌哈希列不在 0001，随会话工作包追加 0002（迁移只追加）。
- 平台 nw_outbox/nw_inbox 由 EnsureSchema 创建（outbox 工作包接入时），不进本域迁移。
- 换向后回执、InvalidateRestoredSessions、x-handler 绑定与后台循环仍 blocked，缺口见 progress.json；site-config 控制库（R1-02 下一包）就绪前运行权威读取等待依赖。

## 检查证据

- 本地 darwin/arm64 go1.27.1 与隔离 VPS（8核，go1.25.5，/root/build/nice-wiki-identity-persistence，tar 同步排除 .git/__pycache__/AppleDouble）：`go build ./...`、`go vet ./...`、`gofmt -l` 为空均通过；根 `make check` 均通过（829文件/894函数/372操作一致）。
- 未运行业务进程与业务验收（site-config 未实现，启动按合同 DEPENDENCY_UNAVAILABLE；无已授权测试）。

## 下一步（一项）

实现纯规则函数组（参考级优先）：NormalizeAccountName（NFC、无控制字符/首尾空白、≤64）与 credential_service.go 的 HashPassword/VerifyPassword/ValidatePasswordChange（Argon2id，x/crypto/argon2，Apache-2.0-BSD风格许可需核对登记）。先读 docs/functions.md 对应条目与产品保持矩阵；完成后登记合同/台账并在本地与 VPS 过检查。会话函数（CreateSession 等）依赖 Argon2id 与账号权威表，随后再领。
