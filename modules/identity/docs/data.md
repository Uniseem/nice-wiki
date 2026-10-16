# 数据、权威与持久化

私有accounts/credential/session/permission/receipt/outbox业务库；独立control.db保存bootstrapClaim、唯一超管身份核对和服务签名密钥。control/account-index.json可供服务器查明文账号名；control/reset-superadmin.txt为带外一次恢复输入。账号/密钥/索引不纳入git。控制库与服务签名密钥不随业务快照替换。

账号名不可修改，删除账户保留墓碑姓名供git历史/审计署名。权限只10项功能全站勾选，首次创建schema permissions可省略且后端默认全选；技术grants仅映射实现，不增加产品文档scope。会话可重建，丢失仅要求重新登录。认证代次保存在active身份库的auth_state单行；恢复启动决定的NewAuthEpoch必须与此权威同步落盘，并撤销全部session和refresh族。session_families是一对一设备族，保存access/refresh/CSRF摘要及auth/data epoch；consumed_refresh_tokens保留已消费摘要供复用撤销判定，不保存原始cookie值。

技术授权唯一映射来源为根contracts/permission-groups.json，普通账户固定基础grants加勾选10组；language与translation不同。job.manage由根jobManagementGrants目录中任一已有业务能力派生，只供portal入口粗预检，不增加用户功能开关。jobs须按持久owner/type再次核对具体业务权限，不能据此管理任意任务或系统任务。

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

## 自身会话列表与撤销持久化（R1-01）

`0002_session_revocation.sql` 仅为 `operation_receipts` 追加可空 `result_json` 和自身会话分页索引，不改写旧会话。自有账户会话撤销事务保存完整计数及其哈希；幂等重放返回原计数，不再次撤销首次提交后新建的会话。旧回执没有完整结果时拒绝猜测恢复结果。嵌入账户状态迁移事务时，回执与提交由外层用例统一负责，身份事件函数只追加同事务 outbox。

自身会话列表按 `issued_at/session_id` 升序键集分页，签名游标绑定当前 `siteId/accountId/dataEpoch` 和 `identity.sessions` 作用域；会话摘要、口令、令牌及网络地址均不进入列表。家族、令牌摘要、认证代次及签发协议随下一工作包确认，未在本迁移预埋。


本轮账号事务落地：active的operation_receipts追加digest_key_id；含口令输入只持久平台按私钥seed域分离生成的HMAC，不存普通可离线猜测SHA。keyId改变而旧key不可用时同op失败关闭，不能当新请求。登录、注册、改密安全回执仅保存account/session/credential引用，raw cookie只存在首次成功响应。

独立control/control.db的schema1保存bootstrap_claim不可逆决定：原operation/caller/actor、HMAC/keyId/dataEpoch、固定accountId/siteId、claimedAt以及reserved/committed。控制库不随active恢复替换；只允许相同原op及原epoch续未决，恢复空active不重开注册。active提交和control提交不宣称跨库原子，启动按安全回执补记已提交控制决定。

账号索引是active账户的派生运维文件；账号业务互斥覆盖active提交至索引补齐，索引自身锁覆盖水位读取与安装，control journal的identity.account-index决定在启动按active权威补齐。未知决定或控制索引领先活动权威仍隔离，不静默回退覆盖。


账号管理回执仅保存完整安全Account原结果；重置输入含原始新口令，只存平台私有HMAC/keyId，不存口令或token。RevokeAllSessions的私有count为非负int64，与未设总数上限的会话模型一致，不因超过10000而阻止安全撤销；不改变产品会话数量策略。分页游标只承载不可变accountName/accountId排序位置，签名作用域绑定site/admin/dataEpoch和子账号筛选，不能当授权凭据。

旧只读hash查询ResolveIdentityOperation已移除；当前写用例直接在原私有事务中用readIdentityReceipt核对HMAC/keyId并回放完整安全结果，避免为未调用的参考助手保留孤立入口。

第四轮本人后台偏好仍使用既有preferences表，以当前权威session/site/account核权后仅按本人account_id访问，不收外部主体键。无行读取system/zh-Hans/dashboard/revision0且不写默认行；新保存须嵌套revision等于expectedRevision，首写仅允许0并生成1，随后每个新成功操作CAS加1（同值也加1）。完整preferences安全结果和HMAC/keyId回执同事务；同op回放原结果不覆写后来值。偏好不改变账号/认证水位，不进入账号索引或授权变更outbox。

允许后台语言和route key仅来自根公共目录生成的gen.InterfaceLocales、gen.AdminRoutes。当前内置后台界面语言为zh-Hans/en，与站点翻译语种分离；目标页按目录Permissions中任一当前权威grant核验。持久记录读取不因后来的权限变化偷偷改写，返回route key本身不授权限。

单次授权响应追加当前已校验主体的不可变accountName，与会话/账户权威同一快照读取，供content编辑锁显示持有人；不接收外部显示名，不返回其他账户资料，不改变grants。
