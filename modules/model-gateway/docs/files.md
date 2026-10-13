# model-gateway 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=52；分类={"handwritten": 36, "ui": 3, "generated": 3, "config": 9, "migration": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/model-gateway/internal/runtime/bootstrap.go` | handwritten | 启动并验证本服务私有持久化与密钥引用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.LoadRuntime` |
| `modules/model-gateway/internal/catalog/provider.go` | handwritten | 验证供应方配置所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ValidateProvider`, `model-gateway.PutProvider`, `model-gateway.SetProviderEnabled` |
| `modules/model-gateway/internal/security/credential.go` | handwritten | 加密存储密钥所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutCredential`, `model-gateway.RotateCredential` |
| `modules/model-gateway/internal/catalog/model.go` | handwritten | 保存不可变模型身份版本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutModel`, `model-gateway.ValidateModel` |
| `modules/model-gateway/internal/catalog/profile.go` | handwritten | 管理AI档位与费用限制所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutProfile`, `model-gateway.ResolveProfile` |
| `modules/model-gateway/internal/routing/route.go` | handwritten | 确定不可变调用路线所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ResolveRoute` |
| `modules/model-gateway/internal/billing/quote.go` | handwritten | 计算可保守预留的最大费用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.QuoteCost` |
| `modules/model-gateway/internal/invocation/create.go` | handwritten | 接受统一AI调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CreateInvocation` |
| `modules/model-gateway/internal/billing/reservation.go` | handwritten | 原子扣占额度所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ReserveCost`, `model-gateway.ExtendReservation` |
| `modules/model-gateway/internal/invocation/dispatch.go` | handwritten | 派发供应方请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.DispatchInvocation` |
| `modules/model-gateway/internal/provider/adapter.go` | handwritten | 映射统一调用到供应方协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.BuildProviderRequest`, `model-gateway.ClassifyFailure` |
| `modules/model-gateway/internal/stream/store.go` | handwritten | 持久保存流序列所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.AppendStreamEvent`, `model-gateway.ResumeStream` |
| `modules/model-gateway/internal/billing/usage.go` | handwritten | 接受供应方用量证据所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RecordUsage` |
| `modules/model-gateway/internal/billing/settlement.go` | handwritten | 结算已知费用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.SettleInvocation` |
| `modules/model-gateway/internal/billing/reconcile.go` | handwritten | 保留不确定收费责任所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.MarkUnknownCharge`, `model-gateway.ReconcileCharge` |
| `modules/model-gateway/internal/invocation/cancel.go` | handwritten | 贯穿取消调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CancelInvocation` |
| `modules/model-gateway/internal/invocation/read.go` | handwritten | 读取调用状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.GetInvocation` |
| `modules/model-gateway/internal/tokens/budget.go` | handwritten | 保守估算请求预算所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.EstimateTokens`, `model-gateway.ValidateEmbeddingSpace` |
| `modules/model-gateway/internal/concurrency/lease.go` | handwritten | 限制供应方并发所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.AcquireProviderSlot`, `model-gateway.ReleaseProviderSlot` |
| `modules/model-gateway/internal/events/outbox.go` | handwritten | 原子记录业务事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.EnqueueOutbox`, `model-gateway.PublishOutbox` |
| `modules/model-gateway/internal/runtime/recovery.go` | handwritten | 恢复崩溃中的调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RecoverLeases` |
| `modules/model-gateway/internal/admin/read.go` | handwritten | 管理端查询费用账本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.GetLedger`, `model-gateway.GetPriceBook`, `model-gateway.ReadModels`, `model-gateway.ReadProfiles`, `model-gateway.ReadFeeBudgetPolicy`, `model-gateway.PutFeeBudgetPolicy` |
| `modules/model-gateway/internal/security/redact.go` | handwritten | 生成最小运行日志所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RedactLog` |
| `modules/model-gateway/internal/admin/usage.go` | handwritten | 跨服务读取权威费用状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.QueryUsage` |
| `modules/model-gateway/internal/protection/protocol.go` | handwritten | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.AcquireBarrier`, `model-gateway.ReleaseBarrier`, `model-gateway.ExportSnapshot`, `model-gateway.ValidateSnapshot`, `model-gateway.PrepareSnapshot`, `model-gateway.CommitSnapshot`, `model-gateway.RollbackSnapshot` |
| `modules/model-gateway/ui/src/views/OverviewView.vue` | ui | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `model-gateway.LoadAdminOverview` |
| `modules/model-gateway/ui/src/views/EditView.vue` | ui | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `model-gateway.SubmitPrimaryEdit` |
| `modules/model-gateway/internal/catalog/task.go` | handwritten | 超管维护单任务模型覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutTaskAssignment`, `model-gateway.ReadTaskAssignments` |
| `modules/model-gateway/internal/catalog/refresh.go` | handwritten | 刷新公开模型目录所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RefreshModelCatalog` |
| `modules/model-gateway/internal/admin/probe.go` | handwritten | 显式测试连接所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CreateConnectionProbe` |
| `modules/model-gateway/internal/invocation/fallback.go` | handwritten | 费用安全保留备用与格式重试所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CreateFallbackAttempt` |
| `modules/model-gateway/internal/provider/anthropic.go` | handwritten | 映射Anthropic消息协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.MapAnthropicRequest` |
| `modules/model-gateway/internal/provider/gemini.go` | handwritten | 映射Gemini协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.MapGeminiRequest` |
| `modules/model-gateway/internal/provider/rerank.go` | handwritten | 解析rerank协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ParseRerankResponse` |
| `modules/model-gateway/internal/runtime/worker.go` | handwritten | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RunWorker` |
| `modules/model-gateway/contracts/generated/schema.gen.go` | generated | 未来由根contract compiler从本域types/openapi生成DTO，Generator(inputSchemas []SchemaRef, outputDirectory string) error；生成前schema refs必须解析，禁止手写编辑。 |  |
| `modules/model-gateway/deploy/service.config.json` | config | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/model-gateway/migrations/001_initial.sql` | migration | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 |  |
| `modules/model-gateway/cmd/model-gateway/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M07-F901` |
| `modules/model-gateway/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M07-F902` |
| `modules/model-gateway/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M07-F903`, `M07-F904`, `M07-F905`, `M07-F906` |
| `modules/model-gateway/go.mod` | config | 独立module nice-wiki/modules/model-gateway；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/model-gateway/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/model-gateway/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/model-gateway；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/model-gateway/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/model-gateway与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/model-gateway/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
| `modules/model-gateway/ui/package.json` | config | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 |  |
| `modules/model-gateway/ui/pnpm-lock.yaml` | generated | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest |  |
| `modules/model-gateway/ui/vite.config.ts` | config | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token |  |
| `modules/model-gateway/ui/tsconfig.json` | config | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 |  |
| `modules/model-gateway/ui/index.html` | config | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret |  |
| `modules/model-gateway/ui/src/main.ts` | ui | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `M07-F907` |
