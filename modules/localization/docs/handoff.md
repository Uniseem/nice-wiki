# 本地化会话交接

## 本次事实

2026-10-15 在独立 worktree、分支 `codex/localization-r1-startup` 基于 `ac593c57861d2242f27d07562a2acae535d18c38` 完成本域 R1 启动与契约生成准备。启动与配置实现提交为 `4263e90cef1ca5530f3b963f5f6b8cc1021da850`；本次三份台账修正随当前交付提交同步。

已实际完成：

- `cmd/localization/main.go` 只装配 platform 进程生命周期与本域参数。
- `internal/app/wire.go` 建立数据目录、加载身份材料、装配 site-config 运行权威客户端和恢复换目录入口；因业务依赖未实现，按合同返回 `DEPENDENCY_UNAVAILABLE`。
- `internal/transport/server.go` 提供健康/就绪路由、HTTP Serve/Shutdown 骨架；未接入普通业务路由、数据库、journal、outbox、事件订阅或 worker。
- `go.mod`、Makefile、非敏感配置、部署 manifest、Dockerfile、Vue/Vite/TypeScript UI 构建基础和 `pnpm-lock.yaml` 已建立。
- `mountModuleUI` 仅完成本域挂载基础与重复挂载取消；未创建业务页面或 API/SSE 客户端。

## 检查与限制

- `pnpm install --lockfile-only`、`pnpm run typecheck`、`pnpm run build`：通过。
- `python scripts/check_docs.py --privacy-only`：通过。
- `python scripts/check_docs.py`：失败，原因是本次新增的 14 个模块文件尚未登记根 `docs/structure/framework.json`；根结构文件不属于本域范围，未修改。
- `make gen`：未执行成功，本机没有 make；现有 Go/TS 生成文件未手改、未变更。
- 本机无 Go、gofmt、Docker，VPS 不可用；未生成 `go.sum`，未执行 Go build/vet/gofmt、Docker 构建、业务测试、真实接口或模型调用。上一代码提交的 staged diff 检查还提示 Makefile 文件末尾有空行，代码范围未在本次台账修正中改动。

## 基线、范围与状态

- 基线：`ac593c57861d2242f27d07562a2acae535d18c38`。
- 代码提交：`4263e90cef1ca5530f3b963f5f6b8cc1021da850`。
- 独占范围：`modules/localization/`；本次台账修正仅修改 `STATUS.md`、`progress.json`、`docs/handoff.md`。
- 合同未修改，`contractSha256` 保持 `02c1a90a759bb421b0d59c1585167e2e5c352f242a3926a4f76a0b5a1082a313`。
- 业务函数、持久化、翻译、术语、人工保护、模型调用、事件处理、任务执行和行为验收仍未完成；progress 中未涉及的条目保持 `planned`。

## 下一步（一项）

实现本域私有持久化与 journal 启动依赖：先读本域 `docs/data.md`、`docs/architecture.md`、`docs/operations.md`、`contracts/implementation.json` 中 `localization.LoadRuntime` 与 `docs/playbooks/handoff.md`，再只认领 `migrations/001_initial.sql` 与 `internal/runtime/bootstrap.go` 对应的存储恢复接线；完成条件是 schema/journal 可恢复、失败不伪造 ready，并把实际提交和检查证据同步回三份台账。不得同时实现翻译、模型或测试。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
