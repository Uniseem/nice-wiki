# 知识入库与混合检索数据

- index_config(config_id,version,enabled,embedding_profile,space,chunk_size,overlap)

- builds(build_id,publication_ref,space_id,state,lease_id,source_hash,generation)

- chunks(chunk_id,build_id,block_id,chunk_hash,text,ordinal,byte_start,byte_end)

- vectors(chunk_id,space_id,dimension,vector_hash,vector)

- terms(chunk_id,language,term,frequency,length)

- active_indexes(document_id,space_id,build_id,revision,visibility_revision)

- extractions(extraction_id,asset_id,asset_hash,extractor_version,text_ref,status)

- inbox(event_id,received_at,state);outbox(event_id,payload,published_at)

状态迁移：Build: queued→fetching→extracting→chunking→embedding→validating→active；任一步→cancelled|failed|superseded。Extraction: queued→running→ready|unsupported|failed。IndexGeneration: staging→active→retired；一次文档/空间只有一份active。

事务规则：聚合变更、版本递增、inbox/outbox同一事务；UTC时间，单调版本，Money为USD整数micros。索引包含owner/siteId/dataEpoch范围；eventId、idempotency key、leaseId唯一约束。写入上限、字段空值和枚举以types.json为准；超出限制返回VALIDATION不部分提交。

## IndexConfig

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| configId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 配置ID |
| version | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | CAS版本 |
| enabled | 是 | `{"type":"boolean"}` | 入库启用 |
| embeddingProfileId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 嵌入档位 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 不可变空间 |
| chunkTokens | 是 | `{"type":"integer","minimum":128,"maximum":1024,"default":512}` | 默认512范围128..1024 |
| overlapTokens | 是 | `{"type":"integer","minimum":0,"maximum":128,"default":64}` | 默认64范围0..128 |
| language | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | BCP47源语言 |
| rerankProfileId | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 可选重排档位 |

## IndexBuild

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建ID |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 发布引用 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 嵌入空间 |
| state | 是 | `{"type":"string","enum":["queued","fetching","extracting","chunking","embedding","validating","active","cancelled","failed","superseded"]}` | 构建状态 |
| generation | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | staging代数 |
| completedChunks | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 完成数量 |
| totalChunks | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 总数 |
| failureCode | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 安全错误码 |

## Chunk

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| chunkId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 分块ID |
| document | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/PublishedRevisionRef"}` | 权威来源 |
| block | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/BlockRef"}` | 源块 |
| ordinal | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 文内序号 |
| chunkHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范文本hash |
| text | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 仅内部检索正文 |
| tokenCount | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 分块token |
| byteStart | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 源UTF8起点 |
| byteEnd | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 排他终点 |
| asset | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 资源派生来源 |

## Candidate

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| chunk | 是 | `{"$ref":"#/$defs/Chunk"}` | 分块 |
| lexicalRank | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 词项名次0为缺失 |
| vectorRank | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 向量名次0为缺失 |
| score | 是 | `{"type":"number","minimum":0,"maximum":1}` | RRF分数 |
| visibilityRevision | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 权威可见性代数 |

## SearchResult

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| candidates | 是 | `{"type":"array","items":{"$ref":"#/$defs/Candidate"},"maxItems":256}` | 可引用材料 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 所用空间 |
| indexVersionHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 所有active版本摘要 |
| degraded | 是 | `{"type":"boolean"}` | 是否词项降级 |
| truncated | 是 | `{"type":"boolean"}` | 预算是否截断 |

## Extraction

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| extractionId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 提取ID |
| asset | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/AssetRef"}` | 资源引用 |
| extractorVersion | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 提取器版本 |
| state | 是 | `{"type":"string","enum":["queued","running","ready","unsupported","failed"]}` | 状态 |
| text | 否；省略不传null | `{"type":"string","minLength":1,"maxLength":65536}` | 内部文本 |
| contentHash | 否；省略不传null | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 提取文本摘要 |
| pageCount | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 页数或帧数 |

## BuildProgress

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| buildId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 构建ID |
| state | 是 | `{"type":"string","minLength":1,"maxLength":65536}` | 构建状态 |
| completed | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 完成单元 |
| total | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 总单元 |
| sequence | 是 | `{"type":"integer","minimum":0,"maximum":9007199254740991}` | 事件序号 |

## GatewayMessage

引用：`../../model-gateway/contracts/types.json#/$defs/GatewayMessage`。

## QueryEmbedding

| 字段 | 必填 | 类型/约束 | 含义 |
|---|---|---|---|
| queryHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 规范基准查询 |
| space | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/EmbeddingSpace"}` | 完整空间 |
| vector | 是 | `{"type":"array","items":{"type":"number"},"minItems":1,"maxItems":65536}` | 向量 |
| vectorHash | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Hash"}` | 向量摘要 |
| invocationId | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 原MG调用 |
| dataEpoch | 是 | `{"$ref":"../../../contracts/common.schema.json#/$defs/Identifier"}` | 代际 |

恢复特殊数据：屏障停止新build、active切换和staging写入，等待reader lease排空；导出active manifest/分块/词项/向量与空间身份。导入空间不匹配的代数隔离且不得自动发模型重建；派生索引可由管理员另发重建任务。

## 2026-10-18 实施收口：独立 SQL 代数与证据映射

本域 schemaVersion 1 的实际迁移为 `migrations/001_initial.sql`，仍只访问本域私有 SQL。`index_identity` 固定站点和代次；`builds` 保存完整发布引用、完整空间、固定嵌入档位和语言、租约、独立 generation、分块计数、清单摘要及一秒内的权威观察时间。`generation_state` 独立记录 staging/active/retired，不把已成功构建的历史结果改成非法 Build 枚举。`active_indexes` 每站点、代次、文档与 spaceId 只有一个指针。

分块、词项和向量均存本域 SQL，不另造第二份索引文件。清单是按 ordinal 排序的完整 Chunk、固定版本词频/长度、向量及 vectorHash 的规范 JSON SHA-256；读者用同一只读事务固定活动指针和全部记录，核对完整空间与清单后将材料载入内存。读事务结束后不持有可删除的索引文件句柄。`ValidateBuild` 对比分块/词项/向量数量、重算确定词项、核每个来源和向量摘要，观察 content 权威后重新打开当前租约写事务，清单未变才保存证明。`ActivateBuild` 在当前安全写窗口内检查观察时间不超过一秒，CAS 预期活动代数，并同事务写 `index.ready` outbox；旧代数保留。

Chunk 新增必填 `sourceOffsets`，长度等于 text 的 UTF-8 字节数，每个值是同一源块原始 UTF-8 字节起点。单个规范化字符可覆盖多个原字符，映射可重复。`byteStart`/排他 `byteEnd` 覆盖该片段全部原字符。拆分输入改成可信 `SourceBlock[]`：每项含单一 BlockRef、规范文本、映射、原文排他终点、模型 tokenizer 给出的严格 UTF-8 tokenEnds 与上下文版本。分块不跨源块合并；长块切分与重叠都按给定 tokenEnds。chunkId 摘要包含完整 BlockRef、可选资源、上下文版本、规范字节区间和 chunkHash，不生成假 blockId，不猜模型分词。实际 `ExecuteBuild` 尚未取得此 tokenizer 前置，仍未完成。

词项分词版本固定 `unicode-code-cjk-bigram-v1`：字母、数字、下划线组成保留大小写的代码词；组合标记跟前词；中日韩连续片段为相邻 bigram，单字片段保留单字；标点和空白分界。BM25 固定 k1=1.2、b=0.75，最多 80 项；向量余弦先按最大绝对值缩放再单位化，排除零向量，最多 80 项；并列按 chunkId。RRF 固定 60，缺失名次贡献零，完整同 ID 内容冲突失败关闭，最多 40 项。所有向量 number 明确 format=double，避免跨服务舍入造成 vectorHash 不一致。

`query_embeddings` 唯一键为站点、代次、可信 rootOperationId、规范 queryHash 和 spaceId，并保存完整 space JSON、首次 operationId 和持久结果。进行中调用在本进程按该身份 singleflight；恢复续读仍使用首次 operationId 和同一 MG 幂等键，不因断线重算。`query_embedding_keys` 将管理员或可信调用者的幂等键绑定到完整身份摘要，同键异输入返回 CONFLICT。结果复用前核完整空间、查询、代次和向量摘要。只有成功后通过当前平台写窗口才能保存；费用和余额仍只由 MG 权威记录。
