# 资源媒体当前进度

## 两轮并行第一轮：ready引用预留（2026-10-16，已交付冻结）

- 基线d2161f41c441b69f080b7d8cf3d40b88afecb357，分支codex/media-next-two-r1，仅拥有modules/media/；源码提交`aab3c6edd25d0c56f677dc02ce843f7fafce5706`，本域不自行push，等待根统一集成、每轮检查与push。
- 新增既有登记reference_service.go/reference_repository.go与获准只追加0003_reference_pins.sql；Complete新ready资产同事务建立逐对象fence，旧0001/0002未改。完整来源/可信caller/epoch和规范摘要绑定，原pin不续期，空集合也落pin，sealed先于旧结果，source→asset稳定锁→store互斥→SQL CAS。
- 新pin至少24h；逐资产真实ready/站点/版本/hash/长度/MIME/唯一schema/fence失败关闭，fence与generation同事务提升。原pin完整输入/结果hash与schema、原pinId/expiry及逐资产保护可核；原结果不依当前资产重建。缺fence/负值INTEGRITY_ERROR，int64合法最大值耗尽LOCKED，不施加额外JS上限。
- 7个新增完整参考助手与迁移verified；Acquire仅ready切片保持in_progress，delete_pending未知计划LOCKED，已知有效清理计划取消仍planned。reference_service/reference_repository/transport为partial，Confirm/Release与事件、公开bytes/其余handler/控制恢复/后台未齐；完整HTTP/app仍blocked，不能称服务可运行或业务验收通过。
- 根统一VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、4源码gofmt、14现有Go模块readonly build/vet/gofmt及完整make check通过（862文件/1226函数/379操作/2272类型）。精确格式/生成快照已完整回传；本域仅本地静态差异/路径/隐私核对，无本机Go/完整check。未新增测试/mock/fixture/样例/临时验证程序，无SQL/HTTP行为验收、真实接口/模型、服务/镜像/部署。
- 公共结构请求：根登记modules/media/migrations/0003_reference_pins.sql；两个源码路径原已登记。第一轮完成冻结，不提前展开第二轮。
- 下一项：等根第一轮统一集成检查并push后，按新main基线实施Confirm/Release真实权威决定闭环；先读content:readReferenceDecision/site-config:readHomeReferenceDecision版本化合同，复用同来源/逐asset锁，不持锁回源，未知/404/pending/过期不释放，封存与释放同事务。

## 推送前复核：SVG样式文本分片（2026-10-16，已验证）

集成会话在main复核本轮时发现style按XML CharData逐片校验会漏掉由CDATA或注释拆分的CSS关键字。现改为每个style合并最终文本后一次校验，并拒绝style内嵌子元素；保持原字节与既有预算。源码提交eafa9275a31b38dc02880e6339694fa1d0f4286c，仅修改既有validateSVG、合同和台账，未加测试或样例；VPS docs/gen、14模块readonly build/vet/gofmt与完整check通过，仅为本阶段检查，业务验收未执行。

## 本轮完成上传与直接资源校验（2026-10-16，已交付冻结）

- 分支codex/media-completion-round1，基线170c1dc9057e122e349de43db532b2c48434b6cf，源码提交 `9c2521a3142db0bc2ebcf1dc2914d3aea6984b46`；独占modules/media/，公共文件由根统一处理。
- 完成CompleteUpload、SniffAssetType、ValidateAssetPayload、ValidateDrawingScene、PublishAssetEvent与相连参考助手共37个完整函数；原回执固定首次成功/安全错误，来源owner/site/创建epoch先核，同站点同kind/hash/bytes/MIME复用保留原creator且SQL主键必须与Asset.ref一致。
- SQL chunk水位与完整SHA后，才核MIME/有界容器编码声明/SVG/scene；确定拒绝blocked CAS及原安全错误同事务。blocked完整临时字节持续计入存储预留，本轮Abort不开放blocked清理，只待后续已登记临时维护明确清理，不能宣称空间释放。
- 成功PREPARED→既有upload/SHA确定stage→file fsync/rename/两目录fsync→再核运行窗口→单事务ready/session CAS/COMMITTED/原结果/新资源outbox。未知Commit留文件仍报错；stage与既有恢复识别一致，ready绝不自动删。
- 新增5个已登记源码：upload_completion.go、mime_validator.go、payload_validator.go、drawing_validator.go、outbox_repository.go；无003或其他迁移，无新外部依赖。types/OpenAPI同步blocked、DrawingScene唯一生成schema可达、内部结果尺寸与完成错误；公共3.0兼容由根维护。
- 最终隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14现有模块readonly build/vet/gofmt及完整make check通过（861文件/1216函数/379操作/2271类型）。首轮Revision类型错误修正后已完整重跑，最终格式/生成回传一致；未新增或运行业务测试、SQL/HTTP行为验收、真实接口/模型、服务/镜像/部署，无本机Go/完整check或push。
- 完整本轮函数与5个完整职责文件verified；outbox_repository尚缺RelayOutbox、transport尚缺完整handler/控制恢复/后台，均保持partial；asset_service既有ListAssets planned口径不变。BuildApplication/全量服务/pins/公开bytes/其余后台仍未完成，不称服务可独立启动或业务已验收。
- 已静态核对暂存路径、提交内容、用户代用邮箱与GPT-6署名；源码提交保留既有用户身份，无私有连接/凭据/本机用户路径或真实外部IP。未修改公共目录/其他域。
- 本轮结束后停止，不自动展开后包。后续仅在用户另行授权时，先读引用合同实施AcquireResourceReferences的本域来源锁与pin事务，依赖content/site-config来源权威读取；不以本轮已有检查代替行为验收。

## 第五轮媒体上传基础组（2026-10-16，已交付冻结）

- 源码提交 `4fcbde766790d125e44e16bedf2f19e53856392e`；分支codex/media-chunks-read-r1，基线5894bfb368bfb79331709b3e0f0e5e6ed92bf098，独占modules/media/。主线实际cherry-pick SHA由根统一映射。
- 完成PutUploadChunk、ReadAsset及获准Begin固定原回执闭环。当前授权/epoch后原成功只读首次快照；新chunk明确长度≤8MiB、精确SHA、暂存fsync/PREPARED、追加fsync后SQL offset/revision CAS与COMMITTED原结果同事务。启动按SQL水位核前缀/截尾，不把PREPARED当成功或ready。
- Begin hash绑定caller/actor/site/epoch及原输入，先回原结果再查新配额；旧NULL epoch/缺快照明确拒绝不猜。quota固定media纯服务并清原用户proof、保留bounded上下文及原用户站点；Commit结果未知保留文件但仍报错，以原回执确认。
- 复用公共NewWriteContext与ValidatePrivatePath；二次runtime epoch不同409、未知503，维护直接暂停。已有上传/暂存文件与media.db打开前后拒硬链接/链接，保留Root/fd/绝对路径身份核对。ReadAsset仅真实ready安全DTO/精确当前版本，固定纯服务caller或用户同站点权限，不泄路径、不造历史。
- 最终统一VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：docs/gen、14现有模块readonly build/vet/gofmt与完整check通过（861文件/1182函数/379操作/2269类型），无回传冲突，明确含最终修正。仅本轮完整函数/文件verified；ListAssets/Complete/pins/全量入口/控制恢复与后台仍planned/blocked。没有行为验收、联调或部署。
- 新实际3文件：internal/chunk_writer.go、internal/asset_service.go、migrations/0002_upload_chunks.sql，根统一framework登记；0001未改，所有生成来自VPS。无新库/测试/mock/fixture/临时业务验证程序，无本机Go工具、真实API/模型、服务或push。
- 用户要求本轮做完停止持续目标，由根顺序集成及实际树复核后push、汇报百分比；本域clean冻结，不自行push，不展开后包。

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
