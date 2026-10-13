# 长期记忆、任务映射与记忆助手数据

- objects/<entryId>/<version>.md：权威Markdown含description/lang/source/limit/readonly；current manifest是唯一激活指针，DB仅索引

- memory_entries(entry_id,version,title,file_ref,language,tags,priority,state,body_hash)

- entry_revisions(entry_id,version,body_hash,file_ref,actor_id,created_at)

- task_mappings(mapping_id,task_id,entry_id,order,enabled,language,revision)

- task_budgets(task_id,maximum_bytes,maximum_entries,revision)

- context_snapshots(snapshot_id,task_id,language,mapping_revision,manifest_hash,body_hash)

- assistant_runs(run_id,source_generation,state,invocation_id,actor_id)

- proposals(proposal_id,run_id,kind,state,payload_hash,expected_entry_version,approved_by,approval_revision)

- proposal_commands(command_id,proposal_id,payload_hash,state,localization_receipt)

- inbox/outbox(event_id,payload_hash,state)

- assistant_conversations(conversation_id,actor_id,version,expires_at);assistant_messages(message_id,conversation_id,ordinal,body,run_id)：默认30天可调保留

状态迁移：Entry: active→archived；版本修订不可变。Mapping: enabled|disabled。Assistant: queued→running→review→completed|cancelled|failed|superseded。Proposal: pending→approved→applying→applied|rejected|conflicted；拒绝pending→rejected，批准后不自动回滚。

事务规则：聚合变更、版本递增、inbox/outbox同一事务；UTC时间，单调版本，Money为USD整数micros。索引包含owner/siteId/dataEpoch范围；eventId、idempotency key、leaseId唯一约束。写入上限、字段空值和枚举以types.json为准；超出限制返回VALIDATION不部分提交。

## MemoryEntry

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| entryId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 条目ID |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| title | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 标题≤256字 |
| body | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 人工正文≤entry.maximumBytes默认1536UTF8bytes |
| bodyHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范正文hash |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | BCP47或und |
| tags | 是 | `{"type":"array","items":{"type":"string","minLength":1,"maxLength":4096},"maxItems":256,"uniqueItems":false}` | 去重tag最多16 |
| priority | 是 | `{"type":"integer","minimum":0,"maximum":1000,"default":100}` | 默认100范围0..1000 |
| state | 是 | `{"type":"string","enum":["active","archived"]}` | 状态 |
| updatedAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 修改时间 |
| description | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 注入时说明 |
| maximumBytes | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":1536}` | 单条默认1536可调 |
| readonly | 是 | `{"type":"boolean"}` | 助手禁止修改 |
| source | 是 | `{"type":"string","enum":["manual","assistant","auto"]}` | 来源 |
| autoUpdateEnabled | 是 | `{"type":"boolean"}` | 项目概述自动更新人工编辑后false |
| fileRef | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/ObjectRef"}` | 权威Markdown对象 |

## TaskMapping

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| mappingId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 映射 |
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 固定任务ID |
| entryId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 记忆条目 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 目标BCP47或und |
| order | 是 | `{"type":"integer","minimum":0,"maximum":10000,"default":0}` | 排序0..10000 |
| enabled | 是 | `{"type":"boolean"}` | 启用 |
| revision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 映射CAS版本 |

## TaskBudget

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 固定任务 |
| maximumBytes | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991,"default":5120}` | 默认5120范围1024..65536 |
| maximumEntries | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 最多64条但受5120bytes限制 |
| revision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS修订 |

## ContextManifest

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| snapshotId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 快照 |
| taskId | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 任务 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 语言 |
| entryIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 按注入顺序 |
| entryVersions | 是 | `{"type":"array","items":{"type":"integer","minimum":0,"maximum":9007199254740991},"maxItems":65536}` | 等长版本 |
| mappingRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 映射全局版本 |
| rendererVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 注入渲染器 |
| bodyHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 正文摘要 |
| versionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 完整manifest摘要 |
| byteSize | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | UTF8字节 |

## AssistantRun

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| runId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手任务 |
| sourceGeneration | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输入记忆代数 |
| state | 是 | `{"type":"string","enum":["queued","running","review","completed","cancelled","failed","superseded"]}` | 状态 |
| invocationId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | MG调用 |
| actorId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 发起管理员 |
| proposalIds | 是 | `{"type":"array","items":{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"},"maxItems":256,"uniqueItems":true}` | 候选提议 |

## MemoryProposal

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| proposalId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提议 |
| runId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手 |
| kind | 是 | `{"type":"string","enum":["entry_create","entry_update","entry_archive","glossary"]}` | 类型 |
| state | 是 | `{"type":"string","enum":["pending","approved","applying","applied","rejected","conflicted"]}` | 状态 |
| entryId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 目标条目 |
| expectedEntryVersion | 否；省略不传null | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 输入版本 |
| proposedTitle | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 拟标题 |
| proposedBody | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 拟正文 |
| proposedLanguage | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 拟语言 |
| sourceTerm | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 术语原文 |
| targetTerm | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 术语译文 |
| targetLanguage | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 术语目标 |
| expectedGlossaryGeneration | 否；省略不传null | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 术语输入版本 |
| payloadHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 候选摘要 |
| approvedBy | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 批准人 |
| approvalRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 批准修订 |

## GatewayMessage

引用：`../../model-gateway/contracts/types.json#/$defs/GatewayMessage`。

## AssistantMessage

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| messageId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 助手会话消息 |
| conversationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 管理员助手会话 |
| ordinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 序号 |
| role | 是 | `{"type":"string","enum":["user","assistant"]}` | 角色 |
| body | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 管理员整理请求或助手差异说明 |
| runId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 关联整理任务 |
| createdAt | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Timestamp"}` | 时间 |

恢复特殊数据：屏障停止条目编辑、批准和command dispatch，保留revision history、proposalHash、批准人及未确认批准命令；导入pending command不能重新批准，先按proposalId/hash等持久回执，不重复创建术语。
