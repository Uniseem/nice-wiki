# model-gateway 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 84 个：手写 69、UI 1、生成 4、配置 6、迁移 4。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/model-gateway/internal/runtime/bootstrap.go` | 手写 | 启动并验证本服务私有持久化与密钥引用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/catalog/provider.go` | 手写 | 验证供应方配置所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/security/credential.go` | 手写 | 凭据AES-GCM密文、敏感输入HMAC原回执与active版本CAS；旧引用生命周期回收尚未交付 |
| `modules/model-gateway/internal/catalog/model.go` | 手写 | 保存不可变模型身份版本所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/catalog/profile.go` | 手写 | 档位配置持久写入、真实任务档位解析与同代安全候选筛选；无供应方或费用操作 |
| `modules/model-gateway/internal/routing/route.go` | 手写 | 确定不可变调用路线所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/billing/quote.go` | 手写 | 计算可保守预留的最大费用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/invocation/create.go` | 手写 | 接受统一AI调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/billing/reservation.go` | 手写 | 原子扣占额度所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/invocation/dispatch.go` | 手写 | 派发供应方请求所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/provider/adapter.go` | 手写 | 映射统一调用到供应方协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/stream/store.go` | 手写 | 持久保存流序列所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/billing/usage.go` | 手写 | 接受供应方用量证据所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/billing/settlement.go` | 手写 | 结算已知费用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/billing/reconcile.go` | 手写 | 保留不确定收费责任所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/invocation/cancel.go` | 手写 | 贯穿取消调用所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/invocation/read.go` | 手写 | 读取调用状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/tokens/budget.go` | 手写 | 保守估算请求预算所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/concurrency/lease.go` | 手写 | 限制供应方并发所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/events/outbox.go` | 手写 | 原子记录业务事件所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/admin/read.go` | 手写 | 真实安全目录读取与费用策略；账本/价格及space/最近探测分支按台账未完成。 |
| `modules/model-gateway/internal/admin/usage.go` | 手写 | 跨服务读取权威费用状态所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/catalog/task.go` | 手写 | 任务分配配置读写与QA免费可用性读取；真实本域SQL、权限、CAS、回执和平台outbox，不调用供应方。 |
| `modules/model-gateway/internal/catalog/refresh.go` | 手写 | 刷新公开模型目录所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/admin/probe.go` | 手写 | 显式测试连接所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/invocation/fallback.go` | 手写 | 费用安全保留备用与格式重试所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/provider/anthropic.go` | 手写 | 映射Anthropic消息协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/provider/gemini.go` | 手写 | 映射Gemini协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/internal/provider/rerank.go` | 手写 | 解析rerank协议所属未来实现文件；Go包，禁止直接访问其他域私有存储。 |
| `modules/model-gateway/deploy/service.config.json` | 配置 | 未来非敏感配置：serviceId、listenAddress、privateDataDirectory、dependency URLs、timeout；secret仅部署引用；本阶段不创建。；启动职责：ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/model-gateway/migrations/001_initial.sql` | 迁移 | schemaVersion1建本域表与唯一约束；跨服务事件只写平台nw_outbox（EnsureSchema建立），不在此建自有outbox。 |
| `modules/model-gateway/migrations/migrations.go` | 手写 | 内嵌本域只追加SQL迁移（ADR0005）作为运行时唯一来源，校验编号从1起连续 |
| `modules/model-gateway/cmd/model-gateway/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/model-gateway/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/model-gateway/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/model-gateway/go.mod` | 配置 | 独立module nice-wiki/modules/model-gateway；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/model-gateway/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/model-gateway/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/model-gateway；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/model-gateway/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/model-gateway与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/model-gateway/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/model-gateway/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/model-gateway/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改；同时从根AI任务目录生成AITaskCatalog owner/defaultTier/RequiredCapabilities，禁止手抄或运行时跨域读源文件 |
| `modules/model-gateway/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/model-gateway/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 |
| `modules/model-gateway/migrations/002_task_availability.sql` | 迁移 | 只追加任务分配、完整QA/后台档位字段与AES-GCM凭据持久schema；无默认种子、无费用变更；凭据读取仅存在性和长度。 |
| `modules/model-gateway/migrations/003_fee_budget_policy.sql` | 手写 | 只追加费用策略单例和本地脱敏审计，无金额默认或探测种子，随active快照恢复。 |
| `modules/model-gateway/migrations/004_price_book.sql` | 迁移 | 只追加不可变人工价目历史，无价格种子、目录来源伪造或费用状态。 |
| `modules/model-gateway/migrations/005_ledger_reads.sql` | 迁移 | 仅追加nullable调用归属/created_at与真实读取水位、触发器、精确分页索引；不回填、不造费用事实。 |
| `modules/model-gateway/internal/runtime/invocation.go` | 手写 | 本域真实执行链私有事务及HTTP适配；不读取其他域私有表。 |
| `modules/model-gateway/internal/invocation/http.go` | 手写 | 本域真实执行链私有事务及HTTP适配；不读取其他域私有表。 |
| `modules/model-gateway/migrations/006_execution.sql` | 手写 | schemaVersion6追加真实attempt/预留投影/原输入回执/凭据引用/人工证明；旧行不猜填。 |
| `modules/model-gateway/internal/security/headers.go` | 手写 | 本域主密钥及供应方固定路线的安全映射，不访问其他域私有表。 |
| `modules/model-gateway/migrations/007_provider_dispatch.sql` | 手写 | schemaVersion7追加固定路线凭据引用、加密额外头及安全结果hash；不猜填旧引用。 |
| `modules/model-gateway/internal/tokens/tokenize.go` | 手写 | 固定版本免费真实BPE分词与知识服务HTTP入口 |
| `modules/model-gateway/internal/catalog/tokenizer.go` | 手写 | 免费分词读取同代模型固定身份 |
| `modules/model-gateway/internal/invocation/recovery.go` | 手写 | 本域原调用付费执行、取消恢复或供应方证据的实际闭环源码；禁止跨域私有访问。 |
| `modules/model-gateway/internal/invocation/execute.go` | 手写 | 本域原调用付费执行、取消恢复或供应方证据的实际闭环源码；禁止跨域私有访问。 |
| `modules/model-gateway/internal/invocation/finish.go` | 手写 | 本域原调用付费执行、取消恢复或供应方证据的实际闭环源码；禁止跨域私有访问。 |
| `modules/model-gateway/internal/runtime/approval.go` | 手写 | 本域原调用付费执行、取消恢复或供应方证据的实际闭环源码；禁止跨域私有访问。 |
| `modules/model-gateway/internal/provider/response.go` | 手写 | 本域原调用付费执行、取消恢复或供应方证据的实际闭环源码；禁止跨域私有访问。 |
| `modules/model-gateway/internal/provider/images.go` | 手写 | 本域原调用付费执行、取消恢复或供应方证据的实际闭环源码；禁止跨域私有访问。 |
| `modules/model-gateway/migrations/008_execution_evidence.sql` | 手写 | schemaVersion8追加供应方脱敏attempt_evidence与不可变execution hash；旧行NULL不猜填，不允许据旧缺证据自动重试。 |
| `modules/model-gateway/internal/runtime/retirement.go` | 手写 | 按全部费用和原引用证明销毁退休密文，保留审计 |
| `modules/model-gateway/internal/runtime/relay.go` | 手写 | 复用平台私有outbox真实relay |
| `modules/model-gateway/migrations/009_retired_secrets.sql` | 手写 | schema9退休材料版本审计与header墓碑 |
| `modules/model-gateway/migrations/010_model_catalog.sql` | 手写 | 明确源身份的公开目录实际更新，manualOverride优先，不猜接口/凭据/空间。 |
| `modules/model-gateway/internal/invocation/lookup.go` | 手写 | 原完整请求纯读查找与完整tracking证明 |
| `modules/model-gateway/migrations/011_invocation_lookup.sql` | 手写 | 创建键投影及不自动生产的tracking来源 |
| `modules/model-gateway/internal/admin/statistics.go` | 手写 | 固定历史水位安全posted账本变更页 |
| `modules/model-gateway/migrations/012_usage_statistics.sql` | 手写 | 只追加安全账项真实变更历史及entry修订；同原账本/void事务完整捕获，未知legacy不补造来源；修正原读水位遗漏void变化 |
| `modules/model-gateway/internal/catalog/bundled/catalog.go` | 手写 | 已批准MIT models.dev固定公开数据嵌入，不含外部业务代码 |
| `modules/model-gateway/internal/catalog/bundled/models.dev.json` | 手写 | MIT公开模型目录固定原始JSON，实际SHA256 80a19a3c78de1a7b0eb0a72e13e44df21180f0681d6cdf1dbd4c4c345fae3fb6 |
| `modules/model-gateway/internal/catalog/bundled/source.json` | 手写 | 真实URL/取得时间/ETag/内容版本/许可溯源 |
| `modules/model-gateway/internal/catalog/bundled/LICENSE` | 手写 | models.dev原MIT许可与版权 |
| `modules/model-gateway/internal/invocation/probe.go` | 手写 | 后台单次真实连接探测及安全结果投影 |
| `modules/model-gateway/migrations/013_admin_probe.sql` | 手写 | 独立probe日限默认0不限及真实探测原调用来源，不修改历史付费事实 |
| `modules/model-gateway/internal/runtime/restore_projection.go` | 手写 | 真实首次创建来源/完整typed SQL快照与原费用target认证投影 |
| `modules/model-gateway/migrations/014_snapshot_authority.sql` | 手写 | 真实首次创建来源/完整typed SQL快照与原费用target认证投影 |
| `modules/model-gateway/migrations/015_snapshot_watermark.sql` | 手写 | 真实首次创建来源/完整typed SQL快照与原费用target认证投影 |
| `modules/model-gateway/migrations/016_usage_observations.sql` | 手写 | 真实安全ledger+invocation/attempt/reservation同一全局变化时间线上界及不可变观察闭包 |
| `modules/model-gateway/internal/admin/observations.go` | 手写 | 无ledger调用/独立attempt/held安全完整观察，和posted共享时间线上界 |
| `modules/model-gateway/web/src/index.tsx` | UI | 模型接入：供应方与密钥、模型、档位、任务分配，一键生成档位与分配 |
| `modules/model-gateway/internal/catalog/discover.go` | 手写 | 参照 opencode 的供应方接入：内置 models.dev 目录里的供应方列表，以及按已存密钥拉取供应方当前可用模型并用目录补全。 |
| `modules/model-gateway/migrations/018_retire_decision_tasks.sql` | 手写 | 退役专用决策任务并把语言建议迁移到快速文本档 |
