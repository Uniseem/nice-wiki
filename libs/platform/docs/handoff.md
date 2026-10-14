# platform 交接（R1-00）

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
- 根 STATUS/plan 由集成会话汇总；本模块交付提交后 progress.json 逐项翻为 implemented/verified。
- 下一项任务：合并R1-00后按 plan.md 认领 R1-01（identity），其 main/BuildApplication装配全部经本库。
