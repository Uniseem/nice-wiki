# qa 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 71 个：手写 52、UI 3、生成 5、配置 10、迁移 1。函数 120 个：约束 40，参考 80。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/qa/internal/runtime/bootstrap.go` | 手写 | 启动问答服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.LoadRuntime`（参考）、`qa.NewRuntime`（参考）、`qa.WithRuntime`（参考）、`qa.FromContext`（参考）、`qa.DecodeResponse`（参考）、`qa.StorageError`（参考）、`qa.RequireAdmin`（参考）、`qa.Transaction.Close`（参考）、`qa.Transaction.Commit`（参考）、`qa.BeginRead`（参考）、`qa.BeginWrite`（参考）、`qa.beginTransaction`（参考）、`qa.ReplayReceipt`（参考）、`qa.SaveReceipt`（参考）、`qa.ReadPolicy`（参考）、`qa.ReadConversation`（参考）、`qa.ReadTurn`（参考）、`qa.IsTerminal`（参考）、`qa.CheckTurnLease`（参考）、`qa.SaveTurn`（参考）、`qa.ServiceReadContext`（参考）、`qa.ReadPublishedDocument`（参考）、`qa.ValidateCitations`（参考）、`qa.PublicationFingerprint`（参考）、`qa.ResolveTaskMemory`（参考）、`qa.CursorActor`（参考） |
| `modules/qa/internal/config/policy.go` | 手写 | 管理公开问答策略所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PutQaPolicy` |
| `modules/qa/internal/conversation/create.go` | 手写 | 创建伪名访客会话所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CreateConversation` |
| `modules/qa/internal/conversation/read.go` | 手写 | 读取自有访客历史所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.GetConversation` |
| `modules/qa/internal/turn/start.go` | 手写 | 准入后开始问答轮次所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.StartTurn` |
| `modules/qa/internal/admission/permit.go` | 手写 | 检查准入绑定所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ValidateAdmission`（参考） |
| `modules/qa/internal/normalize/question.go` | 手写 | 稳定规范问题和cache锚点所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.NormalizeQuestion`（参考）、`qa.validInputText`（参考）、`qa.normalizeWhitespace`（参考）、`qa.foldNaturalCase`（参考）、`qa.isProtectedToken`（参考）、`qa.hasSignificantUppercase`（参考）、`qa.extractAnchors`（参考）、`qa.isAnchorToken`（参考） |
| `modules/qa/internal/turn/context.go` | 手写 | 固定回答上下文版本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ResolveTurnContext`（参考） |
| `modules/qa/internal/cache/exact.go` | 手写 | 查询精确精选缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.LookupExactCache`（参考）、`qa.ScopeHash`（参考）、`qa.ReadCache`（参考）、`qa.SaveCache`（参考）、`qa.InsertCache`（参考）、`qa.EmitCacheChanged`（参考） |
| `modules/qa/internal/cache/semantic.go` | 手写 | 保守查询语义缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.LookupSemanticCache`（参考）、`qa.cosineSimilarity`（参考） |
| `modules/qa/internal/cache/validate.go` | 手写 | 验证缓存仍可公开交付所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ValidateCachedAnswer`（参考） |
| `modules/qa/internal/answer/retrieve.go` | 手写 | 取有源可见证据所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RetrieveEvidence`（参考） |
| `modules/qa/internal/answer/prompt.go` | 手写 | 构造有界回答提示所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.BuildAnswerPrompt`（参考） |
| `modules/qa/internal/answer/generate.go` | 手写 | 执行流式答案调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.GenerateAnswer`（参考） |
| `modules/qa/internal/answer/citation.go` | 手写 | 逐条保护引用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ValidateCitation`（参考） |
| `modules/qa/internal/stream/store.go` | 手写 | 持久流式输出事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.AppendAnswerEvent`（参考）、`qa.SaveEvent`（参考） |
| `modules/qa/internal/stream/read.go` | 手写 | SSE断线重连所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ResumeAnswerStream` |
| `modules/qa/internal/turn/complete.go` | 手写 | 原子结束可见答案所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CompleteTurn`（参考）、`qa.replayCompletedTurn`（参考）、`qa.releaseTurnPermit`（参考） |
| `modules/qa/internal/turn/cancel.go` | 手写 | 取消问答及所有子调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CancelTurn`、`qa.cancelTurnDependencies`（参考） |
| `modules/qa/internal/feedback/feedback.go` | 手写 | 保存访客单条反馈所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RecordFeedback` |
| `modules/qa/internal/cache/curation.go` | 手写 | 人工创建精选候选所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ProposeCacheEntry`、`qa.ApproveCacheEntry`、`qa.DisableCacheEntry` |
| `modules/qa/internal/cache/embedding.go` | 手写 | 为已批准缓存构建语义向量所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.BuildCacheEmbedding` |
| `modules/qa/internal/cache/invalidate.go` | 手写 | 失效受版本变化影响缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.InvalidateCache`、`qa.EmitCacheInvalidated`（参考） |
| `modules/qa/internal/admin/cache.go` | 手写 | 管理精选缓存所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ListCacheEntries` |
| `modules/qa/internal/cache/translation.go` | 手写 | 请求精选答案翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.TranslateCacheEntry`（参考） |
| `modules/qa/internal/history/compaction.go` | 手写 | 申请有界历史摘要所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CreateCompaction`（参考）、`qa.RunCompaction` |
| `modules/qa/internal/history/commit.go` | 手写 | 保护新历史不被旧摘要覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CommitCompaction`（参考） |
| `modules/qa/internal/debug/create.go` | 手写 | 隔离管理诊断所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.CreateDebugTurn` |
| `modules/qa/internal/debug/read.go` | 手写 | 读取管理诊断所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ReadDebugTurn` |
| `modules/qa/internal/conversation/delete.go` | 手写 | 删除自有对话数据所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.DeleteConversation` |
| `modules/qa/internal/runtime/recovery.go` | 手写 | 恢复崩溃问答所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RecoverTurns`（参考） |
| `modules/qa/internal/events/outbox.go` | 手写 | 发布问答脱敏事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PublishOutbox` |
| `modules/qa/internal/runtime/prune.go` | 手写 | 清理到期记录所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PruneExpiredRecords`（参考）、`qa.RedactConversation`（参考） |
| `modules/qa/internal/protection/protocol.go` | 手写 | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.AcquireBarrier`、`qa.ReleaseBarrier`、`qa.ExportSnapshot`、`qa.StageSnapshot`、`qa.DiscardSnapshot` |
| `modules/qa/ui/src/views/OverviewView.vue` | UI | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `qa.LoadAdminOverview`（参考） |
| `modules/qa/ui/src/views/EditView.vue` | UI | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `qa.SubmitPrimaryEdit`（参考） |
| `modules/qa/internal/context/page.go` | 手写 | 验证当前页面上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ResolveCurrentPageHint`（参考） |
| `modules/qa/internal/context/preflight.go` | 手写 | 补全问题、分类、风险与缓存资格所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.PlanQuestion`（参考） |
| `modules/qa/internal/cache/eligibility.go` | 手写 | 严格缓存完整问题门；指代残留未写明，不返回可共享资格。Go包，禁止直接访问其他域私有存储。 | `qa.EvaluateCacheEligibility`（参考）、`qa.ineligible`（参考） |
| `modules/qa/internal/cache/query-embedding.go` | 手写 | 同查询只生成一次嵌入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.AcquireQueryEmbedding`（参考） |
| `modules/qa/internal/answer/inspect.go` | 手写 | 快速检索员有限核查所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.InspectEvidence`（参考） |
| `modules/qa/internal/cache/automatic.go` | 手写 | 按old语义自动保存合格答案所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.WriteAutomaticCache`（参考） |
| `modules/qa/internal/cache/localized.go` | 手写 | 保存缓存的多语言回答所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.StoreLocalizedCacheAnswer` |
| `modules/qa/internal/admin/calibration.go` | 手写 | 缓存阈值校准所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.ReadCacheCalibration` |
| `modules/qa/internal/runtime/worker.go` | 手写 | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `qa.RunWorker` |
| `modules/qa/deploy/service.config.json` | 配置 | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/qa/migrations/001_initial.sql` | 迁移 | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 | 无 |
| `modules/qa/cmd/qa/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `qa.main` |
| `modules/qa/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `qa.BuildApplication` |
| `modules/qa/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `qa.Serve`、`qa.Shutdown`、`qa.Health`、`qa.Ready`、`qa.NewOperationHandlers`（参考）、`qa.jsonOperation`（参考）、`qa.operationContext`（参考）、`qa.writeResult`（参考）、`qa.serveAnswerStream`（参考）、`qa.answerEventStream.Next`（参考）、`qa.answerEventStream.Close`（参考） |
| `modules/qa/go.mod` | 配置 | 独立module nice-wiki/modules/qa；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/qa/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/qa/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/qa；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/qa/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/qa与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/qa/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/qa/ui/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/qa/ui/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/qa/ui/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/qa/ui/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/qa/ui/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/qa/ui/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `qa.mountModuleUI` |
| `modules/qa/internal/events/subscription.go` | 手写 | 订阅并分派本域登记的跨域事件 | `qa.ConsumeEvents` |
| `modules/qa/internal/runtime/model_client.go` | 手写 | 经模型网关执行一次调用并读取到终态 | `qa.InvokeModel`（参考）、`qa.ModelText`（参考） |
| `modules/qa/internal/turn/execute.go` | 手写 | 按固定顺序执行一轮问答 | `qa.ExecuteTurn` |
| `modules/qa/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/qa/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/qa/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/qa/ui/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
| `modules/qa/internal/admin/records.go` | 手写 | 浏览与筛选问答记录 | `qa.ListTurnRecords`、`qa.ReadTurnRecord` |
| `modules/qa/internal/admin/policy.go` | 手写 | 读取问答策略 | `qa.ReadQaPolicy`、`qa.ReadPublicCapability` |
| `modules/qa/internal/snapshot_archive.go` | 手写 | 受控读取已完成域快照归档流 | `qa.ReadSnapshotArchive` |
