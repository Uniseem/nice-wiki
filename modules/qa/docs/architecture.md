# 问答、引用与回答缓存架构

负责访客对话和逐事件流式问答，管理引用、反馈、精选答案、严格版本化缓存、历史压缩及隔离诊断。

Go独立服务，Vue3后台页面由portal装配；私有DB、私有对象目录、独立control.db。服务间HTTP/JSON、SSE、持久事件；禁止跨域读表或读文件。

traffic-guard：访客准入频率/挑战/并发。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

knowledge：混合检索与权威材料分段。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

content：引用当前可见性校验。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

model-gateway：答案/查询embedding/历史压缩收费调用。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

memory：任务注入上下文。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

localization：经批准缓存答案翻译。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

jobs：压缩任务与持久事件。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

site-config：公开问答配置及dataEpoch。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

可信执行上下文使用公共OperationContext；浏览器仅提交业务schema，入口认证后构造actor/rootOperationId/costClass/dataEpoch/cancellationId。普通业务使用active epoch，恢复七项控制操作使用预授权恢复许可，control.db不被业务导入覆盖。

费用归MG；本域不推算余额，网络超时不表示未收费。
