# 持续目标第二轮公共IO错误工具（VPS检查通过）

集成会话新增NormalizeIOError，保留公共错误及统一ctx取消/截止，未知错误只用可信代码选择的安全说明，不传播原始存储错误。problem.FromError复用它，保持未知INTERNAL兜底；新域不再复制context错误分类。无新路径/外部依赖；首次完整编译通过但新工具因既有FromError未登记而结构检查失败，补真实FromError及jobs响应调用边界后待完整重跑，再回填源码SHA；不代表业务验收。

# 两轮并行第二轮：可选主密钥引用（2026-10-16，已检查）

基线53acce862467dbe40b829d1984519d78ccac5608，集成会话独占公共源码。ServiceConfig新增可选masterKeyRef，NW_MASTER_KEY_REF按现有env优先级覆盖文件；非空只要求绝对路径。公共加载不打开主密钥，实际32字节读取、保密和AES-GCM由model-gateway凭据用例负责；免费任务配置读取不因主密钥缺失失效。无新路径/函数/依赖，不新增测试或样例。源码993904524e4d7a792a6c84a80606de6215f7ace5；VPS docs/gen、14模块readonly build/vet/gofmt及完整check通过（865/1258/379/2277），对应台账准确引用该SHA。下一项仅由集成会话核对model-gateway实际凭据调用方并完成第二轮收尾。

# platform 交接（R1-00）

## 第五轮JSON SSE传输（已有检查通过）

ProgressEvent保持业务JSON直出；新增SSEFrame/JSONEventStream与ServeJSONEventStream，旧EventEnvelope流薄适配同一公共pump。单条背压、15秒心跳、30秒写截止、data16MiB和取消/关闭集中实现，域端仍负责当前权限/epoch、7天持久续传与每主体流数量。源码5894bfb，完整Home合成快照和实际集成树88210ba均通过VPS docs/gen、14模块readonly build/vet/gofmt及完整check（858/1109/379/2256），生成零变化；已完成公共条目转verified，不以此宣称尚在开发的jobs进度流已实现。

## 第四轮持久恢复公共机制（合成快照通过）

新增CanonicalJSON、NewWriteContext、RunRecoveryLoop；outbox安全写窗口复用公共实现，清理与领取分别重取窗口。进程模板支持可选BackgroundApplication及统一取消/Drain/后台等待/关闭资源，恢复哨兵统一返回75。输入域保留每次RPC后的epoch/屏障核对，不由循环发起付费重算；公开地址边界拒绝解码后连续斜杠，避免生成协议相对链接。

2026-10-15冻结合成快照已在隔离VPS通过make docs/gen、14个现有Go模块readonly build/vet/gofmt与完整make check（858文件/1105函数/379操作/2253类型）。快照含jobs实际管理代码、首页和公开链接合同，不含尚未完成的Home Go；公共源码实际提交9ff93ae；其独立实际集成树857文件/1091函数/379操作/2244类型通过同样检查，随后jobs实际树858/1105/379/2253再次通过，生成均零变化，相关完整公共条目已回填该源码与证据。不称行为验收，不push。


## 第三轮恢复实施（2026-10-15）

用户已在第二轮停止后重新授权继续。独占公共源码与合同，补ServiceConfig可信代理网段、PrivateNetworkScopes；登录业务规则仍由identity/guard负责。公共源码提交918dd6369ea7a33bf9296949c8b5441a08d1e88f；合成快照VPS docs/gen、14模块readonly build/vet/gofmt与完整check通过（856文件/1047函数/374操作/2222类型）。portal调用方完成闭环；scripts/gen_code.py依据portal OpenAPI元数据只为已有浏览器TS副本合成四个安全认证操作，内部Go和真实路由保持其各自来源。未新增测试或真实调用，不push。


## 第二轮收尾复核（2026-10-15）

- 最终合成快照VPS docs/gen、14模块readonly build/vet/gofmt和完整make check通过，统计855文件/1012函数/374操作/2211类型。
- 补RelayConfig必需WriteWindow：未知权威或安全窗口关闭不写active outbox，逐条网络调用后重新核窗口再确认/重试；保留父ctx更早截止与费用根。
- 新增ValidatePrivatePath供身份control权威启动定位，拒目录链链接、特殊文件与硬链接；仅首次创建允许缺失尾部。
- 客户端查询参数、SSE Last-Event-ID/afterSequence/default limit按OpenAPI生成；SSE帧预算16MiB，不沿用64KiB scanner默认导致合法事件截断。Markdown链接检查忽略代码围栏/inline code伪链接，隐私扫描仍查全文。
- 用户要求第二轮结束push并清空目标：仅收尾，不开始第三轮；guard实际登录保护、portal peer/cookie、owner恢复和完整handler等仍按未完成交接。


## 持续实施第二轮进行中

- 新增PrivateOperationDigest、NewPayloadValidator、ServeEventStream、NormalizePublicURL、NewServiceHandler和窄EventPublisher适配；事件schema/生产方/订阅/具体权限目录由gen生成。
- HTTP入口完整handler与固定caller策略在启动核对；一次服务身份/委托/epoch/期限与严格schema，用户权限回调当前identity权威，健康/就绪匿名。
- 登录网络作用域只允许portal→identity服务签名声明，Go op内部不序列化；受信peer派生与guard begin/outcome接口仍在本轮公共收口，不得用浏览器头兜底。
- 公共快照已在VPS完成make docs/gen、14模块build/vet/gofmt与完整make check（850文件/955函数/374操作/2199类型）；mod/sum均由VPS Go生成，生成回传零变化。调用方工作树仍待合成复核，条目保持in_progress；未执行业务验收，不push。


## 持续实施第一轮最终集成事实（2026-10-15）

- 会话签名、委托、可信配置和worker控制/结果链已通过统一复核；调用服务完整装配仍未完成。
- 主线实际源码提交：dad2dce；模块分支交付经cherry-pick后SHA已在progress.json映射，后文原分支SHA保留用于溯源。
- 三域逐个合入后各自VPS make docs/gen、14模块readonly build/vet/gofmt与完整make check通过；最终850文件/943函数/372操作/2186类型，生成回传零变化。现有设计检查补核managementPermission与jobManagementGrants集合一致。
- 全部编译/格式/vet/生成检查均在VPS；没有新增测试，也没有运行业务测试、SQL行为验收、服务/真实接口/模型、镜像、部署或push。部分文件与入口继续in_progress/blocked，不把检查当业务验收。
- 下一项以本节和根plan最新工作包为准，后文“等待VPS”等旧说明为过程历史。


## 持续实施第一轮公共集成（待合main）

- 统一会话访问签名/独立秘密、专用委托签名、会话版本、可信公共地址与任务完整结果写回。worker控制操作使用独立幂等键，费用根与executor原操作不变；取消确认由owner负责，权威未知暂停领取。
- 跨服务委托先验本跳服务断言，portal首跳核原正文，后续保留原proof并由服务断言绑定衍生正文；仍须即时核权和资源授权。
- 任务目录新增managementPermission；identity派生job.manage粗能力，jobs按持久owner/type核具体业务权限。
- 来源分支codex/continuous-r1-cycle01、基线325f56d7b2f6c5567c16d7e21f66d5a88a97e342。最后修订尚待VPS，台账新增函数保持in_progress；不新增测试、不运行业务接口/模型/部署，不push。
- 下一项：统一VPS复核与逐模块合并后，接入已实现用例的HTTP绑定；先核各域OpenAPI和平台请求校验，不增加影子协议。


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

本次最终检查：2026-10-16持续目标第二轮公共前置：VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、14模块readonly build/vet/gofmt和完整make check通过（872/1332/379/2297）。HTTP用实际RequestBody，dataEpoch仅可信上下文；公共错误分类有真实FromError调用。无新测试/业务验收/真实接口模型/服务部署。
