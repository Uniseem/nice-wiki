# 数据、权威与持久化

正文私有文件 siteId/docId/revisionId.json 为唯一内容真相；Tiptap DocumentBody带schemaVersion。私有DB只树/meta/head/journal/稳定IDledger/引用索引/receipt/outbox/history/快照staging；文件fsync完成+COMMITTED head才可见，已发布ref和共享draft head独立。所有draft/history正文版本不可覆盖。redirect manifest版本化文件经对象API由data-protection纳入git镜像。

所有块type/attrs/content、行内text/marks/math联合体在types.json完整required和additionalProperties=false。image引用原图media variants，drawing只scene/亮SVG/暗SVG hash；platform视频snapshot公开信息只插入一次。节点IDs包含block/listitem/task/cell/row/column，在文档内不复用；译文沿用原块ID，text extraction按列顺序表格按行。正文代码/latex不翻译，链接地址和inlinecode由翻译占位保护。

类型、required/额外字段、枚举、最大长度、空值与格式均由 [types.json](../contracts/types.json) 定义。函数输入和结果各有独立 schema，内部句柄类型标 x-internal。

| 类型 | 精确含义 |
|---|---|
| Identifier | Identifier |
| Revision | Revision |
| Hash | Hash |
| Timestamp | Timestamp |
| AuthContext | AuthContext |
| Error | Error |
| Problem | Problem |
| PageRequest | PageRequest |
| PageInfo | PageInfo |
| PublishedRevisionRef | PublishedRevisionRef |
| PublicationValidation | PublicationValidation |
| AssetRef | AssetRef |
| ObjectRef | ObjectRef |
| BarrierRequest | BarrierRequest |
| BarrierStatus | BarrierStatus |
| SnapshotRef | SnapshotRef |
| SnapshotExportRequest | SnapshotExportRequest |
| SnapshotExportResult | SnapshotExportResult |
| SnapshotStageResult | SnapshotStageResult |
| EventEnvelope | EventEnvelope |
| HealthStatus | HealthStatus |
| LinkTarget | 站内链接按当前语言和树路径渲染，移动/改名不失效 |
| InlineMark | old8项行内能力：5文本marks+highlight+link+inlineMath；同种mark至多一次 |
| InlineNode | Tiptap受控行内联合体，未知节点不静默丢弃 |
| ParagraphBlock | Tiptap paragraph节点 |
| HeadingBlock | Tiptap heading节点 |
| QuoteBlock | Tiptap blockquote节点 |
| DividerBlock | Tiptap horizontalRule节点 |
| ListItem | 列表项；可嵌套列表但受深度限制 |
| BulletListBlock | Tiptap bulletList节点 |
| OrderedListBlock | Tiptap orderedList节点 |
| TaskItem | 任务项 |
| TaskListBlock | Tiptap taskList节点 |
| TableCell | 表格单元格，grid覆盖必须无洞无重叠 |
| TableRow | 表格行 |
| TableBlock | Tiptap table节点 |
| CodeBlock | Tiptap codeBlock节点 |
| MathBlock | Tiptap mathBlock节点 |
| CalloutBlock | Tiptap callout节点 |
| ImageBlock | Tiptap image节点 |
| AttachmentBlock | Tiptap attachment节点 |
| DrawingBlock | Tiptap drawing节点 |
| UploadedVideoSource | 上传视频，服务端不转码 |
| PlatformVideoSource | 插入时一次公开信息快照，失败可手填 |
| VideoBlock | Tiptap video节点 |
| Column | 单栏 |
| ColumnsBlock | Tiptap columns节点 |
| Block | 16类Tiptap块完整联合体；schema版本在DocumentBody，不含未知attrs/原始HTML |
| DocumentBody | 正文文件canonical Tiptap JSON |
| DocumentMetadata | 树/元数据权威，与draft和published独立 |
| DraftRevision | 全篇共享草稿提交，不按人建份 |
| EditLease | 内存锁，不进快照；同人多tab共享账户锁，draftVersion冲突防覆盖 |
| CommitJournal | 正文持久提交意图；只有committed可见 |
| PublicationChangedPayload | 不可变发布快照引用，供public/index/translation/git镜像 |
| PublicationWithdrawnPayload | 只传失效引用，无正文 |
| ResourceReference | 当前/历史引用分开，历史引用不自动清理阻塞提示 |
| ResourceReferencesChangedPayload | 事件仅ref，无正文 |
| PreviewGrant | 公开样式渲染草稿，仅管理员可见 |
| HistoryEntry | 本域版本与data git镜像署名 |
| RouteTombstone | 旧路径→docId，移动父节点须记录所有后代301；删除410/可选redirect |
| CreateDocumentInput | 新建共享草稿文档的输入 |
| CreateDocumentResult | 新建共享草稿文档的结果 |
| ReadDocumentTreeInput | 后台读取整站文档树的输入 |
| ReadDocumentTreeResult | 后台读取整站文档树的结果 |
| ListDocumentsInput | 文档及回收站分页列表的输入 |
| ListDocumentsResult | 文档及回收站分页列表的结果 |
| MoveDocumentInput | 拖拽移动层级和顺序的输入 |
| MoveDocumentResult | 拖拽移动层级和顺序的结果 |
| ReorderChildrenInput | 拖拽调整同级顺序的输入 |
| ReorderChildrenResult | 拖拽调整同级顺序的结果 |
| EnsureTreeAcyclicInput | 检查树无环和层级约束的输入 |
| EnsureTreeAcyclicResult | 检查树无环和层级约束的结果 |
| UpdateDocumentMetadataInput | 修改标题/路径/SEO描述/分享图的输入 |
| UpdateDocumentMetadataResult | 修改标题/路径/SEO描述/分享图的结果 |
| ValidatePathSegmentInput | 校验英文可读路径与同父唯一的输入 |
| ValidatePathSegmentResult | 校验英文可读路径与同父唯一的结果 |
| PlanOldPathRedirectsInput | 规划301旧路径→文档ID的输入 |
| PlanOldPathRedirectsResult | 规划301旧路径→文档ID的结果 |
| AcquireEditLockInput | 打开文档自动获得内存编辑锁的输入 |
| AcquireEditLockResult | 打开文档自动获得内存编辑锁的结果 |
| RenewEditLockInput | 心跳维持锁并处理长时间无输入的输入 |
| RenewEditLockResult | 心跳维持锁并处理长时间无输入的结果 |
| ReleaseEditLockInput | 关闭页面释放自身锁的输入 |
| ReleaseEditLockResult | 关闭页面释放自身锁的结果 |
| ForceReleaseEditLockInput | 只有超管强制释放别人锁的输入 |
| ForceReleaseEditLockResult | 只有超管强制释放别人锁的结果 |
| ExpireIdleLocksInput | 释放失联或提示后仍空闲锁的输入 |
| ExpireIdleLocksResult | 释放失联或提示后仍空闲锁的结果 |
| ReadDraftInput | 读取全篇共享草稿及编辑状态的输入 |
| ReadDraftResult | 读取全篇共享草稿及编辑状态的结果 |
| SaveDraftInput | 自动保存完整Tiptap正文的输入 |
| SaveDraftResult | 自动保存完整Tiptap正文的结果 |
| ValidateBlockTreeInput | 验证全部Tiptap块及稳定ID的输入 |
| ValidateBlockTreeResult | 验证全部Tiptap块及稳定ID的结果 |
| ValidateInlineInput | 验证全部行内格式与链接的输入 |
| ValidateInlineResult | 验证全部行内格式与链接的结果 |
| ValidateTableGridInput | 验证表格合并单元格覆盖的输入 |
| ValidateTableGridResult | 验证表格合并单元格覆盖的结果 |
| ValidateColumnsInput | 校验2–4栏比例和禁止嵌套的输入 |
| ValidateColumnsResult | 校验2–4栏比例和禁止嵌套的结果 |
| CollectResourceReferencesInput | 扫描所有嵌套块资源引用的输入 |
| CollectResourceReferencesResult | 扫描所有嵌套块资源引用的结果 |
| CanonicalizeBodyInput | 规范序列化正文并计算hash的输入 |
| CanonicalizeBodyResult | 规范序列化正文并计算hash的结果 |
| HashBlockInput | 计算翻译/AI引用稳定块hash的输入 |
| HashBlockResult | 计算翻译/AI引用稳定块hash的结果 |
| WriteRevisionFileInput | 持久写权威正文文件的输入 |
| WriteRevisionFileResult | 持久写权威正文文件的结果 |
| CommitDraftJournalInput | 原子提交共享草稿head的输入 |
| CommitDraftJournalResult | 原子提交共享草稿head的结果 |
| RecoverPreparedCommitsInput | 恢复崩溃前正文提交日志的输入 |
| RecoverPreparedCommitsResult | 恢复崩溃前正文提交日志的结果 |
| ReadRevisionBodyInput | 读取COMMITTED正文并核真相的输入 |
| ReadRevisionBodyResult | 读取COMMITTED正文并核真相的结果 |
| PreparePublicationMetadataInput | 人工优先准备首次slug和SEO的输入 |
| PreparePublicationMetadataResult | 人工优先准备首次slug和SEO的结果 |
| PublishDocumentInput | 点击发布才公开精确共享草稿版本的输入 |
| PublishDocumentResult | 点击发布才公开精确共享草稿版本的结果 |
| CommitPublicationInput | 原子提交当前发布快照与署名历史的输入 |
| CommitPublicationResult | 原子提交当前发布快照与署名历史的结果 |
| WithdrawDocumentInput | 撤回后回草稿并让公开页面下线的输入 |
| WithdrawDocumentResult | 撤回后回草稿并让公开页面下线的结果 |
| ValidatePublicationsInput | 同步核公开引用是否仍可见且当前的输入 |
| ValidatePublicationsResult | 同步核公开引用是否仍可见且当前的结果 |
| ReadPublishedRevisionInput | 获取仍公开的精确正文快照的输入 |
| ReadPublishedRevisionResult | 获取仍公开的精确正文快照的结果 |
| TrashDocumentInput | 删除进回收站，可选旧URL替代跳转的输入 |
| TrashDocumentResult | 删除进回收站，可选旧URL替代跳转的结果 |
| RestoreDocumentInput | 从回收站恢复为草稿的输入 |
| RestoreDocumentResult | 从回收站恢复为草稿的结果 |
| PurgeDocumentsInput | 只有超管彻底删除/清空回收站的输入 |
| PurgeDocumentsResult | 只有超管彻底删除/清空回收站的结果 |
| ReadRevisionHistoryInput | 查看署名发布/git版本历史的输入 |
| ReadRevisionHistoryResult | 查看署名发布/git版本历史的结果 |
| DiffRevisionsInput | 按稳定块ID对比历史版本的输入 |
| DiffRevisionsResult | 按稳定块ID对比历史版本的结果 |
| RollbackPublishedRevisionInput | 回滚历史形成新发布/git提交的输入 |
| RollbackPublishedRevisionResult | 回滚历史形成新发布/git提交的结果 |
| CreatePreviewGrantInput | 管理员预览草稿公开样式的输入 |
| CreatePreviewGrantResult | 管理员预览草稿公开样式的结果 |
| ValidatePreviewGrantInput | 只向对应管理员验证草稿预览的输入 |
| ValidatePreviewGrantResult | 只向对应管理员验证草稿预览的结果 |
| ListResourceReferencesInput | 向资源库列出哪些文档引用文件的输入 |
| ListResourceReferencesResult | 向资源库列出哪些文档引用文件的结果 |
| ReadContentObjectInput | 经对象API读取私有文件而不共享目录的输入 |
| ReadContentObjectResult | 经对象API读取私有文件而不共享目录的结果 |
| PublishContentEventInput | 与发布/撤回提交原子登记最小事件的输入 |
| PublishContentEventResult | 与发布/撤回提交原子登记最小事件的结果 |
| ReleaseBarrierInput | 释放已协调完成的屏障的输入 |
| ReleaseBarrierResult | 释放已协调完成的屏障的结果 |
| PublicTreeNode | 仅已发布公共导航节点，无drafthead |
| ReadPublicTreeInput | 公开导航读取已发布文档树的输入 |
| ReadPublicTreeResult | 公开导航读取已发布文档树的结果 |
| ResolvePublicPathInput | 解析当前路径、旧路径301与删除410的输入 |
| ResolvePublicPathResult | 解析当前路径、旧路径301与删除410的结果 |
| EditorSession | Vue编辑会话 |
| EditorAction | 受限编辑动作 |
| createEditorSessionInput | 创建Tiptap3/Vue编辑器会话的输入 |
| createEditorSessionResult | 创建Tiptap3/Vue编辑器会话的结果 |
| applyBlockActionInput | 拖拽手柄菜单排序/转换/复制/删除的输入 |
| applyBlockActionResult | 拖拽手柄菜单排序/转换/复制/删除的结果 |
| resolveSlashMenuInput | slash插入菜单与Markdown输入快捷键的输入 |
| resolveSlashMenuResult | slash插入菜单与Markdown输入快捷键的结果 |
| normalizePastedContentInput | Word/网页/Markdown粘贴清理并转换块的输入 |
| normalizePastedContentResult | Word/网页/Markdown粘贴清理并转换块的结果 |
| scheduleAutosaveInput | 去抖自动保存共享草稿的输入 |
| scheduleAutosaveResult | 去抖自动保存共享草稿的结果 |
| maintainEditHeartbeatInput | 每30秒锁心跳与15分钟空闲提醒的输入 |
| maintainEditHeartbeatResult | 每30秒锁心跳与15分钟空闲提醒的结果 |
| applyForcedReadOnlyInput | 强释/失锁后立刻只读的输入 |
| applyForcedReadOnlyResult | 强释/失锁后立刻只读的结果 |
| selectInternalLinkInput | 插入文档选择器生成ID站内链接的输入 |
| selectInternalLinkResult | 插入文档选择器生成ID站内链接的结果 |
| prepareImageUploadsInput | 浏览器生成多宽WebP连原图上传的输入 |
| prepareImageUploadsResult | 浏览器生成多宽WebP连原图上传的结果 |
| openDrawingEditorInput | Vue包装Excalidraw并按需加载的输入 |
| openDrawingEditorResult | Vue包装Excalidraw并按需加载的结果 |
| saveDrawingFilesInput | 保存场景和亮暗SVG三个内容寻址文件的输入 |
| saveDrawingFilesResult | 保存场景和亮暗SVG三个内容寻址文件的结果 |
| prepareVideoBlockInput | 统一上传/平台视频插入的输入 |
| prepareVideoBlockResult | 统一上传/平台视频插入的结果 |
| resizeColumnsInput | 调整2–4栏宽比例的输入 |
| resizeColumnsResult | 调整2–4栏宽比例的结果 |
| releaseBarrierRequest | 释放屏障请求体 |
| updateDocumentMetadataRequest | 修改标题/路径/SEO描述/分享图请求体 |
| acquireEditLockRequest | 打开文档自动获得内存编辑锁请求体 |
| renewEditLockRequest | 心跳维持锁并处理长时间无输入请求体 |
| releaseEditLockRequest | 关闭页面释放自身锁请求体 |
| forceReleaseEditLockRequest | 只有超管强制释放别人锁请求体 |
| saveDraftRequest | 自动保存完整Tiptap正文请求体 |
| publishDocumentRequest | 点击发布才公开精确共享草稿版本请求体 |
| withdrawDocumentRequest | 撤回后回草稿并让公开页面下线请求体 |
| trashDocumentRequest | 删除进回收站，可选旧URL替代跳转请求体 |
| restoreDocumentRequest | 从回收站恢复为草稿请求体 |
| diffRevisionsRequest | 按稳定块ID对比历史版本请求体 |
| rollbackPublishedRevisionRequest | 回滚历史形成新发布/git提交请求体 |

恢复重建必须验证 manifest 哈希和私有事务水位。derived cache 可重建；权威记录缺失或 schema 未知应隔离并停止对外发布。


## 第四轮首页跨域只读协议

PublicTreeNode.canonicalPath为新增required完整路径，只是公开树响应契约，ReadPublicTree尚未实现，不宣称路由可用。ValidatePublications登记site-config只读链接调用方；不能由消费者读取私有树或猜路径。


## 第四轮首页跨域只读协议

readReferenceDecision仅media纯服务读取引用源最小决定，COMMITTED优先，缺失/未知不得释放pin。该约束函数已在持续目标第二轮编写并通过根VPS编译/结构检查；HTTP与完整服务仍未装配。

## 第五轮正文文件核心

硬上限统一16MiB，替换未有writer历史数据的旧64MiB常量；新写另服受信当前站点maxDocumentBytes，历史读只服硬上限，不等价资源上传配额。规范JSON复用platform.CanonicalJSON，marks按类型英文名升序，Unicode/代码/latex及正文数组顺序不改。所有block/item/row/cell/column ID全树唯一且合计≤10000，深度≤32；历史retired ID由提交事务ledger+head CAS拒复用，不由previousBody猜历史，合法历史读取不会拒绝已退役ID。

WriteRevisionFile参考输入新增expectedHead(nullable)/expectedDraftVersion，来自SaveDraft/Create已核CAS快照；author仅可信op，ObjectRef固定content/revisionId/version1、application/json与完整hash/长度。既有WithDraftCommitStore注入扩展RevisionFileStore，准备持久保存caller/actor/site/epoch/op、完整规范摘要及首次Draft/Journal结果；私有preparing→prepared→committed/aborted/quarantined与原8字段JSONL格式分开，不声明跨FS/DB原子，也不推进head/ledger。

新准备逐层校验私有路径，拒链接/特殊/多硬链接/点目录跳转，0700目录与0600临时文件；公共安全窗口覆盖当前epoch与控制截止，PREPARED日志fsync、tmp/fsync、同目录不覆盖rename及父目录同步后真实MarkPrepared才成功。崩溃preparing复用原材料续文件，同op不重造时间/commitId；prepared/committed复核原文件后回原结果，不因后来配额降低重做业务。

ReadRevisionBody只从受信COMMITTED索引拿完整ObjectRef/归属和资格，nullable expectedHash从该精确修订索引取，不从文件系统推当前head。持久文件边界一次使用生成DocumentBody schema验证，再核块语义、规范编码与hash；读前后资格一致才返回。真实材料/文件损坏持久隔离并关闭公开，无fallback；调用方错expectedHash只拒绝，不隔离与权威一致的文件。没有实际窄存储adapter则失败关闭。

窄存储的FindPreparation必须合并原JSONL最新终止/隔离状态与COMMITTED receipt；不能用旧索引复活启动已隔离孤儿。隔离按COMMITTED优先：已提交损坏关闭文档，未提交坏准备只隔离该准备，不关闭另一个合法head；本轮没有假adapter实现。

本轮纯规则明确资源sourceKey为content+documentId+sourceRevisionId+sourceVersion+kind+blockId+assetId+assetVersion。同块相同资源版本去重，不合并不同块；同assetId/version的哈希、长度或MIME冲突拒绝。画板三文件、平台snapshot.cover与独立poster均保留为精确本地引用，nullable跳过；sourceVersion与当前/发布/历史分类来自已授权上游，不从正文推测提交。retained=true只表示本次被扫描正文仍保留，函数不读media或判断远端文件存亡。

真实私有DB准备/COMMITTED资格/隔离adapter尚缺首次SQLite schema、只追加迁移路径与启动打开协议的结构确认；本轮未实现或猜测这些存储行为。head/ledger/refs/receipt/outbox仍要求在原草稿提交事务内一致。


## 第一轮真实私有正文库

2026-10-16用户已授权本轮所需结构与职责取舍；新增本域internal/storage.go和migrations下的内嵌包、0001_domain.sql。active/content.db使用既有批准modernc.org/sqlite v1.59.0，WAL、foreign_keys、busy_timeout=3000、synchronous=FULL，单连接。路径逐层核私有正规目录/文件及sidecar，不接收客户端文件路径。每份迁移记录编号/文件名/SHA-256；已应用SQL变更、缺号、集外版本与未知既有库拒绝启动，不隐式修复。

store_identity只在首次创建的空库记录可信openedEpoch，既有epoch缺失/变化不按请求覆写；本轮恢复换目录时仍blocked，后续必须从真实换目录决定取得重绑定协议。单站site_id最初为空；Find只返回未命中、Read返回NOT_FOUND，首次真实Prepare在同一SQL事务按可信op绑定站点并持久preparing。同进程其他站点不能在绑定前创建正文目录，也不能覆盖绑定。

revision_preparations持久caller/actor/site/dataEpoch/operationId、完整摘要、不可变修订归属及首次Draft/Journal。准备状态与原8字段JSONL分开；Find合并最新JSONL隔离/终止和真实业务回执，COMMITTED决定不因终止记录被当成未提交，quarantined不得复活。Read只允许revision_preparations、committed_revisions、draft_commit_receipts的精确联查，核原回执Draft一致及完整ObjectRef，不以JSONL、文件存在或head推测提交。

0001交付时只登记提交资格表、draft_heads与document_quarantines，当时没有业务写入提交表/head的能力，不创建文档、编辑锁或假COMMITTED。未来完整草稿事务须同事务提交head、ledger、COMMITTED、refs、原回执和平台outbox；本轮隔离已提交修订时持久关闭整个文档的读取/公开资格，未提交隔离不关闭另一合法head。内存锁不进入SQLite或快照。nw_outbox只通过libs/platform建表，不复制公共schema。

RevisionFileStore独立于DraftCommitStore，新增WithRevisionFileStore且保留旧完整存储注入兼容查找。RevisionStorageConfig由已授权上游提供当前真实正文限额及公共WriteWindow；空配置只可读，不开放写入。配置view不复制数据库，不构造完整草稿提交占位。启动实际open/migrate/recover；未绑定站点但存在非空journal或权威记录失败关闭，空库空journal可恢复为空，不造站点或head。

首个可信新库的epoch与第1迁移及记录同SQL事务提交；JSONL追加/fsync与所有journal扫描共享RWMutex，读快照与追加互斥，锁内不取SQL/Store互斥。journal使用标准NOFOLLOW并在打开前后/读写后验证SameFile及公共私有路径，不持DB事务跨文件IO。

持久prepared/committed决定必须有完整同材料JSONL；Find缺日志即完整性失败。启动反向核对全部已准备/提交索引，不能只扫描现存日志而漏丢失/截空记录；唯一允许无日志的是尚未append的preparing且没有提交资格，缺日志不猜补。


## 第二轮编辑锁与持久文档fence

0002只追加documents的site/doc、生命周期state、元数据revision和delete_fence权威；不插入任何文档，不从待保存请求/文件猜存在或删除状态。隔离继续读document_quarantines；未绑定站点的库有documents记录同样拒绝启动。完整Create/Trash/Restore/Purge必须在同doc mutex与本域业务事务内维护真实行及删除fence，当前仍未实施，不以影子状态代替。

EditLockManager持control/content-leader.lock的flock至Close，私有路径/NOFOLLOW/打开前后SameFile阻止链接和路径替换，从不unlink控制锁文件。所有操作先持manager生命读锁，再取site/doc二元键mutex，后读SQL；SaveFenceGuard持两把锁至Release。Close先取生命写锁排空所有已取guard再释放flock，新旧leader不能在旧guard尚可提交时重叠。内存lease/boot/fence不进快照；数据库仍是文档/delete/隔离权威。

RenewEditLockInput及请求新增required heartbeatSequence/inputSequence（0..9007199254740991），EditLease返回两者当前值，类型/OpenAPI为唯一来源并由根VPS重生成。每个共享账户lease有一对序号，同账户多tab先衔接服务返回值再递增，不生成每tab锁。心跳严格前进才续2分钟TTL，输入严格前进才以服务接收时钟更新LastInputAt并清提醒；等/旧心跳仅返回当前快照，不续期或更新活动。客户端lastInputAt只兼容显示，不信作活跃时间；过期/旧boot/fence不能续回。

15分钟提醒界点固定为LastInputAt+15分钟，LastInputAt+17分钟绝对失效；检测延迟不延后界点，逾期新输入不能续活。提醒/释放快照只是内存事实，尚未通过jobs通知或审计，完整约束用例必须可靠编排后才能对HTTP声明成功。Core显示名参数必须来自identity权威，不能浏览器自报；本轮未伪造姓名生产方或超管授权，也未新增未获消费者确认的事件。

SaveFenceGuard是实际lease/boot/fence和expectedDeleteFence材料的短事务前置，核持久doc状态与隔离；Release后不得用于提交。当时完整DraftCommitStore尚未实现；持续目标第二轮已在同SQL事务重查doc/deleteFence/隔离和head CAS，并完成稳定ID ledger、refs/pin、原回执及平台outbox，guard快照仍不替代数据库事务校验。

cell分配前先查真实文档存在，NOT_FOUND不进入map或后台扫描；调用方取得同doc mutex后仍重读生命周期/delete/隔离权威。仅真实文档保留单调fence，避免随机缺失ID永久积累。


第三次只追加迁移保存edit_lock_receipts、edit_lock_event_revisions和edit_lock_audit。租约授权权威仍仅当前boot内存，原回执中的不可变lease只作历史操作结果，重启不重建；状态事件revision跨boot单调。原回执scope绑定caller/actor/site/epoch/op，用户请求全局scope防跨文档复用，后台过期每篇独立scope且摘要包含可信now。状态变化、原回执和审计与平台outbox在同一本域SQLite事务，不持数据库事务跨HTTP或文件IO。未确认Commit的内存候选只在真实原回执确定后安装，保存guard同步解决pending。

## 持续目标第二轮：真实共享草稿事务

只追加0004迁移，不改变DocumentBody schemaVersion=1或原8字段JSONL格式。站点树版本由content_tree_state统一持久，document_metadata保存完整元数据；已有documents行没有元数据/树版本时失败关闭，不猜补历史行。当前已实现的0002/0003没有文档生产者或旧业务数据，创建生产者从真实空库建立一致状态。迁移不修改既有迁移、不造假文档、正文或COMMITTED资格。

Create/Save在draft_operation_intents先固定动作、caller/actor/site/epoch/op、请求摘要、doc/revision ID、CAS、稳定ID及精确引用集合；该表不存正文。创建固定基准语言和fractional order，站点treeRevision与doc不存在谓词在短SQL复核；保存只能更新真实存在文档并使用当前内存租约。document_id_ledger永久保留retired ID，全部节点ID退役与head/version CAS同事务。draft_resource_references区分当前草稿与不可变历史，原引用JSON保留完整资产hash/长度/MIME。

真实DraftCommitStore延迟打开SQL：先查询COMMITTED原回执，命中释放对象；文件hash/fsync与当前平台窗口读取均在doc guard和SQL之前。保存取真实SaveFenceGuard后才开短SQL，同事务重查文档/deleteFence/隔离、准备材料、旧head/version并提交ledger、COMMITTED资格、引用、原草稿回执、完整Create/Save原结果与平台outbox。创建使用专用受信意图和站点树CAS，绝不以缺文档返回恒成功保存fence。SQL中的COMMITTED资格已权威；JSONL仍允许prepared，启动恢复只根据真实资格补committed，不持SQL跨JSONL写入。

媒体来源只为保存意图开放。完整sourceOperationId/site/object/revision/version/referenceDigest在Acquire前持久；预提交引用按media合同排序后计算规范摘要，pin资产材料核对后固定原pin。COMMITTED后confirm，响应未知时用原op重放完整回执再补确认；未知Commit、依赖故障或正文损坏保留pending与资源保护。仅明确CAS/租约/不存在拒绝且同SQL确认无COMMITTED资格时可原子aborted并阻止迟到提交，再隔离未提交文件并携完整源材料释放；pin未知时允许null而不重新Acquire。COMMITTED即使后来正文隔离，源决定仍为committed；缺资格、损坏或未知不得作为释放证据。

文档资源来源水位使用documents.reference_version，独立于draftVersion。创建初始化为1；保存首次意图固定ExpectedReferenceVersion/SourceVersion，最终同head/ledger/refs事务CAS并推进。发布/撤回等后续引用生产者也必须使用同一水位，不能按草稿版本推导；公共Revision上限耗尽失败。旧行默认0只表示无本轮引用资格，存在旧head却无真实水位时失败关闭，禁止从文件/JSONL猜补；原op重试保持原来源材料，不重新分配水位或Acquire替代pin。

保存首次新操作在持久intent或Acquire pin前读取当前正文限额并按规范UTF8长度拒绝超额请求；COMMITTED原回执仍优先，已prepared材料不因后来降低配额重做。pending重试在任何续作RPC/文件IO前短核原真实租约及head/引用CAS，返回时释放guard；明确失败经统一无COMMITTED证明后封存并Release nullable原pin，未知或正文完整性错误保持资源保护。

## 2026-10-17资源来源查询

只追加0005维护全站reference_scan_state；来源表的插入/更新/删除与原事务一起推进水位，耗尽回滚。每页同SQL快照完整核元数据/head/COMMITTED/回执/引用覆盖，并在签名前释放SQL；签名游标固定过滤、分页大小、主体/会话/代次、水位和位置。跨页水位变化409必须从头，不拼接快照、不持久化读快照。来源扫描各20000预算超限503，不把不完整扫描解释为空。

current_draft与history取真实索引分类；pending_pin来自Acquire前已持久的完整保存意图，retained=true只表示保护，不是COMMITTED。精确COMMITTED资格优先，即使原intent终态尚未补齐；明确aborted/rejected且无COMMITTED才排除。未知已发布/分享图生产资格、缺索引、缺原回执或坏材料均失败关闭。本轮没有实现发布或元数据分享图生产者，也不通过空结果掩盖它们。

两个只读来源HTTP路由使用平台统一服务身份/当前epoch/schema边界；media源决定限定纯服务，列表固定media/site-config/data-protection纯服务或portal即时content.read。RelayOutbox只把真实本域outbox与jobs客户端交给平台单批机制；后台循环和完整启动继续未完成。本轮源码已通过根自部署GitLab CI pipeline 12生成/格式/重复生成、三个实际同时分片覆盖14模块readonly build/vet/gofmt及完整make check（878文件/1435函数/379HTTP/2328类型），未执行SQL/HTTP行为验收或部署。


COMMITTED是不可撤销历史决定；后来正文/文档隔离不撤销完整精确资格、原回执与refs的媒体保护。当前正文读取仍拒绝隔离，缺资格或材料矛盾仍INTEGRITY_ERROR，绝不降pending/abort。

## 2026-10-17第二轮文档树

文档树同快照核真实站点/代次、树水位、原始metadata schema、head/COMMITTED资格、无环与活动父/同父orderKey及slug唯一；活动树按parent/orderKey/ID前序，回收站独立列表，签名cursor绑定主体/会话/权限版本/state/limit、树水位及真实reference_scan_state来源水位，任一水位变化409从头；既有0005触发器覆盖metadata/title/state/head/隔离写，确保SaveDraft仅推进head时也不混完整metadata快照。10000仅为新树查询/操作的明确扫描/输出预算，purged墓碑不计入；超限503不截断。旧Create nil回调无新增总量限制，内部Ensure/Plan可信完整树输入取消maxItems，不新增站点配额。

四个树action只在core按所需功能一次即时identity，DB/锁之前执行；平台仍严格验证服务身份/委托/epoch/schema，仅对实际注册四action不重复授权RPC。trash列表需content.publish，活动读content.read，移动/重排content.edit。真实树写在锁外取得当前写窗口，短SQL做树/metadata CAS、完整原回执和最小content.tree.changed outbox（业务content.edit、jobs事件管理仍沿全域content.publish）；0006仅追加原回执，未知提交按原op查回执。重排要求完整同父活动集合，trash/purged不混入；均匀base62 key不改变slug。

PlanOldPathRedirects完整规划node及所有曾发布后代的无语言前缀旧路径→稳定docId，未知祖先slug失败关闭；非空规划因真实版本化redirect manifest生产链未齐而使Move明确503、不提交，Move及文件继续partial。没有新增假manifest、发布生产者或消费者；当前事件只可靠排队，完整应用因写窗口/全量handler/恢复/后台未齐仍blocked。本轮源码已通过GitLab pipeline23生成/格式/重复生成、三并行分片14模块readonly build/vet/gofmt及完整make check；pipeline21真实编译失败、pipeline22取消均未计通过。未做业务验收或部署，缺可信写窗口在原回执后、新写前503。公共纯服务本站传递仍是后续协议前置，当前不声称跨服务来源链已联通。

## 2026-10-17第三轮metadata

元数据编辑在DB/锁前一次即时content.edit，同代完整scope+规范请求摘要优先回放完整原结果，核result_hash与严格结果schema；新写锁外取得当前平台窗口，短SQL核活动文档、真实metadata/tree CAS并提交标题/SEO/路径、原结果和最小tree.changed。人工SEO非null为manual（包括空串），null为pending；不触发模型或发布共享正文。

仅根级保留navigation、ask；子级允许，同父活动slug唯一。robots.txt/llms.txt/sitemap.xml由slug字符规则自然排除，不另禁无点词、api/admin/v1等。校验使用调用方同SQL可信完整树，纯助手复用给Move，未来发布也沿同规则。PUT metadata与既有树入口均由core一次即时授权，公共边界仍校身份/epoch/schema，无私有缓存marker。

只追加0007保存完整同代原回执，无影子树/正文/媒体来源。shareImage实际改变因精确metadata pin/引用源协议未齐503，原完整ref可保持；曾发布节点或后代路径变化因真实版本化redirect manifest未齐503，Update整体继续partial。当前写窗口缺失新写503，完整启动/行为验收/部署未完成；本轮已通过GitLab pipeline32生成/重复生成、三个并行分片14模块readonly build/vet/gofmt与完整check；pipeline30取消、31 Go过但缺两类型导致整体检查失败均如实记录，生成物精确回传不手改。根同轮补可信服务本站签名与jobs控制proof清理，只是源码/结构证据，未运行真实跨域或业务验收。
