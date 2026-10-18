# knowledge 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 65 个：手写 46、UI 3、生成 5、配置 10、迁移 1。函数 96 个：约束 32，参考 64。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/knowledge/internal/runtime/bootstrap.go` | 手写 | 启动索引服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.LoadRuntime`（参考） |
| `modules/knowledge/internal/config/config.go` | 手写 | 更新索引配置所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.PutIndexConfig`、`knowledge.ReadIndexConfig` |
| `modules/knowledge/internal/events/publication.go` | 手写 | 接受权威发布事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.HandlePublication` |
| `modules/knowledge/internal/build/create.go` | 手写 | 人工或事件申请构建所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CreateBuild` |
| `modules/knowledge/internal/source/content.go` | 手写 | 取得权威正文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.FetchPublishedSource`（参考） |
| `modules/knowledge/internal/source/media.go` | 手写 | 筛选发布引用媒体所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ListExtractableAssets`（参考） |
| `modules/knowledge/internal/extract/create.go` | 手写 | 按hash幂等提取资源所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CreateExtraction`（参考）、`knowledge.ExecuteExtraction` |
| `modules/knowledge/internal/extract/pdf.go` | 手写 | 提取PDF可用文字所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ExtractPdfText`（参考） |
| `modules/knowledge/internal/extract/image.go` | 手写 | 显式生成图片检索描述所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ExtractImageDescription`（参考） |
| `modules/knowledge/internal/extract/video.go` | 手写 | 提取视频题名及授权字幕所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ExtractVideoMetadata`（参考） |
| `modules/knowledge/internal/chunk/normalize.go` | 手写 | 稳定规范化输入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.NormalizeSource`（参考） |
| `modules/knowledge/internal/chunk/split.go` | 手写 | 按源块及token边界分块所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SplitChunks`（参考） |
| `modules/knowledge/internal/index/staging.go` | 手写 | 保存独立分块代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.WriteStagingChunks`（参考）、`knowledge.NewIndexRuntime`（参考）、`knowledge.WithIndexRuntime`（参考）、`knowledge.RuntimeFromContext`（参考）、`knowledge.ReadBuildRecord`（参考）、`knowledge.BeginBuildWrite`（参考）、`knowledge.storageError`（参考）、`knowledge.ReadChunks`（参考）、`knowledge.ReadGeneration`（参考） |
| `modules/knowledge/internal/index/lexical.go` | 手写 | 建立词项索引所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.BuildLexicalIndex`（参考）、`knowledge.TokenizeText`（参考） |
| `modules/knowledge/internal/index/embedding.go` | 手写 | 请求分块嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RequestEmbeddings`（参考）、`knowledge.StoreEmbeddings`（参考）、`knowledge.DecodeVector`（参考） |
| `modules/knowledge/internal/build/validate.go` | 手写 | 验证完整索引代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ValidateBuild`（参考）、`knowledge.validatePublication`（参考） |
| `modules/knowledge/internal/build/activate.go` | 手写 | 原子切换当前索引所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ActivateBuild`（参考） |
| `modules/knowledge/internal/build/cancel.go` | 手写 | 取消入库链路所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CancelBuild` |
| `modules/knowledge/internal/index/retire.go` | 手写 | 响应撤回/删除所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RetireDocumentIndex` |
| `modules/knowledge/internal/search/query.go` | 手写 | 规范检索请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.NormalizeQuery`（参考）、`knowledge.EmbedQuery`（参考） |
| `modules/knowledge/internal/search/lexical.go` | 手写 | 执行全文召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SearchLexical`（参考） |
| `modules/knowledge/internal/search/vector.go` | 手写 | 执行向量召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SearchVector`（参考）、`knowledge.validVector`（参考）、`knowledge.unitVector`（参考） |
| `modules/knowledge/internal/search/fusion.go` | 手写 | 确定性融合两种召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.FuseCandidates`（参考） |
| `modules/knowledge/internal/search/visibility.go` | 手写 | 权威过滤检索材料所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ValidateCandidates`（参考） |
| `modules/knowledge/internal/search/rerank.go` | 手写 | 可选AI重排所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RerankCandidates`（参考）、`knowledge.requestRerank`（参考） |
| `modules/knowledge/internal/search/context.go` | 手写 | 选择答案材料所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SelectContext`（参考） |
| `modules/knowledge/internal/search/service.go` | 手写 | 统一混合检索所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.Retrieve`、`knowledge.NewSearchRuntime`（参考）、`knowledge.WithSearchRuntime`（参考）、`knowledge.searchRuntime`（参考）、`knowledge.activeSnapshot`（参考）、`knowledge.publicationKey`（参考）、`knowledge.sortedSnapshotRows`（参考）、`knowledge.ReadActiveSnapshot`（参考） |
| `modules/knowledge/internal/admin/read.go` | 手写 | 读取入库状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.GetBuild`、`knowledge.ListBuilds`、`knowledge.WithAdminSigningKey`（参考）、`knowledge.authorizeAdmin`（参考）、`knowledge.beginAdminRead`（参考） |
| `modules/knowledge/internal/events/inbox.go` | 手写 | 事件去重与代际保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ConsumeInbox`（参考）、`knowledge.ConsumeEvents` |
| `modules/knowledge/internal/events/outbox.go` | 手写 | 发布索引生命周期事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.PublishOutbox` |
| `modules/knowledge/internal/runtime/recovery.go` | 手写 | 恢复入库执行租约所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RecoverBuildLeases`（参考） |
| `modules/knowledge/internal/runtime/gc.go` | 手写 | 回收失活索引代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CollectRetiredIndexes`（参考） |
| `modules/knowledge/internal/admin/diagnostics.go` | 手写 | 返回安全诊断所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RedactDiagnostics` |
| `modules/knowledge/internal/search/sections.go` | 手写 | 按引用读取证据片段所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ReadSections` |
| `modules/knowledge/internal/protection/protocol.go` | 手写 | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.AcquireBarrier`、`knowledge.ReleaseBarrier`、`knowledge.ExportSnapshot`、`knowledge.StageSnapshot`、`knowledge.DiscardSnapshot` |
| `modules/knowledge/ui/src/views/OverviewView.vue` | UI | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `knowledge.LoadAdminOverview`（参考） |
| `modules/knowledge/ui/src/views/EditView.vue` | UI | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `knowledge.SubmitPrimaryEdit`（参考） |
| `modules/knowledge/internal/chunk/context.go` | 手写 | 发布后补检索片段上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.EnrichChunkContext`（参考） |
| `modules/knowledge/internal/search/inspect.go` | 手写 | 快速检索员有限核查所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.InspectEvidence` |
| `modules/knowledge/internal/search/query-embedding.go` | 手写 | 统一生成并复用查询嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CreateQueryEmbedding`、`knowledge.acquireQueryFlight`（参考） |
| `modules/knowledge/internal/extract/attachment.go` | 手写 | 发布附件描述与非文本提取所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.DescribeAttachment`（参考） |
| `modules/knowledge/internal/runtime/worker.go` | 手写 | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RunWorker` |
| `modules/knowledge/deploy/service.config.json` | 配置 | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/knowledge/migrations/001_initial.sql` | 迁移 | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 | 无 |
| `modules/knowledge/cmd/knowledge/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `knowledge.main` |
| `modules/knowledge/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `knowledge.BuildApplication` |
| `modules/knowledge/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `knowledge.Serve`、`knowledge.Shutdown`、`knowledge.Health`、`knowledge.Ready`、`knowledge.NewOperationHandlers`（参考）、`knowledge.jsonOperation`（参考）、`knowledge.operationContext`（参考）、`knowledge.writeResult`（参考） |
| `modules/knowledge/go.mod` | 配置 | 独立module nice-wiki/modules/knowledge；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/knowledge/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/knowledge/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/knowledge；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/knowledge/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/knowledge与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/knowledge/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/knowledge/ui/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/knowledge/ui/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/knowledge/ui/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/knowledge/ui/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/knowledge/ui/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/knowledge/ui/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `knowledge.mountModuleUI` |
| `modules/knowledge/internal/runtime/model_client.go` | 手写 | 经模型网关执行一次调用并读取到终态 | `knowledge.InvokeModel`（参考）、`knowledge.NewModelRuntime`（参考）、`knowledge.WithModelRuntime`（参考）、`knowledge.CompletedOutput`（参考）、`knowledge.BindAdmissionPermit`（参考） |
| `modules/knowledge/internal/build/execute.go` | 手写 | 执行一次文档索引构建 | `knowledge.ExecuteBuild` |
| `modules/knowledge/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/knowledge/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/knowledge/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/knowledge/ui/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
| `modules/knowledge/internal/build/rebuild.go` | 手写 | 全量重建前估算费用 | `knowledge.EstimateRebuildPlan`、`knowledge.ApproveRebuildPlan` |
