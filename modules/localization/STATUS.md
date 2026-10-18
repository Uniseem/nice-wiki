# 2026-10-18 语言计划源码候选待GitLab

分支 `codex/localization-20261018-r9`，基线 `1f1e1b08cc047fe5be826cbc2bb51547a3779c02`，源码候选 `98c02a0666572f9ec011eea0040fe4e947aa6853`。本轮实现 `EstimateLanguagePlan`、`ApproveLanguagePlan` 及必需支持：公开树/翻译记忆与界面缺项免费统计、文档/UI统一token来源、免费route与quote快照、24小时计划、批准revision/价目重核、原OperationContext冻结、planId+完整输入hash任务意图、jobs接受回执后CAS启用自动翻译。

GitLab 当前 HTTP 522，未生成客户端/schema，未执行 Go build/vet/gofmt、迁移或完整make check；两个约束函数与迁移均记 `blocked`，不记 implemented/verified。Portal 尚未为这两个操作装配可信 `background` costClass，完整浏览器HTTP链不可运行；翻译执行/付费worker及完整服务仍未完成。未调用真实接口、模型或部署，未新增测试、fixture、mock或样例。

## 2026-10-18 用户收尾指令

用户要求push后停止，全部代理已冻结。本次集成树已通过GitLab pipeline118源码检查；未完成职责保持逐项台账，不将独立WIP计入main。后续仅在新授权下接续，不自动继续。

以下为历史记录。

# 2026-10-18 原调用观察接线待GitLab

五类严格响应校验器与原键Get/Lookup生成客户端同步；过期仅只读观察，不延长原付费批准。实际完整输出独立于费用是否已结算，费用未知保留原引用。未运行真实接口或模型，完整服务仍未完成。

# 2026-10-18第三批源码检查通过

自部署GitLab pipeline95（源码1e65c5f；生成436、三个Go分片437/438/439、前端440）全部成功；16个Go模块readonly build/vet/gofmt、已有普通go test、完整make check及三个实际前端类型检查/构建通过。精确生成补丁零字节。普通测试均无测试文件；不代表业务验收或部署。

本域本批完整函数5项通过源码检查。完整服务、快照/恢复、业务验收按逐项台账继续未完成；不把部分装配记完整。未本机/SSH编译、未新增测试、未真实模型或部署。

以下为历史记录。

# 管理员语言建议付费批准源码待GitLab

用户2026-10-18明确选择管理员开启后允许。源码8a56a5e新增默认关闭的同站/同代CAS策略与不可变审批历史，MG纯服务只读当前approvalId，每次attempt按真实许可；旧版本/恢复换代不继承。5函数及005迁移仍blocked待GitLab，未执行模型、迁移或本机编译。全量SuggestLanguage及服务启动尚未齐，未冒充完整。

# 2026-10-18 第二批集成检查通过并已双推源码

自部署GitLab pipeline70（生成311，三个Go分片312/313/314，前端315）全部成功；精确879713字节生成物回传到3deb8ba9cd394ec43274999eaf4f5e0484e224ae；pipeline71对该精确SHA再次全部成功并核对生成零漂移。16个Go模块readonly build/vet/gofmt与go test、完整make check、Vue类型检查与构建通过；go test无测试文件，不代表业务验收。

本域本轮精确清单90项记verified，模块当前91 verified、1 implemented；accepted=0。源码交付 `3deb8ba9cd394ec43274999eaf4f5e0484e224ae` 已普通双推并核对两个远程main一致；本结果记录在同次授权下复查同步。完整服务装配、恢复与其余具体未完项继续按台账blocked/planned，不以编译通过代替业务验收。模块代理均已冻结。

以下为历史记录，其中待CI/真实设计阻塞以最新逐项台账为准。

最终只读复核窄修（2026-10-18）：源码4a1e596d499a9ba387a7294d77b443c986e4f7d5与60d2172bd00f973bc5dd3c7cc6b6c27d05677231。修复ReadTranslationResult.units应为10000、受保护machine不得冒称human、重编号投影65536字符预算，以及稳定anchor下无ID inline插入导致旧数组路径误归属。当前真实保护节点即使全部text消失仍核非稳定树形，不从更旧对象补最新anchor已移除字段。新增1个实际helper ProtectedAnchorShape，总计32个既有/修改业务函数+58助手；本批待统一GitLab CI。body/bodyHash经canonical目标还原与16MiB限制；completed仅计无原文回退单元，review但完整翻译允许completed且partial=false，partial有对应noticeKey；fileRef只写真权威引用，即时派生省略；PreviousTargetText可选，投影不伪造旧父整段。仅Git差异/源码与合同只读复核，无本地Go/gen/检查/测试/真实调用。继续冻结。

冻结交付（2026-10-18）：稳定字段保护投影源码33cd264a55df396f038d693f45c8d661e2eb0cb1，此前偏好Timestamp窄修8743c9f。ReadTranslation与4个实际参考助手就绪，累计32个既有业务/已改项、57个新增参考助手；本批待统一GitLab CI，未提前verified。原父/行换分组安全映射已按真实稳定ID收口；不能证明归属、段形状变化或保护source-only明确失败关闭。fileRef兼容可选，持久单元始终返回真实引用，即时派生省略。完整服务、付费执行/恢复链及模型建议仍未齐；按根要求冻结本域，不扩新功能包。

偏好链源码78097582679cf251b7460eef1f6e861f5b631b9c：SaveLanguagePreference与NewPreferenceHandlers就绪，累计31个既有业务/已改项与53个新增参考辅助，均待GitLab CI。仅保存同伪名的明确选择及30天到期，七天回执重放不延长期限；不存推测信号，不触发模型。SuggestLanguage快速模型缓存分支仍存在授权设计冲突，保持blocked。

人工保护窄修源码0096103e13f9cbd95ae4bfd8b7b99d6b9f2cc640：不可恢复编号模板与source-only分支均失败关闭，旧权威对象保留；ReadTranslation仍真正blocked，不新增ready计数，待统一GitLab CI。

# 第三组源拆分与只读链（2026-10-18，待GitLab CI）

源码35cc93176576b42cbbf3a737bef39758586399db：四个原有函数新代码就绪（SplitTranslationUnits、FetchTranslationSource、ReadTranslationUnits、ListTranslationProgress），同时复核提示/校验/TM/保护继承等旧代码，新增18个实际参考辅助。累计30个既有业务/已改项与52个新增参考辅助，均待GitLab CI，accepted为0。

完整ReadTranslation已有body/bodyHash与双水位读链，但人工保护table父/行换分组的安全旧译投影尚未完成，明确失败关闭并保持真正blocked，不计完整完成；不以原文回退冒充保留手工译文。完整服务与真实付费worker仍未齐。

第三组未本地/SSH编译/生成/格式/测试，未新增测试、fixture、mock、样例或临时验证程序，未真实接口/模型/服务/部署或push。仅git diff --check通过。引用根已新增platform.MemoryContext（TaskID等字段），不再依赖被生成裁剪的gen DTO。

---

# 第二组集中实现（2026-10-18，待统一GitLab CI）

源码5e90f43d6876833467610314722150fc678fe7e6：新增/修改15个既有函数与19个实际参考辅助。两组累计26个既有业务/已改函数、34个参考辅助；全部仍blocked待GitLab CI，完整服务和真实翻译执行尚未完成。

已写权威译文JSON对象/manifest激活、control日志代数预留与启动恢复、人工编辑/保护、精确翻译记忆/增量分类、上下文提示/标签完整性/有限降级/术语一次提醒、QA已就绪译文读取、固定界面字典验证/读取/人工CAS及平台模型/outbox适配。新增SourceText兼容字段和明确有限重试状态，不把targetText当原文。

004只追加迁移将单元身份改为(translation_id,unit_id)，未知旧归属整个迁移失败，不删除或补造。界面来源未安装只使对应能力不可用，不生成样例字典。没有真实模型/接口/进程/部署/测试调用；未本地或SSH编译/生成/格式，只有git diff --check通过。各函数精确源码SHA与ready IDs见progress.json。

---

# 当前集中实现（2026-10-18，首组待GitLab CI）

源码 `21c6a196c990d323c73429b769a38895beca7a93`：11个既有业务/已修改函数与15个实际参考辅助，均blocked待统一GitLab CI。语言原文配置不变，x/text规范化已实现；修正平台入口授权后错误依赖空Grants的问题。术语CRUD/软删/不可回退全站代数/固定代数分页/文档优先解析/自动批撤销/memory批准命令原结果及事件已写，修改与失效/outbox同事务。未发生模型费用，不自动重译。

完整服务仍因其余handler、翻译/事件/任务链未齐失败关闭。未执行本机/SSH的Go/gofmt/build/vet/gen/docs/check或测试，未新增测试/fixture/mock/样例/临时验证程序，未调用真实接口、模型、服务或部署，未push。`git diff --check`通过。精确ready IDs见progress.json.readyAwaitingCI。

下文历史证据不覆盖本轮已修改函数。

---

# 集成检查（2026-10-17）

语言读写已合入 main，GitLab pipeline 42 把本轮函数记为 verified。NormalizeLanguageTag 仍 blocked。下面是合入前的模块记录。

# 2026-10-17 语言配置

阶段：实施。分支 `codex/localization-20261017-r4`，基线 `a080a6c918e31d272c7a91368f59d5ad51d5f458`，源码提交 `650a1e208ff8656531aa2764dfd9e2bc4b1965b8`。部署 `not_started`。accepted 为 0。GitLab CI 未跑，未执行 go、gofmt、make build、make gen、make check 或 SSH。本轮在本域执行了 `make docs`（只调用 Python 渲染）。

- `localization.PutLanguage`、`localization.SetLanguageEnabled`、`localization.ListLanguages` 记为 implemented，验证 not_run。它们读写本域 `languages` 表，核对运行权威与 `runtime_meta` 代次，拒绝非 released 的 `local_barrier`，写入保留 7 天的原回执。
- `localization.NormalizeLanguageTag` 保持 blocked。合同没有写明大小写、脚本子标签和私用用法，函数只拒绝空标签和超过 35 的标签，其余返回 `DEPENDENCY_UNAVAILABLE`，不返回改写后的标签或方向。
- `BuildApplication` 已绑定上述三个 HTTP 操作，仍因其余处理器、事件订阅和任务循环缺失返回 `DEPENDENCY_UNAVAILABLE`。
- 停用只把本域 `translation_jobs` 的 queued、running、review 标为 cancelled，不调用 `jobs:cancelJob`，不删除人工译文，不调用模型，不写默认费用。
- 新文件尚未写入根 `docs/structure/framework.json`。公共协议未改。

下一步：等集成会话审核合入并跑 GitLab CI。

历史记录见下文，其中的旧下一步不再执行。

---

# 2026-10-17第三轮公共站点协议接续（仅合同）

根公共候选统一签名siteId，清proof后仍可保留已核本站，传输层删除旧proof头；无声明的旧服务控制仍兼容system。EstimateLanguagePlan的未来调用协议已同步，不能用system替代真实本站或自报未签名站点。本域函数仍planned，没有业务源码、调用或完成数提升，公共代码检查由根GitLab统一。

下面第二轮“公共站点传递未齐”是当时历史事实；当前公共来源与检查结果以根本轮交付为准。

---

# 2026-10-17第二轮公共免费估算依赖（仅合同）

根集成会话为已登记的EstimateLanguagePlan补统一model-gateway:estimateTokens免费文本估算依赖，替代域内重复字节估算。按接口限制分批传实际文本，禁止截断；纯localization服务清原用户委托并保留站点/epoch/root/费用类别/更早截止，当前功能权限由本域先核。已批准的翻译任务和费用执行均未接入。

交叉审查发现公共前置缺口：当前平台无portal proof的纯服务actor站点固定为system，尚不能在HTTP接收端保留本站。EstimateLanguagePlan实施前须收口可信站点传递协议；不得发送未签名站点或保留原proof绕过，本轮未改公共协议，也未声称此调用链已通。

本次只更新未来函数合同、相应流程与契约摘要，EstimateLanguagePlan保持planned，未创建业务源码或提升完成数。生成与结构检查由根GitLab统一，未执行真实接口/模型、业务验收或部署；本轮授权仍由根统一双远程推送。

---

# 本地化当前进度

更新：2026-10-15。阶段：R1-15 实施中；本域启动私有持久化与恢复校验已落地，翻译、术语、人工保护、模型调用、任务和事件业务尚未完成、尚未验收、尚未部署。

- 2026-10-15 集成复核修复：迁移改为 `migrations/migrations.go` go:embed 读取 `001_initial.sql`，删除内联 SQL 副本；执行器支持多编号迁移并校验名称。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 基线：`8ae6fcfc6302e2fd41a998faef18be52e2a9714c`；当前分支：`main`；本次提交：`cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`。
- 已完成登记的启动与契约生成准备：`cmd/localization` 入口、`BuildApplication`/HTTP 生命周期骨架、独立 `go.mod`、Makefile、非敏感配置、manifest、Dockerfile，以及 Vue/Vite/TypeScript UI 构建基础和 pnpm 锁文件。
- 本次新增本域私有持久化启动切片：active 区 `localization.db` 使用 SQLite WAL/外键/忙等待，control 区追加 `localization-journal.jsonl`；按固定迁移集建立语言、翻译任务/单元、译文清单、术语、界面字典、语言建议、已批准提议和缓存回答译文表，并初始化平台 inbox/outbox。
- `LoadRuntime` 已接入 `BuildApplication`：迁移版本与表结构校验、journal 损坏/未完成决定隔离、默认语言至多一个、术语代数读取及旧租约登记均在启动阶段执行；任一步失败都关闭资源并返回已登记错误码。业务 handler、事件订阅、outbox 转发和任务循环缺失时仍按合同返回 `DEPENDENCY_UNAVAILABLE`，普通业务路由、翻译和模型调用未接入。
- UI `pnpm run typecheck`、`pnpm run build` 通过；未进行浏览器行为验收。`python scripts/check_docs.py --privacy-only` 通过；完整文档检查未运行，根 `docs/structure/framework.json` 的本次新文件登记由集成会话处理。
- 本机没有 Go、gofmt、make、Docker 或可用 VPS，因此未生成 `go.sum`，未执行 `make gen`、Go 编译/vet/gofmt、Docker 构建或业务测试。生成的 Go/TS 文件未手改、未变更。
- 本次没有新增测试、样例、真实接口调用或模型调用；部署状态仍为 `not_started`。逐文件、逐函数事实见 [progress.json](progress.json)。

下一步：接入完整 OpenAPI handler、持有并在 Shutdown 中关闭本域 Store，再按合同接入事件订阅、outbox 转发和任务循环；业务验收仍需单独授权。

并行工作只改本域，公共资料交集成会话；根文件、其他模块和生成目录由集成会话维护。
