# 统一模型网关数据

- providers(provider_id,version,enabled,base_url,adapter,credential_version)

- models(model_id,version,provider_id,provider_model_version,capabilities,dimension,preprocessor_version)

- profiles(profile_id,version,cost_class,tier,ordered_model_ids_json,maximum_cost_micro_usd,concurrency,daily_limit_cost_micro_usd,data_epoch,updated_at)

- task_assignments(task_id,tier,override_model_id,version,data_epoch,updated_at)

- credentials(provider_id,credential_version,nonce,ciphertext,data_epoch)

- invocations(invocation_id,request_hash,root_operation_id,cost_class,state,deadline,price_snapshot)

- reservations(reservation_id,invocation_id,maximum_micro_usd,held_micro_usd,state)

- ledger(entry_id,invocation_id,provider_request_id,actual_micro_usd,usage_hash,price_snapshot)

- stream_events(invocation_id,sequence,event,payload,created_at)

- outbox(event_id,event_type,payload,published_at)

状态迁移：Invocation: accepted→reserved→dispatching→streaming→succeeded|failed|cancel_requested→cancelled|charge_unknown；charge_unknown→reconciled。Reservation: held→settled|released；未知账单保持held。Provider: disabled|enabled。

事务规则：聚合变更、版本递增、inbox/outbox同一事务；UTC时间，单调版本，Money为USD整数micros。索引包含owner/siteId/dataEpoch范围；eventId、idempotency key、leaseId唯一约束。写入上限、字段空值和枚举以types.json为准；超出限制返回VALIDATION不部分提交。

## Provider

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| providerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 供应方ID |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| adapter | 是 | `{"type":"string","enum":["openai_chat","anthropic_messages","gemini","openai_embeddings","gemini_embeddings","rerank"]}` | 适配协议 |
| baseUrl | 是 | `{"type":"string","format":"uri","maxLength":2048}` | HTTPS供应方API根地址 |
| enabled | 是 | `{"type":"boolean"}` | 是否启用 |
| credentialVersion | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 密钥版本 |
| connectTimeoutMs | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 连接超时上限10000 |
| requestTimeoutMs | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 请求超时上限180000 |
| displayName | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 供应方显示名 |
| extraHeaderNames | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 允许附加头名 |
| extraHeaderSecretRefs | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 等长加密secret引用 |
| concurrencyLimit | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 供应方并发上限 |
| streamIdleTimeoutMs | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 流片段间隔超时 |

## Model

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 网关模型ID |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| providerId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 供应方ID |
| providerModelVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 不可变供应方模型版本 |
| capabilities | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | chat vision structured reasoning embedding rerank translation能力集合 |
| dimension | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 嵌入维度chat为0 |
| preprocessorVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 预处理版本 |
| maxInputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输入上限 |
| maxOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输出上限 |
| catalogOrigin | 是 | `{"type":"string","enum":["bundled","remote","manual"]}` | 模型目录来源 |
| manualOverride | 是 | `{"type":"boolean"}` | 手动覆盖目录 |
| enabled | 是 | `{"type":"boolean"}` | 是否显示并可分配 |

## Profile

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 档位ID |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| costClass | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/CostClass"}` | 可信费用类别 |
| tier | 是 | `{"type":"string","enum":["fast","accurate","vision","embedding","rerank"]}` | 五档 |
| orderedModelIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 第一为主其余有序备用 |
| maximumCost | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 每请求费用上限；Money.currency=USD且micros非负整数 |
| concurrency | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 并发上限 |
| dailyLimitCost | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 日消费额度；Money.currency=USD且micros非负整数 |

## Invocation

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用ID |
| rootOperationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 根操作ID |
| costClass | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/CostClass"}` | 可信费用类别 |
| state | 是 | `{"type":"string","enum":["accepted","reserved","dispatching","streaming","succeeded","failed","cancel_requested","cancelled","charge_unknown","reconciled"]}` | 调用状态 |
| reservationId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 预留ID |
| providerRequestId | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 供应方请求ID |
| deadline | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 全链路截止 |
| priceSnapshotId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 固定价目快照 |
| lastSequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | SSE最后序号 |
| attempts | 是 | `{"type":"array","items":{"$ref":"#/$defs/Attempt"},"maxItems":256}` | 全部独立计费尝试 |

## Reservation

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| reservationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 预留ID |
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用ID |
| state | 是 | `{"type":"string","enum":["held","settled","released"]}` | 预留状态 |
| maximumCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 最大收费；Money.currency=USD且micros非负整数 |
| heldCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 仍占用额度；Money.currency=USD且micros非负整数 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| pricingKnown | 是 | `{"type":"boolean"}` | 未知价不伪装0元 |
| unknownUsageCount | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 未知价使用量记录 |

## LedgerEntry

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entryId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 不可变账本ID |
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用ID |
| providerRequestId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 供应方幂等收费标识 |
| actualCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 最终费用；Money.currency=USD且micros非负整数 |
| inputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 供应方输入token |
| outputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 供应方输出token |
| status | 是 | `{"type":"string","enum":["settled","reconciled","price_unknown"]}` | 结算来源 |
| createdAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 记账时间 |
| attemptId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 账本所属尝试 |
| pricingKnown | 是 | `{"type":"boolean"}` | 最终金额是否已知 |

## StreamEvent

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 严格递增序号 |
| event | 是 | `{"type":"string","enum":["accepted","delta","usage","completed","error","cancelled"]}` | SSE事件名 |
| delta | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 文本增量 |
| invocation | 否；省略不传null | `{"$ref":"#/$defs/Invocation"}` | 状态快照 |
| code | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 终态错误代码 |

## PriceSnapshot

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| priceSnapshotId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 快照ID |
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 模型ID |
| inputPerMillionCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 百万输入token费率；Money.currency=USD且micros非负整数 |
| outputPerMillionCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 百万输出token费率；Money.currency=USD且micros非负整数 |
| fixedCost | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 每调用固定费；Money.currency=USD且micros非负整数 |
| currency | 是 | `{"type":"string","enum":["USD"]}` | 结算币种 |
| effectiveAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 生效时刻 |
| pricingKnown | 是 | `{"type":"boolean"}` | 是否可预算 |
| warning | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 缺价无法完整计入费用上限的原产品提示 |

## GatewayMessage

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| role | 是 | `{"type":"string","enum":["system","user","assistant"]}` | 消息角色 |
| content | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | UTF8文本不超过64KiB |
| parts | 否；省略不传null | `{"type":"array","items":{"$ref":"#/$defs/GatewayPart"},"maxItems":256}` | 可选视觉输入片段 |

## Attempt

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| attemptId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 尝试ID |
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 统一调用 |
| ordinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 尝试序号 |
| modelId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 固定模型 |
| state | 是 | `{"type":"string","enum":["reserved","dispatching","streaming","succeeded","failed","charge_unknown","reconciled"]}` | 状态 |
| reservationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 独立费用预留 |
| providerRequestId | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 收费标识 |
| retryReason | 是 | `{"type":"string","enum":["initial","fallback","structured_retry"]}` | 原因 |

## TaskAssignment

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 根唯一任务目录ID |
| tier | 是 | `{"type":"string","enum":["fast","accurate","vision","embedding","rerank"]}` | 默认档位 |
| overrideModelId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 单任务覆盖模型 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |

## GatewayPart

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| kind | 是 | `{"type":"string","enum":["text","image"]}` | 输入片段 |
| text | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 文字仅text必要 |
| image | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 图像引用仅image必要 |

恢复特殊数据：屏障停止新预留/新派发；在途请求本地终止后标charge_unknown保留held与providerRequestId，不自动记零费用。导出包含不可变账本和预留证据，密钥仅在includeSecrets=true时导出加密信封；部署主密钥不导出。缺密钥恢复后所有供应方disabled。


## 最终同步 FeeScopeBinding

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 调用 |
| rootOperationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 可信根 |
| admissionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 已验证permit |
| sessionBudgetScope | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | guard签名会话伪名 |
| networkBudgetScope | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | guard签名网络伪名，不存IP |
| dataEpoch | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 代际 |
| expiresAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 签名期限 |


## 最终同步 FeeBudgetPolicy

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| version | 是 | `{"type":"integer","minimum":0}` | CAS修订 |
| hourlyLimitsEnabled | 是 | `{"type":"boolean"}` | old会话/网络小时费用限制总开关，关闭仍记账 |
| sessionHourlyLimit | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 会话每小时额度，0不限；old未明确具体美元默认，使用已有部署配置不虚构金额 |
| networkMultiplier | 是 | `{"type":"integer","minimum":1,"maximum":100,"default":5}` | network额度=session额度×倍率，old默认5，可修改 |
| dailyQaLimit | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Money"}` | 站点每天qa上限，0不限，background独立 |


## 第四轮首页跨域只读协议

新增readTaskAvailability固定QA链三任务的免费配置投影。缺价格按原产品不阻断，不查询余额，不调用供应方或模型；当前实现状态与检查证据见STATUS和progress.json。

## 第六轮只读配置快照与追加结构

002_task_availability.sql只追加任务分配、完整档位字段与凭据schema，无种子配置或费用记录；不改001。profiles以(data_epoch,cost_class,tier)唯一，防止同代同类别同档位歧义。凭据以provider/version唯一，AES-GCM使用12字节nonce，ciphertext须包含16字节tag及非空secret密文；本包不实现凭据写入、加密或轮换。

读取一次事务先核runtime_meta当前代次，未知空水位返回503、异代返回409，不写入猜测代次。三任务按请求顺序共用该快照，仅选择任务/候选引用、启用与能力字段、凭据存在性/长度；不SELECT密文、主密钥、供应方地址、价格、预算或余额。空任务分配/QA档位为not_configured；已配置无可用候选为unavailable；未知查询或坏数据失败关闭。附加头仅核既有名称/引用数组的结构一致性，不新增secret-ref存储或证明可解密。

任务覆盖替换有序列表首项，保留第二项起的原有备用顺序并去重；原列表为空时覆盖可作为唯一候选，被替换主模型不另加备用。配置数组上限为公开schema的256项；先按完整配置过滤本地eligible，执行ResolveRoute再保序返回至多16项，不能在过滤前截断。免费读取不返回候选列表，只判有无可用配置。免费读取固定选择costClass=qa档位，不要求请求op.costClass=qa；响应不含内部ID、URL、引用或费用配置，不是执行授权或在线保证。

## 任务赋值写入与原结果

本轮复用task_assignments及operation_receipts，不新增表或迁移。assignment.version为expectedVersion+1；首次expectedVersion=0，更新SQL以task_id/version/data_epoch作条件并核RowsAffected。任何非released本地屏障阻止新配置写入，不因其epoch或deadline过期绕过。

回执主键仍为site_id/actor_id/function_id/idempotency_key；input_hash包含callerService、dataEpoch、assignment及expectedVersion，result_json保存完整PutTaskAssignmentResult。7天有效原结果在屏障与当前模型校验之前返回；坏时间/原结果失败关闭，过期查询无写入，只有成功新配置事务末尾替换旧回执。assignment.changed/v1只发布taskId/version/dataEpoch，与赋值和回执通过平台nw_outbox同事务提交。

## 档位写入与原结果

复用002的profiles及001的operation_receipts，不新增迁移。profile.costClass是目标配置类别，不覆盖可信op.costClass。profileId、当前version和dataEpoch共同做条件CAS；同代costClass+tier只允许一个profileId占用，迁移到已有槽返回CONFLICT，空槽允许。orderedModelIds最多256且保序唯一，空数组只清候选、保留行。

7天回执摘要绑定callerService/dataEpoch/profile/expectedVersion，有效原结果先于新写屏障、候选与CAS检查返回；过期只在成功新配置事务替换。配置、profile.changed/v1与完整PutProfileResult回执同事务；事件taskIds来自根目录旧/新tier筛选并去重排序；旧/新含fast时保守包含vision/rerank回退任务，即使独立槽已配置，不广播候选、费用或秘密。

## 凭据密文与敏感原回执

复用002 credentials，providerId/credentialVersion是不可复用历史键；nonce固定12字节，AES-256-GCM AAD为规范JSON {format:credential-aes256gcm-v1,providerId,credentialVersion}。active指针只由专用凭据事务推进，providers.version同时递增；PutProvider创建只能0、编辑保留真实指针。旧密文保留，引用释放与销毁尚未交付。

operation_receipts.input_hash对于凭据写入是同一部署主密钥域分离派生HMAC，绑定caller/epoch/function/provider/version/secret，不是普通secret SHA。回执只含脱敏响应，fingerprint为掩码/截短HMAC，短secret不完整回显。主密钥不入库，换主密钥属于另行迁移协议，不能静默重签旧回执。

第二轮收口窄修：PutProvider同代普通摘要显式绑定callerService/dataEpoch，7天原结果先于新写屏障和当前active指针返回；新写拒绝任意非released屏障，不按屏障epoch/deadline过滤。过期键仅随成功新配置事务替换，PutModel及其旧共享helper未改。readCredentialReceipt与新增渠道回执参考签名均以ctx,op开头。首次候选Go检查通过但结构check因签名顺序失败，该次不记完整通过；修后根全量生成/编译/结构检查通过，证据见本轮交付记录。

## 免费路线内部快照

ResolveProfileInput/ResolveRouteInput仅taskId和costClass，属于内部参考DTO，不改变HTTP输入。ResolveProfileResult保存原始真实profile、assignment及覆盖/回退派生的orderedModelIds、requiredCapabilities、fallback。ResolveRouteResult保存同一SQL快照的resolved、至多16项RouteCandidate及routeVersion；候选只带模型安全元数据、providerVersion与credentialVersion，没有秘密或持久派发引用。routeVersion绑定epoch、实际配置与最终候选，按固定JSON字段顺序SHA256计算；它不替代后续费用预留或派发前版本/epoch核对。

## 管理目录与费用策略持久化

003只追加fee_budget_policy单例（site/epoch/version、小时开关、USD整数会话额度、网络倍率、日QA额度）和fee_policy_audit。初始无行，读取NOT_FOUND，当前超管显式提交完整策略创建；不插入金额默认或probe种子。审计仅actor/rootOperation/epoch/version/time和日额度变化标记，与策略及完整回执同事务；整体跨域审计投影仍待后包。

ProviderSummary为安全管理投影，不包含extraHeaderSecretRefs或密文。credentialConfigured只表示当前凭据材料存在、同代且长度完整；不证明可解密、供应方在线或最近探测。Model声明读取不计算space，归一化历史缺口尚未完成，ReadModels整函数partial。

## 供应方与模型配置写入闭环

启停沿既有PutProvider/PutModel.enabled字段，无独立启停API；内部未实现SetProviderEnabled参考项已合并到PutProvider。供应方条件UPSERT同时绑定真实version/epoch/credentialVersion，RowsAffected必须恰好1；凭据active指针仍只由凭据端点推进。

PutModel在同一严格当前代次事务先核7天原结果，再核所有非released屏障、真实同代provider及model版本。现有modelId的providerId/providerModelVersion/dimension/preprocessorVersion不可变化，变化须新ID；条件UPSERT只改可变元数据与enabled，并核RowsAffected。停用不清任务分配/档位，不改在途调用、held或ledger，后续路由读取真实enabled拒绝新候选。

模型回执摘要绑定callerService/dataEpoch/model/expectedVersion，完整原结果早于当前配置/屏障返回。旧摘要不能默认为新摘要；7天内hash不匹配CONFLICT，不重签旧结果。过期仅成功新事务替换；配置、现有model.changed平台outbox与回执原子提交。现有嵌入space算法与归一化迁移缺口本轮未扩写。HTTP核路径和非空幂等头与正文一致，由公共入口即时superadmin授权；完整服务缺其他handler仍拒绝启动。

M07-R06要求归一化参与空间身份，但现公共schema与旧算法只含五字段，缺确认规则与迁移。新PutModel对embedding创建/编辑/启停明确返回DEPENDENCY_UNAVAILABLE（合同待确认，非运行服务故障），在写库/回执/outbox前拒绝；有效原回执仍先行，非embedding配置照常。完整PutModel/model.go保持partial，不发布未收口的空间身份。

## 第二轮价目与免费预算

004只追加不可变price_book_entries，无价目种子。真实人工生产沿PutModelInput.manualPrice：省略保留，传对象整份替换，所有费率显式USD才已知；缺项（含固定费）unknown且不回退其他source，显式0合法。服务器取有效秒/纳秒，与模型CAS/回执同事务；省略新字段时摘要JSON旧字节不变。目录刷新未实现，不造remote/bundled数据。

读取manual优先，其次remote/bundled，同source取不晚于目标时刻的最新秒/纳秒和版本。GetPriceBook无历史404；QuoteCost无配置用模型持久版本/updated_at构造确定性unknown，不当历史记录。全程只读报价，不占额度或写账。已知价格用大整数分别ceil(inputTokens*inputRate/1000000)、ceil(maxOutputTokens*outputRate/1000000)，再加显式fixed，超安全整数失败；缺价省略maximumCost并warning，不伪造零。

/v1/token-estimates仅localization纯服务。无内置tokenizer时，累计texts/消息content/text parts的UTF8字节，ceil(bytes/2)+32*消息数后乘6/5向上，返回固定算法版本；message.content遵守64KiB字节预算，图片算法未确认返回503。不联网或保留输入，新增入口由真实本地化估算依赖调用。

## 第三轮账本与费用状态读取

005给invocations仅追加nullable owner_service/site_id/created_at，不回填旧归属。ledger_read_state在invocations/reservations/ledger_entries任一变化时同事务推进int64水位，缺行或耗尽失败关闭；分页索引按UTC秒/补齐九位纳秒/entryId精确排序，不按可变小数RFC3339字符串猜顺序。

GetLedger只portal当前model.manage，时间窗正且最多90天，签名cursor绑定请求和当前主体/权限/站点/epoch及水位，变化409重头；验签和签名在SQL外。账单返回原固定priceSnapshot全值。旧行缺归属/时间/空价目前置503，孤立费用行、损坏/矛盾422，不补、丢弃或当成空。

QueryUsage的portal管理员即时核model.manage；AI owner纯服务要求整个root持久归属均同caller/site/epoch，多owner403，无root404。held只汇总调用created_at在窗内且当前held的预留，settled按ledger.created_at窗；同一当前SQL快照，不是历史余额。组内任一未知省略完整相应总额，未知调用ID完整排序；空且全已知的组可为0，不能用部分和代表全额。big.Int聚合后核安全整数，超过256记录/未知ID或可表示范围失败不截断。

本轮不实施Reserve/Create/Extend。background日预算作用域与maximumCost=0含义、QA已消费签名Permit验证须先明确，不能用请求scope或伪调用绕开；不产生accepted/held、真实收费或模型派发。

账本时间合法性由SQLite内建strftime对前19位加0秒规范化往返，精确小数独立核1..9位，非法时间元数据view/部分索引保证坏行不能被窗口隐去；不手写日历，也不把读预算变成业务总量上限。依据为[SQLite日期函数](https://www.sqlite.org/lang_datefunc.html)及[官方往返说明](https://sqlite.org/forum/info/ae8081b04cef606c85a4ccd63903a104de1210ac3608808825b9c817809dcf46)，并只读核固定依赖SQLite3.53.4源码路径，未运行SQL或临时程序。窗内charge_unknown须有实际held或未知金额账单证明，否则503而非零占用。HTTP整数limit保持与入口ParseInt基数0相同语义。

同root全部reservation先核held/settled/released合同三态，非法状态422而非过滤成空；合法settled/released不聚合，不套held金额字段规则，仅held参与当前预留校验和汇总。
