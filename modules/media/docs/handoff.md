# 资源媒体会话交接

## 本次事实（R1-09 上传校验与不可变 blob 边界，2026-10-15）

- 基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`；分支：`codex/media-r1-09-blob`；源码提交：`2517ccb1225eda5c2ce0157e18364f25fa84084f`；台账交接提交：`5f0175b2d53d27914fdf397b391dda80f69f2ad3`。工作始终在指定 Codex worktree，未切换或修改主目录。
- 独占范围仍为 `modules/media/`；本次仅新增 `internal/upload_validator.go` 与 `internal/blob_repository.go`，没有修改根文件、公共 contracts/docs/scripts、libs、deploy 或其他模块。
- 已实现：`media.NormalizeOriginalName` 去路径组件、控制字符、CRLF 和双向控制符并拒绝非法/空/超长结果；`media.HashUpload` 使用固定 64 KiB 缓冲读取私有临时对象，校验上下文、标识、大小和读取进展后返回 SHA-256 与字节数；`media.CommitAssetBlob` 重新核对临时对象摘要，再把同站点同 SHA 的不可变提交交给最小 `blobStore` 边界并校验 ready 资源回执。
- 未接入：HTTP、后台循环、真实对象存储、本域数据库、blob journal 恢复、配额/receipt/outbox 和 `BuildApplication` 装配。未装配 `blobStore` 时按合同返回 `DEPENDENCY_UNAVAILABLE`，不声明提交成功。

## 实现、验证与验收边界

- 状态区分：上述三个合同函数和两个源码文件为 `implemented`；尚未取得 Go 编译、vet 或 gofmt 证据，progress 的 `verification.state` 保持 `not_run`；业务验收与部署保持未执行，不能标记 `accepted`。
- 已执行：模块 contracts JSON、progress JSON 解析；暂存区 `git diff --cached --check`；暂存内容路径/隐私扫描。根 `scripts/check_docs.py` 失败于公共 `docs/structure/framework.json` 未登记本次两个新增文件，该公共缺口留给集成会话。
- 未执行：本机没有 Go/gofmt；未读取 `vps.txt`，因此未执行 `go build`、`go vet`、gofmt、`go mod download`、根 `make check`、真实接口、真实对象存储、测试样例或部署。
- 最终工作树在上述台账提交后保持干净；新增与修改路径均位于 `modules/media/`。

## 下一步（一项）

由后续 media 存储工作包在同一私有包内装配 `blobStore` 的本地权威存储与 journal，并接入 `BuildApplication`；集成会话补登记公共 `framework.json` 后，再在具备 Go/VPS 工具链的环境复核编译和静态检查。

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

- 启动与配置实现提交：`b253a693b305e35881af46678e0403526e84818b`；本次台账修订提交：`88078f96c51971b3f51604751f92ef69e4c53c0b`。本次最终未提交文件：无。
- 检查结果：模块内 JSON 解析、`git diff --cached --check`、隐私检查通过；根 `scripts/check_docs.py` 仍因公共 `framework.json` 尚未登记本模块 9 个新增文件而失败，未修改公共文件。检查阻塞：缺 Go 工具链；VPS 检查需由集成会话按仓库私有流程执行。`go.sum` 当前按与 site-config 相同的已接受依赖闭包锁定，需在有 Go 的环境用 `go mod download` 复核。

## 下一步（一项）

实现 `modules/media/internal/blob_repository.go` 与 `upload_validator.go` 的纯持久化/校验前置函数，先读对应 `docs/functions.md`、`docs/data.md` 与 types 合同；不要接 HTTP handler。完成条件是只实现合同登记的输入校验/哈希与不可变 blob 提交边界，补真实存储/journal 依赖后再把 BuildApplication 从 blocked 推进，并由集成会话在有 Go/VPS 环境执行检查。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
