# 资源媒体（M05）

保持 D24/D26 与资源库产品：内容寻址上传/Range、浏览器WebP/视频封面、四平台公开信息快照、本地封面、Excalidraw三文件，文件不自动删除。

当前为R1实施：私有SQLite、blob恢复与部分上传前置已交付；第五轮补顺序分块/原回执恢复和ready资源安全元数据读取。当前源码待统一VPS，完整应用因其余handler/控制恢复未齐保持blocked；业务验收和部署未执行。

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
- [产品来源 security-ops.md](../../docs/product-baseline/design/security-ops.md)

确认的old功能/默认行为保持，新的服务私有存储/CAS/fencing只改变内部架构。D55优先于先前初始化AI条款。


## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
