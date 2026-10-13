# jobs 架构

提供持久任务、带版本事件、租约、重试、进度与定时触发，协调安全点屏障；领域算法由所属服务通过 HTTP 执行。

未来Go服务拥有本域SQLite/对象目录，HTTP严格绑定→授权/epoch→领域函数→私有存储；Vue3后台模块通过portal入口使用本域HTTP/JSON/SSE，界面不持有service凭证或secret。

任务状态机、owner白名单、去重键、租约fencing和进度SSE

带版本事件至少一次投递、inbox/outbox、消费者游标

定时触发及维护屏障调度暂停

| 依赖域 | 模式 | 用途 | 故障策略 |
|---|---|---|---|
| site-config | sync | 仅通过其权威HTTP接口或登记事件使用site-config能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| identity | sync | 仅通过其权威HTTP接口或登记事件使用identity能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| data-protection | sync | 仅通过其权威HTTP接口或登记事件使用data-protection能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| content | sync | 仅通过其权威HTTP接口或登记事件使用content能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| media | sync | 仅通过其权威HTTP接口或登记事件使用media能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| model-gateway | sync | 仅通过其权威HTTP接口或登记事件使用model-gateway能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| knowledge | sync | 仅通过其权威HTTP接口或登记事件使用knowledge能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| qa | sync | 仅通过其权威HTTP接口或登记事件使用qa能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| localization | sync | 仅通过其权威HTTP接口或登记事件使用localization能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| memory | sync | 仅通过其权威HTTP接口或登记事件使用memory能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| traffic-guard | sync | 仅通过其权威HTTP接口或登记事件使用traffic-guard能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| analytics | sync | 仅通过其权威HTTP接口或登记事件使用analytics能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| public-site | sync | 仅通过其权威HTTP接口或登记事件使用public-site能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |

权威边界：content正文及源历史，site-config运行epoch/maintenance，identity账号权限，model-gateway唯一钱账。派生结果缓存不构成授权真相。所有客户端只接受versioned schema；未知能力返回UNSUPPORTED_CAPABILITY，不能猜测兼容。

故障与完整调用步骤见functions及operations。服务注册和网络地址由部署配置注入，不能借当前机器或vps信息推断生产环境。
