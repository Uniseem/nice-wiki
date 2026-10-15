# 身份权限（M02）

保持 D21/D35 账号产品：唯一超管、账号名口令登录、10项功能授权、初始密码强改与服务器恢复；独立私有身份服务。

当前为R1实施：登录/注册/口令、会话/即时授权与子账号管理已通过VPS现阶段检查。本人后台偏好读取、CAS保存和真实HTTP绑定已通过VPS现阶段检查，后台语言/页面目录与portal消费者使用根统一来源；完整服务、恢复/后台、行为验收与部署尚未完成。以STATUS/progress/handoff的最新记录为准。

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
- [产品来源 accounts.md](../../docs/product-baseline/design/accounts.md)
- [产品来源 security-ops.md](../../docs/product-baseline/design/security-ops.md)

确认的old功能/默认行为保持，新的服务私有存储/CAS/fencing只改变内部架构。D55优先于先前初始化AI条款。


内部Go API继续由本域OpenAPI生成。本域没有独立web/UI；portal已有浏览器生成阶段按其OpenAPI的`x-browser-auth-adapters`与`Browser*`类型合成四个认证操作的安全副本，access/refresh只由cookie承载，不把内部token响应类型直接提供给浏览器。

## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
