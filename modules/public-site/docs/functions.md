# public-site 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 79 个函数：约束 33 个，参考 46 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## public-site.ResolveRoute

- 级别：参考
- 文件：`modules/public-site/internal/routing.go`
- 作用：规范化语言与 slug

```go
func ResolveRoute(ctx context.Context, op OperationContext, in ResolveRouteInput) (ResolveRouteResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ResolveRouteInput` | 规范化语言与 slug；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolveRouteResult` | 规范化语言与 slug成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒绝NUL/反斜杠/重复编码
2. 只对根路径按语言cookie→Accept-Language→基准语言返回302
3. 带前缀页不因偏好跳转
4. 向content查层级slug旧路径映射，改名/移动返回301
5. 删除返回410或管理员指定替代301

**错误**

- `VALIDATION` / HTTP 400：INVALID_ROUTE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：LOCALE_UNSUPPORTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：公开请求
- 并发：纯函数及只读 HTTP；禁止修改全局 locale
- 幂等：同 path 和路由修订同结果
- 依赖：`content:resolvePublicPath`、`site-config:readConfigObject`

## public-site.SelectLocale

- 级别：参考
- 文件：`modules/public-site/internal/locale.go`
- 作用：确定展示语言

```go
func SelectLocale(ctx context.Context, op OperationContext, in SelectLocaleInput) (SelectLocaleResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `SelectLocaleInput` | 确定展示语言；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SelectLocaleResult` | 确定展示语言成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 已带语言前缀严格保留该语言
2. 根路径优先显式cookie
3. 解析Accept-Language q排序，zh-TW/HK→zh-Hant、zh-CN/SG→zh-Hans、pt-BR→pt
4. 无匹配基准语言
5. 翻译缺块使用旧译或原文并显式顶部提示

**错误**

- `VALIDATION` / HTTP 400：LOCALE_UNSUPPORTED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：公开请求
- 并发：纯函数
- 幂等：相同语言集合同结果
- 依赖：`site-config:readConfigObject`

## public-site.CheckPublication

- 级别：参考
- 文件：`modules/public-site/internal/authority.go`
- 作用：执行正文公开权威检查

```go
func CheckPublication(ctx context.Context, op OperationContext, in CheckPublicationInput) (CheckPublicationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `CheckPublicationInput` | 执行正文公开权威检查；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `CheckPublicationResult` | 执行正文公开权威检查成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 比较本次dataEpoch；读取content同一公开树中的精确PublishedRevisionRef
2. 可空expectedRevision为null时读取当前引用，非null必须精确匹配；不签发或缓存第二份公开token
3. 调用validatePublications核visible/matchesCurrent/visibilityRevision和精确ref；撤回返回NOT_FOUND，未知503

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：site.read；rule：服务身份 public-site.read-public
- 并发：每 HTTP 请求执行，不能跨请求复用状态 TTL
- 幂等：只读，重试至请求总 deadline
- 依赖：`content:readPublicTree`、`content:validatePublications`

## public-site.LoadPublishedBundle

- 级别：参考
- 文件：`modules/public-site/internal/content_client.go`
- 作用：取得同一发布修订完整包

```go
func LoadPublishedBundle(ctx context.Context, op OperationContext, in LoadPublishedBundleInput) (LoadPublishedBundleResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `LoadPublishedBundleInput` | 取得同一发布修订完整包；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `LoadPublishedBundleResult` | 取得同一发布修订完整包成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 携完整PublishedRevisionRef读取精确已发布正文，核hash/schema与2MiB限制
2. 原样保留Tiptap结构、标题、元数据与完整AssetVersionRef
3. 安全可用子集：仅原本一个无marks纯text的paragraph/heading可使用同源译文targetText替换；其他富文本保留原文并登记untranslatedBlockIds，不删除格式和链接
4. 旧译或缺失译文仍保留显式语言URL；复杂译文映射与翻译SEO优先规则缺口使本函数整体仍partial，不计完整ready
5. 返回前再次读取content公开权威并精确匹配同一ref

**错误**

- `VERSION_MISMATCH` / HTTP 409：REVISION_CHANGED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：BODY_TOO_LARGE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：site.read；rule：服务身份 public-site.read-public
- 并发：content token 只绑定单修订；失配至多重走一次检查
- 幂等：读取无写；不能回退旧正文
- 依赖：`content:readPublishedRevision`、`localization:readTranslation`、`public-site.ParseTiptapDocument`、`public-site.CheckPublication`

## public-site.safeLink

- 级别：参考
- 文件：`modules/public-site/internal/sanitize.go`
- 作用：静态输出协议与同源根路径白名单

```go
func safeLink(raw string) bool
```

**输入**

无

**输出**

无

**步骤**

1. 静态输出协议与同源根路径白名单

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.BuildTOC

- 级别：参考
- 文件：`modules/public-site/internal/toc.go`
- 作用：生成稳定锚点目录

```go
func BuildTOC(ctx context.Context, op OperationContext, in BuildTOCInput) (BuildTOCResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildTOCInput` | 生成稳定锚点目录；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildTOCResult` | 生成稳定锚点目录成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 标题H1不进入右侧目录
2. 正文H2-H4逐项使用原稳定blockId锚点
3. 同锚点冲突拒INTEGRITY_ERROR
4. 目录包含所有标题且正文不折叠

**错误**

- `VALIDATION` / HTTP 400：INVALID_TOC_DEPTH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：渲染内部调用
- 并发：纯函数；最多 512 项
- 幂等：同 headings 同锚点
- 依赖：无

## public-site.RewriteInternalLinks

- 级别：参考
- 文件：`modules/public-site/internal/links.go`
- 作用：解析跨文档链接

```go
func RewriteInternalLinks(ctx context.Context, op OperationContext, in RewriteInternalLinksInput) (RewriteInternalLinksResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RewriteInternalLinksInput` | 解析跨文档链接；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RewriteInternalLinksResult` | 解析跨文档链接成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 输入为稳定documentId与可空blockId目标；按同次权威公开导航解析，未知目标不输出路径
2. 非空blockId读取精确发布正文核hash/schema与锚点
3. 最终再次读取并逐引用比较公开导航；变化返回STALE_RESULT，输出映射键documentId#blockId

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：site.read；rule：public-site.read-public
- 并发：每批 100 目标，4 并发，总 deadline 2500ms
- 幂等：读取无副作用
- 依赖：`public-site.BuildNavigation`、`public-site.anchorMissing`

## public-site.ResolvePublishedLink

- 级别：约束
- 文件：`modules/public-site/internal/links.go`
- 作用：为site-config按稳定文档及块ID提供当前安全公开相对URL

```go
func ResolvePublishedLink(ctx context.Context, op OperationContext, input ResolvePublishedLinkInput) (ResolvePublishedLinkResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 沿调用链传播取消和截止 | 外部IO使用调用ctx，不创建独立重试窗口 |
| `op` | `OperationContext` | 入口建立的site-config纯服务上下文和当前固定dataEpoch | 仅callerService=site-config且actor.kind=service，不接受用户委托或浏览器自声明scope |
| `input` | `ResolvePublishedLinkInput` | documentId/language/blockId全部必填，blockId可null；language已按调用方baseLanguage补齐 | 严格schema在边界一次验证，函数内不重解析服务断言/期限或重做字段schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolvePublishedLinkResult` | 当前请求安全公开链接判断；false无标题/metadata/draft/旧路径 | true才href/publication/resolvedLanguage非null且available；false三项全null；resolvedLanguage=request.language，不授予后续读取授权 |
| `error` | `error` | 失败仅公共Problem，不返回半结果 | 依赖未知返回503；权威水位变化返回公共错误表的STALE_RESULT/409，不能从旧缓存制造visible=false |

**步骤**

1. 入口静态限制仅已认证site-config纯服务调用，固定当前op.dataEpoch；公共边界一次建立服务断言、caller、严格schema和deadline，拒绝用户委托
2. 通过site-config:readSiteConfig读取已提交安全配置及revision/baseLanguage/publicBaseURL，绝不调用readPublicHome或其主页解析链，避免递归；localization:listLanguages(includeDisabled=false)提供当前启用语言权威
3. 通过content:readPublicTree的同一公开树快照按documentId取精确PublishedRevisionRef、visibilityRevision与权威canonicalPath；不存在/未公开/撤回/删除统一false/unavailable且三项null，不读取草稿或管理metadata
4. canonicalPath须由content同一公开快照提供完整分段URL编码路径，不含语言/basePath/query/fragment，正确保留被公开导航隐藏祖先的路径；不得按裁剪parentId/pathSegment自行拼slug，也不用渲染cache或事件投影替代公开权威
5. 公开目标的language未启用或已停用才false/language_unavailable；已启用语言译文暂缺继续按M06-R006显示旧译/原文，URL保留请求language，resolvedLanguage=request.language，不调用模型或创建翻译
6. blockId非null时携精确publishedRevisionId调用content:readPublishedRevision，只读当前公开正文并按LoadPublishedBundle既有hash/schema及稳定块ID规则核验；锚点不存在候选为false/anchor_missing，不偷偷退到文档顶部，不因缺译判锚点不存在
7. 复用本域BuildCanonical规范机制，将可信publicBaseURL的basePath、请求language和content权威canonicalPath恰好拼接一次，清除全部query；从原始规范URL取得同源相对路径，附已存在稳定blockId的URL编码片段，不在JSON接口写HTML属性转义串或外部origin
8. 返回公开目标的任何路径/锚点/语言候选前，以content:resolvePublicPath(canonicalPath)重核仍指同documentId和当前发布ref，以content:validatePublications核同精确ref的visible、matchesCurrent和visibilityRevision；配置/启用语言仍须保持已读revision/version及epoch。当前已知撤回只false/unavailable；仍公开但修订/路径/配置/语言水位变化直接STALE_RESULT/409，权威未知保持DEPENDENCY_UNAVAILABLE/503，不自动重算或重试
9. 成功响应no-store；true仅href/current publication/request language/available，false仅封闭reason与三项null。解析结果不授予公开正文读取权限，实际页面每次仍执行既有content最终门禁
10. 允许qa用于引用href解析，仍是当前公开发布来源；不得返回草稿或预览链接

**错误**

- `VALIDATION` / HTTP 400：严格输入schema不满足（不可重试）
- `UNAUTHENTICATED` / HTTP 401：服务断言缺失或失效（不可重试）
- `FORBIDDEN` / HTTP 403：不是site-config纯服务主体或夹带用户委托（不可重试）
- `VERSION_MISMATCH` / HTTP 409：请求dataEpoch非当前权威代次（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：content/配置/启用语言权威未知或不可达，不能从旧缓存推定不可见（可重试）
- `STALE_RESULT` / HTTP 409：公开引用/规范路径/visibility/config/language水位变化，不交付不一致URL（可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：调用链截止到达（可重试）

- 副作用：无
- 授权：permission：service；rule：仅site-config或qa纯服务身份；只解析当前公开发布的精确文档/语言/块，返回publication供调用方再核对
- 并发：总deadline=min(ctx/op,2500ms)；外域只读快照与最终ref/path/config/language门禁，不宣称跨域单事务；水位变化直接STALE_RESULT/409，不重试
- 幂等：只读无持久副作用；同输入仅在同权威修订集合下同结果；无需Idempotency-Key，不缓存为后续授权
- 依赖：`site-config:readSiteConfig`、`content:readPublicTree`、`content:readPublishedRevision`、`content:resolvePublicPath`、`content:validatePublications`、`localization:listLanguages`、`public-site.BuildCanonical`、`public-site.anchorMissing`
- 其他依赖说明：content.PublicTreeNode.required canonicalPath由根同步producer；裁剪公开树parentId不能代替完整规范路径；冻结i18n/site/editor设计及M06-R006：翻译缺失不改变显式语言URL；已绑定真实ResolvePublishedLink；正文hash按content CanonicalizeBody的键排序、mark类型升序、platform.CanonicalJSON与SHA-256，并受本域2MiB限制。水位变化的HTTP状态以公共错误表409为准。源码未编译

## public-site.anchorMissing

- 级别：参考
- 文件：`modules/public-site/internal/links.go`
- 作用：读取精确发布正文并核对稳定块锚点

```go
func anchorMissing(ctx context.Context, client *gen.ContentClient, ref gen.PublishedRevisionRef, blockID string) (bool, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 精确发布锚点输入 | 可信服务链内使用 |
| `client` | `ContentClient` | 精确发布锚点输入 | 可信服务链内使用 |
| `ref` | `PublishedRevisionRef` | 精确发布锚点输入 | 可信服务链内使用 |
| `blockID` | `string` | 精确发布锚点输入 | 可信服务链内使用 |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `missing` | `bool` | 缺失标记或安全错误 | 未知来源失败关闭 |
| `error` | `error` | 缺失标记或安全错误 | 未知来源失败关闭 |

**步骤**

1. 经content:readPublishedRevision读取精确publication，按权威blockId/bodyHash判定锚点缺失，未知不能当有效

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：`content:readPublishedRevision`

## public-site.ResolvePublicMedia

- 级别：参考
- 文件：`modules/public-site/internal/media_client.go`
- 作用：取得公开资源清单

```go
func ResolvePublicMedia(ctx context.Context, op OperationContext, in ResolvePublicMediaInput) (ResolvePublicMediaResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ResolvePublicMediaInput` | 取得公开资源清单；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ResolvePublicMediaResult` | 取得公开资源清单成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 携完整PublishedRevisionRef与完整AssetVersionRef请求公开manifest
2. 未知media不可达时只返回空清单，由渲染器固定占位，不构造URL；其他错误保持失败
3. 核manifest的精确发布来源、资源身份、immutable_public与安全URL
4. 原图尺寸/alt和变体URL未在生产方manifest提供，本函数完整PublicAsset投影仍partial，不虚构尺寸或URL

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：MEDIA_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：MEDIA_NOT_PUBLIC：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：site.read；rule：public-site.read-public
- 并发：4 并发，缺图输出尺寸占位且禁止私有 URL
- 幂等：同引用修订可缓存元数据；授权仍逐响应检查
- 依赖：`media:buildDeliveryManifest`、`public-site.safeLink`

## public-site.BuildNavigation

- 级别：参考
- 文件：`modules/public-site/internal/navigation.go`
- 作用：构建可访问公开导航树

```go
func BuildNavigation(ctx context.Context, op OperationContext, in BuildNavigationInput) (BuildNavigationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildNavigationInput` | 构建可访问公开导航树；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildNavigationResult` | 构建可访问公开导航树成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取当前安全站点配置、启用语言与content权威公开树；停用语言拒绝
2. 只对同epoch公开树节点按sortKey稳定DFS排序，使用content完整canonicalPath，不从缺失祖先拼路径
3. 按100项精确PublishedRevisionRef批量核验visible/matchesCurrent及visibilityRevision
4. 返回前重读配置、语言与treeRevision水位，变化返回STALE_RESULT；未知依赖返回503

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：NAV_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新派生导航投影
- 授权：permission：site.read；rule：public-site.read-public
- 并发：投影修订 CAS，不覆盖新修订
- 幂等：以 epoch/locale/sourceRevision 去重
- 依赖：`content:readPublicTree`、`content:validatePublications`、`site-config:readSiteConfig`、`localization:listLanguages`、`localization:readInterfaceCatalog`、`localization:listTranslationTitles`

## public-site.navigationOrder

- 级别：参考
- 文件：`modules/public-site/internal/navigation.go`
- 作用：对已公开节点作稳定DFS并拒绝重复、缺父与环

```go
func navigationOrder(nodes []NavigationNode) ([]NavigationNode, error)
```

**输入**

无

**输出**

无

**步骤**

1. 对已公开节点作稳定DFS并拒绝重复、缺父与环

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.BuildBreadcrumbs

- 级别：参考
- 文件：`modules/public-site/internal/breadcrumbs.go`
- 作用：生成面包屑

```go
func BuildBreadcrumbs(ctx context.Context, op OperationContext, in BuildBreadcrumbsInput) (BuildBreadcrumbsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildBreadcrumbsInput` | 公开树与当前语言homeURL；字段见types.json | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildBreadcrumbsResult` | 生成面包屑成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 定位公开树父链并检测重复与环；未知目标拒绝
2. 使用调用方明确的homeURL输出首页与原始规范链接，不从树节点猜语言首页

**错误**

- `INTEGRITY_ERROR` / HTTP 422：公开树重复/缺父/环（不可重试）
- `NOT_FOUND` / HTTP 404：目标节点不存在（不可重试）

- 副作用：无
- 授权：permission：service；rule：渲染内部调用
- 并发：纯函数，父链最多 32
- 幂等：同树修订同结果
- 依赖：无

## public-site.BuildCanonical

- 级别：参考
- 文件：`modules/public-site/internal/seo.go`
- 作用：生成规范地址

```go
func BuildCanonical(ctx context.Context, op OperationContext, in BuildCanonicalInput) (BuildCanonicalResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildCanonicalInput` | 生成规范地址；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildCanonicalResult` | 生成规范地址成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只信任site-config安全配置publicBaseURL，basePath与调用方语言规范路径恰好组合一次
2. 移除全部query，返回原始规范URL；HTML属性转义仅在HTML输出点执行，不污染JSON链接解析结果

**错误**

- `VALIDATION` / HTTP 400：INVALID_BASE_URL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_ROUTE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：渲染内部调用
- 并发：纯函数
- 幂等：相同输入同结果
- 依赖：无
- 其他依赖说明：调用方传入已读取的publicBaseURL；输入没有配置对象ID，本函数不调用readConfigObject

## public-site.BuildHreflang

- 级别：参考
- 文件：`modules/public-site/internal/hreflang.go`
- 作用：输出已有公开语言版本

```go
func BuildHreflang(ctx context.Context, op OperationContext, in BuildHreflangInput) (BuildHreflangResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildHreflangInput` | 输出已有公开语言版本；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildHreflangResult` | 输出已有公开语言版本成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取当前公开精确ref、配置与启用语言
2. 为每启用语言输出规范URL，缺译仍保留该语言URL；x-default指向站点basePath根
3. 返回前重核路径、精确publication和配置/语言/tree水位，不用事件或缓存授权

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：site.read；rule：public-site.read-public
- 并发：同请求批量权威查询
- 幂等：只读，撤回语言不保留旧 alternate
- 依赖：`content:readPublicTree`、`content:validatePublications`、`content:resolvePublicPath`、`site-config:readSiteConfig`、`localization:listLanguages`、`public-site.BuildCanonical`

## public-site.BuildSEOMetadata

- 级别：参考
- 文件：`modules/public-site/internal/metadata.go`
- 作用：输出 SEO 和分享元数据

```go
func BuildSEOMetadata(ctx context.Context, op OperationContext, in BuildSEOMetadataInput) (BuildSEOMetadataResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildSEOMetadataInput` | 输出 SEO 和分享元数据；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildSEOMetadataResult` | 输出 SEO 和分享元数据成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 使用content已生成或人工优先且已翻译SEO description
2. 分享图优先文档否则站点默认
3. 转义title/description
4. 输出OG/canonical/hreflang/x-default/JSON-LD面包屑与text/markdown alternate，不临时调用模型

**错误**

- `VALIDATION` / HTTP 400：INVALID_CANONICAL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：渲染内部调用
- 并发：纯函数
- 幂等：同字段同结果
- 依赖：无

## public-site.ComposeHTML

- 级别：参考
- 文件：`modules/public-site/internal/html.go`
- 作用：构成完整响应文档

```go
func ComposeHTML(ctx context.Context, op OperationContext, in ComposeHTMLInput) (ComposeHTMLResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ComposeHTMLInput` | 构成完整响应文档；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ComposeHTMLResult` | 构成完整响应文档成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 输出完整Go三栏HTML、当前文档父分支展开导航、正文和右侧TOC；正文标题不折叠
2. 手机文档树与TOC使用按钮展开抽屉，正文无JavaScript仍完整
3. 输出面包屑、文末上下篇及页脚源代码链接；AI未配置显示条件
4. 增强主题切换、复制Markdown与平台点击加载，问答岛标记复用qa同会话；不在服务器代浏览器重复统计

**错误**

- `VALIDATION` / HTTP 400：INVALID_TEMPLATE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：渲染内部调用
- 并发：模板启动时编译；每请求独立缓冲
- 幂等：同输入与 themeRevision 同输出结构
- 依赖：`public-site.navigationOrder`

## public-site.BuildCacheKey

- 级别：参考
- 文件：`modules/public-site/internal/cache_key.go`
- 作用：固定派生缓存身份

```go
func BuildCacheKey(ctx context.Context, op OperationContext, in BuildCacheKeyInput) (BuildCacheKeyResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildCacheKeyInput` | 固定派生缓存身份；字段见本域types.json对应$defs | 严格schema，拒未知字段；metadataRevision必须来自content当前公开读取或content.metadata.changed/v1水位，未知值由调用方失败关闭；不得以默认0或publicationRevision代替；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildCacheKeyResult` | 固定派生缓存身份成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 以 public-site-renderer-v2 作为 rendererVersion；按 dataEpoch、documentId、publicationRevision、metadataRevision、locale、themeRevision 的稳定顺序逐字段长度编码
2. 不复用或读取 public-site-renderer-v1 生成的缓存键，避免旧正文缓存复活旧 OG
3. SHA-256 生成路径

**错误**

- `INTERNAL` / HTTP 500：INVALID_REVISION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：内部调用
- 并发：纯函数
- 幂等：相同 dataEpoch、documentId、publicationRevision、metadataRevision、locale、themeRevision 同 key；任一维度变化均生成不同 v2 key
- 依赖：无

## public-site.ReadRenderCache

- 级别：参考
- 文件：`modules/public-site/internal/cache_store.go`
- 作用：读取派生 HTML

```go
func ReadRenderCache(ctx context.Context, op OperationContext, in ReadRenderCacheInput) (ReadRenderCacheResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReadRenderCacheInput` | 读取派生 HTML；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReadRenderCacheResult` | 读取派生 HTML成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 检查索引 epoch/TTL
2. 读取文件校验 hash
3. 损坏删除索引并记 errorCode

**错误**

- `INTEGRITY_ERROR` / HTTP 422：CACHE_CORRUPT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新 LRU/损坏清理，不含正文日志
- 授权：permission：service；rule：内部已授权调用
- 并发：SQLite 读+独立文件读；cache miss 不算错误
- 幂等：读结果可重复；损坏清理幂等
- 依赖：无
- 其他依赖说明：私有 SQLite/cache 文件

## public-site.WriteRenderCache

- 级别：参考
- 文件：`modules/public-site/internal/cache_store.go`
- 作用：原子安装派生缓存文件

```go
func WriteRenderCache(ctx context.Context, op OperationContext, in WriteRenderCacheInput) (WriteRenderCacheResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `WriteRenderCacheInput` | 原子安装派生缓存文件；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WriteRenderCacheResult` | 原子安装派生缓存文件成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 写临时文件并 fsync
2. rename 至内容哈希路径
3. 事务 upsert 索引
4. 失败临时文件由恢复扫描清理

**错误**

- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `STALE_RESULT` / HTTP 409：STALE_RENDER：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写缓存文件与索引
- 授权：permission：service；rule：内部已授权调用
- 并发：key 单飞锁和 epoch CAS；不宣称文件/SQLite 同事务
- 幂等：同 key/hash upsert 同结果
- 依赖：无
- 其他依赖说明：私有 SQLite/cache 文件

## public-site.InvalidatePublication

- 级别：约束
- 文件：`modules/public-site/internal/invalidation.go`
- 作用：处理发布、撤回与译文就绪事件

```go
func InvalidatePublication(ctx context.Context, op OperationContext, in InvalidatePublicationInput) (InvalidatePublicationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `InvalidatePublicationInput` | 处理发布和撤回事件；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `InvalidatePublicationResult` | 处理发布和撤回事件成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒绝旧 epoch
2. inbox 原子去重
3. 删除该文档低于等于失效修订的缓存和导航投影；translation.ready只失效对应语言投影
4. content.publication.changed与translation.ready在提交后调用jobs:createJob登记public-site.render-publication（幂等键documentId+publicationRevision+language），由public-site.PrewarmPublication执行；withdrawn禁止预热

**错误**

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写 inbox/索引并清理文件
- 授权：permission：service；rule：签名 content 事件
- 并发：事务 inbox+索引；重复或乱序不复活缓存
- 幂等：eventId 与 sourceRevision 去重
- 依赖：`jobs:createJob`、`platform.ClaimInbox`

## public-site.RenderPublicPage

- 级别：参考
- 文件：`modules/public-site/internal/renderer.go`
- 作用：编排公开正文输出

```go
func RenderPublicPage(ctx context.Context, op OperationContext, in RenderPublicPageInput) (RenderPublicPageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenderPublicPageInput` | 编排公开正文输出；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderPublicPageResult` | 编排公开正文输出成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. CheckPublication权威校验当前发布修订与epoch；SelectLocale确定展示语言
2. BuildCacheKey后ReadRenderCache；命中且修订/主题/语言仍当前则使用缓存正文
3. 未命中：LoadPublishedBundle取得正文与译文回退，ResolvePublicMedia取公开资源清单，RenderTiptapHTML渲染；BuildTOC、BuildBreadcrumbs、BuildNavigation、BuildPreviousNext、RenderAnnouncement、BuildCanonical、BuildHreflang、BuildSEOMetadata后ComposeHTML，WriteRenderCache保存
4. 发送响应前再次CheckPublication；撤回或主题修订变化不输出旧缓存
5. 返回no-cache与ETag，前端只做渐进增强

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：REVISION_CHANGED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEADLINE_EXCEEDED` / HTTP 504：RENDER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：可写派生缓存；已撤回内容不送响应
- 授权：permission：anonymous；rule：公开请求
- 并发：5s 总 deadline；撤回后新检查不通过；检查后流式正文禁用
- 幂等：相同修订只生成一个缓存对象
- 依赖：`public-site.LoadPublishedBundle`、`public-site.RenderTiptapHTML`、`public-site.ComposeHTML`、`public-site.CheckPublication`、`public-site.SelectLocale`、`public-site.BuildCacheKey`、`public-site.ReadRenderCache`、`public-site.ResolvePublicMedia`、`public-site.BuildTOC`、`public-site.BuildBreadcrumbs`、`public-site.BuildNavigation`、`public-site.BuildPreviousNext`、`public-site.RenderAnnouncement`、`public-site.BuildCanonical`、`public-site.BuildHreflang`、`public-site.BuildSEOMetadata`、`public-site.WriteRenderCache`

## public-site.HandlePage

- 级别：约束
- 文件：`modules/public-site/internal/http_pages.go`
- 作用：页面 HTTP 入口

```go
func HandlePage(ctx context.Context, op OperationContext, in HandlePageInput) (HandlePageResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandlePageInput` | 页面 HTTP 入口；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandlePageResult` | 页面 HTTP 入口成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 解析路由
2. render 并最终权威校验
3. 校验成功且 etag 相等才 304，否则完整 200
4. 不存在或已删除时输出RenderNotFound（404）或410；页面访问统计由浏览器sendBeacon上报，服务端不重复计数

**错误**

- `VALIDATION` / HTTP 400：INVALID_ROUTE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：非阻塞匿名 beacon；不存请求 IP
- 授权：permission：anonymous；rule：anonymous
- 并发：请求缓冲到最终授权完成；不可先写 header
- 幂等：GET 可重复；statusCode 不缓存安全结论
- 依赖：`public-site.ResolveRoute`、`public-site.RenderPublicPage`、`public-site.RenderNotFound`

## public-site.HandleHome

- 级别：约束
- 文件：`modules/public-site/internal/http_home.go`
- 作用：站点首页

```go
func HandleHome(ctx context.Context, op OperationContext, in HandleHomeInput) (HandleHomeResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleHomeInput` | 站点首页；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleHomeResult` | 站点首页成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取当前语言可见的公开目录和已有搜索摘要，生成简短内容引导；不调用模型
2. 首页以固定一屏的大AI对话窗口为主，展示内容方向和建议问题，回答在窗口内滚动
3. 保留进入第一篇已发布文档的按钮；问答关闭或无脚本时仍可读引导和进入文档
4. 复用站点品牌、语言、主题与公开问答能力，安全编码所有目录和配置文字

**错误**

- `VALIDATION` / HTTP 400：SITE_NOT_INITIALIZED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：可写首页派生缓存
- 授权：permission：anonymous；rule：anonymous
- 并发：主题配置 revision 绑定 cacheKey
- 幂等：GET 幂等
- 依赖：`site-config:readConfigObject`、`media:buildDeliveryManifest`、`public-site.RenderHero`、`public-site.RenderAnnouncement`

## public-site.HandleNavigation

- 级别：约束
- 文件：`modules/public-site/internal/http_navigation.go`
- 作用：公开导航 HTTP

```go
func HandleNavigation(ctx context.Context, op OperationContext, in HandleNavigationInput) (HandleNavigationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleNavigationInput` | 公开导航 HTTP；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleNavigationResult` | 公开导航 HTTP成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取当前权威导航并最终核验每个精确发布引用
2. 按公开树序列及水位生成SHA-256 ETag，HTTP绑定只在本函数成功之后处理If-None-Match

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：NAV_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：更新投影
- 授权：permission：anonymous；rule：anonymous
- 并发：不能只返回历史投影
- 幂等：GET 幂等
- 依赖：`public-site.BuildNavigation`

## public-site.HandleMarkdown

- 级别：约束
- 文件：`modules/public-site/internal/http_markdown.go`
- 作用：导出公开 Markdown

```go
func HandleMarkdown(ctx context.Context, op OperationContext, in HandleMarkdownInput) (HandleMarkdownResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleMarkdownInput` | 导出公开 Markdown；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleMarkdownResult` | 导出公开 Markdown成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 公开路径后缀.md→content规范路由
2. 读取已完成Tiptap JSON
3. RenderTiptapMarkdown输出代码/表格/画板视频图片链接+文字描述
4. 最终publication权威验证
5. text/markdown no-cache+etag
6. AI已知爬虫单独计数

**错误**

- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：同 HTML 5s deadline 与最终状态校验
- 幂等：GET 幂等
- 依赖：`public-site.ResolveRoute`、`public-site.LoadPublishedBundle`、`public-site.RenderTiptapMarkdown`、`content:validatePublications`、`public-site.RecordAnonymousView`

## public-site.HandleRobots

- 级别：约束
- 文件：`modules/public-site/internal/http_robots.go`
- 作用：机器人规则

```go
func HandleRobots(ctx context.Context, op OperationContext, in HandleRobotsInput) (HandleRobotsResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleRobotsInput` | 机器人规则；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleRobotsResult` | 机器人规则成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取SiteConfig.indexingEnabled作为站点统一索引策略
2. 启用时明确Allow GPTBot/ChatGPT-User/ClaudeBot/PerplexityBot，并禁止basePath下admin和v1私有路径；停用时全部Disallow
3. 输出总sitemap及每启用语言sitemap，不产生统计cookie；返回前核配置/语言水位

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：SITE_CONFIG_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：配置不可达返回 503 禁止旧配置承诺
- 幂等：GET 幂等
- 依赖：`site-config:readSiteConfig`、`localization:listLanguages`、`public-site.readIndexingPolicy`

## public-site.readIndexingPolicy

- 级别：参考
- 文件：`modules/public-site/internal/http_robots.go`
- 作用：读取site-config当前站点统一索引开关

```go
func readIndexingPolicy(ctx context.Context) (bool, error)
```

**输入**

无

**输出**

无

**步骤**

1. 读取site-config当前站点统一索引开关

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：`site-config:readSiteConfig`

## public-site.BuildLLMsIndex

- 级别：参考
- 文件：`modules/public-site/internal/llms.go`
- 作用：生成模型可读公开索引

```go
func BuildLLMsIndex(ctx context.Context, op OperationContext, in BuildLLMsIndexInput) (BuildLLMsIndexResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildLLMsIndexInput` | 生成模型可读公开索引；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildLLMsIndexResult` | 生成模型可读公开索引成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 权威枚举并携精确publishedRevisionId读公开SEO摘要
2. 按导航稳定DFS输出标题、摘要和.md链接，输出超过maxBytes拒绝
3. 最终重读并比较每个公开引用与树水位，不出现草稿元数据

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VALIDATION` / HTTP 400：EXPORT_LIMIT：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：可写派生文本缓存
- 授权：permission：site.read；rule：public-site.read-public
- 并发：每项在输出前批量 authority 校验
- 幂等：同公开修订集合可复用缓存
- 依赖：`public-site.BuildNavigation`、`content:readPublishedRevision`、`public-site.markdownLabel`

## public-site.markdownLabel

- 级别：参考
- 文件：`modules/public-site/internal/llms.go`
- 作用：Markdown链接标签转义

```go
func markdownLabel(value string) string
```

**输入**

无

**输出**

无

**步骤**

1. Markdown链接标签转义

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.HandleLLMsIndex

- 级别：约束
- 文件：`modules/public-site/internal/http_llms.go`
- 作用：llms.txt HTTP

```go
func HandleLLMsIndex(ctx context.Context, op OperationContext, in HandleLLMsIndexInput) (HandleLLMsIndexResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleLLMsIndexInput` | llms.txt HTTP；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleLLMsIndexResult` | llms.txt HTTP成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取SiteConfig.indexingEnabled；停用时404
2. 生成同次已授权公开索引
3. 发送前再次读取索引开关，停用时404

**错误**

- `VALIDATION` / HTTP 400：INDEX_DISABLED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：权威批量校验不使用过期索引安全结论
- 幂等：GET 幂等
- 依赖：`public-site.readIndexingPolicy`、`public-site.BuildLLMsIndex`

## public-site.HandleLLMsFull

- 级别：约束
- 文件：`modules/public-site/internal/http_llms.go`
- 作用：输出合并正文或分片索引

```go
func HandleLLMsFull(ctx context.Context, op OperationContext, in HandleLLMsFullInput) (HandleLLMsFullResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleLLMsFullInput` | 输出合并正文或分片索引；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleLLMsFullResult` | 输出合并正文或分片索引成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 权威枚举并读取固定修订
2. 累计 8MiB 时返回分片链接索引
3. 最终批量验证每项仍 published

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：REVISION_CHANGED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：派生文本缓存
- 授权：permission：anonymous；rule：anonymous
- 并发：不得边读边发送；总时限 5s，超限 503
- 幂等：不产生付费调用
- 依赖：`public-site.LoadPublishedBundle`、`public-site.BuildLLMsIndex`

## public-site.BuildSitemapEntries

- 级别：参考
- 文件：`modules/public-site/internal/sitemap.go`
- 作用：生成规范 sitemap 项

```go
func BuildSitemapEntries(ctx context.Context, op OperationContext, in BuildSitemapEntriesInput) (BuildSitemapEntriesResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildSitemapEntriesInput` | 生成规范 sitemap 项；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildSitemapEntriesResult` | 生成规范 sitemap 项成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 权威枚举当前公开导航，携精确publishedRevisionId读发布元数据
2. lastmod仅使用lastPublishedAt，不使用缓存时间；canonical URL采用可信publicBaseURL及完整公开路径
3. 最终重读并比较整份公开树和配置水位，再按loc字典排序

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：site.read；rule：public-site.read-public
- 并发：最终批量 authority 校验
- 幂等：同公开集合同分片边界
- 依赖：`public-site.BuildNavigation`、`site-config:readSiteConfig`、`content:readPublishedRevision`

## public-site.HandleSitemapIndex

- 级别：约束
- 文件：`modules/public-site/internal/http_sitemap.go`
- 作用：sitemap 索引 HTTP

```go
func HandleSitemapIndex(ctx context.Context, op OperationContext, in HandleSitemapIndexInput) (HandleSitemapIndexResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleSitemapIndexInput` | sitemap 索引 HTTP；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleSitemapIndexResult` | sitemap 索引 HTTP成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取启用语言并生成当前公开条目集
2. 每10000项固定分片，只为存在公开内容的语言输出当前epoch分片地址
3. 返回前核配置与语言水位，XML输出点转义

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：可写派生 XML 缓存
- 授权：permission：anonymous；rule：anonymous
- 并发：不使用已撤回分片清单
- 幂等：GET 幂等
- 依赖：`public-site.BuildSitemapEntries`、`site-config:readSiteConfig`、`localization:listLanguages`、`public-site.sitemapIndexXML`

## public-site.HandleSitemapShard

- 级别：约束
- 文件：`modules/public-site/internal/http_sitemap.go`
- 作用：单分片 sitemap

```go
func HandleSitemapShard(ctx context.Context, op OperationContext, in HandleSitemapShardInput) (HandleSitemapShardResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleSitemapShardInput` | 单分片 sitemap；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleSitemapShardResult` | 单分片 sitemap成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒绝负shard，读取当前稳定并最终权威校验的公开条目
2. 每10000项切片，越界404；XML输出点转义

**错误**

- `VALIDATION` / HTTP 400：INVALID_SHARD：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：每响应权威校验所含条目
- 幂等：GET 幂等
- 依赖：`public-site.BuildSitemapEntries`、`public-site.sitemapURLSetXML`

## public-site.xmlText

- 级别：参考
- 文件：`modules/public-site/internal/http_sitemap.go`
- 作用：XML文本转义

```go
func xmlText(value string) string
```

**输入**

无

**输出**

无

**步骤**

1. XML文本转义

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.sitemapIndexXML

- 级别：参考
- 文件：`modules/public-site/internal/http_sitemap.go`
- 作用：输出sitemapindex

```go
func sitemapIndexXML(locations []string) string
```

**输入**

无

**输出**

无

**步骤**

1. 输出sitemapindex

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：`public-site.xmlText`

## public-site.sitemapURLSetXML

- 级别：参考
- 文件：`modules/public-site/internal/http_sitemap.go`
- 作用：输出规范urlset

```go
func sitemapURLSetXML(entries []SitemapEntry) string
```

**输入**

无

**输出**

无

**步骤**

1. 输出规范urlset

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：`public-site.xmlText`

## public-site.IssuePreviewToken

- 级别：约束
- 文件：`modules/public-site/internal/preview.go`
- 作用：签发草稿预览授权

```go
func IssuePreviewToken(ctx context.Context, op OperationContext, in IssuePreviewTokenInput) (IssuePreviewTokenResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `IssuePreviewTokenInput` | 签发草稿预览授权；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `IssuePreviewTokenResult` | 签发草稿预览授权成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 即时核当前admin会话content.preview权限
2. 只以documentId和明确draftRevisionId调用content.createPreviewGrant；旧revision可选弃用，不用于授权或ID映射
3. 核拥有者返回grant的本人、文档与精确草稿，直接返回previewId/token/expiresAt；不造第二份授权真相
4. 同operationId重试由content返回同grant，有效期内刷新可重复本人预览

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：REVISION_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：content拥有预览grant，本域不存token原文
- 授权：permission：content.preview；rule：identity session + content.preview
- 并发：随机 256-bit token；每 token 一次消费
- 幂等：同operationId由content拥有者返回同grant，不本地签发新token。
- 依赖：`identity:checkAuthorization`、`content:createPreviewGrant`

## public-site.ValidatePreviewToken

- 级别：参考
- 文件：`modules/public-site/internal/preview.go`
- 作用：每次重验拥有者草稿预览授权，不消费刷新许可

```go
func ValidatePreviewToken(ctx context.Context, op OperationContext, in ValidatePreviewTokenInput) (ValidatePreviewTokenResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ValidatePreviewTokenInput` | 原子消费预览 token；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ValidatePreviewTokenResult` | 原子消费预览 token成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅当前admin会话可调用，核previewId与token格式大小
2. 携previewId/token及可信op.actor.subjectId调用content.validatePreviewGrant，重验本人/epoch/expiry/精确草稿
3. 仅在本次内部调用链返回已授权body与所需metadata投影；不消费一次性状态，不持久化token

**错误**

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：签入身份必须一致
- 并发：SQLite CAS；重放不可通过
- 幂等：同token/actor在有效期内重复验证只读安全
- 依赖：`content:validatePreviewGrant`

## public-site.RenderPreview

- 级别：参考
- 文件：`modules/public-site/internal/preview_renderer.go`
- 作用：渲染受保护修订

```go
func RenderPreview(ctx context.Context, op OperationContext, in RenderPreviewInput) (RenderPreviewResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenderPreviewInput` | 本次已验证的内部snapshot与原previewId/token，不从HTTP请求接收snapshot。 | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderPreviewResult` | 渲染受保护修订成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅接收HandlePreview本次经content验证的内部snapshot，按生产方唯一schema解析
2. 资源携原previewId/token向media请求private_no_store manifest；核精确资源身份，不进入公开缓存
3. 正文沿同一HTML块渲染和公开样式输出，标题不折叠；内部ID目标只链接当前公开目标
4. 输出noindex元数据，受保护资源依赖失败保持503，不转公开URL

**错误**

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `NOT_FOUND` / HTTP 404：REVISION_NOT_FOUND：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无公开缓存写入
- 授权：permission：content.preview；rule：content.preview
- 并发：独立预览缓冲；禁用公开导航/private 标题混入
- 幂等：只读重渲染无需模型
- 依赖：`public-site.ParseTiptapDocument`、`public-site.RenderTiptapHTML`、`public-site.BuildTOC`、`public-site.ComposeHTML`、`public-site.collectInternalLinks`、`public-site.RewriteInternalLinks`、`media:buildDeliveryManifest`

## public-site.collectInternalLinks

- 级别：参考
- 文件：`modules/public-site/internal/preview_renderer.go`
- 作用：收集正文稳定ID链接目标并去重

```go
func collectInternalLinks(document TiptapDocument) []InternalLinkTarget
```

**输入**

无

**输出**

无

**步骤**

1. 收集正文稳定ID链接目标并去重

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的内部读取/渲染或当前浏览器会话调用链使用，不建立新授权权威
- 并发：调用方传播取消，不修改共享业务记录
- 幂等：只读或复用拥有者同operationId回执；不持久化token
- 依赖：无

## public-site.HandlePreview

- 级别：约束
- 文件：`modules/public-site/internal/http_preview.go`
- 作用：预览 HTTP 入口

```go
func HandlePreview(ctx context.Context, op OperationContext, in HandlePreviewInput) (HandlePreviewResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandlePreviewInput` | 预览 HTTP 入口；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandlePreviewResult` | 预览 HTTP 入口成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 从当前登录上下文与token路径/必填previewId查询建立预览输入
2. ValidatePreviewToken重验本人草稿许可，再RenderPreview输出同样式完整HTML
3. 发送前再次由content验证同grant，快照发生变化拒绝STALE_RESULT
4. 返回CSP/noindex/no-store/private与no-referrer；刷新不消费许可，不把token写日志或公开缓存

**错误**

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无本域token消费状态
- 授权：permission：service；rule：identity session + token
- 并发：不允许 CDN 缓存，content 不可达返回 503
- 幂等：GET同actor/token在有效期重复只读安全
- 依赖：`public-site.ValidatePreviewToken`、`public-site.RenderPreview`

## public-site.RecordAnonymousView

- 级别：参考
- 文件：`modules/public-site/internal/beacon.go`
- 作用：AI爬虫Markdown访问单独计数

```go
func RecordAnonymousView(ctx context.Context, op OperationContext, in RecordAnonymousViewInput) (RecordAnonymousViewResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RecordAnonymousViewInput` | AI爬虫Markdown访问单独计数；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RecordAnonymousViewResult` | AI爬虫Markdown访问单独计数成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅成功Markdown已知GPTBot/ChatGPT-User/ClaudeBot/PerplexityBot请求进入独立crawler计数
2. 只传documentId/locale/crawlerFamily/status，不传UA原文、IP或正文
3. analytics未装配/不可达时丢弃且返回accepted=false，不使HTML或Markdown失败

**错误**

- `VALIDATION` / HTTP 400：BEACON_DROPPED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无本地访问身份历史
- 授权：permission：service；rule：内部公开响应流程
- 并发：缓冲 1024 条，满丢弃不影响页面
- 幂等：beaconId 去重由 analytics 执行
- 依赖：`analytics:recordCrawlerAccess`

## public-site.CreateSnapshot

- 级别：约束
- 文件：`modules/public-site/internal/snapshot.go`
- 作用：创建本域一致快照

```go
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `SnapshotExportRequest` | 创建本域一致快照；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `SnapshotExportResult` | 创建本域一致快照成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 验证 jobs barrier 已达安全点
2. checkpoint SQLite
3. 复制配置和索引，缓存 HTML/token 排除

**错误**

- `LOCKED` / HTTP 409：BARRIER_REQUIRED：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写临时快照目录
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：维护 barrier 下禁止索引写入
- 幂等：snapshotId 重复返回同 manifest
- 依赖：`jobs:coordinateBarrier`、`platform.SnapshotManager.Export`

## public-site.StageSnapshot

- 级别：约束
- 文件：`modules/public-site/internal/import.go`
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

## public-site.DiscardSnapshot

- 级别：约束
- 文件：`modules/public-site/internal/import.go`
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

## public-site.EnterBarrier

- 级别：约束
- 文件：`modules/public-site/internal/barrier.go`
- 作用：公开渲染域进入安全屏障

```go
func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `request` | `BarrierRequest` | 公开渲染域进入安全屏障；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BarrierStatus` | 公开渲染域进入安全屏障成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 拒新预览token和派生索引写
2. 等待正在渲染响应完成/缓存rename日志安全点
3. 冻结水位，公开入口仍受全站maintenance

**错误**

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写barrier状态
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：单barrier独占，不能截断正在写文件宣称safe
- 幂等：barrierId幂等
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.EnterBarrier`

## public-site.ReleaseBarrier

- 级别：约束
- 文件：`modules/public-site/internal/barrier.go`
- 作用：释放公开渲染维护

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ReleaseBarrierInput` | 释放公开渲染维护；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ReleaseBarrierResult` | 释放公开渲染维护成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 确认runtime新epoch和全域ready
2. 清旧epoch缓存/preview token
3. 恢复公开请求与投影写入

**错误**

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写barrier/清缓存
- 授权：permission：service；rule：仅data-protection服务身份；用户授权已由data-protection在发起时核验（手动备份/恢复需superadmin，定时备份按超管保存的设置）
- 并发：全站maintenance退出前不放行
- 幂等：barrierId幂等
- 依赖：`site-config:readRuntimeState`、`platform.SnapshotManager.ReleaseBarrier`

## public-site.HandleRootRedirect

- 级别：约束
- 文件：`modules/public-site/internal/http_root.go`
- 作用：根语言选择HTTP

```go
func HandleRootRedirect(ctx context.Context, op OperationContext, in HandleRootRedirectInput) (HandleRootRedirectResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleRootRedirectInput` | 根语言选择HTTP；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleRootRedirectResult` | 根语言选择HTTP成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 仅根路径读取当前启用语言与基准语言，按cookie→Accept-Language→基准选择
2. 返回302及当前basePath下语言首页；Vary为Accept-Language, Cookie；Cache-Control为no-store
3. 返回前重核配置和启用语言水位；带前缀页不调用

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：功能语言cookie读取，不写统计cookie
- 授权：permission：anonymous；rule：anonymous
- 并发：只读配置
- 幂等：GET幂等
- 依赖：`site-config:readSiteConfig`、`localization:listLanguages`、`public-site.SelectLocale`

## public-site.ParseTiptapDocument

- 级别：参考
- 文件：`modules/public-site/internal/tiptap.go`
- 作用：校验权威Tiptap渲染输入

```go
func ParseTiptapDocument(ctx context.Context, op OperationContext, in ParseTiptapDocumentInput) (ParseTiptapDocumentResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `ParseTiptapDocumentInput` | 校验权威Tiptap渲染输入；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `ParseTiptapDocumentResult` | 校验权威Tiptap渲染输入成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 在正文输入边界使用gen.PayloadValidationSpec中PublishedDocumentBody生产方自包含schema严格校验一次
2. 拒绝超出maxBytes、层级64、重复blockId/columnId/cellId及嵌套columns
3. 原样保留生产方节点与marks；收集标题和完整AssetVersionRef，不做有损归一化

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：UNSUPPORTED_SCHEMA：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：content.preview；rule：published或preview已授权
- 并发：纯函数
- 幂等：同JSON/hash同结果
- 依赖：`platform.NewPayloadValidator`、`public-site.attrString`、`public-site.attrInt`、`public-site.nodeText`
- 其他依赖说明：content.types.TiptapDocument

## public-site.attrString

- 级别：参考
- 文件：`modules/public-site/internal/tiptap.go`
- 作用：读取生产方已校验的字符串属性

```go
func attrString(attrs map[string]any, key string) string
```

**输入**

无

**输出**

无

**步骤**

1. 读取生产方已校验的字符串属性

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.attrInt

- 级别：参考
- 文件：`modules/public-site/internal/tiptap.go`
- 作用：读取生产方已校验的整数属性

```go
func attrInt(attrs map[string]any, key string) int
```

**输入**

无

**输出**

无

**步骤**

1. 读取生产方已校验的整数属性

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.nodeText

- 级别：参考
- 文件：`modules/public-site/internal/tiptap.go`
- 作用：保持次序提取节点已有文字

```go
func nodeText(node TiptapNode) string
```

**输入**

无

**输出**

无

**步骤**

1. 保持次序提取节点已有文字

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.assetFrom

- 级别：参考
- 文件：`modules/public-site/internal/tiptap.go`
- 作用：提取已校验完整资源引用

```go
func assetFrom(value any) (AssetVersionRef, bool)
```

**输入**

无

**输出**

无

**步骤**

1. 提取已校验完整资源引用

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.RenderTiptapHTML

- 级别：参考
- 文件：`modules/public-site/internal/tiptap_html.go`
- 作用：按节点输出完整HTML

```go
func RenderTiptapHTML(ctx context.Context, op OperationContext, in RenderTiptapHTMLInput) (RenderTiptapHTMLResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenderTiptapHTMLInput` | 按节点输出完整HTML；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderTiptapHTMLResult` | 按节点输出完整HTML成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按生产方节点与marks转义渲染段落、标题、引用、分割、列表、任务、表格、callout与多栏
2. chroma v2.20.0高亮代码；公式只输出LaTeX源码标记containsMath
3. 资源只输出media manifest明确授权URL；未知资源固定占位，图片lazy/srcset、附件下载、画板亮暗SVG
4. 上传视频使用controls/playsinline及封面；平台只本地封面并在点击后加载拥有者快照的embedURL，页面不预取第三方播放器
5. 内部ID链接仅使用同次RewriteInternalLinks映射，未知节点或mark拒绝，不静默丢弃

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：UNSUPPORTED_SCHEMA：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：content.preview；rule：published或preview授权
- 并发：纯函数，危险href拒绝，平台iframe仅白名单
- 幂等：同document/assets/renderer版本同输出
- 依赖：`public-site.markHref`、`public-site.attrString`、`public-site.attrInt`、`public-site.nodeText`、`public-site.assetFrom`、`public-site.safeLink`
- 其他依赖说明：media公开元数据

## public-site.RenderTiptapMarkdown

- 级别：参考
- 文件：`modules/public-site/internal/tiptap_markdown.go`
- 作用：Tiptap输出AI可读Markdown

```go
func RenderTiptapMarkdown(ctx context.Context, op OperationContext, in RenderTiptapMarkdownInput) (RenderTiptapMarkdownResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenderTiptapMarkdownInput` | Tiptap输出AI可读Markdown；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderTiptapMarkdownResult` | Tiptap输出AI可读Markdown成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按生产方块顺序输出段落、标题、嵌套列表、表格、代码、LaTeX与callout类型；多栏依序输出
2. 正文和链接转义，代码围栏大于原文最长连续反引号，保留原文
3. 资源只使用明确授权manifest；画板/视频用图片及已有说明，不临时生成描述
4. 内部ID链接使用同次RewriteInternalLinks映射；未知节点或mark失败，不静默丢弃

**错误**

- `UNSUPPORTED_SCHEMA` / HTTP 422：UNSUPPORTED_SCHEMA：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：service；rule：公开authoritative检查完成
- 并发：纯函数
- 幂等：同revision同Markdown
- 依赖：`public-site.markHref`、`public-site.markdownEscape`、`public-site.markdownDestination`、`public-site.codeFence`、`public-site.assetFrom`
- 其他依赖说明：media公开元数据

## public-site.markdownEscape

- 级别：参考
- 文件：`modules/public-site/internal/tiptap_markdown.go`
- 作用：转义Markdown正文

```go
func markdownEscape(value string) string
```

**输入**

无

**输出**

无

**步骤**

1. 转义Markdown正文

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.markdownDestination

- 级别：参考
- 文件：`modules/public-site/internal/tiptap_markdown.go`
- 作用：编码Markdown目标分隔符

```go
func markdownDestination(value string) string
```

**输入**

无

**输出**

无

**步骤**

1. 编码Markdown目标分隔符

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.codeFence

- 级别：参考
- 文件：`modules/public-site/internal/tiptap_markdown.go`
- 作用：选择不与源代码冲突的围栏

```go
func codeFence(value, character string) string
```

**输入**

无

**输出**

无

**步骤**

1. 选择不与源代码冲突的围栏

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：无

## public-site.markHref

- 级别：参考
- 文件：`modules/public-site/internal/tiptap_markdown.go`
- 作用：外链协议过滤及已核验ID映射读取

```go
func markHref(mark TiptapMark, links map[string]string) string
```

**输入**

无

**输出**

无

**步骤**

1. 外链协议过滤及已核验ID映射读取

**错误**

- 无

- 副作用：无
- 授权：permission：service；rule：仅本域已验证的公开读取或渲染调用链调用，不建立新的权限权威
- 并发：只读或纯函数，不修改共享业务状态
- 幂等：相同已校验输入同结果
- 依赖：`public-site.safeLink`

## public-site.RenderHero

- 级别：参考
- 文件：`modules/public-site/internal/hero.go`
- 作用：仅一屏默认主视觉

```go
func RenderHero(ctx context.Context, op OperationContext, in RenderHeroInput) (RenderHeroResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenderHeroInput` | 仅一屏默认主视觉；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderHeroResult` | 仅一屏默认主视觉成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. button0..3
2. 背景Welcome To默认不翻译但可启用
3. 遮罩0..100
4. 纯色/图/动图/video选择
5. video reduced-motion封面与图片preload
6. 不添加第二屏内容
7. 背景装饰文字、文字主题来自明确输入，不把空字符串改成默认；默认值由site-config生产方确定

**错误**

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous config读取
- 并发：纯渲染
- 幂等：同public config revision同输出
- 依赖：`public-site.safeLink`
- 其他依赖说明：site-config公开首页配置

## public-site.RenderAnnouncement

- 级别：参考
- 文件：`modules/public-site/internal/announcement.go`
- 作用：公告翻译横幅

```go
func RenderAnnouncement(ctx context.Context, op OperationContext, in RenderAnnouncementInput) (RenderAnnouncementResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenderAnnouncementInput` | 公告翻译横幅；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderAnnouncementResult` | 公告翻译横幅成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 按[start,end)窗口
2. 输出当前语言text安全link
3. 关闭按钮localStorage存announcementId，同一条不再显示
4. 无统计cookie

**错误**

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：客户端仅localStorage关闭标志
- 授权：permission：anonymous；rule：anonymous
- 并发：配置revision绑定rendercache
- 幂等：同公告/时间窗口同结果
- 依赖：`public-site.safeLink`

## public-site.BuildPreviousNext

- 级别：参考
- 文件：`modules/public-site/internal/navigation_neighbors.go`
- 作用：文末相邻文档

```go
func BuildPreviousNext(ctx context.Context, op OperationContext, in BuildPreviousNextInput) (BuildPreviousNextResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `BuildPreviousNextInput` | 文末相邻文档；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `BuildPreviousNextResult` | 文末相邻文档成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 只接受已由content公开门禁过滤的树
2. 按sortKey稳定DFS顺序返回前后各至多一项；无邻居为空数组，目标不存在返回NOT_FOUND

**错误**

- `INTEGRITY_ERROR` / HTTP 422：公开树重复/缺父/环（不可重试）
- `NOT_FOUND` / HTTP 404：目标节点不存在（不可重试）

- 副作用：无
- 授权：permission：service；rule：公开渲染内部
- 并发：纯函数
- 幂等：同tree修订同neighbors
- 依赖：`public-site.navigationOrder`

## public-site.RenderNotFound

- 级别：参考
- 文件：`modules/public-site/internal/http_errors.go`
- 作用：定制404页面

```go
func RenderNotFound(ctx context.Context, op OperationContext, in RenderNotFoundInput) (RenderNotFoundResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `RenderNotFoundInput` | 定制404页面；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `RenderNotFoundResult` | 定制404页面成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取可公开树
2. 输出文档树和没找到问问AI入口
3. AI未配置显示关闭原因
4. 不泄漏私有标题
5. content不可达503

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：重新authority导航
- 幂等：GET幂等
- 依赖：`public-site.BuildNavigation`、`site-config:readConfigObject`

## public-site.HandleAskShell

- 级别：约束
- 文件：`modules/public-site/internal/http_ask.go`
- 作用：独立问答页面外壳

```go
func HandleAskShell(ctx context.Context, op OperationContext, in HandleAskShellInput) (HandleAskShellResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleAskShellInput` | 独立问答页面外壳；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleAskShellResult` | 独立问答页面外壳成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. Go输出可访问语言外壳
2. 挂载qa同一会话Vue岛
3. 文档浮窗和独立页共享qa会话/当前块引用
4. 未配置AI显示缺项不调用模型

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无本域对话存储
- 授权：permission：anonymous；rule：anonymous
- 并发：qa拥有会话，不创建第二份会话
- 幂等：GET幂等
- 依赖：`site-config:readConfigObject`
- 其他依赖说明：浏览器问答岛经portal调用qa公开接口

## public-site.PrewarmPublication

- 级别：约束
- 文件：`modules/public-site/internal/prewarm.go`
- 作用：发布事件预渲染缓存

```go
func PrewarmPublication(ctx context.Context, op OperationContext, in PrewarmPublicationInput) (PrewarmPublicationResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `PrewarmPublicationInput` | 发布事件预渲染缓存；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `PrewarmPublicationResult` | 发布事件预渲染缓存成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 事件去重/epoch校验
2. 各启用语言RenderPublicPage并存修订缓存
3. 失败标派生任务错误不回滚正文
4. 撤回再验阻止旧缓存输出
5. 预热失败只标派生任务错误，公开请求可按同一不可变ref实时渲染，最终可见性门禁仍生效

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策
- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败（不可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：写派生cache/outbox
- 授权：permission：service；rule：service content事件
- 并发：每文档2并发，不重复付费调用
- 幂等：publicationHash/locale/rendererVersion幂等
- 依赖：`public-site.RenderPublicPage`

## public-site.HandleLLMsRoot

- 级别：约束
- 文件：`modules/public-site/internal/http_llms_root.go`
- 作用：根llms语言索引

```go
func HandleLLMsRoot(ctx context.Context, op OperationContext, in HandleLLMsRootInput) (HandleLLMsRootResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleLLMsRootInput` | 根llms语言索引；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleLLMsRootResult` | 根llms语言索引成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取当前启用语言，逐语言列出llms.txt和llms-full.txt
2. 使用可信publicBaseURL，不混合正文，不输出停用语言
3. 返回前重核配置与语言水位

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：公开配置revision一致
- 幂等：GET幂等
- 依赖：`site-config:readSiteConfig`、`localization:listLanguages`

## public-site.HandleLanguageSitemap

- 级别：约束
- 文件：`modules/public-site/internal/http_language_sitemap.go`
- 作用：每语言sitemap

```go
func HandleLanguageSitemap(ctx context.Context, op OperationContext, in HandleLanguageSitemapInput) (HandleLanguageSitemapResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 调用取消与deadline；不得携带正文到日志 | Go标准库context；用户取消/超时即停止新副作用 |
| `op` | `OperationContext` | 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权 | 公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游 |
| `in` | `HandleLanguageSitemapInput` | 每语言sitemap；字段见本域types.json对应$defs | 严格schema，拒未知字段；字段约束与默认参数见types/requirements |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `HandleLanguageSitemapResult` | 每语言sitemap成功结果；字段见types.json | 成功时满足全部required字段；失败返回零值且不泄漏半结果 |
| `error` | `error` | Go error转公共Problem；TypeScript拒绝Promise携带Problem | 成功nil；失败仅登记公共code，detail不含IP/提问/密钥 |

**步骤**

1. 读取当前语言规范URL并最终权威校验
2. 不超过10000项返回urlset；更多返回仅该语言的固定分片index
3. 撤回不保留旧URL，XML输出点转义

**错误**

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败（可重试）；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- 副作用：无
- 授权：permission：anonymous；rule：anonymous
- 并发：同公开修订集合稳定分片
- 幂等：GET幂等
- 依赖：`public-site.BuildSitemapEntries`、`public-site.sitemapURLSetXML`、`public-site.sitemapIndexXML`、`site-config:readSiteConfig`

## public-site.main

- 级别：约束
- 文件：`modules/public-site/cmd/public-site/main.go`
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
- 依赖：`public-site.BuildApplication`、`public-site.Serve`、`public-site.Shutdown`、`platform.RunService`

## public-site.BuildApplication

- 级别：约束
- 文件：`modules/public-site/internal/app/wire.go`
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
6. public-site装配public-site.ConsumeEvents为受lifecycle取消控制的订阅循环，Shutdown等待在途事件提交后停止，维护不写业务投影
7. 启动后按implementation.json backgroundTasks登记的周期运行后台函数；维护期间暂停业务循环
8. POST /v1/public-links:resolve真实绑定ResolvePublishedLink；静态AllowedCallers仅site-config且限纯服务主体；未实现时不得用stub或忽略缺handler启动
9. 完整装配须把同一SnapshotManager的AdmitBusiness/WriteWindow接真实HTTP、事件、worker与后台整在途scope；回调或清理/跨域封口未齐仍拒绝完整启动，不能仅计HTTP冒安全。

**错误**

- `VALIDATION` / HTTP 400：配置/缺路由绑定（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：迁移或journal校验失败（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：基础关键依赖、control区或服务身份信任锚不可用（可重试）

- 副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚
- 授权：permission：service；rule：进程启动控制面；Health/Ready为匿名安全状态；业务路由仍各自鉴权，初始化不调用模型
- 并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法
- 幂等：启动重入按journal和epoch恢复，不新建paid operation
- 依赖：`public-site.ConsumeEvents`、`site-config:registerActivation`、`public-site.RunWorker`、`platform.ApplyRestoreSwap`、`platform.ReadRuntime`、`platform.NewRequestValidator`、`public-site.ResolvePublishedLink`、`platform.NewSnapshotManager`、`platform.SnapshotManager.AdmitBusiness`、`platform.SnapshotManager.WriteWindow`
- 其他依赖说明：本域implementation全部x-handler；本域journal恢复函数；site-config.readRuntimeState（本域site-config读取自身权威）

## public-site.Serve

- 级别：约束
- 文件：`modules/public-site/internal/transport/server.go`
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
- 依赖：`public-site.BuildApplication`、`public-site.Shutdown`、`platform.WatchRuntime`、`platform.BuildOperationContext`、`platform.WriteProblem`
- 其他依赖说明：本域OpenAPI路由

## public-site.Shutdown

- 级别：约束
- 文件：`modules/public-site/internal/transport/server.go`
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

## public-site.Health

- 级别：约束
- 文件：`modules/public-site/internal/transport/server.go`
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

## public-site.Ready

- 级别：约束
- 文件：`modules/public-site/internal/transport/server.go`
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

## public-site.ConsumeEvents

- 级别：约束
- 文件：`modules/public-site/internal/events/consume.go`
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
- 依赖：`jobs:subscribeEvents`、`jobs:acknowledgeEvent`、`public-site.InvalidatePublication`、`platform.RunConsumer`

## public-site.RunWorker

- 级别：约束
- 文件：`modules/public-site/internal/worker.go`
- 作用：主动领取并执行本域登记的持久任务

```go
func RunWorker(ctx context.Context, op OperationContext, input WorkerLoopInput) (WorkerLoopResult, error)
```

**输入**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `ctx` | `context.Context` | 取消与截止传播；不得携带正文到日志 | 用户取消、进程退出或截止到达即停止新副作用 |
| `op` | `OperationContext` | 可信操作上下文 | 服务边界构建，禁止浏览器自造；dataEpoch等于site-config当前active epoch |
| `input` | `WorkerLoopInput` | 输入 | 必须通过对应schema |

**输出**

| 名称 | 类型 | 说明 | 约束 |
|---|---|---|---|
| `result` | `WorkerLoopResult` | 结果 | 无error时有效 |
| `error` | `Error` | 失败时result无效，返回标准Problem | 只使用登记错误码；detail不含正文、密钥或IP |

**步骤**

1. 进程启动后循环调用jobs:claimJob，只领取本域implementation.json jobTypes登记的类型；维护或屏障非released时暂停领取
2. 领取后调用jobs:startJob确认开始，按jobType调用登记的执行函数；每10秒jobs:heartbeatJob续30秒租约、上报进度并读取cancelRequested
3. 执行函数写回结果前调用jobs:checkLease核对fence仍有效；成功jobs:completeJob，失败jobs:failJob并给出可重试分类；收到取消在安全点停止并以cancelled结束
4. lease过期或fence不符立即停止写入；已派发的模型调用不重放，由网关结算或对账

**错误**

- `DEPENDENCY_UNAVAILABLE`：jobs不可用；退避后重试领取（可重试）

- 副作用：HTTP调用jobs租约接口；执行本域登记的任务函数
- 授权：permission：service；rule：本域后台worker以服务身份调用jobs；不接受外部请求触发
- 并发：每进程一个领取循环，并发不超过maxConcurrency；同一任务只由持有有效fence者写回
- 幂等：jobs按jobId与fence去重；执行函数自身按输入引用幂等
- 依赖：`jobs:claimJob`、`jobs:startJob`、`jobs:heartbeatJob`、`jobs:checkLease`、`jobs:completeJob`、`jobs:failJob`、`public-site.PrewarmPublication`、`platform.RunWorkerLoop`

## public-site.ReadSnapshotArchive

- 级别：约束
- 文件：`modules/public-site/internal/snapshot_archive.go`
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

## public-site.Islands

- 级别：约束
- 文件：`modules/public-site/web/src/site.tsx`
- 作用：公开页交互岛：一个 React 根通过 portal 接管页头操作（问答、语言、明暗）、窄屏文档树抽屉与问答区；另附代码复制、目录高亮、公式、访问信标等增强

```ts
function Islands(props: { header: HTMLElement | null; ask: HTMLElement | null }): React.JSX.Element
```

**输入**

无

**输出**

无

**步骤**

1. 读取页面下发的岛配置与界面文字
2. 页面正文、导航和目录由 Go 完整输出，脚本只增强交互

**错误**

- 无

- 副作用：浏览器内调用同源版本化接口
- 授权：页面可见性只是少展示；每个接口仍由服务端按当前授权校验
- 并发：单页面实例；切换页面时取消未完成的读取
- 幂等：写操作带 Idempotency-Key 与版本前提
- 依赖：无
