# qa 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=62；分类={"handwritten": 46, "ui": 3, "generated": 3, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/qa/internal/runtime/bootstrap.go` | handwritten | 启动问答服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.LoadRuntime` |
| `modules/qa/internal/config/policy.go` | handwritten | 管理公开问答策略所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PutQaPolicy` |
| `modules/qa/internal/conversation/create.go` | handwritten | 创建伪名访客会话所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CreateConversation` |
| `modules/qa/internal/conversation/read.go` | handwritten | 读取自有访客历史所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.GetConversation` |
| `modules/qa/internal/turn/start.go` | handwritten | 准入后开始问答轮次所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.StartTurn` |
| `modules/qa/internal/admission/permit.go` | handwritten | 检查准入绑定所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ValidateAdmission` |
| `modules/qa/internal/turn/normalize.go` | handwritten | 稳定规范问题和cache锚点所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.NormalizeQuestion` |
| `modules/qa/internal/turn/context.go` | handwritten | 固定回答上下文版本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ResolveTurnContext` |
| `modules/qa/internal/cache/exact.go` | handwritten | 查询精确精选缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.LookupExactCache` |
| `modules/qa/internal/cache/semantic.go` | handwritten | 保守查询语义缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.LookupSemanticCache` |
| `modules/qa/internal/cache/validate.go` | handwritten | 验证缓存仍可公开交付所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ValidateCachedAnswer` |
| `modules/qa/internal/answer/retrieve.go` | handwritten | 取有源可见证据所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RetrieveEvidence` |
| `modules/qa/internal/answer/prompt.go` | handwritten | 构造有界回答提示所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.BuildAnswerPrompt` |
| `modules/qa/internal/answer/generate.go` | handwritten | 执行流式答案调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.GenerateAnswer` |
| `modules/qa/internal/answer/citation.go` | handwritten | 逐条保护引用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ValidateCitation` |
| `modules/qa/internal/stream/store.go` | handwritten | 持久流式输出事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.AppendAnswerEvent` |
| `modules/qa/internal/stream/read.go` | handwritten | SSE断线重连所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ResumeAnswerStream` |
| `modules/qa/internal/turn/complete.go` | handwritten | 原子结束可见答案所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CompleteTurn` |
| `modules/qa/internal/turn/cancel.go` | handwritten | 取消问答及所有子调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CancelTurn` |
| `modules/qa/internal/feedback/feedback.go` | handwritten | 保存访客单条反馈所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RecordFeedback` |
| `modules/qa/internal/cache/curation.go` | handwritten | 人工创建精选候选所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ProposeCacheEntry`, `qa.ApproveCacheEntry`, `qa.DisableCacheEntry` |
| `modules/qa/internal/cache/embedding.go` | handwritten | 为已批准缓存构建语义向量所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.BuildCacheEmbedding` |
| `modules/qa/internal/cache/invalidate.go` | handwritten | 失效受版本变化影响缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.InvalidateCache` |
| `modules/qa/internal/admin/cache.go` | handwritten | 管理精选缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ListCacheEntries` |
| `modules/qa/internal/cache/translation.go` | handwritten | 请求精选答案翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.TranslateCacheEntry` |
| `modules/qa/internal/history/compaction.go` | handwritten | 申请有界历史摘要所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CreateCompaction`, `qa.RunCompaction` |
| `modules/qa/internal/history/commit.go` | handwritten | 保护新历史不被旧摘要覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CommitCompaction` |
| `modules/qa/internal/debug/create.go` | handwritten | 隔离管理诊断所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CreateDebugTurn` |
| `modules/qa/internal/debug/read.go` | handwritten | 读取管理诊断所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ReadDebugTurn` |
| `modules/qa/internal/conversation/delete.go` | handwritten | 删除自有对话数据所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.DeleteConversation` |
| `modules/qa/internal/runtime/recovery.go` | handwritten | 恢复崩溃问答所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RecoverTurns` |
| `modules/qa/internal/events/outbox.go` | handwritten | 发布问答脱敏事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PublishOutbox` |
| `modules/qa/internal/runtime/prune.go` | handwritten | 清理到期记录所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PruneExpiredRecords` |
| `modules/qa/internal/protection/protocol.go` | handwritten | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.AcquireBarrier`, `qa.ReleaseBarrier`, `qa.ExportSnapshot`, `qa.ValidateSnapshot`, `qa.PrepareSnapshot`, `qa.CommitSnapshot`, `qa.RollbackSnapshot` |
| `modules/qa/ui/src/views/OverviewView.vue` | ui | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `qa.LoadAdminOverview` |
| `modules/qa/ui/src/views/EditView.vue` | ui | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `qa.SubmitPrimaryEdit` |
| `modules/qa/internal/context/page.go` | handwritten | 验证当前页面上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ResolveCurrentPageHint` |
| `modules/qa/internal/context/preflight.go` | handwritten | 补全问题、分类、风险与缓存资格所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PlanQuestion` |
| `modules/qa/internal/cache/eligibility.go` | handwritten | 严格缓存完整问题门所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.EvaluateCacheEligibility` |
| `modules/qa/internal/cache/query-embedding.go` | handwritten | 同查询只生成一次嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.AcquireQueryEmbedding` |
| `modules/qa/internal/answer/inspect.go` | handwritten | 快速检索员有限核查所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.InspectEvidence` |
| `modules/qa/internal/cache/automatic.go` | handwritten | 按old语义自动保存合格答案所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.WriteAutomaticCache` |
| `modules/qa/internal/cache/localized.go` | handwritten | 保存缓存的多语言回答所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.StoreLocalizedCacheAnswer` |
| `modules/qa/internal/admin/calibration.go` | handwritten | 缓存阈值校准所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ReadCacheCalibration` |
| `modules/qa/internal/runtime/worker.go` | handwritten | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RunWorker` |
| `modules/qa/contracts/generated/schema.gen.go` | generated | 未来由根contract compiler从本域types/openapi生成DTO，Generator(inputSchemas []SchemaRef, outputDirectory string) error；生成前schema refs必须解析，禁止手写编辑。 |  |
| `modules/qa/deploy/service.config.json` | config | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/qa/migrations/001_initial.sql` | migration | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 |  |
| `modules/qa/cmd/qa/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M09-F901` |
| `modules/qa/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M09-F902` |
| `modules/qa/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M09-F903`, `M09-F904`, `M09-F905`, `M09-F906` |
| `modules/qa/go.mod` | config | 独立module nice-wiki/modules/qa；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/qa/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/qa/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/qa；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/qa/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/qa与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/qa/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/qa/ui/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/qa/ui/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/qa/ui/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/qa/ui/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/qa/ui/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/qa/ui/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M09-F907` |
