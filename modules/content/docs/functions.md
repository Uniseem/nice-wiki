# content 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 261 个函数：约束 42 个，参考 219 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

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

1. 即时检查content.edit，经site-config读取基准语言与当前正文限额
2. 读取完整本站treeRevision与元数据快照，核parent非trash及完整32层树，生成固定docId/空Tiptap doc/fractional order并先持久专用创建意图
3. WriteRevisionFile文件IO后，创建专用短SQL以站点树CAS和doc不存在谓词原子建立metadata/head/ledger/COMMITTED/refs/完整原回执/outbox；不取得通用恒成功保存fence

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：私有固定意图、正文资格、稳定ID或完整原回执不一致（不可重试）
- `CONFLICT`：同操作异动作或摘要，或意图已终止（不可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.draftRuntimeFromContext`、`content.authorizeEditLock`、`content.draftRequestDigest`、`content.readDraftOperation`、`content.readDraftSiteConfig`、`content.planDocumentCreation`、`content.draftWriteContext`、`content.prepareDraftOperation`、`content.executeDraftCommit`、`content.decodeStorageJSON`、`site-config:readSiteConfig`

## content.draftWriteContext

- 级别：参考
- 文件：`modules/content/internal/document_service.go`
- 作用：在doc互斥和SQL之前读取当前公共写窗口

```go
func draftWriteContext(ctx context.Context,op OperationContext,runtime DraftRuntime) (context.Context,context.CancelFunc,error)
```

**输入**

无

**输出**

无

**步骤**

1. 在doc互斥和SQL之前读取当前公共写窗口

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`platform.NewWriteContext`

## content.contentServiceContext

- 级别：参考
- 文件：`modules/content/internal/document_service.go`
- 作用：把本站内部引用与配置只读请求装配为content纯服务身份

```go
func contentServiceContext(ctx context.Context,op OperationContext) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 把本站内部引用与配置只读请求装配为content纯服务身份

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.readDraftSiteConfig

- 级别：参考
- 文件：`modules/content/internal/document_service.go`
- 作用：经版本化API读取初始化基准语言及当前正文大小限额

```go
func readDraftSiteConfig(ctx context.Context,op OperationContext,runtime DraftRuntime) (string,int64,error)
```

**输入**

无

**输出**

无

**步骤**

1. 经版本化API读取初始化基准语言及当前正文大小限额

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.contentServiceContext`、`site-config:readSiteConfig`

## content.planDocumentCreation

- 级别：参考
- 文件：`modules/content/internal/document_service.go`
- 作用：以站点树一致快照生成首次空正文文档与fractional顺序材料

```go
func (s *Store) planDocumentCreation(ctx context.Context,op OperationContext,input CreateDocumentInput,language string,body DocumentBody) (draftOperationPlan,error)
```

**输入**

无

**输出**

无

**步骤**

1. 以站点树一致快照生成首次空正文文档与fractional顺序材料

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.checkRevisionScope`、`content.readCreationTree`、`content.fractionalDocumentOrder`、`content.sameIdentifierPointer`、`content.EnsureTreeAcyclic`、`content.CanonicalizeBody`

## content.readCreationTree

- 级别：参考
- 文件：`modules/content/internal/document_service.go`
- 作用：读取完整站点树版本与文档元数据一致快照

```go
func readCreationTree(ctx context.Context, q creationQuery, siteID Identifier, validateMetadata func(json.RawMessage) error) (int64, []gen.DocumentMetadata, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取完整站点树版本与文档元数据一致快照
2. 可选持久metadata schema在原JSON解码前只校验一次；仅新树查询/操作分支排除purged墓碑并扫描最多10001行，超过10000明确503而不截断；旧Create nil回调保持原无新增总量限制，不新增站点业务配额

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.decodeStorageJSON`、`content.contentStorageError`

## content.fractionalDocumentOrder

- 级别：参考
- 文件：`modules/content/internal/document_service.go`
- 作用：计算有界字典序中点，饱和时显式要求重排

```go
func fractionalDocumentOrder(lower,upper string) (string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 计算有界字典序中点，饱和时显式要求重排

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.validateCreateTransaction

- 级别：参考
- 文件：`modules/content/internal/document_service.go`
- 作用：同SQL核受信创建意图、文档不存在、父节点及站点树CAS

```go
func (t *draftCommitTransaction) validateCreateTransaction(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 同SQL核受信创建意图、文档不存在、父节点及站点树CAS

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.readCreationTree`、`content.EnsureTreeAcyclic`

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

1. 仅portal当前admin，即时content.read一次且在DB前；平台服务/epoch/schema边界仍严格执行
2. 同SQL核持久站点/代次、treeRevision、原始metadata schema、metadata/head/COMMITTED资格和完整树；无环/深度32/同父活动key和slug唯一，最多10000节点，损坏不输出部分伪树
3. 按parent/orderKey/ID构建活动draft/published前序，回收站独立ListDocuments，no-store不返回正文

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `UNAUTHENTICATED`：当前会话或委托无效（不可重试）
- `VERSION_MISMATCH`：当前代次/树/元数据CAS已变化或耗尽（不可重试）
- `CONFLICT`：同操作动作/摘要复用或同父路径冲突（不可重试）

- 副作用：无
- 授权：content.read
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.treeRuntimeFromContext`、`content.authorizeTree`、`content.beginTreeRead`、`content.readTreeSnapshot`、`content.treePreorder`

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

1. 仅portal当前admin；draft/published需content.read，trash需content.publish，在DB前一次即时授权
2. 同SQL完整读取真实树/元数据/head资格并核treeRevision，按一致前序筛state；不返回正文，最多100项，no-store
3. 签名cursor绑定site/epoch/caller/actor/会话/权限版本/state/limit及treeRevision/reference_scan_state来源水位/位置；同一SQL读取两个真实水位，跨页任一水位变化409首批重读，末页省略cursor；签名私钥IO前释放SQL
4. 0005来源触发器覆盖documents/state、document_metadata全部字段（含title/draftRevisionId）、draft_heads、提交资格/原回执/preparations、隔离及持久身份；SaveDraft不推进treeRevision时由来源水位保证完整metadata分页不混快照。来源非metadata变化也可保守拒旧页，不新增公共机制/持久读快照

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：当前会话或委托无效（不可重试）
- `VERSION_MISMATCH`：当前代次/树/元数据CAS已变化或耗尽（不可重试）
- `INTEGRITY_ERROR`：持久树、原回执或正文资格不完整（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖或完整树预算不可用（可重试）
- `CONFLICT`：同操作动作/摘要复用或同父路径冲突（不可重试）

- 副作用：无
- 授权：content.read；trash需content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.treeRuntimeFromContext`、`content.authorizeTree`、`content.beginTreeRead`、`content.readTreeSnapshot`、`content.treePreorder`、`content.decodeStorageJSON`、`platform.SignCursor`、`platform.VerifyCursor`

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

1. 仅portal当前admin；content.edit在任何DB/锁前即时核一次，先查同scope动作/规范摘要完整原回执，命中回放不重做CAS
2. 当前平台窗口在Store.mu/短SQL前建立；同SQL核身份/treeRevision/metadata/head/提交资格；EnsureTreeAcyclic禁止移入自身/后代且整棵子树不超32层，新父和相邻必须属于活动树
3. 父改变先PlanOldPathRedirects完整规划曾发布节点和后代旧路径；未知路径失败关闭。非空redirects写入同一事务的版本化manifest全量快照，不另造发布正文
4. 支持的路径或同父顺序分支复用有界base62 fractional order，并核同父活动slug唯一；同SQL treeRevision CAS、metadata revision CAS、parent/order、manifest、完整原回执与content.tree.changed outbox原子提交；未知Commit只凭原回执重试

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久树、原回执或正文资格不完整（不可重试）
- `CONFLICT`：同操作动作/摘要复用或同父路径冲突（不可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：全站Store.mu短SQL/树CAS；不持SQL或mutex跨鉴权/当前平台窗口RPC；Save只改draft head字段，不覆盖本事务新parent/order
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.executeTreeMutation`、`content.decodeStorageJSON`、`content.EnsureTreeAcyclic`、`content.PlanOldPathRedirects`

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

1. 仅portal当前admin，content.edit在DB前一次即时核；同scope动作/摘要原结果优先回放
2. 核parent活动且完整draft/published子集合，拒缺失/重复/跨层；trash/purged不混入活动排序，空集合允许
3. 在现有base62字典序协议以6位均匀key重排并保留插入间隔，不改变slug；同SQL核树CAS和metadata revision CAS、站点/代次，保存完整原回执及content.tree.changed outbox
4. 当前写窗口在锁外；Commit未知不推定失败，重试查原结果不重复推进水位或事件

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：持久树、原回执或正文资格不完整（不可重试）
- `CONFLICT`：同操作动作/摘要复用或同父路径冲突（不可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.executeTreeMutation`、`content.decodeStorageJSON`

## content.NewTreeRuntime

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：预装真实树依赖与完整原结果schema校验器

```go
func NewTreeRuntime(reference ReferenceRuntime, window platform.WriteWindowConfig) (TreeRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 预装真实树依赖与完整原结果schema校验器
2. 原回执schema覆盖移动、重排、元数据、发布和撤回结果；分享图变更仍不是已完成能力

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`platform.NewPayloadValidator`

## content.WithTreeRuntime

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：装配受信树运行依赖

```go
func WithTreeRuntime(ctx context.Context, runtime TreeRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 装配受信树运行依赖

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：无

## content.treeRuntimeFromContext

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：取得完整真实树依赖

```go
func treeRuntimeFromContext(ctx context.Context) (TreeRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 取得完整真实树依赖

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`content.WithTreeRuntime`

## content.authorizeTree

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：仅portal管理主体，一次即时全站功能授权

```go
func authorizeTree(ctx context.Context, op OperationContext, runtime TreeRuntime, permission string, action string) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅portal管理主体，一次即时全站功能授权

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`platform.NewIdentityAuthorizer`

## content.beginTreeRead

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：核本域作用域后建立只读一致SQL快照

```go
func beginTreeRead(ctx context.Context, op OperationContext, runtime TreeRuntime) (*sql.Tx, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核本域作用域后建立只读一致SQL快照

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`content.checkRevisionScope`

## content.readTreeSnapshot

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：核当前身份、完整元数据/head资格及全树结构

```go
func readTreeSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, runtime TreeRuntime) (int64, []gen.DocumentMetadata, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核当前身份、完整元数据/head资格及全树结构

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`content.readCreationTree`、`content.EnsureTreeAcyclic`、`content.validateTreeSiblings`

## content.validateTreeSiblings

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：核活动父关系及同父orderKey/slug唯一

```go
func validateTreeSiblings(documents []gen.DocumentMetadata) error
```

**输入**

无

**输出**

无

**步骤**

1. 核活动父关系及同父orderKey/slug唯一

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：无

## content.treePreorder

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：在已核无环深度受限树按parent/orderKey/ID返回确定性前序

```go
func treePreorder(documents []gen.DocumentMetadata, includeInactive bool) []gen.DocumentMetadata
```

**输入**

无

**输出**

无

**步骤**

1. 在已核无环深度受限树按parent/orderKey/ID返回确定性前序

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：无

## content.readTreeReceipt

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：按完整身份动作摘要读取严格schema原结果，异材料冲突

```go
func readTreeReceipt(ctx context.Context, q creationQuery, op OperationContext, action string, digest Hash, runtime TreeRuntime) (json.RawMessage, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按完整身份动作摘要读取严格schema原结果，异材料冲突

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：无

## content.executeTreeMutation

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：原回执优先；锁外当前写窗口后短SQL提交树/metadata CAS、原结果与outbox

```go
func executeTreeMutation(ctx context.Context, op OperationContext, action string, input any, expected int) (json.RawMessage, error)
```

**输入**

无

**输出**

无

**步骤**

1. 原回执优先；锁外当前写窗口后短SQL提交树/metadata CAS、原结果与outbox
2. 原回执优先回放后，新写前必须已装配Runtime/WriteWindow两个可信回调；缺失明确503，不能调用nil回调；只读和历史回执不依赖新写窗口

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`content.treeRuntimeFromContext`、`content.authorizeTree`、`content.checkRevisionScope`、`content.draftRequestDigest`、`content.readTreeReceipt`、`content.readTreeSnapshot`、`content.planTreeMove`、`content.planTreeReorder`、`content.commitRedirectManifest`、`platform.NewWriteContext`、`platform.EnqueueEvent`

## content.planTreeMove

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：规划支持范围内移动，并返回曾发布旧路径供同一事务写入manifest

```go
func planTreeMove(documents []gen.DocumentMetadata, input MoveDocumentInput, revision int64) ([]gen.DocumentMetadata, []gen.RouteTombstone, error)
```

**输入**

无

**输出**

无

**步骤**

1. 规划支持范围内移动；父变化时返回曾发布旧路径，由executeTreeMutation写入已持久化manifest
2. 已有slug移入根级时复用同一根级navigation/ask规则，不能用Move绕过；其他层级不额外禁名

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`content.EnsureTreeAcyclic`、`content.PlanOldPathRedirects`、`content.sameIdentifierPointer`、`content.fractionalDocumentOrder`、`content.validateTreeSiblings`、`content.validatePathSnapshot`

## content.planTreeReorder

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：核完整活动同级集合并生成均匀base62排序

```go
func planTreeReorder(documents []gen.DocumentMetadata, input ReorderChildrenInput) ([]gen.DocumentMetadata, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核完整活动同级集合并生成均匀base62排序

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`content.sameIdentifierPointer`、`content.reorderedDocumentKey`

## content.reorderedDocumentKey

- 级别：参考
- 文件：`modules/content/internal/tree_service.go`
- 作用：给不超过10000节点生成6位base62均匀有间隔key

```go
func reorderedDocumentKey(index int) string
```

**输入**

无

**输出**

无

**步骤**

1. 给不超过10000节点生成6位base62均匀有间隔key

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
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

1. 仅portal当前admin一次即时content.edit，在DB/锁前，先按site/epoch/operation/caller/actor及规范请求摘要回放完整原回执
2. 锁外取当前平台写窗口；短SQL同快照核站点/epoch、完整树/head资格、活动文档、metadata expectedRevision及当前treeRevision，标题/人工SEO不触发模型或发布共享正文
3. path改变先完整规划node及曾发布后代旧路径，未知路径失败关闭，非空规划写入同一事务的版本化manifest；shareImage实际改变因精确metadata资源pin/引用源合同未齐503不提交，完整Update因此仍不完整
4. 原shareImage同一完整ref可维持；非空path走ValidatePathSegment可信同SQL树及精确根级保留名；seoDescription=null为pending，否则manual（含显式空串）
5. 同SQL CAS metadata revision与treeRevision，更新title/path/SEO并保存同代完整原结果与最小tree.changed outbox；不改变draft/published正文、引用水位或旧历史；Commit未知只按原op回执重试

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `CONFLICT`：同操作摘要冲突、路径保留/同父冲突或持久原结果/树资格损坏（不可重试）
- `INTEGRITY_ERROR`：同操作摘要冲突、路径保留/同父冲突或持久原结果/树资格损坏（不可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope
- 并发：当前权限/写窗口锁外；Store.mu短SQL内读取当前真实parent/head、metadata/tree CAS与回执/outbox，保存并发只更新draftRevisionId不覆盖新metadata
- 幂等：同site/epoch/operationId固定caller/actor和完整规范请求摘要，原结果及SHA256同事务追加并严格校验回放；Commit未知不重复更新或发事件
- 依赖：`content.treeRuntimeFromContext`、`content.authorizeTree`、`content.checkRevisionScope`、`content.draftRequestDigest`、`content.readMetadataReceipt`、`content.readTreeSnapshot`、`content.sameMetadataAsset`、`content.PlanOldPathRedirects`、`content.ValidatePathSegment`、`content.commitRedirectManifest`、`platform.NewWriteContext`、`platform.EnqueueEvent`

## content.readMetadataReceipt

- 级别：参考
- 文件：`modules/content/internal/metadata_service.go`
- 作用：同代完整scope及规范摘要回放原结果；核result_hash、严格结果schema及解码，异身份摘要冲突

```go
func readMetadataReceipt(ctx context.Context, q creationQuery, op OperationContext, digest Hash, runtime TreeRuntime) (UpdateDocumentMetadataResult, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同代完整scope及规范摘要回放原结果；核result_hash、严格结果schema及解码，异身份摘要冲突

**错误**

- 无

- 副作用：无
- 授权：已认证本域metadata调用链或可信同SQL树；业务权限由入口在SQL/锁前一次即时核
- 并发：不另查身份/当前窗口；短SQL内读取持久结果或对可信快照纯计算
- 幂等：完整scope/请求摘要及结果hash回放首次原值；纯函数无副作用
- 依赖：`content.decodeStorageJSON`

## content.sameMetadataAsset

- 级别：参考
- 文件：`modules/content/internal/metadata_service.go`
- 作用：按nullable完整不可变资源身份比较，改变必须等真实metadata pin协议

```go
func sameMetadataAsset(a *gen.AssetVersionRef, b *gen.AssetVersionRef) bool
```

**输入**

无

**输出**

无

**步骤**

1. 按nullable完整不可变资源身份比较，改变必须等真实metadata pin协议

**错误**

- 无

- 副作用：无
- 授权：已认证本域metadata调用链或可信同SQL树；业务权限由入口在SQL/锁前一次即时核
- 并发：不另查身份/当前窗口；短SQL内读取持久结果或对可信快照纯计算
- 幂等：完整scope/请求摘要及结果hash回放首次原值；纯函数无副作用
- 依赖：无

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

1. 正则只允许小写英数与中间单连字符，拒绝点段/编码斜杠；仅parentId=null时保留navigation和ask，与public-site精确语言根路由一致，其他层级允许；不额外禁止api/admin/v1/robots/llms/sitemap
2. 从调用方同一SQL可信完整树中核parent为活动节点、同父活动slug唯一并排除自己；纯校验不另开SQL或重复鉴权
3. 只返回可用性，不写历史redirect；曾发布路径由调用者在同一事务写入版本化manifest

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `LOCKED`：可信树中父节点缺失或非活动（不可重试）
- `NOT_FOUND`：可信树中父节点缺失或非活动（不可重试）

- 副作用：无
- 授权：content.edit或publish私有流程
- 并发：可信同SQL快照的纯业务校验，不另开SQL/重查身份；ctx/op只保留统一私有调用签名
- 幂等：只读
- 依赖：`content.validatePathSnapshot`

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

1. 先核完整原树无环及深度；按前序定位node及全部后代
2. 仅曾公开节点（publishedReference/lastPublishedAt/state/visibilityRevision证据）构造当前父链完整旧路径，任何祖先缺slug明确DEPENDENCY_UNAVAILABLE，绝不当空
3. 不含语言前缀，target为稳定docId而非链式URL；旧路径唯一且不超过2048字节，revision由调用方锁定treeRevision+1提供
4. 只规划不持久化；持久化由commitRedirectManifest在调用者的同一SQL事务完成
5. 输入是本域可信完整树，无内部输入总量硬限制；公开输出/批次上限不改，不能借扫描预算限制Create站点总量

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `NOT_FOUND`：节点缺失或曾发布路径权威未知（不可重试）
- `DEPENDENCY_UNAVAILABLE`：节点缺失或曾发布路径权威未知（可重试）

- 副作用：无
- 授权：纯函数
- 并发：纯函数，输入视为不可变；可以并发调用
- 幂等：确定性输入产生确定性输出；无持久副作用
- 依赖：`content.EnsureTreeAcyclic`、`content.treePreorder`

## content.validatePathSnapshot

- 级别：参考
- 文件：`modules/content/internal/path_service.go`
- 作用：校slug正则、仅根级navigation/ask保留、活动父及同父唯一，排除自己

```go
func validatePathSnapshot(input ValidatePathSegmentInput) (ValidatePathSegmentResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 校slug正则、仅根级navigation/ask保留、活动父及同父唯一，排除自己

**错误**

- 无

- 副作用：无
- 授权：已认证本域metadata调用链或可信同SQL树；业务权限由入口在SQL/锁前一次即时核
- 并发：不另查身份/当前窗口；短SQL内读取持久结果或对可信快照纯计算
- 幂等：完整scope/请求摘要及结果hash回放首次原值；纯函数无副作用
- 依赖：`content.sameIdentifierPointer`

## content.commitRedirectManifest

- 级别：参考
- 文件：`modules/content/internal/path_service.go`
- 作用：在调用者事务里追加一版完整重定向清单

```go
func commitRedirectManifest(ctx context.Context, tx *sql.Tx, op OperationContext, redirects []gen.RouteTombstone, documents []gen.DocumentMetadata) ([]gen.RouteTombstone, error)
```

**输入**

无

**输出**

无

**步骤**

1. 空规划不写。同一旧路径不能改绑其他文档。旧路径若已成为该文档规范路径则移除；若属于其他文档则冲突
2. 新版本复制保留项并CAS清单水位。缺表失败关闭

**错误**

- 无

- 副作用：redirect_manifest_heads、redirect_manifest_entries
- 授权：已认证的树或发布事务
- 并发：只写调用者已经打开的短事务
- 幂等：由调用者的原回执保证，不单独重放
- 依赖：`content.canonicalDocumentPaths`

## content.canonicalDocumentPaths

- 级别：参考
- 文件：`modules/content/internal/path_service.go`
- 作用：从可信树计算活动文档的当前规范路径

```go
func canonicalDocumentPaths(documents []gen.DocumentMetadata) (map[string]Identifier, error)
```

**输入**

无

**输出**

无

**步骤**

1. 缺slug的活动文档不进入路径表。父缺失或超深失败关闭

**错误**

- 无

- 副作用：无
- 授权：调用者已持有可信树快照
- 并发：纯计算
- 幂等：确定性
- 依赖：无

## content.NewEditLockManager

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：真实control区NOFOLLOW/SameFile与flock单leader，持有至Close不unlink

```go
func NewEditLockManager(store *Store) (*EditLockManager,error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实control区NOFOLLOW/SameFile与flock单leader，持有至Close不unlink

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：`content.validateEditLeader`、`platform.ValidatePrivatePath`、`content.syncDirectory`
- 其他依赖说明：锁核心3种业务Input在本域按types.json投影；HTTP生成request DTO拆分路径字段，未来handler经严格边界显式映射，不依赖被裁剪的gen业务Input

## content.validateEditLeader

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：复用私有路径及打开前后SameFile拒绝leader路径替换

```go
func validateEditLeader(file *os.File) error
```

**输入**

无

**输出**

无

**步骤**

1. 复用私有路径及打开前后SameFile拒绝leader路径替换

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：无

## content.lockEditCell

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：先核真实文档存在才分配site/doc cell，拒绝不存在ID无限积累；取得doc锁后用例仍重核权威

```go
func (manager *EditLockManager) lockEditCell(ctx context.Context, op OperationContext, documentID Identifier) (*editLeaseCell, error)
```

**输入**

无

**输出**

无

**步骤**

1. 先核真实文档存在才分配site/doc cell，拒绝不存在ID无限积累；取得doc锁后用例仍重核权威

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：`content.ReadDocumentEditState`

## content.AcquireLease

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：真实文档/删除/隔离检查后分配或复用账户共享租约，其他账户只读，不因开tab重置活动

```go
func (manager *EditLockManager) AcquireLease(ctx context.Context,op OperationContext,input AcquireLeaseInput) (EditLockSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 经完整用例即时授权后，委托统一可靠编辑操作编排，不直接暴露裸内存变更

**错误**

- 无

- 副作用：无
- 授权：仅已即时授权完整本域用例，不能直接绑定HTTP；ExpireLeases仅本域后台
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：统一原scope/规范摘要回执，提交后才安装候选，未知提交pending阻止保存
- 依赖：`content.executeEditLock`

## content.RenewLease

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：严格前进心跳才续2min，严格前进输入才以服务器接收时间更新活动；旧心跳不续期

```go
func (manager *EditLockManager) RenewLease(ctx context.Context,op OperationContext,input RenewEditLockInput) (EditLockSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 经完整用例即时授权后，委托统一可靠编辑操作编排，不直接暴露裸内存变更

**错误**

- 无

- 副作用：无
- 授权：仅已即时授权完整本域用例，不能直接绑定HTTP；ExpireLeases仅本域后台
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：统一原scope/规范摘要回执，提交后才安装候选，未知提交pending阻止保存
- 依赖：`content.executeEditLock`

## content.ReleaseLease

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：只释放匹配holder/lease/boot的当前内存锁并提高fence；完整通知编排另行实施

```go
func (manager *EditLockManager) ReleaseLease(ctx context.Context,op OperationContext,input ReleaseEditLockInput) (EditLockSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 经完整用例即时授权后，委托统一可靠编辑操作编排，不直接暴露裸内存变更

**错误**

- 无

- 副作用：无
- 授权：仅已即时授权完整本域用例，不能直接绑定HTTP；ExpireLeases仅本域后台
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：统一原scope/规范摘要回执，提交后才安装候选，未知提交pending阻止保存
- 依赖：`content.executeEditLock`

## content.ForceReleaseLease

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：已由超管编排授权后的expectedLease匹配内存强释，不实现未交付通知/审计/原回执

```go
func (manager *EditLockManager) ForceReleaseLease(ctx context.Context,op OperationContext,input ForceReleaseEditLockInput) (EditLockSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 经完整用例即时授权后，委托统一可靠编辑操作编排，不直接暴露裸内存变更

**错误**

- 无

- 副作用：无
- 授权：仅已即时授权完整本域用例，不能直接绑定HTTP；ExpireLeases仅本域后台
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：统一原scope/规范摘要回执，提交后才安装候选，未知提交pending阻止保存
- 依赖：`content.executeEditLock`

## content.ExpireLeases

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：本域后台按真实服务时钟逐doc过期或提醒并返回实际变化，未声明通知已经送达

```go
func (manager *EditLockManager) ExpireLeases(ctx context.Context, op OperationContext, now time.Time) ([]EditLockSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 经完整用例即时授权后，委托统一可靠编辑操作编排，不直接暴露裸内存变更

**错误**

- 无

- 副作用：无
- 授权：仅已即时授权完整本域用例，不能直接绑定HTTP；ExpireLeases仅本域后台
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：统一原scope/规范摘要回执，提交后才安装候选，未知提交pending阻止保存
- 依赖：`content.expireEditLocks`

## content.AcquireSaveFence

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：持同doc mutex及生命周期读锁检查真实doc/deleteFence/隔离与当前holder/lease/boot/fence直到Release

```go
func (manager *EditLockManager) AcquireSaveFence(ctx context.Context,op OperationContext,input DraftFenceInput) (*SaveFenceGuard,error)
```

**输入**

无

**输出**

无

**步骤**

1. 持同doc mutex及生命周期读锁检查真实doc/deleteFence/隔离与当前holder/lease/boot/fence直到Release

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：`content.AcquireDocumentFence`、`content.requireEditableDocument`、`content.matchEditLease`、`content.copyEditLease`、`content.ReleaseFence`

## content.AcquireDocumentFence

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：为已授权生命周期短事务持同doc mutex与manager生命周期读锁，并读取真实持久文档状态

```go
func (manager *EditLockManager) AcquireDocumentFence(ctx context.Context,op OperationContext,documentID Identifier) (*SaveFenceGuard,error)
```

**输入**

无

**输出**

无

**步骤**

1. 同doc mutex内先按真实回执解决未知提交pending，不确定则失败关闭，不允许保存跨过未确认变更
2. 为已授权生命周期短事务持同doc mutex与manager生命周期读锁，并读取真实持久文档状态

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：`content.lockEditCell`、`content.ReadDocumentEditState`、`content.resolvePendingEditLock`

## content.DocumentState

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：返回guard取得的持久文档快照，不能替代提交事务内CAS

```go
func (guard *SaveFenceGuard) DocumentState() DocumentEditState
```

**输入**

无

**输出**

无

**步骤**

1. 返回guard取得的持久文档快照，不能替代提交事务内CAS

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：无

## content.ReleaseFence

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：先释放doc mutex再释放manager生命读锁，重复释放安全

```go
func (guard *SaveFenceGuard) ReleaseFence()
```

**输入**

无

**输出**

无

**步骤**

1. 先释放doc mutex再释放manager生命读锁，重复释放安全

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：无

## content.matchEditLease

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：核当前holder/lease/boot/fence及真实服务时间未过期，旧资格不可复活

```go
func (manager *EditLockManager) matchEditLease(cell *editLeaseCell,op OperationContext,input DraftFenceInput,now time.Time) error
```

**输入**

无

**输出**

无

**步骤**

1. 核当前holder/lease/boot/fence及真实服务时间未过期，旧资格不可复活

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：`content.editLeaseExpired`

## content.requireEditableDocument

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：持久隔离、trash/purged阻止编辑，不根据请求猜文档

```go
func requireEditableDocument(state DocumentEditState) error
```

**输入**

无

**输出**

无

**步骤**

1. 持久隔离、trash/purged阻止编辑，不根据请求猜文档

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：无

## content.advanceEditFence

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：提高boot内单调fence，达到安全整数上限则关闭该cell不溢出

```go
func advanceEditFence(cell *editLeaseCell) error
```

**输入**

无

**输出**

无

**步骤**

1. 提高boot内单调fence，达到安全整数上限则关闭该cell不溢出

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：无

## content.editLeaseExpired

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：判断2min心跳或LastInputAt+17min绝对无输入期限，即使没及时检测提醒也不能复活

```go
func editLeaseExpired(lease *gen.EditLease,now time.Time) bool
```

**输入**

无

**输出**

无

**步骤**

1. 判断2min心跳或LastInputAt+17min绝对无输入期限，即使没及时检测提醒也不能复活

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：无

## content.refreshEditLease

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：空闲提醒界点固定LastInputAt+15min，17min绝对失效；检测延迟不能延后宽限

```go
func refreshEditLease(cell *editLeaseCell,now time.Time) (bool,string,error)
```

**输入**

无

**输出**

无

**步骤**

1. 空闲提醒界点固定LastInputAt+15min，17min绝对失效；检测延迟不能延后宽限

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：`content.editLeaseExpired`、`content.advanceEditFence`

## content.copyEditLease

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：深拷贝共享租约及nullable提醒时间，禁止调用方改写权威

```go
func copyEditLease(lease *gen.EditLease) *gen.EditLease
```

**输入**

无

**输出**

无

**步骤**

1. 深拷贝共享租约及nullable提醒时间，禁止调用方改写权威

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：无

## content.editLockSnapshot

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：返回真实内存状态供完整用例可靠通知/审计编排，不当作已提交HTTP回执

```go
func (manager *EditLockManager) editLockSnapshot(op OperationContext,documentID Identifier,cell *editLeaseCell,changed bool,cause string) EditLockSnapshot
```

**输入**

无

**输出**

无

**步骤**

1. 返回真实内存状态供完整用例可靠通知/审计编排，不当作已提交HTTP回执

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
- 依赖：`content.copyEditLease`

## content.CloseEditLocks

- 级别：参考
- 文件：`modules/content/internal/edit_lock_service.go`
- 作用：life写锁先排空全部操作和guard，再释放flock及句柄，关闭后拒绝旧manager

```go
func (manager *EditLockManager) CloseEditLocks() error
```

**输入**

无

**输出**

无

**步骤**

1. life写锁先排空全部操作和guard，再释放flock及句柄，关闭后拒绝旧manager

**错误**

- 无

- 副作用：无
- 授权：受信已即时授权本域用例；姓名必须identity权威，核心不冒充HTTP与通知/审计成功
- 并发：life读锁→doc mutex→短SQL读取；guard持锁到Release，Close取life写锁排空后释放leader，不跨HTTP/文件IO
- 幂等：共享账户lease/boot与单调fence/序号；完整原回执由统一可靠用例编排负责
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
- `CONFLICT`：同操作异动作或摘要，或意图已终止（不可重试）

- 副作用：无
- 授权：content.read
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：只读
- 依赖：`content.draftRuntimeFromContext`、`content.authorizeEditLock`、`content.readDraftSnapshot`、`content.ConfigureRevisionStorage`、`content.ReadRevisionBody`、`content.WithRevisionFileStore`、`content.AcquireDocumentFence`、`content.ReleaseFence`、`content.copyEditLease`、`content.editLeaseExpired`

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

1. 即时content.edit授权，短doc guard核真实holder/boot/fence/lease和原head/version，释放guard后校正文与稳定ID
2. 初读或prepareDraftOperation并发回放后的当前record.State统一分派：COMMITTED回原结果；aborted/rejected隔离并Release原pin后CONFLICT；只有pending继续，不能因初读found=false给已终止源新建文件准备
3. 首次新操作先读取当前站点maxDocumentBytes，规范UTF8长度超额立即VALIDATION，不写intent不申请pin；原COMMITTED回执先回放，已prepared材料仍由文件层先回放，不因降低配额拒绝原成功
4. found pending继续Acquire或文件IO前，短真实SaveFenceGuard核原lease/head/version/独立引用CAS并释放；确定拒绝走统一无COMMITTED证明和nullable原pin封存，未知及完整性错误保留保护
5. 固定来源operation/site/object/revision/version/完整排序引用digest意图先持久，再media Acquire真实pin；网络未知保留pending且不释放
6. WriteRevisionFile不可变fsync后才建立当前窗口与真实SaveFenceGuard；短SQL重核deleteFence/隔离/head CAS并同事务写ledger、COMMITTED、refs、完整原回执和outbox
7. COMMITTED后confirm原pin；失败重试同op先读原结果再补confirm，不重新推进head；COMMITTED/未知Commit/坏正文均不转abort
8. 只有明确CAS/租约/不存在拒绝且同SQL确认无COMMITTED资格才持久aborted、防迟到提交，随后隔离未提交文件并携完整source材料及nullable原pin释放；未知依赖保留保护

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：会话、委托或服务证明缺失、失效或被撤销（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `INTEGRITY_ERROR`：私有固定意图、正文资格、稳定ID或完整原回执不一致（不可重试）
- `CONFLICT`：同操作异动作或摘要，或意图已终止（不可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：content.edit且当前有效锁
- 并发：save登记与锁强释同docmutex；持久head CAS避免同人双tab覆盖；不能持mutex跨无限网络
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.draftRuntimeFromContext`、`content.authorizeEditLock`、`content.CanonicalizeBody`、`content.draftRequestDigest`、`content.readDraftOperation`、`content.planDraftSave`、`content.draftWriteContext`、`content.prepareDraftOperation`、`content.acquireDraftPin`、`content.readDraftSiteConfig`、`content.executeDraftCommit`、`content.abortDraftOperation`、`content.confirmDraftPin`、`content.releaseDraftPin`、`content.replaySavedDraft`、`content.isolateAbortedDraft`、`media:acquireResourceReferences`、`media:confirmResourceReferences`、`media:releaseResourceReferences`、`content.checkPendingDraftSave`、`content.finishDraftSaveFailure`

## content.readDraftSnapshot

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：读取元数据共享head与COMMITTED回执一致快照

```go
func (s *Store) readDraftSnapshot(ctx context.Context,op OperationContext,documentID Identifier) (gen.DocumentMetadata,DraftRevision,error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取元数据共享head与COMMITTED回执一致快照

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.checkRevisionScope`、`content.decodeStorageJSON`、`content.sameStoredDraft`

## content.planDraftSave

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：短guard核真实租约和head后抽取稳定ID及精确引用材料

```go
func planDraftSave(ctx context.Context,op OperationContext,runtime DraftRuntime,input SaveDraftInput,bodyHash Hash) (draftOperationPlan,error)
```

**输入**

无

**输出**

无

**步骤**

1. 短guard核真实租约和head后抽取稳定ID及精确引用材料
2. 引用水位独立于draftVersion：首次意图冻结ExpectedReferenceVersion及SourceVersion，短SQL与head/ledger/refs同事务CAS推进documents.reference_version；公共Revision耗尽失败，旧/未知行不猜补资格

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.ReadDocumentEditState`、`content.AcquireSaveFence`、`content.readDraftSnapshot`、`content.ReleaseFence`、`content.ValidateBlockTree`、`content.CollectResourceReferences`、`content.draftMediaReferences`

## content.executeDraftCommit

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：复用持久修订ID准备正文文件后调用真实短SQL提交

```go
func executeDraftCommit(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation,body DocumentBody,limit int64) (DraftRevision,error)
```

**输入**

无

**输出**

无

**步骤**

1. 复用持久修订ID准备正文文件后调用真实短SQL提交

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.ConfigureRevisionStorage`、`content.WithRevisionFileStore`、`content.WriteRevisionFile`、`content.WithDraftCommitStore`、`content.CommitDraftJournal`

## content.draftMediaReferences

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：投影并排序媒体预提交引用，计算完整规范集合摘要

```go
func draftMediaReferences(refs []gen.ResourceReference) ([]draftMediaReference,Hash,error)
```

**输入**

无

**输出**

无

**步骤**

1. 投影并排序媒体预提交引用，计算完整规范集合摘要

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.draftPinSource

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：从首次持久意图构造完整媒体来源材料

```go
func draftPinSource(op OperationContext,record draftOperation) draftPinMaterial
```

**输入**

无

**输出**

无

**步骤**

1. 从首次持久意图构造完整媒体来源材料
2. sourceVersion取首次持久SourceVersion，原op重试不改来源水位或重新生成pin身份

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.acquireDraftPin

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：意图已持久后调用真实媒体Acquire并固定完整资产pin回执

```go
func acquireDraftPin(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation) (draftOperation,error)
```

**输入**

无

**输出**

无

**步骤**

1. 意图已持久后调用真实媒体Acquire并固定完整资产pin回执

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.draftPinSource`、`content.contentServiceContext`、`content.draftWriteContext`、`content.readDraftOperation`

## content.confirmDraftPin

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：COMMITTED后请求media确认原pin，不释放未知或历史资源

```go
func confirmDraftPin(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation) error
```

**输入**

无

**输出**

无

**步骤**

1. COMMITTED后请求media确认原pin，不释放未知或历史资源

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.contentServiceContext`

## content.replaySavedDraft

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：回放完整原保存回执并重试尚未确认的原pin

```go
func replaySavedDraft(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation) (SaveDraftResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 回放完整原保存回执并重试尚未确认的原pin

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.decodeStorageJSON`、`content.confirmDraftPin`

## content.abortDraftOperation

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：只在真实确认没有COMMITTED资格时原子封存已知失败源

```go
func (s *Store) abortDraftOperation(ctx context.Context,op OperationContext,record draftOperation) (draftOperation,error)
```

**输入**

无

**输出**

无

**步骤**

1. 用当前短SQL快照重新读取同scope完整真实原记录，刷新固定计划、state、pin_id和完整原结果，所有终态分支均使用当前持久pin
2. 若COMMITTED在同一事务核真实资格与完整原回执，明确结束事务后才交给调用方replay/confirm，不带较早pending快照的nil pin
3. 仅确认没有COMMITTED资格时原子封存确定失败源；SQL提交未知不释放资源

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.readDraftOperationFrom`、`content.contentStorageError`

## content.releaseDraftPin

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：仅已持久aborted或rejected的完整源材料调用media封存

```go
func releaseDraftPin(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅已持久aborted或rejected的完整源材料调用media封存

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.draftPinSource`、`content.contentServiceContext`

## content.isolateAbortedDraft

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：源已终止且禁止迟到COMMITTED后隔离该次未提交正文文件

```go
func isolateAbortedDraft(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation) error
```

**输入**

无

**输出**

无

**步骤**

1. 源已终止且禁止迟到COMMITTED后隔离该次未提交正文文件

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.decodeStorageJSON`、`content.revisionBodyPath`、`content.moveBodyToQuarantine`、`content.appendJournalState`

## content.checkPendingDraftSave

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：在pending续作的Acquire和文件IO前短核真实租约及原CAS

```go
func checkPendingDraftSave(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation) error
```

**输入**

无

**输出**

无

**步骤**

1. 按原意图Fence取得真实短guard，核本次共享head/draftVersion及ExpectedReferenceVersion快照
2. 返回前释放guard；期间无RPC或正文文件IO，不生成新修订ID、水位或pin

**错误**

- 无

- 副作用：无
- 授权：只在已认证content.edit保存调用链，信任已核持久意图；无COMMITTED证明前不释放资源
- 并发：check只持真实guard跨短SQL快照；统一失败收敛先结束guard，再读取写窗口及封存SQL，全部RPC/文件IO无guard
- 幂等：同操作复用原source材料与nullable原pin；COMMITTED优先，未知和完整性错误保留保护
- 依赖：`content.AcquireSaveFence`、`content.ReleaseFence`、`content.readDraftSnapshot`

## content.finishDraftSaveFailure

- 级别：参考
- 文件：`modules/content/internal/draft_service.go`
- 作用：统一收敛确定保存拒绝，证明无COMMITTED后封存源并释放原pin

```go
func finishDraftSaveFailure(ctx context.Context,op OperationContext,runtime DraftRuntime,record draftOperation,cause error) (SaveDraftResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅明确VALIDATION/租约/版本CAS/不存在拒绝可继续；Commit/IO未知及完整性损坏直接保留原意图和pin
2. 读取当前公共窗口后用真实SQL证明没有COMMITTED资格并封存原source；若竞争已提交则回原结果并补confirm
3. 源封存后隔离本次未提交正文并携原source材料及nullable pin Release，不重新Acquire

**错误**

- 无

- 副作用：无
- 授权：只在已认证content.edit保存调用链，信任已核持久意图；无COMMITTED证明前不释放资源
- 并发：check只持真实guard跨短SQL快照；统一失败收敛先结束guard，再读取写窗口及封存SQL，全部RPC/文件IO无guard
- 幂等：同操作复用原source材料与nullable原pin；COMMITTED优先，未知和完整性错误保留保护
- 依赖：`content.draftWriteContext`、`content.abortDraftOperation`、`content.replaySavedDraft`、`content.isolateAbortedDraft`、`content.releaseDraftPin`

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
- 依赖：`content.newRevisionWriteWindow`、`content.verifyRevisionMaterialFile`、`content.Read`、`platform.ValidatePrivatePath`、`content.appendJournalState`、`content.syncDirectory`

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
- 依赖：`platform.ValidatePrivatePath`、`content.verifyPreparedRevisionFile`

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
- 依赖：`content.openRevisionJournal`、`content.validateRevisionJournalFile`、`content.syncDirectory`

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

## content.sameJournalIdentity

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：比较持久日志固定身份和正文hash材料

```go
func sameJournalIdentity(left, right CommitJournal) bool
```

**输入**

无

**输出**

无

**步骤**

1. 比较持久日志固定身份和正文hash材料

**错误**

- 无

- 副作用：无
- 授权：仅本域受信私有正文调用链；不可从浏览器传入任意磁盘路径
- 并发：不持SQL事务或doc guard跨正文文件IO
- 幂等：相同固定材料不改变已提交身份
- 依赖：无

## content.moveBodyToQuarantine

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：把已失败关闭的修订文件持久移入本域隔离区

```go
func moveBodyToQuarantine(ctx context.Context, root, siteID string, journal CommitJournal, bodyPath string) error
```

**输入**

无

**输出**

无

**步骤**

1. 把已失败关闭的修订文件持久移入本域隔离区

**错误**

- 无

- 副作用：无
- 授权：仅本域受信私有正文调用链；不可从浏览器传入任意磁盘路径
- 并发：不持SQL事务或doc guard跨正文文件IO
- 幂等：相同固定材料不改变已提交身份
- 依赖：`content.syncDirectory`

## content.syncDirectory

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：持久化私有目录项

```go
func syncDirectory(path string) error
```

**输入**

无

**输出**

无

**步骤**

1. 持久化私有目录项

**错误**

- 无

- 副作用：无
- 授权：仅本域受信私有正文调用链；不可从浏览器传入任意磁盘路径
- 并发：不持SQL事务或doc guard跨正文文件IO
- 幂等：相同固定材料不改变已提交身份
- 依赖：无

## content.revisionBodyPath

- 级别：参考
- 文件：`modules/content/internal/revision_files.go`
- 作用：定位同站点文档不可变正文路径

```go
func revisionBodyPath(root, siteID, documentID, revisionID string) string
```

**输入**

无

**输出**

无

**步骤**

1. 定位同站点文档不可变正文路径

**错误**

- 无

- 副作用：无
- 授权：仅本域受信私有正文调用链；不可从浏览器传入任意磁盘路径
- 并发：不持SQL事务或doc guard跨正文文件IO
- 幂等：相同固定材料不改变已提交身份
- 依赖：无

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

1. 先读同scope摘要COMMITTED原回执，命中后释放真实事务对象并回原结果
2. 文件hash/fsync和私有路径校验在SQL/doc guard之前完成
3. 延迟对象在当前写窗口及真实保存guard内，短SQL原子CAS head/version、ledger、COMMITTED资格、refs、完整用例回执和outbox
4. 文件准备或未知提交不代表可见；COMMITTED不转abort，坏正文或未知状态保留资源保护

**错误**

- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：私有SaveDraft/Create事务
- 并发：doc rowCAS + fence检查
- 幂等：operationId+canonicaldigest去重
- 依赖：`content.validateDraftCommitInput`、`content.draftCommitStoreFromContext`、`content.validateDraftCommitStore`、`content.draftCommitDigest`、`content.revisionBodyPath`、`content.BeginDraftCommit`、`content.FindReceipt`、`content.Rollback`、`content.verifyRevisionMaterialFile`、`content.ValidateFence`、`content.Commit`、`content.normalizeDraftCommitError`

## content.WithDraftCommitStore

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：装配受信真实草稿提交存储

```go
func WithDraftCommitStore(ctx context.Context, store DraftCommitStore) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 装配受信真实草稿提交存储

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.draftCommitStoreFromContext

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：取得本域真实草稿提交适配

```go
func draftCommitStoreFromContext(ctx context.Context) (DraftCommitStore, bool)
```

**输入**

无

**输出**

无

**步骤**

1. 取得本域真实草稿提交适配

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.validateDraftCommitInput

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：核草稿与PREPARED日志、前序head和对象材料一致

```go
func validateDraftCommitInput(op OperationContext, input CommitDraftJournalInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 核草稿与PREPARED日志、前序head和对象材料一致

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.sameIdentifierPointer`

## content.validateDraftCommitStore

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：核私有根和本站提交作用域

```go
func validateDraftCommitStore(store DraftCommitStore, op OperationContext) (string, Identifier, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核私有根和本站提交作用域

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.RevisionSiteID`、`content.RevisionRoot`

## content.verifyPreparedRevisionFile

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：核不可变PREPARED正文大小哈希和文件目录持久性

```go
func verifyPreparedRevisionFile(ctx context.Context, path string, draft DraftRevision) error
```

**输入**

无

**输出**

无

**步骤**

1. 核不可变PREPARED正文大小哈希和文件目录持久性

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.checkDraftCommitIOContext`、`content.syncDirectory`

## content.checkDraftCommitIOContext

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：把可取消正文IO的终止映射为公共错误

```go
func checkDraftCommitIOContext(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 把可取消正文IO的终止映射为公共错误

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.draftCommitDigest

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：以规范JSON计算完整草稿提交摘要

```go
func draftCommitDigest(input CommitDraftJournalInput) (Hash, error)
```

**输入**

无

**输出**

无

**步骤**

1. 以规范JSON计算完整草稿提交摘要

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.sameIdentifierPointer

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：精确比较可空稳定标识

```go
func sameIdentifierPointer(left, right *Identifier) bool
```

**输入**

无

**输出**

无

**步骤**

1. 精确比较可空稳定标识

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.normalizeDraftCommitError

- 级别：参考
- 文件：`modules/content/internal/revision_repository.go`
- 作用：保留公共错误并隐藏私有存储失败细节

```go
func normalizeDraftCommitError(err error, fallback string) error
```

**输入**

无

**输出**

无

**步骤**

1. 保留公共错误并隐藏私有存储失败细节

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
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

1. 人工slug/SEO优先且不被模型覆盖；已有合法英文slug保持
2. 本轮不调用模型，不制造slug或SEO。首次空slug只返回publication.slug_missing，由发布入口提示人工补填
3. 已有manual/generated SEO保持；否则返回publication.seo_missing。缺模型不阻塞已经提供合法元数据的基础发布。路径唯一由发布事务的可信树校验

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `CONFLICT`：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：可选真实模型调用未来契约；本轮禁止执行
- 授权：content.publish
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：父operationId子任务幂等；manual字段不覆写
- 依赖：`content.InvokeModel`

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

1. 授权publish与当前写窗口在锁外完成；先读COMMITTED正文哈希，再读取就绪引用
2. 引用集非空时，用来源身份必须是同一document、revision、sourceVersion和referenceDigest上已经committed的草稿保存操作，先acquire再confirm；跨服务调用不持有本地SQL锁。找不到该决定或摘要不一致则整次失败。引用集为空则不造pin。pin失败则整次发布失败，不先公开
3. PreparePublicationMetadata后，空slug返回校验错误。确认pin之后，CommitPublication才把metadata/tree CAS、不可变快照、历史、引用和outbox放进同一事务
4. 原回执与上述写入同事务。未知Commit只回放原结果，不重复推进水位或事件。分享图非空因metadata pin合同未齐503。提交后的渲染、索引、翻译和git镜像只排队，不回退正文

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
- 依赖：`content.authorizePublication`、`content.readPublicationReceipt`、`content.ReadRevisionBody`、`content.PreparePublicationMetadata`、`content.preparePublicationPin`、`content.pinPublicationReferences`、`content.CommitPublication`、`content.bindPublicationTx`、`content.insertPublicationReceipt`、`media:acquireResourceReferences`、`media:confirmResourceReferences`

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

1. publish授权与当前写窗口在锁外完成；同操作同摘要回放原回执
2. 事务清除publishedReference、state改回draft、提高visibilityRevision，保留共享draft、lastPublishedAt和历史，并把当前发布引用改为history
3. 同一事务登记withdrawn、资源引用和tree.changed。未知Commit只回放原回执，不重复推进水位

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
- 依赖：`content.authorizePublication`、`content.readPublicationReceipt`、`content.bindPublicationTx`、`content.archivePublishedReferences`、`content.listPublicationReferences`、`content.casPublicationIdentity`、`content.enqueuePublication`、`content.insertPublicationReceipt`

## content.authorizePublication

- 级别：参考
- 文件：`modules/content/internal/publication_service.go`
- 作用：在写锁前一次核content.publish，并取同一响应中的账户显示名

```go
func authorizePublication(ctx context.Context, op OperationContext, runtime TreeRuntime, action string, documentID Identifier) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 仅portal当前admin。resource为空的全站权限。账户名取identity同一响应，拒绝空值和控制字符

**错误**

- 无

- 副作用：无
- 授权：content.publish
- 并发：鉴权RPC在Store.mu和SQL事务之前
- 幂等：只读授权，不写回执
- 依赖：无

## content.readPublicationReceipt

- 级别：参考
- 文件：`modules/content/internal/publication_service.go`
- 作用：按站点、代次、操作、调用者和摘要回放发布原结果

```go
func readPublicationReceipt(ctx context.Context, q creationQuery, op OperationContext, action string, digest Hash, runtime TreeRuntime) (json.RawMessage, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同键同摘要返回原JSON并核对结果哈希；同键异身份、动作或摘要冲突。发布和撤回再核唯一结果schema

**错误**

- 无

- 副作用：无
- 授权：已认证发布调用链
- 并发：可读取数据库或当前短事务，不跨远程IO
- 幂等：回放首次原结果
- 依赖：`content.decodeStorageJSON`

## content.insertPublicationReceipt

- 级别：参考
- 文件：`modules/content/internal/publication_service.go`
- 作用：把发布、撤回或独立提交的原结果写入当前事务

```go
func insertPublicationReceipt(ctx context.Context, tx *sql.Tx, op OperationContext, action string, digest Hash, result any) error
```

**输入**

无

**输出**

无

**步骤**

1. 与业务CAS和outbox同一事务追加原结果及哈希

**错误**

- 无

- 副作用：publication_operation_receipts
- 授权：已认证发布调用链
- 并发：只写调用者已经打开的短事务
- 幂等：同一操作主键只追加一次
- 依赖：无

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

1. rollback直接失败关闭，不写历史回滚。独立提交在本地锁之外先完成与Publish相同的资源pin，来源身份同样是已提交草稿保存操作；已绑定事务只接受锁外已经确认的同一引用摘要
2. 非空引用必须匹配已确认pin后才写入发布快照。空引用不造pin。pin失败或摘要变化则整次失败，不先公开
3. 同一事务CAS元数据、树和文档，写入不可变快照、署名历史，并把上一份当前发布引用改为history
4. 同一事务登记publication.changed、resources.changed和tree.changed。未知提交只回放原回执。非空分享图失败关闭

**错误**

- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：私有publish/rollbacktxn
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT
- 依赖：`content.applyPublication`、`content.authorizePublication`、`content.readPublicationReceipt`、`content.preparePublicationPin`、`content.pinPublicationReferences`、`content.bindPublicationTx`、`content.insertPublicationReceipt`、`content.ReadRevisionBody`、`media:acquireResourceReferences`、`media:confirmResourceReferences`

## content.applyPublication

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：在调用者事务内提交当前共享草稿的发布快照

```go
func applyPublication(ctx context.Context, tx *sql.Tx, op OperationContext, input CommitPublicationInput) (PublishDocumentResult, error)
```

**输入**

无

**输出**

无

**步骤**

1. 核对当前head、正文哈希、元数据CAS和英文路径。非空current_draft必须匹配锁外已确认pin后才复制为current_published
2. 曾发布路径变化时写入版本化manifest。事件与快照同一事务

**错误**

- 无

- 副作用：发布快照、历史、引用、manifest和outbox
- 授权：调用者已完成content.publish
- 并发：不另取写锁，不在事务内读正文文件
- 幂等：由调用者的原回执保证，本函数不重复提交
- 依赖：`content.loadSharedDraft`、`content.casPublicationIdentity`、`content.copyPublishedReferences`、`content.commitRedirectManifest`、`content.listPublicationReferences`、`content.enqueuePublication`、`content.enqueueTreeChanged`、`content.ValidatePathSegment`、`content.PlanOldPathRedirects`

## content.loadSharedDraft

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：在当前事务读取共享head对应的COMMITTED草稿和引用水位

```go
func loadSharedDraft(ctx context.Context, tx *sql.Tx, op OperationContext, documentID Identifier) (DraftRevision, int64, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只接受committed回执。缺失或损坏失败关闭

**错误**

- 无

- 副作用：无
- 授权：已认证发布事务
- 并发：只读当前短事务
- 幂等：只读
- 依赖：`content.decodeStorageJSON`

## content.casPublicationIdentity

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：同一事务绑定站点并CAS树修订、文档状态和元数据JSON

```go
func casPublicationIdentity(ctx context.Context, tx *sql.Tx, op OperationContext, treeRevision int64, metadata gen.DocumentMetadata, expectedRevision int, previousState string) error
```

**输入**

无

**输出**

无

**步骤**

1. 站点、树修订和文档修订都必须恰好更新一行

**错误**

- 无

- 副作用：documents、document_metadata、content_tree_state
- 授权：已认证发布事务
- 并发：只写当前短事务
- 幂等：CAS失败不部分保留
- 依赖：无

## content.preparePublicationPin

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：在本地SQL锁之外准备发布引用pin

```go
func preparePublicationPin(ctx context.Context, op OperationContext, runtime TreeRuntime, documentID, revisionID Identifier, expectedRevision int, manualPath, manualSEO *string) (Hash, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 短读元数据和就绪引用后立即释放事务。分享图非空或缺少英文路径时直接失败，不调用媒体
2. 引用集为空则不造pin。非空则交给pinPublicationReferences

**错误**

- 无

- 副作用：无
- 授权：已认证发布调用链
- 并发：跨服务调用前不持有Store.mu或SQL事务
- 幂等：只决定本次是否已经可以进入发布事务
- 依赖：`content.pinPublicationReferences`、`content.PreparePublicationMetadata`

## content.pinPublicationReferences

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：用现有媒体客户端先预留再确认发布快照的来源引用

```go
func pinPublicationReferences(ctx context.Context, op OperationContext, runtime TreeRuntime, documentID, revisionID Identifier, sourceVersion int, refs []gen.ResourceReference) (Hash, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 空引用直接返回，不发送空pin。非空时先读取revision_preparations与action为save且已committed的draft_operation_intents，核对document、revision、sourceVersion和referenceDigest
2. 找不到这条已提交草稿决定，或摘要与当前引用不一致，则返回错误且不调用媒体。一致时用该sourceOperationId按草稿提交同一请求形状先acquire再confirm；已确认pin由媒体按原绑定幂等回放
3. 任一调用失败则返回错误，调用方不得提交公开事务

**错误**

- 无

- 副作用：媒体资源pin
- 授权：已认证发布调用链，以本域服务身份调用媒体
- 并发：不持有本地SQL锁
- 幂等：已确认pin按草稿保存操作的原sourceOperationId幂等回放；发布操作号不是引用源，失败不记成已发布
- 依赖：`media:acquireResourceReferences`、`media:confirmResourceReferences`

## content.copyPublishedReferences

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：把当前草稿的就绪引用复制为当前发布引用

```go
func copyPublishedReferences(ctx context.Context, tx *sql.Tx, op OperationContext, documentID, draftRevisionID, publishedRevisionID Identifier, referenceVersion int64) ([]gen.ResourceReference, int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 引用水位必须等于文档reference_version。非空集合必须等于锁外已确认pin的摘要，空集合不得带pin
2. 旧的current_published改为history后再插入新行

**错误**

- 无

- 副作用：publication_resource_references
- 授权：已认证发布事务
- 并发：只写当前短事务
- 幂等：每个publishedRevisionId只插入一次
- 依赖：`content.decodeStorageJSON`

## content.listPublicationReferences

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：读取一个文档已持久化的发布引用快照

```go
func listPublicationReferences(ctx context.Context, tx *sql.Tx, op OperationContext, documentID Identifier) ([]gen.ResourceReference, int, error)
```

**输入**

无

**输出**

无

**步骤**

1. 列kind为权威。超过10000条失败关闭

**错误**

- 无

- 副作用：无
- 授权：已认证发布事务
- 并发：只读当前短事务
- 幂等：只读
- 依赖：`content.decodeStorageJSON`

## content.archivePublishedReferences

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：撤回时把唯一的当前发布引用改为历史

```go
func archivePublishedReferences(ctx context.Context, tx *sql.Tx, op OperationContext, documentID Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 必须恰好有一行current_published，否则失败关闭

**错误**

- 无

- 副作用：publication_resource_references
- 授权：已认证撤回事务
- 并发：只写当前短事务
- 幂等：重复撤回由原回执拦截
- 依赖：无

## content.enqueuePublication

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：在当前业务事务登记发布变化、撤回和资源引用事件

```go
func enqueuePublication(ctx context.Context, op OperationContext, visibility int, changed *publicationChangedPayload, withdrawn *publicationWithdrawnPayload, documentID Identifier, sourceVersion int, refs []gen.ResourceReference) error
```

**输入**

无

**输出**

无

**步骤**

1. 每种载荷单独一个eventId。信封revision使用本次visibilityRevision

**错误**

- 无

- 副作用：outbox
- 授权：已认证发布事务
- 并发：只写已绑定的业务事务
- 幂等：eventId唯一
- 依赖：`content.PublishContentEvent`

## content.enqueueTreeChanged

- 级别：参考
- 文件：`modules/content/internal/publication_repository.go`
- 作用：发布或撤回引起的树状态变化与原事务一起排队

```go
func enqueueTreeChanged(ctx context.Context, tx *sql.Tx, op OperationContext, treeRevision int64, changed []Identifier) error
```

**输入**

无

**输出**

无

**步骤**

1. 沿用content.tree.changed的站点subject和树修订

**错误**

- 无

- 副作用：outbox
- 授权：已认证发布事务
- 并发：只写当前短事务
- 幂等：eventId唯一
- 依赖：`platform.EnqueueEvent`

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
5. SourceVersion只取首次意图固定的独立文档引用水位并核同一COMMITTED引用记录，不能按draftVersion重算或用旧重放回退当前水位
6. 独立只读ReferenceRuntime；同SQL联查精确来源和真实COMMITTED资格/完整回执/完整引用，复用List来源终局判定，仍不把pending/404当释放证据
7. COMMITTED为不可撤销历史决定；后来正文/文档隔离不撤销完整精确资格、原回执与refs的资源保护。读取当前正文仍由原隔离规则拒绝；缺资格或材料矛盾仍INTEGRITY_ERROR，绝不降pending/abort

**错误**

- `VALIDATION`：请求不符合协议（不可重试）
- `UNAUTHENTICATED`：服务证明无效（不可重试）
- `FORBIDDEN`：非固定调用服务或越过只读范围（不可重试）
- `VERSION_MISMATCH`：当前dataEpoch不匹配（不可重试）
- `INTEGRITY_ERROR`：私有权威数据损坏（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖或存储未知（可重试）
- `NOT_FOUND`：当前站点无该来源意图；不能作为释放证据（不可重试）

- 副作用：无
- 授权：permission：service；rule：只接受已验证纯服务actor，固定调用方media；不授予模型调用或管理写入权限
- 并发：当前权威快照一致读取；不持锁执行跨服务IO
- 幂等：无写入或费用副作用；每次读当前权威，no-store，不创建后台任务
- 依赖：`content.referenceRuntimeFromContext`、`content.checkRevisionScope`、`content.decodeStorageJSON`、`content.validateDraftOperationPlan`、`content.readCommittedReferenceSource`

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

1. 仅登记的纯服务可调用，含memory，因为公开读取内部复用本函数；site-config只看可见性，不读正文
2. 一次只读快照比较当前发布引用、快照哈希、schema和visibilityRevision
3. 未知文档visible=false且不暴露草稿。调用方哈希或修订不当前则visible=false，不把整批当校验错误。分享图或损坏的发布权威返回依赖不可用，不能把旧缓存当有效

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：无
- 授权：service已登记caller；请求refs只含曾公开对象
- 并发：同txn快照，传输开始前再核最新可见水位
- 幂等：只读
- 依赖：`content.readTreeSnapshot`、`content.readPublicationSnapshot`、`content.checkRevisionScope`

## content.readPublicationSnapshot

- 级别：参考
- 文件：`modules/content/internal/visibility_service.go`
- 作用：按站点读取一条不可变发布快照

```go
func readPublicationSnapshot(ctx context.Context, tx *sql.Tx, op OperationContext, publishedRevisionID Identifier) (storedPublication, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 缺失返回未找到。存储错误失败关闭，不把缺表当未发布

**错误**

- 无

- 副作用：无
- 授权：已登记的只读发布调用链
- 并发：只读当前短事务
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
- 依赖：`content.ValidatePublications`、`content.ReadRevisionBody`、`content.loadVisiblePublication`、`content.checkRevisionScope`

## content.loadVisiblePublication

- 级别：参考
- 文件：`modules/content/internal/publication_reader.go`
- 作用：读取当前仍公开且快照一致的文档

```go
func loadVisiblePublication(ctx context.Context, tx *sql.Tx, op OperationContext, runtime TreeRuntime, documentID Identifier) (gen.DocumentMetadata, storedPublication, error)
```

**输入**

无

**输出**

无

**步骤**

1. 非published、无当前引用或分享图未装配时失败关闭。历史修订不在这里返回正文

**错误**

- 无

- 副作用：无
- 授权：已登记的公开读取调用链
- 并发：只读当前短事务
- 幂等：只读
- 依赖：`content.readTreeSnapshot`、`content.readPublicationSnapshot`

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
- 依赖：`content.PlanOldPathRedirects`、`content.ValidatePublications`、`content.AcquireDocumentFence`、`content.DocumentState`、`content.ReleaseFence`

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
- 依赖：`content.ValidatePathSegment`、`content.EnsureTreeAcyclic`、`media:readAsset`、`content.AcquireDocumentFence`、`content.DocumentState`、`content.ReleaseFence`

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
- 依赖：`content.AcquireDocumentFence`、`content.DocumentState`、`content.ReleaseFence`

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

1. 固定media/site-config/data-protection纯服务或portal当前content.read授权，读私有当前站点/打开epoch；不依赖编辑租约
2. 每页同一只读SQL快照核全站reference_scan_state、元数据与head/COMMITTED/原回执/引用的双向覆盖；未知published/分享图生产资格、缺失或损坏失败关闭，不将未实现来源当空
3. 完整扫描当前草稿与不可变历史，以及Acquire前已持久的pending保存意图；COMMITTED资格优先于旧intent状态，明确aborted/rejected且无资格才排除，pending_pin/retained=true仅表示保护
4. 固定来源键排序，每页最多100项，完整源操作和引用扫描预算各20000；超限503而不返回虚假完成；过滤只在完整核对之后
5. 签名cursor绑定过滤/limit/caller/actor/site/epoch/session及版本，记录全站水位与位置；跨页任一来源变化409须首批重读，不拼接快照；末页hasMore=false且省略cursor，返回no-store

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）
- `VALIDATION`：游标签名/绑定或位置无效（不可重试）
- `UNAUTHENTICATED`：当前后台会话已失效（不可重试）
- `VERSION_MISMATCH`：数据代次或跨页来源水位已变化，必须从头读取（可重试）
- `INTEGRITY_ERROR`：私有来源权威、资格或完整引用材料矛盾（不可重试）

- 副作用：无
- 授权：permission：service或content.read；rule：media/site-config/data-protection固定纯服务；portal必须即时content.read，同站点当前epoch
- 并发：只读SQL一致快照；跨页固定全站水位，来源变化拒绝旧游标；不持SQL跨授权HTTP或签名私钥文件读取
- 幂等：只读
- 依赖：`content.referenceRuntimeFromContext`、`content.authorizeEditLock`、`content.readReferenceCursor`、`content.checkRevisionScope`、`content.validateReferenceCoverage`、`content.readReferenceSources`、`content.referenceSortKey`、`platform.SignCursor`

## content.NewReferenceRuntime

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：构造真实只读来源依赖及唯一持久schema校验器

```go
func NewReferenceRuntime(store *Store, identity *gen.IdentityClient, key platform.SigningKey) (ReferenceRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 构造真实只读来源依赖及唯一持久schema校验器

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：`platform.NewPayloadValidator`

## content.WithReferenceRuntime

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：向调用链装配本域只读来源依赖

```go
func WithReferenceRuntime(ctx context.Context, runtime ReferenceRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 向调用链装配本域只读来源依赖

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：无

## content.referenceRuntimeFromContext

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：读取已装配的真实只读来源依赖

```go
func referenceRuntimeFromContext(ctx context.Context) (ReferenceRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取已装配的真实只读来源依赖

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：`content.WithReferenceRuntime`

## content.readReferenceCursor

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：验证绑定查询/主体/代次的签名来源游标

```go
func readReferenceCursor(runtime ReferenceRuntime, op OperationContext, input ListResourceReferencesInput) (referenceCursor, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 验证绑定查询/主体/代次的签名来源游标

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：`content.decodeStorageJSON`、`platform.VerifyCursor`

## content.validateReferenceCoverage

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：核私有身份和草稿、发布快照的双向覆盖；分享图或不完整发布失败关闭

```go
func validateReferenceCoverage(ctx context.Context, tx *sql.Tx, op OperationContext, runtime ReferenceRuntime) error
```

**输入**

无

**输出**

无

**步骤**

1. 完整已发布快照连同已提交草稿修订、历史和current_published可以接受。快照不必等于后来移动过的当前head
2. 分享图、非活动状态、已发布但缺快照或引用、撤回后没有历史的lastPublishedAt失败关闭

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：无

## content.readReferenceSources

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：在同快照有界完整扫描源意图，核正文引用和pin摘要再投影全部保护

```go
func readReferenceSources(ctx context.Context, tx *sql.Tx, op OperationContext, runtime ReferenceRuntime) ([]gen.ResourceReference, error)
```

**输入**

无

**输出**

无

**步骤**

1. 在同快照有界完整扫描源意图，核正文引用和pin摘要再投影全部保护

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：`content.decodeStorageJSON`、`content.validateDraftOperationPlan`、`content.draftMediaReferences`、`content.readCommittedReferenceSource`、`content.referenceSortKey`、`content.readPublicationReferences`

## content.readPublicationReferences

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：读取已发布引用，并以列中的kind覆盖历史分类

```go
func readPublicationReferences(ctx context.Context, tx *sql.Tx, op OperationContext, runtime ReferenceRuntime) ([]gen.ResourceReference, error)
```

**输入**

无

**输出**

无

**步骤**

1. 与草稿历史相同的排序键只保留先读到的一条。来源必须是content且仍保留

**错误**

- 无

- 副作用：无
- 授权：已验证的引用来源读取
- 并发：同一SQL快照
- 幂等：只读
- 依赖：`content.decodeStorageJSON`

## content.readCommittedReferenceSource

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：同SQL核COMMITTED优先的精确资格/原回执/完整引用；无资格的pending投影pending_pin保护

```go
func readCommittedReferenceSource(ctx context.Context, tx *sql.Tx, op OperationContext, source referenceSourceRecord, plan draftOperationPlan) ([]gen.ResourceReference, string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 同SQL核COMMITTED优先的精确资格/原回执/完整引用；无资格的pending投影pending_pin保护
2. COMMITTED为不可撤销历史决定；后来正文/文档隔离不撤销完整精确资格、原回执与refs的资源保护。读取当前正文仍由原隔离规则拒绝；缺资格或材料矛盾仍INTEGRITY_ERROR，绝不降pending/abort

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：`content.decodeStorageJSON`、`content.sameStoredDraft`

## content.referenceSortKey

- 级别：参考
- 文件：`modules/content/internal/reference_reader.go`
- 作用：生成文档/修订/水位/分类/块/精确资产的稳定分页排序键

```go
func referenceSortKey(ref gen.ResourceReference) string
```

**输入**

无

**输出**

无

**步骤**

1. 生成文档/修订/水位/分类/块/精确资产的稳定分页排序键

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
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

1. 必须且仅有一种载荷，禁止正文或密钥。没有已绑定的发布事务则失败关闭
2. 信封revision使用本次visibilityRevision。变化和撤回的发生时间取载荷时间
3. 只在业务事务内写outbox；提交后由content.RelayOutbox经jobs至少一次投递，本函数不直接调用jobs

**错误**

- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `DEPENDENCY_UNAVAILABLE`：权威依赖、存储或密钥不可用；不可把未知状态当允许（可重试）

- 副作用：私有事务/正文提交日志/operation receipt/outbox
- 授权：私有contenttxn
- 并发：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入
- 幂等：eventId唯一
- 依赖：`platform.EnqueueEvent`

## content.bindPublicationTx

- 级别：参考
- 文件：`modules/content/internal/outbox_repository.go`
- 作用：把当前发布事务交给同一调用链的outbox写入

```go
func bindPublicationTx(ctx context.Context, tx *sql.Tx) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 只绑定调用者已经打开的事务，不提交、不另开连接

**错误**

- 无

- 副作用：无
- 授权：已认证发布或撤回事务
- 并发：不跨远程IO
- 幂等：绑定本身无持久副作用
- 依赖：无

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

1. 仅本域后台纯服务；从受信RelayRuntime取得真实Store、版本化JobsClient和当前平台写窗口，核打开epoch与站点
2. 把真实outbox.NewStore及outbox.NewEventPublisher(gen.JobsClient)交给platform.RunRelayBatch，领域不复制行租约、确认、退避、隔离和恢复丢弃机制
3. 单批至多batchLimit；网络返回后由公共机制重新取得当前安全窗口写状态；返回本批relayed/pending/quarantined计数，不代表全域已送达；后续每秒由完整启动后台调用，当前完整生命周期仍未装配

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用或超时；行保留待下一轮（可重试）
- `INTEGRITY_ERROR`：同eventId异payloadHash或信封不合法；行隔离（不可重试）
- `VERSION_MISMATCH`：dataEpoch与当前active epoch不一致；旧代行按恢复规则丢弃（不可重试）
- `FORBIDDEN`：非本域后台纯服务调用（不可重试）

- 副作用：读写本域outbox行状态与租约；HTTP调用jobs:publishEvent
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个转发循环；行级租约防多实例重复领取；同subject按outbox序号串行发送
- 幂等：eventId在业务事务内固定；jobs按eventId+payloadHash去重，重复发送返回原确认
- 依赖：`content.WithRelayRuntime`、`content.checkRevisionScope`、`platform.RunRelayBatch`、`jobs:publishEvent`

## content.WithRelayRuntime

- 级别：参考
- 文件：`modules/content/internal/outbox_repository.go`
- 作用：向本域后台调用链装配真实outbox依赖

```go
func WithRelayRuntime(ctx context.Context, runtime RelayRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 向本域后台调用链装配真实outbox依赖

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：无

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
7. 本轮真实构造独立ReferenceRuntime并在未启动Application绑定两个只读来源路由；完整x-handler/activation/后台生命周期未齐仍返回nil和DEPENDENCY_UNAVAILABLE，逆序关闭真实资源，不运行半成品服务
8. 本轮构造TreeRuntime并注册四树路由和四个真实发布路由；已配置media端点时挂上现有MediaClient供发布pin。没有可信写窗口、全量handler、恢复activation和后台生命周期时仍返回nil和DEPENDENCY_UNAVAILABLE，不运行半成品服务

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`site-config:registerActivation`、`content.RecoverPreparedCommits`、`content.ExpireIdleLocks`、`content.RelayOutbox`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewSiteConfigClient`、`platform.NewRequestValidator`、`content.OpenDatabase`、`content.RecoverDatabaseJournal`、`content.CloseDatabase`、`content.ConfigureRevisionStorage`、`content.WithRevisionFileStore`、`content.NewEditLockManager`、`content.CloseEditLocks`、`content.WithEditLockRuntime`、`content.WithDraftRuntime`、`content.NewReferenceRuntime`、`content.RegisterReferenceRoutes`、`content.NewTreeRuntime`、`content.RegisterTreeRoutes`、`content.RegisterPublicationRoutes`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）；完整handler装配时构造ConfigureRevisionStorage并通过WithRevisionFileStore传递；本轮只有Open/Recover真实接入，完整装配继续blocked；本轮真实构造EditLockManager单leader至退出；完整锁handler/identity安全姓名/通知审计编排仍blocked

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

## content.RegisterReferenceRoutes

- 级别：参考
- 文件：`modules/content/internal/transport/server.go`
- 作用：在未启动应用装配两个真实来源HTTP路由，统一服务身份/代次/schema边界

```go
func RegisterReferenceRoutes(app *Application, runtime ReferenceRuntime, anchors platform.TrustAnchors) error
```

**输入**

无

**输出**

无

**步骤**

1. 在未启动应用装配两个真实来源HTTP路由，统一服务身份/代次/schema边界
2. integer查询参数按统一kin-openapi严格边界的基数0 ParseInt转换，兼容已允许的十六/八进制表示，不使用十进制Atoi误转0

**错误**

- 无

- 副作用：无
- 授权：已验证服务边界或受信启动装配；只读取本域私有权威，不放宽来源资格
- 并发：同一SQL快照不跨远程IO；游标签名在读取结束后执行
- 幂等：只读和依赖装配无付费或后台任务副作用；来源变化不得虚报完整
- 依赖：`content.WithReferenceRuntime`、`content.ListResourceReferences`、`content.ReadReferenceDecision`、`platform.NewServiceHandler`

## content.RegisterTreeRoutes

- 级别：参考
- 文件：`modules/content/internal/transport/server.go`
- 作用：注册四个真实树映射及统一身份/epoch/schema边界

```go
func RegisterTreeRoutes(app *Application, runtime TreeRuntime, anchors platform.TrustAnchors) error
```

**输入**

无

**输出**

无

**步骤**

1. 仅五个已实际注册固定action由核心即时鉴权一次，core按state选read/publish；其他action正常通用Authorize，不使用私有marker/缓存
2. 统一边界验证服务/委托/epoch/schema，再精确映射HTTP DTO；/v1/documents只注册GET，不绕过全量handler启动门槛
3. integer查询参数按统一kin-openapi严格边界的基数0 ParseInt转换，兼容已允许的十六/八进制表示，不使用十进制Atoi误转0
4. 新增PUT metadata精确映射路径documentId与生成的严格bodyDTO；仅新增固定updateDocumentMetadata action由core一次即时content.edit，平台服务/委托/epoch/schema仍严格校验

**错误**

- 无

- 副作用：无
- 授权：受信装配或当前已认证树入口；即时权限在DB/锁之前，内部信任schema输入
- 并发：不持SQL或互斥跨远程IO；写窗口在锁外，当前元数据与CAS同SQL
- 幂等：同scope动作摘要回原结果；纯函数无副作用，未知提交不重复业务
- 依赖：`content.WithTreeRuntime`、`content.ReadDocumentTree`、`content.ListDocuments`、`content.MoveDocument`、`content.ReorderChildren`、`platform.NewServiceHandler`、`platform.NewIdentityAuthorizer`、`content.UpdateDocumentMetadata`

## content.RegisterPublicationRoutes

- 级别：参考
- 文件：`modules/content/internal/transport/server.go`
- 作用：绑定四个已实现的发布HTTP处理函数

```go
func RegisterPublicationRoutes(app *Application, runtime TreeRuntime, anchors platform.TrustAnchors) error
```

**输入**

无

**输出**

无

**步骤**

1. 发布和撤回由核心一次核content.publish，本注册表不再重复用户RPC。可见性与公开读取按固定调用方校验服务身份
2. 不注册占位503。完整启动仍由BuildApplication失败关闭

**错误**

- 无

- 副作用：无
- 授权：受信启动装配；业务权限由发布入口或服务边界执行
- 并发：装配时不持业务事务
- 幂等：重复装配同一进程只注册一次路由表
- 依赖：`content.PublishDocument`、`content.WithdrawDocument`、`content.ValidatePublications`、`content.ReadPublishedRevision`、`platform.NewServiceHandler`

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
3. 校验存储活动epoch后调用平台outbox.EnsureSchema建立共享outbox定义，不在迁移复制机制

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.RunContentMigrations`、`content.syncDirectory`

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

## content.ReadDocumentEditState

- 级别：参考
- 文件：`modules/content/internal/document_edit_state.go`
- 作用：读取真实文档生命周期、删除fence、独立引用水位与隔离状态

```go
func (s *Store) ReadDocumentEditState(ctx context.Context, op OperationContext, documentID Identifier) (DocumentEditState, error)
```

**输入**

无

**输出**

无

**步骤**

1. 从私有documents及document_quarantines读取存在/状态/revision/deleteFence/referenceVersion/隔离；缺行NOT_FOUND，不猜建文档或引用资格

**错误**

- 无

- 副作用：无
- 授权：受信本域启动装配或已经授权的私有正文调用链；scope/epoch/提交资格由本存储负责
- 并发：单active content进程拥有私有根；SQLite单连接及本域互斥；不持DB事务跨文件IO
- 幂等：原scope/摘要回放首次材料；终局不降级，不推进head、不造COMMITTED资格
- 依赖：`content.checkRevisionScope`、`content.contentStorageError`

## content.AcquireEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_operations.go`
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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；UNAUTHENTICATED: 权威会话或权限版本已撤销；CONFLICT: operationId已绑定另一动作/文档/规范摘要；VERSION_MISMATCH: doc/deleteFence/dataEpoch或待释lease已变化；DEPENDENCY_UNAVAILABLE: 身份/运行权威或私有存储不可用，未知提交保持pending；INTEGRITY_ERROR: 原回执哈希或安全姓名不完整 |

**步骤**

1. 以identity:checkAuthorization即时核当前会话、本站功能权限与撤销版本；姓名仅取同一权威响应的accountName并按纯文本显示
2. 规范摘要含动作和完整原输入；scope绑定caller/actor/site/epoch/operationId，先读原回执再读文档状态，同键异动作/文档/参数返回CONFLICT
3. 仅draft/published且未隔离可编辑；不存在NOT_FOUND
4. 空/过期分配bootId+单调fence与2分钟TTL；有效别人持锁返回既有成功DTO lease+writable=false，安全姓名来自其identity授权时快照
5. 同账户多tab共享lease，不重置活跃时刻/序号/fence，不能绕过draftVersion CAS
6. 无原回执时经平台当前写窗口，在life读锁与同doc mutex内复制候选；真实document/state/deleteFence/隔离在短SQL事务内重核
7. 候选状态变化与本地审计、content.edit-lock.changed v1 outbox和完整原回执同事务；提交确认后才安装内存，SQL未知提交保留pending，所有后续编辑/保存先按真实回执解决
8. 重启只回放历史原结果，不从原回执重建租约；旧boot/fence仍永失效。事件由平台outbox经jobs投递；HTTP/在线消费装配另行完成

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `NOT_FOUND`：资源不存在或在当前主体作用域中不可见（不可重试）
- `UNAUTHENTICATED`：权威会话或权限版本已撤销（不可重试）
- `CONFLICT`：operationId已绑定另一动作/文档/规范摘要（不可重试）
- `VERSION_MISMATCH`：doc/deleteFence/dataEpoch或待释lease已变化（不可重试）
- `DEPENDENCY_UNAVAILABLE`：身份/运行权威或私有存储不可用，未知提交保持pending（可重试）
- `INTEGRITY_ERROR`：原回执哈希或安全姓名不完整（不可重试）

- 副作用：本域原操作回执/编辑状态事件outbox/本地审计与提交确认后的内存租约；不改草稿正文或head
- 授权：permission：content.edit；rule：当前管理账户、同站点content.edit即时授权；release另核当前lease归属
- 并发：先取公共写窗口（不持doc锁做HTTP），life读锁→同doc mutex→短私有SQL事务；同保存guard互斥；未知提交pending在保存前解决
- 幂等：caller/actor/site/epoch/operationId唯一；动作和完整原输入规范摘要；同键同摘要回原结果，同键异摘要CONFLICT，回放不改变内存或重复事件
- 依赖：`content.editLockRuntimeFromContext`、`content.authorizeEditLock`、`content.AcquireLease`
- 其他依赖说明：完整用例及可靠排队已实现候选，HTTP绑定/在线通知消费者/完整服务尚未装配；检查前不记完成；身份前置：identity.AuthorizationDecision必填accountName，来自同次权威会话账户读取

## content.RenewEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_operations.go`
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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 权威会话或权限版本已撤销；CONFLICT: operationId已绑定另一动作/文档/规范摘要；VERSION_MISMATCH: doc/deleteFence/dataEpoch或待释lease已变化；DEPENDENCY_UNAVAILABLE: 身份/运行权威或私有存储不可用，未知提交保持pending；INTEGRITY_ERROR: 原回执哈希或安全姓名不完整；NOT_FOUND: 本站真实文档不存在 |

**步骤**

1. 以identity:checkAuthorization即时核当前会话、本站功能权限与撤销版本；姓名仅取同一权威响应的accountName并按纯文本显示
2. 规范摘要含动作和完整原输入；scope绑定caller/actor/site/epoch/operationId，先读原回执再读文档状态，同键异动作/文档/参数返回CONFLICT
3. 核holder/lease/boot/fence当前且未到期，未知或过期不能复活
4. heartbeatSequence严格前进才延长2分钟TTL；inputSequence严格前进才按服务接收时钟更新LastInputAt并清空空闲提醒，客户端lastInputAt仅兼容显示
5. 等或旧心跳仅回当前快照，不续TTL/更新活动；成功响应包含当前共享账户lease的两个最新序号，同账户多tab先衔接服务序号，不拆成每tab锁
6. 提醒界点固定LastInputAt+15分钟，LastInputAt+17分钟绝对失效；检测延迟不能延后，超过界点的新输入不能续活；实际通知/审计仍由完整业务编排可靠发送
7. 无原回执时经平台当前写窗口，在life读锁与同doc mutex内复制候选；真实document/state/deleteFence/隔离在短SQL事务内重核
8. 候选状态变化与本地审计、content.edit-lock.changed v1 outbox和完整原回执同事务；提交确认后才安装内存，SQL未知提交保留pending，所有后续编辑/保存先按真实回执解决
9. 重启只回放历史原结果，不从原回执重建租约；旧boot/fence仍永失效。事件由平台outbox经jobs投递；HTTP/在线消费装配另行完成

**错误**

- `LOCKED`：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行（不可重试）
- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VALIDATION`：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值（不可重试）
- `UNAUTHENTICATED`：权威会话或权限版本已撤销（不可重试）
- `CONFLICT`：operationId已绑定另一动作/文档/规范摘要（不可重试）
- `VERSION_MISMATCH`：doc/deleteFence/dataEpoch或待释lease已变化（不可重试）
- `DEPENDENCY_UNAVAILABLE`：身份/运行权威或私有存储不可用，未知提交保持pending（可重试）
- `INTEGRITY_ERROR`：原回执哈希或安全姓名不完整（不可重试）
- `NOT_FOUND`：本站真实文档不存在（不可重试）

- 副作用：本域原操作回执/编辑状态事件outbox/本地审计与提交确认后的内存租约；不改草稿正文或head
- 授权：permission：content.edit；rule：当前管理账户、同站点content.edit即时授权；release另核当前lease归属
- 并发：先取公共写窗口（不持doc锁做HTTP），life读锁→同doc mutex→短私有SQL事务；同保存guard互斥；未知提交pending在保存前解决
- 幂等：caller/actor/site/epoch/operationId唯一；动作和完整原输入规范摘要；同键同摘要回原结果，同键异摘要CONFLICT，回放不改变内存或重复事件
- 依赖：`content.editLockRuntimeFromContext`、`content.authorizeEditLock`、`content.RenewLease`
- 其他依赖说明：完整用例及可靠排队已实现候选，HTTP绑定/在线通知消费者/完整服务尚未装配；检查前不记完成；身份前置：identity.AuthorizationDecision必填accountName，来自同次权威会话账户读取

## content.ReleaseEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_operations.go`
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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；UNAUTHENTICATED: 权威会话或权限版本已撤销；CONFLICT: operationId已绑定另一动作/文档/规范摘要；VERSION_MISMATCH: doc/deleteFence/dataEpoch或待释lease已变化；DEPENDENCY_UNAVAILABLE: 身份/运行权威或私有存储不可用，未知提交保持pending；INTEGRITY_ERROR: 原回执哈希或安全姓名不完整；LOCKED: 编辑租约失效、文档不接受编辑或维护窗口关闭；NOT_FOUND: 本站真实文档不存在 |

**步骤**

1. 以identity:checkAuthorization即时核当前会话、本站功能权限与撤销版本；姓名仅取同一权威响应的accountName并按纯文本显示
2. 规范摘要含动作和完整原输入；scope绑定caller/actor/site/epoch/operationId，先读原回执再读文档状态，同键异动作/文档/参数返回CONFLICT
3. 核lease归属与boot
4. 锁仍匹配则原子删除；缺失视已释放
5. 发编辑状态元数据，关闭beacon失败2分钟TTL兜底
6. 无原回执时经平台当前写窗口，在life读锁与同doc mutex内复制候选；真实document/state/deleteFence/隔离在短SQL事务内重核
7. 候选状态变化与本地审计、content.edit-lock.changed v1 outbox和完整原回执同事务；提交确认后才安装内存，SQL未知提交保留pending，所有后续编辑/保存先按真实回执解决
8. 重启只回放历史原结果，不从原回执重建租约；旧boot/fence仍永失效。事件由平台outbox经jobs投递；HTTP/在线消费装配另行完成

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `UNAUTHENTICATED`：权威会话或权限版本已撤销（不可重试）
- `CONFLICT`：operationId已绑定另一动作/文档/规范摘要（不可重试）
- `VERSION_MISMATCH`：doc/deleteFence/dataEpoch或待释lease已变化（不可重试）
- `DEPENDENCY_UNAVAILABLE`：身份/运行权威或私有存储不可用，未知提交保持pending（可重试）
- `INTEGRITY_ERROR`：原回执哈希或安全姓名不完整（不可重试）
- `LOCKED`：编辑租约失效、文档不接受编辑或维护窗口关闭（不可重试）
- `NOT_FOUND`：本站真实文档不存在（不可重试）

- 副作用：本域原操作回执/编辑状态事件outbox/本地审计与提交确认后的内存租约；不改草稿正文或head
- 授权：permission：content.edit；rule：当前管理账户、同站点content.edit即时授权；release另核当前lease归属
- 并发：先取公共写窗口（不持doc锁做HTTP），life读锁→同doc mutex→短私有SQL事务；同保存guard互斥；未知提交pending在保存前解决
- 幂等：caller/actor/site/epoch/operationId唯一；动作和完整原输入规范摘要；同键同摘要回原结果，同键异摘要CONFLICT，回放不改变内存或重复事件
- 依赖：`content.editLockRuntimeFromContext`、`content.authorizeEditLock`、`content.ReleaseLease`
- 其他依赖说明：完整用例及可靠排队已实现候选，HTTP绑定/在线通知消费者/完整服务尚未装配；检查前不记完成；身份前置：identity.AuthorizationDecision必填accountName，来自同次权威会话账户读取

## content.ForceReleaseEditLock

- 级别：约束
- 文件：`modules/content/internal/edit_lock_operations.go`
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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；UNAUTHENTICATED: 权威会话或权限版本已撤销；CONFLICT: operationId已绑定另一动作/文档/规范摘要；DEPENDENCY_UNAVAILABLE: 身份/运行权威或私有存储不可用，未知提交保持pending；INTEGRITY_ERROR: 原回执哈希或安全姓名不完整；LOCKED: 编辑租约失效、文档不接受编辑或维护窗口关闭；NOT_FOUND: 本站真实文档不存在 |

**步骤**

1. 以identity:checkAuthorization即时核当前会话、本站功能权限与撤销版本；姓名仅取同一权威响应的accountName并按纯文本显示
2. 规范摘要含动作和完整原输入；scope绑定caller/actor/site/epoch/operationId，先读原回执再读文档状态，同键异动作/文档/参数返回CONFLICT
3. superadmin专属，不可分配给子账号
4. 同doc mutex核expectedLease防释放已换人锁，增加fence并删锁
5. 发强制只读事件；已自动保存草稿保留，旧fence所有后续save拒绝
6. 无原回执时经平台当前写窗口，在life读锁与同doc mutex内复制候选；真实document/state/deleteFence/隔离在短SQL事务内重核
7. 候选状态变化与本地审计、content.edit-lock.changed v1 outbox和完整原回执同事务；提交确认后才安装内存，SQL未知提交保留pending，所有后续编辑/保存先按真实回执解决
8. 重启只回放历史原结果，不从原回执重建租约；旧boot/fence仍永失效。事件由平台outbox经jobs投递；HTTP/在线消费装配另行完成

**错误**

- `FORBIDDEN`：主体没有所需操作权限，或站点/资源作用域不匹配（不可重试）
- `VERSION_MISMATCH`：expectedRevision、对象版本或 dataEpoch 已改变（不可重试）
- `UNAUTHENTICATED`：权威会话或权限版本已撤销（不可重试）
- `CONFLICT`：operationId已绑定另一动作/文档/规范摘要（不可重试）
- `DEPENDENCY_UNAVAILABLE`：身份/运行权威或私有存储不可用，未知提交保持pending（可重试）
- `INTEGRITY_ERROR`：原回执哈希或安全姓名不完整（不可重试）
- `LOCKED`：编辑租约失效、文档不接受编辑或维护窗口关闭（不可重试）
- `NOT_FOUND`：本站真实文档不存在（不可重试）

- 副作用：本域原操作回执/编辑状态事件outbox/本地审计与提交确认后的内存租约；不改草稿正文或head
- 授权：permission：superadmin；rule：唯一超管即时权威授权；不可分配给子账号
- 并发：先取公共写窗口（不持doc锁做HTTP），life读锁→同doc mutex→短私有SQL事务；同保存guard互斥；未知提交pending在保存前解决
- 幂等：caller/actor/site/epoch/operationId唯一；动作和完整原输入规范摘要；同键同摘要回原结果，同键异摘要CONFLICT，回放不改变内存或重复事件
- 依赖：`content.editLockRuntimeFromContext`、`content.authorizeEditLock`、`content.ForceReleaseLease`
- 其他依赖说明：完整用例及可靠排队已实现候选，HTTP绑定/在线通知消费者/完整服务尚未装配；检查前不记完成；身份前置：identity.AuthorizationDecision必填accountName，来自同次权威会话账户读取

## content.ExpireIdleLocks

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
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
| `error` | `Error` | 失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试 | INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；UNAUTHENTICATED: 权威会话或权限版本已撤销；CONFLICT: operationId已绑定另一动作/文档/规范摘要；VERSION_MISMATCH: doc/deleteFence/dataEpoch或待释lease已变化；DEPENDENCY_UNAVAILABLE: 身份/运行权威或私有存储不可用，未知提交保持pending；LOCKED: 编辑租约失效、文档不接受编辑或维护窗口关闭；NOT_FOUND: 本站真实文档不存在 |

**步骤**

1. 仅本站content纯服务后台；输入now来自本域调度器服务时钟，同operationId重试保留该时刻
2. 逐doc mutex检查expiresAt或LastInputAt+17分钟绝对界点；15分钟提醒固定LastInputAt+15分钟，检测延迟不延后
3. 每篇候选提高fence/清租约或提醒，与本地审计/状态事件outbox/原回执同短SQL事务提交；失败不安装候选，未知提交按pending回执解决
4. 同一后台operationId包含每篇独立scope和now摘要，局部失败重试纳入已完成文档并回放原结果；不重复事件；单次最多10000变化，后续调度继续
5. 只枚举本站活跃cell；进程重启不从回执重建锁，bootId变化拒旧请求。通知在线消费与后台调度仍由启动装配负责

**错误**

- `INTEGRITY_ERROR`：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布（不可重试）
- `UNAUTHENTICATED`：权威会话或权限版本已撤销（不可重试）
- `CONFLICT`：operationId已绑定另一动作/文档/规范摘要（不可重试）
- `VERSION_MISMATCH`：doc/deleteFence/dataEpoch或待释lease已变化（不可重试）
- `DEPENDENCY_UNAVAILABLE`：身份/运行权威或私有存储不可用，未知提交保持pending（可重试）
- `LOCKED`：编辑租约失效、文档不接受编辑或维护窗口关闭（不可重试）
- `NOT_FOUND`：本站真实文档不存在（不可重试）

- 副作用：本域原操作回执/编辑状态事件outbox/本地审计与提交确认后的内存租约；不改草稿正文或head
- 授权：permission：service；rule：callerService=content、actor.kind=service、actor.subjectId=content；仅本域后台且本站scope
- 并发：先取公共写窗口（不持doc锁做HTTP），life读锁→同doc mutex→短私有SQL事务；同保存guard互斥；未知提交pending在保存前解决
- 幂等：逐doc原scope+动作+now摘要；同操作局部重试回放已完成结果，不重复通知；重启租约保持清空
- 依赖：`content.editLockRuntimeFromContext`、`content.ExpireLeases`
- 其他依赖说明：完整用例及可靠排队已实现候选，HTTP绑定/在线通知消费者/完整服务尚未装配；检查前不记完成；身份前置：identity.AuthorizationDecision必填accountName，来自同次权威会话账户读取

## content.WithEditLockRuntime

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：装配本域manager、identity客户端与平台当前写窗口，不导入他域业务

```go
func WithEditLockRuntime(ctx context.Context, runtime EditLockRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 装配本域manager、identity客户端与平台当前写窗口，不导入他域业务

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：无

## content.editLockRuntimeFromContext

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：缺依赖失败关闭，不伪造完整用例成功

```go
func editLockRuntimeFromContext(ctx context.Context) (EditLockRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 缺依赖失败关闭，不伪造完整用例成功

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：无

## content.authorizeEditLock

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：即时identity授权并核同主体/site/session/权限版本；安全姓名仅identity权威且按纯文本显示

```go
func authorizeEditLock(ctx context.Context, op OperationContext, client *gen.IdentityClient, permission string, action string, documentID Identifier) (string, error)
```

**输入**

无

**输出**

无

**步骤**

1. 即时identity授权并核同主体/site/session/权限版本；安全姓名仅identity权威且按纯文本显示

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`identity:checkAuthorization`

## content.executeEditLock

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：原scope摘要回放→平台写窗口→同doc候选→真实事务→确认后内存安装；未知提交pending保持失败关闭

```go
func (manager *EditLockManager) executeEditLock(ctx context.Context, op OperationContext, command editLockCommand) (EditLockSnapshot, error)
```

**输入**

无

**输出**

无

**步骤**

1. 原scope摘要回放→平台写窗口→同doc候选→真实事务→确认后内存安装；未知提交pending保持失败关闭

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`content.editLockRuntimeFromContext`、`content.lockEditCell`、`content.resolvePendingEditLock`、`content.readEditLockReceipt`、`content.ReadDocumentEditState`、`content.copyEditLease`、`content.planEditLock`、`content.commitEditLock`、`platform.NewWriteContext`

## content.planEditLock

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：只改变候选：共享lease/单调序号/固定15与17分钟界点/释放及fence；不写权威内存与SQL

```go
func (manager *EditLockManager) planEditLock(op OperationContext, command editLockCommand, state DocumentEditState, candidate *editLeaseCell, now time.Time) (EditLockSnapshot, error)
```

**输入**

无

**输出**

无

**步骤**

1. 只改变候选：共享lease/单调序号/固定15与17分钟界点/释放及fence；不写权威内存与SQL
2. 候选fence耗尽仅返回LOCKED，不安装候选或悄悄清权威lease；旧租约依原到期时间失效，保存仍纯核当前fence/期限

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`content.requireEditableDocument`、`content.refreshEditLease`、`content.advanceEditFence`、`content.matchEditLease`、`content.editLockSnapshot`

## content.resolvePendingEditLock

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：未知提交仅按原回执及候选hash安装或丢弃；读失败保留pending，不允许保存越过

```go
func (manager *EditLockManager) resolvePendingEditLock(ctx context.Context, cell *editLeaseCell) error
```

**输入**

无

**输出**

无

**步骤**

1. 未知提交仅按原回执及候选hash安装或丢弃；读失败保留pending，不允许保存越过

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`content.readEditLockReceipt`

## content.readEditLockReceipt

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：完整scope与摘要查原结果并核hash/文档/site，异摘要CONFLICT；不重建内存锁

```go
func (s *Store) readEditLockReceipt(ctx context.Context, query editLockQuery, key editLockReceiptKey) (EditLockSnapshot, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 完整scope与摘要查原结果并核hash/文档/site，异摘要CONFLICT；不重建内存锁

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`content.decodeStorageJSON`、`content.contentStorageError`

## content.commitEditLock

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：短SQL事务再核全局op冲突/文档生命周期deleteFence隔离，原回执+通知+审计原子；仅Commit尝试后错误返回hash供pending解决

```go
func (s *Store) commitEditLock(ctx context.Context, key editLockReceiptKey, state DocumentEditState, snapshot EditLockSnapshot, previous *gen.EditLease) (Hash,error)
```

**输入**

无

**输出**

无

**步骤**

1. 短SQL事务再核全局op冲突/文档生命周期deleteFence隔离，原回执+通知+审计原子；仅Commit尝试后错误返回hash供pending解决

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`content.readEditLockReceipt`、`content.enqueueEditLockChange`、`content.contentStorageError`

## content.enqueueEditLockChange

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：同事务递增跨boot持久事件revision并写最小状态outbox及本地审计，不含姓名/leaseId/正文

```go
func (s *Store) enqueueEditLockChange(ctx context.Context, tx *sql.Tx, op OperationContext, snapshot EditLockSnapshot, previous *gen.EditLease) error
```

**输入**

无

**输出**

无

**步骤**

1. 同事务递增跨boot持久事件revision并写最小状态outbox及本地审计，不含姓名/leaseId/正文

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`platform.EnqueueEvent`、`content.contentStorageError`

## content.expireEditLocks

- 级别：参考
- 文件：`modules/content/internal/edit_lock_operations.go`
- 作用：逐本站活跃doc与同操作已完成scope交集/并集回放，失败不重复已提交变化；固定调度now，最多10000个状态变化

```go
func (manager *EditLockManager) expireEditLocks(ctx context.Context, op OperationContext, now time.Time) ([]EditLockSnapshot,error)
```

**输入**

无

**输出**

无

**步骤**

1. 逐本站活跃doc与同操作已完成scope交集/并集回放，失败不重复已提交变化；固定调度now，最多10000个状态变化

**错误**

- 无

- 副作用：无
- 授权：已认证且即时授权的本域编辑调用链；权限仅完整入口判定，通用身份边界仅平台实现
- 并发：同doc mutex保护候选和未知提交；不持SQL事务或doc guard跨HTTP/文件IO
- 幂等：同scope规范摘要原回执，不重建重启后租约
- 依赖：`content.checkRevisionScope`、`content.executeEditLock`、`content.contentStorageError`

## content.WithDraftRuntime

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：装配真实本域草稿依赖与客户端

```go
func WithDraftRuntime(ctx context.Context, runtime DraftRuntime) context.Context
```

**输入**

无

**输出**

无

**步骤**

1. 装配真实本域草稿依赖与客户端

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.draftRuntimeFromContext

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：取得同真实Store绑定的草稿运行依赖

```go
func draftRuntimeFromContext(ctx context.Context) (DraftRuntime, error)
```

**输入**

无

**输出**

无

**步骤**

1. 取得同真实Store绑定的草稿运行依赖

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.WithDraftRuntime`

## content.readDraftOperation

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：按完整身份动作摘要回放首次意图和原始完整回执

```go
func (s *Store) readDraftOperation(ctx context.Context, op OperationContext, action string, digest Hash) (draftOperation, bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 按完整身份动作摘要回放首次意图和原始完整回执

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.checkRevisionScope`、`content.readDraftOperationFrom`

## content.prepareDraftOperation

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：在当前写窗口持久首次创建或保存意图并绑定站点

```go
func (s *Store) prepareDraftOperation(ctx context.Context, op OperationContext, record draftOperation) (draftOperation,error)
```

**输入**

无

**输出**

无

**步骤**

1. 在当前写窗口持久首次创建或保存意图并绑定站点

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.readDraftOperation`、`content.contentStorageError`

## content.draftRequestDigest

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：以动作和规范参数摘要绑定原操作

```go
func draftRequestDigest(action string,input any) (Hash,error)
```

**输入**

无

**输出**

无

**步骤**

1. 以动作和规范参数摘要绑定原操作

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.RevisionSiteID

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：返回受信草稿事务本站身份

```go
func (s *draftCommitStore) RevisionSiteID() Identifier
```

**输入**

无

**输出**

无

**步骤**

1. 返回受信草稿事务本站身份

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.BeginDraftCommit

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：建立延迟事务对象，尚不持SQL或doc guard

```go
func (s *draftCommitStore) BeginDraftCommit(ctx context.Context, request DraftCommitRequest) (DraftCommitTransaction,error)
```

**输入**

无

**输出**

无

**步骤**

1. 返回绑定真实Store、受信创建/保存意图及原op的延迟事务对象；此时不打开SQL，不取doc guard

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.FindReceipt

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：真实读取同scope规范摘要的COMMITTED原回执

```go
func (t *draftCommitTransaction) FindReceipt(ctx context.Context, operationID Identifier, digest Hash) (CommitDraftJournalResult,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 真实读取同scope规范摘要的COMMITTED原回执

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.decodeStorageJSON`、`content.sameStoredDraft`、`content.contentStorageError`

## content.ValidateFence

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：文件IO完成后建立当前写窗口和真实保存guard并开启短SQL

```go
func (t *draftCommitTransaction) ValidateFence(ctx context.Context, input CommitDraftJournalInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 先建立当前公共写窗口，再获取真实SaveFenceGuard或创建生命读锁，之后才开短SQL
2. 同事务复核固定意图、PREPARED材料、本站代次、文档/deleteFence/隔离与head；创建必须专用意图和站点树CAS

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.AcquireSaveFence`、`content.validateDraftTransaction`、`platform.NewWriteContext`

## content.validateDraftTransaction

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：同SQL重查意图准备、持久站点代次、生命周期删除fence与head

```go
func (t *draftCommitTransaction) validateDraftTransaction(ctx context.Context,input CommitDraftJournalInput) error
```

**输入**

无

**输出**

无

**步骤**

1. 同SQL重查意图准备、持久站点代次、生命周期删除fence与head
2. 引用水位独立于draftVersion：首次意图冻结ExpectedReferenceVersion及SourceVersion，短SQL与head/ledger/refs同事务CAS推进documents.reference_version；公共Revision耗尽失败，旧/未知行不猜补资格

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.sameStoredDraft`、`content.sameJournalIdentity`、`content.sameIdentifierPointer`、`content.validateCreateTransaction`、`content.requireEditableDocument`、`content.matchEditLease`、`content.decodeStorageJSON`、`content.contentStorageError`

## content.Commit

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：原子提交站点树或head CAS、ledger、COMMITTED资格、refs、完整用例回执与平台outbox

```go
func (t *draftCommitTransaction) Commit(ctx context.Context,request DraftCommitRequest) (CommitDraftJournalResult,error)
```

**输入**

无

**输出**

无

**步骤**

1. 创建只插入本次固定docId，保存只CAS现有head/version；同事务维护全部历史稳定IDledger
2. 同事务落COMMITTED资格和原草稿回执、完整Create/Save原结果、当前/历史引用与平台outbox
3. SQL Commit返回未知时终结并释放短guard，但保留pending意图和pin；仅真实原回执可认定成功
4. 引用水位独立于draftVersion：首次意图冻结ExpectedReferenceVersion及SourceVersion，短SQL与head/ledger/refs同事务CAS推进documents.reference_version；公共Revision耗尽失败，旧/未知行不猜补资格

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.commitStableIDs`、`content.Rollback`、`content.releaseDraftTransaction`、`content.contentStorageError`、`platform.EnqueueEvent`

## content.commitStableIDs

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：同head CAS事务拒绝全部历史retired ID复用并追加退役记录

```go
func (t *draftCommitTransaction) commitStableIDs(ctx context.Context,op OperationContext,plan draftOperationPlan) error
```

**输入**

无

**输出**

无

**步骤**

1. 同head CAS事务拒绝全部历史retired ID复用并追加退役记录

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.contentStorageError`

## content.releaseDraftTransaction

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：释放短事务持有的真实doc或创建生命guard及平台写窗口

```go
func (t *draftCommitTransaction) releaseDraftTransaction()
```

**输入**

无

**输出**

无

**步骤**

1. 释放短事务持有的真实doc或创建生命guard及平台写窗口

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.ReleaseFence`

## content.Rollback

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：回滚尚未终结SQL并释放guard和写窗口，原回执命中也释放

```go
func (t *draftCommitTransaction) Rollback(ctx context.Context) error
```

**输入**

无

**输出**

无

**步骤**

1. 回滚尚未终结SQL并释放guard和写窗口，原回执命中也释放

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：`content.releaseDraftTransaction`

## content.validateDraftOperationPlan

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：核持久固定材料之间的身份、CAS与完整引用摘要关系

```go
func validateDraftOperationPlan(action string,plan draftOperationPlan) error
```

**输入**

无

**输出**

无

**步骤**

1. 核持久固定材料之间的身份、CAS与完整引用摘要关系

**错误**

- 无

- 副作用：无
- 授权：只在已认证本域创建/保存/读取调用链或受信运行装配内使用；私有持久身份和COMMITTED资格失败关闭
- 并发：文件与跨服务IO不持SQL/doc guard；写窗口在doc mutex前，真实保存guard只覆盖短SQL事务
- 幂等：完整scope与规范摘要复用首次固定材料和原回执；未知提交不转abort，COMMITTED优先
- 依赖：无

## content.readDraftOperationFrom

- 级别：参考
- 文件：`modules/content/internal/draft_commit_store.go`
- 作用：在指定真实SQL快照读取完整草稿原意图、当前pin及COMMITTED回执资格

```go
func readDraftOperationFrom(ctx context.Context,query draftOperationQuery,op OperationContext,action string,digest Hash) (draftOperation,bool,error)
```

**输入**

无

**输出**

无

**步骤**

1. 按scope/动作/规范摘要核当前caller与actor，解码固定计划及最新state/pin/result
2. 若COMMITTED，同一查询句柄核真实提交资格和完整原结果，不复用较早pending快照的nil pin或原结果

**错误**

- 无

- 副作用：无
- 授权：本域已授权草稿读取/封存链，持久scope与原结果失败关闭
- 并发：可传私有DB或当前短SQL事务；无RPC或文件IO
- 幂等：回放同操作真实当前持久记录，COMMITTED优先且保留原pin
- 依赖：`content.decodeStorageJSON`、`content.validateDraftOperationPlan`、`content.sameStoredDraft`、`content.contentStorageError`
