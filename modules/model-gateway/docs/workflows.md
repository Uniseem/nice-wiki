# 统一模型网关算法与工作流

状态机：Invocation: accepted→reserved→dispatching→streaming→succeeded|failed|cancel_requested→cancelled|charge_unknown；charge_unknown→reconciled。Reservation: held→settled|released；未知账单保持held。Provider: disabled|enabled。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

本域不消费事件。

### 异步任务

本域没有登记的异步任务类型。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `model-gateway.LoadRuntime` | 启动时 | 启动并验证本服务私有持久化与密钥引用 |
| `model-gateway.RecoverLeases` | 启动时及每30秒 | 恢复崩溃中的调用 |
| `model-gateway.PublishOutbox` | 启动后持续，空闲每1秒 | 持久发布事件 |
| `model-gateway.RefreshModelCatalog` | 启动及每UTC日，完整装配待公共保护。 | 刷新公开模型目录 |
<!-- flows:end -->

## 免费任务配置能力读取

仅已认证QA纯服务调用ReadTaskAvailability，按原请求顺序返回qa.preflight/qa.inspect/qa.answer的安全三态。统一入口处理服务断言、schema、当前runtime、维护与截止，本域只核该只读调用范围；在一次严格epoch的SQL快照内关联任务赋值、QA档位和候选元数据。缺赋值/档位为not_configured；已配但模型/供应方停用、chat不匹配或当前凭据未配为unavailable；存储未知或损坏必须返回公共错误，不伪造false。

单任务覆盖替换orderedModelIds第一项，原第二项起有序备用保留并去重；列表空可仅覆盖，不把被替换主模型加回备用。合法配置上限256项，先按完整配置过滤本地eligible，ResolveRoute通过SelectCandidates再保序返回至多16个候选，不能先截断配置。QA免费读取仍只投影固定三任务chat配置三态，不因此取得执行或费用权限。所有候选只核元数据存在性，不读取密文/主密钥或供应方URL，不查价格、余额、预算，不发HTTP探测、不占额度或调用模型。即使返回available，也只表示当前配置材料具备。

## 任务分配配置读写

任务目录只由根contracts/ai-tasks.json生成到本域既有clients.gen.go；每任务默认档位不由管理员请求改写。覆盖按该任务的目录能力全部检查；保存只核模型已启用、同代和能力，不要求已配置凭据/价格，也不探测供应方。

管理写入经平台入口即时核superadmin；同一SQL事务核非空epoch、7天回执、屏障、模型能力与任务版本。有效同代原回执先返回，不重做当前模型检查或产生副作用；新配置才检查所有非released屏障并执行条件CAS，assignment.changed和原结果同事务写平台outbox/回执。过期键仅随成功新配置事务替换，失败回滚保持原状态。

任务读取按目录owner过滤纯服务范围，portal管理员读取在HTTP边界另查identity当前superadmin权限。只返回已存同代赋值，未配置不从默认目录合成；任务覆盖的缺失表示清除。完整服务仍因其他handler/epoch初始化/恢复/后台未齐拒绝启动。

## 档位配置保存

PutProfile在同一当前代次事务内，依次处理7天原结果、全部非released屏障、当前版本与唯一槽、候选直接能力、条件CAS、profile.changed平台outbox及完整回执。事件taskIds对旧/新fast改动保守包含vision/rerank回退任务并去重排序。有效原结果重放不受后来模型停用或本地写屏障阻断；未知Commit不自行重试。保存只核模型元数据，不读取价格、凭据或供应方在线性。

空候选列表保留配置行。视觉/重排未设回退由同快照ResolveProfile处理，先应用任务覆盖再判断候选是否为空；详细顺序见下节。保存不合成候选、不自动重算嵌入或已有付费结果。

## 凭据保存与版本切换

可信启动只保存可选masterKeyRef，不读取secret；免费配置读取不依赖它。凭据写入才安全有界读取32字节主密钥、计算域分离HMAC输入摘要，严格当前epoch事务先处理7天原结果，再核所有未释放屏障、真实active/供应方CAS和未复用历史版本。新nonce/AAD加密后，密文、active、provider.changed、脱敏回执同事务；无网络探测或费用任务。

Put可首次保存/替换或修复缺失active材料；Rotate需要既有完整active材料，全部旧密文保留。当前没有真实派发版本引用与释放依据，最后引用销毁仍未交付，Rotate/credential.go整体partial；不能因invocations为空而清密文。

第二轮收口窄修：PutProvider同代普通摘要显式绑定callerService/dataEpoch，7天原结果先于新写屏障和当前active指针返回；新写拒绝任意非released屏障，不按屏障epoch/deadline过滤。过期键仅随成功新配置事务替换，PutModel及其旧共享helper未改。readCredentialReceipt与新增渠道回执参考签名均以ctx,op开头。首次候选Go检查通过但结构check因签名顺序失败，该次不记完整通过；修后根全量生成/编译/结构检查通过，证据见本轮交付记录。

## 免费执行路线元数据解析

真实调用链为ResolveRoute → SelectCandidates → ResolveProfile，后两者共享一次严格非空当前epoch的SQL事务。taskId必须在根唯一生成目录且owner等于可信callerService，输入costClass只与可信op.costClass核对。必须已有同代任务赋值，tier必须等于目录defaultTier；缺行NOT_FOUND，不用目录默认值伪造已保存赋值。公开HTTP输入、QA免费三任务gate及其缺赋值not_configured语义保持。

先读取同costClass/defaultTier的真实档位并应用override：替换首项，保留原第二项起备用且去重；空列表可仅覆盖，不把被替换主模型加回。仅vision/rerank在槽缺失或覆盖后仍空时使用同costClass真实fast档位；缺fast报NOT_FOUND，不合成profile或版本。缺目标槽而有override也只由真实fast承载。vision继续要求vision；rerank有override继续要求rerank，只有无override空档回退才改为fast/chat。其他档不回退，不因候选暂不可用而切换档位。

ResolveProfile返回真实原始profile、assignment及派生顺序/能力。SelectCandidates读取全部至多256项引用，按同代启用模型、目标能力、启用供应方和active凭据元数据过滤，保留原顺序后最多取16项；后续候选仍须完成完整性/代次检查。所有eligible embedding候选必须同一现有embeddingSpaceID，异空间失败关闭。未知SQL、损坏或异代返回公共错误，不能伪造空候选；无eligible为DEPENDENCY_UNAVAILABLE。

路线只含模型安全元数据及供应方/凭据版本引用，routeVersion按epoch、真实配置和最终候选的固定JSON字段顺序计算SHA256。凭据只读存在性/代次/nonce及ciphertext长度，不读取密文、主密钥、URL或价格；profile费用字段仅配置声明，不读取余额或判断费用准入。不探测、不调用模型、不获取持久派发引用；后续派发前仍需真实引用/屏障/版本及费用协议，不能把免费候选视为完整执行完成。

## 费用策略配置

Put预读事务先核7天完整原回执，再读真实policy和版本判断首次/daily变化；结束事务后才执行必要的当前superadmin权限RPC，不持SQL事务或进程锁。随后短写事务重新先核回执、全部非released屏障及当前policy/CAS；条件改变而未获必要超管授权则VERSION_MISMATCH，无自动重试。策略、原结果及本地脱敏审计同事务提交，既有held/ledger保留；跨域审计投影仍待实现。

无初始policy不从old猜会话美元默认；只能由当前超管完整输入创建。每日0不限、background独立，小时开关关闭也不意味着清账。原回执同caller/epoch规范输入7天重放先于当前策略/CAS/屏障返回，过期只随成功新配置事务替换，未知Commit不自行重试。

## 供应方与模型配置写入闭环

启停沿既有PutProvider/PutModel.enabled字段，无独立启停API；内部未实现SetProviderEnabled参考项已合并到PutProvider。供应方条件UPSERT同时绑定真实version/epoch/credentialVersion，RowsAffected必须恰好1；凭据active指针仍只由凭据端点推进。

PutModel在同一严格当前代次事务先核7天原结果，再核所有非released屏障、真实同代provider及model版本。现有modelId的providerId/providerModelVersion/dimension/preprocessorVersion不可变化，变化须新ID；条件UPSERT只改可变元数据与enabled，并核RowsAffected。停用不清任务分配/档位，不改在途调用、held或ledger，后续路由读取真实enabled拒绝新候选。

模型回执摘要绑定callerService/dataEpoch/model/expectedVersion，完整原结果早于当前配置/屏障返回。旧摘要不能默认为新摘要；7天内hash不匹配CONFLICT，不重签旧结果。过期仅成功新事务替换；配置、现有model.changed平台outbox与回执原子提交。现有嵌入space算法与归一化迁移缺口本轮未扩写。HTTP核路径和非空幂等头与正文一致，由公共入口即时superadmin授权；完整服务缺其他handler仍拒绝启动。

历史M07-R06归一化缺口已在本轮根裁定收口：保留五字段/原hash算法，预处理默认none，不默示L2；算法改变必须新版本身份。本轮解除PutModel embedding设计守卫并补只读ModelWithSpace生产者，待新的GitLab检查，不据历史证据直接记verified。

## 第二轮价目与免费预算

004只追加不可变price_book_entries，无价目种子。真实人工生产沿PutModelInput.manualPrice：省略保留，传对象整份替换，所有费率显式USD才已知；缺项（含固定费）unknown且不回退其他source，显式0合法。服务器取有效秒/纳秒，与模型CAS/回执同事务；省略新字段时摘要JSON旧字节不变。目录刷新未实现，不造remote/bundled数据。

读取manual优先，其次remote/bundled，同source取不晚于目标时刻的最新秒/纳秒和版本。GetPriceBook无历史404；QuoteCost无配置用模型持久版本/updated_at构造确定性unknown，不当历史记录。全程只读报价，不占额度或写账。已知价格用大整数分别ceil(inputTokens*inputRate/1000000)、ceil(maxOutputTokens*outputRate/1000000)，再加显式fixed，超安全整数失败；缺价省略maximumCost并warning，不伪造零。

/v1/token-estimates仅localization纯服务。无内置tokenizer时，累计texts/消息content/text parts的UTF8字节，ceil(bytes/2)+32*消息数后乘6/5向上，返回固定算法版本；message.content遵守64KiB字节预算，图片算法未确认返回503。不联网或保留输入，新增入口由真实本地化估算依赖调用。

## 第三轮账本与费用状态读取

005给invocations仅追加nullable owner_service/site_id/created_at，不回填旧归属。ledger_read_state在invocations/reservations/ledger_entries任一变化时同事务推进int64水位，缺行或耗尽失败关闭；分页索引按UTC秒/补齐九位纳秒/entryId精确排序，不按可变小数RFC3339字符串猜顺序。

GetLedger只portal当前model.manage，时间窗正且最多90天，签名cursor绑定请求和当前主体/权限/站点/epoch及水位，变化409重头；验签和签名在SQL外。账单返回原固定priceSnapshot全值。旧行缺归属/时间/空价目前置503，孤立费用行、损坏/矛盾422，不补、丢弃或当成空。

QueryUsage的portal管理员即时核model.manage；AI owner纯服务要求整个root持久归属均同caller/site/epoch，多owner403，无root404。held只汇总调用created_at在窗内且当前held的预留，settled按ledger.created_at窗；同一当前SQL快照，不是历史余额。组内任一未知省略完整相应总额，未知调用ID完整排序；空且全已知的组可为0，不能用部分和代表全额。big.Int聚合后核安全整数，超过256记录/未知ID或可表示范围失败不截断。

第三轮只读历史中未实施Reserve/Create/Extend，彼时费用作用域和签名缺口尚未收口；现本轮规则及源码状态以文末费用执行首批为准。

账本时间合法性由SQLite内建strftime对前19位加0秒规范化往返，精确小数独立核1..9位，非法时间元数据view/部分索引保证坏行不能被窗口隐去；不手写日历，也不把读预算变成业务总量上限。依据为[SQLite日期函数](https://www.sqlite.org/lang_datefunc.html)及[官方往返说明](https://sqlite.org/forum/info/ae8081b04cef606c85a4ccd63903a104de1210ac3608808825b9c817809dcf46)，并只读核固定依赖SQLite3.53.4源码路径，未运行SQL或临时程序。窗内charge_unknown须有实际held或未知金额账单证明，否则503而非零占用。HTTP整数limit保持与入口ParseInt基数0相同语义。

同root全部reservation先核held/settled/released合同三态，非法状态422而非过滤成空；合法settled/released不聚合，不套held金额字段规则，仅held参与当前预留校验和汇总。

本轮费用首批已编写Reserve/Extend/RecordUsage/Settle/Unknown/Reconcile及调用安全读取/持久SSE、失败分类、日志白名单；全部待GitLab，完整Create/Execute/Dispatch、供应方网络适配、凭据引用回收及启动仍未完成。首批配置足够时消费者真实读写，不把暂缺原生产入口伪装成完整已上线费用链。root统一补公共snapshot/barrier与tokenizeText，本域不争用公共文件或tokenizer专用所有权。

管理员额外头表单须区分“不修改”和“清空全部”：前者省略secretHeaders，后者明确[]并providerCAS；非空数组是管理员显式替换整份加密映射。秘密值不在管理回读中预填，不进入事件/日志/回执正文，不能把Provider.extraHeaderSecretRefs当配置文件引用。对应UI实际装配仍在后续完整页面批次，不把有schema称为已有界面。


## 原调用执行与关闭窗口

Create先严格owner/task/cost授权与原guard消费签名，再固定路线/价目/媒体原版本；幂等重放优先复用原结果。公共TaskGroup在持久commit前取得真实容量，回调等待commit信号；满容量回滚，不假accepted。Start保留原业务op.Deadline、原values而脱离HTTP断开，进程取消/管理员取消/原deadline仍真实取消IO。

Execute领取30秒lease每10秒续租，按全保守输入+最大输出预算预留。视觉仅模型明确MaxInputTokens可作保守费用上界，不宣称真实图像token。每次新attempt与held原子产生；派发前再核lease/取消、固定space和管理员批准。仅一次供应方HTTP，无重定向/自动重试；发送前先持久dispatching/dispatched_ns。真实delta<=16KiB且累计文本<=64KiB，首delta同事务转streaming，SSE/JSON上限分别64/32MiB；错误原始正文不日志。真实终态/结构结果先核协议和维度/有限值后持久evidence，slot始终释放。

取消先同事务cancel_requested/原回执再TaskGroup.Cancel。未派发或明确not_accepted证据才释放；已派发未知保留held，不能把网络取消当免费。新备用只走固定直接后继，原模型结构重试至多一次，总attempt数不超路线+1；已有完整结果不重算，已有文本不能换供应方拼接。所有attempt共享原root及业务截止。

BeginFinalization专供已发生IO后的真实usage/费用/终态/释放，脱离HTTP取消建立最多5秒当前权威记账窗口；只在控制层仍允许draining时收尾，safe/failed后拒active写。它不延长持久原deadline、不创建新预留/attempt、不允许模型IO。若窗口已关闭，保留原事实给恢复，不猜0。RecoverLeases只沿原不可变hash处理；从未派发且原deadline有效才继续，否则只复用持久付费证据或收敛unknown，禁止重发。完整启动、snapshot/barrier与其精确关闭装配仍待下一批，不在本批伪造ready服务。

完整文本结果收口：同次原delta保持有效UTF8、每条16KiB/累计64KiB，SendProviderRequest只在HTTP2xx、Code空且完整协议结束/终态后构造原chat/vision/translation的ModelOutput.Text指针；实际空答复显式空指针，未完整则无Output。沿既有evidence+winning+record同事务，不修改旧付费证据。Get/Lookup核实际winning尝试/调用列表与完整成功原evidence/record全值摘要；未知字段不丢信息读。旧文本只允许单initial ordinal1、真实winner、完整无错证据及从accepted到completed连续原SSE全部证明后免费只读派生；多尝试或缺流/缺来源失败关闭，不伪空/404、不重付，不写旧hash/费用。

消费者费用安全窄修：Lookup只有真实tracking+原key/root/site/epoch/body确未接受时专用INVOCATION_NOT_ACCEPTED/404；普通NOT_FOUND或旧部署路由404不授权Create。QueryUsage实际始终给兼容可选windowCoversRoot（窗覆盖原root全部调用创建及所有历史ledger时间）与allInvocationsFinal（原记录/尝试列表完整、真实终态/无执行lease且每attempt终局），旧缺事实false/严格失败；不把safeToRetry当新付费审批。报价portal用当前model.manage，原AI owners/localization免费能力保留，knowledge额外纯service+background；免费多模型估价不扩Create/fee。

安全统计posted变更页V1：GET /v1/statistics/ledger仅analytics纯服务且等site-config真实当前site/epoch，afterRevision默认0、watermark首轮取当前历史上界、cursor平台签名/24h到期/稳定service+site+epoch+after+limit绑定；单页最多100、2.5秒，60秒后台任务可持久nextCursor续接，新写不使旧上界变化。schema12为账项真实首次baseline观察revision1与实际后续insert/update/delete/supersede/void独立递增，非event.revision或伪原收费历史版；变化与账本原事务，历史不可原地改删。冻结安全来源对照原execution hash/route/attempt，不读当前models/providers补历史，未知金额省略。仅posted，不覆盖无ledger的held/charge_unknown；feeCoverage=posted-only/completeFeeCoverage=false必须显式，不能称全站完整费用/未知数量为0。跨恢复target原source费用缺认证投影503，不重基旧证据。完整观察feed/worker/公共保护仍未齐。

公开目录来源收口：本次用户已批准models.dev MIT原始公开JSON固定编译快照，内容版本sha256:80a19a3c78de1a7b0eb0a72e13e44df21180f0681d6cdf1dbd4c4c345fae3fb6。远端不可用优先保最近当前代持久源，无持久源才使用bundled；响应source/sourceVersion及降级fallback明确实际来源。人工覆盖优先、无catalogRef旧行保留；没有每次固定费用事实仍未知，不补0。原许可与精确来源随模块，第三方总登记由根统一。
