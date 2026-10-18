# 最终完整文本小批待GitLab

源码 `26210be2b9f38c2d270c5b6f845ad67b9144c8f3`：5实际修改函数 SendProviderRequest/ReadAttemptEvidence/readInvocationRecord/ReadInvocationSnapshot/GetInvocation 与1新增实际参考 ReadLegacyTextResult 均blocked待统一GitLab CI，无新原业务完成计数。原delta UTF8/64KiB真实累加，只有2xx/完整协议结束/Code空才给Text（显式空与没有结果分开），沿既有evidence/winner/record原事务。旧文本仅单initial ordinal1/winner/完整证据/连续原SSE纯读免费派生；缺来源不造空/404、不重付，不改历史fee/evidence/hash。

基线ba19256，独占codex/finish-20261018-gateway-final；未本机Go/gofmt/gen/pnpm/编译/测试、未真实接口/模型/部署/push。用户新批准probe日额度默认0不限、QA/background/admin-probe单独记账，以及models.dev MIT固定编译数据；后续继续安全统计生产页与目录/probe独立链。Create/Lookup tracking、保护/完整启动仍按原具体阻塞，未以本批编译预期记完成。

以下为历史记录。

# 2026-10-18网关后续源码检查通过

GitLab pipeline101（源码b0e723d；生成466、三个Go分片467/468/469、前端470）全部成功；16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及四个实际前端类型检查/构建通过，精确2610字节生成物已回传。普通测试无测试文件，不代表业务验收或部署。

本批100个完整独立或实际修改函数通过源码检查；首次接受/原键不存在证明所需tracking来源未齐，CreateInvocation/LookupInvocation/RequireInvocationLookupTracking仍blocked。13原业务保护/完整启动/探测与目录缺口继续未齐，不把源码编译当可运行。原参考ExtendReservation/RedactLog无实际caller已按ADR0002移除，不以无意义调用过结构。

以下为历史记录。

# 原键查询与费用更正小批

源码a3c7536：新增业务LookupInvocation及4真实助手RequireInvocationLookupTracking、ReadInvocationSnapshot、ReadStatisticsInvocationRecord、readInvocationRecord，统一model-gateway前缀。13项新/修改函数blocked待GitLab，无提前verified。schema011与lookup.go已登记；根需重生成文档/客户端。

POST /v1/inferences/lookup复用完整CreateInput，原key/owner/site/epoch/root及规范输入核原回执和原持久请求，异输入CONFLICT，返回当前Invocation及真实完整output。孤立回执/原键无回执Integrity；只有真实tracking来源完整且没有未索引/未知归属旧行才404。marker缺失503，不从空库自建证明，Create首次接受也门禁。旧已接受回执仍可观察。真实fresh activation/verified restore来源生产者尚待公共保护，整服务不可启动。

invocation.reconciled/v1与原费用更正/void/原回执同事务outbox，charged才带真实entry/amount，not_charged不造账。sourceDataEpoch仅保原来源，target与envelope一致。QueryUsage新增analytics纯服务同实际site/current epoch安全投影，不扩其他AI owner/root或付费权限。根负责消费者登记。

剩余13原业务项：LoadRuntime、RotateCredential、ReadAdminCatalog、AcquireBarrier、ReleaseBarrier、ExportSnapshot、StageSnapshot、DiscardSnapshot、RefreshModelCatalog、CreateConnectionProbe、main、BuildApplication、Ready。保护/原费用恢复投影/真实tracking及引用tracking需公共snapshot/barrier；probe额度与bundled目录许可待用户答复。未发现可脱离这些前置而完整交付的新原业务项，冻结本域等待根调度；不将恢复设计步骤算源码完成。

无本机Go/pnpm/生成/格式/编译/测试、无真实接口/模型/服务/部署/push。仅本地文档读取与git diff --check。新增调用查询不改原总deadline，不造新付费事实；StartAcceptedInvocation已保留根43d47ba右括号修正。

# 网关集成修正待GitLab

pipeline99前端四套及content编译通过；模型Go因转发依赖未登记缺生成JobsClient，完整检查缺9处依赖映射。现补真实转发/任务入口及动态Body reader依赖，Create通过既有ResolveRoute；删除确无真实调用的2个参考助手ExtendReservation/RedactLog，不新增无意义调用。尚待重跑，完整服务与业务验收未齐。

# 原完整结果只读窄修

源码2b4c261：GetInvocationResult可选output，仅原resultAttemptId对应持久完整有效evidence与record.Output摘要全等才返回；不以费用状态猜，不造SSE/模型调用。winner缺证据Integrity，不伪404诱发重算。MarkUnknown不回退已终态结果，引用释放还查未解决price_unknown账单及未reconciled用量冲突，未知费用不退休原密钥。3旧函数重新blocked待CI，无新函数/迁移。

无本机Go/格式/生成/编译/测试，未真实接口/模型/部署/push；公共readInference适配根统一处理。其余审批和公共保护依赖见下方。

# 目录来源与真实结果身份

源码48b210e，7新增实际helper（如下），无新增原业务ready。Refresh远端链已写，但真实bundled快照借用许可待用户答复，整项partial。schema010、catalog/refresh.go需根索引。Model.catalogRef显式两列，手工覆盖优先、无ref旧行保留；固定源30s/10MiB真实公开下载、规范事实/每条hash、模型CAS/价目/事件/原回执同事务，无自动新渠道/凭据。已读API字段说明，未借入快照/SDK或真实下载。

- `model-gateway.ApplyCatalogSnapshot`
- `model-gateway.DownloadModelCatalog`
- `model-gateway.ModelSnapshotHash`
- `model-gateway.NewCatalogRefreshHandlers`
- `model-gateway.NormalizeModelCatalog`
- `model-gateway.ReadInitializationState`
- `model-gateway.SaveCatalogPrice`

MG ModelSnapshotHash对实际固定gen.Model规范JSON，包含配置版本；ReadModels权威给hash，完整有效结果与evidence同事务记resultAttemptId，费用仍可unknown；对账不改winner，旧缺证据不补。根bb9c2ee公共ModelAttempt.ModelSnapshotHash及Invocation.ResultAttemptID必需后统一gen。初始化严格读site-config:readInitializationStatus真实响应，根固定后台名单已补；完整Ready/保护仍待公共装配。

另金额表示128字符/指数128有界后big.Rat，避免极端指数资源消耗；实际embedding返回版本不符为STALE_RESULT终止，不当格式重试重付。11旧修改函数重新待CI。无本机Go/pnpm/编译/格式/生成/测试，未真实接口/模型/服务/部署/push。

剩余设计依赖：probe限额用户待回；bundled许可用户待回；公共snapshot/barrier待ROOT；专有restore projection获ROOT认可，旧费用/原请求/执行hash不可重基，当前target只读/人工费用投影，不新派发旧调用。

# 启动独立链

源码6ea4576：Serve/Shutdown/Health源码ready待CI，3新实际助手Application.ConfigureBusiness/BackgroundTasks/Drain，NewRuntimeClient修改重查。Build/Ready仍partial，缺公共保护/初始化权威实际装配，服务不可运行。复用c5a0dcd LoadServiceIdentity并移除2复制身份助手。Serve取消只Drain，平台等待全部后台后才Shutdown关库；Ready不编造initialized/safePoint/Revision。

无本机Go/pnpm/生成/编译/格式/测试、无真实模型或push。继续明确catalogRef来源更新、待公共保护与用户probe额度/新目录许可。

# 原生PDF小批

源码f9bf34b+合同c3ccdc2一起候选，依赖根dd1e399公共File；3新实际helper：ReadInputBudgetModel、MediaPartAsset、ValidateInputModel，统一model-gateway前缀，均blocked待GitLab。11既有修改函数重新待CI，无新增原业务计数。

file/text/image严格互斥，调用仍vision；仅原生PDF application/pdf，3协议映射，原授权媒体version/hash/MIME/size及PDF头核对，模型明确file+vision+MaxInput，每主/备预留/派发前核。EstimateTokens非文本真实窗口保守上界，不冒称实际token；未知费用仍held。

仅读公开官方API文档，无新SDK/迁移/测试/真实模型，本机未Go/pnpm/生成/格式/编译，无push。继续共享身份加载替换、catalog/probe/保护与完整启动。

# 管理界面与转发第5批

源码91bfe36。4原业务ready：PublishOutbox、LoadAdminOverview、SubmitPrimaryEdit、mountModuleUI，+26实际helper，全部blocked待GitLab。移除重复EnqueueOutbox参考ID不计完成；Rotate完整tracking初始化待后续完整Build，暂partial。

UI沿批准Vue/TS/Vite精确版本，lock仅CI真实pnpm生成；根需typecheck/build及保留dc8435d公共端口。新增schema009、runtime/relay.go、runtime/retirement.go、ui/src/vue.d.ts需公共结构索引。credentials退休审计后删除材料，header墓碑保留；unknown/held/legacy缺证据都不销毁。

实际helper IDs：

- `model-gateway.DestroyRetiredSecrets`
- `model-gateway.RelayOutbox`
- `model-gateway.clearSecrets`
- `model-gateway.clearSensitive`
- `model-gateway.decimal`
- `model-gateway.dollars`
- `model-gateway.editModel`
- `model-gateway.editProfile`
- `model-gateway.editTask`
- `model-gateway.failure`
- `model-gateway.loadLedger`
- `model-gateway.micros`
- `model-gateway.moveModel`
- `model-gateway.perform`
- `model-gateway.pollProbe`
- `model-gateway.refreshCatalog`
- `model-gateway.reload`
- `model-gateway.runProbe`
- `model-gateway.save`
- `model-gateway.saveCredential`
- `model-gateway.saveModel`
- `model-gateway.savePolicy`
- `model-gateway.saveProfile`
- `model-gateway.saveTask`
- `model-gateway.stop`
- `model-gateway.write`

未本机Go/pnpm/生成/编译/格式/测试，未真实接口/模型/部署/push。继续原生PDF file受控链及probe/catalog/保护/完整启动。

# 付费执行第4批交接

源码`b6d8ae0`，7原业务ready：CreateInvocation、ExecuteInvocation、DispatchInvocation、CancelInvocation、CreateFallbackAttempt、RecoverLeases、BuildProviderRequest（统一model-gateway前缀）。均blocked待GitLab，不提前verified。既有QueryUsage、SSE、费用/预留/结算/释放和HTTP映射修改重新待CI。完整BuildApplication仍partial；下一批继续完整启动/保护、管理员catalog/probe、凭据引用回收与UI，不以本批启动成功。

新增42实际参考助手IDs：

- `model-gateway.AuthorizeImages`
- `model-gateway.BeginFinalization`
- `model-gateway.CanonicalInvocationInput`
- `model-gateway.CheckLanguageSuggestionApproval`
- `model-gateway.ClaimInvocationLease`
- `model-gateway.ConsumeProviderFrame`
- `model-gateway.ErrorCode`
- `model-gateway.ExecutionSnapshotHash`
- `model-gateway.FinishCancelledInvocation`
- `model-gateway.FinishInterruptedInvocation`
- `model-gateway.FinishInvocation`
- `model-gateway.InvocationResultState`
- `model-gateway.InvocationSafeToRetry`
- `model-gateway.LoadImages`
- `model-gateway.MergeProviderUsage`
- `model-gateway.NewRuntimeClient`
- `model-gateway.Nonempty`
- `model-gateway.PrepareInvocationAttempt`
- `model-gateway.ProviderReportedCost`
- `model-gateway.ReadAttemptEvidence`
- `model-gateway.ReadDispatchProvider`
- `model-gateway.ReadExecutionAttempt`
- `model-gateway.ReadExecutionRecord`
- `model-gateway.ReadImageBytes`
- `model-gateway.ReleaseResolvedReferences`
- `model-gateway.ReleaseUnacceptedAttempt`
- `model-gateway.RenewInvocationLease`
- `model-gateway.SendProviderRequest`
- `model-gateway.ServiceProviderContext`
- `model-gateway.StartAcceptedInvocation`
- `model-gateway.ValidateInvocationPayload`
- `model-gateway.beginExecution`
- `model-gateway.embeddingSpaceID`
- `model-gateway.ensureDataLayout`
- `model-gateway.loadSigningKey`
- `model-gateway.loadTrustAnchors`
- `model-gateway.newRuntimeClient`
- `model-gateway.providerIdleReader.Read`
- `model-gateway.validCapability`
- `model-gateway.validCatalogOrigin`
- `model-gateway.validModelString`
- `model-gateway.validateConfig`

root候选前置：59ef42d平台TaskGroup/二进制客户端、8a56a5e localization权威批准与公共ExpectedEmbeddingSpace。root须保留9875896 tokenizer实际3ID、app token handler和caller allowlist；本分支未整pick公共提交，避免冲突。MG x-runtime alias引用localization ReadLanguageSuggestionApprovalResult和media ReadAssetResult。schema008_execution_evidence.sql独占，后续从009追加；RecoverLeases移至invocation/recovery.go、ExecuteInvocation移execute.go，同ID职责已同步。根维护公共结构索引。

结果与费用独立；完整真实结果可succeeded/completed但attempt未知/held，缺usage/收费ID不伪零、不重算。QueryUsage核完整不可变execution hash及全部attempt，safeToRetry仅原root全部accepted/reserved未派发；SSE terminal看terminal_ns，费用中间unknown不早EOF。取消只明确未派发/未接收才释放。管理员语言建议每新attempt准备/派发读持久批准，不允许公开空token。expected space主接受及各候选真正预留/派发时核五字段，备用不符不得收费。有限预算未知安全补丁50ab2ae与台账504c7a3已另交根CI86。

未新增测试/样例/临时业务验证程序，未本机Go/生成/格式/编译/vet、未真实接口/模型、未部署/推送。供应方协议依据已阅读官方API说明，未复制第三方源码或新增依赖。仅git diff --check和文档结构/隐私阅读。

# 费用预算安全窄修

# 2026-10-18预算与可信配置修正已通过

GitLab pipeline86（770b93c；生成391、Go三分片392/393/394、前端395）全部成功，预算未知金额修正与可信Git凭据索引已通过16模块readonly build/vet/gofmt、已有普通go test、完整make check和前端类型/构建；精确31561字节生成物回传到123f699f162abaee206e99fe00c3e07e89050538。普通测试无测试文件，不是业务验收。

完整服务仍继续实现。

# 2026-10-18 续写第二批检查通过

GitLab pipeline84（源码72219d7；生成381、三个Go分片382/383/384、前端385）全部成功，16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及前端类型检查/构建通过。pipeline83各编译分片通过但缺既有信封校验登记，已补并复检；实际普通测试无测试文件。源码生成物由GitLab工具回传，不代表业务验收、隔离行为验收或部署。

本域本批62项完整函数通过源码检查，完整启动/快照恢复/业务accepted状态保持未完成。模型网关、知识、数据保护三个worktree继续，未在本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 供应方与加密额外头第3批

源码0fc3227，5个原有协议映射/并发业务与7个必要助手就绪，PutProvider及原回执修改重新待CI；全部blocked。secretHeaders缺失保留/[]清空、主密钥HMAC与AES-GCM安全生产已写；固定route/原凭据引用表已追加但回收尚未启用。媒体二进制客户端、完整供应方网络、Create/Execute/Cancel/恢复/启动仍继续，不提前把辅助映射算完整付费入口。

# 五字段嵌入空间窄批

更新2026-10-18，源码3af2369。PutModel、ValidateModel、ReadModels、ValidateEmbeddingSpace与唯一ModelEmbeddingSpace助手源码ready，统一blocked待GitLab。保留原spaceId算法，默认none，不默示L2；readModels只读ModelWithSpace新增权威embeddingSpace，未扩大模型写入DTO。知识可直接消费版本化响应；完整费用生产派发仍继续。

# 模型网关费用执行首批

更新：2026-10-18。用户授权继续写完剩余代码；不在75%停止。独占modules/model-gateway，分支codex/finish-20261018-gateway，基线770ea1c，源码87a5af0。

- 11个原有费用/状态/流函数及26个实际助手共37项源码ready；旧readLedgerEntries/QueryUsage因attempt价目及更正投影变化重新待CI。全部blocked，未改既有verified保留原证据，业务accepted=0。
- 真实独立attempt费用预留、扩大、用量证据、整数结算、未知held与人工证明对账；持久状态/SSE只读及严格日志白名单已写，Get/Resume/Reconcile已映射HTTP。006只追加并保留旧数据，不造调用、价目、费用或token用量种子。
- maximumCost=0不限，日额按site/UTC日/profile；QA另有全站/会话/网络额，background独立。旧unknown held不跨日释放。每attempt固定价目，未知金额省略，人工缺用量必须给真实tokens，不伪0。
- 完整Create/Execute/Dispatch、供应方适配、凭据引用回收、模型space收口和启动/UI仍继续实施，不能宣称可部署费用入口。根负责tokenizeText专用包与公共snapshot/barrier，本域不并写。
- 本机仅设计/源码/Git隐私审阅及diff检查；没有Go/gofmt/gen/build/vet/check/测试/真实接口/模型/服务或push。主会话及时GitLab检查后双推。

以下是历史交付记录。

# 第三轮账本与费用状态只读已交付

更新：2026-10-17。分支codex/model-20261017-r3，基线d760e39194156403e2b82430d5018fa9f4425e19，独占本域。源码提交39b1e33d2f2a0e61298504704f2dd1802fb9d44f。GetLedger/QueryUsage及9项完整助手共11项首次完成函数verified，usage.go、005迁移及两份生成物verified；admin/read.go及完整应用仍partial/blocked，accepted为零。

- 本轮按根裁定只做真实只读与HTTP，ReserveCost/CreateInvocation/ExtendReservation仍planned，未造accepted/held或不可达执行链。background日预算阈值/作用域、maximumCost=0语义与QA已消费签名Permit验证是未来写链前置，本轮不猜。
- 005仅追加nullable owner_service/site_id/created_at，不回填旧归属；真实ledger_read_state和三表变更触发器、分页索引，无费用或价格种子。孤立费用子行422，旧行缺必要归属/时间/空价目前置503，不过滤成空、不跨站展示。
- GetLedger仅portal当前model.manage，正时间窗最多90天；签名cursor绑定caller/actor/site/epoch/session/权限修订、查询窗/limit/costClass及真实int64水位，变更409从头。精确UTC秒/补齐九位纳秒/entryId keyset，验签与签名IO均在SQL外，返回原账单及必需priceSnapshot全值，不按当前价重算。limit按与kin-openapi一致的ParseInt基数0转换已验证值。
- QueryUsage对portal当前model.manage；根AI owner纯服务逐全部root核同caller/site/epoch，多owner403，无持久root404。held按调用created_at窗内当前held，settled按账单created_at窗，是同一当前SQL快照而非历史余额。big.Int汇总后核安全整数，任一组金额未知则完整对应总额省略，未知ID完整排序；超响应/整数预算失败不截断、不冒报部分和。charge_unknown须真实held或未知账单证据，缺失503；全部reservation先核三态，非法422，合法非held不套held金额规则。
- UTC非法时间view/部分索引在窗口筛选前覆盖坏行；日期与整秒使用SQLite内建+0 seconds规范化往返，小数只核1..9位ASCII且不进入日期函数，避免舍入跨秒。未复制标准库历法、未执行SQL或临时程序；只读核官方文档及固定SQLite3.53.4依赖源码。
- 流水线30已取消不计通过；31三个Go分片均通过，但foundation完整check缺本域SigningKey与公共HeartbeatInfo类型，整体失败。只补本域可信platform.SigningKey内部登记，公共类型由根修，未暴露HTTP密钥字段。流水线32完整通过，失败与修正如实保留。
- 2026-10-17 GitLab CI完整候选流水线32，候选ae7b334dbd9f7c102ade242afe8be6c4ccd676f9：生成/重复生成job148、三个同时分片job149/150/151全部success，覆盖14个Go模块-mod=readonly build/vet/gofmt及完整make check，生成补丁0字节。根已精确回传本域18变更路径（含流水线31格式/生成），源码/合同此后未改。仅源码/生成/编译/结构证据；未SQL/HTTP业务验收、新测试、真实模型/供应方、业务worker、服务/镜像或部署。
- 根与代理静态交审已核权限、scope、分页、UTC范围、未知总额与必要证明；本域另只读复审content元数据原回执/CAS/outbox、发布/分享图503及Move根级保留名防绕过，未编辑他域。全部Go生成/编译/check均GitLab，未本机或SSH执行。未新增测试/fixture/mock/样例/临时业务程序，不运行真实接口/模型、worker或部署。暂存/说明/作者/提交者及GPT-6署名静态隐私扫描通过，保留原GitHub代用身份。本域不push，交付后冻结。

下一项：根顺序合入本域源码与记录，复核实际集成树后按本轮授权普通双推；本域冻结，不提前实施费用写链或新模型规则。

# 第二轮真实人工价目与免费报价已交付

更新：2026-10-17。分支codex/model-20261017-r2，基线8a49af65f048a6b160f963570f02d6b88db100de，独占本域。源码提交bea6446d28ed38841aa5b2cb4782209324c2135a，用户本轮授权三个代理并行后由根普通推送。10项首次完成函数及改过的NewAdminHandlers verified（本轮11项完整函数记录）；quote.go、004迁移和两份生成物verified。PutModel/model.go、tokens/budget.go和admin/read.go继续partial，完整应用blocked，accepted为零。

- 新增获根批准004_price_book.sql，无价目种子、默认美元值或费用表。PutModelInput.manualPrice是真实人工价目生产入口；省略保持已有价且摘要JSON字节兼容原协议，对象整份替换，三个USD费率都显式才已知，缺项/空对象明确unknown且不回退，显式0合法。服务器有效时刻、不可变价目与模型CAS/原回执同事务，历史不覆盖；没有伪造remote/bundled生产者。
- QuoteCost严格同代只读，manual>remote>bundled，同source按秒/纳秒及模型版本取最新；输入费/输出费分别大整数向上计算再加显式fixed，超安全整数失败。缺价maximumCost省略并warning；无配置用持久模型版本/updated_at给确定性unknown全值，不登记为历史价目。报价不写缓存、额度、held或ledger，不要求启用/凭据/在线，不调用供应方。
- GetPriceBook只允许portal当前model.manage（由公共Authorize即时校验，非只限超管），按effectiveAt读真实历史，缺价404。QuoteCost只允许根AI任务owner纯服务，核Actor.Kind/SubjectID与CallerService。新POST /v1/token-estimates只localization纯服务；按合同使用不查ModelId存在的通用UTF8 fallback，累计texts/content/text parts，ceil(bytes/2)+32*消息数后乘6/5向上，明确算法版本；图像缺算法503，不伪造上限，不存正文。
- GitLab流水线21首次AI分片失败：MakePriceSnapshot把RFC3339字符串赋给gen.Timestamp（time.Time）；最小修为显式gen.Timestamp(effectiveAt.UTC())。流水线22未包含修复并已取消，不记通过；流水线23完整成功。未手改生成物，根已精确回传。
- 2026-10-17 GitLab CI流水线23，冻结候选e75c80f63c5424fe67960a875b6f0f70df5c91c1：生成/格式及重复生成job112、三分片job113/114/115均成功，覆盖14个Go模块-mod=readonly build/vet/gofmt及完整make check。根从job112精确apply并回传本域20个变更路径；此后源码和合同未改。只表示源码/生成/编译/结构检查，未SQL/HTTP行为验收、业务测试、真实接口/模型、业务worker、服务/镜像或部署。
- 根/代理静态交叉审查已核权限、完整/缺失价格、整数取整、人工优先、只读无费用副作用和既有embedding护栏。本域另只读复审media列表游标、空过滤页、水位和RPC前后快照，未编辑他域。所有Go生成/格式/编译/check均GitLab，未本机或SSH执行；未新增测试/fixture/mock/样例/临时业务程序，未真实接口/模型、worker或部署。暂存/提交说明/作者/提交者及GPT-6署名已静态隐私扫描，保持既有GitHub代用身份，本域不push。

下一项：根顺序合入本域源码与记录，复核实际集成树后按本轮授权普通推送；本域交付后冻结，不提前推进目录刷新、费用执行或归一化设计。

# 单轮供应方与非嵌入模型配置写入已交付

更新：2026-10-17。分支codex/model-20261017-r1，基线e0f307f8c99f7d0a60443d9d81590bdfa6c572db，独占modules/model-gateway/。源码提交0d0b76649bc8ccad043e4e3f4fcd545e28201258；用户当次授权三个子代理并行一轮，根统一检查及普通推送。本轮3项完整函数及provider.go和两份生成物verified；PutModel/model.go因embedding设计护栏整体in_progress/partial，BuildApplication保持blocked，accepted为零。

- 沿既有PutProvider/PutModel.enabled保存配置与启停，不增HTTP。SetProviderEnabled未实现参考职责整合到PutProvider，删除死合同/台账；新增NewCatalogHandlers真实putProvider/putModel映射，公共入口即时superadmin授权一次，核路径与幂等头，no-store。
- PutProvider保持7天完整原结果、严格同代/凭据active指针与全屏障，补version/epoch/credentialVersion条件UPSERT及RowsAffected。停用不改变在途调用或费用记录。
- PutModel先核caller/epoch/完整输入摘要及7天原结果，再核所有非released屏障、同代provider/model与CAS。同modelId的providerId/providerModelVersion/dimension/preprocessorVersion不可变化，身份变化要求新ID；非embedding配置、完整回执与model.changed平台outbox同事务，过期键仅成功新写替换。旧hash不默认为新摘要，仍有效的旧键冲突，不静默重签或删除。
- M07-R06归一化参与空间身份要求与公共五字段schema未收口。本轮有效原回执先行，新的embedding创建/编辑/启停统一DEPENDENCY_UNAVAILABLE、retryable=false，明确合同待确认，写库/回执/outbox前拒绝；不默认归一化、不发布未确认空间，不自动重算付费结果。ValidateModel源码未改，保留其原SHA/partial。
- 2026-10-17 集成会话自部署GitLab CI流水线12，冻结候选fccc3750bd8310ac0c9de5e092f6aceda0158fde：生成/格式/重复gen通过，三个同时分片覆盖14个Go模块-mod=readonly build/vet/gofmt及完整make check通过（878文件/1435函数/379HTTP/2328类型）。本域6份精确格式/生成路径由流水线11产物回传，流水线12无本域新增漂移，源码/合同此后未变。仅源码/生成/编译/结构证据，未SQL/HTTP行为验收、业务测试、真实接口/模型、服务/镜像/部署。
- 检查均按用户本轮指定在自部署GitLab CI执行；此前VPS握手断开，未回退本机。未新增测试/mock/fixture/样例/临时业务程序，无新路径/迁移/外部依赖；未运行真实接口/模型、服务或部署。本域不push，交付后冻结。源提交暂存/说明/作者/提交者及GPT-6署名已静态隐私扫描，保持既有GitHub代用身份。

本轮下一项：由集成会话顺序合入源码和记录，复核实际集成树并按当次授权普通推送；本域冻结。后续若派发模型空间工作包，先由根收口M07-R06与contracts/common.schema.json、现有空间迁移决策，再实现本域声明/算法；本轮不提前编写。

# 持续目标第二轮：管理读取与费用策略已交付

更新：2026-10-16。分支codex/model-goal-r2，基线471c036082e082763d90e1080b3489d4e4531380，独占本域。源码提交14c5d80cb05d28f345c0db84916deb75dd92ec3e，根VPS生成/编译/结构检查与交叉审读通过；本轮14项完整函数verified，ReadModels和ReadAdminCatalog仍in_progress，admin/read.go整体partial，BuildApplication仍blocked。业务验收与部署未开始。

- 新增既有登记internal/admin/read.go及获根批准003_fee_budget_policy.sql；ReadProfiles在真实同代快照按根任务owner/可信op.costClass过滤，portal当前model.manage可全量。ReadModels只完成声明读取，空间身份/归一化历史缺口未修；ReadAdminCatalog四目录同快照与安全供应方投影已写，最近probe生产者/投影未齐，不虚构结果。
- ProviderSummary只查非秘密配置、active凭据版本/材料存在及长度，不SELECT/返回附加头secret引用、nonce/密文正文；credentialConfigured不表示可解密、在线或已经探测，异代或坏长度失败关闭。超响应上限报错，不静默截断。
- policy无行NOT_FOUND；old保留设计只确认网络默认5倍和daily0不限，未确认会话美元数字，部署资料无策略种子；迁移不插金额默认。Put首次expectedVersion0仅当前superadmin显式完整创建；普通guard.manage仅保留当前日额度调整小时设置，daily变化另即时superadmin。
- 当前guard.manage公共授权先于原回放；7天同caller/epoch规范输入完整原结果先于当前policy/CAS/屏障返回。新写预读事务结束后无锁权限RPC，再开短事务重核回执、所有非released屏障、归属/CAS和授权条件；策略、完整回执及本地脱敏审计同事务，不清held/ledger。整体跨域审计投影仍未实现，未宣称通知analytics。
- 五项真实HTTP映射已装配，no-store及幂等头一致；复用公共NormalizeIOError，未复制通用错误机制。公共前置源码7a4f55b、记录ec1732e；完整服务仍因其他handler、权威epoch初始化、恢复及后台缺失拒绝启动。
- 2026-10-16 根集成会话隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：冻结合成树make docs/gen、read.go与wire.go的gofmt、14个Go模块-mod=readonly build/vet/gofmt及完整make check通过（873文件/1343函数/379操作/2300类型）；7路径精确格式/生成回传，他域零漂移。根和content代理交叉审读无新增阻断；公共依赖7a4f55b（记录ec1732e）。仅源码/生成/编译/结构证据，未SQL/HTTP行为验收、业务测试、真实接口/模型、服务/镜像/部署。
- 未本机Go/gofmt/gen/编译/check，未新增测试/mock/fixture/样例/临时业务程序，未执行真实接口/模型、服务或部署。本域不push，交付后冻结。实际暂存、提交消息、作者/提交者和GPT-6署名已静态隐私扫描；保持既有GitHub代用身份。

# AI 接入当前进度

更新：2026-10-16。用户新授权每轮三个子代理并行编写、统一审核及普通push直到代码编写完成，取代此前两轮停止指令。本轮基线685db4f48bb359f79a7501098204ccc5a4cde6b2，分支codex/model-goal-r1，独占本域。源码提交24e4026d452333061cad9ef6abde3050e2944101；免费同代路由已通过根VPS检查与交叉审核，6项完整路由函数、修改的readAvailabilityCandidate及3个完整文件verified。PutProfile源码本轮未修改，保留原实现SHA。

- ResolveRoute通过SelectCandidates在一个严格当前epoch SQL快照内调用ResolveProfile；任务须目录登记且属于可信callerService，costClass只核对可信op。必须真实同代assignment且tier等于目录defaultTier，不合成缺失赋值。
- 先override替换主模型并保序去重，空列表可仅覆盖；仅vision/rerank在目标槽缺失或覆盖后仍空时使用真实同costClass fast配置。vision仍需vision，rerank有override仍需rerank、无override空档才fast/chat；缺配置NOT_FOUND，其他档不回退。
- 全部256项先按安全元数据筛选，保序最多16项；同代模型/供应方/凭据引用及完整性未知失败关闭，eligible embedding全部同空间。返回模型/供应方/凭据版本及路线摘要；没有解密、探测、费用准入或持久派发引用。
- QA免费gate仍只允许QA纯服务、固定三任务、chat能力及原有缺赋值not_configured语义；仅复用共享安全候选读取器。无新增HTTP、迁移、依赖、测试、样例或临时业务程序，未本机Go/gofmt/编译/check、真实接口/模型、部署或本域push。
- 完整服务、权威epoch初始化、费用预留/派发/结算、凭据引用释放及旧密文销毁仍未完成；BuildApplication保持blocked，Rotate整体partial。候选检查不等于业务验收。

- 2026-10-16 根集成会话隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：冻结候选make docs/gen、14个Go模块-mod=readonly build/vet/gofmt及完整make check通过（869文件/1303函数/379操作/2290类型）；profile.go与docs/files/functions精确格式/生成结果已回传，其余无漂移。仅现阶段源码/生成/编译/结构检查，未SQL/HTTP行为验收、业务测试、真实接口/模型、服务/镜像/部署。

以下为既有交付历史，以页首最新授权与progress.json为准。

# AI 接入当前进度

更新：2026-10-16。第二轮（最后一轮），分支codex/model-next-two-r2，基线53acce862467dbe40b829d1984519d78ccac5608；公共可信masterKeyRef前置993904524e4d7a792a6c84a80606de6215f7ace5已正常合入。独占本域，源码提交c82f663bb8f935ff67a8da9e22c463ea0765d816；PutCredential与紧密辅助、HMAC回执、可信引用装配及PutProvider指针保护已verified。Rotate最后引用销毁生命周期未齐，整函数/credential.go仍partial，完整启动blocked。

- 新增已登记internal/security/credential.go，复用001/002 schema，无新迁移或依赖。主密钥只在凭据写入读取：私有路径、NOFOLLOW/SameFile、最多33字节、恰好32字节及末次文件身份核对；启动/免费配置读取只装配引用，不依赖secret文件。
- AES-256-GCM每次新操作随机96bit nonce，完整providerId/credentialVersion规范AAD。严格同代事务核7天原结果、全部非released屏障、真实active版本和供应方双CAS、历史版本唯一，密文/active指针/provider.changed/脱敏回执同事务。未知Commit不自行重试。
- 敏感摘要使用主密钥派生的域分离HMAC，不保存可离线猜测的普通secret SHA；同一部署主密钥是回执协议，本轮不是主密钥轮换，换键不静默改写旧回执。指纹只有截短HMAC与掩码，超过8个Unicode字符才显示末4位；短secret不完整回显。临时可变字节清零，不承诺擦除Go不可变字符串或库内部副本。
- PutProvider创建credentialVersion只能0，编辑只能保留真实active指针，凭据端点独占版本推进。PutCredential可修复active材料缺失且沿真实版本CAS；Rotate要求既有完整active材料并保留旧密文，引用释放后销毁仍缺，Rotate与credential.go整体partial。
- 真实putCredential/rotateCredential映射复用公共当前superadmin授权并核路径/幂等头。完整其他handler、权威epoch初始化、恢复与后台未齐，BuildApplication仍blocked；无模型/费用调用或自动重算。
- 2026-10-16 集成会话VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：最终修正树make docs/gen、credential.go/bootstrap.go/provider.go/wire.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（867文件/1277函数/379操作/2281类型）；41个本域文件的精确格式/生成快照已回传。本阶段仅源码/生成/编译/结构检查，未SQL/HTTP行为验证、业务测试、真实接口/模型、服务/镜像/部署。
- 首次候选完整check因readCredentialReceipt参数未以ctx,op开头失败，修正后上述全量检查通过；PutProvider同时收口7天原结果/所有屏障/caller+epoch摘要与指针旁路。未本机Go检查，暂存内容/提交说明/用户Git代用身份已静态扫描，身份配置未改；未新增测试/mock/fixture/样例/临时验证程序。本域不push，第二轮交付后冻结停止，不启动第三轮。

以下为第一轮及既有交付历史。

# AI 接入当前进度

更新：2026-10-16。用户授权三个子代理并行写两轮、每轮根统一检查并普通push main。当前为第一轮，分支codex/model-next-two-r1，基线d2161f41c441b69f080b7d8cf3d40b88afecb357，独占本域。源码提交82dee76414a823e4d2e71dd87c623d7220f8387e，PutProfile及三个紧密辅助/真实HTTP映射、改过的模型能力校验和本轮生成物verified；profile.go及wire.go整体partial，完整服务仍blocked，业务验收未执行。

- 复用001/002 schema，新增已登记实际源码internal/catalog/profile.go；同事务严格非空epoch、7天原回执、所有非released屏障、配置版本/代次条件CAS与costClass+tier唯一槽。有效原结果先返回，过期键仅随成功新配置事务替换；提交未知不自行重试。
- 有序候选保持原顺序、唯一且最多256，空数组只清空候选而保留配置行。直接能力复用根唯一目录生成的AITierCapabilities与既有模型元数据校验；不要求凭据/价格/供应方在线，不探测、不调用模型、不自动重算嵌入。
- profile.changed/v1与配置、回执同事务写平台outbox，载荷仅profileId/version/dataEpoch及旧/新档位根目录受影响taskIds去重排序；旧/新含fast时保守包含vision/rerank回退任务。HTTP核路径/正文及幂等头一致性，portal写入复用公共入口即时superadmin授权。
- 视觉及rerank档位行缺失或候选为空时的fast回退明确留在ResolveProfile/ResolveRoute参考合同，仍planned；免费可用性读取中已有空候选为已配但unavailable，不能把配置行声称已删除。
- 2026-10-16 集成会话VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：初次同API候选make docs/gen通过；最终fast回退影响修正树make docs、profile.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（861文件/1219函数/379操作/2272类型），精确格式/生成物已回传。本阶段仅源码/生成/编译/结构检查，未SQL/HTTP行为验证、业务测试、真实接口/模型、服务/镜像/部署。
- 本轮无新增迁移、外部依赖、测试/mock/fixture/样例或临时业务验证程序；未本机Go检查、真实接口/模型或部署。暂存内容、提交说明、用户Git代用身份已静态隐私扫描；身份配置未改。本域不push，第一轮交付后冻结，根每轮统一收口；第二轮待根给首轮新基线后派发，不提前实施。

以下为既有交付历史，本次授权取代旧的单轮停止指令。

# AI 接入当前进度

更新：2026-10-16。本轮任务分配配置读写已交付，分支codex/model-task-write-round1，基线170c1dc9057e122e349de43db532b2c48434b6cf，独占本域。源码提交c7c1cac9e3a59a206d506083ace25fd00c077e40；本轮完整Put/Read、2项辅助和修改后的HTTP映射，以及task.go和本轮生成物verified。wire.go整体partial、BuildApplication blocked，业务验收未执行。

- PutTaskAssignment落实superadmin、根唯一任务目录默认档位与覆盖能力、严格非空epoch、同代7天原始回执、非released屏障、SQL条件CAS与同事务assignment.changed平台outbox。先返回有效原回执，不因后来模型停用或本地屏障阻断同代无副作用重放；异输入/异代回执冲突，过期仅同成功新配置事务替换。
- ReadTaskAssignments只读取根目录owner=callerService的已持久配置，显式跨owner拒绝，未配置保持空数组。portal管理员全量读取通过HTTP边界平台identity即时superadmin授权；写入由公共边界统一授权一次，SQL函数信任可信op。路径、正文及幂等头一致性在本域HTTP绑定核实。
- 沿用既有task.go、wire.go及001/002持久结构；根集成会话在既有clients.gen.go生成AITaskCatalog，不手抄目录、不运行时跨域读文件。缺失assignment.changed生产契约已补最小taskId/version/dataEpoch载荷，无新增消费者。公共前置为ab94e1e。
- 本轮未实现档位写入、凭据加密、模型/费用执行、权威epoch初始化、完整HTTP/后台或可运行服务；不以持久schema和映射冒充完整配置运行。
- 2026-10-16 集成会话基于已集成content的模型候选快照：VPS linux/amd64 go1.25.5、GOMAXPROCS=8，make docs/gen、task.go与wire.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt、暂存隐私扫描与完整make check全部通过（861文件/1187函数/379操作/2271类型）。完整本域生成/格式结果由集成会话精确回传，未本机执行Go检查。仅现阶段源码/生成/编译/结构证据，未执行SQL行为验证、业务测试、真实接口或模型、服务/镜像/部署，不代表业务验收。
- 未新增测试/mock/fixture/样例或临时验证程序，未执行push。本轮交付后停止，不自动进入下一包。

以下为先前交付历史，旧停止指令由本次用户授权单轮并行实施取代。


更新：2026-10-16。第六轮只实现QA免费任务配置可用性读取，分支codex/model-task-availability-r1，基线cbe93952464bc9bd4a159e4d870c1b191855bc3a，独占本域。源码、合同与VPS生成产物已交付，完整12项相关函数及对应完整迁移/生成文件verified；task.go和runtime/bootstrap.go各自未完成职责保持partial，完整启动仍blocked，业务验收未执行。

- ReadTaskAvailability仅QA纯服务、固定三任务，一次当前epoch SQL快照；缺任务/QA档位为not_configured，已配无可用元数据候选为unavailable；空/异代、损坏与查询未知失败关闭。
- 单任务覆盖替换默认主模型，保留原第二项起有序备用并去重；免费读取固定QA档位，不要求op.costClass变为qa。凭据只查存在/代次及nonce/ciphertext长度，不SELECT密文/主密钥/URL/价格/预算/余额，不探测或调用供应方。
- 新实际路径仅已登记internal/catalog/task.go及本轮批准追加migrations/002_task_availability.sql；profiles/assignment完整schema、AES-GCM凭据结构，无种子或费用写入。配置写路径及实际加密/轮换仍未实施，不能凭schema宣称已可配置运行。
- BuildApplication真实OpenDatabase/InstallStore并将单项映射交公共完整工厂，其他handler、权威代次初始化、恢复回执与后台缺失仍拒绝启动；不为免费读取调用需masterKey的LoadRuntime，不覆写未知epoch。
- 未新增测试/mock/fixture/样例或临时业务程序；未本机Go/gofmt/gen/编译/vet/check，未SQL业务验证、真实接口/模型、服务、部署或push。本包收尾后按用户要求停止，不开启后包或自行push，集成会话统一复核。
- 源码提交：23db600ead941c088344af2c3a33bf4204277229。2026-10-16 集成会话最终合成快照：VPS linux/amd64 go1.25.5，GOMAXPROCS=8，make docs/gen、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check全部通过（861文件/1182函数/379操作/2269类型）；包含256项持久配置、先eligible过滤后16项路由参考裁决、最终只读代码和002 schema，生成/格式回传无冲突。仅本阶段源码/格式/结构检查，未执行SQL业务验证、业务测试、真实接口/模型、服务、部署或push。

以下为既有交付历史，当前以页首和progress.json为准。

更新：2026-10-15。阶段：R1实施中；本域已完成目录校验、模型身份纯规则和私有SQLite最小持久化切片，尚未业务验收或部署。

- 2026-10-15 集成复核修复：`PutProvider`/`PutModel` 事件改经平台 `outbox.EnqueueEvent` 写 nw_outbox，删除自建 outbox 表（迁移与内联副本），平台转发循环才能投递（ADR0005）。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 已实现：`LoadRuntime` 建立 active/control/restore 目录、严格迁移和32字节主密钥引用检查；`ValidateProvider`/`ValidateModel` 完成无网络纯校验；`PutProvider`/`PutModel` 完成CAS、幂等回执、屏障检查和变更outbox事务。
- 本地提交：`4eec945320f8cf20bfc6673a8c3c8490385aca2f`（实现切片）。
- 当前仍阻塞：BuildApplication 尚未接入本域完整数据库/所有OpenAPI handler、site-config代次接线、activation回执和后台循环；真实模型、连接探测、密钥加密/轮换与费用预留/结算均未实现。
- `EmbeddingSpace` 现有合同没有独立归一化规则字段，本切片只按 provider/modelVersion/dimensions/preprocessingVersion 计算spaceId，矛盾已记录交接；不得自行扩展公共合同。
- 本机无Go、gofmt、make，未执行编译、迁移运行、静态检查或行为验收；未新增测试/样例、未调用真实接口或模型、未push。
- 下一步：先在有Go工具链的VPS完成格式/编译验证，再接入BuildApplication与全部handler；随后实现凭据加密、档位/路由和预留账本。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。

第二轮收口窄修：PutProvider同代普通摘要显式绑定callerService/dataEpoch，7天原结果先于新写屏障和当前active指针返回；新写拒绝任意非released屏障，不按屏障epoch/deadline过滤。过期键仅随成功新配置事务替换，PutModel及其旧共享helper未改。readCredentialReceipt与新增渠道回执参考签名均以ctx,op开头。首次候选Go检查通过但结构check因签名顺序失败，该次不记完整通过；修后根全量生成/编译/结构检查通过，证据见本轮交付记录。
