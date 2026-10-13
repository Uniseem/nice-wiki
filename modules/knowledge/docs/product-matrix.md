# 知识入库与混合检索产品来源与架构映射

用户要求所有产品功能/细节保持old；这里只参考确认ADR和产品设计思路，独立设计微服务文件、函数、API及数据所有权，不复制旧业务代码。较新ADR优先于草案。

| 来源 | 产品约定 | 处理 | 新合同影响 |
|---|---|---|---|
| ai-qa | 块/小节检索单元、背景前缀、图描述hash复用、混合召回RRF和rerank fallback | 保留 | EnrichChunkContext、SplitChunks、ExtractImageDescription、RerankCandidates。 |
| i18n | 只基准语言建索引 | 保留 | searchKnowledge language为基准；QA前置翻译canonical问题。 |
| ai-qa草案保存触发 vs D47/D15较新发布流程 | 草稿保存不触发付费 | 采用较新ADR | HandlePublication只有published引用；active CAS旧revision不可覆盖。 |
| ai-qa嵌入模型分区 | 完整空间隔离 | 架构改造 | 公共EmbeddingSpace provider/modelVersion/dimensions/preprocessingVersion，查询向量同query复用。 |

引用：[全局产品基线](../../../docs/product-preservation.md)、[来源快照](../../../docs/product-baseline/README.md)、[AI上下文与缓存](../../../docs/ai-context-and-cache.md)。

数值缺省：没有旧确认数字的新增技术控制使用本文明确defaults；它们不新增用户确认或改变产品行为。执行timeout/lease/restore限时是架构执行边界，AI备用/翻译降级仍通过独立attempt费用预留。
