# 长期记忆会话交接

## 本次事实

2026-10-15 在独立 worktree 分支 `codex/memory-persistence` 从基线 `5bd7347fc2ada5f79e48038e5cc1a68a61cdf845` 继续本域工作。提交 `cb316303da36827d34461aa3ac560bae46241ef1` 已实现私有持久化与启动恢复；未 push。

本次只修改 `modules/memory/`。未修改四份 contracts、根文件或生成目录；合同摘要仍为 `a72c9a81f8fab283299badd264a5030dfb452aaf2539203bab667381664492b9`。未新增测试/样例、真实接口或模型调用。

## 实现与阻塞

- `memory.LoadRuntime`：校验可信服务操作上下文与 30 秒本地截止，打开 `active/memory.db`、`control/control.db` 和正文对象根目录，执行 schema 版本 1 迁移，并在 control journal 存在未完成决定时 fail-closed。
- 启动校验：检查每个索引正文对象的路径、ObjectRef owner/version/SHA-256/byteSize、文件常规类型和 UTF-8；校验映射 taskId 为当前根 AI task catalog、映射引用 current 条目、预算范围、批准 command 状态；读取/初始化 `runtime_state` 的 `memoryGeneration` 与 dataEpoch。frontmatter 具体语法未在契约中定义，本次未猜测解析。
- 迁移：`migrations/001_initial.sql` 建立条目/修订/current、映射、预算、上下文快照、助手/提议/命令、inbox/outbox、对话和运行代数表；runtime 内联同版本 SQL 以便安装后不依赖源码目录。
- `memory.BuildApplication` 仍 blocked：业务存储绑定、全部 OpenAPI x-handler、激活回执、outbox/订阅、任务执行器和后台循环尚未接入；本切片不伪造进程业务 ready。
- `go.sum` 仍未生成：本机没有 Go 工具链，待 VPS 执行 `go mod tidy`/下载后登记。

## 检查与环境

- 通过：`python scripts/check_docs.py --privacy-only`、memory JSON 解析、SQLite 内存执行 `migrations/001_initial.sql`、`git diff --check`。
- 本机缺少 Go/gofmt/make/Docker；未运行 `make check`、`make gen`、Go 编译/vet/gofmt、Docker build，也未读取 `vps.txt`。业务行为、HTTP 集成和部署仍未验收。
- 全仓结构登记留给集成会话；新增实际文件尚未登记根 `docs/structure/framework.json`。

## 下一步（一项）

先在 VPS 编译复核 `cb316303da36827d34461aa3ac560bae46241ef1`，再实现 `ValidateEntry` 及条目读写：明确 frontmatter 格式后，按正文文件权威、temp/fsync/rename、manifest/current 指针、版本 CAS 和事务/outbox 合同接入；之后再接 handler、映射写读与上下文解析。继续只修改 `modules/memory/`，不 push。
