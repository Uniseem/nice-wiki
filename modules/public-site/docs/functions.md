# public-site 函数合同

共59函数，与implementation.json一一对应。这里包含所有已设计手写函数及UI交互；未来新增私有helper必须同时登记。输入和结果字段完整schema在types.json，无null默认业务对象。

## M06-F001 ResolveRoute

文件 `modules/public-site/internal/routing.go`。规范化语言与 slug

```go
func ResolveRoute(ctx context.Context, op OperationContext, in ResolveRouteInput) (ResolveRouteResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ResolveRouteInput` — 规范化语言与 slug；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ResolveRouteResult` — 规范化语言与 slug成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`rawPath` rawPath；UTF-8；不得为 null, `query` query；UTF-8；不得为 null, `defaultLocale` defaultLocale；UTF-8；不得为 null

成功结果字段：`locale` locale；UTF-8；不得为 null, `slug` slug；UTF-8；不得为 null, `redirectPath` redirectPath；UTF-8；不得为 null, `redirectStatus` redirectStatus 非负 int64；不用浮点数

步骤：

1. 拒绝NUL/反斜杠/重复编码
2. 只对根路径按语言cookie→Accept-Language→基准语言返回302
3. 带前缀页不因偏好跳转
4. 向content查层级slug旧路径映射，改名/移动返回301
5. 删除返回410或管理员指定替代301

错误：

- `VALIDATION` / HTTP 400：INVALID_ROUTE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：LOCALE_UNSUPPORTED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "公开请求", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数及只读 HTTP；禁止修改全局 locale。

幂等：同 path 和路由修订同结果。

依赖：content.resolvePublicPath、site-config.readConfigObject。

## M06-F002 SelectLocale

文件 `modules/public-site/internal/locale.go`。确定展示语言

```go
func SelectLocale(ctx context.Context, op OperationContext, in SelectLocaleInput) (SelectLocaleResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: SelectLocaleInput` — 确定展示语言；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SelectLocaleResult` — 确定展示语言成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`requested` requested；UTF-8；不得为 null, `available` 有序列表；空列表表示没有可用项, `fallback` fallback；UTF-8；不得为 null, `languageCookie` languageCookie；UTF-8；不得为 null, `acceptLanguage` acceptLanguage；UTF-8；不得为 null

成功结果字段：`selected` selected；UTF-8；不得为 null, `requiresRedirect` requiresRedirect 的明确真假结果，不可 null

步骤：

1. 已带语言前缀严格保留该语言
2. 根路径优先显式cookie
3. 解析Accept-Language q排序，zh-TW/HK→zh-Hant、zh-CN/SG→zh-Hans、pt-BR→pt
4. 无匹配基准语言
5. 翻译缺块使用旧译或原文并显式顶部提示

错误：

- `VALIDATION` / HTTP 400：LOCALE_UNSUPPORTED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "公开请求", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：相同语言集合同结果。

依赖：site-config.readConfigObject。

## M06-F003 CheckPublication

文件 `modules/public-site/internal/authority.go`。执行正文公开权威检查

```go
func CheckPublication(ctx context.Context, op OperationContext, in CheckPublicationInput) (CheckPublicationResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: CheckPublicationInput` — 执行正文公开权威检查；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: CheckPublicationResult` — 执行正文公开权威检查成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `locale` locale；UTF-8；不得为 null, `expectedRevision` expectedRevision 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`published` published 的明确真假结果，不可 null, `publicationRevision` publicationRevision 非负 int64；不用浮点数, `canonicalPath` canonicalPath；UTF-8；不得为 null, `authorityToken` authorityToken；UTF-8；不得为 null

步骤：

1. 检查当前 epoch
2. 调用 content 获取强一致状态
3. 撤回拒绝缓存输出

错误：

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "site.read", "rule": "服务身份 public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：每 HTTP 请求执行，不能跨请求复用状态 TTL。

幂等：只读，重试至请求总 deadline。

依赖：content.validatePublications。

## M06-F004 LoadPublishedBundle

文件 `modules/public-site/internal/content_client.go`。取得同一发布修订完整包

```go
func LoadPublishedBundle(ctx context.Context, op OperationContext, in LoadPublishedBundleInput) (LoadPublishedBundleResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: LoadPublishedBundleInput` — 取得同一发布修订完整包；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: LoadPublishedBundleResult` — 取得同一发布修订完整包成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `locale` locale；UTF-8；不得为 null, `publicationRevision` publicationRevision 非负 int64；不用浮点数, `authorityToken` authorityToken；UTF-8；不得为 null

成功结果字段：`document` TiptapDocument领域数据合同, `title` title；UTF-8；不得为 null, `updatedAt` updatedAt；UTF-8；不得为 null, `mediaRefs` 有序列表；空列表表示没有可用项, `untranslatedBlockIds` 有序列表；空列表表示没有可用项, `publication` 公共 PublishedRevisionRef 合同，遵守其字段和空值规则

步骤：

1. 携PublishedRevisionRef向content.readPublishedRevision
2. 取得Tiptap JSON并验证hash/schemaVersion和2MiB限制
3. localization提供同blockId旧译/原文回退及人工优先元数据，权威publication再验证

错误：

- `VERSION_MISMATCH` / HTTP 409：REVISION_CHANGED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：BODY_TOO_LARGE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "site.read", "rule": "服务身份 public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：content token 只绑定单修订；失配至多重走一次检查。

幂等：读取无写；不能回退旧正文。

依赖：content.readPublishedRevision、localization.readTranslation、content.validatePublications。

## M06-F005 ParseMarkdown

文件 `modules/public-site/internal/markdown.go`。构建确定性 Markdown AST

```go
func ParseMarkdown(ctx context.Context, op OperationContext, in ParseMarkdownInput) (ParseMarkdownResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ParseMarkdownInput` — 构建确定性 Markdown AST；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ParseMarkdownResult` — 构建确定性 Markdown AST成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`body` body；UTF-8；不得为 null, `maxBytes` maxBytes 非负 int64；不用浮点数

成功结果字段：`ast` ASTNode领域数据合同, `headings` 有序列表；空列表表示没有可用项, `links` 有序列表；空列表表示没有可用项

步骤：

1. 校验 UTF-8 和大小
2. 禁用原始 HTML 执行
3. 解析 fenced code/表格/脚注/数学占位并保留源码位置

错误：

- `VALIDATION` / HTTP 400：INVALID_MARKDOWN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：BODY_TOO_LARGE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "content.preview", "rule": "已通过 published 或 preview 授权", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：请求隔离 parser；深度上限 64。

幂等：相同 body/parserVersion 同 AST。

依赖：无。

## M06-F006 SanitizeHTML

文件 `modules/public-site/internal/sanitize.go`。移除危险结构及协议

```go
func SanitizeHTML(ctx context.Context, op OperationContext, in SanitizeHTMLInput) (SanitizeHTMLResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: SanitizeHTMLInput` — 移除危险结构及协议；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SanitizeHTMLResult` — 移除危险结构及协议成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`html` html；UTF-8；不得为 null, `baseURL` baseURL；UTF-8；不得为 null

成功结果字段：`safeHTML` safeHTML；UTF-8；不得为 null

步骤：

1. 允许静态排版标签和限定属性
2. 移除 script/style/事件属性
3. 链接仅允许 http/https/mailto 和同源根路径

错误：

- `VALIDATION` / HTTP 400：INVALID_BASE_URL：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "渲染内部调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数，无共享 DOM。

幂等：同 html/config 同结果。

依赖：无。

## M06-F007 BuildTOC

文件 `modules/public-site/internal/toc.go`。生成稳定锚点目录

```go
func BuildTOC(ctx context.Context, op OperationContext, in BuildTOCInput) (BuildTOCResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildTOCInput` — 生成稳定锚点目录；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildTOCResult` — 生成稳定锚点目录成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`headings` 有序列表；空列表表示没有可用项, `maxDepth` maxDepth 非负 int64；不用浮点数

成功结果字段：`items` 有序列表；空列表表示没有可用项

步骤：

1. 标题H1不进入右侧目录
2. 正文H2-H4逐项使用原稳定blockId锚点
3. 同锚点冲突拒INTEGRITY_ERROR
4. 目录包含所有标题且正文不折叠

错误：

- `VALIDATION` / HTTP 400：INVALID_TOC_DEPTH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "渲染内部调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数；最多 512 项。

幂等：同 headings 同锚点。

依赖：无。

## M06-F008 RewriteInternalLinks

文件 `modules/public-site/internal/links.go`。解析跨文档链接

```go
func RewriteInternalLinks(ctx context.Context, op OperationContext, in RewriteInternalLinksInput) (RewriteInternalLinksResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RewriteInternalLinksInput` — 解析跨文档链接；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RewriteInternalLinksResult` — 解析跨文档链接成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`links` 有序列表；空列表表示没有可用项, `locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`rewritten` 有序列表；空列表表示没有可用项, `unavailable` 有序列表；空列表表示没有可用项

步骤：

1. 批量询问 content 公开路由
2. 仅输出可公开目标
3. 缺失语言保留文档链接但显示不可用提示

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "site.read", "rule": "public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：每批 100 目标，4 并发，总 deadline 2500ms。

幂等：读取无副作用。

依赖：content.resolvePublicPath。

## M06-F009 ResolvePublicMedia

文件 `modules/public-site/internal/media_client.go`。取得公开资源清单

```go
func ResolvePublicMedia(ctx context.Context, op OperationContext, in ResolvePublicMediaInput) (ResolvePublicMediaResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ResolvePublicMediaInput` — 取得公开资源清单；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ResolvePublicMediaResult` — 取得公开资源清单成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`mediaRefs` 有序列表；空列表表示没有可用项, `publicationRevision` publicationRevision 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`assets` 有序列表；空列表表示没有可用项

步骤：

1. media 校验文档引用和发布修订
2. 拒绝草稿资源
3. 固定 width/height/alt 和安全下载 URL

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：MEDIA_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：MEDIA_NOT_PUBLIC：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "site.read", "rule": "public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：4 并发，缺图输出尺寸占位且禁止私有 URL。

幂等：同引用修订可缓存元数据；授权仍逐响应检查。

依赖：media.buildDeliveryManifest。

## M06-F010 BuildNavigation

文件 `modules/public-site/internal/navigation.go`。构建可访问公开导航树

```go
func BuildNavigation(ctx context.Context, op OperationContext, in BuildNavigationInput) (BuildNavigationResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildNavigationInput` — 构建可访问公开导航树；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildNavigationResult` — 构建可访问公开导航树成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧, `maxNodes` maxNodes 非负 int64；不用浮点数

成功结果字段：`tree` 有序列表；空列表表示没有可用项, `revision` revision 非负 int64；不用浮点数

步骤：

1. content 提供权威可公开树
2. 按 stable sortKey 排序
3. 过滤被撤回/权限限制节点后折叠空组

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：NAV_LIMIT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新派生导航投影。

授权：{"permission": "site.read", "rule": "public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：投影修订 CAS，不覆盖新修订。

幂等：以 epoch/locale/sourceRevision 去重。

依赖：content.readPublicTree。

## M06-F011 BuildBreadcrumbs

文件 `modules/public-site/internal/breadcrumbs.go`。生成面包屑

```go
func BuildBreadcrumbs(ctx context.Context, op OperationContext, in BuildBreadcrumbsInput) (BuildBreadcrumbsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildBreadcrumbsInput` — 生成面包屑；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildBreadcrumbsResult` — 生成面包屑成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `tree` 有序列表；空列表表示没有可用项

成功结果字段：`items` 有序列表；空列表表示没有可用项

步骤：

1. 定位节点父链
2. 检测环
3. 输出首页及祖先规范链接

错误：

- `VALIDATION` / HTTP 400：NAV_CYCLE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：NODE_NOT_LISTED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "渲染内部调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数，父链最多 32。

幂等：同树修订同结果。

依赖：无。

## M06-F012 BuildCanonical

文件 `modules/public-site/internal/seo.go`。生成规范地址

```go
func BuildCanonical(ctx context.Context, op OperationContext, in BuildCanonicalInput) (BuildCanonicalResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildCanonicalInput` — 生成规范地址；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildCanonicalResult` — 生成规范地址成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`baseURL` baseURL；UTF-8；不得为 null, `canonicalPath` canonicalPath；UTF-8；不得为 null

成功结果字段：`canonicalURL` canonicalURL；UTF-8；不得为 null

步骤：

1. 仅信任 site-config 配置域名
2. 移除追踪 query
3. 拼接语言路径并 HTML 属性转义

错误：

- `VALIDATION` / HTTP 400：INVALID_BASE_URL：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：INVALID_ROUTE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "渲染内部调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：相同输入同结果。

依赖：site-config.readConfigObject。

## M06-F013 BuildHreflang

文件 `modules/public-site/internal/hreflang.go`。输出已有公开语言版本

```go
func BuildHreflang(ctx context.Context, op OperationContext, in BuildHreflangInput) (BuildHreflangResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildHreflangInput` — 输出已有公开语言版本；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildHreflangResult` — 输出已有公开语言版本成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧, `baseURL` baseURL；UTF-8；不得为 null

成功结果字段：`alternates` 有序列表；空列表表示没有可用项

步骤：

1. content验证当前公开版本
2. 每语言canonical
3. x-default固定指向站点根/
4. 翻译未完成仍可旧译/原文回退展示，不能把语言停用版本列入

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "site.read", "rule": "public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：同请求批量权威查询。

幂等：只读，撤回语言不保留旧 alternate。

依赖：localization.listLanguages。

## M06-F014 BuildSEOMetadata

文件 `modules/public-site/internal/metadata.go`。输出 SEO 和分享元数据

```go
func BuildSEOMetadata(ctx context.Context, op OperationContext, in BuildSEOMetadataInput) (BuildSEOMetadataResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildSEOMetadataInput` — 输出 SEO 和分享元数据；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildSEOMetadataResult` — 输出 SEO 和分享元数据成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`title` title；UTF-8；不得为 null, `description` description；UTF-8；不得为 null, `canonicalURL` canonicalURL；UTF-8；不得为 null, `alternates` 有序列表；空列表表示没有可用项, `shareImage` shareImage；UTF-8；不得为 null, `breadcrumbs` 有序列表；空列表表示没有可用项, `markdownURL` markdownURL；UTF-8；不得为 null

成功结果字段：`metaHTML` metaHTML；UTF-8；不得为 null

步骤：

1. 使用content已生成或人工优先且已翻译SEO description
2. 分享图优先文档否则站点默认
3. 转义title/description
4. 输出OG/canonical/hreflang/x-default/JSON-LD面包屑与text/markdown alternate，不临时调用模型

错误：

- `VALIDATION` / HTTP 400：INVALID_CANONICAL：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "渲染内部调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：同字段同结果。

依赖：无。

## M06-F015 ComposeHTML

文件 `modules/public-site/internal/html.go`。构成完整响应文档

```go
func ComposeHTML(ctx context.Context, op OperationContext, in ComposeHTMLInput) (ComposeHTMLResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ComposeHTMLInput` — 构成完整响应文档；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ComposeHTMLResult` — 构成完整响应文档成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`safeHTML` safeHTML；UTF-8；不得为 null, `title` title；UTF-8；不得为 null, `toc` 有序列表；空列表表示没有可用项, `navigation` 有序列表；空列表表示没有可用项, `metadata` metadata；UTF-8；不得为 null

成功结果字段：`html` html；UTF-8；不得为 null

步骤：

1. 输出Go完整三栏HTML：顶栏AI/语言/主题，左侧当前分支展开树，正文与右侧TOC
2. 手机将树/TOC放抽屉
3. 文末上一篇/下一篇按树顺序
4. 页脚版权/链接/ICP/默认源代码链接
5. 挂Vue问答浮窗和语言建议，正文无JS仍完整且不折叠

错误：

- `VALIDATION` / HTTP 400：INVALID_TEMPLATE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "渲染内部调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：模板启动时编译；每请求独立缓冲。

幂等：同输入与 themeRevision 同输出结构。

依赖：site-config.readConfigObject。

## M06-F016 BuildCacheKey

文件 `modules/public-site/internal/cache_key.go`。固定派生缓存身份

```go
func BuildCacheKey(ctx context.Context, op OperationContext, in BuildCacheKeyInput) (BuildCacheKeyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildCacheKeyInput` — 固定派生缓存身份；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildCacheKeyResult` — 固定派生缓存身份成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `publicationRevision` publicationRevision 非负 int64；不用浮点数, `locale` locale；UTF-8；不得为 null, `themeRevision` themeRevision 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`key` key；UTF-8；不得为 null

步骤：

1. 按字段长度编码序列
2. 加入 rendererVersion
3. SHA-256 生成路径

错误：

- `INTERNAL` / HTTP 500：INVALID_REVISION：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "内部调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：相同身份同 key。

依赖：无。

## M06-F017 ReadRenderCache

文件 `modules/public-site/internal/cache_store.go`。读取派生 HTML

```go
func ReadRenderCache(ctx context.Context, op OperationContext, in ReadRenderCacheInput) (ReadRenderCacheResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ReadRenderCacheInput` — 读取派生 HTML；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ReadRenderCacheResult` — 读取派生 HTML成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`key` key；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`hit` hit 的明确真假结果，不可 null, `html` html；UTF-8；不得为 null, `sha256` sha256；UTF-8；不得为 null

步骤：

1. 检查索引 epoch/TTL
2. 读取文件校验 hash
3. 损坏删除索引并记 errorCode

错误：

- `INTEGRITY_ERROR` / HTTP 422：CACHE_CORRUPT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新 LRU/损坏清理，不含正文日志。

授权：{"permission": "service", "rule": "内部已授权调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：SQLite 读+独立文件读；cache miss 不算错误。

幂等：读结果可重复；损坏清理幂等。

依赖：私有 SQLite/cache 文件。

## M06-F018 WriteRenderCache

文件 `modules/public-site/internal/cache_store.go`。原子安装派生缓存文件

```go
func WriteRenderCache(ctx context.Context, op OperationContext, in WriteRenderCacheInput) (WriteRenderCacheResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: WriteRenderCacheInput` — 原子安装派生缓存文件；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: WriteRenderCacheResult` — 原子安装派生缓存文件成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`key` key；UTF-8；不得为 null, `html` html；UTF-8；不得为 null, `sha256` sha256；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`stored` stored 的明确真假结果，不可 null

步骤：

1. 写临时文件并 fsync
2. rename 至内容哈希路径
3. 事务 upsert 索引
4. 失败临时文件由恢复扫描清理

错误：

- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `STALE_RESULT` / HTTP 409：STALE_RENDER：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写缓存文件与索引。

授权：{"permission": "service", "rule": "内部已授权调用", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：key 单飞锁和 epoch CAS；不宣称文件/SQLite 同事务。

幂等：同 key/hash upsert 同结果。

依赖：私有 SQLite/cache 文件。

## M06-F019 InvalidatePublication

文件 `modules/public-site/internal/invalidation.go`。处理发布和撤回事件

```go
func InvalidatePublication(ctx context.Context, op OperationContext, in InvalidatePublicationInput) (InvalidatePublicationResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: InvalidatePublicationInput` — 处理发布和撤回事件；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: InvalidatePublicationResult` — 处理发布和撤回事件成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`eventId` eventId；UTF-8；不得为 null, `documentId` documentId；UTF-8；不得为 null, `publicationRevision` publicationRevision 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`invalidated` invalidated 非负 int64；不用浮点数

步骤：

1. 拒绝旧 epoch
2. inbox 原子去重
3. 删除该文档低于等于失效修订缓存和导航投影

错误：

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：INVALID_EVENT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写 inbox/索引并清理文件。

授权：{"permission": "service", "rule": "签名 content 事件", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：事务 inbox+索引；重复或乱序不复活缓存。

幂等：eventId 与 sourceRevision 去重。

依赖：content.published/content.withdrawn。

## M06-F020 RenderPublicPage

文件 `modules/public-site/internal/renderer.go`。编排公开正文输出

```go
func RenderPublicPage(ctx context.Context, op OperationContext, in RenderPublicPageInput) (RenderPublicPageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RenderPublicPageInput` — 编排公开正文输出；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RenderPublicPageResult` — 编排公开正文输出成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`html` html；UTF-8；不得为 null, `etag` etag；UTF-8；不得为 null, `publicationRevision` publicationRevision 非负 int64；不用浮点数

步骤：

1. 权威publication校验
2. 读取同修订预渲染缓存或权威TiptapJSON/译文
3. RenderTiptapHTML完整渲染
4. 发响应前再次权威校验
5. 主题修订变化或撤回不输出旧缓存
6. 返回no-cache/ETag，前端仅渐进增强

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：REVISION_CHANGED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEADLINE_EXCEEDED` / HTTP 504：RENDER_TIMEOUT：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：可写派生缓存；已撤回内容不送响应。

授权：{"permission": "anonymous", "rule": "公开请求", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：5s 总 deadline；撤回后新检查不通过；检查后流式正文禁用。

幂等：相同修订只生成一个缓存对象。

依赖：content.validatePublications、LoadPublishedBundle、RenderTiptapHTML、ComposeHTML。

## M06-F021 HandlePage

文件 `modules/public-site/internal/http_pages.go`。页面 HTTP 入口

```go
func HandlePage(ctx context.Context, op OperationContext, in HandlePageInput) (HandlePageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandlePageInput` — 页面 HTTP 入口；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandlePageResult` — 页面 HTTP 入口成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`path` path；UTF-8；不得为 null, `ifNoneMatch` ifNoneMatch；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `headers` 有序列表；空列表表示没有可用项, `body` body；UTF-8；不得为 null

步骤：

1. 解析路由
2. render 并最终权威校验
3. 校验成功且 etag 相等才 304，否则完整 200

错误：

- `VALIDATION` / HTTP 400：INVALID_ROUTE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：非阻塞匿名 beacon；不存请求 IP。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：请求缓冲到最终授权完成；不可先写 header。

幂等：GET 可重复；statusCode 不缓存安全结论。

依赖：ResolveRoute、RenderPublicPage、analytics.ingestBeacon。

## M06-F022 HandleHome

文件 `modules/public-site/internal/http_home.go`。站点首页

```go
func HandleHome(ctx context.Context, op OperationContext, in HandleHomeInput) (HandleHomeResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleHomeInput` — 站点首页；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleHomeResult` — 站点首页成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `html` html；UTF-8；不得为 null

步骤：

1. 读site-config主视觉翻译配置
2. Go输出仅一屏hero，主标题默认站点名、半透明Welcome To、副标题、0..3按钮、背景纯色/图片/动图/视频与0..100遮罩
3. 视频autoplay muted loop playsinline，reduced-motion只显示封面
4. 资源Range交media

错误：

- `VALIDATION` / HTTP 400：SITE_NOT_INITIALIZED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：可写首页派生缓存。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：主题配置 revision 绑定 cacheKey。

幂等：GET 幂等。

依赖：site-config.readConfigObject、media.buildDeliveryManifest、RenderHero。

## M06-F023 HandleNavigation

文件 `modules/public-site/internal/http_navigation.go`。公开导航 HTTP

```go
func HandleNavigation(ctx context.Context, op OperationContext, in HandleNavigationInput) (HandleNavigationResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleNavigationInput` — 公开导航 HTTP；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleNavigationResult` — 公开导航 HTTP成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `tree` 有序列表；空列表表示没有可用项, `etag` etag；UTF-8；不得为 null

步骤：

1. 权威取得可公开树
2. 构建过滤树
3. 按修订生成 etag，304 同样权威查验

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：NAV_LIMIT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新投影。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：不能只返回历史投影。

幂等：GET 幂等。

依赖：BuildNavigation。

## M06-F024 HandleMarkdown

文件 `modules/public-site/internal/http_markdown.go`。导出公开 Markdown

```go
func HandleMarkdown(ctx context.Context, op OperationContext, in HandleMarkdownInput) (HandleMarkdownResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleMarkdownInput` — 导出公开 Markdown；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleMarkdownResult` — 导出公开 Markdown成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `slug` slug；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `markdown` markdown；UTF-8；不得为 null, `etag` etag；UTF-8；不得为 null

步骤：

1. 公开路径后缀.md→content规范路由
2. 读取已完成Tiptap JSON
3. RenderTiptapMarkdown输出代码/表格/画板视频图片链接+文字描述
4. 最终publication权威验证
5. text/markdown no-cache+etag
6. AI已知爬虫单独计数

错误：

- `NOT_FOUND` / HTTP 404：NOT_PUBLISHED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：同 HTML 5s deadline 与最终状态校验。

幂等：GET 幂等。

依赖：ResolveRoute、LoadPublishedBundle、RenderTiptapMarkdown、content.validatePublications、analytics.recordCrawlerAccess。

## M06-F025 HandleRobots

文件 `modules/public-site/internal/http_robots.go`。机器人规则

```go
func HandleRobots(ctx context.Context, op OperationContext, in HandleRobotsInput) (HandleRobotsResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleRobotsInput` — 机器人规则；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleRobotsResult` — 机器人规则成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `text` text；UTF-8；不得为 null

步骤：

1. 读公开索引策略
2. 明确Allow主流AI爬虫（GPTBot/ChatGPT-User/ClaudeBot/PerplexityBot）
3. 拒preview/admin私有接口
4. 列每语言sitemap总索引
5. 不能把功能性cookie变成统计同意弹窗

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：SITE_CONFIG_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：配置不可达返回 503 禁止旧配置承诺。

幂等：GET 幂等。

依赖：site-config.readConfigObject。

## M06-F026 BuildLLMsIndex

文件 `modules/public-site/internal/llms.go`。生成模型可读公开索引

```go
func BuildLLMsIndex(ctx context.Context, op OperationContext, in BuildLLMsIndexInput) (BuildLLMsIndexResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildLLMsIndexInput` — 生成模型可读公开索引；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildLLMsIndexResult` — 生成模型可读公开索引成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧, `maxBytes` maxBytes 非负 int64；不用浮点数

成功结果字段：`text` text；UTF-8；不得为 null, `sourceRevision` sourceRevision 非负 int64；不用浮点数

步骤：

1. 权威枚举公开文档
2. 按导航序输出标题/摘要/Markdown 地址
3. 不出现草稿元数据

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VALIDATION` / HTTP 400：EXPORT_LIMIT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：可写派生文本缓存。

授权：{"permission": "site.read", "rule": "public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：每项在输出前批量 authority 校验。

幂等：同公开修订集合可复用缓存。

依赖：content.readPublicTree。

## M06-F027 HandleLLMsIndex

文件 `modules/public-site/internal/http_llms.go`。llms.txt HTTP

```go
func HandleLLMsIndex(ctx context.Context, op OperationContext, in HandleLLMsIndexInput) (HandleLLMsIndexResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleLLMsIndexInput` — llms.txt HTTP；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleLLMsIndexResult` — llms.txt HTTP成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `text` text；UTF-8；不得为 null

步骤：

1. 读取公开 AI 索引开关
2. 生成已授权索引
3. 禁用时 404

错误：

- `VALIDATION` / HTTP 400：INDEX_DISABLED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：权威批量校验不使用过期索引安全结论。

幂等：GET 幂等。

依赖：BuildLLMsIndex、site-config.readConfigObject。

## M06-F028 HandleLLMsFull

文件 `modules/public-site/internal/http_llms.go`。输出合并正文或分片索引

```go
func HandleLLMsFull(ctx context.Context, op OperationContext, in HandleLLMsFullInput) (HandleLLMsFullResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleLLMsFullInput` — 输出合并正文或分片索引；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleLLMsFullResult` — 输出合并正文或分片索引成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `text` text；UTF-8；不得为 null, `shards` 有序列表；空列表表示没有可用项

步骤：

1. 权威枚举并读取固定修订
2. 累计 8MiB 时返回分片链接索引
3. 最终批量验证每项仍 published

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：REVISION_CHANGED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：派生文本缓存。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：不得边读边发送；总时限 5s，超限 503。

幂等：不产生付费调用。

依赖：LoadPublishedBundle、BuildLLMsIndex。

## M06-F029 BuildSitemapEntries

文件 `modules/public-site/internal/sitemap.go`。生成规范 sitemap 项

```go
func BuildSitemapEntries(ctx context.Context, op OperationContext, in BuildSitemapEntriesInput) (BuildSitemapEntriesResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildSitemapEntriesInput` — 生成规范 sitemap 项；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildSitemapEntriesResult` — 生成规范 sitemap 项成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`entries` 有序列表；空列表表示没有可用项, `revision` revision 非负 int64；不用浮点数

步骤：

1. content 列公开可索引页面
2. 按 canonicalURL 字典排序
3. lastmod 采用实际发布修订时间

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "site.read", "rule": "public-site.read-public", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：最终批量 authority 校验。

幂等：同公开集合同分片边界。

依赖：content.readPublicTree。

## M06-F030 HandleSitemapIndex

文件 `modules/public-site/internal/http_sitemap.go`。sitemap 索引 HTTP

```go
func HandleSitemapIndex(ctx context.Context, op OperationContext, in HandleSitemapIndexInput) (HandleSitemapIndexResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleSitemapIndexInput` — sitemap 索引 HTTP；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleSitemapIndexResult` — sitemap 索引 HTTP成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `xml` xml；UTF-8；不得为 null

步骤：

1. 获取有公开内容语言
2. 每 10000 URL 固定分片
3. 只输出当前 epoch 分片地址

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：可写派生 XML 缓存。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：不使用已撤回分片清单。

幂等：GET 幂等。

依赖：BuildSitemapEntries。

## M06-F031 HandleSitemapShard

文件 `modules/public-site/internal/http_sitemap.go`。单分片 sitemap

```go
func HandleSitemapShard(ctx context.Context, op OperationContext, in HandleSitemapShardInput) (HandleSitemapShardResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleSitemapShardInput` — 单分片 sitemap；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleSitemapShardResult` — 单分片 sitemap成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `shard` shard 非负 int64；不用浮点数, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `xml` xml；UTF-8；不得为 null

步骤：

1. 验证非负 shard
2. 生成当前稳定条目集合
3. 越界 404 并 XML 转义输出

错误：

- `VALIDATION` / HTTP 400：INVALID_SHARD：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：每响应权威校验所含条目。

幂等：GET 幂等。

依赖：BuildSitemapEntries。

## M06-F032 IssuePreviewToken

文件 `modules/public-site/internal/preview.go`。签发草稿预览授权

```go
func IssuePreviewToken(ctx context.Context, op OperationContext, in IssuePreviewTokenInput) (IssuePreviewTokenResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: IssuePreviewTokenInput` — 签发草稿预览授权；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: IssuePreviewTokenResult` — 签发草稿预览授权成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `revision` revision 非负 int64；不用浮点数, `actorId` actorId；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`token` token；UTF-8；不得为 null, `expiresAt` RFC3339 UTC 时间；不含 null

步骤：

1. identity.checkAuthorization核content.preview
2. 委托content.createPreviewGrant绑定当前已登录admin/精确draftRevision/epoch
3. 直接返回拥有者token与expiry，不创建第二份授权真相
4. 刷新可在expiry前重复本人预览

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：REVISION_NOT_FOUND：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：content拥有预览grant，本域不存token原文。

授权：{"permission": "content.preview", "rule": "identity session + content.preview", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：随机 256-bit token；每 token 一次消费。

幂等：同operationId由content复用grant；有效期内重复GET允许。

依赖：identity.checkAuthorization、content.createPreviewGrant。

## M06-F033 ValidatePreviewToken

文件 `modules/public-site/internal/preview.go`。原子消费预览 token

```go
func ValidatePreviewToken(ctx context.Context, op OperationContext, in ValidatePreviewTokenInput) (ValidatePreviewTokenResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidatePreviewTokenInput` — 原子消费预览 token；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidatePreviewTokenResult` — 原子消费预览 token成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`token` token；UTF-8；不得为 null, `actorId` actorId；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`documentId` documentId；UTF-8；不得为 null, `revision` revision 非负 int64；不用浮点数

步骤：

1. 本域解析token大小与格式
2. 调用content.validatePreviewGrant核本人会话/epoch/expiry/精确draft
3. content返回授权draft数据
4. 不消费一次性状态，不因刷新迫使用户重领

错误：

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "签入身份必须一致", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：SQLite CAS；重放不可通过。

幂等：同token/actor在有效期内重复验证只读安全。

依赖：content.validatePreviewGrant。

## M06-F034 RenderPreview

文件 `modules/public-site/internal/preview_renderer.go`。渲染受保护修订

```go
func RenderPreview(ctx context.Context, op OperationContext, in RenderPreviewInput) (RenderPreviewResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RenderPreviewInput` — 渲染受保护修订；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RenderPreviewResult` — 渲染受保护修订成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`previewToken` previewToken；UTF-8；不得为 null, `actorId` actorId；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`html` html；UTF-8；不得为 null

步骤：

1. content.validatePreviewGrant取绑定本人已授权精确draftJSON/metadata
2. RenderTiptapHTML同公开样式
3. media.buildDeliveryManifest携previewGrant输出受保护资源
4. noindex/no-store，绝不公开cache

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：REVISION_NOT_FOUND：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无公开缓存写入。

授权：{"permission": "content.preview", "rule": "content.preview", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：独立预览缓冲；禁用公开导航/private 标题混入。

幂等：只读重渲染无需模型。

依赖：content.validatePreviewGrant、media.buildDeliveryManifest、RenderTiptapHTML。

## M06-F035 HandlePreview

文件 `modules/public-site/internal/http_preview.go`。预览 HTTP 入口

```go
func HandlePreview(ctx context.Context, op OperationContext, in HandlePreviewInput) (HandlePreviewResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandlePreviewInput` — 预览 HTTP 入口；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandlePreviewResult` — 预览 HTTP 入口成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`token` token；UTF-8；不得为 null, `actorId` actorId；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `headers` 有序列表；空列表表示没有可用项, `html` html；UTF-8；不得为 null

步骤：

1. 已登录identity会话+token
2. ValidatePreviewToken
3. RenderPreview
4. CSP/noindex/no-store输出
5. 刷新不消费许可且每次content重验本人草稿权限

错误：

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：CONTENT_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无本域token消费状态。

授权：{"permission": "service", "rule": "identity session + token", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：不允许 CDN 缓存，content 不可达返回 503。

幂等：GET同actor/token在有效期重复只读安全。

依赖：ValidatePreviewToken、RenderPreview。

## M06-F036 RecordAnonymousView

文件 `modules/public-site/internal/beacon.go`。AI爬虫Markdown访问单独计数

```go
func RecordAnonymousView(ctx context.Context, op OperationContext, in RecordAnonymousViewInput) (RecordAnonymousViewResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RecordAnonymousViewInput` — AI爬虫Markdown访问单独计数；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RecordAnonymousViewResult` — AI爬虫Markdown访问单独计数成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `locale` locale；UTF-8；不得为 null, `userAgent` userAgent；UTF-8；不得为 null, `status` status 非负 int64；不用浮点数, `elapsedMs` elapsedMs 非负 int64；不用浮点数

成功结果字段：`accepted` accepted 的明确真假结果，不可 null

步骤：

1. 仅已知AI爬虫成功Markdown请求进入crawler计数
2. 只传crawlerFamily/document/locale/状态桶
3. 普通HTML页面访问由浏览器sendBeacon一次发送，不在服务端双计
4. 不传IP/UA原文

错误：

- `VALIDATION` / HTTP 400：BEACON_DROPPED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无本地访问身份历史。

授权：{"permission": "service", "rule": "内部公开响应流程", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：缓冲 1024 条，满丢弃不影响页面。

幂等：beaconId 去重由 analytics 执行。

依赖：analytics.recordCrawlerAccess。

## M06-F037 CreateSnapshot

文件 `modules/public-site/internal/snapshot.go`。创建本域一致快照

```go
func CreateSnapshot(ctx context.Context, op OperationContext, request SnapshotExportRequest) (SnapshotExportResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotExportRequest` — 创建本域一致快照；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotExportResult` — 创建本域一致快照成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`backupId` #/$defs/Identifier, `barrierId` #/$defs/Identifier, `dataEpoch` #/$defs/Identifier, `includeSecrets` boolean

成功结果字段：`snapshot` #/$defs/SnapshotRef, `checksum` #/$defs/Hash

步骤：

1. 验证 jobs barrier 已达安全点
2. checkpoint SQLite
3. 复制配置和索引，缓存 HTML/token 排除

错误：

- `LOCKED` / HTTP 409：BARRIER_REQUIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTERNAL` / HTTP 500：SNAPSHOT_IO：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写临时快照目录。

授权：{"permission": "service", "rule": "data-protection.snapshot", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：维护 barrier 下禁止索引写入。

幂等：snapshotId 重复返回同 manifest。

依赖：jobs.coordinateBarrier。

## M06-F038 ValidateImport

文件 `modules/public-site/internal/import.go`。验证待恢复快照

```go
func ValidateImport(ctx context.Context, op OperationContext, request SnapshotValidateRequest) (SnapshotValidation, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotValidateRequest` — 验证待恢复快照；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotValidation` — 验证待恢复快照成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`snapshot` #/$defs/SnapshotRef, `targetDataEpoch` #/$defs/Identifier

成功结果字段：`valid` boolean, `issues` array, `migrationPlanId` #/$defs/Identifier

步骤：

1. 校验 hash/schema/service
2. 扫描引用均为派生索引
3. 清空 token 和缓存，不查询旧世界正文

错误：

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：只写隔离验证报告。

授权：{"permission": "service", "rule": "data-protection.restore", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：隔离目录不可被公开 reader 打开。

幂等：restoreId/hash 去重。

依赖：无。

## M06-F039 PrepareImport

文件 `modules/public-site/internal/import.go`。准备隔离数据库

```go
func PrepareImport(ctx context.Context, op OperationContext, request SnapshotPrepareRequest) (SnapshotStageResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotPrepareRequest` — 准备隔离数据库；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotStageResult` — 准备隔离数据库成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`restoreId` #/$defs/Identifier, `snapshot` #/$defs/SnapshotRef, `barrierId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier

成功结果字段：`stageId` #/$defs/Identifier, `restoreId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier, `ready` boolean

步骤：

1. 调用校验
2. 复制隔离 schema
3. 所有派生缓存为空
4. 记录 prepared token

错误：

- `INTEGRITY_ERROR` / HTTP 422：IMPORT_INVALID：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTERNAL` / HTTP 500：DISK_FULL：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写隔离 DB。

授权：{"permission": "service", "rule": "data-protection.restore", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：站点维护中执行；不切活跃指针。

幂等：restoreId/hash/targetEpoch 幂等。

依赖：ValidateImport。

## M06-F040 ActivateImport

文件 `modules/public-site/internal/import.go`。激活已全局提交恢复代次

```go
func ActivateImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotCommitRequest` — 激活已全局提交恢复代次；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotApplyResult` — 激活已全局提交恢复代次成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`stageId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier, `barrierId` #/$defs/Identifier

成功结果字段：`stageId` #/$defs/Identifier, `dataEpoch` #/$defs/Identifier, `state` enum

步骤：

1. 验证恢复 commit 决策签名
2. 原子活跃 DB 指针切换
3. cache 清空并恢复事件 inbox 代次

错误：

- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `NOT_FOUND` / HTTP 404：PREPARE_NOT_FOUND：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：切本域存储指针。

授权：{"permission": "service", "rule": "data-protection.restore", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：全站维护持续至所有域 activated。

幂等：同 decisionId 重放返回 activated。

依赖：data-protection.getRestoreDecision。

## M06-F041 AbortImport

文件 `modules/public-site/internal/import.go`。清理未提交导入

```go
func AbortImport(ctx context.Context, op OperationContext, request SnapshotCommitRequest) (SnapshotApplyResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: SnapshotCommitRequest` — 清理未提交导入；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: SnapshotApplyResult` — 清理未提交导入成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`stageId` #/$defs/Identifier, `targetDataEpoch` #/$defs/Identifier, `barrierId` #/$defs/Identifier

成功结果字段：`stageId` #/$defs/Identifier, `dataEpoch` #/$defs/Identifier, `state` enum

步骤：

1. 查询 decision
2. 仅未 commit 可删除隔离目录
3. 已 commit 拒绝并要求继续激活

错误：

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：删除本域隔离临时文件。

授权：{"permission": "service", "rule": "data-protection.restore", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：restoreId 独占锁。

幂等：重复 abort 成功。

依赖：data-protection.getRestoreDecision。

## M06-F042 EnterBarrier

文件 `modules/public-site/internal/barrier.go`。公开渲染域进入安全屏障

```go
func EnterBarrier(ctx context.Context, op OperationContext, request BarrierRequest) (BarrierStatus, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `request: BarrierRequest` — 公开渲染域进入安全屏障；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BarrierStatus` — 公开渲染域进入安全屏障成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`operationId` #/$defs/Identifier, `barrierId` #/$defs/Identifier, `mode` enum, `deadline` #/$defs/Timestamp

成功结果字段：`barrierId` #/$defs/Identifier, `owner` , `state` enum, `dataEpoch` #/$defs/Identifier, `revisionWatermark` #/$defs/Revision

步骤：

1. 拒新预览token和派生索引写
2. 等待正在渲染响应完成/缓存rename日志安全点
3. 冻结水位，公开入口仍受全站maintenance

错误：

- `DEADLINE_EXCEEDED` / HTTP 504：BARRIER_TIMEOUT：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写barrier状态。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：单barrier独占，不能截断正在写文件宣称safe。

幂等：barrierId幂等。

依赖：site-config.readRuntimeState。

## M06-F043 ReleaseBarrier

文件 `modules/public-site/internal/barrier.go`。释放公开渲染维护

```go
func ReleaseBarrier(ctx context.Context, op OperationContext, in ReleaseBarrierInput) (ReleaseBarrierResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ReleaseBarrierInput` — 释放公开渲染维护；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ReleaseBarrierResult` — 释放公开渲染维护成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`barrierId` barrierId；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` 公共 BarrierStatus 合同，遵守其字段和空值规则

步骤：

1. 确认runtime新epoch和全域ready
2. 清旧epoch缓存/preview token
3. 恢复公开请求与投影写入

错误：

- `LOCKED` / HTTP 409：RESTORE_INCOMPLETE：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：EPOCH_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写barrier/清缓存。

授权：{"permission": "service", "rule": "service+superadmin委托", "recoveryAuthorization": "barrier/snapshot管理在维护下调用ValidateRecoveryAuthorization；只许当前已批准restore。普通业务不可使用该许可"}

并发：全站maintenance退出前不放行。

幂等：barrierId幂等。

依赖：site-config.readRuntimeState。

## M06-F044 ValidateRecoveryAuthorization

文件 `modules/public-site/internal/recovery_authorization.go`。本地验证本次恢复专用控制许可

```go
func ValidateRecoveryAuthorization(ctx context.Context, op OperationContext, in ValidateRecoveryAuthorizationInput) (ValidateRecoveryAuthorizationResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ValidateRecoveryAuthorizationInput` — 本地验证本次恢复专用控制许可；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ValidateRecoveryAuthorizationResult` — 本地验证本次恢复专用控制许可成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`authorization` 公共 RecoveryAuthorization 合同，遵守其字段和空值规则, `restoreId` restoreId；UTF-8；不得为 null, `barrierId` barrierId；UTF-8；不得为 null, `sourceEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧, `targetEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧, `action` action；UTF-8；不得为 null

成功结果字段：`valid` valid 的明确真假结果，不可 null

步骤：

1. 先校验不可被业务导入覆盖的信任锚签名/算法/keyId
2. 核audience本域/expiry/restoreId/barrierId/旧新epoch
3. 读取独立控制记录确认已批准同restore且action只在prepare/commit/rollback/barrier
4. commit核持久commit决定，rollback拒任何commit证据
5. 拒普通业务使用

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `STALE_RESULT` / HTTP 409：TOKEN_EXPIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `LOCKED` / HTTP 409：DECISION_REQUIRED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：RESTORE_COMMITTED：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "service+superadmin预先批准委托", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：校验期间不查询已换代identity业务库；控制记录与信任锚独立目录不被快照覆盖。

幂等：同许可同控制阶段结果；提交决策变化会使rollback失效。

依赖：不可变控制信任锚、data-protection.getRestoreDecision。

## M06-F045 LoadPreview

文件 `modules/public-site/web/src/PreviewPanel.vue`。Vue3模块交互：LoadPreview

```typescript
function LoadPreview(op: OperationContext, input: LoadPreviewInput): Promise<LoadPreviewResult>
```

参数：

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: LoadPreviewInput` — Vue3模块交互：LoadPreview；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: LoadPreviewResult` — Vue3模块交互：LoadPreview成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: Problem` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `revision` revision 非负 int64；不用浮点数

成功结果字段：`token` token；UTF-8；不得为 null, `html` html；UTF-8；不得为 null

步骤：

1. 领取明确修订预览token并同会话加载private预览
2. 消费失败显示需要重新领取，不缓存HTML
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新组件本地状态；仅明确提交动作调用已授权API。

授权：{"permission": "content.preview", "rule": "content.preview", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：组件AbortController取消旧请求；同组件同动作单飞。

幂等：mutation稳定operationId/query只读重复安全。

依赖：public-site OpenAPI HTTP/JSON/SSE。

## M06-F046 LoadPublicCapabilities

文件 `modules/public-site/web/src/PublicSettingsPanel.vue`。Vue3模块交互：LoadPublicCapabilities

```typescript
function LoadPublicCapabilities(op: OperationContext, input: LoadPublicCapabilitiesInput): Promise<LoadPublicCapabilitiesResult>
```

参数：

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: LoadPublicCapabilitiesInput` — Vue3模块交互：LoadPublicCapabilities；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: LoadPublicCapabilitiesResult` — Vue3模块交互：LoadPublicCapabilities成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: Problem` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null

成功结果字段：`capabilities` 有序列表；空列表表示没有可用项

步骤：

1. 加载公开索引和语言路由能力
2. 展示site-config配置revision，写配置仍由其权威处理
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新组件本地状态；仅明确提交动作调用已授权API。

授权：{"permission": "site.manage", "rule": "site.manage", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：组件AbortController取消旧请求；同组件同动作单飞。

幂等：mutation稳定operationId/query只读重复安全。

依赖：public-site OpenAPI HTTP/JSON/SSE。

## M06-F047 RequestPublicNavigation

文件 `modules/public-site/web/src/client.ts`。Vue3模块交互：RequestPublicNavigation

```typescript
function RequestPublicNavigation(op: OperationContext, input: RequestPublicNavigationInput): Promise<RequestPublicNavigationResult>
```

参数：

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RequestPublicNavigationInput` — Vue3模块交互：RequestPublicNavigation；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RequestPublicNavigationResult` — Vue3模块交互：RequestPublicNavigation成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: Problem` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null

成功结果字段：`tree` 有序列表；空列表表示没有可用项

步骤：

1. 请求公开导航JSON
2. 503显示依赖不可用，禁止用过期private投影
3. 处理401重新登录、403清空敏感界面
4. 按operationId重试不重复提交

错误：

- `FORBIDDEN` / HTTP 403：FORBIDDEN：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `CONFLICT` / HTTP 409：CONFLICT：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：更新组件本地状态；仅明确提交动作调用已授权API。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：组件AbortController取消旧请求；同组件同动作单飞。

幂等：mutation稳定operationId/query只读重复安全。

依赖：public-site OpenAPI HTTP/JSON/SSE。

## M06-F048 HandleRootRedirect

文件 `modules/public-site/internal/http_root.go`。根语言选择HTTP

```go
func HandleRootRedirect(ctx context.Context, op OperationContext, in HandleRootRedirectInput) (HandleRootRedirectResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleRootRedirectInput` — 根语言选择HTTP；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleRootRedirectResult` — 根语言选择HTTP成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`languageCookie` languageCookie；UTF-8；不得为 null, `acceptLanguage` acceptLanguage；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `location` location；UTF-8；不得为 null, `headers` 有序列表；空列表表示没有可用项

步骤：

1. 仅/按cookie→Accept-Language地区映射→基准语言
2. 返回302
3. Vary Accept-Language/Cookie，缓存no-store
4. 带前缀不调用

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：功能语言cookie读取，不写统计cookie。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：只读配置。

幂等：GET幂等。

依赖：SelectLocale、site-config.readConfigObject。

## M06-F049 ParseTiptapDocument

文件 `modules/public-site/internal/tiptap.go`。校验权威Tiptap渲染输入

```go
func ParseTiptapDocument(ctx context.Context, op OperationContext, in ParseTiptapDocumentInput) (ParseTiptapDocumentResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: ParseTiptapDocumentInput` — 校验权威Tiptap渲染输入；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: ParseTiptapDocumentResult` — 校验权威Tiptap渲染输入成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`document` TiptapDocument领域数据合同, `maxBytes` maxBytes 非负 int64；不用浮点数

成功结果字段：`document` TiptapDocument领域数据合同, `headings` 有序列表；空列表表示没有可用项, `mediaRefs` 有序列表；空列表表示没有可用项

步骤：

1. schemaVersion逐级已迁移
2. 类型枚举白名单/深度64/稳定blockId唯一
3. 拒未知节点而不默丢
4. 收集H2-H4与媒体refs

错误：

- `UNSUPPORTED_SCHEMA` / HTTP 422：UNSUPPORTED_SCHEMA：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "content.preview", "rule": "published或preview已授权", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：同JSON/hash同结果。

依赖：content.types.TiptapDocument。

## M06-F050 RenderTiptapHTML

文件 `modules/public-site/internal/tiptap_html.go`。按节点输出完整HTML

```go
func RenderTiptapHTML(ctx context.Context, op OperationContext, in RenderTiptapHTMLInput) (RenderTiptapHTMLResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RenderTiptapHTMLInput` — 按节点输出完整HTML；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RenderTiptapHTMLResult` — 按节点输出完整HTML成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`document` TiptapDocument领域数据合同, `locale` locale；UTF-8；不得为 null, `assets` 有序列表；空列表表示没有可用项

成功结果字段：`safeHTML` safeHTML；UTF-8；不得为 null, `containsMath` containsMath 的明确真假结果，不可 null

步骤：

1. 段落/引用/分割线/列表/任务/表格/四类callout转义渲染
2. chroma高亮代码
3. 公式仅源码占位/按需KaTeX
4. 图片srcset lazy与人工alt优先
5. 附件下载卡片/画板亮暗SVG/columns2..4窄屏竖排/上传video或平台封面点击
6. inline格式安全渲染

错误：

- `UNSUPPORTED_SCHEMA` / HTTP 422：UNSUPPORTED_SCHEMA：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "content.preview", "rule": "published或preview授权", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数，危险href拒绝，平台iframe仅白名单。

幂等：同document/assets/renderer版本同输出。

依赖：ParseTiptapDocument、SanitizeHTML、media公开元数据。

## M06-F051 RenderTiptapMarkdown

文件 `modules/public-site/internal/tiptap_markdown.go`。Tiptap输出AI可读Markdown

```go
func RenderTiptapMarkdown(ctx context.Context, op OperationContext, in RenderTiptapMarkdownInput) (RenderTiptapMarkdownResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RenderTiptapMarkdownInput` — Tiptap输出AI可读Markdown；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RenderTiptapMarkdownResult` — Tiptap输出AI可读Markdown成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`document` TiptapDocument领域数据合同, `locale` locale；UTF-8；不得为 null, `assets` 有序列表；空列表表示没有可用项

成功结果字段：`markdown` markdown；UTF-8；不得为 null

步骤：

1. 按块顺序输出段落/列表/表格/代码/LaTeX
2. callout保留类型
3. 多栏依序
4. 附件下载链接
5. 画板和视频输出图片链接+文本描述
6. 内部ID转换同语言URL+blockId
7. 不临时生成描述

错误：

- `UNSUPPORTED_SCHEMA` / HTTP 422：UNSUPPORTED_SCHEMA：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "公开authoritative检查完成", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：同revision同Markdown。

依赖：RewriteInternalLinks、media公开元数据。

## M06-F052 RenderHero

文件 `modules/public-site/internal/hero.go`。仅一屏默认主视觉

```go
func RenderHero(ctx context.Context, op OperationContext, in RenderHeroInput) (RenderHeroResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RenderHeroInput` — 仅一屏默认主视觉；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RenderHeroResult` — 仅一屏默认主视觉成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`title` title；UTF-8；不得为 null, `subtitle` subtitle；UTF-8；不得为 null, `backgroundKind` backgroundKind；UTF-8；不得为 null, `backgroundURL` backgroundURL；UTF-8；不得为 null, `posterURL` posterURL；UTF-8；不得为 null, `overlayPercent` overlayPercent 非负 int64；不用浮点数, `buttonLabels` 有序列表；空列表表示没有可用项, `buttonURLs` 有序列表；空列表表示没有可用项

成功结果字段：`html` html；UTF-8；不得为 null

步骤：

1. button0..3
2. 背景Welcome To默认不翻译但可启用
3. 遮罩0..100
4. 纯色/图/动图/video选择
5. video reduced-motion封面与图片preload
6. 不添加第二屏内容

错误：

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous config读取", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯渲染。

幂等：同public config revision同输出。

依赖：site-config公开首页配置。

## M06-F053 RenderAnnouncement

文件 `modules/public-site/internal/announcement.go`。公告翻译横幅

```go
func RenderAnnouncement(ctx context.Context, op OperationContext, in RenderAnnouncementInput) (RenderAnnouncementResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RenderAnnouncementInput` — 公告翻译横幅；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RenderAnnouncementResult` — 公告翻译横幅成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`announcementId` announcementId；UTF-8；不得为 null, `text` text；UTF-8；不得为 null, `link` link；UTF-8；不得为 null, `startAt` startAt；UTF-8；不得为 null, `endAt` endAt；UTF-8；不得为 null, `now` RFC3339 UTC 时间；不含 null

成功结果字段：`html` html；UTF-8；不得为 null, `visible` visible 的明确真假结果，不可 null

步骤：

1. 按[start,end)窗口
2. 输出当前语言text安全link
3. 关闭按钮localStorage存announcementId，同一条不再显示
4. 无统计cookie

错误：

- `VALIDATION` / HTTP 400：VALIDATION：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：客户端仅localStorage关闭标志。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：配置revision绑定rendercache。

幂等：同公告/时间窗口同结果。

依赖：site-config.readConfigObject。

## M06-F054 BuildPreviousNext

文件 `modules/public-site/internal/navigation_neighbors.go`。文末相邻文档

```go
func BuildPreviousNext(ctx context.Context, op OperationContext, in BuildPreviousNextInput) (BuildPreviousNextResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: BuildPreviousNextInput` — 文末相邻文档；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: BuildPreviousNextResult` — 文末相邻文档成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`documentId` documentId；UTF-8；不得为 null, `tree` 有序列表；空列表表示没有可用项

成功结果字段：`previous` 有序列表；空列表表示没有可用项, `next` 有序列表；空列表表示没有可用项

步骤：

1. 按权威树DFS稳定顺序
2. 排除非published节点
3. 输出前后最多一个
4. 缺相邻为空数组

错误：

- `INTEGRITY_ERROR` / HTTP 422：INTEGRITY_ERROR：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "service", "rule": "公开渲染内部", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：纯函数。

幂等：同tree修订同neighbors。

依赖：BuildNavigation。

## M06-F055 RenderNotFound

文件 `modules/public-site/internal/http_errors.go`。定制404页面

```go
func RenderNotFound(ctx context.Context, op OperationContext, in RenderNotFoundInput) (RenderNotFoundResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: RenderNotFoundInput` — 定制404页面；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: RenderNotFoundResult` — 定制404页面成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `html` html；UTF-8；不得为 null

步骤：

1. 读取可公开树
2. 输出文档树和没找到问问AI入口
3. AI未配置显示关闭原因
4. 不泄漏私有标题
5. content不可达503

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：重新authority导航。

幂等：GET幂等。

依赖：BuildNavigation、site-config公开AI能力。

## M06-F056 HandleAskShell

文件 `modules/public-site/internal/http_ask.go`。独立问答页面外壳

```go
func HandleAskShell(ctx context.Context, op OperationContext, in HandleAskShellInput) (HandleAskShellResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleAskShellInput` — 独立问答页面外壳；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleAskShellResult` — 独立问答页面外壳成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `html` html；UTF-8；不得为 null

步骤：

1. Go输出可访问语言外壳
2. 挂载qa同一会话Vue岛
3. 文档浮窗和独立页共享qa会话/当前块引用
4. 未配置AI显示缺项不调用模型

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无本域对话存储。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：qa拥有会话，不创建第二份会话。

幂等：GET幂等。

依赖：qa公开会话接口、site-config公开能力。

## M06-F057 PrewarmPublication

文件 `modules/public-site/internal/prewarm.go`。发布事件预渲染缓存

```go
func PrewarmPublication(ctx context.Context, op OperationContext, in PrewarmPublicationInput) (PrewarmPublicationResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: PrewarmPublicationInput` — 发布事件预渲染缓存；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: PrewarmPublicationResult` — 发布事件预渲染缓存成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`publication` 公共 PublishedRevisionRef 合同，遵守其字段和空值规则, `locales` 有序列表；空列表表示没有可用项

成功结果字段：`cachedKeys` 有序列表；空列表表示没有可用项

步骤：

1. 事件去重/epoch校验
2. 各启用语言RenderPublicPage并存修订缓存
3. 失败标派生任务错误不回滚正文
4. 撤回再验阻止旧缓存输出

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

- `VERSION_MISMATCH` / HTTP 409：VERSION_MISMATCH：在步骤中的对应校验或依赖失败；retryable=false；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：写派生cache/outbox。

授权：{"permission": "service", "rule": "service content事件", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：每文档2并发，不重复付费调用。

幂等：publicationHash/locale/rendererVersion幂等。

依赖：RenderPublicPage、jobs.createJob。

## M06-F058 HandleLLMsRoot

文件 `modules/public-site/internal/http_llms_root.go`。根llms语言索引

```go
func HandleLLMsRoot(ctx context.Context, op OperationContext, in HandleLLMsRootInput) (HandleLLMsRootResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleLLMsRootInput` — 根llms语言索引；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleLLMsRootResult` — 根llms语言索引成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `text` text；UTF-8；不得为 null

步骤：

1. 读取启用语言
2. 列/<locale>/llms.txt与llms-full链接
3. 不混合多语言正文
4. 停用语言不输出

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：公开配置revision一致。

幂等：GET幂等。

依赖：site-config.readConfigObject。

## M06-F059 HandleLanguageSitemap

文件 `modules/public-site/internal/http_language_sitemap.go`。每语言sitemap

```go
func HandleLanguageSitemap(ctx context.Context, op OperationContext, in HandleLanguageSitemapInput) (HandleLanguageSitemapResult, error)
```

参数：

- `ctx: context.Context` — 调用取消与deadline；不得携带正文到日志；约束：Go标准库context；用户取消/超时即停止新副作用

- `op: OperationContext` — 经可信鉴权中间件构建的操作上下文；含epoch与可选恢复授权；约束：公开入口actor=visitor；服务身份与delegation不接受客户端自声明；deadline不超过上游

- `in: HandleLanguageSitemapInput` — 每语言sitemap；字段见本域types.json对应$defs；约束：严格schema，拒未知字段；字段约束与默认参数见types/requirements

返回：

- `result: HandleLanguageSitemapResult` — 每语言sitemap成功结果；字段见types.json；约束：成功时满足全部required字段；失败返回零值且不泄漏半结果

- `error: error` — Go error转公共Problem；TypeScript拒绝Promise携带Problem；约束：成功nil；失败仅登记公共code，detail不含IP/提问/密钥

输入业务字段：`locale` locale；UTF-8；不得为 null, `dataEpoch` 不可复用的站点代次/密钥代次标识；不使用整数比较新旧

成功结果字段：`status` status 非负 int64；不用浮点数, `xml` xml；UTF-8；不得为 null

步骤：

1. 读取该语言公开规范URL集合
2. ≤10000输出urlset，更多输出该语言分片index
3. 最终权威校验，撤回不保留旧URL

错误：

- `DEPENDENCY_UNAVAILABLE` / HTTP 503：DEPENDENCY_UNAVAILABLE：在步骤中的对应校验或依赖失败；retryable=true；未完成写入回滚事务/删除临时文件；已有持久提交按本函数恢复日志继续，不倒退已提交恢复决策

副作用：无持久或外部写入。

授权：{"permission": "anonymous", "rule": "anonymous", "recoveryAuthorization": "不适用；本函数不得扩大已鉴权调用上下文"}

并发：同公开修订集合稳定分片。

幂等：GET幂等。

依赖：BuildSitemapEntries。

<!-- service-bootstrap:start -->
## 独立服务启动、运行与健康函数

### M06-F901 main

文件 `modules/public-site/cmd/public-site/main.go`。独立进程可执行入口

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

### M06-F902 BuildApplication

本域还须装配 M06-F060 有界事件worker，受应用取消和屏障控制，禁止启动孤立后台进程。

文件 `modules/public-site/internal/app/wire.go`。装配本域存储、客户端、函数路由与worker

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

### M06-F903 Serve

文件 `modules/public-site/internal/transport/server.go`。运行独立HTTP/JSON/SSE服务

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

### M06-F904 Shutdown

文件 `modules/public-site/internal/transport/server.go`。停止入口并使在途操作到可恢复安全点

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

### M06-F905 Health

文件 `modules/public-site/internal/transport/server.go`。GET /healthz普通HTTP存活检查

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

### M06-F906 Ready

文件 `modules/public-site/internal/transport/server.go`。GET /readyz普通HTTP就绪检查

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

### M06-F907 mountModuleUI

文件 `modules/public-site/web/src/main.ts`。本域Vue3应用/岛的唯一UI构建入口

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

## M06-F060 ConsumePublicEvents

`func ConsumePublicEvents(ctx context.Context, op OperationContext, input PublicEventSubscriptionInput) (PublicEventSubscriptionResult, error)`

文件：`modules/public-site/internal/events/consume.go`。消费发布/撤回/译文完成事件，恢复性失效与预热公开投影。

输入：

- `ctx: context.Context`：取消与截止；非nil，取op.deadline较早值
- `op: OperationContext`：服务可信操作上下文；service身份/当前epoch/本域维护屏障有效
- `input: PublicEventSubscriptionInput`：有界订阅批次；schema封闭，cursor仅来自已提交本域记录

输出：

- `result: PublicEventSubscriptionResult`：已经提交的计数和继续游标；错误时零值不可用
- `err: error`：登记公共错误；不含正文、IP、秘密

算法：

1. 用jobs:subscribeEvents拉取本域subscription、当前epoch和三类已登记事件；每批最多limit，30秒总deadline，取消后不推进游标
2. 按生产方载荷schema验证，eventId+payloadHash查本域inbox；同ID异hash隔离并INTEGRITY_ERROR，重复/低revision只安全推进已提交游标，旧epoch拒绝
3. content changed/withdrawn先调用M06-F019失效对应文档所有语言投影，在本域事务持久inbox与预热待处理标识；withdrawn禁止预热
4. content changed及translation.ready由M06-F057按该不可变ref预热；引用/current语言状态重新经content与localization权威读取，stale不覆盖更高版本；无模型调用
5. 预热故障保留本域pending标识，下一批先按有界重试处理，公开请求可按同ref实时渲染且最终门禁仍生效；不能把预热失败变成正文发布失败
6. 成功提交inbox/失效或pending后持久游标；配置与启用语言每请求读取当前site-config/localization状态，不假设有未登记的配置事件消费

错误：VALIDATION（载荷或批次不合法）、INTEGRITY_ERROR（事件ID与hash冲突）、LOCKED（维护禁止业务写）、STALE_RESULT（epoch或source修订过期）、DEPENDENCY_UNAVAILABLE（持久事件源或权威门禁不可用）、CANCELLED（贯穿取消）、DEADLINE_EXCEEDED（批次截止）、INTERNAL（本域提交失败）。

副作用：本域inbox/失效标识/预热pending与游标；无模型费用。

授权：仅已验证public-site服务身份；不提供浏览器管理入口。

并发：subscription单worker租约30秒/10秒续约，fencing token及inbox唯一键；业务写检查当前epoch和屏障，逐source CAS防复活。

幂等：inbox(eventId,payloadHash)唯一，与失效/pending同事务；重放不重复副作用，游标只在提交后推进。

依赖：jobs:subscribeEvents；content:validatePublications；M06-F019；M06-F057。
