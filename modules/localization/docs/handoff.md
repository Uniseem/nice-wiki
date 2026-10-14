# 本地化会话交接

## 本次事实

2026-10-15 在当前集成分支 `main` 基于 `8ae6fcfc6302e2fd41a998faef18be52e2a9714c` 完成本域私有持久化与启动恢复校验切片，提交为 `cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`；未修改合同、生成物、根文件或其他模块。

已实际完成：

- `cmd/localization/main.go` 只装配 platform 进程生命周期与本域参数。
- `internal/app/wire.go` 建立数据目录、加载身份材料、装配 site-config 运行权威客户端和恢复换目录入口；因业务依赖未实现，按合同返回 `DEPENDENCY_UNAVAILABLE`。
- `internal/runtime/bootstrap.go` 打开 active SQLite 与 control journal，初始化 platform inbox/outbox，按 `001_initial.sql` 执行单调迁移，校验表结构、journal、默认语言唯一性和术语代数，并登记旧翻译租约数量；不在启动阶段重放模型或任务副作用。
- `migrations/001_initial.sql` 建立语言、翻译任务/单元、译文清单、术语、界面目录、语言建议、批准提议和缓存回答译文权威表；执行时使用本域二进制内固定迁移内容，生产镜像不依赖源码路径。
- `internal/transport/server.go` 提供健康/就绪路由、HTTP Serve/Shutdown 骨架；未接入普通业务路由、事件订阅或 worker，Store 尚未由 Application 持有。
- `go.mod`、Makefile、非敏感配置、部署 manifest、Dockerfile、Vue/Vite/TypeScript UI 构建基础和 `pnpm-lock.yaml` 已建立。
- `mountModuleUI` 仅完成本域挂载基础与重复挂载取消；未创建业务页面或 API/SSE 客户端。

## 检查与限制

- `pnpm install --lockfile-only`、`pnpm run typecheck`、`pnpm run build`：通过。
- `python scripts/check_docs.py --privacy-only`：通过。
- `git diff --check`、本域 JSON 解析和暂存内容隐私扫描通过。
- `python scripts/check_docs.py`：本次未运行完整检查；根结构登记属于集成会话范围，不能在本域会话修改。
- `make gen`：未执行成功，本机没有 make；现有 Go/TS 生成文件未手改、未变更。
- 本机无 Go、gofmt、Docker，VPS 不可用；未生成 `go.sum`，未执行 Go build/vet/gofmt、Docker 构建、业务测试、真实接口或模型调用。上一代码提交的 staged diff 检查还提示 Makefile 文件末尾有空行，代码范围未在本次台账修正中改动。

## 基线、范围与状态

- 基线：`8ae6fcfc6302e2fd41a998faef18be52e2a9714c`。
- 代码与台账提交：`cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`。
- 独占范围：`modules/localization/`；本次实际修改包含 `go.mod`、`internal/app/wire.go`、`internal/runtime/bootstrap.go`、`migrations/001_initial.sql` 和三份本域台账。
- 合同未修改，`contractSha256` 保持 `02c1a90a759bb421b0d59c1585167e2e5c352f242a3926a4f76a0b5a1082a313`。
- `localization.LoadRuntime`、`internal/runtime/bootstrap.go` 与 `migrations/001_initial.sql` 已登记为 implemented，但因本机无 Go/gofmt，验证仍为 `not_run`。
- 完整 OpenAPI handler、翻译/术语/界面业务、事件与任务循环、Store 持有/Shutdown 接线和行为验收仍未完成；progress 中未涉及的条目保持 `planned`。

## 下一步（一项）

先在具备 Go 工具链的 VPS 或集成环境编译、gofmt、vet 本次本域代码，确认 modernc SQLite 依赖和多语句迁移可用；通过后接入完整 handler，并让 `transport.Application` 持有本域 Store，在 Shutdown 中按安全点关闭。不得同时实现真实模型、测试样例或跨域存储。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
