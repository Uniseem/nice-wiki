# 全域屏障与签名清理

此协议只提供公共机制。14域真实业务回调、jobs水位SQL和data-protection协调器未接齐时，不构成整站safe/Ready证明。旧epoch活动资格必须撤销并隔离保留审计、原费用/输入/输出/来源事实，不能删除后声称未发生，也不能通用改epoch重投递。

激活回执独立签名载荷固定为`version,restoreId,module,stageId,targetDataEpoch,state,manifestHash,recordedAt`。`recordedAt`在载荷摘要前由公共Claims助手解析为UTC并以RFC3339Nano统一编码，签发端与验签端使用同一规范；HTTP服务断言仍只证明caller，site-config还须核当前恢复阶段、restoreId、expectedRevision和域覆盖。

## HTTP与任务scope

ServiceBoundaryConfig.AdmitBusiness覆盖完整已鉴权、已严格schema验证的非控制HTTP与SSE。ControllerOperations只免业务计数，不免身份、权限、epoch。CleanupOperations也不能跳过AllowedCallers/ServiceActorCallers。TaskGroup.SetAdmission必须装真实独立Manager准入；Start不继承HTTP将release的同一租约。CancelActive/WaitIdle用于备份后可恢复的旧任务停止，Drain是进程永久关闭。

WorkerConfig.AdmitBusiness装AdmitJobClaim，BindExecutionScope装BindJobExecutionScope。随机claim root未绑定前不能发业务cleanup；标准worker核job owner、实际worker/fence、factory原op/root/epoch后只绑定一次。根分支的OnCancellationObserved真实取消钩子必须保留。

RelayConfig.AdmitBatch和BindEventScope分别覆盖完整批次及原事件publish到最终mark/retry/quarantine。标准BindOutboxEventScope从真实Store原行绑定eventID控制根；它不是paid root。ConsumerConfig.AdmitBatch/AdmitDelivery覆盖订阅批次和独立delivery，后者到handler本地提交及真实ack结束才release。AllowDrain只允许当前draining的旧事件控制；旧epoch事件hold不伪ack。

## prepare、drain、seal

公共API：PrepareBarrier(ctx,SnapshotBarrierInput)、DrainBarrier(ctx,barrierId)、ObserveBarrier(ctx,barrierId)、SealBarrier(ctx,SnapshotSealProof)、ResumeDrain(ctx,barrierId,expectedSealHash)。旧EnterBarrier只转PrepareBarrier。HTTP BarrierRequest.step省略只能prepare；step允许prepare/drain/observe/seal/resume-drain，由ROOT协调HTTP装配，不能省略当safe。

Prepare持久draining，拒新HTTP/paid/periodic/job claim，取消旧进程租约，BeginDrain仅停止与取消，不等待跨域收尾。所有14域prepare后，Drain才可并行收尾原费用、取消、回执、事件和最后outbox，不运行冻结收费queued job清零。

SnapshotWorkState为{outboxRevision,workRevision,pendingOutbox,pendingWork}。两个revision都为真实SQL单调正数，未知失败。pendingOutbox包括未来重试、leased、quarantined所有未发布活动；旧代已明确撤资格的discarded仍保留原行但不算活动。pendingWork为本域真实待收尾执行/持久变更，不把稳定保留未知费用或冻结收费队列自动当成可继续执行。ObserveBarrier返回{owner,activityRevision,work,revisionWatermark,business,cleanup,fileOps,drainCallbacks}，观察前后activity必须一致且已经Drain。

SnapshotSealProof为{barrierId,dataEpoch,observations,brokerBefore,brokerAfter,nonJobsSeals?,nonJobsProofHash?}。observations必须按owner排序，恰好14个唯一owner全部覆盖。协调器先提供可最终复用的broker基础观察：{revision,pendingDeliveries,activeLeases}来自真实发布/ack/lease水位，前后完全相同且pending/lease为0；它不包含冻结queued job。所有owner进程计数、活动outbox/work均为0。非jobs不在本域封口后等待第二次broker回执，jobs最后重新读取broker并确认仍等于该基础观察。proofHash=SHA256(CanonicalJSON完整proof)。

所有非jobs先用同一不含nonJobs字段且包含稳定零broker基础观察的proof封口。其VerifyGlobalSeal只读取并核当前持久prepare/drain/epoch和协调阶段，不在本域封口后等待第二次broker回执；Manager mutex拒新cleanup，SealWork真实SQL事务CAS两个持久水位并禁止后续本域业务写，状态safe和sealProofHash持久后才返回。jobs最后用原完全相同观察/broker基础水位以及13个{owner,sealProofHash}真实回执，nonJobsProofHash为去掉nonJobs字段后原proof hash；jobs的VerifyGlobalSeal重新读取broker并确认仍同基础水位。该闭包阻止错时采样0后新增发布源，不是各域各自drain就safe。

### `step=observe` 的服务调用边界

`BarrierRequest` 使用既有 `step:"observe"`、`operationId`、`barrierId`、`mode`、`deadline`；不得携带 `sealProof` 或 `expectedSealHash`。该步只读持久 prepare/drain/seal、运行 `dataEpoch`、协调记录和可最终复用的稳定零broker基础观察，不写观察、不签回执、不改变屏障状态。服务调用必须使用本域既有服务断言，`caller` 与服务slug一致，`audience=jobs`，`dataEpoch` 与原操作一致；不携带用户委托，也不能把客户端自报 actor 当授权。

非jobs的 `VerifyGlobalSeal(ctx, proof, proofHash)` 只能接受与本域相同的 `barrierId/dataEpoch`、当前持久阶段和原proof摘要；它可以调用 jobs 的只读 `step=observe` 端口，但只等待 prepare/drain/epoch及已选定基础proof证据，不在本域封口后等待第二次broker回执。缺证据、阶段倒退、epoch不符或协调端不可达时失败关闭；不等待 `nonJobsSeals` 或封口后的最终 broker 水位。

jobs的最终 `VerifyGlobalSeal` 只能在13个非jobs已持久 `safe` 且原回执可由 `VerifyActivationReceipt` 核验后调用；它复用同一不含 `nonJobs` 字段的proof和broker观察，要求13份 `{owner,sealProofHash}`、`nonJobsProofHash`、broker前后相同且pending/lease为0全部成立。prepare/drain/seal/resume-drain等变更步骤仍只由 data-protection 服务身份加超管委托驱动。当前 jobs 的 `coordinateBarrier` OpenAPI 仍把整个操作绑定为 data-protection+超管委托，因此在该合同拆分出 service-only observe 之前，identity、site-config 的跨域回调保持阻塞，不得自行伪造证据。

任何变化都保留维护；显式ResumeDrain绑定原sealHash，只重开旧清理，不开新业务/paid，并需重新Drain/全域取证。重启safe先转failed，不能复用旧进程0计数；必须显式resume后重证。ReleaseBarrier只能在真实全局running且opened epoch匹配、计数0时重开旧持久gate并解除本域屏障。

标准outbox.Store.Observe从SQL统一revision与全部活动行读事实；Seal原子CAS revision、活动0并持久hash，触发器禁止此后INSERT/UPDATE/DELETE。Resume只认同一原hash，重复释放幂等。域业务水位与outbox同库时须用ObserveTx/SealTx/ResumeTx共享调用方同一`platform.Tx`，由调用方把域CAS和outbox封口或重开一并提交或回滚；不得另起事务后自称原子。本域额外业务表、文件权威或周期任务仍须真实SealWork/ObserveWork证据，单靠outbox不证明整个域安全。

激活回执的独立签名载荷ActivationReceiptClaims绑定`version,restoreId,module,stageId,targetDataEpoch,state,manifestHash,recordedAt`。SignActivationReceipt使用模块部署Ed25519密钥与既有CanonicalJSON/JWS底层签发；VerifyActivationReceipt从已登记模块公钥验签并核规范载荷。外层HTTP服务断言仍只证明caller，site-config还必须要求caller=module且请求restoreId、current phase、expectedRevision和域覆盖一致；不能用结构合法或空签名代替持久来源证明。

## cleanupScope

断言内可选cleanupScope={version:1,barrierId,dataEpoch,deadline,sourceService,targetService,action,resourceId,rootOperationId,siteId,costClass?,queryHash,eventControl?}。queryHash=SHA256实际URL.RawQuery，空串也hash；method/path/bodyHash仍由同一服务断言签名。scope期限严格为min(30秒,ctx期限,原barrier期限,原cleanup期限)，没有时钟偏差延长。

只有活跃原私有执行租约或SnapshotCleanup.Context可发行。后者必须AuthorizeCleanupOrigin从原持久对象核原批准任务，不能用协调器随机root代替。ExistingWorkContext只从尚未release的真实旧执行分叉独立短清理租约，脱离原HTTP取消并清除过期用户proof，不复活原用户身份；读运行权威用独立2秒只读截止，原付费deadline不改变，不重签/消费模型permit。固定CleanupRoutes按真实method/path/body资源绑定，Control仅明确纯读控制操作；BrokerControl仅持久event/consumer控制根，与paid任务清理根严格分开，eventControl=true随接收/分叉租约保持；目标固定policy.BrokerControl必须全等。

接收方先验证服务签名、原root/site/epoch/class/queryHash，固定CleanupOperations再提取实际path/body资源，与签名全等，并AuthorizeCleanup从本域权威原对象核归属。仅draining和同barrier可AdmitCleanup计独立租约，safe拒新清理。createInvocation/dispatch/createJob/claim/任意业务写不能登记为cleanup。controller标记也不能给写端口免除真实私有lease。

## 完整捕获

CaptureSnapshotFiles(ctx,activeRoot,workspace,databases map[string]*sql.DB,exclude SnapshotCaptureExclude)只能从Manager真实safe Capture回调调用。activeRoot固定同DataRoot/active，workspace为空隔离树。固定exclude为明确相对路径无glob；拒全树symlink/hardlink/非regular和未登记SQLite。登记数据库以PRAGMA database_list核主文件精确归属，再VACUUM main INTO独立副本；仅跳对应live DB/-wal/-shm，未知回滚journal失败。完整容量真实预检、普通文件流copy+hash/fsync、数据库副本hash/fsync、全部目录fsync后返回真实inventory。本域仍须生成真实objects/publications/assets/invocations/references与secret redaction，不以此工具伪造业务库存。

Manifest V2必须显式assets/invocations数组（真实空也保留[]），AssetVersionRef五字段assetId/version/sha256/mediaType/byteSize，SnapshotInvocationRef原invocationId/rootOperationId/dataEpoch。V1只在没有这些库存与typed引用的明确分支保持原canonical字段集合，不能加null导致原hash漂移；V1旧引用不能当V2空闭包。

恢复staging可调用`outbox.PrepareRestoredOutbox(ctx,db)`，仅私有已认证Stage上下文并核真实staged主DB路径；源须已有真实outbox表，不创建空历史。事务内暂开复制的源gate，旧epoch pending/quarantine只撤活动资格并保原行，然后把gate绑定本次host sealHash继续封口；全局activate后Release才按该hash重开。原payload/epoch/费用证据不改。
