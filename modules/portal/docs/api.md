# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| GET `/v1/bootstrap` | `loadBootstrap` | `portal.LoadBootstrap` | `portal.use` | `—` → 200 `LoadBootstrapResult` | 加载后台外壳 | `VALIDATION`、`UNAUTHENTICATED`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/preferences` | `readPreferences` | `portal.ReadPreferences` | `portal.use` | `—` → 200 `ReadPreferencesResult` | 读自身偏好 | `FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/preferences` | `updatePreferences` | `portal.UpdatePreferences` | `portal.use` | `UpdatePreferencesInput` → 200 `UpdatePreferencesResult` | 修改自身偏好 | `VALIDATION`、`CONFLICT` |
| GET `/v1/events` | `openEventStream` | `portal.OpenEventStream` | `portal.use` | `—` → 200 `string` | 权限过滤 SSE | `VALIDATION`、`FORBIDDEN`、`STALE_RESULT` |
| GET `/v1/readiness` | `collectReadiness` | `portal.CollectReadiness` | `system.manage` | `—` → 200 `CollectReadinessResult` | 读取入口 readiness | `VALIDATION` |
| POST `/v1/barriers` | `acquireBarrier` | `portal.AcquireBarrier` | `service` | `BarrierRequest` → 200 `BarrierStatus` | 取得入口维护屏障 | `FORBIDDEN`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| DELETE `/v1/barriers/{barrierId}` | `releaseBarrier` | `portal.ReleaseBarrier` | `service` | `releaseBarrierRequest` → 200 `ReleaseBarrierResult` | 释放入口屏障 | `FORBIDDEN`、`VERSION_MISMATCH` |
| GET `/healthz` | `health` | `portal.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `portal.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
<!-- openapi:end -->


## 代理后的浏览器认证协议

四个内部identity操作仍由identity OpenAPI拥有；portal不新建重复method/path。portal OpenAPI的`x-browser-auth-adapters`登记浏览器适配来源，浏览器生成阶段以其requestSchema/responseSchema替换内部敏感DTO；内部Go服务/客户端与真实routes目录保留生产方契约。生成目录只由统一VPS的make gen写入。

浏览器refreshSession仅接受application/json空对象，HttpOnly nw_refresh在入口内存中转为内部RefreshSessionInput。服务断言绑定改写后的实际正文hash，匿名刷新不要求有效access或用户proof，仍要求固定部署Origin。登录/首次注册同时签入公共派生的网络scope；浏览器自造身份、XFF或scope头不转发给identity。

认证200结果只输出安全session及该操作允许的account/nextRoute；访问/刷新秘密仅进入Cookie。认证非200只保留已登记code/status/retryable/requestId与固定安全说明，不转发内部原始detail/未知字段，不改旧cookie。未知字段、截断、超限、错误媒体类型或缺少关键cookie材料在任何响应写出前失败关闭。当前设备DELETE revokeSession成功的清cookie规则见OpenAPI的`x-browser-session-revocation`。
