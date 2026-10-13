# nice-wiki · 微服务设计仓库

本 main 从零建立分级设计契约，旧实现和旧文档保存在 old 分支的归档提交 272bddc。此阶段没有可运行的业务服务，文档检查通过只表示规格结构和引用自洽。

从 [文档总索引](docs/README.md) 开始；先读 [全局需求](docs/requirements.md)、[总架构](docs/architecture.md)、[服务地图](docs/service-map.md)、[公共协议](docs/protocols.md)，再进入模块总文档。每个模块索引其架构、数据、API、流程、文件、函数、运维和验收规格。

设计的机器来源是 contracts/modules.json、contracts/common.schema.json 与各模块 contracts 下的 OpenAPI、类型、事件、实现清单。所有未来文件和私有辅助函数都登记；开发者依照准确签名、字段、错误、步骤与状态规则实现，缺契约先补设计。

运行 `make check` 检查文档与契约。无 Make 环境可用 `python scripts/check_docs.py` 读取检查；编译、lint、类型和业务普通测试按 AGENTS.md 优先 VPS。当前未创建业务测试、样例或服务代码。

所有业务模块分别构建部署，通过 API/事件协作；统一后台外壳装配模块页面。许可证见 [LICENSE](LICENSE)，借用规则见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
