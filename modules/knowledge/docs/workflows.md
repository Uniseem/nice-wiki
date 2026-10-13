# 知识入库与混合检索算法与工作流

状态机：Build: queued→fetching→extracting→chunking→embedding→validating→active；任一步→cancelled|failed|superseded。Extraction: queued→running→ready|unsupported|failed。IndexGeneration: staging→active→retired；一次文档/空间只有一份active。

## LoadRuntime

启动索引服务。

1. 1 检查DB与索引文件manifest
2. 2 核验空间/哈希一致
3. 3 不一致active标unavailable
4. 4 注册租约恢复

## PutIndexConfig

更新索引配置。

1. 1 校验chunkTokens/overlap
2. 2 解析模型空间声明
3. 3 空间变化只生成新代数不覆盖active
4. 4 CAS保存

## HandlePublication

接受权威发布事件。

1. 1 inbox按eventId去重
2. 2 验证dataEpoch
3. 3 配置关闭或未配置档位则记录skipped
4. 4 创建唯一document/revision/space build与job outbox同事务

## CreateBuild

人工或事件申请构建。

1. 1 向content验证visible/current
2. 2 固定config/space
3. 3 同修订同空间幂等
4. 4 force新generation且保留旧active

## FetchPublishedSource

取得权威正文。

1. 1 校验build租约及deadline
2. 2 调content只读发布对象
3. 3 对比contentHash
4. 4 保存私有临时source引用

## ListExtractableAssets

筛选发布引用媒体。

1. 1 按正文schema解析引用
2. 2 media批量授权验证
3. 3 限制每文档100资源单件50MiB
4. 4 不支持类型标明unsupported

## CreateExtraction

按hash幂等提取资源。

1. 1 核验允许MIME
2. 2 按asset hash/extractorVersion去重
3. 3 记录来源授权
4. 4 排队独立租约

## ExtractPdfText

提取PDF可用文字。

1. 1 media授权流下载并验证hash
2. 2 在无网络沙箱读取最多500页/50MiB
3. 3 保留页码按阅读序提取
4. 4 无文本标unsupported等待显式OCR任务

## ExtractImageDescription

显式生成图片检索描述。

1. 1 发布构建显式视觉描述配置
2. 2 根knowledge.describe-visual使用vision档（未设采用fast视觉能力）800output token
3. 3 图片/画板/视频封面assetHash+task/model/memory version幂等复用
4. 4 保存machineDerived仅检索不覆盖人工caption，取消旧revision后不写。

## ExtractVideoMetadata

提取视频题名及授权字幕。

1. 1 从media取得已授权平台元数据
2. 2 仅收录已有字幕与题名
3. 3 校验语言/编码
4. 4 无字幕不自动转写

## NormalizeSource

稳定规范化输入。

1. 1 解析块级纯文本
2. 2 Unicode NFKC并统一LF
3. 3 保留段落和代码大小写
4. 4 构建offset映射不丢弃引用源位置

## SplitChunks

按源块及token边界分块。

1. 1 同小节相邻块合并默认512token但不跨来源权限
2. 2 记录全部blockIds与原UTF8偏移
3. 3 图片描述+图注+相邻块，附件长文本再切分
4. 4 chunk identity包括源block hashes与上下文版本，不将派生描述写回人工正文。

## WriteStagingChunks

保存独立分块代数。

1. 1 校验每chunk来源revision
2. 2 验证重复ID同hash
3. 3 一事务批量≤256
4. 4 不修改active pointer

## BuildLexicalIndex

建立词项索引。

1. 1 固定Unicode词界分词版本
2. 2 中日韩使用连续bigram
3. 3 保留代码标识词
4. 4 写freq/length及BM25统计

## RequestEmbeddings

请求分块嵌入。

1. 1 读取固定chunkHash
2. 2 64项且总输入≤模型预算
3. 3 继承ctx调用MG
4. 4 对每返回space和dimension严格校验

## StoreEmbeddings

持久化验证过的向量。

1. 1 等长和维度检查
2. 2 拒绝非有限数
3. 3 校验lease/chunkHash未变
4. 4 原子写vectorHash与向量

## ValidateBuild

验证完整索引代数。

1. 1 count chunks==terms==vectors
2. 2 检查每chunk空间/hash
3. 3 校验索引文件checksum
4. 4 向content再次验证current/visible

## ActivateBuild

原子切换当前索引。

1. 1 读取权威验证结果且有效期≤1秒
2. 2 再校验dataEpoch
3. 3 文档/space锁CAS active generation
4. 4 成功写active及index.ready否则superseded

## CancelBuild

取消入库链路。

1. 1 设取消标记
2. 2 取消jobs及所有MG invocation
3. 3 取消staging写权限
4. 4 保留active与收费未知引用

## RetireDocumentIndex

响应撤回/删除。

1. 1 inbox去重
2. 2 只接受更高visibilityRevision
3. 3 active→retired
4. 4 取消相关build并事件失效

## NormalizeQuery

规范检索请求。

1. 1 NFC/LF
2. 2 trim首尾空白但保留代码大小写
3. 3 上限2048字符/8192字节
4. 4 空查询拒绝

## EmbedQuery

取得匹配空间查询向量。

1. 1 检查MG档位空间完全等同
2. 2 ctx继承调用
3. 3 dimension与有限数检查
4. 4 取消后不返回向量

## SearchLexical

执行全文召回。

1. 1 只读active generations快照
2. 2 BM25 k1=1.2 b=0.75
3. 3 按score降序chunkId稳定排序
4. 4 取最多80项

## SearchVector

执行向量召回。

1. 1 空间完全匹配
2. 2 cosine距离并排除零向量
3. 3 只active索引
4. 4 分数降序chunkId稳定排序最多80

## FuseCandidates

确定性融合两种召回。

1. 1 以chunkId去重
2. 2 score=1/(60+lexicalRank)+1/(60+vectorRank)缺失项0
3. 3 hash冲突拒绝
4. 4 score/chunkId稳定排序40项

## ValidateCandidates

权威过滤检索材料。

1. 1 按PublishedRevisionRef去重
2. 2 content批量validate
3. 3 visible且matchesCurrent且dataEpoch一致才留
4. 4 写visibilityRevision并拒绝不可用结果

## RerankCandidates

可选AI重排。

1. 1 使用rerank档，未设时fast核查排序
2. 2 同候选最多20个且固定ID
3. 3 继承qa root/costClass与共享截止
4. 4 输出只能既有ID排序，最多一次格式重试再有序fallback每次独立MG预留。

## SelectContext

选择答案材料。

1. 1 每文档最多3块
2. 2 按排序累计token
3. 3 超预算跳过整块不截断引用偏移
4. 4 记录truncated

## Retrieve

统一混合检索。

1. 1 query与base language规范
2. 2 embedding存在时校验queryHash/space/vectorHash/epoch后复用，否则MG生成
3. 3 lexical/vector各80→RRF40→可选rerank fast fallback
4. 4 当前页权威范围可优先且最终content可见性validate。

## GetBuild

读取入库状态。

1. 1 验证knowledge.manage
2. 2 读build快照
3. 3 不返回源全文
4. 4 状态未知不自动重跑

## ListBuilds

列入库任务。

1. 1 权限检查
2. 2 按buildId稳定keyset
3. 3 limit默认20
4. 4 返回source ref与错误码

## ConsumeInbox

事件去重与代际保护。

1. 1 校验事件类型/schema
2. 2 dataEpoch==runtime
3. 3 eventId/hash唯一
4. 4 同事务登记业务执行结果

## PublishOutbox

发布索引生命周期事件。

1. 1 租约领取批次
2. 2 jobs持久发布
3. 3 ACK才标记
4. 4 有限三次退避后留待下一轮

## RecoverBuildLeases

恢复入库执行租约。

1. 1 到期worker停止写
2. 2 content验证当前发布
3. 3 已失效标superseded
4. 4 确定性已完成阶段复用，仅缺失embedding在原调用可确证时重续

## CollectRetiredIndexes

回收失活索引代数。

1. 1 检查retired且无reader lease
2. 2 无未完成build引用
3. 3 删除本服务索引文件并校验路径在私有目录
4. 4 删除元数据事务

## RedactDiagnostics

返回安全诊断。

1. 1 只返状态/计数/版本
2. 2 不返全文或媒体凭证
3. 3 检查权限
4. 4 固定诊断无模型探测

## ReadSections

按引用读取证据片段。

1. 1 service身份
2. 2 chunk必须active且来源与传入ref相同
3. 3 content批量校验current/visible/epoch
4. 4 返回完整sourceOffsets与hash

## AcquireBarrier

取得本地维护屏障。

1. 1 校验服务身份及superadmin委托或限定RecoveryAuthorization
2. 2 将本地control.db屏障draining并拒绝新业务写
3. 3 屏障停止新build、active切换和staging写入，等待reader lease排空
4. 导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建
5. 派生索引可由管理员另发重建任务。
6. 4 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。

## ReleaseBarrier

释放已授权维护屏障。

1. 1 校验相同barrierId/operationId恢复许可
2. 2 不允许未决prepared stage直接释放
3. 3 runtime dataEpoch与最终commit/rollback结果一致
4. 4 released后开放该epoch业务写。

## ExportSnapshot

导出安全watermark快照。

1. 1 验证safe屏障及dataEpoch
2. 2 checkpoint本域私有数据并固定revisionWatermark
3. 3 屏障停止新build、active切换和staging写入，等待reader lease排空
4. 导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建
5. 派生索引可由管理员另发重建任务。
6. 4 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。

## ValidateSnapshot

只读检查候选快照。

1. 1 校验owner/对象SHA256/byteSize且不接受本地任意路径
2. 2 schemaVersion仅支持1否则UNSUPPORTED_SCHEMA
3. 3 检查本域字段/唯一约束/引用/密钥兼容性
4. 4 返回field issues无业务写，任何integrity失败valid=false。

## PrepareSnapshot

隔离准备恢复数据。

1. 1 验证限定restoreId/barrierId/旧新epoch/audience/期限许可及safe屏障
2. 2 重复validate并读可信ObjectRef
3. 3 写独立staging DB/文件且不覆盖control.db、屏障、信任锚
4. 4 staging checksum一致才ready并记录恢复前指针。

## CommitSnapshot

原子激活隔离恢复数据。

1. 1 校验stage属于许可restore且目标epoch匹配
2. 2 safe屏障下禁止业务worker
3. 3 原子切换本域数据指针并持久旧指针
4. 4 committed幂等回执，屏障仍safe等待全域runtime epoch激活。

## RollbackSnapshot

回退未完成全域恢复。

1. 1 验证同一许可restore/barrier
2. 2 读取control保存旧指针
3. 3 原子恢复旧数据且stage rolled-back
4. 4 回执旧dataEpoch并保持safe，重复rollback返回同回执。

## EnrichChunkContext

发布后补检索片段上下文。

1. 1 task knowledge.context使用fast档
2. 2 同文整篇原文为固定输入前缀，最多16片段一批
3. 3 生成1至2句产品/功能/小节背景不改变源正文
4. 4 按chunkHash+documentHash+memory版本幂等复用，只为变更块付费

## InspectEvidence

快速检索员有限核查。

1. 1 根task qa.inspect owner=knowledge fast档max800output token并注入project overview
2. 2 只选择本次检索合法IDs，展开小节仍content当前校验
3. 3 3轮/5工具调用计数来自可信根任务状态CAS不由浏览器覆盖
4. 4 到上限用已有材料返回finished，费用继承qa costClass。

## CreateQueryEmbedding

统一生成并复用查询嵌入。

1. 1 rootOperationId/queryHash/space/dataEpoch singleflight且唯一
2. 2 根knowledge.embed embedding档0文本output
3. 3 MG保留独立attempt费用证据
4. 4 返回完整空间/vectorHash供QA cache与本域search复用

## DescribeAttachment

发布附件描述与非文本提取。

1. 1 支持文件MIME/大小白名单且media授权
2. 2 文本附件根knowledge.describe fast档800tokens
3. 3 非文本附件根knowledge.extract-attachment vision文件能力1500tokens，不支持明确unsupported
4. 4 assetHash/extractor/model/memory版本复用，描述完整再长文分块。

## RunWorker

主动领取本域持久任务。

1. 1 向jobs claimJob仅领取本域JOB_HANDLERS登记jobType
2. 2 每10秒heartbeatJob续30秒租约并检查cancel/deadline
3. 3 依jobType执行同域已登记函数，所有中间/提交校验lease与epoch
4. 4 completeJob/failJob持久报告，lease过期停止写，未知MG attempt留held不重放。

共通恢复顺序：AcquireBarrier全部域safe→Export或Validate→Prepare全部stage→Commit全部域指针→site-config transitionRuntimeState激活目标dataEpoch→ReleaseBarrier；任一prepare/commit失败在屏障内Rollback已提交域，回旧epoch后才释放。

任务执行由本域RunWorker主动claim/heartbeat/complete，jobs不调用影子execute-task接口。

| jobType | 本域入口函数ID |
|---|---|
| knowledge.index | knowledge.FetchPublishedSource |
| knowledge.extract-pdf | knowledge.ExtractPdfText |
| knowledge.describe-visual | knowledge.ExtractImageDescription |
| knowledge.describe-attachment | knowledge.DescribeAttachment |
