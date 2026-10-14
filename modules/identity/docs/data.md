# 数据、权威与持久化

私有accounts/credential/session/permission/receipt/outbox业务库；独立control.db保存bootstrapClaim、唯一超管身份核对和服务签名密钥。control/account-index.json可供服务器查明文账号名；control/reset-superadmin.txt为带外一次恢复输入。账号/密钥/索引不纳入git。控制库与服务签名密钥不随业务快照替换。

账号名不可修改，删除账户保留墓碑姓名供git历史/审计署名。权限只10项功能全站勾选，首次创建schema permissions可省略且后端默认全选；技术grants仅映射实现，不增加产品文档scope。会话可重建，丢失仅要求重新登录。

技术授权唯一映射来源为根contracts/permission-groups.json，普通账户固定基础grants加勾选10组；language与translation不同。job.manage只用于获授权任务owner对象委托，不等于任意任务管理。

类型、required/额外字段、枚举、最大长度、空值与格式均由 [types.json](../contracts/types.json) 定义。函数输入和结果各有独立 schema，内部句柄类型标 x-internal。

| 类型 | 精确含义 |
|---|---|
| Identifier | Identifier |
| Revision | Revision |
| Hash | Hash |
| Timestamp | Timestamp |
| AuthContext | AuthContext |
| Error | Error |
| Problem | Problem |
| PageRequest | PageRequest |
| PageInfo | PageInfo |
| PublishedRevisionRef | PublishedRevisionRef |
| PublicationValidation | PublicationValidation |
| AssetRef | AssetRef |
| ObjectRef | ObjectRef |
| BarrierRequest | BarrierRequest |
| BarrierStatus | BarrierStatus |
| SnapshotRef | SnapshotRef |
| SnapshotExportRequest | SnapshotExportRequest |
| SnapshotExportResult | SnapshotExportResult |
| SnapshotStageResult | SnapshotStageResult |
| EventEnvelope | EventEnvelope |
| HealthStatus | HealthStatus |
| AccountName | 创建后不可修改；NFC、不可有控制字符/首尾空白；明文可供服务器查账号 |
| Password | 敏感口令，至少8位；不强制大小写符号组合；不trim；禁止日志 |
| FunctionPermission | 已确认10项功能，默认全部勾选 |
| PermissionSelection | 无重复，首次创建省略时默认10项全部true；不按文档范围授权 |
| Account | 账户元数据；删除后署名保留，不返hash |
| CredentialRecord | 敏感私有凭据 |
| Session | 私有会话元数据 |
| SessionTokens | 敏感会话结果 |
| AuthorizationRequest | 授权请求；产品按功能、无文档范围 |
| AuthorizationDecision | 单次权威授权 |
| PortalPreferences | 自身UI偏好 |
| IdentityChanged | 不含口令/IP的撤销事件 |
| AccountIndex | identity私有control/account-index.json，不进git；保留服务器查账号功能 |
| ServerResetCandidate | 仅私有filewatcher内存，来源control/reset-superadmin.txt |
| ReadRegistrationStateInput | 决定后台显示首次注册或登录的输入 |
| ReadRegistrationStateResult | 决定后台显示首次注册或登录的结果 |
| RegisterFirstSuperadminInput | 首个注册账户原子成为唯一超管的输入 |
| RegisterFirstSuperadminResult | 首个注册账户原子成为唯一超管的结果 |
| CreateAccountInput | 超管创建子账号并标记强改的输入 |
| CreateAccountResult | 超管创建子账号并标记强改的结果 |
| ReadSelfInput | 读取当前账户的输入 |
| ReadSelfResult | 读取当前账户的结果 |
| ListAccountsInput | 超管读取子账号列表的输入 |
| ListAccountsResult | 超管读取子账号列表的结果 |
| SetAccountStatusInput | 停用/恢复/删除子账号的输入 |
| SetAccountStatusResult | 停用/恢复/删除子账号的结果 |
| SetAccountPermissionsInput | 修改10项功能权限的输入 |
| SetAccountPermissionsResult | 修改10项功能权限的结果 |
| ResetAccountPasswordInput | 超管重置子账号初始口令的输入 |
| ResetAccountPasswordResult | 超管重置子账号初始口令的结果 |
| NormalizeAccountNameInput | 规范不可变账号名的输入 |
| NormalizeAccountNameResult | 规范不可变账号名的结果 |
| AssertSuperadminImmutableInput | 保护唯一超管永久有效的输入 |
| AssertSuperadminImmutableResult | 保护唯一超管永久有效的结果 |
| MapFunctionGrantsInput | 将已确认功能勾选映射成服务权限的输入 |
| MapFunctionGrantsResult | 将已确认功能勾选映射成服务权限的结果 |
| HashPasswordInput | 生成Argon2id口令哈希的输入 |
| HashPasswordResult | 生成Argon2id口令哈希的结果 |
| VerifyPasswordInput | 核对口令的输入 |
| VerifyPasswordResult | 核对口令的结果 |
| ValidatePasswordChangeInput | 验证改密三输入规则的输入 |
| ValidatePasswordChangeResult | 验证改密三输入规则的结果 |
| ChangePasswordInput | 改口令并结束其他设备登录的输入 |
| ChangePasswordResult | 改口令并结束其他设备登录的结果 |
| CreateSessionInput | 账号名口令登录的输入 |
| CreateSessionResult | 账号名口令登录的结果 |
| IssueSessionTokensInput | 签发会话和CSRFcookie数据的输入 |
| IssueSessionTokensResult | 签发会话和CSRFcookie数据的结果 |
| RefreshSessionInput | 内部透明轮换session的输入 |
| RefreshSessionResult | 内部透明轮换session的结果 |
| ReadSessionsInput | 读取自身登录设备的输入 |
| ReadSessionsResult | 读取自身登录设备的结果 |
| RevokeSessionInput | 退出当前或指定自身登录的输入 |
| RevokeSessionResult | 退出当前或指定自身登录的结果 |
| RevokeSessionFamilyInput | 使refresh族全部失效的输入 |
| RevokeSessionFamilyResult | 使refresh族全部失效的结果 |
| RevokeAllSessionsInput | 立即失效账户全部登录的输入 |
| RevokeAllSessionsResult | 立即失效账户全部登录的结果 |
| RevokeOtherSessionsInput | 改密后只保留当前设备的输入 |
| RevokeOtherSessionsResult | 改密后只保留当前设备的结果 |
| VerifySessionCSRFInput | portal内部核对CSRF的输入 |
| VerifySessionCSRFResult | portal内部核对CSRF的结果 |
| ReadPreferencesInput | 读自身后台偏好的输入 |
| ReadPreferencesResult | 读自身后台偏好的结果 |
| UpdatePreferencesInput | 保存自身主题/语言/默认后台页的输入 |
| UpdatePreferencesResult | 保存自身主题/语言/默认后台页的结果 |
| ReadServerAccountIndexInput | 服务器运维查明文账号索引的输入 |
| ReadServerAccountIndexResult | 服务器运维查明文账号索引的结果 |
| WriteAccountIndexInput | 维持服务器查账号能力的输入 |
| WriteAccountIndexResult | 维持服务器查账号能力的结果 |
| ObserveSuperadminResetFileInput | 检测服务器超管重置文件的输入 |
| ObserveSuperadminResetFileResult | 检测服务器超管重置文件的结果 |
| ApplySuperadminResetInput | 用服务器文件恢复唯一超管口令的输入 |
| ApplySuperadminResetResult | 用服务器文件恢复唯一超管口令的结果 |
| ResolveIdentityOperationInput | 读取幂等回执的输入 |
| ResolveIdentityOperationResult | 读取幂等回执的结果 |
| PublishIdentityEventInput | 与身份提交原子登记事件的输入 |
| PublishIdentityEventResult | 与身份提交原子登记事件的结果 |
| InvalidateRestoredSessionsInput | 恢复数据后使备份登录失效的输入 |
| InvalidateRestoredSessionsResult | 恢复数据后使备份登录失效的结果 |
| ReleaseBarrierInput | 释放已协调完成的屏障的输入 |
| ReleaseBarrierResult | 释放已协调完成的屏障的结果 |
| releaseBarrierRequest | 释放屏障请求体 |
| setAccountStatusRequest | 停用/恢复/删除子账号请求体 |
| setAccountPermissionsRequest | 修改10项功能权限请求体 |
| resetAccountPasswordRequest | 超管重置子账号初始口令请求体 |
| revokeSessionRequest | 退出当前或指定自身登录请求体 |

恢复重建必须验证 manifest 哈希和私有事务水位。derived cache 可重建；权威记录缺失或 schema 未知应隔离并停止对外发布。
