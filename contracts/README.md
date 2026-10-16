# 公共机器契约

[模块清单](modules.json)、[公共类型](common.schema.json)、[错误](errors.json)、[权限](permissions.json)是全域共享来源。各模块contracts目录保存其OpenAPI、业务类型、事件和未来文件函数清单；人类规范见 [文档标准](../docs/documentation-standard.md)。

公共类型变更须检查所有引用模块并定义兼容版本；不复制相同字段成为第二个源。跨域函数依赖引用module:operationId，不能指向另一个模块的内部函数。

[AI任务目录](ai-tasks.json)统一登记任务归属、默认档位及各档位的覆盖模型能力门槛。`make gen`将该目录投影到model-gateway既有的`internal/gen/clients.gen.go`；运行时只读取生成表，不读取仓库设计文件或手写第二份任务枚举。


产品行为以 [需求保留矩阵](../docs/product-preservation.md) 和 [旧设计基线](../docs/product-baseline/README.md) 为准；AI上下文与缓存见 [详细规格](../docs/ai-context-and-cache.md)。
