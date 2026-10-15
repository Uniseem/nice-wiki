# model-gateway 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 55 个：手写 36、UI 3、生成 5、配置 10、迁移 1。函数 67 个：约束 37，参考 30。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/model-gateway/internal/runtime/bootstrap.go` | 手写 | 启动并验证本服务私有持久化与密钥引用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.LoadRuntime`（参考） |
| `modules/model-gateway/internal/catalog/provider.go` | 手写 | 验证供应方配置所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ValidateProvider`（参考）、`model-gateway.PutProvider`、`model-gateway.SetProviderEnabled`（参考） |
| `modules/model-gateway/internal/security/credential.go` | 手写 | 加密存储密钥所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutCredential`、`model-gateway.RotateCredential` |
| `modules/model-gateway/internal/catalog/model.go` | 手写 | 保存不可变模型身份版本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutModel`、`model-gateway.ValidateModel`（参考） |
| `modules/model-gateway/internal/catalog/profile.go` | 手写 | 管理AI档位与费用限制所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutProfile`、`model-gateway.ResolveProfile`（参考） |
| `modules/model-gateway/internal/routing/route.go` | 手写 | 确定不可变调用路线所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ResolveRoute`（参考） |
| `modules/model-gateway/internal/billing/quote.go` | 手写 | 计算可保守预留的最大费用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.QuoteCost` |
| `modules/model-gateway/internal/invocation/create.go` | 手写 | 接受统一AI调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CreateInvocation` |
| `modules/model-gateway/internal/billing/reservation.go` | 手写 | 原子扣占额度所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ReserveCost`（参考）、`model-gateway.ExtendReservation`（参考） |
| `modules/model-gateway/internal/invocation/dispatch.go` | 手写 | 派发供应方请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.DispatchInvocation`（参考）、`model-gateway.ExecuteInvocation`（参考） |
| `modules/model-gateway/internal/provider/adapter.go` | 手写 | 映射统一调用到供应方协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.BuildProviderRequest`（参考）、`model-gateway.ClassifyFailure`（参考） |
| `modules/model-gateway/internal/stream/store.go` | 手写 | 持久保存流序列所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.AppendStreamEvent`（参考）、`model-gateway.ResumeStream` |
| `modules/model-gateway/internal/billing/usage.go` | 手写 | 接受供应方用量证据所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RecordUsage`（参考） |
| `modules/model-gateway/internal/billing/settlement.go` | 手写 | 结算已知费用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.SettleInvocation`（参考） |
| `modules/model-gateway/internal/billing/reconcile.go` | 手写 | 保留不确定收费责任所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.MarkUnknownCharge`（参考）、`model-gateway.ReconcileCharge` |
| `modules/model-gateway/internal/invocation/cancel.go` | 手写 | 贯穿取消调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CancelInvocation` |
| `modules/model-gateway/internal/invocation/read.go` | 手写 | 读取调用状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.GetInvocation` |
| `modules/model-gateway/internal/tokens/budget.go` | 手写 | 保守估算请求预算所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.EstimateTokens`（参考）、`model-gateway.ValidateEmbeddingSpace`（参考） |
| `modules/model-gateway/internal/concurrency/lease.go` | 手写 | 限制供应方并发所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.AcquireProviderSlot`（参考）、`model-gateway.ReleaseProviderSlot`（参考） |
| `modules/model-gateway/internal/events/outbox.go` | 手写 | 原子记录业务事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.EnqueueOutbox`（参考）、`model-gateway.PublishOutbox` |
| `modules/model-gateway/internal/runtime/recovery.go` | 手写 | 恢复崩溃中的调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RecoverLeases`（参考） |
| `modules/model-gateway/internal/admin/read.go` | 手写 | 管理端查询费用账本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.GetLedger`、`model-gateway.GetPriceBook`、`model-gateway.ReadModels`、`model-gateway.ReadProfiles`、`model-gateway.ReadFeeBudgetPolicy`、`model-gateway.PutFeeBudgetPolicy`、`model-gateway.ReadAdminCatalog` |
| `modules/model-gateway/internal/security/redact.go` | 手写 | 生成最小运行日志所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RedactLog`（参考） |
| `modules/model-gateway/internal/admin/usage.go` | 手写 | 跨服务读取权威费用状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.QueryUsage` |
| `modules/model-gateway/internal/protection/protocol.go` | 手写 | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.AcquireBarrier`、`model-gateway.ReleaseBarrier`、`model-gateway.ExportSnapshot`、`model-gateway.StageSnapshot`、`model-gateway.DiscardSnapshot` |
| `modules/model-gateway/ui/src/views/OverviewView.vue` | UI | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `model-gateway.LoadAdminOverview`（参考） |
| `modules/model-gateway/ui/src/views/EditView.vue` | UI | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `model-gateway.SubmitPrimaryEdit`（参考） |
| `modules/model-gateway/internal/catalog/task.go` | 手写 | 超管维护单任务模型覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.PutTaskAssignment`、`model-gateway.ReadTaskAssignments`、`model-gateway.ReadTaskAvailability` |
| `modules/model-gateway/internal/catalog/refresh.go` | 手写 | 刷新公开模型目录所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.RefreshModelCatalog` |
| `modules/model-gateway/internal/admin/probe.go` | 手写 | 显式测试连接所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CreateConnectionProbe` |
| `modules/model-gateway/internal/invocation/fallback.go` | 手写 | 费用安全保留备用与格式重试所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.CreateFallbackAttempt`（参考） |
| `modules/model-gateway/internal/provider/anthropic.go` | 手写 | 映射Anthropic消息协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.MapAnthropicRequest`（参考） |
| `modules/model-gateway/internal/provider/gemini.go` | 手写 | 映射Gemini协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.MapGeminiRequest`（参考） |
| `modules/model-gateway/internal/provider/rerank.go` | 手写 | 解析rerank协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `model-gateway.ParseRerankResponse`（参考） |
| `modules/model-gateway/deploy/service.config.json` | 配置 | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/model-gateway/migrations/001_initial.sql` | 迁移 | schemaVersion1建本域表与唯一约束；跨服务事件只写平台nw_outbox（EnsureSchema建立），不在此建自有outbox。 | 无 |
| `modules/model-gateway/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 | `model-gateway.All`（参考） |
| `modules/model-gateway/cmd/model-gateway/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `model-gateway.main` |
| `modules/model-gateway/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `model-gateway.BuildApplication` |
| `modules/model-gateway/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `model-gateway.Serve`、`model-gateway.Shutdown`、`model-gateway.Health`、`model-gateway.Ready` |
| `modules/model-gateway/go.mod` | 配置 | 独立module nice-wiki/modules/model-gateway；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/model-gateway/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/model-gateway/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/model-gateway；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/model-gateway/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/model-gateway与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/model-gateway/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/model-gateway/ui/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/model-gateway/ui/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/model-gateway/ui/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/model-gateway/ui/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/model-gateway/ui/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/model-gateway/ui/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `model-gateway.mountModuleUI` |
| `modules/model-gateway/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 | 无 |
| `modules/model-gateway/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 | 无 |
| `modules/model-gateway/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation | 无 |
| `modules/model-gateway/ui/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 | 无 |
