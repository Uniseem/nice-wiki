# 长期记忆、任务映射与记忆助手架构

维护人工可审计记忆条目、任务映射、语言范围、版本化注入预算和只产生提议的记忆助手；批准后才应用记忆或术语变更。

Go独立服务，Vue3后台页面由portal装配；私有DB、私有对象目录、独立control.db。服务间HTTP/JSON、SSE、持久事件；禁止跨域读表或读文件。

identity：人工权限与批准委托。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

model-gateway：记忆助手收费执行。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

localization：异步批准术语命令/终态事件，禁止同步反向依赖。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

jobs：助手调度取消及持久事件。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

site-config：运行dataEpoch。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

可信执行上下文使用公共OperationContext；浏览器仅提交业务schema，入口认证后构造actor/rootOperationId/costClass/dataEpoch/cancellationId。普通业务使用active epoch，恢复七项控制操作使用预授权恢复许可，control.db不被业务导入覆盖。

费用归MG；本域不推算余额，网络超时不表示未收费。
