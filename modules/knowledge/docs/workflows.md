# 知识入库与混合检索算法与工作流

状态机：Build: queued→fetching→extracting→chunking→embedding→validating→active；任一步→cancelled|failed|superseded。Extraction: queued→running→ready|unsupported|failed。IndexGeneration: staging→active→retired；一次文档/空间只有一份active。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `content.publication.changed` | 1 | `knowledge.HandlePublication` | 消费content.publication.changed执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `content.publication.withdrawn` | 1 | `knowledge.RetireDocumentIndex` | 消费content.publication.withdrawn执行本域失效/任务/批准回执；不同步反向请求生产方。 |

### 异步任务

#### `knowledge.build`

- 执行函数：`knowledge.ExecuteBuild`
- 触发：HandlePublication、CreateBuild、RecoverBuildLeases创建

1. FetchPublishedSource取得固定发布修订；ListExtractableAssets列出引用媒体，未有派生文本的资源CreateExtraction排队并等待其完成或标unsupported
2. NormalizeSource→SplitChunks→EnrichChunkContext（仅变更块付费）→WriteStagingChunks写独立代数
3. BuildLexicalIndex；RequestEmbeddings分批后StoreEmbeddings，复用同chunkHash/空间已有向量
4. ValidateBuild后ActivateBuild原子切换；修订已被替代则标superseded；ActivateBuild在同一事务登记index.ready outbox

#### `knowledge.extract`

- 执行函数：`knowledge.ExecuteExtraction`
- 触发：ExecuteBuild经CreateExtraction创建

1. 读取提取记录与资源hash，按MIME分派：PDF→ExtractPdfText，图片/画板/视频封面→ExtractImageDescription，视频→ExtractVideoMetadata，其他附件→DescribeAttachment
2. 结果按assetHash/extractorVersion/模型/记忆版本保存为机器派生文本，不覆盖人工caption
3. 不支持或无文本标unsupported，不自动OCR或转写

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `knowledge.LoadRuntime` | 启动时 | 启动索引服务 |
| `knowledge.RecoverBuildLeases` | 启动时及每60秒 | 恢复入库执行租约 |
| `knowledge.CollectRetiredIndexes` | 每10分钟 | 回收失活索引代数 |
| `knowledge.PublishOutbox` | 启动后持续，空闲每1秒 | 发布索引生命周期事件 |
| `knowledge.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `knowledge.RunWorker` | 启动后持续 | 主动领取本域持久任务 |
<!-- flows:end -->

## 2026-10-18 已编写链路

已编写分块→staging→词项/向量→完整清单→一秒内权威证明→CAS 活动指针与 outbox 的各用例，以及只读活动快照→混合召回→权威过滤→可选重排→整块预算选择→最终权威过滤的检索链。SQL 迁移和完整服务启动尚未装配，后台 `ExecuteBuild`、资源提取及 tokenizer 来源仍未完成；这不是已可运行的服务。

模型调用只经 `platform/modelclient`。网关成功终态的 `output` 承载完整空间和向量，原生重排显式传 query，texts 与 candidateIds 对应；聊天 delta 只用于 fast 核查的 JSON ID 排列。最多 20 项重排，fast 最多 800 输出 token；格式不合格只追加一次独立 attempt，仍不合格保原顺序；收费未知与依赖失败不自动重放。材料超过网关消息预算返回 VALIDATION。开启重排由可信配置决定，未开启时不调用模型。

`NewOperationHandlers` 只提供 searchKnowledge、readSections、createQueryEmbedding、getBuild、listBuilds 和 redactDiagnostics 六个真实映射。它们必须交给平台 `NewServiceHandler`，由其一次完成服务身份、委托、权限、代次、维护和严格请求校验；处理函数从可信上下文取得本域依赖。其他操作缺失时完整应用仍必须启动失败，不用 503 占位宣称已交付服务。
