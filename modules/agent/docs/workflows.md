# 原生 Agent

qa 提交当前消息及已复核来源的 checkpoint，消费请求并回送结果，最终取走文本与新 checkpoint；取消或原操作截止释放运行。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

本域不消费事件。

### 异步任务

本域没有登记的异步任务类型。

### 后台任务

本域没有后台任务。
<!-- flows:end -->
