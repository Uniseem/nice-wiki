# 长期记忆会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `LoadRuntime` 失败或中断不再留下永久 open 的 journal：失败路径补记 aborted，恢复时按条目 ID 聚合，仅 `runtime.load` 这类无副作用的启动校验在中断后补记 aborted，其余未决决定仍按完整性错误拒绝。
- 删除自建 inbox/outbox 表，改由平台 `outbox/inbox.EnsureSchema` 建 nw_outbox/nw_inbox（ADR0005）；迁移改为 go:embed，新增 control 库迁移 `migrations/control/0001_control.sql`。
- 修正非常量格式串（go vet）；运行状态适配解开 `state` 外层；补齐 go.sum。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增文件/函数暂记 `in_progress`，待修复提交后回填 SHA。
- 遗留：迁移文件 `001_initial.sql` 在未部署前删除了 inbox/outbox 表定义；一旦发布即只追加。

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
