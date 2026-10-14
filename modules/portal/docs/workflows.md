# 生命周期与工作流

初始化→登录→加载壳→可用/降级；编辑导航状态 idle→dirty→saving→saved 或 conflict；维护 gate OPEN→DRAINING→SAFE→OPEN。



业务操作在提交前检查取消和期限；提交后返回已记录回执，客户端超时并不等于未提交。恢复屏障拒绝新业务写入，排空已登记事务后才返回 safe；快照仅覆盖屏障水位；恢复暂存不修改active数据，active目录只在进程重启时整体换入或换回。

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
| `portal.CompileRouteRegistry` | 启动时（加载config/routes.json） | 校验并编译公开与管理 API 的静态路由表 |
<!-- flows:end -->
