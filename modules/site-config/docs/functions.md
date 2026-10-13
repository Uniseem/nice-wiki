# 函数填空合同

每个条目与 [implementation.json](../contracts/implementation.json) 一一对应。签名中的领域类型见 types.json；标准 Problem 描述返回 error。未来新增任何私有助手也必须先登记契约。

## site-config.ReadSiteConfig

文件：`modules/site-config/internal/config_service.go`。

`func ReadSiteConfig(ctx RequestContext, input ReadSiteConfigInput) (ReadSiteConfigResult, error)`

读取后台站点配置。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadSiteConfigInput。读取后台站点配置的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadSiteConfigResult。读取后台站点配置的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核对site.read或注册内部调用服务。
2. 读取当前已提交配置一致快照。
3. 不返回初始化capability、秘密或控制库签名key。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：site.read 或 service:portal/public-site/content/media/localization/identity。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## site-config.UpdateSiteConfig

文件：`modules/site-config/internal/config_service.go`。

`func UpdateSiteConfig(ctx RequestContext, input UpdateSiteConfigInput) (UpdateSiteConfigResult, error)`

更新站点常规配置。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：UpdateSiteConfigInput。更新站点常规配置的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：UpdateSiteConfigResult。更新站点常规配置的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. CheckWriteEpoch与site.manage，baseLanguage/initialized/epoch禁止patch。
2. ValidateSiteConfig后WriteConfigRevision。
3. CommitConfigHead CAS提交权威文件；发ConfigChanged翻译与data git镜像。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有配置事务、operation receipt 与 outbox。

**授权**：site.manage；读取site.read，写入校验actor/siteId与活动dataEpoch。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.CheckWriteEpoch；site-config.ValidateSiteConfig；site-config.WriteConfigRevision；site-config.CommitConfigHead。

## site-config.ValidateSiteConfig

文件：`modules/site-config/internal/config_validator.go`。

`func ValidateSiteConfig(input ValidateSiteConfigInput) (ValidateSiteConfigResult, error)`

校验配置所有字段和关联约束。

**输入**

- input：ValidateSiteConfigInput。校验配置所有字段和关联约束的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateSiteConfigResult。校验配置所有字段和关联约束的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力。

**步骤**

1. 核SiteConfig完整schema；不含email/MFA/invitation等新增产品policy。
2. baseLanguage只初始化选择并且SiteConfig对象维持不可变source含义。
3. 品牌颜色/资源/hero/footer/banner完整验证，模板只title/site与安全URL。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_CAPABILITY：当前依赖不提供声明的必需能力；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：site-config.NormalizeBaseURL；site-config.ValidateLanguageTag；site-config.ValidateHomeLinks。

## site-config.NormalizeBaseURL

文件：`modules/site-config/internal/config_validator.go`。

`func NormalizeBaseURL(input NormalizeBaseURLInput) (NormalizeBaseURLResult, error)`

规范公开站点地址。

**输入**

- input：NormalizeBaseURLInput。规范公开站点地址的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：NormalizeBaseURLResult。规范公开站点地址的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 解析绝对URL。
2. 禁止userinfo、query、fragment、控制字符与非允许scheme。
3. host IDNA规范，去尾斜杠，保留部署允许的basePath。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## site-config.ValidateLanguageTag

文件：`modules/site-config/internal/language_service.go`。

`func ValidateLanguageTag(input ValidateLanguageTagInput) (ValidateLanguageTagResult, error)`

规范基准语言 tag。

**输入**

- input：ValidateLanguageTagInput。规范基准语言 tag的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateLanguageTagResult。规范基准语言 tag的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 按BCP47语法解析。
2. 规范语言/脚本/区域大小写并拒绝重复子tag。
3. 不访问模型、不根据翻译供应方限制基准语言。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## site-config.UpdateHome

文件：`modules/site-config/internal/home_service.go`。

`func UpdateHome(ctx RequestContext, input UpdateHomeInput) (UpdateHomeResult, error)`

配置首页介绍、卡片和精选文档。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：UpdateHomeInput。配置首页介绍、卡片和精选文档的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：UpdateHomeResult。配置首页介绍、卡片和精选文档的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 校验一屏hero全部属性、0–3按钮目标、背景种类/遮罩/文字颜色。
2. 验证ready资源并获取引用pin；视频必须poster且浏览器muted/autoplay/loop/playsinline，reduced-motion只poster。
3. CAS写不可变config文件和home revision，confirm pins、发翻译/历史镜像事件；失败release未提交pin。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有配置事务、operation receipt 与 outbox。

**授权**：site.manage；读取site.read，写入校验actor/siteId与活动dataEpoch。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.ValidateHomeLinks；media:acquireResourceReferences；media:confirmResourceReferences；media:releaseResourceReferences。

## site-config.ReadPublicHome

文件：`modules/site-config/internal/home_service.go`。

`func ReadPublicHome(ctx RequestContext, input ReadPublicHomeInput) (ReadPublicHomeResult, error)`

读取可公开的首页配置。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadPublicHomeInput。读取可公开的首页配置的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadPublicHomeResult。读取可公开的首页配置的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. initialized=true读取权威HomeConfig及公开站点品牌/footer/banner摘要。
2. 站内按钮存docId解析当前语言；不可见目标禁用，AI未配置禁用问问AI但hero正常。
3. public-site直接Go模板输出全屏hero，背景图预加载，减少动态效果仅视频poster，不返回private配置。

**错误**：NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：anonymous 或 service:public-site。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content:validatePublications。

## site-config.ValidateHomeLinks

文件：`modules/site-config/internal/home_validator.go`。

`func ValidateHomeLinks(ctx RequestContext, input ValidateHomeLinksInput) (ValidateHomeLinksResult, error)`

校验hero按钮/背景和品牌资源。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidateHomeLinksInput。验证首页目标与引用的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateHomeLinksResult。验证首页目标与引用的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. buttons最多3且ID不重复；document/qa/external判别字段严格校验。
2. 站内docId查询当前发布引用；外链允许http/https/mailto并拒绝javascript/userinfo。
3. 背景image/animation/video资源类型匹配，video有poster，遮罩0–100与textColor枚举。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：site.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读校验。

**依赖**：content:readPublishedRevision；media:readAsset。

## site-config.ReadRuntimeState

文件：`modules/site-config/internal/runtime_state_service.go`。

`func ReadRuntimeState(ctx RequestContext, input ReadRuntimeStateInput) (ReadRuntimeStateResult, error)`

读取独立控制库权威运行状态。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadRuntimeStateInput。读取独立控制库权威运行状态的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadRuntimeStateResult。读取独立控制库权威运行状态的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 只读control.db当前revision。
2. 校验dataEpoch与phase维护布尔不矛盾。
3. 返回签名可见状态，不公开恢复许可/信任密钥。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：service 或 system.manage；入口公开仅返回maintenance布尔经portal。

**并发**：控制库快照；不受业务恢复导入影响。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## site-config.TransitionRuntimeState

文件：`modules/site-config/internal/runtime_transition.go`。

`func TransitionRuntimeState(ctx RequestContext, input RuntimeTransition) (TransitionRuntimeStateResult, error)`

按CAS切换epoch与维护阶段。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RuntimeTransition。按CAS切换epoch与维护阶段的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：TransitionRuntimeStateResult。按CAS切换epoch与维护阶段的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 校验system.manage近期step-up或专用RecoveryAuthorization的action/audience/旧新epoch。
2. ValidateRuntimeTransition检查当前phase与expectedRevision。
3. control.db事务落持久迁移决定；complete_restore仅全部模块签名committed到目标epoch后才切dataEpoch并phase=running。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：独立control.db运行状态和决定日志。

**授权**：system.manage + superadmin批准；恢复协调由service:data-protection携专用恢复许可。

**并发**：控制库全局单写事务；不能从导入快照覆盖runtimeState。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.ValidateRuntimeTransition。

## site-config.ValidateRuntimeTransition

文件：`modules/site-config/internal/runtime_transition.go`。

`func ValidateRuntimeTransition(input ValidateRuntimeTransitionInput) (ValidateRuntimeTransitionResult, error)`

验证维护与恢复状态迁移矩阵。

**输入**

- input：ValidateRuntimeTransitionInput。验证维护与恢复状态迁移矩阵的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateRuntimeTransitionResult。验证维护与恢复状态迁移矩阵的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. running仅begin_backup/begin_restore；restore_draining仅begin_prepare/mark_blocked。
2. preparing仅在所有prepared回执后begin_activation；activating可complete_restore/begin_rollback/mark_blocked。
3. complete_restore要求15域epoch完全一致committed；complete_rollback要求全部rolled-back旧epoch；禁止未知、跳阶段或自行关维护。

**错误**：LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：纯函数，外层必须验证专用恢复许可。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## site-config.RegisterActivation

文件：`modules/site-config/internal/runtime_transition.go`。

`func RegisterActivation(ctx RequestContext, input RegisterActivationInput) (RegisterActivationResult, error)`

登记域恢复激活签名回执。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RegisterActivationInput。登记域恢复激活签名回执的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RegisterActivationResult。登记域恢复激活签名回执的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. mTLS caller必须与receipt.module一致且专用许可允许该模块。
2. 校验签名、stage、restoreId和target epoch；低状态不得覆盖高状态。
3. control.db事务upsert回执和revision；data-protection不能替任何域伪造ready。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；STALE_RESULT：事件/计算针对已过期源版本，拒绝覆盖新状态；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：control.db激活回执。

**授权**：service：对应域 + restore专用许可。

**并发**：(restoreId,module)唯一键；多个域可并发但revision CAS串行。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## site-config.CheckWriteEpoch

文件：`modules/site-config/internal/write_epoch.go`。

`func CheckWriteEpoch(ctx RequestContext, input CheckWriteEpochInput) (CheckWriteEpochResult, error)`

所有业务写入执行epoch与屏障检查。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CheckWriteEpochInput。所有业务写入执行epoch与屏障检查的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CheckWriteEpochResult。所有业务写入执行epoch与屏障检查的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 读control.db当前phase/dataEpoch。
2. 非running或本域barrier非released返回LOCKED。
3. epoch不等返回VERSION_MISMATCH；检查在业务txn登记与控制gate同互斥区。

**错误**：LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：私有业务写入入口。

**并发**：与屏障登记及事务计数共享gate。

**幂等**：只读判定。

**依赖**：无跨函数/跨域依赖。

## site-config.InitializeSite

文件：`modules/site-config/internal/initialization_service.go`。

`func InitializeSite(ctx RequestContext, input InitializeSiteInput) (InitializeSiteResult, error)`

无需AI完成首站点初始化。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：InitializeSiteInput。无需AI完成首站点初始化的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：InitializeSiteResult。无需AI完成首站点初始化的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 要求已注册唯一超管session；siteId取context，禁止匿名请求重开注册，不要初始化码。
2. 仅站点名和基准语言（默认zh-Hans），PrepareDefaultConfig；无渠道/密钥/模型/档位/探测/调用。
3. 持久commit决定后写配置权威文件、默认hero/footer，再initialized=true；中断由同operationId恢复。

**错误**：CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；FORBIDDEN：非唯一超管或未注册会话；retryable=false。

**副作用**：本服务初始化决定与权威站点配置文件；不创建第二owner。

**授权**：superadmin；已完成identity首注册，向导只有站点/基准语言两步。

**并发**：全站初始化唯一锁；相同operationId继续原阶段，不生成第二site。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.PrepareDefaultConfig；site-config.PersistInitializationDecision；site-config.ResumeInitialization。

## site-config.ReadInitializationStatus

文件：`modules/site-config/internal/initialization_service.go`。

`func ReadInitializationStatus(ctx RequestContext, input ReadInitializationStatusInput) (ReadInitializationStatusResult, error)`

读取安全初始化阶段。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadInitializationStatusInput。读取安全初始化阶段的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadInitializationStatusResult。读取安全初始化阶段的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 未初始化返回仅phase和安全错误。
2. 内部identity/site-config按operationId读取持久决定。
3. 不返回ownerEmail/password/capability。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：anonymous仅安全initialized/phase；superadmin或注册service读协调字段。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## site-config.ResumeInitialization

文件：`modules/site-config/internal/initialization_service.go`。

`func ResumeInitialization(ctx RequestContext, input ResumeInitializationInput) (ResumeInitializationResult, error)`

恢复崩溃或依赖断开的初始化阶段。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ResumeInitializationInput。恢复崩溃或依赖断开的初始化阶段的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ResumeInitializationResult。恢复崩溃或依赖断开的初始化阶段的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用。

**步骤**

1. 读取本服务初始化持久decision与config文件stage。
2. commit按journal幂等提交config/hero/footer并完成initialized；不调用identity owner准备。
3. 未知文件结果按hash与COMMITTEDjournal恢复；失败blocked，不要求保存密码。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false。

**副作用**：私有配置事务、operation receipt 与 outbox。

**授权**：service:site-config受控启动或site.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.PersistInitializationDecision；site-config.RecoverAnnouncementJournal。

## site-config.AbortInitialization

文件：`modules/site-config/internal/initialization_service.go`。

`func AbortInitialization(ctx RequestContext, input AbortInitializationInput) (AbortInitializationResult, error)`

取消尚未持久commit的初始化。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：AbortInitializationInput。取消尚未持久commit的初始化的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AbortInitializationResult。取消尚未持久commit的初始化的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. commit决定存在则不能abort，继续恢复完成。
2. 尚未提交则写abort决定并清本服务config临时stage。
3. 不删除唯一超管、不开第二次注册。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有配置事务、operation receipt 与 outbox。

**授权**：sealed初始化capability持有人或superadmin恢复许可。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.PersistInitializationDecision。

## site-config.PrepareDefaultConfig

文件：`modules/site-config/internal/initialization_defaults.go`。

`func PrepareDefaultConfig(input PrepareDefaultConfigInput) (PrepareDefaultConfigResult, error)`

确定性生成首次基础配置。

**输入**

- input：PrepareDefaultConfigInput。确定性生成首次基础配置的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PrepareDefaultConfigResult。确定性生成首次基础配置的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 设置baseLanguage默认zh-Hans、站名为mainTitle、backgroundText=Welcome To且translate=false。
2. 首页只hero一屏，0–3按钮；默认纯色背景/遮罩/浅深文字；默认footer源代码链接。
3. 不制造示例文档、假渠道或调用模型，initialized=false等待持久配置提交。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：site-config.ValidateLanguageTag。

## site-config.PersistInitializationDecision

文件：`modules/site-config/internal/initialization_repository.go`。

`func PersistInitializationDecision(ctx RequestContext, input PersistInitializationDecisionInput) (PersistInitializationDecisionResult, error)`

持久化不可反悔的初始化决定。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PersistInitializationDecisionInput。持久化不可反悔的初始化决定的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PersistInitializationDecisionResult。持久化不可反悔的初始化决定的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 锁operationId初始化record。
2. 要求prepare阶段完整才commit；commit后不能改abort。
3. 私有事务落决定与stage引用并fsync，再执行外部后续动作。

**错误**：CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有初始化决定日志。

**授权**：site-config 私有协调。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：同决定重复返回原版本；不同终局决定CONFLICT。

**依赖**：无跨函数/跨域依赖。

## site-config.ValidateAnnouncementBody

文件：`modules/site-config/internal/announcement_validator.go`。

`func ValidateAnnouncementBody(input ValidateAnnouncementBodyInput) (ValidateAnnouncementBodyResult, error)`

校验公告纯文本正文并规范序列化。

**输入**

- input：ValidateAnnouncementBodyInput。校验公告纯文本正文并规范序列化的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateAnnouncementBodyResult。校验公告纯文本正文并规范序列化的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性。

**步骤**

1. schemaVersion=1，正文只有text和linkTarget，严格拒绝unknown。
2. text会翻译；safe URL只http/https/mailto或docId，禁任意HTML。
3. canonicalJSON与稳定bannerId计算hash。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## site-config.WriteAnnouncementRevision

文件：`modules/site-config/internal/announcement_files.go`。

`func WriteAnnouncementRevision(ctx RequestContext, input WriteAnnouncementRevisionInput) (WriteAnnouncementRevisionResult, error)`

持久写不可变公告正文文件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：WriteAnnouncementRevisionInput。持久写不可变公告正文文件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：WriteAnnouncementRevisionResult。持久写不可变公告正文文件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 服务生成site/announcement/revision路径，不接受用户path。
2. 同文件系统写临时文件、fsync、原子rename并fsync父目录。
3. hash已存在同值返回；同revision异hashINTEGRITY_ERROR；失败不移动draft head。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有正文文件和PREPARED journal。

**授权**：site-config 私有正文提交。

**并发**：revisionId唯一；文件不覆盖。

**幂等**：同id同hash重用，异hash拒绝。

**依赖**：无跨函数/跨域依赖。

## site-config.CommitAnnouncementHead

文件：`modules/site-config/internal/announcement_files.go`。

`func CommitAnnouncementHead(ctx RequestContext, input CommitAnnouncementHeadInput) (CommitAnnouncementHeadResult, error)`

原子提交公告head与正文日志。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CommitAnnouncementHeadInput。原子提交公告head与正文日志的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CommitAnnouncementHeadResult。原子提交公告head与正文日志的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 确认PREPARED banner正文fsync/hash。
2. 在UpdateBanner提交事务CAS唯一currentBanner/head、COMMITTED journal和config revision。
3. 失败文件不公开，后续recover隔离。

**错误**：VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有配置事务、operation receipt 与 outbox。

**授权**：site-config 私有提交。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.RecoverAnnouncementJournal。

## site-config.RecoverAnnouncementJournal

文件：`modules/site-config/internal/announcement_files.go`。

`func RecoverAnnouncementJournal(ctx RequestContext, input RecoverAnnouncementJournalInput) (RecoverAnnouncementJournalResult, error)`

恢复正文文件与提交日志。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RecoverAnnouncementJournalInput。恢复正文文件与提交日志的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RecoverAnnouncementJournalResult。恢复正文文件与提交日志的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 启动停止公告写入。
2. COMMITTED逐一核对head/object/hash，缺失即隔离该公告并告警。
3. PREPARED无已提交head者按保留窗口隔离，不推定发布；恢复完成再开放。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：日志恢复与私有孤儿隔离。

**授权**：service:site-config启动控制。

**并发**：启动exclusive；运行时仅屏障safe执行。

**幂等**：反复恢复不会使PREPARED变COMMITTED。

**依赖**：无跨函数/跨域依赖。

## site-config.ResolveConfigOperation

文件：`modules/site-config/internal/operation_repository.go`。

`func ResolveConfigOperation(ctx RequestContext, input ResolveConfigOperationInput) (ResolveConfigOperationResult, error)`

查询配置幂等回执。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ResolveConfigOperationInput。查询配置幂等回执的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ResolveConfigOperationResult。查询配置幂等回执的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 按actor/site/op唯一键查询。
2. 异hashCONFLICT。
3. 返回已完成revision或in-progress；敏感初始化不持久ownerPassword。

**错误**：CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：私有操作事务。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## site-config.PublishConfigEvent

文件：`modules/site-config/internal/outbox_repository.go`。

`func PublishConfigEvent(ctx RequestContext, input PublishConfigEventInput) (PublishConfigEventResult, error)`

将配置事件与业务提交原子登记。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PublishConfigEventInput。将配置事件与业务提交原子登记的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PublishConfigEventResult。将配置事件与业务提交原子登记的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 校验最小payload。
2. 业务事务写eventId唯一outbox。
3. dispatcher经jobs投递、ack后标完成；低revision不可覆盖。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有配置事务、operation receipt 与 outbox。

**授权**：私有配置提交。

**并发**：与configRevision同一事务。

**幂等**：eventId唯一。

**依赖**：jobs:publishEvent。

## site-config.ReadConfigObject

文件：`modules/site-config/internal/object_reader.go`。

`func ReadConfigObject(ctx RequestContext, input ReadConfigObjectInput) (ReadConfigObjectResult, error)`

经协议读取本域正文或快照对象。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadConfigObjectInput。经协议读取本域正文或快照对象的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadConfigObjectResult。经协议读取本域正文或快照对象的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 只允许注册audience和owner=site-config。
2. 从服务生成对象索引解析私有路径，校验version/hash。
3. 打开只读lease stream；客户端无法指定OS路径。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：短期reader lease。

**授权**：service:public-site/data-protection或site.read。

**并发**：与对象删除fence协调。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## site-config.AcquireBarrier

文件：`modules/site-config/internal/barrier_service.go`。

`func AcquireBarrier(ctx RequestContext, input BarrierRequest) (BarrierStatus, error)`

取得可恢复的数据屏障。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：BarrierRequest。取得可恢复的数据屏障的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：BarrierStatus。取得可恢复的数据屏障的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 认证 data-protection 服务及 superadmin 委托；验证 epoch、operationId 与 deadline。
2. 私有事务登记屏障并拒绝新的业务写入；等待已登记提交完成。
3. 固定 revisionWatermark 并返回 safe；超时返回 draining/failed 且保持维护，不擅自释放。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection；备份superadmin委托，恢复专用RecoveryAuthorization严格restore/barrier/旧新epoch/audience/action/期限，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## site-config.ReleaseBarrier

文件：`modules/site-config/internal/barrier_service.go`。

`func ReleaseBarrier(ctx RequestContext, input ReleaseBarrierInput) (ReleaseBarrierResult, error)`

释放已协调完成的屏障。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReleaseBarrierInput。释放已协调完成的屏障的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReleaseBarrierResult。释放已协调完成的屏障的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变。

**步骤**

1. 查询持久屏障。
2. 验证维护退出决定、所有激活/回滚步骤已完成及 epoch 一致。
3. 原子改 released 并恢复业务接入。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false。

**副作用**：私有屏障状态写入。

**授权**：service:data-protection；备份superadmin委托，恢复专用RecoveryAuthorization严格restore/barrier/旧新epoch/audience/action/期限，本地control信任锚不被快照覆盖。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config:readRuntimeState。

## site-config.ExportSnapshot

文件：`modules/site-config/internal/snapshot_service.go`。

`func ExportSnapshot(ctx RequestContext, input SnapshotExportRequest) (SnapshotExportResult, error)`

导出屏障水位的一致快照。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotExportRequest。导出屏障水位的一致快照的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotExportResult。导出屏障水位的一致快照的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 验证本域 barrier=safe 和当前 dataEpoch；按 includeSecrets 策略封装敏感字段。
2. 流式复制私有数据库一致视图及所有权威文件到本服务只读对象；manifest 列 hash/size/schema。
3. fsync 完成后返回 SnapshotRef；失败只清理未完成临时对象，不修改活动数据。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection；备份superadmin委托，恢复专用RecoveryAuthorization严格restore/barrier/旧新epoch/audience/action/期限，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## site-config.PrepareSnapshot

文件：`modules/site-config/internal/snapshot_service.go`。

`func PrepareSnapshot(ctx RequestContext, input SnapshotPrepareRequest) (SnapshotStageResult, error)`

在私有 staging 区准备恢复。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotPrepareRequest。在私有 staging 区准备恢复的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotStageResult。在私有 staging 区准备恢复的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 要求 restore barrier=safe 和 site-config 维护状态。
2. 重新验证快照并复制到以 restoreId 命名的 staging 根，路径由服务生成。
3. 迁移到当前 schema，记录 stageId、目标 epoch 和旧活动 manifest；重复同摘要复用 stage。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection；备份superadmin委托，恢复专用RecoveryAuthorization严格restore/barrier/旧新epoch/audience/action/期限，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## site-config.ValidateSnapshot

文件：`modules/site-config/internal/snapshot_validator.go`。

`func ValidateSnapshot(ctx RequestContext, input SnapshotValidateRequest) (SnapshotValidation, error)`

验证快照可恢复性。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotValidateRequest。验证快照可恢复性的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotValidation。验证快照可恢复性的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 只经对象读取 API 获取本域快照，拒绝其他域目录访问。
2. 核对 owner、hash、schemaVersion、引用与状态机；检查目标 epoch。
3. 列出精确 field issues 和受支持迁移计划；不写活动数据。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：无。

**授权**：service:data-protection；备份superadmin委托，恢复专用RecoveryAuthorization严格restore/barrier/旧新epoch/audience/action/期限，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## site-config.CommitSnapshot

文件：`modules/site-config/internal/snapshot_repository.go`。

`func CommitSnapshot(ctx RequestContext, input SnapshotCommitRequest) (SnapshotApplyResult, error)`

激活已准备恢复数据。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotCommitRequest。激活已准备恢复数据的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotApplyResult。激活已准备恢复数据的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 要求维护、匹配 barrier/targetDataEpoch 与完整 stage 校验摘要。
2. 持久记录激活决定，原子切换活动 manifest/私有库连接到 stage 并 fsync。
3. 更新本域 epoch 和 commit receipt；保留旧 manifest 可回滚；进程崩溃按决定日志继续，不能部分激活。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection；备份superadmin委托，恢复专用RecoveryAuthorization严格restore/barrier/旧新epoch/audience/action/期限，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## site-config.RollbackSnapshot

文件：`modules/site-config/internal/snapshot_repository.go`。

`func RollbackSnapshot(ctx RequestContext, input SnapshotCommitRequest) (SnapshotApplyResult, error)`

按协调决定回滚恢复阶段。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SnapshotCommitRequest。按协调决定回滚恢复阶段的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SnapshotApplyResult。按协调决定回滚恢复阶段的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准。

**步骤**

1. 要求维护和同一 restore stage；仅接受协调者 rollback 决定。
2. 未激活 stage 删除；已激活则按持久旧 manifest 原子切回并验证 hash。
3. 记录 rolled-back 回执与原 epoch；无法回滚保持屏障并返回 INTEGRITY_ERROR。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；DEADLINE_EXCEEDED：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准；retryable=true。

**副作用**：本服务屏障/快照/恢复日志与私有对象写入。

**授权**：service:data-protection；备份superadmin委托，恢复专用RecoveryAuthorization严格restore/barrier/旧新epoch/audience/action/期限，本地control信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## site-config.UpdateBranding

文件：`modules/site-config/internal/branding_service.go`。

`func UpdateBranding(ctx RequestContext, input UpdateBrandingInput) (UpdateBrandingResult, error)`

更新站名/logo/图标/主题色/默认分享图。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：UpdateBrandingInput。更新站名/logo/图标/主题色/默认分享图的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：UpdateBrandingResult。更新站名/logo/图标/主题色/默认分享图的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核ready同站点资源与格式，权限site.manage。
2. acquire pins后生成不可变新config。
3. CommitConfigHead CAS并confirm资源，ConfigChanged触发翻译/历史镜像。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：不可变配置文件、COMMITTED journal、receipt/outbox。

**授权**：site.manage；所有业务写入CheckWriteEpoch。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.CommitConfigHead；media:acquireResourceReferences；media:confirmResourceReferences。

## site-config.UpdateFooter

文件：`modules/site-config/internal/footer_service.go`。

`func UpdateFooter(ctx RequestContext, input UpdateFooterInput) (UpdateFooterResult, error)`

更新页脚版权/自定义链接/ICP备案/源码链接。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：UpdateFooterInput。更新页脚版权/自定义链接/ICP备案/源码链接的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：UpdateFooterResult。更新页脚版权/自定义链接/ICP备案/源码链接的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 校验链接scheme与ICP号纯文本。
2. sourceCodeURL默认显示且可获得对应AGPL源代码。
3. 写config版本CAS提交并发翻译/历史事件。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：不可变配置文件、COMMITTED journal、receipt/outbox。

**授权**：site.manage；所有业务写入CheckWriteEpoch。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.CommitConfigHead；site-config.PublishConfigEvent。

## site-config.UpdateBanner

文件：`modules/site-config/internal/banner_service.go`。

`func UpdateBanner(ctx RequestContext, input UpdateBannerInput) (UpdateBannerResult, error)`

保存单条可关闭公告横幅。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：UpdateBannerInput。保存单条可关闭公告横幅的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：UpdateBannerResult。保存单条可关闭公告横幅的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核对text/link/time window；新条生成bannerId，同条编辑保留dismiss identity。
2. WriteAnnouncementRevision后同事务换currentBanner/config revision/visibilityRevision。
3. 登记翻译/历史镜像事件，按时间窗公开；浏览器localStorage关闭同一条不再显示。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：不可变配置文件、COMMITTED journal、receipt/outbox。

**授权**：site.manage；所有业务写入CheckWriteEpoch。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.ValidateAnnouncementBody；site-config.WriteAnnouncementRevision；site-config.CommitAnnouncementHead；site-config.PublishConfigEvent。

## site-config.WriteConfigRevision

文件：`modules/site-config/internal/config_files.go`。

`func WriteConfigRevision(ctx RequestContext, input WriteConfigRevisionInput) (WriteConfigRevisionResult, error)`

写权威版本化站点设置文件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：WriteConfigRevisionInput。写权威版本化站点设置文件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：WriteConfigRevisionResult。写权威版本化站点设置文件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 规范JSON、核sha与schema。
2. 私有settings/site/revision.json同FS临时写fsync/rename/fsyncparent。
3. PREPARED journal记hash，未COMMITTED不得读；不纳入secret/账号/代理key。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：不可变配置文件、COMMITTED journal、receipt/outbox。

**授权**：私有config写入。

**并发**：revision文件不可覆盖。

**幂等**：同revision同hash重用，异hash拒绝。

**依赖**：无跨函数/跨域依赖。

## site-config.CommitConfigHead

文件：`modules/site-config/internal/config_files.go`。

`func CommitConfigHead(ctx RequestContext, input CommitConfigHeadInput) (CommitConfigHeadResult, error)`

提交配置权威head与日志。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CommitConfigHeadInput。提交配置权威head与日志的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CommitConfigHeadResult。提交配置权威head与日志的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 检查PREPARED文件hash和fsync。
2. 私有事务CAS head、COMMITTEDjournal、receipt/outbox。
3. 由data-protection经对象API读取镜像写git；任何失败不读取其他域目录。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：不可变配置文件、COMMITTED journal、receipt/outbox。

**授权**：私有config提交。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.WriteConfigRevision；site-config.PublishConfigEvent。

## site-config.ReadConfigHistory

文件：`modules/site-config/internal/config_history.go`。

`func ReadConfigHistory(ctx RequestContext, input ReadConfigHistoryInput) (ReadConfigHistoryResult, error)`

读取可回滚的站点设置历史。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadConfigHistoryInput。读取可回滚的站点设置历史的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadConfigHistoryResult。读取可回滚的站点设置历史的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. site.manage检查。
2. 从本域COMMITTED journal稳定分页。
3. 返回不可变文件和作者元数据，data git由data-protection镜像。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：site.manage。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## site-config.RestoreConfigRevision

文件：`modules/site-config/internal/config_history.go`。

`func RestoreConfigRevision(ctx RequestContext, input RestoreConfigRevisionInput) (RestoreConfigRevisionResult, error)`

回滚首页/公告/品牌/页脚历史。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RestoreConfigRevisionInput。回滚首页/公告/品牌/页脚历史的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RestoreConfigRevisionResult。回滚首页/公告/品牌/页脚历史的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核历史config hash与当前epoch，保留不可变siteId/baseLanguage/initialized控制字段。
2. 重新核媒体存在，已手动清理文件明确列缺失且拒绝无确认回滚。
3. 生成新config版本而不改旧历史，CAS提交并发翻译/历史镜像。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：不可变配置文件、COMMITTED journal、receipt/outbox。

**授权**：site.manage；所有业务写入CheckWriteEpoch。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config.ReadConfigHistory；site-config.WriteConfigRevision；site-config.CommitConfigHead；media:readAsset。

## site-config.ListResourceReferences

文件：`modules/site-config/internal/resource_reference_reader.go`。

`func ListResourceReferences(ctx RequestContext, input ListResourceReferencesInput) (ListResourceReferencesResult, error)`

向媒体返回站点设置权威资源引用快照。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ListResourceReferencesInput。向媒体返回站点设置权威资源引用快照的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ListResourceReferencesResult。向媒体返回站点设置权威资源引用快照的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 从COMMITTED config head与保留history files扫描所有完整asset refs，包含hero背景/poster及branding，不能只扫当前home。
2. 同水位分类current/history，sourceService=site-config、sourceRevisionId=config不可变文件。
3. 签名cursor绑定site/watermark，分页未完全media不可解释为零引用；不扫描任何其他域目录。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：service:media/data-protection或site.manage。

**并发**：不可变COMMITTED文件一致水位读取。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M03-F901 main

文件 `modules/site-config/cmd/site-config/main.go`。独立进程可执行入口

```go
func main()
```

输入：

无参数/返回值。

输出：

无参数/返回值。

步骤：

1. 读取--config/--data-dir/--listen及环境覆盖，严格解析ServiceConfig；配置错误安全stderr后exit2
2. signal.NotifyContext注册SIGINT/SIGTERM；调用BuildApplication，失败exit1且只输出errorCode
3. 调用Serve；正常关闭或signal后以30秒context调用Shutdown；安全退出0，无法安全关闭exit1且保留journal

错误：

- `VALIDATION`：配置未知字段/越界目录/非法端点；HTTP 400；retryable=false
- `INTERNAL`：装配或HTTP运行失败；HTTP 500；retryable=false

副作用：打开本域资源、监听HTTP；关闭按持久安全点。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：一个进程一个Application；OS信号处理只触发一次drain

幂等：进程重启先journal恢复；不重放已付费任务

依赖：BuildApplication、Serve、Shutdown

### M03-F902 BuildApplication

文件 `modules/site-config/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

```go
func BuildApplication(ctx context.Context, config ServiceConfig) (*Application, error)
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `config: ServiceConfig`：已严格解析配置；约束：module固定本域；secret只ref；依赖只HTTP

输出：

- `application: *Application`：只返回完整装配后的内部容器；约束：失败返回nil，已打开资源逆序关闭

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 验证module/listen/data目录/secretRef；建立本域控制区和业务数据区；portal不虚构业务DB
2. 打开本域DB并运行已有schema迁移/文件journal恢复；保留restore控制记录/barrier/trust-anchor
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；已commit恢复未全完成时仅开放控制管理；注册本域主动claim worker并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源

错误：

- `VALIDATION`：配置/缺路由绑定；HTTP 400；retryable=false
- `INTEGRITY_ERROR`：迁移或journal校验失败；HTTP 422；retryable=false
- `DEPENDENCY_UNAVAILABLE`：基础关键依赖/控制信任锚不可用；HTTP 503；retryable=true

副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法

幂等：启动重入按journal和epoch恢复，不新建paid operation

依赖：本域implementation全部x-handler、本域journal恢复函数、site-config.readRuntimeState（本域site-config读取自身权威）

### M03-F903 Serve

文件 `modules/site-config/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

```go
func Serve(ctx context.Context, app *Application) error
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 仅HTTP启动net/http.Server，ReadHeaderTimeout=5s、MaxHeaderBytes=64KiB；写deadline由各路由决定，SSE不用统一短WriteTimeout
2. 启动本域worker循环，匿名Health/Ready优先可访问；普通路由执行权限/epoch/maintenance门禁，SSE响应X-Accel-Buffering:no
3. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

错误：

- `INTERNAL`：监听地址冲突或HTTP server不可恢复错误；HTTP 500；retryable=false
- `CONFLICT`：同Application重复Serve启动；HTTP 409；retryable=false

副作用：监听本域HTTP端口、服务请求和本域worker。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：只有一Serve调用；每请求取消独立，worker只本域

幂等：重复Serve返回CONFLICT语义，不开第二监听

依赖：BuildApplication、Shutdown、本域OpenAPI路由

### M03-F904 Shutdown

文件 `modules/site-config/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

```go
func Shutdown(ctx context.Context, app *Application) error
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. CAS lifecycle ready→draining，拒新写/claim/paid步骤并关闭SSE接受新订阅
2. 取消本域worker的新步骤；等待已有DB事务、文件journal、模型可结算阶段安全确认；不能因超时假定供应方免费或删除paid refs
3. http.Server.Shutdown等在途响应；flush本域outbox/journal、close DB与文件句柄；正常lifecycle closed
4. deadline未安全返回DEADLINE_EXCEEDED并保留完整journal/未决费用引用，启动恢复收敛；不清掉活跃控制记录

错误：

- `DEADLINE_EXCEEDED`：30秒期限仍有未安全在途步骤；HTTP 504；retryable=true
- `INTERNAL`：flush/close失败；HTTP 500；retryable=false

副作用：本域drain、资源关闭、durable恢复日志。

授权：{"permission": "service", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：sync.Once启动drain；其他调用等待同一次结果

幂等：重复Shutdown返回同一次关闭结果；已closed成功

依赖：本域barrier安全点函数、本域journal/outbox flush

### M03-F905 Health

文件 `modules/site-config/internal/transport/server.go`。GET /healthz普通HTTP存活检查

```go
func Health(ctx context.Context, app *Application) (HealthStatus, error)
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `status: HealthStatus`：公共liveness安全字段；约束：仅module/version/status/capabilities

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 仅读本进程lifecycle与HTTP循环是否存活；不查询远程依赖/模型/数据库
2. 存活且未closed返回200 HealthStatus；closed或server不可用返回503；不泄漏路径、端点或secret

错误：

- `DEPENDENCY_UNAVAILABLE`：进程已closed或服务循环不可用；HTTP 503；retryable=true

副作用：无。

授权：{"permission": "anonymous", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：无锁或原子读取lifecycle；不进行外部调用

幂等：GET只读重复安全

依赖：无

### M03-F906 Ready

文件 `modules/site-config/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

```go
func Ready(ctx context.Context, app *Application) (ServiceReadiness, error)
```

输入：

- `ctx: context.Context`：进程/请求取消和deadline；约束：Go标准库；不携敏感正文到日志

- `app: *Application`：本域已装配应用句柄；约束：x-internal；HTTP不能提供或序列化

输出：

- `readiness: ServiceReadiness`：本域启动/恢复就绪与公开门禁状态；约束：ready不等于维护已解除，publicRequestsAllowed必须显式false

- `error: error`：成功nil；失败公共Problem或进程安全退出原因；约束：不把secret/路径凭证/IP写detail

步骤：

1. 确认配置/本域schema/journal/control信任锚完整并在lifecycle ready或维护控制ready
2. 已初始化时核activeDataEpoch等于runtime目标代次；关键运行权威不可达或旧epoch返回503；site-config本域读取自身权威
3. 维护中本域新epoch已激活可返回200 control-ready但publicRequestsAllowed=false，使全域激活检查不会和维护门禁互相等待；所有域完成后runtime解除维护才开放
4. 未初始化可仅带初始化能力ready，activeDataEpoch=null；无AI渠道不使基础内容能力失败，能力缺项显式呈现

错误：

- `DEPENDENCY_UNAVAILABLE`：关键依赖/本域journal未恢复/epoch未匹配；HTTP 503；retryable=true

副作用：无。

授权：{"permission": "anonymous", "rule": "进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型"}

并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测

幂等：GET只读重复安全

依赖：site-config.readRuntimeState（site-config本域自身读取）
<!-- service-bootstrap:end -->
