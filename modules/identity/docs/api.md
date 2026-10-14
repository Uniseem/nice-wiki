# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

<!-- openapi:start -->
## 接口清单

本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。

| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |
|---|---|---|---|---|---|---|
| POST `/v1/barriers` | `acquireBarrier` | `identity.AcquireBarrier` | `superadmin` | `BarrierRequest` → 200 `BarrierStatus` | 取得可恢复的数据屏障 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| POST `/v1/snapshots:export` | `exportSnapshot` | `identity.ExportSnapshot` | `superadmin` | `SnapshotExportRequest` → 200 `SnapshotExportResult` | 导出屏障水位的一致快照 | `FORBIDDEN`、`LOCKED`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE`、`DEADLINE_EXCEEDED` |
| DELETE `/v1/barriers/{barrierId}` | `releaseBarrier` | `identity.ReleaseBarrier` | `superadmin` | `releaseBarrierRequest` → 200 `ReleaseBarrierResult` | 释放屏障 | `FORBIDDEN`、`VERSION_MISMATCH` |
| GET `/v1/registration` | `readRegistrationState` | `identity.ReadRegistrationState` | `anonymous` | `—` → 200 `ReadRegistrationStateResult` | 决定后台显示首次注册或登录 | `DEPENDENCY_UNAVAILABLE` |
| POST `/v1/registration` | `registerFirstSuperadmin` | `identity.RegisterFirstSuperadmin` | `anonymous` | `RegisterFirstSuperadminInput` → 200 `RegisterFirstSuperadminResult` | 首个注册账户原子成为唯一超管 | `VALIDATION`、`CONFLICT`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/accounts` | `listAccounts` | `identity.ListAccounts` | `superadmin` | `—` → 200 `ListAccountsResult` | 超管读取子账号列表 | `VALIDATION`、`FORBIDDEN` |
| POST `/v1/accounts` | `createAccount` | `identity.CreateAccount` | `superadmin` | `CreateAccountInput` → 200 `CreateAccountResult` | 超管创建子账号并标记强改 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/self` | `readSelf` | `identity.ReadSelf` | `account.self` | `—` → 200 `ReadSelfResult` | 读取当前账户 | `UNAUTHENTICATED`、`NOT_FOUND` |
| PUT `/v1/accounts/{accountId}/status` | `setAccountStatus` | `identity.SetAccountStatus` | `superadmin` | `setAccountStatusRequest` → 200 `SetAccountStatusResult` | 停用/恢复/删除子账号 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/accounts/{accountId}/permissions` | `setAccountPermissions` | `identity.SetAccountPermissions` | `superadmin` | `setAccountPermissionsRequest` → 200 `SetAccountPermissionsResult` | 修改10项功能权限 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/accounts/{accountId}/password:reset` | `resetAccountPassword` | `identity.ResetAccountPassword` | `superadmin` | `resetAccountPasswordRequest` → 200 `ResetAccountPasswordResult` | 超管重置子账号初始口令 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/self/password:change` | `changePassword` | `identity.ChangePassword` | `account.self` | `ChangePasswordInput` → 200 `ChangePasswordResult` | 改口令并结束其他设备登录 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/sessions` | `readSessions` | `identity.ReadSessions` | `account.self` | `—` → 200 `ReadSessionsResult` | 读取自身登录设备 | `VALIDATION`、`UNAUTHENTICATED` |
| POST `/v1/sessions` | `createSession` | `identity.CreateSession` | `anonymous` | `CreateSessionInput` → 200 `CreateSessionResult` | 账号名口令登录 | `UNAUTHENTICATED`、`RATE_LIMITED`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/sessions:refresh` | `refreshSession` | `identity.RefreshSession` | `anonymous` | `RefreshSessionInput` → 200 `RefreshSessionResult` | 内部透明轮换session | `UNAUTHENTICATED`、`DEPENDENCY_UNAVAILABLE` |
| DELETE `/v1/sessions/{sessionId}` | `revokeSession` | `identity.RevokeSession` | `account.self` | `revokeSessionRequest` → 200 `RevokeSessionResult` | 退出当前或指定自身登录 | `UNAUTHENTICATED`、`FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/csrf:verify` | `verifySessionCSRF` | `identity.VerifySessionCSRF` | `service` | `VerifySessionCSRFInput` → 200 `VerifySessionCSRFResult` | portal内部核对CSRF | `UNAUTHENTICATED`、`FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/authorizations/check` | `checkAuthorization` | `identity.CheckAuthorization` | `service` | `AuthorizationRequest` → 200 `AuthorizationDecision` | 按最新账号功能权限授权 | `UNAUTHENTICATED`、`FORBIDDEN`、`DEPENDENCY_UNAVAILABLE` |
| GET `/v1/preferences` | `readPreferences` | `identity.ReadPreferences` | `portal.use` | `—` → 200 `ReadPreferencesResult` | 读自身后台偏好 | `UNAUTHENTICATED`、`DEPENDENCY_UNAVAILABLE` |
| PUT `/v1/preferences` | `updatePreferences` | `identity.UpdatePreferences` | `portal.use` | `UpdatePreferencesInput` → 200 `UpdatePreferencesResult` | 保存自身主题/语言/默认后台页 | `VALIDATION`、`UNAUTHENTICATED`、`FORBIDDEN`、`VERSION_MISMATCH`、`DEPENDENCY_UNAVAILABLE` |
| GET `/healthz` | `health` | `identity.Health` | `anonymous` | `—` → 200 `HealthStatus` | 本域进程存活 | `HTTP 503` |
| GET `/readyz` | `ready` | `identity.Ready` | `anonymous` | `—` → 200 `ServiceReadiness` | 本域启动/恢复就绪 | `HTTP 503` |
| POST `/v1/snapshots:stage` | `stageSnapshot` | `identity.StageSnapshot` | `superadmin` | `SnapshotStageRequest` → 200 `SnapshotStageResult` | 暂存本域恢复归档，不改active目录 | `FORBIDDEN`、`CONFLICT`、`LOCKED`、`UNSUPPORTED_SCHEMA`、`INTEGRITY_ERROR`、`DEPENDENCY_UNAVAILABLE` |
| POST `/v1/snapshots:discard` | `discardSnapshot` | `identity.DiscardSnapshot` | `superadmin` | `SnapshotDiscardRequest` → 200 `SnapshotDiscardResult` | 删除恢复暂存或换出的旧数据目录 | `FORBIDDEN`、`LOCKED` |
<!-- openapi:end -->

