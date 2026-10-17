# M07 统一模型网关

唯一拥有供应方、密钥、模型版本、功能档位、费用报价、额度预留、执行与费用账本。所有AI消费必须经过此服务。

阶段：R1实施。2026-10-17第二轮真实人工价目、免费报价与localization文本预算已通过GitLab CI流水线23生成/编译/结构检查并交付，源码与逐项范围见progress；本轮2026-10-17沿既有供应方/模型保存接口接入enabled启停及CAS、身份不可变和原回执，源码及精确格式/生成物已通过根GitLab CI流水线12的生成/编译/结构检查并交付；新的embedding配置因M07-R06归一化身份合同未收口而失败关闭，PutModel整体partial；其余历史状态见本域STATUS。第二轮真实档位读取/费用策略及安全目录分支已通过根VPS检查；ReadModels空间与ReadAdminCatalog最近探测仍partial，不虚报完整。用户最新持续目标取代此前两轮停止指令。本轮免费路由解析候选已写：ResolveRoute通过SelectCandidates在一个SQL快照内调用ResolveProfile，核真实任务赋值、可信费用类别、覆盖/回退、同代安全候选与嵌入空间。源码已交付并通过根VPS生成/编译/结构与交叉审核，相关完整函数verified；实际SHA与范围见progress。完整HTTP、权威epoch初始化、恢复、派发引用生命周期与模型费用仍未齐，BuildApplication保持blocked；未做业务验收或部署。

- [requirements](docs/requirements.md)
- [architecture](docs/architecture.md)
- [data](docs/data.md)
- [api](docs/api.md)
- [workflows](docs/workflows.md)
- [files](docs/files.md)
- [functions](docs/functions.md)
- [operations](docs/operations.md)
- [product-matrix](docs/product-matrix.md)
- [acceptance](docs/acceptance.md)

- [types.json](contracts/types.json)
- [implementation.json](contracts/implementation.json)
- [openapi.json](contracts/openapi.json)
- [events.json](contracts/events.json)

依赖：[根文档标准](../../docs/documentation-standard.md)、[公共类型](../../contracts/common.schema.json)、[权限](../../contracts/permissions.json)、[唯一AI任务目录](../../contracts/ai-tasks.json)、[产品保持原则](../../docs/product-preservation.md)、[AI上下文与缓存](../../docs/ai-context-and-cache.md)。

## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
