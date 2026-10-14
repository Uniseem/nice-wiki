# 资源媒体当前进度

更新：2026-10-15。阶段：R1-09 上传校验与不可变 blob 边界已实现；业务验收与部署未开始。

- 本次基线为 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`，独立分支为 `codex/media-r1-09-blob`，独占 `modules/media/`；启动实现沿用 `b253a693b305e35881af46678e0403526e84818b`，本次源码提交为 `2517ccb1225eda5c2ce0157e18364f25fa84084f`，台账交接提交为 `5f0175b2d53d27914fdf397b391dda80f69f2ad3`。
- 已建立 `cmd/media` 入口、app/transport 生命周期骨架、Go module/锁、Makefile、默认配置、Dockerfile 与部署 manifest。入口只调用平台生命周期模板，不放上传、变体、引用、平台视频或清理业务伪实现。
- `internal/upload_validator.go` 已实现展示文件名规范化和固定 64 KiB 流式 SHA-256；`internal/blob_repository.go` 已实现 SHA/媒体类型/站点边界校验，并通过最小私有存储接口表达同站点同 SHA、fsync/rename、journal 与 ready 回执边界。存储未装配时明确返回 `DEPENDENCY_UNAVAILABLE`，没有伪造成功。
- BuildApplication 已接入 module/目录、信任锚、签名引用、site-config 运行权威与恢复换目录边界；本域存储、OpenAPI x-handler、blob journal、outbox/订阅、jobs worker 未实现时明确返回 `DEPENDENCY_UNAVAILABLE`。
- `internal/gen` 三个登记文件未手改；既有生成物仍由根 `make gen` 管理。实现/验证/行为验收状态以 [progress.json](progress.json) 为准。
- 本机未提供 Go/gofmt，且按仓库约定未读取 `vps.txt`；因此 Go 编译、vet、gofmt 与 `go mod download` 尚未执行。JSON/合同/隐私静态检查在交付前运行并如实记录。
- 根 `scripts/check_docs.py` 因公共 `docs/structure/framework.json` 尚未登记本次两个新增文件而失败；公共文件不在本模块所有权内，已记录到交接，不在本次修改。

并行工作只改本域，公共资料交集成会话；根状态由集成会话在合并后汇总。
