# 统一模型网关架构

唯一拥有供应方、密钥、模型版本、功能档位、费用报价、额度预留、执行与费用账本。所有AI消费必须经过此服务。

Go独立服务，Vue3后台页面由portal装配；私有DB、私有对象目录、独立control.db。服务间HTTP/JSON、SSE、持久事件；禁止跨域读表或读文件。

identity：验证管理权限及服务身份。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

jobs：持久事件发布与未知账单对账任务。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

analytics：消费审计事件；不回调账本。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

site-config：读取启用的功能配置；无同步写回。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

media：视觉输入资源仅经授权asset读取；不得任意URL抓取。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

可信执行上下文使用公共OperationContext；浏览器仅提交业务schema，入口认证后构造actor/rootOperationId/costClass/dataEpoch/cancellationId。普通业务使用active epoch；备份恢复控制操作只接受data-protection服务身份与superadmin委托，control区不随恢复替换。

费用归MG；本域不推算余额，网络超时不表示未收费。

## 原生工具调用

chat/vision 请求可声明 `tools` 和 `toolChoice`，消息允许 assistant 工具调用及成对 tool 结果。网关校验工具声明、调用 ID、参数对象完整性和结果配对；工具由所属业务域执行。

OpenAI 兼容协议使用 `tool_calls`/`tool_call_id`；Anthropic 使用 `tool_use`/`tool_result`，同一批结果合并到紧邻的 user 消息；Gemini 使用 `functionCall`/`functionResponse`，函数签名随历史原样保留。流式参数按调用索引拼接，截断或缺少正常工具结束证明时不交付可执行调用。

完整调用保存在 `ModelOutput.toolCalls`，普通文本仍是独立的完整 `text`，工具参数不混入文本 delta。输入窗口估算包含工具声明和历史参数；每次 agent 模型调用沿用原任务归属、准入、费用根、幂等键及取消机制。

协议参考：[OpenAI 函数调用](https://developers.openai.com/api/docs/guides/function-calling)、[Anthropic 流式消息](https://platform.claude.com/docs/en/build-with-claude/streaming)、[Gemini 函数调用](https://ai.google.dev/gemini-api/docs/function-calling)。
