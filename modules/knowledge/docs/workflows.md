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

1. 精确原job/ref/root/worker/fence读已批准完整BuildJobInput；已完成只返回原持久结果；付费前与最终写前使用平台CheckLease
2. FetchPublishedSource核完整固定源；背景短句按章节分批（全文目录+所在章节原文，按背景模型窗口定批大小），长文档不因整篇长度失败；真实提取独立任务只等待同原ID
3. 完整raw chunks、首次真实task memory、增强chunks与固定单块embedding batches各阶段只冻结一次；背景语义用原整组ID，配置漂移不能绕原unknown
4. 原文字/offsets不改，机器context单独保存；同完整retrieval text/space真实向量复用，未命中按固定原单块批次付费
5. 完整词项/向量/hash/一秒content权威证明后ActivateBuild，active/outbox/最终原JobExecutionResult同事务；jobs回执未知后只重交原真实结果
6. 错误边界持久当前原fence真实failure_code/state并保完整已知paid refs；如果最终业务已原子完成则只重交真实成功结果，不覆盖；控制元数据窗口不延长原批准

#### `knowledge.extract`

- 执行函数：`knowledge.ExecuteExtraction`
- 触发：ExecuteBuild经CreateExtraction创建

1. 真实独立jobs租约读取完整原ExtractionJobInput；核原发布当前性、精确媒体version/hash授权及冻结任务计划
2. PDF公共隔离runner真实提取，图片真实视觉，文本附件完整描述，上传视频只原可信正文元数据；未声明格式/无文字unsupported，无自动OCR/字幕/URL重抓
3. 新付费前通过WithModelLease回核jobs；原owner/稳定语义/原记忆档位/key找回，unknown绝不新owner或换配置重算
4. 核当前镜像fence/lease/cancel与原计划，提取结果/hash/实际模型证明和最终JobExecutionResult同事务；人工caption不动
5. 错误边界持久当前原fence真实failure_code/state并保完整已知paid refs；如果最终业务已原子完成则只重交真实成功结果，不覆盖；控制元数据窗口不延长原批准
6. 来源明确unsupported时直接真实unsupported，不下载超限资源或调用模型；真实模型付费前再次CheckExtractionPlan原声明，已外发仅原key观察

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `knowledge.LoadRuntime` | 启动时 | 启动索引服务 |
| `knowledge.RecoverBuildLeases` | 启动时及每60秒 | 恢复入库执行租约 |
| `knowledge.CollectRetiredIndexes` | 每10分钟 | 回收失活索引代数 |
| `knowledge.PublishOutbox` | 启动后持续，空闲每1秒 | 发布索引生命周期事件 |
| `knowledge.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `knowledge.RunWorker` | 启动后持续 | 主动领取本域持久任务 |
| `knowledge.RefreshSiteOverview` | 启动后每分钟检查，距上次生成满24小时或从没生成过时在后台重算 | 立即重算全站概括 |
<!-- flows:end -->

## 2026-10-18 已编写链路

已编写分块→staging→词项/向量→完整清单→一秒内权威证明→CAS 活动指针与 outbox 的各用例，以及只读活动快照→混合召回→权威过滤→可选重排→整块预算选择→最终权威过滤的检索链。SQL 迁移和完整服务启动尚未装配，后台 `ExecuteBuild`、资源提取及 tokenizer 来源仍未完成；这不是已可运行的服务。

模型调用只经 `platform/modelclient`。网关成功终态的 `output` 承载完整空间和向量，原生重排显式传 query，texts 与 candidateIds 对应；聊天 delta 只用于 fast 核查的 JSON ID 排列。最多 20 项重排，fast 最多 800 输出 token；格式不合格只追加一次独立 attempt，仍不合格保原顺序；收费未知与依赖失败不自动重放。材料超过网关消息预算返回 VALIDATION。开启重排由可信配置决定，未开启时不调用模型。

`NewOperationHandlers` 只提供 searchKnowledge、readSections、createQueryEmbedding、getBuild、listBuilds 和 redactDiagnostics 六个真实映射。它们必须交给平台 `NewServiceHandler`，由其一次完成服务身份、委托、权限、代次、维护和严格请求校验；处理函数从可信上下文取得本域依赖。其他操作缺失时完整应用仍必须启动失败，不用 503 占位宣称已交付服务。


继续实施：发布/撤回只接受公共订阅边界已按生产schema校验的原EventEnvelope，信封与payload的site/document/epoch/visibility必须一致。ConsumeInbox移到index包供两个handler同事务使用，避免events/build/index包循环。配置停用或未配置只消费跳过，不反向同步请求content。撤回关闭active及staging写资格、记录取消意图和逐真实空间的index.retired，同inbox/水位/outbox一起提交。

提交后才锁外请求jobs与公共模型网关取消，不退费、不删除未知引用。重放原撤回仍继续原取消意图，旧事件不会取消新发布的构建。派发先持久dispatching再RPC；pending尚未派发可关闭，接受未知保留状态并失败关闭，不伪造已清理。完整恢复将复用原job/模型调用，不以新dedupe绕过orphan。

事件消费及实际确认统计复用平台NewEventConsumer/RunConsumerWithResult；outbox单批复用NewEventPublisher/RunRelayBatch。schema只在公共订阅边界核一次，handler提交后才确认，失败不确认，退出统计不填假零。PublishOutbox pending只表示本批已领取未确认数量，不声明全域积压。平台同eventId异hash隔离提交语义缺口已交根统一处理，本域只失败关闭，不虚称隔离已持久。


管理UI使用portal注入的非序列化ModuleAPIClient，认证/CSRF/刷新由壳持有。本域不读token、不裸fetch。概览并行读取真实配置、模型/公开文档选项与构建分页；NOT_FOUND配置表示未配置，其余故障显示安全码。编辑带真实版本、完整权威space和固定原幂等键；409保留输入并另读权威当前版本，不自动覆盖。保存不触发模型或重建。人工构建须选择真实发布引用及明确确认可能后台费用，202仅表示接收；取消保留原固定键，不推断退款。导航/卸载AbortController取消，旧响应不更新UI。

所有embedding请求显式携完整expectedEmbeddingSpace给MG付费前硬核主/备用，不只事后核输出；查询原结果优先复用，新调用从纯服务权威目录选可信op.costClass的同完整空间档位，QA不复用后台档位或重写费用类别。

恢复批次仅以原inputRef/root/dedupeKey/dataEpoch续派pending/dispatching，明确失效源按租约literal CAS关闭写资格并实际取消；已dispatched原job到期保留阶段和模型引用由jobs owner恢复证明决策，绝不新建dedupe绕orphan。requeued只记真正已保存job引用，权威未知不作失效。原validatePublication直接导出ValidatePublication复用，不加空wrapper。

视觉/附件描述只运行原批准后台任务，严格任务记忆/源语言与原version授权bytes/hash。PNG/JPEG/WebP/GIF真实image；原生PDF附件任务真实file+vision，不把PDF伪image。普通PDF无文字仍unsupported，不自动OCR；原生PDF分支必须对应另一已批准knowledge.extract-attachment记忆/任务。文本附件完整UTF8≤65536字符不截断；MG拥有实际主备能力、预占和供应方适配，描述仅机器派生不改caption。本域原调用/所有付费结果保留，长期跨构建缓存实际模型版本来源待MG权威证明，不能把事前目录冒充实际派发版本。
