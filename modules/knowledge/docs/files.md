# knowledge 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=59；分类={"handwritten": 43, "ui": 3, "generated": 3, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/knowledge/internal/runtime/bootstrap.go` | handwritten | 启动索引服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.LoadRuntime` |
| `modules/knowledge/internal/config/config.go` | handwritten | 更新索引配置所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.PutIndexConfig` |
| `modules/knowledge/internal/events/publication.go` | handwritten | 接受权威发布事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.HandlePublication` |
| `modules/knowledge/internal/build/create.go` | handwritten | 人工或事件申请构建所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CreateBuild` |
| `modules/knowledge/internal/source/content.go` | handwritten | 取得权威正文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.FetchPublishedSource` |
| `modules/knowledge/internal/source/media.go` | handwritten | 筛选发布引用媒体所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ListExtractableAssets` |
| `modules/knowledge/internal/extract/create.go` | handwritten | 按hash幂等提取资源所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CreateExtraction` |
| `modules/knowledge/internal/extract/pdf.go` | handwritten | 提取PDF可用文字所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ExtractPdfText` |
| `modules/knowledge/internal/extract/image.go` | handwritten | 显式生成图片检索描述所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ExtractImageDescription` |
| `modules/knowledge/internal/extract/video.go` | handwritten | 提取视频题名及授权字幕所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ExtractVideoMetadata` |
| `modules/knowledge/internal/chunk/normalize.go` | handwritten | 稳定规范化输入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.NormalizeSource` |
| `modules/knowledge/internal/chunk/split.go` | handwritten | 按源块及token边界分块所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SplitChunks` |
| `modules/knowledge/internal/index/staging.go` | handwritten | 保存独立分块代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.WriteStagingChunks` |
| `modules/knowledge/internal/index/lexical.go` | handwritten | 建立词项索引所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.BuildLexicalIndex` |
| `modules/knowledge/internal/index/embedding.go` | handwritten | 请求分块嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RequestEmbeddings`, `knowledge.StoreEmbeddings` |
| `modules/knowledge/internal/build/validate.go` | handwritten | 验证完整索引代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ValidateBuild` |
| `modules/knowledge/internal/build/activate.go` | handwritten | 原子切换当前索引所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ActivateBuild` |
| `modules/knowledge/internal/build/cancel.go` | handwritten | 取消入库链路所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CancelBuild` |
| `modules/knowledge/internal/index/retire.go` | handwritten | 响应撤回/删除所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RetireDocumentIndex` |
| `modules/knowledge/internal/search/query.go` | handwritten | 规范检索请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.NormalizeQuery`, `knowledge.EmbedQuery` |
| `modules/knowledge/internal/search/lexical.go` | handwritten | 执行全文召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SearchLexical` |
| `modules/knowledge/internal/search/vector.go` | handwritten | 执行向量召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SearchVector` |
| `modules/knowledge/internal/search/fusion.go` | handwritten | 确定性融合两种召回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.FuseCandidates` |
| `modules/knowledge/internal/search/visibility.go` | handwritten | 权威过滤检索材料所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ValidateCandidates` |
| `modules/knowledge/internal/search/rerank.go` | handwritten | 可选AI重排所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RerankCandidates` |
| `modules/knowledge/internal/search/context.go` | handwritten | 选择答案材料所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.SelectContext` |
| `modules/knowledge/internal/search/service.go` | handwritten | 统一混合检索所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.Retrieve` |
| `modules/knowledge/internal/admin/read.go` | handwritten | 读取入库状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.GetBuild`, `knowledge.ListBuilds` |
| `modules/knowledge/internal/events/inbox.go` | handwritten | 事件去重与代际保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ConsumeInbox` |
| `modules/knowledge/internal/events/outbox.go` | handwritten | 发布索引生命周期事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.PublishOutbox` |
| `modules/knowledge/internal/runtime/recovery.go` | handwritten | 恢复入库执行租约所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RecoverBuildLeases` |
| `modules/knowledge/internal/runtime/gc.go` | handwritten | 回收失活索引代数所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CollectRetiredIndexes` |
| `modules/knowledge/internal/admin/diagnostics.go` | handwritten | 返回安全诊断所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RedactDiagnostics` |
| `modules/knowledge/internal/search/sections.go` | handwritten | 按引用读取证据片段所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.ReadSections` |
| `modules/knowledge/internal/protection/protocol.go` | handwritten | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.AcquireBarrier`, `knowledge.ReleaseBarrier`, `knowledge.ExportSnapshot`, `knowledge.ValidateSnapshot`, `knowledge.PrepareSnapshot`, `knowledge.CommitSnapshot`, `knowledge.RollbackSnapshot` |
| `modules/knowledge/ui/src/views/OverviewView.vue` | ui | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `knowledge.LoadAdminOverview` |
| `modules/knowledge/ui/src/views/EditView.vue` | ui | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `knowledge.SubmitPrimaryEdit` |
| `modules/knowledge/internal/chunk/context.go` | handwritten | 发布后补检索片段上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.EnrichChunkContext` |
| `modules/knowledge/internal/search/inspect.go` | handwritten | 快速检索员有限核查所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.InspectEvidence` |
| `modules/knowledge/internal/search/query-embedding.go` | handwritten | 统一生成并复用查询嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.CreateQueryEmbedding` |
| `modules/knowledge/internal/extract/attachment.go` | handwritten | 发布附件描述与非文本提取所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.DescribeAttachment` |
| `modules/knowledge/internal/runtime/worker.go` | handwritten | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `knowledge.RunWorker` |
| `modules/knowledge/contracts/generated/schema.gen.go` | generated | 未来由根contract compiler从本域types/openapi生成DTO，Generator(inputSchemas []SchemaRef, outputDirectory string) error；生成前schema refs必须解析，禁止手写编辑。 |  |
| `modules/knowledge/deploy/service.config.json` | config | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/knowledge/migrations/001_initial.sql` | migration | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 |  |
| `modules/knowledge/cmd/knowledge/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M08-F901` |
| `modules/knowledge/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M08-F902` |
| `modules/knowledge/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M08-F903`, `M08-F904`, `M08-F905`, `M08-F906` |
| `modules/knowledge/go.mod` | config | 独立module nice-wiki/modules/knowledge；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/knowledge/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/knowledge/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/knowledge；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/knowledge/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/knowledge与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/knowledge/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/knowledge/ui/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/knowledge/ui/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/knowledge/ui/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/knowledge/ui/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/knowledge/ui/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/knowledge/ui/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M08-F907` |
