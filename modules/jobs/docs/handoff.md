# 任务与事件会话交接

## 第五轮任务进度流（已交付）

更新：2026-10-15。持续实施第五轮StreamProgress，分支`codex/jobs-progress-stream-r1`，基线`7a147fc0ded784d16792d5d82e9b8277efd4be6c`；独占本域，原管理交付分支保留。源码、合同与VPS生成产物已交付，本轮完整10项函数及对应完整文件verified；完整服务仍blocked，业务验收未执行。

- StreamProgress输出直接ProgressEvent JSON，SSE event固定progress、id为seq十进制；内部JSONEventStream句柄，公共ServeJSONEventStream负责15秒注释心跳、单帧背压、30秒写截止与关闭，本域不复制pump。
- 每次打开/继续读取从持久job owner/type/epoch核当前具体业务权限，网络核权无Store/SQL锁；每批最多64行，内存上限64，无额外异步队列，actor/site最多10流且取消/断连幂等归还。
- 推荐afterSequence，兼容lastEventId和Last-Event-ID，显式同给必须数值一致、超int64或冲突400；全部缺省0。0首次读取当前7天可重放项；正序号不越未来/缺失/过期段。保留原HTTP ctx/deadline不延长委托，流开始后权限/epoch/截止变化只关闭，不追加Problem；追平终局返回EOF。
- 0007只追加nullable created_at，旧行按同job/seq/epoch及安全payload全部持久来源唯一一致时刻回填；缺失/矛盾保留NULL失败关闭。AppendProgress行和outbox使用同一次platform.Now；读取RFC3339Nano后按实际时刻比较，不做可变小数TEXT边界比较。
- OpenDatabase先核已应用版本合法，再唯一EnsureSchema并执行迁移，fresh库可回填outbox；BuildApplication实际打开/关闭库并组合管理+进度映射交完整工厂。其他handler、完整Store/control/owner后台、定时与ephemeral仍未齐，整体启动继续blocked。
- 新实际路径仅`internal/http_progress.go`（已有登记）和批准的`migrations/0007_progress_retention.sql`。未本机Go/gofmt/gen/vet/测试或SQL临时验证，未新增测试/mock/fixture/样例，未运行真实业务API/模型/服务/部署或push。
- 源码提交：`adcc1e85ff6b5f5d949927933f0497af608670d8`。2026-10-15 第五轮集成会话VPS合成快照：完整make build（14个现有Go模块-mod=readonly build/vet/gofmt）通过；最后CONFLICT合同补登记后make docs/gen和完整make check通过（859文件/1117函数/379操作/2261类型）。Go/迁移未再变，生成与格式回传无冲突；只代表本阶段源码/格式/结构检查，未执行SQL业务验证、业务测试、真实接口/模型、服务、部署或push。

本轮依赖根稳定公共JSON SSE API，VPS组合由root取公共源、从本树取jobs全域；生成目录未手改，最终产物已回传并纳入上列源码提交，台账精确引用该SHA。完整10项为StreamProgress、progressStream.Next、progressStream.Close、readProgressFrame、NewProgressHandlers、bindProgressCursor、AppendProgress、OpenDatabase、runMigrations、Store.Close；BuildApplication继续blocked。7天为读取重放范围，本包不物理删除progress；将来Purge必须保存单任务seq水位，不能删除全部后由MAX(seq)复用旧序号。

本包冻结，不继续扩jobs范围。下一项持续实施工作包为model-gateway免费任务可用性读取，由集成会话另行分派；本域完整handler、Store/runtime-control、实际owner适配、后台/定时及ephemeral继续按各自合同接续，不列为本轮完成。


## 第四轮任务管理组（已交付）

更新：2026-10-15。持续实施第四轮任务管理组，基线`de21a2bcb5e1a8b25e41ec84ba3734fc163b5ab1`，分支`codex/jobs-management-r1`；独占本域，旧停止要求已被本轮持续授权撤销。源码与合同已交付，本轮完整18项（新17函数及既有授权目录复用）verified；完整服务仍blocked，业务验收未执行。

- 已实现ListJobs/ListDeliveries安全摘要、当前具体业务权限过滤、稳定ID分页及平台签名游标；游标4096上限与平台一致，绑定actor/site/session及修订、caller/epoch和完整筛选。owner/producer只缩窄，hasMore和游标只看有权条目，不读任务输入正文或原事件payload。
- 新增本域JobSummary替代错误的AcceptedJob恒queued；状态为真实持久值，rootOperationId未知为null。当前最终失败持久态为dead-letter；兼容failed筛选返回空，不增加无生产分支的状态。
- RetryJob先回原操作回执，不因后来已执行而再次owner确认/排队；首次操作仅dead-letter，锁外owner completed复用结果收敛succeeded，safeToRetry才保留已知原根和已有阶段重排。unknown/pending/nil、orphan、取消意图失败关闭。完整ObjectRef矛盾拒绝，paid ID合并保留，fence提升、revision CAS、结果/outbox/回执同事务。
- 新实际路径：`internal/queries.go`（既有登记）、`migrations/0006_job_management.sql`（本轮批准只追加retry回执）；复用query.go权限和http_events.go映射，BuildApplication实际调用公共完整工厂，其他handler/store/control/owner适配未齐仍blocked。
- 源码提交：`ab4a738b7ff69ed55d7feed39d9ba081d13b519d`。2026-10-15集成会话protocol第三次VPS合成快照：make docs/gen、14个现有Go模块readonly build/vet/gofmt及完整make check全部通过（858文件/1105函数/379操作/2253类型），包含本域管理Go和0006迁移，排除首页消费者HomeWIP；生成/格式产物已回传。仅源码/格式/结构检查，未执行SQL业务验证、测试、真实接口/模型或服务。
- 完整启动、真实owner恢复API适配、control屏障/后台/定时、ephemeral安全投影仍未完成；本轮不伪装可用服务。未本机运行Go/gofmt/gen/测试或临时验证，未新增测试/样例/fixture/mock，未运行真实接口、模型、服务、部署或push。

本域合同与台账已同步，VPS回传functions/files/api生成段、Go与TS生成产物及gofmt；源码SHA精确引用上列提交，台账随后独立提交。旧第二轮停止条款已被本轮持续授权覆盖。

本轮完整函数：ListJobs、ListDeliveries、RetryJob、WithManagementKey、managementPage、finishManagementPage、listJobBatch、listDeliveryBatch、filterManagementBatch、inspectManualRetry、applyManualRetry、readRetryReceipt、retryRequestHash、preserveRetryResults、readJobSummary、jobManagementPermission、NewManagementHandlers；既有authorizeJobManagement复用新目录函数。整体BuildApplication及其他原未完成项没有转为完成。

只读交叉复核另发现公共后台恢复sentinel未保留退出75，以及outbox cleanup与Claim复用旧窗口；均由集成会话在公共域修复并核对闭环。本分支未修改平台文件，也不将其算作本域实现。

下一项建议工作包：实现StreamProgress安全进度SSE与真实HTTP映射，按持久任务owner/type请求当前具体权限，只投影安全阶段和计数并复用平台流机制，不广播原事件payload。真实owner恢复适配、control布局/完整后台及其余启动依赖继续如实保留；当前交付保持冻结，不在本轮开始该后包。


## 第二轮最终集成与停止范围

- 事件目录/schema/接收/投递/确认/重放和真实HTTP核心通过；完整启动/屏障/定时/ephemeral/实际owner适配仍未完成。
- 实际main集成后的VPS make docs/gen、14模块readonly build/vet/gofmt及完整make check通过，统计855文件/1012函数/374操作/2211类型；台账SHA已映射主线提交。
- 用户要求第二轮结束push并停止持续目标，不做第三轮；后续实施需要新的请求。历史“下一步”仅作将来交接，不能当继续授权。
- 未新增或运行业务测试、真实接口/模型、服务、镜像或部署；检查不等于行为验收。


## 持续实施第二轮：事件存储与durable投递（2026-10-15，最终VPS通过、冻结交付）

- 分支`codex/jobs-event-delivery-r1`，基线`468986fc34e1c90b6b5ca575539fdd2b9748cd06`；开始工作树干净，独占`modules/jobs/`，保留旧终局分支。核心源码`0e0a061c3684a0723a8657c73ef258f9a803804b`，必要写窗口`b146b4effe0d4f7be0dd5827cc9ee6f2fa37be71`，最终格式/生成产物`5df9b3cf0d1d2e1e4b87862053275301fc64bde8`，台账随后单独回填；本代理未push。
- 新增实际路径：`internal/event_validation.go`、`event_store.go`、`event_delivery.go`、`http_event_subscription.go`、`migrations/0005_event_deliveries.sql`、`internal/http_events.go`。已有query.go抽出authorizePermission供任务与死信共用；queue.go只增加consumer连接槽计数。未改根、平台、其他域或私有运行数据。
- 根提供生成EventTypes（唯一producer/name/version、managementPermission、consumer列表、内嵌payload规格）与platform.NewPayloadValidator；本域不复制schema验证器、不联网取规格、不手改生成物。信封完整hash覆盖元数据，JSON Number保留大整数精度；稳定eventSeq不因历史清理复用，事件、全部投递和请求回执原子durable后才accepted。
- durable增量流只接受已登记caller=consumer，每consumer最多2流；Next每次在当前epoch/control短事务领取一条queued为delivering并设30秒确认租约，单事件背压。同producer/subject前序未确认或死信会阻塞后序。Last-Event-ID只作传输续接与合法范围核对，不能跳过持久未确认行；重投可重复旧seq，consumer按inbox提交再ack。平台SSE pump负责15秒注释心跳、id帧和关闭。
- Ack只确认本consumer且实际发出的delivery，late ack可确认已回queued的同一事件；从未发出及dead-letter拒绝，重复已ack不改业务。consumer_offsets保存连续确认水位，不跨更早未ack/dead-letter。到期扫描按revision/attempt/expiry CAS，次数加一不超过10则1/2/4秒指数退避上限300秒，超过10转dead-letter，只记录安全错误码。
- Replay输入改为path deliveryId，正文不接受actor/epoch/event/consumer伪造；从持久事件唯一producer/type目录取得具体管理权限后实时请求identity，网络调用不持Store锁或事务。重放事务再核同一事件/consumer/epoch及dead-letter revision，保留原eventId，重置原投递，不覆盖已ack/活动投递。
- 发布及ack在维护draining有效control截止前允许收敛，safe水位后拒写；新投递、到期调度、管理员重放在维护/屏障暂停。真实runtime/control提供者与完整HTTP装配仍由后续装配提供。
- 补jobs.RelayOutbox binding与后台登记：只接platform.EventPublisher、outbox.NewStore和platform.RunRelay，ctx绑定可信op，自指HTTP地址须配置Dependencies[jobs]，不从请求Host推断。平台适配器以eventId稳定幂等键调用生成self JobsClient；不写其余JobsClient假方法。
- 0005只追加事件索引/完整hash、独立全局seq、deliveryId、租约/revision/最后错误、consumer连续水位和event_operations。既有event/delivery不删除；旧信封hash空时按已登记版本校验并只读计算，未知/损坏失败关闭；旧delivering若无可证明expiry明确完整性错误，不猜租约。
- portal ephemeral本轮返回UNSUPPORTED_CAPABILITY，安全投影和逐资源权限尚待根收口，不给portal静态consume条目、不广播原payload。ListDeliveries、完整HTTP/后台/control屏障、定时和实际owner恢复适配仍分别未完成，不能把此包当整站ready。
- 已按公共最终签名补NewEventHandlers四个真实绑定，交NewServiceHandler；Subscribe成功后由apivalidate.ServeEventStream输出，此后不写Problem。其余operation未实现，未伪造全表或启动ready。
- 合成VPS首轮发现的载荷参数类型登记和响应helper签名已调整为[]byte与requestId标量；旧hash空分支补唯一producer及信封ID/修订/时间基本核对。上述修复及Relay WriteWindow均纳入最终VPS快照并通过，不沿用修复前旧证据。
- 第二轮P1收口仅补RelayOutbox必需WriteWindow：每次取完整LeaseRuntimeView，保留SafePointRequested与BarrierDeadline；无截止、provider失败、维护无安全请求全部失败关闭。所有SQL/网络步骤前后的窗口重查和受限IO上下文由平台循环统一执行，不在本域复制机制。用户明确第二轮结束推送后停止，本包不扩owner/ephemeral或第三轮。
- 最终检查：隔离VPS make docs/gen、14模块make build（readonly build/vet/gofmt）与完整make check全部通过，统计855文件/1012函数/374操作/2211类型，已回传最终产物；已交付函数和完整文件verified仅限此检查范围。本机未编译/格式/生成/运行SQL或临时业务程序；未新增测试、样例、fixture或mock，未运行真实接口、服务、模型或部署，本代理未push。

### 本轮后续（一项）

仅由集成会话完成第二轮最终合并复核、用户已授权普通推送和清空目标；本代理保持冻结，不自行push，不启动第三轮。整体启动、control屏障/后台/定时、ListDeliveries、ephemeral投影与实际owner恢复适配继续保留未完成，需以后新的明确授权才开展。

以下为第一轮历史记录：

## 持续实施第一轮最终集成事实（2026-10-15）

- 终局、取消、有限重试、持久任务权限与orphan框架通过；owner恢复适配、事件/定时/屏障和HTTP仍未完成。
- 主线实际源码提交：7566a0c、167aec4；模块分支交付经cherry-pick后SHA已在progress.json映射，后文原分支SHA保留用于溯源。
- 三域逐个合入后各自VPS make docs/gen、14模块readonly build/vet/gofmt与完整make check通过；最终850文件/943函数/372操作/2186类型，生成回传零变化。现有设计检查补核managementPermission与jobManagementGrants集合一致。
- 全部编译/格式/vet/生成检查均在VPS；没有新增测试，也没有运行业务测试、SQL行为验收、服务/真实接口/模型、镜像、部署或push。部分文件与入口继续in_progress/blocked，不把检查当业务验收。
- 下一项以本节和根plan最新工作包为准，后文“等待VPS”等旧说明为过程历史。


## 统一复核追加修复（2026-10-15，VPS通过、待最终集成）

- 追加修复源码`92f4fe029c58baeace5657b9c7762dc9cb5ccd19`，台账回填随后单独提交。原源码`5eabae7a7e9d`及台账`1a16fc73130c`已交付，根复核发现两项真实缺口后追加本包修复；仅本域，未push。
- 管理权限按私有job的owner/type/epoch读取，匹配由各域jobTypes生成的managementPermission；job.manage是入口粗能力，终点请求具体业务功能。读取Store/客户端引用后释放锁再做identity网络请求；Read/Cancel重新取当前runtime并在事务核对已授权任务owner/type/epoch。
- 取消标志以持久cancel_requested为准，Heartbeat不再仅从state推导；Start/Claim/Requeue拒绝已取消意图的任务。Fail在取消请求后报告CANCELLED才确认安全cancelled，其他错误包括可重试依赖/超时均先orphan、保留付费引用并移除lease，等待owner安全确认，不能清除取消或重新执行。
- Fail主动交还当前lease进入orphan时保留当前fencing_counter供原操作只读回执，旧worker因lease已移除及非活动状态无法通过CheckLease；过期回收仍按原合同递增fence。历史queued/retry-wait带取消标记由回收事务隔离。
- 未新增文件、迁移或依赖。隔离VPS make docs/gen成功，14模块make build（只读依赖build/vet/gofmt）全部通过，make check完整通过（850文件/943函数/372操作/2186类型）；managementPermission目录、格式和函数文档已回写。本次源码SHA已回填，交付函数与完整文件记implemented，scheduler因剩余TickSchedules保持in_progress，verified待根最终集成。未本机编译/格式/测试，未新增测试或临时验证。


## 本包事实（R1-03终局与租约恢复，2026-10-15）

- 分支`codex/jobs-execution-complete-r1`；基线`325f56d7b2f6c5567c16d7e21f66d5a88a97e342`；开始工作区干净，独占`modules/jobs/`。源码提交`5eabae7a7e9d7f1fbd74ea3bf74b3015a4d233ef`，台账回填随后单独提交，未push。
- 新增`internal/results.go`、`retry.go`、`cancellation.go`、`query.go`、`scheduler.go`、`migrations/0003_terminal_jobs.sql`、`0004_orphan_jobs.sql`；修改queue/leases及本域types、OpenAPI、implementation。所有生成目录不手改，由集成会话统一make docs/gen。
- 既有终局回执只读重放；换新操作键补写回执前同样核当前control屏障截止，safe水位后拒绝active回执写入。
- Complete/Fail现带workerId；Complete可省略或传null resultRef，无结果任务可完成；Fail可选paidResultRefs，retryAt仅retry-wait存在。成功主结果、已有付费引用、终局状态、租约释放、outbox、操作键及job/fence回执同事务。相同终局可重放，不同worker/fence/结果拒绝。
- RequestCancel/ReadJob调用本域生成IdentityClient取得最新job.manage功能授权，使用可信op.actor，不信任客户端actor或隐藏菜单。取消与完成以同事务revision CAS竞争；leased/running进入cancel-requested，owner通过有效租约Fail(CANCELLED)确认安全取消；orphan只记取消意图。
- Reap每批最多100个精确到期租约或根未知的历史未终局任务，递增job持久fence后进入orphan并移除lease、同事务outbox。事务外OwnerRecoveryChecker确认completed复用持久结果、safeToRetry且根已知时按5/30/120/600秒加确定抖动有限重试；pending/unknown/不可达保持orphan。确认结果须重新核当前epoch/control及orphan/fence/revision CAS。
- 保存可信rootOperationId并随Claim返回，重试不生成新付费根。历史缺根保持未知，不从operationId猜根；允许owner确认已完成后复用结果，安全取消不需要新根。Claim不领取根未知任务。
- 0001/0002未改写；0003追加根操作、安全错误码、取消标志、扩展操作回执及job_outcomes；0004只追加重建jobs状态CHECK和job_orphans，不在迁移中执行无outbox业务状态变更。FK目标保持同名jobs，迁移事务延期检查至重建结束。
- InstallStore接收可信runtime/control提供者、生成IdentityClient和OwnerRecoveryChecker。完整BuildApplication、control持久屏障与实际owner恢复版本化适配仍未接通，明确unavailable，不宣称恢复业务已可运行。
- 检查：集成会话隔离VPS首轮make docs/gen成功，14模块make build（-mod=readonly build/vet/gofmt）通过，BUILD_STATUS=0；make check仅公共可达性与新增文件framework登记待集成收口。源码提交与SHA已回填，已交付函数及完整文件记implemented，scheduler仍有TickSchedules未实施而保持in_progress；完整结构检查尚未标通过。本机只读取与编辑；未编译/格式/vet/运行SQL/临时业务验证，未新增测试、样例、fixture或mock，未调用真实接口/模型、未启动服务或部署。

### 本包下一步（一项）

在实际owner恢复版本化操作确定后装配OwnerRecoveryChecker：先读本域scheduler/恢复合同与根公共协议，逐owner映射持久结果确认及未决付费状态；完成条件是completed/safeToRetry/pending/unknown由版本化权威响应产生，未知失败关闭，BuildApplication仍按其余未完成handler/后台依赖保持blocked。业务验收另行授权，不列补测试。

以下为已合并历史记录：

## 本轮统一复核结果（2026-10-15）

- 三个模块在独立worktree交付，集成分支逐个审查新增逻辑与合同；最终隔离VPS linux/amd64 go1.25.5、GOMAXPROCS=8 的14模块make build和完整make check通过。编译、结构检查不代表业务验收。
- make docs无漂移；make gen第二次运行零变化，核对发生在用户追加VPS专用要求之前；后续所有编译/测试只在VPS，VPS不可用则blocked，不自行本机回退。本轮没有新增或运行业务测试、真实接口、模型、服务或部署。
- 根framework已登记本轮10个实际文件，生成器与消费者统一刷新；本轮已完成函数和完整文件按证据转verified，部分文件与入口仍如实in_progress/blocked。主分支实际实现SHA以progress.json为准。

## 本轮事实（R1-03租约fencing，2026-10-15）

- 独占范围：`modules/jobs/`；分支 `codex/jobs-lease-r1`；基线 `00dd199239aba487bceb5227aac52f27ce949d74`，开始时工作区干净。
- 实现提交：`e6e0d81226358a11a5764ee9ebb43a6706810e9e`（jobs：实现任务租约 fencing 与心跳进度事务）。本段及台账回填随后单独提交，未push。
- 实际代码：新增 `internal/leases.go`（四个binding函数和内部事务/租约/回执/有界候选辅助）、`internal/progress.go`（同事务AppendProgress）；`internal/queue.go`的Store增加可信runtime/control屏障读取钩子，`InstallStore`随启动装配注入。新增只追加迁移 `migrations/0002_leases.sql`，保留原0001内容。

### 已实现的事务与权限

- Claim只领本service所属owner、当前epoch的queued任务；limit省略时显式默认10，实际批次为min(limit,10,owner剩余容量8)。SQL预筛到期候选超集，Go精确排序并只保留10个引用；CAS修改状态、attempt、revision和不随lease释放归零的fencing_counter，写30秒租约、状态outbox和领取回执。
- 同claim操作及输入返回原批次，不续租；不同输入返回CONFLICT。Start核owner/worker/epoch和lease token及持久fence计数，CAS leased→running并写状态事件，同fence重复不重复事件。维护/屏障期间拒绝领取或开始。
- Heartbeat与CheckLease核活动状态、精确expiry及control屏障截止；过期不能复活。维护缺control安全点/截止返回不可用，当前屏障可把续租或重放心跳的有效expiry缩短；返回取消/安全点标志取当前状态。
- 可选progress仅含checkpointKey、安全phase代码与计数；Heartbeat同事务验租约后调用AppendProgress，不嵌套事务、不重复边界校验。检查completed≤total，同job/checkpointKey/inputHash去重，seq单增，同phase历史完成数不能倒退；进度outbox、续租与回执统一提交。

### 合同收口

- OpenAPI先声明CheckLease必填workerId/fencingToken查询参数；types同步claim limit可选默认10/上限10、最多10个返回项；Heartbeat的HTTP/内部输入增加可选ProgressCheckpoint。
- 既有phase“枚举”并无来源，实际schema原为任意字符串；本轮改为1～64位小写字母/数字/点/横线安全代码，事件与进度响应同源。
- 参考函数DispatchOwnerTask的归一化职责并入Claim，删除其未来文件/函数/类型和台账，避免不可达影子函数。AppendProgress改为接收已验租约的本域事务句柄，具体辅助拆分已登记合同/台账并重新make docs。
- `0001`的barriers曾放在active库，与control约束冲突。本包不迁移全量屏障，也不让租约代码再读该表；当前必须注入runtime/control屏障读取钩子，缺少时失败关闭。既有CreateJob的完整屏障与启动接线留后续工作包处理。
- 公共生成器nullable修复与平台JobsClient.CheckLease增加workerId由集成会话提供；本工作分支未cherry-pick公共提交、未手改生成文件，OpenAPI生成物由集成会话统一重建。

### 检查与限制

- 本机darwin/arm64 Go1.27.1，PATH包含Homebrew工具链：根 `make build` 返回0，14个Go模块 `-mod=readonly` build/vet通过且gofmt为空。实现提交前 `git diff --cached --check` 与 `make privacy` 返回0；用户GitHub代用身份保持不变，提交含GPT-6合著者代用地址。
- `make docs`已生成本域functions/files且无范围外漂移。根 `make check` 返回1，仅公共framework未登记3个新增实际文件：`internal/leases.go`、`internal/progress.go`、`migrations/0002_leases.sql`；其余模块合同/类型/接口/依赖/可达性/文档链接/隐私检查均未报错。由集成会话补公共登记并在隔离VPS统一复核。
- 租约组台账暂为implemented并附编译通过证据，未记accepted；完整HTTP、可信runtime/control屏障提供者、应用装配、任务回收/重试、事件投递/死信尚未交付。没有新测试、样例、临时业务验证程序、真实接口/模型、服务运行、联调或部署。
- 未改root/scripts/libs/deploy或其他域；未push。交付后工作区应为空，以Git实际状态为准。

### 下一步（一项）

先收口并实现 `internal/scheduler.go` 的 `ReapExpiredLeases` 与所依赖的重试分类/截止计算：读本域types、functions、operations及root一致性要求，明确owner不可达时orphan确认与可安全重试的分支，再实现同事务递增job.fencing_counter/租约token、保留rootOperationId/inputRef/paid结果引用及状态outbox。不得自动重做不可证明未完成的付费阶段；完整检查通过后同步台账。

## 历史交接


## 本轮下一步（一项）

实现任务完成/失败与过期回收：先核对CompleteJob/FailJob/租约回收合同，延续持久fence与付费结果不重算规则；control屏障提供者与HTTP装配仍待各自工作包，不跨域读取存储。

以下为历史交接，旧下一步以本节为准。


## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `ValidateJobSpec` 白名单改读 make gen 生成的 `internal/gen/job_types.gen.go`（与 config/job-types.json 同源）。
- 迁移改为 `migrations/migrations.go` go:embed 读取 `0001_domain.sql`，删除代码内联的 SQL 副本。
- 合同收紧：`CreateJobInput`/`CreateJobRequestBody` 的 `dedupeKey` 增加 `minLength: 1`，代码同步拒绝空去重键，避免同 owner 的不同任务因空键互相冲突。
- 运行状态适配解开 `state` 外层；修正 go.sum 中 `modernc.org/goabi0` 抄错的校验和并补齐依赖。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增的迁移包、迁移文件与函数已回填修复提交 `978e0cee659dc342650193c628710ff75115add5` 并按上述检查证据转为 `verified`；新增生成表按生成文件约定记 `planned`（由 make gen 重建）。

## 本次事实（R1-05 CreateJob，2026-10-15）

- 本次工作包按上一会话 handoff 的下一项执行；实际共享工作区分支为 `codex/analytics-r1-storage-contract-audit`，基线：`32afbc58bd24b8d03230c920a3b105374c783acb`。
- 实现提交：`588de8fa991340df7dcf019f8c11849f85e1b796`（`jobs：实现任务提交持久化`）；未 push。
- 独占范围：`modules/jobs/`。本次代码提交只包含 `internal/queue.go`、`migrations/0001_domain.sql`、`go.mod` 和 `go.sum`；本交接及台账随后单独同步。未修改根文件、公共合同、生成物、libs、deploy 或其他模块。

## 本次完成与边界

- `jobs.CreateJob` 已按 binding 合同实现：先调用既有 `ValidateJobSpec`，再验证可信 `op.OperationID` 与 `CreateJobInput.dedupeKey` 的 UTF-8/8192字符上限。
- 本域私有 SQLite 已实现 WAL、foreign_keys、busy_timeout=3000ms、单写连接和迁移；事务按 `(owner, dataEpoch, dedupeKey)` 查重，同 inputHash 返回原 `jobId/operationId`，不同 inputHash 返回 `CONFLICT`。
- 新任务以 `queued`、attempt=0、revision=1 持久化，并在相同事务用 `libs/platform/outbox.EnqueueEvent` 登记 `jobs.job-state-changed.v1`。事件只含安全任务引用和状态，不含输入正文；检测到 `barriers.state=draining` 时返回 `LOCKED`。
- `OpenDatabase` 与 `InstallStore` 已提供给后续 BuildApplication 装配；当前 BuildApplication 仍因全部 x-handler、租约、事件/死信、定时、屏障、后台循环和激活回执未完成而按合同返回 `DEPENDENCY_UNAVAILABLE`，没有伪造 ready。
- 迁移文件建立 jobs、leases、progress、job_results、events、deliveries、schedules、barriers、schema_migrations；`nw_outbox` 由 platform 机制幂等建表。没有读取或复制所属服务对象正文。

## 检查与验收

- 已核对初始工作区与本域文档/四份合同；代码提交暂存范围仅为 `modules/jobs/`。共享工作区后来出现的 `modules/site-config/`、`modules/traffic-guard/` 脏改动未触碰、未暂存、未提交。
- 已运行 `git diff --cached --check`，返回码 0；已运行 `python scripts/check_docs.py --privacy-only`，返回码 0。
- 已用 Python `sqlite3` 内存数据库执行 `migrations/0001_domain.sql`，插入一条 queued 任务成功；这不是新增测试样例，也未写入仓库运行数据。
- 完整 `python scripts/check_docs.py` 返回码 1：公共 `docs/structure/framework.json` 尚未登记本域新增实际文件，且存在范围外 site-config 台账脏改动；按本工作包不能修改公共文件或其他模块。
- 当前环境无 `go`、`gofmt` 和 `make`，未执行 Go 编译、vet、gofmt、模块检查或业务行为验收；函数源码状态为 `implemented`，`verified` 和 `accepted` 均未登记。

## 下一步

下一工作包优先实现 `ClaimJob`、`StartJob`、`HeartbeatJob`、`CheckLease` 的租约 fencing，并复用本次 jobs 表、dataEpoch 和迁移边界；之后再接完整 OpenAPI handler 和 BuildApplication。不要修改公共 `framework.json`，由集成会话统一登记本次实际文件。
