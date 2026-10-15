# 资源媒体会话交接

## 第五轮上传分块与资源读取（2026-10-16，已交付冻结）

- 源码 `4fcbde766790d125e44e16bedf2f19e53856392e`；codex/media-chunks-read-r1，基线5894bfb368bfb79331709b3e0f0e5e6ed92bf098，独占modules/media/；主线实际SHA由根统一映射。原site-config分支保留。
- PutUploadChunk固定actor/site/epoch/upload/offset/length/sha/revision摘要，不含reader或候选时钟。当前授权/epoch后先回原成功，不消费流或重查offset/CAS/配额。新候选：明确1..8MiB Content-Length、length+1精确读/SHA → 私有暂存fsync → PREPARED → 追加/fsync → 再核running窗口/expiry → SQL CAS水位+COMMITTED原结果/hash。公共硬body预算413优先，预算内长度协议错误400。
- RecoverUploadChunks仅按SQL已提交水位核连续前缀与每段SHA或截未确认尾；短文件/坏回执失败关闭，PREPARED/未知临时材料不自动计数或发布。旧NULL创建epoch上传保留不可续传，不推测归属。0002只追加创建epoch、分块journal与Begin原结果/hash，0001未动。
- Begin raw业务hash绑定caller/actor/site/epoch与严格输入，原结果先于名称策略/配额；新session、创建epoch和首次安全快照同事务。旧缺原快照不从mutable session重建。Commit尝试前保留候选文件，未知仍报错、同op读原回执确认。quota从bounded op派生固定media纯服务并清PortalProof，不传用户session/grants；deadline/epoch/root/costClass保留，响应siteId必须是原用户站点。
- 真实写窗口只允许running非维护，WriteWindow第二次runtime必须仍为打开epoch，未知503/不同409。复用公共ValidatePrivatePath于文件/DB打开前后拒硬链接/符号链接/特殊对象，同时保留Root/fd/绝对路径SameFile。未复制公共nlink/身份/期限规则。
- ReadAsset按唯一schema和索引一致性仅返回ready安全元数据，明确版本只匹配真实当前ref.version；无历史NOT_FOUND。纯服务固定content/site-config/public-site/localization/knowledge，用户当前权限和同站点；不返回私有路径或发布授权。
- 真实HTTP绑定Begin/Read/Abort/Chunk/ReadAsset及health/ready，统一NewServiceHandler/NewIdentityAuthorizer。缺其他OpenAPI handler严格失败并关库；完整Build/NewApplication/Ready仍blocked，不能称服务已可启动。Complete/pins/ListAssets/控制恢复及后台保持原planned/blocked。
- 最终完整VPS linux/amd64 Go1.25.5 GOMAXPROCS=8：docs/gen、14现有模块readonly build/vet/gofmt、完整check通过（861文件/1182函数/379操作/2269类型），0回传冲突；含hardlink/二次epoch/quota/Begin未知Commit与原回执/name登记修正。本轮完整函数/文件按范围verified，没有SQL/HTTP行为验收、联调或部署。
- 新实际3文件：internal/chunk_writer.go、internal/asset_service.go、migrations/0002_upload_chunks.sql，根统一登记；所有gen/docs来自VPS。无新库/测试/mock/fixture/样例/临时业务验证程序，无本机Go/gofmt/gen/vet/类型检查/测试，无真实接口/模型、服务或push。
- 自动审批曾拒绝批量区间删除旧助手；完整定义/HEAD/全域引用/接口装配只读核对后，独立精确删除两段获准，未影响其他处理器，已闭环。

### 停止范围

本域交付后保持clean冻结，不push、不再展开Complete/pins或其他后包。按用户最新指令，根完成顺序集成/实际树复核后统一push并报告百分比，然后停止持续目标；后续继续须新的用户请求。

## 历史记录

# 资源媒体会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- 存储改为 ADR0005 规定的私有 SQLite（`active/media.db`）：新增 `migrations/0001_domain.sql`（upload_sessions、upload_receipts、assets、blob_journal）与 go:embed `migrations/migrations.go`，弃用此前整份重写的 `metadata.json`/`blob-journal.jsonl`。
- 提交顺序改为“PREPARED 日志单独提交 → 写不可变文件 → 单事务写资源元数据+会话完成+COMMITTED”，消除此前 COMMITTED 先于元数据落盘、崩溃后恢复必报 INTEGRITY_ERROR 的窗口；`RecoverBlobJournal` 改读 blob_journal 表。
- 修正 `internal/app/wire.go` 的错误导入路径；`RecoverBlobJournalInput/Result` 改在本域声明（生成器只产出 OpenAPI 引用的类型）；运行状态适配解开 `state` 外层；平台 nw_outbox/nw_inbox 在同库 EnsureSchema。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增的迁移包、迁移文件与函数已回填修复提交 `978e0cee659dc342650193c628710ff75115add5` 并按上述检查证据转为 `verified`。
- 遗留：分块写入、完成校验、引用与 HTTP 路由仍未实现，BuildApplication 仍按合同返回 DEPENDENCY_UNAVAILABLE。

## 本次事实（媒体私有存储与上传处理，2026-10-15）

- 基线：`3db911390e35a07007d70a1d2f5cc15fe4092667`；分支：`codex/media-upload-storage`；独占范围：`modules/media/`；源码提交：`517719d`。状态、progress 和本交接将在后续台账提交中落地；不 push。
- 本次只修改本域源码和本域三份记录：`internal/app/wire.go`、`internal/blob_repository.go`、`internal/blob_recovery.go`、`internal/upload_service.go`、`internal/upload_validator.go`、`STATUS.md`、`progress.json`、`docs/handoff.md`。未修改根文件、公共 contracts/docs/scripts、生成物、libs、deploy 或其他模块；本次没有未提交代码文件，台账提交前记录文件除外。
- `localBlobStore` 使用 `active/media` 独占目录：元数据以 schemaVersion=1 原子写入，上传临时对象按服务生成 ID 保存，ready blob 按 siteId/SHA 内容寻址；提交前复核临时对象 SHA/长度，并以 journal + fsync + 同文件系统 rename + 目录同步保护 ready 元数据边界。
- `BeginUpload`、`ReadUpload`、`AbortUpload` 已按本域函数签名落地。BeginUpload 从 site-config 读取 `resourceLimits`，校验站点、权限、文件名、种类、大小和24小时 receiving 会话；operationId 与规范摘要写入本域 receipt。同 operationId 同摘要返回已有会话，不同摘要返回 CONFLICT。Read/Abort 不返回路径或句柄，Abort 不触碰 ready 资源。
- `RecoverBlobJournal` 已接入启动：COMMITTED 记录必须同时有 ready 元数据和 SHA 正确 blob；只有未完成 PREPARED 对象进入 isolated；DELETING 不在启动阶段擅自补删，ready 文件不按年龄扫描。
- `BuildApplication` 已调用 site-config 配额客户端装配、私有存储打开和 journal 恢复，但仍因全部 OpenAPI handler、分块写入、完成校验、registerActivation、outbox/订阅/worker 未实现而返回 `DEPENDENCY_UNAVAILABLE`；没有宣称进程 ready 或真实联调成功。

## 实现、验证与验收边界

- progress 中本次五个源码文件及 `media.BeginUpload`、`media.ReadUpload`、`media.AbortUpload`、`media.CommitAssetBlob`、`media.RecoverBlobJournal` 标为 `implemented`；`media.BuildApplication` 仍为 `blocked`。没有标记 `verified` 或 `accepted`。
- 已执行：`git diff --cached --check`、暂存路径范围核对、暂存内容隐私扫描。结果通过，暂存路径全部位于 `modules/media/`。
- 未执行：本机没有 Go/gofmt，未执行 `go build`、`go vet`、gofmt、`go mod download`、根 `make check`、真实 HTTP、真实对象存储、测试样例、模型调用或部署；未读取 `vps.txt`。根 `docs/structure/framework.json` 尚未登记本次新增源码，公共文件由集成会话补登记。
- 业务验收仍未执行：M05-R01～R07、越权/并发/崩溃恢复/epoch恢复/未知字段与 schema 迁移均不能视为通过。

## 明确决策与遗留

- （已被集成复核取代：按 ADR0005 改为私有 SQLite）原决策：本次不引入第三方存储或 SQLite 依赖，使用本域私有文件元数据和 journal。
- 本次不实现 chunk writer、complete upload、MIME/scene/payload 校验、reference pin/confirm、HTTP 全量路由、outbox、订阅和 jobs worker；不以 `DEPENDENCY_UNAVAILABLE` 占位函数冒充实现。
- `BuildApplication` 仍必须在缺少全部约束级 handler 时失败，这是契约要求；集成会话不得把已有启动边界检查当作服务可用。

## 下一步（一项）

实现 `modules/media/internal/chunk_writer.go` 的 `media.PutUploadChunk`：先读本域 `docs/functions.md` 中该函数、`docs/data.md` 上传状态机和 `contracts/types.json` 的 `BinaryChunk`/`PutUploadChunk*`，复用本次 `localBlobStore` 的 upload mutex、receipt 和 active epoch 边界；完成顺序 offset、8 MiB 上限、chunk SHA、fsync 与 revision CAS 后再更新 progress/STATUS/handoff，并在具备 Go 工具链的环境复核编译。不要同时实现 complete、引用确认或 HTTP 全量路由。

## 历史交接

上一项 R1-09 已实现 `NormalizeOriginalName`、`HashUpload` 和 `CommitAssetBlob` 的校验/不可变 blob 边界，源码提交 `2517ccb1225eda5c2ce0157e18364f25fa84084f`、台账交接提交 `5f0175b2d53d27914fdf397b391dda80f69f2ad3`。其下一步是装配本地权威存储与 journal；本次已完成该项并扩展到首组三个上传函数。

更早的启动与契约生成准备已在 `b253a693b305e35881af46678e0403526e84818b` 完成；生成目录仍由根 `make gen` 管理，本次未手改生成物。


## 第四轮首页跨域只读协议

acquire顶层显式site/object/revision/version/operation/digest且refs允许空；统一CanonicalJSON摘要，重放不续期；COMMITTED旧版本只补history，当前水位不倒退；源未知/404/pending/到期均不能release。三个引用生产方函数仍planned，无实际业务实现或数据迁移。


第四轮恢复分支补充：Release以完整来源材料和可空pinId调用。媒体只在回源明确同材料aborted/rejected后，按源互斥及对象稳定锁序，同事务封存来源与释放pending pin；本地无pin也持久封存，晚到Acquire不得再落pin。网络404/未知不是无pin证明，源已提交/current/history均不释放。此协议仍planned，不宣称媒体业务实现。修订后的契约、生成物与实际集成树已在VPS通过docs/gen、14现有模块readonly build/vet/gofmt与完整check（858/1105/379/2253），生成回传无冲突；未执行业务验收。

创建上传在SQL Commit尝试前即保留候选临时文件；Commit错误仍报错但不删除可能已落库会话对应文件，由同op读取持久原回执确认。未知文件不自动计数成功，也不在本包扩GC。

本轮私有文件安全收口复用公共lifecycle.ValidatePrivatePath：既有上传/暂存普通文件打开前后均拒硬链接，并核Root、fd和绝对路径SameFile；media.db打开前后同一公共校验，未知失败关闭，不复制规则。
