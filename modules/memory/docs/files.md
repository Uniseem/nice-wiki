# memory 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 56 个：手写 43、UI 1、生成 4、配置 6、迁移 2。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/memory/internal/runtime/bootstrap.go` | 手写 | 启动记忆服务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/entry/validate.go` | 手写 | 验证记忆内容与预算所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/entry/write.go` | 手写 | 创建人工记忆条目所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/entry/read.go` | 手写 | 读取人工记忆或历史所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/entry/history.go` | 手写 | 读取不可变记忆历史所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/mapping/write.go` | 手写 | 维护任务映射所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/mapping/order.go` | 手写 | 稳定重排任务记忆所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/mapping/read.go` | 手写 | 读取任务映射所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/context/budget.go` | 手写 | 配置任务注入上限所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/context/select.go` | 手写 | 按范围确定注入候选所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/context/render.go` | 手写 | 生成防混淆记忆上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/context/service.go` | 手写 | 跨服务取得版本化记忆上下文所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/context/manifest.go` | 手写 | 查询版本证据清单所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/create.go` | 手写 | 显式运行记忆助手所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/prompt.go` | 手写 | 构造只允许提议助手输入所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/execute.go` | 手写 | 通过MG产生候选提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/proposal/validate.go` | 手写 | 验证候选提议闭包所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/commit.go` | 手写 | 提交候选防旧覆盖所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/read.go` | 手写 | 管理助手提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/proposal/approve.go` | 手写 | 人工批准具体提议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/proposal/apply-entry.go` | 手写 | 应用批准的记忆内容所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/proposal/glossary-command.go` | 手写 | 产生已批准术语命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/proposal/glossary-dispatch.go` | 手写 | 异步发送批准术语命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/events/glossary-result.go` | 手写 | 消费术语批准结果所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/cancel.go` | 手写 | 取消未完成助手调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/runtime/recovery.go` | 手写 | 恢复助手执行与批准命令所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/events/outbox.go` | 手写 | 发布记忆引用式事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/runtime/prune.go` | 手写 | 回收无引用派生注入清单所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/context/preflight.go` | 手写 | 保存前完整投影注入预算所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/overview.go` | 手写 | 自动维护项目概述并保护人工修改所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/assistant/conversation.go` | 手写 | 保存可连续的记忆助手对话所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/internal/runtime/worker.go` | 手写 | 主动领取本域持久任务所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/memory/deploy/service.config.json` | 配置 | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/memory/migrations/001_initial.sql` | 迁移 | schemaVersion1建本域active业务表与唯一约束；nw_outbox/nw_inbox由平台EnsureSchema建立不在此重复；control库迁移见control/0001_control.sql，不属于业务恢复。 |
| `modules/memory/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续；另内嵌control区控制库迁移 |
| `modules/memory/migrations/control/0001_control.sql` | 迁移 | control区控制库初始迁移：control_state（屏障与安全点单例）、restore_receipts与schema_migrations；不随恢复换目录替换 |
| `modules/memory/cmd/memory/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/memory/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/memory/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/memory/go.mod` | 配置 | 独立module nice-wiki/modules/memory；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/memory/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/memory/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/memory；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/memory/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/memory与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/memory/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/memory/internal/events/subscription.go` | 手写 | 订阅并分派本域登记的跨域事件 |
| `modules/memory/internal/runtime/model_client.go` | 手写 | 经模型网关执行一次调用并读取到终态 |
| `modules/memory/internal/events/publication-count.go` | 手写 | 统计新发布数以触发项目概述更新 |
| `modules/memory/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/memory/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 |
| `modules/memory/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/memory/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 |
| `modules/memory/internal/runtime/business.go` | 手写 | 本域实际业务实现与受信装配辅助；仅访问本域私有数据。 |
| `modules/memory/internal/runtime/tasks.go` | 手写 | 本域实际业务实现与受信装配辅助；仅访问本域私有数据。 |
| `modules/memory/internal/transport/business.go` | 手写 | 本域实际业务实现与受信装配辅助；仅访问本域私有数据。 |
| `modules/memory/migrations/002_business.sql` | 手写 | 业务回执、提交manifest、派生清单及助手固定输入的只追加迁移。 |
| `modules/memory/web/src/index.tsx` | UI | 长期记忆：记忆条目的新建、编辑、归档与恢复 |
