# 知识入库与混合检索架构

将content权威发布修订与media授权资源转换为版本化分块、词项和向量索引；只返回经权威可见性校验的材料。

Go独立服务，Vue3后台页面由portal装配；私有DB、私有对象目录、独立control.db。服务间HTTP/JSON、SSE、持久事件；禁止跨域读表或读文件。

content：读取权威发布正文及批量可见性。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

media：授权资源读取和媒体元数据。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

model-gateway：嵌入与可选重排调用。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

memory：检索任务上下文注入。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

jobs：发布触发任务、取消与持久事件。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

site-config：启用配置和dataEpoch。失败策略：必需读取失败关闭；写命令先本地事务+outbox，最多3次transport重试500/1000/2000ms，仍失败保留待处理；模型自动备用用新attempt新预留，旧未知费用保留held。

可信执行上下文使用公共OperationContext；浏览器仅提交业务schema，入口认证后构造actor/rootOperationId/costClass/dataEpoch/cancellationId。普通业务使用active epoch

费用归MG；本域不推算余额，网络超时不表示未收费。

## 分层阅读指南

`readDocumentIndex` 同时维护只读导航视图：全站 `/AGENTS.md` 列顶层文章；`/<documentId>/AGENTS.md` 描述该文档及其直接子文档。调用方按 `guideOffset` 续读，返回 `nextGuideOffset` 时本层尚未读完；分区外和已撤回文档不进入视图。

指南用 content 的当前发布树与本域已保存的文档卡片确定性生成。卡片 `source_hash` 与当前发布正文哈希不一致时只给标题和阅读入口，不沿用旧摘要。版本绑定发布引用、目录关系、路径与有效卡片；发布、移动、撤回和摘要更新会反映到后续读取。生成指南没有新模型调用或共享目录写入。

agent 请求使用 `guideOnly=true`，只返回当前层导航，不附带全站旧式 Markdown 清单或路由选项。阅读指南只作定位资料，正文事实与引用由问答通过版本化内容 API 核验。
