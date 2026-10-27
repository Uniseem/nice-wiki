# knowledge 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 103 个：手写 91、UI 1、生成 4、配置 6、迁移 1。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/knowledge/internal/runtime/bootstrap.go` | 手写 | 启动索引服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/config/config.go` | 手写 | 更新索引配置所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/events/publication.go` | 手写 | 接受权威发布事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/build/create.go` | 手写 | 人工或事件申请构建所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/source/content.go` | 手写 | 取得权威正文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/source/media.go` | 手写 | 筛选发布引用媒体所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/extract/create.go` | 手写 | 按hash幂等提取资源所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/extract/pdf.go` | 手写 | 提取PDF可用文字所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/extract/image.go` | 手写 | 显式生成图片检索描述所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/extract/video.go` | 手写 | 提取视频题名及授权字幕所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/chunk/normalize.go` | 手写 | 稳定规范化输入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/chunk/split.go` | 手写 | 按源块及token边界分块所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/index/staging.go` | 手写 | 保存独立分块代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/index/lexical.go` | 手写 | 建立词项索引所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/index/embedding.go` | 手写 | 请求分块嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/build/validate.go` | 手写 | 验证完整索引代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/build/activate.go` | 手写 | 原子切换当前索引所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/build/cancel.go` | 手写 | 取消入库链路所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/index/retire.go` | 手写 | 响应撤回/删除所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/query.go` | 手写 | 规范检索请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/lexical.go` | 手写 | 执行全文召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/vector.go` | 手写 | 执行向量召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/fusion.go` | 手写 | 确定性融合两种召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/visibility.go` | 手写 | 权威过滤检索材料所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/rerank.go` | 手写 | 可选AI重排所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/context.go` | 手写 | 选择答案材料所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/service.go` | 手写 | 统一混合检索所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/admin/read.go` | 手写 | 读取入库状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/events/inbox.go` | 手写 | 事件去重与代际保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/events/outbox.go` | 手写 | 发布索引生命周期事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/runtime/recovery.go` | 手写 | 恢复入库执行租约所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/runtime/gc.go` | 手写 | 回收失活索引代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/admin/diagnostics.go` | 手写 | 返回安全诊断所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/sections.go` | 手写 | 按引用读取证据片段所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/chunk/context.go` | 手写 | 发布后补检索片段上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/inspect.go` | 手写 | 快速检索员有限核查所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/search/query-embedding.go` | 手写 | 统一生成并复用查询嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/extract/attachment.go` | 手写 | 发布附件描述与非文本提取所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/runtime/worker.go` | 手写 | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/deploy/service.config.json` | 配置 | 实际非敏感部署默认配置：knowledge监听8106、独占dataRoot、严格ServiceConfig与外部secret refs；CLI>env>file>defaults由平台统一加载，不含凭据或业务运行数据 |
| `modules/knowledge/migrations/001_initial.sql` | 迁移 | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 |
| `modules/knowledge/cmd/knowledge/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/knowledge/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/knowledge/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/knowledge/go.mod` | 配置 | 独立module nice-wiki/modules/knowledge；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/knowledge/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/knowledge/Dockerfile` | 配置 | Node22真实独立UI frozen锁类型检查和构建、Go1.25 readonly多阶段镜像；只带二进制/非敏感配置/UI dist，以10008非root与本域独占卷运行；完整app仍按代码缺口失败关闭 |
| `modules/knowledge/Makefile` | 配置 | 本域GitLab编译/readonly vet/gofmt与Node22既有UI类型检查/构建入口；generate委托根唯一gen；不新增测试或样例 |
| `modules/knowledge/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/knowledge/internal/model/client.go` | 手写 | 仅公共模型适配与请求独立许可；独立包避免索引和运行编排循环 |
| `modules/knowledge/internal/build/execute.go` | 手写 | 执行一次文档索引构建 |
| `modules/knowledge/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/knowledge/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 |
| `modules/knowledge/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/knowledge/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 |
| `modules/knowledge/internal/build/rebuild.go` | 手写 | 全量重建前估算费用 |
| `modules/knowledge/internal/index/repository.go` | 手写 | 按本域active私有路径打开SQLite，执行内嵌只追加迁移和平台inbox/outbox建表，旧身份不重标 |
| `modules/knowledge/migrations/002_workflows.sql` | 手写 | 本域配置、原回执与构建调度/模型步骤持久状态，只追加迁移 |
| `modules/knowledge/migrations/migrations.go` | 手写 | 内嵌唯一迁移来源，不读取仓库路径 |
| `modules/knowledge/migrations/003_sources.sql` | 手写 | 原jobs操作ID与固定发布正文私有快照，只追加迁移 |
| `modules/knowledge/internal/index/inbox.go` | 手写 | 发布与撤回共用同事务原信封inbox门禁，避免域内包循环 |
| `modules/knowledge/internal/index/cancellation.go` | 手写 | 已持久取消决定的锁外真实副作用，不实现模型或队列机制 |
| `modules/knowledge/internal/admin/options.go` | 手写 | 现有管理页面真实选项与独立不可用状态 |
| `modules/knowledge/internal/index/catalog.go` | 手写 | 权威无密钥模型目录与精确费用类别/空间选择 |
| `modules/knowledge/internal/model/tokenize.go` | 手写 | 免费模型真实BPE与完整固定空间/字节来源核对 |
| `modules/knowledge/internal/source/blocks.go` | 手写 | 真实原文片段/规范字节映射/BPE游标 |
| `modules/knowledge/internal/model/journal.go` | 手写 | 本域付费步骤意图/真实引用/完整原结果持久，不复制网关机制 |
| `modules/knowledge/migrations/004_model_result_hash.sql` | 手写 | 付费步骤完整结果摘要，只追加迁移 |
| `modules/knowledge/migrations/005_chunk_context.sql` | 手写 | 同源上下文独立付费意图与不可变机器背景缓存，只追加 |
| `modules/knowledge/internal/extract/source.go` | 手写 | 精确原版本和发布引用的受控授权字节流 |
| `modules/knowledge/internal/extract/pdf_worker.go` | 手写 | 固定已批准纯Go PDF库的隔离内存解析算法 |
| `modules/knowledge/internal/search/source-views.go` | 手写 | 证据sourcePath原文视图回读核验，不取latest |
| `modules/knowledge/internal/source/extraction-view.go` | 手写 | 精确资源机器原文视图回读与来源隔离 |
| `modules/knowledge/migrations/006_extraction_source_views.sql` | 手写 | 资源派生证据不可变原文视图和完整原发布/块授权关联 |
| `modules/knowledge/internal/extract/describe.go` | 手写 | 真实视觉/附件任务消息与完整机器派生文字读取 |
| `modules/knowledge/migrations/007_stable_model_semantics.sql` | 手写 | 稳定模型语义唯一键与完整原上下文计划冻结，只追加迁移 |
| `modules/knowledge/internal/model/provenance.go` | 手写 | MG实际winning来源证明与完整输出/费用状态分离 |
| `modules/knowledge/migrations/008_actual_context_provenance.sql` | 手写 | 背景缓存持久实际winning模型opaque证明，旧缺字段结果保留不猜版本 |
| `modules/knowledge/internal/index/publication.go` | 手写 | 本域持久原任务执行及恢复闭包 |
| `modules/knowledge/internal/index/execution.go` | 手写 | 只由真实jobs job/fence编码私有镜像标识，不产生租约权限 |
| `modules/knowledge/internal/index/memory.go` | 手写 | 严格生产schema读取真实任务/语言完整记忆，不自行构造空版本 |
| `modules/knowledge/internal/index/extraction_cancel.go` | 手写 | 原构建取消同事务只取消已无其他有效原批准构建使用的子任务 |
| `modules/knowledge/internal/runtime/job_recovery.go` | 手写 | 只jobs服务核原完整来源及失效租约后给completed/safeToRetry/pending/unknown真实证明 |
| `modules/knowledge/internal/build/plan.go` | 手写 | 读原构建首次完整阶段计划，核原hash与16MiB运行预算 |
| `modules/knowledge/internal/build/embedding_plan.go` | 手写 | 固定真实模型声明和单块批次，完整检索文本真实BPE预算，不因缓存变化改组 |
| `modules/knowledge/internal/extract/execute.go` | 手写 | 本域持久原任务执行及恢复闭包 |
| `modules/knowledge/internal/extract/dispatch.go` | 手写 | 先持久dispatching，再原input/root/op/epoch/dedupe派发独立jobs，未知不换key |
| `modules/knowledge/internal/extract/views.go` | 手写 | 真实提取页/描述按精确发布/块/asset关联不可变机器原文视图，页路径独立偏移零起算 |
| `modules/knowledge/internal/source/material.go` | 手写 | 完整机器文字NFKC真实稳定边界续读，4MiB/1Mi字符BPE片段与原页路径偏移不改 |
| `modules/knowledge/migrations/009_execution_ownership.sql` | 手写 | 真实原任务拥有者/独立提取/阶段计划/最终结果与租约镜像只追加迁移 |
| `modules/knowledge/migrations/010_model_dispatch_proof.sql` | 手写 | 原模型步骤prepared与真实外发前持久标记；旧记录默认已尝试外发保守unknown，不按空ID猜零费用 |
| `modules/knowledge/internal/model/recovery.go` | 手写 | 后台原已接受ID/完整去凭据DTO只读找回、专用时点负证明，不发模型 |
| `modules/knowledge/internal/model/fee-proof.go` | 手写 | 恢复原root完整真实费用终局证明，不把完整输出当结算或猜零金额 |
| `modules/knowledge/migrations/011_background_model_requests.sql` | 手写 | 只背景原DTO、独立最新真实观察与专用未接受时点证明，QA正文和许可不持久 |
| `modules/knowledge/internal/extract/cache.go` | 手写 | 稳定资源语义拥有者、免费恢复原paid输出与新明批失败意图安全替换 |
| `modules/knowledge/internal/build/rebuild_cache.go` | 手写 | 估价只读真实原资源/背景/向量缓存，不创建意图或任务 |
| `modules/knowledge/migrations/012_recovered_extraction_results.sql` | 手写 | 原paid完整输出免费恢复独立证据，保留原失败任务和费用 |
| `modules/knowledge/migrations/013_rebuild_plans.sql` | 手写 | 真实全量估价快照与24小时批准、创建任务回执同事务 |
| `modules/knowledge/internal/index/route.go` | 手写 | 严格MG resolveTaskRoute同快照resolved/candidates/routeVersion和候选modelSnapshotHash/embeddingSpace，不复制筛选/覆盖/快照hash算法 |
| `modules/knowledge/internal/model/expectation.go` | 手写 | 只绑定原明确批准opaque路由和逐真实候选价格快照，接受事务CAS由MG统一拥有 |
| `modules/knowledge/web/src/index.tsx` | UI | 知识索引：入库配置、为已发布文档建立索引、索引任务进度与取消 |
| `modules/knowledge/internal/card/view.go` | 手写 | 文档卡片与按预算压缩的文档索引视图；Go包，禁止直接访问其他域私有存储。 |
| `modules/knowledge/internal/overview/overview.go` | 手写 | 层级摘要与全站概括：每篇文档的摘要包含其下级文档的概述，顶层摘要合成全站概括 |
