# content 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 73 个：手写 46、UI 12、生成 5、配置 10。函数 259 个：约束 42，参考 217。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/content/internal/document_service.go` | 手写 | 新建共享草稿文档 | `content.CreateDocument`、`content.draftWriteContext`（参考）、`content.contentServiceContext`（参考）、`content.readDraftSiteConfig`（参考）、`content.planDocumentCreation`（参考）、`content.readCreationTree`（参考）、`content.fractionalDocumentOrder`（参考）、`content.validateCreateTransaction`（参考） |
| `modules/content/internal/tree_service.go` | 手写 | 后台读取整站文档树 | `content.ReadDocumentTree`、`content.ListDocuments`、`content.MoveDocument`、`content.ReorderChildren`、`content.NewTreeRuntime`（参考）、`content.WithTreeRuntime`（参考）、`content.treeRuntimeFromContext`（参考）、`content.authorizeTree`（参考）、`content.beginTreeRead`（参考）、`content.readTreeSnapshot`（参考）、`content.validateTreeSiblings`（参考）、`content.treePreorder`（参考）、`content.readTreeReceipt`（参考）、`content.executeTreeMutation`（参考）、`content.planTreeMove`（参考）、`content.planTreeReorder`（参考）、`content.reorderedDocumentKey`（参考） |
| `modules/content/internal/tree_validator.go` | 手写 | 检查树无环和层级约束 | `content.EnsureTreeAcyclic`（参考）、`content.documentTreeDepths`（参考） |
| `modules/content/internal/metadata_service.go` | 手写 | 修改标题/路径/SEO描述/分享图 | `content.UpdateDocumentMetadata`、`content.readMetadataReceipt`（参考）、`content.sameMetadataAsset`（参考） |
| `modules/content/internal/path_service.go` | 手写 | 校验英文可读路径与同父唯一 | `content.ValidatePathSegment`（参考）、`content.PlanOldPathRedirects`（参考）、`content.validatePathSnapshot`（参考）、`content.commitRedirectManifest`（参考）、`content.canonicalDocumentPaths`（参考） |
| `modules/content/internal/edit_lock_service.go` | 手写 | 单leader内存租约、共享序号和同文档提交guard；改变租约统一经可靠操作事务 | `content.NewEditLockManager`（参考）、`content.validateEditLeader`（参考）、`content.lockEditCell`（参考）、`content.AcquireLease`（参考）、`content.RenewLease`（参考）、`content.ReleaseLease`（参考）、`content.ForceReleaseLease`（参考）、`content.ExpireLeases`（参考）、`content.AcquireSaveFence`（参考）、`content.AcquireDocumentFence`（参考）、`content.DocumentState`（参考）、`content.ReleaseFence`（参考）、`content.matchEditLease`（参考）、`content.requireEditableDocument`（参考）、`content.advanceEditFence`（参考）、`content.editLeaseExpired`（参考）、`content.refreshEditLease`（参考）、`content.copyEditLease`（参考）、`content.editLockSnapshot`（参考）、`content.CloseEditLocks`（参考） |
| `modules/content/internal/draft_service.go` | 手写 | 读取全篇共享草稿及编辑状态 | `content.ReadDraft`、`content.SaveDraft`、`content.readDraftSnapshot`（参考）、`content.planDraftSave`（参考）、`content.executeDraftCommit`（参考）、`content.draftMediaReferences`（参考）、`content.draftPinSource`（参考）、`content.acquireDraftPin`（参考）、`content.confirmDraftPin`（参考）、`content.replaySavedDraft`（参考）、`content.abortDraftOperation`（参考）、`content.releaseDraftPin`（参考）、`content.isolateAbortedDraft`（参考）、`content.checkPendingDraftSave`（参考）、`content.finishDraftSaveFailure`（参考） |
| `modules/content/internal/block_validator.go` | 手写 | 验证全部Tiptap块及稳定ID | `content.ValidateBlockTree`（参考）、`content.ValidateInline`（参考）、`content.visit`（参考）、`content.validateInlineNodes`（参考）、`content.validateContentLink`（参考）、`content.validateContentMath`（参考）、`content.contentInteger`（参考）、`content.contentNumber`（参考） |
| `modules/content/internal/table_validator.go` | 手写 | 验证表格合并单元格覆盖 | `content.ValidateTableGrid`（参考）、`content.validateTableGridNode`（参考） |
| `modules/content/internal/columns_validator.go` | 手写 | 校验2–4栏比例和禁止嵌套 | `content.ValidateColumns`（参考）、`content.validateColumnsNode`（参考）、`content.hasNestedColumns`（参考） |
| `modules/content/internal/reference_extractor.go` | 手写 | 扫描所有嵌套块资源引用 | `content.CollectResourceReferences`（参考）、`content.visitReferenceNode`（参考）、`content.appendResourceReference`（参考） |
| `modules/content/internal/body_canonicalizer.go` | 手写 | 规范序列化正文并计算hash | `content.CanonicalizeBody`（参考）、`content.HashBlock`（参考）、`content.bodyJSONValue`（参考）、`content.canonicalContentValue`（参考）、`content.orderContentMarks`（参考） |
| `modules/content/internal/revision_files.go` | 手写 | 持久写权威正文文件 | `content.WriteRevisionFile`（参考）、`content.RecoverPreparedCommits`（参考）、`content.revisionPreparationDigest`（参考）、`content.validatePreparedMaterial`（参考）、`content.newRevisionWriteWindow`（参考）、`content.persistPreparedRevision`（参考）、`content.verifyRevisionMaterialFile`（参考）、`content.quarantineRevisionMaterial`（参考）、`content.scanCommitJournal`（参考）、`content.appendJournalState`（参考）、`content.openRevisionJournal`（参考）、`content.validateRevisionJournalFile`（参考）、`content.sameJournalIdentity`（参考）、`content.moveBodyToQuarantine`（参考）、`content.syncDirectory`（参考）、`content.revisionBodyPath`（参考） |
| `modules/content/internal/revision_repository.go` | 手写 | 原子提交共享草稿head | `content.CommitDraftJournal`（参考）、`content.WithDraftCommitStore`（参考）、`content.draftCommitStoreFromContext`（参考）、`content.validateDraftCommitInput`（参考）、`content.validateDraftCommitStore`（参考）、`content.verifyPreparedRevisionFile`（参考）、`content.checkDraftCommitIOContext`（参考）、`content.draftCommitDigest`（参考）、`content.sameIdentifierPointer`（参考）、`content.normalizeDraftCommitError`（参考） |
| `modules/content/internal/revision_reader.go` | 手写 | 读取COMMITTED正文并核真相 | `content.ReadRevisionBody`（参考）、`content.revisionFileStoreFromContext`（参考）、`content.revisionFileLocation`（参考）、`content.validateCommittedMaterial`（参考）、`content.readImmutableRevision`（参考）、`content.Read`（参考）、`content.decodeStoredBody`（参考）、`content.isolateRevisionRead`（参考）、`content.WithRevisionFileStore`（参考） |
| `modules/content/internal/publication_metadata.go` | 手写 | 人工优先准备首次slug和SEO | `content.PreparePublicationMetadata`（参考） |
| `modules/content/internal/publication_service.go` | 手写 | 点击发布才公开精确共享草稿版本 | `content.PublishDocument`、`content.WithdrawDocument`、`content.authorizePublication`（参考）、`content.readPublicationReceipt`（参考）、`content.insertPublicationReceipt`（参考） |
| `modules/content/internal/publication_repository.go` | 手写 | 原子提交当前发布快照与署名历史 | `content.CommitPublication`（参考）、`content.applyPublication`（参考）、`content.loadSharedDraft`（参考）、`content.casPublicationIdentity`（参考）、`content.copyPublishedReferences`（参考）、`content.listPublicationReferences`（参考）、`content.archivePublishedReferences`（参考）、`content.enqueuePublication`（参考）、`content.enqueueTreeChanged`（参考）、`content.ReadReferenceDecision` |
| `modules/content/internal/visibility_service.go` | 手写 | 同步核公开引用是否仍可见且当前 | `content.ValidatePublications`、`content.readPublicationSnapshot`（参考） |
| `modules/content/internal/publication_reader.go` | 手写 | 获取仍公开的精确正文快照 | `content.ReadPublishedRevision`、`content.loadVisiblePublication`（参考） |
| `modules/content/internal/trash_service.go` | 手写 | 删除进回收站，可选旧URL替代跳转 | `content.TrashDocument`、`content.RestoreDocument`、`content.PurgeDocuments` |
| `modules/content/internal/history_service.go` | 手写 | 查看署名发布/git版本历史 | `content.ReadRevisionHistory`、`content.DiffRevisions`、`content.RollbackPublishedRevision` |
| `modules/content/internal/preview_service.go` | 手写 | 管理员预览草稿公开样式 | `content.CreatePreviewGrant`、`content.ValidatePreviewGrant` |
| `modules/content/internal/reference_reader.go` | 手写 | 向资源库列出哪些文档引用文件 | `content.ListResourceReferences`、`content.NewReferenceRuntime`（参考）、`content.WithReferenceRuntime`（参考）、`content.referenceRuntimeFromContext`（参考）、`content.readReferenceCursor`（参考）、`content.validateReferenceCoverage`（参考）、`content.readReferenceSources`（参考）、`content.readPublicationReferences`（参考）、`content.readCommittedReferenceSource`（参考）、`content.referenceSortKey`（参考） |
| `modules/content/internal/object_reader.go` | 手写 | 经对象API读取私有文件而不共享目录 | `content.ReadContentObject` |
| `modules/content/internal/outbox_repository.go` | 手写 | 与发布/撤回提交原子登记最小事件 | `content.PublishContentEvent`（参考）、`content.bindPublicationTx`（参考）、`content.RelayOutbox`、`content.WithRelayRuntime`（参考） |
| `modules/content/internal/barrier_service.go` | 手写 | 取得可恢复的数据屏障 | `content.AcquireBarrier`、`content.ReleaseBarrier` |
| `modules/content/internal/snapshot_service.go` | 手写 | 导出屏障水位的一致快照 | `content.ExportSnapshot`、`content.StageSnapshot`、`content.DiscardSnapshot` |
| `modules/content/internal/public_route_reader.go` | 手写 | 公开导航读取已发布文档树 | `content.ReadPublicTree`、`content.ResolvePublicPath` |
| `modules/content/web/src/editor/editor-runtime.ts` | UI | 创建Tiptap3/Vue编辑器会话 | `content.createEditorSession`（参考） |
| `modules/content/web/src/editor/block-actions.ts` | UI | 拖拽手柄菜单排序/转换/复制/删除 | `content.applyBlockAction`（参考） |
| `modules/content/web/src/editor/slash-menu.ts` | UI | slash插入菜单与Markdown输入快捷键 | `content.resolveSlashMenu`（参考） |
| `modules/content/web/src/editor/paste-adapter.ts` | UI | Word/网页/Markdown粘贴清理并转换块 | `content.normalizePastedContent`（参考） |
| `modules/content/web/src/editor/autosave-controller.ts` | UI | 去抖自动保存共享草稿 | `content.scheduleAutosave`（参考） |
| `modules/content/web/src/editor/lock-client.ts` | UI | 每30秒锁心跳与15分钟空闲提醒 | `content.maintainEditHeartbeat`（参考）、`content.applyForcedReadOnly`（参考） |
| `modules/content/web/src/editor/internal-link-picker.ts` | UI | 插入文档选择器生成ID站内链接 | `content.selectInternalLink`（参考） |
| `modules/content/web/src/editor/image-uploader.ts` | UI | 浏览器生成多宽WebP连原图上传 | `content.prepareImageUploads`（参考） |
| `modules/content/web/src/editor/drawing-wrapper.ts` | UI | Vue包装Excalidraw并按需加载 | `content.openDrawingEditor`（参考）、`content.saveDrawingFiles`（参考） |
| `modules/content/web/src/editor/video-insertion.ts` | UI | 统一上传/平台视频插入 | `content.prepareVideoBlock`（参考） |
| `modules/content/web/src/editor/columns-resizer.ts` | UI | 调整2–4栏宽比例 | `content.resizeColumns`（参考） |
| `modules/content/cmd/content/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `content.main` |
| `modules/content/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `content.BuildApplication` |
| `modules/content/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `content.Serve`、`content.Shutdown`、`content.Health`、`content.Ready`、`content.RegisterReferenceRoutes`（参考）、`content.RegisterTreeRoutes`（参考）、`content.RegisterPublicationRoutes`（参考） |
| `modules/content/go.mod` | 配置 | 独立module nice-wiki/modules/content；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/content/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/content/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/content；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/content/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/content与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/content/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/content/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/content/web/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/content/web/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/content/web/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/content/web/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/content/web/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/content/web/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `content.mountModuleUI` |
| `modules/content/internal/model_client.go` | 手写 | 经模型网关执行一次调用并读取到终态 | `content.InvokeModel`（参考） |
| `modules/content/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/content/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/content/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/content/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
| `modules/content/internal/storage.go` | 手写 | 正文私有SQLite、迁移、准备/提交资格/隔离与独占启动恢复适配 | `content.OpenDatabase`（参考）、`content.RunContentMigrations`（参考）、`content.ConfigureRevisionStorage`（参考）、`content.RevisionRoot`（参考）、`content.RevisionDataEpoch`（参考）、`content.RevisionMaxDocumentBytes`（参考）、`content.RevisionWriteWindow`（参考）、`content.FindRevisionPreparation`（参考）、`content.PrepareRevision`（参考）、`content.MarkRevisionPrepared`（参考）、`content.ReadCommittedRevision`（参考）、`content.QuarantineRevision`（参考）、`content.CurrentHead`（参考）、`content.HasCommittedReceipt`（参考）、`content.QuarantineDocument`（参考）、`content.RecoverDatabaseJournal`（参考）、`content.CloseDatabase`（参考）、`content.findRevisionPreparation`（参考）、`content.checkRevisionScope`（参考）、`content.hasCommittedReceipt`（参考）、`content.preparationJournalState`（参考）、`content.decodePreparation`（参考）、`content.decodeStorageJSON`（参考）、`content.sameStoredDraft`（参考）、`content.contentStorageError`（参考）、`content.validatePreparedJournalCoverage`（参考） |
| `modules/content/migrations/migrations.go` | 手写 | 内嵌本域只追加迁移的唯一来源 | `content.All`（参考） |
| `modules/content/migrations/0001_domain.sql` | 手写 | 初始正文权威schema：绑定、准备、精确已提交索引、回执、head和隔离；不创建文档/锁/提交 | 无 |
| `modules/content/internal/document_edit_state.go` | 手写 | 持久文档存在/生命周期/删除fence及隔离读取 | `content.ReadDocumentEditState`（参考） |
| `modules/content/migrations/0002_document_edit_state.sql` | 手写 | 追加documents身份/生命周期/删除fence表，不插入文档或持久租约 | 无 |
| `modules/content/internal/edit_lock_operations.go` | 手写 | 完整编辑锁用例、即时授权与安全姓名、可靠状态事件/本地审计/原回执及未知提交隔离 | `content.AcquireEditLock`、`content.RenewEditLock`、`content.ReleaseEditLock`、`content.ForceReleaseEditLock`、`content.ExpireIdleLocks`（参考）、`content.WithEditLockRuntime`（参考）、`content.editLockRuntimeFromContext`（参考）、`content.authorizeEditLock`（参考）、`content.executeEditLock`（参考）、`content.planEditLock`（参考）、`content.resolvePendingEditLock`（参考）、`content.readEditLockReceipt`（参考）、`content.commitEditLock`（参考）、`content.enqueueEditLockChange`（参考）、`content.expireEditLocks`（参考） |
| `modules/content/migrations/0003_edit_lock_operations.sql` | 手写 | 只追加编辑原回执、跨boot状态事件修订和本地审计；不恢复权威租约 | 无 |
| `modules/content/internal/draft_commit_store.go` | 手写 | 真实延迟草稿SQL事务、创建保存意图和稳定ID/引用/完整回执原子提交 | `content.WithDraftRuntime`（参考）、`content.draftRuntimeFromContext`（参考）、`content.readDraftOperation`（参考）、`content.prepareDraftOperation`（参考）、`content.draftRequestDigest`（参考）、`content.RevisionSiteID`（参考）、`content.BeginDraftCommit`（参考）、`content.FindReceipt`（参考）、`content.ValidateFence`（参考）、`content.validateDraftTransaction`（参考）、`content.Commit`（参考）、`content.commitStableIDs`（参考）、`content.releaseDraftTransaction`（参考）、`content.Rollback`（参考）、`content.validateDraftOperationPlan`（参考）、`content.readDraftOperationFrom`（参考） |
| `modules/content/migrations/0004_draft_commits.sql` | 手写 | 只追加站点树版本、元数据、草稿意图、历史IDledger与引用权威表 | 无 |
| `modules/content/migrations/0005_reference_snapshots.sql` | 手写 | 只追加全站来源扫描单调水位；触发器在原来源事务内推进，耗尽回滚；不持久化读快照 | 无 |
| `modules/content/migrations/0006_tree_operations.sql` | 手写 | 只追加树操作完整scope/动作/摘要与原结果回执；不另建影子树或虚假manifest | 无 |
| `modules/content/migrations/0007_metadata_operations.sql` | 手写 | 只追加完整同代metadata原回执：scope/caller/actor/request hash/result/hash，同真实CAS/outbox事务，不建资源影子来源 | 无 |
| `modules/content/migrations/0008_publication_snapshots.sql` | 手写 | 只追加发布快照、署名历史、已发布引用和版本化重定向清单 | 无 |
