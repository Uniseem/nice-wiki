# 2026-10-17一轮：人工删除执行与worker边界（GitLab既有检查通过，已交付冻结）

- 基线e0f307f8c99f7d0a60443d9d81590bdfa6c572db，分支codex/media-20261017-r1，独占modules/media/；源码`3d64fbcb71bd955d431c6496ee1d27ea5014ec43`，准确台账另提交。用户授权本轮三个子代理并行编写并由根统一普通推送；本域不push，交付后冻结，主线实际集成SHA由根汇总。
- 完整23函数通过本阶段检查：Execute、RunWorker、有限reader、恢复/Complete/事件/引用投影7项及16参考助手（含3项既有助手补登记）。新增既有登记cleanup_worker.go/worker.go/reader_lease.go及根批准只追加0007；blob_repository/reference_reader/cleanup_worker/reader_lease/blob_recovery/0007完整文件verified，worker/outbox整体仍partial，原cleanup_service/reference_repository/transport及完整启动缺项不冒充完成。accepted为零。
- Execute精确绑定原批准ObjectRef、jobId/op/root/site/epoch、原AcceptedJob及固定waiting，保留批准后24小时窗口。最终两源完整分页及原历史风险、pending/readers/generation/全部marks重核后，整计划DELETING和共享bytes决定同事务；任一新引用/历史风险变化阻断整份计划，未知Commit不unlink。任一决定后Cancel均CONFLICT，避免部分不可逆删除误报可逆。
- 原决定恢复在完整资产锁/store下先核全部原材料及唯一精确DELETING，再做generated blob身份/SHA/长度检查、unlink/目录fsync；deleted/精确marks解除/真实generation outbox/计划终局及固定原结果同事务。Complete同store互斥，未完成同SHA决定阻止提交/复用，计划外任一未删除共享asset保留bytes。启动只恢复同opened epoch明确批准决定，不扫描ready自动授权；历史COMMITTED的deleted资产核原终局证明。
- worker只装配platform.RunWorkerLoop，完整ObjectRef/WorkerID/fence/CheckLease及准确租约到期由根公共机制提供；领取时新建十分钟执行期限，保留原root/站点，使用登记background费用类别。执行器首次批准读取及Execute错误统一走真实取消证明：ctx取消/公共CANCELLED cause后一秒独立只读同快照，原完整终局优先成功，无execution才安全JobCancelled；未终局DELETING或读取未知不声称安全取消。证明不RPC、不写入。
- reader权威为有限持久租约及Release回调，未来bytes必须以其期限断流并close释放；当前bytes入口仍planned。content的pending_pin转未confirmed且未知expiry仍保护，矛盾retained=false与未知kind失败关闭；两源服务未就绪不假造空集合或联调成功。真实JobsClient适配、其余HTTP/事件/控制恢复与完整服务仍blocked/planned。
- 最终依据为自部署GitLab CI pipeline13候选`d2e2d55ed9f8c0797037d55551f10840f0bd43f7`：golang:1.25.5-bookworm，生成/格式及重复docs/gen一致，三个同时分片覆盖14个Go模块readonly build/vet/gofmt，完整make check通过（878文件/1435函数/379HTTP/2328类型）。根精确回传格式/生成快照，三候选交叉复审通过。本地仅静态JSON/文件/差异/范围/隐私核对，无本机Go或完整check；用户最新指令已覆盖原VPS执行规则。
- 失败如实保留：p11生成/格式/重复gen及AI分片通过，但content缺RelayOutbox→jobs:publishEvent依赖导致生成器裁掉客户端，基础check另发现CI身份环境顺序、ClaimedJob类型与实际文件/CI结构登记缺项，整体失败。均修正后p12全通过；p12尚未含首次批准读取取消的最后窄修，p13包含该窄修再次全通过，不把p11或p12当最终树证据。
- 未新增测试/fixture/mock/样例或临时业务验证程序，未运行SQL/HTTP行为验收、真实接口/模型、worker/服务、业务删除、应用镜像或部署。源码与准确记录分开提交，保持既有GitHub代用身份及GPT-6合著署名；无私有连接、配置、凭据或用户数据，所有改动仅本域。生成clients由根CI产出，未手改。
- 下一项：本轮根统一集成检查和普通push后停止；下一轮新授权再处理引用事件ApplyReferenceEvent，先读其函数/事件/类型合同并补齐空集合所需顶层完整来源对象与操作材料，再复用reference_heads/reference_entries实现幂等，不另建引用权威。

# 持续目标第二轮：人工批准与jobs延迟任务桥接（2026-10-16，已交付冻结）

- 基线471c036082e082763d90e1080b3489d4e4531380，分支codex/media-goal-r2，仅拥有modules/media/；源码`dc9e9f8540892a4747b8e3accbb15158a770e561`。根统一VPS/集成/本轮push，本域不自行VPS或push；源码与准确SHA记录分开提交，交付后冻结等待下一轮派发。
- ConfirmManualCleanup完成真实人工批准：原plan/hash/revision/历史明确批准，两源完整当前/历史/pending/水位精确再扫描；完整asset稳定锁及SQL内核原水位、无DELETING，批准/confirmed/至少24h notBefore/delete_pending/精确marks同事务。原扫描不可覆盖，原blocked部分标记与整份取消语义保留。
- 只追加根批准0006_cleanup_approvals.sql；schemaVersion=1批准对象、固定jobs ObjectRef/dedupe/notBefore、原AcceptedJob和原waiting结果独立持久。jobs真实HTTP正文不含dataEpoch，提交op固定原批准代次/op/root并清用户委托。未知只恢复原批准与同键任务，不重置24h；取消竞态迟到成功只留真实回执，不复活计划或返回未完成确认成功。
- 提交前检查以独立只读SQL事务固定计划/marks/assets快照，显式Rollback释放后才RPC；所有RPC不持asset/store锁或读事务。ReadCleanupApprovalObject保留同域reference函数，返回真实原规范UTF8/精确ObjectRef；取消后证据可读，但不授权执行。13项完整新函数verified，修改的3项既有完整助手同步本源码SHA；blob_repository与0006完整范围verified，cleanup_service/transport仍partial，完整应用blocked。ExecuteManualCleanup/RunWorker保持planned。
- 2026-10-16持续目标第二轮media最终冻结候选：根统一隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、4个改动Go gofmt、14模块-mod=readonly build/vet/gofmt与完整make check通过（874文件/1355函数/379HTTP/2308类型）；精确gen openapi及4份生成文档回传，其他域跟踪漂移0。首次docs/gen及14模块build/vet/gofmt通过，但make check唯一失败为多余service对象接口没有真实HTTP调用方；根按同进程worker真实架构裁定收缩为同域reference读取，删HTTP/DTO/装配后最终完整重跑通过，不沿用失败树结论。未新增或运行业务测试/SQL/HTTP行为验收、真实接口/模型、服务/镜像/部署，无本机Go/完整check；verified仅为阶段既有门槛。
- 已核实际暂存源码、中文提交信息、GitHub代用作者/提交者与GPT-6合著署名；只修改本域，无私有连接、配置、凭据、本机用户路径或真实IP。准确记录提交后本工作区clean；主线实际集成SHA由根汇总，不把源码SHA冒充实际合入SHA。
- 下一项：等根本轮顺序集成/实际树复检/push后，在下一轮派发中实施完整ExecuteManualCleanup与紧密worker边界；先读其合同、CleanupApproval、jobs租约与删除journal，按当前waiting/原任务回执/批准/notBefore/精确marks/最终源检查闭环，不在本轮补半截不可逆删除。

# 持续目标第一轮：真实来源快照与人工计划取消前置（2026-10-16，已交付冻结）

- 新授权持续目标已取代旧“两轮后停止”。基线685db4f48bb359f79a7501098204ccc5a4cde6b2，分支codex/media-goal-r1，独占modules/media/；源码`9647d143173e336dee465e7c5163a4b1d95d96b3`。本域不自行VPS或push，由根统一集成、审核及本轮push；交付后冻结等待下一轮派发。
- ReadAssetReferences/ValidateCleanupSources完整：固定content/site-config生成客户端，无锁RPC、清原proof/用户委托、保留epoch/root/deadline；逐页唯一schema、cursor前进及固定watermark，精确asset与完整来源。去重含sourceVersion，合法不同版本保留，精确相同draft/published投影可合并；本地pending核原pin即使到期仍保护，回源后重核generation，未知绝不展示未引用。
- ProposeManualCleanup/CancelManualCleanup及14参考助手完整（含currentBlobStore/hasGrant两个既有助手补登记），只追加0005；计划原扫描与原提案结果/hash独立持久，revision从0起，公共上限9007199254740991耗尽拒绝。提案不标delete_pending、不自动确认；单计划扫描总材料16MiB，超出明确分批；notBefore提案值只展示，未来确认须重设至少24h。
- Acquire/Confirm完整合同已验证：相交计划完整资产在锁前扩展，assetId稳定锁→store→SQL重核计划代次/修订/集合→整份取消与新pin/confirmed同事务。confirmed/waiting要求全部精确marks；blocked允许部分本计划marks，逐项只撤销真实本计划标记，其余项核ready或另一有效计划精确标记；未知delete_pending、缺fence与任何DELETING失败关闭。逐对象fence/generation仍为原int64边界，不能借计划公共上限收紧。
- 最终统一VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14现有Go模块readonly build/vet/gofmt及完整make check通过（872文件/1327函数/379操作/2297类型）；11路径精确格式/生成已回传，其他模块零生成漂移。4主用例/14参考助手、Acquire/Confirm与完整职责文件按范围verified；cleanup_service/reference_repository/transport整体partial，完整应用blocked。
- 失败记录：首次docs/gen成功，但冗余同名ReferenceSnapshot组件alias令4处生成DTO未定义，编译失败；删除多余alias后，第二次build/vet/gofmt通过而check因2旧助手未登记失败。补实际参考合同/内部blobStore与台账，并按全局映射修LOCKED409后，最终同树完整重跑通过，不沿用失败或中间结论。
- 本地仅静态JSON、文件/差异/契约/暂存及隐私核对，无本机Go/gofmt/编译/测试或完整check；无新增测试/fixture/mock/样例/临时业务程序，无SQL/HTTP验收、真实接口/模型、服务/镜像/部署。源snapshot服务仍须其域真实实现，本轮不称联调成功或服务可用。用户GitHub代用身份与GPT-6合著署名已核。
- 未完成：ConfirmManualCleanup、jobs延迟授权/执行删除、列表、引用事件与其余HTTP/控制恢复/后台仍按planned/blocked保留。业务验收/部署未执行。
- 下一项：等根统一集成/检查/push后，在下一轮派发中实施本域cleanup_service.go的ConfirmManualCleanup；先读本域确认合同与jobs:createJob，按原扫描/修订/精确marks及≥24h窗口建立真实人工批准与任务原回执桥接，未知不得产生删除授权，不展开实际删除。

# 资源媒体当前进度

## 两轮并行第二轮：源决定确认/封存（2026-10-16，已交付冻结）

- 基线53acce862467dbe40b829d1984519d78ccac5608，分支codex/media-next-two-r2，独占modules/media/；源码`89138f68209598b736eb9e93ad0d9717ac22fdbd`。首轮main已由根检查并push，本域本轮不自行push，由根统一集成复核与第二轮push；本轮是最后一轮，交付后停止，不展开第三轮。
- 新增reference_decision_client.go与只追加0004_reference_resolution.sql；生成Content/SiteConfig客户端固定回源，共享ReferenceSourceDecision唯一schema进入本域OpenAPI组件，本域8字段struct仅解码投影。纯media服务op清原proof/用户委托，保留原epoch/root/deadline，逐字段核完整来源材料；404/pending/未知/到期均不能释放。
- 源决定RPC→重新runtime窗口RPC均无业务锁；随后sourceOperation→sourceObject→旧current+新pin asset升序→store→SQL。Confirm重读来源与旧head/current逐项/完整计数及原confirmed回执，较高版本推进current并保留旧history，低版只补自身history，空集合推进head，重复原confirmed不增generation；相同水位异源、sealed/COMMITTED矛盾失败关闭。
- Release只同材料明确aborted/rejected后，以同锁/CAS序同事务sealed/原reason/终局结果hash，pending保护退出并提升原pin资产generation；无pin也封存，不以null、404或当前asset状态证明无pin，已确认current/history不释放，异材料/reason冲突。原pin_assets和原Acquire回执永久保留，合法重复Acquire仍可核原成功，sealed始终拒迟到Acquire。
- 新增10个完整参考助手、Release、0004及源客户端文件verified；Acquire/Confirm的真实delete_pending计划取消未实现，两者partial/in_progress；reference_service/reference_repository/transport为partial，其余handler/控制恢复/后台未齐，完整应用blocked。未执行引用链跨域业务验收，不能声称服务可运行。
- 首次VPS生成成功但gen.ReferenceSourceDecision被oapi-codegen裁剪，编译失败；现改公共唯一schema严格验证后的本域投影，未扩大生成器/造HTTP端点/手改gen。修正后根统一VPS linux/amd64 Go1.25.5、GOMAXPROCS=8完整重跑docs/gen、6源码gofmt、14现有Go模块readonly build/vet/gofmt和make check通过（867文件/1268函数/379操作/2278类型），完整49文件格式/生成快照已回传，不沿用失败结论。
- 本域仅本地静态JSON/文件/差异/路径/隐私检查，无本机Go/完整check。无新增测试/mock/fixture/样例/临时业务程序，未运行SQL/HTTP行为验收、真实接口/模型、服务/镜像/部署。公共结构由根登记两个新路径，源码和记录提交保留用户GitHub代用身份与GPT-6署名。
- 下一项：本轮统一检查与push结束后停止；后续新授权先补ApplyReferenceEvent空集合/完整来源机器合同（当前缺顶层object/sourceOperation材料），再实现事件幂等并复用reference_heads/reference_entries，不另建影子引用权威，不自动展开后包。

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
