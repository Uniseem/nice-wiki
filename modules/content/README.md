# 文档生命周期（M04）

保持 D4/D9/D15/D16/D20/D35/D54/D55：Tiptap3块、共享自动保存草稿、内存编辑锁、发布撤回回收站和署名历史；私有正文文件权威。

当前为R1实施：第一轮真实私有正文存储已通过VPS检查；第二轮真实单leader账户共享租约、持久文档/delete-fence读取与SaveFenceGuard前置已通过根统一VPS生成、readonly编译/vet/gofmt与结构检查。完整锁用例即时授权、identity姓名、通知/审计/原回执，完整保存事务与HTTP/后台/恢复激活仍未完成，不把参考核心视作可运行服务。

## 设计索引

- [docs/requirements.md](docs/requirements.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/data.md](docs/data.md)
- [docs/api.md](docs/api.md)
- [docs/workflows.md](docs/workflows.md)
- [docs/files.md](docs/files.md)
- [docs/functions.md](docs/functions.md)
- [docs/operations.md](docs/operations.md)
- [docs/acceptance.md](docs/acceptance.md)

## 机器契约

- [类型与 schema](contracts/types.json)
- [文件与函数实现合同](contracts/implementation.json)
- [OpenAPI 3.1](contracts/openapi.json)
- [事件合同](contracts/events.json)
- [模块工作规则](AGENTS.md)

文件与函数清单及数量见 [docs/files.md](docs/files.md)（由合同生成）。数量只反映登记覆盖，不能视为验收通过。

## 产品来源

- [全局功能保持矩阵](../../docs/product-preservation.md)
- [冻结产品基线](../../docs/product-baseline/README.md)
- [产品来源 editor.md](../../docs/product-baseline/design/editor.md)
- [产品来源 admin.md](../../docs/product-baseline/design/admin.md)
- [产品来源 accounts.md](../../docs/product-baseline/design/accounts.md)
- [产品来源 site.md](../../docs/product-baseline/design/site.md)
- [产品来源 i18n.md](../../docs/product-baseline/design/i18n.md)

确认的old功能/默认行为保持，新的服务私有存储/CAS/fencing只改变内部架构。D55优先于先前初始化AI条款。


## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
