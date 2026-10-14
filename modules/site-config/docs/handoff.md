# 站点配置与初始化会话交接

## 本次事实（R1-02 active 业务存储与公告 journal，2026-10-15）

- 基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`；分支：`codex/site-config-r1-02-active-storage`。
- 独占范围：`modules/site-config/`。未修改根 AGENTS/STATUS/README、根 docs、scripts、根 contracts、libs、deploy 或其他模块；未读取 `vps.txt`、old 分支或旧业务源码。
- 本次只实现 active DB 迁移、公告决定 journal 恢复和 Ready schema/journal 校验；未实现 HTTP 业务路由、outbox 循环、后台任务循环或真实恢复联调。
- 实现提交：`d2a7f9100a3de824cecba9d66eeb645fdf0196e5`、`02b18cb0ed1c97949730ef1c1b6b62047710520d`；台账同步提交：`9656920`；未 push、未部署。

## 本次完成

- `internal/migrations/active/0001_active.sql`：新增 active 私有 SQLite 迁移，建立 schema_migrations、site_config_state、config_revision_index 与 announcement_heads；正文文件仍是权威数据。
- `internal/runtime_store.go`：新增 active 数据库句柄、WAL/foreign_keys/busy_timeout=3000ms 单连接、连续迁移校验、epoch/初始化状态校验及 `ValidateActiveReady`；control.db 仍独立于 active 恢复换目录。
- `internal/announcement_files.go`：实现 `RecoverAnnouncementJournal`；严格解析 JSONL，COMMITTED 逐项核对 head、对象路径、大小、hash 与公告正文 schema，孤儿 PREPARED 写入私有隔离区，不推定为已发布。
- `internal/app/wire.go`：BuildApplication 在无换目录动作时打开 active 库并完成 journal 恢复；任何迁移、epoch、对象或 journal 失败均返回错误，不创建可用应用。
- `internal/transport/server.go`：Ready 接入 active schema、epoch、journal 指纹和初始化状态；Shutdown 关闭 active 与 control 句柄。
- `contracts/implementation.json`、`docs/files.md`、`docs/functions.md`：登记 active 迁移及两个存储校验函数，并用 `python scripts/render_docs.py` 生成同步文档。

## 状态区分

- 已实现：上述 active 存储、迁移、journal 恢复、Ready 校验和启动接线代码；代码提交后以实际 SHA 回填台账。
- 已验证：JSON 解析、生成文档同步、隐私静态检查与 `git diff --check`。
- 未验证：Go 格式化、依赖 tidy、build、vet、现有普通测试和业务行为验收；当前环境没有 `go`/`gofmt`，也未读取 VPS 私有连接信息。
- 未验收：M03-R01～M03-R06、崩溃恢复、并发 revision、未知 schema/损坏对象的运行行为和部署均未执行。

## 遗留阻塞与契约决策

- BuildApplication 仍 blocked：完整 OpenAPI x-handler、换目录后的 RegisterActivation 回执、outbox/后台循环和其余业务用例尚未实现；本次没有用占位成功值掩盖这些缺口。
- active journal 的 COMMITTED 记录要求对应 `active/announcements/<announcementId>/head.json` 与不可变正文对象；缺失、hash/大小/schema 不一致会阻止启动和 Ready。孤儿 PREPARED 只隔离并计数，不会公开。
- 未初始化 active 状态返回 Ready 的 `initialized=false`、`activeDataEpoch=null`；初始化后的状态必须有匹配的 config_revision_index 与 runtime dataEpoch。
- 公共 `docs/structure/framework.json` 以及根第三方声明属于集成会话所有权；本会话只在交接报告，不顺手修改。

## 下一步（一项）

在具备 Go 工具链后，先对本提交执行 `gofmt`、`go build ./cmd/site-config` 与 `go vet ./...`，再由集成会话登记公共结构文件并复跑根 `make check`；随后才实现本域初始化/配置业务函数。
