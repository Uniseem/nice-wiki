# 文档生命周期会话交接

## 本次事实（R1-04 启动与契约生成准备，2026-10-15）

- 基线：`ac593c57861d2242f27d07562a2acae535d18c38`（main）。分支：`codex/content-r1-04`，独立worktree。实现提交：`f62ea3d7c52bc98ebd73d94c1428ca8794caf751`；本交接与台账提交随后记录。
- 独占范围：`modules/content/`。未修改根AGENTS/STATUS/README、根docs、scripts、contracts、libs、deploy或其他模块；未提交文件：无（台账提交完成后核对）。未push、未部署。
- 已创建8个合同已登记文件：`cmd/content/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`Makefile`、`config/service.defaults.json`、`deploy/manifest.json`、`Dockerfile`。生成目录 `internal/gen` 与 `web/src/gen` 未手改。

## 本次实现

- `content.main` 只装配 `platform.ServiceSpec`、`lifecycle.RunService` 和本域 `BuildApplication`/`NewRuntimeClient`，不放文档业务算法。
- `content.BuildApplication` 校验本域服务身份、数据目录和依赖；建立 `active/control/restore` 三区；读取信任锚与服务签名密钥引用；经签名的生成客户端读取 site-config runtime；调用平台 `ReadRuntime` 与 `ApplyRestoreSwap`。换目录后所需activation回执、业务存储/journal恢复、全部OpenAPI x-handler及后台任务尚未实现，因此显式返回 `DEPENDENCY_UNAVAILABLE`，不构造伪Application、不伪造联调成功。
- `content.Serve`、`content.Shutdown`、`content.Health`、`content.Ready` 已按启动蓝图写入：单HTTP监听、匿名`/healthz`与`/readyz`、`ReadHeaderTimeout=5s`、`MaxHeaderBytes=64KiB`、ctx取消、draining/sync.Once和epoch/维护门禁字段。当前Application构造器只保留状态路由，业务依赖未就绪时Ready拒绝；BuildApplication不会返回它。
- `go.mod` 固定Go 1.25.0、kin-openapi v0.133.0、oapi-codegen/runtime v1.1.2和本地platform replace。默认配置只含保留地址、端口、目录及secret引用；manifest配置8103、独占data卷、服务依赖和探针；Dockerfile使用多阶段构建及非root用户；Makefile提供generate/build/run/verify。

## 检查与范围

- 通过：Python解析 `config/service.defaults.json` 与 `deploy/manifest.json`（2/2）；`python scripts/check_docs.py --privacy-only`；暂存差异 `git diff --cached --check`；提交前检查了作者/提交者与敏感字段，未发现真实凭据、个人邮箱、IP或本机路径。
- 未运行：`go mod download`、`go build ./...`、`go vet ./...`、`gofmt -l`、`make verify`、Docker build。原因是当前环境没有Go/gofmt/make/Docker，VPS不可用；不得以本地替代编译结论。
- 根 `python scripts/check_docs.py` 未通过，报告本域8个新文件未登记到根公共结构清单。该清单是集成会话所有权，按本任务禁止修改；集成时需登记后重跑根检查。
- 未新增测试、样例、快照、E2E；未调用真实接口、真实模型或部署。实现、静态检查和业务验收分别记录，业务验收仍为未执行。

## 阻塞与依赖

- `go.sum` 必须由可用Go工具链运行 `go mod download` 生成；本次未手写或复制其他模块锁文件。
- `content.BuildApplication` 仍阻塞于本域业务存储、正文journal恢复、全部OpenAPI x-handler、`RecoverPreparedCommits`/`ExpireIdleLocks`/`RelayOutbox`后台接线及恢复后的site-config activation回执；依赖模块identity、site-config、media、jobs未完成联调时不能假定成功。
- 本次未变更公共契约、生成器、根结构台账或许可清单；若依赖版本/接口需要变化，交给集成会话按公共来源处理。

## 下一步（一项）

实现 `content.RecoverPreparedCommits`（`modules/content/internal/revision_files.go`）：先读本域 `docs/functions.md`、`docs/data.md`、`docs/workflows.md` 与 `implementation.json` 对应条款，落地启动/safe屏障内PREPARED/COMMITTED正文journal恢复、缺失正文隔离和不可擅自推进head规则；完成后再把它接入BuildApplication并按实际提交同步台账。不得添加测试或伪造缺失依赖的成功结果。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
