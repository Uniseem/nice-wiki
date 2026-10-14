# memory 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 53 个：手写 37、UI 3、生成 3、配置 9、迁移 1。函数 57 个：约束 35，参考 22。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/memory/internal/runtime/bootstrap.go` | 手写 | 启动记忆服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.LoadRuntime`（参考） |
| `modules/memory/internal/entry/validate.go` | 手写 | 验证记忆内容与预算所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ValidateEntry`（参考） |
| `modules/memory/internal/entry/write.go` | 手写 | 创建人工记忆条目所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.CreateEntry`、`memory.UpdateEntry`、`memory.ArchiveEntry`、`memory.RestoreEntry`（参考） |
| `modules/memory/internal/entry/read.go` | 手写 | 读取人工记忆或历史所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ReadEntry`、`memory.ListEntries` |
| `modules/memory/internal/entry/history.go` | 手写 | 读取不可变记忆历史所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ListEntryRevisions` |
| `modules/memory/internal/mapping/write.go` | 手写 | 维护任务映射所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.PutMapping`、`memory.RemoveMapping` |
| `modules/memory/internal/mapping/order.go` | 手写 | 稳定重排任务记忆所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ReorderMappings`（参考） |
| `modules/memory/internal/mapping/read.go` | 手写 | 读取任务映射所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ListMappings` |
| `modules/memory/internal/context/budget.go` | 手写 | 配置任务注入上限所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.PutTaskBudget`、`memory.ReadTaskBudget`（参考） |
| `modules/memory/internal/context/select.go` | 手写 | 按范围确定注入候选所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.SelectContextEntries`（参考）、`memory.ApplyContextBudget`（参考） |
| `modules/memory/internal/context/render.go` | 手写 | 生成防混淆记忆上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.RenderContext`（参考） |
| `modules/memory/internal/context/service.go` | 手写 | 跨服务取得版本化记忆上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ResolveContext` |
| `modules/memory/internal/context/manifest.go` | 手写 | 查询版本证据清单所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ReadContextManifest`（参考） |
| `modules/memory/internal/assistant/create.go` | 手写 | 显式运行记忆助手所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.CreateAssistantRun` |
| `modules/memory/internal/assistant/prompt.go` | 手写 | 构造只允许提议助手输入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.BuildAssistantPrompt`（参考） |
| `modules/memory/internal/assistant/execute.go` | 手写 | 通过MG产生候选提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.RunAssistant`（参考） |
| `modules/memory/internal/proposal/validate.go` | 手写 | 验证候选提议闭包所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ValidateProposal`（参考） |
| `modules/memory/internal/assistant/commit.go` | 手写 | 提交候选防旧覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.CommitAssistantProposals`（参考） |
| `modules/memory/internal/assistant/read.go` | 手写 | 管理助手提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ReadAssistantRun` |
| `modules/memory/internal/proposal/approve.go` | 手写 | 人工批准具体提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ApproveProposal`、`memory.RejectProposal` |
| `modules/memory/internal/proposal/apply-entry.go` | 手写 | 应用批准的记忆内容所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ApplyEntryProposal`（参考） |
| `modules/memory/internal/proposal/glossary-command.go` | 手写 | 产生已批准术语命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.EnqueueGlossaryCommand`（参考） |
| `modules/memory/internal/proposal/glossary-dispatch.go` | 手写 | 异步发送批准术语命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.DispatchGlossaryCommand`（参考） |
| `modules/memory/internal/events/glossary-result.go` | 手写 | 消费术语批准结果所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.HandleGlossaryProposalResult` |
| `modules/memory/internal/assistant/cancel.go` | 手写 | 取消未完成助手调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.CancelAssistantRun` |
| `modules/memory/internal/runtime/recovery.go` | 手写 | 恢复助手执行与批准命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.RecoverAssistantLeases`（参考） |
| `modules/memory/internal/events/outbox.go` | 手写 | 发布记忆引用式事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.PublishOutbox` |
| `modules/memory/internal/runtime/prune.go` | 手写 | 回收无引用派生注入清单所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.PruneContextSnapshots`（参考） |
| `modules/memory/internal/protection/protocol.go` | 手写 | 取得本地维护屏障所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.AcquireBarrier`、`memory.ReleaseBarrier`、`memory.ExportSnapshot`、`memory.StageSnapshot`、`memory.DiscardSnapshot` |
| `modules/memory/ui/src/views/OverviewView.vue` | UI | 加载本域后台概览所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `memory.LoadAdminOverview`（参考） |
| `modules/memory/ui/src/views/EditView.vue` | UI | 提交本域明确人工修改所属未来实现文件；Vue3组合式界面，路由由portal装配。 | `memory.SubmitPrimaryEdit`（参考） |
| `modules/memory/internal/context/preflight.go` | 手写 | 保存前完整投影注入预算所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.ValidateProjectedBudgets`（参考） |
| `modules/memory/internal/assistant/overview.go` | 手写 | 自动维护项目概述并保护人工修改所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.UpdateProjectOverview`（参考） |
| `modules/memory/internal/assistant/conversation.go` | 手写 | 保存可连续的记忆助手对话所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.AppendAssistantMessage`、`memory.ReadAssistantConversation`、`memory.DeleteAssistantConversation` |
| `modules/memory/internal/runtime/worker.go` | 手写 | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 | `memory.RunWorker` |
| `modules/memory/contracts/generated/schema.gen.go` | 生成 | 未来由根contract compiler从本域types/openapi生成DTO，Generator(inputSchemas []SchemaRef, outputDirectory string) error；生成前schema refs必须解析，禁止手写编辑。 | 无 |
| `modules/memory/deploy/service.config.json` | 配置 | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref | 无 |
| `modules/memory/migrations/001_initial.sql` | 迁移 | 未来schemaVersion1建本域表、唯一约束与inbox/outbox索引；control.db独立迁移且不属于业务恢复。 | 无 |
| `modules/memory/cmd/memory/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 | `memory.main` |
| `modules/memory/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `memory.BuildApplication` |
| `modules/memory/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `memory.Serve`、`memory.Shutdown`、`memory.Health`、`memory.Ready` |
| `modules/memory/go.mod` | 配置 | 独立module nice-wiki/modules/memory；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 | 无 |
| `modules/memory/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 | 无 |
| `modules/memory/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/memory；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS | 无 |
| `modules/memory/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/memory与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 | 无 |
| `modules/memory/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 | 无 |
| `modules/memory/ui/package.json` | 配置 | 该实际UI根独立pnpm scripts build/dev/typecheck；Vue3+TypeScript6.0.x+Vite及已登记组件依赖，不复制其他模块业务 | 无 |
| `modules/memory/ui/pnpm-lock.yaml` | 生成 | 由本UI package.json和pnpm解析生成的精确依赖锁；CI frozen-lockfile，不用浮动latest | 无 |
| `modules/memory/ui/vite.config.ts` | 配置 | 该UI根src入口与dist/hash资源输出，模块别名只到本域源码和公共UI资源；API经portal同源入口，不能嵌service token | 无 |
| `modules/memory/ui/tsconfig.json` | 配置 | 严格TypeScript6.0.x/Vue3 DOM配置，include现有src和vite配置，构建失败不发镜像 | 无 |
| `modules/memory/ui/index.html` | 配置 | 该UI独立dev/build入口，生产模块由portal或公开Go页面挂载，加载本域src/main.ts，无业务数据或secret | 无 |
| `modules/memory/ui/src/main.ts` | UI | 本实际UI根唯一Vue入口；仅装配已登记UI函数，不新增产品业务 | `memory.mountModuleUI` |
| `modules/memory/internal/events/subscription.go` | 手写 | 订阅并分派本域登记的跨域事件 | `memory.ConsumeEvents` |
