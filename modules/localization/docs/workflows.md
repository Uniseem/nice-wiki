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

## 免费语言估价的统一token来源

EstimateLanguagePlan先核当前语言管理权限，再以本域纯服务身份调用model-gateway:estimateTokens。按接口限制分批发送实际待估文本，不截断文本或复制token算法；清除原portal proof/用户委托并保留站点、epoch、root、费用类别及更早截止。随后调用quoteCost，未知价格保留警告，不伪造零费用。此函数仍planned，本次只修合同，不触发任何模型或额度操作。

上述纯服务保留本站通过平台统一签名siteId传递；清portal proof后建立localization纯服务actor并保留已核本站，不能把旧无声明的system控制作用域替代本站，也不以原portal proof或未签名站点绕过。EstimateLanguagePlan本身仍planned。

## R1 首组规则收口（2026-10-18）

语言标签使用已批准的 x/text/language 解析和标准大小写/脚本/私用标签表示，拒空、长度>35、下划线及解析错误。归一化函数返回规范形式；语言配置仍按用户原文保存和显示，不改写既有禁用语义。写入不允许另一原文标签代表相同规范语言。排版方向继续采用已确认 ar/he/fa/ur 规范主子标签为 rtl、其余为 ltr。权限在平台HTTP入口即时核验，内层不再要求未由该入口填充的 Grants；需要人工的命令仍要求 admin 主体。

术语以 NFC/去首尾空白、规范源/目标BCP47及scope+documentId+sourceTerm建立唯一活动身份。软删不删除历史身份和版本；全站 glossaryGeneration 持久单调，删除最后词条后不回退为0。分页游标绑定源/目标/epoch/首屏代数，变化返回版本冲突。解析以document范围优先global、同范围冲突失败关闭、原词长者优先。任一改动与代数/outbox同事务，只令相关目标机翻manifest stale，protected/human文本保持，仅置review；不自动重译或发生费用。未登记源语言的旧manifest按目标语言保守失效。

memory异步批准命令的人工授权发生在memory批准入口：命令通过memory服务断言携带持久的approvedBy/approvalRevision/proposalHash，受同站/同根/同代绑定保护。本域只接受该来源，不接受浏览器或其他服务自称批准；不依赖原用户10秒委托或要求已过期会话重新批准。术语执行仍核对本域当前代次、维护屏障、expectedGlossaryGeneration及term版本/唯一身份；代数或术语冲突持久记rejected并发布终态，成功记applied及原结果代数。proposalId终身幂等；相同ID但批准人/修订/hash/完整词条/expected代数/根/站/epoch不同均拒绝，不因七天普通回执过期而重做。批准人之后撤销会话不撤销已持久批准事实；新的未批准操作仍由即时权限阻止。
