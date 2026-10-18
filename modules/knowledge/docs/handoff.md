# 继续完成知识域首批（2026-10-18）

独占modules/knowledge/，分支codex/finish-20261018-knowledge，基线770ea1c86a4afcc4cb968f4f6feb1808d467e7c7。用户最新要求三个代理持续写完剩余代码、及时根GitLab与双推，旧75%冻结指令不再适用。源码13d53c7，本域不push。

3个原有主业务函数及6个必要助手已有完整源码，均blocked待GitLab；另外4个既有函数修改后须重验。配置HTTP真实接线，首次0→1/后续CAS、原回执同事务，未配置返回NOT_FOUND。启用核可信基准语言及MG纯服务目录；仅消费MG兼容可选embeddingSpace完整五字段，缺失失败关闭，不复制spaceId算法。明确关闭不依赖模型可达。没有因为修改配置自动重建/花钱。

新增internal/index/repository.go、migrations/migrations.go、002_workflows.sql，根framework须登记；modernc.org/sqlite复用仓库已批准1.59.0，由根CI生成sum。本域内嵌迁移打开同站同代私有DB、平台outbox/inbox建表，拒绝重标旧库。GC仅真实已知退役时间至少24小时且无active/未完成调度或模型引用的代数；所有索引为SQL，同一SQLite快照拥有读者生命周期，没有额外文件句柄。ActivateBuild同步写真实retired_at_ns，旧未知时间不回填。

需根统一gen保留新增七个PayloadValidationSpec schema aliases（MG模型/档位、jobs创建/取消、content发布正文及两事件）。admin私有authorizeAdmin直接改名导出AuthorizeAdmin，供配置跨包实际复用，未加空wrapper。下一批直接实现build/create.go的持久创建与调度，再继续正文/媒体提取、真实分词边界、执行器/启动/UI；tokenizer/PDF项目已获用户批准，版本与公共免费tokenize端口由根协调，本域不伪造模型token。

未本机运行Go/gofmt/gen/docs/build/vet/check/pnpm、SQL验证、真实接口/模型/服务/部署，未新增测试/fixture/mock/样例或临时验证程序；仅文本/Git审阅及git diff --check。

## 本批新增待CI精确ID

- `knowledge.PutIndexConfig`
- `knowledge.ReadIndexConfig`
- `knowledge.CollectRetiredIndexes`
- `knowledge.OpenRepository`
- `knowledge.BeginIndexWrite`
- `knowledge.ReadConfig`
- `knowledge.ReadOperationReceipt`
- `knowledge.SaveOperationReceipt`
- `knowledge.validateConfiguredSpace`

## 本批修改待重验

- `knowledge.AuthorizeAdmin`
- `knowledge.ActivateBuild`
- `knowledge.NewIndexRuntime`
- `knowledge.NewOperationHandlers`

以下为历史记录。

# 2026-10-18 第二批集成检查通过并已双推源码

自部署GitLab pipeline70（生成311，三个Go分片312/313/314，前端315）全部成功；精确879713字节生成物回传到3deb8ba9cd394ec43274999eaf4f5e0484e224ae；pipeline71对该精确SHA再次全部成功并核对生成零漂移。16个Go模块readonly build/vet/gofmt与go test、完整make check、Vue类型检查与构建通过；go test无测试文件，不代表业务验收。

本域本轮精确清单6项记verified，模块当前55 verified、1 implemented；accepted=0。源码交付 `3deb8ba9cd394ec43274999eaf4f5e0484e224ae` 已普通双推并核对两个远程main一致；本结果记录在同次授权下复查同步。完整服务装配、恢复与其余具体未完项继续按台账blocked/planned，不以编译通过代替业务验收。模块代理均已冻结。

以下为历史记录，其中待CI/真实设计阻塞以最新逐项台账为准。

# 2026-10-18 集成检查通过

pipeline60（生成261、三个Go分片262/263/264、前端265）完整通过，精确生成物已回传。逐项台账只把完整待编译清单转verified；partial、完整启动及既有设计阻塞保持blocked。所有Go模块readonly build/vet/gofmt与go test、完整make check通过；public-site前端类型检查及构建通过。go test没有测试文件，不构成业务验收。accepted=0，未部署。

本批源码汇总d344a37，记录提交等待零生成漂移复核后普通双推。持续向75%推进，用户最新要求及时分批检查/推送，历史停止指令不适用于本次授权。

以下为模块原始交付记录，其中“未CI”只描述交付时。

# 索引检索批次交接（2026-10-18）

本批源码提交 `8632a73c68ac7be79e1cfecfaf8c7a936fe38ce2`，基线 `acc9bbcc1fe0cfbeae91d3c2835b09cea5cbe9bd`，分支 `codex/knowledge-20261018-bulk`，只拥有 `modules/knowledge/`。根授权的公共适配 `8c593f3`、原生重排 query `b05944f` 已作为基线分别 cherry-pick 为 `6701308`、`c042a00`，不是本域交付，不要再次作为本域代码合入。

## 代码就绪，等待编译

主业务 22 项（均加 `knowledge.` 前缀）：

- `knowledge.SplitChunks`
- `knowledge.WriteStagingChunks`
- `knowledge.BuildLexicalIndex`
- `knowledge.RequestEmbeddings`
- `knowledge.StoreEmbeddings`
- `knowledge.ValidateBuild`
- `knowledge.ActivateBuild`
- `knowledge.NormalizeQuery`
- `knowledge.EmbedQuery`
- `knowledge.SearchLexical`
- `knowledge.SearchVector`
- `knowledge.FuseCandidates`
- `knowledge.ValidateCandidates`
- `knowledge.RerankCandidates`
- `knowledge.SelectContext`
- `knowledge.Retrieve`
- `knowledge.ReadSections`
- `knowledge.CreateQueryEmbedding`
- `knowledge.GetBuild`
- `knowledge.ListBuilds`
- `knowledge.RedactDiagnostics`

内部助手 32 项全部在实际代码中被调用并已同步 implementation/progress：NewOperationHandlers、jsonOperation、operationContext、writeResult、NewModelRuntime、WithModelRuntime、CompletedOutput、WithAdminSigningKey、authorizeAdmin、beginAdminRead、validVector、unitVector、acquireQueryFlight、NewSearchRuntime、WithSearchRuntime、searchRuntime、activeSnapshot、publicationKey、sortedSnapshotRows、ReadActiveSnapshot、requestRerank、DecodeVector、TokenizeText、NewIndexRuntime、WithIndexRuntime、RuntimeFromContext、ReadBuildRecord、BeginBuildWrite、storageError、ReadChunks、ReadGeneration、validatePublication。共 54 项统一 `blocked`，原因仅为待集中 GitLab 生成/格式/编译检查；不是设计 blocked，不计 implemented/verified。NormalizeSource 旧实现未改动。

## 实际行为与存储

- 分块不跨稳定 block；按选定模型 tokenizer 提供的 tokenEnds 切分与重叠，逐 UTF8 字节保留源映射；输出 ID 绑定来源、上下文与区间。没有伪造模型 tokenizer。
- 本域独立 SQL 保存完整分块/词项/向量，固定词项算法和规范清单 hash。staging 写入使用平台写窗口并绑定持久租约；active 不被改写。ValidateBuild 前后固定清单并向 content 校验，ActivateBuild 仅接受一秒内证明及活动代数 CAS，同事务写 index.ready outbox；旧代数保留。
- 检索固定一次 SQL 只读快照及完整空间；BM25/cosine→RRF→权威过滤→可选重排→整块预算→最终权威过滤。任意权威失败不当作空成功。ReadSections 只给当前 active 精确引用和完整 sourceOffsets。
- 查询向量按站点/代次/可信根/queryHash/spaceId 持久唯一，本进程按该身份 singleflight。原 operationId 与 MG 幂等键保留，失败/断线不重算。另有调用者幂等键→完整身份摘要映射，同键异输入 CONFLICT；保存前重取平台写窗口。
- 模型仅使用平台 NewGatewayClient/InvokeModel；三个 MG 响应和 content 批量校验响应均用生成 PayloadValidationSpec 的严格生产方 schema。embedding 结果只读完成事件结构化 output；native rerank 显式 query；未配原生时 fast/chat 最多 800 输出 token 严格读既有 ID 排列，格式错误只追加一次独立 attempt。费用未知不重放。
- 六个实际 HTTP 映射已提供，须由完整 app 交给平台 NewServiceHandler；缺其他 handler 时 app 应启动失败。没有给未完成功能加占位成功或把完整服务记 ready。

## 合同与统一生成要求

本域 types 追加 Chunk.sourceOffsets、SourceBlock 与准确 tokenEnds，向量 number 统一 double；OpenAPI 对 sections 明确证据映射，并通过 x-runtime-payload-schemas 保留 CreateInvocationResult、CancelInvocationResult、StreamEvent、ContentValidatePublicationsResult 的生产方引用闭包。源码使用未来生成的 `gen.PayloadValidationSpec`、`gen.Chunk.SourceOffsets` 和 double 向量，根候选 CI 必须先 make gen/docs，再分片编译；生成目录没有手改。

实际 schemaVersion 1 迁移文件已编写，完整启动尚未应用，索引不另写第二份文件。data/workflows 已同步 SQL 清单表示和所有实际规则。实现合同及台账同步到实际 95 个函数，其中新增 32 个必要内部助手；不是增加空函数凑比例。

## 验证事实

仅本地 JSON 解析/状态同步、Git 隐私身份核对与 `git diff --check` 通过。未运行 Go/gofmt/build/vet/gen/docs/check、测试、模型、真实接口、业务服务或部署。作者与提交者保留用户 GitHub 代用邮箱，合著 GPT-6 使用指定 noreply。用户最新授权及时 GitLab 检查和推送，由根会话执行，本域不 push。

## 尚未完成与下一项

完整 ExecuteBuild、SourceBlock 的模型 tokenizer 来源、资源提取、配置/构建创建/事件执行、恢复屏障与完整 BuildApplication 尚未完成。这些原有 planned 项没有计入本批 ready。下一项先由根在 GitLab 对本批统一生成、编译和完整 make check；编译/结构问题先修，再把对应 54 项按真实证据更新。行为验收仍未授权且保持未开始。

---

以下旧交接是历史记录；原 SplitChunks 设计缺口已按本次根裁定修正，不能继续当作新阻塞。

# 索引检索会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `NormalizeSource` 按合同步骤5 施加截止 = min(ctx, op.deadline, 5000ms)。
- 补齐 go.sum。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。

## 本次事实

2026-10-15 认领本域文本规范化切片。按本域合同实现了无副作用的 `knowledge.NormalizeSource`：校验封闭输入，统一 CRLF/CR 为 LF，使用已登记的 `golang.org/x/text v0.20.0` 执行 NFKC，并为每个规范化 UTF-8 字节保留原文起点偏移。未读取其他服务私有数据，未调用真实接口、模型、媒体或数据库，未新增测试/样例。

## 基线与交付

- 独占范围：`modules/knowledge/`
- 分支：`codex/knowledge-normalize`
- 基线：`8ae6fcfc6302e2fd41a998faef18be52e2a9714c`
- 实现提交：`77fe57291ab9bcc0f53df2d35018e055e61ca12f`
- 本次修改：`go.mod`、`internal/chunk/normalize.go`
- 状态记录提交：`808d734bb802514ba606da2f31937baacbd6d48c`；未提交文件应为空。

`NormalizeSource` 已标记 `implemented`，尚未标记 `verified` 或 `accepted`。本机无 Go/gofmt，不能完成编译/格式化/行为验证；`go.sum` 尚未生成。公共 `docs/structure/framework.json` 未在本域修改范围内登记新文件。

## 检查与结果

- `git diff --check`：通过。
- Python JSON 解析 `modules/knowledge/progress.json` 及既有本域实现合同：通过。
- `python scripts/check_docs.py`：失败；根 `framework.json` 未登记本轮及其他并行会话新增文件，并报告已有 `site-config` 台账缺少提交 SHA。本次未修改根文件。
- Go 编译、gofmt、VPS `make check`：未运行，本机无 Go/gofmt 且本次不读取私有连接配置。
- 普通测试、E2E、真实接口、真实模型：未运行/未调用，未新增测试授权。

## 阻塞与决策

`knowledge.SplitChunks` 的 `SplitChunksInput` 只有 `buildId/document/normalized/chunkTokens/overlapTokens`，但 `Chunk` 必须带 `BlockRef`（含 `blockId/blockHash`），且步骤要求记录全部 blockIds 与原 UTF-8 偏移。当前契约没有提供块列表、块边界或这些标识的可信来源；本次不生成伪造 block，也不修改公共合同，留待集成会话先修正设计。

## 下一项（一项）

公共契约补齐 `SplitChunks` 的源块边界与稳定 `BlockRef` 输入后，再在 `modules/knowledge/internal/chunk/split.go` 实现 `knowledge.SplitChunks`；开始前复读该函数的 implementation/types/data 条款，完成确定性分块、偏移、hash 和预算校验，并保持不调用模型。


## 继续实施第二批26ee466

3原主函数+4助手完整源码待GitLab。003_sources.sql需根framework登记。CreateBuild已实际HTTP202；原配置/代数水位/精确任务payload/ObjectRef/派发意图/安全原回执同事务，jobs不可达不虚报创建成功而保留queued意图，由后续恢复批继续。DispatchBuild锁外RPC，真实jobId与jobs operationId保留，取消竞态不能遗失接受引用。只有可信background费用类别可排入后台构建，不沿QA费用类别冒充后台。

源正文明确完整canonical DocumentBody JSON，16MiB字节硬上限并核原contentHash；精确publishedRevisionId读取后再次核租约保存私有源。ListExtractableAssets每项传精确version并核本站/ready/SHA等全字段；SourceAssetVersion和原BlockRef单独保留，公共AssetRef不改。每文档100资源，单件50MiB/MIME不支持显式unsupportedAssets，不假空成功。

公共9002b22已以9a43c38合入本分支，是根共享机制不重复作为本域交付。后续使用inbox.NewEventConsumer/RunConsumerWithResult，不造Jobs假方法或空统计。root确认MG免费tokenize支持单叶1Mi Unicode字符及4MiB字节；完整Doc仍16MiB。本域后续SourceBlock按真实叶子/sourcePath/源范围独立分片，同BlockRef可有多个真实片段，真实BPE端点可在UTF8内部，chunk选择完整码点端点但保留真实token计数。

GC窄修保留已付费完成model_steps供哈希复用，不随索引回收删除；未终态/收费未知步骤仍阻止索引删除。未执行Go、生成、测试、SQL/接口/模型/服务或部署，未新增测试/fixture/mock/样例或临时程序。继续事件/取消/恢复及提取执行链。

### 第二批新增待CI精确ID

- `knowledge.CreateBuild`
- `knowledge.FetchPublishedSource`
- `knowledge.ListExtractableAssets`
- `knowledge.QueueBuild`
- `knowledge.DispatchBuild`
- `knowledge.scanAssetSources`
- `knowledge.extractableMediaType`

### 第二批修改待重验

- `knowledge.CollectRetiredIndexes`
- `knowledge.NewIndexRuntime`
- `knowledge.NewOperationHandlers`
- `knowledge.jsonOperation`
- `knowledge.writeResult`


## 继续实施第三批c686488

6个原业务及2必要取消助手完整源码，累计本轮12原业务源码，全部新改blocked待GitLab。新增internal/index/inbox.go与cancellation.go需根framework；ConsumeInbox参考函数移到index包、签名增加内部tx参数，ID保留，events/inbox.go仍拥有ConsumeEvents，不发生包循环。

原EventEnvelope由公共订阅边界核严格生产schema，只核一次；handler再核原身份/水位，平台ClaimInbox与发布构建/撤回失效/安全outbox同事务。停用或未配置只消费跳过；不反向同步回源，不以草稿触发AI。撤回按每个真实旧active的发布/space发完整合schema index.retired，旧历史Build状态保留而generation_state独立retired。

消费复用根9002b22的NewEventConsumer/RunConsumerWithResult真实计数，outbox复用NewEventPublisher/RunRelayBatch。JobsAcknowledgeEventResult生产schema需gen保留。PublishOutbox pending明确仅本批未确认，不冒充全域积压。ClaimInbox异hash隔离提交语义已交根补统一机制，本域失败不确认，不谎称已持久隔离。

取消先同事务清staging写资格/lease及保存cancel_requested，再SQL锁外请求真实jobs和平台模型网关取消。重放原结果也继续未完副作用；不退费，不删收费未知引用。Dispatcher补RPC前pending→dispatching资格事务及cancel_requested门禁，完整原payload/inputRef/root/site/epoch/后台costClass核对，取消可区分未派发和接受未知。未知接受保留原意图等待原回执/worker认领，返回依赖未知，不伪造已清理。撤回重放只处理已持久取消意图，不取消新发布构建。

原任务恢复需jobs OwnerRecoveryChecker正式HTTP协议；本域不建新dedupe绕过orphan。平台Worker续租还需根统一OnLeaseRenewed镜像真实ExpiresAt到本域业务行，不能域内再造心跳。PDF固定版本v0.0.0-20260907135840-6c8c28e0e8a0已获授权，root登记MIT/原BSD来源并CI锁；解析预算与公共受监督沙箱runner接口待根裁定，不能把不可取消goroutine叫真沙箱。UI需portal非序列化apiClient/CSRF桥，不能读取HttpOnly token或假成功。继续本域真实分词/提取与执行器，未运行Go、SQL、服务或真实接口/模型，未新增测试样例。

### 第三批新增待CI精确ID

- `knowledge.HandlePublication`
- `knowledge.RetireDocumentIndex`
- `knowledge.ConsumeInbox`
- `knowledge.ConsumeEvents`
- `knowledge.PublishOutbox`
- `knowledge.CancelBuild`
- `knowledge.CancelBuildSideEffects`
- `knowledge.CancelDocumentSideEffects`

### 第三批修改待重验

- `knowledge.DispatchBuild`
- `knowledge.NewIndexRuntime`
- `knowledge.NewOperationHandlers`


包循环窄修源码b241838：按根认可把runtime/model_client.go实际模型适配整体移到internal/model/client.go，所有index/search真实import与机器合同同步。ID、签名、行为未变；runtime回归启动/worker/恢复，从而可以合法依赖index/build。根framework须删除旧实际路径并登记新路径；以下原模型函数重新blocked待CI：knowledge.InvokeModel, knowledge.NewModelRuntime, knowledge.WithModelRuntime, knowledge.CompletedOutput, knowledge.BindAdmissionPermit。未运行Go或本机生成。


## 第四批bf29364及内部助手同步8800855

新增LoadRuntime原函数、EnsureBuildDiskBudget必要助手，并修订已有NormalizeSource/SplitChunks；累计本轮13原业务源码，仍全部新改blocked待GitLab。三个旧规范化实际助手只是补登记（validBoundedText/canonicalizeLineEndings/checkContext），不计新增原业务。runtime/bootstrap.go新增真实源码需根framework按现有计划核对。

LoadRuntime核可信DataRoot、站点/epoch、SQLite quick_check、所有活动指针及每代完整manifest hash；错误关闭就绪，不重建/调用模型。QueueBuild只在新代数前检查真实Statfs≤10%，旧同源原结果复用不受影响。Root已认可公共59ef42d以6ed7d0c合入；docs/protocols追加段、platform进度冲突保留本分支原待CI事件记录并按实际合并合同重算摘要，不重复作为本域提交交付。

完整DocumentBody仍16MiB；真实text/alt叶1Mi Unicode字符/4MiB UTF8。NormalizeSource按NFKC稳定迭代边界输出有界4MiB/1Mi字符片段、真实原字节范围及nextByte，处理膨胀而不截任意字节，非空nextByte必须继续。SourceBlock加sourcePath/sourceByteStart/end，同原BlockRef不同真实片段允许，不造新块ID或全块假tokenEnds。SplitChunks只接受真实BPE字节端点（允许UTF8内部），选择chunk端点与重叠起点时保证完整码点，tokenCount仍真实token序号差；每块64KiB/每批256，nextCursor必须续读，全构建ordinal保持。Chunk.sourcePath兼容可选，旧active JSON不改，旧无path整块ID算法保留。

PDF只编写了解析算法及待提交go.mod精确require，尚未接root公共seccomp自身子进程runner，不计ExtractPdfText ready；这些未提交文件由本会话拥有，继续接线时一起提交。禁止本机Go/生成/测试/真实接口，事实未变。本域仍继续执行器、提取、恢复、完整启动与UI。

### 第四批新增/旧实际补登记待CI精确ID

- `knowledge.LoadRuntime`
- `knowledge.EnsureBuildDiskBudget`
- `knowledge.validBoundedText`
- `knowledge.canonicalizeLineEndings`
- `knowledge.checkContext`

### 第四批修改待重验

- `knowledge.NormalizeSource`
- `knowledge.SplitChunks`
- `knowledge.QueueBuild`


## 第五批46d3260

新增已获根认可的现有管理只读index-options，knowledge.manage，真实基准语言/模型目录/公开树完整引用，独立ready/unavailable与安全错误码，不虚构空成功。catalog共用严格生产schema、纯knowledge服务身份与完整五字段空间比较，不计算spaceId；background配置与QA选择费用类别分离，新增build要求精确受支持tokenizer声明。HTTP实际映射已接。

事件ClaimInboxBeforeWrite为事务首项写入，真实tx.Commit回调只持久隔离，随后原INTEGRITY_ERROR继续返回不确认。原词项TokenizeText不联网，删除root误登记MG依赖；实际BPE助手另在source/tokenize.go接生产端。

新ready精确ID：knowledge.ReadIndexOptions、knowledge.optionErrorCode、knowledge.ReadModelCatalog、knowledge.EmbeddingProfile。修改重验：knowledge.validateConfiguredSpace、knowledge.ConsumeInbox、knowledge.TokenizeText（仅修合同）、knowledge.NewIndexRuntime、knowledge.NewOperationHandlers。全部blocked待GitLab，原未改证据保留。根framework须登记internal/admin/options.go与internal/index/catalog.go。共享ceab1d7/76df9dd/dc8435d已合入本工作区，不作为本域提交再次合入。PDF算法/go.mod仍未提交，不计ready。未运行本机Go/生成/检查/测试/真实接口或模型。

真实BPE窄批60ae744：新增knowledge.TokenizeSource、新改NewIndexRuntime待CI。严格MG生产schema/full space/tokenizer版本/textHash/计数/递增端点覆盖真实调用；词项TokenizeText不联网。ExecuteBuild仅登记后续真实依赖，完整执行器仍未ready。root framework加internal/source/tokenize.go。


## Vue管理界面9a26e81

3原主函数LoadAdminOverview/SubmitPrimaryEdit/mountModuleUI及6必要UI事件助手真实实现，新改均blocked待Node22 GitLab。源配置/模型安全选项/公开引用/构建分页真实HTTP；未配配置NOT_FOUND仅显示未配。编辑真实space+基准语言、expectedVersion及同输入原幂等键；409保留输入、另读权威最新版本，不自动覆盖重发。保存不自动模型或重建。人工构建明确确认可能后台费用，202只显示接收；取消固定原键不推断退款。导航/卸载取消，过时响应不回写。只import type公共ModuleAPIClient/Bootstrap，由portal实际持有CSRF与认证。入口为库ES模块mountModuleUI，不伪装独立有权限页面。

精确原函数ready：knowledge.LoadAdminOverview、knowledge.SubmitPrimaryEdit、knowledge.mountModuleUI。必要助手：knowledge.safeCode、knowledge.refresh、knowledge.createBuild、knowledge.cancelBuild、knowledge.afterSave、knowledge.save。新增style.css须根framework登记，其余均原计划路径。Vue3.5.22/Vite7.1.7/plugin-vue6.0.1/TS6.0.0-beta/vue-tsc3.1.1复用前已批准版本，不新增借用项目。pnpm-lock仅根CI真实生成，不手写、不本机pnpm。所有checks仍未本机执行。

原叶真实BPE闭环c1cfac4：新增knowledge.BuildSourceBlocks/knowledge.collectSourceLeaves两个必要助手，单次一个真实叶NFKC片段+Next，最近稳定块text/alt原文视图偏移，完整文档hash/sourcePath/真实BPE无伪造。ExecuteBuild依赖实际改为BuildSourceBlocks→TokenizeSource。新文件internal/source/blocks.go须根framework登记，均blocked待CI，不增加原主业务完成数。


## 付费前空间与真实步骤日志aa1b103/dc89973

所有embedding请求显式固定完整expectedEmbeddingSpace，MG付费预占前核主/备用。查询原结果优先，新调用按可信costClass的同空间embedding档位（MG本身每代costClass+tier唯一），不拿background档位给QA或改费用类别。新增任务覆盖若改变实际模型，MG expectedSpace门禁仍先于付费拒绝，不复制MG路由算法。

InvokeModel现在真实使用本域model_steps：原输入去许可token的规范hash+root/costClass、原op/key先落库，MG所有内部事件先保真实invocationId，再转可选UI事件。完整所有终态与hash保存后返回，原结果复用，无论failed/charge_unknown都不另建调用。网络未知/取消保留dispatching/running与原key；后续只同MG幂等键找回，不能作为safeToRetry的新付费授权。构建调用绑真实buildId owner；QA默认root。完成付费记录独立保留，GC与取消原有查询现在真实有来源。NewModelRuntime参考签名新增db/window，无现存实际调用方；完整app后续真实装配，缺失存储时失败关闭。

新增必要助手ready：knowledge.WithModelOwner、knowledge.beginModelWrite、knowledge.BeginModelStep、knowledge.SaveModelProgress、knowledge.CompleteModelStep。旧函数重验：InvokeModel/NewModelRuntime/RequestEmbeddings/CreateQueryEmbedding。根framework新增internal/model/journal.go、migrations/004_model_result_hash.sql。所有均blocked待GitLab，未本机Go/生成/SQL/测试/真实接口模型。共享f287c31仅恢复DTO已合入不再作为本域提交合入。


## 整篇上下文与检索派生字段bde10a9

新增原主函数knowledge.EnrichChunkContext及6实际必要助手ReadContextPrefixes/ContextSourceKey/ClaimContextIntents/ApplyChunkContext/TextProfile/RetrievalText。已核产品基线ai-qa:46及ADR0010要求每单元补背景，无可选降级；完整固定发布canonical JSON作为documentText，核原contentHash，超65536字符/模型预算失败关闭，保留旧active/已付费结果，不截断、无额外摘要。最多16片段、knowledge.context fast后台500输出token，严格完整JSON/原ID/一至两句。

背景按完整文档/记忆/模型声明与真实原块/path/区间缓存，防重复短句串背景；相同源hash重发布可复用。付费前原build/root/op独占同源意图，其他构建等待原结果而不重复付费。机器prefix/contextVersionHash/embeddingTokenCount为Chunk兼容可选字段，绝不改原text/hash/offsets；完整检索文本真实BPE后词项/embedding使用，原引文只原text。RequestEmbeddings不再用装配假上限，真实目录模型与完整文本真实BPE检查总预算、付费前重核持久租约；幂等摘要含完整检索文本hash。

Root认可TokenizeSource保持ID/行为移internal/model/tokenize.go，source和index复用ModelRuntime.Raw+严格schema，旧source/tokenize.go实际删除，避免包环。新框架路径chunk/context.go原planned、model/tokenize.go（删旧）、migrations/005_chunk_context.sql。NewModelRuntime/RequestEmbeddings/BuildLexicalIndex/BuildSourceBlocks/TokenizeSource须重验。全部新改blocked待GitLab，未本机生成/Go/SQL/测试/真实接口或模型。完整ExecuteBuild与启动仍未ready，继续真实提取/恢复。


## 原构建恢复a5944f7

原主函数knowledge.RecoverBuildLeases完成：有界扫描到期/待派发，真实时点解析，锁外content核原引用，未知失败关闭；明确失效只按原lease_until literal CAS关闭旧staging写资格、写取消意图后锁外取消原job/MG，续约竞态不错误隔离。pending/dispatching只原inputRef/root/dedupe/dataEpoch续派；requeued只实际保存job引用，取消no-op不计。已dispatched原任务到期保留阶段/模型/付费结果给jobs原owner恢复，不造新dedupe绕orphan。

发现真实跨域请求漏jobs CreateJob必填dataEpoch，DispatchBuild已补原值，不把编译成功当语义正确。validatePublication直接导出ValidatePublication以供恢复复用，原ID/调用同步，没有新增空wrapper。原函数ready：RecoverBuildLeases；修改重验ValidatePublication/ValidateBuild/CreateBuild/DispatchBuild。均blocked待GitLab，未本机检查/SQL/真实API。runtime/recovery.go原planned路径。

## 视频与精确原文视图82178c4→9aba9fe

新增原函数ExtractVideoMetadata（根已确认）只读固定正文video题名/简介及来源，metadata有真实文字可ready，当前无字幕transcriptState独立unsupported，无URL重抓或自动转写。平台视频无视频asset则内部asset缺失保留sourceBlock，不以poster冒视频。

CollectSourceLeaves直接导出，原ID同步。原文视图顺序明确text有序content→alt→仅video title→description直接串联无分隔符，嵌套稳定块独立起算。ReadSections新sourcePath精确回读私有发布body/hash/原块/JSON叶/UTF8范围；资源路径按完整站点/代次/发布/原块/AssetRef/路径读独立不可变机器原文视图，缺失NOT_FOUND，不冒充人工正文。006是该视图表，真正ExecuteExtraction生产写入未齐不计ready。

新增ready精确ID：knowledge.ExtractVideoMetadata、knowledge.FindVideoBlock、knowledge.DownloadExtractionAsset、knowledge.ValidateSectionSources、knowledge.ValidateSourceRange、knowledge.ReadExtractionSourceView。修改重验：ReadSections、BuildSourceBlocks、CollectSourceLeaves。PDF真授权bytes/精确version与发布/50MiB/hash/Close、公共seccomp runner和500页/8MiB算法已有源码，但ExtractPdfText/RunPDFParserChild/ParsePDFPages保持partial，main实际子入口未齐不计ready。

根framework新增extract/source.go、extract/pdf_worker.go、search/source-views.go、source/extraction-view.go、迁移006，pdf.go/video.go原planned。PDF已批准固定版本仅go.mod require，sum由CI生成，根负责MIT/BSD原notice。全部新改blocked待GitLab，未本机Go/生成/SQL/测试/真实接口模型。继续生产方、worker和入口。

## 实际main/PDF子入口c01547a与别名窄修a7f9b4b

main现在先RunPDFParserChild，handled在任何服务配置/密钥/DB之前exit0/1，无PDF或详细错误输出；普通进程只装配本域平台RunService。PDF实际二进制入口已齐，ExtractPdfText/RunPDFParserChild/ParsePDFPages本批转为源码ready待CI（非已验证）。main/Serve/Shutdown/Health原入口源码完整。HTTP Drain与后台等待先于私有DB关闭，不在Serve信号回调抢先关库。

共享LoadServiceIdentity直接复用，严格站点配置生产schema取得真实siteId/基准语言，runtime/恢复换目录在打开active之前，私有DB/模型步骤存储/现有真实handlers与事件/转发/原恢复/十分钟GC已有实际装配。完整BuildApplication/Ready/NewApplication/bindSearchRuntime/assembleBackgroundTasks仍partial：恢复激活回执、控制/快照、worker/提取、UI静态服务未齐，严格完整HTTP及必要后台入口缺任一就拒容器，不开假服务。Ready safePoint未装配保持false，无配置不虚构模型空间。

a7f9b4b修复先前模型适配迁移时移走runtime.OperationContext别名的遗漏，类型别名在runtime/bootstrap.go恢复，必须同批合入。原LoadRuntime逻辑未变，文件重验。根framework app/wire.go和cmd/knowledge/main.go均原planned。共享c5a0dcd/dd1e399只作基线不重复作为本域交付。source ready新增：knowledge.main、knowledge.Serve、knowledge.Shutdown、knowledge.Health、knowledge.ExtractPdfText、knowledge.RunPDFParserChild、knowledge.ParsePDFPages、knowledge.NewRuntimeClient、knowledge.serviceOperation。全部新改blocked待GitLab，未本机Go/生成/SQL/测试/真实接口模型，完整app不能计完成。

## 真实视觉与受控PDF附件5328558

新增原函数ExtractImageDescription/DescribeAttachment及必要助手SameAsset/DescribeResource/DescriptionText。真实PNG/JPEG/WebP/GIF image与受控原生PDF file+vision，不把PDF当image；精确version授权bytes/hash/长度后，只原批准background费用类别/任务记忆/源语言执行。普通PDF无文字仍unsupported、无自动OCR；原生PDF分支要求另一个已批准knowledge.extract-attachment的精确任务记忆。完整文本附件UTF8不超过65536字符，不截断。MG负责真实能力/主备用/预占和供应方，模型原结果日志保留，不改人工caption。

本批只拥有真实描述任务；跨构建asset/task/model/memory缓存由CreateExtraction/ExecuteExtraction拥有且仍未齐，不提前ready。实际模型版本缺口已精确报根：MG Invocation只带attempt.modelId，不能把事前目录版本冒充实际派发版本，待权威route/model snapshot字段完成长期缓存。本批新函数blocked待GitLab。根framework新增extract/describe.go，image.go/attachment.go原planned。公共File字段为基线，MG两条源码由根一起合入，生成目录未手改，未本机Go/生成/SQL/测试/真实接口模型。

## 稳定模型语义与任务计划冻结a913dc1

根已认可owner加稳定SemanticKey唯一保原MG key/op，同阶段配置/记忆漂移CONFLICT，不用新hash绕旧unknown。context按原chunk IDs，embedding按原batch IDs，资源description按原asset，QA按原规范查询及明确格式attempt身份。原op仍须匹配，换控制op不能另建语义记录。旧记录只有原op及完整旧请求hash一致才绑定原MG key，不能证明则保留原结果拒新付费。

007新增semantic唯一索引和build_context_plans。首次完整任务记忆、档位、声明模型计划及hash持久冻结，后续漂移在新context意图前拒绝。新增ContextSemanticKey/FreezeContextPlan，InvokeModel/BeginModelStep/EnrichChunkContext/RequestEmbeddings/CreateQueryEmbedding/requestRerank/DescribeResource修改重验。全部blocked待GitLab，生成目录未改。根framework新增007迁移。后续执行器还须冻结首次真实分批计划，不能因窗口变动自动重分批绕旧unknown。

## 实际winning来源与缓存93b6f4c

只按MG resultAttemptId所指真实attempt.modelSnapshotHash消费opaque证明，不按state猜winning，不复制MG算法。ReadModels权威hash和本域已存在own-task ReadTaskAssignments纯服务严格schema，只用于当前覆盖/备用缓存资格。背景每项实际snapshot加完整源/记忆版本缓存，prefixContextHashes/modelSnapshotHashes逐项真实；兼容整批contextVersionHash仅请求计划hash，Apply写原Chunk的实际背景hash和contextModelSnapshotHash。资源描述实际modelSnapshotHash保留，旧缺证明结果不猜版本复用。

新增必要助手WinningModelSnapshot/HasCompleteResult/ReadTaskAssignment/ProfileModelSnapshots/ContextResultHash，008追加缓存实际证明列。unknown无完整结果可原MG key继续观察，不另attempt；有效原结果不可覆盖，对账不能改winning来源。公共当前Invocation snapshot及完整输出恢复API由根正在统一补，WinningModelSnapshot/HasCompleteResult/CompletedOutput/BeginModelStep/CompleteModelStep相应恢复消费仍partial，正常新SSE分支真实可用，不提前当完整恢复ready。其余缓存纯助手待CI。根framework新增model/provenance.go、008迁移；共享bb9窄公共API基线c992380不作为本域交付重复pick。全部新改blocked，未本机Go/生成/SQL/测试/真实接口模型。

## 原终态取消清理92c71de

RecoverBuildLeases原扫描漏掉已cancelled/superseded但原job/MG清理失败的持久意图，本批将其纳入同一有界扫描；真实取消不再被未来旧镜像租约跳过。仍只取消原job/key，不续建dedupe或宣称退款。修改函数knowledge.RecoverBuildLeases待根GitLab，未本机Go/生成/检查。执行器/worker未提交工作与本窄修独立。

## 原任务拥有者与执行链76caed2 / 恢复证明f35238c

已写CreateExtraction/ExecuteExtraction/ExecuteBuild/RunWorker真实链；新增原jobs拥有者记录、原完整输入校验、真实续约/取消回调镜像、独立extract派发/原来源缓存、阶段计划、最终业务状态与JobExecutionResult同事务。构建/提取平台循环1+3，等待孩子不inline。根已批准补jobs claim可选jobTypes与TypedWorkerJobsClient；当前工作区尚未包含生产方来源，RunWorker保持partial，不能声称可运行。模型CurrentSnapshot+完整结果恢复消费同样仍等根精确API，两个执行器完整恢复保持partial，正常真实新SSE链已实写，不以错误占位计完成。

新HTTP readJobRecovery先合同后实现，jobs-only、path/body ID同值、完整原ref/root/epoch/worker/fence/attempt/cancel事实；已提交完整真实结果可恢复，未执行确证或安全取消确证才safeToRetry，其它pending/unknown，数组不null。新CreateExtraction完整来源/原批准/初次计划/原意图/独立派发闭包源码ready；只执行不新增批准。其余必要helpers按实际真实caller登记，不拿helper数量充原40函数。

必要职责调整：ValidatePublication同ID同算法从build/validate.go移index/publication.go，被build/create/validate、runtime/recovery与extract/execute共同真实调用，避免包环。ExecuteExtraction约束名称/签名不变移extract/execute.go，create.go只拥有接受与冻结参考助手。新BuildMaterialBlocks把完整机器页按真实NFKC边界续读，原页视图≤8MiB、每真实BPE片段4MiB/1Mi字符，不伪造token。ReadGeneration原词项回验误用原text已改RetrievalText，否则所有增强索引被错误拒绝。背景稳定语义改原整组IDs，缓存变化只能原请求冲突而不能新key。

根framework须登记全部新增源码路径与009迁移（见implementation实际files）；旧ValidatePublication路径不是删整个build/validate.go。根负责统一gen/docs/gofmt/build/vet/check和锁文件，本机只读与git diff --check；未Go/SQL/接口/模型验证。完整启动/Ready仍控制/快照/quote/UI静态未齐，不计完成。

本批源交付76caed2→f35238c；ready精确ID（均blocked待CI，含必要助手不充原主函数数量）：

- `knowledge.CreateBuild`
- `knowledge.QueueBuild`
- `knowledge.CreateExtraction`
- `knowledge.PrepareExtractionPlan`
- `knowledge.ReadExtraction`
- `knowledge.NormalizeSource`
- `knowledge.NewIndexRuntime`
- `knowledge.ReadGeneration`
- `knowledge.ValidateBuild`
- `knowledge.ActivateBuild`
- `knowledge.CancelBuild`
- `knowledge.RetireDocumentIndex`
- `knowledge.RecoverBuildLeases`
- `knowledge.RecoverExtractionTasks`
- `knowledge.CollectRetiredIndexes`
- `knowledge.NewWorkerRuntime`
- `knowledge.WithWorkerRuntime`
- `knowledge.NewOperationHandlers`
- `knowledge.WithModelLease`
- `knowledge.PrepareBuildChunks`
- `knowledge.CancelBuildSideEffects`
- `knowledge.ValidatePublication`
- `knowledge.ExecutionLeaseID`
- `knowledge.ApprovedJobInput`
- `knowledge.AdoptClaimedJob`
- `knowledge.SaveExecutionMirror`
- `knowledge.LoadExecutionRecord`
- `knowledge.ReadExecutionRecord`
- `knowledge.MirrorExecutionLease`
- `knowledge.ReadPaidResultRefs`
- `knowledge.CommitExecutionResult`
- `knowledge.RecordExecutionCancellation`
- `knowledge.ResolveTaskMemory`
- `knowledge.CancelUnusedExtractions`
- `knowledge.CancelExtractionSideEffects`
- `knowledge.ReadJobRecovery`
- `knowledge.ReadBuildPlan`
- `knowledge.FreezeBuildPlan`
- `knowledge.SetBuildStage`
- `knowledge.PrepareEmbeddingPlan`
- `knowledge.ReuseEmbeddingBatch`
- `knowledge.CheckEmbeddingPlan`
- `knowledge.CheckExtractionPlan`
- `knowledge.DispatchExtraction`
- `knowledge.BindExtractionViews`
- `knowledge.BuildMaterialBlocks`

partial精确ID：`knowledge.ExecuteBuild`, `knowledge.ExecuteExtraction`, `knowledge.RunWorker`, `knowledge.BuildApplication`, `knowledge.assembleBackgroundTasks`。完整模型恢复原partial函数仍保持既有blocked，待根精确CurrentSnapshot API后继续，不把新SSE成功分支当完整恢复。

补充精确partial：InvokeModel、EnrichChunkContext、DescribeResource的完整旧调用CurrentSnapshot/output消费尚未闭包，最新台账保留partial；正常新SSE可运行不等于完整恢复。根已交jobs筛选d923cca与公共typed源码3f17df5，必须和926fd07及本批一同集成CI；本工作区尚未含它们，RunWorker不提前verified。

## 工程装配c04abe1

补既有planned Dockerfile/Makefile/deploy/service.config.json/deploy/manifest.json。ADR0005端口8106，用户10008、本域独占/data/knowledge，内部DNS依赖与外部secret refs；没有实际secret/环境/运行数据。Node22冻锁check+build，Go1.25 -mod=readonly，UI dist只进镜像/ui。pnpm锁仍必须根CI生成后才能构建，完整UI静态HTTP/portal挂载协议尚未给出，未伪装已发布界面。未本机编译/安装/构建镜像/服务/部署，不增加原业务完成数。

类型同步89ce0be：NormalizeSource内部完整机器页输入/源游标上限8MiB，content真实叶入口仍1Mi字符/4MiB，单真实BPE输出4MiB/1Mi字符不变；ExtractImageDescriptionInput移除源码不使用的profileId，由原冻结计划权威选择实际档位。仅内部参考类型修正，不扩大HTTP/付费批准。data/operations旧未实现陈述已同步实际，所有新改仍待根GitLab。

## 当前对象消费与失败边界e660552

已按根精确API接Outcome.CurrentSnapshot、GatewayInvocationSnapshot及ModelOutput.Text指针；五个严格生产validator、readInference/lookupInference真实依赖及别名全部先合同后源。HasComplete/Winning优先真实当前对象，Output/Text无完整证明保持pending，不拼假SSE。旧无snapshot完整事件只兼容已保存原结果。背景JSON/描述/fast重排统一CompletedText，embedding/native rerank统一CompletedOutput。原key未知不永远缓存不完整旧unknown；平台错误中实际已接受ID/对象仍短控制窗口持久，完整paid输出核winner和全文/结构相同后保原字节，真实费用终态仅投影state。

010持久prepared/dispatch_started，旧记录默认1保守未知；只有0证明尚未RPC。真实租约门禁后、公共RPC前转1，不能以无ID猜零费用。ExecuteBuild/ExecuteExtraction错误边界只原worker/fence保存真实failed/code和完整known refs；最终业务已原子完成优先真实成功。只有jobs真实更高fence新claim且原批准未过期才恢复同一未取消failed阶段，心跳不能复活失败，控制窗口不续费批准。

本地extract failed仅attempt失败，锁外通过根已同意的jobs owner纯服务readJob查九状态；自动原key retry-wait等继续等，dead-letter/cancelled才止，succeeded必须有真实ready/unsupported原结果。未配置原生重排时此前空FastProfileID确不可调用，现从MG目录按可信费用类别选fast/chat；配置原生rerank也核QA类别，不能拿background档位改费用。

本批必须与Root尚待交的公共CurrentSnapshot/Text代码及MG完整文本真实producer、jobs owner readJob一起集成CI，不提前verified。剩余真实业务规则已报根：同build永远原失败intent；新明确批准build如何在无unknown且原任务真终态时免费恢复原paid描述或产生新owner，当前未擅自另收费；原批准过期且无未决费用的owner恢复状态仍pending待根核jobs分类。公共snapshot/quote/UI资产协议也尚未给，完整app继续partial。未本机Go/gen/SQL/接口/模型或新增测试样例。

本批源码ready精确ID（全部blocked待同源GitLab，不是implemented/verified）：

- `knowledge.validateConfiguredSpace`
- `knowledge.NewIndexRuntime`
- `knowledge.RerankCandidates`
- `knowledge.requestRerank`
- `knowledge.ReadContextPrefixes`
- `knowledge.InvokeModel`
- `knowledge.NewModelRuntime`
- `knowledge.CompletedOutput`
- `knowledge.PrepareBuildChunks`
- `knowledge.BeginModelStep`
- `knowledge.CompleteModelStep`
- `knowledge.RecordModelObservation`
- `knowledge.MarkModelDispatch`
- `knowledge.DescriptionText`
- `knowledge.WinningModelSnapshot`
- `knowledge.HasCompleteResult`
- `knowledge.CompletedText`
- `knowledge.AdoptClaimedJob`
- `knowledge.FinishExecutionFailure`
- `knowledge.CheckEmbeddingPlan`
- `knowledge.CheckExtractionPlan`
- `knowledge.ReadExtractionJobState`

ReadJobRecovery/ExecuteBuild/ExecuteExtraction/CreateExtraction继续partial，根新授权的owner Observe/Lookup BG原DTO与失败费用证明闭包下一批实施。没有把新必要helpers充原主函数完成数。
