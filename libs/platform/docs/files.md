# platform 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 32 个：手写 30、生成 1、配置 1。

| 路径 | 类别 | 职责 |
|---|---|---|
| `libs/platform/serviceauth/assertion.go` | 手写 | 服务断言签发与验证 |
| `libs/platform/serviceauth/delegation.go` | 手写 | 服务签名、用户委托验证、恢复激活回执与作用域绑定的分页游标签名 |
| `libs/platform/opctx/context.go` | 手写 | 可信OperationContext构建 |
| `libs/platform/problem/problem.go` | 手写 | 公共Problem输出 |
| `libs/platform/outbox/outbox.go` | 手写 | 事务内outbox登记 |
| `libs/platform/outbox/relay.go` | 手写 | outbox转发循环 |
| `libs/platform/inbox/inbox.go` | 手写 | 事件inbox去重 |
| `libs/platform/inbox/consumer.go` | 手写 | 事件订阅循环 |
| `libs/platform/worker/loop.go` | 手写 | 任务领取循环 |
| `libs/platform/modelclient/invoke.go` | 手写 | 模型调用助手 |
| `libs/platform/lifecycle/config.go` | 手写 | 服务配置加载 |
| `libs/platform/lifecycle/runtime.go` | 手写 | 运行状态读取与监视 |
| `libs/platform/apivalidate/validator.go` | 手写 | HTTP入口统一请求校验中间件（kin-openapi openapi3filter） |
| `libs/platform/lifecycle/service.go` | 手写 | 进程模板 |
| `libs/platform/cmd/nw-keys/main.go` | 手写 | 服务密钥生成工具 |
| `libs/platform/platform.go` | 手写 | 公共契约类型、接口和确定性纯工具，禁止业务规则 |
| `libs/platform/go.mod` | 配置 | Go module nice-wiki/libs/platform；只依赖标准库与ADR0005登记的第三方库；各服务以相对replace引用 |
| `libs/platform/go.sum` | 生成 | 依赖校验锁；由go mod download生成 |
| `libs/platform/worker/client.go` | 手写 | 生成jobs HTTP端口到公共任务租约机制的一次性生产方schema适配 |
| `libs/platform/modelclient/client.go` | 手写 | 真实生成网关HTTP端口到公共模型调用机制的单一严格适配 |
| `libs/platform/serviceauth/admission.go` | 手写 | 统一风控凭证签名机制 |
| `libs/platform/inbox/client.go` | 手写 | 真实事件订阅HTTP窄适配、失败关闭与准确累计统计 |
| `libs/platform/lifecycle/task_group.go` | 手写 | 受监督本进程任务去重、并发、原截止与关闭排空 |
| `libs/platform/lifecycle/parser_process.go` | 手写 | 受控有界子进程解析，不继承配置密钥或业务句柄 |
| `libs/platform/lifecycle/parser_sandbox_linux_amd64.go` | 手写 | Linux amd64封闭系统调用允许集与硬资源限制 |
| `libs/platform/lifecycle/parser_sandbox_other.go` | 手写 | 其他平台相同入口失败关闭，不降级非隔离PDF |
| `libs/platform/web/module-api.ts` | 手写 | 浏览器非序列化模块API端口与bootstrap类型，不暴露HttpOnly秘密 |
| `libs/platform/lifecycle/identity.go` | 手写 | 各域启动服务身份描述与本站签名公钥匹配共用边界 |
| `libs/platform/lifecycle/task_secret.go` | 手写 | 通用有界异步任务秘密加密，不含业务授权与持久策略 |
| `libs/platform/theme/theme.go` | 手写 | 外观包标准：解析与校验 zip、内容摘要、重新打包 |
| `libs/platform/theme/css.go` | 手写 | 外观样式表校验，配色/字体/版式渲染成公开站内联样式 |
| `libs/platform/theme/builtin.go` | 手写 | 内置外观（builtin/ 下的外观包） |
