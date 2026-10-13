# 长期记忆、任务映射与记忆助手产品来源与架构映射

用户要求所有产品功能/细节保持old；这里只参考确认ADR和产品设计思路，独立设计微服务文件、函数、API及数据所有权，不复制旧业务代码。较新ADR优先于草案。

| 来源 | 产品约定 | 处理 | 新合同影响 |
|---|---|---|---|
| D46、D48 | 权威Markdown/description/1536B单条/5120B任务/readonly/超预算拒保存 | 保留 | MemoryEntry fileRef与ValidateProjectedBudgets，DB仅索引。 |
| memory、D46 | 统一任务取用/语言映射/预设条目/人工变项目概述停止自动/每日更新 | 保留 | 根AI task catalog与UpdateProjectOverview，人工active版本保护。 |
| memory | 助手逐条new/update/delete/noop差异批准/术语独立/助手对话保留 | 保留 | AssistantMessage与Proposal，术语批准命令异步localization而非同步回调环。 |
| memory | 新记忆即时生效/不自动重译/普通cache失效精选豁免 | 保留 | context versionHash事件；localization提示重译报价由人工决定。 |

引用：[全局产品基线](../../../docs/product-preservation.md)、[来源快照](../../../docs/product-baseline/README.md)、[AI上下文与缓存](../../../docs/ai-context-and-cache.md)。

数值缺省：没有旧确认数字的新增技术控制使用本文明确defaults；它们不新增用户确认或改变产品行为。执行timeout/lease/restore限时是架构执行边界，AI备用/翻译降级仍通过独立attempt费用预留。
