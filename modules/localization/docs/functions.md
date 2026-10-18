# localization 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 132 个函数：约束 48 个，参考 84 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## localization.LoadRuntime

- 级别：参考
- 文件：`modules/localization/internal/runtime/bootstrap.go`
- 作用：启动本地化服务

```go
func LoadRuntime(ctx context.Context, op OperationContext, input LoadRuntimeInput) (LoadRuntimeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `LoadRuntimeInput` | 封闭业务输入；逐字段见types $defs/LoadRuntimeInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadRuntimeResult` | 成功结果；逐字段见types $defs/LoadRuntimeResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 检查语言字典schema
2. 校验默认语言唯一
3. 加载术语代数
4. 注册旧租约恢复
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：打开私有存储不自动翻译
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.All`、`localization.recoverJournal`

## localization.recoverJournal

- 级别：参考
- 文件：`modules/localization/internal/runtime/bootstrap.go`
- 作用：本域参考支持 recoverJournal；精确输入输出以实际Go签名为准。

```go
func (s *Store) recoverJournal() error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.Generation`

## localization.Root

- 级别：参考
- 文件：`modules/localization/internal/runtime/bootstrap.go`
- 作用：本域参考支持 Root；精确输入输出以实际Go签名为准。

```ts
func(s *Store)Root()string
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.ReserveObjectGeneration`

## localization.ReserveObjectGeneration

- 级别：参考
- 文件：`modules/localization/internal/runtime/bootstrap.go`
- 作用：本域参考支持 ReserveObjectGeneration；精确输入输出以实际Go签名为准。

```ts
func(s *Store)ReserveObjectGeneration(key string,minimum int)(int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：无

## localization.NormalizeLanguageTag

- 级别：参考
- 文件：`modules/localization/internal/language/tag.go`
- 作用：规范语言标识

```go
func NormalizeLanguageTag(ctx context.Context,op OperationContext,input NormalizeLanguageTagInput)(NormalizeLanguageTagResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 拒空、长度>35和下划线；使用已批准x/text/language解析BCP47，拒解析错误
2. 返回标准大小写/脚本/私用规范表示，规范结果仍限35字节
3. 方向按规范主子标签ar/he/fa/ur为rtl其余ltr；配置展示原文不改写

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.PutLanguage

- 级别：约束
- 文件：`modules/localization/internal/language/config.go`
- 作用：更新语言启用配置

```go
func PutLanguage(ctx context.Context, op OperationContext, input PutLanguageInput) (PutLanguageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutLanguageInput` | 封闭业务输入；逐字段见types $defs/PutLanguageInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutLanguageResult` | 成功结果；逐字段见types $defs/PutLanguageResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 拒绝空标签与长度>35；不改写大小写、脚本或私用段。第一个连字符前的原文与ar/he/fa/ur逐字相同则方向必须为rtl，其余标签必须为ltr
2. 使用NormalizeLanguageTag校验但保持原文展示/存储；拒另一原文键代表同一规范语言。HTTP平台入口即时核权限，内部只落实admin特殊主体要求。
3. profile只核标识声明，不探测模型，不调用档位目录
4. 默认语言不能disabled
5. 按expectedVersion做CAS保存；同事务写7天原回执与localization.language.changed，不创建翻译任务
6. 写入前核op.dataEpoch等于site-config当前active epoch和本域runtime_meta水位；local_barrier非released拒绝。缺少已核本站失败关闭，不用system代替
7. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）

- 副作用：语言配置事件；不直接发翻译任务
- 授权：权限 localization.languages；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.NormalizeLanguageTag`、`localization.rejectLanguageAlias`、`localization.acceptStoredTag`

## localization.SetLanguageEnabled

- 级别：约束
- 文件：`modules/localization/internal/language/config.go`
- 作用：启停公开语言

```go
func SetLanguageEnabled(ctx context.Context, op OperationContext, input SetLanguageEnabledInput) (SetLanguageEnabledResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SetLanguageEnabledInput` | 封闭业务输入；逐字段见types $defs/SetLanguageEnabledInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SetLanguageEnabledResult` | 成功结果；逐字段见types $defs/SetLanguageEnabledResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 默认语言停用拒绝
2. enabled保存状态
3. disabled只把本域该语言queued/running/review任务标为cancelled；不删除人工译文，不调用模型
4. 保留人工译文历史
5. 同事务写7天原回执与localization.language.changed；核代次水位和local_barrier，缺少已核本站失败关闭
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
7. 启用且automatic=true时必须来自ApproveLanguagePlan的已批准计划；直接调用只能启用为手动翻译

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）

- 副作用：配置变化事件；本域未完成任务改为cancelled；不删除人工文本，不调用jobs
- 授权：权限 localization.languages；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.rejectLanguageAlias

- 级别：参考
- 文件：`modules/localization/internal/language/config.go`
- 作用：本域参考支持 rejectLanguageAlias；精确输入输出以实际Go签名为准。

```go
func rejectLanguageAlias(ctx context.Context,tx *sql.Tx,tag string)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.NormalizeLanguageTag`

## localization.ListLanguages

- 级别：约束
- 文件：`modules/localization/internal/language/read.go`
- 作用：读取语言列表

```go
func ListLanguages(ctx context.Context, op OperationContext, input ListLanguagesInput) (ListLanguagesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ListLanguagesInput` | 封闭业务输入；逐字段见types $defs/ListLanguagesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListLanguagesResult` | 成功结果；逐字段见types $defs/ListLanguagesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 公共只启用语言
2. includeDisabled需localization.manage
3. 默认语言先其余tag排序
4. 禁止返回模型密钥
5. 读取前核op.dataEpoch等于site-config当前active epoch和本域水位；维护阶段或local_barrier非released拒绝。public-site纯服务仅允许includeDisabled=false；站点缺失或为system时失败关闭，不自报未签名站点
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
7. HTTP includeDisabled query默认false；public-site已验证纯服务仅允许false只读，管理员仍需原语言管理权限，未知状态503，不把缺译当语言未启用

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）

- 副作用：无
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.acceptStoredTag

- 级别：参考
- 文件：`modules/localization/internal/language/guard.go`
- 作用：本域参考支持 acceptStoredTag；精确输入输出以实际Go签名为准。

```go
func acceptStoredTag(tag string,direction gen.LanguageDirection)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.NormalizeLanguageTag`

## localization.HandlePublication

- 级别：约束
- 文件：`modules/localization/internal/events/publication.go`
- 作用：仅发布触发已配置翻译

```go
func HandlePublication(ctx context.Context, op OperationContext, input HandlePublicationInput) (HandlePublicationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `HandlePublicationInput` | 封闭业务输入；逐字段见types $defs/HandlePublicationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandlePublicationResult` | 成功结果；逐字段见types $defs/HandlePublicationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. inbox去重/epoch校验
2. 按enabled且automatic且profile已设语言
3. 固定源revision和各generation
4. 与jobs任务同事务
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：翻译任务outbox；未配置则skipped记录
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:createJob`、`content:validatePublications`、`platform.ClaimInbox`

## localization.CreateTranslation

- 级别：约束
- 文件：`modules/localization/internal/translation/create.go`
- 作用：申请源修订翻译

```go
func CreateTranslation(ctx context.Context, op OperationContext, input CreateTranslationInput) (CreateTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CreateTranslationInput` | 封闭业务输入；逐字段见types $defs/CreateTranslationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateTranslationResult` | 成功结果；逐字段见types $defs/CreateTranslationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 验证language启用和profile
2. content当前可见
3. 固定术语/memory/human代数
4. source/target/generation唯一排队
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `CONFLICT` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：创建派生任务不修改正文
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`memory:resolveContext`、`jobs:createJob`

## localization.ReadTranslationUnits

- 级别：约束
- 文件：`modules/localization/internal/translation/units.go`
- 作用：按块对齐读取原文与译文供对照编辑

```go
func ReadTranslationUnits(ctx context.Context, op OperationContext, input ReadTranslationUnitsInput) (ReadTranslationUnitsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ReadTranslationUnitsInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadTranslationUnitsResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. localization.manage
2. 经content:readPublishedRevision读取当前源块，与本域译文按blockId对齐
3. 标注人工保护与需复核单元；编辑经EditTranslationUnit提交
4. 平台签名keyset游标固定稳定actor/site/epoch/current source/target generation/glossary/limit，10分钟独立到期，不绑定变化的proof ExpiresAt；远程IO后精确水位复核，未知/变化STALE_RESULT。

**错误**

- `FORBIDDEN`：权限不足（不可重试）

- 副作用：无
- 授权：permission：localization.manage；rule：后台管理员按登记权限
- 并发：只读快照或CAS
- 幂等：读取幂等；写入按operationId
- 依赖：`content:readPublishedRevision`、`localization.SplitTranslationUnits`、`localization.Generation`、`localization.ReadData`、`localization.RequireAdministrator`、`localization.StorageError`、`localization.ReadObject`、`localization.FetchCurrentPublishedSnapshot`、`localization.CheckPublication`、`localization.CursorKey`、`platform.SignCursor`、`platform.VerifyCursor`

## localization.BuildTranslationPrompt

- 级别：参考
- 文件：`modules/localization/internal/translation/prompt.go`
- 作用：构造有界翻译请求

```go
func BuildTranslationPrompt(ctx context.Context,op OperationContext,input BuildTranslationPromptInput)(BuildTranslationPromptResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 同文档按顺序批默认1000预计输出tokens，各批尽量均衡且优先标题边界
2. 整篇原文固定prefix仅本批标签块翻译，模型窗口不足明确缩上下文
3. 前批译文+相邻译文+section title+仅本批出现terms
4. 输出blockId完整和格式标签不变。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无
- 其他依赖说明：2026-10-18内部算法实际字段已同步types.json；见workflows第二组，旧缺来源/重试状态不得猜测。

## localization.RunTranslationBatch

- 级别：参考
- 文件：`modules/localization/internal/translation/execute.go`
- 作用：执行统一网关翻译

```go
func RunTranslationBatch(ctx context.Context, op OperationContext, input RunTranslationBatchInput) (RunTranslationBatchResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RunTranslationBatchInput` | 封闭业务输入；逐字段见types $defs/RunTranslationBatchInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RunTranslationBatchResult` | 成功结果；逐字段见types $defs/RunTranslationBatchResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验cancel/lease/source计划
2. costClass/background可信继承
3. localization.InvokeModel translation
4. 解析候选不直接写target
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG费用；保留调用ID和候选暂存
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.InvokeModel`

## localization.ExecuteTranslation

- 级别：约束
- 文件：`modules/localization/internal/translation/execute.go`
- 作用：执行一篇文档到一种语言的增量翻译

```go
func ExecuteTranslation(ctx context.Context, op OperationContext, input JobExecutionInput) (JobExecutionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `JobExecutionInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `JobExecutionResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. FetchTranslationSource读取固定发布修订；SplitTranslationUnits按稳定blockId切分；PlanIncrementalTranslation选出待译单元，先ReuseTranslationMemory复用≥20字同源译文
2. memory:resolveContext取localization.document任务记忆；ResolveGlossary固定术语快照；BuildTranslationPrompt按约1000输出token分批
3. 每批RunTranslationBatch→ValidateTranslationOutput；失败按PlanTranslationFallback对半→逐块→保留原文；CheckTerminologyCompliance不符时带提醒重译一次；开启反思时ReflectTranslation
4. CommitTranslationBatch按源修订/术语/人工代数CAS写入，protected单元跳过；首次翻译该文档时ExtractAutomaticGlossary
5. 全部单元终态后CompleteTranslation写manifest并登记translation.ready

**错误**

- `STALE_RESULT`：源修订或人工代数已变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：content或网关不可用（可重试）

- 副作用：写译文与进度；模型费用经网关
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：同document/target一次一个任务；所有写入核lease与代数
- 幂等：按source/target/generation唯一
- 依赖：`localization.FetchTranslationSource`、`localization.SplitTranslationUnits`、`localization.PlanIncrementalTranslation`、`localization.ReuseTranslationMemory`、`memory:resolveContext`、`localization.ResolveGlossary`、`localization.BuildTranslationPrompt`、`localization.RunTranslationBatch`、`localization.ValidateTranslationOutput`、`localization.PlanTranslationFallback`、`localization.CheckTerminologyCompliance`、`localization.ReflectTranslation`、`localization.CommitTranslationBatch`、`localization.ExtractAutomaticGlossary`、`localization.CompleteTranslation`

## localization.ValidateTranslationOutput

- 级别：参考
- 文件：`modules/localization/internal/translation/validate.go`
- 作用：校验模型输出格式保护

```go
func ValidateTranslationOutput(ctx context.Context,op OperationContext,input ValidateTranslationOutputInput)(ValidateTranslationOutputResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. blockId齐全唯一且没有未知ID
2. 编号inline标签完整有序/数量正确还原原URL及code
3. 足够长原文≥20字译文完全相同且不同语言则失败
4. 失败明确触发对半→逐块降级，最终失败用原文并后台显示。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.CheckInlineTokens`、`localization.NormalizeLanguageTag`、`localization.VisibleCharacterCount`
- 其他依赖说明：2026-10-18内部算法实际字段已同步types.json；见workflows第二组，旧缺来源/重试状态不得猜测。

## localization.CheckInlineTokens

- 级别：参考
- 文件：`modules/localization/internal/translation/validate.go`
- 作用：本域参考支持 CheckInlineTokens；精确输入输出以实际Go签名为准。

```go
func CheckInlineTokens(source,target string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.ParseSlotValues`

## localization.VisibleCharacterCount

- 级别：参考
- 文件：`modules/localization/internal/translation/validate.go`
- 作用：本域参考支持 VisibleCharacterCount；精确输入输出见实际Go签名。

```go
func VisibleCharacterCount(text string)int
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.CommitTranslationBatch

- 级别：参考
- 文件：`modules/localization/internal/translation/commit.go`
- 作用：提交翻译防旧覆盖

```go
func CommitTranslationBatch(ctx context.Context, op OperationContext, input CommitTranslationBatchInput) (CommitTranslationBatchResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CommitTranslationBatchInput` | 封闭业务输入；逐字段见types $defs/CommitTranslationBatchInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitTranslationBatchResult` | 成功结果；逐字段见types $defs/CommitTranslationBatchResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 当前sourceRevision/epoch仍匹配
2. CAS术语/memory/target/human代数
3. protected单元跳过
4. 一事务写machine文本/进度
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写派生译文；CAS失败标superseded
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`

## localization.CompleteTranslation

- 级别：参考
- 文件：`modules/localization/internal/translation/complete.go`
- 作用：完成派生翻译manifest

```go
func CompleteTranslation(ctx context.Context, op OperationContext, input CompleteTranslationInput) (CompleteTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CompleteTranslationInput` | 封闭业务输入；逐字段见types $defs/CompleteTranslationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CompleteTranslationResult` | 成功结果；逐字段见types $defs/CompleteTranslationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 所有计划单元终态
2. protected且源变需review
3. content当前可见再次检查
4. manifest ready并持久事件
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：translation.ready事件；不更新content正文
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`

## localization.ReadTranslation

- 级别：约束
- 文件：`modules/localization/internal/translation/read.go`
- 作用：公开读取当前译文

```go
func ReadTranslation(ctx context.Context, op OperationContext, input ReadTranslationInput) (ReadTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadTranslationInput` | 封闭业务输入；逐字段见types $defs/ReadTranslationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadTranslationResult` | 成功结果；逐字段见types $defs/ReadTranslationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. enabled语言和content当前公开源验证
2. 按当前blockId匹配，新译文未就绪显示上一generation译文但标对应源hash
3. 新块无译文显示原文且partial顶栏目标语言提示
4. 人工源变保留译文review，撤回文档全部禁止交付。
5. 返回兼容可选body与bodyHash：完整还原目标树绑定当前source publication/targetLanguage/generation，canonical目标hash另算，目标正文≤16MiB；返回前二次核content visibility及本地manifest/glossary。
6. 人工/保护旧字段按最深真实blockId/rowId/cellId/itemId/columnId、节点类型与节点内部text/alt路径对应；只在固定/可译段形状一致时复用真实旧译值并重编号，固定URL/code取当前源。新增真实子节点原文partial/review并继承父保护，投影不改旧权威对象。归属/段形状无法证明或保护块只能source-only时失败关闭DEPENDENCY_UNAVAILABLE，不以原文冒充旧人工译文。
7. 即时fallback及新投影没有独立权威unit时省略fileRef，不写null或借用旧父对象作证明；持久化单元返回真实fileRef。body/bodyHash绑定本次精确source/target generation。
8. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
9. 公开units最多10000，与完整content树稳定节点上限及拆分一致；completed仅计无原文回退的实际单元，不超过total；partial为存在原文回退，review不等于fallback。
10. 同一稳定anchor内无ID inline字段/数组结构变化或全部可译slot消失时失败关闭，禁止content/0/text误匹配新插入文本。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `BUDGET_EXCEEDED` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 500：（不可重试）

- 副作用：只读不隐式启动翻译
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`localization.SplitTranslationUnits`、`localization.Generation`、`localization.EnabledLanguage`、`localization.ReadData`、`localization.StorageError`、`localization.ReadObject`、`localization.RestoreSourceTree`、`localization.ParseSlotValues`、`localization.LatestManifest`、`localization.FetchPublishedSnapshot`、`localization.CheckPublication`、`localization.BuildProtectedProjection`、`localization.ProjectProtectedUnit`

## localization.LatestManifest

- 级别：参考
- 文件：`modules/localization/internal/translation/read.go`
- 作用：本域参考支持 LatestManifest；精确输入输出见实际Go签名。

```go
func LatestManifest(ctx context.Context,tx *sql.Tx,op OperationContext,document,target string,completedOnly bool)(manifestWatermark,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.Generation`、`localization.StorageError`

## localization.CancelTranslation

- 级别：约束
- 文件：`modules/localization/internal/translation/cancel.go`
- 作用：取消翻译子链路

```go
func CancelTranslation(ctx context.Context, op OperationContext, input CancelTranslationInput) (CancelTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CancelTranslationInput` | 封闭业务输入；逐字段见types $defs/CancelTranslationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CancelTranslationResult` | 成功结果；逐字段见types $defs/CancelTranslationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 写取消标志
2. jobs CancelJob
3. MG cancelInference
4. 拒绝所有旧lease写入
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `NOT_FOUND` / HTTP 404：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：保留已有人工作品和已提交机翻；费用由MG处理
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:cancelJob`、`model-gateway:cancelInference`

## localization.PutGlossaryTerm

- 级别：约束
- 文件：`modules/localization/internal/glossary/write.go`
- 作用：人工维护术语

```go
func PutGlossaryTerm(ctx context.Context, op OperationContext, input PutGlossaryTermInput) (PutGlossaryTermResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutGlossaryTermInput` | 封闭业务输入；逐字段见types $defs/PutGlossaryTermInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutGlossaryTermResult` | 成功结果；逐字段见types $defs/PutGlossaryTermResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. BCP47和长度校验
2. 同scope原词唯一
3. CAS并generation++
4. 事件使受影响机翻stale不改protected单元
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写glossary及glossary.changed
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.HandleGlossaryChange`、`localization.Mutate`、`localization.RequireAdministrator`、`localization.NormalizeTerm`、`localization.StoreTerm`、`localization.Generation`

## localization.DeleteGlossaryTerm

- 级别：约束
- 文件：`modules/localization/internal/glossary/write.go`
- 作用：删除术语规则

```go
func DeleteGlossaryTerm(ctx context.Context, op OperationContext, input DeleteGlossaryTermInput) (DeleteGlossaryTermResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `DeleteGlossaryTermInput` | 封闭业务输入；逐字段见types $defs/DeleteGlossaryTermInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DeleteGlossaryTermResult` | 成功结果；逐字段见types $defs/DeleteGlossaryTermResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验权限
2. CAS软删除
3. generation++
4. 发布受影响范围
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：软删除保留审核记录
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.HandleGlossaryChange`、`localization.Mutate`、`localization.RequireAdministrator`、`localization.StorageError`、`localization.AdvanceGeneration`、`localization.PublishChange`、`localization.scanTerm`、`localization.Generation`

## localization.NormalizeTerm

- 级别：参考
- 文件：`modules/localization/internal/glossary/write.go`
- 作用：本域参考支持 NormalizeTerm；精确输入输出以实际Go签名为准。

```go
func NormalizeTerm(ctx context.Context,op OperationContext,t gen.GlossaryTerm)(gen.GlossaryTerm,error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.NormalizeLanguageTag`

## localization.StoreTerm

- 级别：参考
- 文件：`modules/localization/internal/glossary/write.go`
- 作用：本域参考支持 StoreTerm；精确输入输出以实际Go签名为准。

```go
func StoreTerm(ctx context.Context,tx *sql.Tx,op OperationContext,t gen.GlossaryTerm,expected int)(gen.GlossaryTerm,int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.StorageError`、`localization.AdvanceGeneration`、`localization.PublishChange`、`localization.Generation`

## localization.AdvanceGeneration

- 级别：参考
- 文件：`modules/localization/internal/glossary/write.go`
- 作用：本域参考支持 AdvanceGeneration；精确输入输出以实际Go签名为准。

```go
func AdvanceGeneration(ctx context.Context,tx *sql.Tx)(int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.StorageError`、`localization.Generation`

## localization.PublishChange

- 级别：参考
- 文件：`modules/localization/internal/glossary/write.go`
- 作用：本域参考支持 PublishChange；精确输入输出以实际Go签名为准。

```go
func PublishChange(ctx context.Context,tx *sql.Tx,op OperationContext,source,target string,generation int,ids []string,origin string)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.HandleGlossaryChange`、`localization.EmitReference`

## localization.ListGlossary

- 级别：约束
- 文件：`modules/localization/internal/glossary/read.go`
- 作用：管理术语列表

```go
func ListGlossary(ctx context.Context, op OperationContext, input ListGlossaryInput) (ListGlossaryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ListGlossaryInput` | 封闭业务输入；逐字段见types $defs/ListGlossaryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListGlossaryResult` | 成功结果；逐字段见types $defs/ListGlossaryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限检查
2. 按termId稳定分页
3. 固定首屏generation到游标
4. generation改变提示重新查询
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.NormalizeLanguageTag`、`localization.ReadData`、`localization.RequireAdministrator`、`localization.StorageError`、`localization.scanTerm`、`localization.Generation`

## localization.scanTerm

- 级别：参考
- 文件：`modules/localization/internal/glossary/read.go`
- 作用：本域参考支持 scanTerm；精确输入输出以实际Go签名为准。

```go
func scanTerm(row rowScanner)(gen.GlossaryTerm,error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.StorageError`、`localization.Generation`

## localization.Generation

- 级别：参考
- 文件：`modules/localization/internal/glossary/read.go`
- 作用：本域参考支持 Generation；精确输入输出以实际Go签名为准。

```go
func Generation(ctx context.Context,tx *sql.Tx)(int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.StorageError`

## localization.ResolveGlossary

- 级别：参考
- 文件：`modules/localization/internal/glossary/read.go`
- 作用：取得确定性术语快照

```go
func ResolveGlossary(ctx context.Context,op OperationContext,input ResolveGlossaryInput)(ResolveGlossaryResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 固定generation读取
2. document term优先global
3. 原词按长度降序避免短词抢占
4. 相同scope原词冲突拒绝
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.NormalizeLanguageTag`、`localization.ReadData`、`localization.StorageError`、`localization.scanTerm`、`localization.Generation`

## localization.ApplyApprovedGlossaryProposal

- 级别：约束
- 文件：`modules/localization/internal/glossary/approved.go`
- 作用：应用memory已批准提议

```go
func ApplyApprovedGlossaryProposal(ctx context.Context, op OperationContext, input ApplyApprovedGlossaryProposalInput) (ApplyApprovedGlossaryProposalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ApplyApprovedGlossaryProposalInput` | 封闭业务输入；逐字段见types $defs/ApplyApprovedGlossaryProposalInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApplyApprovedGlossaryProposalResult` | 成功结果；逐字段见types $defs/ApplyApprovedGlossaryProposalResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅memory纯服务断言认证其持久人工批准事实（approvedBy/revision/hash），不依赖原用户短期proof；本域继续核当前代次/屏障/expected术语代数/版本。
2. proposalId/hash去重
3. 命令同proposalId终身固定完整term、批准人/修订/hash、expected代数、站/根/epoch；终态重放返回原结果代数；不同内容CONFLICT。冲突持久rejected事件，不回调memory，不重试创建新词条。
4. expectedGlossaryGeneration匹配才CAS term
5. 发glossary.proposal.applied或rejected终态事件
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写术语或拒绝记录；不同步回调memory
- 授权：permission：service；rule：只接受memory服务身份对已持久人工批准命令的签名来源；人工授权在memory的当前memory.manage入口发生，执行不重用过期用户proof；同站/根/epoch，仍核当前本域维护与CAS。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`identity:checkAuthorization`、`localization.HandleGlossaryChange`、`localization.Mutate`、`localization.EmitReference`、`localization.StorageError`、`localization.NormalizeTerm`、`localization.StoreTerm`、`localization.Generation`

## localization.ValidateInterfaceCatalog

- 级别：参考
- 文件：`modules/localization/internal/interface/validate.go`
- 作用：校验界面字符串契约

```go
func ValidateInterfaceCatalog(ctx context.Context,op OperationContext,input ValidateInterfaceCatalogInput)(ValidateInterfaceCatalogResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. key必须在固定源catalog
2. placeholders与源逐字一致
3. 禁HTML与新增key
4. 规范hash
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.ReadData`、`localization.ValidateInSnapshot`

## localization.SourceCatalog

- 级别：参考
- 文件：`modules/localization/internal/interface/validate.go`
- 作用：本域参考支持 SourceCatalog；精确输入输出以实际Go签名为准。

```go
func SourceCatalog(ctx context.Context,tx *sql.Tx)(map[string]gen.InterfaceString,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.StorageError`、`localization.CheckString`

## localization.CheckString

- 级别：参考
- 文件：`modules/localization/internal/interface/validate.go`
- 作用：本域参考支持 CheckString；精确输入输出以实际Go签名为准。

```go
func CheckString(value gen.InterfaceString,source gen.InterfaceString)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：无

## localization.ValidateInSnapshot

- 级别：参考
- 文件：`modules/localization/internal/interface/validate.go`
- 作用：本域参考支持 ValidateInSnapshot；精确输入输出以实际Go签名为准。

```go
func ValidateInSnapshot(ctx context.Context,tx *sql.Tx,strings []gen.InterfaceString)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.SourceCatalog`、`localization.CheckString`

## localization.PutInterfaceString

- 级别：约束
- 文件：`modules/localization/internal/interface/write.go`
- 作用：人工编辑界面文本

```go
func PutInterfaceString(ctx context.Context, op OperationContext, input PutInterfaceStringInput) (PutInterfaceStringResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutInterfaceStringInput` | 封闭业务输入；逐字段见types $defs/PutInterfaceStringInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PutInterfaceStringResult` | 成功结果；逐字段见types $defs/PutInterfaceStringResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验固定key/placeholder
2. CAS
3. origin=human protected=true
4. 更新catalog generation与事件
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写UI字典不重编译业务JS
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.Mutate`、`localization.RequireAdministrator`、`localization.EmitReference`、`localization.StorageError`、`localization.ValidateInSnapshot`、`localization.EnabledLanguage`、`localization.ReadMerged`

## localization.TranslateInterfaceCatalog

- 级别：约束
- 文件：`modules/localization/internal/interface/translate.go`
- 作用：显式翻译界面缺项

```go
func TranslateInterfaceCatalog(ctx context.Context, op OperationContext, input TranslateInterfaceCatalogInput) (TranslateInterfaceCatalogResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `TranslateInterfaceCatalogInput` | 封闭业务输入；逐字段见types $defs/TranslateInterfaceCatalogInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TranslateInterfaceCatalogResult` | 成功结果；逐字段见types $defs/TranslateInterfaceCatalogResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 读取固定catalog generation与目标语言缺项，排除protected与sourceHash未变的key
2. 每批16个key经InvokeModel（localization.ui任务）输出严格schema
3. ValidateInterfaceCatalog校验占位符与HTML后CAS写machine译文，catalog generation++并登记interface-catalog.changed

**错误**

- `CONFLICT` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：jobs翻译任务；可能MG费用
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.InvokeModel`、`localization.ValidateInterfaceCatalog`

## localization.RequestInterfaceTranslation

- 级别：约束
- 文件：`modules/localization/internal/interface/translate.go`
- 作用：手动补译界面字符串缺项

```go
func RequestInterfaceTranslation(ctx context.Context, op OperationContext, input RequestInterfaceTranslationInput) (RequestInterfaceTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `RequestInterfaceTranslationInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RequestInterfaceTranslationResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. localization.manage权限且语言已启用、localization.ui档位已配置
2. 创建localization.ui任务（按language/catalog generation去重）

**错误**

- `UNSUPPORTED_CAPABILITY`：未配置翻译模型（不可重试）
- `VALIDATION`：语言未启用（不可重试）

- 副作用：创建翻译任务
- 授权：permission：localization.manage；rule：翻译管理员
- 并发：同语言同generation一个任务
- 幂等：去重键language+generation
- 依赖：`jobs:createJob`

## localization.ReadInterfaceCatalog

- 级别：约束
- 文件：`modules/localization/internal/interface/read.go`
- 作用：读取运行界面字典

```go
func ReadInterfaceCatalog(ctx context.Context, op OperationContext, input ReadInterfaceCatalogInput) (ReadInterfaceCatalogResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadInterfaceCatalogInput` | 封闭业务输入；逐字段见types $defs/ReadInterfaceCatalogInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadInterfaceCatalogResult` | 成功结果；逐字段见types $defs/ReadInterfaceCatalogResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. language启用
2. 按key合并目标与默认builtin
3. 插值缺失视INTEGRITY
4. 版本相同返回空strings/notModified
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `CONFLICT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.Generation`、`localization.ReadData`、`localization.StorageError`、`localization.SourceCatalog`、`localization.EnabledLanguage`、`localization.ReadMerged`

## localization.EnabledLanguage

- 级别：参考
- 文件：`modules/localization/internal/interface/read.go`
- 作用：本域参考支持 EnabledLanguage；精确输入输出以实际Go签名为准。

```go
func EnabledLanguage(ctx context.Context,tx *sql.Tx,tag string)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.NormalizeLanguageTag`、`localization.StorageError`

## localization.ReadMerged

- 级别：参考
- 文件：`modules/localization/internal/interface/read.go`
- 作用：本域参考支持 ReadMerged；精确输入输出以实际Go签名为准。

```go
func ReadMerged(ctx context.Context,tx *sql.Tx,op OperationContext,target string)([]gen.InterfaceString,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.StorageError`、`localization.SourceCatalog`、`localization.CheckString`

## localization.SuggestLanguage

- 级别：约束
- 文件：`modules/localization/internal/suggestion/select.go`
- 作用：给访客语言建议

```go
func SuggestLanguage(ctx context.Context, op OperationContext, input SuggestLanguageInput) (SuggestLanguageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SuggestLanguageInput` | 封闭业务输入；逐字段见types $defs/SuggestLanguageInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SuggestLanguageResult` | 成功结果；逐字段见types $defs/SuggestLanguageResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. explicitPreference或previouslyRejected返回none
2. key=browserLanguages/timeZone/country+enabledLanguageGeneration缓存
3. miss根task localization.suggest使用fast最多500output token且总5秒，timeout规则fallback
4. confidence≥0.8且建议语言不同才提示，不存IP。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无存储、无模型、无强制跳转
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.InvokeModel`

## localization.SaveLanguagePreference

- 级别：约束
- 文件：`modules/localization/internal/suggestion/preference.go`
- 作用：保存显式访客语言偏好

```go
func SaveLanguagePreference(ctx context.Context, op OperationContext, input SaveLanguagePreferenceInput) (SaveLanguagePreferenceResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SaveLanguagePreferenceInput` | 封闭业务输入；逐字段见types $defs/SaveLanguagePreferenceInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SaveLanguagePreferenceResult` | 成功结果；逐字段见types $defs/SaveLanguagePreferenceResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 核对可信actor.kind=visitor且subjectId等于visitorSubject；不接受服务身份代写任意伪名。
2. 在当前dataEpoch/屏障与本地事务内核对目标语言已启用，按BCP47规范标签保存，不改写语言配置显示原文。
3. 保存expiresAt=首次成功事务时刻+30天；幂等重放返回原值/原到期时刻，不读取或持久化国家、时区、浏览器列表、IP或浏览轨迹。
4. 偏好与七天同主体/函数/幂等键回执同事务写入；未知/禁用语言VALIDATION，SQL故障回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 500：（不可重试）

- 副作用：写偏好
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.Mutate`、`localization.EnabledLanguage`、`localization.StorageError`

## localization.NewPreferenceHandlers

- 级别：参考
- 文件：`modules/localization/internal/suggestion/preference.go`
- 作用：装配已核请求到访客语言偏好用例

```go
func NewPreferenceHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 复用JSONHandler装配saveLanguagePreference，不重复结构/服务身份校验。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.SaveLanguagePreference`、`localization.JSONHandler`

## localization.TranslateCachedAnswer

- 级别：约束
- 文件：`modules/localization/internal/cache-answer/create.go`
- 作用：接受QA精选翻译命令

```go
func TranslateCachedAnswer(ctx context.Context, op OperationContext, input TranslateCachedAnswerInput) (TranslateCachedAnswerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `TranslateCachedAnswerInput` | 封闭业务输入；逐字段见types $defs/TranslateCachedAnswerInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TranslateCachedAnswerResult` | 成功结果；逐字段见types $defs/TranslateCachedAnswerResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. callerService=qa且源为active普通或精选cache，不新增人工批准或模型配置权限门槛
2. 核对sourceHash和基准answerRevision，已配置根qa.cache-translate fast任务并分别取翻译任务记忆
3. 继承原qa rootOperationId/costClass及同一签名admissionPermit到每个MG调用，费用计相同session/network小时作用域
4. 创建localization.cache-answer任务由ExecuteCacheAnswerTranslation执行，完成自动ready，发cache-answer.translated，由QA自动保存对应语言答案
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写缓存译文任务；MG费用由background继承
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:createJob`、`memory:resolveContext`

## localization.ExecuteCacheAnswerTranslation

- 级别：约束
- 文件：`modules/localization/internal/cache-answer/create.go`
- 作用：把一条缓存答案翻译为目标语言

```go
func ExecuteCacheAnswerTranslation(ctx context.Context, op OperationContext, input JobExecutionInput) (JobExecutionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `JobExecutionInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `JobExecutionResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 读取命令固定的源答案hash与answerRevision；memory:resolveContext取qa.cache-translate任务记忆
2. 以继承的qa根操作、费用类别与准入许可调用InvokeModel（fast档），保留引用token
3. CommitCacheAnswerTranslation核对源版本后保存ready译文并登记cache-answer.translated

**错误**

- `STALE_RESULT`：源答案已变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：网关不可用（可重试）

- 副作用：写缓存答案译文；模型费用经网关
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：同cacheId/language一次一个
- 幂等：按cacheId/answerRevision/language唯一
- 依赖：`memory:resolveContext`、`localization.InvokeModel`、`localization.CommitCacheAnswerTranslation`

## localization.CommitCacheAnswerTranslation

- 级别：参考
- 文件：`modules/localization/internal/cache-answer/commit.go`
- 作用：保存精选翻译供QA审核

```go
func CommitCacheAnswerTranslation(ctx context.Context, op OperationContext, input CommitCacheAnswerTranslationInput) (CommitCacheAnswerTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `CommitCacheAnswerTranslationInput` | 封闭业务输入；逐字段见types $defs/CommitCacheAnswerTranslationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitCacheAnswerTranslationResult` | 成功结果；逐字段见types $defs/CommitCacheAnswerTranslationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 源cache answerRevision/hash/epoch匹配
2. 引用token和语言格式核验
3. 保存ready自动译文不加人工批准
4. 发cache-answer.translated引用事件供QA自动存localized答案。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写译文与outbox，不同步调用QA
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.InvalidateDocumentTranslations

- 级别：约束
- 文件：`modules/localization/internal/events/withdraw.go`
- 作用：处理源撤回

```go
func InvalidateDocumentTranslations(ctx context.Context, op OperationContext, input InvalidateDocumentTranslationsInput) (InvalidateDocumentTranslationsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `InvalidateDocumentTranslationsInput` | 封闭业务输入；逐字段见types $defs/InvalidateDocumentTranslationsInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InvalidateDocumentTranslationsResult` | 成功结果；逐字段见types $defs/InvalidateDocumentTranslationsResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. inbox去重
2. 更高可见性代数
3. manifest stale
4. 取消子任务并禁止公开读取
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：译文失活与取消；不删除人工历史
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:cancelJob`、`model-gateway:cancelInference`、`platform.ClaimInbox`

## localization.RecoverTranslationLeases

- 级别：参考
- 文件：`modules/localization/internal/runtime/recovery.go`
- 作用：恢复翻译任务

```go
func RecoverTranslationLeases(ctx context.Context, op OperationContext, input RecoverTranslationLeasesInput) (RecoverTranslationLeasesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RecoverTranslationLeasesInput` | 封闭业务输入；逐字段见types $defs/RecoverTranslationLeasesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverTranslationLeasesResult` | 成功结果；逐字段见types $defs/RecoverTranslationLeasesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 到期lease拒绝写
2. 固定source/current和人工代数
3. 旧任务superseded
4. MG未知调用不重复消费只读其状态
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：任务状态恢复
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:validatePublications`、`model-gateway:readInference`

## localization.PublishOutbox

- 级别：约束
- 文件：`modules/localization/internal/events/outbox.go`
- 作用：持久发布翻译事件

```go
func PublishOutbox(ctx context.Context, op OperationContext, input PublishOutboxInput) (PublishOutboxResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PublishOutboxInput` | 封闭业务输入；逐字段见types $defs/PublishOutboxInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishOutboxResult` | 成功结果；逐字段见types $defs/PublishOutboxResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 租约领批
2. 只发布不可变引用和版本
3. jobs ACK才标
4. 有限重试三次后留待下轮
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：at-least-once事件；无正文广播
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:publishEvent`、`platform.RunRelay`、`localization.PublishBatch`

## localization.AcquireBarrier

- 级别：约束
- 文件：`modules/localization/internal/protection/protocol.go`
- 作用：取得本地维护屏障

```go
func AcquireBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BarrierRequest` | 封闭业务输入；逐字段见types $defs/BarrierRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 成功结果；逐字段见types $defs/BarrierStatus | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验data-protection服务身份（用户授权已由data-protection核验）
2. 将本地control.db屏障draining并拒绝新业务写
3. 屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本
4. 导入必须恢复保护位与源ref，恢复切换完成前译文不能交付，不自动重翻。
5. 排空/终止worker到安全watermark后safe，deadline失败保持failed及拒写。
6. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：持久控制屏障；不纳入业务快照
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.EnterBarrier`

## localization.ReleaseBarrier

- 级别：约束
- 文件：`modules/localization/internal/protection/protocol.go`
- 作用：释放已授权维护屏障

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, input BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `BarrierRequest` | 封闭业务输入；逐字段见types $defs/BarrierRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 成功结果；逐字段见types $defs/BarrierStatus | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 校验相同barrierId/operationId与data-protection服务身份
2. 暂存目录不影响释放；切换只在进程重启时发生
3. runtime处于restore_switching或restore_rolling_back时不恢复业务worker，等待重启换目录
4. released后开放该epoch业务写。
5. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：更新本地control.db；不修改导入业务
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.ReleaseBarrier`

## localization.ExportSnapshot

- 级别：约束
- 文件：`modules/localization/internal/protection/protocol.go`
- 作用：导出安全watermark快照

```go
func ExportSnapshot(ctx context.Context, op OperationContext, input SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SnapshotExportRequest` | 封闭业务输入；逐字段见types $defs/SnapshotExportRequest | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 成功结果；逐字段见types $defs/SnapshotExportResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 验证safe屏障及dataEpoch
2. checkpoint本域私有数据并固定revisionWatermark
3. 屏障停止新翻译批次和人工写入，保留human/protected/generation及术语版本
4. 导入必须恢复保护位与源ref，恢复切换完成前译文不能交付，不自动重翻。
5. 写不可变ObjectRef及SHA256后返回SnapshotRef，失败删除本域临时输出不释放屏障。
6. 备份恢复控制只接受校验data-protection服务身份（用户授权已由data-protection核验），并校验本地safe屏障与op.dataEpoch；control区不随恢复替换。截止=min(ctx deadline,op.deadline,本函数120000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：创建本服务不可变快照对象
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：本域control区按barrierId/restoreId串行CAS；业务worker屏障排空，reader固定代数；active目录只在进程启动时换入或换回。
- 幂等：相同barrierId/restoreId+payload hash返回原控制回执；不同hash返回CONFLICT；暂存与丢弃均可安全重放。
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.Export`

## localization.StageSnapshot

- 级别：约束
- 文件：`modules/localization/internal/protection/protocol.go`
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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`、`platform.SnapshotManager.Stage`

## localization.DiscardSnapshot

- 级别：约束
- 文件：`modules/localization/internal/protection/protocol.go`
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
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.Discard`

## localization.LoadAdminOverview

- 级别：参考
- 文件：`modules/localization/ui/src/views/OverviewView.vue`
- 作用：加载本域后台概览

```ts
export async function loadAdminOverview(input: LoadAdminOverviewInput, signal: AbortSignal): Promise<LoadAdminOverviewResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `LoadAdminOverviewInput` | 本域封闭输入；每个字段规格见types.json $defs/LoadAdminOverviewInput | schema required/additionalProperties/range全部强制；null禁止。 |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadAdminOverviewResult` | 成功结果；逐字段见types $defs/LoadAdminOverviewResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |

**步骤**

1. portal提供已认证apiClient
2. 读取本域管理API
3. 取消旧navigation请求
4. 基于server version显示状态且不在UI推测费用或任务成功
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：只读请求与组件状态更新
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.SubmitPrimaryEdit

- 级别：参考
- 文件：`modules/localization/ui/src/views/EditView.vue`
- 作用：提交本域明确人工修改

```ts
export async function submitPrimaryEdit(input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `SubmitPrimaryEditInput` | 本域封闭输入；每个字段规格见types.json $defs/SubmitPrimaryEditInput | schema required/additionalProperties/range全部强制；null禁止。 |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SubmitPrimaryEditResult` | 成功结果；逐字段见types $defs/SubmitPrimaryEditResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |

**步骤**

1. 表单逐字段验证本域schema
2. 生成幂等键并携expectedVersion
3. 调本域editTranslationUnit API
4. 409保留用户输入并显示当前版本，不自动覆盖重发
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：本域HTTP写请求和组件状态更新
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.ReuseTranslationMemory

- 级别：参考
- 文件：`modules/localization/internal/translation/tm.go`
- 作用：精确翻译记忆复用

```go
func ReuseTranslationMemory(ctx context.Context,op OperationContext,input ReuseTranslationMemoryInput)(ReuseTranslationMemoryResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 字符数≥20配置阈值
2. sourceHash及语言一致且译文正常/当前术语兼容
3. 只复用target text保留新blockId来源
4. 短块因上下文变化不复用
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.ReadData`、`localization.ReuseInSnapshot`、`localization.VisibleCharacterCount`
- 其他依赖说明：2026-10-18内部算法实际字段已同步types.json；见workflows第二组，旧缺来源/重试状态不得猜测。

## localization.ReuseInSnapshot

- 级别：参考
- 文件：`modules/localization/internal/translation/tm.go`
- 作用：本域参考支持 ReuseInSnapshot；精确输入输出以实际Go签名为准。

```go
func ReuseInSnapshot(ctx context.Context,tx *sql.Tx,op OperationContext,input ReuseTranslationMemoryInput)(ReuseTranslationMemoryResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.StorageError`、`localization.ReadObject`、`localization.VisibleCharacterCount`

## localization.ExtractAutomaticGlossary

- 级别：参考
- 文件：`modules/localization/internal/glossary/extract.go`
- 作用：首次翻译直接抽取入术语表

```go
func ExtractAutomaticGlossary(ctx context.Context, op OperationContext, input ExtractAutomaticGlossaryInput) (ExtractAutomaticGlossaryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ExtractAutomaticGlossaryInput` | 封闭业务输入；逐字段见types $defs/ExtractAutomaticGlossaryInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExtractAutomaticGlossaryResult` | 成功结果；逐字段见types $defs/ExtractAutomaticGlossaryResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. document/target firstTranslation唯一
2. 根task localization.terms使用accurate档max1000output token抽取
3. manual词条优先绝不覆盖，auto冲突保留高version
4. 新auto术语直接入库generation++且batchId可撤销。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：MG费用及自动术语写入，不引入人工批准产品步骤
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.InvokeModel`、`localization.HandleGlossaryChange`

## localization.RollbackAutomaticGlossaryBatch

- 级别：约束
- 文件：`modules/localization/internal/glossary/rollback.go`
- 作用：整批撤销自动术语

```go
func RollbackAutomaticGlossaryBatch(ctx context.Context, op OperationContext, input RollbackAutomaticGlossaryBatchInput) (RollbackAutomaticGlossaryBatchResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RollbackAutomaticGlossaryBatchInput` | 封闭业务输入；逐字段见types $defs/RollbackAutomaticGlossaryBatchInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RollbackAutomaticGlossaryBatchResult` | 成功结果；逐字段见types $defs/RollbackAutomaticGlossaryBatchResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. localization.manage
2. CAS generation
3. 仅删除source=auto且仍该batch，已转manual绝不删
4. generation++并失效受影响机翻
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：术语批次撤销及事件
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.HandleGlossaryChange`、`localization.Mutate`、`localization.RequireAdministrator`、`localization.StorageError`、`localization.AdvanceGeneration`、`localization.PublishChange`、`localization.Generation`

## localization.PlanTranslationFallback

- 级别：参考
- 文件：`modules/localization/internal/translation/fallback.go`
- 作用：逐级翻译失败降级

```go
func PlanTranslationFallback(ctx context.Context,op OperationContext,input PlanTranslationFallbackInput)(PlanTranslationFallbackResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. format/unchanged批对半
2. halves仍失败逐块
3. single仍失败保留原文并状态fallback
4. length先减参考context仅1次，terminology提醒重译仅1次，所有新调用独立MG预留。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：保存降级计划非无限重试
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无
- 其他依赖说明：2026-10-18内部算法实际字段已同步types.json；见workflows第二组，旧缺来源/重试状态不得猜测。

## localization.CheckTerminologyCompliance

- 级别：参考
- 文件：`modules/localization/internal/translation/terminology.go`
- 作用：检查规定译法符合度

```go
func CheckTerminologyCompliance(ctx context.Context,op OperationContext,input CheckTerminologyComplianceInput)(CheckTerminologyComplianceResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 只检查源出现terms
2. 目标包含固定译法或允许语言词形变体
3. 第一次不符带提醒重译一次
4. 仍不符review不阻止发布已译文
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写review/重译计划
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.ReflectTranslation

- 级别：参考
- 文件：`modules/localization/internal/translation/reflection.go`
- 作用：可选反思润色

```go
func ReflectTranslation(ctx context.Context, op OperationContext, input ReflectTranslationInput) (ReflectTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReflectTranslationInput` | 封闭业务输入；逐字段见types $defs/ReflectTranslationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReflectTranslationResult` | 成功结果；逐字段见types $defs/ReflectTranslationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. enabled=false直接原样
2. 精确档对照source提供修改意见再改写两次bounded调用
3. 输出仍固定blockIds/inline标签
4. CAS人工保护/source/代数，费用约初译3倍以MG实际账本为准
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：可选MG费用，默认无调用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数180000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.InvokeModel`

## localization.ReadCacheAnswerTranslation

- 级别：约束
- 文件：`modules/localization/internal/cache-answer/read.go`
- 作用：读取已完成的自动缓存翻译

```go
func ReadCacheAnswerTranslation(ctx context.Context, op OperationContext, input ReadCacheAnswerTranslationInput) (ReadCacheAnswerTranslationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadCacheAnswerTranslationInput` | 封闭业务输入；逐字段见types $defs/ReadCacheAnswerTranslationInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadCacheAnswerTranslationResult` | 成功结果；逐字段见types $defs/ReadCacheAnswerTranslationResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. callerService=qa
2. 源answerRevision精确匹配
3. ready且dataEpoch当前才返回
4. 不改变审核策略、不隐式调模型
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.ReadData`、`localization.StorageError`

## localization.RunWorker

- 级别：约束
- 文件：`modules/localization/internal/runtime/worker.go`
- 作用：主动领取本域持久任务

```go
func RunWorker(ctx context.Context, op OperationContext, input RunWorkerInput) (RunWorkerResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `RunWorkerInput` | 封闭业务输入；逐字段见types $defs/RunWorkerInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RunWorkerResult` | 成功结果；逐字段见types $defs/RunWorkerResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 进程启动后循环调用jobs:claimJob，只领取本域implementation.json jobTypes登记的类型；维护或屏障非released时暂停领取
2. 领取后调用jobs:startJob确认开始，按jobType调用登记的执行函数；每10秒jobs:heartbeatJob续30秒租约、上报进度并读取cancelRequested
3. 执行函数写回结果前调用jobs:checkLease核对fence仍有效；成功jobs:completeJob，失败jobs:failJob并给出可重试分类；收到取消在安全点停止并以cancelled结束
4. lease过期或fence不符立即停止写入；已派发的模型调用不重放，由网关结算或对账

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：jobs租约/完成状态及本域工作链副作用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数30000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`jobs:claimJob`、`jobs:heartbeatJob`、`jobs:completeJob`、`jobs:failJob`、`jobs:startJob`、`jobs:checkLease`、`localization.ExecuteTranslation`、`localization.TranslateInterfaceCatalog`、`localization.ExecuteCacheAnswerTranslation`、`platform.RunWorkerLoop`

## localization.All

- 级别：参考
- 文件：`modules/localization/migrations/migrations.go`
- 作用：按编号升序返回本域业务库全部内嵌迁移并校验编号连续

```go
func All() ([]Migration, error)
```

**输入**

无

**输出**

无

**步骤**

1. go:embed读取NNNN_name.sql（既有NNN_name.sql兼容）；文件名或编号不合法、编号不连续即报错

**错误**

- `INTERNAL` / HTTP 500：打开/迁移/恢复失败（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：库损坏、schema未知或存在未完成决定（不可重试）

- 副作用：只读内嵌迁移内容；执行由本域存储装配在事务中完成
- 授权：permission：service；rule：进程启动装配，不对外暴露
- 并发：纯读取，可并发调用
- 幂等：确定性输出
- 依赖：无

## localization.main

- 级别：约束
- 文件：`modules/localization/cmd/localization/main.go`
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
- 依赖：`localization.BuildApplication`、`localization.Serve`、`localization.Shutdown`、`platform.RunService`

## localization.BuildApplication

- 级别：约束
- 文件：`modules/localization/internal/app/wire.go`
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
6. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环
7. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`localization.LoadRuntime`、`localization.RecoverTranslationLeases`、`localization.PublishOutbox`、`localization.ConsumeEvents`、`localization.RunWorker`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`localization.NewGlossaryHandlers`、`localization.NewTranslationHandlers`、`localization.NewInterfaceHandlers`、`localization.NewCacheTranslationHandlers`、`localization.ConfigureExecutionAdapters`、`localization.ConfigureSourceClient`、`localization.NewPreferenceHandlers`、`localization.NewSuggestionPolicyHandlers`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## localization.Serve

- 级别：约束
- 文件：`modules/localization/internal/transport/server.go`
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
- 依赖：`localization.BuildApplication`、`localization.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## localization.Shutdown

- 级别：约束
- 文件：`modules/localization/internal/transport/server.go`
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
- 依赖：无
- 其他依赖说明：本域barrier安全点函数；本域journal/outbox flush

## localization.Health

- 级别：约束
- 文件：`modules/localization/internal/transport/server.go`
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

## localization.Ready

- 级别：约束
- 文件：`modules/localization/internal/transport/server.go`
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
- 依赖：无
- 其他依赖说明：site-config.readRuntimeState（site-config本域自身读取）

## localization.mountModuleUI

- 级别：约束
- 文件：`modules/localization/ui/src/main.ts`
- 作用：本域Vue3应用/岛的唯一UI构建入口

```ts
function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `rootElement` | `HTMLElement` | 已经存在的模块挂载节点 | DOM标准库；为空或不属于当前document拒绝 |
| `bootstrap` | `ModuleUIBootstrap` | 已由portal/Go输出的安全启动元数据 | module固定本域，不含service凭证/secret |

**输出**

无

**步骤**

1. 校验rootElement与bootstrap module/capabilities；按本域已登记Vue页面导入模块路由，createApp挂载，不注册别域业务组件
2. 只装配同源HTTP/JSON/SSE客户端和已有权限清理/语言逻辑，复用identity/qa会话，缺AI能力保持关闭提示
3. 同DOM重复mount先复用既有实例；unmount停止AbortController/SSE/listeners，不能清服务端业务记录

**错误**

- `VALIDATION` / HTTP 400：bootstrap不匹配本域或DOM无效（不可重试）

- 副作用：挂载本域Vue组件和浏览器事件；不新建业务记录
- 授权：permission：anonymous；rule：只挂载UI；业务mutation仍按各本域OpenAPI权限，不能以能力字段绕过权限
- 并发：同DOM一个Vue实例，AbortController取消旧请求
- 幂等：同DOM/buildRevision重复挂载复用实例
- 依赖：无
- 其他依赖说明：本域已登记UI页面函数

## localization.ConsumeEvents

- 级别：约束
- 文件：`modules/localization/internal/events/subscription.go`
- 作用：从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认

```go
func ConsumeEvents(ctx context.Context, op OperationContext, input ConsumeEventsInput) (ConsumeEventsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 本域后台worker以服务身份构建的操作上下文，含当前dataEpoch | callerService为本域；dataEpoch等于site-config当前active epoch |
| `input` | `ConsumeEventsInput` | 订阅循环参数 | 必须通过公共schema；consumerId等于本域slug |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ConsumeEventsResult` | 循环退出时的累计计数 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 进程启动后以服务身份调用jobs:subscribeEvents（durable模式SSE），只订阅本域events.json中consume登记的事件名与版本；断线按Last-Event-ID续传，1/2/4秒指数退避（上限60秒）重连
2. 每条事件校验信封、生产方payloadSchema与dataEpoch：不在登记清单的事件隔离报警并确认；dataEpoch不等于当前active epoch时，若本域尚未换入该epoch则不确认等待重投，否则视为旧代事件记skipped并确认
3. 按events.json该事件的handler字段调用本域处理函数；处理函数在同一本地事务内完成inbox(eventId+payloadHash)去重、subject revision单调检查与业务写入
4. 处理函数成功提交（含重复或低revision的无操作）后才调用jobs:acknowledgeEvent；失败不确认，等jobs确认租约到期重投，连续失败由jobs转dead-letter供管理重放
5. 屏障draining时停止领取新事件，等在途处理完成后报告安全点；维护期间不确认未处理事件

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs订阅或确认不可用；重连后续传（可重试）
- `VALIDATION`：事件信封或载荷不符合登记schema；隔离并确认（不可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash；隔离报警（不可重试）

- 副作用：HTTP长连接jobs:subscribeEvents；调用本域登记的事件处理函数；HTTP调用jobs:acknowledgeEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个订阅循环；同subject串行，不同subject最多maxInFlight并发；事务与CAS由处理函数负责
- 幂等：jobs至少一次投递；本域inbox按eventId+payloadHash去重，重复投递只重新确认
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`localization.HandlePublication`、`localization.InvalidateDocumentTranslations`、`platform.RunConsumer`

## localization.InvokeModel

- 级别：参考
- 文件：`modules/localization/internal/runtime/model_client.go`
- 作用：经模型网关执行一次调用并读取到终态

```go
func InvokeModel(ctx context.Context,op OperationContext,input InvokeModelInput)(InvokeModelResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 以可信op（rootOperationId、costClass，问答类附已消费的准入许可）调用model-gateway:createInference，幂等键=本域操作ID+步骤名；同键重放返回原调用，不重复收费
2. 循环调用model-gateway:resumeInference按lastSequence续读事件直到terminal；stream=true时逐条把delta交给调用方，断线按序号续传
3. ctx取消或截止到达时调用model-gateway:cancelInference并返回CANCELLED；已派发费用由网关结算，本域不推定免费
4. completed返回全部事件；failed、charge_unknown按网关Problem返回，不在本域换模型或重放（备用重试由网关负责）

**错误**

- `BUDGET_EXCEEDED`：网关额度不足（不可重试）
- `CANCELLED`：调用被取消（不可重试）
- `DEPENDENCY_UNAVAILABLE`：网关不可用或调用失败（可重试）
- `UNSUPPORTED_CAPABILITY`：任务未配置模型（不可重试）

- 副作用：HTTP调用model-gateway推理接口；模型费用由网关记账
- 授权：permission：service；rule：以本服务身份调用网关，费用类别继承可信op
- 并发：每次调用独立；同一根操作的子调用共享网关费用作用域
- 幂等：幂等键固定，重试返回原invocation
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`、`platform.InvokeModel`、`localization.PublishBatch`

## localization.ConfigureExecutionAdapters

- 级别：参考
- 文件：`modules/localization/internal/runtime/model_client.go`
- 作用：本域参考支持 ConfigureExecutionAdapters；精确输入输出以实际Go签名为准。

```go
func ConfigureExecutionAdapters(gateway platform.GatewayClient,publisher platform.EventPublisher)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：无

## localization.PublishBatch

- 级别：参考
- 文件：`modules/localization/internal/runtime/model_client.go`
- 作用：本域参考支持 PublishBatch；精确输入输出以实际Go签名为准。

```go
func PublishBatch(ctx context.Context,limit int)(platform.RelayBatchResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：无

## localization.EstimateLanguagePlan

- 级别：约束
- 文件：`modules/localization/internal/language/plan.go`
- 作用：启用语言前估算全量翻译费用

```go
func EstimateLanguagePlan(ctx context.Context, op OperationContext, input EstimateLanguagePlanInput) (EstimateLanguagePlanResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `EstimateLanguagePlanInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EstimateLanguagePlanResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. localization.languages权限；NormalizeLanguageTag；语言未启用或automatic=false
2. content:readPublicTree列出已发布文档，逐篇content:readPublishedRevision后SplitTranslationUnits与ReuseTranslationMemory统计需新译单元；统计界面字符串缺项
3. 按接口限制分批把实际待估单元文本交model-gateway:estimateTokens，禁止截断或在本域复制token算法；再调用model-gateway:quoteCost分别估localization.document与localization.ui任务，缺价保留price_unknown警告，不伪造0
4. 免费估算以localization纯服务调用：清除原portal proof与用户委托，保留本站/dataEpoch/rootOperationId/费用类别及更早截止；本域先核当前localization.languages，估算不调用模型、不预留或扣减额度
5. 使用libs/platform统一签名siteId传递已核本站；清除原portal proof并建立localization纯服务actor，不能回退未声明的system作用域、发送未签名站点或保留原proof绕过纯服务边界
6. 保存计划（文档修订集合、单元数、quoteHash、24小时过期），只读不调用模型

**错误**

- `VALIDATION`：语言不合法或已启用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：content或网关不可用（可重试）

- 副作用：写语言计划
- 授权：permission：localization.languages；rule：语言管理员
- 并发：同语言同时一个有效计划，新估价替换旧计划
- 幂等：同语言与源集合重复估价返回同一计划
- 依赖：`localization.NormalizeLanguageTag`、`content:readPublicTree`、`content:readPublishedRevision`、`localization.SplitTranslationUnits`、`localization.ReuseTranslationMemory`、`model-gateway:quoteCost`、`model-gateway:estimateTokens`

## localization.ApproveLanguagePlan

- 级别：约束
- 文件：`modules/localization/internal/language/plan.go`
- 作用：批准估价后启用语言并创建全量翻译任务

```go
func ApproveLanguagePlan(ctx context.Context, op OperationContext, input ApproveLanguagePlanInput) (ApproveLanguagePlanResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ApproveLanguagePlanInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ApproveLanguagePlanResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. localization.languages权限；计划未过期且quoteHash一致
2. SetLanguageEnabled启用该语言（automatic=true），记录批准人与估价
3. 为计划内每篇文档创建localization.document任务（按source/target/generation去重），另建一个localization.ui任务
4. 翻译完成前公开页面按旧译或原文回退并提示

**错误**

- `VALIDATION`：计划过期或hash不符（不可重试）
- `CONFLICT`：计划已被批准（不可重试）

- 副作用：启用语言；创建翻译任务
- 授权：permission：localization.languages；rule：语言管理员显式批准费用
- 并发：计划CAS pending→approved
- 幂等：同planId重复批准返回原结果
- 依赖：`localization.SetLanguageEnabled`、`jobs:createJob`

## localization.ReadData

- 级别：参考
- 文件：`modules/localization/internal/language/transaction.go`
- 作用：本域参考支持 ReadData；精确输入输出以实际Go签名为准。

```go
func ReadData(ctx context.Context,op OperationContext,read func(*sql.Tx)error)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.Mutate`

## localization.Mutate

- 级别：参考
- 文件：`modules/localization/internal/language/transaction.go`
- 作用：本域参考支持 Mutate；精确输入输出以实际Go签名为准。

```go
func Mutate[I,R any](ctx context.Context,op OperationContext,functionID,key string,input I,result *R,write func(*sql.Tx)error)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.RequireAdministrator`

## localization.RequireAdministrator

- 级别：参考
- 文件：`modules/localization/internal/language/transaction.go`
- 作用：本域参考支持 RequireAdministrator；精确输入输出以实际Go签名为准。

```go
func RequireAdministrator(op OperationContext)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.EmitReference`

## localization.EmitReference

- 级别：参考
- 文件：`modules/localization/internal/language/transaction.go`
- 作用：本域参考支持 EmitReference；精确输入输出以实际Go签名为准。

```go
func EmitReference(tx *sql.Tx,op OperationContext,event,subject string,revision int,payload any)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：无

## localization.StorageError

- 级别：参考
- 文件：`modules/localization/internal/language/transaction.go`
- 作用：本域参考支持 StorageError；精确输入输出以实际Go签名为准。

```go
func StorageError(err error)error
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：无

## localization.NewGlossaryHandlers

- 级别：参考
- 文件：`modules/localization/internal/glossary/http.go`
- 作用：本域参考支持 NewGlossaryHandlers；精确输入输出以实际Go签名为准。

```go
func NewGlossaryHandlers()map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：`localization.PutGlossaryTerm`、`localization.DeleteGlossaryTerm`、`localization.ListGlossary`、`localization.ApplyApprovedGlossaryProposal`、`localization.RollbackAutomaticGlossaryBatch`、`localization.glossaryJSON`

## localization.glossaryJSON

- 级别：参考
- 文件：`modules/localization/internal/glossary/http.go`
- 作用：本域参考支持 glossaryJSON；精确输入输出以实际Go签名为准。

```go
func glossaryJSON[I,R any](fn func(context.Context,OperationContext,I)(R,error),status int,bind ...func(*I,*http.Request)error)http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 复用本域现有事务、原回执、代数及固定快照，不修改外域私有数据；错误仅使用安全ID与状态。

**错误**

- `VALIDATION`：按用例边界或私有IO失败关闭（不可重试）
- `CONFLICT`：按用例边界或私有IO失败关闭（不可重试）
- `VERSION_MISMATCH`：按用例边界或私有IO失败关闭（不可重试）
- `INTEGRITY_ERROR`：按用例边界或私有IO失败关闭（不可重试）
- `FORBIDDEN`：按用例边界或私有IO失败关闭（不可重试）
- `NOT_FOUND`：按用例边界或私有IO失败关闭（不可重试）
- `INTERNAL`：按用例边界或私有IO失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按用例边界或私有IO失败关闭（不可重试）

- 副作用：无
- 授权：功能权限由平台入口建立，内部仅落实本函数人工主体与命令来源特有规则。
- 并发：固定调用方事务/快照；失败回滚。
- 幂等：沿用所属用例原回执或不可变输入，不创建独立付费操作。
- 依赖：无

## localization.HandleGlossaryChange

- 级别：参考
- 文件：`modules/localization/internal/glossary/change.go`
- 作用：使机翻版本按术语变化失效

```go
func HandleGlossaryChange(ctx context.Context,tx *sql.Tx,op OperationContext,source,target string,generation int)(int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 由术语写入函数在同一提交后本地调用，不经事件
2. 只处理更高glossary generation
3. 受影响machine manifest标stale并在后台提示重译估价，不自动产生费用
4. protected文本保持并给出review提示

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：标stale不立即消费模型
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.StorageError`

## localization.PlanIncrementalTranslation

- 级别：参考
- 文件：`modules/localization/internal/translation/plan.go`
- 作用：增量选择待翻译单元

```go
func PlanIncrementalTranslation(ctx context.Context,op OperationContext,input PlanIncrementalTranslationInput)(PlanIncrementalTranslationResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 源hash没变复用
2. changed/new块先同源hash且≥20字translation memory匹配
3. human/protected源变不覆盖只review
4. 未复用块按1000预计输出token均衡顺序分批。
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写plan与stale人工提示不覆盖译文
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.ReadData`、`localization.ReadObject`、`localization.ReuseInSnapshot`
- 其他依赖说明：2026-10-18内部算法实际字段已同步types.json；见workflows第二组，旧缺来源/重试状态不得猜测。

## localization.objectPath

- 级别：参考
- 文件：`modules/localization/internal/translation/store.go`
- 作用：本域参考支持 objectPath；精确输入输出以实际Go签名为准。

```go
func objectPath(root string,t gen.Translation)(string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.NormalizeLanguageTag`、`localization.Generation`

## localization.ReadObject

- 级别：参考
- 文件：`modules/localization/internal/translation/store.go`
- 作用：本域参考支持 ReadObject；精确输入输出以实际Go签名为准。

```go
func ReadObject(ctx context.Context,tx *sql.Tx,op OperationContext,id string)(TranslationObject,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.Generation`、`localization.StorageError`、`localization.objectPath`、`localization.Root`

## localization.WriteObject

- 级别：参考
- 文件：`modules/localization/internal/translation/store.go`
- 作用：本域参考支持 WriteObject；精确输入输出以实际Go签名为准。

```go
func WriteObject(ctx context.Context,tx *sql.Tx,op OperationContext,object TranslationObject,previousGeneration int)(TranslationObject,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.Generation`、`localization.StorageError`、`localization.objectPath`、`localization.Root`、`localization.ReserveObjectGeneration`、`localization.SplitTranslationUnits`、`localization.RestoreSourceTree`

## localization.EditTranslationUnit

- 级别：约束
- 文件：`modules/localization/internal/translation/edit.go`
- 作用：人工编辑并自动保护

```go
func EditTranslationUnit(ctx context.Context, op OperationContext, input EditTranslationUnitInput) (EditTranslationUnitResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `EditTranslationUnitInput` | 封闭业务输入；逐字段见types $defs/EditTranslationUnitInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EditTranslationUnitResult` | 成功结果；逐字段见types $defs/EditTranslationUnitResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 权限和插值校验
2. 单元CAS
3. origin=human protected=true humanGeneration++
4. 取消旧待写批次
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写人工内容与generation事件
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.Generation`、`localization.Mutate`、`localization.RequireAdministrator`、`localization.StorageError`、`localization.ReadObject`、`localization.WriteObject`、`localization.CheckInlineTokens`

## localization.SetUnitProtection

- 级别：约束
- 文件：`modules/localization/internal/translation/edit.go`
- 作用：显式启停人工保护

```go
func SetUnitProtection(ctx context.Context, op OperationContext, input SetUnitProtectionInput) (SetUnitProtectionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `SetUnitProtectionInput` | 封闭业务输入；逐字段见types $defs/SetUnitProtectionInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SetUnitProtectionResult` | 成功结果；逐字段见types $defs/SetUnitProtectionResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 只能人工授权
2. CAS单元
3. humanGeneration++
4. 不自动重翻或覆盖现有文本
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：写保护和审核日志
- 授权：权限 localization.manage；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.Generation`、`localization.Mutate`、`localization.RequireAdministrator`、`localization.StorageError`、`localization.ReadObject`、`localization.WriteObject`

## localization.NewTranslationHandlers

- 级别：参考
- 文件：`modules/localization/internal/translation/http.go`
- 作用：本域参考支持 NewTranslationHandlers；精确输入输出以实际Go签名为准。

```go
func NewTranslationHandlers()map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.EditTranslationUnit`、`localization.SetUnitProtection`、`localization.JSONHandler`、`localization.ReadTranslation`、`localization.ListTranslationProgress`、`localization.ReadTranslationUnits`

## localization.NewInterfaceHandlers

- 级别：参考
- 文件：`modules/localization/internal/interface/http.go`
- 作用：本域参考支持 NewInterfaceHandlers；精确输入输出以实际Go签名为准。

```go
func NewInterfaceHandlers()map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.PutInterfaceString`、`localization.ReadInterfaceCatalog`、`localization.JSONHandler`

## localization.NewCacheTranslationHandlers

- 级别：参考
- 文件：`modules/localization/internal/cache-answer/http.go`
- 作用：本域参考支持 NewCacheTranslationHandlers；精确输入输出以实际Go签名为准。

```go
func NewCacheTranslationHandlers()map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：`localization.ReadCacheAnswerTranslation`、`localization.JSONHandler`

## localization.JSONHandler

- 级别：参考
- 文件：`modules/localization/internal/language/protocol.go`
- 作用：本域参考支持 JSONHandler；精确输入输出以实际Go签名为准。

```go
func JSONHandler[I,R any](execute func(context.Context,OperationContext,I)(R,error),bind ...func(*I,*http.Request)error)http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定来源和调用方事务处理本域正文/元数据或纯算法；保留公开错误，不创建隐式模型调用。

**错误**

- `VALIDATION`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按本用例来源、CAS或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口，特殊人工保护/任务记忆/模型费用及来源约束由所属用例执行。
- 并发：复用固定快照/事务；权威文件先落盘、元数据CAS后激活，失败回滚；取消由ctx传IO。
- 幂等：本域原回执、代数或不可变引用；不自动创建收费重算。
- 依赖：无

## localization.SplitTranslationUnits

- 级别：参考
- 文件：`modules/localization/internal/translation/split.go`
- 作用：确定性切分翻译单元

```go
func SplitTranslationUnits(ctx context.Context,op OperationContext,input SplitTranslationUnitsInput)(SplitTranslationUnitsResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 输入来自严格生产方响应或本域已核hash对象；不重复结构schema。保留16MiB完整源，单元65536字符，超大table只按真实rowId拆并保parentBlockId，其他超大块source-only+fallbackBlockIds，不编造ID。
2. 沿用content稳定blockId与完整块树
3. inline粗体/链接/代码转编号标签，URL/code值保留在本服务不发模型
4. code块/公式/嵌入不翻译但image alt/caption翻译
5. 单块超预算仅table按行拆并保留父blockId，最终还原树。
6. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：（不可重试）
- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：无
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.NormalizeLanguageTag`、`localization.AppendPath`、`localization.CollectSlots`、`localization.WithoutSourceIDs`

## localization.AppendPath

- 级别：参考
- 文件：`modules/localization/internal/translation/split.go`
- 作用：本域参考支持 AppendPath；精确输入输出见实际Go签名。

```go
func AppendPath(path []string,parts ...string)[]string
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.CollectSlots`

## localization.CollectSlots

- 级别：参考
- 文件：`modules/localization/internal/translation/split.go`
- 作用：本域参考支持 CollectSlots；精确输入输出见实际Go签名。

```go
func CollectSlots(node map[string]any,path []string,slots *[]InlineSlot,text *strings.Builder)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.AppendPath`、`localization.AppendTextSlots`、`localization.AppendSlot`

## localization.AppendTextSlots

- 级别：参考
- 文件：`modules/localization/internal/translation/split.go`
- 作用：本域参考支持 AppendTextSlots；精确输入输出见实际Go签名。

```go
func AppendTextSlots(slots *[]InlineSlot,text *strings.Builder,path []string,value string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.AppendSlot`

## localization.AppendSlot

- 级别：参考
- 文件：`modules/localization/internal/translation/split.go`
- 作用：本域参考支持 AppendSlot；精确输入输出见实际Go签名。

```go
func AppendSlot(slots *[]InlineSlot,text *strings.Builder,path []string,value string,fixed bool)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.WithoutSourceIDs`

## localization.WithoutSourceIDs

- 级别：参考
- 文件：`modules/localization/internal/translation/split.go`
- 作用：本域参考支持 WithoutSourceIDs；精确输入输出见实际Go签名。

```go
func WithoutSourceIDs(value any)any
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.RestoreSourceTree

- 级别：参考
- 文件：`modules/localization/internal/translation/restore.go`
- 作用：本域参考支持 RestoreSourceTree；精确输入输出见实际Go签名。

```go
func RestoreSourceTree(source json.RawMessage,templates []UnitTemplate,translated []gen.TranslationUnit)(json.RawMessage,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.ParseSlotValues`、`localization.ReplaceSourceText`、`localization.ValidateSourcePayload`

## localization.ParseSlotValues

- 级别：参考
- 文件：`modules/localization/internal/translation/restore.go`
- 作用：本域参考支持 ParseSlotValues；精确输入输出见实际Go签名。

```go
func ParseSlotValues(slots []InlineSlot,target string)([]string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.ReplaceSourceText

- 级别：参考
- 文件：`modules/localization/internal/translation/restore.go`
- 作用：本域参考支持 ReplaceSourceText；精确输入输出见实际Go签名。

```go
func ReplaceSourceText(root any,path []string,expected,replacement string)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.FetchTranslationSource

- 级别：参考
- 文件：`modules/localization/internal/translation/source.go`
- 作用：读取翻译源

```go
func FetchTranslationSource(ctx context.Context,op OperationContext,input FetchTranslationSourceInput)(FetchTranslationSourceResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 受信JobExecutionInput绑定localization.document、inputRef owner/objectId、epoch及workerId兼容leaseId；锁外CheckLease成功后读取本域固定来源，不复制草稿。
2. content readPublishedRevision
3. hash/schema匹配
4. 分块只读不复制草稿
5. 业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。

**错误**

- `STALE_RESULT` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：私有临时输入引用
- 授权：权限 service；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数10000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`content:readPublishedRevision`、`localization.ReadData`、`localization.StorageError`、`localization.FetchPublishedSnapshot`

## localization.ListTranslationProgress

- 级别：约束
- 文件：`modules/localization/internal/translation/progress.go`
- 作用：查看各语言翻译进度

```go
func ListTranslationProgress(ctx context.Context, op OperationContext, input ListTranslationProgressInput) (ListTranslationProgressResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `ListTranslationProgressInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListTranslationProgressResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. localization.manage
2. 按语言汇总完成、排队与待复核单元

**错误**

- `FORBIDDEN`：权限不足（不可重试）

- 副作用：无
- 授权：permission：localization.manage；rule：后台管理员按登记权限
- 并发：只读快照或CAS
- 幂等：读取幂等；写入按operationId
- 依赖：`localization.NormalizeLanguageTag`、`localization.ReadData`、`localization.RequireAdministrator`、`localization.StorageError`

## localization.ConfigureSourceClient

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 ConfigureSourceClient；精确输入输出见实际Go签名。

```go
func ConfigureSourceClient(client *gen.ContentClient,validators map[string]func(json.RawMessage)error,key platform.SigningKey)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.ValidateSourcePayload

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 ValidateSourcePayload；精确输入输出见实际Go签名。

```go
func ValidateSourcePayload(name string,raw json.RawMessage)error
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.SourceOperation

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 SourceOperation；精确输入输出见实际Go签名。

```go
func SourceOperation(ctx context.Context,op OperationContext)context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.FetchPublishedSnapshot

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 FetchPublishedSnapshot；精确输入输出见实际Go签名。

```go
func FetchPublishedSnapshot(ctx context.Context,op OperationContext,document gen.PublishedRevisionRef)(PublishedSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.readPublishedSnapshot`

## localization.FetchCurrentPublishedSnapshot

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 FetchCurrentPublishedSnapshot；精确输入输出见实际Go签名。

```go
func FetchCurrentPublishedSnapshot(ctx context.Context,op OperationContext,documentID string)(PublishedSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.readPublishedSnapshot`

## localization.readPublishedSnapshot

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 readPublishedSnapshot；精确输入输出见实际Go签名。

```go
func readPublishedSnapshot(ctx context.Context,op OperationContext,documentID string,expected *gen.PublishedRevisionRef)(PublishedSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.ValidateSourcePayload`、`localization.SourceOperation`

## localization.CheckPublication

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 CheckPublication；精确输入输出见实际Go签名。

```go
func CheckPublication(ctx context.Context,op OperationContext,document gen.PublishedRevisionRef,expectedVisibility *int)(int,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.ValidateSourcePayload`、`localization.SourceOperation`、`localization.CursorKey`

## localization.CursorKey

- 级别：参考
- 文件：`modules/localization/internal/runtime/source_client.go`
- 作用：本域参考支持 CursorKey；精确输入输出见实际Go签名。

```go
func CursorKey()(platform.SigningKey,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按已固定源/目标身份完成本域文本模板、只读投影或版本化HTTP适配；不触发模型或改变保护事实。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.StableSlotKey

- 级别：参考
- 文件：`modules/localization/internal/translation/projection.go`
- 作用：安全人工保护投影的本域参考辅助：StableSlotKey

```go
func StableSlotKey(tree any, path []string) (string, any, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只用最深真实节点身份及该节点内路径；祖先数组位置不进入对应。无真实锚点/不能定位失败关闭。
2. 只认content声明的真实节点ID属性，不把link mark等引用attrs.blockId误认成本节点身份；同时返回原稳定节点供非稳定结构签名。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.CollectSlotFields

- 级别：参考
- 文件：`modules/localization/internal/translation/projection.go`
- 作用：安全人工保护投影的本域参考辅助：CollectSlotFields

```go
func CollectSlotFields(tree any, slots []InlineSlot) ([]slotField, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同一text/alt的固定/可译分段聚合；两条原树路径映到同一真实稳定键视为完整性失败，不猜测。
2. 每个真实稳定anchor缓存非稳定子树结构签名；插入无ID inline/text/hardBreak等改变字段或数组形状不得沿旧路径复用。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.StableSlotKey`、`localization.ProtectedAnchorShape`

## localization.BuildProtectedProjection

- 级别：参考
- 文件：`modules/localization/internal/translation/projection.go`
- 作用：安全人工保护投影的本域参考辅助：BuildProtectedProjection

```go
func BuildProtectedProjection(ctx context.Context, op OperationContext, current json.RawMessage, objects []TranslationObject) (ProtectedProjection, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只从真实人工/保护旧单元按旧私有模板解析；较新对象同字段优先。保留真实父保护关系；不将未知旧数据补造成新来源。
2. 字段携带真实origin；仅来源human的旧译值可使投影origin=human，受保护machine不改造来源。
3. 同一anchor只取最新人工/保护来源的完整字段集，禁止从更旧对象补已移除字段；遍历当前真实节点，即使已无可译slot仍核旧保护anchor节点类型/非稳定形状，变化失败关闭。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.SplitTranslationUnits`、`localization.ParseSlotValues`、`localization.CollectSlotFields`、`localization.ProtectedAnchorShape`

## localization.ProjectProtectedUnit

- 级别：参考
- 文件：`modules/localization/internal/translation/projection.go`
- 作用：安全人工保护投影的本域参考辅助：ProjectProtectedUnit

```go
func ProjectProtectedUnit(projection ProtectedProjection, template UnitTemplate, candidate gen.TranslationUnit) (gen.TranslationUnit, bool, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 稳定字段及固定/可译段形状均匹配才复用人工译值；固定值取当前源，输出按新模板重编号。新增节点原文partial/review且继承父保护；无法证明归属继续失败关闭。只构造即时派生，不伪造fileRef或改旧权威对象。
2. 逐段编码时核65536 Unicode字符预算，合并旧长译值与新源超限BUDGET_EXCEEDED；保留protected=true，只有复用真实human字段才标origin=human。PreviousTargetText可选：不把重编号结果伪称某个旧父整段原值。
3. 同一真实anchor的非稳定子树签名必须一致，新增无独立ID文本不能借content数组索引假称对应；新真实稳定子节点方可按原文partial处理。

**错误**

- `VALIDATION`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `CONFLICT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `VERSION_MISMATCH`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `STALE_RESULT`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTEGRITY_ERROR`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `FORBIDDEN`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `NOT_FOUND`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `INTERNAL`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `DEPENDENCY_UNAVAILABLE`：按来源/水位/预算或私有IO边界失败关闭（不可重试）
- `BUDGET_EXCEEDED`：按来源/水位/预算或私有IO边界失败关闭（不可重试）

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：`localization.CollectSlotFields`

## localization.ProtectedAnchorShape

- 级别：参考
- 文件：`modules/localization/internal/translation/projection.go`
- 作用：计算真实稳定节点内部无ID树形签名以拒绝inline数组误对应

```go
func ProtectedAnchorShape(root any) string
```

**输入**

无

**输出**

无

**步骤**

1. 只保留节点type和字段/数组形状，不含文本及URL实际值；真实稳定子节点另行定锚。用于证明同一anchor内text/alt路径未因无ID节点增删而改变。

**错误**

- 无

- 副作用：无
- 授权：信任已核入口；人工主体、源服务来源与游标稳定身份按所属用例绑定。
- 并发：固定快照；外部IO后复核水位；未知或变化整次失败，不拼装部分成功。
- 幂等：确定性源模板或只读版本投影，不创建独立收费操作。
- 依赖：无

## localization.ReadLanguageSuggestionPolicy

- 级别：约束
- 文件：`modules/localization/internal/language/suggestion_policy.go`
- 作用：当前管理员读取本域同site/epoch持久付费建议开关，无配置明确disabled/version0，不造批准ID。

```go
func ReadLanguageSuggestionPolicy(ctx context.Context, op OperationContext, input ReadLanguageSuggestionPolicyInput) (LanguageSuggestionPolicy, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadLanguageSuggestionPolicyInput` | 封闭业务输入；逐字段见types $defs/ListLanguagesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LanguageSuggestionPolicy` | 成功结果；逐字段见types $defs/ListLanguagesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 当前管理员读取本域同site/epoch持久付费建议开关，无配置明确disabled/version0，不造批准ID。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：permission：localization.manage；rule：管理员操作只portal当前localization.manage；批准只model-gateway纯服务，不能接受访客自报开关。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.ReadData`、`localization.RequireAdministrator`、`localization.readSuggestionPolicy`

## localization.PutLanguageSuggestionPolicy

- 级别：约束
- 文件：`modules/localization/internal/language/suggestion_policy.go`
- 作用：当前管理员明确启停，expectedVersion CAS与七天原回执同事务。

```go
func PutLanguageSuggestionPolicy(ctx context.Context, op OperationContext, input PutLanguageSuggestionPolicyInput) (LanguageSuggestionPolicy, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `PutLanguageSuggestionPolicyInput` | 封闭业务输入；逐字段见types $defs/ListLanguagesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LanguageSuggestionPolicy` | 成功结果；逐字段见types $defs/ListLanguagesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 当前管理员明确启停，expectedVersion CAS与七天原回执同事务。
2. 每个变更生成新approvalId与固定site/epoch/approvedAt，记录真实approvedBy/permissionRevision/root；原批准行只追加，当前指针CAS更新。
3. 停用或恢复新dataEpoch后旧批准不再有效；不自动开通、不创建模型或费用任务。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：本域批准历史与当前开关CAS、幂等回执同事务
- 授权：permission：localization.manage；rule：管理员操作只portal当前localization.manage；批准只model-gateway纯服务，不能接受访客自报开关。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.Mutate`、`localization.RequireAdministrator`

## localization.ReadLanguageSuggestionApproval

- 级别：约束
- 文件：`modules/localization/internal/language/suggestion_policy.go`
- 作用：仅model-gateway纯服务读取当前同site/epoch的精确approvalId，旧版本/未知404，返回真实enabled/version/approvedAt，供新attempt失败关闭。

```go
func ReadLanguageSuggestionApproval(ctx context.Context, op OperationContext, input ReadLanguageSuggestionApprovalInput) (ReadLanguageSuggestionApprovalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | Go取消与截止控制 | 不可nil；与op.deadline取最早截止。 |
| `op` | `OperationContext` | 可信执行元数据 | 公共schema；入口验证来源、权限、dataEpoch、rootOperationId和costClass继承。 |
| `input` | `ReadLanguageSuggestionApprovalInput` | 封闭业务输入；逐字段见types $defs/ListLanguagesInput | required字段不得省略，额外字段/null拒绝；UTF8字节限制另按步骤检查。 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadLanguageSuggestionApprovalResult` | 成功结果；逐字段见types $defs/ListLanguagesResult | 仅全部成功校验/提交后返回；未产生字段用schema允许的缺失，不返回null。 |
| `err` | `error` | nil表示成功；非nil映射公共Problem与本函数错误集合 | 只能返回errors登记code；无prompt/secret/IP/原请求体；失败结果零值不得被调用方使用。 |

**步骤**

1. 仅model-gateway纯服务读取当前同site/epoch的精确approvalId，旧版本/未知404，返回真实enabled/version/approvedAt，供新attempt失败关闭。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）

- 副作用：无
- 授权：permission：service；rule：管理员操作只portal当前localization.manage；批准只model-gateway纯服务，不能接受访客自报开关。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.ReadData`、`localization.readSuggestionPolicy`

## localization.readSuggestionPolicy

- 级别：参考
- 文件：`modules/localization/internal/language/suggestion_policy.go`
- 作用：从精确site/epoch持久开关读取安全批准投影

```go
func readSuggestionPolicy(ctx context.Context, tx *sql.Tx, op OperationContext) (LanguageSuggestionPolicy, error)
```

**输入**

无

**输出**

无

**步骤**

1. 不存在时默认关闭；存在时查询当前指针及不可变批准历史，未知或矛盾不授权。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：无

## localization.NewSuggestionPolicyHandlers

- 级别：参考
- 文件：`modules/localization/internal/language/suggestion_policy.go`
- 作用：装配真实语言建议开关和批准读取HTTP

```go
func NewSuggestionPolicyHandlers() map[string]http.Handler
```

**输入**

无

**输出**

无

**步骤**

1. 复用JSONHandler绑定严格输入；批准ID由路径写入，幂等头与正文相同；no-store/统一Problem。

**错误**

- `FORBIDDEN` / HTTP 403：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）

- 副作用：无
- 授权：权限 anonymous；HTTP入口identity:checkAuthorization校验服务/访客/管理员身份，actor.siteId一致且对象owner验证；costClass/rootOperationId仅可信op传递。
- 并发：本服务资源行锁及expectedVersion/expectedRevision CAS；lease绑定owner与到期时间，30秒租约每10秒续约；只读固定快照；业务写入前校验op.dataEpoch等于site-config当前active epoch；本地屏障非released拒绝新业务写。截止=min(ctx deadline,op.deadline,本函数5000ms)，每一外部调用/批次/提交前检查取消；失败事务回滚。
- 幂等：有idempotencyKey时按siteId+actorId+函数ID+key唯一，保留7天；同规范输入SHA256返回原结果，不同hash为CONFLICT；无key的纯读取/计算可重复，后台写入以对象/版本/lease/eventId唯一防重复。
- 依赖：`localization.JSONHandler`、`localization.ReadLanguageSuggestionPolicy`、`localization.PutLanguageSuggestionPolicy`、`localization.ReadLanguageSuggestionApproval`

## localization.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/localization/internal/snapshot_archive.go`
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
