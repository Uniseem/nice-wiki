# 文档层级与合同格式

模块总文档 README.md 必须索引 docs/requirements.md、architecture.md、data.md、api.md、workflows.md、files.md、functions.md、operations.md、acceptance.md，以及四份机器契约。所有需求有 ID，所有未来文件列完整路径与职责，所有登记函数有唯一 ID，格式为“模块slug.函数名”。

implementation.json 顶层：moduleId、module、version、phase=design、requirements、dependencies、files、jobTypes、backgroundTasks。jobTypes 每项含 type（以模块slug开头）、executor（本域binding函数）、trigger，可选 schedule（cron）；backgroundTasks 每项含 function 与 schedule。执行函数必须是 RunWorker 的依赖，后台函数必须是 BuildApplication 的依赖，除界面函数外所有函数都必须能从接口处理、事件处理或启动入口到达（[ADR0004](adr/0004-wiring-and-service-identity.md)）。requirements 每条含 id、description、acceptance；dependencies 每条含 module、mode(sync/event/optional)、purpose、failurePolicy。files 每条含 path（仓库相对路径）、kind(handwritten/generated/config/migration/ui)、purpose、functions。每函数字段：id、name、signature、inputs、outputs、purpose、steps、errors、sideEffects、authorization、concurrency、idempotency、dependencies、contract，可选 resources。contract 取 binding（约束）或 reference（参考）；OpenAPI x-handler、events.json 消费 handler、outbox 转发与事件订阅循环、任务执行函数和启动入口必须是 binding。dependencies 只能是本域函数 ID、共享库函数 ID（如 platform.RunRelay）或“模块:operationId”，由检查逐条解析；存储、外部库、协议等非函数依赖写入 resources。inputs/outputs 每项含 name、type、description、constraints。空参数用空数组；返回 error 必须列出其含义与错误集合；无副作用写空数组，不省略字段。

signature 使用精确 Go 或 TypeScript 签名；参数和返回类型必须能在公共或本域 types.json 找到（标准库类型例外）。Go 操作函数以 `(ctx context.Context, op OperationContext, …)` 开头；浏览器函数为 `(input, signal: AbortSignal)`，不接收服务端操作上下文。每个非生成业务文件必须分配职责，函数步骤必须描述实际校验、读写、状态迁移、失败与回滚，不用“自行实现”替代。生成文件标明来源和生成器接口，不枚举生成器内部函数。文件统计分别列手写、生成、配置、迁移、UI，不用数量证明正确。

types.json 使用 JSON Schema Draft2020-12，业务类型放 $defs，显式 required/additionalProperties、字段含义、枚举、范围及空值规则。类型标识与函数参数一致；内部依赖容器标 x-internal=true。OpenAPI 3.1 的每个 operation 有唯一 operationId、x-handler（函数 ID）、x-permission（登记权限），请求/响应引用明确的 schema；业务错误引用公共 Problem。公共类型引用 ../../../contracts/common.schema.json#/$defs/类型名。

events.json 顶层 module/version/events；每事件含 name、version、direction(publish/consume)、payloadSchema、purpose、delivery(at-least-once)、ordering、idempotency；consume 条目另含 handler（本域 binding 函数 ID）。载荷不用任意对象代替字段规格；事件默认只传不可变引用与必要元数据，不广播提问正文、密钥或 IP。

functions.md 与 files.md 由 `make docs` 从 implementation.json 生成；api.md 中 `<!-- openapi:start -->` 与 `<!-- openapi:end -->` 之间的接口清单由 openapi.json 生成，workflows.md 中 `<!-- flows:start -->` 与 `<!-- flows:end -->` 之间的入口与后台流程由 jobTypes、backgroundTasks 与 events.json 生成，标记外的说明手写。公开 AI 接口用 `x-admission` 标记动作，只能用于匿名接口。生成部分不手改，`make check` 比对生成结果。acceptance.md 是验收行为规格，不是运行测试授权。本阶段未实现，不把业务事项打勾。

公共接口：POST /v1/authorizations/check（identity）；POST /v1/publications:validate（content，refs 为 PublishedRevisionRef 数组，返回 PublicationValidation 数组）；POST /v1/context:resolve（memory，taskId/language 返回 MemoryContext）；POST /v1/inferences（model-gateway）；POST /v1/admissions（traffic-guard）。其他跨服务操作引用生产方 OpenAPI，不复制第二份类型。

所有有状态域必须设计 /v1/barriers、/v1/snapshots:export、/v1/snapshots:stage、/v1/snapshots:discard 管理协议，并在启动时按 runtime 换入或换回 active 目录（ADR0003）；写入受服务身份、超管委托、屏障与 dataEpoch 校验。公共字段见 common.schema.json。portal 无业务存储时只参与入口维护，不虚构数据快照。

共享组件（contracts/modules.json 的 shared，如 libs/platform、deploy）有 README、implementation.json 与生成的 functions.md/files.md，平台库另有 types.json；它们不对外提供 HTTP，也不登记事件。
