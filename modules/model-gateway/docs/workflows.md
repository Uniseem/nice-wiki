# 统一模型网关算法与工作流

状态机：Invocation: accepted→reserved→dispatching→streaming→succeeded|failed|cancel_requested→cancelled|charge_unknown；charge_unknown→reconciled。Reservation: held→settled|released；未知账单保持held。Provider: disabled|enabled。

逐函数的输入、输出与步骤见 [functions.md](functions.md)，本文件只写状态机、跨函数流程与规则。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

本域不消费事件。

### 异步任务

本域没有登记的异步任务类型。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `model-gateway.LoadRuntime` | 启动时 | 启动并验证本服务私有持久化与密钥引用 |
| `model-gateway.RecoverLeases` | 启动时及每30秒 | 恢复崩溃中的调用 |
| `model-gateway.PublishOutbox` | 启动后持续，空闲每1秒 | 持久发布事件 |
<!-- flows:end -->
