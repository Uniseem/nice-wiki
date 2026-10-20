# 2026-10-21 B1 资源事件来源 v2（已提交，待 GitLab CI）

本轮从 `a53fe59c` 建立独立 worktree `/private/tmp/nice-wiki-content-b1-v2-20261021`，分支为 `codex/content-b1-v2-20261021`，独占 `modules/content/`。源码与合同提交已顺序完成：合同 `911dbb29`，producer `d0c0dd5f`，合同窄修 `f34d346c`，移除已完成 content 阻塞的合同修正 `9f0e70cb`。

- `content.resources.changed` 保留 v1 `$defs/ResourceReferencesChangedPayload` 原样作历史只读兼容；新增 v2 `$defs/ResourceReferencesChangedPayloadV2`。v2 的来源身份包含 `sourceService=content`、`sourceOperationId`、`siteId`、`sourceObjectId=documentId`、`sourceRevisionId`、`sourceVersion`、`referenceDigest` 和完整 `AssetReference[]`，`eventId` 只在 jobs 信封中。
- v2 引用数组直接复用已持久 `draftMediaReferences`：`kind=pending_pin`、`confirmed=false`、`expiresAt=null`、原 `asset`、稳定 `referenceId` 与原 `referenceDigest`；空集合仍保留完整来源身份。没有把条目改为 `current/confirmed`，不重新计算摘要。
- `draft_commit_store.Commit` 仅在 `action=save` 的 COMMITTED 草稿保存事务内写 v2 outbox；Create 共用提交路径但不生产资源事件。`PublishContentEvent` 拒绝旧 v1 引用载荷，发布/撤回不再生成资源来源事件；发布内部仍只复制已确认草稿引用到发布快照。
- `ReadReferenceDecision` 继续以已提交、`action=save` 的原来源决定作为非空发布引用资格。分享图 metadata pin、公开 OG 新水位与缓存失效仍是后续独立工作项；当前非空 shareImage 继续失败关闭，旧正文快照不可改写。
- 静态 JSON、范围、隐私、`git diff --check` 已通过；未运行本机 Go、pnpm、build、check 或业务验收。producer 文件/函数只记 `implemented`/`not_run`，不能记 `verified` 或 `accepted`。

## 下一步（一项）

由集成会话复核本域提交并在用户自部署 GitLab CI 运行生成、readonly build/vet/gofmt 与完整结构检查；media 侧需注册 `content.resources.changed` v2、从 jobs 信封传入 `eventId` 并接入 `media.ApplyReferenceEvent`。v1 只读兼容不能补造 v2 来源。

---

# 2026-10-18 收尾检查与停止

自部署GitLab pipeline118：生成及重复生成、16模块readonly build/vet/gofmt、已有普通go test、完整make check、六前端类型/构建通过；无测试文件；仅源码检查，非业务验收。

本域本次明确完整清单7项更新verified，准确ID见progress.json；其余blocked/planned保留，整体服务和全域保护未完成。用户要求push后停止，代理已冻结，不自动启动下一批。

以下为历史记录。

# 集成检查（2026-10-17）

发布快照已合入 main，GitLab pipeline 42 把本轮函数记为 verified。分享图仍 503，完整启动仍 blocked。下面是合入前的模块记录。

# 当前工作包：发布确认改用已提交草稿来源（2026-10-17第四轮再退回，已编写，未编译）

分支 `codex/content-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`。源码 `67f35e4857ee66970b3ff6b84184d7e9eb92c447`。独占 `modules/content`。未跑 GitLab CI，未编译，不记 verified，accepted 为 0。

- 非空引用的 confirm 使用已提交草稿保存操作的来源身份：同一 document、revision、sourceVersion 和 referenceDigest，来自 revision_preparations 与 action 为 save 且已 committed 的 draft_operation_intents。已确认 pin 按媒体原绑定幂等回放。
- 找不到这条决定，或摘要与当前引用不一致，整次发布失败，不先公开。空引用仍不调媒体。分享图非空仍 503。媒体调用仍在本地 SQL 锁之外。
- 函数完成数仍是 227/261。ReadReferenceDecision 的查询范围未改。

## 下一步（一项）

等集成会话审核合入并跑 GitLab CI。

---

# 当前工作包：发布资源 pin 接回（2026-10-17第四轮审核退回，已编写，未编译）

分支 `codex/content-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`。源码 `7edca48674a7287cb9fc454fa509164e61d71fd2`。独占 `modules/content`。未跑 GitLab CI，未编译，不记 verified，accepted 为 0。

- 非空就绪引用已接回现有 MediaClient：发布事务开始前先 acquire 再 confirm，跨服务调用不持有本地 SQL 锁。pin 失败则整次发布失败，不先公开。空引用仍可发布，不造空 pin。
- 分享图非空继续 503，不猜测 metadata pin 协议。Update 仍 in_progress。完整启动仍 blocked。
- 函数完成数 225/259 变为 227/261：新增 preparePublicationPin、pinPublicationReferences，均记 implemented。

## 下一步（一项）

等集成会话审核合入并跑 GitLab CI。

---

# 当前工作包：发布快照闭环（2026-10-17第四轮，已编写，未跑 CI）

分支 `codex/content-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`，初始干净。源码 `b6d27b3dc3ddeaa0311c8f687052bfedd58a29e5`。独占 `modules/content`。未跑 GitLab CI，未编译，不记 verified，accepted 为 0。

- 发布、撤回、提交、可见性校验和公开读取已按约束级步骤落地。人工 slug/SEO 优先，本轮不调用模型，不制造 slug 或 SEO。首次空 slug 在发布时要求人工填写，不因缺模型阻塞已有合法元数据。
- 原回执、metadata/tree 或发布 CAS、正文引用、快照、历史和 outbox 同一事务。未知提交只回放原结果，不重复推进水位或事件。历史回滚直接失败关闭。
- 曾发布路径写入私有 SQL 版本化 manifest，Move 与 Update 在同一事务读取。分享图实际变更仍因精确 pin 合同未齐返回 503，Update 继续 in_progress。
- 四个已有发布 HTTP 操作绑定真实处理函数。BuildApplication 仍返回 nil 和 503，缺可信写窗口的新写继续 503。回收站、历史回滚、预览、屏障和编辑器 UI 未做。
- 函数完成数 199/241 变为 225/259：本轮 32 个函数记 implemented。其中 6 个原先 verified 的函数因源码有变，改回 implemented。
- 集成会话需把本轮新文件登记进结构清单后再跑检查和 GitLab CI。本域不自行修改公共文件，不 push。

## 下一步（一项）

等集成会话审核合入并跑 GitLab CI。

---

# 当前工作包：元数据编辑与精确路径校验（2026-10-17第三轮，已检查交付）

分支codex/content-20261017-r3，基线d760e39194156403e2b82430d5018fa9f4425e19，初始clean；独占modules/content。用户再授权一轮三个子代理同时编写，根统一GitLab生成/格式/三分片编译/check及普通双推；本域不本机/SSH Go、不自行push。

- 源码327f70e0da5080aa9a36fe0316cbf58e5d4c699e，本记录紧随源码。8个本轮完整/修改函数按该SHA verified，净增4项完成，195→199/241；路径文件及0007完整。Update、metadata/tree/transport文件仍partial/in_progress，Move真实manifest分支未齐，完整Build/main blocked，业务accepted=0。
- Update明确分支在DB/锁前一次即时content.edit，同代完整scope/规范请求摘要原回执优先，核原结果hash及严格结果schema。锁外当前写窗口，短SQL读取当前metadata expectedRevision与真实treeRevision双CAS，标题/人工SEO/未发布路径、完整原结果及既有tree.changed outbox同事务。Commit未知只按原op回放，不重复改修订或事件，不发布正文/不触发模型。
- 根批准0007仅追加metadata原回执，不建正文、影子树或媒体来源。分享图同一完整ref可维持，实际改变因精确metadata pin/引用源合同未齐503。曾发布node或后代路径改变因真实manifest未齐503，未知旧路径不当空；不能把支持分支记成完整受约束Update。
- ValidatePathSegment消费调用方同SQL可信完整树，纯助手不另开SQL或重复身份RPC；正则、活动父与同父活动唯一并排除自己。根级仅保留navigation/ask，对应public-site精确语言根路由；子级允许，不禁api/admin/v1/robots/llms/sitemap。私有助手同样用于Move，不能用移动到根绕过；未来发布须沿同规则。
- PUT metadata复用真实RegisterTreeRoutes，五个实际固定action由core一次即时鉴权，平台服务/委托/epoch/schema仍严格执行；路径documentId精确合并生成严格请求DTO。完整启动仍未齐，缺可信写窗口新写503，不运行半成品服务。
- 2026-10-17根自部署GitLab CI pipeline32，候选ae7b334dbd9f7c102ade242afe8be6c4ccd676f9：生成/重复生成job148及三同时编译分片149/150/151全部success，覆盖14模块-mod=readonly build/vet/gofmt和完整make check，最终生成补丁0。根精确回传本域21路径（含pipeline31格式/生成）。pipeline30取消；pipeline31 Go全部通过但整体check缺两项类型登记失败，不计完整通过；修登记后32全过。只证明编译及结构检查，无新增测试/fixture/mock/样例/临时业务程序、SQL/HTTP验收、模型/业务worker/服务/镜像/部署；未本机或SSH Go，未自行push。
- 已只读交叉审根公共签名siteId、委托同站点与独立Claim/真实jobs adapter，确认旧HTTP proof头会清除；提出控制链可能继承旧proof后，根统一在workerControlContext剥离，保留原root/site/费用/截止。并只读审model r3 owner多root/同站同代、未知金额、charge_unknown证据及精确UTC纳秒keyset，无阻断。根本轮已补公共可信本站传递，替代上轮尚缺前置；仅当前源码/合同及CI证据，不声称真实跨域调用链已运行或费用验收。
- 两次实际暂存内容、中文说明、既有GitHub代用作者/提交者与GPT-6 trailer隐私扫描通过；源码18路径、本记录7份模块文档/台账。无其他域或公共文件改动，未自行push；交付后冻结。

## 下一步（一项）

根按源码及记录顺序集成，检查实际合并树后执行本轮已授权普通双推；本域交付后冻结，不自行扩manifest/分享图引用或完整启动。后续只按根新工作包继续。

---

# 当前工作包：文档树读取与完整同级重排（2026-10-17第二轮，已检查交付）

分支codex/content-20261017-r2，基线8a49af65f048a6b160f963570f02d6b88db100de，初始clean，独占modules/content。用户再授权三个子代理同时编写一轮后普通推送；全部Go/生成/格式/编译/check由根自部署GitLab三分片执行，本域不本机/SSH编译、不自行push。

- 源码91f6f0d3246e50281f81f138f3fedcbf70b6e17b，本记录紧随源码；21个本轮完整/修改函数按该SHA verified，净增18项完成，177→195/238。MoveDocument及tree_service整体仍partial/in_progress，path_service仅Plan完整、ValidatePathSegment仍planned，BuildApplication/main blocked，业务accepted=0。
- ReadDocumentTree/ListDocuments同SQL核站点/epoch、原始metadata schema、共享head/COMMITTED资格、无环/深度32、活动父及同父key/slug唯一，确定性前序且不返正文；trash需content.publish，其他读content.read。列表签名cursor绑定主体/会话/权限版本/state/limit、treeRevision和同SQL读取的reference_scan_state来源水位，任一变化409首批重读；0005覆盖metadata/title/state/head/隔离变化，SaveDraft不推进树水位也不混完整metadata快照。
- 完整ReorderChildren要求完整同父活动draft/published子集合，排除trash/purged，6位均匀base62 key保留插入间隔，空集合允许；即时content.edit和平台当前窗口均在锁外，短SQL复核树CAS、metadata CAS，更新真实parent/order并提交完整原结果及content.tree.changed/v1 outbox。Commit未知重试只凭原回执，不重复推进水位/事件。
- 根批准0006仅追加完整scope/动作/摘要原回执，无影子树/假manifest；新事件只含site/treeRevision/changed IDs/epoch，当前Consumers=[]。业务生产权限content.edit，jobs事件管理仍沿既有全域content.publish，没有宣称公开缓存或在线UI已消费。
- 10000仅为新树查询/操作明确扫描/输出预算，purged墓碑不计入，超限503不截断。已按根裁定撤回候选提出的Create新增总量拒绝，旧readCreationTree nil回调保持原无新增总量限制；内部Ensure/Plan可信完整树输入取消maxItems，不新增maxDocuments或永久业务配额。
- Plan完整规划node及全部曾发布后代旧路径，未知祖先slug明确503；非空规划因真实版本化manifest生产链未齐使Move返回503、不提交。因此不能把草稿移动分支当完整受约束Move，发布与manifest继续未完成。
- 四个实际树action由core在schema之后、DB/锁之前按state做一次即时identity；平台Authorize仅这四固定action不重复RPC，其他action保留通用授权，服务/委托/epoch/schema仍严格执行。列表integer适配与kin-openapi基数0 ParseInt一致，本文件既有资源分页亦窄修，不把允许的0x10/010误转0。
- Build只局部构造仍nil/503；新写在原回执回放后显式核Runtime/WriteWindow回调齐全，缺失503，修复候选曾直接调用nil回调会panic的部分装配错误。没有伪造control写窗口、全量handler、恢复activation或后台生命周期。
- 2026-10-17根统一自部署GitLab CI pipeline 23，候选e75c80f63c5424fe67960a875b6f0f70df5c91c1：生成/格式/重复生成job112及三个实际并行编译分片job113/114/115全部success，覆盖14个Go模块-mod=readonly build/vet/gofmt及完整make check；根精确回传本域22变更路径。pipeline21 content-media因Revision与int64比较不匹配真实失败，显式转换最小修正；pipeline22取消，不计通过。只证明编译与结构检查，未新增/运行测试、样例、SQL/HTTP验收、真实接口/模型、业务worker、镜像或部署；未本机/SSH编译或自行push。
- 已交叉只读复审model价目整数、未知价、旧摘要兼容、纯服务与无正文持久化，无本轮阻断。另发现既有纯服务HTTP会把Actor.SiteID固定system，不能兑现清portal proof后保留真实本站；根已将可信本站传递列入localization未来合同/交接前置，EstimateLanguagePlan仍planned。本轮不扩共享签名协议，content/media真实来源链不声称已联通或已行为验收。
- 实际暂存内容、中文说明、既有GitHub代用身份及GPT-6 trailer隐私扫描通过；源码19路径，无其他域或公共文件改动；本域没有运行本机Go、真实接口或push。

## 下一步（一项）

根按源码及记录顺序集成，检查实际合并树后执行本轮已授权普通推送；本域交付后冻结，不自行扩发布/manifest或共享站点协议。后续只按新工作包继续。

---

# 当前工作包：资源来源查询与单批事件转发（2026-10-17，已检查交付）

分支codex/content-20261017-r1，基线e0f307f8c99f7d0a60443d9d81590bdfa6c572db，初始clean；独占modules/content。用户授权三个子代理同时编写一轮后普通push，并最新要求全部编译改走自部署GitLab、尽量三个并行任务。根统一公共机制、CI与最终推送；本域不自行push。

- 源码提交16029fde3736071e1e127c87faf918305eed237a，本记录紧随源码。12项净新增完整函数及修改的ReadReferenceDecision按该SHA记verified；函数完成数165→177（总224）。独立reference_reader与0005迁移完整；outbox_repository/publication_repository/transport仍partial，BuildApplication/main继续blocked，未实现发布及全量HTTP不提前完成。业务accepted=0。
- 独立ReferenceRuntime使查询不依赖编辑锁；List每页同一SQL快照完整核元数据/head、COMMITTED/原回执/精确引用双向覆盖，包含current_draft/history及Acquire前已持久pending_pin。历史按真实索引分类投影，pending_pin的retained=true仅表示保护；明确终止且无COMMITTED才排除。
- 已获根批准的只追加0005仅维护全站单调来源水位与覆盖来源权威的事务触发器，不持久化读快照。签名游标绑定过滤/limit/caller/actor/site/epoch/session及版本、水位和位置；跨页变化409从头。触发器耗尽回滚原写入。材料逐源读取，完整源操作、文档和引用各20000预算，超限503而不伪报完成；SQL在签名私钥IO前释放。
- 当前真实元数据出现尚未支持的published/分享图来源时503，不将未知当完整空集。正文schemaVersion、JSONL原格式未变。根已协调媒体消费者接受pending_pin并投影未确认保护。COMMITTED是不可撤销历史决定；后来正文/文档隔离不撤销精确资格/回执/refs保护，当前正文读取仍拒绝隔离，材料矛盾422不降pending/abort；曾提出隔离一律422的审查意见已撤回。
- 两个真实只读HTTP路由使用平台统一服务身份、当前epoch和schema边界；列表限定固定纯服务或portal即时content.read，决定限定media纯服务。BuildApplication真实构造局部路由后仍因其他handler、恢复activation和后台生命周期未齐返回nil/503，不运行半成品服务。
- RelayOutbox仅装配真实同库outbox.NewStore、版本化JobsClient与根公共RunRelayBatch；行租约、确认、退避、隔离、恢复丢弃和当前安全窗口由平台实现。单批计数不表示全域已送达，完整后台循环未完成。
- 2026-10-17根统一自部署GitLab CI pipeline 12，候选fccc3750bd8310ac0c9de5e092f6aceda0158fde：生成/格式及重复生成成功；三个实际同时编译分片覆盖14个Go模块-mod=readonly build/vet/gofmt与完整make check通过（878文件/1435函数/379HTTP/2328类型）。根精确回传格式/生成物，未手改gen。pipeline 11生成/格式完成但content-media因漏登记jobs:publishEvent导致JobsClient被裁剪真实失败；修契约来源后完整重跑通过。仅证明编译和设计结构，未新增或运行测试/样例、SQL/HTTP行为验收、真实接口/模型、服务/镜像/部署；未本机Go或push。
- 实际暂存源码、中文提交说明、既有作者/提交者GitHub代用邮箱及GPT-6 trailer经隐私扫描通过；未修改Git身份。源码18路径、本记录更新7份模块文档/台账；无其他域或公共文件改动。

## 下一步（一项）

根按源码及记录提交顺序集成，检查实际合并树并执行用户本轮已授权普通推送；本域交付后冻结，不自行开始发布/全量HTTP或部署。后续只在根新工作包下继续。

---

# 当前工作包：持续目标第二轮真实共享草稿闭环（2026-10-16，已交付）

分支codex/content-goal-r2，基线471c036082e082763d90e1080b3489d4e4531380，初始clean；独占modules/content。根批准internal/draft_commit_store.go及只追加0004迁移，统一公共结构、VPS生成/检查和push；本域不自行推送。

- 已编写真实延迟DraftCommitStore/Transaction及完整CreateDocument、ReadDraft、SaveDraft、ReadReferenceDecision。创建固定空正文、IDs、基准语言与fractional order，站点treeRevision CAS加doc不存在谓词；保存只能修改真实存在文档并核有效租约。历史retired ID永久拒复用，head/version、独立引用水位、COMMITTED资格、refs、完整原结果与平台outbox同SQL。
- 文件IO及当前平台窗口RPC均在doc guard/SQL之前，guard只跨短SQL；原回执命中实际释放对象。未知Commit、坏正文或未知依赖保留pending与pin，COMMITTED优先；确定失败须同SQL证明无COMMITTED并封存源，随后隔离该次未提交文件和Release完整原source材料。正文schemaVersion=1及原8字段JSONL不变，0004不猜补未知既有文档或引用资格。
- 根与交叉复核完成四类收口：独立documents.reference_version与草稿版本分离并CAS；首次配额在intent/pin前拒绝且pending续作先短核真实lease/CAS；失败收敛从当前SQL快照刷新完整原记录与pin，竞争COMMITTED先核资格/原结果再结束事务回放；初读/prepare并发返回按真实终态统一分派，仅pending继续准备。原op不改source水位、不替换pin，已成功/已prepared回放不因降低配额重做。
- 第一次VPS make docs/gen与格式完成，build因未使用time导入失败，未运行check；删除残留导入后第二次docs/gen、14模块build/vet/gofmt通过，但check报告4条可达性登记失败。已补真实文件校验/目录fsync调用边，删除全域无调用GuardLease及专用快照字段、合同和台账，没有假调用。
- 最终冻结源码根统一隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、8源码gofmt、14模块-mod=readonly build/vet/gofmt及完整make check通过（876文件/1403函数/379HTTP/2317类型）；精确生成/格式路径回传，其他域跟踪漂移0。只证明编译与设计结构，不代表业务验收。
- publication_repository仍只完成ReadReferenceDecision，CommitPublication及发布/撤回/回收站等生命周期保持未完成；完整HTTP、RelayOutbox/在线消费、后台与启动、恢复activation仍未完成。未新增或运行测试/fixture/mock/样例/临时业务程序、SQL/HTTP业务验收、真实接口/模型、服务/镜像/部署；本域未运行本机Go/gofmt/生成/编译/vet/测试。
- 源码提交7f7ec904b5bb9563bb801750512fea3483513cf6；本记录紧随源码。57个本轮检查函数按该SHA记verified，删除GuardLease无幽灵台账；相对基线完整函数净增52（113→165），新增或补登记49个参考函数并删除1个死参考函数。publication_repository整体partial、全部HTTP/Relay/启动及未实现发布生命周期保持未完成。完整文件净增6，含本轮刷新并核对的两个生成类型文件；不把生成接口解释为已绑定HTTP。
- 两次实际暂存、中文提交说明、作者/提交者GitHub代用邮箱及GPT-6 trailer隐私审核通过；未修改Git身份，未push。本域交付后冻结。

## 下一步（一项）

根按源码与记录提交顺序合入并复核实际集成树，执行本轮已授权普通push；下一工作包由根按新main基线派发。本域交付后冻结，不自行开展HTTP或发布生命周期；R2与部署未开始。

---

# 当前工作包：持续目标第一轮完整编辑锁用例（2026-10-16，已交付）

分支codex/content-goal-r1，基线685db4f48bb359f79a7501098204ccc5a4cde6b2，初始clean；仅拥有modules/content。用户新授权每轮三个代理编写、审核并push直到代码编写完成，已替代历史两轮后停止条款。根统一公共文件、VPS检查和push，本域不自行推送，交付后等待下一轮工作包。

- 源码提交86d5fec5fcd973bc3ee50a8f7fb329e4fcedd74f；本记录紧随源码。新增已获根批准的internal/edit_lock_operations.go与只追加migrations/0003_edit_lock_operations.sql。完整Acquire/Renew/Release/Force/Expire五用例和十个参考助手、四个本轮完整源码/迁移文件按该SHA记verified；五核心委托方法、AcquireDocumentFence与OpenDatabase修改同步新SHA。业务验收未执行。
- 每次用户用例先即时identity核当前会话/本站功能与版本。账户名仅同次权威响应并作为纯文本；identity.AuthorizationDecision必填accountName前置由根先行提交a5b6713及记录f34c565并通过VPS。别人持锁按原DTO成功返回lease+writable=false；原operationId规范摘要包含动作与完整输入，跨动作/文档复用冲突，同键回原结果不改变或复活租约。
- 平台当前写窗口在doc锁前读取；life读锁/doc mutex内只做候选与短SQLite事务。真实文档生命周期/deleteFence/隔离在事务重核，状态事件revision跨boot持久单调，通知/本地审计/完整原回执提交后才安装内存。Commit返回未知时保留pending；编辑与保存guard只按真实原回执及hash解决，未知就失败关闭。候选错误不突变权威cell，SaveFence匹配只纯核到期，不清锁漏事件。耗尽不安装失败候选，旧lease依原界点失效。
- 15分钟提醒与17分钟绝对失效不延后，心跳/输入共享序号分别单调；新输入清提醒发送active状态。后台只处理本站活跃cell和同操作已完成scope，使用可信调度now，逐篇原回执支持部分完成后重试。锁仍只以内存为授权权威；原回执是不可变历史结果，重启从不据此重建租约。
- 可靠通知本轮完成content.edit-lock.changed v1生产者outbox及本地审计，outbox表由平台EnsureSchema创建；jobs目录由根生成，新事件Consumers=[]。完整RelayOutbox/HTTP绑定、portal在线消费/流、启动后台装配仍未完成；不把排队成功解释为在线只读已生效或完整服务可运行。Create/SaveDraft/完整提交事务与恢复activation仍blocked/planned。
- 2026-10-16根统一隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：本轮冻结候选make docs/gen、14模块-mod=readonly build/vet/gofmt与完整make check通过（871文件/1313函数/379操作/2296类型）。根精确回传7份格式/生成文档；根与模型代理交叉审查同doc候选/真实原回执/未知Commit pending/15与17分钟/guard无阻断。只证明编译与设计结构，未新增或运行测试/fixture/mock/样例/临时业务程序、SQL/HTTP业务验收、真实模型、服务/镜像/部署。本域未本机Go/gofmt/生成/编译/测试；实际暂存内容/身份/说明隐私扫描通过。
- 首次git暂存/提交因worktree独立index位于主仓只读.git而被沙盒阻止，没有提交；取得本次已授权Git操作所需自动审查权限后重跑，17份真实暂存内容/身份/说明扫描通过并提交。不把首次空暂存扫描当通过，不改Git身份，不push。

## 下一步（一项）

根按源码与本记录顺序集成、复核并执行本轮已授权push；下一轮由根按新main基线派发。可直接接续既有internal/outbox_repository.go的content.RelayOutbox：先读本域合同与平台RunRelay/当前写窗口，真实装配jobs客户端，不虚构在线通知消费或全服务已启动。本域收到下一包前冻结。

---

# 当前工作包：第二轮真实编辑锁与文档fence前置（2026-10-16，已交付）

分支codex/content-next-two-r2，基线53acce862467dbe40b829d1984519d78ccac5608，初始clean；独占modules/content。用户授权两轮并行，本轮是最后一轮，根统一检查/push后停止，不自动第三轮。

- 新增实际源码internal/edit_lock_service.go、internal/document_edit_state.go与只追加migrations/0002_document_edit_state.sql；前者是既有登记路径，本轮新增2文件合同。真实documents存在/生命周期/delete_fence与隔离读取，不插入假文档，缺行NOT_FOUND。
- 控制区flock加NOFOLLOW/PrivatePath/SameFile持有单leader，不unlink。manager life RWMutex使所有操作及返回guard持读锁，Close先排空再释放flock；所有编辑/提交/生命周期guard使用相同site/doc mutex。
- 账户共享租约、单次boot与单调fence、2分钟TTL和LastInputAt+15分钟提醒与+17分钟绝对失效；新tab复用账户lease不重置活动。新增required heartbeatSequence/inputSequence，严格新心跳才续TTL，新输入才用服务器接收时钟更新活动；旧心跳不续、不接受lastInputAt作权威，EditLease返回最新共享序号。
- SaveFenceGuard真实核holder/lease/boot/fence与私有文档/deleteFence/隔离，并持doc锁至Release；未来完整SQL提交仍必须同事务重查持久状态/CAS/ledger/refs/pin/receipt/outbox，当前没有DraftCommitStore占位。
- 本轮仅参考核心，Acquire/Renew/Release/Force/Expire完整用例仍blocked：即时授权、identity安全账户名、可靠状态通知/审计与原操作回执未装配。Core返回的快照不能当作已通知或HTTP成功。Create/Trash等完整生产者未交付，0002不会制造文档。
- 修正后的同一冻结源码已通过根VPS docs/gen、4源码gofmt、14模块readonly build/vet/gofmt与完整check（869文件/1299函数/379操作/2289类型）；本域不运行本机Go/SQL/测试。未新增测试/fixture/mock/样例/临时业务程序，未真实API/模型、服务/镜像/部署，未自行push。

- 根复核定点收口：缺失doc在cell分配前NOT_FOUND，不进入后台map；拿doc锁后仍重读真实权威。提醒固定LastInputAt+15分钟，+17分钟绝对失效，不因检测延迟推后或被逾期输入续活。

- 首次候选VPS gen/docs成功，但编译发现3个gen业务Input被拆分HTTP request裁剪而undefined；已按types.json在本域投影Renew/Release/ForceRelease业务输入并改核心签名，随后根已完整重跑通过，不把首次失败计为通过。未手改生成目录或扩生成器。

- 源码提交41953e630843abefa15a1da7976bf49ce7c7bd35；本记录紧随源码，22新参考函数及修改的RecoverDatabaseJournal按该SHA记verified，2新完整存储文件与修改storage核心已检查。edit_lock_service整体partial、5完整锁用例及完整保存/启动仍blocked；无业务accepted，两轮后本域停止，不自动第三轮。

---

# 当前工作包：第一轮真实私有正文存储（2026-10-16，已交付）

分支codex/content-next-two-r1，基线d2161f41c441b69f080b7d8cf3d40b88afecb357，初始clean；独占modules/content。用户授权三个子代理并行两轮及每轮根统一检查/push，本域本轮只交付真实正文存储，第二轮待根给新基线。

- 新增internal/storage.go、migrations/migrations.go、migrations/0001_domain.sql。独立RevisionFileStore/WithRevisionFileStore兼容既有完整存储注入，不继承或伪装DraftCommitStore；未添加BeginDraftCommit占位。
- 私有SQLite安全打开/只追加连续迁移及已应用SQL哈希核对，首次新空库绑定可信打开epoch，既有未知/异代库失败关闭。首次Prepare在同SQL事务绑定可信站点并持久原材料；准备不创建文档、不推进head、不造COMMITTED资格。
- 完整scope和摘要原结果回放，合并原8字段JSONL终局及真实COMMITTED回执；提交资格只来自精确修订/准备/原回执联查。坏已提交文件持久隔离并关闭文档资格，坏未提交准备只隔离自身。
- BuildApplication实际打开/迁移/独占恢复后，仍因完整草稿事务、全部handler、activation及后台装配未完成而blocked；恢复换目录后的epoch重绑定未交付时仍失败关闭。maxDocumentBytes和公共写窗口只由已授权上游传入，不虚构配置或开放writer。
- 本轮完整文件和函数按源码提交97b31b1919181a4a697c8259c85ea9575f225a4d记verified；根统一隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，go mod tidy、make docs/gen、14模块readonly build/vet/gofmt与完整make check通过（865文件/1258函数/379操作/2277类型）。go.mod/go.sum和格式/生成物均由根VPS回传，未手写锁文件；本域未运行本机Go/编译/测试。未新增测试/fixture/mock/样例/临时业务程序，未运行SQL行为验收、真实API/模型、服务或部署，未自行push。

- 源码提交97b31b1919181a4a697c8259c85ea9575f225a4d；当前记录提交紧随源码。新3文件/31参考函数及修改的正文读取/恢复源码已逐包检查；首个迁移与epoch同事务，journal共享RWMutex+NOFOLLOW/SameFile避免半行/路径替换，启动反向核所有prepared/committed必须有完整匹配JSONL，preparing无日志合法窗口保留。完整DraftCommitStore/SaveDraft、全部HTTP/后台与恢复activation/epoch重绑定未交付，main/Build仍blocked。

---

# 当前工作包：文档树与资源引用纯规则（2026-10-16，已交付）

分支codex/content-persistence-round1，基线170c1dc9057e122e349de43db532b2c48434b6cf，初始工作区干净，独占modules/content。本轮用户要求三个子代理并行一轮，由根统一集成与VPS检查。

- 源码提交f7bb9b8c029f3cc6af996708d28f50492ffb0546：新增既有登记的internal/tree_validator.go与internal/reference_extractor.go，2完整文件/5函数按准确源码SHA记verified。
- 树规则检查当前完整父索引、重复ID、缺父、环和32层；拟移动拒绝自身/后代，按整棵子树高度核移动后上限，后代ID字典序返回，输入不变。
- 资源规则按正文前序和固定角色顺序保留所有嵌套资源、画板3文件、上传视频及平台本地cover/poster；稳定sourceKey去重保留不同block，完整引用冲突失败关闭，水位/当前历史分类保留，最多10000项。
- 原持久化适配切片因本域尚无登记的私有DB schema/迁移文件与启动打开协议而未展开；根同意改做上述既有纯规则。真实准备记录、COMMITTED资格、隔离adapter与BuildApplication/main继续blocked，未猜建表、未放松head/ledger/refs/receipt/outbox同事务。
- 根统一隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、两源码gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（861文件/1185函数/379操作/2269类型）。暂存内容与提交信息静态隐私扫描通过；verified仅表示当前编译与结构检查，不代表业务验收。
- 不改正文schemaVersion、OpenAPI/公共类型或其他域；未本机Go/gofmt/编译/测试，未新增测试/mock/fixture/样例/临时验证程序，未调用真实接口/模型或部署，未push。本域本轮交付后停止，由根集成，不自行启动后续工作包。

---

# 当前工作包：第五轮不可变正文文件核心（2026-10-16，已交付）

分支codex/content-revision-files-r1，基线9ff93ae71e8f262ea0fc057aa3f6eea71ce52f22，独占modules/content。上一public-site纯契约f9aa83f/9567637已独立交付并保留分支，本轮不再改该域或根。

- 已写WriteRevisionFile/ReadRevisionBody及直接要求的CanonicalizeBody/HashBlock、树/行内/表格/分栏语义源码。准备原材料先持久、复用原结果，fsync/rename不推进head；读取仅可信COMMITTED资格，坏文件真实隔离无fallback。
- 新增实际路径仅既有登记的internal/{body_canonicalizer,block_validator,table_validator,columns_validator,revision_reader}.go；原revision_files/revision_repository复用并统一16MiB硬限。接口扩展遵守根裁定，无DB adapter假实现，无新迁移或真实HTTP成功占位。
- 源码提交02ede7bdfd70b263511a1f82343c602a3c73143c，7个完整核心文件、35个本包完成函数及VPS生成客户端按准确源码SHA记verified；check/gen/build证明编译和结构，不代表业务验收。
- 最终隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：make docs/gen、14现有Go模块readonly build/vet/gofmt及完整make check通过（859文件/1142函数/379操作/2264类型），覆盖最终同一time.Time与contentInteger修订。
- 未本机Go/gofmt/gen/编译/测试，未新增测试/mock/fixture/样例/临时验证程序，未运行真实API/模型/服务，未push或部署。本轮交付后停止实施，由根统一集成、复核与推送，不启动后续包。
- 真实私有DB准备/提交资格/隔离adapter、保存/发布/HTTP/后台仍缺，BuildApplication/main继续blocked；既有fence/稳定ID历史ledger/head/refs/receipt/outbox事务责任未转移到文件准备。

---

# 之前工作包的历史记录

更新：2026-10-15。阶段：R1 实施中；本域业务尚未验收、尚未部署。

- 2026-10-15 集成复核修复：`cmd/content` 补 `Application.Serve/Shutdown` 方法，入口恢复可编译；运行状态适配解开 `state` 外层。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次 `R1-05` 基线为 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`，独立分支为 `codex/content-r1-05-recovery`；前序启动准备仍由 `f62ea3d7c52bc98ebd73d94c1428ca8794caf751` 提供。
- 本次已实现 `content.RecoverPreparedCommits`（提交 `f376a3e779cd7cffc0f1f214a99b8b6ea5090055`）：恢复只能在注入的独占启动/safe 屏障内进行；校验严格 journal、正文哈希与已有 head；有 committed receipt 的 PREPARED 才补记 COMMITTED；缺正文、哈希不符或 head 不一致的提交隔离并关闭文档可见性；函数没有推进 head 的能力。`content.WriteRevisionFile` 及 `BuildApplication` 接线仍按范围保持 planned/blocked。
- 本次实现 `content.CommitDraftJournal`：按 operationId+规范摘要先查已提交 receipt；对 PREPARED 正文拒绝符号链接/非正规文件，复核大小、SHA-256、文件同步与目录同步；校验旧head、草稿版本连续性、journal与draft一致性及站点/dataEpoch；由注入的私有事务在同一边界执行 fence检查、head CAS、稳定ID台账、COMMITTED journal、refs、receipt和outbox。未注入事务存储或发生不确定存储错误时明确返回 `DEPENDENCY_UNAVAILABLE`，不将启动伪装为可用。
- 本次源码与模块台账提交为 `448ad37fbb2980e0eab87845be100d2e731bc409`；该提交只包含 `modules/content/`。
- 状态区分：上述源码为 `implemented`；仅完成 JSON、隐私、差异空白和暂存内容静态检查，Go 编译/gofmt/行为恢复检查未运行，故未标 `verified`；业务验收为 `accepted` 之前的未执行状态。
- 已创建启动与部署准备文件：`cmd/content/main.go`、`internal/app/wire.go`、`internal/transport/server.go`、`go.mod`、`Makefile`、`config/service.defaults.json`、`deploy/manifest.json`、`Dockerfile`；实现提交为 `f62ea3d7c52bc98ebd73d94c1428ca8794caf751`。
- `content.main`、`content.Serve`、`content.Shutdown`、`content.Health`、`content.Ready` 已写入；`content.BuildApplication` 已接入配置、信任锚、site-config运行权威和恢复换目录前置检查，但因业务存储、journal恢复、全部OpenAPI x-handler、后台函数和activation回执未实现，按合同返回 `DEPENDENCY_UNAVAILABLE`，保持 blocked。
- `go.sum` 未创建：当前环境无Go工具链，VPS不可用，不能用手写内容代替 `go mod download`。Go build、vet、gofmt、make verify、Docker构建和业务验收均未执行。
- 本地Python JSON解析与隐私检查通过；`git diff --check` 通过；根 `scripts/check_docs.py` 因公共结构清单尚未登记本域新增文件而报告未登记项。根结构清单属于集成会话所有权，本次未修改。
- 未新增测试、样例、快照、E2E、真实接口或真实模型调用；未push。

逐文件/函数事实见 [progress.json](progress.json)，交接证据见 [docs/handoff.md](docs/handoff.md)。公共资料由集成会话维护，本域只修改 `modules/content/`。

- 本轮集成适配：生成器修正nullable引用后，草稿前序修订按指针空值与expectedHead比较，首次提交和后续CAS语义保持一致；本域build/vet/gofmt通过，业务验收未执行。
