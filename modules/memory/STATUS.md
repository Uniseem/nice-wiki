# 长期记忆当前进度

更新：2026-10-15。阶段：R1 实现中；本域私有持久化与启动恢复已实现，业务 handler、行为验收和部署仍未完成。

- 本次基线为 `5bd7347fc2ada5f79e48038e5cc1a68a61cdf845`，分支为 `codex/memory-persistence`。
- 已提交 `cb316303da36827d34461aa3ac560bae46241ef1`：实现 `memory.LoadRuntime`、active/control SQLite 打开与初始迁移、`runtime_state` 代数/dataEpoch、control journal 追加与恢复检查、正文对象引用/大小/UTF-8/文件摘要校验、任务映射白名单与条目引用校验、任务预算校验、批准命令状态恢复校验；新增 `modernc.org/sqlite v1.59.0` 依赖。未 push。
- `migrations/001_initial.sql` 建立条目索引、不可变修订、current 指针、任务映射/预算、上下文快照、助手/提议/命令、收发箱、对话、运行代数及迁移记录；正文仍以 `objects/<entryId>/<version>.md` 为权威文件，未臆造 frontmatter 语法。
- `BuildApplication` 仍按合同返回 `DEPENDENCY_UNAVAILABLE`：尚未接入本域存储、OpenAPI x-handler、激活回执、outbox/订阅、任务执行器和后台循环；本切片未伪造服务 ready。
- `memory.ValidateEntry`、条目写读、映射写读、上下文选择/渲染、助手提议/批准和恢复控制函数仍为 planned；`memory.LoadRuntime` 已标记 implemented 但未编译验证。
- `go.sum` 尚未生成：本机没有 Go 工具链，不能伪造生成文件，待 VPS 执行 `go mod tidy`/下载后登记。

检查记录：

- `python scripts/check_docs.py --privacy-only`：通过。
- `migrations/001_initial.sql` SQLite 内存脚本解析：通过；memory JSON 解析、`git diff --check`：通过。
- 本机未找到 Go、gofmt、make、Docker；未运行 `make check`、`make gen`、Go 编译/vet、镜像或业务测试。未调用真实接口/模型，未新增测试/样例。
- 全仓结构登记仍由集成会话统一维护；本次未修改根文件、公共契约或生成目录。

下一项：在 VPS 编译复核该切片后，接入 `ValidateEntry` 与条目读写的 Markdown 原子提交/CAS；frontmatter 具体语法必须先由契约明确，不能在实现中猜测。
