# site-config 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 92 个函数：约束 33 个，参考 59 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## site-config.ReadSiteConfig

- 级别：约束
- 文件：`modules/site-config/internal/config_service.go`
- 作用：读取后台站点配置

```go
func ReadSiteConfig(ctx context.Context, op OperationContext, input ReadSiteConfigInput) (ReadSiteConfigResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadSiteConfigInput` | 读取后台站点配置的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadSiteConfigResult` | 读取后台站点配置的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 要求同站点已登录admin具site.read/superadmin，或已登记portal/public-site/content/media/localization/identity服务主体；服务身份仅在actor.kind=service时使用内部读取分支，不能绕过委托用户权限
2. 私有读快照要求initialized=true与dataEpoch一致；核对SQL当前索引、COMMITTED、head和权威文件hash/schema，返回完整已提交配置，准备材料不替代文件
3. 不返回初始化capability、秘密或控制库签名key；读取与本域文件提交互斥，不能在COMMITTED/SQL未补齐时返回陈旧配置

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：权威文件/COMMITTED/SQL或原始材料不一致（不可重试）
- `VERSION_MISMATCH`：活动epoch或expectedRevision已变化（不可重试）

- 副作用：无
- 授权：site.read 或 service:portal/public-site/content/media/localization/identity
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`site-config.authorizeConfigRead`

## site-config.UpdateSiteConfig

- 级别：约束
- 文件：`modules/site-config/internal/config_service.go`
- 作用：更新站点常规配置

```go
func UpdateSiteConfig(ctx context.Context, op OperationContext, input UpdateSiteConfigInput) (UpdateSiteConfigResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdateSiteConfigInput` | 更新站点常规配置的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateSiteConfigResult` | 更新站点常规配置的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 要求同站点已登录admin具site.manage/superadmin；只更新HTTP明确列出的公开地址/SEO/资源上限/安全设置，baseLanguage/siteId/initialized/home/品牌/footer/banner不接受patch
2. 以kind=settings及原始严格输入（含expectedRevision）的规范JSON计算稳定业务hash，先查actor/site/op回执；已提交重试从原revision权威文件返回原配置，不受新的URL/本地HTTP能力策略影响，不重写head/CAS/outbox
3. HTTP维护白名单仅允许到达旧成功回执的只读返回；未命中已完成回执时，任何候选SQL读取/写入和PREPARED文件IO之前先CheckWriteEpoch拒绝维护/错epoch，新操作和未完成操作都不在维护期写候选；Commit内部CAS/epoch门禁继续保留
4. 只有新操作/未完成操作应用当前受信allowLocalHTTP进行NormalizeBaseURL和纯ValidateSiteConfig；读取旧候选时还核对候选字段与本次规范输入一致。候选材料SHA256独立保存规范后的完整配置，不代替原始业务摘要
5. 首次操作在epoch/initialized/site/expectedRevision事务核对后持久完整候选配置规范JSON/hash、稳定revisionId和原始回执；该JSON仅未提交恢复材料，保留最初updatedAt，同operation重试不重算。跨进程以SQLite写锁和现有文件COMMITTED CAS占有目标水位
6. 纯ValidateSiteConfig通过后复用WriteConfigRevision/CommitConfigHead；COMMITTED前可失败，落盘后只补完既有对象/head/SQL回执/outbox；返回成功配置必须来自已提交权威对象。草稿和设置保存不触发AI

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：权威文件/COMMITTED/SQL或原始材料不一致（不可重试）
- `NOT_FOUND`：站点未完成初始化或对象未提交（不可重试）
- `LOCKED`：维护或恢复屏障拒绝新写入（不可重试）

- 副作用：私有配置事务、operation receipt 与 outbox
- 授权：site.manage；读取site.read，写入校验actor/siteId与活动dataEpoch
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.CheckWriteEpoch`、`site-config.ValidateSiteConfig`、`site-config.WriteConfigRevision`、`site-config.CommitConfigHead`、`site-config.ResolveConfigOperation`

## site-config.readConfigAtWatermark

- 级别：参考
- 文件：`modules/site-config/internal/config_service.go`
- 作用：共用head/COMMITTED/SQL一致的正规权威文件读取，不修改调用方水位授权

```go
func (s *ActiveStore) readConfigAtWatermark(ctx context.Context, op OperationContext, tx *sql.Tx, siteID string, revision gen.Revision, revisionID string) (gen.SiteConfig, configJournalRecord, error)
```

**输入**

无

**输出**

无

**步骤**

1. 共用head/COMMITTED/SQL一致的正规权威文件读取，不修改调用方水位授权

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.loadPublicHomeConfig

- 级别：参考
- 文件：`modules/site-config/internal/config_service.go`
- 作用：独立公开首页只读水位入口，仅portal/public-site，既有私有readConfigWatermark授权原封保留

```go
func (s *ActiveStore) loadPublicHomeConfig(ctx context.Context, op OperationContext) (gen.SiteConfig, error)
```

**输入**

无

**输出**

无

**步骤**

1. 独立公开首页只读水位入口，仅portal/public-site，既有私有readConfigWatermark授权原封保留

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.readConfigAtWatermark`

## site-config.authorizeConfigRead

- 级别：参考
- 文件：`modules/site-config/internal/config_service.go`
- 作用：落实配置读取和对象读取不同服务白名单

```go
func authorizeConfigRead(op OperationContext, object bool) error
```

**输入**

无

**输出**

无

**步骤**

1. 已核纯服务读取SiteConfig允许portal/public-site/content/media/localization/identity/knowledge/model-gateway/memory/qa/analytics/traffic-guard/jobs/data-protection，仅配置公开字段，不给写权限。
2. object分支仍只public-site/data-protection；管理员仍当前site.read或superadmin，不扩大对象读取或管理写权限。

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：权威文件/COMMITTED/SQL或原始材料不一致（不可重试）
- `VERSION_MISMATCH`：活动epoch或expectedRevision已变化（不可重试）

- 副作用：无
- 授权：site.read 或 service:portal/public-site/content/media/localization/identity
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## site-config.ValidateSiteConfig

- 级别：参考
- 文件：`modules/site-config/internal/config_validator.go`
- 作用：校验配置所有字段和关联约束

```go
func ValidateSiteConfig(input ValidateSiteConfigInput) (ValidateSiteConfigResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidateSiteConfigInput` | 校验配置所有字段和关联约束的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateSiteConfigResult` | 校验配置所有字段和关联约束的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_CAPABILITY: 当前依赖不提供声明的必需能力 |

**步骤**

1. 结构/required/范围由HTTP严格生成schema或持久IO边界校验，本函数不重复schema解析；只校验配置字段间语义、规范语言、合法模板和安全URL
2. titleTemplate只允许title/site两个占位符；基准语言的不可变性由只允许初始化的更新输入和聚合写入保证，不引入email/MFA/invitation产品policy
3. 校验hero/footer ID唯一、安全外链、公告同站点与时间窗等纯语义；纯函数不访问content/media，不替未改变引用增加远端查询。改变引用由UpdateHome/品牌等用例校验当前发布/ready并持久pin恢复

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`site-config.NormalizeBaseURL`、`site-config.ValidateLanguageTag`、`site-config.validateHomeStructure`

## site-config.NormalizeBaseURL

- 级别：参考
- 文件：`modules/site-config/internal/config_validator.go`
- 作用：规范公开站点地址

```go
func NormalizeBaseURL(input NormalizeBaseURLInput) (NormalizeBaseURLResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `NormalizeBaseURLInput` | 规范公开站点地址的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `NormalizeBaseURLResult` | 规范公开站点地址的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 将raw URL与受信allowLocalHTTP传给platform.NormalizePublicURL；不手写IDNA或复制公共URL机制
2. 统一绝对HTTPS、IDNA、IPv6/有效端口、控制字符、userinfo/query/fragment拒绝；HTTP仅受信能力明确允许且loopback；保留合法部署basePath并去尾斜杠

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`platform.NormalizePublicURL`

## site-config.validateConfigWebLink

- 级别：参考
- 文件：`modules/site-config/internal/config_validator.go`
- 作用：共享配置外链纯规则，拒绝脚本协议与凭据和控制字符

```go
func validateConfigWebLink(raw string, allowMail bool) error
```

**输入**

无

**输出**

无

**步骤**

1. 允许http/https完整主机；按钮可显式允许mailto，禁止userinfo与控制字符，不访问网络

**错误**

- 无

- 副作用：无
- 授权：由调用链建立
- 并发：纯函数
- 幂等：确定性
- 依赖：无

## site-config.ValidateLanguageTag

- 级别：参考
- 文件：`modules/site-config/internal/language_service.go`
- 作用：规范基准语言 tag

```go
func ValidateLanguageTag(input ValidateLanguageTagInput) (ValidateLanguageTagResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidateLanguageTagInput` | 规范基准语言 tag的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateLanguageTagResult` | 规范基准语言 tag的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 按BCP47语法解析
2. 规范语言/脚本/区域大小写并拒绝重复子tag
3. 不访问模型、不根据翻译供应方限制基准语言

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## site-config.UpdateHome

- 级别：约束
- 文件：`modules/site-config/internal/home_service.go`
- 作用：更新仅一屏hero的首页配置

```go
func UpdateHome(ctx context.Context, op OperationContext, input UpdateHomeInput) (UpdateHomeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdateHomeInput` | 配置一屏hero首页的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateHomeResult` | 配置一屏hero首页的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 同站点site.manage；以原严格输入CanonicalJSON+kind生成稳定业务摘要；原成功只读原正式对象，维护仍可读且不重查当前依赖
2. 新/未完成先CheckWriteEpoch；config及home双CAS，验证纯hero语义、公开文档目标、ready且完整版本匹配的资源
3. 固定候选与排序完整refs/acquire intent同事务；空集合也走pin，未知pinId按sourceOperationId/digest找回同一原pin，不延长expiry
4. 持久pin后复用WriteConfigRevision/CommitConfigHead，COMMITTED前与终止共享SQL写锁，阻止已abort的迟到commit
5. home confirm_pending与config SQL水位/原回执/平台outbox同事务；COMMITTED成功即保存，外部confirm网络结果不改报未提交
6. 已有COMMITTED但SQL未完的原操作只补原决定，不能被新可见性/ready/expiry策略拒绝；明确未提交才持久终止并由后台release

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有配置事务、operation receipt 与 outbox
- 授权：site.manage；读取site.read，写入校验actor/siteId与活动dataEpoch
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.ValidateHomeLinks`、`media:acquireResourceReferences`、`media:confirmResourceReferences`、`media:releaseResourceReferences`、`site-config.ResolveConfigOperation`、`platform.CanonicalJSON`、`site-config.CheckWriteEpoch`、`site-config.WriteConfigRevision`、`site-config.CommitConfigHead`、`site-config.compareHomeInput`、`site-config.readHomeReferenceIntent`、`site-config.committedHomeReference`、`site-config.homeTerminatedError`、`site-config.acquireHomePin`、`site-config.finishHomeFailure`、`site-config.saveHomeReferenceIntent`

## site-config.ReadPublicHome

- 级别：约束
- 文件：`modules/site-config/internal/home_service.go`
- 作用：读取可公开的首页配置

```go
func ReadPublicHome(ctx context.Context, op OperationContext, input ReadPublicHomeInput) (ReadPublicHomeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadPublicHomeInput` | 读取可公开的首页配置的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadPublicHomeResult` | 读取可公开的首页配置的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. initialized=true读取head/COMMITTED/SQL一致权威；language缺省baseLanguage并规范BCP47
2. 显式公开siteName/baseLanguage/language/configRevision、PublicHome、四个branding字段及既有FooterConfig，banner仅enabled且当前窗口，其他null；不返回security/limits或原hero私有target
3. 按稳定buttonId保留全部0–3按钮，document调用public-site.resolvePublishedLink，目标/锚点/语言不可用明确disabled且href/publication为null；依赖未知只禁用该按钮
4. qa调用免费readPublicCapability，不探测模型；无配置/未知明确禁用，available才按可信publicBaseURL.basePath及请求language构造/ask
5. 外链沿既有安全规则，href不提前HTML转义；public-site模板输出点统一转义并保留视频poster/reduced-motion语义

**错误**

- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：anonymous 或 service:public-site
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`public-site:resolvePublishedLink`、`qa:readPublicCapability`、`site-config.ValidateLanguageTag`、`site-config.validateHomeStructure`、`site-config.loadPublicHomeConfig`、`site-config.homeDependencies`、`site-config.decodeHomeTarget`、`site-config.resolveHomeDocument`、`site-config.disablePublicHomeButton`、`site-config.readHomeQACapability`、`site-config.validateConfigWebLink`

## site-config.ReadHomeReferenceDecision

- 级别：约束
- 文件：`modules/site-config/internal/home_service.go`
- 作用：媒体读取首页引用来源决定

```go
func ReadHomeReferenceDecision(ctx context.Context, op OperationContext, input ReadHomeReferenceDecisionInput) (ReadHomeReferenceDecisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消和期限 | 只传下游IO |
| `op` | `OperationContext` | 入口或受信后台建立的上下文 | 当前活动dataEpoch，非浏览器自造 |
| `input` | `ReadHomeReferenceDecisionInput` | 已校验输入 | 边界严格schema一次 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadHomeReferenceDecisionResult` | 成功结果 | 无error才有效 |
| `error` | `Error` | 安全标准错误 | 不含私有配置/地址/secret |

**步骤**

1. 仅media纯服务，核已打开active与op当前epoch
2. 固定intent源object/revision/version/digest先校验，COMMITTED文件若存在必须完整核对并优先报告committed
3. 无COMMITTED时只从持久aborted/rejected终局报告；其他报告pending，不存在404/未知关闭

**错误**

- `FORBIDDEN`：调用方不允许（不可重试）
- `NOT_FOUND`：来源操作不存在（不可重试）
- `VERSION_MISMATCH`：非活动epoch（不可重试）
- `INTEGRITY_ERROR`：决定或材料不一致（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威不可用（可重试）

- 副作用：无
- 授权：permission：service；rule：仅media纯服务
- 并发：与CommitConfigHead共享configMu及SQLite写锁；COMMITTED不可回退
- 幂等：固定sourceOperationId/完整引用digest与源修订；同决定幂等，未知保留
- 依赖：`site-config.readHomeReferenceIntent`、`site-config.committedHomeReference`

## site-config.RecoverHomeReferences

- 级别：约束
- 文件：`modules/site-config/internal/home_service.go`
- 作用：按明确决定有界收敛首页媒体pin

```go
func RecoverHomeReferences(ctx context.Context, op OperationContext, input RecoverHomeReferencesInput) (RecoverHomeReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消和期限 | 只传下游IO |
| `op` | `OperationContext` | 入口或受信后台建立的上下文 | 当前活动dataEpoch，非浏览器自造 |
| `input` | `RecoverHomeReferencesInput` | 已校验输入 | 边界严格schema一次 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverHomeReferencesResult` | 成功结果 | 无error才有效 |
| `error` | `Error` | 安全标准错误 | 不含私有配置/地址/secret |

**步骤**

1. 仅完整Application装配后由公共RunRecoveryLoop单批最多100项，构造期只做本地RecoverHomeReferenceState；普通候选绝不自动发布
2. COMMITTED正规文件/哈希优先；未提交只在共享写锁下持久终止，防迟到commit；未知/损坏不release
3. committed仅confirm已持久pin；aborted/rejected直接Release完整来源和可空pinId，原子封存无pin来源，绝不为终止决定重新Acquire
4. confirm低源版本仅收敛history，当前水位不倒退；阶段和完成态均持久，业务保存结果不取决于外部确认
5. 每次RPC前后复用公共写窗口核epoch/本域control；维护暂停，显式可重试依赖错误保留待续办，永久错误上报

**错误**

- `FORBIDDEN`：调用方不允许（不可重试）
- `NOT_FOUND`：来源操作不存在（不可重试）
- `VERSION_MISMATCH`：非活动epoch（不可重试）
- `INTEGRITY_ERROR`：决定或材料不一致（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威不可用（可重试）

- 副作用：持久资源pin收敛决定/回执，跨域调用media
- 授权：permission：service；rule：仅site-config受信后台；不复活旧用户proof；每次RPC与本域写入前后核当前epoch及安全写窗口
- 并发：与CommitConfigHead共享configMu及SQLite写锁；COMMITTED不可回退
- 幂等：固定sourceOperationId/完整引用digest与源修订；同决定幂等，未知保留
- 依赖：`media:confirmResourceReferences`、`media:releaseResourceReferences`、`site-config.CheckWriteEpoch`、`site-config.decideHomeTermination`、`site-config.settleHomeReference`

## site-config.ConfigureHome

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：仅从受信启动注入媒体/公开链接/免费QA客户端

```go
func (s *ActiveStore) ConfigureHome(runtime HomeRuntime)
```

**输入**

无

**输出**

无

**步骤**

1. 仅从受信启动注入媒体/公开链接/免费QA客户端

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.homeDependencies

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：读取进程持有的受信首页依赖

```go
func (s *ActiveStore) homeDependencies() HomeRuntime
```

**输入**

无

**输出**

无

**步骤**

1. 读取进程持有的受信首页依赖

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.homeSourceObjectID

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：固定HashBytes(site-config.home.v1|siteId)的64hex来源标识

```go
func homeSourceObjectID(siteID string) string
```

**输入**

无

**输出**

无

**步骤**

1. 固定HashBytes(site-config.home.v1|siteId)的64hex来源标识

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.homeReferenceMaterial

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：完整AssetReference按referenceId/asset/version/hash排序并公共CanonicalJSON计算digest；空集固定[]

```go
func homeReferenceMaterial(config gen.SiteConfig, revisionID string) ([]homeResourceReference, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整AssetReference按referenceId/asset/version/hash排序并公共CanonicalJSON计算digest；空集固定[]

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.validateHomeStructure`、`site-config.homeSourceObjectID`、`platform.CanonicalJSON`

## site-config.saveHomeReferenceIntent

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：同候选SQL事务保存固定来源与acquire intent，不能在RPC之后补写

```go
func saveHomeReferenceIntent(ctx context.Context, op OperationContext, tx *sql.Tx, config gen.SiteConfig, revisionID string) error
```

**输入**

无

**输出**

无

**步骤**

1. 同候选SQL事务保存固定来源与acquire intent，不能在RPC之后补写

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.homeReferenceMaterial`、`platform.CanonicalJSON`

## site-config.readHomeReferenceIntent

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：有界读取本域refs/pin材料并核hash、source、候选kind及pin完整性

```go
func (s *ActiveStore) readHomeReferenceIntent(ctx context.Context, q ReadTransaction, operationID string) (homeReferenceIntent, error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界读取本域refs/pin材料并核hash、source、候选kind及pin完整性；home与branding复用同一决定表，来源对象按种类区分

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.validateHomePin`、`site-config.homeSourceObjectID`、`platform.CanonicalJSON`

## site-config.validateHomePin

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：核媒体pin绑定的完整资源版本集合、pin身份与期限材料

```go
func validateHomePin(intent homeReferenceIntent, pin homeReferencePin) error
```

**输入**

无

**输出**

无

**步骤**

1. 核媒体pin绑定的完整资源版本集合、pin身份与期限材料

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.committedHomeReference

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：经集中正规文件枚举判断明确COMMITTED是否存在，IO未知绝不当不存在

```go
func (s *ActiveStore) committedHomeReference(ctx context.Context, intent homeReferenceIntent) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 经集中正规文件枚举判断明确COMMITTED是否存在，IO未知绝不当不存在

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.verifyHomeCommittedReference`

## site-config.verifyHomeCommittedReference

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：读取既有权威文件并核COMMITTED原始主体/op/hash/epoch/精确版本与完整引用digest

```go
func (s *ActiveStore) verifyHomeCommittedReference(ctx context.Context, intent homeReferenceIntent) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取既有权威文件并核COMMITTED原始主体/op/hash/epoch/精确版本与完整引用digest

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.homeReferenceMaterial`

## site-config.checkHomeConfigCommit

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：共享写锁下拒绝终止来源迟到commit；明确COMMITTED优先且不再受新pin expiry阻断

```go
func (s *ActiveStore) checkHomeConfigCommit(ctx context.Context, tx *sql.Tx, record configJournalRecord, config gen.SiteConfig) error
```

**输入**

无

**输出**

无

**步骤**

1. 共享写锁下拒绝终止来源迟到commit；明确COMMITTED优先且不再受新pin expiry阻断

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.readHomeReferenceIntent`、`site-config.committedHomeReference`、`site-config.homeReferenceMaterial`

## site-config.markHomeConfigCommitted

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：与config水位/receipt/outbox同事务保存confirm_pending，不回退confirmed

```go
func markHomeConfigCommitted(ctx context.Context, tx *sql.Tx, operationID string) error
```

**输入**

无

**输出**

无

**步骤**

1. 与config水位/receipt/outbox同事务保存confirm_pending，不回退confirmed

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.homeReferenceContext

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：为acquire/confirm/release分别派生稳定operationId，保原root/source并清用户proof

```go
func homeReferenceContext(ctx context.Context, op OperationContext, intent homeReferenceIntent, phase string) (context.Context, OperationContext)
```

**输入**

无

**输出**

无

**步骤**

1. 为acquire/confirm/release分别派生稳定operationId，保原root/source并清用户proof

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.homeServiceContext`

## site-config.persistHomePin

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：RPC成功后核安全窗口再CAS持久原pin，未知保留且不倒退已收敛阶段

```go
func (s *ActiveStore) persistHomePin(ctx context.Context, op OperationContext, intent homeReferenceIntent, pin homeReferencePin) error
```

**输入**

无

**输出**

无

**步骤**

1. RPC成功后核安全窗口再CAS持久原pin，未知保留且不倒退已收敛阶段

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.validateHomePin`、`site-config.CheckWriteEpoch`、`platform.CanonicalJSON`、`site-config.homeWriteContext`

## site-config.acquireHomePin

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：同sourceOperation/digest获取或找回原pin，不续expiry，完整空集合也发同协议

```go
func (s *ActiveStore) acquireHomePin(ctx context.Context, op OperationContext, intent homeReferenceIntent) (homeReferenceIntent, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同sourceOperation/digest获取或找回原pin，不续expiry，完整空集合也发同协议

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.homeDependencies`、`site-config.homeReferenceContext`、`site-config.validateHomePin`、`site-config.persistHomePin`、`site-config.CheckWriteEpoch`、`media:acquireResourceReferences`、`site-config.homeWriteContext`

## site-config.decideHomeTermination

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：同configMu/SQL写锁先查COMMITTED，未提交才持久aborted；未知不release

```go
func (s *ActiveStore) decideHomeTermination(ctx context.Context, op OperationContext, operationID, code string) (homeReferenceIntent, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同configMu/SQL写锁先查COMMITTED，未提交才持久aborted；未知不release

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.readHomeReferenceIntent`、`site-config.committedHomeReference`、`site-config.markHomeConfigCommitted`、`site-config.CheckWriteEpoch`、`site-config.homeWriteContext`

## site-config.settleHomeReference

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：COMMITTED仅确认已持久pin，终止直接携完整source及可空pinId封存释放，RPC前后核写窗口并CAS完成阶段

```go
func (s *ActiveStore) settleHomeReference(ctx context.Context, op OperationContext, intent homeReferenceIntent) error
```

**输入**

无

**输出**

无

**步骤**

1. committed必须已有持久pin，缺失报INTEGRITY_ERROR；不重新Acquire，不因期限回退COMMITTED
2. aborted/rejected只向media.Release发送完整sourceOperationId/siteId/sourceObjectId/sourceRevisionId/sourceVersion/referenceDigest和可空pinId；media回源确认终局并原子sealed，无pin也封存
3. 404/pending/未知/超时不是释放证明；不以NOT_FOUND触发重新Acquire；COMMITTED绝不release
4. RPC前及成功后分别取得公共写窗口并核当前活动epoch，确认或释放完成阶段CAS落盘；低版本confirm只收敛history

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.homeDependencies`、`site-config.homeReferenceContext`、`site-config.CheckWriteEpoch`、`site-config.homeWriteContext`、`media:confirmResourceReferences`、`media:releaseResourceReferences`

## site-config.compareHomeInput

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：未完成重试核原Home字段与双水位，不重造候选时钟

```go
func compareHomeInput(config gen.SiteConfig, input gen.UpdateHomeInput) bool
```

**输入**

无

**输出**

无

**步骤**

1. 未完成重试核原Home字段与双水位，不重造候选时钟

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`platform.CanonicalJSON`

## site-config.homeTerminatedError

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：只回放持久终局安全失败码，损坏回执不以默认错误掩盖

```go
func homeTerminatedError(intent homeReferenceIntent) error
```

**输入**

无

**输出**

无

**步骤**

1. 只回放持久终局安全失败码，损坏回执不以默认错误掩盖

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.finishHomeFailure

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：失败只在安全写窗口持久终止；已有COMMITTED只能补完不可改报未提交

```go
func (s *ActiveStore) finishHomeFailure(ctx context.Context, op OperationContext, cause error) error
```

**输入**

无

**输出**

无

**步骤**

1. 失败只在安全写窗口持久终止；已有COMMITTED只能补完不可改报未提交

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.decideHomeTermination`、`site-config.homeTerminatedError`

## site-config.homeServiceContext

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：仅为已登记依赖装配本站纯服务主体并清可选用户proof

```go
func homeServiceContext(ctx context.Context, op OperationContext, siteID string) (context.Context, OperationContext)
```

**输入**

无

**输出**

无

**步骤**

1. 仅为已登记依赖装配本站纯服务主体并清可选用户proof

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.resolveHomeDocument

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：只读当前公开链接/ref/锚点权威；可见结果resolvedLanguage必须等于请求language，不可见的href/publication/resolvedLanguage均为null

```go
func resolveHomeDocument(ctx context.Context, op OperationContext, runtime HomeRuntime, siteID, language string, target homeLinkValue) (homeResolvedDocument, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读当前公开链接/ref/锚点权威；可见结果resolvedLanguage必须等于请求language，不可见的href/publication/resolvedLanguage均为null
2. 可见publication含合法revisionId/contentHash及正schemaVersion/revision，拒绝缺失或零水位

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.homeServiceContext`、`site-config.safeHomeRelativeHref`、`public-site:resolvePublishedLink`

## site-config.safeHomeRelativeHref

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：消费公开链接响应时拒绝scheme/host/双斜线/反斜线/query/控制空白，仅接受所请求的当前锚点，原值交HTML输出点转义

```go
func safeHomeRelativeHref(raw string, blockID *string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 消费公开链接响应时拒绝scheme/host/双斜线/反斜线/query/控制空白，仅接受所请求的当前锚点，原值交HTML输出点转义

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.disablePublicHomeButton

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：保留按钮和说明，同时把href/publication置null

```go
func disablePublicHomeButton(button *PublicHeroButton, reason string)
```

**输入**

无

**输出**

无

**步骤**

1. 保留按钮和说明，同时把href/publication置null

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.readHomeQACapability

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：只读QA免费能力；未配置或未知禁用按钮，不探测/调用模型

```go
func readHomeQACapability(ctx context.Context, op OperationContext, runtime HomeRuntime, siteID string) (bool, string)
```

**输入**

无

**输出**

无

**步骤**

1. 只读QA免费能力；未配置或未知禁用按钮，不探测/调用模型

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.homeServiceContext`、`qa:readPublicCapability`

## site-config.RecoverHomeReferenceState

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：构造期只验证全部本地首页intent/pin/COMMITTED与active epoch，不发远端RPC；外部收敛留给公共后台

```go
func (s *ActiveStore) RecoverHomeReferenceState(ctx context.Context, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 构造期只验证全部本地首页intent/pin/COMMITTED与active epoch，不发远端RPC；外部收敛留给公共后台

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`site-config.readHomeReferenceIntent`、`site-config.verifyHomeCommittedReference`

## site-config.homeWriteContext

- 级别：参考
- 文件：`modules/site-config/internal/home_service.go`
- 作用：复用公共NewWriteContext建立有界安全写窗口，并核返回epoch与原操作及本域已打开active一致

```go
func (s *ActiveStore) homeWriteContext(ctx context.Context, op OperationContext) (context.Context, OperationContext, context.CancelFunc, error)
```

**输入**

无

**输出**

无

**步骤**

1. 受信Runtime/WriteWindow非nil；公共函数统一截止/安全窗口，不在域内重复机制
2. 核返回DataEpoch与原op及active水位一致，失败cancel且不返回可写ctx；成功调用方必须cancel

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`platform.NewWriteContext`、`site-config.homeDependencies`

## site-config.ValidateHomeLinks

- 级别：参考
- 文件：`modules/site-config/internal/home_validator.go`
- 作用：校验hero按钮/背景和品牌资源

```go
func ValidateHomeLinks(ctx context.Context, op OperationContext, input ValidateHomeLinksInput) (ValidateHomeLinksResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidateHomeLinksInput` | 验证首页目标与引用的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateHomeLinksResult` | 验证首页目标与引用的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 复用validateHomeStructure纯语义；siteId/language来自本域当前候选，不重新从HTTP解析
2. document经public-site.resolvePublishedLink核当前公开引用及锚点，写入不接受不可见或未知目标
3. 媒体readAsset按指定version核完整AssetVersionRef、siteId与ready，image/animation/video/poster实际种类相符，不自动压缩或转码
4. 仅返回精确document refs和ready assetIds，不获取pin或付费结果

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：site.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读校验
- 依赖：`public-site:resolvePublishedLink`、`media:readAsset`、`site-config.validateHomeStructure`、`site-config.homeDependencies`、`site-config.resolveHomeDocument`、`site-config.homeServiceContext`

## site-config.validateHomeStructure

- 级别：参考
- 文件：`modules/site-config/internal/home_validator.go`
- 作用：复用已校验hero结构提取稳定按钮目标与精确资源用途，纯语义不访问网络

```go
func validateHomeStructure(home gen.HomeConfig) ([]homeDocumentTarget, []homeAssetUse, error)
```

**输入**

无

**输出**

无

**步骤**

1. 检查稳定buttonId不重复，外链复用既有安全协议/凭据/控制字符规则
2. 提取documentId/blockId目标，不猜公开路径或可见性
3. image/animation/video与poster检查声明MIME种类，视频封面只接受图像，资源完整版本引用不降成assetId

**错误**

- 无

- 副作用：无
- 授权：调用链由UpdateHome建立site.manage；聚合纯校验信任已严格校验结构
- 并发：纯函数，不查ctx/权限/epoch或远端
- 幂等：确定性提取
- 依赖：`site-config.validateConfigWebLink`、`site-config.decodeHomeTarget`

## site-config.decodeHomeTarget

- 级别：参考
- 文件：`modules/site-config/internal/home_validator.go`
- 作用：转换已校验的首页按钮判别联合体

```go
func decodeHomeTarget(target gen.LinkTarget) (homeLinkValue, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只转换判别字段，不重做结构schema/身份/权限/epoch校验，不猜公开URL

**错误**

- 无

- 副作用：无
- 授权：由入口与调用链建立
- 并发：纯函数
- 幂等：确定性
- 依赖：无

## site-config.ReadRuntimeState

- 级别：约束
- 文件：`modules/site-config/internal/runtime_state_service.go`
- 作用：读取独立控制库权威运行状态

```go
func ReadRuntimeState(ctx context.Context, op OperationContext, input ReadRuntimeStateInput) (ReadRuntimeStateResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadRuntimeStateInput` | 读取独立控制库权威运行状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadRuntimeStateResult` | 读取独立控制库权威运行状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 只读control.db当前revision
2. 校验dataEpoch与phase维护布尔不矛盾
3. 返回签名可见状态，不公开内部密钥

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：service 或 system.manage；入口公开仅返回maintenance布尔经portal
- 并发：控制库快照；不受业务恢复导入影响
- 幂等：只读
- 依赖：无

## site-config.OpenRuntimeStore

- 级别：参考
- 文件：`modules/site-config/internal/runtime_store.go`
- 作用：按ADR0005打开控制库并恢复RuntimeState权威

```go
func OpenRuntimeStore(ctx context.Context, dataRoot string) (*RuntimeStore, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 打开控制库的取消与截止传播 | 取消或截止立即停止新数据库副作用 |
| `dataRoot` | `string` | 本域独占数据根 | 必须为非空受控路径；不写入日志 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `RuntimeStore` | 独立control.db句柄 | control.db位于control区，不随active恢复换目录 |
| `error` | `Error` | 打开、迁移或初始状态完整性失败 | 失败不返回可用句柄；不泄露路径和密钥 |

**步骤**

1. 设置WAL、foreign_keys=ON、busy_timeout=3000ms并限制单连接
2. 执行编号连续的只追加迁移；未知已应用版本拒绝启动
3. 首次建库生成持久dataEpoch、running phase、revision=1和14个requiredModules

**错误**

- `DEPENDENCY_UNAVAILABLE`：控制库不可打开或不可读（可重试）
- `INTEGRITY_ERROR`：迁移集、schema或RuntimeState不一致（不可重试）

- 副作用：创建或更新control/control.db
- 授权：service启动控制面
- 并发：一个进程单控制库句柄；SQLite事务串行写入
- 幂等：重复启动读取已有RuntimeState，不重新生成epoch
- 依赖：`site-config.All`

## site-config.OpenActiveStore

- 级别：参考
- 文件：`modules/site-config/internal/runtime_store.go`
- 作用：按ADR0005打开active私有业务库并校验活动epoch

```go
func OpenActiveStore(ctx context.Context, dataRoot string, dataEpoch Identifier) (*ActiveStore, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播 | 取消或截止立即停止新数据库副作用 |
| `dataRoot` | `string` | 本域独占数据根 | 必须为非空受控路径；不写入日志 |
| `dataEpoch` | `Identifier` | control.db当前活动代次 | 必须为合法Identifier；必须与active状态一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `store` | `ActiveStore` | active业务库与正文目录句柄 | active数据库与control.db分离；失败不返回可用句柄 |
| `error` | `Error` | 打开、迁移或active状态完整性失败 | 不得把未知schema或epoch当成功 |

**步骤**

1. 设置WAL、foreign_keys=ON、busy_timeout=3000ms并限制单连接
2. 执行active目录连续只追加迁移，未知版本拒绝启动
3. 首次建库只建立未初始化状态，不生成伪配置
4. 设置SQLite synchronous=FULL，初始化决定与准备材料在返回前持久提交

**错误**

- `DEPENDENCY_UNAVAILABLE`：active数据库或目录不可用（可重试）
- `INTEGRITY_ERROR`：迁移、schema或活动epoch不一致（不可重试）

- 副作用：创建或更新active/site-config.db及journal/quarantine目录
- 授权：service启动控制面
- 并发：一个进程单active句柄；SQLite事务串行写入
- 幂等：重复启动读取已有active状态，不生成伪站点配置
- 依赖：无

## site-config.ValidateActiveReady

- 级别：参考
- 文件：`modules/site-config/internal/runtime_store.go`
- 作用：Ready前校验active schema、epoch和公告journal恢复指纹

```go
func (s *ActiveStore) ValidateActiveReady(ctx context.Context, dataEpoch Identifier) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 就绪检查截止传播 | 取消或截止立即停止读取 |
| `dataEpoch` | `Identifier` | control.db当前活动代次 | 必须与active状态一致 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `initialized` | `boolean` | active是否已完成初始化 | 仅在schema、epoch、journal均一致时有效 |
| `error` | `Error` | schema、epoch、journal或索引失败 | 失败必须阻止Ready |

**步骤**

1. 校验迁移记录完整且与内嵌迁移一致
2. 校验active状态与control活动epoch一致
3. 校验journal已恢复且文件指纹未改变
4. 配置journal须完成恢复，核对settings权威head、COMMITTED与SQL水位及对象hash/schema；initialized=false但已有config_committed/blocked水位是可恢复中间态，不能公开为已初始化

**错误**

- `DEPENDENCY_UNAVAILABLE`：active数据库或journal不可读（可重试）
- `INTEGRITY_ERROR`：schema、epoch、journal或索引矛盾（不可重试）

- 副作用：无
- 授权：service启动控制面；由匿名Ready调用
- 并发：只读快照，可并发检查
- 幂等：只读
- 依赖：无

## site-config.RuntimeStore.ReadRuntimeView

- 级别：参考
- 文件：`modules/site-config/internal/runtime_store.go`
- 作用：从真实控制状态投影公共运行视图及revision/barrierId

```go
func (s *RuntimeStore) ReadRuntimeView(ctx context.Context) (RuntimeView, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取同一控制库RuntimeState，原样传递DataEpoch/Phase/Maintenance/TargetDataEpoch/Revision/BarrierId；不伪造Ready水位。

**错误**

- `DEPENDENCY_UNAVAILABLE`：active数据库或journal不可读（可重试）
- `INTEGRITY_ERROR`：schema、epoch、journal或索引矛盾（不可重试）

- 副作用：无
- 授权：service启动控制面；由匿名Ready调用
- 并发：只读快照，可并发检查
- 幂等：只读
- 依赖：无

## site-config.TransitionRuntimeState

- 级别：约束
- 文件：`modules/site-config/internal/runtime_transition.go`
- 作用：按CAS切换epoch与维护阶段

```go
func TransitionRuntimeState(ctx context.Context, op OperationContext, input RuntimeTransition) (TransitionRuntimeStateResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RuntimeTransition` | 按CAS切换epoch与维护阶段的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TransitionRuntimeStateResult` | 按CAS切换epoch与维护阶段的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 校验调用方为data-protection服务身份（用户授权已由data-protection核验）
2. ValidateRuntimeTransition检查当前phase与expectedRevision
3. control.db事务落持久迁移：begin_switch记录restoreId与targetDataEpoch；complete_restore仅在全部有状态域switched回执齐全后切dataEpoch并phase=running；complete_rollback在已换入域全部rolled-back后保持原dataEpoch并phase=running

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：独立control.db运行状态和决定日志
- 授权：permission：service；rule：仅data-protection服务身份；手动备份/恢复的superadmin授权与定时备份设置由data-protection核验
- 并发：控制库全局单写事务；不能从导入快照覆盖runtimeState
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.ValidateRuntimeTransition`

## site-config.ValidateRuntimeTransition

- 级别：参考
- 文件：`modules/site-config/internal/runtime_transition.go`
- 作用：验证维护与恢复状态迁移矩阵

```go
func ValidateRuntimeTransition(input ValidateRuntimeTransitionInput) (ValidateRuntimeTransitionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidateRuntimeTransitionInput` | 验证维护与恢复状态迁移矩阵的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateRuntimeTransitionResult` | 验证维护与恢复状态迁移矩阵的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. running仅begin_backup/begin_restore；backup_barrier仅complete_backup
2. restore_staging仅begin_switch/abort_restore/mark_blocked；begin_switch只由data-protection在StageAllDomains全部暂存完成后发起（暂存完成由data-protection核验，本函数校验restoreId/targetDataEpoch/barrierId一致）
3. restore_switching可complete_restore/begin_rollback/mark_blocked；restore_blocked可begin_rollback或再次begin_switch；restore_rolling_back仅complete_rollback
4. complete_restore要求14个有状态域switched到目标epoch；complete_rollback要求已换入域全部rolled-back（不得残留switched回执；从未换入的域不产生回执）；禁止未知、跳阶段或自行关维护

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：纯函数，外层必须验证调用方身份与委托
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## site-config.RegisterActivation

- 级别：约束
- 文件：`modules/site-config/internal/runtime_transition.go`
- 作用：登记域恢复切换/换回回执

```go
func RegisterActivation(ctx context.Context, op OperationContext, input RegisterActivationInput) (RegisterActivationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RegisterActivationInput` | 登记域恢复切换/换回回执的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RegisterActivationResult` | 登记域恢复切换/换回回执的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. caller服务身份必须与receipt.module一致
2. 校验restoreId、targetDataEpoch与当前phase：restore_switching只收switched，restore_rolling_back只收rolled-back；低状态不得覆盖高状态
3. control.db事务upsert回执和revision；data-protection不能替任何域伪造回执

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：control.db切换/换回回执
- 授权：service：对应域服务身份
- 并发：(restoreId,module)唯一键；多个域可并发但revision CAS串行
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：无

## site-config.CheckWriteEpoch

- 级别：参考
- 文件：`modules/site-config/internal/write_epoch.go`
- 作用：所有业务写入执行epoch与屏障检查

```go
func CheckWriteEpoch(ctx context.Context, op OperationContext, input CheckWriteEpochInput) (CheckWriteEpochResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CheckWriteEpochInput` | 所有业务写入执行epoch与屏障检查的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CheckWriteEpochResult` | 所有业务写入执行epoch与屏障检查的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 读control.db当前phase/dataEpoch
2. 非running或本域barrier非released返回LOCKED
3. epoch不等返回VERSION_MISMATCH；检查在业务txn登记与控制gate同互斥区

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：私有业务写入入口
- 并发：与屏障登记及事务计数共享gate
- 幂等：只读判定
- 依赖：无

## site-config.All

- 级别：参考
- 文件：`modules/site-config/internal/migrations/migrations.go`
- 作用：按编号升序返回内嵌控制库迁移

```go
func All() ([]Migration, error)
```

**输入**

无

**输出**

无

**步骤**

1. go:embed读取NNNN_name.sql；文件名或编号不合法、编号不连续即报错

**错误**

- `INTEGRITY_ERROR`：迁移文件名、编号或内容不可读取（不可重试）

- 副作用：无
- 授权：内部装配
- 并发：只读，可并发
- 幂等：只读
- 依赖：无

## site-config.InitializeSite

- 级别：约束
- 文件：`modules/site-config/internal/initialization_service.go`
- 作用：无需AI完成首站点初始化

```go
func InitializeSite(ctx context.Context, op OperationContext, input InitializeSiteInput) (InitializeSiteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `InitializeSiteInput` | 无需AI完成首站点初始化的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InitializeSiteResult` | 无需AI完成首站点初始化的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 要求已注册唯一超管admin会话、非空session/subject/site和superadmin授权；siteId取受信context，不创建第二账号，不重开注册
2. 规范语言后以kind=initialize/name/规范baseLanguage计算原始业务摘要；按actor/site/op查询回执，同operation异请求或主体CONFLICT；已有commit只可同operation继续；准备时钟只在首次生成并随决定保存
3. 从ConfigureInitialization取得已由公共配置边界校验的受信publicBaseURL，缺失失败关闭；PrepareDefaultConfig仅无AI基础配置；持久commit与原始actor/site/epoch/hash回执同事务，ResumeInitialization提交权威文件并完成initialized

**错误**

- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `FORBIDDEN`：非唯一超管或未注册会话（不可重试）
- `INTEGRITY_ERROR`：持久决定、权威对象或回执不一致，失败关闭（不可重试）
- `VERSION_MISMATCH`：CAS或dataEpoch已变化（不可重试）
- `LOCKED`：运行状态处于维护或屏障阶段（不可重试）

- 副作用：本服务初始化决定与权威站点配置文件；不创建第二owner
- 授权：superadmin；已完成identity首注册，向导只有站点/基准语言两步
- 并发：全站初始化唯一锁；相同operationId继续原阶段，不生成第二site
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.PrepareDefaultConfig`、`site-config.PersistInitializationDecision`、`site-config.ResumeInitialization`、`site-config.ResolveConfigOperation`、`site-config.ConfigureInitialization`

## site-config.ReadInitializationStatus

- 级别：约束
- 文件：`modules/site-config/internal/initialization_service.go`
- 作用：读取安全初始化阶段

```go
func ReadInitializationStatus(ctx context.Context, op OperationContext, input ReadInitializationStatusInput) (ReadInitializationStatusResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadInitializationStatusInput` | 读取安全初始化阶段的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadInitializationStatusResult` | 读取安全初始化阶段的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 持久状态或准备配置哈希不一致，失败关闭 |

**步骤**

1. active只读事务读取initialized与当前初始化状态；无协调记录且未初始化时phase=uninitialized、decision=none、revision/configRevision=0、operationId/siteId=null，updatedAt取active建库时间
2. 所有调用返回initialized与仅phase/safeError的state，不输出ownerEmail/password/capability
3. 仅identity/site-config服务身份或同站点admin且含superadmin授权时附coordination完整协调状态；匿名和其他主体完全省略coordination

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：初始化决定、准备配置哈希或active水位不一致（不可重试）

- 副作用：无
- 授权：anonymous仅initialized与安全state；同站点superadmin或service:identity/site-config可读coordination
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`site-config.ReadInitializationRecord`

## site-config.ResumeInitialization

- 级别：约束
- 文件：`modules/site-config/internal/initialization_service.go`
- 作用：恢复崩溃或依赖断开的初始化阶段

```go
func ResumeInitialization(ctx context.Context, op OperationContext, input ResumeInitializationInput) (ResumeInitializationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ResumeInitializationInput` | 恢复崩溃或依赖断开的初始化阶段的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResumeInitializationResult` | 恢复崩溃或依赖断开的初始化阶段的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用 |

**步骤**

1. 同站点已登录admin具site.manage/superadmin或本域service受控启动才允许恢复；读取当前初始化commit决定，核对完整准备材料SHA256和原始actor/site/epoch/业务摘要回执，缺失失败关闭，不重新生成preparedAt、不调用identity owner准备
2. 以epoch/site/初始化operation的SHA256作为稳定revisionId，完整准备配置仅改initialized/revision/home.revision为true/1/1，WriteConfigRevision先持久settings文件及PREPARED；CommitConfigHead先持久COMMITTED决定，再head、SQL索引/回执/outbox/config_committed
3. 仅COMMITTED与已完成SQL水位一致后另事务推进completed和initialized=true，保存原始结果；取消沿ctx停止IO，已持久决定只能补完，失败记录安全blocked，重试不得改决定；completed后返回原初始化结果

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `FORBIDDEN`：主体不具备本函数所需站点权限（不可重试）
- `VERSION_MISMATCH`：CAS或dataEpoch已变化（不可重试）
- `LOCKED`：运行状态处于维护或屏障阶段（不可重试）

- 副作用：私有配置事务、operation receipt 与 outbox
- 授权：同站点已登录admin且site.manage/superadmin；或callerService=site-config且actor.kind=service受控启动
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.ReadInitializationRecord`、`site-config.WriteConfigRevision`、`site-config.CommitConfigHead`

## site-config.AbortInitialization

- 级别：约束
- 文件：`modules/site-config/internal/initialization_service.go`
- 作用：取消尚未持久commit的初始化

```go
func AbortInitialization(ctx context.Context, op OperationContext, input AbortInitializationInput) (AbortInitializationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `AbortInitializationInput` | 取消尚未持久commit的初始化的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AbortInitializationResult` | 取消尚未持久commit的初始化的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 要求已注册唯一超管admin会话；先按actor/site/目标operation和kind=abort/目标operation/expectedRevision摘要返回原取消结果，不能用旧回执替换主体或请求
2. 存在任何commit决定即CONFLICT，只能继续恢复；无commit时CAS expectedRevision持久abort及原始结果回执；本实现不在commit前生成settings临时stage，故无未提交配置stage可删除
3. 不删除唯一超管、不重开注册；已abort可由新operation重新填写本域向导

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久决定、权威对象或回执不一致，失败关闭（不可重试）
- `VERSION_MISMATCH`：CAS或dataEpoch已变化（不可重试）
- `LOCKED`：运行状态处于维护或屏障阶段（不可重试）

- 副作用：私有配置事务、operation receipt 与 outbox
- 授权：同站点已注册唯一超管admin会话；不再使用sealed初始化capability
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.PersistInitializationDecision`、`site-config.ResolveConfigOperation`

## site-config.ConfigureInitialization

- 级别：参考
- 文件：`modules/site-config/internal/initialization_service.go`
- 作用：注入公共配置边界已校验的可信部署公开地址

```go
func (s *ActiveStore) ConfigureInitialization(publicBaseURL string, allowLocalHTTP bool)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `publicBaseURL` | `string` | 来自platform.ServiceConfig的可信地址；不从Host派生 | 共享配置入口已校验https或显式允许的loopback http；空值仅允许既有读取启动 |
| `allowLocalHTTP` | `bool` | 公共配置边界已校验的显式loopback HTTP能力 | 只由进程配置注入；不能取自HTTP字段 |

**输出**

无

**步骤**

1. 启动时把公共配置边界已校验的publicBaseURL保存为本域只读部署能力，供InitializeSite使用；不持久秘密、不改站点配置；同时保存allowLocalHTTP，只给纯URL校验使用

**错误**

- 无

- 副作用：本进程可信初始化环境
- 授权：仅本域进程装配
- 并发：启动装配注入，受私有状态锁保护
- 幂等：相同配置重复注入无业务副作用
- 依赖：无

## site-config.PrepareDefaultConfig

- 级别：参考
- 文件：`modules/site-config/internal/initialization_defaults.go`
- 作用：确定性生成首次基础配置

```go
func PrepareDefaultConfig(input PrepareDefaultConfigInput) (PrepareDefaultConfigResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `PrepareDefaultConfigInput` | 确定性生成首次基础配置的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PrepareDefaultConfigResult` | 确定性生成首次基础配置的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. baseLanguage缺省或空串为zh-Hans；显式标签规范化；updatedAt使用input.preparedAt，revision/home.revision=0且initialized=false
2. 站名为mainTitle；Welcome To不翻译，空subtitle和按钮，纯色背景#0F172A、overlayPercent=0、textColor=light；不制造示例文档或调用模型
3. themeColor=#2563EB；titleTemplate={title} | {site}、description空、indexingEnabled=true；logo/favicon/defaultShareImage/banner为null
4. footer版权和links为空、备案为null、sourceCodeURL=https://github.com/Uniseem/nice-wiki；资源默认doc16MiB/upload128MiB/site10GiB/上传并发4；安全默认previewTTL900s/sessionAbsoluteTTL604800s
5. publicBaseURL保持可信调用方显式输入；不从请求Host推断部署地址

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`site-config.ValidateLanguageTag`

## site-config.PersistInitializationDecision

- 级别：参考
- 文件：`modules/site-config/internal/initialization_repository.go`
- 作用：持久化不可反悔的初始化决定

```go
func PersistInitializationDecision(ctx context.Context, op OperationContext, input PersistInitializationDecisionInput) (PersistInitializationDecisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PersistInitializationDecisionInput` | 持久化不可反悔的初始化决定的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PersistInitializationDecisionResult` | 持久化不可反悔的初始化决定的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅由本域已授权用例私有调用；要求op.dataEpoch与active匹配，state的operationId/siteId非null，用户委托站点若存在须一致；保留原调用OperationContext，不要求调用服务为自身
2. 按operationId读取不可变决定回执；同operation/site/decision/准备配置哈希返回原决定版本，不同请求或不同决定返回CONFLICT；已提交同请求读取回执允许处于维护状态，新决定仍须CheckWriteEpoch通过
3. active私有事务CAS expectedRevision；无记录仅允许0，已commit不得改abort或开启另一operation；已abort可用新operation继续本域向导，但不重开identity注册
4. commit要求state为preparing且完整preparedConfig属于同site、initialized=false、revision/home.revision=0；同时保存规范JSON与SHA256；abort要求preparedConfig=null且无commit
5. 同事务写不可变决定回执与当前状态，revision递增；commit保留preparing等待权威文件提交，abort进入aborted；SQLite synchronous=FULL提交后才执行外部动作
6. 原始业务requestHash与actor/site/epoch/operation、稳定revisionId在同一决定事务保存config_operations；commit记in_progress，abort存committed与原始state结果。材料摘要只判断决定不反悔，不替代原始业务请求摘要

**错误**

- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `VERSION_MISMATCH`：expectedRevision或active dataEpoch已改变（不可重试）
- `LOCKED`：维护或恢复屏障禁止新决定（不可重试）

- 副作用：active初始化决定、完整prepared配置和哈希、当前状态与幂等回执；不发布配置或事件
- 授权：site-config 私有协调
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：同决定重复返回原版本；不同终局决定CONFLICT
- 依赖：`site-config.CheckWriteEpoch`、`site-config.ReadInitializationRecord`

## site-config.ReadInitializationRecord

- 级别：参考
- 文件：`modules/site-config/internal/initialization_repository.go`
- 作用：读取私有初始化协调状态并核对active初始化水位

```go
func (s *ActiveStore) ReadInitializationRecord(ctx context.Context, q ReadTransaction) (InitializationState, bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 传播到私有数据库读取 |  |
| `q` | `ReadTransaction` | 同一active数据库只读或写事务 | 内部句柄，不经HTTP序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `state` | `InitializationState` | 完整可信协调状态，未开始时标识为null |  |
| `initialized` | `bool` | active初始化完成标志 |  |
| `error` | `Error` | 私有存储或完整性失败 |  |

**步骤**

1. 同一事务读取active初始化水位及当前协调JSON
2. 严格解析持久状态，核对决定与完整准备配置SHA256、active水位；无协调记录且未初始化才返回合法初始状态，不生成占位ID

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：初始化决定、准备配置哈希或active水位不一致（不可重试）

- 副作用：无
- 授权：私有数据库读取；调用层裁剪公开字段
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## site-config.ValidateAnnouncementBody

- 级别：参考
- 文件：`modules/site-config/internal/announcement_validator.go`
- 作用：校验公告纯文本正文并规范序列化

```go
func ValidateAnnouncementBody(input ValidateAnnouncementBodyInput) (ValidateAnnouncementBodyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidateAnnouncementBodyInput` | 校验公告纯文本正文并规范序列化的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateAnnouncementBodyResult` | 校验公告纯文本正文并规范序列化的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性 |

**步骤**

1. schemaVersion=1，正文只有text和linkTarget，严格拒绝unknown
2. text会翻译；safe URL只http/https/mailto或docId，禁任意HTML
3. canonicalJSON与稳定bannerId计算hash

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_SCHEMA`：版本或块 schema 不受支持；禁止静默丢弃未知属性（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## site-config.WriteAnnouncementRevision

- 级别：参考
- 文件：`modules/site-config/internal/announcement_files.go`
- 作用：持久写不可变公告正文文件

```go
func WriteAnnouncementRevision(ctx context.Context, op OperationContext, input WriteAnnouncementRevisionInput) (WriteAnnouncementRevisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `WriteAnnouncementRevisionInput` | 持久写不可变公告正文文件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteAnnouncementRevisionResult` | 持久写不可变公告正文文件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 服务生成site/announcement/revision路径，不接受用户path
2. 同文件系统写临时文件、fsync、原子rename并fsync父目录
3. hash已存在同值返回；同revision异hashINTEGRITY_ERROR；失败不移动draft head

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有正文文件和PREPARED journal
- 授权：site-config 私有正文提交
- 并发：revisionId唯一；文件不覆盖
- 幂等：同id同hash重用，异hash拒绝
- 依赖：无

## site-config.CommitAnnouncementHead

- 级别：参考
- 文件：`modules/site-config/internal/announcement_files.go`
- 作用：原子提交公告head与正文日志

```go
func CommitAnnouncementHead(ctx context.Context, op OperationContext, input CommitAnnouncementHeadInput) (CommitAnnouncementHeadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CommitAnnouncementHeadInput` | 原子提交公告head与正文日志的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitAnnouncementHeadResult` | 原子提交公告head与正文日志的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 确认PREPARED正文文件已fsync且hash一致；单独调用只核head预期，不提前公开
2. 配置提交的同一SQLite事务内追加COMMITTED行、替换head并写入announcement_heads；回执与outbox仍由CommitConfigHead提交
3. 正文缺失或hash不符失败关闭，不伪造旧快照；失败后的JSONL与head由既有恢复按最新COMMITTED核验

**错误**

- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有配置事务、operation receipt 与 outbox
- 授权：site-config 私有提交
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.RecoverAnnouncementJournal`

## site-config.RecoverAnnouncementJournal

- 级别：参考
- 文件：`modules/site-config/internal/announcement_files.go`
- 作用：恢复正文文件与提交日志

```go
func RecoverAnnouncementJournal(ctx context.Context, op OperationContext, input RecoverAnnouncementJournalInput) (RecoverAnnouncementJournalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RecoverAnnouncementJournalInput` | 恢复正文文件与提交日志的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverAnnouncementJournalResult` | 恢复正文文件与提交日志的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 启动停止公告写入
2. COMMITTED逐一核对head/object/hash，缺失即隔离该公告并告警
3. PREPARED无已提交head者按保留窗口隔离，不推定发布；恢复完成再开放

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：日志恢复与私有孤儿隔离
- 授权：service:site-config启动控制
- 并发：启动exclusive；运行时仅屏障safe执行
- 幂等：反复恢复不会使PREPARED变COMMITTED
- 依赖：无

## site-config.ResolveConfigOperation

- 级别：参考
- 文件：`modules/site-config/internal/operation_repository.go`
- 作用：查询配置幂等回执

```go
func ResolveConfigOperation(ctx context.Context, op OperationContext, input ResolveConfigOperationInput) (ResolveConfigOperationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ResolveConfigOperationInput` | 查询配置幂等回执的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveConfigOperationResult` | 查询配置幂等回执的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 按全局operationId读取不可重建回执，同时核对原始actor/site/requestHash及当前dataEpoch
2. 同键异主体/site/hash返回CONFLICT，异epoch返回VERSION_MISMATCH
3. 返回absent/in_progress/committed及原始resultRevision；不写密码或将初始化准备材料冒充settings

**错误**

- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `VERSION_MISMATCH`：CAS或dataEpoch已变化（不可重试）

- 副作用：无
- 授权：私有操作事务
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## site-config.PublishConfigEvent

- 级别：参考
- 文件：`modules/site-config/internal/outbox_repository.go`
- 作用：将配置事件与业务提交原子登记

```go
func PublishConfigEvent(ctx context.Context, op OperationContext, tx WriteTransaction, input PublishConfigEventInput) (PublishConfigEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `tx` | `WriteTransaction` | 配置提交方持有的本域SQLite业务事务 | 本函数不自行提交或打开事务 |
| `input` | `PublishConfigEventInput` | 将配置事件与业务提交原子登记的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishConfigEventResult` | 将配置事件与业务提交原子登记的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 装配site.config.changed v1最小引用载荷和固定eventId=revisionId；信封revision/dataEpoch来自当前配置决定
2. 复用platform.EnqueueEvent向调用方写事务登记唯一outbox，与配置水位和回执原子提交；本函数不直接调用jobs
3. 同eventId同payloadHash复用，异hash失败；由后续RelayOutbox装配转发，低revision不可覆盖

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有配置事务、operation receipt 与 outbox
- 授权：私有配置提交
- 并发：与configRevision同一事务
- 幂等：eventId唯一
- 依赖：`platform.EnqueueEvent`

## site-config.RelayOutbox

- 级别：约束
- 文件：`modules/site-config/internal/outbox_repository.go`
- 作用：把已提交的outbox事件可靠转交jobs；进程启动先补发，之后后台循环

```go
func RelayOutbox(ctx context.Context, op OperationContext, input RelayOutboxInput) (RelayOutboxResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 本域后台worker以服务身份构建的操作上下文，含当前dataEpoch | callerService为本域；dataEpoch等于site-config当前active epoch |
| `input` | `RelayOutboxInput` | 单批转发参数 | 必须通过公共schema；batchLimit 1..100 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RelayOutboxResult` | 本批转发计数 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 进程启动完成journal恢复后立即运行一次，之后每次本域提交通知或1秒轮询运行；按outbox序号以30秒行租约领取≤batchLimit条未转发行，只读已提交事务
2. 逐条调用jobs:publishEvent，携带固定eventId、payloadHash、dataEpoch与subject revision；jobs持久确认后在本地事务把该行标relayed并记录jobs eventSeq
3. 网络失败或超时保留该行，按1/2/4秒指数退避（上限60秒）在下一轮重试；同eventId异hash或信封校验失败标quarantined并报警，不再重试也不删除
4. 屏障draining期间继续转发已提交行直至为空或屏障超时；快照包含未转发行；恢复换入新数据后丢弃旧dataEpoch未转发行，以恢复后的权威状态为准

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用或超时；行保留待下一轮（可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash或信封不合法；行隔离（不可重试）
- `VERSION_MISMATCH`：dataEpoch与当前active epoch不一致；旧代行按恢复规则丢弃（不可重试）

- 副作用：读写本域outbox行状态与租约；HTTP调用jobs:publishEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个转发循环；行级租约防多实例重复领取；同subject按outbox序号串行发送
- 幂等：eventId在业务事务内固定；jobs按eventId+payloadHash去重，重复发送返回原确认
- 依赖：`jobs:publishEvent`、`platform.RunRelay`

## site-config.ReadConfigObject

- 级别：约束
- 文件：`modules/site-config/internal/object_reader.go`
- 作用：经协议读取本域正文或快照对象

```go
func ReadConfigObject(ctx context.Context, op OperationContext, input ReadConfigObjectInput) (ReadConfigObjectResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadConfigObjectInput` | 经协议读取本域正文或快照对象的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadConfigObjectResult` | 经协议读取本域正文或快照对象的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 仅public-site/data-protection已登记服务主体或同站点site.read/superadmin用户；owner固定site-config，输入仅objectId/version/hash，不接受磁盘路径
2. 当前读取已真实登记的settings不可变对象：从config_revision_index与初始化/epoch/COMMITTED定位，校验version/hash/大小/schema，尚无完成提交索引的公告或快照对象NOT_FOUND，不制造通用成功
3. 复用已有16MiB正规文件/目录链IO边界取得不可变字节快照，返回本域x-internal io.ReadCloser；HTTP设置绑定ObjectRef的ETag、Content-Type/Length/版本头后消费并关闭，不暴露bodyHandle或建立全局句柄表

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `VERSION_MISMATCH`：活动epoch或expectedRevision已变化（不可重试）
- `DEPENDENCY_UNAVAILABLE`：本域权威存储不可用（可重试）

- 副作用：本进程有界不可变读取快照；Reader由HTTP关闭
- 授权：service:public-site/data-protection或site.read
- 并发：仅已提交历史对象；有界快照不受后续head变化或文件删除影响，不向客户端暴露文件句柄
- 幂等：只读
- 依赖：无

## site-config.AcquireBarrier

- 级别：约束
- 文件：`modules/site-config/internal/barrier_service.go`
- 作用：取得可恢复的数据屏障

```go
func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `BarrierRequest` | 取得可恢复的数据屏障的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 取得可恢复的数据屏障的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准 |

**步骤**

1. 认证 data-protection 服务及 superadmin 委托；验证 epoch、operationId 与 deadline
2. 私有事务登记屏障并拒绝新的业务写入；等待已登记提交完成
3. 固定 revisionWatermark 并返回 safe；超时返回 draining/failed 且保持维护，不擅自释放

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `DEADLINE_EXCEEDED`：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准（可重试）

- 副作用：本服务屏障/快照/恢复日志与私有对象写入
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：屏障与暂存按 barrierId/restoreId 串行；活动事务计数达到零后才 safe；恢复在进程重启时整体换入active目录
- 幂等：operationId/backupId/restoreId+摘要去重；重复返回已记录阶段
- 依赖：`site-config.ReadRuntimeState`、`platform.SnapshotManager.EnterBarrier`

## site-config.ReleaseBarrier

- 级别：约束
- 文件：`modules/site-config/internal/barrier_service.go`
- 作用：释放已协调完成的屏障

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, input ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReleaseBarrierInput` | 释放已协调完成的屏障的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 释放已协调完成的屏障的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变 |

**步骤**

1. 查询持久屏障
2. 验证维护退出决定、全部域切换/换回回执已齐及 epoch 一致
3. 原子改 released 并恢复业务接入

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）

- 副作用：私有屏障状态写入
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.ReadRuntimeState`、`platform.SnapshotManager.ReleaseBarrier`

## site-config.ExportSnapshot

- 级别：约束
- 文件：`modules/site-config/internal/snapshot_service.go`
- 作用：导出屏障水位的一致快照

```go
func ExportSnapshot(ctx context.Context, op OperationContext, input SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `SnapshotExportRequest` | 导出屏障水位的一致快照的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 导出屏障水位的一致快照的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；DEADLINE_EXCEEDED: 依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准 |

**步骤**

1. 验证本域 barrier=safe 和当前 dataEpoch；按 includeSecrets 策略封装敏感字段
2. 流式复制私有数据库一致视图及所有权威文件到本服务只读对象；manifest 列 hash/size/schema
3. fsync 完成后返回 SnapshotRef；失败只清理未完成临时对象，不修改活动数据

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `DEADLINE_EXCEEDED`：依赖或本地操作未在期限内结束；提交状态以 operationId 回执为准（可重试）

- 副作用：本服务屏障/快照/恢复日志与私有对象写入
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：屏障与暂存按 barrierId/restoreId 串行；活动事务计数达到零后才 safe；恢复在进程重启时整体换入active目录
- 幂等：operationId/backupId/restoreId+摘要去重；重复返回已记录阶段
- 依赖：`site-config.ReadRuntimeState`、`platform.SnapshotManager.Export`

## site-config.StageSnapshot

- 级别：约束
- 文件：`modules/site-config/internal/snapshot_service.go`
- 作用：把本域备份归档暂存到restore/<restoreId>/staged，不触碰active数据

```go
func StageSnapshot(ctx context.Context, op OperationContext, input SnapshotStageRequest) (SnapshotStageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | data-protection服务身份携带发起恢复的superadmin委托 | callerService=data-protection；dataEpoch等于site-config当前active epoch |
| `input` | `SnapshotStageRequest` | 暂存请求 | 必须通过公共schema；snapshot.owner等于本域 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotStageResult` | 暂存回执 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含路径、密钥或正文 |

**步骤**

1. 校验data-protection服务身份（用户授权已由data-protection核验）；site-config runtime为restore_staging且restoreId一致；本域restore屏障为safe
2. 经data-protection:serveRestoreArchive按restoreId拉取本域归档，边读边算SHA-256，写入<dataRoot>/restore/<restoreId>/staged.tmp
3. 校验checksum、owner、manifest内文件hash，以及dataSchemaVersion不高于本版本可迁移范围；任一不符删除临时目录并返回INTEGRITY_ERROR或UNSUPPORTED_SCHEMA
4. fsync后改名为staged，在control区记录暂存回执（restoreId、snapshotId、checksum）；active目录与数据库连接保持不变

**错误**

- `FORBIDDEN`：调用方不是data-protection或缺superadmin委托（不可重试）
- `LOCKED`：runtime阶段或restoreId不符，或本域屏障未safe（不可重试）
- `INTEGRITY_ERROR`：归档hash、owner或manifest不一致；临时目录已删除（不可重试）
- `UNSUPPORTED_SCHEMA`：快照schema高于本版本可迁移范围（不可重试）
- `CONFLICT`：同restoreId已暂存不同checksum（不可重试）
- `DEPENDENCY_UNAVAILABLE`：拉取归档失败或磁盘空间不足；临时目录已删除（可重试）

- 副作用：写<dataRoot>/restore/<restoreId>/staged与control区回执；HTTP读取data-protection恢复归档
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：control区按restoreId串行；不锁active读写；同一时间只允许一个恢复
- 幂等：restoreId+checksum唯一；重复调用返回已记录回执
- 依赖：`data-protection:serveRestoreArchive`、`site-config.ReadRuntimeState`、`platform.SnapshotManager.Stage`

## site-config.DiscardSnapshot

- 级别：约束
- 文件：`modules/site-config/internal/snapshot_service.go`
- 作用：删除某次恢复的暂存目录或恢复完成后换出的旧数据目录

```go
func DiscardSnapshot(ctx context.Context, op OperationContext, input SnapshotDiscardRequest) (SnapshotDiscardResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | data-protection服务身份携带发起恢复的superadmin委托 | callerService=data-protection；dataEpoch等于site-config当前active epoch |
| `input` | `SnapshotDiscardRequest` | 丢弃请求 | 必须通过公共schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotDiscardResult` | 丢弃结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含路径、密钥或正文 |

**步骤**

1. 校验data-protection服务身份（用户授权已由data-protection核验）
2. scope=staged：仅当runtime不在restore_switching/restore_rolling_back、且control日志显示本域尚未换入该restoreId时删除restore/<restoreId>/staged
3. scope=before：仅当该恢复已completed或已整体换回、且control日志确认active不是该before时删除restore/<restoreId>/before
4. 删除后在control区记录discarded；目录不存在返回removed=false

**错误**

- `FORBIDDEN`：调用方不是data-protection或缺superadmin委托（不可重试）
- `LOCKED`：当前恢复阶段不允许删除该目录（不可重试）

- 副作用：删除<dataRoot>/restore/<restoreId>下的目录；写control区回执
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：control区按restoreId串行
- 幂等：重复删除返回removed=false
- 依赖：`site-config.ReadRuntimeState`、`platform.SnapshotManager.Discard`

## site-config.UpdateBranding

- 级别：约束
- 文件：`modules/site-config/internal/branding_service.go`
- 作用：更新站名/logo/图标/主题色/默认分享图

```go
func UpdateBranding(ctx context.Context, op OperationContext, input UpdateBrandingInput) (UpdateBrandingResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdateBrandingInput` | 更新站名/logo/图标/主题色/默认分享图的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateBrandingResult` | 更新站名/logo/图标/主题色/默认分享图的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 核ready同站点图片资源与#RRGGBB；权限site.manage；合同未写明的资源引用失败关闭
2. 品牌logo/favicon/分享图复用首页home_reference_decisions与acquire/confirm/release及失败封存；来源对象为HashBytes(site-config.branding.v1|siteId)，不与首页来源混用；空引用仍走空pin
3. CommitConfigHead同一事务写CAS、历史、回执与outbox；confirm阶段由既有首页引用恢复循环收敛

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：不可变配置文件、COMMITTED journal、receipt/outbox
- 授权：site.manage；所有业务写入CheckWriteEpoch
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.CommitConfigHead`、`media:acquireResourceReferences`、`media:confirmResourceReferences`、`site-config.ResolveConfigOperation`

## site-config.UpdateFooter

- 级别：约束
- 文件：`modules/site-config/internal/footer_service.go`
- 作用：更新页脚版权/自定义链接/ICP备案/源码链接

```go
func UpdateFooter(ctx context.Context, op OperationContext, input UpdateFooterInput) (UpdateFooterResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdateFooterInput` | 更新页脚版权/自定义链接/ICP备案/源码链接的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateFooterResult` | 更新页脚版权/自定义链接/ICP备案/源码链接的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 校验链接scheme与ICP号纯文本
2. sourceCodeURL默认显示且可获得对应AGPL源代码
3. 写config版本CAS提交并发翻译/历史事件

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：不可变配置文件、COMMITTED journal、receipt/outbox
- 授权：site.manage；所有业务写入CheckWriteEpoch
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.CommitConfigHead`、`site-config.PublishConfigEvent`、`site-config.ResolveConfigOperation`

## site-config.UpdateBanner

- 级别：约束
- 文件：`modules/site-config/internal/banner_service.go`
- 作用：保存单条可关闭公告横幅

```go
func UpdateBanner(ctx context.Context, op OperationContext, input UpdateBannerInput) (UpdateBannerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdateBannerInput` | 保存单条可关闭公告横幅的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateBannerResult` | 保存单条可关闭公告横幅的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 核对text/link/time window；新条生成bannerId，同条编辑保留dismiss identity
2. WriteAnnouncementRevision后同事务换currentBanner/config revision/visibilityRevision
3. 登记翻译/历史镜像事件，按时间窗公开；浏览器localStorage关闭同一条不再显示

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：不可变配置文件、COMMITTED journal、receipt/outbox
- 授权：site.manage；所有业务写入CheckWriteEpoch
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.ValidateAnnouncementBody`、`site-config.WriteAnnouncementRevision`、`site-config.CommitAnnouncementHead`、`site-config.PublishConfigEvent`、`site-config.ResolveConfigOperation`

## site-config.WriteConfigRevision

- 级别：参考
- 文件：`modules/site-config/internal/config_files.go`
- 作用：写权威版本化站点设置文件

```go
func WriteConfigRevision(ctx context.Context, op OperationContext, input WriteConfigRevisionInput) (WriteConfigRevisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `WriteConfigRevisionInput` | 写权威版本化站点设置文件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteConfigRevisionResult` | 写权威版本化站点设置文件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 配置持久IO是独立权威边界：从文件系统根通过os.Root逐段Lstat取得目录能力，拒绝目录链链接/非目录；读取或复用既有对象/journal/head仅接受正规文件，O_NOFOLLOW/O_NONBLOCK打开并前后核文件身份；配置及journal规范JSON读写均≤16MiB，LimitReader阻止并发增长越界
2. 输入属于本域私有受信配置；持久对象采用ConfigDocument(schemaVersion=1,config)规范JSON；按生成SiteConfig schema及文件hash/version校验；未知schema失败关闭
3. active/settings/<siteId>/<revisionId>.json同FS临时写fsync后Link排他发布，再fsync父目录；已有同revision同字节复用、异内容INTEGRITY_ERROR；head与journal通过临时写fsync/rename/fsyncparent发布
4. 独立原子journal/config/<revisionId>.prepared.json保存PREPARED及ObjectRef；未有COMMITTED与已完成SQL水位不得公开；不包含secret/账号/可信代理key

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：不可变配置文件和PREPARED文件journal；不直接变更公开水位或写outbox
- 授权：私有config写入
- 并发：revision文件不可覆盖
- 幂等：同revision同hash重用，异hash拒绝
- 依赖：无

## site-config.CommitConfigHead

- 级别：参考
- 文件：`modules/site-config/internal/config_files.go`
- 作用：提交配置权威head与日志

```go
func CommitConfigHead(ctx context.Context, op OperationContext, input CommitConfigHeadInput) (CommitConfigHeadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CommitConfigHeadInput` | 提交配置权威head与日志的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitConfigHeadResult` | 提交配置权威head与日志的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 持SQLite写锁后先核同actor/site/epoch/requestHash的已完成回执：要求精确COMMITTED与原权威对象一致，直接返回原配置，即使当前水位已更高；不得用旧修订重写最新head。仅新提交继续CAS/维护检查
2. 先取得active SQLite写锁再做跨进程CAS；已持久COMMITTED决定先于SQL占有目标修订，禁止另一revisionId抢占同水位；核对PREPARED、权威对象hash/schema、原始请求actor/site/epoch/hash回执及expectedRevision；CheckWriteEpoch拒绝新维护期提交
3. 先持久不可变COMMITTED文件决定（含CAS、原始请求及变更组），再原子更新settings/site/head.json；随后同一SQLite FULL事务写索引/水位/回执/platform outbox，初始化阶段变为config_committed；文件决定已落盘后任何中断只允许补完，不能退回或改摘要
4. SQL事务完成不直接等同初始化完成；Resume另事务写completed与initialized=true。读取端必须核对head、COMMITTED和已完成权威水位；启动RecoverConfigJournal补完文件决定，不声称SQL原子覆盖文件系统
5. head与journal读写统一使用已核对目录能力，拒绝symlink/特殊文件；同修订EEXIST复用必须重新有界读取正规文件核字节，不直接ReadFile跟随链接

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久决定、权威对象或回执不一致，失败关闭（不可重试）
- `CONFLICT`：operationId被不同主体或请求复用（不可重试）
- `LOCKED`：运行状态处于维护或屏障阶段（不可重试）

- 副作用：不可变配置文件、COMMITTED journal、receipt/outbox
- 授权：私有config提交
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：原始actor/site/op/业务hash与COMMITTED决定固定；同键同内容补完或返回原配置，异内容CONFLICT/INTEGRITY_ERROR
- 依赖：`site-config.WriteConfigRevision`、`site-config.PublishConfigEvent`、`site-config.CheckWriteEpoch`、`site-config.checkHomeConfigCommit`、`site-config.markHomeConfigCommitted`

## site-config.RecoverConfigJournal

- 级别：参考
- 文件：`modules/site-config/internal/config_files.go`
- 作用：启动前恢复settings权威文件提交决定

```go
func RecoverConfigJournal(ctx context.Context, op OperationContext, input RecoverConfigJournalInput) (RecoverConfigJournalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RecoverConfigJournalInput` | 恢复正文文件与提交日志的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverConfigJournalResult` | 恢复正文文件与提交日志的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 只接受本域受控启动；按修订排序严格读取原子COMMITTED文件决定，要求epoch一致、修订连续且PREPARED/正文hash/schema/原始回执一致
2. 逐条幂等补完SQL索引/水位/回执/outbox，再发布最后COMMITTED head；不得从SQL准备JSON制造配置文件；已提交决定缺权威文件失败关闭
3. 孤儿PREPARED写私有quarantine/config标记，不公开；仍有初始化commit决定的准备材料保留供Resume；完成后深度核对head与权威SQL水位再标恢复就绪
4. 枚举journal前核父目录链与config目录能力，匹配记录不得为symlink/特殊文件；正文、head、journal读取上限16MiB，使用同一正规文件IO边界，任何不符失败关闭

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `FORBIDDEN`：非本域受控启动调用配置恢复（不可重试）

- 副作用：配置head、SQL索引/水位/回执/outbox和孤儿隔离标记
- 授权：callerService=site-config且actor.kind=service；受控启动入口，不接受HTTP触发
- 并发：启动exclusive；运行时仅屏障safe执行
- 幂等：反复恢复不会使PREPARED变COMMITTED
- 依赖：`site-config.PublishConfigEvent`

## site-config.ReadConfigHistory

- 级别：约束
- 文件：`modules/site-config/internal/config_history.go`
- 作用：读取可回滚的站点设置历史

```go
func ReadConfigHistory(ctx context.Context, op OperationContext, input ReadConfigHistoryInput) (ReadConfigHistoryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadConfigHistoryInput` | 读取可回滚的站点设置历史的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadConfigHistoryResult` | 读取可回滚的站点设置历史的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 分页参数或游标不合法；VERSION_MISMATCH: dataEpoch已变化；INTEGRITY_ERROR: 历史journal或不可变文件损坏；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. site.manage检查
2. 从本域COMMITTED journal稳定分页；游标绑定主体、站点、代次与页大小
3. 返回不可变文件和作者元数据，损坏记录失败关闭；data git由data-protection镜像

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：分页参数或游标不合法（不可重试）
- `VERSION_MISMATCH`：dataEpoch已变化（不可重试）
- `INTEGRITY_ERROR`：历史journal或不可变文件损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：site.manage
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## site-config.RestoreConfigRevision

- 级别：约束
- 文件：`modules/site-config/internal/config_history.go`
- 作用：回滚首页/公告/品牌/页脚历史

```go
func RestoreConfigRevision(ctx context.Context, op OperationContext, input RestoreConfigRevisionInput) (RestoreConfigRevisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RestoreConfigRevisionInput` | 回滚首页/公告/品牌/页脚历史的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RestoreConfigRevisionResult` | 回滚首页/公告/品牌/页脚历史的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 核历史COMMITTED文件hash与当前epoch；只保留当前siteId/baseLanguage/initialized，不伪造旧快照
2. 用media readAsset重核首页与品牌资源；缺失或未就绪整单拒绝并列出引用角色；无确认字段，不获取新pin
3. 生成新config版本而不改旧历史；公告正文只复用真实存在的文件；CAS提交并发翻译/历史镜像

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：不可变配置文件、COMMITTED journal、receipt/outbox
- 授权：site.manage；所有业务写入CheckWriteEpoch
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config.ReadConfigHistory`、`site-config.WriteConfigRevision`、`site-config.CommitConfigHead`、`media:readAsset`、`site-config.ResolveConfigOperation`

## site-config.ListResourceReferences

- 级别：约束
- 文件：`modules/site-config/internal/resource_reference_reader.go`
- 作用：向媒体返回站点设置权威资源引用快照

```go
func ListResourceReferences(ctx context.Context, op OperationContext, input ListResourceReferencesInput) (ListResourceReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ListResourceReferencesInput` | 向媒体返回站点设置权威资源引用快照的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListResourceReferencesResult` | 向媒体返回站点设置权威资源引用快照的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 从COMMITTED config head与保留history files扫描所有完整asset refs，包含hero背景/poster及branding，不能只扫当前home
2. 同水位分类current/history，sourceService=site-config、sourceRevisionId=config不可变文件
3. 签名cursor绑定site/watermark，分页未完全media不可解释为零引用；不扫描任何其他域目录

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：service:media/data-protection或site.manage
- 并发：不可变COMMITTED文件一致水位读取
- 幂等：只读
- 依赖：无

## site-config.main

- 级别：约束
- 文件：`modules/site-config/cmd/site-config/main.go`
- 作用：独立进程可执行入口

```go
func main()
```

**输入**

无

**输出**

无

**步骤**

1. 读取--config/--data-dir/--listen及环境覆盖，严格解析ServiceConfig；配置错误安全stderr后exit2
2. signal.NotifyContext注册SIGINT/SIGTERM；调用BuildApplication，失败exit1且只输出errorCode
3. 调用Serve；正常关闭或signal后以30秒context调用Shutdown；安全退出0，无法安全关闭exit1且保留journal

**错误**

- `VALIDATION` / HTTP 400：配置未知字段/越界目录/非法端点（不可重试）
- `INTERNAL` / HTTP 500：装配或HTTP运行失败（不可重试）

- 副作用：打开本域资源、监听HTTP；关闭按持久安全点
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：一个进程一个Application；OS信号处理只触发一次drain
- 幂等：进程重启先journal恢复；不重放已付费任务
- 依赖：`site-config.BuildApplication`、`site-config.Serve`、`site-config.Shutdown`、`platform.RunService`

## site-config.BuildApplication

- 级别：约束
- 文件：`modules/site-config/internal/app/wire.go`
- 作用：装配本域存储、客户端、函数路由与worker

```go
func BuildApplication(ctx context.Context, config ServiceConfig) (*Application, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `config` | `ServiceConfig` | 已严格解析配置 | module固定本域；secret只ref；依赖只HTTP |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `application` | `*Application` | 只返回完整装配后的内部容器 | 失败返回nil，已打开资源逆序关闭 |
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 验证module/listen/data目录/secretRef；建立本域控制区和业务数据区；portal不虚构业务DB
2. 先读control区恢复日志与site-config runtime：phase为restore_switching且本域有该restoreId的staged目录、尚未换入时，把active改名为restore/<restoreId>/before、staged改名为active并fsync父目录后写control日志；phase为restore_rolling_back且已换入时反向换回；换目录后丢弃旧dataEpoch未转发outbox、租约与可重建缓存。随后打开active区DB运行schema迁移与文件journal恢复；control区不随恢复替换
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；维护期间只开放健康/就绪与控制管理接口、不启动worker；本次启动完成换目录或换回时调用site-config:registerActivation登记回执；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源
6. 构造期仅收敛本地journal/COMMITTED与首页intent，不发外部confirm/release；RecoverHomeReferences只装配为BackgroundTasks闭包，BuildApplication返回成功后由统一RunService启动公共恢复循环；退出先取消和Drain、等后台退出再Shutdown关闭DB
7. HTTP装配复用platform.NewServiceHandler与NewIdentityAuthorizer，固定caller及纯service actor子白名单，当前identity权威只校验用户分支；维护仅显式控制操作开放。全部OpenAPI操作必须有真实handler，剩余用例未实现时装配失败并逆序关闭，不填占位成功
8. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config.OpenRuntimeStore`、`site-config.OpenActiveStore`、`site-config.RegisterActivation`、`site-config.RecoverAnnouncementJournal`、`site-config.RelayOutbox`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewRequestValidator`、`site-config.ConfigureInitialization`、`site-config.RecoverConfigJournal`、`site-config.ResumeInitialization`、`site-config.ReadInitializationStatus`、`identity:checkAuthorization`、`platform.NewServiceHandler`、`platform.NewIdentityAuthorizer`、`site-config.ConfigureHome`、`site-config.RecoverHomeReferenceState`、`site-config.BackgroundTasks`、`site-config.RecoverHomeReferences`、`site-config.RuntimeStore.ReadRuntimeView`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）；已实现的config/init/runtime HTTP适配与固定caller策略在transport装配；Home/快照/屏障等未完成时维持blocked

## site-config.Serve

- 级别：约束
- 文件：`modules/site-config/internal/transport/server.go`
- 作用：运行独立HTTP/JSON/SSE服务

```go
func Serve(ctx context.Context, app *Application) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 仅HTTP启动net/http.Server，ReadHeaderTimeout=5s、MaxHeaderBytes=64KiB；写deadline由各路由决定，SSE不用统一短WriteTimeout
2. 启动本域worker循环，匿名Health/Ready优先可访问；普通路由执行权限/epoch/maintenance门禁，SSE响应X-Accel-Buffering:no
3. 运行中每5秒读取site-config runtime；phase进入restore_switching或restore_rolling_back且本域有该restoreId的staged或已换入目录时，停止新请求、在途操作到安全点后以退出码75退出，由进程守护重启并在BuildApplication换目录
4. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

**错误**

- `INTERNAL` / HTTP 500：监听地址冲突或HTTP server不可恢复错误（不可重试）
- `CONFLICT` / HTTP 409：同Application重复Serve启动（不可重试）

- 副作用：监听本域HTTP端口、服务请求和本域worker
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：只有一Serve调用；每请求取消独立，worker只本域
- 幂等：重复Serve返回CONFLICT语义，不开第二监听
- 依赖：`site-config.BuildApplication`、`site-config.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## site-config.Shutdown

- 级别：约束
- 文件：`modules/site-config/internal/transport/server.go`
- 作用：停止入口并使在途操作到可恢复安全点

```go
func Shutdown(ctx context.Context, app *Application) error
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. CAS lifecycle ready→draining，拒新写/claim/paid步骤并关闭SSE接受新订阅
2. 取消本域worker的新步骤；等待已有DB事务、文件journal、模型可结算阶段安全确认；不能因超时假定供应方免费或删除paid refs
3. http.Server.Shutdown等在途响应；flush本域outbox/journal、close DB与文件句柄；正常lifecycle closed
4. deadline未安全返回DEADLINE_EXCEEDED并保留完整journal/未决费用引用，启动恢复收敛；不清掉活跃控制记录

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：30秒期限仍有未安全在途步骤（可重试）
- `INTERNAL` / HTTP 500：flush/close失败（不可重试）

- 副作用：本域drain、资源关闭、durable恢复日志
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：sync.Once启动drain；其他调用等待同一次结果
- 幂等：重复Shutdown返回同一次关闭结果；已closed成功
- 依赖：`site-config.Drain`
- 其他依赖说明：本域barrier安全点函数；本域journal/outbox flush

## site-config.Health

- 级别：约束
- 文件：`modules/site-config/internal/transport/server.go`
- 作用：GET /healthz普通HTTP存活检查

```go
func Health(ctx context.Context, app *Application) (HealthStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `status` | `HealthStatus` | 公共liveness安全字段 | 仅module/version/status/capabilities |
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 仅读本进程lifecycle与HTTP循环是否存活；不查询远程依赖/模型/数据库
2. 存活且未closed返回200 HealthStatus；closed或server不可用返回503；不泄漏路径、端点或secret

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：进程已closed或服务循环不可用（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：无锁或原子读取lifecycle；不进行外部调用
- 幂等：GET只读重复安全
- 依赖：无

## site-config.Ready

- 级别：约束
- 文件：`modules/site-config/internal/transport/server.go`
- 作用：GET /readyz普通HTTP就绪检查

```go
func Ready(ctx context.Context, app *Application) (ServiceReadiness, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 进程/请求取消和deadline | Go标准库；不携敏感正文到日志 |
| `app` | `*Application` | 本域已装配应用句柄 | x-internal；HTTP不能提供或序列化 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `readiness` | `ServiceReadiness` | 本域启动/恢复就绪与公开门禁状态 | ready不等于维护已解除，publicRequestsAllowed必须显式false |
| `error` | `error` | 成功nil；失败公共Problem或进程安全退出原因 | 不把secret/路径凭证/IP写detail |

**步骤**

1. 确认配置/本域schema/journal/control区与服务身份信任锚完整并在lifecycle ready或维护控制ready；返回当前active数据代次与本域恢复回执状态
2. 已初始化时核activeDataEpoch等于runtime目标代次；关键运行权威不可达或旧epoch返回503；site-config本域读取自身权威
3. 维护中本域已换入目标数据可返回200 control-ready但publicRequestsAllowed=false，使全域回执检查不会和维护门禁互相等待；所有域完成后runtime解除维护才开放
4. 未初始化可仅带初始化能力ready，activeDataEpoch=null；无AI渠道不使基础内容能力失败，能力缺项显式呈现

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：关键依赖/本域journal未恢复/epoch未匹配（可重试）

- 副作用：无
- 授权：permission：anonymous；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测
- 幂等：GET只读重复安全
- 依赖：`site-config.ValidateActiveReady`
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## site-config.BackgroundTasks

- 级别：参考
- 文件：`modules/site-config/internal/transport/server.go`
- 作用：只返回已装配后台闭包，统一模板完整构造成功后启动，取消Drain后等后台退出再关库

```go
func (a *Application) BackgroundTasks() []platform.BackgroundTask
```

**输入**

无

**输出**

无

**步骤**

1. 只返回已装配后台闭包，统一模板完整构造成功后启动，取消Drain后等后台退出再关库

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：`platform.RunRecoveryLoop`、`site-config.RecoverHomeReferences`、`site-config.homeRecoveryOperation`

## site-config.Drain

- 级别：参考
- 文件：`modules/site-config/internal/transport/server.go`
- 作用：只停止新HTTP并等待在途请求，不关闭DB；共享30秒退出预算

```go
func (a *Application) Drain(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 只停止新HTTP并等待在途请求，不关闭DB；共享30秒退出预算

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.homeRecoveryOperation

- 级别：参考
- 文件：`modules/site-config/internal/transport/server.go`
- 作用：为受信后台构造当前打开epoch与不超过ctx/30秒的工作上下文

```go
func (a *Application) homeRecoveryOperation(ctx context.Context) (OperationContext, time.Time)
```

**输入**

无

**输出**

无

**步骤**

1. 为受信后台构造当前打开epoch与不超过ctx/30秒的工作上下文

**错误**

- 无

- 副作用：无
- 授权：只装配本域受信参数或在已授权调用链内工作；不重复入口schema/服务验签
- 并发：受信启动单次或本域settings/config互斥与SQL CAS；纯函数不查ctx
- 幂等：保留原sourceOperationId/候选材料与完整digest；未知状态失败关闭
- 依赖：无

## site-config.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/site-config/internal/snapshot_archive.go`
- 作用：受控读取本域已完成且精确引用匹配的快照归档流

```go
func ReadSnapshotArchive(ctx context.Context, op OperationContext, input SnapshotArchiveReadInput) (SnapshotArchiveReadResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 流取消与既有截止 | 透传IO |
| `op` | `OperationContext` | 统一入口可信上下文 | data-protection服务及superadmin委托 |
| `input` | `SnapshotArchiveReadInput` | 精确完成归档引用 | 严格生产方schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotArchiveReadResult` | 只读完成tar流 | 不暴露磁盘路径 |
| `error` | `error` | 未知或不匹配拒绝 | 不含秘密或原路径 |

**步骤**

1. 仅接受data-protection服务及已验证superadmin委托，当前维护backup/restore屏障与本域已完成归档绑定；不把匿名下载或任意URL转为文件读。
2. 从本域固定owner/version=1/mediaType=application/x-tar与snapshotId/sha256/byteSize构造期望ObjectRef，委托公共SnapshotManager.OpenArchive核真实complete记录和完整hash后输出。
3. 只有完整只读流，不写源数据、不删除历史归档；取消透传下游IO。

**错误**

- `FORBIDDEN`：调用方不是data-protection或缺superadmin委托（不可重试）
- `LOCKED`：runtime阶段或restoreId不符，或本域屏障未safe（不可重试）
- `INTEGRITY_ERROR`：归档hash、owner或manifest不一致；临时目录已删除（不可重试）
- `UNSUPPORTED_SCHEMA`：快照schema高于本版本可迁移范围（不可重试）
- `CONFLICT`：同restoreId已暂存不同checksum（不可重试）
- `DEPENDENCY_UNAVAILABLE`：拉取归档失败或磁盘空间不足；临时目录已删除（可重试）

- 副作用：读取本域已完成归档字节
- 授权：permission：superadmin；rule：仅data-protection服务身份加已验证superadmin委托；绑定本域精确完成归档与维护屏障
- 并发：只读流，接收方仍核完整ObjectRef/hash/EOF
- 幂等：同完整引用只读复用，不生成或重算归档
- 依赖：`platform.SnapshotManager.OpenArchive`
