# nice-wiki · 微服务文档站（重建中）

本 main 从零建立分级设计契约并按R1实现微服务，旧实现和旧文档保存在old归档提交272bddc。当前公共底座与部分业务切片已实现，最小完整业务链路、行为验收和部署尚未完成；最新事实见 [STATUS.md](STATUS.md)。

从 [文档总索引](docs/README.md) 开始；先读 [全局需求](docs/requirements.md)、[总架构](docs/architecture.md)、[服务地图](docs/service-map.md)、[公共协议](docs/protocols.md)，再进入模块总文档。每个模块索引其架构、数据、API、流程、文件、函数、运维和验收规格。

设计的机器来源是 contracts/modules.json、contracts/common.schema.json 与各模块 contracts 下的 OpenAPI、类型、事件、实现清单。函数分约束与参考两级：对外处理、事件、任务和启动入口依照准确签名、字段、错误、步骤与状态规则实现；内部辅助拆分是参考，实现时可调整并在交付时同步合同。缺约束契约先补设计。

运行 `make check` 检查文档与契约；改合同后运行 `make docs` 重新生成各模块函数与文件目录。无 Make 环境可用 `python scripts/check_docs.py` 读取检查；编译、lint、类型检查与已有普通测试按AGENTS.md全部在VPS执行，不自行回退本机。当前没有业务测试或行为验收证据。

所有业务模块分别构建部署，通过 API/事件协作；统一后台外壳装配模块页面。服务共用的基础机制在 [libs/platform](libs/platform/README.md)，单机部署编排在 [deploy](deploy/README.md)。许可证见 [LICENSE](LICENSE)，借用规则见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
