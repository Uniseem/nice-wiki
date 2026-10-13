# 统一模型网关产品来源与架构映射

用户要求所有产品功能/细节保持old；这里只参考确认ADR和产品设计思路，独立设计微服务文件、函数、API及数据所有权，不复制旧业务代码。较新ADR优先于草案。

| 来源 | 产品约定 | 处理 | 新合同影响 |
|---|---|---|---|
| D44、ai-models | 五档/有序备用/任务覆盖/六协议/模型目录/手动优先/连接探测 | 保留 | 本域TaskAssignment/RefreshModelCatalog/CreateConnectionProbe/Map协议；不同attempt独立预留和未知价格warning。 |
| D53 | 供应方密钥档位任务只超管 | 保留 | 管理API x-permission=superadmin。 |
| D55 | 无AI初始化可正常文档操作 | 保留 | LoadRuntime只声明配置缺项，不探测模型。 |
| ai-guard、D50 | session/IP小时与QA daily，0不限且background独立 | 架构改造 | guard只生成签名伪名scope；金额额度、预留和账本仅MG。 |

引用：[全局产品基线](../../../docs/product-preservation.md)、[来源快照](../../../docs/product-baseline/README.md)、[AI上下文与缓存](../../../docs/ai-context-and-cache.md)。

数值缺省：没有旧确认数字的新增技术控制使用本文明确defaults；它们不新增用户确认或改变产品行为。执行timeout/lease/restore限时是架构执行边界，AI备用/翻译降级仍通过独立attempt费用预留。
