# public-site 架构

将 content 已发布版本渲染成可索引、可访问的完整 HTML，并提供导航、SEO、多语言路由、Markdown、llms、sitemap 与受保护预览。

未来Go服务拥有本域SQLite/对象目录，HTTP严格绑定→授权/epoch→领域函数→私有存储；Vue3后台模块通过portal入口使用本域HTTP/JSON/SSE，界面不持有service凭证或secret。

完整 HTML 输出、无 JavaScript 可阅读的导航及正文

按语言和发布修订派生的渲染缓存及公开导航投影

公开页面、Markdown、机器人索引与预览的权限边界

保留old D12/D14/D17/D19/D20/D26/D29/D32/D34/D54：Go完整三栏、首页单屏、Markdown .md、AI浮窗/独立页、各语言索引、localStorage公告关闭、标题不折叠

翻译未完成显示旧译/原文并在页首当前语言提示；blockId跨语言保持稳定锚点，切换同文档同位置

| 依赖域 | 模式 | 用途 | 故障策略 |
|---|---|---|---|
| content | sync | 仅通过其权威HTTP接口或登记事件使用content能力，具体调用见函数dependencies | content 停机：所有正文/导航/sitemap/llms/Markdown 返回 503，不能把旧 cache 当安全授权；静态 CSS/图标可继续读。 |
| site-config | sync | 仅通过其权威HTTP接口或登记事件使用site-config能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| identity | sync | 仅通过其权威HTTP接口或登记事件使用identity能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| media | sync | 仅通过其权威HTTP接口或登记事件使用media能力，具体调用见函数dependencies | media 不可达：可公开正文使用固定尺寸占位且不输出未知资源 URL；受保护预览资源失败仍不落公开缓存。 |
| analytics | optional | 仅通过其权威HTTP接口或登记事件使用analytics能力，具体调用见函数dependencies | 匿名beacon失败丢弃不影响公开页 |
| jobs | sync | 仅通过其权威HTTP接口或登记事件使用jobs能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |
| data-protection | sync | 仅通过其权威HTTP接口或登记事件使用data-protection能力，具体调用见函数dependencies | 返回DEPENDENCY_UNAVAILABLE；保留已提交本域日志/outbox，不猜测外域成功、不输出过期授权结果 |

权威边界：content正文及源历史，site-config运行epoch/maintenance，identity账号权限，model-gateway唯一钱账。派生结果缓存不构成授权真相。所有客户端只接受versioned schema；未知能力返回UNSUPPORTED_CAPABILITY，不能猜测兼容。

故障与完整调用步骤见functions及operations。服务注册和网络地址由部署配置注入，不能借当前机器或vps信息推断生产环境。
