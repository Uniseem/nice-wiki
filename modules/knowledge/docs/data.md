# 知识入库与混合检索数据

- index_config(config_id,version,enabled,embedding_profile,space,chunk_size,overlap)

- builds(build_id,publication_ref,space_id,state,lease_id,source_hash,generation)

- chunks(chunk_id,build_id,block_id,chunk_hash,text,ordinal,byte_start,byte_end)

- vectors(chunk_id,space_id,dimension,vector_hash,vector)

- terms(chunk_id,language,term,frequency,length)

- active_indexes(document_id,space_id,build_id,revision,visibility_revision)

- extractions(extraction_id,asset_id,asset_hash,extractor_version,text_ref,status)

- inbox(event_id,received_at,state);outbox(event_id,payload,published_at)

状态迁移：Build: queued→fetching→extracting→chunking→embedding→validating→active；任一步→cancelled|failed|superseded。Extraction: queued→running→ready|unsupported|failed。IndexGeneration: staging→active→retired；一次文档/空间只有一份active。

事务规则：聚合变更、版本递增、inbox/outbox同一事务；UTC时间，单调版本，Money为USD整数micros。索引包含owner/siteId/dataEpoch范围；eventId、idempotency key、leaseId唯一约束。写入上限、字段空值和枚举以types.json为准；超出限制返回VALIDATION不部分提交。

## IndexConfig

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| configId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 配置ID |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| enabled | 是 | `{"type":"boolean"}` | 入库启用 |
| embeddingProfileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 嵌入档位 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 不可变空间 |
| chunkTokens | 是 | `{"type":"integer","minimum":128,"maximum":1024,"default":512}` | 默认512范围128..1024 |
| overlapTokens | 是 | `{"type":"integer","minimum":0,"maximum":128,"default":64}` | 默认64范围0..128 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | BCP47源语言 |
| rerankProfileId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 可选重排档位 |

## IndexBuild

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建ID |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 发布引用 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 嵌入空间 |
| state | 是 | `{"type":"string","enum":["queued","fetching","extracting","chunking","embedding","validating","active","cancelled","failed","superseded"]}` | 构建状态 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | staging代数 |
| completedChunks | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 完成数量 |
| totalChunks | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 总数 |
| failureCode | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 安全错误码 |

## Chunk

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| chunkId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 分块ID |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 权威来源 |
| block | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/BlockRef"}` | 源块 |
| ordinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 文内序号 |
| chunkHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范文本hash |
| text | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 仅内部检索正文 |
| tokenCount | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 分块token |
| byteStart | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 源UTF8起点 |
| byteEnd | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 排他终点 |
| asset | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 资源派生来源 |

## Candidate

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| chunk | 是 | `{"$ref":"#/$defs/Chunk"}` | 分块 |
| lexicalRank | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 词项名次0为缺失 |
| vectorRank | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 向量名次0为缺失 |
| score | 是 | `{"type":"number","minimum":0,"maximum":1}` | RRF分数 |
| visibilityRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 权威可见性代数 |

## SearchResult

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 可引用材料 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 所用空间 |
| indexVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 所有active版本摘要 |
| degraded | 是 | `{"type":"boolean"}` | 是否词项降级 |
| truncated | 是 | `{"type":"boolean"}` | 预算是否截断 |

## Extraction

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extractionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提取ID |
| asset | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 资源引用 |
| extractorVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 提取器版本 |
| state | 是 | `{"type":"string","enum":["queued","running","ready","unsupported","failed"]}` | 状态 |
| text | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 内部文本 |
| contentHash | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 提取文本摘要 |
| pageCount | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 页数或帧数 |

## BuildProgress

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建ID |
| state | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 构建状态 |
| completed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 完成单元 |
| total | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 总单元 |
| sequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 事件序号 |

## GatewayMessage

引用：`../../model-gateway/contracts/types.json#/$defs/GatewayMessage`。

## QueryEmbedding

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| queryHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范基准查询 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 完整空间 |
| vector | 是 | `{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536}` | 向量 |
| vectorHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 向量摘要 |
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 原MG调用 |
| dataEpoch | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 代际 |

恢复特殊数据：屏障停止新build、active切换和staging写入，等待reader lease排空；导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建；派生索引可由管理员另发重建任务。

## 2026-10-18 实施收口：独立 SQL 代数与证据映射

本域 schemaVersion 1 的实际迁移为 `migrations/001_initial.sql`，仍只访问本域私有 SQL。`index_identity` 固定站点和代次；`builds` 保存完整发布引用、完整空间、固定嵌入档位和语言、租约、独立 generation、分块计数、清单摘要及一秒内的权威观察时间。`generation_state` 独立记录 staging/active/retired，不把已成功构建的历史结果改成非法 Build 枚举。`active_indexes` 每站点、代次、文档与 spaceId 只有一个指针。

分块、词项和向量均存本域 SQL，不另造第二份索引文件。清单是按 ordinal 排序的完整 Chunk、固定版本词频/长度、向量及 vectorHash 的规范 JSON SHA-256；读者用同一只读事务固定活动指针和全部记录，核对完整空间与清单后将材料载入内存。读事务结束后不持有可删除的索引文件句柄。`ValidateBuild` 对比分块/词项/向量数量、重算确定词项、核每个来源和向量摘要，观察 content 权威后重新打开当前租约写事务，清单未变才保存证明。`ActivateBuild` 在当前安全写窗口内检查观察时间不超过一秒，CAS 预期活动代数，并同事务写 `index.ready` outbox；旧代数保留。

Chunk 新增必填 `sourceOffsets`，长度等于 text 的 UTF-8 字节数，每个值是同一源块原始 UTF-8 字节起点。单个规范化字符可覆盖多个原字符，映射可重复。`byteStart`/排他 `byteEnd` 覆盖该片段全部原字符。拆分输入改成可信 `SourceBlock[]`：每项含单一 BlockRef、规范文本、映射、原文排他终点、模型 tokenizer 给出的严格 UTF-8 tokenEnds 与上下文版本。分块不跨源块合并；长块切分与重叠都按给定 tokenEnds。chunkId 摘要包含完整 BlockRef、可选资源、上下文版本、规范字节区间和 chunkHash，不生成假 blockId，不猜模型分词。实际 `ExecuteBuild` 尚未取得此 tokenizer 前置，仍未完成。

词项分词版本固定 `unicode-code-cjk-bigram-v1`：字母、数字、下划线组成保留大小写的代码词；组合标记跟前词；中日韩连续片段为相邻 bigram，单字片段保留单字；标点和空白分界。BM25 固定 k1=1.2、b=0.75，最多 80 项；向量余弦先按最大绝对值缩放再单位化，排除零向量，最多 80 项；并列按 chunkId。RRF 固定 60，缺失名次贡献零，完整同 ID 内容冲突失败关闭，最多 40 项。所有向量 number 明确 format=double，避免跨服务舍入造成 vectorHash 不一致。

`query_embeddings` 唯一键为站点、代次、可信 rootOperationId、规范 queryHash 和 spaceId，并保存完整 space JSON、首次 operationId 和持久结果。进行中调用在本进程按该身份 singleflight；恢复续读仍使用首次 operationId 和同一 MG 幂等键，不因断线重算。`query_embedding_keys` 将管理员或可信调用者的幂等键绑定到完整身份摘要，同键异输入返回 CONFLICT。结果复用前核完整空间、查询、代次和向量摘要。只有成功后通过当前平台写窗口才能保存；费用和余额仍只由 MG 权威记录。


2026-10-18继续实施：本域OpenRepository使用已选modernc SQLite及内嵌只追加迁移，index_identity固定site/epoch，不把旧库自动重标到恢复新代次。配置首次0→1及后续CAS、安全原回执同事务；开启时从MG读取完整权威embeddingSpace，不复制spaceId算法。所有索引仍只存SQL；retired_at_ns记录真实退役时刻，缺时刻不补造；GC至少24小时后且无active/未完成调度或模型引用才按外键顺序删除。generation_watermarks保留独立高水位，后续GC不重置代数。


继续实施第二批：build_dispatches保存固定BuildJobInput的规范原payload与完整ObjectRef（owner/objectId/version/hash/mediaType/byteSize），以及原root/截止/费用类别等无凭证批准元数据。只有可信background费用类别可排入knowledge.build；不存portal proof、cookie或模型token。jobs RPC在提交后锁外执行，同去重键重试只保存真实接收jobId/operationId；取消竞态仍保留已接受job引用。generation_watermarks不随旧代数GC减少。已付费完成model_steps独立保留，不随索引GC删除。

FetchPublishedSource的source明确为完整canonical DocumentBody JSON UTF8，字节硬上限16MiB，来源hash必须匹配；build_sources只存本域固定源，不读content私有目录。ListExtractableAssets保留SourceAssetVersion和原BlockRef，精确version读取并核hash；公共AssetRef不变，sources额外保存精确源，unsupportedAssets明确不支持类型/大小，不伪空成功。


真实分词片段收口：完整canonical DocumentBody仍16MiB；单真实text/alt叶按当前content Unicode约束最多1Mi字符/4MiB UTF8。NormalizeSource在NFKC稳定迭代边界返回最多4MiB/1Mi字符的片段、原字节范围和nextByte，处理膨胀时不截任意字节。SourceBlock保留sourcePath/sourceByteStart/end，允许同BlockRef的不同真实叶/范围，不生成假块ID，也不声明合成的全块BPE。含asset时路径明确指向不可变提取原文本，sourceOffsets不伪装人工正文。

SplitChunks接受真实BPE字节端点在UTF8内部，仅chunk端点和重叠起点选择完整码点，TokenCount按真实token序号差；每块≤64KiB，单批仍≤256，nextCursor必须续读，ordinal保持全构建顺序。新Chunk.sourcePath为兼容可选，旧active JSON未改且旧无path ID算法保留。启动LoadRuntime只读核真实SQL/所有活动清单，损坏关闭就绪不自动重算。新build前读可信DataRoot真实Statfs，≤10%拒新构建，原结果复用不受影响。

真实原块文本视图明确为最近稳定块直接拥有的text/attrs.alt原叶按正文content顺序串联，不添加分隔符、嵌套稳定块独立起算；sourcePath是实际JSON字符串指针，sourceOffsets指未转义原文字节。BuildSourceBlocks每次只返回一个原叶的一个有界NFKC稳定片段，Next必须续读，避免全正文膨胀偏移映射同时积压。原块ID/hash不改，contextVersionHash为文档/块/预处理固定规范摘要，不表示已调用上下文模型。所有片段由MG真实免费BPE核固定空间与原文hash，绝不把rune当token。

model_steps现在真实由所有本域InvokeModel入口写入：付费前持久去凭据请求hash、原op/key/owner；内部真实事件先保存invocationId，完整所有终态与result_hash保留≤16MiB，失败/费用未知原样复用，不另建调用。许可令牌不入请求摘要或DB。构建以真实buildId为owner，QA默认可信root；新窗口核本站同代。崩溃/网络未知保留dispatching/running与原MG幂等键，不能作为safeToRetry的新付费授权，不能绕改op/key。004仅追加结果hash列，完成付费结果不随index GC删除。

上下文按产品基线必需：整篇固定正文canonical JSON hash核对后作模型固定消息，超过65536字符/模型预算失败关闭，旧active/付费结果保留，无未授权基础索引降级。contextPrefix/contextVersionHash/embeddingTokenCount是兼容可选机器派生字段，原text/hash/sourceOffsets不动。完整检索文本=非空prefix+换行+原text，固定embedding空间真实BPE计数，词项/embedding共同使用，引用仅原text。背景缓存含完整源/记忆/模型声明及真实原块/path/区间身份；同源付费前独占原build/root/op意图，其他构建等待原结果不再花费。005为只追加迁移。TokenizeSource保持ID移到internal/model/tokenize.go，由source与index复用模型raw+严格生产schema，避免包循环。

视频元数据经根确认从固定权威正文attrs.title/description读取，sourceKind仍原uploaded/platform；不调用插入时URL抓取API。metadata有真实题名简介可ready，当前schema无字幕，transcriptState明确unsupported、无转写。平台视频Extraction.asset兼容可选，sourceBlock保留原块，不拿poster冒视频。原块文本视图顺序明确text有序content→alt→仅video的title→description，直接串联原叶无分隔符，嵌套稳定块独立起算；sourcePath是实际JSON叶字符串。ReadSections新增content sourcePath回读精确私有发布快照hash/原块/叶区间，资源提取原文由BindExtractionViews在当前原构建租约下持久绑定真实提取页/描述。PDF授权下载、公共沙箱及真实main子入口均已写，待GitLab。

006新增资源派生原文视图表，完整site/epoch/publication/BlockRef/AssetRef/sourcePath作用域与原text/hash/source_start独立保存。ReadSections消费者已支持真实人工原叶与机器提取原文两条精确回读链，缺视图NOT_FOUND而非捏造来源。ExecuteExtraction保存独立原提取结果，BindExtractionViews把真实结果绑定当前原构建的精确授权源，消费者按实际视图核验；机器文本不冒充人工caption。完整旧模型恢复消费仍partial。

007使model_steps按owner+稳定SemanticKey唯一，原MG key/op保留；SemanticKey只含原chunk/batch/资源或QA规范查询身份，不含可变配置。原请求hash去许可/临时幂等hash，同语义输入/记忆/配置或原op漂移CONFLICT，不新key绕旧unknown。旧记录只有原op/完整旧请求hash一致才绑定语义并保原MG key，不能证明则拒绝新付费。build_context_plans首次持久完整任务记忆/档位/声明模型及hash，后续漂移在新上下文意图前拒绝；已付费结果继续保留。

模型实际来源只读MG resultAttemptId所指真实attempt.modelSnapshotHash；ReadModels同算法opaque hash只作比较，本域不复制算法、不按attempt.state猜winning。背景缓存逐项绑定实际snapshot/完整源/真实记忆，机器Chunk保留contextModelSnapshotHash；兼容整批contextVersionHash只是请求计划摘要，每项prefixContextHashes才是实际版本。当前任务覆盖声明只限定缓存资格，已付费旧缺证明记录保留而不猜版本复用。008追加实际证明字段。unknown若无完整有效输出仅原MG key继续观察；有效输出与费用状态分离，费用未知仍MG held，原完整输出不因对账改来源。公共当前snapshot/完整输出恢复API根正在补，域不拼假SSE。

### 原任务执行、提取与恢复证明

009只追加本域私有表：execution_records保存jobs真实原job/inputRef/payload/root/op/worker/fence/attempt和权威租约镜像；首次及续约镜像只来自平台真实Heartbeat回执。原批准截止不被claim或控制回执延长。最终业务状态与完整结果字节、真实ObjectRef和完整paidResultRefs同事务提交；jobs完成回执未知只重交原结果。原结果拥有者证明不随退役索引GC删除。

knowledge.build与knowledge.extract由两个平台循环按生产方jobTypes筛选，固定并发1与3，总4；构建等待同一个独立提取ID，不inline、不快速失败耗尽孩子重试。类型筛选生产方/公共worker由根统一提供，未集成前整个worker保持blocked。取消先关闭原写资格并记录原意图；仅在没有其他仍有效批准构建引用时取消共享提取。未知模型与接受结果保留原引用，取消清理只原job/key，不推退款。

提取首次冻结精确AssetSource、任务记忆、档位/任务覆盖和MG opaque模型声明；重放先找原来源关联。新owner只来自原已批准构建，不随unknown或记忆漂移自动产生。只复用原asset hash/MIME/长度/提取器/任务记忆版本与当前声明覆盖的真实winning模型结果。PDF免费真实隔离解析；文本附件/图像按原批准任务；视频元数据只读原发布快照，字幕unsupported。源授权关联单独持久，缓存不把原结果错误绑定到未授权新资源。

机器页/描述原文视图与人工DocumentBody分开。路径为`/extractions/{extractionId}/pages/{原页号}`或`/extractions/{extractionId}/text`，各真实原页/描述的UTF8字节区间独立从0起算；不把人工正文叶计数与机器页静默混合。单页/描述完整原文≤8MiB；规范化按真实稳定边界续读，单BPE片段≤4MiB且≤1Mi Unicode字符。整个构建原文字材料和完整阶段JSON各有16MiB运行硬预算；超过失败关闭，不能截断来源。PDF页号/正文哈希和原BlockRef保持；视频题名/简介已有真实正文叶，不重复做机器文字副本。

构建冻结raw chunks、首次任务记忆、增强chunks与真实模型嵌入批次。嵌入当前采用固定单块批次（不超过公共64上限），避免部分缓存出现后改变原分组绕unknown；完整检索文本真实BPE，原完整空间与真实模型窗口付费前硬核。向量复用要求完整RetrievalText与完整空间一致并核向量hash/维度。词项写入与代数回验均按同一RetrievalText。背景模型语义使用原整组chunk IDs，即使缓存集合变化也不能换新语义绕过旧unknown。

`POST /v1/jobs/{jobId}/recovery`仅jobs纯服务，路径/正文ID一致，核完整原来源和失效租约。completed只返回本域真实最终结果；新版入口证明从未执行且无阶段事实，或原模型全已知终态且完整结果hash核对、子任务均终态并取得完整MG费用证明时才safeToRetry并继续原job/key/冻结阶段计划；原批准过期只供jobs分类，不续付费批准；已关闭取消写资格且同样终态证明时由jobs直接终止（jobs沿原cancel事实终止）。其余pending/unknown，不猜零费用，不授权新owner或新key。paidResultRefs始终非null完整数组。

### 原模型观察、未发出证明和失败诊断

010只追加model_steps.dispatch_started：全部旧记录默认1保守视为可能已经RPC。新稳定语义先prepared/0；真实jobs租约门禁通过后、公共InvokeModel前原子记dispatching/1。只有0可证明尚未发出，不能以缺InvocationID推零费用；1的网络未知仍原key/原请求观察。当前模型目录漂移在尚未外发阶段仍CONFLICT，不以prepared行冒充旧MG已固定路由。

公共CurrentSnapshot只消费真实对象：完整Invocation/resultAttemptId/opaque模型hash和可选完整Output；Text为指针区分完整空文字与缺失，上限64KiB，不能拿部分delta补成当前完整输出。五种生产方回执严格校验，原key读查询及fresh只读期限由platform统一拥有。已保留完整paid输出不可被新观察覆盖；最新真实费用终态仅投影本域步骤state，原完整输出字节/hash独立保留。错误返回只要有真实已接受ID/CurrentSnapshot也用短控制窗口保存，不能让create已接受但后续取消丢失引用；没有真实ID仍保留未知意图。

执行失败边界在原worker/fence下保存真实failure_code/state和完整已知invocation refs。已经原子完成则返回真实原成功结果，不因其后取消改写。只jobs真实更高fence新claim且原批准未过期能恢复同一未取消failed对象阶段；续约不复活失败，取消/已完成不复活；控制元数据窗口不延长原模型批准。

QA重排未指定原生档位时从MG真实目录按可信费用类别选fast/chat，不使用未装配空profile；原生档位和管理配置都核QA类别。完整输出消费统一CurrentSnapshot.Text，真实格式不合格才既有一次格式attempt，不因为费用未知或缺输出另开调用。


### 后台原请求只读恢复与费用终局

011仅对明确background owner首次prepared/0冻结完整原Create DTO（实际持久key、去AdmissionPermit）；QA正文与许可不持久。原semantic输入/op/root/site/epoch不变。owner锁外调用platform.ObserveInvocation/LookupModelInvocation，仅只读原ID或完整原body/key，不调用可能Create的InvokeModel，不取消模型，也不延付费批准；每次至多16项，完整SQL未决集合仍阻止错误safeToRetry。旧无body且无ID保持pending。

原result_json/hash保留完整paid输出不可变；observation_json/hash保存最新真实对象，费用投影只来自Invocation与全部attempt。Outcome.completed仅是完整结果可消费，即使fee unknown也可免费完成本域业务并保留完整paidRefs。OwnerRecovery.completed不假称结算；safeToRetry额外需要原root完整QueryUsage真实显式windowCoversRoot/allInvocationsFinal、held USD0、空unknown与每条已知价格/actualCost，缺字段、旧epoch或响应超预算均保持pending。跨根已完成缓存只免费复用不新授权，原paidRefs完整保留。

普通NOT_FOUND不是负证明。仅MG专用INVOCATION_NOT_ACCEPTED可保存原scope/inputHash/key/时点的not_accepted观察，dispatch_started仍1；它不证明永久无费、不能批准新owner，原RPC晚提交仍同原key去重。原批准有效才可继续原key/原输入，派发前清除旧时点负证明。观察和费用IO后重核原worker/fence/attempt/cancel及实际租约；原批准过期且无未决副作用的safeToRetry仅供jobs按DEADLINE_EXCEEDED分类，实际worker原付费工厂仍拒绝过期批准。


### 稳定资源意图与免费恢复

资源付费拥有者意图固定asset hash/MIME/size/task/extractor语义，不把可变记忆/模型/价格当新意图键。旧按计划hash意图保留全部行，读取真实source/plan并核ApprovedJobInput完整引用hash，按最早真实owner建立语义索引；并发归属变化CONFLICT。同build先读原origin永远原extraction/原key。新build优先免费复用当前记忆和MG真实winning快照覆盖的完整paid输出；012独立保存原输出hash证明派生artifact，不改旧失败任务、执行事实、MG费用或伪造jobs完成。

无可用完整结果时，仅新明确批准build且旧jobs真实dead-letter/cancelled/succeeded终局、完整原root费用无未决才可CAS语义意图产生新owner；旧结果、关联和引用全部保留。任一已外发但缺真实接受ID（包括时点INVOCATION_NOT_ACCEPTED）不能授权新owner，防止原RPC晚接受；旧不同计划未终局CONFLICT，不能用记忆/档位变化绕旧unknown。

### 全量估价与明批

013保存固定完整公开树/发布修订、索引配置、任务记忆、真实MG路由候选/routeVersion和逐候选QuoteCost原PriceSnapshot；计划24小时，原operation同输入回原回执。估价只免费读取、真实BPE、核缓存和QuoteCost，不建资源/paid意图或调用模型。同步完整预算256文档、4096真实分块、原文含资源和策略各16MiB，超预算失败不截断。

已知材料按相同真实分块/原背景/完整检索文字和完整空间回验向量，扣实际复用。未来资源文字尚未产生则chunkCountKnown=false，chunkCount仅已知材料扣复用的数量；quote.pricingKnown=false并省maximumCost，保真实已知分项和明确warning，不捏token/分块数/总价。逐候选输入使用其真实声明完整模型窗口上界，输出按既定任务800/500/0，整数micros逐真实报价乘数量累加，不用一条PriceSnapshot冒全量。

MG免费resolveTaskRoute只读HTTP复用其统一路由，知识不筛eligible或复制route/model hash。候选同快照给真实opaque modelSnapshotHash，embeddingSpace五字段由MG权威产生。明批保存原routeVersion与expectedPriceSnapshots；所有相关Create传期望，由MG接受事务重新核整候选（包括disabled→enabled）和当前价格CAS，客户端预读只诊断，不冒原子门禁。未知总价可在明确提示后由管理员显式批准，仍按实际额度规则执行，不自动补总价/重估或换原批准。

批准先核即时knowledge.manage、原管理员/本站同代、24小时和quoteHash，锁外重新核完整来源/配置/模型/记忆/报价/缓存；变化CONFLICT。安全写事务中一次创建全原集合force新代数和原明批策略/批准人/root/时点/回执，任一失败全回滚；锁外仅派发已提交原任务。原批准后漂移CONFLICT，不新key绕过。旧active保留至新代数真实核验并激活。

拥有者安全证明前，免费恢复artifact必须逐hash核原真实模型结果、实际winning模型、完整文字和原asset/块来源。完整免费缓存不要求旧任务执行回执，但不伪造旧jobs completed；原paidRefs完整保留。跨根已完成缓存可免费续用，同根费用仍由完整RootFeesFinal涵盖，不因artifact存在推零费。
