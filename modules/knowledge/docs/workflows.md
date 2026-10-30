# 知识入库与混合检索算法与工作流

状态机：Build: queued→fetching→extracting→chunking→embedding→validating→active；任一步→cancelled|failed|superseded。Extraction: queued→running→ready|unsupported|failed。IndexGeneration: staging→active→retired；一次文档/空间只有一份active。

本文件只写状态机、跨流程规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `content.publication.changed` | 1 | `search::build::handle` | 消费content.publication.changed执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `content.publication.withdrawn` | 1 | `search::build::withdraw` | 消费content.publication.withdrawn执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `identity.site.changed` | 1 | `search::build::handle` | 记下站点状态：后台循环只处理正常站点 |
| `identity.site.purged` | 1 | `search::build::handle` | 删掉本域里该站点的全部数据和文件 |

### 异步任务

本域没有登记的异步任务类型。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `search::build::consume` | 启动后持续 | 从 PostgreSQL 事件队列领取本域订阅的发布与站点事件，处理成功后确认 |
| `search::build::work` | 启动后持续，空闲每5秒或有新构建时 | 领取到期的索引构建（15 分钟租约），失败按退避重排，5 次后记为失败 |
| `search::overview::schedule` | 每分钟检查，启用索引的站点距上次检查满24小时或从没生成过时在后台重算 | 重算层级摘要与全站概括 |
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
