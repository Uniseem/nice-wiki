# data-protection 架构

在全域维护安全点创建一致备份、校验并恢复统一代次；把各域已有历史投影成 data git 版本镜像并管理远程同步。

未来Go服务拥有本域SQLite/对象目录，HTTP严格绑定→授权/epoch→领域函数→私有存储；Vue3后台模块通过portal入口使用本域HTTP/JSON/SSE，界面不持有service凭证或secret。

全站备份manifest、校验报告、恢复暂存与切换决策日志及恢复进度

加密备份对象、保留策略、下载授权

data git确定性版本镜像与受配置远程同步

| 依赖域 | 模式 | 用途 | 故障策略 |
|---|---|---|---|
| identity | sync | 仅通过其权威HTTP接口或登记事件使用identity能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| site-config | sync | 仅通过其权威HTTP接口或登记事件使用site-config能力，具体调用见函数dependencies | site-config停机：不能打开/切代/退出maintenance；各服务readiness fail closed，不假设缓存epoch可继续写。 |
| jobs | sync | 仅通过其权威HTTP接口或登记事件使用jobs能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| content | sync | 仅通过其权威HTTP接口或登记事件使用content能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| media | sync | 仅通过其权威HTTP接口或登记事件使用media能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| public-site | sync | 仅通过其权威HTTP接口或登记事件使用public-site能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| model-gateway | sync | 仅通过其权威HTTP接口或登记事件使用model-gateway能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| knowledge | sync | 仅通过其权威HTTP接口或登记事件使用knowledge能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| qa | sync | 仅通过其权威HTTP接口或登记事件使用qa能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| localization | sync | 仅通过其权威HTTP接口或登记事件使用localization能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| memory | sync | 仅通过其权威HTTP接口或登记事件使用memory能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| traffic-guard | sync | 仅通过其权威HTTP接口或登记事件使用traffic-guard能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| analytics | sync | 仅通过其权威HTTP接口或登记事件使用analytics能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |

权威边界：content正文及源历史，site-config运行epoch/maintenance，identity账号权限，model-gateway唯一钱账。派生结果缓存不构成授权真相。所有客户端只接受versioned schema；未知能力返回UNSUPPORTED_CAPABILITY，不能猜测兼容。

故障与完整调用步骤见functions及operations。服务注册和网络地址由部署配置注入，不能借当前机器或vps信息推断生产环境。
