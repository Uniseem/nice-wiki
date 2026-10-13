# 语言、翻译与术语产品来源与架构映射

用户要求所有产品功能/细节保持old；这里只参考确认ADR和产品设计思路，独立设计微服务文件、函数、API及数据所有权，不复制旧业务代码。较新ADR优先于草案。

| 来源 | 产品约定 | 处理 | 新合同影响 |
|---|---|---|---|
| D47、i18n | 1000token顺序小批/整文前缀/前批相邻译文/20字TM/对半逐块降级 | 保留 | BuildTranslationPrompt、ReuseTranslationMemory、PlanTranslationFallback。 |
| D47 | 首次自动术语直接入库manual优先可整批撤销/符合度重译一次/反思默认off | 保留 | ExtractAutomaticGlossary、RollbackAutomaticGlossaryBatch、CheckTerminologyCompliance、ReflectTranslation。 |
| i18n | 人工源变review不覆盖/未完旧译文新块原文顶栏/稳定block树/标题目录站点UI | 保留 | ReadTranslation、TranslationUnit status及权威文件manifest，人工generation CAS。 |
| i18n | 语言建议浏览器时区离线国家/快速模型缓存规则fallback/拒绝不再提示 | 保留 | SuggestLanguage 5秒限制、无IP持久化；根路径选择属public-site。 |

引用：[全局产品基线](../../../docs/product-preservation.md)、[来源快照](../../../docs/product-baseline/README.md)、[AI上下文与缓存](../../../docs/ai-context-and-cache.md)。

数值缺省：没有旧确认数字的新增技术控制使用本文明确defaults；它们不新增用户确认或改变产品行为。执行timeout/lease/restore限时是架构执行边界，AI备用/翻译降级仍通过独立attempt费用预留。
