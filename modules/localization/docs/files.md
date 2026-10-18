# localization 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 95 个：手写 75、UI 3、生成 5、配置 10、迁移 2。函数 163 个：约束 48，参考 115。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/localization/internal/runtime/bootstrap.go` | 手写 | 启动本地化服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.LoadRuntime`（参考）、`localization.recoverJournal`（参考）、`localization.Root`（参考）、`localization.ReserveObjectGeneration`（参考） |
| `modules/localization/internal/language/tag.go` | 手写 | 规范语言标识；大小写、脚本与私用归一化未写明，成功路径保持失败关闭。 | `localization.NormalizeLanguageTag`（参考） |
| `modules/localization/internal/language/config.go` | 手写 | 更新语言启用配置；同事务完成代次、屏障、CAS、原回执和语言变更事件。 | `localization.PutLanguage`、`localization.SetLanguageEnabled`、`localization.rejectLanguageAlias`（参考） |
| `modules/localization/internal/language/plan_enable.go` | 手写 | 语言计划最终启用事务。 | `localization.EnableLanguageForPlan`（参考） |
| `modules/localization/internal/language/read.go` | 手写 | 读取当前代次语言配置；不返回模型密钥，不连接翻译正文。 | `localization.ListLanguages`、`localization.LanguageForTag`（参考） |
| `modules/localization/internal/language/guard.go` | 手写 | 语言配置参考助手：已核本站、代次水位、维护阶段、local_barrier、7天原回执、语言行读取和language.changed。不单独暴露HTTP。 | `localization.acceptStoredTag`（参考） |
| `modules/localization/internal/language/http.go` | 手写 | 把putLanguage、setLanguageEnabled、listLanguages绑定到真实处理函数；不装配其余操作。 | 无 |
| `modules/localization/internal/events/publication.go` | 手写 | 仅发布触发已配置翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.HandlePublication` |
| `modules/localization/internal/translation/create.go` | 手写 | 申请源修订翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CreateTranslation` |
| `modules/localization/internal/source/content.go` | 手写 | 读取翻译源所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | 无 |
| `modules/localization/internal/translation/units.go` | 手写 | 确定性切分翻译单元所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReadTranslationUnits` |
| `modules/localization/internal/translation/incremental.go` | 手写 | 增量选择待翻译单元所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | 无 |
| `modules/localization/internal/glossary/resolve.go` | 手写 | 取得确定性术语快照所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | 无 |
| `modules/localization/internal/translation/prompt.go` | 手写 | 构造有界翻译请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.BuildTranslationPrompt`（参考） |
| `modules/localization/internal/translation/execute.go` | 手写 | 执行统一网关翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RunTranslationBatch`（参考）、`localization.ExecuteTranslation` |
| `modules/localization/internal/translation/validate.go` | 手写 | 校验模型输出格式保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ValidateTranslationOutput`（参考）、`localization.CheckInlineTokens`（参考）、`localization.VisibleCharacterCount`（参考） |
| `modules/localization/internal/translation/commit.go` | 手写 | 提交翻译防旧覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CommitTranslationBatch`（参考） |
| `modules/localization/internal/translation/complete.go` | 手写 | 完成派生翻译manifest所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CompleteTranslation`（参考） |
| `modules/localization/internal/translation/read.go` | 手写 | 公开读取当前译文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReadTranslation`、`localization.LatestManifest`（参考） |
| `modules/localization/internal/human/edit.go` | 手写 | 人工编辑并自动保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | 无 |
| `modules/localization/internal/human/protection.go` | 手写 | 显式启停人工保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | 无 |
| `modules/localization/internal/translation/cancel.go` | 手写 | 取消翻译子链路所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CancelTranslation` |
| `modules/localization/internal/glossary/write.go` | 手写 | 人工维护术语所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PutGlossaryTerm`、`localization.DeleteGlossaryTerm`、`localization.NormalizeTerm`（参考）、`localization.StoreTerm`（参考）、`localization.AdvanceGeneration`（参考）、`localization.PublishChange`（参考） |
| `modules/localization/internal/glossary/read.go` | 手写 | 管理术语列表所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ListGlossary`、`localization.scanTerm`（参考）、`localization.Generation`（参考）、`localization.ResolveGlossary`（参考） |
| `modules/localization/internal/glossary/approved.go` | 手写 | 应用memory已批准提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ApplyApprovedGlossaryProposal` |
| `modules/localization/internal/events/glossary.go` | 手写 | 使机翻版本按术语变化失效所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | 无 |
| `modules/localization/internal/interface/validate.go` | 手写 | 校验界面字符串契约所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ValidateInterfaceCatalog`（参考）、`localization.SourceCatalog`（参考）、`localization.CheckString`（参考）、`localization.ValidateInSnapshot`（参考） |
| `modules/localization/internal/interface/write.go` | 手写 | 人工编辑界面文本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PutInterfaceString` |
| `modules/localization/internal/interface/translate.go` | 手写 | 显式翻译界面缺项所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.TranslateInterfaceCatalog`、`localization.RequestInterfaceTranslation` |
| `modules/localization/internal/interface/read.go` | 手写 | 读取运行界面字典所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReadInterfaceCatalog`、`localization.EnabledLanguage`（参考）、`localization.ReadMerged`（参考） |
| `modules/localization/internal/suggestion/select.go` | 手写 | 给访客语言建议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.SuggestLanguage` |
| `modules/localization/internal/suggestion/preference.go` | 手写 | 保存显式访客语言偏好所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.SaveLanguagePreference`、`localization.NewPreferenceHandlers`（参考） |
| `modules/localization/internal/cache-answer/create.go` | 手写 | 接受QA精选翻译命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.TranslateCachedAnswer`、`localization.ExecuteCacheAnswerTranslation` |
| `modules/localization/internal/cache-answer/commit.go` | 手写 | 保存精选翻译供QA审核所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CommitCacheAnswerTranslation`（参考） |
| `modules/localization/internal/events/withdraw.go` | 手写 | 处理源撤回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.InvalidateDocumentTranslations` |
| `modules/localization/internal/runtime/recovery.go` | 手写 | 恢复翻译任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RecoverTranslationLeases`（参考） |
| `modules/localization/internal/events/outbox.go` | 手写 | 持久发布翻译事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PublishOutbox` |
| `modules/localization/internal/protection/protocol.go` | 手写 | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.AcquireBarrier`、`localization.ReleaseBarrier`、`localization.ExportSnapshot`、`localization.StageSnapshot`、`localization.DiscardSnapshot` |
| `modules/localization/ui/src/views/OverviewView.vue` | UI | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `localization.LoadAdminOverview`（参考） |
| `modules/localization/ui/src/views/EditView.vue` | UI | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `localization.SubmitPrimaryEdit`（参考） |
| `modules/localization/internal/translation/tm.go` | 手写 | 精确翻译记忆复用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReuseTranslationMemory`（参考）、`localization.ReuseInSnapshot`（参考） |
| `modules/localization/internal/glossary/extract.go` | 手写 | 首次翻译直接抽取入术语表所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ExtractAutomaticGlossary`（参考） |
| `modules/localization/internal/glossary/rollback.go` | 手写 | 整批撤销自动术语所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RollbackAutomaticGlossaryBatch` |
| `modules/localization/internal/translation/fallback.go` | 手写 | 逐级翻译失败降级所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PlanTranslationFallback`（参考） |
| `modules/localization/internal/translation/terminology.go` | 手写 | 检查规定译法符合度所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CheckTerminologyCompliance`（参考） |
| `modules/localization/internal/translation/reflection.go` | 手写 | 可选反思润色所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReflectTranslation`（参考） |
| `modules/localization/internal/cache-answer/read.go` | 手写 | 读取已完成的自动缓存翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReadCacheAnswerTranslation` |
| `modules/localization/internal/runtime/worker.go` | 手写 | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RunWorker` |
| `modules/localization/deploy/service.config.json` | 配置 | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/localization/migrations/001_initial.sql` | 迁移 | schemaVersion1建本域表与唯一约束；nw_outbox/nw_inbox由平台EnsureSchema建立不在此重复；control区决定日志独立且不属于业务恢复。 | 无 |
| `modules/localization/migrations/002_language_config.sql` | 迁移 | 只追加语言配置原回执、代次水位和本地屏障读侧；不播种语言、费用或翻译任务。初始屏障为released，不实现AcquireBarrier。 | 无 |
| `modules/localization/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 | `localization.All`（参考） |
| `modules/localization/cmd/localization/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `localization.main` |
| `modules/localization/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `localization.BuildApplication` |
| `modules/localization/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `localization.Serve`、`localization.Shutdown`、`localization.Health`、`localization.Ready` |
| `modules/localization/go.mod` | 配置 | 独立module nice-wiki/modules/localization；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/localization/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/localization/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/localization；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/localization/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/localization与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/localization/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/localization/ui/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/localization/ui/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/localization/ui/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/localization/ui/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/localization/ui/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/localization/ui/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `localization.mountModuleUI` |
| `modules/localization/internal/events/subscription.go` | 手写 | 订阅并分派本域登记的跨域事件 | `localization.ConsumeEvents` |
| `modules/localization/internal/runtime/model_client.go` | 手写 | 经模型网关执行一次调用并读取到终态 | `localization.InvokeModel`（参考）、`localization.ConfigureExecutionAdapters`（参考）、`localization.PublishBatch`（参考）、`localization.ConfigurePlanAdapters`（参考）、`localization.validatePlanPayload`（参考）、`localization.ResolvePlanTaskRoute`（参考）、`localization.EstimatePlanTextTokens`（参考）、`localization.QuotePlanCost`（参考）、`localization.CreatePlanJob`（参考） |
| `modules/localization/internal/translation/language_plan.go` | 手写 | 增量规划与启用前免费估价、明确批准和持久任务意图 | `localization.EstimateLanguagePlan`、`localization.ApproveLanguagePlan`、`localization.estimateDocumentPlan`（参考）、`localization.readInterfacePlan`（参考）、`localization.conservativeOutputTokens`（参考）、`localization.combineLanguagePlanQuotes`（参考）、`localization.planRouteFromRuntime`（参考）、`localization.checkedInt`（参考）、`localization.saveLanguagePlan`（参考）、`localization.readPlanResult`（参考）、`localization.checkLanguagePlanForApproval`（参考）、`localization.selectApprovalOperation`（参考）、`localization.checkLanguagePlanRevisions`（参考）、`localization.loadLanguagePlan`（参考）、`localization.ensureLanguagePlanJobs`（参考）、`localization.ensureLanguagePlanJob`（参考）、`localization.loadLanguagePlanJobs`（参考）、`localization.samePlanDocuments`（参考）、`localization.samePlanRoute`（参考）、`localization.sameApprovalOperation`（参考）、`localization.planJobType`（参考）、`localization.planJobIdempotencyKey`（参考）、`localization.checkLanguagePlanLocalState`（参考） |
| `modules/localization/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/localization/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/localization/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/localization/ui/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
| `modules/localization/internal/language/transaction.go` | 手写 | 本域术语或既有事务机制的实际参考级支持。 | `localization.ReadData`（参考）、`localization.Mutate`（参考）、`localization.RequireAdministrator`（参考）、`localization.EmitReference`（参考）、`localization.StorageError`（参考）、`localization.WriteData`（参考） |
| `modules/localization/internal/glossary/http.go` | 手写 | 本域术语或既有事务机制的实际参考级支持。 | `localization.NewGlossaryHandlers`（参考）、`localization.glossaryJSON`（参考） |
| `modules/localization/internal/glossary/change.go` | 手写 | 本域术语或既有事务机制的实际参考级支持。 | `localization.HandleGlossaryChange`（参考） |
| `modules/localization/migrations/003_glossary_catalog.sql` | 手写 | 术语软删、持久代数、固定字典来源及批准命令绑定的只追加迁移。 | 无 |
| `modules/localization/internal/translation/plan.go` | 手写 | 本域权威译文、纯翻译规则、固定字典或平台适配的实际实现。 | `localization.PlanIncrementalTranslation`（参考） |
| `modules/localization/internal/translation/store.go` | 手写 | 本域权威译文、纯翻译规则、固定字典或平台适配的实际实现。 | `localization.objectPath`（参考）、`localization.ReadObject`（参考）、`localization.WriteObject`（参考） |
| `modules/localization/internal/translation/edit.go` | 手写 | 本域权威译文、纯翻译规则、固定字典或平台适配的实际实现。 | `localization.EditTranslationUnit`、`localization.SetUnitProtection` |
| `modules/localization/internal/translation/http.go` | 手写 | 本域权威译文、纯翻译规则、固定字典或平台适配的实际实现。 | `localization.NewTranslationHandlers`（参考） |
| `modules/localization/internal/interface/http.go` | 手写 | 本域权威译文、纯翻译规则、固定字典或平台适配的实际实现。 | `localization.NewInterfaceHandlers`（参考） |
| `modules/localization/internal/cache-answer/http.go` | 手写 | 本域权威译文、纯翻译规则、固定字典或平台适配的实际实现。 | `localization.NewCacheTranslationHandlers`（参考） |
| `modules/localization/internal/language/protocol.go` | 手写 | 本域权威译文、纯翻译规则、固定字典或平台适配的实际实现。 | `localization.JSONHandler`（参考） |
| `modules/localization/migrations/004_translation_objects.sql` | 手写 | 译文归属证明、复合身份及权威对象manifest的只追加迁移。 | 无 |
| `modules/localization/internal/translation/split.go` | 手写 | 本域完整源模板/只读交付或权威客户端的实际实现。 | `localization.SplitTranslationUnits`（参考）、`localization.AppendPath`（参考）、`localization.CollectSlots`（参考）、`localization.AppendTextSlots`（参考）、`localization.AppendSlot`（参考）、`localization.WithoutSourceIDs`（参考） |
| `modules/localization/internal/translation/restore.go` | 手写 | 本域完整源模板/只读交付或权威客户端的实际实现。 | `localization.RestoreSourceTree`（参考）、`localization.ParseSlotValues`（参考）、`localization.ReplaceSourceText`（参考） |
| `modules/localization/internal/translation/source.go` | 手写 | 本域完整源模板/只读交付或权威客户端的实际实现。 | `localization.FetchTranslationSource`（参考） |
| `modules/localization/internal/translation/progress.go` | 手写 | 本域完整源模板/只读交付或权威客户端的实际实现。 | `localization.ListTranslationProgress` |
| `modules/localization/internal/runtime/source_client.go` | 手写 | 本域完整源模板/只读交付或权威客户端的实际实现。 | `localization.ConfigureSourceClient`（参考）、`localization.ValidateSourcePayload`（参考）、`localization.SourceOperation`（参考）、`localization.FetchPublishedSnapshot`（参考）、`localization.FetchCurrentPublishedSnapshot`（参考）、`localization.readPublishedSnapshot`（参考）、`localization.CheckPublication`（参考）、`localization.CursorKey`（参考）、`localization.ReadPublicTree`（参考） |
| `modules/localization/internal/translation/projection.go` | 手写 | 按真实稳定节点和字段段形状保留人工/保护译值，不按全局位置猜映射。 | `localization.StableSlotKey`（参考）、`localization.CollectSlotFields`（参考）、`localization.BuildProtectedProjection`（参考）、`localization.ProjectProtectedUnit`（参考）、`localization.ProtectedAnchorShape`（参考） |
| `modules/localization/internal/language/suggestion_policy.go` | 手写 | 管理员明确开启的付费建议批准与网关只读核验 | `localization.ReadLanguageSuggestionPolicy`、`localization.PutLanguageSuggestionPolicy`、`localization.ReadLanguageSuggestionApproval`、`localization.readSuggestionPolicy`（参考）、`localization.NewSuggestionPolicyHandlers`（参考） |
| `modules/localization/migrations/005_suggestion_policy.sql` | 手写 | 只追加付费建议当前指针与不可变审批历史，不种默认允许 | 无 |
| `modules/localization/migrations/006_language_plan.sql` | 手写 | 只追加语言估价计划、发布revision快照、原批准scope与逐任务持久意图 | 无 |
| `modules/localization/internal/snapshot_archive.go` | 手写 | 受控读取已完成域快照归档流 | `localization.ReadSnapshotArchive` |
