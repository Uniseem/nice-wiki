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
