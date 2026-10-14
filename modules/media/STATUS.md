# 资源媒体当前进度

更新：2026-10-15。阶段：R1实施中；本次完成本域 active 私有存储、blob journal 恢复和首组三个上传处理函数；业务验收与部署未开始。

- 基线为 `3db911390e35a07007d70a1d2f5cc15fe4092667`，分支为 `codex/media-upload-storage`，独占范围为 `modules/media/`。源码提交为 `517719d`；状态、台账和交接将在随后提交中记录该源码提交。
- `internal/blob_repository.go` 已落地本域 active/media 私有目录、schemaVersion=1 的原子元数据快照、临时上传文件、站点隔离的 SHA 内容寻址 blob、同站点同 SHA 复用、临时对象哈希复核、fsync/rename/目录持久化和 operation receipt。
- `internal/blob_recovery.go` 已实现启动时独占读取 blob journal：校验 PREPARED/COMMITTED/DELETING 记录，确认已提交 blob 的元数据与 SHA；未完成对象只进入 isolated，不扫描或按年龄删除 ready 文件。
- `internal/upload_service.go` 已实现 `BeginUpload`、`ReadUpload`、`AbortUpload`：读取 site-config 版本化资源配额，校验 `media.upload`/`media.manage`、规范文件名、创建24小时 receiving 会话、执行同 operationId 请求摘要幂等；不返回私有路径或句柄。
- `internal/app/wire.go` 已在启动恢复边界装配 site-config 配额客户端、本域私有存储和 blob journal 恢复。全部 OpenAPI handler、分块写入、完成校验、引用确认、outbox/订阅/worker 尚未实现，故 BuildApplication 仍按契约返回 `DEPENDENCY_UNAVAILABLE`，没有伪造 ready 或联调成功。
- `internal/upload_validator.go` 的 HashUpload 已改用线程安全的当前 blobStore 读取；生成目录未改，未新增测试、样例、真实接口、部署或 push。
- 本机没有 Go/gofmt，因此未执行 `go build`、`go vet`、gofmt、`go mod download` 或业务验收；已执行暂存差异检查、路径范围检查和隐私扫描。未读取 `vps.txt`。
- 根 `scripts/check_docs.py` 尚未执行；公共 `docs/structure/framework.json` 不在本域所有权内，集成会话需在合并时登记本轮实际文件。

本域状态以 [progress.json](progress.json) 为准；实现、验证、行为验收和部署状态分开记录。公共资料由集成会话维护。
