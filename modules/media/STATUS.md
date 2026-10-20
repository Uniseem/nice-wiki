# 当前交付状态（2026-10-21，B1 v2，未跑CI）

- 基线 `a53fe59cd1ef11ad51be4508788ec13132784417`，分支 `codex/media-b1-v2-20261021`，独占 `modules/media/`。本轮纳入 `1127e16e2ead5dcdab7df31a98ca5d3f131daf33` 源码修复与 `c29a68e3878676d9d2f9895491aa4788618a8e5a` media-only 交接记录；最终源码交付提交 `5bf2afd19ed5f6c32d777c36c726225512a7253b`。
- media 事件合同已升级为 `content.resources.changed@2`，events/OpenAPI 指向 content `ResourceReferencesChangedPayloadV2`；严格映射保留 `pending_pin` 原材料，eventId只从jobs信封注入。v1不静默解码。消费实现登记在已存在的 `worker.go`，不修改根结构文件。
- content 合同 `911dbb29a56f8c26f286c73ddd4f8ad082a9d92d` 已只读核对；content 源码生产迁移、根 jobs v2 producer/subscription登记和生成物更新仍是集成前置。
- 本域静态检查完成：`make docs`、四份合同JSON解析、`git diff --check`、隐私扫描。`python3 scripts/check_docs.py` 当前剩余3项预期跨域错误（两条 content v2 JSON pointer、一条 v2 producer未合并）；未运行Go/pnpm/build/vet/gofmt/make check或CI，不能记verified/accepted。

# 2026-10-20：确认来源事件幂等回放修复（未跑CI）

本轮从 `main=6204dfc9` 独立接续，仅修改 `modules/media/`。源码、media 实现合同与生成函数文档已提交：`1127e16e2ead5dcdab7df31a98ca5d3f131daf33`。

- `media.ApplyReferenceEvent` 现在核对已 `confirmed` 的 `sourceOperationId` 与事件的规范完整引用集合、来源字段和 `referenceDigest`；同材料回放按现有对象 head 水位补齐对应 `reference_entries` 与必要的 `reference_heads`。
- 回放不重复提升 `reference_generation`；低版本只补为 `history`，不覆盖新 head；同版本异来源、异材料或异 digest 仍返回 `INTEGRITY_ERROR`；空集合沿用同一来源绑定和 head CAS。
- 复用现有来源→对象→资产→store→SQL 边界、`ClaimInboxBeforeWrite` 首写隔离及 `reference_heads` CAS；未修改 content 事件 v1/v2、共享类型或其他模块。
- 已完成 `git diff --check`、JSON 解析和本域文档生成一致性核对。未运行 GitLab CI、本机 Go/gofmt/build/vet/test、make check、真实接口/模型/服务或部署，状态保持 `in_progress`，等待 CI 复核。

# 2026-10-18 收尾检查与停止

自部署GitLab pipeline118：生成及重复生成、16模块readonly build/vet/gofmt、已有普通go test、完整make check、六前端类型/构建通过；无测试文件；仅源码检查，非业务验收。

本域本次明确完整清单2项更新verified，准确ID见progress.json；其余blocked/planned保留，整体服务和全域保护未完成。用户要求push后停止，代理已冻结，不自动启动下一批。

以下为历史记录。

# 2026-10-18 续写第二批检查通过

GitLab pipeline84（源码72219d7；生成381、三个Go分片382/383/384、前端385）全部成功，16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及前端类型检查/构建通过。pipeline83各编译分片通过但缺既有信封校验登记，已补并复检；实际普通测试无测试文件。源码生成物由GitLab工具回传，不代表业务验收、隔离行为验收或部署。

本域本批1项完整函数通过源码检查，完整启动/快照恢复/业务accepted状态保持未完成。模型网关、知识、数据保护三个worktree继续，未在本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 2026-10-17第六轮：上传分块准备决定先于水位截尾（已实现，未跑 CI）

- 分支 `codex/media-20261017-r6`，基线 `9a10eff89c91095670f7d336e84e7a010348adc6`，源码 `35105a5561b5dc5cb8e1f4fbbe0f20d6020e2dff`。独占 `modules/media/`。未 push。准确台账另提交。
- 只调整 `media.PutUploadChunk`：同 op 先回放原 UploadSession，不消费分块流。新写经 `lifecycle.NewWriteContext` 取真实 running 窗口，维护直接 LOCKED。入口仍要求明确 Content-Length 1..8MiB；内部有界读 length+1，长度与 SHA 一致才 fsync。PREPARED 确认写入后再按已提交 SQL 水位截尾、追加并 CAS。不发布资源或事件。
- 复用既有 upload mutex、创建回执和 opened epoch，以及 `mediaWriteContext`、`hashChunkRequest`、`readChunkOperation`、`readUploadRow`、`openMediaRoot`、`reconcileUploadFile`、`writeChunkStage`、`openMediaRegular`、`chunkStageName`、`commitChunk`。依赖齐全。没有另起存储，没有改合同。
- `chunk_writer.go` 与 `PutUploadChunk` 记 implemented，verification 为 not_run。未跑 GitLab CI，不能记 verified 或 accepted。公共入口超限先返回 `BUDGET_EXCEEDED`；`errors.json` 把该码映射为 HTTP 429，本操作 OpenAPI 另列 413。本轮不改平台映射。
- 下一步：等集成会话用 GitLab CI 编译本函数后再决定能否记 verified。订阅仍不能接，因为 `content.resources.changed` 的生产方载荷和 `ApplyReferenceEventInput` 不一致。先改合同再改载荷。不要自行展开 bytes。

# 集成检查（2026-10-17）

引用事件写入已合入 main，GitLab pipeline 42 把 ApplyReferenceEvent 记为 verified。生产方载荷仍未对齐，订阅没有接通。下面是合入前的模块记录。

# 2026-10-17第四轮：引用事件幂等快照（已实现，未跑 CI，订阅未接通）

- 分支 `codex/media-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`（main），源码 `ff91d6aa27a8783c262b07932bbdb85bb0e4ee23`。独占 `modules/media/`。未 push。准确台账另提交。
- 完成范围只限 `media.ApplyReferenceEvent`：输入补齐与 pin 相同的 `sourceOperationId`、`siteId`、`sourceObjectId`、`sourceRevisionId`、`sourceVersion`、`referenceDigest`，并保留 `eventId`。函数记 implemented，不记 verified。accepted 为 0。
- 183 个函数中 161 个 implemented/verified、154 个 verified、4 个 blocked、18 个 planned。55 个文件中 37 个 implemented/verified、29 个 verified、2 个 partial（`app/wire.go`、`transport/server.go`）、1 个 blocked、15 个 planned。`reference_repository.go` 整文件改为 implemented，旧助手函数记录仍为 verified。`BuildApplication`、`NewApplication`、`main`、`Ready` 仍 blocked。`ConsumeEvents` 仍 planned。
- 事件 payload 仍未对齐。`ResourceReferencesChangedPayload` 没有 `sourceOperationId` 和 `referenceDigest`，引用项也不是 pin 形态的 `AssetReference`，订阅不能声称已接通。
- 未跑 go、gofmt、make build、make gen、make check，未 SSH，未跑 CI。本轮只跑 make docs 重写本域 `functions.md`。不能把本轮记成编译通过。无新增测试、迁移或删除许可。
- 下一步：等集成会话审核合入，并让生产方 `content.resources.changed` 载荷与 `ApplyReferenceEventInput` 一致。不要自行展开 bytes。

# 2026-10-17第三轮：真实worker/relay与runtime schema（GitLab检查通过，已交付冻结）

- 用户再授权一轮三个代理并行、检查通过后双推；基线d760e39194156403e2b82430d5018fa9f4425e19，分支codex/media-20261017-r3，独占modules/media/。源码`b439c516fcf58d7df32d6227dc6df18ceb0871ce`，准确台账另提交；无新实际路径/迁移，不展开bytes或模糊引用事件。
- 本轮10个完整函数按真实源码记verified；worker/outbox/blob_repository/drawing_validator及两个精确生成文件verified，app/wire与transport整文件继续partial；Build/NewApplication/Ready及完整服务、bytes、引用事件/其余HTTP、控制恢复与其他后台保持blocked/planned。183函数中160 implemented/verified、154 verified、4 blocked、19 planned；55文件中36 implemented/verified、29 verified、3 partial(in_progress)、1 blocked、15 planned；accepted仍为零。
- NewCleanupWorkerClient仅从jobs拥有的六Result schema构建validator map交公共worker.NewJobsClient窄六方法，协议转换、心跳/租约/取消/终局与paid refs均由平台负责。Claim以最新runtime的纯media/system作用域领取；随后不可变原批准/原任务回执/waiting恢复真实site/epoch/root，所有对象RPC依该已核上下文签名，不读未签名头或猜payload。
- RelayOutbox只执行真实单批SQLite outbox→jobs publish，持久确认后本地标relayed，不复制jobs序号；持续包装仅用根批准RunRecoveryLoop每秒串行调用，公共预检与批内IO写窗口保留。relay基础system仅为producer广播，事件来自已提交outbox，未从任意payload选择RPC站点。
- 完整HTTP/恢复门槛成功后才装配两个背景闭包，不在构造中运行。公共生命周期先取消、Drain仅停HTTP、等待所有背景退出后Shutdown关DB；现完整门槛仍未齐，不运行部分服务或清理未知决定。
- 基线生成Swagger的旧六孤立组件被prune仅为静态发现，旧编译不能当运行初始化/行为通过。显式manifest登记store11+Drawing1+jobs六Result共18及引用闭包；工厂/store/drawing均读取生成PayloadValidationSpec，字段仍唯一来自contracts，无手写schema或扩大HTTP DTO。
- 最终完整依据GitLab pipeline32候选`ae7b334dbd9f7c102ade242afe8be6c4ccd676f9`：生成/重复生成job148、三同时分片149/150/151全部success，14模块readonly build/vet/gofmt和完整make check通过，最终生成补丁0。根精确回传本域19路径（含p31格式/生成）；p30取消，p31三个Go分片通过但整体check缺两个类型，均不作最终完整证据。
- 只读交审公共生成器的显式闭包及jobs六协议、paid refs；发现adapter按领域Input多发Claim dataEpoch与lease jobId/dataEpoch，根最小修为HTTP RequestBody、jobId仅path/epoch仅可信头，复审确认，无其余阻塞。回传后未改Go/HTTP/schema行为，仅根批准订正Drawing首step来源文案GetSwagger→PayloadValidationSpec；未手改生成docs/functions，根实际公共+media集成CI会重新make docs并覆盖该纯说明差异后记录最终证据。
- 本地仅静态JSON/依赖/范围/隐私与Git核对，保持原GitHub代用身份及GPT-6合著。无本机/SSH Go/gen/check，无新增测试/fixture/mock/样例/临时业务程序，无SQL/HTTP业务执行、真实接口/模型、worker/删除、启动、应用镜像或部署。本域不push，交付后冻结，主线实际SHA与双远程推送由根记录。
- 下一项：根完成实际公共+media集成检查、其余域顺序集成与普通双推后停止；下轮须新授权再沿现有工作包继续，不自动展开bytes或引用事件。

# 2026-10-17第二轮：真实资源与未引用列表（GitLab检查通过，已交付冻结）

- 用户再次授权一轮编写后推送，三个子代理独立worktree、全部GitLab三分片。基线8a49af65f048a6b160f963570f02d6b88db100de，分支codex/media-20261017-r2，独占modules/media/；源码`a44a13a1ab65fc19c86e032d757e570195909e22`，准确台账另提交。本域不push，交付后冻结，主线实际集成SHA由根统一记录。
- 完整ListAssets/ListUnusedAssets、7个参考助手及已有serviceHandlers实际绑定共10函数按准确源码记verified（其中9项新完成）。asset_service/cleanup_service/blob_repository/0008四完整文件verified；app/wire与transport整体继续partial，Build/NewApplication/Ready及bytes/真实worker客户端适配/控制恢复保持blocked/planned，业务accepted为零。
- 根批准只追加0008_asset_listing.sql，独立int64列表水位及kind/createdAt/assetId稳定索引；assets/reference_objects变更在原事务递增，缺水位/耗尽失败关闭，不重置或转浮点。源水位独立绑定，既有pin/current/history仍按原事务提升对象generation，不把本地列表水位当作引用源或删除许可。
- ListAssets只读本站真实安全Asset，引用详情独立ReadAssetReferences，无虚构摘要。ListUnused保留既有kind query并补可选nullable输入，每页最多limit个ready候选逐项锁外ValidateCleanupSources；content/site-config完整水位必须全页和跨页相等，current/pending排除，真实历史文档风险去重排序汇总，超预算明确减小limit，未知不当空。
- 本地候选快照结束后才RPC，全部回源后另一个短只读快照重核本地水位及每项Asset/ref/revision/ready/fence/generation；签名和验签复用平台服务key并在SQL/业务锁外。scope绑定caller/管理员/site/epoch/session及权限修订、limit/kind/mode；载荷含本地水位、unused独立两源水位及最后扫描位置。
- unused按最后扫描候选推进，空匹配页可hasMore=true但nextCursor严格前进，末页才false；本地完全无候选不声称实际资产未引用。严格OpenAPI limit=1..100默认20、kind枚举，两GET均no-store；整数转换与固定kin-openapi入口一致采用ParseInt base0，不重做范围校验。不生成计划、批准、任务、模型或删除。
- 最终依据GitLab pipeline23候选`e75c80f63c5424fe67960a875b6f0f70df5c91c1`：生成/格式/重复gen job112、三个同时编译分片113/114/115全部success，14Go模块readonly build/vet/gofmt与完整make check通过。根精确回传本域18路径格式/生成产物，源码未在回传后改动；只更新人类状态说明。p21整体失败（content先编译失败，media未完整执行），p22取消；均不作最终证据，亦不沿用上一轮p16/p18。
- 只读交叉审查content树/metadata CAS、原回执、当前授权一次、发布路径503与purged/预算；最新缺window503及整数解码修正经根确认，未改对方文件。根统一交审后形成最终候选；本域本地仅静态文件/JSON/差异/路径/隐私及Git审阅，无本机或SSH Go/gen/check，无新增测试/fixture/mock/样例/临时业务程序，无实际接口、SQL行为验收、模型、worker、业务删除、应用镜像或部署。
- 本轮18改动路径仅本域，唯一新实际路径为0008，Go源码仅既有5文件，生成物由GitLab生成不手改，保持既有GitHub代用身份及GPT-6合著署名。根顺序合入后仍需实际树复检和本次普通双推，不将候选SHA冒充主线实际SHA。
- 下一项：根本轮统一集成检查与普通push后停止；下一轮新授权再按引用事件合同补齐顶层完整来源对象/操作材料并实施ApplyReferenceEvent，复用现有引用权威，不自动展开bytes或任务适配。

# 2026-10-17补充窄修：pending双投影（GitLab pipeline16通过，最终交付冻结）

- 原p13源码`3d64fbcb71bd955d431c6496ee1d27ea5014ec43`及准确记录`1a5743fd0a1d5da4868304e8b9cc6fefa672da57`保留；根随后复审发现合法远端pending无expiry、本地原pin有expiry会误报冲突。窄修追加源码`2dbc137cb5809e6b9e42addbac2143a7f6ca7eaa`，不改写已提交历史；本记录按准确新SHA追加。
- ReadAssetReferences按完整来源身份（含sourceVersion）先合并再分类。仅两项均未confirmed的pending允许expiry投影差异，去expiry后其他全部规范字段尤其完整Asset必须一致；任一未知expiry保留nil，两者已知取真实较晚时间仅供展示，不写回、延长或按到期释放原pin，过期仍持续保护。current/history及其他矛盾仍严格失败关闭。
- 本窄修最终依据自部署GitLab CI pipeline16候选`8d142bc2dea0877f3b2439f8697e78b64d5865da`全部通过：生成/格式与重复gen一致，三个同时分片覆盖14Go模块readonly build/vet/gofmt及完整makecheck（878文件/1435函数/379HTTP/2328类型）。根已精确回传reference_reader.go与functions.md；不以p13给窄修后树背书，不推定其他未说明候选失败。
- 仅ReadAssetReferences及reference_reader.go按新准确SHA恢复verified；其他p13通过范围保留。worker/outbox整体partial、真实JobsClient适配/bytes/控制恢复及完整服务blocked/planned不变，业务accepted为零。无新增路径、助手、测试/fixture/mock/样例/临时业务程序，无本机或旧VPS编译，无实际接口、SQL行为验收、worker/服务、业务删除、模型调用、应用镜像或部署。
- 当前分支codex/media-20261017-r1，基线e0f307f8c99f7d0a60443d9d81590bdfa6c572db，最终四提交交付仅modules/media/；源码和准确记录分开提交，保持既有GitHub代用身份及GPT-6合著署名。本域不push，根统一合入、实际树复检和本轮普通推送，主线实际SHA由根汇总。
- 下一项：本轮根完成集成与普通push后停止；下一轮新授权再按原交接补完整引用事件来源材料并实施ApplyReferenceEvent，不自动展开后包。

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
