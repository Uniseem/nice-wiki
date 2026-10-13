# 文档总索引

## 阅读顺序

1. [全局需求](requirements.md)：功能目标、边界和不变量。
2. [总架构](architecture.md)：独立服务、数据权威和部署约束。
3. [服务地图](service-map.md)：十五域与模块入口。
4. [公共协议](protocols.md)、[数据与一致性](consistency.md)、[依赖图](dependencies.md)、[安全与隐私](security.md)：共同规则。
5. [界面装配](frontend.md)、[部署与运维](operations.md)：跨模块交付方式。
6. [文档标准](documentation-standard.md)、[结构契约](structure/README.md)、[函数登记规则](structure/functions.md)：精确到未来文件和函数的要求。
7. [实施计划](plan.md)、[验收规则](acceptance.md)、[决策记录](adr/README.md)：状态和执行门槛。

## 模块总文档

| 编号 | 模块入口 | 独占职责 |
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

每个模块 README.md 继续索引 requirements / architecture / data / api / workflows / files / functions / operations / acceptance 九份子文档和四份机器契约。不能跳过全局约束，只读单个函数就编写代码。


产品行为以 [需求保留矩阵](product-preservation.md) 和 [旧设计基线](product-baseline/README.md) 为准；AI上下文与缓存见 [详细规格](ai-context-and-cache.md)。

多会话开发按 [并行开发规则](playbooks/parallel-development.md)，新会话与跨天继续按 [交接流程](playbooks/handoff.md)。每个模块的 STATUS/progress/handoff 记录当前事实；设计台账初始全部未实现。

各模块独立构建、进程入口、健康与退出的共同工程要求见 [服务启动蓝图](service-bootstrap.md)，具体文件和函数已合入每模块合同。
