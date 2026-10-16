# content 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 134 个函数：约束 42 个，参考 92 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## content.CreateDocument

- 级别：约束
- 文件：`modules/content/internal/document_service.go`
- 作用：新建共享草稿文档

```go
func CreateDocument(ctx context.Context, op OperationContext, input CreateDocumentInput) (CreateDocumentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CreateDocumentInput` | 新建共享草稿文档的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreateDocumentResult` | 新建共享草稿文档的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 检查content.edit、parent非trash和treeRevision，site-config取基准语言
2. 生成docId/空Tiptap doc与fractional order；WriteRevisionFile
3. 事务建tree metadata/shared draft head/ID ledger/COMMITTED journal；未发草稿不可公开

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`site-config:readSiteConfig`、`content.WriteRevisionFile`、`content.CommitDraftJournal`

## content.ReadDocumentTree

- 级别：约束
- 文件：`modules/content/internal/tree_service.go`
- 作用：后台读取整站文档树

```go
func ReadDocumentTree(ctx context.Context, op OperationContext, input ReadDocumentTreeInput) (ReadDocumentTreeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadDocumentTreeInput` | 后台读取整站文档树的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadDocumentTreeResult` | 后台读取整站文档树的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 检查content.read
2. 按parent/orderKey构建一致前序，保留draft徽标，回收站独立读取
3. 验证无环；损坏不输出部分伪树

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：content.read
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.EnsureTreeAcyclic`

## content.ListDocuments

- 级别：约束
- 文件：`modules/content/internal/tree_service.go`
- 作用：文档及回收站分页列表

```go
func ListDocuments(ctx context.Context, op OperationContext, input ListDocumentsInput) (ListDocumentsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ListDocumentsInput` | 文档及回收站分页列表的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListDocumentsResult` | 文档及回收站分页列表的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 按功能权限content.read或publish读取相应状态
2. 签名cursor绑定site/state/tree watermark
3. 不在列表返回正文

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：content.read；trash需content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## content.MoveDocument

- 级别：约束
- 文件：`modules/content/internal/tree_service.go`
- 作用：拖拽移动层级和顺序

```go
func MoveDocument(ctx context.Context, op OperationContext, input MoveDocumentInput) (MoveDocumentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `MoveDocumentInput` | 拖拽移动层级和顺序的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `MoveDocumentResult` | 拖拽移动层级和顺序的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 锁tree水位，EnsureTreeAcyclic防移入自己后代、检查同父slug唯一
2. PlanOldPathRedirects捕获已公开节点及全部后代旧路径→docId
3. 事务更新parent/order/treeRevision/redirect manifest并outbox，链接因docId不失效

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：全站树事务CAS，移动和发布路径规划同一tree锁
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.EnsureTreeAcyclic`、`content.PlanOldPathRedirects`

## content.ReorderChildren

- 级别：约束
- 文件：`modules/content/internal/tree_service.go`
- 作用：拖拽调整同级顺序

```go
func ReorderChildren(ctx context.Context, op OperationContext, input ReorderChildrenInput) (ReorderChildrenResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReorderChildrenInput` | 拖拽调整同级顺序的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReorderChildrenResult` | 拖拽调整同级顺序的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 核完整集合与parent关系，拒绝缺失/重复/跨层
2. fracdex生成顺序，不改变slug
3. 事务CAS treeRevision，前后篇按新树排序

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：无

## content.EnsureTreeAcyclic

- 级别：参考
- 文件：`modules/content/internal/tree_validator.go`
- 作用：检查树无环和层级约束

```go
func EnsureTreeAcyclic(input EnsureTreeAcyclicInput) (EnsureTreeAcyclicResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `EnsureTreeAcyclicInput` | 检查树无环和层级约束的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `EnsureTreeAcyclicResult` | 检查树无环和层级约束的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 索引完整当前树的documentId/parentId；重复ID、缺父与原树环返回INTEGRITY_ERROR，根为第一层，原树超过32层返回VALIDATION；不因拟移动可修复而接受损坏原树
2. movingId为null时只校验当前树、忽略拟父并返回空descendants；非null必须存在，非null拟父也必须存在；缺拟移/拟父返回VALIDATION
3. 从原树收集拟移节点全部后代，不包含自己；拟父等于自身或任一后代返回VALIDATION
4. 以拟父当前深度+拟移子树完整高度核移动后最长层级，超过32返回VALIDATION；移至根的拟父深度为0，不仅核拟移节点自身
5. 成功返回valid=true及原树后代ID字典序，所有输入及父关系保持不变，空结果序列化为[]

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.documentTreeDepths`

## content.documentTreeDepths

- 级别：参考
- 文件：`modules/content/internal/tree_validator.go`
- 作用：按已核完整父索引计算原树深度并拒绝环/超过32层

```go
func documentTreeDepths(documents []gen.DocumentMetadata, index map[Identifier]int) ([]int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 沿完整有限父链作三色遍历，黑色深度复用、灰色再入为环；无递归且每节点只处理一次，逆序回填根起第1层的深度并拒绝超过32层

**错误**

- 无

- 副作用：无
- 授权：纯函数；调用方已完成入口结构校验和本域资源授权
- 并发：只读取输入，局部状态不跨调用共享；可以并发调用
- 幂等：相同输入产生相同输出，无持久副作用
- 依赖：无

## content.UpdateDocumentMetadata

- 级别：约束
- 文件：`modules/content/internal/metadata_service.go`
- 作用：修改标题/路径/SEO描述/分享图

```go
func UpdateDocumentMetadata(ctx context.Context, op OperationContext, input UpdateDocumentMetadataInput) (UpdateDocumentMetadataResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `UpdateDocumentMetadataInput` | 修改标题/路径/SEO描述/分享图的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `UpdateDocumentMetadataResult` | 修改标题/路径/SEO描述/分享图的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. content.edit；人工SEO标manual且进后续翻译，OG资源需ready
2. ValidatePathSegment同父唯一，已公开path更改PlanOldPathRedirects覆盖后代
3. metadata CAS与tree路径映射原子更新；不自动把共享draft发给访客

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.ValidatePathSegment`、`content.PlanOldPathRedirects`、`media:readAsset`

## content.ValidatePathSegment

- 级别：参考
- 文件：`modules/content/internal/path_service.go`
- 作用：校验英文可读路径与同父唯一

```go
func ValidatePathSegment(ctx context.Context, op OperationContext, input ValidatePathSegmentInput) (ValidatePathSegmentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidatePathSegmentInput` | 校验英文可读路径与同父唯一的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePathSegmentResult` | 校验英文可读路径与同父唯一的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 正则与保留路径名检查，禁止点段/编码斜杠
2. 查询同父活动节点排除自己
3. 冲突CONFLICT，历史redirect不能覆写活跃目标

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：content.edit或publish私有流程
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## content.PlanOldPathRedirects

- 级别：参考
- 文件：`modules/content/internal/path_service.go`
- 作用：规划301旧路径→文档ID

```go
func PlanOldPathRedirects(input PlanOldPathRedirectsInput) (PlanOldPathRedirectsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `PlanOldPathRedirectsInput` | 规划301旧路径→文档ID的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PlanOldPathRedirectsResult` | 规划301旧路径→文档ID的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 前序取已公开节点及后代完整旧path
2. 不含语言前缀，因为所有语言共slug
3. 记录docId而不链式URL；公开解析当前路径避免跳转环

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：无

## content.AcquireEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：打开文档自动获得内存编辑锁

```go
func AcquireEditLock(ctx context.Context, op OperationContext, input AcquireEditLockInput) (AcquireEditLockResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `AcquireEditLockInput` | 打开文档自动获得内存编辑锁的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `AcquireEditLockResult` | 打开文档自动获得内存编辑锁的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；NOT_FOUND: 资源不存在或在当前主体作用域中不可见 |

**步骤**

1. content.edit与doc状态校验；单active content leader管理锁
2. 空/过期锁分配bootId+单调fence，期限2分钟；别人持锁返回LOCKED与safe holderName只读
3. 同账户不同tab可共享账户锁，不能绕过draftVersion CAS

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）

- 副作用：本进程锁表
- 授权：content.edit
- 并发：docId内存mutex，content只有单写leader
- 幂等：同account/doc有效lease复用，不换fence
- 依赖：无

## content.RenewEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：心跳维持锁并处理长时间无输入

```go
func RenewEditLock(ctx context.Context, op OperationContext, input RenewEditLockInput) (RenewEditLockResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RenewEditLockInput` | 心跳维持锁并处理长时间无输入的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenewEditLockResult` | 心跳维持锁并处理长时间无输入的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. 核holder/lease/boot/fence当前且未到期
2. 心跳延长2分钟，但15分钟无输入触发提醒，宽限2分钟仍无输入释放为只读
3. 忽略客户端未来lastInputAt，服务依据输入序号/可信接收时间更新

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：内存lease时间
- 授权：content.edit
- 并发：doc mutex
- 幂等：心跳序号单调，旧心跳不延长已释放锁
- 依赖：无

## content.ReleaseEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：关闭页面释放自身锁

```go
func ReleaseEditLock(ctx context.Context, op OperationContext, input ReleaseEditLockInput) (ReleaseEditLockResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReleaseEditLockInput` | 关闭页面释放自身锁的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseEditLockResult` | 关闭页面释放自身锁的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配 |

**步骤**

1. 核lease归属与boot
2. 锁仍匹配则原子删除；缺失视已释放
3. 发编辑状态元数据，关闭beacon失败2分钟TTL兜底

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）

- 副作用：内存锁删除
- 授权：content.edit自身
- 并发：doc mutex
- 幂等：重复release无效果
- 依赖：无

## content.ForceReleaseEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：只有超管强制释放别人锁

```go
func ForceReleaseEditLock(ctx context.Context, op OperationContext, input ForceReleaseEditLockInput) (ForceReleaseEditLockResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ForceReleaseEditLockInput` | 只有超管强制释放别人锁的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ForceReleaseEditLockResult` | 只有超管强制释放别人锁的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变 |

**步骤**

1. superadmin专属，不可分配给子账号
2. 同doc mutex核expectedLease防释放已换人锁，增加fence并删锁
3. 发强制只读事件；已自动保存草稿保留，旧fence所有后续save拒绝

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）

- 副作用：内存fence/锁与审计outbox
- 授权：superadmin
- 并发：与save登记同doc mutex
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：无

## content.ExpireIdleLocks

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：释放失联或提示后仍空闲锁

```go
func ExpireIdleLocks(ctx context.Context, op OperationContext, input ExpireIdleLocksInput) (ExpireIdleLocksResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ExpireIdleLocksInput` | 释放失联或提示后仍空闲锁的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ExpireIdleLocksResult` | 释放失联或提示后仍空闲锁的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 逐doc mutex检查now≥expiresAt或15分钟提示+2分钟无输入
2. 释放提高fence，标事件给在线编辑器
3. 进程重启所有锁清空，bootId改变拒绝旧请求

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：锁表与元数据事件
- 授权：service:content内存maintenance
- 并发：逐docmutex避免全局阻塞
- 幂等：已释放不重复事件
- 依赖：无

## content.ReadDraft

- 级别：约束
- 文件：`modules/content/internal/draft_service.go`
- 作用：读取全篇共享草稿及编辑状态

```go
func ReadDraft(ctx context.Context, op OperationContext, input ReadDraftInput) (ReadDraftResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadDraftInput` | 读取全篇共享草稿及编辑状态的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadDraftResult` | 读取全篇共享草稿及编辑状态的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. content.read，同站点非purged
2. COMMITTED head→ReadRevisionBody校hash
3. 返回draftVersion/lease供editor只读或可写决定；无访客入口

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：content.read
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.ReadRevisionBody`

## content.SaveDraft

- 级别：约束
- 文件：`modules/content/internal/draft_service.go`
- 作用：自动保存完整Tiptap正文

```go
func SaveDraft(ctx context.Context, op OperationContext, input SaveDraftInput) (SaveDraftResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `SaveDraftInput` | 自动保存完整Tiptap正文的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SaveDraftResult` | 自动保存完整Tiptap正文的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. doc mutex检查有效holder/boot/fence/lease，ctx授权与epoch；ValidateBlockTree/IDs
2. CollectResourceReferences先media acquire pins，再WriteRevisionFile不可变fsync
3. 同私有txn再检查lease/fence和expectedHead/version，CommitDraftJournal+refs+receipt/outbox；commit后confirm pins，失败release且隔离未提交文件
4. WriteRevisionFile的expectedHead取已核expectedDraftRevisionId，expectedDraftVersion取原输入；author不由浏览器提供。abort pin释放携sourceOperationId/siteId/sourceObjectId/sourceRevisionId/sourceVersion/referenceDigest、nullable pinId及reason；先持久aborted/rejected，再由media原子封存源，pin未知不得先重新Acquire，COMMITTED不释放

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：content.edit且当前有效锁
- 并发：save登记与锁强释同docmutex；持久head CAS避免同人双tab覆盖；不能持mutex跨无限网络
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.ValidateBlockTree`、`content.CollectResourceReferences`、`content.WriteRevisionFile`、`content.CommitDraftJournal`、`media:acquireResourceReferences`、`media:confirmResourceReferences`、`media:releaseResourceReferences`

## content.ValidateBlockTree

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：验证全部Tiptap块及稳定ID

```go
func ValidateBlockTree(ctx context.Context, op OperationContext, input ValidateBlockTreeInput) (ValidateBlockTreeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidateBlockTreeInput` | 验证全部Tiptap块及稳定ID的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateBlockTreeResult` | 验证全部Tiptap块及稳定ID的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 正文结构/未知节点属性marks由请求入口或持久文件边界生成schema一次校验；本函数信任该结构，不重复schema验证
2. 全树block/item/cell/row/column ID唯一，合计≤10000、结构深度≤32；列表首项paragraph，语义调用Inline/TableGrid/Columns，不改变ID/内容
3. 删除ID历史禁止复用由CommitDraftTransaction.Commit的稳定ID ledger与head CAS同事务落实；previousBody不能代替全部历史，历史读取不做current retired判断

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_SCHEMA`：版本或块 schema 不受支持；禁止静默丢弃未知属性（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：私有正文验证
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.bodyJSONValue`、`content.visit`

## content.ValidateInline

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：验证全部行内格式与链接

```go
func ValidateInline(input ValidateInlineInput) (ValidateInlineResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidateInlineInput` | 验证全部行内格式与链接的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateInlineResult` | 验证全部行内格式与链接的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性 |

**步骤**

1. text非空且无NUL，marks类型至多一次，code正文不作为HTML
2. link内部docId/blockId或http/https/mailto，禁javascript和userinfo
3. inlineMath latex大小有界，不含文件/网络宏；代码/公式与地址不交翻译模型

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_SCHEMA`：版本或块 schema 不受支持；禁止静默丢弃未知属性（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.validateInlineNodes`
- 其他依赖说明：Go内部参数是已过边界schema并用UseNumber转换的JSON节点表示；本树单次转换后真实调用，不重复Marshal/schema；JSON形状由types唯一来源约束

## content.visit

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：全树稳定ID前序去重/总数与深度、列表首段和嵌套语义

```go
func (state *blockTreeValidation) visit(node map[string]any, depth int) error
```

**输入**

无

**输出**

无

**步骤**

1. 全树稳定ID前序去重/总数与深度、列表首段和嵌套语义

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.ValidateInline`、`content.ValidateTableGrid`、`content.ValidateColumns`、`content.validateContentMath`

## content.validateInlineNodes

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：拒行内NUL/重复mark，外链和公式执行已登记语义检查

```go
func validateInlineNodes(nodes []any) error
```

**输入**

无

**输出**

无

**步骤**

1. 拒行内NUL/重复mark，外链和公式执行已登记语义检查

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.validateContentLink`、`content.validateContentMath`

## content.validateContentLink

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：只允许有有效host的http/https或mailto，拒userinfo/控制URL，不记录原值

```go
func validateContentLink(href string) error
```

**输入**

无

**输出**

无

**步骤**

1. 只允许有有效host的http/https或mailto，拒userinfo/控制URL，不记录原值

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.validateContentMath

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：拒文件/网络/HTML命令头input/include/includegraphics/openin/openout/read/write/href/url/htmlClass/htmlId/htmlStyle/htmlData，不运行TeX或模型

```go
func validateContentMath(latex string) error
```

**输入**

无

**输出**

无

**步骤**

1. 拒文件/网络/HTML命令头input/include/includegraphics/openin/openout/read/write/href/url/htmlClass/htmlId/htmlStyle/htmlData，不运行TeX或模型

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.contentInteger

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：读取已校验JSON整数供网格计算

```go
func contentInteger(value any) int
```

**输入**

无

**输出**

无

**步骤**

1. 读取已校验JSON整数供网格计算

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.contentNumber

- 级别：参考
- 文件：`modules/content/internal/block_validator.go`
- 作用：读取已校验JSON数值供分栏误差计算

```go
func contentNumber(value any) float64
```

**输入**

无

**输出**

无

**步骤**

1. 读取已校验JSON数值供分栏误差计算

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.ValidateTableGrid

- 级别：参考
- 文件：`modules/content/internal/table_validator.go`
- 作用：验证表格合并单元格覆盖

```go
func ValidateTableGrid(input ValidateTableGridInput) (ValidateTableGridResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidateTableGridInput` | 验证表格合并单元格覆盖的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateTableGridResult` | 验证表格合并单元格覆盖的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 逐行以rowspan已占位和colspan放origin cells
2. 覆盖矩阵不得重叠/缺洞/越200×30，colwidth长度等colspan
3. header单元格类型保留，所有cell ID唯一，内文递归block校验

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.validateTableGridNode`
- 其他依赖说明：Go内部参数是已过边界schema并用UseNumber转换的JSON节点表示；本树单次转换后真实调用，不重复Marshal/schema；JSON形状由types唯一来源约束

## content.validateTableGridNode

- 级别：参考
- 文件：`modules/content/internal/table_validator.go`
- 作用：行优先放单元格，拒交叠/空洞/越界/零逻辑列及colwidth不等colspan

```go
func validateTableGridNode(table map[string]any) (ValidateTableGridResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 行优先放单元格，拒交叠/空洞/越界/零逻辑列及colwidth不等colspan

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.contentInteger`

## content.ValidateColumns

- 级别：参考
- 文件：`modules/content/internal/columns_validator.go`
- 作用：校验2–4栏比例和禁止嵌套

```go
func ValidateColumns(input ValidateColumnsInput) (ValidateColumnsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `ValidateColumnsInput` | 校验2–4栏比例和禁止嵌套的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidateColumnsResult` | 校验2–4栏比例和禁止嵌套的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值 |

**步骤**

1. content数量2–4与widthRatios长度一致
2. 比例正且总和1（误差≤1e-6），columnId唯一
3. 递归所有后代不得出现columns；窄屏上下排列按列顺序文本展开

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.validateColumnsNode`
- 其他依赖说明：Go内部参数是已过边界schema并用UseNumber转换的JSON节点表示；本树单次转换后真实调用，不重复Marshal/schema；JSON形状由types唯一来源约束

## content.validateColumnsNode

- 级别：参考
- 文件：`modules/content/internal/columns_validator.go`
- 作用：检查比例数量/总和误差及column ID，迭代拒全部后代columns

```go
func validateColumnsNode(node map[string]any) error
```

**输入**

无

**输出**

无

**步骤**

1. 检查比例数量/总和误差及column ID，迭代拒全部后代columns

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.contentNumber`、`content.hasNestedColumns`

## content.hasNestedColumns

- 级别：参考
- 文件：`modules/content/internal/columns_validator.go`
- 作用：迭代检查所有content后代是否columns，避免递归栈兜底

```go
func hasNestedColumns(node map[string]any) bool
```

**输入**

无

**输出**

无

**步骤**

1. 迭代检查所有content后代是否columns，避免递归栈兜底

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.CollectResourceReferences

- 级别：参考
- 文件：`modules/content/internal/reference_extractor.go`
- 作用：扫描所有嵌套块资源引用

```go
func CollectResourceReferences(input CollectResourceReferencesInput) (CollectResourceReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `CollectResourceReferencesInput` | 扫描所有嵌套块资源引用的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CollectResourceReferencesResult` | 扫描所有嵌套块资源引用的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；VALIDATION: 正文JSON硬限或去重后引用数超过10000项 |

**步骤**

1. 对已通过入口schema和上游块树语义的正文作只读JSON表示转换；沿content数组规范前序遍历，包含列表/任务/引述/提示/表格/分栏等全部嵌套层级，不重新校验schema/权限
2. image/attachment取asset；drawing固定scene、lightSVG、darkSVG顺序；uploaded video先取source.asset，platform video先取snapshot.cover，再取video.poster，nullable封面跳过，不抓URL或探测media
3. 每项完整保留assetId/version/sha256/mediaType/byteSize与稳定blockId；version以精确整数转换，不能经float64丢精度，同assetId/version出现不同hash/长度/MIME返回INTEGRITY_ERROR
4. sourceKey为(sourceService=content,documentId,sourceRevisionId,sourceVersion,kind,blockId,assetId,assetVersion)；本次调用固定前五项，同一block同一asset版本只保留首次，不吞掉不同block的引用，保留前序/角色顺序
5. kind原样保留current_draft/current_published/history，retained=true表示本次仍保留正文中的引用，不从状态猜测历史已被删除；去重后超过10000项返回VALIDATION，空结果序列化为[]，不修改输入

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `VALIDATION`：正文JSON硬限或去重后引用数超过10000项（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.bodyJSONValue`、`content.visitReferenceNode`

## content.visitReferenceNode

- 级别：参考
- 文件：`modules/content/internal/reference_extractor.go`
- 作用：按正文前序和固定资源角色顺序访问资源块

```go
func (collector *resourceReferenceCollector) visitReferenceNode(node map[string]any) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅抽取已登记资源槽；nullable平台cover/独立poster分别访问后去重，所有content后代按输入顺序递归

**错误**

- 无

- 副作用：无
- 授权：纯函数；调用方已完成入口结构校验和本域资源授权
- 并发：只读取输入，局部状态不跨调用共享；可以并发调用
- 幂等：相同输入产生相同输出，无持久副作用
- 依赖：`content.appendResourceReference`、`content.visitReferenceNode`

## content.appendResourceReference

- 级别：参考
- 文件：`modules/content/internal/reference_extractor.go`
- 作用：精确资源版本转换、跨槽完整性核对与稳定sourceKey去重

```go
func (collector *resourceReferenceCollector) appendResourceReference(blockID Identifier, value any) error
```

**输入**

无

**输出**

无

**步骤**

1. 跳过nullable资源；version精确转服务整数；同assetId/version的完整不可变元数据必须一致；同block的同版本去重，超过10000拒绝；沿用来源水位/分类并标retained

**错误**

- 无

- 副作用：无
- 授权：纯函数；调用方已完成入口结构校验和本域资源授权
- 并发：只读取输入，局部状态不跨调用共享；可以并发调用
- 幂等：相同输入产生相同输出，无持久副作用
- 依赖：`content.contentInteger`

## content.CanonicalizeBody

- 级别：参考
- 文件：`modules/content/internal/body_canonicalizer.go`
- 作用：规范序列化正文并计算hash

```go
func CanonicalizeBody(input CanonicalizeBodyInput) (CanonicalizeBodyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `CanonicalizeBodyInput` | 规范序列化正文并计算hash的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CanonicalizeBodyResult` | 规范序列化正文并计算hash的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性 |

**步骤**

1. 仅规范对象键，Unicode文字/代码/latex不NFC、不trim、不改变正文数组顺序
2. marks按类型英文名升序作唯一固定顺序，不维护第二份产品类型表
3. 复用platform.CanonicalJSON生成UTF-8规范表示并SHA256；硬限16MiB，站点新写动态大小由WriteRevisionFile负责

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNSUPPORTED_SCHEMA`：版本或块 schema 不受支持；禁止静默丢弃未知属性（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.bodyJSONValue`、`content.canonicalContentValue`

## content.HashBlock

- 级别：参考
- 文件：`modules/content/internal/body_canonicalizer.go`
- 作用：计算翻译/AI引用稳定块hash

```go
func HashBlock(input HashBlockInput) (HashBlockResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `HashBlockInput` | 计算翻译/AI引用稳定块hash的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HashBlockResult` | 计算翻译/AI引用稳定块hash的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性 |

**步骤**

1. 包含全部可呈现attrs、内容、不可变assetrefs和嵌套子内容
2. 排除运行锁/编辑UI句柄，保持blockId一致
3. 规范编码SHA256；修改影响hash，不以时间替代

**错误**

- `UNSUPPORTED_SCHEMA`：版本或块 schema 不受支持；禁止静默丢弃未知属性（不可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.bodyJSONValue`、`content.canonicalContentValue`

## content.bodyJSONValue

- 级别：参考
- 文件：`modules/content/internal/body_canonicalizer.go`
- 作用：将已过边界schema的值转为保整数精度的JSON表示，硬限16MiB

```go
func bodyJSONValue(input any) (any, error)
```

**输入**

无

**输出**

无

**步骤**

1. 将已过边界schema的值转为保整数精度的JSON表示，硬限16MiB

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.canonicalContentValue

- 级别：参考
- 文件：`modules/content/internal/body_canonicalizer.go`
- 作用：规范mark类型次序后调用唯一平台JSON规范化

```go
func canonicalContentValue(value any) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 规范mark类型次序后调用唯一平台JSON规范化

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.orderContentMarks`、`platform.CanonicalJSON`

## content.orderContentMarks

- 级别：参考
- 文件：`modules/content/internal/body_canonicalizer.go`
- 作用：只在新表示中按mark类型英文名升序，不变更调用方正文

```go
func orderContentMarks(value any)
```

**输入**

无

**输出**

无

**步骤**

1. 只在新表示中按mark类型英文名升序，不变更调用方正文

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.WriteRevisionFile

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：持久写权威正文文件

```go
func WriteRevisionFile(ctx context.Context, op OperationContext, input WriteRevisionFileInput) (WriteRevisionFileResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `WriteRevisionFileInput` | 持久写权威正文文件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteRevisionFileResult` | 持久写权威正文文件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. CanonicalizeBody得到≤16MiB规范JSON/hash；从独立WithRevisionFileStore或兼容既有WithDraftCommitStore读取RevisionFileStore，核可信active root/site/打开epoch，路径仅服务生成site/doc/revisionId.json，拒绝路径跳转/链接/特殊文件/多硬链接
2. 原操作按caller/actor/site/dataEpoch/operation及包含doc/revision/expectedHead/expectedDraftVersion/bodyHash的完整规范摘要先查持久准备材料；异摘要CONFLICT；同op使用首次Draft/Journal，不重造时间/commitId或受后来站点配额下降影响
3. 新操作ValidateBlockTree语义并核当前受信maxDocumentBytes（1024..16MiB）；author只取可信op，ObjectRef固定owner=content/objectId=revisionId/version=1、application/json及完整hash/长度；draftVersion=expectedDraftVersion+1，previousRevisionId=expectedHead
4. 公共lifecycle.NewWriteContext建立当前epoch和control截止；受信私有adapter原子保存preparing+摘要+原结果，不推进head；同revision归属冲突拒绝，缺真实adapter直接DEPENDENCY_UNAVAILABLE
5. 单进程私有根文件准备串行到MarkPrepared；prepared/committed原结果只复核既有不可变文件并返回，aborted/quarantined不复活；preparing才续写，不新增自动重试
6. 刷新安全窗口，沿公共ValidatePrivatePath逐段核目录/文件，Mkdir0700并同步所有新父目录；原8字段JSONL追加同commitId PREPARED并fsync，崩溃重复同记录由既有scanner去重，不改变格式或虚称跨库原子
7. tmp0600同目录写入/文件fsync/close，IO后重新取当前窗口和epoch再原子rename+目录fsync；已有目标只核同hash/长度，绝不覆盖既有正文；安装后复核hash/长度/正规路径
8. 真实持久MarkRevisionPrepared在当前安全窗口将preparing推进prepared后才返回原结果；不推进head/ledger/refs/业务COMMITTED receipt/outbox，业务提交仍由CommitDraftTransaction的fence+CAS完成。真实文件/材料损坏按COMMITTED优先持久隔离，未提交只隔离准备而不关闭另一合法head，不fallback，普通IO失败保留可恢复准备决定

**错误**

- `VALIDATION`：正文语义或站点大小不满足（不可重试）
- `CONFLICT`：原操作摘要或修订归属不一致，或原准备已终止（不可重试）
- `VERSION_MISMATCH`：当前安全窗口或打开epoch不匹配（不可重试）
- `LOCKED`：维护/屏障安全窗口关闭（可重试）
- `INTEGRITY_ERROR`：原材料、文件hash/长度/正规路径不一致（不可重试）
- `DEPENDENCY_UNAVAILABLE`：真实存储adapter/配置/文件持久化未知（可重试）
- `CANCELLED`：调用取消（可重试）
- `DEADLINE_EXCEEDED`：IO截止到达（可重试）

- 副作用：正文文件/PREPAREDjournal
- 授权：私有正文提交
- 并发：私有根单进程拥有；文件准备进程内串行，窄adapter原子幂等/修订归属；临时文件不可见，只有业务COMMITTED后可读，不宣称FS与DB单事务
- 幂等：caller/actor/site/epoch/op+完整规范摘要，先取首次原材料再新策略/CAS；同键异摘要冲突，不重造结果；preparing崩溃可续，不复活终局
- 依赖：`content.CanonicalizeBody`、`content.ValidateBlockTree`、`content.revisionFileStoreFromContext`、`content.revisionFileLocation`、`content.revisionPreparationDigest`、`content.validatePreparedMaterial`、`content.newRevisionWriteWindow`、`content.persistPreparedRevision`、`content.verifyRevisionMaterialFile`、`content.quarantineRevisionMaterial`、`platform.NewWriteContext`、`platform.ValidatePrivatePath`、`content.FindRevisionPreparation`、`content.PrepareRevision`、`content.MarkRevisionPrepared`、`content.RevisionMaxDocumentBytes`
- 其他依赖说明：Go实际公共函数为lifecycle.NewWriteContext与lifecycle.ValidatePrivatePath；contract平台前缀不是Go包名；RevisionFileStore所有持久方法需真实本域adapter；本项只实现文件核心，未装配时失败关闭；历史正文硬限16MiB，新写另服站点maxDocumentBytes；不等价上传配额

## content.RecoverPreparedCommits

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：恢复崩溃前正文提交日志

```go
func RecoverPreparedCommits(ctx context.Context, op OperationContext, input RecoverPreparedCommitsInput) (RecoverPreparedCommitsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RecoverPreparedCommitsInput` | 恢复崩溃前正文提交日志的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecoverPreparedCommitsResult` | 恢复崩溃前正文提交日志的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. exclusive启动或safe屏障内扫描journal
2. COMMITTED必须有同epoch私有提交资格及原回执，再查file/hash/head；缺回执启动失败，文件损坏隔离doc并关闭公开，不从JSONL猜业务提交
3. PREPARED若无已提交receipt则隔离，不能自行推进head；保留窗口后仅受控清未见孤儿

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：日志恢复/孤儿隔离
- 授权：service:content启动控制
- 并发：exclusive gate
- 幂等：重复恢复不发布PREPARED
- 依赖：`content.scanCommitJournal`、`content.appendJournalState`

## content.revisionPreparationDigest

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：按caller/actor/site/epoch/动作及完整规范材料生成私有输入摘要

```go
func revisionPreparationDigest(op OperationContext, input WriteRevisionFileInput, bodyHash Hash) (Hash, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按caller/actor/site/epoch/动作及完整规范材料生成私有输入摘要

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`platform.CanonicalJSON`

## content.validatePreparedMaterial

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：核持久原Draft/Journal与当前同摘要材料身份，不重造或覆盖结果

```go
func validatePreparedMaterial(op OperationContext, input WriteRevisionFileInput, result WriteRevisionFileResult, canonical CanonicalizeBodyResult) error
```

**输入**

无

**输出**

无

**步骤**

1. 核持久原Draft/Journal与当前同摘要材料身份，不重造或覆盖结果

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.newRevisionWriteWindow

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：只用lifecycle.NewWriteContext受信回调建立当前epoch与控制截止，不复制公共机制

```go
func newRevisionWriteWindow(ctx context.Context, op OperationContext, store RevisionFileStore) (context.Context, context.CancelFunc, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只用lifecycle.NewWriteContext受信回调建立当前epoch与控制截止，不复制公共机制

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`platform.NewWriteContext`、`content.RevisionWriteWindow`、`content.RevisionDataEpoch`

## content.persistPreparedRevision

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：安全窗口中持久原8字段PREPARED、tmp/fsync/不覆盖rename/父目录及最终hash；崩溃保留准备决定

```go
func persistPreparedRevision(ctx context.Context, op OperationContext, store RevisionFileStore, root, path string, result WriteRevisionFileResult, raw []byte) error
```

**输入**

无

**输出**

无

**步骤**

1. 安全窗口中持久原8字段PREPARED、tmp/fsync/不覆盖rename/父目录及最终hash；崩溃保留准备决定

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.newRevisionWriteWindow`、`content.verifyRevisionMaterialFile`、`content.Read`、`platform.ValidatePrivatePath`、`content.appendJournalState`

## content.verifyRevisionMaterialFile

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：复用既有文件hash/长度/fsync核验并在前后核私有正规路径

```go
func verifyRevisionMaterialFile(ctx context.Context, path string, draft DraftRevision) error
```

**输入**

无

**输出**

无

**步骤**

1. 复用既有文件hash/长度/fsync核验并在前后核私有正规路径

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`platform.ValidatePrivatePath`

## content.quarantineRevisionMaterial

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：将文件写回发现的真实损坏交同一持久隔离边界

```go
func quarantineRevisionMaterial(ctx context.Context, op OperationContext, store RevisionFileStore, input WriteRevisionFileInput, cause error) error
```

**输入**

无

**输出**

无

**步骤**

1. 将文件写回发现的真实损坏交同一持久隔离边界

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.isolateRevisionRead`

## content.scanCommitJournal

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：共享journal读锁内取得严格恢复快照；NOFOLLOW与打开前后身份核对，锁内不取SQL或Store互斥

```go
func scanCommitJournal(ctx context.Context, store RevisionRecoveryStore, journalPath string) ([]journalEntry, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 共享journal读锁内取得严格恢复快照；NOFOLLOW与打开前后身份核对，锁内不取SQL或Store互斥

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.openRevisionJournal`、`content.validateRevisionJournalFile`

## content.appendJournalState

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：共享journal写锁覆盖安全打开、完整追加/fsync/close/目录同步；锁内无SQL

```go
func appendJournalState(ctx context.Context, journalPath string, journal CommitJournal, state string) error
```

**输入**

无

**输出**

无

**步骤**

1. 共享journal写锁覆盖安全打开、完整追加/fsync/close/目录同步；锁内无SQL

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.openRevisionJournal`、`content.validateRevisionJournalFile`

## content.openRevisionJournal

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：公共私有路径验证和标准NOFOLLOW打开，创建仅O_EXCL，拒绝链接与路径替换

```go
func openRevisionJournal(path string, appendMode bool) (*os.File, error)
```

**输入**

无

**输出**

无

**步骤**

1. 公共私有路径验证和标准NOFOLLOW打开，创建仅O_EXCL，拒绝链接与路径替换

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.validateRevisionJournalFile`、`platform.ValidatePrivatePath`

## content.validateRevisionJournalFile

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：读写后复核公共私有正规路径、已打开文件与当前路径SameFile一致

```go
func validateRevisionJournalFile(file *os.File) error
```

**输入**

无

**输出**

无

**步骤**

1. 读写后复核公共私有正规路径、已打开文件与当前路径SameFile一致

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`platform.ValidatePrivatePath`

## content.CommitDraftJournal

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：原子提交共享草稿head

```go
func CommitDraftJournal(ctx context.Context, op OperationContext, input CommitDraftJournalInput) (CommitDraftJournalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CommitDraftJournalInput` | 原子提交共享草稿head的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitDraftJournalResult` | 原子提交共享草稿head的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 核PREPARED文件hash/fsync与有效fence登记
2. 同事务CAS head/version、append稳定IDledger、journal COMMITTED、refs/receipt/outbox
3. CAS失败正文未可见；已commitreceipt返原revision，不能网络失败重复改head
4. 稳定ID ledger必须同head CAS原子拒绝所有历史retired ID复用；仅看previousBody不构成历史检查，不能由文件准备推进ledger

**错误**

- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：私有SaveDraft/Create事务
- 并发：doc rowCAS + fence检查
- 幂等：operationId+canonicaldigest去重
- 依赖：无

## content.ReadRevisionBody

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：读取COMMITTED正文并核真相

```go
func ReadRevisionBody(ctx context.Context, op OperationContext, input ReadRevisionBodyInput) (ReadRevisionBodyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadRevisionBodyInput` | 读取COMMITTED正文并核真相的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadRevisionBodyResult` | 读取COMMITTED正文并核真相的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性 |

**步骤**

1. 从受信RevisionFileStore读取同site/doc/revision/当前epoch且未隔离的COMMITTED资格与完整ObjectRef/Journal，不从8字段JSONL猜draftVersion或接收OSpath；未装adapter失败关闭，PREPARED不能作为正文返回
2. 只按已核标识符生成既有私有路径，ObjectRef固定content/revisionId/version1且journal/hash/归属一致；expectedHash null取该提交索引真相，非null不符只拒绝错误引用，不隔离与权威一致的合法文件
3. 公共ValidatePrivatePath逐段拒链接/特殊/多硬链接；有界读取完整字节，核文件身份、长度≤16MiB及ObjectRef SHA256，不按后来降低的新写配额拒绝历史读，也不截断当成功
4. 持久文件边界一次用生成DocumentBody schema严格验证UTF-8/未知字段/版本，再执行本树语义和规范编码核hash；历史读不查询当前retired ledger，合法历史可含现已删除块ID
5. 读取后再核不可变引用和COMMITTED资格；文件、材料或schema损坏必须真实持久隔离修订/文档并关闭公开，不fallback；普通依赖/取消故障不假装不存在或隔离合法文件

**错误**

- `NOT_FOUND`：该修订无可读COMMITTED资格或不在已授权作用域（不可重试）
- `INTEGRITY_ERROR`：持久文件/引用/严格schema/规范hash矛盾，隔离无fallback（不可重试）
- `UNSUPPORTED_SCHEMA`：持久正文格式未知，不静默迁移或丢字段（不可重试）
- `VERSION_MISMATCH`：打开数据代次不等于当前op（不可重试）
- `DEPENDENCY_UNAVAILABLE`：可信提交索引/adapter/schema或普通IO未知（可重试）
- `CANCELLED`：调用取消（可重试）
- `DEADLINE_EXCEEDED`：IO截止到达（可重试）

- 副作用：损坏时调用真实私有隔离持久方法关闭该修订/文档公开资格；不改写正文或推进head
- 授权：调用方先做content.read或已验证publication/preview
- 并发：COMMITTED资格前后校验，文件不可变且打开epoch固定；不把多个服务或FS/DB读取宣称成单事务
- 幂等：只读
- 依赖：`content.revisionFileStoreFromContext`、`content.revisionFileLocation`、`content.validateCommittedMaterial`、`content.readImmutableRevision`、`content.decodeStoredBody`、`content.ValidateBlockTree`、`content.CanonicalizeBody`、`content.isolateRevisionRead`、`platform.ValidatePrivatePath`、`content.ReadCommittedRevision`

## content.revisionFileStoreFromContext

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：复用既有DraftCommitStore受信注入并要求真实文件扩展adapter

```go
func revisionFileStoreFromContext(ctx context.Context) (RevisionFileStore, error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用既有DraftCommitStore受信注入并要求真实文件扩展adapter

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无
- 其他依赖说明：FindPreparation必须合并私有决定/原JSONL最新状态/COMMITTED receipt，不能用旧准备索引复活启动已隔离孤儿；真实adapter未装配拒绝

## content.revisionFileLocation

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：核打开epoch/site/root，服务生成旧路径并拒点跳转/私有目录异常

```go
func revisionFileLocation(store RevisionFileStore, op OperationContext, documentID, revisionID Identifier) (string, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核打开epoch/site/root，服务生成旧路径并拒点跳转/私有目录异常

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`platform.ValidatePrivatePath`、`content.RevisionRoot`、`content.RevisionDataEpoch`

## content.validateCommittedMaterial

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：核持久COMMITTED材料的doc/revision/hash/前序/版本/对象不可变归属

```go
func validateCommittedMaterial(input ReadRevisionBodyInput, record CommittedRevisionFile) error
```

**输入**

无

**输出**

无

**步骤**

1. 核持久COMMITTED材料的doc/revision/hash/前序/版本/对象不可变归属

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.readImmutableRevision

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：有界核正规文件身份/完整长度/SHA与路径，读后不输出未验证字节

```go
func readImmutableRevision(ctx context.Context, path string, object gen.ObjectRef) ([]byte, error)
```

**输入**

无

**输出**

无

**步骤**

1. 有界核正规文件身份/完整长度/SHA与路径，读后不输出未验证字节

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.Read`、`platform.ValidatePrivatePath`

## content.Read

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：将ctx取消传递给不原生接ctx的文件/字节Reader，纯函数不重复期限解析

```go
func (reader revisionContextReader) Read(buffer []byte) (int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 将ctx取消传递给不原生接ctx的文件/字节Reader，纯函数不重复期限解析

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.decodeStoredBody

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：持久边界一次用生成DocumentBody schema核UTF8/未知字段/版本，不静默迁移

```go
func decodeStoredBody(raw []byte) (DocumentBody, error)
```

**输入**

无

**输出**

无

**步骤**

1. 持久边界一次用生成DocumentBody schema核UTF8/未知字段/版本，不静默迁移

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：无

## content.isolateRevisionRead

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：只有真实材料/文件/schema损坏才持久隔离，普通依赖/取消和调用方错hash不隔离

```go
func isolateRevisionRead(ctx context.Context, op OperationContext, store RevisionFileStore, input ReadRevisionBodyInput, cause error) error
```

**输入**

无

**输出**

无

**步骤**

1. 只有真实材料/文件/schema损坏才持久隔离，普通依赖/取消和调用方错hash不隔离

**错误**

- 无

- 副作用：无
- 授权：仅已授权本域私有正文调用链或受信装配，通用边界不在此重复
- 并发：遵守外层文件串行准备/不可变读取及真实窄存储资格；纯值函数无副作用
- 幂等：纯派生或遵循外层原准备/COMMITTED资格，不推进head或新增付费
- 依赖：`content.QuarantineRevision`

## content.WithRevisionFileStore

- 级别：参考
- 文件：`modules/content/internal/revision_reader.go`
- 作用：独立注入正文文件存储，兼容既有完整草稿存储注入

```go
func WithRevisionFileStore(ctx context.Context, store RevisionFileStore) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 仅注入受信独立存储，不制造完整草稿提交能力

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.PreparePublicationMetadata

- 级别：参考
- 文件：`modules/content/internal/publication_metadata.go`
- 作用：人工优先准备首次slug和SEO

```go
func PreparePublicationMetadata(ctx context.Context, op OperationContext, input PreparePublicationMetadataInput) (PreparePublicationMetadataResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PreparePublicationMetadataInput` | 人工优先准备首次slug和SEO的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PreparePublicationMetadataResult` | 人工优先准备首次slug和SEO的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 人工slug/SEO优先且不被模型覆盖；已有英文slug保持
2. 首次空slug仅在content.slug已配置时经content.InvokeModel生成并ValidatePathSegment；无模型提示补填字段，不造doc-ID/fake结果
3. SEO未人工设且任务可用时content.seo生成；无模型仅提示缺项/人工填写，不能以AI缺项阻塞已提供合法元数据的基础发布

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：可选真实模型调用未来契约；本轮禁止执行
- 授权：content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：父operationId子任务幂等；manual字段不覆写
- 依赖：`content.ValidatePathSegment`、`content.InvokeModel`

## content.PublishDocument

- 级别：约束
- 文件：`modules/content/internal/publication_service.go`
- 作用：点击发布才公开精确共享草稿版本

```go
func PublishDocument(ctx context.Context, op OperationContext, input PublishDocumentInput) (PublishDocumentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PublishDocumentInput` | 点击发布才公开精确共享草稿版本的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishDocumentResult` | 点击发布才公开精确共享草稿版本的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 授权publish+epoch，读COMMITTED指定draft/version；PreparePublicationMetadata人工优先，resources当前ready引用pin
2. 再次CAS metadata/draft/tree水位，CommitPublication原子选immutable发布ref、visibilityRevision/history/作者/outbox
3. 提交后render/index/translation/data git镜像异步由jobs触发；无AI仍基础公开，optional失败不回退已发布正文

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：content.publish
- 并发：doc/treeCAS持久事务，外部计算在事务外以源hash回验
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.ReadRevisionBody`、`content.PreparePublicationMetadata`、`content.CommitPublication`、`media:acquireResourceReferences`、`media:confirmResourceReferences`

## content.WithdrawDocument

- 级别：约束
- 文件：`modules/content/internal/publication_service.go`
- 作用：撤回后回草稿并让公开页面下线

```go
func WithdrawDocument(ctx context.Context, op OperationContext, input WithdrawDocumentInput) (WithdrawDocumentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `WithdrawDocumentInput` | 撤回后回草稿并让公开页面下线的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WithdrawDocumentResult` | 撤回后回草稿并让公开页面下线的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. publish授权核doc非trash
2. 事务清publishedReference、state=draft、增visibilityRevision，保留共享draft和历史
3. 同步validatePublications立即false；outbox通知所有缓存，延迟/乱序不能复活

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：content.publish
- 并发：与publish/trash同docCAS
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.PublishContentEvent`

## content.CommitPublication

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：原子提交当前发布快照与署名历史

```go
func CommitPublication(ctx context.Context, op OperationContext, input CommitPublicationInput) (CommitPublicationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CommitPublicationInput` | 原子提交当前发布快照与署名历史的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CommitPublicationResult` | 原子提交当前发布快照与署名历史的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 确认draft文件hash/ready资源pin并构造公共PublishedRevisionRef
2. 事务CAS当前元数据，publishedReference换新、statepublished、增revision/visibility，writehistory作者账号和serverUTC
3. 同事务发布PublicationChangedPayload+资源refs+data git镜像事件，事件未发送不影响权威公开判定

**错误**

- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：私有publish/rollbacktxn
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.PublishContentEvent`

## content.ReadReferenceDecision

- 级别：约束
- 文件：`modules/content/internal/publication_repository.go`
- 作用：读取资源引用源持久提交决定

```go
func ReadReferenceDecision(ctx context.Context, op OperationContext, input ReadReferenceDecisionInput) (ReadReferenceDecisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | IO取消和截止 | 已在服务入口严格校验；内部不重复schema解析 |
| `op` | `OperationContext` | 已验证服务身份、当前epoch和期限 | 已在服务入口严格校验；内部不重复schema解析 |
| `input` | `ReadReferenceDecisionInput` | 严格封闭输入 | 已在服务入口严格校验；内部不重复schema解析 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadReferenceDecisionResult` | 安全完整响应 | 已在服务入口严格校验；内部不重复schema解析 |
| `error` | `Error` | 公共错误，不返回私密原值 | 已在服务入口严格校验；内部不重复schema解析 |

**步骤**

1. 仅media已认证纯服务、当前active epoch，输入只为源operationId；不接受浏览器自报actor/site/决定
2. 从源操作私有pin intent读取site/object/revision/version/referenceDigest；固定材料在acquire前持久化，只有引用提交操作可查询
3. 源正文COMMITTED决定为权威，SQL待补不当abort；明确aborted/rejected必须持久且防迟到commit；未找到404、跨代409，未知或损坏503/422
4. 只输出共享ReferenceSourceDecision最小字段，no-store；pending/404/timeout都不代表资源可释放，不输出文档正文或私密账户

**错误**

- `VALIDATION`：请求不符合协议（不可重试）
- `UNAUTHENTICATED`：服务证明无效（不可重试）
- `FORBIDDEN`：非固定调用服务或越过只读范围（不可重试）
- `VERSION_MISMATCH`：当前dataEpoch不匹配（不可重试）
- `INTEGRITY_ERROR`：私有权威数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖或存储未知（可重试）

- 副作用：无
- 授权：permission：service；rule：只接受已验证纯服务actor，固定调用方media；不授予模型调用或管理写入权限
- 并发：当前权威快照一致读取；不持锁执行跨服务IO
- 幂等：无写入或费用副作用；每次读当前权威，no-store，不创建后台任务
- 依赖：无

## content.ValidatePublications

- 级别：约束
- 文件：`modules/content/internal/visibility_service.go`
- 作用：同步核公开引用是否仍可见且当前

```go
func ValidatePublications(ctx context.Context, op OperationContext, input ValidatePublicationsInput) (ValidatePublicationsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidatePublicationsInput` | 同步核公开引用是否仍可见且当前的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePublicationsResult` | 同步核公开引用是否仍可见且当前的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 仅注册public-site/knowledge/qa/localization/site-config纯服务调用方或公开不可枚举批次；site-config仅核首页/页脚链接当前公开性，不得读草稿
2. 一次一致txn比dataEpoch/doc状态/currentref/hash/schema与visibilityRevision
3. unknown/notfound返回visible=false而不暴露draft；依赖不可用不能把旧缓存当有效

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：service已登记caller；请求refs只含曾公开对象
- 并发：同txn快照，传输开始前再核最新可见水位
- 幂等：只读
- 依赖：无

## content.ReadPublishedRevision

- 级别：约束
- 文件：`modules/content/internal/publication_reader.go`
- 作用：获取仍公开的精确正文快照

```go
func ReadPublishedRevision(ctx context.Context, op OperationContext, input ReadPublishedRevisionInput) (ReadPublishedRevisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadPublishedRevisionInput` | 获取仍公开的精确正文快照的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadPublishedRevisionResult` | 获取仍公开的精确正文快照的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | NOT_FOUND: 资源不存在或在当前主体作用域中不可见；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 读current published ref并ValidatePublications
2. 只允许当前可见精确ref，不因历史id给访客读撤回历史
3. ReadRevisionBody核hash后传输前再查visibility，改变则STALE_RESULT

**错误**

- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `STALE_RESULT`：事件/计算针对已过期源版本，拒绝覆盖新状态（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：anonymous或已注册service；历史另走publisher history接口
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.ValidatePublications`、`content.ReadRevisionBody`

## content.TrashDocument

- 级别：约束
- 文件：`modules/content/internal/trash_service.go`
- 作用：删除进回收站，可选旧URL替代跳转

```go
func TrashDocument(ctx context.Context, op OperationContext, input TrashDocumentInput) (TrashDocumentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `TrashDocumentInput` | 删除进回收站，可选旧URL替代跳转的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `TrashDocumentResult` | 删除进回收站，可选旧URL替代跳转的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用 |

**步骤**

1. publish授权；非includeSubtree而有子节点必须指定迁移方案否则CONFLICT
2. 先快照所有旧path和公开状态，replacement必须当前可见且不在待删集合
3. 同事务trash节点/清公开ref/增visibility，旧URL410或目标docIdredirect；正文历史保留

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.PlanOldPathRedirects`、`content.ValidatePublications`

## content.RestoreDocument

- 级别：约束
- 文件：`modules/content/internal/trash_service.go`
- 作用：从回收站恢复为草稿

```go
func RestoreDocument(ctx context.Context, op OperationContext, input RestoreDocumentInput) (RestoreDocumentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RestoreDocumentInput` | 从回收站恢复为草稿的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RestoreDocumentResult` | 从回收站恢复为草稿的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. publish权限、目标parent有效无环、slug同父唯一
2. 历史资源存在性报告；文件清理风险不能静默掩盖
3. CAS清trashedAt/state=draft不自动重新发布；旧tombstone按恢复路径规则更新

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.ValidatePathSegment`、`content.EnsureTreeAcyclic`、`media:readAsset`

## content.PurgeDocuments

- 级别：约束
- 文件：`modules/content/internal/trash_service.go`
- 作用：只有超管彻底删除/清空回收站

```go
func PurgeDocuments(ctx context.Context, op OperationContext, input PurgeDocumentsInput) (PurgeDocumentsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PurgeDocumentsInput` | 只有超管彻底删除/清空回收站的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PurgeDocumentsResult` | 只有超管彻底删除/清空回收站的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. superadmin专属；核当前trash集合与confirmationHash，活动文档拒绝
2. 先持久purge决定+墓碑，移除本域正文head/索引/引用并返回410
3. 本域文件经决定日志删除；data git已提交历史由data-protection保留，不删别域目录/自动清媒体

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：本域显式永久清理与历史保留事件
- 授权：superadmin
- 并发：purge/tree互斥，资源引用版本原子提高
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：无

## content.ReadRevisionHistory

- 级别：约束
- 文件：`modules/content/internal/history_service.go`
- 作用：查看署名发布/git版本历史

```go
func ReadRevisionHistory(ctx context.Context, op OperationContext, input ReadRevisionHistoryInput) (ReadRevisionHistoryResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadRevisionHistoryInput` | 查看署名发布/git版本历史的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadRevisionHistoryResult` | 查看署名发布/git版本历史的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. publish功能权限
2. 读取本域history + data git镜像回执，不读取DP目录
3. 显示执行发布账号的署名与时间，即使账号deleted仍保留

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## content.DiffRevisions

- 级别：约束
- 文件：`modules/content/internal/history_service.go`
- 作用：按稳定块ID对比历史版本

```go
func DiffRevisions(ctx context.Context, op OperationContext, input DiffRevisionsInput) (DiffRevisionsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `DiffRevisionsInput` | 按稳定块ID对比历史版本的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `DiffRevisionsResult` | 按稳定块ID对比历史版本的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. publish权限读两个COMMITTED文件
2. 按blockId比较hash与前序父位置
3. 区分文本/属性变化和移动，不将ID变化误配历史

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：确定性
- 依赖：`content.ReadRevisionBody`、`content.HashBlock`

## content.RollbackPublishedRevision

- 级别：约束
- 文件：`modules/content/internal/history_service.go`
- 作用：回滚历史形成新发布/git提交

```go
func RollbackPublishedRevision(ctx context.Context, op OperationContext, input RollbackPublishedRevisionInput) (RollbackPublishedRevisionResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `RollbackPublishedRevisionInput` | 回滚历史形成新发布/git提交的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RollbackPublishedRevisionResult` | 回滚历史形成新发布/git提交的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. publish授权，ReadRevisionHistory核精确历史body/meta
2. 核资源；手动清理后缺文件先提示，需要显式ack才保留缺资源占位而不伪造bytes
3. 新revision恢复历史稿并走CommitPublication，保留旧history、执行回滚账号署名，重发render/index/translation

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.ReadRevisionHistory`、`content.ReadRevisionBody`、`content.CommitPublication`、`media:readAsset`

## content.CreatePreviewGrant

- 级别：约束
- 文件：`modules/content/internal/preview_service.go`
- 作用：管理员预览草稿公开样式

```go
func CreatePreviewGrant(ctx context.Context, op OperationContext, input CreatePreviewGrantInput) (CreatePreviewGrantResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `CreatePreviewGrantInput` | 管理员预览草稿公开样式的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CreatePreviewGrantResult` | 管理员预览草稿公开样式的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. content.read/编辑或发布已登录管理主体
2. 确认稿COMMITTED且创建短期绑定account精确ref grant
3. public-site按此grant渲染并noindex/no-store，不造匿名分享预览

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有短期preview digest
- 授权：content.read，已登录管理员
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：同operationId返回同grant
- 依赖：无

## content.ValidatePreviewGrant

- 级别：约束
- 文件：`modules/content/internal/preview_service.go`
- 作用：只向对应管理员验证草稿预览

```go
func ValidatePreviewGrant(ctx context.Context, op OperationContext, input ValidatePreviewGrantInput) (ValidatePreviewGrantResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ValidatePreviewGrantInput` | 只向对应管理员验证草稿预览的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePreviewGrantResult` | 只向对应管理员验证草稿预览的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. service:public-site核token digest/expiry/currentactor
2. 绑定doc/draft/account/epoch，退出或停用立即失效
3. ReadRevisionBody精确稿，禁止preview token调用其他业务或公开cache

**错误**

- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：service:public-site + 对应管理员session
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`identity:checkAuthorization`、`content.ReadRevisionBody`

## content.ListResourceReferences

- 级别：约束
- 文件：`modules/content/internal/reference_reader.go`
- 作用：向资源库列出哪些文档引用文件

```go
func ListResourceReferences(ctx context.Context, op OperationContext, input ListResourceReferencesInput) (ListResourceReferencesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ListResourceReferencesInput` | 向资源库列出哪些文档引用文件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ListResourceReferencesResult` | 向资源库列出哪些文档引用文件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. media注册service或content.read检查
2. 一致水位全量/分页列current_draft/current_published/history
3. unknown/incomplete不得解释为空引用，返回水位/完成cursor

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：service:media/site-config/data-protection 或content.read
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## content.ReadContentObject

- 级别：约束
- 文件：`modules/content/internal/object_reader.go`
- 作用：经对象API读取私有文件而不共享目录

```go
func ReadContentObject(ctx context.Context, op OperationContext, input ReadContentObjectInput) (ReadContentObjectResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadContentObjectInput` | 经对象API读取私有文件而不共享目录的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadContentObjectResult` | 经对象API读取私有文件而不共享目录的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 注册service核owner/content与purpose
2. 索引解析服务生成path，version/hash一致
3. 短期reader lease提供只读流，媒体或DP不得访问OSroot

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：短期reader lease
- 授权：service:data-protection/localization/knowledge；正文先验证publish或管理委托
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：无

## content.PublishContentEvent

- 级别：参考
- 文件：`modules/content/internal/outbox_repository.go`
- 作用：与发布/撤回提交原子登记最小事件

```go
func PublishContentEvent(ctx context.Context, op OperationContext, input PublishContentEventInput) (PublishContentEventResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `PublishContentEventInput` | 与发布/撤回提交原子登记最小事件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PublishContentEventResult` | 与发布/撤回提交原子登记最小事件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 必须且仅有一payload，禁止正文/密钥
2. 同business事务eventId唯一outbox
3. 只在业务事务内写outbox；提交后由content.RelayOutbox经jobs至少一次投递并在确认后标relayed，本函数不直接调用jobs；乱序由source revision拒绝

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：私有contenttxn
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：eventId唯一
- 依赖：`platform.EnqueueEvent`

## content.RelayOutbox

- 级别：约束
- 文件：`modules/content/internal/outbox_repository.go`
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

## content.AcquireBarrier

- 级别：约束
- 文件：`modules/content/internal/barrier_service.go`
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
- 依赖：`site-config:readRuntimeState`

## content.ReleaseBarrier

- 级别：约束
- 文件：`modules/content/internal/barrier_service.go`
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
- 依赖：`site-config:readRuntimeState`

## content.ExportSnapshot

- 级别：约束
- 文件：`modules/content/internal/snapshot_service.go`
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
- 依赖：`site-config:readRuntimeState`

## content.StageSnapshot

- 级别：约束
- 文件：`modules/content/internal/snapshot_service.go`
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
- 依赖：`data-protection:serveRestoreArchive`、`site-config:readRuntimeState`

## content.DiscardSnapshot

- 级别：约束
- 文件：`modules/content/internal/snapshot_service.go`
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
- 依赖：`site-config:readRuntimeState`

## content.ReadPublicTree

- 级别：约束
- 文件：`modules/content/internal/public_route_reader.go`
- 作用：公开导航读取已发布文档树

```go
func ReadPublicTree(ctx context.Context, op OperationContext, input ReadPublicTreeInput) (ReadPublicTreeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ReadPublicTreeInput` | 公开导航读取已发布文档树的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadPublicTreeResult` | 公开导航读取已发布文档树的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布 |

**步骤**

1. 过滤所有非published/visibility不可见
2. 按全站树前序输出公开DTO，未公开父不泄漏其标题，仍发布子按公开导航规则接最近公开祖先
3. 输出统一水位；公开previous/next依据当前语言可用文档映射
4. 同一公开树权威快照返回每节点完整canonicalPath，包括可读路由所需祖先slug但不附未公开祖先标题/正文；跳过父节点不使消费者丢失路径，treeRevision变化时旧路径失效

**错误**

- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）

- 副作用：无
- 授权：anonymous/service:public-site/knowledge
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.ValidatePublications`

## content.ResolvePublicPath

- 级别：约束
- 文件：`modules/content/internal/public_route_reader.go`
- 作用：解析当前路径、旧路径301与删除410

```go
func ResolvePublicPath(ctx context.Context, op OperationContext, input ResolvePublicPathInput) (ResolvePublicPathResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造 | deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查 |
| `input` | `ResolvePublicPathInput` | 解析当前路径、旧路径301与删除410的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolvePublicPathResult` | 解析当前路径、旧路径301与删除410的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 规范path禁点段/重复编码
2. 优先活动公开path；tombstone oldpath→docId解析当前canonical，target已删则410而非环
3. found必须ValidatePublications当前；全语言共用slug，返回状态让public-site输出相应HTTP

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：anonymous/service:public-site
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.ValidatePublications`

## content.createEditorSession

- 级别：参考
- 文件：`modules/content/web/src/editor/editor-runtime.ts`
- 作用：创建Tiptap3/Vue编辑器会话

```ts
createEditorSession(input: createEditorSessionInput, signal: AbortSignal): Promise<createEditorSessionResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `createEditorSessionInput` | 创建Tiptap3/Vue编辑器会话的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `createEditorSessionResult` | 创建Tiptap3/Vue编辑器会话的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 按16类扩展和稳定ID建立editor，未知schema只读提示
2. 装入共享draftVersion与lease，别人持锁只读显示holder
3. 基础编辑/预览/发布不因AI缺项禁用

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.applyBlockAction

- 级别：参考
- 文件：`modules/content/web/src/editor/block-actions.ts`
- 作用：拖拽手柄菜单排序/转换/复制/删除

```ts
applyBlockAction(input: applyBlockActionInput, signal: AbortSignal): Promise<applyBlockActionResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `applyBlockActionInput` | 拖拽手柄菜单排序/转换/复制/删除的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `applyBlockActionResult` | 拖拽手柄菜单排序/转换/复制/删除的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 拖动保持原ID，复制产生新ID，删除ID标记不复用
2. 转换保留语义与被允许attrs，无法无损转换先呈现确认差异
3. Tiptap事务更新dirty，不做块引用/反向链接/图谱

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.resolveSlashMenu

- 级别：参考
- 文件：`modules/content/web/src/editor/slash-menu.ts`
- 作用：slash插入菜单与Markdown输入快捷键

```ts
resolveSlashMenu(input: resolveSlashMenuInput, signal: AbortSignal): Promise<resolveSlashMenuResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `resolveSlashMenuInput` | slash插入菜单与Markdown输入快捷键的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `resolveSlashMenuResult` | slash插入菜单与Markdown输入快捷键的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. /显示16类与两/三栏插入
2. ##到H2、-/1.列表、代码围栏规则受控转换
3. 选中文字BubbleMenu仅已确认8项行内能力

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.normalizePastedContent

- 级别：参考
- 文件：`modules/content/web/src/editor/paste-adapter.ts`
- 作用：Word/网页/Markdown粘贴清理并转换块

```ts
normalizePastedContent(input: normalizePastedContentInput, signal: AbortSignal): Promise<normalizePastedContentResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `normalizePastedContentInput` | Word/网页/Markdown粘贴清理并转换块的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `normalizePastedContentResult` | Word/网页/Markdown粘贴清理并转换块的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. DOM白名单转Tiptap不执行script/style事件/远程主动内容
2. 将常见段落标题列表表格代码转换稳定新块ID
3. 无法保留格式降为文字并提示；不是D30排除的批量导入功能

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.scheduleAutosave

- 级别：参考
- 文件：`modules/content/web/src/editor/autosave-controller.ts`
- 作用：去抖自动保存共享草稿

```ts
scheduleAutosave(input: scheduleAutosaveInput, signal: AbortSignal): Promise<scheduleAutosaveResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `scheduleAutosaveInput` | 去抖自动保存共享草稿的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `scheduleAutosaveResult` | 去抖自动保存共享草稿的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 文本变更500ms去抖、最长2秒触发saveDraft，保持pending operationId
2. 串行保存拿新head/version，后一请求不覆盖未经确认的服务器版本
3. VERSION_MISMATCH保留本地未存差异并只读提示，禁止静默last-write-wins

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.SaveDraft`

## content.maintainEditHeartbeat

- 级别：参考
- 文件：`modules/content/web/src/editor/lock-client.ts`
- 作用：每30秒锁心跳与15分钟空闲提醒

```ts
maintainEditHeartbeat(input: maintainEditHeartbeatInput, signal: AbortSignal): Promise<maintainEditHeartbeatResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `maintainEditHeartbeatInput` | 每30秒锁心跳与15分钟空闲提醒的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `maintainEditHeartbeatResult` | 每30秒锁心跳与15分钟空闲提醒的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 有效锁每30秒renew且传输入观测
2. 15分钟无输入提示，宽限2分钟无输入后只读释放
3. 关闭release beacon；网络失联最多2分钟后服务释放，旧boot/fence不能保存

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.RenewEditLock`

## content.applyForcedReadOnly

- 级别：参考
- 文件：`modules/content/web/src/editor/lock-client.ts`
- 作用：强释/失锁后立刻只读

```ts
applyForcedReadOnly(input: applyForcedReadOnlyInput, signal: AbortSignal): Promise<applyForcedReadOnlyResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `applyForcedReadOnlyInput` | 强释/失锁后立刻只读的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `applyForcedReadOnlyResult` | 强释/失锁后立刻只读的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 比较fence/boot判定失锁
2. 停autosave与编辑输入，保留已自动保存稿及未存本地差异
3. 提示管理员谁释放/如何重获锁，不自动丢弃或覆盖服务器稿

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.selectInternalLink

- 级别：参考
- 文件：`modules/content/web/src/editor/internal-link-picker.ts`
- 作用：插入文档选择器生成ID站内链接

```ts
selectInternalLink(input: selectInternalLinkInput, signal: AbortSignal): Promise<selectInternalLinkResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `selectInternalLinkInput` | 插入文档选择器生成ID站内链接的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `selectInternalLinkResult` | 插入文档选择器生成ID站内链接的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 从当前后台tree选择docId
2. 只存docId/blockId，不存当前URL/语言
3. 公开渲染按当前语言转路由

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.prepareImageUploads

- 级别：参考
- 文件：`modules/content/web/src/editor/image-uploader.ts`
- 作用：浏览器生成多宽WebP连原图上传

```ts
prepareImageUploads(input: prepareImageUploadsInput, signal: AbortSignal): Promise<prepareImageUploadsResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `prepareImageUploadsInput` | 浏览器生成多宽WebP连原图上传的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `prepareImageUploadsResult` | 浏览器生成多宽WebP连原图上传的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 识别动图GIF/动态WebP/AVIF则只保原图不压缩
2. 静图browser-image-compression生成640/1280/1920不放大原尺寸WebP
3. 每文件SHA内容寻址独立上传并关联variants，服务端不图像处理

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.openDrawingEditor

- 级别：参考
- 文件：`modules/content/web/src/editor/drawing-wrapper.ts`
- 作用：Vue包装Excalidraw并按需加载

```ts
openDrawingEditor(input: openDrawingEditorInput, signal: AbortSignal): Promise<openDrawingEditorResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `openDrawingEditorInput` | Vue包装Excalidraw并按需加载的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `openDrawingEditorResult` | Vue包装Excalidraw并按需加载的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 点击预览才lazy load React+Excalidraw MIT
2. 获取已保存.excalidraw场景恢复全屏编辑
3. 主编辑器未打开画板不加载React

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.saveDrawingFiles

- 级别：参考
- 文件：`modules/content/web/src/editor/drawing-wrapper.ts`
- 作用：保存场景和亮暗SVG三个内容寻址文件

```ts
saveDrawingFiles(input: saveDrawingFilesInput, signal: AbortSignal): Promise<saveDrawingFilesResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `saveDrawingFilesInput` | 保存场景和亮暗SVG三个内容寻址文件的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `saveDrawingFilesResult` | 保存场景和亮暗SVG三个内容寻址文件的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. Excalidraw导出scene JSON、亮SVG、暗SVG
2. 浏览器计算SHA分别upload；全部ready后更新drawing块三个ref
3. 新编辑只换新hash，旧文件历史保留，公开只SVG不加载React

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.prepareVideoBlock

- 级别：参考
- 文件：`modules/content/web/src/editor/video-insertion.ts`
- 作用：统一上传/平台视频插入

```ts
prepareVideoBlock(input: prepareVideoBlockInput, signal: AbortSignal): Promise<prepareVideoBlockResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `prepareVideoBlockInput` | 统一上传/平台视频插入的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `prepareVideoBlockResult` | 统一上传/平台视频插入的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 上传格式验证且浏览器截poster可自传，服务不转码
2. 平台仅插入时readVideoMetadata抓公开信息/本地封面，失败让用户手填
3. 生成统一source/title/description/poster attrs；平台播放器点击后加载

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`media:readVideoMetadata`

## content.resizeColumns

- 级别：参考
- 文件：`modules/content/web/src/editor/columns-resizer.ts`
- 作用：调整2–4栏宽比例

```ts
resizeColumns(input: resizeColumnsInput, signal: AbortSignal): Promise<resizeColumnsResult>
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `input` | `resizeColumnsInput` | 调整2–4栏宽比例的已声明参数 | 必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema |
| `signal` | `AbortSignal` | 界面导航及取消信号 | 取消后不更新已卸载组件 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `resizeColumnsResult` | 调整2–4栏宽比例的完整成功值 | 无 error 时才有效；满足 types.json schema |
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许 |

**步骤**

1. 数量匹配并总和1
2. 拖进列保块ID，禁止列内任何后代columns
3. 存attrs.widthRatios，窄屏按列顺序叠排

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：浏览器Tiptap状态/有界网络请求
- 授权：浏览器当前主体content.edit；后端saveDraft再次验证
- 并发：每editor会话串行事务；保存单飞，失锁立即取消新写
- 幂等：编辑动作依TiptaptransactionID去重，网络写operationId去重
- 依赖：`content.ValidateBlockTree`

## content.main

- 级别：约束
- 文件：`modules/content/cmd/content/main.go`
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
- 依赖：`content.BuildApplication`、`content.Serve`、`content.Shutdown`、`platform.RunService`

## content.BuildApplication

- 级别：约束
- 文件：`modules/content/internal/app/wire.go`
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
2. 先读control区恢复日志与site-config runtime：phase为restore_switching且本域有该restoreId的staged目录、尚未换入时，把active改名为restore/<restoreId>/before、staged改名为active并fsync父目录后写control日志；phase为restore_rolling_back且已换入时反向换回；换目录后丢弃旧dataEpoch未转发outbox、租约与可重建缓存。随后打开active区DB运行schema迁移与文件journal恢复；control区不随恢复替换；本轮仅在SwapNone时真实OpenDatabase/RecoverDatabaseJournal；恢复换目录后的epoch重绑定与activation未交付则保持失败关闭，不能按请求覆写既有epoch
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；维护期间只开放健康/就绪与控制管理接口、不启动worker；本次启动完成换目录或换回时调用site-config:registerActivation登记回执；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源
6. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`content.RecoverPreparedCommits`、`content.ExpireIdleLocks`、`content.RelayOutbox`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`content.OpenDatabase`、`content.RecoverDatabaseJournal`、`content.CloseDatabase`、`content.ConfigureRevisionStorage`、`content.WithRevisionFileStore`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）；完整handler装配时构造ConfigureRevisionStorage并通过WithRevisionFileStore传递；本轮只有Open/Recover真实接入，完整装配继续blocked

## content.Serve

- 级别：约束
- 文件：`modules/content/internal/transport/server.go`
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
- 依赖：`content.BuildApplication`、`content.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## content.Shutdown

- 级别：约束
- 文件：`modules/content/internal/transport/server.go`
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

## content.Health

- 级别：约束
- 文件：`modules/content/internal/transport/server.go`
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

## content.Ready

- 级别：约束
- 文件：`modules/content/internal/transport/server.go`
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

## content.mountModuleUI

- 级别：约束
- 文件：`modules/content/web/src/main.ts`
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

## content.InvokeModel

- 级别：参考
- 文件：`modules/content/internal/model_client.go`
- 作用：经模型网关执行一次调用并读取到终态

```go
func InvokeModel(ctx context.Context, op OperationContext, input InvokeModelInput) (InvokeModelResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `InvokeModelInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InvokeModelResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

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
- 依赖：`model-gateway:createInference`、`model-gateway:resumeInference`、`model-gateway:cancelInference`、`platform.InvokeModel`

## content.OpenDatabase

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：打开私有SQLite、核安全路径、执行只追加迁移及严格绑定dataEpoch

```go
func OpenDatabase(ctx context.Context, dataRoot string, epoch Identifier) (*Store, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只有本次O_EXCL创建的新空库可将可信openedEpoch在首个schema迁移和迁移记录的同SQL事务初始化；既有未知/异代库失败关闭
2. 逐层ValidatePrivatePath验证active与DB/sidecar，SQLite采用WAL/foreign_keys/busy_timeout3000/synchronousFULL单连接，成功后目录fsync

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.RunContentMigrations`

## content.RunContentMigrations

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：事务应用连续内嵌迁移，版本/文件名/hash未知时拒绝启动

```go
func (s *Store) RunContentMigrations(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 读取内嵌连续迁移，已应用版本/文件名/SQL hash必须精确匹配
2. 每个新增版本SQL与迁移记录同事务；可信首次新库的store_identity也在第1迁移同事务初始化，不能另开epoch插入窗口

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.All`

## content.ConfigureRevisionStorage

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：返回独立受信正文写入view，不冒充完整DraftCommitStore

```go
func (s *Store) ConfigureRevisionStorage(config RevisionStorageConfig) RevisionFileStore
```

**输入**

无

**输出**

无

**步骤**

1. 返回独立受信正文写入view，不冒充完整DraftCommitStore

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.FindRevisionPreparation`、`content.PrepareRevision`、`content.MarkRevisionPrepared`、`content.ReadCommittedRevision`、`content.QuarantineRevision`、`content.RevisionRoot`、`content.RevisionDataEpoch`、`content.RevisionMaxDocumentBytes`、`content.RevisionWriteWindow`

## content.RevisionRoot

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：读取本域active根

```go
func (s *revisionStorage) RevisionRoot() string
```

**输入**

无

**输出**

无

**步骤**

1. 读取本域active根

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.RevisionDataEpoch

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：读取严格绑定的活动epoch

```go
func (s *revisionStorage) RevisionDataEpoch() Identifier
```

**输入**

无

**输出**

无

**步骤**

1. 读取严格绑定的活动epoch

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.RevisionMaxDocumentBytes

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：读取本次上游授权获得的站点正文限额

```go
func (s *revisionStorage) RevisionMaxDocumentBytes() int64
```

**输入**

无

**输出**

无

**步骤**

1. 读取本次上游授权获得的站点正文限额

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.RevisionWriteWindow

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：读取受信公共写窗口回调

```go
func (s *revisionStorage) RevisionWriteWindow() platform.WriteWindowConfig
```

**输入**

无

**输出**

无

**步骤**

1. 读取受信公共写窗口回调

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.FindRevisionPreparation

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：按完整scope及摘要读首次原结果，合并JSONL终局和真实提交回执，不复活隔离

```go
func (s *revisionStorage) FindRevisionPreparation(ctx context.Context, op OperationContext, digest Hash) (PreparedRevisionFile, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按完整scope及摘要读首次原结果，合并JSONL终局和真实提交回执，不复活隔离
2. 持久prepared/committed或真实receipt已提交时必须有完整匹配JSONL，日志缺失/截空INTEGRITY_ERROR；preparing尚未append的合法窗口不误拒

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.findRevisionPreparation`

## content.PrepareRevision

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：同事务首次绑定空库站点并持久preparing原材料，重复返原结果、修订归属冲突拒绝

```go
func (s *revisionStorage) PrepareRevision(ctx context.Context, op OperationContext, digest Hash, result WriteRevisionFileResult) (PreparedRevisionFile, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同事务首次绑定空库站点并持久preparing原材料，重复返原结果、修订归属冲突拒绝

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.findRevisionPreparation`

## content.MarkRevisionPrepared

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：核scope/commit及JSONL准备状态后只推进preparing到prepared

```go
func (s *revisionStorage) MarkRevisionPrepared(ctx context.Context, op OperationContext, commitID Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 核scope/commit及JSONL准备状态后只推进preparing到prepared

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.findRevisionPreparation`、`content.preparationJournalState`

## content.ReadCommittedRevision

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：只读同scope/epoch有私有COMMITTED修订与回执且未隔离的完整资格，事务不跨文件IO

```go
func (s *revisionStorage) ReadCommittedRevision(ctx context.Context, op OperationContext, documentID, revisionID Identifier) (CommittedRevisionFile, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读同scope/epoch有私有COMMITTED修订与回执且未隔离的完整资格，事务不跨文件IO

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.checkRevisionScope`、`content.decodeStorageJSON`、`content.sameStoredDraft`、`content.preparationJournalState`

## content.QuarantineRevision

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：事务隔离真实准备；已提交损坏同时持久关闭文档公开资格，未提交不关闭合法head

```go
func (s *revisionStorage) QuarantineRevision(ctx context.Context, op OperationContext, documentID, revisionID Identifier, reason string) error
```

**输入**

无

**输出**

无

**步骤**

1. 事务隔离真实准备；已提交损坏同时持久关闭文档公开资格，未提交不关闭合法head

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.checkRevisionScope`

## content.CurrentHead

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：恢复只读已绑定站点真实draft head，不推进head

```go
func (s *Store) CurrentHead(ctx context.Context, documentID Identifier) (Identifier, error)
```

**输入**

无

**输出**

无

**步骤**

1. 恢复只读已绑定站点真实draft head，不推进head

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.HasCommittedReceipt

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：恢复查询同epoch真实COMMITTED资格与原回执，不把JSONL当业务提交

```go
func (s *Store) HasCommittedReceipt(ctx context.Context, commitID Identifier) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 恢复查询同epoch真实COMMITTED资格与原回执，不把JSONL当业务提交

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.hasCommittedReceipt`

## content.QuarantineDocument

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：独占恢复阶段持久隔离文档并关闭公开资格

```go
func (s *Store) QuarantineDocument(ctx context.Context, documentID Identifier, reason string) error
```

**输入**

无

**输出**

无

**步骤**

1. 独占恢复阶段持久隔离文档并关闭公开资格

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.RecoverDatabaseJournal

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：独占启动读取真实站点绑定、运行既有正文恢复并同步隔离终局，不伪造站点或head

```go
func (s *Store) RecoverDatabaseJournal(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 独占启动读取真实站点绑定、运行既有正文恢复并同步隔离终局，不伪造站点或head

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.RecoverPreparedCommits`、`content.CurrentHead`、`content.HasCommittedReceipt`、`content.QuarantineDocument`、`content.validatePreparedJournalCoverage`

## content.CloseDatabase

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：关闭私有SQLite连接，不删除正文或隔离记录

```go
func (s *Store) CloseDatabase() error
```

**输入**

无

**输出**

无

**步骤**

1. 关闭私有SQLite连接，不删除正文或隔离记录

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.findRevisionPreparation

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：在存储互斥内读取首次材料并合并终局

```go
func (s *Store) findRevisionPreparation(ctx context.Context, op OperationContext, digest Hash) (PreparedRevisionFile, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在存储互斥内读取首次材料并合并终局

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.checkRevisionScope`、`content.decodePreparation`、`content.preparationJournalState`、`content.hasCommittedReceipt`、`content.contentStorageError`

## content.checkRevisionScope

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：私有库持久站点及epoch与可信操作严格一致，空站点只允许未命中及后续事务绑定

```go
func (s *Store) checkRevisionScope(ctx context.Context, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 私有库持久站点及epoch与可信操作严格一致，空站点只允许未命中及后续事务绑定

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.hasCommittedReceipt

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：联查真实修订及scope回执并核首次Draft一致，隔离不抹去既有提交决定

```go
func (s *Store) hasCommittedReceipt(ctx context.Context, commitID Identifier) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 联查真实修订及scope回执并核首次Draft一致，隔离不抹去既有提交决定

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.decodeStorageJSON`、`content.sameStoredDraft`

## content.preparationJournalState

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：只读严格JSONL与合法状态迁移，核同commit身份，不制造或回退决定

```go
func (s *Store) preparationJournalState(ctx context.Context, expected CommitJournal) (string, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只读严格JSONL与合法状态迁移，核同commit身份，不制造或回退决定

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.openRevisionJournal`、`content.validateRevisionJournalFile`

## content.decodePreparation

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：持久原材料边界严格JSON及scope身份校验

```go
func decodePreparation(raw []byte, row *preparationRow, op OperationContext) error
```

**输入**

无

**输出**

无

**步骤**

1. 持久原材料边界严格JSON及scope身份校验

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.decodeStorageJSON`

## content.decodeStorageJSON

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：持久记录边界拒绝未知字段及额外JSON值

```go
func decodeStorageJSON(raw []byte, value any) error
```

**输入**

无

**输出**

无

**步骤**

1. 持久记录边界拒绝未知字段及额外JSON值

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.sameStoredDraft

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：比较持久回执与首次Draft完整JSON值

```go
func sameStoredDraft(left, right DraftRevision) bool
```

**输入**

无

**输出**

无

**步骤**

1. 比较持久回执与首次Draft完整JSON值

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.contentStorageError

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：产生不泄漏路径或原始SQLite消息的标准依赖错误

```go
func contentStorageError() error
```

**输入**

无

**输出**

无

**步骤**

1. 产生不泄漏路径或原始SQLite消息的标准依赖错误

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无

## content.validatePreparedJournalCoverage

- 级别：参考
- 文件：`modules/content/internal/storage.go`
- 作用：启动反向核所有持久prepared/committed准备都被完整同材料JSONL覆盖，缺失或截空失败关闭

```go
func (s *Store) validatePreparedJournalCoverage(ctx context.Context, recovery RevisionRecoveryStore) error
```

**输入**

无

**输出**

无

**步骤**

1. 先在共享journal读锁下取得严格快照，释放锁后才读SQL，锁内没有SQL
2. 逐条核所有prepared/committed和真实committed索引的首次scope/epoch/完整Draft及Journal；缺日志或身份不匹配INTEGRITY_ERROR，不猜补
3. 尚处preparing且无提交资格的首次窗口允许没有日志，其他已完成准备/提交缺失不能被空扫描掩盖

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.scanCommitJournal`、`content.decodePreparation`

## content.All

- 级别：参考
- 文件：`modules/content/migrations/migrations.go`
- 作用：按文件名排序读取内嵌SQL并校验四位编号从1连续

```go
func All() ([]Migration, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按文件名排序读取内嵌SQL并校验四位编号从1连续

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：无
