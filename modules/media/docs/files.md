# media 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 57 个：手写 44、UI 1、生成 3、配置 6、迁移 3。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/media/internal/upload_service.go` | 手写 | 开始内容寻址可续传上传 |
| `modules/media/internal/upload_validator.go` | 手写 | 清理展示文件名而不用于路径 |
| `modules/media/internal/chunk_writer.go` | 手写 | 顺序原子写有界上传块 |
| `modules/media/internal/upload_completion.go` | 手写 | 完成hash/MIME验证后发布ready资源 |
| `modules/media/internal/upload_maintenance.go` | 手写 | 仅回收过期未完成临时字节 |
| `modules/media/internal/mime_validator.go` | 手写 | 从字节识别类型 |
| `modules/media/internal/payload_validator.go` | 手写 | 验证SVG/场景/附件/视频安全交付 |
| `modules/media/internal/blob_repository.go` | 手写 | 将已验证文件提交不可变内容寻址存储；私有SQLite保存上传会话、资源元数据、回执与blob日志，COMMITTED与资源元数据同事务 |
| `modules/media/internal/asset_service.go` | 手写 | 读取资源/variants安全元数据 |
| `modules/media/internal/image_variants.go` | 手写 | 关联浏览器多宽WebP原图 |
| `modules/media/internal/drawing_validator.go` | 手写 | 校验Excalidraw场景和提取文字 |
| `modules/media/internal/reference_service.go` | 手写 | 正文/config提交前保护资源迟到引用 |
| `modules/media/internal/reference_repository.go` | 手写 | 消费引用事件防重复/乱序 |
| `modules/media/internal/reference_reader.go` | 手写 | 两个权威源完整同水位分页与本地pending引用快照 |
| `modules/media/internal/cleanup_service.go` | 手写 | 人工计划提案/取消、原回执及与新增引用共享的完整计划取消事务；列表/确认后续实施 |
| `modules/media/internal/cleanup_sources.go` | 手写 | 确认所有权威引用源后才认定可清理 |
| `modules/media/internal/cleanup_worker.go` | 手写 | 只执行已人工确认的安全清理计划 |
| `modules/media/internal/range_service.go` | 手写 | 规范Range支持iOS视频 |
| `modules/media/internal/asset_delivery.go` | 手写 | 鉴权读取immutable bytes和Range |
| `modules/media/internal/reader_lease.go` | 手写 | 防正在下载资源被最终删除 |
| `modules/media/internal/manifest_service.go` | 手写 | 给公开/管理员预览返回SHA URLs/srcset/封面 |
| `modules/media/internal/video_service.go` | 手写 | 插入时一次获取平台公开信息并本地封面 |
| `modules/media/internal/provider_registry.go` | 手写 | 识别Bilibili/YouTube/抖音/TikTok链接 |
| `modules/media/internal/provider_url_guard.go` | 手写 | 仅平台短链接受控重定向解析 |
| `modules/media/internal/provider_fetch.go` | 手写 | provider接口抓公开非登录信息 |
| `modules/media/internal/provider_embed.go` | 手写 | 生成四平台白名单embed资料 |
| `modules/media/internal/cover_import.go` | 手写 | 下载平台封面到本地内容寻址资源 |
| `modules/media/internal/blob_recovery.go` | 手写 | 恢复blob完成/删除持久日志：读取私有库blob_journal，隔离无COMMITTED的PREPARED产物 |
| `modules/media/internal/outbox_repository.go` | 手写 | 原子写最小资源状态outbox |
| `modules/media/cmd/media/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/media/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/media/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/media/go.mod` | 配置 | 独立module nice-wiki/modules/media；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/media/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/media/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/media；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/media/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/media与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/media/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/media/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/media/internal/worker.go` | 手写 | 主动领取本域持久任务的循环 |
| `modules/media/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/media/internal/gen/clients.gen.go` | 生成 | 生成的版本化客户端及显式runtime schema闭包；不手改，不扩大HTTP DTO |
| `modules/media/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/media/migrations/0001_domain.sql` | 迁移 | 初始权威表（ADR0005私有SQLite）：upload_sessions、upload_receipts、assets、blob_journal与schema_migrations；nw_outbox/nw_inbox由平台EnsureSchema创建不在此登记 |
| `modules/media/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 |
| `modules/media/migrations/0002_upload_chunks.sql` | 手写 | 只追加上传创建epoch和分块PREPARED/COMMITTED原结果日志，不重写0001或猜测旧上传代次 |
| `modules/media/migrations/0003_reference_pins.sql` | 手写 | 只追加引用来源完整绑定/终止封存、pending pin原结果与逐资产fence保护；旧迁移不改 |
| `modules/media/internal/reference_decision_client.go` | 手写 | 只经固定版本化源决定API/共享schema读取，清proof的media纯服务上下文，不持本域锁 |
| `modules/media/migrations/0004_reference_resolution.sql` | 手写 | 只追加源终局原回执、对象current水位与保留history的确认引用行，原pin保护不删除 |
| `modules/media/migrations/0005_manual_cleanup.sql` | 手写 | 只追加人工计划、原扫描、精确pending标记及操作原回执权威表；不自动确认/删除 |
| `modules/media/migrations/0006_cleanup_approvals.sql` | 手写 | 只追加人工计划一次批准、固定延迟任务输入、原任务回执和原waiting结果桥接；取消保留批准证据 |
| `modules/media/migrations/0007_cleanup_execution.sql` | 手写 | 只追加人工执行/逐资产决定及有限reader租约权威，不产生自动清理授权 |
| `modules/media/migrations/0008_asset_listing.sql` | 手写 | 只追加独立int64资产列表水位与稳定排序索引，资产/对象变更在原事务推进；不是引用源水位或删除许可 |
| `modules/media/migrations/0009_upload_expiration.sql` | 迁移 | 仅已过期未完成上传删除意图与续清，不触碰ready资产 |
| `modules/media/migrations/0010_public_assets.sql` | 迁移 | 原图→变体依赖、已公开资源版本登记、含变体的当前引用视图 |
| `modules/media/web/src/index.tsx` | UI | 资源库：全部/未被引用资源、引用详情、清理计划与确认 |
| `modules/media/internal/job_recovery.go` | 手写 | 只jobs服务核原清理批准与任务绑定后给completed/safeToRetry真实证明 |
| `modules/media/internal/groups.go` | 手写 | 本地资源库分组 |
