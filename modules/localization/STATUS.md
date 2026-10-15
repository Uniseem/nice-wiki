# 本地化当前进度

更新：2026-10-15。阶段：R1-15 实施中；本域启动私有持久化与恢复校验已落地，翻译、术语、人工保护、模型调用、任务和事件业务尚未完成、尚未验收、尚未部署。

- 2026-10-15 集成复核修复：迁移改为 `migrations/migrations.go` go:embed 读取 `001_initial.sql`，删除内联 SQL 副本；执行器支持多编号迁移并校验名称。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 基线：`8ae6fcfc6302e2fd41a998faef18be52e2a9714c`；当前分支：`main`；本次提交：`cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`。
- 已完成登记的启动与契约生成准备：`cmd/localization` 入口、`BuildApplication`/HTTP 生命周期骨架、独立 `go.mod`、Makefile、非敏感配置、manifest、Dockerfile，以及 Vue/Vite/TypeScript UI 构建基础和 pnpm 锁文件。
- 本次新增本域私有持久化启动切片：active 区 `localization.db` 使用 SQLite WAL/外键/忙等待，control 区追加 `localization-journal.jsonl`；按固定迁移集建立语言、翻译任务/单元、译文清单、术语、界面字典、语言建议、已批准提议和缓存回答译文表，并初始化平台 inbox/outbox。
- `LoadRuntime` 已接入 `BuildApplication`：迁移版本与表结构校验、journal 损坏/未完成决定隔离、默认语言至多一个、术语代数读取及旧租约登记均在启动阶段执行；任一步失败都关闭资源并返回已登记错误码。业务 handler、事件订阅、outbox 转发和任务循环缺失时仍按合同返回 `DEPENDENCY_UNAVAILABLE`，普通业务路由、翻译和模型调用未接入。
- UI `pnpm run typecheck`、`pnpm run build` 通过；未进行浏览器行为验收。`python scripts/check_docs.py --privacy-only` 通过；完整文档检查未运行，根 `docs/structure/framework.json` 的本次新文件登记由集成会话处理。
- 本机没有 Go、gofmt、make、Docker 或可用 VPS，因此未生成 `go.sum`，未执行 `make gen`、Go 编译/vet/gofmt、Docker 构建或业务测试。生成的 Go/TS 文件未手改、未变更。
- 本次没有新增测试、样例、真实接口调用或模型调用；部署状态仍为 `not_started`。逐文件、逐函数事实见 [progress.json](progress.json)。

下一步：接入完整 OpenAPI handler、持有并在 Shutdown 中关闭本域 Store，再按合同接入事件订阅、outbox 转发和任务循环；业务验收仍需单独授权。

并行工作只改本域，公共资料交集成会话；根文件、其他模块和生成目录由集成会话维护。
