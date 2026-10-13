# localization 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=62；分类={"handwritten": 46, "ui": 3, "generated": 3, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/localization/internal/runtime/bootstrap.go` | handwritten | 启动本地化服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.LoadRuntime` |
| `modules/localization/internal/language/tag.go` | handwritten | 规范语言标识所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.NormalizeLanguageTag` |
| `modules/localization/internal/language/config.go` | handwritten | 更新语言启用配置所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PutLanguage`, `localization.SetLanguageEnabled` |
| `modules/localization/internal/language/read.go` | handwritten | 读取语言列表所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ListLanguages` |
| `modules/localization/internal/events/publication.go` | handwritten | 仅发布触发已配置翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.HandlePublication` |
| `modules/localization/internal/translation/create.go` | handwritten | 申请源修订翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CreateTranslation` |
| `modules/localization/internal/source/content.go` | handwritten | 读取翻译源所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.FetchTranslationSource` |
| `modules/localization/internal/translation/units.go` | handwritten | 确定性切分翻译单元所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.SplitTranslationUnits` |
| `modules/localization/internal/translation/incremental.go` | handwritten | 增量选择待翻译单元所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PlanIncrementalTranslation` |
| `modules/localization/internal/glossary/resolve.go` | handwritten | 取得确定性术语快照所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ResolveGlossary` |
| `modules/localization/internal/translation/prompt.go` | handwritten | 构造有界翻译请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.BuildTranslationPrompt` |
| `modules/localization/internal/translation/execute.go` | handwritten | 执行统一网关翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RunTranslationBatch` |
| `modules/localization/internal/translation/validate.go` | handwritten | 校验模型输出格式保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ValidateTranslationOutput` |
| `modules/localization/internal/translation/commit.go` | handwritten | 提交翻译防旧覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CommitTranslationBatch` |
| `modules/localization/internal/translation/complete.go` | handwritten | 完成派生翻译manifest所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CompleteTranslation` |
| `modules/localization/internal/translation/read.go` | handwritten | 公开读取当前译文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReadTranslation` |
| `modules/localization/internal/human/edit.go` | handwritten | 人工编辑并自动保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.EditTranslationUnit` |
| `modules/localization/internal/human/protection.go` | handwritten | 显式启停人工保护所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.SetUnitProtection` |
| `modules/localization/internal/translation/cancel.go` | handwritten | 取消翻译子链路所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CancelTranslation` |
| `modules/localization/internal/glossary/write.go` | handwritten | 人工维护术语所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PutGlossaryTerm`, `localization.DeleteGlossaryTerm` |
| `modules/localization/internal/glossary/read.go` | handwritten | 管理术语列表所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ListGlossary` |
| `modules/localization/internal/glossary/approved.go` | handwritten | 应用memory已批准提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ApplyApprovedGlossaryProposal` |
| `modules/localization/internal/events/glossary.go` | handwritten | 使机翻版本按术语变化失效所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.HandleGlossaryChange` |
| `modules/localization/internal/interface/validate.go` | handwritten | 校验界面字符串契约所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ValidateInterfaceCatalog` |
| `modules/localization/internal/interface/write.go` | handwritten | 人工编辑界面文本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PutInterfaceString` |
| `modules/localization/internal/interface/translate.go` | handwritten | 显式翻译界面缺项所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.TranslateInterfaceCatalog` |
| `modules/localization/internal/interface/read.go` | handwritten | 读取运行界面字典所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReadInterfaceCatalog` |
| `modules/localization/internal/suggestion/select.go` | handwritten | 给访客语言建议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.SuggestLanguage` |
| `modules/localization/internal/suggestion/preference.go` | handwritten | 保存显式访客语言偏好所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.SaveLanguagePreference` |
| `modules/localization/internal/cache-answer/create.go` | handwritten | 接受QA精选翻译命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.TranslateCachedAnswer` |
| `modules/localization/internal/cache-answer/commit.go` | handwritten | 保存精选翻译供QA审核所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CommitCacheAnswerTranslation` |
| `modules/localization/internal/events/withdraw.go` | handwritten | 处理源撤回所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.InvalidateDocumentTranslations` |
| `modules/localization/internal/runtime/recovery.go` | handwritten | 恢复翻译任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RecoverTranslationLeases` |
| `modules/localization/internal/events/outbox.go` | handwritten | 持久发布翻译事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PublishOutbox` |
| `modules/localization/internal/protection/protocol.go` | handwritten | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.AcquireBarrier`, `localization.ReleaseBarrier`, `localization.ExportSnapshot`, `localization.ValidateSnapshot`, `localization.PrepareSnapshot`, `localization.CommitSnapshot`, `localization.RollbackSnapshot` |
| `modules/localization/ui/src/views/OverviewView.vue` | ui | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `localization.LoadAdminOverview` |
| `modules/localization/ui/src/views/EditView.vue` | ui | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `localization.SubmitPrimaryEdit` |
| `modules/localization/internal/translation/tm.go` | handwritten | 精确翻译记忆复用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReuseTranslationMemory` |
| `modules/localization/internal/glossary/extract.go` | handwritten | 首次翻译直接抽取入术语表所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ExtractAutomaticGlossary` |
| `modules/localization/internal/glossary/rollback.go` | handwritten | 整批撤销自动术语所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RollbackAutomaticGlossaryBatch` |
| `modules/localization/internal/translation/fallback.go` | handwritten | 逐级翻译失败降级所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.PlanTranslationFallback` |
| `modules/localization/internal/translation/terminology.go` | handwritten | 检查规定译法符合度所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.CheckTerminologyCompliance` |
| `modules/localization/internal/translation/reflection.go` | handwritten | 可选反思润色所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReflectTranslation` |
| `modules/localization/internal/cache-answer/read.go` | handwritten | 读取已完成的自动缓存翻译所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.ReadCacheAnswerTranslation` |
| `modules/localization/internal/runtime/worker.go` | handwritten | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `localization.RunWorker` |
| `modules/localization/contracts/generated/schema.gen.go` | generated | 未来由根contract compiler从本域types/openapi生成DTO，Generator(inputSchemas []SchemaRef, outputDirectory string) error；生成前schema refs必须解析，禁止手写编辑。 |  |
| `modules/localization/deploy/service.config.json` | config | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/localization/migrations/001_initial.sql` | migration | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 |  |
| `modules/localization/cmd/localization/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M10-F901` |
| `modules/localization/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M10-F902` |
| `modules/localization/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M10-F903`, `M10-F904`, `M10-F905`, `M10-F906` |
| `modules/localization/go.mod` | config | 独立module nice-wiki/modules/localization；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/localization/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/localization/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/localization；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/localization/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/localization与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/localization/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/localization/ui/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/localization/ui/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/localization/ui/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/localization/ui/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/localization/ui/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/localization/ui/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M10-F907` |
