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

## 本轮完成上传与直接校验（2026-10-16，已有检查通过，行为未验收）

继续使用0001/0002的私有upload_receipts，不新增迁移。Begin原UploadSession快照保持；Complete摘要中kind=complete_upload，封套version=1固定首次Asset或安全错误，result_hash核原字节。来源上传owner/site/创建epoch匹配后才回放；不借当前资源重建原结果，也不因当前revision、配额、到期或维护改变旧成功。不同主体可在同站点复用原ready资源，保留原资源creator；本次授权仍以本次上传owner为准。

UploadSession新增blocked枚举，表示确定的完整SHA/格式/场景校验失败，revision+1与原错误同事务；原有receiving/complete/aborted/expired值兼容，未知状态仍拒绝。尚未收齐、revision冲突、依赖失败与取消不猜作blocked。完整成功按upload owner/site/创建epoch、receiving、原revision与已确认字节SQL CAS；ready/session/COMMITTED/原结果/新资源media.asset.changed outbox同一事务。Commit未知保留临时与不可变文件并报错，同op查回执确认。

MIME忽略浏览器声明；附件固定application/octet-stream，后续字节交付须attachment+nosniff。图片只解析PNG/JPEG/GIF/WebP/AVIF容器/尺寸/动画元数据，不解码像素；MP4/WebM只核允许的编码声明，不解码帧。SVG≤16MiB、画板scene≤64MiB，XML节点≤200000/深度≤128；JPEG头读取≤16MiB、容器节点有界。图片单维1..100000，不支持的容器/编码明确拒绝。SVG保留原字节并带sandbox约束，实际bytes交付函数仍planned，不能将本轮校验视为公开交付已实现。

DrawingScene的files以additionalProperties值schema及propertyNames原精确模式等价表达；OpenAPI 3.0不能表达的键限制由Drawing专属语义按原模式与key/id一致完成，其余字段仍走唯一生成schema。保持场景version=2，未知字段/版本不静默丢弃。文件只允许受限raster base64 data URL，图元引用、frame无环、压力点数量和安全链接另核语义。

完成拒绝的blocked临时文件继续按expected_bytes计入站点存储预留，上传并发数仍只计receiving；尚未明确清理的坏MIME不能反复绕过存储上限。本轮Abort只允许receiving，blocked明确拒绝且继续占用预留，不能沿忽略unlink错误的旧路径释放blocked配额。后续显式临时清理须先建立可恢复的删除/释放决定，不能把unlink失败或未知提交当成空间已释放。

同站点资源复用在SQL选择时即按hash/byteSize/mediaType/kind精确匹配；持久Asset通过唯一schema及所选asset_id/索引字段核对后才进入COMMITTED/原回执，不能在LIMIT1后丢弃不匹配kind的首行造成重复元数据。

SVG的style在XML解析后按完整文本校验：合并所有CharData/CDATA片段、忽略注释边界，每个style结束时检查CSS关键字和URL；拒绝style子元素，保持原字节及既有16MiB/节点/深度上限。

## 两轮并行第一轮：ready 引用预留（2026-10-16，既有检查通过，行为未验收）

只追加0003_reference_pins.sql，旧0001/0002不变。reference_sources按可信sourceService/dataEpoch/sourceOperationId唯一绑定site/object/revision/version/digest、规范完整输入及首次pin结果/hash；pending/confirmed保留原回执，sealed先于旧成功检查。源站点来自明确input.siteId并逐资源核实，纯服务Actor的system站点不能代替来源绑定。references为空也生成pin；重复同材料返回原pin/expiry，不续期、不再次增加generation。

reference_objects为逐资产持久deletionFence/referenceGeneration，迁移一次回填既有资产，新ready资源在完成上传同事务建立0水位。运行时缺fence失败关闭。新pin在同事务逐对象fence和generation各加一，并以reference_pin_assets保存实际fence快照；返回pin.assets按assetId唯一升序，deletionFence只是快照最大值，空集合0，不是全局删除授权。原pin回执核其不可变逐资产保护，不因当前资源变化重建。

锁顺序固定来源caller/epoch/sourceOperationId → assetId唯一升序 → 本域store提交互斥 → SQL事务，释放逆序；不持这些锁回源。新写使用公共running写窗口，旧成功仅核已打开epoch和原材料。现有delete_pending没有真实计划生产方，本轮一律LOCKED；有效人工计划取消分支仍未实现，不能称Acquire完整支持清理闭环。源决定读取不属于Acquire必要步骤，真实local pin不以content/site-config读取未实现为由返回占位成功。

## 两轮并行第二轮：源决定与引用终局（2026-10-16，待统一检查）

0004只追加reference_sources的resolution_json/hash/resolved_at与reference_heads/reference_entries。reference_heads是sourceService/epoch/site/object的单调current水位；reference_entries只记录已确认完整集合的referenceId/assetId及current/history分类，精确不可变元数据与原pending材料永久取自该来源input_json，不能与事件另建影子权威。原pin_assets在confirmed/sealed后都不删除，是否仍为pending保护取reference_sources.state；不能只凭pin行或expiresAt自动释放。

Confirm无锁回源核共享最小决定的完整材料和committed；当前epoch/running窗口重新取得后，sourceOperation→sourceObject→旧current与新pin资产升序→store→SQL。旧head必须是原confirmed来源且当前行逐项/总数完整；较高版本把旧current转history并提升head，较低已提交版本只补history，空集合也有真实head。只对实际变化资产提升generation/fence，重复原confirmed不再改变水位。当前只接受ready；未知delete_pending拒绝，真实清理计划取消未实现，Confirm保持partial。

Release只核完整同材料的aborted/rejected；源决定/运行窗口RPC不持本地锁，随后按相同锁序重读，来源终局与原安全结果/hash同事务。已有pending按状态退出保护并提升generation；从未有pin仍持久sealed，pinId非空只是一致性提示。已confirmed不降级，任何current/history与pending矛盾拒绝；原回执缺失不能从mutable结果重建。未知Commit报错、保留持久材料，同来源重试查原终局；Acquire始终先拒sealed。
