# 持续目标第二轮：管理读取与费用策略已交付

更新：2026-10-16。分支codex/model-goal-r2，基线471c036082e082763d90e1080b3489d4e4531380，独占本域。源码提交14c5d80cb05d28f345c0db84916deb75dd92ec3e，根VPS生成/编译/结构检查与交叉审读通过；本轮14项完整函数verified，ReadModels和ReadAdminCatalog仍in_progress，admin/read.go整体partial，BuildApplication仍blocked。业务验收与部署未开始。

- 新增既有登记internal/admin/read.go及获根批准003_fee_budget_policy.sql；ReadProfiles在真实同代快照按根任务owner/可信op.costClass过滤，portal当前model.manage可全量。ReadModels只完成声明读取，空间身份/归一化历史缺口未修；ReadAdminCatalog四目录同快照与安全供应方投影已写，最近probe生产者/投影未齐，不虚构结果。
- ProviderSummary只查非秘密配置、active凭据版本/材料存在及长度，不SELECT/返回附加头secret引用、nonce/密文正文；credentialConfigured不表示可解密、在线或已经探测，异代或坏长度失败关闭。超响应上限报错，不静默截断。
- policy无行NOT_FOUND；old保留设计只确认网络默认5倍和daily0不限，未确认会话美元数字，部署资料无策略种子；迁移不插金额默认。Put首次expectedVersion0仅当前superadmin显式完整创建；普通guard.manage仅保留当前日额度调整小时设置，daily变化另即时superadmin。
- 当前guard.manage公共授权先于原回放；7天同caller/epoch规范输入完整原结果先于当前policy/CAS/屏障返回。新写预读事务结束后无锁权限RPC，再开短事务重核回执、所有非released屏障、归属/CAS和授权条件；策略、完整回执及本地脱敏审计同事务，不清held/ledger。整体跨域审计投影仍未实现，未宣称通知analytics。
- 五项真实HTTP映射已装配，no-store及幂等头一致；复用公共NormalizeIOError，未复制通用错误机制。公共前置源码7a4f55b、记录ec1732e；完整服务仍因其他handler、权威epoch初始化、恢复及后台缺失拒绝启动。
- 2026-10-16 根集成会话隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：冻结合成树make docs/gen、read.go与wire.go的gofmt、14个Go模块-mod=readonly build/vet/gofmt及完整make check通过（873文件/1343函数/379操作/2300类型）；7路径精确格式/生成回传，他域零漂移。根和content代理交叉审读无新增阻断；公共依赖7a4f55b（记录ec1732e）。仅源码/生成/编译/结构证据，未SQL/HTTP行为验收、业务测试、真实接口/模型、服务/镜像/部署。
- 未本机Go/gofmt/gen/编译/check，未新增测试/mock/fixture/样例/临时业务程序，未执行真实接口/模型、服务或部署。本域不push，交付后冻结。实际暂存、提交消息、作者/提交者和GPT-6署名已静态隐私扫描；保持既有GitHub代用身份。

# AI 接入当前进度

更新：2026-10-16。用户新授权每轮三个子代理并行编写、统一审核及普通push直到代码编写完成，取代此前两轮停止指令。本轮基线685db4f48bb359f79a7501098204ccc5a4cde6b2，分支codex/model-goal-r1，独占本域。源码提交24e4026d452333061cad9ef6abde3050e2944101；免费同代路由已通过根VPS检查与交叉审核，6项完整路由函数、修改的readAvailabilityCandidate及3个完整文件verified。PutProfile源码本轮未修改，保留原实现SHA。

- ResolveRoute通过SelectCandidates在一个严格当前epoch SQL快照内调用ResolveProfile；任务须目录登记且属于可信callerService，costClass只核对可信op。必须真实同代assignment且tier等于目录defaultTier，不合成缺失赋值。
- 先override替换主模型并保序去重，空列表可仅覆盖；仅vision/rerank在目标槽缺失或覆盖后仍空时使用真实同costClass fast配置。vision仍需vision，rerank有override仍需rerank、无override空档才fast/chat；缺配置NOT_FOUND，其他档不回退。
- 全部256项先按安全元数据筛选，保序最多16项；同代模型/供应方/凭据引用及完整性未知失败关闭，eligible embedding全部同空间。返回模型/供应方/凭据版本及路线摘要；没有解密、探测、费用准入或持久派发引用。
- QA免费gate仍只允许QA纯服务、固定三任务、chat能力及原有缺赋值not_configured语义；仅复用共享安全候选读取器。无新增HTTP、迁移、依赖、测试、样例或临时业务程序，未本机Go/gofmt/编译/check、真实接口/模型、部署或本域push。
- 完整服务、权威epoch初始化、费用预留/派发/结算、凭据引用释放及旧密文销毁仍未完成；BuildApplication保持blocked，Rotate整体partial。候选检查不等于业务验收。

- 2026-10-16 根集成会话隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：冻结候选make docs/gen、14个Go模块-mod=readonly build/vet/gofmt及完整make check通过（869文件/1303函数/379操作/2290类型）；profile.go与docs/files/functions精确格式/生成结果已回传，其余无漂移。仅现阶段源码/生成/编译/结构检查，未SQL/HTTP行为验收、业务测试、真实接口/模型、服务/镜像/部署。

以下为既有交付历史，以页首最新授权与progress.json为准。

# AI 接入当前进度

更新：2026-10-16。第二轮（最后一轮），分支codex/model-next-two-r2，基线53acce862467dbe40b829d1984519d78ccac5608；公共可信masterKeyRef前置993904524e4d7a792a6c84a80606de6215f7ace5已正常合入。独占本域，源码提交c82f663bb8f935ff67a8da9e22c463ea0765d816；PutCredential与紧密辅助、HMAC回执、可信引用装配及PutProvider指针保护已verified。Rotate最后引用销毁生命周期未齐，整函数/credential.go仍partial，完整启动blocked。

- 新增已登记internal/security/credential.go，复用001/002 schema，无新迁移或依赖。主密钥只在凭据写入读取：私有路径、NOFOLLOW/SameFile、最多33字节、恰好32字节及末次文件身份核对；启动/免费配置读取只装配引用，不依赖secret文件。
- AES-256-GCM每次新操作随机96bit nonce，完整providerId/credentialVersion规范AAD。严格同代事务核7天原结果、全部非released屏障、真实active版本和供应方双CAS、历史版本唯一，密文/active指针/provider.changed/脱敏回执同事务。未知Commit不自行重试。
- 敏感摘要使用主密钥派生的域分离HMAC，不保存可离线猜测的普通secret SHA；同一部署主密钥是回执协议，本轮不是主密钥轮换，换键不静默改写旧回执。指纹只有截短HMAC与掩码，超过8个Unicode字符才显示末4位；短secret不完整回显。临时可变字节清零，不承诺擦除Go不可变字符串或库内部副本。
- PutProvider创建credentialVersion只能0，编辑只能保留真实active指针，凭据端点独占版本推进。PutCredential可修复active材料缺失且沿真实版本CAS；Rotate要求既有完整active材料并保留旧密文，引用释放后销毁仍缺，Rotate与credential.go整体partial。
- 真实putCredential/rotateCredential映射复用公共当前superadmin授权并核路径/幂等头。完整其他handler、权威epoch初始化、恢复与后台未齐，BuildApplication仍blocked；无模型/费用调用或自动重算。
- 2026-10-16 集成会话VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：最终修正树make docs/gen、credential.go/bootstrap.go/provider.go/wire.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（867文件/1277函数/379操作/2281类型）；41个本域文件的精确格式/生成快照已回传。本阶段仅源码/生成/编译/结构检查，未SQL/HTTP行为验证、业务测试、真实接口/模型、服务/镜像/部署。
- 首次候选完整check因readCredentialReceipt参数未以ctx,op开头失败，修正后上述全量检查通过；PutProvider同时收口7天原结果/所有屏障/caller+epoch摘要与指针旁路。未本机Go检查，暂存内容/提交说明/用户Git代用身份已静态扫描，身份配置未改；未新增测试/mock/fixture/样例/临时验证程序。本域不push，第二轮交付后冻结停止，不启动第三轮。

以下为第一轮及既有交付历史。

# AI 接入当前进度

更新：2026-10-16。用户授权三个子代理并行写两轮、每轮根统一检查并普通push main。当前为第一轮，分支codex/model-next-two-r1，基线d2161f41c441b69f080b7d8cf3d40b88afecb357，独占本域。源码提交82dee76414a823e4d2e71dd87c623d7220f8387e，PutProfile及三个紧密辅助/真实HTTP映射、改过的模型能力校验和本轮生成物verified；profile.go及wire.go整体partial，完整服务仍blocked，业务验收未执行。

- 复用001/002 schema，新增已登记实际源码internal/catalog/profile.go；同事务严格非空epoch、7天原回执、所有非released屏障、配置版本/代次条件CAS与costClass+tier唯一槽。有效原结果先返回，过期键仅随成功新配置事务替换；提交未知不自行重试。
- 有序候选保持原顺序、唯一且最多256，空数组只清空候选而保留配置行。直接能力复用根唯一目录生成的AITierCapabilities与既有模型元数据校验；不要求凭据/价格/供应方在线，不探测、不调用模型、不自动重算嵌入。
- profile.changed/v1与配置、回执同事务写平台outbox，载荷仅profileId/version/dataEpoch及旧/新档位根目录受影响taskIds去重排序；旧/新含fast时保守包含vision/rerank回退任务。HTTP核路径/正文及幂等头一致性，portal写入复用公共入口即时superadmin授权。
- 视觉及rerank档位行缺失或候选为空时的fast回退明确留在ResolveProfile/ResolveRoute参考合同，仍planned；免费可用性读取中已有空候选为已配但unavailable，不能把配置行声称已删除。
- 2026-10-16 集成会话VPS linux/amd64 Go1.25.5、GOMAXPROCS=8：初次同API候选make docs/gen通过；最终fast回退影响修正树make docs、profile.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check通过（861文件/1219函数/379操作/2272类型），精确格式/生成物已回传。本阶段仅源码/生成/编译/结构检查，未SQL/HTTP行为验证、业务测试、真实接口/模型、服务/镜像/部署。
- 本轮无新增迁移、外部依赖、测试/mock/fixture/样例或临时业务验证程序；未本机Go检查、真实接口/模型或部署。暂存内容、提交说明、用户Git代用身份已静态隐私扫描；身份配置未改。本域不push，第一轮交付后冻结，根每轮统一收口；第二轮待根给首轮新基线后派发，不提前实施。

以下为既有交付历史，本次授权取代旧的单轮停止指令。

# AI 接入当前进度

更新：2026-10-16。本轮任务分配配置读写已交付，分支codex/model-task-write-round1，基线170c1dc9057e122e349de43db532b2c48434b6cf，独占本域。源码提交c7c1cac9e3a59a206d506083ace25fd00c077e40；本轮完整Put/Read、2项辅助和修改后的HTTP映射，以及task.go和本轮生成物verified。wire.go整体partial、BuildApplication blocked，业务验收未执行。

- PutTaskAssignment落实superadmin、根唯一任务目录默认档位与覆盖能力、严格非空epoch、同代7天原始回执、非released屏障、SQL条件CAS与同事务assignment.changed平台outbox。先返回有效原回执，不因后来模型停用或本地屏障阻断同代无副作用重放；异输入/异代回执冲突，过期仅同成功新配置事务替换。
- ReadTaskAssignments只读取根目录owner=callerService的已持久配置，显式跨owner拒绝，未配置保持空数组。portal管理员全量读取通过HTTP边界平台identity即时superadmin授权；写入由公共边界统一授权一次，SQL函数信任可信op。路径、正文及幂等头一致性在本域HTTP绑定核实。
- 沿用既有task.go、wire.go及001/002持久结构；根集成会话在既有clients.gen.go生成AITaskCatalog，不手抄目录、不运行时跨域读文件。缺失assignment.changed生产契约已补最小taskId/version/dataEpoch载荷，无新增消费者。公共前置为ab94e1e。
- 本轮未实现档位写入、凭据加密、模型/费用执行、权威epoch初始化、完整HTTP/后台或可运行服务；不以持久schema和映射冒充完整配置运行。
- 2026-10-16 集成会话基于已集成content的模型候选快照：VPS linux/amd64 go1.25.5、GOMAXPROCS=8，make docs/gen、task.go与wire.go的gofmt、14个现有Go模块-mod=readonly build/vet/gofmt、暂存隐私扫描与完整make check全部通过（861文件/1187函数/379操作/2271类型）。完整本域生成/格式结果由集成会话精确回传，未本机执行Go检查。仅现阶段源码/生成/编译/结构证据，未执行SQL行为验证、业务测试、真实接口或模型、服务/镜像/部署，不代表业务验收。
- 未新增测试/mock/fixture/样例或临时验证程序，未执行push。本轮交付后停止，不自动进入下一包。

以下为先前交付历史，旧停止指令由本次用户授权单轮并行实施取代。


更新：2026-10-16。第六轮只实现QA免费任务配置可用性读取，分支codex/model-task-availability-r1，基线cbe93952464bc9bd4a159e4d870c1b191855bc3a，独占本域。源码、合同与VPS生成产物已交付，完整12项相关函数及对应完整迁移/生成文件verified；task.go和runtime/bootstrap.go各自未完成职责保持partial，完整启动仍blocked，业务验收未执行。

- ReadTaskAvailability仅QA纯服务、固定三任务，一次当前epoch SQL快照；缺任务/QA档位为not_configured，已配无可用元数据候选为unavailable；空/异代、损坏与查询未知失败关闭。
- 单任务覆盖替换默认主模型，保留原第二项起有序备用并去重；免费读取固定QA档位，不要求op.costClass变为qa。凭据只查存在/代次及nonce/ciphertext长度，不SELECT密文/主密钥/URL/价格/预算/余额，不探测或调用供应方。
- 新实际路径仅已登记internal/catalog/task.go及本轮批准追加migrations/002_task_availability.sql；profiles/assignment完整schema、AES-GCM凭据结构，无种子或费用写入。配置写路径及实际加密/轮换仍未实施，不能凭schema宣称已可配置运行。
- BuildApplication真实OpenDatabase/InstallStore并将单项映射交公共完整工厂，其他handler、权威代次初始化、恢复回执与后台缺失仍拒绝启动；不为免费读取调用需masterKey的LoadRuntime，不覆写未知epoch。
- 未新增测试/mock/fixture/样例或临时业务程序；未本机Go/gofmt/gen/编译/vet/check，未SQL业务验证、真实接口/模型、服务、部署或push。本包收尾后按用户要求停止，不开启后包或自行push，集成会话统一复核。
- 源码提交：23db600ead941c088344af2c3a33bf4204277229。2026-10-16 集成会话最终合成快照：VPS linux/amd64 go1.25.5，GOMAXPROCS=8，make docs/gen、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check全部通过（861文件/1182函数/379操作/2269类型）；包含256项持久配置、先eligible过滤后16项路由参考裁决、最终只读代码和002 schema，生成/格式回传无冲突。仅本阶段源码/格式/结构检查，未执行SQL业务验证、业务测试、真实接口/模型、服务、部署或push。

以下为既有交付历史，当前以页首和progress.json为准。

更新：2026-10-15。阶段：R1实施中；本域已完成目录校验、模型身份纯规则和私有SQLite最小持久化切片，尚未业务验收或部署。

- 2026-10-15 集成复核修复：`PutProvider`/`PutModel` 事件改经平台 `outbox.EnqueueEvent` 写 nw_outbox，删除自建 outbox 表（迁移与内联副本），平台转发循环才能投递（ADR0005）。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 已实现：`LoadRuntime` 建立 active/control/restore 目录、严格迁移和32字节主密钥引用检查；`ValidateProvider`/`ValidateModel` 完成无网络纯校验；`PutProvider`/`PutModel` 完成CAS、幂等回执、屏障检查和变更outbox事务。
- 本地提交：`4eec945320f8cf20bfc6673a8c3c8490385aca2f`（实现切片）。
- 当前仍阻塞：BuildApplication 尚未接入本域完整数据库/所有OpenAPI handler、site-config代次接线、activation回执和后台循环；真实模型、连接探测、密钥加密/轮换与费用预留/结算均未实现。
- `EmbeddingSpace` 现有合同没有独立归一化规则字段，本切片只按 provider/modelVersion/dimensions/preprocessingVersion 计算spaceId，矛盾已记录交接；不得自行扩展公共合同。
- 本机无Go、gofmt、make，未执行编译、迁移运行、静态检查或行为验收；未新增测试/样例、未调用真实接口或模型、未push。
- 下一步：先在有Go工具链的VPS完成格式/编译验证，再接入BuildApplication与全部handler；随后实现凭据加密、档位/路由和预留账本。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。

第二轮收口窄修：PutProvider同代普通摘要显式绑定callerService/dataEpoch，7天原结果先于新写屏障和当前active指针返回；新写拒绝任意非released屏障，不按屏障epoch/deadline过滤。过期键仅随成功新配置事务替换，PutModel及其旧共享helper未改。readCredentialReceipt与新增渠道回执参考签名均以ctx,op开头。首次候选Go检查通过但结构check因签名顺序失败，该次不记完整通过；修后根全量生成/编译/结构检查通过，证据见本轮交付记录。
