# 本地化当前进度

更新：2026-10-15。阶段：实现准备；本域翻译、术语、人工保护、模型调用、任务和事件业务尚未完成、尚未验收、尚未部署。

- 基线：`ac593c57861d2242f27d07562a2acae535d18c38`；独立分支：`codex/localization-r1-startup`；启动与配置代码提交：`4263e90cef1ca5530f3b963f5f6b8cc1021da850`。
- 已完成登记的启动与契约生成准备：`cmd/localization` 入口、`BuildApplication`/HTTP 生命周期骨架、独立 `go.mod`、Makefile、非敏感配置、manifest、Dockerfile，以及 Vue/Vite/TypeScript UI 构建基础和 pnpm 锁文件。
- `BuildApplication` 对尚未存在的本域 handler、持久化、journal、outbox、事件订阅和任务循环依赖按合同返回 `DEPENDENCY_UNAVAILABLE`，未写伪实现；普通业务路由、翻译和模型调用未接入。
- UI `pnpm run typecheck`、`pnpm run build` 通过；未进行浏览器行为验收。隐私检查通过，完整文档检查仍因根 `docs/structure/framework.json` 未登记本次新文件而失败；根结构文件按范围未修改。
- 本机没有 Go、gofmt、make、Docker 或可用 VPS，因此未生成 `go.sum`，未执行 `make gen`、Go 编译/vet/gofmt、Docker 构建或业务测试。生成的 Go/TS 文件未手改、未变更。
- 本次没有新增测试、样例、真实接口调用或模型调用；部署状态仍为 `not_started`。逐文件、逐函数事实见 [progress.json](progress.json)。

下一步：先按 [交接](docs/handoff.md) 实现本域私有持久化与 journal 启动依赖，再接入完整路由和后台循环；业务验收仍需单独授权。

并行工作只改本域，公共资料交集成会话；根文件、其他模块和生成目录由集成会话维护。
