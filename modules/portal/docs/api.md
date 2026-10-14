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
| GET `/v1/readiness` | `collectReadiness` | `portal.CollectReadiness` | `service` | `—` → 200 `CollectReadinessResult` | 读取入口 readiness | `VALIDATION` |
| POST `/v1/barriers` | `acquireBarrier` | `portal.AcquireBarrier` | `superadmin` | `BarrierRequest` → 200 `BarrierStatus` | 取得入口维护屏障 | `FORBIDDEN`、`LOCKED`、`DEPENDENCY_UNAVAILABLE` |
| DELETE `/v1/barriers/{barrierId}` | `releaseBarrier` | `portal.ReleaseBarrier` | `superadmin` | `releaseBarrierRequest` → 200 `ReleaseBarrierResult` | 释放入口屏障 | `FORBIDDEN`、`VERSION_MISMATCH` |
| GET `/healthz` | `health` | `portal.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `portal.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
<!-- openapi:end -->

