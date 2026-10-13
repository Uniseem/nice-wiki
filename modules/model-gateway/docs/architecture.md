# 统一模型网关架构

唯一拥有供应方、密钥、模型版本、功能档位、费用报价、额度预留、执行与费用账本。所有AI消费必须经过此服务。

Go独立服务，Vue3后台页面由portal装配；私有DB、私有对象目录、独立control.db。服务间HTTP/JSON、SSE、持久事件；禁止跨域读表或读文件。

identity：验证管理权限及服务身份。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

jobs：持久事件发布与未知账单对账任务。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

analytics：消费审计事件；不回调账本。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

site-config：读取启用的功能配置；无同步写回。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

media：视觉输入资源仅经授权asset读取；不得任意URL抓取。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

可信执行上下文使用公共OperationContext；浏览器仅提交业务schema，入口认证后构造actor/rootOperationId/costClass/dataEpoch/cancellationId。普通业务使用active epoch，恢复七项控制操作使用预授权恢复许可，control.db不被业务导入覆盖。

费用归MG；本域不推算余额，网络超时不表示未收费。
