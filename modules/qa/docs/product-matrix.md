# 问答、引用与回答缓存产品来源与架构映射

用户要求所有产品功能/细节保持old；这里只参考确认ADR和产品设计思路，独立设计微服务文件、函数、API及数据所有权，不复制旧业务代码。较新ADR优先于草案。

| 来源 | 产品约定 | 处理 | 新合同影响 |
|---|---|---|---|
| D49、ai-qa | 第一问exact优先/追问补全/完整问题门/默认AI语义判断/阈值校准 | 保留 | PlanQuestion、EvaluateCacheEligibility、LookupExactCache、LookupSemanticCache、ReadCacheCalibration。 |
| ai-qa | 普通自动缓存7天/负反馈拒入/精选优先免TTL与memory变化 | 保留 | WriteAutomaticCache、ApproveCacheEntry；引用撤回/hash失效仍执行。 |
| ai-qa、i18n | 基准问题跨语言共享/缺语言快速自动翻译保存 | 保留 | LocalizedAnswer、TranslateCacheEntry、StoreLocalizedCacheAnswer，不加人工审核。 |
| D50 | 500字/500 800 1500tokens/4子问/3轮5tools/1活跃排队/10%+15000增量压缩 | 保留 | QaPolicy、InspectEvidence、CreateCompaction/RunCompaction，较小窗口明确预算裁剪。 |
| ai-qa | 当前页面/独立页浮窗共用对话/原问题和完整问题记录 | 保留 | StartTurn currentPage验证及同conversationId；上下文层单独边界。 |

引用：[全局产品基线](../../../docs/product-preservation.md)、[来源快照](../../../docs/product-baseline/README.md)、[AI上下文与缓存](../../../docs/ai-context-and-cache.md)。

数值缺省：没有旧确认数字的新增技术控制使用本文明确defaults；它们不新增用户确认或改变产品行为。执行timeout/lease/restore限时是架构执行边界，AI备用/翻译降级仍通过独立attempt费用预留。
