# 语言、翻译与术语算法与工作流

状态机：Language: disabled→enabled→disabled。Translation: queued→running→review→ready|superseded|failed|cancelled；ready→stale。Unit: untranslated→machine|human；human具有protected布尔值。GlossaryProposal: accepted→applied|rejected。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `content.publication.changed` | 1 | `localization.HandlePublication` | 消费content.publication.changed执行本域失效/任务/批准回执；不同步反向请求生产方。 |
| `content.publication.withdrawn` | 1 | `localization.InvalidateDocumentTranslations` | 消费content.publication.withdrawn执行本域失效/任务/批准回执；不同步反向请求生产方。 |

### 异步任务

#### `localization.document`

- 执行函数：`localization.ExecuteTranslation`
- 触发：HandlePublication、CreateTranslation、ApproveLanguagePlan、RecoverTranslationLeases创建

1. FetchTranslationSource读取固定发布修订；SplitTranslationUnits按稳定blockId切分；PlanIncrementalTranslation选出待译单元，先ReuseTranslationMemory复用≥20字同源译文
2. memory:resolveContext取localization.document任务记忆；ResolveGlossary固定术语快照；BuildTranslationPrompt按约1000输出token分批
3. 每批RunTranslationBatch→ValidateTranslationOutput；失败按PlanTranslationFallback对半→逐块→保留原文；CheckTerminologyCompliance不符时带提醒重译一次；开启反思时ReflectTranslation
4. CommitTranslationBatch按源修订/术语/人工代数CAS写入，protected单元跳过；首次翻译该文档时ExtractAutomaticGlossary
5. 全部单元终态后CompleteTranslation写manifest并登记translation.ready

#### `localization.ui`

- 执行函数：`localization.TranslateInterfaceCatalog`
- 触发：ApproveLanguagePlan与RequestInterfaceTranslation创建

1. 读取固定catalog generation与目标语言缺项，排除protected与sourceHash未变的key
2. 每批16个key经InvokeModel（localization.ui任务）输出严格schema
3. ValidateInterfaceCatalog校验占位符与HTML后CAS写machine译文，catalog generation++并登记interface-catalog.changed

#### `localization.cache-answer`

- 执行函数：`localization.ExecuteCacheAnswerTranslation`
- 触发：TranslateCachedAnswer创建

1. 读取命令固定的源答案hash与answerRevision；memory:resolveContext取qa.cache-translate任务记忆
2. 以继承的qa根操作、费用类别与准入许可调用InvokeModel（fast档），保留引用token
3. CommitCacheAnswerTranslation核对源版本后保存ready译文并登记cache-answer.translated

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `localization.LoadRuntime` | 启动时 | 启动本地化服务 |
| `localization.RecoverTranslationLeases` | 启动时及每60秒 | 恢复翻译任务 |
| `localization.PublishOutbox` | 启动后持续，空闲每1秒 | 持久发布翻译事件 |
| `localization.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `localization.RunWorker` | 启动后持续 | 主动领取本域持久任务 |
<!-- flows:end -->
