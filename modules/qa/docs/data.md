# 问答、引用与回答缓存数据

- conversations(conversation_id,visitor_subject,language,state,version,expires_at)

- messages(message_id,conversation_id,ordinal,role,text,revision)

- turns(turn_id,root_operation_id,state,request_hash,context_hash,invocation_ids,deadline)

- citations(citation_id,turn_id,chunk_id,publication_ref,block_ref,excerpt_hash)

- turn_events(turn_id,sequence,type,payload,created_at)

- feedback(turn_id,visitor_subject,value,version)

- answer_cache(cache_id,scope_hash,question_hash,answer_revision,approved_by,state,citation_refs,memory_hash)

- cache_vectors(cache_id,space_id,vector,anchor_hash)

- compactions(conversation_id,source_generation,summary,generation,state)

- debug_turns(debug_id,actor_id,namespace,state,expires_at)

- inbox/outbox(event_id,payload_hash,state)

状态迁移：Turn: admitted→queued→retrieving→generating→completed；任一步→cancelled|failed|invalidated。Cache: pending→approved→active→stale|disabled。Compaction: queued→running→committed|superseded|failed。Conversation: active→expired|deleted。

事务规则：聚合变更、版本递增、inbox/outbox同一事务；UTC时间，单调版本，Money为USD整数micros。索引包含owner/siteId/dataEpoch范围；eventId、idempotency key、leaseId唯一约束。写入上限、字段空值和枚举以types.json为准；超出限制返回VALIDATION不部分提交。

## Conversation

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| visitorSubject | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | guard伪名访客 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | BCP47语言 |
| state | 是 | `{"type":"string","enum":["active","expired","deleted"]}` | 状态 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |
| historyGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 历史代数 |
| expiresAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 失效时间 |

## Turn

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| rootOperationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 根操作 |
| state | 是 | `{"type":"string","enum":["admitted","queued","retrieving","generating","completed","cancelled","failed","invalidated"]}` | 状态 |
| questionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范问题hash |
| contextHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 证据及记忆摘要 |
| lastSequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 最后事件 |
| answerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 回答版本 |
| cacheHit | 是 | `{"type":"string","enum":["none","exact","semantic"]}` | 命中类型 |

## Citation

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| citationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 引用 |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 当前发布来源 |
| block | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/BlockRef"}` | 源块 |
| chunkId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | knowledge块 |
| excerptHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 引用片段hash |
| title | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 安全标题 |
| href | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 公开路由 |
| visibilityRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 授权检查版本 |

## AnswerEvent

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| sequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 严格递增SSE序号 |
| event | 是 | `{"type":"string","enum":["accepted","progress","delta","citation","invalidate","completed","error","cancelled"]}` | 事件 |
| delta | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 文本增量 |
| citation | 否；省略不传null | `{"$ref":"#/$defs/Citation"}` | 引用 |
| turn | 否；省略不传null | `{"$ref":"#/$defs/Turn"}` | 终态 |
| code | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 公共错误代码 |

## CacheEntry

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 缓存 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| question | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 公开精选问题 |
| answer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 精选回答 |
| state | 是 | `{"type":"string","enum":["pending","approved","active","stale","disabled"]}` | 状态 |
| questionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范问题 |
| scopeHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 站点配置版本与上下文摘要 |
| memoryVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 注入版本 |
| citations | 是 | `{"type":"array","items":{"$ref":"#/$defs/Citation"},"maxItems":256}` | 引用 |
| approvedBy | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 批准人 |
| answerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 回答版本 |
| curated | 是 | `{"type":"boolean"}` | 是否精选 |
| expiresAt | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 普通缓存TTL精选无此字段 |
| canonicalQuestion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 基准语言完整问题 |
| baseLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 基准语言 |
| questionComplete | 是 | `{"type":"boolean"}` | 必须true |
| localizedAnswers | 是 | `{"type":"array","items":{"$ref":"#/$defs/LocalizedAnswer"},"maxItems":256}` | 多语言答案 |

## Message

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messageId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 消息 |
| ordinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 会话序号 |
| role | 是 | `{"type":"string","enum":["user","assistant"]}` | 角色 |
| text | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 消息正文 |
| turnId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 轮次 |
| revision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 消息版本 |

## DebugTurn

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| debugId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 诊断ID |
| actorId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 管理员 |
| namespace | 是 | `{"type":"string","enum":["debug"]}` | 隔离空间 |
| turn | 是 | `{"$ref":"#/$defs/Turn"}` | 诊断轮次 |
| expiresAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 默认7天过期 |

## Compaction

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| compactionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 压缩 |
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 会话 |
| sourceGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输入历史代数 |
| summary | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 派生摘要 |
| summaryHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 摘要hash |
| state | 是 | `{"type":"string","enum":["queued","running","committed","superseded","failed"]}` | 状态 |
| previousSummaryHash | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 前次摘要 |
| newMessageStartOrdinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 增量起始 |
| newMessageEndOrdinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 增量排他结束 |
| retainedFromOrdinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 保留最近历史起点 |

## QaPolicy

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 策略版本 |
| enabled | 是 | `{"type":"boolean"}` | 公开问答启用 |
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 答案档位 |
| retrievalSpace | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 检索空间 |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":1500}` | 默认1500 |
| maximumHistoryTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 模型较小窗口历史预算 |
| maximumContextTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":4096}` | 默认4096 |
| semanticThreshold | 是 | `{"type":"number","minimum":0,"maximum":1}` | 阈值模式可调默认0.94 |
| maximumQuestionCharacters | 是 | `{"type":"integer","minimum":1,"maximum":10000,"default":500}` | 默认500 |
| preflightOutputTokens | 是 | `{"type":"integer","minimum":1,"maximum":8192,"default":500}` | 默认500 |
| inspectionOutputTokens | 是 | `{"type":"integer","minimum":1,"maximum":8192,"default":800}` | 默认800 |
| maximumSubqueries | 是 | `{"type":"integer","minimum":1,"maximum":4,"default":4}` | 默认4 |
| maximumRetrievalRounds | 是 | `{"type":"integer","minimum":1,"maximum":3,"default":3}` | 默认3 |
| maximumToolCalls | 是 | `{"type":"integer","minimum":1,"maximum":5,"default":5}` | 默认5 |
| semanticMode | 是 | `{"type":"string","enum":["ai_equivalence","threshold"]}` | 默认ai_equivalence |
| cacheTtlDays | 是 | `{"type":"integer","minimum":1,"maximum":365,"default":7}` | 普通缓存默认7天 |
| recordRetentionDays | 是 | `{"type":"integer","minimum":1,"maximum":3650,"default":30}` | 问答记录默认30天可调 |
| compactionEnabled | 是 | `{"type":"boolean"}` | 默认true |
| retainedRecentTokens | 是 | `{"type":"integer","minimum":1,"maximum":128000,"default":15000}` | 默认15000 |
| contextReservePercent | 是 | `{"type":"integer","minimum":1,"maximum":50,"default":10}` | 默认10 |

## GatewayMessage

引用：`../../model-gateway/contracts/types.json#/$defs/GatewayMessage`。

## LocalizedAnswer

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 访客语言 |
| answer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 对应语言文本 |
| sourceAnswerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 基准回答版本 |
| translationVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 翻译规则及来源摘要 |

## QuestionPlan

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| originalQuestion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原问题 |
| canonicalQuestion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 完整基准语言问题 |
| visitorLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 回答语言 |
| complete | 是 | `{"type":"boolean"}` | 意思完整且无需缺失指代 |
| cacheEligible | 是 | `{"type":"boolean"}` | 完整非个人化且非敏感动态问题 |
| subqueries | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 最多4个完整子问题 |
| classification | 是 | `{"type":"string","enum":["smalltalk","out_of_scope","simple","complex","needs_clarification"]}` | 类型 |
| risk | 是 | `{"type":"string","enum":["normal","suspicious","abusive"]}` | 并入前置模型风险 |
| contextFingerprint | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 当前页历史及记忆摘要 |

## QueryEmbedding

引用：`../../knowledge/contracts/types.json#/$defs/QueryEmbedding`。

恢复特殊数据：屏障停止新turn/cache审批/压缩提交；取消活动turn并持久终态和原MG调用引用；导入的visitor cookie不得恢复授权，恢复dataEpoch使旧流/permit失效，原消息只经重新授权可读。


## 最终同步 QaPolicy

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 策略版本 |
| enabled | 是 | `{"type":"boolean"}` | 公开问答启用 |
| profileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 答案档位 |
| retrievalSpace | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 检索空间 |
| maximumOutputTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":1500}` | 默认1500 |
| maximumHistoryTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 模型较小窗口历史预算 |
| maximumContextTokens | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":4096}` | 默认4096 |
| semanticThreshold | 是 | `{"type":"number","minimum":0,"maximum":1}` | 阈值模式可调默认0.94 |
| maximumQuestionCharacters | 是 | `{"type":"integer","minimum":1,"maximum":10000,"default":500}` | 默认500 |
| preflightOutputTokens | 是 | `{"type":"integer","minimum":1,"maximum":8192,"default":500}` | 默认500 |
| inspectionOutputTokens | 是 | `{"type":"integer","minimum":1,"maximum":8192,"default":800}` | 默认800 |
| maximumSubqueries | 是 | `{"type":"integer","minimum":1,"maximum":4,"default":4}` | 默认4 |
| maximumRetrievalRounds | 是 | `{"type":"integer","minimum":1,"maximum":3,"default":3}` | 默认3 |
| maximumToolCalls | 是 | `{"type":"integer","minimum":1,"maximum":5,"default":5}` | 默认5 |
| semanticMode | 是 | `{"type":"string","enum":["ai_equivalence","threshold"]}` | 默认ai_equivalence |
| cacheTtlDays | 是 | `{"type":"integer","minimum":1,"maximum":365,"default":7}` | 普通缓存默认7天 |
| recordRetentionDays | 是 | `{"type":"integer","minimum":1,"maximum":3650,"default":30}` | 问答记录默认30天可调 |
| compactionEnabled | 是 | `{"type":"boolean"}` | 默认true |
| retainedRecentTokens | 是 | `{"type":"integer","minimum":1,"maximum":128000,"default":15000}` | 默认15000 |
| contextReservePercent | 是 | `{"type":"integer","minimum":1,"maximum":50,"default":10}` | 默认10 |
| cacheEnabled | 是 | `{"type":"boolean","default":true}` | 独立缓存总开关；关闭仍存问答记录，不查/写exact/semantic/多语言缓存 |


## 第四轮首页跨域只读协议

新增readPublicCapability免费只读入口配置能力，未知503由首页禁用QA按钮但保留hero。生产方函数planned，不以readyz或无参数假成功充当AI已配置。
