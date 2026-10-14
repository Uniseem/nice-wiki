# 文档生命周期当前进度

更新：2026-10-15。阶段：R1 实施中；本域业务尚未验收、尚未部署。

- 本次 `R1-05` 基线为 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`，独立分支为 `codex/content-r1-05-recovery`；前序启动准备仍由 `f62ea3d7c52bc98ebd73d94c1428ca8794caf751` 提供。
- 本次已实现 `content.RecoverPreparedCommits`（提交 `f376a3e779cd7cffc0f1f214a99b8b6ea5090055`）：恢复只能在注入的独占启动/safe 屏障内进行；校验严格 journal、正文哈希与已有 head；有 committed receipt 的 PREPARED 才补记 COMMITTED；缺正文、哈希不符或 head 不一致的提交隔离并关闭文档可见性；函数没有推进 head 的能力。`content.WriteRevisionFile` 及 `BuildApplication` 接线仍按范围保持 planned/blocked。
- 本次实现 `content.CommitDraftJournal`：按 operationId+规范摘要先查已提交 receipt；对 PREPARED 正文拒绝符号链接/非正规文件，复核大小、SHA-256、文件同步与目录同步；校验旧head、草稿版本连续性、journal与draft一致性及站点/dataEpoch；由注入的私有事务在同一边界执行 fence检查、head CAS、稳定ID台账、COMMITTED journal、refs、receipt和outbox。未注入事务存储或发生不确定存储错误时明确返回 `DEPENDENCY_UNAVAILABLE`，不将启动伪装为可用。
- 本次源码与模块台账提交为 `448ad37`；该提交只包含 `modules/content/`。
- 状态区分：上述源码为 `implemented`；仅完成 JSON、隐私、差异空白和暂存内容静态检查，Go 编译/gofmt/行为恢复检查未运行，故未标 `verified`；业务验收为 `accepted` 之前的未执行状态。
- 已创建启动与部署准备文件：`cmd/content/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`Makefile`、`config/service.defaults.json`、`deploy/manifest.json`、`Dockerfile`；实现提交为 `f62ea3d7c52bc98ebd73d94c1428ca8794caf751`。
- `content.main`、`content.Serve`、`content.Shutdown`、`content.Health`、`content.Ready` 已写入；`content.BuildApplication` 已接入配置、信任锚、site-config运行权威和恢复换目录前置检查，但因业务存储、journal恢复、全部OpenAPI x-handler、后台函数和activation回执未实现，按合同返回 `DEPENDENCY_UNAVAILABLE`，保持 blocked。
- `go.sum` 未创建：当前环境无Go工具链，VPS不可用，不能用手写内容代替 `go mod download`。Go build、vet、gofmt、make verify、Docker构建和业务验收均未执行。
- 本地Python JSON解析与隐私检查通过；`git diff --check` 通过；根 `scripts/check_docs.py` 因公共结构清单尚未登记本域新增文件而报告未登记项。根结构清单属于集成会话所有权，本次未修改。
- 未新增测试、样例、快照、E2E、真实接口或真实模型调用；未push。

逐文件/函数事实见 [progress.json](progress.json)，交接证据见 [docs/handoff.md](docs/handoff.md)。公共资料由集成会话维护，本域只修改 `modules/content/`。
