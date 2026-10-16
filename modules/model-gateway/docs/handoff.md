# AI 接入会话交接

## 两轮并行第一轮：档位配置写入（待VPS检查）

更新：2026-10-16。用户当次明确授权三个子代理并行写两轮，实施所需结构/契约取舍由助手决定，每轮根统一复核并普通push main，中间不询问。独占modules/model-gateway/，分支codex/model-next-two-r1，基线d2161f41c441b69f080b7d8cf3d40b88afecb357。当前源码code-ready尚未提交；等待集成会话VPS生成、格式、编译、vet和完整检查，未标implemented/verified。

实际范围：新增已登记internal/catalog/profile.go，实现PutProfile/readProfileReceipt/validateProfileModels/NewProfileHandlers。task.go已有validateTaskOverride复用于档位直接能力，错误说明改为所选模型；wire.go只合并真实putProfile映射与portal白名单。四契约、README/STATUS/progress与本域data/api/workflows同步；files/functions由根VPS make docs生成，生成目录只接收根VPS make gen回传。

写入复用001/002原有profiles/operation_receipts与平台outbox。首先核admin+superadmin与expectedVersion+1，再同事务取得严格非空runtime_meta当前代次。输入摘要绑定callerService/dataEpoch/profile/expectedVersion，按site/actor/PutProfile/key读取7天完整原结果；同代同hash在后来模型校验、写屏障与当前CAS之前返回。异hash冲突、损坏失败关闭；过期只读视为未命中，只有后续成功的新配置事务才替换。

新写拒绝任何非released屏障，不因屏障epoch/deadline变化放行。profileId+version+dataEpoch条件CAS，首次只允许expectedVersion=0；同代costClass+tier槽被其他profileId占用明确CONFLICT。允许移到空槽；profile.costClass是配置目标，不覆盖可信op.costClass。有序候选上限256且沿schema唯一性，空数组只清候选、保留配置行；按根唯一目录生成AITierCapabilities与已有validateTaskOverride只核模型同代/启用/直接能力，structured/translation等建议不新增硬门槛，不读价格、凭据或供应方可用性。

profile.changed/v1最小载荷为profileId/version/dataEpoch及根AITaskCatalog旧/新tier所涉taskIds去重按ID排序；旧/新含fast时保守包含vision/rerank回退任务，即使当前独立槽已配置；与profiles和原回执同事务写平台outbox。HTTP入口核profileId路径等于正文、非空Idempotency-Key头等于正文key；portal唯一管理调用方，平台统一入口只做一次即时superadmin授权。完整其他handler、权威epoch初始化、恢复回执及后台仍缺，BuildApplication继续拒绝启动，wire.go及profile.go整体partial。

保留old规则：视觉、rerank行缺失或候选为空时，后续ResolveProfile读取分别回退fast中的vision候选、或由fast代替重排；该解析仍planned，不在PutProfile悄悄填充。配置写不触发模型、费用任务、嵌入重建或自动重算已有付费结果。免费可用性读取沿既有约定空候选为已配但unavailable，不能把行称为已删除。

公共配合：根负责scripts/gen_code.py从contracts/ai-tasks.json.tierCapabilities新增AITierCapabilities map[string][]string至既有clients.gen.go，以及已登记profile.go实际结构启用；本域不改公共目录、不手抄生成表、不争用VPS。

检查：尚未执行Go/gofmt/gen/编译/vet/完整check；所有既有检查均由根VPS统一执行并回传后再登记verified。未新增测试、mock、fixture、样例或临时业务验证程序，未执行SQL/HTTP行为验证、真实接口/模型、服务/镜像/部署；本域不push。

下一项仅为将第一轮候选交根VPS检查并按回传格式/生成物收口源码与记录，冻结等待首轮统一集成push及第二轮新基线；不提前开展凭据加密或完整服务。

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
