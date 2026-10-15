# 数据、权威与持久化

本服务私有siteId/blobs/sha256不可变原图/WebP/附件/视频/scene/亮暗SVG/本地封面。私有数据库meta/blobjournal/uploadsessions/quota/refs/pins/readers/manualplans/receipt/outbox/stages；路径只服务生成，其他域只AssetVersionRef与只读URL/manifest，不共享可写目录。

每文件sha256内容寻址；assetId/version做权限/生命周期索引，sha决定字节真相。imageVariants只关联浏览器已生成readyWebP；anim保持原图。DrawingScene受控完整Excalidraw导出schema，未知持久字段/版本明确迁移不静默丢弃；Drawing块只三份sha ref。PlatformVideoSnapshot只插入时公开metadata，不定时刷新，不模型生成描述（knowledge拥有后续AI描述）。

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
| AssetVersionRef | 完整不可变资源引用；正文按SHA内容寻址，文件版本不覆盖 |
| ImageVariant | 浏览器生成WebP尺寸，最多640/1280/1920且不放大原图 |
| Asset | 资源元数据，不含OSpath；服务无图像处理或视频转码库 |
| UploadSession | 可续传上传，仅临时字节到期可清理，ready文件绝不自动删 |
| BinaryChunk | 上传字节内部句柄 |
| ByteRange | 单范围归一化；支持bytes=N-M/N-/后缀-N，超界416 |
| AssetReference | 引用源版本单调，当前与历史区分 |
| ReferencePin | 提交前pin，final delete共享对象fence |
| ReferenceSnapshot | 未知或不完整不能认定未引用 |
| CleanupPlan | 只能管理员明确手动创建和确认，禁止周期自动清ready文件 |
| PlatformProvider | 四个已确认平台 |
| PlatformVideoSnapshot | 插入时一次公开快照，不自动刷新、不假造信息；失败可手填标题简介 |
| DeliveryItem | 公开/预览资产manifest |
| DeliveryManifest | public-site无需共享media目录 |
| AssetChangedPayload | 最小资源事件 |
| DrawingShape | 完整DrawingShape持久字段；vendor新增字段需声明schema迁移 |
| DrawingText | 完整DrawingText持久字段；vendor新增字段需声明schema迁移 |
| DrawingBinding | 线绑定 |
| DrawingLine | 完整DrawingLine持久字段；vendor新增字段需声明schema迁移 |
| DrawingFreedraw | 完整DrawingFreedraw持久字段；vendor新增字段需声明schema迁移 |
| DrawingImage | 完整DrawingImage持久字段；vendor新增字段需声明schema迁移 |
| DrawingFrame | 完整DrawingFrame持久字段；vendor新增字段需声明schema迁移 |
| DrawingElement | Excalidraw持久图元联合体，不执行link或HTML |
| DrawingScene | .excalidraw导出规范化受支持schema，adapter保留所有已登记持久字段，未知版本只读 |
| ProviderURL | 不含账号cookie/auth/private API |
| EmbedDescriptor | provider生成embed，页面点击后加载 |
| BeginUploadInput | 开始内容寻址可续传上传的输入 |
| BeginUploadResult | 开始内容寻址可续传上传的结果 |
| NormalizeOriginalNameInput | 清理展示文件名而不用于路径的输入 |
| NormalizeOriginalNameResult | 清理展示文件名而不用于路径的结果 |
| ReadUploadInput | 查询续传offset的输入 |
| ReadUploadResult | 查询续传offset的结果 |
| PutUploadChunkInput | 顺序原子写有界上传块的输入 |
| PutUploadChunkResult | 顺序原子写有界上传块的结果 |
| CompleteUploadInput | 完成hash/MIME验证后发布ready资源的输入 |
| CompleteUploadResult | 完成hash/MIME验证后发布ready资源的结果 |
| AbortUploadInput | 取消未完成上传释放配额的输入 |
| AbortUploadResult | 取消未完成上传释放配额的结果 |
| ExpireUploadSessionsInput | 仅回收过期未完成临时字节的输入 |
| ExpireUploadSessionsResult | 仅回收过期未完成临时字节的结果 |
| HashUploadInput | 流计算对象真相摘要的输入 |
| HashUploadResult | 流计算对象真相摘要的结果 |
| SniffAssetTypeInput | 从字节识别类型的输入 |
| SniffAssetTypeResult | 从字节识别类型的结果 |
| ValidateAssetPayloadInput | 验证SVG/场景/附件/视频安全交付的输入 |
| ValidateAssetPayloadResult | 验证SVG/场景/附件/视频安全交付的结果 |
| CommitAssetBlobInput | 将已验证文件提交不可变内容寻址存储的输入 |
| CommitAssetBlobResult | 将已验证文件提交不可变内容寻址存储的结果 |
| ReadAssetInput | 读取资源/variants安全元数据的输入 |
| ReadAssetResult | 读取资源/variants安全元数据的结果 |
| ListAssetsInput | 资源库列图片/附件/视频/画板及引用摘要的输入 |
| ListAssetsResult | 资源库列图片/附件/视频/画板及引用摘要的结果 |
| AttachImageVariantsInput | 关联浏览器多宽WebP原图的输入 |
| AttachImageVariantsResult | 关联浏览器多宽WebP原图的结果 |
| ValidateDrawingSceneInput | 校验Excalidraw场景和提取文字的输入 |
| ValidateDrawingSceneResult | 校验Excalidraw场景和提取文字的结果 |
| AcquireResourceReferencesInput | 正文/config提交前保护资源迟到引用的输入 |
| AcquireResourceReferencesResult | 正文/config提交前保护资源迟到引用的结果 |
| ConfirmResourceReferencesInput | 源提交后确认pin为引用的输入 |
| ConfirmResourceReferencesResult | 源提交后确认pin为引用的结果 |
| ReleaseResourceReferencesInput | 仅明确源未提交才释放pin的输入 |
| ReleaseResourceReferencesResult | 仅明确源未提交才释放pin的结果 |
| ApplyReferenceEventInput | 消费引用事件防重复/乱序的输入 |
| ApplyReferenceEventResult | 消费引用事件防重复/乱序的结果 |
| ReadAssetReferencesInput | 列哪些文档当前/历史引用资源的输入 |
| ReadAssetReferencesResult | 列哪些文档当前/历史引用资源的结果 |
| ListUnusedAssetsInput | 后台单列当前未引用文件的输入 |
| ListUnusedAssetsResult | 后台单列当前未引用文件的结果 |
| ProposeManualCleanupInput | 人工清理前生成精确风险计划的输入 |
| ProposeManualCleanupResult | 人工清理前生成精确风险计划的结果 |
| ConfirmManualCleanupInput | 管理员确认后进入迟到引用保护窗口的输入 |
| ConfirmManualCleanupResult | 管理员确认后进入迟到引用保护窗口的结果 |
| CancelManualCleanupInput | 删除前取消人工清理的输入 |
| CancelManualCleanupResult | 删除前取消人工清理的结果 |
| ValidateCleanupSourcesInput | 确认所有权威引用源后才认定可清理的输入 |
| ValidateCleanupSourcesResult | 确认所有权威引用源后才认定可清理的结果 |
| ExecuteManualCleanupInput | 只执行已人工确认的安全清理计划的输入 |
| ExecuteManualCleanupResult | 只执行已人工确认的安全清理计划的结果 |
| ParseByteRangeInput | 规范Range支持iOS视频的输入 |
| ParseByteRangeResult | 规范Range支持iOS视频的结果 |
| ReadAssetBytesInput | 鉴权读取immutable bytes和Range的输入 |
| ReadAssetBytesResult | 鉴权读取immutable bytes和Range的结果 |
| HeadAssetBytesInput | HEAD同GET授权只返header的输入 |
| HeadAssetBytesResult | HEAD同GET授权只返header的结果 |
| AcquireReaderLeaseInput | 防正在下载资源被最终删除的输入 |
| AcquireReaderLeaseResult | 防正在下载资源被最终删除的结果 |
| BuildDeliveryManifestInput | 给公开/管理员预览返回SHA URLs/srcset/封面的输入 |
| BuildDeliveryManifestResult | 给公开/管理员预览返回SHA URLs/srcset/封面的结果 |
| ReadVideoMetadataInput | 插入时一次获取平台公开信息并本地封面的输入 |
| ReadVideoMetadataResult | 插入时一次获取平台公开信息并本地封面的结果 |
| RecognizePlatformURLInput | 识别Bilibili/YouTube/抖音/TikTok链接的输入 |
| RecognizePlatformURLResult | 识别Bilibili/YouTube/抖音/TikTok链接的结果 |
| ResolveProviderRedirectInput | 仅平台短链接受控重定向解析的输入 |
| ResolveProviderRedirectResult | 仅平台短链接受控重定向解析的结果 |
| FetchProviderPublicMetadataInput | provider接口抓公开非登录信息的输入 |
| FetchProviderPublicMetadataResult | provider接口抓公开非登录信息的结果 |
| BuildProviderEmbedInput | 生成四平台白名单embed资料的输入 |
| BuildProviderEmbedResult | 生成四平台白名单embed资料的结果 |
| ImportRemoteCoverInput | 下载平台封面到本地内容寻址资源的输入 |
| ImportRemoteCoverResult | 下载平台封面到本地内容寻址资源的结果 |
| RecoverBlobJournalInput | 恢复blob完成/删除持久日志的输入 |
| RecoverBlobJournalResult | 恢复blob完成/删除持久日志的结果 |
| PublishAssetEventInput | 原子写最小资源状态outbox的输入 |
| PublishAssetEventResult | 原子写最小资源状态outbox的结果 |
| ReleaseBarrierInput | 释放已协调完成的屏障的输入 |
| ReleaseBarrierResult | 释放已协调完成的屏障的结果 |
| releaseBarrierRequest | 释放屏障请求体 |
| putUploadChunkRequest | 顺序原子写有界上传块请求体 |
| completeUploadRequest | 完成hash/MIME验证后发布ready资源请求体 |
| abortUploadRequest | 取消未完成上传释放配额请求体 |
| attachImageVariantsRequest | 关联浏览器多宽WebP原图请求体 |
| confirmManualCleanupRequest | 管理员确认后进入迟到引用保护窗口请求体 |
| cancelManualCleanupRequest | 删除前取消人工清理请求体 |

恢复重建必须验证 manifest 哈希和私有事务水位。derived cache 可重建；权威记录缺失或 schema 未知应隔离并停止对外发布。


## 第四轮首页跨域只读协议

acquire顶层显式site/object/revision/version/operation/digest且refs允许空；统一CanonicalJSON摘要，重放不续期；COMMITTED旧版本只补history，当前水位不倒退；源未知/404/pending/到期均不能release。三个引用生产方函数仍planned，无实际业务实现或数据迁移。


第四轮恢复分支补充：Release以完整来源材料和可空pinId调用。媒体只在回源明确同材料aborted/rejected后，按源互斥及对象稳定锁序，同事务封存来源与释放pending pin；本地无pin也持久封存，晚到Acquire不得再落pin。网络404/未知不是无pin证明，源已提交/current/history均不释放。此协议仍planned，不宣称媒体业务实现。


## 第五轮上传分块与原回执

0002_upload_chunks.sql只追加upload_sessions.data_epoch、upload_chunks和Begin回执result_json/result_hash。旧session的NULL epoch不推定为当前代次，旧Begin回执缺原快照不从mutable session重建。新Begin原始输入连同caller/actor/site/epoch规范摘要，当前授权/epoch后、配额和名称策略前回放首次安全UploadSession；新session/创建epoch/原结果/hash同事务。

每个chunk的actor/site/epoch/upload、offset/length/SHA/expectedRevision、服务生成stageId、阶段及原结果独立持久。临时chunk精确长度和SHA核对、文件/目录fsync后才PREPARED；追加上传文件fsync后，同SQL事务CAS offset/revision并写COMMITTED原UploadSession和hash。没有回执的尾字节不是真实已接收水位；启动核连续COMMITTED前缀与SHA，尾只按SQL水位截去，短文件或坏回执失败关闭。PREPARED和未知临时材料不自动发布或计数，ready文件不自动清理。

分块body由HTTP解析后的明确Content-Length和io.Reader绑定，单块1..8MiB，内部没有跨进程句柄表。所有上传路径仅受信根目录与服务生成ID构造，Lstat拒目录链符号链接/特殊对象，目录能力与NOFOLLOW保护普通文件打开；持久JSON读取16MiB有界，schema来自唯一生成规格。数据库FULL同步。

ReadAsset只读真实ready元数据并核索引与唯一Asset schema，明确版本只匹配当前真实ref.version，不存在历史NOT_FOUND。DTO不含OS路径、文件句柄或发布授权。注册纯服务精确名单固定；用户当前授权且同站点。读op与已打开active epoch不符拒绝；调用服务仍核结果siteId与自己的来源。

创建上传在SQL Commit尝试前即保留候选临时文件；Commit错误仍报错但不删除可能已落库会话对应文件，由同op读取持久原回执确认。未知文件不自动计数成功，也不在本包扩GC。

本轮私有文件安全收口复用公共lifecycle.ValidatePrivatePath：既有上传/暂存普通文件打开前后均拒硬链接，并核Root、fd和绝对路径SameFile；media.db打开前后同一公共校验，未知失败关闭，不复制规则。

Begin配额读取在当前用户media.upload/原站点与回执边界之后，按bounded op保留deadline/epoch/root/costClass，派生固定media纯服务Actor并清原PortalProof，不携用户session/grants，不额外要求编辑者site.read。site-config返回siteId必须仍等于原用户站点。
