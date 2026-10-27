# public-site 数据与持久化

未来私有目录 `data/public-site/active/` 保存SQLite和领域对象；`data/public-site/control/` 保存不随业务恢复替换的barrier、恢复回执和操作journal。外域不得直接读这些路径。

- `render_cache(cache_key PK, data_epoch, document_id, publication_revision, metadata_revision, locale, theme_revision, html_path, sha256, expires_at, bytes, last_access_at)`

- `navigation_projection(site_id, locale, projection_revision, nodes_json, data_epoch; PK site_id/locale)`

- `event_inbox(event_id PK, source_revision, data_epoch, accepted_at)`

- `snapshot_catalog(snapshot_id PK, barrier_id, data_epoch, manifest_sha256, status)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

control区不随恢复替换。

首页公开链接解析使用ResolvePublishedLinkInput/Result：输入documentId、language、blockId全部必填（blockId可null）；输出visible、href、publication、resolvedLanguage、reason全部必填。oneOf强制true时三项非null且reason=available，false时三项全null且reason只能unavailable/anchor_missing/language_unavailable。false不返回标题、metadata、草稿、旧路径或具体未公开原因；未知依赖返回503而非负结果。

content公开树的required canonicalPath由content同一快照给出完整规范层级，已分段URL编码且不含语言/basePath/query/fragment；公开导航跳过未公开祖先后的parentId不可用于消费方拼路径。href由本域既有canonical机制组合当前可信basePath、显式启用语言和该路径，附已存在块锚点，不含query/外部origin。ResolvedLanguage表示URL语言，已启用语言缺译时仍等于请求语言，正文沿既定规则旧译/原文回退。
