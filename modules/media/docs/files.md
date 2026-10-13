# media 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=42；分类={"handwritten": 36, "config": 5, "generated": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/media/internal/upload_service.go` | handwritten | 开始内容寻址可续传上传 | `media.BeginUpload`, `media.ReadUpload`, `media.AbortUpload` |
| `modules/media/internal/upload_validator.go` | handwritten | 清理展示文件名而不用于路径 | `media.NormalizeOriginalName`, `media.HashUpload` |
| `modules/media/internal/chunk_writer.go` | handwritten | 顺序原子写有界上传块 | `media.PutUploadChunk` |
| `modules/media/internal/upload_completion.go` | handwritten | 完成hash/MIME验证后发布ready资源 | `media.CompleteUpload` |
| `modules/media/internal/upload_maintenance.go` | handwritten | 仅回收过期未完成临时字节 | `media.ExpireUploadSessions` |
| `modules/media/internal/mime_validator.go` | handwritten | 从字节识别类型 | `media.SniffAssetType` |
| `modules/media/internal/payload_validator.go` | handwritten | 验证SVG/场景/附件/视频安全交付 | `media.ValidateAssetPayload` |
| `modules/media/internal/blob_repository.go` | handwritten | 将已验证文件提交不可变内容寻址存储 | `media.CommitAssetBlob` |
| `modules/media/internal/asset_service.go` | handwritten | 读取资源/variants安全元数据 | `media.ReadAsset`, `media.ListAssets` |
| `modules/media/internal/image_variants.go` | handwritten | 关联浏览器多宽WebP原图 | `media.AttachImageVariants` |
| `modules/media/internal/drawing_validator.go` | handwritten | 校验Excalidraw场景和提取文字 | `media.ValidateDrawingScene` |
| `modules/media/internal/reference_service.go` | handwritten | 正文/config提交前保护资源迟到引用 | `media.AcquireResourceReferences`, `media.ConfirmResourceReferences`, `media.ReleaseResourceReferences` |
| `modules/media/internal/reference_repository.go` | handwritten | 消费引用事件防重复/乱序 | `media.ApplyReferenceEvent` |
| `modules/media/internal/reference_reader.go` | handwritten | 列哪些文档当前/历史引用资源 | `media.ReadAssetReferences` |
| `modules/media/internal/cleanup_service.go` | handwritten | 后台单列当前未引用文件 | `media.ListUnusedAssets`, `media.ProposeManualCleanup`, `media.ConfirmManualCleanup`, `media.CancelManualCleanup` |
| `modules/media/internal/cleanup_sources.go` | handwritten | 确认所有权威引用源后才认定可清理 | `media.ValidateCleanupSources` |
| `modules/media/internal/cleanup_worker.go` | handwritten | 只执行已人工确认的安全清理计划 | `media.ExecuteManualCleanup` |
| `modules/media/internal/range_service.go` | handwritten | 规范Range支持iOS视频 | `media.ParseByteRange` |
| `modules/media/internal/asset_delivery.go` | handwritten | 鉴权读取immutable bytes和Range | `media.ReadAssetBytes`, `media.HeadAssetBytes` |
| `modules/media/internal/reader_lease.go` | handwritten | 防正在下载资源被最终删除 | `media.AcquireReaderLease` |
| `modules/media/internal/manifest_service.go` | handwritten | 给公开/管理员预览返回SHA URLs/srcset/封面 | `media.BuildDeliveryManifest` |
| `modules/media/internal/video_service.go` | handwritten | 插入时一次获取平台公开信息并本地封面 | `media.ReadVideoMetadata` |
| `modules/media/internal/provider_registry.go` | handwritten | 识别Bilibili/YouTube/抖音/TikTok链接 | `media.RecognizePlatformURL` |
| `modules/media/internal/provider_url_guard.go` | handwritten | 仅平台短链接受控重定向解析 | `media.ResolveProviderRedirect` |
| `modules/media/internal/provider_fetch.go` | handwritten | provider接口抓公开非登录信息 | `media.FetchProviderPublicMetadata` |
| `modules/media/internal/provider_embed.go` | handwritten | 生成四平台白名单embed资料 | `media.BuildProviderEmbed` |
| `modules/media/internal/cover_import.go` | handwritten | 下载平台封面到本地内容寻址资源 | `media.ImportRemoteCover` |
| `modules/media/internal/blob_recovery.go` | handwritten | 恢复blob完成/删除持久日志 | `media.RecoverBlobJournal` |
| `modules/media/internal/outbox_repository.go` | handwritten | 原子写最小资源状态outbox | `media.PublishAssetEvent` |
| `modules/media/internal/barrier_service.go` | handwritten | 取得可恢复的数据屏障 | `media.AcquireBarrier`, `media.ReleaseBarrier` |
| `modules/media/internal/snapshot_service.go` | handwritten | 导出屏障水位的一致快照 | `media.ExportSnapshot`, `media.PrepareSnapshot` |
| `modules/media/internal/snapshot_validator.go` | handwritten | 验证快照可恢复性 | `media.ValidateSnapshot` |
| `modules/media/internal/snapshot_repository.go` | handwritten | 激活已准备恢复数据 | `media.CommitSnapshot`, `media.RollbackSnapshot` |
| `modules/media/cmd/media/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M05-F901` |
| `modules/media/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M05-F902` |
| `modules/media/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M05-F903`, `M05-F904`, `M05-F905`, `M05-F906` |
| `modules/media/go.mod` | config | 独立module nice-wiki/modules/media；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/media/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/media/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/media；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/media/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/media与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/media/config/service.defaults.json` | config | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/media/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
