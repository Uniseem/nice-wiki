# public-site HTTP/JSON/SSE

完整schema、参数、错误和security见 [OpenAPI](../contracts/openapi.json)。可信op由入口/服务鉴权构建，requestId由入口生成；Idempotency-Key绑定规范body hash，不同body返回CONFLICT。dataEpoch是runtime权威opaque ID，任何写入必须比较相等；operationContext和恢复许可由可信控制通道转发，不能由匿名请求自声明。

| 方法与路径 | operationId | handler ID | 权限 | 响应/行为 |
|---|---|---|---|---|
| GET `/robots.txt` | `handleRobots` | `M06-F025` | `anonymous` | 站点级索引策略，不输出私有地址 |
| GET `/llms.txt` | `handleLLMsRoot` | `M06-F058` | `anonymous` | 仅可公开引用的已发布文档摘要索引 |
| GET `/sitemap.xml` | `handleSitemapIndex` | `M06-F030` | `anonymous` | 站点 sitemap 分片索引 |
| GET `/sitemap/{locale}/{shard}.xml` | `handleSitemapShard` | `M06-F031` | `anonymous` | 公开、规范、已发布地址和更新时间 |
| POST `/v1/preview/tokens` | `issuePreviewToken` | `M06-F032` | `content.preview` | 为明确 documentId/revision 签发 120 秒单用途预览 token |
| GET `/v1/preview/{token}` | `handlePreview` | `M06-F035` | `service` | 不缓存 HTML；禁用 robots 索引；引用资源用 media 短期预览授权 |
| GET `/` | `handleRootRedirect` | `M06-F048` | `anonymous` | 仅根路径302，cookie/Accept-Language/基准语言优先，Vary正确 |
| GET `/{locale}/` | `handleHome` | `M06-F022` | `anonymous` | 仅全屏首页hero |
| GET `/{locale}/{slug}` | `handlePage` | `M06-F021` | `anonymous` | 完整Go三栏HTML、no-cache+etag、旧slug301、删除410 |
| GET `/{locale}/{slug}.md` | `handleMarkdown` | `M06-F024` | `anonymous` | Tiptap→Markdown，alternate链接，权威撤回检查 |
| GET `/{locale}/navigation` | `handleNavigation` | `M06-F023` | `anonymous` | 公开语言文档树 |
| GET `/{locale}/llms.txt` | `handleLLMsIndex` | `M06-F027` | `anonymous` | 该语言文档树索引 |
| GET `/{locale}/llms-full.txt` | `handleLLMsFull` | `M06-F028` | `anonymous` | 该语言全正文或明确分片链接 |
| GET `/{locale}/ask` | `handleAskShell` | `M06-F056` | `anonymous` | 独立问答页，与文档右下浮窗共用qa会话 |
| GET `/{locale}/sitemap.xml` | `handleLanguageSitemap` | `M06-F059` | `anonymous` | 每语言sitemap，root sitemap.xml总索引 |
| POST `/v1/barriers` | `acquireBarrier` | `M06-F042` | `service` | 公共BarrierRequest→BarrierStatus；持久draining/safe，等待本域实际安全点 |
| POST `/v1/barriers/{barrierId}:release` | `releaseBarrier` | `M06-F043` | `service` | 仅全域恢复完成或commit前全部abort确认后释放 |
| POST `/v1/snapshots:export` | `exportSnapshot` | `M06-F037` | `service` | 公共SnapshotExportRequest→SnapshotExportResult；必须safe |
| POST `/v1/snapshots:validate` | `validateSnapshot` | `M06-F038` | `service` | 公共SnapshotValidateRequest→SnapshotValidation；只写隔离报告 |
| POST `/v1/snapshots:prepare` | `prepareSnapshot` | `M06-F039` | `service` | 公共SnapshotPrepareRequest→SnapshotStageResult；不切指针 |
| POST `/v1/snapshots:commit` | `commitSnapshot` | `M06-F040` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；需持久commit决定 |
| POST `/v1/snapshots:rollback` | `rollbackSnapshot` | `M06-F041` | `service` | 公共SnapshotCommitRequest→SnapshotApplyResult；commit后禁止 |

普通错误使用公共Problem，含requestId/code/retryable/安全detail。事件载荷遵守events.json；public字段不包括IP、提问或secret。服务deadline默认2500ms，长流程只返回accepted引用，实际进度经jobs SSE；取消停止新副作用，已提交结果由恢复日志收敛。

屏障请求持久draining并等待真实在途安全点；export要求safe；validate只读隔离；prepare不切active；commit校验签名决策；rollback只可commit前。已commit失败必须继续同一targetEpoch并保持维护。

<!-- service-bootstrap:start -->
## 独立进程健康与就绪

- `GET /healthz` → `health`，匿名HealthStatus，只本进程存活；200/503，不查远程或模型。
- `GET /readyz` → `ready`，匿名ServiceReadiness，检查schema/journal/control/epoch/关键依赖；维护control-ready不等于公开门禁解除，publicRequestsAllowed=false。

函数及配置完整合同见本域启动函数和根[启动蓝图](../../../docs/service-bootstrap.md)。
<!-- service-bootstrap:end -->
