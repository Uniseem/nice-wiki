# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

## loadBootstrap

`GET /v1/bootstrap` — 加载后台外壳。

- handler：`portal.LoadBootstrap`；权限：`portal.use`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：LoadBootstrapResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## readPreferences

`GET /v1/preferences` — 读自身偏好。

- handler：`portal.ReadPreferences`；权限：`portal.use`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadPreferencesResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## updatePreferences

`PUT /v1/preferences` — 修改自身偏好。

- handler：`portal.UpdatePreferences`；权限：`portal.use`。
- 路径参数：无；查询参数：无。
- 请求：UpdatePreferencesInput；成功 200：UpdatePreferencesResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）。

## openEventStream

`GET /v1/events` — 权限过滤 SSE。

- handler：`portal.OpenEventStream`；权限：`portal.use`。
- 路径参数：无；查询参数：cursor。
- 请求：无正文；成功 200：PortalEvent（SSE 流）。
- 错误：STALE_RESULT（事件/计算针对已过期源版本，拒绝覆盖新状态）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## collectReadiness

`GET /v1/readiness` — 读取入口 readiness。

- handler：`portal.CollectReadiness`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：CollectReadinessResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## acquireBarrier

`POST /v1/barriers` — 取得入口维护屏障。

- handler：`portal.AcquireBarrier`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：BarrierRequest；成功 200：BarrierStatus。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## releaseBarrier

`DELETE /v1/barriers/{barrierId}` — 释放入口屏障。

- handler：`portal.ReleaseBarrier`；权限：`superadmin`。
- 路径参数：barrierId；查询参数：无。
- 请求：ReleaseBarrierInput；成功 200：ReleaseBarrierResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
