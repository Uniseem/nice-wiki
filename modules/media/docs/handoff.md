# 资源媒体会话交接

## 本次事实（R1-08 启动与契约生成准备，2026-10-15）

- 基线：`ac593c57861d2242f27d07562a2acae535d18c38`；分支：`codex/media-r1-bootstrap`；在 Codex 管理的独立 worktree 中完成。
- 独占范围：`modules/media/`。根 contracts/docs/scripts、libs、deploy 拓扑和其他模块交集成会话维护；本次没有读取 `vps.txt`、old 分支或旧业务源码。
- 已实现启动与配置文件：`cmd/media/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`go.sum`、`Makefile`、`config/service.defaults.json`、`Dockerfile`、`deploy/manifest.json`。
- `main` 只装配 `platform.ServiceSpec` 与 `lifecycle.RunService`；app 层验证本域身份、active/control/restore 目录、信任锚、签名引用、四个已登记依赖、site-config runtime 和恢复换目录；transport 层提供合同要求的 Serve/Shutdown/Health/Ready 生命周期。
- 本域私有存储、全部 OpenAPI x-handler、blob journal 恢复、换向回执、outbox/事件订阅、jobs worker 尚未实现；`BuildApplication` 在完成启动边界检查后按合同返回 `DEPENDENCY_UNAVAILABLE`，没有空 handler、伪造数据或联调成功声明。上传、变体、引用确认、平台视频、清理函数仍为 `planned`。
- `internal/gen/openapi.gen.go`、`clients.gen.go`、`oapi-codegen.yaml` 为既有登记生成物，本次未手改；生成入口仍是根 `make gen`。

## 实现、验证与验收边界

- 已实现：启动文件、生命周期函数代码和非敏感配置已写入；progress 中业务依赖未满足的 `main`/`BuildApplication` 保持 `blocked`。
- 未验证：本机没有 Go/gofmt；按仓库约定未读取 VPS 私有连接信息，因此未执行 `go mod download`、`go build`、`go vet`、gofmt、容器构建或根 `make gen`。
- 已执行或待执行的静态范围：本次仅可进行模块内 JSON 解析、合同/路径/隐私差异检查；`make check` 若不依赖 Go 可由交付前运行。格式/编译通过不等于业务验收。
- 业务验收：未执行；未新增测试、样例、快照、E2E、真实接口、真实模型调用或部署。

## 交付记录

- 交付提交：`b253a693b305e35881af46678e0403526e84818b`（启动与配置实现提交；台账修订随后提交）。本次最终未提交文件：无。
- 检查结果：模块内 JSON 解析、`git diff --cached --check`、隐私检查通过；根 `scripts/check_docs.py` 仍因公共 `framework.json` 尚未登记本模块 9 个新增文件而失败，未修改公共文件。检查阻塞：缺 Go 工具链；VPS 检查需由集成会话按仓库私有流程执行。`go.sum` 当前按与 site-config 相同的已接受依赖闭包锁定，需在有 Go 的环境用 `go mod download` 复核。

## 下一步（一项）

实现 `modules/media/internal/blob_repository.go` 与 `upload_validator.go` 的纯持久化/校验前置函数，先读对应 `docs/functions.md`、`docs/data.md` 与 types 合同；不要接 HTTP handler。完成条件是只实现合同登记的输入校验/哈希与不可变 blob 提交边界，补真实存储/journal 依赖后再把 BuildApplication 从 blocked 推进，并由集成会话在有 Go/VPS 环境执行检查。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
