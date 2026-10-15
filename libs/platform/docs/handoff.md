# platform 交接（R1-00）

## 本轮统一复核结果（2026-10-15）

- 三个模块在独立worktree交付，集成分支逐个审查新增逻辑与合同；最终隔离VPS linux/amd64 go1.25.5、GOMAXPROCS=8 的14模块make build和完整make check通过。编译、结构检查不代表业务验收。
- make docs无漂移；make gen第二次运行零变化，核对发生在用户追加VPS专用要求之前；后续所有编译/测试只在VPS，VPS不可用则blocked，不自行本机回退。本轮没有新增或运行业务测试、真实接口、模型、服务或部署。
- 根framework已登记本轮10个实际文件，生成器与消费者统一刷新；本轮已完成函数和完整文件按证据转verified，部分文件与入口仍如实in_progress/blocked。主分支实际实现SHA以progress.json为准。

## 本轮公共支持（2026-10-15，三个工作包集成）

- `serviceauth.SignCursor/VerifyCursor` 统一 Ed25519 分页游标签名；专用 NW-CURSOR 类型，scope 由调用方绑定模块、查询、站点、主体和 dataEpoch，游标不替代授权。密钥复用 LoadSigningKey，不在各域另造签名工具。
- `JobsClient.CheckLease` 同时接收 jobId、workerId 和 fencingToken；worker 写回前原样携带当前 worker 身份。
- 当前在集成分支实现，平台模块本地 readonly build/vet/gofmt 通过；新增条目已随三个模块交付按隔离VPS全量make build与可达性检查转verified。行为验收未执行。

## 公共机制补充（2026-10-15，集成会话，main）

- `lifecycle.NewSiteConfigClient(cfg, key)`（runtime.go）：各域共用的 site-config 运行状态客户端，以本服务断言调用 GET /v1/runtime-state 并解开 `{"state": RuntimeState}` 外层；10 个模块的 `NewRuntimeClient` 已改为调用它，删除各自的 runtimeAuthority 适配。
- `apivalidate.NewRequestValidator(spec)`（新文件 apivalidate/validator.go）：按 ADR0005 用 kin-openapi openapi3filter 基于各域 `gen.GetSwagger()` 内嵌规格在 HTTP 入口校验请求（ADR0006 边界一次校验），失败返回 VALIDATION Problem，issues 只给位置与 schema 关键字。各域 HTTP 绑定工作包在 transport 装配；目前尚无模块装配。
- 平台库因此新增 kin-openapi v0.133.0 依赖（ADR0005 已批准），不再是“仅标准库”；gorilla/mux 等只是 kin-openapi 的测试依赖，只记 go.sum 校验和。
- 合同：implementation.json 增 1 文件 2 函数，types.json 增内部类型 OpenAPIDocument、HTTPMiddleware；15 域 BuildApplication 依赖登记 `platform.NewRequestValidator`，使用运行状态客户端的 10 域另登记 `platform.NewSiteConfigClient`。
- 台账：新增文件与两个函数已回填提交 `051c3834de1fe339dd8e3b9c1b40fc7e6721ccd8` 并按检查证据转为 verified。
- 验证：`make build`（14 个 Go 模块 -mod=readonly build/vet/gofmt）与 make check 通过；按 ADR0006 未写测试，中间件对各域规格的实际路由构建与校验行为要到首个 HTTP 绑定工作包启动服务时才验证。

- 分支：codex/platform-r1-00；基线提交 d7cf792431d8fe1c4669ce05957ff288c3e182aa；实现提交 fa70c657322cfabbb575282f3436f4605f31d69d；台账交付记录紧随其后单独提交。
- 独占范围：libs/platform/ 全部、scripts/gen_code.py、Makefile的gen目标、15域internal/gen与web|ui生成物、jobs/portal生成配置；跨域合同修订仅jobs ClaimedJob（attempt/dataEpoch）与各域clients.gen.go职责措辞（合并时同步）。
- 未提交文件：无。

## 本次完成

- libs/platform 全部17个约束函数按合同实现：serviceauth（断言签发/验证、委托验证、签名传输层）、opctx、problem、outbox（登记+转发循环）、inbox（去重+订阅循环）、worker（领取循环）、modelclient、lifecycle（配置/运行状态/恢复换目录/进程模板）、cmd/nw-keys。
- 新增 libs/platform/platform.go：types.json全部类型、公共schema映射（OperationContext/Problem/EventEnvelope/AuthContext/ObjectRef/JobExecutionInput等）与 Tx/OutboxStore/JobsClient/GatewayClient/SiteConfigClient/EventStream/ModelEventStream/EventHandlerRegistry/JobExecutorRegistry/ServiceSpec 接口。平台库仅依赖标准库。
- scripts/gen_code.py + make gen：15个模块 internal/gen/openapi.gen.go（oapi-codegen v2.5.1，3.1规格确定性降级为3.0形态后生成）、clients.gen.go（本仓库模板：多生产方合并单文件、自动服务断言、SSE适配platform流接口）、web|ui/src/gen/api.gen.ts（openapi-typescript 7.10.1）、jobs config/job-types.json与event-subscriptions.json、portal config/routes.json。重复运行结果一致（已验证）；全部15域生成物在临时模块编译通过。
- nw-keys 冒烟通过：密钥生成0600、重复运行不覆盖、--rotate预留、公钥表确定性排序。

## 实现口径（供后续域对齐）

- 服务断言：JWS compact，header {alg:EdDSA,kid,typ:JWT}，载荷为 AssertionClaims（时间RFC3339）；时钟偏差统一±30秒；jti重放缓存在进程内。私钥缓存按路径+mtime。
- OperationContext头：X-Operation-Id/X-Root-Operation-Id/X-Cost-Class/X-Deadline/X-Cancellation-Id/X-Data-Epoch/X-Request-Id；X-Portal-Proof 经 context 向下游转发。缺省deadline=30秒。dataEpoch不一致→VERSION_MISMATCH。
- outbox/inbox 表（nw_outbox/nw_inbox）DDL 由本库 SchemaSQL 定义、启动时 EnsureSchema 创建，不进SQL迁移；唯一约束冲突以错误串识别（SQLite驱动通用）。
- RunRelay/RunConsumer/RunWorkerLoop 通过 config 里的 runtime 钩子读取RuntimeView：屏障与恢复阶段暂停领取；旧代事件确认跳过、目标代不确认；换代后丢弃旧代outbox行。
- CompleteJob 的 resultRef 传空 ObjectRef（jobs需接受无结果任务）；Plan：jobs实施时对齐。
- nw-keys 的 services.json 子集 {version,services:[{slug,secretsDir}]}；portal委托密钥默认1对、上限4；旧公钥在--rotate后保留至手动清理。

## 检查证据（本地）

- cd libs/platform && gofmt -l . 为空；go vet ./... 通过；go build ./... 通过。
- make gen 连续两次运行 git 状态无新增差异；gofmt -l modules/ 为空。
- 15个模块生成物在临时go.mod（replace到本库）下 go build 全部通过。
- make check 通过（设计结构+生成文档一致性+隐私）。
- 未在VPS复跑；未运行业务测试（无授权新增测试）。

## 遗留与下一步

- deploy 会话需按上述口径创建 deploy/config/services.json；jobs 会话对齐 ClaimedJob.attempt/dataEpoch 与 CompleteJob.resultRef 可空口径。
- 各域 progress.json 中生成文件保持 planned（其模块仍处design阶段），生成物由make gen随时重建；模块自身实现包交付时一并翻状态。
- 署名约定：本次实现由GLM（ZCode）完成；2026-10-14用户确认GLM代用地址 noreply@z.ai，已登记AGENTS与隐私扫描白名单，后续提交直接使用。
- 下一项任务：合并R1-00后按 plan.md 认领 R1-01（identity），其 main/BuildApplication装配全部经本库。
