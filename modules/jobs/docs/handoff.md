# 任务与事件会话交接

## 本次事实（R1-03 启动与契约生成准备，2026-10-15）

- 独立 worktree（Codex 管理路径）；分支：`codex/jobs-r1-03`；基线：`ac593c57861d2242f27d07562a2acae535d18c38`。
- 启动骨架实现提交：`dba434eb1e03b1a183d2f8b66adf0eff8f3e2bbd`；台账同步提交：`8745387d74c7fe7715c291ce3be429ef9b1a2d09`；未 push。
- 独占范围：`modules/jobs/`；根 AGENTS/STATUS/README、根 docs、scripts、contracts、libs、deploy 拓扑和其他模块未修改。未 push、未部署。
- 本次修改文件：`cmd/jobs/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`go.sum`、`Makefile`、`Dockerfile`、`config/defaults.json`、`deploy/manifest.json`，并更新本模块 `STATUS.md`、`progress.json` 和本交接文件。

## 本次完成与未完成

- 已实现：main 通过 `platform.ServiceSpec` 接入统一进程模板；BuildApplication 的目录、信任锚、签名引用、site-config runtime、恢复换目录前置装配；HTTP server 的 `/healthz`、`/readyz` 路由、Serve/Shutdown/Health/Ready 生命周期骨架；固定 Go module、默认配置、非 root Docker 镜像和部署探针。
- 明确阻塞：jobs 的任务租约、任务结果、事件投递/确认/重放/死信、定时、屏障、快照、私有存储/迁移、全部 OpenAPI x-handler、后台函数和恢复激活回执仍未实现。BuildApplication 不创建假 Application，按合同返回 `DEPENDENCY_UNAVAILABLE`。
- `go.sum` 目前沿用同一直接依赖图的 site-config 锁内容；本环境没有 Go 工具链，未能运行 `go mod tidy` 重新生成，已在 progress.json 标明待 VPS 核对。

## 检查与验收

- 已核对合同登记、路径范围、默认配置和部署 manifest；未改合同，未运行 `make gen`，也没有手改生成目录。
- 已通过新增配置/台账/四份合同 JSON 解析、依赖端点 13/13 一致性、`python scripts/check_docs.py --privacy-only` 和 `git diff --check`；基线到当前提交的变更路径全部在本域，生成目录未修改。完整 `python scripts/check_docs.py` 仅因根 `docs/structure/framework.json` 尚未登记本次 9 个新增文件而失败，该公共清单交集成会话维护。
- 环境没有 `go`、`gofmt`、`make`，无法执行 Go build/vet/format、模块 verify 或 Docker 构建；按仓库规则未改用本地编译冒充 VPS 证据。
- 业务行为验收、真实服务联调、模型调用、测试样例和部署验收均未执行，不能将启动骨架标为 verified/accepted。

## 下一步（一项）

实现 `modules/jobs/internal/validation.go` 的 `jobs.ValidateJobSpec`（参考级）：先读本函数合同、types.json 任务字段和 M13-R002/R009/R014；只做输入/owner/epoch/去重键规则校验，不写任务状态、不调用外部服务；完成后同步本域台账并在可用 VPS 上运行既有检查。
