# 站点配置与初始化（M03）

保持 D17/D34/D55 的全屏首页、公告横幅、站点品牌页脚与无AI初始化；dataEpoch/maintenance唯一控制权威。

当前为 R1-02 实施中：运行权威与私有持久化已落地，初始化与 settings 权威提交/恢复已通过首轮检查；本轮推进一般设置权威读写、纯校验与原始对象流，已交付核心通过统一 VPS 门槛，完整 HTTP/后台及行为验收尚未完成；用户要求第二轮后停止自动实施。

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
- [产品来源 home.md](../../docs/product-baseline/design/home.md)
- [产品来源 site.md](../../docs/product-baseline/design/site.md)
- [产品来源 admin.md](../../docs/product-baseline/design/admin.md)
- [产品来源 i18n.md](../../docs/product-baseline/design/i18n.md)
- [产品来源 security-ops.md](../../docs/product-baseline/design/security-ops.md)

确认的old功能/默认行为保持，新的服务私有存储/CAS/fencing只改变内部架构。D55优先于先前初始化AI条款。


## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
