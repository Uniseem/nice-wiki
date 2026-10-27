# 语言、翻译与术语算法与工作流

状态机：Language: disabled→enabled→disabled。Translation: queued→running→review→ready|superseded|failed|cancelled；ready→stale。Unit: untranslated→machine|human；human具有protected布尔值。GlossaryProposal: accepted→applied|rejected。

本文件只写状态机、跨流程规则。

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

EstimateLanguagePlan先核当前语言管理权限，再以本域纯服务身份读取document/UI两类真实任务路由并调用model-gateway:estimateTokens。按4096 Unicode字符上限分批发送完整实际待估文本，不截断文本或复制token算法；清除原portal proof/用户委托并保留站点、epoch、root、费用类别及更早截止。随后对两类任务分别调用quoteCost，只有主模型和PriceSnapshot完全一致时才合并为一个保守金额；未知价格保留`price_unknown`警告，不伪造零费用。估价只写本域计划，不调用模型、不预留或扣减额度。

上述纯服务保留本站通过平台统一签名siteId传递；清portal proof后建立localization纯服务actor并保留已核本站，不能把旧无声明的system控制作用域替代本站，也不以原portal proof或未签名站点绕过。

### 批准与失败恢复

ApproveLanguagePlan在批准时重新读取公开树、语言配置、术语/界面代数和两类routeVersion，并免费调用quoteCost核对原token输入、输出上限与价目。网关routeVersion不含价目，因此任何PriceSnapshot或报价变化都使计划失效。计划原24小时expiresAt不延长，任务输入固定携带该时刻及ExpectedPriceSnapshots。

首次批准开始在同一事务保存逐文档/UI任务意图和完整OperationContext，包括原rootOperationId、主体、dataEpoch与deadline；后续重试只能使用该持久scope和更早原截止，不能换成新请求的root、主体或deadline。jobs dedupeKey绑定planId和完整任务输入hash，因此新计划不会与旧计划的相同文档碰撞，原计划重放又始终复用同一ObjectRef与input。任一jobs未持久接受时不返回成功，计划保持pending；全部接受后才CAS approved并启用automatic=true。执行/付费worker尚未实现，保持blocked。

## R1 首组规则收口（2026-10-18）

语言标签使用已批准的 x/text/language 解析和标准大小写/脚本/私用标签表示，拒空、长度>35、下划线及解析错误。归一化函数返回规范形式；语言配置仍按用户原文保存和显示，不改写既有禁用语义。写入不允许另一原文标签代表相同规范语言。排版方向继续采用已确认 ar/he/fa/ur 规范主子标签为 rtl、其余为 ltr。权限在平台HTTP入口即时核验，内层不再要求未由该入口填充的 Grants；需要人工的命令仍要求 admin 主体。

术语以 NFC/去首尾空白、规范源/目标BCP47及scope+documentId+sourceTerm建立唯一活动身份。软删不删除历史身份和版本；全站 glossaryGeneration 持久单调，删除最后词条后不回退为0。分页游标绑定源/目标/epoch/首屏代数，变化返回版本冲突。解析以document范围优先global、同范围冲突失败关闭、原词长者优先。任一改动与代数/outbox同事务，只令相关目标机翻manifest stale，protected/human文本保持，仅置review；不自动重译或发生费用。未登记源语言的旧manifest按目标语言保守失效。

memory异步批准命令的人工授权发生在memory批准入口：命令通过memory服务断言携带持久的approvedBy/approvalRevision/proposalHash，受同站/同根/同代绑定保护。本域只接受该来源，不接受浏览器或其他服务自称批准；不依赖原用户10秒委托或要求已过期会话重新批准。代数或术语冲突持久记rejected并发布终态，成功记applied及原结果代数。proposalId终身幂等；相同ID但批准人/修订/hash/完整词条/expected代数/根/站/epoch不同均拒绝，不因七天普通回执过期而重做。批准人之后撤销会话不撤销已持久批准事实；新的未批准操作仍由即时权限阻止。

## R1 第二组算法边界（2026-10-18）

人工编辑经当前单元version CAS，正文对象/manifest/索引/七天原回执同事务激活，origin=human/protected=true，单元及任务humanGeneration递增；已排队/运行的旧批次本域状态superseded，使后续提交无法覆盖。显式切换保护也递增人工代数，但不改正文、不发起重译。

受控模型片段使用 `<iN>`/`</iN>` 与 `<vN/>` 编号边界，URL/代码/公式原值保留在源树；目标编号标签数量和顺序须逐字相同，不接受其他HTML。ValidateTranslationOutput还要求完整唯一unitId，固定sourceLanguage与目标不同且≥20字符时拒绝原样回传。源文本缺失则该AI分支不可用。提示仅带本批文本/IDs、整篇已脱敏受控上下文、前批/相邻译文、节标题及本批实际出现术语；不会把原始sourceTree当上下文直接发模型。人工保护块不能加入机翻批次。

翻译记忆按实际Unicode字符数与该语言阈值（默认20），精确sourceHash/sourceText/目标语言、normal状态、当前术语代数和权威对象命中；只把目标文本/hash放入新候选，保留新blockId/发布来源。增量计划把人工/保护内容单列，源变时只提出review，不覆盖其文本；未变源和有效TM复用，其余列待译。

降级输入显式记录contextReduced/terminologyRetried，输出nextStage：长度只缩上下文一次；format/unchanged从batch对半到halves逐块再single原文回退；术语提醒只重试一次，仍不符进入review、不改回原文。每个真正新attempt仍须经MG独立预留，纯规划函数不调用模型。词形合规当前只认可已指定译法的NFC/Unicode大小写等价；无明确登记的词形不猜测，走既有一次提醒及review。

界面字典仅以实际安装的interface_sources为固定来源，未安装时只有该能力明确不可用，不生成示例目录。插值为`{name}`且名称/出现次数必须与来源一致，拒未知key、重复key和HTML。人工值CAS后human/protected，catalogGeneration递增并发送完整合并字典hash事件。读取按key合并当前目标与builtin来源；旧sourceHash不匹配的目标值按合同回退builtin，来源未安装时不能凭knownGeneration伪造notModified。

## R1 第三组源模板与交付（2026-10-18）

content完整发布响应在HTTP依赖边界严格校验一次，随后Splitter信任合法DocumentBody，仅解析并核对canonical源hash/schemaVersion。16类源树完整保留：paragraph/heading及可用image alt/caption建立文本单元；codeBlock/mathBlock/媒体嵌入保持原树；列表、引用、callout、列容器保持层级并沿稳定子块处理。行内每个文本段用`<iN>`包裹，代码/公式/换行/URL为`<vN/>`，原值只留本域私有slots，绝不把源树、URL或代码原值放进模型上下文。纯只读值/空白块不创建收费单元。

每单元仍限65536 Unicode字符。超大table只按content真实rowId拆、parentBlockId追溯原表格；超大行及其他不可拆大块原树保留并列fallbackBlockIds，不截断、不编造源ID。源输入按生产方16MiB UTF8，private聚合对象64MiB，公开目标树另限16MiB；在写入/完成前检查实际完整对象预算。源ID仅从TM片段hash排除结构身份attrs，链接目标的blockId/URL与代码结构仍参与hash；20字阈值只计去编号/转义后的可翻译Unicode字符。

目标还原只能写固定路径的text/alt，逐片核对原值；完整有序标签之外的文字、未知单元、重复ID、额外HTML及未配对标签全部拒绝。只读URL/code等值和全部原属性、稳定blockId/rowId不变。还原结果再次通过content schema，不把private slots发外域。

ReadTranslation先读取当前权威源与visibility水位；同一本地快照选最新及必要时上一完成generation（两对象总IO≤64MiB），按当前单元对齐。未完成使用可还原旧译文并保留其源hash/原文引用追溯，新块或格式不兼容块用原树fallback；超大块亦按已有partial分支提示。输出兼容新增body/bodyHash：完整目标树对应translation.document的当前源publication、targetLanguage和目标generation，bodyHash为新目标canonical正文SHA256，绝不直接拿source contentHash作译文hash。返回前重新核content可见性及本地manifest/glossary精确水位，变化STALE_RESULT，不交付混装成功。

ReadTranslationUnits管理分页使用平台serviceauth.SignCursor/VerifyCursor。scope绑定稳定subject/session/sessionRevision/permissionRevision、site/epoch、译文ID/目标语言、当前源publication、target generation、glossary代数与页大小；不含每次proof变化的ExpiresAt。payload仅含最后稳定unitId及10分钟独立到期，按固定源顺序keyset推进。未知/过期/水位改变一律STALE_RESULT，整页拒绝。页面同样在外部源IO后复核本地水位，不自行重签身份或复制加密机制。

保护表格分组边界：whole table单元变为row单元时继承父表人工/保护事实；row单元合并回父表时同样不能排入机翻。普通row分组不扩大到其余无保护行。管理读页标识继承保护与review。完整公开ReadTranslation在旧人工编号无法安全投影到新分组时明确DEPENDENCY_UNAVAILABLE，本函数仍作为真实复杂分支blocked，不把原文fallback说成已保留人工旧译；后续须按稳定子块/行定位完成安全映射后才可记全部实现。相关持久人工译文与付费对象不删、不覆写。

### 明确保存语言偏好

public-site转发已核访客请求；新命令可更改选择；相同命令返回原到期时刻。该链不调用模型、不根据推测信号保存偏好。SuggestLanguage的快速模型缓存分支与全局付费批准边界仍待设计收口，本轮没有以规则降级代替完整模型行为。

### 人工保护的稳定节点投影

整表与真实行分片互换时，先从旧源私有模板解析旧人工译值，再按最深真实稳定blockId/rowId/cellId/itemId/columnId、节点类型及该节点内部text/alt路径对应当前字段。祖先数组序号不参加对应；字段内固定/可译分段必须同形，固定URL/代码永远采用当前源。对应值按新slot编号编码。新增真实子节点原文交付并partial/review且继承父保护；保护父块只能source-only、字段归属或段形状无法证明时失败关闭，保留旧对象待人工处理。新投影没有独立持久unit引用，省略fileRef；真实原单元沿用真实引用。

投影来源保持真实：仅复用了真实human字段才标human，受保护machine仍为machine。投影按完整新编号文本逐段限制65536 Unicode字符，超限失败关闭。公开units上限10000与content全树节点/拆分一致；review但全部译值可恢复时completed计完成且partial=false，有新增原文才partial。PreviousTargetText仅可追溯真实旧整段，不要求即时跨分组投影伪造该可选值。

同一最深稳定anchor的无ID子树形状必须整体一致：只保留type及字段/数组结构的签名发生变化即失败关闭，包括inline文本插入/删除或全部text消失。不能把旧content/0/text映到新插入无ID文本；有独立真实稳定ID的新增子节点才可单独原文partial。同一anchor只取最新保护来源完整字段集，不从更旧代补已移除字段。

## 管理员语言建议付费许可

用户2026-10-18明确选择管理员开启后才允许。默认关闭，PUT /v1/language-suggestion-policy仅当前localization.manage管理员，以CAS/幂等回执持久新版本与不可变approvalId/site/epoch/批准时间/主体/权限修订/原root。MG纯服务每新attempt经GET /v1/language-suggestion-approvals/{approvalId}读取当前真实启用许可；停用、版本变化、换代或未知全部失败关闭，不能用访客或本域自报enabled。后台付费仍经MG预算与费用链，免费guard.locale.suggest不会因此获得付费token。恢复不重基旧批准为target epoch，须管理员重新明确开启。
