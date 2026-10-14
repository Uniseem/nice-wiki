# content 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 55 个：手写 32、UI 12、生成 2、配置 9。函数 72 个：约束 41，参考 31。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/content/internal/document_service.go` | 手写 | 新建共享草稿文档 | `content.CreateDocument` |
| `modules/content/internal/tree_service.go` | 手写 | 后台读取整站文档树 | `content.ReadDocumentTree`、`content.ListDocuments`、`content.MoveDocument`、`content.ReorderChildren` |
| `modules/content/internal/tree_validator.go` | 手写 | 检查树无环和层级约束 | `content.EnsureTreeAcyclic`（参考） |
| `modules/content/internal/metadata_service.go` | 手写 | 修改标题/路径/SEO描述/分享图 | `content.UpdateDocumentMetadata` |
| `modules/content/internal/path_service.go` | 手写 | 校验英文可读路径与同父唯一 | `content.ValidatePathSegment`（参考）、`content.PlanOldPathRedirects`（参考） |
| `modules/content/internal/edit_lock_service.go` | 手写 | 打开文档自动获得内存编辑锁 | `content.AcquireEditLock`、`content.RenewEditLock`、`content.ReleaseEditLock`、`content.ForceReleaseEditLock`、`content.ExpireIdleLocks`（参考） |
| `modules/content/internal/draft_service.go` | 手写 | 读取全篇共享草稿及编辑状态 | `content.ReadDraft`、`content.SaveDraft` |
| `modules/content/internal/block_validator.go` | 手写 | 验证全部Tiptap块及稳定ID | `content.ValidateBlockTree`（参考）、`content.ValidateInline`（参考） |
| `modules/content/internal/table_validator.go` | 手写 | 验证表格合并单元格覆盖 | `content.ValidateTableGrid`（参考） |
| `modules/content/internal/columns_validator.go` | 手写 | 校验2–4栏比例和禁止嵌套 | `content.ValidateColumns`（参考） |
| `modules/content/internal/reference_extractor.go` | 手写 | 扫描所有嵌套块资源引用 | `content.CollectResourceReferences`（参考） |
| `modules/content/internal/body_canonicalizer.go` | 手写 | 规范序列化正文并计算hash | `content.CanonicalizeBody`（参考）、`content.HashBlock`（参考） |
| `modules/content/internal/revision_files.go` | 手写 | 持久写权威正文文件 | `content.WriteRevisionFile`（参考）、`content.RecoverPreparedCommits`（参考） |
| `modules/content/internal/revision_repository.go` | 手写 | 原子提交共享草稿head | `content.CommitDraftJournal`（参考） |
| `modules/content/internal/revision_reader.go` | 手写 | 读取COMMITTED正文并核真相 | `content.ReadRevisionBody`（参考） |
| `modules/content/internal/publication_metadata.go` | 手写 | 人工优先准备首次slug和SEO | `content.PreparePublicationMetadata`（参考） |
| `modules/content/internal/publication_service.go` | 手写 | 点击发布才公开精确共享草稿版本 | `content.PublishDocument`、`content.WithdrawDocument` |
| `modules/content/internal/publication_repository.go` | 手写 | 原子提交当前发布快照与署名历史 | `content.CommitPublication`（参考） |
| `modules/content/internal/visibility_service.go` | 手写 | 同步核公开引用是否仍可见且当前 | `content.ValidatePublications` |
| `modules/content/internal/publication_reader.go` | 手写 | 获取仍公开的精确正文快照 | `content.ReadPublishedRevision` |
| `modules/content/internal/trash_service.go` | 手写 | 删除进回收站，可选旧URL替代跳转 | `content.TrashDocument`、`content.RestoreDocument`、`content.PurgeDocuments` |
| `modules/content/internal/history_service.go` | 手写 | 查看署名发布/git版本历史 | `content.ReadRevisionHistory`、`content.DiffRevisions`、`content.RollbackPublishedRevision` |
| `modules/content/internal/preview_service.go` | 手写 | 管理员预览草稿公开样式 | `content.CreatePreviewGrant`、`content.ValidatePreviewGrant` |
| `modules/content/internal/reference_reader.go` | 手写 | 向资源库列出哪些文档引用文件 | `content.ListResourceReferences` |
| `modules/content/internal/object_reader.go` | 手写 | 经对象API读取私有文件而不共享目录 | `content.ReadContentObject` |
| `modules/content/internal/outbox_repository.go` | 手写 | 与发布/撤回提交原子登记最小事件 | `content.PublishContentEvent`（参考）、`content.RelayOutbox` |
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
| `modules/content/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `content.Serve`、`content.Shutdown`、`content.Health`、`content.Ready` |
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
