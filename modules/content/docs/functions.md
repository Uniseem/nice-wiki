# 函数填空合同

每个条目与 [implementation.json](../contracts/implementation.json) 一一对应。签名中的领域类型见 types.json；标准 Problem 描述返回 error。未来新增任何私有助手也必须先登记契约。

## content.CreateDocument

文件：`modules/content/internal/document_service.go`。

`func CreateDocument(ctx RequestContext, input CreateDocumentInput) (CreateDocumentResult, error)`

新建共享草稿文档。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CreateDocumentInput。新建共享草稿文档的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CreateDocumentResult。新建共享草稿文档的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 检查content.edit、parent非trash和treeRevision，site-config取基准语言。
2. 生成docId/空Tiptap doc与fractional order；WriteRevisionFile。
3. 事务建tree metadata/shared draft head/ID ledger/COMMITTED journal；未发草稿不可公开。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config:readSiteConfig；content.WriteRevisionFile；content.CommitDraftJournal。

## content.ReadDocumentTree

文件：`modules/content/internal/tree_service.go`。

`func ReadDocumentTree(ctx RequestContext, input ReadDocumentTreeInput) (ReadDocumentTreeResult, error)`

后台读取整站文档树。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadDocumentTreeInput。后台读取整站文档树的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadDocumentTreeResult。后台读取整站文档树的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 检查content.read。
2. 按parent/orderKey构建一致前序，保留draft徽标，回收站独立读取。
3. 验证无环；损坏不输出部分伪树。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：content.read。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content.EnsureTreeAcyclic。

## content.ListDocuments

文件：`modules/content/internal/tree_service.go`。

`func ListDocuments(ctx RequestContext, input ListDocumentsInput) (ListDocumentsResult, error)`

文档及回收站分页列表。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ListDocumentsInput。文档及回收站分页列表的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ListDocumentsResult。文档及回收站分页列表的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 按功能权限content.read或publish读取相应状态。
2. 签名cursor绑定site/state/tree watermark。
3. 不在列表返回正文。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：content.read；trash需content.publish。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## content.MoveDocument

文件：`modules/content/internal/tree_service.go`。

`func MoveDocument(ctx RequestContext, input MoveDocumentInput) (MoveDocumentResult, error)`

拖拽移动层级和顺序。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：MoveDocumentInput。拖拽移动层级和顺序的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：MoveDocumentResult。拖拽移动层级和顺序的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 锁tree水位，EnsureTreeAcyclic防移入自己后代、检查同父slug唯一。
2. PlanOldPathRedirects捕获已公开节点及全部后代旧路径→docId。
3. 事务更新parent/order/treeRevision/redirect manifest并outbox，链接因docId不失效。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope。

**并发**：全站树事务CAS，移动和发布路径规划同一tree锁。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.EnsureTreeAcyclic；content.PlanOldPathRedirects。

## content.ReorderChildren

文件：`modules/content/internal/tree_service.go`。

`func ReorderChildren(ctx RequestContext, input ReorderChildrenInput) (ReorderChildrenResult, error)`

拖拽调整同级顺序。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReorderChildrenInput。拖拽调整同级顺序的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReorderChildrenResult。拖拽调整同级顺序的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核完整集合与parent关系，拒绝缺失/重复/跨层。
2. fracdex生成顺序，不改变slug。
3. 事务CAS treeRevision，前后篇按新树排序。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## content.EnsureTreeAcyclic

文件：`modules/content/internal/tree_validator.go`。

`func EnsureTreeAcyclic(input EnsureTreeAcyclicInput) (EnsureTreeAcyclicResult, error)`

检查树无环和层级约束。

**输入**

- input：EnsureTreeAcyclicInput。检查树无环和层级约束的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：EnsureTreeAcyclicResult。检查树无环和层级约束的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 索引ID/parent，缺父、重复ID拒绝。
2. DFS颜色判环，最长层级32。
3. movingId不得等于parent或在其后代内。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.UpdateDocumentMetadata

文件：`modules/content/internal/metadata_service.go`。

`func UpdateDocumentMetadata(ctx RequestContext, input UpdateDocumentMetadataInput) (UpdateDocumentMetadataResult, error)`

修改标题/路径/SEO描述/分享图。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：UpdateDocumentMetadataInput。修改标题/路径/SEO描述/分享图的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：UpdateDocumentMetadataResult。修改标题/路径/SEO描述/分享图的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. content.edit；人工SEO标manual且进后续翻译，OG资源需ready。
2. ValidatePathSegment同父唯一，已公开path更改PlanOldPathRedirects覆盖后代。
3. metadata CAS与tree路径映射原子更新；不自动把共享draft发给访客。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：按D35功能：content.edit编辑，content.publish发布/撤回/回收站恢复/回滚；全站功能无文档scope。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.ValidatePathSegment；content.PlanOldPathRedirects；media:readAsset。

## content.ValidatePathSegment

文件：`modules/content/internal/path_service.go`。

`func ValidatePathSegment(ctx RequestContext, input ValidatePathSegmentInput) (ValidatePathSegmentResult, error)`

校验英文可读路径与同父唯一。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidatePathSegmentInput。校验英文可读路径与同父唯一的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidatePathSegmentResult。校验英文可读路径与同父唯一的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 正则与保留路径名检查，禁止点段/编码斜杠。
2. 查询同父活动节点排除自己。
3. 冲突CONFLICT，历史redirect不能覆写活跃目标。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：content.edit或publish私有流程。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## content.PlanOldPathRedirects

文件：`modules/content/internal/path_service.go`。

`func PlanOldPathRedirects(input PlanOldPathRedirectsInput) (PlanOldPathRedirectsResult, error)`

规划301旧路径→文档ID。

**输入**

- input：PlanOldPathRedirectsInput。规划301旧路径→文档ID的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PlanOldPathRedirectsResult。规划301旧路径→文档ID的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 前序取已公开节点及后代完整旧path。
2. 不含语言前缀，因为所有语言共slug。
3. 记录docId而不链式URL；公开解析当前路径避免跳转环。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.AcquireEditLock

文件：`modules/content/internal/edit_lock_service.go`。

`func AcquireEditLock(ctx RequestContext, input AcquireEditLockInput) (AcquireEditLockResult, error)`

打开文档自动获得内存编辑锁。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：AcquireEditLockInput。打开文档自动获得内存编辑锁的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：AcquireEditLockResult。打开文档自动获得内存编辑锁的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；NOT_FOUND: 资源不存在或在当前主体作用域中不可见。

**步骤**

1. content.edit与doc状态校验；单active content leader管理锁。
2. 空/过期锁分配bootId+单调fence，期限2分钟；别人持锁返回LOCKED与safe holderName只读。
3. 同账户不同tab可共享账户锁，不能绕过draftVersion CAS。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false。

**副作用**：本进程锁表。

**授权**：content.edit。

**并发**：docId内存mutex，content只有单写leader。

**幂等**：同account/doc有效lease复用，不换fence。

**依赖**：无跨函数/跨域依赖。

## content.RenewEditLock

文件：`modules/content/internal/edit_lock_service.go`。

`func RenewEditLock(ctx RequestContext, input RenewEditLockInput) (RenewEditLockResult, error)`

心跳维持锁并处理长时间无输入。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RenewEditLockInput。心跳维持锁并处理长时间无输入的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RenewEditLockResult。心跳维持锁并处理长时间无输入的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. 核holder/lease/boot/fence当前且未到期。
2. 心跳延长2分钟，但15分钟无输入触发提醒，宽限2分钟仍无输入释放为只读。
3. 忽略客户端未来lastInputAt，服务依据输入序号/可信接收时间更新。

**错误**：LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：内存lease时间。

**授权**：content.edit。

**并发**：doc mutex。

**幂等**：心跳序号单调，旧心跳不延长已释放锁。

**依赖**：无跨函数/跨域依赖。

## content.ReleaseEditLock

文件：`modules/content/internal/edit_lock_service.go`。

`func ReleaseEditLock(ctx RequestContext, input ReleaseEditLockInput) (ReleaseEditLockResult, error)`

关闭页面释放自身锁。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReleaseEditLockInput。关闭页面释放自身锁的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReleaseEditLockResult。关闭页面释放自身锁的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配。

**步骤**

1. 核lease归属与boot。
2. 锁仍匹配则原子删除；缺失视已释放。
3. 发编辑状态元数据，关闭beacon失败2分钟TTL兜底。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false。

**副作用**：内存锁删除。

**授权**：content.edit自身。

**并发**：doc mutex。

**幂等**：重复release无效果。

**依赖**：无跨函数/跨域依赖。

## content.ForceReleaseEditLock

文件：`modules/content/internal/edit_lock_service.go`。

`func ForceReleaseEditLock(ctx RequestContext, input ForceReleaseEditLockInput) (ForceReleaseEditLockResult, error)`

只有超管强制释放别人锁。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ForceReleaseEditLockInput。只有超管强制释放别人锁的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ForceReleaseEditLockResult。只有超管强制释放别人锁的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变。

**步骤**

1. superadmin专属，不可分配给子账号。
2. 同doc mutex核expectedLease防释放已换人锁，增加fence并删锁。
3. 发强制只读事件；已自动保存草稿保留，旧fence所有后续save拒绝。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false。

**副作用**：内存fence/锁与审计outbox。

**授权**：superadmin。

**并发**：与save登记同doc mutex。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## content.ExpireIdleLocks

文件：`modules/content/internal/edit_lock_service.go`。

`func ExpireIdleLocks(ctx RequestContext, input ExpireIdleLocksInput) (ExpireIdleLocksResult, error)`

释放失联或提示后仍空闲锁。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ExpireIdleLocksInput。释放失联或提示后仍空闲锁的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ExpireIdleLocksResult。释放失联或提示后仍空闲锁的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 逐doc mutex检查now≥expiresAt或15分钟提示+2分钟无输入。
2. 释放提高fence，标事件给在线编辑器。
3. 进程重启所有锁清空，bootId改变拒绝旧请求。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：锁表与元数据事件。

**授权**：service:content内存maintenance。

**并发**：逐docmutex避免全局阻塞。

**幂等**：已释放不重复事件。

**依赖**：无跨函数/跨域依赖。

## content.ReadDraft

文件：`modules/content/internal/draft_service.go`。

`func ReadDraft(ctx RequestContext, input ReadDraftInput) (ReadDraftResult, error)`

读取全篇共享草稿及编辑状态。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadDraftInput。读取全篇共享草稿及编辑状态的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadDraftResult。读取全篇共享草稿及编辑状态的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. content.read，同站点非purged。
2. COMMITTED head→ReadRevisionBody校hash。
3. 返回draftVersion/lease供editor只读或可写决定；无访客入口。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：content.read。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content.ReadRevisionBody。

## content.SaveDraft

文件：`modules/content/internal/draft_service.go`。

`func SaveDraft(ctx RequestContext, input SaveDraftInput) (SaveDraftResult, error)`

自动保存完整Tiptap正文。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：SaveDraftInput。自动保存完整Tiptap正文的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：SaveDraftResult。自动保存完整Tiptap正文的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. doc mutex检查有效holder/boot/fence/lease，ctx授权与epoch；ValidateBlockTree/IDs。
2. CollectResourceReferences先media acquire pins，再WriteRevisionFile不可变fsync。
3. 同私有txn再检查lease/fence和expectedHead/version，CommitDraftJournal+refs+receipt/outbox；commit后confirm pins，失败release且隔离未提交文件。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：content.edit且当前有效锁。

**并发**：save登记与锁强释同docmutex；持久head CAS避免同人双tab覆盖；不能持mutex跨无限网络。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.ValidateBlockTree；content.CollectResourceReferences；content.WriteRevisionFile；content.CommitDraftJournal；media:acquireResourceReferences；media:confirmResourceReferences；media:releaseResourceReferences。

## content.ValidateBlockTree

文件：`modules/content/internal/block_validator.go`。

`func ValidateBlockTree(ctx RequestContext, input ValidateBlockTreeInput) (ValidateBlockTreeResult, error)`

验证全部Tiptap块及稳定ID。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidateBlockTreeInput。验证全部Tiptap块及稳定ID的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateBlockTreeResult。验证全部Tiptap块及稳定ID的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. schemaVersion逐级已支持；strict types拒绝unknown node/attrs/mark与HTML。
2. 全树含list/cell/column IDs唯一，深度≤32/块≤10000，总JSON≤站点限；删除ID不可被另一内容重用。
3. ValidateInline/TableGrid/Columns；已有块保ID，复制新ID，移动不换ID；不增加block引用/图谱/实时协作。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：私有正文验证。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content.ValidateInline；content.ValidateTableGrid；content.ValidateColumns。

## content.ValidateInline

文件：`modules/content/internal/block_validator.go`。

`func ValidateInline(input ValidateInlineInput) (ValidateInlineResult, error)`

验证全部行内格式与链接。

**输入**

- input：ValidateInlineInput。验证全部行内格式与链接的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateInlineResult。验证全部行内格式与链接的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性。

**步骤**

1. text非空且无NUL，marks类型至多一次，code正文不作为HTML。
2. link内部docId/blockId或http/https/mailto，禁javascript和userinfo。
3. inlineMath latex大小有界，不含文件/网络宏；代码/公式与地址不交翻译模型。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.ValidateTableGrid

文件：`modules/content/internal/table_validator.go`。

`func ValidateTableGrid(input ValidateTableGridInput) (ValidateTableGridResult, error)`

验证表格合并单元格覆盖。

**输入**

- input：ValidateTableGridInput。验证表格合并单元格覆盖的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateTableGridResult。验证表格合并单元格覆盖的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 逐行以rowspan已占位和colspan放origin cells。
2. 覆盖矩阵不得重叠/缺洞/越200×30，colwidth长度等colspan。
3. header单元格类型保留，所有cell ID唯一，内文递归block校验。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.ValidateColumns

文件：`modules/content/internal/columns_validator.go`。

`func ValidateColumns(input ValidateColumnsInput) (ValidateColumnsResult, error)`

校验2–4栏比例和禁止嵌套。

**输入**

- input：ValidateColumnsInput。校验2–4栏比例和禁止嵌套的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidateColumnsResult。校验2–4栏比例和禁止嵌套的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值。

**步骤**

1. content数量2–4与widthRatios长度一致。
2. 比例正且总和1（误差≤1e-6），columnId唯一。
3. 递归所有后代不得出现columns；窄屏上下排列按列顺序文本展开。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.CollectResourceReferences

文件：`modules/content/internal/reference_extractor.go`。

`func CollectResourceReferences(input CollectResourceReferencesInput) (CollectResourceReferencesResult, error)`

扫描所有嵌套块资源引用。

**输入**

- input：CollectResourceReferencesInput。扫描所有嵌套块资源引用的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CollectResourceReferencesResult。扫描所有嵌套块资源引用的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 规范前序遍历image/attachment/drawing(scene+亮暗svg)/video及poster。
2. 保assetId/version/hash和blockId，不能只扫顶层或当前公开。
3. 去重sourceKey，当前/历史明确分类以支持手动清理警告。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.CanonicalizeBody

文件：`modules/content/internal/body_canonicalizer.go`。

`func CanonicalizeBody(input CanonicalizeBodyInput) (CanonicalizeBodyResult, error)`

规范序列化正文并计算hash。

**输入**

- input：CanonicalizeBodyInput。规范序列化正文并计算hash的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CanonicalizeBodyResult。规范序列化正文并计算hash的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性。

**步骤**

1. 按schema键排序，不改Unicode文字/代码/latex。
2. 数组顺序保留，marks按固定类型排序而不变语义。
3. UTF8 canonicalJSON SHA256作为文件真相hash。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.HashBlock

文件：`modules/content/internal/body_canonicalizer.go`。

`func HashBlock(input HashBlockInput) (HashBlockResult, error)`

计算翻译/AI引用稳定块hash。

**输入**

- input：HashBlockInput。计算翻译/AI引用稳定块hash的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：HashBlockResult。计算翻译/AI引用稳定块hash的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性。

**步骤**

1. 包含全部可呈现attrs、内容、不可变assetrefs和嵌套子内容。
2. 排除运行锁/编辑UI句柄，保持blockId一致。
3. 规范编码SHA256；修改影响hash，不以时间替代。

**错误**：UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false。

**副作用**：无。

**授权**：纯函数。

**并发**：纯函数，输入视为不可变；可以并发调用。

**幂等**：确定性输入产生确定性输出；无持久副作用。

**依赖**：无跨函数/跨域依赖。

## content.WriteRevisionFile

文件：`modules/content/internal/revision_files.go`。

`func WriteRevisionFile(ctx RequestContext, input WriteRevisionFileInput) (WriteRevisionFileResult, error)`

持久写权威正文文件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：WriteRevisionFileInput。持久写权威正文文件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：WriteRevisionFileResult。持久写权威正文文件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. CanonicalizeBody，服务生成site/doc/revision.json私有路径。
2. 登记PREPAREDjournal，tmp写fsync→同FSrename→fsyncparent并核hash。
3. 同revision异hash拒绝；文件成功不代表head提交，失败不暴露PREPARED。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：正文文件/PREPAREDjournal。

**授权**：私有正文提交。

**并发**：revisionId唯一文件永不覆盖。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.CanonicalizeBody。

## content.RecoverPreparedCommits

文件：`modules/content/internal/revision_files.go`。

`func RecoverPreparedCommits(ctx RequestContext, input RecoverPreparedCommitsInput) (RecoverPreparedCommitsResult, error)`

恢复崩溃前正文提交日志。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RecoverPreparedCommitsInput。恢复崩溃前正文提交日志的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RecoverPreparedCommitsResult。恢复崩溃前正文提交日志的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. exclusive启动或safe屏障内扫描journal。
2. COMMITTED查file/hash/head；缺失隔离doc并关闭公开。
3. PREPARED若无已提交receipt则隔离，不能自行推进head；保留窗口后仅受控清未见孤儿。

**错误**：INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：日志恢复/孤儿隔离。

**授权**：service:content启动控制。

**并发**：exclusive gate。

**幂等**：重复恢复不发布PREPARED。

**依赖**：无跨函数/跨域依赖。

## content.CommitDraftJournal

文件：`modules/content/internal/revision_repository.go`。

`func CommitDraftJournal(ctx RequestContext, input CommitDraftJournalInput) (CommitDraftJournalResult, error)`

原子提交共享草稿head。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CommitDraftJournalInput。原子提交共享草稿head的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CommitDraftJournalResult。原子提交共享草稿head的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 核PREPARED文件hash/fsync与有效fence登记。
2. 同事务CAS head/version、append稳定IDledger、journal COMMITTED、refs/receipt/outbox。
3. CAS失败正文未可见；已commitreceipt返原revision，不能网络失败重复改head。

**错误**：VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：私有SaveDraft/Create事务。

**并发**：doc rowCAS + fence检查。

**幂等**：operationId+canonicaldigest去重。

**依赖**：无跨函数/跨域依赖。

## content.ReadRevisionBody

文件：`modules/content/internal/revision_reader.go`。

`func ReadRevisionBody(ctx RequestContext, input ReadRevisionBodyInput) (ReadRevisionBodyResult, error)`

读取COMMITTED正文并核真相。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadRevisionBodyInput。读取COMMITTED正文并核真相的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadRevisionBodyResult。读取COMMITTED正文并核真相的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；UNSUPPORTED_SCHEMA: 版本或块 schema 不受支持；禁止静默丢弃未知属性。

**步骤**

1. 根据私有journal索引解析服务生成路径，不接受OSpath。
2. COMMITTED且site/doc/version归属匹配。
3. 流读取bounded JSON、sha/schema核对；失败INTEGRITY隔离无fallback。

**错误**：NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；UNSUPPORTED_SCHEMA：版本或块 schema 不受支持；禁止静默丢弃未知属性；retryable=false。

**副作用**：无。

**授权**：调用方先做content.read或已验证publication/preview。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## content.PreparePublicationMetadata

文件：`modules/content/internal/publication_metadata.go`。

`func PreparePublicationMetadata(ctx RequestContext, input PreparePublicationMetadataInput) (PreparePublicationMetadataResult, error)`

人工优先准备首次slug和SEO。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PreparePublicationMetadataInput。人工优先准备首次slug和SEO的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PreparePublicationMetadataResult。人工优先准备首次slug和SEO的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 人工slug/SEO优先且不被模型覆盖；已有英文slug保持。
2. 首次空slug仅在content.slug已配置时经model-gateway生成并ValidatePathSegment；无模型提示补填字段，不造doc-ID/fake结果。
3. SEO未人工设且任务可用时content.seo生成；无模型仅提示缺项/人工填写，不能以AI缺项阻塞已提供合法元数据的基础发布。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：可选真实模型调用未来契约；本轮禁止执行。

**授权**：content.publish。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：父operationId子任务幂等；manual字段不覆写。

**依赖**：content.ValidatePathSegment；model-gateway:createInference。

## content.PublishDocument

文件：`modules/content/internal/publication_service.go`。

`func PublishDocument(ctx RequestContext, input PublishDocumentInput) (PublishDocumentResult, error)`

点击发布才公开精确共享草稿版本。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PublishDocumentInput。点击发布才公开精确共享草稿版本的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PublishDocumentResult。点击发布才公开精确共享草稿版本的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 授权publish+epoch，读COMMITTED指定draft/version；PreparePublicationMetadata人工优先，resources当前ready引用pin。
2. 再次CAS metadata/draft/tree水位，CommitPublication原子选immutable发布ref、visibilityRevision/history/作者/outbox。
3. 提交后render/index/translation/data git镜像异步由jobs触发；无AI仍基础公开，optional失败不回退已发布正文。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：content.publish。

**并发**：doc/treeCAS持久事务，外部计算在事务外以源hash回验。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.ReadRevisionBody；content.PreparePublicationMetadata；content.CommitPublication；media:acquireResourceReferences；media:confirmResourceReferences。

## content.WithdrawDocument

文件：`modules/content/internal/publication_service.go`。

`func WithdrawDocument(ctx RequestContext, input WithdrawDocumentInput) (WithdrawDocumentResult, error)`

撤回后回草稿并让公开页面下线。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：WithdrawDocumentInput。撤回后回草稿并让公开页面下线的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：WithdrawDocumentResult。撤回后回草稿并让公开页面下线的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. publish授权核doc非trash。
2. 事务清publishedReference、state=draft、增visibilityRevision，保留共享draft和历史。
3. 同步validatePublications立即false；outbox通知所有缓存，延迟/乱序不能复活。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：content.publish。

**并发**：与publish/trash同docCAS。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.PublishContentEvent。

## content.CommitPublication

文件：`modules/content/internal/publication_repository.go`。

`func CommitPublication(ctx RequestContext, input CommitPublicationInput) (CommitPublicationResult, error)`

原子提交当前发布快照与署名历史。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CommitPublicationInput。原子提交当前发布快照与署名历史的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CommitPublicationResult。原子提交当前发布快照与署名历史的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 确认draft文件hash/ready资源pin并构造公共PublishedRevisionRef。
2. 事务CAS当前元数据，publishedReference换新、statepublished、增revision/visibility，writehistory作者账号和serverUTC。
3. 同事务发布PublicationChangedPayload+资源refs+data git镜像事件，事件未发送不影响权威公开判定。

**错误**：VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：私有publish/rollbacktxn。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.PublishContentEvent。

## content.ValidatePublications

文件：`modules/content/internal/visibility_service.go`。

`func ValidatePublications(ctx RequestContext, input ValidatePublicationsInput) (ValidatePublicationsResult, error)`

同步核公开引用是否仍可见且当前。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidatePublicationsInput。同步核公开引用是否仍可见且当前的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidatePublicationsResult。同步核公开引用是否仍可见且当前的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 仅注册public/knowledge/qa/localization调用方或公开不可枚举批次。
2. 一次一致txn比dataEpoch/doc状态/currentref/hash/schema与visibilityRevision。
3. unknown/notfound返回visible=false而不暴露draft；依赖不可用不能把旧缓存当有效。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：service已登记caller；请求refs只含曾公开对象。

**并发**：同txn快照，传输开始前再核最新可见水位。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## content.ReadPublishedRevision

文件：`modules/content/internal/publication_reader.go`。

`func ReadPublishedRevision(ctx RequestContext, input ReadPublishedRevisionInput) (ReadPublishedRevisionResult, error)`

获取仍公开的精确正文快照。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadPublishedRevisionInput。获取仍公开的精确正文快照的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadPublishedRevisionResult。获取仍公开的精确正文快照的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：NOT_FOUND: 资源不存在或在当前主体作用域中不可见；STALE_RESULT: 事件/计算针对已过期源版本，拒绝覆盖新状态；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 读current published ref并ValidatePublications。
2. 只允许当前可见精确ref，不因历史id给访客读撤回历史。
3. ReadRevisionBody核hash后传输前再查visibility，改变则STALE_RESULT。

**错误**：NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；STALE_RESULT：事件/计算针对已过期源版本，拒绝覆盖新状态；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：anonymous或已注册service；历史另走publisher history接口。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content.ValidatePublications；content.ReadRevisionBody。

## content.TrashDocument

文件：`modules/content/internal/trash_service.go`。

`func TrashDocument(ctx RequestContext, input TrashDocumentInput) (TrashDocumentResult, error)`

删除进回收站，可选旧URL替代跳转。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：TrashDocumentInput。删除进回收站，可选旧URL替代跳转的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：TrashDocumentResult。删除进回收站，可选旧URL替代跳转的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用。

**步骤**

1. publish授权；非includeSubtree而有子节点必须指定迁移方案否则CONFLICT。
2. 先快照所有旧path和公开状态，replacement必须当前可见且不在待删集合。
3. 同事务trash节点/清公开ref/增visibility，旧URL410或目标docIdredirect；正文历史保留。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：content.publish。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.PlanOldPathRedirects；content.ValidatePublications。

## content.RestoreDocument

文件：`modules/content/internal/trash_service.go`。

`func RestoreDocument(ctx RequestContext, input RestoreDocumentInput) (RestoreDocumentResult, error)`

从回收站恢复为草稿。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RestoreDocumentInput。从回收站恢复为草稿的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RestoreDocumentResult。从回收站恢复为草稿的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. publish权限、目标parent有效无环、slug同父唯一。
2. 历史资源存在性报告；文件清理风险不能静默掩盖。
3. CAS清trashedAt/state=draft不自动重新发布；旧tombstone按恢复路径规则更新。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：content.publish。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.ValidatePathSegment；content.EnsureTreeAcyclic；media:readAsset。

## content.PurgeDocuments

文件：`modules/content/internal/trash_service.go`。

`func PurgeDocuments(ctx RequestContext, input PurgeDocumentsInput) (PurgeDocumentsResult, error)`

只有超管彻底删除/清空回收站。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PurgeDocumentsInput。只有超管彻底删除/清空回收站的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PurgeDocumentsResult。只有超管彻底删除/清空回收站的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；CONFLICT: 状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. superadmin专属；核当前trash集合与confirmationHash，活动文档拒绝。
2. 先持久purge决定+墓碑，移除本域正文head/索引/引用并返回410。
3. 本域文件经决定日志删除；data git已提交历史由data-protection保留，不删别域目录/自动清媒体。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；CONFLICT：状态冲突、唯一约束冲突或 operationId 被不同请求摘要复用；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：本域显式永久清理与历史保留事件。

**授权**：superadmin。

**并发**：purge/tree互斥，资源引用版本原子提高。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：无跨函数/跨域依赖。

## content.ReadRevisionHistory

文件：`modules/content/internal/history_service.go`。

`func ReadRevisionHistory(ctx RequestContext, input ReadRevisionHistoryInput) (ReadRevisionHistoryResult, error)`

查看署名发布/git版本历史。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadRevisionHistoryInput。查看署名发布/git版本历史的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadRevisionHistoryResult。查看署名发布/git版本历史的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. publish功能权限。
2. 读取本域history + data git镜像回执，不读取DP目录。
3. 显示执行发布账号的署名与时间，即使账号deleted仍保留。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：content.publish。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## content.DiffRevisions

文件：`modules/content/internal/history_service.go`。

`func DiffRevisions(ctx RequestContext, input DiffRevisionsInput) (DiffRevisionsResult, error)`

按稳定块ID对比历史版本。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：DiffRevisionsInput。按稳定块ID对比历史版本的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：DiffRevisionsResult。按稳定块ID对比历史版本的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. publish权限读两个COMMITTED文件。
2. 按blockId比较hash与前序父位置。
3. 区分文本/属性变化和移动，不将ID变化误配历史。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：content.publish。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：确定性。

**依赖**：content.ReadRevisionBody；content.HashBlock。

## content.RollbackPublishedRevision

文件：`modules/content/internal/history_service.go`。

`func RollbackPublishedRevision(ctx RequestContext, input RollbackPublishedRevisionInput) (RollbackPublishedRevisionResult, error)`

回滚历史形成新发布/git提交。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：RollbackPublishedRevisionInput。回滚历史形成新发布/git提交的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：RollbackPublishedRevisionResult。回滚历史形成新发布/git提交的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. publish授权，ReadRevisionHistory核精确历史body/meta。
2. 核资源；手动清理后缺文件先提示，需要显式ack才保留缺资源占位而不伪造bytes。
3. 新revision恢复历史稿并走CommitPublication，保留旧history、执行回滚账号署名，重发render/index/translation。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：content.publish。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：content.ReadRevisionHistory；content.ReadRevisionBody；content.CommitPublication；media:readAsset。

## content.CreatePreviewGrant

文件：`modules/content/internal/preview_service.go`。

`func CreatePreviewGrant(ctx RequestContext, input CreatePreviewGrantInput) (CreatePreviewGrantResult, error)`

管理员预览草稿公开样式。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：CreatePreviewGrantInput。管理员预览草稿公开样式的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：CreatePreviewGrantResult。管理员预览草稿公开样式的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. content.read/编辑或发布已登录管理主体。
2. 确认稿COMMITTED且创建短期绑定account精确ref grant。
3. public-site按此grant渲染并noindex/no-store，不造匿名分享预览。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有短期preview digest。

**授权**：content.read，已登录管理员。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：同operationId返回同grant。

**依赖**：无跨函数/跨域依赖。

## content.ValidatePreviewGrant

文件：`modules/content/internal/preview_service.go`。

`func ValidatePreviewGrant(ctx RequestContext, input ValidatePreviewGrantInput) (ValidatePreviewGrantResult, error)`

只向对应管理员验证草稿预览。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ValidatePreviewGrantInput。只向对应管理员验证草稿预览的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ValidatePreviewGrantResult。只向对应管理员验证草稿预览的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：UNAUTHENTICATED: 会话、委托或服务证明缺失、失效或被撤销；FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. service:public-site核token digest/expiry/currentactor。
2. 绑定doc/draft/account/epoch，退出或停用立即失效。
3. ReadRevisionBody精确稿，禁止preview token调用其他业务或公开cache。

**错误**：UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销；retryable=false；FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：service:public-site + 对应管理员session。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：identity:checkAuthorization；content.ReadRevisionBody。

## content.ListResourceReferences

文件：`modules/content/internal/reference_reader.go`。

`func ListResourceReferences(ctx RequestContext, input ListResourceReferencesInput) (ListResourceReferencesResult, error)`

向资源库列出哪些文档引用文件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ListResourceReferencesInput。向资源库列出哪些文档引用文件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ListResourceReferencesResult。向资源库列出哪些文档引用文件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. media注册service或content.read检查。
2. 一致水位全量/分页列current_draft/current_published/history。
3. unknown/incomplete不得解释为空引用，返回水位/完成cursor。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：service:media/site-config/data-protection 或content.read。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## content.ReadContentObject

文件：`modules/content/internal/object_reader.go`。

`func ReadContentObject(ctx RequestContext, input ReadContentObjectInput) (ReadContentObjectResult, error)`

经对象API读取私有文件而不共享目录。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadContentObjectInput。经对象API读取私有文件而不共享目录的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadContentObjectResult。经对象API读取私有文件而不共享目录的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：FORBIDDEN: 主体没有所需操作权限，或站点/资源作用域不匹配；NOT_FOUND: 资源不存在或在当前主体作用域中不可见；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 注册service核owner/content与purpose。
2. 索引解析服务生成path，version/hash一致。
3. 短期reader lease提供只读流，媒体或DP不得访问OSroot。

**错误**：FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配；retryable=false；NOT_FOUND：资源不存在或在当前主体作用域中不可见；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：短期reader lease。

**授权**：service:data-protection/localization/knowledge；正文先验证publish或管理委托。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：无跨函数/跨域依赖。

## content.PublishContentEvent

文件：`modules/content/internal/outbox_repository.go`。

`func PublishContentEvent(ctx RequestContext, input PublishContentEventInput) (PublishContentEventResult, error)`

与发布/撤回提交原子登记最小事件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：PublishContentEventInput。与发布/撤回提交原子登记最小事件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：PublishContentEventResult。与发布/撤回提交原子登记最小事件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 必须且仅有一payload，禁止正文/密钥。
2. 同business事务eventId唯一outbox。
3. jobs at-least-once ack后标发；乱序由source revision拒绝。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：私有事务/正文提交日志/operation receipt/outbox。

**授权**：私有contenttxn。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：eventId唯一。

**依赖**：jobs:publishEvent。

## content.AcquireBarrier

文件：`modules/content/internal/barrier_service.go`。

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

**授权**：service:data-protection + 备份superadmin委托/恢复专用RecoveryAuthorization，严格核restore/barrier/epoch/audience/action，独立本地信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## content.ReleaseBarrier

文件：`modules/content/internal/barrier_service.go`。

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

**授权**：service:data-protection + 备份superadmin委托/恢复专用RecoveryAuthorization，严格核restore/barrier/epoch/audience/action，独立本地信任锚不被快照覆盖。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：写请求 operationId+规范请求摘要为唯一键；同键同摘要返回已存结果，同键异摘要 CONFLICT。

**依赖**：site-config:readRuntimeState。

## content.ExportSnapshot

文件：`modules/content/internal/snapshot_service.go`。

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

**授权**：service:data-protection + 备份superadmin委托/恢复专用RecoveryAuthorization，严格核restore/barrier/epoch/audience/action，独立本地信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## content.PrepareSnapshot

文件：`modules/content/internal/snapshot_service.go`。

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

**授权**：service:data-protection + 备份superadmin委托/恢复专用RecoveryAuthorization，严格核restore/barrier/epoch/audience/action，独立本地信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## content.ValidateSnapshot

文件：`modules/content/internal/snapshot_validator.go`。

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

**授权**：service:data-protection + 备份superadmin委托/恢复专用RecoveryAuthorization，严格核restore/barrier/epoch/audience/action，独立本地信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## content.CommitSnapshot

文件：`modules/content/internal/snapshot_repository.go`。

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

**授权**：service:data-protection + 备份superadmin委托/恢复专用RecoveryAuthorization，严格核restore/barrier/epoch/audience/action，独立本地信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## content.RollbackSnapshot

文件：`modules/content/internal/snapshot_repository.go`。

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

**授权**：service:data-protection + 备份superadmin委托/恢复专用RecoveryAuthorization，严格核restore/barrier/epoch/audience/action，独立本地信任锚不被快照覆盖。

**并发**：屏障与恢复按 barrierId/stageId 串行；活动事务计数达到零后才 safe；恢复通过原子 manifest 切换。

**幂等**：operationId/backupId/restoreId/stageId+摘要去重；重复返回已记录阶段，不能重新激活。

**依赖**：site-config:readRuntimeState。

## content.ReadPublicTree

文件：`modules/content/internal/public_route_reader.go`。

`func ReadPublicTree(ctx RequestContext, input ReadPublicTreeInput) (ReadPublicTreeResult, error)`

公开导航读取已发布文档树。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ReadPublicTreeInput。公开导航读取已发布文档树的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ReadPublicTreeResult。公开导航读取已发布文档树的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许；INTEGRITY_ERROR: 哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布。

**步骤**

1. 过滤所有非published/visibility不可见。
2. 按全站树前序输出公开DTO，未公开父不泄漏其标题，仍发布子按公开导航规则接最近公开祖先。
3. 输出统一水位；公开previous/next依据当前语言可用文档映射。

**错误**：DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true；INTEGRITY_ERROR：哈希、引用、覆盖网格或提交日志不一致；隔离并禁止发布；retryable=false。

**副作用**：无。

**授权**：anonymous/service:public-site/knowledge。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content.ValidatePublications。

## content.ResolvePublicPath

文件：`modules/content/internal/public_route_reader.go`。

`func ResolvePublicPath(ctx RequestContext, input ResolvePublicPathInput) (ResolvePublicPathResult, error)`

解析当前路径、旧路径301与删除410。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：ResolvePublicPathInput。解析当前路径、旧路径301与删除410的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：ResolvePublicPathResult。解析当前路径、旧路径301与删除410的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 规范path禁点段/重复编码。
2. 优先活动公开path；tombstone oldpath→docId解析当前canonical，target已删则410而非环。
3. found必须ValidatePublications当前；全语言共用slug，返回状态让public-site输出相应HTTP。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：无。

**授权**：anonymous/service:public-site。

**并发**：服务私有事务；检查 expectedRevision 或单调版本；恢复屏障期间拒绝业务写入。

**幂等**：只读。

**依赖**：content.ValidatePublications。

## content.createEditorSession

文件：`modules/content/web/src/editor/editor-runtime.ts`。

`createEditorSession(ctx: RequestContext, input: createEditorSessionInput): Promise<createEditorSessionResult>`

创建Tiptap3/Vue编辑器会话。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：createEditorSessionInput。创建Tiptap3/Vue编辑器会话的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：createEditorSessionResult。创建Tiptap3/Vue编辑器会话的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 按16类扩展和稳定ID建立editor，未知schema只读提示。
2. 装入共享draftVersion与lease，别人持锁只读显示holder。
3. 基础编辑/预览/发布不因AI缺项禁用。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.applyBlockAction

文件：`modules/content/web/src/editor/block-actions.ts`。

`applyBlockAction(ctx: RequestContext, input: applyBlockActionInput): Promise<applyBlockActionResult>`

拖拽手柄菜单排序/转换/复制/删除。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：applyBlockActionInput。拖拽手柄菜单排序/转换/复制/删除的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：applyBlockActionResult。拖拽手柄菜单排序/转换/复制/删除的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 拖动保持原ID，复制产生新ID，删除ID标记不复用。
2. 转换保留语义与被允许attrs，无法无损转换先呈现确认差异。
3. Tiptap事务更新dirty，不做块引用/反向链接/图谱。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.resolveSlashMenu

文件：`modules/content/web/src/editor/slash-menu.ts`。

`resolveSlashMenu(ctx: RequestContext, input: resolveSlashMenuInput): Promise<resolveSlashMenuResult>`

slash插入菜单与Markdown输入快捷键。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：resolveSlashMenuInput。slash插入菜单与Markdown输入快捷键的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：resolveSlashMenuResult。slash插入菜单与Markdown输入快捷键的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. /显示16类与两/三栏插入。
2. ##到H2、-/1.列表、代码围栏规则受控转换。
3. 选中文字BubbleMenu仅已确认8项行内能力。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.normalizePastedContent

文件：`modules/content/web/src/editor/paste-adapter.ts`。

`normalizePastedContent(ctx: RequestContext, input: normalizePastedContentInput): Promise<normalizePastedContentResult>`

Word/网页/Markdown粘贴清理并转换块。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：normalizePastedContentInput。Word/网页/Markdown粘贴清理并转换块的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：normalizePastedContentResult。Word/网页/Markdown粘贴清理并转换块的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. DOM白名单转Tiptap不执行script/style事件/远程主动内容。
2. 将常见段落标题列表表格代码转换稳定新块ID。
3. 无法保留格式降为文字并提示；不是D30排除的批量导入功能。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.scheduleAutosave

文件：`modules/content/web/src/editor/autosave-controller.ts`。

`scheduleAutosave(ctx: RequestContext, input: scheduleAutosaveInput): Promise<scheduleAutosaveResult>`

去抖自动保存共享草稿。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：scheduleAutosaveInput。去抖自动保存共享草稿的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：scheduleAutosaveResult。去抖自动保存共享草稿的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 文本变更500ms去抖、最长2秒触发saveDraft，保持pending operationId。
2. 串行保存拿新head/version，后一请求不覆盖未经确认的服务器版本。
3. VERSION_MISMATCH保留本地未存差异并只读提示，禁止静默last-write-wins。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content:saveDraft。

## content.maintainEditHeartbeat

文件：`modules/content/web/src/editor/lock-client.ts`。

`maintainEditHeartbeat(ctx: RequestContext, input: maintainEditHeartbeatInput): Promise<maintainEditHeartbeatResult>`

每30秒锁心跳与15分钟空闲提醒。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：maintainEditHeartbeatInput。每30秒锁心跳与15分钟空闲提醒的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：maintainEditHeartbeatResult。每30秒锁心跳与15分钟空闲提醒的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 有效锁每30秒renew且传输入观测。
2. 15分钟无输入提示，宽限2分钟无输入后只读释放。
3. 关闭release beacon；网络失联最多2分钟后服务释放，旧boot/fence不能保存。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content:renewEditLock。

## content.applyForcedReadOnly

文件：`modules/content/web/src/editor/lock-client.ts`。

`applyForcedReadOnly(ctx: RequestContext, input: applyForcedReadOnlyInput): Promise<applyForcedReadOnlyResult>`

强释/失锁后立刻只读。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：applyForcedReadOnlyInput。强释/失锁后立刻只读的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：applyForcedReadOnlyResult。强释/失锁后立刻只读的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 比较fence/boot判定失锁。
2. 停autosave与编辑输入，保留已自动保存稿及未存本地差异。
3. 提示管理员谁释放/如何重获锁，不自动丢弃或覆盖服务器稿。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.selectInternalLink

文件：`modules/content/web/src/editor/internal-link-picker.ts`。

`selectInternalLink(ctx: RequestContext, input: selectInternalLinkInput): Promise<selectInternalLinkResult>`

插入文档选择器生成ID站内链接。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：selectInternalLinkInput。插入文档选择器生成ID站内链接的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：selectInternalLinkResult。插入文档选择器生成ID站内链接的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 从当前后台tree选择docId。
2. 只存docId/blockId，不存当前URL/语言。
3. 公开渲染按当前语言转路由。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.prepareImageUploads

文件：`modules/content/web/src/editor/image-uploader.ts`。

`prepareImageUploads(ctx: RequestContext, input: prepareImageUploadsInput): Promise<prepareImageUploadsResult>`

浏览器生成多宽WebP连原图上传。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：prepareImageUploadsInput。浏览器生成多宽WebP连原图上传的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：prepareImageUploadsResult。浏览器生成多宽WebP连原图上传的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 识别动图GIF/动态WebP/AVIF则只保原图不压缩。
2. 静图browser-image-compression生成640/1280/1920不放大原尺寸WebP。
3. 每文件SHA内容寻址独立上传并关联variants，服务端不图像处理。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.openDrawingEditor

文件：`modules/content/web/src/editor/drawing-wrapper.ts`。

`openDrawingEditor(ctx: RequestContext, input: openDrawingEditorInput): Promise<openDrawingEditorResult>`

Vue包装Excalidraw并按需加载。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：openDrawingEditorInput。Vue包装Excalidraw并按需加载的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：openDrawingEditorResult。Vue包装Excalidraw并按需加载的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 点击预览才lazy load React+Excalidraw MIT。
2. 获取已保存.excalidraw场景恢复全屏编辑。
3. 主编辑器未打开画板不加载React。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.saveDrawingFiles

文件：`modules/content/web/src/editor/drawing-wrapper.ts`。

`saveDrawingFiles(ctx: RequestContext, input: saveDrawingFilesInput): Promise<saveDrawingFilesResult>`

保存场景和亮暗SVG三个内容寻址文件。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：saveDrawingFilesInput。保存场景和亮暗SVG三个内容寻址文件的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：saveDrawingFilesResult。保存场景和亮暗SVG三个内容寻址文件的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. Excalidraw导出scene JSON、亮SVG、暗SVG。
2. 浏览器计算SHA分别upload；全部ready后更新drawing块三个ref。
3. 新编辑只换新hash，旧文件历史保留，公开只SVG不加载React。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

## content.prepareVideoBlock

文件：`modules/content/web/src/editor/video-insertion.ts`。

`prepareVideoBlock(ctx: RequestContext, input: prepareVideoBlockInput): Promise<prepareVideoBlockResult>`

统一上传/平台视频插入。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：prepareVideoBlockInput。统一上传/平台视频插入的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：prepareVideoBlockResult。统一上传/平台视频插入的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 上传格式验证且浏览器截poster可自传，服务不转码。
2. 平台仅插入时readVideoMetadata抓公开信息/本地封面，失败让用户手填。
3. 生成统一source/title/description/poster attrs；平台播放器点击后加载。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：media:readVideoMetadata。

## content.resizeColumns

文件：`modules/content/web/src/editor/columns-resizer.ts`。

`resizeColumns(ctx: RequestContext, input: resizeColumnsInput): Promise<resizeColumnsResult>`

调整2–4栏宽比例。

**输入**

- ctx：RequestContext。调用方、委托主体、operationId、期限与 dataEpoch；服务边界验签且禁止浏览器自造；约束：deadline 未过期；dataEpoch 与 site-config 活动 epoch 一致；权限按资源再次检查。
- input：resizeColumnsInput。调整2–4栏宽比例的已声明参数；约束：必须通过 types.json 对应 schema；拒绝未知字段；字节/范围/空值按 schema。

**输出**

- result：resizeColumnsResult。调整2–4栏宽比例的完整成功值；约束：无 error 时才有效；满足 types.json schema。
- error：Error。失败时 result 无效，返回标准 Problem；已提交但客户端超时必须用相同 operationId 查询/重试；约束：VALIDATION: 字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；LOCKED: 锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；VERSION_MISMATCH: expectedRevision、对象版本或 dataEpoch 已改变；DEPENDENCY_UNAVAILABLE: 权威依赖、存储或密钥不可用；不可把未知状态当允许。

**步骤**

1. 数量匹配并总和1。
2. 拖进列保块ID，禁止列内任何后代columns。
3. 存attrs.widthRatios，窄屏按列顺序叠排。

**错误**：VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值；retryable=false；LOCKED：锁、屏障、删除 fence 或正在处理的互斥操作阻止执行；retryable=false；VERSION_MISMATCH：expectedRevision、对象版本或 dataEpoch 已改变；retryable=false；DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许；retryable=true。

**副作用**：浏览器Tiptap状态/有界网络请求。

**授权**：浏览器当前主体content.edit；后端saveDraft再次验证。

**并发**：每editor会话串行事务；保存单飞，失锁立即取消新写。

**幂等**：编辑动作依TiptaptransactionID去重，网络写operationId去重。

**依赖**：content.ValidateBlockTree。

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M04-F901 main

文件 `modules/content/cmd/content/main.go`。独立进程可执行入口

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

### M04-F902 BuildApplication

文件 `modules/content/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M04-F903 Serve

文件 `modules/content/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M04-F904 Shutdown

文件 `modules/content/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M04-F905 Health

文件 `modules/content/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M04-F906 Ready

文件 `modules/content/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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

### M04-F907 mountModuleUI

文件 `modules/content/web/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

```typescript
function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void
```

输入：

- `rootElement: HTMLElement`：已经存在的模块挂载节点；约束：DOM标准库；为空或不属于当前document拒绝

- `bootstrap: ModuleUIBootstrap`：已由portal/Go输出的安全启动元数据；约束：module固定本域，不含service凭证/secret

输出：

无参数/返回值。

步骤：

1. 校验rootElement与bootstrap module/capabilities；按本域已登记Vue页面导入模块路由，createApp挂载，不注册别域业务组件
2. 只装配同源HTTP/JSON/SSE客户端和已有权限清理/语言逻辑，复用identity/qa会话，缺AI能力保持关闭提示
3. 同DOM重复mount先复用既有实例；unmount停止AbortController/SSE/listeners，不能清服务端业务记录

错误：

- `VALIDATION`：bootstrap不匹配本域或DOM无效；HTTP 400；retryable=false

副作用：挂载本域Vue组件和浏览器事件；不新建业务记录。

授权：{"permission": "anonymous", "rule": "只挂载UI；业务mutation仍按各本域OpenAPI权限，不能以能力字段绕过权限"}

并发：同DOM一个Vue实例，AbortController取消旧请求

幂等：同DOM/buildRevision重复挂载复用实例

依赖：本域已登记UI页面函数
<!-- service-bootstrap:end -->
