# 平台公共库（L01）

各服务共用的基础设施实现，Go module `nice-wiki/libs/platform`。决策见 [ADR0005](../../docs/adr/0005-engineering-and-deployment.md)。

只放与业务无关的机制：服务断言与用户委托、可信操作上下文、公共错误输出、outbox 登记与转发、inbox 去重与事件订阅、任务领取循环、模型调用助手、配置加载、运行状态监视、进程模板，以及生成服务密钥的 `nw-keys` 工具。

规则：

- 不含任何业务规则，不访问任何服务的私有数据，不导入服务代码；服务只经本库公开函数使用这些机制。
- 各服务的 RelayOutbox、ConsumeEvents、RunWorker、InvokeModel、main、BuildApplication、Serve 等约束函数只装配本域参数并调用本库。
- 改动本库视为公共契约变更，改完跑 `make build` 确认所有服务仍能编译。

资料：[文件目录](docs/files.md)、[机器合同](contracts/implementation.json)、[类型](contracts/types.json)。

ServiceConfig的可选`masterKeyRef`（环境覆盖`NW_MASTER_KEY_REF`）只传递部署主密钥绝对路径；配置加载不打开密钥，使用域按具体加密操作读取。

## 原生工具 agent

问答主链路现在使用 `zcode` 包连接独立 M16 服务中的 ZCode `AgentRuntime`。平台只承载 Node 进程、已签名 API 的请求帧及取消；模型决策、工具调度和接近默认模型窗口时的压缩直接使用原生框架。问答域仍负责模型网关身份、公开文档权限、引用复核及派生 checkpoint 保存。构建执行 `pnpm build:agent`，不运行本地模型。

`modelagent.Run` 保留为旧循环的兼容实现，问答主链路不再调用；`modelagent.ToolResult` 仍用于既有工具回调。平台不访问业务数据，也不决定文档可见性。

`GatewayMessage` 支持 assistant `toolCalls` 和 tool `toolCallId`；完整 `ModelOutput` 可携带 `toolCalls` 与 `finishReason`。`ValidateToolConversation` 校验成对历史，`ValidateToolCalls` 拒绝残缺 JSON、非对象参数和重复身份。角色与工具过程须由服务端持久记录重放，不解释访客 JSON 为角色。

## 原模型调用只读观察

标准`modelclient.NewGatewayClient`的`GatewayResponseValidators`必须有`CreateInvocationResult`、`CancelInvocationResult`、`StreamEvent`、`GetInvocationResult`、`LookupInvocationResult`五个真实生产方schema校验器。旧`RawGatewayClient`三方法接口保持；可选`RawLookupGatewayClient.LookupInference(ctx,body)`和`RawCurrentGatewayClient.ReadInference(ctx,id)`提供只读端口。公共可选`InvocationLookupReader.LookupInference(ctx,key,input)`与`CurrentInvocationReader.GetInference(ctx,id)`返回`GatewayInvocationSnapshot{Invocation,Output?}`。

`InvokeModel`先以原完整请求和原幂等键查找。只有可信tracking的专用INVOCATION_NOT_ACCEPTED(404)否证明，且原付费截止仍有效才Create相同原键/原输入；缺tracking、孤立记录或读取失败不当作不存在，不更换键。原付费deadline只约束原Create/执行，过期后仍可用独立5秒的只读RPC截止观察；原owner、actor、root、site、epoch、qa/background费用类别及完整body保持，不重签/续消费permit。取消同原ID也用独立短控制截止，不能把过期付费op直接送去导致取消被拒。

`InvokeModelOutcome.CurrentSnapshot`只保留真实已观察的当前权威，与`Events`独立；错误或取消仍保留已接受ID、最后有效快照和已收事件，调用方不得丢弃这些事实。完整winning output可直接交付；`ModelOutput.Text`指针区分真实空文本与缺失，文本不超过64KiB UTF8，chat/vision/translation与embedding/rerank字段互斥。未知费用不抹去完整结果，也不伪造零账单。

首次历史读取从排他序号0开始，最多4096条，缺号或超限失败而不静默拼接。历史终态、断流及EOF后都读当前权威；过时的StreamError/charge_unknown仍保留在Events，但不作为当前终态回调，不掩盖后来完整结果。不会合成SSE事件。旧完整文本若只有真实持久事件，仍按原序号读完并保留；缺完整证据失败关闭，不将旧付费结果当404重派。当前实际Get/Lookup故障均不能触发新模型费用。

owner恢复查询使用`modelclient.ObserveInvocation(ctx,client,op,knownInvocationId)`或`LookupModelInvocation(ctx,client,op,originalInvokeRequest)`，只读原ID或完整原body/key。二者即使原付费截止仍有效也没有Create、Cancel或模型分支，不调用可创建的InvokeModel，不回调SSE；即使原截止已过也可按原scope获得fresh5秒读取。业务owner必须锁外执行网络观察，再以原持久输入/hash/阶段证明决定本地恢复，不从工具返回推导新的收费许可。

原键查找的普通NOT_FOUND可能为旧部署或路由缺失，绝不作为原调用未接受证明。只有lookup在真实marker和完整原键/body/root/site/epoch核对后产生的`INVOCATION_NOT_ACCEPTED`精确码才允许Invoke在原付费截止有效时Create原键。只读工具保留该精确码，已知ID的Get404仍Integrity。

## 实际在途scope接线

可信装配的`WorkAdmission`返回独立ctx与release。`ServiceBoundaryConfig.AdmitBusiness`包围非`ControllerOperations`的完整handler和响应流；控制分类只免业务计数，仍经过原服务身份、schema、当前权限和epoch。`WorkerConfig.AdmitBusiness`、`ConsumerConfig.AdmitBatch`/`AdmitDelivery`、`RelayConfig.AdmitBatch`和`RecoveryLoopConfig.AdmitBatch`分别覆盖任务领取、订阅投递、outbox转发和恢复批次；缺hook时用`lifecycle.SimpleAdmission`，只核数据代次一致。`TaskGroup.SetAdmission`只可启动期一次，每次新Start重新admit独立任务lease。

## 任务秘密

`lifecycle.SealTaskSecret`/`OpenTaskSecret`为异步任务提供固定AES256-GCM有界秘密密文：原始明文≤64KiB、固定私有原始32B key文件，版本1与原owner/site/epoch/root/purpose完整AAD绑定。返回nonce/ciphertext，持久策略仍由本域负责；缺key准确503，不延长permit有效期，不把解密当付费授权。
