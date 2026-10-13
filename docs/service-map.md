# 服务地图

| 编号 | 模块入口 | 职责 |
|---|---|---|
| M01 | [主系统](../modules/portal/README.md) | 入口、后台外壳、模块装配 |
| M02 | [身份权限](../modules/identity/README.md) | 账号、会话、权限与撤销 |
| M03 | [站点配置与初始化](../modules/site-config/README.md) | 站点、首页、公告与初始化 |
| M04 | [文档生命周期](../modules/content/README.md) | 正文、块、草稿、发布、撤回和历史 |
| M05 | [资源媒体](../modules/media/README.md) | 上传、下载、资源引用与平台视频 |
| M06 | [公开交付](../modules/public-site/README.md) | HTML、SEO、语言路由与受保护预览 |
| M07 | [AI 接入](../modules/model-gateway/README.md) | 供应方适配、调用、费用账本与费用额度 |
| M08 | [索引检索](../modules/knowledge/README.md) | 入库、描述、全文/向量检索与重排 |
| M09 | [问答与缓存](../modules/qa/README.md) | 对话、流式回答、引用、反馈、精选和缓存 |
| M10 | [本地化](../modules/localization/README.md) | 语言、翻译、人工保护、术语与建议 |
| M11 | [长期记忆](../modules/memory/README.md) | 条目、映射、注入、助手与批准提议 |
| M12 | [流量风控](../modules/traffic-guard/README.md) | 机器人刷流、频率、并发、验证与封禁 |
| M13 | [任务与事件](../modules/jobs/README.md) | 队列、事件、租约、重试、进度与定时 |
| M14 | [统计与审计](../modules/analytics/README.md) | 匿名访问、操作审计与用量汇总 |
| M15 | [数据保护](../modules/data-protection/README.md) | 一致备份、恢复、版本镜像与 data git 同步 |

模块间调用的唯一来源是各域 OpenAPI；函数依赖用 module:operationId 或本域函数 ID，不用文件路径猜测 API。
