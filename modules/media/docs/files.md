# media 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 54 个：手写 44、生成 3、配置 6、迁移 1。函数 171 个：约束 36，参考 135。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/media/internal/upload_service.go` | 手写 | 开始内容寻址可续传上传 | `media.BeginUpload`、`media.ReadUpload`、`media.AbortUpload`、`media.hashBeginUploadRequest`（参考）、`media.validateUploadActor`（参考）、`media.readUploadQuota`（参考）、`media.hasGrant`（参考） |
| `modules/media/internal/upload_validator.go` | 手写 | 清理展示文件名而不用于路径 | `media.NormalizeOriginalName`（参考）、`media.HashUpload`（参考） |
| `modules/media/internal/chunk_writer.go` | 手写 | 顺序原子写有界上传块 | `media.PutUploadChunk`、`media.mediaWriteContext`（参考）、`media.hashChunkRequest`（参考）、`media.readChunkOperation`（参考）、`media.chunkContextReader.Read`（参考）、`media.chunkStageName`（参考）、`media.writeChunkStage`（参考）、`media.reconcileUploadFile`（参考）、`media.verifyUploadChunks`（参考）、`media.commitChunk`（参考）、`media.RecoverUploadChunks`（参考） |
| `modules/media/internal/upload_completion.go` | 手写 | 完成hash/MIME验证后发布ready资源 | `media.CompleteUpload`、`media.hashCompleteRequest`（参考）、`media.replayCompletion`（参考）、`media.completionRejectionCode`（参考）、`media.rejectCompletion`（参考）、`media.casCompleteSession`（参考）、`media.writeCompletionReceipt`（参考） |
| `modules/media/internal/upload_maintenance.go` | 手写 | 仅回收过期未完成临时字节 | `media.ExpireUploadSessions`（参考） |
| `modules/media/internal/mime_validator.go` | 手写 | 从字节识别类型 | `media.SniffAssetType`（参考）、`media.invalidMedia`（参考）、`media.inspectPNG`（参考）、`media.inspectGIF`（参考）、`media.inspectWebP`（参考）、`media.readMediaBox`（参考）、`media.inspectBrands`（参考）、`media.inspectBMFF`（参考）、`media.readEBMLNumber`（参考）、`media.inspectWebM`（参考）、`media.validateMP4SampleEntry`（参考）、`media.readMP4Descriptor`（参考）、`media.validateAACDescriptor`（参考）、`media.validateAACConfig`（参考）、`media.validateWebMTrack`（参考） |
| `modules/media/internal/payload_validator.go` | 手写 | 验证SVG/场景/附件/视频安全交付 | `media.ValidateAssetPayload`（参考）、`media.validateSVG`（参考）、`media.validateSVGCSS`（参考）、`media.safeDrawingLink`（参考） |
| `modules/media/internal/blob_repository.go` | 手写 | 将已验证文件提交不可变内容寻址存储；私有SQLite保存上传会话、资源元数据、回执与blob日志，COMMITTED与资源元数据同事务 | `media.CommitAssetBlob`（参考）、`media.InstallLocalBlobStore`（参考）、`media.openLocalBlobStore`（参考）、`media.readUploadRow`（参考）、`media.writeUploadRow`（参考）、`media.ensureMediaDirectory`（参考）、`media.openMediaRoot`（参考）、`media.openMediaRegular`（参考）、`media.ConfigureWriteWindow`（参考）、`media.CloseLocalBlobStore`（参考）、`media.LocalStorageRecovered`（参考）、`media.replayBeginUpload`（参考）、`media.CreateUpload`（参考）、`media.OpenUpload`（参考）、`media.CommitVerifiedBlob`（参考）、`media.findReadyAsset`（参考）、`media.hashFile`（参考）、`media.currentBlobStore`（参考）、`media.appendJournal`（参考）、`media.blobPath`（参考）、`media.syncDirectory`（参考） |
| `modules/media/internal/asset_service.go` | 手写 | 读取资源/variants安全元数据 | `media.ReadAsset`、`media.ListAssets` |
| `modules/media/internal/image_variants.go` | 手写 | 关联浏览器多宽WebP原图 | `media.AttachImageVariants` |
| `modules/media/internal/drawing_validator.go` | 手写 | 校验Excalidraw场景和提取文字 | `media.ValidateDrawingScene`（参考）、`media.decodeDrawingRaster`（参考）、`media.validateDrawingRaster`（参考） |
| `modules/media/internal/reference_service.go` | 手写 | 正文/config提交前保护资源迟到引用 | `media.AcquireResourceReferences`、`media.ConfirmResourceReferences`、`media.ReleaseResourceReferences`、`media.validateReferenceSource`（参考）、`media.canonicalReferenceInput`（参考）、`media.referenceBindingHash`（参考） |
| `modules/media/internal/reference_repository.go` | 手写 | 消费引用事件防重复/乱序 | `media.ApplyReferenceEvent`、`media.lockReferenceKey`（参考）、`media.lockReferenceAssets`（参考）、`media.readReferenceSource`（参考）、`media.acquireReferencePin`（参考）、`media.readReferenceAsset`（参考）、`media.readReferenceRecord`（参考）、`media.readReferenceResolution`（参考）、`media.writeReferenceResolution`（参考）、`media.referenceObjectAssets`（参考）、`media.advanceReferenceGeneration`（参考）、`media.confirmReferencePin`（参考）、`media.releaseReferencePin`（参考） |
| `modules/media/internal/reference_reader.go` | 手写 | 两个权威源完整同水位分页与本地pending引用快照 | `media.ReadAssetReferences`、`media.readSourceReferences`（参考）、`media.pendingAssetReferences`（参考） |
| `modules/media/internal/cleanup_service.go` | 手写 | 人工计划提案/取消、原回执及与新增引用共享的完整计划取消事务；列表/确认后续实施 | `media.ListUnusedAssets`、`media.ProposeManualCleanup`、`media.ConfirmManualCleanup`、`media.CancelManualCleanup`、`media.cleanupStore`（参考）、`media.readCleanupAsset`（参考）、`media.cleanupRequestHash`（参考）、`media.readCleanupReceipt`（参考）、`media.writeCleanupReceipt`（参考）、`media.readCleanupPlan`（参考）、`media.cancelCleanupPlan`（参考）、`media.referenceCleanupAssets`（参考）、`media.assetCleanupPlans`（参考）、`media.cancelReferenceCleanupPlans`（参考）、`media.InstallCleanupJobsClient`（参考）、`media.cleanupJobRequest`（参考）、`media.loadCleanupApproval`（参考）、`media.readCleanupApproval`（参考）、`media.scanCleanupConfirmation`（参考）、`media.blockCleanupProposal`（参考）、`media.updateCleanupState`（参考）、`media.approveCleanupPlan`（参考）、`media.checkCleanupApprovalPending`（参考）、`media.submitCleanupApproval`（参考）、`media.completeCleanupApproval`（参考）、`media.ReadCleanupApprovalObject`（参考） |
| `modules/media/internal/cleanup_sources.go` | 手写 | 确认所有权威引用源后才认定可清理 | `media.ValidateCleanupSources`（参考） |
| `modules/media/internal/cleanup_worker.go` | 手写 | 只执行已人工确认的安全清理计划 | `media.ExecuteManualCleanup`、`media.checkCleanupJobBinding`（参考）、`media.cleanupExecutionMaterial`（参考）、`media.readCleanupExecution`（参考）、`media.cleanupWaitingEqual`（参考）、`media.blockCleanupExecution`（参考）、`media.decideCleanupExecution`（参考）、`media.finishCleanupExecutionLocked`（参考）、`media.removeDecidedBlob`（参考）、`media.recoverCleanupExecutionsLocked`（参考）、`media.confirmDeletedAssetLocked`（参考） |
| `modules/media/internal/range_service.go` | 手写 | 规范Range支持iOS视频 | `media.ParseByteRange`（参考） |
| `modules/media/internal/asset_delivery.go` | 手写 | 鉴权读取immutable bytes和Range | `media.ReadAssetBytes`、`media.HeadAssetBytes` |
| `modules/media/internal/reader_lease.go` | 手写 | 防正在下载资源被最终删除 | `media.AcquireReaderLease`（参考）、`media.releaseReaderLease`（参考） |
| `modules/media/internal/manifest_service.go` | 手写 | 给公开/管理员预览返回SHA URLs/srcset/封面 | `media.BuildDeliveryManifest` |
| `modules/media/internal/video_service.go` | 手写 | 插入时一次获取平台公开信息并本地封面 | `media.ReadVideoMetadata` |
| `modules/media/internal/provider_registry.go` | 手写 | 识别Bilibili/YouTube/抖音/TikTok链接 | `media.RecognizePlatformURL`（参考） |
| `modules/media/internal/provider_url_guard.go` | 手写 | 仅平台短链接受控重定向解析 | `media.ResolveProviderRedirect`（参考） |
| `modules/media/internal/provider_fetch.go` | 手写 | provider接口抓公开非登录信息 | `media.FetchProviderPublicMetadata`（参考） |
| `modules/media/internal/provider_embed.go` | 手写 | 生成四平台白名单embed资料 | `media.BuildProviderEmbed`（参考） |
| `modules/media/internal/cover_import.go` | 手写 | 下载平台封面到本地内容寻址资源 | `media.ImportRemoteCover`（参考） |
| `modules/media/internal/blob_recovery.go` | 手写 | 恢复blob完成/删除持久日志：读取私有库blob_journal，隔离无COMMITTED的PREPARED产物 | `media.RecoverBlobJournal`（参考） |
| `modules/media/internal/outbox_repository.go` | 手写 | 原子写最小资源状态outbox | `media.PublishAssetEvent`（参考）、`media.RelayOutbox` |
| `modules/media/internal/barrier_service.go` | 手写 | 取得可恢复的数据屏障 | `media.AcquireBarrier`、`media.ReleaseBarrier` |
| `modules/media/internal/snapshot_service.go` | 手写 | 导出屏障水位的一致快照 | `media.ExportSnapshot`、`media.StageSnapshot`、`media.DiscardSnapshot` |
| `modules/media/cmd/media/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `media.main` |
| `modules/media/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `media.BuildApplication` |
| `modules/media/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `media.Serve`、`media.Shutdown`、`media.Health`、`media.Ready`、`media.NewApplication`（参考）、`media.serviceHandlers`（参考）、`media.serveUploadChunk`（参考）、`media.writeRequestProblem`（参考） |
| `modules/media/go.mod` | 配置 | 独立module nice-wiki/modules/media；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/media/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/media/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/media；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/media/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/media与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/media/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/media/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/media/internal/event_subscription.go` | 手写 | 订阅并分派本域登记的跨域事件 | `media.ConsumeEvents` |
| `modules/media/internal/worker.go` | 手写 | 主动领取本域持久任务的循环 | `media.RunWorker`、`media.cleanupWorkerOperation`（参考）、`media.cleanupWorkerFailure`（参考） |
| `modules/media/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/media/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/media/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/media/migrations/0001_domain.sql` | 迁移 | 初始权威表（ADR0005私有SQLite）：upload_sessions、upload_receipts、assets、blob_journal与schema_migrations；nw_outbox/nw_inbox由平台EnsureSchema创建不在此登记 | 无 |
| `modules/media/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 | `media.All`（参考） |
| `modules/media/migrations/0002_upload_chunks.sql` | 手写 | 只追加上传创建epoch和分块PREPARED/COMMITTED原结果日志，不重写0001或猜测旧上传代次 | 无 |
| `modules/media/migrations/0003_reference_pins.sql` | 手写 | 只追加引用来源完整绑定/终止封存、pending pin原结果与逐资产fence保护；旧迁移不改 | 无 |
| `modules/media/internal/reference_decision_client.go` | 手写 | 只经固定版本化源决定API/共享schema读取，清proof的media纯服务上下文，不持本域锁 | `media.InstallReferenceDecisionClients`（参考）、`media.readReferenceDecision`（参考） |
| `modules/media/migrations/0004_reference_resolution.sql` | 手写 | 只追加源终局原回执、对象current水位与保留history的确认引用行，原pin保护不删除 | 无 |
| `modules/media/migrations/0005_manual_cleanup.sql` | 手写 | 只追加人工计划、原扫描、精确pending标记及操作原回执权威表；不自动确认/删除 | 无 |
| `modules/media/migrations/0006_cleanup_approvals.sql` | 手写 | 只追加人工计划一次批准、固定延迟任务输入、原任务回执和原waiting结果桥接；取消保留批准证据 | 无 |
| `modules/media/migrations/0007_cleanup_execution.sql` | 手写 | 只追加人工执行/逐资产决定及有限reader租约权威，不产生自动清理授权 | 无 |
