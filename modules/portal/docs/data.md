# 数据、权威与持久化

无业务私有库、无正文目录；仅进程 gate、短期读取缓存及结构化运行日志。用户偏好位于 identity 快照。



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
| PermissionRequirement | 路由权限 |
| RouteDefinition | 静态入口路由 |
| RouteRegistry | 只读内部路由索引 |
| ResolvedRoute | 匹配结果 |
| HeaderMap | HTTP 头的有界内部表示 |
| IncomingRequest | 入口请求 |
| RequestLimits | 路由读取预算 |
| ValidatedEnvelope | 完成入口基础校验的请求 |
| SessionCookies | 敏感请求凭据 |
| InternalProof | 入口内部短期 proof |
| UpstreamResponse | 内部上游响应 |
| Capability | 后台能力 |
| NavigationItem | 后台导航 |
| SiteFeatureFlags | 站点展示功能 |
| PortalBootstrap | 后台壳 bootstrap |
| PortalPreferences | identity 拥有的用户显示偏好 |
| PortalEvent | 最小化 SSE 事件；不含正文、密钥或用户 query |
| EventCursor | 游标 |
| PortalEventStream | SSE 流内部描述 |
| ReadinessReport | 入口 readiness |
| RedactedAuditRequest | 安全访问日志字段 |
| RequestOutcome | 请求结果 |
| UiFailure | 内部 Vue 状态；具体状态迁移见 workflows |
| SessionState | 内部 Vue 状态；具体状态迁移见 workflows |
| RouteDecision | 内部 Vue 状态；具体状态迁移见 workflows |
| ShellState | 内部 Vue 状态；具体状态迁移见 workflows |
| NavigationDecision | 内部 Vue 状态；具体状态迁移见 workflows |
| SourceCursor | 内部 Vue 状态；具体状态迁移见 workflows |
| PermissionChangedEvent | 内部 Vue 状态；具体状态迁移见 workflows |
| EditorSession | 内部 Vue 状态；具体状态迁移见 workflows |
| NavigationIntent | 内部 Vue 状态；具体状态迁移见 workflows |
| RouteLocation | 内部 Vue 状态；具体状态迁移见 workflows |
| BootstrapState | 内部 Vue 状态；具体状态迁移见 workflows |
| PreferencesPatch | 内部 Vue 状态；具体状态迁移见 workflows |
| SessionEndReason | 内部 Vue 状态；具体状态迁移见 workflows |
| ApiRequest | 内部 Vue 状态；具体状态迁移见 workflows |
| CompileRouteRegistryInput | 校验并编译公开与管理 API 的静态路由表的输入 |
| CompileRouteRegistryResult | 校验并编译公开与管理 API 的静态路由表的结果 |
| ResolveRouteInput | 将请求解析为唯一服务及相对路径的输入 |
| ResolveRouteResult | 将请求解析为唯一服务及相对路径的结果 |
| ValidateEnvelopeInput | 限制请求体、请求头与 operationId的输入 |
| ValidateEnvelopeResult | 限制请求体、请求头与 operationId的结果 |
| NormalizeForwardHeadersInput | 创建可信上游请求头的输入 |
| NormalizeForwardHeadersResult | 创建可信上游请求头的结果 |
| AssertSameOriginInput | 对写入及凭据请求检查来源的输入 |
| AssertSameOriginResult | 对写入及凭据请求检查来源的结果 |
| VerifyCSRFInput | 验证身份服务绑定会话的 CSRF token的输入 |
| VerifyCSRFResult | 验证身份服务绑定会话的 CSRF token的结果 |
| ResolveAuthContextInput | 取得经 identity 校验的当前权限上下文的输入 |
| ResolveAuthContextResult | 取得经 identity 校验的当前权限上下文的结果 |
| BuildInternalProofInput | 生成仅当前路由可使用的内部身份声明的输入 |
| BuildInternalProofResult | 生成仅当前路由可使用的内部身份声明的结果 |
| AuthorizeRouteInput | 入口按声明执行提前授权的输入 |
| AuthorizeRouteResult | 入口按声明执行提前授权的结果 |
| ForwardRequestInput | 发送一次有界请求到拥有服务的输入 |
| ForwardRequestResult | 发送一次有界请求到拥有服务的结果 |
| StreamResponseInput | 有背压地输出上游内容及 Range 响应的输入 |
| StreamResponseResult | 有背压地输出上游内容及 Range 响应的结果 |
| MapUpstreamFailureInput | 转换内部失败为标准公开错误的输入 |
| MapUpstreamFailureResult | 转换内部失败为标准公开错误的结果 |
| LoadBootstrapInput | 聚合当前站点与用户后台壳信息的输入 |
| LoadBootstrapResult | 聚合当前站点与用户后台壳信息的结果 |
| BuildCapabilitiesInput | 将权限及站点功能转为 UI capabilities的输入 |
| BuildCapabilitiesResult | 将权限及站点功能转为 UI capabilities的结果 |
| BuildNavigationInput | 构建稳定导航与可访问路由的输入 |
| BuildNavigationResult | 构建稳定导航与可访问路由的结果 |
| ReadPreferencesInput | 读取用户的显示偏好的输入 |
| ReadPreferencesResult | 读取用户的显示偏好的结果 |
| UpdatePreferencesInput | 更新主题、语言及默认后台页的输入 |
| UpdatePreferencesResult | 更新主题、语言及默认后台页的结果 |
| ValidatePreferenceRouteInput | 保证首页偏好对应当前可访问路由的输入 |
| ValidatePreferenceRouteResult | 保证首页偏好对应当前可访问路由的结果 |
| OpenEventStreamInput | 提供后台权限过滤 SSE的输入 |
| OpenEventStreamResult | 提供后台权限过滤 SSE的结果 |
| FilterPortalEventInput | 移除当前用户不可读取事件的输入 |
| FilterPortalEventResult | 移除当前用户不可读取事件的结果 |
| EncodeSSEFrameInput | 安全编码 SSE frame的输入 |
| EncodeSSEFrameResult | 安全编码 SSE frame的结果 |
| ResumeEventCursorInput | 验证重连游标的签名和上下文的输入 |
| ResumeEventCursorResult | 验证重连游标的签名和上下文的结果 |
| CollectReadinessInput | 输出入口自身及必要依赖可用性的输入 |
| CollectReadinessResult | 输出入口自身及必要依赖可用性的结果 |
| RedactRequestInput | 生成不会泄漏密码、token 或正文的访问日志字段的输入 |
| RedactRequestResult | 生成不会泄漏密码、token 或正文的访问日志字段的结果 |
| EmitAccessAuditInput | 可靠投递入口访问结果的输入 |
| EmitAccessAuditResult | 可靠投递入口访问结果的结果 |
| sendApiInput | 统一浏览器 API 请求与错误状态的输入 |
| sendApiResult | 统一浏览器 API 请求与错误状态的结果 |
| refreshSingleFlightInput | 对并发过期请求只刷新一次会话的输入 |
| refreshSingleFlightResult | 对并发过期请求只刷新一次会话的结果 |
| classifyApiErrorInput | 把业务错误映射到可恢复用户状态的输入 |
| classifyApiErrorResult | 把业务错误映射到可恢复用户状态的结果 |
| hydrateSessionInput | 将服务端 session 和权限装入界面状态的输入 |
| hydrateSessionResult | 将服务端 session 和权限装入界面状态的结果 |
| applyPermissionEventInput | 及时撤去被取消权限的 UI 与编辑能力的输入 |
| applyPermissionEventResult | 及时撤去被取消权限的 UI 与编辑能力的结果 |
| clearSessionInput | 清理退出或失效后的本地身份状态的输入 |
| clearSessionResult | 清理退出或失效后的本地身份状态的结果 |
| resolveAdminRouteInput | 解析后台地址并检查能力的输入 |
| resolveAdminRouteResult | 解析后台地址并检查能力的结果 |
| guardUnsavedNavigationInput | 在离开页面前保留未保存编辑的输入 |
| guardUnsavedNavigationResult | 在离开页面前保留未保存编辑的结果 |
| restoreReturnLocationInput | 安全恢复登录前的内部页面的输入 |
| restoreReturnLocationResult | 安全恢复登录前的内部页面的结果 |
| connectEventStreamInput | 连接并自动恢复后台 SSE的输入 |
| connectEventStreamResult | 连接并自动恢复后台 SSE的结果 |
| dispatchPortalEventInput | 把校验后的事件路由到相关界面状态的输入 |
| dispatchPortalEventResult | 把校验后的事件路由到相关界面状态的结果 |
| loadPreferencesInput | 加载并应用用户显示偏好的输入 |
| loadPreferencesResult | 加载并应用用户显示偏好的结果 |
| savePreferencesInput | 以 CAS 保存显示偏好的输入 |
| savePreferencesResult | 以 CAS 保存显示偏好的结果 |
| deriveShellStateInput | 计算初始化、登录、后台及降级壳状态的输入 |
| deriveShellStateResult | 计算初始化、登录、后台及降级壳状态的结果 |
| updateDocumentMetadataInput | 更新后台页面标题及无索引声明的输入 |
| updateDocumentMetadataResult | 更新后台页面标题及无索引声明的结果 |
| ReadRuntimeStateInput | 读取唯一权威运行状态的输入 |
| ReadRuntimeStateResult | 读取唯一权威运行状态的结果 |
| CheckEntranceGateInput | 对维护和恢复 epoch 实施入口屏障的输入 |
| CheckEntranceGateResult | 对维护和恢复 epoch 实施入口屏障的结果 |
| ReleaseBarrierInput | 按权威状态恢复业务入口的输入 |
| ReleaseBarrierResult | 按权威状态恢复业务入口的结果 |
| releaseBarrierRequest | 释放入口屏障请求体 |

恢复重建必须验证 manifest 哈希和私有事务水位。derived cache 可重建；权威记录缺失或 schema 未知应隔离并停止对外发布。

## 浏览器认证边界（本轮待VPS）

identity内部SessionTokens只在portal本次响应内存出现；本域不持久化会话、秘密、原始地址或敏感响应。四个签发操作的浏览器结果以OpenAPI的`x-browser-auth-adapters`为机器来源，字段为安全`session`及各操作允许的`account`、`nextRoute`。`session`只含sessionId、auth、mustChangePassword、expiresAt、csrfToken，accessToken与refreshToken不会进入浏览器JSON。CSRF只供当前页面内存与X-CSRF-Token，不写可读cookie或本地持久状态。

| Cookie | 属性 | 生命周期权威 |
|---|---|---|
| nw_sid | host-only、Path=/、Secure、HttpOnly、SameSite=Strict | Expires使用identity本次expiresAt |
| nw_refresh | host-only、Path=/、Secure、HttpOnly、SameSite=Strict | 浏览器session cookie；identity仍核服务端绝对期限 |

认证响应总界64KiB，cookie完整序列化每枚不超过4096字节。读取上游成功DTO、构造安全JSON与校验cookie全部在写头之前完成；认证成功/失败均no-store。失败不设置或清除旧cookie；敏感成功回执的CONFLICT要求重新登录。当前设备的权威撤销成功才以同属性MaxAge=-1和过去Expires删除两枚cookie；撤其他设备不清当前会话。

登录/首次注册的LoginNetworkScopes只由公共PrivateNetworkScopes从真实socket RemoteAddr和受信代理后缀派生，包含当前及上一UTC日HMAC。TrustedProxyCIDRs默认空，未受信socket完全忽略XFF；受信socket缺失、损坏或超界代理链失败关闭。原始地址仅公共函数调用期间驻留内存，不进入断言、日志或JSON。摘要按日变化只用于traffic-guard网络限速，不提供跨日持久身份。
