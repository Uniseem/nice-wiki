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

## 浏览器认证与敏感响应

同源与维护门禁通过后，登录/首次注册由公共可信代理解析器派生当前及上一UTC日网络HMAC并签入本次服务断言。刷新浏览器正文必须为空对象，入口只从唯一nw_refresh取秘密，编码内部正文后重算字节数与哈希。改密保留当前access/CSRF权威校验和精确静态动作委托。

上游200先有界读取、解码和构造安全session结果，再设置两枚Secure HttpOnly SameSite=Strict cookie；原始tokens/currentSessionTokens从不写浏览器正文。上游失败及敏感回执重放只返回安全Problem，不清旧cookie，也不重新发起登录/刷新。只有当前权威sessionId等于被撤销路径sessionId且收到200 revoked=true才清本设备cookie。响应已开始后取消或写失败只结束该响应，不追加第二份Problem。

完整本域handler覆盖与初始化/恢复权威尚未完成，NewApplication明确失败关闭；此认证适配源码不等于完整浏览器登录链已可运行。
