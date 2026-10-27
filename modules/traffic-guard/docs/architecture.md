# traffic-guard 架构

在请求进入高成本或高并发路径前执行频率、并发、挑战及封禁策略，只发准入凭证；model-gateway 仍是唯一实时额度和钱账权威。

未来Go服务拥有本域SQLite/对象目录，HTTP严格绑定→授权/epoch→领域函数→私有存储；Vue3后台模块通过portal入口使用本域HTTP/JSON/SSE，界面不持有service凭证或secret。

按站点/动作/主体 token bucket 和持久并发租约

基于匿名信号的挑战、临时禁令及后台策略

签名短期 AdmissionPermit 与事件审计引用

| 依赖域 | 模式 | 用途 | 故障策略 |
|---|---|---|---|
| identity | sync | 仅通过其权威HTTP接口或登记事件使用identity能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| site-config | sync | 仅通过其权威HTTP接口或登记事件使用site-config能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| jobs | sync | 仅通过其权威HTTP接口或登记事件使用jobs能力，具体调用见函数dependencies | jobs不可达：outbox持久重试；不把事件派发失败当原事务失败或重扣令牌。 |

派生结果缓存不构成授权真相。所有客户端只接受versioned schema；未知能力返回UNSUPPORTED_CAPABILITY，不能猜测兼容。

故障与完整调用步骤见functions及operations。服务注册和网络地址由部署配置注入，不能借当前机器或vps信息推断生产环境。
