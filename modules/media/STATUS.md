# 资源媒体当前进度

## 第五轮媒体上传基础组（2026-10-16，待统一VPS）

- 分支codex/media-chunks-read-r1，基线5894bfb368bfb79331709b3e0f0e5e6ed92bf098，独占modules/media/。本轮PutUploadChunk、ReadAsset及获准BeginUpload固定原回执闭环已写源码，全部本轮改变in_progress；不提前称implemented/verified。
- BinaryChunk仅本进程io.Reader，HTTP要求明确1..8MiB Content-Length；顺序offset/8MiB/精确长度/SHA/expiry/revision，先暂存fsync+PREPARED，再追加fsync，最后SQL CAS水位+COMMITTED原结果同事务。旧成功在当前授权/epoch后只读原快照，不消费流或落新候选。
- 0002只追加上传创建epoch、分块日志及Begin原结果/hash；旧NULL epoch或旧无快照回执不猜原结果、不重建上传。启动核连续已提交前缀SHA，未提交尾只按SQL水位在真实running窗口截尾，PREPARED不自动计数或发布ready。
- ReadAsset真实读取唯一schema与索引一致的ready安全DTO，固定注册纯服务caller/用户同站点权限；指定不存在版本NOT_FOUND，不造历史或私有路径。HTTP复用公共严格入口/identity权威授权；缺Complete/其他handler仍构造失败并关库，Ready不伪造控制恢复。
- 新实际3文件为internal/chunk_writer.go、internal/asset_service.go、migrations/0002_upload_chunks.sql，均在已登记/获批路径。生成物未手改，待根VPS docs/gen/build/check；无新库、测试/mock/fixture/临时业务验证程序，无本机Go工具、真实API/模型、服务、部署或push。
- 用户最新要求本轮完成后停止持续目标，根统一复核后push；本域只收尾当前组并冻结，不展开CompleteUpload/引用pin或后包。

## 历史记录

# 资源媒体当前进度

更新：2026-10-15。阶段：R1实施中；本次完成本域 active 私有存储、blob journal 恢复和首组三个上传处理函数；业务验收与部署未开始。

- 2026-10-15 集成复核修复：存储改为 ADR0005 规定的私有 SQLite（`active/media.db`）：新增 `migrations/0001_domain.sql`（upload_sessions、upload_receipts、assets、blob_journal）与 go:embed `migrations/migrations.go`，弃用此前整份重写的 `metadata.json`/`blob-journal.jsonl`。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 基线为 `3db911390e35a07007d70a1d2f5cc15fe4092667`，分支为 `codex/media-upload-storage`，独占范围为 `modules/media/`。源码提交为 `517719d`；状态、台账和交接将在随后提交中记录该源码提交。
- （已由集成复核改为私有 SQLite，见首条）`internal/blob_repository.go` 原先落地本域 active/media 私有目录、schemaVersion=1 的原子元数据快照、临时上传文件、站点隔离的 SHA 内容寻址 blob、同站点同 SHA 复用、临时对象哈希复核、fsync/rename/目录持久化和 operation receipt。
- `internal/blob_recovery.go` 已实现启动时独占读取 blob journal：校验 PREPARED/COMMITTED/DELETING 记录，确认已提交 blob 的元数据与 SHA；未完成对象只进入 isolated，不扫描或按年龄删除 ready 文件。
- `internal/upload_service.go` 已实现 `BeginUpload`、`ReadUpload`、`AbortUpload`：读取 site-config 版本化资源配额，校验 `media.upload`/`media.manage`、规范文件名、创建24小时 receiving 会话、执行同 operationId 请求摘要幂等；不返回私有路径或句柄。
- `internal/app/wire.go` 已在启动恢复边界装配 site-config 配额客户端、本域私有存储和 blob journal 恢复。全部 OpenAPI handler、分块写入、完成校验、引用确认、outbox/订阅/worker 尚未实现，故 BuildApplication 仍按契约返回 `DEPENDENCY_UNAVAILABLE`，没有伪造 ready 或联调成功。
- `internal/upload_validator.go` 的 HashUpload 已改用线程安全的当前 blobStore 读取；生成目录未改，未新增测试、样例、真实接口、部署或 push。
- 本机没有 Go/gofmt，因此未执行 `go build`、`go vet`、gofmt、`go mod download` 或业务验收；已执行暂存差异检查、路径范围检查和隐私扫描。未读取 `vps.txt`。
- 根 `scripts/check_docs.py` 尚未执行；公共 `docs/structure/framework.json` 不在本域所有权内，集成会话需在合并时登记本轮实际文件。

本域状态以 [progress.json](progress.json) 为准；实现、验证、行为验收和部署状态分开记录。公共资料由集成会话维护。
