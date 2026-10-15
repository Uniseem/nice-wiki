# media 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 47 个：手写 37、生成 3、配置 6、迁移 1。函数 54 个：约束 36，参考 18。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/media/internal/upload_service.go` | 手写 | 开始内容寻址可续传上传 | `media.BeginUpload`、`media.ReadUpload`、`media.AbortUpload` |
| `modules/media/internal/upload_validator.go` | 手写 | 清理展示文件名而不用于路径 | `media.NormalizeOriginalName`（参考）、`media.HashUpload`（参考） |
| `modules/media/internal/chunk_writer.go` | 手写 | 顺序原子写有界上传块 | `media.PutUploadChunk` |
| `modules/media/internal/upload_completion.go` | 手写 | 完成hash/MIME验证后发布ready资源 | `media.CompleteUpload` |
| `modules/media/internal/upload_maintenance.go` | 手写 | 仅回收过期未完成临时字节 | `media.ExpireUploadSessions`（参考） |
| `modules/media/internal/mime_validator.go` | 手写 | 从字节识别类型 | `media.SniffAssetType`（参考） |
| `modules/media/internal/payload_validator.go` | 手写 | 验证SVG/场景/附件/视频安全交付 | `media.ValidateAssetPayload`（参考） |
| `modules/media/internal/blob_repository.go` | 手写 | 将已验证文件提交不可变内容寻址存储；私有SQLite保存上传会话、资源元数据、回执与blob日志，COMMITTED与资源元数据同事务 | `media.CommitAssetBlob`（参考） |
| `modules/media/internal/asset_service.go` | 手写 | 读取资源/variants安全元数据 | `media.ReadAsset`、`media.ListAssets` |
| `modules/media/internal/image_variants.go` | 手写 | 关联浏览器多宽WebP原图 | `media.AttachImageVariants` |
| `modules/media/internal/drawing_validator.go` | 手写 | 校验Excalidraw场景和提取文字 | `media.ValidateDrawingScene`（参考） |
| `modules/media/internal/reference_service.go` | 手写 | 正文/config提交前保护资源迟到引用 | `media.AcquireResourceReferences`、`media.ConfirmResourceReferences`、`media.ReleaseResourceReferences` |
| `modules/media/internal/reference_repository.go` | 手写 | 消费引用事件防重复/乱序 | `media.ApplyReferenceEvent` |
| `modules/media/internal/reference_reader.go` | 手写 | 列哪些文档当前/历史引用资源 | `media.ReadAssetReferences` |
| `modules/media/internal/cleanup_service.go` | 手写 | 后台单列当前未引用文件 | `media.ListUnusedAssets`、`media.ProposeManualCleanup`、`media.ConfirmManualCleanup`、`media.CancelManualCleanup` |
| `modules/media/internal/cleanup_sources.go` | 手写 | 确认所有权威引用源后才认定可清理 | `media.ValidateCleanupSources`（参考） |
| `modules/media/internal/cleanup_worker.go` | 手写 | 只执行已人工确认的安全清理计划 | `media.ExecuteManualCleanup` |
| `modules/media/internal/range_service.go` | 手写 | 规范Range支持iOS视频 | `media.ParseByteRange`（参考） |
| `modules/media/internal/asset_delivery.go` | 手写 | 鉴权读取immutable bytes和Range | `media.ReadAssetBytes`、`media.HeadAssetBytes` |
| `modules/media/internal/reader_lease.go` | 手写 | 防正在下载资源被最终删除 | `media.AcquireReaderLease`（参考） |
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
| `modules/media/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `media.Serve`、`media.Shutdown`、`media.Health`、`media.Ready` |
| `modules/media/go.mod` | 配置 | 独立module nice-wiki/modules/media；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/media/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/media/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/media；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/media/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/media与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/media/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/media/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/media/internal/event_subscription.go` | 手写 | 订阅并分派本域登记的跨域事件 | `media.ConsumeEvents` |
| `modules/media/internal/worker.go` | 手写 | 主动领取本域持久任务的循环 | `media.RunWorker` |
| `modules/media/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/media/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/media/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/media/migrations/0001_domain.sql` | 迁移 | 初始权威表（ADR0005私有SQLite）：upload_sessions、upload_receipts、assets、blob_journal与schema_migrations；nw_outbox/nw_inbox由平台EnsureSchema创建不在此登记 | 无 |
| `modules/media/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 | `media.All`（参考） |
