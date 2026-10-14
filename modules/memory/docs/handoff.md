# 长期记忆会话交接

## 本次事实

2026-10-15 在独立 worktree 分支 `codex/memory-r1-13-bootstrap` 从基线 `ac593c57861d2242f27d07562a2acae535d18c38` 认领本域唯一任务：启动与契约生成准备。提交 `fa8de2243d3865be49411739ea3afc5ccb3c68b9` 已建立入口、装配、生命周期、独立 module、Makefile、非敏感默认配置、部署 manifest 与非 root Dockerfile；未 push。

本次只修改 `modules/memory/`。未修改四份 contracts，合同摘要仍为 `a72c9a81f8fab283299badd264a5030dfb452aaf2539203bab667381664492b9`；未手改 `internal/gen` 或 `ui/src/gen`。没有实现记忆条目、映射、上下文注入、助手提议/批准、数据库、迁移、事件处理、模型调用、测试或样例。

## 实现与阻塞

- `memory.main`：入口已用 `platform.ServiceSpec` 装配 `lifecycle.RunService`，整体状态因后续 `BuildApplication` 依赖未齐而 blocked。
- `memory.BuildApplication`：已装配配置模块校验、五项依赖存在性检查、active/control/restore 目录、信任锚、签名密钥、site-config 运行态客户端、`ApplyRestoreSwap`；缺少业务存储、全部 OpenAPI x-handler、恢复激活回执、outbox/订阅、任务执行器和后台循环时按合同返回 `DEPENDENCY_UNAVAILABLE`，状态 blocked。
- `memory.Serve`、`memory.Shutdown`、`memory.Health`、`memory.Ready`：生命周期与匿名健康/就绪路由已建立，当前为 implemented、未验证。Ready 不报告业务 journal 已恢复，也不开放业务请求。
- `go.mod`、Makefile、`deploy/service.config.json`、`deploy/manifest.json`、Dockerfile：已建立，未完成构建验证。`go.sum` 未创建，因为本机没有 Go 工具链，不能伪造生成文件。

## 检查与环境

- 通过：`python scripts/check_docs.py --privacy-only`、memory JSON 解析、`git diff --check`。
- `python scripts/check_docs.py` 失败原因仅为根公共 `docs/structure/framework.json` 未登记本次 8 个新文件；按并行规则留给集成会话处理。
- 本机缺少 Go、gofmt、make、Docker；因此未运行 `make check`、`make gen`、Go 编译/vet/gofmt、Docker build，也未读 `vps.txt`。没有真实接口/模型调用，没有新增测试/样例。

## 下一步（一项）

实现 memory 私有持久化与启动恢复：先读本域 `docs/data.md`、`docs/workflows.md`、`memory.LoadRuntime` 的 `functions.md` 合同及 `migrations/001_initial.sql` 登记，建立正文 Markdown、索引、journal、CAS、dataEpoch 和恢复安全点；完成后再接入 handler、outbox、订阅与 worker。需在独立分支继续，只修改 `modules/memory/`，并按新会话流程先核对 HEAD/脏文件。
