# API 与授权

唯一机器权威为 [openapi.json](../contracts/openapi.json)。本页索引每个操作；具体参数、响应与错误对应 handler 的 types/implementation。请求正文严格拒绝未知字段，路径参数与正文同名字段不得矛盾。GET 不需要 CSRF；所有 cookie 身份写入必须同 Origin 并核对 X-CSRF-Token。

## acquireBarrier

`POST /v1/barriers` — 取得可恢复的数据屏障。

- handler：`identity.AcquireBarrier`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：BarrierRequest；成功 200：BarrierStatus。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## exportSnapshot

`POST /v1/snapshots:export` — 导出屏障水位的一致快照。

- handler：`identity.ExportSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotExportRequest；成功 200：SnapshotExportResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## validateSnapshot

`POST /v1/snapshots:validate` — 验证快照可恢复性。

- handler：`identity.ValidateSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotValidateRequest；成功 200：SnapshotValidation。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## prepareSnapshot

`POST /v1/snapshots:prepare` — 在私有 staging 区准备恢复。

- handler：`identity.PrepareSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotPrepareRequest；成功 200：SnapshotStageResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## commitSnapshot

`POST /v1/snapshots:commit` — 激活已准备恢复数据。

- handler：`identity.CommitSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## rollbackSnapshot

`POST /v1/snapshots:rollback` — 按协调决定回滚恢复阶段。

- handler：`identity.RollbackSnapshot`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：SnapshotCommitRequest；成功 200：SnapshotApplyResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；INTEGRITY_ERROR（哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）；DEADLINE_EXCEEDED（依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准）。

## releaseBarrier

`DELETE /v1/barriers/{barrierId}` — 释放屏障。

- handler：`identity.ReleaseBarrier`；权限：`superadmin`。
- 路径参数：barrierId；查询参数：无。
- 请求：ReleaseBarrierInput；成功 200：ReleaseBarrierResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；LOCKED（锁、屏障、删除 fence 或正在处理的互斥操作阻止执行）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）。

## readRegistrationState

`GET /v1/registration` — 决定后台显示首次注册或登录。

- handler：`identity.ReadRegistrationState`；权限：`anonymous`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadRegistrationStateResult。
- 错误：DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## registerFirstSuperadmin

`POST /v1/registration` — 首个注册账户原子成为唯一超管。

- handler：`identity.RegisterFirstSuperadmin`；权限：`anonymous`。
- 路径参数：无；查询参数：无。
- 请求：RegisterFirstSuperadminInput；成功 200：RegisterFirstSuperadminResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；RATE_LIMITED（频率、并发或容量上限已达，返回 retryAfter）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## createAccount

`POST /v1/accounts` — 超管创建子账号并标记强改。

- handler：`identity.CreateAccount`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：CreateAccountInput；成功 200：CreateAccountResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## listAccounts

`GET /v1/accounts` — 超管读取子账号列表。

- handler：`identity.ListAccounts`；权限：`superadmin`。
- 路径参数：无；查询参数：limit, cursor。
- 请求：无正文；成功 200：ListAccountsResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## readSelf

`GET /v1/self` — 读取当前账户。

- handler：`identity.ReadSelf`；权限：`account.self`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadSelfResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；NOT_FOUND（资源不存在或在当前主体作用域中不可见）。

## setAccountStatus

`PUT /v1/accounts/{accountId}/status` — 停用/恢复/删除子账号。

- handler：`identity.SetAccountStatus`；权限：`superadmin`。
- 路径参数：accountId；查询参数：无。
- 请求：SetAccountStatusInput；成功 200：SetAccountStatusResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## setAccountPermissions

`PUT /v1/accounts/{accountId}/permissions` — 修改10项功能权限。

- handler：`identity.SetAccountPermissions`；权限：`superadmin`。
- 路径参数：accountId；查询参数：无。
- 请求：SetAccountPermissionsInput；成功 200：SetAccountPermissionsResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## resetAccountPassword

`POST /v1/accounts/{accountId}/password:reset` — 超管重置子账号初始口令。

- handler：`identity.ResetAccountPassword`；权限：`superadmin`。
- 路径参数：accountId；查询参数：无。
- 请求：ResetAccountPasswordInput；成功 200：ResetAccountPasswordResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## changePassword

`POST /v1/self/password:change` — 改口令并结束其他设备登录。

- handler：`identity.ChangePassword`；权限：`account.self`。
- 路径参数：无；查询参数：无。
- 请求：ChangePasswordInput；成功 200：ChangePasswordResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## createSession

`POST /v1/sessions` — 账号名口令登录。

- handler：`identity.CreateSession`；权限：`anonymous`。
- 路径参数：无；查询参数：无。
- 请求：CreateSessionInput；成功 200：CreateSessionResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；RATE_LIMITED（频率、并发或容量上限已达，返回 retryAfter）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## refreshSession

`POST /v1/sessions:refresh` — 内部透明轮换session。

- handler：`identity.RefreshSession`；权限：`anonymous`。
- 路径参数：无；查询参数：无。
- 请求：RefreshSessionInput；成功 200：RefreshSessionResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readSessions

`GET /v1/sessions` — 读取自身登录设备。

- handler：`identity.ReadSessions`；权限：`account.self`。
- 路径参数：无；查询参数：limit, cursor。
- 请求：无正文；成功 200：ReadSessionsResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）。

## revokeSession

`DELETE /v1/sessions/{sessionId}` — 退出当前或指定自身登录。

- handler：`identity.RevokeSession`；权限：`account.self`。
- 路径参数：sessionId；查询参数：无。
- 请求：RevokeSessionInput；成功 200：RevokeSessionResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## verifySessionCSRF

`POST /v1/csrf:verify` — portal内部核对CSRF。

- handler：`identity.VerifySessionCSRF`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：VerifySessionCSRFInput；成功 200：VerifySessionCSRFResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## checkAuthorization

`POST /v1/authorizations/check` — 按最新账号功能权限授权。

- handler：`identity.CheckAuthorization`；权限：`service`。
- 路径参数：无；查询参数：无。
- 请求：AuthorizationRequest；成功 200：AuthorizationDecision。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## readPreferences

`GET /v1/preferences` — 读自身后台偏好。

- handler：`identity.ReadPreferences`；权限：`portal.use`。
- 路径参数：无；查询参数：无。
- 请求：无正文；成功 200：ReadPreferencesResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## updatePreferences

`PUT /v1/preferences` — 保存自身主题/语言/默认后台页。

- handler：`identity.UpdatePreferences`；权限：`portal.use`。
- 路径参数：无；查询参数：无。
- 请求：UpdatePreferencesInput；成功 200：UpdatePreferencesResult。
- 错误：VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；CONFLICT（状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## issueRecoveryAuthorization

`POST /v1/recovery-authorizations` — 超管批准后签发有限恢复许可。

- handler：`identity.IssueRecoveryAuthorization`；权限：`superadmin`。
- 路径参数：无；查询参数：无。
- 请求：IssueRecoveryAuthorizationInput；成功 200：IssueRecoveryAuthorizationResult。
- 错误：UNAUTHENTICATED（会话、委托或服务证明缺失、失效或被撤销）；FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VALIDATION（字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

## revokeRecoveryAuthorization

`DELETE /v1/recovery-authorizations/{restoreId}` — 撤销尚未完成的恢复许可。

- handler：`identity.RevokeRecoveryAuthorization`；权限：`superadmin`。
- 路径参数：restoreId；查询参数：无。
- 请求：RevokeRecoveryAuthorizationInput；成功 200：RevokeRecoveryAuthorizationResult。
- 错误：FORBIDDEN（主体没有所需操作权限，或站点/资源作用域不匹配）；VERSION_MISMATCH（expectedRevision、对象版本或 dataEpoch 已改变）；DEPENDENCY_UNAVAILABLE（权威依赖、存储或密钥不可用；不可把未知状态当允许）。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
