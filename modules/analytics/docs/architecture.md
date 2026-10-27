# analytics 架构

聚合匿名公开访问、管理操作审计和 model-gateway 费用引用，提供后台统计及导出；实时预算、账本余额仍由 model-gateway 决定。

未来Go服务拥有本域SQLite/对象目录，HTTP严格绑定→授权/epoch→领域函数→私有存储；Vue3后台模块通过portal入口使用本域HTTP/JSON/SSE，界面不持有service凭证或secret。

允许字段匿名beacon、小时/日聚合与漏数计数

管理审计引用、安全结构化元数据和权限查询

按gateway唯一ledgerEntryId派生费用聚合及对账

| 依赖域 | 模式 | 用途 | 故障策略 |
|---|---|---|---|
| identity | sync | 仅通过其权威HTTP接口或登记事件使用identity能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| site-config | sync | 仅通过其权威HTTP接口或登记事件使用site-config能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| model-gateway | sync | 仅通过其权威HTTP接口或登记事件使用model-gateway能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| jobs | sync | 仅通过其权威HTTP接口或登记事件使用jobs能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |

派生结果缓存不构成授权真相。所有客户端只接受versioned schema；未知能力返回UNSUPPORTED_CAPABILITY，不能猜测兼容。

故障与完整调用步骤见functions及operations。服务注册和网络地址由部署配置注入，不能借当前机器或vps信息推断生产环境。

统计UI库：固定已批准Vue/Vite/TS版本，mountModuleUI只消费portal同源apiClient；同DOM/build/capabilities/端口一致复用，统一nice-wiki:unmount停止控制器/监听/下载并清实例。访问/费用并行但费用不可用明确标记，显示真实双水位/漏数/最后完整对账；不说实时余额、分桶访次非跨桶独立人。审计按action/UTC日期、固定上界签名游标与真实链校验；日期过滤进入服务SQL及游标绑定，权限失效清列表详情。导出固定权限路由、稳定operationId重试、32MiB有界流读取后交浏览器，不把未知kind默认pages，不声明用户最终保存成功。
