# 站点配置与初始化会话交接

## 本次事实（R1-02 私有运行状态控制库，2026-10-15）

- 基线：`ac593c57861d2242f27d07562a2acae535d18c38`；分支：`codex/site-config-r1-02-runtime-state`。
- 独占范围：`modules/site-config/`。未修改根 AGENTS/STATUS/README、根 docs、scripts、contracts、libs、deploy 或其他模块；未读取 `vps.txt`、old 分支或旧业务源码。
- 未新增测试、样例、快照、E2E、真实接口或模型调用；未 push、未部署。实现提交和交接台账提交待本次最终提交后填入。

## 本次完成

- `internal/runtime_store.go`：打开 `control/control.db`，设置 WAL/foreign_keys/busy_timeout=3000ms 与单连接，执行只追加迁移，首次建库稳定保存运行 epoch、running phase、revision=1 和 14 个 requiredModules；control 区不随 active 恢复换目录。
- `internal/migrations/`：加入连续编号校验与 `0001_control.sql`，表仅覆盖 runtime_state、activation_receipts、runtime_operation_receipts 和 schema_migrations，不创建 active 业务表、journal、outbox 或测试数据。
- `internal/runtime_state_service.go`：ReadRuntimeState 真实读取控制库并拒绝未知/矛盾状态。
- `internal/runtime_transition.go`：落地 RuntimePhase 迁移矩阵、revision CAS、操作幂等摘要、14 域回执齐全判定和 RegisterActivation 的服务身份/阶段/哈希/代次校验与 upsert。
- `internal/write_epoch.go`：按 control.db 的 phase 与 dataEpoch 判定业务写入门禁。
- `internal/app/wire.go`：localRuntime 真实打开并复用控制库句柄；BuildApplication 步骤 2 读取真实权威；应用关闭时释放句柄。
- `internal/transport/server.go`：Ready 使用真实活动 epoch、maintenance、publicRequestsAllowed、runtime revision；Shutdown 关闭控制库。
- `contracts/types.json`：将 requiredModules 数量约束由 15 修正为 14，并登记 RuntimeStore 内部类型；`implementation.json` 登记新增存储和迁移文件；已运行 `python scripts/render_docs.py` 同步生成文档。

## 验证与限制

- 已通过：JSON 解析、生成文档比对、链接/契约/权限/事件/隐私静态检查，以及 `python scripts/check_docs.py --privacy-only`、`git diff --check`。
- 未执行：Go 格式化、依赖 tidy、build、vet、现有普通测试和业务运行；原因是当前执行环境没有 `go`/`gofmt`，也未读取 VPS 私有连接信息或自行改用其他主机。
- 根 `python scripts/check_docs.py` 仍因公共 `docs/structure/framework.json` 未登记本次 6 个 site-config 文件而失败；集成会话必须在合并时补登记并重新运行根 `make check`。本模块不越权修改该公共文件。
- `go.sum` 按 identity 已验证的 modernc.org/sqlite v1.59.0 锁定依赖图补齐，但因本机无 Go 尚未由 `go mod tidy` 重算；集成/具备 Go 工具链的会话需核对。

实现状态与行为验收分开：本次控制库代码为已实现待 Go 验证，业务验收和部署仍为未执行；不得标记 accepted。

## 遗留阻塞与契约决策

- BuildApplication 仍 blocked：active 区业务 DB/schema、公告 journal 恢复、完整 x-handler 路由、换目录后的 RegisterActivation 调用、RecoverAnnouncementJournal/RelayOutbox 循环尚未实现。本次没有用占位成功值掩盖这些缺口。
- `requiredModules` 的原 15 与明确 14 域设计矛盾已按可登记的 ActivationReceipt 枚举和 M03-R06 修正为 14；集成会话需确认不再恢复为 15。
- 根 framework.json 与根第三方声明属于公共文件，由集成会话按合并流程登记新增文件并补充 modernc 使用范围；本会话只在交接报告，不顺手修改。

## 下一步（一项）

实现 `modules/site-config/internal` 的 active 区私有业务存储与公告决定日志恢复：先读本域 `data.md`、`workflows.md`、ADR0005 和现有 control.db/BuildApplication 口径；仅新增本模块文件，完成 active DB 迁移、journal 恢复与 Ready 的 schema/journal 校验，仍不实现 HTTP 业务路由或 outbox 循环。
