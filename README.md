# nice-wiki · 多用户文档平台

本 main 是多用户文档平台（GitBook 式）：用户自己注册、建站、邀请协作者、绑定域名，站点内容有编辑器、多语言、AI 问答与统计。设计见 [docs/architecture-v2.md](docs/architecture-v2.md)（以它为准，第 18 节记录与设计的差异），最新进度与本地启动方式见 [STATUS.md](STATUS.md)。旧实现和旧文档保存在old归档提交272bddc 与 archive-16-services 分支。

从 [文档总索引](docs/README.md) 开始；先读 [全局需求](docs/requirements.md)、[总架构](docs/architecture.md)、[服务地图](docs/service-map.md)、[公共协议](docs/protocols.md)，再进入模块总文档。每个模块索引其架构、数据、API、流程、文件、函数、运维和验收规格。

设计的机器来源是 contracts/modules.json、contracts/common.schema.json 与各模块 contracts 下的 OpenAPI、类型、事件、实现清单。函数分约束与参考两级：对外处理、事件、任务和启动入口依照准确签名、字段、错误、步骤与状态规则实现；内部辅助拆分是参考，实现时可调整并在交付时同步合同。缺约束契约先补设计。

日常在本地执行 `make build`（全部 Go 模块 build/vet/gofmt）、`make check`（合同结构与隐私），改合同后执行 `make gen`、`make docs`，生成目录不手改。自部署 GitLab CI（[.gitlab-ci.yml](.gitlab-ci.yml)）只作推送后的兜底。工作方式见 [AGENTS.md](AGENTS.md)。

所有业务模块分别构建部署，通过 API/事件协作；统一后台外壳装配模块页面。服务共用的基础机制在 [libs/platform](libs/platform/README.md)，单机部署编排在 [deploy](deploy/README.md)。许可证见 [LICENSE](LICENSE)，借用规则见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
