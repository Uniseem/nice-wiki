# AI 接入会话交接

## 持续目标第一轮：免费真实路由解析（已交付）

2026-10-16。用户新授权每轮三个子代理同时编写，根统一审核后普通push直到代码编写部分完成；此前两轮停止不再适用。独占modules/model-gateway/，分支codex/model-goal-r1，基线685db4f48bb359f79a7501098204ccc5a4cde6b2。源码提交24e4026d452333061cad9ef6abde3050e2944101，根统一VPS检查通过，6项完整路由函数、修改readAvailabilityCandidate与3个完整文件verified。PutProfile本轮源码未修改，其既有costClass条件UPDATE与参数已核对，不虚记本轮修复。

本轮实际源码：internal/catalog/profile.go新增真实ResolveProfile、SelectCandidates及readRouteProfile/applyTaskOverride/readRouteCandidate；新增已登记internal/routing/route.go实现ResolveRoute；task.go的readAvailabilityCandidate复用共享读取器。四份机器来源中只修改implementation/types，HTTP/events不变。人工状态/交接/README/data/workflows同步，functions/files已由根VPS make docs精确回传。无公共目录修改、迁移或外部依赖。

最终真实调用链ResolveRoute → SelectCandidates → ResolveProfile；SelectCandidates持有一次严格当前epoch SQL事务并结束回滚，不先开独立配置快照。目录owner和可信costClass核对，必须真实同代assignment/defaultTier；缺赋值NOT_FOUND。先覆盖替换首项并保序去重，再仅vision/rerank缺槽/仍空时取同costClass真实fast槽；缺fast不能合成。vision继续vision，rerank有override继续rerank，只有无override空档回退fast/chat。其他档不回退，也不因eligible过滤后为空切换档位。

候选遍历完整至多256项并先过滤后保序16，后续项仍检查代次/完整性；embedding所有eligible候选须同空间。安全输出绑定模型元数据、providerVersion、credentialVersion和固定JSON字段顺序路线SHA256。凭据只读代次/长度，不读密文/主密钥/URL/价格；profile费用配置仅声明，不作为费用准入。不获取派发引用、不调供应方、不自动重算付费结果，不能把免费读取器当作完整执行或安全回收依据。

QA免费读取仍固定QA纯服务/三任务/chat，缺assignment保持not_configured，未扩大范围或增加费用权限。完整HTTP/启动/epoch初始化/恢复/后台及真实执行费用、引用释放销毁仍partial/blocked。

已有检查：2026-10-16 根集成会话隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：冻结候选make docs/gen、14个Go模块-mod=readonly build/vet/gofmt及完整make check通过（869文件/1303函数/379操作/2290类型）；profile.go与docs/files/functions精确格式/生成结果已回传，其余无漂移。仅现阶段源码/生成/编译/结构检查，未SQL/HTTP行为验收、业务测试、真实接口/模型、服务/镜像/部署。 本地仅git diff --check与暂存/消息/用户代用身份静态隐私扫描；未执行本机Go/gofmt/编译/vet/测试或make check/gen/docs。源码SHA已精确回填对应verified条目。无新增测试、fixture、mock、样例、临时业务验证程序；未真实接口/模型、部署或本域push。

下一项：根顺序集成本轮源码与记录后统一复核实际树并普通push；本域冻结，后续真实凭据版本引用/释放协议工作包等待根以新基线派发，不提前实现或推送。

以下为既有交付历史。

# AI 接入会话交接

## 两轮并行第二轮：凭据安全持久化（已交付）

更新：2026-10-16。用户授权三子代理并行两轮、每轮根统一检查并普通push main；本轮是最后一轮，完成后停止。基线53acce862467dbe40b829d1984519d78ccac5608，分支codex/model-next-two-r2；公共ServiceConfig.MasterKeyRef/NW_MASTER_KEY_REF前置993904524e4d7a792a6c84a80606de6215f7ace5已正常ff合入。只拥有modules/model-gateway/，源码提交c82f663bb8f935ff67a8da9e22c463ea0765d816；Put完整、紧密辅助与修改后的完整函数verified，Rotate旧引用生命周期与整体启动缺口保持partial/blocked。

实际路径：新增已登记internal/security/credential.go；修改runtime/bootstrap.go（只装配可选secret引用）、app/wire.go（可信引用和真实Put/Rotate映射）、catalog/provider.go（保护active指针及同事务严格epoch）、本域deploy配置/manifest。复用001/002既有credentials/providers/operation_receipts和平台outbox，无迁移、依赖或外部借用。四契约与文档/台账同步，生成物由根VPS回传，不手改。

安全边界：主密钥只来自可信Store装配引用，仅凭据写入安全打开部署secret；平台私有路径检查、Lstat、O_NOFOLLOW/O_NONBLOCK、SameFile、有界33字节读取且恰好32、末次身份/长度/mtime核对。启动LoadRuntime不再读取secret，免费配置读取不受缺键影响。每个新写使用96bit随机nonce，AES-256-GCM AAD固定为规范JSON {format:credential-aes256gcm-v1,providerId,credentialVersion}；不解密旧密文、不探测供应方。

HMAC回执：主密钥按固定用途域派生子键，再HMAC规范输入（callerService/dataEpoch/function/provider/expectedVersion/secret）；不存普通secret SHA、明文或请求体。7天键按site/actor/函数/key固定，当前同代同HMAC先返完整脱敏原结果，后续才核屏障及provider；异输入/异代或换主密钥导致HMAC不一致CONFLICT。过期只随成功新事务替换；本轮不是主密钥轮换，不静默重签旧回执。可变key/plaintext/规范编码/subkey临时字节清零，不承诺Go immutable string及库内部副本擦除。

指纹：使用独立fingerprint域HMAC，固定12个十六进制摘要字符；secret超过8个Unicode字符才附末4位，否则只掩码。原回执只含Put脱敏指纹/新凭据版本或Rotate新/旧版本；事件只含providerId/version/enabled/dataEpoch，无明文、密文、指纹、HMAC或秘密路径。

事务：严格非空当前epoch，所有非released屏障阻止新操作；供应方当前epoch与真实credential_version比较expectedVersion，再INSERT未使用新历史版本，UPDATE同时条件绑定providerId/provider.version/credential_version/dataEpoch并递增provider.version。密文、active、provider.changed与脱敏原结果同事务，未知Commit不重试。PutProvider创建指针必须0、编辑只能保留真实当前指针，不能绕过凭据端点回退/推进。

PutCredential支持首次/替换，也可修复active材料缺失但必须沿真实版本CAS；Rotate要求已有完整同代active材料。全部旧密文保留，不能以目前invocations表空推断无引用。原Rotate“最后引用释放后销毁”生命周期尚无真实派发/引用释放依据，保留该责任并将Rotate整体in_progress/partial、credential.go整体partial；不为完成率挪走责任。完整启动/HTTP其余handler/权威epoch初始化/恢复/后台/模型费用仍未交付，BuildApplication保持blocked。

检查：2026-10-16 集成会话VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：最终修正树make docs/gen、credential.go/bootstrap.go/provider.go/wire.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（867文件/1277函数/379操作/2281类型）；41个本域文件的精确格式/生成快照已回传。本阶段仅源码/生成/编译/结构检查，未SQL/HTTP行为验证、业务测试、真实接口/模型、服务/镜像/部署。 首次候选check因readCredentialReceipt参数顺序失败，该次不计完整通过；改为ctx,op,tx并补全PutProvider原结果/屏障闭环后，全量重跑通过。全部Go检查在根VPS，本机仅git diff及暂存/提交/用户代用身份静态隐私扫描，未改身份配置。未新增测试、mock、fixture、样例或临时业务程序；本域不push。

第二轮源码共22个本域路径，新增已登记credential.go，其他为四机器合同、真实装配/provider/runtime、本域部署引用、人工/生成文档、状态与两个API生成物；无新增迁移或依赖。PutCredential、PutProvider、InstallStore、Store.MasterKeyRef、七项凭据辅助/映射及readProviderReceipt共12个完整条目verified；Rotate/LoadRuntime及整文件缺口未误记完成。

下一项仅为根顺序集成本轮源码/记录、统一复核并按用户本次授权普通push main；本域冻结停止，不第三轮。未来另获授权时，先完善真实派发的凭据版本引用与释放协议，再实现最后引用后的旧密文销毁，不能把当前表空当作安全回收证据。

以下为第一轮及既有交付历史。

# AI 接入会话交接

## 两轮并行第一轮：档位配置写入（已交付）

更新：2026-10-16。用户当次明确授权三个子代理并行写两轮，实施所需结构/契约取舍由助手决定，每轮根统一复核并普通push main，中间不询问。独占modules/model-gateway/，分支codex/model-next-two-r1，基线d2161f41c441b69f080b7d8cf3d40b88afecb357。源码提交82dee76414a823e4d2e71dd87c623d7220f8387e；本轮四个完整新/完成函数、改过的validateTaskOverride及本轮生成物通过根VPS检查并verified；整文件/完整服务缺口保持partial/blocked。

实际范围：新增已登记internal/catalog/profile.go，实现PutProfile/readProfileReceipt/validateProfileModels/NewProfileHandlers。task.go已有validateTaskOverride复用于档位直接能力，错误说明改为所选模型；wire.go只合并真实putProfile映射与portal白名单。四契约、README/STATUS/progress与本域data/api/workflows同步；files/functions由根VPS make docs生成，生成目录只接收根VPS make gen回传。

写入复用001/002原有profiles/operation_receipts与平台outbox。首先核admin+superadmin与expectedVersion+1，再同事务取得严格非空runtime_meta当前代次。输入摘要绑定callerService/dataEpoch/profile/expectedVersion，按site/actor/PutProfile/key读取7天完整原结果；同代同hash在后来模型校验、写屏障与当前CAS之前返回。异hash冲突、损坏失败关闭；过期只读视为未命中，只有后续成功的新配置事务才替换。

新写拒绝任何非released屏障，不因屏障epoch/deadline变化放行。profileId+version+dataEpoch条件CAS，首次只允许expectedVersion=0；同代costClass+tier槽被其他profileId占用明确CONFLICT。允许移到空槽；profile.costClass是配置目标，不覆盖可信op.costClass。有序候选上限256且沿schema唯一性，空数组只清候选、保留配置行；按根唯一目录生成AITierCapabilities与已有validateTaskOverride只核模型同代/启用/直接能力，structured/translation等建议不新增硬门槛，不读价格、凭据或供应方可用性。

profile.changed/v1最小载荷为profileId/version/dataEpoch及根AITaskCatalog旧/新tier所涉taskIds去重按ID排序；旧/新含fast时保守包含vision/rerank回退任务，即使当前独立槽已配置；与profiles和原回执同事务写平台outbox。HTTP入口核profileId路径等于正文、非空Idempotency-Key头等于正文key；portal唯一管理调用方，平台统一入口只做一次即时superadmin授权。完整其他handler、权威epoch初始化、恢复回执及后台仍缺，BuildApplication继续拒绝启动，wire.go及profile.go整体partial。

保留old规则：视觉、rerank行缺失或候选为空时，后续ResolveProfile读取分别回退fast中的vision候选、或由fast代替重排；该解析仍planned，不在PutProfile悄悄填充。配置写不触发模型、费用任务、嵌入重建或自动重算已有付费结果。免费可用性读取沿既有约定空候选为已配但unavailable，不能把行称为已删除。

公共配合已由根提交6b4b15b完成：scripts/gen_code.py从contracts/ai-tasks.json.tierCapabilities生成AITierCapabilities map[string][]string至既有clients.gen.go，已登记profile.go实际结构由根启用；本域不改公共目录、不手抄生成表、不争用VPS。

检查：2026-10-16 集成会话VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：初次同API候选make docs/gen通过；最终fast回退影响修正树make docs、profile.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（861文件/1219函数/379操作/2272类型），精确格式/生成物已回传。本阶段仅源码/生成/编译/结构检查，未SQL/HTTP行为验证、业务测试、真实接口/模型、服务/镜像/部署。 本地只执行git diff --check与既有静态隐私扫描（暂存内容、提交说明和用户Git代用身份）；未改身份配置。未新增测试、mock、fixture、样例或临时业务验证程序；本域不push。

第一轮源码共19个本域路径，包含四机器合同、README/STATUS/progress、api/data/files/functions/handoff/workflows、profile.go/task.go/wire.go及三个既有生成物；无未提交业务源码。

下一项仅为凭据安全持久化：先读internal/security/credential.go精确合同及部署masterKey引用装配，复用现有credentials schema与严格epoch/屏障/CAS/7天回执，实施AES-GCM PutCredential/RotateCredential，不探测供应方；依赖首轮根统一集成检查、push与新的明确基线。当前冻结等待根派发，不能提前开始第二轮或完整服务。

以下为既有交付历史，本次两轮授权取代旧的停止指令。

# AI 接入会话交接

## 本轮任务分配配置读写（已交付）

更新：2026-10-16。用户当次授权三个子代理并行编写一轮；本域独占modules/model-gateway/，分支codex/model-task-write-round1，基线170c1dc9057e122e349de43db532b2c48434b6cf。源码提交c7c1cac9e3a59a206d506083ace25fd00c077e40；deliveryCommit与本轮完整函数/文件台账均引用该实际SHA，检查后记录在后续记录提交。任务配置源码已交付，不开展后包。

实际源码改动仅internal/catalog/task.go与internal/app/wire.go；四份机器合同同步真实职责与协议。源码提交另含README/STATUS/progress与docs api/data/workflows/handoff、VPS make docs生成的files/functions，以及根VPS生成并回传的internal/gen/clients.gen.go、internal/gen/openapi.gen.go、ui/src/gen/api.gen.ts；共18个本域路径。现有001/002 schema足够，无新路径、迁移或外部依赖。完整函数为PutTaskAssignment、ReadTaskAssignments、readTaskAssignmentReceipt、validateTaskOverride及修改后的NewTaskAvailabilityHandlers；task.go全部登记职责与本轮生成物verified。BuildApplication只补配置映射与当前授权，整文件仍partial、函数仍blocked。

PutTaskAssignment检查admin+superadmin、目录任务/defaultTier及expectedVersion+1；本地事务先核非空当前epoch，再按site/actor/函数/key读7天原结果，hash额外绑定callerService和dataEpoch。同代同hash先返回持久结果，不重查后来已停用的模型；异hash为CONFLICT，坏时间/原结果不匹配为INTEGRITY_ERROR。过期读取无副作用；仅新配置事务在非released屏障均不存在、能力和条件CAS成功后替换旧回执。覆盖只读models enabled/capabilities/dataEpoch并满足目录能力，不读秘密、价格或供应方地址，不探测模型。同事务通过平台outbox写assignment.changed/v1最小taskId/version/dataEpoch，Commit未知不自行重试。

ReadTaskAssignments纯服务仅owner范围，显式未知任务VALIDATION、跨owner FORBIDDEN；按taskId排序只返回已持久同代赋值，未配置保持非null空数组。portal admin+superadmin可读全部；因service权限入口不会自动查管理员权限，本域HTTP map在管理员读取分支调用平台NewIdentityAuthorizer即时授权，写入只由公共ServiceBoundaryConfig.Authorize查一次。函数内部不重复身份/schema/deadline校验；HTTP绑定核路径与正文taskId、非空Idempotency-Key头与正文键一致。

公共配合：根已确认assignment.changed按已有约束补生产方，无需新消费者；根AI目录新增tierCapabilities并由make gen生成AITaskCatalog Owner/DefaultTier/RequiredCapabilities。能力沿old：fast/accurate为chat，vision为vision，embedding为embedding，rerank为rerank，不额外强制translation标签。公共根文件、jobs事件生成目录由集成会话拥有；公共前置提交ab94e1e由根集成顺序合入，不在本域工作树重复合并。

检查：2026-10-16 集成会话基于已集成content的模型候选快照：VPS linux/amd64 go1.25.5、GOMAXPROCS=8，make docs/gen、task.go与wire.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt、暂存隐私扫描与完整make check全部通过（861文件/1187函数/379操作/2271类型）。完整本域生成/格式结果由集成会话精确回传，未本机执行Go检查。仅现阶段源码/生成/编译/结构证据，未执行SQL行为验证、业务测试、真实接口或模型、服务/镜像/部署，不代表业务验收。 提交前仅在本地运行既有静态隐私扫描，包含暂存源码、提交信息与用户Git代用身份；未更改身份配置。未新增测试、fixture、mock、样例或临时验证程序，未push。

遗留：完整服务仍因其他handler、权威epoch初始化、恢复回执与后台缺失而拒绝启动；档位/凭据写入、真实模型、费用与完整服务机制均未实施。当前verified仅对应本阶段已有检查，不能视为SQL行为或业务accepted。

下一项仅为集成会话按独立提交顺序合入并复核本轮最终实际树，汇总根台账；本轮结束不自动实施后包，push仍需当次明确授权。

以下为先前交付历史，本次授权已恢复单轮实施。


本轮统一集成实际源码71cd82a，最终实际树6c44451通过VPS docs/gen、14模块readonly build/vet/gofmt与完整check（861/1182/379/2269），生成零变化。progress已映射主线实际SHA，后文原分支SHA保留溯源；用户要求本轮结束停止持续目标后push，本域已交付，不开展后包。

## 第六轮免费任务配置读取（已交付）

更新：2026-10-16。第六轮只实现QA免费任务配置可用性读取，分支codex/model-task-availability-r1，基线cbe93952464bc9bd4a159e4d870c1b191855bc3a，独占本域。源码、合同与VPS生成产物已交付，完整12项相关函数及对应完整迁移/生成文件verified；task.go和runtime/bootstrap.go各自未完成职责保持partial，完整启动仍blocked，业务验收未执行。

- ReadTaskAvailability仅QA纯服务、固定三任务，一次当前epoch SQL快照；缺任务/QA档位为not_configured，已配无可用元数据候选为unavailable；空/异代、损坏与查询未知失败关闭。
- 单任务覆盖替换默认主模型，保留原第二项起有序备用并去重；免费读取固定QA档位，不要求op.costClass变为qa。凭据只查存在/代次及nonce/ciphertext长度，不SELECT密文/主密钥/URL/价格/预算/余额，不探测或调用供应方。
- 新实际路径仅已登记internal/catalog/task.go及本轮批准追加migrations/002_task_availability.sql；profiles/assignment完整schema、AES-GCM凭据结构，无种子或费用写入。配置写路径及实际加密/轮换仍未实施，不能凭schema宣称已可配置运行。
- BuildApplication真实OpenDatabase/InstallStore并将单项映射交公共完整工厂，其他handler、权威代次初始化、恢复回执与后台缺失仍拒绝启动；不为免费读取调用需masterKey的LoadRuntime，不覆写未知epoch。
- 未新增测试/mock/fixture/样例或临时业务程序；未本机Go/gofmt/gen/编译/vet/check，未SQL业务验证、真实接口/模型、服务、部署或push。本包收尾后按用户要求停止，不开启后包或自行push，集成会话统一复核。
- 源码提交：23db600ead941c088344af2c3a33bf4204277229。2026-10-16 集成会话最终合成快照：VPS linux/amd64 go1.25.5，GOMAXPROCS=8，make docs/gen、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check全部通过（861文件/1182函数/379操作/2269类型）；包含256项持久配置、先eligible过滤后16项路由参考裁决、最终只读代码和002 schema，生成/格式回传无冲突。仅本阶段源码/格式/结构检查，未执行SQL业务验证、业务测试、真实接口/模型、服务、部署或push。

完整函数为ReadTaskAvailability、beginAvailabilitySnapshot、readAvailabilityTask、readAvailabilityCandidate、availabilityReadError、NewTaskAvailabilityHandlers，以及本轮同步实际合同的既有OpenDatabase、CurrentStore、InstallStore、Store.BeginTx、Store.Close、runMigrations。生成目录未手改，VPS回传产物已纳入上述源码提交，台账精确引用该SHA。task.go剩余PutTaskAssignment/ReadTaskAssignments与完整启动/模型费用缺口没有转为完成。

下一项：本包完成后按用户要求停止目标，不自动继续模型执行、费用或其他工作包；后续仅由集成会话收口本包检查、交付与普通推送。

以下为既有交付历史。

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `PutProvider`/`PutModel` 事件改经平台 `outbox.EnqueueEvent` 写 nw_outbox，删除自建 outbox 表（迁移与内联副本），平台转发循环才能投递（ADR0005）。
- 迁移改为 go:embed；`cmd` 补 `Application.Serve/Shutdown` 方法；运行状态适配解开 `state` 外层；补齐 go.sum。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增的迁移包、迁移文件与函数已回填修复提交 `978e0cee659dc342650193c628710ff75115add5` 并按上述检查证据转为 `verified`。
- 遗留：迁移文件 `001_initial.sql` 在未部署前删除了 outbox 表定义；一旦发布即只追加。

## 本次事实

2026-10-15 完成本域最小实施切片，提交 `4eec945320f8cf20bfc6673a8c3c8490385aca2f`。新增 `internal/runtime/bootstrap.go`、`internal/catalog/provider.go`、`internal/catalog/model.go`、`migrations/001_initial.sql`，并为本域 `go.mod` 登记 `modernc.org/sqlite v1.59.0`。

- `LoadRuntime`：校验绝对目录和恰好32字节主密钥引用，建立本域SQLite/WAL、严格schema迁移和运行代次水位；不调用模型。
- `ValidateProvider`：校验HTTPS根URL、协议白名单、超时上限、附加头与Authorization覆盖，拒绝userinfo/fragment/query；不做网络探测。
- `ValidateModel`：校验能力集合、嵌入维度和chat/translation输出上限，计算稳定embedding space hash；不收费。
- `PutProvider`/`PutModel`：完成superadmin前置、dataEpoch水位、屏障、CAS、幂等回执、配置表和provider/model.changed outbox的同事务写入。

真实供应方调用、连接探测、密钥AES-GCM存储、档位/路由、预留/派发/结算/未知收费对账、完整HTTP handler与后台循环仍未实现。未新增测试/样例/部署，未调用真实接口或模型，未push。

## 基线与文件

本域合同摘要及每文件/函数状态在 [progress.json](../progress.json)。设计基线为 `cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`；本次交付提交为 `4eec945320f8cf20bfc6673a8c3c8490385aca2f`，分支为 `main`。先用git status核对并行会话留下的未提交工作，不能把它们纳入本域提交。

独占范围是 modules/model-gateway/。根contracts/docs/scripts和其他模块由集成会话维护；禁止跨目录顺手修接口。现有功能、默认及用户操作以 [功能保持矩阵](../../../docs/product-preservation.md) 和本域requirements为准。

## 下一步（一项）

在VPS补跑Go/gofmt/迁移验证后，把本次Store接入 `BuildApplication`：由启动装配把site-config读取到的active dataEpoch写入本域水位，注册全量OpenAPI handler和activation回执；缺任一项继续返回 `DEPENDENCY_UNAVAILABLE`，不开放半成品业务入口。

完成条件：VPS格式/编译通过、Store迁移与代次门禁验证、全量handler绑定与启动失败语义符合合同；新测试、真实接口或模型仍需单独授权。不能将其他业务函数一起标记完成。

## 下一次结束填写

交付提交：`4eec945320f8cf20bfc6673a8c3c8490385aca2f`。实际完成函数：5个（LoadRuntime、ValidateProvider、PutProvider、ValidateModel、PutModel）；实际修改仅限 `modules/model-gateway/`。检查：`git diff --check`通过；因本机无Go/gofmt/make，未编译或运行迁移/行为验收。阻塞：BuildApplication全量接线、归一化规则合同缺口及工具链缺失。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。


## 第四轮首页跨域只读协议

新增readTaskAvailability固定QA链三任务的免费配置投影。缺价格按原产品不阻断，不查询余额，不调用供应方或模型；仅合同与生成来源，业务函数仍planned。

第二轮收口窄修：PutProvider同代普通摘要显式绑定callerService/dataEpoch，7天原结果先于新写屏障和当前active指针返回；新写拒绝任意非released屏障，不按屏障epoch/deadline过滤。过期键仅随成功新配置事务替换，PutModel及其旧共享helper未改。readCredentialReceipt与新增渠道回执参考签名均以ctx,op开头。首次候选Go检查通过但结构check因签名顺序失败，该次不记完整通过；修后根全量生成/编译/结构检查通过，证据见本轮交付记录。
