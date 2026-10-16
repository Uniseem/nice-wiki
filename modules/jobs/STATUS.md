# 持续目标第二轮公共前置：延迟任务创建（VPS检查通过）

集成会话独占本次jobs公共API配合，基线471c036，新增可选notBefore：显式UTC时刻纳入去重且写既有not_before；省略保持原hash和首次立即可领。补真实createJob HTTP映射并限生成任务owner调用方，完整HTTP/存储/control/后台启动仍blocked。仅传输创建RPC受op.deadline限制，任务执行期限由owner领取时重建。无新迁移/依赖/路径，旧queued任务和旧无延迟去重回执不改写。

首次VPS gen成功但HTTP输入引用误用被裁剪的业务DTO而编译失败，已改用实际CreateJobRequestBody并同步其可选字段，待完整重跑；不能新增implemented/verified；根统一VPS docs/gen/build/check，最终VPS已完整通过（872/1332/379/2297），源码SHA随下一记录提交回填。未新增测试/样例，未运行真实接口/模型、服务或部署。

---

# 任务与事件当前进度

更新：2026-10-15。持续实施第五轮StreamProgress，分支`codex/jobs-progress-stream-r1`，基线`7a147fc0ded784d16792d5d82e9b8277efd4be6c`；独占本域，原管理交付分支保留。源码、合同与VPS生成产物已交付，本轮完整10项函数及对应完整文件verified；完整服务仍blocked，业务验收未执行。

- StreamProgress输出直接ProgressEvent JSON，SSE event固定progress、id为seq十进制；内部JSONEventStream句柄，公共ServeJSONEventStream负责15秒注释心跳、单帧背压、30秒写截止与关闭，本域不复制pump。
- 每次打开/继续读取从持久job owner/type/epoch核当前具体业务权限，网络核权无Store/SQL锁；每批最多64行，内存上限64，无额外异步队列，actor/site最多10流且取消/断连幂等归还。
- 推荐afterSequence，兼容lastEventId和Last-Event-ID，显式同给必须数值一致、超int64或冲突400；全部缺省0。0首次读取当前7天可重放项；正序号不越未来/缺失/过期段。保留原HTTP ctx/deadline不延长委托，流开始后权限/epoch/截止变化只关闭，不追加Problem；追平终局返回EOF。
- 0007只追加nullable created_at，旧行按同job/seq/epoch及安全payload全部持久来源唯一一致时刻回填；缺失/矛盾保留NULL失败关闭。AppendProgress行和outbox使用同一次platform.Now；读取RFC3339Nano后按实际时刻比较，不做可变小数TEXT边界比较。
- OpenDatabase先核已应用版本合法，再唯一EnsureSchema并执行迁移，fresh库可回填outbox；BuildApplication实际打开/关闭库并组合管理+进度映射交完整工厂。其他handler、完整Store/control/owner后台、定时与ephemeral仍未齐，整体启动继续blocked。
- 新实际路径仅`internal/http_progress.go`（已有登记）和批准的`migrations/0007_progress_retention.sql`。未本机Go/gofmt/gen/vet/测试或SQL临时验证，未新增测试/mock/fixture/样例，未运行真实业务API/模型/服务/部署或push。
- 源码提交：`adcc1e85ff6b5f5d949927933f0497af608670d8`。2026-10-15 第五轮集成会话VPS合成快照：完整make build（14个现有Go模块-mod=readonly build/vet/gofmt）通过；最后CONFLICT合同补登记后make docs/gen和完整make check通过（859文件/1117函数/379操作/2261类型）。Go/迁移未再变，生成与格式回传无冲突；只代表本阶段源码/格式/结构检查，未执行SQL业务验证、业务测试、真实接口/模型、服务、部署或push。

以下为前轮历史，当前以后续页首与台账为准。

更新：2026-10-15。持续实施第四轮任务管理组，基线`de21a2bcb5e1a8b25e41ec84ba3734fc163b5ab1`，分支`codex/jobs-management-r1`；独占本域，旧停止要求已被本轮持续授权撤销。源码与合同已交付，本轮完整18项（新17函数及既有授权目录复用）verified；完整服务仍blocked，业务验收未执行。

- 已实现ListJobs/ListDeliveries安全摘要、当前具体业务权限过滤、稳定ID分页及平台签名游标；游标4096上限与平台一致，绑定actor/site/session及修订、caller/epoch和完整筛选。owner/producer只缩窄，hasMore和游标只看有权条目，不读任务输入正文或原事件payload。
- 新增本域JobSummary替代错误的AcceptedJob恒queued；状态为真实持久值，rootOperationId未知为null。当前最终失败持久态为dead-letter；兼容failed筛选返回空，不增加无生产分支的状态。
- RetryJob先回原操作回执，不因后来已执行而再次owner确认/排队；首次操作仅dead-letter，锁外owner completed复用结果收敛succeeded，safeToRetry才保留已知原根和已有阶段重排。unknown/pending/nil、orphan、取消意图失败关闭。完整ObjectRef矛盾拒绝，paid ID合并保留，fence提升、revision CAS、结果/outbox/回执同事务。
- 新实际路径：`internal/queries.go`（既有登记）、`migrations/0006_job_management.sql`（本轮批准只追加retry回执）；复用query.go权限和http_events.go映射，BuildApplication实际调用公共完整工厂，其他handler/store/control/owner适配未齐仍blocked。
- 源码提交：`ab4a738b7ff69ed55d7feed39d9ba081d13b519d`。2026-10-15集成会话protocol第三次VPS合成快照：make docs/gen、14个现有Go模块readonly build/vet/gofmt及完整make check全部通过（858文件/1105函数/379操作/2253类型），包含本域管理Go和0006迁移，排除首页消费者HomeWIP；生成/格式产物已回传。仅源码/格式/结构检查，未执行SQL业务验证、测试、真实接口/模型或服务。
- 完整启动、真实owner恢复API适配、control屏障/后台/定时、ephemeral安全投影仍未完成；本轮不伪装可用服务。未本机运行Go/gofmt/gen/测试或临时验证，未新增测试/样例/fixture/mock，未运行真实接口、模型、服务、部署或push。

以下为历史交付事实，下一项以本轮为准。

## 第二轮最终集成与停止范围

- 事件目录/schema/接收/投递/确认/重放和真实HTTP核心通过；完整启动/屏障/定时/ephemeral/实际owner适配仍未完成。
- 实际main集成后的VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过，统计855文件/1012函数/374操作/2211类型；台账SHA已映射主线提交。
- 用户要求第二轮结束push并停止持续目标，不做第三轮；后续实施需要新的请求。历史“下一步”仅作将来交接，不能当继续授权。
- 未新增或运行业务测试、真实接口/模型、服务、镜像或部署；检查不等于行为验收。


第二轮最终交付（2026-10-15）：事件核心、确认水位、死信重放、四个真实HTTP映射及Relay写窗口已通过最终合成VPS `make docs/gen`、14模块只读依赖build/vet/gofmt与完整make check（855文件/1012函数/374操作/2211类型）。核心源码`0e0a061c3684a0723a8657c73ef258f9a803804b`，必要安全窗口`b146b4effe0d4f7be0dd5827cc9ee6f2fa37be71`，最终格式/生成产物`5df9b3cf0d1d2e1e4b87862053275301fc64bde8`；本轮完整交付条目按对应检查证据verified。

整体启动、完整control屏障/后台/定时、ListDeliveries、portal ephemeral安全投影及实际owner恢复适配仍未完成，业务验收与部署未执行。用户要求第二轮收尾后由集成会话普通推送并清空目标，本代理冻结交付、不自行push、不启动第三轮。以下“待VPS”等记录保留为阶段历史，以本段和progress.json为准。


持续实施第二轮事件包（2026-10-15）：分支`codex/jobs-event-delivery-r1`，基线`468986fc34e1c90b6b5ca575539fdd2b9748cd06`，独占`modules/jobs/`。事件发布、durable增量订阅、确认/连续水位、到期退避死信与具体功能授权重放源码已准备，待统一VPS生成/编译/结构检查，全部本轮条目in_progress。合成VPS首轮14模块make build已通过并回写生成/格式/依赖产物；载荷参数登记、响应helper签名和旧事件producer完整性三项最小修复等待VPS复跑，未提前升级状态。

- ValidateEnvelope按根生成EventTypes核唯一producer/version/严格payloadSchema，校验器只用platform。PublishEvent规范完整信封hash，事件/consumer投递/稳定seq/操作回执同事务；相同eventId不重建delivery。
- SubscribeEvents返回真实platform.EventStream，每consumer最多2流，逐事件短事务领取30秒确认租约，同producer/subject前序未ack含死信阻塞后序；Last-Event-ID不越过未ack，关闭归还连接槽。
- Ack绑定已认证consumer与当前epoch，只有实际发出的投递可确认，连续确认水位不能跨未ack。到期按1/2/4秒退避上限300秒，过期次数超过10进死信。ReplayDelivery按持久deliveryId和生产方具体业务权限恢复原event/consumer，不能覆盖已ack或活动投递。
- 发布和ack可在有效control截止前draining收敛；维护/屏障停止新投递、重排和重放。jobs自有outbox补RelayOutbox薄入口，完全复用platform.RunRelay/SQL Store。
- portal ephemeral安全投影未收口，本轮明确UNSUPPORTED_CAPABILITY，不广播原payload；公共HTTP/SSE装配、实际owner恢复适配、完整屏障与定时仍未完成。未新增测试/样例/临时业务程序，未本机编译/格式/生成/测试，未运行真实接口/服务/模型、未部署或push。

以下为第一轮已合并历史事实：

最新统一集成事实（2026-10-15）：终局、取消、有限重试、持久任务权限与orphan框架通过；owner恢复适配、事件/定时/屏障和HTTP仍未完成。
VPS顺序集成后make docs/gen、14模块make build和完整make check通过；生成最终零变化。主线实际源码提交：7566a0c、167aec4，逐项以progress.json为准。未执行业务验收或部署。
以下旧说明作为历史记录，当前状态以前述事实和台账为准。


统一复核追加修复（2026-10-15，VPS通过、待最终集成）：管理终点从持久任务owner/type映射具体managementPermission后请求identity，job.manage仅作入口粗能力；外部授权请求不持有Store锁，事务内复核任务作用域。持久cancel_requested优先于临时失败重试：未明确安全的失败进入orphan，CANCELLED确认后才终止；claim/start/requeue/heartbeat共同保留取消意图。本轮隔离VPS make docs/gen及14模块make build（只读依赖build/vet/gofmt）全部通过，make check完整通过（850文件/943函数/372操作/2186类型）；修复源码`92f4fe029c58baeace5657b9c7762dc9cb5ccd19`已回填，交付条目记implemented，verified等待根最终集成。

更新：2026-10-15。R1-03终局与回收包源码及VPS编译门槛已完成，源码提交`5eabae7a7e9d7f1fbd74ea3bf74b3015a4d233ef`，交付条目记implemented；完整结构检查待集成收口。分支`codex/jobs-execution-complete-r1`，基线`325f56d7b2f6c5567c16d7e21f66d5a88a97e342`，独占`modules/jobs/`。

- 新增CompleteJob、FailJob、RequestCancel、ConfirmCancelled、ReadJob、固定重试与到期重排；结果/状态/租约/outbox/操作与终局回执同事务，worker/owner/epoch/fence及control截止限制保持。已付费引用合并保留；无结果任务不生成空ObjectRef。
- ReapExpiredLeases先失效旧fence并隔离orphan，再经可信OwnerRecoveryChecker确认completed或safeToRetry，pending/unknown保持orphan；实际owner版本化适配未接通，不能视为恢复业务可用。维护/control屏障期间暂停回收与重排。
- CreateJob改读可信runtime/control屏障；新增任务保存原rootOperationId并在领取/恢复时复用，历史根未知任务不领取，由回收事务隔离，禁止猜根后自动重跑付费阶段。
- 新增5个源码与2个只追加迁移；生成物由集成会话在VPS统一刷新；VPS首轮make docs/gen成功、14模块make build通过（只读依赖build/vet/gofmt，BUILD_STATUS=0），make check仅公共可达性与framework新增文件登记待集成收口。未本机编译/格式/vet/运行SQL或临时验证程序，未新增测试/样例，未运行真实接口、模型、服务、部署或push。

以下为已合并历史记录：

最新集成结果：隔离VPS的14模块make build与完整make check通过；本轮已完成条目转verified，实际主分支SHA见progress.json。部分入口/用例仍未实现，业务验收和部署未开始。下列本机与待登记说明保留为交付时历史事实。

更新：2026-10-15。阶段：R1-03租约fencing与可选心跳进度已实现并通过编译门槛；完整HTTP/启动装配、重试回收、事件投递/死信、业务验收与部署尚未完成。

- 本轮独占 `modules/jobs/`，分支 `codex/jobs-lease-r1`，基线 `00dd199239aba487bceb5227aac52f27ce949d74`；实现提交 `e6e0d81226358a11a5764ee9ebb43a6706810e9e`，未push。
- 已实现 `ClaimJob`、`StartJob`、`HeartbeatJob`、`CheckLease`：owner/epoch/worker/fence隔离、到期拒绝、30秒租约、owner在途上限8、批次默认10及最多10、持久fence计数、领取/心跳幂等回执、状态outbox。
- `AppendProgress`随租约包实现为同事务参考函数；心跳可选报告安全阶段代码与计数，checkpointKey/inputHash去重，seq单增且同phase不能倒退。续租、进度、事件和回执一起提交或回滚。
- `LeaseRuntimeProvider`要求装配层提供可信runtime与control屏障安全点/截止；缺少钩子、维护缺有效截止时失败关闭。租约组不继续使用错误放在active库的旧barriers表；全量屏障布局留后续专包收口。
- 本机Go1.27.1根 `make build` 通过（14模块只读依赖build/vet/gofmt），`make docs`和`make privacy`通过。`make check`仅有新增3个实际文件待公共framework登记，其余合同、类型、引用、可达性、权限和隐私检查通过；台账暂记implemented，待集成完整检查后升级verified。
- OpenAPI补CheckLease必填workerId/fencingToken查询参数、claim可省略limit及heartbeat可选progress；生成目录未手改，由集成会话统一生成。平台JobsClient的workerId支持由集成会话公共提交提供。
- 未新增测试/样例/临时验证程序，未运行真实接口、业务进程、模型、联调或部署；完整BuildApplication仍按未装配依赖返回不可用，业务验收未登记完成。

## 既有工作记录


更新：2026-10-15。阶段：R1-05 `CreateJob` 任务提交持久化已实现；租约、事件投递/死信、完整装配、业务验收与部署未开始。

- 2026-10-15 集成复核修复：`ValidateJobSpec` 白名单改读 make gen 生成的 `internal/gen/job_types.gen.go`（与 config/job-types.json 同源）。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次会话按 handoff 下一项实现 `jobs.CreateJob`；实际分支为 `codex/analytics-r1-storage-contract-audit`，基线 `32afbc58bd24b8d03230c920a3b105374c783acb`。
- 实现提交为 `588de8fa991340df7dcf019f8c11849f85e1b796`（`jobs：实现任务提交持久化`），仅包含 `modules/jobs/` 内的代码、迁移和依赖锁定变更，未 push。
- `internal/queue.go` 已实现：复用 `ValidateJobSpec` 的 owner/type/ObjectRef/epoch 校验，补充 operationId 与 dedupeKey UTF-8/8192字符约束；按 `(owner, dataEpoch, dedupeKey)` 查询，同 inputHash 复用原任务，不同输入返回 `CONFLICT`。
- 同一 SQLite 写事务插入 queued 任务和 `jobs.job-state-changed.v1` 初始 outbox 事件；事件载荷只含任务引用、状态、修订和 owner，不含对象正文、提问、密钥或 IP。排空屏障拒绝新任务并返回 `LOCKED`。
- `migrations/0001_domain.sql` 已建立 jobs、leases、progress、job_results、events、deliveries、schedules、barriers 和 schema_migrations；`OpenDatabase` 使用 WAL/foreign_keys/busy_timeout，并调用 platform outbox 建表。数据库入口尚未接入完整 BuildApplication，避免在缺少全部 handler 时伪造 ready。
- `modernc.org/sqlite v1.59.0` 已登记到本域 go.mod/go.sum；Go 工具链不可用，未完成 Go 编译、gofmt、vet 或运行验收。已通过 `git diff --cached --check`、隐私检查，并用 Python sqlite3 内存数据库执行迁移插入校验。
- 完整 `python scripts/check_docs.py` 仍因公共 `docs/structure/framework.json` 未登记本域新增实际文件，以及范围外 site-config 脏改动而失败；公共文件按本工作包范围未修改。业务行为验收仍为0，未新增测试/样例、真实接口、真实模型或部署。

前置启动骨架、`ValidateJobSpec`、`Serve`、`Shutdown`、`Health`、`Ready` 的既有状态与未验证限制保持不变。下一项优先实现 `ClaimJob`/租约 fencing，之后再接入完整存储与 handler 装配。

公共契约、根状态、scripts、libs、deploy 拓扑和其他模块均未修改；并行工作只改本域，公共资料交集成会话。

本次最终检查：2026-10-16持续目标第二轮公共前置：VPS linux/amd64 Go1.25.5、GOMAXPROCS=8，make docs/gen、14模块readonly build/vet/gofmt和完整make check通过（872/1332/379/2297）。HTTP用实际RequestBody，dataEpoch仅可信上下文；公共错误分类有真实FromError调用。无新测试/业务验收/真实接口模型/服务部署。
