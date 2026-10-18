# 语言、翻译与术语数据

- objects/<language>/<documentId>/<generation>.json：权威译文文件按blockId保持原树；DB只保存索引/任务/派生数据，manifest原子激活；备份和git镜像通过本域snapshot

- languages(language_tag,version,enabled,default_language,translation_profile,automatic)

- operation_receipts(site_id,actor_id,function_id,idempotency_key,data_epoch,input_hash,result_json,created_at)：语言配置 7 天原回执，不保存模型密钥

- runtime_meta(singleton,data_epoch,updated_at)：本域当前代次水位，不代替 site-config

- local_barrier(singleton,state,data_epoch,updated_at)：语言写入和列表读取的屏障状态。初始 released。AcquireBarrier 尚未实现，本表不是完整屏障协议

- translation_jobs(job_id,document_ref,target_language,state,source_generation,glossary_generation,human_generation)

- translation_units(unit_id,document_id,source_hash,target_language,target_file_ref,target_hash,origin,protected,version)

- translation_manifests(translation_id,publication_ref,target_language,generation,memory_hash,glossary_generation,state)

- glossary(term_id,source_language,target_language,source_term,target_term,scope,version,generation)

- interface_catalog(language,key,source_hash,value_file_ref,placeholders,generation)

- language_preferences(visitor_subject,language,expires_at)

- approved_proposals(proposal_id,proposal_hash,state,term_id)

- cache_answer_translations(translation_id,cache_id,answer_revision,source_hash,target_language,state)

- inbox/outbox(event_id,payload_hash,state)

状态迁移：Language: disabled→enabled→disabled。Translation: queued→running→review→ready|superseded|failed|cancelled；ready→stale。Unit: untranslated→machine|human；human具有protected布尔值。GlossaryProposal: accepted→applied|rejected。

事务规则：聚合变更、版本递增、inbox/outbox同一事务；UTC时间，单调版本，Money为USD整数micros。索引包含owner/siteId/dataEpoch范围；eventId、idempotency key、leaseId唯一约束。写入上限、字段空值和枚举以types.json为准；超出限制返回VALIDATION不部分提交。

## Language

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| tag | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 规范BCP47语言 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| enabled | 是 | `{"type":"boolean"}` | 站点启用 |
| isDefault | 是 | `{"type":"boolean"}` | 默认语言 |
| automaticTranslation | 是 | `{"type":"boolean"}` | 发布自动翻译 |
| profileId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | MG翻译档位 |
| direction | 是 | `{"type":"string","enum":["ltr","rtl"]}` | 排版方向 |
| reflectionEnabled | 是 | `{"type":"boolean"}` | 默认false |
| batchOutputTokens | 是 | `{"type":"integer","minimum":100,"maximum":8192,"default":1000}` | 默认1000 |
| translationMemoryMinimumCharacters | 是 | `{"type":"integer","minimum":1,"maximum":1000,"default":20}` | 默认20 |

## TranslationUnit

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| unitId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 稳定源块标识 |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 源修订 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| sourceHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 源片段hash |
| targetText | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 译文 |
| targetHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 译文hash |
| origin | 是 | `{"type":"string","enum":["machine","human"]}` | 来源 |
| protected | 是 | `{"type":"boolean"}` | 人工保护 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| humanGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 人工修改代数 |
| status | 是 | `{"type":"string","enum":["normal","review","fallback"]}` | 人工变源待复核或失败原文 |
| previousTargetText | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 在新译文完成前交付旧译文 |
| fileRef | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/ObjectRef"}` | 权威译文对象 |

## Translation

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 译文ID |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 权威源 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 目标代数 |
| glossaryGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 术语版本 |
| memoryVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 注入版本 |
| state | 是 | `{"type":"string","enum":["queued","running","review","ready","superseded","failed","cancelled","stale"]}` | 状态 |
| completed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 完成单元 |
| total | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 总单元 |

## GlossaryTerm

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| termId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 术语ID |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| sourceLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 源语言 |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标语言 |
| sourceTerm | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 原词最长256字符 |
| targetTerm | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标词最长256字符 |
| scope | 是 | `{"type":"string","enum":["global","document"]}` | 范围 |
| documentId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | document范围必要 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 全站术语代数 |
| source | 是 | `{"type":"string","enum":["manual","auto"]}` | 词条来源 |
| extractionBatchId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 自动抽取批次可整批撤销 |

## InterfaceString

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| key | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 固定字典key |
| sourceHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 原字符串摘要 |
| value | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标字符串 |
| placeholders | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 严格插值名称 |
| origin | 是 | `{"type":"string","enum":["builtin","machine","human"]}` | 来源 |
| protected | 是 | `{"type":"boolean"}` | 人工保护 |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 版本 |

## ApprovedGlossaryProposal

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposalId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | memory提议ID |
| approvedBy | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 批准人 |
| approvalRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 批准修订 |
| proposalHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 原提议摘要 |
| term | 是 | `{"$ref":"#/$defs/GlossaryTerm"}` | 完整术语 |
| expectedGlossaryGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 原术语代数 |

## CacheAnswerTranslation

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| translationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 译文 |
| cacheId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | QA精选ID |
| answerRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 源答案版本 |
| sourceHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 源回答hash |
| targetLanguage | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标语言 |
| translatedQuestion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 问题译文 |
| translatedAnswer | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 回答译文 |
| state | 是 | `{"type":"string","enum":["queued","running","ready","failed","cancelled","superseded"]}` | 状态 |

## GatewayMessage

引用：`../../model-gateway/contracts/types.json#/$defs/GatewayMessage`。

恢复特殊数据：屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本；导入必须恢复保护位与源ref，恢复切换完成前译文不能交付，不自动重翻。


## 第四轮首页跨域只读协议

listLanguages明确includeDisabled默认false，public-site纯服务仅该只读范围。接口/后台仍按台账未完成，不能据页面进程ready推断语言启用。

R1新增 `003_glossary_catalog.sql`：glossary.deleted及活动scope唯一索引、domain_versions全站术语/界面代数；approved_proposals补siteId/rootOperationId/结果代数，保证跨重试的批准事实绑定。interface_sources为既有固定界面字典来源的本域私有持久表，尚未接入界面写链时不声称已有可用字典。迁移001/002不修改。
