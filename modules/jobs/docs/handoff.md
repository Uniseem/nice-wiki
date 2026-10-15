# 任务与事件会话交接

## 统一复核追加修复（2026-10-15，VPS通过、待最终集成）

- 原源码`5eabae7a7e9d`及台账`1a16fc73130c`已交付，根复核发现两项真实缺口后追加本包修复；仅本域，未push。
- 管理权限按私有job的owner/type/epoch读取，匹配由各域jobTypes生成的managementPermission；job.manage是入口粗能力，终点请求具体业务功能。读取Store/客户端引用后释放锁再做identity网络请求；Read/Cancel重新取当前runtime并在事务核对已授权任务owner/type/epoch。
- 取消标志以持久cancel_requested为准，Heartbeat不再仅从state推导；Start/Claim/Requeue拒绝已取消意图的任务。Fail在取消请求后报告CANCELLED才确认安全cancelled，其他错误包括可重试依赖/超时均先orphan、保留付费引用并移除lease，等待owner安全确认，不能清除取消或重新执行。
- Fail主动交还当前lease进入orphan时保留当前fencing_counter供原操作只读回执，旧worker因lease已移除及非活动状态无法通过CheckLease；过期回收仍按原合同递增fence。历史queued/retry-wait带取消标记由回收事务隔离。
- 未新增文件、迁移或依赖。隔离VPS make docs/gen成功，14模块make build（只读依赖build/vet/gofmt）全部通过，make check完整通过（850文件/943函数/372操作/2186类型）；managementPermission目录、格式和函数文档已回写。源码提交后回填SHA记implemented，verified待根最终集成。未本机编译/格式/测试，未新增测试或临时验证。


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
