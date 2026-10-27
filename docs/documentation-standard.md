# 文档层级与合同格式

模块总文档 README.md 必须索引 docs/requirements.md、architecture.md、data.md、api.md、workflows.md、files.md、operations.md、acceptance.md，以及四份机器契约。所有需求有 ID，所有登记的文件列完整路径与职责。

implementation.json 顶层：moduleId、module、version、phase、requirements、dependencies、files、consumes（本域调用的其他域接口，生产方到 operationId 列表）、jobTypes、backgroundTasks。jobTypes 每项含 type（以模块slug开头）、executor、trigger、managementPermission、steps，可选 schedule（cron）；backgroundTasks 每项含 function、schedule 与 purpose。不再登记函数。

types.json 使用 JSON Schema Draft2020-12，业务类型放 $defs，显式 required/additionalProperties、字段含义、枚举、范围及空值规则。类型标识与函数参数一致；内部依赖容器标 x-internal=true。OpenAPI 3.1 的每个 operation 有唯一 operationId、x-handler（处理函数名）、x-permission（登记权限），请求/响应引用明确的 schema；业务错误引用公共 Problem。公共类型引用 ../../../contracts/common.schema.json#/$defs/类型名。

events.json 顶层 module/version/events；每事件含 name、version、direction(publish/consume)、payloadSchema、purpose、delivery(at-least-once)、ordering、idempotency；consume 条目另含 handler（本域处理函数名）。载荷不用任意对象代替字段规格；事件默认只传不可变引用与必要元数据，不广播提问正文、密钥或 IP。

files.md 由 `make docs` 从 implementation.json 生成；api.md 中 `<!-- openapi:start -->` 与 `<!-- openapi:end -->` 之间的接口清单由 openapi.json 生成，workflows.md 中 `<!-- flows:start -->` 与 `<!-- flows:end -->` 之间的入口与后台流程由 jobTypes、backgroundTasks 与 events.json 生成，标记外的说明手写。公开 AI 接口用 `x-admission` 标记动作，只能用于匿名接口。生成部分不手改，`make check` 比对生成结果。acceptance.md 是验收行为规格，不是运行测试授权。本阶段未实现，不把业务事项打勾。

公共接口：POST /v1/authorizations/check（identity）；POST /v1/publications:validate（content，refs 为 PublishedRevisionRef 数组，返回 PublicationValidation 数组）；POST /v1/context:resolve（memory，taskId/language 返回 MemoryContext）；POST /v1/inferences（model-gateway）；POST /v1/admissions（traffic-guard）。其他跨服务操作引用生产方 OpenAPI，不复制第二份类型。


共享组件（contracts/modules.json 的 shared，如 libs/platform、deploy）有 README、implementation.json 与生成的 files.md，平台库另有 types.json；它们不对外提供 HTTP，也不登记事件。
