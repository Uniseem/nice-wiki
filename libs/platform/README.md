# 平台公共库（L01）

各服务共用的基础设施实现，Go module `nice-wiki/libs/platform`。决策见 [ADR0005](../../docs/adr/0005-engineering-and-deployment.md)。

只放与业务无关的机制：服务断言与用户委托、可信操作上下文、公共错误输出、outbox 登记与转发、inbox 去重与事件订阅、任务领取循环、模型调用助手、配置加载、运行状态监视、恢复换目录、进程模板，以及生成服务密钥的 `nw-keys` 工具。

规则：

- 不含任何业务规则，不访问任何服务的私有数据，不导入服务代码；服务只经本库公开函数使用这些机制。
- 各服务的 RelayOutbox、ConsumeEvents、RunWorker、InvokeModel、main、BuildApplication、Serve 等约束函数只装配本域参数并调用本库。
- 改动本库视为公共契约变更，由集成会话统一修改并同步所有服务。

资料：[函数合同](docs/functions.md)、[文件目录](docs/files.md)、[机器合同](contracts/implementation.json)、[类型](contracts/types.json)、[进度台账](progress.json)。

ServiceConfig的可选`masterKeyRef`（环境覆盖`NW_MASTER_KEY_REF`）只传递部署主密钥绝对路径；配置加载不打开密钥，使用域按具体加密操作读取。
