# content 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=57；分类={"handwritten": 34, "ui": 12, "config": 9, "generated": 2}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/content/internal/document_service.go` | handwritten | 新建共享草稿文档 | `content.CreateDocument` |
| `modules/content/internal/tree_service.go` | handwritten | 后台读取整站文档树 | `content.ReadDocumentTree`, `content.ListDocuments`, `content.MoveDocument`, `content.ReorderChildren` |
| `modules/content/internal/tree_validator.go` | handwritten | 检查树无环和层级约束 | `content.EnsureTreeAcyclic` |
| `modules/content/internal/metadata_service.go` | handwritten | 修改标题/路径/SEO描述/分享图 | `content.UpdateDocumentMetadata` |
| `modules/content/internal/path_service.go` | handwritten | 校验英文可读路径与同父唯一 | `content.ValidatePathSegment`, `content.PlanOldPathRedirects` |
| `modules/content/internal/edit_lock_service.go` | handwritten | 打开文档自动获得内存编辑锁 | `content.AcquireEditLock`, `content.RenewEditLock`, `content.ReleaseEditLock`, `content.ForceReleaseEditLock`, `content.ExpireIdleLocks` |
| `modules/content/internal/draft_service.go` | handwritten | 读取全篇共享草稿及编辑状态 | `content.ReadDraft`, `content.SaveDraft` |
| `modules/content/internal/block_validator.go` | handwritten | 验证全部Tiptap块及稳定ID | `content.ValidateBlockTree`, `content.ValidateInline` |
| `modules/content/internal/table_validator.go` | handwritten | 验证表格合并单元格覆盖 | `content.ValidateTableGrid` |
| `modules/content/internal/columns_validator.go` | handwritten | 校验2–4栏比例和禁止嵌套 | `content.ValidateColumns` |
| `modules/content/internal/reference_extractor.go` | handwritten | 扫描所有嵌套块资源引用 | `content.CollectResourceReferences` |
| `modules/content/internal/body_canonicalizer.go` | handwritten | 规范序列化正文并计算hash | `content.CanonicalizeBody`, `content.HashBlock` |
| `modules/content/internal/revision_files.go` | handwritten | 持久写权威正文文件 | `content.WriteRevisionFile`, `content.RecoverPreparedCommits` |
| `modules/content/internal/revision_repository.go` | handwritten | 原子提交共享草稿head | `content.CommitDraftJournal` |
| `modules/content/internal/revision_reader.go` | handwritten | 读取COMMITTED正文并核真相 | `content.ReadRevisionBody` |
| `modules/content/internal/publication_metadata.go` | handwritten | 人工优先准备首次slug和SEO | `content.PreparePublicationMetadata` |
| `modules/content/internal/publication_service.go` | handwritten | 点击发布才公开精确共享草稿版本 | `content.PublishDocument`, `content.WithdrawDocument` |
| `modules/content/internal/publication_repository.go` | handwritten | 原子提交当前发布快照与署名历史 | `content.CommitPublication` |
| `modules/content/internal/visibility_service.go` | handwritten | 同步核公开引用是否仍可见且当前 | `content.ValidatePublications` |
| `modules/content/internal/publication_reader.go` | handwritten | 获取仍公开的精确正文快照 | `content.ReadPublishedRevision` |
| `modules/content/internal/trash_service.go` | handwritten | 删除进回收站，可选旧URL替代跳转 | `content.TrashDocument`, `content.RestoreDocument`, `content.PurgeDocuments` |
| `modules/content/internal/history_service.go` | handwritten | 查看署名发布/git版本历史 | `content.ReadRevisionHistory`, `content.DiffRevisions`, `content.RollbackPublishedRevision` |
| `modules/content/internal/preview_service.go` | handwritten | 管理员预览草稿公开样式 | `content.CreatePreviewGrant`, `content.ValidatePreviewGrant` |
| `modules/content/internal/reference_reader.go` | handwritten | 向资源库列出哪些文档引用文件 | `content.ListResourceReferences` |
| `modules/content/internal/object_reader.go` | handwritten | 经对象API读取私有文件而不共享目录 | `content.ReadContentObject` |
| `modules/content/internal/outbox_repository.go` | handwritten | 与发布/撤回提交原子登记最小事件 | `content.PublishContentEvent` |
| `modules/content/internal/barrier_service.go` | handwritten | 取得可恢复的数据屏障 | `content.AcquireBarrier`, `content.ReleaseBarrier` |
| `modules/content/internal/snapshot_service.go` | handwritten | 导出屏障水位的一致快照 | `content.ExportSnapshot`, `content.PrepareSnapshot` |
| `modules/content/internal/snapshot_validator.go` | handwritten | 验证快照可恢复性 | `content.ValidateSnapshot` |
| `modules/content/internal/snapshot_repository.go` | handwritten | 激活已准备恢复数据 | `content.CommitSnapshot`, `content.RollbackSnapshot` |
| `modules/content/internal/public_route_reader.go` | handwritten | 公开导航读取已发布文档树 | `content.ReadPublicTree`, `content.ResolvePublicPath` |
| `modules/content/web/src/editor/editor-runtime.ts` | ui | 创建Tiptap3/Vue编辑器会话 | `content.createEditorSession` |
| `modules/content/web/src/editor/block-actions.ts` | ui | 拖拽手柄菜单排序/转换/复制/删除 | `content.applyBlockAction` |
| `modules/content/web/src/editor/slash-menu.ts` | ui | slash插入菜单与Markdown输入快捷键 | `content.resolveSlashMenu` |
| `modules/content/web/src/editor/paste-adapter.ts` | ui | Word/网页/Markdown粘贴清理并转换块 | `content.normalizePastedContent` |
| `modules/content/web/src/editor/autosave-controller.ts` | ui | 去抖自动保存共享草稿 | `content.scheduleAutosave` |
| `modules/content/web/src/editor/lock-client.ts` | ui | 每30秒锁心跳与15分钟空闲提醒 | `content.maintainEditHeartbeat`, `content.applyForcedReadOnly` |
| `modules/content/web/src/editor/internal-link-picker.ts` | ui | 插入文档选择器生成ID站内链接 | `content.selectInternalLink` |
| `modules/content/web/src/editor/image-uploader.ts` | ui | 浏览器生成多宽WebP连原图上传 | `content.prepareImageUploads` |
| `modules/content/web/src/editor/drawing-wrapper.ts` | ui | Vue包装Excalidraw并按需加载 | `content.openDrawingEditor`, `content.saveDrawingFiles` |
| `modules/content/web/src/editor/video-insertion.ts` | ui | 统一上传/平台视频插入 | `content.prepareVideoBlock` |
| `modules/content/web/src/editor/columns-resizer.ts` | ui | 调整2–4栏宽比例 | `content.resizeColumns` |
| `modules/content/cmd/content/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M04-F901` |
| `modules/content/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M04-F902` |
| `modules/content/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M04-F903`, `M04-F904`, `M04-F905`, `M04-F906` |
| `modules/content/go.mod` | config | 独立module nice-wiki/modules/content；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/content/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/content/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/content；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/content/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/content与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/content/config/service.defaults.json` | config | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/content/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/content/web/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/content/web/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/content/web/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/content/web/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/content/web/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/content/web/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M04-F907` |
