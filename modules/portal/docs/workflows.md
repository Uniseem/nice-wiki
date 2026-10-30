# 生命周期与工作流

初始化→登录→加载壳→可用/降级；编辑导航状态 idle→dirty→saving→saved 或 conflict；维护 gate OPEN→DRAINING→SAFE→OPEN。



业务操作在提交前检查取消和期限；提交后返回已记录回执，客户端超时并不等于未提交。

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

## 浏览器认证与敏感响应

同源与维护门禁通过后，登录/首次注册由公共可信代理解析器派生当前及上一UTC日网络HMAC并签入本次服务断言。刷新浏览器正文必须为空对象，入口只从唯一nw_refresh取秘密，编码内部正文后重算字节数与哈希。改密保留当前access/CSRF权威校验和精确静态动作委托。

上游200先有界读取、解码和构造安全session结果，再设置两枚Secure HttpOnly SameSite=Strict cookie；原始tokens/currentSessionTokens从不写浏览器正文。上游失败及敏感回执重放只返回安全Problem，不清旧cookie，也不重新发起登录/刷新。只有当前权威sessionId等于被撤销路径sessionId且收到200 revoked=true才清本设备cookie。响应已开始后取消或写失败只结束该响应，不追加第二份Problem。

完整本域handler覆盖与初始化/恢复权威尚未完成，NewApplication明确失败关闭；此认证适配源码不等于完整浏览器登录链已可运行。

## 访问审计脱敏

`RedactRequest` 是纯函数，不调用 analytics 或其他外部服务。成功结果只含静态路由的 method、routeId，以及请求的 requestId 与体字节数。cookie 与 authorization 不进入结果。query 敏感键和路径变量敏感标识尚未写明；这些字段以及其他请求头出现时失败关闭。

`EmitAccessAudit` 的输入已经是上述脱敏字段加上 `RequestOutcome`。`analytics:appendAudit` 的请求体与这些字段没有已写明的一一映射，函数返回 `DEPENDENCY_UNAVAILABLE`，不构造请求体，也不把 cookie、authorization、正文或原始地址写入结果。结果类型只有 `accepted`，没有 `degraded` 字段。

匿名统计只公开/v1/beacons：2048字节严格公开schema；入口从真实连接受信代理后缀与HTTP User-Agent注入内存来源、当前epoch，重算完整bodyHash后以portal纯服务身份投固定/v1/internal/beacons。内部入口不加入浏览器代理路由，原IP不日志或持久；内部正文4096字节上限，UA≤1024字节。
